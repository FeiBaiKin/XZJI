// Package user 管理用户账号：创建、凭证校验、资料与密码修改，以及后台的角色和状态管理。
package user

import (
	"database/sql"
	"time"

	"xzji/internal/dbgen"
	"xzji/internal/platform/errs"
)

type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

type Status string

const (
	StatusActive Status = "active"
	StatusBanned Status = "banned"
)

type User struct {
	ID            int64
	Email         string
	Nickname      string
	Role          Role
	Status        Status
	InviterID     *int64
	EmailVerified bool
	LastLoginAt   *time.Time
	CreatedAt     time.Time

	passwordHash string
}

func fromRow(r dbgen.User) User {
	u := User{
		ID:            r.ID,
		Email:         r.Email,
		Nickname:      r.Nickname,
		Role:          Role(r.Role),
		Status:        Status(r.Status),
		EmailVerified: r.EmailVerifiedAt.Valid,
		CreatedAt:     r.CreatedAt,
		passwordHash:  r.PasswordHash,
	}
	if r.InviterID.Valid {
		u.InviterID = &r.InviterID.Int64
	}
	if r.LastLoginAt.Valid {
		u.LastLoginAt = &r.LastLoginAt.Time
	}
	return u
}

func nullInt64(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}

var (
	ErrNotFound           = errs.NotFound("USER_NOT_FOUND", "用户不存在")
	ErrEmailTaken         = errs.New(errs.KindConflict, "EMAIL_TAKEN", "这个邮箱已经注册过了，可以直接登录")
	ErrInvalidEmail       = errs.Invalid("INVALID_EMAIL", "邮箱格式不正确")
	ErrWeakPassword       = errs.Invalid("WEAK_PASSWORD", "密码至少 8 位，且要同时包含字母和数字")
	ErrInvalidNickname    = errs.Invalid("INVALID_NICKNAME", "昵称需要 2 到 20 个字符")
	ErrWrongPassword      = errs.Invalid("WRONG_PASSWORD", "原密码不正确")
	ErrInvalidCredentials = errs.New(errs.KindUnauthorized, "INVALID_CREDENTIALS", "邮箱或密码不正确")
	ErrBanned             = errs.New(errs.KindForbidden, "USER_BANNED", "账号已被封禁，如有疑问请联系客服")
	ErrCannotModifySelf   = errs.Invalid("CANNOT_MODIFY_SELF", "不能修改自己的角色或状态")
	ErrInvalidRole        = errs.Invalid("INVALID_ROLE", "角色不正确")
	ErrInvalidStatus      = errs.Invalid("INVALID_STATUS", "状态不正确")
)
