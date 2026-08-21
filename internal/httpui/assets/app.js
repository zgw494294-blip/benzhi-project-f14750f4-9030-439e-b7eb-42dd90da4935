const state = {
  sessions: [], active: null, detail: null, summary: null,
  query: new URLSearchParams(location.search), page: 1, pages: 0,
  editDevice: null, editCue: null, batchValid: false,
};
const $ = (selector) => document.querySelector(selector);
const $$ = (selector) => [...document.querySelectorAll(selector)];
const sessionList = $('#session-list');
const content = $('#content');
const toast = $('#toast');

function escapeHTML(value) {
  return String(value ?? '').replace(/[&<>'"]/g, (character) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;' }[character]));
}
function showToast(message, error = false) {
  toast.textContent = message;
  toast.style.borderLeft = `4px solid ${error ? '#eb6b4f' : '#d6f85e'}`;
  toast.classList.add('show');
  setTimeout(() => toast.classList.remove('show'), 3200);
}
async function api(path, options = {}) {
  const response = await fetch(path, { headers: { 'Content-Type': 'application/json', ...(options.headers || {}) }, ...options });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) {
    const error = new Error(body.error?.message || '请求失败');
    error.details = body.error;
    throw error;
  }
  return body;
}
function key(prefix) { return `${prefix}-${crypto.randomUUID()}`; }
function meta() { return { expectedVersion: state.detail.session.version, idempotencyKey: key('web') }; }

function renderRiskSummary() {
  const summary = state.summary || { statusCounts: {}, pendingCueCount: 0, failedCueCount: 0, certificateCount: 0 };
  const activeStates = Object.entries(summary.statusCounts || {}).filter(([, count]) => count).map(([status, count]) => `${status} ${count}`).join(' · ');
  $('#risk-summary').innerHTML = `<div class="risk-grid"><span><strong>${summary.pendingCueCount}</strong>待执行</span><span><strong>${summary.failedCueCount}</strong>失败</span><span><strong>${summary.certificateCount}</strong>证书</span></div><small>${escapeHTML(activeStates || '当前筛选无会话')}</small>`;
}
function renderSessions() {
  if (!state.sessions.length) {
    sessionList.innerHTML = '<div class="empty-state">当前筛选没有验证会话</div>';
    return;
  }
  const items = state.sessions.map((session) => `<button class="session-item ${session.id === state.active ? 'active' : ''}" data-id="${escapeHTML(session.id)}"><span class="session-name">${escapeHTML(session.productionName)}</span><span class="session-meta"><span>${escapeHTML(session.venue)}</span><span>${escapeHTML(session.status)}</span></span><span class="risk-line">待办 ${session.cueCount - session.passedCount - session.failedCount} · 失败 ${session.failedCount}</span></button>`).join('');
  const paging = state.pages > 1 ? `<div class="pager"><button data-page="${state.page - 1}" ${state.page <= 1 ? 'disabled' : ''} title="上一页">←</button><span>${state.page} / ${state.pages}</span><button data-page="${state.page + 1}" ${state.page >= state.pages ? 'disabled' : ''} title="下一页">→</button></div>` : '';
  sessionList.innerHTML = items + paging;
  $$('.session-item').forEach((item) => { item.onclick = () => openSession(item.dataset.id); });
  $$('[data-page]').forEach((item) => { item.onclick = () => { state.query.set('page', item.dataset.page); syncQueryForm(); loadSessions(false); }; });
}
async function loadSessions(refreshDetail = true) {
  try {
    if (!state.query.has('pageSize')) state.query.set('pageSize', '20');
    const data = await api(`/api/sessions?${state.query.toString()}`);
    state.sessions = data.sessions || [];
    state.summary = data.summary;
    state.page = data.page;
    state.pages = data.pages;
    history.replaceState(null, '', `${location.pathname}?${state.query.toString()}`);
    renderRiskSummary();
    renderSessions();
    if (refreshDetail && state.active) await openSession(state.active);
  } catch (error) { showToast(error.message, true); }
}
async function openSession(id) {
  try {
    state.detail = await api(`/api/sessions/${id}`);
    state.active = id;
    renderSessions();
    renderDetail();
  } catch (error) { showToast(error.message, true); }
}
function commandButton(label, action) { return `<button class="action" data-action="${action}">${label}</button>`; }

