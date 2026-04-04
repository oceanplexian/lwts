/* fn2 Kanban — Auth Module (SPA) */

const Auth = {
  TOKEN_KEY: 'lwts_access_token',
  REFRESH_KEY: 'lwts_refresh_token',

  getAccessToken() {
    return localStorage.getItem(this.TOKEN_KEY);
  },

  getRefreshToken() {
    return localStorage.getItem(this.REFRESH_KEY);
  },

  setTokens(access, refresh) {
    localStorage.setItem(this.TOKEN_KEY, access);
    if (refresh) localStorage.setItem(this.REFRESH_KEY, refresh);
  },

  clearTokens() {
    localStorage.removeItem(this.TOKEN_KEY);
    localStorage.removeItem(this.REFRESH_KEY);
  },

  isAuthenticated() {
    return !!this.getAccessToken();
  },

  getUser() {
    const token = this.getAccessToken();
    if (!token) return null;
    try {
      const payload = JSON.parse(atob(token.split('.')[1]));
      return { id: payload.sub, email: payload.email, role: payload.role };
    } catch { return null; }
  },

  /* ── SPA layer switching ── */

  showAuth() {
    this.clearTokens();
    document.getElementById('app-board').style.display = 'none';
    const authEl = document.getElementById('app-auth');
    authEl.style.display = '';
    authEl.style.opacity = '0';
    requestAnimationFrame(() => {
      authEl.style.transition = 'opacity 0.3s ease';
      authEl.style.opacity = '1';
      setTimeout(() => { authEl.style.transition = ''; }, 350);
    });
    LoginPage.showView('login', false);
    LoginPage.revealCard();
  },

  showBoard(user) {
    const authEl = document.getElementById('app-auth');
    const boardEl = document.getElementById('app-board');

    // Fade out auth
    authEl.style.transition = 'opacity 0.25s ease';
    authEl.style.opacity = '0';
    setTimeout(() => {
      authEl.style.display = 'none';
      authEl.style.transition = '';

      // If user hasn't been welcomed, show the modal instantly before board fades in
      if (user && !user.welcomed && window.showWelcome) {
        window.showWelcome();
      }

      // Show and fade in board behind the modal
      boardEl.style.opacity = '0';
      boardEl.style.display = '';
      requestAnimationFrame(() => {
        boardEl.style.transition = 'opacity 0.3s ease';
        boardEl.style.opacity = '1';
        setTimeout(() => { boardEl.style.transition = ''; boardEl.style.opacity = ''; }, 350);
      });

      // Initialize board if not yet done
      if (window.initBoard && !window._boardInitialized) {
        window._boardInitialized = true;
        window.initBoard();
      }
    }, 250);
  },

  redirectToLogin() {
    this.showAuth();
  },

  checkAuth() {
    if (!this.isAuthenticated()) {
      this.redirectToLogin();
      return false;
    }
    return true;
  },

  _refreshing: null,

  async request(url, options = {}) {
    const headers = { 'Content-Type': 'application/json', ...options.headers };
    const token = this.getAccessToken();
    if (token) headers['Authorization'] = `Bearer ${token}`;

    let res = await fetch(url, { ...options, headers });

    if (res.status === 401 && this.getRefreshToken()) {
      res = await this._refreshAndRetry(url, { ...options, headers });
    }

    return res;
  },

  async _refreshAndRetry(url, options) {
    if (!this._refreshing) {
      this._refreshing = fetch('/api/auth/refresh', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refresh_token: this.getRefreshToken() })
      }).then(async (res) => {
        if (!res.ok) throw new Error('Refresh failed');
        const data = await res.json();
        this.setTokens(data.access_token, data.refresh_token);
        return data.access_token;
      }).catch(() => {
        this.clearTokens();
        return null;
      }).finally(() => {
        this._refreshing = null;
      });
    }

    const newToken = await this._refreshing;
    if (!newToken) return new Response(null, { status: 401 });

    options.headers['Authorization'] = `Bearer ${newToken}`;
    return fetch(url, options);
  }
};

