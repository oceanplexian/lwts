// fn2 Kanban — Related Tickets UI

// ═══════════════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════════════

function getRelatedCardIds(card) {
  return card.related_card_ids || [];
}

function getBlockedCardIds(card) {
  return card.blocked_card_ids || [];
}

function _findCardById(id) {
  for (const col of window.COLUMNS) {
    const c = (window.state[col.id] || []).find(c => c.id === id);
    if (c) return c;
  }
  if (window.state.cleared) {
    const c = window.state.cleared.find(c => c.id === id);
    if (c) return c;
  }
  return null;
}

function _isCardDone(card) {
  return card && card.column_id === 'done';
}

function _saveCardLinks(card, field, ids) {
  card[field] = ids;
  const json = JSON.stringify(ids);
  let promise = Promise.resolve();
  if (card.id && !card.id.startsWith('temp-') && typeof window.API !== 'undefined') {
    var update = { version: card.version || 0 };
    update[field] = json;
    promise = window.API.updateCard(card.id, update).then(updated => {
      if (updated && updated.version) {
        card.version = updated.version;
        if (window.cardIndex[card.id]) window.cardIndex[card.id].version = updated.version;
      }
    }).catch(() => {
      window.Toast.error('Failed to save link');
    });
  }
  if (typeof window.save === 'function') window.save();
  return promise;
}

// ═══════════════════════════════════════════════════════════════
// CARD BADGE (on board cards) — shows related count
// ═══════════════════════════════════════════════════════════════

function injectRelatedBadges() {
  // Link badges hidden from card view — related/blocked still visible in detail panel
}

// ═══════════════════════════════════════════════════════════════
// DETAIL MODAL — RELATED TICKETS LIST
// ═══════════════════════════════════════════════════════════════

let _relatedPickerOpen = false;
let _pickerLinkType = 'related'; // 'related' | 'blocked'

function renderRelatedTickets() {
  if (!window.detailCard) {
    var old = document.getElementById('related-section');
    if (old) old.remove();
    return;
  }

  let container = document.getElementById('related-section');
  if (!container) {
    const descEdit = document.querySelector('.detail-desc-edit');
    if (!descEdit) return;

    container = document.createElement('div');
    container.id = 'related-section';
    container.className = 'detail-related';
    descEdit.parentNode.insertBefore(container, descEdit.nextSibling);
  }

  const relIds = getRelatedCardIds(window.detailCard);
  const blkIds = getBlockedCardIds(window.detailCard);
  const totalCount = relIds.length + blkIds.length;

  container.innerHTML = '';

  // Section header
  const label = document.createElement('div');
  label.className = 'detail-section-label';
  label.style.display = 'flex';
  label.style.alignItems = 'center';
  label.style.justifyContent = 'space-between';
  label.innerHTML = `
    <span>Related${totalCount > 0 ? ' (' + totalCount + ')' : ''}</span>
    <button class="related-add-toggle" onclick="toggleRelatedPicker()" title="Link a ticket">
      <svg viewBox="0 0 24 24" width="14" height="14" stroke="currentColor" fill="none" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
    </button>
  `;
  container.appendChild(label);

  // Icon SVGs
  var _linkIcon = '<svg viewBox="0 0 24 24" width="14" height="14" stroke="currentColor" fill="none" stroke-width="2"><path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/></svg>';
  var _blockedIcon = '<svg viewBox="0 0 24 24" width="14" height="14" stroke="currentColor" fill="none" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="4.93" y1="4.93" x2="19.07" y2="19.07"/></svg>';

  function _renderLinkRow(id, type) {
    var linked = _findCardById(id);
    if (!linked) return null;
    var done = _isCardDone(linked);
    var row = document.createElement('div');
    row.className = 'related-item' + (done ? ' done' : '') + (type === 'blocked' ? ' blocked' : '');
    row.innerHTML =
      '<span class="related-item-icon' + (type === 'blocked' ? ' blocked-icon' : '') + '">' +
        (type === 'blocked' ? _blockedIcon : _linkIcon) +
      '</span>' +
      '<span class="related-item-text">' +
        '<span class="related-item-key">' + window.esc(linked.key) + '</span>' +
        '<span class="related-item-title">' + window.esc(linked.title) + '</span>' +
      '</span>' +
      '<button class="related-item-remove" onclick="event.stopPropagation(); removeLinkCard(\'' + id + '\',\'' + type + '\')" title="Unlink">&times;</button>';
    row.addEventListener('click', function() {
      _detailNavMode = 'related';
      window.openDetail(id);
      _detailNavMode = 'normal';
    });
    return row;
  }

  if (relIds.length > 0 || blkIds.length > 0) {
    var list = document.createElement('div');
    list.className = 'related-list';
    blkIds.forEach(function(id) { var r = _renderLinkRow(id, 'blocked'); if (r) list.appendChild(r); });
    relIds.forEach(function(id) { var r = _renderLinkRow(id, 'related'); if (r) list.appendChild(r); });
    container.appendChild(list);
  }

  if (_relatedPickerOpen) {
    var picker = document.createElement('div');
    picker.className = 'related-picker';
    picker.innerHTML =
      '<div class="related-picker-type-row">' +
        '<button class="related-picker-type' + (_pickerLinkType === 'related' ? ' active' : '') + '" data-type="related">Related</button>' +
        '<button class="related-picker-type' + (_pickerLinkType === 'blocked' ? ' active' : '') + '" data-type="blocked">Blocked by</button>' +
      '</div>' +
      '<input type="text" class="related-picker-input" id="related-picker-input" placeholder="Search by key or title..." autofocus>' +
      '<div class="related-picker-results" id="related-picker-results"></div>';
    container.appendChild(picker);

    requestAnimationFrame(function() {
      // Type toggle buttons
      picker.querySelectorAll('.related-picker-type').forEach(function(btn) {
        btn.addEventListener('click', function(e) {
          e.stopPropagation();
          _pickerLinkType = btn.dataset.type;
          picker.querySelectorAll('.related-picker-type').forEach(function(b) { b.classList.toggle('active', b.dataset.type === _pickerLinkType); });
        });
      });
      var input = document.getElementById('related-picker-input');
      if (input) {
        input.focus();
        input.addEventListener('input', function() { _filterRelatedPicker(input.value); });
        input.addEventListener('keydown', function(e) {
          if (e.key === 'Escape') { _relatedPickerOpen = false; renderRelatedTickets(); }
        });
        _filterRelatedPicker('');
      }
      function _onClickOutside(e) {
        var p = document.querySelector('.related-picker');
        var toggle = document.querySelector('.related-add-toggle');
        if (p && !p.contains(e.target) && (!toggle || !toggle.contains(e.target))) {
          _relatedPickerOpen = false;
          renderRelatedTickets();
          document.removeEventListener('mousedown', _onClickOutside, true);
        }
      }
      document.addEventListener('mousedown', _onClickOutside, true);
    });
  }
}

