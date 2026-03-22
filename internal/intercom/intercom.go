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


// Package intercom implements the email bridge for Taskschmiede messaging.
// It handles outbound delivery (internal messages -> email) and inbound
// processing (email replies -> internal messages) through a dedicated
// intercom address.
//
// Outbound emails carry a deterministic Message-ID header and a short
// [TS-xxxx] reference in the subject line. Inbound replies are matched
// back to the original delivery via the In-Reply-To header (primary) or
// subject-line reference code (fallback).
package intercom

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/email"
	"github.com/QuestFinTech/taskschmiede/internal/security"
	"github.com/QuestFinTech/taskschmiede/internal/service"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// Config holds intercom configuration.
type Config struct {
	Enabled          bool
	Address          string        // intercom@taskschmiede.dev
	DisplayName      string        // "Taskschmiede Intercom"
	ReplyTTL         time.Duration // how long a reply is accepted after delivery (default 30 days)
	SweepInterval    time.Duration // default 1 minute
	SendInterval     time.Duration // default 30 seconds
	MaxRetries       int           // default 3
	MaxInboundPerHour int          // anti-bombing rate limit per sender (default 20)
	DedupWindow      time.Duration // content-based dedup window (default 1h)
}

// refCodeRegex matches [TS-xxxxxxxx] in subject lines.
var refCodeRegex = regexp.MustCompile(`\[TS-([a-f0-9]{8})\]`)

// Intercom handles the email bridge: outbound delivery and inbound polling.
type Intercom struct {
	smtpClient   *email.SMTPClient
	imapConfig   *email.Config
	msgSvc       *service.MessageService
	msgDB        *storage.MessageDB
	mainDB       *storage.DB
	audit        *security.AuditService
	logger       *slog.Logger
	config       Config
	inbound      inboundGuard
	stopCh       chan struct{}
	doneCh       chan struct{}
}

// inboundGuard provides in-memory rate limiting and deduplication for inbound emails.
type inboundGuard struct {
	mu     sync.Mutex
	counts map[string][]time.Time // sender email -> timestamps of recent inbound
	hashes map[string]time.Time   // sha256(sender+content)[:16] -> last processed time
}

// New creates a new Intercom instance with the given dependencies and configuration.
func New(smtpClient *email.SMTPClient, imapConfig *email.Config, msgSvc *service.MessageService, msgDB *storage.MessageDB, mainDB *storage.DB, audit *security.AuditService, logger *slog.Logger, config Config) (*Intercom, error) {
	if config.ReplyTTL == 0 {
		config.ReplyTTL = 30 * 24 * time.Hour
	}
	if config.MaxInboundPerHour <= 0 {
		config.MaxInboundPerHour = 20
	}
	if config.DedupWindow == 0 {
		config.DedupWindow = 1 * time.Hour
	}

	return &Intercom{
		smtpClient: smtpClient,
		imapConfig: imapConfig,
		msgSvc:     msgSvc,
		msgDB:      msgDB,
		mainDB:     mainDB,
		audit:      audit,
		logger:     logger,
		config:     config,
		inbound: inboundGuard{
			counts: make(map[string][]time.Time),
			hashes: make(map[string]time.Time),
		},
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}, nil
}

// Start launches the background goroutines for outbound sending and inbound sweeping.
func (ic *Intercom) Start() {
	go ic.run()
}

// Stop gracefully shuts down the intercom goroutines.
func (ic *Intercom) Stop() {
	close(ic.stopCh)
	<-ic.doneCh
}

