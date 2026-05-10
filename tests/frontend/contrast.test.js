// Contrast tests — guard against dark-on-dark / barely-visible UI in
// blueprint (and other dark-bg) board themes.
//
// Run with: node --test tests/frontend/contrast.test.js
//
// Strategy: jsdom doesn't resolve var()/color-mix() inside computed styles,
// so we read computed values, manually trace one var() level via
// getPropertyValue, and resolve color-mix() ourselves. Then we compute the
// WCAG 2.1 contrast ratio and assert a sensible floor.

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { JSDOM } = require('jsdom');
const postcss = require('postcss');

const ROOT = path.join(__dirname, '..', '..');
const board = fs.readFileSync(path.join(ROOT, 'web/styles/board.css'), 'utf-8');
const theme = fs.readFileSync(path.join(ROOT, 'web/styles/theme.css'), 'utf-8');

// Pre-parse CSS so we can walk every declaration in source order — jsdom's
// rule.style collapses duplicates of the same property, dropping the var()
// fallback that sits next to the color-mix() declaration.
function parseRules(...sources) {
  const out = [];
  for (const src of sources) {
    const root = postcss.parse(src);
    root.walkRules((rule) => {
      const decls = [];
      rule.walkDecls((d) => decls.push({ prop: d.prop, value: d.value }));
      // Each comma-separated selector is its own rule for matching purposes.
      for (const sel of rule.selectors) out.push({ selector: sel, decls });
    });
  }
  return out;
}

const ALL_RULES = parseRules(theme, board);

function buildDom({ light, boardTheme, html }) {
  const cls = light ? 'light-theme' : '';
  const dom = new JSDOM(
    `<!DOCTYPE html><html><head><style>${theme}\n${board}</style></head>` +
    `<body class="${cls}" data-board-theme="${boardTheme}">${html}</body></html>`,
    { pretendToBeVisual: true }
  );
  return dom.window;
}

// ── Color parsing / blending ──────────────────────────────────────────

