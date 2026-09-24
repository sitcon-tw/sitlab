package org.sitcon.sitlab.persistence

import android.content.Context
import androidx.room.Room
import androidx.sqlite.driver.bundled.BundledSQLiteDriver
import kotlinx.coroutines.Dispatchers

class AndroidDatabaseFactory(private val context: Context) : DatabaseFactory {
    override fun create(): SitLabDatabase = Room.databaseBuilder<SitLabDatabase>(
        context = context.applicationContext,
        name = context.getDatabasePath("sitlab.db").absolutePath,
    ).setDriver(BundledSQLiteDriver())
        .setQueryCoroutineContext(Dispatchers.IO)
        .build()
}
