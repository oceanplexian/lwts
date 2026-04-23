/* fn2 Kanban — List View (Vite ES module) */

let currentView = localStorage.getItem('lwts-view') || 'board';
let listSortCol = 'updated';
let listSortDir = 'desc';

// ── Infinite scroll state ──
const LIST_PAGE_SIZE = 50;
let _listAllCards = [];    // sorted, full dataset
let _listRendered = 0;     // how many rows rendered so far
let _listTbody = null;     // reference to current tbody
let _listScrollHandler = null;

// ── Drag state (list) ──
let listDragCard = null;
let listDragSourceCol = null;
let listDragEl = null;
let listDidDrag = false;

// Build status maps dynamically from COLUMNS (falls back to defaults for legacy boards)
function _buildStatusOrder() {
  const cols = (typeof window.COLUMNS !== 'undefined' && window.COLUMNS.length) ? window.COLUMNS : [];
  const order = {};
  cols.forEach((c, i) => { order[c.id] = i; });
  order.cleared = cols.length;
  return order;
}
function _buildStatusColors() {
  const cols = (typeof window.COLUMNS !== 'undefined' && window.COLUMNS.length) ? window.COLUMNS : [];
  const palette = ['#8c8c8c','#579DFF','#fb8c00','#4ade80','#f44336','#9f8fef','#6cc3e0','#f5cd47'];
  const colors = {};
  cols.forEach((c, i) => { colors[c.id] = c.color || palette[i % palette.length]; });
  colors.cleared = 'var(--text-dimmed)';
  return colors;
}
function _buildStatusLabels() {
  const cols = (typeof window.COLUMNS !== 'undefined' && window.COLUMNS.length) ? window.COLUMNS : [];
  const labels = {};
  cols.forEach(c => { labels[c.id] = c.label; });
  labels.cleared = 'Cleared';
  return labels;
}
// These are rebuilt each render to reflect current board columns
let STATUS_ORDER = _buildStatusOrder();
const PRIORITY_ORDER = { highest: 0, high: 1, medium: 2, low: 3, lowest: 4 };
let STATUS_COLORS = _buildStatusColors();
let STATUS_LABELS = _buildStatusLabels();

// ═══════════════════════════════════════════════════════════════
// VIEW SWITCHING
// ═══════════════════════════════════════════════════════════════

function switchView(view) {
  currentView = view;
  window.currentView = view;
  localStorage.setItem('lwts-view', view);

  const board = document.getElementById('board');
  const listView = document.getElementById('list-view');

  // Update switcher button active states immediately
  document.querySelectorAll('.view-switcher-btn').forEach(btn => {
    btn.classList.toggle('active', btn.dataset.view === view);
  });

  if (view === 'board') {
    listView.style.display = 'none';
    // "All Boards" is list-only. When returning to the board, restore the last concrete board.
    if (window.currentBoardId === 'all' && window.boardList && window.boardList.length > 0) {
      const fallbackBoardId = typeof window.getDefaultBoardId === 'function'
        ? window.getDefaultBoardId()
        : window.boardList[0].id;
      const fallbackBoard = window.boardList.find(b => b.id === fallbackBoardId);
      // Clear old content first to avoid flash of stale board
      board.innerHTML = '';
      board.style.display = '';
      window.selectBoard(fallbackBoardId, fallbackBoard ? fallbackBoard.name : fallbackBoardId);
      return;
    }
    board.style.display = '';
    window._renderAnimateCards = true;
    window.render();
  } else {
    board.style.display = 'none';
    listView.style.display = '';
    renderListView();
  }
}

// ═══════════════════════════════════════════════════════════════
// RENDER LIST VIEW
// ═══════════════════════════════════════════════════════════════

function collectAllCards() {
  const cards = [];
  window.COLUMNS.forEach(col => {
    (window.state[col.id] || []).forEach(card => {
      cards.push(Object.assign({}, card, { _column: col.id, _cleared: false }));
    });
  });
  (window.state.cleared || []).forEach(card => {
    cards.push(Object.assign({}, card, { _column: 'cleared', _cleared: true }));
  });
  return cards;
}

