package dingtalk

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Client sends messages to DingTalk via webhook.
type Client struct {
	webhook string
	secret  string
}

func NewClient(webhook, secret string) *Client {
	return &Client{webhook: webhook, secret: secret}
}

// Send sends a message to DingTalk via the configured webhook.
func (c *Client) Send(msg *OutgoingMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	webhookURL := c.signedURL()

	resp, err := http.Post(webhookURL, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("send to dingtalk: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("dingtalk error: status=%d body=%s", resp.StatusCode, string(body))
	}

	slog.Debug("message sent to dingtalk", "type", msg.MsgType)
	return nil
}

// SendToSession sends a message using a session webhook (reply to specific conversation).
func (c *Client) SendToSession(sessionWebhook string, msg *OutgoingMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	resp, err := http.Post(sessionWebhook, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("send to session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("session send error: status=%d body=%s", resp.StatusCode, string(body))
	}
	return nil
}

// signedURL generates a signed DingTalk webhook URL with timestamp + HMAC.
func (c *Client) signedURL() string {
	if c.secret == "" {
		return c.webhook
	}

	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	stringToSign := timestamp + "\n" + c.secret

	mac := hmac.New(sha256.New, []byte(c.secret))
	mac.Write([]byte(stringToSign))
	sign := url.QueryEscape(base64.StdEncoding.EncodeToString(mac.Sum(nil)))

	return fmt.Sprintf("%s&timestamp=%s&sign=%s", c.webhook, timestamp, sign)
}

// VerifySignature verifies an incoming DingTalk webhook request signature.
func VerifySignature(token, timestamp, sign, body string) bool {
	if token == "" {
		return true // no verification configured
	}

	stringToSign := timestamp + "\n" + token
	mac := hmac.New(sha256.New, []byte(token))
	mac.Write([]byte(stringToSign))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(sign), []byte(expected))
}