// run is the main loop that drives both outbound and inbound processing.
func (ic *Intercom) run() {
	defer close(ic.doneCh)

	sendTicker := time.NewTicker(ic.config.SendInterval)
	defer sendTicker.Stop()

	sweepTicker := time.NewTicker(ic.config.SweepInterval)
	defer sweepTicker.Stop()

	ic.logger.Info("Intercom started",
		"address", ic.config.Address,
		"send_interval", ic.config.SendInterval,
		"sweep_interval", ic.config.SweepInterval,
	)

	for {
		select {
		case <-ic.stopCh:
			ic.logger.Info("Intercom stopping")
			return
		case <-sendTicker.C:
			ic.sendPending()
			ic.sendCopies()
		case <-sweepTicker.C:
			ic.sweepInbox()
		}
	}
}

// sendPending processes pending email deliveries.
func (ic *Intercom) sendPending() {
	pending, err := ic.msgDB.ListPendingEmailDeliveries(50)
	if err != nil {
		ic.logger.Error("Failed to list pending email deliveries", "error", err)
		return
	}

	if len(pending) == 0 {
		return
	}

	ic.logger.Debug("Processing pending email deliveries", "count", len(pending))

	for _, pd := range pending {
		if err := ic.sendOne(pd); err != nil {
			ic.logger.Error("Failed to send email delivery",
				"delivery_id", pd.DeliveryID,
				"recipient_id", pd.RecipientID,
				"error", err,
			)
			_ = ic.msgDB.MarkFailed(pd.DeliveryID)
		}
	}
}

// sendOne sends a single email delivery with deterministic Message-ID and
// [TS-refCode] subject tag for reply matching.
func (ic *Intercom) sendOne(pd *storage.PendingDelivery) error {
	// Look up recipient's email from resource metadata
	recipientEmail := ic.resolveEmail(pd.RecipientID)
	if recipientEmail == "" {
		return fmt.Errorf("no email configured for resource %s", pd.RecipientID)
	}

	// Look up sender's display name
	senderName := ic.msgSvc.ResolveSenderName(pd.SenderID)

	// Derive ref_code: first 8 hex chars after the "mdl_" prefix of delivery ID.
	refCode := strings.TrimPrefix(pd.DeliveryID, "mdl_")
	if len(refCode) > 8 {
		refCode = refCode[:8]
	}

	// Deterministic Message-ID from the delivery ID.
	emailMessageID := fmt.Sprintf("<%s@taskschmiede.dev>", pd.DeliveryID)

	// Build subject with [TS-refCode] tag.
	subject := pd.Subject
	if subject == "" {
		subject = fmt.Sprintf("Message from %s", senderName)
	}
	subject = fmt.Sprintf("[TS-%s] %s", refCode, subject)

	// Build email body. If the content contains HTML, produce both a
	// text/html and text/plain part (multipart/alternative).
	msg := &email.OutgoingMessage{
		To:        []string{recipientEmail},
		Subject:   subject,
		ReplyTo:   ic.config.Address,
		MessageID: emailMessageID,
	}

	contentIsHTML := containsHTML(pd.Content)

	// Footer lines shared by both versions.
	contextLine := ""
	if pd.EntityType != "" && pd.EntityID != "" {
		contextLine = fmt.Sprintf("Context: %s %s", pd.EntityType, pd.EntityID)
	}

	// Plain text part (always present).
	var text strings.Builder
	fmt.Fprintf(&text, "Message from %s\n", senderName)
	text.WriteString(strings.Repeat("-", 40))
	text.WriteString("\n\n")
	if pd.Subject != "" {
		fmt.Fprintf(&text, "Subject: %s\n\n", pd.Subject)
	}
	if contentIsHTML {
		text.WriteString(stripHTML(pd.Content))
	} else {
		text.WriteString(pd.Content)
	}
	text.WriteString("\n\n")
	text.WriteString(strings.Repeat("-", 40))
	text.WriteString("\n")
	if contextLine != "" {
		text.WriteString(contextLine)
		text.WriteString("\n")
	}
	fmt.Fprintf(&text, "Reply to this email to respond to %s.\n", senderName)
	text.WriteString("Sent via Taskschmiede Intercom\n")
	msg.Body = text.String()

	// HTML part (only when content contains HTML).
	if contentIsHTML {
		var html strings.Builder
		html.WriteString("<!DOCTYPE html>\n<html><body>\n")
		fmt.Fprintf(&html, "<p><strong>Message from %s</strong></p>\n<hr>\n", senderName)
		html.WriteString(pd.Content)
		html.WriteString("\n<hr>\n")
		if contextLine != "" {
			fmt.Fprintf(&html, "<p style=\"color:#888;font-size:12px\">%s</p>\n", contextLine)
		}
		fmt.Fprintf(&html, "<p style=\"color:#888;font-size:12px\">Reply to this email to respond to %s.<br>Sent via Taskschmiede Intercom</p>\n", senderName)
		html.WriteString("</body></html>\n")
		msg.HTMLBody = html.String()
	}

	if err := ic.smtpClient.Send(msg); err != nil {
		return fmt.Errorf("send email: %w", err)
	}

	// Store email routing info for inbound matching.
	if err := ic.msgDB.SetEmailRouting(pd.DeliveryID, emailMessageID, refCode); err != nil {
		ic.logger.Warn("Email sent but failed to set routing",
			"delivery_id", pd.DeliveryID, "error", err)
	}

	// Mark as delivered
	if err := ic.msgDB.MarkDelivered(pd.DeliveryID); err != nil {
		ic.logger.Warn("Email sent but failed to mark delivered",
			"delivery_id", pd.DeliveryID, "error", err)
	}

	// Audit log
	if ic.audit != nil {
		ic.audit.Log(&security.AuditEntry{
			Action:   security.AuditIntercomSend,
			ActorID:  pd.SenderID,
			Resource: "message:" + pd.MessageID,
			Source:   "system",
			Metadata: map[string]interface{}{
				"recipient_id": pd.RecipientID,
				"delivery_id":  pd.DeliveryID,
				"ref_code":     refCode,
			},
		})
	}

	ic.logger.Info("Intercom email sent",
		"delivery_id", pd.DeliveryID,
		"message_id", pd.MessageID,
		"to", recipientEmail,
		"ref_code", refCode,
	)

	return nil
}

