// fn2 Kanban — UI Features: Multi-Board, Filtering, Due Dates
// This file adds features on top of kanban.js without modifying existing functions directly.

// ═══════════════════════════════════════════════════════════════
// 1. MULTI-BOARD SUPPORT
// ═══════════════════════════════════════════════════════════════

// currentBoardId and boardList are declared in kanban.js — reuse them
let currentBoardStream = null;

function connectBoardStream(boardId) {
  if (currentBoardStream) {
    currentBoardStream.disconnect();
    currentBoardStream = null;
  }
  if (typeof window.BoardStream !== 'undefined') {
    currentBoardStream = new window.BoardStream(boardId, {});
    window.currentBoardStream = currentBoardStream;
    if (typeof window.wirePresenceHandlers === 'function') window.wirePresenceHandlers(currentBoardStream);
    currentBoardStream.connect();
  }
}

// Populate board picker from API
async function loadBoardList() {
  // kanban.js loadFromAPI() already fetches boards and sets boardList/currentBoardId.
  // If boards are already loaded, just ensure the SSE stream is connected.
  if (window.boardList && window.boardList.length > 0) {
    if (!currentBoardStream && window.currentBoardId) {
      connectBoardStream(window.currentBoardId);
    }
    return;
  }
  try {
    if (typeof window.API !== 'undefined') {
      window.boardList = await window.API.listBoards();
    }
    if (!window.boardList || !window.boardList.length) return;
    renderBoardPicker();
    if (!window.currentBoardId) {
      selectBoard(window.boardList[0].id, window.boardList[0].name);
    }
  } catch {
    // API not available yet — use local state
  }
}

function _cap(s) { return s ? s.charAt(0).toUpperCase() + s.slice(1) : s; }

const _gearIcon = '<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>';

// Shared menu builder used by both board picker and settings picker
function _buildPickerMenu(menu, opts) {
  menu.innerHTML = '';
  const closeMenu = opts.closeMenu || function() {};
  const activeId = opts.activeId || null; // 'settings' or a board id
  const inSettings = opts.inSettings || false;

  // Board options
  window.boardList.forEach((board, i) => {
    const opt = document.createElement('div');
    opt.className = 'header-board-option' + (activeId === board.id ? ' active' : '');
    opt.style.cssText = 'display:flex;align-items:center;justify-content:space-between';

    const nameSpan = document.createElement('span');
    nameSpan.textContent = _cap(board.name);
    nameSpan.style.flex = '1';
    nameSpan.onclick = () => {
      closeMenu();
      if (inSettings && typeof window.closeSettings === 'function') window.closeSettings();
      selectBoard(board.id, board.name);
    };
    opt.appendChild(nameSpan);

    // Delete button for non-first boards (only in board picker, not settings)
    if (!inSettings && i > 0) {
      const del = document.createElement('span');
      del.className = 'board-delete-btn';
      del.innerHTML = '&times;';
      del.title = 'Delete board';
      del.style.cssText = 'margin-left:8px;color:var(--text-dimmed);cursor:pointer;font-size:1rem;line-height:1;padding:0 4px;border-radius:3px;';
      del.onmouseenter = () => { del.style.color = '#2B7DE9'; del.style.background = 'rgba(43,125,233,0.1)'; };
      del.onmouseleave = () => { del.style.color = 'var(--text-dimmed)'; del.style.background = ''; };
      del.onclick = (e) => { e.stopPropagation(); deleteBoard(board.id, board.name); };
      opt.appendChild(del);
    }

    menu.appendChild(opt);
  });

  // Separator
  const sep = document.createElement('div');
  sep.className = 'header-board-sep';
  menu.appendChild(sep);

  // All Boards
  const allIcon = '<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/></svg>';
  const allOpt = document.createElement('div');
  allOpt.className = 'header-board-option' + (activeId === 'all' ? ' active' : '');
  allOpt.style.cssText = 'display:flex;align-items:center;gap:6px';
  allOpt.innerHTML = allIcon + ' All Boards';
  allOpt.onclick = () => {
    closeMenu();
    if (inSettings && typeof window.closeSettings === 'function') window.closeSettings();
    selectBoard('all', 'All Boards');
  };
  menu.appendChild(allOpt);

  // + New board
  const newOpt = document.createElement('div');
  newOpt.className = 'header-board-option new';
  newOpt.textContent = '+ New board';
  newOpt.onclick = () => {
    closeMenu();
    if (inSettings && typeof window.closeSettings === 'function') window.closeSettings();
    setTimeout(() => openNewBoardModal(), inSettings ? 400 : 0);
  };
  menu.appendChild(newOpt);

  // Settings
  const settingsOpt = document.createElement('div');
  settingsOpt.className = 'header-board-option' + (activeId === 'settings' ? ' active' : '');
  settingsOpt.style.cssText = 'display:flex;align-items:center;gap:6px';
  settingsOpt.innerHTML = _gearIcon + ' Settings';
  settingsOpt.onclick = () => { closeMenu(); location.hash = '#settings/boards'; };
  menu.appendChild(settingsOpt);
}

function renderBoardPicker() {
  const menu = document.getElementById('board-menu');
  if (!menu) return;

  _buildPickerMenu(menu, {
    activeId: window.currentBoardId,
    closeMenu: () => { if (typeof window.closeBoardPicker === 'function') window.closeBoardPicker(); },
  });

  // Update label
  const current = window.boardList.find(b => b.id === window.currentBoardId);
  const label = document.getElementById('board-picker-label');
  if (label && current) label.textContent = _cap(current.name);
}

async function deleteBoard(boardId, boardName) {
  const ok = await window.fnConfirm('Delete board "' + boardName + '"? All cards on it will be lost.', 'Delete board', 'Delete');
  if (!ok) return;
  try {
    await window.API.deleteBoard(boardId);
    window.boardList = window.boardList.filter(b => b.id !== boardId);
    // If we deleted the current board, switch to first
    if (window.currentBoardId === boardId && window.boardList.length > 0) {
      selectBoard(window.boardList[0].id, window.boardList[0].name);
    }
    renderBoardPicker();
    window.Toast.success('Board "' + boardName + '" deleted');
  } catch (e) {
    window.Toast.error('Failed to delete board: ' + (e.message || 'unknown'));
  }
}

