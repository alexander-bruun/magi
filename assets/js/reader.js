/**
 * Reader Module - Handles different reading modes for manga chapters
 * Supports: webtoon (vertical scroll), single-page, and side-by-side modes
 */
(function() {
    'use strict';

    const STORAGE_KEY = 'magi-reading-mode';
    const MODES = {
        WEBTOON: 'webtoon',
        SINGLE: 'single',
        SIDE_BY_SIDE: 'side-by-side'
    };

    let currentMode = MODES.WEBTOON;
    let currentPage = 0;
    let images = [];
    let containerElement = null;

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
        }
        if (paginationBottom) {
            paginationBottom.style.display = currentMode === MODES.WEBTOON ? 'none' : 'flex';
        }

        // Re-initialize lazy loading for new images
        if (window.LazyLoad && window.LazyLoad.init) {
            window.LazyLoad.init();
        }
    }

    /**
     * Render webtoon mode (vertical scroll)
     */
    function renderWebtoonMode() {
        // Reset to original classes for webtoon mode
        containerElement.className = 'flex flex-col items-center p-0 sm:p-4 w-full lg:w-3/5';
        containerElement.innerHTML = images.map(src => 
            `<img data-src="${src}" class="w-full h-auto max-w-full" alt="loading page..."/>`
        ).join('');
    }

    /**
     * Render single page mode
     */
    function renderSinglePageMode() {
        // Remove flex-col and width constraints for single page
        containerElement.className = 'reader-single-page-container';
        
        if (images.length === 0) {
            containerElement.innerHTML = '<p class="uk-text-muted">No pages available</p>';
            return;
        }

        const imgSrc = images[currentPage] || images[0];
        containerElement.innerHTML = `
            <div class="reader-single-page">
                <img src="${imgSrc}" class="reader-page-image" alt="Page ${currentPage + 1}"/>
            </div>
        `;

        updatePageCounter();
    }

    /**
     * Render side-by-side mode (two pages)
     */
    function renderSideBySideMode() {
        // Remove ALL existing classes that might interfere
        containerElement.className = 'reader-side-by-side-container';
        
        if (images.length === 0) {
            containerElement.innerHTML = '<p class="uk-text-muted">No pages available</p>';
            return;
        }

        // For side-by-side, show two pages at once
        const leftPage = images[currentPage];
        const rightPage = images[currentPage + 1];

        console.log('Side-by-side mode:', {
            currentPage,
            totalImages: images.length,
            leftPage,
            rightPage,
            allImages: images
        });

        let html = '<div class="reader-side-by-side">';
        
        // Always render left page if it exists
        if (leftPage) {
            html += `<div class="reader-page-left">
                <img src="${leftPage}" class="reader-page-image" alt="Page ${currentPage + 1}"/>
            </div>`;
        } else {
            console.warn('Left page is undefined at index:', currentPage);
        }
        
        // Always render right page if it exists
        if (rightPage) {
            html += `<div class="reader-page-right">
                <img src="${rightPage}" class="reader-page-image" alt="Page ${currentPage + 2}"/>
            </div>`;
        } else {
            console.log('Right page is undefined at index:', currentPage + 1);
        }
        
        html += '</div>';
        containerElement.innerHTML = html;

        updatePageCounter();
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
        if (window.scrollToTopInstant) {
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
     * Initialize the reader
     */
    function init() {
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
            const existingImages = containerElement.querySelectorAll('img[data-src]');
            images = Array.from(existingImages).map(img => img.dataset.src);
        }

        if (images.length === 0) {
            console.warn('No images found for reader');
            return;
        }

        // Load saved mode
        currentMode = getStoredMode();
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
        if (event.detail.target.id === 'content' && document.getElementById('reader-images-container')) {
            setTimeout(init, 50); // Small delay to ensure DOM is ready
        }
    });
})();