// sendCopies processes pending email copy deliveries (internal messages with copy_email set).
func (ic *Intercom) sendCopies() {
	pending, err := ic.msgDB.ListPendingCopyDeliveries(50)
	if err != nil {
		ic.logger.Error("Failed to list pending copy deliveries", "error", err)
		return
	}

	if len(pending) == 0 {
		return
	}

	ic.logger.Debug("Processing pending email copy deliveries", "count", len(pending))

	for _, pd := range pending {
		// Extract copy_email from metadata (set by ListPendingCopyDeliveries)
		copyEmail, _ := pd.Metadata["_copy_email"].(string)
		if copyEmail == "" {
			ic.logger.Warn("Copy delivery missing email", "delivery_id", pd.DeliveryID)
			continue
		}

		if err := ic.sendCopyOne(pd, copyEmail); err != nil {
			ic.logger.Error("Failed to send email copy",
				"delivery_id", pd.DeliveryID,
				"email", copyEmail,
				"error", err,
			)
			// Do not mark as failed -- retry on next tick.
			// Email copy failures should not break the internal delivery.
			continue
		}
	}
}

// sendCopyOne sends a single email copy for an internal delivery.
func (ic *Intercom) sendCopyOne(pd *storage.PendingDelivery, recipientEmail string) error {
	senderName := ic.msgSvc.ResolveSenderName(pd.SenderID)

	subject := pd.Subject
	if subject == "" {
		subject = fmt.Sprintf("Message from %s", senderName)
	}
	subject = fmt.Sprintf("[Copy] %s", subject)

	msg := &email.OutgoingMessage{
		To:      []string{recipientEmail},
		Subject: subject,
	}

	contentIsHTML := containsHTML(pd.Content)

	// Plain text
	var text strings.Builder
	fmt.Fprintf(&text, "Copy of internal message from %s\n", senderName)
	text.WriteString(strings.Repeat("-", 40))
	text.WriteString("\n\n")
	if pd.Subject != "" {
		fmt.Fprintf(&text, "Subject: %s\n\n", pd.Subject)
	}
	if contentIsHTML {
		text.WriteString(stripHTML(pd.Content))
	} else {
		text.WriteString(pd.Content)
	}
	text.WriteString("\n\n")
	text.WriteString(strings.Repeat("-", 40))
	text.WriteString("\n")
	text.WriteString("This is an automatic copy of an internal Taskschmiede message.\n")
	msg.Body = text.String()

	if contentIsHTML {
		var html strings.Builder
		html.WriteString("<!DOCTYPE html>\n<html><body>\n")
		fmt.Fprintf(&html, "<p><strong>Copy of internal message from %s</strong></p>\n<hr>\n", senderName)
		html.WriteString(pd.Content)
		html.WriteString("\n<hr>\n")
		html.WriteString("<p style=\"color:#888;font-size:12px\">This is an automatic copy of an internal Taskschmiede message.</p>\n")
		html.WriteString("</body></html>\n")
		msg.HTMLBody = html.String()
	}

	if err := ic.smtpClient.Send(msg); err != nil {
		return fmt.Errorf("send copy email: %w", err)
	}

	// Mark as "copied" (not "delivered" -- that's for full email channel)
	if err := ic.msgDB.MarkCopied(pd.DeliveryID); err != nil {
		ic.logger.Warn("Email copy sent but failed to mark copied",
			"delivery_id", pd.DeliveryID, "error", err)
	}

	ic.logger.Info("Email copy sent",
		"delivery_id", pd.DeliveryID,
		"message_id", pd.MessageID,
		"to", recipientEmail,
	)

	return nil
}

