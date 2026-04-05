// fn2 Kanban — Board Logic

const COLUMNS = [
  { id: 'backlog',     label: 'Backlog' },
  { id: 'todo',        label: 'To Do' },
  { id: 'in-progress', label: 'In Progress' },
  { id: 'done',        label: 'Done' },
];

const TAG_LABELS = { blue: 'feature', green: 'fix', orange: 'infra', red: 'bug', epic: 'epic' };
const TAG_COLORS = {
  blue:   'var(--accent-blue)',
  green:  'var(--accent-green)',
  orange: 'var(--accent-orange)',
  red:    'var(--accent-red)',
  epic:   'rgba(192,132,252,0.9)',
};

// Jira-style epic lane colors — base RGB values, alpha applied per theme
const _EPIC_BASE = [
  [87,157,255],   // blue
  [159,143,239],  // purple
  [108,195,224],  // teal
  [75,206,151],   // green
  [245,205,71],   // yellow
  [231,116,187],  // pink
];
function _getEpicColors() {
  const isLight = document.body.classList.contains('light-theme');
  return _EPIC_BASE.map(([r,g,b]) => {
    if (isLight) {
      return {
        bg: `linear-gradient(135deg, rgba(${r},${g},${b},0.18) 0%, rgba(${r},${g},${b},0.08) 50%, rgba(${r},${g},${b},0.14) 100%)`,
        border: `rgba(${r},${g},${b},0.35)`,
      };
    }
    return {
      bg: `linear-gradient(135deg, rgba(${r},${g},${b},0.08) 0%, rgba(${r},${g},${b},0.03) 50%, rgba(${r},${g},${b},0.06) 100%)`,
      border: `rgba(${r},${g},${b},0.2)`,
    };
  });
}
let EPIC_LANE_COLORS = _getEpicColors();

const PRIORITIES = [
  { value: 'highest', label: 'Highest', color: '#f44336' },
  { value: 'high',    label: 'High',    color: '#fb8c00' },
  { value: 'medium',  label: 'Medium',  color: '#fbc02d' },
  { value: 'low',     label: 'Low',     color: '#82B1FF' },
  { value: 'lowest',  label: 'Lowest',  color: '#4ade80' },
];

// Users loaded from API — initialized with unassigned fallback
let USERS = [
  { value: 'unassigned', label: 'Unassigned', initials: '', avatar_color: '' },
];
let AVATAR_COLORS = {};
let currentUser = null;

async function loadUsers() {
  try {
    const [users, me] = await Promise.all([window.API.listUsers(), window.API.getMe()]);
    currentUser = me;
    USERS = [{ value: 'unassigned', label: 'Unassigned', initials: '', avatar_color: '', avatar_url: '' }];
    AVATAR_COLORS = {};
    (users || []).forEach(u => {
      USERS.push({
        value: u.id,
        label: u.id === me.id ? 'You' : u.name,
        initials: u.initials || u.name.split(' ').map(w => w[0]).join('').toUpperCase().slice(0, 2),
        avatar_color: u.avatar_color || '#82B1FF',
        avatar_url: u.avatar_url || '',
      });
      AVATAR_COLORS[u.id] = u.avatar_color || '#82B1FF';
    });
  } catch (e) {
    console.error('Failed to load users', e);
  }
}

function getUserById(id) {
  if (!id) return USERS[0];
  return USERS.find(u => u.value === id) || { value: id, label: id, initials: '?', avatar_color: '#82B1FF', avatar_url: '' };
}

const PRIORITY_ICONS = {
  highest: '<svg viewBox="0 0 24 24" fill="none" stroke="#f44336" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="18 15 12 9 6 15"/><polyline points="18 9 12 3 6 9"/></svg>',
  high:    '<svg viewBox="0 0 24 24" fill="none" stroke="#fb8c00" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="18 16 12 10 6 16"/></svg>',
  medium:  '<svg viewBox="0 0 24 24" fill="none" stroke="#fbc02d" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="5" y1="12" x2="19" y2="12"/></svg>',
  low:     '<svg viewBox="0 0 24 24" fill="none" stroke="#82B1FF" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="6 9 12 15 18 9"/></svg>',
  lowest:  '<svg viewBox="0 0 24 24" fill="none" stroke="#4ade80" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="6 9 12 15 18 9"/><polyline points="6 15 12 21 18 15"/></svg>',
};

// ── State ──
let state = { backlog: [], todo: [], 'in-progress': [], done: [], cleared: [] };
let nextId = Date.now();
let currentBoardId = localStorage.getItem('lwts-board-id') || null;
let boardList = [];
// Map of card id → server card object (for version tracking)
let cardIndex = {};

// Convert server card to local format
function fromAPI(card) {
  let relatedIds = [];
  if (card.related_card_ids) {
    try { relatedIds = typeof card.related_card_ids === 'string' ? JSON.parse(card.related_card_ids) : card.related_card_ids; } catch(e) {}
  }
  let blockedIds = [];
  if (card.blocked_card_ids) {
    try { blockedIds = typeof card.blocked_card_ids === 'string' ? JSON.parse(card.blocked_card_ids) : card.blocked_card_ids; } catch(e) {}
  }
  return {
    id: card.id,
    key: card.key || '',
    title: card.title || '',
    tag: card.tag || 'blue',
    priority: card.priority || 'medium',
    reporter: card.reporter_id || null,
    assignee: card.assignee_id || null,
    points: card.points || 0,
    due_date: card.due_date || '',
    date: card.created_at ? card.created_at.split('T')[0] : '',
    created_at: card.created_at || '',
    updated_at: card.updated_at || '',
    description: card.description || '',
    comments: [],
    attachments: [],
    related_card_ids: relatedIds,
    blocked_card_ids: blockedIds,
    version: card.version || 0,
    board_id: card.board_id,
    column_id: card.column_id,
    epic_id: card.epic_id || null,
  };
}

// ── Welcome Modal ──
function showWelcome() {
  document.getElementById('welcome-modal').classList.add('active');
}
function closeWelcome() {
  document.getElementById('welcome-modal').classList.remove('active');
  // Mark user as welcomed on the server
  window.Auth.request('/api/auth/welcomed', { method: 'POST' }).catch(() => {});
}

// Load state from API
async function loadFromAPI() {
  try {
    boardList = await window.API.listBoards();
    if (!boardList || boardList.length === 0) {
      // No boards — create a default one
      const board = await window.API.createBoard({ name: 'kanban', project_key: 'KANB' });
      boardList = [board];
    }

    // Use board ID from URL, then localStorage, then first board
    const urlBoardId = new URL(window.location).searchParams.get('board');
    if (urlBoardId && boardList.find(b => b.id === urlBoardId)) {
      currentBoardId = urlBoardId;
    }
    if (!currentBoardId || !boardList.find(b => b.id === currentBoardId)) {
      currentBoardId = boardList[0].id;
    }
    localStorage.setItem('lwts-board-id', currentBoardId);
    // Sync URL with active board
    const url = new URL(window.location);
    url.searchParams.set('board', currentBoardId);
    window.history.replaceState(null, '', url);

    await loadBoardCards(currentBoardId);
    window.renderBoardPicker();

    // Connect SSE stream for real-time updates
    if (typeof window.connectBoardStream === 'function') {
      window.connectBoardStream(currentBoardId);
    }

    // Welcome modal is shown by showBoard() before the fade-in,
    // so it's already visible by the time we get here.
  } catch (e) {
    if (e.status === 401) {
      // Auth failed — redirect to login (guarded against loops)
      window.Auth.redirectToLogin();
      return;
    }
    console.error('Failed to load from API, falling back to localStorage', e);
    loadFromLocalStorage();
  }
}

async function loadBoardCards(boardId) {
  // Sync COLUMNS from the board's column config
  const board = boardList.find(b => b.id === boardId);
  if (board && board.columns) {
    try {
      const cols = JSON.parse(board.columns);
      if (Array.isArray(cols) && cols.length > 0) {
        COLUMNS.length = 0;
        cols.forEach(c => COLUMNS.push(c));
      }
    } catch (e) { /* keep defaults */ }
  }

  const grouped = await window.API.listCards(boardId);
  cardIndex = {};
  const prevCleared = state.cleared || [];
  state = { cleared: prevCleared };
  COLUMNS.forEach(col => { state[col.id] = []; });
  for (const [colId, cards] of Object.entries(grouped || {})) {
    if (colId === 'cleared') {
      // Cleared cards from server go into cleared array
      state.cleared = (cards || []).map(c => {
        const local = fromAPI(c);
        cardIndex[c.id] = c;
        return local;
      });
    } else {
      state[colId] = (cards || []).map(c => {
        const local = fromAPI(c);
        cardIndex[c.id] = c;
        return local;
      });
    }
  }
  window.render();
  if (typeof window.currentView !== "undefined" && window.currentView === 'list' && typeof renderListView === 'function') {
    window.renderListView();
  }
}

// Fallback to localStorage (offline/dev)
function loadFromLocalStorage() {
  try {
    const d = JSON.parse(localStorage.getItem('lwts-kanban') || 'null');
    if (d) {
      for (const col of Object.values(d)) {
        if (!Array.isArray(col)) continue;
        for (const card of col) {
          if (!card.comments) card.comments = [];
          if (!card.reporter) card.reporter = null;
          if (!card.assignee) card.assignee = null;
          if (!card.attachments) card.attachments = [];
          if (!card.priority) card.priority = 'medium';
        }
      }
      if (!d.cleared) d.cleared = [];
      state = d;
    }
  } catch { /* ignore */ }
  window.render();
}

function save() {
  // Keep localStorage as cache for fast reload
  localStorage.setItem('lwts-kanban', JSON.stringify(state));
}

function _capitalize(s) {
  return s ? s.charAt(0).toUpperCase() + s.slice(1) : s;
}

function renderBoardPicker() {
  const menu = document.getElementById('board-menu');
  if (!menu) return;
  menu.innerHTML = '';
  boardList.forEach(b => {
    const opt = document.createElement('div');
    opt.className = 'header-board-option' + (b.id === currentBoardId ? ' active' : '');
    opt.textContent = _capitalize(b.name);
    opt.onclick = () => switchBoard(b.id, b.name);
    menu.appendChild(opt);
  });

  let sep = document.createElement('div');
  sep.className = 'header-board-sep';
  menu.appendChild(sep);

  const newOpt = document.createElement('div');
  newOpt.className = 'header-board-option new';
  newOpt.textContent = '+ New board';
  newOpt.onclick = createBoard;
  menu.appendChild(newOpt);

  const settingsOpt = document.createElement('div');
  settingsOpt.className = 'header-board-option';
  settingsOpt.innerHTML = '<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg> Settings';
  settingsOpt.onclick = () => { closeBoardPicker(); location.hash = '#settings/boards'; };
  menu.appendChild(settingsOpt);

  // Update label
  const current = boardList.find(b => b.id === currentBoardId);
  const label = document.getElementById('board-picker-label');
  if (label && current) label.textContent = _capitalize(current.name);
}

// ── Drag state ──
let dragCard = null;
let dragSourceCol = null;
let dragEl = null;
let didDrag = false;

// ── Create modal dropdowns + editor ──

// ── Detail modal state ──
let detailCard = null;
let detailCol = null;
let ddDetailColumn = null, ddDetailTag = null, ddDetailPriority = null;
let ddDetailReporter = null, ddDetailAssignee = null, ddDetailProject = null, ddDetailEpic = null;
let detailEditor = null;
let commentEditor = null;

// ═══════════════════════════════════════════════════════════════
// RENDER
// ═══════════════════════════════════════════════════════════════

let _renderAnimateCards = true;

