package com.mscale.app

import android.Manifest
import android.content.Context
import android.content.pm.PackageManager
import android.os.Build
import android.telephony.TelephonyManager
import android.telephony.SubscriptionManager
import androidx.core.content.ContextCompat

object PhoneUtils {
    fun readSimPhone(context: Context): String {
        if (ContextCompat.checkSelfPermission(context, Manifest.permission.READ_PHONE_STATE)
            != PackageManager.PERMISSION_GRANTED
        ) {
            return ""
        }
        return try {
            val tm = context.getSystemService(Context.TELEPHONY_SERVICE) as TelephonyManager
            val direct = tm.line1Number?.trim().orEmpty()
            if (direct.isNotEmpty() && direct != "null") return normalize(direct)
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.LOLLIPOP_MR1) {
                val sm = context.getSystemService(SubscriptionManager::class.java)
                sm?.activeSubscriptionInfoList?.forEach { info ->
                    val subTm = tm.createForSubscriptionId(info.subscriptionId)
                    val n = subTm.line1Number?.trim().orEmpty()
                    if (n.isNotEmpty() && n != "null") return normalize(n)
                }
            }
            ""
        } catch (_: Exception) {
            ""
        }
    }

    fun normalize(raw: String): String {
        val b = StringBuilder()
        for (c in raw.trim()) {
            if (c.isDigit() || c == '+') b.append(c)
        }
        return b.toString()
    }
}
