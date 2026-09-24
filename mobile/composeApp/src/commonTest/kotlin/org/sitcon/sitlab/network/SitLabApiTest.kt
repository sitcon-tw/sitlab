package org.sitcon.sitlab.network

import io.ktor.client.HttpClient
import io.ktor.client.engine.mock.MockEngine
import io.ktor.client.engine.mock.respond
import io.ktor.http.ContentType
import io.ktor.http.Headers
import io.ktor.http.HttpHeaders
import io.ktor.http.HttpStatusCode
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlinx.coroutines.test.runTest

class SitLabApiTest {
    @Test
    fun authenticatedRequestsPersistRollingCookie() = runTest {
        var updated: String? = null
        val engine = MockEngine { request ->
            assertEquals("session=old", request.headers[HttpHeaders.Cookie])
            respond(
                content = """{"token":"csrf"}""",
                status = HttpStatusCode.OK,
                headers = Headers.build {
                    append(HttpHeaders.ContentType, ContentType.Application.Json.toString())
                    append(HttpHeaders.SetCookie, "session=new; Path=/; Secure; HttpOnly")
                },
            )
        }
        val api = SitLabApi(HttpClient(engine), "https://example.test", { "session=old" }, { updated = it })
        assertEquals("csrf", api.csrf().token)
        assertEquals("session=new", updated)
    }

    @Test
    fun problemDetailsAreExposedToCallers() = runTest {
        val engine = MockEngine {
            respond(
                content = """{"type":"about:blank","title":"Invalid","status":422,"code":"CARD_INVALID","detail":"Fix the title"}""",
                status = HttpStatusCode.UnprocessableEntity,
                headers = Headers.build { append(HttpHeaders.ContentType, "application/problem+json") },
            )
        }
        val problem = assertFailsWith<ApiProblem> { SitLabApi(HttpClient(engine), "https://example.test").csrf() }
        assertEquals("CARD_INVALID", problem.problem.code)
    }
}