function toggleRelatedPicker() {
  _relatedPickerOpen = !_relatedPickerOpen;
  renderRelatedTickets();
}

function _filterRelatedPicker(query) {
  var results = document.getElementById('related-picker-results');
  if (!results || !window.detailCard) return;
  results.innerHTML = '';

  var q = query.toLowerCase().trim();
  // Exclude cards already linked as either type
  var existing = new Set(getRelatedCardIds(window.detailCard).concat(getBlockedCardIds(window.detailCard)));
  existing.add(window.detailCard.id);

  var matches = [];
  for (var col of window.COLUMNS) {
    for (var card of (window.state[col.id] || [])) {
      if (existing.has(card.id)) continue;
      if (q && !card.key.toLowerCase().includes(q) && !card.title.toLowerCase().includes(q)) continue;
      matches.push(card);
      if (matches.length >= 8) break;
    }
    if (matches.length >= 8) break;
  }

  if (matches.length === 0) {
    results.innerHTML = '<div class="related-picker-empty">No matching tickets</div>';
    return;
  }

  matches.forEach(function(card) {
    var done = _isCardDone(card);
    var opt = document.createElement('div');
    opt.className = 'related-picker-option' + (done ? ' done' : '');
    opt.innerHTML =
      '<span class="related-item-key">' + window.esc(card.key) + '</span>' +
      '<span class="related-item-title">' + window.esc(card.title) + '</span>';
    opt.addEventListener('click', function() {
      addLinkCard(card.id, _pickerLinkType);
    });
    results.appendChild(opt);
  });
}

function addLinkCard(targetId, type) {
  if (!window.detailCard) return;
  var field = type === 'blocked' ? 'blocked_card_ids' : 'related_card_ids';
  var ids = (window.detailCard[field] || []).slice();
  if (ids.includes(targetId)) return;

  ids.push(targetId);
  _saveCardLinks(window.detailCard, field, ids).then(() => {
    // Bidirectional: add reverse link on the target (after first save completes)
    var target = _findCardById(targetId);
    if (target) {
      var reverseIds = (target[field] || []).slice();
      if (!reverseIds.includes(window.detailCard.id)) {
        reverseIds.push(window.detailCard.id);
        _saveCardLinks(target, field, reverseIds);
      }
    }
  });

  _relatedPickerOpen = false;
  renderRelatedTickets();
}

