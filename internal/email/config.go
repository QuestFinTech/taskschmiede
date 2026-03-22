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


// Package email provides SMTP and IMAP email functionality for Taskschmiede.
package email

import (
	"fmt"
)

// Config holds all email configuration loaded from environment variables.
type Config struct {
	// Sender identity
	Name    string // EMAIL_NAME
	Address string // EMAIL_ADDRESS

	// Authentication
	Username string // EMAIL_USER
	Password string // EMAIL_PASSWORD

	// SMTP (outgoing)
	SMTPHost   string // OUTGOING_MAIL_SERVER
	SMTPPort   int    // OUTGOING_MAIL_PORT
	SMTPUseTLS bool   // OUTGOING_MAIL_USE_TLS
	SMTPUseSSL bool   // OUTGOING_MAIL_USE_SSL

	// IMAP (incoming)
	IMAPHost   string // INCOMING_MAIL_SERVER
	IMAPPort   int    // INCOMING_MAIL_PORT
	IMAPUseTLS bool   // INCOMING_MAIL_USE_TLS
	IMAPUseSSL bool   // INCOMING_MAIL_USE_SSL
}

// ValidateSMTP checks if SMTP configuration is complete.
func (c *Config) ValidateSMTP() error {
	if c.SMTPHost == "" {
		return fmt.Errorf("OUTGOING_MAIL_SERVER is required")
	}
	if c.SMTPPort == 0 {
		return fmt.Errorf("OUTGOING_MAIL_PORT is required")
	}
	if c.Username == "" {
		return fmt.Errorf("EMAIL_USER is required")
	}
	if c.Password == "" {
		return fmt.Errorf("EMAIL_PASSWORD is required")
	}
	if c.Address == "" {
		return fmt.Errorf("EMAIL_ADDRESS is required")
	}
	return nil
}

// ValidateIMAP checks if IMAP configuration is complete.
func (c *Config) ValidateIMAP() error {
	if c.IMAPHost == "" {
		return fmt.Errorf("INCOMING_MAIL_SERVER is required")
	}
	if c.IMAPPort == 0 {
		return fmt.Errorf("INCOMING_MAIL_PORT is required")
	}
	if c.Username == "" {
		return fmt.Errorf("EMAIL_USER is required")
	}
	if c.Password == "" {
		return fmt.Errorf("EMAIL_PASSWORD is required")
	}
	return nil
}

