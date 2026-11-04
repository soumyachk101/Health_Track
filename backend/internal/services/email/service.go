// Package email ports createTransporter and the three email helpers from
// routes/auth.js using net/smtp + crypto/tls only.
package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"

	"healthtrack/internal/config"
)

var bar = strings.Repeat("=", 60)

type Service struct {
	cfg *config.Config
}

func New(cfg *config.Config) *Service {
	return &Service{cfg: cfg}
}

// smtpClient connects to the SMTP server. Mirrors the nodemailer settings in
// routes/auth.js: EMAIL_USE_TLS=true forces plain SMTP (no STARTTLS); port
// 465 dials over TLS directly; anything else (587) connects in cleartext
// and upgrades via STARTTLS.
func (s *Service) smtpClient() (*smtp.Client, error) {
	// Bounds the whole SMTP exchange so a stalled relay can't wedge a
	// goroutine (and a pool slot) forever.
	d := &net.Dialer{Timeout: 15 * time.Second}
	host := s.cfg.EmailHost
	if host == "" {
		host = "smtp.gmail.com"
	}
	port := s.cfg.EmailPort
	if port == 0 {
		port = 587
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	var c *smtp.Client
	var err error
	switch {
	case s.cfg.EmailUseTLS:
		// Plain SMTP, no STARTTLS (nodemailer secure: false).
		conn, dErr := d.Dial("tcp", addr)
		if dErr != nil {
			return nil, dErr
		}
		c, err = smtp.NewClient(conn, host)
	case port == 465:
		conn, tlsErr := tls.DialWithDialer(d, "tcp", addr, &tls.Config{ServerName: host})
		if tlsErr != nil {
			return nil, tlsErr
		}
		c, err = smtp.NewClient(conn, host)
	default:
		conn, dErr := d.Dial("tcp", addr)
		if dErr != nil {
			return nil, dErr
		}
		c, err = smtp.NewClient(conn, host)
		if err == nil {
			if sErr := c.StartTLS(&tls.Config{ServerName: host}); sErr != nil {
				return nil, sErr
			}
		}
	}
	if err != nil {
		return nil, err
	}
	if s.cfg.EmailUser != "" && s.cfg.EmailPass != "" {
		auth := smtp.PlainAuth("", s.cfg.EmailUser, s.cfg.EmailPass, host)
		if aErr := c.Auth(auth); aErr != nil {
			return nil, aErr
		}
	}
	return c, nil
}

func (s *Service) from() string {
	if s.cfg.FromEmail != "" {
		return s.cfg.FromEmail
	}
	return fmt.Sprintf(`"HealthTrack+ (No-Reply)" <%s>`, s.cfg.EmailUser)
}

// sendMail builds and sends a MIME message. When html is non-empty the body
// is a multipart/alternative of text/plain + text/html parts.
func (s *Service) sendMail(to, subject, text, html string) error {
	c, err := s.smtpClient()
	if err != nil {
		return err
	}
	defer c.Close()

	if err := c.Mail(s.cfg.EmailUser); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}

	var b strings.Builder

	b.WriteString(fmt.Sprintf("From: %s\r\n", s.from()))
	b.WriteString(fmt.Sprintf("To: %s\r\n", to))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))

	if html != "" {
		boundary := "00000000healthtrackplus"
		b.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=%q\r\n\r\n", boundary))
		b.WriteString(fmt.Sprintf("--%s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n", boundary, text))
		b.WriteString(fmt.Sprintf("--%s\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s\r\n", boundary, html))
		b.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
	} else {
		b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
		b.WriteString(text)
	}

	if _, err := w.Write([]byte(b.String())); err != nil {
		return err
	}
	return w.Close()
}

// SendOtpEmail ports sendOtpEmail from routes/auth.js. Returns true when the
// email was sent.
func (s *Service) SendOtpEmail(ctx context.Context, to, otp, firstName string) bool {
	if s.cfg.EmailUser == "" {
		log.Printf("email: EMAIL_HOST_USER not configured; OTP for %s not sent", to)
		return false
	}
	name := firstName
	if name == "" {
		name = "there"
	}
	if s.devFallback() {
		// Development-only: print the code so tests can read it without a
		// relay. Gated so production logs never contain OTP values.
		fmt.Fprintf(os.Stdout, "--- [DEVELOPMENT ONLY] OTP for %s: %s ---\n", to, otp)
	}

	subject := "Your HealthTrack+ Verification Code"
	text := fmt.Sprintf("Hi %s,\n\nYour verification code for HealthTrack+ is:\n\n    %s\n\nThis code will expire in 10 minutes.\n\nIf you did not request this code, please ignore this email.\n\nBest regards,\nThe HealthTrack+ Team", name, otp)

	if err := s.sendMail(to, subject, text, ""); err != nil {
		fmt.Fprintf(os.Stdout, "Error sending OTP email: %v\n", err)
		return false
	}
	fmt.Fprintf(os.Stdout, "Email sent successfully to %s\n", to)
	return true
}

