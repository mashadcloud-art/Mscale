import { WindowMinimise, Quit, BrowserOpenURL } from '../wailsjs/runtime/runtime.js';

const ADMIN_WEB_URL = 'https://mashad.shop/mscale/';

let isConnected = false;
let actionBusy = false;
let currentMode = "mesh";
let currentExitNodeID = "";

const video = document.getElementById('bg-video');
const loginView = document.getElementById('login-section');
const vpnView = document.getElementById('vpn-section');
const loginBtn = document.getElementById('login-btn');
const loginFormPanel = document.getElementById('login-form-panel');
const loginLoadingState = document.getElementById('login-loading-state');
const loginErrorBanner = document.getElementById('login-error-banner');
const loginErrorText = document.getElementById('login-error-text');
const connectBtn = document.getElementById('connect-btn');
const btnInner = document.getElementById('btn-inner');
const btnOuter = document.getElementById('btn-outer');
const statusDot = document.getElementById('status-dot');
const statusText = document.getElementById('status-text');
const networkStats = document.getElementById('network-stats');
const statIp = document.getElementById('stat-ip');

const REMEMBER_EMAIL_KEY = 'mscale_remember_email';
const SAVED_EMAIL_KEY = 'mscale_saved_email';
const SHARE_AS_EXIT_KEY = 'mscale_share_as_exit';
const EXIT_PROMPT_SHOWN_KEY = 'mscale_exit_prompt_shown';

let shareAsExit = localStorage.getItem(SHARE_AS_EXIT_KEY) === '1';

function updateExitShareUI() {
    const toggle = document.getElementById('exit-share-toggle');
    const hint = document.getElementById('exit-share-hint');
    const cardInner = document.getElementById('exit-share-card-inner');
    const routingCard = document.getElementById('routing-mode-card');
    if (toggle) toggle.checked = shareAsExit;
    if (hint) {
        if (shareAsExit && isConnected) {
            hint.textContent = 'Active — other devices can use this PC';
        } else if (shareAsExit) {
            hint.textContent = 'Connecting… keep the app open';
        } else {
            hint.textContent = 'Share this PC\'s internet with your other devices';
        }
    }
    if (cardInner) {
        cardInner.classList.toggle('ring-2', shareAsExit);
        cardInner.classList.toggle('ring-indigo-400/50', shareAsExit);
        cardInner.classList.toggle('bg-indigo-50', shareAsExit);
        cardInner.classList.toggle('dark:bg-indigo-950/30', shareAsExit);
    }
    routingCard?.classList.toggle('hidden', shareAsExit);
}

function showExitPromptIfNeeded() {
    if (localStorage.getItem(EXIT_PROMPT_SHOWN_KEY) === '1') return;
    const modal = document.getElementById('exit-prompt-modal');
    if (!modal) return;
    modal.classList.remove('hidden');
    modal.classList.add('flex');
}

window.dismissExitPrompt = function (enable) {
    localStorage.setItem(EXIT_PROMPT_SHOWN_KEY, '1');
    const modal = document.getElementById('exit-prompt-modal');
    modal?.classList.add('hidden');
    modal?.classList.remove('flex');
    if (enable) {
        window.toggleShareAsExit(true);
    }
};

window.toggleShareAsExit = async function (enabled) {
    if (actionBusy) {
        updateExitShareUI();
        return;
    }
    if (!window.go?.main?.App?.SetShareAsExit) {
        alert('Exit node requires a rebuilt desktop app.');
        updateExitShareUI();
        return;
    }

    actionBusy = true;
    setVpnActionsBusy(true);
    shareAsExit = enabled;
    localStorage.setItem(SHARE_AS_EXIT_KEY, enabled ? '1' : '0');
    updateExitShareUI();

    if (enabled) {
        currentMode = 'mesh';
        currentExitNodeID = '';
        if (statusText) statusText.innerText = 'Connecting exit node...';
    } else if (statusText) {
        statusText.innerText = 'Disconnecting...';
    }

    try {
        const result = await window.go.main.App.SetShareAsExit(enabled, '');
        if (result.startsWith('Error')) {
            alert(result);
            shareAsExit = false;
            localStorage.setItem(SHARE_AS_EXIT_KEY, '0');
            if (statusText) statusText.innerText = 'Disconnected';
            resetVpnConnectionUI();
        } else if (enabled) {
            applyConnectedUI(result.split('\n')[0].includes('Assigned IP') ? result : 'Exit node active — ' + result);
            window.go.main.App.GetStatus().then(s => {
                if (s.includes('Exit node') || s.startsWith('Connected')) applyConnectedUI(s);
            }).catch(() => {});
        } else {
            resetVpnConnectionUI();
            playVideo('/videos/login.mp4');
            if (statusText) statusText.innerText = 'Disconnected';
        }
    } catch (err) {
        alert('Exit node failed: ' + err);
        shareAsExit = false;
        localStorage.setItem(SHARE_AS_EXIT_KEY, '0');
        resetVpnConnectionUI();
    } finally {
        actionBusy = false;
        setVpnActionsBusy(false);
        updateExitShareUI();
    }
};

