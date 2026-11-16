/* ==============================================================
   APEX ADMIN - FACEBOOK ADS (Meta Conversions API + Dataset Quality)
   ==============================================================

   Chức năng:
     1. Cấu hình CAPI theo landing (pixel, token, gateway, giá trị khoá học)
     2. Gửi Test Event (kiểm tra kết nối tới Meta)
     3. Đọc Dataset Quality API (EMQ, coverage, dedupe, freshness, delay, value)
     4. Nhật ký gửi sự kiện + thống kê tổng hợp
     5. Báo cáo hiệu quả theo chiến dịch / adset / ad (QC)
     6. Phân tích URL quảng cáo Meta (kiểm tra cấu hình URL Parameters)
   ============================================================== */

let fbAdsConfigs = [];
let fbAdsActiveLanding = '';
let fbAdsLogs = [];
let fbAdsCampaigns = [];
/** Danh sách landing lấy từ /api/landings — dùng cho dropdown bật/tắt CAPI theo từng landing. */
let fbAdsLandings = [];

const FB_ADS_STATUSES = [
    { key: 'success', label: 'Thành công', cls: 'bg-emerald-950 text-emerald-400 border-emerald-800' },
    { key: 'failed', label: 'Thất bại', cls: 'bg-rose-950 text-rose-400 border-rose-800' },
    { key: 'skipped', label: 'Bỏ qua', cls: 'bg-slate-950 text-slate-400 border-slate-800' },
];

/**
 * escapeHTMLFb - escape chuỗi trước khi chèn vào DOM.
 */
function escapeHTMLFb(v) {
    return String(v == null ? '' : v)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#039;');
}

/**
 * fbAdsToast - hiện thông báo nhỏ trong tab.
 */
function fbAdsToast(msg, type = 'info') {
    const el = document.getElementById('fbAdsToast');
    if (!el) return;
    const cls = type === 'error'
        ? 'bg-rose-950/95 border-rose-700 text-rose-200'
        : type === 'success'
            ? 'bg-emerald-950/95 border-emerald-700 text-emerald-200'
            : 'bg-slate-900/95 border-slate-700 text-slate-200';
    el.className = `mb-3 px-4 py-2.5 rounded-xl border text-xs font-semibold ${cls}`;
    el.innerText = msg;
    el.classList.remove('hidden');
    clearTimeout(window.__fbAdsToastTimer);
    window.__fbAdsToastTimer = setTimeout(() => el.classList.add('hidden'), 6000);
}

/**
 * fbAdsApi - wrapper fetch cho API Facebook Ads.
 */
async function fbAdsApi(url, options) {
    const res = await fetch(url, options);
    let json = {};
    try { json = await res.json(); } catch (_) {}
    if (!res.ok) {
        throw new Error(json.error || ('HTTP ' + res.status));
    }
    return json;
}

// ══════════════════════════════════════════════════════════════
// 1. LOAD & RENDER
// ══════════════════════════════════════════════════════════════

/**
 * loadFacebookAdsConfigs nạp danh sách cấu hình CAPI.
 */
async function loadFacebookAdsConfigs() {
    try {
        const json = await fbAdsApi('/api/facebook-ads/configs');
        fbAdsConfigs = json.configs || [];
        renderFacebookAdsConfigs();
        await loadFacebookAdsLandings();
        renderFacebookAdsLandingOptions();
    } catch (e) {
        fbAdsToast('Lỗi tải cấu hình CAPI: ' + e.message, 'error');
    }
}

/**
 * loadFacebookAdsLandings nạp danh sách landing từ /api/landings.
 *
 * Mục đích: cho Admin bật/tắt CAPI ngay trên từng landing, kể cả landing
 * chưa có cấu hình CAPI riêng (hiển thị trạng thái "chưa cấu hình").
 */
async function loadFacebookAdsLandings() {
    try {
        const res = await fetch('/api/landings');
        if (!res.ok) throw new Error('HTTP ' + res.status);
        const data = await res.json();
        fbAdsLandings = data.landings || [];
    } catch (e) {
        fbAdsLandings = [];
    }
}

/**
 * fbAdsFindConfig trả cấu hình CAPI (theo landing_id) đã có trong DB.
 *
 * ⚠️ Chỉ khớp CHÍNH XÁC landing_id — không dùng cấu hình mặc định
 * (landing_id = '') làm "đại diện" cho landing khác, vì mặc định áp dụng
 * cho mọi landing chưa có cấu hình riêng.
 */
function fbAdsFindConfig(landingId) {
    return fbAdsConfigs.find(c => (c.landing_id || '') === (landingId || '')) || null;
}

/**
 * renderFacebookAdsLandingOptions dựng danh sách landing cho các dropdown.
 *
 * Gom cả: landing đã có cấu hình CAPI + landing đang tồn tại trong hệ thống
 * (để Admin có thể tạo cấu hình mới cho landing chưa có).
 */
function renderFacebookAdsLandingOptions() {
    // Giữ lựa chọn hiện tại của từng dropdown (render lại không làm mất
    // lựa chọn đang dùng của Admin).
    const prev = {};
    ['fbAdsLandingSelect', 'fbAdsLandingFilter', 'fbAdsFormLanding'].forEach(selId => {
        const sel = document.getElementById(selId);
        prev[selId] = sel ? sel.value : undefined;
    });

    const cur = fbAdsActiveLanding;
    // 0) Luôn có lựa chọn "Mặc định" (landing_id = '') — fallback cho mọi
    //    landing chưa có cấu hình riêng.
    const ids = [''];

    // 1) Landing đã có cấu hình CAPI
    fbAdsConfigs.forEach(c => { if (!ids.includes(c.landing_id || '')) ids.push(c.landing_id || ''); });
    // 2) Landing đang tồn tại trong hệ thống
    fbAdsLandings.forEach(l => {
        const id = l.page_id || '';
        if (id && !ids.includes(id)) ids.push(id);
    });

    const opts = ids.map(id => {
        const l = fbAdsLandings.find(x => (x.page_id || '') === id);
        const label = id === ''
            ? 'Mặc định (áp dụng mọi landing)'
            : (l && l.name ? id + ' — ' + l.name : id);
        const cfg = fbAdsFindConfig(id);
        const badge = cfg ? (cfg.is_active ? ' ●' : ' ○') : '';
        return `<option value="${escapeHTMLFb(id)}"${id === cur ? ' selected' : ''}>${escapeHTMLFb(label + badge)}</option>`;
    }).join('');

    ['fbAdsLandingSelect', 'fbAdsLandingFilter', 'fbAdsFormLanding'].forEach(selId => {
        const sel = document.getElementById(selId);
        if (!sel) return;
        sel.innerHTML = (opts || '<option value="">Chưa có cấu hình nào</option>');
        // Khôi phục lựa chọn cũ nếu option đó vẫn còn.
        const keep = prev[selId];
        if (keep !== undefined && Array.from(sel.options).some(o => o.value === keep)) {
            sel.value = keep;
        }
    });
}