function sortCards(cards) {
  const dir = listSortDir === 'asc' ? 1 : -1;
  const col = listSortCol;

  cards.sort((a, b) => {
    let cmp = 0;
    switch (col) {
      case 'key': {
        const pa = (a.key || '').split('-');
        const pb = (b.key || '').split('-');
        cmp = (pa[0] || '').localeCompare(pb[0] || '');
        if (cmp === 0) cmp = (parseInt(pa[1]) || 0) - (parseInt(pb[1]) || 0);
        break;
      }
      case 'title':
        cmp = (a.title || '').localeCompare(b.title || '');
        break;
      case 'status':
        cmp = (STATUS_ORDER[a._column] ?? 99) - (STATUS_ORDER[b._column] ?? 99);
        break;
      case 'tag':
        cmp = (window.TAG_LABELS[a.tag] || a.tag || '').localeCompare(window.TAG_LABELS[b.tag] || b.tag || '');
        break;
      case 'priority':
        cmp = (PRIORITY_ORDER[a.priority] ?? 99) - (PRIORITY_ORDER[b.priority] ?? 99);
        break;
      case 'assignee': {
        const ua = window.getUserById(a.assignee);
        const ub = window.getUserById(b.assignee);
        cmp = (ua.label || '').localeCompare(ub.label || '');
        break;
      }
      case 'points':
        cmp = (a.points || 0) - (b.points || 0);
        break;
      case 'date': {
        const da = a.date ? new Date(a.date).getTime() : 0;
        const db = b.date ? new Date(b.date).getTime() : 0;
        cmp = da - db;
        break;
      }
      case 'created': {
        const ca = a.created_at ? new Date(a.created_at).getTime() : 0;
        const cb = b.created_at ? new Date(b.created_at).getTime() : 0;
        cmp = ca - cb;
        break;
      }
      case 'updated': {
        const ua = a.updated_at ? new Date(a.updated_at).getTime() : 0;
        const ub = b.updated_at ? new Date(b.updated_at).getTime() : 0;
        cmp = ua - ub;
        break;
      }
    }
    return cmp * dir;
  });
  return cards;
}

