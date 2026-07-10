/* =============================================================
   Log Analyzer — SPA Application
   =============================================================
   Architecture: module-level state, page functions, SVG charts
   Zero dependencies, ES module, embedded via //go:embed
   ============================================================= */

/* --- Utils -------------------------------------------------- */
const $ = (s, p) => (p || document).querySelector(s);
const $$ = (s, p) => Array.from((p || document).querySelectorAll(s));

const API = '';
const PAGE_SIZE = 100;

function escapeHtml(s) {
  if (!s) return '';
  const d = document.createElement('div');
  d.textContent = s;
  return d.innerHTML;
}

function inlineMarkdown(s) {
  return s
    .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
    .replace(/\*(.+?)\*/g, '<em>$1</em>')
    .replace(/`(.+?)`/g, '<code>$1</code>')
    .replace(/__(.+?)__/g, '<strong>$1</strong>')
    .replace(/_(.+?)_/g, '<em>$1</em>');
}

function renderMarkdown(text) {
  if (!text) return '';
  const e = escapeHtml(text);
  const lines = e.split('\n');
  let html = '';
  let inList = false;

  function closeList() {
    if (inList === 'ul') { html += '</ul>'; inList = false; }
    else if (inList === 'ol') { html += '</ol>'; inList = false; }
  }

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];

    const h3 = line.match(/^### (.+)/);
    const h2 = line.match(/^## (.+)/);
    const h1 = line.match(/^# (.+)/);
    if (h3) { closeList(); html += '<h3>' + inlineMarkdown(h3[1]) + '</h3>'; continue; }
    if (h2) { closeList(); html += '<h2>' + inlineMarkdown(h2[1]) + '</h2>'; continue; }
    if (h1) { closeList(); html += '<h1>' + inlineMarkdown(h1[1]) + '</h1>'; continue; }

    if (/^-{3,}$/.test(line.trim())) { closeList(); html += '<hr>'; continue; }

    const ulMatch = line.match(/^- (.+)/);
    if (ulMatch) {
      if (!inList) { html += '<ul>'; inList = 'ul'; }
      html += '<li>' + inlineMarkdown(ulMatch[1]) + '</li>';
      continue;
    }

    const olMatch = line.match(/^\d+\.\s+(.+)/);
    if (olMatch) {
      if (!inList) { html += '<ol>'; inList = 'ol'; }
      html += '<li>' + inlineMarkdown(olMatch[1]) + '</li>';
      continue;
    }

    closeList();
    if (line.trim() === '') continue;
    html += '<p>' + inlineMarkdown(line) + '</p>';
  }

  closeList();
  return html;
}

function formatBytes(b) {
  if (b < 1024) return b + ' B';
  if (b < 1048576) return (b / 1024).toFixed(1) + ' KB';
  return (b / 1048576).toFixed(1) + ' MB';
}

function formatTime(iso) {
  if (!iso) return '';
  return new Date(iso).toLocaleString();
}

function formatTimeShort(iso) {
  if (!iso) return '';
  return new Date(iso).toLocaleTimeString();
}

function formatDuration(sec) {
  if (sec < 60) return sec + 's';
  if (sec < 3600) return Math.floor(sec / 60) + 'm ' + (sec % 60) + 's';
  return Math.floor(sec / 3600) + 'h ' + Math.floor((sec % 3600) / 60) + 'm';
}

function debounce(fn, ms) {
  let timer;
  return function (...args) {
    clearTimeout(timer);
    timer = setTimeout(() => fn.apply(this, args), ms);
  };
}

async function fetchJSON(url, opts) {
  try {
    const res = await fetch(url, opts);
    if (!res.ok) {
      const text = await res.text().catch(() => '');
      throw new Error(text || `HTTP ${res.status}`);
    }
    return await res.json();
  } catch (err) {
    throw err;
  }
}

/* --- Toast System ------------------------------------------- */
let toastContainer = null;

function ensureToastContainer() {
  if (!toastContainer) {
    toastContainer = document.createElement('div');
    toastContainer.className = 'toast-container';
    document.body.appendChild(toastContainer);
  }
  return toastContainer;
}

function showToast(message, type) {
  const c = ensureToastContainer();
  const t = document.createElement('div');
  t.className = 'toast toast-' + (type || 'info');
  t.textContent = message;
  c.appendChild(t);
  setTimeout(() => {
    t.classList.add('toast-out');
    setTimeout(() => t.remove(), 200);
  }, 3000);
}

/* --- SVG Charts --------------------------------------------- */
function renderLevelChart(svgEl, levels) {
  const entries = Object.entries(levels).filter(([_, v]) => v > 0);
  if (entries.length === 0) { svgEl.innerHTML = ''; return; }
  const total = entries.reduce((s, [_, v]) => s + v, 0);
  const order = ['FATAL', 'ERROR', 'WARN', 'INFO', 'DEBUG'];
  const sorted = order.filter(k => levels[k]).map(k => [k, levels[k]]);
  let html = '<div class="chart-bar">';
  for (const [level, count] of sorted) {
    const pct = total > 0 ? (count / total) * 100 : 0;
    html += `<div class="chart-bar-row">
      <span class="chart-bar-label level-${level}">${level}</span>
      <div class="chart-bar-track"><div class="chart-bar-fill level-${level}" style="width:${pct}%"></div></div>
      <span class="chart-bar-val">${count.toLocaleString()} (${pct.toFixed(1)}%)</span>
    </div>`;
  }
  svgEl.innerHTML = html;
}

/* --- Theme -------------------------------------------------- */
function setTheme(theme) {
  document.documentElement.setAttribute('data-theme', theme);
  localStorage.setItem('theme', theme);
}

function toggleTheme() {
  const current = document.documentElement.getAttribute('data-theme');
  setTheme(current === 'dark' ? 'light' : 'dark');
}

/* --- Event Bus ---------------------------------------------- */
const bus = {
  _listeners: {},
  on(evt, fn) { (this._listeners[evt] ||= []).push(fn); },
  off(evt, fn) { this._listeners[evt] = this._listeners[evt]?.filter(f => f !== fn); },
  emit(evt, data) { this._listeners[evt]?.forEach(fn => fn(data)); }
};

/* --- Sidebar ------------------------------------------------- */
function renderSidebar(currentPath) {
  const navItems = [
    { path: '/', label: 'Dashboard', icon: 'M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6', shortcut: '⌘1' },
    { path: '/upload', label: 'Upload', icon: 'M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12', shortcut: '⌘U' },
  ];

  const html = `
    <aside class="sidebar" id="sidebar">
      <div class="sidebar-brand">
        <h1>Log Analyzer</h1>
        <div class="version">v1.0.0</div>
      </div>
      <nav class="sidebar-nav">
        ${navItems.map(item => `
          <a href="${item.path}" class="nav-item ${currentPath === item.path ? 'active' : ''}" data-nav>
            <svg class="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="${item.icon}"/></svg>
            ${item.label}
            <span class="nav-shortcut">${item.shortcut}</span>
          </a>
        `).join('')}
      </nav>
      <div class="sidebar-footer">
        <button class="theme-toggle" id="theme-btn" title="Toggle theme">
          <svg class="toggle-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M21 12.79A9 9 0 1111.21 3 7 7 0 0021 12.79z"/>
          </svg>
          <span>Toggle theme</span>
        </button>
      </div>
    </aside>
  `;
  return html;
}

/* --- Layout wrapper ----------------------------------------- */
function renderLayout(mainHTML, currentPath) {
  return `
    ${renderSidebar(currentPath)}
    <main class="main-content">
      <div class="page-enter" id="page-content">${mainHTML}</div>
    </main>
  `;
}

/* =============================================================
   Pages
   ============================================================= */

/* --- Dashboard ----------------------------------------------- */
function renderDashboard() {
  const html = `
    <div class="page-header">
      <h2>Dashboard</h2>
      <div class="header-actions">
        <a href="/upload" class="btn btn-primary btn-sm" data-nav>
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/></svg>
          Upload
        </a>
      </div>
    </div>
    <div class="stat-grid" id="dash-stats">
      <div class="stat-card stat-total"><div class="stat-value">—</div><div class="stat-label">Sessions</div></div>
      <div class="stat-card stat-total"><div class="stat-value">—</div><div class="stat-label">Files analyzed</div></div>
      <div class="stat-card stat-info"><div class="stat-value">—</div><div class="stat-label">Total lines</div></div>
      <div class="stat-card stat-error"><div class="stat-value">—</div><div class="stat-label">Errors found</div></div>
    </div>
    <div class="card">
      <div class="card-header">
        <h3>Sessions</h3>
        <div style="display:flex;gap:8px;align-items:center">
          <input type="text" id="session-search" placeholder="Search sessions..." style="padding:4px 8px;border:1px solid var(--border);border-radius:4px;background:var(--bg);color:var(--text-primary);font-size:13px">
        </div>
      </div>
      <div id="session-table-wrap"><div class="empty-state"><h3>Loading...</h3></div></div>
    </div>
  `;

  const app = $('#app');
  app.innerHTML = renderLayout(html, '/');

  loadDashboardStats();
  loadSessionTable('', '');
  setupDashboardListeners();
}

function setupDashboardListeners() {
  const search = $('#session-search');
  if (search) {
    search.addEventListener('input', debounce(function() {
      loadSessionTable(this.value, '');
    }, 250));
  }
  bus.on('session-deleted', () => loadSessionTable('', ''));
}

async function loadDashboardStats() {
  try {
    const data = await fetchJSON(`${API}/api/sessions`);
    const sessions = data.sessions || [];
    let totalLines = 0;
    let totalErrors = 0;
    let filesAnalyzed = 0;

    for (const s of sessions) {
      if (s.status === 'complete') {
        filesAnalyzed++;
        try {
          const r = await fetchJSON(`${API}/api/results/${s.id}`);
          if (r.report) {
            totalLines += r.report.total_lines || 0;
            totalErrors += (r.report.levels?.ERROR || 0) + (r.report.levels?.FATAL || 0);
          }
        } catch (_) {}
      }
    }

    const stats = $('#dash-stats');
    if (stats) {
      const vals = stats.querySelectorAll('.stat-value');
      if (vals[0]) vals[0].textContent = sessions.length;
      if (vals[1]) vals[1].textContent = filesAnalyzed;
      if (vals[2]) vals[2].textContent = totalLines.toLocaleString();
      if (vals[3]) vals[3].textContent = totalErrors.toLocaleString();
    }
  } catch (_) {}
}

async function loadSessionTable(search, sort) {
  try {
    const data = await fetchJSON(`${API}/api/sessions`);
    let sessions = data.sessions || [];

    if (search) {
      const q = search.toLowerCase();
      sessions = sessions.filter(s =>
        s.file_name?.toLowerCase().includes(q) || s.status?.toLowerCase().includes(q)
      );
    }

    if (sort === 'name') sessions.sort((a, b) => (a.file_name || '').localeCompare(b.file_name || ''));
    else if (sort === 'date') sessions.sort((a, b) => (b.created_at || '').localeCompare(a.created_at || ''));
    else sessions.sort((a, b) => (b.created_at || '').localeCompare(a.created_at || ''));

    const wrap = $('#session-table-wrap');
    if (!wrap) return;

    if (sessions.length === 0) {
      wrap.innerHTML = `
        <div class="empty-state">
          <svg class="empty-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/><polyline points="10 9 9 9 8 9"/></svg>
          <h3>No sessions yet</h3>
          <p>Upload a log file to start analyzing.</p>
          <a href="/upload" class="btn btn-primary" data-nav>Upload your first file</a>
        </div>`;
      $$('[data-nav]').forEach(el => el.addEventListener('click', navClick));
      return;
    }

    let html = `<div class="table-container"><table><thead><tr>
      <th data-sort="name">File <span class="sort-icon">↕</span></th>
      <th>Status</th>
      <th data-sort="date">Created <span class="sort-icon">↕</span></th>
      <th>Actions</th>
    </tr></thead><tbody>`;

    for (const s of sessions) {
      html += `<tr class="clickable" data-id="${s.id}">
        <td>${escapeHtml(s.file_name)}</td>
        <td><span class="badge badge-${s.status}">${s.status}</span></td>
        <td>${formatTime(s.created_at)}</td>
        <td>
          <a href="/session/${s.id}" class="btn btn-sm" data-nav>View</a>
          <button class="btn btn-sm btn-danger" data-delete="${s.id}">Delete</button>
        </td>
      </tr>`;
    }
    html += '</tbody></table></div>';
    wrap.innerHTML = html;

    wrap.querySelectorAll('[data-delete]').forEach(btn => {
      btn.addEventListener('click', async (e) => {
        e.stopPropagation();
        const id = btn.getAttribute('data-delete');
        if (!confirm('Delete this session?')) return;
        try {
          await fetchJSON(`${API}/api/sessions/${id}`, { method: 'DELETE' });
          showToast('Session deleted', 'success');
          bus.emit('session-deleted');
        } catch (err) {
          showToast('Failed to delete: ' + err.message, 'error');
        }
      });
    });

    wrap.querySelectorAll('tr.clickable').forEach(row => {
      row.addEventListener('click', () => {
        const id = row.getAttribute('data-id');
        if (id) navigate(`/session/${id}`);
      });
    });

    wrap.querySelectorAll('th[data-sort]').forEach(th => {
      th.addEventListener('click', () => loadSessionTable(search, th.getAttribute('data-sort')));
    });

    $$('[data-nav]').forEach(el => el.addEventListener('click', navClick));
  } catch (err) {
    const wrap = $('#session-table-wrap');
    if (wrap) wrap.innerHTML = `<div class="error-message">Failed to load sessions: ${escapeHtml(err.message)}</div>`;
  }
}

/* --- Upload -------------------------------------------------- */
let _uploadedFile = null;

function renderUpload() {
  const html = `
    <div class="page-header">
      <h2>Upload Log File</h2>
    </div>
    <div class="card">
      <div class="upload-zone" id="drop-zone">
        <svg class="upload-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/></svg>
        <p>Drag &amp; drop a log file here, or click to select</p>
        <div class="upload-hint">Supported: .log .txt .csv .gz (max 100 MB)</div>
        <input type="file" id="file-input" accept=".log,.txt,.csv,.gz" style="display:none">
      </div>
    </div>
    <div class="card" id="options-card" style="display:none">
      <div class="card-header"><h3>Analysis Options</h3></div>
      <div style="display:grid;grid-template-columns:1fr 1fr;gap:16px">
        <div class="form-group">
          <label>Command</label>
          <select id="cmd">
            <option value="scan">Scan — full report</option>
            <option value="errors">Errors — error lines</option>
            <option value="top">Top — top errors</option>
            <option value="grep">Grep — regex search</option>
          </select>
        </div>
        <div class="form-group">
          <label>Level filter</label>
          <select id="level">
            <option value="">All levels</option>
            <option value="error">Error + Fatal</option>
            <option value="warn">Warn + Error + Fatal</option>
            <option value="info">Info + above</option>
          </select>
        </div>
        <div class="form-group">
          <label>Regex pattern</label>
          <input type="text" id="regex" placeholder="e.g. timeout|5[0-9][0-9]" autocomplete="off">
        </div>
        <div class="form-group">
          <label>Limit (top errors)</label>
          <input type="number" id="limit" value="10" min="1" max="1000">
        </div>
        <div class="form-group">
          <label>Since</label>
          <input type="text" id="since" placeholder="e.g. 1h, 30m" autocomplete="off">
        </div>
      </div>
      <div style="margin-top:16px;display:flex;gap:8px;align-items:center">
        <button class="btn btn-primary" id="analyze-btn" disabled>Analyze</button>
        <span id="file-status" style="font-size:13px;color:var(--text-secondary)"></span>
      </div>
    </div>
    <div id="upload-progress" style="display:none">
      <div class="card">
        <div style="display:flex;align-items:center;gap:12px">
          <div class="spinner" style="width:16px;height:16px;border:2px solid var(--border);border-top-color:var(--accent);border-radius:50%;animation:spin 0.8s linear infinite"></div>
          <span id="progress-text" style="font-size:14px;color:var(--text-secondary)">Uploading...</span>
        </div>
        <div class="progress-bar"><div class="progress-bar-fill" id="progress-fill"></div></div>
      </div>
    </div>
    <div id="upload-result"></div>
  `;

  _uploadedFile = null;
  const app = $('#app');
  app.innerHTML = renderLayout(html, '/upload');

  setupUploadUI();
}

function setupUploadUI() {
  const dropZone = $('#drop-zone');
  const fileInput = $('#file-input');
  const optionsCard = $('#options-card');
  const analyzeBtn = $('#analyze-btn');
  const fileStatus = $('#file-status');

  dropZone.addEventListener('click', () => fileInput.click());

  dropZone.addEventListener('dragover', (e) => { e.preventDefault(); dropZone.classList.add('drag-over'); });
  dropZone.addEventListener('dragleave', () => dropZone.classList.remove('drag-over'));
  dropZone.addEventListener('drop', (e) => {
    e.preventDefault();
    dropZone.classList.remove('drag-over');
    if (e.dataTransfer.files.length > 0) handleUploadFile(e.dataTransfer.files[0]);
  });
  fileInput.addEventListener('change', () => {
    if (fileInput.files.length > 0) handleUploadFile(fileInput.files[0]);
  });

  function handleUploadFile(file) {
    if (file.size > 100 * 1024 * 1024) {
      showToast('File exceeds 100 MB limit', 'error');
      return;
    }
    _uploadedFile = file;
    dropZone.innerHTML = `
      <svg class="upload-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>
      <p class="file-info">${escapeHtml(file.name)} (${formatBytes(file.size)})</p>`;
    optionsCard.style.display = 'block';
    analyzeBtn.disabled = false;
    fileStatus.textContent = 'Ready to analyze';
  }

  analyzeBtn.addEventListener('click', startAnalysis);
}

async function startAnalysis() {
  if (!_uploadedFile) return;
  const analyzeBtn = $('#analyze-btn');
  const progress = $('#upload-progress');
  const progressText = $('#progress-text');
  const progressFill = $('#progress-fill');
  const resultEl = $('#upload-result');

  analyzeBtn.disabled = true;
  analyzeBtn.textContent = 'Analyzing...';
  resultEl.innerHTML = '';
  progress.style.display = 'block';
  progressText.textContent = 'Uploading...';
  progressFill.style.width = '0%';

  const formData = new FormData();
  formData.append('file', _uploadedFile);

  try {
    progressFill.style.width = '30%';
    const uploadData = await fetchJSON(`${API}/api/upload`, { method: 'POST', body: formData });
    progressFill.style.width = '60%';
    const sessionId = uploadData.session_id;

    const cmd = $('#cmd').value;
    const level = $('#level').value;
    const regex = $('#regex').value;
    const limit = parseInt($('#limit').value) || 10;
    const since = $('#since').value;

    progressText.textContent = 'Analyzing...';
    await fetchJSON(`${API}/api/analyze/${sessionId}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ command: cmd, level, regex, limit, since })
    });

    progressFill.style.width = '90%';
    progressText.textContent = 'Processing results...';

    await pollForResults(sessionId, cmd, resultEl);
    progressFill.style.width = '100%';
    progress.style.display = 'none';
    analyzeBtn.textContent = 'Analyze';
    analyzeBtn.disabled = false;
  } catch (err) {
    progress.style.display = 'none';
    resultEl.innerHTML = `<div class="error-message">Failed: ${escapeHtml(err.message)}</div>`;
    analyzeBtn.textContent = 'Analyze';
    analyzeBtn.disabled = false;
  }
}

async function pollForResults(sessionId, cmd, resultEl) {
  return new Promise((resolve, reject) => {
    const evtSource = new EventSource(`${API}/api/status/${sessionId}`);
    const timeout = setTimeout(() => {
      evtSource.close();
      reject(new Error('Analysis timed out'));
    }, 120000);

    evtSource.addEventListener('complete', async () => {
      clearTimeout(timeout);
      evtSource.close();
      try {
        await renderUploadResults(sessionId, cmd, resultEl);
        resolve();
      } catch (err) {
        reject(err);
      }
    });

    evtSource.addEventListener('error', async () => {
      clearTimeout(timeout);
      evtSource.close();
      try {
        await renderUploadResults(sessionId, cmd, resultEl);
        resolve();
      } catch (err) {
        reject(err);
      }
    });
  });
}

async function renderUploadResults(sessionId, cmd, resultEl) {
  const data = await fetchJSON(`${API}/api/results/${sessionId}`);
  if (data.status === 'error') {
    resultEl.innerHTML = `<div class="error-message">Analysis error: ${escapeHtml(data.error || 'unknown')}</div>`;
    return;
  }

  let html = '';
  if ((cmd === 'scan' || cmd === 'top') && data.report) {
    html = buildReportHTML(data.report);
  } else if ((cmd === 'errors' || cmd === 'grep') && data.events) {
    html = `<div class="card">
      <div class="event-count-header">${data.events.length} matching events</div>
      ${data.events.slice(0, 50).map(e => buildEventRowHTML(e)).join('')}
      ${data.events.length > 50 ? `<p style="color:var(--text-tertiary);font-size:13px;padding:8px;text-align:center">Showing first 50 of ${data.events.length} events — <a href="/session/${sessionId}" data-nav>View all in session</a></p>` : ''}
    </div>`;
  }

  html += `<div style="margin-top:12px"><a href="/session/${sessionId}" class="btn btn-sm" data-nav>View full session →</a></div>`;
  resultEl.innerHTML = html;
  if (data.report) {
    const chartEl = resultEl.querySelector('#level-chart');
    if (chartEl) renderLevelChart(chartEl, data.report.levels || {});
  }
  $$('[data-nav]').forEach(el => el.addEventListener('click', navClick));
}

/* --- Session Detail ------------------------------------------ */
function renderSession(id) {
  const html = `
    <div class="breadcrumb">
      <a href="/" data-nav>Dashboard</a>
      <span class="sep">/</span>
      <span class="current" id="session-title">Session</span>
    </div>
    <div class="tab-bar">
      <button class="tab-item active" data-tab="overview">Overview</button>
      <button class="tab-item" data-tab="events">Events</button>
      <button class="tab-item" data-tab="insights">AI Insights</button>
      <button class="tab-item" data-tab="raw">Raw</button>
    </div>
    <div id="tab-overview" class="tab-content active"><div class="card"><div class="empty-state"><h3>Loading...</h3></div></div></div>
    <div id="tab-events" class="tab-content"><div class="card"><div class="empty-state"><h3>Loading...</h3></div></div></div>
    <div id="tab-insights" class="tab-content"><div class="card"><div class="empty-state"><h3>Loading...</h3></div></div></div>
    <div id="tab-raw" class="tab-content"><div class="empty-state" id="raw-loading"><h3>Loading...</h3></div></div>
  `;

  const app = $('#app');
  app.innerHTML = renderLayout(html, `/session/${id}`);

  setupSessionTabs();
  loadSessionData(id);
}

function setupSessionTabs() {
  $$('.tab-item').forEach(tab => {
    tab.addEventListener('click', function() {
      $$('.tab-item').forEach(t => t.classList.remove('active'));
      $$('.tab-content').forEach(c => c.classList.remove('active'));
      this.classList.add('active');
      const tabName = this.getAttribute('data-tab');
      const target = $('#tab-' + tabName);
      if (target) target.classList.add('active');

      if (tabName === 'insights') {
        const id = location.pathname.split('/')[2];
        if (id) loadInsightsTab(id);
      }
    });
  });
}

async function loadSessionData(id) {
  try {
    let data = await fetchJSON(`${API}/api/results/${id}`);

    if (data.status === 'running') {
      $('#tab-overview').innerHTML = `<div class="empty-state"><h3>Analysis in progress...</h3></div>`;
      data = await pollSession(id);
      if (!data) {
        $('#tab-overview').innerHTML = `<div class="empty-state"><h3>Session no longer available</h3></div>`;
        return;
      }
    }

    const title = $('#session-title');
    if (title) title.textContent = 'Session: ' + (data.report?.source || id);

    if (data.status === 'error') {
      $('#tab-overview').innerHTML = `<div class="error-message">${escapeHtml(data.error || 'Analysis failed')}</div>`;
      return;
    }

    if (data.status === 'uploaded') {
      $('#tab-overview').innerHTML = `<div class="empty-state"><h3>Analysis not started</h3><p>This session has not been analyzed yet.</p></div>`;
      return;
    }

    if (data.report) {
      $('#tab-overview').innerHTML = buildReportHTML(data.report);
      const chartEl = $('#level-chart');
      if (chartEl) renderLevelChart(chartEl, data.report.levels || {});
    } else {
      $('#tab-overview').innerHTML = '<div class="empty-state"><h3>No report data</h3></div>';
    }

    const cmd = data.command || 'scan';
    if (cmd === 'errors' || cmd === 'grep') {
      loadEventsTab(id);
    } else {
      const top = data.report?.top_errors;
      if (top && top.length > 0) {
        $('#tab-events').innerHTML = buildErrorGroupsHTML(top);
      } else {
        $('#tab-events').innerHTML = '<div class="empty-state"><h3>No errors found</h3></div>';
      }
    }

    loadRawTab(id);
  } catch (err) {
    $('#tab-overview').innerHTML = `<div class="error-message">Failed to load session: ${escapeHtml(err.message)}</div>`;
  }
}

async function pollSession(id) {
  while ($('#session-title')) {
    await new Promise(resolve => setTimeout(resolve, 1000));
    try {
      const data = await fetchJSON(`${API}/api/results/${id}`);
      if (data.status === 'complete' || data.status === 'error' || data.status === 'uploaded') {
        return data;
      }
    } catch {
      return null;
    }
  }
  return null;
}

function buildErrorGroupsHTML(top) {
  if (!top || top.length === 0) return '';
  let html = `<div class="card"><div class="card-header"><h3>Top Errors (${top.length})</h3></div>`;
  for (const g of top) {
    const first = g.first_seen ? formatTimeShort(g.first_seen) : '';
    const last = g.last_seen ? formatTimeShort(g.last_seen) : '';
    html += `
      <div class="group-item">
        <div class="group-header" data-toggle-group>
          <span class="group-count">${g.count}x</span>
          <span class="group-sig">${escapeHtml(g.signature)}</span>
          <span class="group-range">${first ? `[${first} — ${last}]` : ''}</span>
        </div>
        <div class="group-body">
          <div style="font-size:12px;color:var(--text-tertiary);margin-bottom:4px">Sample:</div>
          <pre>${escapeHtml(g.sample)}</pre>
        </div>
      </div>`;
  }
  html += `</div>`;
  return html;
}

function buildReportHTML(report) {
  if (!report) return '';
  const levels = report.levels || {};
  const total = report.total_lines || 0;
  const tr = report.time_range || {};
  const timeHtml = tr.first
    ? `<div style="font-size:13px;color:var(--text-secondary);margin-top:4px">${formatTime(tr.first)} — ${formatTime(tr.last)} (${formatDuration(tr.duration_sec || 0)})</div>`
    : '';

  let html = `
    <div class="stat-grid">
      <div class="stat-card stat-total"><div class="stat-value">${total.toLocaleString()}</div><div class="stat-label">Total lines</div>${timeHtml}</div>
      <div class="stat-card stat-error"><div class="stat-value">${(levels.ERROR || 0).toLocaleString()}</div><div class="stat-label">Errors</div></div>
      <div class="stat-card stat-warn"><div class="stat-value">${(levels.WARN || 0).toLocaleString()}</div><div class="stat-label">Warnings</div></div>
      <div class="stat-card stat-info"><div class="stat-value">${(levels.INFO || 0).toLocaleString()}</div><div class="stat-label">Info</div></div>
    </div>
    <div class="card">
      <div class="card-header"><h3>Level Breakdown</h3></div>
      <div class="chart-container" id="level-chart"></div>
    </div>`;

  html += buildErrorGroupsHTML(report.top_errors);

  html += `<p style="color:var(--text-secondary);font-size:13px">Source: ${escapeHtml(report.source || 'unknown')}</p>`;

  return html;
}

function buildEventRowHTML(e) {
  const ts = e.timestamp ? formatTimeShort(e.timestamp) : '';
  const level = e.level || 'INFO';
  return `
    <div class="event-row" data-expand-event>
      <span class="event-ts">${ts}</span>
      <span class="level-badge level-${level}">${level}</span>
      <span class="event-msg">${escapeHtml(e.message)}</span>
      <span class="event-line-num">#${e.line || ''}</span>
    </div>
    <div class="event-detail">${escapeHtml(e.raw || e.message)}</div>`;
}

/* --- Events tab with pagination ------------------------------ */
let _eventsState = { id: '', offset: 0, total: 0, filter: '', level: '' };

function loadEventsTab(id) {
  _eventsState = { id, offset: 0, total: 0, filter: '', level: '' };
  $('#tab-events').innerHTML = `
    <div class="card">
      <div class="filter-bar">
        <select id="evt-level-filter">
          <option value="">All levels</option>
          <option value="FATAL">Fatal</option>
          <option value="ERROR">Error</option>
          <option value="WARN">Warning</option>
          <option value="INFO">Info</option>
          <option value="DEBUG">Debug</option>
        </select>
        <input type="text" id="evt-search" placeholder="Filter events..." autocomplete="off">
        <span id="evt-total" style="font-size:13px;color:var(--text-tertiary);margin-left:auto"></span>
      </div>
      <div id="evt-list"><div class="empty-state"><h3>Loading...</h3></div></div>
      <div class="paginator" id="evt-paginator"></div>
    </div>`;

  $('#evt-level-filter').addEventListener('change', () => {
    _eventsState.level = $('#evt-level-filter').value;
    _eventsState.offset = 0;
    fetchEventsPage(id);
  });

  $('#evt-search').addEventListener('input', debounce(function() {
    _eventsState.filter = this.value;
    _eventsState.offset = 0;
    fetchEventsPage(id);
  }, 250));

  fetchEventsPage(id);
}

async function fetchEventsPage(id) {
  try {
    const offset = _eventsState.offset;
    const limit = PAGE_SIZE;
    const data = await fetchJSON(`${API}/api/results/${id}/events?offset=${offset}&limit=${limit}`);
    const events = data.events || [];
    _eventsState.total = data.total || 0;

    const totalEl = $('#evt-total');
    if (totalEl) totalEl.textContent = `${data.total} total events`;

    let filtered = events;
    if (_eventsState.level) {
      filtered = filtered.filter(e => e.level === _eventsState.level);
    }
    if (_eventsState.filter) {
      const q = _eventsState.filter.toLowerCase();
      filtered = filtered.filter(e =>
        (e.message || '').toLowerCase().includes(q)
      );
    }

    const list = $('#evt-list');
    if (filtered.length === 0) {
      list.innerHTML = '<div class="empty-state"><h3>No matching events</h3></div>';
    } else {
      list.innerHTML = filtered.map(e => buildEventRowHTML(e)).join('');
      setupEventExpand();
    }

    renderPaginator(id, data.total);
  } catch (err) {
    $('#evt-list').innerHTML = `<div class="error-message">${escapeHtml(err.message)}</div>`;
  }
}

function setupEventExpand() {
  $$('.event-row[data-expand-event]').forEach(row => {
    row.addEventListener('click', function() {
      const detail = this.nextElementSibling;
      if (detail && detail.classList.contains('event-detail')) {
        this.classList.toggle('expanded');
      }
    });
  });
}

function renderPaginator(id, total) {
  const el = $('#evt-paginator');
  if (!el) return;
  const totalPages = Math.ceil(total / PAGE_SIZE);
  const currentPage = Math.floor(_eventsState.offset / PAGE_SIZE) + 1;

  if (totalPages <= 1) { el.innerHTML = ''; return; }

  let html = `<div class="page-info">Page ${currentPage} of ${totalPages} (${total} events)</div>
    <div class="page-controls">
      <button class="btn btn-sm" data-page="prev" ${_eventsState.offset <= 0 ? 'disabled' : ''}>← Prev</button>`;

  const start = Math.max(1, currentPage - 2);
  const end = Math.min(totalPages, currentPage + 2);
  for (let i = start; i <= end; i++) {
    const p = i - 1;
    html += `<button class="btn btn-sm ${p * PAGE_SIZE === _eventsState.offset ? 'btn-primary' : ''}" data-page="${p * PAGE_SIZE}">${i}</button>`;
  }

  html += `<button class="btn btn-sm" data-page="next" ${_eventsState.offset + PAGE_SIZE >= total ? 'disabled' : ''}>Next →</button></div>`;
  el.innerHTML = html;

  el.querySelectorAll('[data-page]').forEach(btn => {
    btn.addEventListener('click', () => {
      let newOffset;
      const val = btn.getAttribute('data-page');
      if (val === 'prev') newOffset = Math.max(0, _eventsState.offset - PAGE_SIZE);
      else if (val === 'next') newOffset = Math.min(total - 1, _eventsState.offset + PAGE_SIZE);
      else newOffset = parseInt(val);
      if (!isNaN(newOffset) && newOffset !== _eventsState.offset) {
        _eventsState.offset = newOffset;
        fetchEventsPage(id);
        $$('.tab-item').forEach(t => t.classList.remove('active'));
      }
    });
  });
}

/* --- Raw tab ------------------------------------------------- */
async function loadRawTab(id) {
  try {
    const res = await fetch(`${API}/api/uploaded/${id}`);
    if (!res.ok) {
      $('#tab-raw').innerHTML = '<div class="empty-state"><h3>Raw file not available</h3><p>The uploaded file could not be loaded.</p></div>';
      return;
    }
    const text = await res.text();
    const lines = text.split('\n');
    let html = '<div class="raw-view">';
    for (let i = 0; i < lines.length; i++) {
      html += `<div class="raw-line"><span class="raw-line-num">${i + 1}</span><span class="raw-line-text">${escapeHtml(lines[i])}</span></div>`;
    }
    html += '</div>';
    $('#tab-raw').innerHTML = `<div class="card"><div class="card-header"><h3>Original File (${lines.length} lines)</h3></div>${html}</div>`;
  } catch (_) {
    $('#tab-raw').innerHTML = '<div class="empty-state"><h3>Failed to load raw file</h3></div>';
  }
}

/* --- AI Insights tab ----------------------------------------- */
function loadInsightsTab(id) {
  const el = $('#tab-insights');
  if (!el) return;

  el.innerHTML = '<div class="card"><div class="empty-state"><div class="spinner" style="width:16px;height:16px;border:2px solid var(--border);border-top-color:var(--accent);border-radius:50%;animation:spin 0.8s linear infinite;margin:0 auto 12px"></div><h3>Analyzing with AI...</h3></div></div>';

  const evtSource = new EventSource(`${API}/api/insights/${id}/stream`);
  let fullText = '';

  evtSource.addEventListener('message', function(e) {
    try {
      const data = JSON.parse(e.data);
      if (data.type === 'text') {
        fullText += data.content;
        el.innerHTML = `<div class="card">
          <div class="card-header"><h3>AI Insights</h3></div>
          <div class="insights-text">${renderMarkdown(fullText)}<span class="cursor-blink">▌</span></div>
        </div>`;
      }
    } catch (_) {}
  });

  evtSource.addEventListener('complete', function() {
    evtSource.close();
    el.innerHTML = `<div class="card">
      <div class="card-header"><h3>AI Insights</h3></div>
      <div class="insights-text">${renderMarkdown(fullText)}</div>
    </div>`;
  });

  evtSource.addEventListener('error', async function(e) {
    evtSource.close();
    if (!fullText) {
      // Check REST endpoint in case background auto-generation cached it
      try {
        const data = await fetchJSON(`${API}/api/insights/${id}`);
        if (data.summary) {
          el.innerHTML = `<div class="card">
            <div class="card-header"><h3>AI Insights</h3></div>
            <div class="insights-text">${renderMarkdown(data.summary)}</div>
          </div>`;
          return;
        }
      } catch (_) {}

      // Use the actual SSE error message when available
      let msg = 'AI summarizer may not be configured. Ensure --ai-endpoint is set. Try again or contact admin.';
      try {
        const d = JSON.parse(e.data);
        if (d.content) msg = d.content;
      } catch (_) {}
      el.innerHTML = `<div class="card">
        <div class="card-header"><h3>AI Insights</h3></div>
        <div class="empty-state"><h3>Not available</h3>
        <p>${escapeHtml(msg)}</p></div>
      </div>`;
    }
  });
}

/* =============================================================
   Router
   ============================================================= */
let _currentPath = '';

function navigate(path) {
  history.pushState(null, '', path);
  renderPage();
}

function navClick(e) {
  e.preventDefault();
  const href = e.currentTarget.getAttribute('href');
  if (href) navigate(href);
}

function renderPage() {
  const path = location.pathname;
  _currentPath = path;

  if (path === '/' || path === '') {
    renderDashboard();
  } else if (path === '/upload') {
    renderUpload();
  } else if (path.startsWith('/session/')) {
    const id = path.split('/')[2];
    if (id) renderSession(id);
  } else {
    const app = $('#app');
    app.innerHTML = renderLayout(`
      <div class="empty-state">
        <h3>Page not found</h3>
        <p>The page you're looking for doesn't exist.</p>
        <a href="/" class="btn btn-primary" data-nav>Go to Dashboard</a>
      </div>`, path);
    $$('[data-nav]').forEach(el => el.addEventListener('click', navClick));
  }
}

/* --- Keyboard shortcuts -------------------------------------- */
document.addEventListener('keydown', (e) => {
  if (e.metaKey || e.ctrlKey) {
    switch (e.key) {
      case '1': e.preventDefault(); navigate('/'); break;
      case 'u': e.preventDefault(); navigate('/upload'); break;
    }
  }
  if (e.key === 'Escape') {
    const sidebar = $('#sidebar');
    if (sidebar && sidebar.classList.contains('open')) {
      sidebar.classList.remove('open');
    }
  }
});

/* --- Navigation links + theme toggle (delegated) --------------- */
document.addEventListener('click', (e) => {
  const link = e.target.closest('[data-nav]');
  if (link) {
    e.preventDefault();
    const href = link.getAttribute('href');
    if (href) navigate(href);
    return;
  }
  if (e.target.closest('#theme-btn')) {
    toggleTheme();
  }
});

/* --- History API ---------------------------------------------- */
window.addEventListener('popstate', renderPage);

/* --- Init ---------------------------------------------------- */
renderPage();

/* Inject keyframe animation for spinner */
const style = document.createElement('style');
style.textContent = '@keyframes spin { to { transform: rotate(360deg); } }';
document.head.appendChild(style);
