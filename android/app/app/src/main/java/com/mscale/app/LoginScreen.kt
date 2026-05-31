package com.mscale.app

import android.content.Intent
import android.net.Uri
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.material.icons.Icons
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONObject
import java.io.OutputStreamWriter
import java.net.HttpURLConnection
import java.net.URL
import java.util.UUID

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun LoginScreen(modifier: Modifier = Modifier, onLoginSuccess: (String, String, String) -> Unit) {
    val context = LocalContext.current
    val coroutineScope = rememberCoroutineScope()
    var isPolling by remember { mutableStateOf(false) }
    var isEmailLoading by remember { mutableStateOf(false) }
    var errorText by remember { mutableStateOf("") }
    
    var email by remember { mutableStateOf(WakePrefs.getRememberedEmail(context)) }
    var password by remember { mutableStateOf("") }
    var rememberEmail by remember { mutableStateOf(WakePrefs.getRememberedEmail(context).isNotEmpty()) }
    var passwordVisible by remember { mutableStateOf(false) }

    val bgColor = MaterialTheme.colorScheme.background
    val textColor = MaterialTheme.colorScheme.onBackground
    val textSecondary = MaterialTheme.colorScheme.onSurfaceVariant
    val dividerColor = MaterialTheme.colorScheme.outlineVariant
    val surfaceColor = MaterialTheme.colorScheme.surface


    Column(
        modifier = modifier
            .fillMaxSize()
            .background(bgColor)
            .padding(32.dp)
            .verticalScroll(rememberScrollState()),
        horizontalAlignment = Alignment.Start,
        verticalArrangement = Arrangement.Top
    ) {
        Spacer(modifier = Modifier.height(24.dp))
        
        // Logo
        Image(
            painter = painterResource(id = R.drawable.logo),
            contentDescription = "Mscale Logo",
            modifier = Modifier.size(64.dp)
        )
        
        Spacer(modifier = Modifier.height(16.dp))
        
        Text(
            text = "Sign in to Mscale",
            fontSize = 28.sp,
            fontWeight = FontWeight.Bold,
            color = textColor
        )
        
        Spacer(modifier = Modifier.height(8.dp))
        
        Text(
            text = "Enter your details to access your mesh network.",
            fontSize = 14.sp,
            color = textSecondary
        )
        
        Spacer(modifier = Modifier.height(32.dp))
        
        // Or use password Divider
        Row(
            verticalAlignment = Alignment.CenterVertically,
            modifier = Modifier.fillMaxWidth()
        ) {
            Divider(modifier = Modifier.weight(1f), color = dividerColor)
            Text(
                text = "Or use password",
                color = textSecondary,
                fontSize = 12.sp,
                modifier = Modifier.padding(horizontal = 8.dp)
            )
            Divider(modifier = Modifier.weight(1f), color = dividerColor)
        }

        Spacer(modifier = Modifier.height(24.dp))

        // Email Field
        Text(text = "EMAIL ADDRESS", fontSize = 12.sp, fontWeight = FontWeight.Bold, color = textSecondary)
        Spacer(modifier = Modifier.height(8.dp))
        OutlinedTextField(
            value = email,
            onValueChange = { email = it },
            placeholder = { Text("you@example.com", color = textSecondary) },
            modifier = Modifier.fillMaxWidth(),
            shape = RoundedCornerShape(12.dp),
            singleLine = true,
            colors = OutlinedTextFieldDefaults.colors(
                unfocusedContainerColor = surfaceColor,
                focusedContainerColor = surfaceColor,
                unfocusedBorderColor = dividerColor,
                focusedBorderColor = Color(0xFF3B82F6)
            )
        )

        Spacer(modifier = Modifier.height(16.dp))

        // Password Field
        Text(text = "PASSWORD", fontSize = 12.sp, fontWeight = FontWeight.Bold, color = textSecondary)
        Spacer(modifier = Modifier.height(8.dp))
        OutlinedTextField(
            value = password,
            onValueChange = { password = it },
            placeholder = { Text("********", color = textSecondary) },
            modifier = Modifier.fillMaxWidth(),
            shape = RoundedCornerShape(12.dp),
            singleLine = true,
            visualTransformation = if (passwordVisible) VisualTransformation.None else PasswordVisualTransformation(),
            trailingIcon = {
                TextButton(onClick = { passwordVisible = !passwordVisible }) {
                    Text(if (passwordVisible) "Hide" else "Show", color = textSecondary)
                }
            },
            colors = OutlinedTextFieldDefaults.colors(
                unfocusedContainerColor = surfaceColor,
                focusedContainerColor = surfaceColor,
                unfocusedBorderColor = dividerColor,
                focusedBorderColor = Color(0xFF3B82F6)
            )
        )

        Spacer(modifier = Modifier.height(16.dp))

        // Remember Me & Forgot Password
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically
        ) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Checkbox(
                    checked = rememberEmail,
                    onCheckedChange = { rememberEmail = it },
                    colors = CheckboxDefaults.colors(checkedColor = Color(0xFF3B82F6), uncheckedColor = textSecondary)
                )
                Text(text = "Remember email", fontSize = 14.sp, color = textSecondary)
            }
            Text(
                text = "Forgot password?",
                fontSize = 14.sp,
                color = Color(0xFF3B82F6),
                modifier = Modifier.clickable { /* Handle forgot password */ }
            )
        }

        Spacer(modifier = Modifier.height(16.dp))

        if (errorText.isNotEmpty()) {
            Text(text = errorText, color = Color.Red, fontSize = 14.sp, modifier = Modifier.padding(bottom = 8.dp))
        }

        // Sign In Button
        Button(
            onClick = {
                if (!isEmailLoading && email.isNotEmpty() && password.isNotEmpty()) {
                    isEmailLoading = true
                    errorText = ""
                    coroutineScope.launch {
                        loginWithEmail(email, password, { token, userName, userEmail ->
                            if (rememberEmail) WakePrefs.setRememberedEmail(context, userEmail.ifEmpty { email.trim() })
                            else WakePrefs.setRememberedEmail(context, "")
                            onLoginSuccess(token, userName, userEmail)
                            isEmailLoading = false
                        }) { err ->
                            errorText = err
                            isEmailLoading = false
                        }
                    }
                }
            },
            modifier = Modifier.fillMaxWidth().height(56.dp),
            shape = RoundedCornerShape(12.dp),
            colors = ButtonDefaults.buttonColors(containerColor = Color(0xFF3B82F6)), // Desktop Blue
            enabled = !isEmailLoading
        ) {
            Text(text = if (isEmailLoading) "Signing in..." else "Sign in ->", fontSize = 16.sp, fontWeight = FontWeight.Bold)
        }

        Spacer(modifier = Modifier.height(32.dp))

        // Or continue with Divider
        Row(
            verticalAlignment = Alignment.CenterVertically,
            modifier = Modifier.fillMaxWidth()
        ) {
            Divider(modifier = Modifier.weight(1f), color = dividerColor)
            Text(
                text = "Or continue with",
                color = textSecondary,
                fontSize = 12.sp,
                modifier = Modifier.padding(horizontal = 8.dp)
            )
            Divider(modifier = Modifier.weight(1f), color = dividerColor)
        }

        Spacer(modifier = Modifier.height(24.dp))

        // Social Buttons Row
        Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(16.dp)) {
            OutlinedButton(
                onClick = {
                    if (!isPolling) {
                        val sessionId = UUID.randomUUID().toString()
                        val authUrl = "https://mashad.shop/mscale/api/auth/google/login?type=desktop&session=$sessionId"
                        val browserIntent = Intent(Intent.ACTION_VIEW, Uri.parse(authUrl))
                        context.startActivity(browserIntent)
                        
                        isPolling = true
                        errorText = "Waiting for Google Auth in browser..."
                        
                        coroutineScope.launch {
                            pollAuthentication(sessionId, onLoginSuccess) { err ->
                                errorText = err
                                isPolling = false
                            }
                        }
                    }
                },
                modifier = Modifier.weight(1f).height(48.dp),
                shape = RoundedCornerShape(12.dp),
                border = androidx.compose.foundation.BorderStroke(1.dp, dividerColor),
                colors = ButtonDefaults.outlinedButtonColors(containerColor = surfaceColor),
                enabled = !isPolling
            ) {
                Text(text = "G Google", color = textColor, fontWeight = FontWeight.SemiBold)
            }

            OutlinedButton(
                onClick = { errorText = "GitHub auth coming soon" },
                modifier = Modifier.weight(1f).height(48.dp),
                shape = RoundedCornerShape(12.dp),
                colors = ButtonDefaults.outlinedButtonColors(containerColor = surfaceColor),
                border = androidx.compose.foundation.BorderStroke(1.dp, dividerColor)
            ) {
                Text(text = "GitHub", color = textColor, fontWeight = FontWeight.SemiBold)
            }
        }
    }
}

