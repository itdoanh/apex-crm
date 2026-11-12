/* ============================================
   APEX FINTECH — DOANH_CHUNG ADAPTER (2026-11-12)
   ------------------------------------------------------------
   Landing `doanh_chung` được lấy nguyên mã nguồn từ
   C:\code\apex_moi\apex\chungkhoan\chung.

   Bản gốc gửi lead thẳng tới Google Apps Script Web App
   (assets/js/sheets-direct.js) với SECRET hardcode trong client.
   ⇒ Sai kiến trúc: lead không vào CRM, không có analytics, không có
      chống bot, không ghi Google Sheets qua pipeline chuẩn của Apex.

   File này là ADAPTER thay thế `sheets-direct.js`:
     1. Gửi lead qua `window.ApexTracker.sendLeadToBackend()` → POST /api/leads
        của Apex Backend (có PoW chống bot, honeypot, dedupe, geo, Sheets sync).
     2. Bổ sung TOÀN BỘ thông số Meta Ads macro + click ID + thiết bị chi tiết.
     3. Tách hẳn storage theo landing (`apex_*_doanh_chung`) để không lẫn
        thông số với các landing khác.

   ⚠️ KHÔNG dùng file này cho landing khác.
   ============================================ */

(function () {
  'use strict';

  var LANDING_ID = 'doanh_chung';
  var DEBUG = false;

  function log() {
    if (!DEBUG) return;
    try { console.log.apply(console, ['[doanh_chung]'].concat([].slice.call(arguments))); } catch (_) {}
  }

  /* ───────────────────────────────────────────────────────────
     1. Đọc thông số Meta Ads macro + click ID từ URL
     ─────────────────────────────────────────────────────────── */

  // Chuỗi UTM khuyến nghị cho Meta Ads Manager:
  //   utm_medium=paid&utm_source={{site_source_name}}
  //   &utm_placement={{placement}}&utm_campaign={{campaign.id}}
  //   &utm_content={{adset.id}}&utm_term={{ad.id}}
  //   &campaign_name={{campaign.name}}&adset_name={{adset.name}}&ad_name={{ad.name}}
  function qParam(name) {
    try {
      var m = String(location.search || '').match(new RegExp('[?&]' + name + '=([^&#]*)'));
      return m ? decodeURIComponent(m[1].replace(/\+/g, ' ')) : '';
    } catch (_) { return ''; }
  }

  function readCookie(name) {
    try {
      var parts = (document.cookie || '').split(';');
      for (var i = 0; i < parts.length; i++) {
        var kv = parts[i].trim().split('=');
        if (kv[0] === name) return decodeURIComponent(kv.slice(1).join('=') || '');
      }
    } catch (_) {}
    return '';
  }

  function collectMetaMacros() {
    var fbclid = qParam('fbclid');
    var igshid = qParam('igshid');
    var fbc = readCookie('_fbc');
    var fbp = readCookie('_fbp');

    // Dựng lại `_fbc` từ `fbclid` (định dạng Meta: fb.1.<ts>.<fbclid>)
    if (!fbc && fbclid) {
      fbc = (fbclid.indexOf('fb.') === 0 && (fbclid.match(/\./g) || []).length >= 3)
        ? fbclid
        : 'fb.1.0.' + fbclid;
    }

    var siteSourceName = qParam('site_source_name');
    if (!siteSourceName) {
      var src = qParam('utm_source').toLowerCase();
      if (src === 'facebook' || src === 'fb') siteSourceName = 'fb';
      else if (src === 'instagram' || src === 'ig') siteSourceName = 'ig';
      else if (src === 'messenger' || src === 'msg') siteSourceName = 'msg';
      else if (src === 'audience network' || src === 'an') siteSourceName = 'an';
    }

    return {
      utm_source: qParam('utm_source'),
      utm_medium: qParam('utm_medium'),
      utm_campaign: qParam('utm_campaign'),
      utm_content: qParam('utm_content'),
      utm_term: qParam('utm_term'),
      utm_placement: qParam('utm_placement') || qParam('placement'),
      campaign_name: qParam('campaign_name'),
      adset_name: qParam('adset_name'),
      ad_name: qParam('ad_name'),
      ad_account_id: qParam('ad_account_id') || qParam('acc_id') || qParam('ad_account'),
      site_source_name: siteSourceName,
      fbclid: fbclid,
      fbc: fbc,
      fbp: fbp,
      igshid: igshid,
      meta_click_id: fbclid || igshid
    };
  }

  /* ───────────────────────────────────────────────────────────
     2. Chuẩn hoá SĐT Việt Nam
     ─────────────────────────────────────────────────────────── */

  function cleanPhone(phone) {
    var v = String(phone || '').replace(/[\s.\-()]/g, '');
    if (v.indexOf('+84') === 0) return '0' + v.slice(3);
    if (v.indexOf('84') === 0 && v.length === 11) return '0' + v.slice(2);
    return v;
  }

  function isValidVnPhone(phone) {
    return /^(0[35789])[0-9]{8}$/.test(cleanPhone(phone));
  }

  /* ───────────────────────────────────────────────────────────
     3. Storage tách theo landing (KHÔNG lẫn với landing khác)
     ─────────────────────────────────────────────────────────── */

  function scopedKey(base) {
    return base + '_' + LANDING_ID;
  }

  var ApexSafeStorage = {
    set: function (key, value) {
      try { localStorage.setItem(scopedKey(key), String(value)); } catch (_) {}
    },
    get: function (key) {
      try { return localStorage.getItem(scopedKey(key)) || ''; } catch (_) { return ''; }
    },
    remove: function (key) {
      try { localStorage.removeItem(scopedKey(key)); } catch (_) {}
    }
  };
  window.ApexSafeStorage = ApexSafeStorage;

  /* ───────────────────────────────────────────────────────────
     4. Gửi lead qua Apex Backend (thay cho Google Apps Script)
     ─────────────────────────────────────────────────────────── */

  var sending = false;

  function sendLead(payload) {
    if (sending) {
      return Promise.resolve({ ok: true, duplicate: true, id: 0 });
    }
    sending = true;

    var meta = collectMetaMacros();
    var phone = cleanPhone(payload.phone || payload.so_dien_thoai || '');
    var name = String(payload.name || payload.fullName || '').trim();

    var tracker = window.ApexTracker;

    var fullPayload = {
      landing_id: LANDING_ID,
      nguon: 'doanh_chung',
      so_dien_thoai: phone,
      phone: phone,
      fullName: name,
      name: name,
      session_id: (tracker && tracker.sessionId) || payload.session_id || '',
      visitor_id: (tracker && tracker.visitorId) || '',
      entry_url: payload.page_url || location.href,
      url: payload.page_url || location.href,
      referrer: payload.referrer || document.referrer || '',

      // ── Macro Meta Ads ──
      utm_source: meta.utm_source,
      utm_medium: meta.utm_medium,
      utm_campaign: meta.utm_campaign,
      utm_content: meta.utm_content,
      utm_term: meta.utm_term,
      utm_placement: meta.utm_placement,
      campaign_name: meta.campaign_name,
      adset_name: meta.adset_name,
      ad_name: meta.ad_name,
      ad_account_id: meta.ad_account_id,
      site_source_name: meta.site_source_name,

      // ── Định danh lượt nhấp Meta (CAPI) ──
      fbclid: meta.fbclid,
      fbc: meta.fbc,
      fbp: meta.fbp,
      igshid: meta.igshid,
      meta_click_id: meta.meta_click_id,

      // ── Thiết bị (lấy từ tracker.js nếu có) ──
      device_type: payload.device_type || '',
      browser: payload.browser || '',
      os: payload.os || '',
      screen_res: payload.screen_res || '',

      // ── Hành vi / form ──
      form_type: payload.form_type || '',
      target_group: payload.target_group || 'chung',
      channel: payload.channel || '',
      time_on_page: payload.time_on_page || 0,
      max_scroll: payload.max_scroll || 0,
      clicks_count: payload.clicks_count || 0,
      is_autofill: payload.is_autofill || false,
      autofill_source: payload.autofill_source || '',
      idempotency_key: payload.idempotency_key || ''
    };

    // Bổ sung thông số thiết bị chi tiết + hành vi từ tracker.js
    if (tracker && typeof tracker.collectDeviceInfo === 'function') {
      try {
        var dev = tracker.collectDeviceInfo();
        for (var k in dev) {
          if (dev[k] !== '' && dev[k] !== undefined && fullPayload[k] === undefined) {
            fullPayload[k] = dev[k];
          }
        }
      } catch (_) {}
    }
    if (tracker && typeof tracker.getTrackingState === 'function') {
      try {
        var st = tracker.getTrackingState();
        fullPayload.visible_seconds = st.visible_seconds;
        fullPayload.hidden_seconds = st.hidden_seconds;
        fullPayload.session_count = st.session_count;
        fullPayload.has_video_play = st.has_video_play;
        fullPayload.has_zalo_click = st.has_zalo_click;
        fullPayload.has_tel_click = st.has_tel_click;
        fullPayload.scroll_depth_25 = !!st.scroll_milestones['25'];
        fullPayload.scroll_depth_50 = !!st.scroll_milestones['50'];
        fullPayload.scroll_depth_75 = !!st.scroll_milestones['75'];
        fullPayload.scroll_depth_90 = !!st.scroll_milestones['90'];
        fullPayload.scroll_depth_100 = !!st.scroll_milestones['100'];
      } catch (_) {}
    }

    if (!tracker || typeof tracker.sendLeadToBackend !== 'function') {
      sending = false;
      log('tracker.js chưa sẵn sàng — không gửi lead');
      return Promise.resolve({ ok: false, error: 'tracker_not_ready' });
    }

    return tracker.sendLeadToBackend(fullPayload)
      .then(function (res) {
        sending = false;
        log('lead sent', res);
        return { ok: !!(res && res.success !== false), id: (res && res.id) || 0, raw: res };
      })
      .catch(function (err) {
        sending = false;
        log('lead error', err);
        return { ok: false, error: String(err) };
      });
  }

  /* ───────────────────────────────────────────────────────────
     5. API tương thích `window.ApexSheetsDirect`
     ───────────────────────────────────────────────────────────
     main.js (bản gốc) gọi `window.ApexSheetsDirect.send(...)` ở một số chỗ
     và `sendLead(...)` ở chỗ khác. Cung cấp đủ 2 API để không phá gì.
     ─────────────────────────────────────────────────────────── */

  window.ApexSheetsDirect = {
    send: function (leadInput) {
      return sendLead(leadInput || {});
    },
    sendLead: sendLead,
    flush: function () { return Promise.resolve({ ok: true }); },
    isBot: false,
    isWhitelistedBot: false,
    sessionId: '',
    configure: function () { /* không còn dùng Apps Script */ },
    getConfig: function () {
      return { backend: 'apex', landing_id: LANDING_ID, secret_configured: false };
    },
    __buf: []
  };

  /* ───────────────────────────────────────────────────────────
     6. Track sự kiện hành vi của main.js vào timeline chuẩn
     ─────────────────────────────────────────────────────────── */

  (function bridgeTrackEvent() {
    // main.js định nghĩa trackEvent() nội bộ (không export ra window).
    // main.js gọi `window.ApexTracker.send()` nếu tồn tại ⇒ tracker.js chuẩn
    // đã hỗ trợ alias `send` ⇒ không cần bridge.
    // Ở đây chỉ bảo đảm các hàm vi không lỗi khi tracker.js chưa nạp.
    window.addEventListener('load', function () {
      if (!window.ApexTracker) log('CẢNH BÁO: tracker.js chưa được nạp — analytics sẽ không hoạt động');
    });
  })();

  /* ───────────────────────────────────────────────────────────
     7. Chặn submit trùng ở mức form (double-click)
     ─────────────────────────────────────────────────────────── */

  document.addEventListener('submit', function (e) {
    var form = e.target;
    if (!form || !form.tagName || form.tagName !== 'FORM') return;
    if (form.getAttribute('data-apex-submitting') === 'true') {
      e.preventDefault();
      e.stopPropagation();
      return;
    }
    form.setAttribute('data-apex-submitting', 'true');
    // Timeout 12s: nếu request treo quá lâu thì mở khoá để khách thử lại.
    setTimeout(function () {
      try { form.removeAttribute('data-apex-submitting'); } catch (_) {}
    }, 12000);
  }, true);

  window.ApexDoanhChung = {
    landingId: LANDING_ID,
    sendLead: sendLead,
    collectMetaMacros: collectMetaMacros,
    scopedKey: scopedKey,
    cleanPhone: cleanPhone,
    isValidVnPhone: isValidVnPhone
  };

})();
