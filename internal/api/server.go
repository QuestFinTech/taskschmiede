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


// Package api provides the REST API for Taskschmiede.
// All entity operations are exposed as RESTful endpoints under /api/v1/.
// MCP tools and Web UI consume this same layer.
package api

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/auth"
	"github.com/QuestFinTech/taskschmiede/internal/email"
	"github.com/QuestFinTech/taskschmiede/internal/i18n"
	"github.com/QuestFinTech/taskschmiede/internal/security"
	"github.com/QuestFinTech/taskschmiede/internal/service"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// EmailSender is the interface for sending emails.
type EmailSender interface {
	SendEmail(to, subject, body string) error
}

// TemplatedEmailSender extends EmailSender with HTML template methods.
// Implemented by *email.Service. Callsites type-assert to check availability.
type TemplatedEmailSender interface {
	EmailSender
	SendVerificationCode(to, subject string, data *email.VerificationCodeData) error
	SendWaitlistWelcome(to, subject string, data *email.WaitlistWelcomeData) error
	SendInactivityWarning(to, subject string, data *email.InactivityData) error
	SendInactivityDeactivation(to, subject string, data *email.InactivityData) error
}

// API holds the REST API dependencies and configuration.
type API struct {
	db     *storage.DB
	logger *slog.Logger

	// Services
	authSvc     *auth.Service
	orgSvc      *service.OrganizationService
	edvSvc      *service.EndeavourService
	tskSvc      *service.TaskService
	resSvc      *service.ResourceService
	relSvc      *service.RelationService
	dmdSvc      *service.DemandService
	artSvc      *service.ArtifactService
	rtlSvc      *service.RitualService
	rtrSvc      *service.RitualRunService
	tplSvc      *service.TemplateService
	usrSvc      *service.UserService
	cmtSvc      *service.CommentService
	aprSvc      *service.ApprovalService
	dodSvc      *service.DodService
	msgSvc      *service.MessageService
	messageDB   *storage.MessageDB
	emailSender EmailSender

	// Security
	auditSvc           *security.AuditService
	entityAuditFn      func(*security.EntityChangeEntry) // optional entity CRUD logger
	entityChangeWriter *security.EntityChangeDBWriter
	rateLimiter        *security.RateLimiter

	// Velocity tracking
	velocity *velocityTracker

	// i18n
	i18n *i18n.Bundle

	// Config
	corsOrigins   []string
	agentTokenTTL time.Duration
	portalURL     string

	// Taskschmied callbacks (set from main if ritual executor is enabled).
	taskschmiedStatusFn  func() map[string]interface{}
	taskschmiedToggleFn  func(target string, disabled bool)

	// Deployment mode and onboarding gates.
	deploymentMode                string
	allowSelfRegistration         bool
	requireAgentEmailVerification bool
	requireAgentInterview         bool
}

// Config holds REST API configuration.
type Config struct {
	DB                *storage.DB
	Logger            *slog.Logger
	AuthService       *auth.Service
	AuditService        *security.AuditService
	EntityAuditLogger   *security.EntityAuditLogger
	EntityChangeWriter  *security.EntityChangeDBWriter
	EmailSender       EmailSender
	CORSOrigins       []string
	AgentTokenTTL     time.Duration
	PortalURL         string

	// Services
	OrgService *service.OrganizationService
	EdvService *service.EndeavourService
	TskService *service.TaskService
	ResService *service.ResourceService
	RelService *service.RelationService
	DmdService *service.DemandService
	ArtService *service.ArtifactService
	RtlService *service.RitualService
	RtrService *service.RitualRunService
	TplService *service.TemplateService
	UsrService *service.UserService
	CmtService *service.CommentService
	AprService *service.ApprovalService
	DodService *service.DodService
	MsgService  *service.MessageService
	MessageDB   *storage.MessageDB
	RateLimiter *security.RateLimiter

	// Deployment mode and onboarding gates.
	DeploymentMode                string
	AllowSelfRegistration         bool
	RequireAgentEmailVerification bool
	RequireAgentInterview         bool
}

