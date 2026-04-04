// Theme toggle — matches the app's body.light-theme class
function toggleTheme() {
  document.body.classList.toggle('light-theme');
  localStorage.setItem('lwts-theme', document.body.classList.contains('light-theme') ? 'light' : 'dark');
}
// Restore saved preference
if (localStorage.getItem('lwts-theme') === 'light') {
  document.body.classList.add('light-theme');
}

// Tab switching
document.querySelectorAll('.tab').forEach(tab => {
  tab.addEventListener('click', () => {
    const target = tab.dataset.tab;
    document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
    document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
    tab.classList.add('active');
    document.getElementById('tab-' + target).classList.add('active');
  });
});

// Copy code
function copyCode(btn) {
  const block = btn.closest('.code-block');
  const code = block.querySelector('code').textContent;
  navigator.clipboard.writeText(code).then(() => {
    const orig = btn.textContent;
    btn.textContent = 'Copied!';
    btn.style.color = '#16a34a';
    setTimeout(() => { btn.textContent = orig; btn.style.color = ''; }, 2000);
  });
}

// Intersection observer
if (window.matchMedia('(prefers-reduced-motion: no-preference)').matches) {
  const observer = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
      if (entry.isIntersecting) {
        entry.target.style.animationPlayState = 'running';
        observer.unobserve(entry.target);
      }
    });
  }, { threshold: 0.08 });
  document.querySelectorAll('.feature-card, .arch-card, .demo-epic-lane .epic-card, .demo-list-row').forEach(el => {
    el.style.animationPlayState = 'paused';
    observer.observe(el);
  });
}

// Mobile nav
document.querySelectorAll('.nav-links a').forEach(link => {
  link.addEventListener('click', () => {
    document.querySelector('.nav-links').classList.remove('open');
  });
});

// ──────────────────────────────────────────────
// Demo board — HTML5 drag-and-drop
// Copied from kanban.js: onDragStart, onDragEnd, onDragOver, onDragLeave, onDrop
// ──────────────────────────────────────────────
(function() {
  let dragEl = null;
  let dropTargetCard = null;
  let dropTargetCol = null;

  function clearDropTarget() {
    document.querySelectorAll('.mini-card.drop-target').forEach(el => el.classList.remove('drop-target'));
    dropTargetCard = null;
    dropTargetCol = null;
  }

  function updateCounts() {
    document.querySelectorAll('.mini-col').forEach(col => {
      const count = col.querySelectorAll('.mini-col-body > .mini-card').length;
      const badge = col.querySelector('.mini-col-count');
      if (badge) badge.textContent = count;
    });
  }

  // dragstart
  document.querySelectorAll('.mini-card[draggable]').forEach(card => {
    card.addEventListener('dragstart', function(e) {
      dragEl = this;
      this.classList.add('dragging');
      e.dataTransfer.effectAllowed = 'move';
      e.dataTransfer.setData('text/plain', '');
    });

    card.addEventListener('dragend', function() {
      if (dragEl) dragEl.classList.remove('dragging');
      clearDropTarget();
      document.querySelectorAll('.mini-col-body.drag-over').forEach(el => el.classList.remove('drag-over'));
      dragEl = null;
    });
  });

  // dragover / dragleave / drop on column bodies
  document.querySelectorAll('.mini-col-body').forEach(body => {
    body.addEventListener('dragover', function(e) {
      e.preventDefault();
      e.dataTransfer.dropEffect = 'move';
      if (!dragEl) return;

      this.classList.add('drag-over');
      dropTargetCol = this;

      // Find card under cursor
      const cards = this.querySelectorAll('.mini-card:not(.dragging)');
      let found = null;
      for (const card of cards) {
        const rect = card.getBoundingClientRect();
        if (e.clientY >= rect.top && e.clientY <= rect.bottom) {
          found = card;
          break;
        }
      }

      if (found !== dropTargetCard) {
        if (dropTargetCard) dropTargetCard.classList.remove('drop-target');
        dropTargetCard = found;
        if (dropTargetCard) dropTargetCard.classList.add('drop-target');
      }
    });

    body.addEventListener('dragleave', function(e) {
      if (!this.contains(e.relatedTarget)) {
        this.classList.remove('drag-over');
        clearDropTarget();
      }
    });

    body.addEventListener('drop', function(e) {
      e.preventDefault();
      this.classList.remove('drag-over');
      if (!dragEl) { clearDropTarget(); return; }

      const movedEl = dragEl;

      // Surgical DOM move — same as kanban.js
      movedEl.remove();
      if (dropTargetCard && dropTargetCard !== movedEl) {
        this.insertBefore(movedEl, dropTargetCard);
      } else {
        this.appendChild(movedEl);
      }

      // Remove dragging class, null out so dragend is a no-op
      movedEl.classList.remove('dragging');
      dragEl = null;
      clearDropTarget();

      // Animate: start from dragging state, spring into place
      // Exact copy of kanban.js lines 464-478
      movedEl.style.opacity = '0.3';
      movedEl.style.transform = 'scale(0.95) translateY(-4px)';
      void movedEl.offsetWidth; // force reflow
      movedEl.style.transition = 'opacity 0.25s ease, transform 0.4s cubic-bezier(0.34, 1.56, 0.64, 1)';
      movedEl.style.opacity = '1';
      movedEl.style.transform = 'scale(1) translateY(0)';
      movedEl.addEventListener('transitionend', function handler(ev) {
        if (ev.propertyName === 'transform') {
          movedEl.style.transition = '';
          movedEl.style.opacity = '';
          movedEl.style.transform = '';
          movedEl.removeEventListener('transitionend', handler);
        }
      });

      updateCounts();
    });
  });

  updateCounts();
})();

