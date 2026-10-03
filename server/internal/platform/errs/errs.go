// Package errs 定义业务模块返回的领域错误。错误码和提示语可以直接给到用户，
// 由接口层按 Kind 映射为 HTTP 状态码，业务模块不依赖 HTTP。
package errs

type Kind int

const (
	KindInvalid Kind = iota + 1
	KindUnauthorized
	KindForbidden
	KindNotFound
	KindConflict
	KindTooMany
	KindUnavailable
)

type Error struct {
	Kind    Kind
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func New(kind Kind, code, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message}
}

func Invalid(code, message string) *Error  { return New(KindInvalid, code, message) }
func TooMany(message string) *Error        { return New(KindTooMany, "TOO_MANY_REQUESTS", message) }
func NotFound(code, message string) *Error { return New(KindNotFound, code, message) }
