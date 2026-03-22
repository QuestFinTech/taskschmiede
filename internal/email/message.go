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

import "time"

// OutgoingMessage represents an email to be sent.
type OutgoingMessage struct {
	To        []string
	Cc        []string
	Bcc       []string
	Subject   string
	Body      string
	HTMLBody  string
	ReplyTo   string
	MessageID string // RFC 2822 Message-ID (e.g. "<mdl_xxx@taskschmiede.dev>")
}

// IncomingMessage represents a received email.
type IncomingMessage struct {
	UID            uint32
	MessageID      string
	Date           time.Time
	From           string
	To             []string
	Subject        string
	BodyText       string
	BodyHTML       string
	Headers        map[string][]string
	Flags          []string
	Size           uint32
	HasAttachments bool
}

// Attachment represents an email attachment.
type Attachment struct {
	Filename    string
	ContentType string
	Size        int64
	Data        []byte
}
