import './style.css';
import './app.css';

import { GetPeerID, GetPeers, GetNoteContent, UpdateNote } from '../wailsjs/go/main/App';
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
const peerListEl = document.getElementById('peer-list');
const peerCountEl = document.getElementById('peer-count');
const noPeersMsg = document.getElementById('no-peers-msg');
const editor = document.getElementById('note-editor');

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
    } catch (err) {
        console.error('Init error:', err);
    }
}

// Update peer list UI
function updatePeerList(peers) {
    peerCountEl.textContent = peers.length;

    if (peers.length === 0) {
        peerListEl.innerHTML = '<li class="no-peers" id="no-peers-msg">No peers found yet...</li>';
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

// Listen for remote note updates
EventsOn('onNoteReceived', (data) => {
    if (data && typeof data.content === 'string') {
        // Preserve cursor position
        const start = editor.selectionStart;
        const end = editor.selectionEnd;
        const hadFocus = document.activeElement === editor;

        isRemoteUpdate = true;
        editor.value = data.content;
        isRemoteUpdate = false;

        // Restore cursor if editor was focused
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
