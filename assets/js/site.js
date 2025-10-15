// site.js - common site scripts moved from inline templates
// Organized into modules: sidebar, navigation, tags, scroll helpers

(function () {
  'use strict';

  // === Constants ===
  const STORAGE_KEY = '__FRANKEN_SIDEBAR_COLLAPSED__';
  const MOBILE_BREAKPOINT = '(max-width: 768px)';

  // === Utility Functions ===
  const isMobile = () => window.matchMedia(MOBILE_BREAKPOINT).matches;

  const safeExecute = (fn, errorContext) => {
    try {
      fn();
    } catch (e) {
      console.error(`${errorContext} error:`, e);
    }
  };

  // === Sidebar Management ===
  const SidebarManager = {
    elements: null,
    mediaQuery: null,

    init() {
      this.elements = {
        body: document.body,
        toggle: document.getElementById('sidebar-toggle'),
        sidebar: document.getElementById('sidebar'),
        backdrop: document.getElementById('sidebar-backdrop')
      };
      
      this.mediaQuery = window.matchMedia(MOBILE_BREAKPOINT);
      this.initializeState();
      this.attachEventListeners();
    },

    initializeState() {
      const stored = localStorage.getItem(STORAGE_KEY);
      const shouldCollapse = stored ? stored === '1' : this.mediaQuery.matches;
      this.applyCollapsed(shouldCollapse);
    },

    applyCollapsed(collapsed) {
      this.elements.body.classList.toggle('sidebar-collapsed', collapsed);
      this.elements.toggle?.setAttribute('aria-expanded', String(!collapsed));
    },

    openMobile() {
      if (!this.elements.sidebar) return;
      this.elements.body.classList.add('sidebar-open');
      this.elements.toggle?.setAttribute('aria-expanded', 'true');
      this.elements.backdrop?.removeAttribute('hidden');
    },

    closeMobile() {
      this.elements.body.classList.remove('sidebar-open');
      this.elements.toggle?.setAttribute('aria-expanded', 'false');
      this.elements.backdrop?.setAttribute('hidden', '');
    },

    handleToggle() {
      if (this.mediaQuery.matches) {
        this.elements.body.classList.contains('sidebar-open') 
          ? this.closeMobile() 
          : this.openMobile();
      } else {
        const isCollapsed = this.elements.body.classList.toggle('sidebar-collapsed');
        localStorage.setItem(STORAGE_KEY, isCollapsed ? '1' : '0');
        this.elements.toggle?.setAttribute('aria-expanded', String(!isCollapsed));
      }
    },

    handleMediaQueryChange(e) {
      if (!localStorage.getItem(STORAGE_KEY)) {
        this.applyCollapsed(e.matches);
      }
      if (!e.matches) {
        this.closeMobile();
      }
    },

    attachEventListeners() {
      this.elements.toggle?.addEventListener('click', () => this.handleToggle());
      this.elements.backdrop?.addEventListener('click', () => this.closeMobile());
      
      document.addEventListener('click', (e) => {
        if (this.mediaQuery.matches && e.target.closest('.sidebar a[href]')) {
          this.closeMobile();
        }
      });

      document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape' && 
            this.mediaQuery.matches && 
            this.elements.body.classList.contains('sidebar-open')) {
          this.closeMobile();
        }
      });

      const handleChange = (e) => this.handleMediaQueryChange(e);
      this.mediaQuery.addEventListener?.('change', handleChange) || 
        this.mediaQuery.addListener?.(handleChange);
    }
  };

  // === Navigation Active State ===
  const NavigationManager = {
    normalizePath(path) {
      try {
        return new URL(path, location.origin).pathname.replace(/\/+$/g, '/') || '/';
      } catch {
        return '/';
      }
    },

    getNavLinks() {
      return Array.from(document.querySelectorAll('.uk-nav a[href]'))
        .filter(a => a.getAttribute('href'))
        .map(a => ({
          element: a,
          path: this.normalizePath(a.getAttribute('href')),
          parent: a.closest('li'),
          group: a.closest('.uk-nav')
        }));
    },

    clearActiveStates() {
      document.querySelectorAll('.uk-nav li.uk-active')
        .forEach(li => li.classList.remove('uk-active'));
    },

    findBestMatches(currentPath, navLinks) {
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

      return bestMatches;
    },

    updateActiveNav() {
      const currentPath = this.normalizePath(location.pathname);
      const navLinks = this.getNavLinks();
      
      this.clearActiveStates();
      
      const bestMatches = this.findBestMatches(currentPath, navLinks);
      bestMatches.forEach(({ parent }) => parent.classList.add('uk-active'));
    },

    init() {
      const update = () => this.updateActiveNav();
      
      if (document.readyState !== 'loading') {
        update();
      } else {
        document.addEventListener('DOMContentLoaded', update);
      }

      document.addEventListener('htmx:afterSwap', update);
      document.addEventListener('htmx:afterSettle', update);
      window.addEventListener('popstate', update);

      document.addEventListener('click', (ev) => {
        const link = ev.target.closest('a[href]');
        if (!link || link.target === '_blank') return;
        
        const href = link.getAttribute('href');
        if (!href || (href.startsWith('http') && new URL(href).origin !== location.origin)) return;
        
        setTimeout(update, 10);
      });
    }
  };

  // === Tag Filtering ===
  const TagFilterManager = {
    getFormElements() {
      const form = document.getElementById('tag-filter-form');
      if (!form) return null;

      return {
        form,
        sortInput: form.querySelector('input[name="sort"]'),
        orderInput: form.querySelector('input[name="order"]'),
        modeInput: form.querySelector('input[name="tag_mode"]'),
        sortSelect: document.getElementById('manga-sort-select'),
        modeToggle: document.getElementById('tag-mode-toggle'),
        tagList: document.getElementById('tag-list'),
        hiddenSummary: document.getElementById('tag-hidden-summary')
      };
    },

    getUrlParams() {
      const params = new URLSearchParams(window.location.search);
      let tagMode = (params.get('tag_mode') || 'all').toLowerCase();
      tagMode = (tagMode === 'any' || tagMode === 'all') ? tagMode : 'all';

      return {
        sort: params.get('sort') || '',
        order: params.get('order') || '',
        tagMode
      };
    },

    syncFormState() {
      const elements = this.getFormElements();
      if (!elements) return;

      const params = this.getUrlParams();
      const sort = elements.sortSelect?.value || params.sort;

      // Update hidden inputs
      if (sort && elements.sortInput) {
        elements.sortInput.value = sort;
      }
      if (params.order && elements.orderInput) {
        elements.orderInput.value = params.order;
      }
      if (elements.modeInput) {
        elements.modeInput.value = params.tagMode;
      }

      // Update toggle button
      if (elements.modeToggle) {
        elements.modeToggle.setAttribute('data-mode', params.tagMode);
        elements.modeToggle.textContent = params.tagMode === 'any' ? 'Any' : 'All';
      }
    },

    refreshTagFragment() {
      const path = window.location.pathname || '';
      if (path.startsWith('/account/')) {
        this.syncFormState();
        return;
      }

      const qs = window.location.search || '';
      fetch('/mangas/tags-fragment' + qs, { credentials: 'same-origin' })
        .then(resp => resp.ok ? resp.text() : Promise.reject())
        .then(html => {
          const tagList = document.getElementById('tag-list');
          if (tagList) {
            tagList.innerHTML = html;
          }
          this.syncFormState();
        })
        .catch(() => this.syncFormState());
    },

    updateHiddenSummary() {
      const elements = this.getFormElements();
      if (!elements?.hiddenSummary) return;

      const checked = Array.from(document.querySelectorAll('#tag-list input[name="tags"]:checked'));
      elements.hiddenSummary.value = checked.map(cb => cb.value).filter(Boolean).join(',');
    },

    extractValue(detail) {
      if (detail && typeof detail === 'object') {
        return detail.value ?? detail.text ?? detail;
      }
      if (typeof detail === 'string' || typeof detail === 'number') {
        return detail;
      }
      return null;
    },

    updateHiddenSelect(target, value) {
      if (value == null) return;

      const select = target.tagName === 'UK-SELECT' 
        ? target.querySelector('select[hidden]')
        : target.closest('uk-select')?.querySelector('select[hidden]') 
          || document.querySelector('select[name="sort"][hidden]');

      if (!select) return;

      const options = Array.from(select.options);
      const match = options.find(opt => 
        opt.value === String(value) || opt.text === String(value)
      );
      
      select.value = match ? match.value : String(value);
    },

    handleTagModeToggle(btn) {
      const elements = this.getFormElements();
      if (!elements) return;

      const currentMode = (elements.modeInput?.value || 'all').toLowerCase();
      const nextMode = currentMode === 'any' ? 'all' : 'any';

      if (elements.modeInput) {
        elements.modeInput.value = nextMode;
      }
      
      btn.setAttribute('data-mode', nextMode);
      btn.textContent = nextMode === 'any' ? 'Any' : 'All';

      try {
        const url = new URL(window.location.href);
        url.searchParams.set('tag_mode', nextMode);
        window.history.replaceState({}, '', url);
      } catch {}
    },

    init() {
      const sync = () => this.syncFormState();
      const refresh = () => this.refreshTagFragment();

      if (document.readyState !== 'loading') {
        sync();
      } else {
        document.addEventListener('DOMContentLoaded', sync);
      }

      window.addEventListener('popstate', sync);

      document.addEventListener('htmx:afterSwap', (e) => {
        const targetId = e.detail?.target?.id;
        if (targetId === 'content') {
          setTimeout(refresh, 10);
          window.scrollTo(0, 0);
        } else if (targetId === 'tag-list') {
          sync();
        }
      });

      document.addEventListener('uk-select:input', (e) => {
        this.updateHiddenSummary();
        const value = this.extractValue(e.detail);
        this.updateHiddenSelect(e.target, value);
      }, true);

      document.addEventListener('click', (e) => {
        const btn = e.target.closest('#tag-mode-toggle');
        if (btn) {
          this.handleTagModeToggle(btn);
        }
      });
    }
  };

  // === Chapter Eye Icon Hover ===
  const ChapterHoverManager = {
    toggleEyeIcons(icon, showOpen) {
      const openEye = icon.querySelector('.eye-open');
      const closedEye = icon.querySelector('.eye-closed');
      if (!openEye || !closedEye) return;

      openEye.style.display = showOpen ? 'inline-flex' : 'none';
      closedEye.style.display = showOpen ? 'none' : 'inline-flex';
    },

    handleMouseOver(icon) {
      const openEye = icon.querySelector('.eye-open');
      const closedEye = icon.querySelector('.eye-closed');
      if (!openEye || !closedEye) return;

      const isOpen = window.getComputedStyle(openEye).display !== 'none';
      this.toggleEyeIcons(icon, !isOpen);
    },

    handleMouseOut(icon) {
      const form = icon.querySelector('form');
      const isUnread = form?.getAttribute('hx-post')?.includes('/unread');
      this.toggleEyeIcons(icon, isUnread);
    },

    init() {
      document.addEventListener('mouseover', (e) => {
        const icon = e.target.closest('.chapter-read-icon');
        if (icon) this.handleMouseOver(icon);
      });

      document.addEventListener('mouseout', (e) => {
        const icon = e.target.closest('.chapter-read-icon');
        if (icon) this.handleMouseOut(icon);
      });
    }
  };

  // === Scroll Helpers ===
  const ScrollHelpers = {
    init() {
      window.scrollToTop = () => window.scrollTo({ top: 0, behavior: 'smooth' });
      window.scrollToTopInstant = () => window.scrollTo({ top: 0, behavior: 'auto' });
    }
  };

  // === Main Initialization ===
  function init() {
    safeExecute(() => SidebarManager.init(), 'Sidebar init');
    safeExecute(() => NavigationManager.init(), 'Navigation sync');
    safeExecute(() => TagFilterManager.init(), 'Tag filtering');
    safeExecute(() => ChapterHoverManager.init(), 'Chapter hover');
    safeExecute(() => ScrollHelpers.init(), 'Scroll helpers');
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
