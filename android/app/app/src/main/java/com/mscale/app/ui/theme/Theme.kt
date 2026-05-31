package com.mscale.app.ui.theme

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color

private val MscaleDarkScheme = darkColorScheme(
    primary = MscaleViolet,
    onPrimary = Color.White,
    primaryContainer = Color(0xFF312E81),
    onPrimaryContainer = Color(0xFFE0E7FF),
    secondary = MscaleIndigo,
    onSecondary = Color.White,
    tertiary = MscaleEmerald,
    background = MscaleBgDark,
    onBackground = Color(0xFFF1F5F9),
    surface = MscaleSurfaceDark,
    onSurface = Color(0xFFF1F5F9),
    surfaceVariant = MscaleCardDark,
    onSurfaceVariant = MscaleTextMuted,
    outline = MscaleStrokeDark,
    outlineVariant = Color(0xFF374151)
)

private val MscaleLightScheme = lightColorScheme(
    primary = MscaleIndigo,
    onPrimary = Color.White,
    primaryContainer = Color(0xFFE0E7FF),
    onPrimaryContainer = Color(0xFF312E81),
    secondary = MscaleViolet,
    onSecondary = Color.White,
    tertiary = MscaleEmeraldDark,
    background = MscaleBgLight,
    onBackground = Color(0xFF0F172A),
    surface = MscaleSurfaceLight,
    onSurface = Color(0xFF0F172A),
    surfaceVariant = MscaleCardLight,
    onSurfaceVariant = Color(0xFF64748B),
    outline = MscaleStrokeLight,
    outlineVariant = Color(0xFFCBD5E1)
)

@Composable
fun MscaleTheme(
    darkTheme: Boolean = isSystemInDarkTheme(),
    dynamicColor: Boolean = false,
    content: @Composable () -> Unit
) {
    val colorScheme = if (darkTheme) MscaleDarkScheme else MscaleLightScheme

    MaterialTheme(
        colorScheme = colorScheme,
        typography = Typography,
        content = content
    )
}
