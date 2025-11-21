/**
 * Reader Module - Handles different reading modes for manga chapters
 * Supports: webtoon (vertical scroll), single-page, and side-by-side modes
 */
(function() {
    'use strict';

    const STORAGE_KEY = 'magi-reading-mode';
    const FOCUS_STATE_STORAGE_KEY = 'magi-focus-state';
    const MODES = {
        WEBTOON: 'webtoon',
        SINGLE: 'single',
        SIDE_BY_SIDE: 'side-by-side'
    };

    let currentMode = MODES.WEBTOON;
    let currentPage = 0;
    let images = [];
    let containerElement = null;
    let focusModal = null;
    let focusModalContent = null;
    let scrollPositionBeforeFocus = 0;
    let focusModalZoom = 1;
    let focusModalViewportWidth = 0; // Track the focus modal's viewport width for aspect ratio calculations
    let focusStateData = {
        imageIndex: 0,
        mainImageWidth: 0,
        mainImageHeight: 0,
        viewportScrollOffset: 0,
        naturalAspectRatio: 0,
        focusImageHeight: 0
    };

    /**
     * Save focus state (scroll position within modal and main page) to localStorage
     */
    function saveFocusState(modalScrollTop) {
        try {
            const state = {
                imageIndex: focusStateData.imageIndex,
                mainImageWidth: focusStateData.mainImageWidth,
                mainImageHeight: focusStateData.mainImageHeight,
                viewportScrollOffset: focusStateData.viewportScrollOffset,
                naturalAspectRatio: focusStateData.naturalAspectRatio,
                focusImageHeight: focusStateData.focusImageHeight,
                modalScrollTop: modalScrollTop || 0,
                mainPageScrollTop: scrollPositionBeforeFocus,
                timestamp: Date.now()
            };
            localStorage.setItem(FOCUS_STATE_STORAGE_KEY, JSON.stringify(state));
        } catch (e) {
            console.warn('Could not save focus state to localStorage:', e);
        }
    }

    /**
     * Get saved focus state from localStorage
     */
    function getSavedFocusState() {
        try {
            const saved = localStorage.getItem(FOCUS_STATE_STORAGE_KEY);
            return saved ? JSON.parse(saved) : null;
        } catch (e) {
            console.warn('Could not retrieve focus state from localStorage:', e);
            return null;
        }
    }

    /**
     * Clear focus state from localStorage
     */
    function clearFocusState() {
        try {
            localStorage.removeItem(FOCUS_STATE_STORAGE_KEY);
        } catch (e) {
            console.warn('Could not clear focus state from localStorage:', e);
        }
    }

    /**
     * Create focus modal HTML structure
     */
    function createFocusModal() {
        const modal = document.createElement('div');
        modal.className = 'webtoon-focus-modal';
        modal.id = 'webtoon-focus-modal';
        
        const overlay = document.createElement('div');
        overlay.className = 'webtoon-focus-modal-overlay';
        overlay.addEventListener('click', closeFocusModal);
        
        const scrollContainer = document.createElement('div');
        scrollContainer.className = 'webtoon-focus-modal-scroll';
        scrollContainer.id = 'webtoon-focus-modal-scroll';
        scrollContainer.addEventListener('click', (e) => {
            // Close if clicking on the scrollContainer directly
            if (e.target === scrollContainer) {
                closeFocusModal();
            }
        });
        
        const closeBtn = document.createElement('button');
        closeBtn.className = 'webtoon-focus-modal-close';
        closeBtn.innerHTML = '&times;';
        closeBtn.setAttribute('aria-label', 'Close');
        closeBtn.addEventListener('click', closeFocusModal);
        
        modal.appendChild(overlay);
        modal.appendChild(scrollContainer);
        modal.appendChild(closeBtn);
        
        document.body.appendChild(modal);
        return modal;
    }

    /**
     * Get or create the focus modal
     */
    function getFocusModal() {
        if (!focusModal) {
            focusModal = createFocusModal();
        }
        return focusModal;
    }

    /**
     * Handle image click inside focus modal - prevent closing/navigation
     */
    function handleFocusModalImageClick(e) {
        // Stop propagation so clicks on images don't trigger modal close
        e.stopPropagation();
    }

    /**
     * Open focus modal with existing images
     */
    function openFocusModal(clickedImg) {
        const modal = getFocusModal();
        const scrollContainer = modal.querySelector('.webtoon-focus-modal-scroll');
        
        // Check if we have a saved focus state to restore
        const savedState = getSavedFocusState();
        const shouldRestoreScroll = savedState && clickedImg === undefined;
        
        // Store current main page scroll position
        const mainElement = document.querySelector('main.site-main');
        scrollPositionBeforeFocus = mainElement ? mainElement.scrollTop : (window.scrollY || document.documentElement.scrollTop);
        
        // Find which image was clicked
        let clickedIndex = -1;
        const mainImages = containerElement.querySelectorAll('img');
        mainImages.forEach((img, index) => {
            if (img === clickedImg) clickedIndex = index;
        });
        if (clickedIndex === -1 && clickedImg.src) {
            clickedIndex = Array.from(mainImages).findIndex(img => img.src === clickedImg.src);
        }
        if (clickedIndex === -1) clickedIndex = 0;
        
        // Store the main image dimensions and click position
        const mainImageWidth = clickedImg.offsetWidth;
        const mainImageHeight = clickedImg.offsetHeight;
        
        // Calculate the position of the clicked image from the start of the container
        let imageTopPosition = 0;
        for (let i = 0; i < clickedIndex; i++) {
            imageTopPosition += mainImages[i].offsetHeight;
        }
        
        // Get container's top padding to account for it in scroll calculations
        const containerStyles = window.getComputedStyle(containerElement);
        const containerPaddingTop = parseFloat(containerStyles.paddingTop) || 0;
        
        // Calculate offset within the clicked image based on current scroll position
        // We need to find how far into this image the current scroll is
        // Account for container offset AND container padding
        const scrollOffsetFromImageStart = scrollPositionBeforeFocus - (imageTopPosition + containerElement.offsetTop + containerPaddingTop);
        
        // Get natural aspect ratio for later calculations
        const naturalAspectRatio = clickedImg.naturalWidth / clickedImg.naturalHeight;
        
        // Store all needed data
        focusStateData = {
            imageIndex: clickedIndex,
            mainImageWidth: mainImageWidth,
            mainImageHeight: mainImageHeight,
            viewportScrollOffset: scrollOffsetFromImageStart,
            naturalAspectRatio: naturalAspectRatio
        };
        
        // Clone images to modal instead of moving them
        scrollContainer.innerHTML = '';
        mainImages.forEach((img, index) => {
            const clone = img.cloneNode(true);
            // Ensure cloned images have no margins, padding or gaps to prevent spacing issues when resizing
            clone.style.display = 'block';
            clone.style.margin = '0';
            clone.style.padding = '0';
            clone.addEventListener('click', handleFocusModalImageClick);
            scrollContainer.appendChild(clone);
        });
        
        // Activate modal
        modal.classList.add('active');
        document.body.classList.add('webtoon-focus-open');
        
        // Immediately calculate and set scroll position before animation
        const focusImages = scrollContainer.querySelectorAll('img');
        if (focusImages[clickedIndex]) {
            const focusImage = focusImages[clickedIndex];
            
            // Calculate target scroll position
            let targetScrollTop = 0;
            
            // Sum all previous images
            for (let i = 0; i < clickedIndex; i++) {
                targetScrollTop += focusImages[i].offsetHeight;
            }
            
            // Add offset into current image, scaled by height change
            const focusImageHeight = focusImage.offsetHeight;
            const heightRatio = focusImageHeight / mainImageHeight;
            const scaledOffset = focusStateData.viewportScrollOffset * heightRatio;
            targetScrollTop += scaledOffset;
            
            // Use saved state if restoring
            if (shouldRestoreScroll && savedState && savedState.modalScrollTop !== undefined) {
                targetScrollTop = savedState.modalScrollTop;
            }
            
            // Store the calculated target for later restoration
            focusStateData.targetScrollTop = targetScrollTop;
            focusStateData.focusImageHeight = focusImageHeight;
            
            // Set scroll position directly to target - the modal fade-in animation provides visual feedback
            scrollContainer.scrollTop = targetScrollTop;
            
            // Save state immediately
            saveFocusState(targetScrollTop);
        }
    }

    /**
     * Close focus modal and return to normal view
     */
    function closeFocusModal() {
        if (!focusModal) return;
        
        const scrollContainer = focusModal.querySelector('.webtoon-focus-modal-scroll');
        let modalScrollPosition = 0;
        let offsetInFocusImage = 0;
        let focusImageHeight = 0;
        
        if (scrollContainer) {
            modalScrollPosition = scrollContainer.scrollTop;
            
            // Calculate offset from where we are in focus mode
            const focusImages = scrollContainer.querySelectorAll('img');
            let currentImageTopInModal = 0;
            for (let i = 0; i < focusStateData.imageIndex; i++) {
                currentImageTopInModal += focusImages[i].offsetHeight;
            }
            offsetInFocusImage = modalScrollPosition - currentImageTopInModal;
            
            // Capture focus image height before clearing
            if (focusImages[focusStateData.imageIndex]) {
                focusImageHeight = focusImages[focusStateData.imageIndex].offsetHeight;
            }
            
            // Reset zoom level for all images
            scrollContainer.querySelectorAll('img').forEach(img => {
                img.style.transform = '';
                img.style.transformOrigin = '';
            });
        }
        
        focusModalZoom = 1;
        
        // Calculate and set the scroll position in main view BEFORE closing the modal
        updateMainViewScrollPosition(offsetInFocusImage, focusImageHeight);
        
        // Add closing class to trigger backdrop animation
        focusModal.classList.add('closing');
        
        // Wait for animation to complete, then finalize
        setTimeout(() => {
            finishCloseFocusModal(scrollContainer, modalScrollPosition);
        }, 350);
    }
    
    /**
     * Update main view scroll position based on focus modal scroll
     */
    function updateMainViewScrollPosition(offsetInFocusImage, focusImageHeight) {
        const mainElement = document.querySelector('main.site-main');
        const mainImages = containerElement.querySelectorAll('img');
        let targetScrollTop = scrollPositionBeforeFocus;
        
        if (mainImages[focusStateData.imageIndex]) {
            // Get container's top padding
            const containerStyles = window.getComputedStyle(containerElement);
            const containerPaddingTop = parseFloat(containerStyles.paddingTop) || 0;
            
            // Calculate position of the target image from the start of container
            let imageTopPosition = 0;
            for (let i = 0; i < focusStateData.imageIndex; i++) {
                imageTopPosition += mainImages[i].offsetHeight;
            }
            
            // Scale the offset from focus modal back to main view dimensions
            const mainImageHeight = mainImages[focusStateData.imageIndex].offsetHeight;
            const effectiveFocusImageHeight = focusImageHeight || focusStateData.focusImageHeight || mainImageHeight;
            
            // Scale the offset proportionally based on height ratio
            const heightRatio = mainImageHeight / effectiveFocusImageHeight;
            const scaledOffset = offsetInFocusImage * heightRatio;
            
            // Update the viewport scroll offset to reflect how far the user actually scrolled in focus mode
            focusStateData.viewportScrollOffset = scaledOffset;
            
            // Calculate total scroll position: account for container offset AND container padding
            targetScrollTop = containerElement.offsetTop + containerPaddingTop + imageTopPosition + focusStateData.viewportScrollOffset;
        }
        
        // Set scroll position immediately - no animation
        if (mainElement) {
            mainElement.scrollTop = targetScrollTop;
        } else {
            window.scrollTo({
                top: targetScrollTop,
                left: 0
            });
        }
    }
    
    /**
     * Finish closing focus modal after animation
     */
    function finishCloseFocusModal(scrollContainer, modalScrollPosition) {
        // Deactivate modal
        focusModal.classList.remove('active');
        focusModal.classList.remove('closing');
        document.body.classList.remove('webtoon-focus-open');
        
        // Clear the scroll container
        if (scrollContainer) {
            scrollContainer.innerHTML = '';
        }
        
        // Save state
        saveFocusState(modalScrollPosition);
    }

    /**
     * Handle focus modal keyboard events
     */
    function handleFocusModalKeyboard(e) {
        if (focusModal && focusModal.classList.contains('active')) {
            if (e.key === 'Escape') {
                closeFocusModal();
                e.preventDefault();
            }
        }
    }

    /**
     * Set up focus modal event listeners
     */
    function setupFocusModal() {
        document.addEventListener('keydown', handleFocusModalKeyboard);
        document.addEventListener('wheel', handleFocusModalWheel, { passive: false });
    }

    /**
     * Handle wheel scroll in focus modal
     */
    function handleFocusModalWheel(e) {
        if (!focusModal || !focusModal.classList.contains('active')) return;
        
        const scrollContainer = focusModal.querySelector('.webtoon-focus-modal-scroll');
        if (!scrollContainer) return;
        
        // Check if CTRL (or CMD on Mac) is pressed for zoom
        if (e.ctrlKey || e.metaKey) {
            // Prevent default zoom behavior
            e.preventDefault();
            
            // Calculate zoom change (deltaY is negative for scroll up = zoom in)
            const zoomStep = 0.1;
            const direction = e.deltaY > 0 ? -1 : 1; // Invert: scroll down = zoom out
            const newZoom = Math.max(0.5, Math.min(3, focusModalZoom + (direction * zoomStep)));
            
            if (newZoom !== focusModalZoom) {
                // Get current scroll position and viewport info
                const oldZoom = focusModalZoom;
                const scrollTop = scrollContainer.scrollTop;
                const viewportHeight = scrollContainer.clientHeight;
                
                // Calculate the center of the viewport in the unscaled coordinate system
                const centerInView = scrollTop + viewportHeight / 2;
                
                focusModalZoom = newZoom;
                
                // Apply zoom to all images in the modal
                const images = scrollContainer.querySelectorAll('img');
                images.forEach(img => {
                    img.style.transform = `scale(${focusModalZoom})`;
                    img.style.transformOrigin = 'top center';
                });
                
                // Adjust scroll position to keep the center point in view
                // When zooming, the scaled content changes, so we need to adjust the scroll
                const zoomRatio = newZoom / oldZoom;
                const newScrollTop = centerInView * zoomRatio - viewportHeight / 2;
                
                // Use requestAnimationFrame to ensure DOM has updated before scrolling
                requestAnimationFrame(() => {
                    scrollContainer.scrollTop = Math.max(0, newScrollTop);
                });
            }
        } else {
            // Normal scrolling behavior
            e.preventDefault();
            
            // Scroll the modal instead
            scrollContainer.scrollTop += e.deltaY;
        }
    }

    /**
     * Apply zoom to the focus modal images and update the zoom input
     */
    function applyZoom(newZoom) {
        if (!focusModal) return;
        
        const scrollContainer = focusModal.querySelector('.webtoon-focus-modal-scroll');
        if (!scrollContainer) return;
        
        newZoom = Math.max(0.5, Math.min(3, newZoom));
        
        if (newZoom !== focusModalZoom) {
            const oldZoom = focusModalZoom;
            const scrollTop = scrollContainer.scrollTop;
            const viewportHeight = scrollContainer.clientHeight;
            
            // Calculate the center of the viewport in the unscaled coordinate system
            const centerInView = scrollTop + viewportHeight / 2;
            
            focusModalZoom = newZoom;
            
            // Update the zoom input field
            const zoomInput = focusModal.querySelector('#webtoon-zoom-input');
            if (zoomInput) {
                zoomInput.value = Math.round(focusModalZoom * 100) + '%';
            }
            
            // Apply zoom to all images in the modal
            const images = scrollContainer.querySelectorAll('img');
            images.forEach(img => {
                img.style.transform = `scale(${focusModalZoom})`;
                img.style.transformOrigin = 'top center';
            });
            
            // Adjust scroll position to keep the center point in view
            const zoomRatio = newZoom / oldZoom;
            const newScrollTop = centerInView * zoomRatio - viewportHeight / 2;
            
            // Use requestAnimationFrame to ensure DOM has updated before scrolling
            requestAnimationFrame(() => {
                scrollContainer.scrollTop = Math.max(0, newScrollTop);
            });
        }
    }

    /**
     * Handle zoom button clicks (minus/plus buttons)
     */
    function handleZoomClick(direction) {
        const zoomStep = 0.1;
        const newZoom = focusModalZoom + (direction * zoomStep);
        applyZoom(newZoom);
    }

    /**
     * Handle zoom input change
     */
    function handleZoomInputChange(e) {
        let value = e.target.value.trim();
        
        // Remove the % sign if present
        value = value.replace('%', '').trim();
        
        // Parse as a number
        const zoomPercent = parseFloat(value);
        
        if (!isNaN(zoomPercent) && zoomPercent > 0) {
            const newZoom = Math.max(0.5, Math.min(3, zoomPercent / 100));
            applyZoom(newZoom);
        } else {
            // Reset to current zoom if invalid
            const zoomInput = focusModal.querySelector('#webtoon-zoom-input');
            if (zoomInput) {
                zoomInput.value = Math.round(focusModalZoom * 100) + '%';
            }
        }
    }

    /**
     * Handle zoom input blur (restore valid value if needed)
     */
    function handleZoomInputBlur(e) {
        const zoomInput = e.target;
        if (zoomInput && focusModal) {
            // Ensure the input always shows a valid zoom percentage
            zoomInput.value = Math.round(focusModalZoom * 100) + '%';
        }
    }

    /**
     * Handle image click in webtoon mode
     */
    function handleWebtoonImageClick(e) {
        if (currentMode === MODES.WEBTOON) {
            const img = e.target;
            if (img.tagName === 'IMG') {
                openFocusModal(img);
            }
        }
    }

    /**
     * Get saved reading mode from localStorage
     */
    function getStoredMode() {
        try {
            const stored = localStorage.getItem(STORAGE_KEY);
            return stored && Object.values(MODES).includes(stored) ? stored : MODES.WEBTOON;
        } catch (e) {
            console.warn('Could not access localStorage:', e);
            return MODES.WEBTOON;
        }
    }

    /**
     * Save reading mode to localStorage
     */
    function saveMode(mode) {
        try {
            localStorage.setItem(STORAGE_KEY, mode);
        } catch (e) {
            console.warn('Could not save to localStorage:', e);
        }
    }

    /**
     * Set the reading mode
     */
    function setMode(mode) {
        if (!Object.values(MODES).includes(mode)) {
            console.error('Invalid reading mode:', mode);
            return;
        }

        currentMode = mode;
        saveMode(mode);
        updateModeButtons();
        renderCurrentMode();
    }

    /**
     * Update mode button states
     */
    function updateModeButtons() {
        document.querySelectorAll('.reader-mode-btn').forEach(btn => {
            const btnMode = btn.dataset.mode;
            if (btnMode === currentMode) {
                btn.classList.add('uk-active');
            } else {
                btn.classList.remove('uk-active');
            }
        });
    }

    /**
     * Render images based on current mode
     */
    function renderCurrentMode() {
        if (!containerElement) return;

        // Ensure container is visible
        containerElement.style.display = 'block';

        switch (currentMode) {
            case MODES.WEBTOON:
                renderWebtoonMode();
                break;
            case MODES.SINGLE:
                renderSinglePageMode();
                break;
            case MODES.SIDE_BY_SIDE:
                renderSideBySideMode();
                break;
        }

        // Hide/show pagination controls
        const paginationControls = document.getElementById('reader-pagination');
        const paginationBottom = document.getElementById('reader-pagination-bottom');
        if (paginationControls) {
            paginationControls.style.display = currentMode === MODES.WEBTOON ? 'none' : 'flex';
            if (currentMode !== MODES.WEBTOON) {
                paginationControls.classList.remove('hide-controls');
            }
        }
        if (paginationBottom) {
            paginationBottom.style.display = currentMode === MODES.WEBTOON ? 'none' : 'flex';
            if (currentMode !== MODES.WEBTOON) {
                paginationBottom.classList.remove('hide-controls');
            }
        }

        // Restart auto-hide timer when mode changes
        if (currentMode !== MODES.WEBTOON) {
            showPaginationControls();
        }
    }

    /**
     * Render webtoon mode (vertical scroll)
     */
    function renderWebtoonMode() {
        // Reset to original classes for webtoon mode with centering
        containerElement.className = 'flex flex-col items-center justify-center p-0 sm:p-4 w-full mx-auto';
        containerElement.style.maxWidth = '1200px';
        
        // Clear container safely
        containerElement.textContent = '';
        
        // Create and append images safely using DOM methods
        images.forEach(src => {
            const img = document.createElement('img');
            img.src = src;
            img.className = 'h-auto max-w-full mx-auto';
            img.alt = 'loading page...';
            img.style.cursor = 'pointer';
            img.addEventListener('click', handleWebtoonImageClick);
            containerElement.appendChild(img);
        });
    }

    /**
     * Render single page mode
     */
    function renderSinglePageMode() {
        // Remove flex-col and width constraints for single page
        containerElement.className = 'reader-single-page-container';
        
        if (images.length === 0) {
            containerElement.textContent = 'No pages available';
            return;
        }

        const imgSrc = images[currentPage] || images[0];
        
        // Clear container safely
        containerElement.textContent = '';
        
        // Create elements safely using DOM methods
        const pageDiv = document.createElement('div');
        pageDiv.className = 'reader-single-page';
        
        const img = document.createElement('img');
        img.src = imgSrc;
        img.className = 'reader-page-image';
        img.alt = `Page ${currentPage + 1}`;
        img.style.cursor = 'pointer';
        
        // Add click handler for navigation
        img.addEventListener('click', handleImageClick);
        
        pageDiv.appendChild(img);
        containerElement.appendChild(pageDiv);

        updatePageCounter();
    }

    /**
     * Render side-by-side mode (two pages)
     */
    function renderSideBySideMode() {
        // Remove ALL existing classes that might interfere
        containerElement.className = 'reader-side-by-side-container';
        
        if (images.length === 0) {
            containerElement.textContent = 'No pages available';
            return;
        }

        // For side-by-side, show two pages at once
        const leftPage = images[currentPage];
        const rightPage = images[currentPage + 1];

        // Clear container safely
        containerElement.textContent = '';
        
        // Create wrapper div
        const wrapperDiv = document.createElement('div');
        wrapperDiv.className = 'reader-side-by-side';
        
        // Always render left page if it exists
        if (leftPage) {
            const leftDiv = document.createElement('div');
            leftDiv.className = 'reader-page-left';
            
            const leftImg = document.createElement('img');
            leftImg.src = leftPage;
            leftImg.className = 'reader-page-image';
            leftImg.alt = `Page ${currentPage + 1}`;
            leftImg.style.cursor = 'pointer';
            
            // Add click handler for navigation
            leftImg.addEventListener('click', handleImageClick);
            
            leftDiv.appendChild(leftImg);
            wrapperDiv.appendChild(leftDiv);
        } else {
            console.warn('Left page is undefined at index:', currentPage);
        }
        
        // Always render right page if it exists
        if (rightPage) {
            const rightDiv = document.createElement('div');
            rightDiv.className = 'reader-page-right';
            
            const rightImg = document.createElement('img');
            rightImg.src = rightPage;
            rightImg.className = 'reader-page-image';
            rightImg.alt = `Page ${currentPage + 2}`;
            rightImg.style.cursor = 'pointer';
            
            // Add click handler for navigation
            rightImg.addEventListener('click', handleImageClick);
            
            rightDiv.appendChild(rightImg);
            wrapperDiv.appendChild(rightDiv);
        }
        
        containerElement.appendChild(wrapperDiv);

        updatePageCounter();
    }

    /**
     * Handle click on image for navigation
     * Left 50% goes to previous page, right 50% goes to next page
     */
    function handleImageClick(e) {
        // Only handle in single or side-by-side mode
        if (currentMode === MODES.WEBTOON) return;

        const img = e.currentTarget;
        const rect = img.getBoundingClientRect();
        const clickX = e.clientX - rect.left;
        const imgWidth = rect.width;
        
        // Determine if click was on left or right half
        if (clickX < imgWidth / 2) {
            // Left half - go to previous page
            prevPage();
        } else {
            // Right half - go to next page
            nextPage();
        }
    }

    /**
     * Navigate to next page(s)
     */
    function nextPage() {
        if (currentMode === MODES.WEBTOON) return;

        const increment = currentMode === MODES.SIDE_BY_SIDE ? 2 : 1;
        
        if (currentMode === MODES.SIDE_BY_SIDE) {
            // For side-by-side, ensure we don't go past the last page
            if (currentPage + increment < images.length) {
                currentPage += increment;
            } else if (currentPage + 1 < images.length) {
                // If only one page left, move to it
                currentPage += 1;
            } else {
                return; // Already at the end
            }
        } else {
            if (currentPage < images.length - 1) {
                currentPage += increment;
            } else {
                return; // Already at the end
            }
        }
        
        renderCurrentMode();
        scrollToTopInstant();
    }

    /**
     * Navigate to previous page(s)
     */
    function prevPage() {
        if (currentMode === MODES.WEBTOON) return;

        const decrement = currentMode === MODES.SIDE_BY_SIDE ? 2 : 1;

        if (currentPage >= decrement) {
            currentPage -= decrement;
            renderCurrentMode();
            scrollToTopInstant();
        }
    }

    /**
     * Go to first page
     */
    function firstPage() {
        if (currentMode === MODES.WEBTOON) return;
        currentPage = 0;
        renderCurrentMode();
        scrollToTopInstant();
    }

    /**
     * Go to last page
     */
    function lastPage() {
        if (currentMode === MODES.WEBTOON) return;
        
        if (currentMode === MODES.SIDE_BY_SIDE) {
            // Go to the last pair of pages
            currentPage = images.length % 2 === 0 ? images.length - 2 : images.length - 1;
        } else {
            currentPage = images.length - 1;
        }
        renderCurrentMode();
        scrollToTopInstant();
    }

    /**
     * Update page counter display
     */
    function updatePageCounter() {
        const counter = document.getElementById('page-counter');
        const counterBottom = document.getElementById('page-counter-bottom');
        
        let counterText = '';
        if (currentMode === MODES.SIDE_BY_SIDE) {
            const endPage = Math.min(currentPage + 2, images.length);
            counterText = `${currentPage + 1}-${endPage} / ${images.length}`;
        } else {
            counterText = `${currentPage + 1} / ${images.length}`;
        }
        
        if (counter) counter.textContent = counterText;
        if (counterBottom) counterBottom.textContent = counterText;

        // Update button states
        const prevBtn = document.getElementById('prev-page-btn');
        const nextBtn = document.getElementById('next-page-btn');
        const firstBtn = document.getElementById('first-page-btn');
        const lastBtn = document.getElementById('last-page-btn');

        if (prevBtn) prevBtn.disabled = currentPage === 0;
        if (firstBtn) firstBtn.disabled = currentPage === 0;

        const atEnd = currentMode === MODES.SIDE_BY_SIDE ? 
            currentPage >= images.length - 1 : currentPage >= images.length - 1;
        
        if (nextBtn) nextBtn.disabled = atEnd;
        if (lastBtn) lastBtn.disabled = atEnd;
    }

    /**
     * Scroll to top instantly (used when changing pages)
     */
    function scrollToTopInstant() {
        // Scroll the reader container into view, centered when possible
        if (containerElement) {
            containerElement.scrollIntoView({ behavior: 'auto', block: 'center', inline: 'nearest' });
        } else if (window.scrollToTopInstant) {
            window.scrollToTopInstant();
        } else {
            window.scrollTo({ top: 0, behavior: 'auto' });
        }
    }

    /**
     * Handle keyboard navigation
     */
    function handleKeyboard(e) {
        if (currentMode === MODES.WEBTOON) return;

        // Ignore if user is typing in an input
        if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') {
            return;
        }

        switch(e.key) {
            case 'ArrowLeft':
                prevPage();
                e.preventDefault();
                break;
            case 'ArrowRight':
                nextPage();
                e.preventDefault();
                break;
            case 'Home':
                firstPage();
                e.preventDefault();
                break;
            case 'End':
                lastPage();
                e.preventDefault();
                break;
        }
    }

    /**
     * Auto-hide pagination controls on mouse inactivity
     */
    let hideControlsTimeout = null;
    let stopMovingTimeout = null;
    let isMouseMoving = false;
    
    function showPaginationControls() {
        const topControls = document.getElementById('reader-pagination');
        const bottomControls = document.getElementById('reader-pagination-bottom');
        
        if (topControls) topControls.classList.remove('hide-controls');
        if (bottomControls) bottomControls.classList.remove('hide-controls');
        
        // Clear existing timeouts
        if (hideControlsTimeout) {
            clearTimeout(hideControlsTimeout);
        }
        if (stopMovingTimeout) {
            clearTimeout(stopMovingTimeout);
        }
        
        // Mark that mouse is moving
        isMouseMoving = true;
        
        // Wait 500ms to confirm mouse has actually stopped moving
        stopMovingTimeout = setTimeout(() => {
            isMouseMoving = false;
            // After confirming mouse stopped, wait 4 more seconds before hiding
            hideControlsTimeout = setTimeout(() => {
                if (!isMouseMoving && currentMode !== MODES.WEBTOON) {
                    if (topControls) topControls.classList.add('hide-controls');
                    if (bottomControls) bottomControls.classList.add('hide-controls');
                }
            }, 4000);
        }, 500);
    }
    
    function setupAutoHideControls() {
        // Show controls on mouse move
        document.addEventListener('mousemove', showPaginationControls);
        
        // Show controls on touch
        document.addEventListener('touchstart', showPaginationControls);
        
        // Initial show
        showPaginationControls();
    }

    /**
     * Restore focus mode from saved state if available
     */
    function restoreFocusState() {
        const savedState = getSavedFocusState();
        if (!savedState) {
            return;
        }
        
        // Restore the focus state data
        focusStateData = {
            imageIndex: savedState.imageIndex || 0,
            mainImageWidth: savedState.mainImageWidth || 0,
            mainImageHeight: savedState.mainImageHeight || 0,
            viewportScrollOffset: savedState.viewportScrollOffset || 0,
            naturalAspectRatio: savedState.naturalAspectRatio || 0,
            focusImageHeight: savedState.focusImageHeight || 0
        };
        
        // Restore main page scroll position
        scrollPositionBeforeFocus = savedState.mainPageScrollTop || 0;
        
        // Check if we still have images loaded
        const mainImages = containerElement.querySelectorAll('img');
        if (mainImages.length === 0) {
            clearFocusState();
            return;
        }
        
        // Open focus modal without clicking an image
        const clickedImg = mainImages[focusStateData.imageIndex];
        if (clickedImg) {
            openFocusModal(clickedImg);
        }
    }

    /**
     * Initialize the reader
     */
    function init() {
        // Clear any previous focus state when loading a new chapter
        clearFocusState();
        
        // Get the container element
        containerElement = document.getElementById('reader-images-container');
        if (!containerElement) {
            console.warn('Reader container not found');
            return;
        }

        // Extract images from data attribute or existing img tags
        const imagesData = containerElement.dataset.images;
        if (imagesData) {
            try {
                images = JSON.parse(imagesData);
            } catch (e) {
                console.error('Could not parse images data:', e);
                return;
            }
        } else {
            // Fallback: extract from existing img tags
            const existingImages = containerElement.querySelectorAll('img[src]');
            images = Array.from(existingImages).map(img => img.src);
        }

        if (images.length === 0) {
            console.warn('No images found for reader');
            return;
        }

        // Set reading mode based on manga type before loading images
        const mangaType = containerElement.dataset.mangaType;
        if (mangaType === 'webtoon' || mangaType === 'manhwa') {
            // Webtoons and manhwa default to webtoon reading mode
            currentMode = MODES.WEBTOON;
            saveMode(MODES.WEBTOON);
        } else if (mangaType === 'manga') {
            // Manga defaults to single page reading mode
            currentMode = MODES.SINGLE;
            saveMode(MODES.SINGLE);
        } else {
            // Load saved mode for other types
            currentMode = getStoredMode();
        }
        currentPage = 0;

        // Set up mode buttons
        document.querySelectorAll('.reader-mode-btn').forEach(btn => {
            btn.addEventListener('click', (e) => {
                e.preventDefault();
                setMode(btn.dataset.mode);
            });
        });

        // Set up pagination buttons
        const prevBtn = document.getElementById('prev-page-btn');
        const nextBtn = document.getElementById('next-page-btn');
        const firstBtn = document.getElementById('first-page-btn');
        const lastBtn = document.getElementById('last-page-btn');

        if (prevBtn) prevBtn.addEventListener('click', prevPage);
        if (nextBtn) nextBtn.addEventListener('click', nextPage);
        if (firstBtn) firstBtn.addEventListener('click', firstPage);
        if (lastBtn) lastBtn.addEventListener('click', lastPage);

        // Set up keyboard navigation
        document.addEventListener('keydown', handleKeyboard);

        // Set up auto-hide controls
        setupAutoHideControls();

        // Set up focus modal
        setupFocusModal();

        // Initial render
        updateModeButtons();
        renderCurrentMode();
    }

    // Expose public API
    window.MangaReader = {
        init,
        setMode,
        nextPage,
        prevPage,
        firstPage,
        lastPage,
        restoreFocusState,
        clearFocusState,
        MODES
    };

    // Auto-initialize when DOM is ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        // DOM already loaded, check if we need to initialize
        if (document.getElementById('reader-images-container')) {
            init();
        }
    }

    // Re-initialize on HTMX content swap
    document.addEventListener('htmx:afterSwap', (event) => {
        if (event.detail.target && event.detail.target.id === 'content' && document.getElementById('reader-images-container')) {
            setTimeout(init, 50); // Small delay to ensure DOM is ready
        }
    });
})();
