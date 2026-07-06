/* ═══════════════════════════════════════════════════════════════
   GoChat — Client Application
   ═══════════════════════════════════════════════════════════════ */

// ─── API Configuration ────────────────────────────────────────
const API_BASE = window.location.origin;
const WS_BASE = `ws://${window.location.host}`;

// ─── Application State ───────────────────────────────────────
let state = {
  token: null,
  refreshToken: null,
  user: null,
  ws: null,
  currentRoom: null,
  rooms: [],
  typingTimeout: null,
  reconnectAttempts: 0,
  maxReconnectAttempts: 10,
  reconnectTimer: null,
  typingUsers: {}       // { roomId: { username: timeoutId } }
};

// ─── DOM Elements ────────────────────────────────────────────
const $ = (sel) => document.querySelector(sel);
const $$ = (sel) => document.querySelectorAll(sel);

const dom = {
  authView:          $('#auth-view'),
  chatView:          $('#chat-view'),
  loginForm:         $('#login-form'),
  registerForm:      $('#register-form'),
  messagesContainer: $('#messages-container'),
  messagesWrapper:   $('#messages-wrapper'),
  chatEmpty:         $('#chat-empty'),
  messageForm:       $('#message-form'),
  messageInput:      $('#message-input'),
  sendBtn:           $('#send-btn'),
  roomsList:         $('#rooms-list'),
  headerUsername:    $('#header-username'),
  headerRoomName:   $('#header-room-name'),
  headerRoomDesc:   $('#header-room-desc'),
  typingIndicator:   $('#typing-indicator'),
  typingText:        $('#typing-text'),
  connectionBar:     $('#connection-bar'),
  connectionText:    $('#connection-bar-text'),
  modalOverlay:      $('#modal-overlay'),
  modalTitle:        $('#modal-title'),
  createRoomForm:    $('#create-room-form'),
  joinRoomForm:      $('#join-room-form'),
  toastContainer:    $('#toast-container'),
  sidebar:           $('#sidebar'),
};


/* ═══════════════════════════════════════════════════════════════
   INITIALIZATION
   ═══════════════════════════════════════════════════════════════ */

document.addEventListener('DOMContentLoaded', () => {
  // Check for stored session
  const savedToken = localStorage.getItem('gochat_token');
  const savedRefresh = localStorage.getItem('gochat_refresh_token');
  const savedUser = localStorage.getItem('gochat_user');

  if (savedToken && savedUser) {
    try {
      state.token = savedToken;
      state.refreshToken = savedRefresh;
      state.user = JSON.parse(savedUser);
      enterChat();
    } catch {
      clearSession();
    }
  }

  // Auto-resize textarea
  dom.messageInput.addEventListener('input', autoResizeTextarea);

  // Enable / disable send button based on input
  dom.messageInput.addEventListener('input', () => {
    dom.sendBtn.disabled = !dom.messageInput.value.trim();
  });
});


/* ═══════════════════════════════════════════════════════════════
   AUTH
   ═══════════════════════════════════════════════════════════════ */

/**
 * Switch between Login and Register tabs.
 */
function switchAuthTab(tab) {
  $$('.auth-tab').forEach(t => t.classList.toggle('active', t.dataset.tab === tab));
  $('.auth-tabs').dataset.active = tab;

  dom.loginForm.style.display    = tab === 'login' ? 'flex' : 'none';
  dom.registerForm.style.display = tab === 'register' ? 'flex' : 'none';
}

/**
 * Handle login form submission.
 */
async function handleLogin(e) {
  e.preventDefault();
  const btn = $('#login-btn');
  const username = $('#login-username').value.trim();
  const password = $('#login-password').value;

  if (!username || !password) return;

  setLoading(btn, true);
  try {
    const data = await apiPost('/api/auth/login', { username, password });

    state.token = data.tokens.access_token;
    state.refreshToken = data.tokens.refresh_token;
    state.user = data.user;
    saveSession();
    enterChat();
    showToast('Welcome back, ' + state.user.username + '!', 'success');
  } catch (err) {
    showToast(err.message || 'Login failed', 'error');
  } finally {
    setLoading(btn, false);
  }
}

/**
 * Handle register form submission.
 */
