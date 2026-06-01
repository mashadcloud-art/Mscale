package com.mscale.app

import android.Manifest
import android.app.Activity
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.content.pm.PackageManager
import android.net.Uri
import android.net.VpnService
import android.os.Build
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.animateDpAsState
import androidx.compose.animation.core.tween
import androidx.compose.animation.expandVertically
import androidx.compose.animation.shrinkVertically
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import kotlinx.coroutines.launch
import androidx.compose.foundation.clickable
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import android.widget.Toast
import androidx.core.content.ContextCompat
import androidx.core.view.WindowCompat
import com.mscale.app.ui.theme.MscaleTheme

class MainActivity : ComponentActivity() {

    private lateinit var vpnController: mscalecore.VpnController
    private var pendingWakeIntent: Intent? = null
    private val mainHandler = Handler(Looper.getMainLooper())
    
    // States that need to survive recomposition but be accessible to VpnService methods
    private var sessionToken by mutableStateOf("")
    private val isConnectedState = mutableStateOf(false)
    private var shareAsExit by mutableStateOf(false)
    private var shareCountry by mutableStateOf("IN")
    private var shareExitMode by mutableStateOf("exit_node")
    private var selectedExitNodeId by mutableStateOf("")
    private var dnsServer by mutableStateOf("8.8.8.8")
    private var wakeRemoteEnabled by mutableStateOf(false)
    private var wakeOnAnyCall by mutableStateOf(false)
    private var wakeWifiAwake by mutableStateOf(false)
    private var wakePhoneInput by mutableStateOf("")
    private var deviceMyPhone by mutableStateOf("")

    private val vpnStateReceiver = object : BroadcastReceiver() {
        override fun onReceive(context: Context?, intent: Intent?) {
            when (intent?.action) {
                MscaleVpnService.ACTION_VPN_CONNECTED -> {
                    isConnectedState.value = true
                }
                MscaleVpnService.ACTION_VPN_FAILED -> {
                    isConnectedState.value = false
                    val err = intent.getStringExtra(MscaleVpnService.EXTRA_VPN_ERROR).orEmpty()
                    if (err.isNotEmpty()) {
                        Toast.makeText(this@MainActivity, err, Toast.LENGTH_LONG).show()
                    } else {
                        Toast.makeText(this@MainActivity, "VPN connection failed", Toast.LENGTH_LONG).show()
                    }
                }
                MscaleVpnService.ACTION_VPN_DISCONNECTED -> {
                    isConnectedState.value = false
                }
            }
        }
    }

    private val vpnPermissionLauncher = registerForActivityResult(ActivityResultContracts.StartActivityForResult()) { result ->
        if (result.resultCode == RESULT_OK) {
            startVpnService()
        }
    }

