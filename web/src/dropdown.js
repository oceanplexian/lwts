// fn2 Dropdown — custom styled dropdown with search

class FnDropdown {
  constructor(el, options = {}) {
    this.el = el;
    this.options = options.options || [];
    this.value = options.value || '';
    this.onChange = options.onChange || null;
    this.compact = options.compact || false;
    this.isOpen = false;
    this._onDocClick = this._onDocClick.bind(this);
    this._onKeyDown = this._onKeyDown.bind(this);
    this.render();
  }

  render() {
    this.el.classList.add('fn-dropdown');
    if (this.compact) this.el.classList.add('compact');
    this.el.innerHTML = '';

    const trigger = document.createElement('button');
    trigger.type = 'button';
    trigger.className = 'fn-dropdown-trigger';
    const selected = this.options.find(o => o.value === this.value);
    trigger.innerHTML = `
      <span>${this._renderLabel(selected)}</span>
      <svg class="fn-dropdown-chevron" viewBox="0 0 24 24"><polyline points="6 9 12 15 18 9"/></svg>
    `;
    trigger.addEventListener('click', (e) => { e.stopPropagation(); this.toggle(); });
    this.trigger = trigger;
    this.el.appendChild(trigger);

    const menu = document.createElement('div');
    menu.className = 'fn-dropdown-menu';

    const search = document.createElement('div');
    search.className = 'fn-dropdown-search';
    const searchInput = document.createElement('input');
    searchInput.type = 'text';
    searchInput.className = 'fn-dropdown-search-input';
    searchInput.placeholder = 'Search...';
    searchInput.addEventListener('input', () => this._filter(searchInput.value));
    searchInput.addEventListener('click', (e) => e.stopPropagation());
    search.appendChild(searchInput);
    this.searchInput = searchInput;
    menu.appendChild(search);

    const list = document.createElement('div');
    list.className = 'fn-dropdown-list';
    this.optionEls = [];
    this.options.forEach(opt => {
      const item = document.createElement('div');
      item.className = 'fn-dropdown-option' + (opt.value === this.value ? ' selected' : '');
      item.dataset.value = opt.value;
      item.innerHTML = `
        <span class="fn-dropdown-option-label">
          ${opt.dot ? `<span class="fn-dropdown-dot" style="background:${opt.dot}"></span>` : ''}
          ${this._esc(opt.label)}
        </span>
        <svg class="fn-dropdown-check" viewBox="0 0 24 24"><polyline points="20 6 9 17 4 12"/></svg>
      `;
      item.addEventListener('click', (e) => { e.stopPropagation(); this.setValue(opt.value); this.close(); });
      list.appendChild(item);
      this.optionEls.push({ el: item, opt });
    });
    menu.appendChild(list);

    this.menu = menu;
    this.el.appendChild(menu);
  }

  _renderLabel(opt) {
    if (!opt) return '';
    if (opt.dot) return `<span class="fn-dropdown-dot" style="background:${opt.dot}"></span> ${this._esc(opt.label)}`;
    return this._esc(opt.label);
  }

  _filter(query) {
    const q = query.toLowerCase().trim();
    this.optionEls.forEach(({ el, opt }) => { el.style.display = (!q || opt.label.toLowerCase().includes(q)) ? '' : 'none'; });
  }

  toggle() { this.isOpen ? this.close() : this.open(); }

  open() {
    this.isOpen = true;
    this.trigger.classList.add('open');
    this.menu.classList.add('visible');
    this.searchInput.value = '';
    this._filter('');
    this._autoFlip();
    setTimeout(() => this.searchInput.focus(), 30);
    document.addEventListener('click', this._onDocClick, true);
    document.addEventListener('keydown', this._onKeyDown, true);
  }

  _autoFlip() {
    const triggerRect = this.trigger.getBoundingClientRect();
    const menuHeight = this.menu.scrollHeight || 260;
    const spaceBelow = window.innerHeight - triggerRect.bottom;
    const spaceAbove = triggerRect.top;
    if (spaceBelow < menuHeight + 8 && spaceAbove > spaceBelow) this.el.classList.add('drop-up');
    else this.el.classList.remove('drop-up');
  }

  close() {
    this.isOpen = false;
    this.trigger.classList.remove('open');
    this.menu.classList.remove('visible');
    document.removeEventListener('click', this._onDocClick, true);
    document.removeEventListener('keydown', this._onKeyDown, true);
  }

  _onDocClick(e) { if (!this.el.contains(e.target)) this.close(); }
  _onKeyDown(e) { if (e.key === 'Escape') { this.close(); e.stopPropagation(); } }

  setValue(val, silent) {
    this.value = val;
    const selected = this.options.find(o => o.value === val);
    const label = this.trigger.querySelector('span');
    label.innerHTML = this._renderLabel(selected);
    this.optionEls.forEach(({ el, opt }) => { el.classList.toggle('selected', opt.value === val); });
    if (!silent && this.onChange) this.onChange(val);
  }

  getValue() { return this.value; }

  setOptions(newOptions) { this.options = newOptions; this.render(); }

  _esc(s) { const d = document.createElement('div'); d.textContent = s; return d.innerHTML; }
}

window.FnDropdown = FnDropdown;
