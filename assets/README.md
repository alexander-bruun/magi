# Magi Assets Structure

This directory contains all static assets for the Magi application, organized for clarity and maintainability.

## Directory Structure

```
assets/
├── css/
│   ├── styles.css                 # Custom application styles
│   └── vendor/                    # Third-party CSS libraries
│       ├── codemirror.min.css     # CodeMirror editor styles
│       ├── core.min.css           # Franken UI core styles
│       ├── cropper.min.css        # Cropper.js image cropping
│       ├── dracula.min.css        # Dracula theme for CodeMirror
│       └── utilities.min.css      # Franken UI utilities
├── js/
│   ├── magi.js                    # Core Magi library (consolidated)
│   ├── reader.js                  # Manga reader module
│   └── vendor/                    # Third-party JavaScript libraries
│       ├── codemirror.min.js      # CodeMirror editor
│       ├── core.iife.js           # Franken UI core
│       ├── cropper.min.js         # Cropper.js image cropping
│       ├── htmx.min.js            # HTMX for AJAX interactions
│       ├── icon.iife.js           # Franken UI icon system
│       └── shell.min.js           # Shell syntax highlighting
└── img/                           # Application images
```

## Core Libraries

### magi.js
The consolidated core library containing:
- **SmoothScroll**: Smooth scrolling utilities for HTMX navigation
- **WebSocketHandler**: Generic WebSocket client with auto-reconnection
- **LogViewer**: Real-time log streaming component
- **SiteUI**: Site-wide UI interactions (sidebar, navigation, tags, etc.)

All components are globally available via the `window.Magi` namespace.

### reader.js
Specialized module for manga chapter reading with support for:
- Webtoon (vertical scroll) mode
- Single page mode
- Side-by-side reading mode
- Focus/zoom capabilities
- Reading progress tracking

## Usage

### In Templates

The core `magi.js` is automatically loaded in the base layout, so all components are available globally:

```html
<!-- WebSocket Log Viewer -->
<script>
  new LogViewer({
    wsUrl: 'ws://localhost/logs',
    containerId: 'log-container',
    outputId: 'log-output'
  });
</script>

<!-- Smooth Scroll -->
<div data-smooth-scroll data-scroll-offset="80" 
     hx-get="/api/data" hx-target="this">
  Content will scroll here after HTMX loads
</div>
```

### Backward Compatibility

All classes maintain backward compatibility:
- `window.WebSocketHandler` - Direct access to WebSocket handler
- `window.LogViewer` - Direct access to log viewer
- `window.smoothScroll` - Smooth scroll utilities
- `window.Magi` - Main namespace with all components

## Development

When adding new JavaScript:
1. **In-house utilities**: Add to `magi.js` under appropriate module
2. **Specialized modules**: Create separate files (like `reader.js`)
3. **Third-party libraries**: Place in `vendor/` directories

## File Sizes

- `magi.js`: ~30KB (unminified)
- `reader.js`: ~33KB (unminified)
- Vendor files: Minified versions from CDNs

## Version

Current version: 1.0.0