    private val phonePermissionLauncher = registerForActivityResult(ActivityResultContracts.RequestPermission()) { isGranted ->
        if (isGranted) {
            syncWakeConfigToServer()
            MscaleWakeService.start(this)
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        WindowCompat.setDecorFitsSystemWindows(window, false)
        enableEdgeToEdge()

        vpnController = mscalecore.Mscalecore.newVpnController()
        sessionToken = WakePrefs.getToken(this)
        shareAsExit = WakePrefs.isShareAsExit(this)
        shareCountry = defaultExitCountry()
        WakePrefs.setShareCountry(this, shareCountry)
        shareExitMode = WakePrefs.getRoutingMode(this).ifEmpty { "mesh" }
        selectedExitNodeId = WakePrefs.getSelectedExitNodeId(this)
        wakeRemoteEnabled = WakePrefs.isWakeEnabled(this)
        wakeOnAnyCall = WakePrefs.isWakeOnAnyCall(this)
        wakeWifiAwake = WakePrefs.isWakeWifiAwakeEnabled(this)
        deviceMyPhone = WakePrefs.getDevicePhone(this)
        dnsServer = WakePrefs.getDnsServer(this)

        val vpnFilter = IntentFilter().apply {
            addAction(MscaleVpnService.ACTION_VPN_CONNECTED)
            addAction(MscaleVpnService.ACTION_VPN_DISCONNECTED)
            addAction(MscaleVpnService.ACTION_VPN_FAILED)
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            registerReceiver(vpnStateReceiver, vpnFilter, RECEIVER_NOT_EXPORTED)
        } else {
            @Suppress("DEPRECATION")
            registerReceiver(vpnStateReceiver, vpnFilter)
        }
        
        setContent {
            val isConnected by isConnectedState
            val themeMode = WakePrefs.getThemeMode(this@MainActivity)
            val isDark = when (themeMode) {
                1 -> false
                2 -> true
                else -> isSystemInDarkTheme()
            }
            MscaleTheme(darkTheme = isDark, dynamicColor = false) {
                Surface(modifier = Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
                    if (sessionToken.isEmpty()) {
                        LoginScreen(
                            onLoginSuccess = { token, userName, email ->
                                sessionToken = token
                                WakePrefs.saveSession(
                                    this@MainActivity,
                                    token,
                                    (Build.MANUFACTURER + " " + Build.MODEL).trim(),
                                    userName,
                                    email
                                )
                            }
                        )
                    } else {
                        LaunchedEffect(sessionToken) {
                            if (sessionToken.isNotEmpty()) {
                                processWakeIntent(pendingWakeIntent)
                                pendingWakeIntent = null
                            }
                        }
                        DashboardScreen(
                            vpnController = vpnController,
                            sessionToken = sessionToken,
                            isConnected = isConnected,
                            onExitNodeSelected = {
                                selectedExitNodeId = it
                                WakePrefs.setSelectedExitNodeId(this@MainActivity, it)
                                if (shareAsExit) applyShareAsExit(false, autoConnect = false)
                            },
                            shareAsExit = shareAsExit,
                            onShareAsExitChanged = { applyShareAsExit(it) },
                            shareExitMode = shareExitMode,
                            onShareExitModeChanged = {
                                shareExitMode = it
                                WakePrefs.setRoutingMode(this@MainActivity, it)
                                if (it == "exit_node" && shareAsExit) applyShareAsExit(false, autoConnect = false)
                            },
                            dnsServer = dnsServer,
                            onDnsServerChanged = { 
                                dnsServer = it
                                WakePrefs.setDnsServer(this@MainActivity, it)
                            },
                            wakeRemoteEnabled = wakeRemoteEnabled,
                            wakeOnAnyCall = wakeOnAnyCall,
                            wakeWifiAwake = wakeWifiAwake,
                            wakePhoneInput = wakePhoneInput,
                            onWakeRemoteChanged = { enabled -> applyWakeRemoteSetting(enabled) },
                            onWakeOnAnyCallChanged = { wakeOnAnyCall = it; WakePrefs.setWakeOnAnyCall(this@MainActivity, it) },
                            onWakeWifiAwakeChanged = {
                                wakeWifiAwake = it
                                WakePrefs.setWakeWifiAwakeEnabled(this@MainActivity, it)
                            },
                            onWakePhoneInputChanged = { wakePhoneInput = it },
                            deviceMyPhone = deviceMyPhone,
                            onDeviceMyPhoneChanged = {
                                deviceMyPhone = it
                                WakePrefs.setDevicePhone(this@MainActivity, it)
                                syncWakeConfigToServer()
                            },
                            onCallToWake = { id, phone -> callToWakeDevice(sessionToken, id, phone) },
                            onConnectRequest = { requestVpnPermission() },
                            onDisconnectRequest = { stopVpnService() },
                            onLogoutRequest = {
                                stopVpnService()
                                sessionToken = ""
                                WakePrefs.saveSession(this@MainActivity, "", "", "", "")
                            }
                        )
                    }
                }
            }
        }
    }

    override fun onDestroy() {
        try {
            unregisterReceiver(vpnStateReceiver)
        } catch (_: Exception) {
        }
        super.onDestroy()
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        if (intent.getBooleanExtra(EXTRA_AUTO_WAKE, false)) {
            processWakeIntent(intent)
        }
    }

    private fun applyWakeRemoteSetting(enabled: Boolean) {
        wakeRemoteEnabled = enabled
        WakePrefs.setWakeEnabled(this, enabled)
        if (enabled) {
            if (deviceMyPhone.isEmpty()) {
                val sim = PhoneUtils.readSimPhone(this)
                if (sim.isNotEmpty()) deviceMyPhone = sim
            }
            if (ContextCompat.checkSelfPermission(this, Manifest.permission.READ_PHONE_STATE)
                != PackageManager.PERMISSION_GRANTED
            ) {
                phonePermissionLauncher.launch(Manifest.permission.READ_PHONE_STATE)
            } else {
                syncWakeConfigToServer()
                MscaleWakeService.start(this)
            }
        } else {
            MscaleWakeService.stop(this)
            syncWakeConfigToServer()
        }
    }

    private fun syncWakeConfigToServer() {
        val token = sessionToken.ifEmpty { WakePrefs.getToken(this) }
        if (token.isEmpty()) return
        val numbers = wakePhoneInput.split(",").map { it.trim() }.filter { it.isNotEmpty() }
        val myPhone = PhoneUtils.normalize(deviceMyPhone)
        WakePrefs.setWakeNumbers(this, numbers)
        if (myPhone.isNotEmpty()) WakePrefs.setDevicePhone(this, myPhone)
        Thread {
            try {
                var deviceId = WakePrefs.getDeviceId(this)
                if (deviceId.isEmpty()) {
                    deviceId = WakeApi.resolveDeviceId(token, WakePrefs.getDeviceName(this))
                    if (deviceId.isNotEmpty()) WakePrefs.saveDeviceId(this, deviceId)
                }
                if (deviceId.isNotEmpty()) {
                    WakeApi.saveWakeConfig(token, deviceId, wakeRemoteEnabled, numbers, myPhone)
                }
            } catch (_: Exception) {
            }
        }.start()
    }

    fun callToWakeDevice(token: String, deviceId: String, phone: String) {
        Thread { try { WakeApi.queueWake(token, deviceId) } catch (_: Exception) {} }.start()
        startActivity(Intent(Intent.ACTION_DIAL, Uri.parse("tel:$phone")))
    }

    private fun processWakeIntent(intent: Intent?) {
        if (intent == null || !intent.getBooleanExtra(EXTRA_AUTO_WAKE, false)) return
        val token = sessionToken.ifEmpty { WakePrefs.getToken(this) }
        if (token.isEmpty()) {
            pendingWakeIntent = intent
            return
        }
        when (intent.getStringExtra(EXTRA_WAKE_ACTION).orEmpty().ifEmpty { "connect" }) {
            "enable_exit" -> {
                shareCountry = intent.getStringExtra(EXTRA_WAKE_COUNTRY).orEmpty().ifEmpty { defaultExitCountry() }
                WakePrefs.setShareCountry(this, shareCountry)
                applyShareAsExit(true, autoConnect = false)
            }
            "route_via" -> {
                applyShareAsExit(false, autoConnect = false)
                selectedExitNodeId = intent.getStringExtra(EXTRA_WAKE_EXIT_NODE_ID).orEmpty()
                shareExitMode = "exit_node"
                WakePrefs.setRoutingMode(this, "exit_node")
                WakePrefs.setSelectedExitNodeId(this, selectedExitNodeId)
            }
            "mesh" -> {
                applyShareAsExit(false, autoConnect = false)
                selectedExitNodeId = ""
                shareExitMode = "mesh"
                WakePrefs.setRoutingMode(this, "mesh")
                WakePrefs.setSelectedExitNodeId(this, "")
            }
            else -> { /* connect with current settings */ }
        }
        if (!isConnectedState.value) {
            requestVpnPermission()
        }
        intent.removeExtra(EXTRA_AUTO_WAKE)
    }

    private fun defaultExitCountry(): String = PhoneUtils.readCountryCode(this)

    /** Toggle exit-share preference; VPN starts only when user taps Connect & share (or main Connect). */
    private fun applyShareAsExit(enabled: Boolean, autoConnect: Boolean = false) {
        shareAsExit = enabled
        WakePrefs.setShareAsExit(this, enabled)
        if (enabled) {
            shareCountry = defaultExitCountry()
            WakePrefs.setShareCountry(this, shareCountry)
            shareExitMode = "mesh"
            selectedExitNodeId = ""
            WakePrefs.setRoutingMode(this, "mesh")
            WakePrefs.setSelectedExitNodeId(this, "")
            if (autoConnect) {
                if (isConnectedState.value) {
                    stopVpnService()
                    mainHandler.postDelayed({ requestVpnPermission() }, 700)
                } else {
                    requestVpnPermission()
                }
            }
        } else {
            WakePrefs.setShareAsExit(this, false)
            if (isConnectedState.value) {
                stopVpnService()
            }
        }
    }

    private fun requestVpnPermission() {
        if (shareAsExit) {
            // sharing as exit — mesh routing only
        } else if (shareExitMode == "exit_node" && selectedExitNodeId.isEmpty()) {
            Toast.makeText(this, "Select an exit server first (Servers tab)", Toast.LENGTH_LONG).show()
            return
        }
        val intent = VpnService.prepare(this)
        if (intent != null) {
            // Permission is needed, show the Android system dialogue
            vpnPermissionLauncher.launch(intent)
        } else {
            // Permission already granted
            startVpnService()
        }
    }

    private fun startVpnService() {
        if (shareAsExit) {
            shareCountry = defaultExitCountry()
            WakePrefs.setShareCountry(this, shareCountry)
        }
        val useExitRouting = !shareAsExit && shareExitMode == "exit_node" && selectedExitNodeId.isNotEmpty()
        val intent = Intent(this, MscaleVpnService::class.java).apply {
            putExtra(MscaleVpnService.EXTRA_TOKEN, sessionToken)
            putExtra(MscaleVpnService.EXTRA_SHARE_EXIT, shareAsExit)
            putExtra(MscaleVpnService.EXTRA_SHARE_COUNTRY, shareCountry)
            putExtra(MscaleVpnService.EXTRA_EXIT_MODE, if (shareAsExit) "proxy" else shareExitMode)
            putExtra(MscaleVpnService.EXTRA_DNS_SERVER, dnsServer)
            putExtra(MscaleVpnService.EXTRA_EXIT_NODE_ID, if (useExitRouting) selectedExitNodeId else "")
        }
        startForegroundService(intent)
    }

    override fun onResume() {
        super.onResume()
        syncVpnConnectedState()
    }

    private fun syncVpnConnectedState() {
        if (!MscaleVpnService.isRunning) {
            isConnectedState.value = false
        }
    }

    private fun stopVpnService() {
        isConnectedState.value = false
        val intent = Intent(this, MscaleVpnService::class.java).apply {
            action = MscaleVpnService.ACTION_DISCONNECT
        }
        startService(intent)
        mainHandler.postDelayed({ syncVpnConnectedState() }, 800)
    }
    
    companion object {
        const val EXTRA_AUTO_WAKE = "auto_wake"
        const val EXTRA_WAKE_ACTION = "wake_action"
        const val EXTRA_WAKE_COUNTRY = "wake_country"
        const val EXTRA_WAKE_EXIT_NODE_ID = "wake_exit_node_id"
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun DashboardScreen(
    modifier: Modifier = Modifier, 
    vpnController: mscalecore.VpnController,
    sessionToken: String,
    isConnected: Boolean,
    onExitNodeSelected: (String) -> Unit,
    shareAsExit: Boolean,
    onShareAsExitChanged: (Boolean) -> Unit,
    shareExitMode: String,
    onShareExitModeChanged: (String) -> Unit,
    dnsServer: String,
    onDnsServerChanged: (String) -> Unit,
    wakeRemoteEnabled: Boolean,
    wakeOnAnyCall: Boolean,
    wakeWifiAwake: Boolean,
    wakePhoneInput: String,
    onWakeRemoteChanged: (Boolean) -> Unit,
    onWakeOnAnyCallChanged: (Boolean) -> Unit,
    onWakeWifiAwakeChanged: (Boolean) -> Unit,
    onWakePhoneInputChanged: (String) -> Unit,
    deviceMyPhone: String,
    onDeviceMyPhoneChanged: (String) -> Unit,
    onCallToWake: (deviceId: String, phone: String) -> Unit,
    onConnectRequest: () -> Unit, 
    onDisconnectRequest: () -> Unit,
    onLogoutRequest: () -> Unit
) {
    var selectedTab by remember { mutableStateOf(0) }
    val ctx = LocalContext.current
    var selectedExitLabel by remember { mutableStateOf("") }
    var selectedExitId by remember { mutableStateOf(WakePrefs.getSelectedExitNodeId(ctx)) }
    val exitOptions = remember { mutableStateListOf<ExitNodeOption>() }
    var showExitPrompt by remember { mutableStateOf(false) }
    var exitNodesLoading by remember { mutableStateOf(false) }
    val exitScope = rememberCoroutineScope()
    
    val userName = remember { WakePrefs.getUserName(ctx).ifEmpty { "User" } }
    val userEmail = remember { WakePrefs.getUserEmail(ctx) }
    val deviceName = remember { WakePrefs.getDeviceName(ctx) }
    val userInitial = userName.take(1).uppercase()
    var themeMode by remember { mutableStateOf(WakePrefs.getThemeMode(ctx)) }
    
    // Live Stats
    var sessionDurationSeconds by remember { mutableStateOf(0L) }
    var downloadBytes by remember { mutableStateOf(0L) }
    var uploadBytes by remember { mutableStateOf(0L) }
    
    // Track stats while connected
    LaunchedEffect(isConnected) {
        if (isConnected) {
            val startRx = android.net.TrafficStats.getUidRxBytes(android.os.Process.myUid())
            val startTx = android.net.TrafficStats.getUidTxBytes(android.os.Process.myUid())
            var startSecs = System.currentTimeMillis() / 1000
            
            while(true) {
                kotlinx.coroutines.delay(1000)
                sessionDurationSeconds = (System.currentTimeMillis() / 1000) - startSecs
                
                val currRx = android.net.TrafficStats.getUidRxBytes(android.os.Process.myUid())
                val currTx = android.net.TrafficStats.getUidTxBytes(android.os.Process.myUid())
                if (currRx != android.net.TrafficStats.UNSUPPORTED.toLong()) {
                    downloadBytes = (currRx - startRx).coerceAtLeast(0L)
                }
                if (currTx != android.net.TrafficStats.UNSUPPORTED.toLong()) {
                    uploadBytes = (currTx - startTx).coerceAtLeast(0L)
                }
            }
        } else {
            sessionDurationSeconds = 0L
            downloadBytes = 0L
            uploadBytes = 0L
        }
    }
    
    LaunchedEffect(sessionToken) {
        if (sessionToken.isNotEmpty() && !WakePrefs.hasSeenExitPrompt(ctx)) {
            showExitPrompt = true
        }
    }

    suspend fun loadExitNodes(autoSelect: Boolean) {
        exitNodesLoading = true
        kotlinx.coroutines.withContext(kotlinx.coroutines.Dispatchers.IO) {
            try {
                val jsonStr = vpnController.fetchExitNodes(sessionToken)
                val jsonArray = org.json.JSONArray(jsonStr)
                val nodes = mutableListOf<ExitNodeOption>()
                var indiaId = ""
                var indiaLabel = ""
                val myDeviceId = WakePrefs.getDeviceId(ctx)
                val myDeviceName = WakePrefs.getDeviceName(ctx)
                for (i in 0 until jsonArray.length()) {
                    val obj = jsonArray.getJSONObject(i)
                    val id = obj.optString("id", "")
                    if (id.isEmpty()) continue
                    val nodeDeviceId = obj.optString("device_id", "")
                    val deviceName = obj.optString("device_name", "node")
                    if (nodeDeviceId.isNotEmpty() && nodeDeviceId == myDeviceId) continue
                    if (myDeviceName.isNotEmpty() && deviceName.equals(myDeviceName, ignoreCase = true)) continue
                    val country = obj.optString("country_code", obj.optString("label", "Exit"))
                    val label = "$country ($deviceName)"
                    nodes.add(ExitNodeOption(id, label, country))
                    if (country.equals("IN", true) || country.contains("India", true)) {
                        indiaId = id
                        indiaLabel = label
                    }
                }
                kotlinx.coroutines.withContext(kotlinx.coroutines.Dispatchers.Main) {
                    exitOptions.clear()
                    exitOptions.addAll(nodes)
                    if (!autoSelect) return@withContext
                    val savedId = selectedExitId.ifEmpty { WakePrefs.getSelectedExitNodeId(ctx) }
                    if (savedId.isNotEmpty()) {
                        val match = nodes.find { it.id == savedId }
                        if (match != null) {
                            selectedExitId = savedId
                            selectedExitLabel = match.label
                            onExitNodeSelected(savedId)
                            onShareExitModeChanged("exit_node")
                        }
                    } else if (indiaId.isNotEmpty() && !shareAsExit) {
                        selectedExitLabel = indiaLabel
                        selectedExitId = indiaId
                        onExitNodeSelected(indiaId)
                        onShareExitModeChanged("exit_node")
                    }
                }
            } catch (_: Exception) { }
        }
        exitNodesLoading = false
    }

    LaunchedEffect(sessionToken) {
        if (sessionToken.isNotEmpty()) loadExitNodes(autoSelect = true)
    }

    LaunchedEffect(selectedTab) {
        if (selectedTab == 1 && sessionToken.isNotEmpty()) loadExitNodes(autoSelect = false)
    }

    val isDark = when (themeMode) {
        1 -> false
        2 -> true
        else -> isSystemInDarkTheme()
    }
    val bgDark = MaterialTheme.colorScheme.background
    val cardDark = MaterialTheme.colorScheme.surfaceVariant
    val cardStroke = MaterialTheme.colorScheme.outlineVariant
    val textPrimary = MaterialTheme.colorScheme.onBackground
    val textSecondary = MaterialTheme.colorScheme.onSurfaceVariant
    val gradientPrimary = androidx.compose.ui.graphics.Brush.linearGradient(listOf(Color(0xFF6366F1), Color(0xFF8B5CF6)))
    val gradientConnected = androidx.compose.ui.graphics.Brush.linearGradient(listOf(Color(0xFF059669), Color(0xFF10B981)))

    if (showExitPrompt) {
        AlertDialog(
            onDismissRequest = {
                WakePrefs.setExitPromptShown(ctx)
                showExitPrompt = false
            },
            title = { Text("Run as exit node?", fontWeight = FontWeight.Bold) },
            text = {
                Text(
                    "Allow other devices on your account to route internet through this phone.\n\n" +
                        "Turn on the switch, then tap Connect & share.",
                    color = textSecondary
                )
            },
            confirmButton = {
                TextButton(onClick = {
                    WakePrefs.setExitPromptShown(ctx)
                    showExitPrompt = false
                    onShareAsExitChanged(true)
                }) { Text("Enable", fontWeight = FontWeight.Bold) }
            },
            dismissButton = {
                TextButton(onClick = {
                    WakePrefs.setExitPromptShown(ctx)
                    showExitPrompt = false
                }) { Text("Not now") }
            }
        )
    }

    Scaffold(
        containerColor = bgDark,
        bottomBar = {
            NavigationBar(
                containerColor = MaterialTheme.colorScheme.surface,
                tonalElevation = 8.dp
            ) {
                val items = listOf("Home" to Icons.Default.Home, "Servers" to Icons.Default.Lock, "Stats" to Icons.Default.Info, "Profile" to Icons.Default.Person)
                items.forEachIndexed { index, pair ->
                    NavigationBarItem(
                        icon = { Icon(pair.second, contentDescription = pair.first) },
                        label = { Text(pair.first) },
                        selected = selectedTab == index,
                        onClick = { selectedTab = index },
                        colors = NavigationBarItemDefaults.colors(
                            selectedIconColor = Color(0xFF8B5CF6),
                            selectedTextColor = Color(0xFF8B5CF6),
                            unselectedIconColor = Color(0xFF4B5563),
                            unselectedTextColor = Color(0xFF4B5563),
                            indicatorColor = Color(0xFF8B5CF6).copy(alpha=0.2f)
                        )
                    )
                }
            }
        }
    ) { paddingValues ->
        Box(
            modifier = modifier
                .fillMaxSize()
                .padding(paddingValues)
        ) {
            when (selectedTab) {
                0 -> HomeScreen(
                    userName = userName,
                    userEmail = userEmail,
                    userInitial = userInitial,
                    isConnected = isConnected,
                    shareAsExit = shareAsExit,
                    onShareAsExitChanged = onShareAsExitChanged,
                    routingMode = shareExitMode,
                    onRoutingModeChanged = onShareExitModeChanged,
                    isDark = isDark,
                    themeMode = themeMode,
                    onThemeModeChanged = {
                        themeMode = it
                        WakePrefs.setThemeMode(ctx, it)
                        (ctx as? android.app.Activity)?.recreate()
                    },
                    selectedExitLabel = selectedExitLabel,
                    downloadBytes = downloadBytes,
                    uploadBytes = uploadBytes,
                    sessionDurationSeconds = sessionDurationSeconds,
                    onConnectRequest = onConnectRequest,
                    onDisconnectRequest = onDisconnectRequest,
                    onNavigateToServers = { selectedTab = 1 },
                    bgDark = bgDark, cardDark = cardDark, textPrimary = textPrimary, textSecondary = textSecondary, gradientPrimary = gradientPrimary, gradientConnected = gradientConnected
                )
                1 -> ServersScreen(
                    exitOptions = exitOptions,
                    selectedExitId = selectedExitId,
                    isConnected = isConnected,
                    isLoading = exitNodesLoading,
                    onRefresh = { exitScope.launch { loadExitNodes(autoSelect = false) } },
                    shareExitMode = shareExitMode,
                    onShareExitModeChanged = onShareExitModeChanged,
                    onExitSelected = { id, label ->
                        selectedExitId = id
                        selectedExitLabel = label
                        onExitNodeSelected(id)
                    },
                    cardDark = cardDark, textPrimary = textPrimary, textSecondary = textSecondary
                )
                2 -> StatsScreen(
                    sessionDurationSeconds = sessionDurationSeconds,
                    downloadBytes = downloadBytes,
                    uploadBytes = uploadBytes,
                    isConnected = isConnected,
                    dnsServer = dnsServer,
                    shareExitMode = shareExitMode,
                    selectedExitLabel = selectedExitLabel,
                    cardDark = cardDark, textPrimary = textPrimary, textSecondary = textSecondary
                )
                3 -> ProfileScreen(
                    userName = userName,
                    userEmail = userEmail,
                    deviceName = deviceName,
                    userInitial = userInitial,
                    themeMode = themeMode,
                    onThemeModeChanged = { 
                        themeMode = it
                        WakePrefs.setThemeMode(ctx, it)
                        (ctx as? android.app.Activity)?.recreate()
                    },
                    dnsServer = dnsServer,
                    onDnsServerChanged = onDnsServerChanged,
                    wakeRemoteEnabled = wakeRemoteEnabled,
                    onWakeRemoteChanged = onWakeRemoteChanged,
                    deviceMyPhone = deviceMyPhone,
                    onDeviceMyPhoneChanged = onDeviceMyPhoneChanged,
                    onLogoutRequest = onLogoutRequest,
                    isConnected = isConnected,
                    cardDark = cardDark, textPrimary = textPrimary, textSecondary = textSecondary, gradientPrimary = gradientPrimary
                )
            }
        }
    }
}

@Composable
fun CollapsibleSection(
    title: String,
    cardDark: Color,
    defaultExpanded: Boolean = false,
    content: @Composable () -> Unit
) {
    var expanded by remember { mutableStateOf(defaultExpanded) }
    
    Column(modifier = Modifier.fillMaxWidth()) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .clickable { expanded = !expanded }
                .padding(vertical = 12.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.SpaceBetween
        ) {
            Text(title, color = Color(0xFF6B7280), fontSize = 12.sp, fontWeight = FontWeight.Bold, letterSpacing = 1.sp)
            Icon(
                imageVector = if (expanded) Icons.Default.KeyboardArrowUp else Icons.Default.KeyboardArrowDown,
                contentDescription = if (expanded) "Collapse" else "Expand",
                tint = Color(0xFF6B7280),
                modifier = Modifier.size(20.dp)
            )
        }
        
        AnimatedVisibility(
            visible = expanded,
            enter = expandVertically(animationSpec = tween(300)),
            exit = shrinkVertically(animationSpec = tween(300))
        ) {
            Card(
                modifier = Modifier.fillMaxWidth().padding(bottom = 16.dp),
                colors = CardDefaults.cardColors(containerColor = cardDark)
            ) {
                content()
            }
        }
    }
}

@Composable
fun HomeScreen(
    userName: String, userEmail: String, userInitial: String,
    isConnected: Boolean,
    shareAsExit: Boolean,
    onShareAsExitChanged: (Boolean) -> Unit,
    routingMode: String, onRoutingModeChanged: (String) -> Unit,
    isDark: Boolean, themeMode: Int, onThemeModeChanged: (Int) -> Unit,
    selectedExitLabel: String,
    downloadBytes: Long, uploadBytes: Long, sessionDurationSeconds: Long,
    onConnectRequest: () -> Unit, onDisconnectRequest: () -> Unit,
    onNavigateToServers: () -> Unit,
    bgDark: Color, cardDark: Color, textPrimary: Color, textSecondary: Color, gradientPrimary: androidx.compose.ui.graphics.Brush, gradientConnected: androidx.compose.ui.graphics.Brush
) {
    Column(
        modifier = Modifier.fillMaxSize().padding(horizontal = 20.dp, vertical = 12.dp).verticalScroll(rememberScrollState()),
    ) {
        // Header
        Row(modifier = Modifier.fillMaxWidth().padding(bottom = 16.dp), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.SpaceBetween) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Box(modifier = Modifier.size(40.dp).clip(CircleShape).background(gradientPrimary), contentAlignment = Alignment.Center) {
                    Text(userInitial, color = Color.White, fontWeight = FontWeight.Bold, fontSize = 16.sp)
                }
                Spacer(modifier = Modifier.width(12.dp))
                Column {
                    Text(userName, color = textPrimary, fontWeight = FontWeight.Bold, fontSize = 16.sp)
                    Text(
                        userEmail.ifEmpty { "Premium Plan" },
                        color = if (userEmail.isNotEmpty()) textSecondary else Color(0xFF6B7280),
                        fontSize = 12.sp,
                        maxLines = 1
                    )
                }
            }
            IconButton(onClick = {
                val nextMode = if (isDark) 1 else 2
                onThemeModeChanged(nextMode)
            }) {
                Text(text = if (isDark) "☀️" else "🌙", fontSize = 20.sp)
            }
        }
        
        PublicLocationBadge(isConnected = isConnected, isDark = isDark)

        // Shield Area
        Column(modifier = Modifier.fillMaxWidth().padding(bottom = 16.dp), horizontalAlignment = Alignment.CenterHorizontally) {
            Box(modifier = Modifier.size(130.dp), contentAlignment = Alignment.Center) {
                Box(modifier = Modifier.fillMaxSize().clip(CircleShape).background(
                    if (isConnected) androidx.compose.ui.graphics.Brush.sweepGradient(listOf(Color(0xFF059669), Color(0xFF10B981), Color(0xFF059669)))
                    else androidx.compose.ui.graphics.Brush.sweepGradient(listOf(Color(0xFF6366F1), Color(0xFF8B5CF6), Color(0xFF1A1D25), Color(0xFF6366F1)))
                ))
                Box(modifier = Modifier.size(105.dp).clip(CircleShape).background(Color(0xFF12141B)), contentAlignment = Alignment.Center) {
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        Icon(imageVector = if (isConnected) androidx.compose.material.icons.Icons.Default.CheckCircle else androidx.compose.material.icons.Icons.Default.Lock, contentDescription = null, tint = if (isConnected) Color(0xFF10B981) else Color(0xFF8B5CF6), modifier = Modifier.size(28.dp))
                        Spacer(modifier = Modifier.height(4.dp))
                        Text(if (isConnected) "PROTECTED" else "UNPROTECTED", color = if (isConnected) Color(0xFF10B981) else Color(0xFFA78BFA), fontSize = 10.sp, fontWeight = FontWeight.Bold, letterSpacing = 1.sp)
                    }
                }
            }
            Spacer(modifier = Modifier.height(12.dp))
            Button(
                onClick = { if (isConnected) onDisconnectRequest() else onConnectRequest() },
                modifier = Modifier.height(40.dp).width(120.dp),
                colors = ButtonDefaults.buttonColors(containerColor = Color.Transparent), contentPadding = PaddingValues(0.dp), shape = RoundedCornerShape(22.dp)
            ) {
                Box(modifier = Modifier.fillMaxSize().background(if (isConnected) gradientConnected else gradientPrimary), contentAlignment = Alignment.Center) {
                    Text(if (isConnected) "Disconnect" else "Connect", color = Color.White, fontWeight = FontWeight.Bold, fontSize = 14.sp)
                }
            }
        }

