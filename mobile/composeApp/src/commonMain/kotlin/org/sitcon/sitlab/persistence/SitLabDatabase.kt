package org.sitcon.sitlab.persistence

import androidx.room.ConstructedBy
import androidx.room.Dao
import androidx.room.Database
import androidx.room.Entity
import androidx.room.PrimaryKey
import androidx.room.Query
import androidx.room.RoomDatabase
import androidx.room.RoomDatabaseConstructor
import androidx.room.Upsert
import kotlinx.coroutines.flow.Flow

@Entity(tableName = "board_lists")
data class BoardListEntity(
    @PrimaryKey val key: String,
    val name: String,
    val position: Int,
    val closed: Boolean,
    val color: String,
)

@Entity(tableName = "cards")
data class CardEntity(
    @PrimaryKey val issueIid: Long,
    val issueId: Long?,
    val title: String,
    val description: String,
    val webUrl: String?,
    val listKey: String,
    val position: Int,
    val teamKey: String,
    val assigneeIdsJson: String,
    val startDate: String?,
    val dueDate: String?,
    val labelsJson: String,
    val gitLabStatusName: String?,
    val syncState: String,
    val syncError: String?,
    val pendingOperationId: String?,
    val createdAt: String,
    val updatedAt: String,
)

@Entity(tableName = "teams")
data class TeamEntity(
    @PrimaryKey val key: String,
    val displayName: String,
    val titlePrefix: String,
    val gitLabLabel: String,
    val position: Int,
    val active: Boolean,
)

@Entity(tableName = "members")
data class MemberEntity(
    @PrimaryKey val gitLabUserId: Long,
    val username: String,
    val displayName: String,
    val avatarUrl: String?,
    val profileUrl: String,
    val teamKeysJson: String,
    val state: String,
)

@Entity(tableName = "milestones")
data class MilestoneEntity(@PrimaryKey val key: String, val name: String, val date: String, val kind: String)

@Entity(tableName = "metadata")
data class MetadataEntity(@PrimaryKey val key: String, val value: String)

@Entity(tableName = "pending_requests")
data class PendingRequestEntity(
    @PrimaryKey val operationId: String,
    val requestKind: String,
    val issueIid: Long?,
    val payloadJson: String,
    val createdAtEpochMillis: Long,
    val attempts: Int,
    val permanentError: String?,
)

@Entity(tableName = "activity_cache")
data class ActivityCacheEntity(
    @PrimaryKey val cacheKey: String,
    val payloadJson: String,
    val fetchedAtEpochMillis: Long,
)

@Entity(tableName = "notification_ledger")
data class NotificationLedgerEntity(
    @PrimaryKey val notificationId: String,
    val issueIid: Long,
    val dueDate: String,
    val leadDays: Int,
    val deliveredAtEpochMillis: Long?,
)

@Dao
interface SitLabDao {
    @Query("SELECT * FROM board_lists ORDER BY position") fun observeLists(): Flow<List<BoardListEntity>>
    @Query("SELECT * FROM cards ORDER BY listKey, position, issueIid") fun observeCards(): Flow<List<CardEntity>>
    @Query("SELECT * FROM teams ORDER BY position") fun observeTeams(): Flow<List<TeamEntity>>
    @Query("SELECT * FROM members ORDER BY displayName") fun observeMembers(): Flow<List<MemberEntity>>
    @Query("SELECT * FROM cards WHERE issueIid = :issueIid") suspend fun card(issueIid: Long): CardEntity?
    @Query("SELECT * FROM cards") suspend fun allCards(): List<CardEntity>
    @Query("SELECT * FROM metadata WHERE key = :key") suspend fun metadata(key: String): MetadataEntity?
    @Query("SELECT * FROM metadata WHERE key = :key") fun observeMetadata(key: String): Flow<MetadataEntity?>
    @Query("SELECT * FROM pending_requests ORDER BY createdAtEpochMillis") suspend fun pendingRequests(): List<PendingRequestEntity>
    @Query("SELECT * FROM notification_ledger") suspend fun notificationLedger(): List<NotificationLedgerEntity>
    @Query("SELECT * FROM notification_ledger WHERE notificationId = :id") suspend fun notificationLedger(id: String): NotificationLedgerEntity?

    @Upsert suspend fun upsertLists(values: List<BoardListEntity>)
    @Upsert suspend fun upsertCards(values: List<CardEntity>)
    @Upsert suspend fun upsertTeams(values: List<TeamEntity>)
    @Upsert suspend fun upsertMembers(values: List<MemberEntity>)
    @Upsert suspend fun upsertMilestones(values: List<MilestoneEntity>)
    @Upsert suspend fun upsertMetadata(value: MetadataEntity)
    @Upsert suspend fun upsertPendingRequest(value: PendingRequestEntity)
    @Upsert suspend fun upsertNotificationLedger(values: List<NotificationLedgerEntity>)

    @Query("DELETE FROM cards WHERE issueIid = :issueIid") suspend fun deleteCard(issueIid: Long)
    @Query("DELETE FROM board_lists WHERE key = :key") suspend fun deleteList(key: String)
    @Query("DELETE FROM teams WHERE key = :key") suspend fun deleteTeam(key: String)
    @Query("DELETE FROM members WHERE gitLabUserId = :id") suspend fun deleteMember(id: Long)
    @Query("DELETE FROM pending_requests WHERE operationId = :operationId") suspend fun deletePendingRequest(operationId: String)
    @Query("DELETE FROM notification_ledger WHERE notificationId IN (:ids)") suspend fun deleteNotificationLedger(ids: Set<String>)
    @Query("DELETE FROM notification_ledger") suspend fun clearNotificationLedger()
    @Query("DELETE FROM cards") suspend fun clearCards()
    @Query("DELETE FROM board_lists") suspend fun clearLists()
    @Query("DELETE FROM teams") suspend fun clearTeams()
    @Query("DELETE FROM members") suspend fun clearMembers()
    @Query("DELETE FROM milestones") suspend fun clearMilestones()
    @Query("DELETE FROM metadata") suspend fun clearMetadata()
    @Query("DELETE FROM pending_requests") suspend fun clearPendingRequests()
    @Query("DELETE FROM activity_cache") suspend fun clearActivityCache()
}

@Database(
    entities = [
        BoardListEntity::class,
        CardEntity::class,
        TeamEntity::class,
        MemberEntity::class,
        MilestoneEntity::class,
        MetadataEntity::class,
        PendingRequestEntity::class,
        ActivityCacheEntity::class,
        NotificationLedgerEntity::class,
    ],
    version = 1,
    exportSchema = true,
)
@ConstructedBy(SitLabDatabaseConstructor::class)
abstract class SitLabDatabase : RoomDatabase() {
    abstract fun dao(): SitLabDao
}

interface DatabaseFactory {
    fun create(): SitLabDatabase
}

@Suppress("NO_ACTUAL_FOR_EXPECT")
expect object SitLabDatabaseConstructor : RoomDatabaseConstructor<SitLabDatabase> {
    override fun initialize(): SitLabDatabase
}
