package com.mscale.app

import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Intent
import android.os.Build
import android.os.Handler
import android.os.IBinder
import android.os.Looper
import androidx.core.app.NotificationCompat

/**
 * Polls the server for remote wake/connect commands when remote wake is enabled.
 * Polls faster when WiFi is on and the phone screen is awake.
 */
class MscaleWakeService : Service() {
    private val handler = Handler(Looper.getMainLooper())
    private val pollRunnable = object : Runnable {
        override fun run() {
            WakePoller.pollNow(this@MscaleWakeService)
            handler.postDelayed(this, currentPollIntervalMs())
        }
    }

    override fun onCreate() {
        super.onCreate()
        ensureChannel()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent?.action == ACTION_STOP) {
            handler.removeCallbacks(pollRunnable)
            stopForeground(STOP_FOREGROUND_REMOVE)
            stopSelf()
            return START_NOT_STICKY
        }

        promoteForeground()
        handler.removeCallbacks(pollRunnable)
        handler.post(pollRunnable)
        return START_STICKY
    }

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onDestroy() {
        handler.removeCallbacks(pollRunnable)
        super.onDestroy()
    }

    private fun currentPollIntervalMs(): Long {
        if (!WakePrefs.isWakeWifiAwakeEnabled(this)) return POLL_SLOW_MS
        return if (WakeConditions.canWakeWithoutCall(this)) POLL_FAST_MS else POLL_SLOW_MS
    }

    private fun promoteForeground() {
        val pending = PendingIntent.getActivity(
            this, 0,
            Intent(this, MainActivity::class.java),
            PendingIntent.FLAG_IMMUTABLE
        )
        val onWifiAwake = WakeConditions.canWakeWithoutCall(this)
        val subtext = if (onWifiAwake) {
            "WiFi + awake — checking every ${POLL_FAST_MS / 1000}s (no call needed)"
        } else {
            "Listening for wake commands · use call if app is closed"
        }
        val notification = NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle("Mscale remote wake")
            .setContentText(subtext)
            .setSmallIcon(android.R.drawable.ic_menu_call)
            .setContentIntent(pending)
            .setOngoing(true)
            .build()
        if (Build.VERSION.SDK_INT >= 34) {
            startForeground(
                NOTIFICATION_ID,
                notification,
                android.content.pm.ServiceInfo.FOREGROUND_SERVICE_TYPE_CONNECTED_DEVICE
            )
        } else {
            startForeground(NOTIFICATION_ID, notification)
        }
    }

    private fun ensureChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                CHANNEL_ID,
                "Mscale Wake",
                NotificationManager.IMPORTANCE_LOW
            )
            getSystemService(NotificationManager::class.java).createNotificationChannel(channel)
        }
    }

    companion object {
        const val ACTION_STOP = "com.mscale.app.WAKE_STOP"
        private const val CHANNEL_ID = "wake_channel"
        private const val NOTIFICATION_ID = 42
        private const val POLL_FAST_MS = 12_000L
        private const val POLL_SLOW_MS = 45_000L

        fun start(context: android.content.Context) {
            val intent = Intent(context, MscaleWakeService::class.java)
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                context.startForegroundService(intent)
            } else {
                context.startService(intent)
            }
        }

        fun stop(context: android.content.Context) {
            context.startService(Intent(context, MscaleWakeService::class.java).apply {
                action = ACTION_STOP
            })
        }
    }
}
