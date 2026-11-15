/* ==============================================================
   APEX ADMIN - LEAD STATUS CATALOG (Pipeline trạng thái khách hàng)
   ==============================================================

   NGUYÊN TẮC:
   Danh sách trạng thái được lấy TỪ BACKEND (`/api/facebook-ads/lead-statuses`)
   để không phải hard-code ở 2 nơi (Go + JS). Nếu API lỗi → dùng fallback
   cục bộ (APEX_STATUS_FALLBACK) để CRM vẫn dùng được.

   Pipeline (ngành "Giải Mã Đầu Tư Hàng Hóa" — khách mua 1 lần là đủ):

     1. 📥 Mới tiếp nhận            (moi)
     2. 📞 Đã gọi điện              (da_goi)
     3. 💬 Đã kết bạn Zalo          (ket_ban_zalo)
     4. 💻 Đã học buổi chia sẻ       (da_hoc_buoi_chia_se)   ← miễn phí
     5. 📘 Đã học chuyên sâu         (da_hoc_chuyen_sau)     ← ≈6.000.000đ
     6. 🏆 Khách hàng đã mua hàng    (da_mua_hang)           ← chuyển đổi cuối
     7. 🔁 Đã học trước đó rồi       (da_hoc_truoc_do)
     8. 🙅 Không quan tâm            (khong_quan_tam)
     9. 🚫 Spam / Không liên lạc được (spam_khong_lien_lac)
    10. ❌ Hủy / Sai số              (huy)

   Tương thích ngược: 'tiem_nang' → 'da_hoc_buoi_chia_se',
   'chot' → 'da_mua_hang', 'da_vao_zoom' → 'da_hoc_buoi_chia_se'.
   ============================================================== */

const APEX_STATUS_FALLBACK = [
    { key: 'moi', label: 'Mới tiếp nhận', short: 'Mới', icon: '📥', stage: 'new', color: 'sky', badge: 'bg-sky-950 text-sky-400 border-sky-800', has_value: false, capi_event: 'Lead' },
    { key: 'da_goi', label: 'Đã gọi điện', short: 'Đã gọi', icon: '📞', stage: 'contacted', color: 'amber', badge: 'bg-amber-950 text-amber-400 border-amber-800', has_value: false, capi_event: 'Contact' },
    { key: 'ket_ban_zalo', label: 'Đã kết bạn Zalo', short: 'Zalo', icon: '💬', stage: 'contacted', color: 'blue', badge: 'bg-blue-950 text-blue-400 border-blue-800', has_value: false, capi_event: '' },
    { key: 'da_hoc_buoi_chia_se', label: 'Đã học buổi chia sẻ', short: 'Chia sẻ', icon: '💻', stage: 'free', color: 'purple', badge: 'bg-purple-950 text-purple-400 border-purple-800', has_value: false, capi_event: 'StartTrial' },
    { key: 'da_hoc_chuyen_sau', label: 'Đã học chuyên sâu', short: 'Chuyên sâu', icon: '📘', stage: 'paid', color: 'indigo', badge: 'bg-indigo-950 text-indigo-400 border-indigo-800', has_value: true, capi_event: 'Subscribe', capi_has_value: true },
    { key: 'da_mua_hang', label: 'Khách hàng đã mua hàng', short: 'Đã mua', icon: '🏆', stage: 'won', color: 'emerald', badge: 'bg-emerald-950 text-emerald-400 border-emerald-800', has_value: true, capi_event: 'Purchase', capi_has_value: true },
    { key: 'da_hoc_truoc_do', label: 'Đã học trước đó rồi', short: 'Học trước', icon: '🔁', stage: 'lost', color: 'slate', badge: 'bg-slate-950 text-slate-400 border-slate-800', has_value: false, capi_event: '' },
    { key: 'khong_quan_tam', label: 'Không quan tâm', short: 'Không QT', icon: '🙅', stage: 'lost', color: 'orange', badge: 'bg-orange-950 text-orange-400 border-orange-800', has_value: false, capi_event: '' },
    { key: 'spam_khong_lien_lac', label: 'Spam / Không liên lạc được', short: 'Spam', icon: '🚫', stage: 'lost', color: 'rose', badge: 'bg-rose-950 text-rose-400 border-rose-800', has_value: false, capi_event: '' },
    { key: 'huy', label: 'Hủy / Sai số', short: 'Hủy', icon: '❌', stage: 'lost', color: 'zinc', badge: 'bg-zinc-950 text-zinc-400 border-zinc-800', has_value: false, capi_event: '' },
];

// Danh sách trạng thái đang dùng (sẽ được thay bằng dữ liệu từ backend).
let APEX_LEAD_STATUSES = APEX_STATUS_FALLBACK.slice();
let APEX_STATUS_MAP = {};
let APEX_STATUS_LOADED = false;

// Ánh xạ giá trị cũ → giá trị mới (giống hệt NormalizeLeadStatus ở Go).
const APEX_STATUS_LEGACY_MAP = {
    'tiem_nang': 'da_hoc_buoi_chia_se',
    'chot': 'da_mua_hang',
    'da_vao_zoom': 'da_hoc_buoi_chia_se',
};

/**
 * normalizeStatus chuẩn hoá trạng thái (hợp nhất giá trị cũ).
 */
