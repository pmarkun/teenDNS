// decision.test.mjs — runs the decision engine against the policy semantics
// exercised by internal/policy/policy_test.go.
import test from 'node:test';
import assert from 'node:assert/strict';
import {
	ACTION_ALLOW,
	ACTION_BLOCK,
	ACTION_OBSERVE,
	localClock,
	parseMinute,
	windowActive,
	decideAt,
	normalizeHostname,
} from './decision.js';
import { buildRules, ruleSignature, effectiveGroupAction, blockedPath } from './rules.js';

function parts(hour, minute, weekday = 2) {
	return { weekday, hour, minute };
}

const snapshot = {
	label: 'Casa',
	fingerprint: 'p-home',
	default_action: ACTION_ALLOW,
	version: 3,
	time_zone: 'America/Sao_Paulo',
	rules: [],
	groups: [],
	pauses: [],
};

test('parseMinute accepts strict HH:MM and rejects loose formats', () => {
	assert.equal(parseMinute('23:59'), 23 * 60 + 59);
	assert.equal(parseMinute('00:00'), 0);
	assert.equal(parseMinute('8:00'), null);
	assert.equal(parseMinute('24:00'), null);
	assert.equal(parseMinute('15:60'), null);
	assert.equal(parseMinute('not'), null);
});

test('windowActive honors overnight windows and previous-day rollover', () => {
	const overnight = { days: [2], start: '22:00', end: '06:00' };
	assert.ok(windowActive(overnight, parts(23, 0, 2)));       // same day after start
	assert.ok(windowActive(overnight, parts(5, 59, 3)));        // next day before end
	assert.ok(!windowActive(overnight, parts(6, 0, 3)));        // exactly at end
	assert.ok(!windowActive(overnight, parts(21, 59, 2)));      // before start
	const daytime = { days: [1], start: '08:00', end: '18:00' };
	assert.ok(windowActive(daytime, parts(12, 0, 1)));
	assert.ok(!windowActive(daytime, parts(12, 0, 2)));
	assert.ok(!windowActive(daytime, parts(18, 0, 1)));
});

test('decideAt defaults to the profile default action', () => {
	assert.equal(decideAt(snapshot, 'example.com', parts(10, 0)).action, ACTION_ALLOW);
});

test('decideAt picks the longest matching rule', () => {
	const policy = {
		...snapshot,
		rules: [
			{ domain: 'example.com', action: ACTION_OBSERVE },
			{ domain: 'deep.example.com', include_subdomains: true, action: ACTION_BLOCK, category: 'porno', reason: 'Conteúdo adulto' },
		],
	};
	const decision = decideAt(policy, 'x.deep.example.com', parts(10, 0));
	assert.equal(decision.action, ACTION_BLOCK);
	assert.equal(decision.category, 'porno');
	assert.equal(decision.matched_domain, 'deep.example.com');
});

test('decideAt honors exact vs subdomain semantics', () => {
	const policy = {
		...snapshot,
		rules: [{ domain: 'example.com', include_subdomains: false, action: ACTION_BLOCK, reason: 'exato' }],
	};
	assert.equal(decideAt(policy, 'example.com', parts(10, 0)).action, ACTION_BLOCK);
	assert.equal(decideAt(policy, 'www.example.com', parts(10, 0)).action, ACTION_ALLOW);
});

test('decideAt applies group blocks with subdomains', () => {
	const policy = {
		...snapshot,
		groups: [{
			id: 'g1', name: 'Milho', action: ACTION_BLOCK, category: 'jogos', reason: 'Sem apostas',
			domains: ['bet.test'],
		}],
	};
	const decision = decideAt(policy, 'm.bet.test', parts(10, 0));
	assert.equal(decision.action, ACTION_BLOCK);
	assert.equal(decision.group_name, 'Milho');
	assert.equal(decision.matched_domain, 'bet.test');
});

test('decideAt lets an active schedule override a group action', () => {
	const policy = {
		...snapshot,
		groups: [{
			id: 'g1', name: 'Redes', action: ACTION_BLOCK, category: 'redes', reason: 'Hora de estudar',
			domains: ['social.test'],
			schedules: [{
				id: 'w', label: 'Final de semana', days: [6], start: '08:00', end: '22:00', action: ACTION_ALLOW,
			}],
		}],
	};
	const saturdayMorning = parts(9, 30, 6);
	assert.equal(decideAt(policy, 'social.test', saturdayMorning).action, ACTION_ALLOW);
	assert.equal(decideAt(policy, 'social.test', saturdayMorning).schedule_label, 'Final de semana');
	const monday = parts(9, 30, 1);
	assert.equal(decideAt(policy, 'social.test', monday).action, ACTION_BLOCK);
});

