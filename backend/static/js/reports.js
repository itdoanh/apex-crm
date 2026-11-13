/* ==============================================================
   APEX ADMIN — TRUNG TÂM BÁO CÁO & BIỂU ĐỒ (thiết kế lại 2026-11-13)
   ==============================================================
   6 phân hệ báo cáo, tất cả dùng CHUNG một bộ lọc toàn cục:
     1. Tổng quan & Hiệu suất Đa Landing
     2. Quảng cáo & Attribution (Meta Ads)
     3. Hành vi Người dùng & Chất lượng Nội dung
     4. Phễu Biểu Mẫu & Tối ưu Chuyển đổi
     5. Thiết bị, Hạ tầng & Trải nghiệm Kỹ thuật
     6. Bảo mật, Chống Bot & Vùng cách ly Spam
   ============================================================== */

// ──────────────────────────────────────────────────────────────
// 1. CROSS-DOMAIN FETCH INTERCEPTOR (Bearer token + 401 handling)
// ──────────────────────────────────────────────────────────────
(function () {
    const originalFetch = window.fetch;
    window.fetch = function (input, init) {
        init = init || {};
        if (!init.credentials) init.credentials = 'include';

        const token = localStorage.getItem('apex_admin_token');
        if (token) {
            if (!init.headers) init.headers = {};
            if (init.headers instanceof Headers) {
                if (!init.headers.has('Authorization')) init.headers.set('Authorization', 'Bearer ' + token);
            } else if (Array.isArray(init.headers)) {
                if (!init.headers.some(h => h[0].toLowerCase() === 'authorization')) {
                    init.headers.push(['Authorization', 'Bearer ' + token]);
                }
            } else if (typeof init.headers === 'object') {
                if (!init.headers['Authorization'] && !init.headers['authorization']) {
                    init.headers['Authorization'] = 'Bearer ' + token;
                }
            }
        }

        let url = typeof input === 'string' ? input : (input && input.url ? input.url : '');
        if (url && url.startsWith('/api/')) {
            const base = (window.APEX_ADMIN_CONFIG && window.APEX_ADMIN_CONFIG.API_BASE_URL) ? window.APEX_ADMIN_CONFIG.API_BASE_URL : '';
            if (base) {
                url = base + url;
                if (typeof input === 'string') input = url;
                else if (input && input.url) input = new Request(url, input);
            }
        }

        return originalFetch.call(this, input, init).then(res => {
            if (res.status === 401 && !url.includes('/api/admin/login')) {
                showRpLogin('Phiên làm việc đã hết hạn hoặc chưa xác thực. Vui lòng đăng nhập lại.');
            }
            return res;
        });
    };
})();

// ──────────────────────────────────────────────────────────────
// 2. STATE
// ──────────────────────────────────────────────────────────────
let rpActiveTab = 'overview';
let rpGranularity = 'day';
let rpHeatmapMetric = 'leads';
let rpSelectedLandings = new Set();
let rpFilterOptions = null;
let rpQuarantineOffset = 0;
const RP_QUARANTINE_LIMIT = 25;

const rpCharts = {};

// ══════════════════════════════════════════════════════════════════════
// TỰ ĐỘNG TẢI LẠI DỮ LIỆU (LIVE) — yêu cầu 2026-11-13
// ══════════════════════════════════════════════════════════════════════
//
// Cho phép chọn chu kỳ tự động load dữ liệu mới nhất: 10s / 30s / 1' / 5' /
// 10' / Tắt.
//
// THIẾT KẾ:
// - Dùng `setInterval` (KHÔNG dùng `setTimeout` lặp) để tránh tràn timer khi
//   tab bị ẩn/hiện liên tục.
// - Timer tạm dừng khi admin đang tương tác bộ lọc, và tự bỏ qua lần gọi khi
//   tab đang ẩn ⇒ không hót băng server khi admin rời bàn phím.
// - Lựa chọn được lưu `localStorage` ⇒ giữ nguyên sau F5.
// - `silent=true` khi auto-refresh ⇒ không nháy toast lỗi, không cuộn trang.
let rpAutoRefreshMs = 0;
let rpAutoRefreshTimer = null;
let rpAutoRefreshPaused = false;
const RP_AUTO_REFRESH_KEY = 'apex_rp_auto_refresh';

function rpAutoRefreshLabel(ms) {
    if (!ms) return 'Tắt';
    if (ms < 60000) return (ms / 1000) + 's';
    return (ms / 60000) + ' phút';
}

// Dừng timer khi admin đang chọn bộ lọc hoặc tab bị ẩn.
function rpAutoRefreshPause(paused) {
    rpAutoRefreshPaused = paused;
    if (rpAutoRefreshTimer) {
        clearInterval(rpAutoRefreshTimer);
        rpAutoRefreshTimer = null;
    }
    if (!paused) rpAutoRefreshStart();
}

// Bắt đầu timer auto-refresh theo chu kỳ đã chọn.
function rpAutoRefreshStart() {
    if (rpAutoRefreshTimer) {
        clearInterval(rpAutoRefreshTimer);
        rpAutoRefreshTimer = null;
    }
    if (!rpAutoRefreshMs || rpAutoRefreshPaused) {
        rpAutoRefreshUpdateStatus();
        return;
    }
    rpAutoRefreshTimer = setInterval(() => {
        // Bỏ qua nếu tab đang ẩn (admin rời bàn phím) hoặc đang tương tác bộ lọc.
        if (document.hidden || rpAutoRefreshPaused) return;
        rpAutoRefreshUpdateStatus();
        refreshActiveTab(true);
    }, rpAutoRefreshMs);
    rpAutoRefreshUpdateStatus();
}

// Hiển thị trạng thái live đang chạy.
function rpAutoRefreshUpdateStatus() {
    const el = document.getElementById('rpAutoRefreshStatus');
    if (!el) return;
    if (!rpAutoRefreshMs) {
        el.classList.add('hidden');
        el.textContent = '';
        return;
    }
    el.classList.remove('hidden');
    el.textContent = '● ' + rpAutoRefreshLabel(rpAutoRefreshMs);
}

// setAutoRefresh được gọi khi admin đổi lựa chọn trong dropdown.
function setAutoRefresh(value) {
    const ms = parseInt(value, 10);
    rpAutoRefreshMs = (isNaN(ms) || ms <= 0) ? 0 : ms;
    try {
        localStorage.setItem(RP_AUTO_REFRESH_KEY, String(rpAutoRefreshMs));
    } catch (_) {}
    rpAutoRefreshStart();
    // Nếu đang bật live ⇒ nạp lại ngay để thấy dữ liệu mới nhất.
    if (rpAutoRefreshMs > 0) refreshActiveTab(true);
}

// ──────────────────────────────────────────────────────────────
// 3. TIỆN ÍCH
// ──────────────────────────────────────────────────────────────
function escapeHTML(str) {
    if (str === null || str === undefined) return '';
    return String(str)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#039;');
}

function rpNum(v, digits) {
    const n = Number(v || 0);
    if (!isFinite(n)) return '0';
    if (digits === undefined) {
        return n.toLocaleString('vi-VN', { maximumFractionDigits: 0 });
    }
    return n.toLocaleString('vi-VN', { minimumFractionDigits: digits, maximumFractionDigits: digits });
}

function rpPct(v, digits) {
    const n = Number(v || 0);
    return rpNum(n, digits === undefined ? 2 : digits) + '%';
}

function rpMs(v) {
    const n = Number(v || 0);
    if (!n) return '--';
    if (n < 1000) return rpNum(n) + ' ms';
    return rpNum(n / 1000, 1) + ' giây';
}

function rpDateStr(d) {
    const y = d.getFullYear();
    const m = String(d.getMonth() + 1).padStart(2, '0');
    const day = String(d.getDate()).padStart(2, '0');
    return `${y}-${m}-${day}`;
}

// rpToast hiển thị thông báo nhỏ góc phải.
//
// ⚠️ Khi auto-refresh đang chạy (`rpAutoRefreshSilent = true`) thì bỏ qua
// toast LỖI ⇒ tránh việc admin bị "popup đỏ" liên tục mỗi 10-30 giây khi
// server chậm hoặc mạng chập chờn.
let rpAutoRefreshSilent = false;

function rpToast(msg, type) {
    if (rpAutoRefreshSilent && type === 'error') return;
    const el = document.getElementById('rpToast');
    if (!el) return;
    el.className = 'fixed bottom-6 right-6 z-[150] px-5 py-3.5 rounded-xl shadow-2xl text-white text-sm backdrop-blur-md ' +
        (type === 'error'
            ? 'bg-rose-950/95 border border-rose-500/60'
            : 'bg-slate-900/95 border border-emerald-500/60');
    el.innerHTML = (type === 'error'
        ? '<i class="fa-solid fa-triangle-exclamation text-rose-400 mr-2"></i>'
        : '<i class="fa-solid fa-circle-check text-emerald-400 mr-2"></i>') + escapeHTML(msg);
    el.classList.remove('hidden');
    clearTimeout(window.__rpToastTimer);
    window.__rpToastTimer = setTimeout(() => el.classList.add('hidden'), 3200);
}

// ──────────────────────────────────────────────────────────────
// 4. ĐĂNG NHẬP
// ──────────────────────────────────────────────────────────────
function showRpLogin(msg) {
    const screen = document.getElementById('adminLoginScreen');
    if (screen) screen.classList.remove('hidden');
    if (msg) {
        const box = document.getElementById('loginErrorMessage');
        const txt = document.getElementById('loginErrorText');
        if (txt) txt.innerText = msg;
        if (box) box.classList.remove('hidden');
    }
}

