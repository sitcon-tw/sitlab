package org.sitcon.sitlab.domain

import kotlin.time.Instant
import org.sitcon.sitlab.persistence.NotificationLedgerEntity
import org.sitcon.sitlab.persistence.SitLabDao
import org.sitcon.sitlab.platform.LocalNotifications

class NotificationReconciler(
    private val dao: SitLabDao,
    private val notifications: LocalNotifications,
) {
    suspend fun reconcile(
        cards: List<Card>,
        openListKeys: Set<String>,
        signedInGitLabUserId: Long,
        leadDays: Int?,
        now: Instant,
    ) {
        val ledger = dao.notificationLedger()
        val delivered = ledger.filter { it.deliveredAtEpochMillis != null }.mapTo(mutableSetOf()) { it.notificationId }
        val pending = notifications.pendingIds()
        val plan = planReminders(cards, openListKeys, signedInGitLabUserId, leadDays, now, pending, delivered)
        notifications.cancel(plan.obsoleteIds)
        dao.deleteNotificationLedger(plan.obsoleteIds)
        (plan.scheduled + plan.deliverImmediately).forEach { notification ->
            notifications.schedule(notification)
            val parts = notification.id.split(':')
            dao.upsertNotificationLedger(listOf(NotificationLedgerEntity(
                notificationId = notification.id,
                issueIid = parts[1].toLong(),
                dueDate = parts[2],
                leadDays = parts[3].toInt(),
                deliveredAtEpochMillis = if (notification in plan.deliverImmediately) now.toEpochMilliseconds() else null,
            )))
        }
    }

    suspend fun clearOnLogout() {
        notifications.cancel(notifications.pendingIds())
        dao.clearNotificationLedger()
    }
}