function render() {
  EPIC_LANE_COLORS = _getEpicColors();
  const board = document.getElementById('board');
  const animate = _renderAnimateCards;
  _renderAnimateCards = false;

  // Detect epics across all columns
  const epics = [];
  for (const col of COLUMNS) {
    for (const card of (state[col.id] || [])) {
      if (card.tag === 'epic') epics.push(card);
    }
  }

  const frag = document.createDocumentFragment();

  if (epics.length === 0) {
    board.classList.remove('board-epic-mode');
    _renderStandardBoard(frag);
  } else {
    board.classList.add('board-epic-mode');
    _renderEpicBoard(frag, epics);
  }

  board.innerHTML = '';
  board.appendChild(frag);

  // Staggered unfurl animation
  if (animate) {
    requestAnimationFrame(() => {
      document.querySelectorAll('.column-body').forEach(col => {
        col.querySelectorAll('.card').forEach((card, i) => {
          card.style.animationDelay = (i * 18) + 'ms';
          card.classList.add('unfurl');
          card.addEventListener('animationend', () => card.classList.remove('unfurl'), { once: true });
        });
      });
    });
  }

}

function _renderStandardBoard(frag) {
  COLUMNS.forEach(col => {
    const cards = state[col.id] || [];
    const colEl = document.createElement('div');
    colEl.className = 'column';
    colEl.innerHTML = `
      <div class="column-header">
        <span class="column-label">
          <span class="column-dot ${col.id}"></span>
          ${col.label}
        </span>
        <span class="column-count">${cards.length}</span>
      </div>
    `;

    const body = document.createElement('div');
    body.className = 'column-body';
    body.dataset.col = col.id;

    if (cards.length === 0) {
      const empty = document.createElement('div');
      empty.className = 'column-empty';
      empty.textContent = 'No cards';
      body.appendChild(empty);
    }

    cards.forEach((card, idx) => {
      body.appendChild(createCardEl(card, col.id, idx));
    });

    body.addEventListener('dragover', onDragOver);
    body.addEventListener('dragleave', onDragLeave);
    body.addEventListener('drop', onDrop);

    colEl.appendChild(body);
    frag.appendChild(colEl);
  });
}

function _renderEpicBoard(frag, epics) {
  // Sticky column headers
  const headerRow = document.createElement('div');
  headerRow.className = 'board-column-headers';
  COLUMNS.forEach(col => {
    const total = (state[col.id] || []).length;
    const hdr = document.createElement('div');
    hdr.className = 'column-header';
    hdr.innerHTML = `
      <span class="column-label">
        <span class="column-dot ${col.id}"></span>
        ${col.label}
      </span>
      <span class="column-count">${total}</span>
    `;
    headerRow.appendChild(hdr);
  });
  frag.appendChild(headerRow);

  // Epic lanes
  epics.forEach((epic, ei) => {
    const color = EPIC_LANE_COLORS[ei % EPIC_LANE_COLORS.length];
    const lane = document.createElement('div');
    lane.className = 'epic-lane';
    lane.dataset.epicId = epic.id;
    lane.style.background = color.bg;

    const laneCount = COLUMNS.reduce((n, col) => {
      return n + (state[col.id] || []).filter(c => c.epic_id === epic.id && c.tag !== 'epic').length;
    }, 0);

    const hdr = document.createElement('div');
    hdr.className = 'epic-lane-header';
    hdr.innerHTML = `
      <span class="epic-lane-key">${esc(epic.key)}</span>
      <span class="epic-lane-title">${esc(epic.title)}</span>
      <span class="epic-lane-count">${laneCount} card${laneCount !== 1 ? 's' : ''}</span>
    `;
    hdr.title = epic.description || epic.title;
    hdr.style.cursor = 'pointer';
    hdr.addEventListener('click', () => window.openDetail(epic.id));
    lane.appendChild(hdr);

    const cols = document.createElement('div');
    cols.className = 'epic-lane-columns';

    COLUMNS.forEach(col => {
      const cell = document.createElement('div');
      cell.className = 'epic-lane-cell';

      const body = document.createElement('div');
      body.className = 'column-body';
      body.dataset.col = col.id;
      body.dataset.epic = epic.id;

      const laneCards = (state[col.id] || []).filter(c => c.epic_id === epic.id && c.tag !== 'epic');
      laneCards.forEach((card, idx) => {
        body.appendChild(createCardEl(card, col.id, idx));
      });

      body.addEventListener('dragover', onDragOver);
      body.addEventListener('dragleave', onDragLeave);
      body.addEventListener('drop', onDrop);

      cell.appendChild(body);
      cols.appendChild(cell);
    });

    lane.appendChild(cols);
    frag.appendChild(lane);
  });

  // Ungrouped cards — no lane header, just standard columns
  const hasUngrouped = COLUMNS.some(col =>
    (state[col.id] || []).some(c => !c.epic_id || c.tag === 'epic')
  );

  if (hasUngrouped) {
    const lane = document.createElement('div');
    lane.className = 'epic-lane epic-lane-ungrouped';

    const cols = document.createElement('div');
    cols.className = 'epic-lane-columns';

    COLUMNS.forEach(col => {
      const cell = document.createElement('div');
      cell.className = 'epic-lane-cell';

      const body = document.createElement('div');
      body.className = 'column-body';
      body.dataset.col = col.id;
      body.dataset.epic = '';

      const ungroupedCards = (state[col.id] || []).filter(c => !c.epic_id || c.tag === 'epic');
      ungroupedCards.forEach((card, idx) => {
        body.appendChild(createCardEl(card, col.id, idx));
      });

      body.addEventListener('dragover', onDragOver);
      body.addEventListener('dragleave', onDragLeave);
      body.addEventListener('drop', onDrop);

      cell.appendChild(body);
      cols.appendChild(cell);
    });

    lane.appendChild(cols);
    frag.appendChild(lane);
  }
}

function createCardEl(card, colId, idx) {
  const el = document.createElement('div');
  el.className = 'card';
  el.draggable = true;
  el.dataset.id = card.id;
  el.dataset.col = colId;
  el.dataset.idx = idx;

  const tagLabel = TAG_LABELS[card.tag] || card.tag;
  const priorityIcon = PRIORITY_ICONS[card.priority] || PRIORITY_ICONS.medium;
  const user = getUserById(card.assignee);
  const avatarColor = AVATAR_COLORS[card.assignee] || 'var(--surface-active)';
  const initials = user.initials || '?';
  const avatarBg = user.initials ? `${avatarColor}25` : 'var(--surface-active)';
  const avatarFg = user.initials ? avatarColor : 'var(--text-dimmed)';
  const avatarBorder = user.initials ? `${avatarColor}40` : 'var(--border-light)';
  const commentCount = (card.comments || []).length + (card.comment_count || 0);
  const avatarContent = user.avatar_url
    ? `<img src="${esc(user.avatar_url)}" alt="${esc(initials)}">`
    : esc(initials);

  el.innerHTML = `
    <div class="card-bubble card-avatar card-avatar-corner" style="background:${avatarBg};color:${avatarFg};border-color:${avatarBorder}">${avatarContent}</div>
    <div class="card-title">${esc(card.title)}</div>
    <div class="card-tags">
      <span class="card-tag tag-${esc(card.tag)}">${esc(tagLabel)}</span>
    </div>
    <div class="card-footer">
      <div class="card-footer-left">
        <span class="card-key">${esc(card.key || '')}</span>
      </div>
      <div class="card-footer-right">
        ${card.points ? `<span class="card-bubble card-points">${card.points}</span>` : ''}
        <span class="card-bubble card-priority">${priorityIcon}</span>
      </div>
    </div>
  `;

  el.addEventListener('dragstart', onDragStart);
  el.addEventListener('dragend', onDragEnd);
  el.addEventListener('click', () => {
    if (didDrag) { didDrag = false; return; }
    window.openDetail(card.id);
  });
  return el;
}

// ═══════════════════════════════════════════════════════════════
// DRAG & DROP
// ═══════════════════════════════════════════════════════════════

let dropTargetCard = null;
let dropTargetCol = null;

function onDragStart(e) {
  didDrag = true;
  dragEl = e.target.closest('.card');
  dragSourceCol = dragEl.dataset.col;
  dragCard = (state[dragSourceCol] || []).find(c => c.id === dragEl.dataset.id) || state[dragSourceCol][parseInt(dragEl.dataset.idx)];
  dragEl.classList.add('dragging');
  e.dataTransfer.effectAllowed = 'move';
  e.dataTransfer.setData('text/plain', dragCard.id);
}

function onDragEnd(e) {
  // If drop already handled cleanup, dragCard is null — skip
  if (dragEl) dragEl.classList.remove('dragging');
  clearDropTarget();
  document.querySelectorAll('.column-body.drag-over').forEach(el => el.classList.remove('drag-over'));
  dragCard = null; dragSourceCol = null; dragEl = null;
}

function onDragOver(e) {
  e.preventDefault();
  e.dataTransfer.dropEffect = 'move';
  if (!dragCard) return;

  const body = e.currentTarget;
  body.classList.add('drag-over');
  dropTargetCol = body.dataset.col;

  // Find which card we're over
  const cards = body.querySelectorAll('.card:not(.dragging)');
  let found = null;
  for (const card of cards) {
    const rect = card.getBoundingClientRect();
    if (e.clientY >= rect.top && e.clientY <= rect.bottom) {
      found = card;
      break;
    }
  }

  // Update highlight
  if (found !== dropTargetCard) {
    if (dropTargetCard) dropTargetCard.classList.remove('drop-target');
    dropTargetCard = found;
    if (dropTargetCard) dropTargetCard.classList.add('drop-target');
  }
}

function onDragLeave(e) {
  if (!e.currentTarget.contains(e.relatedTarget)) {
    e.currentTarget.classList.remove('drag-over');
    clearDropTarget();
  }
}

function onDrop(e) {
  e.preventDefault();
  e.currentTarget.classList.remove('drag-over');
  if (!dragCard) { clearDropTarget(); return; }

  const targetCol = dropTargetCol || e.currentTarget.dataset.col;
  const targetEpicId = e.currentTarget.dataset.epic || null;
  const origEpicId = dragCard.epic_id || null;
  const epicChanged = targetEpicId !== origEpicId;
  const origSourceCol = dragSourceCol;
  const origState = JSON.parse(JSON.stringify(state));

  // Remove from source
  const srcIdx = state[dragSourceCol].findIndex(c => c.id === dragCard.id);
  if (srcIdx !== -1) state[dragSourceCol].splice(srcIdx, 1);

  let insertIdx;
  if (dropTargetCard) {
    const targetIdx = parseInt(dropTargetCard.dataset.idx);
    insertIdx = targetIdx;
    if (dragSourceCol === targetCol && srcIdx < targetIdx) insertIdx--;
    state[targetCol].splice(insertIdx, 0, dragCard);
  } else {
    state[targetCol] = state[targetCol] || [];
    state[targetCol].push(dragCard);
    insertIdx = state[targetCol].length - 1;
  }

  if (epicChanged) dragCard.epic_id = targetEpicId;

  const droppedId = dragCard.id;
  const movedCard = dragCard;
  const movedEl = dragEl;
  clearDropTarget();
  save();

  const isEpicMode = document.getElementById('board').classList.contains('board-epic-mode');
  if (epicChanged || isEpicMode) {
    dragEl = null; dragCard = null; dragSourceCol = null;
    window.render();
    // Wiggle animation on the dropped card
    requestAnimationFrame(() => {
      const droppedEl = document.querySelector('.card[data-id="' + droppedId + '"]');
      if (droppedEl) {
        droppedEl.classList.add('drop-wiggle');
        droppedEl.addEventListener('animationend', () => droppedEl.classList.remove('drop-wiggle'), { once: true });
      }
    });
  } else if (movedEl) {
    // Keep dragging style until animation starts — no flash
    movedEl.remove();

    const targetBody = document.querySelector('.column-body[data-col="' + targetCol + '"]');
    if (targetBody) {
      const emptyEl = targetBody.querySelector('.column-empty');
      if (emptyEl) emptyEl.remove();

      const children = targetBody.querySelectorAll('.card');
      if (insertIdx < children.length) {
        targetBody.insertBefore(movedEl, children[insertIdx]);
      } else {
        targetBody.appendChild(movedEl);
      }

      movedEl.dataset.col = targetCol;
      movedEl.dataset.idx = insertIdx;
      targetBody.querySelectorAll('.card').forEach((c, i) => c.dataset.idx = i);
    }

    // Re-index siblings in source column if different
    if (origSourceCol !== targetCol) {
      const srcBody = document.querySelector('.column-body[data-col="' + origSourceCol + '"]');
      if (srcBody) {
        srcBody.querySelectorAll('.card').forEach((c, i) => c.dataset.idx = i);
        // Add empty state if no cards left
        if (srcBody.querySelectorAll('.card').length === 0) {
          const empty = document.createElement('div');
          empty.className = 'column-empty';
          empty.textContent = 'No cards';
          srcBody.appendChild(empty);
        }
      }
    }

    // Update column counts
    COLUMNS.forEach(col => {
      const body = document.querySelector('.column-body[data-col="' + col.id + '"]');
      if (body) {
        const count = body.querySelectorAll('.card').length;
        const countEl = body.closest('.column').querySelector('.column-count');
        if (countEl) countEl.textContent = count;
      }
    });

    // Remove dragging class and null out so onDragEnd is a no-op
    movedEl.classList.remove('dragging');
    dragEl = null; dragCard = null; dragSourceCol = null;

    // Wiggle animation
    movedEl.classList.add('drop-wiggle');
    movedEl.addEventListener('animationend', () => movedEl.classList.remove('drop-wiggle'), { once: true });
  } else {
    // Fallback to full render if we lost the element reference
    window.render();
  }

  // Sync with API
  if (currentBoardId) {
    const movePayload = {
      column_id: targetCol,
      position: insertIdx,
      version: movedCard.version || 0,
    };
    if (epicChanged) movePayload.epic_id = targetEpicId || '';
    window.API.moveCard(droppedId, movePayload).then(updated => {
      movedCard.version = updated.version;
      cardIndex[droppedId] = updated;
    }).catch(err => {
      if (err.status === 422 && err.data && err.data.blockers) {
        // Transition blocked — revert and show blockers
        state = origState;
        save(); window.render();
        const msgs = err.data.blockers.map(b => b.message);
        window.Toast.error(msgs.join('\n'), { duration: 5000 });
      } else if (err.status === 409) {
        // 409 conflict — silently refresh

        loadBoardCards(currentBoardId);
      } else {
        window.Toast.error('Failed to move card');
        state = origState;
        save(); window.render();
      }
    });
  }
}

