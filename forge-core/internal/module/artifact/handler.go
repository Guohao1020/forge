package artifact

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

func (h *Handler) ListArtifacts(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid project id")
		return
	}
	arts, err := h.svc.ListArtifacts(c.Request.Context(), projectID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, ArtifactListResponse{Artifacts: arts})
}

func (h *Handler) GetArtifact(c *gin.Context) {
	artifactID, err := strconv.ParseInt(c.Param("artifactId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid artifact id")
		return
	}
	art, err := h.svc.GetArtifact(c.Request.Context(), artifactID)
	if err != nil {
		response.Fail(c, http.StatusNotFound, err.Error())
		return
	}
	response.OK(c, art)
}
