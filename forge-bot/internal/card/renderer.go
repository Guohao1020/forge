package card

import (
	"fmt"
	"strings"

	"github.com/shulex/forge/forge-bot/internal/dingtalk"
)

// Renderer creates DingTalk ActionCard messages from Forge data.
type Renderer struct{}

func NewRenderer() *Renderer {
	return &Renderer{}
}

// WelcomeCard renders a welcome card when the bot is first mentioned.
func (r *Renderer) WelcomeCard() *dingtalk.OutgoingMessage {
	return &dingtalk.OutgoingMessage{
		MsgType: "actionCard",
		ActionCard: &dingtalk.ActionCard{
			Title: "Forge AI 助手",
			Text: "## Forge AI 开发助手\n\n" +
				"我可以帮你：\n" +
				"- **创建需求** — 描述你想要的功能\n" +
				"- **查看项目** — 了解项目状态\n" +
				"- **查看任务** — 跟踪开发进度\n\n" +
				"直接 @我 + 你的需求描述即可开始！",
			SingleTitle: "打开 Forge 工作台",
			SingleURL:   "dingtalk://dingtalkclient/page/link?url=http://localhost:3000",
		},
	}
}

// RequirementClarificationCard renders AI clarification options.
func (r *Renderer) RequirementClarificationCard(taskID int64, question string, options []string) *dingtalk.OutgoingMessage {
	text := fmt.Sprintf("## 需求澄清\n\n%s\n\n", question)
	for i, opt := range options {
		text += fmt.Sprintf("%d. %s\n", i+1, opt)
	}

	var btns []dingtalk.Button
	for i, opt := range options {
		btns = append(btns, dingtalk.Button{
			Title:     fmt.Sprintf("%d. %s", i+1, truncate(opt, 20)),
			ActionURL: fmt.Sprintf("dingtalk://dingtalkclient/page/link?url=http://localhost:8085/dingtalk/callback?task=%d&choice=%d", taskID, i+1),
		})
	}

	return &dingtalk.OutgoingMessage{
		MsgType: "actionCard",
		ActionCard: &dingtalk.ActionCard{
			Title:          "需求澄清",
			Text:           text,
			BtnOrientation: "0",
			Btns:           btns,
		},
	}
}

// PlanSummaryCard renders a task plan summary with approve/reject buttons.
func (r *Renderer) PlanSummaryCard(taskID int64, title string, steps []string) *dingtalk.OutgoingMessage {
	text := fmt.Sprintf("## 开发计划\n\n**任务**: %s\n\n", title)
	text += "**步骤**:\n"
	for i, step := range steps {
		text += fmt.Sprintf("%d. %s\n", i+1, step)
	}

	return &dingtalk.OutgoingMessage{
		MsgType: "actionCard",
		ActionCard: &dingtalk.ActionCard{
			Title:          "开发计划 — " + truncate(title, 30),
			Text:           text,
			BtnOrientation: "1",
			Btns: []dingtalk.Button{
				{Title: "批准执行", ActionURL: fmt.Sprintf("dingtalk://dingtalkclient/page/link?url=http://localhost:8085/dingtalk/callback?task=%d&action=approve", taskID)},
				{Title: "需要修改", ActionURL: fmt.Sprintf("dingtalk://dingtalkclient/page/link?url=http://localhost:8085/dingtalk/callback?task=%d&action=revise", taskID)},
			},
		},
	}
}

// TaskCompletedCard renders a task completion notification with PR link.
func (r *Renderer) TaskCompletedCard(taskID int64, title, prURL, branch string, filesChanged int) *dingtalk.OutgoingMessage {
	text := fmt.Sprintf("## 任务完成\n\n**任务**: %s\n\n", title)
	text += fmt.Sprintf("- **分支**: `%s`\n", branch)
	text += fmt.Sprintf("- **文件变更**: %d 个文件\n", filesChanged)
	if prURL != "" {
		text += fmt.Sprintf("- **PR**: [查看 Pull Request](%s)\n", prURL)
	}

	var btns []dingtalk.Button
	if prURL != "" {
		btns = append(btns, dingtalk.Button{
			Title:     "查看 PR",
			ActionURL: fmt.Sprintf("dingtalk://dingtalkclient/page/link?url=%s", prURL),
		})
	}
	btns = append(btns, dingtalk.Button{
		Title:     "打开工作台",
		ActionURL: fmt.Sprintf("dingtalk://dingtalkclient/page/link?url=http://localhost:3000/tasks/%d", taskID),
	})

	return &dingtalk.OutgoingMessage{
		MsgType: "actionCard",
		ActionCard: &dingtalk.ActionCard{
			Title:          "任务完成 — " + truncate(title, 30),
			Text:           text,
			BtnOrientation: "1",
			Btns:           btns,
		},
	}
}

// TaskProgressCard renders a progress update notification.
func (r *Renderer) TaskProgressCard(taskID int64, title, stage, detail string) *dingtalk.OutgoingMessage {
	stageIcons := map[string]string{
		"PLAN":         "📋",
		"TEST_WRITING": "🧪",
		"GENERATE":     "⚙️",
		"LINT":         "🔍",
		"REVIEW":       "📝",
		"TEST":         "✅",
		"DEPLOY":       "🚀",
	}
	icon := stageIcons[stage]
	if icon == "" {
		icon = "🔄"
	}

	return &dingtalk.OutgoingMessage{
		MsgType: "markdown",
		Markdown: &dingtalk.Markdown{
			Title: fmt.Sprintf("%s %s", icon, stage),
			Text: fmt.Sprintf("### %s %s\n\n**任务**: %s\n\n%s",
				icon, stage, truncate(title, 50), detail),
		},
	}
}

// ErrorCard renders an error message.
func (r *Renderer) ErrorCard(message string) *dingtalk.OutgoingMessage {
	return &dingtalk.OutgoingMessage{
		MsgType: "markdown",
		Markdown: &dingtalk.Markdown{
			Title: "错误",
			Text:  fmt.Sprintf("### ⚠️ 处理失败\n\n%s\n\n请稍后重试或联系管理员。", message),
		},
	}
}

// ProjectListCard renders a list of projects.
func (r *Renderer) ProjectListCard(projects []map[string]interface{}) *dingtalk.OutgoingMessage {
	text := "## 项目列表\n\n"
	if len(projects) == 0 {
		text += "暂无项目。请先在 Forge 工作台中创建或导入项目。"
	} else {
		for _, p := range projects {
			name, _ := p["name"].(string)
			desc, _ := p["description"].(string)
			if desc == "" {
				desc = "暂无描述"
			}
			text += fmt.Sprintf("- **%s** — %s\n", name, truncate(desc, 50))
		}
	}

	return &dingtalk.OutgoingMessage{
		MsgType: "markdown",
		Markdown: &dingtalk.Markdown{
			Title: "项目列表",
			Text:  text,
		},
	}
}

func truncate(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len([]rune(s)) <= maxLen {
		return s
	}
	return string([]rune(s)[:maxLen]) + "..."
}
