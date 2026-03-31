package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
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
