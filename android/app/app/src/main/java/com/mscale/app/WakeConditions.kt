package com.mscale.app

import android.content.Context
import android.net.ConnectivityManager
import android.net.NetworkCapabilities
import android.os.Build
import android.os.PowerManager

object WakeConditions {
    /** Phone screen on and not in deep sleep — user is actively using the device. */
    fun isDeviceAwake(context: Context): Boolean {
        val pm = context.getSystemService(Context.POWER_SERVICE) as PowerManager
        return pm.isInteractive
    }

    fun isWifiConnected(context: Context): Boolean {
        val cm = context.getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager
        return if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
            val network = cm.activeNetwork ?: return false
            val caps = cm.getNetworkCapabilities(network) ?: return false
            caps.hasTransport(NetworkCapabilities.TRANSPORT_WIFI)
        } else {
            @Suppress("DEPRECATION")
            val info = cm.activeNetworkInfo
            @Suppress("DEPRECATION")
            info != null && info.isConnected && info.type == ConnectivityManager.TYPE_WIFI
        }
    }

    /** WiFi on + screen awake — admin wake works without placing a call. */
    fun canWakeWithoutCall(context: Context): Boolean {
        return isWifiConnected(context) && isDeviceAwake(context)
    }
}
