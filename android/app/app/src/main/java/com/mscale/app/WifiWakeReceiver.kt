package com.mscale.app

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.net.ConnectivityManager
import android.util.Log

/** When WiFi connects while the phone is awake, poll for admin wake commands. */
class WifiWakeReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent?) {
        if (intent?.action != ConnectivityManager.CONNECTIVITY_ACTION) return
        if (!WakePrefs.isWakeEnabled(context)) return
        if (!WakePrefs.isWakeWifiAwakeEnabled(context)) return
        if (!WakeConditions.canWakeWithoutCall(context)) return

        Log.i(TAG, "WiFi connected while awake — polling for remote wake")
        WakePoller.pollNow(context)
        MscaleWakeService.start(context.applicationContext)
    }

    companion object {
        private const val TAG = "WifiWakeReceiver"
    }
}
