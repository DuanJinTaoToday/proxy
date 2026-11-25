package accountmail

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/mail"
	"net/smtp"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/conf"
)

type sender struct {
	address, host, from, username, password string
	allowPlaintext                          bool
}

func New(settings *conf.Biz) (access.MailSender, error) {
	config := settings.GetEmailAuth()
	if config == nil {
		return nil, nil
	}
	address := config.GetSmtpAddress()
	host, port, err := net.SplitHostPort(address)
	if err != nil || host == "" || port == "" {
		return nil, fmt.Errorf("configure biz.email_auth.smtp_address as host:port")
	}
	from := config.GetFromAddress()
	parsed, err := mail.ParseAddress(from)
	if err != nil || parsed.Address != from {
		return nil, fmt.Errorf("configure biz.email_auth.from_address as a mailbox")
	}
	username, password := config.GetSmtpUsername(), config.GetSmtpPassword()
	if (username == "") != (password == "") || (config.GetAllowPlaintext() && username != "") {
		return nil, fmt.Errorf("configure both SMTP credentials over TLS, or omit both")
	}
	return &sender{address: address, host: host, from: from, username: username, password: password, allowPlaintext: config.GetAllowPlaintext()}, nil
}

func (s *sender) Deliver(ctx context.Context, recipient, subject, body string) error {
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", s.address)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	if startTLS, _ := client.Extension("STARTTLS"); startTLS {
		if err := client.StartTLS(&tls.Config{ServerName: s.host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	} else if !s.allowPlaintext {
		return fmt.Errorf("SMTP relay does not support STARTTLS")
	}
	if s.username != "" {
		if err := client.Auth(smtp.PlainAuth("", s.username, s.password, s.host)); err != nil {
			return err
		}
	}
	if err := client.Mail(s.from); err != nil {
		return err
	}
	if err := client.Rcpt(recipient); err != nil {
		return err
	}
	message, err := client.Data()
	if err != nil {
		return err
	}
	_, err = io.WriteString(message, "Date: "+time.Now().UTC().Format(time.RFC1123Z)+"\r\nFrom: "+s.from+"\r\nTo: "+recipient+"\r\nSubject: "+subject+
		"\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n"+body+"\r\n")
	if err != nil {
		return err
	}
	return message.Close()
}