async function handleRegister(e) {
  e.preventDefault();
  const btn = $('#register-btn');
  const username = $('#register-username').value.trim();
  const email    = $('#register-email').value.trim();
  const password = $('#register-password').value;

  if (!username || !email || !password) return;

  setLoading(btn, true);
  try {
    await apiPost('/api/auth/register', { username, email, password });
    showToast('Account created! Please sign in.', 'success');
    switchAuthTab('login');
    $('#login-username').value = username;
    $('#login-password').focus();
  } catch (err) {
    showToast(err.message || 'Registration failed', 'error');
  } finally {
    setLoading(btn, false);
  }
}

/**
 * Handle logout.
 */
async function handleLogout() {
  try {
    await apiPost('/api/auth/logout', {});
  } catch {
    // Ignore errors, log out anyway
  }
  disconnectWS();
  clearSession();
  switchView('auth');
  showToast('Logged out', 'info');
}

/**
 * Enter the chat view — load rooms and connect WebSocket.
 */
function enterChat() {
  dom.headerUsername.textContent = state.user.username;
  switchView('chat');
  loadRooms();
  connectWebSocket();
}


/* ═══════════════════════════════════════════════════════════════
   ROOMS
   ═══════════════════════════════════════════════════════════════ */

/**
 * Fetch all rooms from the server and render them.
 */
async function loadRooms() {
  try {
    const data = await apiGet('/api/rooms');
    state.rooms = data.rooms || [];
    renderRoomList();
  } catch (err) {
    showToast('Failed to load rooms: ' + err.message, 'error');
  }
}

/**
 * Create a new room.
 */
async function handleCreateRoom(e) {
  e.preventDefault();
  const name = $('#room-name').value.trim();
  const description = $('#room-description').value.trim();

  if (!name) return;

  try {
    const data = await apiPost('/api/rooms', { name, description });
    closeModal();
    showToast('Room "' + name + '" created!', 'success');
    await loadRooms();
    // Select the new room if the response includes it
    if (data.room && data.room.id) {
      selectRoom(data.room.id);
    }
  } catch (err) {
    showToast(err.message || 'Failed to create room', 'error');
  }
}

/**
 * Join an existing room by ID.
 */
async function handleJoinRoom(e) {
  e.preventDefault();
  const roomId = $('#join-room-id').value.trim();

  if (!roomId) return;

  try {
    await apiPost(`/api/rooms/${roomId}/join`, {});
    closeModal();
    showToast('Joined room successfully!', 'success');
    await loadRooms();
    selectRoom(roomId);
  } catch (err) {
    showToast(err.message || 'Failed to join room', 'error');
  }
}

/**
 * Select a room to view its messages.
 */
async function selectRoom(roomId) {
  state.currentRoom = parseInt(roomId, 10);

  // Update sidebar highlight
  $$('.room-item').forEach(el => {
    el.classList.toggle('active', el.dataset.roomId === String(roomId));
  });

  // Find room info
  const room = state.rooms.find(r => String(r.id) === String(roomId));
  dom.headerRoomName.textContent = room ? `# ${room.name}` : 'Chat';
  dom.headerRoomDesc.textContent = room ? (room.description || '') : '';

  // Show messages area, hide empty state
  dom.chatEmpty.style.display = 'none';
  dom.messagesWrapper.style.display = 'flex';

  // Clear existing messages
  dom.messagesContainer.innerHTML = '';

  // Load messages
  await loadMessages(roomId);

  // Close sidebar on mobile
  if (window.innerWidth <= 768) {
    toggleSidebar(false);
  }

  // Focus input
  dom.messageInput.focus();
}

/**
 * Load chat messages for a specific room.
 */
async function loadMessages(roomId) {
  try {
    const data = await apiGet(`/api/rooms/${roomId}/messages?limit=50`);
    const messages = data.messages || [];

    dom.messagesContainer.innerHTML = '';

    if (messages.length === 0) {
      dom.messagesContainer.innerHTML = `
        <div class="chat-empty-content" style="margin: auto;">
          <p style="color: var(--text-muted); font-size: 0.9rem;">No messages yet. Start the conversation!</p>
        </div>
      `;
      return;
    }

    let lastDate = null;
    messages.forEach(msg => {
      // Insert date separators
      const msgDate = formatDateSeparator(msg.created_at);
      if (msgDate !== lastDate) {
        const sep = document.createElement('div');
        sep.className = 'date-separator';
        sep.textContent = msgDate;
        dom.messagesContainer.appendChild(sep);
        lastDate = msgDate;
      }
      dom.messagesContainer.appendChild(createMessageElement(msg));
    });

    scrollToBottom();
  } catch (err) {
    showToast('Failed to load messages: ' + err.message, 'error');
  }
}

