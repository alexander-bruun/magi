/**
 * Magi.js - Core JavaScript library for the Magi application
 * Consolidates common utilities, WebSocket handling, and UI interactions
 * 
 * @version 1.0.0
 * @author Magi Development Team
 */

(function(window) {
  'use strict';

  // ============================================================================
  // SMOOTH SCROLL MODULE
  // ============================================================================
  
  /**
   * Generic smooth scrolling utility for HTMX-powered applications.
   * Automatically scrolls to elements after HTMX swaps/settles when they have
   * the data-smooth-scroll attribute.
   * 
   * Usage:
   * 1. Add data-smooth-scroll attribute to any element you want to scroll to after HTMX updates
   * 2. Optionally add data-scroll-offset="N" to specify offset in pixels (useful for fixed headers)
   */
  const SmoothScroll = (function() {
    /**
     * Find the nearest scrollable ancestor of an element
     */
    function getScrollableAncestor(el) {
      var node = el.parentNode;
      while (node && node !== document.body && node !== document.documentElement) {
        var style = window.getComputedStyle(node);
        var overflowY = style.overflowY;
        if (overflowY === 'auto' || overflowY === 'scroll') return node;
        node = node.parentNode;
      }
      return window;
    }

    /**
     * Animate scroll with easing
     */
    function animateScroll(container, target, duration) {
      duration = duration || 360;
      var start = container === window ? window.pageYOffset : container.scrollTop;
      var change = target - start;
      if (Math.abs(change) < 2) return;
      var startTime = performance.now();

      function easeInOut(t) {
        return 0.5 - 0.5 * Math.cos(Math.PI * t);
      }

      function step(now) {
        var elapsed = now - startTime;
        var t = Math.min(1, elapsed / duration);
        var eased = easeInOut(t);
        var current = Math.round(start + change * eased);
        if (container === window) {
          window.scrollTo(0, current);
        } else {
          container.scrollTop = current;
        }
        if (t < 1) {
          requestAnimationFrame(step);
        }
      }

      requestAnimationFrame(step);
    }

    /**
     * Smoothly scroll to an element
     */
    function scrollToElement(el, offset) {
      try {
        offset = offset || 0;
        var container = getScrollableAncestor(el);

        // Blur any focused element to avoid browsers jumping to focused elements
        try { 
          if (document.activeElement && typeof document.activeElement.blur === 'function') {
            document.activeElement.blur();
          }
        } catch(e) {}

        if (container === window) {
          var top = el.getBoundingClientRect().top + window.pageYOffset - offset;
          animateScroll(window, Math.max(0, top), 380);
        } else {
          var containerRect = container.getBoundingClientRect();
          var elRect = el.getBoundingClientRect();
          var targetScroll = container.scrollTop + (elRect.top - containerRect.top) - offset;
          animateScroll(container, Math.max(0, targetScroll), 380);
        }
      } catch (err) {
        console.error('Smooth scroll error:', err);
      }
    }

    /**
     * Get scroll offset for an element from its data attribute
     */
    function getOffsetForElement(el) {
      var data = el && el.dataset && el.dataset.scrollOffset;
      var n = parseInt(data, 10);
      return isNaN(n) ? 0 : n;
    }

    /**
     * Handle scroll for an element if it has the data-smooth-scroll attribute
     */
    function handleScrollIfNeeded(target) {
      if (!target) return;
      
      // Check if the target or any of its children have the data-smooth-scroll attribute
      var scrollTarget = target.hasAttribute('data-smooth-scroll') 
        ? target 
        : target.querySelector('[data-smooth-scroll]');
      
      if (!scrollTarget) return;
      
      var offset = getOffsetForElement(scrollTarget);
      // Use a short timeout to allow layout/images to settle
      setTimeout(function() { 
        scrollToElement(scrollTarget, offset); 
      }, 90);
    }

    /**
     * Initialize smooth scroll listeners
     */
    function init() {
      // Listen for HTMX afterSettle event (most reliable - after all processing is complete)
      document.addEventListener('htmx:afterSettle', function(evt) {
        handleScrollIfNeeded(evt && evt.detail && evt.detail.target);
      });

      // Also listen for afterSwap as a quicker attempt
      document.addEventListener('htmx:afterSwap', function(evt) {
        handleScrollIfNeeded(evt && evt.detail && evt.detail.target);
      });
    }

    return {
      init: init,
      toElement: scrollToElement,
      getScrollableAncestor: getScrollableAncestor
    };
  })();

  // ============================================================================
  // WEBSOCKET HANDLER
  // ============================================================================

  /**
   * Generic WebSocket Handler
   * A reusable WebSocket client with automatic reconnection and lifecycle management
   */
  class WebSocketHandler {
    constructor(options) {
      // Required options
      this.wsUrl = options.wsUrl;
      
      // Connection options
      this.reconnectInterval = options.reconnectInterval || 3000;
      this.maxReconnectAttempts = options.maxReconnectAttempts || Infinity;
      this.autoConnect = options.autoConnect !== false;
      
      // Callbacks
      this.onOpen = options.onOpen || (() => {});
      this.onMessage = options.onMessage || (() => {});
      this.onError = options.onError || ((error) => console.error('WebSocket error:', error));
      this.onClose = options.onClose || (() => {});
      
      // Message parsing
      this.parseMessage = options.parseMessage || this.defaultParseMessage.bind(this);
      
      // Internal state
      this.ws = null;
      this.reconnectAttempts = 0;
      this.reconnectTimer = null;
      this.isManualClose = false;
      
      // Auto-connect if enabled
      if (this.autoConnect) {
        this.connect();
      }
      
      // Setup cleanup on page unload
      this.setupCleanup();
    }
    
    defaultParseMessage(event) {
      try {
        return JSON.parse(event.data);
      } catch (e) {
        console.error('Failed to parse WebSocket message:', e);
        return null;
      }
    }
    
    connect() {
      if (this.ws && this.ws.readyState === WebSocket.OPEN) {
        return;
      }
      
      this.isManualClose = false;
      
      try {
        this.ws = new WebSocket(this.wsUrl);
        
        this.ws.onopen = (event) => {
          this.reconnectAttempts = 0;
          
          if (this.reconnectTimer) {
            clearTimeout(this.reconnectTimer);
            this.reconnectTimer = null;
          }
          
          this.onOpen(event);
        };
        
        this.ws.onmessage = (event) => {
          const data = this.parseMessage(event);
          if (data !== null) {
            this.onMessage(data, event);
          }
        };
        
        this.ws.onerror = (error) => {
          this.onError(error);
        };
        
        this.ws.onclose = (event) => {
          this.onClose(event);
          
          if (!this.isManualClose) {
            this.handleReconnect();
          }
        };
      } catch (e) {
        console.error('Failed to connect WebSocket:', e);
        this.onError(e);
        
        if (!this.isManualClose) {
          this.handleReconnect();
        }
      }
    }
    
    handleReconnect() {
      if (this.reconnectAttempts >= this.maxReconnectAttempts) {
        console.warn('Max reconnect attempts reached');
        return;
      }
      
      this.reconnectAttempts++;
      
      this.reconnectTimer = setTimeout(() => {
        this.connect();
      }, this.reconnectInterval);
    }
    
    send(data) {
      if (this.ws && this.ws.readyState === WebSocket.OPEN) {
        const message = typeof data === 'string' ? data : JSON.stringify(data);
        this.ws.send(message);
        return true;
      } else {
        console.warn('WebSocket is not connected. Cannot send message.');
        return false;
      }
    }
    
    disconnect() {
      this.isManualClose = true;
      
      if (this.reconnectTimer) {
        clearTimeout(this.reconnectTimer);
        this.reconnectTimer = null;
      }
      
      if (this.ws) {
        this.ws.close();
        this.ws = null;
      }
    }
    
    reconnect() {
      this.disconnect();
      this.reconnectAttempts = 0;
      this.connect();
    }
    
    getState() {
      if (!this.ws) return 'CLOSED';
      
      switch (this.ws.readyState) {
        case WebSocket.CONNECTING: return 'CONNECTING';
        case WebSocket.OPEN: return 'OPEN';
        case WebSocket.CLOSING: return 'CLOSING';
        case WebSocket.CLOSED: return 'CLOSED';
        default: return 'UNKNOWN';
      }
    }
    
    setupCleanup() {
      window.addEventListener('beforeunload', () => {
        this.disconnect();
      });
    }
  }

  // ============================================================================
  // ANSI COLOR UTILITIES
  // ============================================================================

  /**
   * Parse ANSI color codes in text and convert to HTML with inline styles
   * @param {string} text - Text containing ANSI escape sequences
   * @returns {string} HTML string with styled spans
   */
  function parseAnsiColors(text) {
    if (!text) return '';

    // ANSI escape sequence regex: \x1b[...m
    const ansiRegex = /\x1b\[([0-9;]*)m/g;
    let result = '';
    let lastIndex = 0;
    let currentStyles = [];

    text.replace(ansiRegex, (match, codes, offset) => {
      // Add text before this escape sequence
      if (offset > lastIndex) {
        const plainText = text.substring(lastIndex, offset);
        result += applyAnsiStyles(plainText, currentStyles);
      }

      // Parse the codes
      const codeArray = codes.split(';').filter(code => code !== '');

      if (codeArray.length === 0 || codeArray[0] === '0') {
        // Reset all styles
        currentStyles = [];
      } else {
        // Apply new styles
        codeArray.forEach(code => {
          const codeNum = parseInt(code);
          if (codeNum === 1) currentStyles.push('bold');
          else if (codeNum === 4) currentStyles.push('underline');
          else if (codeNum >= 30 && codeNum <= 37) currentStyles.push(`color-${codeNum - 30}`);
          else if (codeNum >= 40 && codeNum <= 47) currentStyles.push(`bg-${codeNum - 40}`);
          else if (codeNum >= 90 && codeNum <= 97) currentStyles.push(`bright-color-${codeNum - 90}`);
          else if (codeNum >= 100 && codeNum <= 107) currentStyles.push(`bright-bg-${codeNum - 100}`);
        });
      }

      lastIndex = offset + match.length;
      return match;
    });

    // Add remaining text
    if (lastIndex < text.length) {
      const remainingText = text.substring(lastIndex);
      result += applyAnsiStyles(remainingText, currentStyles);
    }

    return result;
  }

  /**
   * Apply CSS styles to text based on ANSI codes
   * @param {string} text - Plain text to style
   * @param {Array} styles - Array of style names
   * @returns {string} HTML span with applied styles
   */
  function applyAnsiStyles(text, styles) {
    if (styles.length === 0) {
      return escapeHtml(text);
    }

    let cssStyles = styles.map(style => {
      switch (style) {
        case 'bold': return 'font-weight: bold;';
        case 'underline': return 'text-decoration: underline;';
        case 'color-0': return 'color: #000000;'; // Black
        case 'color-1': return 'color: #cd0000;'; // Red
        case 'color-2': return 'color: #00cd00;'; // Green
        case 'color-3': return 'color: #cdcd00;'; // Yellow
        case 'color-4': return 'color: #0000cd;'; // Blue
        case 'color-5': return 'color: #cd00cd;'; // Magenta
        case 'color-6': return 'color: #00cdcd;'; // Cyan
        case 'color-7': return 'color: #e5e5e5;'; // White
        case 'bright-color-0': return 'color: #666666;'; // Bright Black
        case 'bright-color-1': return 'color: #ff0000;'; // Bright Red
        case 'bright-color-2': return 'color: #00ff00;'; // Bright Green
        case 'bright-color-3': return 'color: #ffff00;'; // Bright Yellow
        case 'bright-color-4': return 'color: #0000ff;'; // Bright Blue
        case 'bright-color-5': return 'color: #ff00ff;'; // Bright Magenta
        case 'bright-color-6': return 'color: #00ffff;'; // Bright Cyan
        case 'bright-color-7': return 'color: #ffffff;'; // Bright White
        default: return '';
      }
    }).filter(style => style !== '').join(' ');

    return `<span style="${cssStyles}">${escapeHtml(text)}</span>`;
  }

  /**
   * Escape HTML entities
   * @param {string} text - Text to escape
   * @returns {string} Escaped HTML
   */
  function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }

  // ============================================================================
  // LOG VIEWER
  // ============================================================================

  /**
   * WebSocket Log Viewer
   * Reusable component for streaming logs via WebSocket
   * Built on top of the generic WebSocketHandler
   */
  class LogViewer {
    constructor(options) {
      // UI elements
      this.containerId = options.containerId;
      this.outputId = options.outputId;
      this.maxEntries = options.maxEntries || 1000;
      
      // Log formatting
      this.colorMap = options.colorMap || this.getDefaultColorMap();
      this.formatMessage = options.formatMessage || this.defaultFormatMessage.bind(this);
      this.enableAnsiColors = options.enableAnsiColors !== false; // Default to true
      
      // Get DOM elements
      this.container = document.getElementById(this.containerId);
      this.output = document.getElementById(this.outputId);
      
      if (!this.container || !this.output) {
        console.error('LogViewer: Container or output element not found');
        return;
      }
      
      // Create WebSocket handler with log-specific callbacks
      this.wsHandler = new WebSocketHandler({
        wsUrl: options.wsUrl,
        reconnectInterval: options.reconnectInterval || 3000,
        maxReconnectAttempts: options.maxReconnectAttempts || Infinity,
        onMessage: (data) => this.addLogEntry(data),
        onError: (error) => console.error('LogViewer: WebSocket error:', error)
      });
    }
    
    getDefaultColorMap() {
      return {
        error: '#ff6b6b',
        stderr: '#ff6b6b',
        fatal: '#ff6b6b',
        warn: '#ffd93d',
        warning: '#ffd93d',
        success: '#6bcf7f',
        info: '#6bcf7f',
        default: '#6bcf7f'
      };
    }
    
    defaultFormatMessage(data) {
      return data.message;
    }
    
    /**
     * Parse ANSI escape sequences and convert to HTML spans with CSS styling
     * @param {string} text - Text containing ANSI escape sequences
     * @returns {string} HTML string with styled spans
     */
    parseAnsiColors(text) {
      if (!this.enableAnsiColors || !text) {
        return escapeHtml(text);
      }
      
      return parseAnsiColors(text);
    }
    
    /**
     * Apply CSS styles to text based on ANSI codes
     * @param {string} text - Plain text to style
     * @param {Array} styles - Array of style names
     * @returns {string} HTML span with applied styles
     */
    applyStyles(text, styles) {
      return applyAnsiStyles(text, styles);
    }
    
    /**
     * Escape HTML entities
     * @param {string} text - Text to escape
     * @returns {string} HTML-escaped text
     */
    escapeHtml(text) {
      return escapeHtml(text);
    }
    
    addLogEntry(data) {
      const logEntry = document.createElement('div');
      logEntry.className = 'log-entry';
      
      // Check if message contains ANSI escape sequences
      const message = this.formatMessage(data);
      const hasAnsiCodes = /\x1b\[([0-9;]*)m/.test(message);
      
      if (hasAnsiCodes && this.enableAnsiColors) {
        // Parse ANSI colors and render as HTML
        // Don't apply global color when ANSI codes are present
        logEntry.innerHTML = this.parseAnsiColors(message);
      } else {
        // Regular text rendering with global color
        const color = this.getLogColor(data);
        logEntry.style.color = color;
        logEntry.textContent = message;
      }
      
      // Add to output
      this.output.appendChild(logEntry);
      
      // Limit entries for performance
      const entries = this.output.children;
      if (entries.length > this.maxEntries) {
        this.output.removeChild(entries[0]);
      }
      
      // Auto-scroll to bottom
      this.container.scrollTop = this.container.scrollHeight;
    }
    
    getLogColor(data) {
      const type = (data.type || '').toLowerCase();
      const message = (data.message || '').toLowerCase();
      
      // Check type first
      if (this.colorMap[type]) {
        return this.colorMap[type];
      }
      
      // Check message content for keywords
      for (const keyword in this.colorMap) {
        if (message.includes(keyword)) {
          return this.colorMap[keyword];
        }
      }
      
      return this.colorMap.default;
    }
    
    // Public API methods
    disconnect() {
      if (this.wsHandler) {
        this.wsHandler.disconnect();
      }
    }
    
    reconnect() {
      if (this.wsHandler) {
        this.wsHandler.reconnect();
      }
    }
    
    getState() {
      return this.wsHandler ? this.wsHandler.getState() : 'CLOSED';
    }
  }

  // ============================================================================
  // CONFIG MANAGER MODULE
  // ============================================================================

  /**
   * Manages configuration page interactions
   */
  const ConfigManager = (function() {
    function updateTokenFields() {
      const providerSelect = document.getElementById('metadata-provider-select');
      const malField = document.getElementById('mal-token-field');
      const anilistField = document.getElementById('anilist-token-field');
      
      if (providerSelect && malField && anilistField) {
        const provider = providerSelect.value;
        malField.style.display = provider === 'mal' ? 'block' : 'none';
        anilistField.style.display = provider === 'anilist' ? 'block' : 'none';
      }
    }

    function init() {
      document.addEventListener('DOMContentLoaded', function() {
        const providerSelect = document.getElementById('metadata-provider-select');
        if (providerSelect) {
          providerSelect.addEventListener('change', updateTokenFields);
          updateTokenFields(); // Initialize on page load
        }

        // Auto-initialize console logs
        initConsoleLogs();
      });

      // Reinitialize on HTMX swap
      document.addEventListener('htmx:afterSettle', function(event) {
        if (event.detail.xhr && event.detail.xhr.status === 200) {
          const providerSelect = document.getElementById('metadata-provider-select');
          if (providerSelect) {
            providerSelect.removeEventListener('change', updateTokenFields);
            providerSelect.addEventListener('change', updateTokenFields);
            updateTokenFields();
          }
        }

        // Reinitialize console logs after HTMX swaps
        initConsoleLogs();
      });
    }

    function initConsoleLogs() {
      const container = document.getElementById('console-logs-container');
      const output = document.getElementById('console-logs-output');
      if (!container || !output || container.dataset.initialized) return;

      container.dataset.initialized = 'true';

      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const wsUrl = protocol + '//' + window.location.host + '/admin/config/logs';
      
      new LogViewer({
        wsUrl: wsUrl,
        containerId: 'console-logs-container',
        outputId: 'console-logs-output',
        maxEntries: 1000,
        reconnectInterval: 3000,
        colorMap: {
          error: '#ff6b6b',
          stderr: '#ff6b6b',
          success: 'rgb(var(--success))',
          info: 'white',
          default: 'white'
        }
      });
    }

    return {
      init: init,
      updateTokenFields: updateTokenFields
    };
  })();

  /**
   * Manages scraper script UI interactions
   */
  const ScraperManager = (function() {
    function updateCodeEditor(languageSelect) {
      // Update code editor language
      const codeEditor = document.getElementById('script-content');
      if (codeEditor) {
        let newMode;
        if (languageSelect.value === 'python') {
          codeEditor.setAttribute('data-code-editor', 'python');
          codeEditor.placeholder = "# Python script\n# Enter your python script here\nprint('Hello from scraper')";
          newMode = 'python';
        } else {
          codeEditor.setAttribute('data-code-editor', 'shell');
          codeEditor.placeholder = "#!/bin/bash\n# Enter your bash script here\necho 'Hello from scraper'";
          newMode = 'shell';
        }
        
        // Update CodeMirror editor mode if it exists
        if (window.CodeEditorManager) {
          window.CodeEditorManager.updateEditorMode(codeEditor, newMode);
        }
      }
    }

    function init() {
      // Handle language change for code editor updates
      document.addEventListener('change', function(e) {
        if (e.target.id === 'script-language') {
          updateCodeEditor(e.target);
        }
      });

      // Initial code editor setup
      const languageSelect = document.getElementById('script-language');
      if (languageSelect) {
        updateCodeEditor(languageSelect);
      }

      // Handle run/cancel button visibility
      const runBtn = document.getElementById('run-btn');
      const cancelBtn = document.getElementById('cancel-btn');

      if (runBtn && cancelBtn) {
        document.addEventListener('htmx:beforeRequest', function(evt) {
          if (evt.detail.verb === 'POST' && evt.detail.path.includes('/run')) {
            runBtn.style.display = 'none';
            cancelBtn.style.display = 'inline-block';
          }
        });

        document.addEventListener('htmx:afterRequest', function(evt) {
          if (evt.detail.verb === 'POST' && (evt.detail.path.includes('/run') || evt.detail.path.includes('/cancel'))) {
            runBtn.style.display = 'inline-block';
            cancelBtn.style.display = 'none';
          }
        });
      }

      // Initialize first tab as active
      document.addEventListener('DOMContentLoaded', function() {
        const firstTab = document.querySelector('[data-script-id]');
        if (firstTab) {
          firstTab.classList.add('uk-active');
        }

        // Auto-initialize scraper log viewers
        initScraperLogViewers();
      });

      // Reinitialize after HTMX swaps
      document.addEventListener('htmx:afterSwap', function() {
        initScraperLogViewers();
      });
    }

    function initScraperLogViewers() {
      document.querySelectorAll('[data-script-id]').forEach(function(viewer) {
        if (viewer.dataset.logViewerInitialized) return;
        viewer.dataset.logViewerInitialized = 'true';

        const scriptId = viewer.getAttribute('data-script-id');
        const container = viewer;
        const output = viewer.querySelector('#log-output-container');
        if (!output) return;

        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = protocol + '//' + window.location.host + '/admin/scraper/' + scriptId + '/logs';
        
        new LogViewer({
          wsUrl: wsUrl,
          containerId: viewer.id,
          outputId: output.id,
          maxEntries: 500,
          reconnectInterval: 1000,
          maxReconnectAttempts: 5,
          enableAnsiColors: true,
          formatMessage: function(data) {
            // If message contains ANSI colors, it's already formatted - don't add prefix
            if (/\x1b\[([0-9;]*)m/.test(data.message)) {
              return data.message;
            }
            // Otherwise, add the type prefix for plain messages
            return '[' + data.type.toUpperCase() + '] ' + data.message;
          },
          colorMap: {
            error: '#ff6b6b',
            stderr: '#ff6b6b',
            success: 'rgb(var(--success))',
            info: 'white',
            default: 'white'
          }
        });
      });
    }

    return {
      init: init
    };
  })();

  /**
   * Manages Cropper.js initialization for image cropping functionality
   */
  const CropperManager = (function() {
    function initCropper(img, cropDataInput) {
      if (!img || !cropDataInput) {
        console.error('Missing required elements for Cropper.js');
        return;
      }
      
      if (typeof Cropper === 'undefined') {
        setTimeout(() => initCropper(img, cropDataInput), 100);
        return;
      }
      
      // Find and disable the save button initially
      const container = cropDataInput.closest('.cropper-container') || cropDataInput.parentElement;
      const saveButton = container.querySelector('button[hx-post*="poster/set"]');
      if (saveButton) {
        saveButton.disabled = true;
      }
      
      // Destroy any existing cropper
      if (img.cropper) {
        img.cropper.destroy();
      }
      
      // Initialize Cropper.js with 2:3 aspect ratio for poster
      const cropper = new Cropper(img, {
        aspectRatio: 2 / 3,
        viewMode: 1,
        autoCropArea: 2,
        responsive: true,
        restore: true,
        guides: true,
        center: true,
        highlight: true,
        cropBoxMovable: true,
        cropBoxResizable: true,
        toggleDragModeOnDblclick: true,
        zoom: function(e) {
          // Limit zoom range
          if (e.detail.ratio > 4) {
            e.preventDefault();
          }
        },
        ready: function() {
          // Enable the save button when cropper is ready
          if (saveButton) {
            saveButton.disabled = false;
          }
          
          // Calculate zoom to fit image width to container width
          const container = document.getElementById('image-container');
          const containerWidth = container ? container.clientWidth : img.parentElement.clientWidth;
          const imageWidth = cropper.getImageData().naturalWidth;
          
          // Zoom to fit width exactly
          const fitZoom = containerWidth / imageWidth;
          cropper.zoomTo(fitZoom);
          
          // Position at top-left (0, 0)
          cropper.setCanvasData({
            left: 0,
            top: 0
          });
          
          // Set initial crop data
          const data = cropper.getData(true);
          const cropData = {
            x: Math.round(data.x),
            y: Math.round(data.y),
            width: Math.round(data.width),
            height: Math.round(data.height)
          };
          cropDataInput.value = JSON.stringify(cropData);
        },
        crop: function(e) {
          // Update crop data whenever the crop area changes
          const data = cropper.getData(true);
          
          // Convert from image coordinates to actual image pixel coordinates
          const cropData = {
            x: Math.round(data.x),
            y: Math.round(data.y),
            width: Math.round(data.width),
            height: Math.round(data.height)
          };
          
          cropDataInput.value = JSON.stringify(cropData);
        }
      });
    }

    function initializeCroppers() {
      document.querySelectorAll('input[data-crop-data]').forEach(function(cropDataInput) {
        const container = cropDataInput.closest('.cropper-container') || cropDataInput.parentElement;
        const img = container.querySelector('img');
        
        if (img) {
          if (img.complete) {
            initCropper(img, cropDataInput);
          } else {
            img.addEventListener('load', function() {
              initCropper(img, cropDataInput);
            });
          }
        }
      });
    }

    function init() {
      // Auto-initialize cropper for inputs with data-crop-data attribute
      document.addEventListener('DOMContentLoaded', function() {
        initializeCroppers();
      });
      // Also initialize after HTMX swaps
      document.addEventListener('htmx:afterSwap', function() {
        initializeCroppers();
      });
    }

    return {
      init: init,
      initCropper: initCropper
    };
  })();

  const SiteUI = (function() {
    const STORAGE_KEY = '__FRANKEN_SIDEBAR_COLLAPSED__';
    const MOBILE_BREAKPOINT = '(max-width: 768px)';

    const isMobile = () => window.matchMedia(MOBILE_BREAKPOINT).matches;

    const safeExecute = (fn, errorContext) => {
      try {
        fn();
      } catch (e) {
        console.error(`${errorContext} error:`, e);
      }
    };

    // Sidebar Management
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
        
        if (this.mediaQuery.matches) {
          this.closeMobile();
        }
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

    // Navigation Active State
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

    // Tag Filtering
    const TagFilterManager = {
      getFormElements() {
        const form = document.getElementById('tag-filter-form');
        if (!form) return null;

        return {
          form,
          sortInput: form.querySelector('input[name="sort"]'),
          orderInput: form.querySelector('input[name="order"]'),
          modeInput: form.querySelector('input[name="tag_mode"]'),
          sortSelect: document.getElementById('sort-select'),
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

        if (sort && elements.sortInput) {
          elements.sortInput.value = sort;
        }
        if (params.order && elements.orderInput) {
          elements.orderInput.value = params.order;
        }
        if (elements.modeInput) {
          elements.modeInput.value = params.tagMode;
        }

        if (elements.modeToggle) {
          elements.modeToggle.setAttribute('data-mode', params.tagMode);
          elements.modeToggle.textContent = params.tagMode === 'any' ? 'Any' : 'All';
        }

        // Ensure uk-select reflects the selected value
        if (elements.sortSelect) {
          if (sort) {
            elements.sortSelect.value = sort;
          }
          
          // Force uk-select to update its display
          const ukSelect = elements.sortSelect.closest('uk-select') || document.querySelector('uk-select');
          if (ukSelect) {
            // Manually update the button text
            const button = ukSelect.querySelector('button');
            const selectedOption = elements.sortSelect.querySelector('option:checked');
            if (button) {
              if (selectedOption) {
                button.textContent = selectedOption.textContent.trim();
              } else {
                button.textContent = 'Sort by';
              }
            }
          }
        }
      },

      refreshTagFragment() {
        const path = window.location.pathname || '';
        if (path.startsWith('/account/')) {
          this.syncFormState();
          return;
        }

        const qs = window.location.search || '';
        fetch('/series/tags/fragment' + qs, { credentials: 'same-origin' })
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
        select.dispatchEvent(new Event('change', { bubbles: true }));
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

    // Chapter Eye Icon Hover
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

    // Code Editor Enhancer
    const CodeEditorManager = {
      editors: new Set(),

      currentTheme() {
        return document.documentElement.classList.contains('dark') ? 'dracula' : 'default';
      },

      mountEditors(root) {
        const scope = root instanceof Element ? root : document;
        scope.querySelectorAll('textarea[data-code-editor]:not([data-code-editor-init])')
          .forEach((ta) => this.createEditor(ta));
      },

      createEditor(ta) {
        // Prevent double initialization
        if (ta.dataset.codeEditorInit) return;
        
        // Mark as initializing immediately
        ta.dataset.codeEditorInit = '1';
        
        const mode = ta.dataset.codeEditor || ta.dataset.codeMode || 'shell';
        const height = ta.dataset.codeEditorHeight || '384px';
        
        try {
          const editor = window.CodeMirror.fromTextArea(ta, {
            mode,
            lineNumbers: true,
            lineWrapping: true,
            theme: this.currentTheme()
          });
          editor.setSize('100%', height);
          ta.classList.add('code-editor-hidden');
          const form = ta.closest('form');
          if (form) {
            form.addEventListener('submit', () => editor.save());
          }
          // Store reference to editor on textarea for easy access
          ta._codeMirrorEditor = editor;
          this.editors.add(editor);
        } catch (error) {
          console.error('Failed to create CodeMirror editor:', error);
          // Reset flag if creation failed
          delete ta.dataset.codeEditorInit;
        }
      },

      updateEditorMode(ta, newMode) {
        if (ta._codeMirrorEditor) {
          ta._codeMirrorEditor.setOption('mode', newMode);
        }
      },

      observeTheme() {
        if (this.themeObserver) return;
        this.themeObserver = new MutationObserver(() => {
          const theme = this.currentTheme();
          this.editors.forEach((editor) => editor.setOption('theme', theme));
        });
        this.themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] });
      },

      init() {
        const ready = () => {
          // Wait for CodeMirror to be available
          const waitForCodeMirror = () => {
            if (typeof window.CodeMirror !== 'undefined') {
              this.mountEditors(document.body);
            } else {
              setTimeout(waitForCodeMirror, 50);
            }
          };
          waitForCodeMirror();
        };
        
        if (document.readyState === 'loading') {
          document.addEventListener('DOMContentLoaded', ready);
        } else {
          ready();
        }
        
        document.addEventListener('htmx:afterSwap', (event) => {
          const target = event.detail?.target || event.target;
          if (target) {
            // Wait for CodeMirror to be available after HTMX swap
            const waitForCodeMirror = () => {
              if (typeof window.CodeMirror !== 'undefined') {
                this.mountEditors(target);
              } else {
                setTimeout(waitForCodeMirror, 50);
              }
            };
            waitForCodeMirror();
          }
        });
        
        this.observeTheme();
      }
    };

    // Expose CodeEditorManager globally
    window.CodeEditorManager = CodeEditorManager;

    // Scroll Helpers
    const ScrollHelpers = {
      init() {
        window.scrollToTop = () => {
          const mainContent = document.getElementById('main-content');
          if (mainContent) {
            mainContent.scrollTo({ top: 0, behavior: 'smooth' });
          } else {
            window.scrollTo({ top: 0, behavior: 'smooth' });
          }
        };
        window.scrollToTopInstant = () => {
          const mainContent = document.getElementById('main-content');
          if (mainContent) {
            mainContent.scrollTo({ top: 0, behavior: 'auto' });
          } else {
            window.scrollTo({ top: 0, behavior: 'auto' });
          }
        };
      }
    };

    // Dropdown Auto-close
    const DropdownManager = {
      closeAllDropdowns() {
        if (typeof UIkit !== 'undefined' && UIkit.dropdown) {
          const dropdownElements = document.querySelectorAll('.uk-dropdown');
          dropdownElements.forEach(el => {
            const dropdown = UIkit.dropdown(el);
            if (dropdown && dropdown.isActive()) {
              dropdown.hide(false);
            }
          });
        }
      },

      scrollActiveIntoView(dropdownEl) {
        const activeItem = dropdownEl.querySelector('li.uk-active');
        if (activeItem) {
          setTimeout(() => {
            activeItem.scrollIntoView({
              block: 'center',
              behavior: 'instant'
            });
          }, 50);
        }
      },

      init() {
        document.body.addEventListener('click', (e) => {
          const link = e.target.closest('a[href]');
          const dropdown = e.target.closest('.uk-dropdown');
          
          if (link && dropdown && (link.hasAttribute('hx-get') || link.hasAttribute('hx-post'))) {
            setTimeout(() => this.closeAllDropdowns(), 0);
          }
        });

        document.body.addEventListener('htmx:afterOnLoad', () => {
          this.closeAllDropdowns();
        });

        document.body.addEventListener('shown', (e) => {
          const dropdownEl = e.target;
          if (dropdownEl && dropdownEl.id && dropdownEl.id.includes('chapter-list-drop')) {
            this.scrollActiveIntoView(dropdownEl);
          }
        });
      }
    };
    const ThemeManager = {
      applyTheme() {
        const config = JSON.parse(localStorage.getItem("__FRANKEN__") || "{}");
        const htmlElement = document.documentElement;
        
        // Apply mode
        if (config.mode === 'dark') {
          htmlElement.classList.add('dark');
        } else {
          htmlElement.classList.remove('dark');
        }
        
        // Apply other theme options
        Object.keys(config).forEach(key => {
          if (key !== 'mode') {
            const value = config[key];
            // Remove existing class for this key
            const existingClasses = Array.from(htmlElement.classList).filter(cls => cls.startsWith(`uk-${key}-`));
            existingClasses.forEach(cls => htmlElement.classList.remove(cls));
            // Add new class
            htmlElement.classList.add(value);
          }
        });
      },
      init() {
        // Apply theme on load
        this.applyTheme();
        
        document.addEventListener('click', (e) => {
          if (e.target.classList.contains('theme-option')) {
            e.preventDefault();
            const key = e.target.dataset.key;
            const value = e.target.dataset.value;
            
            // Update localStorage
            const config = JSON.parse(localStorage.getItem("__FRANKEN__") || "{}");
            config[key] = value;
            localStorage.setItem("__FRANKEN__", JSON.stringify(config));
            
            // Update classes
            const htmlElement = document.documentElement;
            if (key === 'mode') {
              if (value === 'dark') {
                htmlElement.classList.add('dark');
              } else {
                htmlElement.classList.remove('dark');
              }
            } else {
              // Remove existing class for this key
              const existingClasses = Array.from(htmlElement.classList).filter(cls => cls.startsWith(`uk-${key}-`));
              existingClasses.forEach(cls => htmlElement.classList.remove(cls));
              // Add new class
              htmlElement.classList.add(value);
            }
            
            // Update active states
            document.querySelectorAll(`.theme-option[data-key="${key}"]`).forEach(btn => {
              btn.classList.remove('uk-active');
            });
            e.target.classList.add('uk-active');
          }
        });
        
        // Reapply theme on HTMX swaps (for history navigation)
        document.addEventListener('htmx:afterSwap', () => {
          this.applyTheme();
        });

        // Parse ANSI colors in log outputs after HTMX swaps
        document.addEventListener('htmx:afterSwap', function() {
          document.querySelectorAll('.log-output').forEach(function(element) {
            if (!element.dataset.ansiParsed) {
              const originalText = element.textContent;
              if (originalText && /\x1b\[([0-9;]*)m/.test(originalText)) {
                element.innerHTML = window.Magi.parseAnsiColors(originalText);
                element.dataset.ansiParsed = 'true';
              }
            }
          });
        });
        
        // Set initial active states
        const config = JSON.parse(localStorage.getItem("__FRANKEN__") || "{}");
        Object.keys(config).forEach(key => {
          const value = config[key];
          const btn = document.querySelector(`.theme-option[data-key="${key}"][data-value="${value}"]`);
          if (btn) {
            btn.classList.add('uk-active');
          }
        });
      }
    };

    // Search Modal Focus Manager
    const SearchModalManager = {
      init() {
        const searchModal = document.getElementById('search-modal');
        if (!searchModal) return;

        // Focus search input when modal is shown
        UIkit.util.on(searchModal, 'shown', () => {
          setTimeout(() => {
            const searchInput = document.getElementById('searchInput');
            if (searchInput) {
              searchInput.focus();
            }
          }, 100); // Small delay to ensure modal is fully rendered
        });
      }
    };

    // Main initialization
    function init() {
      safeExecute(() => SidebarManager.init(), 'Sidebar init');
      safeExecute(() => NavigationManager.init(), 'Navigation sync');
      safeExecute(() => TagFilterManager.init(), 'Tag filtering');
      safeExecute(() => ChapterHoverManager.init(), 'Chapter hover');
      safeExecute(() => CodeEditorManager.init(), 'Code editor');
      safeExecute(() => ScrollHelpers.init(), 'Scroll helpers');
      safeExecute(() => DropdownManager.init(), 'Dropdown manager');
      safeExecute(() => ThemeManager.init(), 'Theme manager');
      safeExecute(() => SearchModalManager.init(), 'Search modal focus');
      safeExecute(() => CropperManager.init(), 'Cropper manager');
      safeExecute(() => ScraperManager.init(), 'Scraper manager');
      safeExecute(() => ConfigManager.init(), 'Config manager');
      safeExecute(() => JobStatusManager.init(), 'Job status manager');
    }

    return { init: init };
  })();

  // ============================================================================
  // JOB STATUS MANAGER MODULE
  // ============================================================================
  
  /**
   * Manages WebSocket connection for job status updates and displays a spinning
   * loader in the sidebar when indexer or scraper jobs are running.
   */
  const JobStatusManager = (function() {
    let ws = null;
    let reconnectTimer = null;
    let activeJobs = [];
    let indicator = null;
    let tooltipDiv = null;

    function connect() {
      if (ws && (ws.readyState === WebSocket.CONNECTING || ws.readyState === WebSocket.OPEN)) {
        return;
      }

      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const wsUrl = `${protocol}//${window.location.host}/ws/job-status`;

      try {
        ws = new WebSocket(wsUrl);

        ws.onopen = function() {
          if (reconnectTimer) {
            clearTimeout(reconnectTimer);
            reconnectTimer = null;
          }
        };

        ws.onmessage = function(event) {
          try {
            const data = JSON.parse(event.data);
            if (data.action === 'jobs_update') {
              updateJobStatus(data.jobs || []);
            }
          } catch (err) {
            console.error('[JobStatus] Error parsing message:', err);
          }
        };

        ws.onerror = function(error) {
          console.error('[JobStatus] WebSocket error:', error);
        };

        ws.onclose = function() {
          ws = null;
          reconnectTimer = setTimeout(connect, 5000);
        };
      } catch (err) {
        console.error('[JobStatus] Failed to create WebSocket:', err);
        reconnectTimer = setTimeout(connect, 5000);
      }
    }

    // Escape special HTML characters in any user-provided string
    function escapeHtml(str) {
      return String(str)
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#39;");
    }

    function updateJobStatus(jobs) {
      activeJobs = jobs;
      
      if (!indicator) {
        console.warn('[JobStatus] Indicator not available for update');
        return;
      }

      if (jobs.length > 0) {
        // Build tooltip text
        const tooltipText = jobs.map(job => {
          if (job.type === 'scraper') {
            return `Scraper: ${escapeHtml(job.name)}`;
          } else if (job.type === 'indexer') {
            let text = `Indexer: ${escapeHtml(job.name)}`;
            if (job.current_manga) {
              text += `<br><small>${escapeHtml(job.current_manga)}</small>`;
            }
            if (job.progress) {
              text += ` <small>(${escapeHtml(job.progress)})</small>`;
            }
            return text;
          }
          return escapeHtml(job.name);
        }).join('<br>');

        // Show the indicator
        indicator.style.display = 'block';

        // Update custom tooltip content
        if (tooltipDiv) {
          tooltipDiv.innerHTML = tooltipText;
          // Reposition tooltip if it's currently shown (to account for size changes)
          if (tooltipDiv.style.display === 'block') {
            updateTooltipPosition();
          }
        }
      } else {
        // No jobs, hide immediately
        indicator.style.display = 'none';
      }
    }

    function init() {
      indicator = document.getElementById('job-status-indicator');
      if (!indicator) {
        console.warn('[JobStatus] Job status indicator element not found');
        return;
      }
      
      // Create custom tooltip
      tooltipDiv = document.createElement('div');
      tooltipDiv.className = 'job-status-tooltip';
      tooltipDiv.style.cssText = 'position: fixed; display: none; background: var(--ui-bg-base, #1f2937); color: var(--text-color, white); padding: 8px 12px; border-radius: 6px; font-size: 13px; line-height: 1.4; z-index: 10000; pointer-events: none; box-shadow: 0 4px 6px rgba(0,0,0,0.3); max-width: 250px;';
      document.body.appendChild(tooltipDiv);
      
      // Show tooltip on hover
      indicator.addEventListener('mouseenter', () => {
        if (activeJobs.length > 0) {
          tooltipDiv.style.display = 'block';
          updateTooltipPosition();
        }
      });
      
      indicator.addEventListener('mouseleave', () => {
        tooltipDiv.style.display = 'none';
      });
      
      console.debug('[JobStatus] Indicator found, initializing WebSocket connection');
      connect();
    }
    
    function updateTooltipPosition() {
      if (!tooltipDiv || !indicator) return;
      
      const rect = indicator.getBoundingClientRect();
      const offset = 10;
      
      // Position to the right of the indicator
      tooltipDiv.style.left = (rect.right + offset) + 'px';
      tooltipDiv.style.top = (rect.top + (rect.height / 2) - (tooltipDiv.offsetHeight / 2)) + 'px';
    }

    function disconnect() {
      if (reconnectTimer) {
        clearTimeout(reconnectTimer);
        reconnectTimer = null;
      }
      if (ws) {
        ws.close();
        ws = null;
      }
    }

    return {
      init: init,
      disconnect: disconnect,
      getActiveJobs: () => activeJobs
    };
  })();

  // ============================================================================
  // GLOBAL EXPORTS
  // ============================================================================

  // Initialize smooth scroll
  SmoothScroll.init();

  // Initialize site UI on DOM ready
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => SiteUI.init());
  } else {
    SiteUI.init();
  }

  // ============================================================================
  // NOTIFICATION SYSTEM
  // ============================================================================

  /**
   * Handle HTMX-triggered notifications
   */
  document.addEventListener('showNotification', function(event) {
    const detail = event.detail;
    if (detail && detail.message) {
      const options = {
        message: detail.message,
        status: detail.status || 'primary',
        timeout: 5000,
        pos: 'top-right'
      };
      UIkit.notification(options);
    }
  });

  // Export to window
  window.Magi = {
    SmoothScroll: SmoothScroll,
    WebSocketHandler: WebSocketHandler,
    LogViewer: LogViewer,
    JobStatusManager: JobStatusManager,
    CropperManager: CropperManager,
    ScraperManager: ScraperManager,
    ConfigManager: ConfigManager,
    parseAnsiColors: parseAnsiColors,
    applyAnsiStyles: applyAnsiStyles,
    escapeHtml: escapeHtml,
    version: '1.0.0'
  };

  // Maintain backward compatibility
  window.WebSocketHandler = WebSocketHandler;
  window.LogViewer = LogViewer;
  window.smoothScroll = SmoothScroll;

  // ============================================================================
  // FILE EXPLORER MODULE
  // ============================================================================

  /**
   * Opens the file explorer modal for selecting a folder path
   * @param {HTMLElement} button - The browse button that was clicked
   */
  window.openFileExplorer = function(button) {
    const targetInputID = button.getAttribute('data-target-input');
    
    // Hide the select button initially
    const selectBtn = document.getElementById('select-folder-btn');
    if (selectBtn) {
      selectBtn.style.display = 'none';
    }
    
    // Load initial directory listing
    fetch('/admin/libraries/helpers/browse?path=/')
      .then(response => response.json())
      .then(entries => {
        // Update the modal content
        const modal = document.getElementById('file-explorer-modal');
        const content = modal.querySelector('#file-explorer-content');
        
        // Create the content HTML
        let html = '<ul class="uk-list uk-list-divider">';
        html += '<li class="uk-text-muted">Current path: /</li>';
        
        entries.forEach(entry => {
          if (entry.IsDir) {
            html += `<li><a href="#" onclick="navigateToFolder('${entry.Path}', '${targetInputID}')" class="uk-link-text"><span class="inline-flex items-center gap-2"><uk-icon icon="Folder"></uk-icon> ${entry.Name}</span></a></li>`;
          } else {
            html += `<li class="uk-text-muted"><span class="inline-flex items-center gap-2"><uk-icon icon="File"></uk-icon> ${entry.Name}</span></li>`;
          }
        });
        
        html += '</ul>';
        
        content.innerHTML = html;
        
        // Update the select button
        const selectBtn = document.getElementById('select-folder-btn');
        selectBtn.style.display = 'block';
        selectBtn.onclick = () => selectFolder('/', targetInputID);
        
        // Show the modal
        UIkit.modal(modal).show();
      })
      .catch(error => {
        console.error('Error loading directory:', error);
      });
  };

  /**
   * Navigates to a folder in the file explorer
   * @param {string} path - The path to navigate to
   * @param {string} targetInputID - The ID of the input to update
   */
  window.navigateToFolder = function(path, targetInputID) {
    fetch('/admin/libraries/helpers/browse?path=' + encodeURIComponent(path))
      .then(response => response.json())
      .then(entries => {
        const modal = document.getElementById('file-explorer-modal');
        const content = modal.querySelector('#file-explorer-content');
        
        let html = '<ul class="uk-list uk-list-divider">';
        
        // Add parent directory link if not at root
        if (path !== '/') {
          const parentPath = path.substring(0, path.lastIndexOf('/')) || '/';
          html += `<li><a href="#" onclick="navigateToFolder('${parentPath}', '${targetInputID}')" class="uk-link-text"><span class="inline-flex items-center gap-2"><uk-icon icon="ArrowLeft"></uk-icon> ..</span></a></li>`;
        }
        
        html += `<li class="uk-text-muted">Current path: ${path}</li>`;
        
        entries.forEach(entry => {
          if (entry.IsDir) {
            html += `<li><a href="#" onclick="navigateToFolder('${entry.Path}', '${targetInputID}')" class="uk-link-text"><span class="inline-flex items-center gap-2"><uk-icon icon="Folder"></uk-icon> ${entry.Name}</span></a></li>`;
          } else {
            html += `<li class="uk-text-muted"><span class="inline-flex items-center gap-2"><uk-icon icon="File"></uk-icon> ${entry.Name}</span></li>`;
          }
        });
        
        html += '</ul>';
        
        content.innerHTML = html;
        
        // Update the select button
        const selectBtn = document.getElementById('select-folder-btn');
        selectBtn.style.display = 'block';
        selectBtn.onclick = () => selectFolder(path, targetInputID);
      })
      .catch(error => {
        console.error('Error loading directory:', error);
      });
  };

  /**
   * Selects the current folder and closes the modal
   * @param {string} path - The selected path
   * @param {string} targetInputID - The ID of the input to update
   */
  window.selectFolder = function(path, targetInputID) {
    const input = document.getElementById(targetInputID);
    if (input) {
      input.value = path;
    }
    
    // Hide the select button
    const selectBtn = document.getElementById('select-folder-btn');
    if (selectBtn) {
      selectBtn.style.display = 'none';
    }
    
    // Close the modal
    UIkit.modal(document.getElementById('file-explorer-modal')).hide();
  };

  // ============================================================================
  // SLIDER DRAG PREVENTION MODULE
  // ============================================================================
  
  /**
   * Prevents clicks on slider items when they were dragged instead of clicked.
   * This fixes the issue where dragging the slider navigates to unintended series.
   */
  const SliderDragPrevention = (function() {
    let isDragging = false;
    let dragThreshold = 5; // pixels
    let startX, startY;
    
    function init() {
      // Track mousedown on slider
      document.addEventListener('mousedown', function(e) {
        if (e.target.closest('[uk-slider]')) {
          startX = e.clientX;
          startY = e.clientY;
          isDragging = false;
        }
      });
      
      // Track mousemove to detect drag
      document.addEventListener('mousemove', function(e) {
        if (startX !== undefined && !isDragging) {
          const deltaX = Math.abs(e.clientX - startX);
          const deltaY = Math.abs(e.clientY - startY);
          if (deltaX > dragThreshold || deltaY > dragThreshold) {
            isDragging = true;
          }
        }
      });
      
      // Reset on mouseup
      document.addEventListener('mouseup', function() {
        startX = undefined;
        startY = undefined;
        // Keep isDragging true briefly to prevent the click
        setTimeout(function() {
          isDragging = false;
        }, 50); // Increased timeout
      });
      
      // Prevent clicks on slider items if we detected a drag
      document.addEventListener('click', function(e) {
        if (isDragging && e.target.closest('[uk-slider] a')) {
          e.preventDefault();
          e.stopPropagation();
          isDragging = false;
          return false;
        }
      }, true); // Use capture phase
    }
    
    return {
      init: init
    };
  })();
  
  // Initialize slider drag prevention
  SliderDragPrevention.init();

})(window);
