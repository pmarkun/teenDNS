// background.js — teenDNS MV3 service worker.
//
// Owns the paired policy snapshot (chrome.storage), the declarativeNetRequest
// session rules rebuilt on schedule boundaries, and the per-tab badge showing
// the DNS-equivalent decision (blocked/observed/allowed). No telemetry.

import {
	ACTION_ALLOW,
	ACTION_BLOCK,
	ACTION_OBSERVE,
	DEFAULT_TIME_ZONE,
	decideAt,
	localClock,
	normalizeHostname,
} from './decision.js';
import { buildRules, ruleSignature } from './rules.js';

const STORAGE_KEY = 'state';
const ALARM_RULES = 'teenRules';
const ALARM_REFRESH = 'teenRefresh';
const RULES_TICK_MS = 60 * 1000;
const REFRESH_MS = 20 * 60 * 1000;
const SESSION_MARGIN_MS = 60 * 1000;
const DEFAULT_BASE_URL = 'https://teendns.lab.markun.com.br';
const PAIR_POLL_MS = 1000;
const PAIR_POLL_ATTEMPTS = 45;

const BADGE_COLORS = Object.freeze({
	[ACTION_BLOCK]: { color: '#f32b20', text: '#ffffff' },
	[ACTION_OBSERVE]: { color: '#1748e8', text: '#ffffff' },
	[ACTION_ALLOW]: { color: '#adff19', text: '#0b0b0a' },
});

const defaultState = () => ({ baseUrl: DEFAULT_BASE_URL });

// recentNavigation remembers only the most recent in-flight URL per tab. It is
// kept in memory and chrome.storage.session (cleared when Chrome exits) so a
// worker restart between redirect and page load does not lose the explanation.
const recentNavigation = new Map();

function navigationKey(tabId) {
	return `recentNavigation:${tabId}`;
}

async function getState() {
	const { [STORAGE_KEY]: state } = await chrome.storage.local.get(STORAGE_KEY);
	return { ...defaultState(), ...(state || {}) };
}

function setState(patch) {
	return chrome.storage.local.set({ [STORAGE_KEY]: patch });
}

function apiURL(baseUrl, path) {
	return new URL(path, baseUrl.replace(/\/$/, '') + '/').toString();
}

async function apiFetch(baseUrl, path, init = {}) {
	const response = await fetch(apiURL(baseUrl, path), {
		method: init.method || 'GET',
		headers: { 'content-type': 'application/json', ...(init.headers || {}) },
		body: init.body,
	});
	if (!response.ok) {
		throw new Error(`API ${response.status}`);
	}
	return response.json();
}

// probeChallenge triggers a real DNS lookup of the challenge host through the
// service worker's network stack — the same width the panel uses on its own
// device. mode 'no-cors' keeps the probe read-only.
function probeChallenge(dnsName) {
	return fetch(`https://${dnsName}/parear.gif?t=${Date.now()}`, {
		mode: 'no-cors',
		cache: 'no-store',
	}).catch(() => {});
}

async function wait(ms) {
	return new Promise((resolve) => setTimeout(resolve, ms));
}

// pairFlow repeats the panel's youth pairing: create a challenge, resolve its
// host, poll until the gateway observes it and hands back a session token.
async function pairFlow(baseUrl) {
	const challenge = await apiFetch(baseUrl, '/api/v1/pairing/challenges', { method: 'POST' });
	await probeChallenge(challenge.dns_name);
	for (let attempt = 0; attempt < PAIR_POLL_ATTEMPTS; attempt += 1) {
		await wait(PAIR_POLL_MS);
		const status = await apiFetch(baseUrl, `/api/v1/pairing/challenges/${challenge.id}`);
		if (status.paired && status.session_token) {
			return {
				sessionToken: status.session_token,
				expiresAt: status.expires_at || null,
			};
		}
	}
	throw new Error('O DNS não respondeu o desafio de pareamento.');
}

async function fetchSnapshot(baseUrl, sessionToken) {
	const snapshot = await apiFetch(baseUrl, '/api/v1/extension/policy', {
		headers: { authorization: `Bearer ${sessionToken}` },
	});
	if (!Array.isArray(snapshot.rules) || !Array.isArray(snapshot.groups)) {
		throw new Error('Resposta de política inválida.');
	}
	return {
		...snapshot,
		time_zone: snapshot.time_zone || DEFAULT_TIME_ZONE,
	};
}

// ensureFreshSnapshot returns a valid session token + snapshot, pairing again
// when the stored session expired (the token lasts an hour server-side).
// Pairing is only attempted when shouldPair, so background renewals do not
// spawn challenges before the user has ever paired.
async function ensureFreshSnapshot(shouldPair) {
	const state = await getState();
	let token = state.sessionToken;
	let expiresAt = state.sessionExpiresAt;
	if (token && expiresAt && Date.now() < new Date(expiresAt).getTime() - SESSION_MARGIN_MS) {
		const snapshot = await fetchSnapshot(state.baseUrl, token);
		return { snapshot, token, expiresAt };
	}
	if (!shouldPair) {
		throw new Error('Nenhum perfil pareado.');
	}
	const paired = await pairFlow(state.baseUrl);
	token = paired.sessionToken;
	expiresAt = paired.expiresAt;
	const snapshot = await fetchSnapshot(state.baseUrl, token);
	return { snapshot, token, expiresAt };
}

