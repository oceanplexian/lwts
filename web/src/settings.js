// ── Confirm Modal (replaces browser confirm()) ──

// Hash routing suppression flag (shared with kanban.js)
var _suppressHashChange = false;

let _fnConfirmResolve = null;

function fnConfirm(message, title, okLabel) {
  return new Promise(resolve => {
    _fnConfirmResolve = resolve;
    document.getElementById('confirm-modal-title').textContent = title || 'Confirm';
    document.getElementById('confirm-modal-message').textContent = message;
    const okBtn = document.getElementById('confirm-modal-ok');
    okBtn.textContent = okLabel || 'Confirm';
    // Danger styling if title contains danger words
    if (/delete|remove|revoke|clear|reset/i.test(title || message)) {
      okBtn.style.background = 'rgba(248,113,113,0.15)';
      okBtn.style.borderColor = 'rgba(248,113,113,0.4)';
      okBtn.style.color = 'rgba(248,113,113,0.95)';
    } else {
      okBtn.style.background = '';
      okBtn.style.borderColor = '';
      okBtn.style.color = '';
    }
    document.getElementById('confirm-modal').classList.add('active');
  });
}

function fnConfirmResolve(result) {
  document.getElementById('confirm-modal').classList.remove('active');
  if (_fnConfirmResolve) {
    _fnConfirmResolve(result);
    _fnConfirmResolve = null;
  }
}

// ── User Modal (shared for create + edit) ──

let _userModalRoleDropdown = null;
let _userModalEditId = null; // null = create mode, string = edit mode

function _initUserRoleDropdown(value) {
  const roleEl = document.getElementById('create-user-role');
  if (!_userModalRoleDropdown) {
    _userModalRoleDropdown = new window.FnDropdown(roleEl, {
      options: [
        { value: 'member', label: 'Member' },
        { value: 'admin', label: 'Admin' },
        { value: 'viewer', label: 'Viewer' }
      ],
      value: value || 'member',
      compact: true
    });
  } else {
    _userModalRoleDropdown.setValue(value || 'member', true);
  }
}

const AVATAR_OPTIONS = [
  'flower', 'leaf', 'wave', 'mountain', 'sun',
  'star', 'cloud', 'lightning', 'drop', 'flame',
];

function _initAvatarPicker(currentUrl) {
  const picker = document.getElementById('avatar-picker');
  const hidden = document.getElementById('create-user-avatar');
  if (!picker) return;
  hidden.value = currentUrl || '';
  picker.innerHTML = '';

  AVATAR_OPTIONS.forEach((name, i) => {
    const color = _userColors[i % _userColors.length];
    const url = '/avatars/options/' + name + '.png';
    const opt = document.createElement('div');
    opt.className = 'avatar-picker-option' + (currentUrl === url ? ' selected' : '');
    opt.style.borderColor = color + '40';
    opt.innerHTML = '<img src="' + url + '" alt="' + name + '">' +
      '<svg class="avatar-check" viewBox="0 0 24 24"><path d="M20 6L9 17l-5-5"/></svg>';
    opt.addEventListener('click', () => {
      picker.querySelectorAll('.avatar-picker-option').forEach(o => o.classList.remove('selected'));
      opt.classList.add('selected');
      hidden.value = url;
    });
    picker.appendChild(opt);
  });
}

// Avatar upload handler
document.addEventListener('DOMContentLoaded', () => {
  const uploadInput = document.getElementById('avatar-upload-input');
  if (uploadInput) {
    uploadInput.addEventListener('change', (e) => {
      const file = e.target.files[0];
      if (!file) return;
      const reader = new FileReader();
      reader.onload = () => {
        document.getElementById('create-user-avatar').value = reader.result;
        // Deselect any picked avatar
        document.querySelectorAll('#avatar-picker .avatar-picker-option').forEach(o => o.classList.remove('selected'));
      };
      reader.readAsDataURL(file);
      uploadInput.value = '';
    });
  }
});

let _userModalBotMode = false;

function openCreateUserModal() {
  _userModalEditId = null;
  _userModalBotMode = false;
  document.getElementById('user-modal-title').textContent = 'Add team member';
  document.getElementById('create-user-name').value = '';
  document.getElementById('create-user-email').value = '';
  document.getElementById('create-user-email').disabled = false;
  document.getElementById('create-user-email').style.opacity = '';
  document.getElementById('create-user-password').value = '';
  document.getElementById('create-user-password').disabled = false;
  document.getElementById('create-user-password').style.opacity = '';
  document.getElementById('create-user-password').placeholder = 'Initial password';
  document.getElementById('user-modal-submit').textContent = 'Create';
  document.getElementById('user-modal-delete').classList.add('hidden');
  const botNotice = document.getElementById('user-modal-bot-notice');
  if (botNotice) botNotice.style.display = 'none';
  _initUserRoleDropdown('member');
  _initAvatarPicker('');
  document.getElementById('create-user-modal').classList.add('active');
  setTimeout(() => document.getElementById('create-user-name').focus(), 100);
}

function openCreateBotModal() {
  _userModalEditId = null;
  _userModalBotMode = true;
  document.getElementById('user-modal-title').textContent = 'Create bot';
  document.getElementById('create-user-name').value = '';
  document.getElementById('create-user-email').value = '';
  document.getElementById('create-user-email').disabled = true;
  document.getElementById('create-user-email').style.opacity = '0.35';
  document.getElementById('create-user-email').placeholder = 'Auto-generated for bots';
  document.getElementById('create-user-password').value = '';
  document.getElementById('create-user-password').disabled = true;
  document.getElementById('create-user-password').style.opacity = '0.35';
  document.getElementById('create-user-password').placeholder = 'Not required for bots';
  document.getElementById('user-modal-submit').textContent = 'Create Bot';
  document.getElementById('user-modal-delete').classList.add('hidden');
  // Show bot notice
  let botNotice = document.getElementById('user-modal-bot-notice');
  if (!botNotice) {
    botNotice = document.createElement('div');
    botNotice.id = 'user-modal-bot-notice';
    botNotice.className = 'settings-restricted-notice';
    botNotice.textContent = 'Bots authenticate via API keys. Create one in API Keys after saving.';
    const form = document.getElementById('create-user-name').closest('.lwts-modal-body') || document.getElementById('create-user-name').parentNode.parentNode;
    form.insertBefore(botNotice, form.firstChild);
  }
  botNotice.style.display = '';
  _initUserRoleDropdown('member');
  _initAvatarPicker('');
  document.getElementById('create-user-modal').classList.add('active');
  setTimeout(() => document.getElementById('create-user-name').focus(), 100);
}

function openEditUserModal(user) {
  _userModalEditId = user.id;
  document.getElementById('user-modal-title').textContent = user.name;
  document.getElementById('create-user-name').value = user.name;
  document.getElementById('create-user-email').value = user.email;
  document.getElementById('create-user-password').value = '';
  document.getElementById('create-user-password').placeholder = 'New password (leave blank to keep)';
  document.getElementById('user-modal-submit').textContent = 'Save';
  const me = window.Auth.getUser && window.Auth.getUser();
  const isMe = me && me.id === user.id;
  const deleteBtn = document.getElementById('user-modal-delete');
  deleteBtn.classList.toggle('hidden', isMe || user.role === 'owner');
  _initUserRoleDropdown(user.role === 'owner' ? 'admin' : user.role);
  _initAvatarPicker(user.avatar_url || '');
  document.getElementById('create-user-modal').classList.add('active');
  setTimeout(() => document.getElementById('create-user-name').focus(), 100);
}

function openEditBotModal(bot) {
  _userModalEditId = bot.id;
  _userModalBotMode = true;
  document.getElementById('user-modal-title').textContent = bot.name;
  document.getElementById('create-user-name').value = bot.name;
  document.getElementById('create-user-email').value = '';
  document.getElementById('create-user-email').disabled = true;
  document.getElementById('create-user-email').style.opacity = '0.35';
  document.getElementById('create-user-email').placeholder = 'Auto-generated for bots';
  document.getElementById('create-user-password').value = '';
  document.getElementById('create-user-password').disabled = true;
  document.getElementById('create-user-password').style.opacity = '0.35';
  document.getElementById('create-user-password').placeholder = 'Not required for bots';
  document.getElementById('user-modal-submit').textContent = 'Save';
  const deleteBtn = document.getElementById('user-modal-delete');
  deleteBtn.classList.remove('hidden');
  _initUserRoleDropdown(bot.role === 'owner' ? 'admin' : bot.role);
  _initAvatarPicker('');
  const botNotice = document.getElementById('user-modal-bot-notice');
  if (botNotice) botNotice.style.display = '';
  document.getElementById('create-user-modal').classList.add('active');
  setTimeout(() => document.getElementById('create-user-name').focus(), 100);
}

function closeCreateUserModal() {
  document.getElementById('create-user-modal').classList.remove('active');
}

async function submitUserModal() {
  const name = document.getElementById('create-user-name').value.trim();
  const email = document.getElementById('create-user-email').value.trim();
  const password = document.getElementById('create-user-password').value;
  const role = _userModalRoleDropdown ? _userModalRoleDropdown.getValue() : 'member';
  if (!name) { window.Toast.error('Name is required'); return; }
  if (!_userModalBotMode && (!email || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email))) { window.Toast.error('Valid email is required'); return; }

  const avatarUrl = document.getElementById('create-user-avatar').value;

  if (_userModalEditId) {
    // Edit mode
    try {
      await window.API.updateUserRole(_userModalEditId, role);
      if (avatarUrl !== undefined) {
        await window.API.updateUserAvatar(_userModalEditId, avatarUrl);
      }
      closeCreateUserModal();
      window.Toast.success('User updated');
      loadTeamMembers();
    } catch (e) {
      window.Toast.error(e.message || 'Failed to update user');
    }
  } else if (_userModalBotMode) {
    // Bot create mode
    try {
      await window.API.createBot(name, role);
      closeCreateUserModal();
      window.Toast.success(name + ' bot created');
      loadTeamMembers();
    } catch (e) {
      window.Toast.error(e.message || 'Failed to create bot');
    }
  } else {
    // User create mode
    if (!password) { window.Toast.error('Password is required'); return; }
    try {
      await window.API.createUser(name, email, password, role);
      closeCreateUserModal();
      window.Toast.success(name + ' added to the team');
      loadTeamMembers();
    } catch (e) {
      window.Toast.error(e.message || 'Failed to create user');
    }
  }
}

async function deleteUserFromModal() {
  if (!_userModalEditId) return;
  const name = document.getElementById('create-user-name').value.trim();
  try {
    await window.API.deleteUser(_userModalEditId);
    closeCreateUserModal();
    window.Toast.success(name + ' removed');
    loadTeamMembers();
  } catch (e) {
    window.Toast.error(e.message);
  }
}

// fn2 Kanban — Settings Page

const SETTINGS_SECTIONS = [
  { id: 'general',     label: 'General',      icon: '<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>', group: 'workspace' },
  { id: 'appearance',  label: 'Appearance',    icon: '<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="5"/><line x1="12" y1="1" x2="12" y2="3"/><line x1="12" y1="21" x2="12" y2="23"/><line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/><line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/><line x1="1" y1="12" x2="3" y2="12"/><line x1="21" y1="12" x2="23" y2="12"/><line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/><line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/></svg>', group: 'workspace' },
  { id: 'boards',      label: 'Boards',        icon: '<svg viewBox="0 0 24 24"><rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/></svg>', group: 'workspace' },
  { id: 'transitions', label: 'Transitions',   icon: '<svg viewBox="0 0 24 24"><polyline points="13 17 18 12 13 7"/><polyline points="6 17 11 12 6 7"/></svg>', group: 'workspace' },
  { id: 'team',        label: 'Team',          icon: '<svg viewBox="0 0 24 24"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>', group: 'people' },
  { id: 'notifications', label: 'Notifications', icon: '<svg viewBox="0 0 24 24"><path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.73 21a2 2 0 0 1-3.46 0"/></svg>', group: 'people' },
  { id: 'integrations', label: 'Integrations', icon: '<svg viewBox="0 0 24 24"><path d="M16 18l2-2-4-4 4-4-2-2-6 6z"/><path d="M8 6L6 8l4 4-4 4 2 2 6-6z"/></svg>', group: 'people' },
  { id: 'api',         label: 'API Keys',      icon: '<svg viewBox="0 0 24 24"><path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.778 7.778 5.5 5.5 0 0 1 7.777-7.777zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4"/></svg>', group: 'advanced' },
  { id: 'import',      label: 'Import / Export', icon: '<svg viewBox="0 0 24 24"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>', group: 'advanced' },
  { id: 'danger',      label: 'Danger Zone',   icon: '<svg viewBox="0 0 24 24"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>', group: 'advanced' },
];