// New creates a new REST API instance.
// If service fields on Config are nil, they are auto-created from DB and Logger.
func New(cfg *Config) *API {
	a := &API{
		db:          cfg.DB,
		logger:      cfg.Logger,
		authSvc:     cfg.AuthService,
		auditSvc:    cfg.AuditService,
		emailSender: cfg.EmailSender,
		entityAuditFn: func() func(*security.EntityChangeEntry) {
			if cfg.EntityChangeWriter != nil {
				return cfg.EntityChangeWriter.Log
			}
			if cfg.EntityAuditLogger != nil {
				return cfg.EntityAuditLogger.Log
			}
			return nil
		}(),
		entityChangeWriter: cfg.EntityChangeWriter,
		rateLimiter:        cfg.RateLimiter,
		velocity:           newVelocityTracker(),
		corsOrigins:        cfg.CORSOrigins,
		agentTokenTTL: cfg.AgentTokenTTL,
		portalURL:     cfg.PortalURL,
		orgSvc:      cfg.OrgService,
		edvSvc:      cfg.EdvService,
		tskSvc:      cfg.TskService,
		resSvc:      cfg.ResService,
		relSvc:      cfg.RelService,
		dmdSvc:      cfg.DmdService,
		artSvc:      cfg.ArtService,
		rtlSvc:      cfg.RtlService,
		rtrSvc:      cfg.RtrService,
		tplSvc:      cfg.TplService,
		usrSvc:      cfg.UsrService,
		cmtSvc:      cfg.CmtService,
		aprSvc:      cfg.AprService,
		dodSvc:      cfg.DodService,
		msgSvc:      cfg.MsgService,
		messageDB:   cfg.MessageDB,
		deploymentMode:                cfg.DeploymentMode,
		allowSelfRegistration:         cfg.AllowSelfRegistration,
		requireAgentEmailVerification: cfg.RequireAgentEmailVerification,
		requireAgentInterview:         cfg.RequireAgentInterview,
	}

	// Auto-create services when not provided (e.g. when MCP server
	// creates the API instance without pre-existing service references).
	if a.orgSvc == nil {
		a.orgSvc = service.NewOrganizationService(cfg.DB, cfg.Logger)
	}
	if a.edvSvc == nil {
		a.edvSvc = service.NewEndeavourService(cfg.DB, cfg.Logger)
	}
	if a.tskSvc == nil {
		a.tskSvc = service.NewTaskService(cfg.DB, cfg.Logger)
	}
	if a.resSvc == nil {
		a.resSvc = service.NewResourceService(cfg.DB, cfg.Logger)
	}
	if a.relSvc == nil {
		a.relSvc = service.NewRelationService(cfg.DB, cfg.Logger)
	}
	if a.dmdSvc == nil {
		a.dmdSvc = service.NewDemandService(cfg.DB, cfg.Logger)
	}
	if a.artSvc == nil {
		a.artSvc = service.NewArtifactService(cfg.DB, cfg.Logger)
	}
	if a.rtlSvc == nil {
		a.rtlSvc = service.NewRitualService(cfg.DB, cfg.Logger)
	}
	if a.rtrSvc == nil {
		a.rtrSvc = service.NewRitualRunService(cfg.DB, cfg.Logger)
	}
	if a.tplSvc == nil {
		a.tplSvc = service.NewTemplateService(cfg.DB, cfg.Logger)
	}
	if a.usrSvc == nil {
		a.usrSvc = service.NewUserService(cfg.DB, cfg.Logger)
	}
	if a.cmtSvc == nil {
		a.cmtSvc = service.NewCommentService(cfg.DB, cfg.Logger)
	}
	if a.aprSvc == nil {
		a.aprSvc = service.NewApprovalService(cfg.DB, cfg.Logger)
	}
	if a.dodSvc == nil {
		a.dodSvc = service.NewDodService(cfg.DB, cfg.Logger)
	}

	// Wire cross-service dependencies.
	a.orgSvc.SetEndeavourService(a.edvSvc)

	// Initialize i18n for report generation.
	if bundle, err := i18n.New(); err == nil {
		a.i18n = bundle
	}

	return a
}

// Close releases resources held by the API (background goroutines, etc.).
func (a *API) Close() {
	if a.velocity != nil {
		a.velocity.Close()
	}
}

// logEntityChange records a CRUD operation in the entity audit log.
// No-op if the entity audit logger is not configured.
// The metadata parameter carries new values for changed fields (e.g., {"status": "canceled"}).
func (a *API) logEntityChange(actorID, action, entityType, entityID, endeavourID string, fields []string, metadata map[string]interface{}) {
	if a.entityAuditFn == nil {
		return
	}
	a.entityAuditFn(&security.EntityChangeEntry{
		ActorID:     actorID,
		Action:      action,
		EntityType:  entityType,
		EntityID:    entityID,
		EndeavourID: endeavourID,
		Fields:      fields,
		Metadata:    metadata,
	})
}

