// Browser (Chrome) notification support with a master preference and
// per-board per-event triggers. Events are sourced from BoardStream (SSE).

const PREF_KEY = 'lwts-notifications';
const ICON_URL = '/favicon.svg';

const EVENT_KEYS = ['on_create', 'on_transition', 'on_done', 'on_closed'];

function isSupported() {
  return typeof window !== 'undefined' && 'Notification' in window;
}

function currentPermission() {
  if (!isSupported()) return 'unsupported';
  return Notification.permission;
}

function getPref() {
  try {
    const raw = localStorage.getItem(PREF_KEY);
    const parsed = raw ? JSON.parse(raw) : {};
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch { return {}; }
}

function setPref(next) {
  try { localStorage.setItem(PREF_KEY, JSON.stringify(next || {})); } catch {}
}

function isMasterEnabled() {
  return !!getPref().enabled;
}

function setMasterEnabled(enabled) {
  const p = getPref();
  p.enabled = !!enabled;
  setPref(p);
}

function canNotify() {
  if (!isSupported()) return false;
  if (Notification.permission !== 'granted') return false;
  return isMasterEnabled();
}

async function requestPermission() {
  if (!isSupported()) return 'unsupported';
  if (Notification.permission === 'granted') return 'granted';
  if (Notification.permission === 'denied') return 'denied';
  try {
    const result = await Notification.requestPermission();
    return result;
  } catch {
    return Notification.permission;
  }
}

function normalizeTriggers(boardSettings) {
  const s = boardSettings || {};
  const tri = s.triggers || {};
  const legacy = s.webhooks || {};
  const legacyMap = {
    on_create: legacy.on_create,
    on_transition: legacy.on_transition,
    on_done: legacy.on_complete,
    on_closed: undefined,
  };
  const out = {};
  EVENT_KEYS.forEach(k => {
    const t = tri[k] || {};
    out[k] = {
      notify: t.notify === true,
      webhook: typeof t.webhook === 'string' ? t.webhook : (legacyMap[k] || ''),
    };
  });
  return out;
}

function getBoard(boardId) {
  return (window.boardList || []).find(b => b.id === boardId) || null;
}

function getBoardTriggers(boardId) {
  const board = getBoard(boardId);
  if (!board) return normalizeTriggers({});
  const s = window.parseBoardSettings ? window.parseBoardSettings(board.settings) : {};
  return normalizeTriggers(s);
}

function getColumn(boardId, colId) {
  const board = getBoard(boardId);
  if (!board) return null;
  try {
    const cols = JSON.parse(board.columns || '[]');
    const idx = cols.findIndex(c => c.id === colId);
    if (idx === -1) return null;
    const col = cols[idx];
    const type = col.type || (idx === 0 ? 'start' : idx === cols.length - 1 ? 'done' : 'active');
    return { ...col, type, index: idx, total: cols.length };
  } catch { return null; }
}

function columnLabel(boardId, colId) {
  if (colId === 'cleared') return 'Closed';
  const col = getColumn(boardId, colId);
  return col ? col.label : colId;
}

function columnKind(boardId, colId) {
  if (colId === 'cleared') return 'closed';
  const col = getColumn(boardId, colId);
  return col ? col.type : '';
}

function boardName(boardId) {
  const b = getBoard(boardId);
  return b ? b.name : '';
}

function truncate(s, max) {
  const t = (s || '').replace(/\s+/g, ' ').trim();
  return t.length > max ? t.slice(0, max - 1) + '…' : t;
}

function fire(title, opts) {
  if (!canNotify()) return null;
  const options = opts || {};
  // Skip if the user is already focused on the tab — they don't need a ping.
  if (options.onlyWhenHidden !== false) {
    const visible = typeof document !== 'undefined' && !document.hidden;
    const focused = typeof document !== 'undefined' && typeof document.hasFocus === 'function' ? document.hasFocus() : false;
    if (visible && focused) return null;
  }
  try {
    const n = new Notification(title, {
      icon: ICON_URL,
      badge: ICON_URL,
      body: options.body || '',
      tag: options.tag || undefined,
      renotify: !!options.renotify,
      silent: !!options.silent,
    });
    n.onclick = () => {
      try { window.focus(); } catch {}
      n.close();
    };
    return n;
  } catch {
    return null;
  }
}

function cardTag(id) { return 'lwts-card-' + (id || ''); }

function handleCardCreated(data) {
  if (!data || !data.board_id) return;
  const t = getBoardTriggers(data.board_id);
  if (!t.on_create.notify) return;
  const board = boardName(data.board_id);
  fire('New card' + (board ? ' · ' + board : ''), {
    body: truncate(data.title || 'Untitled', 120),
    tag: cardTag(data.id),
  });
}

function handleCardMoved(data) {
  if (!data || !data.board_id) return;
  const t = getBoardTriggers(data.board_id);
  const kind = columnKind(data.board_id, data.column_id);
  const destLabel = columnLabel(data.board_id, data.column_id);
  const board = boardName(data.board_id);
  const suffix = board ? ' · ' + board : '';
  const body = truncate(data.title || 'Untitled', 120) + ' → ' + destLabel;

  if (kind === 'closed' && t.on_closed.notify) {
    fire('Card closed' + suffix, { body, tag: cardTag(data.id) });
    return;
  }
  if (kind === 'done' && t.on_done.notify) {
    fire('Card done' + suffix, { body, tag: cardTag(data.id) });
    return;
  }
  if (t.on_transition.notify) {
    fire('Card moved' + suffix, { body, tag: cardTag(data.id) });
  }
}

function handleCardsBulkMoved(data) {
  if (!data || !Array.isArray(data.cards) || data.cards.length === 0) return;
  const first = data.cards[0];
  const boardId = first.board_id;
  if (!boardId) return;
  const t = getBoardTriggers(boardId);
  const colId = data.column_id || first.column_id;
  const kind = columnKind(boardId, colId);
  const destLabel = columnLabel(boardId, colId);
  const board = boardName(boardId);
  const suffix = board ? ' · ' + board : '';
  const count = data.cards.length;
  const body = count + ' card' + (count === 1 ? '' : 's') + ' → ' + destLabel;

  if (kind === 'closed' && t.on_closed.notify) {
    fire('Cards closed' + suffix, { body, tag: 'lwts-bulk-' + boardId });
    return;
  }
  if (kind === 'done' && t.on_done.notify) {
    fire('Cards done' + suffix, { body, tag: 'lwts-bulk-' + boardId });
    return;
  }
  if (t.on_transition.notify) {
    fire('Cards moved' + suffix, { body, tag: 'lwts-bulk-' + boardId });
  }
}

function wireNotificationHandlers(boardStream) {
  if (!boardStream || !boardStream.handlers) return;
  const wrap = (key, fn) => {
    const prev = boardStream.handlers[key];
    boardStream.handlers[key] = (data) => {
      try { fn(data); } catch {}
      if (typeof prev === 'function') prev(data);
    };
  };
  wrap('onCardCreated', handleCardCreated);
  wrap('onCardMoved', handleCardMoved);
  wrap('onCardsBulkMoved', handleCardsBulkMoved);
}

function testNotification() {
  if (!isSupported()) {
    if (window.Toast) window.Toast.error('Browser notifications are not supported in this browser');
    return;
  }
  if (Notification.permission !== 'granted') {
    if (window.Toast) window.Toast.error('Notification permission is ' + Notification.permission);
    return;
  }
  try {
    const n = new Notification('LWTS notifications enabled', {
      body: 'You’ll get pings here for your chosen events.',
      icon: ICON_URL,
      tag: 'lwts-test',
      renotify: true,
    });
    n.onclick = () => { try { window.focus(); } catch {} n.close(); };
    n.onerror = (ev) => {
      console.warn('Notification error', ev);
      if (window.Toast) window.Toast.error('OS rejected the notification — check macOS System Settings → Notifications → Chrome.');
    };
    if (window.Toast) window.Toast.info('Test sent — if nothing appears, check the OS notification center or allow Chrome in system notifications.');
  } catch (err) {
    console.error('Failed to create Notification', err);
    if (window.Toast) window.Toast.error('Failed to show notification: ' + ((err && err.message) || 'unknown'));
  }
}

window.Notifier = {
  EVENT_KEYS,
  isSupported,
  currentPermission,
  isMasterEnabled,
  setMasterEnabled,
  canNotify,
  requestPermission,
  getBoardTriggers,
  normalizeTriggers,
  wireNotificationHandlers,
  testNotification,
  fire,
};

window.wireNotificationHandlers = wireNotificationHandlers;
