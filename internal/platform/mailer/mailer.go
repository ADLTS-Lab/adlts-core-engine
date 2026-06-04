package mailer

import (
	"crypto/tls"
	"errors"
	"fmt"
	"html"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

var ErrNotConfigured = errors.New("smtp mailer is not configured")

type Encryption string

const (
	EncryptionAuto     Encryption = "auto"
	EncryptionStartTLS Encryption = "starttls"
	EncryptionTLS      Encryption = "tls"
	EncryptionNone     Encryption = "none"
)

type Config struct {
	Host       string
	Port       string
	User       string
	Password   string
	From       string
	FromName   string
	Encryption Encryption
	Timeout    time.Duration
}

type sender interface {
	Send(from string, recipients []string, msg []byte) error
}

type Mailer struct {
	cfg    Config
	sender sender
}

func New(host, port, user, password, from, fromName string) *Mailer {
	return NewSMTP(Config{
		Host:       host,
		Port:       port,
		User:       user,
		Password:   password,
		From:       from,
		FromName:   fromName,
		Encryption: EncryptionAuto,
		Timeout:    10 * time.Second,
	})
}

func NewSMTP(cfg Config) *Mailer {
	cfg = normalizeConfig(cfg)
	return &Mailer{
		cfg:    cfg,
		sender: smtpSender{cfg: cfg},
	}
}

func NewDiscard() *Mailer {
	return &Mailer{
		cfg: Config{
			From:     "noreply@adlts.et",
			FromName: "ADLTS",
		},
		sender: discardSender{},
	}
}

func (m *Mailer) SendOTP(to, code string) error {
	subject := "Your ADLTS verification code"
	text := fmt.Sprintf(
		"Your verification code is: %s\n\nThis code expires in 5 minutes.\n\nDo not share this code with anyone.",
		code,
	)
	htmlBody := fmt.Sprintf(
		`<p>Your verification code is:</p><p style="font-size:24px;font-weight:700;letter-spacing:4px">%s</p><p>This code expires in 5 minutes.</p><p>Do not share this code with anyone.</p>`,
		html.EscapeString(code),
	)
	return m.send(to, subject, text, htmlBody)
}

func (m *Mailer) SendPasswordReset(to, resetLink string) error {
	subject := "Reset your ADLTS password"
	text := fmt.Sprintf(
		"You requested a password reset.\n\nOpen the link below to set a new password:\n%s\n\nThis link expires in 15 minutes.\n\nIf you did not request this, ignore this email.",
		resetLink,
	)
	htmlBody := fmt.Sprintf(
		`<p>You requested a password reset.</p><p><a href="%s">Reset your password</a></p><p>This link expires in 15 minutes.</p><p>If you did not request this, ignore this email.</p>`,
		html.EscapeString(resetLink),
	)
	return m.send(to, subject, text, htmlBody)
}

func (m *Mailer) SendInvitation(to, role, inviteLink string) error {
	subject := "You've been invited to ADLTS"
	text := fmt.Sprintf(
		"You have been invited to join ADLTS as %s.\n\nOpen the link below to set up your account:\n%s\n\nThis invitation expires in 72 hours.",
		role, inviteLink,
	)
	htmlBody := fmt.Sprintf(
		`<p>You have been invited to join ADLTS as <strong>%s</strong>.</p><p><a href="%s">Accept invitation</a></p><p>This invitation expires in 72 hours.</p>`,
		html.EscapeString(role),
		html.EscapeString(inviteLink),
	)
	return m.send(to, subject, text, htmlBody)
}

func (m *Mailer) send(to, subject, textBody, htmlBody string) error {
	if m == nil || m.sender == nil {
		return ErrNotConfigured
	}
	if err := m.validate(); err != nil {
		return err
	}

	recipient, err := mail.ParseAddress(strings.TrimSpace(to))
	if err != nil {
		return fmt.Errorf("invalid recipient email %q: %w", to, err)
	}
	fromAddr, err := mail.ParseAddress(strings.TrimSpace(m.cfg.From))
	if err != nil {
		return fmt.Errorf("invalid sender email %q: %w", m.cfg.From, err)
	}
	fromAddr.Name = strings.TrimSpace(m.cfg.FromName)

	msg := buildMessage(fromAddr.String(), recipient.String(), subject, textBody, htmlBody)
	if err := m.sender.Send(fromAddr.Address, []string{recipient.Address}, []byte(msg)); err != nil {
		return fmt.Errorf("send email to %s: %w", recipient.Address, err)
	}
	return nil
}

func (m *Mailer) validate() error {
	if _, ok := m.sender.(discardSender); ok {
		return nil
	}
	if strings.TrimSpace(m.cfg.Host) == "" || strings.TrimSpace(m.cfg.Port) == "" || strings.TrimSpace(m.cfg.From) == "" {
		return ErrNotConfigured
	}
	switch m.cfg.Encryption {
	case EncryptionAuto, EncryptionStartTLS, EncryptionTLS, EncryptionNone:
	default:
		return fmt.Errorf("unsupported smtp encryption mode %q", m.cfg.Encryption)
	}
	return nil
}

func buildMessage(from, to, subject, textBody, htmlBody string) string {
	boundary := fmt.Sprintf("adlts-%d", time.Now().UnixNano())
	headers := []string{
		"MIME-Version: 1.0",
		"Date: " + time.Now().Format(time.RFC1123Z),
		"From: " + from,
		"To: " + to,
		"Subject: " + mime.QEncoding.Encode("UTF-8", sanitizeHeader(subject)),
		`Content-Type: multipart/alternative; boundary="` + boundary + `"`,
	}

	var b strings.Builder
	b.WriteString(strings.Join(headers, "\r\n"))
	b.WriteString("\r\n\r\n")
	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(strings.TrimSpace(textBody))
	b.WriteString("\r\n\r\n")
	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(strings.TrimSpace(htmlBody))
	b.WriteString("\r\n\r\n")
	b.WriteString("--" + boundary + "--\r\n")
	return b.String()
}

func sanitizeHeader(v string) string {
	v = strings.ReplaceAll(v, "\r", " ")
	v = strings.ReplaceAll(v, "\n", " ")
	return strings.TrimSpace(v)
}

func normalizeConfig(cfg Config) Config {
	cfg.Host = strings.TrimSpace(cfg.Host)
	cfg.Port = strings.TrimSpace(cfg.Port)
	cfg.User = strings.TrimSpace(cfg.User)
	cfg.From = strings.TrimSpace(cfg.From)
	cfg.FromName = strings.TrimSpace(cfg.FromName)
	if cfg.Port == "" {
		cfg.Port = "587"
	}
	if cfg.FromName == "" {
		cfg.FromName = "ADLTS"
	}
	cfg.Encryption = Encryption(strings.ToLower(strings.TrimSpace(string(cfg.Encryption))))
	if cfg.Encryption == "" {
		cfg.Encryption = EncryptionAuto
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	return cfg
}

type smtpSender struct {
	cfg Config
}

func (s smtpSender) Send(from string, recipients []string, msg []byte) error {
	addr := net.JoinHostPort(s.cfg.Host, s.cfg.Port)
	conn, err := s.dial(addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(s.cfg.Timeout))

	client, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		return err
	}
	defer client.Close()

	if err := s.startTLS(client); err != nil {
		return err
	}
	if s.cfg.User != "" {
		if err := client.Auth(smtp.PlainAuth("", s.cfg.User, s.cfg.Password, s.cfg.Host)); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("smtp rcpt %s: %w", recipient, err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		_ = w.Close()
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close data: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("smtp quit: %w", err)
	}
	return nil
}

func (s smtpSender) dial(addr string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: s.cfg.Timeout}
	if s.cfg.Encryption == EncryptionTLS {
		return tls.DialWithDialer(dialer, "tcp", addr, s.tlsConfig())
	}
	return dialer.Dial("tcp", addr)
}

func (s smtpSender) startTLS(client *smtp.Client) error {
	if s.cfg.Encryption != EncryptionAuto && s.cfg.Encryption != EncryptionStartTLS {
		return nil
	}
	ok, _ := client.Extension("STARTTLS")
	if !ok {
		if s.cfg.Encryption == EncryptionStartTLS {
			return errors.New("smtp server does not advertise STARTTLS")
		}
		return nil
	}
	if err := client.StartTLS(s.tlsConfig()); err != nil {
		return fmt.Errorf("smtp starttls: %w", err)
	}
	return nil
}

func (s smtpSender) tlsConfig() *tls.Config {
	return &tls.Config{
		ServerName: s.cfg.Host,
		MinVersion: tls.VersionTLS12,
	}
}

type discardSender struct{}

func (discardSender) Send(_ string, _ []string, _ []byte) error {
	return nil
}