function renderDetail() {
  const d = state.detail;
  const s = d.session;
  const draft = s.status === 'Draft';
  const failed = d.cues.some((cue) => cue.status === 'Failed');
  const toolbar = [
    draft && commandButton('＋ 登记设备', 'device'), draft && commandButton('＋ 配置动作', 'cue'), draft && commandButton('▦ 批量配置', 'batch'),
    draft && d.devices.length && d.cues.length && commandButton('确认方案 → Prepared', 'prepare'),
    s.status === 'Prepared' && commandButton('启动干运行 → Running', 'run'),
    s.status === 'Running' && d.pendingCue && commandButton('连续动作实测', 'attempt'),
    s.status === 'Review' && !failed && commandButton('批准安全复核', 'review-approve'),
    s.status === 'Review' && failed && commandButton('退回逐项整改', 'review-correction'),
    s.status === 'Correction' && commandButton('维护整改任务', 'correction'),
    s.status === 'Review' && !d.certificate && !failed && d.reviews.some((review) => review.decision === 'Approved') && commandButton('签发就绪证书', 'certificate'),
  ].filter(Boolean).join('');
  content.innerHTML = `<div class="detail-head"><div><span class="eyebrow">VALIDATION / ${escapeHTML(s.id)}</span><h2>${escapeHTML(s.productionName)}</h2><p>${escapeHTML(s.venue)} · ${escapeHTML(s.technicalDirector)} · ${s.performanceDate.slice(0, 10)}</p></div><span class="state-badge">${escapeHTML(s.status)}</span></div><div class="stat-grid"><div class="stat"><label>当前版本</label><strong>${s.version}</strong></div><div class="stat"><label>设备</label><strong>${d.devices.length}</strong></div><div class="stat"><label>通过动作</label><strong>${d.cues.filter((cue) => cue.status === 'Passed').length}/${d.cues.length}</strong></div><div class="stat"><label>违规项</label><strong>${d.violations.reduce((total, item) => total + item.violations.length, 0)}</strong></div></div><div class="toolbar">${toolbar}</div>${renderDevices(d, draft)}${renderCues(d, draft)}${renderCorrectionTasks(d)}${renderViolations(d)}${renderCertificate(d)}${renderTimeline(d)}`;
  $$('[data-action]').forEach((item) => { item.onclick = () => handleAction(item.dataset.action, item.dataset.id); });
}
function renderDevices(d, draft) {
  return `<section class="section-block"><div class="block-title"><h3>设备基线 / DEVICES</h3><span class="mono">${d.devices.length} registered</span></div><table class="data-table"><thead><tr><th>设备</th><th>类型</th><th>额定载荷</th><th>安全区域</th><th>急停</th>${draft ? '<th></th>' : ''}</tr></thead><tbody>${d.devices.map((device) => `<tr><td><strong>${escapeHTML(device.name)}</strong><small class="row-id">${escapeHTML(device.id)}</small></td><td>${escapeHTML(device.deviceType)}</td><td class="mono">${device.ratedLoadKg} kg</td><td>${escapeHTML(device.safeZone)}</td><td>${device.emergencyStopRequired ? '必检' : '不要求'}</td>${draft ? `<td class="row-actions"><button data-action="edit-device" data-id="${escapeHTML(device.id)}" title="修订设备">✎</button><button data-action="delete-device" data-id="${escapeHTML(device.id)}" title="删除设备">×</button></td>` : ''}</tr>`).join('') || `<tr><td colspan="${draft ? 6 : 5}">尚未登记设备</td></tr>`}</tbody></table></section>`;
}
function renderCues(d, draft) {
  return `<section class="section-block"><div class="block-title"><h3>动作基线 / CUES</h3><span class="mono">按 sequence 有序执行</span></div><table class="data-table"><thead><tr><th>#</th><th>动作</th><th>设备</th><th>阈值</th><th>状态</th>${draft ? '<th></th>' : ''}</tr></thead><tbody>${d.cues.map((cue, index) => `<tr><td class="mono">${String(cue.sequence).padStart(2, '0')}</td><td><strong>${escapeHTML(cue.action)}</strong><small class="row-id">${escapeHTML(cue.id)}</small></td><td>${escapeHTML(cue.deviceID)}</td><td class="mono">${cue.expectedLoadKg}kg · ≥ ${cue.minimumClearanceCm}cm · ≤ ${cue.maximumStopMs}ms</td><td><span class="cue-status ${cue.status.toLowerCase()}">${escapeHTML(cue.status)}</span></td>${draft ? `<td class="row-actions"><button data-action="cue-up" data-id="${escapeHTML(cue.id)}" ${index === 0 ? 'disabled' : ''} title="上移">↑</button><button data-action="cue-down" data-id="${escapeHTML(cue.id)}" ${index === d.cues.length - 1 ? 'disabled' : ''} title="下移">↓</button><button data-action="edit-cue" data-id="${escapeHTML(cue.id)}" title="修订动作">✎</button><button data-action="delete-cue" data-id="${escapeHTML(cue.id)}" title="删除动作">×</button></td>` : ''}</tr>`).join('') || `<tr><td colspan="${draft ? 6 : 5}">尚未配置动作</td></tr>`}</tbody></table></section>`;
}
function renderCorrectionTasks(d) {
  if (!d.correctionTasks?.length) return '';
  return `<section class="section-block"><div class="block-title"><h3>本轮整改任务 / CORRECTIONS</h3></div>${d.correctionTasks.map((task) => `<div class="task-strip"><strong>${escapeHTML(task.cueID)}</strong><span>${escapeHTML(task.owner || '未分配')}</span><span>${task.closedAt ? '已关闭' : '待闭合'}</span><small>${escapeHTML(task.violations.map((item) => item.code).join(' · '))}</small></div>`).join('')}</section>`;
}
function renderViolations(d) {
  if (!d.violations.length) return '';
  return `<section class="section-block"><div class="block-title"><h3>需关注的违规项 / FINDINGS</h3></div>${d.violations.map((item) => item.violations.map((violation) => `<div class="violation"><strong>${escapeHTML(`${item.cueSequence} / ${item.cueAction}`)} · ${escapeHTML(violation.code)}</strong><small>${escapeHTML(violation.message)} · 证据记录 ${escapeHTML(item.attemptID)}</small></div>`).join('')).join('')}</section>`;
}
function renderCertificate(d) {
  if (!d.certificate) return '';
  const certificate = d.certificate;
  return `<section class="section-block"><div class="block-title"><h3>演出就绪证书 / CERTIFICATE</h3><button class="link-button" data-action="verify-certificate">验真报告</button></div><div class="certificate"><h3>READY FOR PERFORMANCE</h3><dl><dt>证书编号</dt><dd>${escapeHTML(certificate.id)}</dd><dt>签发检查员</dt><dd>${escapeHTML(certificate.reviewer)}</dd><dt>签发时间</dt><dd>${escapeHTML(certificate.issuedAt)}</dd><dt>事件链头</dt><dd class="mono">${escapeHTML(certificate.eventHeadHash)}</dd><dt>摘要</dt><dd class="mono">${escapeHTML(certificate.digest)}</dd></dl></div></section>`;
}
function renderTimeline(d) {
  return `<section class="section-block"><div class="block-title"><h3>验证时间线 / EVENT LOG</h3><span class="mono">${d.timeline.length} events</span></div><div class="timeline">${d.timeline.map((entry) => `<div class="timeline-row"><span class="seq">#${String(entry.sequence).padStart(3, '0')}</span><span class="type">${escapeHTML(entry.label)}</span><span class="mono">${escapeHTML(entry.checksum.slice(0, 16))}…</span><time>${new Date(entry.at).toLocaleString('zh-CN', { hour12: false })}</time></div>`).join('')}</div></section>`;
}
async function submit(path, payload, method = 'POST') {
  const result = await api(path, { method, body: JSON.stringify(payload) });
  state.detail = result.detail;
  renderDetail();
  await loadSessions(false);
  showToast(result.commit.duplicate ? '已返回原提交结果' : '操作已写入事件链');
  return result;
}

