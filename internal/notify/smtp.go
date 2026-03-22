// Copyright 2026 Quest Financial Technologies S.à r.l.-S., Luxembourg
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.


package notify

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// SMTPConfig holds configuration for minimal SMTP notifications.
// This is intentionally separate from the app's email package -- the proxy
// needs to send alerts even when the app server is down.
type SMTPConfig struct {
	Host     string   `yaml:"host"`
	Port     int      `yaml:"port"`
	UseSSL   bool     `yaml:"use-ssl"`
	Username string   `yaml:"username"`
	Password string   `yaml:"password"`
	From     string   `yaml:"from"`
	FromName string   `yaml:"from-name"`
	To       []string `yaml:"to"`
}

// SMTPSender sends plain text email notifications.
type SMTPSender struct {
	cfg    SMTPConfig
	logger *slog.Logger
}

// newSMTPSender creates an SMTPSender with the given configuration.
func newSMTPSender(logger *slog.Logger, cfg SMTPConfig) *SMTPSender {
	return &SMTPSender{cfg: cfg, logger: logger}
}

// Send delivers a plain text email to all configured recipients.
// Supports implicit TLS (port 465) and STARTTLS (port 587).
func (s *SMTPSender) Send(subject, body string) error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	msg := s.buildMessage(subject, body)

	client, err := s.connect(addr)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	// Authenticate
	if s.cfg.Username != "" {
		auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth: %w", err)
		}
	}

	// Send envelope
	if err := client.Mail(s.cfg.From); err != nil {
		return fmt.Errorf("SMTP MAIL: %w", err)
	}
	for _, to := range s.cfg.To {
		if err := client.Rcpt(to); err != nil {
			return fmt.Errorf("SMTP RCPT %s: %w", to, err)
		}
	}

	// Write message
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA: %w", err)
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		_ = w.Close()
		return fmt.Errorf("SMTP write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("SMTP close data: %w", err)
	}

	_ = client.Quit()

	s.logger.Debug("SMTP notification sent",
		"to", strings.Join(s.cfg.To, ","), "subject", subject)
	return nil
}

// connect establishes an SMTP connection. Uses implicit TLS for UseSSL=true,
// otherwise tries STARTTLS if the server supports it.
func (s *SMTPSender) connect(addr string) (*smtp.Client, error) {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	tlsConfig := &tls.Config{ServerName: s.cfg.Host}

	if s.cfg.UseSSL {
		// Implicit TLS (typically port 465)
		conn, err := tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
		if err != nil {
			return nil, fmt.Errorf("TLS dial %s: %w", addr, err)
		}
		client, err := smtp.NewClient(conn, s.cfg.Host)
		if err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("SMTP client on TLS: %w", err)
		}
		return client, nil
	}

	// Plain connection, try STARTTLS if available
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	client, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("SMTP client: %w", err)
	}
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(tlsConfig); err != nil {
			_ = client.Close()
			return nil, fmt.Errorf("STARTTLS: %w", err)
		}
	}
	return client, nil
}

// buildMessage constructs an RFC 2822 email message.
func (s *SMTPSender) buildMessage(subject, body string) string {
	from := s.cfg.From
	if s.cfg.FromName != "" {
		from = fmt.Sprintf("%s <%s>", s.cfg.FromName, s.cfg.From)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(s.cfg.To, ", "))
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z))
	fmt.Fprintf(&b, "Content-Type: text/plain; charset=utf-8\r\n")
	fmt.Fprintf(&b, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&b, "\r\n")
	fmt.Fprintf(&b, "%s\r\n", body)
	return b.String()
}
