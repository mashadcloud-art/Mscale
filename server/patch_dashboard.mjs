import fs from 'fs';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';

const __dir = dirname(fileURLToPath(import.meta.url));
const p = join(__dir, 'dashboard.html');
let html = fs.readFileSync(p, 'utf8');

html = html.replace(
  `                        <!-- Profile Dropdown -->
                        <div class="relative">
                            <button onclick="toggleProfileDropdown()" class="relative rounded-full focus:outline-none flex items-center" aria-label="User menu" id="profileButton">
                                <div class="relative shrink-0 rounded-full overflow-hidden w-8 h-8 bg-blue-100 dark:bg-blue-900 border border-blue-200 dark:border-blue-700 flex items-center justify-center">
                                    <span class="text-xs font-bold text-blue-700 dark:text-blue-300">MH</span>
                                </div>
                            </button>
                            <!-- Dropdown Card -->
                            <div id="profileDropdown" class="hidden absolute right-0 mt-2 w-64 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg py-2 z-50">
                                <div class="px-4 py-3 border-b border-gray-100 dark:border-gray-700">
                                    <p class="text-xs text-gray-500 dark:text-gray-400">Signed in as</p>
                                    <p class="text-sm font-semibold truncate text-gray-800 dark:text-gray-100">mashadandhaz@gmail.com</p>
                                </div>
                                <div class="px-4 py-2 hover:bg-gray-50 dark:hover:bg-gray-700 cursor-pointer text-sm text-red-600 dark:text-red-400" onclick="logoutUser()">Log Out</div>
                            </div>
                        </div>`,
  `                        <!-- Profile Dropdown -->
                        <div class="relative" onclick="event.stopPropagation()">
                            <button onclick="toggleProfileDropdown(event)" class="relative rounded-full focus:outline-none flex items-center" aria-label="User menu" id="profileButton">
                                <div class="relative shrink-0 rounded-full overflow-hidden w-8 h-8 bg-blue-100 dark:bg-blue-900 border border-blue-200 dark:border-blue-700 flex items-center justify-center">
                                    <span class="text-xs font-bold text-blue-700 dark:text-blue-300" id="profile-initials">MH</span>
                                </div>
                            </button>
                            <div id="profileDropdown" class="hidden absolute right-0 mt-2 w-64 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg py-2 z-50">
                                <div class="px-4 py-3 border-b border-gray-100 dark:border-gray-700">
                                    <p class="text-xs text-gray-500 dark:text-gray-400">Signed in as</p>
                                    <p class="text-sm font-semibold truncate text-gray-800 dark:text-gray-100" id="profile-email-dd">—</p>
                                </div>
                                <button type="button" onclick="logoutUser(event)" class="w-full text-left px-4 py-2 hover:bg-gray-50 dark:hover:bg-gray-700 text-sm text-red-600 dark:text-red-400">Log Out</button>
                            </div>
                        </div>`
);