function createListRow(card, animIdx) {
  const row = document.createElement('tr');
  row.className = 'list-row';
  row.dataset.id = card.id;
  row.dataset.col = card._column;
  if (card._cleared) row.classList.add('cleared');
  row.draggable = true;

  // Key
  const tdKey = document.createElement('td');
  tdKey.className = 'list-cell list-cell-key';
  tdKey.textContent = card.key || '';
  row.appendChild(tdKey);

  // Title
  const tdTitle = document.createElement('td');
  tdTitle.className = 'list-cell list-cell-title';
  tdTitle.textContent = card.title || '';
  row.appendChild(tdTitle);

  // Status
  const tdStatus = document.createElement('td');
  tdStatus.className = 'list-cell list-cell-status';
  const statusWrap = document.createElement('span');
  statusWrap.className = 'list-status-wrap';
  statusWrap.style.display = 'inline-flex';
  statusWrap.style.alignItems = 'center';
  statusWrap.style.gap = '6px';
  const dot = document.createElement('span');
  dot.className = 'list-status-dot';
  dot.style.background = STATUS_COLORS[card._column] || 'var(--text-dimmed)';
  statusWrap.appendChild(dot);
  statusWrap.appendChild(document.createTextNode(STATUS_LABELS[card._column] || card._column));
  tdStatus.appendChild(statusWrap);
  row.appendChild(tdStatus);

  // Type (tag)
  const tdTag = document.createElement('td');
  tdTag.className = 'list-cell list-cell-tag';
  const tagSpan = document.createElement('span');
  tagSpan.className = 'card-tag tag-' + window.esc(card.tag || 'blue');
  tagSpan.textContent = window.TAG_LABELS[card.tag] || card.tag || '';
  tdTag.appendChild(tagSpan);
  row.appendChild(tdTag);

  // Priority
  const tdPriority = document.createElement('td');
  tdPriority.className = 'list-cell list-cell-priority';
  const priWrap = document.createElement('span');
  priWrap.style.display = 'inline-flex';
  priWrap.style.alignItems = 'center';
  priWrap.style.gap = '4px';
  const priIcon = document.createElement('span');
  priIcon.className = 'card-priority';
  priIcon.innerHTML = window.PRIORITY_ICONS[card.priority] || window.PRIORITY_ICONS.medium;
  priWrap.appendChild(priIcon);
  const priLabel = window.PRIORITIES.find(p => p.value === card.priority);
  priWrap.appendChild(document.createTextNode(priLabel ? priLabel.label : card.priority || ''));
  tdPriority.appendChild(priWrap);
  row.appendChild(tdPriority);

  // Assignee
  const tdAssignee = document.createElement('td');
  tdAssignee.className = 'list-cell list-cell-assignee';
  const user = window.getUserById(card.assignee);
  if (user.value !== 'unassigned') {
    const assigneeWrap = document.createElement('span');
    assigneeWrap.style.display = 'inline-flex';
    assigneeWrap.style.alignItems = 'center';
    assigneeWrap.style.gap = '6px';
    const avatarColor = window.AVATAR_COLORS[card.assignee] || 'var(--surface-active)';
    const avatarBg = user.initials ? avatarColor + '25' : 'var(--surface-active)';
    const avatarFg = user.initials ? avatarColor : 'var(--text-dimmed)';
    const avatarBorder = user.initials ? avatarColor + '40' : 'var(--border-light)';
    const avatar = document.createElement('span');
    avatar.className = 'card-bubble card-avatar list-avatar';
    avatar.style.background = avatarBg;
    avatar.style.color = avatarFg;
    avatar.style.borderColor = avatarBorder;
    if (user.avatar_url) {
      const img = document.createElement('img');
      img.src = user.avatar_url;
      img.alt = user.initials || '?';
      avatar.appendChild(img);
    } else {
      avatar.textContent = user.initials || '?';
    }
    assigneeWrap.appendChild(avatar);
    assigneeWrap.appendChild(document.createTextNode(user.label));
    tdAssignee.appendChild(assigneeWrap);
  }
  row.appendChild(tdAssignee);

  // Points
  const tdPoints = document.createElement('td');
  tdPoints.className = 'list-cell list-cell-points';
  tdPoints.textContent = card.points || '';
  row.appendChild(tdPoints);

  // Created
  const tdCreated = document.createElement('td');
  tdCreated.className = 'list-cell list-cell-date';
  tdCreated.textContent = card.created_at ? formatRelativeTime(card.created_at) : '';
  if (card.created_at) tdCreated.title = new Date(card.created_at).toLocaleString();
  row.appendChild(tdCreated);

  // Updated
  const tdUpdated = document.createElement('td');
  tdUpdated.className = 'list-cell list-cell-date';
  tdUpdated.textContent = card.updated_at ? formatRelativeTime(card.updated_at) : '';
  if (card.updated_at) tdUpdated.title = new Date(card.updated_at).toLocaleString();
  row.appendChild(tdUpdated);

  // Due Date
  const tdDate = document.createElement('td');
  tdDate.className = 'list-cell list-cell-date';
  tdDate.textContent = card.date || '';
  row.appendChild(tdDate);

  // Click to open detail — works for ALL cards including cleared
  row.addEventListener('click', () => {
    if (listDidDrag) { listDidDrag = false; return; }
    window.openDetail(card.id);
  });

  // Drag events — all cards can be dragged (reopen cleared by dragging to a column)
  row.addEventListener('dragstart', onListDragStart);
  row.addEventListener('dragend', onListDragEnd);

  // Stagger animation (cleared rows animate too, but stay dimmed via .cleared opacity)
  if (animIdx >= 0) {
    row.style.animationDelay = (animIdx * 15) + 'ms';
    row.classList.add('unfurl');
    row.addEventListener('animationend', () => row.classList.remove('unfurl'), { once: true });
  }

  return row;
}

function appendNextPage() {
  if (!_listTbody || _listRendered >= _listAllCards.length) return;

  const end = Math.min(_listRendered + LIST_PAGE_SIZE, _listAllCards.length);
  const frag = document.createDocumentFragment();
  for (let i = _listRendered; i < end; i++) {
    frag.appendChild(createListRow(_listAllCards[i], i < _listRendered + LIST_PAGE_SIZE ? i - _listRendered : -1));
  }
  _listTbody.appendChild(frag);
  _listRendered = end;
}

function onListScroll() {
  const container = document.getElementById('list-view');
  if (!container) return;
  // Load more when within 200px of bottom
  if (container.scrollTop + container.clientHeight >= container.scrollHeight - 200) {
    if (_listRendered < _listAllCards.length) {
      appendNextPage();
    }
  }
}

let _listSurgicalDrop = false; // guard: true during surgical DOM moves