function removeLinkCard(targetId, type) {
  if (!window.detailCard) return;
  var field = type === 'blocked' ? 'blocked_card_ids' : 'related_card_ids';
  var ids = window.detailCard[field] || [];
  var idx = ids.indexOf(targetId);
  if (idx === -1) return;

  // Animate out, then remove
  var rows = document.querySelectorAll('.related-item');
  var targetRow = null;
  rows.forEach(function(r) {
    if (r.querySelector('[onclick*="' + targetId + '"]')) targetRow = r;
  });

  function doRemove() {
    ids.splice(idx, 1);
    _saveCardLinks(window.detailCard, field, ids).then(() => {
      var target = _findCardById(targetId);
      if (target) {
        var reverseIds = target[field] || [];
        var tIdx = reverseIds.indexOf(window.detailCard.id);
        if (tIdx !== -1) {
          reverseIds.splice(tIdx, 1);
          _saveCardLinks(target, field, reverseIds);
        }
      }
    });
    renderRelatedTickets();
  }

  if (targetRow) {
    targetRow.classList.add('removing');
    targetRow.addEventListener('animationend', doRemove, { once: true });
  } else {
    doRemove();
  }
}

// ═══════════════════════════════════════════════════════════════
// DETAIL NAVIGATION STACK (back button for related ticket drill-down)
// ═══════════════════════════════════════════════════════════════

let _detailHistory = [];
let _detailNavMode = 'normal'; // 'normal' | 'related' | 'back'

function _updateBackButton() {
  const btn = document.getElementById('detail-back-btn');
  if (!btn) return;
  if (_detailHistory.length > 0) {
    const prev = _findCardById(_detailHistory[_detailHistory.length - 1]);
    btn.classList.remove('hidden');
    btn.title = prev ? 'Back to ' + prev.key : 'Back';
  } else {
    btn.classList.add('hidden');
  }
}

function detailGoBack() {
  if (_detailHistory.length === 0) return;
  const prevId = _detailHistory.pop();
  _detailNavMode = 'back';
  window.openDetail(prevId);
  _detailNavMode = 'normal';
}

// ═══════════════════════════════════════════════════════════════
// INIT — Hook into render + openDetail
// ═══════════════════════════════════════════════════════════════

const _renderBeforeRelated = window.render;
if (_renderBeforeRelated) {
  window.render = function() {
    _renderBeforeRelated();
    injectRelatedBadges();
  };
}

const _openDetailBeforeRelated = window.openDetail;
if (_openDetailBeforeRelated) {
  window.openDetail = function(cardId, _fromHash) {
    if (_detailNavMode === 'related' && window.detailCard && window.detailCard.id !== cardId) {
      _detailHistory.push(window.detailCard.id);
    } else if (_detailNavMode === 'normal') {
      _detailHistory = [];
    }
    _relatedPickerOpen = false;
    _openDetailBeforeRelated(cardId, _fromHash);
    // Animate content transition on related/back navigation
    if (_detailNavMode === 'related' || _detailNavMode === 'back') {
      var body = document.querySelector('#detail-modal .detail-body');
      if (body) {
        var cls = _detailNavMode === 'related' ? 'nav-forward' : 'nav-back';
        body.classList.remove('nav-forward', 'nav-back');
        void body.offsetWidth;
        body.classList.add(cls);
        body.addEventListener('animationend', function() { body.classList.remove(cls); }, { once: true });
      }
    }
    _updateBackButton();
    renderRelatedTickets();
  };
}

// Clear history when detail modal is closed
const _closeDetailBeforeRelated = window.closeDetail;
if (_closeDetailBeforeRelated) {
  window.closeDetail = function() {
    _detailHistory = [];
    _closeDetailBeforeRelated();
  };
}

document.addEventListener('DOMContentLoaded', () => {
  if (typeof window.state !== 'undefined' && typeof window.COLUMNS !== 'undefined') {
    window.COLUMNS.forEach(col => {
      (window.state[col.id] || []).forEach(card => {
        if (!card.related_card_ids) card.related_card_ids = [];
        if (!card.blocked_card_ids) card.blocked_card_ids = [];
      });
    });
  }
  injectRelatedBadges();
});

window.getRelatedCardIds = getRelatedCardIds;
window.getBlockedCardIds = getBlockedCardIds;
window.injectRelatedBadges = injectRelatedBadges;
window.renderRelatedTickets = renderRelatedTickets;
window.toggleRelatedPicker = toggleRelatedPicker;
window.addLinkCard = addLinkCard;
window.removeLinkCard = removeLinkCard;
window.detailGoBack = detailGoBack;
