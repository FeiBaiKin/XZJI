// Package apierr 定义接口的统一错误响应：{"error": {"code": "...", "message": "..."}}。
package apierr

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"xzji/internal/platform/errs"
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
	Unauthorized     = New(http.StatusUnauthorized, "UNAUTHORIZED", "请先登录")
	CSRFInvalid      = New(http.StatusForbidden, "CSRF_INVALID", "页面已过期，请刷新后重试")
	CSRFOrigin       = New(http.StatusForbidden, "CSRF_ORIGIN", "请求来源不被允许")
)

var kindStatus = map[errs.Kind]int{
	errs.KindInvalid:      http.StatusBadRequest,
	errs.KindUnauthorized: http.StatusUnauthorized,
	errs.KindForbidden:    http.StatusForbidden,
	errs.KindNotFound:     http.StatusNotFound,
	errs.KindConflict:     http.StatusConflict,
	errs.KindTooMany:      http.StatusTooManyRequests,
	errs.KindUnavailable:  http.StatusServiceUnavailable,
}

type body struct {
	Error detail `json:"error"`
}

type detail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Write 输出错误响应。业务模块的领域错误按类别映射状态码；其他错误一律按内部错误处理，
// 原始错误记入请求日志，不暴露给客户端。
func Write(c *gin.Context, err error) {
	var e *Error
	var de *errs.Error
	switch {
	case errors.As(err, &e):
	case errors.As(err, &de):
		status, ok := kindStatus[de.Kind]
		if !ok {
			status = http.StatusBadRequest
		}
		e = New(status, de.Code, de.Message)
	default:
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
