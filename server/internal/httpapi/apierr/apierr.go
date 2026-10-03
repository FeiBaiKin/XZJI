// Package apierr 定义接口的统一错误响应：{"error": {"code": "...", "message": "..."}}。
package apierr

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Error 是可以直接返回给客户端的业务错误。
type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func New(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

var (
	BadRequest       = New(http.StatusBadRequest, "BAD_REQUEST", "请求参数不正确")
	NotFound         = New(http.StatusNotFound, "NOT_FOUND", "请求的地址不存在")
	MethodNotAllowed = New(http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "不支持该请求方法")
	Internal         = New(http.StatusInternalServerError, "INTERNAL_ERROR", "服务器内部错误，请稍后重试")
)

type body struct {
	Error detail `json:"error"`
}

type detail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Write 输出错误响应。非 *Error 的错误一律按内部错误处理，原始错误记入请求日志，不暴露给客户端。
func Write(c *gin.Context, err error) {
	var e *Error
	if !errors.As(err, &e) {
		_ = c.Error(err)
		e = Internal
	}
	c.AbortWithStatusJSON(e.Status, body{Error: detail{Code: e.Code, Message: e.Message}})
}

// WriteBadRequest 输出参数错误，解析失败的细节只记入日志。
func WriteBadRequest(c *gin.Context, err error) {
	_ = c.Error(err)
	Write(c, BadRequest)
}
