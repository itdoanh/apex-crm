/* ==============================================================
   APEX ADMIN - LEADS CRM (TABLE, KANBAN, CUSTOMER 360 DRAWER)
   ============================================================== */

let currentCrmView = 'table'; // 'table' or 'kanban'
let crmPage = 1;
let crmLimit = 50;
let crmTotal = 0;
let curLeads = [];
let currentOpenLead = null;

        const STATUS_INFO = {
            'moi':          { label: 'Mới Tiếp Nhận',   color: 'sky',     badge: 'bg-sky-950 text-sky-400 border-sky-800' },
            'da_goi':       { label: 'Đã Gọi Điện',     color: 'amber',   badge: 'bg-amber-950 text-amber-400 border-amber-800' },
            'ket_ban_zalo': { label: 'Đã Kết Bạn Zalo', color: 'blue',    badge: 'bg-blue-950 text-blue-400 border-blue-800' },
            'da_vao_zoom':  { label: 'Đã Vào Zoom',     color: 'purple',  badge: 'bg-purple-950 text-purple-400 border-purple-800' },
            'tiem_nang':    { label: 'Khách Tiềm Năng', color: 'orange',  badge: 'bg-orange-950 text-orange-400 border-orange-800' },
            'chot':         { label: 'Đã Chốt HĐ',      color: 'emerald', badge: 'bg-emerald-950 text-emerald-400 border-emerald-800' },
            'huy':          { label: 'Hủy / Sai Số',    color: 'rose',    badge: 'bg-rose-950 text-rose-400 border-rose-800' }
        };

        function getRegionBadgeClass(region) {
            const r = (region || '').toLowerCase();
            if (r.includes('bắc')) return 'bg-sky-950 text-sky-400 border border-sky-800';
            if (r.includes('trung')) return 'bg-amber-950 text-amber-400 border border-amber-800';
            if (r.includes('nam')) return 'bg-emerald-950 text-emerald-400 border border-emerald-800';
            if (r.includes('tế')) return 'bg-purple-950 text-purple-400 border border-purple-800';
            return 'bg-slate-800 text-slate-300 border border-slate-700';
        }

        function getAdPlatformBadgeClass(platform) {
            const p = (platform || '').toLowerCase();
            if (p.includes('meta') || p.includes('facebook') || p.includes('instagram')) return 'bg-blue-950 text-blue-300 border border-blue-800';
            if (p.includes('google')) return 'bg-emerald-950 text-emerald-300 border border-emerald-800';
            if (p.includes('tiktok')) return 'bg-teal-950 text-teal-300 border border-teal-800';
            if (p.includes('zalo')) return 'bg-indigo-950 text-indigo-300 border border-indigo-800';
            return 'bg-slate-800 text-slate-400 border border-slate-700';
        }

        function getAdPlacementBadgeClass(platform, placement) {
            const pl = (placement || '').toLowerCase();
            const pf = (platform || '').toLowerCase();
            if (pl.includes('instagram') || pl.includes('stories') || pl.includes('reels') || pl.includes('explore')) {
                return 'bg-gradient-to-r from-pink-950 to-purple-950 text-pink-300 border border-pink-700/60';
            }
            if (pl.includes('audience') || pl.includes('an')) {
                return 'bg-indigo-950 text-indigo-300 border border-indigo-800';
            }
            if (pl.includes('messenger') || pl.includes('msg')) {
                return 'bg-cyan-950 text-cyan-300 border border-cyan-800';
            }
            if (pl.includes('facebook') || pl.includes('feed') || pf.includes('meta')) {
                return 'bg-blue-950 text-blue-300 border border-blue-800';
            }
            if (pl.includes('search')) {
                return 'bg-emerald-950 text-emerald-300 border border-emerald-800';
            }
            if (pl.includes('display') || pl.includes('gdn')) {
                return 'bg-sky-950 text-sky-300 border border-sky-800';
            }
            if (pl.includes('youtube') || pl.includes('video')) {
                return 'bg-rose-950 text-rose-300 border border-rose-800';
            }
            if (pl.includes('max') || pl.includes('pmax')) {
                return 'bg-amber-950 text-amber-300 border border-amber-800';
            }
            if (pl.includes('pangle')) {
                return 'bg-teal-950 text-teal-300 border border-teal-800';
            }
            if (pl.includes('tiktok')) {
                return 'bg-teal-950 text-teal-400 border border-teal-700';
            }
            if (pl.includes('zalo') || pl.includes('báo mới') || pl.includes('zing')) {
                return 'bg-blue-950 text-blue-400 border border-blue-700';
            }
            return 'bg-slate-800 text-slate-300 border border-slate-700';
        }



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



        // 6. CRM Stats & Charts Loader
        async function loadCRMStats() {
            try {
                const p = buildFilterParams();
                const res = await fetch('/api/crm/stats?' + p.toString());
                if (!res.ok) return;
                const json = await res.json();
                if (!json || !json.success || !json.data) return;

                const d = json.data;
                const s = d.summary || {};

                // Update KPI Cards
                document.getElementById('kpiCrmTotal').innerText = Number(s.total || 0).toLocaleString();
                document.getElementById('kpiCrmToday').innerText = Number(s.today || 0).toLocaleString();
                document.getElementById('kpiCrmYesterdaySub').innerText = `Hôm qua: ${Number(s.yesterday || 0).toLocaleString()}`;
                document.getElementById('kpiCrmPeakHour').innerText = s.peak_hour || '--:--';
                document.getElementById('kpiCrmPeakCount').innerText = `${s.peak_hour_count || 0} leads đỉnh điểm`;
                document.getElementById('kpiCrmClosed').innerText = Number(s.closed || 0).toLocaleString();
                const winRate = s.total > 0 ? Math.round(((s.closed || 0) / s.total) * 100) : 0;
                document.getElementById('kpiCrmWinRate').innerText = `${winRate}% tỷ lệ chốt`;

                // Autofill ratio
                const af = d.autofill_ratio || {};
                const afTotal = (af.autofill || 0) + (af.manual || 0);
                const afPercent = afTotal > 0 ? Math.round(((af.autofill || 0) / afTotal) * 100) : 0;
                document.getElementById('kpiCrmAutofillRate').innerText = `${afPercent}%`;

                // Badge total
                document.getElementById('badgeCrmTotal').innerText = s.total || 0;

                // Peak Hour Badge
                const peakBadge = document.getElementById('chartPeakHourBadge');
                if (peakBadge) peakBadge.innerText = `Giờ Vàng: ${s.peak_hour || '--:--'}`;

                // Populate Dropdowns if not populated
                populateOptions(d.landing_ids || [], d.sources || []);

                // Render Charts if analytics module is loaded
                if (typeof renderHourlyChart === 'function') {
                    renderHourlyChart(d.hourly_distribution || [], s.peak_hour);
                    renderDailyTrendChart(d.daily_trend || []);
                    renderStatusChart(d.status_counts || {});
                    renderSourceChart(d.source_counts || []);
                    renderCarrierChart(d.carrier_counts || {});
                    renderAutofillChart(d.autofill_ratio || {});
                    renderFunnelChart(s.total || 0, s.high_intent || 0, s.closed || 0, d.funnel || null);
                }

                if (currentMainTab === 'analytics') {
                    loadGeoAnalyticsCharts();
                    loadAdPlacementAnalyticsCharts();
                }

            } catch (err) {
                console.error("Lỗi nạp CRM Stats:", err);
            }
        }

        function populateOptions(landingIDs, sources) {
            const selLanding = document.getElementById('filterLanding');
            const currentL = selLanding.value;
            if (selLanding.options.length <= 1) {
                landingIDs.forEach(lid => {
                    const opt = document.createElement('option');
                    opt.value = lid;
                    opt.innerText = lid;
                    selLanding.appendChild(opt);
                });
                selLanding.value = currentL;
            }

            // Đồng bộ dropdown báo cáo riêng từng landing (2026-11-12)
            if (typeof populateReportLandingOptions === 'function') {
                populateReportLandingOptions(landingIDs);
            }

            const selNguon = document.getElementById('filterNguon');
            const currentN = selNguon.value;
            if (selNguon.options.length <= 1) {
                sources.forEach(src => {
                    const opt = document.createElement('option');
                    opt.value = src;
                    opt.innerText = src;
                    selNguon.appendChild(opt);
                });
                selNguon.value = currentN;
            }
        }

        // 7. Load CRM Leads (Table & Kanban)
        async function loadCRMLeads() {
            const tbody = document.getElementById('crmTableBody');
            try {
                const p = buildFilterParams();
                p.set('page', crmPage);
                p.set('limit', crmLimit);

                const res = await fetch('/api/leads?' + p.toString());
                if (!res.ok) throw new Error('HTTP ' + res.status);
                const json = await res.json();

                curLeads = json.data || [];
                crmTotal = json.total || 0;

                document.getElementById('crmTotalRows').innerText = crmTotal;
                document.getElementById('crmViewCount').innerText = curLeads.length;
                document.getElementById('crmCurPage').innerText = crmPage;
                const totalPages = Math.max(1, Math.ceil(crmTotal / crmLimit));
                document.getElementById('crmTotalPages').innerText = totalPages;

                document.getElementById('crmPrevBtn').disabled = (crmPage <= 1);
                document.getElementById('crmNextBtn').disabled = (crmPage >= totalPages);

                renderDataTable(curLeads);
                if (currentCrmView === 'kanban') {
                    renderKanbanBoard(curLeads);
                }

            } catch (err) {
                tbody.innerHTML = `
                    <tr>
                        <td colspan="9" class="py-8 text-center text-rose-400">
                            <i class="fa-solid fa-triangle-exclamation mr-1.5"></i>Lỗi tải dữ liệu leads: ${escapeHTML(err.message)}
                        </td>
                    </tr>
                `;
            }
        }

        // 8. Render Data Table
        function renderDataTable(leads) {
            const tbody = document.getElementById('crmTableBody');
            if (!leads || leads.length === 0) {
                tbody.innerHTML = `
                    <tr>
                        <td colspan="10" class="py-12 text-center text-slate-500">
                            <i class="fa-solid fa-folder-open text-3xl mb-2 text-slate-600"></i>
                            <div>Không có dữ liệu lead nào khớp với bộ lọc hiện tại.</div>
                        </td>
                    </tr>
                `;
                return;
            }

            tbody.innerHTML = leads.map(lead => {
                let tr = {};
                try {
                    tr = typeof lead.tracking === 'string' ? JSON.parse(lead.tracking) : (lead.tracking || {});
                } catch (_) {}

                const fullName = tr.fullName || tr.name || tr.ho_ten || 'Khách vãng lai';
                const carrier = tr.carrier || 'Khác';
                const isAutofill = tr.is_autofill === true || tr.is_autofill === '1' || !!tr.autofill_source;
                const dev = tr.device_type || 'desktop';
                const status = lead.trang_thai || 'moi';
                const stInfo = STATUS_INFO[status] || STATUS_INFO['moi'];

                const city = lead.city || tr.city || 'Chưa rõ';
                const region = lead.region || tr.region || 'Miền Bắc';
                const isp = lead.isp || tr.isp || 'Khác';
                const adPlatform = lead.ad_platform || tr.ad_platform || 'Trực Tiếp';
                const adPlacement = lead.ad_placement || tr.ad_placement || 'Trực Tiếp (Direct)';

                const cleanPhone = lead.so_dien_thoai.replace(/\s+/g, '');

                return `
                    <tr class="hover:bg-slate-800/60 transition cursor-pointer group" onclick="openLeadDrawer(${lead.id})">
                        <td class="py-3 px-3 text-center font-mono text-slate-500 text-[11px]">#${lead.id}</td>
                        <td class="py-3 px-4">
                            <div class="flex items-center space-x-2.5">
                                <div class="w-7 h-7 rounded-lg bg-slate-800 text-orange-400 border border-slate-700 flex items-center justify-center font-bold text-xs uppercase flex-shrink-0">
                                    ${escapeHTML(fullName.charAt(0))}
                                </div>
                                <div>
                                    <div class="font-bold text-slate-100 group-hover:text-orange-400 transition">${escapeHTML(fullName)}</div>
                                    <div class="text-[10px] text-slate-500 font-mono">${escapeHTML(tr.server_ip || '-')}</div>
                                </div>
                            </div>
                        </td>
                        <td class="py-3 px-4">
                            <div class="flex items-center space-x-1.5">
                                <span class="font-mono font-bold text-slate-200 text-xs">${escapeHTML(lead.so_dien_thoai)}</span>
                                <span class="text-[10px] px-1.5 py-0.2 rounded font-mono font-bold ${getCarrierBadgeClass(carrier)}">${escapeHTML(carrier)}</span>
                            </div>
                        </td>
                        <td class="py-3 px-4 font-mono text-slate-300 text-[11px] whitespace-nowrap">
                            ${escapeHTML(lead.thoi_gian)}
                        </td>
                        <td class="py-3 px-4">
                            <span class="text-orange-400 font-mono font-semibold">${escapeHTML(lead.landing_id || 'default')}</span>
                            <div class="text-[10px] text-slate-400">${escapeHTML(lead.nguon || 'direct')}</div>
                        </td>
                        <td class="py-3 px-4">
                            <div class="flex items-center space-x-1.5">
                                <span class="font-bold text-slate-100 flex items-center gap-1">
                                    <i class="fa-solid fa-location-dot text-rose-400 text-[11px]"></i>
                                    <span>${escapeHTML(city)}</span>
                                </span>
                                <span class="text-[10px] px-1.5 py-0.2 rounded font-mono font-bold ${getRegionBadgeClass(region)}">${escapeHTML(region)}</span>
                            </div>
                            <div class="mt-1 flex items-center space-x-1.5">
                                <span class="text-[10px] px-2 py-0.5 rounded font-bold ${getAdPlacementBadgeClass(adPlatform, adPlacement)}">${escapeHTML(adPlacement)}</span>
                                <span class="text-[10px] text-slate-400 font-mono">${escapeHTML(isp)}</span>
                            </div>
                        </td>
                        <td class="py-3 px-4" onclick="event.stopPropagation()">
                            <select onchange="updateLeadStatusInline(${lead.id}, this.value)" class="bg-slate-950 border border-slate-700 text-xs rounded-lg px-2 py-1 font-semibold focus:outline-none cursor-pointer">
                                <option value="moi" ${status === 'moi' ? 'selected' : ''}>📥 Mới tiếp nhận</option>
                                <option value="da_goi" ${status === 'da_goi' ? 'selected' : ''}>📞 Đã gọi điện</option>
                                <option value="ket_ban_zalo" ${status === 'ket_ban_zalo' ? 'selected' : ''}>💬 Đã kết bạn Zalo</option>
                                <option value="da_vao_zoom" ${status === 'da_vao_zoom' ? 'selected' : ''}>💻 Đã vào Zoom</option>
                                <option value="tiem_nang" ${status === 'tiem_nang' ? 'selected' : ''}>⭐ Tiềm năng</option>
                                <option value="chot" ${status === 'chot' ? 'selected' : ''}>🏆 Đã chốt HĐ</option>
                                <option value="huy" ${status === 'huy' ? 'selected' : ''}>❌ Hủy / Sai số</option>
                            </select>
                        </td>
                        <td class="py-3 px-4 max-w-xs truncate text-[11px] text-slate-300">
                            ${escapeHTML(lead.ghi_chu || '-')}
                        </td>
                        <td class="py-3 px-4 text-[11px]">
                            ${isAutofill ? '<span class="text-blue-400 font-semibold"><i class="fa-solid fa-wand-magic-sparkles mr-1"></i>Autofill</span>' : '<span class="text-slate-500">Gõ tay</span>'}
                            <span class="text-slate-500 ml-1">(${escapeHTML(dev)})</span>
                        </td>
                        <td class="py-3 px-4 text-center" onclick="event.stopPropagation()">
                            <div class="flex items-center justify-center space-x-1.5">
                                <a href="tel:${cleanPhone}" class="w-7 h-7 rounded-lg bg-emerald-950 text-emerald-400 border border-emerald-800 hover:bg-emerald-900 flex items-center justify-center transition" title="Gọi điện">
                                    <i class="fa-solid fa-phone text-xs"></i>
                                </a>
                                <a href="https://zalo.me/${cleanPhone}" target="_blank" class="w-7 h-7 rounded-lg bg-blue-950 text-blue-400 border border-blue-800 hover:bg-blue-900 flex items-center justify-center transition" title="Mở Zalo">
                                    <i class="fa-solid fa-comment-dots text-xs"></i>
                                </a>
                                <button onclick="openLeadDrawer(${lead.id})" class="w-7 h-7 rounded-lg bg-slate-800 text-slate-300 hover:text-white hover:bg-slate-700 flex items-center justify-center transition" title="Xem hồ sơ">
                                    <i class="fa-solid fa-chevron-right text-xs"></i>
                                </button>
                            </div>
                        </td>
                    </tr>
                `;
            }).join('');
        }

        // 9. Render Kanban Board
        function renderKanbanBoard(leads) {
            const cols = ['moi', 'da_goi', 'ket_ban_zalo', 'da_vao_zoom', 'tiem_nang', 'chot', 'huy'];
            const groups = {};
            cols.forEach(c => groups[c] = []);

            (leads || []).forEach(lead => {
                const st = lead.trang_thai || 'moi';
                if (groups[st]) {
                    groups[st].push(lead);
                } else {
                    groups['moi'].push(lead);
                }
            });

            cols.forEach(c => {
                const countBadge = document.getElementById('kanbanCount_' + c);
                const colContainer = document.getElementById('kanbanList_' + c);
                const items = groups[c] || [];

                if (countBadge) countBadge.innerText = items.length;
                if (!colContainer) return;

                if (items.length === 0) {
                    colContainer.innerHTML = '<div class="py-8 text-center text-slate-600 text-xs">Trống</div>';
                    return;
                }

                colContainer.innerHTML = items.map(lead => {
                    let tr = {};
                    try {
                        tr = typeof lead.tracking === 'string' ? JSON.parse(lead.tracking) : (lead.tracking || {});
                    } catch (_) {}
                    const fullName = tr.fullName || tr.name || tr.ho_ten || 'Khách vãng lai';
                    const carrier = tr.carrier || 'Khác';
                    const cleanPhone = lead.so_dien_thoai.replace(/\s+/g, '');
                    const city = lead.city || tr.city || 'Chưa rõ';
                    const region = lead.region || tr.region || 'Miền Bắc';
                    const adPlatform = lead.ad_platform || tr.ad_platform || 'Trực Tiếp';
                    const adPlacement = lead.ad_placement || tr.ad_placement || 'Trực Tiếp';

                    return `
                        <div class="bg-slate-950 border border-slate-800 hover:border-orange-500/60 p-3 rounded-xl shadow cursor-pointer transition space-y-2 group" onclick="openLeadDrawer(${lead.id})">
                            <div class="flex items-start justify-between gap-1">
                                <div class="font-bold text-white text-xs group-hover:text-orange-400 transition truncate">${escapeHTML(fullName)}</div>
                                <span class="text-[10px] px-1.5 py-0.2 rounded font-mono font-bold ${getCarrierBadgeClass(carrier)} flex-shrink-0">${escapeHTML(carrier)}</span>
                            </div>

                            <div class="font-mono text-xs text-orange-400 font-bold">${escapeHTML(lead.so_dien_thoai)}</div>

                            <!-- City & Detailed Ad Placement Tags -->
                            <div class="flex items-center justify-between text-[10px] pt-1 border-t border-slate-850">
                                <span class="text-slate-300 truncate max-w-[110px] flex items-center gap-1">
                                    <i class="fa-solid fa-location-dot text-rose-400 text-[10px]"></i>
                                    <span>${escapeHTML(city)}</span>
                                </span>
                                <span class="text-[9px] px-1.5 py-0.2 rounded font-bold ${getAdPlacementBadgeClass(adPlatform, adPlacement)} truncate max-w-[100px]">${escapeHTML(adPlacement)}</span>
                            </div>

                            <div class="text-[11px] text-slate-400 truncate">
                                ${escapeHTML(lead.ghi_chu || 'Chưa có ghi chú...')}
                            </div>

                            <div class="flex items-center justify-between text-[10px] text-slate-500 pt-1 border-t border-slate-850">
                                <span class="font-mono">${escapeHTML(lead.thoi_gian.slice(5, 16))}</span>
                                <div class="flex items-center space-x-1" onclick="event.stopPropagation()">
                                    <a href="tel:${cleanPhone}" class="w-6 h-6 rounded bg-emerald-950 text-emerald-400 border border-emerald-800 flex items-center justify-center" title="Gọi">
                                        <i class="fa-solid fa-phone text-[10px]"></i>
                                    </a>
                                    <a href="https://zalo.me/${cleanPhone}" target="_blank" class="w-6 h-6 rounded bg-blue-950 text-blue-400 border border-blue-800 flex items-center justify-center" title="Zalo">
                                        <i class="fa-solid fa-comment text-[10px]"></i>
                                    </a>
                                </div>
                            </div>
                        </div>
                    `;
                }).join('');
            });
        }

        function getCarrierBadgeClass(carrier) {
            const c = (carrier || '').toLowerCase();
            if (c.includes('viettel')) return 'bg-rose-950 text-rose-400 border border-rose-800';
            if (c.includes('vina')) return 'bg-blue-950 text-blue-400 border border-blue-800';
            if (c.includes('mobi')) return 'bg-amber-950 text-amber-400 border border-amber-800';
            if (c.includes('vietnam')) return 'bg-orange-950 text-orange-400 border border-orange-800';
            return 'bg-slate-800 text-slate-400 border border-slate-700';
        }

        // 10. Customer 360° Drawer Management
        function openLeadDrawer(leadId) {
            const lead = curLeads.find(l => l.id === leadId);
            if (!lead) return;
            currentOpenLead = lead;

            let tr = {};
            try {
                tr = typeof lead.tracking === 'string' ? JSON.parse(lead.tracking) : (lead.tracking || {});
            } catch (_) {}

            const fullName = tr.fullName || tr.name || tr.ho_ten || 'Khách vãng lai';
            const carrier = tr.carrier || 'Khác';
            const cleanPhone = lead.so_dien_thoai.replace(/\s+/g, '');

            document.getElementById('drawerAvatar').innerText = fullName.charAt(0).toUpperCase();
            document.getElementById('drawerName').innerText = fullName;
            document.getElementById('drawerCarrierBadge').innerText = carrier;
            document.getElementById('drawerCarrierBadge').className = `text-[10px] px-2 py-0.5 rounded-full font-mono font-bold ${getCarrierBadgeClass(carrier)}`;
            document.getElementById('drawerPhone').innerText = lead.so_dien_thoai;
            document.getElementById('drawerCreatedTime').innerText = lead.thoi_gian;

            document.getElementById('drawerCallBtn').href = 'tel:' + cleanPhone;
            document.getElementById('drawerZaloBtn').href = 'https://zalo.me/' + cleanPhone;

            document.getElementById('drawerStatusSelect').value = lead.trang_thai || 'moi';
            document.getElementById('drawerNoteText').value = lead.ghi_chu || '';
            document.getElementById('drawerSaveMsg').classList.add('hidden');

            const isAutofill = tr.is_autofill === true || tr.is_autofill === '1' || !!tr.autofill_source;
            document.getElementById('drawerAutofill').innerText = isAutofill ? `Tự động điền (${tr.autofill_source || 'Browser 1-Tap'})` : 'Gõ tay thông thường';
            document.getElementById('drawerFillDuration').innerText = tr.fill_duration_ms ? `${tr.fill_duration_ms} ms` : 'Không xác định';
            document.getElementById('drawerTimeOnPage').innerText = tr.time_on_page ? `${tr.time_on_page} giây` : 'Vừa vào';
            document.getElementById('drawerMaxScroll').innerText = tr.max_scroll ? `${tr.max_scroll}%` : '0%';
            document.getElementById('drawerDevice').innerText = `${tr.device_type || 'desktop'} • ${tr.screen_res || ''}`;
            document.getElementById('drawerIP').innerText = tr.server_ip || '-';

            // Geographic & ISP Real Inspector
            const city = lead.city || tr.city || 'Khác / Chưa rõ';
            const region = lead.region || tr.region || 'Miền Bắc';
            const country = lead.country || tr.country || 'Việt Nam 🇻🇳';
            const isp = lead.isp || tr.isp || 'Khác';
            const adPlatform = lead.ad_platform || tr.ad_platform || 'Trực Tiếp';
            const adPlacement = lead.ad_placement || tr.ad_placement || 'Trực Tiếp (Direct)';
            const adSourceTag = tr.ad_source_tag || tr.utm_source || 'direct';
            const entryURL = tr.entry_url || window.location.href;
            const referrer = tr.referrer || tr.server_referer || '-';

            const cityEl = document.getElementById('drawerCityText');
            if (cityEl) cityEl.innerText = city;
            const regEl = document.getElementById('drawerRegionBadge');
            if (regEl) {
                regEl.innerText = region;
                regEl.className = `text-[10px] px-2 py-0.5 rounded font-mono font-bold inline-block ${getRegionBadgeClass(region)}`;
            }
            const countryEl = document.getElementById('drawerCountry');
            if (countryEl) countryEl.innerText = country;
            const ispEl = document.getElementById('drawerISP');
            if (ispEl) ispEl.innerText = isp;

            // Ad Placement Inspector
            const platEl = document.getElementById('drawerAdPlatformBadge');
            if (platEl) {
                platEl.innerText = adPlatform;
                platEl.className = `text-xs font-bold px-2 py-0.5 rounded ${getAdPlatformBadgeClass(adPlatform)}`;
            }
            const placeEl = document.getElementById('drawerAdPlacement');
            if (placeEl) placeEl.innerText = adPlacement;
            const tagEl = document.getElementById('drawerAdSourceTag');
            if (tagEl) tagEl.innerText = adSourceTag;
            const entryEl = document.getElementById('drawerEntryURL');
            if (entryEl) entryEl.innerText = entryURL;
            const refEl = document.getElementById('drawerReferrer');
            if (refEl) refEl.innerText = referrer;

            // URL trang khách đã đăng ký (ưu tiên cột `url` trong DB, fallback tracking JSON)
            const leadURL = lead.url || tr.url || tr.entry_url || '';
            const leadURLElement = document.getElementById('drawerLeadURL');
            if (leadURLElement) {
                if (leadURL) {
                    leadURLElement.innerText = leadURL;
                    leadURLElement.href = leadURL;
                    leadURLElement.title = leadURL;
                } else {
                    leadURLElement.innerText = 'Không ghi nhận';
                    leadURLElement.removeAttribute('href');
                    leadURLElement.title = 'Không ghi nhận';
                }
            }

            document.getElementById('drawerLanding').innerText = lead.landing_id || 'default_landing';
            document.getElementById('drawerSource').innerText = lead.nguon || 'direct';
            document.getElementById('drawerUTM').innerText = tr.utm_campaign || tr.utm_source || 'Không có UTM';

            document.getElementById('drawerRawJSON').innerText = JSON.stringify(tr, null, 2);

            // Open Drawer
            const drawer = document.getElementById('leadDrawer');
            const backdrop = document.getElementById('drawerBackdrop');
            backdrop.classList.remove('hidden');
            drawer.classList.remove('translate-x-full');
            drawer.classList.add('translate-x-0');
        }

        function closeLeadDrawer() {
            const drawer = document.getElementById('leadDrawer');
            const backdrop = document.getElementById('drawerBackdrop');
            drawer.classList.remove('translate-x-0');
            drawer.classList.add('translate-x-full');
            setTimeout(() => backdrop.classList.add('hidden'), 200);
            currentOpenLead = null;
        }

        async function saveLeadFromDrawer() {
            if (!currentOpenLead) return;
            const newStatus = document.getElementById('drawerStatusSelect').value;
            const newNote = document.getElementById('drawerNoteText').value.trim();

            try {
                const res = await fetch('/api/leads/update', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        id: currentOpenLead.id,
                        trang_thai: newStatus,
                        ghi_chu: newNote
                    })
                });
                if (res.ok) {
                    currentOpenLead.trang_thai = newStatus;
                    currentOpenLead.ghi_chu = newNote;
                    document.getElementById('drawerSaveMsg').classList.remove('hidden');
                    renderDataTable(curLeads);
                    if (currentCrmView === 'kanban') renderKanbanBoard(curLeads);
                    loadCRMStats();
                    setTimeout(() => document.getElementById('drawerSaveMsg').classList.add('hidden'), 2500);
                }
            } catch (err) {
                alert('Lỗi cập nhật CRM: ' + err.message);
            }
        }

        async function updateLeadStatusInline(leadId, newStatus) {
            try {
                await fetch('/api/leads/update', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        id: leadId,
                        trang_thai: newStatus
                    })
                });
                const lead = curLeads.find(l => l.id === leadId);
                if (lead) lead.trang_thai = newStatus;
                if (currentCrmView === 'kanban') renderKanbanBoard(curLeads);
                loadCRMStats();
            } catch (err) {
                console.error("Lỗi đổi trạng thái:", err);
            }
        }

        function copyDrawerJSON() {
            if (!currentOpenLead) return;
            navigator.clipboard.writeText(JSON.stringify(currentOpenLead.tracking, null, 2))
                .then(() => alert("Đã copy toàn bộ JSON Tracking vào clipboard!"))
                .catch(() => {});
        }

        // 11. Pagination Handlers
        function prevCrmPage() {
            if (crmPage > 1) {
                crmPage--;
                loadCRMLeads();
            }
        }

        function nextCrmPage() {
            const totalPages = Math.max(1, Math.ceil(crmTotal / crmLimit));
            if (crmPage < totalPages) {
                crmPage++;
                loadCRMLeads();
            }
        }

        function changeCrmLimit() {
            crmLimit = parseInt(document.getElementById('crmLimitSelect').value, 10);
            crmPage = 1;
            loadCRMLeads();
        }



        // 15. Excel/CSV Export
        function exportCurrentCSV() {
            const p = buildFilterParams();
            window.location.href = '/api/leads/export.csv?' + p.toString();
        }

