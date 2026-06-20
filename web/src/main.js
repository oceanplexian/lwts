// LWTS Kanban — Vite entry point (SPA)
// CSS is loaded via <link> tags in index.html to prevent FOUC.

// JS modules — loaded in dependency order
import './auth.js';
import './api.js';
import './dropdown.js';
import './editor.js';
import './boardstream.js';
import './presence.js';
import './notifications.js';
import './kanban.js';
import './boardthemes.js';
import './settings.js';
import './listview.js';
import './features.js';
import './subtasks.js';
import './a11y.js';
import './plugins.js';

document.addEventListener('DOMContentLoaded', () => {
  const LP = window.LoginPage;
  const authRoot = document.getElementById('app-auth');
  const boardRoot = document.getElementById('app-board');
  if (!LP || typeof LP.init !== 'function' || !window.Auth || !authRoot || !boardRoot) {
    return;
  }
  LP.init();

  // Auth theme toggle
  const toggleBtn = document.getElementById('auth-theme-toggle');
  if (toggleBtn) {
    toggleBtn.addEventListener('click', () => {
      document.body.classList.toggle('light-theme');
    });
  }

  // If we have a token, validate it before showing the board
  if (window.Auth.isAuthenticated()) {
    const showBoard = (user) => {
      // Show welcome modal before board if user hasn't been welcomed
      if (user && !user.welcomed && window.showWelcome) {
        window.showWelcome();
      }
      // If hash points to settings, hide the board div before
      // making the container visible to prevent a flash of board content.
      const hash = window.location.hash.replace(/^#/, '');
      const isSettingsRoute = hash === 'settings' || hash.startsWith('settings/');
      if (isSettingsRoute) {
        const boardDiv = document.getElementById('board');
        if (boardDiv) boardDiv.style.display = 'none';
      }
      boardRoot.style.display = '';
      if (window.initBoard && !window._boardInitialized) {
        window._boardInitialized = true;
        window.initBoard();
      }
    };

    const meHeaders = () => ({ 'Authorization': 'Bearer ' + window.Auth.getAccessToken() });

    fetch('/api/auth/me', { headers: meHeaders() })
      .then(async (res) => {
        if (res.ok) {
          showBoard(await res.json());
          return;
        }
        // Only a genuine auth rejection should end the session. Try a token
        // refresh first; retry /me if it succeeds before giving up.
        if (res.status === 401 || res.status === 403) {
          const canRefresh = window.API && window.Auth.getRefreshToken();
          if (canRefresh && await window.API._doRefresh()) {
            const retry = await fetch('/api/auth/me', { headers: meHeaders() });
            if (retry.ok) { showBoard(await retry.json()); return; }
          }
          window.Auth.clearTokens();
          showAuthFlow();
          return;
        }
        // 5xx / unexpected: token is probably still valid, the server is just
        // unhappy. Keep the session and show the board; its own API calls go
        // through the refresh-aware client.
        showBoard(null);
      })
      .catch(() => {
        // Network error / timeout (e.g. a slow load). Do NOT log the user out —
        // spuriously clearing tokens here was a cause of frequent re-logins.
        // Keep the tokens and show the board; the refresh-aware API client
        // handles auth on the board's own requests.
        showBoard(null);
      });
    return;
  }

  showAuthFlow();

  function showAuthFlow() {
    authRoot.style.display = '';

    Promise.allSettled([
      fetch('/api/v1/registration-status').then(r => r.json()),
      fetch('/api/v1/lambda-demo').then(r => r.json()),
    ])
      .then(([registrationResult, lambdaDemoResult]) => {
        const registration = registrationResult.status === 'fulfilled' ? registrationResult.value : null;
        const lambdaDemoEnabled =
          lambdaDemoResult.status === 'fulfilled' &&
          lambdaDemoResult.value &&
          lambdaDemoResult.value.lambda_demo === true;

        if (registration && registration.first_run) {
          LP.showInitialView('register');
          const subtitle = document.getElementById('auth-subtitle');
          if (subtitle) subtitle.textContent = 'Create your admin account';
          document.querySelectorAll('.auth-footer').forEach(f => { f.style.display = 'none'; });
          return;
        }

        if (registration && !registration.allowed) {
          const regView = document.getElementById('view-register');
          if (regView) {
            regView.classList.add('auth-disabled');
            regView.querySelectorAll('.auth-input').forEach(inp => { inp.disabled = true; });
            const btn = regView.querySelector('.auth-btn');
            if (btn) btn.disabled = true;
          }
        }

        LP.showInitialView('login');
        if (lambdaDemoEnabled) {
          applyLambdaDemoLoginDefaults();
        }
      })
      .catch(() => {
        LP.showInitialView('login');
      });
  }

  function applyLambdaDemoLoginDefaults() {
    const emailInput = document.getElementById('login-email');
    const passwordInput = document.getElementById('login-password');
    const subtitle = document.getElementById('auth-subtitle');

    if (emailInput && !emailInput.value) {
      emailInput.value = 'demo@demo.com';
    }
    if (passwordInput && !passwordInput.value) {
      passwordInput.value = 'demo';
    }
    if (subtitle) {
      subtitle.textContent = 'Demo - click Sign in to explore';
    }
  }
});
