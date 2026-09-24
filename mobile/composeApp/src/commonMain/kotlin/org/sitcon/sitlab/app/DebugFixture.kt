package org.sitcon.sitlab.app

import org.sitcon.sitlab.api.generated.CardComment
import org.sitcon.sitlab.api.generated.CardCommentAuthor
import org.sitcon.sitlab.api.generated.CurrentUser
import org.sitcon.sitlab.api.generated.ProjectLabel
import org.sitcon.sitlab.domain.BoardList
import org.sitcon.sitlab.domain.Card
import org.sitcon.sitlab.domain.Member
import org.sitcon.sitlab.domain.Team
import org.sitcon.sitlab.ui.AppUiState
import org.sitcon.sitlab.ui.Route

internal fun debugFixture(debugToolsEnabled: Boolean): AppUiState {
    val alice = Member(101, "alice", "Alice Chen", setOf("dev"))
    val bob = Member(102, "bob", "Bob Lin", setOf("design", "dev"))
    val carol = Member(103, "carol", "Carol Wu", setOf("ops"))
    val teams = listOf(
        Team("dev", "Development", true, 1, listOf(101), "Team::Development"),
        Team("design", "Design", true, 2, listOf(102), "Team::Design"),
        Team("ops", "Operations", true, 3, listOf(103), "Team::Operations"),
    )
    val lists = listOf(
        BoardList("inbox", "Inbox", 1, false, "#6750A4"),
        BoardList("doing", "Doing", 2, false, "#006A6A"),
        BoardList("review", "Review", 3, false, "#8B5000"),
        BoardList("closed", "Closed", 4, true, "#5F5F5F"),
    )
    val cards = listOf(
        Card(101, "Finish mobile authentication", "Verify callback recovery and secure cookies.", "doing", 0, "dev", "Development", listOf(alice), "2026-09-22", "2026-09-26", listOf("mobile", "priority::high"), "2026-09-24T08:30:00Z", webUrl = "https://sitlab.sitcon.org/mobile/cards/101", gitLabStatusName = "In progress"),
        Card(102, "Review tablet board layout", "Check the expanded multi-lane presentation.", "review", 0, "design", "Design", listOf(bob), "2026-09-24", "2026-09-29", listOf("mobile", "ux"), "2026-09-24T07:15:00Z", webUrl = "https://sitlab.sitcon.org/mobile/cards/102", gitLabStatusName = "Review"),
        Card(103, "Prepare reminder runbook", "Document best-effort delivery behavior.", "inbox", 0, "ops", "Operations", listOf(carol), null, "2026-10-02", listOf("docs"), "2026-09-23T11:00:00Z", webUrl = "https://sitlab.sitcon.org/mobile/cards/103"),
        Card(104, "Resolve invalid schedule", "This deliberately exercises invalid Gantt ranges.", "doing", 1, "dev", "Development", listOf(alice, bob), "2026-10-05", "2026-10-01", listOf("needs-attention"), "2026-09-24T06:00:00Z", syncError = "Example validation failure"),
        Card(105, "Archived launch checklist", "Completed fixture card.", "closed", 0, "ops", "Operations", emptyList(), "2026-09-01", "2026-09-10", listOf("release"), "2026-09-10T09:00:00Z"),
    )
    val author = CardCommentAuthor(101, "alice", "Alice Chen", null, "https://gitlab.com/alice")
    return AppUiState(
        route = Route.Board,
        lists = lists,
        cards = cards,
        teams = teams,
        members = listOf(alice, bob, carol),
        comments = mapOf(101L to listOf(CardComment(1, "Debug fixture comment", author, false, "2026-09-24T08:00:00Z", "2026-09-24T08:00:00Z"))),
        projectLabels = listOf(
            ProjectLabel(1, "mobile", "#6750A4", "#FFFFFF", "Mobile application work"),
            ProjectLabel(2, "ux", "#006A6A", "#FFFFFF", null),
            ProjectLabel(3, "docs", "#8B5000", "#FFFFFF", null),
            ProjectLabel(4, "priority::high", "#BA1A1A", "#FFFFFF", null),
        ),
        currentUser = CurrentUser("debug-user", 101, "alice", "Alice Chen", null, "https://gitlab.com/alice", 50),
        authenticated = true,
        onboardingComplete = true,
        debugToolsEnabled = debugToolsEnabled,
        lastSync = "Debug fixture",
        realtimeConnected = true,
    )
}
