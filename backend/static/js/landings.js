/* ==============================================================
   APEX ADMIN - LANDINGS MANAGEMENT & SETTINGS MODAL
   ============================================================== */

        // ============================================================
        // 17. APEX LADI STUDIO: CONTROLLERS & DATA STORE
        // ============================================================
        let allLandingsList = [];
        let currentStudioLanding = null;
        let currentStudioFiles = [];
        let activeStudioFile = 'index.html';
        var studioDeviceMode = 'pc'; // 'pc', 'laptop', 'mobile'
        let modalPreviewDeviceMode = 'pc';
        let studioSplitOpen = true;
        let studioEditorDirty = false;
        let studioFileContents = {}; // Map[filepath] = string

        // 17.1. Tải danh sách Landing Pages từ Backend CSDL & RAM Cache
        async function loadLandingPages() {
            try {
                const res = await fetch('/api/landings');
                if (!res.ok) throw new Error('Không thể tải danh sách Landing');
                const data = await res.json();
                allLandingsList = data.landings || [];
                window.allLandingsList = allLandingsList;

                // Cập nhật thống kê KPI
                let activeCount = 0;
                let draftCount = 0;
                let totalViews = 0;
                let totalLeads = 0;

                allLandingsList.forEach(p => {
                    if (p.is_active) activeCount++;
                    else draftCount++;
                    totalViews += (p.views_count || 0);
                    totalLeads += (p.leads_count || 0);
                });

                const setElText = (id, text) => {
                    const el = document.getElementById(id);
                    if (el) el.innerText = text;
                };

                setElText('kpiLadiTotal', allLandingsList.length);
                setElText('kpiLadiActive', activeCount);
                setElText('kpiLadiDraft', draftCount);
                setElText('kpiLadiViews', totalViews.toLocaleString('vi-VN'));
                setElText('kpiLadiLeads', totalLeads.toLocaleString('vi-VN'));
                setElText('badgeLadiTotal', allLandingsList.length);

                filterLandingsGrid();
            } catch (e) {
                console.error('Lỗi loadLandingPages:', e);
            }
        }

        // 17.2. Lọc danh sách theo từ khóa và trạng thái
        function filterLandingsGrid() {
            const searchInput = document.getElementById('filterLadiSearch');
            const statusSelect = document.getElementById('filterLadiStatus');
            const query = (searchInput && searchInput.value ? searchInput.value : '').toLowerCase().trim();
            const statusFilter = statusSelect && statusSelect.value ? statusSelect.value : 'all';

            const filtered = (allLandingsList || []).filter(p => {
                const matchQuery = !query ||
                    p.page_id.toLowerCase().includes(query) ||
                    p.name.toLowerCase().includes(query) ||
                    (p.domain && p.domain.toLowerCase().includes(query));

                let matchStatus = true;
                if (statusFilter === 'active') matchStatus = p.is_active === true;
                else if (statusFilter === 'inactive') matchStatus = p.is_active === false;
                else if (statusFilter === 'published') matchStatus = p.status === 'published';
                else if (statusFilter === 'draft') matchStatus = p.status === 'draft';

                return matchQuery && matchStatus;
            });

            const displayCountEl = document.getElementById('ladiDisplayCount');
            if (displayCountEl) displayCountEl.innerText = filtered.length;
            renderLandingsGrid(filtered);
        }

        // 17.3. Render danh sách Landing Page Cards
        function renderLandingsGrid(landings) {
            const grid = document.getElementById('landingsGrid');
            if (!grid) return;

            if (!landings || landings.length === 0) {
                grid.innerHTML = `
                    <div class="col-span-full py-16 text-center bg-slate-900 border border-slate-800 rounded-2xl p-8">
                        <div class="w-16 h-16 bg-cyan-500/10 border border-cyan-500/30 rounded-2xl flex items-center justify-center text-cyan-400 text-2xl mx-auto mb-4">
                            <i class="fa-solid fa-laptop-code"></i>
                        </div>
                        <h3 class="text-base font-bold text-white mb-1">Chưa Có Landing Page Nào Phù Hợp</h3>
                        <p class="text-xs text-slate-400 mb-5 max-w-md mx-auto">Bạn có thể tạo mới landing page tùy chỉnh hoặc nhân bản từ mẫu chuẩn MXV 080 để bắt đầu ngay.</p>
                        <div class="flex items-center justify-center gap-3">
                            <button onclick="openCreateLandingModal()" class="bg-cyan-600 hover:bg-cyan-500 text-white font-bold text-xs px-4 py-2 rounded-xl transition shadow">
                                <i class="fa-solid fa-plus mr-1"></i> Tạo Mới Landing
                            </button>
                            <button onclick="duplicateTemplate('mxv_080_online')" class="bg-slate-800 hover:bg-slate-700 text-cyan-300 font-semibold text-xs px-4 py-2 rounded-xl border border-slate-700 transition">
                                <i class="fa-solid fa-copy mr-1"></i> Nhân Bản Mẫu MXV 080
                            </button>
                        </div>
                    </div>
                `;
                return;
            }

            grid.innerHTML = landings.map(p => {
                const convRate = p.views_count > 0 ? ((p.leads_count / p.views_count) * 100).toFixed(1) : '0.0';
                const statusBadge = p.status === 'published'
                    ? '<span class="bg-emerald-950 text-emerald-400 border border-emerald-800/80 text-[10px] font-bold px-2 py-0.5 rounded-full uppercase">Đã Xuất Bản</span>'
                    : '<span class="bg-amber-950 text-amber-400 border border-amber-800/80 text-[10px] font-bold px-2 py-0.5 rounded-full uppercase">Bản Nháp (Draft)</span>';

                const domainDisplay = p.domain
                    ? `<span class="bg-slate-800 text-cyan-300 border border-slate-700 text-[10px] px-2 py-0.5 rounded font-mono truncate max-w-[140px]" title="${escapeHTML(p.domain)}"><i class="fa-solid fa-globe mr-1"></i>${escapeHTML(p.domain)}</span>`
                    : `<span class="text-slate-500 text-[10px] font-mono">Chung Domain</span>`;

                const domainUrl = (p.domain && (p.domain.startsWith('localhost') || p.domain.startsWith('127.0.0.1'))) ? ('http://' + p.domain) : (p.domain ? ('https://' + p.domain) : '');
                const publicUrl = p.domain ? `${domainUrl}${p.path_prefix || '/'}` : `/p/${encodeURIComponent(p.page_id)}`;

                const s = p.settings || {};
                const sheetMode = s.google_sheet_sync_mode || 'selected';
                const sheetIds = s.google_sheet_ids || [];
                let sheetBadge = '';
                if (sheetMode === 'disabled') {
                    sheetBadge = `<span class="bg-slate-800 text-slate-400 border border-slate-700 text-[10px] px-2 py-0.5 rounded font-mono flex items-center gap-1" title="Tắt đồng bộ Google Sheet"><i class="fa-solid fa-ban text-rose-400"></i> Sheet: Tắt</span>`;
                } else if (sheetMode === 'all') {
                    sheetBadge = `<span class="bg-cyan-950/70 text-cyan-400 border border-cyan-800/60 text-[10px] px-2 py-0.5 rounded font-mono flex items-center gap-1" title="Đồng bộ tới tất cả sheet"><i class="fa-solid fa-table-cells text-cyan-400"></i> Tất cả Sheet</span>`;
                } else if (sheetIds.length > 0) {
                    sheetBadge = `<span class="bg-emerald-950/70 text-emerald-400 border border-emerald-800/60 text-[10px] px-2 py-0.5 rounded font-mono flex items-center gap-1" title="Chỉ gửi riêng tới ${sheetIds.length} bảng tính"><i class="fa-solid fa-table-cells text-emerald-400"></i> ${sheetIds.length} Sheet riêng</span>`;
                } else {
                    sheetBadge = `<span class="bg-slate-800/80 text-amber-400 border border-amber-800/40 text-[10px] px-2 py-0.5 rounded font-mono flex items-center gap-1" title="Chưa tích chọn sheet nào"><i class="fa-solid fa-circle-exclamation text-amber-400"></i> 0 Sheet</span>`;
                }

                return `
                    <div class="bg-slate-900 border ${p.is_active ? 'border-slate-800 hover:border-cyan-500/50' : 'border-rose-900/40 opacity-80'} rounded-2xl p-5 shadow-xl transition-all duration-200 flex flex-col justify-between relative group">
                        <div>
                            <!-- Header Card: Page ID, Status & Active Toggle -->
                            <div class="flex items-start justify-between gap-3 mb-3">
                                <div>
                                    <div class="flex items-center gap-2 mb-1 flex-wrap">
                                        <span class="font-mono text-xs font-bold text-cyan-400 bg-cyan-950/80 border border-cyan-800/80 px-2 py-0.5 rounded-lg flex items-center gap-1">
                                            <i class="fa-solid fa-link text-[10px]"></i>
                                            <span>/p/${escapeHTML(p.page_id)}</span>
                                        </span>
                                        ${domainDisplay}
                                        ${statusBadge}
                                        ${sheetBadge}
                                    </div>
                                    <h3 class="text-sm font-bold text-white group-hover:text-cyan-300 transition line-clamp-1" title="${escapeHTML(p.name)}">
                                        ${escapeHTML(p.name)}
                                    </h3>
                                </div>

                                <!-- Feature Flag Active Toggle -->
                                <div class="flex flex-col items-end flex-shrink-0">
                                    <label class="relative inline-flex items-center cursor-pointer" title="${p.is_active ? 'Đang bật nhận khách - Bấm để tạm ngưng' : 'Đang tạm dừng - Bấm để kích hoạt'}">
                                        <input type="checkbox" ${p.is_active ? 'checked' : ''} onchange="toggleLandingActive('${escapeHTML(p.domain)}', '${escapeHTML(p.page_id)}', this.checked)" class="sr-only peer">
                                        <div class="w-9 h-5 bg-slate-800 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-slate-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-emerald-600"></div>
                                    </label>
                                    <span class="text-[9px] mt-1 font-mono ${p.is_active ? 'text-emerald-400 font-bold' : 'text-slate-500'}">
                                        ${p.is_active ? 'ACTIVE' : 'OFF'}
                                    </span>
                                </div>
                            </div>

                            <!-- Performance Stats Row -->
                            <div class="grid grid-cols-3 gap-2 bg-slate-950/80 border border-slate-800 rounded-xl p-2.5 my-3 text-center font-mono">
                                <div>
                                    <div class="text-[10px] text-slate-500 uppercase">Lượt Xem</div>
                                    <div class="text-sm font-bold text-blue-400">${(p.views_count || 0).toLocaleString('vi-VN')}</div>
                                </div>
                                <div>
                                    <div class="text-[10px] text-slate-500 uppercase">Leads</div>
                                    <div class="text-sm font-bold text-orange-400">${(p.leads_count || 0).toLocaleString('vi-VN')}</div>
                                </div>
                                <div>
                                    <div class="text-[10px] text-slate-500 uppercase">CR (%)</div>
                                    <div class="text-sm font-bold text-emerald-400">${convRate}%</div>
                                </div>
                            </div>

                            <!-- Meta details -->
                            <div class="text-[11px] text-slate-400 space-y-1 mb-4">
                                <div class="flex items-center justify-between">
                                    <span class="text-slate-500">Nguồn mặc định:</span>
                                    <span class="text-slate-300 font-mono">${escapeHTML(p.nguon_default || 'landing_chinh')}</span>
                                </div>
                                <div class="flex items-center justify-between">
                                    <span class="text-slate-500">Cập nhật lúc:</span>
                                    <span class="text-slate-400 font-mono">${escapeHTML(p.updated_at || '')}</span>
                                </div>
                            </div>
                        </div>

                        <!-- Action Buttons Grid -->
                        <div class="pt-3 border-t border-slate-800 space-y-2">
                            <!-- Primary Row: Code Studio, Settings & Preview -->
                            <div class="grid grid-cols-3 gap-1.5">
                                <button onclick="openCodeStudio('${escapeHTML(p.domain)}', '${escapeHTML(p.page_id)}')" class="bg-cyan-600/90 hover:bg-cyan-500 text-white font-bold text-[11px] py-1.5 px-2 rounded-lg transition shadow flex items-center justify-center gap-1" title="Mở trình soạn thảo Code HTML/CSS/JS">
                                    <i class="fa-solid fa-code"></i>
                                    <span>Code Studio</span>
                                </button>
                                <button onclick="openLandingSettings('${escapeHTML(p.domain)}', '${escapeHTML(p.page_id)}')" class="bg-slate-800 hover:bg-slate-700 text-slate-200 font-semibold text-[11px] py-1.5 px-2 rounded-lg border border-slate-700 transition flex items-center justify-center gap-1" title="Cài đặt Domain, SEO, Pixel, Form, Hotline">
                                    <i class="fa-solid fa-sliders"></i>
                                    <span>Cài Đặt</span>
                                </button>
                                <button onclick="openLandingPreview('${escapeHTML(p.page_id)}', '${escapeHTML(p.domain)}')" class="bg-slate-800 hover:bg-slate-700 text-slate-200 font-semibold text-[11px] py-1.5 px-2 rounded-lg border border-slate-700 transition flex items-center justify-center gap-1" title="Xem trước trực tiếp Responsive">
                                    <i class="fa-solid fa-eye"></i>
                                    <span>Preview</span>
                                </button>
                            </div>

                            <!-- Secondary Row: Deploy, Duplicate, Open & Delete -->
                            <div class="flex items-center justify-between gap-1 text-[11px]">
                                <button onclick="handleDeployLandingClick('${escapeHTML(p.domain)}', '${escapeHTML(p.page_id)}', '${escapeHTML(p.path_prefix || '/')}')" class="text-emerald-400 hover:text-emerald-300 font-bold px-2 py-1 rounded hover:bg-slate-800 transition flex items-center gap-1" title="Triển khai Landing lên tên miền và đường dẫn">
                                    <i class="fa-solid fa-rocket"></i>
                                    <span>Deploy</span>
                                </button>
                                <button onclick="duplicateLanding('${escapeHTML(p.domain)}', '${escapeHTML(p.page_id)}')" class="text-slate-300 hover:text-white px-2 py-1 rounded hover:bg-slate-800 transition flex items-center gap-1" title="Nhân bản sang trang mới">
                                    <i class="fa-solid fa-copy"></i>
                                    <span>Nhân bản</span>
                                </button>
                                <a href="${publicUrl}" target="_blank" class="text-sky-400 hover:text-sky-300 px-2 py-1 rounded hover:bg-slate-800 transition flex items-center gap-1" title="Mở trang web trong tab mới">
                                    <i class="fa-solid fa-arrow-up-right-from-square"></i>
                                    <span>Mở Link</span>
                                </a>
                                <button onclick="deleteLanding('${escapeHTML(p.domain)}', '${escapeHTML(p.page_id)}')" class="text-rose-400 hover:text-rose-300 p-1.5 rounded hover:bg-slate-800 transition" title="Xóa landing page này">
                                    <i class="fa-solid fa-trash-can"></i>
                                </button>
                            </div>
                        </div>
                    </div>
                `;
            }).join('');
        }

        // 17.4. Bật / Tắt kích hoạt Landing Page (Feature Flag)
        async function toggleLandingActive(domain, pageId, isActive) {
            try {
                const res = await fetch('/api/landings/toggle', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ domain: domain, page_id: pageId, is_active: isActive })
                });
                if (!res.ok) throw new Error('Không thể cập nhật trạng thái');
                const data = await res.json();
                loadLandingPages();
            } catch (e) {
                alert('Lỗi: ' + e.message);
                loadLandingPages();
            }
        }

        // 17.5. Deploy Landing Page lên Tên Miền & Thư Mục (Multi-Domain Routing)
        async function handleDeployLandingClick(domain, pageId, pathPrefix) {
            if (typeof openDeployRouteModal === 'function') {
                await openDeployRouteModal(null, domain, pageId, pathPrefix || '/');
            } else if (typeof window.openDeployRouteModal === 'function') {
                await window.openDeployRouteModal(null, domain, pageId, pathPrefix || '/');
            } else {
                alert('Đang nạp mô-đun Tên Miền, vui lòng thử lại sau giây lát...');
            }
        }
        window.handleDeployLandingClick = handleDeployLandingClick;

        // Deploy nhanh Landing Page (Fallback)
        async function quickDeployLanding(domain, pageId) {
            try {
                // Lấy code hiện tại của landing
                const resGet = await fetch(`/api/landings/get?page_id=${encodeURIComponent(pageId)}&domain=${encodeURIComponent(domain)}`);
                if (!resGet.ok) throw new Error('Không thể tải code để xuất bản');
                const dataGet = await resGet.json();
                const codeHTML = dataGet.landing.code_html || '';

                const resDeploy = await fetch('/api/landings/deploy', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ domain: domain || '', page_id: pageId, code_html: codeHTML })
                });
                if (!resDeploy.ok) throw new Error('Xuất bản thất bại');
                const deployResult = await resDeploy.json();
                const targetUrl = (deployResult.public_url && deployResult.public_url.startsWith('http'))
                    ? deployResult.public_url
                    : `${window.location.origin}${deployResult.public_url || ('/p/' + pageId)}`;
                alert(`🚀 ${deployResult.message}\nLink trang hoạt động: ${targetUrl}`);
                loadLandingPages();
            } catch (e) {
                alert('Lỗi Deploy: ' + e.message);
            }
        }

        // 17.6. Nhân bản Landing Page
        async function duplicateLanding(sourceDomain, sourcePageId) {
            const newId = prompt('Nhập Mã Landing mới (slug không dấu, vd: landing_vip_2026):', sourcePageId + '_copy');
            if (!newId || !newId.trim()) return;
            const newName = prompt('Nhập Tên Gợi Nhớ cho Landing mới:', 'Bản sao của ' + sourcePageId);
            if (!newName || !newName.trim()) return;

            try {
                const res = await fetch('/api/landings/duplicate', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        source_domain: sourceDomain,
                        source_page_id: sourcePageId,
                        new_page_id: newId.trim(),
                        new_name: newName.trim()
                    })
                });
                if (!res.ok) throw new Error('Không thể nhân bản');
                alert('Đã nhân bản landing page thành công!');
                loadLandingPages();
            } catch (e) {
                alert('Lỗi: ' + e.message);
            }
        }

        async function duplicateTemplate(templateId) {
            duplicateLanding('', templateId);
        }

        // 17.7. Xóa Landing Page
        async function deleteLanding(domain, pageId) {
            if (!confirm(`Bạn có chắc chắn muốn xóa Landing Page '${pageId}' và toàn bộ tệp tin của trang này không? Hành động này không thể hoàn tác.`)) return;

            try {
                const res = await fetch(`/api/landings/delete?page_id=${encodeURIComponent(pageId)}&domain=${encodeURIComponent(domain)}`, {
                    method: 'DELETE'
                });
                if (!res.ok) throw new Error('Không thể xóa');
                alert(`Đã xóa Landing Page '${pageId}' thành công`);
                loadLandingPages();
            } catch (e) {
                alert('Lỗi: ' + e.message);
            }
        }

        // ============================================================
        // 17.8. LANDING SETTINGS MODAL CONTROLLERS (4 TABS)
        // Lưu ý: chỉ còn Domain / Pixel / SEO / Google Sheets.
        // Mọi cấu hình "ép ghi đè" nội dung landing (hotline, zalo, hero title,
        // metric box, form scanner, dynamic content) đã bị gỡ bỏ — HTML viết
        // tay trong Code Studio là nguồn sự thật duy nhất.
        // ============================================================
        function switchSettingsTab(tab) {
            ['domain', 'pixel', 'seo', 'sheets'].forEach(t => {
                const btn = document.getElementById('cfgTabBtn' + (t.charAt(0).toUpperCase() + t.slice(1)));
                const sec = document.getElementById('cfgTabSec' + (t.charAt(0).toUpperCase() + t.slice(1)));
                if (btn) {
                    btn.classList.remove('border-b-2', 'border-cyan-400', 'text-white', 'font-bold');
                    btn.classList.add('text-slate-400', 'font-semibold');
                }
                if (sec) sec.classList.add('hidden');
            });

            const activeBtn = document.getElementById('cfgTabBtn' + (tab.charAt(0).toUpperCase() + tab.slice(1)));
            const activeSec = document.getElementById('cfgTabSec' + (tab.charAt(0).toUpperCase() + tab.slice(1)));
            if (activeBtn) {
                activeBtn.classList.add('border-b-2', 'border-cyan-400', 'text-white', 'font-bold');
                activeBtn.classList.remove('text-slate-400', 'font-semibold');
            }
            if (activeSec) activeSec.classList.remove('hidden');
        }

        // 17.8.1. Cấu hình đồng bộ Google Sheets riêng biệt theo từng Landing Page
        let cachedAllSheets = [];

        async function loadSheetsForLandingSettings(selectedIds = [], syncMode = 'selected') {
            const listEl = document.getElementById('cfgSheetCheckboxesList');
            if (!listEl) return;
            listEl.innerHTML = '<div class="text-xs text-slate-500 py-3 text-center"><i class="fa-solid fa-spinner fa-spin mr-1"></i> Đang tải danh sách Google Sheets...</div>';

            try {
                const res = await fetch('/api/integrations/sheets/configs');
                const json = await res.json();
                cachedAllSheets = (json && json.success && json.configs) ? json.configs : [];

                if (cachedAllSheets.length === 0) {
                    listEl.innerHTML = `
                        <div class="p-4 bg-slate-950 border border-slate-800 rounded-xl text-center text-xs text-slate-400 space-y-2">
                            <i class="fa-solid fa-circle-info text-amber-400 text-base"></i>
                            <div>Chưa có cấu hình Google Sheet nào trong hệ thống CRM.</div>
                            <div>
                                <button type="button" onclick="closeLandingSettingsModal(); switchMainTab('sheets');" class="bg-emerald-600 hover:bg-emerald-500 text-white text-[11px] font-bold px-3 py-1.5 rounded-lg transition inline-flex items-center gap-1.5">
                                    <i class="fa-solid fa-plus"></i> Kết Nối Google Sheet Ngay
                                </button>
                            </div>
                        </div>
                    `;
                    updateSelectedSheetsCounter();
                    return;
                }

                const selIdSet = new Set((selectedIds || []).map(x => parseInt(x, 10)));
                listEl.innerHTML = cachedAllSheets.map(c => {
                    const isChecked = selIdSet.has(c.id);
                    return `
                        <label class="flex items-start justify-between gap-3 p-3 bg-slate-950/80 rounded-xl border ${isChecked ? 'border-emerald-600/70 bg-emerald-950/20' : 'border-slate-800'} hover:border-slate-700 cursor-pointer transition text-xs">
                            <div class="flex items-start gap-2.5">
                                <input type="checkbox" name="cfgSheetCheckbox" value="${c.id}" ${isChecked ? 'checked' : ''} onchange="onSheetCheckboxChange(this)" class="mt-0.5 rounded bg-slate-900 border-slate-700 text-emerald-500 focus:ring-0">
                                <div>
                                    <div class="font-bold text-white flex items-center gap-1.5">
                                        <i class="fa-solid fa-table text-emerald-400 text-[11px]"></i>
                                        <span>${escapeHTML(c.name)}</span>
                                        <span class="text-[10px] font-mono text-slate-400 bg-slate-900 px-1.5 py-0.2 rounded border border-slate-800">#${c.id}</span>
                                    </div>
                                    <div class="text-[11px] text-slate-400 mt-0.5">
                                        <span>Bảng: <strong class="text-slate-300">${escapeHTML(c.spreadsheet_name || c.spreadsheet_id)}</strong></span>
                                        <span class="text-slate-600 mx-1">&bull;</span>
                                        <span>Tab: <strong class="text-emerald-400">${escapeHTML(c.sheet_tab_name)}</strong></span>
                                    </div>
                                    <div class="text-[10px] text-slate-500 font-mono mt-0.5 truncate max-w-sm">
                                        ID: ${escapeHTML(c.spreadsheet_id)}
                                    </div>
                                </div>
                            </div>
                            <div class="flex flex-col items-end gap-1 flex-shrink-0">
                                <span class="text-[10px] px-2 py-0.5 rounded-full font-bold ${c.is_active ? 'bg-emerald-950 text-emerald-400 border border-emerald-800/80' : 'bg-slate-900 text-slate-500 border border-slate-800'}">
                                    ${c.is_active ? 'Đang bật' : 'Tạm tắt'}
                                </span>
                                ${c.last_synced_at ? `<span class="text-[9px] text-slate-500 font-mono">${escapeHTML(c.last_synced_at)}</span>` : '<span class="text-[9px] text-slate-600 font-mono">Chưa sync</span>'}
                            </div>
                        </label>
                    `;
                }).join('');

                updateSelectedSheetsCounter();
            } catch (e) {
                listEl.innerHTML = `<div class="text-xs text-rose-400 py-2">Lỗi tải danh sách Google Sheets: ${escapeHTML(e.message)}</div>`;
            }
        }

        function onSheetCheckboxChange(checkbox) {
            const label = checkbox.closest('label');
            if (label) {
                if (checkbox.checked) {
                    label.classList.add('border-emerald-600/70', 'bg-emerald-950/20');
                    label.classList.remove('border-slate-800');
                } else {
                    label.classList.remove('border-emerald-600/70', 'bg-emerald-950/20');
                    label.classList.add('border-slate-800');
                }
            }
            updateSelectedSheetsCounter();
        }

        function selectAllSheets(select) {
            const boxes = document.querySelectorAll('input[name="cfgSheetCheckbox"]');
            boxes.forEach(b => {
                b.checked = select;
                const label = b.closest('label');
                if (label) {
                    if (select) {
                        label.classList.add('border-emerald-600/70', 'bg-emerald-950/20');
                        label.classList.remove('border-slate-800');
                    } else {
                        label.classList.remove('border-emerald-600/70', 'bg-emerald-950/20');
                        label.classList.add('border-slate-800');
                    }
                }
            });
            updateSelectedSheetsCounter();
        }

        function updateSelectedSheetsCounter() {
            const checked = document.querySelectorAll('input[name="cfgSheetCheckbox"]:checked').length;
            const counterEl = document.getElementById('cfgSelectedSheetsCounter');
            if (counterEl) counterEl.innerText = `Đã chọn: ${checked} sheet`;
            const badgeEl = document.getElementById('cfgSheetBadgeCount');
            if (badgeEl) badgeEl.innerText = checked;
        }

        function toggleSheetSelectionUI() {
            const modeEl = document.querySelector('input[name="cfgSheetSyncMode"]:checked');
            const container = document.getElementById('cfgSheetSelectionContainer');
            if (!container) return;
            if (modeEl && modeEl.value === 'disabled') {
                container.classList.add('opacity-30', 'pointer-events-none');
            } else {
                container.classList.remove('opacity-30', 'pointer-events-none');
            }
        }

        // ============================================================
        // 17.8. MODAL KHỞI TẠO LANDING PAGE MỚI TINH GỌN & CODE STUDIO
        // ============================================================
        async function openCreateLandingModal() {
            const autoId = 'landing_' + Math.random().toString(36).substring(2, 8);
            const nameEl = document.getElementById('newLandingName');
            const pageIdEl = document.getElementById('newLandingPageId');
            const domainEl = document.getElementById('newLandingDomain');
            const codeEl = document.getElementById('newLandingInitialCode');
            const pasteBox = document.getElementById('newLandingPasteCodeContainer');
            const radioClean = document.querySelector('input[name="newLandingInitType"][value="clean"]');

            if (nameEl) nameEl.value = '';
            if (pageIdEl) pageIdEl.value = autoId;
            if (codeEl) codeEl.value = '';
            if (pasteBox) pasteBox.classList.add('hidden');
            if (radioClean) radioClean.checked = true;

            // Đảm bảo nạp danh sách tên miền vào dropdown
            let doms = (window.allDomains && window.allDomains.length > 0) ? window.allDomains : [];
            if (doms.length === 0 && typeof ensureDomainsLoaded === 'function') {
                doms = await ensureDomainsLoaded();
            }
            if (domainEl) {
                domainEl.innerHTML = '<option value="">-- Mặc định (Tất cả domain) --</option>';
                doms.forEach(d => {
                    const opt = document.createElement('option');
                    opt.value = d.domain;
                    opt.textContent = `${d.domain} (${d.dns_status === 'active' ? 'DNS OK' : 'Chưa DNS'})`;
                    domainEl.appendChild(opt);
                });
            }

            toggleInitTypeUI();
            const modal = document.getElementById('modalCreateLanding');
            if (modal) modal.classList.remove('hidden');
        }

        function closeCreateLandingModal() {
            const modal = document.getElementById('modalCreateLanding');
            if (modal) modal.classList.add('hidden');
        }

        function autoGeneratePageId(name) {
            if (!name) return;
            let slug = name.toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '');
            slug = slug.replace(/[đĐ]/g, 'd');
            slug = slug.replace(/[^a-z0-9]/g, '_').replace(/_+/g, '_').replace(/^_|_$/g, '');
            const el = document.getElementById('newLandingPageId');
            if (el) el.value = slug || 'landing_' + Math.random().toString(36).substring(2, 8);
        }

        function toggleInitTypeUI() {
            const selected = document.querySelector('input[name="newLandingInitType"]:checked')?.value || 'clean';
            const pasteContainer = document.getElementById('newLandingPasteCodeContainer');
            const labelClean = document.getElementById('labelInitClean');
            const labelPaste = document.getElementById('labelInitPaste');

            if (selected === 'paste') {
                if (pasteContainer) pasteContainer.classList.remove('hidden');
                if (labelPaste) {
                    labelPaste.className = 'relative flex flex-col p-3 rounded-xl border border-cyan-500/50 bg-cyan-950/20 cursor-pointer hover:border-cyan-400 transition';
                }
                if (labelClean) {
                    labelClean.className = 'relative flex flex-col p-3 rounded-xl border border-slate-800 bg-slate-950/60 cursor-pointer hover:border-slate-700 transition';
                }
            } else {
                if (pasteContainer) pasteContainer.classList.add('hidden');
                if (labelClean) {
                    labelClean.className = 'relative flex flex-col p-3 rounded-xl border border-cyan-500/50 bg-cyan-950/20 cursor-pointer hover:border-cyan-400 transition';
                }
                if (labelPaste) {
                    labelPaste.className = 'relative flex flex-col p-3 rounded-xl border border-slate-800 bg-slate-950/60 cursor-pointer hover:border-slate-700 transition';
                }
            }
        }

        async function submitCreateLandingForm(e) {
            if (e && e.preventDefault) e.preventDefault();
            const name = (document.getElementById('newLandingName')?.value || '').trim();
            let pageId = (document.getElementById('newLandingPageId')?.value || '').trim();
            const domain = (document.getElementById('newLandingDomain')?.value || '').trim();
            const initType = document.querySelector('input[name="newLandingInitType"]:checked')?.value || 'clean';
            const initialCode = (document.getElementById('newLandingInitialCode')?.value || '').trim();

            if (!name) {
                alert('Vui lòng nhập Tên Gợi Nhớ cho Landing Page.');
                return;
            }
            if (!pageId) {
                autoGeneratePageId(name);
                pageId = (document.getElementById('newLandingPageId')?.value || '').trim();
            }
            pageId = pageId.toLowerCase().replace(/[^a-z0-9_]/g, '_').replace(/_+/g, '_').replace(/^_|_$/g, '');

            let htmlContent = '';
            if (initType === 'paste' && initialCode) {
                htmlContent = initialCode;
            } else {
                htmlContent = `<!DOCTYPE html>
<html lang="vi">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>${escapeHtml(name)}</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.4.0/css/all.min.css">
    <link rel="stylesheet" href="style.css">
</head>
<body class="bg-slate-950 text-white min-h-screen">
    <!-- Landing Page: ${escapeHtml(name)} -->
    <header class="py-6 px-4 border-b border-slate-800 max-w-6xl mx-auto flex items-center justify-between">
        <div class="flex items-center gap-2">
            <span class="text-cyan-400 text-xl font-bold"><i class="fa-solid fa-cube mr-1.5"></i>Apex</span>
        </div>
        <a href="#register" class="btn bg-cyan-600 hover:bg-cyan-500 text-white font-bold text-xs px-4 py-2 rounded-xl transition">Đăng Ký Ngay</a>
    </header>

    <main class="max-w-4xl mx-auto py-16 px-4 text-center">
        <h1 class="hero-title text-3xl sm:text-5xl font-black mb-4 tracking-tight leading-tight bg-gradient-to-r from-white via-slate-100 to-slate-400 bg-clip-text text-transparent">
            ${escapeHtml(name)}
        </h1>
        <p class="text-sm sm:text-base text-slate-400 max-w-2xl mx-auto mb-8">
            Chào mừng bạn đến với trang Landing Page mới. Bạn có thể mở Code Studio để tạo tệp tin, soạn thảo mã nguồn và quét tùy chỉnh nhanh nội dung.
        </p>
        <div class="flex items-center justify-center gap-3">
            <a href="#register" class="btn bg-gradient-to-r from-cyan-600 to-indigo-600 hover:from-cyan-500 hover:to-indigo-500 text-white font-bold text-sm px-6 py-3 rounded-xl transition shadow-lg">
                Bắt Đầu Ngay
            </a>
        </div>
    </main>

    <footer class="py-8 border-t border-slate-900 text-center text-xs text-slate-500">
        <p>© 2026 ${escapeHtml(name)}. Mọi quyền được bảo lưu.</p>
    </footer>

    <script src="script.js"></script>
</body>
</html>`;
            }

            const btn = document.getElementById('btnSubmitCreateLanding');
            const originalHtml = btn ? btn.innerHTML : '';
            if (btn) {
                btn.disabled = true;
                btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin mr-1"></i> Đang khởi tạo...';
            }

            try {
                const res = await fetch('/api/landings/save', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        page_id: pageId,
                        name: name,
                        domain: domain,
                        path_prefix: '/',
                        nguon_default: 'landing_chinh',
                        is_active: true,
                        code_html: htmlContent,
                        settings: {}
                    })
                });

                if (!res.ok) {
                    const errText = await res.text();
                    throw new Error(errText || 'Lỗi khởi tạo landing page');
                }

                closeCreateLandingModal();
                await loadLandingPages();
                if (typeof loadDomains === 'function') loadDomains();

                // Lập tức đưa người dùng vào Code Studio để quản lý thư mục, file và code
                if (typeof openCodeStudio === 'function') {
                    openCodeStudio(domain, pageId);
                }
            } catch (err) {
                alert('Lỗi tạo Landing Page: ' + err.message);
            } finally {
                if (btn) {
                    btn.disabled = false;
                    btn.innerHTML = originalHtml;
                }
            }
        }

        async function openLandingSettings(domain, pageId) {
            try {
                // Đảm bảo nạp đầy đủ danh sách tên miền
                if (typeof ensureDomainsLoaded === 'function') {
                    await ensureDomainsLoaded();
                }

                const res = await fetch(`/api/landings/get?page_id=${encodeURIComponent(pageId)}&domain=${encodeURIComponent(domain)}`);
                if (!res.ok) throw new Error('Không thể tải thông tin landing');
                const data = await res.json();
                const p = data.landing;
                const s = p.settings || {};

                document.getElementById('cfgPageId').value = p.page_id;
                document.getElementById('cfgPageId').setAttribute('readonly', 'true');
                document.getElementById('cfgPageName').value = p.name || '';
                document.getElementById('cfgDomain').value = p.domain || '';
                document.getElementById('cfgPathPrefix').value = p.path_prefix || s.path_prefix || '/';
                if (typeof populateDomainDropdowns === 'function') {
                    populateDomainDropdowns();
                }
                const curDomSel = document.getElementById('cfgDomainSelect');
                if (curDomSel) curDomSel.value = p.domain || '';
                document.getElementById('cfgNguonDefault').value = p.nguon_default || 'landing_chinh';
                document.getElementById('cfgIsActive').checked = p.is_active;
                document.getElementById('cfgRedirectUrl').value = p.redirect_url || '';

                document.getElementById('cfgFacebookPixel').value = s.facebook_pixel_id || '';
                document.getElementById('cfgGoogleAnalytics').value = s.google_analytics_id || '';
                document.getElementById('cfgTikTokPixel').value = s.tiktok_pixel_id || '';
                document.getElementById('cfgCustomHead').value = s.custom_head_scripts || '';
                document.getElementById('cfgCustomBody').value = s.custom_body_scripts || '';

                // Cấu hình Google Sheets riêng biệt cho Landing Page này
                const syncMode = s.google_sheet_sync_mode || 'selected';
                const sheetIds = s.google_sheet_ids || [];
                if (syncMode === 'all') {
                    const r = document.getElementById('cfgSyncModeAll');
                    if (r) r.checked = true;
                } else if (syncMode === 'disabled') {
                    const r = document.getElementById('cfgSyncModeDisabled');
                    if (r) r.checked = true;
                } else {
                    const r = document.getElementById('cfgSyncModeSelected');
                    if (r) r.checked = true;
                }
                loadSheetsForLandingSettings(sheetIds, syncMode);
                toggleSheetSelectionUI();

                switchSettingsTab('domain');
                document.getElementById('landingSettingsModal').classList.remove('hidden');
            } catch (e) {
                alert('Lỗi: ' + e.message);
            }
        }

        function closeLandingSettingsModal() {
            document.getElementById('landingSettingsModal').classList.add('hidden');
        }

        function escapeHtml(str) {
            if (!str) return '';
            return String(str)
                .replace(/&/g, '&amp;')
                .replace(/</g, '&lt;')
                .replace(/>/g, '&gt;')
                .replace(/"/g, '&quot;')
                .replace(/'/g, '&#039;');
        }


        function closeLandingSettings() {
            document.getElementById('landingSettingsModal').classList.add('hidden');
        }

        async function saveLandingSettings() {
            const pageId = document.getElementById('cfgPageId').value.trim();
            const pageName = document.getElementById('cfgPageName').value.trim();
            if (!pageId) {
                alert('Vui lòng nhập Mã Định Danh (Page ID)');
                return;
            }

            // Đọc cấu hình Google Sheets riêng biệt
            const syncModeEl = document.querySelector('input[name="cfgSheetSyncMode"]:checked');
            const syncMode = syncModeEl ? syncModeEl.value : 'selected';
            const checkedBoxes = document.querySelectorAll('input[name="cfgSheetCheckbox"]:checked');
            const selectedSheetIds = Array.from(checkedBoxes).map(cb => parseInt(cb.value, 10)).filter(id => !isNaN(id));

            const domainVal = (document.getElementById('cfgDomain')?.value || document.getElementById('cfgDomainSelect')?.value || '').trim();
            let pathPrefixVal = (document.getElementById('cfgPathPrefix')?.value || '/').trim();
            if (!pathPrefixVal.startsWith('/')) pathPrefixVal = '/' + pathPrefixVal;
            if (pathPrefixVal.length > 1) pathPrefixVal = pathPrefixVal.replace(/\/+$/, '');

            // Bảo toàn các key cấu hình không còn UI quản lý (hotline, zalo_link,
            // form_mapping, brand_header, tieu_de, metric_boxes, thoi_gian_chia_se,
            // khoa_hoc, form_config...) để không làm mất dữ liệu đang chạy.
            // Hệ thống KHÔNG còn ghi đè nội dung landing từ các key này — HTML
            // viết tay trong Code Studio là nguồn sự thật duy nhất.
            let existingSettings = {};
            try {
                const gRes = await fetch(`/api/landings/get?page_id=${encodeURIComponent(pageId)}&domain=${encodeURIComponent(domainVal)}`);
                if (gRes.ok) {
                    const gData = await gRes.json();
                    existingSettings = (gData.landing && gData.landing.settings) || {};
                }
            } catch (_) {}

            const managedSettings = {
                path_prefix: pathPrefixVal,
                title: document.getElementById('cfgMetaTitle').value.trim(),
                meta_description: document.getElementById('cfgMetaDesc').value.trim(),
                og_image: document.getElementById('cfgOgImage').value.trim(),
                facebook_pixel_id: document.getElementById('cfgFacebookPixel').value.trim(),
                google_analytics_id: document.getElementById('cfgGoogleAnalytics').value.trim(),
                tiktok_pixel_id: document.getElementById('cfgTikTokPixel').value.trim(),
                custom_head_scripts: document.getElementById('cfgCustomHead').value.trim(),
                custom_body_scripts: document.getElementById('cfgCustomBody').value.trim(),
                google_sheet_sync_mode: syncMode,
                google_sheet_ids: selectedSheetIds,
            };

            const settings = Object.assign({}, existingSettings, managedSettings);
            settings.path_prefix = pathPrefixVal;

            const payload = {
                page_id: pageId,
                name: pageName || pageId,
                domain: domainVal,
                path_prefix: pathPrefixVal,
                nguon_default: document.getElementById('cfgNguonDefault').value.trim() || 'landing_chinh',
                is_active: document.getElementById('cfgIsActive').checked,
                redirect_url: document.getElementById('cfgRedirectUrl').value.trim(),
                settings: settings
            };

            try {
                const res = await fetch('/api/landings/save', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(payload)
                });
                if (!res.ok) throw new Error('Không thể lưu cài đặt');
                alert('✅ Đã lưu cài đặt Landing Page thành công!');
                closeLandingSettingsModal();
                loadLandingPages();
                if (typeof loadDomains === 'function') loadDomains();

                // Đồng bộ thông tin cấu hình vào Code Studio nếu đang mở
                if (currentStudioLanding && currentStudioLanding.page_id === payload.page_id) {
                    currentStudioLanding.name = payload.name;
                    currentStudioLanding.domain = payload.domain;
                    currentStudioLanding.is_active = payload.is_active;
                    currentStudioLanding.settings = payload.settings;
                    const nameEl = document.getElementById('csLandingName');
                    if (nameEl) nameEl.innerText = payload.name || payload.page_id;
                    refreshStudioPreview();
                }
            } catch (e) {
                alert('Lỗi lưu cài đặt: ' + e.message);
            }
        }

        // Mở cài đặt cấu hình trực tiếp từ bên trong Code Studio
        function openStudioSettings() {
            if (!currentStudioLanding) return;
            openLandingSettings(currentStudioLanding.domain || '', currentStudioLanding.page_id);
        }



        function openLandingPreview(pageId, domain) {
            currentModalPreviewPageId = pageId;
            currentModalPreviewDomain = domain || '';
            document.getElementById('previewPageIdBadge').innerText = pageId;
            setModalPreviewDevice('pc');
            refreshModalPreview();
            document.getElementById('landingPreviewModal').classList.remove('hidden');
        }

        function closeLandingPreviewModal() {
            document.getElementById('landingPreviewModal').classList.add('hidden');
        }

        function setModalPreviewDevice(mode) {
            modalPreviewDeviceMode = mode;
            const btnPC = document.getElementById('pvBtnPC');
            const btnLaptop = document.getElementById('pvBtnLaptop');
            const btnMobile = document.getElementById('pvBtnMobile');
            const wrapper = document.getElementById('pvFrameWrapper');

            [btnPC, btnLaptop, btnMobile].forEach(b => {
                b.className = 'px-3 py-1.5 rounded-lg font-bold transition flex items-center gap-1.5 text-slate-300 hover:text-white';
            });

            if (mode === 'pc') {
                btnPC.className = 'px-3 py-1.5 rounded-lg font-bold transition flex items-center gap-1.5 bg-cyan-600 text-white';
                wrapper.style.width = '100%';
                wrapper.style.height = '100%';
                wrapper.style.maxWidth = 'none';
                wrapper.style.borderRadius = '16px';
            } else if (mode === 'laptop') {
                btnLaptop.className = 'px-3 py-1.5 rounded-lg font-bold transition flex items-center gap-1.5 bg-cyan-600 text-white';
                wrapper.style.width = '1024px';
                wrapper.style.height = '100%';
                wrapper.style.maxWidth = '100%';
                wrapper.style.borderRadius = '20px';
            } else if (mode === 'mobile') {
                btnMobile.className = 'px-3 py-1.5 rounded-lg font-bold transition flex items-center gap-1.5 bg-cyan-600 text-white';
                wrapper.style.width = '390px';
                wrapper.style.height = '844px';
                wrapper.style.maxWidth = '100%';
                wrapper.style.borderRadius = '36px';
                wrapper.style.border = '5px solid #334155';
            }
        }

        function refreshModalPreview() {
            if (!currentModalPreviewPageId) return;
            const iframe = document.getElementById('pvFrameIframe');
            iframe.src = `/api/landings/preview?page_id=${encodeURIComponent(currentModalPreviewPageId)}&domain=${encodeURIComponent(currentModalPreviewDomain || '')}&bypass=1&preview=1&v=${Date.now()}`;
        }

        function openModalPreviewNewTab() {
            if (!currentModalPreviewPageId) return;
            window.open(`/api/landings/preview?page_id=${encodeURIComponent(currentModalPreviewPageId)}&domain=${encodeURIComponent(currentModalPreviewDomain || '')}&bypass=1&preview=1`, '_blank');
        }

        // Điều hướng sang Visual Web Builder (Studio Workspace độc lập)
        function openCodeStudio(domain, pageId) {
            const pId = pageId || 'mxv_080_online';
            const dom = domain || '';
            const studioUrl = `studio.html?page_id=${encodeURIComponent(pId)}&domain=${encodeURIComponent(dom)}`;
            window.location.href = studioUrl;
        }

