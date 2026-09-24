package org.sitcon.sitlab

import android.content.Intent
import android.content.pm.ApplicationInfo
import android.os.Bundle
import android.os.Build
import android.view.HapticFeedbackConstants
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.runtime.getValue
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.lifecycleScope
import java.util.UUID
import kotlinx.coroutines.launch
import kotlinx.coroutines.CompletableDeferred
import org.sitcon.sitlab.app.SitLabStore
import org.sitcon.sitlab.network.SitLabApi
import org.sitcon.sitlab.network.platformHttpClient
import org.sitcon.sitlab.persistence.AndroidDatabaseFactory
import org.sitcon.sitlab.platform.HapticEffect
import org.sitcon.sitlab.platform.Haptics
import org.sitcon.sitlab.platform.BackgroundInterval
import org.sitcon.sitlab.sync.SyncEngine
import org.sitcon.sitlab.ui.SitLabApp

class MainActivity : ComponentActivity() {
    private lateinit var sessionStore: AndroidSecureSessionStore
    private lateinit var api: SitLabApi
    private lateinit var store: SitLabStore
    private var permissionRequest: CompletableDeferred<Boolean>? = null
    private val notificationPermissionLauncher = registerForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
        permissionRequest?.complete(granted)
        permissionRequest = null
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        sessionStore = AndroidSecureSessionStore(applicationContext)
        val database = AndroidDatabaseFactory(applicationContext).create()
        val preferences = AndroidPreferencesStore(applicationContext)
        val notifications = AndroidLocalNotifications(applicationContext, ::requestNotificationPermission)
        val reminders = AndroidReminderCoordinator(database, preferences, notifications)
        api = SitLabApi(platformHttpClient(), sessionCookie = sessionStore::readCookie, sessionCookieUpdated = sessionStore::writeCookie)
        val syncEngine = SyncEngine(api, database) { reminders.reconcile() }
        store = SitLabStore(
            scope = lifecycleScope,
            api = api,
            database = database,
            sessionStore = sessionStore,
            syncEngine = syncEngine,
            newOperationId = { UUID.randomUUID().toString() },
            startPlatformLogin = { MobileOAuthCoordinator(this).start() },
            debugToolsEnabled = applicationInfo.flags and ApplicationInfo.FLAG_DEBUGGABLE != 0,
            haptics = AndroidHaptics(this),
            preferencesStore = preferences,
            backgroundRefresh = AndroidBackgroundRefresh(applicationContext),
            notifications = notifications,
            shareSheet = AndroidShareSheet(applicationContext),
            systemAppearance = AndroidSystemAppearance(applicationContext),
            remindersChanged = reminders::reconcile,
            sessionCleared = reminders::clear,
        )
        lifecycleScope.launch {
            val hours = preferences.read().backgroundHours
            val scheduler = AndroidBackgroundRefresh(applicationContext)
            if (hours == null) scheduler.cancel() else scheduler.replace(BackgroundInterval.entries.first { it.hours == hours })
        }
        handleDeepLink(intent)
        setContent {
            val state by store.state.collectAsStateWithLifecycle()
            SitLabApp(state, store)
        }
    }

    private suspend fun requestNotificationPermission(): Boolean {
        if (Build.VERSION.SDK_INT < 33) return true
        val pending = permissionRequest ?: CompletableDeferred<Boolean>().also {
            permissionRequest = it
            notificationPermissionLauncher.launch(android.Manifest.permission.POST_NOTIFICATIONS)
        }
        return pending.await()
    }

    override fun onResume() {
        super.onResume()
        if (::store.isInitialized) store.onForeground()
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        handleDeepLink(intent)
    }

    private fun handleDeepLink(intent: Intent?) {
        val uri = intent?.data ?: return
        when {
            uri.path == MobileOAuthCoordinator.CallbackPath -> {
                val callback = MobileOAuthCoordinator(this).complete(uri)
                if (callback != null) lifecycleScope.launch {
                    runCatching { api.exchange(callback.code, callback.state, callback.verifier) }
                        .onSuccess { cookie -> sessionStore.writeCookie(cookie); store.onSessionEstablished() }
                        .onFailure { store.reportError(it.message ?: "GitLab sign-in failed") }
                }
                else uri.getQueryParameter("error")?.let { store.reportError("GitLab sign-in failed: $it") }
            }
            uri.scheme == "https" && uri.host == "sitlab.sitcon.org" && uri.path?.startsWith("/mobile/cards/") == true ->
                uri.lastPathSegment?.toLongOrNull()?.let(store::openCard)
        }
    }
}

private class AndroidHaptics(private val activity: ComponentActivity) : Haptics {
    override fun perform(effect: HapticEffect) {
        val feedback = when (effect) {
            HapticEffect.Selection, HapticEffect.ReorderTarget -> HapticFeedbackConstants.CLOCK_TICK
            HapticEffect.PickUp -> HapticFeedbackConstants.LONG_PRESS
            HapticEffect.Warning, HapticEffect.Error -> if (Build.VERSION.SDK_INT >= 30) HapticFeedbackConstants.REJECT else HapticFeedbackConstants.LONG_PRESS
            HapticEffect.Confirm, HapticEffect.Success -> if (Build.VERSION.SDK_INT >= 30) HapticFeedbackConstants.CONFIRM else HapticFeedbackConstants.KEYBOARD_TAP
        }
        activity.window.decorView.performHapticFeedback(feedback)
    }
}
