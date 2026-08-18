package board

import "strings"

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
