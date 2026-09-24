package org.sitcon.sitlab

import androidx.compose.runtime.getValue
import androidx.compose.runtime.collectAsState
import androidx.compose.ui.window.ComposeUIViewController
import kotlinx.coroutines.MainScope
import kotlinx.coroutines.launch
import org.sitcon.sitlab.app.SitLabStore
import org.sitcon.sitlab.network.SitLabApi
import org.sitcon.sitlab.network.platformHttpClient
import org.sitcon.sitlab.persistence.IosDatabaseFactory
import org.sitcon.sitlab.sync.SyncEngine
import org.sitcon.sitlab.ui.SitLabApp
import platform.Foundation.NSUUID

private object IosAppRuntime {
    val scope = MainScope()
    val session = IosSecureSessionStore()
    val database = IosDatabaseFactory().create()
    val api = SitLabApi(platformHttpClient(), sessionCookie = session::readCookie, sessionCookieUpdated = session::writeCookie)
    val preferences = IosPreferencesStore()
    val notifications = IosLocalNotifications()
    val reminders = IosReminderCoordinator(database, preferences, notifications)
    val sync = SyncEngine(api, database) { reminders.reconcile() }
    val oauth = IosOAuthCoordinator()
    val background = IosBackgroundRefresh(scope) {
        if (session.readCookie() == null) false else runCatching { sync.refresh() }.isSuccess
    }
    val store = SitLabStore(
        scope = scope,
        api = api,
        database = database,
        sessionStore = session,
        syncEngine = sync,
        newOperationId = { NSUUID().UUIDString },
        startPlatformLogin = oauth::start,
        preferencesStore = preferences,
        haptics = IosHaptics(),
        backgroundRefresh = background,
        notifications = notifications,
        shareSheet = IosShareSheet(),
        systemAppearance = IosSystemAppearance(),
        remindersChanged = reminders::reconcile,
        sessionCleared = reminders::clear,
    )
}

fun StartIosRuntime() {
    IosAppRuntime.store
}

fun ApplicationDidBecomeActive() {
    IosAppRuntime.store.onForeground()
}

fun RecordNotificationDelivery(notificationId: String) {
    IosAppRuntime.scope.launch {
        val dao = IosAppRuntime.database.dao()
        dao.notificationLedger(notificationId)?.let { entry ->
            dao.upsertNotificationLedger(
                listOf(entry.copy(deliveredAtEpochMillis = kotlin.time.Clock.System.now().toEpochMilliseconds())),
            )
        }
    }
}

fun HandleDeepLink(url: String) {
    val callback = IosAppRuntime.oauth.complete(url) ?: run {
        IosAppRuntime.oauth.callbackError(url)?.let { IosAppRuntime.store.reportError("GitLab sign-in failed: $it") }
        IosAppRuntime.oauth.cardIid(url)?.let(IosAppRuntime.store::openCard)
        return
    }
    IosAppRuntime.scope.launch {
        runCatching { IosAppRuntime.api.exchange(callback.code, callback.state, callback.verifier) }
            .onSuccess { IosAppRuntime.session.writeCookie(it); IosAppRuntime.store.onSessionEstablished() }
            .onFailure { IosAppRuntime.store.reportError(it.message ?: "GitLab sign-in failed") }
    }
}

fun MainViewController() = ComposeUIViewController {
    val state by IosAppRuntime.store.state.collectAsState()
    SitLabApp(state, IosAppRuntime.store)
}
