package org.sitcon.sitlab.network

import io.ktor.client.HttpClient
import io.ktor.client.call.body
import io.ktor.client.plugins.HttpResponseValidator
import io.ktor.client.plugins.contentnegotiation.ContentNegotiation
import io.ktor.client.plugins.cookies.HttpCookies
import io.ktor.client.plugins.sse.SSE
import io.ktor.client.plugins.sse.serverSentEvents
import io.ktor.client.request.get
import io.ktor.client.request.header
import io.ktor.client.request.parameter
import io.ktor.client.request.post
import io.ktor.client.request.setBody
import io.ktor.http.HttpHeaders
import io.ktor.http.isSuccess
import io.ktor.client.statement.HttpResponse
import io.ktor.serialization.kotlinx.json.json
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import kotlinx.coroutines.flow.collect
import org.sitcon.sitlab.api.generated.BootstrapResponse
import org.sitcon.sitlab.api.generated.ProblemDetails
import org.sitcon.sitlab.api.generated.SyncDeltaResponse

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

    suspend fun observeSyncEvents(onRevision: suspend (String?) -> Unit): Nothing {
        while (true) {
            val cookie = sessionCookie()
            client.serverSentEvents("$origin/api/v1/events/sync", request = {
                cookie?.let { header(HttpHeaders.Cookie, it) }
            }) {
                incoming.collect { event -> onRevision(event.data) }
            }
        }
    }

    @PublishedApi
    internal suspend fun io.ktor.client.request.HttpRequestBuilder.authenticate() {
        sessionCookie()?.let { header(HttpHeaders.Cookie, it) }
    }
}

expect fun platformHttpClient(): HttpClient