function hideRpLogin() {
    const screen = document.getElementById('adminLoginScreen');
    if (screen) screen.classList.add('hidden');
}

async function checkRpAuth() {
    try {
        const res = await fetch('/api/admin/me');
        if (!res.ok) { showRpLogin(); return false; }
        const json = await res.json();
        if (json && json.success && json.admin) {
            const nameEl = document.getElementById('headerAdminUsername');
            if (nameEl) nameEl.innerText = json.admin.username;
            hideRpLogin();
            return true;
        }
        showRpLogin();
        return false;
    } catch (_) {
        showRpLogin();
        return false;
    }
}

async function handleAdminLogin(e) {
    e.preventDefault();
    const uEl = document.getElementById('loginUsername');
    const pEl = document.getElementById('loginPassword');
    const errBox = document.getElementById('loginErrorMessage');
    const errTxt = document.getElementById('loginErrorText');
    const btn = document.getElementById('btnLoginSubmit');
    const orig = btn ? btn.innerHTML : '';

    const username = uEl ? uEl.value.trim() : '';
    const password = pEl ? pEl.value : '';
    if (!username || !password) {
        if (errTxt) errTxt.innerText = 'Vui lòng nhập đầy đủ tài khoản và mật khẩu.';
        if (errBox) errBox.classList.remove('hidden');
        return;
    }
    if (btn) { btn.disabled = true; btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> Đang xác thực...'; }
    if (errBox) errBox.classList.add('hidden');

    try {
        const res = await fetch('/api/admin/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ username, password })
        });
        const data = await res.json();
        if (!res.ok || !data.success) throw new Error(data.error || 'Tên đăng nhập hoặc mật khẩu không chính xác.');
        if (data.token) { try { localStorage.setItem('apex_admin_token', data.token); } catch (_) {} }
        const nameEl = document.getElementById('headerAdminUsername');
        if (nameEl) nameEl.innerText = data.admin.username;
        hideRpLogin();
        if (pEl) pEl.value = '';
        initReportCenter();
    } catch (err) {
        if (errTxt) errTxt.innerText = err.message;
        if (errBox) errBox.classList.remove('hidden');
    } finally {
        if (btn) { btn.disabled = false; btn.innerHTML = orig; }
    }
}

async function handleAdminLogout() {
    if (!confirm('Bạn có chắc chắn muốn đăng xuất khỏi phiên quản trị này không?')) return;
    try { await fetch('/api/admin/logout', { method: 'POST' }); } catch (_) {}
    try { localStorage.removeItem('apex_admin_token'); } catch (_) {}
    showRpLogin('Bạn đã đăng xuất an toàn khỏi hệ thống.');
}

// ──────────────────────────────────────────────────────────────
// 5. BỘ LỌC TOÀN CỤC
// ──────────────────────────────────────────────────────────────

// buildReportParams dựng query string từ toàn bộ bộ lọc đang chọn.
//
// ⚠️ Mọi endpoint /api/analytics/report/* đều dùng chung bộ lọc này ⇒ các
// thẻ KPI, biểu đồ và bảng luôn khớp nhau.
function buildReportParams(extra) {
    const p = new URLSearchParams();

    const from = document.getElementById('rpFromDate').value;
    const to = document.getElementById('rpToDate').value;
    if (from) p.set('from_date', from);
    if (to) p.set('to_date', to);

    const preset = rpCurrentPreset;
    if (!from && !to) {
        // Chỉ khi KHÔNG có from/to mới dùng tham số `days` (backend sẽ tự
        // tính "N ngày gần nhất"). Các preset chuẩn đều điền from/to nên
        // nhánh này chỉ xảy ra khi admin tự xoá cả 2 ô ngày.
        const days = { today: 1, yesterday: 1, '7days': 7, '30days': 30, thisMonth: 30, year: 365 }[preset] || 30;
        p.set('days', days);
    }

    // Khung giờ
    const fh = document.getElementById('rpFromHour').value;
    const th = document.getElementById('rpToHour').value;
    if (fh !== '' && th !== '') {
        p.set('from_hour', fh);
        p.set('to_hour', th);
    }

    // Landing (multi-select)
    if (rpSelectedLandings.size > 0) {
        p.set('landing_ids', Array.from(rpSelectedLandings).join(','));
    }

    const addSel = (id, key) => {
        const el = document.getElementById(id);
        if (el && el.value) p.set(key, el.value);
    };
    addSel('rpNguon', 'nguon');
    addSel('rpLandingPath', 'landing_path');
    addSel('rpAdAccount', 'ad_account_id');
    addSel('rpCampaign', 'campaign_name');
    addSel('rpAdset', 'adset_name');
    addSel('rpAdName', 'ad_name');
    addSel('rpSiteSource', 'site_source_name');
    addSel('rpPlacement', 'utm_placement');
    addSel('rpVisitorType', 'visitor_type');
    addSel('rpDeviceType', 'device_type');
    addSel('rpOS', 'os');
    addSel('rpInApp', 'is_in_app_browser');
    addSel('rpInAppType', 'in_app_type');
    addSel('rpTrafficScope', 'traffic_scope');

    const cmp = document.getElementById('rpCompare');
    if (cmp && cmp.checked) p.set('compare', '1');

    if (extra) {
        for (const k in extra) p.set(k, extra[k]);
    }
    return p;
}

let rpCurrentPreset = '30days';

function setTimePreset(preset) {
    rpCurrentPreset = preset;
    const now = new Date();
    const fromEl = document.getElementById('rpFromDate');
    const toEl = document.getElementById('rpToDate');

    document.querySelectorAll('.rp-btn').forEach(b => {
        const isActive = b.dataset.preset === preset;
        b.classList.toggle('active', isActive);
        b.classList.toggle('bg-orange-600', isActive);
        b.classList.toggle('border-orange-600', isActive);
        b.classList.toggle('text-white', isActive);
    });

    if (preset === 'today') {
        const t = rpDateStr(now);
        fromEl.value = t; toEl.value = t;
    } else if (preset === 'yesterday') {
        const y = new Date(now); y.setDate(y.getDate() - 1);
        const t = rpDateStr(y);
        fromEl.value = t; toEl.value = t;
    } else if (preset === '7days') {
        const f = new Date(now); f.setDate(f.getDate() - 6);
        fromEl.value = rpDateStr(f); toEl.value = rpDateStr(now);
    } else if (preset === '30days') {
        const f = new Date(now); f.setDate(f.getDate() - 29);
        fromEl.value = rpDateStr(f); toEl.value = rpDateStr(now);
    } else if (preset === 'thisMonth') {
        const first = new Date(now.getFullYear(), now.getMonth(), 1);
        fromEl.value = rpDateStr(first); toEl.value = rpDateStr(now);
    } else if (preset === 'year') {
        const f = new Date(now); f.setDate(f.getDate() - 364);
        fromEl.value = rpDateStr(f); toEl.value = rpDateStr(now);
    }

    refreshActiveTab();
}

function onCustomDateChange() {
    // Chọn ngày tuỳ chỉnh ⇒ bỏ trạng thái active của preset.
    const from = document.getElementById('rpFromDate').value;
    const to = document.getElementById('rpToDate').value;
    if (!from && !to) {
        rpCurrentPreset = '30days';
        refreshActiveTab();
        return;
    }
    rpCurrentPreset = 'custom';
    document.querySelectorAll('.rp-btn').forEach(b => {
        b.classList.remove('active', 'bg-orange-600', 'border-orange-600', 'text-white');
    });
    refreshActiveTab();
}

function toggleAdvancedFilters() {
    const el = document.getElementById('rpAdvancedFilters');
    if (!el) return;
    el.classList.toggle('hidden');
    updateAdvancedCount();
}

function updateAdvancedCount() {
    const ids = ['rpNguon', 'rpLandingPath', 'rpAdAccount', 'rpCampaign', 'rpAdset', 'rpAdName',
        'rpSiteSource', 'rpPlacement', 'rpVisitorType', 'rpDeviceType', 'rpOS', 'rpInApp', 'rpInAppType'];
    let n = 0;
    ids.forEach(id => {
        const el = document.getElementById(id);
        if (el && el.value) n++;
    });
    if (rpSelectedLandings.size > 0) n++;
    const badge = document.getElementById('rpAdvancedCount');
    if (badge) {
        badge.textContent = n;
        badge.classList.toggle('hidden', n === 0);
    }
}

function resetReportFilters() {
    ['rpNguon', 'rpLandingPath', 'rpAdAccount', 'rpCampaign', 'rpAdset', 'rpAdName',
        'rpSiteSource', 'rpPlacement', 'rpDeviceType', 'rpOS', 'rpInAppType'].forEach(id => {
        const el = document.getElementById(id);
        if (el) el.value = '';
    });
    const vt = document.getElementById('rpVisitorType');
    if (vt) vt.value = 'all';
    const ts = document.getElementById('rpTrafficScope');
    if (ts) ts.value = 'human';
    const fh = document.getElementById('rpFromHour');
    const th = document.getElementById('rpToHour');
    if (fh) fh.value = '';
    if (th) th.value = '';
    const cmp = document.getElementById('rpCompare');
    if (cmp) cmp.checked = false;
    rpSelectedLandings.clear();
    renderLandingList();
    setTimePreset('30days');
}

