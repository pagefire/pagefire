package store

import (
	"context"
	"errors"
	"time"
)

// Sentinel errors for store operations.
var (
	ErrNotFound     = errors.New("not found")
	ErrDuplicateKey = errors.New("duplicate key")
	ErrConflict     = errors.New("conflict")
)

// Store is the top-level repository interface. Each sub-store is accessed via
// a method so implementations can share a single database connection.
type Store interface {
	// Shared Platform (v0.1)
	Users() UserStore
	Teams() TeamStore
	Notifications() NotificationStore
	Incidents() IncidentStore

	// Pillar 1: On-Call (v0.1)
	Alerts() AlertStore
	Services() ServiceStore
	EscalationPolicies() EscalationPolicyStore
	Schedules() ScheduleStore

	// Lifecycle
	Ping(ctx context.Context) error
	Migrate(ctx context.Context) error
	Close() error

	// Cleanup — bulk purge of old data
	PurgeResolvedAlerts(ctx context.Context, olderThan time.Time) (int64, error)
	PurgeOldNotifications(ctx context.Context, olderThan time.Time) (int64, error)
	PurgeExpiredOverrides(ctx context.Context) (int64, error)
}

// TeamStore manages teams and team membership.
type TeamStore interface {
	Create(ctx context.Context, t *Team) error
	Get(ctx context.Context, id string) (*Team, error)
	List(ctx context.Context) ([]Team, error)
	Update(ctx context.Context, t *Team) error
	Delete(ctx context.Context, id string) error

	AddMember(ctx context.Context, teamID, userID, role string) error
	RemoveMember(ctx context.Context, teamID, userID string) error
	ListMembers(ctx context.Context, teamID string) ([]TeamMember, error)
	ListTeamsForUser(ctx context.Context, userID string) ([]Team, error)
}

// UserStore manages users, contact methods, and notification rules.
type UserStore interface {
	Create(ctx context.Context, u *User) error
	Get(ctx context.Context, id string) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	List(ctx context.Context) ([]User, error)
	Update(ctx context.Context, u *User) error
	Delete(ctx context.Context, id string) error
	SetPassword(ctx context.Context, id string, passwordHash string) error
	SetLastLogin(ctx context.Context, id string) error
	CountUsers(ctx context.Context) (int, error)

	CreateContactMethod(ctx context.Context, cm *ContactMethod) error
	ListContactMethods(ctx context.Context, userID string) ([]ContactMethod, error)
	DeleteContactMethod(ctx context.Context, id string) error

	CreateNotificationRule(ctx context.Context, nr *NotificationRule) error
	ListNotificationRules(ctx context.Context, userID string) ([]NotificationRule, error)
	DeleteNotificationRule(ctx context.Context, id string) error

	// Invite tokens
	CreateInviteToken(ctx context.Context, t *InviteToken) error
	GetInviteTokenByHash(ctx context.Context, tokenHash string) (*InviteToken, error)
	UseInviteToken(ctx context.Context, id string) error

	// API tokens
	CreateAPIToken(ctx context.Context, t *APIToken, tokenHash string) error
	ListAPITokens(ctx context.Context, userID string) ([]APIToken, error)
	GetAPITokenByHash(ctx context.Context, tokenHash string) (*APIToken, error)
	RevokeAPIToken(ctx context.Context, id string) error
	TouchAPIToken(ctx context.Context, id string) error
}

// ServiceStore manages services and integration keys.
type ServiceStore interface {
	Create(ctx context.Context, s *Service) error
	Get(ctx context.Context, id string) (*Service, error)
	List(ctx context.Context) ([]Service, error)
	ListByTeam(ctx context.Context, teamID string) ([]Service, error)
	Update(ctx context.Context, s *Service) error
	Delete(ctx context.Context, id string) error

	CreateIntegrationKey(ctx context.Context, ik *IntegrationKey) error
	GetIntegrationKey(ctx context.Context, id string) (*IntegrationKey, error)
	ListIntegrationKeys(ctx context.Context, serviceID string) ([]IntegrationKey, error)
	GetIntegrationKeyBySecret(ctx context.Context, secret string) (*IntegrationKey, error)
	DeleteIntegrationKey(ctx context.Context, id string) error

	CreateRoutingRule(ctx context.Context, rule *RoutingRule) error
	ListRoutingRules(ctx context.Context, serviceID string) ([]RoutingRule, error)
	DeleteRoutingRule(ctx context.Context, id string) error
}