function renderListView() {
  // Rebuild status maps from current board columns
  STATUS_ORDER = _buildStatusOrder();
  STATUS_COLORS = _buildStatusColors();
  STATUS_LABELS = _buildStatusLabels();
  if (_listSurgicalDrop) {
    console.error('BUG: renderListView called during surgical drop — should only mutate DOM directly');
    if (typeof window.__TEST__ !== 'undefined') throw new Error('renderListView during surgical drop');
  }
  const container = document.getElementById('list-view');
  if (!container) return;

  // Remove old scroll handler
  if (_listScrollHandler) {
    container.removeEventListener('scroll', _listScrollHandler);
  }

  _listAllCards = sortCards(collectAllCards());
  _listRendered = 0;

  // Build table shell
  const table = document.createElement('table');
  table.className = 'list-table';

  // Header
  const thead = document.createElement('thead');
  thead.className = 'list-header';
  const headerRow = document.createElement('tr');
  const columns = [
    { key: 'key', label: 'Key' },
    { key: 'title', label: 'Title' },
    { key: 'status', label: 'Status' },
    { key: 'tag', label: 'Type' },
    { key: 'priority', label: 'Priority' },
    { key: 'assignee', label: 'Assignee' },
    { key: 'points', label: 'Pts' },
    { key: 'created', label: 'Created' },
    { key: 'updated', label: 'Updated' },
    { key: 'date', label: 'Due' },
  ];

  columns.forEach(col => {
    const th = document.createElement('th');
    th.textContent = col.label;
    th.dataset.sort = col.key;
    th.className = 'list-header-cell sortable';
    if (listSortCol === col.key) {
      th.classList.add(listSortDir === 'asc' ? 'sort-asc' : 'sort-desc');
    }
    th.addEventListener('click', () => sortListView(col.key));
    headerRow.appendChild(th);
  });

  thead.appendChild(headerRow);
  table.appendChild(thead);

  // Body
  const tbody = document.createElement('tbody');
  _listTbody = tbody;
  table.appendChild(tbody);

  // Drop events on tbody
  tbody.addEventListener('dragover', onListDragOver);
  tbody.addEventListener('drop', onListDrop);

  // Single DOM swap
  container.innerHTML = '';
  container.appendChild(table);

  // Check for epics — group rows if any exist
  const epics = _listAllCards.filter(c => c.tag === 'epic');
  if (epics.length > 0) {
    _renderGroupedList(tbody, epics);
  } else {
    appendNextPage();
  }

  // Attach scroll listener for infinite scroll
  _listScrollHandler = onListScroll;
  container.addEventListener('scroll', _listScrollHandler);
}

function _renderGroupedList(tbody, epics) {
  if (typeof window._getEpicColors === 'function') window.EPIC_LANE_COLORS = window._getEpicColors();
  const COLORS = window.EPIC_LANE_COLORS || [];
  let animIdx = 0;

  epics.forEach((epic, ei) => {
    const color = COLORS[ei % COLORS.length] || { bg: 'transparent', border: 'transparent' };
    const children = _listAllCards.filter(c => c.epic_id === epic.id && c.tag !== 'epic');

    // Epic group header row
    const headerRow = document.createElement('tr');
    headerRow.className = 'list-epic-header';
    const headerCell = document.createElement('td');
    headerCell.colSpan = 10;
    headerCell.innerHTML = `
      <span class="list-epic-key">${window.esc(epic.key)}</span>
      <span class="list-epic-title">${window.esc(epic.title)}</span>
      <span class="list-epic-count">${children.length} card${children.length !== 1 ? 's' : ''}</span>
    `;
    // Apply gradient bg from board colors
    headerCell.style.background = color.bg;
    headerRow.dataset.epic = epic.id;
    headerRow.appendChild(headerCell);
    headerRow.style.cursor = 'pointer';
    headerRow.addEventListener('click', () => window.openDetail(epic.id));
    tbody.appendChild(headerRow);

    // Children — with epic color tint
    children.forEach(card => {
      const row = createListRow(card, animIdx++);
      row.classList.add('list-epic-child');
      row.dataset.epic = epic.id;
      row.style.background = color.bg;
      tbody.appendChild(row);
    });
  });

  // Ungrouped cards (no epic_id, including epic cards themselves)
  const ungrouped = _listAllCards.filter(c => (!c.epic_id && c.tag !== 'epic'));
  if (ungrouped.length > 0 && epics.length > 0) {
    const headerRow = document.createElement('tr');
    headerRow.className = 'list-epic-header list-epic-ungrouped';
    const headerCell = document.createElement('td');
    headerCell.colSpan = 10;
    headerCell.innerHTML = `
      <span class="list-epic-title">Other</span>
      <span class="list-epic-count">${ungrouped.length} card${ungrouped.length !== 1 ? 's' : ''}</span>
    `;
    headerRow.appendChild(headerCell);
    tbody.appendChild(headerRow);
  }

  ungrouped.forEach(card => {
    tbody.appendChild(createListRow(card, animIdx++));
  });

  _listRendered = _listAllCards.length;
}