function clearDropTarget() {
  document.querySelectorAll('.card.drop-target').forEach(el => el.classList.remove('drop-target'));
  dropTargetCard = null;
  dropTargetCol = null;
}

// ═══════════════════════════════════════════════════════════════
// CREATE MODAL
// ═══════════════════════════════════════════════════════════════

let _isCreateMode = false;

function openCreateModal() {
  _isCreateMode = true;
  _pendingDescription = '';
  _pendingComments = [];
  detailCard = null;
  detailCol = null;

  // Reset comment input
  collapseCommentInput();

  // Header — show "New card" instead of key
  document.getElementById('detail-key-text').textContent = 'NEW';
  document.getElementById('detail-header-title').textContent = '';

  // Title
  const titleEl = document.getElementById('detail-title');
  titleEl.value = '';
  titleEl.placeholder = 'What needs to be done?';
  autoResizeTextarea(titleEl);

  // Description
  collapseDescription();
  const descView = document.getElementById('detail-desc-view');
  if (descView) {
    descView.innerHTML = '<span class="detail-desc-placeholder">Add a description...</span>';
  }

  // Sidebar dropdowns — defaults
  if (ddDetailProject) {
    // Refresh board options in case boards changed
    const projectOpts = boardList.map(b => ({ value: b.id, label: b.name }));
    ddDetailProject.setOptions(projectOpts);
    ddDetailProject.setValue(currentBoardId, true);
  }
  // Show project row in create mode
  const projectRow = document.getElementById('detail-project-row');
  if (projectRow) projectRow.style.display = '';

  ddDetailColumn.setValue('todo', true);
  ddDetailTag.setValue('blue', true);
  ddDetailPriority.setValue('medium', true);
  ddDetailReporter.setValue(currentUser ? currentUser.id : 'unassigned', true);
  ddDetailAssignee.setValue('unassigned', true);
  document.getElementById('detail-points').value = '';

  // Due date — default to +2 weeks
  const twoWeeks = new Date();
  twoWeeks.setDate(twoWeeks.getDate() + 14);
  window._pendingDueDate = twoWeeks.toISOString().split('T')[0];
  if (typeof window.initDueDateField === 'function') window.initDueDateField();
  if (typeof window.refreshDueDateText === 'function') window.refreshDueDateText();

  refreshSidebarTexts();
  closeAllSidebarFields();

  // Created date
  document.getElementById('detail-created').textContent = '';

  // Comments — empty for new card
  const commentsList = document.getElementById('detail-comments-list');
  if (commentsList) commentsList.innerHTML = '';

  // Attachments — empty
  const attachGrid = document.getElementById('detail-attachments');
  if (attachGrid) attachGrid.innerHTML = '';

  // Footer — swap Close for Create
  const footer = document.querySelector('#detail-modal .detail-footer');
  if (footer) {
    footer.dataset.origHtml = footer.innerHTML;
    footer.innerHTML = `
      <div></div>
      <div class="modal-footer-right">
        <button class="lwts-modal-btn-cancel" onclick="closeCreateMode()">Cancel</button>
        <button class="lwts-modal-btn-primary" id="create-submit-btn" onclick="submitCreateFromDetail()" disabled>Create</button>
      </div>`;
  }

  // Enable Create button when title is non-empty
  titleEl.addEventListener('input', _updateCreateBtnState);

  document.getElementById('detail-modal').classList.add('active');
  setTimeout(() => titleEl.focus(), 50);
}

function _updateCreateBtnState() {
  const btn = document.getElementById('create-submit-btn');
  if (btn) btn.disabled = !document.getElementById('detail-title').value.trim();
}

function closeCreateMode() {
  const titleEl = document.getElementById('detail-title');
  if (titleEl) titleEl.removeEventListener('input', _updateCreateBtnState);
  _isCreateMode = false;
  const footer = document.querySelector('#detail-modal .detail-footer');
  if (footer && footer.dataset.origHtml) {
    footer.innerHTML = footer.dataset.origHtml;
    delete footer.dataset.origHtml;
  }
  document.getElementById('detail-modal').classList.remove('active');
}

function submitCreateFromDetail() {
  const title = document.getElementById('detail-title').value.trim();
  if (!title) { document.getElementById('detail-title').focus(); return; }

  const col = ddDetailColumn.getValue();
  let assignee = ddDetailAssignee.getValue();
  const descEdit = document.getElementById('detail-desc-edit');
  const description = (detailEditor && !descEdit.classList.contains('hidden')) ? detailEditor.getMarkdown() : _pendingDescription;

  if (assignee === 'unassigned' && window._settingsCache.general && window._settingsCache.general.default_assignee_id) {
    assignee = window._settingsCache.general.default_assignee_id;
  }

  const tempId = 'temp-' + (nextId++);
  const card = {
    id: tempId,
    key: '',
    title,
    tag: ddDetailTag.getValue(),
    priority: ddDetailPriority.getValue(),
    reporter: ddDetailReporter.getValue() === 'unassigned' ? null : ddDetailReporter.getValue(),
    assignee: assignee === 'unassigned' ? null : assignee,
    points: parseInt(document.getElementById('detail-points').value) || 0,
    due_date: window._pendingDueDate || null,
    date: new Date().toISOString().split('T')[0],
    description,
    attachments: [],
    comments: _pendingComments.slice(),
    version: 0,
  };

  const pendingCommentTexts = _pendingComments.map(c => c.body || c.text);

  const targetBoardId = (ddDetailProject ? ddDetailProject.getValue() : null) || currentBoardId;
  const isCurrentBoard = targetBoardId === currentBoardId;

  if (isCurrentBoard) {
    state[col] = state[col] || [];
    state[col].push(card);
    _renderAnimateCards = true;
    save(); window.render();
  }

  _pendingDescription = '';
  _pendingComments = [];
  window._pendingDueDate = null;
  closeCreateMode();

  if (targetBoardId) {
    window.API.createCard(targetBoardId, {
      column_id: col,
      title,
      description,
      tag: card.tag,
      priority: card.priority,
      assignee_id: card.assignee,
      reporter_id: card.reporter,
      points: card.points || null,
      due_date: card.due_date || null,
    }).then(serverCard => {
      if (isCurrentBoard) {
        const colCards = state[col];
        const idx = colCards.findIndex(c => c.id === tempId);
        if (idx !== -1) {
          colCards[idx] = fromAPI(serverCard);
          cardIndex[serverCard.id] = serverCard;
          save(); window.render();
        }
      } else {
        const boardName = boardList.find(b => b.id === targetBoardId)?.name || targetBoardId;
        window.Toast.success('Card created on ' + boardName);
      }
      // Post pending comments to the newly created card
      pendingCommentTexts.forEach(text => {
        window.API.createComment(serverCard.id, text).catch(() => {});
      });
    }).catch(err => {
      window.Toast.error('Failed to create card: ' + (err.message || 'unknown error'));
      if (isCurrentBoard) {
        state[col] = state[col].filter(c => c.id !== tempId);
        save(); window.render();
      }
    });
  }
}


// ═══════════════════════════════════════════════════════════════
// DETAIL VIEW MODAL
// ═══════════════════════════════════════════════════════════════

function initDetailDropdowns() {
  const colOpts = COLUMNS.map(c => ({ value: c.id, label: c.label }));
  const tagOpts = Object.entries(TAG_LABELS).map(([val, label]) => ({
    value: val, label: label.charAt(0).toUpperCase() + label.slice(1), dot: TAG_COLORS[val],
  }));
  const priOpts = PRIORITIES.map(p => ({ value: p.value, label: p.label, dot: p.color }));
  const userOpts = USERS.map(u => ({ value: u.value, label: u.label }));

  ddDetailColumn = new window.FnDropdown(document.getElementById('detail-column'), {
    value: 'todo', options: colOpts, onChange: (v) => detailFieldChanged('column', v),
  });
  ddDetailTag = new window.FnDropdown(document.getElementById('detail-tag'), {
    value: 'blue', options: tagOpts, onChange: (v) => detailFieldChanged('tag', v),
  });
  ddDetailPriority = new window.FnDropdown(document.getElementById('detail-priority'), {
    value: 'medium', options: priOpts, onChange: (v) => detailFieldChanged('priority', v),
  });
  ddDetailReporter = new window.FnDropdown(document.getElementById('detail-reporter'), {
    value: 'you', options: userOpts, onChange: (v) => detailFieldChanged('reporter', v),
  });
  ddDetailAssignee = new window.FnDropdown(document.getElementById('detail-assignee'), {
    value: 'unassigned', options: userOpts, onChange: (v) => detailFieldChanged('assignee', v),
  });

  // Project dropdown — populated from boardList
  const projectOpts = boardList.map(b => ({ value: b.id, label: b.name }));
  ddDetailProject = new window.FnDropdown(document.getElementById('detail-project'), {
    value: currentBoardId || (boardList[0] && boardList[0].id) || '', options: projectOpts,
  });

  // Epic dropdown — populated from epic cards on the board
  _refreshEpicDropdown();
}

function _refreshEpicDropdown() {
  const epicOpts = [{ value: '', label: 'None' }];
  for (const col of COLUMNS) {
    for (const c of (state[col.id] || [])) {
      if (c.tag === 'epic') epicOpts.push({ value: c.id, label: c.key + ' — ' + c.title });
    }
  }
  const epicEl = document.getElementById('detail-epic');
  if (epicEl) epicEl.innerHTML = '';
  ddDetailEpic = new window.FnDropdown(epicEl, {
    value: '', options: epicOpts, onChange: (v) => detailFieldChanged('epic', v || null),
  });
}

