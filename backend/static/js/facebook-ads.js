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
        renderFacebookAdsLandingOptions();
    } catch (e) {
        fbAdsToast('Lỗi tải cấu hình CAPI: ' + e.message, 'error');
    }
}

/**
 * renderFacebookAdsLandingOptions dựng danh sách landing cho các dropdown.
 */
function renderFacebookAdsLandingOptions() {
    const ids = Array.from(new Set(fbAdsConfigs.map(c => c.landing_id || '')));
    const cur = fbAdsActiveLanding;

    const opts = ids.map(id => {
        const label = id === '' ? 'Mặc định (áp dụng mọi landing)' : id;
        return `<option value="${escapeHTMLFb(id)}"${id === cur ? ' selected' : ''}>${escapeHTMLFb(label)}</option>`;
    }).join('');

    ['fbAdsLandingSelect', 'fbAdsLandingFilter'].forEach(selId => {
        const sel = document.getElementById(selId);
        if (!sel) return;
        sel.innerHTML = (opts || '<option value="">Chưa có cấu hình nào</option>');
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
        const active = c.is_active
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
                        <button onclick="fbAdsEditConfig('${escapeHTMLFb(c.landing_id)}')" class="text-[10px] px-2 py-1 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 transition">Sửa</button>
                        <button onclick="fbAdsDeleteConfig('${escapeHTMLFb(c.landing_id)}')" class="text-[10px] px-2 py-1 rounded bg-rose-950 hover:bg-rose-900 text-rose-400 border border-rose-900 transition">Xoá</button>
                    </div>
                </td>
            </tr>
        `;
    }).join('');
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
 */
async function fbAdsDeleteConfig(landingId) {
    if (!confirm(`Xoá cấu hình CAPI của "${landingId || 'Mặc định'}"?\n\nLưu ý: log gửi sự kiện vẫn được giữ lại để đối soát.`)) return;
    try {
        await fbAdsApi('/api/facebook-ads/config/delete?landing_id=' + encodeURIComponent(landingId || ''), { method: 'POST' });
        fbAdsToast('Đã xoá cấu hình', 'success');
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
 */
function renderFacebookAdsQuality(q) {
    const emq = q.event_match_quality || {};
    const cov = q.event_coverage || {};
    const dedupe = q.dedupe_key_feedback || {};
    const fresh = q.data_freshness || {};
    const delay = q.event_delay || {};
    const value = q.value_diagnostics || {};

    const emqScore = emq.score != null ? Number(emq.score).toFixed(1) : '—';
    const emqScoreNorm = emq.score_norm != null ? Number(emq.score_norm).toFixed(4) : null;

    const card = (title, icon, color, body) => `
        <div class="bg-slate-950 border border-slate-800 rounded-xl p-3.5">
            <div class="flex items-center gap-1.5 mb-2">
                <i class="fa-solid ${icon} ${color} text-xs"></i>
                <span class="text-[11px] font-bold text-slate-200">${title}</span>
            </div>
            ${body}
        </div>`;

    const kv = (rows) => `<div class="space-y-1">${rows.map(r =>
        `<div class="flex items-center justify-between text-[10px]">
            <span class="text-slate-500">${escapeHTMLFb(r[0])}</span>
            <span class="font-mono font-bold ${r[2] || 'text-slate-200'}">${escapeHTMLFb(r[1])}</span>
        </div>`).join('')}</div>`;

    const scoreColor = emqScoreNorm == null ? 'text-slate-300'
        : emqScoreNorm >= 6 ? 'text-emerald-400'
            : emqScoreNorm >= 4 ? 'text-amber-400' : 'text-rose-400';

    return `
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
            ${card('Event Match Quality (EMQ)', 'fa-bullseye', 'text-emerald-400', `
                <div class="text-center py-2">
                    <div class="text-3xl font-black font-mono ${scoreColor}">${escapeHTMLFb(emqScore)}</div>
                    <div class="text-[10px] text-slate-500 mt-0.5">điểm EMQ (0–10)</div>
                    ${emqScoreNorm != null ? `<div class="text-[10px] font-mono ${scoreColor} mt-1">norm: ${escapeHTMLFb(emqScoreNorm)}</div>` : ''}
                </div>
                ${kv([
        ['Tổng sự kiện khớp', emq.matched_event_count ?? emq.total_matched ?? '—'],
        ['Sự kiện có thể khớp', emq.matched_and_missed_event_count ?? '—'],
        ['Tỉ lệ khớp (%)', emq.matched_event_rate ?? '—', 'text-emerald-400'],
        ['Sự kiện bị bỏ qua', emq.missed_event_count ?? '—', 'text-rose-400'],
    ])}
            `)}

            ${card('Event Coverage', 'fa-chart-simple', 'text-blue-400', kv([
        ['Sự kiện đã gửi (server)', cov.server_events ?? '—'],
        ['Sự kiện trình duyệt', cov.browser_events ?? '—'],
        ['Tổng sự kiện', cov.total_events ?? '—'],
        ['Tỉ lệ phủ (%)', cov.event_coverage_ratio ?? '—', 'text-blue-400'],
    ]))}

            ${card('Dedupe Key Feedback', 'fa-clone', 'text-purple-400', kv([
        ['Sự kiện trùng (browser)', dedupe.browser_dupes ?? '—', 'text-amber-400'],
        ['Sự kiện trùng (server)', dedupe.server_dupes ?? '—', 'text-amber-400'],
        ['Tỉ lệ trùng (%)', dedupe.dedupe_ratio ?? '—'],
        ['Gợi ý', dedupe.dedupe_key_suggestions ?? '—', 'text-slate-400'],
    ]))}

            ${card('Data Freshness', 'fa-clock', 'text-cyan-400', kv([
        ['Sự kiện mới nhất', fresh.last_event_timestamp ?? '—'],
        ['Độ trễ dữ liệu (giây)', fresh.data_freshness_seconds ?? '—', 'text-cyan-400'],
        ['Trạng thái', fresh.status ?? '—'],
    ]))}

            ${card('Event Delay', 'fa-hourglass-half', 'text-amber-400', kv([
        ['Độ trễ trung bình (phút)', delay.average_event_delay_minutes ?? '—'],
        ['Độ trễ tối đa (phút)', delay.max_event_delay_minutes ?? '—', 'text-amber-400'],
        ['Số sự kiện bị trễ', delay.delayed_events ?? '—'],
    ]))}

            ${card('Value Diagnostics', 'fa-money-bill-wave', 'text-emerald-400', kv([
        ['Tổng giá trị', value.total_value ?? '—', 'text-emerald-400'],
        ['Số sự kiện có giá trị', value.events_with_value ?? '—'],
        ['Tiền tệ', value.currency ?? '—'],
    ]))}
        </div>
    `;
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

        return `
            <tr class="hover:bg-slate-800/40 transition">
                <td class="py-2.5 px-3 font-mono text-[10px] text-slate-500">${escapeHTMLFb(l.created_at)}</td>
                <td class="py-2.5 px-3">
                    <span class="text-[10px] px-2 py-0.5 rounded font-mono font-bold ${st.cls}">${escapeHTMLFb(st.label)}</span>
                </td>
                <td class="py-2.5 px-3 font-mono text-[11px] font-bold text-orange-400">${escapeHTMLFb(l.event_name || '-')}</td>
                <td class="py-2.5 px-3 font-mono text-[10px] text-slate-500">${escapeHTMLFb((l.event_id || '-').slice(0, 26))}</td>
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