// ═══════════════════════════════════════════════════════════════
// SORTING
// ═══════════════════════════════════════════════════════════════

function sortListView(column) {
  if (listSortCol === column) {
    listSortDir = listSortDir === 'asc' ? 'desc' : 'asc';
  } else {
    listSortCol = column;
    listSortDir = 'asc';
  }
  renderListView();
}

// ═══════════════════════════════════════════════════════════════
// LIST DRAG & DROP
// ═══════════════════════════════════════════════════════════════

function onListDragStart(e) {
  listDragEl = e.target.closest('.list-row');
  listDragSourceCol = listDragEl.dataset.col;
  const cardId = listDragEl.dataset.id;
  listDragCard = findCardById(cardId) || findClearedCardById(cardId);
  listDragEl.classList.add('dragging');
  e.dataTransfer.effectAllowed = 'move';
  e.dataTransfer.setData('text/plain', cardId);
}

function onListDragOver(e) {
  e.preventDefault();
  e.dataTransfer.dropEffect = 'move';
  listDidDrag = true;
  if (!listDragCard) return;

  // Clear old drop targets
  document.querySelectorAll('.drop-target').forEach(el => el.classList.remove('drop-target'));

  // Find row or epic header under cursor
  const rows = e.currentTarget.querySelectorAll('.list-row:not(.dragging), .list-epic-header');
  let target = null;
  for (const row of rows) {
    const rect = row.getBoundingClientRect();
    if (e.clientY >= rect.top && e.clientY <= rect.bottom) {
      target = row;
      break;
    }
  }
  if (target) target.classList.add('drop-target');
}

