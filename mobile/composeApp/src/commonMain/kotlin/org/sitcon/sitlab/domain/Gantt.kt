package org.sitcon.sitlab.domain

import kotlinx.datetime.LocalDate
import kotlinx.datetime.daysUntil

enum class GanttScale { Day, Week }

sealed interface GanttSpan {
    val card: Card
    data class Range(override val card: Card, val start: LocalDate, val endInclusive: LocalDate, val invalid: Boolean) : GanttSpan
    data class StartOnly(override val card: Card, val date: LocalDate) : GanttSpan
    data class DueOnly(override val card: Card, val date: LocalDate) : GanttSpan
    data class Unscheduled(override val card: Card) : GanttSpan
}

data class GanttGroup(val teamKey: String, val teamName: String, val spans: List<GanttSpan>)

fun ganttGroups(cards: List<Card>, lists: List<BoardList>): List<GanttGroup> {
    val closed = lists.filter(BoardList::closed).mapTo(mutableSetOf(), BoardList::key)
    return cards.filterNot { it.listKey in closed }.groupBy { it.teamKey to it.teamName }
        .entries.sortedBy { it.key.second }.map { (team, teamCards) ->
            GanttGroup(team.first, team.second, teamCards.map(::toGanttSpan))
        }
}

fun toGanttSpan(card: Card): GanttSpan {
    val start = card.startDate?.let(LocalDate::parse)
    val due = card.dueDate?.let(LocalDate::parse)
    return when {
        start != null && due != null -> GanttSpan.Range(card, start, due, start > due)
        start != null -> GanttSpan.StartOnly(card, start)
        due != null -> GanttSpan.DueOnly(card, due)
        else -> GanttSpan.Unscheduled(card)
    }
}

fun GanttSpan.Range.inclusiveDays(): Int = start.daysUntil(endInclusive) + 1
