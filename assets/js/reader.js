/**
 * Reader Module - Handles reading modes for manga and light novels
 */
(function() {
    'use strict';

    const STORAGE_KEY = 'magi-reading-mode';
    const FOCUS_STATE_KEY = 'magi-focus-state';
    const NOVEL_SETTINGS_KEY = 'lightNovelReaderSettings';
    const MODES = { WEBTOON: 'webtoon', SINGLE: 'single', SIDE_BY_SIDE: 'side-by-side' };

    // Lazy loading setup
    const lazyLoadObserver = new IntersectionObserver((entries, observer) => {
        entries.forEach(entry => {
            if (entry.isIntersecting) {
                const img = entry.target;
                img.src = img.dataset.src;
                img.classList.remove('lazy');
                observer.unobserve(img);
            }
        });
    }, { rootMargin: '50px' }); // Load 50px before entering viewport

    const observeLazyImages = () => {
        document.querySelectorAll('img.lazy').forEach(img => {
            lazyLoadObserver.observe(img);
        });
    };

    // State
    let currentMode = MODES.WEBTOON;
    let currentPage = 0;
    let images = [];
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

    const handleFocusModalImageClick = (e) => {
        e.stopPropagation();
        e.preventDefault();
        closeFocusModal();
    };

    const openFocusModal = (clickedImg) => {
        const modal = getFocusModal();
        const scrollContainer = modal.querySelector('.webtoon-focus-modal-scroll');

        const savedState = getSavedFocusState();
        const shouldRestoreScroll = savedState && clickedImg === undefined;

        scrollPositionBeforeFocus = document.querySelector('main.site-main')?.scrollTop ?? document.documentElement.scrollTop;

        const navbar = document.querySelector('nav') || document.querySelector('.navbar') || document.querySelector('.top-navbar');
        const navbarHeight = navbar ? navbar.offsetHeight : 0;
        const effectiveScrollPosition = scrollPositionBeforeFocus + navbarHeight;

        let clickedIndex = -1;
        const mainImages = containerElement.querySelectorAll('.reader-image');
        mainImages.forEach((img, index) => { if (img === clickedImg) clickedIndex = index; });
        if (clickedIndex === -1 && clickedImg && clickedImg.src) clickedIndex = Array.from(mainImages).findIndex(img => img.src === clickedImg.src);
        if (clickedIndex === -1) clickedIndex = 0;

        const mainImageWidth = clickedImg.offsetWidth;
        const mainImageHeight = clickedImg.offsetHeight;

        let imageTopPosition = 0;
        for (let i = 0; i < clickedIndex; i++) imageTopPosition += mainImages[i].offsetHeight;

        const containerStyles = window.getComputedStyle(containerElement);
        const containerPaddingTop = parseFloat(containerStyles.paddingTop) || 0;
        const scrollOffsetFromImageStart = effectiveScrollPosition - (imageTopPosition + containerElement.offsetTop + containerPaddingTop);
        const naturalAspectRatio = clickedImg.naturalWidth / clickedImg.naturalHeight;

        focusStateData = { imageIndex: clickedIndex, mainImageWidth, mainImageHeight, viewportScrollOffset: scrollOffsetFromImageStart, naturalAspectRatio };

        scrollContainer.innerHTML = '';
        if (currentMode === MODES.WEBTOON) {
            originalImageParents = [];
            mainImages.forEach((img, index) => {
                originalImageParents.push(img.parentNode);
                scrollContainer.appendChild(img);
                img.addEventListener('click', handleFocusModalImageClick);
                // Load lazy images immediately when opening focus modal
                if (img.classList.contains('lazy')) {
                    img.src = img.dataset.src;
                    img.classList.remove('lazy');
                    lazyLoadObserver.unobserve(img);
                }
                // Remove the open focus listener while in modal
                if (img.openFocusListener) {
                    img.removeEventListener('click', img.openFocusListener);
                }
            });
        } else {
            // For single/double, create enlarged copies of current page
            if (currentMode === MODES.SINGLE) {
                const currentSrc = images[currentPage];
                if (currentSrc) {
                    const img = document.createElement('img');
                    img.src = currentSrc;
                    img.className = 'reader-image';
                    img.style.maxWidth = '100%';
                    img.style.height = 'auto';
                    scrollContainer.appendChild(img);
                    img.addEventListener('click', handleFocusModalImageClick);
                }
            } else if (currentMode === MODES.SIDE_BY_SIDE) {
                const leftSrc = images[currentPage * 2];
                const rightSrc = images[currentPage * 2 + 1];
                if (leftSrc) {
                    const img = document.createElement('img');
                    img.src = leftSrc;
                    img.className = 'reader-image';
                    img.style.setProperty('width', '50%', 'important');
                    img.style.setProperty('height', 'auto', 'important');
                    scrollContainer.appendChild(img);
                    img.addEventListener('click', handleFocusModalImageClick);
                }
                if (rightSrc) {
                    const img = document.createElement('img');
                    img.src = rightSrc;
                    img.className = 'reader-image';
                    img.style.setProperty('width', '50%', 'important');
                    img.style.setProperty('height', 'auto', 'important');
                    scrollContainer.appendChild(img);
                    img.addEventListener('click', handleFocusModalImageClick);
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

        const focusImages = scrollContainer.querySelectorAll('.reader-image');
        if (focusImages[clickedIndex]) {
            const focusImage = focusImages[clickedIndex];
            let targetScrollTop = 0;
            for (let i = 0; i < clickedIndex; i++) targetScrollTop += focusImages[i].offsetHeight;
            const heightRatio = focusImage.offsetHeight / mainImageHeight;
            targetScrollTop += scrollOffsetFromImageStart * heightRatio;
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

        if (currentMode === MODES.WEBTOON) {
            const focusImages = scrollContainer.querySelectorAll('.reader-image');

        // Store modal heights before moving images back
        const modalHeights = Array.from(focusImages).map(img => img.offsetHeight);

        // Calculate the current position in the modal
        let currentImageIndex = 0;
        let cumulativeModalHeight = 0;
        for (let i = 0; i < focusImages.length; i++) {
            const imgHeight = modalHeights[i];
            if (cumulativeModalHeight + imgHeight > currentModalScrollTop) {
                currentImageIndex = i;
                break;
            }
            cumulativeModalHeight += imgHeight;
        }
        const offsetInModal = currentModalScrollTop - cumulativeModalHeight;

        // Move images back
        focusImages.forEach((img, index) => {
            if (originalImageParents[index]) {
                originalImageParents[index].appendChild(img);
                img.removeEventListener('click', handleFocusModalImageClick);
                // Restore the open focus listener
                if (img.openFocusListener) {
                    img.addEventListener('click', img.openFocusListener);
                }
            }
        });

        // Restore scroll position with proper mapping
        requestAnimationFrame(() => {
            const mainElement = document.querySelector('main.site-main');
            const mainImages = containerElement.querySelectorAll('.reader-image');
            const containerStyles = window.getComputedStyle(containerElement);
            const containerPaddingTop = parseFloat(containerStyles.paddingTop) || 0;

            let mainScrollTop = containerElement.offsetTop + containerPaddingTop;
            for (let i = 0; i < currentImageIndex; i++) {
                mainScrollTop += mainImages[i].offsetHeight;
            }

            const heightRatio = (currentImageIndex < mainImages.length && modalHeights[currentImageIndex]) ?
                mainImages[currentImageIndex].offsetHeight / modalHeights[currentImageIndex] : 1;
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

        const focusImages = scrollContainer.querySelectorAll('.reader-image');
        focusImages.forEach(img => {
            img.style.transform = `scale(${focusModalZoom})`;
            img.style.transformOrigin = 'top center';
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
        const allImgs = containerElement.querySelectorAll('.reader-image');
        const wrappers = containerElement.querySelectorAll('.webtoon-image-wrapper');
        allImgs.forEach(img => {
            img.style.display = 'none';
            img.style.maxWidth = '';
            img.style.height = '';
            img.style.width = '';
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
            allImgs.forEach(img => {
                img.style.display = 'block';
                img.style.maxWidth = '';
                img.style.height = 'auto';
                img.style.width = '';
            });
        } else if (currentMode === MODES.SINGLE) {
            containerElement.className = 'reader-single-page-container';
            containerElement.style.display = '';
            containerElement.style.justifyContent = '';
            containerElement.style.alignItems = '';
            containerElement.style.gap = '';
            containerElement.style.maxWidth = '';
            allImgs.forEach((img, index) => {
                if (index === currentPage) {
                    img.style.display = 'block';
                    img.style.maxWidth = '100%';
                    img.style.height = 'auto';
                    img.style.width = '';
                }
            });
        } else if (currentMode === MODES.SIDE_BY_SIDE) {
            containerElement.className = '';
            containerElement.style.display = 'flex';
            containerElement.style.justifyContent = 'space-between';
            containerElement.style.alignItems = 'flex-start';
            containerElement.style.gap = '0';
            containerElement.style.maxWidth = '';
            allImgs.forEach((img, index) => {
                if (index === currentPage * 2 || index === currentPage * 2 + 1) {
                    img.style.display = 'block';
                    img.style.maxWidth = '50%';
                    img.style.height = 'auto';
                    img.style.width = '';
                }
            });
        }
        attachImageListeners();
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
                const focusImages = scrollContainer.querySelectorAll('.reader-image');
                let targetScrollTop = 0;
                let targetIndex = currentPage; // For webtoon, currentPage is 0
                for (let i = 0; i < targetIndex && i < focusImages.length; i++) {
                    targetScrollTop += focusImages[i].offsetHeight;
                }
                scrollContainer.scrollTo({ top: targetScrollTop, behavior: 'smooth' });
            } else {
                // Update modal content for single/double
                const scrollContainer = focusModal.querySelector('.webtoon-focus-modal-scroll');
                scrollContainer.innerHTML = '';
                if (currentMode === MODES.SINGLE) {
                    const currentSrc = images[currentPage];
                    if (currentSrc) {
                        const img = document.createElement('img');
                        img.src = currentSrc;
                        img.className = 'reader-image';
                        img.style.maxWidth = '100%';
                        img.style.height = 'auto';
                        scrollContainer.appendChild(img);
                        img.addEventListener('click', handleFocusModalImageClick);
                    }
                } else if (currentMode === MODES.SIDE_BY_SIDE) {
                    const leftSrc = images[currentPage * 2];
                    const rightSrc = images[currentPage * 2 + 1];
                    if (leftSrc) {
                        const img = document.createElement('img');
                        img.src = leftSrc;
                        img.className = 'reader-image';
                        img.style.setProperty('width', '50%', 'important');
                        img.style.setProperty('height', 'auto', 'important');
                        scrollContainer.appendChild(img);
                        img.addEventListener('click', handleFocusModalImageClick);
                    }
                    if (rightSrc) {
                        const img = document.createElement('img');
                        img.src = rightSrc;
                        img.className = 'reader-image';
                        img.style.setProperty('width', '50%', 'important');
                        img.style.setProperty('height', 'auto', 'important');
                        scrollContainer.appendChild(img);
                        img.addEventListener('click', handleFocusModalImageClick);
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

    const attachImageListeners = () => {
        containerElement.querySelectorAll('.reader-image').forEach(img => {
            if (!img.openFocusListener) {
                img.openFocusListener = () => openFocusModal(img);
                img.addEventListener('click', img.openFocusListener);
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

        images = Array.from(containerElement.querySelectorAll('.reader-image')).map(img => img.dataset.src);
        currentPage = 0;
        observeLazyImages();
        updateModeVisibility();
        updateModeButtons();

        // Event listeners
        document.addEventListener('keydown', (e) => {
            if (document.activeElement.tagName === 'INPUT' || document.activeElement.tagName === 'TEXTAREA') return;
            switch (e.key) {
                case 'ArrowRight': case ' ': nextPage(); break;
                case 'ArrowLeft': prevPage(); break;
                case 'f': case 'F': openFocusModal(); break;
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