/**
 * renderFacebookAdsConfigs vẽ bảng cấu hình.
 */
function renderFacebookAdsConfigs() {
    const tb = document.getElementById('fbAdsConfigTableBody');
    if (!tb) return;

    if (!fbAdsConfigs.length) {
        tb.innerHTML = `<tr><td colspan="8" class="py-8 text-center text-slate-500 text-xs">
            Chưa có cấu hình nào. Bấm "Tạo cấu hình mặc định" để bắt đầu.
        </td></tr>`;
        return;
    }

    tb.innerHTML = fbAdsConfigs.map(c => {
        const on = !!c.is_active;
        const active = on
            ? '<span class="text-[10px] px-2 py-0.5 rounded-full font-mono font-bold bg-emerald-950 text-emerald-400 border border-emerald-800">ĐANG BẬT</span>'
            : '<span class="text-[10px] px-2 py-0.5 rounded-full font-mono font-bold bg-slate-950 text-slate-500 border border-slate-800">TẮT</span>';
        const gw = c.gateway_enabled
            ? '<span class="text-[10px] px-1.5 py-0.5 rounded bg-indigo-950 text-indigo-400 border border-indigo-800">GATEWAY</span>'
            : '';
        const landingLabel = c.landing_id === '' ? 'Mặc định' : c.landing_id;
        const events = [
            c.send_page_view ? 'PV' : '',
            c.send_lead ? 'Lead' : '',
            c.send_purchase ? 'Purchase' : '',
            c.send_crm_sync ? 'CRM' : '',
        ].filter(Boolean).join(' · ');

        // Nút bật/tắt nhanh — hiển thị cho CẢ dòng mặc định (landing_id = '')
        // lẫn dòng cấu hình riêng.
        //
        // ⚠️ Dòng mặc định áp dụng cho MỌI landing chưa có cấu hình riêng ⇒
        // tắt nó sẽ ngừng gửi CAPI cho các landing đó. Vì vậy hàm
        // fbAdsToggleConfig sẽ hỏi lại xác nhận trước khi tắt (fix 2026-11-16).
        const toggleBtn = `<button onclick="fbAdsToggleConfig('${escapeHTMLFb(c.landing_id)}', ${!on})"
                    title="${on ? 'Tắt CAPI cho landing này' : 'Bật CAPI cho landing này'}"
                    class="text-[10px] px-2 py-1 rounded ${on ? 'bg-emerald-950 hover:bg-emerald-900 text-emerald-400 border border-emerald-900' : 'bg-slate-800 hover:bg-slate-700 text-slate-300'} transition">
                    <i class="fa-solid fa-power-off"></i> ${on ? 'Tắt' : 'Bật'}
                </button>`;

        return `
            <tr class="hover:bg-slate-800/40 transition">
                <td class="py-2.5 px-3 font-semibold text-slate-200">${escapeHTMLFb(landingLabel)}</td>
                <td class="py-2.5 px-3 font-mono text-[11px] text-orange-400">${escapeHTMLFb(c.pixel_id || '-')}</td>
                <td class="py-2.5 px-3 font-mono text-[11px] text-slate-400">${escapeHTMLFb(c.access_token || '-')}</td>
                <td class="py-2.5 px-3">${gw}</td>
                <td class="py-2.5 px-3 font-mono text-[11px] text-slate-400">${escapeHTMLFb(c.api_version || 'v27.0')}</td>
                <td class="py-2.5 px-3 font-mono text-[11px] text-emerald-400">${Number(c.course_value || 0).toLocaleString('vi-VN')} ${escapeHTMLFb(c.currency || 'VND')}</td>
                <td class="py-2.5 px-3 text-[11px] text-slate-400">${escapeHTMLFb(events || '-')}</td>
                <td class="py-2.5 px-3 text-center">
                    ${active}
                    <div class="mt-1.5 flex items-center justify-center gap-1.5">
                        ${toggleBtn}
                        <button onclick="fbAdsEditConfig('${escapeHTMLFb(c.landing_id)}')" class="text-[10px] px-2 py-1 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 transition">Sửa</button>
                        <button onclick="fbAdsDeleteConfig('${escapeHTMLFb(c.landing_id)}')" class="text-[10px] px-2 py-1 rounded bg-rose-950 hover:bg-rose-900 text-rose-400 border border-rose-900 transition">Xoá</button>
                    </div>
                </td>
            </tr>
        `;
    }).join('');
}

/**
 * fbAdsToggleConfig bật/tắt nhanh CAPI cho 1 landing.
 *
 * ⚠️ Dòng MẶC ĐỊNH (landing_id = '') áp dụng cho mọi landing CHƯA có cấu
 * hình riêng ⇒ tắt nó sẽ ngừng gửi CAPI cho các landing đó. Hỏi xác nhận
 * trước khi tắt để tránh tắt nhầm toàn hệ thống (fix 2026-11-16).
 */
async function fbAdsToggleConfig(landingId, wantActive) {
    const isDefault = !landingId;
    if (!wantActive && isDefault) {
        const ok = confirm(
            'TẮT CAPI MẶC ĐỊNH?\n\n' +
            'Cấu hình mặc định áp dụng cho MỌI landing chưa có cấu hình riêng.\n' +
            'Tắt ⇒ các landing đó sẽ ngừng gửi sự kiện lên Meta.\n\n' +
            'Landing đã có cấu hình riêng vẫn hoạt động bình thường.'
        );
        if (!ok) return;
    }
    try {
        const json = await fbAdsApi('/api/facebook-ads/config/toggle', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ landing_id: landingId, is_active: !!wantActive }),
        });
        fbAdsToast(
            json.message ||
            ((wantActive ? 'Đã BẬT' : 'Đã TẮT') + ' CAPI cho ' + (isDefault ? 'cấu hình mặc định' : 'landing ' + landingId)),
            'success'
        );
        await loadFacebookAdsConfigs();
    } catch (e) {
        fbAdsToast('Lỗi bật/tắt: ' + e.message, 'error');
    }
}

/**
 * fbAdsEditConfig nạp cấu hình vào form chỉnh sửa.
 */
