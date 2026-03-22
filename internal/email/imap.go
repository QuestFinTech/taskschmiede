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

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
)

// IMAPClient handles receiving emails via IMAP.
type IMAPClient struct {
	config   *Config
	client   *client.Client
	selected string
}

// NewIMAPClient creates a new IMAP client.
func NewIMAPClient(config *Config) (*IMAPClient, error) {
	if err := config.ValidateIMAP(); err != nil {
		return nil, fmt.Errorf("invalid IMAP config: %w", err)
	}
	return &IMAPClient{config: config}, nil
}

// Connect establishes connection to the IMAP server.
func (c *IMAPClient) Connect() error {
	addr := fmt.Sprintf("%s:%d", c.config.IMAPHost, c.config.IMAPPort)
	dialer := &net.Dialer{Timeout: 15 * time.Second}

	var err error
	if c.config.IMAPUseSSL || c.config.IMAPUseTLS {
		c.client, err = client.DialWithDialerTLS(dialer, addr, &tls.Config{ServerName: c.config.IMAPHost})
	} else {
		c.client, err = client.DialWithDialer(dialer, addr)
	}

	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}

	c.client.Timeout = 30 * time.Second

	// Authenticate
	if err := c.client.Login(c.config.Username, c.config.Password); err != nil {
		_ = c.client.Logout()
		return fmt.Errorf("authentication failed: %w", err)
	}

	return nil
}

// Close closes the IMAP connection.
func (c *IMAPClient) Close() error {
	if c.client != nil {
		err := c.client.Logout()
		c.client = nil
		return err
	}
	return nil
}

// ListFolders returns available mailbox folders.
func (c *IMAPClient) ListFolders() ([]string, error) {
	if c.client == nil {
		return nil, fmt.Errorf("not connected")
	}

	mailboxes := make(chan *imap.MailboxInfo, 10)
	done := make(chan error, 1)
	go func() {
		done <- c.client.List("", "*", mailboxes)
	}()

	var folders []string
	for m := range mailboxes {
		folders = append(folders, m.Name)
	}

	if err := <-done; err != nil {
		return nil, fmt.Errorf("list folders failed: %w", err)
	}

	return folders, nil
}

// SelectFolder selects a folder for message retrieval.
func (c *IMAPClient) SelectFolder(folder string) (uint32, error) {
	if c.client == nil {
		return 0, fmt.Errorf("not connected")
	}

	mbox, err := c.client.Select(folder, false)
	if err != nil {
		return 0, fmt.Errorf("select folder failed: %w", err)
	}

	c.selected = folder
	return mbox.Messages, nil
}

// SearchCriteria defines search parameters.
type SearchCriteria struct {
	Since       time.Time
	UnreadOnly  bool
	MaxMessages int
}

// SearchMessages searches for messages matching criteria.
func (c *IMAPClient) SearchMessages(criteria SearchCriteria) ([]uint32, error) {
	if c.client == nil {
		return nil, fmt.Errorf("not connected")
	}
	if c.selected == "" {
		return nil, fmt.Errorf("no folder selected")
	}

	// Build IMAP search criteria
	searchCriteria := imap.NewSearchCriteria()

	if !criteria.Since.IsZero() {
		searchCriteria.Since = criteria.Since
	}

	if criteria.UnreadOnly {
		searchCriteria.WithoutFlags = []string{imap.SeenFlag}
	}

	uids, err := c.client.UidSearch(searchCriteria)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Apply limit
	if criteria.MaxMessages > 0 && len(uids) > criteria.MaxMessages {
		uids = uids[:criteria.MaxMessages]
	}

	return uids, nil
}

