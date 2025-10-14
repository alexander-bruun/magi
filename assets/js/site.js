// site.js - common site scripts moved from inline templates
// Organized into modules: sidebar, navigation, tags, scroll helpers

(function () {
  'use strict';

  // === Sidebar Management ===
  function initSidebar() {
    const STORAGE_KEY = '__FRANKEN_SIDEBAR_COLLAPSED__';
    const MOBILE_BREAKPOINT = '(max-width: 768px)';
    
    const elements = {
      body: document.body,
      toggle: document.getElementById('sidebar-toggle'),
      sidebar: document.getElementById('sidebar'),
      backdrop: document.getElementById('sidebar-backdrop')
    };
    
    const mq = window.matchMedia ? window.matchMedia(MOBILE_BREAKPOINT) : null;
    
    function applyCollapsed(collapsed) {
      elements.body.classList.toggle('sidebar-collapsed', collapsed);
      elements.toggle?.setAttribute('aria-expanded', String(!collapsed));
    }

    function openMobileSidebar() {
      if (!elements.sidebar) return;
      elements.body.classList.add('sidebar-open');
      elements.toggle?.setAttribute('aria-expanded', 'true');
      elements.backdrop?.removeAttribute('hidden');
    }

    function closeMobileSidebar() {
      elements.body.classList.remove('sidebar-open');
      elements.toggle?.setAttribute('aria-expanded', 'false');
      elements.backdrop?.setAttribute('hidden', '');
    }

    function handleToggleClick() {
      const isMobile = mq?.matches;
      
      if (isMobile) {
        elements.body.classList.contains('sidebar-open') 
          ? closeMobileSidebar() 
          : openMobileSidebar();
      } else {
        const isCollapsed = elements.body.classList.toggle('sidebar-collapsed');
        localStorage.setItem(STORAGE_KEY, isCollapsed ? '1' : '0');
        elements.toggle?.setAttribute('aria-expanded', String(!isCollapsed));
      }
    }

    // Initialize collapsed state
    const stored = localStorage.getItem(STORAGE_KEY);
    applyCollapsed(stored ? stored === '1' : mq?.matches || false);

    // Event listeners
    elements.toggle?.addEventListener('click', handleToggleClick);
    elements.backdrop?.addEventListener('click', closeMobileSidebar);
    
    document.addEventListener('click', (e) => {
      if (mq?.matches && e.target.closest('.sidebar a[href]')) {
        closeMobileSidebar();
      }
    });

    document.addEventListener('keydown', (e) => {
      if (e.key === 'Escape' && mq?.matches && elements.body.classList.contains('sidebar-open')) {
        closeMobileSidebar();
      }
    });

    // Handle media query changes
    const handleMqChange = (e) => {
      if (!localStorage.getItem(STORAGE_KEY)) {
        applyCollapsed(e.matches);
      }
      if (!e.matches) closeMobileSidebar();
    };

    mq?.addEventListener?.('change', handleMqChange) || mq?.addListener?.(handleMqChange);
  }

  // === Navigation Active State ===
  function initNavigationSync() {
    function normalizePath(path) {
      try {
        return new URL(path, location.origin).pathname.replace(/\/+$/g, '/') || '/';
      } catch {
        return '/';
      }
    }

    function updateActiveNav() {
      const currentPath = normalizePath(location.pathname);
      const navLinks = Array.from(document.querySelectorAll('.uk-nav a[href]'))
        .filter(a => a.getAttribute('href'))
        .map(a => ({
          element: a,
          path: normalizePath(a.getAttribute('href')),
          parent: a.closest('li'),
          group: a.closest('.uk-nav')
        }));

      // Remove all active states
      document.querySelectorAll('.uk-nav li.uk-active').forEach(li => 
        li.classList.remove('uk-active')
      );

      // Find best match for each nav group
      const bestMatches = new Map();
      navLinks.forEach(({ path, parent, group }) => {
        if (!parent || !group) return;
        
        const isMatch = path === '/' 
          ? currentPath === '/' 
          : currentPath === path || currentPath.startsWith(path + '/');
        
        if (isMatch) {
          const score = path.length;
          const current = bestMatches.get(group);
          if (!current || score > current.score) {
            bestMatches.set(group, { parent, score });
          }
        }
      });

      bestMatches.forEach(({ parent }) => parent.classList.add('uk-active'));
    }

    // Initialize and setup event listeners
    if (document.readyState !== 'loading') {
      updateActiveNav();
    } else {
      document.addEventListener('DOMContentLoaded', updateActiveNav);
    }

    document.addEventListener('htmx:afterSwap', updateActiveNav);
    document.addEventListener('htmx:afterSettle', updateActiveNav);
    window.addEventListener('popstate', updateActiveNav);

    document.addEventListener('click', (ev) => {
      const link = ev.target.closest('a[href]');
      if (!link || link.target === '_blank') return;
      
      const href = link.getAttribute('href');
      if (!href || (href.startsWith('http') && new URL(href).origin !== location.origin)) return;
      
      setTimeout(updateActiveNav, 10);
    });
  }

  // === Tag Filtering ===
  function initTagFiltering() {
    function syncTagFormSortOrder() {
      const form = document.getElementById('tag-filter-form');
      if (!form) return;
      
      const params = new URLSearchParams(window.location.search);
      const select = document.getElementById('manga-sort-select');
      
      const sort = select?.value || params.get('sort') || '';
      const order = params.get('order') || '';
      let tagMode = (params.get('tag_mode') || 'all').toLowerCase();
      tagMode = (tagMode === 'any' || tagMode === 'all') ? tagMode : 'all';

      // Update hidden inputs
      const inputs = {
        sort: form.querySelector('input[name="sort"]'),
        order: form.querySelector('input[name="order"]'),
        mode: form.querySelector('input[name="tag_mode"]')
      };

      if (sort && inputs.sort) inputs.sort.value = sort;
      if (order && inputs.order) inputs.order.value = order;
      if (inputs.mode) inputs.mode.value = tagMode;

      // Update toggle button
      const toggle = document.getElementById('tag-mode-toggle');
      if (toggle) {
        toggle.setAttribute('data-mode', tagMode);
        toggle.textContent = tagMode === 'any' ? 'Any' : 'All';
      }
    }

    function refreshTagFragment() {
      const path = window.location.pathname || '';
      if (path.startsWith('/account/')) {
        syncTagFormSortOrder();
        return;
      }

      const qs = window.location.search || '';
      fetch('/mangas/tags-fragment' + qs, { credentials: 'same-origin' })
        .then(resp => resp.ok ? resp.text() : Promise.reject())
        .then(html => {
          const tagList = document.getElementById('tag-list');
          if (tagList) tagList.innerHTML = html;
          syncTagFormSortOrder();
        })
        .catch(() => syncTagFormSortOrder());
    }

    // Initialize
    if (document.readyState !== 'loading') {
      syncTagFormSortOrder();
    } else {
      document.addEventListener('DOMContentLoaded', syncTagFormSortOrder);
    }

    window.addEventListener('popstate', syncTagFormSortOrder);

    document.addEventListener('htmx:afterSwap', (e) => {
      const targetId = e.detail?.target?.id;
      if (targetId === 'content') {
        setTimeout(refreshTagFragment, 10);
      } else if (targetId === 'tag-list') {
        syncTagFormSortOrder();
      }
    });

    // Handle uk-select custom events
    document.addEventListener('uk-select:input', (e) => {
      // Update hidden summary from checked checkboxes
      const hiddenSummary = document.getElementById('tag-hidden-summary');
      if (hiddenSummary) {
        const checked = Array.from(document.querySelectorAll('#tag-list input[name="tags"]:checked'));
        hiddenSummary.value = checked.map(cb => cb.value).filter(Boolean).join(',');
      }

      // Extract value from event detail
      const detail = e.detail;
      let value = null;
      if (detail && typeof detail === 'object') {
        value = detail.value ?? detail.text ?? detail;
      } else if (typeof detail === 'string' || typeof detail === 'number') {
        value = detail;
      }

      // Find and update hidden select
      let select = e.target.tagName === 'UK-SELECT' 
        ? e.target.querySelector('select[hidden]')
        : e.target.closest('uk-select')?.querySelector('select[hidden]') 
          || document.querySelector('select[name="sort"][hidden]');

      if (select && value != null) {
        const options = Array.from(select.options);
        const match = options.find(opt => opt.value === String(value) || opt.text === String(value));
        select.value = match ? match.value : String(value);
      }
    }, true);

    // Scroll to top after content swap
    document.addEventListener('htmx:afterSwap', (e) => {
      if (e.detail?.target?.id === 'content') {
        window.scrollTo(0, 0);
      }
    });

    // Tag mode toggle button
    document.addEventListener('click', (e) => {
      const btn = e.target.closest('#tag-mode-toggle');
      if (!btn) return;

      const form = document.getElementById('tag-filter-form');
      if (!form) return;

      const modeInput = form.querySelector('input[name="tag_mode"]');
      const currentMode = (modeInput?.value || 'all').toLowerCase();
      const nextMode = currentMode === 'any' ? 'all' : 'any';

      if (modeInput) modeInput.value = nextMode;
      btn.setAttribute('data-mode', nextMode);
      btn.textContent = nextMode === 'any' ? 'Any' : 'All';

      try {
        const url = new URL(window.location.href);
        url.searchParams.set('tag_mode', nextMode);
        window.history.replaceState({}, '', url);
      } catch {}
    });
  }

  // === Chapter Eye Icon Hover ===
  function initChapterHover() {
    document.addEventListener('mouseover', (e) => {
      const icon = e.target.closest('.chapter-read-icon');
      if (!icon) return;

      const openEye = icon.querySelector('.eye-open');
      const closedEye = icon.querySelector('.eye-closed');
      if (!openEye || !closedEye) return;

      const isOpen = window.getComputedStyle(openEye).display !== 'none';
      openEye.style.display = isOpen ? 'none' : 'inline-flex';
      closedEye.style.display = isOpen ? 'inline-flex' : 'none';
    });

    document.addEventListener('mouseout', (e) => {
      const icon = e.target.closest('.chapter-read-icon');
      if (!icon) return;

      const openEye = icon.querySelector('.eye-open');
      const closedEye = icon.querySelector('.eye-closed');
      if (!openEye || !closedEye) return;

      const form = icon.querySelector('form');
      const isUnread = form?.getAttribute('hx-post')?.includes('/unread');

      openEye.style.display = isUnread ? 'inline-flex' : 'none';
      closedEye.style.display = isUnread ? 'none' : 'inline-flex';
    });
  }

  // === Scroll Helpers ===
  function initScrollHelpers() {
    window.scrollToTop = () => window.scrollTo({ top: 0, behavior: 'smooth' });
    window.scrollToTopInstant = () => window.scrollTo({ top: 0, behavior: 'auto' });
  }

  // === Main Initialization ===
  function init() {
    try { initSidebar(); } catch (e) { console.error('Sidebar init error:', e); }
    try { initNavigationSync(); } catch (e) { console.error('Navigation sync error:', e); }
    try { initTagFiltering(); } catch (e) { console.error('Tag filtering error:', e); }
    try { initChapterHover(); } catch (e) { console.error('Chapter hover error:', e); }
    try { initScrollHelpers(); } catch (e) { console.error('Scroll helpers error:', e); }
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
