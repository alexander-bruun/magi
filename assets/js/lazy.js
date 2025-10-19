/**
 * Custom Lazy Loading with UIkit Spinner
 * A sleek and simple lazy loading implementation using Intersection Observer
 */
(function() {
    'use strict';

    // Configuration
    const config = {
        rootMargin: '50px', // Start loading 50px before image enters viewport
        threshold: 0.01,
        loadingClass: 'lazy-loading',
        loadedClass: 'lazy-loaded',
        errorClass: 'lazy-error'
    };

    /**
     * Create a spinner element overlay with Franken UI spinner
     */
    function createSpinner() {
        const overlay = document.createElement('div');
        overlay.className = 'lazy-spinner-overlay';
        
        const spinner = document.createElement('div');
        spinner.setAttribute('uk-spinner', 'ratio: 1');
        
        overlay.appendChild(spinner);
        return overlay;
    }

    /**
     * Load an image and handle success/error
     */
    function loadImage(img) {
        return new Promise((resolve, reject) => {
            const tempImg = new Image();
            
            tempImg.onload = () => {
                img.src = tempImg.src;
                if (img.dataset.srcset) {
                    img.srcset = img.dataset.srcset;
                }
                resolve(img);
            };
            
            tempImg.onerror = () => reject(img);
            
            // Start loading
            tempImg.src = img.dataset.src;
            if (img.dataset.srcset) {
                tempImg.srcset = img.dataset.srcset;
            }
        });
    }

    /**
     * Process a lazy image element
     */
    function processImage(img, observer) {
        // Don't process if already loading or loaded
        if (img.classList.contains(config.loadingClass) || 
            img.classList.contains(config.loadedClass)) {
            return;
        }

        // Add loading class
        img.classList.add(config.loadingClass);

        // Create and add spinner
        const spinner = createSpinner();
        const container = img.parentElement;
        container.style.position = container.style.position || 'relative';
        container.appendChild(spinner);

        // Load the image
        loadImage(img)
            .then((loadedImg) => {
                // Remove spinner
                spinner.remove();
                
                // Add loaded class, remove loading class
                loadedImg.classList.remove(config.loadingClass);
                loadedImg.classList.add(config.loadedClass);
                
                // Fade in effect
                loadedImg.style.opacity = '0';
                loadedImg.style.transition = 'opacity 0.3s ease-in-out';
                setTimeout(() => {
                    loadedImg.style.opacity = '1';
                }, 10);

                // Stop observing this image
                observer.unobserve(loadedImg);
            })
            .catch((failedImg) => {
                // Add error class
                failedImg.classList.remove(config.loadingClass);
                failedImg.classList.add(config.errorClass);
                
                // Stop observing this image
                observer.unobserve(failedImg);
            });
    }

    /**
     * Initialize lazy loading
     */
    function init() {
        // Check for browser support
        if (!('IntersectionObserver' in window)) {
            console.warn('IntersectionObserver not supported, loading all images immediately');
            // Fallback: load all images immediately
            document.querySelectorAll('img[data-src]').forEach(img => {
                img.src = img.dataset.src;
                if (img.dataset.srcset) {
                    img.srcset = img.dataset.srcset;
                }
            });
            return;
        }

        // Create intersection observer
        const observer = new IntersectionObserver((entries) => {
            entries.forEach(entry => {
                if (entry.isIntersecting) {
                    processImage(entry.target, observer);
                }
            });
        }, config);

        // Observe all lazy images
        document.querySelectorAll('img[data-src]').forEach(img => {
            observer.observe(img);
        });

        // Watch for dynamically added images (for HTMX and dynamic content)
        if ('MutationObserver' in window) {
            const mutationObserver = new MutationObserver((mutations) => {
                mutations.forEach(mutation => {
                    mutation.addedNodes.forEach(node => {
                        if (node.nodeType === 1) { // Element node
                            // Check if node itself is a lazy image
                            if (node.tagName === 'IMG' && node.dataset.src) {
                                observer.observe(node);
                            }
                            // Check for lazy images within the node
                            node.querySelectorAll && node.querySelectorAll('img[data-src]').forEach(img => {
                                observer.observe(img);
                            });
                        }
                    });
                });
            });

            mutationObserver.observe(document.body, {
                childList: true,
                subtree: true
            });
        }
    }

    // Initialize when DOM is ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }

    // Expose API for manual triggering if needed
    window.LazyLoad = {
        init: init,
        loadImage: function(selector) {
            const img = typeof selector === 'string' ? document.querySelector(selector) : selector;
            if (img && img.dataset.src) {
                const observer = new IntersectionObserver(() => {}, config);
                processImage(img, observer);
            }
        }
    };
})();
