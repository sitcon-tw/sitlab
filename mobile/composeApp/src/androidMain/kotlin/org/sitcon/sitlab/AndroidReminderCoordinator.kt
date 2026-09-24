package org.sitcon.sitlab

import kotlinx.serialization.json.Json
import kotlinx.coroutines.flow.first
import org.sitcon.sitlab.api.generated.CurrentUser
import org.sitcon.sitlab.domain.Card
import org.sitcon.sitlab.domain.Member
import org.sitcon.sitlab.domain.NotificationReconciler
import org.sitcon.sitlab.persistence.MemberEntity
import org.sitcon.sitlab.persistence.SitLabDatabase

class AndroidReminderCoordinator(
    private val database: SitLabDatabase,
    private val preferences: AndroidPreferencesStore,
    notifications: AndroidLocalNotifications,
) {
    private val json = Json { ignoreUnknownKeys = true }
    private val reconciler = NotificationReconciler(database.dao(), notifications)

    suspend fun reconcile() {
        val dao = database.dao()
        val currentUser = dao.metadata("current_user")?.value?.let { json.decodeFromString<CurrentUser>(it) } ?: return
        val lists = dao.observeLists()
        val members = dao.observeMembers()
        val listSnapshot = lists.first()
        val memberSnapshot = members.first()
        val memberModels = memberSnapshot.associate { it.gitLabUserId to it.toDomain() }
        val cards = dao.allCards().map { entity ->
            Card(
                entity.issueIid, entity.title, entity.description, entity.listKey, entity.position, entity.teamKey, entity.teamKey,
                json.decodeFromString<List<Long>>(entity.assigneeIdsJson).mapNotNull(memberModels::get), entity.startDate,
                entity.dueDate, json.decodeFromString(entity.labelsJson), entity.updatedAt, entity.syncState == "synced",
                entity.syncError, entity.webUrl, entity.gitLabStatusName, entity.pendingOperationId,
            )
        }
        reconciler.reconcile(
            cards = cards,
            openListKeys = listSnapshot.filterNot { it.closed }.mapTo(mutableSetOf()) { it.key },
            signedInGitLabUserId = currentUser.gitLabUserId,
            leadDays = preferences.read().reminderLeadDays,
            now = kotlin.time.Clock.System.now(),
        )
    }

    suspend fun clear() = reconciler.clearOnLogout()

    private fun MemberEntity.toDomain() = Member(gitLabUserId, username, displayName, json.decodeFromString(teamKeysJson))
}