function fbAdsEditConfig(landingId) {
    const cfg = fbAdsConfigs.find(c => (c.landing_id || '') === (landingId || ''));
    if (!cfg) return;

    document.getElementById('fbAdsFormLanding').value = cfg.landing_id || '';
    document.getElementById('fbAdsFormPixel').value = cfg.pixel_id || '';
    document.getElementById('fbAdsFormToken').value = '';
    document.getElementById('fbAdsFormToken').placeholder = cfg.access_token
        ? 'Đã lưu: ' + cfg.access_token + ' (bỏ trống để giữ nguyên)'
        : 'Chưa có — dán access token của System User';
    document.getElementById('fbAdsFormVersion').value = cfg.api_version || 'v27.0';
    document.getElementById('fbAdsFormActive').checked = !!cfg.is_active;

    document.getElementById('fbAdsFormGatewayEnabled').checked = !!cfg.gateway_enabled;
    document.getElementById('fbAdsFormGatewayEndpoint').value = cfg.gateway_endpoint || '';
    document.getElementById('fbAdsFormGatewayKey').value = '';
    document.getElementById('fbAdsFormGatewayKey').placeholder = cfg.gateway_access_key
        ? 'Đã lưu: ' + cfg.gateway_access_key + ' (bỏ trống để giữ nguyên)'
        : 'Chưa có — dán access key của Gateway';

    document.getElementById('fbAdsFormSendPV').checked = !!cfg.send_page_view;
    document.getElementById('fbAdsFormSendLead').checked = !!cfg.send_lead;
    document.getElementById('fbAdsFormSendPurchase').checked = !!cfg.send_purchase;
    document.getElementById('fbAdsFormSendCRM').checked = !!cfg.send_crm_sync;
    document.getElementById('fbAdsFormOnlyFB').checked = cfg.only_facebook_traffic !== false;

    document.getElementById('fbAdsFormValue').value = cfg.course_value || 6000000;
    document.getElementById('fbAdsFormCurrency').value = cfg.currency || 'VND';

    document.getElementById('fbAdsFormHashEmail').checked = !!cfg.hash_email;
    document.getElementById('fbAdsFormHashPhone').checked = !!cfg.hash_phone;
    document.getElementById('fbAdsFormHashName').checked = !!cfg.hash_name;
    document.getElementById('fbAdsFormHashAddress').checked = !!cfg.hash_address;
    document.getElementById('fbAdsFormHashExternal').checked = !!cfg.hash_external_id;
    document.getElementById('fbAdsFormHashCity').checked = !!cfg.hash_city;
    document.getElementById('fbAdsFormHashState').checked = !!cfg.hash_state;
    document.getElementById('fbAdsFormHashZip').checked = !!cfg.hash_zip;
    document.getElementById('fbAdsFormHashCountry').checked = !!cfg.hash_country;
    document.getElementById('fbAdsFormSendUserData').checked = !!cfg.send_user_data;

    document.getElementById('fbAdsFormTitle').innerText = 'Chỉnh sửa cấu hình: ' + (cfg.landing_id || 'Mặc định');
    document.getElementById('fbAdsForm').scrollIntoView({ behavior: 'smooth', block: 'center' });
}

/**
 * fbAdsResetForm làm mới form về trạng thái tạo mới.
 */
function fbAdsResetForm() {
    document.getElementById('fbAdsFormLanding').value = '';
    document.getElementById('fbAdsFormPixel').value = '';
    document.getElementById('fbAdsFormToken').value = '';
    document.getElementById('fbAdsFormToken').placeholder = 'Dán access token của System User';
    document.getElementById('fbAdsFormVersion').value = 'v27.0';
    document.getElementById('fbAdsFormActive').checked = true;
    document.getElementById('fbAdsFormGatewayEnabled').checked = false;
    document.getElementById('fbAdsFormGatewayEndpoint').value = '';
    document.getElementById('fbAdsFormGatewayKey').value = '';
    document.getElementById('fbAdsFormSendPV').checked = true;
    document.getElementById('fbAdsFormSendLead').checked = true;
    document.getElementById('fbAdsFormSendPurchase').checked = true;
    document.getElementById('fbAdsFormSendCRM').checked = true;
    document.getElementById('fbAdsFormOnlyFB').checked = true;
    document.getElementById('fbAdsFormValue').value = 6000000;
    document.getElementById('fbAdsFormCurrency').value = 'VND';
    document.getElementById('fbAdsFormHashEmail').checked = true;
    document.getElementById('fbAdsFormHashPhone').checked = true;
    document.getElementById('fbAdsFormHashName').checked = true;
    document.getElementById('fbAdsFormHashAddress').checked = true;
    document.getElementById('fbAdsFormHashExternal').checked = true;
    document.getElementById('fbAdsFormHashCity').checked = true;
    document.getElementById('fbAdsFormHashState').checked = true;
    document.getElementById('fbAdsFormHashZip').checked = true;
    document.getElementById('fbAdsFormHashCountry').checked = true;
    document.getElementById('fbAdsFormSendUserData').checked = true;
    document.getElementById('fbAdsFormTitle').innerText = 'Tạo / Cập nhật cấu hình Conversions API';
}

/**
 * fbAdsSaveConfig lưu cấu hình.
 */
async function fbAdsSaveConfig() {
    const payload = {
        landing_id: document.getElementById('fbAdsFormLanding').value.trim(),
        is_active: document.getElementById('fbAdsFormActive').checked,
        pixel_id: document.getElementById('fbAdsFormPixel').value.trim(),
        access_token: document.getElementById('fbAdsFormToken').value.trim(),
        api_version: document.getElementById('fbAdsFormVersion').value.trim() || 'v27.0',
        gateway_enabled: document.getElementById('fbAdsFormGatewayEnabled').checked,
        gateway_endpoint: document.getElementById('fbAdsFormGatewayEndpoint').value.trim(),
        gateway_access_key: document.getElementById('fbAdsFormGatewayKey').value.trim(),
        send_page_view: document.getElementById('fbAdsFormSendPV').checked,
        send_lead: document.getElementById('fbAdsFormSendLead').checked,
        send_purchase: document.getElementById('fbAdsFormSendPurchase').checked,
        send_crm_sync: document.getElementById('fbAdsFormSendCRM').checked,
        only_facebook_traffic: document.getElementById('fbAdsFormOnlyFB').checked,
        course_value: parseFloat(document.getElementById('fbAdsFormValue').value) || 6000000,
        currency: document.getElementById('fbAdsFormCurrency').value.trim() || 'VND',
        hash_email: document.getElementById('fbAdsFormHashEmail').checked,
        hash_phone: document.getElementById('fbAdsFormHashPhone').checked,
        hash_name: document.getElementById('fbAdsFormHashName').checked,
        hash_address: document.getElementById('fbAdsFormHashAddress').checked,
        hash_external_id: document.getElementById('fbAdsFormHashExternal').checked,
        hash_city: document.getElementById('fbAdsFormHashCity').checked,
        hash_state: document.getElementById('fbAdsFormHashState').checked,
        hash_zip: document.getElementById('fbAdsFormHashZip').checked,
        hash_country: document.getElementById('fbAdsFormHashCountry').checked,
        send_user_data: document.getElementById('fbAdsFormSendUserData').checked,
    };

    try {
        await fbAdsApi('/api/facebook-ads/config', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
        });
        fbAdsToast('Đã lưu cấu hình Conversions API', 'success');
        await loadFacebookAdsConfigs();
        fbAdsResetForm();
    } catch (e) {
        fbAdsToast('Lỗi lưu cấu hình: ' + e.message, 'error');
    }
}