        // Entrance & Exit Grid
        Row(
            modifier = Modifier.fillMaxWidth().padding(bottom = 16.dp),
            horizontalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            // Entrance
            Card(
                modifier = Modifier.weight(1f).height(100.dp).clickable { onNavigateToServers() },
                colors = CardDefaults.cardColors(containerColor = cardDark),
                shape = RoundedCornerShape(16.dp),
                border = androidx.compose.foundation.BorderStroke(1.dp, if (routingMode == "exit_node") Color(0xFF8B5CF6) else androidx.compose.material3.MaterialTheme.colorScheme.outlineVariant)
            ) {
                Column(
                    modifier = Modifier.fillMaxSize().padding(12.dp),
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.Center
                ) {
                    Box(modifier = Modifier.size(36.dp).clip(CircleShape).background(if (routingMode == "exit_node") Color(0xFF8B5CF6).copy(alpha=0.2f) else Color(0xFFEEF2FF).copy(alpha=if(isDark) 0.1f else 1f)), contentAlignment = Alignment.Center) {
                        Icon(androidx.compose.material.icons.Icons.Default.KeyboardArrowDown, contentDescription = null, tint = if (routingMode == "exit_node") Color(0xFF8B5CF6) else Color(0xFF6366F1))
                    }
                    Spacer(modifier = Modifier.height(8.dp))
                    Text("Entrance", fontWeight = FontWeight.Bold, color = textPrimary, fontSize = 14.sp)
                    Text("CONNECT TO", fontSize = 10.sp, color = textSecondary, fontWeight = FontWeight.Bold)
                }
            }
            
            // Exit
            Card(
                modifier = Modifier.weight(1f).height(100.dp).clickable { onShareAsExitChanged(!shareAsExit) },
                colors = CardDefaults.cardColors(containerColor = if (shareAsExit) Color(0xFF1E1B4B) else cardDark),
                shape = RoundedCornerShape(16.dp),
                border = androidx.compose.foundation.BorderStroke(1.dp, if (shareAsExit) Color(0xFF8B5CF6) else androidx.compose.material3.MaterialTheme.colorScheme.outlineVariant)
            ) {
                Column(
                    modifier = Modifier.fillMaxSize().padding(12.dp),
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.Center
                ) {
                    Box(modifier = Modifier.size(36.dp).clip(CircleShape).background(if (shareAsExit) Color(0xFF8B5CF6).copy(alpha=0.2f) else Color(0xFFEEF2FF).copy(alpha=if(isDark) 0.1f else 1f)), contentAlignment = Alignment.Center) {
                        Icon(androidx.compose.material.icons.Icons.Default.KeyboardArrowUp, contentDescription = null, tint = if (shareAsExit) Color(0xFF8B5CF6) else Color(0xFF6366F1))
                    }
                    Spacer(modifier = Modifier.height(8.dp))
                    Text("Exit", fontWeight = FontWeight.Bold, color = if (shareAsExit) Color.White else textPrimary, fontSize = 14.sp)
                    Text("SHARE INTERNET", fontSize = 10.sp, color = if (shareAsExit) Color(0xFFA5B4FC) else textSecondary, fontWeight = FontWeight.Bold)
                }
            }
        }

