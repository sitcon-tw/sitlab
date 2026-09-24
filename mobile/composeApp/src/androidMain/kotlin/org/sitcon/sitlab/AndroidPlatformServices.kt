package org.sitcon.sitlab

import android.Manifest
import android.app.AlarmManager
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.provider.Settings
import androidx.browser.customtabs.CustomTabsIntent
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.FileProvider
import androidx.work.CoroutineWorker
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.PeriodicWorkRequestBuilder
import androidx.work.WorkManager
import androidx.work.WorkerParameters
import java.io.File
import java.util.concurrent.TimeUnit
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import org.sitcon.sitlab.network.SitLabApi
import org.sitcon.sitlab.network.platformHttpClient
import org.sitcon.sitlab.persistence.AndroidDatabaseFactory
import org.sitcon.sitlab.platform.BackgroundInterval
import org.sitcon.sitlab.platform.BackgroundRefresh
import org.sitcon.sitlab.platform.ExternalBrowser
import org.sitcon.sitlab.platform.LocalNotification
import org.sitcon.sitlab.platform.LocalNotifications
import org.sitcon.sitlab.platform.PermissionStatus
import org.sitcon.sitlab.platform.ShareSheet
import org.sitcon.sitlab.platform.SystemAppearance
import org.sitcon.sitlab.sync.SyncEngine

class AndroidExternalBrowser(private val context: Context) : ExternalBrowser {
    override suspend fun open(url: String) = CustomTabsIntent.Builder().build().launchUrl(context, Uri.parse(url))
}

class AndroidLocalNotifications(
    private val context: Context,
    private val permissionRequester: (suspend () -> Boolean)? = null,
) : LocalNotifications {
    private val alarms = context.getSystemService(AlarmManager::class.java)
    private val preferences = context.getSharedPreferences(PendingStore, Context.MODE_PRIVATE)

    init { createChannel(context) }

    override suspend fun permissionStatus(): PermissionStatus = when {
        Build.VERSION.SDK_INT < 33 -> PermissionStatus.Granted
        context.checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED -> PermissionStatus.Granted
        !preferences.getBoolean(PermissionRequested, false) -> PermissionStatus.NotDetermined
        else -> PermissionStatus.Denied
    }

    override suspend fun requestPermission(): PermissionStatus {
        if (Build.VERSION.SDK_INT < 33 || permissionStatus() == PermissionStatus.Granted) return PermissionStatus.Granted
        val granted = permissionRequester?.invoke() ?: false
        preferences.edit().putBoolean(PermissionRequested, true).apply()
        return if (granted) PermissionStatus.Granted else PermissionStatus.Denied
    }
    override suspend fun pendingIds(): Set<String> = preferences.getStringSet(PendingIds, emptySet()).orEmpty()

    override suspend fun schedule(notification: LocalNotification) {
        val intent = Intent(context, NotificationAlarmReceiver::class.java).apply {
            putExtra(Id, notification.id); putExtra(Title, notification.title); putExtra(Body, notification.body); putExtra(DeepLink, notification.deepLink)
        }
        alarms.setAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, notification.triggerEpochMillis, pendingIntent(notification.id, intent))
        preferences.edit().putStringSet(PendingIds, pendingIds() + notification.id).apply()
    }

    override suspend fun cancel(ids: Set<String>) {
        ids.forEach { alarms.cancel(pendingIntent(it, Intent(context, NotificationAlarmReceiver::class.java))) }
        preferences.edit().putStringSet(PendingIds, pendingIds() - ids).apply()
    }

    override suspend fun openSystemSettings() {
        context.startActivity(Intent(Settings.ACTION_APP_NOTIFICATION_SETTINGS).putExtra(Settings.EXTRA_APP_PACKAGE, context.packageName).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
    }

    private fun pendingIntent(id: String, intent: Intent) = PendingIntent.getBroadcast(
        context, id.hashCode(), intent, PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
    )

    companion object {
        const val Channel = "deadlines"
        const val PendingStore = "pending_notifications"
        const val PendingIds = "ids"
        const val PermissionRequested = "permission_requested"
        const val Id = "id"
        const val Title = "title"
        const val Body = "body"
        const val DeepLink = "deep_link"

        fun createChannel(context: Context) {
            context.getSystemService(NotificationManager::class.java).createNotificationChannel(
                NotificationChannel(Channel, "Card deadlines", NotificationManager.IMPORTANCE_DEFAULT),
            )
        }
    }
}

class NotificationAlarmReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        val id = intent.getStringExtra(AndroidLocalNotifications.Id) ?: return
        AndroidLocalNotifications.createChannel(context)
        val destination = Intent(Intent.ACTION_VIEW, Uri.parse(intent.getStringExtra(AndroidLocalNotifications.DeepLink)), context, MainActivity::class.java)
        val contentIntent = PendingIntent.getActivity(context, id.hashCode(), destination, PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE)
        val notification = NotificationCompat.Builder(context, AndroidLocalNotifications.Channel)
            .setSmallIcon(android.R.drawable.ic_dialog_info)
            .setContentTitle(intent.getStringExtra(AndroidLocalNotifications.Title))
            .setContentText(intent.getStringExtra(AndroidLocalNotifications.Body))
            .setContentIntent(contentIntent).setAutoCancel(true).build()
        if (Build.VERSION.SDK_INT < 33 || context.checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED) {
            NotificationManagerCompat.from(context).notify(id.hashCode(), notification)
        }
        val preferences = context.getSharedPreferences(AndroidLocalNotifications.PendingStore, Context.MODE_PRIVATE)
        preferences.edit().putStringSet(AndroidLocalNotifications.PendingIds, preferences.getStringSet(AndroidLocalNotifications.PendingIds, emptySet()).orEmpty() - id).apply()
        val pendingResult = goAsync()
        CoroutineScope(Dispatchers.IO).launch {
            runCatching {
                val dao = AndroidDatabaseFactory(context).create().dao()
                dao.notificationLedger(id)?.let { dao.upsertNotificationLedger(listOf(it.copy(deliveredAtEpochMillis = kotlin.time.Clock.System.now().toEpochMilliseconds()))) }
            }
            pendingResult.finish()
        }
    }
}

class ReminderRescheduleReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        val pendingResult = goAsync()
        CoroutineScope(Dispatchers.IO).launch {
            runCatching {
                val database = AndroidDatabaseFactory(context).create()
                AndroidReminderCoordinator(
                    database,
                    AndroidPreferencesStore(context),
                    AndroidLocalNotifications(context),
                ).reconcile()
            }
            pendingResult.finish()
        }
    }
}

class AndroidBackgroundRefresh(private val context: Context) : BackgroundRefresh {
    override suspend fun replace(interval: BackgroundInterval) {
        val hours = interval.hours ?: return cancel()
        val request = PeriodicWorkRequestBuilder<SitLabSyncWorker>(hours.toLong(), TimeUnit.HOURS).build()
        WorkManager.getInstance(context).enqueueUniquePeriodicWork(WorkName, ExistingPeriodicWorkPolicy.UPDATE, request)
    }
    override suspend fun cancel() { WorkManager.getInstance(context).cancelUniqueWork(WorkName) }
    private companion object { const val WorkName = "sitlab-background-sync" }
}

class SitLabSyncWorker(context: Context, parameters: WorkerParameters) : CoroutineWorker(context, parameters) {
    override suspend fun doWork(): Result {
        val session = AndroidSecureSessionStore(applicationContext)
        if (session.readCookie() == null) return Result.success()
        val api = SitLabApi(platformHttpClient(), sessionCookie = session::readCookie, sessionCookieUpdated = session::writeCookie)
        val database = AndroidDatabaseFactory(applicationContext).create()
        val preferences = AndroidPreferencesStore(applicationContext)
        val notifications = AndroidLocalNotifications(applicationContext)
        val engine = SyncEngine(api, database) { AndroidReminderCoordinator(database, preferences, notifications).reconcile() }
        return runCatching { engine.refresh() }.fold({ Result.success() }, { Result.retry() })
    }
}

class AndroidShareSheet(private val context: Context) : ShareSheet {
    override suspend fun shareText(text: String) = share(Intent(Intent.ACTION_SEND).setType("text/plain").putExtra(Intent.EXTRA_TEXT, text))
    override suspend fun sharePng(bytes: ByteArray, filename: String) {
        val directory = File(context.cacheDir, "shared").apply { mkdirs() }
        val file = File(directory, filename).apply { writeBytes(bytes) }
        val uri = FileProvider.getUriForFile(context, "${context.packageName}.files", file)
        share(Intent(Intent.ACTION_SEND).setType("image/png").putExtra(Intent.EXTRA_STREAM, uri).addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION))
    }
    private fun share(intent: Intent) { context.startActivity(Intent.createChooser(intent, null).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)) }
}

class AndroidSystemAppearance(private val context: Context) : SystemAppearance {
    override val reducedMotion: Flow<Boolean> = MutableStateFlow(Settings.Global.getFloat(context.contentResolver, Settings.Global.ANIMATOR_DURATION_SCALE, 1f) == 0f)
    override val supportsDynamicColor: Boolean = Build.VERSION.SDK_INT >= 31
}