function maybeAutoConnectExitShare() {
    if (!shareAsExit || isConnected || actionBusy) return;
    if (!window.go?.main?.App?.SetShareAsExit) return;
    setTimeout(() => window.toggleShareAsExit(true), 600);
}

function accountInitials(name) {
    const parts = (name || 'Account').trim().split(/\s+/).filter(Boolean);
    if (parts.length >= 2) {
        return (parts[0][0] + parts[1][0]).toUpperCase();
    }
    return (parts[0] || 'A').slice(0, 2).toUpperCase();
}

function accountHue(email) {
    let hash = 0;
    for (let i = 0; i < email.length; i++) {
        hash = email.charCodeAt(i) + ((hash << 5) - hash);
    }
    const hues = [221, 248, 271, 199, 168, 328, 142];
    return hues[Math.abs(hash) % hues.length];
}

function displayNameOnly(label) {
    if (!label) return '';
    const idx = label.indexOf(' (');
    if (idx > 0) return label.slice(0, idx);
    if (label.includes('@')) return label.split('@')[0].replace(/[._-]/g, ' ');
    return label;
}

function updateLoginScrollFade() {
    const shell = document.getElementById('login-section');
    const scroller = document.getElementById('login-scroll');
    if (!shell || !scroller) return;

    const maxScroll = scroller.scrollHeight - scroller.clientHeight;
    shell.classList.toggle('can-scroll-up', scroller.scrollTop > 8);
    shell.classList.toggle('can-scroll-down', maxScroll > 8 && scroller.scrollTop < maxScroll - 8);
}

function initLoginScrollFade() {
    const scroller = document.getElementById('login-scroll');
    if (!scroller) return;
    scroller.addEventListener('scroll', updateLoginScrollFade, { passive: true });
    window.addEventListener('resize', updateLoginScrollFade);
    setTimeout(updateLoginScrollFade, 100);
}

function showLoginError(msg) {
    if (loginErrorBanner && loginErrorText) {
        loginErrorText.innerText = msg;
        loginErrorBanner.classList.remove('hidden');
    } else {
        alert(msg);
    }
}

function hideLoginError() {
    loginErrorBanner?.classList.add('hidden');
}

function playVideo(src) {
    if (video) {
        video.src = src;
        video.play().catch(() => {});
    }
}

function resetVpnConnectionUI() {
    isConnected = false;
    shareAsExit = localStorage.getItem(SHARE_AS_EXIT_KEY) === '1';
    connectBtn?.classList.remove('disconnect', 'connected');
    btnInner?.classList.remove('connected');
    btnOuter?.classList.remove('connected-ring');
    if (statusText) statusText.innerText = 'Disconnected';
    statusDot?.classList.remove('active');
    networkStats?.classList.add('opacity-30', 'h-0', 'translate-y-2', 'pointer-events-none');
    networkStats?.classList.remove('opacity-100', 'h-auto', 'translate-y-0', 'mb-6');
    if (statIp) statIp.innerText = '—';
    const statProto = document.getElementById('stat-proto');
    if (statProto) statProto.innerText = '—';
    clearExitTestUI();
    const badge = document.getElementById('status-badge');
    if (badge) {
        badge.innerText = 'Ready';
        badge.classList.remove('connected-badge');
    }
    updateVerifyExitButton();
}

function applyConnectedUI(status) {
    isConnected = true;
    connectBtn?.classList.add('disconnect', 'connected');
    btnInner?.classList.add('connected');
    btnOuter?.classList.add('connected-ring');
    statusDot?.classList.add('active');

    let displayStatus = status;
    if (status.includes('|')) {
        const parts = status.split('|');
        const userPart = displayNameOnly(parts.slice(1).join('|').trim());
        displayStatus = `${parts[0].trim()} | ${userPart}`;
    }
    if (statusText) statusText.innerText = displayStatus;

    const ipMatch = status.match(/IP:\s*([^\s|]+)/i);
    if (statIp && ipMatch) statIp.innerText = ipMatch[1];
    const statProto = document.getElementById('stat-proto');
    if (statProto) statProto.innerText = 'WireGuard';

    networkStats?.classList.remove('opacity-30', 'h-0', 'translate-y-2', 'pointer-events-none');
    networkStats?.classList.add('opacity-100', 'h-auto', 'translate-y-0', 'mb-6');

    const badge = document.getElementById('status-badge');
    if (badge) {
        badge.innerText = status.includes('Exit node') ? 'Exit node' : 'Connected';
        badge.classList.add('connected-badge');
    }

    updateExitShareUI();
    updateExitViaUI();
    updateVerifyExitButton();
    playVideo('/videos/Connected.mp4');
}

function updateExitViaUI() {
    const info = document.getElementById('exit-via-info');
    const nameEl = document.getElementById('exit-via-device');
    const activeLocation = document.getElementById('active-location');
    const usingExit = isConnected && (currentMode === 'exit-via' || currentExitNodeID);
    if (info) info.classList.toggle('hidden', !usingExit);
    if (nameEl && usingExit) {
        const label = activeLocation?.innerText?.trim();
        nameEl.innerText = label && label !== 'Mesh Network Only' ? label : 'Selected exit device';
    }
}