test('decideAt blocks everything during a global pause', () => {
	const policy = {
		...snapshot,
		pauses: [{ id: 'p1', label: 'Estudos', days: [2], start: '18:00', end: '21:00' }],
		rules: [{ domain: 'example.com', action: ACTION_ALLOW }],
	};
	const pauseActive = parts(19, 0, 2);
	const decision = decideAt(policy, 'example.com', pauseActive);
	assert.equal(decision.action, ACTION_BLOCK);
	assert.equal(decision.category, 'global_pause');
	assert.equal(decision.reason, 'Pausa geral');
	assert.equal(decision.schedule_label, 'Estudos');
});

test('localClock resolves parts in a profile time zone', () => {
	const clock = localClock('America/Sao_Paulo', new Date('2026-01-01T12:00:00Z'));
	assert.ok(clock.hour >= 0 && clock.hour <= 23);
	assert.ok(clock.weekday >= 0 && clock.weekday <= 6);
	assert.equal(clock.zone, 'America/Sao_Paulo');
});

test('localClock returns null for an unknown time zone', () => {
	assert.equal(localClock('Mars/Olympus', new Date()), null);
});

test('normalizeHostname mirrors Go normalization', () => {
	assert.equal(normalizeHostname('Example.COM.'), 'example.com');
	assert.equal(normalizeHostname('  example.com '), 'example.com');
	assert.equal(normalizeHostname(''), null);
	assert.equal(normalizeHostname('bad host'), null);
});

test('buildRules skips groups whose schedule currently allows', () => {
	const policy = {
		...snapshot,
		version: 9,
		groups: [{
			id: 'g1', name: 'Redes', action: ACTION_BLOCK, category: 'redes', reason: 'bloqueado',
			domains: ['social.test'],
			schedules: [{
				id: 'w', label: 'Fim de semana', days: [6], start: '08:00', end: '22:00', action: ACTION_ALLOW,
			}],
		}],
	};
	const rules = buildRules(policy, parts(9, 0, 6), 'extid');
	const allowed = rules.find((rule) => rule.condition?.requestDomains?.includes('social.test'));
	assert.equal(allowed.action.type, 'allow');
	const monday = parts(9, 0, 1);
	const blocked = buildRules(policy, monday, 'extid');
	const match = blocked.find((rule) => rule.condition?.requestDomains?.includes('social.test'));
	assert.ok(match, 'expected a redirect rule while schedule is inactive');
	assert.deepEqual(match.action.redirect, { extensionPath: '/blocked.html' });
});

test('blockedPath points to the documented extensionPath target', () => {
	assert.equal(blockedPath(), '/blocked.html');
});

test('buildRules produces a pause catch-all redirect when pausa ativa', () => {
	const policy = {
		...snapshot,
		pauses: [{ id: 'p1', label: 'Estudos', days: [2], start: '18:00', end: '21:00' }],
	};
	const rules = buildRules(policy, parts(19, 0, 2), 'extid');
	assert.equal(rules.length, 1);
	assert.equal(rules[0].priority, 2_147_483_647);
	assert.deepEqual(rules[0].action.redirect, { extensionPath: '/blocked.html' });
	assert.deepEqual(rules[0].condition.resourceTypes, ['main_frame']);
});

test('same-domain explicit allow wins over a group by policy order', () => {
	const policy = {
		...snapshot,
		groups: [{
			id: 'g1', name: 'Jogos', action: ACTION_BLOCK, domains: ['bet.test'],
		}],
		rules: [{ domain: 'bet.test', include_subdomains: true, action: ACTION_ALLOW }],
	};
	const rules = buildRules(policy, parts(10, 0), 'extid');
	const allowRule = rules.find((rule) => rule.action.type === 'allow');
	const redirectRule = rules.find((rule) => rule.action.type === 'redirect');
	assert.ok(allowRule.priority > redirectRule.priority, 'allow must outrank redirect');
});

test('buildRules redirects by default when default_action is block', () => {
	const policy = { ...snapshot, default_action: ACTION_BLOCK };
	const rules = buildRules(policy, parts(10, 0), 'extid');
	assert.equal(rules.length, 1);
	assert.equal(rules[0].priority, 1);
	assert.equal(rules[0].action.type, 'redirect');
	assert.deepEqual(rules[0].condition.resourceTypes, ['main_frame']);
});