const oldBannerStart = '                <!-- "Add First Device" Interactive Hero / Banner -->';
const oldBannerEnd = '                <!-- Restore Banner Button';
const i0 = html.indexOf(oldBannerStart);
const i1 = html.indexOf(oldBannerEnd);
if (i0 === -1 || i1 === -1) throw new Error('banner block not found: ' + i0 + ' ' + i1);
const newBanner = `                <!-- Quick Start banner -->
                <div id="first-device-banner" class="relative rounded-xl overflow-hidden border border-[#D5DAE8] dark:border-gray-700 bg-[#E7EAF2] dark:bg-gray-900 transition-all duration-300 min-h-[200px]">
                    <img id="banner-bg" src="assets/cheetahbanner.png" alt="" class="absolute inset-0 w-full h-full object-cover object-center md:object-right pointer-events-none select-none" aria-hidden="true">
                    <div class="absolute inset-0 bg-gradient-to-r from-[#E7EAF2]/95 via-[#E7EAF2]/75 to-transparent dark:from-gray-900/95 dark:via-gray-900/70 dark:to-transparent pointer-events-none"></div>
                    <button onclick="minimizeBanner()" class="absolute right-3 top-3 z-20 text-slate-500 dark:text-gray-400 hover:text-slate-800 dark:hover:text-white p-1.5 rounded-lg hover:bg-white/60 dark:hover:bg-black/30 transition" aria-label="Minimize block">
                        <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="5" x2="19" y1="12" y2="12"/></svg>
                    </button>
                    <div class="relative z-10 p-6 md:p-8 max-w-xl space-y-3">
                        <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold bg-blue-100 dark:bg-blue-900 text-blue-800 dark:text-blue-300">Quick Start</span>
                        <h4 class="font-semibold text-lg md:text-xl text-gray-900 dark:text-white">Connect your ecosystem devices</h4>
                        <p class="text-gray-600 dark:text-gray-400 text-sm max-w-md leading-relaxed">
                            Add desktop, mobile, or cloud nodes to your Mscale mesh. Use <strong>Wake / manage</strong> on any device to open the app remotely.
                        </p>
                        <div class="pt-1 flex flex-wrap gap-2">
                            <button onclick="openAddDeviceModal()" class="bg-blue-600 dark:bg-blue-500 hover:bg-blue-700 text-white font-medium text-xs px-3.5 py-2 rounded shadow-sm transition">Add device</button>
                            <a href="download.html" class="border border-blue-200 dark:border-blue-800 text-blue-700 dark:text-blue-300 text-xs px-3.5 py-2 rounded hover:bg-white/50 dark:hover:bg-gray-800 transition">Download apps</a>
                        </div>
                    </div>
                </div>

                <!-- Restore Banner Button`;
html = html.slice(0, i0) + newBanner + html.slice(i1);

html = html.replace('<th scope="col" class="px-6 py-4">App</th>', '<th scope="col" class="px-6 py-4">App</th>\n                                    <th scope="col" class="px-6 py-4 text-right">Actions</th>');
html = html.replace('Tailscale Mesh Status: Connected', 'Mscale Mesh: Live');
html = html.replace('Admin Tailnet Simulation Console', 'Mscale — Secure Mesh Networking');

const wakeModal = `
    <!-- Wake / Manage Device Modal -->
    <div id="modal-device-actions" class="hidden fixed inset-0 bg-black/60 backdrop-blur-xs flex items-center justify-center p-4 z-[100]">
        <div class="bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-800 rounded-xl max-w-md w-full overflow-hidden shadow-2xl max-h-[90vh] overflow-y-auto">
            <div class="p-6 border-b border-gray-100 dark:border-gray-800 flex justify-between items-center sticky top-0 bg-white dark:bg-gray-900 z-10">
                <h3 class="font-semibold text-lg text-gray-900 dark:text-white" id="device-actions-title">Wake / manage</h3>
                <button onclick="closeDeviceActionsModal()" class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200">✕</button>
            </div>
            <div class="p-6 space-y-4">
                <input type="hidden" id="action-device-id">
                <p class="text-xs text-gray-500" id="action-device-status">—</p>
                <div class="rounded-xl border border-emerald-200 dark:border-emerald-900 bg-emerald-50/80 dark:bg-emerald-950/20 p-4 space-y-3">
                    <p class="text-xs font-semibold text-emerald-800 dark:text-emerald-300 uppercase tracking-wider">Call to open app (when closed)</p>
                    <div class="flex gap-2">
                        <input type="tel" id="action-device-phone" placeholder="+91 98765 43210" class="flex-1 bg-white dark:bg-gray-900 border border-gray-300 dark:border-gray-700 rounded-lg px-3 py-2 text-xs font-mono">
                        <button type="button" onclick="saveDevicePhone()" class="shrink-0 bg-gray-800 text-white text-xs px-3 py-2 rounded-lg">Save</button>
                    </div>
                    <div class="flex flex-wrap gap-2">
                        <button type="button" onclick="queueWakeOnly()" class="bg-emerald-600 hover:bg-emerald-700 text-white text-xs px-3 py-2 rounded-lg">Queue wake only</button>
                        <button type="button" onclick="callAndWake()" class="bg-blue-600 hover:bg-blue-700 text-white text-xs px-3 py-2 rounded-lg">Call &amp; wake</button>
                        <button type="button" onclick="wakeAndConnect()" class="border border-emerald-300 text-emerald-800 text-xs px-3 py-2 rounded-lg">Wake &amp; connect</button>
                    </div>
                </div>
                <div class="rounded-xl border border-gray-200 dark:border-gray-800 p-4 space-y-3">
                    <p class="text-xs font-semibold text-gray-500 uppercase tracking-wider">Exit routing (hub)</p>
                    <select id="action-exit-node" class="w-full bg-white dark:bg-gray-900 border border-gray-300 dark:border-gray-700 rounded-lg px-3 py-2 text-xs"><option value="">— Select exit node —</option></select>
                    <div class="flex flex-wrap gap-2">
                        <button type="button" onclick="adminRouteDevice('exit-via')" class="bg-indigo-600 text-white text-xs px-3 py-2 rounded-lg">Route via exit</button>
                        <button type="button" onclick="adminRouteDevice('enable_exit')" class="border border-indigo-300 text-indigo-700 text-xs px-3 py-2 rounded-lg">Share as exit</button>
                        <button type="button" onclick="adminRouteDevice('mesh')" class="border border-gray-300 text-gray-600 text-xs px-3 py-2 rounded-lg">Mesh only</button>
                    </div>
                </div>
            </div>
        </div>
    </div>

`;
html = html.replace('    <!-- FOOTER STATUS STRIP -->', wakeModal + '    <!-- FOOTER STATUS STRIP -->');