function openDetail(cardId, _fromHash) {
  let foundCard = null, foundCol = null;
  for (const col of COLUMNS) {
    const c = (state[col.id] || []).find(c => c.id === cardId);
    if (c) { foundCard = c; foundCol = col.id; break; }
  }
  // Also check cleared cards
  if (!foundCard && state.cleared) {
    const c = state.cleared.find(c => c.id === cardId);
    if (c) { foundCard = c; foundCol = 'cleared'; }
  }
  if (!foundCard) return;
  window._pendingDueDate = null;

  // Update URL hash (unless we're already responding to a hash change)
  if (!_fromHash && foundCard.key) {
    window._suppressHashChange = true;
    history.pushState(null, '', '#' + foundCard.key);
    window._suppressHashChange = false;
  }

  detailCard = foundCard;
  detailCol = foundCol;

  // Reset comment input to collapsed
  collapseCommentInput();

  // Header
  document.getElementById('detail-key-text').textContent = foundCard.key || '';
  document.getElementById('detail-header-title').textContent = foundCard.title;

  // Content
  const titleEl = document.getElementById('detail-title');
  titleEl.value = foundCard.title;
  autoResizeTextarea(titleEl);

  // Description: show as rendered text, hide editor
  collapseDescription();

  // Sidebar dropdowns (hidden by default — text shown instead)
  // Show project as read-only in detail mode (can't move cards between boards)
  if (ddDetailProject) {
    ddDetailProject.setValue(foundCard.board_id || currentBoardId, true);
  }
  const projectRow = document.getElementById('detail-project-row');
  if (projectRow) projectRow.style.display = '';

  ddDetailColumn.setValue(foundCol, true);
  ddDetailTag.setValue(foundCard.tag, true);
  ddDetailPriority.setValue(foundCard.priority || 'medium', true);
  ddDetailReporter.setValue(foundCard.reporter || 'you', true);
  ddDetailAssignee.setValue(foundCard.assignee || 'unassigned', true);

  // Epic — show only for non-epic cards when epics exist
  const hasEpics = COLUMNS.some(col => (state[col.id] || []).some(c => c.tag === 'epic'));
  const isEpicCard = foundCard.tag === 'epic';
  const epicRow = document.getElementById('detail-epic-row');
  if (epicRow) epicRow.style.display = (hasEpics && !isEpicCard) ? '' : 'none';
  if (ddDetailEpic) {
    _refreshEpicDropdown();
    ddDetailEpic.setValue(foundCard.epic_id || '', true);
  }

  // Points
  document.getElementById('detail-points').value = foundCard.points || 0;

  // Refresh sidebar text values & hide all dropdowns
  refreshSidebarTexts();
  closeAllSidebarFields();

  // Created date
  document.getElementById('detail-created').textContent = foundCard.date ? `Created ${formatDateLong(foundCard.date)}` : '';

  // Comments — render local first, then fetch from API
  renderComments();
  loadCommentsFromAPI(foundCard.id);

  // Attachments
  renderAttachments();

  document.getElementById('detail-modal').classList.add('active');
}

function closeDetail(_fromHash) {
  if (_isCreateMode) { closeCreateMode(); return; }
  // Auto-save any pending changes
  const closingCard = detailCard;
  if (detailCard) {
    saveDetailFields();
  }
  document.getElementById('detail-modal').classList.remove('active');
  detailCard = null;
  detailCol = null;
  [ddDetailColumn, ddDetailTag, ddDetailPriority, ddDetailReporter, ddDetailAssignee]
    .forEach(dd => { if (dd) dd.close(); });

  // Update views — surgical list row update when in list view, full board render otherwise
  if (typeof window.currentView !== 'undefined' && window.currentView === 'list') {
    if (closingCard && typeof window._updateListRow === 'function') {
      window._updateListRow(closingCard);
    }
  } else {
    window.render();
  }

  // Clear hash (unless we're already responding to a hash change)
  if (!_fromHash && location.hash) {
    window._suppressHashChange = true;
    history.pushState(null, '', location.pathname + location.search);
    window._suppressHashChange = false;
  }
}

function saveDetailFields() {
  if (!detailCard) return;

  const oldTitle = detailCard.title;
  detailCard.title = document.getElementById('detail-title').value.trim() || detailCard.title;
  // Only update description from editor if the editor is visible (user was editing).
  // When in view mode the editor contains stale content from a previous card.
  const descEdit = document.getElementById('detail-desc-edit');
  if (descEdit && !descEdit.classList.contains('hidden') && detailEditor) {
    detailCard.description = detailEditor.getMarkdown();
  }
  detailCard.points = parseInt(document.getElementById('detail-points').value) || 0;

  // Column move
  const newCol = ddDetailColumn.getValue();
  const movedColumn = newCol !== detailCol;
  if (movedColumn) {
    state[detailCol] = state[detailCol].filter(c => c.id !== detailCard.id);
    state[newCol] = state[newCol] || [];
    state[newCol].push(detailCard);
    detailCol = newCol;
  }

  const wasEpic = detailCard.tag === 'epic';
  detailCard.tag = ddDetailTag.getValue();
  detailCard.priority = ddDetailPriority.getValue();
  detailCard.reporter = ddDetailReporter.getValue();
  detailCard.assignee = ddDetailAssignee.getValue();
  if (ddDetailEpic && detailCard.tag !== 'epic') {
    detailCard.epic_id = ddDetailEpic.getValue() || null;
  }

  // If card was an epic and type changed, release all children
  if (wasEpic && detailCard.tag !== 'epic') {
    const epicId = detailCard.id;
    for (const col of COLUMNS) {
      for (const card of (state[col.id] || [])) {
        if (card.epic_id === epicId) {
          card.epic_id = null;
          // Also update server
          if (currentBoardId && !card.id.startsWith('temp-')) {
            window.API.updateCard(card.id, { epic_id: null, version: card.version || 0 })
              .then(u => { card.version = u.version; })
              .catch(() => {});
          }
        }
      }
    }
  }

  save();

  // Sync with API
  if (currentBoardId && detailCard.id && !detailCard.id.startsWith('temp-')) {
    if (movedColumn) {
      window.API.moveCard(detailCard.id, {
        column_id: newCol,
        position: state[newCol].length - 1,
        version: detailCard.version || 0,
      }).then(updated => {
        detailCard.version = updated.version;
        cardIndex[detailCard.id] = updated;
      }).catch(err => {
        if (err.status === 422 && err.data && err.data.blockers) {
          // Revert column move
          state[newCol] = state[newCol].filter(c => c.id !== detailCard.id);
          state[detailCol] = state[detailCol] || [];
          // Undo: detailCol was already updated, need to use the old column
          // We stored movedColumn as true, so reverse it
          const prevCol = detailCol;
          // Actually detailCol was already set to newCol above, so we need to go back
          // Re-read from the card's server state
          loadBoardCards(currentBoardId);
          const msgs = err.data.blockers.map(b => b.message);
          window.Toast.error(msgs.join('\n'), { duration: 5000 });
        } else if (err.status === 409) {
          // 409 conflict — silently refresh

          loadBoardCards(currentBoardId);
        }
      });
    }

    // Debounced field update
    clearTimeout(saveDetailFields._timer);
    saveDetailFields._timer = setTimeout(() => {
      if (!detailCard || detailCard.id.startsWith('temp-')) return;
      const assignee = detailCard.assignee;
      window.API.updateCard(detailCard.id, {
        title: detailCard.title,
        description: detailCard.description,
        tag: detailCard.tag,
        priority: detailCard.priority,
        assignee_id: assignee === 'unassigned' ? null : assignee,
        epic_id: detailCard.epic_id || null,
        points: detailCard.points || null,
        version: detailCard.version || 0,
      }).then(updated => {
        detailCard.version = updated.version;
        cardIndex[detailCard.id] = updated;
      }).catch(err => {
        if (err.status === 409) {
          // 409 conflict — silently refresh

          loadBoardCards(currentBoardId);
        } else {
          window.Toast.error('Failed to save card');
        }
      });
    }, 500);
  }
}
saveDetailFields._timer = null;

function detailFieldChanged(field, value) {
  if (!detailCard && !_isCreateMode) return;
  if (detailCard) saveDetailFields();
  refreshSidebarTexts();
  closeSidebarField(field);
  // Flash the row
  var textEl = document.getElementById('detail-' + field + '-text');
  if (textEl) {
    var row = textEl.closest('.detail-field-row');
    if (row) {
      row.classList.remove('flash');
      void row.offsetWidth; // force reflow
      row.classList.add('flash');
      row.addEventListener('animationend', function() { row.classList.remove('flash'); }, { once: true });
    }
  }
}

// ── Sidebar click-to-edit ──

const SIDEBAR_FIELDS = ['project', 'column', 'tag', 'priority', 'reporter', 'assignee', 'epic', 'points'];

function refreshSidebarTexts() {
  if (!detailCard && !_isCreateMode) return;

  // Project
  if (ddDetailProject) {
    const projVal = ddDetailProject.getValue();
    const projBoard = boardList.find(b => b.id === projVal);
    document.getElementById('detail-project-text').textContent = projBoard ? projBoard.name : projVal;
  }

  const colLabel = COLUMNS.find(c => c.id === ddDetailColumn.getValue())?.label || '';
  document.getElementById('detail-column-text').innerHTML = esc(colLabel);

  const tagVal = ddDetailTag.getValue();
  const tagLabel = TAG_LABELS[tagVal] || tagVal;
  const tagColor = TAG_COLORS[tagVal] || '';
  document.getElementById('detail-tag-text').innerHTML =
    (tagColor ? `<span class="detail-field-dot" style="background:${tagColor}"></span>` : '') +
    esc(tagLabel.charAt(0).toUpperCase() + tagLabel.slice(1));

  const priVal = ddDetailPriority.getValue();
  const pri = PRIORITIES.find(p => p.value === priVal);
  document.getElementById('detail-priority-text').innerHTML =
    (pri ? `<span class="detail-field-dot" style="background:${pri.color}"></span>` : '') +
    esc(pri?.label || priVal);

  const repVal = ddDetailReporter.getValue();
  const repUser = getUserById(repVal);
  document.getElementById('detail-reporter-text').textContent = repUser.label || repVal;

  const assVal = ddDetailAssignee.getValue();
  const assUser = getUserById(assVal);
  document.getElementById('detail-assignee-text').textContent = assUser.label || assVal;

  // Epic
  const epicTextEl = document.getElementById('detail-epic-text');
  if (epicTextEl && detailCard) {
    const epicId = detailCard.epic_id || (ddDetailEpic ? ddDetailEpic.getValue() : '');
    if (epicId) {
      // Find the epic card to show its key
      let epicCard = null;
      for (const col of COLUMNS) {
        epicCard = (state[col.id] || []).find(c => c.id === epicId);
        if (epicCard) break;
      }
      epicTextEl.textContent = epicCard ? epicCard.key : epicId;
      epicTextEl.title = epicCard ? epicCard.title : '';
    } else {
      epicTextEl.textContent = 'None';
      epicTextEl.title = '';
    }
  }

  const pts = document.getElementById('detail-points').value;
  document.getElementById('detail-points-text').textContent = pts || '0';

  // GitHub links
  refreshGithubLinks();
}

// ── GitHub link detection & sidebar rendering ──

const GITHUB_URL_RE = /https:\/\/github\.com\/([\w.-]+)\/([\w.-]+)\/(issues|pull)\/(\d+)/g;
const _ghTitleCache = new Map();

function parseGithubUrls(card) {
  if (!card) return [];
  const seen = new Set();
  const results = [];
  const sources = [card.description || ''];
  (card.comments || []).forEach(c => sources.push(c.text || c.body || ''));
  for (const text of sources) {
    let m;
    GITHUB_URL_RE.lastIndex = 0;
    while ((m = GITHUB_URL_RE.exec(text)) !== null) {
      const key = m[1] + '/' + m[2] + '#' + m[4];
      if (seen.has(key)) continue;
      seen.add(key);
      results.push({ owner: m[1], repo: m[2], type: m[3], number: parseInt(m[4], 10), url: m[0] });
    }
  }
  return results;
}

