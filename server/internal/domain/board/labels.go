package board

import (
	"slices"
	"strings"
)

// TeamLabelPrefix namespaces the labels the bundled directory owns.
const TeamLabelPrefix = "Team::"

var legacyStatusListKeys = map[string]string{
	"Wating":  "wating",
	"Waiting": "wating",
	"Inbox":   "inbox",
	"To Do":   "todo",
	"Todo":    "todo",
	"Doing":   "doing",
	"Review":  "review",
	"Closed":  "closed",
}

var DeprecatedLabels = []string{
	"Doing", "Inbox", "Review", "Status::Inbox", "To Do", "Todo", "Wating",
	"組別::總召", "組別::行政", "組別::開發",
}

// DeprecatedLabel reports whether a label belongs to the legacy workflow or to
// the lifecycle namespace the board manages itself. It is the single source of
// truth for both the read filter and the write guard.
func DeprecatedLabel(name string) bool {
	if _, legacyStatus := LegacyStatusListKey(name); legacyStatus {
		return true
	}
	if strings.HasPrefix(name, "Status::") {
		return true
	}
	return slices.Contains(DeprecatedLabels, name)
}

// ReservedLabel reports whether a label is owned by the board configuration
// rather than by users, and therefore cannot be created, renamed, or deleted.
//
// The whole Team:: prefix is reserved, not only the configured names: a
// user-created Team::新組 would not match issueTeam during sync, so it would
// behave like an ordinary label until someone added it to board-directory.yml,
// and would then start reassigning cards.
func ReservedLabel(name string, teamLabels []string) bool {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return true
	}
	if strings.HasPrefix(trimmed, TeamLabelPrefix) {
		return true
	}
	return DeprecatedLabel(trimmed) || slices.Contains(teamLabels, trimmed)
}

func LegacyStatusListKey(label string) (string, bool) {
	key, ok := legacyStatusListKeys[label]
	return key, ok
}

func NormalizeLabels(labels []string) ([]string, bool) {
	result := make([]string, 0, len(labels))
	seen := make(map[string]struct{}, len(labels))
	for _, raw := range labels {
		label := strings.TrimSpace(raw)
		if label == "" {
			return nil, false
		}
		if _, exists := seen[label]; exists {
			continue
		}
		seen[label] = struct{}{}
		result = append(result, label)
	}
	return result, true
}

func CanonicalLabels(existing []string, teamLabel string, teamLabels []string) []string {
	reserved := make(map[string]struct{}, len(teamLabels)+len(DeprecatedLabels))
	for _, label := range teamLabels {
		reserved[label] = struct{}{}
	}
	for _, label := range DeprecatedLabels {
		reserved[label] = struct{}{}
	}
	labels := make([]string, 0, len(existing)+1)
	for _, label := range existing {
		_, isReserved := reserved[label]
		_, isLegacyStatus := LegacyStatusListKey(label)
		if !isReserved && !isLegacyStatus && !strings.HasPrefix(label, "Status::") {
			labels = append(labels, label)
		}
	}
	if teamLabel != "" {
		labels = append(labels, teamLabel)
	}
	return labels
}
