/* fn2 Kanban — API Client */

const API = {
  baseURL: '',
  _refreshing: null,
  _refreshQueue: [],

  async request(method, path, body = null) {
    const headers = { 'Content-Type': 'application/json' };
    const token = window.Auth.getAccessToken();
    if (token) headers['Authorization'] = `Bearer ${token}`;

    const opts = { method, headers };
    if (body !== null) opts.body = JSON.stringify(body);

    let res = await fetch(this.baseURL + path, opts);

    if (res.status === 401) {
      if (window.Auth.getRefreshToken()) {
        const refreshed = await this._doRefresh();
        if (refreshed) {
          headers['Authorization'] = `Bearer ${window.Auth.getAccessToken()}`;
          res = await fetch(this.baseURL + path, { method, headers, body: opts.body });
        } else {
          window.Auth.clearTokens();
          const err = new Error('Authentication expired');
          err.status = 401;
          throw err;
        }
      } else {
        window.Auth.clearTokens();
        const err = new Error('Not authenticated');
        err.status = 401;
        throw err;
      }
    }

    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: res.statusText }));
      const error = new Error(err.error || res.statusText);
      error.status = res.status;
      error.data = err;
      throw error;
    }

    return res.status === 204 ? null : res.json();
  },

  async _doRefresh() {
    if (this._refreshing) return this._refreshing;
    this._refreshing = (async () => {
      try {
        const res = await fetch(this.baseURL + '/api/auth/refresh', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ refresh_token: window.Auth.getRefreshToken() })
        });
        if (!res.ok) throw new Error('refresh failed');
        const data = await res.json();
        window.Auth.setTokens(data.access_token, data.refresh_token);
        return true;
      } catch {
        window.Auth.clearTokens();
        return false;
      } finally {
        this._refreshing = null;
      }
    })();
    return this._refreshing;
  },

  get(path)        { return this.request('GET', path); },
  post(path, body) { return this.request('POST', path, body); },
  put(path, body)  { return this.request('PUT', path, body); },
  del(path)        { return this.request('DELETE', path); },

  listBoards()     { return this.get('/api/v1/boards'); },
  getBoard(id)     { return this.get('/api/v1/boards/' + id); },
  createBoard(data) { return this.post('/api/v1/boards', data); },
  updateBoard(id, data) { return this.put('/api/v1/boards/' + id, data); },
  deleteBoard(id)  { return this.del('/api/v1/boards/' + id); },

  listCards(boardId) { return this.get('/api/v1/boards/' + boardId + '/cards'); },
  getCard(id)        { return this.get('/api/v1/cards/' + id); },
  createCard(boardId, data) { return this.post('/api/v1/boards/' + boardId + '/cards', data); },
  updateCard(id, data)      { return this.put('/api/v1/cards/' + id, data); },
  moveCard(id, data)        { return this.post('/api/v1/cards/' + id + '/move', data); },
  bulkMoveCards(boardId, cardIds, columnId) { return this.post('/api/v1/boards/' + boardId + '/cards/bulk-move', { card_ids: cardIds, column_id: columnId }); },
  deleteCard(id)    { return this.del('/api/v1/cards/' + id); },

  listComments(cardId)       { return this.get('/api/v1/cards/' + cardId + '/comments'); },
  createComment(cardId, body) { return this.post('/api/v1/cards/' + cardId + '/comments', { body }); },
  updateComment(id, body)    { return this.put('/api/v1/comments/' + id, { body }); },
  deleteComment(id)          { return this.del('/api/v1/comments/' + id); },

  listUsers()  { return this.get('/api/v1/users'); },
  getMe()      { return this.get('/api/auth/me'); },

  searchCards(query, boardId) {
    let path = '/api/v1/search?q=' + encodeURIComponent(query);
    if (boardId && boardId !== 'all') path += '&board_id=' + encodeURIComponent(boardId);
    return this.get(path);
  },

  getSettings(category)      { return this.get('/api/v1/settings/' + category); },
  putSettings(category, data) { return this.put('/api/v1/settings/' + category, data); },
  updateUserRole(id, role)   { return this.put('/api/v1/users/' + id, { role }); },
  updateUserAvatar(id, avatarUrl) { return this.put('/api/v1/users/' + id, { avatar_url: avatarUrl }); },
  deleteUser(id)             { return this.del('/api/v1/users/' + id); },
  createInvite(email, role)  { return this.post('/api/v1/invites', { email, role }); },
  createUser(name, email, password, role) { return this.post('/api/v1/users', { name, email, password, role }); },

  listKeys()          { return this.get('/api/v1/keys'); },
  createKey(name, userId) {
    const body = { name };
    if (userId) body.user_id = userId;
    return this.post('/api/v1/keys', body);
  },
  revealKey(id)       { return this.get('/api/v1/keys/' + id + '/reveal'); },
  deleteKey(id)       { return this.del('/api/v1/keys/' + id); },

  createBot(name, role) { return this.post('/api/v1/bots', { name, role }); },

  exportData()        { return this.get('/api/v1/export'); },
  importJira(data)    { return this.post('/api/v1/import/jira', data); },
  importTrello(data)  { return this.post('/api/v1/import/trello', data); },

  resetWorkspace(mode) { return this.post('/api/v1/settings/reset', { mode: mode || 'empty' }); },

  getDiscord()          { return this.get('/api/v1/integrations/discord'); },
  putDiscord(data)      { return this.put('/api/v1/integrations/discord', data); },
  testDiscord()         { return this.post('/api/v1/integrations/discord/test'); },
};

const Toast = {
  container: null,
  init() {
    this.container = document.createElement('div');
    this.container.className = 'toast-container';
    document.body.appendChild(this.container);
  },
  show(message, type = 'error', duration) {
    if (duration === undefined || duration === null) duration = 5000;
    if (!this.container) this.init();
    const el = document.createElement('div');
    el.className = 'toast toast-' + type;
    const lines = message.split('\n').map(l => this._esc(l)).join('<br>');
    el.innerHTML = `
      <span class="toast-message">${lines}</span>
      <button class="toast-close" onclick="this.parentElement.remove()">&times;</button>
    `;
    this.container.appendChild(el);
    requestAnimationFrame(() => el.classList.add('visible'));
    if (duration > 0) {
      setTimeout(() => {
        el.classList.remove('visible');
        el.addEventListener('transitionend', () => el.remove(), { once: true });
        setTimeout(() => el.remove(), 500);
      }, duration);
    }
  },
  success(msg, opts) { this.show(msg, 'success', opts && opts.duration); },
  error(msg, opts)   { this.show(msg, 'error', opts && opts.duration); },
  info(msg, opts)    { this.show(msg, 'info', opts && opts.duration); },
  _esc(s) { const d = document.createElement('div'); d.textContent = s; return d.innerHTML; }
};

window.API = API;
window.Toast = Toast;
