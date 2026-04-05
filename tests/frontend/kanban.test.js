// Frontend unit tests for pure functions in web/src/kanban.js
// Run with: node --test tests/frontend/kanban.test.js

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');

// ── Minimal DOM shim for esc() which uses document.createElement ──
const { JSDOM } = (() => {
  try { return require('jsdom'); } catch { return { JSDOM: null }; }
})();

let esc, _inlineMd, renderMarkdownInline, _capitalize, parseGithubUrls;

if (JSDOM) {
  // Use jsdom for faithful esc()
  const dom = new JSDOM('<!DOCTYPE html><html><body></body></html>');
  global.document = dom.window.document;
} else {
  // Lightweight shim — covers &, <, >, ", '
  global.document = {
    createElement() {
      let _text = '';
      return {
        set textContent(v) { _text = v; },
        get innerHTML() {
          return _text
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#039;');
        }
      };
    }
  };
}

// ── Extract functions from kanban.js ──
// We eval the source in a controlled scope to pull out the pure functions.
const fs = require('fs');
const path = require('path');
const src = fs.readFileSync(
  path.join(__dirname, '..', '..', 'web', 'src', 'kanban.js'),
  'utf-8'
);

// Build a module by wrapping the source. We stub out browser globals that
// the top-level code references (localStorage, document.getElementById, etc.)
// and then export the functions we care about.
const wrapped = `
  // Browser stubs
  const localStorage = { getItem(){ return null; }, setItem(){} };
  const window = { FnEditor: function(){}, addEventListener(){}, location: { hash: '' } };
  if (typeof document === 'undefined') var document = global.document;
  if (!document.getElementById) document.getElementById = function(){ return null; };
  if (!document.addEventListener) document.addEventListener = function(){};
  if (!document.body) document.body = { classList: { contains(){ return false; } } };
  if (!document.querySelectorAll) document.querySelectorAll = function(){ return []; };
  if (!document.querySelector) document.querySelector = function(){ return null; };
  const history = { replaceState(){} };
  const fetch = function(){ return Promise.resolve({ ok: true, json(){ return Promise.resolve({}); } }); };
  const console = { log(){}, error(){}, warn(){} };
  const alert = function(){};

  ${src}

  module.exports = { esc, _inlineMd, renderMarkdownInline, _capitalize, parseGithubUrls, fromAPI, COLUMNS, TAG_LABELS };
`;

const mod = {};
const fn = new Function('require', 'module', 'exports', 'global', wrapped);
fn(require, mod, mod.exports || {}, global);
const K = mod.exports;

esc = K.esc;
_inlineMd = K._inlineMd;
renderMarkdownInline = K.renderMarkdownInline;
_capitalize = K._capitalize;
parseGithubUrls = K.parseGithubUrls;

// ═══════════════════════════════════════════════════════════════════════
// Tests
// ═══════════════════════════════════════════════════════════════════════

// ── _capitalize ──────────────────────────────────────────────────────
describe('_capitalize', () => {
  it('capitalizes the first character', () => {
    assert.equal(_capitalize('hello'), 'Hello');
  });

  it('returns empty-ish values unchanged', () => {
    assert.equal(_capitalize(''), '');
    assert.equal(_capitalize(null), null);
    assert.equal(_capitalize(undefined), undefined);
  });

  it('handles single character', () => {
    assert.equal(_capitalize('a'), 'A');
  });
});

// ── esc (HTML escaping) ──────────────────────────────────────────────
describe('esc', () => {
  it('escapes angle brackets', () => {
    assert.ok(esc('<script>').includes('&lt;'));
    assert.ok(esc('<script>').includes('&gt;'));
  });

  it('escapes ampersand', () => {
    assert.ok(esc('a & b').includes('&amp;'));
  });

  it('leaves plain text unchanged', () => {
    assert.equal(esc('hello world'), 'hello world');
  });
});

// ── _inlineMd (auto-linkify + inline markdown) ──────────────────────
describe('_inlineMd', () => {
  // Bold / italic / code
  it('renders bold text', () => {
    assert.equal(_inlineMd('**bold**'), '<strong>bold</strong>');
  });

  it('renders italic text', () => {
    assert.equal(_inlineMd('*italic*'), '<em>italic</em>');
  });

  it('renders inline code', () => {
    assert.equal(_inlineMd('use `foo` here'), 'use <code>foo</code> here');
  });

  // Markdown links
  it('renders markdown links with target _blank', () => {
    const out = _inlineMd('[click](https://example.com)');
    assert.ok(out.includes('href="https://example.com"'), 'should have href');
    assert.ok(out.includes('>click</a>'), 'should have link text');
    assert.ok(out.includes('target="_blank"'), 'should open in new tab');
  });

  // Auto-linkify bare URLs
  it('auto-linkifies bare https:// URLs', () => {
    const out = _inlineMd('visit https://example.com today');
    assert.ok(out.includes('<a href="https://example.com"'), 'bare URL should be linked');
    assert.ok(out.includes('target="_blank"'));
  });

  it('auto-linkifies bare http:// URLs', () => {
    const out = _inlineMd('see http://example.com/path?q=1');
    assert.ok(out.includes('<a href="http://example.com/path?q=1"'), 'http URL should be linked');
  });

  // URLs inside backtick code spans should NOT be linkified
  it('does NOT linkify URLs inside inline code', () => {
    const out = _inlineMd('run `curl https://api.example.com` to test');
    // The URL should be inside <code>...</code> and NOT wrapped in <a>
    assert.ok(out.includes('<code>curl https://api.example.com</code>'), 'URL in code should stay plain');
    // Make sure there is no <a> wrapping that URL
    assert.ok(!out.includes('<a href="https://api.example.com"'), 'URL in code must not be linkified');
  });

  // URL already in a markdown link should not be double-linked
  it('does NOT double-linkify URLs inside markdown links', () => {
    const out = _inlineMd('[docs](https://docs.example.com)');
    // Count <a occurrences — should be exactly 1
    const count = (out.match(/<a /g) || []).length;
    assert.equal(count, 1, 'should have exactly one <a> tag, not double-wrapped');
  });

  // Mixed: bold + link
  it('handles bold with a URL on the same line', () => {
    const out = _inlineMd('**note** https://example.com');
    assert.ok(out.includes('<strong>note</strong>'));
    assert.ok(out.includes('<a href="https://example.com"'));
  });

  // HTML escaping before markdown
  it('escapes HTML entities in input', () => {
    const out = _inlineMd('<b>not bold</b>');
    assert.ok(!out.includes('<b>'), 'raw HTML tags should be escaped');
    assert.ok(out.includes('&lt;b&gt;'));
  });
});

