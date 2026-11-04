// Opening a card while the new-card form is open (browser Back/Forward or a #KEY link).
// Run with: node --test tests/frontend/open-card-while-creating.test.js

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { JSDOM } = require('jsdom');

const ROOT = path.join(__dirname, '..', '..');
const read = (p) => fs.readFileSync(path.join(ROOT, p), 'utf-8');
const indexHtml = read('web/index.html').replace(/<script\b[\s\S]*?<\/script>/gi, '');

async function load() {
  const dom = new JSDOM(indexHtml, { url: 'https://board.test/', pretendToBeVisual: true, runScripts: 'outside-only' });
  const w = dom.window;
  const loaded = new Promise((r) => w.document.addEventListener('DOMContentLoaded', () => setTimeout(r, 0), { once: true }));
  const created = [];
  w.API = {
    createCard: (boardId, body) => { created.push(body); return Promise.resolve({ ...body, id: 'srv-1', key: 'ALP-2' }); },
    updateCard: () => Promise.resolve({}),
    listComments: () => Promise.resolve([]),
  };
  w.Toast = { success() {}, error() {}, info() {} };
  w.FnEditor = function () { return { getMarkdown: () => '', setMarkdown() {}, destroy() {}, focus() {} }; };
  w._settingsCache = {};
  w.HTMLElement.prototype.scrollIntoView = function () {};
  // The browser loads these as ES modules: top-level names stay private, only window.* is shared.
  w.eval(`(function () {\n${read('web/src/dropdown.js')}\n})();`);
  w.eval(`(function () {\n${read('web/src/kanban.js')}\n})();`);
  // As in the browser, kanban.js builds the detail dropdowns on DOMContentLoaded; wait for that.
  await loaded;

  const state = {};
  for (const col of w.COLUMNS) state[col.id] = [];
  state.todo.push({ id: 'c1', key: 'ALP-1', board_id: 'b1', title: 'Existing card', tag: 'blue', priority: 'medium',
    assignee: null, reporter: null, points: 0, date: '2026-01-01', description: '', attachments: [], comments: [],
    version: 1, custom_fields: {}, epic_id: null });
  w.state = state;
  w.currentBoardId = 'b1';
  w.boardList = [{ id: 'b1', name: 'Alpha', project_key: 'ALP' }];
  return { w, created };
}

const footerButtons = (w) =>
  Array.from(w.document.querySelectorAll('#detail-modal .detail-footer button')).map((b) => b.textContent.trim());
function typeTitle(w, text) {
  const el = w.document.getElementById('detail-title');
  el.value = text;
  el.dispatchEvent(new w.Event('input', { bubbles: true }));
}
const nextHashChange = (w) => new Promise((resolve) => w.addEventListener('hashchange', () => setTimeout(resolve, 0), { once: true }));

describe('opening a card while the new-card form is open', () => {
  it('opens the card normally instead of loading it into the create form', async () => {
    const { w, created } = await load();
    w.openCreateModal();
    typeTitle(w, 'Half-written new card');

    const changed = nextHashChange(w);
    w.location.hash = '#ALP-1';
    await changed;

    assert.equal(w._isCreateMode, false);
    assert.equal(w.document.getElementById('detail-title').value, 'Existing card');
    assert.equal(w.document.getElementById('detail-key-text').textContent.trim(), 'ALP-1');
    assert.deepEqual(footerButtons(w), ['Delete card', 'Close']);
    assert.equal(created.length, 0);
  });

  it('does not save a copy of the opened card', async () => {
    const { w, created } = await load();
    w.openCreateModal();
    typeTitle(w, 'Half-written new card'); // typing enables Create
    const changed = nextHashChange(w);
    w.location.hash = '#ALP-1';
    await changed;
    // Previously the footer still offered Create here, and using it duplicated the card.
    // (Footer buttons use inline onclick attributes, which jsdom does not run here, so call the handler.)
    const create = Array.from(w.document.querySelectorAll('#detail-modal .detail-footer button'))
      .find((b) => b.textContent.trim() === 'Create' && !b.disabled);
    if (create) w.submitCreateFromDetail();
    await new Promise((r) => setTimeout(r, 0));
    assert.equal(created.length, 0, 'created: ' + JSON.stringify(created.map((c) => c.title)));
    assert.equal(Object.values(w.state).flat().filter((c) => c.title === 'Existing card').length, 1);
  });
});
