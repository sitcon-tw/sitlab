package org.sitcon.sitlab.persistence

import androidx.room.Room
import androidx.sqlite.driver.bundled.BundledSQLiteDriver
import kotlinx.cinterop.ExperimentalForeignApi
import kotlinx.coroutines.Dispatchers
import platform.Foundation.NSDocumentDirectory
import platform.Foundation.NSFileManager
import platform.Foundation.NSUserDomainMask

class IosDatabaseFactory : DatabaseFactory {
    @OptIn(ExperimentalForeignApi::class)
    override fun create(): SitLabDatabase {
        val directory = NSFileManager.defaultManager.URLForDirectory(
            NSDocumentDirectory,
            NSUserDomainMask,
            appropriateForURL = null,
            create = true,
            error = null,
        )?.path ?: error("Documents directory is unavailable")
        return Room.databaseBuilder<SitLabDatabase>(
            name = "$directory/sitlab.db",
            factory = { SitLabDatabaseConstructor.initialize() },
        ).setDriver(BundledSQLiteDriver())
            .setQueryCoroutineContext(Dispatchers.Default)
            .build()
    }
}
