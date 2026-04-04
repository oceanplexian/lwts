// LWTS Kanban — Vite entry point (SPA)
// CSS is loaded via <link> tags in index.html to prevent FOUC.

// JS modules — loaded in dependency order
import './auth.js';
import './api.js';
import './dropdown.js';
import './editor.js';
import './boardstream.js';
import './presence.js';
import './kanban.js';
import './settings.js';
import './listview.js';
import './features.js';
import './subtasks.js';
import './a11y.js';

document.addEventListener('DOMContentLoaded', () => {
  const LP = window.LoginPage;
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
    fetch('/api/auth/me', { headers: { 'Authorization': 'Bearer ' + window.Auth.getAccessToken() } })
      .then(res => {
        if (res.ok) {
          return res.json().then(user => {
            // Show welcome modal before board if user hasn't been welcomed
            if (!user.welcomed && window.showWelcome) {
              window.showWelcome();
            }
            document.getElementById('app-board').style.display = '';
            if (window.initBoard && !window._boardInitialized) {
              window._boardInitialized = true;
              window.initBoard();
            }
          });
        } else {
          window.Auth.clearTokens();
          showAuthFlow();
        }
      })
      .catch(() => {
        window.Auth.clearTokens();
        showAuthFlow();
      });
    return;
  }

  showAuthFlow();

  function showAuthFlow() {
    document.getElementById('app-auth').style.display = '';

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
