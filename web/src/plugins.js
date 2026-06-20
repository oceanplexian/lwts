// LWTS header plugin system.
//
// Lets an external plugin customize the board header brand (.header-title)
// without forking lwts core. The server advertises plugin script paths via
// GET /api/v1/plugins (config: LWTS_HEADER_PLUGINS, served from
// LWTS_PLUGINS_DIR at /plugins/). This loader:
//   1. fetches the manifest and <script>-loads each entry (same-origin), and
//   2. exposes window.lwts.registerHeaderPlugin({ mount(el), unmount(el) }).
//
// On boot, each registered plugin's mount(el) is called with every
// .header-title element (the LWTS brand slot). Plugins that register after
// boot mount into the live slots immediately. With no plugin configured the
// default LWTS title is left untouched. The right-side header actions
// (.header-actions — Add card, Clear done, search, user menu) are never
// touched by this system.

window.lwts = window.lwts || {};

const _plugins = [];
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
}

window.lwts.registerHeaderPlugin = function (plugin) {
  _plugins.push(plugin);
  if (_booted) applyPlugin(plugin);
};

function boot() {
  _slots = document.querySelectorAll('.header-title');
  _booted = true;
  _plugins.forEach(applyPlugin);
}

function loadManifest() {
  fetch('/api/v1/plugins', { headers: { Accept: 'application/json' }, credentials: 'same-origin' })
    .then((r) => (r.ok ? r.json() : { headerPlugins: [] }))
    .then((data) => {
      const urls = (data && Array.isArray(data.headerPlugins)) ? data.headerPlugins : [];
      urls.forEach((src) => {
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
// Kick off the manifest fetch immediately; scripts that load before boot
// queue via registerHeaderPlugin, ones that load after mount on register.
loadManifest();