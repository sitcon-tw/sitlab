package org.sitcon.sitlab

import kotlinx.cinterop.ExperimentalForeignApi
import kotlinx.cinterop.BetaInteropApi
import kotlinx.cinterop.addressOf
import kotlinx.cinterop.usePinned
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.launch
import kotlin.coroutines.resume
import kotlin.coroutines.suspendCoroutine
import org.sitcon.sitlab.platform.HapticEffect
import org.sitcon.sitlab.platform.Haptics
import org.sitcon.sitlab.platform.LocalNotification
import org.sitcon.sitlab.platform.LocalNotifications
import org.sitcon.sitlab.platform.PermissionStatus
import org.sitcon.sitlab.platform.ShareSheet
import org.sitcon.sitlab.platform.SystemAppearance
import org.sitcon.sitlab.platform.BackgroundInterval
import org.sitcon.sitlab.platform.BackgroundRefresh
import platform.BackgroundTasks.BGAppRefreshTask
import platform.BackgroundTasks.BGAppRefreshTaskRequest
import platform.BackgroundTasks.BGTaskScheduler
import platform.Foundation.NSData
import platform.Foundation.NSDate
import platform.Foundation.NSURL
import platform.Foundation.create
import platform.UIKit.UIAccessibilityIsReduceMotionEnabled
import platform.UIKit.UIActivityViewController
import platform.UIKit.UIApplication
import platform.UIKit.UIApplicationOpenSettingsURLString
import platform.UIKit.UIImpactFeedbackGenerator
import platform.UIKit.UIImpactFeedbackStyle
import platform.UIKit.UINotificationFeedbackGenerator
import platform.UIKit.UINotificationFeedbackType
import platform.UserNotifications.UNAuthorizationOptionAlert
import platform.UserNotifications.UNAuthorizationOptionBadge
import platform.UserNotifications.UNAuthorizationOptionSound
import platform.UserNotifications.UNAuthorizationStatusAuthorized
import platform.UserNotifications.UNAuthorizationStatusDenied
import platform.UserNotifications.UNAuthorizationStatusNotDetermined
import platform.UserNotifications.UNMutableNotificationContent
import platform.UserNotifications.UNNotificationRequest
import platform.UserNotifications.UNTimeIntervalNotificationTrigger
import platform.UserNotifications.UNUserNotificationCenter

class IosHaptics : Haptics {
    override fun perform(effect: HapticEffect) {
        when (effect) {
            HapticEffect.Warning -> UINotificationFeedbackGenerator().notificationOccurred(UINotificationFeedbackType.UINotificationFeedbackTypeWarning)
            HapticEffect.Error -> UINotificationFeedbackGenerator().notificationOccurred(UINotificationFeedbackType.UINotificationFeedbackTypeError)
            HapticEffect.Success, HapticEffect.Confirm -> UINotificationFeedbackGenerator().notificationOccurred(UINotificationFeedbackType.UINotificationFeedbackTypeSuccess)
            HapticEffect.PickUp -> UIImpactFeedbackGenerator(UIImpactFeedbackStyle.UIImpactFeedbackStyleMedium).impactOccurred()
            else -> UIImpactFeedbackGenerator(UIImpactFeedbackStyle.UIImpactFeedbackStyleLight).impactOccurred()
        }
    }
}

@OptIn(ExperimentalForeignApi::class)
class IosLocalNotifications : LocalNotifications {
    private val center = UNUserNotificationCenter.currentNotificationCenter()

    override suspend fun permissionStatus(): PermissionStatus = suspendCoroutine { continuation ->
        center.getNotificationSettingsWithCompletionHandler { settings ->
            continuation.resume(when (settings?.authorizationStatus) {
                UNAuthorizationStatusAuthorized -> PermissionStatus.Granted
                UNAuthorizationStatusDenied -> PermissionStatus.Denied
                UNAuthorizationStatusNotDetermined -> PermissionStatus.NotDetermined
                else -> PermissionStatus.Denied
            })
        }
    }

    override suspend fun requestPermission(): PermissionStatus = suspendCoroutine { continuation ->
        center.requestAuthorizationWithOptions(UNAuthorizationOptionAlert or UNAuthorizationOptionSound or UNAuthorizationOptionBadge) { granted, _ ->
            continuation.resume(if (granted) PermissionStatus.Granted else PermissionStatus.Denied)
        }
    }