// ── Multi-select landing ──
function toggleLandingDropdown(ev) {
    if (ev) ev.stopPropagation();
    const dd = document.getElementById('rpLandingDropdown');
    if (dd) dd.classList.toggle('hidden');
}

function selectAllLandings() {
    rpSelectedLandings.clear();
    if (rpFilterOptions) {
        (rpFilterOptions.landing_ids || []).forEach(l => rpSelectedLandings.add(l));
    }
    renderLandingList();
    updateAdvancedCount();
    refreshActiveTab();
}

function clearLandings() {
    rpSelectedLandings.clear();
    renderLandingList();
    updateAdvancedCount();
    refreshActiveTab();
}

function toggleLanding(id) {
    if (rpSelectedLandings.has(id)) rpSelectedLandings.delete(id);
    else rpSelectedLandings.add(id);
    renderLandingList();
    updateAdvancedCount();
    refreshActiveTab();
}

function renderLandingList() {
    const box = document.getElementById('rpLandingList');
    if (!box) return;
    const ids = (rpFilterOptions && rpFilterOptions.landing_ids) || [];
    if (!ids.length) {
        box.innerHTML = '<div class="text-[11px] text-slate-500 p-2">Chưa có landing nào</div>';
    } else {
        box.innerHTML = ids.map(id => {
            const checked = rpSelectedLandings.has(id) ? 'checked' : '';
            return `<label class="flex items-center gap-2 px-2 py-1 rounded-lg hover:bg-slate-800 cursor-pointer">
                <input type="checkbox" class="accent-orange-500 w-3.5 h-3.5" ${checked} onchange="toggleLanding('${escapeHTML(id)}')">
                <span class="text-[11px] text-slate-200 truncate">${escapeHTML(id)}</span>
            </label>`;
        }).join('');
    }
    const label = document.getElementById('rpLandingLabel');
    if (label) {
        label.textContent = rpSelectedLandings.size === 0
            ? 'Tất cả Landing'
            : (rpSelectedLandings.size === 1
                ? Array.from(rpSelectedLandings)[0]
                : rpSelectedLandings.size + ' landing đã chọn');
    }
}

// ──────────────────────────────────────────────────────────────
// 6. ĐIỀU KHIỂN TAB
// ──────────────────────────────────────────────────────────────
function switchReportTab(tab) {
    rpActiveTab = tab;
    document.querySelectorAll('.rp-tab').forEach(b => {
        const isActive = b.dataset.rtab === tab;
        b.classList.toggle('active', isActive);
        b.classList.toggle('bg-orange-600', isActive);
        b.classList.toggle('border-orange-600', isActive);
        b.classList.toggle('text-white', isActive);
        b.classList.toggle('border-slate-700', !isActive);
        b.classList.toggle('text-slate-300', !isActive);
    });
    document.querySelectorAll('.rp-panel').forEach(p => {
        p.classList.toggle('hidden', p.id !== 'rpPanel-' + tab);
    });
    refreshActiveTab();
}

function setGranularity(g) {
    rpGranularity = g;
    document.querySelectorAll('.rp-gran-btn').forEach(b => {
        const isActive = b.dataset.gran === g;
        b.classList.toggle('active', isActive);
        b.classList.toggle('bg-orange-600', isActive);
        b.classList.toggle('border-orange-600', isActive);
        b.classList.toggle('text-white', isActive);
    });
    loadOverview();
}

function setHeatmapMetric(m) {
    rpHeatmapMetric = m;
    document.querySelectorAll('.rp-hm-btn').forEach(b => {
        const isActive = b.dataset.hm === m;
        b.classList.toggle('active', isActive);
        b.classList.toggle('bg-rose-600', isActive);
        b.classList.toggle('border-rose-600', isActive);
        b.classList.toggle('text-white', isActive);
    });
    loadMeta();
}

// ──────────────────────────────────────────────────────────────
// 7. GỌI API
// ──────────────────────────────────────────────────────────────
async function rpFetch(path, extra, opts) {
    const p = buildReportParams(extra);
    const res = await fetch(path + '?' + p.toString(), opts);
    if (!res.ok) {
        let msg = 'HTTP ' + res.status;
        try { const j = await res.json(); if (j && j.error) msg = j.error; } catch (_) {}
        throw new Error(msg);
    }
    return res.json();
}

async function rpFetchJson(path, body) {
    const res = await fetch(path, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body)
    });
    const j = await res.json().catch(() => ({}));
    if (!res.ok || !j.success) throw new Error(j.error || ('HTTP ' + res.status));
    return j;
}

// ──────────────────────────────────────────────────────────────
// 8. BIỂU ĐỒ CHART.JS — HÀM DÙNG CHUNG
// ──────────────────────────────────────────────────────────────
const RP_PALETTE = ['#f97316', '#38bdf8', '#a78bfa', '#34d399', '#fbbf24', '#f472b6', '#22d3ee', '#fb923c', '#4ade80', '#c084fc'];

// Bảng màu icon cho thẻ KPI.
//
// ⚠️ KHÔNG ghép chuỗi Tailwind động (`text-${tone}-400`) — Tailwind Play CDN
// chỉ quét class có sẵn khi render, class sinh sau bằng innerHTML có thể bị
// bỏ sót ⇒ icon mất màu. Dùng bảng tra cứu tĩnh thay thế.
const RP_ICON_TONE = {
    rose: 'text-rose-400', amber: 'text-amber-400', orange: 'text-orange-400',
    sky: 'text-sky-400', violet: 'text-violet-400', emerald: 'text-emerald-400',
    cyan: 'text-cyan-400', teal: 'text-teal-400', indigo: 'text-indigo-400'
};
function rpIconTone(tone) { return RP_ICON_TONE[tone] || 'text-slate-400'; }

function rpChartDefaults() {
    return {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
            legend: { labels: { color: '#cbd5e1', boxWidth: 12, font: { size: 11 } } },
            tooltip: {
                backgroundColor: 'rgba(15,23,42,0.95)',
                borderColor: '#334155',
                borderWidth: 1,
                titleColor: '#f1f5f9',
                bodyColor: '#cbd5e1',
                padding: 10
            }
        },
        scales: {
            x: { grid: { color: 'rgba(51,65,85,0.25)' }, ticks: { color: '#94a3b8', font: { size: 10 } } },
            y: { beginAtZero: true, grid: { color: 'rgba(51,65,85,0.25)' }, ticks: { color: '#94a3b8', font: { size: 10 } } }
        }
    };
}

function rpDestroyChart(key) {
    if (rpCharts[key]) {
        try { rpCharts[key].destroy(); } catch (_) {}
        rpCharts[key] = null;
    }
}

function rpMakeChart(key, canvasId, config) {
    rpDestroyChart(key);
    const ctx = document.getElementById(canvasId);
    if (!ctx || typeof Chart === 'undefined') return null;
    rpCharts[key] = new Chart(ctx, config);
    return rpCharts[key];
}

function rpEmptyState(canvasId) {
    const ctx = document.getElementById(canvasId);
    if (!ctx) return;
    const parent = ctx.parentElement;
    if (!parent) return;
    let empty = parent.querySelector('.rp-empty');
    if (!empty) {
        empty = document.createElement('div');
        empty.className = 'rp-empty absolute inset-0 flex items-center justify-center text-xs text-slate-500 pointer-events-none';
        parent.parentElement.style.position = 'relative';
        parent.appendChild(empty);
    }
    empty.textContent = 'Không có dữ liệu trong khoảng thời gian đã chọn';
}

// ══════════════════════════════════════════════════════════════════
// TAB 1 — TỔNG QUAN & HIỆU SUẤT ĐA LANDING
// ══════════════════════════════════════════════════════════════════
async function loadOverview(silent) {
    try {
        const [summary, ts, board] = await Promise.all([
            rpFetch('/api/analytics/report/summary'),
            rpFetch('/api/analytics/report/timeseries', { granularity: rpGranularity }),
            rpFetch('/api/analytics/report/landings')
        ]);
        renderKpiCards(summary.data.kpis || []);
        renderTimeseries(ts.data || []);
        renderLeaderboard(board.data || []);
    } catch (err) {
        rpToast('Lỗi tải tab Tổng quan: ' + err.message, 'error');
    }
}

function renderKpiCards(kpis) {
    const grid = document.getElementById('rpKpiGrid');
    if (!grid) return;
    if (!kpis.length) {
        grid.innerHTML = '<div class="col-span-full text-center text-xs text-slate-500 py-8">Không có dữ liệu</div>';
        return;
    }
    grid.innerHTML = kpis.map(k => {
        const dir = k.direction || 'flat';
        let badge = '';
        if (dir === 'up') {
            badge = `<span class="text-emerald-400"><i class="fa-solid fa-arrow-up"></i> ${rpNum(Math.abs(k.change_percent), 1)}%</span>`;
        } else if (dir === 'down') {
            badge = `<span class="text-rose-400"><i class="fa-solid fa-arrow-down"></i> ${rpNum(Math.abs(k.change_percent), 1)}%</span>`;
        } else {
            badge = `<span class="text-slate-500">—</span>`;
        }
        const digits = (k.unit === '%') ? 2 : 0;
        const icon = {
            sessions: 'fa-eye', unique_visitors: 'fa-users', leads: 'fa-address-book',
            cr: 'fa-percent', avg_visible: 'fa-clock', contact_rate: 'fa-phone-volume',
            form_interactions: 'fa-file-lines', avg_scroll: 'fa-arrow-down-long'
        }[k.key] || 'fa-chart-simple';

        return `<div class="bg-slate-900/60 border border-slate-800 rounded-2xl p-3.5 relative overflow-hidden">
            <div class="flex items-center justify-between text-slate-400 text-[10px] mb-1">
                <span class="truncate" title="${escapeHTML(k.label)}">${escapeHTML(k.label)}</span>
                <i class="fa-solid ${icon} text-orange-400/70"></i>
            </div>
            <div class="text-xl sm:text-2xl font-black text-white font-mono leading-tight">${rpNum(k.value, digits)}<span class="text-[10px] text-slate-500 font-sans ml-1">${escapeHTML(k.unit || '')}</span></div>
            <div class="text-[10px] mt-1 font-mono">${badge}<span class="text-slate-600 ml-1">${escapeHTML(k.hint || '')}</span></div>
        </div>`;
    }).join('');
}