function parseColor(s) {
  if (!s || s === 'transparent') return { r: 0, g: 0, b: 0, a: 0 };
  s = s.trim();
  let m;
  if ((m = s.match(/^rgba?\(\s*([\d.]+)[\s,]+([\d.]+)[\s,]+([\d.]+)(?:[\s,/]+([\d.]+))?\s*\)$/i))) {
    return { r: +m[1], g: +m[2], b: +m[3], a: m[4] === undefined ? 1 : +m[4] };
  }
  if ((m = s.match(/^#([0-9a-f]{3}|[0-9a-f]{6})$/i))) {
    const h = m[1];
    if (h.length === 3) return { r: parseInt(h[0]+h[0],16), g: parseInt(h[1]+h[1],16), b: parseInt(h[2]+h[2],16), a: 1 };
    return { r: parseInt(h.slice(0,2),16), g: parseInt(h.slice(2,4),16), b: parseInt(h.slice(4,6),16), a: 1 };
  }
  if (s === 'white') return { r: 255, g: 255, b: 255, a: 1 };
  if (s === 'black') return { r: 0, g: 0, b: 0, a: 1 };
  throw new Error('Unparseable color: ' + s);
}

function mix(a, b, pctA) {
  const f = pctA / 100;
  const g = 1 - f;
  return {
    r: Math.round(a.r * f + b.r * g),
    g: Math.round(a.g * f + b.g * g),
    b: Math.round(a.b * f + b.b * g),
    a: a.a * f + b.a * g,
  };
}

function over(fg, bg) {
  // Composite fg over bg using straight alpha. bg is assumed opaque.
  const fa = fg.a;
  return {
    r: Math.round(fg.r * fa + bg.r * (1 - fa)),
    g: Math.round(fg.g * fa + bg.g * (1 - fa)),
    b: Math.round(fg.b * fa + bg.b * (1 - fa)),
    a: 1,
  };
}

// Resolve a computed-style value into an opaque RGB color. Handles direct
// rgb()/rgba()/hex, single-level var(--name) lookups against :root/body,
// and color-mix(in srgb, X N%, Y M%) where X and Y are themselves resolvable.
function resolveColor(window, raw, bgForBlend = { r: 255, g: 255, b: 255, a: 1 }) {
  if (!raw || raw === 'transparent' || raw === 'rgba(0, 0, 0, 0)') return bgForBlend;
  raw = raw.trim();

  let m = raw.match(/^var\((--[^,)]+)(?:,\s*(.+))?\)$/);
  if (m) {
    // jsdom only resolves :root vars on documentElement (not inherited to
    // body/children), so try both.
    const body = window.getComputedStyle(window.document.body).getPropertyValue(m[1]).trim();
    const root = window.getComputedStyle(window.document.documentElement).getPropertyValue(m[1]).trim();
    const v = body || root;
    return resolveColor(window, v || m[2] || 'transparent', bgForBlend);
  }

  m = raw.match(/^color-mix\(\s*in\s+srgb\s*,\s*(.+)\)$/);
  if (m) {
    const args = splitTopLevel(m[1]);
    if (args.length !== 2) throw new Error('color-mix expects 2 args: ' + raw);
    const [a, ap] = parseMixArg(args[0]);
    const [b, bp] = parseMixArg(args[1]);
    const ra = resolveColor(window, a, bgForBlend);
    const rb = resolveColor(window, b, bgForBlend);
    const total = (ap ?? 50) + (bp ?? 50);
    const aPct = ap !== null ? ap : 100 - bp;
    return mix(ra, rb, (aPct / total) * 100);
  }

  return parseColor(raw);
}

function splitTopLevel(s) {
  const out = [];
  let depth = 0, start = 0;
  for (let i = 0; i < s.length; i++) {
    const c = s[i];
    if (c === '(') depth++;
    else if (c === ')') depth--;
    else if (c === ',' && depth === 0) { out.push(s.slice(start, i).trim()); start = i + 1; }
  }
  out.push(s.slice(start).trim());
  return out;
}

function parseMixArg(s) {
  const m = s.match(/^(.+?)\s+(-?[\d.]+)%$/);
  if (m) return [m[1].trim(), parseFloat(m[2])];
  return [s.trim(), null];
}

// ── WCAG contrast ──────────────────────────────────────────────────────

function relLum({ r, g, b }) {
  const f = (c) => {
    const s = c / 255;
    return s <= 0.03928 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4);
  };
  return 0.2126 * f(r) + 0.7152 * f(g) + 0.0722 * f(b);
}

function contrast(a, b) {
  const la = relLum(a), lb = relLum(b);
  const [hi, lo] = la > lb ? [la, lb] : [lb, la];
  return (hi + 0.05) / (lo + 0.05);
}

// ── Helper: read fg/bg of an element, resolved over a known parent bg ──
//
// jsdom's getComputedStyle drops declarations it can't parse (notably
// color-mix()), so we walk the stylesheet ourselves and grab the last
// matching `color` / `background[-color]` declaration the cascade would
// have applied. Specificity ordering for the selectors we test is
// preserved by source order (the blueprint overrides come later in the
// file than the base rules).

function lastDeclMatching(window, sel, propRe) {
  const el = window.document.querySelector(sel);
  assert.ok(el, `selector did not match: ${sel}`);
  const re = new RegExp(`^(?:${propRe})$`);
  let lastVal = null;
  for (const rule of ALL_RULES) {
    let matches = false;
    try { matches = el.matches(rule.selector); } catch { continue; }
    if (!matches) continue;
    for (const d of rule.decls) {
      if (re.test(d.prop)) lastVal = d.value;
    }
  }
  return lastVal;
}

function readFgBg(window, sel, parentBgRgb) {
  const fgRaw = lastDeclMatching(window, sel, 'color') || 'rgb(0,0,0)';
  const bgRaw = lastDeclMatching(window, sel, 'background(?:-color)?') || 'transparent';
  let bg = resolveColor(window, bgRaw, parentBgRgb);
  bg = over(bg, parentBgRgb);
  const fg = over(resolveColor(window, fgRaw, bg), bg);
  return { fg, bg };
}

// ── Tests ──────────────────────────────────────────────────────────────

