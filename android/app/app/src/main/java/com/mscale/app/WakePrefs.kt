package com.mscale.app

import android.content.Context

object WakePrefs {
    private const val PREFS = "mscale_wake"
    private const val KEY_TOKEN = "session_token"
    private const val KEY_DEVICE_ID = "device_id"
    private const val KEY_DEVICE_NAME = "device_name"
    private const val KEY_USER_NAME = "user_name"
    private const val KEY_USER_EMAIL = "user_email"
    private const val KEY_ROUTING_MODE = "routing_mode"
    private const val KEY_SELECTED_EXIT_ID = "selected_exit_node_id"
    private const val KEY_SHARE_AS_EXIT = "share_as_exit"
    private const val KEY_SHARE_COUNTRY = "share_country"
    private const val KEY_EXIT_PROMPT_SHOWN = "exit_prompt_shown"
    private const val KEY_WAKE_ENABLED = "wake_remote_enabled"
    private const val KEY_WAKE_NUMBERS = "wake_call_numbers"
    private const val KEY_WAKE_ON_ANY_CALL = "wake_on_any_call"
    private const val KEY_WAKE_WIFI_AWAKE = "wake_wifi_awake_enabled"
    private const val KEY_DEVICE_PHONE = "device_phone"

    fun getDevicePhone(context: Context): String =
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).getString(KEY_DEVICE_PHONE, "").orEmpty()

    fun setDevicePhone(context: Context, phone: String) {
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit()
            .putString(KEY_DEVICE_PHONE, phone)
            .apply()
    }

    fun saveSession(context: Context, token: String, deviceName: String, userName: String = "", userEmail: String = "") {
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit()
            .putString(KEY_TOKEN, token)
            .putString(KEY_DEVICE_NAME, deviceName)
            .putString(KEY_USER_NAME, userName)
            .putString(KEY_USER_EMAIL, userEmail)
            .apply()
    }

    fun saveDeviceId(context: Context, deviceId: String) {
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit()
            .putString(KEY_DEVICE_ID, deviceId)
            .apply()
    }

    fun getToken(context: Context): String =
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).getString(KEY_TOKEN, "").orEmpty()

    fun getDeviceId(context: Context): String =
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).getString(KEY_DEVICE_ID, "").orEmpty()

    fun getDeviceName(context: Context): String =
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).getString(KEY_DEVICE_NAME, "").orEmpty()

    fun getUserName(context: Context): String =
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).getString(KEY_USER_NAME, "User").orEmpty()

    fun getUserEmail(context: Context): String =
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).getString(KEY_USER_EMAIL, "").orEmpty()

    fun getRoutingMode(context: Context): String =
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).getString(KEY_ROUTING_MODE, "exit_node").orEmpty()

    fun setRoutingMode(context: Context, mode: String) {
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit()
            .putString(KEY_ROUTING_MODE, mode)
            .apply()
    }

    fun getSelectedExitNodeId(context: Context): String =
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).getString(KEY_SELECTED_EXIT_ID, "").orEmpty()

    fun setSelectedExitNodeId(context: Context, exitNodeId: String) {
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit()
            .putString(KEY_SELECTED_EXIT_ID, exitNodeId)
            .apply()
    }

    fun isShareAsExit(context: Context): Boolean =
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).getBoolean(KEY_SHARE_AS_EXIT, false)

    fun setShareAsExit(context: Context, enabled: Boolean) {
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit()
            .putBoolean(KEY_SHARE_AS_EXIT, enabled)
            .apply()
    }

    fun getShareCountry(context: Context): String =
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).getString(KEY_SHARE_COUNTRY, "IN").orEmpty()

    fun setShareCountry(context: Context, country: String) {
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit()
            .putString(KEY_SHARE_COUNTRY, country.uppercase())
            .apply()
    }

    fun hasSeenExitPrompt(context: Context): Boolean =
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).getBoolean(KEY_EXIT_PROMPT_SHOWN, false)

    fun setExitPromptShown(context: Context) {
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit()
            .putBoolean(KEY_EXIT_PROMPT_SHOWN, true)
            .apply()
    }

    fun getRememberedEmail(context: Context): String =
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).getString("remembered_email", "").orEmpty()

    fun setRememberedEmail(context: Context, email: String) {
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit()
            .putString("remembered_email", email)
            .apply()
    }

    fun isWakeEnabled(context: Context): Boolean =
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).getBoolean(KEY_WAKE_ENABLED, false)

    fun setWakeEnabled(context: Context, enabled: Boolean) {
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit()
            .putBoolean(KEY_WAKE_ENABLED, enabled)
            .apply()
    }

    fun isWakeOnAnyCall(context: Context): Boolean =
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).getBoolean(KEY_WAKE_ON_ANY_CALL, true)

    fun setWakeOnAnyCall(context: Context, anyCall: Boolean) {
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit()
            .putBoolean(KEY_WAKE_ON_ANY_CALL, anyCall)
            .apply()
    }

    fun isWakeWifiAwakeEnabled(context: Context): Boolean =
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).getBoolean(KEY_WAKE_WIFI_AWAKE, true)

    fun setWakeWifiAwakeEnabled(context: Context, enabled: Boolean) {
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit()
            .putBoolean(KEY_WAKE_WIFI_AWAKE, enabled)
            .apply()
    }

    fun getWakeNumbers(context: Context): List<String> {
        val raw = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
            .getString(KEY_WAKE_NUMBERS, "").orEmpty()
        if (raw.isBlank()) return emptyList()
        return raw.split(",").map { it.trim() }.filter { it.isNotEmpty() }
    }

    fun setWakeNumbers(context: Context, numbers: List<String>) {
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit()
            .putString(KEY_WAKE_NUMBERS, numbers.joinToString(","))
            .apply()
    }

    fun getThemeMode(context: Context): Int =
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).getInt("theme_mode", 0)

    fun setThemeMode(context: Context, mode: Int) {
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit()
            .putInt("theme_mode", mode)
            .apply()
    }
    
    fun getDnsServer(context: Context): String =
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).getString("dns_server", "8.8.8.8") ?: "8.8.8.8"
        
    fun setDnsServer(context: Context, dns: String) {
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit()
            .putString("dns_server", dns)
            .apply()
    }
}
