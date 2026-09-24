package org.sitcon.sitlab.sync

import androidx.room.immediateTransaction
import androidx.room.useWriterConnection
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import org.sitcon.sitlab.api.generated.BoardCard
import org.sitcon.sitlab.api.generated.BootstrapResponse
import org.sitcon.sitlab.api.generated.CardOrderSyncAction
import org.sitcon.sitlab.api.generated.CardSyncAction
import org.sitcon.sitlab.api.generated.DirectoryMember
import org.sitcon.sitlab.api.generated.DirectoryMilestone
import org.sitcon.sitlab.api.generated.DirectoryTeam
import org.sitcon.sitlab.api.generated.ListSyncAction
import org.sitcon.sitlab.api.generated.MemberSyncAction
import org.sitcon.sitlab.api.generated.MilestoneSyncAction
import org.sitcon.sitlab.api.generated.PreferenceSyncAction
import org.sitcon.sitlab.api.generated.SyncAction
import org.sitcon.sitlab.api.generated.SyncStatusSyncAction
import org.sitcon.sitlab.api.generated.TeamSyncAction
import org.sitcon.sitlab.network.ApiProblem
import org.sitcon.sitlab.network.SitLabApi
import org.sitcon.sitlab.persistence.BoardListEntity
import org.sitcon.sitlab.persistence.CardEntity
import org.sitcon.sitlab.persistence.MemberEntity
import org.sitcon.sitlab.persistence.MetadataEntity
import org.sitcon.sitlab.persistence.MilestoneEntity
import org.sitcon.sitlab.persistence.SitLabDatabase
import org.sitcon.sitlab.persistence.TeamEntity

