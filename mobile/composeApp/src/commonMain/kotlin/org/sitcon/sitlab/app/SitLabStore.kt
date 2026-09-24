package org.sitcon.sitlab.app

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import org.sitcon.sitlab.api.generated.BoardCard
import org.sitcon.sitlab.api.generated.CreateCardRequest
import org.sitcon.sitlab.api.generated.CreateCardCommentRequest
import org.sitcon.sitlab.api.generated.CreateProjectLabelRequest
import org.sitcon.sitlab.api.generated.CurrentUser
import org.sitcon.sitlab.api.generated.DeleteCardRequest
import org.sitcon.sitlab.api.generated.MoveCardRequest
import org.sitcon.sitlab.api.generated.UpdateCardDetailsRequest
import org.sitcon.sitlab.api.generated.UpdateCardAssigneeRequest
import org.sitcon.sitlab.api.generated.UpdateCardDueDateRequest
import org.sitcon.sitlab.api.generated.UpdateCardLabelsRequest
import org.sitcon.sitlab.api.generated.UpdateCardStartDateRequest
import org.sitcon.sitlab.api.generated.UpdateCardTeamRequest
import org.sitcon.sitlab.domain.BoardList
import org.sitcon.sitlab.domain.Card
import org.sitcon.sitlab.domain.Member
import org.sitcon.sitlab.domain.Team
import org.sitcon.sitlab.network.ApiProblem
import org.sitcon.sitlab.network.ProductionOrigin
import org.sitcon.sitlab.network.SitLabApi
import org.sitcon.sitlab.persistence.CardEntity
import org.sitcon.sitlab.persistence.MemberEntity
import org.sitcon.sitlab.persistence.PendingRequestEntity
import org.sitcon.sitlab.persistence.PreferencesStore
import org.sitcon.sitlab.persistence.SitLabDatabase
import org.sitcon.sitlab.persistence.TeamEntity
import org.sitcon.sitlab.platform.HapticEffect
import org.sitcon.sitlab.platform.Haptics
import org.sitcon.sitlab.platform.BackgroundInterval
import org.sitcon.sitlab.platform.BackgroundRefresh
import org.sitcon.sitlab.platform.LocalNotifications
import org.sitcon.sitlab.platform.PermissionStatus
import org.sitcon.sitlab.platform.ShareSheet
import org.sitcon.sitlab.platform.SystemAppearance
import org.sitcon.sitlab.sync.SyncEngine
import org.sitcon.sitlab.ui.AppActions
import org.sitcon.sitlab.ui.AppUiState
import org.sitcon.sitlab.ui.Route

