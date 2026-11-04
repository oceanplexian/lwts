// Status options in the card detail and the new-card form follow the current board.
// Run with: node --test tests/frontend/detail-board-options.test.js

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { JSDOM } = require('jsdom');

const ROOT = path.join(__dirname, '..', '..');
const read = (p) => fs.readFileSync(path.join(ROOT, p), 'utf-8');
const indexHtml = read('web/index.html').replace(/<script\b[\s\S]*?<\/script>/gi, '');

const ALPHA = { id: 'b1', name: 'Alpha', project_key: 'ALP' };
const BETA = { id: 'b2', name: 'Beta', project_key: 'BET' };
const BETA_COLUMNS = [
  { id: 'open', label: 'Open' },
  { id: 'doing', label: 'Doing' },
  { id: 'closed', label: 'Closed' },
];

async function load() {
  const dom = new JSDOM(indexHtml, { url: 'https://board.test/', pretendToBeVisual: true, runScripts: 'outside-only' });
  const w = dom.window;
  const loaded = new Promise((r) => w.document.addEventListener('DOMContentLoaded', () => setTimeout(r, 0), { once: true }));
  w.API = { updateCard: () => Promise.resolve({}), listComments: () => Promise.resolve([]) };
  w.Toast = { success() {}, error() {}, info() {} };
  w.FnEditor = function () { return { getMarkdown: () => '', setMarkdown() {}, destroy() {}, focus() {} }; };
  w._settingsCache = {};
  w.HTMLElement.prototype.scrollIntoView = function () {};
  // The browser loads these as ES modules: top-level names stay private, only window.* is shared.
  w.eval(`(function () {\n${read('web/src/dropdown.js')}\n})();`);
  w.eval(`(function () {\n${read('web/src/kanban.js')}\n})();`);
  // As in the browser, kanban.js builds the detail dropdowns on DOMContentLoaded, before any board
  // has loaded. Wait for that, then open a board.
  w.boardList = [];
  await loaded;
  return w;
}

// What loadBoardCards does when a board is opened: COLUMNS follows the board's columns.
function showBoard(w, board, columns, cards) {
  w.COLUMNS.length = 0;
  columns.forEach((c) => w.COLUMNS.push(c));
  const state = {};
  for (const c of columns) state[c.id] = [];
  for (const card of cards) state[card.column_id].push(card);
  w.state = state;
  w.currentBoardId = board.id;
}

function card(over) {
  return { id: 'c1', key: 'BET-1', board_id: BETA.id, column_id: 'doing', title: 'A card', tag: 'blue',
    priority: 'medium', assignee: null, reporter: null, points: 0, date: '2026-01-01', description: '',
    attachments: [], comments: [], version: 1, custom_fields: {}, epic_id: null, ...over };
}

const options = (w, id) => Array.from(w.document.querySelectorAll(`#${id} .fn-dropdown-option`)).map((o) => o.textContent.trim());

describe('card detail options follow the current board', () => {
  it('lists the columns of the board the card is on', async () => {
    const w = await load();
    w.boardList = [ALPHA, BETA];
    showBoard(w, BETA, BETA_COLUMNS, [card()]);
    w.openDetail('c1');
    await new Promise((r) => setTimeout(r, 0));
    assert.deepEqual(options(w, 'detail-column'), ['Open', 'Doing', 'Closed']);
    assert.equal(w.document.getElementById('detail-column-text').textContent.trim(), 'Doing');
  });

  it('lists the current board\'s columns in the new-card form', async () => {
    const w = await load();
    w.boardList = [ALPHA, BETA];
    showBoard(w, BETA, BETA_COLUMNS, []);
    w.openCreateModal();
    assert.deepEqual(options(w, 'detail-column'), ['Open', 'Doing', 'Closed']);
  });
});
