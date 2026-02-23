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

    /**
     * JS-driven eased scroll — works regardless of any CSS overflow on parent elements.
     * @param {number} targetY  - absolute page Y position to scroll to
     * @param {number} duration - animation duration in ms
     */
    function smoothScrollTo(targetY, duration) {
        // Cancel any in-progress scroll animation
        if (smoothScrollTo._raf) cancelAnimationFrame(smoothScrollTo._raf);

        const startY = window.scrollY;
        const distance = targetY - startY;
        if (distance === 0) return;

        const startTime = performance.now();

        // Ease in-out cubic
        function easeInOutCubic(t) {
            return t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2;
        }

        function step(now) {
            const elapsed = now - startTime;
            const progress = Math.min(elapsed / duration, 1);
            window.scrollTo(0, startY + distance * easeInOutCubic(progress));
            if (progress < 1) {
                smoothScrollTo._raf = requestAnimationFrame(step);
            }
        }

        smoothScrollTo._raf = requestAnimationFrame(step);
    }

    // Reader class for modularity
    class Reader {
        constructor(containerSelector) {
            this.container = document.querySelector(containerSelector);
            if (!this.container) throw new Error('Reader container not found');

            this.currentMode = MODES.WEBTOON;
            this.currentPage = 0;
            this.images = [];
            this.lazyImages = [];
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
            // Collect image URLs from divs and remove them
            const imageDivs = Array.from(this.container.querySelectorAll('.reader-image'));
            this.images = imageDivs.map(div => div.dataset.url).filter(url => url && url.trim() && url !== 'undefined');
            // Remove all image URL divs
            imageDivs.forEach(div => div.remove());
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
            document.addEventListener('contextmenu', (e) => e.preventDefault());

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
                        if (entry.isIntersecting && this.lazyImages.length > 0 && !this.isInitialLoad) {
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
            const initialImages = this.images.slice(0, 2);
            this.lazyImages = [];
            this.currentLoadedIndex = 0;

            const loadPromises = initialImages.map((url, i) => this.createAndLoadCanvas(url, i));

            Promise.all(loadPromises).then(() => {
                this.updateMode();
                // Set up intersection observer for lazy loading
                const lazyObserver = new IntersectionObserver((entries) => {
                    entries.forEach(entry => {
                        if (entry.isIntersecting) {
                            const canvas = entry.target;
                            const imageIndex = parseInt(canvas.dataset.index);
                            if (imageIndex >= this.currentLoadedIndex && imageIndex < this.images.length) {
                                this.loadImageToCanvas(canvas, this.images[imageIndex]);
                                this.currentLoadedIndex++;
                                lazyObserver.unobserve(canvas);
                            }
                        }
                    });
                }, { rootMargin: '100px' });

                // Store reference to lazyObserver for use in scrollToImage
                this.lazyObserver = lazyObserver;

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
                    this.lazyImages.push(canvas);
                    lazyObserver.observe(canvas);

                    // Also store canvas reference so scrollToImage can find it
                    this.loadedCanvases[i] = canvas;
                }
                this.isInitialLoad = false;
            });
        }

        createAndLoadCanvas(url, index) {
            const canvas = document.createElement('canvas');
            canvas.className = 'reader-canvas';
            canvas.width = DIMENSIONS.PLACEHOLDER_SIZE;
            canvas.height = DIMENSIONS.PLACEHOLDER_SIZE;
            canvas.style.display = 'block';
            canvas.dataset.index = index;
            this.container.appendChild(canvas);
            this.loadedCanvases[index] = canvas;
            return this.loadImageToCanvas(canvas, url);
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
            if (this.lazyImages.length === 0) return;
            const numToLoad = Math.min(2, this.lazyImages.length);
            const loadPromises = [];

            for (let i = 0; i < numToLoad; i++) {
                const imageUrl = this.lazyImages.shift();
                const canvas = document.createElement('canvas');
                canvas.className = 'reader-canvas';
                canvas.width = DIMENSIONS.PLACEHOLDER_SIZE;
                canvas.height = DIMENSIONS.PLACEHOLDER_SIZE;
                canvas.style.height = 'auto';
                canvas.style.display = 'block';
                canvas.dataset.index = this.currentLoadedIndex + i;
                this.container.appendChild(canvas);
                loadPromises.push(this.loadImageToCanvas(canvas, imageUrl));
                this.currentLoadedIndex++;
            }

            Promise.all(loadPromises).then(() => {
                const lastCanvas = this.container.querySelector('.reader-canvas:last-child');
                if (lastCanvas) this.observer.observe(lastCanvas);
            });
        }

        /**
         * Scroll to a specific image by index.
         * Handles progressive loading: if the canvas hasn't been loaded yet,
         * it triggers eager loading of all canvases up to and including the target,
         * then scrolls once the target is ready.
         */
        async scrollToImage(index) {
            // Clamp index to valid range
            index = Math.max(0, Math.min(index, this.images.length - 1));

            const canvas = this.loadedCanvases[index];
            if (!canvas) return;

            // Check if this canvas is still a placeholder (hasn't been loaded yet).
            // Placeholder canvases have width === PLACEHOLDER_SIZE.
            const isPlaceholder = canvas.width === DIMENSIONS.PLACEHOLDER_SIZE && canvas.height === DIMENSIONS.PLACEHOLDER_SIZE;

            if (isPlaceholder) {
                // Eagerly load all canvases from currentLoadedIndex up to target index.
                // We do this by temporarily unobserving placeholders and loading them directly.
                const loadChain = [];
                for (let i = this.currentLoadedIndex; i <= index; i++) {
                    const c = this.loadedCanvases[i];
                    if (c && c.width === DIMENSIONS.PLACEHOLDER_SIZE) {
                        // Stop the lazy observer from handling this one
                        if (this.lazyObserver) this.lazyObserver.unobserve(c);
                        // Remove from lazyImages queue if present
                        const qIdx = this.lazyImages.indexOf(c);
                        if (qIdx !== -1) this.lazyImages.splice(qIdx, 1);
                        loadChain.push(this.loadImageToCanvas(c, this.images[i]));
                        this.currentLoadedIndex = Math.max(this.currentLoadedIndex, i + 1);
                    }
                }
                // Wait for the target canvas to finish loading
                await Promise.all(loadChain);
            }

            // JS-driven eased scroll — bypasses any CSS overflow interference
            smoothScrollTo(canvas.getBoundingClientRect().top + window.scrollY, 600);
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
            const imageUrl = this.images[pageIndex];
            const canvas = document.createElement('canvas');
            canvas.className = 'reader-canvas';
            canvas.width = DIMENSIONS.PLACEHOLDER_SIZE;
            canvas.height = DIMENSIONS.PLACEHOLDER_SIZE;
            canvas.style.display = 'block';
            this.container.appendChild(canvas);
            this.loadedCanvases[pageIndex] = canvas;
            this.loadImageToCanvas(canvas, imageUrl);
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
                    const imageUrl = this.images[this.currentFocusIndex];
                    try {
                        const response = await fetch(imageUrl);
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
                    const imageUrl = this.images[this.currentFocusIndex];
                    try {
                        const response = await fetch(imageUrl);
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
                    const imageUrl = this.images[this.currentFocusIndex + 1];
                    try {
                        const response = await fetch(imageUrl);
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

    // --- Reader Progress Bar (Webtoon mode) ---

    // Inject progress bar enhancement styles once
    function injectProgressBarStyles() {
        const id = 'magi-progress-bar-styles';
        if (document.getElementById(id)) return;
        const style = document.createElement('style');
        style.id = id;
        style.textContent = `
            .progress-square {
                position: relative;
                overflow: hidden;
                transition: transform 0.18s cubic-bezier(0.34, 1.56, 0.64, 1),
                            opacity 0.18s ease,
                            background-color 0.18s ease;
                transform-origin: center center;
                will-change: transform;
            }
            .progress-square:hover {
                transform: scaleY(1.6) scaleX(1.25);
                opacity: 1 !important;
                filter: brightness(1.25);
                z-index: 1;
            }
            .progress-square.active {
                transform: scaleY(1.5) scaleX(1.2);
                z-index: 1;
            }
            .progress-square.active:hover {
                transform: scaleY(1.8) scaleX(1.35);
            }
            .progress-square.active::after {
                content: '';
                position: absolute;
                inset: 0;
                background: linear-gradient(
                    90deg,
                    transparent 0%,
                    rgba(255,255,255,0.55) 50%,
                    transparent 100%
                );
                animation: progress-scan 1.2s ease-in-out infinite;
            }
            @keyframes progress-scan {
                0%   { transform: translateX(-100%); }
                100% { transform: translateX(100%); }
            }
        `;
        document.head.appendChild(style);
    }

    function initReaderProgressBar(images) {
        const progress = document.getElementById('progress');
        if (!progress || !images || images.length === 0) return;

        injectProgressBarStyles();

        // Remove old squares if any
        progress.innerHTML = '';

        for (let i = 0; i < images.length; i++) {
            const sq = document.createElement('div');
            sq.className = 'progress-square';
            sq.title = `Page ${i + 1}`;
            sq.style.cursor = 'pointer';

            // Click handler: delegate to the reader's scrollToImage method.
            // We capture `i` in the closure.
            sq.addEventListener('click', () => {
                const reader = window.reader;
                if (reader && typeof reader.scrollToImage === 'function') {
                    reader.scrollToImage(i);
                } else {
                    // Fallback: find the canvas by data-index and scroll to it directly.
                    const canvas = document.querySelector(
                        `${SELECTORS.CONTAINER} .reader-canvas[data-index="${i}"]`
                    );
                    if (canvas) {
                        smoothScrollTo(canvas.getBoundingClientRect().top + window.scrollY, 600);
                    }
                }
            });

            progress.appendChild(sq);
        }

        const squares = progress.querySelectorAll('.progress-square');
        const visibleRatios = new Array(images.length).fill(0);

        const observer = new window.IntersectionObserver(entries => {
            entries.forEach(entry => {
                const index = parseInt(entry.target.dataset.index, 10);
                if (!isNaN(index)) {
                    visibleRatios[index] = entry.intersectionRatio;
                }
            });

            let maxIndex = 0;
            let maxRatio = 0;
            visibleRatios.forEach((ratio, i) => {
                if (ratio > maxRatio) {
                    maxRatio = ratio;
                    maxIndex = i;
                }
            });

            squares.forEach(s => s.classList.remove('active'));
            if (maxRatio > 0) {
                squares[maxIndex].classList.add('active');
            } else if (squares.length > 0) {
                squares[0].classList.add('active'); // fallback: highlight first
            }
        }, {
            threshold: buildThresholdList()
        });

        function buildThresholdList() {
            let thresholds = [];
            for (let i = 0; i <= 1; i += 0.05) {
                thresholds.push(i);
            }
            return thresholds;
        }

        // Observe all current and future canvases
        const container = document.querySelector('#reader-images-container');

        function observeAllCanvases() {
            const canvases = container.querySelectorAll('.reader-canvas');
            canvases.forEach(canvas => observer.observe(canvas));
        }
        observeAllCanvases();

        // MutationObserver to watch for new canvases
        const mutationObserver = new MutationObserver(mutations => {
            mutations.forEach(mutation => {
                mutation.addedNodes.forEach(node => {
                    if (node.nodeType === 1 && node.classList.contains('reader-canvas')) {
                        observer.observe(node);
                    }
                });
            });
        });
        mutationObserver.observe(container, { childList: true });
    }

    // Wait for canvases to be rendered by JS
    function waitForCanvasesAndInitProgressBar(images) {
        function tryInit() {
            if (document.querySelectorAll('.reader-canvas').length > 0) {
                initReaderProgressBar(images);
            } else {
                setTimeout(tryInit, 100);
            }
        }
        tryInit();
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
        let images = [];
        const container = document.querySelector('#reader-images-container');
        if (container) {
            images = Array.from(container.querySelectorAll('.reader-image')).map(div => div.dataset.url).filter(url => url && url.trim() && url !== 'undefined');
        }
        initializeReaders();
        waitForCanvasesAndInitProgressBar(images);
    });

    // Also initialize after HTMX content swaps
    document.addEventListener('htmx:afterSwap', function() {
        console.log('[Reader] htmx:afterSwap fired');
        let images = [];
        const container = document.querySelector('#reader-images-container');
        if (container) {
            images = Array.from(container.querySelectorAll('.reader-image')).map(div => div.dataset.url).filter(url => url && url.trim() && url !== 'undefined');
        }
        initializeReaders();
        waitForCanvasesAndInitProgressBar(images);
    });

})();