// sweepInbox processes inbound emails from the IMAP inbox.
func (ic *Intercom) sweepInbox() {
	if ic.imapConfig == nil {
		return
	}

	imapClient, err := email.NewIMAPClient(ic.imapConfig)
	if err != nil {
		ic.logger.Error("Failed to create IMAP client", "error", err)
		return
	}

	if err := imapClient.Connect(); err != nil {
		ic.logger.Error("Failed to connect to IMAP", "error", err)
		return
	}
	defer func() { _ = imapClient.Close() }()

	msgCount, err := imapClient.SelectFolder("INBOX")
	if err != nil {
		ic.logger.Error("Failed to select INBOX", "error", err)
		return
	}

	if msgCount == 0 {
		return
	}

	ic.logger.Debug("Sweeping IMAP inbox", "messages", msgCount)

	// Search all messages
	uids, err := imapClient.SearchMessages(email.SearchCriteria{MaxMessages: 50})
	if err != nil {
		ic.logger.Error("Failed to search messages", "error", err)
		return
	}

	for _, uid := range uids {
		ic.processInbound(imapClient, uid)
	}

	// Expunge deleted messages
	if err := imapClient.Expunge(); err != nil {
		ic.logger.Warn("Failed to expunge", "error", err)
	}

	// Prune stale inbound guard entries.
	ic.inbound.cleanup(ic.config.DedupWindow)
}