// ──────────────────────────────────────────────
// Detail modal — click card to open, same layout as the real app
// ──────────────────────────────────────────────
var CARD_DATA = {
  'LWTS-2': {
    title: 'Investigate ES shard rebalancing', tag: 'INFRA', tagClass: 'tag-orange',
    status: 'Backlog', type: 'Task', priority: 'High', priorityColor: '#f44336',
    reporter: 'Sam Rivera', assignee: 'Sam Rivera', assigneeColor: '#5c6bc0',
    points: 3, due: 'Aug 20', created: 'Mar 15, 2026',
    desc: 'Cluster went yellow after the last force-merge. West region shard 0 replica couldn\'t allocate due to disk watermark on elasticsearch-west-1. Need to investigate whether we should add a third node per region or just increase disk.',
    comments: [
      { author: 'Alex Kim', color: '#ef5350', time: 'Mar 16 at 9:22 AM', text: 'Disk usage on west-1 is at 88%. We have about 60GB headroom before hitting the 90% watermark again.' },
      { author: 'Sam Rivera', color: '#5c6bc0', time: 'Mar 16 at 10:05 AM', text: 'Going to run a reconcile:compare job first to see how many orphaned docs we have. Might free up space without adding hardware.' }
    ]
  },
  'LWTS-3': {
    title: 'Mobile responsive layout', tag: 'FEATURE', tagClass: 'tag-blue',
    status: 'Backlog', type: 'Feature', priority: 'Medium', priorityColor: '#fb8c00',
    reporter: 'Jane Doe', assignee: 'Jane Doe', assigneeColor: '#26a69a',
    points: 8, due: 'Nov 10', created: 'Mar 20, 2026',
    desc: 'Single-column view with tab switching between Backlog/To Do/In Progress/Done on screens under 768px. Bottom sheet for board picker. Touch-friendly tap targets (44px minimum). Card detail should go full-screen on mobile with sidebar below content.',
    comments: [
      { author: 'Jane Doe', color: '#26a69a', time: 'Mar 21 at 3:30 PM', text: 'Started with the column tabs. Using a horizontal scroll container for now but might switch to proper tab switching if performance is better.' }
    ]
  },
  'LWTS-7': {
    title: 'Deploy polygon streamer v2', tag: 'FIX', tagClass: 'tag-green',
    status: 'To Do', type: 'Task', priority: 'Medium', priorityColor: '#fb8c00',
    reporter: 'Alex Kim', assignee: 'Alex Kim', assigneeColor: '#ef5350',
    points: 2, due: 'Nov 26', created: 'Mar 22, 2026',
    desc: 'The v2 streamer uses the new websocket multiplexing instead of one connection per symbol. Should reduce connection count from ~4000 to ~50. Needs to be deployed during off-market hours since it requires a full restart.',
    comments: [
      { author: 'Sam Rivera', color: '#5c6bc0', time: 'Mar 23 at 8:15 AM', text: 'Can we do a canary deploy to west first? Want to compare latency numbers before cutting over east.' },
      { author: 'Alex Kim', color: '#ef5350', time: 'Mar 23 at 8:44 AM', text: 'Yeah, I\'ll set up a parallel deployment on west. Will leave the old streamer running until we validate.' }
    ]
  },
  'LWTS-26': {
    title: 'Fix webhook retry backoff', tag: 'FIX', tagClass: 'tag-green',
    status: 'To Do', type: 'Bug', priority: 'High', priorityColor: '#f44336',
    reporter: 'Sam Rivera', assignee: 'Sam Rivera', assigneeColor: '#5c6bc0',
    points: 3, due: 'Today', created: 'Apr 1, 2026',
    desc: 'Webhook deliveries are retrying immediately instead of using exponential backoff. The retry delay calculation is using milliseconds but the sleep function expects seconds. Causing rate limit hits on downstream services.',
    comments: [
      { author: 'Sam Rivera', color: '#5c6bc0', time: 'Apr 1 at 11:30 AM', text: 'Found it — dispatcher.go line 142. time.Duration is in nanoseconds, we\'re passing raw integers. Need to multiply by time.Second.' },
      { author: 'Jane Doe', color: '#26a69a', time: 'Apr 1 at 11:45 AM', text: 'Good catch. Also noticed the semaphore isn\'t being released on timeout errors. That might explain why deliveries queue up.' }
    ]
  },
  'LWTS-4': {
    title: 'Fix SSE heartbeat timeout on slow connections', tag: 'BUG', tagClass: 'tag-red',
    status: 'In Progress', type: 'Bug', priority: 'Highest', priorityColor: '#f44336',
    reporter: 'You', assignee: 'You', assigneeColor: '#42a5f5',
    points: 5, due: 'Apr 1', created: 'Apr 1, 2026',
    desc: 'SSE connections are dropping after 30s on slow networks. The heartbeat interval is 30s but the Envoy idle timeout is also 30s, so if a heartbeat is even slightly delayed the connection gets killed. Users see "Reconnecting..." flash repeatedly.',
    comments: [
      { author: 'Sam Rivera', color: '#5c6bc0', time: 'Apr 3 at 1:05 AM', text: 'Confirmed — seeing this in west region too.' },
      { author: 'You', color: '#42a5f5', time: 'Apr 3 at 1:05 AM', text: 'Root cause is Envoy BackendTrafficPolicy default. Need to bump to 600s.' }
    ]
  },
  'LWTS-5': {
    title: 'Add dark/light theme toggle', tag: 'FEATURE', tagClass: 'tag-blue',
    status: 'In Progress', type: 'Feature', priority: 'Medium', priorityColor: '#fb8c00',
    reporter: 'Jane Doe', assignee: 'Jane Doe', assigneeColor: '#26a69a',
    points: 5, due: 'Nov 30', created: 'Mar 25, 2026',
    desc: 'Add a toggle in settings to switch between dark and light themes. Store preference in localStorage and apply via CSS custom properties. Need to define the full light-mode palette — inverting the dark values won\'t work, need proper contrast ratios.',
    comments: [
      { author: 'Jane Doe', color: '#26a69a', time: 'Mar 26 at 4:12 PM', text: 'Light mode palette is done. 48 variables total. All pass WCAG AA contrast against the #f5f5f5 background.' },
      { author: 'Alex Kim', color: '#ef5350', time: 'Mar 27 at 9:00 AM', text: 'Make sure the toggle respects prefers-color-scheme on first load before any localStorage value is set.' }
    ]
  },
  'LWTS-8': {
    title: 'Add FRED series to macro dashboard', tag: 'FEATURE', tagClass: 'tag-blue',
    status: 'In Progress', type: 'Feature', priority: 'Low', priorityColor: '#4caf50',
    reporter: 'Alex Kim', assignee: 'Alex Kim', assigneeColor: '#ef5350',
    points: 3, due: 'Apr 1', created: 'Mar 28, 2026',
    desc: 'Pull in the 43 FRED economic series (unemployment, CPI, GDP, etc.) and display them in a new macro tab on the dashboard. The stock-server already has /api/v1/fred/refresh and /api/v1/fred/status endpoints. Just need the frontend chart components.',
    comments: [
      { author: 'Alex Kim', color: '#ef5350', time: 'Mar 29 at 2:30 PM', text: 'Charts are rendering. Using simple SVG line paths instead of a charting library to keep the zero-dependency philosophy.' }
    ]
  },
  'LWTS-9': {
    title: 'JWT token refresh race condition', tag: 'FIX', tagClass: 'tag-green',
    status: 'Done', type: 'Bug', priority: 'High', priorityColor: '#f44336',
    reporter: 'Alex Kim', assignee: 'Alex Kim', assigneeColor: '#ef5350',
    points: 2, due: 'Mar 28', created: 'Mar 18, 2026',
    desc: 'When multiple API calls hit a 401 simultaneously, each one triggers a token refresh. The second refresh invalidates the first\'s new token (JTI rotation), causing a cascade of 401s. Fixed by adding a queue that holds concurrent requests until the first refresh completes.',
    comments: [
      { author: 'Alex Kim', color: '#ef5350', time: 'Mar 19 at 10:00 AM', text: 'Implemented a promise-based queue in api.js. First 401 triggers refresh, subsequent ones wait on the same promise. All queued requests replay with the new token.' },
      { author: 'Sam Rivera', color: '#5c6bc0', time: 'Mar 19 at 11:30 AM', text: 'Tested with 10 concurrent fetches on an expired token. All 10 succeed now with only 1 refresh call. Merging.' }
    ]
  }
};