class SitLabStore(
    private val scope: CoroutineScope,
    private val api: SitLabApi,
    private val database: SitLabDatabase,
    private val sessionStore: org.sitcon.sitlab.platform.SecureSessionStore,
    private val syncEngine: SyncEngine,
    private val newOperationId: () -> String,
    private val startPlatformLogin: () -> Unit,
    private val haptics: Haptics? = null,
    private val preferencesStore: PreferencesStore? = null,
    private val backgroundRefresh: BackgroundRefresh? = null,
    private val notifications: LocalNotifications? = null,
    private val shareSheet: ShareSheet? = null,
    private val systemAppearance: SystemAppearance? = null,
    private val remindersChanged: suspend () -> Unit = {},
    private val sessionCleared: suspend () -> Unit = {},
) : AppActions {
    private val json = Json { ignoreUnknownKeys = true; explicitNulls = false }
    private val mutableState = MutableStateFlow(AppUiState())
    val state: StateFlow<AppUiState> = mutableState.asStateFlow()
    private var realtimeJob: Job? = null
    private var pendingCardIid: Long? = null
    private val mutationMutex = Mutex()

    init {
        observeCache()
        scope.launch { restoreSession() }
        systemAppearance?.let { appearance ->
            scope.launch { appearance.reducedMotion.collect { value -> mutableState.value = mutableState.value.copy(reducedMotion = value) } }
        }
        scope.launch {
            mutableState.value = mutableState.value.copy(
                notificationPermission = notifications?.permissionStatus() ?: PermissionStatus.NotDetermined,
                supportsDynamicColor = systemAppearance?.supportsDynamicColor == true,
            )
        }
    }

    private fun observeCache() {
        val dao = database.dao()
        scope.launch {
            combine(dao.observeLists(), dao.observeCards(), dao.observeTeams(), dao.observeMembers()) { lists, cards, teams, members ->
                val teamNames = teams.associateBy(TeamEntity::key, TeamEntity::displayName)
                val memberModels = members.associate { it.gitLabUserId to it.toDomain() }
                CacheSnapshot(
                    lists.map { BoardList(it.key, it.name, it.position, it.closed, it.color) },
                    cards.map { it.toDomain(teamNames, memberModels) },
                    teams.map { Team(it.key, it.displayName, it.active, it.position, emptyList(), it.gitLabLabel) },
                    memberModels.values.sortedBy(Member::displayName),
                )
            }.collect { snapshot ->
                mutableState.value = mutableState.value.copy(
                    lists = snapshot.lists,
                    cards = snapshot.cards,
                    teams = snapshot.teams,
                    members = snapshot.members,
                )
                pendingCardIid?.takeIf { iid -> snapshot.cards.any { it.issueIid == iid } }?.let { iid ->
                    pendingCardIid = null
                    mutableState.value = mutableState.value.copy(route = Route.CardDetail(iid))
                }
            }
        }
        scope.launch {
            combine(
                dao.observeMetadata(CurrentUserKey),
                dao.observeMetadata(LastSyncKey),
            ) { user, lastSync -> user?.value to lastSync?.value }.collect { (user, lastSync) ->
                mutableState.value = mutableState.value.copy(
                    currentUser = user?.let { runCatching { json.decodeFromString<CurrentUser>(it) }.getOrNull() },
                    lastSync = lastSync,
                )
            }
        }
    }

    private suspend fun restoreSession() {
        val preferences = loadPreferences()
        val authenticated = sessionStore.readCookie() != null
        mutableState.value = mutableState.value.copy(
            authenticated = authenticated,
            route = if (!authenticated) Route.Login else if (preferences?.onboardingComplete == false) Route.Onboarding else Route.Board,
        )
        if (authenticated) {
            replayPendingRequests()
            refresh()
            startRealtime()
        }
    }

    private suspend fun replayPendingRequests() {
        database.dao().pendingRequests().filter { it.permanentError == null }.forEach { pending ->
            runCatching {
                when (pending.requestKind) {
                    "create_card" -> {
                        val result = api.createCard(csrf(), json.decodeFromString<CreateCardRequest>(pending.payloadJson))
                        pending.issueIid?.let { database.dao().deleteCard(it) }
                        database.dao().upsertCards(listOf(result.card.toEntity()))
                    }
                    "move_card" -> {
                        val iid = requireNotNull(pending.issueIid)
                        val result = api.moveCard(iid, csrf(), json.decodeFromString<MoveCardRequest>(pending.payloadJson))
                        database.dao().upsertCards(listOf(result.card.toEntity()))
                    }
                    "update_details" -> {
                        val iid = requireNotNull(pending.issueIid)
                        val result = api.updateDetails(iid, csrf(), json.decodeFromString<UpdateCardDetailsRequest>(pending.payloadJson))
                        database.dao().upsertCards(listOf(result.card.toEntity()))
                    }
                    "update_team" -> {
                        val iid = requireNotNull(pending.issueIid)
                        database.dao().upsertCards(listOf(api.updateTeam(iid, csrf(), json.decodeFromString<UpdateCardTeamRequest>(pending.payloadJson)).card.toEntity()))
                    }
                    "update_assignees" -> {
                        val iid = requireNotNull(pending.issueIid)
                        database.dao().upsertCards(listOf(api.updateAssignees(iid, csrf(), json.decodeFromString<UpdateCardAssigneeRequest>(pending.payloadJson)).card.toEntity()))
                    }
                    "update_start_date" -> {
                        val iid = requireNotNull(pending.issueIid)
                        database.dao().upsertCards(listOf(api.updateStartDate(iid, csrf(), json.decodeFromString<UpdateCardStartDateRequest>(pending.payloadJson)).card.toEntity()))
                    }
                    "update_due_date" -> {
                        val iid = requireNotNull(pending.issueIid)
                        database.dao().upsertCards(listOf(api.updateDueDate(iid, csrf(), json.decodeFromString<UpdateCardDueDateRequest>(pending.payloadJson)).card.toEntity()))
                    }
                    "update_labels" -> {
                        val iid = requireNotNull(pending.issueIid)
                        database.dao().upsertCards(listOf(api.updateLabels(iid, csrf(), json.decodeFromString<UpdateCardLabelsRequest>(pending.payloadJson)).card.toEntity()))
                    }
                    "delete_card" -> {
                        val iid = requireNotNull(pending.issueIid)
                        api.deleteCard(iid, csrf(), json.decodeFromString<DeleteCardRequest>(pending.payloadJson))
                        database.dao().deleteCard(iid)
                    }
                }
                database.dao().deletePendingRequest(pending.operationId)
            }.onFailure { error ->
                val permanent = (error as? ApiProblem)?.problem?.status?.let { it in 400..499 && it != 409 } == true
                database.dao().upsertPendingRequest(pending.copy(attempts = pending.attempts + 1, permanentError = if (permanent) error.message else null))
            }
        }
    }

    fun onSessionEstablished() {
        scope.launch {
            val preferences = loadPreferences()
            mutableState.value = mutableState.value.copy(
                authenticated = true,
                route = if (preferences?.onboardingComplete == false) Route.Onboarding else Route.Board,
                error = null,
            )
            refresh()
            startRealtime()
        }
    }

    fun onForeground() {
        refresh()
        scope.launch {
            mutableState.value = mutableState.value.copy(
                notificationPermission = notifications?.permissionStatus() ?: PermissionStatus.NotDetermined,
            )
        }
    }

    fun reportError(message: String) {
        mutableState.value = mutableState.value.copy(error = message)
    }

    private fun startRealtime() {
        if (realtimeJob?.isActive == true) return
        realtimeJob = scope.launch {
            runCatching { syncEngine.observeForeground() }
                .onFailure { mutableState.value = mutableState.value.copy(realtimeConnected = false) }
        }
    }

    override fun startLogin() = startPlatformLogin()
    override fun updateQuery(value: String) {
        mutableState.value = mutableState.value.copy(filter = mutableState.value.filter.copy(query = value), closedPage = 1)
        scope.launch { preferencesStore?.update { current -> current.copy(query = value) } }
    }
    override fun updateFilter(filter: org.sitcon.sitlab.domain.BoardFilter) {
        mutableState.value = mutableState.value.copy(filter = filter, closedPage = 1)
        scope.launch { preferencesStore?.update { current -> current.copy(query = filter.query, teamKeys = filter.teamKeys, memberIds = filter.memberIds, labels = filter.labels) } }
    }
    override fun navigate(route: Route) { mutableState.value = mutableState.value.copy(route = route) }
    override fun openCard(issueIid: Long) {
        if (mutableState.value.cards.any { it.issueIid == issueIid }) navigate(Route.CardDetail(issueIid))
        else {
            pendingCardIid = issueIid
            refresh()
        }
    }

    override fun moveCard(issueIid: Long, listKey: String?) {
        val target = listKey ?: mutableState.value.lists.firstOrNull { it.key != mutableState.value.cards.firstOrNull { card -> card.issueIid == issueIid }?.listKey }?.key ?: return
        val card = mutableState.value.cards.firstOrNull { it.issueIid == issueIid } ?: return
        val operationId = newOperationId()
        scope.launchMutation(operationId, "move_card", issueIid, json.encodeToString(MoveCardRequest(operationId, target))) {
            val entity = database.dao().card(issueIid) ?: return@launchMutation
            database.dao().upsertCards(listOf(entity.copy(listKey = target, syncState = "pending", pendingOperationId = operationId, syncError = null)))
            val result = api.moveCard(issueIid, csrf(), MoveCardRequest(operationId, target))
            database.dao().upsertCards(listOf(result.card.toEntity()))
            haptics?.perform(HapticEffect.Confirm)
        }
    }

    override fun saveCard(issueIid: Long, title: String, description: String) {
        val operationId = newOperationId()
        val request = UpdateCardDetailsRequest(operationId, title.trim(), description)
        scope.launchMutation(operationId, "update_details", issueIid, json.encodeToString(request)) {
            val entity = database.dao().card(issueIid) ?: return@launchMutation
            database.dao().upsertCards(listOf(entity.copy(title = request.title, description = description, syncState = "pending", pendingOperationId = operationId, syncError = null)))
            val result = api.updateDetails(issueIid, csrf(), request)
            database.dao().upsertCards(listOf(result.card.toEntity()))
            haptics?.perform(HapticEffect.Success)
        }
    }

    override fun createCard(title: String, teamKey: String, listKey: String) {
        val operationId = newOperationId()
        val temporaryIid = -kotlin.random.Random.nextLong(1, Long.MAX_VALUE)
        val request = CreateCardRequest(operationId, title.trim(), teamKey, listKey, "", emptyList(), emptyList(), null, null)
        scope.launchMutation(operationId, "create_card", temporaryIid, json.encodeToString(request)) {
            val now = "1970-01-01T00:00:00Z"
            database.dao().upsertCards(listOf(CardEntity(temporaryIid, null, request.title, "", null, listKey, Int.MAX_VALUE, teamKey, "[]", null, null, "[]", null, "pending", null, operationId, now, now)))
            mutableState.value = mutableState.value.copy(route = Route.CardDetail(temporaryIid))
            val result = api.createCard(csrf(), request)
            database.dao().deleteCard(temporaryIid)
            database.dao().upsertCards(listOf(result.card.toEntity()))
            if (mutableState.value.route == Route.CardDetail(temporaryIid)) mutableState.value = mutableState.value.copy(route = Route.CardDetail(result.card.issueIid))
            haptics?.perform(HapticEffect.Success)
        }
    }

    override fun deleteCard(issueIid: Long) {
        val operationId = newOperationId()
        scope.launchMutation(operationId, "delete_card", issueIid, json.encodeToString(DeleteCardRequest(operationId))) {
            api.deleteCard(issueIid, csrf(), DeleteCardRequest(operationId))
            database.dao().deleteCard(issueIid)
            mutableState.value = mutableState.value.copy(route = Route.Board)
        }
    }

    override fun refresh() {
        if (!mutableState.value.authenticated || mutableState.value.syncing) return
        mutableState.value = mutableState.value.copy(syncing = true, error = null)
        scope.launch {
            runCatching { syncEngine.refresh() }
                .onSuccess { mutableState.value = mutableState.value.copy(syncing = false, error = null, realtimeConnected = true) }
                .onFailure { error ->
                    if (error is ApiProblem && error.problem.status == 401) expireSession()
                    else mutableState.value = mutableState.value.copy(syncing = false, error = error.message ?: "Synchronization failed")
                }
        }
    }

    override fun logout() {
        scope.launch {
            runCatching { api.logout(csrf()) }
            expireSession()
        }
    }

    override fun retryPending() {
        scope.launch {
            database.dao().pendingRequests().filter { it.permanentError != null }.forEach {
                database.dao().upsertPendingRequest(it.copy(permanentError = null))
            }
            replayPendingRequests()
            refresh()
        }
    }

    override fun loadCardActivity(issueIid: Long) {
        scope.launch {
            runCatching { api.comments(issueIid).comments }
                .onSuccess { values -> mutableState.value = mutableState.value.copy(comments = mutableState.value.comments + (issueIid to values)) }
                .onFailure { mutableState.value = mutableState.value.copy(error = it.message) }
        }
    }

    override fun addComment(issueIid: Long, body: String) {
        scope.launch {
            runCatching { api.createComment(issueIid, csrf(), CreateCardCommentRequest(body.trim())) }
                .onSuccess { loadCardActivity(issueIid); haptics?.perform(HapticEffect.Success) }
                .onFailure { mutableState.value = mutableState.value.copy(error = it.message); haptics?.perform(HapticEffect.Error) }
        }
    }

    override fun loadRelationships(issueIid: Long) {
        scope.launch {
            val children = runCatching { api.childItems(issueIid).items }.getOrElse { emptyList() }
            val links = runCatching { api.linkedItems(issueIid).items }.getOrElse { emptyList() }
            mutableState.value = mutableState.value.copy(
                childItems = mutableState.value.childItems + (issueIid to children),
                linkedItems = mutableState.value.linkedItems + (issueIid to links),
            )
        }
    }

    override fun loadLabels() {
        scope.launch {
            runCatching { api.labels().labels }
                .onSuccess { mutableState.value = mutableState.value.copy(projectLabels = it) }
                .onFailure { mutableState.value = mutableState.value.copy(error = it.message) }
        }
    }

    override fun createLabel(name: String, color: String) {
        scope.launch {
            runCatching { api.createLabel(csrf(), CreateProjectLabelRequest(name.trim(), color.trim(), null)) }
                .onSuccess { loadLabels(); haptics?.perform(HapticEffect.Success) }
                .onFailure { mutableState.value = mutableState.value.copy(error = it.message); haptics?.perform(HapticEffect.Error) }
        }
    }

    private suspend fun loadPreferences(): org.sitcon.sitlab.persistence.MobilePreferences? {
        val preferences = preferencesStore?.read() ?: return null
        mutableState.value = mutableState.value.copy(
            appearance = preferences.appearance,
            colorStyle = preferences.colorStyle,
            backgroundHours = preferences.backgroundHours,
            reminderLeadDays = preferences.reminderLeadDays,
            onboardingComplete = preferences.onboardingComplete,
            filter = mutableState.value.filter.copy(
                query = preferences.query,
                teamKeys = preferences.teamKeys,
                memberIds = preferences.memberIds,
                labels = preferences.labels,
            ),
        )
        return preferences
    }

    override fun setAppearance(value: org.sitcon.sitlab.persistence.Appearance) = updatePreferences { copy(appearance = value) }
    override fun setColorStyle(value: org.sitcon.sitlab.persistence.ColorStyle) = updatePreferences { copy(colorStyle = value) }
    override fun setBackgroundHours(value: Int?) {
        updatePreferences { copy(backgroundHours = value) }
        scope.launch {
            val interval = BackgroundInterval.entries.first { it.hours == value }
            if (value == null) backgroundRefresh?.cancel() else backgroundRefresh?.replace(interval)
        }
    }
    override fun setReminderLeadDays(value: Int?) {
        updatePreferences { copy(reminderLeadDays = value) }
        scope.launch { remindersChanged() }
    }

    override fun completeOnboarding() {
        updatePreferences { copy(onboardingComplete = true) }
        mutableState.value = mutableState.value.copy(onboardingComplete = true, route = Route.Board)
    }

    override fun requestNotificationPermission() {
        scope.launch {
            val status = notifications?.requestPermission() ?: PermissionStatus.NotDetermined
            mutableState.value = mutableState.value.copy(notificationPermission = status)
            if (status == PermissionStatus.Granted) remindersChanged()
        }
    }

    override fun openNotificationSettings() {
        scope.launch { notifications?.openSystemSettings() }
    }

    override fun shareCard(issueIid: Long) {
        val card = mutableState.value.cards.firstOrNull { it.issueIid == issueIid } ?: return
        scope.launch { shareSheet?.shareText(card.webUrl ?: "$ProductionOrigin/mobile/cards/$issueIid") }
    }

    override fun updateCardTeam(issueIid: Long, teamKey: String) {
        val operationId = newOperationId()
        val request = UpdateCardTeamRequest(operationId, teamKey)
        scope.launchMutation(operationId, "update_team", issueIid, json.encodeToString(request)) {
            val entity = database.dao().card(issueIid) ?: return@launchMutation
            database.dao().upsertCards(listOf(entity.copy(teamKey = teamKey, syncState = "pending", pendingOperationId = operationId, syncError = null)))
            database.dao().upsertCards(listOf(api.updateTeam(issueIid, csrf(), request).card.toEntity()))
        }
    }

    override fun updateCardAssignees(issueIid: Long, memberIds: List<Long>) {
        val operationId = newOperationId()
        val request = UpdateCardAssigneeRequest(operationId, memberIds.distinct())
        scope.launchMutation(operationId, "update_assignees", issueIid, json.encodeToString(request)) {
            val entity = database.dao().card(issueIid) ?: return@launchMutation
            database.dao().upsertCards(listOf(entity.copy(assigneeIdsJson = json.encodeToString(request.assigneeGitLabUserIds), syncState = "pending", pendingOperationId = operationId, syncError = null)))
            database.dao().upsertCards(listOf(api.updateAssignees(issueIid, csrf(), request).card.toEntity()))
        }
    }

    override fun updateCardDates(issueIid: Long, startDate: String?, dueDate: String?) {
        val current = mutableState.value.cards.firstOrNull { it.issueIid == issueIid } ?: return
        scope.launch {
            if (startDate != current.startDate) updateStartDate(issueIid, startDate)
            if (dueDate != current.dueDate) updateDueDate(issueIid, dueDate)
        }
    }

    private suspend fun updateStartDate(issueIid: Long, value: String?) {
        val operationId = newOperationId()
        val request = UpdateCardStartDateRequest(operationId, value)
        executeMutation(operationId, "update_start_date", issueIid, json.encodeToString(request)) {
            val entity = database.dao().card(issueIid) ?: return@executeMutation
            database.dao().upsertCards(listOf(entity.copy(startDate = value, syncState = "pending", pendingOperationId = operationId, syncError = null)))
            database.dao().upsertCards(listOf(api.updateStartDate(issueIid, csrf(), request).card.toEntity()))
        }
    }

    private suspend fun updateDueDate(issueIid: Long, value: String?) {
        val operationId = newOperationId()
        val request = UpdateCardDueDateRequest(operationId, value)
        executeMutation(operationId, "update_due_date", issueIid, json.encodeToString(request)) {
            val entity = database.dao().card(issueIid) ?: return@executeMutation
            database.dao().upsertCards(listOf(entity.copy(dueDate = value, syncState = "pending", pendingOperationId = operationId, syncError = null)))
            database.dao().upsertCards(listOf(api.updateDueDate(issueIid, csrf(), request).card.toEntity()))
        }
    }

    override fun updateCardLabels(issueIid: Long, labels: List<String>) {
        val operationId = newOperationId()
        val request = UpdateCardLabelsRequest(operationId, labels.distinct())
        scope.launchMutation(operationId, "update_labels", issueIid, json.encodeToString(request)) {
            val entity = database.dao().card(issueIid) ?: return@launchMutation
            database.dao().upsertCards(listOf(entity.copy(labelsJson = json.encodeToString(request.labels), syncState = "pending", pendingOperationId = operationId, syncError = null)))
            database.dao().upsertCards(listOf(api.updateLabels(issueIid, csrf(), request).card.toEntity()))
        }
    }

    override fun loadMoreClosedCards() {
        mutableState.value = mutableState.value.copy(closedPage = mutableState.value.closedPage + 1)
    }

    private fun updatePreferences(transform: org.sitcon.sitlab.persistence.MobilePreferences.() -> org.sitcon.sitlab.persistence.MobilePreferences) {
        scope.launch {
            preferencesStore?.update { current -> transform(current) }
            loadPreferences()
        }
    }

    private suspend fun expireSession() {
        realtimeJob?.cancel(); realtimeJob = null
        sessionStore.clear()
        sessionCleared()
        val dao = database.dao()
        dao.clearCards(); dao.clearLists(); dao.clearTeams(); dao.clearMembers(); dao.clearMilestones()
        dao.clearMetadata(); dao.clearPendingRequests(); dao.clearActivityCache(); dao.clearNotificationLedger()
        mutableState.value = AppUiState(route = Route.Login)
    }

    private suspend fun csrf(): String = database.dao().metadata(CsrfKey)?.value ?: api.csrf().token

    private fun CoroutineScope.launchMutation(operationId: String, kind: String, issueIid: Long?, payload: String, block: suspend () -> Unit) =
        launch { executeMutation(operationId, kind, issueIid, payload, block) }

    private suspend fun executeMutation(operationId: String, kind: String, issueIid: Long?, payload: String, block: suspend () -> Unit) = mutationMutex.withLock {
        val pending = PendingRequestEntity(operationId, kind, issueIid, payload, kotlin.time.Clock.System.now().toEpochMilliseconds(), 0, null)
        database.dao().upsertPendingRequest(pending)
        runCatching { block() }
            .onSuccess { database.dao().deletePendingRequest(operationId); remindersChanged() }
            .onFailure { error ->
                val permanent = (error as? ApiProblem)?.problem?.status?.let { it in 400..499 && it != 409 }
                database.dao().upsertPendingRequest(pending.copy(attempts = 1, permanentError = if (permanent == true) error.message else null))
                issueIid?.let { iid -> database.dao().card(iid)?.let { database.dao().upsertCards(listOf(it.copy(syncState = "failed", syncError = error.message))) } }
                haptics?.perform(HapticEffect.Error)
            }
    }

    private data class CacheSnapshot(val lists: List<BoardList>, val cards: List<Card>, val teams: List<Team>, val members: List<Member>)

    private fun MemberEntity.toDomain() = Member(gitLabUserId, username, displayName, json.decodeFromString(teamKeysJson))
    private fun CardEntity.toDomain(teamNames: Map<String, String>, members: Map<Long, Member>) = Card(
        issueIid, title, description, listKey, position, teamKey, teamNames[teamKey] ?: teamKey,
        json.decodeFromString<List<Long>>(assigneeIdsJson).mapNotNull(members::get), startDate, dueDate,
        json.decodeFromString(labelsJson), updatedAt, syncState == "synced", syncError, webUrl,
        gitLabStatusName, pendingOperationId,
    )
    private fun BoardCard.toEntity() = CardEntity(
        issueIid, issueId, title, description, webUrl, listKey, position, teamKey,
        json.encodeToString(assigneeGitLabUserIds), startDate, dueDate, json.encodeToString(labels),
        gitLabStatusName, syncState, syncError, pendingOperationId, createdAt, updatedAt,
    )

    private companion object {
        const val CsrfKey = "csrf_token"
        const val CurrentUserKey = "current_user"
        const val LastSyncKey = "last_successful_sync"
    }
}
