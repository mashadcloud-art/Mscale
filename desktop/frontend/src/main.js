let isConnected = false;

window.toggleConnection = function () {
    const btn = document.getElementById('toggle-btn');
    const token = document.getElementById('token').value;
    const statusText = document.getElementById('status-text');
    const statusDot = document.getElementById('status-dot');
    const ipDisplay = document.getElementById('ip-display');
    const mode = document.getElementById('routing-mode').value;

    if (!token && !isConnected) {
        alert("Please enter your token first!");
        return;
    }

    if (!isConnected) {
        // Connecting...
        btn.innerText = "Connecting...";
        
        // Call the Go function ConnectTunnel
        window.go.main.App.ConnectTunnel(token, mode).then(result => {
            if (result.startsWith("Error")) {
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