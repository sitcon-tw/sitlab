package org.sitcon.sitlab

import android.content.Context
import androidx.work.CoroutineWorker
import androidx.work.Data
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.WorkManager
import androidx.work.WorkerParameters
import org.sitcon.sitlab.network.SitLabApi
import org.sitcon.sitlab.network.platformHttpClient

class MobileOAuthExchange(context: Context, parameters: WorkerParameters) : CoroutineWorker(context, parameters) {
    override suspend fun doWork(): Result = runCatching {
        val cookie = SitLabApi(platformHttpClient()).exchange(
            inputData.getString("code") ?: error("missing code"),
            inputData.getString("state") ?: error("missing state"),
            inputData.getString("verifier") ?: error("missing verifier"),
        )
        AndroidSecureSessionStore(applicationContext).writeCookie(cookie)
    }.fold(onSuccess = { Result.success() }, onFailure = { Result.failure() })

    companion object {
        fun enqueue(context: Context, code: String, state: String, verifier: String): java.util.UUID {
            val request = OneTimeWorkRequestBuilder<MobileOAuthExchange>().setInputData(
                Data.Builder().putString("code", code).putString("state", state).putString("verifier", verifier).build(),
            ).build()
            WorkManager.getInstance(context).enqueue(request)
            return request.id
        }
    }
}
