package org.sitcon.sitlab

import kotlinx.coroutines.flow.first
import kotlinx.serialization.json.Json
import org.sitcon.sitlab.api.generated.CurrentUser
import org.sitcon.sitlab.domain.Card
import org.sitcon.sitlab.domain.Member
import org.sitcon.sitlab.domain.NotificationReconciler
import org.sitcon.sitlab.persistence.MemberEntity
import org.sitcon.sitlab.persistence.SitLabDatabase

class IosReminderCoordinator(
    private val database: SitLabDatabase,
    private val preferences: IosPreferencesStore,
    notifications: IosLocalNotifications,
) {
    private val json = Json { ignoreUnknownKeys = true }
    private val reconciler = NotificationReconciler(database.dao(), notifications)

    suspend fun reconcile() {
        val dao = database.dao()
        val currentUser = dao.metadata("current_user")?.value
            ?.let { runCatching { json.decodeFromString<CurrentUser>(it) }.getOrNull() }
            ?: return
        val lists = dao.observeLists().first()
        val members = dao.observeMembers().first().associate { it.gitLabUserId to it.toDomain() }
        val cards = dao.allCards().map { entity ->
            Card(
                entity.issueIid, entity.title, entity.description, entity.listKey, entity.position,
                entity.teamKey, entity.teamKey,
                json.decodeFromString<List<Long>>(entity.assigneeIdsJson).mapNotNull(members::get),
                entity.startDate, entity.dueDate, json.decodeFromString(entity.labelsJson),
                entity.updatedAt, entity.syncState == "synced", entity.syncError, entity.webUrl,
                entity.gitLabStatusName, entity.pendingOperationId,
            )
        }
        reconciler.reconcile(
            cards = cards,
            openListKeys = lists.filterNot { it.closed }.mapTo(mutableSetOf()) { it.key },
            signedInGitLabUserId = currentUser.gitLabUserId,
            leadDays = preferences.read().reminderLeadDays,
            now = kotlin.time.Clock.System.now(),
        )
    }

    suspend fun clear() = reconciler.clearOnLogout()

    private fun MemberEntity.toDomain() =
        Member(gitLabUserId, username, displayName, json.decodeFromString(teamKeysJson))
}