const SETTINGS_GROUPS = {
  workspace: 'Workspace',
  people: 'People',
  advanced: 'Advanced',
};

let activeSettingsSection = 'general';
let _lambdaDemoMode = null;

function _isLambdaDemoMode() {
  return _lambdaDemoMode === true;
}

async function _ensureLambdaDemoMode() {
  if (_lambdaDemoMode !== null) return _isLambdaDemoMode();
  try {
    const res = await fetch('/api/v1/lambda-demo');
    if (!res.ok) throw new Error('failed to load lambda demo status');
    const data = await res.json();
    _lambdaDemoMode = data && data.lambda_demo === true;
  } catch (e) {
    _lambdaDemoMode = false;
  }
  return _isLambdaDemoMode();
}

function toggleBoardConfig(id) {
  const el = document.getElementById(id);
  if (!el) return;
  const card = el.closest('.settings-board-card');
  const isOpen = !el.classList.contains('hidden');

  // Close ALL board configs first
  document.querySelectorAll('.settings-board-config').forEach(cfg => {
    cfg.classList.add('hidden');
    const parentCard = cfg.closest('.settings-board-card');
    if (parentCard) parentCard.classList.remove('open');
  });

  // If it was closed, open it
  if (!isOpen) {
    el.classList.remove('hidden');
    if (card) card.classList.add('open');
  }
}

function renderSettingsNav() {
  const nav = document.getElementById('settings-nav');
  if (!nav) return;
  nav.innerHTML = '';

  let lastGroup = null;
  SETTINGS_SECTIONS.forEach(sec => {
    if (sec.group !== lastGroup) {
      lastGroup = sec.group;
      const header = document.createElement('div');
      header.className = 'settings-nav-section';
      header.textContent = SETTINGS_GROUPS[sec.group] || sec.group;
      nav.appendChild(header);
    }
    const item = document.createElement('div');
    item.className = 'settings-nav-item' + (sec.id === window.activeSettingsSection ? ' active' : '');
    item.innerHTML = sec.icon + '<span>' + sec.label + '</span>';
    item.onclick = () => showSettingsSection(sec.id);
    nav.appendChild(item);
  });
}

function _isAdmin() {
  const role = window.currentUser && window.currentUser.role;
  return role === 'admin' || role === 'owner';
}

function _addRestrictionNotice(sectionEl, message) {
  if (sectionEl.querySelector('.settings-restricted-notice')) return;
  const notice = document.createElement('div');
  notice.className = 'settings-restricted-notice';
  notice.textContent = message;
  const desc = sectionEl.querySelector('.settings-section-desc');
  if (desc) {
    desc.after(notice);
  } else {
    sectionEl.prepend(notice);
  }
}

function _applyRoleRestrictions() {
  if (_isAdmin()) return;

  // General — disable all inputs
  const general = document.getElementById('settings-general');
  if (general) {
    general.classList.add('settings-restricted');
    _addRestrictionNotice(general, 'Only admins can change workspace settings.');
  }

  // Boards — read-only config
  const boards = document.getElementById('settings-boards');
  if (boards) {
    boards.classList.add('settings-restricted');
    _addRestrictionNotice(boards, 'Only admins can modify board configuration.');
  }

  // Transitions — disable toggles
  const transitions = document.getElementById('settings-transitions');
  if (transitions) {
    transitions.classList.add('settings-restricted');
    _addRestrictionNotice(transitions, 'Only admins can change transition rules.');
  }

  // Integrations — restrict
  const integrations = document.getElementById('settings-integrations');
  if (integrations) {
    integrations.classList.add('settings-restricted');
    _addRestrictionNotice(integrations, 'Only admins can manage integrations.');
  }

  // Team — hide add member button
  const team = document.getElementById('settings-team');
  if (team) {
    const addBtn = team.querySelector('.settings-section-action');
    if (addBtn) addBtn.style.display = 'none';
  }

  // Danger Zone — fully hidden
  const danger = document.getElementById('settings-danger');
  if (danger) {
    danger.style.display = 'none';
  }

  // Also hide danger from nav
  const navItems = document.querySelectorAll('.settings-nav-item');
  navItems.forEach((el, i) => {
    if (SETTINGS_SECTIONS[i] && SETTINGS_SECTIONS[i].id === 'danger') {
      el.style.display = 'none';
    }
  });
}

function _applyLambdaDemoRestrictions() {
  if (!_isLambdaDemoMode()) return;

  const api = document.getElementById('settings-api');
  if (api) {
    api.classList.add('settings-restricted');
    _addRestrictionNotice(api, 'API keys are disabled in Lambda demo mode.');
    const addBtn = api.querySelector('.settings-section-action');
    if (addBtn) addBtn.style.display = 'none';
  }

  const integrations = document.getElementById('settings-integrations');
  if (integrations) {
    integrations.classList.add('settings-restricted');
    _addRestrictionNotice(integrations, 'Integrations are disabled in Lambda demo mode.');
    integrations.querySelectorAll('.integration-card').forEach(card => card.classList.add('disabled'));
  }

  const importExport = document.getElementById('settings-import');
  if (importExport) {
    importExport.classList.add('settings-restricted');
    _addRestrictionNotice(importExport, 'Import/export is disabled in Lambda demo mode.');
  }
}

function showSettingsSection(id, _fromHash) {
  window.activeSettingsSection = id;
  document.querySelectorAll('.settings-nav-item').forEach((el, i) => {
    el.classList.toggle('active', SETTINGS_SECTIONS[i].id === id);
  });
  document.querySelectorAll('.settings-section').forEach(el => {
    el.classList.toggle('active', el.id === 'settings-' + id);
  });

  // Update hash with section (replaceState to avoid polluting history)
  if (!_fromHash) {
    window._suppressHashChange = true;
    const hashSection = id === 'general' ? '#settings' : '#settings/' + id;
    history.replaceState(null, '', hashSection);
    window._suppressHashChange = false;
  }

  _applyRoleRestrictions();
  _applyLambdaDemoRestrictions();
}

// ── Appearance Application ──

const DEFAULT_LANE_OPACITY = 81;
const DEFAULT_CARD_OPACITY = 100;
const DEFAULT_SURFACE_BLUR = 15;

function _resolveThemeId(themeId) {
  if (typeof window.resolveBoardThemeId === 'function') {
    return window.resolveBoardThemeId(themeId);
  }
  return (window.BOARD_THEMES && window.BOARD_THEMES[themeId]) ? themeId : 'default';
}

function _normalizeAppearanceSettings(data) {
  return Object.assign({
    theme: 'default',
    lane_opacity: DEFAULT_LANE_OPACITY,
    card_opacity: DEFAULT_CARD_OPACITY,
    surface_blur: DEFAULT_SURFACE_BLUR,
  }, data || {});
}

function _normalizeLaneOpacity(value) {
  const numeric = Number(value);
  if (!Number.isFinite(numeric)) return DEFAULT_LANE_OPACITY;
  const rounded = Math.round(numeric);
  return Math.min(100, Math.max(0, rounded));
}

function _updateSettingsRangeFill(input) {
  if (!input) return;
  const min = Number(input.min || 0);
  const max = Number(input.max || 100);
  const value = Number(input.value || min);
  const percent = max === min ? 0 : ((value - min) / (max - min)) * 100;
  input.style.setProperty('--range-fill', percent.toFixed(2) + '%');
}

function _applyLaneOpacity(value) {
  const laneOpacity = _normalizeLaneOpacity(value);
  const listOpacity = Math.round(laneOpacity * 0.79);
  const rowOpacity = Math.round(laneOpacity * 0.43);
  const rowHoverOpacity = Math.round(laneOpacity * 0.86);
  const shellAlpha = laneOpacity <= 0 ? 0 : 0.09 * Math.sqrt(laneOpacity / 100);
  const rootStyle = document.documentElement.style;
  rootStyle.setProperty('--board-pattern-lane-opacity', laneOpacity + '%');
  rootStyle.setProperty('--board-pattern-list-opacity', listOpacity + '%');
  rootStyle.setProperty('--board-pattern-row-opacity', rowOpacity + '%');
  rootStyle.setProperty('--board-pattern-row-hover-opacity', rowHoverOpacity + '%');
  rootStyle.setProperty('--board-pattern-shell-alpha', shellAlpha.toFixed(3));

  const input = document.querySelector('[data-setting="appearance.lane_opacity"]');
  if (input && input.value !== String(laneOpacity)) {
    input.value = String(laneOpacity);
  }
  _updateSettingsRangeFill(input);
  const valueEl = document.getElementById('settings-lane-opacity-value');
  if (valueEl) valueEl.textContent = laneOpacity + '%';
}

function _normalizeSurfaceBlur(value) {
  const numeric = Number(value);
  if (!Number.isFinite(numeric)) return DEFAULT_SURFACE_BLUR;
  const rounded = Math.round(numeric);
  return Math.min(100, Math.max(0, rounded));
}

function _getSurfaceBlurScale(blur) {
  if (blur <= 50) return blur / 50;
  return 1 + ((blur - 50) / 50) * 0.5;
}

function _applySurfaceBlur(value) {
  const surfaceBlur = _normalizeSurfaceBlur(value);
  const blurScale = _getSurfaceBlurScale(surfaceBlur);
  const rootStyle = document.documentElement.style;
  rootStyle.setProperty('--board-pattern-epic-blur', (12 * blurScale).toFixed(2) + 'px');
  rootStyle.setProperty('--board-pattern-column-blur', (2 * blurScale).toFixed(2) + 'px');
  rootStyle.setProperty('--board-pattern-list-blur', (24 * blurScale).toFixed(2) + 'px');
  rootStyle.setProperty('--board-pattern-list-header-blur', (38 * blurScale).toFixed(2) + 'px');
  rootStyle.setProperty('--board-pattern-epic-header-blur', (24 * blurScale).toFixed(2) + 'px');
  rootStyle.setProperty('--board-pattern-epic-child-blur', (18 * blurScale).toFixed(2) + 'px');

  const input = document.querySelector('[data-setting="appearance.surface_blur"]');
  if (input && input.value !== String(surfaceBlur)) {
    input.value = String(surfaceBlur);
  }
  _updateSettingsRangeFill(input);
  const valueEl = document.getElementById('settings-surface-blur-value');
  if (valueEl) valueEl.textContent = surfaceBlur + '%';
}

function _normalizeCardOpacity(value) {
  const numeric = Number(value);
  if (!Number.isFinite(numeric)) return DEFAULT_CARD_OPACITY;
  const rounded = Math.round(numeric);
  return Math.min(100, Math.max(0, rounded));
}

function _applyCardOpacity(value) {
  const cardOpacity = _normalizeCardOpacity(value);
  const lightAlpha = Math.max(0.82, Math.min(0.96, 0.52 + (cardOpacity / 200)));
  const rootStyle = document.documentElement.style;
  rootStyle.setProperty('--board-pattern-card-opacity', cardOpacity + '%');
  rootStyle.setProperty('--board-pattern-card-hover-opacity', Math.min(98, cardOpacity + 8) + '%');
  rootStyle.setProperty('--board-pattern-card-dark-opacity', Math.min(96, cardOpacity + 6) + '%');
  rootStyle.setProperty('--board-pattern-card-dark-hover-opacity', Math.min(98, cardOpacity + 8) + '%');
  rootStyle.setProperty('--board-pattern-card-light-alpha', lightAlpha.toFixed(3));
  rootStyle.setProperty('--board-pattern-card-light-hover-alpha', Math.min(0.98, lightAlpha + 0.04).toFixed(3));

  const input = document.querySelector('[data-setting="appearance.card_opacity"]');
  if (input && input.value !== String(cardOpacity)) {
    input.value = String(cardOpacity);
  }
  _updateSettingsRangeFill(input);
  const valueEl = document.getElementById('settings-card-opacity-value');
  if (valueEl) valueEl.textContent = cardOpacity + '%';
}