function fetchGithubTitle(owner, repo, number) {
  const cacheKey = owner + '/' + repo + '#' + number;
  if (_ghTitleCache.has(cacheKey)) return Promise.resolve(_ghTitleCache.get(cacheKey));
  return fetch('https://api.github.com/repos/' + owner + '/' + repo + '/issues/' + number)
    .then(function(r) { return r.ok ? r.json() : null; })
    .then(function(data) {
      var title = data ? data.title : null;
      _ghTitleCache.set(cacheKey, title);
      return title;
    })
    .catch(function() { _ghTitleCache.set(cacheKey, null); return null; });
}

const GH_ICON_SVG = '<svg viewBox="0 0 16 16" aria-hidden="true"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27s1.36.09 2 .27c1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0016 8c0-4.42-3.58-8-8-8z"/></svg>';

function refreshGithubLinks() {
  const row = document.getElementById('detail-github-row');
  const container = document.getElementById('detail-github-links');
  if (!row || !container) return;

  const card = detailCard;
  const links = parseGithubUrls(card);

  if (links.length === 0) {
    row.style.display = 'none';
    container.innerHTML = '';
    return;
  }

  row.style.display = '';
  container.innerHTML = links.map(function(l) {
    const label = esc(l.owner + '/' + l.repo + ' #' + l.number);
    return '<a class="github-link" href="' + esc(l.url) + '" target="_blank" rel="noopener" data-gh="' + esc(l.owner + '/' + l.repo + '#' + l.number) + '">'
      + GH_ICON_SVG + '<span>' + label + '</span></a>';
  }).join('');

  // Fetch titles asynchronously
  links.forEach(function(l) {
    fetchGithubTitle(l.owner, l.repo, l.number).then(function(title) {
      if (!title) return;
      const el = container.querySelector('[data-gh="' + l.owner + '/' + l.repo + '#' + l.number + '"]');
      if (el && !el.querySelector('.github-link-title')) {
        el.insertAdjacentHTML('afterend', '<div class="github-link-title" title="' + esc(title) + '">' + esc(title) + '</div>');
      }
    });
  });
}

function toggleSidebarField(field) {
  const ddEl = document.getElementById('detail-' + field + '-dd');
  if (!ddEl) return;

  const isOpen = !ddEl.classList.contains('hidden');
  closeAllSidebarFields();

  if (!isOpen) {
    ddEl.classList.remove('hidden');
    // Auto-open the dropdown menu directly
    const dd = { project: ddDetailProject, column: ddDetailColumn, tag: ddDetailTag, priority: ddDetailPriority, reporter: ddDetailReporter, assignee: ddDetailAssignee, epic: ddDetailEpic }[field];
    if (dd) setTimeout(() => dd.open(), 30);
    // Focus points input
    if (field === 'points') {
      setTimeout(() => document.getElementById('detail-points').focus(), 30);
    }
  }
}

function closeSidebarField(field) {
  const ddEl = document.getElementById('detail-' + field + '-dd');
  if (!ddEl) return;
  ddEl.classList.add('hidden');
  // Close the FnDropdown too
  const dd = { project: ddDetailProject, column: ddDetailColumn, tag: ddDetailTag, priority: ddDetailPriority, reporter: ddDetailReporter, assignee: ddDetailAssignee, epic: ddDetailEpic }[field];
  if (dd) dd.close();
}

function closeAllSidebarFields() {
  SIDEBAR_FIELDS.forEach(f => closeSidebarField(f));
  if (typeof closeDueDatePicker === 'function') closeDueDatePicker();
}

// ── Description: click-to-edit ──

function renderDescriptionView() {
  const view = document.getElementById('detail-desc-view');
  if (!view) return;
  const md = detailCard ? (detailCard.description || '') : _pendingDescription;
  if (md) {
    view.innerHTML = renderMarkdownInline(md);
  } else {
    view.innerHTML = '<span class="detail-desc-placeholder">Add a description...</span>';
  }
  // Clamp long descriptions with expand/collapse toggle
  view.classList.remove('clamped');
  var toggle = document.getElementById('detail-desc-toggle');
  if (toggle) toggle.remove();
  if (md) {
    requestAnimationFrame(function() {
      if (view.scrollHeight > 170) {
        view.classList.add('clamped');
        var btn = document.createElement('button');
        btn.id = 'detail-desc-toggle';
        btn.className = 'detail-desc-toggle';
        btn.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="6 9 12 15 18 9"/></svg> Show more';
        btn.onclick = function(e) { e.stopPropagation(); toggleDescriptionClamp(); };
        view.parentNode.insertBefore(btn, view.nextSibling);
      }
    });
  }
}

function toggleDescriptionClamp() {
  var view = document.getElementById('detail-desc-view');
  var btn = document.getElementById('detail-desc-toggle');
  if (!view || !btn) return;
  var clamped = view.classList.toggle('clamped');
  if (clamped) {
    btn.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="6 9 12 15 18 9"/></svg> Show more';
  } else {
    btn.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="18 15 12 9 6 15"/></svg> Show less';
  }
}

function expandDescription() {
  document.getElementById('detail-desc-view').classList.add('hidden');
  document.getElementById('detail-desc-edit').classList.remove('hidden');
  if (detailEditor) {
    detailEditor.setMarkdown(detailCard ? detailCard.description || '' : _pendingDescription);
    setTimeout(function() { detailEditor.focus(); }, 50);
  }
}

function collapseDescription() {
  document.getElementById('detail-desc-view').classList.remove('hidden');
  document.getElementById('detail-desc-edit').classList.add('hidden');
  renderDescriptionView();
}

let _pendingDescription = '';
let _pendingComments = [];

function saveDescription() {
  if (detailEditor) {
    var md = detailEditor.getMarkdown();
    if (detailCard) {
      detailCard.description = md;
      save();
      if (currentBoardId && detailCard.id && !detailCard.id.startsWith('temp-')) {
        window.API.updateCard(detailCard.id, {
          description: detailCard.description,
          version: detailCard.version || 0,
        }).then(updated => {
          detailCard.version = updated.version;
          cardIndex[detailCard.id] = updated;
        }).catch(err => {
          if (err.status === 409) { /* 409 conflict — silently refresh */ }
          else window.Toast.error('Failed to save description');
        });
      }
    } else {
      // Create mode — store for submitCreateFromDetail
      _pendingDescription = md;
    }
  }
  collapseDescription();
  var view = document.getElementById('detail-desc-view');
  if (view) {
    view.classList.remove('saved');
    void view.offsetWidth;
    view.classList.add('saved');
    view.addEventListener('animationend', function() { view.classList.remove('saved'); }, { once: true });
  }
}

function cancelDescription() {
  collapseDescription();
}

function deleteCardFromDetail() {
  if (!detailCard || !detailCol) return;
  const cardId = detailCard.id;
  state[detailCol] = state[detailCol].filter(c => c.id !== cardId);
  detailCard = null;
  save(); window.render();
  document.getElementById('detail-modal').classList.remove('active');

  if (currentBoardId && cardId && !cardId.startsWith('temp-')) {
    window.API.deleteCard(cardId).catch(err => {
      window.Toast.error('Failed to delete card');
    });
    delete cardIndex[cardId];
  }
}

// ── Comments ──

let activeEditEditor = null;
let activeEditIdx = null;

function renderComments() {
  const list = document.getElementById('detail-comments-list');
  list.innerHTML = '';
  const comments = detailCard ? (detailCard.comments || []) : _pendingComments;
  if (!comments.length) return;

  comments.forEach((c, idx) => {
    const authorId = c.author || c.author_id;
    const user = getUserById(authorId);
    const color = AVATAR_COLORS[authorId] || user.avatar_color || 'var(--text-dimmed)';
    const initials = user.initials || '?';
    const timeStr = formatCommentTime(c.time || c.created_at);
    const isEdited = c.updated_at && c.created_at && c.updated_at !== c.created_at &&
      new Date(c.updated_at).getTime() - new Date(c.created_at).getTime() > 1000;
    const editedStr = isEdited ? formatCommentTime(c.updated_at) : '';
    const rendered = renderMarkdownInline(c.text || c.body);
    const canEdit = currentUser && (authorId === currentUser.id || currentUser.role === 'admin' || currentUser.role === 'owner');
    const canDelete = currentUser && (authorId === currentUser.id || currentUser.role === 'admin' || currentUser.role === 'owner');
    const commentAvatarContent = user.avatar_url
      ? `<img src="${esc(user.avatar_url)}" alt="${esc(initials)}">`
      : esc(initials);

    const el = document.createElement('div');
    el.className = 'detail-comment';
    el.innerHTML = `
      <div class="detail-comment-avatar" style="background:${color}30;color:${color};border:2px solid ${color}40">${commentAvatarContent}</div>
      <div class="detail-comment-body">
        <div class="detail-comment-header">
          <span class="detail-comment-author">${esc(user.label)}</span>
          <span class="detail-comment-time">${timeStr}</span>${isEdited ? `<span class="detail-comment-edited" title="Edited ${editedStr}">(edited)</span>` : ''}
        </div>
        <div class="detail-comment-text" id="comment-text-${idx}">${rendered}</div>
        <div class="detail-comment-actions" id="comment-actions-${idx}">
          ${canEdit ? `<button class="detail-comment-action" onclick="startEditComment(${idx})">Edit</button>` : ''}
          ${canDelete ? `<button class="detail-comment-action delete" onclick="deleteComment(${idx})">Delete</button>` : ''}
        </div>
      </div>
    `;
    list.appendChild(el);
  });
}

async function loadCommentsFromAPI(cardId) {
  if (!cardId || cardId.startsWith('temp-')) return;
  try {
    const comments = await window.API.listComments(cardId);
    if (detailCard && detailCard.id === cardId) {
      detailCard.comments = (comments || []).map(c => ({
        id: c.id,
        author: c.author_id,
        author_id: c.author_id,
        text: c.body,
        body: c.body,
        time: c.created_at,
        created_at: c.created_at,
        updated_at: c.updated_at,
      }));
      renderComments();
    }
  } catch (e) {
    console.error('Failed to load comments', e);
  }
}

function renderMarkdownInline(text) {
  if (!text) return '';

  // Process line by line for block-level elements
  const lines = text.split('\n');
  const out = [];
  let inCode = false;
  let codeBlock = [];
  let inList = false;

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];

    // Fenced code blocks
    if (line.trimStart().startsWith('```')) {
      if (inCode) {
        out.push('<pre><code>' + esc(codeBlock.join('\n')) + '</code></pre>');
        codeBlock = [];
        inCode = false;
      } else {
        if (inList) { out.push('</ul>'); inList = false; }
        inCode = true;
      }
      continue;
    }
    if (inCode) { codeBlock.push(line); continue; }

    // Headings
    const h1 = line.match(/^# (.+)/);
    if (h1) { if (inList) { out.push('</ul>'); inList = false; } out.push('<div class="md-h1">' + _inlineMd(h1[1]) + '</div>'); continue; }
    const h2 = line.match(/^## (.+)/);
    if (h2) { if (inList) { out.push('</ul>'); inList = false; } out.push('<div class="md-h2">' + _inlineMd(h2[1]) + '</div>'); continue; }
    const h3 = line.match(/^### (.+)/);
    if (h3) { if (inList) { out.push('</ul>'); inList = false; } out.push('<div class="md-h3">' + _inlineMd(h3[1]) + '</div>'); continue; }

    // Bullets
    const bullet = line.match(/^[\-\*] (.+)/);
    if (bullet) {
      if (!inList) { out.push('<ul class="md-list">'); inList = true; }
      out.push('<li>' + _inlineMd(bullet[1]) + '</li>');
      continue;
    }

    // End list if non-bullet line
    if (inList) { out.push('</ul>'); inList = false; }

    // Empty line
    if (line.trim() === '') { out.push('<br>'); continue; }

    // Regular line
    out.push(_inlineMd(line) + '<br>');
  }

  if (inCode) out.push('<pre><code>' + esc(codeBlock.join('\n')) + '</code></pre>');
  if (inList) out.push('</ul>');

  // Clean trailing <br>
  let html = out.join('');
  html = html.replace(/(<br>)+$/, '');
  return html;
}

function _inlineMd(line) {
  // Escape HTML first, then apply markdown.
  // Order matters: code spans and markdown links are processed first so
  // their URLs are wrapped in tags.  The final bare-URL pass skips any
  // URL already inside an href="..." or between <code>...</code> tags.
  let s = esc(line)
    .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
    .replace(/\*(.+?)\*/g, '<em>$1</em>')
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    .replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>');

  // Auto-linkify bare URLs that are NOT inside <code>…</code> or <a>…</a>.
  // Split the string on existing tags, only linkify in text segments.
  s = s.replace(/(<code>[\s\S]*?<\/code>|<a\s[^>]*>[\s\S]*?<\/a>)|((https?:\/\/[^\s<)]+))/g,
    function(match, skip, url) {
      if (skip) return skip;                       // inside <code> or <a> — leave alone
      return '<a href="' + url + '" target="_blank" rel="noopener">' + url + '</a>';
    });
  return s;
}