// processInbound handles a single inbound email using header-based matching.
func (ic *Intercom) processInbound(imapClient *email.IMAPClient, uid uint32) {
	msg, err := imapClient.FetchMessage(uid)
	if err != nil {
		ic.logger.Error("Failed to fetch message", "uid", uid, "error", err)
		return
	}

	senderEmail := extractEmailAddr(msg.From)

	// Anti-bombing: rate limit per sender.
	if !ic.inbound.allowSender(senderEmail, ic.config.MaxInboundPerHour) {
		ic.logger.Info("Inbound email rejected: rate limit exceeded",
			"from", senderEmail, "uid", uid)
		_ = imapClient.Delete(uid)
		if ic.audit != nil {
			ic.audit.Log(&security.AuditEntry{
				Action:   security.AuditIntercomRejectFlooded,
				Resource: fmt.Sprintf("email:uid=%d", uid),
				Source:   "system",
				Metadata: map[string]interface{}{"from": senderEmail},
			})
		}
		return
	}

	// Extract reply content.
	content := msg.BodyText
	if content == "" {
		content = msg.BodyHTML
	}
	if content == "" {
		ic.logger.Info("Inbound email skipped: empty body", "uid", uid)
		_ = imapClient.Delete(uid)
		return
	}

	// Dedup: reject identical content from same sender within window.
	if ic.inbound.isDuplicate(senderEmail, content, ic.config.DedupWindow) {
		ic.logger.Info("Inbound email rejected: duplicate content",
			"from", senderEmail, "uid", uid)
		_ = imapClient.Delete(uid)
		if ic.audit != nil {
			ic.audit.Log(&security.AuditEntry{
				Action:   security.AuditIntercomRejectDuplicate,
				Resource: fmt.Sprintf("email:uid=%d", uid),
				Source:   "system",
				Metadata: map[string]interface{}{"from": senderEmail},
			})
		}
		return
	}

	// Route: try In-Reply-To header first, then subject [TS-xxxx] fallback.
	delivery := ic.matchDelivery(msg)
	if delivery == nil {
		ic.logger.Info("Inbound email rejected: no matching delivery",
			"from", senderEmail, "uid", uid)
		_ = imapClient.Delete(uid)
		if ic.audit != nil {
			ic.audit.Log(&security.AuditEntry{
				Action:   security.AuditIntercomRejectNoMatch,
				Resource: fmt.Sprintf("email:uid=%d", uid),
				Source:   "system",
				Metadata: map[string]interface{}{"from": senderEmail},
			})
		}
		return
	}

	// TTL: check whether the reply is within the allowed window.
	if delivery.DeliveredAt != nil {
		age := storage.UTCNow().Sub(*delivery.DeliveredAt)
		if age > ic.config.ReplyTTL {
			ic.logger.Info("Inbound email rejected: delivery too old",
				"from", senderEmail, "uid", uid,
				"delivered_at", delivery.DeliveredAt, "age", age)
			_ = imapClient.Delete(uid)
			if ic.audit != nil {
				ic.audit.Log(&security.AuditEntry{
					Action:   security.AuditIntercomRejectExpired,
					Resource: fmt.Sprintf("email:uid=%d", uid),
					Source:   "system",
					Metadata: map[string]interface{}{
						"from":         senderEmail,
						"delivered_at": delivery.DeliveredAt.Format("2006-01-02T15:04:05Z"),
					},
				})
			}
			return
		}
	}

	// Sender verification: the replying human must be the original recipient.
	expectedEmail := ic.resolveEmail(delivery.RecipientID)
	if expectedEmail != "" && !strings.EqualFold(senderEmail, expectedEmail) {
		ic.logger.Info("Inbound email rejected: sender mismatch",
			"expected", expectedEmail, "got", senderEmail, "uid", uid)
		_ = imapClient.Delete(uid)
		if ic.audit != nil {
			ic.audit.Log(&security.AuditEntry{
				Action:   security.AuditIntercomRejectMismatch,
				Resource: fmt.Sprintf("email:uid=%d", uid),
				Source:   "system",
				Metadata: map[string]interface{}{
					"from":     senderEmail,
					"expected": expectedEmail,
				},
			})
		}
		return
	}

	// Deliver: create internal reply from the human (recipient) to the original sender.
	ctx := context.Background()
	_, err = ic.msgSvc.Reply(ctx, delivery.RecipientID, delivery.MessageID, content, nil)
	if err != nil {
		ic.logger.Error("Failed to create internal reply from inbound email",
			"uid", uid, "error", err)
		// Don't delete -- will retry on next sweep
		return
	}

	// Record successful processing for dedup.
	ic.inbound.record(senderEmail, content)

	// Delete processed message
	_ = imapClient.Delete(uid)

	// Audit log
	if ic.audit != nil {
		ic.audit.Log(&security.AuditEntry{
			Action:   security.AuditIntercomReceive,
			ActorID:  delivery.RecipientID,
			Resource: "message:" + delivery.MessageID,
			Source:   "system",
			Metadata: map[string]interface{}{
				"from_email":  senderEmail,
				"original_to": delivery.SenderID,
			},
		})
	}

	// Attachment notice: if the inbound email had attachments, notify the sender.
	if msg.HasAttachments {
		ic.sendAttachmentNotice(senderEmail, msg.Subject)
	}

	ic.logger.Info("Inbound email processed",
		"uid", uid,
		"from_resource", delivery.RecipientID,
		"to_resource", delivery.SenderID,
		"original_message", delivery.MessageID,
		"had_attachments", msg.HasAttachments,
	)
}

