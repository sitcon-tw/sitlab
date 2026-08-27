package httpserver

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appsync "example.com/project-template/internal/controller/application/sync"
	"example.com/project-template/internal/domain/board"
)

type webhookSyncFake struct {
	delivery board.WebhookDelivery
}

func (*webhookSyncFake) RequestRefresh() time.Time { return time.Time{} }
func (*webhookSyncFake) Delta(context.Context, appsync.DeltaQuery) (board.SyncDelta, error) {
	return board.SyncDelta{}, nil
}
func (f *webhookSyncFake) EnqueueWebhook(_ context.Context, delivery board.WebhookDelivery) (bool, error) {
	f.delivery = delivery
	return false, nil
}

func TestVerifyGitLabWebhookSignatureAndTimestamp(t *testing.T) {
	now := time.Unix(1_750_000_000, 0)
	token := testSigningToken()
	body := []byte(`{"object_kind":"issue"}`)
	headers := signedWebhookHeaders(token, "delivery-1", now, body)
	if err := verifyGitLabWebhook(token, headers, body, now); err != nil {
		t.Fatalf("verifyGitLabWebhook() error = %v", err)
	}
	headers.Set("webhook-signature", "v1,invalid")
	if err := verifyGitLabWebhook(token, headers, body, now); err == nil {
		t.Fatal("invalid signature was accepted")
	}
	headers = signedWebhookHeaders(token, "delivery-1", now.Add(-11*time.Minute), body)
	if err := verifyGitLabWebhook(token, headers, body, now); err == nil {
		t.Fatal("stale timestamp was accepted")
	}
}

func TestProjectWebhookQueuesSignedIssueEvent(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	token := testSigningToken()
	body := []byte(`{"object_kind":"issue","event_type":"issue","project":{"path_with_namespace":"sitcon-tw/2027"},"object_attributes":{"iid":42,"type":"Issue"}}`)
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/webhooks/gitlab/project", strings.NewReader(string(body)))
	request.Header = signedWebhookHeaders(token, "delivery-42", now, body)
	request.Header.Set("X-Gitlab-Event", "Issue Hook")
	recorder := httptest.NewRecorder()
	sync := &webhookSyncFake{}
	h := handler{sync: sync, webhooks: WebhookConfig{ProjectSigningToken: token, ProjectPath: "sitcon-tw/2027"}}
	h.receiveProjectWebhook(recorder, request)
	if recorder.Code != http.StatusAccepted || sync.delivery.ID != "delivery-42" || sync.delivery.IssueIID == nil || *sync.delivery.IssueIID != 42 {
		t.Fatalf("response=%d body=%s delivery=%#v", recorder.Code, recorder.Body.String(), sync.delivery)
	}
}

func TestProjectWebhookAcceptsConfidentialIssueHook(t *testing.T) {
	// GitLab routes confidential issues through confidential_issue_hooks, so it sends
	// "Confidential Issue Hook" for them. Dropping those would leave confidential
	// cards stale until a full reconciliation sweep.
	for _, testCase := range []struct {
		event  string
		queued bool
	}{
		{event: "Issue Hook", queued: true},
		{event: "Confidential Issue Hook", queued: true},
		{event: "Pipeline Hook", queued: false},
	} {
		t.Run(testCase.event, func(t *testing.T) {
			now := time.Now().UTC().Truncate(time.Second)
			token := testSigningToken()
			body := []byte(`{"object_kind":"issue","event_type":"issue","project":{"path_with_namespace":"sitcon-tw/2027"},"object_attributes":{"iid":77,"type":"Issue"}}`)
			request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/webhooks/gitlab/project", strings.NewReader(string(body)))
			request.Header = signedWebhookHeaders(token, "delivery-77", now, body)
			request.Header.Set("X-Gitlab-Event", testCase.event)
			recorder := httptest.NewRecorder()
			sync := &webhookSyncFake{}
			h := handler{sync: sync, webhooks: WebhookConfig{ProjectSigningToken: token, ProjectPath: "sitcon-tw/2027"}}
			h.receiveProjectWebhook(recorder, request)
			if recorder.Code != http.StatusAccepted {
				t.Fatalf("response = %d, body = %s", recorder.Code, recorder.Body.String())
			}
			queued := sync.delivery.IssueIID != nil
			if queued != testCase.queued {
				t.Fatalf("queued = %t, want %t (delivery = %#v)", queued, testCase.queued, sync.delivery)
			}
			if queued && *sync.delivery.IssueIID != 77 {
				t.Fatalf("issue IID = %d, want 77", *sync.delivery.IssueIID)
			}
		})
	}
}

func TestGroupWebhookQueuesSignedMemberEvent(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	token := testSigningToken()
	body := []byte(`{"group_path":"sitcon-tw","event_name":"user_update_for_group"}`)
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/webhooks/gitlab/group", strings.NewReader(string(body)))
	request.Header = signedWebhookHeaders(token, "member-delivery", now, body)
	request.Header.Set("X-Gitlab-Event", "Member Hook")
	recorder := httptest.NewRecorder()
	sync := &webhookSyncFake{}
	h := handler{sync: sync, webhooks: WebhookConfig{GroupSigningToken: token, GroupPath: "sitcon-tw"}}
	h.receiveGroupWebhook(recorder, request)
	if recorder.Code != http.StatusAccepted || sync.delivery.EventKind != "member" || sync.delivery.EventName != "user_update_for_group" {
		t.Fatalf("response=%d body=%s delivery=%#v", recorder.Code, recorder.Body.String(), sync.delivery)
	}
}

func testSigningToken() string {
	key := make([]byte, 32)
	for index := range key {
		key[index] = byte(index + 1)
	}
	return "whsec_" + base64.StdEncoding.EncodeToString(key)
}

func signedWebhookHeaders(token, id string, at time.Time, body []byte) http.Header {
	timestamp := fmt.Sprintf("%d", at.Unix())
	key, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(token, "whsec_"))
	digest := hmac.New(sha256.New, key)
	_, _ = digest.Write(append([]byte(id+"."+timestamp+"."), body...))
	headers := make(http.Header)
	headers.Set("webhook-id", id)
	headers.Set("webhook-timestamp", timestamp)
	headers.Set("webhook-signature", "v1,"+base64.StdEncoding.EncodeToString(digest.Sum(nil)))
	return headers
}
