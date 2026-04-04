// LWTS Markdown Editor — plain text with syntax shortcuts

class FnEditor {
  constructor(el, options = {}) {
    this.el = el;
    this.compact = options.compact || false;
    this.placeholder = options.placeholder || '';
    this.onChange = options.onChange || null;
    this.onSubmit = options.onSubmit || null;
    this._markdown = '';
    this.render();
  }

  render() {
    this.el.classList.add('lwts-editor');
    if (this.compact) this.el.classList.add('compact');
    this.el.innerHTML = '';

    const toolbar = document.createElement('div');
    toolbar.className = 'lwts-editor-toolbar';

    const buttons = [
      { cmd: 'bold', icon: '<b>B</b>', title: 'Bold (Ctrl+B)', wrap: ['**', '**'] },
      { cmd: 'italic', icon: '<i>I</i>', title: 'Italic (Ctrl+I)', wrap: ['*', '*'] },
      { sep: true },
      { cmd: 'bullet', icon: '<svg viewBox="0 0 24 24"><line x1="8" y1="6" x2="21" y2="6"/><line x1="8" y1="12" x2="21" y2="12"/><line x1="8" y1="18" x2="21" y2="18"/><circle cx="4" cy="6" r="1" fill="currentColor" stroke="none"/><circle cx="4" cy="12" r="1" fill="currentColor" stroke="none"/><circle cx="4" cy="18" r="1" fill="currentColor" stroke="none"/></svg>', title: 'Bullet list', prefix: '- ' },
      { sep: true },
      { cmd: 'code', icon: '<svg viewBox="0 0 24 24"><polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/></svg>', title: 'Code', wrap: ['`', '`'] },
      { cmd: 'heading', icon: 'H', title: 'Heading', prefix: '## ' },
    ];

    buttons.forEach(b => {
      if (b.sep) {
        const sep = document.createElement('div');
        sep.className = 'lwts-editor-sep';
        toolbar.appendChild(sep);
        return;
      }
      const btn = document.createElement('button');
      btn.type = 'button';
      btn.className = 'lwts-editor-btn';
      btn.innerHTML = b.icon;
      btn.title = b.title || '';
      btn.addEventListener('mousedown', (e) => {
        e.preventDefault();
        if (b.wrap) this._wrapSelection(b.wrap[0], b.wrap[1]);
        else if (b.prefix) this._prefixLine(b.prefix);
      });
      toolbar.appendChild(btn);
    });
    this.toolbar = toolbar;
    this.el.appendChild(toolbar);

    const body = document.createElement('textarea');
    body.className = 'lwts-editor-body';
    body.placeholder = this.placeholder;
    body.addEventListener('input', () => this._emitChange());
    body.addEventListener('keydown', (e) => {
      // Ctrl/Cmd+B = bold, Ctrl/Cmd+I = italic
      if ((e.ctrlKey || e.metaKey) && e.key === 'b') { e.preventDefault(); this._wrapSelection('**', '**'); }
      if ((e.ctrlKey || e.metaKey) && e.key === 'i') { e.preventDefault(); this._wrapSelection('*', '*'); }
      // Enter in compact mode = submit
      if (e.key === 'Enter' && !e.shiftKey && this.compact && this.onSubmit) { e.preventDefault(); this.onSubmit(); }
      // Tab = insert 2 spaces
      if (e.key === 'Tab') { e.preventDefault(); this._insertAtCursor('  '); }
    });
    this.body = body;
    this.el.appendChild(body);

    // Auto-resize
    body.addEventListener('input', () => this._autoResize());
  }

  _wrapSelection(before, after) {
    const ta = this.body;
    const start = ta.selectionStart;
    const end = ta.selectionEnd;
    const text = ta.value;
    const selected = text.slice(start, end);

    // If already wrapped, unwrap
    if (start >= before.length && text.slice(start - before.length, start) === before &&
        text.slice(end, end + after.length) === after) {
      ta.value = text.slice(0, start - before.length) + selected + text.slice(end + after.length);
      ta.selectionStart = start - before.length;
      ta.selectionEnd = end - before.length;
    } else {
      ta.value = text.slice(0, start) + before + selected + after + text.slice(end);
      ta.selectionStart = start + before.length;
      ta.selectionEnd = end + before.length;
    }
    ta.focus();
    this._emitChange();
  }

  _prefixLine(prefix) {
    const ta = this.body;
    const start = ta.selectionStart;
    const text = ta.value;

    // Find start of current line
    const lineStart = text.lastIndexOf('\n', start - 1) + 1;
    const lineEnd = text.indexOf('\n', start);
    const line = text.slice(lineStart, lineEnd === -1 ? text.length : lineEnd);

    // Toggle prefix
    if (line.startsWith(prefix)) {
      ta.value = text.slice(0, lineStart) + line.slice(prefix.length) + text.slice(lineEnd === -1 ? text.length : lineEnd);
      ta.selectionStart = ta.selectionEnd = start - prefix.length;
    } else {
      ta.value = text.slice(0, lineStart) + prefix + text.slice(lineStart);
      ta.selectionStart = ta.selectionEnd = start + prefix.length;
    }
    ta.focus();
    this._emitChange();
  }

  _insertAtCursor(text) {
    const ta = this.body;
    const start = ta.selectionStart;
    ta.value = ta.value.slice(0, start) + text + ta.value.slice(ta.selectionEnd);
    ta.selectionStart = ta.selectionEnd = start + text.length;
    this._emitChange();
  }

  _autoResize() {
    const ta = this.body;
    ta.style.height = 'auto';
    ta.style.height = ta.scrollHeight + 'px';
  }

  _emitChange() { if (this.onChange) this.onChange(this.getMarkdown()); }

  getMarkdown() { return this.body.value; }

  setMarkdown(md) {
    this._markdown = md || '';
    this.body.value = this._markdown;
    this._autoResize();
  }

  getHTML() { return this.body.value; }
  setHTML(html) { this.body.value = html || ''; }

  focus() { this.body.focus(); }
  clear() { this.body.value = ''; this._markdown = ''; }
}

window.FnEditor = FnEditor;
