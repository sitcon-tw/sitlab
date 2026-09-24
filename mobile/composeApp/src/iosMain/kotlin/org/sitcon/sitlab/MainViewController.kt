package org.sitcon.sitlab

import androidx.compose.ui.window.ComposeUIViewController
import org.sitcon.sitlab.ui.AppActions
import org.sitcon.sitlab.ui.AppUiState
import org.sitcon.sitlab.ui.SitLabApp

private object PreviewActions : AppActions {
    override fun startLogin() = Unit
    override fun updateQuery(value: String) = Unit
    override fun openCard(issueIid: Long) = Unit
    override fun moveCard(issueIid: Long) = Unit
    override fun refresh() = Unit
}

fun MainViewController() = ComposeUIViewController { SitLabApp(AppUiState(), PreviewActions) }
