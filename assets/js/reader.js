/**
 * Reader Module - Handles reading modes for manga and light novels
 */
(function() {
    'use strict';

    const STORAGE_KEY = 'magi-reading-mode';
    const FOCUS_STATE_KEY = 'magi-focus-state';
    const NOVEL_SETTINGS_KEY = 'lightNovelReaderSettings';
    const MODES = { WEBTOON: 'webtoon', SINGLE: 'single', SIDE_BY_SIDE: 'side-by-side' };

    // Lazy loading setup - check scroll position to load more canvases
    let scrollCheckTimeout = null;
    
    const checkScrollForLazyLoading = () => {
        if (lazyTokens.length === 0) return;
        
        const canvases = containerElement.querySelectorAll('.reader-canvas');
        if (canvases.length === 0) return;
        
        const lastCanvas = canvases[canvases.length - 1];
        const rect = lastCanvas.getBoundingClientRect();
        const viewportHeight = window.innerHeight;
        
        // Load next canvas if top of last canvas is within 0.5 viewport heights of viewport bottom
        if (rect.top <= viewportHeight + viewportHeight * 0.5) {
            loadNextCanvas();
        }
    };

    const observeLazyCanvases = () => {
        const scrollElement = document.querySelector('main.site-main') || window;
        // Remove previous scroll listener if exists
        scrollElement.removeEventListener('scroll', handleScroll);
        // Add scroll listener
        scrollElement.addEventListener('scroll', handleScroll, { passive: true });
    };

    const handleScroll = () => {
        checkScrollForLazyLoading();
    };

    const loadInitialCanvases = async () => {
        const initialTokens = images.slice(0, 2);
        lazyTokens = images.slice(2);
        currentLoadedIndex = 1;

        // Load first 2 canvases
        for (let i = 0; i < initialTokens.length; i++) {
            const token = initialTokens[i];
            const canvas = document.createElement('canvas');
            canvas.className = 'reader-canvas';
            canvas.width = 100; // placeholder
            canvas.height = 100; // placeholder
            canvas.style.width = '100%';
            canvas.style.height = 'auto';
            canvas.style.display = 'block';
            containerElement.appendChild(canvas);
            await loadImageToCanvas(canvas, `/api/image?token=${token}`);
        }

        // Now start observing for lazy loading
        updateModeVisibility();
        observeLazyCanvases();
    };

    const loadImageToCanvas = async (canvas, url) => {
        try {
            const response = await fetch(url);
            if (!response.ok) {
                throw new Error(`Failed to fetch image: ${response.status} ${response.statusText}`);
            }
            const blob = await response.blob();
            const bitmap = await createImageBitmap(blob);
            
            // Set canvas dimensions to bitmap size
            canvas.width = bitmap.width;
            canvas.height = bitmap.height;
            const ctx = canvas.getContext('2d');
            ctx.drawImage(bitmap, 0, 0);
            
            // Update canvas style to maintain aspect ratio and respect container width
            canvas.style.width = '100%';
            canvas.style.height = 'auto';
            canvas.style.maxWidth = '100%';
            canvas.style.objectFit = 'contain';
        } catch (error) {
            console.error('Failed to load image to canvas:', error);
        }
    };

    const loadNextCanvas = async () => {
        if (lazyTokens.length === 0) return;
        
        const token = lazyTokens.shift(); // Take the next token
        const canvas = document.createElement('canvas');
        canvas.className = 'reader-canvas';
        canvas.width = 100; // placeholder
        canvas.height = 100; // placeholder
        canvas.style.width = '100%';
        canvas.style.height = 'auto';
        canvas.style.display = 'block';
        containerElement.appendChild(canvas);
        await loadImageToCanvas(canvas, `/api/image?token=${token}`);
        currentLoadedIndex++;
    };

    // State
    let currentMode = MODES.WEBTOON;
    let currentPage = 0;
    let images = [];
    let lazyTokens = [];
    let currentLoadedIndex = -1;
    let containerElement = null;
    let focusModal = null;
    let scrollPositionBeforeFocus = 0;
    let focusModalZoom = 1;
    let focusStateData = {};
    let isLightNovel = false;
    let originalImageParents = [];

    // Light novel settings
    const novelSettings = {
        fontSize: 18,
        textColor: null,
        bgColor: null,
        textAlign: 'justify',
        margin: 20
    };
    const novelLimits = {
        fontSize: { min: 12, max: 32, step: 2 },
        margin: { min: 0, max: 50, step: 5 }
    };

    // Utility functions
    const saveToStorage = (key, data) => {
        try { localStorage.setItem(key, JSON.stringify(data)); } catch (e) { console.warn(`Failed to save ${key}:`, e); }
    };
    const loadFromStorage = (key) => {
        try { return JSON.parse(localStorage.getItem(key)); } catch (e) { console.warn(`Failed to load ${key}:`, e); return null; }
    };
    const removeFromStorage = (key) => {
        try { localStorage.removeItem(key); } catch (e) { console.warn(`Failed to remove ${key}:`, e); }
    };

    // Mobile detection
    const isMobile = () => window.innerWidth <= 768;

    const updateContainerWidth = () => {
        const ukContainer = containerElement.closest('.uk-container');
        const mainContent = containerElement.closest('main');
        if (containerElement) {
            if (isMobile()) {
                containerElement.style.width = '100vw';
                containerElement.style.position = 'relative';
                containerElement.style.left = '50%';
                containerElement.style.transform = 'translateX(-50%)';
                containerElement.style.maxWidth = 'none';
            } else {
                containerElement.style.width = '';
                containerElement.style.position = '';
                containerElement.style.left = '';
                containerElement.style.transform = '';
                containerElement.style.maxWidth = '';
            }
        }
    };

    // Light novel functions
    const loadNovelSettings = () => {
        const saved = loadFromStorage(NOVEL_SETTINGS_KEY);
        if (saved) Object.assign(novelSettings, saved);
    };

    const saveNovelSettings = () => saveToStorage(NOVEL_SETTINGS_KEY, novelSettings);

    const applyNovelSettings = () => {
        const reader = document.querySelector('.epub-reader');
        if (!reader) return;

        const card = reader.closest('.uk-card');

        reader.style.fontSize = novelSettings.fontSize + 'px';
        const fontSizeDisplay = document.getElementById('current-font-size');
        if (fontSizeDisplay) fontSizeDisplay.textContent = novelSettings.fontSize + 'px';

        if (novelSettings.textColor) {
            reader.style.color = novelSettings.textColor;
            reader.querySelectorAll('p, h1, h2, h3, h4, h5, h6, span, div, a').forEach(el => el.style.color = novelSettings.textColor);
        } else {
            reader.style.color = '';
            reader.querySelectorAll('p, h1, h2, h3, h4, h5, h6, span, div, a').forEach(el => el.style.color = '');
        }

        if (card) {
            card.style.backgroundColor = novelSettings.bgColor || '';
            if (novelSettings.bgColor) reader.setAttribute('data-bg-color', novelSettings.bgColor);
            else reader.removeAttribute('data-bg-color');
        }

        reader.style.textAlign = novelSettings.textAlign;
        document.documentElement.style.setProperty('--reader-margin', novelSettings.margin + 'px');

        const colorPicker = document.getElementById('font-color-picker');
        if (colorPicker) colorPicker.value = novelSettings.textColor || '#000000';
        const bgColorPicker = document.getElementById('bg-color-picker');
        if (bgColorPicker) bgColorPicker.value = novelSettings.bgColor || '#ffffff';

        document.querySelectorAll('.text-align-btn').forEach(btn => {
            btn.classList.toggle('uk-btn-primary', btn.getAttribute('data-align') === novelSettings.textAlign);
            btn.classList.toggle('uk-btn-default', btn.getAttribute('data-align') !== novelSettings.textAlign);
        });

        const marginDisplay = document.getElementById('current-margin');
        if (marginDisplay) marginDisplay.textContent = novelSettings.margin + 'px';
    };

    const adjustSetting = (key, direction) => {
        const limit = novelLimits[key];
        if (!limit) return;
        const newValue = novelSettings[key] + (direction * limit.step);
        if (newValue >= limit.min && newValue <= limit.max) {
            novelSettings[key] = newValue;
            applyNovelSettings();
            saveNovelSettings();
        }
    };

    const resetNovelSettings = () => {
        Object.assign(novelSettings, { fontSize: 18, textColor: null, bgColor: null, textAlign: 'justify', margin: 20 });
        applyNovelSettings();
        removeFromStorage(NOVEL_SETTINGS_KEY);
    };

    // Focus modal functions
    const createFocusModal = () => {
        const modal = document.createElement('div');
        modal.className = 'webtoon-focus-modal';
        modal.id = 'webtoon-focus-modal';
        modal.style.position = 'fixed';
        modal.style.top = '0';
        modal.style.left = '0';
        modal.style.width = '100%';
        modal.style.height = '100%';
        modal.style.zIndex = '9999';
        modal.style.display = 'none';

        const overlay = document.createElement('div');
        overlay.className = 'webtoon-focus-modal-overlay';
        overlay.style.position = 'absolute';
        overlay.style.top = '0';
        overlay.style.left = '0';
        overlay.style.width = '100%';
        overlay.style.height = '100%';
        overlay.style.backgroundColor = 'rgba(0,0,0,0.8)';
        overlay.style.zIndex = '1';
        overlay.addEventListener('click', closeFocusModal);

        const scrollContainer = document.createElement('div');
        scrollContainer.className = 'webtoon-focus-modal-scroll';
        scrollContainer.id = 'webtoon-focus-modal-scroll';
        scrollContainer.style.position = 'absolute';
        scrollContainer.style.top = '0';
        scrollContainer.style.left = '0';
        scrollContainer.style.width = '100%';
        scrollContainer.style.height = '100%';
        scrollContainer.style.overflow = 'auto';
        scrollContainer.style.zIndex = '2';
        scrollContainer.addEventListener('click', (e) => { if (e.target === scrollContainer) closeFocusModal(); });

        const closeBtn = document.createElement('button');
        closeBtn.className = 'webtoon-focus-modal-close';
        closeBtn.innerHTML = '&times;';
        closeBtn.style.position = 'absolute';
        closeBtn.style.top = '10px';
        closeBtn.style.right = '10px';
        closeBtn.style.zIndex = '10000';
        closeBtn.style.background = 'rgba(0,0,0,0.5)';
        closeBtn.style.color = 'white';
        closeBtn.style.border = 'none';
        closeBtn.style.padding = '10px';
        closeBtn.style.cursor = 'pointer';
        closeBtn.setAttribute('aria-label', 'Close');
        closeBtn.addEventListener('click', closeFocusModal);

        modal.append(overlay, scrollContainer, closeBtn);
        document.body.appendChild(modal);
        return modal;
    };    const getFocusModal = () => focusModal || (focusModal = createFocusModal());

    const saveFocusState = (modalScrollTop = 0) => {
        const state = { ...focusStateData, modalScrollTop, mainPageScrollTop: scrollPositionBeforeFocus, timestamp: Date.now() };
        saveToStorage(FOCUS_STATE_KEY, state);
    };

    const getSavedFocusState = () => loadFromStorage(FOCUS_STATE_KEY);

    const clearFocusState = () => removeFromStorage(FOCUS_STATE_KEY);

    const handleFocusModalCanvasClick = (e) => {
        e.stopPropagation();
        e.preventDefault();
        closeFocusModal();
    };

    const openFocusModal = (clickedCanvas) => {
        const modal = getFocusModal();
        const scrollContainer = modal.querySelector('.webtoon-focus-modal-scroll');

        const savedState = getSavedFocusState();
        const shouldRestoreScroll = savedState && clickedCanvas === undefined;

        scrollPositionBeforeFocus = document.querySelector('main.site-main')?.scrollTop ?? document.documentElement.scrollTop;

        const navbar = document.querySelector('nav') || document.querySelector('.navbar') || document.querySelector('.top-navbar');
        const navbarHeight = navbar ? navbar.offsetHeight : 0;
        const effectiveScrollPosition = scrollPositionBeforeFocus + navbarHeight;

        let clickedIndex = -1;
        const mainCanvases = containerElement.querySelectorAll('.reader-canvas');
        mainCanvases.forEach((canvas, index) => { if (canvas === clickedCanvas) clickedIndex = index; });
        if (clickedIndex === -1) clickedIndex = 0;

        const mainCanvasWidth = clickedCanvas.offsetWidth;
        const mainCanvasHeight = clickedCanvas.offsetHeight;

        let canvasTopPosition = 0;
        for (let i = 0; i < clickedIndex; i++) canvasTopPosition += mainCanvases[i].offsetHeight;

        const containerStyles = window.getComputedStyle(containerElement);
        const containerPaddingTop = parseFloat(containerStyles.paddingTop) || 0;
        const scrollOffsetFromCanvasStart = effectiveScrollPosition - (canvasTopPosition + containerElement.offsetTop + containerPaddingTop);
        const naturalAspectRatio = clickedCanvas.width / clickedCanvas.height;

        focusStateData = { imageIndex: clickedIndex, mainImageWidth: mainCanvasWidth, mainImageHeight: mainCanvasHeight, viewportScrollOffset: scrollOffsetFromCanvasStart, naturalAspectRatio };

        scrollContainer.innerHTML = '';
        if (currentMode === MODES.WEBTOON) {
            originalImageParents = [];
            mainCanvases.forEach((canvas, index) => {
                originalImageParents.push(canvas.parentNode);
                scrollContainer.appendChild(canvas);
                canvas.addEventListener('click', handleFocusModalCanvasClick);
                // Remove the open focus listener while in modal
                if (canvas.openFocusListener) {
                    canvas.removeEventListener('click', canvas.openFocusListener);
                }
            });
            // Set up lazy loading in focus modal
            let modalLazyTokens = [...lazyTokens];
            const checkScrollForLazyLoadingModal = () => {
                if (modalLazyTokens.length === 0) return;
                const canvases = scrollContainer.querySelectorAll('.reader-canvas');
                if (canvases.length === 0) return;
                const lastCanvas = canvases[canvases.length - 1];
                const scrollTop = scrollContainer.scrollTop;
                const modalHeight = scrollContainer.clientHeight;
                const lastCanvasTop = lastCanvas.offsetTop;
                // Load next if last canvas top is within 0.5 modal heights from bottom
                if (lastCanvasTop - scrollTop <= modalHeight + modalHeight * 0.5) {
                    const token = modalLazyTokens.shift();
                    const canvas = document.createElement('canvas');
                    canvas.className = 'reader-canvas';
                    canvas.width = 100;
                    canvas.height = 100;
                    canvas.style.width = '100%';
                    canvas.style.height = 'auto';
                    canvas.style.display = 'block';
                    scrollContainer.appendChild(canvas);
                    loadImageToCanvas(canvas, `/api/image?token=${token}`);
                    canvas.addEventListener('click', handleFocusModalCanvasClick);
                }
            };
            const handleModalScroll = () => checkScrollForLazyLoadingModal();
            scrollContainer.addEventListener('scroll', handleModalScroll);
            // Store to remove later
            scrollContainer.modalScrollHandler = handleModalScroll;
        } else {
            // For single/double, create enlarged copies of current page
            if (currentMode === MODES.SINGLE) {
                const currentSrc = images[currentPage];
                if (currentSrc) {
                    const canvas = document.createElement('canvas');
                    canvas.className = 'reader-canvas';
                    canvas.style.maxWidth = '100%';
                    canvas.style.height = 'auto';
                    scrollContainer.appendChild(canvas);
                    loadImageToCanvas(canvas, currentSrc);
                    canvas.addEventListener('click', handleFocusModalCanvasClick);
                }
            } else if (currentMode === MODES.SIDE_BY_SIDE) {
                const leftSrc = images[currentPage * 2];
                const rightSrc = images[currentPage * 2 + 1];
                if (leftSrc) {
                    const canvas = document.createElement('canvas');
                    canvas.className = 'reader-canvas';
                    canvas.style.setProperty('width', '50%', 'important');
                    canvas.style.setProperty('height', 'auto', 'important');
                    scrollContainer.appendChild(canvas);
                    loadImageToCanvas(canvas, leftSrc);
                    canvas.addEventListener('click', handleFocusModalCanvasClick);
                }
                if (rightSrc) {
                    const canvas = document.createElement('canvas');
                    canvas.className = 'reader-canvas';
                    canvas.style.setProperty('width', '50%', 'important');
                    canvas.style.setProperty('height', 'auto', 'important');
                    scrollContainer.appendChild(canvas);
                    loadImageToCanvas(canvas, rightSrc);
                    canvas.addEventListener('click', handleFocusModalCanvasClick);
                }
                // Set container to flex for side by side
                scrollContainer.style.setProperty('display', 'flex', 'important');
                scrollContainer.style.setProperty('flex-direction', 'row', 'important');
                scrollContainer.style.setProperty('justify-content', 'space-between', 'important');
                scrollContainer.style.setProperty('align-items', 'flex-start', 'important');
            }
        }

        modal.classList.add('active');
        modal.style.display = 'flex';
        document.body.classList.add('webtoon-focus-open');

        const focusCanvases = scrollContainer.querySelectorAll('.reader-canvas');
        if (focusCanvases[clickedIndex]) {
            const focusCanvas = focusCanvases[clickedIndex];
            let targetScrollTop = 0;
            for (let i = 0; i < clickedIndex; i++) targetScrollTop += focusCanvases[i].offsetHeight;
            const heightRatio = focusCanvas.offsetHeight / mainCanvasHeight;
            targetScrollTop += scrollOffsetFromCanvasStart * heightRatio;
            scrollContainer.scrollTop = targetScrollTop;
        }

        if (shouldRestoreScroll && savedState.modalScrollTop !== undefined) {
            scrollContainer.scrollTop = savedState.modalScrollTop;
        }

        focusModalZoom = 1;
        updateFocusModalZoom();

        // Bring pagination to top in focus mode for single/double modes
        if (currentMode !== MODES.WEBTOON) {
            const pagination = document.getElementById('reader-pagination-bottom');
            if (pagination) {
                pagination.style.setProperty('z-index', '10002', 'important');
                pagination.style.setProperty('position', 'fixed', 'important');
                pagination.style.setProperty('bottom', '10px', 'important');
                pagination.style.setProperty('top', 'auto', 'important');
                pagination.style.setProperty('left', '50%', 'important');
                pagination.style.setProperty('display', 'flex', 'important');
                pagination.stopPropagationListener = (e) => e.stopPropagation();
                pagination.addEventListener('click', pagination.stopPropagationListener);
            }
        }

        scrollContainer.addEventListener('scroll', () => saveFocusState(scrollContainer.scrollTop));
    };

    const closeFocusModal = () => {
        if (!focusModal) return;

        const scrollContainer = focusModal.querySelector('.webtoon-focus-modal-scroll');
        const currentModalScrollTop = scrollContainer.scrollTop;
        saveFocusState(currentModalScrollTop);

        // Remove modal lazy loading scroll handler
        if (scrollContainer.modalScrollHandler) {
            scrollContainer.removeEventListener('scroll', scrollContainer.modalScrollHandler);
            delete scrollContainer.modalScrollHandler;
        }

        if (currentMode === MODES.WEBTOON) {
            const focusCanvases = scrollContainer.querySelectorAll('.reader-canvas');

        // Store modal heights before moving canvases back
        const modalHeights = Array.from(focusCanvases).map(canvas => canvas.offsetHeight);

        // Calculate the current position in the modal
        let currentCanvasIndex = 0;
        let cumulativeModalHeight = 0;
        for (let i = 0; i < focusCanvases.length; i++) {
            const canvasHeight = modalHeights[i];
            if (cumulativeModalHeight + canvasHeight > currentModalScrollTop) {
                currentCanvasIndex = i;
                break;
            }
            cumulativeModalHeight += canvasHeight;
        }
        const offsetInModal = currentModalScrollTop - cumulativeModalHeight;

        // Move canvases back
        focusCanvases.forEach((canvas, index) => {
            const parent = originalImageParents[index] || containerElement;
            parent.appendChild(canvas);
            canvas.removeEventListener('click', handleFocusModalCanvasClick);
            // Restore the open focus listener
            if (canvas.openFocusListener) {
                canvas.addEventListener('click', canvas.openFocusListener);
            }
        });

        // Restore scroll position with proper mapping
        requestAnimationFrame(() => {
            const mainElement = document.querySelector('main.site-main');
            const mainCanvases = containerElement.querySelectorAll('.reader-canvas');
            const containerStyles = window.getComputedStyle(containerElement);
            const containerPaddingTop = parseFloat(containerStyles.paddingTop) || 0;

            let mainScrollTop = containerElement.offsetTop + containerPaddingTop;
            for (let i = 0; i < currentCanvasIndex; i++) {
                mainScrollTop += mainCanvases[i].offsetHeight;
            }

            const heightRatio = (currentCanvasIndex < mainCanvases.length && modalHeights[currentCanvasIndex]) ?
                mainCanvases[currentCanvasIndex].offsetHeight / modalHeights[currentCanvasIndex] : 1;
            mainScrollTop += offsetInModal * heightRatio;

            // Adjust for navbar height to match perceived position
            const navbar = document.querySelector('nav') || document.querySelector('.navbar') || document.querySelector('.top-navbar');
            const navbarHeight = navbar ? navbar.offsetHeight : 0;
            mainScrollTop = Math.max(0, mainScrollTop - navbarHeight);

            if (mainElement) {
                mainElement.scrollTop = mainScrollTop;
            } else {
                document.documentElement.scrollTop = mainScrollTop;
            }

            // Reset pagination styles
            const pagination = document.getElementById('reader-pagination-bottom');
            if (pagination) {
                if (pagination.stopPropagationListener) {
                    pagination.removeEventListener('click', pagination.stopPropagationListener);
                    delete pagination.stopPropagationListener;
                }
                pagination.style.removeProperty('z-index');
                pagination.style.removeProperty('position');
                pagination.style.removeProperty('bottom');
                pagination.style.removeProperty('top');
                pagination.style.removeProperty('left');
                pagination.style.removeProperty('transform');
                pagination.style.removeProperty('display');
            }

            // Hide the modal
            focusModal.classList.remove('active');
            focusModal.style.display = 'none';
            document.body.classList.remove('webtoon-focus-open');

            // Update visibility after modal is hidden
            updateModeVisibility();
            // Re-observe for lazy loading in main view
            observeLazyCanvases();
        });
        } else {
            // For single/double, just hide the modal
            requestAnimationFrame(() => {
                // Update visibility after modal is hidden
                updateModeVisibility();

                // Hide the modal
                focusModal.classList.remove('active');
                focusModal.style.display = 'none';
                document.body.classList.remove('webtoon-focus-open');

                // Reset pagination styles
                const pagination = document.getElementById('reader-pagination-bottom');
                if (pagination) {
                    if (pagination.stopPropagationListener) {
                        pagination.removeEventListener('click', pagination.stopPropagationListener);
                        delete pagination.stopPropagationListener;
                    }
                    pagination.style.removeProperty('z-index');
                    pagination.style.removeProperty('position');
                    pagination.style.removeProperty('bottom');
                    pagination.style.removeProperty('top');
                    pagination.style.removeProperty('left');
                    pagination.style.removeProperty('transform');
                    pagination.style.removeProperty('display');
                }
            });
        }
    };

    const updateFocusModalZoom = () => {
        const scrollContainer = focusModal ? focusModal.querySelector('.webtoon-focus-modal-scroll') : null;
        if (!scrollContainer) return;

        const focusCanvases = scrollContainer.querySelectorAll('.reader-canvas');
        focusCanvases.forEach(canvas => {
            canvas.style.transform = `scale(${focusModalZoom})`;
            canvas.style.transformOrigin = 'top center';
        });
    };

    const zoomInFocusModal = () => {
        if (focusModalZoom < 3) {
            focusModalZoom += 0.25;
            updateFocusModalZoom();
        }
    };

    const zoomOutFocusModal = () => {
        if (focusModalZoom > 0.5) {
            focusModalZoom -= 0.25;
            updateFocusModalZoom();
        }
    };

    const resetFocusModalZoom = () => {
        focusModalZoom = 1;
        updateFocusModalZoom();
    };

    // Reading mode functions
    const setReadingMode = (mode) => {
        currentMode = mode;
        localStorage.setItem(STORAGE_KEY, mode);
        updateModeVisibility();
        updateModeButtons();
    };

    const updatePageCounter = () => {
        const maxPages = currentMode === MODES.SIDE_BY_SIDE ? Math.ceil(images.length / 2) : images.length;
        const counter = (currentPage + 1) + ' / ' + maxPages;
        const pageCounter = document.getElementById('page-counter');
        if (pageCounter) pageCounter.textContent = counter;
        const pageCounterBottom = document.getElementById('page-counter-bottom');
        if (pageCounterBottom) pageCounterBottom.textContent = counter;
    };

    const updateModeVisibility = () => {
        if (isLightNovel) return;
        const allCanvases = containerElement.querySelectorAll('.reader-canvas');
        const wrappers = containerElement.querySelectorAll('.webtoon-image-wrapper');
        allCanvases.forEach(canvas => {
            canvas.style.display = 'none';
            canvas.style.maxWidth = '';
            canvas.style.height = '';
            canvas.style.width = '';
        });
        wrappers.forEach(wrapper => wrapper.style.display = 'none');
        if (currentMode === MODES.WEBTOON) {
            containerElement.className = 'flex flex-col items-center justify-center p-0 sm:p-4 w-full mx-auto';
            containerElement.style.maxWidth = '1200px';
            containerElement.style.display = '';
            containerElement.style.justifyContent = '';
            containerElement.style.alignItems = '';
            containerElement.style.gap = '';
            wrappers.forEach(wrapper => wrapper.style.display = 'block');
            allCanvases.forEach(canvas => {
                canvas.style.display = 'block';
                canvas.style.maxWidth = '100%';
                canvas.style.height = 'auto';
                canvas.style.width = '100%';
            });
        } else if (currentMode === MODES.SINGLE) {
            containerElement.className = 'reader-single-page-container';
            containerElement.style.display = '';
            containerElement.style.justifyContent = '';
            containerElement.style.alignItems = '';
            containerElement.style.gap = '';
            containerElement.style.maxWidth = '';
            allCanvases.forEach((canvas, index) => {
                if (index === currentPage) {
                    canvas.style.display = 'block';
                    canvas.style.maxWidth = '100%';
                    canvas.style.height = 'auto';
                    canvas.style.width = '';
                }
            });
        } else if (currentMode === MODES.SIDE_BY_SIDE) {
            containerElement.className = '';
            containerElement.style.display = 'flex';
            containerElement.style.justifyContent = 'space-between';
            containerElement.style.alignItems = 'flex-start';
            containerElement.style.gap = '0';
            containerElement.style.maxWidth = '';
            allCanvases.forEach((canvas, index) => {
                if (index === currentPage * 2 || index === currentPage * 2 + 1) {
                    canvas.style.display = 'block';
                    canvas.style.maxWidth = '50%';
                    canvas.style.height = 'auto';
                    canvas.style.width = '';
                }
            });
        }
        // Handle uk-container width on mobile for full-width images
        updateContainerWidth();
        attachCanvasListeners();
        updatePageCounter();
        const pagination = document.getElementById('reader-pagination');
        const paginationBottom = document.getElementById('reader-pagination-bottom');
        if (currentMode === MODES.WEBTOON) {
            if (pagination) pagination.style.display = 'none';
            if (paginationBottom) paginationBottom.style.display = 'none';
        } else {
            if (pagination) pagination.style.display = 'flex';
            if (paginationBottom) paginationBottom.style.display = 'flex';
            centerPaginationToContent();
        }

        // If focus modal is active
        if (focusModal && focusModal.classList.contains('active')) {
            if (currentMode === MODES.WEBTOON) {
                const scrollContainer = focusModal.querySelector('.webtoon-focus-modal-scroll');
                const focusCanvases = scrollContainer.querySelectorAll('.reader-canvas');
                let targetScrollTop = 0;
                let targetIndex = currentPage; // For webtoon, currentPage is 0
                for (let i = 0; i < targetIndex && i < focusCanvases.length; i++) {
                    targetScrollTop += focusCanvases[i].offsetHeight;
                }
                scrollContainer.scrollTo({ top: targetScrollTop, behavior: 'smooth' });
            } else {
                // Update modal content for single/double
                const scrollContainer = focusModal.querySelector('.webtoon-focus-modal-scroll');
                scrollContainer.innerHTML = '';
                if (currentMode === MODES.SINGLE) {
                    const currentSrc = images[currentPage];
                    if (currentSrc) {
                        const canvas = document.createElement('canvas');
                        canvas.className = 'reader-canvas';
                        canvas.style.maxWidth = '100%';
                        canvas.style.height = 'auto';
                        scrollContainer.appendChild(canvas);
                        loadImageToCanvas(canvas, currentSrc);
                        canvas.addEventListener('click', handleFocusModalCanvasClick);
                    }
                } else if (currentMode === MODES.SIDE_BY_SIDE) {
                    const leftSrc = images[currentPage * 2];
                    const rightSrc = images[currentPage * 2 + 1];
                    if (leftSrc) {
                        const canvas = document.createElement('canvas');
                        canvas.className = 'reader-canvas';
                        canvas.style.setProperty('width', '50%', 'important');
                        canvas.style.setProperty('height', 'auto', 'important');
                        scrollContainer.appendChild(canvas);
                        loadImageToCanvas(canvas, leftSrc);
                        canvas.addEventListener('click', handleFocusModalCanvasClick);
                    }
                    if (rightSrc) {
                        const canvas = document.createElement('canvas');
                        canvas.className = 'reader-canvas';
                        canvas.style.setProperty('width', '50%', 'important');
                        canvas.style.setProperty('height', 'auto', 'important');
                        scrollContainer.appendChild(canvas);
                        loadImageToCanvas(canvas, rightSrc);
                        canvas.addEventListener('click', handleFocusModalCanvasClick);
                    }
                    // Set container to flex for side by side
                    scrollContainer.style.setProperty('display', 'flex', 'important');
                    scrollContainer.style.setProperty('flex-direction', 'row', 'important');
                    scrollContainer.style.setProperty('justify-content', 'space-between', 'important');
                    scrollContainer.style.setProperty('align-items', 'flex-start', 'important');
                }
            }
        }
    };

    const updateModeButtons = () => {
        document.querySelectorAll('.reader-mode-btn').forEach(btn => {
            const mode = btn.getAttribute('data-mode');
            btn.classList.toggle('uk-btn-primary', mode === currentMode);
            btn.classList.toggle('uk-btn-default', mode !== currentMode);
        });
    };

    const attachCanvasListeners = () => {
        containerElement.querySelectorAll('.reader-canvas').forEach(canvas => {
            if (!canvas.openFocusListener) {
                canvas.openFocusListener = () => {
                    if (isMobile()) {
                        // On mobile, scroll down slightly instead of opening focus modal
                        const scrollAmount = window.innerHeight * 0.75; // Scroll down by 3/4 viewport height
                        window.scrollBy({ top: scrollAmount, behavior: 'smooth' });
                    } else {
                        openFocusModal(canvas);
                    }
                };
                canvas.addEventListener('click', canvas.openFocusListener);
            }
        });
    };

    const centerPaginationToContent = () => {
        const pagination = document.getElementById('reader-pagination');
        const paginationBottom = document.getElementById('reader-pagination-bottom');
        if (!containerElement || (!pagination && !paginationBottom)) return;

        const containerRect = containerElement.getBoundingClientRect();
        const centerX = containerRect.left + (containerRect.width / 2);

        if (pagination) {
            pagination.style.left = centerX + 'px';
            pagination.style.transform = 'translateX(-50%)';
        }
        if (paginationBottom) {
            paginationBottom.style.left = centerX + 'px';
            paginationBottom.style.transform = 'translateX(-50%) translateY(0)';
        }
    };

    // Navigation functions
    const nextPage = () => {
        if (isLightNovel) return;
        const maxPages = currentMode === MODES.SIDE_BY_SIDE ? Math.ceil(images.length / 2) : images.length;
        if (currentPage < maxPages - 1) {
            currentPage++;
            updateModeVisibility();
        }
    };

    const prevPage = () => {
        if (isLightNovel) return;
        if (currentPage > 0) {
            currentPage--;
            updateModeVisibility();
        }
    };

    const goToPage = (page) => {
        if (isLightNovel) return;
        const maxPages = currentMode === MODES.SIDE_BY_SIDE ? Math.ceil(images.length / 2) : images.length;
        if (page >= 0 && page < maxPages) {
            currentPage = page;
            updateModeVisibility();
        }
    };

    // Initialization
    const init = () => {
        containerElement = document.getElementById('reader-images-container');
        const textContainer = document.getElementById('reader-text-container');
        isLightNovel = !!textContainer;

        if (isLightNovel) {
            loadNovelSettings();
            applyNovelSettings();
            return;
        }

        if (!containerElement) return;

        const savedMode = localStorage.getItem(STORAGE_KEY);
        if (savedMode && Object.values(MODES).includes(savedMode)) currentMode = savedMode;

        // Collect tokens from divs and remove them
        const tokenDivs = Array.from(containerElement.querySelectorAll('.reader-token'));
        images = tokenDivs.map(div => div.dataset.token);
        
        // Remove all token divs
        tokenDivs.forEach(div => div.remove());

        // Load initial 2 canvases asynchronously
        loadInitialCanvases();

        updateModeButtons();

        // Event listeners
        document.addEventListener('keydown', (e) => {
            if (document.activeElement.tagName === 'INPUT' || document.activeElement.tagName === 'TEXTAREA') return;
            switch (e.key) {
                case 'ArrowRight': case ' ': nextPage(); break;
                case 'ArrowLeft': prevPage(); break;
                case 'f': case 'F': if (!isMobile()) openFocusModal(); break;
                case 'Escape': if (focusModal && focusModal.classList.contains('active')) closeFocusModal(); break;
            }
        });

        document.querySelectorAll('.reader-mode-btn').forEach(btn => {
            btn.addEventListener('click', (e) => {
                e.preventDefault();
                setReadingMode(btn.getAttribute('data-mode'));
            });
        });

        // Pagination button event listeners
        const firstPageBtn = document.getElementById('first-page-btn');
        if (firstPageBtn) firstPageBtn.addEventListener('click', () => goToPage(0));
        const prevPageBtn = document.getElementById('prev-page-btn');
        if (prevPageBtn) prevPageBtn.addEventListener('click', prevPage);
        const nextPageBtn = document.getElementById('next-page-btn');
        if (nextPageBtn) nextPageBtn.addEventListener('click', nextPage);
        const lastPageBtn = document.getElementById('last-page-btn');
        if (lastPageBtn) {
            lastPageBtn.addEventListener('click', () => {
                const maxPages = currentMode === MODES.SIDE_BY_SIDE ? Math.ceil(images.length / 2) : images.length;
                goToPage(maxPages - 1);
            });
        }

        // Center pagination on window resize
        window.addEventListener('resize', () => {
            if (currentMode !== MODES.WEBTOON) {
                centerPaginationToContent();
            }
            updateContainerWidth();
        });

        // Light novel controls
        const increaseFontSizeBtn = document.getElementById('increase-font-size');
        if (increaseFontSizeBtn) increaseFontSizeBtn.addEventListener('click', () => adjustSetting('fontSize', 1));
        const decreaseFontSizeBtn = document.getElementById('decrease-font-size');
        if (decreaseFontSizeBtn) decreaseFontSizeBtn.addEventListener('click', () => adjustSetting('fontSize', -1));
        const increaseMarginBtn = document.getElementById('increase-margin');
        if (increaseMarginBtn) increaseMarginBtn.addEventListener('click', () => adjustSetting('margin', 1));
        const decreaseMarginBtn = document.getElementById('decrease-margin');
        if (decreaseMarginBtn) decreaseMarginBtn.addEventListener('click', () => adjustSetting('margin', -1));
        const resetBtn = document.getElementById('reset-light-novel-settings');
        if (resetBtn) resetBtn.addEventListener('click', resetNovelSettings);

        const fontColorPicker = document.getElementById('font-color-picker');
        if (fontColorPicker) fontColorPicker.addEventListener('input', (e) => {
            novelSettings.textColor = e.target.value;
            applyNovelSettings();
            saveNovelSettings();
        });

        const bgColorPicker = document.getElementById('bg-color-picker');
        if (bgColorPicker) bgColorPicker.addEventListener('input', (e) => {
            novelSettings.bgColor = e.target.value;
            applyNovelSettings();
            saveNovelSettings();
        });

        document.querySelectorAll('.text-align-btn').forEach(btn => {
            btn.addEventListener('click', () => {
                novelSettings.textAlign = btn.getAttribute('data-align');
                applyNovelSettings();
                saveNovelSettings();
            });
        });

        // Focus modal controls
        document.addEventListener('keydown', (e) => {
            if (!focusModal || !focusModal.classList.contains('active')) return;
            switch (e.key) {
                case '+': case '=': zoomInFocusModal(); break;
                case '-': zoomOutFocusModal(); break;
                case '0': resetFocusModalZoom(); break;
            }
        });
    };

    // Auto-init on page load
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }

    // Re-init on HTMX content swap
    document.addEventListener('htmx:afterSwap', (event) => {
        if (event.detail.target && event.detail.target.id === 'content' &&
            (document.getElementById('reader-images-container') || document.getElementById('reader-text-container'))) {
            setTimeout(init, 50); // Small delay to ensure DOM is ready
        }
    });

    // Expose functions globally if needed
    window.ReaderModule = { init, setReadingMode, openFocusModal, closeFocusModal };
})();
