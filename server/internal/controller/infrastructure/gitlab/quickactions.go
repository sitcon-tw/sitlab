package gitlab

import (
	"context"
	"strconv"

	appactivity "example.com/project-template/internal/controller/application/cardactivity"
)

const quickActionsQuery = `query QuickActions($fullPath: ID!, $iids: [String!]) {
  project(fullPath: $fullPath) {
    workItems(first: 1, iids: $iids, types: [ISSUE]) {
      nodes {
        availableQuickActions { name aliases params description warning icon }
      }
    }
  }
}`

type quickActionsData struct {
	Project struct {
		WorkItems struct {
			Nodes []struct {
				AvailableQuickActions []appactivity.QuickActionCommand `json:"availableQuickActions"`
			} `json:"nodes"`
		} `json:"workItems"`
	} `json:"project"`
}

// QuickActions asks GitLab for the commands available to the acting user and
// current work item. With no IID (Quick Create), GitLab evaluates a real issue
// from the same fixed project; the versioned fallback only covers an empty
// project and keeps command discovery useful until the first card exists.
func (c *Client) QuickActions(ctx context.Context, issueIID int64, actorAccessToken string) ([]appactivity.QuickActionCommand, error) {
	var iids []string
	if issueIID > 0 {
		iids = []string{strconv.FormatInt(issueIID, 10)}
	}
	var data quickActionsData
	if err := c.graphQL(ctx, quickActionsQuery, map[string]any{"fullPath": c.config.ProjectPath, "iids": iids}, "", actorAccessToken, &data); err != nil {
		return nil, err
	}
	if len(data.Project.WorkItems.Nodes) == 0 {
		return createContextQuickActions(), nil
	}
	return data.Project.WorkItems.Nodes[0].AvailableQuickActions, nil
}

// GitLab cannot expose per-user availableQuickActions until a work item exists.
// Keep this create-only compatibility catalog flat and data-driven so updating
// it for a GitLab release is a mechanical change, not another parser branch.
func createContextQuickActions() []appactivity.QuickActionCommand {
	params := map[string][]string{
		"add_child": {"#item"}, "add_contacts": {"[contact:email]"}, "add_email": {"email"},
		"assign": {"@user"}, "blocked_by": {"#item"}, "blocks": {"#item"}, "board_move": {"~label"},
		"checkin_reminder": {"<interval>"}, "clone": {"project/path"}, "copy_metadata": {"#item"},
		"due": {"<date>"}, "duplicate": {"#issue"}, "epic": {"&epic"}, "estimate": {"<time>"},
		"health_status": {"<status>"}, "iteration": {"*iteration"}, "label": {"~label"}, "link": {"<URL>"},
		"milestone": {"%milestone"}, "move": {"project/path"}, "promote_to": {"<type>"}, "react": {":emoji:"},
		"reassign": {"@user"}, "relate": {"#item"}, "remove_child": {"#item"}, "set_parent": {"#item"},
		"severity": {"<severity>"}, "spend": {"<time>", "[<date>]"}, "status": {"\"status\""},
		"target_branch": {"<branch>"}, "timeline": {"<event>", "|", "<date>"}, "title": {"<new title>"},
		"type": {"\"type\""}, "unassign": {"[@user]"}, "unlabel": {"~label"}, "weight": {"<value>"},
		"zoom": {"<Zoom URL>"},
	}
	names := []string{
		"add_child", "add_contacts", "add_email", "approve", "assign", "assign_reviewer", "award", "blocked_by", "blocks", "board_move",
		"checkin_reminder", "clear_health_status", "clear_weight", "clone", "close", "confidential", "convert_to_ticket", "copy_metadata",
		"create_merge_request", "done", "draft", "due", "duplicate", "epic", "estimate", "health_status", "internal_note", "iteration", "label",
		"link", "lock", "merge", "milestone", "move", "page", "promote_to", "promote_to_incident", "publish", "react", "ready", "reassign",
		"reassign_reviewer", "rebase", "relabel", "relate", "remove_child", "remove_contacts", "remove_due_date", "remove_email", "remove_estimate",
		"remove_iteration", "remove_milestone", "remove_parent", "remove_time_spent", "remove_zoom", "reopen", "request_review", "run_pipeline",
		"set_parent", "severity", "shrug", "spend", "status", "submit_review", "subscribe", "tableflip", "target_branch", "timeline", "title", "todo",
		"type", "unapprove", "unassign", "unassign_reviewer", "unlabel", "unlink", "unlock", "unsubscribe", "weight", "zoom",
	}
	result := make([]appactivity.QuickActionCommand, 0, len(names))
	for _, name := range names {
		result = append(result, appactivity.QuickActionCommand{Name: name, Params: params[name]})
	}
	return result
}
