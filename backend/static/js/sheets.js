/* ==============================================================
   APEX ADMIN - GOOGLE SHEETS MULTI-DESTINATION INTEGRATION
   ============================================================== */

        // ==============================================================
        // GOOGLE SHEETS MULTI-DESTINATION INTEGRATION CONTROLLER
        // ==============================================================
        let sheetConfigs = [];
        let currentFieldMappings = [];
        let availableLeadFields = [
            { key: "thoi_gian", label: "Thời Gian Đăng Ký" },
            { key: "fullName", label: "Họ Và Tên" },
            { key: "so_dien_thoai", label: "Số Điện Thoại" },
            { key: "carrier", label: "Nhà Mạng Viễn Thông" },
            { key: "landing_id", label: "ID Trang Landing" },
            { key: "nguon", label: "Nguồn Traffic (Kênh)" },
            { key: "trang_thai", label: "Trạng Thái Lead (Pipeline)" },
            { key: "ghi_chu", label: "Ghi Chú Tư Vấn" },
            { key: "is_autofill", label: "Tự Động Điền (1-Tap)" },
            { key: "fill_duration_ms", label: "Tốc Độ Điền Form (ms)" },
            { key: "time_on_page", label: "Thời Gian Xem Trang" },
            { key: "max_scroll", label: "Tỷ Lệ Cuộn Trang (%)" },
            { key: "device_type", label: "Thiết Bị (Mobile/Desktop)" },
            { key: "ip_address", label: "Địa Chỉ IP" },
            { key: "utm_source", label: "UTM Source" },
            { key: "utm_medium", label: "UTM Medium" },
            { key: "utm_campaign", label: "UTM Campaign" },
            { key: "utm_content", label: "UTM Content" },
            { key: "utm_term", label: "UTM Term" },
            { key: "session_id", label: "Mã Phiên (Session ID)" },
            { key: "tracking_json", label: "Toàn Bộ JSON Tracking" },
            { key: "custom_text", label: "Văn Bản Cố Định" }
        ];

        let systemBotEmail = "sheet-bot@rinva-drive-tool.iam.gserviceaccount.com";

        async function fetchBotInfo() {
            try {
                const res = await fetch('/api/integrations/sheets/bot-info');
                const json = await res.json();
                if (json && json.success && json.bot_email) {
                    systemBotEmail = json.bot_email;
                    const b1 = document.getElementById('bannerBotEmailDisplay');
                    if (b1) b1.innerText = systemBotEmail;
                    const b2 = document.getElementById('gsModalBotEmailDisplay');
                    if (b2) b2.innerText = systemBotEmail;
                }
            } catch(e) {}
        }

        function copySystemBotEmail() {
            navigator.clipboard.writeText(systemBotEmail).then(() => {
                const btnText = document.getElementById('btnCopySystemBotEmailText');
                if (btnText) {
                    const old = btnText.innerText;
                    btnText.innerText = "Đã chép!";
                    setTimeout(() => { btnText.innerText = old; }, 2000);
                }
            }).catch(() => {
                prompt("Sao chép email bot để cấp quyền Editor trên Google Sheet:", systemBotEmail);
            });
        }

        function copyBotEmailFromModal() {
            navigator.clipboard.writeText(systemBotEmail).then(() => {
                const btnText = document.getElementById('btnCopyBotEmailModalText');
                if (btnText) {
                    const old = btnText.innerText;
                    btnText.innerText = "Đã sao chép!";
                    setTimeout(() => { btnText.innerText = old; }, 2000);
                }
            }).catch(() => {
                prompt("Sao chép email bot để cấp quyền Editor trên Google Sheet:", systemBotEmail);
            });
        }

        function toggleAdvancedCustomKey() {
            const c = document.getElementById('advancedCustomKeyContainer');
            const chev = document.getElementById('advancedKeyChevron');
            if (!c) return;
            const isHidden = c.classList.contains('hidden');
            if (isHidden) {
                c.classList.remove('hidden');
                if (chev) chev.classList.add('rotate-180');
            } else {
                c.classList.add('hidden');
                if (chev) chev.classList.remove('rotate-180');
            }
        }

        async function loadGoogleSheetsConfigs() {
            fetchBotInfo();
            try {
                const res = await fetch('/api/integrations/sheets/configs');
                const json = await res.json();
                if (!json || !json.success) return;

                sheetConfigs = json.configs || [];

                // Cập nhật Counters
                const total = sheetConfigs.length;
                const active = sheetConfigs.filter(c => c.is_active).length;
                document.getElementById('statSheetsTotal').innerText = total;
                document.getElementById('statSheetsActive').innerText = active;
                document.getElementById('badgeSheetsActive').innerText = active;

                // Nạp số lượng đã sync
                try {
                    const logRes = await fetch('/api/integrations/sheets/logs?limit=1');
                    const logJson = await logRes.json();
                    if (logJson && logJson.success) {
                        document.getElementById('statSheetsSyncedCount').innerText = logJson.total || 0;
                    }
                } catch(e) {}

                // Cập nhật dropdown bộ lọc logs
                const selectLog = document.getElementById('logFilterConfigSelect');
                if (selectLog) {
                    let curVal = selectLog.value;
                    selectLog.innerHTML = '<option value="0">Tất cả cấu hình Sheet</option>';
                    sheetConfigs.forEach(c => {
                        selectLog.innerHTML += `<option value="${c.id}">${escapeHTML(c.name)}</option>`;
                    });
                    selectLog.value = curVal;
                }

                // Render danh sách Sheets
                const container = document.getElementById('sheetsListContainer');
                if (!container) return;

                if (sheetConfigs.length === 0) {
                    container.innerHTML = `
                        <div class="text-center py-12 px-4 border border-dashed border-slate-800 rounded-xl">
                            <div class="w-14 h-14 mx-auto rounded-2xl bg-emerald-500/10 border border-emerald-500/30 flex items-center justify-center text-emerald-400 text-2xl mb-3">
                                <i class="fa-solid fa-table-cells-large"></i>
                            </div>
                            <h4 class="text-sm font-bold text-white">Chưa có kết nối Google Sheets nào</h4>
                            <p class="text-xs text-slate-400 max-w-md mx-auto mt-1 mb-4 leading-relaxed">
                                Bạn có thể tạo kết nối tới một hoặc nhiều Google Sheets (cho từng Landing hoặc nguồn chiến dịch riêng) để tự động bắn dữ liệu Lead thời gian thực ngay khi khách đăng ký.
                            </p>
                            <button onclick="openGoogleSheetModal()" class="bg-gradient-to-r from-emerald-600 to-teal-600 hover:from-emerald-500 hover:to-teal-500 text-white text-xs font-bold px-4 py-2 rounded-xl transition shadow flex items-center gap-2 mx-auto">
                                <i class="fa-solid fa-plus"></i>
                                <span>Thêm Kết Nối Google Sheet Đầu Tiên</span>
                            </button>
                        </div>
                    `;
                    return;
                }

                container.innerHTML = sheetConfigs.map(c => {
                    const sheetUrl = `https://docs.google.com/spreadsheets/d/${c.spreadsheet_id}/edit`;
                    const mapCount = (c.field_mappings || []).length;
                    const lastSyncText = c.last_synced_at ? `<span class="text-emerald-400 font-mono">${c.last_synced_at}</span>` : '<span class="text-slate-500">Chưa có lượt đồng bộ</span>';

                    return `
                        <div class="bg-slate-950/80 border ${c.is_active ? 'border-slate-800' : 'border-slate-800/50 opacity-70'} rounded-xl p-4 transition hover:border-slate-700 shadow flex flex-col lg:flex-row items-start lg:items-center justify-between gap-4">
                            <div class="flex items-start gap-3.5 flex-1 min-w-0">
                                <div class="w-11 h-11 rounded-xl ${c.is_active ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-400' : 'bg-slate-800 border-slate-700 text-slate-400'} border flex items-center justify-center text-xl shrink-0">
                                    <i class="fa-solid fa-file-excel"></i>
                                </div>
                                <div class="min-w-0 flex-1">
                                    <div class="flex flex-wrap items-center gap-2">
                                        <h4 class="text-sm font-bold text-white truncate">${escapeHTML(c.name)}</h4>
                                        <span class="text-[10px] px-2 py-0.5 rounded-full font-bold ${c.is_active ? 'bg-emerald-950 text-emerald-400 border border-emerald-800/80' : 'bg-slate-800 text-slate-400 border border-slate-700'}">
                                            ${c.is_active ? 'ĐANG ĐỒNG BỘ' : 'ĐÃ TẮT'}
                                        </span>
                                        ${c.filter_landing_id ? `<span class="text-[10px] bg-slate-900 text-orange-300 border border-orange-800/60 px-2 py-0.5 rounded font-mono">Landing: ${escapeHTML(c.filter_landing_id)}</span>` : '<span class="text-[10px] bg-slate-900 text-slate-400 border border-slate-800 px-2 py-0.5 rounded">Tất cả Landing</span>'}
                                        ${c.filter_nguon ? `<span class="text-[10px] bg-slate-900 text-sky-300 border border-sky-800/60 px-2 py-0.5 rounded font-mono">Nguồn: ${escapeHTML(c.filter_nguon)}</span>` : ''}
                                    </div>

                                    <div class="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-slate-400 mt-1.5">
                                        <span class="flex items-center gap-1.5">
                                            <i class="fa-solid fa-table text-slate-500"></i>
                                            <span class="text-white font-semibold">${escapeHTML(c.spreadsheet_name || c.spreadsheet_id)}</span>
                                            <span class="text-slate-500">&rarr;</span>
                                            <span class="text-emerald-300 font-mono font-bold bg-slate-900 px-1.5 py-0.2 rounded border border-slate-800">${escapeHTML(c.sheet_tab_name)}</span>
                                        </span>
                                        <a href="${sheetUrl}" target="_blank" class="text-sky-400 hover:underline flex items-center gap-1 font-mono text-[11px]" title="Mở trên Google Docs">
                                            <i class="fa-solid fa-arrow-up-right-from-square"></i> Mở Sheet
                                        </a>
                                        <span><i class="fa-solid fa-columns text-slate-500 mr-1"></i>${mapCount} cột ánh xạ</span>
                                        <span><i class="fa-solid fa-clock-rotate-left text-slate-500 mr-1"></i>Gần nhất: ${lastSyncText}</span>
                                    </div>

                                    ${c.service_account_email ? `
                                        <div class="text-[11px] text-slate-500 font-mono mt-1 flex items-center gap-1">
                                            <i class="fa-solid fa-robot text-slate-600"></i>
                                            <span>Service Account: ${escapeHTML(c.service_account_email)}</span>
                                        </div>
                                    ` : ''}
                                </div>
                            </div>

                            <!-- Actions -->
                            <div class="flex flex-wrap items-center gap-2 self-end lg:self-center shrink-0">
                                <!-- Active Toggle -->
                                <button onclick="toggleGoogleSheetConfig(${c.id}, ${!c.is_active})" class="text-xs px-3 py-1.5 rounded-lg border font-semibold transition ${c.is_active ? 'bg-emerald-950/60 hover:bg-emerald-900/60 text-emerald-300 border-emerald-800' : 'bg-slate-800 hover:bg-slate-700 text-slate-400 border-slate-700'}" title="Bật/Tắt đồng bộ">
                                    <i class="fa-solid ${c.is_active ? 'fa-toggle-on text-emerald-400' : 'fa-toggle-off text-slate-500'} mr-1.5 text-sm"></i>
                                    <span>${c.is_active ? 'Bật' : 'Tắt'}</span>
                                </button>

                                <button onclick="testConfigRow(${c.id})" class="bg-sky-950/60 hover:bg-sky-900/60 text-sky-300 border border-sky-800 text-xs px-3 py-1.5 rounded-lg font-semibold transition flex items-center gap-1.5" title="Bắn 1 dòng test mẫu vào Sheet">
                                    <i class="fa-solid fa-paper-plane text-sky-400"></i>
                                    <span>Test Dòng</span>
                                </button>

                                <button onclick="openSyncLogsModal(${c.id})" class="bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700 text-xs px-3 py-1.5 rounded-lg font-semibold transition flex items-center gap-1.5" title="Xem lịch sử đồng bộ của Sheet này">
                                    <i class="fa-solid fa-list-ul"></i>
                                    <span>Lịch Sử</span>
                                </button>

                                <button onclick="openGoogleSheetModal(${c.id})" class="bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 text-xs px-3 py-1.5 rounded-lg font-semibold transition flex items-center gap-1.5" title="Chỉnh sửa cấu hình & ánh xạ cột">
                                    <i class="fa-solid fa-pen-to-square text-amber-400"></i>
                                    <span>Sửa</span>
                                </button>

                                <button onclick="deleteGoogleSheetConfig(${c.id}, '${escapeHTML(c.name)}')" class="bg-rose-950/60 hover:bg-rose-900/60 text-rose-300 border border-rose-800 text-xs px-2.5 py-1.5 rounded-lg font-semibold transition" title="Xóa cấu hình này">
                                    <i class="fa-solid fa-trash-can"></i>
                                </button>
                            </div>
                        </div>
                    `;
                }).join('');
            } catch(err) {
                console.error("Lỗi nạp cấu hình Google Sheets:", err);
            }
        }

        async function openGoogleSheetModal(configId = 0) {
            document.getElementById('gsConfigId').value = configId;
            document.getElementById('gsTestResultBox').className = 'hidden';
            document.getElementById('driveFilesContainer').classList.add('hidden');
            fetchBotInfo();

            const customContainer = document.getElementById('advancedCustomKeyContainer');
            const chev = document.getElementById('advancedKeyChevron');

            if (configId > 0) {
                document.getElementById('gsModalTitle').innerText = "Chỉnh Sửa Cấu Hình Google Sheet";
                try {
                    const res = await fetch(`/api/integrations/sheets/configs/get?id=${configId}`);
                    const json = await res.json();
                    if (json && json.success && json.config) {
                        const cfg = json.config;
                        document.getElementById('gsConfigName').value = cfg.name || '';
                        document.getElementById('gsCredentialsJson').value = cfg.credentials_json || '';
                        document.getElementById('gsSpreadsheetInput').value = cfg.spreadsheet_id || '';
                        document.getElementById('gsFilterLandingId').value = cfg.filter_landing_id || '';
                        document.getElementById('gsFilterNguon').value = cfg.filter_nguon || '';
                        document.getElementById('gsAutoHeader').checked = cfg.auto_create_header !== false;

                        detectServiceAccountEmail();

                        if (cfg.credentials_json && customContainer) {
                            customContainer.classList.remove('hidden');
                            if (chev) chev.classList.add('rotate-180');
                        } else if (customContainer) {
                            customContainer.classList.add('hidden');
                            if (chev) chev.classList.remove('rotate-180');
                        }

                        currentFieldMappings = (cfg.field_mappings && cfg.field_mappings.length > 0) ? cfg.field_mappings : getDefaultStandardMappings();
                        renderMappingsTable();

                        // Nạp metadata để điền tab dropdown
                        if (cfg.spreadsheet_id) {
                            fetchSheetMetadata(cfg.sheet_tab_name);
                        }
                    }
                } catch(e) {
                    alert("Lỗi tải chi tiết cấu hình: " + e.message);
                }
            } else {
                document.getElementById('gsModalTitle').innerText = "Thêm Mới Kết Nối Google Sheet";
                document.getElementById('gsConfigName').value = "Sheet Tổng Hợp Leads";
                document.getElementById('gsCredentialsJson').value = "";
                document.getElementById('gsSpreadsheetInput').value = "";
                document.getElementById('gsFilterLandingId').value = "";
                document.getElementById('gsFilterNguon').value = "";
                document.getElementById('gsAutoHeader').checked = true;
                document.getElementById('gsDetectedEmailBadge').classList.add('hidden');
                document.getElementById('gsMetaResult').classList.add('hidden');
                if (customContainer) customContainer.classList.add('hidden');
                if (chev) chev.classList.remove('rotate-180');

                currentFieldMappings = getDefaultStandardMappings();
                renderMappingsTable();
            }

            document.getElementById('googleSheetModal').classList.remove('hidden');
        }

        function closeGoogleSheetModal() {
            document.getElementById('googleSheetModal').classList.add('hidden');
        }

        function detectServiceAccountEmail() {
            const raw = document.getElementById('gsCredentialsJson').value;
            const badge = document.getElementById('gsDetectedEmailBadge');
            const emailSpan = document.getElementById('gsDetectedEmailText');

            try {
                const parsed = JSON.parse(raw);
                if (parsed.client_email) {
                    emailSpan.innerText = parsed.client_email;
                    badge.classList.remove('hidden');
                    return parsed.client_email;
                }
            } catch(e) {
                // Regex fallback
                const match = raw.match(/"client_email"\s*:\s*"([^"]+)"/);
                if (match && match[1]) {
                    emailSpan.innerText = match[1];
                    badge.classList.remove('hidden');
                    return match[1];
                }
            }

            badge.classList.add('hidden');
            return "";
        }

        function copyServiceAccountEmail() {
            const email = document.getElementById('gsDetectedEmailText').innerText;
            if (email) {
                navigator.clipboard.writeText(email).then(() => {
                    alert("Đã sao chép email Service Account:\n" + email + "\n\nHãy mở Google Sheet -> Bấm nút 'Chia sẻ' -> Thêm email này với quyền 'Người chỉnh sửa (Editor)'.");
                });
            }
        }

        async function fetchSheetMetadata(selectedTab = "") {
            const creds = document.getElementById('gsCredentialsJson').value.trim();
            const sheetInput = document.getElementById('gsSpreadsheetInput').value.trim();

            if (!sheetInput) {
                alert("Vui lòng dán Link Google Sheet hoặc Spreadsheet ID vào ô nhập.");
                document.getElementById('gsSpreadsheetInput').focus();
                return;
            }

            const btn = document.getElementById('btnFetchMeta');
            btn.disabled = true;
            btn.innerHTML = `<i class="fa-solid fa-spinner fa-spin"></i> <span>Đang kết nối...</span>`;

            try {
                const res = await fetch('/api/integrations/sheets/meta', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        credentials_json: creds,
                        spreadsheet_id: sheetInput
                    })
                });

                const json = await res.json();
                if (!json.success) {
                    const errMsg = json.error || "Không rõ nguyên nhân";
                    alert("Không thể kết nối Google Sheets:\n\n" + errMsg + "\n\n💡 MẸO: Hãy kiểm tra bạn đã bấm 'Chia sẻ' trên Google Sheet và thêm email '" + systemBotEmail + "' với quyền 'Người chỉnh sửa (Editor)' chưa.");
                    return;
                }

                const d = json.data;
                document.getElementById('gsSpreadsheetTitleDisplay').innerText = d.title || 'Google Sheet';
                document.getElementById('gsSpreadsheetLinkDisplay').href = d.spreadsheet_url || `https://docs.google.com/spreadsheets/d/${d.spreadsheet_id}/edit`;
                document.getElementById('gsServiceAccountDisplay').innerText = `Tài khoản: ${d.service_account || systemBotEmail}`;

                const tabSelect = document.getElementById('gsTabSelect');
                tabSelect.innerHTML = (d.tabs || []).map(t => {
                    const sel = (selectedTab && t.title === selectedTab) ? 'selected' : '';
                    return `<option value="${escapeHTML(t.title)}" ${sel}>${escapeHTML(t.title)} (${t.row_count} dòng x ${t.column_count} cột)</option>`;
                }).join('');

                document.getElementById('gsMetaResult').classList.remove('hidden');
            } catch(err) {
                alert("Lỗi kết nối API: " + err.message);
            } finally {
                btn.disabled = false;
                btn.innerHTML = `<i class="fa-solid fa-bolt"></i> <span>Kết Nối &amp; Tải Cấu Trúc</span>`;
            }
        }

        async function browseDriveFiles() {
            const creds = document.getElementById('gsCredentialsJson').value.trim();

            const btn = document.getElementById('btnBrowseDrive');
            btn.disabled = true;
            btn.innerHTML = `<i class="fa-solid fa-spinner fa-spin"></i> <span>Đang duyệt...</span>`;

            try {
                const res = await fetch('/api/integrations/sheets/drive-files', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ credentials_json: creds })
                });

                const json = await res.json();
                if (!json.success) {
                    alert("Không thể duyệt Google Drive:\n" + json.error);
                    return;
                }

                const files = json.files || [];
                const container = document.getElementById('driveFilesContainer');
                const select = document.getElementById('driveFilesSelect');

                if (files.length === 0) {
                    alert("Không tìm thấy file Spreadsheet nào trong Google Drive của Bot.\n\nHãy mở Google Sheet của bạn -> Bấm 'Chia sẻ' -> Thêm email bot: " + systemBotEmail + " với quyền Editor.");
                    return;
                }

                select.innerHTML = '<option value="">-- Chọn một file Sheet từ Google Drive --</option>' +
                    files.map(f => `<option value="${f.id}">${escapeHTML(f.name)} (ID: ${f.id})</option>`).join('');

                container.classList.remove('hidden');
            } catch(err) {
                alert("Lỗi duyệt Drive: " + err.message);
            } finally {
                btn.disabled = false;
                btn.innerHTML = `<i class="fa-brands fa-google-drive text-amber-400"></i> <span>Duyệt Drive</span>`;
            }
        }

        function selectDriveFile() {
            const select = document.getElementById('driveFilesSelect');
            const fileId = select.value;
            if (fileId) {
                document.getElementById('gsSpreadsheetInput').value = fileId;
                fetchSheetMetadata();
            }
        }

        function onTabSelected() {
            // Tab change listener
        }

        async function fetchSheetHeaders() {
            const creds = document.getElementById('gsCredentialsJson').value.trim();
            const sheetInput = document.getElementById('gsSpreadsheetInput').value.trim();
            const tabName = document.getElementById('gsTabSelect').value;

            if (!sheetInput || !tabName) {
                alert("Vui lòng bấm 'Kết Nối & Tải Cấu Trúc' trước để chọn tab trang tính.");
                return;
            }

            try {
                const res = await fetch('/api/integrations/sheets/headers', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        credentials_json: creds,
                        spreadsheet_id: sheetInput,
                        tab_name: tabName
                    })
                });
                const json = await res.json();
                if (!json.success) {
                    alert("Lỗi đọc tiêu đề: " + json.error);
                    return;
                }

                const headers = json.headers || [];
                if (headers.length === 0) {
                    alert(`Tab '${tabName}' chưa có dòng tiêu đề nào (dòng 1 trống). Hãy dùng 14 cột chuẩn hoặc tự nhập.`);
                    return;
                }

                // Tự động map header dựa trên tên
                currentFieldMappings = headers.map((h, i) => {
                    const colLetter = getColumnLetter(i);
                    const matchedField = guessFieldFromHeader(h);
                    return {
                        column: colLetter,
                        header: h,
                        field: matchedField,
                        custom_text: ""
                    };
                });

                renderMappingsTable();
                alert(`Đã nhận diện thành công ${headers.length} cột từ dòng 1 của tab '${tabName}'!`);
            } catch(err) {
                alert("Lỗi đọc tiêu đề: " + err.message);
            }
        }

        function guessFieldFromHeader(header) {
            const h = header.toLowerCase().trim();
            if (h.includes("thời gian") || h.includes("ngày") || h.includes("time") || h.includes("date")) return "thoi_gian";
            if (h.includes("họ tên") || h.includes("họ và tên") || h.includes("tên") || h.includes("name")) return "fullName";
            if (h.includes("sđt") || h.includes("điện thoại") || h.includes("phone") || h.includes("tel")) return "so_dien_thoai";
            if (h.includes("nhà mạng") || h.includes("carrier") || h.includes("mạng")) return "carrier";
            if (h.includes("landing") || h.includes("trang")) return "landing_id";
            if (h.includes("nguồn") || h.includes("source") || h.includes("kênh")) return "nguon";
            if (h.includes("trạng thái") || h.includes("status") || h.includes("pipeline")) return "trang_thai";
            if (h.includes("ghi chú") || h.includes("note") || h.includes("nhận xét")) return "ghi_chu";
            if (h.includes("tự động điền") || h.includes("autofill")) return "is_autofill";
            if (h.includes("chiến dịch") || h.includes("campaign")) return "utm_campaign";
            if (h.includes("ip") || h.includes("địa chỉ ip")) return "ip_address";
            if (h.includes("thiết bị") || h.includes("device")) return "device_type";
            if (h.includes("thời gian xem") || h.includes("time on page")) return "time_on_page";
            if (h.includes("cuộn") || h.includes("scroll")) return "max_scroll";
            if (h.includes("tracking") || h.includes("raw")) return "tracking_json";
            return "fullName";
        }

        function getDefaultStandardMappings() {
            return [
                { column: "A", header: "Thời Gian", field: "thoi_gian", custom_text: "" },
                { column: "B", header: "Họ Và Tên", field: "fullName", custom_text: "" },
                { column: "C", header: "Số Điện Thoại", field: "so_dien_thoai", custom_text: "" },
                { column: "D", header: "Nhà Mạng", field: "carrier", custom_text: "" },
                { column: "E", header: "Landing ID", field: "landing_id", custom_text: "" },
                { column: "F", header: "Nguồn Traffic", field: "nguon", custom_text: "" },
                { column: "G", header: "Trạng Thái CRM", field: "trang_thai", custom_text: "" },
                { column: "H", header: "Tự Động Điền", field: "is_autofill", custom_text: "" },
                { column: "I", header: "UTM Campaign", field: "utm_campaign", custom_text: "" },
                { column: "J", header: "Địa Chỉ IP", field: "ip_address", custom_text: "" },
                { column: "K", header: "Thiết Bị", field: "device_type", custom_text: "" },
                { column: "L", header: "Thời Gian Xem Trang", field: "time_on_page", custom_text: "" },
                { column: "M", header: "Tỷ Lệ Cuộn", field: "max_scroll", custom_text: "" },
                { column: "N", header: "Ghi Chú", field: "ghi_chu", custom_text: "" }
            ];
        }

        function resetDefaultMappings() {
            currentFieldMappings = getDefaultStandardMappings();
            renderMappingsTable();
        }

        function getColumnLetter(index) {
            let letter = '';
            while (index >= 0) {
                letter = String.fromCharCode((index % 26) + 65) + letter;
                index = Math.floor(index / 26) - 1;
            }
            return letter;
        }

        function addMappingRow(col = "", header = "", field = "fullName", customText = "") {
            const nextIdx = currentFieldMappings.length;
            const colLetter = col || getColumnLetter(nextIdx);
            currentFieldMappings.push({
                column: colLetter,
                header: header || `Cột ${colLetter}`,
                field: field,
                custom_text: customText
            });
            renderMappingsTable();
        }

        function removeMappingRow(index) {
            currentFieldMappings.splice(index, 1);
            // Cập nhật lại ký hiệu cột
            currentFieldMappings.forEach((m, idx) => {
                m.column = getColumnLetter(idx);
            });
            renderMappingsTable();
        }

        function updateMappingField(index, key, value) {
            if (currentFieldMappings[index]) {
                currentFieldMappings[index][key] = value;
                if (key === 'field') {
                    const customInput = document.getElementById(`gsCustomText_${index}`);
                    if (customInput) {
                        customInput.disabled = (value !== 'custom_text');
                        if (value !== 'custom_text') customInput.value = "";
                    }
                }
            }
        }

        function renderMappingsTable() {
            const tbody = document.getElementById('gsMappingsTableBody');
            if (!tbody) return;

            tbody.innerHTML = currentFieldMappings.map((m, idx) => {
                const optionsHtml = availableLeadFields.map(f => {
                    const sel = (f.key === m.field) ? 'selected' : '';
                    return `<option value="${f.key}" ${sel}>${f.label}</option>`;
                }).join('');

                const isCustom = (m.field === 'custom_text');

                return `
                    <tr class="hover:bg-slate-900/50">
                        <td class="py-2 px-3 text-center font-mono font-bold text-emerald-400">
                            <input type="text" value="${escapeHTML(m.column || getColumnLetter(idx))}" onchange="updateMappingField(${idx}, 'column', this.value)" class="w-10 text-center bg-slate-900 border border-slate-700 rounded px-1 py-1 font-mono text-emerald-400 text-xs">
                        </td>
                        <td class="py-2 px-3">
                            <input type="text" value="${escapeHTML(m.header || '')}" onchange="updateMappingField(${idx}, 'header', this.value)" placeholder="Tên tiêu đề cột" class="w-full bg-slate-900 border border-slate-700 rounded-lg px-2.5 py-1 text-white text-xs font-semibold">
                        </td>
                        <td class="py-2 px-3">
                            <select onchange="updateMappingField(${idx}, 'field', this.value)" class="w-full bg-slate-900 border border-slate-700 rounded-lg px-2.5 py-1 text-slate-200 text-xs cursor-pointer">
                                ${optionsHtml}
                            </select>
                        </td>
                        <td class="py-2 px-3">
                            <input type="text" id="gsCustomText_${idx}" value="${escapeHTML(m.custom_text || '')}" ${isCustom ? '' : 'disabled'} onchange="updateMappingField(${idx}, 'custom_text', this.value)" placeholder="${isCustom ? 'Nhập văn bản cố định...' : '(Chỉ dùng cho Văn bản cố định)'}" class="w-full bg-slate-900 border border-slate-700 rounded-lg px-2.5 py-1 text-xs text-white disabled:opacity-30">
                        </td>
                        <td class="py-2 px-3 text-center">
                            <button type="button" onclick="removeMappingRow(${idx})" class="text-rose-400 hover:text-rose-300 p-1" title="Xóa cột này">
                                <i class="fa-solid fa-trash-can"></i>
                            </button>
                        </td>
                    </tr>
                `;
            }).join('');
        }

        async function testAppendRow() {
            const creds = document.getElementById('gsCredentialsJson').value.trim();
            const sheetInput = document.getElementById('gsSpreadsheetInput').value.trim();
            const tabName = document.getElementById('gsTabSelect').value;

            if (!sheetInput || !tabName) {
                alert("Vui lòng dán link Google Sheet và bấm 'Kết Nối & Tải Cấu Trúc' để chọn Tab trước khi gửi dòng thử nghiệm.");
                return;
            }

            const resultBox = document.getElementById('gsTestResultBox');
            const btn = document.getElementById('btnTestAppend');

            btn.disabled = true;
            btn.innerHTML = `<i class="fa-solid fa-spinner fa-spin"></i> <span>Đang ghi dòng test...</span>`;
            resultBox.className = 'p-3 rounded-xl border border-sky-800 bg-sky-950/60 text-sky-300 text-xs font-mono';
            resultBox.innerHTML = `Đang kết nối tới Google Sheets API và ghi dữ liệu mẫu...`;

            try {
                const res = await fetch('/api/integrations/sheets/test', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        credentials_json: creds,
                        spreadsheet_id: sheetInput,
                        tab_name: tabName,
                        field_mappings: currentFieldMappings
                    })
                });

                const json = await res.json();
                if (json.success) {
                    resultBox.className = 'p-3 rounded-xl border border-emerald-800 bg-emerald-950/80 text-emerald-300 text-xs font-mono space-y-1';
                    resultBox.innerHTML = `
                        <div class="font-bold text-white flex items-center gap-1.5">
                            <i class="fa-solid fa-circle-check text-emerald-400"></i>
                            <span>${json.message}</span>
                        </div>
                        <div>Dòng cập nhật trên Sheet: <b class="text-emerald-400">${json.updated_range || 'OK'}</b></div>
                        <div class="text-[11px] text-slate-400">Hãy mở Google Sheet để xem dòng dữ liệu vừa được chèn vào!</div>
                    `;
                } else {
                    resultBox.className = 'p-3 rounded-xl border border-rose-800 bg-rose-950/80 text-rose-300 text-xs font-mono space-y-1';
                    resultBox.innerHTML = `
                        <div class="font-bold flex items-center gap-1.5">
                            <i class="fa-solid fa-circle-xmark text-rose-400"></i>
                            <span>Lỗi ghi dữ liệu:</span>
                        </div>
                        <div class="text-[11px] leading-relaxed whitespace-pre-wrap">${json.error}</div>
                    `;
                }
            } catch(err) {
                resultBox.className = 'p-3 rounded-xl border border-rose-800 bg-rose-950/80 text-rose-300 text-xs font-mono';
                resultBox.innerText = "Lỗi kết nối: " + err.message;
            } finally {
                btn.disabled = false;
                btn.innerHTML = `<i class="fa-solid fa-paper-plane"></i> <span>Gửi Dòng Thử Nghiệm Ngay</span>`;
            }
        }

        async function saveGoogleSheetConfig() {
            const configId = parseInt(document.getElementById('gsConfigId').value) || 0;
            const name = document.getElementById('gsConfigName').value.trim();
            const creds = document.getElementById('gsCredentialsJson').value.trim();
            const sheetInput = document.getElementById('gsSpreadsheetInput').value.trim();
            const tabName = document.getElementById('gsTabSelect').value;
            const sheetTitle = document.getElementById('gsSpreadsheetTitleDisplay').innerText;
            const filterLanding = document.getElementById('gsFilterLandingId').value.trim();
            const filterNguon = document.getElementById('gsFilterNguon').value.trim();
            const autoHeader = document.getElementById('gsAutoHeader').checked;

            if (!name) {
                alert("Vui lòng nhập Tên Cấu Hình Kết Nối.");
                document.getElementById('gsConfigName').focus();
                return;
            }
            if (!sheetInput) {
                alert("Vui lòng nhập Link hoặc ID của Google Sheet.");
                document.getElementById('gsSpreadsheetInput').focus();
                return;
            }
            if (!tabName) {
                alert("Vui lòng bấm 'Kết Nối & Tải Cấu Trúc' và chọn một tab trang tính trước khi lưu.");
                return;
            }

            const payload = {
                id: configId,
                name: name,
                auth_type: "service_account",
                credentials_json: creds,
                spreadsheet_id: sheetInput,
                spreadsheet_name: (sheetTitle && sheetTitle !== '--') ? sheetTitle : name,
                sheet_tab_name: tabName,
                field_mappings: currentFieldMappings,
                filter_landing_id: filterLanding,
                filter_nguon: filterNguon,
                is_active: true,
                auto_create_header: autoHeader
            };

            const btn = document.getElementById('btnSaveConfig');
            btn.disabled = true;
            btn.innerHTML = `<i class="fa-solid fa-spinner fa-spin"></i> <span>Đang lưu...</span>`;

            try {
                const res = await fetch('/api/integrations/sheets/configs', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(payload)
                });
                const json = await res.json();
                if (!json.success) {
                    alert("Lỗi lưu cấu hình: " + json.error);
                    return;
                }

                closeGoogleSheetModal();
                loadGoogleSheetsConfigs();
                alert(json.message || "Lưu cấu hình kết nối Google Sheet thành công!");
            } catch(err) {
                alert("Lỗi kết nối lưu cấu hình: " + err.message);
            } finally {
                btn.disabled = false;
                btn.innerHTML = `<i class="fa-solid fa-floppy-disk"></i> <span>Lưu Cấu Hình Kết Nối</span>`;
            }
        }

        async function toggleGoogleSheetConfig(id, isActive) {
            try {
                const res = await fetch('/api/integrations/sheets/configs/toggle', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ id: id, is_active: isActive })
                });
                const json = await res.json();
                if (json.success) {
                    loadGoogleSheetsConfigs();
                } else {
                    alert("Lỗi cập nhật trạng thái: " + json.error);
                }
            } catch(e) {
                alert("Lỗi kết nối: " + e.message);
            }
        }

        async function deleteGoogleSheetConfig(id, name) {
            if (!confirm(`Bạn có chắc chắn muốn xóa cấu hình Google Sheet "${name}" không?\nToàn bộ lịch sử sync log của sheet này cũng sẽ được dọn dẹp.`)) {
                return;
            }

            try {
                const res = await fetch('/api/integrations/sheets/configs/delete', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ id: id })
                });
                const json = await res.json();
                if (json.success) {
                    loadGoogleSheetsConfigs();
                } else {
                    alert("Lỗi xóa cấu hình: " + json.error);
                }
            } catch(e) {
                alert("Lỗi kết nối: " + e.message);
            }
        }

        async function testConfigRow(id) {
            try {
                const res = await fetch(`/api/integrations/sheets/configs/get?id=${id}`);
                const json = await res.json();
                if (!json.success || !json.config) {
                    alert("Không thể đọc cấu hình: " + (json.error || ""));
                    return;
                }

                const c = json.config;
                const testRes = await fetch('/api/integrations/sheets/test', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        credentials_json: c.credentials_json,
                        spreadsheet_id: c.spreadsheet_id,
                        tab_name: c.sheet_tab_name,
                        field_mappings: c.field_mappings
                    })
                });

                const testJson = await testRes.json();
                if (testJson.success) {
                    alert(`Gửi dòng test thành công vào Sheet "${c.name}"!\nDòng đã ghi: ${testJson.updated_range}\n\nHãy mở Google Sheet để kiểm tra dữ liệu.`);
                    loadGoogleSheetsConfigs();
                } else {
                    alert(`Lỗi khi gửi dòng test:\n${testJson.error}`);
                }
            } catch(e) {
                alert("Lỗi kết nối test dòng: " + e.message);
            }
        }

        // ==============================================================
        // SYNC AUDIT LOGS MODAL CONTROLLER
        // ==============================================================
        function openSyncLogsModal(configId = 0) {
            const select = document.getElementById('logFilterConfigSelect');
            if (select) select.value = configId || "0";
            document.getElementById('syncLogsModal').classList.remove('hidden');
            loadSyncLogs();
        }

        function closeSyncLogsModal() {
            document.getElementById('syncLogsModal').classList.add('hidden');
        }

        async function loadSyncLogs() {
            const select = document.getElementById('logFilterConfigSelect');
            const configId = select ? parseInt(select.value) || 0 : 0;
            const tbody = document.getElementById('syncLogsTableBody');
            tbody.innerHTML = `<tr><td colspan="8" class="text-center py-8 text-slate-500"><i class="fa-solid fa-spinner fa-spin mr-2"></i>Đang tải nhật ký...</td></tr>`;

            try {
                const url = `/api/integrations/sheets/logs?limit=50&config_id=${configId}`;
                const res = await fetch(url);
                const json = await res.json();
                if (!json.success) {
                    tbody.innerHTML = `<tr><td colspan="8" class="text-center py-6 text-rose-400">Lỗi tải nhật ký: ${json.error}</td></tr>`;
                    return;
                }

                const logs = json.logs || [];
                document.getElementById('syncLogsTotalCount').innerText = `Tổng số: ${json.total || 0} lượt đồng bộ (Hiển thị ${logs.length})`;

                if (logs.length === 0) {
                    tbody.innerHTML = `<tr><td colspan="8" class="text-center py-8 text-slate-500">Chưa có lượt đồng bộ nào được ghi nhận.</td></tr>`;
                    return;
                }

                tbody.innerHTML = logs.map(l => {
                    const isSuccess = (l.status === 'success');
                    return `
                        <tr class="hover:bg-slate-900/60 transition">
                            <td class="py-2.5 px-3 text-center font-mono text-slate-500">#${l.id}</td>
                            <td class="py-2.5 px-3 font-mono text-slate-300">${escapeHTML(l.synced_at)}</td>
                            <td class="py-2.5 px-3 font-semibold text-white">${escapeHTML(l.config_name || 'Sheet #' + l.config_id)}</td>
                            <td class="py-2.5 px-3 text-center">
                                <span class="text-[10px] px-2 py-0.5 rounded-full font-bold ${isSuccess ? 'bg-emerald-950 text-emerald-400 border border-emerald-800' : 'bg-rose-950 text-rose-400 border border-rose-800'}">
                                    ${isSuccess ? 'THÀNH CÔNG' : 'THẤT BẠI'}
                                </span>
                            </td>
                            <td class="py-2.5 px-3 text-slate-200">${escapeHTML(l.lead_name || 'Khách hàng')}</td>
                            <td class="py-2.5 px-3 font-mono text-emerald-400 font-bold">${escapeHTML(l.lead_phone)}</td>
                            <td class="py-2.5 px-3 font-mono text-slate-300">${escapeHTML(l.row_range || '--')}</td>
                            <td class="py-2.5 px-3 text-slate-400 max-w-xs truncate" title="${escapeHTML(l.error_message)}">
                                ${isSuccess ? '<span class="text-slate-500 text-[11px]">Đã ghi nhận vào Sheet</span>' : `<span class="text-rose-400 text-[11px]"><i class="fa-solid fa-triangle-exclamation mr-1"></i>${escapeHTML(l.error_message)}</span>`}
                            </td>
                        </tr>
                    `;
                }).join('');
            } catch(e) {
                tbody.innerHTML = `<tr><td colspan="8" class="text-center py-6 text-rose-400">Lỗi kết nối: ${e.message}</td></tr>`;
            }
        }