// EscalationPolicyStore manages escalation policies, steps, and targets.
type EscalationPolicyStore interface {
	Create(ctx context.Context, ep *EscalationPolicy) error
	Get(ctx context.Context, id string) (*EscalationPolicy, error)
	List(ctx context.Context) ([]EscalationPolicy, error)
	ListByTeam(ctx context.Context, teamID string) ([]EscalationPolicy, error)
	Update(ctx context.Context, ep *EscalationPolicy) error
	Delete(ctx context.Context, id string) error

	CreateStep(ctx context.Context, step *EscalationStep) error
	ListSteps(ctx context.Context, policyID string) ([]EscalationStep, error)
	DeleteStep(ctx context.Context, id string) error

	CreateStepTarget(ctx context.Context, target *EscalationStepTarget) error
	ListStepTargets(ctx context.Context, stepID string) ([]EscalationStepTarget, error)
	DeleteStepTarget(ctx context.Context, id string) error

	// GetFullPolicy returns a snapshot of the entire escalation policy tree,
	// suitable for serializing as JSON on an alert (ADR-9).
	GetFullPolicy(ctx context.Context, id string) (*EscalationSnapshot, error)
}

// ScheduleStore manages schedules, rotations, participants, and overrides.
type ScheduleStore interface {
	Create(ctx context.Context, s *Schedule) error
	Get(ctx context.Context, id string) (*Schedule, error)
	List(ctx context.Context) ([]Schedule, error)
	ListByTeam(ctx context.Context, teamID string) ([]Schedule, error)
	Update(ctx context.Context, s *Schedule) error
	Delete(ctx context.Context, id string) error

	CreateRotation(ctx context.Context, r *Rotation) error
	GetRotation(ctx context.Context, id string) (*Rotation, error)
	ListRotations(ctx context.Context, scheduleID string) ([]Rotation, error)
	DeleteRotation(ctx context.Context, id string) error

	CreateParticipant(ctx context.Context, p *RotationParticipant) error
	ListParticipants(ctx context.Context, rotationID string) ([]RotationParticipant, error)
	DeleteParticipant(ctx context.Context, id string) error

	CreateOverride(ctx context.Context, o *ScheduleOverride) error
	ListOverrides(ctx context.Context, scheduleID string) ([]ScheduleOverride, error)
	ListActiveOverrides(ctx context.Context, scheduleID string, at time.Time) ([]ScheduleOverride, error)
	DeleteOverride(ctx context.Context, id string) error
}

// AlertStore manages alerts and alert logs.
type AlertStore interface {
	Create(ctx context.Context, a *Alert) error
	Get(ctx context.Context, id string) (*Alert, error)
	List(ctx context.Context, filter AlertFilter) ([]Alert, error)
	Acknowledge(ctx context.Context, id string, userID string) error
	Resolve(ctx context.Context, id string) error

	// FindPendingEscalations returns triggered alerts whose next_escalation_at <= before.
	FindPendingEscalations(ctx context.Context, before time.Time) ([]Alert, error)
	UpdateEscalationStep(ctx context.Context, id string, step int, loopCount int, nextAt time.Time) error

	CreateLog(ctx context.Context, log *AlertLog) error
	ListLogs(ctx context.Context, alertID string) ([]AlertLog, error)
}

// NotificationStore manages the notification dispatch queue.
type NotificationStore interface {
	Enqueue(ctx context.Context, n *Notification) error
	FindPending(ctx context.Context, limit int) ([]Notification, error)
	MarkSending(ctx context.Context, id string) error
	MarkSent(ctx context.Context, id string, providerID string) error
	MarkFailed(ctx context.Context, id string) error
	IncrementAttempts(ctx context.Context, id string, nextAt time.Time) error
}

// IncidentStore manages incidents and incident updates.
type IncidentStore interface {
	Create(ctx context.Context, inc *Incident) error
	Get(ctx context.Context, id string) (*Incident, error)
	List(ctx context.Context, filter IncidentFilter) ([]Incident, error)
	Update(ctx context.Context, inc *Incident) error

	AddService(ctx context.Context, incidentID, serviceID string) error
	ListServices(ctx context.Context, incidentID string) ([]string, error)

	LinkAlert(ctx context.Context, incidentID, alertID string) error
	UnlinkAlert(ctx context.Context, incidentID, alertID string) error
	ListAlerts(ctx context.Context, incidentID string) ([]*Alert, error)
	GetIncidentForAlert(ctx context.Context, alertID string) (*Incident, error)

	CreateUpdate(ctx context.Context, u *IncidentUpdate) error
	ListUpdates(ctx context.Context, incidentID string) ([]IncidentUpdate, error)
}