function onListDrop(e) {
  e.preventDefault();
  if (!listDragCard) return;

  const tbody = e.currentTarget;

  // Find target row (card rows or epic headers)
  const rows = tbody.querySelectorAll('.list-row:not(.dragging), .list-epic-header');
  let targetRow = null;
  for (const row of rows) {
    const rect = row.getBoundingClientRect();
    if (e.clientY >= rect.top && e.clientY <= rect.bottom) {
      if (row.classList.contains('list-epic-header')) {
        // Find the first card row after this header to use as insert point
        let next = row.nextElementSibling;
        if (next && next.classList.contains('list-row')) {
          targetRow = next;
        } else {
          targetRow = row; // will fall through to append logic
        }
        // Stash the epic id on targetRow for the epic detection below
        if (!targetRow.dataset.epic && row.dataset.epic) {
          targetRow._dropEpicId = row.dataset.epic;
        }
      } else {
        targetRow = row;
      }
      break;
    }
  }

  const origState = JSON.parse(JSON.stringify(window.state));
  const sourceCol = listDragSourceCol;
  const movedCard = listDragCard;
  const movedEl = listDragEl;

  // Determine epic from drop target — check row data, then walk up to find epic header
  let targetEpicId = null;
  if (targetRow) {
    if (targetRow._dropEpicId) {
      targetEpicId = targetRow._dropEpicId;
      delete targetRow._dropEpicId;
    } else if (targetRow.dataset.epic) {
      targetEpicId = targetRow.dataset.epic;
    } else {
      let prev = targetRow.previousElementSibling;
      while (prev) {
        if (prev.classList.contains('list-epic-header') && prev.dataset.epic) {
          targetEpicId = prev.dataset.epic;
          break;
        }
        if (prev.classList.contains('list-epic-header') && prev.classList.contains('list-epic-ungrouped')) {
          targetEpicId = null;
          break;
        }
        prev = prev.previousElementSibling;
      }
    }
  }

  const origEpicId = movedCard.epic_id || null;
  const epicChanged = targetEpicId !== origEpicId;

  // Remove from source column (including cleared)
  const srcArr = window.state[sourceCol] || [];
  const srcIdx = srcArr.findIndex(c => c.id === movedCard.id);
  if (srcIdx !== -1) srcArr.splice(srcIdx, 1);

  let targetCol = sourceCol;
  let insertIdx;

  if (targetRow) {
    targetCol = targetRow.dataset.col || sourceCol;
    const targetId = targetRow.dataset.id;
    const targetArr = window.state[targetCol] || [];
    const targetIdx = targetArr.findIndex(c => c.id === targetId);
    insertIdx = targetIdx >= 0 ? targetIdx : targetArr.length;
    if (sourceCol === targetCol && srcIdx < insertIdx) insertIdx--;
    if (!window.state[targetCol]) window.state[targetCol] = [];
    window.state[targetCol].splice(insertIdx, 0, movedCard);
  } else {
    if (!window.state[targetCol]) window.state[targetCol] = [];
    window.state[targetCol].push(movedCard);
    insertIdx = window.state[targetCol].length - 1;
  }

  if (epicChanged) movedCard.epic_id = targetEpicId;

  const droppedId = movedCard.id;
  window.save();

  // Clean up drag state
  document.querySelectorAll('.drop-target').forEach(el => el.classList.remove('drop-target'));
  listDragCard = null;
  listDragSourceCol = null;
  listDragEl = null;

  if (movedEl) {
    _listSurgicalDrop = true;
    movedEl.remove();
    if (targetRow) {
      targetRow.parentNode.insertBefore(movedEl, targetRow);
    } else {
      tbody.appendChild(movedEl);
    }

    // Update row data attributes if column changed
    if (sourceCol !== targetCol) {
      movedEl.dataset.col = targetCol;
      const statusWrap = movedEl.querySelector('.list-status-wrap');
      if (statusWrap) {
        const dot = statusWrap.querySelector('.list-status-dot');
        if (dot) dot.style.background = STATUS_COLORS[targetCol] || 'var(--text-dimmed)';
        const textNode = statusWrap.childNodes[1];
        if (textNode) textNode.textContent = STATUS_LABELS[targetCol] || targetCol;
      }
      if (sourceCol === 'cleared' && targetCol !== 'cleared') {
        movedEl.classList.remove('cleared');
      }
    }

    // Update epic styling
    if (epicChanged) {
      movedEl.dataset.epic = targetEpicId || '';
      if (targetEpicId) {
        movedEl.classList.add('list-epic-child');
        // Match background of sibling epic children
        if (targetRow && targetRow.style.background) {
          movedEl.style.background = targetRow.style.background;
        }
      } else {
        movedEl.classList.remove('list-epic-child');
        movedEl.style.background = '';
      }
      // Update epic group counts
      _updateEpicCounts(tbody);
    }

    // Spring animation
    movedEl.classList.remove('dragging');
    movedEl.style.opacity = '0.3';
    movedEl.style.transform = 'scale(0.98) translateY(-4px)';
    void movedEl.offsetWidth;
    movedEl.style.transition = 'opacity 0.25s ease, transform 0.4s cubic-bezier(0.34, 1.56, 0.64, 1)';
    movedEl.style.opacity = '1';
    movedEl.style.transform = 'scale(1) translateY(0)';
    movedEl.addEventListener('transitionend', function handler(ev) {
      if (ev.propertyName === 'transform') {
        movedEl.style.transition = '';
        movedEl.style.opacity = '';
        movedEl.style.transform = '';
        movedEl.removeEventListener('transitionend', handler);
      }
    });
    _listSurgicalDrop = false;
  } else {
    renderListView();
  }

  // Sync with API — serialize epic change then move to avoid version conflicts
  if (window.currentBoardId) {
    const doMove = (version) => {
      window.API.moveCard(droppedId, {
        column_id: targetCol,
        position: insertIdx,
        version: version,
      }).then(updated => {
        movedCard.version = updated.version;
        window.cardIndex[droppedId] = updated;
      }).catch(err => {
        if (err.status === 422 && err.data && err.data.blockers) {
          window.state = origState;
          window.save();
          renderListView();
          const msgs = err.data.blockers.map(b => b.message);
          window.Toast.error(msgs.join('\n'), { duration: 5000 });
        } else if (err.status === 409) {
          window.Toast.info('Card was modified, refreshing...');
          loadBoardCards(window.currentBoardId);
        } else {
          window.Toast.error('Failed to move card');
          window.state = origState;
          window.save();
          renderListView();
        }
      });
    };

    if (epicChanged && droppedId && !droppedId.startsWith('temp-')) {
      window.API.updateCard(droppedId, {
        epic_id: targetEpicId || null,
        version: movedCard.version || 0,
      }).then(updated => {
        movedCard.version = updated.version;
        window.cardIndex[droppedId] = updated;
        doMove(updated.version);
      }).catch(err => {
        window.Toast.error('Failed to update epic');
        doMove(movedCard.version || 0);
      });
    } else {
      doMove(movedCard.version || 0);
    }
  }

  // Board will pick up state changes when user switches back to board view
}

