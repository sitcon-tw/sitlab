package org.sitcon.sitlab.ui

import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.pager.HorizontalPager
import androidx.compose.foundation.pager.rememberPagerState
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.automirrored.filled.Label
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.CustomAccessibilityAction
import androidx.compose.ui.semantics.customActions
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.serialization.Serializable
import org.sitcon.sitlab.api.generated.CurrentUser
import org.sitcon.sitlab.api.generated.CardComment
import org.sitcon.sitlab.api.generated.LinkedWorkItem
import org.sitcon.sitlab.api.generated.ProjectLabel
import org.sitcon.sitlab.api.generated.WorkItemSummary
import org.sitcon.sitlab.domain.*
import org.sitcon.sitlab.persistence.Appearance
import org.sitcon.sitlab.persistence.ColorStyle
import org.sitcon.sitlab.platform.PermissionStatus
import org.sitcon.sitlab.ui.theme.SitLabTheme

@Serializable sealed interface Route {
    @Serializable data object Login : Route
    @Serializable data object Onboarding : Route
    @Serializable data object Board : Route
    @Serializable data object Gantt : Route
    @Serializable data class CardDetail(val issueIid: Long) : Route
    @Serializable data object Directory : Route
    @Serializable data object Labels : Route
    @Serializable data object Settings : Route
    @Serializable data class Relationships(val issueIid: Long) : Route
}

data class AppUiState(
    val route: Route = Route.Login,
    val lists: List<BoardList> = emptyList(),
    val cards: List<Card> = emptyList(),
    val teams: List<Team> = emptyList(),
    val members: List<Member> = emptyList(),
    val comments: Map<Long, List<CardComment>> = emptyMap(),
    val childItems: Map<Long, List<WorkItemSummary>> = emptyMap(),
    val linkedItems: Map<Long, List<LinkedWorkItem>> = emptyMap(),
    val projectLabels: List<ProjectLabel> = emptyList(),
    val currentUser: CurrentUser? = null,
    val filter: BoardFilter = BoardFilter(),
    val appearance: Appearance = Appearance.System,
    val colorStyle: ColorStyle = ColorStyle.Sitcon,
    val backgroundHours: Int? = 1,
    val reminderLeadDays: Int? = 1,
    val reducedMotion: Boolean = false,
    val syncing: Boolean = false,
    val realtimeConnected: Boolean = false,
    val lastSync: String? = null,
    val authenticated: Boolean = false,
    val onboardingComplete: Boolean = false,
    val notificationPermission: PermissionStatus = PermissionStatus.NotDetermined,
    val supportsDynamicColor: Boolean = false,
    val debugToolsEnabled: Boolean = false,
    val closedPage: Int = 1,
    val error: String? = null,
)

interface AppActions {
    fun startLogin() {}
    fun updateQuery(value: String) {}
    fun updateFilter(filter: BoardFilter) {}
    fun navigate(route: Route) {}
    fun openCard(issueIid: Long) {}
    fun moveCard(issueIid: Long, listKey: String? = null) {}
    fun saveCard(issueIid: Long, title: String, description: String) {}
    fun createCard(title: String, teamKey: String, listKey: String) {}
    fun deleteCard(issueIid: Long) {}
    fun refresh() {}
    fun logout() {}
    fun retryPending() {}
    fun loadCardActivity(issueIid: Long) {}
    fun addComment(issueIid: Long, body: String) {}
    fun loadRelationships(issueIid: Long) {}
    fun loadLabels() {}
    fun createLabel(name: String, color: String) {}
    fun setAppearance(value: Appearance) {}
    fun setColorStyle(value: ColorStyle) {}
    fun setBackgroundHours(value: Int?) {}
    fun setReminderLeadDays(value: Int?) {}
    fun completeOnboarding() {}
    fun requestNotificationPermission() {}
    fun openNotificationSettings() {}
    fun shareCard(issueIid: Long) {}
    fun updateCardTeam(issueIid: Long, teamKey: String) {}
    fun updateCardAssignees(issueIid: Long, memberIds: List<Long>) {}
    fun updateCardDates(issueIid: Long, startDate: String?, dueDate: String?) {}
    fun updateCardLabels(issueIid: Long, labels: List<String>) {}
    fun loadMoreClosedCards() {}
    fun loadDebugFixture() {}
}

