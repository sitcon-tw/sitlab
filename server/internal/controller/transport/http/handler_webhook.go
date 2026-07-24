package httpserver

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"example.com/project-template/internal/controller/application/apperror"
	"example.com/project-template/internal/domain/board"
)

const webhookTimestampTolerance = 10 * time.Minute

type gitLabWebhookPayload struct {
	ObjectKind string `json:"object_kind"`
	EventType  string `json:"event_type"`
	EventName  string `json:"event_name"`
	GroupPath  string `json:"group_path"`
	Project    struct {
		PathWithNamespace string `json:"path_with_namespace"`
	} `json:"project"`
	ObjectAttributes struct {
		IID  int64  `json:"iid"`
		Type string `json:"type"`
	} `json:"object_attributes"`
}

func (h handler) receiveProjectWebhook(w http.ResponseWriter, r *http.Request) {
	h.receiveWebhook(w, r, "project", h.webhooks.ProjectSigningToken)
}

func (h handler) receiveGroupWebhook(w http.ResponseWriter, r *http.Request) {
	h.receiveWebhook(w, r, "group", h.webhooks.GroupSigningToken)
}

func (h handler) receiveWebhook(w http.ResponseWriter, r *http.Request, scope, signingToken string) {
	metricResult := "rejected"
	defer func() {
		if h.metrics != nil {
			h.metrics.WebhookDelivery(scope, metricResult)
		}
	}()
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeError(w, r, apperror.Malformed("webhook body is invalid"))
		return
	}
	if err := verifyGitLabWebhook(signingToken, r.Header, body, time.Now().UTC()); err != nil {
		writeError(w, r, apperror.Unauthorized("GITLAB_INVALID_WEBHOOK", "webhook signature is invalid"))
		return
	}
	var payload gitLabWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(w, r, apperror.Malformed("webhook body is invalid"))
		return
	}
	delivery := board.WebhookDelivery{
		ID: r.Header.Get("webhook-id"), Scope: scope,
		EventName: r.Header.Get("X-Gitlab-Event"), ReceivedAt: time.Now().UTC(),
	}
	if scope == "project" {
		if payload.Project.PathWithNamespace != h.webhooks.ProjectPath {
			writeError(w, r, apperror.Malformed("webhook project does not match the configured project"))
			return
		}
		if r.Header.Get("X-Gitlab-Event") != "Issue Hook" || payload.ObjectKind != "issue" ||
			(payload.ObjectAttributes.Type != "" && payload.ObjectAttributes.Type != "Issue") {
			metricResult = "ignored"
			writeJSON(w, http.StatusAccepted, map[string]any{"accepted": true, "duplicate": false})
			return
		}
		if payload.ObjectAttributes.IID <= 0 {
			writeError(w, r, apperror.Malformed("webhook issue IID is invalid"))
			return
		}
		delivery.EventKind = "issue"
		delivery.IssueIID = &payload.ObjectAttributes.IID
	} else {
		if payload.GroupPath != h.webhooks.GroupPath {
			writeError(w, r, apperror.Malformed("webhook group does not match the configured group"))
			return
		}
		if r.Header.Get("X-Gitlab-Event") != "Member Hook" {
			metricResult = "ignored"
			writeJSON(w, http.StatusAccepted, map[string]any{"accepted": true, "duplicate": false})
			return
		}
		if payload.EventName == "" {
			writeError(w, r, apperror.Malformed("webhook member event name is invalid"))
			return
		}
		delivery.EventKind = "member"
		delivery.EventName = payload.EventName
	}
	duplicate, err := h.sync.EnqueueWebhook(r.Context(), delivery)
	if err != nil {
		metricResult = "unavailable"
		writeError(w, r, apperror.Unavailable("webhook could not be queued"))
		return
	}
	if duplicate {
		metricResult = "duplicate"
	} else {
		metricResult = "queued"
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"accepted": true, "duplicate": duplicate})
}

func verifyGitLabWebhook(signingToken string, headers http.Header, body []byte, now time.Time) error {
	if !strings.HasPrefix(signingToken, "whsec_") {
		return fmt.Errorf("signing token is not configured")
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(signingToken, "whsec_"))
	if err != nil || len(key) != 32 {
		return fmt.Errorf("signing token is invalid")
	}
	messageID := headers.Get("webhook-id")
	timestampValue := headers.Get("webhook-timestamp")
	timestamp, err := strconv.ParseInt(timestampValue, 10, 64)
	if err != nil || messageID == "" {
		return fmt.Errorf("webhook delivery headers are invalid")
	}
	delta := now.Sub(time.Unix(timestamp, 0))
	if delta < -webhookTimestampTolerance || delta > webhookTimestampTolerance {
		return fmt.Errorf("webhook timestamp is outside the accepted window")
	}
	message := append([]byte(messageID+"."+timestampValue+"."), body...)
	digest := hmac.New(sha256.New, key)
	_, _ = digest.Write(message)
	expected := "v1," + base64.StdEncoding.EncodeToString(digest.Sum(nil))
	for _, signature := range strings.Fields(headers.Get("webhook-signature")) {
		if hmac.Equal([]byte(expected), []byte(signature)) {
			return nil
		}
	}
	return fmt.Errorf("webhook signature does not match")
}