function _renderThemeOptions(selectedThemeId) {
  const resolvedThemeId = _resolveThemeId(selectedThemeId);
  const themes = Object.values(window.BOARD_THEMES || {});
  return themes.map(theme => `
    <button
      type="button"
      class="board-theme-option appearance-theme-option${theme.id === resolvedThemeId ? ' selected' : ''}"
      data-theme-id="${theme.id}"
      aria-pressed="${theme.id === resolvedThemeId ? 'true' : 'false'}"
    >
      <span class="board-theme-preview" data-theme-id="${theme.id}">
        <span class="board-theme-preview-card card-a"></span>
        <span class="board-theme-preview-card card-b"></span>
        <span class="board-theme-preview-card card-c"></span>
      </span>
      <span class="board-theme-copy">
        <span class="board-theme-label">${_escHtml(theme.label)}</span>
        <span class="board-theme-desc">${_escHtml(theme.description)}</span>
      </span>
    </button>
  `).join('');
}

function _setSelectedAppearanceTheme(themeId) {
  const resolvedThemeId = _resolveThemeId(themeId);
  document.querySelectorAll('.appearance-theme-option').forEach(el => {
    const isSelected = el.dataset.themeId === resolvedThemeId;
    el.classList.toggle('selected', isSelected);
    el.setAttribute('aria-pressed', isSelected ? 'true' : 'false');
  });
}

function _renderAppearanceThemePicker(selectedThemeId) {
  const container = document.getElementById('settings-theme-grid');
  if (!container) return;
  container.innerHTML = _renderThemeOptions(selectedThemeId);
  document.querySelectorAll('.appearance-theme-option').forEach(el => {
    el.onclick = () => _saveAppearanceTheme(el.dataset.themeId);
  });
}

function _saveAppearanceTheme(themeId) {
  const resolvedThemeId = _resolveThemeId(themeId);
  if (!_settingsCache.appearance) _settingsCache.appearance = _normalizeAppearanceSettings();
  _settingsCache.appearance.theme = resolvedThemeId;
  applyAppearanceSettings({ theme: resolvedThemeId });
  clearTimeout(_settingsDebounce.appearance);
  _settingsDebounce.appearance = setTimeout(() => {
    window.API.putSettings('appearance', { theme: resolvedThemeId }).catch(() => {});
  }, 500);
}

function applyAppearanceSettings(data) {
  const body = document.body;

  // Cache to localStorage for anti-FOUC on next load
  try {
    var cached = JSON.parse(localStorage.getItem('lwts-appearance') || '{}');
    Object.assign(cached, data);
    localStorage.setItem('lwts-appearance', JSON.stringify(cached));
  } catch(e) {}

  // Dark mode (default is dark; toggling OFF = light theme)
  if ('dark_mode' in data) {
    body.classList.toggle('light-theme', !data.dark_mode);
  }

  // Accent color
  if (data.accent_color) {
    document.documentElement.style.setProperty('--accent-blue', data.accent_color);
    // Derive translucent variants
    const hex = data.accent_color;
    document.documentElement.style.setProperty('--accent-blue-bg', hex + '1a');
    document.documentElement.style.setProperty('--accent-blue-border', hex + '33');
  }

  // Card animations
  if ('card_animations' in data) {
    body.classList.toggle('no-animations', !data.card_animations);
  }

  if ('theme' in data) {
    const themeId = _resolveThemeId(data.theme);
    if (typeof window.applyBoardTheme === 'function') {
      window.applyBoardTheme(themeId);
    } else {
      body.dataset.boardTheme = themeId;
    }
    _setSelectedAppearanceTheme(themeId);
  }

  // Density
  if (data.density) {
    body.classList.remove('density-compact', 'density-comfortable');
    if (data.density !== 'default') {
      body.classList.add('density-' + data.density);
    }
  }

  // Font size
  if (data.font_size) {
    body.classList.remove('font-small', 'font-large');
    if (data.font_size !== 'medium') {
      body.classList.add('font-' + data.font_size);
    }
  }

  // Show/hide card elements
  if ('show_card_ids' in data) {
    body.classList.toggle('hide-card-ids', !data.show_card_ids);
  }
  if ('show_avatars' in data) {
    body.classList.toggle('hide-avatars', !data.show_avatars);
  }
  if ('show_priority_icons' in data) {
    body.classList.toggle('hide-priority', !data.show_priority_icons);
  }
  if ('lane_opacity' in data) {
    _applyLaneOpacity(data.lane_opacity);
  }
  if ('card_opacity' in data) {
    _applyCardOpacity(data.card_opacity);
  }
  if ('surface_blur' in data) {
    _applySurfaceBlur(data.surface_blur);
  }
}

// Load and apply appearance on page load
async function initAppearance() {
  try {
    const data = _normalizeAppearanceSettings(await window.API.getSettings('appearance'));
    _settingsCache['appearance'] = data;
    applyAppearanceSettings(data);
  } catch (e) {
    // defaults are fine
  }
}

// ── Settings API Integration ──

let _settingsCache = {};
let _settingsDebounce = {};
let _settingsDropdowns = {};

async function loadSettings(category) {
  try {
    const raw = await window.API.getSettings(category);
    const data = category === 'appearance' ? _normalizeAppearanceSettings(raw) : raw;
    _settingsCache[category] = data;
    populateSettingsForm(category, data);
    if (category === 'appearance') applyAppearanceSettings(data);
    if (category === 'general') applyGeneralSettings(data);
  } catch (e) {
    // Use defaults from HTML
  }
}

function applyGeneralSettings(data) {
  try { localStorage.setItem('lwts-general', JSON.stringify(data)); } catch(e) {}
  if (data.compact_cards) {
    document.getElementById('board').classList.add('compact-cards');
  } else {
    document.getElementById('board').classList.remove('compact-cards');
  }
  applyRegistrationSetting(!!data.allow_registration);
}

function applyRegistrationSetting(allowed) {
  // Store in localStorage for login page to read
  try { localStorage.setItem('lwts-allow-registration', allowed ? '1' : '0'); } catch(e) {}
}

function populateSettingsForm(category, data) {
  document.querySelectorAll('[data-setting^="' + category + '."]').forEach(el => {
    const key = el.dataset.setting.split('.')[1];
    if (!(key in data)) return;
    if (el.type === 'checkbox') {
      el.checked = !!data[key];
    } else {
      el.value = data[key];
    }
  });
  // Update FnDropdown instances
  if (category === 'general') {
    if (data.session_length_days && _settingsDropdowns.session_length_days) {
      _settingsDropdowns.session_length_days.setValue(String(data.session_length_days), true);
    }
  }
  if (category === 'appearance') {
    _renderAppearanceThemePicker(data.theme);
    if (data.density && _settingsDropdowns.density) {
      _settingsDropdowns.density.setValue(data.density, true);
    }
    if (data.font_size && _settingsDropdowns.font_size) {
      _settingsDropdowns.font_size.setValue(data.font_size, true);
    }
    _applyLaneOpacity(data.lane_opacity);
    _applyCardOpacity(data.card_opacity);
    _applySurfaceBlur(data.surface_blur);
  }
}

function initSettingsDropdowns() {
  const densityEl = document.getElementById('settings-density-dropdown');
  if (densityEl && (!_settingsDropdowns.density || !densityEl.querySelector('.fn-dropdown-trigger'))) {
    _settingsDropdowns.density = null;
    _settingsDropdowns.density = new window.FnDropdown(densityEl, {
      options: [
        { value: 'comfortable', label: 'Comfortable' },
        { value: 'default', label: 'Default' },
        { value: 'compact', label: 'Compact' }
      ],
      value: 'default',
      compact: true,
      onChange: (val) => _onSettingsDropdownChange('appearance', 'density', val)
    });
  }
  const sessionEl = document.getElementById('settings-session-length-dropdown');
  if (sessionEl && (!_settingsDropdowns.session_length_days || !sessionEl.querySelector('.fn-dropdown-trigger'))) {
    _settingsDropdowns.session_length_days = null;
    _settingsDropdowns.session_length_days = new window.FnDropdown(sessionEl, {
      options: [
        { value: '1', label: '1 day' },
        { value: '7', label: '7 days' },
        { value: '30', label: '30 days' },
        { value: '365', label: '1 year' }
      ],
      value: '7',
      compact: true,
      onChange: (val) => _onSettingsDropdownChange('general', 'session_length_days', parseInt(val, 10))
    });
  }
  const fontEl = document.getElementById('settings-fontsize-dropdown');
  if (fontEl && (!_settingsDropdowns.font_size || !fontEl.querySelector('.fn-dropdown-trigger'))) {
    _settingsDropdowns.font_size = null;
    _settingsDropdowns.font_size = new window.FnDropdown(fontEl, {
      options: [
        { value: 'small', label: 'Small' },
        { value: 'medium', label: 'Medium' },
        { value: 'large', label: 'Large' }
      ],
      value: 'medium',
      compact: true,
      onChange: (val) => _onSettingsDropdownChange('appearance', 'font_size', val)
    });
  }
}

function _onSettingsDropdownChange(category, key, value) {
  if (!_settingsCache[category]) _settingsCache[category] = {};
  _settingsCache[category][key] = value;
  if (category === 'appearance') {
    applyAppearanceSettings({ [key]: value });
  }
  clearTimeout(_settingsDebounce[category]);
  _settingsDebounce[category] = setTimeout(() => {
    window.API.putSettings(category, { [key]: value }).catch(() => {});
  }, 500);
}

function initSettingsBindings() {
  document.querySelectorAll('[data-setting]').forEach(el => {
    const [category, key] = el.dataset.setting.split('.');
    const event = el.type === 'checkbox' ? 'change' : 'input';
    if (el.type === 'range') _updateSettingsRangeFill(el);
    el.addEventListener(event, () => {
      const value = el.type === 'checkbox' ? el.checked : el.value;
      if (el.type === 'range') _updateSettingsRangeFill(el);
      // Update local cache immediately
      if (!_settingsCache[category]) _settingsCache[category] = {};
      _settingsCache[category][key] = value;
      // Apply appearance changes immediately
      if (category === 'appearance') {
        applyAppearanceSettings({ [key]: value });
      }
      // Live-update header when workspace name changes
      if (category === 'general' && key === 'workspace_name') {
        const headerTitle = document.querySelector('.header-title');
        if (headerTitle) headerTitle.textContent = el.value || 'LWTS';
      }
      // Toggle compact cards on board
      if (category === 'general' && key === 'compact_cards') {
        document.getElementById('board').classList.toggle('compact-cards', el.checked);
      }
      clearTimeout(_settingsDebounce[category]);
      _settingsDebounce[category] = setTimeout(() => {
        window.API.putSettings(category, { [key]: value }).catch(() => {});
      }, 500);
    });
  });
  _renderAppearanceThemePicker((_settingsCache.appearance || {}).theme || (typeof window.getCachedBoardThemeId === 'function' ? window.getCachedBoardThemeId() : 'default'));
}

// ── Team Section ──

const _userColors = ['#82B1FF', '#fbc02d', '#4ade80', '#fb8c00', '#e040fb', '#00bcd4'];

function _initials(name) {
  return name.split(' ').map(w => w[0]).join('').toUpperCase().slice(0, 2);
}

