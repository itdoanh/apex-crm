/* ==============================================================
   APEX ADMIN - APP CORE ROUTER, TABS & GLOBAL REFRESH
   ============================================================== */

// Core state
let currentMainTab = 'crm';
let currentTimePreset = 'all';

// Auto-refresh state
let autoRefreshSec = 10;
let autoRefreshTimer = null;
let countdownSec = 10;

function formatYMD(d) {
    const year = d.getFullYear();
    const month = String(d.getMonth() + 1).padStart(2, '0');
    const day = String(d.getDate()).padStart(2, '0');
    return `${year}-${month}-${day}`;
}

        // 1. Tab Switching
        function switchMainTab(tab) {
            currentMainTab = tab;
            ['CRM', 'Analytics', 'Landing', 'Security', 'Sheets', 'LadiStudio', 'Domains', 'AuditLogs'].forEach(t => {
                const btn = document.getElementById('tabBtn' + t);
                const sec = document.getElementById('sectionTab' + t);
                if (btn) btn.classList.remove('active');
                if (sec) sec.classList.add('hidden');
            });

            let tabKey = tab;
            if (tab === 'crm') tabKey = 'CRM';
            else if (tab === 'ladiStudio') tabKey = 'LadiStudio';
            else if (tab === 'domains') tabKey = 'Domains';
            else if (tab === 'auditLogs') tabKey = 'AuditLogs';
            else tabKey = tab.charAt(0).toUpperCase() + tab.slice(1);

            const activeBtn = document.getElementById('tabBtn' + tabKey);
            const activeSec = document.getElementById('sectionTab' + tabKey);
            if (activeBtn) activeBtn.classList.add('active');
            if (activeSec) activeSec.classList.remove('hidden');

            loadCurrentTabData();
        }

        // 2. CRM Sub-View Switching (Table vs Kanban)
        function switchCRMView(view) {
            currentCrmView = view;
            const btnTable = document.getElementById('viewBtnTable');
            const btnKanban = document.getElementById('viewBtnKanban');
            const viewTable = document.getElementById('crmTableView');
            const viewKanban = document.getElementById('crmKanbanView');

            btnTable.classList.remove('active');
            btnKanban.classList.remove('active');

            if (view === 'table') {
                btnTable.classList.add('active');
                viewTable.classList.remove('hidden');
                viewKanban.classList.add('hidden');
            } else {
                btnKanban.classList.add('active');
                viewTable.classList.add('hidden');
                viewKanban.classList.remove('hidden');
                renderKanbanBoard(curLeads);
            }
        }

        // 3. Time Presets
        function setTimePreset(preset) {
            currentTimePreset = preset;
            document.querySelectorAll('.time-preset-btn').forEach(btn => btn.classList.remove('active'));
            const activeBtn = document.getElementById('presetBtn_' + preset);
            if (activeBtn) activeBtn.classList.add('active');

            const now = new Date();
            const formatYMD = d => {
                const year = d.getFullYear();
                const month = String(d.getMonth() + 1).padStart(2, '0');
                const day = String(d.getDate()).padStart(2, '0');
                return `${year}-${month}-${day}`;
            };

            const inputFrom = document.getElementById('filterFromDate');
            const inputTo = document.getElementById('filterToDate');

            if (preset === 'all') {
                inputFrom.value = '';
                inputTo.value = '';
            } else if (preset === 'today') {
                const todayStr = formatYMD(now);
                inputFrom.value = todayStr;
                inputTo.value = todayStr;
            } else if (preset === 'yesterday') {
                const y = new Date(now);
                y.setDate(y.getDate() - 1);
                const yStr = formatYMD(y);
                inputFrom.value = yStr;
                inputTo.value = yStr;
            } else if (preset === '7days') {
                const f = new Date(now);
                f.setDate(f.getDate() - 6);
                inputFrom.value = formatYMD(f);
                inputTo.value = formatYMD(now);
            } else if (preset === '30days') {
                const f = new Date(now);
                f.setDate(f.getDate() - 29);
                inputFrom.value = formatYMD(f);
                inputTo.value = formatYMD(now);
            } else if (preset === 'thisMonth') {
                const firstDay = new Date(now.getFullYear(), now.getMonth(), 1);
                inputFrom.value = formatYMD(firstDay);
                inputTo.value = formatYMD(now);
            }

            crmPage = 1;
            loadCurrentTabData();
        }

        // 4. Query Params Builder
        function buildFilterParams() {
            const p = new URLSearchParams();
            const fromDate = document.getElementById('filterFromDate').value.trim();
            const toDate = document.getElementById('filterToDate').value.trim();
            const hourRange = document.getElementById('filterHourRange').value;
            const landing = document.getElementById('filterLanding').value.trim();
            const nguon = document.getElementById('filterNguon').value.trim();
            const status = document.getElementById('filterStatus').value.trim();
            const carrier = document.getElementById('filterCarrier').value.trim();
            const search = document.getElementById('filterSearch').value.trim();

            if (fromDate) p.set('from_date', fromDate);
            if (toDate) p.set('to_date', toDate);
            if (landing) p.set('landing_id', landing);
            if (nguon) p.set('nguon', nguon);
            if (status) p.set('status', status);
            if (carrier) p.set('carrier', carrier);
            if (search) p.set('search', search);

            // ⚠️ 2026-11-12: from_hour/to_to cũng được áp dụng cho các API
            // analytics (không chỉ /api/leads). Trước đây bộ lọc khung giờ
            // bị bỏ qua ở toàn bộ biểu đồ ⇒ số liệu không khớp bảng lead.

            if (hourRange === 'golden') {
                p.set('from_hour', '19');
                p.set('to_hour', '22');
            } else if (hourRange === 'morning') {
                p.set('from_hour', '8');
                p.set('to_hour', '12');
            } else if (hourRange === 'afternoon') {
                p.set('from_hour', '13');
                p.set('to_hour', '17');
            } else if (hourRange === 'night') {
                // Khoảng vòng qua nửa đêm (22h → 06h). Backend đã hỗ trợ
                // from_hour > to_hour ⇒ dùng đúng khoảng này.
                p.set('from_hour', '22');
                p.set('to_hour', '6');
            }

            return p;
        }

        function applyFilters() {
            crmPage = 1;
            loadCurrentTabData();
        }

        let searchDebounceTimer = null;
        function debounceSearch() {
            clearTimeout(searchDebounceTimer);
            searchDebounceTimer = setTimeout(() => {
                applyFilters();
            }, 350);
        }

        function resetAllFilters() {
            document.getElementById('filterFromDate').value = '';
            document.getElementById('filterToDate').value = '';
            document.getElementById('filterHourRange').value = 'all';
            document.getElementById('filterLanding').value = '';
            document.getElementById('filterNguon').value = '';
            document.getElementById('filterStatus').value = '';
            document.getElementById('filterCarrier').value = '';
            document.getElementById('filterSearch').value = '';
            setTimePreset('all');
        }

