let isConnected = false;

window.loginUser = function () {
    const email = document.getElementById('email').value;
    const password = document.getElementById('password').value;
    const btn = document.querySelector('#login-section button');
    const statusText = document.getElementById('status-text');
    const statusDot = document.getElementById('status-dot');
    const ipDisplay = document.getElementById('ip-display');

    if (!email || !password) {
        alert("Please enter both email and password.");
        return;
    }

    btn.innerText = "Logging in...";

    window.go.main.App.Login(email, password).then(result => {
        if (result.startsWith("Success")) {
            document.getElementById('login-section').style.display = 'none';
            document.getElementById('vpn-section').style.display = 'flex';

            statusText.innerText = result.replace("Success: ", "");
            statusDot.classList.remove("active");
            ipDisplay.innerText = "";
        } else {
            alert(result);
            btn.innerText = "Login";
        }
    }).catch(err => {
        alert("Login failed: " + err);
        btn.innerText = "Login";
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
                statusDot.classList.add("active");
                ipDisplay.innerText = result;

                window.go.main.App.GetStatus().then(status => {
                    statusText.innerText = status;
                }).catch(() => {
                    statusText.innerText = mode === "exit-node"
                        ? "Connected (Exit Node Active)"
                        : "Connected (Mesh Only)";
                });
            }
        }).catch(err => {
            alert("Connection failed: " + err);
            btn.innerText = "Connect";
        });
    } else {
        window.go.main.App.DisconnectTunnel().then(() => {
            isConnected = false;
            btn.innerText = "Connect";
            btn.classList.remove("disconnect");
            statusText.innerText = "Disconnected";
            statusDot.classList.remove("active");
            ipDisplay.innerText = "";
        }).catch(err => {
            alert("Disconnect failed: " + err);
        });
    }
};