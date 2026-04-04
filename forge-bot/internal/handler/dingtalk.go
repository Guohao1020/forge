package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/shulex/forge/forge-bot/internal/card"
	dt "github.com/shulex/forge/forge-bot/internal/dingtalk"
	"github.com/shulex/forge/forge-bot/internal/forgeapi"
)

// DingTalkHandler processes DingTalk webhook messages and card callbacks.
type DingTalkHandler struct {
	token    string
	secret   string
	dtClient *dt.Client
	forge    *forgeapi.Client
	cards    *card.Renderer
}

func NewDingTalkHandler(token, secret, webhook string, forge *forgeapi.Client, cards *card.Renderer) *DingTalkHandler {
	return &DingTalkHandler{
		token:    token,
		secret:   secret,
		dtClient: dt.NewClient(webhook, secret),
		forge:    forge,
		cards:    cards,
	}
}

// HandleWebhook processes incoming DingTalk messages.
// POST /dingtalk/webhook
func (h *DingTalkHandler) HandleWebhook(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "read body failed"})
		return
	}

	// Verify signature if configured
	timestamp := c.GetHeader("timestamp")
	sign := c.GetHeader("sign")
	if h.token != "" && !dt.VerifySignature(h.token, timestamp, sign, string(body)) {
		slog.Warn("signature verification failed")
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid signature"})
		return
	}

	var msg dt.IncomingMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		slog.Error("parse webhook message failed", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid message"})
		return
	}

	slog.Info("received dingtalk message",
		"sender", msg.SenderNick,
		"type", msg.MsgType,
		"conversation", msg.ConversationType,
	)

	// Process the message
	response := h.processMessage(&msg)

	// Reply via session webhook (if available) or configured webhook
	if msg.SessionWebhook != "" {
		if err := h.dtClient.SendToSession(msg.SessionWebhook, response); err != nil {
			slog.Error("send session reply failed", "error", err)
		}
	} else {
		if err := h.dtClient.Send(response); err != nil {
			slog.Error("send reply failed", "error", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// processMessage routes the message to the appropriate handler.
func (h *DingTalkHandler) processMessage(msg *dt.IncomingMessage) *dt.OutgoingMessage {
	if msg.Text == nil || strings.TrimSpace(msg.Text.Content) == "" {
		return h.cards.WelcomeCard()
	}

	content := strings.TrimSpace(msg.Text.Content)
	// Remove @bot mention prefix
	if idx := strings.Index(content, " "); idx > 0 && strings.HasPrefix(content, "@") {
		content = strings.TrimSpace(content[idx:])
	}
	content = strings.TrimSpace(content)

	slog.Info("processing command", "content", content, "sender", msg.SenderNick)

	// Route commands
	switch {
	case content == "帮助" || content == "help" || content == "":
		return h.cards.WelcomeCard()

	case content == "项目列表" || content == "projects" || content == "list":
		return h.handleListProjects()

	case strings.HasPrefix(content, "需求") || strings.HasPrefix(content, "我想") ||
		strings.HasPrefix(content, "帮我") || strings.HasPrefix(content, "请"):
		return h.handleNewRequirement(content, msg.SenderNick)

	default:
		// Treat as a new requirement
		return h.handleNewRequirement(content, msg.SenderNick)
	}
}

// handleListProjects lists available projects.
func (h *DingTalkHandler) handleListProjects() *dt.OutgoingMessage {
	projects, err := h.forge.ListProjects()
	if err != nil {
		slog.Error("list projects failed", "error", err)
		return h.cards.ErrorCard("获取项目列表失败: " + err.Error())
	}
	return h.cards.ProjectListCard(projects)
}

// handleNewRequirement creates a task from the requirement text.
func (h *DingTalkHandler) handleNewRequirement(content, senderNick string) *dt.OutgoingMessage {
	// TODO: Auto-detect or ask which project to use
	// For now, return a markdown message acknowledging the requirement
	return &dt.OutgoingMessage{
		MsgType: "markdown",
		Markdown: &dt.Markdown{
			Title: "需求已接收",
			Text: "### 📋 需求已接收\n\n" +
				"**来自**: " + senderNick + "\n\n" +
				"**内容**: " + truncateContent(content, 200) + "\n\n" +
				"正在处理中，请稍候...\n\n" +
				"> 提示：请在 Forge 工作台中选择目标项目后再提交需求，" +
				"或者用「项目列表」命令查看可用项目。",
		},
	}
}

// HandleCardCallback processes DingTalk interactive card callbacks.
// POST /dingtalk/callback
func (h *DingTalkHandler) HandleCardCallback(c *gin.Context) {
	var req dt.CardCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid callback"})
		return
	}

	slog.Info("card callback received",
		"user_id", req.UserID,
		"msg_id", req.MsgID,
		"content", req.Content,
	)

	// TODO: Parse action from content JSON, route to appropriate handler
	// (approve plan, select option, etc.)

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func truncateContent(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len([]rune(s)) <= maxLen {
		return s
	}
	return string([]rune(s)[:maxLen]) + "..."
}