@Composable
fun SitLabApp(state: AppUiState, actions: AppActions) {
    SitLabTheme(state.appearance, state.colorStyle, state.reducedMotion) {
        when {
            !state.authenticated || state.route == Route.Login -> LoginScreen(state, actions)
            state.route == Route.Onboarding -> OnboardingScreen(actions)
            state.route == Route.Board -> BoardScreen(state, actions)
            state.route == Route.Gantt -> GanttScreen(state, actions)
            state.route == Route.Directory -> DirectoryScreen(state, actions)
            state.route == Route.Labels -> LabelsScreen(state, actions)
            state.route == Route.Settings -> SettingsScreen(state, actions)
            state.route is Route.CardDetail -> CardDetailScreen(state, state.route.issueIid, actions)
            state.route is Route.Relationships -> RelationshipScreen(state, state.route.issueIid, actions)
            else -> BoardScreen(state, actions)
        }
    }
}

@Composable
private fun OnboardingScreen(actions: AppActions) {
    Column(
        Modifier.fillMaxSize().padding(24.dp),
        verticalArrangement = Arrangement.Center,
    ) {
        Text("Your board, available offline", style = MaterialTheme.typography.headlineMedium)
        Spacer(Modifier.height(12.dp))
        Text("SitLab keeps a local copy of your board, queues recoverable edits, and can remind you about synchronized deadlines.")
        Spacer(Modifier.height(24.dp))
        Button(onClick = actions::completeOnboarding) { Text("Open board") }
    }
}

@Composable
private fun LoginScreen(state: AppUiState, actions: AppActions) {
    Box(Modifier.fillMaxSize().padding(24.dp)) {
        Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(16.dp)) {
            Spacer(Modifier.height(72.dp))
            Text("SITCON Lab", style = MaterialTheme.typography.displaySmall, fontWeight = FontWeight.Bold)
            Text("Plan and synchronize SITCON 2027 work from Android or iOS.")
            Button(onClick = actions::startLogin) { Text("Continue with GitLab") }
            if (state.debugToolsEnabled) {
                FilledTonalButton(onClick = actions::loadDebugFixture) {
                    Icon(Icons.Default.BugReport, null)
                    Spacer(Modifier.width(8.dp))
                    Text("Load debug fixture")
                }
                Text("Debug builds only · no server connection", style = MaterialTheme.typography.labelMedium)
            }
            state.error?.let { Text(it, color = MaterialTheme.colorScheme.error) }
        }
    }
}

