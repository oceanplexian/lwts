// fn2 Kanban — User Presence + Conflict Resolution

let presenceUsers = [];
const MAX_VISIBLE_AVATARS = 5;
let lastBoardResyncAt = 0;

const PRESENCE_COLORS = [
  '#82B1FF', '#fbc02d', '#4ade80', '#fb8c00', '#579DFF',
  '#ab47bc', '#26a69a', '#ef5350', '#5c6bc0', '#66bb6a',
];

function hashColor(str) {
  let hash = 0;
  for (let i = 0; i < str.length; i++) hash = str.charCodeAt(i) + ((hash << 5) - hash);
  return PRESENCE_COLORS[Math.abs(hash) % PRESENCE_COLORS.length];
}

function getInitials(name) {
  if (!name) return '?';
  const parts = name.trim().split(/\s+/);
  if (parts.length >= 2) return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
  return name.slice(0, 2).toUpperCase();
}

function renderPresence() {
  let container = document.getElementById('presence-row');
  if (!container) {
    container = document.createElement('div');
    container.id = 'presence-row';
    container.className = 'presence-row';
    const actions = document.querySelector('.header-actions');
    if (actions) actions.insertBefore(container, actions.firstChild);
  }
  container.innerHTML = '';

  const visible = presenceUsers.slice(0, MAX_VISIBLE_AVATARS);
  const overflow = presenceUsers.length - MAX_VISIBLE_AVATARS;

  visible.forEach(user => {
    const el = document.createElement('div');
    el.className = 'presence-avatar';
    el.setAttribute('data-tooltip', user.username || user.id);
    el.dataset.userId = user.id;
    if (user.avatar_url) {
      el.style.backgroundColor = 'transparent';
      el.innerHTML = `<img src="${user.avatar_url}" alt="${user.initials || getInitials(user.username || user.id)}">`;
    } else {
      el.style.backgroundColor = user.avatar_color || hashColor(user.username || user.id);
      el.textContent = user.initials || getInitials(user.username || user.id);
    }
    container.appendChild(el);
  });

  if (overflow > 0) {
    const pill = document.createElement('div');
    pill.className = 'presence-overflow';
    pill.textContent = '+' + overflow;
    container.appendChild(pill);
  }
}

function loadPresence(boardId) {
  const token = Auth.getAccessToken() || '';
  fetch(`/api/v1/boards/${boardId}/presence`, {
    headers: token ? { 'Authorization': 'Bearer ' + token } : {},
  })
    .then(r => r.ok ? r.json() : [])
    .then(users => { presenceUsers = users; renderPresence(); })
    .catch(() => {});
}

function clearPresence() {
  presenceUsers = [];
  renderPresence();
}

function resyncCurrentBoard(boardId) {
  const activeBoardId = boardId || window.currentBoardId;
  if (!activeBoardId || activeBoardId === 'all') return;
  if (window.currentBoardId && activeBoardId !== window.currentBoardId) return;
  if (typeof window.loadBoardCards !== 'function') return;

  const now = Date.now();
  if (now - lastBoardResyncAt < 1000) return;
  lastBoardResyncAt = now;

  window.loadBoardCards(activeBoardId).catch(() => {});
}

function addPresenceUser(data) {
  if (presenceUsers.some(u => u.id === data.user_id)) return;
  presenceUsers.push({
    id: data.user_id,
    username: data.username,
    initials: getInitials(data.username),
    avatar_color: data.avatar_color,
    avatar_url: data.avatar_url || '',
  });
  renderPresence();
}

function removePresenceUser(data) {
  presenceUsers = presenceUsers.filter(u => u.id !== data.user_id);
  renderPresence();
}

function showConflictToast(message) {
  const existing = document.querySelector('.conflict-toast');
  if (existing) existing.remove();

  const toast = document.createElement('div');
  toast.className = 'conflict-toast';
  toast.textContent = message || 'Card was modified by another user — refreshed';
  document.body.appendChild(toast);

  setTimeout(() => {
    toast.classList.add('fade-out');
    setTimeout(() => toast.remove(), 300);
  }, 4000);
}

function showUpdatedIndicator(username) {
  const existing = document.querySelector('.card-updated-indicator');
  if (existing) existing.remove();

  const detail = document.querySelector('.detail-header') || document.querySelector('.modal-content');
  if (!detail) return;

  const indicator = document.createElement('div');
  indicator.className = 'card-updated-indicator';
  indicator.textContent = 'Updated by ' + (username || 'another user');
  detail.appendChild(indicator);

  setTimeout(() => { if (indicator.parentNode) indicator.remove(); }, 5000);
}

