// fn2 Kanban — Accessibility + Mobile Column Tabs

// ═══════════════════════════════════════════════════════════════
// ARIA ATTRIBUTES
// ═══════════════════════════════════════════════════════════════

function applyARIA() {
  document.querySelectorAll('.column-body').forEach(body => {
    body.setAttribute('role', 'list');
    const colId = body.dataset.col;
    const col = typeof window.COLUMNS !== 'undefined' ? window.COLUMNS.find(c => c.id === colId) : null;
    const count = body.querySelectorAll('.card').length;
    if (col) {
      body.setAttribute('aria-label', `${col.label} - ${count} card${count !== 1 ? 's' : ''}`);
    }
  });

  document.querySelectorAll('.card').forEach(cardEl => {
    cardEl.setAttribute('role', 'listitem');
    cardEl.setAttribute('tabindex', '0');

    const cardId = cardEl.dataset.id;
    const colId = cardEl.dataset.col;
    const cards = typeof window.state !== 'undefined' ? (window.state[colId] || []) : [];
    const card = cards.find(c => c.id === cardId);

    if (card) {
      const col = typeof window.COLUMNS !== 'undefined' ? window.COLUMNS.find(c => c.id === colId) : null;
      const colLabel = col ? col.label : colId;
      const priLabel = card.priority || 'medium';
      const assignee = card.assignee || 'unassigned';
      cardEl.setAttribute('aria-label',
        `${card.title}, ${priLabel} priority, assigned to ${assignee}, in ${colLabel}`);
      cardEl.setAttribute('aria-roledescription', 'Draggable card');
    }
  });

  ['create-modal', 'detail-modal', 'new-board-modal'].forEach(id => {
    const modal = document.getElementById(id);
    if (modal) {
      modal.setAttribute('role', 'dialog');
      modal.setAttribute('aria-modal', 'true');
      const title = modal.querySelector('.lwts-modal-title, .detail-header-title, .lwts-modal-header');
      if (title) {
        const titleId = id + '-title';
        title.id = title.id || titleId;
        modal.setAttribute('aria-labelledby', title.id);
      }
    }
  });

  document.querySelectorAll('.column-header').forEach(header => {
    const col = header.closest('.column');
    if (!col) return;
    const label = header.querySelector('.column-label');
    const count = header.querySelector('.column-count');
    if (label && count) {
      header.setAttribute('aria-label', `${label.textContent.trim()} - ${count.textContent.trim()} cards`);
    }
  });
}

// ═══════════════════════════════════════════════════════════════
// KEYBOARD NAVIGATION
// ═══════════════════════════════════════════════════════════════

document.addEventListener('keydown', (e) => {
  const focused = document.activeElement;

  if (focused && focused.classList.contains('card')) {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      focused.click();
      return;
    }

    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      const cards = Array.from(focused.closest('.column-body')?.querySelectorAll('.card') || []);
      const idx = cards.indexOf(focused);
      const next = e.key === 'ArrowDown' ? cards[idx + 1] : cards[idx - 1];
      if (next) next.focus();
      return;
    }

    if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') {
      e.preventDefault();
      const colBodies = Array.from(document.querySelectorAll('.column-body'));
      const currentBody = focused.closest('.column-body');
      const colIdx = colBodies.indexOf(currentBody);
      const targetBody = e.key === 'ArrowRight' ? colBodies[colIdx + 1] : colBodies[colIdx - 1];
      if (targetBody) {
        const firstCard = targetBody.querySelector('.card');
        if (firstCard) firstCard.focus();
      }
      return;
    }
  }

  if (focused && focused.closest('.filter-menu')) {
    const items = Array.from(focused.closest('.filter-menu').querySelectorAll('.filter-checkbox-item input'));
    const idx = items.indexOf(focused);
    if (e.key === 'ArrowDown' && idx < items.length - 1) {
      e.preventDefault();
      items[idx + 1].focus();
    } else if (e.key === 'ArrowUp' && idx > 0) {
      e.preventDefault();
      items[idx - 1].focus();
    }
  }
});

// ═══════════════════════════════════════════════════════════════
// LIVE REGION for real-time updates
// ═══════════════════════════════════════════════════════════════

let liveRegion = null;

function initLiveRegion() {
  if (liveRegion) return;
  liveRegion = document.createElement('div');
  liveRegion.id = 'a11y-live';
  liveRegion.setAttribute('role', 'status');
  liveRegion.setAttribute('aria-live', 'polite');
  liveRegion.setAttribute('aria-atomic', 'true');
  liveRegion.className = 'sr-only';
  document.body.appendChild(liveRegion);
}

function announce(message) {
  if (!liveRegion) initLiveRegion();
  liveRegion.textContent = '';
  requestAnimationFrame(() => {
    liveRegion.textContent = message;
  });
}

function wireA11yAnnouncements() {
  if (typeof window.currentBoardStream === 'undefined' || !window.currentBoardStream) return;

  const origCardCreated = window.currentBoardStream.handlers.onCardCreated;
  window.currentBoardStream.handlers.onCardCreated = (data) => {
    if (origCardCreated) origCardCreated(data);
    announce(`New card added: ${data.title || 'Untitled'}`);
  };

  const origCardMoved = window.currentBoardStream.handlers.onCardMoved;
  window.currentBoardStream.handlers.onCardMoved = (data) => {
    if (origCardMoved) origCardMoved(data);
    const to = data.to_column || data.column_id || '';
    announce(`Card moved to ${to}`);
  };

  const origUserJoined = window.currentBoardStream.handlers.onUserJoined;
  window.currentBoardStream.handlers.onUserJoined = (data) => {
    if (origUserJoined) origUserJoined(data);
    announce(`${data.username || 'A user'} joined the board`);
  };

  const origUserLeft = window.currentBoardStream.handlers.onUserLeft;
  window.currentBoardStream.handlers.onUserLeft = (data) => {
    if (origUserLeft) origUserLeft(data);
    announce(`${data.username || 'A user'} left the board`);
  };
}