// ── Add comment: click-to-expand ──

function expandCommentInput() {
  document.getElementById('comment-fake-input').style.display = 'none';
  const expanded = document.getElementById('comment-expanded');
  expanded.classList.remove('hidden');

  // Create editor if needed
  if (!commentEditor) {
    commentEditor = new window.FnEditor(document.getElementById('comment-editor-container'), {
      compact: true,
      placeholder: 'Write a comment...',
      onSubmit: function() { submitComment(); },
    });
  }
  commentEditor.clear();
  setTimeout(() => commentEditor.focus(), 50);
}

function collapseCommentInput() {
  document.getElementById('comment-fake-input').style.display = '';
  document.getElementById('comment-expanded').classList.add('hidden');
}

function submitComment() {
  if (!commentEditor) return;
  const md = commentEditor.getMarkdown().trim();
  if (!md) return;

  const authorId = currentUser ? currentUser.id : 'you';
  const tempComment = {
    id: 'temp-' + Date.now(),
    author: authorId,
    author_id: authorId,
    text: md,
    body: md,
    time: new Date().toISOString(),
    created_at: new Date().toISOString(),
  };

  if (!detailCard) {
    // Create mode — stash locally
    _pendingComments.push(tempComment);
    renderComments();
    collapseCommentInput();
    return;
  }

  detailCard.comments = detailCard.comments || [];
  detailCard.comments.push(tempComment);

  save();
  renderComments();
  collapseCommentInput();
  // Animate the new comment
  var list = document.getElementById('detail-comments-list');
  var last = list ? list.lastElementChild : null;
  if (last) {
    last.classList.add('new');
    last.addEventListener('animationend', function() { last.classList.remove('new'); }, { once: true });
  }

  // Sync with API
  if (currentBoardId && detailCard.id && !detailCard.id.startsWith('temp-')) {
    window.API.createComment(detailCard.id, md).then(serverComment => {
      const idx = detailCard.comments.findIndex(c => c.id === tempComment.id);
      if (idx !== -1) {
        detailCard.comments[idx] = {
          id: serverComment.id,
          author: serverComment.author_id,
          author_id: serverComment.author_id,
          text: serverComment.body || md,
          body: serverComment.body || md,
          time: serverComment.created_at || tempComment.time,
          created_at: serverComment.created_at || tempComment.time,
        };
      }
    }).catch(err => {
      window.Toast.error('Failed to save comment');
    });
  }
}

// ── Edit existing comment ──

function startEditComment(idx) {
  const comments = detailCard ? detailCard.comments : _pendingComments;
  const comment = comments[idx];
  if (!comment) return;

  // Hide the text + actions, show editor
  const textEl = document.getElementById('comment-text-' + idx);
  const actionsEl = document.getElementById('comment-actions-' + idx);
  if (!textEl || !actionsEl) return;

  textEl.style.display = 'none';
  actionsEl.style.display = 'none';

  // Create edit zone
  const zone = document.createElement('div');
  zone.className = 'detail-comment-edit-zone';
  zone.id = 'comment-edit-zone-' + idx;

  const editorContainer = document.createElement('div');
  zone.appendChild(editorContainer);

  const actions = document.createElement('div');
  actions.className = 'detail-comment-edit-actions';
  actions.innerHTML = `
    <button class="lwts-modal-btn-primary" style="height:32px;padding:0 14px;font-size:0.8rem;" onclick="saveEditComment(${idx})">Save</button>
    <button class="lwts-modal-btn-cancel" style="height:32px;padding:0 12px;font-size:0.8rem;" onclick="cancelEditComment(${idx})">Cancel</button>
  `;
  zone.appendChild(actions);

  actionsEl.parentNode.insertBefore(zone, actionsEl.nextSibling);

  // Init editor with current content
  activeEditIdx = idx;
  activeEditEditor = new window.FnEditor(editorContainer, {
    compact: true,
    placeholder: 'Edit comment...',
  });
  activeEditEditor.setMarkdown(comment.text);
  setTimeout(() => activeEditEditor.focus(), 50);
}

function saveEditComment(idx) {
  if (!activeEditEditor) return;
  const md = activeEditEditor.getMarkdown().trim();
  if (!md) return;

  const comments = detailCard ? detailCard.comments : _pendingComments;
  const comment = comments[idx];
  comment.text = md;
  comment.body = md;
  comment.updated_at = new Date().toISOString();
  if (detailCard) save();

  activeEditEditor = null;
  activeEditIdx = null;
  renderComments();
  var textEl = document.getElementById('comment-text-' + idx);
  if (textEl) {
    textEl.classList.add('saved');
    textEl.addEventListener('animationend', function() { textEl.classList.remove('saved'); }, { once: true });
  }

  // Sync with API
  if (comment.id && !comment.id.startsWith('temp-')) {
    window.API.updateComment(comment.id, md).then(updated => {
      comment.updated_at = updated.updated_at;
      renderComments();
    }).catch(() => window.Toast.error('Failed to save comment edit'));
  }
}

function cancelEditComment(idx) {
  activeEditEditor = null;
  activeEditIdx = null;
  renderComments();
}

async function deleteComment(idx) {
  const comments = detailCard ? detailCard.comments : _pendingComments;
  const ok = await window.fnConfirm('Delete this comment?', 'Delete comment', 'Delete');
  if (!ok) return;
  const comment = comments[idx];
  var list = document.getElementById('detail-comments-list');
  var el = list ? list.children[idx] : null;

  function doDelete() {
    comments.splice(idx, 1);
    if (detailCard) save();
    renderComments();

    // Sync with API
    if (comment && comment.id && !comment.id.startsWith('temp-')) {
      window.API.deleteComment(comment.id).catch(() => {});
    }
  }

  if (el) {
    el.classList.add('removing');
    el.addEventListener('animationend', doDelete, { once: true });
  } else {
    doDelete();
  }
}

// ── Attachments ──

function renderAttachments() {
  const grid = document.getElementById('detail-attachments');
  if (!grid || !detailCard) { if (grid) grid.innerHTML = ''; return; }
  grid.innerHTML = '';

  (detailCard.attachments || []).forEach((url, idx) => {
    const el = document.createElement('div');
    el.className = 'detail-attachment';
    el.innerHTML = `
      <img src="${esc(url)}" alt="attachment" loading="lazy">
      <button class="detail-attachment-remove" onclick="event.stopPropagation(); removeAttachment(${idx})">&times;</button>
    `;
    el.addEventListener('click', () => openLightbox(url));
    grid.appendChild(el);
  });
}

function addAttachment(url) {
  if (!detailCard) return;
  detailCard.attachments = detailCard.attachments || [];
  detailCard.attachments.push(url);
  save();
  renderAttachments();
  // Attachments stored locally for now — file upload API not yet implemented
}

function removeAttachment(idx) {
  if (!detailCard) return;
  detailCard.attachments = detailCard.attachments || [];
  detailCard.attachments.splice(idx, 1);
  save();
  renderAttachments();
}

function openLightbox(url) {
  const lb = document.getElementById('lightbox');
  document.getElementById('lightbox-img').src = url;
  lb.classList.add('active');
}

function closeLightbox() {
  document.getElementById('lightbox').classList.remove('active');
}

function copyCardLink() {
  if (!detailCard) return;
  const url = window.location.origin + window.location.pathname + '#' + detailCard.key;
  navigator.clipboard.writeText(url).then(() => {
    const el = document.getElementById('detail-key-text');
    const orig = el.textContent;
    el.textContent = 'Copied!';
    setTimeout(() => { el.textContent = orig; }, 1200);
  });
}

// ═══════════════════════════════════════════════════════════════
// BOARD PICKER
// ═══════════════════════════════════════════════════════════════

function toggleBoardPicker() {
  const menu = document.getElementById('board-menu');
  const picker = document.getElementById('board-picker');
  const isHidden = menu.classList.contains('hidden');
  if (isHidden) {
    menu.classList.remove('hidden');
    requestAnimationFrame(() => menu.classList.add('visible'));
    picker.classList.add('open');
    document.addEventListener('click', closeBoardPickerOnClick, true);
  } else {
    closeBoardPicker();
  }
}

function closeBoardPicker() {
  const menu = document.getElementById('board-menu');
  menu.classList.remove('visible');
  document.getElementById('board-picker').classList.remove('open');
  setTimeout(() => menu.classList.add('hidden'), 200);
  document.removeEventListener('click', closeBoardPickerOnClick, true);
}

function closeBoardPickerOnClick(e) {
  if (!document.querySelector('.header-left').contains(e.target)) {
    closeBoardPicker();
  }
}

function switchBoard(id, name) {
  // Delegate to selectBoard (features.js) if available — it handles SSE + presence
  if (typeof window.selectBoard === "function") {
    window.selectBoard(id, name || id);
  } else {
    currentBoardId = id;
    localStorage.setItem('lwts-board-id', id);
    // Update URL with board ID for persistence across refreshes
    const url = new URL(window.location);
    url.searchParams.set('board', id);
    window.history.replaceState(null, '', url);
    document.getElementById('board-picker-label').textContent = _capitalize(name || id);
    closeBoardPicker();
    loadBoardCards(id);
  }
  window.renderBoardPicker();
}

function createBoard() {
  // Delegate to the modal in features.js if available
  if (typeof window.openNewBoardModal === "function") {
    window.openNewBoardModal();
  } else {
    const name = prompt('Board name:');
    if (!name) return;
    closeBoardPicker();
    window.API.createBoard({ name, project_key: name.replace(/\s+/g, '-').toUpperCase().substring(0, 6) })
      .then(board => {
        boardList.push(board);
        window.renderBoardPicker();
        switchBoard(board.id, board.name);
      })
      .catch(err => {
        window.Toast.error('Failed to create board: ' + (err.message || 'unknown'));
      });
  }
}

// ═══════════════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════════════

function formatDate(d) {
  const dt = new Date(d + 'T00:00:00');
  const months = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec'];
  return `${months[dt.getMonth()]} ${dt.getDate()}`;
}

function formatDateLong(d) {
  const dt = new Date(d + 'T00:00:00');
  const months = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec'];
  return `${months[dt.getMonth()]} ${dt.getDate()}, ${dt.getFullYear()}`;
}

function formatCommentTime(iso) {
  if (!iso) return '';
  const dt = new Date(iso);
  const months = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec'];
  const h = dt.getHours();
  const m = dt.getMinutes().toString().padStart(2, '0');
  const ampm = h >= 12 ? 'PM' : 'AM';
  const h12 = h % 12 || 12;
  return `${months[dt.getMonth()]} ${dt.getDate()} at ${h12}:${m} ${ampm}`;
}

function autoResizeTextarea(el) {
  el.style.height = 'auto';
  el.style.height = el.scrollHeight + 'px';
}