async function handleAction(action, entityID) {
  const id = state.active;
  try {
    if (action === 'device') return openDeviceDialog();
    if (action === 'cue') return openCueDialog();
    if (action === 'batch') return openBatchDialog();
    if (action === 'attempt') return openAttemptDialog();
    if (action === 'edit-device') return openDeviceDialog(state.detail.devices.find((item) => item.id === entityID));
    if (action === 'edit-cue') return openCueDialog(state.detail.cues.find((item) => item.id === entityID));
    if (action === 'delete-device' && confirm('确认删除这台未被引用的设备？')) await submit(`/api/sessions/${id}/devices/${entityID}`, meta(), 'DELETE');
    if (action === 'delete-cue' && confirm('确认删除动作并压紧后续序号？')) await submit(`/api/sessions/${id}/cues/${entityID}`, meta(), 'DELETE');
    if (action === 'cue-up' || action === 'cue-down') await moveCue(entityID, action === 'cue-up' ? -1 : 1);
    if (action === 'prepare') await submit(`/api/sessions/${id}/prepare`, meta());
    if (action === 'run') await submit(`/api/sessions/${id}/run`, meta());
    if (action === 'review-approve') await submitReview(false);
    if (action === 'review-correction') await submitReview(true);
    if (action === 'correction') return openCorrectionDialog();
    if (action === 'certificate') await submit(`/api/sessions/${id}/certificate`, { ...meta(), id: `cert-${Date.now()}` });
    if (action === 'verify-certificate') return openVerificationDialog();
  } catch (error) { showToast(error.message, true); }
}
async function moveCue(cueID, delta) {
  const ids = state.detail.cues.map((cue) => cue.id);
  const from = ids.indexOf(cueID);
  const to = from + delta;
  [ids[from], ids[to]] = [ids[to], ids[from]];
  await submit(`/api/sessions/${state.active}/cues/reorder`, { ...meta(), cueIDs: ids });
}
async function submitReview(correction) {
  const reviewer = prompt('检查员', state.detail.session.technicalDirector);
  if (!reviewer) return;
  let findings = [];
  let correctionNote = '';
  if (correction) {
    const value = prompt('复核发现项（逗号分隔）', '按结构化违规项逐动作整改');
    if (!value) return;
    findings = value.split(',').map((item) => item.trim()).filter(Boolean);
    correctionNote = prompt('整改要求', '完成负责人、措施、证据并关闭任务') || '';
    if (!correctionNote) return;
  }
  await submit(`/api/sessions/${state.active}/reviews`, { ...meta(), id: `review-${Date.now()}`, reviewer, decision: correction ? 'NeedsCorrection' : 'Approved', findings, correctionNote });
}

