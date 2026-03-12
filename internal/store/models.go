package store

import "time"

// Status constants for alerts.
const (
	AlertStatusTriggered    = "triggered"
	AlertStatusAcknowledged = "acknowledged"
	AlertStatusResolved     = "resolved"
)

// Status constants for incidents.
const (
	IncidentStatusTriggered     = "triggered"
	IncidentStatusAcknowledged  = "acknowledged"
	IncidentStatusInvestigating = "investigating"
	IncidentStatusIdentified    = "identified"
	IncidentStatusMonitoring    = "monitoring"
	IncidentStatusResolved      = "resolved"
)

// Severity constants for incidents.
const (
	SeverityCritical = "critical"
	SeverityMajor    = "major"
	SeverityMinor    = "minor"
)

// Notification status constants.
const (
	NotificationStatusPending = "pending"
	NotificationStatusSending = "sending"
	NotificationStatusSent    = "sent"
	NotificationStatusFailed  = "failed"
)

// Rotation type constants.
const (
	RotationTypeDaily  = "daily"
	RotationTypeWeekly = "weekly"
	RotationTypeCustom = "custom"
)

// Target type constants for escalation step targets.
const (
	TargetTypeUser     = "user"
	TargetTypeSchedule = "schedule"
)

// Team represents an organizational unit for grouping resources.
type Team struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// TeamMember links a user to a team with a role.
type TeamMember struct {
	TeamID string `json:"team_id"`
	UserID string `json:"user_id"`
	Role   string `json:"role"` // "admin", "member"
}

// User represents a platform user.
type User struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Email          string    `json:"email"`
	Role           string    `json:"role"`
	Timezone       string    `json:"timezone"`
	AvatarURL      string    `json:"avatar_url,omitempty"`
	PasswordHash   string    `json:"-"`
	AuthProvider   *string   `json:"auth_provider,omitempty"`
	AuthProviderID *string   `json:"-"`
	IsActive       bool      `json:"is_active"`
	LastLogin      *time.Time `json:"last_login,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// UserRole constants.
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// InviteToken represents a one-time token for a new user to set their password.
type InviteToken struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	TokenHash string    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// APIToken represents a long-lived token for programmatic API access.
type APIToken struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	Name      string     `json:"name"`
	Prefix    string     `json:"prefix"`
	CreatedAt time.Time  `json:"created_at"`
	LastUsed  *time.Time `json:"last_used,omitempty"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

// ContactMethod represents a user's notification contact method.
type ContactMethod struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Type      string    `json:"type"` // "email", "sms", "slack_dm", "webhook"
	Value     string    `json:"value"`
	Verified  bool      `json:"verified"`
	CreatedAt time.Time `json:"created_at"`
}

// NotificationRule defines when/how a user gets notified for alerts.
type NotificationRule struct {
	ID              string `json:"id"`
	UserID          string `json:"user_id"`
	ContactMethodID string `json:"contact_method_id"`
	DelayMinutes    int    `json:"delay_minutes"`
}

// Service represents a thing that can break.
type Service struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Description        string    `json:"description,omitempty"`
	EscalationPolicyID string    `json:"escalation_policy_id"`
	TeamID             *string   `json:"team_id,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}

// IntegrationKey represents an inbound webhook endpoint for a service.
type IntegrationKey struct {
	ID        string    `json:"id"`
	ServiceID string    `json:"service_id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"` // "generic", "grafana", "prometheus"
	Secret    string    `json:"secret"`
	CreatedAt time.Time `json:"created_at"`
}

// EscalationPolicy defines a named, reusable escalation chain.
type EscalationPolicy struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Repeat      int       `json:"repeat"` // 0-5 times to loop
	TeamID      *string   `json:"team_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// EscalationStep is an ordered step within an escalation policy.
type EscalationStep struct {
	ID                 string `json:"id"`
	EscalationPolicyID string `json:"escalation_policy_id"`
	StepNumber         int    `json:"step_number"`
	DelayMinutes       int    `json:"delay_minutes"`
}

// EscalationStepTarget is a target within an escalation step.
type EscalationStepTarget struct {
	ID               string `json:"id"`
	EscalationStepID string `json:"escalation_step_id"`
	TargetType       string `json:"target_type"` // "user", "schedule"
	TargetID         string `json:"target_id"`
}

