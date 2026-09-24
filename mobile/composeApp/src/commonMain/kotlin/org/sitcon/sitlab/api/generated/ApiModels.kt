// Generated from api/**/*.tsp via OpenAPI. DO NOT EDIT.
@file:OptIn(ExperimentalSerializationApi::class)

package org.sitcon.sitlab.api.generated

import kotlinx.serialization.ExperimentalSerializationApi
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonClassDiscriminator
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

@Serializable
@JsonClassDiscriminator("entity")
sealed interface SyncAction {
    val syncId: String
    val entityId: String
    val operation: String
    val actorGitLabUserId: Long?
    val occurredAt: String
}

@Serializable
@SerialName("card")
data class CardSyncAction(
    override val syncId: String,
    override val entityId: String,
    override val operation: String,
    override val actorGitLabUserId: Long?,
    override val occurredAt: String,
    val card: BoardCard?
) : SyncAction

@Serializable
@SerialName("cardOrder")
data class CardOrderSyncAction(
    override val syncId: String,
    override val entityId: String,
    override val operation: String,
    override val actorGitLabUserId: Long?,
    override val occurredAt: String,
    val order: CardOrder
) : SyncAction

@Serializable
@SerialName("list")
data class ListSyncAction(
    override val syncId: String,
    override val entityId: String,
    override val operation: String,
    override val actorGitLabUserId: Long?,
    override val occurredAt: String,
    val list: BoardList?
) : SyncAction

@Serializable
@SerialName("team")
data class TeamSyncAction(
    override val syncId: String,
    override val entityId: String,
    override val operation: String,
    override val actorGitLabUserId: Long?,
    override val occurredAt: String,
    val team: DirectoryTeam?
) : SyncAction

@Serializable
@SerialName("member")
data class MemberSyncAction(
    override val syncId: String,
    override val entityId: String,
    override val operation: String,
    override val actorGitLabUserId: Long?,
    override val occurredAt: String,
    val member: DirectoryMember?
) : SyncAction

@Serializable
@SerialName("preference")
data class PreferenceSyncAction(
    override val syncId: String,
    override val entityId: String,
    override val operation: String,
    override val actorGitLabUserId: Long?,
    override val occurredAt: String,
    val preferences: UserPreferences
) : SyncAction

@Serializable
@SerialName("syncStatus")
data class SyncStatusSyncAction(
    override val syncId: String,
    override val entityId: String,
    override val operation: String,
    override val actorGitLabUserId: Long?,
    override val occurredAt: String,
    val sync: SyncStatus
) : SyncAction

@Serializable
@SerialName("milestone")
data class MilestoneSyncAction(
    override val syncId: String,
    override val entityId: String,
    override val operation: String,
    override val actorGitLabUserId: Long?,
    override val occurredAt: String,
    val milestones: List<DirectoryMilestone>
) : SyncAction

@Serializable
data class APIStatusResponse(
    val name: String,
    val version: String
)

@Serializable
data class AuthResponse(
    val user: CurrentUser
)

@Serializable
data class BoardCard(
    val issueIid: Long,
    val issueId: Long?,
    val title: String,
    val description: String,
    val webUrl: String?,
    val listKey: String,
    val position: Int,
    val teamKey: String,
    val assigneeGitLabUserIds: List<Long>,
    val startDate: String?,
    val dueDate: String?,
    val labels: List<String>,
    val gitLabStatusName: String?,
    val syncState: String,
    val syncError: String?,
    val pendingOperationId: uuid?,
    val createdAt: String,
    val updatedAt: String
)

@Serializable
data class BoardList(
    val key: String,
    val name: String,
    val position: Int,
    val closed: Boolean,
    val color: String
)

@Serializable
data class BoardSnapshot(
    val lists: List<BoardList>,
    val cards: List<BoardCard>,
    val syncedAt: String
)

@Serializable
data class BootstrapResponse(
    val revision: String,
    val me: CurrentUser,
    val csrfToken: String,
    val teams: List<DirectoryTeam>,
    val members: List<DirectoryMember>,
    val milestones: List<DirectoryMilestone>,
    val board: BoardSnapshot,
    val preferences: UserPreferences,
    val sync: SyncStatus
)

