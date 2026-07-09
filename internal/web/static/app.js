const API = '';

let currentPage = 'home';

function navigate(path) {
  history.pushState(null, '', path);
  render();
}

window.addEventListener('popstate', render);
document.addEventListener('click', (e) => {
  const link = e.target.closest('[data-nav]');
  if (link) {
    e.preventDefault();
    navigate(link.getAttribute('href'));
  }
});

async function render() {
  const path = location.pathname;
  const main = document.getElementById('main');
  if (!main) return;

  if (path === '/upload') {
    await renderUpload(main);
  } else if (path.startsWith('/session/')) {
    const id = path.split('/')[2];
    await renderSession(main, id);
  } else {
    await renderHome(main);
  }
}

async function renderHome(main) {
  main.innerHTML = '<div class="card"><h2>Sessions</h2><div id="session-list">Loading...</div></div>';
  try {
    const res = await fetch(`${API}/api/sessions`);
    const data = await res.json();
    const list = data.sessions || [];
    if (list.length === 0) {
      document.getElementById('session-list').innerHTML = '<div class="empty-state">No sessions yet. <a href="/upload" data-nav>Upload a log file</a> to get started.</div>';
      document.querySelectorAll('[data-nav]').forEach(el => el.addEventListener('click', (e) => { e.preventDefault(); navigate(el.getAttribute('href')); }));
      return;
    }
    let html = '<table><thead><tr><th>File</th><th>Status</th><th>Created</th><th>Actions</th></tr></thead><tbody>';
    for (const s of list) {
      html += `<tr>
        <td>${escapeHtml(s.file_name)}</td>
        <td><span class="badge badge-${s.status}">${s.status}</span></td>
        <td>${formatTime(s.created_at)}</td>
        <td>
          <a href="/session/${s.id}" data-nav class="btn btn-sm">View</a>
          <button class="btn btn-sm btn-danger" onclick="deleteSession('${s.id}')">Delete</button>
        </td>
      </tr>`;
    }
    html += '</tbody></table>';
    document.getElementById('session-list').innerHTML = html;
    document.querySelectorAll('[data-nav]').forEach(el => el.addEventListener('click', (e) => { e.preventDefault(); navigate(el.getAttribute('href')); }));
  } catch (err) {
    document.getElementById('session-list').innerHTML = `<div class="error-message">Failed to load sessions: ${err.message}</div>`;
  }
}

