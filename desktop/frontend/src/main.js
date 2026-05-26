let isConnected = false;

window.toggleConnection = function () {
    const btn = document.getElementById('toggle-btn');
    const email = document.getElementById('email').value;
    const password = document.getElementById('password').value;
    const statusText = document.getElementById('status-text');
    const statusDot = document.getElementById('status-dot');
    const ipDisplay = document.getElementById('ip-display');
    const mode = document.getElementById('routing-mode').value;

    if (!email || !password) {
        if (!isConnected) {
            alert("Please enter your email and password first!");
            return;
        }
    }

    if (!isConnected) {
        // Connecting...
        btn.innerText = "Connecting...";
        
        // Call the Go function Login
        window.go.main.App.Login(email, password).then(loginResult => {
            if (loginResult && loginResult.startsWith("Error")) {
                alert(loginResult);
                btn.innerText = "Connect";
                return;
            }
            
            // If login successful, connect the tunnel
            window.go.main.App.ConnectTunnel(mode).then(result => {
                if (result && result.startsWith("Error")) {
                    alert(result);
                    btn.innerText = "Connect";
                } else {
                    isConnected = true;
                    btn.innerText = "Disconnect";
                    btn.classList.add("disconnect");
                    statusText.innerText = mode === "exit-node" ? "Connected (Exit Node Active)" : "Connected (Mesh Only)";
                    statusDot.classList.add("active");
                    ipDisplay.innerText = result; // Shows the IP and Mode
                }
            }).catch(err => {
                alert("Connect error: " + err);
                btn.innerText = "Connect";
            });
        }).catch(err => {
            alert("Login error: " + err);
            btn.innerText = "Connect";
        });
    } else {
        // Disconnecting...
        window.go.main.App.DisconnectTunnel().then(() => {
            isConnected = false;
            btn.innerText = "Connect";
            btn.classList.remove("disconnect");
            statusText.innerText = "Disconnected";
            statusDot.classList.remove("active");
            ipDisplay.innerText = "";
        });
    }
};