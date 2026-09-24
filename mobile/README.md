# SitLab Mobile

Android and iOS Kotlin Multiplatform client for `https://sitlab.sitcon.org`. The shared module owns Compose UI, domain behavior, sync, persistence, and notification planning. Platform source sets own secure storage, browser/deep-link integration, background work, haptics, notification delivery, and sharing.

There are deliberately no desktop, JVM application, JavaScript, or Wasm targets.

Use the root `Justfile` commands. The iOS wrapper runs `:composeApp:embedAndSignAppleFrameworkForXcode`; Apple association and signing values remain external release prerequisites.