/**
 * fbAdsDeleteConfig xoá cấu hình.
 *
 * ⚠️ Dòng MẶC ĐỊNH (landing_id = '') cũng xoá được — backend khớp CHÍNH XÁC
 * `landing_id` nên không xoá nhầm cấu hình của landing khác (fix 2026-11-16).
 */
async function fbAdsDeleteConfig(landingId) {
    const isDefault = !landingId;
    const label = isDefault ? 'cấu hình CAPI MẶC ĐỊNH' : ('cấu hình CAPI của "' + landingId + '"');
    const warn = isDefault
        ? '\n\n⚠️ Cấu hình mặc định áp dụng cho MỌI landing chưa có cấu hình riêng.\n' +
          'Xoá ⇒ các landing đó sẽ không gửi sự kiện CAPI lên Meta nữa.'
        : '';
    if (!confirm(`Xoá ${label}?${warn}\n\nLưu ý: log gửi sự kiện vẫn được giữ lại để đối soát.`)) return;
    try {
        const json = await fbAdsApi('/api/facebook-ads/config/delete?landing_id=' + encodeURIComponent(landingId || ''), { method: 'POST' });
        fbAdsToast(json.message || 'Đã xoá cấu hình', 'success');
        await loadFacebookAdsConfigs();
    } catch (e) {
        fbAdsToast('Lỗi xoá: ' + e.message, 'error');
    }
}

// ══════════════════════════════════════════════════════════════
// 2. TEST EVENT
// ══════════════════════════════════════════════════════════════

/**
 * fbAdsSendTestEvent gửi 1 test event lên Meta.
 */
async function fbAdsSendTestEvent() {
    const landingId = document.getElementById('fbAdsLandingSelect').value;
    const payload = {
        landing_id: landingId,
        event_name: document.getElementById('fbAdsTestEventName').value || 'Lead',
        email: document.getElementById('fbAdsTestEmail').value.trim(),
        phone: document.getElementById('fbAdsTestPhone').value.trim(),
        first_name: document.getElementById('fbAdsTestName').value.trim(),
        value: parseFloat(document.getElementById('fbAdsTestValue').value) || 0,
        currency: document.getElementById('fbAdsTestCurrency').value.trim() || 'VND',
        event_source_url: document.getElementById('fbAdsTestURL').value.trim(),
        test_event_code: document.getElementById('fbAdsTestCode').value.trim(),
    };

    try {
        const json = await fbAdsApi('/api/facebook-ads/test-event', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
        });
        if (json.success) {
            fbAdsToast('✅ ' + (json.message || 'Đã gửi test event'), 'success');
        } else {
            fbAdsToast('❌ ' + (json.error || 'Meta từ chối'), 'error');
        }
        document.getElementById('fbAdsTestResult').innerHTML =
            '<pre class="text-[10px] text-slate-300 whitespace-pre-wrap break-all">' +
            escapeHTMLFb(JSON.stringify(json.response || json, null, 2)) + '</pre>';
        await loadFacebookAdsLogs();
    } catch (e) {
        fbAdsToast('❌ ' + e.message, 'error');
    }
}

// ══════════════════════════════════════════════════════════════
// 3. DATASET QUALITY
// ══════════════════════════════════════════════════════════════

/**
 * fbAdsLoadDatasetQuality đọc chỉ số chất lượng dữ liệu từ Meta.
 */
async function fbAdsLoadDatasetQuality() {
    const landingId = document.getElementById('fbAdsLandingSelect').value;
    const box = document.getElementById('fbAdsQualityBox');
    box.innerHTML = '<div class="py-6 text-center text-slate-500 text-xs"><i class="fa-solid fa-spinner fa-spin mr-2"></i>Đang gọi Meta Dataset Quality API...</div>';

    try {
        const json = await fbAdsApi('/api/facebook-ads/dataset-quality?landing_id=' + encodeURIComponent(landingId));
        if (!json.success) {
            box.innerHTML = `<div class="py-6 text-center text-rose-400 text-xs">${escapeHTMLFb(json.error || 'Không đọc được dữ liệu')}</div>`;
            return;
        }
        const q = json.quality || {};
        box.innerHTML = renderFacebookAdsQuality(q);
    } catch (e) {
        box.innerHTML = `<div class="py-6 text-center text-rose-400 text-xs">${escapeHTMLFb(e.message)}</div>`;
    }
}

/**
 * renderFacebookAdsQuality dựng HTML cho các chỉ số Dataset Quality.
 *
 * 📌 FORMAT META v27.0: `web` là MẢNG, mỗi phần tử = 1 `event_name` với
 * toàn bộ chỉ số riêng. Bản cũ đọc `web` như 1 object ⇒ mọi thẻ đều "—".
 *
 * @param {object} q  payload `quality` từ API
 *                     { pixel_id, fetched_at, events: [...], aggregate: {...} }
 */