/**
 * Render room list in the sidebar.
 */
function renderRoomList(filter = '') {
  const filtered = filter
    ? state.rooms.filter(r => r.name.toLowerCase().includes(filter.toLowerCase()))
    : state.rooms;

  if (filtered.length === 0) {
    dom.roomsList.innerHTML = `
      <div class="rooms-empty">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="48" height="48">
          <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>
        </svg>
        <p>${filter ? 'No matching rooms' : 'No rooms yet'}</p>
        <span>${filter ? 'Try a different search' : 'Create or join a room to start chatting'}</span>
      </div>
    `;
    return;
  }

  dom.roomsList.innerHTML = filtered.map(room => `
    <div class="room-item ${String(room.id) === String(state.currentRoom) ? 'active' : ''}"
         data-room-id="${room.id}"
         onclick="selectRoom('${room.id}')">
      <div class="room-item-icon">${room.name.charAt(0)}</div>
      <div class="room-item-info">
        <div class="room-item-name"># ${escapeHtml(room.name)}</div>
        ${room.description ? `<div class="room-item-desc">${escapeHtml(room.description)}</div>` : ''}
      </div>
    </div>
  `).join('');
}

/**
 * Filter rooms by search query.
 */
function filterRooms(query) {
  renderRoomList(query);
}


/* ═══════════════════════════════════════════════════════════════
   WEBSOCKET
   ═══════════════════════════════════════════════════════════════ */

/**
 * Establish WebSocket connection to the server.
 */
function connectWebSocket() {
  if (state.ws) {
    state.ws.close();
  }

  try {
    const wsUrl = WS_BASE + '/ws?token=' + encodeURIComponent(state.token);
    state.ws = new WebSocket(wsUrl);

    state.ws.onopen = () => {
      console.log('[WS] Connected');
      state.reconnectAttempts = 0;
      hideConnectionBar();
    };

    state.ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data);
        handleWSMessage(msg);
      } catch (err) {
        console.error('[WS] Failed to parse message:', err);
      }
    };

    state.ws.onclose = (event) => {
      console.log('[WS] Disconnected', event.code, event.reason);
      state.ws = null;
      if (state.token) {
        // Only attempt reconnect if we're still logged in
        scheduleReconnect();
      }
    };

    state.ws.onerror = (err) => {
      console.error('[WS] Error:', err);
    };
  } catch (err) {
    console.error('[WS] Connection failed:', err);
    scheduleReconnect();
  }
}

/**
 * Disconnect WebSocket.
 */
function disconnectWS() {
  if (state.reconnectTimer) {
    clearTimeout(state.reconnectTimer);
    state.reconnectTimer = null;
  }
  if (state.ws) {
    state.ws.close();
    state.ws = null;
  }
  state.reconnectAttempts = 0;
}

/**
 * Schedule a reconnect attempt with exponential backoff.
 */
function scheduleReconnect() {
  if (state.reconnectAttempts >= state.maxReconnectAttempts) {
    showConnectionBar('Connection lost. Please refresh the page.');
    showToast('Unable to reconnect. Please refresh.', 'error');
    return;
  }

  state.reconnectAttempts++;
  const delay = Math.min(1000 * Math.pow(2, state.reconnectAttempts - 1), 30000);
  showConnectionBar(`Reconnecting in ${Math.round(delay / 1000)}s... (attempt ${state.reconnectAttempts})`);

  state.reconnectTimer = setTimeout(() => {
    showConnectionBar('Reconnecting...');
    connectWebSocket();
  }, delay);
}

/**
 * Handle incoming WebSocket messages.
 */
function handleWSMessage(msg) {
  switch (msg.type) {
    case 'chat_message':
      handleChatMessage(msg);
      break;

    case 'typing':
      handleTypingEvent(msg);
      break;

    case 'stop_typing':
      handleStopTypingEvent(msg);
      break;

    case 'system':
      handleSystemMessage(msg);
      break;

    case 'error':
      showToast(msg.content || 'An error occurred', 'error');
      break;

    default:
      console.log('[WS] Unknown message type:', msg.type, msg);
  }
}