async function applySnapshot({ snapshot, token, expiresAt }) {
	const state = await getState();
	await setState({
		...state,
		sessionToken: token,
		sessionExpiresAt: expiresAt,
		paired: true,
		snapshot,
		syncedAt: new Date().toISOString(),
		lastError: null,
	});
	return syncFromStorage();
}

async function syncNow(allowPair = false) {
	const state = await getState();
	if (!state.sessionToken && !state.paired && !allowPair) {
		return { ok: false, quiet: true };
	}
	try {
		const fresh = await ensureFreshSnapshot(allowPair || state.paired);
		await applySnapshot(fresh);
		return { ok: true, syncedAt: new Date().toISOString() };
	} catch (error) {
		await setState({ ...state, lastError: String(error?.message || error) });
		return { ok: false, error: String(error?.message || error) };
	}
}

async function currentSessionRuleIds() {
	const rules = await chrome.declarativeNetRequest.getSessionRules();
	return rules.map((rule) => rule.id);
}

let ruleUpdateQueue = Promise.resolve();

// recomputeRules rebuilds session rules whenever the effective block surface
// changes (schedule tick or new snapshot). Serialize updates: a storage change
// and the caller that wrote it can both request reconciliation at once.
function recomputeRules() {
	const update = ruleUpdateQueue.catch(() => {}).then(recomputeRulesNow);
	ruleUpdateQueue = update;
	return update;
}

async function recomputeRulesNow() {
	const state = await getState();
	const snapshot = state.snapshot;
	if (!snapshot) {
		await chrome.declarativeNetRequest.updateSessionRules({
			removeRuleIds: await currentSessionRuleIds(),
		});
		if (state.rulesSig !== null) {
			await setState({ ...state, rulesSig: null });
		}
		return;
	}
	const details = localClock(snapshot.time_zone);
	if (!details) {
		return;
	}
	const signature = ruleSignature(snapshot, details);
	if (state.rulesSig === signature) {
		return;
	}
	const rules = buildRules(snapshot, details, chrome.runtime.id);
	await chrome.declarativeNetRequest.updateSessionRules({
		removeRuleIds: await currentSessionRuleIds(),
		addRules: rules,
	});
	const next = await getState();
	await setState({ ...next, rulesSig: signature });
}

// ---- per-tab badge -------------------------------------------------------

function decisionForUrl(snapshot, url, at = new Date()) {
	if (!url) {
		return null;
	}
	const parsed = new URL(url);
	if (parsed.protocol === 'chrome-extension:') {
		if (parsed.pathname.endsWith('/blocked.html')) {
			return {
				action: ACTION_BLOCK,
				category: parsed.searchParams.get('category') || '',
				reason: parsed.searchParams.get('reason') || '',
				group_name: parsed.searchParams.get('group') || null,
				source: 'blocked-page',
			};
		}
		return null;
	}
	const hostname = normalizeHostname(parsed.hostname);
	if (!hostname) {
		return null;
	}
	return decideAt(snapshot, hostname, localClock(snapshot.time_zone, at));
}

