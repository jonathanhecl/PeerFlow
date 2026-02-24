import './style.css';
import './app.css';

import { GetPeerID, GetPeers, GetNoteContent, UpdateNote, GetNetworkInfo, ConnectToPeer } from '../wailsjs/go/main/App';
import { EventsOn } from '../wailsjs/runtime/runtime';

// State
let peerID = '';
let debounceTimer = null;
let isRemoteUpdate = false;

// Render the UI shell
document.querySelector('#app').innerHTML = `
    <aside class="sidebar">
        <div class="sidebar-header">
            <div class="logo-icon">⚡</div>
            <h1>PeerFlow</h1>
        </div>

        <div class="info-card">
            <div class="label">Your Peer ID</div>
            <div class="value" id="my-peer-id">Loading...</div>
            <div class="net-detail" id="my-address">Loading...</div>
        </div>

        <button class="toggle-advanced-btn" id="btn-toggle-advanced">⚙ Network Settings</button>

        <div class="advanced-panel hidden" id="advanced-panel">
            <div class="info-card">
                <div class="label">All Local IPs</div>
                <div class="net-detail" id="net-ips">Scanning...</div>
                <div class="net-detail" id="net-port">TCP Port: —</div>
            </div>

            <div class="connect-box">
                <div class="label">Manual Connect</div>
                <div class="connect-row">
                    <input type="text" id="manual-ip" placeholder="192.168.1.x:port" />
                    <button id="btn-connect">→</button>
                </div>
                <div class="connect-status" id="connect-status"></div>
            </div>
        </div>

        <div class="peer-section">
            <div class="section-title">
                Connected Peers
                <span class="peer-count-badge" id="peer-count">0</span>
            </div>
            <ul class="peer-list" id="peer-list">
                <li class="no-peers" id="no-peers-msg">Scanning LAN...</li>
            </ul>
        </div>

        <div class="sidebar-footer">PeerFlow v1.0.0</div>
    </aside>

    <main class="editor-area">
        <div class="editor-toolbar">
            <div class="title">📝 Shared Notepad</div>
            <div class="sync-indicator">
                <span class="sync-dot"></span>
                Live Sync
            </div>
        </div>
        <div class="editor-container">
            <textarea id="note-editor" placeholder="Start typing... your notes will sync across all connected peers in real-time."></textarea>
        </div>
    </main>
`;

// DOM references
const peerIdEl = document.getElementById('my-peer-id');
const myAddressEl = document.getElementById('my-address');
const peerListEl = document.getElementById('peer-list');
const peerCountEl = document.getElementById('peer-count');
const editor = document.getElementById('note-editor');
const netIPs = document.getElementById('net-ips');
const netPort = document.getElementById('net-port');
const manualIpInput = document.getElementById('manual-ip');
const btnConnect = document.getElementById('btn-connect');
const connectStatus = document.getElementById('connect-status');
const btnToggleAdvanced = document.getElementById('btn-toggle-advanced');
const advancedPanel = document.getElementById('advanced-panel');

// Toggle advanced panel
btnToggleAdvanced.addEventListener('click', () => {
    advancedPanel.classList.toggle('hidden');
    btnToggleAdvanced.classList.toggle('active');
});

// Initialize
async function init() {
    try {
        peerID = await GetPeerID();
        peerIdEl.textContent = peerID;

        const content = await GetNoteContent();
        if (content) {
            editor.value = content;
        }

        const peers = await GetPeers();
        updatePeerList(peers);

        // Load network diagnostics
        await refreshNetworkInfo();
    } catch (err) {
        console.error('Init error:', err);
    }
}

// Refresh network info
async function refreshNetworkInfo() {
    try {
        const info = await GetNetworkInfo();
        // Show primary IP:port next to peer ID
        if (info.localIPs && info.localIPs.length > 0) {
            myAddressEl.textContent = info.localIPs[0] + ':' + info.tcpPort;
            netIPs.textContent = info.localIPs.join(', ');
        } else {
            myAddressEl.textContent = 'No network detected';
            netIPs.textContent = 'None detected';
        }
        netPort.textContent = 'TCP Port: ' + info.tcpPort;
    } catch (err) {
        console.error('NetworkInfo error:', err);
    }
}

// Refresh network info periodically
setInterval(refreshNetworkInfo, 10000);

// Update peer list UI
function updatePeerList(peers) {
    peerCountEl.textContent = peers.length;

    if (peers.length === 0) {
        peerListEl.innerHTML = '<li class="no-peers">No peers found yet...</li>';
        return;
    }

    peerListEl.innerHTML = peers.map(id => `
        <li class="peer-item">
            <span class="peer-dot"></span>
            <span class="peer-id">${escapeHtml(id)}</span>
        </li>
    `).join('');
}

// Show a toast notification
function showToast(message) {
    const existing = document.querySelector('.toast');
    if (existing) existing.remove();

    const toast = document.createElement('div');
    toast.className = 'toast';
    toast.textContent = message;
    document.body.appendChild(toast);
    setTimeout(() => toast.remove(), 3000);
}

// Escape HTML to prevent XSS
function escapeHtml(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

// Editor input handler (debounced)
editor.addEventListener('input', () => {
    if (isRemoteUpdate) return;

    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => {
        const content = editor.value;
        UpdateNote(content).catch(err => console.error('UpdateNote error:', err));
    }, 300);
});

// Manual connect button
btnConnect.addEventListener('click', async () => {
    const address = manualIpInput.value.trim();
    if (!address) return;

    connectStatus.textContent = 'Connecting...';
    connectStatus.className = 'connect-status connecting';

    try {
        const errMsg = await ConnectToPeer(address);
        if (errMsg) {
            connectStatus.textContent = '✗ ' + errMsg;
            connectStatus.className = 'connect-status error';
        } else {
            connectStatus.textContent = '✓ Connected!';
            connectStatus.className = 'connect-status success';
            manualIpInput.value = '';
            showToast('🔗 Connected to ' + address);
        }
    } catch (err) {
        connectStatus.textContent = '✗ ' + err;
        connectStatus.className = 'connect-status error';
    }

    setTimeout(() => { connectStatus.textContent = ''; }, 5000);
});

// Allow Enter key to connect
manualIpInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') btnConnect.click();
});

// Listen for remote note updates
EventsOn('onNoteReceived', (data) => {
    if (data && typeof data.content === 'string') {
        const start = editor.selectionStart;
        const end = editor.selectionEnd;
        const hadFocus = document.activeElement === editor;

        isRemoteUpdate = true;
        editor.value = data.content;
        isRemoteUpdate = false;

        if (hadFocus) {
            editor.setSelectionRange(
                Math.min(start, data.content.length),
                Math.min(end, data.content.length)
            );
        }

        showToast('📡 Note updated from peer');
    }
});

// Listen for peer list updates
EventsOn('onPeerUpdate', (peers) => {
    if (Array.isArray(peers)) {
        updatePeerList(peers);
    }
});

// Kick it off
init();
