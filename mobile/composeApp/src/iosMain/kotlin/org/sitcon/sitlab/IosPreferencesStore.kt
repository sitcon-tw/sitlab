package org.sitcon.sitlab

import androidx.datastore.preferences.core.PreferenceDataStoreFactory
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import kotlinx.cinterop.ExperimentalForeignApi
import kotlinx.coroutines.flow.first
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import okio.Path.Companion.toPath
import org.sitcon.sitlab.persistence.MobilePreferences
import org.sitcon.sitlab.persistence.PreferencesStore
import platform.Foundation.NSApplicationSupportDirectory
import platform.Foundation.NSFileManager
import platform.Foundation.NSUserDomainMask

@OptIn(ExperimentalForeignApi::class)
class IosPreferencesStore : PreferencesStore {
    private val dataStore = PreferenceDataStoreFactory.createWithPath {
        val directory = NSFileManager.defaultManager.URLForDirectory(
            NSApplicationSupportDirectory,
            NSUserDomainMask,
            appropriateForURL = null,
            create = true,
            error = null,
        )?.path ?: error("Application Support directory is unavailable")
        "$directory/mobile.preferences_pb".toPath()
    }
    private val json = Json { ignoreUnknownKeys = true; encodeDefaults = true }

    override suspend fun read(): MobilePreferences = dataStore.data.first()[Value]
        ?.let { runCatching { json.decodeFromString<MobilePreferences>(it) }.getOrNull() }
        ?: MobilePreferences()

    override suspend fun update(transform: (MobilePreferences) -> MobilePreferences) {
        val encoded = json.encodeToString(transform(read()))
        dataStore.edit { values -> values[Value] = encoded }
    }

    private companion object { val Value = stringPreferencesKey("value") }
}