function clearExitTestUI() {
    const panel = document.getElementById('exit-test-result');
    if (panel) panel.classList.add('hidden');
    const info = document.getElementById('exit-via-info');
    if (info) info.classList.add('hidden');
}

function showExitTestResult(ok, title, detail) {
    const panel = document.getElementById('exit-test-result');
    const titleEl = document.getElementById('exit-test-title');
    const detailEl = document.getElementById('exit-test-detail');
    if (!panel || !titleEl || !detailEl) return;
    panel.classList.remove('hidden');
    panel.classList.toggle('bg-emerald-50', ok);
    panel.classList.toggle('dark:bg-emerald-950/30', ok);
    panel.classList.toggle('border-emerald-200', ok);
    panel.classList.toggle('dark:border-emerald-900/40', ok);
    panel.classList.toggle('bg-red-50', !ok);
    panel.classList.toggle('dark:bg-red-950/30', !ok);
    panel.classList.toggle('border-red-200', !ok);
    panel.classList.toggle('dark:border-red-900/40', !ok);
    titleEl.className = 'text-[10px] font-bold uppercase tracking-wider ' + (ok ? 'text-emerald-700 dark:text-emerald-400' : 'text-red-700 dark:text-red-400');
    titleEl.innerText = title;
    detailEl.innerText = detail;
}

function updateVerifyExitButton() {
    const btn = document.getElementById('verify-exit-btn');
    if (!btn) return;
    const show = isConnected && (currentMode === 'exit-via' || currentExitNodeID);
    btn.classList.toggle('hidden', !show);
}

async function restoreVpnState() {
    if (!window.go?.main?.App?.GetStatus) return;
    try {
        if (window.go?.main?.App?.GetShareAsExit) {
            shareAsExit = await window.go.main.App.GetShareAsExit();
            localStorage.setItem(SHARE_AS_EXIT_KEY, shareAsExit ? '1' : '0');
        }
        const status = await window.go.main.App.GetStatus();
        if (status.startsWith('Connected') || status.startsWith('Logged in') || status.includes('Exit node')) {
            showVpnAfterLogin(false);
        }
        if (status.startsWith('Connected') || status.includes('Exit node')) {
            applyConnectedUI(status);
        }
        updateExitShareUI();
    } catch (_) {}
}

function scheduleRestoreVpnState() {
    restoreVpnState();
    setTimeout(restoreVpnState, 300);
}

function showVpnAfterLogin(playLoginVideo = true) {
    loginView?.classList.add('hidden');
    vpnView?.classList.remove('hidden');
    if (playLoginVideo && !isConnected) {
        playVideo('/videos/login.mp4');
    }
    loadExitNodes();
    updateExitShareUI();
    showExitPromptIfNeeded();
    maybeAutoConnectExitShare();

    if (window.go?.main?.App?.GetLoggedInUser) {
        window.go.main.App.GetLoggedInUser().then(user => {
            const el = document.getElementById('logged-in-user');
            if (el) el.innerText = displayNameOnly(user);
        }).catch(() => {});
    }
}

function showLoginView() {
    closeAdminConsole(false);
    vpnView?.classList.add('hidden');
    loginView?.classList.remove('hidden');
    loginFormPanel?.classList.remove('hidden');
    loginLoadingState?.classList.add('hidden');
    loginLoadingState?.classList.remove('flex');
    resetVpnConnectionUI();
    playVideo('/videos/login.mp4');
    loadSavedAccounts();
    setTimeout(updateLoginScrollFade, 50);
}

function cancelLogin() {
    loginLoadingState?.classList.add('hidden');
    loginLoadingState?.classList.remove('flex');
    loginFormPanel?.classList.remove('hidden');
}

function setVpnActionsBusy(busy) {
    document.querySelectorAll('[data-vpn-action]').forEach(el => {
        el.classList.toggle('action-busy', busy);
    });
}

function runBackgroundCleanup(task) {
    Promise.resolve().then(task).catch(err => console.error(err));
}

function saveRememberEmail(email) {
    const remember = document.getElementById('remember-me')?.checked;
    if (remember && email) {
        localStorage.setItem(REMEMBER_EMAIL_KEY, '1');
        localStorage.setItem(SAVED_EMAIL_KEY, email);
    } else {
        localStorage.removeItem(REMEMBER_EMAIL_KEY);
        localStorage.removeItem(SAVED_EMAIL_KEY);
    }
}

function restoreRememberEmail() {
    if (localStorage.getItem(REMEMBER_EMAIL_KEY) === '1') {
        const email = localStorage.getItem(SAVED_EMAIL_KEY) || '';
        const emailInput = document.getElementById('email');
        const remember = document.getElementById('remember-me');
        if (emailInput && email) emailInput.value = email;
        if (remember) remember.checked = true;
    }
}