function openSessionDialog() { $('#session-form').reset(); $('#session-dialog').showModal(); }
function openDeviceDialog(device = null) {
  state.editDevice = device;
  const form = $('#device-form');
  form.reset();
  form.querySelector('h2').textContent = device ? '修订吊挂设备' : '登记吊挂设备';
  if (device) Object.entries(device).forEach(([name, value]) => { const field = form.elements[name]; if (field) field.type === 'checkbox' ? field.checked = value : field.value = value; });
  $('#device-dialog').showModal();
}
function openCueDialog(cue = null) {
  state.editCue = cue;
  const form = $('#cue-form');
  form.reset();
  form.querySelector('h2').textContent = cue ? '修订安全动作' : '配置安全动作';
  form.elements.deviceID.innerHTML = state.detail.devices.map((device) => `<option value="${escapeHTML(device.id)}">${escapeHTML(device.name)}</option>`).join('');
  if (cue) Object.entries(cue).forEach(([name, value]) => { if (form.elements[name]) form.elements[name].value = value; });
  else form.elements.sequence.value = state.detail.cues.length + 1;
  $('#cue-dialog').showModal();
}

function addBatchDevice(values = {}) {
  $('#batch-devices').insertAdjacentHTML('beforeend', `<div class="batch-row device-batch-row"><input data-field="id" placeholder="设备 ID" required value="${escapeHTML(values.id || '')}"><input data-field="name" placeholder="设备名称" required><input data-field="deviceType" placeholder="设备类型" required><input data-field="ratedLoadKg" type="number" min="0.1" step="0.1" placeholder="额定 kg" required><input data-field="safeZone" placeholder="安全区域" required><label class="compact-check"><input data-field="emergencyStopRequired" type="checkbox" checked>急停</label><button type="button" class="remove-row" title="删除行">×</button></div>`);
  invalidatePreflight();
}
function addBatchCue(values = {}) {
  $('#batch-cues').insertAdjacentHTML('beforeend', `<div class="batch-row cue-batch-row"><input data-field="id" placeholder="动作 ID" required><input data-field="sequence" type="number" min="1" placeholder="#" required value="${values.sequence || state.detail.cues.length + $$('.cue-batch-row').length + 1}"><input data-field="deviceID" placeholder="设备 ID" required value="${escapeHTML(values.deviceID || state.detail.devices[0]?.id || '')}"><input data-field="action" placeholder="动作说明" required><input data-field="expectedLoadKg" type="number" min="0.1" step="0.1" placeholder="载荷 kg" required><input data-field="minimumClearanceCm" type="number" min="0.1" step="0.1" placeholder="净空 cm" required><input data-field="maximumStopMs" type="number" min="0" placeholder="急停 ms" required><button type="button" class="remove-row" title="删除行">×</button></div>`);
  invalidatePreflight();
}
function wireBatchRows() {
  $$('.remove-row').forEach((button) => { button.onclick = () => { button.parentElement.remove(); invalidatePreflight(); }; });
  $$('#batch-form input').forEach((input) => { input.oninput = invalidatePreflight; });
}
function invalidatePreflight() { state.batchValid = false; $('#confirm-batch').disabled = true; $('#preflight-result').textContent = ''; setTimeout(wireBatchRows); }
function openBatchDialog() {
  $('#batch-form').reset();
  $('#batch-devices').innerHTML = '';
  $('#batch-cues').innerHTML = '';
  addBatchDevice();
  addBatchCue();
  wireBatchRows();
  $('#batch-dialog').showModal();
}
function collectBatch() {
  const devices = $$('.device-batch-row').map((row) => ({ id: row.querySelector('[data-field=id]').value, name: row.querySelector('[data-field=name]').value, deviceType: row.querySelector('[data-field=deviceType]').value, ratedLoadKg: Number(row.querySelector('[data-field=ratedLoadKg]').value), safeZone: row.querySelector('[data-field=safeZone]').value, emergencyStopRequired: row.querySelector('[data-field=emergencyStopRequired]').checked }));
  const cues = $$('.cue-batch-row').map((row) => ({ id: row.querySelector('[data-field=id]').value, sequence: Number(row.querySelector('[data-field=sequence]').value), deviceID: row.querySelector('[data-field=deviceID]').value, action: row.querySelector('[data-field=action]').value, expectedLoadKg: Number(row.querySelector('[data-field=expectedLoadKg]').value), minimumClearanceCm: Number(row.querySelector('[data-field=minimumClearanceCm]').value), maximumStopMs: Number(row.querySelector('[data-field=maximumStopMs]').value) }));
  return { devices, cues };
}
async function runPreflight() {
  try {
    const report = await api(`/api/sessions/${state.active}/configuration/preflight`, { method: 'POST', body: JSON.stringify(collectBatch()) });
    state.batchValid = report.valid;
    $('#confirm-batch').disabled = !report.valid;
    $('#preflight-result').innerHTML = report.valid ? `<strong class="pass-text">预检通过：${report.deviceCount} 台设备，${report.cueCount} 个动作</strong>` : report.problems.map((problem) => `<div class="problem-line">${problem.row ? `第 ${problem.row} 行 · ` : ''}${escapeHTML(problem.entity)}.${escapeHTML(problem.field)} · ${escapeHTML(problem.message)}</div>`).join('');
  } catch (error) { showToast(error.message, true); }
}

