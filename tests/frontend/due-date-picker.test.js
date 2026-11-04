// Placement of the due date picker in web/src/features.js.
// Run with: node --test tests/frontend/due-date-picker.test.js

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { JSDOM } = require('jsdom');

const ROOT = path.join(__dirname, '..', '..');
const featuresSrc = fs.readFileSync(path.join(ROOT, 'web', 'src', 'features.js'), 'utf-8');

const PICKER_W = 280;

// jsdom has no layout: the test supplies the window height, the field's position and the picker's height.
function load({ innerHeight, field, pickerHeight }) {
  const dom = new JSDOM('<!DOCTYPE html><html><body><span id="detail-duedate-text">Apr 4</span></body></html>', {
    url: 'https://board.test/', pretendToBeVisual: true, runScripts: 'outside-only',
  });
  const w = dom.window;
  Object.defineProperty(w, 'innerHeight', { value: innerHeight, configurable: true });
  Object.defineProperty(w, 'innerWidth', { value: 1280, configurable: true });
  const height = { value: pickerHeight };
  Object.defineProperty(w.HTMLElement.prototype, 'offsetHeight', {
    configurable: true, get() { return this.id === 'due-date-picker' ? height.value : 0; },
  });
  Object.defineProperty(w.HTMLElement.prototype, 'offsetWidth', {
    configurable: true, get() { return this.id === 'due-date-picker' ? PICKER_W : 0; },
  });
  w.document.getElementById('detail-duedate-text').getBoundingClientRect = () =>
    ({ top: field.top, bottom: field.top + 20, left: field.left, right: field.left + 60, width: 60, height: 20 });
  w.detailCard = { due_date: '2026-04-04' };
  w.eval(`(function () {\n${featuresSrc}\n})();`);
  return { w, height };
}

function open(opts) {
  const { w, height } = load(opts);
  w.openDueDatePicker();
  const picker = w.document.getElementById('due-date-picker');
  const top = () => parseFloat(picker.style.top);
  return { w, picker, top, height };
}

describe('due date picker placement', () => {
  it('opens below the field when it fits (unchanged)', () => {
    const { top } = open({ innerHeight: 1200, field: { top: 490, left: 940 }, pickerHeight: 350 });
    assert.equal(top(), 516);
  });

  it('opens above the field when it would run past the bottom of the window', () => {
    // Field low in the sidebar of an 800px-tall window: below would end at 866px.
    const { top, picker } = open({ innerHeight: 800, field: { top: 490, left: 940 }, pickerHeight: 350 });
    assert.equal(top(), 490 - 6 - 350);
    assert.ok(top() + 350 <= 800);
    assert.equal(picker.style.transformOrigin, 'bottom left');
  });

  it('stays inside the window when it fits neither below nor above', () => {
    const { top } = open({ innerHeight: 500, field: { top: 250, left: 940 }, pickerHeight: 350 });
    assert.ok(top() >= 8, `top ${top()}`);
    assert.ok(top() + 350 <= 500 - 8, `bottom ${top() + 350}`);
  });

  it('stays inside the window horizontally', () => {
    const { picker } = open({ innerHeight: 1200, field: { top: 300, left: 1200 }, pickerHeight: 350 });
    assert.ok(parseFloat(picker.style.left) + PICKER_W <= 1280 - 8);
  });

  it('is placed again when a month change makes it taller', () => {
    const { w, top, height } = open({ innerHeight: 800, field: { top: 490, left: 940 }, pickerHeight: 310 });
    assert.ok(top() + 310 <= 800);
    height.value = 390; // a month that needs six rows
    w._calNext();
    assert.ok(top() + 390 <= 800, `bottom ${top() + 390}`);
  });
});