        // Stats Row
        val mbDown = String.format("%.2f MB", downloadBytes / (1024.0 * 1024.0))
        val mbUp = String.format("%.2f MB", uploadBytes / (1024.0 * 1024.0))
        val min = sessionDurationSeconds / 60
        val sec = sessionDurationSeconds % 60
        val sessionStr = String.format("%d:%02d", min, sec)
        Row(modifier = Modifier.fillMaxWidth().padding(bottom = 16.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            listOf(Pair("Download", mbDown), Pair("Upload", mbUp), Pair("Session", sessionStr)).forEach { (label, value) ->
                Column(modifier = Modifier.weight(1f).clip(RoundedCornerShape(12.dp)).background(cardDark).padding(vertical = 10.dp), horizontalAlignment = Alignment.CenterHorizontally) {
                    Text(value, color = textPrimary, fontWeight = FontWeight.Bold, fontSize = 14.sp)
                    Spacer(modifier = Modifier.height(2.dp))
                    Text(label.uppercase(), color = Color(0xFF6B7280), fontSize = 9.sp, fontWeight = FontWeight.Bold, letterSpacing = 0.5.sp)
                }
            }
        }

        // Connection Mode (use another device as exit — optional)
        if (!shareAsExit) {
        Text("CONNECTION MODE", color = Color(0xFF6B7280), fontSize = 11.sp, fontWeight = FontWeight.Bold, letterSpacing = 1.sp, modifier = Modifier.padding(bottom = 8.dp))
        Row(modifier = Modifier.fillMaxWidth().padding(bottom = 16.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Box(modifier = Modifier.weight(1f).clip(RoundedCornerShape(10.dp)).background(if (routingMode == "mesh") Color(0xFF1E1B4B) else cardDark).clickable(enabled = !isConnected) { onRoutingModeChanged("mesh") }.padding(vertical = 10.dp), contentAlignment = Alignment.Center) {
                Text("Mesh", color = if (routingMode == "mesh") Color(0xFFA78BFA) else textSecondary, fontWeight = if (routingMode == "mesh") FontWeight.Bold else FontWeight.Medium, fontSize = 13.sp)
            }
            Box(modifier = Modifier.weight(1f).clip(RoundedCornerShape(10.dp)).background(if (routingMode == "exit_node") Color(0xFF1E1B4B) else cardDark).clickable(enabled = !isConnected) { onRoutingModeChanged("exit_node") }.padding(vertical = 10.dp), contentAlignment = Alignment.Center) {
                Text("Exit Node", color = if (routingMode == "exit_node") Color(0xFFA78BFA) else textSecondary, fontWeight = if (routingMode == "exit_node") FontWeight.Bold else FontWeight.Medium, fontSize = 13.sp)
            }
        }
        
        if (routingMode == "exit_node") {
            Text("CURRENT SERVER", color = Color(0xFF6B7280), fontSize = 11.sp, fontWeight = FontWeight.Bold, letterSpacing = 1.sp, modifier = Modifier.padding(bottom = 8.dp))
            Row(
                modifier = Modifier.fillMaxWidth().clip(RoundedCornerShape(12.dp)).background(cardDark).clickable(enabled = !isConnected) { onNavigateToServers() }.padding(12.dp),
                verticalAlignment = Alignment.CenterVertically
            ) {
                Icon(imageVector = androidx.compose.material.icons.Icons.Default.LocationOn, contentDescription = null, tint = textSecondary, modifier = Modifier.size(20.dp))
                Spacer(modifier = Modifier.width(8.dp))
                Text(selectedExitLabel.ifEmpty { "Select a server..." }, color = textPrimary, fontWeight = FontWeight.Bold, fontSize = 13.sp, modifier = Modifier.weight(1f))
                Icon(imageVector = Icons.Default.ArrowDropDown, contentDescription = null, tint = textSecondary)
            }
            if (selectedExitLabel.isEmpty()) {
                Text(
                    "Pick an exit server under Servers before connecting for exit IP.",
                    color = Color(0xFFF59E0B),
                    fontSize = 12.sp,
                    modifier = Modifier.padding(top = 8.dp)
                )
            }
        }
        }
    }
}

@Composable
fun ServersScreen(
    exitOptions: List<ExitNodeOption>, selectedExitId: String, isConnected: Boolean,
    isLoading: Boolean, onRefresh: () -> Unit,
    shareExitMode: String, onShareExitModeChanged: (String) -> Unit,
    onExitSelected: (String, String) -> Unit,
    cardDark: Color, textPrimary: Color, textSecondary: Color
) {
    Column(modifier = Modifier.fillMaxSize().padding(horizontal = 20.dp, vertical = 24.dp).verticalScroll(rememberScrollState())) {
        Row(
            modifier = Modifier.fillMaxWidth().padding(bottom = 4.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.SpaceBetween
        ) {
            Text(
                "Entrance Options",
                color = textPrimary,
                fontWeight = FontWeight.Bold,
                fontSize = 18.sp
            )
            IconButton(onClick = onRefresh, enabled = !isLoading) {
                if (isLoading) {
                    CircularProgressIndicator(modifier = Modifier.size(20.dp), strokeWidth = 2.dp)
                } else {
                    Icon(Icons.Default.Refresh, contentDescription = "Refresh entrance nodes", tint = textSecondary)
                }
            }
        }
        Text(
            "Select an Entrance device to route your traffic through.",
            color = textSecondary,
            fontSize = 13.sp,
            modifier = Modifier.padding(bottom = 16.dp)
        )

        CollapsibleSection("ROUTING OPTIONS", cardDark, defaultExpanded = true) {
            Column(modifier = Modifier.padding(16.dp)) {
        val isNoneSelected = shareExitMode == "mesh"
        Row(
            modifier = Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp))
                .background(if (isNoneSelected) MaterialTheme.colorScheme.primaryContainer else cardDark)
                .clickable(enabled = !isConnected) { onShareExitModeChanged("mesh") }
                .padding(16.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            Icon(Icons.Default.Close, contentDescription = null, tint = if (isNoneSelected) MaterialTheme.colorScheme.onPrimaryContainer else textSecondary, modifier = Modifier.size(24.dp))
            Spacer(modifier = Modifier.width(12.dp))
            Text("Mesh only", color = if (isNoneSelected) MaterialTheme.colorScheme.onPrimaryContainer else textPrimary, fontWeight = FontWeight.Bold, fontSize = 14.sp, modifier = Modifier.weight(1f))
            if (isNoneSelected) {
                Box(modifier = Modifier.clip(RoundedCornerShape(6.dp)).background(MaterialTheme.colorScheme.primary).padding(horizontal = 8.dp, vertical = 4.dp)) {
                    Text("Sel", color = MaterialTheme.colorScheme.onPrimary, fontSize = 11.sp, fontWeight = FontWeight.Bold)
                }
            }
        }
            }
        }

        CollapsibleSection("AVAILABLE EXIT NODES", cardDark, defaultExpanded = true) {
            Column(modifier = Modifier.padding(16.dp)) {
        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
            exitOptions.forEach { node ->
                val isSelected = shareExitMode == "exit_node" && selectedExitId == node.id
                val flag = when(node.country.uppercase()) {
                    "IN" -> "IN"
                    "AE" -> "AE"
                    "US" -> "US"
                    "DE" -> "DE"
                    else -> node.country.take(2).uppercase()
                }
                Row(
                    modifier = Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp))
                        .background(if (isSelected) MaterialTheme.colorScheme.primaryContainer else cardDark)
                        .clickable(enabled = !isConnected) { 
                            onExitSelected(node.id, node.label)
                            onShareExitModeChanged("exit_node")
                        }
                        .padding(16.dp),
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Box(modifier = Modifier.size(36.dp).clip(RoundedCornerShape(8.dp)).background(if (isSelected) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.surfaceVariant), contentAlignment = Alignment.Center) {
                        Text(flag, fontSize = 14.sp, color = if (isSelected) MaterialTheme.colorScheme.onPrimary else textPrimary)
                    }
                    Spacer(modifier = Modifier.width(12.dp))
                    Column(modifier = Modifier.weight(1f)) {
                        Text(node.label, color = if (isSelected) MaterialTheme.colorScheme.onPrimaryContainer else textPrimary, fontWeight = FontWeight.Bold, fontSize = 14.sp)
                        Text(node.id, color = if (isSelected) MaterialTheme.colorScheme.onPrimaryContainer.copy(alpha=0.7f) else textSecondary, fontSize = 12.sp, maxLines = 1)
                    }
                    if (isSelected) {
                        Box(modifier = Modifier.clip(RoundedCornerShape(6.dp)).background(MaterialTheme.colorScheme.primary).padding(horizontal = 8.dp, vertical = 4.dp)) {
                            Text("Sel", color = MaterialTheme.colorScheme.onPrimary, fontSize = 11.sp, fontWeight = FontWeight.Bold)
                        }
                    }
                }
            }
                }
                if (exitOptions.isEmpty()) {
                    Text(
                        "No exit nodes available.\n\n" +
                            "On your Xiaomi: turn Run as exit node ON, tap Connect, and keep it connected.\n\n" +
                            "On this phone: turn Run as exit node OFF, then open this tab again.",
                        color = textSecondary,
                        fontSize = 14.sp,
                        modifier = Modifier.padding(vertical = 16.dp)
                    )
                }
            }
        }
    }
}