function selectBoard(boardId, boardName) {
  window._renderAnimateCards = true;
  window.currentBoardId = boardId;
  localStorage.setItem('lwts-board-id', boardId);
  document.getElementById('board-picker-label').textContent = _cap(boardName);

  // Update URL with board ID for persistence across refreshes
  const url = new URL(window.location);
  if (boardId === 'all') {
    url.searchParams.delete('board');
  } else {
    url.searchParams.set('board', boardId);
  }
  window.history.replaceState(null, '', url);

  // Close picker
  if (typeof window.closeBoardPicker === 'function') window.closeBoardPicker();

  if (boardId === 'all') {
    // Load cards from all boards
    window.loadAllBoardCards();
  } else {
    // Disconnect old SSE stream, connect new
    connectBoardStream(boardId);
    // Load presence
    if (typeof window.loadPresence === 'function') window.loadPresence(boardId);
    // Fetch cards for this board
    window.loadBoardCards(boardId);
  }

  // Update picker highlight
  renderBoardPicker();
}

// loadBoardCards is defined in kanban.js — don't redefine it here.
// The selectBoard function above calls it directly.

// ── New Board Modal ──

let _newBoardDefaultColDropdown = null;

function openNewBoardModal() {
  if (typeof window.closeBoardPicker === 'function') window.closeBoardPicker();

  let modal = document.getElementById('new-board-modal');
  if (!modal) {
    modal = document.createElement('div');
    modal.id = 'new-board-modal';
    modal.className = 'lwts-modal-overlay';
    modal.innerHTML = `
      <div class="lwts-modal">
        <div class="lwts-modal-header">
          <h3>New board</h3>
          <button class="lwts-modal-close" onclick="closeNewBoardModal()">&times;</button>
        </div>
        <div class="lwts-modal-body">
          <form onsubmit="return false;">
            <div class="settings-group">
              <div class="settings-group-title">Configuration</div>
              <div class="settings-row">
                <div class="settings-row-label"><div class="settings-row-title">Board name</div><div class="settings-row-desc">Display name for this board</div></div>
                <div class="settings-row-control"><input id="new-board-name" class="settings-input" placeholder="My Board" /></div>
              </div>
              <div class="settings-row">
                <div class="settings-row-label"><div class="settings-row-title">Project key</div><div class="settings-row-desc">Prefix for ticket IDs (e.g. PROJ-101)</div></div>
                <div class="settings-row-control"><input id="new-board-key" class="settings-input" style="width:100px;text-transform:uppercase" placeholder="PROJ" maxlength="5" /></div>
              </div>
              <div class="settings-row">
                <div class="settings-row-label"><div class="settings-row-title">Default column</div><div class="settings-row-desc">New cards land here</div></div>
                <div class="settings-row-control"><div id="new-board-default-col" style="width:150px"></div></div>
              </div>
            </div>
            <div class="settings-group">
              <div class="settings-group-title">Columns</div>
              <div class="settings-row">
                <div class="settings-row-label"><div class="settings-row-title">Backlog</div><div class="settings-row-desc">Starting column</div></div>
                <div class="settings-row-control"><label class="settings-toggle"><input type="checkbox" id="new-board-col-backlog" checked><span class="toggle-track"></span></label></div>
              </div>
              <div class="settings-row">
                <div class="settings-row-label"><div class="settings-row-title">To Do</div></div>
                <div class="settings-row-control"><label class="settings-toggle"><input type="checkbox" id="new-board-col-todo" checked><span class="toggle-track"></span></label></div>
              </div>
              <div class="settings-row">
                <div class="settings-row-label"><div class="settings-row-title">In Progress</div></div>
                <div class="settings-row-control"><label class="settings-toggle"><input type="checkbox" id="new-board-col-inprogress" checked><span class="toggle-track"></span></label></div>
              </div>
              <div class="settings-row">
                <div class="settings-row-label"><div class="settings-row-title">Done</div></div>
                <div class="settings-row-control"><label class="settings-toggle"><input type="checkbox" id="new-board-col-done" checked><span class="toggle-track"></span></label></div>
              </div>
            </div>
            <div class="settings-group">
              <div class="settings-group-title">Webhooks</div>
              <div class="settings-row">
                <div class="settings-row-label"><div class="settings-row-title">On transition</div><div class="settings-row-desc">POST when a card moves columns</div></div>
                <div class="settings-row-control" style="display:flex;gap:8px"><input id="new-board-wh-transition" class="settings-input" placeholder="https://..." /><button class="btn" style="font-size:0.78rem;flex-shrink:0" type="button" onclick="testWebhook(this)">Test</button></div>
              </div>
              <div class="settings-row">
                <div class="settings-row-label"><div class="settings-row-title">On create</div><div class="settings-row-desc">POST when a new card is created</div></div>
                <div class="settings-row-control" style="display:flex;gap:8px"><input id="new-board-wh-create" class="settings-input" placeholder="https://..." /><button class="btn" style="font-size:0.78rem;flex-shrink:0" type="button" onclick="testWebhook(this)">Test</button></div>
              </div>
              <div class="settings-row">
                <div class="settings-row-label"><div class="settings-row-title">On complete</div><div class="settings-row-desc">POST when a card moves to Done</div></div>
                <div class="settings-row-control" style="display:flex;gap:8px"><input id="new-board-wh-complete" class="settings-input" placeholder="https://..." /><button class="btn" style="font-size:0.78rem;flex-shrink:0" type="button" onclick="testWebhook(this)">Test</button></div>
              </div>
            </div>
          </form>
        </div>
        <div class="lwts-modal-footer">
          <div></div>
          <div class="modal-footer-right">
            <button class="lwts-modal-btn-cancel" onclick="closeNewBoardModal()">Cancel</button>
            <button class="lwts-modal-btn-primary" onclick="submitNewBoard()">Create board</button>
          </div>
        </div>
      </div>
    `;
    modal.addEventListener('click', (e) => {
      if (e.target === modal) closeNewBoardModal();
    });
    document.body.appendChild(modal);

    // Init default column dropdown
    _newBoardDefaultColDropdown = new window.FnDropdown(document.getElementById('new-board-default-col'), {
      options: [
        { value: 'backlog', label: 'Backlog' },
        { value: 'todo', label: 'To Do' },
        { value: 'in-progress', label: 'In Progress' },
        { value: 'done', label: 'Done' }
      ],
      value: 'todo',
      compact: true
    });

    // Enter to submit from key field
    document.getElementById('new-board-key').addEventListener('keydown', (e) => {
      if (e.key === 'Enter') submitNewBoard();
    });
  }

  // Reset fields
  document.getElementById('new-board-name').value = '';
  document.getElementById('new-board-key').value = '';
  ['new-board-col-backlog', 'new-board-col-todo', 'new-board-col-inprogress', 'new-board-col-done'].forEach(id => {
    document.getElementById(id).checked = true;
  });
  ['new-board-wh-transition', 'new-board-wh-create', 'new-board-wh-complete'].forEach(id => {
    document.getElementById(id).value = '';
  });
  if (_newBoardDefaultColDropdown) _newBoardDefaultColDropdown.setValue('todo', true);

  // Ensure transition fires: remove active, force layout, then add active
  modal.classList.remove('active');
  void modal.offsetWidth;
  requestAnimationFrame(() => {
    modal.classList.add('active');
    setTimeout(() => document.getElementById('new-board-name').focus(), 100);
  });
}

