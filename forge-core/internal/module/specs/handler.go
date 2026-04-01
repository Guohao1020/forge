package specs

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/shulex/forge/forge-core/internal/pkg/response"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Helper: extract tenant_id from JWT context (set by auth middleware)
func getTenantID(c *gin.Context) int64 {
	if v, ok := c.Get("tenant_id"); ok {
		if tid, ok := v.(int64); ok {
			return tid
		}
	}
	return 1 // default tenant
}

// Helper: extract user_id from JWT context
func getUserID(c *gin.Context) int64 {
	if v, ok := c.Get("user_id"); ok {
		if uid, ok := v.(int64); ok {
			return uid
		}
	}
	return 0
}

// Helper: parse path param :id as int64
func parseID(c *gin.Context, param string) (int64, bool) {
	idStr := c.Param(param)
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid id parameter")
		return 0, false
	}
	return id, true
}

// ==================== Standards Handlers ====================

func (h *Handler) ListStandards(c *gin.Context) {
	tenantID := getTenantID(c)
	var filter StandardFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid query parameters: "+err.Error())
		return
	}
	result, err := h.svc.ListStandards(c.Request.Context(), tenantID, filter)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to list standards: "+err.Error())
		return
	}
	response.OK(c, result)
}

func (h *Handler) GetStandard(c *gin.Context) {
	tenantID := getTenantID(c)
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	result, err := h.svc.GetStandard(c.Request.Context(), tenantID, id)
	if err != nil {
		response.Fail(c, http.StatusNotFound, "standard not found")
		return
	}
	response.OK(c, result)
}

func (h *Handler) CreateStandard(c *gin.Context) {
	tenantID := getTenantID(c)
	userID := getUserID(c)
	var req CreateStandardReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	result, err := h.svc.CreateStandard(c.Request.Context(), tenantID, userID, req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to create standard: "+err.Error())
		return
	}
	response.OK(c, result)
}

func (h *Handler) UpdateStandard(c *gin.Context) {
	tenantID := getTenantID(c)
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req UpdateStandardReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	result, err := h.svc.UpdateStandard(c.Request.Context(), tenantID, id, req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to update standard: "+err.Error())
		return
	}
	response.OK(c, result)
}

func (h *Handler) DeleteStandard(c *gin.Context) {
	tenantID := getTenantID(c)
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteStandard(c.Request.Context(), tenantID, id); err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to delete standard: "+err.Error())
		return
	}
	response.OK(c, nil)
}

// ==================== Prompt Templates Handlers ====================

func (h *Handler) ListPromptTemplates(c *gin.Context) {
	tenantID := getTenantID(c)
	var filter PromptTemplateFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid query parameters: "+err.Error())
		return
	}
	result, err := h.svc.ListPromptTemplates(c.Request.Context(), tenantID, filter)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to list prompt templates: "+err.Error())
		return
	}
	response.OK(c, result)
}

func (h *Handler) GetPromptTemplate(c *gin.Context) {
	tenantID := getTenantID(c)
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	result, err := h.svc.GetPromptTemplate(c.Request.Context(), tenantID, id)
	if err != nil {
		response.Fail(c, http.StatusNotFound, "prompt template not found")
		return
	}
	response.OK(c, result)
}

func (h *Handler) CreatePromptTemplate(c *gin.Context) {
	tenantID := getTenantID(c)
	userID := getUserID(c)
	var req CreatePromptTemplateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	result, err := h.svc.CreatePromptTemplate(c.Request.Context(), tenantID, userID, req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to create prompt template: "+err.Error())
		return
	}
	response.OK(c, result)
}

func (h *Handler) UpdatePromptTemplate(c *gin.Context) {
	tenantID := getTenantID(c)
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req UpdatePromptTemplateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	result, err := h.svc.UpdatePromptTemplate(c.Request.Context(), tenantID, id, req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to update prompt template: "+err.Error())
		return
	}
	response.OK(c, result)
}

func (h *Handler) DeletePromptTemplate(c *gin.Context) {
	tenantID := getTenantID(c)
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.DeletePromptTemplate(c.Request.Context(), tenantID, id); err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to delete prompt template: "+err.Error())
		return
	}
	response.OK(c, nil)
}

// ==================== Review Rules Handlers ====================

func (h *Handler) ListReviewRules(c *gin.Context) {
	tenantID := getTenantID(c)
	var filter ReviewRuleFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid query parameters: "+err.Error())
		return
	}
	result, err := h.svc.ListReviewRules(c.Request.Context(), tenantID, filter)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to list review rules: "+err.Error())
		return
	}
	response.OK(c, result)
}

func (h *Handler) GetReviewRule(c *gin.Context) {
	tenantID := getTenantID(c)
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	result, err := h.svc.GetReviewRule(c.Request.Context(), tenantID, id)
	if err != nil {
		response.Fail(c, http.StatusNotFound, "review rule not found")
		return
	}
	response.OK(c, result)
}

func (h *Handler) CreateReviewRule(c *gin.Context) {
	tenantID := getTenantID(c)
	userID := getUserID(c)
	var req CreateReviewRuleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	result, err := h.svc.CreateReviewRule(c.Request.Context(), tenantID, userID, req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to create review rule: "+err.Error())
		return
	}
	response.OK(c, result)
}

func (h *Handler) UpdateReviewRule(c *gin.Context) {
	tenantID := getTenantID(c)
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req UpdateReviewRuleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	result, err := h.svc.UpdateReviewRule(c.Request.Context(), tenantID, id, req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to update review rule: "+err.Error())
		return
	}
	response.OK(c, result)
}

func (h *Handler) ToggleReviewRule(c *gin.Context) {
	tenantID := getTenantID(c)
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	result, err := h.svc.ToggleReviewRule(c.Request.Context(), tenantID, id)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to toggle review rule: "+err.Error())
		return
	}
	response.OK(c, result)
}

// ==================== Scaffold Templates Handlers ====================

func (h *Handler) ListScaffoldTemplates(c *gin.Context) {
	tenantID := getTenantID(c)
	var filter ScaffoldFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid query parameters: "+err.Error())
		return
	}
	result, err := h.svc.ListScaffoldTemplates(c.Request.Context(), tenantID, filter)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to list scaffold templates: "+err.Error())
		return
	}
	response.OK(c, result)
}

func (h *Handler) GetScaffoldTemplate(c *gin.Context) {
	tenantID := getTenantID(c)
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	result, err := h.svc.GetScaffoldTemplate(c.Request.Context(), tenantID, id)
	if err != nil {
		response.Fail(c, http.StatusNotFound, "scaffold template not found")
		return
	}
	response.OK(c, result)
}

// ==================== Effective Specs Handler ====================

func (h *Handler) GetEffectiveSpecs(c *gin.Context) {
	tenantID := getTenantID(c)
	projectID, ok := parseID(c, "projectId")
	if !ok {
		return
	}
	result, err := h.svc.GetEffectiveSpecs(c.Request.Context(), tenantID, projectID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to get effective specs: "+err.Error())
		return
	}
	response.OK(c, result)
}