function onListDragEnd(e) {
  if (listDragEl) listDragEl.classList.remove('dragging');
  document.querySelectorAll('.drop-target').forEach(el => el.classList.remove('drop-target'));
  listDragCard = null;
  listDragSourceCol = null;
  listDragEl = null;
  // Reset didDrag after a tick so the click handler can still check it
  setTimeout(() => { listDidDrag = false; }, 0);
}

// ── Update epic group header counts after surgical move ──

// ── Surgical single-row update (e.g. after closing detail modal) ──

function _updateListRow(card) {
  const row = document.querySelector('.list-row[data-id="' + card.id + '"]');
  if (!row) return;

  // Title
  const titleCell = row.querySelector('.list-cell-title');
  if (titleCell) titleCell.textContent = card.title || '';

  // Status
  const col = card._column || card.column_id || row.dataset.col;
  row.dataset.col = col;
  const statusWrap = row.querySelector('.list-status-wrap');
  if (statusWrap) {
    const dot = statusWrap.querySelector('.list-status-dot');
    if (dot) dot.style.background = STATUS_COLORS[col] || 'var(--text-dimmed)';
    const textNode = statusWrap.childNodes[1];
    if (textNode) textNode.textContent = STATUS_LABELS[col] || col;
  }

  // Tag
  const tagSpan = row.querySelector('.list-cell-tag .card-tag');
  if (tagSpan) {
    tagSpan.className = 'card-tag tag-' + window.esc(card.tag || 'blue');
    tagSpan.textContent = window.TAG_LABELS[card.tag] || card.tag || '';
  }

  // Priority
  const priCell = row.querySelector('.list-cell-priority');
  if (priCell) {
    const priIcon = priCell.querySelector('.card-priority');
    if (priIcon) priIcon.innerHTML = window.PRIORITY_ICONS[card.priority] || window.PRIORITY_ICONS.medium;
    const priLabel = window.PRIORITIES.find(p => p.value === card.priority);
    const priText = priCell.querySelector('span');
    if (priText) {
      const textNode = priText.childNodes[1];
      if (textNode) textNode.textContent = priLabel ? priLabel.label : card.priority || '';
    }
  }

  // Assignee
  const assCell = row.querySelector('.list-cell-assignee');
  if (assCell) {
    const user = window.getUserById(card.assignee);
    if (user.value !== 'unassigned') {
      const nameNode = assCell.querySelector('span');
      if (nameNode && nameNode.childNodes.length > 1) {
        nameNode.lastChild.textContent = user.label;
      }
    }
  }

  // Points
  const ptsCell = row.querySelector('.list-cell-points');
  if (ptsCell) ptsCell.textContent = card.points || '';
}

function _updateEpicCounts(tbody) {
  const headers = tbody.querySelectorAll('.list-epic-header[data-epic]');
  headers.forEach(header => {
    const epicId = header.dataset.epic;
    const count = tbody.querySelectorAll('.list-row[data-epic="' + epicId + '"]').length;
    const countEl = header.querySelector('.list-epic-count');
    if (countEl) countEl.textContent = count + ' card' + (count !== 1 ? 's' : '');
  });
  // Update "Other" count
  const ungroupedHeader = tbody.querySelector('.list-epic-ungrouped');
  if (ungroupedHeader) {
    const allRows = tbody.querySelectorAll('.list-row');
    let ungrouped = 0;
    allRows.forEach(r => { if (!r.dataset.epic) ungrouped++; });
    const countEl = ungroupedHeader.querySelector('.list-epic-count');
    if (countEl) countEl.textContent = ungrouped + ' card' + (ungrouped !== 1 ? 's' : '');
  }
}

