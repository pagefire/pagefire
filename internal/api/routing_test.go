package api

import (
	"testing"

	"github.com/pagefire/pagefire/internal/store"
)

func TestRouteAlert_NoRules(t *testing.T) {
	svc := &store.Service{EscalationPolicyID: "default-ep"}
	got := routeAlert(svc, nil, "cpu high", "details", "integration")
	if got != "default-ep" {
		t.Errorf("got %q, want %q", got, "default-ep")
	}
}

func TestRouteAlert_ContainsMatch(t *testing.T) {
	svc := &store.Service{EscalationPolicyID: "default-ep"}
	rules := []store.RoutingRule{
		{ConditionField: "summary", ConditionMatchType: "contains", ConditionValue: "database", EscalationPolicyID: "db-ep"},
	}

	got := routeAlert(svc, rules, "Database connection timeout", "details", "integration")
	if got != "db-ep" {
		t.Errorf("got %q, want %q", got, "db-ep")
	}
}

func TestRouteAlert_ContainsCaseInsensitive(t *testing.T) {
	svc := &store.Service{EscalationPolicyID: "default-ep"}
	rules := []store.RoutingRule{
		{ConditionField: "summary", ConditionMatchType: "contains", ConditionValue: "CRITICAL", EscalationPolicyID: "crit-ep"},
	}

	got := routeAlert(svc, rules, "critical: disk full", "details", "integration")
	if got != "crit-ep" {
		t.Errorf("got %q, want %q", got, "crit-ep")
	}
}

func TestRouteAlert_ContainsNoMatch(t *testing.T) {
	svc := &store.Service{EscalationPolicyID: "default-ep"}
	rules := []store.RoutingRule{
		{ConditionField: "summary", ConditionMatchType: "contains", ConditionValue: "database", EscalationPolicyID: "db-ep"},
	}

	got := routeAlert(svc, rules, "cpu high on web-01", "details", "integration")
	if got != "default-ep" {
		t.Errorf("got %q, want %q", got, "default-ep")
	}
}

func TestRouteAlert_RegexMatch(t *testing.T) {
	svc := &store.Service{EscalationPolicyID: "default-ep"}
	rules := []store.RoutingRule{
		{ConditionField: "summary", ConditionMatchType: "regex", ConditionValue: `^(CRITICAL|FATAL):`, EscalationPolicyID: "crit-ep"},
	}

	got := routeAlert(svc, rules, "CRITICAL: out of memory", "details", "integration")
	if got != "crit-ep" {
		t.Errorf("got %q, want %q", got, "crit-ep")
	}
}

func TestRouteAlert_RegexNoMatch(t *testing.T) {
	svc := &store.Service{EscalationPolicyID: "default-ep"}
	rules := []store.RoutingRule{
		{ConditionField: "summary", ConditionMatchType: "regex", ConditionValue: `^CRITICAL:`, EscalationPolicyID: "crit-ep"},
	}

	got := routeAlert(svc, rules, "WARNING: high latency", "details", "integration")
	if got != "default-ep" {
		t.Errorf("got %q, want %q", got, "default-ep")
	}
}

func TestRouteAlert_InvalidRegexFallsThrough(t *testing.T) {
	svc := &store.Service{EscalationPolicyID: "default-ep"}
	rules := []store.RoutingRule{
		{ConditionField: "summary", ConditionMatchType: "regex", ConditionValue: `[invalid`, EscalationPolicyID: "bad-ep"},
	}

	got := routeAlert(svc, rules, "anything", "details", "integration")
	if got != "default-ep" {
		t.Errorf("invalid regex should fall through to default, got %q", got)
	}
}

func TestRouteAlert_PriorityOrder(t *testing.T) {
	svc := &store.Service{EscalationPolicyID: "default-ep"}
	rules := []store.RoutingRule{
		{Priority: 0, ConditionField: "summary", ConditionMatchType: "contains", ConditionValue: "critical", EscalationPolicyID: "crit-ep"},
		{Priority: 1, ConditionField: "summary", ConditionMatchType: "contains", ConditionValue: "critical", EscalationPolicyID: "other-ep"},
	}

	// First rule should win.
	got := routeAlert(svc, rules, "critical: disk full", "details", "integration")
	if got != "crit-ep" {
		t.Errorf("got %q, want %q (first rule should win)", got, "crit-ep")
	}
}

func TestRouteAlert_DetailsField(t *testing.T) {
	svc := &store.Service{EscalationPolicyID: "default-ep"}
	rules := []store.RoutingRule{
		{ConditionField: "details", ConditionMatchType: "contains", ConditionValue: "production", EscalationPolicyID: "prod-ep"},
	}

	got := routeAlert(svc, rules, "generic alert", "production environment affected", "integration")
	if got != "prod-ep" {
		t.Errorf("got %q, want %q", got, "prod-ep")
	}
}

func TestRouteAlert_SourceField(t *testing.T) {
	svc := &store.Service{EscalationPolicyID: "default-ep"}
	rules := []store.RoutingRule{
		{ConditionField: "source", ConditionMatchType: "contains", ConditionValue: "grafana", EscalationPolicyID: "grafana-ep"},
	}

	got := routeAlert(svc, rules, "alert", "details", "grafana")
	if got != "grafana-ep" {
		t.Errorf("got %q, want %q", got, "grafana-ep")
	}
}

func TestRouteAlert_UnknownFieldSkipped(t *testing.T) {
	svc := &store.Service{EscalationPolicyID: "default-ep"}
	rules := []store.RoutingRule{
		{ConditionField: "invalid_field", ConditionMatchType: "contains", ConditionValue: "test", EscalationPolicyID: "bad-ep"},
	}

	got := routeAlert(svc, rules, "test alert", "test details", "integration")
	if got != "default-ep" {
		t.Errorf("unknown field should be skipped, got %q", got)
	}
}

func TestRouteAlert_UnknownMatchTypeSkipped(t *testing.T) {
	svc := &store.Service{EscalationPolicyID: "default-ep"}
	rules := []store.RoutingRule{
		{ConditionField: "summary", ConditionMatchType: "exact", ConditionValue: "test", EscalationPolicyID: "bad-ep"},
	}

	got := routeAlert(svc, rules, "test", "details", "integration")
	if got != "default-ep" {
		t.Errorf("unknown match type should be skipped, got %q", got)
	}
}

func TestRouteAlert_MultipleRulesFirstMatchWins(t *testing.T) {
	svc := &store.Service{EscalationPolicyID: "default-ep"}
	rules := []store.RoutingRule{
		{Priority: 0, ConditionField: "summary", ConditionMatchType: "contains", ConditionValue: "network", EscalationPolicyID: "network-ep"},
		{Priority: 1, ConditionField: "details", ConditionMatchType: "contains", ConditionValue: "production", EscalationPolicyID: "prod-ep"},
	}

	// Both would match, but first rule wins.
	got := routeAlert(svc, rules, "network timeout", "production cluster", "integration")
	if got != "network-ep" {
		t.Errorf("got %q, want %q (first match wins)", got, "network-ep")
	}

	// Only second rule matches.
	got = routeAlert(svc, rules, "cpu spike", "production cluster", "integration")
	if got != "prod-ep" {
		t.Errorf("got %q, want %q", got, "prod-ep")
	}
}
