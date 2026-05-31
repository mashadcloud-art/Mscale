package com.mscale.app

import android.content.Context
import android.os.Build
import android.util.Log

/** Shared server poll used by wake service and WiFi/screen receivers. */
object WakePoller {
    private const val TAG = "WakePoller"

    fun pollNow(context: Context) {
        Thread { pollBlocking(context.applicationContext) }.start()
    }

    fun pollBlocking(context: Context) {
        if (!WakePrefs.isWakeEnabled(context)) return
        val token = WakePrefs.getToken(context)
        if (token.isEmpty()) return

        var deviceId = WakePrefs.getDeviceId(context)
        if (deviceId.isEmpty()) {
            val name = WakePrefs.getDeviceName(context).ifEmpty {
                (Build.MANUFACTURER + " " + Build.MODEL).trim()
            }
            deviceId = WakeApi.resolveDeviceId(token, name)
            if (deviceId.isNotEmpty()) WakePrefs.saveDeviceId(context, deviceId)
        }
        if (deviceId.isEmpty()) return

        try {
            val cmd = WakeApi.fetchPendingCommand(token, deviceId) ?: return
            Log.i(TAG, "Executing wake command: ${cmd.commandType}")
            WakeLauncher.launchFromCommand(context, cmd)
            WakeApi.ackCommand(token, deviceId, cmd.commandId, "completed")
        } catch (e: Exception) {
            Log.w(TAG, "Wake poll failed", e)
        }
    }
}