// FetchMessage retrieves a complete message by UID.
func (c *IMAPClient) FetchMessage(uid uint32) (*IncomingMessage, error) {
	if c.client == nil {
		return nil, fmt.Errorf("not connected")
	}
	if c.selected == "" {
		return nil, fmt.Errorf("no folder selected")
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(uid)

	items := []imap.FetchItem{
		imap.FetchEnvelope,
		imap.FetchFlags,
		imap.FetchRFC822,
		imap.FetchRFC822Size,
		imap.FetchUid,
	}

	messages := make(chan *imap.Message, 1)
	done := make(chan error, 1)
	go func() {
		done <- c.client.UidFetch(seqset, items, messages)
	}()

	msg := <-messages
	if msg == nil {
		return nil, fmt.Errorf("message not found: UID %d", uid)
	}

	if err := <-done; err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}

	return c.parseMessage(msg)
}

// parseMessage converts IMAP message to IncomingMessage.
func (c *IMAPClient) parseMessage(msg *imap.Message) (*IncomingMessage, error) {
	email := &IncomingMessage{
		UID:     msg.Uid,
		Headers: make(map[string][]string),
		Size:    msg.Size,
	}

	// Extract envelope
	if msg.Envelope != nil {
		email.Date = msg.Envelope.Date
		email.Subject = msg.Envelope.Subject
		email.MessageID = msg.Envelope.MessageId

		if len(msg.Envelope.From) > 0 {
			email.From = msg.Envelope.From[0].Address()
		}

		for _, to := range msg.Envelope.To {
			email.To = append(email.To, to.Address())
		}
	}

	// Extract flags
	email.Flags = append(email.Flags, msg.Flags...)

	// Parse RFC822 body
	section := &imap.BodySectionName{}
	if body := msg.GetBody(section); body != nil {
		if err := c.parseBody(body, email); err != nil {
			// Non-fatal - continue with partial data
			_ = err
		}
	}

	return email, nil
}

// parseBody parses the RFC822 message body.
func (c *IMAPClient) parseBody(r io.Reader, email *IncomingMessage) error {
	mr, err := mail.CreateReader(r)
	if err != nil {
		return fmt.Errorf("create mail reader failed: %w", err)
	}

	// Parse headers
	header := mr.Header
	fields := header.Fields()
	for fields.Next() {
		key := fields.Key()
		value, _ := fields.Text()
		email.Headers[key] = append(email.Headers[key], value)
	}

	// Parse parts
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read part failed: %w", err)
		}

		switch h := part.Header.(type) {
		case *mail.InlineHeader:
			contentType, _, _ := h.ContentType()
			body, err := io.ReadAll(part.Body)
			if err != nil {
				continue
			}

			switch contentType {
			case "text/plain":
				email.BodyText = string(body)
			case "text/html":
				email.BodyHTML = string(body)
			}
		case *mail.AttachmentHeader:
			email.HasAttachments = true
		}
	}

	return nil
}

// MarkAsRead marks a message as read.
func (c *IMAPClient) MarkAsRead(uid uint32) error {
	if c.client == nil {
		return fmt.Errorf("not connected")
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(uid)

	flags := []interface{}{imap.SeenFlag}
	item := imap.FormatFlagsOp(imap.AddFlags, true)

	return c.client.UidStore(seqset, item, flags, nil)
}

// Delete marks a message for deletion.
func (c *IMAPClient) Delete(uid uint32) error {
	if c.client == nil {
		return fmt.Errorf("not connected")
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(uid)

	flags := []interface{}{imap.DeletedFlag}
	item := imap.FormatFlagsOp(imap.AddFlags, true)

	return c.client.UidStore(seqset, item, flags, nil)
}

// Expunge permanently removes messages marked for deletion.
func (c *IMAPClient) Expunge() error {
	if c.client == nil {
		return fmt.Errorf("not connected")
	}

	return c.client.Expunge(nil)
}

// TestConnection tests the IMAP connection.
func (c *IMAPClient) TestConnection() error {
	if err := c.Connect(); err != nil {
		return err
	}
	defer func() { _ = c.Close() }()

	// Try to list folders as a connectivity test
	_, err := c.ListFolders()
	return err
}
