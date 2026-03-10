'use strict';

const DIAGNOSIS_COLORS = {
  whitelist_active:  '#C44D4D',
  no_internet:       '#D4943A',
  normal_filtering:  '#4A8B6E',
  everything_works:  '#415A80',
  not_in_russia:     '#A5D4DC',
};

const DIAGNOSIS_LABELS = {
  whitelist_active:  'Белый список активен',
  no_internet:       'Нет интернета',
  normal_filtering:  'Обычная фильтрация',
  everything_works:  'Всё работает',
  not_in_russia:     'Не в России',
};

// ── Map init ──────────────────────────────────────────────────────────────────

const map = L.map('map', {
  center: [62, 90],
  zoom: 4,
  minZoom: 3,
  maxZoom: 14,
  zoomControl: true,
});

L.tileLayer('https://{s}.basemaps.cartocdn.com/light_nolabels/{z}/{x}/{y}{r}.png', {
  attribution: '© <a href="https://openstreetmap.org">OSM</a> © <a href="https://carto.com">CARTO</a>',
  subdomains: 'abcd',
  maxZoom: 19,
}).addTo(map);

// Add a subtle label layer on top (country/city names without background)
L.tileLayer('https://{s}.basemaps.cartocdn.com/light_only_labels/{z}/{x}/{y}{r}.png', {
  attribution: '',
  subdomains: 'abcd',
  maxZoom: 19,
  pane: 'shadowPane', // renders above markers would need custom pane; skip for now
  opacity: 0.5,
}).addTo(map);

const circleLayer = L.layerGroup().addTo(map);

// ── Helpers ───────────────────────────────────────────────────────────────────

function radiusForCount(count, zoom) {
  const base = 5 + Math.log10(Math.max(count, 1)) * 6;
  const zoomScale = Math.max(0.5, zoom / 8);
  return base * zoomScale;
}

function formatCount(n) {
  if (n >= 1000) return (n / 1000).toFixed(1) + 'k';
  return String(n);
}

// ── Data loading ──────────────────────────────────────────────────────────────

let loadingTimer = null;

function showLoading() {
  document.getElementById('loading').classList.remove('hidden');
}

function hideLoading() {
  document.getElementById('loading').classList.add('hidden');
}

async function loadMapData() {
  const bounds = map.getBounds();
  const zoom   = map.getZoom();
  const params = new URLSearchParams({
    zoom,
    bounds: [
      bounds.getSouth().toFixed(4),
      bounds.getWest().toFixed(4),
      bounds.getNorth().toFixed(4),
      bounds.getEast().toFixed(4),
    ].join(','),
  });

  showLoading();
  try {
    const res = await fetch(`/api/map?${params}`);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const { cells } = await res.json();
    renderCells(cells || [], zoom);
  } catch (err) {
    console.error('map load error:', err);
  } finally {
    hideLoading();
  }
}

function renderCells(cells, zoom) {
  circleLayer.clearLayers();

  cells.forEach(cell => {
    const color  = DIAGNOSIS_COLORS[cell.diagnosis] || '#999';
    const radius = radiusForCount(cell.count, zoom);
    const label  = DIAGNOSIS_LABELS[cell.diagnosis] || cell.diagnosis;

    L.circleMarker([cell.lat, cell.lon], {
      radius,
      color,
      fillColor:   color,
      fillOpacity: 0.55,
      weight:      1.5,
      opacity:     0.85,
    })
    .bindPopup(`
      <div style="text-align:center;padding:4px 0">
        <div style="font-weight:700;font-size:13px;color:${color}">${label}</div>
        <div style="margin-top:4px;color:#6B6B7E;font-size:11px">${formatCount(cell.count)} ${pluralReport(cell.count)}</div>
      </div>
    `)
    .addTo(circleLayer);
  });

  // Show/hide empty state
  const emptyState = document.getElementById('empty-state');
  if (emptyState) {
    if (cells.length === 0) {
      emptyState.classList.add('visible');
    } else {
      emptyState.classList.remove('visible');
    }
  }
}

function pluralReport(n) {
  const mod10 = n % 10;
  const mod100 = n % 100;
  if (mod10 === 1 && mod100 !== 11) return 'отчёт';
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) return 'отчёта';
  return 'отчётов';
}

// ── Stats ─────────────────────────────────────────────────────────────────────

async function loadStats() {
  try {
    const res = await fetch('/api/stats');
    if (!res.ok) return;
    const { total, last24h } = await res.json();
    document.getElementById('stat-total').textContent = total.toLocaleString('ru');
    document.getElementById('stat-24h').textContent   = last24h.toLocaleString('ru');
  } catch (err) {
    console.error('stats error:', err);
  }
}

// ── Events ────────────────────────────────────────────────────────────────────

let moveDebounce;
map.on('moveend zoomend', () => {
  clearTimeout(moveDebounce);
  moveDebounce = setTimeout(loadMapData, 300);
});

// Auto-refresh every 3 minutes
setInterval(() => {
  loadMapData();
  loadStats();
}, 3 * 60 * 1000);

// ── Initial load ──────────────────────────────────────────────────────────────

loadMapData();
loadStats();
