package org.sitcon.sitlab.domain

import kotlinx.datetime.DatePeriod
import kotlinx.datetime.LocalDate
import kotlinx.datetime.LocalDateTime
import kotlinx.datetime.TimeZone
import kotlinx.datetime.atTime
import kotlinx.datetime.minus
import kotlinx.datetime.toInstant
import kotlin.time.Instant
import org.sitcon.sitlab.platform.LocalNotification

private val Taipei = TimeZone.of("Asia/Taipei")

data class ReminderPlan(
    val scheduled: List<LocalNotification>,
    val deliverImmediately: List<LocalNotification>,
    val obsoleteIds: Set<String>,
)

fun planReminders(
    cards: List<Card>,
    openListKeys: Set<String>,
    signedInGitLabUserId: Long,
    leadDays: Int?,
    now: Instant,
    pendingIds: Set<String>,
    deliveredIds: Set<String>,
): ReminderPlan {
    if (leadDays == null) return ReminderPlan(emptyList(), emptyList(), pendingIds)
    val candidates = cards.mapNotNull { card ->
        val due = card.dueDate?.let(LocalDate::parse) ?: return@mapNotNull null
        if (!card.synchronized || card.listKey !in openListKeys || card.assignees.none { it.gitLabUserId == signedInGitLabUserId }) return@mapNotNull null
        val id = reminderId(card.issueIid, due, leadDays)
        if (id in deliveredIds) return@mapNotNull null
        val trigger = due.minus(DatePeriod(days = leadDays)).atTime(9, 0).toInstant(Taipei)
        val dueInstant = due.atTime(23, 59, 59).toInstant(Taipei)
        if (now >= dueInstant) return@mapNotNull null
        val notification = LocalNotification(
            id = id,
            title = "#${card.issueIid} ${card.title}",
            body = if (leadDays == 0) "Due today · ${card.dueDate}" else "Due ${card.dueDate} · $leadDays day${if (leadDays == 1) "" else "s"} remaining",
            triggerEpochMillis = trigger.toEpochMilliseconds(),
            deepLink = "https://sitlab.sitcon.org/mobile/cards/${card.issueIid}",
        )
        trigger to notification
    }
    val activeIds = candidates.mapTo(mutableSetOf()) { it.second.id }
    return ReminderPlan(
        scheduled = candidates.filter { (trigger) -> trigger > now }.map { it.second },
        deliverImmediately = candidates.filter { (trigger) -> trigger <= now }.map { it.second.copy(triggerEpochMillis = now.toEpochMilliseconds()) },
        obsoleteIds = pendingIds - activeIds,
    )
}

fun reminderId(issueIid: Long, dueDate: LocalDate, leadDays: Int): String =
    "deadline:$issueIid:$dueDate:$leadDays"