function renderTimeseries(items) {
    const labels = items.map(i => i.label);
    rpMakeChart('timeseries', 'rpChartTimeseries', {
        data: {
            labels: labels,
            datasets: [
                {
                    type: 'bar', label: 'Sessions', data: items.map(i => i.sessions),
                    backgroundColor: 'rgba(56,189,248,0.55)', borderColor: '#38bdf8', borderWidth: 1, borderRadius: 4, yAxisID: 'y'
                },
                {
                    type: 'bar', label: 'Unique Visitors', data: items.map(i => i.unique_visitors),
                    backgroundColor: 'rgba(167,139,250,0.45)', borderColor: '#a78bfa', borderWidth: 1, borderRadius: 4, yAxisID: 'y'
                },
                {
                    type: 'line', label: 'Leads', data: items.map(i => i.leads),
                    borderColor: '#34d399', backgroundColor: 'rgba(52,211,153,0.15)',
                    tension: 0.35, borderWidth: 2.5, pointRadius: 3, pointBackgroundColor: '#34d399', yAxisID: 'y1'
                },
                {
                    type: 'line', label: 'CR %', data: items.map(i => i.cr),
                    borderColor: '#fbbf24', backgroundColor: 'rgba(251,191,36,0.1)',
                    tension: 0.35, borderWidth: 2, borderDash: [5, 4], pointRadius: 2, yAxisID: 'y2'
                }
            ]
        },
        options: Object.assign(rpChartDefaults(), {
            interaction: { mode: 'index', intersect: false },
            scales: {
                x: { grid: { color: 'rgba(51,65,85,0.25)' }, ticks: { color: '#94a3b8', font: { size: 10 }, maxRotation: 0, autoSkipPadding: 12 } },
                y: { beginAtZero: true, position: 'left', grid: { color: 'rgba(51,65,85,0.25)' }, ticks: { color: '#94a3b8', font: { size: 10 }, precision: 0 } },
                y1: { beginAtZero: true, position: 'right', grid: { display: false }, ticks: { color: '#34d399', font: { size: 10 }, precision: 0 } },
                y2: { beginAtZero: true, position: 'right', grid: { display: false }, ticks: { color: '#fbbf24', font: { size: 10 }, callback: v => v + '%' } }
            }
        })
    });
}

function renderLeaderboard(items) {
    const body = document.getElementById('rpLeaderboardBody');
    if (!body) return;
    if (!items.length) {
        body.innerHTML = '<tr><td colspan="11" class="py-6 text-center text-slate-500">Không có dữ liệu</td></tr>';
        return;
    }
    body.innerHTML = items.map(r => `
        <tr class="hover:bg-slate-800/50 transition border-t border-slate-800/60">
            <td class="py-2.5 px-3 font-mono text-orange-400 font-bold">${escapeHTML(r.landing_id)}</td>
            <td class="py-2.5 px-3 text-right font-mono">${rpNum(r.sessions)}</td>
            <td class="py-2.5 px-3 text-right font-mono">${rpNum(r.unique_visitors)}</td>
            <td class="py-2.5 px-3 text-right font-mono ${r.returning_percent > 30 ? 'text-amber-400' : 'text-slate-400'}">${rpPct(r.returning_percent, 1)}</td>
            <td class="py-2.5 px-3 text-right font-mono">${rpNum(r.avg_visible, 1)}s</td>
            <td class="py-2.5 px-3 text-right font-mono">${rpPct(r.avg_scroll, 1)}</td>
            <td class="py-2.5 px-3 text-right font-mono">${rpNum(r.contact_clicks)}</td>
            <td class="py-2.5 px-3 text-right font-mono">${rpNum(r.submit_attempts)}</td>
            <td class="py-2.5 px-3 text-right font-mono font-bold text-emerald-400">${rpNum(r.leads)}</td>
            <td class="py-2.5 px-3 text-right font-mono font-bold ${r.cr >= 5 ? 'text-emerald-400' : (r.cr > 0 ? 'text-amber-400' : 'text-slate-500')}">${rpPct(r.cr, 2)}</td>
            <td class="py-2.5 px-3 text-right font-mono ${r.bot_rate > 20 ? 'text-rose-400' : 'text-slate-500'}">${rpPct(r.bot_rate, 1)}</td>
        </tr>`).join('');
}

// ══════════════════════════════════════════════════════════════════
// TAB 2 — QUẢNG CÁO & ATTRIBUTION
// ══════════════════════════════════════════════════════════════════
async function loadMeta(silent) {
    try {
        const [meta, hierarchy, heatmap] = await Promise.all([
            rpFetch('/api/analytics/report/meta'),
            rpFetch('/api/analytics/report/ad-hierarchy', { limit: 300 }),
            rpFetch('/api/analytics/report/heatmap', { metric: rpHeatmapMetric })
        ]);
        renderPlatformChart(meta.data.platforms || []);
        renderPlacementChart(meta.data.placements || []);
        renderInAppChart(meta.data.in_app || []);
        renderAdHierarchy(hierarchy.data || []);
        renderHeatmap(heatmap.data);
    } catch (err) {
        rpToast('Lỗi tải tab Quảng cáo: ' + err.message, 'error');
    }
}

function renderPlatformChart(items) {
    rpMakeChart('platform', 'rpChartPlatform', {
        type: 'doughnut',
        data: {
            labels: items.map(i => (i.site_source_name || 'unknown').toUpperCase()),
            datasets: [{
                label: 'Sessions',
                data: items.map(i => i.sessions),
                backgroundColor: RP_PALETTE,
                borderColor: 'rgba(15,23,42,0.9)',
                borderWidth: 2
            }]
        },
        options: Object.assign(rpChartDefaults(), {
            cutout: '55%',
            scales: {},
            plugins: Object.assign(rpChartDefaults().plugins, {
                tooltip: {
                    backgroundColor: 'rgba(15,23,42,0.95)',
                    callbacks: {
                        label: (ctx) => {
                            const it = items[ctx.dataIndex];
                            return ` Sessions: ${rpNum(it.sessions)} · Lead: ${rpNum(it.leads)} · CR: ${rpPct(it.cr)}`;
                        }
                    }
                }
            })
        })
    });
}

function renderPlacementChart(items) {
    rpMakeChart('placement', 'rpChartPlacement', {
        type: 'bar',
        data: {
            labels: items.map(i => i.placement),
            datasets: [
                { label: 'Sessions', data: items.map(i => i.sessions), backgroundColor: 'rgba(56,189,248,0.6)', borderColor: '#38bdf8', borderWidth: 1, borderRadius: 4 },
                { label: 'Leads', data: items.map(i => i.leads), backgroundColor: 'rgba(52,211,153,0.65)', borderColor: '#34d399', borderWidth: 1, borderRadius: 4 }
            ]
        },
        options: Object.assign(rpChartDefaults(), {
            indexAxis: 'y',
            scales: {
                x: { beginAtZero: true, grid: { color: 'rgba(51,65,85,0.25)' }, ticks: { color: '#94a3b8', font: { size: 10 }, precision: 0 } },
                y: { grid: { color: 'rgba(51,65,85,0.25)' }, ticks: { color: '#94a3b8', font: { size: 10 } } }
            }
        })
    });
}

function renderInAppChart(items) {
    rpMakeChart('inapp', 'rpChartInApp', {
        type: 'bar',
        data: {
            labels: items.map(i => i.group_name),
            datasets: [
                { label: 'CR %', data: items.map(i => i.cr), backgroundColor: 'rgba(34,211,238,0.6)', borderColor: '#22d3ee', borderWidth: 1, borderRadius: 4 },
                { label: 'Leads', data: items.map(i => i.leads), backgroundColor: 'rgba(251,191,36,0.5)', borderColor: '#fbbf24', borderWidth: 1, borderRadius: 4 }
            ]
        },
        options: Object.assign(rpChartDefaults(), {
            indexAxis: 'y',
            scales: {
                x: { beginAtZero: true, grid: { color: 'rgba(51,65,85,0.25)' }, ticks: { color: '#94a3b8', font: { size: 10 } } },
                y: { grid: { color: 'rgba(51,65,85,0.25)' }, ticks: { color: '#94a3b8', font: { size: 10 } } }
            }
        })
    });
}

