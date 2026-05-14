package mailer

import (
	"fmt"
	"net/smtp"
	"strings"
)

type Mailer struct {
	host     string
	port     string
	user     string
	password string
	from     string
	fromName string
}

func New(host, port, user, password, from, fromName string) *Mailer {
	return &Mailer{
		host:     host,
		port:     port,
		user:     user,
		password: password,
		from:     from,
		fromName: fromName,
	}
}

func (m *Mailer) SendOTP(to, code string) error {
	subject := "Your ADLTS verification code"
	body := fmt.Sprintf(
		"Your verification code is: %s\n\nThis code expires in 5 minutes.\n\nDo not share this code with anyone.",
		code,
	)
	return m.send(to, subject, body)
}

func (m *Mailer) SendPasswordReset(to, resetLink string) error {
	subject := "Reset your ADLTS password"
	body := fmt.Sprintf(
		"You requested a password reset.\n\nClick the link below to set a new password:\n%s\n\nThis link expires in 15 minutes.\n\nIf you did not request this, ignore this email.",
		resetLink,
	)
	return m.send(to, subject, body)
}

func (m *Mailer) SendInvitation(to, role, inviteLink string) error {
	subject := "You've been invited to ADLTS"
	body := fmt.Sprintf(
		"You have been invited to join ADLTS as %s.\n\nClick the link below to set up your account:\n%s\n\nThis invitation expires in 72 hours.",
		role, inviteLink,
	)
	return m.send(to, subject, body)
}

func (m *Mailer) send(to, subject, body string) error {
	if m.user == "" || m.password == "" || m.from == "" {
		// SMTP not configured — log and skip (dev mode)
		fmt.Printf("[mailer] SKIP (not configured) to=%s subject=%q\n", to, subject)
		return nil
	}

	auth := smtp.PlainAuth("", m.user, m.password, m.host)

	fromHeader := fmt.Sprintf("%s <%s>", m.fromName, m.from)
	msg := strings.Join([]string{
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=\"UTF-8\"",
		"From: " + fromHeader,
		"To: " + to,
		"Subject: " + subject,
		"",
		body,
	}, "\r\n")

	addr := m.host + ":" + m.port
	if err := smtp.SendMail(addr, auth, m.from, []string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("smtp send to %s: %w", to, err)
	}
	return nil
}
