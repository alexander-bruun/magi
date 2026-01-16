(function() {
    'use strict';

    // Constants
    const MODES = {
        WEBTOON: 'webtoon',
        SINGLE: 'single',
        SIDE_BY_SIDE: 'side-by-side'
    };

    const STORAGE_KEYS = {
        READING_MODE: 'magi-reading-mode',
        FOCUS_STATE: 'magi-focus-state',
        NOVEL_SETTINGS: 'lightNovelReaderSettings'
    };

    const DIMENSIONS = {
        PLACEHOLDER_SIZE: 100,
        LAZY_LOAD_MARGIN: 200,
        INITIAL_LOAD_DELAY: 100,
        HEADER_HEIGHT_FALLBACK: 64,
        DROPDOWN_HEIGHT_FALLBACK: 100,
        FOCUS_Z_INDEX: 10002,
        PAGINATION_BOTTOM_OFFSET: 10
    };

    const SELECTORS = {
        CONTAINER: '#reader-images-container',
        PAGINATION_BOTTOM: '#reader-pagination-bottom',
        HEADER: 'nav, header, .uk-navbar',
        DROPDOWN: '#chapter-dropdown, .chapter-dropdown, .uk-dropdown'
    };

    // Utility functions for dynamic sizing
    const getElementHeight = (selector, fallback) => {
        const element = document.querySelector(selector);
        return element ? element.offsetHeight : fallback;
    };

    const getHeaderHeight = () => getElementHeight(SELECTORS.HEADER, DIMENSIONS.HEADER_HEIGHT_FALLBACK);
    const getDropdownHeight = () => getElementHeight(SELECTORS.DROPDOWN, DIMENSIONS.DROPDOWN_HEIGHT_FALLBACK);

    const getViewportHeight = () => window.innerHeight;

    const saveToStorage = (key, data) => {
        try { localStorage.setItem(key, JSON.stringify(data)); } catch (e) { console.warn(`Failed to save ${key}:`, e); }
    };

    const loadFromStorage = (key) => {
        try {
            const item = localStorage.getItem(key);
            if (item === null) return null;
            // Try to parse as JSON, if fails, return as string (for backward compatibility)
            try {
                return JSON.parse(item);
            } catch (parseError) {
                return item; // Return raw string if not JSON
            }
        } catch (e) {
            console.warn(`Failed to load ${key}:`, e);
            return null;
        }
    };

    // Reader class for modularity
    class Reader {
        constructor(containerSelector) {
            this.container = document.querySelector(containerSelector);
            if (!this.container) throw new Error('Reader container not found');

            this.currentMode = MODES.WEBTOON;
            this.currentPage = 0;
            this.images = [];
            this.lazyTokens = [];
            this.loadedCanvases = [];
            this.focusModal = null;
            this.isLightNovel = window.isLightNovel || false;
            this.observer = null;
            this.isInitialLoad = true;
            this.focusStateData = {};
            this.scrollPositionBeforeFocus = 0;
            this.focusModalZoom = 1;
            this.currentFocusIndex = 0;

            this.init();
        }

        init() {
            this.loadSettings();
            // Collect tokens from divs and remove them
            const tokenDivs = Array.from(this.container.querySelectorAll('.reader-token'));
            this.images = tokenDivs.map(div => div.dataset.token).filter(token => token && token.trim() && token !== 'undefined');
            // Remove all token divs
            tokenDivs.forEach(div => div.remove());
            this.setupEventListeners();
            this.setupLazyLoading();
            this.loadInitialImages();
            this.updateMode();
        }

        loadSettings() {
            this.currentMode = loadFromStorage(STORAGE_KEYS.READING_MODE) || MODES.WEBTOON;
        }

        saveSettings() {
            saveToStorage(STORAGE_KEYS.READING_MODE, this.currentMode);
        }

        setupEventListeners() {
            window.addEventListener('resize', () => this.handleResize());
            window.addEventListener('keydown', (e) => this.handleKeydown(e));

            // Pagination buttons
            const prevBtn = document.getElementById('prev-page-btn');
            const nextBtn = document.getElementById('next-page-btn');
            if (prevBtn) prevBtn.addEventListener('click', () => this.prevPage());
            if (nextBtn) nextBtn.addEventListener('click', () => this.nextPage());
        }

        setupLazyLoading() {
            this.observer = new IntersectionObserver(
                (entries) => {
                    entries.forEach(entry => {
                        if (entry.isIntersecting && this.lazyTokens.length > 0 && !this.isInitialLoad) {
                            this.loadNextImage();
                        }
                    });
                },
                {
                    root: null,
                    rootMargin: `0px 0px ${DIMENSIONS.LAZY_LOAD_MARGIN}px 0px`,
                    threshold: 0
                }
            );
        }

        loadInitialImages() {
            const initialTokens = this.images.slice(0, 2);
            this.lazyTokens = [];
            this.currentLoadedIndex = 0;

            const loadPromises = initialTokens.map((token, i) => this.createAndLoadCanvas(token, i));

            Promise.all(loadPromises).then(() => {
                this.updateMode();
                // Set up intersection observer for lazy loading
                const observer = new IntersectionObserver((entries) => {
                    entries.forEach(entry => {
                        if (entry.isIntersecting) {
                            const canvas = entry.target;
                            const imageIndex = parseInt(canvas.dataset.index);
                            if (imageIndex >= this.currentLoadedIndex && imageIndex < this.images.length) {
                                this.loadImageToCanvas(canvas, `/api/image?token=${this.images[imageIndex]}`);
                                this.currentLoadedIndex++;
                                observer.unobserve(canvas);
                            }
                        }
                    });
                }, { rootMargin: '100px' });

                // Create placeholder canvases for remaining images
                for (let i = 2; i < this.images.length; i++) {
                    const canvas = document.createElement('canvas');
                    canvas.className = 'reader-canvas';
                    canvas.width = DIMENSIONS.PLACEHOLDER_SIZE;
                    canvas.height = DIMENSIONS.PLACEHOLDER_SIZE;
                    canvas.style.display = 'block';
                    canvas.dataset.index = i;
                    canvas.addEventListener('click', (e) => {
                        e.preventDefault();
                        this.openFocusModal(canvas);
                    });
                    this.container.appendChild(canvas);
                    this.lazyTokens.push(canvas);
                    observer.observe(canvas);
                }
                this.isInitialLoad = false;
            });
        }

        createAndLoadCanvas(token, index) {
            const canvas = document.createElement('canvas');
            canvas.className = 'reader-canvas';
            canvas.width = DIMENSIONS.PLACEHOLDER_SIZE;
            canvas.height = DIMENSIONS.PLACEHOLDER_SIZE;
            canvas.style.display = 'block';
            canvas.dataset.index = index;
            this.container.appendChild(canvas);
            this.loadedCanvases[index] = canvas;
            return this.loadImageToCanvas(canvas, `/api/image?token=${token}`);
        }

        async loadImageToCanvas(canvas, url) {
            try {
                const response = await fetch(url);
                if (!response.ok) throw new Error(`Fetch failed: ${response.status}`);
                const blob = await response.blob();
                const bitmap = await createImageBitmap(blob);
                const ctx = canvas.getContext('2d');

                let width = bitmap.width;
                let height = bitmap.height;

                if (this.currentMode === MODES.WEBTOON) {
                    // No scaling, let CSS handle sizing
                }

                canvas.width = width;
                canvas.height = height;
                ctx.drawImage(bitmap, 0, 0, width, height);
                canvas.style.width = '100%';
                canvas.style.height = 'auto';
                canvas.style.objectFit = 'contain';

                // Update focus modal if active
                if (this.focusModal && this.focusModal.classList.contains('active')) {
                    await this.updateFocusModal();
                }
            } catch (error) {
                console.error('Failed to load image:', error);
            }
        }

        loadNextImage() {
            if (this.lazyTokens.length === 0) return;
            const numToLoad = Math.min(2, this.lazyTokens.length);
            const loadPromises = [];

            for (let i = 0; i < numToLoad; i++) {
                const token = this.lazyTokens.shift();
                const canvas = document.createElement('canvas');
                canvas.className = 'reader-canvas';
                canvas.width = DIMENSIONS.PLACEHOLDER_SIZE;
                canvas.height = DIMENSIONS.PLACEHOLDER_SIZE;
                canvas.style.height = 'auto';
                canvas.style.display = 'block';
                canvas.dataset.index = this.currentLoadedIndex + i;
                this.container.appendChild(canvas);
                loadPromises.push(this.loadImageToCanvas(canvas, `/api/image?token=${token}`));
                this.currentLoadedIndex++;
            }

            Promise.all(loadPromises).then(() => {
                const lastCanvas = this.container.querySelector('.reader-canvas:last-child');
                if (lastCanvas) this.observer.observe(lastCanvas);
            });
        }

        updateMode() {
            if (this.isLightNovel) return;

            // Reset container styles
            this.container.style.overflowY = '';
            this.container.style.height = '';
            this.container.style.position = '';
            this.container.style.textAlign = '';
            this.container.style.display = '';
            this.container.style.justifyContent = '';
            this.container.style.alignItems = '';
            this.container.style.gap = '';
            this.container.style.maxWidth = '';
            this.container.className = '';

            this.loadedCanvases.forEach(canvas => {
                if (canvas) {
                    canvas.style.display = 'none';
                    canvas.style.maxWidth = '';
                    canvas.style.height = '';
                    canvas.style.width = '';
                    canvas.style.position = '';
                    canvas.style.left = '';
                    canvas.style.top = '';
                }
            });

            if (this.currentMode === MODES.WEBTOON) {
                this.setupWebtoonMode();
            }

            this.updatePaginationVisibility();
            this.attachCanvasListeners();
            this.updatePageCounter();
        }

        setupWebtoonMode() {
            this.container.className = 'flex flex-col p-0 w-full mx-auto';
            this.container.style.display = '';
            this.container.style.justifyContent = '';
            this.container.style.alignItems = '';
            this.container.style.gap = '';
            this.container.style.maxWidth = '60%';
            this.loadedCanvases.forEach(canvas => {
                if (canvas) {
                    canvas.style.display = 'block';
                }
            });
        }

        loadImageForPage(pageIndex) {
            const token = this.images[pageIndex];
            const canvas = document.createElement('canvas');
            canvas.className = 'reader-canvas';
            canvas.width = DIMENSIONS.PLACEHOLDER_SIZE;
            canvas.height = DIMENSIONS.PLACEHOLDER_SIZE;
            canvas.style.display = 'block';
            this.container.appendChild(canvas);
            this.loadedCanvases[pageIndex] = canvas;
            this.loadImageToCanvas(canvas, `/api/image?token=${token}`);
        }

        updatePaginationVisibility() {
            const paginationBottom = document.querySelector(SELECTORS.PAGINATION_BOTTOM);
            if (paginationBottom) paginationBottom.style.display = 'none';
        }

        attachCanvasListeners() {
            this.loadedCanvases.forEach(canvas => {
                if (canvas) {
                    canvas.addEventListener('click', () => {
                        this.openFocusModal(canvas);
                    });
                }
            });
        }

        updatePageCounter() {
            const counter = document.getElementById('page-counter');
            const counterBottom = document.getElementById('page-counter-bottom');
            const total = this.images.length;
            const current = this.currentPage + 1;
            const text = total > 0 ? `${current} / ${total}` : 'No images';
            if (counter) counter.textContent = text;
            if (counterBottom) counterBottom.textContent = text;
        }

        handleResize() {
            // No action needed for webtoon mode
        }

        handleKeydown(e) {
            switch (e.key) {
                case 'ArrowLeft':
                case 'ArrowUp':
                    e.preventDefault();
                    this.prevPage();
                    break;
                case 'ArrowRight':
                case 'ArrowDown':
                case ' ':
                    e.preventDefault();
                    this.nextPage();
                    break;
                case 'Escape':
                    if (this.focusModal) {
                        this.closeFocusModal();
                    }
                    break;
            }
        }

        async nextPage() {
            if (this.focusModal && this.focusModal.classList.contains('active')) {
                // Handle focus modal navigation
                const increment = this.focusModalMode === MODES.SIDE_BY_SIDE ? 2 : 1;
                const maxPage = this.images.length - 1;
                if (this.currentFocusIndex < maxPage) {
                    this.currentFocusIndex += increment;
                    await this.updateFocusModal();
                }
                return;
            }

            if (this.currentMode === MODES.WEBTOON) {
                window.scrollBy(0, window.innerHeight);
                return;
            }
        }

        async prevPage() {
            if (this.focusModal && this.focusModal.classList.contains('active')) {
                // Handle focus modal navigation
                const decrement = this.focusModalMode === MODES.SIDE_BY_SIDE ? 2 : 1;
                if (this.currentFocusIndex >= decrement) {
                    this.currentFocusIndex -= decrement;
                    await this.updateFocusModal();
                }
                return;
            }

            if (this.currentMode === MODES.WEBTOON) {
                window.scrollBy(0, -window.innerHeight);
                return;
            }
        }

        async changeMode(newMode) {
            if (newMode === MODES.WEBTOON) {
                this.currentMode = newMode;
                this.saveSettings();
                this.currentPage = 0;
                this.updateMode();
            } else if (this.focusModal && this.focusModal.classList.contains('active')) {
                this.focusModalMode = newMode;
                await this.updateFocusModal();
            }
        }

        async openFocusModal(canvas) {
            if (!this.focusModal) {
                this.focusModal = document.createElement('div');
                this.focusModal.className = 'focus-modal';
                this.focusModal.innerHTML = `
                    <div class="focus-modal-overlay"></div>
                    <div class="focus-modal-content">
                        <button class="focus-nav-btn focus-prev-btn" style="background: linear-gradient(to right, rgba(0,0,0,0.8) 0%, rgba(0,0,0,0.6) 25%, rgba(0,0,0,0.4) 50%, rgba(0,0,0,0.2) 75%, rgba(0,0,0,0) 100%); color: white; border: none; cursor: pointer; position: fixed; left: 0; top: 0; width: 100px; height: 100vh; display: flex; justify-content: center; align-items: center; z-index: 10001;">
                            <uk-icon height=64 width=64 icon="chevron-left"></uk-icon>
                        </button>
                        <div class="focus-canvas-container" style="display: flex; justify-content: center; align-items: center; height: 100vh; gap: 0px;">
                            <canvas class="focus-canvas focus-canvas-left"></canvas>
                            <canvas class="focus-canvas focus-canvas-right" style="display: none;"></canvas>
                        </div>
                        <button class="focus-nav-btn focus-next-btn" style="background: linear-gradient(to left, rgba(0,0,0,0.8) 0%, rgba(0,0,0,0.6) 25%, rgba(0,0,0,0.4) 50%, rgba(0,0,0,0.2) 75%, rgba(0,0,0,0) 100%); color: white; border: none; cursor: pointer; position: fixed; right: 0; top: 0; width: 100px; height: 100vh; display: flex; justify-content: center; align-items: center; z-index: 10001;">
                            <uk-icon height=64 width=64 icon="chevron-right"></uk-icon>
                        </button>
                        <button class="uk-btn uk-btn-default focus-toggle-btn" style="position: fixed; top: 10px; right: 60px; z-index: 10002; cursor: pointer;">
                            Single
                        </button>
                        <button class="uk-btn uk-btn-destructive btn-circular focus-close-btn" style="position: fixed; top: 10px; right: 10px; z-index: 10002; cursor: pointer;">
                            <uk-icon height=32 width=32 icon="X"></uk-icon>
                        </button>
                        <div class="focus-page-number" style="position: fixed; bottom: 10px; left: 50%; transform: translateX(-50%); background: rgba(0,0,0,0.8); color: white; padding: 10px; border-radius: 5px; z-index: 10002;"></div>
                    </div>
                `;
                document.body.appendChild(this.focusModal);

                this.focusModal.querySelector('.focus-modal-overlay').addEventListener('click', () => this.closeFocusModal());
                this.focusModal.querySelector('.focus-prev-btn').addEventListener('click', () => this.prevPage());
                this.focusModal.querySelector('.focus-next-btn').addEventListener('click', () => this.nextPage());
                this.focusModal.querySelector('.focus-toggle-btn').addEventListener('click', () => this.toggleFocusMode());
                this.focusModal.querySelector('.focus-close-btn').addEventListener('click', () => this.closeFocusModal());
            }

            this.focusModalMode = MODES.SINGLE;
            this.currentFocusIndex = parseInt(canvas.dataset.index);

            this.focusModal.classList.add('active');
            await this.updateFocusModal();

            this.scrollPositionBeforeFocus = window.scrollY;
            document.body.style.overflow = 'hidden';
        }

        async updateFocusModal() {
            if (!this.focusModal || !this.focusModal.classList.contains('active')) return;

            const leftCanvas = this.focusModal.querySelector('.focus-canvas-left');
            const rightCanvas = this.focusModal.querySelector('.focus-canvas-right');

            // Clear canvases to prevent showing previous images
            const leftCtx = leftCanvas.getContext('2d');
            leftCtx.clearRect(0, 0, leftCanvas.width, leftCanvas.height);
            const rightCtx = rightCanvas.getContext('2d');
            rightCtx.clearRect(0, 0, rightCanvas.width, rightCanvas.height);

            if (this.focusModalMode === MODES.SINGLE) {
                this.focusModal.style.flexDirection = 'column';
                const canvas = this.loadedCanvases[this.currentFocusIndex];
                const contentDiv = document.getElementById('content');
                const maxW = contentDiv ? contentDiv.getBoundingClientRect().width : window.innerWidth * 0.9;
                const maxH = window.innerHeight * 0.9;
                if (!canvas) {
                    // Load the image on demand
                    const token = this.images[this.currentFocusIndex];
                    try {
                        const response = await fetch(`/api/image?token=${token}`);
                        if (!response.ok) throw new Error(`Fetch failed: ${response.status}`);
                        const blob = await response.blob();
                        const bitmap = await createImageBitmap(blob);
                        const scale = Math.min(1, maxW / bitmap.width, maxH / bitmap.height);
                        leftCanvas.width = bitmap.width * scale;
                        leftCanvas.height = bitmap.height * scale;
                        const ctx = leftCanvas.getContext('2d');
                        ctx.drawImage(bitmap, 0, 0, bitmap.width, bitmap.height, 0, 0, leftCanvas.width, leftCanvas.height);
                    } catch (error) {
                        console.error('Failed to load image for focus modal:', error);
                    }
                } else {
                    const scale = Math.min(1, maxW / canvas.width, maxH / canvas.height);
                    leftCanvas.width = canvas.width * scale;
                    leftCanvas.height = canvas.height * scale;
                    const ctx = leftCanvas.getContext('2d');
                    ctx.drawImage(canvas, 0, 0, canvas.width, canvas.height, 0, 0, leftCanvas.width, leftCanvas.height);
                }
                leftCanvas.style.width = '100%';
                leftCanvas.style.height = 'auto';
                leftCanvas.style.display = 'block';
                rightCanvas.style.display = 'none';
            } else if (this.focusModalMode === MODES.SIDE_BY_SIDE) {
                this.focusModal.style.flexDirection = 'row';
                const contentDiv = document.getElementById('content');
                const maxW = contentDiv ? contentDiv.getBoundingClientRect().width * 0.49 : window.innerWidth * 0.49;
                const maxH = window.innerHeight * 0.9;
                // Load left image
                const leftImg = this.loadedCanvases[this.currentFocusIndex];
                if (!leftImg) {
                    const token = this.images[this.currentFocusIndex];
                    try {
                        const response = await fetch(`/api/image?token=${token}`);
                        if (!response.ok) throw new Error(`Fetch failed: ${response.status}`);
                        const blob = await response.blob();
                        const bitmap = await createImageBitmap(blob);
                        const scale = Math.min(1, maxW / bitmap.width, maxH / bitmap.height);
                        leftCanvas.width = bitmap.width * scale;
                        leftCanvas.height = bitmap.height * scale;
                        const ctx = leftCanvas.getContext('2d');
                        ctx.drawImage(bitmap, 0, 0, bitmap.width, bitmap.height, 0, 0, leftCanvas.width, leftCanvas.height);
                    } catch (error) {
                        console.error('Failed to load left image for focus modal:', error);
                    }
                } else {
                    const scale = Math.min(1, maxW / leftImg.width, maxH / leftImg.height);
                    leftCanvas.width = leftImg.width * scale;
                    leftCanvas.height = leftImg.height * scale;
                    const ctxLeft = leftCanvas.getContext('2d');
                    ctxLeft.drawImage(leftImg, 0, 0, leftImg.width, leftImg.height, 0, 0, leftCanvas.width, leftCanvas.height);
                }

                // Load right image
                const rightImg = this.loadedCanvases[this.currentFocusIndex + 1];
                if (rightImg) {
                    const scale = Math.min(1, maxW / rightImg.width, maxH / rightImg.height);
                    rightCanvas.width = rightImg.width * scale;
                    rightCanvas.height = rightImg.height * scale;
                    const ctxRight = rightCanvas.getContext('2d');
                    ctxRight.drawImage(rightImg, 0, 0, rightImg.width, rightImg.height, 0, 0, rightCanvas.width, rightCanvas.height);
                    rightCanvas.style.display = 'block';
                } else if (this.currentFocusIndex + 1 < this.images.length) {
                    const token = this.images[this.currentFocusIndex + 1];
                    try {
                        const response = await fetch(`/api/image?token=${token}`);
                        if (!response.ok) throw new Error(`Fetch failed: ${response.status}`);
                        const blob = await response.blob();
                        const bitmap = await createImageBitmap(blob);
                        const scale = Math.min(1, maxW / bitmap.width, maxH / bitmap.height);
                        rightCanvas.width = bitmap.width * scale;
                        rightCanvas.height = bitmap.height * scale;
                        const ctx = rightCanvas.getContext('2d');
                        ctx.drawImage(bitmap, 0, 0, bitmap.width, bitmap.height, 0, 0, rightCanvas.width, rightCanvas.height);
                        rightCanvas.style.display = 'block';
                    } catch (error) {
                        console.error('Failed to load right image for focus modal:', error);
                        rightCanvas.style.display = 'none';
                    }
                } else {
                    rightCanvas.style.display = 'none';
                }

                leftCanvas.style.display = 'block';
            }

            this.updateFocusPageNumber();
        }

        async toggleFocusMode() {
            if (this.focusModalMode === MODES.SINGLE) {
                this.focusModalMode = MODES.SIDE_BY_SIDE;
            } else {
                this.focusModalMode = MODES.SINGLE;
            }
            const toggleBtn = this.focusModal.querySelector('.focus-toggle-btn');
            toggleBtn.textContent = this.focusModalMode === MODES.SINGLE ? 'Single' : 'Side-by-Side';
            await this.updateFocusModal();
        }

        updateFocusPageNumber() {
            const pageNumberDiv = this.focusModal.querySelector('.focus-page-number');
            if (!pageNumberDiv) return;

            const total = this.images.length;
            if (this.focusModalMode === MODES.SINGLE) {
                const current = this.currentFocusIndex + 1;
                pageNumberDiv.textContent = `Page ${current} of ${total}`;
            } else {
                const start = this.currentFocusIndex + 1;
                const end = Math.min(this.currentFocusIndex + 2, total);
                pageNumberDiv.textContent = `Pages ${start}-${end} of ${total}`;
            }
        }

        closeFocusModal() {
            if (this.focusModal) {
                this.focusModal.classList.remove('active');
                document.body.style.overflow = '';
                window.scrollTo(0, this.scrollPositionBeforeFocus);
            }
        }
    }

    // Initialize when DOM is ready
    function initializeReaders() {
        console.log('[Reader] Initializing readers...');

        const container = document.querySelector(SELECTORS.CONTAINER);
        console.log('[Reader] Container found:', !!container);

        if (container) {
            const reader = new Reader(SELECTORS.CONTAINER);
            // Expose reader globally if needed
            window.reader = reader;
            console.log('[Reader] Manga reader initialized');
        }

        // Light Novel Reader Logic
        console.log('[Reader] Calling initializeLightNovelReader');
        initializeLightNovelReader();
    }

    function initializeLightNovelReader() {
        console.log('[Light Novel Reader] Initializing...');

        // Check if this is a light novel page
        const epubReader = document.querySelector('.epub-reader');
        console.log('[Light Novel Reader] epub-reader element found:', !!epubReader);

        if (!epubReader) {
            console.log('[Light Novel Reader] No epub-reader found, skipping initialization');
            return;
        }

        console.log('[Light Novel Reader] epub-reader found, proceeding with initialization');

        // Reader customization variables
        let currentFontSize = 18;
        const minFontSize = 12;
        const maxFontSize = 28;
        const fontSizeStep = 2;

        const minMargin = 0;
        const maxMargin = 100;
        const marginStep = 5;

        let currentTextColor = null; // null means use default
        let currentBgColor = null; // null means transparent
        let currentTextAlign = 'justify';
        let currentMargin = 20; // default margin in pixels

        function loadReaderSettings() {
            console.log('[Light Novel Reader] Loading settings...');

            // Check for new settings format first
            const saved = localStorage.getItem('novelReaderSettings');
            console.log('[Light Novel Reader] Saved settings found:', !!saved);

            if (saved) {
                const settings = JSON.parse(saved);
                currentFontSize = settings.fontSize || 18;
                currentTextColor = settings.textColor || null;
                currentBgColor = settings.bgColor || null;
                currentTextAlign = settings.textAlign || 'justify';
                currentMargin = settings.margin || 20;
                console.log('[Light Novel Reader] Loaded settings:', { currentFontSize, currentTextColor, currentBgColor, currentTextAlign, currentMargin });
            } else {
                // Backward compatibility: check for old font size setting
                const oldFontSize = localStorage.getItem('novelFontSize');
                console.log('[Light Novel Reader] Old font size setting found:', !!oldFontSize);

                if (oldFontSize) {
                    currentFontSize = parseInt(oldFontSize);
                    // Remove old setting and save new format
                    localStorage.removeItem('novelFontSize');
                    saveReaderSettings();
                    console.log('[Light Novel Reader] Migrated old font size:', currentFontSize);
                }
            }
            applyReaderSettings();
        }

        function saveReaderSettings() {
            const settings = {
                fontSize: currentFontSize,
                textColor: currentTextColor,
                bgColor: currentBgColor,
                textAlign: currentTextAlign,
                margin: currentMargin
            };
            localStorage.setItem('novelReaderSettings', JSON.stringify(settings));
        }

        function applyReaderSettings() {
            console.log('[Light Novel Reader] Applying settings: fontSize=', currentFontSize, 'textAlign=', currentTextAlign);
            const reader = document.querySelector('.epub-reader');
            const card = reader.closest('.uk-card');

            // Apply font size
            reader.style.setProperty('font-size', currentFontSize + 'px', 'important');
            const fontSizeDisplay = document.getElementById('current-font-size');
            if (fontSizeDisplay) fontSizeDisplay.textContent = currentFontSize + 'px';

            // Apply text color if set
            if (currentTextColor) {
                reader.style.setProperty('color', currentTextColor, 'important');
                const elements = reader.querySelectorAll('p, h1, h2, h3, h4, h5, h6, span, div, a');
                elements.forEach(el => {
                    el.style.setProperty('color', currentTextColor, 'important');
                });
            } else {
                // Reset to default
                reader.style.removeProperty('color');
                const elements = reader.querySelectorAll('p, h1, h2, h3, h4, h5, h6, span, div, a');
                elements.forEach(el => {
                    el.style.removeProperty('color');
                });
            }

            // Apply background to the card
            if (currentBgColor) {
                card.style.setProperty('background-color', currentBgColor, 'important');
                reader.setAttribute('data-bg-color', currentBgColor);
            } else {
                card.style.removeProperty('background-color');
                reader.removeAttribute('data-bg-color');
            }

            // Apply text alignment directly
            reader.style.setProperty('text-align', currentTextAlign, 'important');

            // Apply margin
            document.documentElement.style.setProperty('--reader-margin', currentMargin + 'px');

            // Update UI controls
            const fontColorPicker = document.getElementById('font-color-picker');
            if (fontColorPicker) fontColorPicker.value = currentTextColor || '#000000';
            const bgColorPicker = document.getElementById('bg-color-picker');
            if (bgColorPicker) bgColorPicker.value = currentBgColor || '#ffffff';

            // Update alignment buttons
            document.querySelectorAll('.text-align-btn').forEach(btn => {
                btn.classList.toggle('uk-btn-primary', btn.getAttribute('data-align') === currentTextAlign);
                btn.classList.toggle('uk-btn-default', btn.getAttribute('data-align') !== currentTextAlign);
            });

            // Update margin display
            const marginDisplay = document.getElementById('current-margin');
            if (marginDisplay) marginDisplay.textContent = currentMargin + 'px';
        }

        function increaseFontSize() {
            console.log('[Light Novel Reader] Increasing font size from', currentFontSize);
            if (currentFontSize < maxFontSize) {
                currentFontSize += fontSizeStep;
                applyReaderSettings();
                saveReaderSettings();
                console.log('[Light Novel Reader] Font size increased to', currentFontSize);
            } else {
                console.log('[Light Novel Reader] Font size already at max');
            }
        }

        function decreaseFontSize() {
            console.log('[Light Novel Reader] Decreasing font size from', currentFontSize);
            if (currentFontSize > minFontSize) {
                currentFontSize -= fontSizeStep;
                applyReaderSettings();
                saveReaderSettings();
                console.log('[Light Novel Reader] Font size decreased to', currentFontSize);
            } else {
                console.log('[Light Novel Reader] Font size already at min');
            }
        }

        function increaseMargin() {
            if (currentMargin < maxMargin) {
                currentMargin += marginStep;
                applyReaderSettings();
                saveReaderSettings();
            }
        }

        function decreaseMargin() {
            if (currentMargin > minMargin) {
                currentMargin -= marginStep;
                applyReaderSettings();
                saveReaderSettings();
            }
        }

        // Initialize on page load or HTMX swap
        loadReaderSettings();

        // Font size button handlers
        const fontSizeBtns = document.querySelectorAll('.font-size-btn');
        console.log('[Light Novel Reader] Font size buttons found:', fontSizeBtns.length);

        document.querySelectorAll('.font-size-btn').forEach(btn => {
            console.log('[Light Novel Reader] Attaching listener to font size button');
            btn.addEventListener('click', function() {
                console.log('[Light Novel Reader] Font size button event fired');
                const action = this.getAttribute('data-action');
                console.log('[Light Novel Reader] Font size button clicked:', action);
                if (action === 'increase') {
                    increaseFontSize();
                } else if (action === 'decrease') {
                    decreaseFontSize();
                }
            });
        });

        // Color picker handlers
        const fontColorPicker = document.getElementById('font-color-picker');
        if (fontColorPicker) {
            fontColorPicker.addEventListener('input', function(e) {
                currentTextColor = e.target.value;
                applyReaderSettings();
                saveReaderSettings();
            });
        }

        const bgColorPicker = document.getElementById('bg-color-picker');
        if (bgColorPicker) {
            bgColorPicker.addEventListener('input', function(e) {
                currentBgColor = e.target.value;
                applyReaderSettings();
                saveReaderSettings();
            });
        }

        // Reset button handler
        const resetBtn = document.getElementById('reset-btn');
        if (resetBtn) {
            resetBtn.addEventListener('click', function() {
                if (confirm('Are you sure you want to reset all reading customizations?')) {
                    localStorage.removeItem('novelReaderSettings');
                    // Reset variables to defaults
                    currentFontSize = 18;
                    currentTextColor = null;
                    currentBgColor = null;
                    currentTextAlign = 'justify';
                    currentMargin = 20;
                    applyReaderSettings();
                }
            });
        }

        // Text alignment handlers
        document.querySelectorAll('.text-align-btn').forEach(btn => {
            btn.addEventListener('click', function() {
                currentTextAlign = this.getAttribute('data-align');
                applyReaderSettings();
                saveReaderSettings();
            });
        });

        // Margin button handlers
        document.querySelectorAll('.margin-btn').forEach(btn => {
            btn.addEventListener('click', function() {
                const action = this.getAttribute('data-action');
                if (action === 'increase') {
                    increaseMargin();
                } else if (action === 'decrease') {
                    decreaseMargin();
                }
            });
        });

        // Smooth scrolling for TOC links
        const tocLinks = document.querySelectorAll('.toc-content a');
        tocLinks.forEach(link => {
            link.addEventListener('click', function(e) {
                e.preventDefault();
                const targetId = this.getAttribute('href').substring(1);
                const target = document.getElementById(targetId);
                if (target) {
                    target.scrollIntoView({ behavior: 'smooth', block: 'start' });
                }
                // Close the modal after clicking a TOC link
                if (typeof UIkit !== 'undefined' && UIkit.modal) {
                    UIkit.modal('#toc-modal').hide();
                }
            });
        });
    }

    document.addEventListener('DOMContentLoaded', function() {
        console.log('[Reader] DOMContentLoaded fired');
        initializeReaders();
    });

    // Also initialize after HTMX content swaps
    document.addEventListener('htmx:afterSwap', function() {
        console.log('[Reader] htmx:afterSwap fired');
        initializeReaders();
    });

})();