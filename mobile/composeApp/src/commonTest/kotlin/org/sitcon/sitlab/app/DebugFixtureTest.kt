package org.sitcon.sitlab.app

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue
import org.sitcon.sitlab.ui.Route
import org.sitcon.sitlab.ui.AppUiState

class DebugFixtureTest {
    @Test
    fun debugToolsAreDisabledByDefault() {
        assertFalse(AppUiState().debugToolsEnabled)
    }

    @Test
    fun fixtureExercisesRepresentativeAppStates() {
        val state = debugFixture(debugToolsEnabled = true)

        assertTrue(state.debugToolsEnabled)
        assertTrue(state.authenticated)
        assertEquals(Route.Board, state.route)
        assertTrue(state.lists.any { it.closed })
        assertTrue(state.cards.any { it.syncError != null })
        assertTrue(state.cards.any { it.startDate != null && it.dueDate != null && it.startDate > it.dueDate })
        assertTrue(state.comments.isNotEmpty())
        assertFalse(state.projectLabels.isEmpty())
    }
}
