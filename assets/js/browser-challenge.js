/**
 * Browser Challenge - Invisible bot detection using proof-of-work
 * This script automatically solves a JavaScript challenge in the background
 * to verify the client is running in a real browser environment.
 */
(function() {
	'use strict';
	
	var BC_STORAGE_KEY = '__magi_bc';
	var bcInitialized = false;
	var bcSolving = false;
	var bcRetryCount = 0;
	var MAX_RETRIES = 3;
	
	/**
	 * Compute SHA-256 hash using Web Crypto API
	 * @param {string} message - The message to hash
	 * @returns {Promise<string>} - Hex-encoded hash
	 */
	async function sha256(message) {
		var msgBuffer = new TextEncoder().encode(message);
		var hashBuffer = await crypto.subtle.digest('SHA-256', msgBuffer);
		var hashArray = Array.from(new Uint8Array(hashBuffer));
		return hashArray.map(function(b) { return b.toString(16).padStart(2, '0'); }).join('');
	}
	
	/**
	 * Solve the proof-of-work challenge
	 * Find a number that when appended to the challenge produces a hash with required leading zeros
	 * @param {string} challenge - The challenge string
	 * @param {number} difficulty - Number of leading zeros required
	 * @returns {Promise<number>} - The solution (answer)
	 */
	async function solveChallenge(challenge, difficulty) {
		var target = '';
		for (var i = 0; i < difficulty; i++) {
			target += '0';
		}
		
		var answer = 0;
		var batchSize = 5000; // Process in batches to avoid blocking UI
		
		while (true) {
			for (var i = 0; i < batchSize; i++) {
				var hash = await sha256(challenge + ':' + answer);
				if (hash.substring(0, difficulty) === target) {
					return answer;
				}
				answer++;
			}
			// Yield to main thread to keep UI responsive
			await new Promise(function(resolve) { setTimeout(resolve, 0); });
		}
	}
	
	/**
	 * Collect browser fingerprint - characteristics that scripts can't easily fake
	 * This binds the verification token to the specific browser/environment
	 * @returns {string} - Fingerprint string
	 */
	function collectFingerprint() {
		var fp = [];
		
		// Screen properties
		fp.push(screen.width + 'x' + screen.height);
		fp.push(screen.colorDepth);
		fp.push(window.devicePixelRatio || 1);
		
		// Timezone
		try {
			fp.push(Intl.DateTimeFormat().resolvedOptions().timeZone);
		} catch (e) {
			fp.push('unknown');
		}
		
		// Language
		fp.push(navigator.language);
		
		// Platform
		fp.push(navigator.platform);
		
		// Hardware concurrency (CPU cores)
		fp.push(navigator.hardwareConcurrency || 0);
		
		// Device memory (if available)
		fp.push(navigator.deviceMemory || 0);
		
		// Touch support
		fp.push(navigator.maxTouchPoints || 0);
		
		// WebGL renderer (hard to fake without a real browser)
		try {
			var canvas = document.createElement('canvas');
			var gl = canvas.getContext('webgl') || canvas.getContext('experimental-webgl');
			if (gl) {
				var debugInfo = gl.getExtension('WEBGL_debug_renderer_info');
				if (debugInfo) {
					fp.push(gl.getParameter(debugInfo.UNMASKED_RENDERER_WEBGL));
				}
			}
		} catch (e) {}
		
		// Canvas fingerprint (rendering differences between systems)
		try {
			var canvas = document.createElement('canvas');
			canvas.width = 200;
			canvas.height = 50;
			var ctx = canvas.getContext('2d');
			ctx.textBaseline = 'top';
			ctx.font = '14px Arial';
			ctx.fillStyle = '#f60';
			ctx.fillRect(125, 1, 62, 20);
			ctx.fillStyle = '#069';
			ctx.fillText('Cwm fjordbank glyphs vext quiz', 2, 15);
			ctx.fillStyle = 'rgba(102, 204, 0, 0.7)';
			ctx.fillText('Cwm fjordbank glyphs vext quiz', 4, 17);
			fp.push(canvas.toDataURL().slice(-50)); // Last 50 chars of data URL
		} catch (e) {}
		
		return fp.join('|');
	}

	/**
	 * Initialize and solve the browser challenge
	 */
	async function initBrowserChallenge() {
		if (bcInitialized || bcSolving) return;
		bcSolving = true;
		
		try {
			// Check if Web Crypto API is available (required for challenge)
			if (!window.crypto || !window.crypto.subtle) {
				console.debug('Browser challenge: Web Crypto API not available');
				bcSolving = false;
				return;
			}
			
			// Request a challenge from the server
			var initResp = await fetch('/api/browser-challenge/init', {
				method: 'GET',
				credentials: 'same-origin',
				headers: {
					'Accept': 'application/json'
				}
			});
			
			if (!initResp.ok) {
				console.debug('Browser challenge: Failed to get challenge');
				bcSolving = false;
				return;
			}
			
			var data = await initResp.json();
			
			if (data.verified) {
				// Already verified, no need to solve
				bcInitialized = true;
				bcSolving = false;
				sessionStorage.setItem(BC_STORAGE_KEY, 'verified');
				return;
			}
			
			// Solve the challenge in the background
			var answer = await solveChallenge(data.challenge, data.difficulty);
			
			// Collect browser fingerprint to bind the token
			var fingerprint = collectFingerprint();
			
			// Submit the solution with fingerprint
			var verifyResp = await fetch('/api/browser-challenge/verify', {
				method: 'POST',
				credentials: 'same-origin',
				headers: {
					'Content-Type': 'application/json',
					'Accept': 'application/json'
				},
				body: JSON.stringify({
					nonce: data.challenge,
					solution: data.signature,
					answer: answer,
					fingerprint: fingerprint
				})
			});
			
			if (verifyResp.ok) {
				var result = await verifyResp.json();
				if (result.success) {
					bcInitialized = true;
					sessionStorage.setItem(BC_STORAGE_KEY, 'verified');
					console.debug('Browser challenge: Verified');
				}
			} else {
				console.debug('Browser challenge: Verification failed');
				// Retry on failure
				if (bcRetryCount < MAX_RETRIES) {
					bcRetryCount++;
					bcSolving = false;
					setTimeout(initBrowserChallenge, 1000 * bcRetryCount);
					return;
				}
			}
		} catch (e) {
			console.debug('Browser challenge: Error', e);
			// Retry on error
			if (bcRetryCount < MAX_RETRIES) {
				bcRetryCount++;
				bcSolving = false;
				setTimeout(initBrowserChallenge, 1000 * bcRetryCount);
				return;
			}
		}
		
		bcSolving = false;
	}
	
	/**
	 * Check if we should run the challenge
	 * Skip if already verified in this session
	 */
	function shouldRunChallenge() {
		// Check session storage first (avoids unnecessary API calls)
		if (sessionStorage.getItem(BC_STORAGE_KEY) === 'verified') {
			bcInitialized = true;
			return false;
		}
		return true;
	}
	
	// Start the challenge when the page loads
	if (shouldRunChallenge()) {
		if (document.readyState === 'loading') {
			document.addEventListener('DOMContentLoaded', initBrowserChallenge);
		} else {
			// Small delay to not block initial page render
			setTimeout(initBrowserChallenge, 100);
		}
	}
	
	// Also run when tab becomes visible (in case it was backgrounded)
	document.addEventListener('visibilitychange', function() {
		if (document.visibilityState === 'visible' && !bcInitialized && shouldRunChallenge()) {
			initBrowserChallenge();
		}
	});
	
	// Expose minimal API for debugging
	window.__magiBC = {
		isVerified: function() { return bcInitialized; },
		retry: function() { 
			bcRetryCount = 0;
			bcSolving = false;
			bcInitialized = false;
			sessionStorage.removeItem(BC_STORAGE_KEY);
			initBrowserChallenge();
		}
	};
})();