function renderAdHierarchy(items) {
    const body = document.getElementById('rpAdHierarchyBody');
    if (!body) return;
    if (!items.length) {
        body.innerHTML = '<tr><td colspan="8" class="py-6 text-center text-slate-500">Không có dữ liệu quảng cáo</td></tr>';
        return;
    }

    // Gom theo campaign → adset → ad.
    const campaigns = new Map();
    items.forEach(r => {
        if (!campaigns.has(r.campaign_name)) campaigns.set(r.campaign_name, new Map());
        const adsets = campaigns.get(r.campaign_name);
        if (!adsets.has(r.adset_name)) adsets.set(r.adset_name, []);
        adsets.get(r.adset_name).push(r);
    });

    let html = '';
    let campIdx = 0;
    campaigns.forEach((adsets, campName) => {
        const campKey = 'c' + (campIdx++);
        const campRows = [];
        adsets.forEach(ads => ads.forEach(a => campRows.push(a)));
        const sum = (key) => campRows.reduce((s, r) => s + (Number(r[key]) || 0), 0);
        const campCr = sum('unique_visitors') > 0 ? sum('leads') / sum('unique_visitors') * 100 : 0;

        html += `<tr class="bg-orange-950/30 hover:bg-orange-950/50 transition cursor-pointer" onclick="toggleAdsetRows('${campKey}')">
            <td class="py-2.5 px-3 font-bold text-orange-300">
                <i class="fa-solid fa-chevron-down text-[9px] mr-1.5" id="chev-${campKey}"></i>
                ${escapeHTML(campName)}
            </td>
            <td class="py-2.5 px-3 text-right font-mono">${rpNum(sum('sessions'))}</td>
            <td class="py-2.5 px-3 text-right font-mono">${rpNum(sum('unique_visitors'))}</td>
            <td class="py-2.5 px-3 text-right font-mono text-rose-300">${rpPct(campRows.length ? sum('fast_bounce') / sum('sessions') * 100 : 0, 1)}</td>
            <td class="py-2.5 px-3 text-right font-mono text-sky-300">${rpPct(campRows.length ? sum('scroll_50') / sum('sessions') * 100 : 0, 1)}</td>
            <td class="py-2.5 px-3 text-right font-mono font-bold text-emerald-400">${rpNum(sum('leads'))}</td>
            <td class="py-2.5 px-3 text-right font-mono font-bold">${rpPct(campCr, 2)}</td>
            <td class="py-2.5 px-3 text-right font-mono text-indigo-300">${rpPct(campRows.length ? sum('capi_matched') / sum('sessions') * 100 : 0, 1)}</td>
        </tr>`;

        adsets.forEach((ads, adsetName) => {
            const adsetRows = ads.slice();
            const sumA = (key) => adsetRows.reduce((s, r) => s + (Number(r[key]) || 0), 0);
            const adsetCr = sumA('unique_visitors') > 0 ? sumA('leads') / sumA('unique_visitors') * 100 : 0;

            html += `<tr class="adset-row-${campKey} hidden bg-slate-950/40 hover:bg-slate-900/60 transition">
                <td class="py-2 px-3 pl-8 font-semibold text-amber-300/90">
                    <i class="fa-solid fa-layer-group text-[9px] mr-1.5"></i>${escapeHTML(adsetName)}
                </td>
                <td class="py-2 px-3 text-right font-mono">${rpNum(sumA('sessions'))}</td>
                <td class="py-2 px-3 text-right font-mono">${rpNum(sumA('unique_visitors'))}</td>
                <td class="py-2 px-3 text-right font-mono text-rose-300">${rpPct(sumA('sessions') ? sumA('fast_bounce') / sumA('sessions') * 100 : 0, 1)}</td>
                <td class="py-2 px-3 text-right font-mono text-sky-300">${rpPct(sumA('sessions') ? sumA('scroll_50') / sumA('sessions') * 100 : 0, 1)}</td>
                <td class="py-2 px-3 text-right font-mono font-bold text-emerald-400">${rpNum(sumA('leads'))}</td>
                <td class="py-2 px-3 text-right font-mono font-bold">${rpPct(adsetCr, 2)}</td>
                <td class="py-2 px-3 text-right font-mono text-indigo-300">${rpPct(sumA('sessions') ? sumA('capi_matched') / sumA('sessions') * 100 : 0, 1)}</td>
            </tr>`;

            adsetRows.forEach(a => {
                html += `<tr class="bg-slate-950/20 hover:bg-slate-900/40 transition">
                    <td class="py-1.5 px-3 pl-12 text-slate-400 font-mono text-[11px]">
                        <i class="fa-solid fa-circle text-[5px] mr-1.5 align-middle"></i>${escapeHTML(a.ad_name)}
                    </td>
                    <td class="py-1.5 px-3 text-right font-mono text-slate-400">${rpNum(a.sessions)}</td>
                    <td class="py-1.5 px-3 text-right font-mono text-slate-400">${rpNum(a.unique_visitors)}</td>
                    <td class="py-1.5 px-3 text-right font-mono ${a.fast_bounce_rate > 40 ? 'text-rose-400' : 'text-slate-500'}">${rpPct(a.fast_bounce_rate, 1)}</td>
                    <td class="py-1.5 px-3 text-right font-mono text-slate-500">${rpPct(a.scroll_50_rate, 1)}</td>
                    <td class="py-1.5 px-3 text-right font-mono text-emerald-400">${rpNum(a.leads)}</td>
                    <td class="py-1.5 px-3 text-right font-mono ${a.cr >= 5 ? 'text-emerald-400' : 'text-slate-400'}">${rpPct(a.cr, 2)}</td>
                    <td class="py-1.5 px-3 text-right font-mono text-slate-500">${rpPct(a.capi_match_rate, 1)}</td>
                </tr>`;
            });
        });
    });

    body.innerHTML = html;
}

function toggleAdsetRows(campKey) {
    document.querySelectorAll('.adset-row-' + CSS.escape(campKey)).forEach(r => r.classList.toggle('hidden'));
    const chev = document.getElementById('chev-' + campKey);
    if (chev) chev.style.transform = chev.style.transform === 'rotate(-90deg)' ? '' : 'rotate(-90deg)';
}

function renderHeatmap(data) {
    const box = document.getElementById('rpHeatmap');
    if (!box) return;
    const matrix = data.matrix || [];
    const cr = data.cr || [];
    const days = data.days || [];
    const hours = data.hours || [];

    const isCR = rpHeatmapMetric === 'cr';
    const values = isCR ? cr : matrix;
    let max = 0;
    values.forEach(row => row.forEach(v => { if (Number(v) > max) max = Number(v); }));
    if (!max) max = 1;

    let html = '<table class="w-full text-[10px] border-separate" style="border-spacing:2px"><thead><tr><th class="text-slate-500 font-semibold"></th>';
    hours.forEach(h => { html += `<th class="text-slate-500 font-mono font-normal">${h}h</th>`; });
    html += '</tr></thead><tbody>';

    days.forEach((d, di) => {
        html += `<tr><td class="text-slate-400 font-semibold pr-1 whitespace-nowrap">${escapeHTML(d)}</td>`;
        for (let h = 0; h < 24; h++) {
            const v = Number((values[di] || [])[h] || 0);
            const ratio = v / max;
            // Thang màu từ xanh lam (nhạt) → đỏ (đậm).
            const alpha = ratio <= 0 ? 0 : 0.12 + ratio * 0.85;
            const bg = ratio <= 0 ? 'rgba(51,65,85,0.15)' : `rgba(244,63,94,${alpha.toFixed(3)})`;
            const color = ratio > 0.55 ? '#fff' : '#94a3b8';
            html += `<td title="${escapeHTML(d)} ${h}h — ${isCR ? rpPct(v) : rpNum(v)}" style="background:${bg};color:${color}" class="text-center font-mono rounded py-1">${isCR ? (v ? rpNum(v, 1) : '') : (v || '')}</td>`;
        }
        html += '</tr>';
    });
    html += '</tbody></table>';

    box.innerHTML = html +
        '<div class="flex items-center gap-2 mt-2 text-[10px] text-slate-500 justify-end">' +
        '<span>Nhạt</span>' +
        '<span style="display:inline-block;width:90px;height:10px;border-radius:5px;background:linear-gradient(90deg,rgba(244,63,94,0.12),rgba(244,63,94,0.97))"></span>' +
        '<span>Đậm (tối đa ' + (isCR ? rpPct(max, 1) : rpNum(max)) + ')</span></div>';
}

// ══════════════════════════════════════════════════════════════════
// TAB 3 — HÀNH VI & NỘI DUNG
// ══════════════════════════════════════════════════════════════════
async function loadBehavior(silent) {
    try {
        const data = (await rpFetch('/api/analytics/report/behavior')).data || {};
        renderScrollFunnel(data.scroll_funnel || []);
        renderVisibleDist(data.visible_dist || []);
        renderEngagement(data.engagement || []);
        renderTopClicked(data.top_clicked || []);
    } catch (err) {
        rpToast('Lỗi tải tab Hành vi: ' + err.message, 'error');
    }
}

function renderScrollFunnel(items) {
    rpMakeChart('scroll', 'rpChartScroll', {
        type: 'bar',
        data: {
            labels: items.map(i => i.label),
            datasets: [{
                label: '% đạt tới mốc',
                data: items.map(i => i.percent),
                backgroundColor: ['rgba(56,189,248,0.75)', 'rgba(99,102,241,0.7)', 'rgba(167,139,250,0.7)', 'rgba(217,70,239,0.7)', 'rgba(244,114,182,0.7)', 'rgba(52,211,153,0.75)'],
                borderColor: '#0f172a', borderWidth: 1, borderRadius: 5
            }]
        },
        options: Object.assign(rpChartDefaults(), {
            plugins: Object.assign(rpChartDefaults().plugins, {
                tooltip: {
                    backgroundColor: 'rgba(15,23,42,0.95)',
                    callbacks: {
                        label: (ctx) => {
                            const it = items[ctx.dataIndex];
                            return ` ${rpNum(it.count)} phiên (${rpPct(it.percent, 1)})`;
                        }
                    }
                }
            }),
            scales: {
                x: { grid: { display: false }, ticks: { color: '#94a3b8', font: { size: 10 } } },
                y: { beginAtZero: true, max: 100, grid: { color: 'rgba(51,65,85,0.25)' }, ticks: { color: '#94a3b8', font: { size: 10 }, callback: v => v + '%' } }
            }
        })
    });
}

