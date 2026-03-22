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


package email

import "fmt"

// Service wraps a Renderer and SMTPClient to send templated emails.
// It also implements the SendEmail(to, subject, body) interface for
// backward compatibility with callsites that send plain text.
type Service struct {
	renderer *Renderer
	client   *SMTPClient
}

// NewService creates a Service by initializing a Renderer and storing the SMTP client.
func NewService(client *SMTPClient) (*Service, error) {
	r, err := NewRenderer()
	if err != nil {
		return nil, fmt.Errorf("create renderer: %w", err)
	}
	return &Service{renderer: r, client: client}, nil
}

// SendEmail sends a plain text email. Implements the EmailSender interface
// used by api, mcp, and ticker packages.
func (s *Service) SendEmail(to, subject, body string) error {
	msg := &OutgoingMessage{
		To:      []string{to},
		Subject: subject,
		Body:    body,
	}
	return s.client.Send(msg)
}

// SendVerificationCode sends a verification code email with HTML formatting.
func (s *Service) SendVerificationCode(to, subject string, data *VerificationCodeData) error {
	msg, err := s.renderer.RenderVerificationCode(data)
	if err != nil {
		return fmt.Errorf("render verification code: %w", err)
	}
	msg.To = []string{to}
	msg.Subject = subject
	return s.client.Send(msg)
}

// SendWaitlistWelcome sends a waitlist welcome email with HTML formatting.
func (s *Service) SendWaitlistWelcome(to, subject string, data *WaitlistWelcomeData) error {
	msg, err := s.renderer.RenderWaitlistWelcome(data)
	if err != nil {
		return fmt.Errorf("render waitlist welcome: %w", err)
	}
	msg.To = []string{to}
	msg.Subject = subject
	return s.client.Send(msg)
}

// SendInactivityWarning sends an inactivity warning email with HTML formatting.
func (s *Service) SendInactivityWarning(to, subject string, data *InactivityData) error {
	msg, err := s.renderer.RenderInactivityWarning(data)
	if err != nil {
		return fmt.Errorf("render inactivity warning: %w", err)
	}
	msg.To = []string{to}
	msg.Subject = subject
	return s.client.Send(msg)
}

// SendInactivityDeactivation sends an inactivity deactivation email with HTML formatting.
func (s *Service) SendInactivityDeactivation(to, subject string, data *InactivityData) error {
	msg, err := s.renderer.RenderInactivityDeactivation(data)
	if err != nil {
		return fmt.Errorf("render inactivity deactivation: %w", err)
	}
	msg.To = []string{to}
	msg.Subject = subject
	return s.client.Send(msg)
}
