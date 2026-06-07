package nacos

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// providerGroup is the fixed Nacos config-center group that holds the LLM
// provider catalog. One config (dataId = provider name) per provider.
const providerGroup = "forge-llm-providers"

// ProviderClient is the REST adapter over the Nacos 3.x config center
// (cs/config). It implements ProviderQuerier. Unlike the MCP catalog (AI
// Registry), providers live as plain configs because the AI Registry has no
// native LLM-provider resource type. Every request carries the identity header
// which the Nacos 3.x admin API requires even when auth is disabled (dev
// standalone). See REST-providers.md for the measured API.
type ProviderClient struct {
	base   string
	idKey  string
	idVal  string
	client *http.Client
}

func NewProviderClient(base, identityKey, identityValue string) *ProviderClient {
	return &ProviderClient{
		base:   strings.TrimRight(base, "/"),
		idKey:  identityKey,
		idVal:  identityValue,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

// do mirrors Client.do: set the identity header, decode the {code,message,data}
// envelope, and return the raw data payload (or an error).
func (c *ProviderClient) do(ctx context.Context, method, path string, query, form url.Values) (json.RawMessage, error) {
	u := c.base + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set(c.idKey, c.idVal)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nacos %s %s: status %d: %s", method, path, resp.StatusCode, string(raw))
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("nacos %s %s: decode: %w", method, path, err)
	}
	if env.Code != 0 {
		return nil, fmt.Errorf("nacos %s %s: code %d: %s", method, path, env.Code, env.Message)
	}
	return env.Data, nil
}

// RegisterProvider publishes (UPSERT) a provider config. Re-publishing the same
// dataId overwrites in place, so register doubles as update — no create/update
// branch needed. dataId = provider name; content = ProviderShape JSON.
func (c *ProviderClient) RegisterProvider(ctx context.Context, namespace string, p ProviderShape) error {
	// Namespace is request-scoped metadata, not part of the stored content.
	p.Namespace = ""
	content, err := json.Marshal(p)
	if err != nil {
		return err
	}
	form := url.Values{
		"dataId":      {p.Name},
		"groupName":   {providerGroup},
		"namespaceId": {namespace},
		"type":        {"json"},
		"content":     {string(content)},
	}
	_, err = c.do(ctx, http.MethodPost, "/nacos/v3/admin/cs/config", nil, form)
	return err
}

// GetProvider reads back a single provider config. The get response is an
// envelope whose data.content holds the ProviderShape as an escaped JSON string,
// so it needs a second unmarshal.
//
// ref: pinning a specific historical version is out of scope for the basic
// adapter (the config center's basic GET returns only the current content;
// per-version retrieval would require the history API). Tags and concrete
// versions alike fall through to the current published content.
func (c *ProviderClient) GetProvider(ctx context.Context, namespace, name, ref string) (ProviderShape, error) {
	_ = ref // see doc comment: current adapter always returns current content
	q := url.Values{
		"dataId":      {name},
		"groupName":   {providerGroup},
		"namespaceId": {namespace},
	}
	data, err := c.do(ctx, http.MethodGet, "/nacos/v3/admin/cs/config", q, nil)
	if err != nil {
		return ProviderShape{}, err
	}
	var meta struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return ProviderShape{}, fmt.Errorf("nacos get %s: decode envelope data: %w", name, err)
	}
	var shape ProviderShape
	if err := json.Unmarshal([]byte(meta.Content), &shape); err != nil {
		return ProviderShape{}, fmt.Errorf("nacos get %s: decode content: %w", name, err)
	}
	shape.Namespace = namespace
	return shape, nil
}

// ListProviders lists the provider configs in a namespace. The list endpoint's
// pageItems carry only metadata (no content), so each dataId is fetched
// individually via GetProvider to fill the shape. Entries that fail to fetch are
// skipped so one bad config does not blank the whole catalog.
func (c *ProviderClient) ListProviders(ctx context.Context, namespace string) ([]ProviderShape, error) {
	q := url.Values{
		"groupName":   {providerGroup},
		"namespaceId": {namespace},
		"pageNo":      {"1"},
		"pageSize":    {"500"},
	}
	data, err := c.do(ctx, http.MethodGet, "/nacos/v3/admin/cs/config/list", q, nil)
	if err != nil {
		return nil, err
	}
	var page struct {
		PageItems []struct {
			DataID string `json:"dataId"`
		} `json:"pageItems"`
	}
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, err
	}
	out := make([]ProviderShape, 0, len(page.PageItems))
	for _, item := range page.PageItems {
		shape, err := c.GetProvider(ctx, namespace, item.DataID, "")
		if err != nil {
			continue // skip configs that fail to fetch; keep the rest of the catalog
		}
		out = append(out, shape)
	}
	return out, nil
}

// SetProviderLifecycle flips a provider's lifecycle field and re-publishes. The
// config center has no native lifecycle concept, so lifecycle is just a content
// field; offline is a soft delete (the config and its history stay intact).
//
// version is accepted for signature symmetry with the MCP adapter but is
// currently IGNORED: GetProvider only returns the current published content (the
// config center has no per-version retrieval in the basic API), so the flip
// always targets the single live config regardless of version. Per-version
// lifecycle would require the config history API; until then a provider has one
// effective version. Callers should not rely on version to scope the flip.
func (c *ProviderClient) SetProviderLifecycle(ctx context.Context, namespace, name, version, lifecycle string) error {
	shape, err := c.GetProvider(ctx, namespace, name, version)
	if err != nil {
		return err
	}
	shape.Lifecycle = lifecycle
	return c.RegisterProvider(ctx, namespace, shape)
}
