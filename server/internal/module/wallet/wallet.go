// Package wallet 管理用户余额。余额只能通过写流水变动：同一事务内先写 wallet_ledger，再改 wallets.balance。
package wallet

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	sq "github.com/Masterminds/squirrel"

	"xzji/internal/dbgen"
	platformdb "xzji/internal/platform/db"
	"xzji/internal/platform/errs"
)

const AccountBalance = "balance"

// 流水的业务类型
const (
	BizRecharge    = "recharge"
	BizPurchase    = "purchase"
	BizRefund      = "refund"
	BizAdminAdjust = "admin_adjust"
)

// MaxBalance 单个钱包的余额上限（分），防止误操作和溢出
const MaxBalance int64 = 100_000_000_00

var (
	ErrInsufficient   = errs.New(errs.KindConflict, "INSUFFICIENT_BALANCE", "余额不足")
	ErrBalanceLimit   = errs.Invalid("BALANCE_LIMIT", "余额超出上限")
	ErrZeroAmount     = errs.Invalid("INVALID_AMOUNT", "变动金额不能为 0")
	ErrRemarkRequired = errs.Invalid("REMARK_REQUIRED", "请填写调整原因（1 到 100 个字）")
)

// Apply 计算一笔变动后的余额：出账后不能为负，入账后不能超过上限。
func Apply(balance, amount int64) (int64, error) {
	if amount == 0 {
		return balance, ErrZeroAmount
	}
	if amount < 0 && balance+amount < 0 {
		return balance, ErrInsufficient
	}
	if amount > 0 && amount > MaxBalance-balance {
		return balance, ErrBalanceLimit
	}
	return balance + amount, nil
}

// Change 一笔余额变动。Amount 正数入账、负数出账。
type Change struct {
	UserID  int64
	Amount  int64
	BizType string
	BizID   int64
	Remark  string
}

type Entry struct {
	ID           int64
	UserID       int64
	Amount       int64
	BalanceAfter int64
	BizType      string
	BizID        int64
	Remark       string
	CreatedAt    time.Time
}

type EntryPage struct {
	Entries  []Entry
	Total    int64
	Page     int
	PageSize int
}

// Auditor 记录后台操作日志。
type Auditor interface {
	Record(ctx context.Context, adminID int64, action, targetType string, targetID int64, detail any, ip string) error
}

// UserChecker 确认用户存在。
type UserChecker interface {
	Exists(ctx context.Context, id int64) error
}

type Service struct {
	db    *sql.DB
	q     *dbgen.Queries
	users UserChecker
	audit Auditor
}

func NewService(db *sql.DB, users UserChecker, audit Auditor) *Service {
	return &Service{db: db, q: dbgen.New(db), users: users, audit: audit}
}

// Balance 当前余额，没有钱包时为 0。
func (s *Service) Balance(ctx context.Context, userID int64) (int64, error) {
	b, err := s.q.GetWalletBalance(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("查询余额：%w", err)
	}
	return b, nil
}

// Balances 批量查询余额，没有钱包的用户不在结果里。
func (s *Service) Balances(ctx context.Context, ids []int64) (map[int64]int64, error) {
	out := make(map[int64]int64, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.q.ListWalletBalances(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("批量查询余额：%w", err)
	}
	for _, r := range rows {
		out[r.UserID] = r.Balance
	}
	return out, nil
}

// ChangeTx 在调用方的事务里变动余额：锁定钱包行、校验、先写流水再改余额。返回变动后的余额。
func (s *Service) ChangeTx(ctx context.Context, tx *sql.Tx, c Change) (int64, error) {
	q := s.q.WithTx(tx)
	if err := q.EnsureWallet(ctx, c.UserID); err != nil {
		return 0, fmt.Errorf("创建钱包：%w", err)
	}
	balance, err := q.GetWalletForUpdate(ctx, c.UserID)
	if err != nil {
		return 0, fmt.Errorf("锁定钱包：%w", err)
	}
	after, err := Apply(balance, c.Amount)
	if err != nil {
		return balance, err
	}
	if err := q.CreateLedgerEntry(ctx, dbgen.CreateLedgerEntryParams{
		UserID: c.UserID, Account: AccountBalance, Amount: c.Amount, BalanceAfter: after,
		BizType: c.BizType, BizID: c.BizID, Remark: truncate(c.Remark, 255),
	}); err != nil {
		return 0, fmt.Errorf("写余额流水：%w", err)
	}
	if err := q.SetWalletBalance(ctx, dbgen.SetWalletBalanceParams{Balance: after, UserID: c.UserID}); err != nil {
		return 0, fmt.Errorf("更新余额：%w", err)
	}
	return after, nil
}

// Adjust 后台手动调整余额，必须填写原因并记录操作日志。
func (s *Service) Adjust(ctx context.Context, adminID int64, ip string, userID, amount int64, remark string) (int64, error) {
	remark = strings.TrimSpace(remark)
	if n := utf8.RuneCountInString(remark); n == 0 || n > 100 {
		return 0, ErrRemarkRequired
	}
	if err := s.users.Exists(ctx, userID); err != nil {
		return 0, err
	}
	var after int64
	err := platformdb.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		var err error
		after, err = s.ChangeTx(ctx, tx, Change{UserID: userID, Amount: amount, BizType: BizAdminAdjust, BizID: adminID, Remark: remark})
		return err
	})
	if err != nil {
		return 0, err
	}
	detail := map[string]any{"amount": amount, "balance_after": after, "remark": remark}
	if err := s.audit.Record(ctx, adminID, "wallet.adjust", "user", userID, detail, ip); err != nil {
		return 0, err
	}
	return after, nil
}