async function renderUpload(main) {
  main.innerHTML = `
    <div class="card">
      <h2>Upload Log File</h2>
      <div class="upload-zone" id="drop-zone">
        <p>Drag & drop a log file here, or click to select</p>
        <input type="file" id="file-input" accept=".log,.txt,.csv,.gz" style="display:none">
        <button class="btn" id="select-btn">Select File</button>
      </div>
    </div>
    <div class="card" id="options-card" style="display:none">
      <h2>Analysis Options</h2>
      <div class="upload-options">
        <label>Command:</label>
        <select id="cmd">
          <option value="scan">scan — full report</option>
          <option value="errors">errors — error lines</option>
          <option value="top">top — top errors</option>
          <option value="grep">grep — regex search</option>
        </select>
        <label>Level:</label>
        <select id="level">
          <option value="">all</option>
          <option value="error">error+</option>
          <option value="warn">warn+</option>
          <option value="info">info+</option>
        </select>
        <label>Regex:</label>
        <input type="text" id="regex" placeholder="e.g. timeout|error">
        <label>Limit:</label>
        <input type="number" id="limit" value="10" min="1" max="1000" style="width:60px">
        <label>Since:</label>
        <input type="text" id="since" placeholder="e.g. 1h, 30m">
      </div>
      <button class="btn" id="analyze-btn" style="margin-top:1rem" disabled>Analyze</button>
    </div>
    <div id="result"></div>
  `;

  const dropZone = document.getElementById('drop-zone');
  const fileInput = document.getElementById('file-input');
  const selectBtn = document.getElementById('select-btn');
  const optionsCard = document.getElementById('options-card');
  const analyzeBtn = document.getElementById('analyze-btn');

  let uploadedFile = null;

  dropZone.addEventListener('click', () => fileInput.click());
  selectBtn.addEventListener('click', (e) => { e.stopPropagation(); fileInput.click(); });

  dropZone.addEventListener('dragover', (e) => { e.preventDefault(); dropZone.classList.add('drag-over'); });
  dropZone.addEventListener('dragleave', () => dropZone.classList.remove('drag-over'));
  dropZone.addEventListener('drop', (e) => {
    e.preventDefault();
    dropZone.classList.remove('drag-over');
    if (e.dataTransfer.files.length > 0) {
      handleFile(e.dataTransfer.files[0]);
    }
  });

  fileInput.addEventListener('change', () => {
    if (fileInput.files.length > 0) handleFile(fileInput.files[0]);
  });

  function handleFile(file) {
    uploadedFile = file;
    dropZone.innerHTML = `<p style="color:#3fb950">Selected: ${escapeHtml(file.name)} (${formatBytes(file.size)})</p>`;
    optionsCard.style.display = 'block';
  }

  analyzeBtn.addEventListener('click', async () => {
    if (!uploadedFile) return;
    analyzeBtn.disabled = true;
    analyzeBtn.textContent = 'Uploading...';
    document.getElementById('result').innerHTML = '';

    const formData = new FormData();
    formData.append('file', uploadedFile);

    try {
      const uploadRes = await fetch(`${API}/api/upload`, { method: 'POST', body: formData });
      const uploadData = await uploadRes.json();
      const sessionId = uploadData.session_id;

      const cmd = document.getElementById('cmd').value;
      const level = document.getElementById('level').value;
      const regex = document.getElementById('regex').value;
      const limit = parseInt(document.getElementById('limit').value) || 10;
      const since = document.getElementById('since').value;

      const analyzeRes = await fetch(`${API}/api/analyze/${sessionId}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ command: cmd, level, regex, limit, since }),
      });
      const analyzeData = await analyzeRes.json();

      if (analyzeData.status === 'running') {
        pollStatus(sessionId, cmd, document.getElementById('result'));
      }
    } catch (err) {
      document.getElementById('result').innerHTML = `<div class="error-message">Upload failed: ${err.message}</div>`;
      analyzeBtn.disabled = false;
      analyzeBtn.textContent = 'Analyze';
    }
  });
}

function pollStatus(sessionId, cmd, resultEl) {
  resultEl.innerHTML = `
    <div class="card">
      <div class="status-bar">
        <div class="spinner"></div>
        <span id="progress-text">Analyzing...</span>
      </div>
    </div>
  `;

  const evtSource = new EventSource(`${API}/api/status/${sessionId}`);
  evtSource.addEventListener('progress', (e) => {
    const data = JSON.parse(e.data);
    document.getElementById('progress-text').textContent = data.progress || 'Analyzing...';
  });
  evtSource.addEventListener('complete', () => {
    evtSource.close();
    fetchResults(sessionId, cmd, resultEl);
  });
  evtSource.addEventListener('error', () => {
    evtSource.close();
    document.getElementById('progress-text').textContent = 'Analysis complete';
    fetchResults(sessionId, cmd, resultEl);
  });
}

async function fetchResults(sessionId, cmd, resultEl) {
  try {
    const res = await fetch(`${API}/api/results/${sessionId}`);
    const data = await res.json();
    if (data.status === 'error') {
      resultEl.innerHTML = `<div class="error-message">Analysis error: ${data.error || 'unknown'}</div>`;
      return;
    }

    if ((cmd === 'scan' || cmd === 'top') && data.report) {
      renderReport(resultEl, data.report, cmd);
    } else if ((cmd === 'errors' || cmd === 'grep') && data.events) {
      renderEvents(resultEl, data.events);
    } else {
      resultEl.innerHTML = '<div class="error-message">No results available</div>';
    }

    const nav = document.querySelector('[data-nav="session-link"]');
    if (nav) {
      nav.href = `/session/${sessionId}`;
      nav.textContent = 'View session';
    }
  } catch (err) {
    resultEl.innerHTML = `<div class="error-message">Failed to load results: ${err.message}</div>`;
  }
}

function renderReport(el, report, cmd) {
  const levels = report.levels || {};
  const top = report.top_errors || [];
  const total = report.total_lines || 0;
  const tr = report.time_range || {};

  let levelRows = '';
  for (const [level, count] of Object.entries(levels)) {
    const pct = total > 0 ? ((count / total) * 100).toFixed(1) : '0.0';
    levelRows += `<tr><td><span class="event-level event-level-${level}">${level}</span></td><td>${count}</td><td>${pct}%</td></tr>`;
  }

  let topHtml = '';
  for (let i = 0; i < top.length; i++) {
    const g = top[i];
    const first = g.first_seen ? formatTimeShort(g.first_seen) : '';
    const last = g.last_seen ? formatTimeShort(g.last_seen) : '';
    topHtml += `<div class="group-item"><span class="group-count">${g.count}x</span><span class="group-sig">${escapeHtml(g.signature)}</span><span class="group-range">[${first} - ${last}]</span></div>`;
  }

  const timeRangeHtml = tr.first ? `<div class="stat-label">${formatTime(tr.first)} — ${formatTime(tr.last)} (${tr.duration_sec}s)</div>` : '';

  el.innerHTML = `
    <div class="report-grid">
      <div class="card">
        <h2>${escapeHtml(report.source || 'log')}</h2>
        <div class="stat">${total}</div>
        <div class="stat-label">Total lines</div>
        ${timeRangeHtml}
      </div>
      <div class="card">
        <h2>Level Breakdown</h2>
        <table><thead><tr><th>Level</th><th>Count</th><th>%</th></tr></thead><tbody>${levelRows}</tbody></table>
      </div>
    </div>
    ${topHtml ? `<div class="card"><h2>Top Errors</h2>${topHtml}</div>` : ''}
    <div style="margin-top:1rem"><a href="/session/${location.pathname.split('/')[2]}" data-nav class="btn">View Full Session</a></div>
  `;
}

function renderEvents(el, events) {
  if (events.length === 0) {
    el.innerHTML = '<div class="card"><div class="empty-state">No matching events found.</div></div>';
    return;
  }
  let html = `<div class="card"><h2>${events.length} matching events</h2>`;
  for (const evt of events) {
    const ts = evt.timestamp ? formatTimeShort(evt.timestamp) : '';
    html += `<div class="event-line">
      <span class="event-ts">${ts}</span>
      <span class="event-level event-level-${evt.level}">${evt.level}</span>
      ${escapeHtml(evt.message)}
    </div>`;
  }
  html += '</div>';
  el.innerHTML = html;
}

async function renderSession(main, id) {
  main.innerHTML = '<div class="card"><h2>Loading...</h2></div>';
  try {
    const res = await fetch(`${API}/api/results/${id}`);
    const data = await res.json();

    if (!data || data.status === 'error' || data.status === 'uploaded') {
      main.innerHTML = `<div class="card">
        <h2>Session ${id}</h2>
        ${data.status === 'uploaded' ? '<p>Analysis not yet started.</p><button class="btn" onclick="navigate(\'/\')">Back to Home</button>' : `<div class="error-message">${data.error || 'Session not found'}</div>`}
      </div>`;
      return;
    }

    if (data.report) {
      const reportEl = document.createElement('div');
      renderReport(reportEl, data.report, 'scan');
      main.innerHTML = `<div class="card"><h2>Session: ${escapeHtml(data.report.source || id)}</h2></div>`;
      main.appendChild(reportEl.querySelector('.report-grid'));
      const top = reportEl.querySelector('.card:last-child');
      if (top) main.appendChild(top);
      if (data.events && data.events.length > 0) {
        const evtEl = document.createElement('div');
        renderEvents(evtEl, data.events);
        main.appendChild(evtEl);
      }
    } else if (data.events) {
      renderEvents(main, data.events);
    } else {
      main.innerHTML = '<div class="card"><div class="empty-state">No results available.</div></div>';
    }
  } catch (err) {
    main.innerHTML = `<div class="card"><div class="error-message">Failed to load session: ${err.message}</div></div>`;
  }
}

async function deleteSession(id) {
  if (!confirm('Delete this session?')) return;
  try {
    await fetch(`${API}/api/sessions/${id}`, { method: 'DELETE' });
    navigate('/');
  } catch (err) {
    alert('Failed to delete: ' + err.message);
  }
}

function escapeHtml(s) {
  if (!s) return '';
  const div = document.createElement('div');
  div.textContent = s;
  return div.innerHTML;
}

function formatTime(iso) {
  if (!iso) return '';
  const d = new Date(iso);
  return d.toLocaleString();
}

function formatTimeShort(iso) {
  if (!iso) return '';
  const d = new Date(iso);
  return d.toLocaleTimeString();
}

function formatBytes(bytes) {
  if (bytes < 1024) return bytes + ' B';
  if (bytes < 1048576) return (bytes/1024).toFixed(1) + ' KB';
  return (bytes/1048576).toFixed(1) + ' MB';
}

render();