function isBlockedPageURL(url) {
	return /^chrome-extension:\/\/[^/]+\/blocked\.html(\?|#|$)/.test(url);
}

function applyTabBadge(tabId, url) {
	return chrome.storage.local.get(STORAGE_KEY).then(async ({ [STORAGE_KEY]: state }) => {
		const snapshot = state?.snapshot;
		const decision = snapshot ? decisionForUrl(snapshot, url) : null;
		const style = decision && BADGE_COLORS[decision.action];
		if (!style) {
			await chrome.action.setBadgeText({ tabId, text: '' });
			return;
		}
		await Promise.all([
			chrome.action.setBadgeText({ tabId, text: '\u25cf' }),
			chrome.action.setBadgeBackgroundColor({ tabId, color: style.color }),
			chrome.action.setBadgeTextColor({ tabId, color: style.text }),
		]);
		const title = badgeTitle(snapshot, decision);
		await chrome.action.setTitle({ tabId, title });
	});
}

function badgeTitle(snapshot, decision) {
	const base = `teenDNS · ${snapshot?.label || snapshot?.fingerprint || 'perfil'}`;
	switch (decision.action) {
		case ACTION_BLOCK:
			return `${base}\nBloqueado — ${decision.reason || decision.group_name || decision.category || 'protegido pelo perfil'}`;
		case ACTION_OBSERVE:
			return `${base}\nObservado`;
		default:
			return `${base}\nLiberado`;
	}
}

async function refreshTabById(tabId) {
	const tab = await chrome.tabs.get(tabId).catch(() => null);
	if (tab) {
		await applyTabBadge(tab.id, tab.url);
	}
}

async function refreshAllTabs() {
	const tabs = await chrome.tabs.query({});
	await Promise.all(tabs.map((tab) => applyTabBadge(tab.id, tab.url)));
}

// syncFromStorage reconciles after storage changes land (popup pairing, etc.).
async function syncFromStorage() {
	await Promise.all([recomputeRules(), refreshAllTabs()]);
}

async function bootstrap() {
	await chrome.alarms.create(ALARM_RULES, { periodInMinutes: RULES_TICK_MS / 60000 });
	await chrome.alarms.create(ALARM_REFRESH, { periodInMinutes: REFRESH_MS / 60000 });
	let state = await getState();
	// DNR session rules are cleared when Chrome starts. The persisted signature
	// must not suppress their reconstruction from the cached policy snapshot.
	if (state.rulesSig) {
		state = { ...state, rulesSig: null };
		await setState(state);
	}
	if (state.paired) {
		await syncNow();
	} else {
		await syncFromStorage();
	}
}

chrome.runtime.onInstalled.addListener(() => {
	void bootstrap();
});

chrome.runtime.onStartup.addListener(() => {
	void bootstrap();
});

chrome.alarms.onAlarm.addListener((alarm) => {
	if (alarm.name === ALARM_RULES) {
		void recomputeRules();
	} else if (alarm.name === ALARM_REFRESH) {
		void syncNow();
	}
});

chrome.storage.onChanged.addListener((changes, area) => {
	if (area === 'local' && changes[STORAGE_KEY]) {
		void syncFromStorage();
	}
});

chrome.tabs.onUpdated.addListener((tabId, changeInfo, tab) => {
	if (changeInfo.url) {
		const pendingURL = changeInfo.url;
		if (!isBlockedPageURL(pendingURL)) {
			const navigation = { url: pendingURL, at: Date.now() };
			recentNavigation.set(tabId, navigation);
			void chrome.storage.session.set({ [navigationKey(tabId)]: navigation });
		}
	}
	if (changeInfo.status === 'complete' && !isBlockedPageURL(tab.url || '')) {
		recentNavigation.delete(tabId);
		void chrome.storage.session.remove(navigationKey(tabId));
	}
	if (changeInfo.url || changeInfo.status === 'loading') {
		void refreshTabById(tabId);
	}
});
chrome.tabs.onActivated.addListener(({ tabId }) => {
	void refreshTabById(tabId);
});
chrome.tabs.onReplaced.addListener((addedTabId) => {
	void refreshTabById(addedTabId);
});
chrome.tabs.onRemoved.addListener((removedTabId) => {
	recentNavigation.delete(removedTabId);
	void chrome.storage.session.remove(navigationKey(removedTabId));
});

async function unpair() {
	const state = await getState();
	await setState({
		...state,
		sessionToken: null,
		sessionExpiresAt: null,
		paired: false,
		snapshot: null,
		rulesSig: null,
		lastError: null,
	});
	await syncFromStorage();
}

chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
	(async () => {
		switch (message?.command) {
			case 'getState': {
				const state = await getState();
				sendResponse({ ok: true, state });
				return;
			}
			case 'pair': {
				await syncNow(true);
				const state = await getState();
				sendResponse({ ok: !!state.snapshot, error: state.lastError || null });
				return;
			}
			case 'syncNow': {
				const result = await syncNow(true);
				sendResponse(result);
				return;
			}
			case 'setBaseUrl': {
				const url = String(message.baseUrl || '').trim();
				if (!/^https:\/\/.+/.test(url)) {
					sendResponse({ ok: false, error: 'URL precisa começar com https://' });
					return;
				}
				const state = await getState();
				await setState({
					...state,
					baseUrl: url.replace(/\/+$/, ''),
					sessionToken: null,
					sessionExpiresAt: null,
					paired: false,
					snapshot: null,
					rulesSig: null,
					lastError: null,
				});
				await syncFromStorage();
				sendResponse({ ok: true });
				return;
			}
			case 'unpair': {
				await unpair();
				sendResponse({ ok: true });
				return;
			}
			case 'blockedInfo': {
				const tabId = message?.tabId;
				let navigation = recentNavigation.get(tabId);
				if (!navigation && Number.isInteger(tabId)) {
					const stored = await chrome.storage.session.get(navigationKey(tabId));
					navigation = stored[navigationKey(tabId)];
				}
				const state = await getState();
				const freshNavigation = navigation && Date.now() - navigation.at < 30_000;
				const decision = freshNavigation && state.snapshot
					? decisionForUrl(state.snapshot, navigation.url, new Date(navigation.at))
					: null;
				recentNavigation.delete(tabId);
				if (Number.isInteger(tabId)) {
					await chrome.storage.session.remove(navigationKey(tabId));
				}
				sendResponse({
					ok: true,
					decision: decision?.action === ACTION_BLOCK ? decision : null,
					originalUrl: decision?.action === ACTION_BLOCK ? navigation.url : null,
				});
				return;
			}
			default:
				sendResponse({ ok: false, error: 'comando desconhecido' });
		}
	})();
	return true;
});