func normPage(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	return page, size
}

// Ledger 用户的余额流水，新的在前。
func (s *Service) Ledger(ctx context.Context, userID int64, page, size int) (EntryPage, error) {
	page, size = normPage(page, size)
	rows, err := s.q.ListUserLedger(ctx, dbgen.ListUserLedgerParams{UserID: userID, Account: AccountBalance, Limit: int32(size), Offset: int32((page - 1) * size)})
	if err != nil {
		return EntryPage{}, fmt.Errorf("查询余额流水：%w", err)
	}
	total, err := s.q.CountUserLedger(ctx, dbgen.CountUserLedgerParams{UserID: userID, Account: AccountBalance})
	if err != nil {
		return EntryPage{}, fmt.Errorf("统计余额流水：%w", err)
	}
	out := EntryPage{Entries: make([]Entry, len(rows)), Total: total, Page: page, PageSize: size}
	for i, r := range rows {
		out.Entries[i] = toEntry(r)
	}
	return out, nil
}

func toEntry(r dbgen.WalletLedger) Entry {
	return Entry{
		ID: r.ID, UserID: r.UserID, Amount: r.Amount, BalanceAfter: r.BalanceAfter,
		BizType: r.BizType, BizID: r.BizID, Remark: r.Remark, CreatedAt: r.CreatedAt,
	}
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// LedgerFilter 后台流水筛选，零值表示不限。
type LedgerFilter struct {
	UserID   int64
	BizType  string
	Page     int
	PageSize int
}

// AllLedger 后台查看全站流水。
func (s *Service) AllLedger(ctx context.Context, f LedgerFilter) (EntryPage, error) {
	page, size := normPage(f.Page, f.PageSize)
	w := sq.And{sq.Eq{"account": AccountBalance}}
	if f.UserID > 0 {
		w = append(w, sq.Eq{"user_id": f.UserID})
	}
	if f.BizType != "" {
		w = append(w, sq.Eq{"biz_type": f.BizType})
	}
	query, args, err := sq.Select("id", "user_id", "amount", "balance_after", "biz_type", "biz_id", "remark", "created_at").
		From("wallet_ledger").Where(w).OrderBy("id DESC").Limit(uint64(size)).Offset(uint64((page - 1) * size)).ToSql()
	if err != nil {
		return EntryPage{}, err
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return EntryPage{}, fmt.Errorf("查询余额流水：%w", err)
	}
	defer rows.Close()
	out := EntryPage{Entries: []Entry{}, Page: page, PageSize: size}
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.UserID, &e.Amount, &e.BalanceAfter, &e.BizType, &e.BizID, &e.Remark, &e.CreatedAt); err != nil {
			return EntryPage{}, fmt.Errorf("读取余额流水：%w", err)
		}
		out.Entries = append(out.Entries, e)
	}
	if err := rows.Err(); err != nil {
		return EntryPage{}, fmt.Errorf("读取余额流水：%w", err)
	}
	countSQL, countArgs, err := sq.Select("COUNT(*)").From("wallet_ledger").Where(w).ToSql()
	if err != nil {
		return EntryPage{}, err
	}
	if err := s.db.QueryRowContext(ctx, countSQL, countArgs...).Scan(&out.Total); err != nil {
		return EntryPage{}, fmt.Errorf("统计余额流水：%w", err)
	}
	return out, nil
}
