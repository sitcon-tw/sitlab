package org.sitcon.sitlab.network

import io.ktor.client.HttpClient
import io.ktor.client.call.body
import io.ktor.client.plugins.HttpResponseValidator
import io.ktor.client.plugins.contentnegotiation.ContentNegotiation
import io.ktor.client.plugins.cookies.HttpCookies
import io.ktor.client.plugins.sse.SSE
import io.ktor.client.plugins.sse.serverSentEvents
import io.ktor.client.request.get
import io.ktor.client.request.delete
import io.ktor.client.request.header
import io.ktor.client.request.parameter
import io.ktor.client.request.post
import io.ktor.client.request.put
import io.ktor.client.request.setBody
import io.ktor.http.HttpHeaders
import io.ktor.http.isSuccess
import io.ktor.client.statement.HttpResponse
import io.ktor.serialization.kotlinx.json.json
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import kotlinx.coroutines.flow.collect
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import org.sitcon.sitlab.api.generated.AuthResponse
import org.sitcon.sitlab.api.generated.BootstrapResponse
import org.sitcon.sitlab.api.generated.CSRFResponse
import org.sitcon.sitlab.api.generated.CardCommentsResponse
import org.sitcon.sitlab.api.generated.CardDeletionResponse
import org.sitcon.sitlab.api.generated.CardMutationResponse
import org.sitcon.sitlab.api.generated.ChildItemsResponse
import org.sitcon.sitlab.api.generated.CreateCardCommentRequest
import org.sitcon.sitlab.api.generated.CreateCardCommentResult
import org.sitcon.sitlab.api.generated.CreateCardRequest
import org.sitcon.sitlab.api.generated.CreateChildItemRequest
import org.sitcon.sitlab.api.generated.CreateLinkedItemsRequest
import org.sitcon.sitlab.api.generated.CreateProjectLabelRequest
import org.sitcon.sitlab.api.generated.DeleteCardRequest
import org.sitcon.sitlab.api.generated.DirectoryResponse
import org.sitcon.sitlab.api.generated.LinkedItemsResponse
import org.sitcon.sitlab.api.generated.MoveCardRequest
import org.sitcon.sitlab.api.generated.PreferencesResponse
import org.sitcon.sitlab.api.generated.ProblemDetails
import org.sitcon.sitlab.api.generated.ProjectLabelsResponse
import org.sitcon.sitlab.api.generated.QuickActionCommandsResponse
import org.sitcon.sitlab.api.generated.QuickActionSuggestionsResponse
import org.sitcon.sitlab.api.generated.RefreshSyncResponse
import org.sitcon.sitlab.api.generated.RetryOperationResponse
import org.sitcon.sitlab.api.generated.SyncDeltaResponse
import org.sitcon.sitlab.api.generated.UpdateCardAssigneeRequest
import org.sitcon.sitlab.api.generated.UpdateCardDetailsRequest
import org.sitcon.sitlab.api.generated.UpdateCardDueDateRequest
import org.sitcon.sitlab.api.generated.UpdateCardLabelsRequest
import org.sitcon.sitlab.api.generated.UpdateCardStartDateRequest
import org.sitcon.sitlab.api.generated.UpdateCardTeamRequest
import org.sitcon.sitlab.api.generated.UpdatePreferencesRequest
import org.sitcon.sitlab.api.generated.UpdateProjectLabelRequest
import org.sitcon.sitlab.api.generated.WorkItemCandidatesResponse
import org.sitcon.sitlab.api.generated.WorkItemSummary

const val ProductionOrigin = "https://sitlab.sitcon.org"

class ApiProblem(val problem: ProblemDetails) : Exception(problem.detail ?: problem.title)

@Serializable
data class MobileExchangeRequest(
    val code: String,
    val state: String,
    val codeVerifier: String,
)