async function loadSavedAccounts() {
    const panel = document.getElementById('saved-accounts-panel');
    const list = document.getElementById('saved-accounts-list');
    if (!panel || !list || !window.go?.main?.App?.ListSavedAccountsJSON) {
        panel?.classList.add('hidden');
        return;
    }

    try {
        const jsonStr = await window.go.main.App.ListSavedAccountsJSON();
        const accounts = JSON.parse(jsonStr || '[]');
        if (!accounts.length) {
            panel.classList.add('hidden');
            list.innerHTML = '';
            return;
        }

        panel.classList.remove('hidden');
        list.innerHTML = accounts.map(acct => {
            const name = displayNameOnly(acct.display_name || acct.email);
            const initials = accountInitials(name);
            const hue = accountHue(acct.email);
            const safeEmail = acct.email.replace(/'/g, "\\'");
            return `
                <div class="account-chip group relative">
                    <button type="button" onclick="switchToAccount('${safeEmail}')" title="${acct.email}"
                        class="flex flex-col items-center gap-1.5 w-[4.75rem] focus:outline-none">
                        <div class="w-11 h-11 rounded-full flex items-center justify-center text-white text-xs font-bold shadow-md ring-2 ring-white dark:ring-gray-950 group-hover:scale-105 transition-transform duration-200"
                            style="background: linear-gradient(135deg, hsl(${hue} 78% 52%), hsl(${(hue + 28) % 360} 72% 42%));">
                            ${initials}
                        </div>
                        <span class="text-[10px] font-semibold text-gray-700 dark:text-gray-300 max-w-[4.75rem] truncate text-center leading-tight">${name}</span>
                    </button>
                    <button type="button" onclick="event.stopPropagation(); removeSavedAccount('${safeEmail}')" title="Remove ${name}"
                        class="absolute -top-0.5 -right-0.5 w-4 h-4 rounded-full bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 text-[10px] text-gray-400 hover:text-red-500 hover:border-red-300 opacity-0 group-hover:opacity-100 transition shadow-sm leading-none">
                        ×
                    </button>
                </div>`;
        }).join('');
        setTimeout(updateLoginScrollFade, 50);
    } catch (_) {
        panel?.classList.add('hidden');
    }
}

window.showNewAccountForm = async function () {
    hideLoginError();
    closeAdminConsole(false);
    try {
        if (window.go?.main?.App?.IsLoggedIn && window.go?.main?.App?.Logout) {
            const loggedIn = await window.go.main.App.IsLoggedIn();
            if (loggedIn) {
                await window.go.main.App.Logout();
            }
        }
    } catch (_) {}
    const emailInput = document.getElementById('email');
    const passwordInput = document.getElementById('password');
    if (emailInput) emailInput.value = '';
    if (passwordInput) passwordInput.value = '';
    emailInput?.focus();
};

window.switchToAccount = async function (email) {
    if (actionBusy) return;
    hideLoginError();
    if (!window.go?.main?.App?.SwitchAccount) {
        showLoginError('Account switching requires the desktop backend.');
        return;
    }

    actionBusy = true;
    setVpnActionsBusy(true);
    loginFormPanel?.classList.add('hidden');
    loginLoadingState?.classList.remove('hidden');
    loginLoadingState?.classList.add('flex');

    const wasConnected = isConnected;
    if (wasConnected) {
        resetVpnConnectionUI();
        runBackgroundCleanup(() => window.go.main.App.DisconnectTunnel());
    }

    try {
        const result = await window.go.main.App.SwitchAccount(email);
        if (result.startsWith('Success')) {
            closeAdminConsole(false);
            window.go.main.App.PrepareAdminSession?.().catch(() => {});
            showVpnAfterLogin();
        } else {
            showLoginView();
            const emailInput = document.getElementById('email');
            if (emailInput) emailInput.value = email;
            showLoginError(result.replace(/^Error:\s*/, ''));
        }
    } catch (err) {
        showLoginView();
        showLoginError('Could not switch account: ' + err);
    } finally {
        actionBusy = false;
        setVpnActionsBusy(false);
        cancelLogin();
    }
};

window.removeSavedAccount = async function (email) {
    if (!window.go?.main?.App?.RemoveSavedAccount) return;
    try {
        await window.go.main.App.RemoveSavedAccount(email);
        await loadSavedAccounts();
    } catch (_) {}
};

window.loginUser = function () {
    const email = document.getElementById('email')?.value?.trim();
    const password = document.getElementById('password')?.value || '';
    if (!email || !password) {
        showLoginError('Please enter email and password.');
        return;
    }
    hideLoginError();
    if (loginBtn) loginBtn.disabled = true;

    window.go.main.App.Login(email, password).then(result => {
        if (loginBtn) loginBtn.disabled = false;
        if (result.startsWith('Success')) {
            saveRememberEmail(email);
            closeAdminConsole(false);
            window.go.main.App.PrepareAdminSession?.().catch(() => {});
            loadSavedAccounts();
            showVpnAfterLogin();
        } else {
            showLoginError(result.replace(/^Error:\s*/, ''));
            cancelLogin();
        }
    }).catch(err => {
        if (loginBtn) loginBtn.disabled = false;
        showLoginError('Login failed: ' + err);
        cancelLogin();
    });
};

window.loginWithGoogle = function () {
    loginFormPanel?.classList.add('hidden');
    loginLoadingState?.classList.remove('hidden');
    loginLoadingState?.classList.add('flex');
    hideLoginError();

    if (!window.go?.main?.App?.LoginWithGoogle) {
        showLoginError('Google sign-in requires the desktop backend.');
        cancelLogin();
        return;
    }

    window.go.main.App.LoginWithGoogle().then(result => {
        if (result.startsWith('Success')) {
            closeAdminConsole(false);
            window.go.main.App.PrepareAdminSession?.().catch(() => {});
            loadSavedAccounts();
            showVpnAfterLogin();
        } else {
            showLoginError(result.replace(/^Error:\s*/, ''));
            cancelLogin();
        }
    }).catch(err => {
        showLoginError('Google login failed: ' + err);
        cancelLogin();
    });
};

window.logoutUser = function () {
    if (actionBusy) return;
    actionBusy = true;
    setVpnActionsBusy(true);

    const wasConnected = isConnected;
    closeAdminConsole(false);
    showLoginView();

    runBackgroundCleanup(async () => {
        try {
            if (wasConnected && window.go?.main?.App?.DisconnectTunnel) {
                await window.go.main.App.DisconnectTunnel();
            }
            if (window.go?.main?.App?.Logout) {
                await window.go.main.App.Logout();
            }
        } finally {
            actionBusy = false;
            setVpnActionsBusy(false);
        }
    });
};

window.switchAccountFromVpn = function () {
    if (actionBusy) return;
    actionBusy = true;
    setVpnActionsBusy(true);

    const wasConnected = isConnected;
    showLoginView();

    runBackgroundCleanup(async () => {
        try {
            if (wasConnected && window.go?.main?.App?.DisconnectTunnel) {
                await window.go.main.App.DisconnectTunnel();
            }
            if (window.go?.main?.App?.Logout) {
                await window.go.main.App.Logout();
            }
        } finally {
            actionBusy = false;
            setVpnActionsBusy(false);
        }
    });
};

window.toggleConnection = function () {
    if (actionBusy) return;

    if (shareAsExit) {
        window.toggleShareAsExit(!isConnected);
        return;
    }

    if (!isConnected) {
        actionBusy = true;
        connectBtn?.classList.add('action-busy');
        if (statusText) statusText.innerText = 'Connecting...';

        let dnsSetting = 'off';
        const overrideDns = document.getElementById('setting-dns-toggle')?.checked;
        
        if (overrideDns !== false) { // defaults to true if not found
            dnsSetting = document.getElementById('setting-dns-mode')?.value || 'mscale';
            if (dnsSetting === 'custom') {
                const customIp = document.getElementById('setting-custom-dns-ip')?.value?.trim();
                if (customIp) dnsSetting = customIp;
                else dnsSetting = 'google'; // fallback if they selected custom but left it blank
            }
        }
        window.go.main.App.ConnectTunnel(currentMode, currentExitNodeID, dnsSetting).then(result => {
            if (result.startsWith('Error')) {
                alert(result);
                if (statusText) statusText.innerText = 'Disconnected';
            } else {
                applyConnectedUI(result.split('\n')[0].startsWith('Assigned IP:')
                    ? `Connected — IP: ${result.split('\n')[0].replace('Assigned IP: ', '')}`
                    : 'Connected');
                if (currentMode === 'exit-via') {
                    updateVerifyExitButton();
                    scheduleExitTestAfterConnect();
                }

                window.go.main.App.GetStatus().then(status => {
                    if (status.startsWith('Connected')) {
                        applyConnectedUI(status);
                    } else if (statusText) {
                        statusText.innerText = status;
                    }
                }).catch(() => {});
            }
        }).catch(err => {
            alert('Connection failed: ' + err);
            if (statusText) statusText.innerText = 'Disconnected';
        }).finally(() => {
            actionBusy = false;
            connectBtn?.classList.remove('action-busy');
        });
    } else {
        actionBusy = true;
        if (statusText) statusText.innerText = 'Disconnecting...';
        resetVpnConnectionUI();
        playVideo('/videos/login.mp4');

        window.go.main.App.DisconnectTunnel().catch(err => {
            alert('Disconnect failed: ' + err);
        }).finally(() => {
            actionBusy = false;
            if (statusText) statusText.innerText = 'Disconnected';
        });
    }
};

window.openModal = function (id) {
    const modal = document.getElementById(id);
    const backdrop = document.getElementById('modal-backdrop');
    if (!modal || !backdrop) return;
    backdrop.classList.remove('hidden');
    modal.classList.remove('hidden');
    setTimeout(() => {
        backdrop.classList.remove('opacity-0');
        modal.classList.remove('opacity-0', 'translate-y-full', 'translate-x-full');
    }, 10);
};

window.closeModal = function (id) {
    const modal = document.getElementById(id);
    const backdrop = document.getElementById('modal-backdrop');
    if (!modal) return;
    modal.classList.add('opacity-0');
    if (id === 'location-modal') modal.classList.add('translate-y-full');
    if (id === 'settings-modal') modal.classList.add('translate-x-full');

    if (backdrop) backdrop.classList.add('opacity-0');

    setTimeout(() => {
        modal.classList.add('hidden');
        if (backdrop) backdrop.classList.add('hidden');
    }, 300);
};

window.closeAllModals = function () {
    ['location-modal', 'settings-modal', 'logs-modal'].forEach(id => {
        window.closeModal(id);
    });
};

window.selectRoutingMode = function (mode, name) {
    if (shareAsExit) return;
    currentMode = mode;
    const activeLocationText = document.getElementById('active-location');
    if (activeLocationText) activeLocationText.innerText = name;

    const container = document.getElementById('exit-nodes-container');
    if (mode === 'exit-via') {
        container?.classList.remove('hidden');
    } else {
        container?.classList.add('hidden');
        window.closeModal('location-modal');
    }
};

let allNodes = [];
window.loadExitNodes = async function () {
    try {
        if (window.go?.main?.App?.ListExitNodesJSON) {
            const jsonStr = await window.go.main.App.ListExitNodesJSON();
            allNodes = JSON.parse(jsonStr);
            if (allNodes.length > 0 && allNodes[0]._error) {
                console.error('Exit node load error:', allNodes[0]._error);
                allNodes = [];
            }
            renderNodes(allNodes);
        }
    } catch (e) {
        console.error(e);
    }
};

window.filterExitNodes = function () {
    const term = (document.getElementById('exit-node-search')?.value || '').toLowerCase();
    const filtered = allNodes.filter(n =>
        (n.label && n.label.toLowerCase().includes(term)) ||
        (n.device_name && n.device_name.toLowerCase().includes(term))
    );
    renderNodes(filtered);
};

function renderNodes(nodes) {
    const list = document.getElementById('exit-nodes-list');
    if (!list) return;
    if (!nodes || nodes.length === 0) {
        list.innerHTML = '<div class="text-center py-4 text-xs text-gray-400">No exit nodes available.</div>';
        return;
    }
    let html = '';
    for (let i = 0; i < nodes.length; i++) {
        const n = nodes[i];
        const label = n.label || n.device_name;
        html += `<div onclick="selectExitNode('${n.id}', '${label.replace(/'/g, "\\'")}')" class="p-3 rounded-xl border border-gray-150 dark:border-gray-800 hover:bg-gray-50 dark:hover:bg-gray-800 cursor-pointer transition">`;
        html += `<p class="text-sm font-bold text-gray-900 dark:text-white">${label}</p>`;
        html += `<p class="text-[10px] text-emerald-500 mt-1">${n.status}</p>`;
        html += '</div>';
    }
    list.innerHTML = html;
}

window.selectExitNode = function (id, label) {
    currentExitNodeID = id;
    currentMode = 'exit-via';
    const activeLocationText = document.getElementById('active-location');
    if (activeLocationText) activeLocationText.innerText = label;
    window.closeModal('location-modal');
    updateExitViaUI();
    updateVerifyExitButton();
};

window.verifyExitConnection = async function (autoRun) {
    if (!window.go?.main?.App?.VerifyExitRouteJSON) {
        showExitTestResult(false, 'Update required', 'Rebuild the desktop app to use exit ping.');
        return;
    }
    const btn = document.getElementById('verify-exit-btn');
    const spinner = document.getElementById('verify-exit-spinner');
    const label = document.getElementById('verify-exit-label');
    if (btn) btn.disabled = true;
    spinner?.classList.remove('hidden');
    if (label) label.innerText = autoRun ? 'Testing exit route…' : 'Sending ping…';

    try {
        const raw = await window.go.main.App.VerifyExitRouteJSON();
        const data = JSON.parse(raw || '{}');
        if (data.error) {
            const hint = data.hint ? '\n' + data.hint : '';
            showExitTestResult(false, 'Not routed via exit', String(data.error) + hint);
            if (!autoRun) alert(data.error + (data.hint ? '\n\n' + data.hint : ''));
            return;
        }
        if (data.test_error) {
            showExitTestResult(false, 'Ping failed', String(data.test_error));
            return;
        }

        const exitName = data.exit_device || data.exit_device_name || 'exit device';
        let detail = data.message || 'Exit route is active.';
        if (data.notification) detail += '\n\n' + data.notification;
        if (data.exit_overlay) detail += '\nVPN IP: ' + data.exit_overlay;
        detail += '\n\nCheck your phone — you should get a notification within a few seconds.';

        const sent = data.test_sent === true || !!data.command_id;
        showExitTestResult(
            sent,
            sent ? 'Ping sent to ' + exitName : 'Route confirmed',
            detail
        );
    } catch (e) {
        showExitTestResult(false, 'Test failed', String(e));
    } finally {
        if (btn) btn.disabled = false;
        spinner?.classList.add('hidden');
        if (label) label.innerText = 'Ping exit device';
    }
};

function scheduleExitTestAfterConnect() {
    if (currentMode !== 'exit-via' || !currentExitNodeID || !isConnected) return;
    updateExitViaUI();
    setTimeout(() => verifyExitConnection(true), 2000);
}

window.selectDnsOption = function (val, text) {
    const hiddenInput = document.getElementById('setting-dns-mode');
    const selectedText = document.getElementById('setting-dns-selected-text');
    const optionsMenu = document.getElementById('setting-dns-options-menu');
    
    if (hiddenInput && selectedText) {
        hiddenInput.value = val;
        selectedText.innerText = text;
        localStorage.setItem('mscale_dns_mode', val);
        
        // Trigger manual update
        const customDnsContainer = document.getElementById('setting-custom-dns-container');
        if (customDnsContainer) {
            if (val === 'custom') {
                customDnsContainer.classList.remove('hidden');
            } else {
                customDnsContainer.classList.add('hidden');
            }
        }
    }
    
    // Close menu
    if (optionsMenu) {
        optionsMenu.classList.add('opacity-0', 'invisible', 'scale-95');
        optionsMenu.classList.remove('opacity-100', 'visible', 'scale-100');
    }
};

window.copyAdminWebLink = async function () {
    try {
        await navigator.clipboard.writeText(ADMIN_WEB_URL);
    } catch (_) {
        const el = document.createElement('textarea');
        el.value = ADMIN_WEB_URL;
        document.body.appendChild(el);
        el.select();
        document.execCommand('copy');
        document.body.removeChild(el);
    }
    const hint = document.getElementById('admin-link-hint');
    if (hint) {
        const prev = hint.textContent;
        hint.textContent = 'Link copied!';
        setTimeout(() => { hint.textContent = prev; }, 1800);
    }
};

window.openAdminInBrowser = async function () {
    if (!window.go?.main?.App?.GetAdminConsoleURL) {
        BrowserOpenURL(ADMIN_WEB_URL);
        return;
    }
    try {
        const url = await window.go.main.App.GetAdminConsoleURL();
        BrowserOpenURL(url || ADMIN_WEB_URL);
    } catch (_) {
        BrowserOpenURL(ADMIN_WEB_URL);
    }
};

window.openAdminConsole = async function () {
    const overlay = document.getElementById('admin-overlay');
    const frame = document.getElementById('admin-frame');
    const loading = document.getElementById('admin-loading');
    if (!overlay || !frame) return;

    if (!window.go?.main?.App?.IsLoggedIn) {
        alert('Please sign in to the app first.');
        return;
    }

    try {
        const loggedIn = await window.go.main.App.IsLoggedIn();
        if (!loggedIn) {
            alert('Session expired. Please sign in again.');
            showLoginView();
            return;
        }
        await window.go.main.App.PrepareAdminSession?.().catch(() => {});
    } catch (_) {}

    loading?.classList.remove('hidden');
    overlay.classList.remove('hidden');
    overlay.classList.add('open', 'flex');

    frame.onload = () => {
        loading?.classList.add('hidden');
    };

    // Force fresh load so profile matches current login session.
    frame.src = 'about:blank';
    requestAnimationFrame(() => {
        frame.src = '/admin.html?embedded=1&t=' + Date.now();
    });
};

window.closeAdminConsole = function (restore = true) {
    const overlay = document.getElementById('admin-overlay');
    const frame = document.getElementById('admin-frame');
    const loading = document.getElementById('admin-loading');
    if (!overlay || overlay.classList.contains('hidden')) return;

    overlay.classList.add('hidden');
    overlay.classList.remove('open', 'flex');
    if (frame) frame.src = 'about:blank';
    loading?.classList.remove('hidden');
    if (restore) {
        scheduleRestoreVpnState();
    }
};

function applyTheme(dark) {
    document.documentElement.classList.toggle('dark', dark);
    document.getElementById('sun-icon')?.classList.toggle('hidden', !dark);
    document.getElementById('moon-icon')?.classList.toggle('hidden', dark);
    localStorage.setItem('mscale_theme', dark ? 'dark' : 'light');
}

function initTheme() {
    const saved = localStorage.getItem('mscale_theme');
    const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
    applyTheme(saved ? saved === 'dark' : prefersDark);
}

function initAdminRpcBridge() {
    window.addEventListener('message', async (event) => {
        const data = event.data;
        if (!data) return;

        if (data.type === 'mscale-admin-request-signin') {
            closeAdminConsole(false);
            showLoginView();
            return;
        }
        if (data.type === 'mscale-admin-logout') {
            logoutUser();
            return;
        }
        if (data.type === 'mscale-admin-open-browser') {
            closeAdminConsole(false);
            openAdminInBrowser();
            return;
        }

        if (data.type !== 'mscale-admin-rpc') return;

        const frame = document.getElementById('admin-frame');
        if (!frame?.contentWindow || event.source !== frame.contentWindow) return;

        const { id, method, args = [] } = data;
        const reply = (payload) => {
            try {
                frame.contentWindow.postMessage({ type: 'mscale-admin-rpc-result', id, ...payload }, '*');
            } catch (_) {}
        };

        try {
            const app = window.go?.main?.App;
            if (!app || typeof app[method] !== 'function') {
                reply({ error: 'Method not available: ' + method });
                return;
            }
            const result = await app[method](...args);
            reply({ result });
        } catch (err) {
            reply({ error: String(err) });
        }
    });
}

let pendingUpdateUrl = '';

async function checkForUpdates() {
    if (!window.go?.main?.App?.CheckForUpdateJSON) return;
    try {
        const raw = await window.go.main.App.CheckForUpdateJSON();
        const info = JSON.parse(raw || '{}');
        if (!info.update_available) return;
        pendingUpdateUrl = info.setup_url || info.download_url || 'https://mashad.shop/mscale/download.html';
        const banner = document.getElementById('update-banner');
        const text = document.getElementById('update-banner-text');
        if (text) {
            const notes = info.notes ? ` — ${info.notes}` : '';
            text.textContent = `Update available: v${info.latest_version} (you have v${info.current_version})${notes}`;
        }
        banner?.classList.remove('hidden');
    } catch (_) {}
}

window.openUpdateDownload = function () {
    const url = pendingUpdateUrl || 'https://mashad.shop/mscale/download.html';
    BrowserOpenURL(url);
};

window.dismissUpdateBanner = function () {
    document.getElementById('update-banner')?.classList.add('hidden');
    try { localStorage.setItem('mscale_update_dismissed', pendingUpdateUrl); } catch (_) {}
};

document.addEventListener('DOMContentLoaded', () => {
    initTheme();
    initAdminRpcBridge();
    restoreRememberEmail();
    loadSavedAccounts();
    initLoginScrollFade();

    const dnsToggle = document.getElementById('setting-dns-toggle');
    const dnsDropdownContainer = document.getElementById('setting-dns-dropdown-container');
    const dnsModeSelect = document.getElementById('setting-dns-mode');
    const customDnsContainer = document.getElementById('setting-custom-dns-container');
    const customDnsIp = document.getElementById('setting-custom-dns-ip');

    if (dnsModeSelect && customDnsContainer && customDnsIp) {
        if (dnsToggle) {
            const savedToggle = localStorage.getItem('mscale_dns_override') || 'true';
            dnsToggle.checked = (savedToggle === 'true');
        }

        const savedDnsMode = localStorage.getItem('mscale_dns_mode') || 'mscale';
        dnsModeSelect.value = savedDnsMode;
        
        const savedCustomIp = localStorage.getItem('mscale_custom_dns_ip') || '';
        customDnsIp.value = savedCustomIp;

        const updateCustomDnsVisibility = () => {
            if (dnsToggle && !dnsToggle.checked) {
                dnsDropdownContainer?.classList.add('hidden');
                customDnsContainer.classList.add('hidden');
                return;
            }
            dnsDropdownContainer?.classList.remove('hidden');
            if (dnsModeSelect.value === 'custom') {
                customDnsContainer.classList.remove('hidden');
            } else {
                customDnsContainer.classList.add('hidden');
            }
        };
        updateCustomDnsVisibility();

        dnsToggle?.addEventListener('change', (e) => {
            localStorage.setItem('mscale_dns_override', e.target.checked);
            updateCustomDnsVisibility();
        });

        // Initialize custom dropdown text
        const dnsSelectedText = document.getElementById('setting-dns-selected-text');
        if (dnsSelectedText) {
            if (savedDnsMode === 'google') dnsSelectedText.innerText = 'Google Public DNS';
            else if (savedDnsMode === 'custom') dnsSelectedText.innerText = 'Custom DNS...';
            else dnsSelectedText.innerText = 'Mscale MagicDNS';
        }

        // Custom dropdown toggle logic
        const dnsDropdownBtn = document.getElementById('setting-dns-dropdown-btn');
        const dnsOptionsMenu = document.getElementById('setting-dns-options-menu');
        
        if (dnsDropdownBtn && dnsOptionsMenu) {
            dnsDropdownBtn.addEventListener('click', (e) => {
                e.stopPropagation();
                const isClosed = dnsOptionsMenu.classList.contains('opacity-0');
                if (isClosed) {
                    dnsOptionsMenu.classList.remove('opacity-0', 'invisible', 'scale-95');
                    dnsOptionsMenu.classList.add('opacity-100', 'visible', 'scale-100');
                } else {
                    dnsOptionsMenu.classList.add('opacity-0', 'invisible', 'scale-95');
                    dnsOptionsMenu.classList.remove('opacity-100', 'visible', 'scale-100');
                }
            });

            // Close when clicking outside
            document.addEventListener('click', (e) => {
                if (!dnsDropdownBtn.contains(e.target) && !dnsOptionsMenu.contains(e.target)) {
                    dnsOptionsMenu.classList.add('opacity-0', 'invisible', 'scale-95');
                    dnsOptionsMenu.classList.remove('opacity-100', 'visible', 'scale-100');
                }
            });
        }

        customDnsIp.addEventListener('input', (e) => {
            localStorage.setItem('mscale_custom_dns_ip', e.target.value.trim());
        });
    }

    document.getElementById('btn-minimize')?.addEventListener('click', () => {
        WindowMinimise();
    });

    document.getElementById('btn-close')?.addEventListener('click', () => {
        Quit();
    });

    document.getElementById('btn-theme')?.addEventListener('click', () => {
        const dark = !document.documentElement.classList.contains('dark');
        applyTheme(dark);
    });

    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape') {
            closeAdminConsole();
        }
    });

    if (window.go?.main?.App?.GetStatus) {
        scheduleRestoreVpnState();
    }
    checkForUpdates();
});