/**
 * Handle incoming chat message.
 */
function handleChatMessage(msg) {
  // Only display if we're viewing that room
  if (String(msg.room_id) !== String(state.currentRoom)) return;

  // Clear typing for this user
  removeTypingUser(msg.room_id, msg.sender);

  const msgEl = createMessageElement({
    id: msg.data?.message_id,
    content: msg.content,
    sender_id: msg.sender_id,
    sender: msg.sender,
    created_at: msg.timestamp || new Date().toISOString()
  });

  dom.messagesContainer.appendChild(msgEl);
  scrollToBottom(true);
}

/**
 * Handle incoming system message.
 */
function handleSystemMessage(msg) {
  if (String(msg.room_id) !== String(state.currentRoom)) return;

  const msgEl = createMessageElement({
    content: msg.content,
    sender: 'system',
    created_at: msg.timestamp || new Date().toISOString()
  });

  dom.messagesContainer.appendChild(msgEl);
  scrollToBottom(true);
}

/**
 * Handle typing event from another user.
 */
function handleTypingEvent(msg) {
  if (String(msg.room_id) !== String(state.currentRoom)) return;
  if (String(msg.sender_id) === String(state.user.id)) return;

  addTypingUser(msg.room_id, msg.sender);
}

/**
 * Handle stop_typing event.
 */
function handleStopTypingEvent(msg) {
  removeTypingUser(msg.room_id, msg.sender);
}


/* ═══════════════════════════════════════════════════════════════
   TYPING INDICATOR
   ═══════════════════════════════════════════════════════════════ */

/**
 * Add a user to the typing list.
 */
function addTypingUser(roomId, username) {
  if (!state.typingUsers[roomId]) {
    state.typingUsers[roomId] = {};
  }

  // Clear any existing timeout
  if (state.typingUsers[roomId][username]) {
    clearTimeout(state.typingUsers[roomId][username]);
  }

  // Auto-remove after 3 seconds
  state.typingUsers[roomId][username] = setTimeout(() => {
    removeTypingUser(roomId, username);
  }, 3000);

  updateTypingUI();
}

/**
 * Remove a user from the typing list.
 */
function removeTypingUser(roomId, username) {
  if (state.typingUsers[roomId] && state.typingUsers[roomId][username]) {
    clearTimeout(state.typingUsers[roomId][username]);
    delete state.typingUsers[roomId][username];
    updateTypingUI();
  }
}

/**
 * Update the typing indicator in the UI.
 */
function updateTypingUI() {
  const users = state.typingUsers[state.currentRoom]
    ? Object.keys(state.typingUsers[state.currentRoom])
    : [];

  if (users.length === 0) {
    dom.typingIndicator.hidden = true;
    return;
  }

  dom.typingIndicator.hidden = false;

  if (users.length === 1) {
    dom.typingText.textContent = `${users[0]} is typing`;
  } else if (users.length === 2) {
    dom.typingText.textContent = `${users[0]} and ${users[1]} are typing`;
  } else {
    dom.typingText.textContent = `${users.length} people are typing`;
  }
}


/* ═══════════════════════════════════════════════════════════════
   MESSAGING
   ═══════════════════════════════════════════════════════════════ */

/**
 * Handle sending a message.
 */
function handleSendMessage(e) {
  e.preventDefault();
  const text = dom.messageInput.value.trim();

  if (!text || !state.currentRoom || !state.ws) return;

  // Send via WebSocket
  state.ws.send(JSON.stringify({
    type: 'chat_message',
    room_id: state.currentRoom,
    content: text
  }));

  // Send stop_typing
  sendStopTyping();

  // Clear input
  dom.messageInput.value = '';
  dom.sendBtn.disabled = true;
  autoResizeTextarea();
  dom.messageInput.focus();
}

/**
 * Handle Enter key in message input (Shift+Enter for new line).
 */
function handleMessageKeydown(e) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault();
    handleSendMessage(e);
  }
}

/**
 * Handle typing detection — send typing / stop_typing events.
 */
