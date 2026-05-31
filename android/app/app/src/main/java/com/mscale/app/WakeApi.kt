package com.mscale.app

import org.json.JSONArray
import org.json.JSONObject
import java.net.HttpURLConnection
import java.net.URL

object WakeApi {
    private const val API_BASE = "https://mashad.shop/mscale"
    private const val SESSION_COOKIE = "mscale_session"

    data class PendingCommand(
        val commandId: String,
        val commandType: String,
        val payload: JSONObject
    )

    fun resolveDeviceId(token: String, deviceName: String): String {
        val devices = fetchDevices(token)
        val target = deviceName.trim().lowercase()
        for (i in 0 until devices.length()) {
            val obj = devices.getJSONObject(i)
            val name = obj.optString("device_name", "").lowercase()
            if (name == target || name.contains(target) || target.contains(name)) {
                return obj.optString("id", "")
            }
        }
        if (devices.length() == 1) {
            return devices.getJSONObject(0).optString("id", "")
        }
        return ""
    }

    fun fetchDevices(token: String): JSONArray {
        val conn = open("GET", "/api/devices", token)
        return try {
            val code = conn.responseCode
            val body = readBody(conn, code)
            if (code in 200..299) JSONArray(body) else JSONArray()
        } finally {
            conn.disconnect()
        }
    }

    fun fetchPendingCommand(token: String, deviceId: String): PendingCommand? {
        val conn = open("GET", "/api/devices/pending-command?device_id=$deviceId", token)
        return try {
            val code = conn.responseCode
            val body = readBody(conn, code)
            if (code !in 200..299) return null
            val root = JSONObject(body)
            if (root.isNull("command")) return null
            val cmd = root.getJSONObject("command")
            PendingCommand(
                commandId = cmd.optString("command_id", ""),
                commandType = cmd.optString("command_type", ""),
                payload = cmd.optJSONObject("payload") ?: JSONObject()
            )
        } finally {
            conn.disconnect()
        }
    }

    fun ackCommand(token: String, deviceId: String, commandId: String, status: String = "completed") {
        val payload = JSONObject()
            .put("command_id", commandId)
            .put("device_id", deviceId)
            .put("status", status)
        val conn = open("POST", "/api/devices/command-ack", token, payload.toString())
        try {
            conn.responseCode
            readBody(conn, conn.responseCode)
        } finally {
            conn.disconnect()
        }
    }

    fun saveWakeConfig(token: String, deviceId: String, enabled: Boolean, numbers: List<String>, devicePhone: String = "") {
        val payload = JSONObject()
            .put("device_id", deviceId)
            .put("enabled", enabled)
            .put("wake_numbers", JSONArray(numbers))
        if (devicePhone.isNotBlank()) {
            payload.put("device_phone", devicePhone)
        }
        val conn = open("POST", "/api/devices/wake-config", token, payload.toString())
        try {
            conn.responseCode
            readBody(conn, conn.responseCode)
        } finally {
            conn.disconnect()
        }
    }

    fun queueWake(token: String, targetDeviceId: String) {
        val payload = JSONObject()
            .put("target_device_id", targetDeviceId)
            .put("action", "connect")
        val conn = open("POST", "/api/devices/wake", token, payload.toString())
        try {
            conn.responseCode
            readBody(conn, conn.responseCode)
        } finally {
            conn.disconnect()
        }
    }

    private fun open(method: String, path: String, token: String, body: String? = null): HttpURLConnection {
        val conn = URL("$API_BASE$path").openConnection() as HttpURLConnection
        conn.requestMethod = method
        conn.setRequestProperty("Cookie", "$SESSION_COOKIE=$token")
        conn.connectTimeout = 15000
        conn.readTimeout = 15000
        if (body != null) {
            conn.doOutput = true
            conn.setRequestProperty("Content-Type", "application/json")
            conn.outputStream.use { it.write(body.toByteArray()) }
        }
        return conn
    }

    private fun readBody(conn: HttpURLConnection, code: Int): String {
        val stream = if (code in 200..299) conn.inputStream else conn.errorStream
        return stream?.bufferedReader()?.use { it.readText() }.orEmpty()
    }
}