function closeNewBoardModal() {
  const modal = document.getElementById('new-board-modal');
  if (modal) modal.classList.remove('active');
}

async function submitNewBoard() {
  const name = document.getElementById('new-board-name').value.trim();
  const key = document.getElementById('new-board-key').value.trim().toUpperCase();

  if (!name) { window.Toast.error('Board name is required'); return; }
  if (!key || key.length < 2 || key.length > 5 || !/^[A-Z]+$/.test(key)) {
    window.Toast.error('Project key must be 2-5 uppercase letters');
    return;
  }

  try {
    const board = await window.API.createBoard({ name, project_key: key });
    closeNewBoardModal();
    window.boardList.push(board);
    renderBoardPicker();
    selectBoard(board.id, board.name);
    window.Toast.success('Board "' + name + '" created');
  } catch (e) {
    window.Toast.error('Failed to create board: ' + (e.message || 'unknown'));
  }
}

// ═══════════════════════════════════════════════════════════════
// 2. CARD FILTERING
// ═══════════════════════════════════════════════════════════════

let activeFilters = {
  assignees: [],    // multi-select
  priorities: [],   // multi-select
  tags: [],         // multi-select
  search: '',       // text search
};

let searchDebounceTimer = null;

function initFilterBar() {
  const board = document.getElementById('board');
  if (!board || document.getElementById('filter-bar')) return;

  const bar = document.createElement('div');
  bar.id = 'filter-bar';
  bar.className = 'filter-bar';
  bar.innerHTML = `
    <div class="filter-group">
      <div class="filter-dropdown" id="filter-assignee-wrap">
        <button class="filter-btn" onclick="toggleFilterDropdown('assignee')">
          <svg viewBox="0 0 24 24" width="14" height="14" stroke="currentColor" fill="none" stroke-width="2"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>
          Assignee
          <span class="filter-badge" id="filter-assignee-badge"></span>
        </button>
        <div class="filter-menu hidden" id="filter-assignee-menu"></div>
      </div>
      <div class="filter-dropdown" id="filter-priority-wrap">
        <button class="filter-btn" onclick="toggleFilterDropdown('priority')">
          <svg viewBox="0 0 24 24" width="14" height="14" stroke="currentColor" fill="none" stroke-width="2"><polyline points="18 15 12 9 6 15"/></svg>
          Priority
          <span class="filter-badge" id="filter-priority-badge"></span>
        </button>
        <div class="filter-menu hidden" id="filter-priority-menu"></div>
      </div>
      <div class="filter-dropdown" id="filter-tag-wrap">
        <button class="filter-btn" onclick="toggleFilterDropdown('tag')">
          <svg viewBox="0 0 24 24" width="14" height="14" stroke="currentColor" fill="none" stroke-width="2"><path d="M20.59 13.41l-7.17 7.17a2 2 0 0 1-2.83 0L2 12V2h10l8.59 8.59a2 2 0 0 1 0 2.82z"/><line x1="7" y1="7" x2="7.01" y2="7"/></svg>
          Type
          <span class="filter-badge" id="filter-tag-badge"></span>
        </button>
        <div class="filter-menu hidden" id="filter-tag-menu"></div>
      </div>
      <div class="filter-dropdown" id="filter-density-wrap">
        <button class="filter-btn" onclick="toggleFilterDropdown('density')">
          <svg viewBox="0 0 24 24" width="14" height="14" stroke="currentColor" fill="none" stroke-width="2"><line x1="3" y1="6" x2="21" y2="6"/><line x1="3" y1="12" x2="21" y2="12"/><line x1="3" y1="18" x2="21" y2="18"/></svg>
          <span id="density-toggle-label">Default</span>
        </button>
        <div class="filter-menu hidden" id="filter-density-menu"></div>
      </div>
    </div>
    <div class="filter-status" id="filter-status"></div>
  `;

  board.parentNode.insertBefore(bar, board);

  // Build dropdown contents
  buildFilterCheckboxes('assignee', window.USERS.filter(u => u.value !== 'unassigned').map(u => ({ value: u.value, label: u.label })));
  buildFilterCheckboxes('priority', window.PRIORITIES.map(p => ({ value: p.value, label: p.label })));
  buildFilterCheckboxes('tag', Object.entries(window.TAG_LABELS).map(([k, v]) => ({ value: k, label: v.charAt(0).toUpperCase() + v.slice(1) })));

  // Build density dropdown options
  buildDensityOptions();
  initDensityToggleLabel();

  // Close dropdowns on outside click
  document.addEventListener('click', (e) => {
    if (!e.target.closest('.filter-dropdown')) {
      document.querySelectorAll('.filter-menu').forEach(m => m.classList.add('hidden'));
    }
  });
}

