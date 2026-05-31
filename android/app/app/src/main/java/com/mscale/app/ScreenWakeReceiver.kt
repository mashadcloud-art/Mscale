package com.mscale.app

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.util.Log

/**
 * When the user wakes the phone (screen on) on WiFi, check for admin wake commands immediately.
 * Works even if Mscale was closed — no phone call required.
 */
class ScreenWakeReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent?) {
        val action = intent?.action ?: return
        if (action != Intent.ACTION_SCREEN_ON &&
            action != Intent.ACTION_USER_PRESENT
        ) {
            return
        }
        if (!WakePrefs.isWakeEnabled(context)) return
        if (!WakePrefs.isWakeWifiAwakeEnabled(context)) return
        if (!WakeConditions.canWakeWithoutCall(context)) return

        Log.i(TAG, "Screen awake + WiFi — polling for remote wake")
        WakePoller.pollNow(context)
        MscaleWakeService.start(context.applicationContext)
    }

    companion object {
        private const val TAG = "ScreenWakeReceiver"
    }
}