async function loadTeamMembers() {
  try {
    const users = await window.API.listUsers();
    const me = window.Auth.getUser && window.Auth.getUser();
    const list = document.getElementById('settings-team-list');
    if (!list) return;
    list.innerHTML = '';

    // Separate humans and bots
    const humans = users.filter(u => !u.email || !u.email.endsWith('@bots.local'));
    const bots = users.filter(u => u.email && u.email.endsWith('@bots.local'));

    humans.forEach((u, i) => {
      const color = _userColors[i % _userColors.length];
      const isMe = me && me.id === u.id;
      const div = document.createElement('div');
      div.className = 'settings-user clickable';
      div.addEventListener('click', () => openEditUserModal(u));
      const roleLabel = u.role.charAt(0).toUpperCase() + u.role.slice(1);
      const roleBadge = '<span class="settings-user-role-badge ' + u.role + '">' + roleLabel + '</span>';
      const avatarInner = u.avatar_url
        ? `<img src="${u.avatar_url}" alt="${_initials(u.name)}">`
        : _initials(u.name);
      div.innerHTML = `
        <div class="settings-user-avatar-wrap">
          <div class="settings-user-avatar" style="background:${u.avatar_url ? 'transparent' : color + '30'};color:${color};border-color:${color}40">${avatarInner}</div>
        </div>
        <div class="settings-user-info"><div class="settings-user-name">${u.name}${isMe ? ' (you)' : ''}</div><div class="settings-user-role">${u.email}</div></div>
        <div class="settings-user-actions">${roleBadge}</div>`;
      list.appendChild(div);
    });

    // Bots subsection
    _renderBotsSection(bots, list);
  } catch (e) {
    // silently fail
  }
}

function _renderBotsSection(bots, container) {
  // Group header with right-aligned button
  const header = document.createElement('div');
  header.className = 'settings-group-title';
  header.style.cssText = 'margin-top:24px;display:flex;align-items:center;justify-content:space-between';
  header.innerHTML = '<span>Bots</span>' +
    (_isAdmin() ? '<button class="btn settings-action-btn" onclick="openCreateBotModal()">+ Create bot</button>' : '');
  container.appendChild(header);

  if (bots.length === 0) {
    const empty = document.createElement('div');
    empty.style.cssText = 'color:var(--text-dimmed);font-size:0.85rem;padding:8px 0';
    empty.textContent = 'No bots yet';
    container.appendChild(empty);
  } else {
    bots.forEach((bot, i) => {
      const color = '#9ca3af';
      const div = document.createElement('div');
      div.className = 'settings-user clickable';
      div.addEventListener('click', () => openEditBotModal(bot));
      const roleLabel = bot.role.charAt(0).toUpperCase() + bot.role.slice(1);
      const roleBadge = '<span class="settings-user-role-badge ' + bot.role + '">' + roleLabel + '</span>';
      div.innerHTML = `
        <div class="settings-user-avatar-wrap">
          <div class="settings-user-avatar" style="background:${color}30;color:${color};border-color:${color}40">
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="11" width="18" height="10" rx="2"/><circle cx="12" cy="5" r="2"/><line x1="12" y1="7" x2="12" y2="11"/><circle cx="8" cy="16" r="1"/><circle cx="16" cy="16" r="1"/></svg>
          </div>
        </div>
        <div class="settings-user-info">
          <div class="settings-user-name">${_escHtml(bot.name)}<span class="settings-user-bot-badge">BOT</span></div>
          <div class="settings-user-bot-note">Set an API key in API Keys settings</div>
        </div>
        <div class="settings-user-actions">${roleBadge}</div>`;
      container.appendChild(div);
    });
  }

}

async function updateUserRole(userId, role) {
  try {
    await window.API.updateUserRole(userId, role.toLowerCase());
    window.Toast.success('Role updated');
  } catch (e) {
    window.Toast.error(e.message);
    loadTeamMembers();
  }
}

async function removeTeamMember(userId, name) {
  const ok = await fnConfirm('Remove ' + name + ' from the team?', 'Remove member', 'Remove');
  if (!ok) return;
  try {
    await window.API.deleteUser(userId);
    window.Toast.success(name + ' removed');
    loadTeamMembers();
  } catch (e) {
    window.Toast.error(e.message);
  }
}

async function sendInvite() {
  const input = document.getElementById('invite-email-input');
  const email = input.value.trim();
  if (!email) { window.Toast.error('Email is required'); return; }
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) { window.Toast.error('Enter a valid email address'); return; }
  try {
    const result = await window.API.createInvite(email, 'member');
    input.value = '';
    window.Toast.success('Invite sent — ' + result.invite_url);
  } catch (e) {
    window.Toast.error(e.message);
  }
}

// ── API Keys Section ──

async function loadAPIKeys() {
  if (await _ensureLambdaDemoMode()) {
    _applyLambdaDemoRestrictions();
    const list = document.getElementById('settings-api-keys-list');
    if (list) {
      list.innerHTML = '<div style="color:var(--text-dimmed);font-size:0.85rem;padding:8px 0">Disabled in Lambda demo mode</div>';
    }
    return;
  }

  try {
    const keys = await window.API.listKeys();
    const list = document.getElementById('settings-api-keys-list');
    if (!list) return;
    list.innerHTML = '';
    if (keys.length === 0) {
      list.innerHTML = '<div style="color:var(--text-dimmed);font-size:0.85rem;padding:8px 0">No API keys yet</div>';
      return;
    }
    keys.forEach(k => {
      const card = document.createElement('div');
      card.className = 'settings-board-card';
      const keyId = k.id;
      card.innerHTML = `
        <div class="settings-board-header">
          <div style="display:flex;align-items:center;gap:8px;min-width:0;flex:1">
            <code style="font-family:'SF Mono',Monaco,Consolas,monospace;font-size:0.82rem;color:var(--text-secondary);white-space:nowrap;overflow:hidden;text-overflow:ellipsis">${k.key_masked}</code>
            <button class="lwts-modal-btn-cancel settings-api-key-action-btn" onclick="copyAPIKeyToClipboard('${keyId}')" title="Copy key">Copy</button>
          </div>
          <div class="settings-board-meta" style="display:flex;align-items:center;gap:8px;flex-shrink:0">
            ${k.name}
            <button class="settings-btn-danger" style="font-size:0.75rem;height:26px;padding:0 10px" onclick="revokeAPIKey('${keyId}')">Revoke</button>
          </div>
        </div>`;
      list.appendChild(card);
    });
  } catch (e) {
    // silently fail
  }
}

async function copyAPIKeyToClipboard(keyId) {
  if (await _ensureLambdaDemoMode()) {
    window.Toast.info('API keys are disabled in Lambda demo mode');
    return;
  }

  try {
    const result = await window.API.revealKey(keyId);
    await navigator.clipboard.writeText(result.key);
    window.Toast.success('Key copied to clipboard');
  } catch (e) {
    window.Toast.error('Failed to copy key');
  }
}

async function openCreateKeyModal() {
  if (await _ensureLambdaDemoMode()) {
    window.Toast.info('API keys are disabled in Lambda demo mode');
    return;
  }

  document.getElementById('create-key-name').value = '';
  document.getElementById('created-key-result').style.display = 'none';
  document.getElementById('create-key-name').closest('.form-group').style.display = '';
  document.getElementById('key-modal-submit').style.display = '';
  document.getElementById('key-modal-title').textContent = 'Create API key';

  // User selector for admins
  const selectorGroup = document.getElementById('key-user-selector-group');
  const selector = document.getElementById('create-key-user-id');
  if (_isAdmin() && selectorGroup && selector) {
    selectorGroup.style.display = '';
    selector.innerHTML = '<option value="">Myself</option>';
    try {
      const users = await window.API.listUsers();
      users.forEach(u => {
        const opt = document.createElement('option');
        opt.value = u.id;
        const isBot = u.email && u.email.endsWith('@bots.local');
        opt.textContent = u.name + (isBot ? ' (bot)' : '');
        selector.appendChild(opt);
      });
    } catch (e) { /* ignore */ }
  } else if (selectorGroup) {
    selectorGroup.style.display = 'none';
  }

  document.getElementById('create-key-modal').classList.add('active');
  setTimeout(() => document.getElementById('create-key-name').focus(), 100);
}

function closeCreateKeyModal() {
  document.getElementById('create-key-modal').classList.remove('active');
}

let _createdKeyValue = '';

async function submitCreateKeyModal() {
  if (await _ensureLambdaDemoMode()) {
    window.Toast.info('API keys are disabled in Lambda demo mode');
    return;
  }

  const input = document.getElementById('create-key-name');
  const name = input.value.trim();
  if (!name) { window.Toast.error('Key name required'); return; }
  try {
    const userSelect = document.getElementById('create-key-user-id');
    const userId = (_isAdmin() && userSelect && userSelect.value) ? userSelect.value : undefined;
    const result = await window.API.createKey(name, userId);
    _createdKeyValue = result.key || '';
    // Show the created key for copying
    input.closest('.form-group').style.display = 'none';
    document.getElementById('created-key-value').textContent = _createdKeyValue;
    document.getElementById('created-key-result').style.display = '';
    document.getElementById('key-modal-submit').style.display = 'none';
    document.getElementById('key-modal-title').textContent = 'API key created';
    window.Toast.success('API key created');
    loadAPIKeys();
  } catch (e) {
    window.Toast.error(e.message);
  }
}

function copyCreatedKey() {
  if (!_createdKeyValue) return;
  navigator.clipboard.writeText(_createdKeyValue).then(() => {
    window.Toast.success('Key copied to clipboard');
  }).catch(() => {
    window.Toast.error('Failed to copy');
  });
}

async function revokeAPIKey(id) {
  if (await _ensureLambdaDemoMode()) {
    window.Toast.info('API keys are disabled in Lambda demo mode');
    return;
  }

  const ok = await fnConfirm('Revoke this API key? This cannot be undone.', 'Revoke key', 'Revoke');
  if (!ok) return;
  try {
    await window.API.deleteKey(id);
    window.Toast.success('Key revoked');
    loadAPIKeys();
  } catch (e) {
    window.Toast.error(e.message);
  }
}

// ── Import ──

function importFromJira() {
  if (_isLambdaDemoMode()) {
    window.Toast.info('Import/export is disabled in Lambda demo mode');
    return;
  }

  const input = document.createElement('input');
  input.type = 'file';
  input.accept = '.json,.csv';
  input.onchange = async (e) => {
    const file = e.target.files[0];
    if (!file) return;
    try {
      const text = await file.text();
      const data = JSON.parse(text);
      await window.API.importJira(data);
      window.Toast.success('Jira import complete');
    } catch (err) {
      window.Toast.error('Import failed: ' + err.message);
    }
  };
  input.click();
}

function importFromTrello() {
  if (_isLambdaDemoMode()) {
    window.Toast.info('Import/export is disabled in Lambda demo mode');
    return;
  }

  const input = document.createElement('input');
  input.type = 'file';
  input.accept = '.json';
  input.onchange = async (e) => {
    const file = e.target.files[0];
    if (!file) return;
    try {
      const text = await file.text();
      const data = JSON.parse(text);
      await window.API.importTrello(data);
      window.Toast.success('Trello import complete');
    } catch (err) {
      window.Toast.error('Import failed: ' + err.message);
    }
  };
  input.click();
}

// ── Export ──

async function exportData() {
  if (_isLambdaDemoMode()) {
    window.Toast.info('Import/export is disabled in Lambda demo mode');
    return;
  }

  try {
    const data = await window.API.exportData();
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url; a.download = 'lwts-export.json'; a.click();
    URL.revokeObjectURL(url);
    window.Toast.success('Export downloaded');
  } catch (e) {
    window.Toast.error(e.message);
  }
}

// ── Danger Zone ──

function openResetModal() {
  const modal = document.getElementById('reset-modal');
  const hasDemo = document.getElementById('danger-reset-btn').textContent === 'Initialize';
  document.getElementById('reset-modal-title').textContent = hasDemo ? 'Initialize Workspace' : 'Reset Workspace';
  if (window.currentUser) {
    document.getElementById('reset-admin-info').textContent =
      window.currentUser.name + ' (' + window.currentUser.email + ') will be kept. All other users will be removed. Make sure you know this account\'s password.';
  }
  modal.classList.add('active');
}

function closeResetModal() {
  document.getElementById('reset-modal').classList.remove('active');
}