suspend fun loginWithEmail(
    email: String,
    pass: String,
    onSuccess: (String, String, String) -> Unit,
    onError: (String) -> Unit
) {
    withContext(Dispatchers.IO) {
        try {
            val trimmedEmail = email.trim()
            val url = URL("https://mashad.shop/mscale/api/auth/login")
            val conn = url.openConnection() as HttpURLConnection
            conn.requestMethod = "POST"
            conn.setRequestProperty("Content-Type", "application/json")
            conn.doOutput = true
            conn.connectTimeout = 5000

            val jsonParam = JSONObject()
            jsonParam.put("email", trimmedEmail)
            jsonParam.put("password", pass)

            val os = OutputStreamWriter(conn.outputStream)
            os.write(jsonParam.toString())
            os.flush()
            os.close()

            if (conn.responseCode == 200) {
                // Try to extract the session cookie
                var token = "dummy_token" // Fallback
                val headerFields = conn.headerFields
                val cookiesHeader = headerFields["Set-Cookie"]
                if (cookiesHeader != null) {
                    for (cookie in cookiesHeader) {
                        if (cookie.startsWith("mscale_session=")) {
                            val parts = cookie.split(";")[0].split("=")
                            if (parts.size > 1) {
                                token = parts[1]
                                break
                            }
                        }
                    }
                }
                
                var userName = trimmedEmail.substringBefore("@").replaceFirstChar { if (it.isLowerCase()) it.titlecase() else it.toString() }.ifEmpty { "User" }
                var userEmail = trimmedEmail
                try {
                    val responseBody = conn.inputStream.bufferedReader().use { it.readText() }
                    val jsonResponse = JSONObject(responseBody)
                    val userObj = jsonResponse.optJSONObject("user")
                    if (userObj != null) {
                        val name = userObj.optString("name")
                        val dName = userObj.optString("display_name")
                        val uEmail = userObj.optString("email")
                        if (uEmail.isNotEmpty() && uEmail != "null") userEmail = uEmail
                        if (name.isNotEmpty() && name != "null") userName = name
                        else if (dName.isNotEmpty() && dName != "null") userName = dName
                        else if (userEmail.isNotEmpty()) userName = userEmail.substringBefore("@").replaceFirstChar { if (it.isLowerCase()) it.titlecase() else it.toString() }
                    } else {
                        val name = jsonResponse.optString("name")
                        val dName = jsonResponse.optString("display_name")
                        val uEmail = jsonResponse.optString("email")
                        if (uEmail.isNotEmpty() && uEmail != "null") userEmail = uEmail
                        if (name.isNotEmpty() && name != "null") userName = name
                        else if (dName.isNotEmpty() && dName != "null") userName = dName
                        else if (userEmail.isNotEmpty()) userName = userEmail.substringBefore("@").replaceFirstChar { if (it.isLowerCase()) it.titlecase() else it.toString() }
                    }
                } catch (e: Exception) {
                    // Ignore JSON parsing errors
                }

                withContext(Dispatchers.Main) {
                    onSuccess(token, userName, userEmail)
                }
            } else {
                withContext(Dispatchers.Main) {
                    onError("Invalid email or password")
                }
            }
            conn.disconnect()
        } catch (e: Exception) {
            withContext(Dispatchers.Main) {
                onError("Network error: ${e.message}")
            }
        }
    }
}