const inject = `
        function isEmbeddedAdmin() { return new URLSearchParams(window.location.search).get('embedded') === '1'; }
        function getDesktopApp() { try { return window.go?.main?.App || window.parent?.go?.main?.App; } catch (_) { return null; } }
        function isDesktopApp() { return window.location.hostname === 'wails.localhost' || isEmbeddedAdmin() || !!getDesktopApp(); }
        function apiPathPrefix() { return window.location.pathname.startsWith('/mscale') ? '/mscale' : ''; }
        function assetPath(name) { if (isDesktopApp() || isEmbeddedAdmin()) return name; const p = apiPathPrefix(); return p ? p + '/assets/' + name : 'assets/' + name; }
        function applyAssetPaths() { const b = assetPath('cheetahbanner.png'); document.getElementById('banner-bg')?.setAttribute('src', b); }
        async function apiFetch(path, options = {}) {
            const method = options.method || 'GET'; const body = options.body || '';
            if (isDesktopApp() && getDesktopApp()?.AdminAPIJSON) {
                const out = JSON.parse(await getDesktopApp().AdminAPIJSON(path, method, body) || '{}');
                return { ok: out.status >= 200 && out.status < 300, status: out.status, json: async () => out.body };
            }
            return fetch(apiPathPrefix() + path, { credentials: 'include', method, body, headers: body ? {'Content-Type':'application/json'} : {} });
        }
        async function postAPI(path, payload) { const res = await apiFetch(path, { method: 'POST', body: JSON.stringify(payload) }); const data = await res.json(); if (!res.ok) throw new Error(data.error || 'Request failed'); return data; }
        let actionDevice = null; let exitNodesCache = [];
`;
html = html.replace('        let currentUser = null;', '        let currentUser = null;\n' + inject);
html = html.replace('function isRealDesktopDevice(d)', 'function isManagedDevice(d)');
html = html.replace("if (t !== 'desktop' && t !== 'server') return false;", "if (t !== 'desktop' && t !== 'server' && t !== 'mobile') return false;");
html = html.replace('list.filter(isRealDesktopDevice)', 'list.filter(isManagedDevice)');

html = html.replace(
`        function formatRouting(m) {
            if (m.exit_node && m.exit_node.is_enabled) {
                const cc = m.exit_node.country_code || '?';
                return 'Exit node (' + cc + ')';
            }
            const mode = (m.tunnel_mode || 'mesh').toLowerCase();
            if (mode === 'exit-via') return 'Uses another exit node';
            if (mode === 'exit-node') return 'Exit via UAE server';
            return 'Mesh only';
        }`,
`        function formatRouting(m) {
            const mode = (m.tunnel_mode || 'mesh').toLowerCase();
            const en = m.exit_node;
            if (en && en.is_enabled && en.device_id === m.id) return 'Exit provider (' + (en.country_code || '?') + ')';
            if (mode === 'exit-via') return 'Via exit (' + ((en && en.country_code) || '?') + ')';
            if (mode === 'exit-node') return 'Exit provider';
            return 'Mesh only';
        }`);