function openDetail(key) {
  var d = CARD_DATA[key];
  if (!d) return;
  document.getElementById('detail-key').textContent = key;
  document.getElementById('detail-header-title').textContent = d.title;
  document.getElementById('detail-title').value = d.title;

  // Description
  var descEl = document.getElementById('detail-desc');
  if (d.desc) {
    descEl.innerHTML = d.desc;
  } else {
    descEl.innerHTML = '<span class="detail-desc-placeholder">Add a description...</span>';
  }

  // Sidebar fields
  document.getElementById('detail-status').innerHTML = '<span class="detail-field-dot" style="background:' + d.priorityColor + '"></span> ' + d.status;
  document.getElementById('detail-type').innerHTML = '<span class="detail-field-dot" style="background:' + d.priorityColor + '"></span> ' + d.type;
  document.getElementById('detail-priority').innerHTML = '<span class="detail-field-dot" style="background:' + d.priorityColor + '"></span> ' + d.priority;
  document.getElementById('detail-reporter').textContent = d.reporter;
  document.getElementById('detail-assignee').textContent = d.assignee;
  document.getElementById('detail-points').value = d.points;
  document.getElementById('detail-due').innerHTML = '<span style="color:' + (d.due === 'Today' ? '#fb8c00' : d.due === 'Apr 1' ? '#f44336' : 'inherit') + '">' + d.due + '</span>';
  document.getElementById('detail-meta').textContent = 'Created ' + d.created;

  // Comments
  var cl = document.getElementById('detail-comments-list');
  cl.innerHTML = '';
  d.comments.forEach(function(c) {
    var initials = c.author.split(' ').map(function(w){ return w[0]; }).join('');
    cl.innerHTML += '<div class="detail-comment">' +
      '<div class="detail-comment-avatar" style="background:' + c.color + '">' + initials + '</div>' +
      '<div class="detail-comment-body">' +
        '<div class="detail-comment-header">' +
          '<span class="detail-comment-author">' + c.author + '</span>' +
          '<span class="detail-comment-time">' + c.time + '</span>' +
        '</div>' +
        '<div class="detail-comment-text">' + c.text + '</div>' +
        '<div class="detail-comment-actions">' +
          '<button class="detail-comment-action">Edit</button>' +
          '<button class="detail-comment-action delete">Delete</button>' +
        '</div>' +
      '</div>' +
    '</div>';
  });

  document.getElementById('detail-overlay').classList.add('active');
}

