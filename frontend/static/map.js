'use strict';

const DIAGNOSIS_COLORS = {
  whitelist_active:  '#C44D4D',
  normal_filtering:  '#4A8B6E',
  everything_works:  '#415A80',
};

const DIAGNOSIS_LABELS = {
  whitelist_active:  'Белый список активен',
  normal_filtering:  'Обычная фильтрация',
  everything_works:  'Всё работает',
};

// ── Map init ──────────────────────────────────────────────────────────────────

const RUSSIA_BOUNDS = L.latLngBounds([39.0, 17.0], [83.0, 193.0]);

const map = L.map('map', {
  center: [62, 90],
  zoom: 4,
  minZoom: 3,
  maxZoom: 16,
  zoomControl: true,
  maxBounds: RUSSIA_BOUNDS,
  maxBoundsViscosity: 0.85,
  preferCanvas: false,  // SVG needed for filter blur
});

L.tileLayer('https://{s}.basemaps.cartocdn.com/light_nolabels/{z}/{x}/{y}{r}.png', {
  attribution: '© <a href="https://openstreetmap.org">OSM</a> © <a href="https://carto.com">CARTO</a>',
  subdomains: 'abcd',
  maxZoom: 19,
}).addTo(map);

// ── Fog pane ──────────────────────────────────────────────────────────────────
// A separate SVG pane with CSS blur — creates the atmospheric fog look.

map.createPane('fogPane');
const fogPaneEl = map.getPane('fogPane');
fogPaneEl.style.zIndex = 400;
fogPaneEl.style.filter = 'blur(8px)';
fogPaneEl.style.opacity = '1';

// Label pane sits above fog, no blur
map.createPane('labelPane');
map.getPane('labelPane').style.zIndex = 450;
map.getPane('labelPane').style.pointerEvents = 'none';

const fogLayer   = L.layerGroup().addTo(map);
const hitLayer   = L.layerGroup().addTo(map);   // transparent interactive circles for popups

// ── Helpers ───────────────────────────────────────────────────────────────────

// Grid size matches server-side gridSize()
function gridSizeForZoom(zoom) {
  if (zoom <= 4)  return 2.0;
  if (zoom <= 6)  return 1.0;
  if (zoom <= 8)  return 0.5;
  if (zoom <= 11) return 0.1;
  return 0.02;
}

// Fog circle radius in meters.
// Covers roughly half the grid cell so adjacent points naturally overlap and merge.
function fogRadiusMeters(zoom, count) {
  const gs = gridSizeForZoom(zoom);
  // 1° lat ≈ 111 km; use 22% of half-cell — tight fog, blur does the rest
  const base = gs * 111000 * 0.22;
  // Scale slightly with count so denser areas glow more
  const scale = 1 + Math.min(Math.log10(Math.max(count, 1)) * 0.2, 0.5);
  return Math.max(base * scale, 350);
}

// Opacity scales with count: lone point is subtle, cluster is vivid
function fogOpacity(count) {
  return Math.min(0.42 + Math.log10(Math.max(count, 1)) * 0.2, 0.82);
}

function formatCount(n) {
  if (n >= 1000) return (n / 1000).toFixed(1) + 'k';
  return String(n);
}

function pluralReport(n) {
  const m10 = n % 10, m100 = n % 100;
  if (m10 === 1 && m100 !== 11) return 'отчёт';
  if (m10 >= 2 && m10 <= 4 && (m100 < 10 || m100 >= 20)) return 'отчёта';
  return 'отчётов';
}

// ── Rendering ─────────────────────────────────────────────────────────────────

function renderCells(cells, zoom) {
  fogLayer.clearLayers();
  hitLayer.clearLayers();

  cells.forEach(cell => {
    const color   = DIAGNOSIS_COLORS[cell.diagnosis] || '#888';
    const label   = DIAGNOSIS_LABELS[cell.diagnosis] || cell.diagnosis;
    const radius  = fogRadiusMeters(zoom, cell.count);
    const opacity = fogOpacity(cell.count);
    const latlng  = [cell.lat, cell.lon];

    // Fog blob — blurred, visual only, not interactive
    L.circle(latlng, {
      pane:        'fogPane',
      radius,
      stroke:      false,
      fillColor:   color,
      fillOpacity: opacity,
      interactive: false,
    }).addTo(fogLayer);

    // Invisible hit area — same radius, transparent, catches clicks for popup
    const popup = `
      <div style="text-align:center;padding:4px 2px">
        <div style="width:10px;height:10px;border-radius:50%;background:${color};margin:0 auto 6px"></div>
        <div style="font-weight:700;font-size:13px;color:${color}">${label}</div>
        <div style="margin-top:4px;color:#6B6B7E;font-size:11px">${formatCount(cell.count)} ${pluralReport(cell.count)}</div>
      </div>`;

    L.circle(latlng, {
      radius,
      stroke:      false,
      fillColor:   color,
      fillOpacity: 0,
      interactive: true,
    })
    .bindPopup(popup)
    .addTo(hitLayer);
  });

  const emptyState = document.getElementById('empty-state');
  if (emptyState) {
    emptyState.classList.toggle('visible', cells.length === 0);
  }
}

// ── Data loading ──────────────────────────────────────────────────────────────

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

  document.getElementById('loading').classList.remove('hidden');
  try {
    const res = await fetch(`/api/map?${params}`);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const { cells } = await res.json();
    renderCells(cells || [], zoom);
  } catch (err) {
    console.error('map load error:', err);
  } finally {
    document.getElementById('loading').classList.add('hidden');
  }
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

setInterval(() => { loadMapData(); loadStats(); }, 3 * 60 * 1000);

// ── VPN Notice ────────────────────────────────────────────────────────────────

(function () {
  const notice   = document.getElementById('vpn-notice');
  const closeBtn = document.getElementById('vpn-notice-close');
  if (!notice || !closeBtn) return;

  function updateNoticeHeight() {
    const h = notice.classList.contains('hidden') ? 0 : notice.offsetHeight;
    document.documentElement.style.setProperty('--notice-h', h + 'px');
    map.invalidateSize();
  }

  updateNoticeHeight();

  closeBtn.addEventListener('click', () => {
    notice.classList.add('hidden');
    setTimeout(updateNoticeHeight, 320);
    sessionStorage.setItem('vpn-notice-dismissed', '1');
  });

  if (sessionStorage.getItem('vpn-notice-dismissed')) {
    notice.classList.add('hidden');
    setTimeout(updateNoticeHeight, 0);
  }
})();

// ── Initial load ──────────────────────────────────────────────────────────────

loadMapData();
loadStats();