@OptIn(ExperimentalFoundationApi::class, ExperimentalMaterial3Api::class)
@Composable
private fun BoardScreen(state: AppUiState, actions: AppActions) {
    var filtersOpen by remember { mutableStateOf(false) }
    var createOpen by remember { mutableStateOf(false) }
    var debouncedQuery by remember { mutableStateOf(state.filter.query) }
    LaunchedEffect(state.filter.query) { delay(300); debouncedQuery = state.filter.query }
    val effectiveFilter = state.filter.copy(query = debouncedQuery)
    val filtered = remember(state.cards, effectiveFilter) { filterAndSortCards(state.cards, effectiveFilter) }
    val pagerState = rememberPagerState(pageCount = { state.lists.size })
    val scope = rememberCoroutineScope()

    Scaffold(
        topBar = { AppTopBar("SITCON / 2027", state, actions) },
        bottomBar = { MainNavigation(Route.Board, actions) },
        floatingActionButton = { FloatingActionButton(onClick = { createOpen = true }) { Icon(Icons.Default.Add, "Create card") } },
    ) { padding ->
        Column(Modifier.fillMaxSize().padding(padding)) {
            state.error?.let { Text(it, color = MaterialTheme.colorScheme.error, modifier = Modifier.padding(horizontal = 12.dp)) }
            TextField(
                value = state.filter.query,
                onValueChange = actions::updateQuery,
                modifier = Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 8.dp),
                leadingIcon = { Icon(Icons.Default.Search, null) },
                trailingIcon = { FilledTonalIconButton(onClick = { filtersOpen = true }) { Icon(Icons.Default.FilterList, "Advanced filters") } },
                placeholder = { Text("Title, IID, team, people, labels") },
                singleLine = true,
            )
            if (state.lists.isEmpty()) {
                Box(Modifier.fillMaxSize().padding(24.dp)) { Text("No cached board yet. Refresh when online.") }
                return@Column
            }
            BoxWithConstraints(Modifier.fillMaxSize()) {
                if (maxWidth >= 840.dp) {
                    LazyRow(Modifier.fillMaxSize().padding(horizontal = 12.dp), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                        items(state.lists, key = BoardList::key) { list ->
                            val cards = visibleLaneCards(filtered, list, closedPage = state.closedPage)
                            LazyColumn(Modifier.width(320.dp).fillParentMaxHeight(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                                item { Text("${list.name} · ${cards.size}", style = MaterialTheme.typography.titleLarge, modifier = Modifier.padding(top = 12.dp)) }
                                items(cards, key = { it.issueIid }) { card -> BoardCard(card, { actions.openCard(card.issueIid) }, { actions.moveCard(card.issueIid) }) }
                                if (list.closed && cards.size < filtered.count { it.listKey == list.key }) item { TextButton(onClick = actions::loadMoreClosedCards) { Text("Load 50 more") } }
                                item { Spacer(Modifier.height(72.dp)) }
                            }
                        }
                    }
                } else {
                    Column {
                        PrimaryScrollableTabRow(selectedTabIndex = pagerState.currentPage) {
                            state.lists.forEachIndexed { index, list ->
                                val count = filtered.count { it.listKey == list.key }
                                Tab(
                                    selected = index == pagerState.currentPage,
                                    onClick = { scope.launch { pagerState.animateScrollToPage(index) } },
                                    text = { BadgedBox(badge = { Badge { Text(count.toString()) } }) { Text(list.name, Modifier.padding(end = 8.dp)) } },
                                )
                            }
                        }
                        HorizontalPager(state = pagerState, modifier = Modifier.fillMaxSize()) { page ->
                            val list = state.lists[page]
                            val cards = visibleLaneCards(filtered, list, closedPage = state.closedPage)
                            LazyColumn(Modifier.fillMaxSize().padding(horizontal = 12.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                                item { Text(list.name, style = MaterialTheme.typography.titleLarge, modifier = Modifier.padding(top = 12.dp)) }
                                items(cards, key = { it.issueIid }) { card -> BoardCard(card, { actions.openCard(card.issueIid) }, { actions.moveCard(card.issueIid) }) }
                                if (list.closed && cards.size < filtered.count { it.listKey == list.key }) item { TextButton(onClick = actions::loadMoreClosedCards) { Text("Load 50 more") } }
                                item { Spacer(Modifier.height(72.dp)) }
                            }
                        }
                    }
                }
            }
        }
    }
    if (filtersOpen) FilterSheet(state, filtered.size, actions) { filtersOpen = false }
    if (createOpen) CreateCardDialog(state, actions) { createOpen = false }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun AppTopBar(title: String, state: AppUiState, actions: AppActions, back: Boolean = false) {
    CenterAlignedTopAppBar(
        title = { Column { Text(title, fontWeight = FontWeight.Bold); state.lastSync?.let { Text("Synced $it", style = MaterialTheme.typography.labelSmall) } } },
        navigationIcon = { if (back) IconButton(onClick = { actions.navigate(Route.Board) }) { Icon(Icons.AutoMirrored.Filled.ArrowBack, "Back") } },
        actions = { IconButton(onClick = actions::refresh) { Icon(Icons.Default.Refresh, if (state.syncing) "Synchronizing" else "Refresh") } },
    )
}

@Composable
private fun BoardCard(card: Card, onOpen: () -> Unit, onMove: () -> Unit) {
    Card(onClick = onOpen, modifier = Modifier.fillMaxWidth().semantics { customActions = listOf(CustomAccessibilityAction("Move card") { onMove(); true }) }) {
        Column(Modifier.padding(14.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                Text("#${card.issueIid}", style = MaterialTheme.typography.labelMedium)
                Text(card.teamName, style = MaterialTheme.typography.labelMedium)
            }
            Text(card.title, style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.SemiBold)
            if (card.labels.isNotEmpty()) Text(card.labels.joinToString(" · "), style = MaterialTheme.typography.bodySmall)
            card.dueDate?.let { Text("Due $it", style = MaterialTheme.typography.bodySmall) }
            if (!card.synchronized) Text("Waiting for synchronization", style = MaterialTheme.typography.labelSmall)
            card.syncError?.let { Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall) }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun FilterSheet(state: AppUiState, count: Int, actions: AppActions, dismiss: () -> Unit) {
    ModalBottomSheet(onDismissRequest = dismiss) {
        LazyColumn(Modifier.fillMaxWidth().padding(24.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
            item { Text("Filters and sorting", style = MaterialTheme.typography.headlineSmall) }
            item { Text("$count matching cards") }
            item { Text("Teams", fontWeight = FontWeight.Bold) }
            item {
                LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    items(state.teams.filter(Team::active)) { team ->
                        val selected = team.key in state.filter.teamKeys
                        FilterChip(selected, { actions.updateFilter(state.filter.copy(teamKeys = state.filter.teamKeys.toggle(team.key))) }, { Text(team.name) })
                    }
                }
            }
            item { Text("People", fontWeight = FontWeight.Bold) }
            item {
                LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    items(state.members) { member ->
                        val selected = member.gitLabUserId in state.filter.memberIds
                        FilterChip(selected, { actions.updateFilter(state.filter.copy(memberIds = state.filter.memberIds.toggle(member.gitLabUserId))) }, { Text(member.displayName) })
                    }
                }
            }
            item { Text("Labels (match all)", fontWeight = FontWeight.Bold) }
            item {
                LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    items(state.cards.flatMap(Card::labels).distinct().sorted()) { label ->
                        val selected = label in state.filter.labels
                        FilterChip(selected, { actions.updateFilter(state.filter.copy(labels = state.filter.labels.toggle(label))) }, { Text(label) })
                    }
                }
            }
            item { Text("Sort", fontWeight = FontWeight.Bold) }
            item {
                LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    items(SortField.entries) { field -> FilterChip(state.filter.sortField == field, { actions.updateFilter(state.filter.copy(sortField = field)) }, { Text(field.name) }) }
                }
            }
            item {
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    FilterChip(state.filter.sortDirection == SortDirection.Ascending, { actions.updateFilter(state.filter.copy(sortDirection = SortDirection.Ascending)) }, { Text("Ascending") })
                    FilterChip(state.filter.sortDirection == SortDirection.Descending, { actions.updateFilter(state.filter.copy(sortDirection = SortDirection.Descending)) }, { Text("Descending") })
                }
            }
            item { Button(onClick = dismiss) { Text("Done") } }
        }
    }
}

