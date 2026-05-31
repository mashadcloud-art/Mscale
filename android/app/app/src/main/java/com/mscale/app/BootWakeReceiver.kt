package com.mscale.app

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.util.Log

/** Restarts remote-wake polling after device reboot if the user enabled it previously. */
class BootWakeReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent?) {
        if (intent?.action != Intent.ACTION_BOOT_COMPLETED &&
            intent?.action != Intent.ACTION_MY_PACKAGE_REPLACED
        ) {
            return
        }
        if (!WakePrefs.isWakeEnabled(context)) return
        if (WakePrefs.getToken(context).isEmpty()) return
        Log.i(TAG, "Starting wake service after boot/update")
        MscaleWakeService.start(context.applicationContext)
    }

    companion object {
        private const val TAG = "BootWakeReceiver"
    }
}
