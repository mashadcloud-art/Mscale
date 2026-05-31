package com.mscale.app

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.telephony.TelephonyManager
import android.util.Log

/**
 * Opens Mscale when an incoming call arrives (Wake on Call).
 * On Android 10+ caller ID may be hidden unless READ_CALL_LOG is granted — optional "any call" mode handles that.
 */
class IncomingCallReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent?) {
        if (intent?.action != TelephonyManager.ACTION_PHONE_STATE_CHANGED) return
        if (!WakePrefs.isWakeEnabled(context)) return

        val state = intent.getStringExtra(TelephonyManager.EXTRA_STATE) ?: return
        if (state != TelephonyManager.EXTRA_STATE_RINGING) return

        val incoming = intent.getStringExtra(TelephonyManager.EXTRA_INCOMING_NUMBER).orEmpty()
        val allowed = WakePrefs.getWakeNumbers(context)
        val anyCall = WakePrefs.isWakeOnAnyCall(context)

        val normalizedIncoming = normalizePhone(incoming)
        val match = when {
            anyCall -> true
            normalizedIncoming.isEmpty() -> false
            allowed.isEmpty() -> true
            else -> allowed.any { normalizePhone(it) == normalizedIncoming ||
                normalizedIncoming.endsWith(normalizePhone(it).takeLast(10)) }
        }

        if (!match) {
            Log.d(TAG, "Incoming call ignored (not in wake list)")
            return
        }

        Log.i(TAG, "Wake on call — launching Mscale")
        WakeLauncher.launchFromIncomingCall(context.applicationContext)
        MscaleWakeService.start(context.applicationContext)
    }

    private fun normalizePhone(s: String): String {
        val b = StringBuilder()
        for (c in s) {
            if (c.isDigit() || c == '+') b.append(c)
        }
        return b.toString()
    }

    companion object {
        private const val TAG = "IncomingCallReceiver"
    }
}