html = html.replace(
`        async function fetchUserProfile() {
            try {
                const res = await fetch('/mscale/api/me');
                if (res.ok) {
                    currentUser = await res.json();
                    document.getElementById('user-email-header').textContent = currentUser.email;
                    
                    const profileEmails = document.querySelectorAll('#profileDropdown p.text-sm.font-semibold.truncate');
                    if (profileEmails.length > 0) profileEmails[0].textContent = currentUser.email;
                    
                    const profileIconText = document.querySelector('#profileButton span.text-xs.font-bold');
                    if (profileIconText && currentUser.display_name) profileIconText.textContent = currentUser.display_name.substring(0, 2).toUpperCase();
                } else if (res.status === 401) {
                    window.location.href = '/mscale/login.html';
                }
            } catch (e) {
                console.error("Failed to fetch user profile", e);
            }
        }`,
`        async function fetchUserProfile() {
            try {
                if (isDesktopApp() && getDesktopApp()?.GetSessionUserJSON) {
                    const json = await getDesktopApp().GetSessionUserJSON();
                    if (!json) { window.location.replace(apiPathPrefix() + '/login.html?logout=1'); return; }
                    currentUser = JSON.parse(json);
                } else {
                    const res = await apiFetch('/api/me');
                    if (res.status === 401) { window.location.replace(apiPathPrefix() + '/login.html?logout=1'); return; }
                    if (!res.ok) return;
                    currentUser = await res.json();
                }
                document.getElementById('user-email-header').textContent = currentUser.email;
                const pe = document.getElementById('profile-email-dd'); if (pe) pe.textContent = currentUser.email;
                const pi = document.getElementById('profile-initials');
                if (pi) pi.textContent = (currentUser.display_name || currentUser.email || 'MA').substring(0, 2).toUpperCase();
            } catch (e) { console.error('Failed to fetch user profile', e); }
        }`);

html = html.replace(
`        async function logoutUser() {
            try {
                const res = await fetch('/mscale/api/auth/logout', { method: 'POST' });
                if (res.ok) {
                    if (sessionStorage.getItem('mscale_desktop') === '1') {
                        window.location.href = 'http://wails.localhost/index.html?logout=1';
                        return;
                    }
                    window.location.href = '/mscale/login.html';
                } else {
                    alertNotification("Logout failed. Please try again.");
                }
            } catch (e) {
                console.error("Logout error", e);
            }
        }`,
`        async function logoutUser(e) {
            if (e) { e.preventDefault(); e.stopPropagation(); }
            document.getElementById('profileDropdown')?.classList.add('hidden');
            currentUser = null;
            try {
                if (isDesktopApp() && getDesktopApp()?.AdminLogout) await getDesktopApp().AdminLogout();
                else await apiFetch('/api/auth/logout', { method: 'POST' });
            } catch (_) {}
            if (isDesktopApp() || sessionStorage.getItem('mscale_desktop') === '1') {
                window.location.replace('index.html?logout=1');
            } else {
                window.location.replace(apiPathPrefix() + '/login.html?logout=1');
            }
        }`);

html = html.replace(
  'const res = await fetch(`${pathPrefix}/api/devices`, { cache: \'no-store\' });',
  'const res = await fetch(`${pathPrefix}/api/devices`, { cache: \'no-store\', credentials: \'include\' });'
);

html = html.replace(
  "const matchesOS = filterOS === 'all' || os.toLowerCase() === filterOS;",
  `const osNorm = (os || '').toLowerCase();
                let matchesOS = filterOS === 'all';
                if (!matchesOS && filterOS === 'ios') matchesOS = osNorm === 'ios' || osNorm === 'android';
                else if (!matchesOS) matchesOS = osNorm === filterOS || osNorm.includes(filterOS);`
);

html = html.replace(
  "const isOnline = m.status === 'Online';",
  "const isOnline = ['online','connected'].includes((m.status || '').toLowerCase());"
);

