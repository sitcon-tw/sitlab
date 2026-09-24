package org.sitcon.sitlab.domain

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertIs
import kotlin.test.assertTrue
import kotlinx.datetime.LocalDate
import kotlin.time.Instant

class BoardModelsTest {
    private val alice = Member(101, "alice", "Alice Chen", setOf("dev"))
    private val cards = listOf(
        Card(1, "Ship OAuth", "Mobile callback", "doing", 1, "dev", "Development", listOf(alice), "2026-09-20", "2026-09-24", listOf("mobile", "urgent"), "2026-09-23T10:00:00Z"),
        Card(2, "Write docs", "Deployment", "todo", 2, "ops", "Operations", emptyList(), null, "2026-09-30", listOf("docs"), "2026-09-22T10:00:00Z"),
    )

    @Test fun searchUsesNormalizedAndMatchingAcrossFields() {
        val result = filterAndSortCards(cards, BoardFilter(query = "ALICE urgent"))
        assertEquals(listOf(1L), result.map(Card::issueIid))
        assertTrue(filterAndSortCards(cards, BoardFilter(query = "#1 development")).isEmpty())
        assertEquals(listOf(1L), filterAndSortCards(cards, BoardFilter(query = "1 development")).map(Card::issueIid))
    }

    @Test fun teamMemberAndLabelsAreCombined() {
        val result = filterAndSortCards(cards, BoardFilter(
            teamKeys = setOf("dev"), memberIds = setOf(101), labels = setOf("mobile", "urgent"),
        ))
        assertEquals(listOf(1L), result.map(Card::issueIid))
        assertTrue(filterAndSortCards(cards, BoardFilter(labels = setOf("mobile", "docs"))).isEmpty())
    }

    @Test fun rekeyKeepsSelectedDetailOnCreateCompletion() {
        val temporary = cards[0].copy(issueIid = -1, synchronized = false)
        val result = OptimisticCardState(listOf(temporary), -1).rekeyTemporaryCard(-1, cards[0])
        assertEquals(1, result.selectedIssueIid)
        assertEquals(1, result.cards.single().issueIid)
    }

    @Test fun ganttIsTeamGroupedAndMarksInvalidRanges() {
        val groups = ganttGroups(cards + cards[0].copy(issueIid = 3, startDate = "2026-10-02", dueDate = "2026-10-01"), listOf(
            BoardList("closed", "Close", 5, true, "#000000"),
        ))
        assertEquals(2, groups.size)
        assertTrue(groups.flatMap(GanttGroup::spans).filterIsInstance<GanttSpan.Range>().any(GanttSpan.Range::invalid))
    }

    @Test fun remindersScheduleAtNineTaipeiAndDeduplicateLateDelivery() {
        val card = cards[0].copy(dueDate = "2026-09-24")
        val before = planReminders(
            listOf(card), setOf("doing"), 101, 1,
            Instant.parse("2026-09-22T12:00:00Z"), emptySet(), emptySet(),
        )
        assertEquals(Instant.parse("2026-09-23T01:00:00Z").toEpochMilliseconds(), before.scheduled.single().triggerEpochMillis)

        val late = planReminders(
            listOf(card), setOf("doing"), 101, 1,
            Instant.parse("2026-09-23T02:00:00Z"), emptySet(), emptySet(),
        )
        assertEquals(1, late.deliverImmediately.size)
        val delivered = setOf(reminderId(1, LocalDate.parse("2026-09-24"), 1))
        assertTrue(planReminders(listOf(card), setOf("doing"), 101, 1, Instant.parse("2026-09-23T02:00:00Z"), emptySet(), delivered).deliverImmediately.isEmpty())
    }

    @Test fun remindersExcludeClosedUnsyncedAndUnassignedCards() {
        val now = Instant.parse("2026-09-20T00:00:00Z")
        assertTrue(planReminders(cards, setOf("todo"), 101, 1, now, emptySet(), emptySet()).scheduled.isEmpty())
        assertTrue(planReminders(listOf(cards[0].copy(synchronized = false)), setOf("doing"), 101, 1, now, emptySet(), emptySet()).scheduled.isEmpty())
    }
}
