// fn2 Kanban — Vite entry point (login.html)
// CSS is loaded via <link> tags in login.html to prevent FOUC.

import './auth.js';

// Apply saved theme before DOMContentLoaded to avoid flash
if (localStorage.getItem('lwts-theme') === 'light') {
  document.body.classList.add('light-theme');
}

document.addEventListener('DOMContentLoaded', () => {
  const LP = window.LoginPage;
  LP.init();

  // Theme toggle
  const toggleBtn = document.getElementById('auth-theme-toggle');
  if (toggleBtn) {
    toggleBtn.addEventListener('click', () => {
      const isLight = document.body.classList.toggle('light-theme');
      localStorage.setItem('lwts-theme', isLight ? 'light' : 'dark');
    });
  }

  // Debug view switcher
  const debugSwitcher = document.getElementById('auth-debug-switcher');
  if (debugSwitcher) {
    debugSwitcher.querySelectorAll('button').forEach(btn => {
      btn.addEventListener('click', () => {
        LP.showView(btn.dataset.view);
        debugSwitcher.querySelectorAll('button').forEach(b => b.classList.remove('active'));
        btn.classList.add('active');
      });
    });
  }

  // Determine initial view based on server registration status
  fetch('/api/v1/registration-status')
    .then(r => r.json())
    .then(data => {
      if (data.first_run) {
        LP.showInitialView('register');
        const subtitle = document.getElementById('auth-subtitle');
        if (subtitle) subtitle.textContent = 'Create your admin account';
        document.querySelectorAll('.auth-footer').forEach(f => { f.style.display = 'none'; });
      } else {
        if (!data.allowed) {
          const regView = document.getElementById('view-register');
          if (regView) {
            regView.classList.add('auth-disabled');
            regView.querySelectorAll('.auth-input').forEach(inp => { inp.disabled = true; });
            const btn = regView.querySelector('.auth-btn');
            if (btn) btn.disabled = true;
          }
        }
        LP.showInitialView('login');
      }
    })
    .catch(() => {
      LP.showInitialView('login');
    });
});