html = html.replace(
  `                    <td class="px-6 py-4 font-mono text-xs text-gray-500 dark:text-gray-400">\${displayVersion}</td>
                \`;`,
  `                    <td class="px-6 py-4 font-mono text-xs text-gray-500 dark:text-gray-400">\${displayVersion}</td>
                    <td class="px-6 py-4 text-right">
                        <button onclick="openDeviceActionsModal('\${m.id}')" class="bg-emerald-600 hover:bg-emerald-700 text-white text-xs px-2.5 py-1 rounded">Wake / manage</button>
                    </td>
                \`;`
);

const wakeJs = `
        function toggleProfileDropdown(e) { if (e) e.stopPropagation(); document.getElementById('profileDropdown').classList.toggle('hidden'); }
        document.addEventListener('click', function(e) { if (!e.target.closest('#profileDropdown, #profileButton')) document.getElementById('profileDropdown')?.classList.add('hidden'); });

        async function loadExitNodesForActions() {
            try { const res = await apiFetch('/api/exit-nodes'); if (res.ok) exitNodesCache = await res.json(); } catch (_) { exitNodesCache = []; }
            const sel = document.getElementById('action-exit-node'); if (!sel) return;
            sel.innerHTML = '<option value="">— Select exit node —</option>' + exitNodesCache.filter(n => n.is_enabled).map(n => '<option value="' + n.id + '">' + (n.label || n.device_name || n.id) + '</option>').join('');
        }
        async function openDeviceActionsModal(deviceId) {
            actionDevice = machines.find(x => x.id === deviceId); if (!actionDevice) return;
            document.getElementById('action-device-id').value = deviceId;
            document.getElementById('device-actions-title').textContent = actionDevice.device_name || 'Wake / manage';
            document.getElementById('action-device-status').textContent = (actionDevice.status || 'offline') + ' • ' + (actionDevice.platform || '');
            document.getElementById('action-device-phone').value = actionDevice.device_phone || '';
            await loadExitNodesForActions();
            document.getElementById('modal-device-actions').classList.remove('hidden');
        }
        function closeDeviceActionsModal() { document.getElementById('modal-device-actions')?.classList.add('hidden'); actionDevice = null; }
        async function saveDevicePhone() {
            const id = document.getElementById('action-device-id').value;
            await postAPI('/api/devices/device-phone', { device_id: id, device_phone: document.getElementById('action-device-phone').value.trim() });
            alertNotification('Phone saved.');
        }
        async function queueWakeOnly() {
            const id = document.getElementById('action-device-id').value;
            const data = await postAPI('/api/devices/wake', { target_device_id: id, action: 'connect' });
            alertNotification(data.message || 'Wake queued.');
        }
        async function wakeAndConnect() { await queueWakeOnly(); }
        async function callAndWake() {
            const id = document.getElementById('action-device-id').value;
            const phone = document.getElementById('action-device-phone').value.trim() || actionDevice?.device_phone || '';
            await postAPI('/api/devices/wake', { target_device_id: id, action: 'connect' });
            if (phone) window.location.href = 'tel:' + phone.replace(/\\s/g, '');
            alertNotification('Wake queued.');
        }
        async function adminRouteDevice(mode) {
            const id = document.getElementById('action-device-id').value;
            const payload = { device_id: id, mode };
            if (mode === 'exit-via') { payload.exit_node_id = document.getElementById('action-exit-node').value; if (!payload.exit_node_id) { alertNotification('Select exit node'); return; } }
            const data = await postAPI('/api/admin/route-device', payload);
            alertNotification(data.message || 'Updated'); await fetchDevices();
        }

`;
html = html.replace('        // Initialize: REST poll + WebSocket', wakeJs + '        // Initialize: REST poll + WebSocket');

html = html.replace(
`        window.onload = async function() {
            showDesktopBackButton();
            await fetchUserProfile();
            await fetchDevices();
            initWebSocket();
            setInterval(fetchDevices, 8000);
        };`,
`        window.onload = async function() {
            applyAssetPaths();
            if (isEmbeddedAdmin()) document.body.classList.add('embedded-admin');
            try { if (localStorage.getItem('mscale_banner_min') === '1') minimizeBanner(); } catch (_) {}
            showDesktopBackButton();
            await fetchUserProfile();
            if (!currentUser) return;
            await fetchDevices();
            initWebSocket();
            setInterval(fetchDevices, 8000);
        };`);

fs.writeFileSync(p, html);
console.log('Patched dashboard.html', html.length, 'bytes');