const DENSITY_OPTIONS = [
  { value: 'default', label: 'Default' },
  { value: 'compact', label: 'Compact' },
  { value: 'comfortable', label: 'Comfortable' },
];
const DENSITY_LABELS = { default: 'Default', compact: 'Compact', comfortable: 'Comfortable' };

function buildDensityOptions() {
  const menu = document.getElementById('filter-density-menu');
  if (!menu) return;
  menu.innerHTML = '';

  const current = getCurrentDensity();
  DENSITY_OPTIONS.forEach(opt => {
    const item = document.createElement('div');
    item.className = 'filter-checkbox-item' + (opt.value === current ? ' active' : '');
    item.textContent = opt.label;
    item.style.cursor = 'pointer';
    item.onclick = () => selectDensity(opt.value);
    menu.appendChild(item);
  });
}

function getCurrentDensity() {
  if (document.body.classList.contains('density-compact')) return 'compact';
  if (document.body.classList.contains('density-comfortable')) return 'comfortable';
  return 'default';
}

function selectDensity(value) {
  document.body.classList.remove('density-compact', 'density-comfortable');
  if (value !== 'default') document.body.classList.add('density-' + value);

  // Update label
  const label = document.getElementById('density-toggle-label');
  if (label) label.textContent = DENSITY_LABELS[value];

  // Persist to API (single source of truth) + localStorage cache for anti-FOUC
  if (typeof window.API !== 'undefined') {
    window.API.putSettings('appearance', { density: value }).catch(() => {});
  }
  const cached = JSON.parse(localStorage.getItem('lwts-appearance') || '{}');
  cached.density = value;
  localStorage.setItem('lwts-appearance', JSON.stringify(cached));

  // Sync settings dropdown if open
  if (typeof window._settingsDropdowns !== 'undefined' && window._settingsDropdowns.density) {
    window._settingsDropdowns.density.setValue(value, true);
  }

  // Rebuild to update active state, then close
  buildDensityOptions();
  document.getElementById('filter-density-menu').classList.add('hidden');
}

function initDensityToggleLabel() {
  const label = document.getElementById('density-toggle-label');
  if (label) label.textContent = DENSITY_LABELS[getCurrentDensity()];
}

function buildFilterCheckboxes(type, options) {
  const menu = document.getElementById(`filter-${type}-menu`);
  if (!menu) return;
  menu.innerHTML = '';

  // Add search input for assignee filter
  if (type === 'assignee') {
    const searchWrap = document.createElement('div');
    searchWrap.className = 'filter-menu-search';
    searchWrap.innerHTML = `
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="filter-menu-search-icon"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
      <input type="text" class="filter-menu-search-input" placeholder="Search people..." />
    `;
    menu.appendChild(searchWrap);

    const input = searchWrap.querySelector('input');
    input.addEventListener('keyup', () => {
      const q = input.value.toLowerCase();
      menu.querySelectorAll('.filter-checkbox-item').forEach(item => {
        const name = item.dataset.label || '';
        item.style.display = name.toLowerCase().includes(q) ? '' : 'none';
      });
    });
    // Prevent dropdown from closing when clicking/typing in search
    input.addEventListener('click', e => e.stopPropagation());
  }

  const itemsWrap = document.createElement('div');
  itemsWrap.className = 'filter-menu-items';
  menu.appendChild(itemsWrap);

  options.forEach(opt => {
    const label = document.createElement('label');
    label.className = 'filter-checkbox-item';
    label.dataset.label = opt.label || '';

    let avatarHtml = '';
    if (type === 'assignee' && opt.initials !== undefined) {
      const color = opt.avatar_color || '#82B1FF';
      const bg = opt.initials ? `${color}25` : 'var(--surface-active)';
      const fg = opt.initials ? color : 'var(--text-dimmed)';
      const border = opt.initials ? `${color}40` : 'var(--border-light)';
      const content = opt.avatar_url
        ? `<img src="${window.esc(opt.avatar_url)}" alt="${window.esc(opt.initials)}">`
        : window.esc(opt.initials || '?');
      avatarHtml = `<span class="card-bubble card-avatar filter-avatar" style="background:${bg};color:${fg};border-color:${border}">${content}</span>`;
    }

    label.innerHTML = `
      <input type="checkbox" value="${window.esc(opt.value)}" onchange="onFilterCheckbox('${type}', this)">
      ${avatarHtml}
      <span>${window.esc(opt.label)}</span>
    `;
    itemsWrap.appendChild(label);
  });
}

function toggleFilterDropdown(type) {
  const menu = document.getElementById(`filter-${type}-menu`);
  if (!menu) return;
  const wasHidden = menu.classList.contains('hidden');

  // Close all menus
  document.querySelectorAll('.filter-menu').forEach(m => m.classList.add('hidden'));

  if (wasHidden) menu.classList.remove('hidden');
}

function onFilterCheckbox(type, checkbox) {
  const key = type === 'assignee' ? 'assignees' : type === 'priority' ? 'priorities' : 'tags';
  if (checkbox.checked) {
    if (!activeFilters[key].includes(checkbox.value)) {
      activeFilters[key].push(checkbox.value);
    }
  } else {
    activeFilters[key] = activeFilters[key].filter(v => v !== checkbox.value);
  }
  window.applyFilters();
}

function onFilterSearch(value) {
  clearTimeout(searchDebounceTimer);
  searchDebounceTimer = setTimeout(() => {
    activeFilters.search = value.trim().toLowerCase();
    window.applyFilters();
  }, 200);
}

