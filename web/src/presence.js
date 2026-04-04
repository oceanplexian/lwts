// fn2 Kanban — User Presence + Conflict Resolution

let presenceUsers = [];
const MAX_VISIBLE_AVATARS = 5;

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
  const token = localStorage.getItem('access_token') || '';
  fetch(`/api/v1/boards/${boardId}/presence`, {
    headers: token ? { 'Authorization': 'Bearer ' + token } : {},
  })
    .then(r => r.ok ? r.json() : [])
    .then(users => { presenceUsers = users; renderPresence(); })
    .catch(() => {});
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
    loadPresence(boardStream.boardId);
  };
}

window.presenceUsers = presenceUsers;
window.renderPresence = renderPresence;
window.loadPresence = loadPresence;
window.addPresenceUser = addPresenceUser;
window.removePresenceUser = removePresenceUser;
window.showConflictToast = showConflictToast;
window.showUpdatedIndicator = showUpdatedIndicator;
window.conflictAwareFetch = conflictAwareFetch;
window.wirePresenceHandlers = wirePresenceHandlers;
