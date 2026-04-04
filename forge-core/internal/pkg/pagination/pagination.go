package pagination

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// Params holds parsed pagination parameters.
type Params struct {
	Page    int `json:"page"`
	PerPage int `json:"perPage"`
	Offset  int `json:"-"`
}

// Result wraps a paginated response.
type Result struct {
	Items      interface{} `json:"items"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PerPage    int         `json:"perPage"`
	TotalPages int         `json:"totalPages"`
}

// Parse extracts page and perPage from query params with defaults.
// Default: page=1, perPage=20, max perPage=100.
func Parse(c *gin.Context) Params {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	return Params{
		Page:    page,
		PerPage: perPage,
		Offset:  (page - 1) * perPage,
	}
}

// NewResult creates a pagination result with total pages calculated.
func NewResult(items interface{}, total int64, p Params) Result {
	totalPages := int(total) / p.PerPage
	if int(total)%p.PerPage > 0 {
		totalPages++
	}

	return Result{
		Items:      items,
		Total:      total,
		Page:       p.Page,
		PerPage:    p.PerPage,
		TotalPages: totalPages,
	}
}