async function doReset(mode) {
  closeResetModal();
  try {
    await window.API.post('/api/v1/settings/reset', { mode: mode });
    window.Toast.success(mode === 'demo' ? 'Workspace reset with demo data' : 'Workspace reset to empty');
    localStorage.removeItem('lwts-board-id');
    localStorage.removeItem('lwts-kanban');
    window.currentBoardId = null;
    window.boardList = [];
    window.state = { backlog: [], todo: [], 'in-progress': [], done: [], cleared: [] };
    // Close settings without triggering loadFromAPI (closeSettings restores board view)
    closeSettings();
    // Wait a tick for settings to close, then reload fresh
    setTimeout(function() {
      window._renderAnimateCards = true;
      window.loadFromAPI();
    }, 200);
  } catch (e) {
    window.Toast.error(e.message);
  }
}

async function updateDangerZoneLabels() {
  try {
    const res = await fetch('/api/v1/workspace-status', {
      headers: { 'Authorization': 'Bearer ' + window.Auth.getAccessToken() }
    });
    const status = await res.json();
    const title = document.getElementById('danger-reset-title');
    const desc = document.getElementById('danger-reset-desc');
    const btn = document.getElementById('danger-reset-btn');
    if (status.has_demo) {
      title.textContent = 'Initialize Workspace';
      desc.textContent = 'Clear demo data and start fresh, or reset to demo state';
      btn.textContent = 'Initialize';
    } else {
      title.textContent = 'Reset Workspace';
      desc.textContent = 'Delete all boards, cards, and settings. Your admin account is preserved.';
      btn.textContent = 'Reset';
    }
  } catch (e) { /* ignore */ }
}

async function openSettings(_fromHash) {
  await _ensureLambdaDemoMode();

  // Update danger zone labels based on workspace state
  updateDangerZoneLabels();

  // Update URL hash (unless responding to a hash change)
  if (!_fromHash) {
    window._suppressHashChange = true;
    history.pushState(null, '', '#settings' + (window.activeSettingsSection && window.activeSettingsSection !== 'general' ? '/' + window.activeSettingsSection : ''));
    window._suppressHashChange = false;
  }

  const board = document.getElementById('board');
  const settings = document.getElementById('settings-page');
  const headerLeft = document.querySelector('.header-left');
  const headerActions = document.querySelector('.header-actions');

  // Fade out board/list view
  board.classList.add('fade-out');
  const listView = document.getElementById('list-view');
  if (listView) listView.style.display = 'none';
  setTimeout(() => {
    board.style.display = 'none';
    board.classList.remove('fade-out');

    // Show settings with fade in
    settings.classList.add('active');
    requestAnimationFrame(() => {
      requestAnimationFrame(() => settings.classList.add('visible'));
    });

    // Hide board picker + board-only actions + filter bar, keep search + user menu
    const boardPicker = document.getElementById('board-picker');
    if (boardPicker) boardPicker.style.display = 'none';
    headerActions.querySelectorAll('.board-only-action').forEach(el => el.style.display = 'none');
    const filterBar = document.getElementById('filter-bar');
    if (filterBar) filterBar.style.display = 'none';

    // Add settings picker (dropdown to navigate back to boards)
    let crumb = document.getElementById('settings-breadcrumb');
    if (!crumb) {
      crumb = document.createElement('div');
      crumb.id = 'settings-breadcrumb';
      crumb.className = 'header-board-picker';
      crumb.onclick = _toggleSettingsPicker;
      crumb.innerHTML = '<span id="settings-picker-label">Settings</span>' +
        '<svg class="header-board-chevron" viewBox="0 0 24 24"><polyline points="6 9 12 15 18 9"/></svg>' +
        '<div class="header-board-menu hidden" id="settings-picker-menu"></div>';
      headerLeft.appendChild(crumb);
    }
    crumb.style.display = 'flex';
    _buildSettingsPickerMenu();

    renderSettingsNav();
    showSettingsSection(window.activeSettingsSection);
    initSettingsDropdowns();
    initSettingsBindings();
    loadSettings('general');
    loadSettings('appearance');
    loadBoardsSettings();
    loadTransitionsSettings();
    loadTeamMembers();
    loadAPIKeys();
    loadDiscordConfig();
  }, 100);
}

function _toggleSettingsPicker(e) {
  if (e && e.target.closest('#settings-picker-menu')) return;
  const menu = document.getElementById('settings-picker-menu');
  const crumb = document.getElementById('settings-breadcrumb');
  if (!menu) return;
  const isOpen = menu.classList.contains('visible');
  if (isOpen) {
    menu.classList.remove('visible');
    if (crumb) crumb.classList.remove('open');
    setTimeout(() => menu.classList.add('hidden'), 200);
  } else {
    menu.classList.remove('hidden');
    if (crumb) crumb.classList.add('open');
    requestAnimationFrame(() => menu.classList.add('visible'));
  }
}

function _buildSettingsPickerMenu() {
  const menu = document.getElementById('settings-picker-menu');
  if (!menu) return;

  function closeMenu() {
    menu.classList.remove('visible');
    setTimeout(() => menu.classList.add('hidden'), 200);
    document.getElementById('settings-breadcrumb')?.classList.remove('open');
  }

  if (typeof window._buildPickerMenu === 'function') {
    window._buildPickerMenu(menu, {
      activeId: 'settings',
      inSettings: true,
      closeMenu: closeMenu,
    });
  }
}

function closeSettings(_fromHash) {
  // If settings isn't open, nothing to do
  const _sp = document.getElementById('settings-page');
  if (_sp && !_sp.classList.contains('active')) return;

  // Clear hash (unless responding to a hash change)
  if (!_fromHash && location.hash.startsWith('#settings')) {
    window._suppressHashChange = true;
    history.pushState(null, '', location.pathname + location.search);
    window._suppressHashChange = false;
  }

  const board = document.getElementById('board');
  const settings = document.getElementById('settings-page');
  const headerLeft = document.querySelector('.header-left');
  const headerActions = document.querySelector('.header-actions');

  // Fade out settings
  settings.classList.remove('visible');
  setTimeout(() => {
    settings.classList.remove('active');

    // Show board or list view depending on current view
    const listView = document.getElementById('list-view');
    if (typeof window.currentView !== 'undefined' && window.currentView === 'list') {
      board.style.display = 'none';
      if (listView) {
        listView.style.display = '';
        if (typeof window.renderListView === 'function') window.renderListView();
      }
    } else {
      board.style.display = '';
      if (listView) listView.style.display = 'none';
      board.classList.add('fade-in');
      board.addEventListener('animationend', () => board.classList.remove('fade-in'), { once: true });
    }

    // Restore board picker + board-only actions + filter bar, hide settings breadcrumb
    const boardPicker = document.getElementById('board-picker');
    if (boardPicker) boardPicker.style.display = '';
    headerActions.querySelectorAll('.board-only-action').forEach(el => el.style.display = '');
    const filterBar = document.getElementById('filter-bar');
    if (filterBar) filterBar.style.display = '';
    const crumb = document.getElementById('settings-breadcrumb');
    if (crumb) crumb.style.display = 'none';

    // Re-hide board menu via CSS class (not inline style)
    const menu = document.getElementById('board-menu');
    if (menu) { menu.classList.add('hidden'); menu.classList.remove('visible'); }
  }, 120);
}

// ── Board Enable/Disable ──

function toggleBoardEnabled(checkbox) {
  const card = checkbox.closest('.settings-board-card');
  if (!card) return;
  if (checkbox.checked) {
    card.classList.remove('disabled');
  } else {
    card.classList.add('disabled');
  }
}

// ── Boards Settings (dynamic from API) ──

let _boardSettingsDebounce = {};

async function loadBoardsSettings() {
  const container = document.getElementById('settings-boards-list');
  if (!container) return;
  container.innerHTML = '';

  try {
    const boards = await window.API.listBoards();
    if (!boards || boards.length === 0) {
      container.innerHTML = '<div style="color:var(--text-dimmed);font-size:0.85rem;padding:8px 0">No boards yet</div>';
      return;
    }
    window.boardList = boards;
    if (typeof window.syncCurrentBoardTheme === 'function') {
      window.syncCurrentBoardTheme();
    }

    // Fetch card counts for each board in parallel
    const details = await Promise.all(boards.map(b => window.API.getBoard(b.id).catch(() => null)));

    boards.forEach((board, idx) => {
      const detail = details[idx];
      const cardCounts = detail ? detail.card_counts || {} : {};
      const totalCards = Object.values(cardCounts).reduce((s, n) => s + n, 0);
      const columns = JSON.parse(board.columns || '[]');
      const configId = 'board-config-' + board.id;
      const isDefault = idx === 0;
      const boardIcon = '<svg viewBox="0 0 24 24" style="width:16px;height:16px;stroke:var(--text-dimmed);fill:none;stroke-width:2"><rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/></svg>';

      const card = document.createElement('div');
      card.className = 'settings-board-card';

      // Header
      const header = document.createElement('div');
      header.className = 'settings-board-header';
      header.onclick = () => toggleBoardConfig(configId);
      header.innerHTML = `
        <div class="settings-board-name">
          ${boardIcon}
          ${_escHtml(board.name)}
          ${isDefault ? '<span class="settings-board-badge">default</span>' : ''}
        </div>
        <div class="settings-board-meta">${totalCards} card${totalCards !== 1 ? 's' : ''} <svg class="settings-board-chevron" viewBox="0 0 24 24"><polyline points="6 9 12 15 18 9"/></svg></div>`;
      card.appendChild(header);

      // Config panel
      const config = document.createElement('div');
      config.className = 'settings-board-config hidden';
      config.id = configId;

      // Configuration group
      const settings = window.parseBoardSettings ? window.parseBoardSettings(board.settings) : JSON.parse(board.settings || '{}');
      let configHTML = `
        <div class="settings-group">
          <div class="settings-group-title">Configuration</div>
          <div class="settings-row">
            <div class="settings-row-label"><div class="settings-row-title">Project key</div><div class="settings-row-desc">Prefix for ticket IDs (e.g. ${_escHtml(board.project_key)}-101)</div></div>
            <div class="settings-row-control"><input class="settings-input board-project-key-input" data-board-id="${board.id}" value="${_escHtml(board.project_key)}" style="width:100px" /></div>
          </div>
        </div>`;

      // Columns group with editable labels, colors, drag reorder, add/remove
      const COL_PALETTE = ['#8c8c8c','#579DFF','#fb8c00','#4ade80','#f44336','#9f8fef','#6cc3e0','#f5cd47'];
      configHTML += `
        <div class="settings-group">
          <div class="settings-group-title">Columns</div>
          <div class="settings-columns-list" data-board-id="${board.id}">`;
      columns.forEach((col, ci) => {
        const count = cardCounts[col.id] || 0;
        const color = col.color || COL_PALETTE[ci % COL_PALETTE.length];
        const isFirst = ci === 0;
        const isLast = ci === columns.length - 1;
        const typeLabel = isFirst ? 'Start' : isLast ? 'Done' : '';
        configHTML += `
            <div class="settings-column-row" data-col-index="${ci}" data-col-id="${_escHtml(col.id)}">
              <span class="settings-column-drag" draggable="true" title="Drag to reorder">&#x2261;</span>
              <input class="settings-input board-column-label-input" data-board-id="${board.id}" data-col-index="${ci}" value="${_escHtml(col.label)}" style="width:140px;height:30px;font-size:0.88rem;font-weight:500" />
              <button class="settings-column-color-btn" data-col-index="${ci}" style="background:${color}" title="Change color"></button>
              <span class="settings-column-count">${count} card${count !== 1 ? 's' : ''}${typeLabel ? ' · ' + typeLabel : ''}</span>
              ${columns.length > 2 ? '<button class="settings-column-remove-btn" data-col-index="' + ci + '" data-col-id="' + _escHtml(col.id) + '" data-card-count="' + count + '" title="Remove column">&times;</button>' : ''}
            </div>`;
      });
      configHTML += `
          </div>
          <button class="btn settings-column-add-btn" data-board-id="${board.id}" style="margin-top:10px;font-size:0.82rem;padding:4px 14px">+ Add column</button>
        </div>`;

      // Webhooks group
      const webhooks = settings.webhooks || {};
      configHTML += `
        <div class="settings-group">
          <div class="settings-group-title">Webhooks</div>
          <div class="settings-row"><div class="settings-row-label"><div class="settings-row-title">On transition</div><div class="settings-row-desc">POST when a card moves columns</div></div><div class="settings-row-control" style="display:flex;gap:8px"><input class="settings-input board-webhook-input" data-board-id="${board.id}" data-webhook="on_transition" value="${_escHtml(webhooks.on_transition || '')}" placeholder="https://..." /><button class="btn" style="font-size:0.78rem;flex-shrink:0" onclick="testWebhook(this)">Test</button></div></div>
          <div class="settings-row"><div class="settings-row-label"><div class="settings-row-title">On create</div><div class="settings-row-desc">POST when a new card is created</div></div><div class="settings-row-control" style="display:flex;gap:8px"><input class="settings-input board-webhook-input" data-board-id="${board.id}" data-webhook="on_create" value="${_escHtml(webhooks.on_create || '')}" placeholder="https://..." /><button class="btn" style="font-size:0.78rem;flex-shrink:0" onclick="testWebhook(this)">Test</button></div></div>
          <div class="settings-row"><div class="settings-row-label"><div class="settings-row-title">On complete</div><div class="settings-row-desc">POST when a card moves to Done</div></div><div class="settings-row-control" style="display:flex;gap:8px"><input class="settings-input board-webhook-input" data-board-id="${board.id}" data-webhook="on_complete" value="${_escHtml(webhooks.on_complete || '')}" placeholder="https://..." /><button class="btn" style="font-size:0.78rem;flex-shrink:0" onclick="testWebhook(this)">Test</button></div></div>
        </div>`;

      config.innerHTML = configHTML;
      card.appendChild(config);
      container.appendChild(card);
    });

    // Bind change handlers for all board inputs
    _bindBoardSettingsInputs();
  } catch (e) {
    container.innerHTML = '<div style="color:var(--text-dimmed);font-size:0.85rem;padding:8px 0">Failed to load boards</div>';
  }
}

