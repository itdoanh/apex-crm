/* ==============================================================
   APEX ADMIN - DOMAIN MANAGEMENT, DNS & MULTI-DOMAIN ROUTING
   ============================================================== */

let allDomains = [];

        // ==============================================================

        // 17.1. Tải danh sách tên miền từ Backend API
        async function loadDomains() {
            try {
                const res = await fetch('/api/domains');
                if (!res.ok) throw new Error('Không thể tải danh sách tên miền');
                const json = await res.json();
                if (!json || !json.success) throw new Error(json.error || 'Lỗi dữ liệu');

                allDomains = json.data || [];

                // Cập nhật thống kê KPI
                let activeCount = 0;
                let pendingCount = 0;
                let totalRoutes = 0;

                allDomains.forEach(d => {
                    if (d.dns_status === 'active') activeCount++;
                    else pendingCount++;
                    totalRoutes += (d.routes ? d.routes.length : 0);
                });

                const kpiTotal = document.getElementById('kpiDomainsTotal');
                const kpiActive = document.getElementById('kpiDomainsActive');
                const kpiPending = document.getElementById('kpiDomainsPending');
                const kpiRoutes = document.getElementById('kpiDomainsRoutes');
                const badgeActive = document.getElementById('badgeDomainsActive');

                if (kpiTotal) kpiTotal.innerText = allDomains.length;
                if (kpiActive) kpiActive.innerText = activeCount;
                if (kpiPending) kpiPending.innerText = pendingCount;
                if (kpiRoutes) kpiRoutes.innerText = totalRoutes;
                if (badgeActive) badgeActive.innerText = activeCount;

                // Đồng bộ dropdown chọn domain trên các modal
                populateDomainDropdowns();

                // Render danh sách cards
                renderDomainsList();
            } catch (err) {
                console.error('Lỗi loadDomains:', err);
            }
        }

        // 17.2. Đồng bộ các dropdown chọn domain
        function populateDomainDropdowns() {
            // Dropdown trong cài đặt landing page (Ladi Studio)
            const cfgSelect = document.getElementById('cfgDomainSelect');
            if (cfgSelect) {
                const curVal = cfgSelect.value;
                cfgSelect.innerHTML = '<option value="">-- Dùng chung domain mặc định --</option>';
                allDomains.forEach(d => {
                    const opt = document.createElement('option');
                    opt.value = d.domain;
                    opt.textContent = `${d.domain} (${d.dns_status === 'active' ? 'DNS OK' : 'Chờ DNS'})`;
                    cfgSelect.appendChild(opt);
                });
                if (curVal) cfgSelect.value = curVal;
            }

            // Dropdown trong Modal Deploy Route
            const deploySelect = document.getElementById('deployRouteDomainSelect');
            if (deploySelect) {
                const curVal = deploySelect.value;
                deploySelect.innerHTML = '<option value="">-- Chọn tên miền đã trỏ về --</option>';
                allDomains.forEach(d => {
                    const opt = document.createElement('option');
                    opt.value = d.id;
                    opt.textContent = `${d.domain} [${d.dns_status.toUpperCase()}]`;
                    opt.setAttribute('data-domain', d.domain);
                    deploySelect.appendChild(opt);
                });
                if (curVal) deploySelect.value = curVal;
            }

            // Dropdown Landing Page trong Modal Thêm Domain
            const addDomainLanding = document.getElementById('addDomainDefaultLanding');
            if (addDomainLanding && allLandingsList) {
                const curVal = addDomainLanding.value;
                addDomainLanding.innerHTML = '';
                allLandingsList.forEach(p => {
                    const opt = document.createElement('option');
                    opt.value = p.page_id;
                    opt.textContent = `${p.name} (${p.page_id})`;
                    addDomainLanding.appendChild(opt);
                });
                if (curVal) addDomainLanding.value = curVal;
            }

            // Dropdown Landing Page trong Modal Deploy Route
            const deployLanding = document.getElementById('deployRouteLandingSelect');
            if (deployLanding && allLandingsList) {
                const curVal = deployLanding.value;
                deployLanding.innerHTML = '';
                allLandingsList.forEach(p => {
                    const opt = document.createElement('option');
                    opt.value = p.page_id;
                    opt.textContent = `${p.name} (${p.page_id})`;
                    deployLanding.appendChild(opt);
                });
                if (curVal) deployLanding.value = curVal;
            }
        }

        // 17.3. Render danh sách Domain Cards
        function renderDomainsList() {
            const container = document.getElementById('domainsListContainer');
            if (!container) return;

            const query = (document.getElementById('filterDomainSearch')?.value || '').toLowerCase().trim();
            const statusFilter = document.getElementById('filterDomainStatus')?.value || 'all';

            const filtered = allDomains.filter(d => {
                const matchQuery = !query || d.domain.toLowerCase().includes(query) || (d.notes && d.notes.toLowerCase().includes(query));
                const matchStatus = statusFilter === 'all' || d.dns_status === statusFilter;
                return matchQuery && matchStatus;
            });

            const countEl = document.getElementById('domainsDisplayCount');
            if (countEl) countEl.innerText = filtered.length;

            if (filtered.length === 0) {
                container.innerHTML = `
                    <div class="bg-slate-900 border border-slate-800 rounded-2xl p-12 text-center text-slate-400 space-y-3">
                        <i class="fa-solid fa-globe text-4xl text-slate-600"></i>
                        <h4 class="font-bold text-white text-base">Không tìm thấy tên miền nào</h4>
                        <p class="text-xs max-w-md mx-auto">Chưa có tên miền nào phù hợp với bộ lọc hoặc chưa có tên miền nào được thêm vào hệ thống.</p>
                        <button onclick="openAddDomainModal()" class="mt-2 bg-indigo-600 hover:bg-indigo-500 text-white font-bold text-xs px-4 py-2 rounded-xl transition inline-flex items-center gap-1.5 shadow">
                            <i class="fa-solid fa-plus"></i> Thêm Tên Miền Đầu Tiên
                        </button>
                    </div>
                `;
                return;
            }

            container.innerHTML = filtered.map(d => {
                let badgeClass = 'bg-amber-950 text-amber-400 border-amber-800';
                let statusLabel = 'Chờ Trỏ DNS';
                let statusIcon = 'fa-solid fa-clock';

                if (d.dns_status === 'active') {
                    badgeClass = 'bg-emerald-950 text-emerald-400 border-emerald-800';
                    statusLabel = 'DNS Hoạt Động';
                    statusIcon = 'fa-solid fa-circle-check';
                } else if (d.dns_status === 'error') {
                    badgeClass = 'bg-rose-950 text-rose-400 border-rose-800';
                    statusLabel = 'Lỗi Trỏ DNS';
                    statusIcon = 'fa-solid fa-circle-xmark';
                }

                const routes = d.routes || [];

                return `
                    <div class="bg-slate-900 border border-slate-800 hover:border-slate-700 rounded-2xl p-5 shadow-sm transition space-y-4">
                        <!-- Card Header -->
                        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b border-slate-800 pb-3">
                            <div class="flex items-center gap-3">
                                <span class="w-10 h-10 rounded-xl bg-indigo-500/10 border border-indigo-500/20 flex items-center justify-center text-indigo-400 text-lg">
                                    <i class="fa-solid fa-globe"></i>
                                </span>
                                <div>
                                    <div class="flex items-center gap-2">
                                        <a href="http://${escapeHTML(d.domain)}" target="_blank" class="text-base font-black text-white hover:text-indigo-400 transition flex items-center gap-1.5">
                                            <span>${escapeHTML(d.domain)}</span>
                                            <i class="fa-solid fa-arrow-up-right-from-square text-xs text-slate-500"></i>
                                        </a>
                                        <span class="${badgeClass} border text-[10px] px-2 py-0.5 rounded-full font-bold flex items-center gap-1">
                                            <i class="${statusIcon}"></i>
                                            <span>${statusLabel}</span>
                                        </span>
                                    </div>
                                    <div class="text-[11px] text-slate-400 flex flex-wrap items-center gap-x-3 gap-y-1 mt-0.5">
                                        <span>Target IP: <code class="text-amber-400 font-mono">${escapeHTML(d.target_ip || '103.173.226.15')}</code></span>
                                        <span>NS1: <code class="text-indigo-300 font-mono">${escapeHTML(d.ns1 || 'ns1.hanghoaphaisinh.net')}</code></span>
                                        <span>NS2: <code class="text-indigo-300 font-mono">${escapeHTML(d.ns2 || 'ns2.hanghoaphaisinh.net')}</code></span>
                                        ${d.notes ? `<span class="text-slate-500 italic"><i class="fa-regular fa-note-sticky mr-1"></i>${escapeHTML(d.notes)}</span>` : ''}
                                    </div>
                                </div>
                            </div>
                            <div class="flex flex-wrap items-center gap-2">
                                <button onclick="checkDomainDNS(${d.id}, '${escapeHTML(d.domain)}')" id="btnCheckDNS_${d.id}" class="bg-indigo-950/80 hover:bg-indigo-900 text-indigo-300 border border-indigo-800 text-xs font-semibold px-3 py-1.5 rounded-xl transition flex items-center gap-1.5 shadow-sm" title="Kiểm tra DNS trực tiếp qua mạng Internet">
                                    <i class="fa-solid fa-network-wired"></i>
                                    <span>Kiểm Tra DNS</span>
                                </button>
                                <button onclick="openDeployRouteModal(${d.id}, '${escapeHTML(d.domain)}')" class="bg-cyan-600 hover:bg-cyan-500 text-white text-xs font-bold px-3 py-1.5 rounded-xl transition flex items-center gap-1.5 shadow-sm" title="Triển khai landing lên domain này">
                                    <i class="fa-solid fa-plus"></i>
                                    <span>Triển Khai Landing</span>
                                </button>
                                <button onclick="deleteDomain(${d.id}, '${escapeHTML(d.domain)}')" class="bg-slate-800 hover:bg-rose-900 text-slate-400 hover:text-rose-200 text-xs p-2 rounded-xl border border-slate-700 transition" title="Xóa tên miền này">
                                    <i class="fa-regular fa-trash-can"></i>
                                </button>
                            </div>
                        </div>

                        <!-- Card Body: Routes List -->
                        <div class="space-y-2">
                            <div class="flex items-center justify-between text-xs">
                                <span class="font-bold text-slate-300 flex items-center gap-1.5">
                                    <i class="fa-solid fa-route text-cyan-400"></i>
                                    <span>Các Đường Dẫn &amp; Thư Mục Đang Phục Vụ (${routes.length})</span>
                                </span>
                                <span class="text-[11px] text-slate-500">Longest Prefix Dynamic Routing</span>
                            </div>

                            ${routes.length === 0 ? `
                                <div class="bg-slate-950/60 p-3.5 rounded-xl border border-dashed border-slate-800 text-center text-xs text-slate-400 flex items-center justify-between">
                                    <span>Chưa gán đường dẫn nào. Landing Page mặc định cho Root là: <code class="text-cyan-400">${escapeHTML(d.default_landing_id || 'mxv_080_online')}</code></span>
                                    <button onclick="openDeployRouteModal(${d.id}, '${escapeHTML(d.domain)}')" class="text-indigo-400 hover:underline font-bold text-xs flex items-center gap-1">
                                        <i class="fa-solid fa-plus"></i> Deploy Ngay
                                    </button>
                                </div>
                            ` : `
                                <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-2.5">
                                    ${routes.map(r => `
                                        <div class="bg-slate-950 p-3 rounded-xl border border-slate-800 flex items-center justify-between text-xs hover:border-slate-700 transition">
                                            <div class="truncate mr-2">
                                                <div class="flex items-center gap-1.5 font-mono text-cyan-400 font-bold">
                                                    <span>${escapeHTML(r.path_prefix)}</span>
                                                    <span class="text-[9px] bg-slate-800 text-slate-400 border border-slate-700 px-1.5 py-0.2 rounded font-sans font-normal">Active</span>
                                                </div>
                                                <div class="text-[11px] text-slate-400 truncate">
                                                    Landing: <code class="text-slate-300">${escapeHTML(r.landing_id)}</code>
                                                </div>
                                            </div>
                                            <div class="flex items-center space-x-1.5 flex-shrink-0">
                                                <a href="http://${escapeHTML(d.domain)}${r.path_prefix === '/' ? '' : r.path_prefix}" target="_blank" class="text-slate-400 hover:text-white p-1 rounded hover:bg-slate-800 transition" title="Mở trang xem thử">
                                                    <i class="fa-solid fa-arrow-up-right-from-square text-xs"></i>
                                                </a>
                                                <button onclick="deleteRoute(${r.id})" class="text-slate-500 hover:text-rose-400 p-1 rounded hover:bg-slate-800 transition" title="Gỡ bỏ route này">
                                                    <i class="fa-solid fa-trash-can text-xs"></i>
                                                </button>
                                            </div>
                                        </div>
                                    `).join('')}
                                </div>
                            `}
                        </div>
                    </div>
                `;
            }).join('');
        }

        // 17.4. Kiểm tra DNS của 1 tên miền qua Go Net Resolver
        async function checkDomainDNS(domainId, domainName) {
            const btn = document.getElementById('btnCheckDNS_' + domainId);
            const origHTML = btn ? btn.innerHTML : '';
            if (btn) {
                btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i><span>Đang tra cứu DNS...</span>';
                btn.disabled = true;
            }

            try {
                const res = await fetch('/api/domains/check-dns', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ domain_id: domainId, domain: domainName })
                });

                if (!res.ok) throw new Error('Không thể kiểm tra DNS');
                const json = await res.json();
                if (!json || !json.success) throw new Error(json.error || 'Kiểm tra thất bại');

                const data = json.data;
                showDNSResultModal(data);
                // Nạp lại danh sách để cập nhật badge
                loadDomains();
            } catch (err) {
                alert('Lỗi kiểm tra DNS: ' + err.message);
            } finally {
                if (btn) {
                    btn.innerHTML = origHTML;
                    btn.disabled = false;
                }
            }
        }

        // 17.5. Kiểm tra toàn bộ DNS
        async function checkAllDomainsDNS() {
            if (allDomains.length === 0) {
                alert('Chưa có tên miền nào để kiểm tra.');
                return;
            }

            let checked = 0;
            for (const d of allDomains) {
                try {
                    await fetch('/api/domains/check-dns', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ domain_id: d.id, domain: d.domain })
                    });
                    checked++;
                } catch (_) {}
            }

            await loadDomains();
            alert(`Đã hoàn thành kiểm tra DNS thời gian thực cho ${checked}/${allDomains.length} tên miền!`);
        }

        // 17.6. Hiển thị Modal Kết Quả Kiểm Tra DNS
        function showDNSResultModal(data) {
            document.getElementById('dnsResultDomainText').innerText = data.domain || '--';
            document.getElementById('dnsCheckedAt').innerText = data.checked_at || new Date().toLocaleString('vi-VN');

            const banner = document.getElementById('dnsResultBanner');
            const iconEl = document.getElementById('dnsResultStatusIcon');
            const titleEl = document.getElementById('dnsResultStatusTitle');
            const msgEl = document.getElementById('dnsResultMessage');

            if (data.status === 'active') {
                banner.className = 'p-3.5 rounded-xl border flex items-center gap-3 bg-emerald-950/40 border-emerald-800/80';
                iconEl.className = 'text-2xl text-emerald-400';
                iconEl.innerHTML = '<i class="fa-solid fa-circle-check"></i>';
                titleEl.className = 'font-bold text-sm text-emerald-400';
                titleEl.innerText = 'Trỏ DNS Thành Công (Hoạt Động)';
            } else if (data.status === 'pending') {
                banner.className = 'p-3.5 rounded-xl border flex items-center gap-3 bg-amber-950/40 border-amber-800/80';
                iconEl.className = 'text-2xl text-amber-400';
                iconEl.innerHTML = '<i class="fa-solid fa-clock"></i>';
                titleEl.className = 'font-bold text-sm text-amber-400';
                titleEl.innerText = 'Đang Chờ Trỏ DNS / Chưa Khớp';
            } else {
                banner.className = 'p-3.5 rounded-xl border flex items-center gap-3 bg-rose-950/40 border-rose-800/80';
                iconEl.className = 'text-2xl text-rose-400';
                iconEl.innerHTML = '<i class="fa-solid fa-circle-xmark"></i>';
                titleEl.className = 'font-bold text-sm text-rose-400';
                titleEl.innerText = 'Không Tìm Thấy Bản Ghi DNS';
            }

            msgEl.innerText = data.message || '';

            // Nameserver details
            const expNS = document.getElementById('dnsExpectedNS');
            const fndNS = document.getElementById('dnsFoundNS');
            expNS.innerHTML = (data.expected_ns || ['ns1.hanghoaphaisinh.net', 'ns2.hanghoaphaisinh.net']).map(ns => `<div>${escapeHTML(ns)}</div>`).join('');
            if (data.found_ns && data.found_ns.length > 0) {
                fndNS.innerHTML = data.found_ns.map(ns => `<div class="${data.is_ns_matched ? 'text-emerald-400 font-bold' : 'text-slate-300'}">${escapeHTML(ns)}</div>`).join('');
            } else {
                fndNS.innerHTML = '<span class="text-rose-400 italic">Chưa tìm thấy bản ghi NS</span>';
            }

            // IP details
            document.getElementById('dnsExpectedIP').innerText = data.expected_ip || '103.173.226.15';
            const fndIP = document.getElementById('dnsFoundIP');
            if (data.found_ips && data.found_ips.length > 0) {
                fndIP.innerHTML = data.found_ips.map(ip => `<div class="${data.is_ip_matched ? 'text-emerald-400 font-bold' : 'text-slate-300'}">${escapeHTML(ip)}</div>`).join('');
            } else {
                fndIP.innerHTML = '<span class="text-rose-400 italic">Chưa tìm thấy bản ghi A</span>';
            }

            document.getElementById('modalDNSResult').classList.remove('hidden');
        }

        function closeDNSResultModal() {
            document.getElementById('modalDNSResult').classList.add('hidden');
        }

        // 17.7. Thao tác Thêm Tên Miền Mới
        function openAddDomainModal() {
            document.getElementById('addDomainInput').value = '';
            document.getElementById('addDomainNs1').value = 'ns1.hanghoaphaisinh.net';
            document.getElementById('addDomainNs2').value = 'ns2.hanghoaphaisinh.net';
            document.getElementById('addDomainTargetIp').value = '103.173.226.15';
            document.getElementById('addDomainNotes').value = '';
            populateDomainDropdowns();
            document.getElementById('modalAddDomain').classList.remove('hidden');
        }

        function closeAddDomainModal() {
            document.getElementById('modalAddDomain').classList.add('hidden');
        }

        async function saveDomainFromModal(event) {
            event.preventDefault();
            const domain = document.getElementById('addDomainInput').value.trim();
            const ns1 = document.getElementById('addDomainNs1').value.trim();
            const ns2 = document.getElementById('addDomainNs2').value.trim();
            const target_ip = document.getElementById('addDomainTargetIp').value.trim();
            const default_landing_id = document.getElementById('addDomainDefaultLanding').value;
            const notes = document.getElementById('addDomainNotes').value.trim();

            if (!domain) {
                alert('Vui lòng nhập tên miền.');
                return;
            }

            try {
                const res = await fetch('/api/domains/save', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        domain,
                        ns1,
                        ns2,
                        target_ip,
                        default_landing_id,
                        notes
                    })
                });

                if (!res.ok) throw new Error('Không thể lưu tên miền');
                const json = await res.json();
                if (!json || !json.success) throw new Error(json.error || 'Lưu thất bại');

                closeAddDomainModal();
                await loadDomains();

                // Tự động kiểm tra DNS ngay sau khi thêm
                if (json.data && json.data.id) {
                    checkDomainDNS(json.data.id, json.data.domain);
                }
            } catch (err) {
                alert('Lỗi: ' + err.message);
            }
        }

        // 17.8. Xóa tên miền
        async function deleteDomain(domainId, domainName) {
            if (!confirm(`Bạn có chắc chắn muốn xóa tên miền "${domainName}" khỏi hệ thống? Mọi đường dẫn và liên kết sẽ bị gỡ bỏ.`)) {
                return;
            }

            try {
                const res = await fetch('/api/domains/delete', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ id: domainId })
                });

                if (!res.ok) throw new Error('Không thể xóa tên miền');
                const json = await res.json();
                if (!json || !json.success) throw new Error(json.error || 'Xóa thất bại');

                await loadDomains();
            } catch (err) {
                alert('Lỗi: ' + err.message);
            }
        }

        // 17.9. Thao tác Triển Khai Route (Deploy Landing lên Domain + Path)
        function openDeployRouteModal(domainId, domainName) {
            populateDomainDropdowns();
            const select = document.getElementById('deployRouteDomainSelect');
            if (select && domainId) {
                select.value = domainId;
            }
            document.getElementById('deployRoutePathPrefix').value = '/';
            document.getElementById('deployRouteRedirectUrl').value = '';
            updatePreviewDeployUrl();
            document.getElementById('modalDeployRoute').classList.remove('hidden');
        }

        function closeDeployRouteModal() {
            document.getElementById('modalDeployRoute').classList.add('hidden');
        }

        function updatePreviewDeployUrl() {
            const select = document.getElementById('deployRouteDomainSelect');
            const pathInput = document.getElementById('deployRoutePathPrefix');
            const previewEl = document.getElementById('previewDeployUrl');
            if (!previewEl || !select || !pathInput) return;

            const selectedOpt = select.options[select.selectedIndex];
            const domain = selectedOpt ? (selectedOpt.getAttribute('data-domain') || selectedOpt.text.split(' ')[0]) : 'domain.com';
            let path = pathInput.value.trim();
            if (!path.startsWith('/')) path = '/' + path;

            previewEl.innerText = `https://${domain}${path === '/' ? '' : path}`;
        }

        async function saveDeployRouteFromModal(event) {
            event.preventDefault();
            const domainId = parseInt(document.getElementById('deployRouteDomainSelect').value, 10);
            let pathPrefix = document.getElementById('deployRoutePathPrefix').value.trim();
            const landingId = document.getElementById('deployRouteLandingSelect').value;
            const redirectUrl = document.getElementById('deployRouteRedirectUrl').value.trim();

            if (!domainId) {
                alert('Vui lòng chọn tên miền.');
                return;
            }
            if (!pathPrefix.startsWith('/')) {
                pathPrefix = '/' + pathPrefix;
            }

            try {
                const res = await fetch('/api/domains/deploy', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        domain_id: domainId,
                        path_prefix: pathPrefix,
                        landing_id: landingId,
                        redirect_url: redirectUrl
                    })
                });

                if (!res.ok) throw new Error('Không thể triển khai');
                const json = await res.json();
                if (!json || !json.success) throw new Error(json.error || 'Triển khai thất bại');

                closeDeployRouteModal();
                await loadDomains();
                alert(`🚀 Đã triển khai Landing Page [${landingId}] lên đường dẫn [${pathPrefix}] thành công!`);
            } catch (err) {
                alert('Lỗi: ' + err.message);
            }
        }

        async function deleteRoute(routeId) {
            if (!confirm('Bạn có chắc muốn gỡ bỏ đường dẫn này khỏi tên miền?')) return;

            try {
                const res = await fetch('/api/domains/routes/delete', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ route_id: routeId })
                });

                if (!res.ok) throw new Error('Không thể gỡ bỏ route');
                const json = await res.json();
                if (!json || !json.success) throw new Error(json.error || 'Gỡ bỏ thất bại');

                await loadDomains();
            } catch (err) {
                alert('Lỗi: ' + err.message);
            }
        }

        // 17.10. Tích hợp Deploy nhanh từ Modal Cài Đặt Landing Page (Ladi Studio)
        function onCfgDomainSelectChange() {
            const sel = document.getElementById('cfgDomainSelect');
            const inp = document.getElementById('cfgDomain');
            if (sel && inp && sel.value) {
                inp.value = sel.value;
            }
        }

        async function deployFromLandingSettings() {
            const pageId = document.getElementById('cfgPageId')?.value.trim();
            const domain = (document.getElementById('cfgDomain')?.value || document.getElementById('cfgDomainSelect')?.value || '').trim();
            let pathPrefix = (document.getElementById('cfgPathPrefix')?.value || '/').trim();

            if (!pageId) {
                alert('Chưa có mã Landing Page.');
                return;
            }
            if (!domain) {
                alert('Vui lòng chọn hoặc nhập tên miền để triển khai.');
                return;
            }
            if (!pathPrefix.startsWith('/')) {
                pathPrefix = '/' + pathPrefix;
            }

            try {
                const res = await fetch('/api/domains/deploy', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        domain: domain,
                        path_prefix: pathPrefix,
                        landing_id: pageId,
                        redirect_url: document.getElementById('cfgRedirectUrl')?.value || ''
                    })
                });

                if (!res.ok) throw new Error('Không thể triển khai');
                const json = await res.json();
                if (!json || !json.success) throw new Error(json.error || 'Triển khai thất bại');

                await loadDomains();
                alert(`🚀 Đã triển khai Landing Page [${pageId}] lên tên miền [${domain}${pathPrefix}] thành công!`);
            } catch (err) {
                alert('Lỗi triển khai: ' + err.message);
            }
        }

        // 17.11. Cấu hình Backend API Base URL (Cross-Domain)
        function openApiConfigModal() {
            const input = document.getElementById('inputApiEndpoint');
            const resultBox = document.getElementById('apiTestResult');
            if (resultBox) resultBox.classList.add('hidden');

            const current = (window.APEX_ADMIN_CONFIG && window.APEX_ADMIN_CONFIG.API_BASE_URL) ? window.APEX_ADMIN_CONFIG.API_BASE_URL : '';
            if (input) input.value = current;

            document.getElementById('modalApiConfig').classList.remove('hidden');
        }

        function closeApiConfigModal() {
            document.getElementById('modalApiConfig').classList.add('hidden');
        }

        function setQuickApiEndpoint(url) {
            const input = document.getElementById('inputApiEndpoint');
            if (input) input.value = url;
        }

        async function testCurrentApiEndpoint() {
            const input = document.getElementById('inputApiEndpoint');
            const resultBox = document.getElementById('apiTestResult');
            if (!input || !resultBox) return;

            const endpoint = input.value.trim().replace(/\/+$/, '');
            const testUrl = (endpoint || '') + '/health';

            resultBox.classList.remove('hidden');
            resultBox.innerHTML = '<span class="text-slate-400"><i class="fa-solid fa-spinner fa-spin mr-1"></i>Đang kiểm tra kết nối tới ' + escapeHTML(testUrl) + '...</span>';

            try {
                const res = await fetch(testUrl, { cache: 'no-store' });
                if (res.ok) {
                    const data = await res.json();
                    resultBox.innerHTML = `
                        <div class="text-emerald-400 font-bold flex items-center gap-1.5">
                            <i class="fa-solid fa-circle-check"></i>
                            <span>Kết nối thành công tới Backend API!</span>
                        </div>
                        <div class="text-[10px] text-slate-400 mt-1">Phản hồi: <code>${escapeHTML(JSON.stringify(data))}</code></div>
                    `;
                } else {
                    resultBox.innerHTML = `<span class="text-rose-400 font-semibold"><i class="fa-solid fa-circle-xmark mr-1"></i>Máy chủ phản hồi mã lỗi HTTP ${res.status}</span>`;
                }
            } catch (err) {
                resultBox.innerHTML = `<span class="text-rose-400 font-semibold"><i class="fa-solid fa-circle-xmark mr-1"></i>Không thể kết nối: ${escapeHTML(err.message)}</span>`;
            }
        }

        function saveApiConfigModal() {
            const input = document.getElementById('inputApiEndpoint');
            if (!input) return;
            const newUrl = input.value.trim().replace(/\/+$/, '');

            if (window.APEX_ADMIN_CONFIG && window.APEX_ADMIN_CONFIG.setApiEndpoint) {
                window.APEX_ADMIN_CONFIG.setApiEndpoint(newUrl);
            } else {
                try {
                    if (newUrl) localStorage.setItem('apex_api_endpoint', newUrl);
                    else localStorage.removeItem('apex_api_endpoint');
                } catch (_) {}
            }

            updateApiStatusBadge();
            closeApiConfigModal();
            loadCurrentTabData();
            alert('Đã cập nhật Backend API Endpoint thành công!');
        }

        function updateApiStatusBadge() {
            const badge = document.getElementById('badgeApiEndpointText');
            const dot = document.getElementById('apiStatusDot');
            const base = (window.APEX_ADMIN_CONFIG && window.APEX_ADMIN_CONFIG.API_BASE_URL) ? window.APEX_ADMIN_CONFIG.API_BASE_URL : '';
            if (!badge || !dot) return;

            if (base) {
                try {
                    const u = new URL(base);
                    badge.innerText = `API: ${u.host}`;
                } catch (_) {
                    badge.innerText = `API: ${base.substring(0, 16)}`;
                }
                dot.className = 'w-1.5 h-1.5 rounded-full bg-emerald-400';
            } else {
                badge.innerText = 'API: Local / Proxy';
                dot.className = 'w-1.5 h-1.5 rounded-full bg-cyan-400';
            }
        }

        function copyToClipboard(text, btnElement) {
            if (navigator.clipboard && navigator.clipboard.writeText) {
                navigator.clipboard.writeText(text);
            } else {
                const ta = document.createElement('textarea');
                ta.value = text;
                document.body.appendChild(ta);
                ta.select();
                document.execCommand('copy');
                document.body.removeChild(ta);
            }

            if (btnElement) {
                const orig = btnElement.innerHTML;
                btnElement.innerHTML = '<i class="fa-solid fa-check text-emerald-400"></i> Đã chép!';
                setTimeout(() => {
                    btnElement.innerHTML = orig;
                }, 2000);
            }
        }

