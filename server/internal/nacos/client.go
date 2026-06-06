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

// Client is the REST adapter over the Nacos 3.x AI Registry MCP API. It
// implements NacosQuerier. Every request carries the identity header
// (NACOS_AUTH_IDENTITY_KEY:VALUE) which the Nacos 3.x admin API requires even
// when auth is disabled (dev standalone). See REST.md for the measured API.
type Client struct {
	base   string
	idKey  string
	idVal  string
	client *http.Client
}

func NewClient(base, identityKey, identityValue string) *Client {
	return &Client{
		base:   strings.TrimRight(base, "/"),
		idKey:  identityKey,
		idVal:  identityValue,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

type envelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// nacosMCP is the read shape returned by get + list pageItems.
type nacosMCP struct {
	Name          string `json:"name"`
	Protocol      string `json:"protocol"`
	Description   string `json:"description"`
	Version       string `json:"version"`
	VersionDetail struct {
		Version string `json:"version"`
	} `json:"versionDetail"`
	Enabled            bool       `json:"enabled"`
	LocalServerConfig  *localCfg  `json:"localServerConfig"`
	RemoteServerConfig *remoteCfg `json:"remoteServerConfig"`
}

type localCfg struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
	EnvKeys []string `json:"env_keys"`
}

type remoteCfg struct {
	URL        string   `json:"url"`
	HeaderKeys []string `json:"header_keys"`
}

func (n nacosMCP) toShape() MCPServerShape {
	s := MCPServerShape{
		Name:      n.Name,
		Version:   n.Version,
		Transport: n.Protocol,
		Lifecycle: "offline",
	}
	if s.Version == "" {
		s.Version = n.VersionDetail.Version
	}
	if n.Enabled {
		s.Lifecycle = "published"
	}
	if n.LocalServerConfig != nil {
		s.Command = n.LocalServerConfig.Command
		s.Args = n.LocalServerConfig.Args
		s.EnvKeys = n.LocalServerConfig.EnvKeys
	}
	if n.RemoteServerConfig != nil {
		s.URL = n.RemoteServerConfig.URL
		s.HeaderKeys = n.RemoteServerConfig.HeaderKeys
	}
	return s
}

func (c *Client) do(ctx context.Context, method, path string, query, form url.Values) (json.RawMessage, error) {
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

func (c *Client) ListMCPServers(ctx context.Context, namespace string) ([]MCPServerShape, error) {
	q := url.Values{"namespaceId": {namespace}, "pageNo": {"1"}, "pageSize": {"500"}}
	data, err := c.do(ctx, http.MethodGet, "/nacos/v3/admin/ai/mcp/list", q, nil)
	if err != nil {
		return nil, err
	}
	var page struct {
		PageItems []nacosMCP `json:"pageItems"`
	}
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, err
	}
	out := make([]MCPServerShape, 0, len(page.PageItems))
	for _, n := range page.PageItems {
		out = append(out, n.toShape())
	}
	return out, nil
}

func (c *Client) GetMCPServer(ctx context.Context, namespace, name, ref string) (MCPServerShape, error) {
	q := url.Values{"namespaceId": {namespace}, "mcpName": {name}}
	if ref != "" && ref != "stable" && ref != "latest" {
		q.Set("version", ref) // pinned version; tags fall through to latest
	}
	data, err := c.do(ctx, http.MethodGet, "/nacos/v3/admin/ai/mcp", q, nil)
	if err != nil {
		return MCPServerShape{}, err
	}
	var n nacosMCP
	if err := json.Unmarshal(data, &n); err != nil {
		return MCPServerShape{}, err
	}
	return n.toShape(), nil
}

func (c *Client) RegisterMCPServer(ctx context.Context, namespace string, s MCPServerShape) error {
	spec := map[string]any{
		"name":          s.Name,
		"protocol":      s.Transport,
		"description":   "",
		"versionDetail": map[string]string{"version": s.Version},
	}
	if s.Transport == "stdio" {
		spec["localServerConfig"] = map[string]any{"command": s.Command, "args": s.Args, "env_keys": s.EnvKeys}
	} else {
		spec["remoteServerConfig"] = map[string]any{"url": s.URL, "header_keys": s.HeaderKeys}
	}
	specJSON, err := json.Marshal(spec)
	if err != nil {
		return err
	}
	form := url.Values{
		"namespaceId":         {namespace},
		"mcpName":             {s.Name},
		"serverSpecification": {string(specJSON)},
	}
	_, err = c.do(ctx, http.MethodPost, "/nacos/v3/admin/ai/mcp", nil, form)
	return err
}

func (c *Client) SetMCPLifecycle(ctx context.Context, namespace, name, version, lifecycle string) error {
	spec := map[string]any{
		"name":          name,
		"versionDetail": map[string]string{"version": version},
		"enabled":       lifecycle == "published",
	}
	specJSON, _ := json.Marshal(spec)
	form := url.Values{
		"namespaceId":         {namespace},
		"mcpName":             {name},
		"serverSpecification": {string(specJSON)},
	}
	_, err := c.do(ctx, http.MethodPut, "/nacos/v3/admin/ai/mcp", nil, form)
	return err
}
