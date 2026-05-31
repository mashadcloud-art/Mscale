package com.mscale.app

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.mscale.app.ui.theme.MscaleEmerald
import com.mscale.app.ui.theme.MscaleIndigo
import com.mscale.app.ui.theme.MscaleViolet

@Composable
fun PremiumUserHeader(
    userName: String,
    userInitial: String,
    isConnected: Boolean,
    modifier: Modifier = Modifier
) {
    val isDark = MaterialTheme.colorScheme.background.luminance() < 0.5f
    val headerGradient = if (isDark) {
        Brush.verticalGradient(
            colors = listOf(
                Color(0xFF1E1B4B),
                Color(0xFF151821),
                Color(0xFF0B0D12)
            )
        )
    } else {
        Brush.verticalGradient(
            colors = listOf(
                Color(0xFFEEF2FF),
                Color(0xFFF8FAFC),
                Color(0xFFF8FAFC)
            )
        )
    }

    Box(
        modifier = modifier
            .fillMaxWidth()
            .background(headerGradient)
            .windowInsetsPadding(WindowInsets.statusBars)
            .padding(horizontal = 20.dp)
            .padding(top = 12.dp, bottom = 20.dp)
    ) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.SpaceBetween
        ) {
            Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.weight(1f)) {
                Box(
                    modifier = Modifier
                        .size(52.dp)
                        .border(
                            width = 2.dp,
                            brush = if (isConnected) {
                                Brush.sweepGradient(listOf(MscaleEmerald, Color(0xFF34D399), MscaleEmerald))
                            } else {
                                Brush.linearGradient(listOf(MscaleIndigo, MscaleViolet))
                            },
                            shape = CircleShape
                        )
                        .padding(3.dp)
                        .clip(CircleShape)
                        .background(
                            Brush.linearGradient(listOf(MscaleIndigo, MscaleViolet))
                        ),
                    contentAlignment = Alignment.Center
                ) {
                    Text(
                        text = userInitial,
                        color = Color.White,
                        fontWeight = FontWeight.Bold,
                        fontSize = 20.sp
                    )
                }
                Spacer(modifier = Modifier.width(14.dp))
                Column {
                    Text(
                        text = "Welcome back",
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        fontSize = 12.sp,
                        fontWeight = FontWeight.Medium
                    )
                    Text(
                        text = userName,
                        color = MaterialTheme.colorScheme.onBackground,
                        fontWeight = FontWeight.Bold,
                        fontSize = 20.sp,
                        maxLines = 1
                    )
                    Text(
                        text = "Mscale Premium",
                        color = MscaleViolet,
                        fontSize = 11.sp,
                        fontWeight = FontWeight.SemiBold,
                        letterSpacing = 0.5.sp
                    )
                }
            }
            ConnectionStatusPill(isConnected = isConnected)
        }
    }
}

@Composable
private fun ConnectionStatusPill(isConnected: Boolean) {
    val bg = if (isConnected) Color(0xFF052E16) else Color(0xFF1E1B4B)
    val dot = if (isConnected) MscaleEmerald else MscaleViolet
    val label = if (isConnected) "Secure" else "Offline"
    val textColor = if (isConnected) Color(0xFF6EE7B7) else Color(0xFFC4B5FD)

    Surface(
        color = bg,
        shape = RoundedCornerShape(20.dp),
        shadowElevation = 0.dp
    ) {
        Row(
            modifier = Modifier.padding(horizontal = 12.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(6.dp)
        ) {
            Box(
                modifier = Modifier
                    .size(8.dp)
                    .clip(CircleShape)
                    .background(dot)
            )
            Text(
                text = label,
                color = textColor,
                fontSize = 12.sp,
                fontWeight = FontWeight.Bold
            )
        }
    }
}

private fun Color.luminance(): Float {
    return 0.299f * red + 0.587f * green + 0.114f * blue
}
