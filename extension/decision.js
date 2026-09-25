// decision.js — mirror of internal/policy.DecideAt in pure JavaScript.
//
// This module is intentionally free of chrome.* APIs so it can be unit-tested
// with Node. The extension host (background service worker) supplies the local
// clock parts via localClock() so timezone handling stays here too, keeping
// DNS and extension decisions aligned on the same schedule semantics.

export const ACTION_ALLOW = 'allow';
export const ACTION_BLOCK = 'block';
export const ACTION_OBSERVE = 'observe';

export const DEFAULT_TIME_ZONE = 'America/Sao_Paulo';

const WEEKDAY_INDEX = Object.freeze({
	sun: 0, mon: 1, tue: 2, wed: 3, thu: 4, fri: 5, sat: 6,
});

// parseMinute mirrors policy.parseMinute: strict "HH:MM", hour 00-23.
export function parseMinute(value) {
	if (typeof value !== 'string' || !/^\d{2}:\d{2}$/.test(value)) {
		return null;
	}
	const hour = Number(value.slice(0, 2));
	const minute = Number(value.slice(3, 5));
	if (hour > 23 || minute > 59) {
		return null;
	}
	return hour * 60 + minute;
}

// windowActive mirrors policy.windowActive. parts must be the result of
// localClock(): { weekday: 0-6 (Go convention, Sunday=0), hour, minute }.
export function windowActive(window, parts) {
	const start = parseMinute(window.start);
	const end = parseMinute(window.end);
	if (start === null || end === null) {
		return false;
	}
	const minute = parts.hour * 60 + parts.minute;
	const today = parts.weekday;
	const previousDay = (today + 6) % 7;
	for (const day of window.days || []) {
		if (day === today) {
			if (end > start && minute >= start && minute < end) {
				return true;
			}
			if (end < start && minute >= start) {
				return true;
			}
		}
		if (end < start && day === previousDay && minute < end) {
			return true;
		}
	}
	return false;
}

// activeScheduledAction mirrors policy.activeScheduledAction: the first
// scheduled action whose window is currently active, if any.
export function activeScheduledAction(schedules, parts) {
	for (const schedule of schedules || []) {
		if (windowActive(schedule, parts)) {
			return schedule;
		}
	}
	return null;
}

// normalizeHostname mirrors policy.normalizeName for a hostname.
export function normalizeHostname(value) {
	const name = String(value || '').toLowerCase().trim().replace(/\.$/, '');
	if (!name) {
		return null;
	}
	if (/[ /:@]/.test(name)) {
		return null;
	}
	return name;
}

// localClock resolves an instant in a profile's IANA time zone and returns the
// parts the engine needs. Returns null when the zone is unknown so callers can
// fall back to downgraded behavior instead of crashing.
export function localClock(timeZone, date = new Date()) {
	const zone = timeZone || DEFAULT_TIME_ZONE;
	let parts;
	try {
		parts = new Intl.DateTimeFormat('en-US', {
			timeZone: zone,
			weekday: 'short',
			hour: '2-digit',
			minute: '2-digit',
			hour12: false,
		}).formatToParts(date);
	} catch {
		return null;
	}
	const read = (type) => {
		const part = parts.find((item) => item.type === type);
		return part ? part.value : '';
	};
	const weekday = WEEKDAY_INDEX[read('weekday').toLowerCase()];
	if (weekday === undefined) {
		return null;
	}
	return {
		weekday,
		hour: Number(read('hour')),
		minute: Number(read('minute')),
		zone,
	};
}

// decisionSnapshot groups the fields of the policy endpoint response that the
// engine consumes. Callers may pass the full response object.
function toSnapshot(policy) {
	return {
		default_action: policy.default_action ?? ACTION_ALLOW,
		version: policy.version ?? 0,
		pauses: policy.pauses || [],
		rules: policy.rules || [],
		groups: policy.groups || [],
	};
}

// decideAt mirrors policy.DecideAt. parts comes from localClock().
// Returns { action, category, reason, matched_domain, group_name,
// schedule_label, policy_version, matching }.
export function decideAt(policy, hostname, parts) {
	const name = normalizeHostname(hostname);
	const snapshot = toSnapshot(policy);
	const decision = {
		action: snapshot.default_action,
		category: '',
		reason: '',
		matched_domain: null,
		group_name: null,
		schedule_label: null,
		policy_version: snapshot.version,
		matching: name !== null,
	};
	if (name === null) {
		decision.matching = false;
		return decision;
	}

	for (const pause of snapshot.pauses) {
		if (windowActive(pause, parts)) {
			decision.action = ACTION_BLOCK;
			decision.category = 'global_pause';
			decision.reason = 'Pausa geral';
			decision.schedule_label = pause.label || null;
			decision.matched_domain = null;
			return decision;
		}
	}

	let bestMatchLength = -1;

	for (const rule of snapshot.rules) {
		let matches = name === rule.domain;
		if (rule.include_subdomains) {
			matches = matches || name.endsWith('.' + rule.domain);
		}
		if (!matches || String(rule.domain).length <= bestMatchLength) {
			continue;
		}
		bestMatchLength = rule.domain.length;
		decision.action = rule.action;
		decision.category = rule.category || '';
		decision.reason = rule.reason || '';
		decision.matched_domain = rule.domain;
		decision.schedule_label = null;
	}

	for (const group of snapshot.groups) {
		for (const domain of group.domains || []) {
			const matches = name === domain || name.endsWith('.' + domain);
			if (!matches || String(domain).length <= bestMatchLength) {
				continue;
			}
			bestMatchLength = domain.length;
			decision.action = group.action;
			decision.category = group.category || '';
			decision.reason = group.reason || '';
			decision.matched_domain = domain;
			decision.group_name = group.name || null;
			const schedule = activeScheduledAction(group.schedules, parts);
			if (schedule) {
				decision.action = schedule.action;
				decision.schedule_label = schedule.label || null;
			}
		}
	}

	return decision;
}