function openAttemptDialog() {
  const pending = state.detail.cues.filter((cue) => cue.status === 'Pending');
  $('#attempt-count').max = pending.length;
  $('#attempt-count').value = 1;
  renderAttemptRows();
  $('#attempt-dialog').showModal();
}
function renderAttemptRows() {
  const count = Math.max(1, Math.min(Number($('#attempt-count').value), state.detail.cues.filter((cue) => cue.status === 'Pending').length));
  const pending = state.detail.cues.filter((cue) => cue.status === 'Pending').slice(0, count);
  $('#attempt-rows').innerHTML = pending.map((cue) => `<fieldset class="attempt-row" data-cue-id="${escapeHTML(cue.id)}"><legend>${cue.sequence} / ${escapeHTML(cue.action)}</legend><div class="form-grid three"><label>实测载荷<input data-field="measuredLoadKg" type="number" min="0" step="0.1" required></label><label>实测净空<input data-field="measuredClearanceCm" type="number" min="0" step="0.1" required></label><label>急停时间<input data-field="measuredStopMs" type="number" min="0" required></label></div><div class="form-grid"><label>操作员<input data-field="operator" required></label><label>现场证据<input data-field="evidenceNote" required></label></div></fieldset>`).join('');
}
function collectAttempts() {
  return $$('.attempt-row').map((row, index) => ({ id: `att-${Date.now()}-${index + 1}`, cueID: row.dataset.cueId, measuredLoadKg: Number(row.querySelector('[data-field=measuredLoadKg]').value), measuredClearanceCm: Number(row.querySelector('[data-field=measuredClearanceCm]').value), measuredStopMs: Number(row.querySelector('[data-field=measuredStopMs]').value), operator: row.querySelector('[data-field=operator]').value, evidenceNote: row.querySelector('[data-field=evidenceNote]').value }));
}

