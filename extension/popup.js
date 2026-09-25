// popup.js — renders the extension state and drives pairing from the worker.
// One-click flow mirrors the panel youth screen: a challenge on the panel API
// plus a DNS probe answered by the house resolver, then a policy snapshot.

const root = document.getElementById('root');

const apiState = () => chrome.runtime.sendMessage({ command: 'getState' });
const pair = () => chrome.runtime.sendMessage({ command: 'pair' });
const syncNow = () => chrome.runtime.sendMessage({ command: 'syncNow' });
const setBaseUrl = (baseUrl) => chrome.runtime.sendMessage({ command: 'setBaseUrl', baseUrl });
const unpair = () => chrome.runtime.sendMessage({ command: 'unpair' });

function el(tag, className, text) {
	const node = document.createElement(tag);
	if (className) node.className = className;
	if (text) node.textContent = text;
	return node;
}

async function render() {
	const { state } = await apiState();
	const snapshot = state.snapshot;
	root.replaceChildren(snapshot ? pairedView(state, snapshot) : unpairedView(state));
}

function pairedView(state, snapshot) {
	const section = el('section', 'view paired');
	section.append(el('p', 'kicker', 'PERFIL ' + String(snapshot.label || '').toUpperCase()));
	section.append(el('h1', 'title', 'O COMBINADO DA CASA\nESTÁ ATIVO AQUI.'));

	const meta = el('dl', 'meta');
	const blockedCount = (snapshot.groups || []).filter((group) => group.action === 'block').length;
	const row = (term, value) => {
		const item = el('div', 'meta-row');
		item.append(el('dt', '', term), el('dd', '', value));
		return item;
	};
	meta.append(
		row('perfil', snapshot.label || '—'),
		row('bloqueando', blockedCount + ' categorias'),
		row('última sincronia', state.syncedAt ? new Date(state.syncedAt).toLocaleString() : '—'),
		row('endpoint', snapshot.fingerprint || '—'),
		row('fusohorário', snapshot.time_zone || '—'),
	);
	section.append(meta);

	if (state.lastError && !state.syncing) {
		section.append(el('p', 'error', '⚠ ' + state.lastError));
	}

	section.append(el('p', 'hint', 'O ícone da extensão mostra a cor de cada aba: vermelho é bloqueado, azul é observado e limão é liberado.'));

	const actions = el('div', 'actions');
	const syncButton = el('button', 'button button--acid', 'CHECAR AGORA');
	syncButton.addEventListener('click', async () => {
		syncButton.disabled = true;
		syncButton.textContent = 'SINCRONIZANDO…';
		await syncNow();
		await render();
	});
	const unpairButton = el('button', 'button button--outline', 'DESVINCULAR');
	unpairButton.addEventListener('click', async () => {
		await unpair();
		await render();
	});
	actions.append(syncButton, unpairButton);
	section.append(actions);

	section.append(endpointField(state.baseUrl, false));
	return section;
}

function unpairedView(state) {
	const section = el('section', 'view unpaired');
	section.append(el('p', 'kicker', 'SEM LOGIN. SEM SENHA.'));
	section.append(el('h1', 'title', 'VINCULAR\nESTE NAVEGADOR'));
	section.append(el('p', 'copy', 'A extensão pergunta pro DNS da casa qual perfil está ativo neste aparelho e espelha a política localmente. Um clique só.'));

	const form = document.createElement('form');
	form.className = 'pair-form';
	form.append(endpointField(state.baseUrl, true));

	const button = el('button', 'button button--acid', 'VINCULAR PERFIL');
	button.type = 'submit';
	form.append(button);
	form.addEventListener('submit', async (event) => {
		event.preventDefault();
		button.disabled = true;
		button.textContent = 'PERGUNTANDO PRO DNS…';
		const input = form.querySelector('input');
		const base = input.value.trim();
		if (base !== state.baseUrl) {
			const baseResult = await setBaseUrl(base);
			if (!baseResult.ok) {
				button.disabled = false;
				button.textContent = 'VINCULAR PERFIL';
				showError(form, baseResult.error);
				return;
			}
		}
		const pairResult = await pair();
		if (!pairResult.ok) {
			button.disabled = false;
			button.textContent = 'VINCULAR PERFIL';
			showError(form, pairResult.error || 'Não foi possível vincular agora.');
			return;
		}
		await render();
	});
	section.append(form);

	if (state.lastError) {
		section.append(el('p', 'error', '⚠ ' + state.lastError));
	}

	section.append(el('p', 'hint', 'Importante: o aparelho precisa estar usando o endereço teenDNS da casa para o desafio ser respondido.'));
	return section;
}

function endpointField(value, editable) {
	const label = el('label', 'endpoint-field');
	label.append(el('small', '', 'endpoint teenDNS'));
	const input = document.createElement('input');
	input.value = value;
	if (editable) {
		label.append(input);
	} else {
		input.readOnly = true;
		label.append(input);
	}
	return label;
}

function showError(container, message) {
	container.querySelectorAll('.error').forEach((node) => node.remove());
	const error = el('p', 'error', '⚠ ' + message);
	container.before(error);
}

await render();
