package org.sitcon.sitlab.ui.theme

import androidx.compose.animation.core.Spring
import androidx.compose.animation.core.spring
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.ColorScheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color
import org.sitcon.sitlab.persistence.Appearance
import org.sitcon.sitlab.persistence.ColorStyle

data class SitLabMotionScheme(val reduced: Boolean) {
    fun <T> fast() = spring<T>(
        dampingRatio = if (reduced) Spring.DampingRatioNoBouncy else 0.82f,
        stiffness = if (reduced) 10_000f else Spring.StiffnessHigh,
    )
    fun <T> spatial() = spring<T>(
        dampingRatio = if (reduced) Spring.DampingRatioNoBouncy else 0.78f,
        stiffness = if (reduced) 10_000f else Spring.StiffnessMedium,
    )
    fun <T> expressive() = spring<T>(
        dampingRatio = if (reduced) Spring.DampingRatioNoBouncy else 0.68f,
        stiffness = if (reduced) 10_000f else Spring.StiffnessLow,
    )
}

val LocalSitLabMotion = staticCompositionLocalOf { SitLabMotionScheme(reduced = false) }

@Composable
expect fun platformDynamicColorScheme(dark: Boolean): ColorScheme?

@Composable
fun SitLabTheme(
    appearance: Appearance = Appearance.System,
    colorStyle: ColorStyle = ColorStyle.Sitcon,
    reducedMotion: Boolean = false,
    content: @Composable () -> Unit,
) {
    val dark = when (appearance) {
        Appearance.System -> isSystemInDarkTheme()
        Appearance.Light -> false
        Appearance.Dark -> true
    }
    val dynamic = if (colorStyle == ColorStyle.Dynamic) platformDynamicColorScheme(dark) else null
    androidx.compose.runtime.CompositionLocalProvider(LocalSitLabMotion provides SitLabMotionScheme(reducedMotion)) {
        MaterialTheme(
            colorScheme = dynamic ?: if (dark) sitLabDarkScheme() else sitLabLightScheme(),
            content = content,
        )
    }
}

private fun sitLabLightScheme() = with(GeneratedSitLabColors.Light) {
    lightColorScheme(
        primary = Color(primary), onPrimary = Color(onPrimary),
        primaryContainer = Color(primaryContainer), onPrimaryContainer = Color(onPrimaryContainer),
        secondary = Color(secondary), secondaryContainer = Color(secondaryContainer), tertiary = Color(tertiary),
        error = Color(error), errorContainer = Color(errorContainer), surface = Color(surface),
        onSurface = Color(onSurface), surfaceContainer = Color(surfaceContainer),
        surfaceContainerHigh = Color(surfaceContainerHigh), outline = Color(outline),
    )
}

private fun sitLabDarkScheme() = with(GeneratedSitLabColors.Dark) {
    darkColorScheme(
        primary = Color(primary), onPrimary = Color(onPrimary),
        primaryContainer = Color(primaryContainer), onPrimaryContainer = Color(onPrimaryContainer),
        secondary = Color(secondary), secondaryContainer = Color(secondaryContainer), tertiary = Color(tertiary),
        error = Color(error), errorContainer = Color(errorContainer), surface = Color(surface),
        onSurface = Color(onSurface), surfaceContainer = Color(surfaceContainer),
        surfaceContainerHigh = Color(surfaceContainerHigh), outline = Color(outline),
    )
}