/* ── Login page controller ── */
const LoginPage = {
  currentView: 'login',

  init() {
    this.form = {
      login: document.getElementById('login-form'),
      register: document.getElementById('register-form'),
      forgot: document.getElementById('forgot-form')
    };

    this.banner = document.getElementById('auth-banner');

    document.querySelectorAll('[data-auth-view]').forEach(el => {
      el.addEventListener('click', (e) => {
        e.preventDefault();
        this.showView(el.dataset.authView);
      });
    });

    document.querySelector('.auth-banner-close')?.addEventListener('click', () => {
      this.hideBanner();
    });

    this.form.login?.addEventListener('submit', (e) => { e.preventDefault(); this.handleLogin(); });
    this.form.register?.addEventListener('submit', (e) => { e.preventDefault(); this.handleRegister(); });
    this.form.forgot?.addEventListener('submit', (e) => { e.preventDefault(); this.handleForgot(); });

    document.querySelectorAll('.auth-input[data-validate]').forEach(input => {
      input.addEventListener('blur', () => this.validateField(input));
      input.addEventListener('input', () => {
        this.clearFieldError(input);
        if (input.id === 'reg-password') this.updateStrength(input.value);
      });
    });

    document.querySelectorAll('.auth-pw-toggle').forEach(btn => {
      btn.addEventListener('click', () => {
        const input = btn.previousElementSibling;
        input.type = input.type === 'password' ? 'text' : 'password';
      });
    });

    this._ready = true;
  },

  showInitialView(view) {
    this.showView(view, false);
    this.revealCard();
  },

  revealCard() {
    const card = document.querySelector('.auth-card');
    if (!card) return;
    const reveal = () => {
      card.style.transition = 'opacity 0.25s ease';
      card.style.opacity = '1';
      setTimeout(() => { card.style.transition = ''; }, 300);
    };
    if (document.fonts && document.fonts.ready) {
      document.fonts.ready.then(reveal);
    } else {
      reveal();
    }
  },

  showView(view, animate) {
    const card = document.querySelector('.auth-card');
    const from = card ? card.offsetHeight : 0;

    this.currentView = view;
    this.hideBanner();
    document.querySelectorAll('.auth-form-view').forEach(el => {
      el.classList.remove('active');
      el.style.display = '';
    });
    document.getElementById(`view-${view}`)?.classList.add('active');

    const subtitles = { login: 'Sign in to your workspace', register: 'Create your account', forgot: 'Reset your password' };
    document.getElementById('auth-subtitle').textContent = subtitles[view] || '';

    document.querySelectorAll('.auth-footer').forEach(f => f.style.display = '');
    document.querySelectorAll('.reg-hidden').forEach(el => el.classList.remove('reg-hidden'));

    const viewEl = document.getElementById(`view-${view}`);
    if (view === 'register' && viewEl?.classList.contains('auth-disabled')) {
      this.showBanner('Registration is currently disabled by the administrator.');
    }

    if (animate !== false && card && from > 0) {
      card.style.height = 'auto';
      const to = card.offsetHeight;
      if (from !== to) {
        card.style.height = from + 'px';
        card.offsetHeight;
        card.style.transition = 'height 0.25s ease';
        card.style.height = to + 'px';
        const done = () => { card.style.height = 'auto'; card.style.transition = ''; card.removeEventListener('transitionend', done); };
        card.addEventListener('transitionend', done);
      }
    }
  },

  validateField(input) {
    const rules = input.dataset.validate;
    const value = input.value.trim();
    let error = '';
    if (rules.includes('required') && !value) {
      const label = input.closest('.auth-field').querySelector('.auth-label').textContent;
      error = `${label} is required`;
    } else if (rules.includes('email') && value && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)) {
      error = 'Enter a valid email address';
    } else if (rules.includes('minlen')) {
      const min = parseInt(rules.match(/minlen:(\d+)/)?.[1] || '2');
      if (value && value.length < min) error = `Must be at least ${min} characters`;
    } else if (rules.includes('match')) {
      const target = document.getElementById(rules.match(/match:(\S+)/)?.[1]);
      if (target && value !== target.value) error = 'Passwords do not match';
    }
    this.setFieldError(input, error);
    return !error;
  },

  validateForm(formEl) {
    let valid = true;
    formEl.querySelectorAll('.auth-input[data-validate]').forEach(input => {
      if (!this.validateField(input)) valid = false;
    });
    return valid;
  },

  setFieldError(input, msg) {
    const errorEl = input.closest('.auth-field').querySelector('.auth-error');
    if (errorEl) errorEl.textContent = msg;
    input.classList.toggle('invalid', !!msg);
  },

  clearFieldError(input) { this.setFieldError(input, ''); },

  updateStrength(pw) {
    const meter = document.getElementById('pw-strength');
    const label = document.getElementById('pw-strength-label');
    if (!meter || !label) return;
    let score = 0;
    if (pw.length >= 8) score++;
    if (/[a-z]/.test(pw) && /[A-Z]/.test(pw)) score++;
    if (/\d/.test(pw)) score++;
    if (/[^a-zA-Z0-9]/.test(pw)) score++;
    const labels = ['', 'Weak', 'Fair', 'Good', 'Strong'];
    meter.dataset.level = pw ? score || 1 : 0;
    label.textContent = pw ? labels[score] || 'Weak' : '';
  },

  showBanner(msg) {
    this.banner.querySelector('.auth-banner-text').textContent = msg;
    this.banner.classList.add('visible');
  },

  hideBanner() { this.banner?.classList.remove('visible'); },

  setLoading(formEl, loading, text) {
    const btn = formEl.querySelector('.auth-btn');
    const btnText = btn.querySelector('.auth-btn-text');
    const spinner = btn.querySelector('.auth-spinner');
    formEl.querySelectorAll('.auth-input, .auth-btn').forEach(el => el.disabled = loading);
    if (spinner) spinner.style.display = loading ? 'block' : 'none';
    if (btnText) btnText.textContent = loading ? text : btnText.dataset.default;
  },

  async handleLogin() {
    if (!this.validateForm(this.form.login)) return;
    const email = document.getElementById('login-email').value.trim();
    const password = document.getElementById('login-password').value;
    this.setLoading(this.form.login, true, 'Signing in\u2026');
    try {
      const res = await fetch('/api/auth/login', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password })
      });
      const data = await res.json();
      if (!res.ok) {
        this.showBanner(res.status === 429 ? 'Too many attempts. Please try again later.' : (data.error || 'Invalid email or password'));
        return;
      }
      Auth.setTokens(data.access_token, data.refresh_token);
      Auth.showBoard(data.user);
    } catch { this.showBanner('Unable to connect. Please try again.'); }
    finally { this.setLoading(this.form.login, false); }
  },

  async handleRegister() {
    if (!this.validateForm(this.form.register)) return;
    const name = document.getElementById('reg-name').value.trim();
    const email = document.getElementById('reg-email').value.trim();
    const password = document.getElementById('reg-password').value;
    this.setLoading(this.form.register, true, 'Creating account\u2026');
    try {
      const res = await fetch('/api/auth/register', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, email, password })
      });
      const data = await res.json();
      if (!res.ok) {
        if (res.status === 409) this.showBanner('An account with this email already exists.');
        else if (res.status === 429) this.showBanner('Too many attempts. Please try again later.');
        else this.showBanner(data.error || 'Registration failed. Please try again.');
        return;
      }
      Auth.setTokens(data.access_token, data.refresh_token);
      Auth.showBoard(data.user);
    } catch { this.showBanner('Unable to connect. Please try again.'); }
    finally { this.setLoading(this.form.register, false); }
  },

  async handleForgot() {
    const emailInput = document.getElementById('forgot-email');
    if (!this.validateField(emailInput)) return;
    this.setLoading(this.form.forgot, true, 'Sending\u2026');
    try {
      await fetch('/api/auth/forgot-password', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: emailInput.value.trim() })
      });
    } catch { /* Always show success */ }
    finally { this.setLoading(this.form.forgot, false); }
    document.getElementById('forgot-form-fields').style.display = 'none';
    document.getElementById('forgot-submit-btn').style.display = 'none';
    document.getElementById('forgot-success').style.display = 'block';
  }
};

window.Auth = Auth;
window.LoginPage = LoginPage;
