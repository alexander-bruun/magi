/**
 * Browser Challenge Page Solver - Immediate challenge solving for challenge pages
 * This script runs immediately on challenge pages to solve the proof-of-work
 * and redirect to the original content.
 */
(async function(){
	const challenge = document.body.dataset.challenge;
	const signature = document.body.dataset.signature;
	const difficulty = parseInt(document.body.dataset.difficulty);

	async function sha256(m){
		const b = new TextEncoder().encode(m);
		const h = await crypto.subtle.digest('SHA-256', b);
		return Array.from(new Uint8Array(h)).map(x => x.toString(16).padStart(2, '0')).join('');
	}

	function fingerprint(){
		const f = [];
		f.push(screen.width + 'x' + screen.height);
		f.push(screen.colorDepth);
		f.push(window.devicePixelRatio || 1);
		try { f.push(Intl.DateTimeFormat().resolvedOptions().timeZone); } catch(e) { f.push('?'); }
		f.push(navigator.language);
		f.push(navigator.platform);
		f.push(navigator.hardwareConcurrency || 0);
		f.push(navigator.deviceMemory || 0);
		f.push(navigator.maxTouchPoints || 0);
		try {
			const c = document.createElement('canvas');
			const g = c.getContext('webgl') || c.getContext('experimental-webgl');
			if (g) {
				const d = g.getExtension('WEBGL_debug_renderer_info');
				if (d) f.push(g.getParameter(d.UNMASKED_RENDERER_WEBGL));
			}
		} catch(e) {}
		try {
			const c = document.createElement('canvas');
			c.width = 200;
			c.height = 50;
			const x = c.getContext('2d');
			x.textBaseline = 'top';
			x.font = '14px Arial';
			x.fillStyle = '#f60';
			x.fillRect(125, 1, 62, 20);
			x.fillStyle = '#069';
			x.fillText('Cwm fjordbank', 2, 15);
			f.push(c.toDataURL().slice(-50));
		} catch(e) {}
		return f.join('|');
	}

	const target = '0'.repeat(difficulty);
	let answer = 0;
	let solved = false;
	while (!solved) {
		for (let i = 0; i < 5000; i++) {
			const h = await sha256(challenge + ':' + answer);
			if (h.substring(0, difficulty) === target) {
				const fp = fingerprint();
				const r = await fetch('/api/browser-challenge/verify', {
					method: 'POST',
					credentials: 'same-origin',
					headers: { 'Content-Type': 'application/json' },
					body: JSON.stringify({ nonce: challenge, solution: signature, answer: answer, fingerprint: fp })
				});
				if (r.ok) {
					solved = true;
					await new Promise(r => setTimeout(r, 100));
					const chk = await fetch('/api/browser-challenge/init', { method: 'GET', credentials: 'same-origin' });
					if (chk.ok) {
						const d = await chk.json();
						if (d.verified) {
							location.reload();
							return;
						}
					}
					location.reload();
					return;
				}
			}
			answer++;
		}
		await new Promise(r => setTimeout(r, 0));
	}
})();

setTimeout(function() {
	document.getElementById('error').style.display = 'block';
	document.getElementById('spinner').style.display = 'none';
}, 10000);