// ==============================================================
// NATIVE ES / SCRIPT DYNAMIC IMPORT & LAZY LOADING REGISTRY
// ==============================================================
const loadedModules = new Set(['auth', 'app', 'crm']);
const moduleLoadingPromises = {};

// APP_ASSET_VERSION: tăng mỗi khi deploy để cache-busting các module lazy-load.
// Server cũng trả `Cache-Control: no-cache` cho .js/.css, nhưng browser vẫn có thể
// dùng bản cache khi offline hoặc khi proxy cache. Query ?v= đảm bảo luôn lấy bản mới.
const APP_ASSET_VERSION = '20261104-02';

async function ensureTabModuleLoaded(moduleName) {
    if (loadedModules.has(moduleName)) return true;
    if (moduleLoadingPromises[moduleName]) return moduleLoadingPromises[moduleName];

    moduleLoadingPromises[moduleName] = new Promise((resolve, reject) => {
        const script = document.createElement('script');
        script.src = `js/${moduleName}.js?v=${APP_ASSET_VERSION}`;
        script.async = true;
        script.onload = () => {
            loadedModules.add(moduleName);
            console.log(`[Apex Dynamic Import] Lazy loaded module: js/${moduleName}.js?v=${APP_ASSET_VERSION}`);
            resolve(true);
        };
        script.onerror = (err) => {
            console.error(`[Apex Dynamic Import] Failed to load module: js/${moduleName}.js`, err);
            reject(err);
        };
        document.body.appendChild(script);
    });

    return moduleLoadingPromises[moduleName];
}

        // 5. Data Fetchers
        async function loadCurrentTabData() {
            // Luôn nạp lại thống kê KPI & Options
            if (typeof loadCRMStats === 'function') loadCRMStats();

            if (currentMainTab === 'crm') {
                if (typeof loadCRMLeads === 'function') loadCRMLeads();
            } else if (currentMainTab === 'analytics') {
                await ensureTabModuleLoaded('analytics');
                if (typeof loadGeoAnalyticsCharts === 'function') loadGeoAnalyticsCharts();
                if (typeof loadAdPlacementAnalyticsCharts === 'function') loadAdPlacementAnalyticsCharts();
                if (typeof loadLandingReport === 'function') loadLandingReport(true);
            } else if (currentMainTab === 'landing') {
                await ensureTabModuleLoaded('analytics');
                if (typeof loadLandingPerformance === 'function') loadLandingPerformance();
                if (typeof loadSourcePerformance === 'function') loadSourcePerformance();
                if (typeof loadSessions === 'function') loadSessions();
            } else if (currentMainTab === 'security') {
                await ensureTabModuleLoaded('security');
                if (typeof loadSecurityDashboard === 'function') loadSecurityDashboard();
            } else if (currentMainTab === 'sheets') {
                await ensureTabModuleLoaded('sheets');
                if (typeof loadGoogleSheetsConfigs === 'function') loadGoogleSheetsConfigs();
            } else if (currentMainTab === 'ladiStudio') {
                await ensureTabModuleLoaded('landings');
                if (typeof loadLandingPages === 'function') loadLandingPages();
            } else if (currentMainTab === 'domains') {
                await ensureTabModuleLoaded('domains');
                if (typeof loadDomains === 'function') loadDomains();
            } else if (currentMainTab === 'auditLogs') {
                if (typeof loadActiveSessions === 'function') loadActiveSessions();
                if (typeof loadAuditLogs === 'function') loadAuditLogs(1);
            }
        }