// ── renderMarkdownInline (block-level markdown) ─────────────────────
describe('renderMarkdownInline', () => {
  it('returns empty string for falsy input', () => {
    assert.equal(renderMarkdownInline(''), '');
    assert.equal(renderMarkdownInline(null), '');
    assert.equal(renderMarkdownInline(undefined), '');
  });

  it('renders fenced code blocks', () => {
    const out = renderMarkdownInline('```\nconst x = 1;\n```');
    assert.ok(out.includes('<pre><code>'), 'should have code block');
    assert.ok(out.includes('const x = 1;'));
  });

  it('renders headings', () => {
    const h1 = renderMarkdownInline('# Title');
    assert.ok(h1.includes('md-h1'));
    const h2 = renderMarkdownInline('## Subtitle');
    assert.ok(h2.includes('md-h2'));
  });

  it('renders bullet lists', () => {
    const out = renderMarkdownInline('- item one\n- item two');
    assert.ok(out.includes('<ul class="md-list">'));
    assert.ok(out.includes('<li>item one</li>'));
    assert.ok(out.includes('<li>item two</li>'));
  });

  it('strips trailing <br> tags', () => {
    const out = renderMarkdownInline('hello\n');
    assert.ok(!out.endsWith('<br>'), 'should not end with <br>');
  });

  it('applies inline markdown inside headings', () => {
    const out = renderMarkdownInline('# **bold title**');
    assert.ok(out.includes('<strong>bold title</strong>'));
  });
});

// ── parseGithubUrls ─────────────────────────────────────────────────
describe('parseGithubUrls', () => {
  it('extracts issue URLs from description', () => {
    const card = { description: 'See https://github.com/acme/repo/issues/42' };
    const results = parseGithubUrls(card);
    assert.equal(results.length, 1);
    assert.equal(results[0].owner, 'acme');
    assert.equal(results[0].repo, 'repo');
    assert.equal(results[0].type, 'issues');
    assert.equal(results[0].number, 42);
  });

  it('extracts pull request URLs', () => {
    const card = { description: 'Fix in https://github.com/org/proj/pull/7' };
    const results = parseGithubUrls(card);
    assert.equal(results.length, 1);
    assert.equal(results[0].type, 'pull');
    assert.equal(results[0].number, 7);
  });

  it('deduplicates repeated URLs', () => {
    const card = { description: 'https://github.com/a/b/issues/1 and https://github.com/a/b/issues/1' };
    const results = parseGithubUrls(card);
    assert.equal(results.length, 1);
  });

  it('extracts from comments too', () => {
    const card = {
      description: '',
      comments: [{ text: 'https://github.com/x/y/pull/99' }]
    };
    const results = parseGithubUrls(card);
    assert.equal(results.length, 1);
    assert.equal(results[0].number, 99);
  });

  it('returns empty array for null card', () => {
    assert.deepEqual(parseGithubUrls(null), []);
  });

  it('returns empty array when no GitHub URLs', () => {
    assert.deepEqual(parseGithubUrls({ description: 'no links here' }), []);
  });
});

// ── CSS property assertions for .detail-points-input ────────────────
describe('.detail-points-input CSS', () => {
  const css = fs.readFileSync(
    path.join(__dirname, '..', '..', 'web', 'styles', 'detail.css'),
    'utf-8'
  );

  // Extract the .detail-points-input block
  const blockMatch = css.match(/\.detail-points-input\s*\{([^}]+)\}/);
  assert.ok(blockMatch, '.detail-points-input rule should exist in detail.css');
  const block = blockMatch[1];

  it('has vertical-align: top', () => {
    assert.ok(/vertical-align\s*:\s*top/.test(block), 'should have vertical-align: top');
  });

  it('has explicit height', () => {
    assert.ok(/height\s*:\s*34px/.test(block), 'should have height: 34px');
  });

  it('has box-sizing: border-box', () => {
    assert.ok(/box-sizing\s*:\s*border-box/.test(block), 'should have box-sizing: border-box');
  });

  it('has line-height set', () => {
    assert.ok(/line-height/.test(block), 'should have line-height for vertical centering');
  });

  it('hides number spinners', () => {
    assert.ok(css.includes('.detail-points-input::-webkit-inner-spin-button'), 'should hide webkit spin buttons');
    assert.ok(css.includes('-moz-appearance: textfield'), 'should hide Firefox spin buttons');
  });
});
