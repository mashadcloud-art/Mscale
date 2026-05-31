package com.mscale.app

import android.app.NotificationManager
import android.content.Context
import android.util.Log
import androidx.core.app.NotificationCompat

/** Polls for exit_test_ping while this device is sharing exit node. */
object ExitCommandPoller {
    private const val TAG = "ExitCommandPoller"
    @Volatile private var running = false

    fun start(context: Context) {
        if (running) return
        running = true
        Thread {
            val app = context.applicationContext
            while (running) {
                try {
                    pollOnce(app)
                } catch (e: Exception) {
                    Log.w(TAG, "poll error", e)
                }
                Thread.sleep(5_000)
            }
        }.start()
    }

    /** Poll immediately after a desktop exit test is queued. */
    fun nudge(context: Context) {
        if (!running) return
        Thread {
            try {
                pollOnce(context.applicationContext)
            } catch (e: Exception) {
                Log.w(TAG, "nudge poll error", e)
            }
        }.start()
    }

    fun stop() {
        running = false
    }

    private fun pollOnce(context: Context) {
        val token = WakePrefs.getToken(context)
        if (token.isEmpty()) return
        var deviceId = WakePrefs.getDeviceId(context)
        if (deviceId.isEmpty()) {
            val name = WakePrefs.getDeviceName(context).ifEmpty {
                android.os.Build.MANUFACTURER + " " + android.os.Build.MODEL
            }
            deviceId = WakeApi.resolveDeviceId(token, name.trim())
            if (deviceId.isNotEmpty()) WakePrefs.saveDeviceId(context, deviceId)
        }
        if (deviceId.isEmpty()) return

        val cmd = WakeApi.fetchPendingCommand(token, deviceId) ?: return
        if (cmd.commandType != "exit_test_ping") return

        val fromName = cmd.payload.optString("from_device_name", "Another device")
        val message = cmd.payload.optString("message", "$fromName is using you as exit node")
        showNotification(context, "Exit node in use", message)
        WakeApi.ackCommand(token, deviceId, cmd.commandId, "completed")
        Log.i(TAG, "exit_test_ping delivered from $fromName")
    }

    private fun showNotification(context: Context, title: String, text: String) {
        val channelId = "exit_test_channel"
        if (android.os.Build.VERSION.SDK_INT >= android.os.Build.VERSION_CODES.O) {
            val mgr = context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
            mgr.createNotificationChannel(
                android.app.NotificationChannel(
                    channelId,
                    "Exit node alerts",
                    NotificationManager.IMPORTANCE_HIGH
                )
            )
        }
        val notification = NotificationCompat.Builder(context, channelId)
            .setContentTitle(title)
            .setContentText(text)
            .setStyle(NotificationCompat.BigTextStyle().bigText(text))
            .setSmallIcon(android.R.drawable.ic_dialog_info)
            .setPriority(NotificationCompat.PRIORITY_HIGH)
            .setDefaults(NotificationCompat.DEFAULT_ALL)
            .setAutoCancel(true)
            .build()
        val mgr = context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        mgr.notify(9001, notification)
    }
}