function openCorrectionDialog() {
  $('#correction-tasks').innerHTML = state.detail.correctionTasks.map((task) => `<fieldset class="correction-row" data-cue-id="${escapeHTML(task.cueID)}"><legend>${escapeHTML(task.cueID)} · 实测 ${escapeHTML(task.attemptID)}</legend><div class="task-violations">${task.violations.map((item) => escapeHTML(item.message)).join('<br>')}</div><label>整改措施<input data-field="measure" required value="${escapeHTML(task.measure)}"></label><div class="form-grid"><label>负责人<input data-field="owner" required value="${escapeHTML(task.owner)}"></label><label>完成证据<input data-field="evidenceNote" required value="${escapeHTML(task.evidenceNote)}"></label></div><label class="check-label"><input data-field="closed" type="checkbox" ${task.closedAt ? 'checked' : ''}> 关闭本项整改任务</label></fieldset>`).join('');
  $('#correction-dialog').showModal();
}

function openVerificationDialog() {
  const certificate = state.detail.certificate;
  const form = $('#verification-form');
  form.elements.digest.value = certificate.digest;
  form.elements.eventHeadHash.value = certificate.eventHeadHash;
  form.elements.sessionVersion.value = certificate.sessionVersion;
  $('#verification-result').innerHTML = '';
  $('#verification-dialog').showModal();
}