function clearCompleted() {
  const doneCards = state.done.slice();
  if (doneCards.length === 0) return;

  // Collect DOM elements to animate (board cards + list rows)
  const isList = typeof window.currentView !== 'undefined' && window.currentView === 'list';
  const cardEls = isList
    ? Array.from(document.querySelectorAll('.list-row[data-col="done"]'))
    : Array.from(document.querySelectorAll('.column-body[data-col="done"] .card'));

  if (cardEls.length === 0) {
    // No visible elements — fall through to instant clear
    _finishClear(doneCards);
    return;
  }

  // Respect prefers-reduced-motion — skip animation entirely
  const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  if (reducedMotion) {
    _finishClear(doneCards);
    return;
  }

  // Stagger the .clearing class across cards
  const STAGGER = 60; // ms between each card
  cardEls.forEach((el, i) => {
    el.style.animationDelay = (i * STAGGER) + 'ms';
    el.classList.add('clearing');
  });

  // Wait for the last card's animation to finish, then commit the state change
  const lastEl = cardEls[cardEls.length - 1];
  const totalDuration = (cardEls.length - 1) * STAGGER + 300; // 300ms = animation duration

  function onDone() {
    lastEl.removeEventListener('animationend', onDone);
    clearTimeout(fallback);
    _finishClear(doneCards);
  }
  lastEl.addEventListener('animationend', onDone, { once: true });
  // Safety fallback in case animationend never fires
  const fallback = setTimeout(onDone, totalDuration + 50);
}

function _finishClear(doneCards) {
  state.done = [];
  if (!state.cleared) state.cleared = [];
  state.cleared.push(...doneCards);
  save(); window.render();
  if (typeof window.currentView !== 'undefined' && window.currentView === 'list' && typeof renderListView === 'function') {
    window.renderListView();
  }

  // Move cleared cards to 'cleared' column via API
  if (currentBoardId) {
    doneCards.forEach((card, i) => {
      if (card.id && !card.id.startsWith('temp-')) {
        window.API.moveCard(card.id, {
          column_id: 'cleared',
          position: state.cleared.length - doneCards.length + i,
          version: card.version || 0,
        }).then(updated => {
          card.version = updated.version;
          cardIndex[card.id] = updated;
        }).catch(() => {});
      }
    });
  }
}

function esc(s) {
  const d = document.createElement('div');
  d.textContent = s;
  return d.innerHTML;
}

function isInputFocused() {
  const tag = document.activeElement?.tagName;
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT';
}

function anyModalOpen() {
  return document.getElementById('detail-modal').classList.contains('active');
}

// ═══════════════════════════════════════════════════════════════
// KEYBOARD & INIT
// ═══════════════════════════════════════════════════════════════

document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape') {
    if (document.getElementById('lightbox').classList.contains('active')) { closeLightbox(); return; }
    if (document.getElementById('detail-modal').classList.contains('active')) { closeDetail(); return; }
  }
  if (e.key === 'n' && !e.ctrlKey && !e.metaKey && !anyModalOpen() && !isInputFocused()) {
    e.preventDefault();
    openCreateModal();
  }
  // Ctrl/Cmd+F → focus search bar
  if (e.key === 'f' && (e.ctrlKey || e.metaKey)) {
    const search = document.getElementById('header-search');
    if (search) {
      e.preventDefault();
      search.focus();
      search.select();
    }
  }
});

document.addEventListener('DOMContentLoaded', () => {
  // Universal: click overlay background to close any modal
  document.querySelectorAll('.lwts-modal-overlay').forEach(overlay => {
    overlay.addEventListener('click', (e) => {
      if (e.target !== overlay) return;
      overlay.classList.remove('active');
      // Also handle create mode cleanup
      if (overlay.id === 'detail-modal' && _isCreateMode) { closeCreateMode(); return; }
      if (overlay.id === 'detail-modal') { closeDetail(); return; }
      if (overlay.id === 'confirm-modal') { window.fnConfirmResolve(false); return; }
    });
  });

  // Detail: auto-resize title + sync to header
  var titleEl = document.getElementById('detail-title');
  titleEl.addEventListener('input', function() {
    autoResizeTextarea(this);
    document.getElementById('detail-header-title').textContent = this.value;
  });
  titleEl.addEventListener('keydown', function(e) {
    if (e.key === 'Enter') { e.preventDefault(); this.blur(); }
  });
  titleEl.addEventListener('blur', function() {
    if (detailCard && this.value.trim()) {
      this.classList.remove('saved');
      void this.offsetWidth;
      this.classList.add('saved');
      this.addEventListener('animationend', function() { this.classList.remove('saved'); }, { once: true });
    }
  });

  initDetailDropdowns();

  // WYSIWYG editor
  detailEditor = new window.FnEditor(document.getElementById('detail-description'), {
    placeholder: 'Add a description...',
    onImageUpload: (url) => addAttachment(url),
  });

  // Initialize toast system
  window.Toast.init();

  // Initialize global search
  initGlobalSearch();

  // Appearance and settings are loaded in initBoard() after auth

  // Clean up old localStorage data
  localStorage.removeItem('lwts-kanban');
});

// Board data init — called after auth succeeds (SPA flow)
window.initBoard = function() {
  if (typeof window.initAppearance === "function") window.initAppearance();
  if (typeof window.loadSettings === "function") window.loadSettings('general');
  loadUsers().then(() => {
    reinitUserDropdowns();
    if (typeof window.buildFilterCheckboxes === "function") {
      window.buildFilterCheckboxes('assignee', USERS.filter(u => u.value !== 'unassigned').map(u => ({ value: u.value, label: u.label, initials: u.initials, avatar_color: u.avatar_color, avatar_url: u.avatar_url })));
    }
    loadFromAPI().then(() => {
      if (location.hash) navigateToHash();
    });
    initUserMenu();
  });
};

function reinitUserDropdowns() {
  const userOpts = USERS.map(u => ({ value: u.value, label: u.label }));
  if (ddDetailReporter) ddDetailReporter.setOptions(userOpts);
  if (ddDetailAssignee) ddDetailAssignee.setOptions(userOpts);
}

// ═══════════════════════════════════════════════════════════════
// GLOBAL SEARCH
// ═══════════════════════════════════════════════════════════════

let searchDebounce = null;

function initGlobalSearch() {
  const input = document.getElementById('header-search');
  if (!input) return;

  input.addEventListener('input', () => {
    const q = input.value.trim();
    document.getElementById('header-search-clear').classList.toggle('hidden', !q);
    clearTimeout(searchDebounce);
    if (!q) {
      filterCardsInline('');
      if (typeof window.filterListInline === "function") window.filterListInline('');
      hideSearchResults();
      return;
    }
    searchDebounce = setTimeout(() => {
      // Filter cards inline in whichever view is visible
      const board = document.getElementById('board');
      if (board && board.style.display !== 'none') {
        filterCardsInline(q.toLowerCase());
      }
      if (typeof window.filterListInline === "function" && typeof window.currentView !== "undefined" && window.currentView === 'list') {
        window.filterListInline(q.toLowerCase());
      }
      doGlobalSearch(q);
    }, 250);
  });

  input.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
      clearGlobalSearch();
      input.blur();
    }
  });

  document.addEventListener('click', (e) => {
    if (!document.getElementById('header-search-wrap').contains(e.target)) {
      hideSearchResults();
    }
  });
}

function filterCardsInline(query) {
  const allCards = document.querySelectorAll('.card');

  allCards.forEach(cardEl => {
    if (!query) {
      if (cardEl.classList.contains('search-hidden')) {
        cardEl.classList.remove('search-hidden');
        cardEl.classList.add('search-reveal');
        cardEl.addEventListener('animationend', () => cardEl.classList.remove('search-reveal'), { once: true });
      }
      return;
    }

    const title = (cardEl.querySelector('.card-title') || {}).textContent || '';
    const key = (cardEl.querySelector('.card-key') || {}).textContent || '';
    const tag = (cardEl.querySelector('.card-tag') || {}).textContent || '';
    const match = title.toLowerCase().includes(query) ||
                  key.toLowerCase().includes(query) ||
                  tag.toLowerCase().includes(query);

    if (match) {
      if (cardEl.classList.contains('search-hidden')) {
        cardEl.classList.remove('search-hidden');
        cardEl.classList.add('search-reveal');
        cardEl.addEventListener('animationend', () => cardEl.classList.remove('search-reveal'), { once: true });
      }
    } else {
      cardEl.classList.add('search-hidden');
      cardEl.classList.remove('search-reveal');
    }
  });

  // Update column counts in headers
  document.querySelectorAll('.column-header').forEach(hdr => {
    const countEl = hdr.querySelector('.column-count');
    if (!countEl) return;
    const dot = hdr.querySelector('.column-dot');
    if (!dot) return;
    const colId = [...dot.classList].find(c => c !== 'column-dot') || '';
    const allInCol = document.querySelectorAll('.column-body[data-col="' + colId + '"] .card');
    const visibleInCol = document.querySelectorAll('.column-body[data-col="' + colId + '"] .card:not(.search-hidden)');
    countEl.textContent = query ? visibleInCol.length + '/' + allInCol.length : allInCol.length;
  });

  // Hide epic lanes with no visible cards
  document.querySelectorAll('.epic-lane').forEach(lane => {
    const visibleCards = lane.querySelectorAll('.card:not(.search-hidden)').length;
    lane.style.display = (query && visibleCards === 0) ? 'none' : '';
  });
}

async function doGlobalSearch(query) {
  const results = document.getElementById('header-search-results');
  results.classList.remove('hidden');
  results.innerHTML = '<div class="search-loading">Searching...</div>';

  try {
    const cards = await window.API.searchCards(query, currentBoardId);
    if (!cards || cards.length === 0) {
      results.innerHTML = '<div class="search-empty">No cards matching \'' + esc(query) + '\'</div>';
      return;
    }
    results.innerHTML = '';
    cards.slice(0, 10).forEach(card => {
      const col = COLUMNS.find(c => c.id === card.column_id);
      const el = document.createElement('div');
      el.className = 'search-result-item';
      el.innerHTML = '<span class="search-result-key">' + esc(card.key || '') + '</span>' +
        '<span class="search-result-title">' + esc(card.title) + '</span>' +
        '<span class="search-result-col">' + esc(col ? col.label : card.column_id) + '</span>';
      el.addEventListener('click', () => {
        hideSearchResults();
        clearGlobalSearch();
        // If in settings, close settings first then open the card
        const settingsPage = document.getElementById('settings-page');
        if (settingsPage && settingsPage.classList.contains('active')) {
          window.closeSettings();
          setTimeout(() => window.openDetail(card.id), 150);
        } else {
          window.openDetail(card.id);
        }
      });
      results.appendChild(el);
    });
  } catch (e) {
    results.innerHTML = '<div class="search-empty">Search failed</div>';
  }
}

function clearGlobalSearch() {
  const input = document.getElementById('header-search');
  input.value = '';
  document.getElementById('header-search-clear').classList.add('hidden');
  hideSearchResults();
  const board = document.getElementById('board');
  if (board && board.style.display !== 'none') {
    filterCardsInline('');
  }
}

function hideSearchResults() {
  const el = document.getElementById('header-search-results');
  if (el) el.classList.add('hidden');
}

// ── User Menu ──

function initUserMenu() {
  const avatar = document.getElementById('header-user-avatar');
  if (!avatar || !currentUser) return;
  const u = getUserById(currentUser.id);
  const color = u.avatar_color || '#82B1FF';
  if (u.avatar_url) {
    avatar.innerHTML = `<img src="${u.avatar_url}">`;
    avatar.style.background = 'transparent';
    avatar.style.borderColor = color + '40';
  } else {
    avatar.textContent = u.initials || '?';
    avatar.style.background = color + '30';
    avatar.style.color = color;
    avatar.style.borderColor = color + '40';
  }

  const infoRow = document.getElementById('header-user-info-row');
  if (infoRow) {
    const miniAvatar = u.avatar_url
      ? `<img src="${u.avatar_url}" style="width:32px;height:32px;border-radius:50%;object-fit:cover">`
      : `<div style="width:32px;height:32px;border-radius:50%;display:flex;align-items:center;justify-content:center;font-size:0.6rem;font-weight:600;background:${color}30;color:${color};border:2px solid ${color}40;text-transform:uppercase">${u.initials || '?'}</div>`;
    infoRow.innerHTML = `${miniAvatar}<div><div class="header-user-info-name">${currentUser.name || ''}</div><div class="header-user-info-email">${currentUser.email || ''}</div></div>`;
  }

  // Update theme label
  const label = document.getElementById('theme-toggle-label');
  if (label) label.textContent = document.body.classList.contains('light-theme') ? 'Dark mode' : 'Light mode';
}