@Serializable
data class CSRFResponse(
    val token: String
)

@Serializable
data class CardComment(
    val id: Long,
    val body: String,
    val author: CardCommentAuthor,
    val system: Boolean,
    val createdAt: String,
    val updatedAt: String
)

@Serializable
data class CardCommentAuthor(
    val gitLabUserId: Long,
    val username: String,
    val displayName: String,
    val avatarUrl: String?,
    val profileUrl: String
)

@Serializable
data class CardCommentsResponse(
    val comments: List<CardComment>
)

@Serializable
data class CardDeletionResponse(
    val operation: DurableOperation
)

@Serializable
data class CardMutationResponse(
    val card: BoardCard,
    val operation: DurableOperation
)

@Serializable
data class CardOrder(
    val listKey: String,
    val issueIids: List<Long>
)

@Serializable
data class ChildItemsResponse(
    val items: List<WorkItemSummary>,
    val totalCount: Int,
    val nextCursor: String?
)

@Serializable
data class CreateCardCommentRequest(
    val body: String
)

@Serializable
data class CreateCardCommentResult(
    val comment: CardComment?,
    val quickActionsApplied: Boolean,
    val summary: List<String>
)

@Serializable
data class CreateCardRequest(
    val operationId: uuid,
    val title: String,
    val teamKey: String,
    val listKey: String,
    val description: String,
    val assigneeGitLabUserIds: List<Long>,
    val labels: List<String>,
    val startDate: String?,
    val dueDate: String?
)

@Serializable
data class CreateChildItemRequest(
    val title: String
)

@Serializable
data class CreateLinkedItemsRequest(
    val workItemIds: List<Long>,
    val linkType: String
)

@Serializable
data class CreateProjectLabelRequest(
    val name: String,
    val color: String,
    val description: String?
)

@Serializable
data class CurrentUser(
    val id: uuid,
    val gitLabUserId: Long,
    val username: String,
    val displayName: String,
    val avatarUrl: String?,
    val profileUrl: String,
    val accessLevel: Int
)

@Serializable
data class DeleteCardRequest(
    val operationId: uuid
)

@Serializable
data class DirectoryMember(
    val gitLabUserId: Long,
    val username: String,
    val displayName: String,
    val avatarUrl: String?,
    val profileUrl: String,
    val accessLevel: Int,
    val state: String,
    val teamKeys: List<String>
)

@Serializable
data class DirectoryMilestone(
    val name: String,
    val date: String,
    val kind: String
)

@Serializable
data class DirectoryResponse(
    val directory: DirectorySnapshot
)

@Serializable
data class DirectorySnapshot(
    val teams: List<DirectoryTeam>,
    val members: List<DirectoryMember>,
    val milestones: List<DirectoryMilestone>,
    val sourceRevision: String,
    val syncedAt: String
)

@Serializable
data class DirectoryTeam(
    val key: String,
    val name: String,
    val titlePrefix: String,
    val gitLabLabel: String,
    val active: Boolean,
    val sortOrder: Int,
    val memberGitLabUserIds: List<Long>,
    val leaderGitLabUserIds: List<Long>
)

@Serializable
data class DurableOperation(
    val id: uuid,
    val kind: String,
    val state: String,
    val attempts: Int,
    val lastError: String?,
    val createdAt: String,
    val updatedAt: String
)

@Serializable
data class HealthResponse(
    val status: String
)

@Serializable
data class LinkedItemsResponse(
    val items: List<LinkedWorkItem>,
    val totalCount: Int,
    val nextCursor: String?
)

@Serializable
data class LinkedWorkItem(
    val gitLabWorkItemId: Long,
    val iid: Long,
    val type: String,
    val title: String,
    val state: String,
    val webUrl: String,
    val status: WorkItemStatus?,
    val assignees: List<WorkItemAssignee>,
    val startDate: String?,
    val dueDate: String?,
    val labels: List<WorkItemLabel>,
    val linkType: String
)