$('#new-session').onclick = openSessionDialog;
$('#empty-new').onclick = openSessionDialog;
$('#query-form').onsubmit = (event) => {
  event.preventDefault();
  const form = new FormData(event.target);
  state.query = new URLSearchParams();
  for (const [field, value] of form.entries()) if (value) state.query.set(field, value);
  state.query.set('page', '1');
  state.query.set('pageSize', '20');
  loadSessions(false);
};
$('#query-form').onreset = () => { setTimeout(() => { state.query = new URLSearchParams('page=1&pageSize=20'); loadSessions(false); }); };
function syncQueryForm() {
  const form = $('#query-form');
  for (const field of ['q', 'status', 'venue', 'technicalDirector', 'performanceFrom', 'performanceTo', 'sort']) form.elements[field].value = state.query.get(field) || (field === 'sort' ? 'createdAt' : '');
}
$('#session-form').onsubmit = async (event) => {
  event.preventDefault();
  const form = new FormData(event.target);
  try {
    const result = await api('/api/sessions', { method: 'POST', body: JSON.stringify({ productionName: form.get('productionName'), venue: form.get('venue'), performanceDate: `${form.get('performanceDate')}T00:00:00Z`, technicalDirector: form.get('technicalDirector'), expectedVersion: 0, idempotencyKey: key('session') }) });
    $('#session-dialog').close();
    state.active = result.detail.session.id;
    state.detail = result.detail;
    await loadSessions(false);
    renderDetail();
    showToast('已创建 Draft 会话');
  } catch (error) { showToast(error.message, true); }
};
$('#device-form').onsubmit = async (event) => {
  event.preventDefault();
  const form = new FormData(event.target);
  const payload = { ...meta(), name: form.get('name'), deviceType: form.get('deviceType'), ratedLoadKg: Number(form.get('ratedLoadKg')), safeZone: form.get('safeZone'), emergencyStopRequired: form.get('emergencyStopRequired') === 'on' };
  try {
    const path = state.editDevice ? `/api/sessions/${state.active}/devices/${state.editDevice.id}` : `/api/sessions/${state.active}/devices`;
    await submit(path, payload, state.editDevice ? 'PUT' : 'POST');
    $('#device-dialog').close();
  } catch (error) { showToast(error.message, true); }
};
$('#cue-form').onsubmit = async (event) => {
  event.preventDefault();
  const form = new FormData(event.target);
  const payload = { ...meta(), sequence: Number(form.get('sequence')), deviceID: form.get('deviceID'), action: form.get('action'), expectedLoadKg: Number(form.get('expectedLoadKg')), minimumClearanceCm: Number(form.get('minimumClearanceCm')), maximumStopMs: Number(form.get('maximumStopMs')) };
  try {
    const path = state.editCue ? `/api/sessions/${state.active}/cues/${state.editCue.id}` : `/api/sessions/${state.active}/cues`;
    await submit(path, payload, state.editCue ? 'PUT' : 'POST');
    $('#cue-dialog').close();
  } catch (error) { showToast(error.message, true); }
};
$('#add-batch-device').onclick = () => addBatchDevice();
$('#add-batch-cue').onclick = () => addBatchCue();
$('#run-preflight').onclick = runPreflight;
$('#batch-form').onsubmit = async (event) => {
  event.preventDefault();
  if (!state.batchValid) return;
  try { await submit(`/api/sessions/${state.active}/configuration/batch`, { ...meta(), ...collectBatch() }); $('#batch-dialog').close(); } catch (error) { showToast(error.message, true); }
};
$('#attempt-count').oninput = renderAttemptRows;
$('#attempt-form').onsubmit = async (event) => {
  event.preventDefault();
  try { await submit(`/api/sessions/${state.active}/attempts/batch`, { ...meta(), attempts: collectAttempts() }); $('#attempt-dialog').close(); } catch (error) { showToast(error.message, true); }
};
$('#correction-form').onsubmit = async (event) => {
  event.preventDefault();
  try {
    for (const row of $$('.correction-row')) {
      await submit(`/api/sessions/${state.active}/corrections/${row.dataset.cueId}`, { ...meta(), measure: row.querySelector('[data-field=measure]').value, owner: row.querySelector('[data-field=owner]').value, evidenceNote: row.querySelector('[data-field=evidenceNote]').value, closed: row.querySelector('[data-field=closed]').checked }, 'PUT');
    }
    await submit(`/api/sessions/${state.active}/corrections`, { ...meta(), note: '逐动作整改任务均已闭合' });
    $('#correction-dialog').close();
  } catch (error) { showToast(error.message, true); }
};
$('#verification-form').onsubmit = async (event) => {
  event.preventDefault();
  const form = new FormData(event.target);
  const query = new URLSearchParams({ digest: form.get('digest'), eventHeadHash: form.get('eventHeadHash'), sessionVersion: form.get('sessionVersion') });
  try {
    const certificate = state.detail.certificate;
    const report = await api(`/api/sessions/${state.active}/certificates/${certificate.id}/verification?${query}`);
    $('#verification-result').innerHTML = `<div class="verification-head ${report.valid ? 'passed' : 'failed'}">${report.valid ? '验真通过' : `验真失败 · 首个失败序号 ${report.firstFailureSequence || '未定位'}`}</div>${report.checks.map((check) => `<div class="check-row"><span>${check.passed ? '✓' : '×'}</span><strong>${escapeHTML(check.name)}</strong><small>${escapeHTML(check.message)}</small></div>`).join('')}`;
  } catch (error) { showToast(error.message, true); }
};

setInterval(() => { $('#clock').textContent = new Date().toLocaleTimeString('zh-CN', { hour12: false }); }, 1000);
syncQueryForm();
loadSessions(false);