function toggleUserMenu() {
  const dd = document.getElementById('header-user-dropdown');
  if (dd.classList.contains('hidden')) {
    dd.classList.remove('hidden');
    setTimeout(() => document.addEventListener('click', _userMenuOutside, true), 10);
  } else {
    closeUserMenu();
  }
}

function closeUserMenu() {
  document.getElementById('header-user-dropdown').classList.add('hidden');
  document.removeEventListener('click', _userMenuOutside, true);
}

function _userMenuOutside(e) {
  const menu = document.getElementById('header-user-menu');
  if (menu && !menu.contains(e.target)) closeUserMenu();
}

function toggleTheme() {
  const isLight = document.body.classList.contains('light-theme');
  document.body.classList.toggle('light-theme', !isLight);
  const label = document.getElementById('theme-toggle-label');
  if (label) label.textContent = isLight ? 'Light mode' : 'Dark mode';
  // Save preference
  if (!window._settingsCache['appearance']) window._settingsCache['appearance'] = {};
  window._settingsCache['appearance'].dark_mode = isLight;
  window.API.putSettings('appearance', { dark_mode: isLight }).catch(() => {});
  // Update checkbox in settings if visible
  const cb = document.querySelector('[data-setting="appearance.dark_mode"]');
  if (cb) cb.checked = isLight;
}

function switchAccount() {
  closeUserMenu();
  if (window.Auth.clearTokens) window.Auth.clearTokens();
  location.reload();
}

function logOut() {
  closeUserMenu();
  window.Auth.showAuth();
}

// ═══════════════════════════════════════════════════════════════
// URL HASH ROUTING
// ═══════════════════════════════════════════════════════════════

// window._suppressHashChange is declared in settings.js (loads first)

function findCardByKey(key) {
  for (const col of COLUMNS) {
    const card = (state[col.id] || []).find(c => c.key === key);
    if (card) return card;
  }
  return null;
}

function navigateToHash() {
  const hash = location.hash.replace(/^#/, '');
  if (!hash) {
    // No hash — close any open modal/settings
    const detailModal = document.getElementById('detail-modal');
    if (detailModal && detailModal.classList.contains('active')) {
      closeDetail(true);
    }
    const settingsPage = document.getElementById('settings-page');
    if (settingsPage && settingsPage.classList.contains('active')) {
      window.closeSettings(true);
    }
    return;
  }

  // Settings: #settings or #settings/section
  if (hash === 'settings' || hash.startsWith('settings/')) {
    const section = hash.includes('/') ? hash.split('/')[1] : 'general';
    const settingsPage = document.getElementById('settings-page');

    // Close detail modal if open
    const detailModal = document.getElementById('detail-modal');
    if (detailModal && detailModal.classList.contains('active')) {
      closeDetail(true);
    }

    // Set section before opening so openSettings picks it up
    if (typeof window.activeSettingsSection !== "undefined") {
      window.activeSettingsSection = section;
    }

    if (!settingsPage || !settingsPage.classList.contains('active')) {
      window.openSettings(true);
    } else {
      // Already in settings, just switch section
      window.showSettingsSection(section, true);
    }
    return;
  }

  // Card key: PREFIX-NUMBER pattern (e.g. LWTS-9)
  if (/^[A-Z]+-\d+$/i.test(hash)) {
    // Close settings if open
    const settingsPage = document.getElementById('settings-page');
    if (settingsPage && settingsPage.classList.contains('active')) {
      window.closeSettings(true);
    }

    const card = findCardByKey(hash.toUpperCase());
    if (card) {
      window.openDetail(card.id, true);
    }
    return;
  }
}

window.addEventListener('hashchange', () => {
  if (window._suppressHashChange) return;
  navigateToHash();
});

// ═══════════════════════════════════════════════════════════════
// WINDOW EXPORTS — expose everything for HTML onclick handlers
// and cross-file monkey-patching (features.js, subtasks.js, a11y.js)
// ═══════════════════════════════════════════════════════════════

window.COLUMNS = COLUMNS;
window.TAG_LABELS = TAG_LABELS;
window.TAG_COLORS = TAG_COLORS;
window.EPIC_LANE_COLORS = EPIC_LANE_COLORS;
window._getEpicColors = _getEpicColors;
window.PRIORITIES = PRIORITIES;
Object.defineProperty(window, 'USERS', { get() { return USERS; }, set(v) { USERS = v; }, configurable: true });
Object.defineProperty(window, 'AVATAR_COLORS', { get() { return AVATAR_COLORS; }, set(v) { AVATAR_COLORS = v; }, configurable: true });
Object.defineProperty(window, 'currentUser', { get() { return currentUser; }, set(v) { currentUser = v; }, configurable: true });
window.loadUsers = loadUsers;
window.getUserById = getUserById;
window.PRIORITY_ICONS = PRIORITY_ICONS;
Object.defineProperty(window, 'state', { get() { return state; }, set(v) { state = v; }, configurable: true });
Object.defineProperty(window, 'nextId', { get() { return nextId; }, set(v) { nextId = v; }, configurable: true });
Object.defineProperty(window, 'currentBoardId', { get() { return currentBoardId; }, set(v) { currentBoardId = v; }, configurable: true });
Object.defineProperty(window, 'boardList', { get() { return boardList; }, set(v) { boardList = v; }, configurable: true });
Object.defineProperty(window, 'cardIndex', { get() { return cardIndex; }, set(v) { cardIndex = v; }, configurable: true });
window.fromAPI = fromAPI;
window.showWelcome = showWelcome;
window.closeWelcome = closeWelcome;
window.loadFromAPI = loadFromAPI;
window.loadBoardCards = loadBoardCards;
window.loadFromLocalStorage = loadFromLocalStorage;
window.save = save;
window.render = render;
window.createCardEl = createCardEl;
window.onDragStart = onDragStart;
window.onDragEnd = onDragEnd;
window.onDragOver = onDragOver;
window.onDragLeave = onDragLeave;
window.onDrop = onDrop;
window.clearDropTarget = clearDropTarget;
window.openCreateModal = openCreateModal;
window.closeCreateMode = closeCreateMode;
window.submitCreateFromDetail = submitCreateFromDetail;
window.initDetailDropdowns = initDetailDropdowns;
window.openDetail = openDetail;
window.closeDetail = closeDetail;
window.saveDetailFields = saveDetailFields;
window.detailFieldChanged = detailFieldChanged;
window.refreshSidebarTexts = refreshSidebarTexts;
window.toggleSidebarField = toggleSidebarField;
window.closeSidebarField = closeSidebarField;
window.closeAllSidebarFields = closeAllSidebarFields;
window.renderDescriptionView = renderDescriptionView;
window.toggleDescriptionClamp = toggleDescriptionClamp;
window.expandDescription = expandDescription;
window.collapseDescription = collapseDescription;
window.saveDescription = saveDescription;
window.cancelDescription = cancelDescription;
window.deleteCardFromDetail = deleteCardFromDetail;
window.renderComments = renderComments;
window.loadCommentsFromAPI = loadCommentsFromAPI;
window.renderMarkdownInline = renderMarkdownInline;
window.expandCommentInput = expandCommentInput;
window.collapseCommentInput = collapseCommentInput;
window.submitComment = submitComment;
window.startEditComment = startEditComment;
window.saveEditComment = saveEditComment;
window.cancelEditComment = cancelEditComment;
window.deleteComment = deleteComment;
window.renderAttachments = renderAttachments;
window.addAttachment = addAttachment;
window.removeAttachment = removeAttachment;
window.openLightbox = openLightbox;
window.closeLightbox = closeLightbox;
window.copyCardLink = copyCardLink;
window.toggleBoardPicker = toggleBoardPicker;
window.closeBoardPicker = closeBoardPicker;
window.closeBoardPickerOnClick = closeBoardPickerOnClick;
window.switchBoard = switchBoard;
window.createBoard = createBoard;
window.formatDate = formatDate;
window.formatDateLong = formatDateLong;
window.formatCommentTime = formatCommentTime;
window.autoResizeTextarea = autoResizeTextarea;
window.clearCompleted = clearCompleted;
window.esc = esc;
window.isInputFocused = isInputFocused;
window.anyModalOpen = anyModalOpen;
window.reinitUserDropdowns = reinitUserDropdowns;
window.initGlobalSearch = initGlobalSearch;
window.filterCardsInline = filterCardsInline;
window.doGlobalSearch = doGlobalSearch;
window.clearGlobalSearch = clearGlobalSearch;
window.hideSearchResults = hideSearchResults;
window.initUserMenu = initUserMenu;
window.toggleUserMenu = toggleUserMenu;
window.closeUserMenu = closeUserMenu;
window.toggleTheme = toggleTheme;
window.switchAccount = switchAccount;
window.logOut = logOut;
window.findCardByKey = findCardByKey;
window.navigateToHash = navigateToHash;
window.renderBoardPicker = renderBoardPicker;
window.SIDEBAR_FIELDS = SIDEBAR_FIELDS;
window.refreshGithubLinks = refreshGithubLinks;
window.parseGithubUrls = parseGithubUrls;

// Mutable state — defineProperty so cross-file reads see current values
Object.defineProperty(window, 'detailCard', { get() { return detailCard; }, set(v) { detailCard = v; }, configurable: true });
Object.defineProperty(window, 'detailCol', { get() { return detailCol; }, set(v) { detailCol = v; }, configurable: true });
Object.defineProperty(window, 'ddDetailColumn', { get() { return ddDetailColumn; }, set(v) { ddDetailColumn = v; }, configurable: true });
Object.defineProperty(window, 'ddDetailTag', { get() { return ddDetailTag; }, set(v) { ddDetailTag = v; }, configurable: true });
Object.defineProperty(window, 'ddDetailPriority', { get() { return ddDetailPriority; }, set(v) { ddDetailPriority = v; }, configurable: true });
Object.defineProperty(window, 'ddDetailReporter', { get() { return ddDetailReporter; }, set(v) { ddDetailReporter = v; }, configurable: true });
Object.defineProperty(window, 'ddDetailAssignee', { get() { return ddDetailAssignee; }, set(v) { ddDetailAssignee = v; }, configurable: true });
Object.defineProperty(window, 'ddDetailProject', { get() { return ddDetailProject; }, set(v) { ddDetailProject = v; }, configurable: true });
Object.defineProperty(window, 'detailEditor', { get() { return detailEditor; }, set(v) { detailEditor = v; }, configurable: true });
Object.defineProperty(window, 'commentEditor', { get() { return commentEditor; }, set(v) { commentEditor = v; }, configurable: true });
Object.defineProperty(window, '_isCreateMode', { get() { return _isCreateMode; }, set(v) { _isCreateMode = v; }, configurable: true });
Object.defineProperty(window, '_renderAnimateCards', { get() { return _renderAnimateCards; }, set(v) { _renderAnimateCards = v; }, configurable: true });
Object.defineProperty(window, '_updateCreateBtnState', { get() { return _updateCreateBtnState; }, configurable: true });
Object.defineProperty(window, 'dragCard', { get() { return dragCard; }, set(v) { dragCard = v; }, configurable: true });
Object.defineProperty(window, 'dragSourceCol', { get() { return dragSourceCol; }, set(v) { dragSourceCol = v; }, configurable: true });
Object.defineProperty(window, 'dragEl', { get() { return dragEl; }, set(v) { dragEl = v; }, configurable: true });
Object.defineProperty(window, 'didDrag', { get() { return didDrag; }, set(v) { didDrag = v; }, configurable: true });
Object.defineProperty(window, 'activeEditEditor', { get() { return activeEditEditor; }, set(v) { activeEditEditor = v; }, configurable: true });
Object.defineProperty(window, 'activeEditIdx', { get() { return activeEditIdx; }, set(v) { activeEditIdx = v; }, configurable: true });
Object.defineProperty(window, 'searchDebounce', { get() { return searchDebounce; }, set(v) { searchDebounce = v; }, configurable: true });
