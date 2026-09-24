package org.sitcon.sitlab

import kotlinx.cinterop.ExperimentalForeignApi
import kotlinx.cinterop.addressOf
import kotlinx.cinterop.usePinned
import org.sitcon.sitlab.network.ProductionOrigin
import platform.CoreCrypto.CC_SHA256
import platform.CoreCrypto.CC_SHA256_DIGEST_LENGTH
import platform.Foundation.NSURL
import platform.Foundation.NSURLComponents
import platform.Foundation.NSURLQueryItem
import platform.Foundation.NSUserDefaults
import platform.Security.SecRandomCopyBytes
import platform.Security.errSecSuccess
import platform.Security.kSecRandomDefault
import platform.UIKit.UIApplication

data class IosOAuthCallback(val code: String, val state: String, val verifier: String)

@OptIn(ExperimentalForeignApi::class, ExperimentalUnsignedTypes::class)
class IosOAuthCoordinator {
    private val defaults = NSUserDefaults.standardUserDefaults

    fun start() {
        val random = ByteArray(32)
        check(random.usePinned { SecRandomCopyBytes(kSecRandomDefault, random.size.toULong(), it.addressOf(0)) } == errSecSuccess)
        val verifier = random.base64Url()
        val digest = UByteArray(CC_SHA256_DIGEST_LENGTH)
        val verifierBytes = verifier.encodeToByteArray()
        verifierBytes.usePinned { input ->
            digest.usePinned { output -> CC_SHA256(input.addressOf(0), verifierBytes.size.toUInt(), output.addressOf(0)) }
        }
        defaults.setObject(verifier, forKey = Verifier)
        val challenge = ByteArray(digest.size) { digest[it].toByte() }.base64Url()
        NSURL.URLWithString("$ProductionOrigin/api/v1/auth/gitlab/mobile?codeChallenge=$challenge")?.let {
            UIApplication.sharedApplication.openURL(it, emptyMap<Any?, Any>(), null)
        }
    }

    fun complete(url: String): IosOAuthCallback? {
        val components = NSURLComponents.componentsWithString(url) ?: return null
        if (components.scheme != "https" || components.host != "sitlab.sitcon.org" || components.path != CallbackPath) return null
        val values = components.queryItems.orEmpty().filterIsInstance<NSURLQueryItem>().associate { item -> item.name to item.value }
        if (values["error"] != null) return null
        val verifier = defaults.stringForKey(Verifier) ?: return null
        defaults.removeObjectForKey(Verifier)
        return IosOAuthCallback(values["code"] ?: return null, values["state"] ?: return null, verifier)
    }

    fun cardIid(url: String): Long? {
        val components = NSURLComponents.componentsWithString(url) ?: return null
        if (components.scheme != "https" || components.host != "sitlab.sitcon.org") return null
        val path = components.path ?: return null
        if (!path.startsWith("/mobile/cards/")) return null
        return path.removePrefix("/mobile/cards/").substringBefore('/').toLongOrNull()
    }

    fun callbackError(url: String): String? {
        val components = NSURLComponents.componentsWithString(url) ?: return null
        if (components.scheme != "https" || components.host != "sitlab.sitcon.org" || components.path != CallbackPath) return null
        return components.queryItems.orEmpty().filterIsInstance<NSURLQueryItem>()
            .firstOrNull { it.name == "error" }?.value
    }

    private fun ByteArray.base64Url(): String {
        val alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
        val result = StringBuilder((size * 4 + 2) / 3)
        var index = 0
        while (index < size) {
            val first = this[index++].toInt() and 0xff
            val second = if (index < size) this[index++].toInt() and 0xff else -1
            val third = if (index < size) this[index++].toInt() and 0xff else -1
            result.append(alphabet[first ushr 2])
            result.append(alphabet[((first and 3) shl 4) or if (second >= 0) second ushr 4 else 0])
            if (second >= 0) result.append(alphabet[((second and 15) shl 2) or if (third >= 0) third ushr 6 else 0])
            if (third >= 0) result.append(alphabet[third and 63])
        }
        return result.toString()
    }

    private companion object {
        const val Verifier = "oauth_pkce_verifier"
        const val CallbackPath = "/api/v1/auth/gitlab/mobile/callback"
    }
}
