/*
 * Copyright 2026 Philterd, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package dashboard

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (d *Dashboard) serveUI(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(dashboardHTML))
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Phield Dashboard</title>
<script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.7/dist/chart.umd.min.js"></script>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #0f1419; color: #e1e8ed; min-height: 100vh; }
.header { background: #1a2332; border-bottom: 1px solid #2d3748; padding: 16px 24px; display: flex; align-items: center; justify-content: space-between; }
.header h1 { font-size: 20px; font-weight: 600; color: #fff; }
.header h1 span { color: #4da6ff; }
.controls { display: flex; gap: 12px; align-items: center; }
.controls select { background: #2d3748; color: #e1e8ed; border: 1px solid #4a5568; border-radius: 6px; padding: 6px 12px; font-size: 13px; cursor: pointer; }
.controls button { background: #4da6ff; color: #fff; border: none; border-radius: 6px; padding: 6px 16px; font-size: 13px; cursor: pointer; font-weight: 500; }
.controls button:hover { background: #3b8fd4; }
.status { font-size: 12px; color: #718096; }
.grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 16px; padding: 24px; }
.stat-card { background: #1a2332; border: 1px solid #2d3748; border-radius: 8px; padding: 16px; }
.stat-card .label { font-size: 11px; text-transform: uppercase; letter-spacing: 0.5px; color: #718096; margin-bottom: 4px; }
.stat-card .value { font-size: 28px; font-weight: 700; color: #fff; }
.stat-card .value.alert { color: #fc8181; }
.charts { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; padding: 0 24px 24px; }
.chart-card { background: #1a2332; border: 1px solid #2d3748; border-radius: 8px; padding: 16px; }
.chart-card h3 { font-size: 14px; font-weight: 500; margin-bottom: 12px; color: #a0aec0; }
.chart-card canvas { max-height: 250px; }
.alerts-section { padding: 0 24px 24px; }
.alerts-card { background: #1a2332; border: 1px solid #2d3748; border-radius: 8px; padding: 16px; }
.alerts-card h3 { font-size: 14px; font-weight: 500; margin-bottom: 12px; color: #a0aec0; }
.alert-row { display: grid; grid-template-columns: 160px 120px 1fr 100px 80px; gap: 12px; padding: 8px 0; border-bottom: 1px solid #2d3748; font-size: 13px; align-items: center; }
.alert-row:last-child { border-bottom: none; }
.alert-row .time { color: #718096; }
.alert-row .type { color: #fc8181; font-weight: 500; }
.alert-row .context { color: #a0aec0; }
.alert-row .count { color: #fff; font-weight: 600; text-align: right; }
.alert-row .zscore { color: #fbd38d; text-align: right; }
.empty { color: #4a5568; font-style: italic; padding: 16px 0; }
.flows-section { padding: 0 24px 24px; }
.flow-card { background: #1a2332; border: 1px solid #2d3748; border-radius: 8px; padding: 16px; }
.flow-card h3 { font-size: 14px; font-weight: 500; margin-bottom: 12px; color: #a0aec0; }
.flow-row { display: flex; align-items: center; gap: 12px; padding: 8px 0; border-bottom: 1px solid #2d3748; font-size: 13px; }
.flow-row:last-child { border-bottom: none; }
.flow-source { color: #4da6ff; font-weight: 500; min-width: 120px; }
.flow-arrow { color: #4a5568; }
.flow-context { color: #68d391; min-width: 120px; }
.flow-types { color: #a0aec0; flex: 1; }
.flow-total { color: #fff; font-weight: 600; min-width: 60px; text-align: right; }
.trends-section { padding: 0 24px 24px; }
.trend-card { background: #1a2332; border: 1px solid #2d3748; border-radius: 8px; padding: 16px; }
.trend-card h3 { font-size: 14px; font-weight: 500; margin-bottom: 12px; color: #a0aec0; }
.trend-row { display: grid; grid-template-columns: 120px 120px 1fr 100px 100px 80px; gap: 12px; padding: 8px 0; border-bottom: 1px solid #2d3748; font-size: 13px; align-items: center; }
.trend-row:last-child { border-bottom: none; }
.trend-row .source { color: #4da6ff; }
.trend-row .pii-type { color: #a0aec0; }
.trend-row .context { color: #68d391; }
.trend-row .baseline { color: #718096; text-align: right; }
.trend-row .recent { color: #fff; text-align: right; }
.trend-row .deviation { text-align: right; font-weight: 600; }
.deviation.up { color: #fc8181; }
.deviation.down { color: #68d391; }
.deviation.flat { color: #718096; }
@media (max-width: 900px) { .charts { grid-template-columns: 1fr; } }
</style>
</head>
<body>
<div class="header">
  <h1><span>Phield</span> Dashboard</h1>
  <div class="controls">
    <select id="timeRange">
      <option value="1">Last 1 hour</option>
      <option value="6">Last 6 hours</option>
      <option value="24" selected>Last 24 hours</option>
      <option value="72">Last 3 days</option>
      <option value="168">Last 7 days</option>
    </select>
    <button onclick="refresh()">Refresh</button>
    <span class="status" id="status">Loading...</span>
  </div>
</div>

<div class="grid" id="summary"></div>

<div class="charts">
  <div class="chart-card">
    <h3>PII Entity Types</h3>
    <canvas id="entityChart"></canvas>
  </div>
  <div class="chart-card">
    <h3>PII Volume Over Time</h3>
    <canvas id="timelineChart"></canvas>
  </div>
</div>

<div class="trends-section">
  <div class="trend-card">
    <h3>Baseline vs. Current Traffic</h3>
    <div id="trends"></div>
  </div>
</div>

<div class="flows-section">
  <div class="flow-card">
    <h3>PII Flows (Source &rarr; Context)</h3>
    <div id="flows"></div>
  </div>
</div>

<div class="alerts-section">
  <div class="alerts-card">
    <h3>Alert Timeline</h3>
    <div id="alerts"></div>
  </div>
</div>

<script>
let entityChart = null;
let timelineChart = null;
let refreshInterval = null;

const colors = ['#4da6ff','#fc8181','#68d391','#fbd38d','#b794f4','#f687b3','#63b3ed','#4fd1c5'];

function getHours() {
  return document.getElementById('timeRange').value;
}

async function fetchJSON(url) {
  const resp = await fetch(url);
  if (!resp.ok) throw new Error(resp.statusText);
  return resp.json();
}

async function loadSummary() {
  const data = await fetchJSON('/api/dashboard/summary?hours=' + getHours());
  document.getElementById('summary').innerHTML =
    card('Data Points', data.total_entries) +
    card('Alerts', data.total_breaches, data.total_breaches > 0) +
    card('Sources', data.unique_sources) +
    card('PII Types', data.unique_types) +
    card('Contexts', data.unique_contexts);
}

function card(label, value, isAlert) {
  return '<div class="stat-card"><div class="label">' + label + '</div><div class="value' + (isAlert ? ' alert' : '') + '">' + value + '</div></div>';
}

async function loadEntities() {
  const data = await fetchJSON('/api/dashboard/entities?hours=' + getHours());
  const labels = Object.keys(data.totals || {});
  const values = labels.map(l => data.totals[l]);

  if (entityChart) entityChart.destroy();
  const ctx = document.getElementById('entityChart').getContext('2d');
  entityChart = new Chart(ctx, {
    type: 'doughnut',
    data: {
      labels: labels,
      datasets: [{
        data: values,
        backgroundColor: colors.slice(0, labels.length),
        borderWidth: 0
      }]
    },
    options: {
      responsive: true,
      plugins: { legend: { position: 'right', labels: { color: '#a0aec0', font: { size: 11 } } } }
    }
  });

  // Timeline chart
  const timeline = data.timeline || {};
  const allTimes = new Set();
  Object.values(timeline).forEach(points => points.forEach(p => allTimes.add(p.time)));
  const sortedTimes = Array.from(allTimes).sort();

  const datasets = Object.keys(timeline).map((type, i) => {
    const pointMap = {};
    timeline[type].forEach(p => { pointMap[p.time] = p.count; });
    return {
      label: type,
      data: sortedTimes.map(t => pointMap[t] || 0),
      borderColor: colors[i % colors.length],
      backgroundColor: colors[i % colors.length] + '33',
      fill: true,
      tension: 0.3,
      borderWidth: 2,
      pointRadius: 0
    };
  });

  if (timelineChart) timelineChart.destroy();
  const ctx2 = document.getElementById('timelineChart').getContext('2d');
  timelineChart = new Chart(ctx2, {
    type: 'line',
    data: { labels: sortedTimes.map(t => new Date(t).toLocaleTimeString([], {hour: '2-digit', minute:'2-digit'})), datasets: datasets },
    options: {
      responsive: true,
      interaction: { intersect: false, mode: 'index' },
      scales: {
        x: { ticks: { color: '#718096', maxTicksLimit: 12 }, grid: { color: '#2d3748' } },
        y: { ticks: { color: '#718096' }, grid: { color: '#2d3748' }, beginAtZero: true }
      },
      plugins: { legend: { labels: { color: '#a0aec0', font: { size: 11 } } } }
    }
  });
}

async function loadAlerts() {
  const data = await fetchJSON('/api/dashboard/alerts?hours=' + getHours());
  const el = document.getElementById('alerts');
  const alerts = data.alerts || [];
  if (alerts.length === 0) {
    el.innerHTML = '<div class="empty">No alerts in this time range.</div>';
    return;
  }
  el.innerHTML = alerts.slice(0, 50).map(a =>
    '<div class="alert-row">' +
    '<span class="time">' + new Date(a.timestamp).toLocaleString() + '</span>' +
    '<span class="type">' + a.pii_type + '</span>' +
    '<span class="context">' + a.source_id + ' / ' + a.context + '</span>' +
    '<span class="count">' + a.count + '</span>' +
    '<span class="zscore">' + (a.z_score ? a.z_score.toFixed(2) : '-') + '</span>' +
    '</div>'
  ).join('');
}

async function loadFlows() {
  const data = await fetchJSON('/api/dashboard/flows?hours=' + getHours());
  const el = document.getElementById('flows');
  const flows = (data.flows || []).sort((a, b) => b.total - a.total);
  if (flows.length === 0) {
    el.innerHTML = '<div class="empty">No flow data in this time range.</div>';
    return;
  }
  el.innerHTML = flows.slice(0, 20).map(f =>
    '<div class="flow-row">' +
    '<span class="flow-source">' + f.source + '</span>' +
    '<span class="flow-arrow">&rarr;</span>' +
    '<span class="flow-context">' + f.context + '</span>' +
    '<span class="flow-types">' + Object.keys(f.types).join(', ') + '</span>' +
    '<span class="flow-total">' + f.total.toLocaleString() + '</span>' +
    '</div>'
  ).join('');
}

async function loadTrends() {
  const data = await fetchJSON('/api/dashboard/trends?hours=' + getHours());
  const el = document.getElementById('trends');
  const trends = (data.trends || []).sort((a, b) => Math.abs(b.deviation_pct) - Math.abs(a.deviation_pct));
  if (trends.length === 0) {
    el.innerHTML = '<div class="empty">No trend data available yet.</div>';
    return;
  }
  el.innerHTML = trends.slice(0, 20).map(t => {
    let devClass = 'flat';
    if (t.deviation_pct > 10) devClass = 'up';
    else if (t.deviation_pct < -10) devClass = 'down';
    return '<div class="trend-row">' +
    '<span class="source">' + t.source_id + '</span>' +
    '<span class="pii-type">' + t.pii_type + '</span>' +
    '<span class="context">' + t.context + '</span>' +
    '<span class="baseline">' + t.baseline_mean.toFixed(1) + '</span>' +
    '<span class="recent">' + t.recent_mean.toFixed(1) + '</span>' +
    '<span class="deviation ' + devClass + '">' + (t.deviation_pct > 0 ? '+' : '') + t.deviation_pct.toFixed(1) + '%</span>' +
    '</div>';
  }).join('');
}

async function refresh() {
  const status = document.getElementById('status');
  status.textContent = 'Refreshing...';
  try {
    await Promise.all([loadSummary(), loadEntities(), loadAlerts(), loadFlows(), loadTrends()]);
    status.textContent = 'Updated ' + new Date().toLocaleTimeString();
  } catch (err) {
    status.textContent = 'Error: ' + err.message;
  }
}

document.getElementById('timeRange').addEventListener('change', refresh);

refresh();
refreshInterval = setInterval(refresh, 30000);
</script>
</body>
</html>`