@Composable
fun StatsScreen(
    sessionDurationSeconds: Long, downloadBytes: Long, uploadBytes: Long,
    isConnected: Boolean, dnsServer: String, shareExitMode: String, selectedExitLabel: String,
    cardDark: Color, textPrimary: Color, textSecondary: Color
) {
    var logs by remember { mutableStateOf("Fetching logs...") }
    
    LaunchedEffect(Unit) {
        kotlinx.coroutines.withContext(kotlinx.coroutines.Dispatchers.IO) {
            try {
                // Fetch last 100 lines of logcat for our app
                val process = Runtime.getRuntime().exec("logcat -d -v time -t 100")
                val reader = java.io.BufferedReader(java.io.InputStreamReader(process.inputStream))
                val sb = StringBuilder()
                var line: String?
                while (reader.readLine().also { line = it } != null) {
                    if (line?.contains("Mscale") == true || line?.contains("VpnService") == true) {
                        sb.append(line).append("\\n")
                    }
                }
                kotlinx.coroutines.withContext(kotlinx.coroutines.Dispatchers.Main) {
                    logs = sb.toString().ifEmpty { "No recent Mscale logs found." }
                }
            } catch (e: Exception) {
                logs = "Failed to fetch logs: ${e.message}"
            }
        }
    }

    Column(modifier = Modifier.fillMaxSize().padding(horizontal = 20.dp, vertical = 24.dp).verticalScroll(rememberScrollState())) {
        CollapsibleSection("NETWORK STATUS", cardDark, defaultExpanded = true) {
            Column(modifier = Modifier.padding(16.dp)) {
                Text("Connection State", color = textSecondary, fontSize = 12.sp)
                Text(if (isConnected) "Connected" else "Disconnected", color = if (isConnected) Color(0xFF10B981) else Color(0xFFEF4444), fontWeight = FontWeight.Bold)
                Spacer(modifier = Modifier.height(12.dp))
                
                Text("Assigned IP", color = textSecondary, fontSize = 12.sp)
                Text(if (isConnected) "100.64.0.2 (Mocked)" else "Not Assigned", color = textPrimary)
                Spacer(modifier = Modifier.height(12.dp))
                
                val dnsName = when(dnsServer) {
                    "8.8.8.8" -> "Google (8.8.8.8)"
                    "1.1.1.1" -> "Cloudflare (1.1.1.1)"
                    "208.67.222.222" -> "OpenDNS (208.67.222.222)"
                    else -> if (dnsServer.isEmpty()) "Google (Fallback)" else "Custom ($dnsServer)"
                }
                Text("Active DNS", color = textSecondary, fontSize = 12.sp)
                Text(dnsName, color = textPrimary)
            }
        }
        
        CollapsibleSection("ROUTING INFO", cardDark, defaultExpanded = true) {
            Column(modifier = Modifier.padding(16.dp)) {
                Text("Routing Mode", color = textSecondary, fontSize = 12.sp)
                val modeLabel = if (shareExitMode == "exit_node") "Exit Node (Routed)" else "Mesh Only"
                Text(modeLabel, color = textPrimary, fontWeight = FontWeight.Bold)
                
                Spacer(modifier = Modifier.height(12.dp))
                Text("Traffic Location", color = textSecondary, fontSize = 12.sp)
                if (shareExitMode == "exit_node") {
                    Text(selectedExitLabel.ifEmpty { "Pending Selection" }, color = textPrimary)
                } else {
                    Text("Local (Direct)", color = textPrimary)
                }
            }
        }

        CollapsibleSection("BANDWIDTH USAGE", cardDark, defaultExpanded = false) {
            Column(modifier = Modifier.padding(16.dp)) {
                Text("Total Download: ${String.format("%.2f MB", downloadBytes / (1024.0 * 1024.0))}", color = textPrimary)
                Spacer(modifier = Modifier.height(8.dp))
                Text("Total Upload: ${String.format("%.2f MB", uploadBytes / (1024.0 * 1024.0))}", color = textPrimary)
                Spacer(modifier = Modifier.height(8.dp))
                val min = sessionDurationSeconds / 60
                val sec = sessionDurationSeconds % 60
                Text("Session Time: ${String.format("%d:%02d", min, sec)}", color = textPrimary)
            }
        }
        
        CollapsibleSection("SYSTEM LOGS", cardDark, defaultExpanded = false) {
            Box(modifier = Modifier.fillMaxWidth().height(250.dp).clip(RoundedCornerShape(14.dp)).background(Color.Black).padding(16.dp).verticalScroll(rememberScrollState())) {
                Text(logs, color = Color(0xFF34D399), fontSize = 10.sp, fontFamily = androidx.compose.ui.text.font.FontFamily.Monospace)
            }
        }
    }
}

