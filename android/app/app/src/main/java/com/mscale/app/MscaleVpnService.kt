package com.mscale.app

import android.content.Intent
import android.net.IpPrefix
import android.net.VpnService
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.os.ParcelFileDescriptor
import android.util.Log
import mscalecore.SocketProtector
import java.net.InetAddress

class MscaleVpnService : VpnService() {
    private var vpnInterface: ParcelFileDescriptor? = null
    private val mainHandler = Handler(Looper.getMainLooper())
    private val teardownLock = Any()
    @Volatile
    private var isTearingDown = false

    override fun onCreate() {
        super.onCreate()
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = android.app.NotificationChannel(
                "vpn_channel",
                "Mscale VPN Status",
                android.app.NotificationManager.IMPORTANCE_LOW
            )
            val manager = getSystemService(NOTIFICATION_SERVICE) as android.app.NotificationManager
            manager.createNotificationChannel(channel)
        }
    }

    private fun promoteToForeground(shareExit: Boolean) {
        val intent = Intent(this, MainActivity::class.java)
        val flags = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
            android.app.PendingIntent.FLAG_IMMUTABLE
        } else {
            0
        }
        val pendingIntent = android.app.PendingIntent.getActivity(this, 0, intent, flags)
        val title = if (shareExit) "Mscale — Exit node active" else "Mscale VPN"
        val text = if (shareExit) {
            "This phone is sharing its internet with your other devices"
        } else {
            "Connected and securing traffic"
        }
        val notification = androidx.core.app.NotificationCompat.Builder(this, "vpn_channel")
            .setContentTitle(title)
            .setContentText(text)
            .setSmallIcon(android.R.drawable.ic_secure)
            .setContentIntent(pendingIntent)
            .build()
        if (Build.VERSION.SDK_INT >= 34) {
            startForeground(1, notification, android.content.pm.ServiceInfo.FOREGROUND_SERVICE_TYPE_CONNECTED_DEVICE)
        } else {
            startForeground(1, notification)
        }
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent?.action == ACTION_DISCONNECT) {
            performDisconnect()
            return START_NOT_STICKY
        }

        synchronized(teardownLock) { isTearingDown = false }

        val shareExit = intent?.getBooleanExtra(EXTRA_SHARE_EXIT, false) ?: false
        promoteToForeground(shareExit)

        val token = intent?.getStringExtra(EXTRA_TOKEN).orEmpty()
        val exitNodeId = intent?.getStringExtra(EXTRA_EXIT_NODE_ID).orEmpty()
        val shareCountry = intent?.getStringExtra(EXTRA_SHARE_COUNTRY).orEmpty().ifEmpty { "IN" }
        val exitMode = intent?.getStringExtra(EXTRA_EXIT_MODE).orEmpty().ifEmpty { "proxy" }
        val dnsServer = intent?.getStringExtra(EXTRA_DNS_SERVER).orEmpty().ifEmpty { "8.8.8.8" }
        val deviceName = intent?.getStringExtra(EXTRA_DEVICE_NAME)
            ?: (Build.MANUFACTURER + " " + Build.MODEL).trim()
        val controller = VpnControllerHolder.instance

        controller.setSocketProtector(object : SocketProtector {
            override fun protect(fd: Long): Boolean {
                return this@MscaleVpnService.protect(fd.toInt())
            }
        })

        Thread {
            try {
                if (isTearingDown) return@Thread

                val storageDir = filesDir.absolutePath
                val overlayIp = if (shareExit) {
                    controller.prepareMeshAsExit(token, deviceName, storageDir, shareCountry, exitMode)
                } else {
                    controller.prepareMesh(token, deviceName, storageDir, exitNodeId)
                }
                if (overlayIp.isEmpty()) {
                    Log.e(TAG, "PrepareMesh failed: ${controller.getLastError()}")
                    finishService()
                    return@Thread
                }

                val useExit = !shareExit && exitNodeId.isNotEmpty()
                val builder = Builder()
                builder.setMtu(1280)
                builder.addAddress(overlayIp, 10)
                try {
                    builder.addDisallowedApplication(packageName)
                } catch (e: Exception) {
                    Log.e(TAG, "Failed to exclude app from VPN", e)
                }
                if (useExit) {
                    builder.addRoute("0.0.0.0", 1)
                    builder.addRoute("128.0.0.0", 1)
                    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
                        try {
                            builder.excludeRoute(
                                IpPrefix(InetAddress.getByName("129.151.146.44"), 32)
                            )
                        } catch (e: Exception) {
                            Log.w(TAG, "excludeRoute hub IP failed", e)
                        }
                    }
                } else {
                    builder.addRoute("100.64.0.0", 10)
                }
                builder.addDnsServer(dnsServer)
                builder.addSearchDomain("mscale")
                builder.setSession("Mscale VPN")

                vpnInterface = builder.establish()
                if (vpnInterface == null) {
                    Log.e(TAG, "VPN establish() returned null")
                    finishService()
                    return@Thread
                }

                val err = controller.startTunnel(vpnInterface!!.detachFd().toLong())
                if (err.isNotEmpty()) {
                    Log.e(TAG, "StartTunnel failed: $err")
                    disconnectSync()
                    finishService()
                } else {
                    val registeredId = controller.getDeviceID()
                    if (registeredId.isNotEmpty()) {
                        WakePrefs.saveDeviceId(this@MscaleVpnService, registeredId)
                    }
                    if (shareExit) {
                        ExitCommandPoller.start(this@MscaleVpnService)
                    }
                }
            } catch (e: Exception) {
                Log.e(TAG, "VPN error", e)
                disconnectSync()
                finishService()
            }
        }.start()

        return START_NOT_STICKY
    }

    /** Stop WireGuard first, then close VPN fd — prevents native crash on disconnect. */
    private fun disconnectSync() {
        synchronized(teardownLock) {
            if (isTearingDown) return
            isTearingDown = true
        }
        try {
            VpnControllerHolder.instance.setSocketProtector(null)
            VpnControllerHolder.instance.disconnect()
        } catch (t: Throwable) {
            Log.e(TAG, "disconnect failed", t)
        }
        try {
            vpnInterface?.close()
        } catch (t: Throwable) {
            Log.w(TAG, "vpn fd close", t)
        }
        vpnInterface = null
    }

    private fun performDisconnect() {
        ExitCommandPoller.stop()
        Thread {
            disconnectSync()
            finishService()
        }.start()
    }

    private fun finishService() {
        mainHandler.post {
            try {
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.N) {
                    stopForeground(STOP_FOREGROUND_REMOVE)
                } else {
                    @Suppress("DEPRECATION")
                    stopForeground(true)
                }
            } catch (_: Throwable) {
            }
            stopSelf()
        }
    }

    override fun onDestroy() {
        disconnectSync()
        super.onDestroy()
    }

    companion object {
        const val ACTION_DISCONNECT = "com.mscale.app.DISCONNECT"
        const val EXTRA_TOKEN = "session_token"
        const val EXTRA_EXIT_NODE_ID = "exit_node_id"
        const val EXTRA_DEVICE_NAME = "device_name"
        const val EXTRA_SHARE_EXIT = "share_exit"
        const val EXTRA_SHARE_COUNTRY = "share_country"
        const val EXTRA_EXIT_MODE = "exit_mode"
        const val EXTRA_DNS_SERVER = "dns_server"
        private const val TAG = "MscaleVpnService"
    }
}

object VpnControllerHolder {
    val instance: mscalecore.VpnController by lazy { mscalecore.Mscalecore.newVpnController() }
}
