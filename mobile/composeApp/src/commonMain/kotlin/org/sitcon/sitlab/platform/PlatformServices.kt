package org.sitcon.sitlab.platform

import kotlinx.coroutines.flow.Flow

interface SecureSessionStore {
    suspend fun readCookie(): String?
    suspend fun writeCookie(cookie: String)
    suspend fun clear()
}

interface ExternalBrowser {
    suspend fun open(url: String)
}

enum class HapticEffect { Selection, PickUp, ReorderTarget, Confirm, Success, Warning, Error }

interface Haptics {
    fun perform(effect: HapticEffect)
}

data class LocalNotification(
    val id: String,
    val title: String,
    val body: String,
    val triggerEpochMillis: Long,
    val deepLink: String,
)

interface LocalNotifications {
    suspend fun permissionStatus(): PermissionStatus
    suspend fun requestPermission(): PermissionStatus
    suspend fun pendingIds(): Set<String>
    suspend fun schedule(notification: LocalNotification)
    suspend fun cancel(ids: Set<String>)
    suspend fun openSystemSettings()
}

enum class PermissionStatus { NotDetermined, Granted, Denied }

enum class BackgroundInterval(val hours: Int?) {
    Manual(null), OneHour(1), SixHours(6), TwelveHours(12), TwentyFourHours(24)
}

interface BackgroundRefresh {
    suspend fun replace(interval: BackgroundInterval)
    suspend fun cancel()
}

interface ShareSheet {
    suspend fun shareText(text: String)
    suspend fun sharePng(bytes: ByteArray, filename: String)
}

interface SystemAppearance {
    val reducedMotion: Flow<Boolean>
    val supportsDynamicColor: Boolean
}

interface DeepLinks {
    val incoming: Flow<String>
}

data class PlatformServices(
    val secureSessionStore: SecureSessionStore,
    val browser: ExternalBrowser,
    val haptics: Haptics,
    val notifications: LocalNotifications,
    val backgroundRefresh: BackgroundRefresh,
    val shareSheet: ShareSheet,
    val appearance: SystemAppearance,
    val deepLinks: DeepLinks,
)