@Composable
fun ProfileScreen(
    userName: String, userEmail: String, deviceName: String,
    userInitial: String, themeMode: Int, onThemeModeChanged: (Int) -> Unit,
    dnsServer: String, onDnsServerChanged: (String) -> Unit,
    wakeRemoteEnabled: Boolean, onWakeRemoteChanged: (Boolean) -> Unit,
    deviceMyPhone: String, onDeviceMyPhoneChanged: (String) -> Unit,
    onLogoutRequest: () -> Unit, isConnected: Boolean,
    cardDark: Color, textPrimary: Color, textSecondary: Color, gradientPrimary: androidx.compose.ui.graphics.Brush
) {
    Column(modifier = Modifier.fillMaxSize().padding(horizontal = 20.dp, vertical = 24.dp).verticalScroll(rememberScrollState())) {
        Row(modifier = Modifier.fillMaxWidth().padding(bottom = 32.dp), verticalAlignment = Alignment.CenterVertically) {
            Box(modifier = Modifier.size(64.dp).clip(CircleShape).background(gradientPrimary), contentAlignment = Alignment.Center) {
                Text(userInitial, color = Color.White, fontWeight = FontWeight.Bold, fontSize = 24.sp)
            }
            Spacer(modifier = Modifier.width(16.dp))
            Column {
                Text(userName, color = textPrimary, fontWeight = FontWeight.Bold, fontSize = 20.sp)
                Text(
                    userEmail.ifEmpty { "No email saved — sign in again to refresh" },
                    color = textSecondary,
                    fontSize = 14.sp
                )
                if (deviceName.isNotEmpty()) {
                    Text("Device: $deviceName", color = textSecondary, fontSize = 12.sp)
                }
            }
        }

        CollapsibleSection("ACCOUNT", cardDark, defaultExpanded = true) {
            Column(modifier = Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                AccountDetailRow("Name", userName, textPrimary, textSecondary)
                AccountDetailRow("Email", userEmail.ifEmpty { "—" }, textPrimary, textSecondary)
                AccountDetailRow("Device", deviceName.ifEmpty { "—" }, textPrimary, textSecondary)
            }
        }

        CollapsibleSection("DNS SETTINGS", cardDark, defaultExpanded = false) {
            Column(modifier = Modifier.padding(16.dp)) {
                val isCustomDns = dnsServer != "8.8.8.8" && dnsServer != "1.1.1.1" && dnsServer != "208.67.222.222" && dnsServer != "100.64.0.1"
                Text("Primary DNS Server", color = textPrimary, fontWeight = FontWeight.SemiBold)
                
                val dnsName = when(dnsServer) {
                    "100.64.0.1" -> "Mscale DNS (100.64.0.1)"
                    "8.8.8.8" -> "Google (8.8.8.8)"
                    "1.1.1.1" -> "Cloudflare (1.1.1.1)"
                    "208.67.222.222" -> "OpenDNS (208.67.222.222)"
                    else -> if (dnsServer.isEmpty()) "Custom (No DNS)" else "Custom ($dnsServer)"
                }
                Text("Currently active: $dnsName", fontSize = 12.sp, color = textSecondary)
                
                Spacer(modifier = Modifier.height(12.dp))
                Row(modifier = Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    FilterChip(selected = dnsServer == "100.64.0.1", onClick = { onDnsServerChanged("100.64.0.1") }, label = { Text("Mscale DNS") })
                    FilterChip(selected = dnsServer == "8.8.8.8", onClick = { onDnsServerChanged("8.8.8.8") }, label = { Text("Google") })
                    FilterChip(selected = dnsServer == "1.1.1.1", onClick = { onDnsServerChanged("1.1.1.1") }, label = { Text("Cloudflare") })
                    FilterChip(selected = dnsServer == "208.67.222.222", onClick = { onDnsServerChanged("208.67.222.222") }, label = { Text("OpenDNS") })
                    FilterChip(selected = isCustomDns, onClick = { onDnsServerChanged("") }, label = { Text("Manual") })
                }
                if (isCustomDns) {
                    Spacer(modifier = Modifier.height(8.dp))
                    OutlinedTextField(
                        value = dnsServer,
                        onValueChange = onDnsServerChanged,
                        label = { Text("Custom DNS", color = textSecondary) },
                        singleLine = true, modifier = Modifier.fillMaxWidth(),
                        colors = OutlinedTextFieldDefaults.colors(focusedTextColor = textPrimary, unfocusedTextColor = textPrimary)
                    )
                }
            }
        }
        
        CollapsibleSection("ADVANCED SETTINGS", cardDark, defaultExpanded = false) {
            Column(modifier = Modifier.padding(16.dp)) {
                Text("Remote Wake", color = textPrimary, fontWeight = FontWeight.SemiBold)
                Text("Allow remote wake", fontSize = 12.sp, color = textSecondary)
                Switch(checked = wakeRemoteEnabled, onCheckedChange = onWakeRemoteChanged)
                if (wakeRemoteEnabled) {
                    OutlinedTextField(
                        value = deviceMyPhone, onValueChange = onDeviceMyPhoneChanged,
                        label = { Text("This phone's number", color = textSecondary) },
                        singleLine = true, modifier = Modifier.fillMaxWidth(),
                        colors = OutlinedTextFieldDefaults.colors(focusedTextColor = textPrimary, unfocusedTextColor = textPrimary)
                    )
                }
            }
        }
        
        Spacer(modifier = Modifier.height(32.dp))
        Button(
            onClick = onLogoutRequest, modifier = Modifier.fillMaxWidth().height(56.dp),
            colors = ButtonDefaults.buttonColors(containerColor = Color(0xFFEF4444)), shape = RoundedCornerShape(16.dp)
        ) {
            Text("Logout", fontWeight = FontWeight.Bold, fontSize = 16.sp)
        }
    }
}