function closeDetail() {
  document.getElementById('detail-overlay').classList.remove('active');
}

// Close on overlay click
document.getElementById('detail-overlay').addEventListener('click', function(e) {
  if (e.target === this) closeDetail();
});

// Close on Escape
document.addEventListener('keydown', function(e) {
  if (e.key === 'Escape') closeDetail();
});

// ──────────────────────────────────────────────
// Search demo — auto-types "heartbeat" and reveals results on scroll
// ──────────────────────────────────────────────
(function() {
  var searchDemo = document.getElementById('search-demo');
  if (!searchDemo) return;

  var typed = document.getElementById('search-typed');
  var cursor = document.getElementById('search-cursor');
  var results = document.getElementById('search-results');
  var clear = document.getElementById('search-clear');
  var word = 'heartbeat';
  var items = [document.getElementById('sr-0'), document.getElementById('sr-1'), document.getElementById('sr-2')];
  var running = false;
  var hasPlayed = false;
  var resetTimer = null;

  function reset() {
    typed.textContent = '';
    results.style.display = 'none';
    clear.style.display = 'none';
    items.forEach(function(el) { el.style.display = 'none'; });
    running = false;
    hasPlayed = false;
  }

  function play() {
    if (running) return;
    running = true;
    hasPlayed = true;
    typed.textContent = '';
    results.style.display = 'none';
    clear.style.display = 'none';
    items.forEach(function(el) { el.style.display = 'none'; });

    var i = 0;
    function typeNext() {
      if (!running) return;
      if (i < word.length) {
        typed.textContent += word[i];
        i++;
        // Show results dropdown after 3rd char
        if (i === 3) {
          results.style.display = 'block';
          clear.style.display = 'block';
        }
        // Reveal result items staggered
        if (i === 4 && items[0]) items[0].style.display = 'flex';
        if (i === 6 && items[1]) items[1].style.display = 'flex';
        if (i === 8 && items[2]) items[2].style.display = 'flex';
        setTimeout(typeNext, 80 + Math.random() * 60);
      } else {
        // Done typing — hold for a bit then reset and replay
        resetTimer = setTimeout(function() {
          if (!running) return;
          reset();
          setTimeout(function() { play(); }, 800);
        }, 4000);
      }
    }
    setTimeout(typeNext, 600);
  }

  // Intersection observer — play when visible, reset when not
  var obs = new IntersectionObserver(function(entries) {
    entries.forEach(function(entry) {
      if (entry.isIntersecting) {
        play();
      } else {
        clearTimeout(resetTimer);
        running = false;
        // Keep final state visible if it played, reset if scrolling far
        setTimeout(function() { if (!running) reset(); }, 100);
      }
    });
  }, { threshold: 0.3 });
  obs.observe(searchDemo);
})();

// Wire up card clicks — don't open on drag
document.querySelectorAll('.mini-card[draggable]').forEach(function(card) {
  var didDrag = false;
  card.addEventListener('dragstart', function() { didDrag = true; });
  card.addEventListener('click', function(e) {
    if (didDrag) { didDrag = false; return; }
    var keyEl = this.querySelector('.mini-card-key');
    if (keyEl) openDetail(keyEl.textContent);
  });
  card.addEventListener('dragend', function() {
    // Reset after a tick so click doesn't fire
    setTimeout(function() { didDrag = false; }, 0);
  });
});
