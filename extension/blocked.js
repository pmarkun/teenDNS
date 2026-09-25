// blocked.js — renders the reason a navigation was redirected here. The
// category/reason/group can come from the query string (future-proofing) but
// the primary channel is the service worker, which records the navigation that
// preceded this page. Keeping the redirect URL as a plain extensionPath target
// complies with the documented declarativeNetRequest contract.

const params = new URLSearchParams(location.search);

const fallbackReason = 'Este site faz parte do combinado da família.';
const reasonEl = document.getElementById('reason');
const categoryEl = document.getElementById('category');
const groupEl = document.getElementById('group');
const kickerEl = document.getElementById('kicker');

function render(reason, category, group) {
	reasonEl.textContent = reason || fallbackReason;
	categoryEl.textContent = category || '—';
	groupEl.textContent = group || '—';
	if (reason || group) {
		kickerEl.textContent = 'PROTEGIDO' + (group ? ' — ' + group.toUpperCase() : '');
	}
}

(async () => {
	if (params.has('reason') || params.has('category') || params.has('group')) {
		render(params.get('reason'), params.get('category'), params.get('group'));
		return;
	}
	try {
		const tab = await chrome.tabs.getCurrent();
		const info = await chrome.runtime.sendMessage({ command: 'blockedInfo', tabId: tab?.id });
		render(
			info?.decision?.reason,
			info?.decision?.category,
			info?.decision?.group_name,
		);
	} catch {
		render('', '', '');
	}
})();