package com.mscale.app

import android.content.Context
import android.content.Intent
import org.json.JSONObject

object WakeLauncher {
    fun launchFromCommand(context: Context, cmd: WakeApi.PendingCommand) {
        val payload = cmd.payload
        val action = payload.optString("action", "connect")
        val intent = Intent(context, MainActivity::class.java).apply {
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP or Intent.FLAG_ACTIVITY_SINGLE_TOP)
            putExtra(MainActivity.EXTRA_AUTO_WAKE, true)
            putExtra(MainActivity.EXTRA_WAKE_ACTION, action)
            putExtra(MainActivity.EXTRA_WAKE_EXIT_NODE_ID, payload.optString("exit_node_id", ""))
            putExtra(MainActivity.EXTRA_WAKE_COUNTRY, payload.optString("country_code", "IN"))
        }
        context.startActivity(intent)
    }

    fun launchFromIncomingCall(context: Context) {
        val intent = Intent(context, MainActivity::class.java).apply {
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP or Intent.FLAG_ACTIVITY_SINGLE_TOP)
            putExtra(MainActivity.EXTRA_AUTO_WAKE, true)
            putExtra(MainActivity.EXTRA_WAKE_ACTION, "connect")
        }
        context.startActivity(intent)
    }
}