function renderFacebookAdsQuality(q) {
    const events = q.events || [];
    const agg = q.aggregate || {};

    const card = (title, icon, color, body) => `
        <div class="bg-slate-950 border border-slate-800 rounded-xl p-3.5">
            <div class="flex items-center gap-1.5 mb-2">
                <i class="fa-solid ${icon} ${color} text-xs"></i>
                <span class="text-[11px] font-bold text-slate-200">${escapeHTMLFb(title)}</span>
            </div>
            ${body}
        </div>`;

    const emqColor = (v) => (v == null ? 'text-slate-300'
        : v >= 7 ? 'text-emerald-400' : v >= 5 ? 'text-amber-400' : 'text-rose-400');

    if (!events.length) {
        return `<div class="py-8 text-center text-slate-500 text-xs">
            <i class="fa-solid fa-chart-line mb-2 block text-slate-700"></i>
            Meta chưa trả về số liệu cho dataset này.
            <br><span class="text-slate-600">Meta cần ≥ 20 phút sau khi gửi sự kiện đầu tiên, và dữ liệu thường trễ vài giờ.</span>
        </div>`;
    }

    // ─── Thẻ tổng quan toàn dataset ────────────────────────────────
    const overview = `
        <div class="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
            ${card('EMQ trung bình', 'fa-bullseye', 'text-emerald-400', `
                <div class="text-center py-1">
                    <div class="text-2xl font-black font-mono ${emqColor(agg.avg_emq)}">${escapeHTMLFb(agg.avg_emq ? Number(agg.avg_emq).toFixed(1) : '—')}</div>
                    <div class="text-[9px] text-slate-500">điểm EMQ (0–10)</div>
                </div>`)}
            ${card('Event Coverage TB', 'fa-chart-simple', 'text-blue-400', `
                <div class="text-center py-1">
                    <div class="text-2xl font-black font-mono text-blue-400">${escapeHTMLFb(agg.avg_coverage ? Number(agg.avg_coverage).toFixed(1) + '%' : '—')}</div>
                    <div class="text-[9px] text-slate-500">mục tiêu 75%</div>
                </div>`)}
            ${card('Chi tiêu sự kiện', 'fa-money-bill-wave', 'text-amber-400', `
                <div class="text-center py-1">
                    <div class="text-2xl font-black font-mono text-amber-400">${escapeHTMLFb(agg.total_spend ? Number(agg.total_spend).toLocaleString('vi-VN') : '—')}</div>
                    <div class="text-[9px] text-slate-500">${escapeHTMLFb(agg.spend_currency_code || '')}</div>
                </div>`)}
            ${card('Sự kiện theo dõi', 'fa-layer-group', 'text-slate-300', `
                <div class="text-center py-1">
                    <div class="text-2xl font-black font-mono text-slate-200">${escapeHTMLFb(agg.event_count || 0)}</div>
                    <div class="text-[9px] text-slate-500">${escapeHTMLFb(agg.critical_issues || 0)} vấn đề</div>
                </div>`)}
        </div>`;

    // ─── Bảng chi tiết từng event ────────────────────────────────
    const rows = events.map(ev => {
        const emq = ev.event_match_quality || {};
        const cov = ev.event_coverage || {};
        const delay = ev.event_delay || {};
        const fresh = ev.data_freshness || {};
        const acr = ev.acr || {};
        const issueCount = (ev.value_diagnostics || []).length + (emq.diagnostics || []).length;

        return `
            <details class="bg-slate-950 border border-slate-800 rounded-xl overflow-hidden">
                <summary class="px-3.5 py-2.5 cursor-pointer hover:bg-slate-900/60 flex items-center gap-3 flex-wrap">
                    <span class="font-mono font-bold text-orange-400 text-xs">${escapeHTMLFb(ev.event_name || '—')}</span>
                    <span class="text-[10px] font-mono ${emqColor(emq.composite_score)}">EMQ ${escapeHTMLFb(emq.composite_score != null ? Number(emq.composite_score).toFixed(1) : '—')}</span>
                    <span class="text-[10px] font-mono text-blue-400">Coverage ${escapeHTMLFb(cov.percentage != null ? Number(cov.percentage).toFixed(1) + '%' : '—')}</span>
                    ${acr.percentage != null ? `<span class="text-[10px] font-mono text-emerald-400">ACR +${escapeHTMLFb(Number(acr.percentage).toFixed(1))}%</span>` : ''}
                    ${fresh.upload_frequency ? `<span class="text-[10px] font-mono text-cyan-400">${escapeHTMLFb(fresh.upload_frequency)}</span>` : ''}
                    ${issueCount > 0 ? `<span class="text-[10px] font-mono text-rose-400">${escapeHTMLFb(issueCount)} vấn đề</span>` : ''}
                    <i class="fa-solid fa-chevron-down text-slate-600 text-[10px] ml-auto"></i>
                </summary>
                <div class="px-3.5 pb-3.5 border-t border-slate-800/80 pt-3">
                    ${renderDQEventDetail(ev)}
                </div>
            </details>`;
    }).join('');

    return overview + `<div class="space-y-2">${rows}</div>`;
}

/**
 * renderDQEventDetail vẽ chi tiết 1 event (EMQ keys, dedupe, coverage,
 * delay, value diagnostics, chẩn đoán EMQ, ad sets).
 */
