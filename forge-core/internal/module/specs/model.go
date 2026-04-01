package specs

import "time"

// ==================== Standards ====================

type Standard struct {
	ID        int64     `json:"id"`
	TenantID  int64     `json:"tenantId"`
	Name      string    `json:"name"`
	Category  string    `json:"category"`
	Scope     string    `json:"scope"`
	ScopeID   int64     `json:"scopeId"`
	ParentID  *int64    `json:"parentId,omitempty"`
	Content   string    `json:"content"`
	Version   int       `json:"version"`
	Status    string    `json:"status"`
	CreatedBy *int64    `json:"createdBy,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type CreateStandardReq struct {
	Name     string `json:"name" binding:"required,max=200"`
	Category string `json:"category" binding:"required,oneof=JAVA SQL REDIS KAFKA API SECURITY NAMING GIT"`
	Scope    string `json:"scope" binding:"required,oneof=COMPANY TEAM PROJECT"`
	ScopeID  int64  `json:"scopeId"`
	ParentID *int64 `json:"parentId"`
	Content  string `json:"content" binding:"required"`
}

type UpdateStandardReq struct {
	Name    string `json:"name" binding:"required,max=200"`
	Content string `json:"content" binding:"required"`
}

type StandardFilter struct {
	Category string `form:"category"`
	Scope    string `form:"scope"`
	ScopeID  *int64 `form:"scopeId"`
	Status   string `form:"status"`
	Page     int    `form:"page,default=1"`
	PageSize int    `form:"pageSize,default=20"`
}

// ==================== Prompt Templates ====================

type PromptTemplate struct {
	ID           int64     `json:"id"`
	TenantID     int64     `json:"tenantId"`
	Name         string    `json:"name"`
	Purpose      string    `json:"purpose"`
	SystemPrompt string    `json:"systemPrompt"`
	UserTemplate string    `json:"userTemplate"`
	Variables    []string  `json:"variables"`
	Version      int       `json:"version"`
	IsDefault    bool      `json:"isDefault"`
	CreatedBy    *int64    `json:"createdBy,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type CreatePromptTemplateReq struct {
	Name         string   `json:"name" binding:"required,max=200"`
	Purpose      string   `json:"purpose" binding:"required,oneof=requirement-analysis code-generation code-review test-generation fix-generation doc-generation"`
	SystemPrompt string   `json:"systemPrompt" binding:"required"`
	UserTemplate string   `json:"userTemplate" binding:"required"`
	Variables    []string `json:"variables"`
	IsDefault    bool     `json:"isDefault"`
}

type UpdatePromptTemplateReq struct {
	Name         string   `json:"name" binding:"required,max=200"`
	Purpose      string   `json:"purpose" binding:"required,oneof=requirement-analysis code-generation code-review test-generation fix-generation doc-generation"`
	SystemPrompt string   `json:"systemPrompt" binding:"required"`
	UserTemplate string   `json:"userTemplate" binding:"required"`
	Variables    []string `json:"variables"`
	IsDefault    bool     `json:"isDefault"`
}

type PromptTemplateFilter struct {
	Purpose  string `form:"purpose"`
	Page     int    `form:"page,default=1"`
	PageSize int    `form:"pageSize,default=20"`
}

// ==================== Review Rules ====================

type ReviewRule struct {
	ID          int64                  `json:"id"`
	TenantID    int64                  `json:"tenantId"`
	Name        string                 `json:"name"`
	Category    string                 `json:"category"`
	Scope       string                 `json:"scope"`
	ScopeID     int64                  `json:"scopeId"`
	RuleType    string                 `json:"ruleType"`
	Definition  map[string]interface{} `json:"definition"`
	Severity    string                 `json:"severity"`
	AutoFix     bool                   `json:"autoFix"`
	FixTemplate *string                `json:"fixTemplate,omitempty"`
	Enabled     bool                   `json:"enabled"`
	CreatedBy   *int64                 `json:"createdBy,omitempty"`
	CreatedAt   time.Time              `json:"createdAt"`
	UpdatedAt   time.Time              `json:"updatedAt"`
}

type CreateReviewRuleReq struct {
	Name        string                 `json:"name" binding:"required,max=200"`
	Category    string                 `json:"category" binding:"required,oneof=CODING SECURITY PERFORMANCE DATABASE API_COMPAT CUSTOM"`
	Scope       string                 `json:"scope" binding:"required,oneof=COMPANY TEAM PROJECT"`
	ScopeID     int64                  `json:"scopeId"`
	RuleType    string                 `json:"ruleType" binding:"required,oneof=PATTERN AST AI_CHECK"`
	Definition  map[string]interface{} `json:"definition" binding:"required"`
	Severity    string                 `json:"severity" binding:"required,oneof=ERROR WARNING INFO"`
	AutoFix     bool                   `json:"autoFix"`
	FixTemplate *string                `json:"fixTemplate"`
}

type UpdateReviewRuleReq struct {
	Name        string                 `json:"name" binding:"required,max=200"`
	Category    string                 `json:"category" binding:"required,oneof=CODING SECURITY PERFORMANCE DATABASE API_COMPAT CUSTOM"`
	RuleType    string                 `json:"ruleType" binding:"required,oneof=PATTERN AST AI_CHECK"`
	Definition  map[string]interface{} `json:"definition" binding:"required"`
	Severity    string                 `json:"severity" binding:"required,oneof=ERROR WARNING INFO"`
	AutoFix     bool                   `json:"autoFix"`
	FixTemplate *string                `json:"fixTemplate"`
}

type ReviewRuleFilter struct {
	Category string `form:"category"`
	Severity string `form:"severity"`
	Scope    string `form:"scope"`
	ScopeID  *int64 `form:"scopeId"`
	Page     int    `form:"page,default=1"`
	PageSize int    `form:"pageSize,default=20"`
}

// ==================== Scaffold Templates ====================

type ScaffoldTemplate struct {
	ID           int64     `json:"id"`
	TenantID     int64     `json:"tenantId"`
	Name         string    `json:"name"`
	ProjectType  string    `json:"projectType"`
	Description  *string   `json:"description,omitempty"`
	TemplateRepo *string   `json:"templateRepo,omitempty"`
	Variables    []string  `json:"variables"`
	PostHooks    []string  `json:"postHooks"`
	Version      int       `json:"version"`
	CreatedBy    *int64    `json:"createdBy,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type ScaffoldFilter struct {
	ProjectType string `form:"projectType"`
	Page        int    `form:"page,default=1"`
	PageSize    int    `form:"pageSize,default=20"`
}

// ==================== Effective Specs ====================

type EffectiveSpecs struct {
	Standards []*Standard   `json:"standards"`
	Rules     []*ReviewRule `json:"rules"`
}

// ==================== Pagination ====================

type PageResult[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}
