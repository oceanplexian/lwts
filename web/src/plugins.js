// LWTS header plugin system.
//
// Lets an external plugin customize the board header brand (.header-title)
// without forking lwts core. Plugin scripts are injected into <head> by the
// server (see server/cmd/plugins.go) as <script defer> + a FOUC guard that
// hides the default .header-title, so the old brand never flashes before the
// plugin mounts. Plugins register via the order-independent queue
// window.lwts._hp (or window.lwts.registerHeaderPlugin); this loader boots
// on DOMContentLoaded and calls mount(el) for every .header-title.
//
// In dev (no server-side injection) it falls back to fetching
// GET /api/v1/plugins and <script>-loading each entry. The right-side
// header actions (.header-actions — Add card, Clear done, search, user menu)
// are never touched.

window.lwts = window.lwts || {};

// Order-independent registration queue. A plugin loaded before this module
// (a defer <script> in <head>) just pushes here; a plugin loaded after can
// use registerHeaderPlugin (which also pushes + mounts immediately).
window.lwts._hp = window.lwts._hp || [];

const _mounted = [];
let _slots = null;   // NodeList of .header-title, once booted
let _booted = false;

function applyPlugin(plugin) {
  if (!_slots) return;
  _slots.forEach((el) => {
    try {
      if (typeof plugin.mount === 'function') plugin.mount(el);
    } catch (e) {
      console.error('[lwts header plugin] mount failed:', e);
    }
  });
  _mounted.push(plugin);
}

window.lwts.registerHeaderPlugin = function (plugin) {
  (window.lwts._hp = window.lwts._hp || []).push(plugin);
  if (_booted) applyPlugin(plugin);
};

function boot() {
  _slots = document.querySelectorAll('.header-title');
  _booted = true;
  (window.lwts._hp || []).forEach(applyPlugin);

  // FOUC guard: if no plugin mounted (e.g. the plugin script failed to
  // load), remove the hide-<style> so the default LWTS title shows again.
  if (_mounted.length === 0) {
    const guard = document.getElementById('lwts-plugin-fouc');
    if (guard) guard.remove();
  }
}

function loadManifest() {
  // Fallback for dev (no server-side injection): fetch the manifest and
  // <script>-load each entry. In prod the server injects the tags directly,
  // so skip URLs already present to avoid double-loading.
  fetch('/api/v1/plugins', { headers: { Accept: 'application/json' }, credentials: 'same-origin' })
    .then((r) => (r.ok ? r.json() : { headerPlugins: [] }))
    .then((data) => {
      const urls = (data && Array.isArray(data.headerPlugins)) ? data.headerPlugins : [];
      const have = new Set(Array.from(document.scripts).map((s) => s.src));
      urls.forEach((src) => {
        let full = src;
        try { full = new URL(src, location.href).href; } catch (e) { /* keep src */ }
        if (have.has(full)) return; // already injected server-side
        const s = document.createElement('script');
        s.src = src;
        s.defer = true;
        s.async = false; // preserve declaration order across plugins
        s.onerror = () => console.error('[lwts header plugin] failed to load:', src);
        document.head.appendChild(s);
      });
    })
    .catch(() => {
      // No manifest / fetch failed — default LWTS title stays in place.
    });
}

document.addEventListener('DOMContentLoaded', boot);
// Kick off the manifest fetch immediately (no-op in prod where scripts are
// already injected; used by the dev fallback).
loadManifest();