function renderDQEventDetail(ev) {
    const emq = ev.event_match_quality || {};
    const cov = ev.event_coverage || {};
    const delay = ev.event_delay || {};
    const spend = ev.event_spend || {};
    const fresh = ev.data_freshness || {};

    const kv = (rows) => `<div class="space-y-1">${rows.map(r =>
        `<div class="flex items-center justify-between text-[10px] gap-2">
            <span class="text-slate-500 truncate">${escapeHTMLFb(r[0])}</span>
            <span class="font-mono font-bold ${r[2] || 'text-slate-200'} shrink-0">${escapeHTMLFb(r[1])}</span>
        </div>`).join('')}</div>`;

    const block = (title, body) => `
        <div class="bg-slate-900/60 border border-slate-800 rounded-lg p-2.5">
            <div class="text-[10px] font-bold text-slate-400 mb-1.5 uppercase tracking-wider">${escapeHTMLFb(title)}</div>
            ${body}
        </div>`;

    // Khoá so khớp (match keys) + coverage + ACR potential
    const keys = emq.match_key_feedback || [];
    const keysHtml = keys.length ? `<table class="w-full text-[10px]">
        <thead><tr class="text-slate-500">
            <th class="text-left font-semibold">Khoá</th>
            <th class="text-right font-semibold">Coverage</th>
            <th class="text-right font-semibold">ACR potential</th>
        </tr></thead><tbody>
        ${keys.map(k => `<tr class="border-t border-slate-800/60">
            <td class="font-mono text-slate-300 py-0.5">${escapeHTMLFb(k.identifier || '—')}</td>
            <td class="font-mono text-right ${Number(k.coverage && k.coverage.percentage) >= 90 ? 'text-emerald-400' : 'text-amber-400'} py-0.5">${escapeHTMLFb(k.coverage && k.coverage.percentage != null ? Number(k.coverage.percentage).toFixed(1) + '%' : '—')}</td>
            <td class="font-mono text-right text-slate-400 py-0.5">${escapeHTMLFb(k.potential_aly_acr_increase && k.potential_aly_acr_increase.percentage != null ? '+' + Number(k.potential_aly_acr_increase.percentage).toFixed(1) + '%' : '—')}</td>
        </tr>`).join('')}
        </tbody></table>` : '<div class="text-[10px] text-slate-600">Không có dữ liệu khoá so khớp.</div>';

    // Dedupe keys
    const dedupe = ev.dedupe_key_feedback || [];
    const dedupeHtml = dedupe.length ? `<table class="w-full text-[10px]">
        <thead><tr class="text-slate-500">
            <th class="text-left font-semibold">Dedupe key</th>
            <th class="text-right font-semibold">Browser</th>
            <th class="text-right font-semibold">Server</th>
            <th class="text-right font-semibold">Coverage</th>
        </tr></thead><tbody>
        ${dedupe.map(d => `<tr class="border-t border-slate-800/60">
            <td class="font-mono text-slate-300 py-0.5">${escapeHTMLFb(d.dedupe_key || '—')}</td>
            <td class="font-mono text-right text-slate-400 py-0.5">${escapeHTMLFb(d.browser_events_with_dedupe_key && d.browser_events_with_dedupe_key.percentage != null ? Number(d.browser_events_with_dedupe_key.percentage).toFixed(1) + '%' : '—')}</td>
            <td class="font-mono text-right text-slate-400 py-0.5">${escapeHTMLFb(d.server_events_with_dedupe_key && d.server_events_with_dedupe_key.percentage != null ? Number(d.server_events_with_dedupe_key.percentage).toFixed(1) + '%' : '—')}</td>
            <td class="font-mono text-right ${Number(d.overall_browser_coverage_from_dedupe_key && d.overall_browser_coverage_from_dedupe_key.percentage) > 0 ? 'text-emerald-400' : 'text-slate-500'} py-0.5">${escapeHTMLFb(d.overall_browser_coverage_from_dedupe_key && d.overall_browser_coverage_from_dedupe_key.percentage != null ? Number(d.overall_browser_coverage_from_dedupe_key.percentage).toFixed(1) + '%' : '—')}</td>
        </tr>`).join('')}
        </tbody></table>` : '<div class="text-[10px] text-slate-600">Không có phản hồi dedupe.</div>';

    // Chẩn đoán EMQ (6 nhóm lỗi mới từ 09/2026)
    const diags = emq.diagnostics || [];
    const diagHtml = diags.length ? diags.map(d => `
        <div class="bg-rose-950/30 border border-rose-900/60 rounded-lg p-2 mb-1.5">
            <div class="flex items-center justify-between gap-2">
                <span class="text-[10px] font-bold text-rose-300">${escapeHTMLFb(d.name || 'Vấn đề')}</span>
                <span class="font-mono text-[10px] font-bold text-rose-400 shrink-0">${escapeHTMLFb(d.percentage != null ? Number(d.percentage).toFixed(1) + '%' : '')}</span>
            </div>
            ${d.description ? `<div class="text-[9px] text-slate-400 mt-1 leading-relaxed">${escapeHTMLFb(d.description)}</div>` : ''}
            ${d.solution ? `<div class="text-[9px] text-emerald-400/80 mt-1 leading-relaxed"><i class="fa-solid fa-lightbulb mr-1"></i>${escapeHTMLFb(d.solution)}</div>` : ''}
            ${d.affected_event_count ? `<div class="text-[9px] text-slate-500 mt-1 font-mono">${escapeHTMLFb(d.affected_event_count)} / ${escapeHTMLFb(d.total_event_count || 0)} sự kiện bị ảnh hưởng</div>` : ''}
        </div>`).join('') : '<div class="text-[10px] text-emerald-500">Không có vấn đề EMQ nào được phát hiện.</div>';

    // Chẩn đoán value/currency
    const vdiags = ev.value_diagnostics || [];
    const vdiagHtml = vdiags.length ? `<table class="w-full text-[10px]">
        <thead><tr class="text-slate-500">
            <th class="text-left font-semibold">Loại lỗi</th>
            <th class="text-right font-semibold">Số lỗi</th>
            <th class="text-right font-semibold">Tỉ lệ</th>
        </tr></thead><tbody>
        ${vdiags.map(d => `<tr class="border-t border-slate-800/60">
            <td class="font-mono text-slate-300 py-0.5">${escapeHTMLFb(d.issue_type || '—')}</td>
            <td class="font-mono text-right text-rose-400 py-0.5">${escapeHTMLFb(d.issue_count || 0)}</td>
            <td class="font-mono text-right text-amber-400 py-0.5">${escapeHTMLFb(d.issue_rate != null ? (Number(d.issue_rate) * 100).toFixed(1) + '%' : '—')}</td>
        </tr>`).join('')}
        </tbody></table>` : '<div class="text-[10px] text-emerald-500">Không có lỗi value/currency.</div>';

    // Nhóm quảng cáo
    const adsets = ev.event_ad_sets || [];
    const adsetHtml = adsets.length ? `<table class="w-full text-[10px]">
        <thead><tr class="text-slate-500">
            <th class="text-left font-semibold">Ad set</th>
            <th class="text-right font-semibold">Chi tiêu</th>
            <th class="text-right font-semibold">Cách dùng</th>
        </tr></thead><tbody>
        ${adsets.map(a => `<tr class="border-t border-slate-800/60">
            <td class="text-slate-300 py-0.5 truncate" title="${escapeHTMLFb(a.ad_set_name || '')}">${escapeHTMLFb(a.ad_set_name || a.ad_set_id || '—')}</td>
            <td class="font-mono text-right text-amber-400 py-0.5">${escapeHTMLFb(a.spend_amount != null ? Number(a.spend_amount).toLocaleString('vi-VN') : '—')} ${escapeHTMLFb(a.spend_currency_code || '')}</td>
            <td class="font-mono text-right text-slate-400 py-0.5">${escapeHTMLFb(a.event_use || '')}</td>
        </tr>`).join('')}
        </tbody></table>` : '<div class="text-[10px] text-slate-600">Không có nhóm quảng cáo liên kết.</div>';

    return `
        <div class="grid grid-cols-1 lg:grid-cols-2 gap-2.5">
            ${block('Khoá so khớp (Match Keys)', keysHtml)}
            ${block('Bỏ trùng lặp (Dedupe Keys)', dedupeHtml)}
            ${block('Chất lượng & độ trễ', kv([
                ['Coverage', cov.percentage != null ? Number(cov.percentage).toFixed(1) + '%' : '—', 'text-blue-400'],
                ['Mục tiêu coverage', cov.goal_percentage != null ? Number(cov.goal_percentage).toFixed(0) + '%' : '—'],
                ['Upload frequency', fresh.upload_frequency || '—', 'text-cyan-400'],
                ['Tổng sự kiện (delay)', delay.total_events || '—'],
                ['Trễ > 2h', delay.events_delayed_over_2h || 0, delay.events_delayed_over_2h > 0 ? 'text-amber-400' : 'text-slate-300'],
                ['Trễ > 24h', delay.events_delayed_over_24h || 0, delay.events_delayed_over_24h > 0 ? 'text-rose-400' : 'text-slate-300'],
                ['% trễ > 24h', delay.pct_delayed_over_24h != null ? Number(delay.pct_delayed_over_24h).toFixed(1) + '%' : '—'],
                ['Chi tiêu sự kiện', spend.amount != null ? Number(spend.amount).toLocaleString('vi-VN') + ' ' + (spend.currency_code || '') : '—', 'text-amber-400'],
            ]))}
            ${block('Chẩn đoán EMQ', diagHtml)}
            ${block('Chẩn đoán giá trị', vdiagHtml)}
            ${block('Nhóm quảng cáo', adsetHtml)}
        </div>`;
}

