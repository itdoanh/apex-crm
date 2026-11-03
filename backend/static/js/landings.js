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
        let studioDeviceMode = 'pc'; // 'pc', 'laptop', 'mobile'
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

                document.getElementById('kpiLadiTotal').innerText = allLandingsList.length;
                document.getElementById('kpiLadiActive').innerText = activeCount;
                document.getElementById('kpiLadiDraft').innerText = draftCount;
                document.getElementById('kpiLadiViews').innerText = totalViews.toLocaleString('vi-VN');
                document.getElementById('kpiLadiLeads').innerText = totalLeads.toLocaleString('vi-VN');
                document.getElementById('badgeLadiTotal').innerText = allLandingsList.length;

                filterLandingsGrid();
            } catch (e) {
                console.error('Lỗi loadLandingPages:', e);
            }
        }

        // 17.2. Lọc danh sách theo từ khóa và trạng thái
        function filterLandingsGrid() {
            const query = (document.getElementById('filterLadiSearch').value || '').toLowerCase().trim();
            const statusFilter = document.getElementById('filterLadiStatus').value;

            const filtered = allLandingsList.filter(p => {
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

            document.getElementById('ladiDisplayCount').innerText = filtered.length;
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

                const publicUrl = `/p/${encodeURIComponent(p.page_id)}`;

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
                                <button onclick="openLandingPreview('${escapeHTML(p.page_id)}')" class="bg-slate-800 hover:bg-slate-700 text-slate-200 font-semibold text-[11px] py-1.5 px-2 rounded-lg border border-slate-700 transition flex items-center justify-center gap-1" title="Xem trước trực tiếp Responsive">
                                    <i class="fa-solid fa-eye"></i>
                                    <span>Preview</span>
                                </button>
                            </div>

                            <!-- Secondary Row: Deploy, Duplicate, Open & Delete -->
                            <div class="flex items-center justify-between gap-1 text-[11px]">
                                <button onclick="quickDeployLanding('${escapeHTML(p.domain)}', '${escapeHTML(p.page_id)}')" class="text-emerald-400 hover:text-emerald-300 font-bold px-2 py-1 rounded hover:bg-slate-800 transition flex items-center gap-1" title="Xuất bản chính thức">
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

        // 17.5. Deploy nhanh Landing Page
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
                    body: JSON.stringify({ domain: domain, page_id: pageId, code_html: codeHTML })
                });
                if (!resDeploy.ok) throw new Error('Xuất bản thất bại');
                const deployResult = await resDeploy.json();
                alert(`🚀 ${deployResult.message}\nLink trang: ${window.location.origin}${deployResult.public_url}`);
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
        // 17.8. LANDING SETTINGS MODAL CONTROLLERS (6 TABS KÈM GOOGLE SHEETS)
        // ============================================================
        function switchSettingsTab(tab) {
            ['domain', 'form', 'pixel', 'event', 'seo', 'sheets'].forEach(t => {
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

        function openCreateLandingModal() {
            const autoId = 'landing_' + Math.random().toString(36).substring(2, 8);
            document.getElementById('cfgPageId').value = autoId;
            document.getElementById('cfgPageId').removeAttribute('readonly');
            document.getElementById('cfgPageName').value = 'Chiến Dịch Landing Page Mới';
            document.getElementById('cfgDomain').value = '';
            document.getElementById('cfgPathPrefix').value = '/';
            populateDomainDropdowns();
            const rDomainSel = document.getElementById('cfgDomainSelect');
            if (rDomainSel) rDomainSel.value = '';
            document.getElementById('cfgNguonDefault').value = 'landing_chinh';
            document.getElementById('cfgIsActive').checked = true;
            document.getElementById('cfgRedirectUrl').value = 'chucmung.html';
            document.getElementById('cfgHotline').value = '0984.386.538';
            document.getElementById('cfgZaloLink').value = 'https://zalo.me/0984386538';
            document.getElementById('cfgSuccessRedirect').value = 'chucmung.html';
            document.getElementById('cfgFacebookPixel').value = '1558828029597854';
            document.getElementById('cfgGoogleAnalytics').value = '';
            document.getElementById('cfgTikTokPixel').value = '';
            document.getElementById('cfgCustomHead').value = '';
            document.getElementById('cfgCustomBody').value = '';

            // Cấu hình Brand, Tiêu Đề và 3 Ô thông tin mặc định
            document.getElementById('cfgBrandHeader1').value = 'Bộ Công Thương';
            document.getElementById('cfgBrandHeader2').value = 'Sở giao dịch hàng hóa Việt Nam MXV';
            document.getElementById('cfgBrandLogo').value = 'assets/img/logo_APEX_K_NEN.webp';
            document.getElementById('cfgBrandMemberText').value = 'Thành viên 080 MXV';
            document.getElementById('cfgBadgeText').value = 'LỚP HỌC ONLINE MIỄN PHÍ';
            document.getElementById('cfgTitleLine1').value = 'GIẢI MÃ ĐẦU TƯ';
            document.getElementById('cfgTitleLine2').value = 'HÀNG HÓA';
            document.getElementById('cfgSubtitle').value = 'Hiểu thị trường – Nắm cơ hội – Quản trị rủi ro';
            document.getElementById('cfgBox1Top').value = '03/11/2026';
            document.getElementById('cfgBox1Bottom').value = '20:00 Online';
            document.getElementById('cfgBox2Top').value = 'Học Online';
            document.getElementById('cfgBox2Bottom').value = 'Qua Zoom';
            document.getElementById('cfgBox3Top').value = '2 Giờ';
            document.getElementById('cfgBox3Bottom').value = 'Thực chiến';

            document.getElementById('cfgEventTitle').value = 'Giải Mã Đầu Tư Hàng Hóa';
            document.getElementById('cfgTargetDate').value = '2026-11-03T20:00:00';
            document.getElementById('cfgEventQuota').value = 'Chỉ dành cho 100 suất';
            document.getElementById('cfgMetaTitle').value = 'Hội Thảo Giao Dịch Hàng Hóa - Khóa Học MXV 080';
            document.getElementById('cfgMetaDesc').value = 'Khóa học thực chiến đầu tư hàng hóa phái sinh cùng chuyên gia hàng đầu. Miễn phí tham gia online qua Zoom.';
            document.getElementById('cfgOgImage').value = '';

            // Cấu hình Form Mapping & Heuristics mặc định
            document.getElementById('cfgFormSelector').value = '';
            document.getElementById('cfgOptValidatePhone').checked = true;
            document.getElementById('cfgOptHoneypot').checked = true;
            document.getElementById('cfgOptUtmCapture').checked = true;
            document.getElementById('cfgOptCookie180d').checked = true;
            document.getElementById('scanFormStatus').innerHTML = `<span>Chưa thực hiện quét</span><span class="text-[10px] bg-slate-800 px-2 py-0.5 rounded text-slate-400">Sẵn sàng</span>`;
            renderScannedInputsTable([]);

            const rSel = document.getElementById('cfgSyncModeSelected');
            if (rSel) rSel.checked = true;
            loadSheetsForLandingSettings([], 'selected');
            toggleSheetSelectionUI();

            switchSettingsTab('domain');
            document.getElementById('landingSettingsModal').classList.remove('hidden');
        }

        async function openLandingSettings(domain, pageId) {
            try {
                const res = await fetch(`/api/landings/get?page_id=${encodeURIComponent(pageId)}&domain=${encodeURIComponent(domain)}`);
                if (!res.ok) throw new Error('Không thể tải thông tin landing');
                const data = await res.json();
                const p = data.landing;
                const s = p.settings || {};
                const tg = s.thoi_gian_chia_se || {};
                const kh = s.khoa_hoc || {};
                const fc = s.form_config || {};

                document.getElementById('cfgPageId').value = p.page_id;
                document.getElementById('cfgPageId').setAttribute('readonly', 'true');
                document.getElementById('cfgPageName').value = p.name || '';
                document.getElementById('cfgDomain').value = p.domain || '';
                document.getElementById('cfgPathPrefix').value = '/';
                populateDomainDropdowns();
                const curDomSel = document.getElementById('cfgDomainSelect');
                if (curDomSel) curDomSel.value = p.domain || '';
                document.getElementById('cfgNguonDefault').value = p.nguon_default || 'landing_chinh';
                document.getElementById('cfgIsActive').checked = p.is_active;
                document.getElementById('cfgRedirectUrl').value = p.redirect_url || '';

                document.getElementById('cfgHotline').value = s.hotline || '';
                document.getElementById('cfgZaloLink').value = s.zalo_link || '';
                document.getElementById('cfgSuccessRedirect').value = fc.success_redirect || p.redirect_url || 'chucmung.html';

                document.getElementById('cfgFacebookPixel').value = s.facebook_pixel_id || '';
                document.getElementById('cfgGoogleAnalytics').value = s.google_analytics_id || '';
                document.getElementById('cfgTikTokPixel').value = s.tiktok_pixel_id || '';
                document.getElementById('cfgCustomHead').value = s.custom_head_scripts || '';
                document.getElementById('cfgCustomBody').value = s.custom_body_scripts || '';

                const bh = s.brand_header || {};
                const td = s.tieu_de || {};
                const mb = s.metric_boxes || {};
                const b1 = mb.box_1 || {};
                const b2 = mb.box_2 || {};
                const b3 = mb.box_3 || {};

                document.getElementById('cfgBrandHeader1').value = bh.co_quan_1 || 'Bộ Công Thương';
                document.getElementById('cfgBrandHeader2').value = bh.co_quan_2 || 'Sở giao dịch hàng hóa Việt Nam MXV';
                document.getElementById('cfgBrandLogo').value = bh.logo_url || 'assets/img/logo_APEX_K_NEN.webp';
                document.getElementById('cfgBrandMemberText').value = bh.member_text || 'Thành viên 080 MXV';

                document.getElementById('cfgBadgeText').value = td.badge || 'LỚP HỌC ONLINE MIỄN PHÍ';
                document.getElementById('cfgTitleLine1').value = td.line_1 || 'GIẢI MÃ ĐẦU TƯ';
                document.getElementById('cfgTitleLine2').value = td.line_2 || 'HÀNG HÓA';
                document.getElementById('cfgSubtitle').value = td.mo_ta || 'Hiểu thị trường – Nắm cơ hội – Quản trị rủi ro';

                document.getElementById('cfgBox1Top').value = b1.top || (tg.ngay || '03/11/2026');
                document.getElementById('cfgBox1Bottom').value = b1.bottom || (tg.gio || '20:00 Online');
                document.getElementById('cfgBox2Top').value = b2.top || 'Học Online';
                document.getElementById('cfgBox2Bottom').value = b2.bottom || 'Qua Zoom';
                document.getElementById('cfgBox3Top').value = b3.top || '2 Giờ';
                document.getElementById('cfgBox3Bottom').value = b3.bottom || 'Thực chiến';

                document.getElementById('cfgEventTitle').value = kh.ten || '';
                document.getElementById('cfgTargetDate').value = tg.target_date || '';
                document.getElementById('cfgEventQuota').value = kh.suat_hoc || '';

                document.getElementById('cfgMetaTitle').value = s.title || '';
                document.getElementById('cfgMetaDesc').value = s.meta_description || '';
                document.getElementById('cfgOgImage').value = s.og_image || '';

                // Cấu hình Form Mapping & Heuristics
                const fm = s.form_mapping || {};
                document.getElementById('cfgFormSelector').value = fm.form_selector || '';
                const fmOpts = fm.options || {};
                document.getElementById('cfgOptValidatePhone').checked = fmOpts.validate_phone !== false;
                document.getElementById('cfgOptHoneypot').checked = fmOpts.honeypot !== false;
                document.getElementById('cfgOptUtmCapture').checked = fmOpts.utm_capture !== false;
                document.getElementById('cfgOptCookie180d').checked = fmOpts.cookie_180d !== false;

                if (fm.fields && Array.isArray(fm.fields) && fm.fields.length > 0) {
                    renderScannedInputsTable(fm.fields);
                    document.getElementById('scanFormStatus').innerHTML = `<span class="text-emerald-400 font-bold"><i class="fa-solid fa-circle-check mr-1"></i> Đã cấu hình ${fm.fields.length} trường</span>`;
                } else {
                    renderScannedInputsTable([]);
                    document.getElementById('scanFormStatus').innerHTML = `<span>Chưa thực hiện quét</span><span class="text-[10px] bg-slate-800 px-2 py-0.5 rounded text-slate-400">Sẵn sàng</span>`;
                }

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

            const box1Top = document.getElementById('cfgBox1Top').value.trim() || '03/11/2026';
            const box1Btm = document.getElementById('cfgBox1Bottom').value.trim() || '20:00 Online';
            const box2Top = document.getElementById('cfgBox2Top').value.trim() || 'Học Online';
            const box2Btm = document.getElementById('cfgBox2Bottom').value.trim() || 'Qua Zoom';
            const box3Top = document.getElementById('cfgBox3Top').value.trim() || '2 Giờ';
            const box3Btm = document.getElementById('cfgBox3Bottom').value.trim() || 'Thực chiến';

            const settings = {
                title: document.getElementById('cfgMetaTitle').value.trim(),
                meta_description: document.getElementById('cfgMetaDesc').value.trim(),
                og_image: document.getElementById('cfgOgImage').value.trim(),
                hotline: document.getElementById('cfgHotline').value.trim(),
                zalo_link: document.getElementById('cfgZaloLink').value.trim(),
                facebook_pixel_id: document.getElementById('cfgFacebookPixel').value.trim(),
                google_analytics_id: document.getElementById('cfgGoogleAnalytics').value.trim(),
                tiktok_pixel_id: document.getElementById('cfgTikTokPixel').value.trim(),
                custom_head_scripts: document.getElementById('cfgCustomHead').value.trim(),
                custom_body_scripts: document.getElementById('cfgCustomBody').value.trim(),
                google_sheet_sync_mode: syncMode,
                google_sheet_ids: selectedSheetIds,
                brand_header: {
                    co_quan_1: document.getElementById('cfgBrandHeader1').value.trim() || 'Bộ Công Thương',
                    co_quan_2: document.getElementById('cfgBrandHeader2').value.trim() || 'Sở giao dịch hàng hóa Việt Nam MXV',
                    agency_1: document.getElementById('cfgBrandHeader1').value.trim() || 'Bộ Công Thương',
                    agency_2: document.getElementById('cfgBrandHeader2').value.trim() || 'Sở giao dịch hàng hóa Việt Nam MXV',
                    logo_url: document.getElementById('cfgBrandLogo').value.trim() || 'assets/img/logo_APEX_K_NEN.webp',
                    member_text: document.getElementById('cfgBrandMemberText').value.trim() || 'Thành viên 080 MXV',
                },
                tieu_de: {
                    badge: document.getElementById('cfgBadgeText').value.trim() || 'LỚP HỌC ONLINE MIỄN PHÍ',
                    line_1: document.getElementById('cfgTitleLine1').value.trim() || 'GIẢI MÃ ĐẦU TƯ',
                    line_2: document.getElementById('cfgTitleLine2').value.trim() || 'HÀNG HÓA',
                    mo_ta: document.getElementById('cfgSubtitle').value.trim() || 'Hiểu thị trường – Nắm cơ hội – Quản trị rủi ro',
                },
                metric_boxes: {
                    box_1: { top: box1Top, bottom: box1Btm },
                    box_2: { top: box2Top, bottom: box2Btm },
                    box_3: { top: box3Top, bottom: box3Btm },
                },
                thoi_gian_chia_se: {
                    ngay: box1Top,
                    gio: box1Btm,
                    hien_thi: `${box1Btm} - ${box1Top}`,
                    hinh_thuc: `${box2Top} - ${box2Btm}`,
                    target_date: document.getElementById('cfgTargetDate').value.trim(),
                },
                khoa_hoc: {
                    ten: document.getElementById('cfgEventTitle').value.trim(),
                    suat_hoc: document.getElementById('cfgEventQuota').value.trim(),
                },
                form_config: {
                    require_phone: true,
                    require_name: true,
                    success_redirect: document.getElementById('cfgSuccessRedirect').value.trim(),
                },
                form_mapping: (function() {
                    const formSelector = document.getElementById('cfgFormSelector').value.trim();
                    const fieldRows = document.querySelectorAll('#tbodyScannedInputs tr[data-field-row]');
                    const mappingFields = [];
                    fieldRows.forEach(row => {
                        const sel = row.querySelector('.map-selector')?.value.trim() || '';
                        const leadField = row.querySelector('.map-lead-field')?.value || 'ignore';
                        const tag = row.getAttribute('data-tag') || 'input';
                        const type = row.getAttribute('data-type') || 'text';
                        const req = row.querySelector('.map-required')?.checked || false;
                        const name = row.getAttribute('data-name') || '';
                        const id = row.getAttribute('data-id') || '';
                        const ph = row.getAttribute('data-ph') || '';
                        if (sel && leadField !== 'ignore') {
                            mappingFields.push({
                                selector: sel,
                                lead_field: leadField,
                                tag: tag,
                                type: type,
                                name: name,
                                id: id,
                                placeholder: ph,
                                required: req
                            });
                        }
                    });
                    return {
                        form_selector: formSelector,
                        fields: mappingFields,
                        options: {
                            validate_phone: document.getElementById('cfgOptValidatePhone').checked,
                            honeypot: document.getElementById('cfgOptHoneypot').checked,
                            utm_capture: document.getElementById('cfgOptUtmCapture').checked,
                            cookie_180d: document.getElementById('cfgOptCookie180d').checked,
                        }
                    };
                })()
            };

            const payload = {
                page_id: pageId,
                name: pageName || pageId,
                domain: document.getElementById('cfgDomain').value.trim(),
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



        function openLandingPreview(pageId) {
            currentModalPreviewPageId = pageId;
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
            iframe.src = `/api/landings/preview?page_id=${encodeURIComponent(currentModalPreviewPageId)}&bypass=1&preview=1&v=${Date.now()}`;
        }

        function openModalPreviewNewTab() {
            if (!currentModalPreviewPageId) return;
            window.open(`/api/landings/preview?page_id=${encodeURIComponent(currentModalPreviewPageId)}&bypass=1&preview=1`, '_blank');
        }

        // Điều hướng sang Visual Web Builder (Studio Workspace độc lập)
        function openCodeStudio(domain, pageId) {
            const pId = pageId || 'mxv_080_online';
            const dom = domain || '';
            const studioUrl = `studio.html?page_id=${encodeURIComponent(pId)}&domain=${encodeURIComponent(dom)}`;
            window.location.href = studioUrl;
        }

