// Copyright 2026 Quest Financial Technologies S.a r.l.-S., Luxembourg
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


package email

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
	"time"
)

// SMTPClient handles sending emails via SMTP.
type SMTPClient struct {
	config *Config
}

// NewSMTPClient creates a new SMTP client.
func NewSMTPClient(config *Config) (*SMTPClient, error) {
	if err := config.ValidateSMTP(); err != nil {
		return nil, fmt.Errorf("invalid SMTP config: %w", err)
	}
	return &SMTPClient{config: config}, nil
}

// Send sends an email message.
func (c *SMTPClient) Send(msg *OutgoingMessage) error {
	if len(msg.To) == 0 {
		return fmt.Errorf("no recipients specified")
	}
	if msg.Subject == "" {
		return fmt.Errorf("subject is required")
	}
	if msg.Body == "" && msg.HTMLBody == "" {
		return fmt.Errorf("body is required")
	}

	// Build email content
	content := c.buildContent(msg)

	// Collect all recipients
	recipients := append([]string{}, msg.To...)
	recipients = append(recipients, msg.Cc...)
	recipients = append(recipients, msg.Bcc...)

	// Create auth
	auth := smtp.PlainAuth("", c.config.Username, c.config.Password, c.config.SMTPHost)
	addr := fmt.Sprintf("%s:%d", c.config.SMTPHost, c.config.SMTPPort)

	// Send based on SSL/TLS settings
	if c.config.SMTPUseSSL {
		return c.sendWithSSL(addr, auth, recipients, content)
	}
	if c.config.SMTPUseTLS {
		return c.sendWithTLS(addr, auth, recipients, content)
	}
	return smtp.SendMail(addr, auth, c.config.Address, recipients, content)
}

// dialSSL establishes a direct SSL/TLS connection and returns an SMTP client.
func (c *SMTPClient) dialSSL(addr string) (*smtp.Client, func(), error) {
	tlsConfig := &tls.Config{ServerName: c.config.SMTPHost}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("SSL connection failed: %w", err)
	}

	client, err := smtp.NewClient(conn, c.config.SMTPHost)
	if err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("SMTP client creation failed: %w", err)
	}

	cleanup := func() {
		_ = client.Close()
		_ = conn.Close()
	}
	return client, cleanup, nil
}

// dialSTARTTLS establishes a plain connection and upgrades via STARTTLS.
func (c *SMTPClient) dialSTARTTLS(addr string) (*smtp.Client, func(), error) {
	client, err := smtp.Dial(addr)
	if err != nil {
		return nil, nil, fmt.Errorf("connection failed: %w", err)
	}

	if err = client.StartTLS(&tls.Config{ServerName: c.config.SMTPHost}); err != nil {
		_ = client.Close()
		return nil, nil, fmt.Errorf("STARTTLS failed: %w", err)
	}

	cleanup := func() { _ = client.Close() }
	return client, cleanup, nil
}

// sendViaClient authenticates and sends content through an established SMTP client.
func sendViaClient(client *smtp.Client, auth smtp.Auth, from string, recipients []string, content []byte) error {
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("MAIL command failed: %w", err)
	}

	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("RCPT command failed for %s: %w", recipient, err)
		}
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA command failed: %w", err)
	}

	_, err = writer.Write(content)
	if err != nil {
		_ = writer.Close()
		return fmt.Errorf("write failed: %w", err)
	}

	if err = writer.Close(); err != nil {
		return fmt.Errorf("DATA completion failed: %w", err)
	}

	_ = client.Quit()
	return nil
}

// sendWithSSL sends email using direct SSL/TLS connection (port 465).
func (c *SMTPClient) sendWithSSL(addr string, auth smtp.Auth, recipients []string, content []byte) error {
	client, cleanup, err := c.dialSSL(addr)
	if err != nil {
		return err
	}
	defer cleanup()

	return sendViaClient(client, auth, c.config.Address, recipients, content)
}

// sendWithTLS sends email using STARTTLS (port 587).
func (c *SMTPClient) sendWithTLS(addr string, auth smtp.Auth, recipients []string, content []byte) error {
	client, cleanup, err := c.dialSTARTTLS(addr)
	if err != nil {
		return err
	}
	defer cleanup()

	return sendViaClient(client, auth, c.config.Address, recipients, content)
}

// buildContent builds the email content with headers.
func (c *SMTPClient) buildContent(msg *OutgoingMessage) []byte {
	var buf bytes.Buffer

	// Headers
	if c.config.Name != "" {
		fmt.Fprintf(&buf, "From: %s <%s>\r\n", c.config.Name, c.config.Address)
	} else {
		fmt.Fprintf(&buf, "From: %s\r\n", c.config.Address)
	}

	fmt.Fprintf(&buf, "To: %s\r\n", strings.Join(msg.To, ", "))

	if len(msg.Cc) > 0 {
		fmt.Fprintf(&buf, "Cc: %s\r\n", strings.Join(msg.Cc, ", "))
	}

	if msg.ReplyTo != "" {
		fmt.Fprintf(&buf, "Reply-To: %s\r\n", msg.ReplyTo)
	}

	if msg.MessageID != "" {
		fmt.Fprintf(&buf, "Message-ID: %s\r\n", msg.MessageID)
	}

	fmt.Fprintf(&buf, "Subject: %s\r\n", msg.Subject)
	fmt.Fprintf(&buf, "Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z))
	buf.WriteString("MIME-Version: 1.0\r\n")

	// Body
	if msg.HTMLBody != "" && msg.Body != "" {
		// Multipart message
		boundary := fmt.Sprintf("taskschmiede-%d", time.Now().UTC().UnixNano())
		fmt.Fprintf(&buf, "Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary)
		buf.WriteString("\r\n")

		// Plain text part
		fmt.Fprintf(&buf, "--%s\r\n", boundary)
		buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		buf.WriteString("Content-Transfer-Encoding: 7bit\r\n")
		buf.WriteString("\r\n")
		buf.WriteString(msg.Body)
		buf.WriteString("\r\n\r\n")

		// HTML part
		fmt.Fprintf(&buf, "--%s\r\n", boundary)
		buf.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
		buf.WriteString("Content-Transfer-Encoding: 7bit\r\n")
		buf.WriteString("\r\n")
		buf.WriteString(msg.HTMLBody)
		buf.WriteString("\r\n\r\n")

		fmt.Fprintf(&buf, "--%s--\r\n", boundary)
	} else if msg.HTMLBody != "" {
		buf.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
		buf.WriteString("\r\n")
		buf.WriteString(msg.HTMLBody)
	} else {
		buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		buf.WriteString("\r\n")
		buf.WriteString(msg.Body)
	}

	return buf.Bytes()
}

// TestConnection tests the SMTP connection.
func (c *SMTPClient) TestConnection() error {
	auth := smtp.PlainAuth("", c.config.Username, c.config.Password, c.config.SMTPHost)
	addr := fmt.Sprintf("%s:%d", c.config.SMTPHost, c.config.SMTPPort)

	if c.config.SMTPUseSSL {
		client, cleanup, err := c.dialSSL(addr)
		if err != nil {
			return err
		}
		defer cleanup()
		return client.Auth(auth)
	}

	if c.config.SMTPUseTLS {
		client, cleanup, err := c.dialSTARTTLS(addr)
		if err != nil {
			return err
		}
		defer cleanup()
		return client.Auth(auth)
	}

	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer func() { _ = client.Close() }()

	return client.Auth(auth)
}
