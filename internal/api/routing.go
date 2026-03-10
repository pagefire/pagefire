package api

import (
	"regexp"
	"strings"

	"github.com/pagefire/pagefire/internal/store"
)

// routeAlert evaluates routing rules against an alert's content and returns
// the escalation policy ID to use. Rules are evaluated in priority order;
// the first match wins. If no rule matches, the service's default
// escalation_policy_id is returned.
func routeAlert(svc *store.Service, rules []store.RoutingRule, summary, details, source string) string {
	for _, rule := range rules {
		var field string
		switch rule.ConditionField {
		case "summary":
			field = summary
		case "details":
			field = details
		case "source":
			field = source
		default:
			continue
		}

		if matchesCondition(field, rule.ConditionMatchType, rule.ConditionValue) {
			return rule.EscalationPolicyID
		}
	}
	return svc.EscalationPolicyID
}

// matchesCondition checks if a field value matches a condition.
func matchesCondition(field, matchType, value string) bool {
	switch matchType {
	case "contains":
		return strings.Contains(strings.ToLower(field), strings.ToLower(value))
	case "regex":
		re, err := regexp.Compile(value)
		if err != nil {
			return false
		}
		return re.MatchString(field)
	default:
		return false
	}
}