// Schedule is a named on-call schedule.
type Schedule struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Timezone    string    `json:"timezone"`
	TeamID      *string   `json:"team_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Rotation is a rotation within a schedule.
type Rotation struct {
	ID          string    `json:"id"`
	ScheduleID  string    `json:"schedule_id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"` // "daily", "weekly", "custom"
	ShiftLength int       `json:"shift_length"`
	StartTime   time.Time `json:"start_time"`
	HandoffTime string    `json:"handoff_time"` // "09:00"
	CreatedAt   time.Time `json:"created_at"`
}

// RotationParticipant is an ordered participant in a rotation.
type RotationParticipant struct {
	ID         string `json:"id"`
	RotationID string `json:"rotation_id"`
	UserID     string `json:"user_id"`
	Position   int    `json:"position"`
}

// ScheduleOverride is a temporary user swap in a schedule.
type ScheduleOverride struct {
	ID           string    `json:"id"`
	ScheduleID   string    `json:"schedule_id"`
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	ReplaceUser  string    `json:"replace_user"`
	OverrideUser string    `json:"override_user"`
	CreatedAt    time.Time `json:"created_at"`
}

// Alert is the core alert object.
type Alert struct {
	ID                       string     `json:"id"`
	ServiceID                string     `json:"service_id"`
	Status                   string     `json:"status"`
	Summary                  string     `json:"summary"`
	Details                  string     `json:"details,omitempty"`
	Source                   string     `json:"source"` // "api", "integration", "monitor"
	DeduplicationKey         string     `json:"dedup_key,omitempty"`
	GroupKey                 string     `json:"group_key,omitempty"`
	EscalationPolicySnapshot string     `json:"escalation_policy_snapshot"` // JSON blob
	EscalationStep           int        `json:"escalation_step"`
	LoopCount                int        `json:"loop_count"`
	NextEscalationAt         *time.Time `json:"next_escalation_at,omitempty"`
	AcknowledgedBy           *string    `json:"acknowledged_by,omitempty"`
	AcknowledgedAt           *time.Time `json:"acknowledged_at,omitempty"`
	ResolvedAt               *time.Time `json:"resolved_at,omitempty"`
	CreatedAt                time.Time  `json:"created_at"`
}

// AlertLog is an audit trail entry for an alert lifecycle event.
type AlertLog struct {
	ID        string    `json:"id"`
	AlertID   string    `json:"alert_id"`
	Event     string    `json:"event"` // "created", "escalated", "acknowledged", "resolved"
	Message   string    `json:"message"`
	UserID    *string   `json:"user_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Notification is a queued notification for dispatch.
type Notification struct {
	ID              string     `json:"id"`
	AlertID         string     `json:"alert_id,omitempty"`
	UserID          string     `json:"user_id,omitempty"`
	UserName        string     `json:"user_name,omitempty" db:"-"` // resolved at dispatch time, not persisted
	ContactMethodID string     `json:"contact_method_id,omitempty"`
	Type            string     `json:"type"` // "alert", "subscriber"
	DestinationType string     `json:"destination_type"`
	Destination     string     `json:"destination"`
	Subject         string     `json:"subject"`
	Body            string     `json:"body"`
	Status          string     `json:"status"`
	Attempts        int        `json:"attempts"`
	MaxAttempts     int        `json:"max_attempts"`
	NextAttemptAt   *time.Time `json:"next_attempt_at,omitempty"`
	SentAt          *time.Time `json:"sent_at,omitempty"`
	ProviderID      string     `json:"provider_id,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// Incident represents an incident spanning one or more services.
type Incident struct {
	ID         string     `json:"id"`
	Title      string     `json:"title"`
	Status     string     `json:"status"`
	Severity   string     `json:"severity"`
	Summary    string     `json:"summary,omitempty"`
	Source     string     `json:"source"` // "manual", "alert", "monitor"
	CreatedBy  string     `json:"created_by,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

// IncidentService links an incident to a service.
type IncidentService struct {
	IncidentID string `json:"incident_id"`
	ServiceID  string `json:"service_id"`
}

// IncidentUpdate is a timeline entry on an incident.
type IncidentUpdate struct {
	ID            string    `json:"id"`
	IncidentID    string    `json:"incident_id"`
	Status        string    `json:"status,omitempty"`
	Message       string    `json:"message"`
	CreatedBy     string    `json:"created_by,omitempty"`
	CreatedByName string    `json:"created_by_name,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// EscalationSnapshot is a frozen copy of an escalation policy stored on an alert.
type EscalationSnapshot struct {
	PolicyID   string                   `json:"policy_id"`
	PolicyName string                   `json:"policy_name"`
	Repeat     int                      `json:"repeat"`
	Steps      []EscalationStepSnapshot `json:"steps"`
}

// EscalationStepSnapshot is a frozen copy of an escalation step.
type EscalationStepSnapshot struct {
	StepNumber   int              `json:"step_number"`
	DelayMinutes int              `json:"delay_minutes"`
	Targets      []TargetSnapshot `json:"targets"`
}

// TargetSnapshot is a frozen copy of an escalation step target.
type TargetSnapshot struct {
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	TargetName string `json:"target_name"`
}

// RoutingRule defines a content-based routing rule for a service.
// Rules are evaluated in priority order; the first match determines
// which escalation policy is used. If no rule matches, the service's
// default escalation_policy_id is used.
type RoutingRule struct {
	ID                 string    `json:"id"`
	ServiceID          string    `json:"service_id"`
	Priority           int       `json:"priority"`
	ConditionField     string    `json:"condition_field"`     // "summary", "details", "source"
	ConditionMatchType string    `json:"condition_match_type"` // "contains", "regex"
	ConditionValue     string    `json:"condition_value"`
	EscalationPolicyID string    `json:"escalation_policy_id"`
	CreatedAt          time.Time `json:"created_at"`
}

// Filter types for list queries.

type AlertFilter struct {
	Status    string `json:"status,omitempty"`
	ServiceID string `json:"service_id,omitempty"`
	GroupKey  string `json:"group_key,omitempty"`
	Search    string `json:"search,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	Offset    int    `json:"offset,omitempty"`
}

type IncidentFilter struct {
	Status    string `json:"status,omitempty"`
	ServiceID string `json:"service_id,omitempty"`
	Search    string `json:"search,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	Offset    int    `json:"offset,omitempty"`
}