function _cardMatchesFilters(card) {
  if (activeFilters.assignees.length > 0 && !activeFilters.assignees.includes(card.assignee)) return false;
  if (activeFilters.priorities.length > 0 && !activeFilters.priorities.includes(card.priority)) return false;
  if (activeFilters.tags.length > 0 && !activeFilters.tags.includes(card.tag)) return false;
  if (activeFilters.search) {
    const q = activeFilters.search;
    const matchTitle = card.title.toLowerCase().includes(q);
    const matchKey = (card.key || '').toLowerCase().includes(q);
    const matchDesc = (card.description || '').toLowerCase().includes(q);
    let matchEpic = false;
    if (card.epic_id) {
      for (const col of window.COLUMNS) {
        const epic = (window.state[col.id] || []).find(c => c.id === card.epic_id);
        if (epic && (epic.title.toLowerCase().includes(q) || (epic.key || '').toLowerCase().includes(q))) {
          matchEpic = true; break;
        }
      }
    }
    if (card.tag === 'epic' && matchTitle) matchEpic = true;
    if (!matchTitle && !matchKey && !matchDesc && !matchEpic) return false;
  }
  return true;
}

function applyFilters() {
  const hasFilters = activeFilters.assignees.length > 0 || activeFilters.priorities.length > 0 ||
                     activeFilters.tags.length > 0 || activeFilters.search.length > 0;

  // Filter all cards across all column-body elements (works in both standard and epic mode)
  document.querySelectorAll('.column-body .card').forEach(cardEl => {
    const cardId = cardEl.dataset.id;
    const colId = cardEl.dataset.col;
    const cards = window.state[colId] || [];
    const card = cards.find(c => c.id === cardId);

    if (!card) return;

    const visible = !hasFilters || _cardMatchesFilters(card);

    if (visible) {
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

  // Update column counts (works for both .column and .board-column-headers)
  document.querySelectorAll('.column-header').forEach(hdrEl => {
    const countEl = hdrEl.querySelector('.column-count');
    if (!countEl) return;
    // Find the column id from the dot class or sibling body
    const dot = hdrEl.querySelector('.column-dot');
    if (!dot) return;
    const colId = [...dot.classList].find(c => c !== 'column-dot') || '';
    const total = (window.state[colId] || []).length;
    if (hasFilters) {
      const visible = (window.state[colId] || []).filter(c => _cardMatchesFilters(c)).length;
      countEl.textContent = `${visible}/${total}`;
    } else {
      countEl.textContent = total;
    }
  });

  // Hide any epic lane (including ungrouped) that has zero visible cards
  document.querySelectorAll('.epic-lane').forEach(lane => {
    const visibleCards = lane.querySelectorAll('.card:not(.search-hidden)').length;
    lane.style.display = (hasFilters && visibleCards === 0) ? 'none' : '';
  });

  // Filter list view rows
  document.querySelectorAll('#list-view .list-row').forEach(rowEl => {
    const cardId = rowEl.dataset.id;
    const colId = rowEl.dataset.col;
    const cards = window.state[colId] || [];
    const card = cards.find(c => c.id === cardId);
    if (!card) { rowEl.style.display = ''; return; }

    let visible = true;
    if (activeFilters.assignees.length > 0 && !activeFilters.assignees.includes(card.assignee)) visible = false;
    if (activeFilters.priorities.length > 0 && !activeFilters.priorities.includes(card.priority)) visible = false;
    if (activeFilters.tags.length > 0 && !activeFilters.tags.includes(card.tag)) visible = false;
    if (activeFilters.search) {
      const q = activeFilters.search;
      const match = card.title.toLowerCase().includes(q) ||
        (card.key || '').toLowerCase().includes(q) ||
        (card.description || '').toLowerCase().includes(q);
      if (!match) visible = false;
    }

    rowEl.style.display = visible ? '' : 'none';
  });

  // Update badges
  updateFilterBadge('assignee', activeFilters.assignees.length);
  updateFilterBadge('priority', activeFilters.priorities.length);
  updateFilterBadge('tag', activeFilters.tags.length);

  // Update status text
  const statusEl = document.getElementById('filter-status');
  if (statusEl) {
    const count = activeFilters.assignees.length + activeFilters.priorities.length +
                  activeFilters.tags.length + (activeFilters.search ? 1 : 0);
    if (count > 0) {
      statusEl.innerHTML = `<span class="filter-count">${count} filter${count > 1 ? 's' : ''}</span>` +
        `<button class="filter-clear" onclick="clearAllFilters()">Clear all</button>`;
    } else {
      statusEl.innerHTML = '';
    }
  }
}

function updateFilterBadge(type, count) {
  const badge = document.getElementById(`filter-${type}-badge`);
  if (badge) {
    badge.textContent = count > 0 ? count : '';
    badge.style.display = count > 0 ? 'inline-flex' : 'none';
  }
}

function clearAllFilters() {
  activeFilters = { assignees: [], priorities: [], tags: [], search: '' };
  const filterSearchEl = document.getElementById('filter-search');
  if (filterSearchEl) filterSearchEl.value = '';
  document.querySelectorAll('.filter-menu input[type="checkbox"]').forEach(cb => { cb.checked = false; });
  window.applyFilters();
}


// ═══════════════════════════════════════════════════════════════
// 3. DUE DATES
// ═══════════════════════════════════════════════════════════════

function getDueDateInfo(dateStr) {
  if (!dateStr) return null;

  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const due = new Date(dateStr + 'T00:00:00');
  due.setHours(0, 0, 0, 0);

  const diffDays = Math.round((due - today) / (1000 * 60 * 60 * 24));

  let label, cssClass;

  if (diffDays < -1) {
    label = window.formatDate(dateStr);
    cssClass = 'due-overdue';
  } else if (diffDays === -1) {
    label = 'Yesterday';
    cssClass = 'due-overdue';
  } else if (diffDays === 0) {
    label = 'Today';
    cssClass = 'due-today';
  } else if (diffDays === 1) {
    label = 'Tomorrow';
    cssClass = 'due-soon';
  } else if (diffDays <= 7) {
    label = window.formatDate(dateStr);
    cssClass = 'due-week';
  } else {
    label = window.formatDate(dateStr);
    cssClass = 'due-future';
  }

  return { label, cssClass, diffDays };
}

// Inject due date chip into card element (called after card render)
function injectDueDateChips() {
  document.querySelectorAll('.card').forEach(cardEl => {
    const cardId = cardEl.dataset.id;
    const colId = cardEl.dataset.col;
    const cards = window.state[colId] || [];
    const card = cards.find(c => c.id === cardId);

    if (!card) return;

    // Use due_date field, fall back to date for seed data
    const dateStr = card.due_date || card.date;
    if (!dateStr) return;

    const info = getDueDateInfo(dateStr);
    if (!info) return;

    // Don't duplicate
    if (cardEl.querySelector('.card-due-chip')) return;

    const footerRight = cardEl.querySelector('.card-footer-right') || cardEl.querySelector('.card-meta-right');
    if (!footerRight) return;

    const chip = document.createElement('span');
    chip.className = 'card-due-chip ' + info.cssClass;
    chip.innerHTML = `<svg viewBox="0 0 24 24" width="11" height="11" stroke="currentColor" fill="none" stroke-width="2"><rect x="3" y="4" width="18" height="18" rx="2" ry="2"/><line x1="16" y1="2" x2="16" y2="6"/><line x1="8" y1="2" x2="8" y2="6"/><line x1="3" y1="10" x2="21" y2="10"/></svg> ${window.esc(info.label)}`;
    footerRight.insertBefore(chip, footerRight.firstChild);
  });
}

// Due date field in detail sidebar
function initDueDateField() {
  const table = document.querySelector('.detail-fields-table');
  if (!table || document.getElementById('detail-duedate-row')) return;

  const row = document.createElement('tr');
  row.id = 'detail-duedate-row';
  row.className = 'detail-field-row';
  row.innerHTML = `
    <td class="detail-field-label">Due date</td>
    <td class="detail-field-cell">
      <span class="detail-field-text" id="detail-duedate-text" onclick="openDueDatePicker()"></span>
    </td>
  `;
  table.appendChild(row);
}

// ── Custom Calendar Picker ──

let _calendarMonth = null;
let _calendarSelected = null;

function openDueDatePicker() {
  closeDueDatePicker();

  const textEl = document.getElementById('detail-duedate-text');
  if (!textEl) return;

  const dateStr = window.detailCard ? (window.detailCard.due_date || window.detailCard.date) : (window._pendingDueDate || null);
  if (dateStr) {
    const [y, m] = dateStr.split('-').map(Number);
    _calendarMonth = { year: y, month: m - 1 };
    _calendarSelected = dateStr;
  } else {
    const now = new Date();
    _calendarMonth = { year: now.getFullYear(), month: now.getMonth() };
    _calendarSelected = null;
  }

  const picker = document.createElement('div');
  picker.id = 'due-date-picker';
  picker.className = 'due-date-picker';

  const rect = textEl.getBoundingClientRect();
  picker.style.top = (rect.bottom + 6) + 'px';
  picker.style.left = rect.left + 'px';

  _renderCalendar(picker);
  document.body.appendChild(picker);

  requestAnimationFrame(() => picker.classList.add('visible'));

  setTimeout(() => {
    document.addEventListener('mousedown', _calendarOutsideClick, true);
    document.addEventListener('keydown', _calendarKeydown, true);
  }, 0);
}

function closeDueDatePicker() {
  const picker = document.getElementById('due-date-picker');
  if (picker) {
    picker.classList.remove('visible');
    picker.addEventListener('transitionend', () => picker.remove(), { once: true });
    setTimeout(() => { if (picker.parentNode) picker.remove(); }, 300);
  }
  document.removeEventListener('mousedown', _calendarOutsideClick, true);
  document.removeEventListener('keydown', _calendarKeydown, true);
}

function _calendarOutsideClick(e) {
  const picker = document.getElementById('due-date-picker');
  if (picker && !picker.contains(e.target)) closeDueDatePicker();
}

function _calendarKeydown(e) {
  if (e.key === 'Escape') closeDueDatePicker();
}

function _renderCalendar(picker) {
  const { year, month } = _calendarMonth;
  const monthNames = ['January','February','March','April','May','June','July','August','September','October','November','December'];
  const dayNames = ['Su','Mo','Tu','We','Th','Fr','Sa'];

  const today = new Date();
  today.setHours(0,0,0,0);
  const todayStr = today.getFullYear() + '-' + String(today.getMonth()+1).padStart(2,'0') + '-' + String(today.getDate()).padStart(2,'0');

  const firstDay = new Date(year, month, 1).getDay();
  const daysInMonth = new Date(year, month + 1, 0).getDate();
  const prevMonthDays = new Date(year, month, 0).getDate();

  let html = `
    <div class="cal-header">
      <button class="cal-nav" onclick="_calPrev()">&lsaquo;</button>
      <span class="cal-title">${monthNames[month]} ${year}</span>
      <button class="cal-nav" onclick="_calNext()">&rsaquo;</button>
    </div>
    <div class="cal-days">
      ${dayNames.map(d => `<span class="cal-day-label">${d}</span>`).join('')}
  `;

  // Always render 42 cells (6 rows) for consistent size
  for (let i = 0; i < 42; i++) {
    const dayOffset = i - firstDay;
    const d = dayOffset + 1;
    if (d < 1) {
      html += `<span class="cal-day outside">${prevMonthDays + d}</span>`;
    } else if (d > daysInMonth) {
      html += `<span class="cal-day outside">${d - daysInMonth}</span>`;
    } else {
      const dateStr = year + '-' + String(month+1).padStart(2,'0') + '-' + String(d).padStart(2,'0');
      const classes = ['cal-day'];
      if (dateStr === todayStr) classes.push('today');
      if (dateStr === _calendarSelected) classes.push('selected');
      html += `<span class="${classes.join(' ')}" onclick="_calSelect('${dateStr}')">${d}</span>`;
    }
  }

  html += `</div>
    <div class="cal-footer">
      <button class="cal-footer-btn" onclick="_calSelect('${todayStr}')">Today</button>
      <button class="cal-footer-btn cal-clear" onclick="_calSelect('')">Clear</button>
    </div>
  `;

  picker.innerHTML = html;
}

function _calPrev() {
  _calendarMonth.month--;
  if (_calendarMonth.month < 0) { _calendarMonth.month = 11; _calendarMonth.year--; }
  const picker = document.getElementById('due-date-picker');
  if (picker) _renderCalendar(picker);
}

function _calNext() {
  _calendarMonth.month++;
  if (_calendarMonth.month > 11) { _calendarMonth.month = 0; _calendarMonth.year++; }
  const picker = document.getElementById('due-date-picker');
  if (picker) _renderCalendar(picker);
}

function _calSelect(dateStr) {
  onDueDateChange(dateStr);
  closeDueDatePicker();
}

function onDueDateChange(value) {
  if (window.detailCard) {
    window.detailCard.due_date = value || null;
    window.detailCard.date = value || window.detailCard.date;
    refreshDueDateText();
    window.save();
  } else {
    // Create mode
    window._pendingDueDate = value || null;
    refreshDueDateText();
  }
}

function refreshDueDateText() {
  const textEl = document.getElementById('detail-duedate-text');
  if (!textEl) return;

  const dateStr = window.detailCard
    ? (window.detailCard.due_date || window.detailCard.date)
    : (window._pendingDueDate || null);
  if (dateStr) {
    const info = getDueDateInfo(dateStr);
    textEl.innerHTML = info ? `<span class="detail-due-chip ${info.cssClass}">${window.esc(info.label)}</span>` : window.esc(dateStr);
  } else {
    textEl.innerHTML = '<span style="color:var(--text-dimmed)">None</span>';
  }
}


// ═══════════════════════════════════════════════════════════════
// SSE STATE UPDATE HELPERS (for presence.js wiring)
// ═══════════════════════════════════════════════════════════════

function updateCardInState(data) {
  for (const col of window.COLUMNS) {
    const cards = window.state[col.id] || [];
    const idx = cards.findIndex(c => c.id === data.id);
    if (idx !== -1) {
      // Merge updated fields
      Object.assign(cards[idx], {
        title: data.title ?? cards[idx].title,
        description: data.description ?? cards[idx].description,
        tag: data.tag ?? cards[idx].tag,
        priority: data.priority ?? cards[idx].priority,
        version: data.version ?? cards[idx].version,
        assignee: data.assignee_id ?? cards[idx].assignee,
        points: data.points ?? cards[idx].points,
        date: data.due_date ?? cards[idx].date,
        due_date: data.due_date ?? cards[idx].due_date,
        epic_id: data.epic_id ?? cards[idx].epic_id,
        reporter: data.reporter_id ?? cards[idx].reporter,
        key: data.key ?? cards[idx].key,
        related_card_ids: data.related_card_ids ?? cards[idx].related_card_ids,
        blocked_card_ids: data.blocked_card_ids ?? cards[idx].blocked_card_ids,
      });
      window.save();
      window.render();
      // If the detail modal is open for this card, refresh all detail views
      if (window.detailCard && window.detailCard.id === data.id) {
        // Update the detailCard object with new data
        Object.assign(window.detailCard, {
          title: data.title ?? window.detailCard.title,
          description: data.description ?? window.detailCard.description,
          tag: data.tag ?? window.detailCard.tag,
          priority: data.priority ?? window.detailCard.priority,
          version: data.version ?? window.detailCard.version,
          assignee_id: data.assignee_id ?? window.detailCard.assignee_id,
          points: data.points ?? window.detailCard.points,
          due_date: data.due_date ?? window.detailCard.due_date,
          epic_id: data.epic_id ?? window.detailCard.epic_id,
          reporter_id: data.reporter_id ?? window.detailCard.reporter_id,
          key: data.key ?? window.detailCard.key,
        });
        // Refresh title
        var titleEl = document.getElementById('detail-title');
        if (titleEl && data.title != null) titleEl.value = data.title;
        // Refresh sidebar fields
        if (typeof window.refreshSidebarTexts === 'function') window.refreshSidebarTexts();
        if (typeof window.renderDescriptionView === 'function') window.renderDescriptionView();
        if (typeof window.refreshGithubLinks === 'function') window.refreshGithubLinks();
      }
      injectDueDateChips();
      window.applyFilters();
      // Highlight the updated card
      const el = document.querySelector('.card[data-id="' + data.id + '"]')
        || document.querySelector('.list-row[data-id="' + data.id + '"]');
      if (el) {
        el.classList.remove('sse-highlight');
        void el.offsetWidth; // force reflow to restart animation
        el.classList.add('sse-highlight');
        el.addEventListener('animationend', () => el.classList.remove('sse-highlight'), { once: true });
      }
      return;
    }
  }
}

function addCardToState(data) {
  const colId = data.column_id || 'backlog';
  if (!window.state[colId]) window.state[colId] = [];
  // Don't duplicate
  if (window.state[colId].some(c => c.id === data.id)) return;
  window.state[colId].push({
    id: data.id, key: data.key, title: data.title,
    description: data.description || '', tag: data.tag || 'blue',
    priority: data.priority || 'medium', assignee: data.assignee_id || 'unassigned',
    reporter: data.reporter_id || 'you', points: data.points || 0,
    date: data.due_date || '', due_date: data.due_date || null,
    epic_id: data.epic_id || null,
    version: data.version || 1, comments: [],
  });
  window.save();
  window.render();
  injectDueDateChips();
  window.applyFilters();
  // Animate the newly added card
  const el = document.querySelector('.card[data-id="' + data.id + '"]')
    || document.querySelector('.list-row[data-id="' + data.id + '"]');
  if (el) {
    el.classList.add('sse-entering');
    el.addEventListener('animationend', () => el.classList.remove('sse-entering'), { once: true });
  }
}

function moveCardInState(data) {
  // Remove from old column
  for (const col of window.COLUMNS) {
    const idx = (window.state[col.id] || []).findIndex(c => c.id === data.id);
    if (idx !== -1) {
      window.state[col.id].splice(idx, 1);
      break;
    }
  }
  // Add to new column
  const toCol = data.to_column || data.column_id || 'backlog';
  if (!window.state[toCol]) window.state[toCol] = [];
  const pos = data.position ?? window.state[toCol].length;
  window.state[toCol].splice(pos, 0, {
    id: data.id, key: data.key, title: data.title,
    tag: data.tag || 'blue', priority: data.priority || 'medium',
    version: data.version || 1, assignee: data.assignee_id || 'unassigned',
    reporter: data.reporter_id || 'you', points: data.points || 0,
    date: data.due_date || '', due_date: data.due_date || null,
    epic_id: data.epic_id || null,
    description: data.description || '', comments: [],
  });
  window.save();
  window.render();
  // If detail modal is open for this card, refresh sidebar (status changed)
  if (window.detailCard && window.detailCard.id === data.id) {
    window.detailCard.column_id = toCol;
    if (typeof window.refreshSidebarTexts === 'function') window.refreshSidebarTexts();
  }
  injectDueDateChips();
  window.applyFilters();
  // Highlight the moved card in its new position
  const el = document.querySelector('.card[data-id="' + data.id + '"]')
    || document.querySelector('.list-row[data-id="' + data.id + '"]');
  if (el) {
    el.classList.add('sse-highlight');
    el.addEventListener('animationend', () => el.classList.remove('sse-highlight'), { once: true });
  }
}

function removeCardFromState(data) {
  // Animate exit before removing from state
  const el = document.querySelector('.card[data-id="' + data.id + '"]')
    || document.querySelector('.list-row[data-id="' + data.id + '"]');
  if (el) {
    el.classList.add('sse-exiting');
    el.addEventListener('animationend', () => {
      for (const col of window.COLUMNS) {
        const idx = (window.state[col.id] || []).findIndex(c => c.id === data.id);
        if (idx !== -1) {
          window.state[col.id].splice(idx, 1);
          window.save();
          window.render();
          injectDueDateChips();
          window.applyFilters();
          return;
        }
      }
    }, { once: true });
    return;
  }
  // Fallback: no DOM element found, just remove from state directly
  for (const col of window.COLUMNS) {
    const idx = (window.state[col.id] || []).findIndex(c => c.id === data.id);
    if (idx !== -1) {
      window.state[col.id].splice(idx, 1);
      window.save();
      window.render();
      injectDueDateChips();
      window.applyFilters();
      return;
    }
  }
}


// ═══════════════════════════════════════════════════════════════
// INIT — Hook into existing lifecycle
// ═══════════════════════════════════════════════════════════════

// Patch render() to inject due date chips and filter bar after each render
const _origRender = typeof window.render === 'function' ? window.render : null;
if (_origRender) {
  // Override global render to also inject our features
  window._baseRender = _origRender;
  window.render = function() {
    _origRender();
    initFilterBar();
    injectDueDateChips();
    // Re-apply filters if any are active
    if (activeFilters.assignees.length || activeFilters.priorities.length ||
        activeFilters.tags.length || activeFilters.search) {
      window.applyFilters();
    }
  };
}

// Patch openDetail to also show due date
const _origOpenDetail = typeof window.openDetail === 'function' ? window.openDetail : null;
if (_origOpenDetail) {
  window.openDetail = function(cardId) {
    _origOpenDetail(cardId);
    initDueDateField();
    refreshDueDateText();
  };
}


// On page load
document.addEventListener('DOMContentLoaded', () => {
  initFilterBar();
  initDueDateField();
  injectDueDateChips();

  // Try to load boards from API (falls back gracefully)
  loadBoardList();
});

// ═══════════════════════════════════════════════════════════════
// EXPORT ALL TOP-LEVEL SYMBOLS TO WINDOW
// ═══════════════════════════════════════════════════════════════

window.currentBoardStream = currentBoardStream;
window.loadBoardList = loadBoardList;
window.connectBoardStream = connectBoardStream;
window.renderBoardPicker = renderBoardPicker;
window._buildPickerMenu = _buildPickerMenu;
window.deleteBoard = deleteBoard;
window.selectBoard = selectBoard;
window.openNewBoardModal = openNewBoardModal;
window.closeNewBoardModal = closeNewBoardModal;
window.submitNewBoard = submitNewBoard;
window.activeFilters = activeFilters;
window.initFilterBar = initFilterBar;
window.buildDensityOptions = buildDensityOptions;
window.getCurrentDensity = getCurrentDensity;
window.selectDensity = selectDensity;
window.initDensityToggleLabel = initDensityToggleLabel;
window.buildFilterCheckboxes = buildFilterCheckboxes;
window.toggleFilterDropdown = toggleFilterDropdown;
window.onFilterCheckbox = onFilterCheckbox;
window.onFilterSearch = onFilterSearch;
window.applyFilters = applyFilters;
window.updateFilterBadge = updateFilterBadge;
window.clearAllFilters = clearAllFilters;
window.getDueDateInfo = getDueDateInfo;
window.injectDueDateChips = injectDueDateChips;
window.initDueDateField = initDueDateField;
window.openDueDatePicker = openDueDatePicker;
window.closeDueDatePicker = closeDueDatePicker;
window._calPrev = _calPrev;
window._calNext = _calNext;
window._calSelect = _calSelect;
window._renderCalendar = _renderCalendar;
window.onDueDateChange = onDueDateChange;
window.refreshDueDateText = refreshDueDateText;
window.updateCardInState = updateCardInState;
window.addCardToState = addCardToState;
window.moveCardInState = moveCardInState;
window.removeCardFromState = removeCardFromState;
window.DENSITY_OPTIONS = DENSITY_OPTIONS;
window.DENSITY_LABELS = DENSITY_LABELS;
window._newBoardDefaultColDropdown = _newBoardDefaultColDropdown;
window.searchDebounceTimer = searchDebounceTimer;
