package pipeline

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

func (h *Handler) ListEnvironments(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid project id")
		return
	}
	envs, err := h.svc.ListEnvironments(c.Request.Context(), projectID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, EnvironmentListResponse{Environments: envs})
}

func (h *Handler) GetEnvironment(c *gin.Context) {
	envID, err := strconv.ParseInt(c.Param("envId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid environment id")
		return
	}
	env, err := h.svc.GetEnvironment(c.Request.Context(), envID)
	if err != nil {
		response.Fail(c, http.StatusNotFound, err.Error())
		return
	}
	response.OK(c, env)
}
