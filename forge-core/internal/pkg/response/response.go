package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/shulex/forge/forge-core/internal/pkg/errcode"
)

type Result struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Result{Code: 0, Message: "ok", Data: data})
}

func Fail(c *gin.Context, httpStatus int, message string) {
	c.JSON(httpStatus, Result{Code: -1, Message: message})
}

// FailWithError responds with a structured AppError.
func FailWithError(c *gin.Context, err error) {
	var appErr *errcode.AppError
	if errors.As(err, &appErr) {
		c.JSON(appErr.HTTPStatus(), Result{Code: appErr.Code, Message: appErr.Message})
		return
	}
	c.JSON(http.StatusInternalServerError, Result{Code: errcode.InternalError, Message: "内部错误"})
}
