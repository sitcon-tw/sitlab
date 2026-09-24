package org.sitcon.sitlab

import kotlinx.cinterop.ExperimentalForeignApi
import kotlinx.cinterop.addressOf
import kotlinx.cinterop.alloc
import kotlinx.cinterop.memScoped
import kotlinx.cinterop.ptr
import kotlinx.cinterop.reinterpret
import kotlinx.cinterop.readBytes
import kotlinx.cinterop.usePinned
import kotlinx.cinterop.value
import org.sitcon.sitlab.platform.SecureSessionStore
import platform.CoreFoundation.CFDataCreate
import platform.CoreFoundation.CFDataGetBytePtr
import platform.CoreFoundation.CFDataGetLength
import platform.CoreFoundation.CFDictionaryCreateMutable
import platform.CoreFoundation.CFDictionaryRef
import platform.CoreFoundation.CFDictionarySetValue
import platform.CoreFoundation.CFRelease
import platform.CoreFoundation.CFStringCreateWithCString
import platform.CoreFoundation.CFTypeRefVar
import platform.CoreFoundation.kCFAllocatorDefault
import platform.CoreFoundation.kCFBooleanTrue
import platform.CoreFoundation.kCFStringEncodingUTF8
import platform.CoreFoundation.kCFTypeDictionaryKeyCallBacks
import platform.CoreFoundation.kCFTypeDictionaryValueCallBacks
import platform.Security.SecItemAdd
import platform.Security.SecItemCopyMatching
import platform.Security.SecItemDelete
import platform.Security.errSecSuccess
import platform.Security.kSecAttrAccount
import platform.Security.kSecAttrService
import platform.Security.kSecClass
import platform.Security.kSecClassGenericPassword
import platform.Security.kSecMatchLimit
import platform.Security.kSecMatchLimitOne
import platform.Security.kSecReturnData
import platform.Security.kSecValueData

@OptIn(ExperimentalForeignApi::class)
class IosSecureSessionStore : SecureSessionStore {
    override suspend fun readCookie(): String? = memScoped {
        val query = baseQuery()
        CFDictionarySetValue(query, kSecReturnData, kCFBooleanTrue)
        CFDictionarySetValue(query, kSecMatchLimit, kSecMatchLimitOne)
        val result = alloc<CFTypeRefVar>()
        val status = SecItemCopyMatching(query, result.ptr)
        CFRelease(query)
        if (status != errSecSuccess || result.value == null) return@memScoped null
        val data = result.value!!.reinterpret<cnames.structs.__CFData>()
        val size = CFDataGetLength(data).toInt()
        val bytes = CFDataGetBytePtr(data) ?: return@memScoped null
        bytes.readBytes(size).decodeToString()
    }

    override suspend fun writeCookie(cookie: String) = memScoped {
        clear()
        val query = baseQuery()
        val bytes = cookie.encodeToByteArray()
        val data = bytes.usePinned { CFDataCreate(kCFAllocatorDefault, it.addressOf(0).reinterpret(), bytes.size.toLong()) }
        CFDictionarySetValue(query, kSecValueData, data)
        SecItemAdd(query, null)
        data?.let(::CFRelease)
        CFRelease(query)
    }

    override suspend fun clear() {
        val query = baseQuery()
        SecItemDelete(query)
        CFRelease(query)
    }

    private fun baseQuery(): CFDictionaryRef {
        val query = memScoped {
            CFDictionaryCreateMutable(kCFAllocatorDefault, 0, kCFTypeDictionaryKeyCallBacks.ptr, kCFTypeDictionaryValueCallBacks.ptr)
        } ?: error("Could not allocate Keychain query")
        val service = CFStringCreateWithCString(kCFAllocatorDefault, Service, kCFStringEncodingUTF8)
        val account = CFStringCreateWithCString(kCFAllocatorDefault, Account, kCFStringEncodingUTF8)
        CFDictionarySetValue(query, kSecClass, kSecClassGenericPassword)
        CFDictionarySetValue(query, kSecAttrService, service)
        CFDictionarySetValue(query, kSecAttrAccount, account)
        service?.let(::CFRelease)
        account?.let(::CFRelease)
        return query
    }

    private companion object {
        const val Service = "org.sitcon.sitlab"
        const val Account = "session-cookie"
    }
}