// ── Helpers: find card by ID across all columns ──

function findCardById(id) {
  for (const col of window.COLUMNS) {
    const card = (window.state[col.id] || []).find(c => c.id === id);
    if (card) return card;
  }
  return null;
}

function findClearedCardById(id) {
  return (window.state.cleared || []).find(c => c.id === id) || null;
}

// ═══════════════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════════════

function formatRelativeTime(iso) {
  if (!iso) return '';
  const now = Date.now();
  const then = new Date(iso).getTime();
  const diff = now - then;
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return mins + 'm ago';
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return hrs + 'h ago';
  const days = Math.floor(hrs / 24);
  if (days < 30) return days + 'd ago';
  const months = Math.floor(days / 30);
  if (months < 12) return months + 'mo ago';
  return Math.floor(months / 12) + 'y ago';
}

// ═══════════════════════════════════════════════════════════════
// LIST SEARCH FILTERING
// ═══════════════════════════════════════════════════════════════

function filterListInline(query) {
  const rows = document.querySelectorAll('#list-view .list-row');
  // window._searchExtraIds is populated by kanban.js after the global search
  // returns. Done/cleared rows that match semantically (or by ticket key) but
  // not by visible text should still be revealed.
  const extra = window._searchExtraIds || new Set();
  rows.forEach(row => {
    if (!query) {
      if (row.classList.contains('search-hidden')) {
        row.classList.remove('search-hidden');
        row.classList.add('search-reveal');
        row.addEventListener('animationend', () => row.classList.remove('search-reveal'), { once: true });
      }
      return;
    }
    const text = row.textContent.toLowerCase();
    const idMatch = extra.has(row.dataset.id);
    if (idMatch || text.includes(query)) {
      if (row.classList.contains('search-hidden')) {
        row.classList.remove('search-hidden');
        row.classList.add('search-reveal');
        row.addEventListener('animationend', () => row.classList.remove('search-reveal'), { once: true });
      }
    } else {
      row.classList.add('search-hidden');
      row.classList.remove('search-reveal');
    }
  });
}

// ═══════════════════════════════════════════════════════════════
// INIT
// ═══════════════════════════════════════════════════════════════

function initListView() {
  const board = document.getElementById('board');
  const listView = document.getElementById('list-view');

  if (currentView === 'list') {
    board.style.display = 'none';
    listView.style.display = '';
  } else {
    listView.style.display = 'none';
  }

  // Set up view switcher buttons
  document.querySelectorAll('.view-switcher-btn').forEach(btn => {
    btn.classList.toggle('active', btn.dataset.view === currentView);
    btn.addEventListener('click', () => switchView(btn.dataset.view));
  });
}

// Hook into DOMContentLoaded — runs after kanban.js init
document.addEventListener('DOMContentLoaded', () => {
  initListView();
});

// ═══════════════════════════════════════════════════════════════
// EXPOSE TO WINDOW (Vite ES module bridge)
// ═══════════════════════════════════════════════════════════════
window.currentView = currentView;
window.listSortCol = listSortCol;
window.listSortDir = listSortDir;
window.LIST_PAGE_SIZE = LIST_PAGE_SIZE;
window.STATUS_ORDER = STATUS_ORDER;
window.PRIORITY_ORDER = PRIORITY_ORDER;
window.STATUS_COLORS = STATUS_COLORS;
window.STATUS_LABELS = STATUS_LABELS;
window.switchView = switchView;
window.collectAllCards = collectAllCards;
window.sortCards = sortCards;
window.createListRow = createListRow;
window.appendNextPage = appendNextPage;
window.onListScroll = onListScroll;
window.renderListView = renderListView;
window.sortListView = sortListView;
window.onListDragStart = onListDragStart;
window.onListDragOver = onListDragOver;
window.onListDrop = onListDrop;
window.onListDragEnd = onListDragEnd;
window.findCardById = findCardById;
window.findClearedCardById = findClearedCardById;
window.formatRelativeTime = formatRelativeTime;
window.filterListInline = filterListInline;
window.initListView = initListView;
window._updateListRow = _updateListRow;