class SyncEngine(
    private val api: SitLabApi,
    private val database: SitLabDatabase,
    private val afterSuccessfulSync: suspend (BootstrapResponse?) -> Unit = {},
) {
    private val mutex = Mutex()
    private var dragging = false
    private val delayedOrderActions = mutableListOf<CardOrderSyncAction>()

    suspend fun cachedCheckpoint(): String? = database.dao().metadata(Checkpoint)?.value

    suspend fun refresh() = mutex.withLock {
        val checkpoint = cachedCheckpoint()
        if (checkpoint == null) {
            replaceWithBootstrap(api.bootstrap())
            return@withLock
        }
        runCatching { applyDeltas(checkpoint) }.getOrElse { error ->
            if (error is ApiProblem && error.problem.code == "SYNC_CHECKPOINT_TOO_OLD") {
                replaceWithBootstrap(api.bootstrap())
            } else {
                throw error
            }
        }
        afterSuccessfulSync(null)
    }

    suspend fun observeForeground(): Nothing = api.observeSyncEvents { refresh() }

    suspend fun beginDrag() = mutex.withLock { dragging = true }

    suspend fun endDrag() = mutex.withLock {
        dragging = false
        delayedOrderActions.forEach { applyAction(it) }
        delayedOrderActions.clear()
    }

    private suspend fun applyDeltas(initialCheckpoint: String) {
        var checkpoint = initialCheckpoint
        var hasMore: Boolean
        do {
            val delta = api.sync(checkpoint)
            database.useWriterConnection { connection ->
                connection.immediateTransaction {
                    delta.actions.forEach { action ->
                        if (dragging && action is CardOrderSyncAction) delayedOrderActions += action else applyAction(action)
                    }
                    database.dao().upsertMetadata(MetadataEntity(Checkpoint, delta.checkpoint))
                }
            }
            checkpoint = delta.checkpoint
            hasMore = delta.hasMore
        } while (hasMore)
    }

    private suspend fun replaceWithBootstrap(bootstrap: BootstrapResponse) {
        database.useWriterConnection { connection ->
            connection.immediateTransaction {
                val dao = database.dao()
                dao.clearCards(); dao.clearLists(); dao.clearTeams(); dao.clearMembers(); dao.clearMilestones()
                dao.upsertLists(bootstrap.board.lists.map { BoardListEntity(it.key, it.name, it.position, it.closed, it.color) })
                dao.upsertCards(bootstrap.board.cards.map(::cardEntity))
                dao.upsertTeams(bootstrap.teams.map(::teamEntity))
                dao.upsertMembers(bootstrap.members.map(::memberEntity))
                dao.upsertMilestones(bootstrap.milestones.map(::milestoneEntity))
                dao.upsertMetadata(MetadataEntity(Checkpoint, bootstrap.revision))
                dao.upsertMetadata(MetadataEntity(Csrf, bootstrap.csrfToken))
                dao.upsertMetadata(MetadataEntity(CurrentUser, Json.encodeToString(bootstrap.me)))
                dao.upsertMetadata(MetadataEntity(Preferences, Json.encodeToString(bootstrap.preferences)))
                dao.upsertMetadata(MetadataEntity(LastSuccessfulSync, bootstrap.sync.lastSuccessAt))
            }
        }
        afterSuccessfulSync(bootstrap)
    }

    private suspend fun applyAction(action: SyncAction) {
        val dao = database.dao()
        when (action) {
            is CardSyncAction -> if (action.operation == "delete" || action.card == null) dao.deleteCard(action.entityId.toLong()) else dao.upsertCards(listOf(cardEntity(action.card)))
            is CardOrderSyncAction -> {
                val positions = action.order.issueIids.withIndex().associate { it.value to it.index }
                dao.upsertCards(dao.allCards().map { card -> positions[card.issueIid]?.let { card.copy(listKey = action.order.listKey, position = it) } ?: card })
            }
            is ListSyncAction -> if (action.operation == "delete" || action.list == null) dao.deleteList(action.entityId) else dao.upsertLists(listOf(action.list.let { BoardListEntity(it.key, it.name, it.position, it.closed, it.color) }))
            is TeamSyncAction -> if (action.operation == "delete" || action.team == null) dao.deleteTeam(action.entityId) else dao.upsertTeams(listOf(teamEntity(action.team)))
            is MemberSyncAction -> if (action.operation == "delete" || action.member == null) dao.deleteMember(action.entityId.toLong()) else dao.upsertMembers(listOf(memberEntity(action.member)))
            is MilestoneSyncAction -> {
                dao.clearMilestones()
                dao.upsertMilestones(action.milestones.map(::milestoneEntity))
            }
            is PreferenceSyncAction -> dao.upsertMetadata(MetadataEntity(Preferences, Json.encodeToString(action.preferences)))
            is SyncStatusSyncAction -> dao.upsertMetadata(MetadataEntity(LastSuccessfulSync, action.sync.lastSuccessAt))
        }
    }

    private fun cardEntity(card: BoardCard) = CardEntity(
        card.issueIid, card.issueId, card.title, card.description, card.webUrl, card.listKey, card.position,
        card.teamKey, Json.encodeToString(card.assigneeGitLabUserIds), card.startDate, card.dueDate,
        Json.encodeToString(card.labels), card.gitLabStatusName, card.syncState, card.syncError,
        card.pendingOperationId, card.createdAt, card.updatedAt,
    )

    private fun teamEntity(team: DirectoryTeam) = TeamEntity(team.key, team.name, team.titlePrefix, team.gitLabLabel, team.sortOrder, team.active)
    private fun memberEntity(member: DirectoryMember) = MemberEntity(
        member.gitLabUserId, member.username, member.displayName, member.avatarUrl, member.profileUrl,
        Json.encodeToString(member.teamKeys), member.state,
    )
    private fun milestoneEntity(milestone: DirectoryMilestone) = MilestoneEntity(
        "${milestone.kind}:${milestone.name}", milestone.name, milestone.date, milestone.kind,
    )

    private companion object {
        const val Checkpoint = "sync_checkpoint"
        const val Csrf = "csrf_token"
        const val CurrentUser = "current_user"
        const val Preferences = "user_preferences"
        const val LastSuccessfulSync = "last_successful_sync"
    }
}