@Composable
private fun CreateCardDialog(state: AppUiState, actions: AppActions, dismiss: () -> Unit) {
    var title by remember { mutableStateOf("") }
    var teamIndex by remember { mutableIntStateOf(0) }
    val teams = state.teams.filter(Team::active)
    AlertDialog(
        onDismissRequest = dismiss,
        title = { Text("Create card") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                TextField(title, { title = it }, label = { Text("Title") })
                if (teams.isNotEmpty()) {
                    Text("Team")
                    LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        items(teams.size) { index -> FilterChip(index == teamIndex, { teamIndex = index }, { Text(teams[index].name) }) }
                    }
                }
            }
        },
        confirmButton = {
            Button(enabled = title.isNotBlank() && teams.isNotEmpty() && state.lists.any { !it.closed }, onClick = {
                actions.createCard(title, teams[teamIndex].key, state.lists.first { !it.closed }.key); dismiss()
            }) { Text("Create") }
        },
        dismissButton = { TextButton(onClick = dismiss) { Text("Cancel") } },
    )
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun CardDetailScreen(state: AppUiState, issueIid: Long, actions: AppActions) {
    val card = state.cards.firstOrNull { it.issueIid == issueIid }
    var title by remember(card?.issueIid, card?.title) { mutableStateOf(card?.title.orEmpty()) }
    var description by remember(card?.issueIid, card?.description) { mutableStateOf(card?.description.orEmpty()) }
    var deleteOpen by remember { mutableStateOf(false) }
    var comment by remember { mutableStateOf("") }
    var startDate by remember(card?.issueIid, card?.startDate) { mutableStateOf(card?.startDate.orEmpty()) }
    var dueDate by remember(card?.issueIid, card?.dueDate) { mutableStateOf(card?.dueDate.orEmpty()) }
    LaunchedEffect(issueIid) { actions.loadCardActivity(issueIid); actions.loadLabels() }
    Scaffold(topBar = { AppTopBar(card?.let { "#${it.issueIid}" } ?: "Card", state, actions, back = true) }) { padding ->
        if (card == null) Box(Modifier.fillMaxSize().padding(padding).padding(24.dp)) { Text("Card is not available in the local cache.") }
        else LazyColumn(Modifier.fillMaxSize().padding(padding).padding(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
            item { TextField(title, { title = it }, Modifier.fillMaxWidth(), label = { Text("Title") }) }
            item { TextField(description, { description = it }, Modifier.fillMaxWidth().height(180.dp), label = { Text("Description (Markdown)") }) }
            item {
                Column {
                    Text("Team", fontWeight = FontWeight.Bold)
                    LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        items(state.teams.filter(Team::active), key = Team::key) { team ->
                            FilterChip(team.key == card.teamKey, { actions.updateCardTeam(card.issueIid, team.key) }, { Text(team.name) })
                        }
                    }
                }
            }
            item { Text("Status: ${card.gitLabStatusName ?: card.listKey}") }
            item {
                Column {
                    Text("Assignees", fontWeight = FontWeight.Bold)
                    LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        items(state.members, key = Member::gitLabUserId) { member ->
                            val selected = card.assignees.any { it.gitLabUserId == member.gitLabUserId }
                            FilterChip(selected, {
                                val ids = card.assignees.map(Member::gitLabUserId).toSet().toggle(member.gitLabUserId).toList()
                                actions.updateCardAssignees(card.issueIid, ids)
                            }, { Text(member.displayName) })
                        }
                    }
                }
            }
            item {
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    TextField(startDate, { startDate = it }, Modifier.weight(1f), label = { Text("Start YYYY-MM-DD") })
                    TextField(dueDate, { dueDate = it }, Modifier.weight(1f), label = { Text("Due YYYY-MM-DD") })
                }
                FilledTonalButton(onClick = { actions.updateCardDates(card.issueIid, startDate.ifBlank { null }, dueDate.ifBlank { null }) }) { Text("Update dates") }
            }
            item {
                Column {
                    Text("Labels", fontWeight = FontWeight.Bold)
                    LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        items(state.projectLabels, key = ProjectLabel::id) { label ->
                            FilterChip(label.name in card.labels, { actions.updateCardLabels(card.issueIid, card.labels.toSet().toggle(label.name).toList()) }, { Text(label.name) })
                        }
                    }
                }
            }
            item {
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Button(enabled = title.isNotBlank() && (title != card.title || description != card.description), onClick = { actions.saveCard(card.issueIid, title, description) }) { Text("Save") }
                    FilledTonalButton(onClick = { title = card.title; description = card.description }) { Text("Revert") }
                }
            }
            item { FilledTonalButton(onClick = { actions.navigate(Route.Relationships(card.issueIid)) }) { Text("Children and linked items") } }
            item { FilledTonalButton(onClick = { actions.shareCard(card.issueIid) }) { Icon(Icons.Default.Share, null); Spacer(Modifier.width(8.dp)); Text("Share card") } }
            item { HorizontalDivider(); Text("Comments", style = MaterialTheme.typography.titleLarge) }
            items(state.comments[card.issueIid].orEmpty(), key = { it.id }) { note ->
                Card(Modifier.fillMaxWidth()) { Column(Modifier.padding(12.dp)) { Text(note.author.displayName, fontWeight = FontWeight.Bold); Text(note.body) } }
            }
            item {
                Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    TextField(comment, { comment = it }, Modifier.fillMaxWidth(), label = { Text("Comment or GitLab Quick Action") })
                    Button(enabled = comment.isNotBlank(), onClick = { actions.addComment(card.issueIid, comment); comment = "" }) { Text("Post") }
                }
            }
            item { TextButton(onClick = { deleteOpen = true }) { Text("Delete permanently", color = MaterialTheme.colorScheme.error) } }
        }
    }
    if (deleteOpen && card != null) AlertDialog(
        onDismissRequest = { deleteOpen = false }, title = { Text("Delete #${card.issueIid}?") },
        text = { Text("This permanently deletes the GitLab issue and cannot be undone.") },
        confirmButton = { Button(onClick = { actions.deleteCard(card.issueIid); deleteOpen = false }) { Text("Delete") } },
        dismissButton = { TextButton(onClick = { deleteOpen = false }) { Text("Cancel") } },
    )
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun GanttScreen(state: AppUiState, actions: AppActions) {
    var scale by remember { mutableStateOf(GanttScale.Day) }
    val groups = remember(state.cards, state.lists) { ganttGroups(state.cards, state.lists) }
    Scaffold(topBar = { AppTopBar("Gantt", state, actions) }, bottomBar = { MainNavigation(Route.Gantt, actions) }) { padding ->
        LazyColumn(Modifier.fillMaxSize().padding(padding).padding(12.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            item { Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) { GanttScale.entries.forEach { FilterChip(scale == it, { scale = it }, { Text(it.name) }) } } }
            groups.forEach { group ->
                item { Text(group.teamName, style = MaterialTheme.typography.titleLarge) }
                items(group.spans) { span -> Card(onClick = { actions.openCard(span.card.issueIid) }, modifier = Modifier.fillMaxWidth()) { Text("#${span.card.issueIid} ${span.card.title}", Modifier.padding(14.dp)) } }
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun DirectoryScreen(state: AppUiState, actions: AppActions) {
    Scaffold(topBar = { AppTopBar("Directory", state, actions) }, bottomBar = { MainNavigation(Route.Directory, actions) }) { padding ->
        LazyColumn(Modifier.fillMaxSize().padding(padding).padding(12.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            state.teams.filter(Team::active).forEach { team ->
                item { Text(team.name, style = MaterialTheme.typography.titleLarge) }
                items(state.members.filter { team.key in it.teamKeys }) { member -> Card(Modifier.fillMaxWidth()) { Column(Modifier.padding(12.dp)) { Text(member.displayName); Text("@${member.username}") } } }
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun LabelsScreen(state: AppUiState, actions: AppActions) {
    var createOpen by remember { mutableStateOf(false) }
    LaunchedEffect(Unit) { actions.loadLabels() }
    Scaffold(topBar = { AppTopBar("Labels", state, actions, back = true) }) { padding ->
        LazyColumn(Modifier.fillMaxSize().padding(padding).padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            item { Button(onClick = { createOpen = true }) { Text("Create label") } }
            items(state.projectLabels, key = { it.id }) { label -> Card(Modifier.fillMaxWidth()) { Column(Modifier.padding(14.dp)) { Text(label.name, fontWeight = FontWeight.Bold); label.description?.let { Text(it) } } } }
        }
    }
    if (createOpen) {
        var name by remember { mutableStateOf("") }
        var color by remember { mutableStateOf("#77B55A") }
        val reserved = isReservedLabel(name, state.teams)
        AlertDialog(onDismissRequest = { createOpen = false }, title = { Text("Create label") }, text = { Column { TextField(name, { name = it }, label = { Text("Name") }, isError = name.isNotBlank() && reserved); if (name.isNotBlank() && reserved) Text("This label is managed by the board configuration.", color = MaterialTheme.colorScheme.error); TextField(color, { color = it }, label = { Text("Color") }) } }, confirmButton = { Button(enabled = !reserved, onClick = { actions.createLabel(name, color); createOpen = false }) { Text("Create") } }, dismissButton = { TextButton(onClick = { createOpen = false }) { Text("Cancel") } })
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun SettingsScreen(state: AppUiState, actions: AppActions) {
    Scaffold(topBar = { AppTopBar("Settings", state, actions) }, bottomBar = { MainNavigation(Route.Settings, actions) }) { padding ->
        LazyColumn(Modifier.fillMaxSize().padding(padding).padding(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
            item { Text(state.currentUser?.displayName ?: "Signed in", style = MaterialTheme.typography.headlineSmall) }
            item { Text("Appearance: ${state.appearance.name} · Color: ${state.colorStyle.name}") }
            item { LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) { items(Appearance.entries) { value -> FilterChip(state.appearance == value, { actions.setAppearance(value) }, { Text(value.name) }) } } }
            item { LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) { items(ColorStyle.entries.filter { it != ColorStyle.Dynamic || state.supportsDynamicColor }) { value -> FilterChip(state.colorStyle == value, { actions.setColorStyle(value) }, { Text(value.name) }) } } }
            item { Text("Background refresh", fontWeight = FontWeight.Bold) }
            item { LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) { items(listOf(null, 1, 6, 12, 24)) { hours -> FilterChip(state.backgroundHours == hours, { actions.setBackgroundHours(hours) }, { Text(hours?.let { "${it}h" } ?: "Manual") }) } } }
            item { Text("Deadline reminder", fontWeight = FontWeight.Bold) }
            item { LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) { items(listOf(null, 0, 1, 3, 7)) { days -> FilterChip(state.reminderLeadDays == days, { actions.setReminderLeadDays(days) }, { Text(days?.let { if (it == 0) "Due day" else "$it days" } ?: "Off") }) } } }
            item { Text("Last successful sync: ${state.lastSync ?: "Not yet synchronized"}") }
            item {
                Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    Text("Notification permission: ${state.notificationPermission.name}")
                    if (state.notificationPermission == PermissionStatus.NotDetermined) {
                        FilledTonalButton(onClick = actions::requestNotificationPermission) { Text("Allow notifications") }
                    } else if (state.notificationPermission == PermissionStatus.Denied) {
                        FilledTonalButton(onClick = actions::openNotificationSettings) { Text("Open notification settings") }
                    }
                }
            }
            if (state.cards.any { it.syncError != null }) item { FilledTonalButton(onClick = actions::retryPending) { Text("Retry failed changes") } }
            item { Text("Background refresh and local deadline reminders are best-effort and can reflect stale server data.") }
            item { FilledTonalButton(onClick = { actions.navigate(Route.Labels) }) { Icon(Icons.AutoMirrored.Filled.Label, null); Spacer(Modifier.width(8.dp)); Text("Manage labels") } }
            item { Button(onClick = actions::logout) { Text("Sign out") } }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun RelationshipScreen(state: AppUiState, issueIid: Long, actions: AppActions) {
    LaunchedEffect(issueIid) { actions.loadRelationships(issueIid) }
    Scaffold(topBar = { AppTopBar("Relationships · #$issueIid", state, actions, back = true) }) { padding ->
        Column(Modifier.fillMaxSize().padding(padding).padding(24.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
            Text("Child items and linked work items", style = MaterialTheme.typography.headlineSmall)
            Text("Children", fontWeight = FontWeight.Bold)
            state.childItems[issueIid].orEmpty().forEach { Text("#${it.iid} ${it.title}") }
            HorizontalDivider()
            Text("Linked items", fontWeight = FontWeight.Bold)
            state.linkedItems[issueIid].orEmpty().forEach { Text("#${it.iid} ${it.title} · ${it.linkType}") }
        }
    }
}

@Composable
private fun MainNavigation(selected: Route, actions: AppActions) {
    NavigationBar {
        NavigationBarItem(selected == Route.Board, { actions.navigate(Route.Board) }, { Icon(Icons.Default.ViewKanban, null) }, label = { Text("Board") })
        NavigationBarItem(selected == Route.Gantt, { actions.navigate(Route.Gantt) }, { Icon(Icons.Default.CalendarMonth, null) }, label = { Text("Gantt") })
        NavigationBarItem(selected == Route.Directory, { actions.navigate(Route.Directory) }, { Icon(Icons.Default.Group, null) }, label = { Text("People") })
        NavigationBarItem(selected == Route.Settings, { actions.navigate(Route.Settings) }, { Icon(Icons.Default.Settings, null) }, label = { Text("Settings") })
    }
}

private fun <T> Set<T>.toggle(value: T): Set<T> = if (value in this) this - value else this + value