// devFallback returns true iff we are not in production, mirroring the JS
// "local dev proceeds even when email fails" semantics.
func (s *Service) devFallback() bool {
	return s.cfg.NodeEnv != "production"
}

// SendResetEmail ports sendResetEmail from routes/auth.js.
func (s *Service) SendResetEmail(ctx context.Context, to, resetLink, firstName string) bool {
	name := firstName
	if name == "" {
		name = "there"
	}
	if s.devFallback() {
		// Development-only banner containing the link; gated so production
		// logs never carry reset codes.
		fmt.Fprintln(os.Stdout, "\n"+bar)
		fmt.Fprintf(os.Stdout, "🔑 [DEVELOPMENT] PASSWORD RESET LINK\n")
		fmt.Fprintf(os.Stdout, "📧 TO: %s\n", to)
		fmt.Fprintf(os.Stdout, "🔗 LINK: %s\n", resetLink)
		fmt.Fprintln(os.Stdout, bar+"\n")
	}

	if s.cfg.EmailUser == "" {
		log.Printf("email: EMAIL_HOST_USER not configured; reset email to %s not sent", to)
		return s.devFallback()
	}

	subject := "Reset Your HealthTrack+ Password"
	text := fmt.Sprintf("Hi %s,\n\nClick the link below to reset your password. This link expires in 1 hour.\n\n%s\n\nIf you did not request a password reset, ignore this email.\n\nHealthTrack+ Team", name, resetLink)
	html := `<div style="font-family:sans-serif;max-width:480px;margin:auto">
    <h2 style="color:#0d9488">Reset Your Password</h2>
    <p>Hi ` + name + `,</p>
    <p>Click the button below to reset your HealthTrack+ password. This link expires in <strong>1 hour</strong>.</p>
    <a href="` + resetLink + `" style="display:inline-block;margin:16px 0;padding:12px 24px;background:#0d9488;color:#fff;border-radius:8px;text-decoration:none;font-weight:bold">Reset Password</a>
    <p style="color:#64748b;font-size:12px">If the button doesn't work, copy this link:<br/><a href="` + resetLink + `">` + resetLink + `</a></p>
    <p style="color:#64748b;font-size:12px">If you didn't request this, ignore this email.</p>
  </div>`

	if err := s.sendMail(to, subject, text, html); err != nil {
		fmt.Fprintf(os.Stdout, "Error sending reset email: %v\n", err)
		return s.devFallback()
	}
	fmt.Fprintf(os.Stdout, "Reset email sent to %s\n", to)
	return true
}

// SendVerificationLinkEmail ports sendVerificationLinkEmail from routes/auth.js.
func (s *Service) SendVerificationLinkEmail(ctx context.Context, to, verificationLink, firstName string) bool {
	name := firstName
	if name == "" {
		name = "there"
	}
	if s.devFallback() {
		// Development-only banner containing the link; gated so production
		// logs never carry verification codes.
		fmt.Fprintln(os.Stdout, "\n"+bar)
		fmt.Fprintf(os.Stdout, "🚀 [DEVELOPMENT] EMAIL VERIFICATION LINK\n")
		fmt.Fprintf(os.Stdout, "📧 TO: %s\n", to)
		fmt.Fprintf(os.Stdout, "🔗 LINK: %s\n", verificationLink)
		fmt.Fprintln(os.Stdout, bar+"\n")
	}

	if s.cfg.EmailUser == "" {
		log.Printf("email: EMAIL_HOST_USER not configured; verification email to %s not sent", to)
		// sendVerificationLinkEmail returns false here (unlike sendResetEmail,
		// which dev-fallbacks), matching the JS exactly.
		return false
	}

	subject := "Verify Your HealthTrack+ Email"
	text := fmt.Sprintf("Hi %s,\n\nClick the link below to verify your email and complete your registration. This link expires in 10 minutes.\n\n%s\n\nIf you did not create an account, ignore this email.\n\nHealthTrack+ Team", name, verificationLink)
	html := `<div style="font-family:sans-serif;max-width:480px;margin:auto">
    <h2 style="color:#0d9488">Verify Your Email</h2>
    <p>Hi ` + name + `,</p>
    <p>Click the button below to verify your email and activate your HealthTrack+ account. This link expires in <strong>10 minutes</strong>.</p>
    <a href="` + verificationLink + `" style="display:inline-block;margin:16px 0;padding:12px 24px;background:#0d9488;color:#fff;border-radius:8px;text-decoration:none;font-weight:bold">Verify Email</a>
    <p style="color:#64748b;font-size:12px">If the button doesn't work, copy this link:<br/><a href="` + verificationLink + `">` + verificationLink + `</a></p>
    <p style="color:#64748b;font-size:12px">If you didn't request this, ignore this email.</p>
  </div>`

	if err := s.sendMail(to, subject, text, html); err != nil {
		fmt.Fprintf(os.Stdout, "Error sending verification email: %v\n", err)
		return s.devFallback()
	}
	fmt.Fprintf(os.Stdout, "Verification email sent to %s\n", to)
	return true
}
