import re

with open('C:\\Users\\PC\\Desktop\\Mscale\\Mscale\\android\\app\\app\\src\\main\\java\\com\\mscale\\app\\MainActivity.kt', 'r', encoding='utf-8') as f:
    content = f.read()

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
    var showSettings by remember { mutableStateOf(false) }
    var routingMode by remember { mutableStateOf("mesh") } // "mesh" or "exit_node"
    var selectedExitLabel by remember { mutableStateOf("") }
    var expanded by remember { mutableStateOf(false) }
    val exitOptions = remember { mutableStateListOf<ExitNodeOption>() }
    
    val ctx = LocalContext.current
    val userName = remember { WakePrefs.getUserName(ctx) }
    
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
                    nodes.add(ExitNodeOption(id, label))
                    if (country.equals("IN", true) || country.contains("India", true)) {
                        indiaId = id
                        indiaLabel = label
                    }
                }
                kotlinx.coroutines.withContext(kotlinx.coroutines.Dispatchers.Main) {
                    exitOptions.clear()
                    exitOptions.addAll(nodes)
                    if (indiaId.isNotEmpty()) {
                        selectedExitLabel = indiaLabel
                        onExitNodeSelected(indiaId)
                    }
                }
            } catch (_: Exception) { }
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { 
                    Column {
                        Text(userName, fontWeight = FontWeight.Bold, fontSize = 20.sp)
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Text(
                                text = if (isConnected) "Connected" else "Unprotected",
                                fontSize = 14.sp,
                                color = if (isConnected) Color(0xFF10B981) else MaterialTheme.colorScheme.error,
                                fontWeight = FontWeight.SemiBold
                            )
                            Spacer(modifier = Modifier.width(8.dp))
                            Switch(
                                checked = isConnected,
                                onCheckedChange = { 
                                    if (it) onConnectRequest() else onDisconnectRequest() 
                                },
                                modifier = Modifier.scale(0.7f)
                            )
                        }
                    }
                },
                actions = {
                    IconButton(onClick = { showSettings = true }) {
                        Icon(imageVector = androidx.compose.material.icons.Icons.Default.Settings, contentDescription = "Settings")
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(containerColor = Color.Transparent)
            )
        },
        containerColor = MaterialTheme.colorScheme.surface
    ) { paddingValues ->
        Column(
            modifier = modifier
                .fillMaxSize()
                .padding(paddingValues)
                .padding(24.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center
        ) {
            
            // Routing Mode Selection
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceEvenly
            ) {
                OutlinedButton(
                    onClick = { 
                        routingMode = "mesh"
                        onExitNodeSelected("")
                    },
                    colors = ButtonDefaults.outlinedButtonColors(
                        containerColor = if (routingMode == "mesh") MaterialTheme.colorScheme.primaryContainer else Color.Transparent
                    ),
                    modifier = Modifier.weight(1f).padding(end = 8.dp),
                    enabled = !isConnected
                ) {
                    Text("Mesh")
                }
                
                OutlinedButton(
                    onClick = { routingMode = "exit_node" },
                    colors = ButtonDefaults.outlinedButtonColors(
                        containerColor = if (routingMode == "exit_node") MaterialTheme.colorScheme.primaryContainer else Color.Transparent
                    ),
                    modifier = Modifier.weight(1f).padding(start = 8.dp),
                    enabled = !isConnected
                ) {
                    Text("Exit Node")
                }
            }
            
            Spacer(modifier = Modifier.height(32.dp))
            
            // Dynamic Middle Content based on State
            if (routingMode == "exit_node") {
                if (isConnected) {
                    // Connected State - Show selected machine name
                    Card(
                        modifier = Modifier.fillMaxWidth(0.9f).clickable { onDisconnectRequest() },
                        colors = CardDefaults.cardColors(containerColor = Color(0xFF10B981)),
                        shape = RoundedCornerShape(16.dp)
                    ) {
                        Column(
                            modifier = Modifier.padding(24.dp).fillMaxWidth(),
                            horizontalAlignment = Alignment.CenterHorizontally
                        ) {
                            Text("Connected to:", color = Color.White, fontSize = 14.sp)
                            Spacer(modifier = Modifier.height(4.dp))
                            Text(selectedExitLabel, color = Color.White, fontWeight = FontWeight.Bold, fontSize = 18.sp)
                            Spacer(modifier = Modifier.height(8.dp))
                            Text("Tap to Disconnect", color = Color.White.copy(alpha=0.8f), fontSize = 12.sp)
                        }
                    }
                } else {
                    // Disconnected State - Show dropdown list
                    Box {
                        OutlinedButton(
                            onClick = { expanded = true },
                            modifier = Modifier.fillMaxWidth(0.9f).height(56.dp),
                            shape = RoundedCornerShape(12.dp)
                        ) {
                            Text(if (selectedExitLabel.isEmpty()) "Select Exit Node..." else selectedExitLabel)
                        }
                        DropdownMenu(
                            expanded = expanded,
                            onDismissRequest = { expanded = false }
                        ) {
                            exitOptions.forEach { node ->
                                DropdownMenuItem(
                                    text = { Text(node.label) },
                                    onClick = {
                                        selectedExitLabel = node.label
                                        onExitNodeSelected(node.id)
                                        expanded = false
                                    }
                                )
                            }
                        }
                    }
                    
                    Spacer(modifier = Modifier.height(24.dp))
                    
                    Button(
                        onClick = { onConnectRequest() },
                        modifier = Modifier.fillMaxWidth(0.9f).height(56.dp),
                        shape = RoundedCornerShape(12.dp),
                        colors = ButtonDefaults.buttonColors(containerColor = MaterialTheme.colorScheme.primary),
                        enabled = selectedExitLabel.isNotEmpty()
                    ) {
                        Text("Connect", fontSize = 16.sp, fontWeight = FontWeight.Bold)
                    }
                }
            } else {
                // Mesh Mode
                if (isConnected) {
                    Card(
                        modifier = Modifier.fillMaxWidth(0.9f).clickable { onDisconnectRequest() },
                        colors = CardDefaults.cardColors(containerColor = Color(0xFF10B981)),
                        shape = RoundedCornerShape(16.dp)
                    ) {
                        Column(
                            modifier = Modifier.padding(24.dp).fillMaxWidth(),
                            horizontalAlignment = Alignment.CenterHorizontally
                        ) {
                            Text("Connected to Mesh", color = Color.White, fontWeight = FontWeight.Bold, fontSize = 18.sp)
                            Spacer(modifier = Modifier.height(8.dp))
                            Text("Local peer IP only", color = Color.White.copy(alpha=0.8f), fontSize = 14.sp)
                        }
                    }
                } else {
                    Button(
                        onClick = { onConnectRequest() },
                        modifier = Modifier.fillMaxWidth(0.9f).height(56.dp),
                        shape = RoundedCornerShape(12.dp),
                        colors = ButtonDefaults.buttonColors(containerColor = MaterialTheme.colorScheme.primary)
                    ) {
                        Text("Connect to Mesh", fontSize = 16.sp, fontWeight = FontWeight.Bold)
                    }
                }
            }
        }
    }
    
    if (showSettings) {
        androidx.compose.ui.window.Dialog(onDismissRequest = { showSettings = false }) {
            Card(
                modifier = Modifier.fillMaxWidth().verticalScroll(rememberScrollState()),
                shape = RoundedCornerShape(16.dp),
                colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface)
            ) {
                Column(modifier = Modifier.padding(24.dp)) {
                    Text("Settings", fontWeight = FontWeight.Bold, fontSize = 20.sp)
                    Spacer(modifier = Modifier.height(24.dp))
                    
                    // Share as Exit Node
                    Text("Share as exit node", fontWeight = FontWeight.SemiBold)
                    Text("Others can use this phone's IP", fontSize = 12.sp, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    Switch(checked = shareAsExit, onCheckedChange = onShareAsExitChanged, enabled = !isConnected)
                    
                    if (shareAsExit) {
                        Row {
                            FilterChip(selected = shareCountry == "IN", onClick = { onShareCountryChanged("IN") }, label = { Text("IN") }, enabled = !isConnected)
                            Spacer(modifier = Modifier.width(8.dp))
                            FilterChip(selected = shareCountry == "AE", onClick = { onShareCountryChanged("AE") }, label = { Text("AE") }, enabled = !isConnected)
                        }
                    }
                    
                    Divider(modifier = Modifier.padding(vertical = 16.dp))
                    
                    // Wake on Call
                    Text("Wake on Call", fontWeight = FontWeight.SemiBold)
                    Text("Allow remote wake", fontSize = 12.sp, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    Switch(checked = wakeRemoteEnabled, onCheckedChange = onWakeRemoteChanged)
                    
                    if (wakeRemoteEnabled) {
                        OutlinedTextField(
                            value = deviceMyPhone,
                            onValueChange = onDeviceMyPhoneChanged,
                            label = { Text("This phone's number") },
                            singleLine = true,
                            modifier = Modifier.fillMaxWidth()
                        )
                    }
                    
                    Spacer(modifier = Modifier.height(32.dp))
                    
                    Button(
                        onClick = { 
                            showSettings = false
                            onLogoutRequest() 
                        },
                        modifier = Modifier.fillMaxWidth(),
                        colors = ButtonDefaults.buttonColors(containerColor = MaterialTheme.colorScheme.error)
                    ) {
                        Text("Logout")
                    }
                    
                    Spacer(modifier = Modifier.height(8.dp))
                    
                    OutlinedButton(onClick = { showSettings = false }, modifier = Modifier.fillMaxWidth()) {
                        Text("Close")
                    }
                }
            }
        }
    }
}

data class ExitNodeOption(val id: String, val label: String)
"""

start_idx = content.find('@OptIn(ExperimentalMaterial3Api::class)\n@Composable\nfun DashboardScreen(')
if start_idx != -1:
    content = content[:start_idx] + new_dashboard
    with open('C:\\Users\\PC\\Desktop\\Mscale\\Mscale\\android\\app\\app\\src\\main\\java\\com\\mscale\\app\\MainActivity.kt', 'w', encoding='utf-8') as f:
        f.write(content)
    print("Replaced successfully")
else:
    print("Could not find DashboardScreen in MainActivity.kt")
