let isConnected = false;

window.loginUser = function () {
    const email = document.getElementById('email').value;
    const password = document.getElementById('password').value;
    const btn = document.querySelector('#login-section button');

    if (!email || !password) {
        alert("Please enter both email and password.");
        return;
    }

    btn.innerText = "Logging in...";
    window.go.main.App.Login(email, password).then(result => {
        if (result === "Success") {
            document.getElementById('login-section').style.display = 'none';
            document.getElementById('vpn-section').style.display = 'flex';
        } else {
            alert(result);
            btn.innerText = "Login";
        }
    });
};

window.toggleConnection = function () {
    const btn = document.getElementById('toggle-btn');
    const statusText = document.getElementById('status-text');
    const statusDot = document.getElementById('status-dot');
    const ipDisplay = document.getElementById('ip-display');
    const mode = document.getElementById('routing-mode').value;

    if (!isConnected) {
        btn.innerText = "Connecting...";
        
        window.go.main.App.ConnectTunnel(mode).then(result => {
            if (result.startsWith("Error")) {
                alert(result);
                btn.innerText = "Connect";
            } else {
                isConnected = true;
                btn.innerText = "Disconnect";
                btn.classList.add("disconnect");
                statusText.innerText = mode === "exit-node" ? "Connected (Exit Node Active)" : "Connected (Mesh Only)";
                statusDot.classList.add("active");
                ipDisplay.innerText = result;
            }
        });
    } else {
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