// ═══════════════════════════════════════════════════════════════
// MOBILE COLUMN TABS
// ═══════════════════════════════════════════════════════════════

let mobileActiveCol = 'todo';

function initMobileColumnTabs() {
  if (document.getElementById('mobile-column-tabs')) return;

  const tabs = document.createElement('div');
  tabs.id = 'mobile-column-tabs';
  tabs.className = 'mobile-column-tabs';

  if (typeof window.COLUMNS !== 'undefined') {
    window.COLUMNS.forEach(col => {
      const tab = document.createElement('div');
      tab.className = 'mobile-tab' + (col.id === mobileActiveCol ? ' active' : '');
      tab.dataset.col = col.id;
      const count = document.querySelectorAll(`.card[data-col="${col.id}"]`).length ||
                    (typeof window.state !== 'undefined' ? (window.state[col.id] || []).length : 0);
      tab.innerHTML = `${col.label}<span class="tab-count">${count}</span>`;
      tab.onclick = () => switchMobileTab(col.id);
      tab.setAttribute('role', 'tab');
      tab.setAttribute('aria-selected', col.id === mobileActiveCol ? 'true' : 'false');
      tabs.appendChild(tab);
    });
  }

  const filterBar = document.getElementById('filter-bar');
  const header = document.querySelector('.header');
  const insertAfter = filterBar || header;
  if (insertAfter && insertAfter.nextSibling) {
    insertAfter.parentNode.insertBefore(tabs, insertAfter.nextSibling);
  }

  applyMobileActiveColumn();
}

function switchMobileTab(colId) {
  mobileActiveCol = colId;

  document.querySelectorAll('.mobile-tab').forEach(tab => {
    const isActive = tab.dataset.col === colId;
    tab.classList.toggle('active', isActive);
    tab.setAttribute('aria-selected', isActive ? 'true' : 'false');
  });

  applyMobileActiveColumn();
  announce(`Showing ${colId.replace('-', ' ')} column`);
}

function applyMobileActiveColumn() {
  if (window.innerWidth > 768) return;

  // Standard columns
  document.querySelectorAll('.column').forEach(col => {
    const body = col.querySelector('.column-body');
    const colId = body?.dataset.col;
    col.classList.toggle('mobile-active', colId === mobileActiveCol);
  });

  // Epic mode: show/hide cells within each epic lane
  document.querySelectorAll('.epic-lane-cell').forEach(cell => {
    const body = cell.querySelector('.column-body');
    const colId = body?.dataset.col;
    cell.classList.toggle('mobile-active', colId === mobileActiveCol);
  });
}

function updateMobileTabCounts() {
  if (typeof window.COLUMNS === 'undefined' || typeof window.state === 'undefined') return;
  document.querySelectorAll('.mobile-tab').forEach(tab => {
    const colId = tab.dataset.col;
    const count = (window.state[colId] || []).length;
    const countEl = tab.querySelector('.tab-count');
    if (countEl) countEl.textContent = count;
  });
}

// ═══════════════════════════════════════════════════════════════
// INIT
// ═══════════════════════════════════════════════════════════════

// Patch render for ARIA + mobile tabs
const _renderBeforeA11y = window.render;
if (_renderBeforeA11y) {
  window.render = function() {
    _renderBeforeA11y();
    applyARIA();
    if (window.innerWidth <= 768) {
      initMobileColumnTabs();
      applyMobileActiveColumn();
      updateMobileTabCounts();
    }
  };
}

// Patch filter apply to announce
const _origApplyFilters = typeof window.applyFilters === 'function' ? window.applyFilters : null;
if (_origApplyFilters) {
  window.applyFilters = function() {
    _origApplyFilters();
    const af = window.activeFilters || {};
    const count = (af.assignees?.length || 0) + (af.priorities?.length || 0) +
                  (af.tags?.length || 0) + (af.search ? 1 : 0);
    if (count > 0) {
      const visible = document.querySelectorAll('.card:not([style*="display: none"])').length;
      announce(`${count} filter${count > 1 ? 's' : ''} active, showing ${visible} cards`);
    }
  };
}

document.addEventListener('DOMContentLoaded', () => {
  initLiveRegion();
  applyARIA();

  let resizeTimer;
  window.addEventListener('resize', () => {
    clearTimeout(resizeTimer);
    resizeTimer = setTimeout(() => {
      if (window.innerWidth <= 768) {
        initMobileColumnTabs();
        applyMobileActiveColumn();
      }
    }, 150);
  });

  if (window.innerWidth <= 768) {
    initMobileColumnTabs();
  }

  setTimeout(() => wireA11yAnnouncements(), 2000);
});

window.applyARIA = applyARIA;
window.wireA11yAnnouncements = wireA11yAnnouncements;
window.initMobileColumnTabs = initMobileColumnTabs;
window.switchMobileTab = switchMobileTab;
window.applyMobileActiveColumn = applyMobileActiveColumn;
window.updateMobileTabCounts = updateMobileTabCounts;
window.announce = announce;
window.initLiveRegion = initLiveRegion;
