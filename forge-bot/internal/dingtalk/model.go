package dingtalk

// IncomingMessage is the DingTalk webhook message format.
type IncomingMessage struct {
	MsgType            string `json:"msgtype"`
	Text               *Text  `json:"text,omitempty"`
	MsgID              string `json:"msgId"`
	CreateAt           int64  `json:"createAt"`
	ConversationType   string `json:"conversationType"` // 1=single, 2=group
	ConversationID     string `json:"conversationId"`
	ConversationTitle  string `json:"conversationTitle,omitempty"`
	SenderID           string `json:"senderId"`
	SenderNick         string `json:"senderNick"`
	SenderCorpID       string `json:"senderCorpId,omitempty"`
	SenderStaffID      string `json:"senderStaffId,omitempty"`
	ChatbotCorpID      string `json:"chatbotCorpId,omitempty"`
	ChatbotUserID      string `json:"chatbotUserId,omitempty"`
	IsAtAll            bool   `json:"isAtAll"`
	AtUsers            []AtUser `json:"atUsers,omitempty"`
	SessionWebhook     string `json:"sessionWebhook"`
	SessionWebhookExpiredTime int64 `json:"sessionWebhookExpiredTime"`
}

type Text struct {
	Content string `json:"content"`
}

type AtUser struct {
	DingtalkID string `json:"dingtalkId"`
	StaffID    string `json:"staffId,omitempty"`
}

// OutgoingMessage is the response message format.
type OutgoingMessage struct {
	MsgType    string      `json:"msgtype"`
	Text       *Text       `json:"text,omitempty"`
	Markdown   *Markdown   `json:"markdown,omitempty"`
	ActionCard *ActionCard `json:"actionCard,omitempty"`
}

type Markdown struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

type ActionCard struct {
	Title          string   `json:"title"`
	Text           string   `json:"text"`
	SingleTitle    string   `json:"singleTitle,omitempty"`
	SingleURL      string   `json:"singleURL,omitempty"`
	BtnOrientation string   `json:"btnOrientation,omitempty"` // 0=vertical, 1=horizontal
	Btns           []Button `json:"btns,omitempty"`
}

type Button struct {
	Title     string `json:"title"`
	ActionURL string `json:"actionURL"`
}

// CardCallbackRequest is DingTalk's interactive card callback format.
type CardCallbackRequest struct {
	UserID    string `json:"userId"`
	Content   string `json:"content"` // JSON string with card action data
	MsgID     string `json:"msgId"`
	CorpID    string `json:"corpId"`
	ChatType  string `json:"chatType"`
}
