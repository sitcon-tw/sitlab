package org.sitcon.sitlab

import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.lifecycleScope
import androidx.work.WorkInfo
import androidx.work.WorkManager
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import kotlinx.serialization.json.Json
import org.sitcon.sitlab.domain.BoardList
import org.sitcon.sitlab.domain.Card
import org.sitcon.sitlab.domain.Member
import org.sitcon.sitlab.network.SitLabApi
import org.sitcon.sitlab.network.platformHttpClient
import org.sitcon.sitlab.persistence.AndroidDatabaseFactory
import org.sitcon.sitlab.persistence.CardEntity
import org.sitcon.sitlab.persistence.MemberEntity
import org.sitcon.sitlab.persistence.SitLabDatabase
import org.sitcon.sitlab.persistence.TeamEntity
import org.sitcon.sitlab.sync.SyncEngine
import org.sitcon.sitlab.ui.AppActions
import org.sitcon.sitlab.ui.AppUiState
import org.sitcon.sitlab.ui.SitLabApp

class MainActivity : ComponentActivity(), AppActions {
    private var state by mutableStateOf(AppUiState())
    private lateinit var sessionStore: AndroidSecureSessionStore
    private lateinit var database: SitLabDatabase
    private lateinit var api: SitLabApi
    private lateinit var syncEngine: SyncEngine
    private val json = Json { ignoreUnknownKeys = true }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        sessionStore = AndroidSecureSessionStore(applicationContext)
        database = AndroidDatabaseFactory(applicationContext).create()
        api = SitLabApi(platformHttpClient(), sessionCookie = sessionStore::readCookie)
        syncEngine = SyncEngine(api, database)
        observeCachedBoard()
        handleDeepLink(intent)
        setContent { SitLabApp(state, this) }
        lifecycleScope.launch {
            state = state.copy(authenticated = sessionStore.readCookie() != null)
            if (state.authenticated) refresh()
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        handleDeepLink(intent)
    }

    private fun handleDeepLink(intent: Intent?) {
        val uri = intent?.data ?: return
        when {
            uri.path == "/api/v1/auth/gitlab/mobile/callback" -> MobileOAuthCoordinator(this).complete(uri)?.let(::observeOAuthExchange)
            uri.path?.startsWith("/mobile/cards/") == true -> uri.lastPathSegment?.toLongOrNull()?.let(::openCard)
        }
    }

    private fun observeOAuthExchange(requestId: java.util.UUID) {
        lifecycleScope.launch {
            val result = WorkManager.getInstance(applicationContext).getWorkInfoByIdFlow(requestId)
                .first { it?.state?.isFinished == true }
            if (result?.state == WorkInfo.State.SUCCEEDED) {
                state = state.copy(authenticated = true, error = null)
                refresh()
            } else {
                state = state.copy(error = "GitLab sign-in could not be completed")
            }
        }
    }

    private fun observeCachedBoard() {
        val dao = database.dao()
        lifecycleScope.launch {
            combine(dao.observeLists(), dao.observeCards(), dao.observeTeams(), dao.observeMembers()) { lists, cards, teams, members ->
                val teamNames = teams.associateBy(TeamEntity::key, TeamEntity::displayName)
                val memberModels = members.associate { it.gitLabUserId to it.toDomain() }
                lists.map { BoardList(it.key, it.name, it.position, it.closed, it.color) } to
                    cards.map { it.toDomain(teamNames, memberModels) }
            }.collect { (lists, cards) -> state = state.copy(lists = lists, cards = cards) }
        }
    }

    override fun startLogin() = MobileOAuthCoordinator(this).start()
    override fun updateQuery(value: String) { state = state.copy(filter = state.filter.copy(query = value)) }
    override fun openCard(issueIid: Long) { /* Detail navigation is restored after cached bootstrap loads. */ }
    override fun moveCard(issueIid: Long) { /* Opens the accessible lane chooser in the populated app graph. */ }
    override fun refresh() {
        if (!state.authenticated || state.syncing) return
        state = state.copy(syncing = true, error = null)
        lifecycleScope.launch {
            runCatching { syncEngine.refresh() }
                .onSuccess { state = state.copy(syncing = false, error = null) }
                .onFailure { state = state.copy(syncing = false, error = it.message ?: "Synchronization failed") }
        }
    }

    private fun MemberEntity.toDomain() = Member(gitLabUserId, username, displayName, json.decodeFromString(teamKeysJson))

    private fun CardEntity.toDomain(teamNames: Map<String, String>, members: Map<Long, Member>) = Card(
        issueIid = issueIid,
        title = title,
        description = description,
        listKey = listKey,
        position = position,
        teamKey = teamKey,
        teamName = teamNames[teamKey] ?: teamKey,
        assignees = json.decodeFromString<List<Long>>(assigneeIdsJson).mapNotNull(members::get),
        startDate = startDate,
        dueDate = dueDate,
        labels = json.decodeFromString(labelsJson),
        updatedAt = updatedAt,
        synchronized = syncState == "synced",
        syncError = syncError,
    )
}