async function conflictAwareFetch(url, options) {
  const response = await fetch(url, options);

  if (response.status === 409) {
    const body = await response.json().catch(() => ({}));
    // 409 conflict — silently handle
    if (body.current) return { conflict: true, current: body.current, response };
    return { conflict: true, response };
  }

  return { conflict: false, response };
}

function wirePresenceHandlers(boardStream) {
  if (!boardStream) return;

  boardStream.handlers.onUserJoined = (data) => addPresenceUser(data);
  boardStream.handlers.onUserLeft = (data) => removePresenceUser(data);

  boardStream.handlers.onCardUpdated = (data) => {
    if (typeof window.updateCardInState === 'function') window.updateCardInState(data);
    if (window.detailCard && window.detailCard.id === data.id) showUpdatedIndicator(data.updated_by);
  };

  boardStream.handlers.onCardCreated = (data) => {
    if (typeof window.addCardToState === 'function') window.addCardToState(data);
  };

  boardStream.handlers.onCardMoved = (data) => {
    if (typeof window.moveCardInState === 'function') window.moveCardInState(data);
  };

  boardStream.handlers.onCardDeleted = (data) => {
    if (typeof window.removeCardFromState === 'function') window.removeCardFromState(data);
  };

  boardStream.handlers.onConnected = () => {
    const wasReconnect = !!boardStream._hasConnectedOnce;
    boardStream._hasConnectedOnce = true;
    loadPresence(boardStream.boardId);
    updateConnectionDot(true);
    if (wasReconnect) resyncCurrentBoard(boardStream.boardId);
  };

  boardStream.handlers.onDisconnect = () => {
    updateConnectionDot(false);
  };

  boardStream.handlers.onCommentAdded = (data) => {
    if (window.detailCard && window.detailCard.id === data.card_id) {
      // Avoid duplicating a comment we already have (e.g. from our own submission)
      const existing = (window.detailCard.comments || []);
      if (existing.some(c => c.id === data.id)) return;

      const comment = {
        id: data.id,
        author: data.author_id,
        author_id: data.author_id,
        text: data.body,
        body: data.body,
        time: data.created_at,
        created_at: data.created_at,
        updated_at: data.updated_at || null,
      };
      window.detailCard.comments = window.detailCard.comments || [];
      window.detailCard.comments.push(comment);
      if (typeof window.renderComments === 'function') window.renderComments();

      // Animate the new comment
      const list = document.getElementById('detail-comments-list');
      const last = list ? list.lastElementChild : null;
      if (last) {
        last.classList.add('new');
        last.addEventListener('animationend', () => last.classList.remove('new'), { once: true });
      }
    }
  };

  boardStream.handlers.onCommentDeleted = (data) => {
    if (window.detailCard && window.detailCard.id === data.card_id) {
      const comments = window.detailCard.comments || [];
      const idx = comments.findIndex(c => c.id === data.id);
      if (idx === -1) return;

      // Find the DOM element (comments are rendered in order, so use the idx-th child)
      const list = document.getElementById('detail-comments-list');
      const el = list ? list.children[idx] : null;
      if (el) {
        el.classList.add('removing');
        el.addEventListener('animationend', () => {
          comments.splice(idx, 1);
          if (typeof window.renderComments === 'function') window.renderComments();
        }, { once: true });
      } else {
        comments.splice(idx, 1);
        if (typeof window.renderComments === 'function') window.renderComments();
      }
    }
  };
}

document.addEventListener('visibilitychange', () => {
  if (document.visibilityState === 'visible') {
    resyncCurrentBoard();
  }
});

window.addEventListener('focus', () => {
  resyncCurrentBoard();
});

function updateConnectionDot(connected) {
  let dot = document.querySelector('.connection-dot');
  if (!dot) {
    dot = document.createElement('span');
    dot.className = 'connection-dot';
    const container = document.getElementById('presence-row') || document.querySelector('.header-actions');
    if (container) container.insertBefore(dot, container.firstChild);
  }
  dot.classList.toggle('connected', connected);
  dot.classList.toggle('disconnected', !connected);
  dot.title = connected ? 'Connected' : 'Disconnected';
}

window.presenceUsers = presenceUsers;
window.renderPresence = renderPresence;
window.loadPresence = loadPresence;
window.clearPresence = clearPresence;
window.resyncCurrentBoard = resyncCurrentBoard;
window.addPresenceUser = addPresenceUser;
window.removePresenceUser = removePresenceUser;
window.showConflictToast = showConflictToast;
window.showUpdatedIndicator = showUpdatedIndicator;
window.conflictAwareFetch = conflictAwareFetch;
window.wirePresenceHandlers = wirePresenceHandlers;
window.updateConnectionDot = updateConnectionDot;