test('buildRules emits allow exceptions for allow and observe groups', () => {
	const policy = {
		...snapshot,
		default_action: ACTION_BLOCK,
		groups: [
			{ id: 'allow', name: 'Permitido', action: ACTION_ALLOW, domains: ['allow.test'] },
			{ id: 'observe', name: 'Observar', action: ACTION_OBSERVE, domains: ['observe.test'] },
		],
	};
	const rules = buildRules(policy, parts(10, 0), 'extid');
	for (const domain of ['allow.test', 'observe.test']) {
		const exception = rules.find((rule) => rule.condition?.requestDomains?.includes(domain));
		assert.ok(exception, `missing exception for ${domain}`);
		assert.equal(exception.action.type, 'allow');
		assert.ok(exception.priority > 1, 'policy exceptions must beat the default-block rule');
	}
});

test('buildRules preserves longest-domain precedence over action type', () => {
	const policy = {
		...snapshot,
		rules: [{ domain: 'example.test', include_subdomains: true, action: ACTION_ALLOW }],
		groups: [{ id: 'child', name: 'Bloqueado', action: ACTION_BLOCK, domains: ['deep.example.test'] }],
	};
	const rules = buildRules(policy, parts(10, 0), 'extid');
	const broadAllow = rules.find((rule) => rule.action.type === 'allow');
	const specificBlock = rules.find((rule) => rule.action.type === 'redirect');
	assert.ok(specificBlock.priority > broadAllow.priority);
});

test('buildRules preserves DNS tie order: explicit rule wins over group', () => {
	const policy = {
		...snapshot,
		rules: [{ domain: 'same.test', action: ACTION_BLOCK }],
		groups: [{ id: 'same', name: 'Permitido', action: ACTION_ALLOW, domains: ['same.test'] }],
	};
	const rules = buildRules(policy, parts(10, 0), 'extid');
	const explicitBlock = rules.find((rule) => rule.action.type === 'redirect');
	const groupAllow = rules.find((rule) => rule.action.type === 'allow');
	assert.ok(explicitBlock.priority > groupAllow.priority);
});

test('exact domain rules use a host-anchored condition, not a domain suffix filter', () => {
	const policy = {
		...snapshot,
		rules: [{ domain: 'exact.test', include_subdomains: false, action: ACTION_BLOCK }],
	};
	const [rule] = buildRules(policy, parts(10, 0), 'extid');
	assert.deepEqual(rule.condition.requestDomains, ['exact.test']);
	assert.equal(rule.condition.regexFilter, '^https?://(?:[^/?#@]*@)?exact\\.test(?::[0-9]+)?(?:/|$)');
	assert.ok(!Object.hasOwn(rule.condition, 'urlFilter'));
	const exactHost = new RegExp(rule.condition.regexFilter);
	assert.ok(exactHost.test('https://exact.test/path'));
	assert.ok(exactHost.test('http://exact.test:8443/'));
	assert.ok(exactHost.test('https://user:pass@exact.test/path'));
	assert.ok(!exactHost.test('https://www.exact.test/path'));
	assert.ok(!exactHost.test('https://exact.test.evil/path'));
});

test('ruleSignature changes across schedule boundaries', () => {
	const group = {
		id: 'g1', name: 'Redes', action: ACTION_BLOCK, domains: ['social.test'],
		schedules: [{ id: 'w', label: 'Fim de semana', days: [6], start: '08:00', end: '22:00', action: ACTION_ALLOW }],
	};
	const policy = { ...snapshot, version: 4, groups: [group] };
	const saturday = parts(9, 0, 6);
	const monday = parts(9, 0, 1);
	assert.notEqual(ruleSignature(policy, saturday), ruleSignature(policy, monday));
	assert.equal(ruleSignature(policy, saturday), ruleSignature(policy, saturday));
});

test('ruleSignature changes when the default action changes', () => {
	const allow = { ...snapshot, default_action: ACTION_ALLOW };
	const block = { ...snapshot, default_action: ACTION_BLOCK };
	assert.notEqual(ruleSignature(allow, parts(10, 0)), ruleSignature(block, parts(10, 0)));
});

test('effectiveGroupAction applies scheduled override', () => {
	const group = {
		id: 'g1', action: ACTION_BLOCK,
		schedules: [{ id: 'w', label: 'w', days: [0], start: '00:00', end: '23:59', action: ACTION_OBSERVE }],
	};
	assert.equal(effectiveGroupAction(group, parts(5, 0, 0)), ACTION_OBSERVE);
	assert.equal(effectiveGroupAction(group, parts(5, 0, 3)), ACTION_BLOCK);
});
