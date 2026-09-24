package org.sitcon.sitlab.domain

data class BoardList(val key: String, val name: String, val position: Int, val closed: Boolean, val color: String)

data class Member(
    val gitLabUserId: Long,
    val username: String,
    val displayName: String,
    val teamKeys: Set<String>,
)

data class Card(
    val issueIid: Long,
    val title: String,
    val description: String,
    val listKey: String,
    val position: Int,
    val teamKey: String,
    val teamName: String,
    val assignees: List<Member>,
    val startDate: String?,
    val dueDate: String?,
    val labels: List<String>,
    val updatedAt: String,
    val synchronized: Boolean = issueIid > 0,
    val syncError: String? = null,
)

enum class SortField { Manual, Due, Start, Updated }
enum class SortDirection { Ascending, Descending }

data class BoardFilter(
    val query: String = "",
    val teamKeys: Set<String> = emptySet(),
    val memberIds: Set<Long> = emptySet(),
    val labels: Set<String> = emptySet(),
    val sortField: SortField = SortField.Due,
    val sortDirection: SortDirection = SortDirection.Ascending,
)

fun filterAndSortCards(cards: List<Card>, filter: BoardFilter): List<Card> {
    val terms = filter.query.normalizeForSearch().split(Regex("\\s+")).filter(String::isNotBlank)
    val matching = cards.filter { card ->
        val haystack = buildString {
            append(card.title).append(' ')
            append(card.description).append(' ')
            append(card.issueIid).append(' ')
            append(card.teamName).append(' ')
            append(card.assignees.joinToString(" ") { "${it.displayName} ${it.username}" }).append(' ')
            append(card.labels.joinToString(" "))
        }.normalizeForSearch()
        terms.all(haystack::contains) &&
            (filter.teamKeys.isEmpty() || card.teamKey in filter.teamKeys) &&
            (filter.memberIds.isEmpty() || card.assignees.any { it.gitLabUserId in filter.memberIds }) &&
            filter.labels.all(card.labels::contains)
    }
    val comparator = when (filter.sortField) {
        SortField.Manual -> compareBy<Card>({ it.position }, { it.issueIid })
        SortField.Due -> compareBy<Card>({ it.dueDate == null }, { it.dueDate ?: "" }, { it.issueIid })
        SortField.Start -> compareBy<Card>({ it.startDate == null }, { it.startDate ?: "" }, { it.issueIid })
        SortField.Updated -> compareBy<Card>({ it.updatedAt }, { it.issueIid })
    }
    return matching.sortedWith(if (filter.sortDirection == SortDirection.Ascending) comparator else comparator.reversed())
}

private fun String.normalizeForSearch(): String =
    lowercase().replace(Regex("[\\p{Z}\\s]+"), " ").trim()

fun visibleLaneCards(cards: List<Card>, list: BoardList, closedBatchSize: Int = 50, closedPage: Int = 1): List<Card> {
    val lane = cards.filter { it.listKey == list.key }
    return if (list.closed) lane.take(closedBatchSize * closedPage) else lane
}

data class OptimisticCardState(
    val cards: List<Card>,
    val selectedIssueIid: Long? = null,
)

fun OptimisticCardState.rekeyTemporaryCard(temporaryIid: Long, synchronized: Card): OptimisticCardState = copy(
    cards = cards.map { if (it.issueIid == temporaryIid) synchronized else it },
    selectedIssueIid = if (selectedIssueIid == temporaryIid) synchronized.issueIid else selectedIssueIid,
)
