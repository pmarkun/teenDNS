// rules.js — compile the policy decision order into declarativeNetRequest
// session rules. The policy engine selects the longest matching domain; ties
// keep the first explicit rule, then the first group/domain, so priorities here
// encode both specificity and source order.

import {
	ACTION_ALLOW,
	ACTION_BLOCK,
	ACTION_OBSERVE,
	windowActive,
	activeScheduledAction,
} from './decision.js';

export const BLOCKED_PATH = '/blocked.html';

const RESOURCE_TYPES = ['main_frame'];
const DEFAULT_BLOCK_PRIORITY = 1;
const PAUSE_PRIORITY = 2_147_483_647;

// effectiveGroupAction applies the first active scheduled action, if any.
export function effectiveGroupAction(group, parts) {
	const schedule = activeScheduledAction(group.schedules, parts);
	return schedule ? schedule.action : group.action;
}

// pauseActive reports whether any global pause window is currently active.
export function pauseActive(pauses, parts) {
	for (const pause of pauses || []) {
		if (windowActive(pause, parts)) {
			return pause;
		}
	}
	return null;
}

export function blockedPath() {
	return BLOCKED_PATH;
}

function escapeRegex(value) {
	return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function conditionForDomain(domain, includeSubdomains) {
	const condition = { resourceTypes: RESOURCE_TYPES };
	if (includeSubdomains) {
		condition.requestDomains = [domain];
	} else {
		// requestDomains alone includes subdomains. Keep it as a host filter and
		// intersect it with an anchored URL regex for exact-host semantics.
		condition.requestDomains = [domain];
		condition.regexFilter = `^https?://(?:[^/?#@]*@)?${escapeRegex(domain)}(?::[0-9]+)?(?:/|$)`;
	}
	return condition;
}

function actionForPolicy(action) {
	if (action === ACTION_BLOCK) {
		return {
			type: 'redirect',
			redirect: { extensionPath: BLOCKED_PATH },
		};
	}
	if (action === ACTION_ALLOW || action === ACTION_OBSERVE) {
		// An allow candidate is needed both for default-block profiles and to
		// suppress a less-specific redirect when allow/observe wins by specificity.
		return { type: 'allow' };
	}
	return null;
}

function candidatePriority(domainLength, sourceOrder, sourceCount) {
	const stride = sourceCount + 1;
	// Each character of specificity outweighs every possible order tie-break.
	// Policy inputs are far below the signed 32-bit DNR priority ceiling; keep a
	// guard so malformed/oversized policy cannot silently invert ordering.
	const priority = 2 + domainLength * stride + (stride - sourceOrder);
	if (!Number.isSafeInteger(priority) || priority >= PAUSE_PRIORITY) {
		throw new RangeError('policy is too large to compile into DNR priorities');
	}
	return priority;
}

function addCandidateRule(output, candidate, sourceCount, extensionId) {
	const action = actionForPolicy(candidate.action);
	if (!action) {
		return;
	}
	const condition = candidate.domains
		? {
			resourceTypes: RESOURCE_TYPES,
			requestDomains: candidate.domains,
		}
		: conditionForDomain(candidate.domain, candidate.includeSubdomains);
	if (extensionId) {
		condition.excludedRequestDomains = [extensionId];
	}
	output.push({
		id: output.length + 1,
		priority: candidatePriority(candidate.domainLength, candidate.sourceOrder, sourceCount),
		action,
		condition,
	});
}

// buildRules returns the session rules matching the policy's blocking surface
// at `parts`. Group domains with the same length into one DNR rule: they have
// identical specificity and group action, while keeping catalog lists compact.
export function buildRules(snapshot, parts, extensionId) {
	const rules = [];
	const pause = pauseActive(snapshot.pauses, parts);
	if (pause) {
		return [{
			id: 1,
			priority: PAUSE_PRIORITY,
			action: { type: 'redirect', redirect: { extensionPath: BLOCKED_PATH } },
			condition: {
				resourceTypes: RESOURCE_TYPES,
				...((extensionId && { excludedRequestDomains: [extensionId] }) || {}),
			},
		}];
	}

	if (snapshot.default_action === ACTION_BLOCK) {
		rules.push({
			id: rules.length + 1,
			priority: DEFAULT_BLOCK_PRIORITY,
			action: { type: 'redirect', redirect: { extensionPath: BLOCKED_PATH } },
			condition: {
				resourceTypes: RESOURCE_TYPES,
				...((extensionId && { excludedRequestDomains: [extensionId] }) || {}),
			},
		});
	}

	const entries = [];
	for (const [index, rule] of (snapshot.rules || []).entries()) {
		entries.push({
			domain: rule.domain,
			domainLength: String(rule.domain || '').length,
			includeSubdomains: Boolean(rule.include_subdomains),
			action: rule.action,
			sourceOrder: index,
		});
	}

	const groupCount = (snapshot.groups || []).length;
	for (const [groupIndex, group] of (snapshot.groups || []).entries()) {
		const action = effectiveGroupAction(group, parts);
		const buckets = new Map();
		for (const domain of group.domains || []) {
			const length = String(domain).length;
			if (!buckets.has(length)) buckets.set(length, []);
			buckets.get(length).push(domain);
		}
		for (const [domainLength, domains] of buckets) {
			entries.push({
				domains,
				domainLength,
				action,
				sourceOrder: (snapshot.rules || []).length + groupIndex,
			});
		}
	}

	const sourceCount = (snapshot.rules || []).length + groupCount;
	for (const candidate of entries) {
		addCandidateRule(rules, candidate, sourceCount, extensionId);
	}
	return rules;
}

// The signature covers the actual generated DNR conditions and actions, so it
// changes with default action, policy edits, and scheduled effective actions.
export function ruleSignature(snapshot, parts) {
	return JSON.stringify(buildRules(snapshot, parts));
}
