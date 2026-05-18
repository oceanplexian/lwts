const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { JSDOM } = require('jsdom');

const ROOT = path.join(__dirname, '..', '..');
const settingsSrc = fs.readFileSync(path.join(ROOT, 'web', 'src', 'settings.js'), 'utf-8');

function loadSettingsModule() {
  const dom = new JSDOM('<!DOCTYPE html><html><body><div id="root"></div><div id="confirm-modal"><div id="confirm-modal-title"></div><div id="confirm-modal-message"></div><button id="confirm-modal-ok"></button></div></body></html>', {
    url: 'https://board.test/',
    pretendToBeVisual: true,
  });
  global.window = dom.window;
  global.document = dom.window.document;
  dom.window.HTMLCanvasElement.prototype.getContext = () => ({
    fillRect(){},
    beginPath(){},
    arc(){},
    fill(){},
    stroke(){},
    moveTo(){},
    lineTo(){},
    closePath(){},
  });
  dom.window.HTMLCanvasElement.prototype.toDataURL = () => 'data:image/png;base64,test';

  const updates = [];
  const toasts = [];
  dom.window.boardList = [];
  dom.window.currentBoardId = '';
  dom.window.parseBoardSettings = (raw) => JSON.parse(raw || '{}');
  dom.window.API = {
    updateBoard: async (id, body) => {
      updates.push({ id, body });
      return { id, settings: body.settings };
    },
  };
  dom.window.Toast = {
    error: (msg) => toasts.push({ type: 'error', msg }),
    info: (msg) => toasts.push({ type: 'info', msg }),
    success: (msg) => toasts.push({ type: 'success', msg }),
  };

  const wrapped = `
    const window = global.window;
    const document = global.document;
    const localStorage = window.localStorage;
    const history = { pushState(){}, replaceState(){} };
    const location = window.location;
    const confirm = () => true;
    const FileReader = function(){};
    ${settingsSrc}
    module.exports = {
      _bindCustomFieldInputs,
      _renderCustomFieldsConfig,
      _collectCustomFieldsFromDOM,
      _addCustomFieldOption,
      _removeCustomFieldOption,
      _showCustomFieldOptionColorPicker,
    };
  `;
  const mod = { exports: {} };
  const fn = new Function('require', 'module', 'exports', 'global', wrapped);
  fn(require, mod, mod.exports, global);
  return { dom, mod: mod.exports, updates, toasts };
}

function renderBoard(dom, mod, board) {
  dom.window.boardList = [board];
  const settings = JSON.parse(board.settings || '{}');
  dom.window.document.getElementById('root').innerHTML = mod._renderCustomFieldsConfig(board.id, settings.custom_fields || []);
}

describe('settings custom field choices', () => {
  it('renders choice fields with explicit option rows and add-choice control', () => {
    const { dom, mod } = loadSettingsModule();
    const board = {
      id: 'board-1',
      settings: JSON.stringify({
        custom_fields: [{
          id: 'severity',
          name: 'Severity',
          type: 'select',
          options: [{ id: 'sev1', label: 'SEV 1', color: '#579DFF' }],
        }],
      }),
    };

    renderBoard(dom, mod, board);
    const root = dom.window.document.getElementById('root');

    assert.equal(root.querySelectorAll('.settings-custom-field-option-row').length, 1);
    assert.ok(root.querySelector('.settings-custom-field-option-add-btn'));
    assert.equal(root.querySelector('.board-custom-field-options'), null);
    assert.equal(root.textContent.includes('Choices separated by commas'), false);

    const collected = mod._collectCustomFieldsFromDOM('board-1');
    assert.equal(collected[0].options[0].id, 'sev1');
    assert.equal(collected[0].options[0].label, 'SEV 1');
    assert.equal(collected[0].options[0].color, '#579DFF');
  });

  it('adds a new choice through the add-choice control and preserves existing option ids', async () => {
    const { dom, mod, updates } = loadSettingsModule();
    const board = {
      id: 'board-1',
      settings: JSON.stringify({
        custom_fields: [{
          id: 'severity',
          name: 'Severity',
          type: 'select',
          options: [{ id: 'sev1', label: 'SEV 1', color: '#579DFF' }],
        }],
      }),
    };

    renderBoard(dom, mod, board);
    dom.window.document.querySelector('.settings-custom-field-option-new').value = 'SEV 2';

    await mod._addCustomFieldOption('board-1', 'severity');

    assert.equal(updates.length, 1);
    const settings = JSON.parse(updates[0].body.settings);
    assert.deepEqual(settings.custom_fields[0].options.map(o => o.id), ['sev1', 'sev-2']);
    assert.deepEqual(settings.custom_fields[0].options.map(o => o.label), ['SEV 1', 'SEV 2']);
    assert.equal(dom.window.document.querySelectorAll('.settings-custom-field-option-row').length, 2);
  });

  it('rejects duplicate choice labels client-side before sending an API update', async () => {
    const { dom, mod, updates, toasts } = loadSettingsModule();
    const board = {
      id: 'board-1',
      settings: JSON.stringify({
        custom_fields: [{
          id: 'severity',
          name: 'Severity',
          type: 'select',
          options: [{ id: 'sev1', label: 'SEV 1', color: '#579DFF' }],
        }],
      }),
    };

    renderBoard(dom, mod, board);
    dom.window.document.querySelector('.settings-custom-field-option-new').value = 'sev 1';

    await mod._addCustomFieldOption('board-1', 'severity');

    assert.equal(updates.length, 0);
    assert.deepEqual(toasts, [{ type: 'info', msg: 'Choice already exists' }]);
  });

  it('does not allow removing the final choice from a choice field', async () => {
    const { dom, mod, updates, toasts } = loadSettingsModule();
    const board = {
      id: 'board-1',
      settings: JSON.stringify({
        custom_fields: [{
          id: 'severity',
          name: 'Severity',
          type: 'select',
          options: [{ id: 'sev1', label: 'SEV 1', color: '#579DFF' }],
        }],
      }),
    };

    renderBoard(dom, mod, board);

    await mod._removeCustomFieldOption('board-1', 'severity', 'sev1');

    assert.equal(updates.length, 0);
    assert.deepEqual(toasts, [{ type: 'error', msg: 'Choice fields need at least one choice' }]);
    assert.equal(dom.window.document.querySelectorAll('.settings-custom-field-option-row').length, 1);
  });
});