function _escHtml(s) {
  const d = document.createElement('div');
  d.textContent = s || '';
  return d.innerHTML;
}

function _bindBoardSettingsInputs() {
  // Project key inputs
  document.querySelectorAll('.board-project-key-input').forEach(el => {
    el.addEventListener('input', () => {
      const boardId = el.dataset.boardId;
      clearTimeout(_boardSettingsDebounce['pk-' + boardId]);
      _boardSettingsDebounce['pk-' + boardId] = setTimeout(() => {
        window.API.updateBoard(boardId, { project_key: el.value.trim() }).catch(() => window.Toast.error('Failed to update project key'));
      }, 600);
    });
  });

  // Column label inputs
  document.querySelectorAll('.board-column-label-input').forEach(el => {
    el.addEventListener('input', () => {
      const boardId = el.dataset.boardId;
      clearTimeout(_boardSettingsDebounce['col-' + boardId]);
      _boardSettingsDebounce['col-' + boardId] = setTimeout(() => {
        _saveBoardColumns(boardId, { silent: true });
      }, 600);
    });
  });

  // Column color buttons
  document.querySelectorAll('.settings-column-color-btn').forEach(btn => {
    btn.addEventListener('click', (e) => {
      e.stopPropagation();
      const row = btn.closest('.settings-column-row');
      const list = btn.closest('.settings-columns-list');
      const boardId = list.dataset.boardId;
      _showColumnColorPicker(btn, boardId, parseInt(row.dataset.colIndex));
    });
  });

  // Column remove buttons
  document.querySelectorAll('.settings-column-remove-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      const list = btn.closest('.settings-columns-list');
      const boardId = list.dataset.boardId;
      const colId = btn.dataset.colId;
      const cardCount = parseInt(btn.dataset.cardCount);
      _removeColumn(boardId, colId, cardCount);
    });
  });

  // Column add buttons
  document.querySelectorAll('.settings-column-add-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      _addColumn(btn.dataset.boardId);
    });
  });

  // Column drag reorder
  document.querySelectorAll('.settings-columns-list').forEach(list => {
    _bindColumnDragReorder(list);
  });

  // Webhook inputs
  document.querySelectorAll('.board-webhook-input').forEach(el => {
    el.addEventListener('input', () => {
      const boardId = el.dataset.boardId;
      clearTimeout(_boardSettingsDebounce['wh-' + boardId]);
      _boardSettingsDebounce['wh-' + boardId] = setTimeout(() => {
        _saveBoardWebhooks(boardId);
      }, 600);
    });
  });
}

function _saveBoardColumns(boardId, opts) {
  const board = window.boardList.find(b => b.id === boardId);
  if (!board) return;
  let columns = JSON.parse(board.columns || '[]');

  // Sync labels from DOM inputs if they exist (skip after reorder since indices are stale)
  if (!(opts && opts.skipLabelSync)) {
    const inputs = document.querySelectorAll(`.board-column-label-input[data-board-id="${boardId}"]`);
    inputs.forEach(input => {
      const idx = parseInt(input.dataset.colIndex);
      if (columns[idx]) {
        columns[idx].label = input.value.trim() || columns[idx].label;
      }
    });
  }

  // Ensure types are assigned: first = start, last = done, rest = active
  columns.forEach((col, i) => {
    if (i === 0) col.type = 'start';
    else if (i === columns.length - 1) col.type = 'done';
    else col.type = 'active';
  });

  const columnsJSON = JSON.stringify(columns);
  board.columns = columnsJSON;
  const payload = { columns: columnsJSON };
  if (opts && opts.migrate_to) payload.migrate_to = opts.migrate_to;
  const silent = opts && opts.silent;

  window.API.updateBoard(boardId, payload).then(() => {
    if (boardId === window.currentBoardId && typeof window.COLUMNS !== 'undefined') {
      window.COLUMNS.length = 0;
      columns.forEach(c => window.COLUMNS.push(c));
      if (typeof window.render === 'function') window.render();
    }
    if (!silent) window.Toast.success('Columns updated');
  }).catch((err) => {
    const data = err && err.data;
    if (data && data.column_id) {
      window.Toast.error('Column "' + data.column_id + '" still has ' + data.card_count + ' cards');
    } else {
      window.Toast.error(data && data.error ? data.error : 'Failed to update columns');
    }
  });
}

// Re-render just the column rows inside an already-open config panel
function _rerenderColumnList(boardId) {
  const list = document.querySelector(`.settings-columns-list[data-board-id="${boardId}"]`);
  if (!list) return;
  const board = window.boardList.find(b => b.id === boardId);
  if (!board) return;
  const columns = JSON.parse(board.columns || '[]');

  // Fetch card counts from the existing rendered counts (avoid extra API call)
  const cardCounts = {};
  list.querySelectorAll('.settings-column-row').forEach(row => {
    const colId = row.dataset.colId;
    const countEl = row.querySelector('.settings-column-count');
    if (countEl && colId) {
      const m = countEl.textContent.match(/^(\d+)/);
      if (m) cardCounts[colId] = parseInt(m[1]);
    }
  });

  const COL_PALETTE = ['#8c8c8c','#579DFF','#fb8c00','#4ade80','#f44336','#9f8fef','#6cc3e0','#f5cd47'];
  let html = '';
  columns.forEach((col, ci) => {
    const count = cardCounts[col.id] || 0;
    const color = col.color || COL_PALETTE[ci % COL_PALETTE.length];
    const isFirst = ci === 0;
    const isLast = ci === columns.length - 1;
    const typeLabel = isFirst ? 'Start' : isLast ? 'Done' : '';
    html += `
      <div class="settings-column-row" data-col-index="${ci}" data-col-id="${_escHtml(col.id)}">
        <span class="settings-column-drag" draggable="true" title="Drag to reorder">&#x2261;</span>
        <input class="settings-input board-column-label-input" data-board-id="${boardId}" data-col-index="${ci}" value="${_escHtml(col.label)}" style="width:140px;height:30px;font-size:0.88rem;font-weight:500" />
        <button class="settings-column-color-btn" data-col-index="${ci}" style="background:${color}" title="Change color"></button>
        <span class="settings-column-count">${count} card${count !== 1 ? 's' : ''}${typeLabel ? ' · ' + typeLabel : ''}</span>
        ${columns.length > 2 ? '<button class="settings-column-remove-btn" data-col-index="' + ci + '" data-col-id="' + _escHtml(col.id) + '" data-card-count="' + count + '" title="Remove column">&times;</button>' : ''}
      </div>`;
  });
  list.innerHTML = html;

  // Re-bind events for the new elements inside this list
  list.querySelectorAll('.board-column-label-input').forEach(el => {
    el.addEventListener('input', () => {
      clearTimeout(_boardSettingsDebounce['col-' + boardId]);
      _boardSettingsDebounce['col-' + boardId] = setTimeout(() => {
        _saveBoardColumns(boardId, { silent: true });
      }, 600);
    });
  });
  list.querySelectorAll('.settings-column-color-btn').forEach(btn => {
    btn.addEventListener('click', (e) => {
      e.stopPropagation();
      const row = btn.closest('.settings-column-row');
      _showColumnColorPicker(btn, boardId, parseInt(row.dataset.colIndex));
    });
  });
  list.querySelectorAll('.settings-column-remove-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      _removeColumn(boardId, btn.dataset.colId, parseInt(btn.dataset.cardCount));
    });
  });
  _bindColumnDragReorder(list);
}

function _slugifyColumnId(label) {
  let slug = label.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
  if (!slug) slug = 'column';
  return slug;
}

function _uniqueColumnId(slug, columns) {
  const ids = new Set(columns.map(c => c.id));
  if (!ids.has(slug)) return slug;
  let n = 2;
  while (ids.has(slug + '-' + n)) n++;
  return slug + '-' + n;
}

function _addColumn(boardId) {
  const board = window.boardList.find(b => b.id === boardId);
  if (!board) return;
  const columns = JSON.parse(board.columns || '[]');
  const COL_PALETTE = ['#8c8c8c','#579DFF','#fb8c00','#4ade80','#f44336','#9f8fef','#6cc3e0','#f5cd47'];
  const label = 'New Column';
  const id = _uniqueColumnId(_slugifyColumnId(label), columns);
  columns.push({ id, label, color: COL_PALETTE[columns.length % COL_PALETTE.length], type: 'active' });
  board.columns = JSON.stringify(columns);
  _saveBoardColumns(boardId, { silent: true });
  _rerenderColumnList(boardId);
  // Focus the new column's label input
  setTimeout(() => {
    const inputs = document.querySelectorAll(`.board-column-label-input[data-board-id="${boardId}"]`);
    const last = inputs[inputs.length - 1];
    if (last) { last.focus(); last.select(); }
  }, 50);
}

function _removeColumn(boardId, colId, cardCount) {
  const board = window.boardList.find(b => b.id === boardId);
  if (!board) return;
  const columns = JSON.parse(board.columns || '[]');
  if (columns.length <= 2) {
    window.Toast.error('Board must have at least 2 columns');
    return;
  }

  const doRemove = (migrateTo) => {
    const newCols = columns.filter(c => c.id !== colId);
    board.columns = JSON.stringify(newCols);
    const opts = { silent: true };
    if (migrateTo) opts.migrate_to = migrateTo;
    _saveBoardColumns(boardId, opts);
    _rerenderColumnList(boardId);
  };

  if (cardCount > 0) {
    // Show a simple confirmation with migration target
    const otherCols = columns.filter(c => c.id !== colId);
    const target = otherCols[0]; // migrate to first available column
    if (confirm('Column has ' + cardCount + ' card(s). Move them to "' + target.label + '" and remove?')) {
      doRemove(target.id);
    }
  } else {
    doRemove(null);
  }
}