// taskFieldValues extracts the new values for fields that were actually updated.
func taskFieldValues(f storage.UpdateTaskFields, updated []string) map[string]interface{} {
	if len(updated) == 0 {
		return nil
	}
	m := make(map[string]interface{}, len(updated))
	for _, name := range updated {
		switch name {
		case "status":
			if f.Status != nil {
				m["status"] = *f.Status
			}
		case "title":
			if f.Title != nil {
				m["title"] = *f.Title
			}
		case "description":
			if f.Description != nil {
				m["description"] = "(updated)"
			}
		case "owner_id":
			if f.OwnerID != nil {
				m["owner_id"] = *f.OwnerID
			}
		case "assignee_id":
			if f.AssigneeID != nil {
				m["assignee_id"] = *f.AssigneeID
			}
		case "endeavour_id":
			if f.EndeavourID != nil {
				m["endeavour_id"] = *f.EndeavourID
			}
		case "demand_id":
			if f.DemandID != nil {
				m["demand_id"] = *f.DemandID
			}
		case "estimate":
			if f.Estimate != nil {
				m["estimate"] = fmt.Sprintf("%.1fh", *f.Estimate)
			}
		case "actual":
			if f.Actual != nil {
				m["actual"] = fmt.Sprintf("%.1fh", *f.Actual)
			}
		case "due_date":
			if f.DueDate != nil {
				if *f.DueDate == "" {
					m["due_date"] = "(cleared)"
				} else {
					m["due_date"] = *f.DueDate
				}
			}
		case "canceled_reason":
			if f.CanceledReason != nil {
				m["canceled_reason"] = *f.CanceledReason
			}
		}
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

// demandFieldValues extracts the new values for fields that were actually updated.
func demandFieldValues(f storage.UpdateDemandFields, updated []string) map[string]interface{} {
	if len(updated) == 0 {
		return nil
	}
	m := make(map[string]interface{}, len(updated))
	for _, name := range updated {
		switch name {
		case "status":
			if f.Status != nil {
				m["status"] = *f.Status
			}
		case "title":
			if f.Title != nil {
				m["title"] = *f.Title
			}
		case "description":
			if f.Description != nil {
				m["description"] = "(updated)"
			}
		case "priority":
			if f.Priority != nil {
				m["priority"] = *f.Priority
			}
		case "type":
			if f.Type != nil {
				m["type"] = *f.Type
			}
		case "owner_id":
			if f.OwnerID != nil {
				m["owner_id"] = *f.OwnerID
			}
		case "endeavour_id":
			if f.EndeavourID != nil {
				m["endeavour_id"] = *f.EndeavourID
			}
		case "due_date":
			if f.DueDate != nil {
				if *f.DueDate == "" {
					m["due_date"] = "(cleared)"
				} else {
					m["due_date"] = *f.DueDate
				}
			}
		case "canceled_reason":
			if f.CanceledReason != nil {
				m["canceled_reason"] = *f.CanceledReason
			}
		}
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

// endeavourFieldValues extracts the new values for fields that were actually updated.
func endeavourFieldValues(f storage.UpdateEndeavourFields, updated []string) map[string]interface{} {
	if len(updated) == 0 {
		return nil
	}
	m := make(map[string]interface{}, len(updated))
	for _, name := range updated {
		switch name {
		case "status":
			if f.Status != nil {
				m["status"] = *f.Status
			}
		case "name":
			if f.Name != nil {
				m["name"] = *f.Name
			}
		case "description":
			if f.Description != nil {
				m["description"] = "(updated)"
			}
		case "timezone":
			if f.Timezone != nil {
				m["timezone"] = *f.Timezone
			}
		case "lang":
			if f.Lang != nil {
				m["lang"] = *f.Lang
			}
		case "start_date":
			if f.StartDate != nil {
				if *f.StartDate == "" {
					m["start_date"] = "(cleared)"
				} else {
					m["start_date"] = *f.StartDate
				}
			}
		case "end_date":
			if f.EndDate != nil {
				if *f.EndDate == "" {
					m["end_date"] = "(cleared)"
				} else {
					m["end_date"] = *f.EndDate
				}
			}
		}
	}
	if len(m) == 0 {
		return nil
	}
	return m
}
