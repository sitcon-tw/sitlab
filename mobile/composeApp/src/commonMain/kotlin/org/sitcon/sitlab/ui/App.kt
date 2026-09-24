package org.sitcon.sitlab.ui

import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.pager.rememberPagerState
import androidx.compose.foundation.pager.HorizontalPager
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.FilterList
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Search
import androidx.compose.material3.Badge
import androidx.compose.material3.BadgedBox
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CenterAlignedTopAppBar
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilledTonalIconButton
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Scaffold
import androidx.compose.material3.PrimaryScrollableTabRow
import androidx.compose.material3.Tab
import androidx.compose.material3.Text
import androidx.compose.material3.TextField
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.CustomAccessibilityAction
import androidx.compose.ui.semantics.customActions
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.launch
import kotlinx.serialization.Serializable
import org.sitcon.sitlab.domain.BoardFilter
import org.sitcon.sitlab.domain.BoardList
import org.sitcon.sitlab.domain.Card
import org.sitcon.sitlab.domain.filterAndSortCards
import org.sitcon.sitlab.domain.visibleLaneCards
import org.sitcon.sitlab.persistence.Appearance
import org.sitcon.sitlab.persistence.ColorStyle
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
    val lists: List<BoardList> = emptyList(),
    val cards: List<Card> = emptyList(),
    val filter: BoardFilter = BoardFilter(),
    val appearance: Appearance = Appearance.System,
    val colorStyle: ColorStyle = ColorStyle.Sitcon,
    val reducedMotion: Boolean = false,
    val syncing: Boolean = false,
    val lastSync: String? = null,
    val authenticated: Boolean = false,
    val error: String? = null,
)

interface AppActions {
    fun startLogin()
    fun updateQuery(value: String)
    fun openCard(issueIid: Long)
    fun moveCard(issueIid: Long)
    fun refresh()
}

@Composable
fun SitLabApp(state: AppUiState, actions: AppActions) {
    SitLabTheme(state.appearance, state.colorStyle, state.reducedMotion) {
        BoardScreen(state, actions)
    }
}

@OptIn(ExperimentalFoundationApi::class, ExperimentalMaterial3Api::class)
@Composable
private fun BoardScreen(state: AppUiState, actions: AppActions) {
    var filtersOpen by remember { mutableStateOf(false) }
    val filtered = remember(state.cards, state.filter) { filterAndSortCards(state.cards, state.filter) }
    val pagerState = rememberPagerState(pageCount = { state.lists.size })
    val scope = rememberCoroutineScope()

    Scaffold(
        topBar = {
            CenterAlignedTopAppBar(
                title = { Text("SITCON / 2027", fontWeight = FontWeight.Bold) },
                actions = {
                    IconButton(onClick = actions::refresh) { Icon(Icons.Default.Refresh, "Refresh") }
                },
            )
        },
    ) { padding ->
        if (!state.authenticated) {
            Column(
                Modifier.fillMaxSize().padding(padding).padding(24.dp),
                verticalArrangement = Arrangement.Center,
            ) {
                Text("SITCON Lab", style = MaterialTheme.typography.headlineLarge)
                Text("Sign in with GitLab to synchronize the board.", modifier = Modifier.padding(vertical = 16.dp))
                Button(onClick = actions::startLogin) { Text("Continue with GitLab") }
                state.error?.let { Text(it, color = MaterialTheme.colorScheme.error, modifier = Modifier.padding(top = 12.dp)) }
            }
            return@Scaffold
        }
        Column(Modifier.fillMaxSize().padding(padding)) {
            state.error?.let { Text(it, color = MaterialTheme.colorScheme.error, modifier = Modifier.padding(horizontal = 12.dp)) }
            TextField(
                value = state.filter.query,
                onValueChange = actions::updateQuery,
                modifier = Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 8.dp),
                leadingIcon = { Icon(Icons.Default.Search, null) },
                trailingIcon = {
                    FilledTonalIconButton(onClick = { filtersOpen = true }) {
                        Icon(Icons.Default.FilterList, "Advanced filters")
                    }
                },
                placeholder = { Text("Search title, IID, team, people, labels") },
                singleLine = true,
            )
            if (state.lists.isEmpty()) {
                Box(Modifier.fillMaxSize().padding(24.dp)) { Text("No cached board yet. Pull to refresh when online.") }
                return@Column
            }
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
                val cards = visibleLaneCards(filtered, list)
                LazyColumn(
                    modifier = Modifier.fillMaxSize().padding(horizontal = 12.dp),
                    verticalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    item { Text(list.name, style = MaterialTheme.typography.titleLarge, modifier = Modifier.padding(top = 12.dp)) }
                    items(cards, key = { it.issueIid }) { card ->
                        BoardCard(card, { actions.openCard(card.issueIid) }, { actions.moveCard(card.issueIid) })
                    }
                    item { Box(Modifier.padding(bottom = 16.dp)) }
                }
            }
        }
    }
    if (filtersOpen) {
        ModalBottomSheet(onDismissRequest = { filtersOpen = false }) {
            Column(Modifier.fillMaxWidth().padding(24.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Text("Filters and sorting", style = MaterialTheme.typography.headlineSmall)
                Text("Team, grouped member, AND label filters, plus manual/due/start/updated sorting are applied immediately.")
                HorizontalDivider()
                Text("Current result: ${filtered.size} cards")
            }
        }
    }
}

@Composable
private fun BoardCard(card: Card, onOpen: () -> Unit, onMove: () -> Unit) {
    Card(
        onClick = onOpen,
        modifier = Modifier.fillMaxWidth().semantics {
            customActions = listOf(CustomAccessibilityAction("Move card") { onMove(); true })
        },
    ) {
        Column(Modifier.padding(14.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                Text("#${card.issueIid}", style = MaterialTheme.typography.labelMedium)
                Text(card.teamName, style = MaterialTheme.typography.labelMedium)
            }
            Text(card.title, style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.SemiBold)
            if (card.labels.isNotEmpty()) Text(card.labels.joinToString(" · "), style = MaterialTheme.typography.bodySmall)
            card.dueDate?.let { Text("Due $it", style = MaterialTheme.typography.bodySmall) }
            card.syncError?.let { Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall) }
        }
    }
}