    override suspend fun pendingIds(): Set<String> = suspendCoroutine { continuation ->
        center.getPendingNotificationRequestsWithCompletionHandler { requests ->
            continuation.resume(requests.orEmpty().filterIsInstance<UNNotificationRequest>().mapTo(mutableSetOf()) { it.identifier })
        }
    }

    override suspend fun schedule(notification: LocalNotification) {
        val content = UNMutableNotificationContent().apply {
            setTitle(notification.title)
            setBody(notification.body)
            setUserInfo(mapOf("deepLink" to notification.deepLink))
        }
        val delaySeconds = ((notification.triggerEpochMillis - kotlin.time.Clock.System.now().toEpochMilliseconds()) / 1000.0).coerceAtLeast(1.0)
        val trigger = UNTimeIntervalNotificationTrigger.triggerWithTimeInterval(delaySeconds, repeats = false)
        val request = UNNotificationRequest.requestWithIdentifier(notification.id, content, trigger)
        suspendCoroutine { continuation -> center.addNotificationRequest(request) { continuation.resume(Unit) } }
    }

    override suspend fun cancel(ids: Set<String>) { center.removePendingNotificationRequestsWithIdentifiers(ids.toList()) }
    override suspend fun openSystemSettings() {
        NSURL.URLWithString(UIApplicationOpenSettingsURLString)?.let { UIApplication.sharedApplication.openURL(it, emptyMap<Any?, Any>(), null) }
    }
}

@OptIn(ExperimentalForeignApi::class, BetaInteropApi::class)
class IosShareSheet : ShareSheet {
    override suspend fun shareText(text: String) = present(listOf(text))
    override suspend fun sharePng(bytes: ByteArray, filename: String) {
        val data = bytes.usePinned { NSData.create(bytes = it.addressOf(0), length = bytes.size.toULong()) }
        present(listOf(data))
    }
    private fun present(items: List<Any>) {
        val controller = UIActivityViewController(items, applicationActivities = null)
        UIApplication.sharedApplication.keyWindow?.rootViewController?.presentViewController(controller, animated = true, completion = null)
    }
}

@OptIn(ExperimentalForeignApi::class)
class IosBackgroundRefresh(
    private val scope: CoroutineScope,
    private val performRefresh: suspend () -> Boolean,
) : BackgroundRefresh {
    init {
        BGTaskScheduler.sharedScheduler.registerForTaskWithIdentifier(Identifier, usingQueue = null) { task ->
            val refreshTask = task as? BGAppRefreshTask ?: return@registerForTaskWithIdentifier
            scope.launch {
                val successful = runCatching { performRefresh() }.getOrDefault(false)
                refreshTask.setTaskCompletedWithSuccess(successful)
                scheduleStoredInterval()
            }
        }
    }

    override suspend fun replace(interval: BackgroundInterval) {
        val hours = interval.hours ?: return cancel()
        platform.Foundation.NSUserDefaults.standardUserDefaults.setInteger(hours.toLong(), BackgroundHoursKey)
        submit(hours)
    }

    override suspend fun cancel() {
        platform.Foundation.NSUserDefaults.standardUserDefaults.removeObjectForKey(BackgroundHoursKey)
        BGTaskScheduler.sharedScheduler.cancelTaskRequestWithIdentifier(Identifier)
    }

    private fun scheduleStoredInterval() {
        val hours = platform.Foundation.NSUserDefaults.standardUserDefaults.integerForKey(BackgroundHoursKey).toInt()
        if (hours > 0) submit(hours)
    }

    private fun submit(hours: Int) {
        BGTaskScheduler.sharedScheduler.cancelTaskRequestWithIdentifier(Identifier)
        val request = BGAppRefreshTaskRequest(Identifier).apply {
            earliestBeginDate = NSDate(NSDate().timeIntervalSinceReferenceDate + hours * 3_600.0)
        }
        BGTaskScheduler.sharedScheduler.submitTaskRequest(request, error = null)
    }

    private companion object {
        const val Identifier = "org.sitcon.sitlab.refresh"
        const val BackgroundHoursKey = "background_refresh_hours"
    }
}

class IosSystemAppearance : SystemAppearance {
    override val reducedMotion: Flow<Boolean> = MutableStateFlow(UIAccessibilityIsReduceMotionEnabled())
    override val supportsDynamicColor: Boolean = false
}
