import "@typespec/http";
import "@typespec/openapi";

using Http;
using OpenAPI;

namespace SitconBoard;

model WebhookAcceptedResponse {
	accepted: true;
	duplicate: boolean;
}

model GitLabWebhookHeaders {
	@header("webhook-id")
	webhookId: string;

	@header("webhook-timestamp")
	webhookTimestamp: string;

	@header("webhook-signature")
	webhookSignature: string;

	@header("X-Gitlab-Event")
	gitLabEvent: string;
}

model EventStreamResponse {
	@header
	contentType: "text/event-stream";

	@body
	body: string;
}

@tag("Realtime")
interface RealtimeService {
	@operationId("receiveGitLabProjectWebhook")
	@summary("Receive a signed GitLab project webhook")
	@post
	@route("/webhooks/gitlab/project")
	receiveGitLabProjectWebhook(...GitLabWebhookHeaders, @body body: unknown):
		| AcceptedJson<WebhookAcceptedResponse>
		| BadRequestProblem
		| UnauthorizedProblem
		| ServiceUnavailableProblem
		| InternalServerProblem;

	@operationId("receiveGitLabGroupWebhook")
	@summary("Receive a signed GitLab group webhook")
	@post
	@route("/webhooks/gitlab/group")
	receiveGitLabGroupWebhook(...GitLabWebhookHeaders, @body body: unknown):
		| AcceptedJson<WebhookAcceptedResponse>
		| BadRequestProblem
		| UnauthorizedProblem
		| ServiceUnavailableProblem
		| InternalServerProblem;

	@operationId("streamBootstrapEvents")
	@summary("Stream authenticated bootstrap revision changes")
	@useAuth(SessionCookieAuth)
	@get
	@route("/events/bootstrap")
	streamBootstrapEvents(): EventStreamResponse | UnauthorizedProblem | ServiceUnavailableProblem | InternalServerProblem;
}
