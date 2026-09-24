package org.sitcon.sitlab

import android.content.Context
import android.net.Uri
import androidx.browser.customtabs.CustomTabsIntent
import java.security.MessageDigest
import java.security.SecureRandom
import java.util.Base64
import org.sitcon.sitlab.network.ProductionOrigin

class MobileOAuthCoordinator(private val context: Context) {
    private val preferences = context.getSharedPreferences("oauth_transient", Context.MODE_PRIVATE)

    fun start() {
        val verifier = ByteArray(32).also(SecureRandom()::nextBytes).base64Url()
        val challenge = MessageDigest.getInstance("SHA-256").digest(verifier.toByteArray()).base64Url()
        preferences.edit().putString("verifier", verifier).apply()
        val uri = Uri.parse("$ProductionOrigin/api/v1/auth/gitlab/mobile").buildUpon()
            .appendQueryParameter("codeChallenge", challenge).build()
        CustomTabsIntent.Builder().build().launchUrl(context, uri)
    }

    fun complete(uri: Uri): java.util.UUID? {
        val code = uri.getQueryParameter("code") ?: return null
        val state = uri.getQueryParameter("state") ?: return null
        val verifier = preferences.getString("verifier", null) ?: return null
        preferences.edit().remove("verifier").apply()
        return MobileOAuthExchange.enqueue(context, code, state, verifier)
    }
}

private fun ByteArray.base64Url(): String = Base64.getUrlEncoder().withoutPadding().encodeToString(this)
