package org.sitcon.sitlab

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec
import org.sitcon.sitlab.platform.SecureSessionStore

class AndroidSecureSessionStore(private val context: Context) : SecureSessionStore {
    private val preferences = context.getSharedPreferences("secure_session", Context.MODE_PRIVATE)

    override suspend fun readCookie(): String? {
        val encrypted = preferences.getString("cookie", null)?.hexToBytes() ?: return null
        val iv = preferences.getString("iv", null)?.hexToBytes() ?: return null
        return Cipher.getInstance(Transformation).run {
            init(Cipher.DECRYPT_MODE, key(), GCMParameterSpec(128, iv))
            doFinal(encrypted).decodeToString()
        }
    }

    override suspend fun writeCookie(cookie: String) {
        val cipher = Cipher.getInstance(Transformation).apply { init(Cipher.ENCRYPT_MODE, key()) }
        preferences.edit()
            .putString("cookie", cipher.doFinal(cookie.encodeToByteArray()).toHex())
            .putString("iv", cipher.iv.toHex())
            .apply()
    }

    override suspend fun clear() { preferences.edit().clear().apply() }

    private fun key(): SecretKey {
        val store = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        (store.getKey(Alias, null) as? SecretKey)?.let { return it }
        return KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore").run {
            init(KeyGenParameterSpec.Builder(Alias, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .build())
            generateKey()
        }
    }

    private companion object {
        const val Alias = "sitlab_session_cookie"
        const val Transformation = "AES/GCM/NoPadding"
    }
}

private fun ByteArray.toHex() = joinToString("") { it.toUByte().toString(16).padStart(2, '0') }
private fun String.hexToBytes() = chunked(2).map { it.toInt(16).toByte() }.toByteArray()