function handleTyping() {
  if (!state.currentRoom || !state.ws) return;

  // Send typing event
  if (!state.typingTimeout) {
    state.ws.send(JSON.stringify({
      type: 'typing',
      room_id: state.currentRoom
    }));
  }

  // Reset stop_typing timer
  clearTimeout(state.typingTimeout);
  state.typingTimeout = setTimeout(() => {
    sendStopTyping();
  }, 2000);
}

/**
 * Send stop_typing event.
 */
function sendStopTyping() {
  if (state.typingTimeout) {
    clearTimeout(state.typingTimeout);
    state.typingTimeout = null;
  }
  if (state.ws && state.currentRoom) {
    state.ws.send(JSON.stringify({
      type: 'stop_typing',
      room_id: state.currentRoom
    }));
  }
}


/* ═══════════════════════════════════════════════════════════════
   UI HELPERS
   ═══════════════════════════════════════════════════════════════ */

/**
 * Create a DOM element for a single message.
 */
function createMessageElement(msg) {
  const div = document.createElement('div');
  const isOwn = String(msg.sender_id) === String(state.user.id);
  const isSystem = msg.sender === 'system';

  if (isSystem) {
    div.className = 'message message--system';
    div.innerHTML = `<div class="message-bubble">${escapeHtml(msg.content)}</div>`;
  } else {
    div.className = `message ${isOwn ? 'message--own' : 'message--other'}`;
    div.innerHTML = `
      ${!isOwn ? `<span class="message-sender">${escapeHtml(msg.sender || 'Unknown')}</span>` : ''}
      <div class="message-bubble">${escapeHtml(msg.content)}</div>
      <span class="message-time">${formatTime(msg.created_at)}</span>
    `;
  }

  return div;
}

/**
 * Switch between auth and chat views.
 */
function switchView(view) {
  dom.authView.style.display = view === 'auth' ? 'flex' : 'none';
  dom.chatView.style.display = view === 'chat' ? 'block' : 'none';
}

/**
 * Toggle sidebar visibility (mobile).
 */
function toggleSidebar(force) {
  const isOpen = dom.sidebar.classList.contains('open');
  const shouldOpen = force !== undefined ? force : !isOpen;

  dom.sidebar.classList.toggle('open', shouldOpen);
  $('.sidebar-overlay').classList.toggle('active', shouldOpen);
}

/**
 * Open modal for create / join room.
 */
function openModal(type) {
  dom.modalOverlay.hidden = false;

  if (type === 'create') {
    dom.modalTitle.textContent = 'Create Room';
    dom.createRoomForm.style.display = 'flex';
    dom.joinRoomForm.style.display = 'none';
    dom.createRoomForm.reset();
    setTimeout(() => $('#room-name').focus(), 100);
  } else {
    dom.modalTitle.textContent = 'Join Room';
    dom.createRoomForm.style.display = 'none';
    dom.joinRoomForm.style.display = 'flex';
    dom.joinRoomForm.reset();
    setTimeout(() => $('#join-room-id').focus(), 100);
  }
}

/**
 * Close modal.
 */
function closeModal() {
  dom.modalOverlay.hidden = true;
}

/**
 * Show connection status bar.
 */
function showConnectionBar(text) {
  dom.connectionBar.hidden = false;
  dom.connectionText.textContent = text;
}

/**
 * Hide connection status bar.
 */
function hideConnectionBar() {
  dom.connectionBar.hidden = true;
}

/**
 * Scroll messages to bottom.
 */
function scrollToBottom(smooth = false) {
  requestAnimationFrame(() => {
    dom.messagesContainer.scrollTop = dom.messagesContainer.scrollHeight;
  });
}

/**
 * Auto-resize textarea to fit content.
 */
function autoResizeTextarea() {
  const el = dom.messageInput;
  el.style.height = 'auto';
  el.style.height = Math.min(el.scrollHeight, 120) + 'px';
}

/**
 * Set a button into loading state.
 */
function setLoading(btn, loading) {
  const text   = btn.querySelector('.btn-text');
  const loader = btn.querySelector('.btn-loader');
  if (text)   text.hidden   = loading;
  if (loader) loader.hidden = !loading;
  btn.disabled = loading;
}


/* ═══════════════════════════════════════════════════════════════
   TOAST NOTIFICATIONS
   ═══════════════════════════════════════════════════════════════ */