function renderVisibleDist(items) {
    rpMakeChart('visibleDist', 'rpChartVisibleDist', {
        type: 'bar',
        data: {
            labels: items.map(i => i.bucket),
            datasets: [
                { label: 'Số phiên', data: items.map(i => i.sessions), backgroundColor: 'rgba(251,191,36,0.6)', borderColor: '#fbbf24', borderWidth: 1, borderRadius: 4 },
                { label: 'Avg visible (s)', data: items.map(i => i.avg_visible), type: 'line', borderColor: '#f97316', backgroundColor: 'rgba(249,115,22,0.1)', tension: 0.35, borderWidth: 2, pointRadius: 3, yAxisID: 'y1' },
                { label: 'Avg hidden (s)', data: items.map(i => i.avg_hidden), type: 'line', borderColor: '#64748b', borderDash: [5, 4], tension: 0.35, borderWidth: 2, pointRadius: 0, yAxisID: 'y1' }
            ]
        },
        options: Object.assign(rpChartDefaults(), {
            scales: {
                x: { grid: { color: 'rgba(51,65,85,0.25)' }, ticks: { color: '#94a3b8', font: { size: 10 } } },
                y: { beginAtZero: true, grid: { color: 'rgba(51,65,85,0.25)' }, ticks: { color: '#94a3b8', font: { size: 10 }, precision: 0 } },
                y1: { beginAtZero: true, position: 'right', grid: { display: false }, ticks: { color: '#f97316', font: { size: 10 } } }
            }
        })
    });
}

function renderEngagement(items) {
    rpMakeChart('engagement', 'rpChartEngagement', {
        type: 'bar',
        data: {
            labels: items.map(i => i.label),
            datasets: [
                { label: 'Sessions', data: items.map(i => i.sessions), backgroundColor: 'rgba(56,189,248,0.5)', borderColor: '#38bdf8', borderWidth: 1, borderRadius: 4 },
                { label: 'Leads', data: items.map(i => i.leads), backgroundColor: 'rgba(52,211,153,0.6)', borderColor: '#34d399', borderWidth: 1, borderRadius: 4 },
                { label: 'CR %', data: items.map(i => i.cr), type: 'line', borderColor: '#fbbf24', backgroundColor: 'rgba(251,191,36,0.1)', tension: 0.35, borderWidth: 2, pointRadius: 3, yAxisID: 'y1' }
            ]
        },
        options: Object.assign(rpChartDefaults(), {
            scales: {
                x: { grid: { color: 'rgba(51,65,85,0.25)' }, ticks: { color: '#94a3b8', font: { size: 10 }, maxRotation: 30, minRotation: 0 } },
                y: { beginAtZero: true, grid: { color: 'rgba(51,65,85,0.25)' }, ticks: { color: '#94a3b8', font: { size: 10 }, precision: 0 } },
                y1: { beginAtZero: true, position: 'right', grid: { display: false }, ticks: { color: '#fbbf24', font: { size: 10 }, callback: v => v + '%' } }
            }
        })
    });
}

function renderTopClicked(items) {
    const body = document.getElementById('rpTopClickedBody');
    if (!body) return;
    if (!items.length) {
        body.innerHTML = '<tr><td colspan="7" class="py-6 text-center text-slate-500">Chưa ghi nhận sự kiện click nào</td></tr>';
        return;
    }
    body.innerHTML = items.map((r, i) => `
        <tr class="hover:bg-slate-800/50 transition border-t border-slate-800/60">
            <td class="py-2.5 px-3 font-mono text-slate-500">${i + 1}</td>
            <td class="py-2.5 px-3 font-mono text-orange-400">${escapeHTML(r.target)}</td>
            <td class="py-2.5 px-3 text-slate-300">${escapeHTML(r.text || '—')}</td>
            <td class="py-2.5 px-3 text-right font-mono font-bold text-white">${rpNum(r.clicks)}</td>
            <td class="py-2.5 px-3 text-right font-mono text-slate-400">${rpNum(r.sessions)}</td>
            <td class="py-2.5 px-3 text-right font-mono text-slate-400">${rpNum(r.visitors)}</td>
            <td class="py-2.5 px-3 text-right font-mono text-slate-400">${rpNum(r.landings)}</td>
        </tr>`).join('');
}

// ══════════════════════════════════════════════════════════════════
// TAB 4 — PHỄU BIỂU MẪU
// ══════════════════════════════════════════════════════════════════
async function loadForm(silent) {
    try {
        const data = (await rpFetch('/api/analytics/report/form')).data || {};
        const funnel = data.form_funnel || {};
        const steps = funnel.funnel || [];

        const grid = document.getElementById('rpFormKpiGrid');
        if (grid) {
            const kpis = [
                { label: 'Tỷ lệ bỏ ngang form', value: rpPct(funnel.form_abandon_rate, 1), hint: 'Đã chạm form nhưng không ra lead', icon: 'fa-door-open', tone: 'rose' },
                { label: 'Tỷ lệ lỗi nhập liệu', value: rpPct(funnel.validation_error_rate, 1), hint: rpNum(funnel.total_errors) + ' lỗi / ' + rpNum(funnel.total_attempts) + ' lần gửi', icon: 'fa-triangle-exclamation', tone: 'amber' },
                { label: 'Thời gian điền TB', value: rpMs(funnel.avg_fill_duration_ms), hint: 'Nhập tay: ' + rpMs(funnel.avg_fill_duration_ms_manual), icon: 'fa-stopwatch', tone: 'sky' },
                { label: 'Thời gian điền (Autofill)', value: rpMs(funnel.avg_fill_duration_ms_auto), hint: rpNum(funnel.autofill_sessions) + ' phiên dùng autofill', icon: 'fa-wand-magic-sparkles', tone: 'violet' }
            ];
            grid.innerHTML = kpis.map(k => `
                <div class="bg-slate-900/60 border border-slate-800 rounded-2xl p-3.5">
                    <div class="flex items-center justify-between text-slate-400 text-[10px] mb-1">
                        <span class="truncate">${escapeHTML(k.label)}</span>
                        <i class="fa-solid ${k.icon} ${rpIconTone(k.tone)}"></i>
                    </div>
                    <div class="text-xl sm:text-2xl font-black text-white font-mono leading-tight">${k.value}</div>
                    <div class="text-[10px] text-slate-500 mt-1 truncate">${escapeHTML(k.hint)}</div>
                </div>`).join('');
        }

        rpMakeChart('formFunnel', 'rpChartFormFunnel', {
            type: 'bar',
            data: {
                labels: steps.map(s => s.label),
                datasets: [
                    { label: 'Số phiên', data: steps.map(s => s.count), backgroundColor: 'rgba(52,211,153,0.55)', borderColor: '#34d399', borderWidth: 1, borderRadius: 5 },
                    { label: '% từ Pageview', data: steps.map(s => s.percent), type: 'line', borderColor: '#fbbf24', backgroundColor: 'rgba(251,191,36,0.1)', tension: 0.3, borderWidth: 2, pointRadius: 4, yAxisID: 'y1' }
                ]
            },
            options: Object.assign(rpChartDefaults(), {
                indexAxis: 'y',
                scales: {
                    x: { beginAtZero: true, grid: { color: 'rgba(51,65,85,0.25)' }, ticks: { color: '#94a3b8', font: { size: 10 }, precision: 0 } },
                    y: { grid: { color: 'rgba(51,65,85,0.25)' }, ticks: { color: '#94a3b8', font: { size: 10 } } },
                    y1: { beginAtZero: true, max: 100, position: 'right', grid: { display: false }, ticks: { color: '#fbbf24', font: { size: 10 }, callback: v => v + '%' } }
                }
            })
        });

        const af = data.autofill_source || [];
        rpMakeChart('autofill', 'rpChartAutofill', {
            type: 'doughnut',
            data: {
                labels: af.map(a => a.source_name),
                datasets: [{
                    label: 'Sessions',
                    data: af.map(a => a.sessions),
                    backgroundColor: RP_PALETTE,
                    borderColor: 'rgba(15,23,42,0.9)',
                    borderWidth: 2
                }]
            },
            options: Object.assign(rpChartDefaults(), {
                cutout: '55%',
                scales: {}
            })
        });
    } catch (err) {
        rpToast('Lỗi tải tab Phễu biểu mẫu: ' + err.message, 'error');
    }
}

// ══════════════════════════════════════════════════════════════════
// TAB 5 — THIẾT BỊ & HẠ TẦNG
// ══════════════════════════════════════════════════════════════════
async function loadDevices(silent) {
    try {
        const data = (await rpFetch('/api/analytics/report/devices')).data || {};
        renderDeviceChart(data.breakdown || []);
        renderHardwareChart(data.hardware || []);
        renderDeviceModels(data.models || []);
    } catch (err) {
        rpToast('Lỗi tải tab Thiết bị: ' + err.message, 'error');
    }
}

