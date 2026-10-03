// Package mail 发送邮件。开发环境用 log 驱动只写日志，生产环境用 SMTP。
package mail

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"log/slog"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"xzji/internal/config"
)

type Message struct {
	To      string
	Subject string
	Text    string
}

type Sender interface {
	Send(ctx context.Context, msg Message) error
}

func New(cfg config.Mail, log *slog.Logger) (Sender, error) {
	switch cfg.Driver {
	case "log":
		return &logSender{log: log}, nil
	case "smtp":
		from, err := mail.ParseAddress(cfg.SMTP.From)
		if err != nil {
			return nil, fmt.Errorf("mail.smtp.from 格式不正确：%w", err)
		}
		return &smtpSender{cfg: cfg.SMTP, from: from}, nil
	default:
		return nil, fmt.Errorf("未知的邮件驱动：%s", cfg.Driver)
	}
}

type logSender struct{ log *slog.Logger }

func (s *logSender) Send(_ context.Context, msg Message) error {
	s.log.Info("邮件（log 驱动，未真实发送）", "to", msg.To, "subject", msg.Subject, "text", msg.Text)
	return nil
}

type smtpSender struct {
	cfg  config.SMTP
	from *mail.Address
}

func (s *smtpSender) Send(ctx context.Context, msg Message) error {
	addr := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	tlsCfg := &tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}

	var conn net.Conn
	var err error
	if s.cfg.ImplicitTLS {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsCfg)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("连接邮件服务器：%w", err)
	}
	deadline := time.Now().Add(20 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)

	c, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("建立 SMTP 会话：%w", err)
	}
	defer c.Close()

	if !s.cfg.ImplicitTLS {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return fmt.Errorf("邮件服务器不支持 STARTTLS")
		}
		if err := c.StartTLS(tlsCfg); err != nil {
			return fmt.Errorf("STARTTLS：%w", err)
		}
	}
	if s.cfg.Username != "" {
		if err := c.Auth(smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)); err != nil {
			return fmt.Errorf("SMTP 认证：%w", err)
		}
	}
	if err := c.Mail(s.from.Address); err != nil {
		return fmt.Errorf("SMTP MAIL FROM：%w", err)
	}
	if err := c.Rcpt(msg.To); err != nil {
		return fmt.Errorf("SMTP RCPT TO：%w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA：%w", err)
	}
	if _, err := w.Write(buildMessage(s.from, msg)); err != nil {
		return fmt.Errorf("写入邮件内容：%w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("提交邮件：%w", err)
	}
	return c.Quit()
}

func buildMessage(from *mail.Address, msg Message) []byte {
	var b strings.Builder
	b.WriteString("From: " + from.String() + "\r\n")
	b.WriteString("To: " + msg.To + "\r\n")
	b.WriteString("Subject: " + mime.BEncoding.Encode("UTF-8", msg.Subject) + "\r\n")
	b.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
	enc := base64.StdEncoding.EncodeToString([]byte(msg.Text))
	for len(enc) > 76 {
		b.WriteString(enc[:76] + "\r\n")
		enc = enc[76:]
	}
	b.WriteString(enc + "\r\n")
	return []byte(b.String())
}