function normalizeStatus(raw) {
    const s = (raw || '').toString().trim();
    if (!s) return 'moi';
    if (APEX_STATUS_LEGACY_MAP[s]) return APEX_STATUS_LEGACY_MAP[s];
    if (APEX_STATUS_MAP[s]) return s;
    return 'moi';
}

/**
 * getStatusInfo trả về định nghĩa trạng thái (luôn có giá trị).
 */
function getStatusInfo(key) {
    const k = normalizeStatus(key);
    return APEX_STATUS_MAP[k] || APEX_STATUS_MAP['moi'] || APEX_STATUS_FALLBACK[0];
}

/**
 * getStatusBadge trả về class badge Tailwind của trạng thái.
 */
function getStatusBadge(key) {
    return getStatusInfo(key).badge;
}

/**
 * renderStatusOptions dựng <option> cho thẻ select.
 *
 * @param {string} current  - trạng thái đang chọn (để đánh dấu selected)
 * @param {boolean} includeEmpty - có thêm option "Tất cả" ở đầu không
 */
function renderStatusOptions(current, includeEmpty) {
    const cur = normalizeStatus(current);
    let html = '';
    if (includeEmpty) {
        html += '<option value="">Tất cả Trạng thái CRM</option>';
    }
    APEX_LEAD_STATUSES.forEach((s, idx) => {
        const sel = s.key === cur ? ' selected' : '';
        html += `<option value="${s.key}"${sel}>${s.icon} ${s.label}</option>`;
    });
    return html;
}

/**
 * loadLeadStatuses nạp danh mục trạng thái từ backend (1 lần duy nhất).
 */
async function loadLeadStatuses() {
    if (APEX_STATUS_LOADED) return APEX_LEAD_STATUSES;
    try {
        const res = await fetch('/api/facebook-ads/lead-statuses');
        if (res.ok) {
            const json = await res.json();
            if (json && json.success && Array.isArray(json.statuses) && json.statuses.length > 0) {
                APEX_LEAD_STATUSES = json.statuses.map(s => ({
                    key: s.key,
                    label: s.label,
                    short: s.short || s.label,
                    icon: s.icon || '',
                    stage: s.stage || 'lost',
                    color: s.color || 'slate',
                    badge: s.badge || 'bg-slate-950 text-slate-400 border-slate-800',
                    has_value: !!s.has_value,
                    capi_event: s.capi_event || '',
                    capi_has_value: !!s.capi_has_value,
                }));
            }
        }
    } catch (e) {
        console.warn('[APEX] Không tải được danh mục trạng thái, dùng fallback:', e);
    }
    APEX_STATUS_MAP = {};
    APEX_LEAD_STATUSES.forEach(s => { APEX_STATUS_MAP[s.key] = s; });
    APEX_STATUS_LOADED = true;
    applyStatusCatalogToUI();
    return APEX_LEAD_STATUSES;
}

/**
 * applyStatusCatalogToUI đồng bộ các UI đang hard-code danh sách trạng thái:
 *  - Filter trạng thái trên thanh công cụ CRM
 *  - Kanban board (dựng lại 10 cột)
 *  - Biểu đồ tròn phân bổ trạng thái
 */
function applyStatusCatalogToUI() {
    // 1. Filter trạng thái
    const filterSel = document.getElementById('filterStatus');
    if (filterSel) {
        const cur = filterSel.value;
        filterSel.innerHTML = renderStatusOptions(cur, true);
    }

    // 2. Select trạng thái trong hộp chi tiết lead (Customer 360 drawer)
    const drawerSel = document.getElementById('drawerStatusSelect');
    if (drawerSel) {
        const cur = drawerSel.value;
        drawerSel.innerHTML = renderStatusOptions(cur, false);
        if (cur) drawerSel.value = cur;
    }

    // 3. Kanban: dựng lại các cột theo danh mục mới
    const kanbanHost = document.getElementById('kanbanColumns');
    if (kanbanHost) {
        kanbanHost.innerHTML = APEX_LEAD_STATUSES.map((s, idx) => `
            <div class="w-64 bg-slate-900 border border-slate-800 rounded-2xl flex flex-col flex-shrink-0 shadow-lg">
                <div class="p-3 bg-slate-850 border-b border-slate-800 rounded-t-2xl flex items-center justify-between">
                    <div class="flex items-center space-x-2 min-w-0">
                        <span class="w-2.5 h-2.5 rounded-full bg-${s.color}-400 flex-shrink-0"></span>
                        <h3 class="font-bold text-xs text-white uppercase tracking-wider truncate">${idx + 1}. ${s.label}</h3>
                    </div>
                    <span id="kanbanCount_${s.key}" class="${s.badge} text-xs px-2 py-0.5 rounded-full font-mono font-bold">0</span>
                </div>
                <div id="kanbanList_${s.key}" class="kanban-col-scroll p-2.5 space-y-2.5 flex-1"></div>
            </div>
        `).join('');
    }

    // 4. Biểu đồ tròn (nếu đang hiển thị)
    if (window.__apexStatusCounts) {
        if (typeof renderStatusChart === 'function') {
            renderStatusChart(window.__apexStatusCounts);
        }
    }
}
