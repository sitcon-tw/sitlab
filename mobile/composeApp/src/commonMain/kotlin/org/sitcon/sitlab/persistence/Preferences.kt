package org.sitcon.sitlab.persistence

import kotlinx.serialization.Serializable

enum class Appearance { System, Light, Dark }
enum class ColorStyle { Sitcon, Dynamic }
enum class ReminderLead(val days: Int?) { Off(null), DueDay(0), OneDay(1), ThreeDays(3), SevenDays(7) }

@Serializable
data class MobilePreferences(
    val appearance: Appearance = Appearance.System,
    val colorStyle: ColorStyle = ColorStyle.Sitcon,
    val query: String = "",
    val teamKeys: Set<String> = emptySet(),
    val memberIds: Set<Long> = emptySet(),
    val labels: Set<String> = emptySet(),
    val backgroundHours: Int? = 1,
    val reminderLeadDays: Int? = 1,
    val onboardingComplete: Boolean = false,
)

interface PreferencesStore {
    suspend fun read(): MobilePreferences
    suspend fun update(transform: (MobilePreferences) -> MobilePreferences)
}