describe('blueprint chip-style controls — transparent bg, readable text in both modes', () => {
  const html = `
    <button class="filter-btn">Assignee</button>
    <span class="column-count">7</span>
    <span class="epic-lane-key">FNAI-191</span>
  `;
  // Page bg in blueprint light is near-white; in blueprint dark it's near-black.
  const lightParent = { r: 245, g: 245, b: 245, a: 1 };
  const darkParent = { r: 12, g: 18, b: 26, a: 1 };

  for (const sel of ['.filter-btn', '.column-count', '.epic-lane-key']) {
    it(`${sel} — has transparent background in blueprint (light)`, () => {
      const w = buildDom({ light: true, boardTheme: 'blueprint', html });
      const bgRaw = lastDeclMatching(w, sel, 'background(?:-color)?') || 'transparent';
      // For this assertion we want the chip's *own* alpha, not what it
      // looks like composited over the page. Pass an alpha-0 sentinel as
      // bgForBlend so resolveColor doesn't substitute the page bg when it
      // hits 'transparent'.
      const bg = resolveColor(w, bgRaw, { r: 0, g: 0, b: 0, a: 0 });
      assert.equal(bg.a, 0, `${sel}: bg should be transparent, got alpha=${bg.a} (${bgRaw})`);
    });

    it(`${sel} — text vs page contrast >= 7.0 in blueprint (light)`, () => {
      const w = buildDom({ light: true, boardTheme: 'blueprint', html });
      const { fg, bg } = readFgBg(w, sel, lightParent);
      const ratio = contrast(fg, bg);
      assert.ok(
        ratio >= 7.0,
        `${sel} (light): contrast ${ratio.toFixed(2)} too low — fg=rgb(${fg.r},${fg.g},${fg.b}) bg=rgb(${bg.r},${bg.g},${bg.b})`
      );
    });

    it(`${sel} — text vs page contrast >= 7.0 in blueprint (dark)`, () => {
      const w = buildDom({ light: false, boardTheme: 'blueprint', html });
      const { fg, bg } = readFgBg(w, sel, darkParent);
      const ratio = contrast(fg, bg);
      assert.ok(
        ratio >= 7.0,
        `${sel} (dark): contrast ${ratio.toFixed(2)} too low — fg=rgb(${fg.r},${fg.g},${fg.b}) bg=rgb(${bg.r},${bg.g},${bg.b})`
      );
    });
  }
});

describe('blueprint dark theme — search-result active state is distinguishable', () => {
  it('.search-result-item.active background differs from sibling rows by >= 1.2x', () => {
    const html = `
      <div class="header-search-results">
        <div class="search-result-item active">
          <span class="search-result-key">FNAI-1</span>
          <span class="search-result-title">item</span>
        </div>
        <div class="search-result-item">
          <span class="search-result-key">FNAI-2</span>
          <span class="search-result-title">other</span>
        </div>
      </div>`;
    const w = buildDom({ light: false, boardTheme: 'blueprint', html });
    // The dropdown background is the surface we sit on.
    const bodyCs = w.getComputedStyle(w.document.body);
    const dropdownBg = resolveColor(w, bodyCs.getPropertyValue('--surface-dropdown'), { r: 8, g: 18, b: 29, a: 1 });
    const dropdownOpaque = over(dropdownBg, { r: 0, g: 0, b: 0, a: 1 });

    const activeRaw = lastDeclMatching(w, '.search-result-item.active', 'background(?:-color)?') || 'transparent';
    const inactiveRaw = lastDeclMatching(w, '.search-result-item:not(.active)', 'background(?:-color)?') || 'transparent';
    const activeBg = over(resolveColor(w, activeRaw, dropdownOpaque), dropdownOpaque);
    const inactiveBg = over(resolveColor(w, inactiveRaw, dropdownOpaque), dropdownOpaque);
    const ratio = contrast(activeBg, inactiveBg);
    assert.ok(
      ratio >= 1.2,
      `active vs inactive bg contrast ${ratio.toFixed(3)} too low to see the selection (active=${JSON.stringify(activeBg)} inactive=${JSON.stringify(inactiveBg)})`
    );
  });
});
