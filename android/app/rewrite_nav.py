import re

with open('C:\\Users\\PC\\Desktop\\Mscale\\Mscale\\android\\app\\app\\src\\main\\java\\com\\mscale\\app\\MainActivity.kt', 'r', encoding='utf-8') as f:
    content = f.read()

# Make edge-to-edge
if "WindowCompat.setDecorFitsSystemWindows(window, false)" not in content:
    content = content.replace(
        "super.onCreate(savedInstanceState)",
        "super.onCreate(savedInstanceState)\n        androidx.core.view.WindowCompat.setDecorFitsSystemWindows(window, false)"
    )

# Change setContent theme logic
if "val themeMode = remember" not in content:
    content = content.replace(
        "MscaleTheme {",
        """val themeMode = remember { mutableStateOf(WakePrefs.getThemeMode(this@MainActivity)) }
                val isDark = when(themeMode.value) {
                    1 -> false
                    2 -> true
                    else -> androidx.compose.foundation.isSystemInDarkTheme()
                }
                MscaleTheme(darkTheme = isDark) {"""
    )

# Now, we need to rewrite DashboardScreen
new_dashboard = """@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun DashboardScreen(
    modifier: Modifier = Modifier, 
    vpnController: mscalecore.VpnController,
    sessionToken: String,
    isConnected: Boolean,
    onExitNodeSelected: (String) -> Unit,
    shareAsExit: Boolean,
    onShareAsExitChanged: (Boolean) -> Unit,
    shareCountry: String,
    onShareCountryChanged: (String) -> Unit,
    shareExitMode: String,
    onShareExitModeChanged: (String) -> Unit,
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
    var routingMode by remember { mutableStateOf("exit_node") }
    var selectedExitLabel by remember { mutableStateOf("") }
    var selectedExitId by remember { mutableStateOf("") }
    val exitOptions = remember { mutableStateListOf<ExitNodeOption>() }
    
    val ctx = LocalContext.current
    val userName = remember { WakePrefs.getUserName(ctx).ifEmpty { "User" } }
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
        kotlinx.coroutines.withContext(kotlinx.coroutines.Dispatchers.IO) {
            try {
                val jsonStr = vpnController.fetchExitNodes(sessionToken)
                val jsonArray = org.json.JSONArray(jsonStr)
                val nodes = mutableListOf<ExitNodeOption>()
                var indiaId = ""
                var indiaLabel = ""
                for (i in 0 until jsonArray.length()) {
                    val obj = jsonArray.getJSONObject(i)
                    val id = obj.optString("id", "")
                    if (id.isEmpty()) continue
                    val country = obj.optString("country_code", obj.optString("label", "Exit"))
                    val deviceName = obj.optString("device_name", "node")
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
                    if (indiaId.isNotEmpty() && selectedExitId.isEmpty()) {
                        selectedExitLabel = indiaLabel
                        selectedExitId = indiaId
                        onExitNodeSelected(indiaId)
                    }
                }
            } catch (_: Exception) { }
        }
    }

    val isDark = androidx.compose.foundation.isSystemInDarkTheme() || themeMode == 2
    val bgDark = MaterialTheme.colorScheme.background
    val cardDark = MaterialTheme.colorScheme.surfaceVariant
    val cardStroke = MaterialTheme.colorScheme.outlineVariant
    val textPrimary = MaterialTheme.colorScheme.onBackground
    val textSecondary = MaterialTheme.colorScheme.onSurfaceVariant
    val gradientPrimary = androidx.compose.ui.graphics.Brush.linearGradient(listOf(Color(0xFF6366F1), Color(0xFF8B5CF6)))
    val gradientConnected = androidx.compose.ui.graphics.Brush.linearGradient(listOf(Color(0xFF059669), Color(0xFF10B981)))

    Scaffold(
        containerColor = bgDark,
        bottomBar = {
            androidx.compose.material3.NavigationBar(
                containerColor = MaterialTheme.colorScheme.surface,
                tonalElevation = 8.dp
            ) {
                val items = listOf("Home" to androidx.compose.material.icons.Icons.Default.Home, "Servers" to androidx.compose.material.icons.Icons.Default.Lock, "Stats" to androidx.compose.material.icons.Icons.Default.Info, "Profile" to androidx.compose.material.icons.Icons.Default.Person)
                items.forEachIndexed { index, pair ->
                    androidx.compose.material3.NavigationBarItem(
                        icon = { Icon(pair.second, contentDescription = pair.first) },
                        label = { Text(pair.first) },
                        selected = selectedTab == index,
                        onClick = { selectedTab = index },
                        colors = androidx.compose.material3.NavigationBarItemDefaults.colors(
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
                .padding(androidx.compose.foundation.layout.WindowInsets.statusBars.asPaddingValues())
        ) {
            when (selectedTab) {
                0 -> HomeScreen(
                    userName = userName,
                    userInitial = userInitial,
                    isConnected = isConnected,
                    routingMode = routingMode,
                    onRoutingModeChanged = { routingMode = it },
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
                    cardDark = cardDark, textPrimary = textPrimary, textSecondary = textSecondary
                )
                3 -> ProfileScreen(
                    userName = userName,
                    userInitial = userInitial,
                    themeMode = themeMode,
                    onThemeModeChanged = { 
                        themeMode = it
                        WakePrefs.setThemeMode(ctx, it)
                        // Trigger an activity recreate to apply theme globally
                        (ctx as? android.app.Activity)?.recreate()
                    },
                    shareAsExit = shareAsExit,
                    onShareAsExitChanged = onShareAsExitChanged,
                    shareCountry = shareCountry,
                    onShareCountryChanged = onShareCountryChanged,
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
fun HomeScreen(
    userName: String, userInitial: String,
    isConnected: Boolean,
    routingMode: String, onRoutingModeChanged: (String) -> Unit,
    selectedExitLabel: String,
    downloadBytes: Long, uploadBytes: Long, sessionDurationSeconds: Long,
    onConnectRequest: () -> Unit, onDisconnectRequest: () -> Unit,
    onNavigateToServers: () -> Unit,
    bgDark: Color, cardDark: Color, textPrimary: Color, textSecondary: Color, gradientPrimary: androidx.compose.ui.graphics.Brush, gradientConnected: androidx.compose.ui.graphics.Brush
) {
    Column(
        modifier = Modifier.fillMaxSize().padding(horizontal = 20.dp, vertical = 24.dp).verticalScroll(rememberScrollState()),
    ) {
        // Header
        Row(modifier = Modifier.fillMaxWidth().padding(bottom = 24.dp), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.SpaceBetween) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Box(modifier = Modifier.size(40.dp).clip(CircleShape).background(gradientPrimary), contentAlignment = Alignment.Center) {
                    Text(userInitial, color = Color.White, fontWeight = FontWeight.Bold, fontSize = 16.sp)
                }
                Spacer(modifier = Modifier.width(12.dp))
                Column {
                    Text(userName, color = textPrimary, fontWeight = FontWeight.Bold, fontSize = 16.sp)
                    Text("Premium Plan", color = Color(0xFF6B7280), fontSize = 12.sp)
                }
            }
        }

        // Shield Area
        Column(modifier = Modifier.fillMaxWidth().padding(bottom = 24.dp), horizontalAlignment = Alignment.CenterHorizontally) {
            Box(modifier = Modifier.size(160.dp), contentAlignment = Alignment.Center) {
                Box(modifier = Modifier.fillMaxSize().clip(CircleShape).background(
                    if (isConnected) androidx.compose.ui.graphics.Brush.sweepGradient(listOf(Color(0xFF059669), Color(0xFF10B981), Color(0xFF059669)))
                    else androidx.compose.ui.graphics.Brush.sweepGradient(listOf(Color(0xFF6366F1), Color(0xFF8B5CF6), Color(0xFF1A1D25), Color(0xFF6366F1)))
                ))
                Box(modifier = Modifier.size(130.dp).clip(CircleShape).background(Color(0xFF12141B)), contentAlignment = Alignment.Center) {
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        Icon(imageVector = if (isConnected) androidx.compose.material.icons.Icons.Default.CheckCircle else androidx.compose.material.icons.Icons.Default.Lock, contentDescription = null, tint = if (isConnected) Color(0xFF10B981) else Color(0xFF8B5CF6), modifier = Modifier.size(36.dp))
                        Spacer(modifier = Modifier.height(8.dp))
                        Text(if (isConnected) "PROTECTED" else "UNPROTECTED", color = if (isConnected) Color(0xFF10B981) else Color(0xFFA78BFA), fontSize = 12.sp, fontWeight = FontWeight.Bold, letterSpacing = 1.5.sp)
                    }
                }
            }
            Spacer(modifier = Modifier.height(20.dp))
            Button(
                onClick = { if (isConnected) onDisconnectRequest() else onConnectRequest() },
                modifier = Modifier.height(44.dp).width(140.dp),
                colors = ButtonDefaults.buttonColors(containerColor = Color.Transparent), contentPadding = PaddingValues(0.dp), shape = RoundedCornerShape(22.dp)
            ) {
                Box(modifier = Modifier.fillMaxSize().background(if (isConnected) gradientConnected else gradientPrimary), contentAlignment = Alignment.Center) {
                    Text(if (isConnected) "Disconnect" else "Connect", color = Color.White, fontWeight = FontWeight.Bold, fontSize = 14.sp)
                }
            }
        }

        // Stats Row
        val mbDown = String.format("%.2f MB", downloadBytes / (1024.0 * 1024.0))
        val mbUp = String.format("%.2f MB", uploadBytes / (1024.0 * 1024.0))
        val min = sessionDurationSeconds / 60
        val sec = sessionDurationSeconds % 60
        val sessionStr = String.format("%d:%02d", min, sec)
        Row(modifier = Modifier.fillMaxWidth().padding(bottom = 24.dp), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            listOf(Pair("Download", mbDown), Pair("Upload", mbUp), Pair("Session", sessionStr)).forEach { (label, value) ->
                Column(modifier = Modifier.weight(1f).clip(RoundedCornerShape(14.dp)).background(cardDark).padding(vertical = 12.dp), horizontalAlignment = Alignment.CenterHorizontally) {
                    Text(value, color = textPrimary, fontWeight = FontWeight.Bold, fontSize = 16.sp)
                    Spacer(modifier = Modifier.height(4.dp))
                    Text(label.uppercase(), color = Color(0xFF6B7280), fontSize = 10.sp, fontWeight = FontWeight.Bold, letterSpacing = 1.sp)
                }
            }
        }

        // Connection Mode
        Text("CONNECTION MODE", color = Color(0xFF6B7280), fontSize = 12.sp, fontWeight = FontWeight.Bold, letterSpacing = 1.sp, modifier = Modifier.padding(bottom = 12.dp))
        Row(modifier = Modifier.fillMaxWidth().padding(bottom = 24.dp), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            Box(modifier = Modifier.weight(1f).clip(RoundedCornerShape(10.dp)).background(if (routingMode == "mesh") Color(0xFF1E1B4B) else cardDark).clickable(enabled = !isConnected) { onRoutingModeChanged("mesh") }.padding(vertical = 12.dp), contentAlignment = Alignment.Center) {
                Text("Mesh", color = if (routingMode == "mesh") Color(0xFFA78BFA) else textSecondary, fontWeight = if (routingMode == "mesh") FontWeight.Bold else FontWeight.Medium, fontSize = 14.sp)
            }
            Box(modifier = Modifier.weight(1f).clip(RoundedCornerShape(10.dp)).background(if (routingMode == "exit_node") Color(0xFF1E1B4B) else cardDark).clickable(enabled = !isConnected) { onRoutingModeChanged("exit_node") }.padding(vertical = 12.dp), contentAlignment = Alignment.Center) {
                Text("Exit Node", color = if (routingMode == "exit_node") Color(0xFFA78BFA) else textSecondary, fontWeight = if (routingMode == "exit_node") FontWeight.Bold else FontWeight.Medium, fontSize = 14.sp)
            }
        }
        
        if (routingMode == "exit_node") {
            Text("CURRENT SERVER", color = Color(0xFF6B7280), fontSize = 12.sp, fontWeight = FontWeight.Bold, letterSpacing = 1.sp, modifier = Modifier.padding(bottom = 12.dp))
            Row(
                modifier = Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp)).background(cardDark).clickable(enabled = !isConnected) { onNavigateToServers() }.padding(16.dp),
                verticalAlignment = Alignment.CenterVertically
            ) {
                Icon(imageVector = androidx.compose.material.icons.Icons.Default.LocationOn, contentDescription = null, tint = textSecondary, modifier = Modifier.size(24.dp))
                Spacer(modifier = Modifier.width(12.dp))
                Text(selectedExitLabel.ifEmpty { "Select a server..." }, color = textPrimary, fontWeight = FontWeight.Bold, fontSize = 14.sp, modifier = Modifier.weight(1f))
                Icon(imageVector = androidx.compose.material.icons.Icons.Default.ArrowDropDown, contentDescription = null, tint = textSecondary)
            }
        }
    }
}

@Composable
fun ServersScreen(
    exitOptions: List<ExitNodeOption>, selectedExitId: String, isConnected: Boolean,
    onExitSelected: (String, String) -> Unit,
    cardDark: Color, textPrimary: Color, textSecondary: Color
) {
    Column(modifier = Modifier.fillMaxSize().padding(horizontal = 20.dp, vertical = 24.dp)) {
        Text("SELECT SERVER", color = Color(0xFF6B7280), fontSize = 12.sp, fontWeight = FontWeight.Bold, letterSpacing = 1.sp, modifier = Modifier.padding(bottom = 16.dp))
        androidx.compose.foundation.lazy.LazyColumn(verticalArrangement = Arrangement.spacedBy(8.dp)) {
            items(exitOptions.size) { index ->
                val node = exitOptions[index]
                val isSelected = selectedExitId == node.id
                val flag = when(node.country.uppercase()) {
                    "IN" -> "IN"
                    "AE" -> "AE"
                    "US" -> "US"
                    "DE" -> "DE"
                    else -> node.country.take(2).uppercase()
                }
                Row(
                    modifier = Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp)).background(if (isSelected) Color(0xFF1A1B2E) else cardDark).clickable(enabled = !isConnected) { onExitSelected(node.id, node.label) }.padding(16.dp),
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Box(modifier = Modifier.size(36.dp).clip(RoundedCornerShape(8.dp)).background(Color(0xFF0F172A)), contentAlignment = Alignment.Center) {
                        Text(flag, fontSize = 14.sp, color = textPrimary)
                    }
                    Spacer(modifier = Modifier.width(12.dp))
                    Column(modifier = Modifier.weight(1f)) {
                        Text(node.label, color = textPrimary, fontWeight = FontWeight.Bold, fontSize = 14.sp)
                        Text(node.id, color = textSecondary, fontSize = 12.sp, maxLines = 1)
                    }
                    if (isSelected) {
                        Box(modifier = Modifier.clip(RoundedCornerShape(6.dp)).background(Color(0xFF052E16)).padding(horizontal = 8.dp, vertical = 4.dp)) {
                            Text("Sel", color = Color(0xFF34D399), fontSize = 11.sp, fontWeight = FontWeight.Bold)
                        }
                    }
                }
            }
        }
        if (exitOptions.isEmpty()) {
            Text("No exit nodes available", color = textSecondary, fontSize = 14.sp, modifier = Modifier.padding(vertical = 16.dp))
        }
    }
}

@Composable
fun StatsScreen(
    sessionDurationSeconds: Long, downloadBytes: Long, uploadBytes: Long,
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

    Column(modifier = Modifier.fillMaxSize().padding(horizontal = 20.dp, vertical = 24.dp)) {
        Text("CONNECTION STATS", color = Color(0xFF6B7280), fontSize = 12.sp, fontWeight = FontWeight.Bold, letterSpacing = 1.sp, modifier = Modifier.padding(bottom = 16.dp))
        
        Card(modifier = Modifier.fillMaxWidth(), colors = CardDefaults.cardColors(containerColor = cardDark)) {
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
        
        Spacer(modifier = Modifier.height(24.dp))
        Text("SYSTEM LOGS", color = Color(0xFF6B7280), fontSize = 12.sp, fontWeight = FontWeight.Bold, letterSpacing = 1.sp, modifier = Modifier.padding(bottom = 16.dp))
        
        Box(modifier = Modifier.fillMaxWidth().weight(1f).clip(RoundedCornerShape(14.dp)).background(Color.Black).padding(16.dp).verticalScroll(rememberScrollState())) {
            Text(logs, color = Color(0xFF34D399), fontSize = 10.sp, fontFamily = androidx.compose.ui.text.font.FontFamily.Monospace)
        }
    }
}

@Composable
fun ProfileScreen(
    userName: String, userInitial: String, themeMode: Int, onThemeModeChanged: (Int) -> Unit,
    shareAsExit: Boolean, onShareAsExitChanged: (Boolean) -> Unit,
    shareCountry: String, onShareCountryChanged: (String) -> Unit,
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
                Text("Premium Member", color = Color(0xFF6B7280), fontSize = 14.sp)
            }
        }
        
        Text("APPEARANCE", color = Color(0xFF6B7280), fontSize = 12.sp, fontWeight = FontWeight.Bold, letterSpacing = 1.sp, modifier = Modifier.padding(bottom = 12.dp))
        Card(modifier = Modifier.fillMaxWidth(), colors = CardDefaults.cardColors(containerColor = cardDark)) {
            Column(modifier = Modifier.padding(16.dp)) {
                Text("Theme", color = textPrimary, fontWeight = FontWeight.SemiBold)
                Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    FilterChip(selected = themeMode == 0, onClick = { onThemeModeChanged(0) }, label = { Text("System") })
                    FilterChip(selected = themeMode == 1, onClick = { onThemeModeChanged(1) }, label = { Text("Light") })
                    FilterChip(selected = themeMode == 2, onClick = { onThemeModeChanged(2) }, label = { Text("Dark") })
                }
            }
        }
        
        Spacer(modifier = Modifier.height(24.dp))
        Text("NETWORK SETTINGS", color = Color(0xFF6B7280), fontSize = 12.sp, fontWeight = FontWeight.Bold, letterSpacing = 1.sp, modifier = Modifier.padding(bottom = 12.dp))
        Card(modifier = Modifier.fillMaxWidth(), colors = CardDefaults.cardColors(containerColor = cardDark)) {
            Column(modifier = Modifier.padding(16.dp)) {
                Text("Share as exit node", color = textPrimary, fontWeight = FontWeight.SemiBold)
                Text("Others can use this phone's IP", fontSize = 12.sp, color = textSecondary)
                Switch(checked = shareAsExit, onCheckedChange = onShareAsExitChanged, enabled = !isConnected)
                if (shareAsExit) {
                    Row {
                        FilterChip(selected = shareCountry == "IN", onClick = { onShareCountryChanged("IN") }, label = { Text("IN") }, enabled = !isConnected)
                        Spacer(modifier = Modifier.width(8.dp))
                        FilterChip(selected = shareCountry == "AE", onClick = { onShareCountryChanged("AE") }, label = { Text("AE") }, enabled = !isConnected)
                    }
                }
            }
        }
        
        Spacer(modifier = Modifier.height(24.dp))
        Text("ADVANCED SETTINGS", color = Color(0xFF6B7280), fontSize = 12.sp, fontWeight = FontWeight.Bold, letterSpacing = 1.sp, modifier = Modifier.padding(bottom = 12.dp))
        Card(modifier = Modifier.fillMaxWidth(), colors = CardDefaults.cardColors(containerColor = cardDark)) {
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
"""

start_idx = content.find('@OptIn(ExperimentalMaterial3Api::class)\n@Composable\nfun DashboardScreen(')
if start_idx != -1:
    content = content[:start_idx] + new_dashboard
    with open('C:\\Users\\PC\\Desktop\\Mscale\\Mscale\\android\\app\\app\\src\\main\\java\\com\\mscale\\app\\MainActivity.kt', 'w', encoding='utf-8') as f:
        f.write(content)
    print("Replaced successfully")
else:
    print("Could not find DashboardScreen in MainActivity.kt")