// sendAttachmentNotice sends an auto-reply to the sender informing them that
// attachments are not supported. The text body of their reply was delivered
// normally; only the attachments were dropped.
func (ic *Intercom) sendAttachmentNotice(recipientEmail, originalSubject string) {
	subject := "Attachments not supported"
	if originalSubject != "" {
		subject = "Re: " + originalSubject
	}

	body := "Your message was delivered, but any attached files were not processed.\n\n" +
		"Taskschmiede does not currently support email attachments. " +
		"If you need to share files, please use a link or paste the content directly into your message.\n\n" +
		"-- Taskschmiede Intercom\n"

	msg := &email.OutgoingMessage{
		To:      []string{recipientEmail},
		Subject: subject,
		Body:    body,
	}

	if err := ic.smtpClient.Send(msg); err != nil {
		ic.logger.Warn("Failed to send attachment notice",
			"to", recipientEmail, "error", err)
		return
	}

	if ic.audit != nil {
		ic.audit.Log(&security.AuditEntry{
			Action:   security.AuditIntercomAttachmentNotice,
			Resource: fmt.Sprintf("email:%s", recipientEmail),
			Source:   "system",
			Metadata: map[string]interface{}{"to": recipientEmail},
		})
	}

	ic.logger.Info("Attachment notice sent", "to", recipientEmail)
}

// matchDelivery tries to match an inbound email to a previous outbound delivery.
// Primary: In-Reply-To header -> LookupDeliveryByEmailMessageID.
// Fallback: [TS-xxxxxxxx] in subject -> LookupDeliveryByRefCode.
func (ic *Intercom) matchDelivery(msg *email.IncomingMessage) *storage.EmailDeliveryLookup {
	// Primary: In-Reply-To header
	if inReplyTo := msg.Headers["In-Reply-To"]; len(inReplyTo) > 0 {
		ref := strings.TrimSpace(inReplyTo[0])
		if ref != "" {
			delivery, err := ic.msgDB.LookupDeliveryByEmailMessageID(ref)
			if err == nil {
				return delivery
			}
		}
	}

	// Fallback: [TS-xxxxxxxx] in subject
	if matches := refCodeRegex.FindStringSubmatch(msg.Subject); len(matches) == 2 {
		delivery, err := ic.msgDB.LookupDeliveryByRefCode(matches[1])
		if err == nil {
			return delivery
		}
	}

	return nil
}

// resolveEmail looks up a resource's email from its metadata.
func (ic *Intercom) resolveEmail(resourceID string) string {
	res, err := ic.mainDB.GetResource(resourceID)
	if err != nil {
		return ""
	}

	if delivery, ok := res.Metadata["delivery"].(map[string]interface{}); ok {
		if emailAddr, ok := delivery["email"].(string); ok {
			return emailAddr
		}
	}

	return ""
}

