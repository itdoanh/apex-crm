/* ==============================================================
   APEX ADMIN - AUTHENTICATION, SESSIONS & AUDIT LOGS
   ============================================================== */

// 1. CROSS-DOMAIN API CALL INTERCEPTOR & CONFIG ENGINE
        const originalFetch = window.fetch;
        window.fetch = function(input, init) {
            init = init || {};
            // Include credentials (HttpOnly persistent cookies) in all requests
            if (!init.credentials) {
                init.credentials = 'include';
            }

            // Attach Bearer token from localStorage for Cross-Domain API calls
            const token = localStorage.getItem('apex_admin_token');
            if (token) {
                if (!init.headers) {
                    init.headers = {};
                }
                if (init.headers instanceof Headers) {
                    if (!init.headers.has('Authorization')) {
                        init.headers.set('Authorization', 'Bearer ' + token);
                    }
                } else if (Array.isArray(init.headers)) {
                    const hasAuth = init.headers.some(h => h[0].toLowerCase() === 'authorization');
                    if (!hasAuth) init.headers.push(['Authorization', 'Bearer ' + token]);
                } else if (typeof init.headers === 'object') {
                    if (!init.headers['Authorization'] && !init.headers['authorization']) {
                        init.headers['Authorization'] = 'Bearer ' + token;
                    }
                }
            }

            let url = typeof input === 'string' ? input : (input && input.url ? input.url : '');
            if (url && (url.startsWith('/api/') || url === '/api')) {
                const base = (window.APEX_ADMIN_CONFIG && window.APEX_ADMIN_CONFIG.API_BASE_URL) ? window.APEX_ADMIN_CONFIG.API_BASE_URL : '';
                if (base) {
                    url = base + url;
                    if (typeof input === 'string') {
                        input = url;
                    } else if (input && input.url) {
                        input = new Request(url, input);
                    }
                }
            }

            return originalFetch.call(this, input, init).then(res => {
                // If 401 Unauthorized on any admin API (except /api/admin/login), trigger secure login screen
                if (res.status === 401 && !url.includes('/api/admin/login')) {
                    showLoginScreen('Phiên làm việc đã hết hạn hoặc chưa xác thực. Vui lòng đăng nhập lại.');
                }
                return res;
            });
        };


// 2. ADMIN AUTH & AUDIT STATE
        let currentAdminUser = null;
        let activeSessions = [];
        let auditLogsList = [];
        let auditPage = 1;
        let auditLimit = 50;
        let auditTotal = 0;
        let auditDebounceTimer = null;