/**
 * Show a toast notification.
 * @param {string} message
 * @param {'success'|'error'|'warning'|'info'} type
 * @param {number} duration  - ms before auto-dismiss (default 4000)
 */
function showToast(message, type = 'info', duration = 4000) {
  const toast = document.createElement('div');
  toast.className = `toast toast--${type}`;

  const icons = {
    success: '<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="#22c55e" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>',
    error:   '<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="#ef4444" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>',
    warning: '<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="#f59e0b" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>',
    info:    '<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="#3b82f6" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg>'
  };

  toast.innerHTML = `${icons[type] || icons.info}<span>${escapeHtml(message)}</span>`;
  dom.toastContainer.appendChild(toast);

  // Auto-dismiss
  setTimeout(() => {
    toast.classList.add('removing');
    toast.addEventListener('animationend', () => toast.remove());
  }, duration);
}


/* ═══════════════════════════════════════════════════════════════
   API HELPERS
   ═══════════════════════════════════════════════════════════════ */

/**
 * Make an authenticated GET request.
 */
async function apiGet(path) {
  const res = await fetch(API_BASE + path, {
    headers: authHeaders()
  });
  return handleResponse(res);
}

/**
 * Make an authenticated POST request with JSON body.
 */
async function apiPost(path, body) {
  const res = await fetch(API_BASE + path, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...authHeaders()
    },
    body: JSON.stringify(body)
  });
  return handleResponse(res);
}

/**
 * Build auth headers if we have a token.
 */
function authHeaders() {
  if (state.token) {
    return { 'Authorization': `Bearer ${state.token}` };
  }
  return {};
}

/**
 * Handle fetch response: parse JSON, throw on error.
 */
async function handleResponse(res) {
  let data;
  try {
    data = await res.json();
  } catch {
    data = {};
  }

  if (!res.ok) {
    throw new Error(data.error || data.message || `Request failed (${res.status})`);
  }

  return data;
}


/* ═══════════════════════════════════════════════════════════════
   SESSION HELPERS
   ═══════════════════════════════════════════════════════════════ */

function saveSession() {
  localStorage.setItem('gochat_token', state.token);
  localStorage.setItem('gochat_refresh_token', state.refreshToken);
  localStorage.setItem('gochat_user', JSON.stringify(state.user));
}

function clearSession() {
  state.token = null;
  state.refreshToken = null;
  state.user = null;
  state.currentRoom = null;
  state.rooms = [];
  localStorage.removeItem('gochat_token');
  localStorage.removeItem('gochat_refresh_token');
  localStorage.removeItem('gochat_user');
}


/* ═══════════════════════════════════════════════════════════════
   FORMATTING HELPERS
   ═══════════════════════════════════════════════════════════════ */

/**
 * Format a timestamp to a readable time string.
 */
function formatTime(isoString) {
  if (!isoString) return '';
  const date = new Date(isoString);
  const now = new Date();

  const isToday = date.toDateString() === now.toDateString();
  const yesterday = new Date(now);
  yesterday.setDate(yesterday.getDate() - 1);
  const isYesterday = date.toDateString() === yesterday.toDateString();

  const time = date.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });

  if (isToday) return time;
  if (isYesterday) return `Yesterday ${time}`;
  return date.toLocaleDateString([], { month: 'short', day: 'numeric' }) + ' ' + time;
}

/**
 * Format a date for date separators between messages.
 */
function formatDateSeparator(isoString) {
  if (!isoString) return '';
  const date = new Date(isoString);
  const now = new Date();

  const isToday = date.toDateString() === now.toDateString();
  if (isToday) return 'Today';

  const yesterday = new Date(now);
  yesterday.setDate(yesterday.getDate() - 1);
  if (date.toDateString() === yesterday.toDateString()) return 'Yesterday';

  return date.toLocaleDateString([], { weekday: 'long', month: 'long', day: 'numeric', year: 'numeric' });
}

/**
 * Escape HTML to prevent XSS.
 */
function escapeHtml(str) {
  if (!str) return '';
  const div = document.createElement('div');
  div.textContent = str;
  return div.innerHTML;
}

/* ─── Close modal on Escape key ───────────────────────────── */
document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape') {
    closeModal();
  }
});