suspend fun pollAuthentication(sessionId: String, onSuccess: (String, String, String) -> Unit, onError: (String) -> Unit) {
    withContext(Dispatchers.IO) {
        for (i in 0 until 45) { // Poll for ~90 seconds
            try {
                delay(2000)
                val url = URL("https://mashad.shop/mscale/api/auth/google/status?session=$sessionId")
                val conn = url.openConnection() as HttpURLConnection
                conn.requestMethod = "GET"
                conn.connectTimeout = 5000
                
                if (conn.responseCode == 200) {
                    val response = conn.inputStream.bufferedReader().use { it.readText() }
                    val json = JSONObject(response)
                    
                    if (json.has("status") && json.getString("status") == "success") {
                        val token = json.getString("token")
                        var userName = "User"
                        var userEmail = ""
                        val userObj = json.optJSONObject("user")
                        if (userObj != null) {
                            val name = userObj.optString("name")
                            val dName = userObj.optString("display_name")
                            val uEmail = userObj.optString("email")
                            if (uEmail.isNotEmpty() && uEmail != "null") userEmail = uEmail
                            if (name.isNotEmpty() && name != "null") userName = name
                            else if (dName.isNotEmpty() && dName != "null") userName = dName
                            else if (userEmail.isNotEmpty()) userName = userEmail.substringBefore("@").replaceFirstChar { if (it.isLowerCase()) it.titlecase() else it.toString() }
                        } else {
                            val name = json.optString("name")
                            val dName = json.optString("display_name")
                            val uEmail = json.optString("email")
                            if (uEmail.isNotEmpty() && uEmail != "null") userEmail = uEmail
                            if (name.isNotEmpty() && name != "null") userName = name
                            else if (dName.isNotEmpty() && dName != "null") userName = dName
                            else if (userEmail.isNotEmpty()) userName = userEmail.substringBefore("@").replaceFirstChar { if (it.isLowerCase()) it.titlecase() else it.toString() }
                        }
                        withContext(Dispatchers.Main) {
                            onSuccess(token, userName, userEmail)
                        }
                        return@withContext
                    }
                }
                conn.disconnect()
            } catch (e: Exception) {
                // Ignore transient errors and keep polling
            }
        }
        
        withContext(Dispatchers.Main) {
            onError("Authentication timed out. Try again.")
        }
    }
}