function renderDeviceChart(items) {
    // Gom theo device_type → os (bỏ os_version để biểu đồ không quá rối).
    const map = new Map();
    items.forEach(r => {
        const key = (r.device_type || 'unknown') + ' › ' + (r.os || 'unknown');
        if (!map.has(key)) {
            map.set(key, { device: r.device_type || 'unknown', os: r.os || 'unknown', sessions: 0, leads: 0, uv: 0 });
        }
        const o = map.get(key);
        o.sessions += r.sessions || 0;
        o.leads += r.leads || 0;
        o.uv += r.unique_visitors || 0;
    });
    const rows = Array.from(map.values()).sort((a, b) => b.sessions - a.sessions).slice(0, 12);

    rpMakeChart('device', 'rpChartDevice', {
        type: 'bar',
        data: {
            labels: rows.map(r => r.device + ' › ' + r.os),
            datasets: [
                { label: 'Sessions', data: rows.map(r => r.sessions), backgroundColor: 'rgba(34,211,238,0.55)', borderColor: '#22d3ee', borderWidth: 1, borderRadius: 4 },
                { label: 'Leads', data: rows.map(r => r.leads), backgroundColor: 'rgba(52,211,153,0.6)', borderColor: '#34d399', borderWidth: 1, borderRadius: 4 }
            ]
        },
        options: Object.assign(rpChartDefaults(), {
            indexAxis: 'y',
            scales: {
                x: { beginAtZero: true, grid: { color: 'rgba(51,65,85,0.25)' }, ticks: { color: '#94a3b8', font: { size: 10 }, precision: 0 } },
                y: { grid: { color: 'rgba(51,65,85,0.25)' }, ticks: { color: '#94a3b8', font: { size: 10 } } }
            }
        })
    });
}

function renderHardwareChart(items) {
    const map = new Map();
    items.forEach(r => {
        const key = (r.connection_type || 'unknown') + ' · ' + (r.memory_bucket || '?') + ' · ' + (r.cores_bucket || '?');
        if (!map.has(key)) map.set(key, { sessions: 0, leads: 0, fast: 0 });
        const o = map.get(key);
        o.sessions += r.sessions || 0;
        o.leads += r.leads || 0;
        o.fast += r.fast_bounce || 0;
    });
    const rows = Array.from(map.entries()).map(([k, v]) => ({ k: k, ...v }))
        .sort((a, b) => b.sessions - a.sessions).slice(0, 12);

    rpMakeChart('hardware', 'rpChartHardware', {
        type: 'bar',
        data: {
            labels: rows.map(r => r.k),
            datasets: [
                { label: 'Sessions', data: rows.map(r => r.sessions), backgroundColor: 'rgba(45,212,191,0.55)', borderColor: '#2dd4bf', borderWidth: 1, borderRadius: 4 },
                { label: 'Thoát nhanh (<5s)', data: rows.map(r => r.fast), backgroundColor: 'rgba(244,63,94,0.5)', borderColor: '#f43f5e', borderWidth: 1, borderRadius: 4 },
                { label: 'Leads', data: rows.map(r => r.leads), backgroundColor: 'rgba(52,211,153,0.6)', borderColor: '#34d399', borderWidth: 1, borderRadius: 4 }
            ]
        },
        options: Object.assign(rpChartDefaults(), {
            indexAxis: 'y',
            scales: {
                x: { beginAtZero: true, grid: { color: 'rgba(51,65,85,0.25)' }, ticks: { color: '#94a3b8', font: { size: 10 }, precision: 0 } },
                y: { grid: { color: 'rgba(51,65,85,0.25)' }, ticks: { color: '#94a3b8', font: { size: 10 } } }
            }
        })
    });
}

function renderDeviceModels(items) {
    const body = document.getElementById('rpDeviceModelsBody');
    if (!body) return;
    if (!items.length) {
        body.innerHTML = '<tr><td colspan="10" class="py-6 text-center text-slate-500">Không có dữ liệu</td></tr>';
        return;
    }
    body.innerHTML = items.map(r => {
        // Cảnh báo: nhiều click nhưng 0 tương tác form ⇒ nghi vỡ layout.
        const suspicious = r.clicked_sessions > 0 && r.interacted_sessions === 0 && r.sessions >= 5;
        return `<tr class="hover:bg-slate-800/50 transition border-t border-slate-800/60 ${suspicious ? 'bg-rose-950/20' : ''}">
            <td class="py-2.5 px-3 font-mono text-cyan-400">${escapeHTML(r.model_code)}</td>
            <td class="py-2.5 px-3 text-slate-300">${escapeHTML(r.model_name)}</td>
            <td class="py-2.5 px-3 font-mono text-slate-400">${escapeHTML(r.viewport_res)}</td>
            <td class="py-2.5 px-3 font-mono text-slate-500">${escapeHTML(r.screen_res || '—')}</td>
            <td class="py-2.5 px-3 text-right font-mono">${rpNum(r.sessions)}</td>
            <td class="py-2.5 px-3 text-right font-mono text-slate-400">${rpNum(r.unique_visitors)}</td>
            <td class="py-2.5 px-3 text-right font-mono text-sky-300">${rpNum(r.clicked_sessions)}</td>
            <td class="py-2.5 px-3 text-right font-mono ${r.interacted_sessions === 0 ? 'text-rose-400 font-bold' : 'text-slate-400'}">${rpNum(r.interacted_sessions)}${suspicious ? ' <i class="fa-solid fa-triangle-exclamation text-rose-400" title="Nghi vỡ giao diện: có click nhưng không chạm form"></i>' : ''}</td>
            <td class="py-2.5 px-3 text-right font-mono text-emerald-400">${rpNum(r.leads)}</td>
            <td class="py-2.5 px-3 text-right font-mono">${rpPct(r.cr, 2)}</td>
        </tr>`;
    }).join('');
}

// ══════════════════════════════════════════════════════════════════
// TAB 6 — BẢO MẬT & CHỐNG BOT
// ══════════════════════════════════════════════════════════════════
async function loadSecurity(silent) {
    try {
        const data = (await rpFetch('/api/analytics/report/security')).data || {};
        const grid = document.getElementById('rpSecurityKpiGrid');
        if (grid) {
            const kpis = [
                { label: 'Tổng traffic bot bị chặn', value: rpNum(data.bot_sessions), hint: 'Điểm bot TB: ' + rpNum(data.avg_bot_score, 0), icon: 'fa-robot', tone: 'rose' },
                { label: 'Tỷ lệ bot / toàn traffic', value: rpPct(data.bot_rate, 1), hint: rpNum(data.total_sessions) + ' phiên tổng', icon: 'fa-percent', tone: 'amber' },
                { label: 'Lead trong vùng cách ly', value: rpNum(data.quarantine_count), hint: 'Cần admin duyệt tay', icon: 'fa-shield-virus', tone: 'orange' },
                { label: 'Lượt truy cập người thật', value: rpNum(data.human_sessions), hint: 'Traffic sạch', icon: 'fa-user-check', tone: 'emerald' },
                { label: 'Bot score cao nhất', value: rpNum(data.max_bot_score), hint: 'Ngưỡng chặn: 50', icon: 'fa-gauge-high', tone: 'violet' }
            ];
            grid.innerHTML = kpis.map(k => `
                <div class="bg-slate-900/60 border border-slate-800 rounded-2xl p-3.5">
                    <div class="flex items-center justify-between text-slate-400 text-[10px] mb-1">
                        <span class="truncate">${escapeHTML(k.label)}</span>
                        <i class="fa-solid ${k.icon} ${rpIconTone(k.tone)}"></i>
                    </div>
                    <div class="text-xl sm:text-2xl font-black text-white font-mono leading-tight">${k.value}</div>
                    <div class="text-[10px] text-slate-500 mt-1 truncate">${escapeHTML(k.hint)}</div>
                </div>`).join('');
        }

        const groups = data.reason_groups || [];
        rpMakeChart('botReasons', 'rpChartBotReasons', {
            type: 'bar',
            data: {
                labels: groups.map(g => g.name),
                datasets: [{
                    label: 'Số phiên bị chặn',
                    data: groups.map(g => g.count),
                    backgroundColor: 'rgba(244,63,94,0.55)',
                    borderColor: '#f43f5e', borderWidth: 1, borderRadius: 5
                }]
            },
            options: Object.assign(rpChartDefaults(), {
                indexAxis: 'y',
                scales: {
                    x: { beginAtZero: true, grid: { color: 'rgba(51,65,85,0.25)' }, ticks: { color: '#94a3b8', font: { size: 10 }, precision: 0 } },
                    y: { grid: { color: 'rgba(51,65,85,0.25)' }, ticks: { color: '#94a3b8', font: { size: 10 } } }
                }
            })
        });

        await loadQuarantine();
    } catch (err) {
        rpToast('Lỗi tải tab Bảo mật: ' + err.message, 'error');
    }
}