// Auto Refresh Loop & Global Trigger
        function changeAutoRefresh() {
            autoRefreshSec = parseInt(document.getElementById('autoRefreshSelect').value, 10);
            countdownSec = autoRefreshSec;
            updateCountdownText();
        }

        function updateCountdownText() {
            const el = document.getElementById('refreshCountdown');
            if (!el) return;
            if (autoRefreshSec === 0) {
                el.innerText = 'Tắt';
                el.className = 'text-[10px] font-mono text-slate-500 w-5 text-right';
            } else {
                el.innerText = `${countdownSec}s`;
                el.className = 'text-[10px] font-mono text-emerald-400 w-5 text-right';
            }
        }

        function triggerManualRefresh() {
            countdownSec = autoRefreshSec;
            loadCurrentTabData();
        }

        function startAutoRefreshLoop() {
            if (autoRefreshTimer) clearInterval(autoRefreshTimer);
            autoRefreshTimer = setInterval(() => {
                if (autoRefreshSec > 0) {
                    countdownSec--;
                    if (countdownSec <= 0) {
                        countdownSec = autoRefreshSec;
                        loadCurrentTabData();
                    }
                    updateCountdownText();
                }
            }, 1000);
        }



// Global Toast Notification Helper
function showToastNotification(message, type = 'success') {
    let toast = document.getElementById('toastNotification');
    if (!toast) {
        toast = document.createElement('div');
        toast.id = 'toastNotification';
        toast.className = 'fixed bottom-6 right-6 z-[100] hidden flex items-center space-x-3 px-5 py-3.5 bg-slate-900/95 border border-emerald-500/60 rounded-xl shadow-2xl text-white text-sm backdrop-blur-md transition-all duration-300';
        toast.innerHTML = '<i id="toastIcon" class="fa-solid fa-circle-check text-emerald-400 text-lg"></i><span id="toastMessage"></span>';
        document.body.appendChild(toast);
    }
    const msgEl = document.getElementById('toastMessage');
    const iconEl = document.getElementById('toastIcon');
    if (msgEl) msgEl.innerText = message;
    if (iconEl) {
        if (type === 'error') {
            iconEl.className = 'fa-solid fa-triangle-exclamation text-rose-400 text-lg';
            toast.className = 'fixed bottom-6 right-6 z-[100] flex items-center space-x-3 px-5 py-3.5 bg-rose-950/95 border border-rose-500/60 rounded-xl shadow-2xl text-white text-sm backdrop-blur-md transition-all duration-300 transform translate-y-0 opacity-100';
        } else {
            iconEl.className = 'fa-solid fa-circle-check text-emerald-400 text-lg';
            toast.className = 'fixed bottom-6 right-6 z-[100] flex items-center space-x-3 px-5 py-3.5 bg-slate-900/95 border border-emerald-500/60 rounded-xl shadow-2xl text-white text-sm backdrop-blur-md transition-all duration-300 transform translate-y-0 opacity-100';
        }
    }
    toast.classList.remove('hidden');
    clearTimeout(window.__toastTimer);
    window.__toastTimer = setTimeout(() => {
        toast.classList.add('hidden');
    }, 3200);
}

// App Initialization
document.addEventListener('DOMContentLoaded', async () => {
    if (typeof updateApiStatusBadge === 'function') updateApiStatusBadge();
    const isAuthed = await checkAdminAuth();
    if (isAuthed) {
        loadCurrentTabData();
        if (typeof loadDomains === 'function') loadDomains();
        startAutoRefreshLoop();

        // Idle background pre-fetch: lazy-load remaining tab modules without blocking initial paint
        const prefetchModules = () => {
            ['analytics', 'landings', 'sheets', 'domains', 'security'].forEach(m => ensureTabModuleLoaded(m));
        };
        if ('requestIdleCallback' in window) {
            requestIdleCallback(prefetchModules, { timeout: 2000 });
        } else {
            setTimeout(prefetchModules, 600);
        }
    }
});