// extractEmailAddr extracts the bare email from "Name <email>" format.
func extractEmailAddr(addr string) string {
	if idx := strings.Index(addr, "<"); idx >= 0 {
		end := strings.Index(addr, ">")
		if end > idx {
			return addr[idx+1 : end]
		}
	}
	return strings.TrimSpace(addr)
}

// --- inboundGuard methods ---

// allowSender checks whether a sender is within the hourly rate limit.
// Uses a sliding window of timestamps.
func (g *inboundGuard) allowSender(senderEmail string, maxPerHour int) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := storage.UTCNow()
	cutoff := now.Add(-1 * time.Hour)

	// Prune old timestamps.
	timestamps := g.counts[senderEmail]
	pruned := timestamps[:0]
	for _, ts := range timestamps {
		if ts.After(cutoff) {
			pruned = append(pruned, ts)
		}
	}

	if len(pruned) >= maxPerHour {
		g.counts[senderEmail] = pruned
		return false
	}

	g.counts[senderEmail] = append(pruned, now)
	return true
}

// isDuplicate returns true if the same sender+content combination was seen
// within the dedup window.
func (g *inboundGuard) isDuplicate(senderEmail, content string, window time.Duration) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	key := hashKey(senderEmail, content)
	if lastSeen, ok := g.hashes[key]; ok {
		if storage.UTCNow().Sub(lastSeen) < window {
			return true
		}
	}
	return false
}

// record marks a sender+content combination as processed.
func (g *inboundGuard) record(senderEmail, content string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	key := hashKey(senderEmail, content)
	g.hashes[key] = storage.UTCNow()
}

// cleanup removes stale entries from counts and hashes.
func (g *inboundGuard) cleanup(dedupWindow time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := storage.UTCNow()
	hourAgo := now.Add(-1 * time.Hour)

	// Prune rate-limit counters.
	for sender, timestamps := range g.counts {
		pruned := timestamps[:0]
		for _, ts := range timestamps {
			if ts.After(hourAgo) {
				pruned = append(pruned, ts)
			}
		}
		if len(pruned) == 0 {
			delete(g.counts, sender)
		} else {
			g.counts[sender] = pruned
		}
	}

	// Prune dedup hashes.
	cutoff := now.Add(-dedupWindow)
	for key, ts := range g.hashes {
		if ts.Before(cutoff) {
			delete(g.hashes, key)
		}
	}
}

// hashKey produces a short hex key for sender+content dedup.
func hashKey(senderEmail, content string) string {
	h := sha256.Sum256([]byte(senderEmail + "\x00" + content))
	return fmt.Sprintf("%x", h[:8])
}

// htmlTagRegex matches HTML tags for detection and stripping.
var htmlTagRegex = regexp.MustCompile(`<[a-zA-Z/][^>]*>`)

// htmlBlockBreakRegex matches <br>, <br/>, <hr>, and block-level closing tags.
var htmlBlockBreakRegex = regexp.MustCompile(`(?i)<br\s*/?>|<hr\s*/?>|</(?:p|div|h[1-6]|li|tr)>`)

// excessiveNewlineRegex matches three or more consecutive newlines.
var excessiveNewlineRegex = regexp.MustCompile(`\n{3,}`)

// containsHTML returns true if s appears to contain HTML markup.
func containsHTML(s string) bool {
	return htmlTagRegex.MatchString(s)
}

// stripHTML removes HTML tags and collapses whitespace for a plain text fallback.
func stripHTML(s string) string {
	// Replace <br>, <br/>, <hr>, block-level closing tags with newlines.
	s = htmlBlockBreakRegex.ReplaceAllString(s, "\n")
	// Remove all remaining tags.
	s = htmlTagRegex.ReplaceAllString(s, "")
	// Decode common HTML entities.
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	// Collapse excessive blank lines.
	s = excessiveNewlineRegex.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}
