package user

import (
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"
)

// NormalizeEmail 去掉首尾空白并转为小写，格式不合法时返回 ErrInvalidEmail。
func NormalizeEmail(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if len(s) < 3 || len(s) > 191 {
		return "", ErrInvalidEmail
	}
	a, err := mail.ParseAddress(s)
	if err != nil || a.Name != "" || a.Address != s || !strings.Contains(s[strings.LastIndexByte(s, '@')+1:], ".") {
		return "", ErrInvalidEmail
	}
	return s, nil
}

// ValidatePassword 要求 8 到 128 位，且同时包含字母和数字。
func ValidatePassword(p string) error {
	n := utf8.RuneCountInString(p)
	if n < 8 || n > 128 {
		return ErrWeakPassword
	}
	var letter, digit bool
	for _, r := range p {
		switch {
		case unicode.IsLetter(r):
			letter = true
		case unicode.IsDigit(r):
			digit = true
		}
	}
	if !letter || !digit {
		return ErrWeakPassword
	}
	return nil
}

// NormalizeNickname 去掉首尾空白，要求 2 到 20 个字符且不含控制字符。
func NormalizeNickname(s string) (string, error) {
	s = strings.TrimSpace(s)
	n := utf8.RuneCountInString(s)
	if n < 2 || n > 20 {
		return "", ErrInvalidNickname
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return "", ErrInvalidNickname
		}
	}
	return s, nil
}

// DefaultNickname 取邮箱 @ 前面的部分，超长截断，过短补前缀。
func DefaultNickname(email string) string {
	local := email
	if i := strings.IndexByte(email, '@'); i >= 0 {
		local = email[:i]
	}
	r := []rune(local)
	if len(r) > 20 {
		r = r[:20]
	}
	if len(r) < 2 {
		return "用户" + string(r)
	}
	return string(r)
}