data class ExitNodeOption(val id: String, val label: String, val country: String = "")

@Composable
private fun AccountDetailRow(label: String, value: String, textPrimary: Color, textSecondary: Color) {
    Column {
        Text(label.uppercase(), color = textSecondary, fontSize = 10.sp, fontWeight = FontWeight.Bold, letterSpacing = 1.sp)
        Text(value, color = textPrimary, fontSize = 15.sp, fontWeight = FontWeight.Medium)
    }
}

@Composable
fun PublicLocationBadge(isConnected: Boolean, isDark: Boolean) {
    var locationText by remember { mutableStateOf("Fetching...") }

    LaunchedEffect(isConnected) {
        if (!isConnected) {
            locationText = "Not connected"
            return@LaunchedEffect
        }
        kotlinx.coroutines.delay(1500)
        kotlinx.coroutines.withContext(kotlinx.coroutines.Dispatchers.IO) {
            try {
                val url = java.net.URL("https://ipwho.is/")
                val conn = url.openConnection() as java.net.HttpURLConnection
                conn.requestMethod = "GET"
                conn.connectTimeout = 5000
                conn.readTimeout = 5000
                if (conn.responseCode == 200) {
                    val stream = conn.inputStream.bufferedReader().readText()
                    val obj = org.json.JSONObject(stream)
                    val city = obj.optString("city", "")
                    val country = obj.optString("country", "")
                    if (city.isNotEmpty() && country.isNotEmpty()) {
                        locationText = "$city, $country"
                    } else {
                        locationText = "Unknown Location"
                    }
                } else {
                    locationText = "Offline / Unknown"
                }
            } catch (e: Exception) {
                locationText = "Offline / Unknown"
            }
        }
    }

    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(bottom = 16.dp),
        horizontalArrangement = Arrangement.Center
    ) {
        Surface(
            color = if (isDark) Color(0xFF312E81).copy(alpha = 0.2f) else Color(0xFFEEF2FF),
            shape = RoundedCornerShape(16.dp),
            modifier = Modifier.border(1.dp, if (isDark) Color(0xFF3730A3) else Color(0xFFE0E7FF), RoundedCornerShape(16.dp))
        ) {
            Row(
                modifier = Modifier.padding(horizontal = 12.dp, vertical = 6.dp),
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text("📍", fontSize = 14.sp)
                Spacer(modifier = Modifier.width(6.dp))
                Column(horizontalAlignment = Alignment.Start) {
                    Text("YOUR LOCATION", color = if (isDark) Color(0xFF6366F1) else Color(0xFF818CF8), fontSize = 9.sp, fontWeight = FontWeight.Bold, letterSpacing = 1.sp)
                    Text(locationText, color = if (isDark) Color(0xFFA5B4FC) else Color(0xFF4338CA), fontSize = 12.sp, fontWeight = FontWeight.Bold)
                }
            }
        }
    }
}



