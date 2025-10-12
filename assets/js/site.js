// site.js - common site scripts moved from inline templates
// - titleHandler: tiny helper to set the document title
// - sidebar collapse persistence and toggle handling
// - HTMX-aware nav active state syncing

function titleHandler(title) {
  document.title = title;
}

(function () {
  // Run initialization after DOM is ready
  function init() {
    // Sidebar collapsed persistence
    try {
      const STORAGE_KEY = '__FRANKEN_SIDEBAR_COLLAPSED__';
      const body = document.body;
      const toggle = document.getElementById('sidebar-toggle');
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

      toggle && toggle.addEventListener('click', function () {
        const isCollapsed = body.classList.toggle('sidebar-collapsed');
        localStorage.setItem(STORAGE_KEY, isCollapsed ? '1' : '0');
        this.setAttribute('aria-expanded', isCollapsed ? 'false' : 'true');
      });

      function handleMqChange(e) {
        const storedPref = localStorage.getItem(STORAGE_KEY);
        if (storedPref === null) {
          applyCollapsed(e.matches);
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
        anchors.forEach(({a, path}) => {
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

    // Sync dropdown active item for sort dropdowns
    try {
      document.addEventListener('show', function (e) {
        try {
          if (!e.target.classList.contains('uk-dropdown')) return;
          const drop = e.target;
          // parse sort param from current URL
          const params = new URLSearchParams(window.location.search);
          let sort = params.get('sort') || '';
          if (sort === 'title') sort = 'name';
          // clear existing uk-active
          const items = drop.querySelectorAll('.uk-dropdown-nav.uk-nav li');
          items.forEach(i => i.classList.remove('uk-active'));
          if (!sort) {
            // fallback: try matching the button label
            const btn = document.getElementById('manga-sort-btn');
            if (btn) {
              const labelEl = btn.querySelector('.sort-label');
              if (labelEl) sort = labelEl.textContent.trim().toLowerCase();
            }
          }
          if (sort) {
            const match = Array.from(drop.querySelectorAll('[data-sort-key]')).find(a => a.getAttribute('data-sort-key') === sort || a.textContent.trim().toLowerCase() === sort);
            if (match) {
              const li = match.closest('li');
              if (li) li.classList.add('uk-active');
            }
          }
        } catch (err) {
          // ignore
        }
      }, false);
    } catch (e) {
      console.error('site.js dropdown sync error', e);
    }
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