function _showColumnColorPicker(btn, boardId, colIndex) {
  // Remove any existing picker
  document.querySelectorAll('.settings-color-picker').forEach(p => p.remove());

  const COL_PALETTE = ['#8c8c8c','#579DFF','#fb8c00','#4ade80','#f44336','#9f8fef','#6cc3e0','#f5cd47'];
  const picker = document.createElement('div');
  picker.className = 'settings-color-picker';
  picker.style.cssText = 'position:absolute;display:flex;gap:6px;padding:8px;background:var(--surface-dropdown,#222);border:1px solid var(--border-light);border-radius:8px;box-shadow:var(--shadow-md);z-index:200';

  COL_PALETTE.forEach(color => {
    const swatch = document.createElement('button');
    swatch.style.cssText = 'width:22px;height:22px;border-radius:50%;border:2px solid transparent;cursor:pointer;background:' + color;
    swatch.addEventListener('click', () => {
      btn.style.background = color;
      picker.remove();
      const board = window.boardList.find(b => b.id === boardId);
      if (!board) return;
      const columns = JSON.parse(board.columns || '[]');
      if (columns[colIndex]) {
        columns[colIndex].color = color;
        board.columns = JSON.stringify(columns);
        _saveBoardColumns(boardId);
      }
    });
    picker.appendChild(swatch);
  });

  btn.style.position = 'relative';
  btn.parentElement.style.position = 'relative';
  btn.parentElement.appendChild(picker);
  picker.style.top = btn.offsetTop + btn.offsetHeight + 4 + 'px';
  picker.style.left = btn.offsetLeft + 'px';

  // Close on outside click
  const close = (e) => { if (!picker.contains(e.target) && e.target !== btn) { picker.remove(); document.removeEventListener('click', close); } };
  setTimeout(() => document.addEventListener('click', close), 0);
}

function _bindColumnDragReorder(list) {
  let dragIdx = null;
  let dragRow = null;

  // Drag starts from the handle, but we track the parent row
  list.querySelectorAll('.settings-column-drag').forEach(handle => {
    handle.addEventListener('dragstart', (e) => {
      dragRow = handle.closest('.settings-column-row');
      dragIdx = parseInt(dragRow.dataset.colIndex);
      dragRow.style.opacity = '0.4';
      e.dataTransfer.effectAllowed = 'move';
      e.dataTransfer.setDragImage(dragRow, 0, 0);
    });
    handle.addEventListener('dragend', () => {
      if (dragRow) dragRow.style.opacity = '';
      dragRow = null;
      dragIdx = null;
    });
  });

  // Drop targets are the rows
  list.querySelectorAll('.settings-column-row').forEach(row => {
    row.addEventListener('dragover', (e) => {
      e.preventDefault();
      e.dataTransfer.dropEffect = 'move';
      row.style.boxShadow = 'inset 0 2px 0 0 rgba(100,100,100,0.6)';
    });
    row.addEventListener('dragleave', () => { row.style.boxShadow = ''; });
    row.addEventListener('drop', (e) => {
      e.preventDefault();
      row.style.boxShadow = '';
      const dropIdx = parseInt(row.dataset.colIndex);
      if (dragIdx === null || dragIdx === dropIdx) return;
      const boardId = list.dataset.boardId;
      const board = window.boardList.find(b => b.id === boardId);
      if (!board) return;
      const columns = JSON.parse(board.columns || '[]');
      const [moved] = columns.splice(dragIdx, 1);
      columns.splice(dropIdx, 0, moved);
      board.columns = JSON.stringify(columns);
      _rerenderColumnList(boardId);
      _saveBoardColumns(boardId, { silent: true, skipLabelSync: true });
    });
  });
}

function _saveBoardWebhooks(boardId) {
  const inputs = document.querySelectorAll(`.board-webhook-input[data-board-id="${boardId}"]`);
  const board = window.boardList.find(b => b.id === boardId);
  if (!board) return;
  const settings = window.parseBoardSettings ? window.parseBoardSettings(board.settings) : JSON.parse(board.settings || '{}');
  if (!settings.webhooks) settings.webhooks = {};
  inputs.forEach(input => {
    settings.webhooks[input.dataset.webhook] = input.value.trim();
  });
  board.settings = JSON.stringify(settings);
  window.API.updateBoard(boardId, { settings: JSON.stringify(settings) }).catch(() => window.Toast.error('Failed to update webhooks'));
}

// ── Transition Rules ──

const TRANSITION_RULES = [
  {
    id: 'no_blocked_to_done',
    label: 'Blocked tickets cannot be closed',
    desc: 'Prevent moving a card to Done if it has blocking dependencies',
    icon: '<svg viewBox="0 0 24 24" width="18" height="18" stroke="currentColor" fill="none" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="4.93" y1="4.93" x2="19.07" y2="19.07"/></svg>',
  },
  {
    id: 'require_comment_done',
    label: 'Require a comment to close',
    desc: 'At least one comment must exist before moving to Done',
    icon: '<svg viewBox="0 0 24 24" width="18" height="18" stroke="currentColor" fill="none" stroke-width="2"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>',
  },
  {
    id: 'require_assignee_prog',
    label: 'Require assignee to start',
    desc: 'A card must have an assignee before moving to In Progress',
    icon: '<svg viewBox="0 0 24 24" width="18" height="18" stroke="currentColor" fill="none" stroke-width="2"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>',
  },
  {
    id: 'require_desc_done',
    label: 'Require description to close',
    desc: 'Card must have a description before moving to Done',
    icon: '<svg viewBox="0 0 24 24" width="18" height="18" stroke="currentColor" fill="none" stroke-width="2"><line x1="17" y1="10" x2="3" y2="10"/><line x1="21" y1="6" x2="3" y2="6"/><line x1="21" y1="14" x2="3" y2="14"/><line x1="17" y1="18" x2="3" y2="18"/></svg>',
  },
  {
    id: 'no_done_backward',
    label: 'Prevent reopening closed tickets',
    desc: 'Cards in Done cannot be moved back to earlier columns',
    icon: '<svg viewBox="0 0 24 24" width="18" height="18" stroke="currentColor" fill="none" stroke-width="2"><polyline points="9 10 4 15 9 20"/><path d="M20 4v7a4 4 0 0 1-4 4H4"/></svg>',
  },
];

async function loadTransitionsSettings() {
  const container = document.getElementById('settings-transitions-content');
  if (!container) return;
  container.innerHTML = '';

  try {
    const boards = await window.API.listBoards();
    if (!boards || boards.length === 0) {
      container.innerHTML = '<div style="color:var(--text-dimmed);font-size:0.85rem;padding:8px 0">No boards yet</div>';
      return;
    }

    // Board selector if multiple boards
    let selectedBoardId = window.currentBoardId || boards[0].id;
    const selectedBoard = boards.find(b => b.id === selectedBoardId) || boards[0];
    selectedBoardId = selectedBoard.id;

    if (boards.length > 1) {
      const selector = document.createElement('div');
      selector.className = 'settings-group';
      selector.innerHTML = `
        <div class="settings-row">
          <div class="settings-row-label"><div class="settings-row-title">Board</div></div>
          <div class="settings-row-control">
            <select class="settings-input" id="transitions-board-select" style="width:200px;height:34px">
              ${boards.map(b => `<option value="${b.id}" ${b.id === selectedBoardId ? 'selected' : ''}>${_escHtml(b.name)}</option>`).join('')}
            </select>
          </div>
        </div>`;
      container.appendChild(selector);
      selector.querySelector('#transitions-board-select').addEventListener('change', function() {
        _renderTransitionRules(this.value, boards);
      });
    }

    const rulesContainer = document.createElement('div');
    rulesContainer.id = 'transitions-rules-list';
    container.appendChild(rulesContainer);

    _renderTransitionRules(selectedBoardId, boards);
  } catch (e) {
    container.innerHTML = '<div style="color:var(--text-dimmed);font-size:0.85rem;padding:8px 0">Failed to load boards</div>';
  }
}

function _renderTransitionRules(boardId, boards) {
  const container = document.getElementById('transitions-rules-list');
  if (!container) return;

  const board = boards.find(b => b.id === boardId);
  if (!board) return;

  const settings = JSON.parse(board.settings || '{}');
  const rules = settings.transition_rules || {};

  let html = '<div class="settings-group"><div class="settings-group-title">Rules</div>';

  TRANSITION_RULES.forEach(rule => {
    const enabled = !!rules[rule.id];
    html += `
      <div class="settings-row transition-rule-row">
        <div class="settings-row-label" style="display:flex;align-items:flex-start;gap:12px">
          <span class="transition-rule-icon">${rule.icon}</span>
          <div>
            <div class="settings-row-title">${rule.label}</div>
            <div class="settings-row-desc">${rule.desc}</div>
          </div>
        </div>
        <div class="settings-row-control">
          <label class="settings-toggle">
            <input type="checkbox" class="transition-rule-toggle" data-board-id="${boardId}" data-rule-id="${rule.id}" ${enabled ? 'checked' : ''} />
            <span class="toggle-track"></span>
          </label>
        </div>
      </div>`;
  });

  html += '</div>';
  container.innerHTML = html;

  // Bind toggles
  container.querySelectorAll('.transition-rule-toggle').forEach(toggle => {
    toggle.addEventListener('change', () => {
      _saveTransitionRule(toggle.dataset.boardId, toggle.dataset.ruleId, toggle.checked, boards);
    });
  });
}

function _saveTransitionRule(boardId, ruleId, enabled, boards) {
  const board = boards.find(b => b.id === boardId);
  if (!board) return;

  const settings = JSON.parse(board.settings || '{}');
  if (!settings.transition_rules) settings.transition_rules = {};
  settings.transition_rules[ruleId] = enabled;

  // Clean up false values
  if (!enabled) delete settings.transition_rules[ruleId];
  if (Object.keys(settings.transition_rules).length === 0) delete settings.transition_rules;

  board.settings = JSON.stringify(settings);
  window.API.updateBoard(boardId, { settings: JSON.stringify(settings) })
    .then(() => window.Toast.success('Transition rule updated'))
    .catch(() => window.Toast.error('Failed to update rule'));
}

// ── Webhook Test ──

async function testWebhook(btn) {
  const control = btn.closest('.settings-row-control');
  const input = control ? control.querySelector('.settings-input') : null;
  const url = input ? input.value.trim() : '';
  if (!url) {
    window.Toast.error('Enter a webhook URL first');
    return;
  }
  btn.disabled = true;
  btn.textContent = '...';
  try {
    const resp = await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ event: 'test', timestamp: new Date().toISOString() }),
    });
    if (resp.ok) {
      window.Toast.success('Webhook responded ' + resp.status);
    } else {
      window.Toast.error('Webhook returned ' + resp.status);
    }
  } catch (e) {
    window.Toast.error('Webhook failed: ' + e.message);
  } finally {
    btn.disabled = false;
    btn.textContent = 'Test';
  }
}

// ── Avatar Picker ──

const DEFAULT_AVATARS = (function() {
  const colors = ['#82B1FF', '#fbc02d', '#4ade80', '#fb8c00', '#579DFF', '#ce93d8', '#4dd0e1', '#2B7DE9'];
  const patterns = ['circle', 'ring', 'diamond', 'stripe'];
  const avatars = [];
  for (let i = 0; i < 8; i++) {
    const c = colors[i];
    const canvas = document.createElement('canvas');
    canvas.width = 64; canvas.height = 64;
    const ctx = canvas.getContext('2d');
    const pat = patterns[i % patterns.length];
    // Background
    ctx.fillStyle = c + '30';
    ctx.fillRect(0, 0, 64, 64);
    ctx.fillStyle = c;
    if (pat === 'circle') {
      ctx.beginPath(); ctx.arc(32, 32, 20, 0, Math.PI * 2); ctx.fill();
    } else if (pat === 'ring') {
      ctx.lineWidth = 6; ctx.strokeStyle = c;
      ctx.beginPath(); ctx.arc(32, 32, 18, 0, Math.PI * 2); ctx.stroke();
      ctx.beginPath(); ctx.arc(32, 32, 8, 0, Math.PI * 2); ctx.fill();
    } else if (pat === 'diamond') {
      ctx.beginPath(); ctx.moveTo(32, 8); ctx.lineTo(56, 32); ctx.lineTo(32, 56); ctx.lineTo(8, 32); ctx.closePath(); ctx.fill();
    } else if (pat === 'stripe') {
      for (let s = 0; s < 4; s++) {
        ctx.fillRect(8 + s * 14, 16, 8, 32);
      }
    }
    avatars.push(canvas.toDataURL('image/png'));
  }
  return avatars;
})();