// ══════════════════════════════════════════════════════════════
// 4. LOGS & STATS
// ══════════════════════════════════════════════════════════════

/**
 * loadFacebookAdsLogs nạp log gửi sự kiện + thống kê.
 */
async function loadFacebookAdsLogs() {
    const landingId = document.getElementById('fbAdsLandingFilter').value;
    try {
        const json = await fbAdsApi('/api/facebook-ads/logs?landing_id=' + encodeURIComponent(landingId) + '&limit=150');
        fbAdsLogs = json.logs || [];
        renderFacebookAdsLogs();

        const st = await fbAdsApi('/api/facebook-ads/stats?landing_id=' + encodeURIComponent(landingId) + '&days=30');
        renderFacebookAdsStats(st.stats || {});
    } catch (e) {
        fbAdsToast('Lỗi tải log CAPI: ' + e.message, 'error');
    }
}

/**
 * renderFacebookAdsStats vẽ 4 thẻ KPI.
 */
function renderFacebookAdsStats(s) {
    const set = (id, v) => { const el = document.getElementById(id); if (el) el.innerText = Number(v || 0).toLocaleString('vi-VN'); };
    set('fbAdsStatSent', s.total_sent);
    set('fbAdsStatSuccess', s.total_success);
    set('fbAdsStatFailed', s.total_failed);
    set('fbAdsStatSkipped', s.total_skipped);

    const evBox = document.getElementById('fbAdsEventBreakdown');
    if (evBox) {
        const byEvent = s.by_event || {};
        const keys = Object.keys(byEvent);
        evBox.innerHTML = keys.length
            ? keys.map(k => `
                <div class="flex items-center justify-between text-[11px] py-1 border-b border-slate-800/60 last:border-0">
                    <span class="font-mono text-slate-300">${escapeHTMLFb(k)}</span>
                    <span class="font-mono font-bold text-orange-400">${byEvent[k]}</span>
                </div>`).join('')
            : '<div class="py-3 text-center text-slate-600 text-[11px]">Chưa có sự kiện nào</div>';
    }

    const errBox = document.getElementById('fbAdsLastError');
    if (errBox) {
        if (s.last_error) {
            errBox.className = 'mt-2 text-[10px] text-rose-400 leading-relaxed break-words';
            errBox.innerText = `Lỗi gần nhất (${escapeHTMLFb(s.last_error_at || '')}): ${escapeHTMLFb(s.last_error)}`;
        } else {
            errBox.className = 'mt-2 text-[10px] text-emerald-400';
            errBox.innerText = 'Không có lỗi ghi nhận trong kỳ.';
        }
    }
}

/**
 * renderFacebookAdsLogs vẽ bảng log.
 */
function renderFacebookAdsLogs() {
    const tb = document.getElementById('fbAdsLogTableBody');
    if (!tb) return;

    if (!fbAdsLogs.length) {
        tb.innerHTML = '<tr><td colspan="9" class="py-8 text-center text-slate-500 text-xs">Chưa có log gửi sự kiện nào.</td></tr>';
        return;
    }

    tb.innerHTML = fbAdsLogs.map(l => {
        const st = FB_ADS_STATUSES.find(s => s.key === l.status) || { label: l.status, cls: 'bg-slate-950 text-slate-400 border-slate-800' };
        const trigger = {
            lead_submit: 'Gửi form',
            crm_status_change: 'Đổi trạng thái CRM',
            page_view: 'PageView',
            manual_test: 'Test thủ công',
        }[l.triggered_by] || l.triggered_by || '-';

        // ⚠️ Dedup Pixel ↔ CAPI (fix 2026-11-16):
        // Sự kiện `PageView` có `event_id` dạng `pv_*` ⇒ đó là event_id
        // DÙNG CHUNG với Pixel trình duyệt ⇒ Meta gộp 2 sự kiện thành 1
        // (1 lượt xem = 1 PageView, không đếm trùng).
        const evtID = l.event_id || '-';
        const isPVShared = typeof l.event_id === 'string' && l.event_id.indexOf('pv') === 0;
        const eventIdCell = isPVShared
            ? `<span class="font-mono text-[10px] text-emerald-400" title="Event ID dùng chung với Meta Pixel trình duyệt ⇒ Meta gộp 2 sự kiện thành 1">${escapeHTMLFb(evtID)}</span>
               <span class="block text-[9px] text-emerald-500/80 mt-0.5">✓ đã gộp với Pixel</span>`
            : `<span class="font-mono text-[10px] text-slate-500" title="${escapeHTMLFb(evtID)}">${escapeHTMLFb(evtID.slice(0, 26))}${evtID.length > 26 ? '…' : ''}</span>`;

        return `
            <tr class="hover:bg-slate-800/40 transition">
                <td class="py-2.5 px-3 font-mono text-[10px] text-slate-500">${escapeHTMLFb(l.created_at)}</td>
                <td class="py-2.5 px-3">
                    <span class="text-[10px] px-2 py-0.5 rounded font-mono font-bold ${st.cls}">${escapeHTMLFb(st.label)}</span>
                </td>
                <td class="py-2.5 px-3 font-mono text-[11px] font-bold text-orange-400">${escapeHTMLFb(l.event_name || '-')}</td>
                <td class="py-2.5 px-3">${eventIdCell}</td>
                <td class="py-2.5 px-3 text-[11px] text-slate-400">${escapeHTMLFb(l.landing_id || 'Mặc định')}</td>
                <td class="py-2.5 px-3 text-[11px] text-slate-400">${l.lead_id ? '#' + escapeHTMLFb(l.lead_id) : '-'}</td>
                <td class="py-2.5 px-3 text-[11px] text-slate-400">${escapeHTMLFb(trigger)}</td>
                <td class="py-2.5 px-3 font-mono text-[10px] text-slate-500">${escapeHTMLFb(l.http_status || '-')}</td>
                <td class="py-2.5 px-3 text-[11px] text-rose-300 max-w-xs break-words">${escapeHTMLFb(l.error || '')}</td>
            </tr>
        `;
    }).join('');
}