@Serializable
data class MobileOAuthExchangeRequest(
    val code: String,
    val state: String,
    val codeVerifier: String
)

@Serializable
data class MobileOAuthExchangeResult(
    val authenticated: Boolean
)

@Serializable
data class MoveCardRequest(
    val operationId: uuid,
    val listKey: String,
    val position: Int? = null
)

@Serializable
data class PreferencesResponse(
    val preferences: UserPreferences
)

@Serializable
data class ProblemDetails(
    val type: String,
    val title: String,
    val status: Int,
    val code: String,
    val detail: String? = null,
    val requestId: String? = null,
    val errors: List<ProblemError>? = null
)

@Serializable
data class ProblemError(
    val code: String,
    val message: String,
    val location: String? = null
)

@Serializable
data class ProjectLabel(
    val id: Long,
    val name: String,
    val color: String,
    val textColor: String,
    val description: String?
)

@Serializable
data class ProjectLabelsResponse(
    val labels: List<ProjectLabel>
)

@Serializable
data class QuickActionCommand(
    val name: String,
    val aliases: List<String>,
    val params: List<String>,
    val description: String?,
    val warning: String?,
    val icon: String?
)

@Serializable
data class QuickActionCommandsResponse(
    val commands: List<QuickActionCommand>
)

@Serializable
data class QuickActionSuggestion(
    val id: String,
    val kind: String,
    val value: String,
    val label: String,
    val detail: String?,
    val avatarUrl: String?,
    val color: String?
)

@Serializable
data class QuickActionSuggestionsResponse(
    val suggestions: List<QuickActionSuggestion>
)

@Serializable
data class RefreshSyncResponse(
    val acceptedAt: String
)

@Serializable
data class RetryOperationResponse(
    val operation: DurableOperation
)

@Serializable
data class SyncDeltaResponse(
    val checkpoint: String,
    val actions: List<SyncAction>,
    val hasMore: Boolean
)

@Serializable
data class SyncStatus(
    val state: String,
    val lastSuccessAt: String,
    val message: String?
)

@Serializable
data class UpdateCardAssigneeRequest(
    val operationId: uuid,
    val assigneeGitLabUserIds: List<Long>
)

@Serializable
data class UpdateCardDetailsRequest(
    val operationId: uuid,
    val title: String,
    val description: String
)

@Serializable
data class UpdateCardDueDateRequest(
    val operationId: uuid,
    val dueDate: String?
)

@Serializable
data class UpdateCardLabelsRequest(
    val operationId: uuid,
    val labels: List<String>
)

@Serializable
data class UpdateCardStartDateRequest(
    val operationId: uuid,
    val startDate: String?
)

@Serializable
data class UpdateCardTeamRequest(
    val operationId: uuid,
    val teamKey: String
)

@Serializable
data class UpdatePreferencesRequest(
    val defaultTeamKey: String
)

@Serializable
data class UpdateProjectLabelRequest(
    val name: String,
    val color: String,
    val description: String?
)

@Serializable
data class UserPreferences(
    val defaultTeamKey: String?,
    val confirmedAt: String?,
    val directoryTeamKeys: List<String>
)

@Serializable
data class WebhookAcceptedResponse(
    val accepted: Boolean,
    val duplicate: Boolean
)

@Serializable
data class WorkItemAssignee(
    val gitLabUserId: Long,
    val username: String,
    val displayName: String,
    val avatarUrl: String?,
    val profileUrl: String
)

@Serializable
data class WorkItemCandidatesResponse(
    val items: List<WorkItemSummary>
)

@Serializable
data class WorkItemLabel(
    val name: String,
    val color: String,
    val textColor: String
)

@Serializable
data class WorkItemStatus(
    val name: String,
    val category: String?,
    val color: String?
)

@Serializable
data class WorkItemSummary(
    val gitLabWorkItemId: Long,
    val iid: Long,
    val type: String,
    val title: String,
    val state: String,
    val webUrl: String,
    val status: WorkItemStatus?,
    val assignees: List<WorkItemAssignee>,
    val startDate: String?,
    val dueDate: String?,
    val labels: List<WorkItemLabel>
)

typealias uuid = String
