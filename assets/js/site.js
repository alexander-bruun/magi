// site.js - common site scripts moved from inline templates
// - titleHandler: tiny helper to set the document title
// - sidebar collapse persistence and toggle handling
// - HTMX-aware nav active state syncing

(function () {
  // Run initialization after DOM is ready
  function init() {
    // Sidebar collapsed persistence
    try {
      const STORAGE_KEY = '__FRANKEN_SIDEBAR_COLLAPSED__';
      const body = document.body;
      const toggle = document.getElementById('sidebar-toggle');
      const sidebar = document.getElementById('sidebar');
      const backdrop = document.getElementById('sidebar-backdrop');
      const SMALL_QUERY = '(max-width: 768px)';
      const mq = window.matchMedia ? window.matchMedia(SMALL_QUERY) : null;

      function applyCollapsed(collapsed) {
        if (collapsed) {
          body.classList.add('sidebar-collapsed');
          toggle && toggle.setAttribute('aria-expanded', 'false');
        } else {
          body.classList.remove('sidebar-collapsed');
          toggle && toggle.setAttribute('aria-expanded', 'true');
        }
      }

      const stored = localStorage.getItem(STORAGE_KEY);
      if (stored === '1' || stored === '0') {
        applyCollapsed(stored === '1');
      } else {
        applyCollapsed(mq ? mq.matches : false);
      }

      function openMobileSidebar() {
        if (!sidebar) return;
        body.classList.add('sidebar-open');
        toggle && toggle.setAttribute('aria-expanded', 'true');
        if (backdrop) backdrop.removeAttribute('hidden');
      }

      function closeMobileSidebar() {
        body.classList.remove('sidebar-open');
        toggle && toggle.setAttribute('aria-expanded', 'false');
        if (backdrop) backdrop.setAttribute('hidden', '');
      }

      function handleToggleClick() {
        if (mq && mq.matches) {
          // Mobile: open/close off-canvas, do not store collapsed state
          if (body.classList.contains('sidebar-open')) {
            closeMobileSidebar();
          } else {
            openMobileSidebar();
          }
        } else {
          // Desktop: collapse/expand and persist
          const isCollapsed = body.classList.toggle('sidebar-collapsed');
          localStorage.setItem(STORAGE_KEY, isCollapsed ? '1' : '0');
          toggle && toggle.setAttribute('aria-expanded', isCollapsed ? 'false' : 'true');
        }
      }

      toggle && toggle.addEventListener('click', handleToggleClick);

      // Close sidebar when clicking backdrop or a nav link on mobile
      backdrop && backdrop.addEventListener('click', closeMobileSidebar);
      document.addEventListener('click', function (e) {
        if (!(mq && mq.matches)) return;
        const link = e.target.closest && e.target.closest('.sidebar a[href]');
        if (link) {
          closeMobileSidebar();
        }
      });

      // ESC to close on mobile
      document.addEventListener('keydown', function (e) {
        if (e.key === 'Escape' && mq && mq.matches && body.classList.contains('sidebar-open')) {
          closeMobileSidebar();
        }
      });

      function handleMqChange(e) {
        const storedPref = localStorage.getItem(STORAGE_KEY);
        if (storedPref === null) {
          applyCollapsed(e.matches);
        }
        // Close mobile drawer when switching to desktop
        if (!e.matches) {
          closeMobileSidebar();
        }
      }

      if (mq && typeof mq.addEventListener === 'function') {
        mq.addEventListener('change', handleMqChange);
      } else if (mq && typeof mq.addListener === 'function') {
        mq.addListener(handleMqChange);
      }
    } catch (e) {
      // don't break the site if JS errors occur
      console.error('site.js init error', e);
    }

    // HTMX-aware navigation active syncing
    try {
      function normalizePath(path) {
        try {
          return new URL(path, location.origin).pathname.replace(/\/+$/g, '/') || '/';
        } catch (e) {
          return '/';
        }
      }

      function updateActiveNav() {
        const current = normalizePath(location.pathname);
        const anchors = Array.from(document.querySelectorAll('.uk-nav a[href]'))
          .filter(a => a.getAttribute('href'))
          .map(a => ({ a: a, path: normalizePath(a.getAttribute('href')) }));

        document.querySelectorAll('.uk-nav li.uk-active').forEach(li => li.classList.remove('uk-active'));

        const groups = new Map();
        anchors.forEach(({ a, path }) => {
          const li = a.closest('li');
          if (!li) return;
          const group = li.closest('.uk-nav');
          if (!group) return;
          const cur = groups.get(group) || null;
          const matches = (path === '/') ? (current === '/') : (current === path || current.startsWith(path + '/'));
          if (!matches) return;
          const score = path.length;
          if (!cur || score > cur.score) {
            groups.set(group, { li, score });
          }
        });

        groups.forEach(({ li }) => li.classList.add('uk-active'));
      }

      if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', updateActiveNav);
      } else {
        updateActiveNav();
      }

      document.addEventListener('htmx:afterSwap', updateActiveNav);
      document.addEventListener('htmx:afterSettle', updateActiveNav);
      window.addEventListener('popstate', updateActiveNav);

      document.addEventListener('click', function (ev) {
        const a = ev.target.closest && ev.target.closest('a[href]');
        if (!a) return;
        if (a.target === '_blank') return;
        const href = a.getAttribute('href');
        if (!href || (href.startsWith('http') && new URL(href).origin !== location.origin)) return;
        setTimeout(updateActiveNav, 10);
      });
    } catch (e) {
      console.error('site.js nav sync error', e);
    }
    try {
      function syncTagFormSortOrder() {
        try {
          const form = document.getElementById('tag-filter-form'); if (!form) return; const params = new URLSearchParams(window.location.search); let sort = params.get('sort') || ''; let order = params.get('order') || ''; let tagMode = (params.get('tag_mode') || '').toLowerCase();

          // Prefer explicit select control value if present
          const select = document.getElementById('manga-sort-select');
          if (select && select.value) {
            sort = select.value;
          }

          if (tagMode !== 'any' && tagMode !== 'all') tagMode = 'all';
          const sortInput = form.querySelector('input[name="sort"]'); const orderInput = form.querySelector('input[name="order"]'); const modeInput = form.querySelector('input[name="tag_mode"]'); if (sort && sortInput) sortInput.value = sort; if (order && orderInput) orderInput.value = order; if (modeInput) modeInput.value = tagMode; const toggle = document.getElementById('tag-mode-toggle'); if (toggle) { toggle.setAttribute('data-mode', tagMode); toggle.textContent = (tagMode === 'any') ? 'Any' : 'All'; }
        } catch (e) { }
      }

      if (document.readyState === 'loading') { document.addEventListener('DOMContentLoaded', syncTagFormSortOrder); } else { syncTagFormSortOrder(); }
      window.addEventListener('popstate', syncTagFormSortOrder);
      document.addEventListener('htmx:afterSwap', function (e) {
        if (e.detail && e.detail.target) {
          var tid = e.detail.target.id;
          if (tid === 'content') {
            // After main content swap, re-fetch the tag fragment so checkboxes reflect current querystring
            try {
              var qs = window.location.search || '';
              fetch('/mangas/tags-fragment' + qs, { credentials: 'same-origin' })
                .then(function (resp) { if (!resp.ok) throw new Error('Network response not ok'); return resp.text(); })
                .then(function (html) {
                  var el = document.getElementById('tag-list');
                  if (el) el.innerHTML = html;
                  try { syncTagFormSortOrder(); } catch (_) { }
                })
                .catch(function () { try { syncTagFormSortOrder(); } catch (_) { } });
            } catch (_) { try { syncTagFormSortOrder(); } catch (_) { } }
          } else if (tid === 'tag-list') {
            try { syncTagFormSortOrder(); } catch (_) { }
          }
        }
      });

      // Ensure uk-select custom events update native select before HTMX handles the event
      // Use a capturing listener so we run before bubble-phase HTMX handlers
      document.addEventListener('uk-select:input', function (e) {
        try {
          // Build canonical comma-separated tag summary from checked checkboxes
          try {
            var hiddenSummary = document.getElementById('tag-hidden-summary');
            if (hiddenSummary) {
              var tagChecks = document.querySelectorAll('#tag-list input[name="tags"]:checked');
              var vals = Array.prototype.slice.call(tagChecks).map(function (cb) { return cb.value; }).filter(Boolean);
              hiddenSummary.value = vals.join(',');
            }
          } catch (_) { }
          var detail = (e && e.detail) ? e.detail : null;
          var candidate = null;
          if (detail != null) {
            if (typeof detail === 'object') {
              if (typeof detail.value !== 'undefined') candidate = detail.value;
              else if (typeof detail.text !== 'undefined') candidate = detail.text;
              else candidate = detail;
            } else if (typeof detail === 'string' || typeof detail === 'number') {
              candidate = detail;
            }
          }
          var hidden = document.getElementById('manga-sort-select');
          if (hidden && candidate != null) {
            // try to match an existing option by value or visible text
            var opts = hidden.options;
            var matched = false;
            for (var i = 0; i < opts.length; i++) {
              try {
                if (opts[i].value === String(candidate) || opts[i].text === String(candidate)) {
                  hidden.value = opts[i].value;
                  matched = true;
                  break;
                }
              } catch (_) { }
            }
            if (!matched) {
              // last resort: set raw string
              hidden.value = String(candidate);
            }
            // No immediate change event dispatched; HTMX will include the updated element when it handles the uk-select:input trigger.
          }
        } catch (err) {
          // noop
        }
      }, true);

      // When HTMX swaps main content, ensure we scroll to top for user context
      document.addEventListener('htmx:afterSwap', function (event) {
        try {
          if (event.detail && event.detail.target && event.detail.target.id === 'content') {
            window.scrollTo(0, 0);
          }
        } catch (_) { }
      });

      document.addEventListener('click', function (e) { const btn = e.target.closest && e.target.closest('#tag-mode-toggle'); if (!btn) return; const form = document.getElementById('tag-filter-form'); if (!form) return; const modeInput = form.querySelector('input[name="tag_mode"]'); const current = (modeInput && modeInput.value) ? modeInput.value.toLowerCase() : 'all'; const next = current === 'any' ? 'all' : 'any'; if (modeInput) modeInput.value = next; btn.setAttribute('data-mode', next); btn.textContent = (next === 'any') ? 'Any' : 'All'; try { const u = new URL(window.location.href); u.searchParams.set('tag_mode', next); window.history.replaceState({}, '', u); } catch (_) { } });
    } catch (err) {
      console.error('site.js tag dropdown error', err);
    }

    try {
      document.addEventListener('mouseover', function (e) {
        const el = e.target.closest && e.target.closest('.chapter-read-icon');
        if (!el) return;
        const open = el.querySelector('.eye-open');
        const closed = el.querySelector('.eye-closed');
        if (!open || !closed) return;
        const openVisible = window.getComputedStyle(open).display !== 'none';
        if (openVisible) {
          open.style.display = 'none';
          closed.style.display = 'inline-flex';
        } else {
          open.style.display = 'inline-flex';
          closed.style.display = 'none';
        }
      });

      document.addEventListener('mouseout', function (e) {
        const el = e.target.closest && e.target.closest('.chapter-read-icon');
        if (!el) return;
        const open = el.querySelector('.eye-open');
        const closed = el.querySelector('.eye-closed');
        if (!open || !closed) return;
        const form = el.querySelector('form');
        if (form && form.getAttribute('hx-post') && form.getAttribute('hx-post').includes('/unread')) {
          open.style.display = 'inline-flex';
          closed.style.display = 'none';
        } else {
          open.style.display = 'none';
          closed.style.display = 'inline-flex';
        }
      });
    } catch (err) {
      console.error('site.js chapter hover handlers error', err);
    }

    // Scroll helpers used by templates (exposed globally)
    try {
      window.scrollToTop = function () {
        window.scrollTo({ top: 0, behavior: 'smooth' });
      };
      window.scrollToTopInstant = function () {
        window.scrollTo({ top: 0, behavior: 'auto' });
      };
    } catch (err) {
      // noop
    }
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