// ══════════════════════════════════════════════════════════════
// 5. CAMPAIGN BREAKDOWN (QC)
// ══════════════════════════════════════════════════════════════

/**
 * loadFacebookAdsCampaigns nạp báo cáo hiệu quả theo chiến dịch.
 */
async function loadFacebookAdsCampaigns() {
    const landingId = document.getElementById('fbAdsLandingFilter').value;
    const tb = document.getElementById('fbAdsCampaignTableBody');
    if (tb) tb.innerHTML = '<tr><td colspan="9" class="py-6 text-center text-slate-500 text-xs"><i class="fa-solid fa-spinner fa-spin mr-2"></i>Đang tải...</td></tr>';

    try {
        const json = await fbAdsApi('/api/facebook-ads/campaigns?landing_id=' + encodeURIComponent(landingId) + '&limit=100');
        fbAdsCampaigns = json.rows || [];
        renderFacebookAdsCampaigns();
    } catch (e) {
        if (tb) tb.innerHTML = `<tr><td colspan="9" class="py-6 text-center text-rose-400 text-xs">${escapeHTMLFb(e.message)}</td></tr>`;
    }
}

/**
 * renderFacebookAdsCampaigns vẽ bảng QC chiến dịch.
 */
function renderFacebookAdsCampaigns() {
    const tb = document.getElementById('fbAdsCampaignTableBody');
    if (!tb) return;

    if (!fbAdsCampaigns.length) {
        tb.innerHTML = '<tr><td colspan="9" class="py-8 text-center text-slate-500 text-xs">Chưa có dữ liệu chiến dịch. Kiểm tra lại URL Parameters trong Ads Manager (utm_campaign/utm_content/utm_term).</td></tr>';
        return;
    }

    tb.innerHTML = fbAdsCampaigns.map(r => {
        const rate = Number(r.won_rate || 0);
        const rateCls = rate >= 20 ? 'text-emerald-400' : rate >= 5 ? 'text-amber-400' : 'text-slate-400';
        return `
            <tr class="hover:bg-slate-800/40 transition">
                <td class="py-2.5 px-3">
                    <div class="font-semibold text-slate-200 text-[11px]">${escapeHTMLFb(r.campaign_name || '(không có tên)')}</div>
                    <div class="font-mono text-[10px] text-slate-500">${escapeHTMLFb(r.campaign_id || '-')}</div>
                </td>
                <td class="py-2.5 px-3">
                    <div class="text-[11px] text-slate-300">${escapeHTMLFb(r.adset_name || '(không có tên)')}</div>
                    <div class="font-mono text-[10px] text-slate-500">${escapeHTMLFb(r.adset_id || '-')}</div>
                </td>
                <td class="py-2.5 px-3">
                    <div class="text-[11px] text-slate-300">${escapeHTMLFb(r.ad_name || '(không có tên)')}</div>
                    <div class="font-mono text-[10px] text-slate-500">${escapeHTMLFb(r.ad_id || '-')}</div>
                </td>
                <td class="py-2.5 px-3 text-[11px] text-slate-400">${escapeHTMLFb(r.placement || '-')}</td>
                <td class="py-2.5 px-3 text-center font-mono font-bold text-orange-400">${r.leads}</td>
                <td class="py-2.5 px-3 text-center font-mono text-slate-400">${r.new_leads}</td>
                <td class="py-2.5 px-3 text-center font-mono font-bold text-emerald-400">${r.won_leads}</td>
                <td class="py-2.5 px-3 text-center font-mono font-bold ${rateCls}">${rate.toFixed(1)}%</td>
                <td class="py-2.5 px-3 text-right font-mono text-[11px] text-emerald-400">${Number(r.revenue || 0).toLocaleString('vi-VN')}</td>
            </tr>
        `;
    }).join('');
}

// ══════════════════════════════════════════════════════════════
// 6. PARSE URL (kiểm tra URL Parameters)
// ══════════════════════════════════════════════════════════════

/**
 * fbAdsParseURL phân tích URL quảng cáo Meta.
 */
async function fbAdsParseURL() {
    const url = document.getElementById('fbAdsParseURLInput').value.trim();
    if (!url) { fbAdsToast('Dán URL landing có tham số quảng cáo vào ô bên trên', 'error'); return; }

    try {
        const json = await fbAdsApi('/api/facebook-ads/parse-url?url=' + encodeURIComponent(url));
        const p = json.params || {};
        const box = document.getElementById('fbAdsParseResult');
        box.innerHTML = `
            <div class="grid grid-cols-2 md:grid-cols-3 gap-2">
                ${[
            ['utm_campaign → campaign.id', p.campaign_id],
            ['utm_content → adset.id', p.adset_id],
            ['utm_term → ad.id', p.ad_id],
            ['utm_placement → placement', p.placement],
            ['utm_source → source', p.utm_source],
            ['site_source_name', p.site_source_name],
            ['campaign_name', p.campaign_name],
            ['adset_name', p.adset_name],
            ['ad_name', p.ad_name],
            ['fbclid', (p.fbclid || '').slice(0, 24) + ((p.fbclid || '').length > 24 ? '…' : '')],
            ['_fbc (dựng lại)', json.fbc],
        ].map(([label, val]) => `
                    <div class="bg-slate-950 border border-slate-800 rounded-lg p-2">
                        <div class="text-[9px] text-slate-500 uppercase tracking-wider">${escapeHTMLFb(label)}</div>
                        <div class="font-mono text-[11px] ${val ? 'text-emerald-400' : 'text-slate-600'} break-all mt-0.5">${escapeHTMLFb(val || '(trống)')}</div>
                    </div>`).join('')}
            </div>
            ${(json.notes || []).map(n => `<div class="mt-2 text-[10px] text-slate-500 flex items-start gap-1.5"><i class="fa-solid fa-circle-info mt-0.5"></i><span>${escapeHTMLFb(n)}</span></div>`).join('')}
        `;
    } catch (e) {
        fbAdsToast('Lỗi phân tích URL: ' + e.message, 'error');
    }
}

// ══════════════════════════════════════════════════════════════
// 7. LOAD TOÀN BỘ TAB
// ══════════════════════════════════════════════════════════════

/**
 * loadFacebookAdsTab nạp toàn bộ dữ liệu của tab.
 */
async function loadFacebookAdsTab() {
    await loadFacebookAdsConfigs();
    await Promise.all([
        loadFacebookAdsLogs(),
        loadFacebookAdsCampaigns(),
    ]);
}