class SitLabApi(
    engineClient: HttpClient,
    @PublishedApi internal val origin: String = ProductionOrigin,
    @PublishedApi internal val sessionCookie: suspend () -> String? = { null },
    private val sessionCookieUpdated: suspend (String) -> Unit = {},
) {
    private val json = Json {
        ignoreUnknownKeys = true
        explicitNulls = false
        classDiscriminator = "entity"
    }

    val client = engineClient.config {
        install(ContentNegotiation) { json(json) }
        install(HttpCookies)
        install(SSE)
        HttpResponseValidator {
            validateResponse { response ->
                response.headers.getAll(HttpHeaders.SetCookie).orEmpty().firstOrNull()
                    ?.substringBefore(';')?.takeIf(String::isNotBlank)?.let { sessionCookieUpdated(it) }
                if (!response.status.isSuccess()) {
                    val contentType = response.headers[HttpHeaders.ContentType].orEmpty()
                    if (contentType.startsWith("application/problem+json")) {
                        throw ApiProblem(response.body())
                    }
                }
            }
        }
    }

    fun mobileLoginUrl(challenge: String): String =
        "$origin/api/v1/auth/gitlab/mobile?codeChallenge=$challenge"

    suspend fun exchange(code: String, state: String, verifier: String): String {
        val response: HttpResponse = client.post("$origin/api/v1/auth/gitlab/mobile/exchange") {
            setBody(MobileExchangeRequest(code, state, verifier))
        }
        return response.headers.getAll(HttpHeaders.SetCookie).orEmpty().firstOrNull()
            ?.substringBefore(';') ?: error("mobile exchange did not issue a session cookie")
    }

    suspend fun bootstrap(): BootstrapResponse = client.get("$origin/api/v1/bootstrap") { authenticate() }.body()

    suspend fun me(): AuthResponse = client.get("$origin/api/v1/auth/me") { authenticate() }.body()
    suspend fun csrf(): CSRFResponse = client.get("$origin/api/v1/auth/csrf") { authenticate() }.body()

    suspend fun logout(csrfToken: String) {
        client.post("$origin/api/v1/auth/logout") { authenticate(); header("X-CSRF-Token", csrfToken) }
    }

    suspend fun sync(since: String, limit: Int = 500): SyncDeltaResponse =
        client.get("$origin/api/v1/sync") {
            authenticate()
            parameter("since", since)
            parameter("limit", limit)
        }.body()

    suspend inline fun <reified Request : Any, reified Response : Any> mutate(
        path: String,
        csrfToken: String,
        body: Request,
    ): Response = client.post("$origin/api/v1/$path") {
        authenticate()
        header("X-CSRF-Token", csrfToken)
        setBody(body)
    }.body()

    suspend fun createCard(csrf: String, body: CreateCardRequest): CardMutationResponse = post("cards", csrf, body)
    suspend fun deleteCard(issueIid: Long, csrf: String, body: DeleteCardRequest): CardDeletionResponse = delete("cards/$issueIid", csrf, body)
    suspend fun updateDetails(issueIid: Long, csrf: String, body: UpdateCardDetailsRequest): CardMutationResponse = put("cards/$issueIid/details", csrf, body)
    suspend fun updateTeam(issueIid: Long, csrf: String, body: UpdateCardTeamRequest): CardMutationResponse = put("cards/$issueIid/team", csrf, body)
    suspend fun updateAssignees(issueIid: Long, csrf: String, body: UpdateCardAssigneeRequest): CardMutationResponse = put("cards/$issueIid/assignee", csrf, body)
    suspend fun updateStartDate(issueIid: Long, csrf: String, body: UpdateCardStartDateRequest): CardMutationResponse = put("cards/$issueIid/start-date", csrf, body)
    suspend fun updateDueDate(issueIid: Long, csrf: String, body: UpdateCardDueDateRequest): CardMutationResponse = put("cards/$issueIid/due-date", csrf, body)
    suspend fun updateLabels(issueIid: Long, csrf: String, body: UpdateCardLabelsRequest): CardMutationResponse = put("cards/$issueIid/labels", csrf, body)
    suspend fun moveCard(issueIid: Long, csrf: String, body: MoveCardRequest): CardMutationResponse = put("cards/$issueIid/position", csrf, body)
    suspend fun comments(issueIid: Long): CardCommentsResponse = get("cards/$issueIid/comments")
    suspend fun createComment(issueIid: Long, csrf: String, body: CreateCardCommentRequest): CreateCardCommentResult = post("cards/$issueIid/comments", csrf, body)
    suspend fun childItems(issueIid: Long, cursor: String? = null): ChildItemsResponse = get("cards/$issueIid/child-items", cursor)
    suspend fun createChild(issueIid: Long, csrf: String, body: CreateChildItemRequest): WorkItemSummary = post("cards/$issueIid/child-items", csrf, body)
    suspend fun attachChild(issueIid: Long, workItemId: Long, csrf: String) = putNoContent("cards/$issueIid/child-items/$workItemId", csrf)
    suspend fun detachChild(issueIid: Long, workItemId: Long, csrf: String) = deleteNoContent("cards/$issueIid/child-items/$workItemId", csrf)
    suspend fun linkedItems(issueIid: Long, cursor: String? = null): LinkedItemsResponse = get("cards/$issueIid/linked-items", cursor)
    suspend fun createLinks(issueIid: Long, csrf: String, body: CreateLinkedItemsRequest) = postNoContent("cards/$issueIid/linked-items", csrf, body)
    suspend fun deleteLink(issueIid: Long, workItemId: Long, csrf: String) = deleteNoContent("cards/$issueIid/linked-items/$workItemId", csrf)
    suspend fun relationshipCandidates(issueIid: Long, kind: String, query: String): WorkItemCandidatesResponse =
        client.get("$origin/api/v1/cards/$issueIid/relationship-candidates") { authenticate(); parameter("kind", kind); parameter("query", query) }.body()
    suspend fun labels(): ProjectLabelsResponse = get("labels")
    suspend fun createLabel(csrf: String, body: CreateProjectLabelRequest): org.sitcon.sitlab.api.generated.ProjectLabel = post("labels", csrf, body)
    suspend fun updateLabel(id: Long, csrf: String, body: UpdateProjectLabelRequest): org.sitcon.sitlab.api.generated.ProjectLabel = put("labels/$id", csrf, body)
    suspend fun deleteLabel(id: Long, csrf: String) = deleteNoContent("labels/$id", csrf)
    suspend fun quickActions(issueIid: Long? = null): QuickActionCommandsResponse =
        client.get("$origin/api/v1/quick-actions") { authenticate(); issueIid?.let { parameter("issueIid", it) } }.body()
    suspend fun quickActionSuggestions(kind: String, query: String, issueIid: Long? = null): QuickActionSuggestionsResponse =
        client.get("$origin/api/v1/quick-actions/suggestions") { authenticate(); parameter("kind", kind); parameter("query", query); issueIid?.let { parameter("issueIid", it) } }.body()
    suspend fun directory(): DirectoryResponse = get("directory")
    suspend fun updatePreferences(csrf: String, body: UpdatePreferencesRequest): PreferencesResponse = put("me/preferences", csrf, body)
    suspend fun retry(operationId: String, csrf: String): RetryOperationResponse = postNoBody("operations/$operationId/retry", csrf)
    suspend fun requestRefresh(csrf: String): RefreshSyncResponse = postNoBody("sync/refresh", csrf)

    private suspend inline fun <reified Response : Any> get(path: String, cursor: String? = null): Response =
        client.get("$origin/api/v1/$path") { authenticate(); cursor?.let { parameter("cursor", it) } }.body()

    private suspend inline fun <reified Request : Any, reified Response : Any> post(path: String, csrf: String, body: Request): Response =
        client.post("$origin/api/v1/$path") { authenticate(); header("X-CSRF-Token", csrf); setBody(body) }.body()

    private suspend inline fun <reified Request : Any, reified Response : Any> put(path: String, csrf: String, body: Request): Response =
        client.put("$origin/api/v1/$path") { authenticate(); header("X-CSRF-Token", csrf); setBody(body) }.body()

    private suspend inline fun <reified Request : Any, reified Response : Any> delete(path: String, csrf: String, body: Request): Response =
        client.delete("$origin/api/v1/$path") { authenticate(); header("X-CSRF-Token", csrf); setBody(body) }.body()

    private suspend inline fun <reified Response : Any> postNoBody(path: String, csrf: String): Response =
        client.post("$origin/api/v1/$path") { authenticate(); header("X-CSRF-Token", csrf) }.body()

    private suspend inline fun <reified Request : Any> postNoContent(path: String, csrf: String, body: Request) {
        client.post("$origin/api/v1/$path") { authenticate(); header("X-CSRF-Token", csrf); setBody(body) }
    }

    private suspend fun putNoContent(path: String, csrf: String) {
        client.put("$origin/api/v1/$path") { authenticate(); header("X-CSRF-Token", csrf) }
    }

    private suspend fun deleteNoContent(path: String, csrf: String) {
        client.delete("$origin/api/v1/$path") { authenticate(); header("X-CSRF-Token", csrf) }
    }

    suspend fun observeSyncEvents(since: String?, onEvent: suspend (SyncStreamEvent) -> Unit): Nothing {
        while (true) {
            val cookie = sessionCookie()
            client.serverSentEvents("$origin/api/v1/events/sync", request = {
                cookie?.let { header(HttpHeaders.Cookie, it) }
                since?.let { parameter("since", it) }
            }) {
                incoming.collect { event ->
                    val payload = event.data ?: return@collect
                    runCatching {
                        when (event.event) {
                            "sync" -> SyncStreamEvent.Delta(json.decodeFromString<SyncDeltaResponse>(payload))
                            "heartbeat" -> SyncStreamEvent.Heartbeat(Json.parseToJsonElement(payload).jsonObject.getValue("checkpoint").jsonPrimitive.content)
                            "reset" -> Json.parseToJsonElement(payload).jsonObject.let {
                                SyncStreamEvent.Reset(it["reason"]?.jsonPrimitive?.content.orEmpty(), it.getValue("checkpoint").jsonPrimitive.content)
                            }
                            else -> null
                        }
                    }.getOrNull()?.let { onEvent(it) }
                }
            }
        }
    }

    @PublishedApi
    internal suspend fun io.ktor.client.request.HttpRequestBuilder.authenticate() {
        sessionCookie()?.let { header(HttpHeaders.Cookie, it) }
    }
}

sealed interface SyncStreamEvent {
    data class Delta(val value: SyncDeltaResponse) : SyncStreamEvent
    data class Heartbeat(val checkpoint: String) : SyncStreamEvent
    data class Reset(val reason: String, val checkpoint: String) : SyncStreamEvent
}

expect fun platformHttpClient(): HttpClient
