package org.sitcon.sitlab

import android.content.Context
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStoreFile
import androidx.datastore.preferences.core.PreferenceDataStoreFactory
import kotlinx.coroutines.flow.first
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import org.sitcon.sitlab.persistence.MobilePreferences
import org.sitcon.sitlab.persistence.PreferencesStore

class AndroidPreferencesStore(context: Context) : PreferencesStore {
    private val dataStore = PreferenceDataStoreFactory.create { context.preferencesDataStoreFile("mobile.preferences_pb") }
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