function openAvatarPicker(userId, btnEl) {
  // Close any existing picker
  closeAvatarPicker();

  const popover = document.createElement('div');
  popover.className = 'avatar-picker-popover';
  popover.id = 'avatar-picker-popover';

  popover.innerHTML = `
    <div class="avatar-picker-title">Choose Avatar</div>
    <button class="avatar-picker-upload-btn" onclick="uploadAvatarFile('${userId}')">
      <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/></svg>
      Upload from file
    </button>
    <div class="avatar-picker-divider"></div>
    <div class="avatar-picker-grid">
      ${DEFAULT_AVATARS.map((url, i) => `<div class="avatar-picker-option" onclick="selectDefaultAvatar('${userId}', ${i})"><img src="${url}"></div>`).join('')}
    </div>
    <div class="avatar-picker-divider"></div>
    <button class="avatar-picker-remove-btn" onclick="removeAvatar('${userId}')">Remove avatar</button>
  `;

  // Position near the button
  const wrap = btnEl.closest('.settings-user-avatar-wrap') || btnEl.parentElement;
  wrap.style.position = 'relative';
  wrap.appendChild(popover);

  // Close on outside click
  setTimeout(() => {
    document.addEventListener('click', _avatarPickerOutsideClick);
  }, 10);
}

function _avatarPickerOutsideClick(e) {
  const popover = document.getElementById('avatar-picker-popover');
  if (popover && !popover.contains(e.target) && !e.target.closest('.settings-user-avatar')) {
    closeAvatarPicker();
  }
}

function closeAvatarPicker() {
  const popover = document.getElementById('avatar-picker-popover');
  if (popover) popover.remove();
  document.removeEventListener('click', _avatarPickerOutsideClick);
}

function uploadAvatarFile(userId) {
  const input = document.createElement('input');
  input.type = 'file';
  input.accept = 'image/*';
  input.onchange = async (e) => {
    const file = e.target.files[0];
    if (!file) return;
    // Resize and convert to data URL
    const dataUrl = await resizeImageToDataURL(file, 128);
    await saveAvatar(userId, dataUrl);
  };
  input.click();
}

function selectDefaultAvatar(userId, idx) {
  saveAvatar(userId, DEFAULT_AVATARS[idx]);
}

function removeAvatar(userId) {
  saveAvatar(userId, '');
}

async function saveAvatar(userId, dataUrl) {
  closeAvatarPicker();
  try {
    await window.API.updateUserAvatar(userId, dataUrl);
    window.Toast.success('Avatar updated');
    loadTeamMembers();
    // Refresh user list in kanban if available
    if (typeof window.loadUsers === 'function') window.loadUsers();
  } catch (e) {
    window.Toast.error('Failed to update avatar: ' + e.message);
  }
}

function resizeImageToDataURL(file, maxSize) {
  return new Promise((resolve) => {
    const reader = new FileReader();
    reader.onload = (e) => {
      const img = new Image();
      img.onload = () => {
        const canvas = document.createElement('canvas');
        let w = img.width, h = img.height;
        if (w > h) { h = maxSize * h / w; w = maxSize; }
        else { w = maxSize * w / h; h = maxSize; }
        canvas.width = w; canvas.height = h;
        const ctx = canvas.getContext('2d');
        ctx.drawImage(img, 0, 0, w, h);
        resolve(canvas.toDataURL('image/png'));
      };
      img.src = e.target.result;
    };
    reader.readAsDataURL(file);
  });
}

// ── Expose all public symbols to window for HTML onclick handlers and cross-file access ──
window._suppressHashChange = _suppressHashChange;
window.fnConfirm = fnConfirm;
window.fnConfirmResolve = fnConfirmResolve;
window._fnConfirmResolve = _fnConfirmResolve;
window._userModalRoleDropdown = _userModalRoleDropdown;
window._userModalEditId = _userModalEditId;
window._initUserRoleDropdown = _initUserRoleDropdown;
window.openCreateUserModal = openCreateUserModal;
window.openCreateBotModal = openCreateBotModal;
window.openEditBotModal = openEditBotModal;
window.openEditUserModal = openEditUserModal;
window.closeCreateUserModal = closeCreateUserModal;
window.submitUserModal = submitUserModal;
window.deleteUserFromModal = deleteUserFromModal;
window.SETTINGS_SECTIONS = SETTINGS_SECTIONS;
window.SETTINGS_GROUPS = SETTINGS_GROUPS;
window.activeSettingsSection = activeSettingsSection;
window.toggleBoardConfig = toggleBoardConfig;
window.renderSettingsNav = renderSettingsNav;
window.showSettingsSection = showSettingsSection;
window.applyAppearanceSettings = applyAppearanceSettings;
window.initAppearance = initAppearance;
window._settingsCache = _settingsCache;
window.loadSettings = loadSettings;
window.applyGeneralSettings = applyGeneralSettings;
window.applyRegistrationSetting = applyRegistrationSetting;
window.populateSettingsForm = populateSettingsForm;
window.initSettingsDropdowns = initSettingsDropdowns;
window.initSettingsBindings = initSettingsBindings;
// ── Discord Integration ──

let _discordConfig = null;

async function loadDiscordConfig() {
  if (await _ensureLambdaDemoMode()) {
    _applyLambdaDemoRestrictions();
    _updateDiscordCardStatus({ demo_disabled: true });
    return;
  }

  if (!_isAdmin()) {
    const section = document.getElementById('settings-integrations');
    if (section) {
      section.classList.add('settings-restricted');
      _addRestrictionNotice(section, 'Only admins can manage integrations.');
    }
    return;
  }
  try {
    const cfg = await window.API.getDiscord();
    _discordConfig = cfg;
    _updateDiscordCardStatus(cfg);
  } catch (e) {
    // Not configured yet
  }
}

function _updateDiscordCardStatus(cfg) {
  const statusEl = document.getElementById('discord-card-status');
  if (!statusEl) return;
  if (cfg && cfg.demo_disabled) {
    statusEl.textContent = 'Disabled';
    statusEl.className = 'integration-card-status disconnected';
    return;
  }
  if (cfg && cfg.enabled && cfg.channel_id) {
    statusEl.textContent = 'Connected';
    statusEl.className = 'integration-card-status connected';
  } else if (cfg && cfg.channel_id) {
    statusEl.textContent = 'Disabled';
    statusEl.className = 'integration-card-status disconnected';
  } else {
    statusEl.textContent = 'Not configured';
    statusEl.className = 'integration-card-status disconnected';
  }
}

async function openIntegrationModal(type) {
  if (await _ensureLambdaDemoMode()) {
    window.Toast.info('Integrations are disabled in Lambda demo mode');
    return;
  }

  if (type !== 'discord') return;
  const modal = document.getElementById('discord-modal');
  if (!modal) return;
  const cfg = _discordConfig || {};
  document.getElementById('discord-enabled').checked = !!cfg.enabled;
  document.getElementById('discord-bot-token').value = cfg.bot_token || '';
  document.getElementById('discord-guild-id').value = cfg.guild_id || '';
  document.getElementById('discord-channel-id').value = cfg.channel_id || '';
  // Notification preferences
  const notify = cfg.notify || {};
  document.getElementById('discord-notify-assigned').checked = notify.assigned !== false;
  document.getElementById('discord-notify-done').checked = notify.done !== false;
  document.getElementById('discord-notify-comment').checked = notify.comment !== false;
  document.getElementById('discord-notify-created').checked = !!notify.created;
  document.getElementById('discord-notify-priority').checked = notify.priority !== false;
  _updateDiscordStatus('');
  modal.classList.add('active');
}

function closeIntegrationModal() {
  document.getElementById('discord-modal').classList.remove('active');
}

async function saveDiscordConfig() {
  if (await _ensureLambdaDemoMode()) {
    window.Toast.info('Integrations are disabled in Lambda demo mode');
    return;
  }

  const btn = document.getElementById('discord-save-btn');
  btn.disabled = true;
  btn.textContent = 'Saving...';
  try {
    const data = {
      enabled: document.getElementById('discord-enabled').checked,
      bot_token: document.getElementById('discord-bot-token').value.trim(),
      guild_id: document.getElementById('discord-guild-id').value.trim(),
      channel_id: document.getElementById('discord-channel-id').value.trim(),
      notify: {
        assigned: document.getElementById('discord-notify-assigned').checked,
        done: document.getElementById('discord-notify-done').checked,
        comment: document.getElementById('discord-notify-comment').checked,
        created: document.getElementById('discord-notify-created').checked,
        priority: document.getElementById('discord-notify-priority').checked,
      },
    };
    // Don't send masked token back
    if (data.bot_token.includes('\u2022\u2022')) {
      delete data.bot_token;
    }
    const result = await window.API.putDiscord(data);
    _discordConfig = { ...(_discordConfig || {}), ...data, ...result };
    _updateDiscordCardStatus(_discordConfig);
    window.Toast.success('Discord settings saved');
    closeIntegrationModal();
  } catch (e) {
    window.Toast.error(e.message || 'Failed to save Discord settings');
  } finally {
    btn.disabled = false;
    btn.textContent = 'Save';
  }
}

async function testDiscordMessage() {
  if (await _ensureLambdaDemoMode()) {
    window.Toast.info('Integrations are disabled in Lambda demo mode');
    return;
  }

  const btn = document.getElementById('discord-test-btn');
  btn.disabled = true;
  btn.textContent = 'Sending...';
  _updateDiscordStatus('');
  try {
    const res = await window.API.testDiscord();
    window.Toast.success(res.status || 'Test message sent!');
    _updateDiscordStatus('Message sent!', 'success');
  } catch (e) {
    window.Toast.error(e.message || 'Failed to send test message');
    _updateDiscordStatus(e.message || 'Failed', 'error');
  } finally {
    btn.disabled = false;
    btn.textContent = 'Send Test Message';
  }
}

function _updateDiscordStatus(text, type) {
  const el = document.getElementById('discord-status');
  if (!el) return;
  el.textContent = text;
  el.className = 'integration-status';
  if (type) el.classList.add('integration-status-' + type);
}

window.loadDiscordConfig = loadDiscordConfig;
window.saveDiscordConfig = saveDiscordConfig;
window.testDiscordMessage = testDiscordMessage;
window.openIntegrationModal = openIntegrationModal;
window.closeIntegrationModal = closeIntegrationModal;

window.loadTeamMembers = loadTeamMembers;
window.updateUserRole = updateUserRole;
window.removeTeamMember = removeTeamMember;
window.sendInvite = sendInvite;
window.loadAPIKeys = loadAPIKeys;
window.copyAPIKeyToClipboard = copyAPIKeyToClipboard;
window.openCreateKeyModal = openCreateKeyModal;
window.closeCreateKeyModal = closeCreateKeyModal;
window.submitCreateKeyModal = submitCreateKeyModal;
window.copyCreatedKey = copyCreatedKey;
window.revokeAPIKey = revokeAPIKey;
window.importFromJira = importFromJira;
window.importFromTrello = importFromTrello;
window.exportData = exportData;
window.openResetModal = openResetModal;
window.closeResetModal = closeResetModal;
window.doReset = doReset;
window.updateDangerZoneLabels = updateDangerZoneLabels;
window.openSettings = openSettings;
window.closeSettings = closeSettings;
window.toggleBoardEnabled = toggleBoardEnabled;
window.loadBoardsSettings = loadBoardsSettings;
window.loadTransitionsSettings = loadTransitionsSettings;
window.testWebhook = testWebhook;
window.DEFAULT_AVATARS = DEFAULT_AVATARS;
window.openAvatarPicker = openAvatarPicker;
window.closeAvatarPicker = closeAvatarPicker;
window.uploadAvatarFile = uploadAvatarFile;
window.selectDefaultAvatar = selectDefaultAvatar;
window.removeAvatar = removeAvatar;
window.saveAvatar = saveAvatar;
window.resizeImageToDataURL = resizeImageToDataURL;
window._isAdmin = _isAdmin;
window._applyRoleRestrictions = _applyRoleRestrictions;