// 3. ADMIN AUTHENTICATION, ACTIVE SESSIONS & AUDIT LOGS CONTROLLER
        // ==============================================================
        async function checkAdminAuth() {
            try {
                const res = await fetch('/api/admin/me');
                if (!res.ok) {
                    showLoginScreen();
                    return false;
                }
                const json = await res.json();
                if (json && json.success && json.admin) {
                    currentAdminUser = json.admin;
                    const nameEl = document.getElementById('headerAdminUsername');
                    if (nameEl) nameEl.innerText = currentAdminUser.username;
                    hideLoginScreen();
                    return true;
                } else {
                    showLoginScreen();
                    return false;
                }
            } catch (err) {
                showLoginScreen();
                return false;
            }
        }

        function showLoginScreen(errMsg = '') {
            const screen = document.getElementById('adminLoginScreen');
            if (screen) screen.classList.remove('hidden');
            const errBox = document.getElementById('loginErrorMessage');
            const errText = document.getElementById('loginErrorText');
            if (errMsg) {
                if (errText) errText.innerText = errMsg;
                if (errBox) errBox.classList.remove('hidden');
            } else {
                if (errBox) errBox.classList.add('hidden');
            }
        }

        function hideLoginScreen() {
            const screen = document.getElementById('adminLoginScreen');
            if (screen) screen.classList.add('hidden');
        }

        async function handleAdminLogin(e) {
            e.preventDefault();
            const usernameInput = document.getElementById('loginUsername');
            const passwordInput = document.getElementById('loginPassword');
            const errBox = document.getElementById('loginErrorMessage');
            const errText = document.getElementById('loginErrorText');
            const btn = document.getElementById('btnLoginSubmit');
            const origHTML = btn ? btn.innerHTML : '';

            const username = usernameInput ? usernameInput.value.trim() : '';
            const password = passwordInput ? passwordInput.value : '';

            if (!username || !password) {
                if (errText) errText.innerText = 'Vui lòng nhập đầy đủ tài khoản và mật khẩu.';
                if (errBox) errBox.classList.remove('hidden');
                return;
            }

            if (btn) {
                btn.disabled = true;
                btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i><span>Đang xác thực...</span>';
            }
            if (errBox) errBox.classList.add('hidden');

            try {
                const res = await fetch('/api/admin/login', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ username, password })
                });

                const data = await res.json();
                if (!res.ok || !data.success) {
                    throw new Error(data.error || 'Tên đăng nhập hoặc mật khẩu không chính xác.');
                }

                // Lưu Bearer token vào localStorage cho cross-domain admin
                if (data.token) {
                    try { localStorage.setItem('apex_admin_token', data.token); } catch (_) {}
                }
                currentAdminUser = data.admin;
                const nameEl = document.getElementById('headerAdminUsername');
                if (nameEl) nameEl.innerText = currentAdminUser.username;

                hideLoginScreen();
                if (passwordInput) passwordInput.value = '';

                // Nạp lại dữ liệu hệ thống
                loadCurrentTabData();
                if (typeof loadDomains === 'function') loadDomains();
            } catch (err) {
                if (errText) errText.innerText = err.message || 'Đăng nhập thất bại. Vui lòng thử lại.';
                if (errBox) errBox.classList.remove('hidden');
            } finally {
                if (btn) {
                    btn.disabled = false;
                    btn.innerHTML = origHTML;
                }
            }
        }

        async function handleAdminLogout() {
            if (!confirm('Bạn có chắc chắn muốn đăng xuất khỏi phiên quản trị này không?')) return;
            try {
                await fetch('/api/admin/logout', { method: 'POST' });
            } catch (_) {}
            try { localStorage.removeItem('apex_admin_token'); } catch (_) {}
            currentAdminUser = null;
            showLoginScreen('Bạn đã đăng xuất an toàn khỏi hệ thống.');
        }

        function togglePasswordVisibility(inputId, btn) {
            const input = document.getElementById(inputId);
            if (!input) return;
            const icon = btn ? btn.querySelector('i') : null;
            if (input.type === 'password') {
                input.type = 'text';
                if (icon) icon.className = 'fa-regular fa-eye-slash text-xs text-indigo-400';
            } else {
                input.type = 'password';
                if (icon) icon.className = 'fa-regular fa-eye text-xs';
            }
        }

        // Đổi Mật Khẩu Modal (Argon2id)
        function openChangePasswordModal() {
            const alertBox = document.getElementById('changePasswordAlert');
            if (alertBox) alertBox.classList.add('hidden');
            document.getElementById('pwCurrent').value = '';
            document.getElementById('pwNew').value = '';
            document.getElementById('pwConfirm').value = '';
            document.getElementById('modalChangePassword').classList.remove('hidden');
        }

        function closeChangePasswordModal() {
            document.getElementById('modalChangePassword').classList.add('hidden');
        }

        async function submitChangePassword(e) {
            e.preventDefault();
            const curPw = document.getElementById('pwCurrent').value;
            const newPw = document.getElementById('pwNew').value;
            const confirmPw = document.getElementById('pwConfirm').value;
            const alertBox = document.getElementById('changePasswordAlert');
            const btn = document.getElementById('btnSubmitChangePw');

            if (newPw !== confirmPw) {
                alertBox.className = 'p-3 rounded-xl bg-rose-950/80 border border-rose-800 text-rose-300';
                alertBox.innerText = 'Mật khẩu mới và xác nhận mật khẩu không trùng khớp.';
                alertBox.classList.remove('hidden');
                return;
            }

            if (newPw.length < 8) {
                alertBox.className = 'p-3 rounded-xl bg-rose-950/80 border border-rose-800 text-rose-300';
                alertBox.innerText = 'Mật khẩu mới phải có tối thiểu 8 ký tự.';
                alertBox.classList.remove('hidden');
                return;
            }

            const origHTML = btn ? btn.innerHTML : '';
            if (btn) {
                btn.disabled = true;
                btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i><span>Đang xử lý...</span>';
            }
            alertBox.classList.add('hidden');

            try {
                const res = await fetch('/api/admin/change-password', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ current_password: curPw, new_password: newPw })
                });
                const data = await res.json();
                if (!res.ok || !data.success) {
                    throw new Error(data.error || 'Đổi mật khẩu thất bại.');
                }

                alert('✅ ' + (data.message || 'Đổi mật khẩu thành công! Vui lòng ghi nhớ mật khẩu mới.'));
                closeChangePasswordModal();
            } catch (err) {
                alertBox.className = 'p-3 rounded-xl bg-rose-950/80 border border-rose-800 text-rose-300';
                alertBox.innerText = err.message || 'Lỗi cập nhật mật khẩu.';
                alertBox.classList.remove('hidden');
            } finally {
                if (btn) {
                    btn.disabled = false;
                    btn.innerHTML = origHTML;
                }
            }
        }

        // Quản lý Phiên Đăng Nhập Đang Hoạt Động (Active Sessions)
        async function loadActiveSessions() {
            const container = document.getElementById('activeSessionsList');
            if (!container) return;

            try {
                const res = await fetch('/api/admin/sessions');
                if (!res.ok) return;
                const json = await res.json();
                if (!json || !json.success) return;

                activeSessions = json.data || [];
                const kpiCount = document.getElementById('kpiAuditActiveSessions');
                if (kpiCount) kpiCount.innerText = activeSessions.length;

                // Nạp thông tin phiên hiện tại
                const currentSess = activeSessions.find(s => s.is_current) || activeSessions[0];
                if (currentSess) {
                    const devEl = document.getElementById('kpiAuditCurrentDevice');
                    const osEl = document.getElementById('kpiAuditCurrentOSBrowser');
                    const ipEl = document.getElementById('kpiAuditCurrentIP');
                    if (devEl) devEl.innerText = (currentSess.device_type || 'Desktop').toUpperCase();
                    if (osEl) osEl.innerText = `${currentSess.browser || 'Browser'} / ${currentSess.os || 'OS'}`;
                    if (ipEl) ipEl.innerText = currentSess.ip_address || '127.0.0.1';
                }

                if (activeSessions.length === 0) {
                    container.innerHTML = '<div class="text-center py-6 text-slate-500 text-xs">Không tìm thấy phiên đăng nhập nào.</div>';
                    return;
                }

                container.innerHTML = activeSessions.map(s => {
                    let devIcon = 'fa-solid fa-laptop text-indigo-400';
                    if (s.device_type === 'mobile') devIcon = 'fa-solid fa-mobile-screen-button text-cyan-400';
                    else if (s.device_type === 'tablet') devIcon = 'fa-solid fa-tablet-screen-button text-purple-400';

                    const isCurrent = s.is_current;
                    const createdDate = s.created_at ? new Date(s.created_at).toLocaleString('vi-VN') : '--';
                    const lastActive = s.last_active_at ? new Date(s.last_active_at).toLocaleString('vi-VN') : '--';

                    return `
                        <div class="bg-slate-950/80 border ${isCurrent ? 'border-emerald-600/70 bg-emerald-950/10' : 'border-slate-800'} rounded-xl p-4 flex flex-col sm:flex-row sm:items-center justify-between gap-3.5 hover:border-slate-700 transition">
                            <div class="flex items-start gap-3.5">
                                <div class="w-11 h-11 rounded-xl bg-slate-900 border border-slate-800 flex items-center justify-center text-xl shrink-0 shadow-sm">
                                    <i class="${devIcon}"></i>
                                </div>
                                <div>
                                    <div class="flex flex-wrap items-center gap-2">
                                        <span class="font-bold text-sm text-white">${escapeHTML(s.browser || 'Trình duyệt')} trên ${escapeHTML(s.os || 'Hệ điều hành')}</span>
                                        ${isCurrent ? `
                                            <span class="bg-emerald-950 text-emerald-300 border border-emerald-800 text-[10px] px-2 py-0.2 rounded-full font-bold flex items-center gap-1 font-mono">
                                                <span class="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse"></span>
                                                <span>Thiết Bị Này (Đang Trực Tuyến)</span>
                                            </span>
                                        ` : `
                                            <span class="bg-slate-800 text-slate-400 border border-slate-700 text-[10px] px-2 py-0.2 rounded-full font-mono">Thiết Bị Khác</span>
                                        `}
                                    </div>
                                    <div class="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-slate-400 mt-1 font-mono">
                                        <span class="text-amber-400 font-semibold"><i class="fa-solid fa-network-wired mr-1 text-slate-500"></i>${escapeHTML(s.ip_address || 'Unknown')}</span>
                                        ${s.city || s.country ? `<span>&bull; ${escapeHTML(s.city || '')} ${escapeHTML(s.country || 'Việt Nam')}</span>` : ''}
                                        <span>&bull; Đăng nhập: ${createdDate}</span>
                                        <span>&bull; Hoạt động: ${lastActive}</span>
                                    </div>
                                </div>
                            </div>
                            <div class="flex items-center gap-2 shrink-0 self-end sm:self-center">
                                ${!isCurrent ? `
                                    <button type="button" onclick="handleRevokeSession(${s.id})" class="bg-rose-950/70 hover:bg-rose-900 text-rose-300 hover:text-rose-100 text-xs px-3 py-1.5 rounded-xl border border-rose-800 transition flex items-center gap-1.5 shadow-sm" title="Thu hồi phiên và đăng xuất thiết bị này ngay lập tức">
                                        <i class="fa-solid fa-ban text-[11px]"></i>
                                        <span>Đăng Xuất Thiết Bị</span>
                                    </button>
                                ` : `
                                    <button type="button" onclick="handleAdminLogout()" class="bg-slate-800 hover:bg-rose-950/80 text-slate-300 hover:text-rose-300 text-xs px-3 py-1.5 rounded-xl border border-slate-700 hover:border-rose-800 transition flex items-center gap-1.5 shadow-sm">
                                        <i class="fa-solid fa-right-from-bracket text-[11px]"></i>
                                        <span>Đăng Xuất</span>
                                    </button>
                                `}
                            </div>
                        </div>
                    `;
                }).join('');
            } catch (err) {
                console.error('Error loading sessions:', err);
            }
        }

        async function handleRevokeSession(sessionId) {
            if (!confirm('Bạn có chắc chắn muốn đăng xuất và thu hồi quyền truy cập của thiết bị này không?')) return;
            try {
                const res = await fetch('/api/admin/sessions/revoke', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ session_id: sessionId })
                });
                const data = await res.json();
                if (!res.ok || !data.success) throw new Error(data.error || 'Thu hồi thất bại');
                await loadActiveSessions();
                loadAuditLogs(1);
            } catch (err) {
                alert('Lỗi: ' + err.message);
            }
        }

        async function handleRevokeAllOtherSessions() {
            if (!confirm('Bạn có chắc chắn muốn đăng xuất khỏi TẤT CẢ các thiết bị khác không? Các phiên đăng nhập trên trình duyệt/máy tính khác sẽ lập tức bị thu hồi.')) return;
            try {
                const res = await fetch('/api/admin/sessions/revoke-others', { method: 'POST' });
                const data = await res.json();
                if (!res.ok || !data.success) throw new Error(data.error || 'Thu hồi thất bại');
                alert('✅ ' + (data.message || 'Đã đăng xuất khỏi tất cả các thiết bị khác thành công!'));
                await loadActiveSessions();
                loadAuditLogs(1);
            } catch (err) {
                alert('Lỗi: ' + err.message);
            }
        }

        // Quản lý Nhật Ký Hoạt Động (Audit Logs)
        function debounceLoadAuditLogs() {
            clearTimeout(auditDebounceTimer);
            auditDebounceTimer = setTimeout(() => {
                auditPage = 1;
                loadAuditLogs(1);
            }, 300);
        }

        function prevAuditPage() {
            if (auditPage > 1) {
                auditPage--;
                loadAuditLogs(auditPage);
            }
        }

        function nextAuditPage() {
            const maxPage = Math.ceil(auditTotal / auditLimit) || 1;
            if (auditPage < maxPage) {
                auditPage++;
                loadAuditLogs(auditPage);
            }
        }

        async function loadAuditLogs(page = 1) {
            auditPage = page;
            const tbody = document.getElementById('auditLogsTableBody');
            const searchInput = document.getElementById('filterAuditSearch');
            const actionSelect = document.getElementById('filterAuditAction');
            const limitSelect = document.getElementById('filterAuditLimit');

            const search = searchInput ? searchInput.value.trim() : '';
            const action = actionSelect ? actionSelect.value : '';
            const limit = limitSelect ? parseInt(limitSelect.value, 10) : 50;
            auditLimit = limit;

            if (tbody) {
                tbody.innerHTML = `<tr><td colspan="7" class="py-8 text-center text-slate-500"><i class="fa-solid fa-spinner fa-spin mr-1.5 text-indigo-400"></i> Đang tải dữ liệu nhật ký bảo mật...</td></tr>`;
            }

            try {
                const params = new URLSearchParams({
                    page: auditPage,
                    limit: auditLimit,
                    search: search,
                    action: action
                });

                const res = await fetch('/api/admin/audit-logs?' + params.toString());
                if (!res.ok) throw new Error('Không thể tải nhật ký');
                const json = await res.json();
                if (!json || !json.success) throw new Error(json.error || 'Lỗi truy vấn');

                auditLogsList = json.data || [];
                auditTotal = json.total || 0;

                // Cập nhật counters & badges
                const kpiTotal = document.getElementById('kpiAuditTotal');
                const badgeCount = document.getElementById('badgeAuditCount');
                if (kpiTotal) kpiTotal.innerText = Number(auditTotal).toLocaleString();
                if (badgeCount) badgeCount.innerText = Number(auditTotal).toLocaleString();

                const countText = document.getElementById('auditLogsCountText');
                if (countText) countText.innerText = `Hiển thị ${auditLogsList.length}/${auditTotal} hành vi ghi nhận`;

                const pageInd = document.getElementById('auditPageIndicator');
                const maxPage = Math.ceil(auditTotal / auditLimit) || 1;
                if (pageInd) pageInd.innerText = `Trang ${auditPage}/${maxPage}`;

                const btnPrev = document.getElementById('btnAuditPrevPage');
                const btnNext = document.getElementById('btnAuditNextPage');
                if (btnPrev) btnPrev.disabled = (auditPage <= 1);
                if (btnNext) btnNext.disabled = (auditPage >= maxPage);

                if (!tbody) return;
                if (auditLogsList.length === 0) {
                    tbody.innerHTML = `<tr><td colspan="7" class="py-8 text-center text-slate-500">Chưa tìm thấy nhật ký hoạt động nào phù hợp với bộ lọc.</td></tr>`;
                    return;
                }

                tbody.innerHTML = auditLogsList.map(log => {
                    const timeStr = log.created_at ? new Date(log.created_at).toLocaleString('vi-VN') : '--';
                    const actBadge = getActionBadge(log.action, log.status);

                    let devIcon = 'fa-solid fa-desktop text-slate-400';
                    if (log.device_type === 'mobile') devIcon = 'fa-solid fa-mobile-screen-button text-cyan-400';
                    else if (log.device_type === 'tablet') devIcon = 'fa-solid fa-tablet text-purple-400';

                    return `
                        <tr class="hover:bg-slate-800/40 transition">
                            <td class="py-3 px-3.5 font-mono text-[11px] text-slate-400 whitespace-nowrap">${timeStr}</td>
                            <td class="py-3 px-3.5 whitespace-nowrap">${actBadge}</td>
                            <td class="py-3 px-3.5 whitespace-nowrap">
                                <span class="font-bold text-slate-200">${escapeHTML(log.username || 'admin')}</span>
                            </td>
                            <td class="py-3 px-3.5 whitespace-nowrap">
                                <span class="font-mono text-[11px] text-slate-300">${escapeHTML(log.resource || '--')}</span>
                                ${log.resource_id ? `<span class="text-slate-500 font-mono text-[10px]"> (${escapeHTML(log.resource_id)})</span>` : ''}
                            </td>
                            <td class="py-3 px-3.5 whitespace-nowrap text-slate-300">
                                <div class="flex items-center gap-1.5 text-xs">
                                    <i class="${devIcon}"></i>
                                    <span>${escapeHTML(log.browser || 'Browser')} / ${escapeHTML(log.os || 'OS')}</span>
                                </div>
                            </td>
                            <td class="py-3 px-3.5 font-mono text-amber-400 font-semibold whitespace-nowrap">${escapeHTML(log.ip_address || '--')}</td>
                            <td class="py-3 px-3.5 text-center whitespace-nowrap">
                                <button type="button" onclick="openAuditDetailModal(${log.id})" class="bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white px-2.5 py-1 rounded-lg border border-slate-700 text-[11px] font-mono transition inline-flex items-center gap-1 shadow-sm">
                                    <i class="fa-solid fa-eye text-cyan-400 text-[10px]"></i> Chi Tiết
                                </button>
                            </td>
                        </tr>
                    `;
                }).join('');
            } catch (err) {
                if (tbody) {
                    tbody.innerHTML = `<tr><td colspan="7" class="py-8 text-center text-rose-400">Lỗi tải nhật ký: ${escapeHTML(err.message)}</td></tr>`;
                }
            }
        }

        function getActionBadge(action, status) {
            const a = action || '';
            let label = a;
            let badgeClass = 'bg-slate-800 text-slate-300 border-slate-700';

            if (a === 'AUTH_LOGIN_SUCCESS') {
                label = 'Đăng Nhập Thành Công';
                badgeClass = 'bg-emerald-950 text-emerald-300 border-emerald-800';
            } else if (a === 'AUTH_LOGIN_FAILED') {
                label = 'Đăng Nhập Thất Bại';
                badgeClass = 'bg-rose-950 text-rose-300 border-rose-800';
            } else if (a === 'AUTH_LOGOUT') {
                label = 'Đăng Xuất';
                badgeClass = 'bg-slate-800 text-slate-300 border-slate-700';
            } else if (a === 'REVOKE_SESSION') {
                label = 'Thu Hồi Phiên (1 Thiết Bị)';
                badgeClass = 'bg-rose-950 text-rose-300 border-rose-800';
            } else if (a === 'REVOKE_ALL_OTHER_SESSIONS') {
                label = 'Đăng Xuất Tất Cả Thiết Bị Khác';
                badgeClass = 'bg-rose-950 text-rose-200 border-rose-700';
            } else if (a === 'UPDATE_PASSWORD') {
                label = 'Đổi Mật Khẩu (Argon2id)';
                badgeClass = 'bg-amber-950 text-amber-300 border-amber-800';
            } else if (a === 'UPDATE_LEAD_STATUS') {
                label = 'Đổi Trạng Thái Lead';
                badgeClass = 'bg-sky-950 text-sky-300 border-sky-800';
            } else if (a === 'EXPORT_LEADS_EXCEL') {
                label = 'Xuất File Excel/CSV';
                badgeClass = 'bg-indigo-950 text-indigo-300 border-indigo-800';
            } else if (a === 'SAVE_DOMAIN') {
                label = 'Lưu Tên Miền';
                badgeClass = 'bg-cyan-950 text-cyan-300 border-cyan-800';
            } else if (a === 'DELETE_DOMAIN') {
                label = 'Xóa Tên Miền';
                badgeClass = 'bg-rose-950 text-rose-300 border-rose-800';
            } else if (a === 'DEPLOY_DOMAIN_ROUTE') {
                label = 'Deploy Landing Route';
                badgeClass = 'bg-teal-950 text-teal-300 border-teal-800';
            } else if (a === 'DELETE_DOMAIN_ROUTE') {
                label = 'Xóa Route Tên Miền';
                badgeClass = 'bg-rose-950 text-rose-300 border-rose-800';
            }

            return `<span class="${badgeClass} border text-[10px] px-2 py-0.5 rounded-full font-bold font-mono inline-block">${escapeHTML(label)}</span>`;
        }

        let selectedAuditLogForModal = null;
        function openAuditDetailModal(logId) {
            const log = auditLogsList.find(l => l.id === logId);
            if (!log) return;
            selectedAuditLogForModal = log;

            document.getElementById('auditDetailTitleSubtitle').innerText = `ID: #${log.id} &bull; Resource: ${log.resource || '--'}`;
            document.getElementById('auditDetailTime').innerText = log.created_at ? new Date(log.created_at).toLocaleString('vi-VN') : '--';
            document.getElementById('auditDetailAction').innerText = log.action || '--';
            document.getElementById('auditDetailUser').innerText = log.username || 'admin';
            document.getElementById('auditDetailIP').innerText = log.ip_address || '--';
            document.getElementById('auditDetailDevice').innerText = `${log.device_type || 'desktop'} | ${log.browser || ''} | ${log.os || ''} | UA: ${log.user_agent || '--'}`;

            let formattedJson = '{}';
            try {
                formattedJson = JSON.stringify(log.details || {}, null, 2);
            } catch (_) {
                formattedJson = String(log.details);
            }
            document.getElementById('auditDetailJson').innerText = formattedJson;

            document.getElementById('modalAuditDetail').classList.remove('hidden');
        }

        function closeAuditDetailModal() {
            document.getElementById('modalAuditDetail').classList.add('hidden');
        }

        function copyAuditDetailsJson(btn) {
            if (!selectedAuditLogForModal) return;
            let jsonText = '';
            try {
                jsonText = JSON.stringify(selectedAuditLogForModal.details || {}, null, 2);
            } catch (_) {
                jsonText = String(selectedAuditLogForModal.details);
            }
            copyToClipboard(jsonText, btn);
        }

        // 18. Utilities
        function escapeHTML(str) {
            if (str == null) return '';
            return String(str)
                .replace(/&/g, '&amp;')
                .replace(/</g, '&lt;')
                .replace(/>/g, '&gt;')
                .replace(/"/g, '&quot;')
                .replace(/'/g, '&#39;');
        }

