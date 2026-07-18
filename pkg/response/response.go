package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Body 统一响应结构体。
type Body struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}

// OK 返回成功响应。
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{
		Code: 0,
		Msg:  "成功",
		Data: data,
	})
}

// OKMsg 返回成功响应并携带自定义消息。
func OKMsg(c *gin.Context, msg string, data any) {
	c.JSON(http.StatusOK, Body{
		Code: 0,
		Msg:  msg,
		Data: data,
	})
}

// Error 返回错误响应。
func Error(c *gin.Context, code int, msg string) {
	c.JSON(code, Body{
		Code: code,
		Msg:  msg,
	})
}

// BadRequest 返回 400 错误。
func BadRequest(c *gin.Context, msg string) {
	Error(c, http.StatusBadRequest, msg)
}

// Unauthorized 返回 401 错误。
func Unauthorized(c *gin.Context, msg string) {
	Error(c, http.StatusUnauthorized, msg)
}
func NotFound(c *gin.Context, msg string) {
	Error(c, http.StatusNotFound, msg)
}

// InternalError 返回 500 错误。
func InternalError(c *gin.Context, msg string) {
	Error(c, http.StatusInternalServerError, msg)
}