async function loadQuarantine() {
    const body = document.getElementById('rpQuarantineBody');
    if (!body) return;
    try {
        const landingEl = document.getElementById('rpQuarantineLanding');
        const p = new URLSearchParams();
        p.set('limit', RP_QUARANTINE_LIMIT);
        p.set('offset', rpQuarantineOffset);
        if (landingEl && landingEl.value) p.set('landing_id', landingEl.value);

        const res = await fetch('/api/analytics/report/quarantine?' + p.toString());
        if (!res.ok) throw new Error('HTTP ' + res.status);
        const json = await res.json();
        const items = json.data || [];

        if (!items.length) {
            body.innerHTML = '<tr><td colspan="9" class="py-6 text-center text-slate-500">Không có request nào bị cách ly</td></tr>';
        } else {
            body.innerHTML = items.map(r => `
                <tr class="hover:bg-slate-800/50 transition border-t border-slate-800/60">
                    <td class="py-2.5 px-3 font-mono text-slate-400 text-[11px]">${escapeHTML(r.created_at || '')}</td>
                    <td class="py-2.5 px-3 font-mono text-orange-400">${escapeHTML(r.landing_id || '—')}</td>
                    <td class="py-2.5 px-3 text-slate-300">${escapeHTML(r.name || '—')}</td>
                    <td class="py-2.5 px-3 font-mono text-slate-200">${escapeHTML(r.phone || '—')}</td>
                    <td class="py-2.5 px-3"><span class="bg-rose-950 text-rose-300 border border-rose-800 text-[10px] px-2 py-0.5 rounded-full font-semibold">${escapeHTML(r.reason || '—')}</span></td>
                    <td class="py-2.5 px-3 text-right font-mono text-slate-400">${rpNum(r.fill_duration_ms || 0)}</td>
                    <td class="py-2.5 px-3 text-right font-mono text-slate-400">${rpNum(r.visible_seconds || 0)}</td>
                    <td class="py-2.5 px-3 text-right font-mono text-slate-400">${rpNum(r.max_scroll || 0)}%</td>
                    <td class="py-2.5 px-3 text-center">
                        <button onclick="approveQuarantine(${r.id}, '${escapeHTML(r.phone || '')}')" class="bg-emerald-700 hover:bg-emerald-600 text-white text-[10px] font-bold px-2.5 py-1 rounded-lg border border-emerald-600 transition">
                            <i class="fa-solid fa-check mr-1"></i>Duyệt
                        </button>
                    </td>
                </tr>`).join('');
        }

        const info = document.getElementById('rpQuarantineInfo');
        if (info) {
            const total = json.total || 0;
            const from = total ? rpQuarantineOffset + 1 : 0;
            const to = Math.min(rpQuarantineOffset + RP_QUARANTINE_LIMIT, total);
            info.textContent = `Hiển thị ${from}–${to} / ${rpNum(total)} request bị cách ly`;
        }
    } catch (err) {
        body.innerHTML = '<tr><td colspan="9" class="py-6 text-center text-rose-400">Lỗi tải dữ liệu: ' + escapeHTML(err.message) + '</td></tr>';
    }
}

function changeQuarantinePage(delta) {
    const next = rpQuarantineOffset + delta * RP_QUARANTINE_LIMIT;
    if (next < 0) return;
    rpQuarantineOffset = next;
    loadQuarantine();
}

async function approveQuarantine(id, phone) {
    if (!confirm('Duyệt thủ công lead này và đẩy vào CRM?\nSĐT: ' + phone)) return;
    try {
        const j = await rpFetchJson('/api/analytics/report/quarantine/approve', { id: id });
        rpToast('Đã duyệt lead ' + (j.data && j.data.name ? j.data.name : phone) + ' → CRM (Lead ID ' + (j.data && j.data.lead_id) + ')');
        loadQuarantine();
        loadSecurity();
    } catch (err) {
        rpToast('Không duyệt được: ' + err.message, 'error');
    }
}

// ──────────────────────────────────────────────────────────────
// 9. ĐIỀU KHIỂN TAB CHÍNH
// ──────────────────────────────────────────────────────────────
//
// `silent` = true khi gọi tự động (auto-refresh) ⇒ không nháy toast lỗi và
// không cuộn trang, tránh giật màn hình khi admin đang đọc báo cáo.
function refreshActiveTab(silent) {
    updateAdvancedCount();
    rpAutoRefreshSilent = !!silent;
    let p;
    switch (rpActiveTab) {
        case 'overview': p = loadOverview(silent); break;
        case 'meta': p = loadMeta(silent); break;
        case 'behavior': p = loadBehavior(silent); break;
        case 'form': p = loadForm(silent); break;
        case 'devices': p = loadDevices(silent); break;
        case 'security': p = loadSecurity(silent); break;
        default: p = Promise.resolve();
    }
    // ⚠️ Reset cờ silent SAU khi lần tải hiện tại hoàn tất ⇒ các lỗi do thao tác
    // tay của admin (bấm nút, đổi tab) vẫn hiện toast bình thường.
    return Promise.resolve(p).finally(() => { rpAutoRefreshSilent = false; });
}

// ──────────────────────────────────────────────────────────────
// 10. KHỞI TẠO
// ──────────────────────────────────────────────────────────────
async function loadFilterOptions() {
    try {
        const res = await fetch('/api/analytics/report/filters');
        if (!res.ok) return;
        const json = await res.json();
        if (!json.success) return;
        rpFilterOptions = json.data || {};

        const fill = (selId, values, keepFirst) => {
            const el = document.getElementById(selId);
            if (!el) return;
            const cur = el.value;
            const first = keepFirst && el.options.length ? el.options[0].outerHTML : '';
            el.innerHTML = first;
            (values || []).forEach(v => {
                const o = document.createElement('option');
                o.value = v;
                o.textContent = v;
                el.appendChild(o);
            });
            if (cur) el.value = cur;
        };

        fill('rpNguon', rpFilterOptions.nguon, true);
        fill('rpLandingPath', rpFilterOptions.landing_paths, true);
        fill('rpAdAccount', rpFilterOptions.ad_account_ids, true);
        fill('rpCampaign', rpFilterOptions.campaign_names, true);
        fill('rpAdset', rpFilterOptions.adset_names, true);
        fill('rpAdName', rpFilterOptions.ad_names, true);
        fill('rpSiteSource', rpFilterOptions.site_source_names, true);
        fill('rpPlacement', rpFilterOptions.utm_placements, true);
        fill('rpDeviceType', rpFilterOptions.device_types, true);
        fill('rpOS', rpFilterOptions.os, true);
        fill('rpInAppType', rpFilterOptions.in_app_types, true);

        renderLandingList();

        // Dropdown landing trong bảng vùng cách ly
        const qSel = document.getElementById('rpQuarantineLanding');
        if (qSel && qSel.options.length <= 1) {
            (rpFilterOptions.landing_ids || []).forEach(v => {
                const o = document.createElement('option');
                o.value = v; o.textContent = v;
                qSel.appendChild(o);
            });
        }
    } catch (_) {}
}

function initHourSelectors() {
    const fh = document.getElementById('rpFromHour');
    const th = document.getElementById('rpToHour');
    if (!fh || !th) return;
    const opt = (v, label) => `<option value="${v}">${label}</option>`;
    fh.innerHTML = opt('', 'Tất cả') + Array.from({ length: 24 }, (_, i) => opt(i, String(i).padStart(2, '0') + 'h')).join('');
    th.innerHTML = opt('', 'Tất cả') + Array.from({ length: 24 }, (_, i) => opt(i, String(i).padStart(2, '0') + 'h')).join('');
}

function initReportCenter() {
    initHourSelectors();
    setTimePreset('30days');
    loadFilterOptions().then(() => refreshActiveTab());
    initAutoRefresh();
}

// initAutoRefresh khôi phục chu kỳ live đã lưu & gắn sự kiện tạm dừng khi
// admin đang tương tác bộ lọc.
function initAutoRefresh() {
    const sel = document.getElementById('rpAutoRefresh');
    if (sel) {
        let saved = 0;
        try { saved = parseInt(localStorage.getItem(RP_AUTO_REFRESH_KEY) || '0', 10) || 0; } catch (_) {}
        // Chỉ chấp nhận các giá trị hợp lệ (10s/30s/1'/5'/10').
        const allowed = [0, 10000, 30000, 60000, 300000, 600000];
        if (allowed.indexOf(saved) === -1) saved = 0;
        sel.value = String(saved);
        rpAutoRefreshMs = saved;
    }

    // Tạm dừng live khi admin đang chọn ngày / bộ lọc để không "giật" dữ liệu.
    const pauseOnFocus = () => rpAutoRefreshPause(true);
    const resumeOnBlur = () => rpAutoRefreshPause(false);
    ['rpFromDate', 'rpToDate', 'rpFromHour', 'rpToHour'].forEach(id => {
        const el = document.getElementById(id);
        if (!el) return;
        el.addEventListener('focus', pauseOnFocus);
        el.addEventListener('change', resumeOnBlur);
        el.addEventListener('blur', resumeOnBlur);
    });

    // Dừng hẳn timer khi tab bị ẩn (admin rời bàn phím).
    document.addEventListener('visibilitychange', () => {
        if (document.hidden) {
            if (rpAutoRefreshTimer) {
                clearInterval(rpAutoRefreshTimer);
                rpAutoRefreshTimer = null;
            }
        } else {
            rpAutoRefreshStart();
        }
    });

    rpAutoRefreshStart();
}

document.addEventListener('DOMContentLoaded', async () => {
    // Đóng dropdown multi-landing khi click ra ngoài.
    document.addEventListener('click', (ev) => {
        const dd = document.getElementById('rpLandingDropdown');
        const btn = document.getElementById('rpLandingBtn');
        if (dd && btn && !btn.contains(ev.target) && !dd.contains(ev.target)) {
            dd.classList.add('hidden');
        }
    });

    const isAuthed = await checkRpAuth();
    if (isAuthed) initReportCenter();
});
