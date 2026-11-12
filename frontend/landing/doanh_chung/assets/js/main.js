/* ============================================
   APEX FINTECH - MAIN.JS — CHIASEV1.2 (frontend-only)
   Không backend PHP. Gửi lead thẳng qua window.ApexSheetsDirect.send().
   sheets-direct.js tự xử lý retry/queue/localStorage nội bộ.

   Tính năng:
     - 3 forms (hero / multi-step / modal popup)
     - Validation SĐT Việt Nam (03/05/07/08/09 + +84)
     - Countdown timer, video lazy-load, evidence carousel
     - Sticky CTA, mobile menu
     - Meta Pixel Lead tracking
   ============================================ */

  (function(){
    'use strict';

    // ════════════════════════════════════════════════════════════════
    //  DEBUG FLAG — Production: false (tắt console để tránh leak info)
    //  Dev:        true
    // ════════════════════════════════════════════════════════════════
    var DEBUG = false;
    var _log = function () { if (DEBUG) { try { console.log.apply(console, ['[APEX]'].concat(Array.prototype.slice.call(arguments))); } catch (_) {} } };
    var _warn = function () { if (DEBUG) { try { console.warn.apply(console, ['[APEX]'].concat(Array.prototype.slice.call(arguments))); } catch (_) {} } };

    const ready = fn => document.readyState !== 'loading'
      ? fn()
      : document.addEventListener('DOMContentLoaded', fn);

    function init(){
      initMobileMenu();
      initModalPopup();
      initForms();
      initMultiStepForm();
      initSmoothScroll();
      initStickyCtaVisibility();
      initSeatsCounter();
      initGiftsCounter();
      initCountdownTimer();
      initVideoPlayer();
      initEvidenceCarousel();
    }

    /* ===========================================================
       1) MOBILE MENU
       =========================================================== */
    function initMobileMenu(){
      const toggle = document.getElementById('menuToggle');
      const menu = document.getElementById('mobileMenu');
      if (!toggle || !menu) return;
      toggle.addEventListener('click', () => menu.classList.remove('hidden'));
      menu.querySelectorAll('[data-close-mobile-menu]').forEach(el => {
        el.addEventListener('click', () => menu.classList.add('hidden'));
      });
    }

    /* ===========================================================
       2) MODAL POPUP
       =========================================================== */
    function initModalPopup(){
      const modal = document.getElementById('leadModal');
      if (!modal) return;

      let lastTrigger = null;

      const open = (trigger) => {
        lastTrigger = trigger;
        modal.classList.add('is-open');
        modal.setAttribute('aria-hidden', 'false');
        document.body.classList.add('modal-open');
        const ctaLabel = trigger?.dataset?.ctaLabel || trigger?.dataset?.modalTrigger || 'unknown';
        trackEvent('modal_open', ctaLabel);
        setTimeout(() => {
          const name = modal.querySelector('input[name="name"]');
          if (name) name.focus({ preventScroll: true });
        }, 250);
      };

      const close = () => {
        modal.classList.remove('is-open');
        modal.setAttribute('aria-hidden', 'true');
        document.body.classList.remove('modal-open');
        const form = modal.querySelector('form');
        const success = modal.querySelector('#modalSuccess');
        if (form) {
          form.style.display = '';
          form.reset();
        }
        if (success) success.classList.add('hidden');
        // Khôi phục lại header modal (có thể bị ẩn khi submit thành công)
        const header = modal.querySelector('.modal-header-new');
        if (header) header.style.display = '';
        // Ẩn phone hint khi đóng modal
        const hint = document.getElementById('modalPhoneHint');
        if (hint) hint.classList.add('hidden');
      };

      document.querySelectorAll('[data-modal-trigger]').forEach(btn => {
        btn.addEventListener('click', e => {
          e.preventDefault();
          open(btn);
        });
      });
      modal.querySelectorAll('[data-close-modal]').forEach(el => {
        el.addEventListener('click', close);
      });
      document.addEventListener('keydown', e => {
        if (e.key === 'Escape' && modal.classList.contains('is-open')) close();
      });
      window.ApexModal = { open, close };
    }

    /* ===========================================================
       3) FORM HANDLING (HERO + MODAL)
       =========================================================== */
    function initForms(){
      const forms = [
        { id: 'heroForm',  type: 'hero',  nameField: 'heroName',  phoneField: 'heroPhone',  successId: 'heroSuccess',  phoneHintId: 'heroPhoneHint'  },
        { id: 'modalForm', type: 'modal', nameField: 'modalName', phoneField: 'modalPhone', successId: 'modalSuccess', phoneHintId: 'modalPhoneHint' }
      ];
      forms.forEach(cfg => setupForm(cfg));
    }

    function setupForm(cfg){
      const form = document.getElementById(cfg.id);
      if (!form) { _warn('form NOT FOUND:', cfg.id); return; }
      const nameInput = document.getElementById(cfg.nameField);
      const phoneInput = document.getElementById(cfg.phoneField);
      const phoneHint = document.getElementById(cfg.phoneHintId);
      const successEl = document.getElementById(cfg.successId);
      if (DEBUG) _log('setupForm:', cfg.id, 'successEl=', !!successEl, 'nameInput=', !!nameInput, 'phoneInput=', !!phoneInput);

      // Format phone
      if (phoneInput) {
        phoneInput.addEventListener('input', () => {
          // Cho phép 1 dấu '+' ở đầu + các chữ số (0-9)
          let v = phoneInput.value;
          v = v.replace(/[^\d+]/g, '');
          v = v.replace(/\+/g, (m, offset) => (offset === 0 ? '+' : ''));
          // +84XXX → tối đa 13 ký tự (1 dấu + + 2 số + 9 số = 12) hoặc +84XXXXXXXXX (12 ký tự)
          // 0XXXXXXXXX → tối đa 11 ký tự
          // Cho phép max 13 để cover "+84XXXXXXXXX" (12) và "+849XXXXXXXX" (12)
          if (v.length > 13) v = v.slice(0, 13);
          phoneInput.value = v;
          if (v.length > 0) {
            phoneInput.classList.remove('error');
            phoneInput.classList.add('has-value');
            // Ẩn hint khi user đang nhập lại
            if (phoneHint) phoneHint.classList.add('hidden');
          } else {
            phoneInput.classList.remove('has-value');
          }
          trackEvent('phone_input', `${cfg.type}|len=${v.length}`);
        });
      }
      if (nameInput) {
        nameInput.addEventListener('input', () => {
          if (nameInput.value.length > 0) {
            nameInput.classList.remove('error');
            nameInput.classList.add('has-value');
          } else {
            nameInput.classList.remove('has-value');
          }
          trackEvent('name_input', `${cfg.type}|len=${nameInput.value.length}`);
        });
      }

      // Focus/blur tracking
      if (nameInput) {
        nameInput.addEventListener('focus', () => trackEvent('input_focus', `${cfg.type}|name`));
        nameInput.addEventListener('blur',  () => trackEvent('input_blur',  `${cfg.type}|name`));
      }
      if (phoneInput) {
        phoneInput.addEventListener('focus', () => trackEvent('input_focus', `${cfg.type}|phone`));
        phoneInput.addEventListener('blur',  () => trackEvent('input_blur',  `${cfg.type}|phone`));
      }

form.addEventListener('submit', async e => {
        e.preventDefault();
        if (DEBUG) _log('form submit:', cfg.id, cfg.type);
        if (form.dataset.submitting === '1') return;
        const rawName = nameInput ? nameInput.value.trim() : '';
        const phone = phoneInput ? phoneInput.value.trim() : '';

        // Client-side validation
        let ok = true;
        let name = rawName; // mặc định, sẽ set lại khi pass validation
        // Name: bắt buộc, tối thiểu 2 ký tự (validation nhẹ — không ép đầy đủ Họ Tên)
        if (rawName.length < 2) {
          if (nameInput) {
            nameInput.classList.add('error');
            shake(nameInput);
            trackEvent('validation_error', `${cfg.type}|name|${rawName.length === 0 ? 'empty' : 'too_short'}`);
          }
          ok = false;
        } else if (rawName.length > 80) {
          if (nameInput) {
            nameInput.classList.add('error');
            shake(nameInput);
            trackEvent('validation_error', `${cfg.type}|name|too_long`);
          }
          ok = false;
        } else {
          name = rawName;
        }
        // Phone: bắt buộc. Validation theo đầu số Việt Nam.
        const phoneValid = validateVietnamPhone(phone);
        if (!phoneValid) {
          if (phoneInput) {
            shake(phoneInput);
            trackEvent('validation_error', `${cfg.type}|phone|invalid_vn`);
          }
          if (phoneHint) phoneHint.classList.remove('hidden');
          ok = false;
        }
        if (!ok) return;

        // Honeypot
        const honeypot = form.querySelector('input[name="website"]');
        if (honeypot && honeypot.value) return;

        form.dataset.submitting = '1';
        const submitBtn = form.querySelector('button[type="submit"]');
        if (submitBtn) {
          submitBtn.classList.add('btn-loading');
          submitBtn.dataset.originalHtml = submitBtn.innerHTML;
          submitBtn.innerHTML = '<span>ĐANG GỬI...</span>';
        }

        // === SINH IDEMPOTENCY KEY ===
        const idempotencyKey = generateUUID();
        const sessionId = getSessionId();
        const submitStartedAt = Date.now();

        // Build payload - GIỮ NGUYÊN 100% cấu trúc để không phá backend
        const payload = {
          name,
          phone,
          form_type: cfg.type,
          page_url: location.href,
          referrer: document.referrer || '',
          user_agent: navigator.userAgent,
          screen_res: `${screen.width}x${screen.height}`,
          viewport: `${window.innerWidth}x${window.innerHeight}`,
          device_type: /Mobi|Andr|iP(hone|ad|od)/i.test(navigator.userAgent) ? 'mobile' : 'desktop',
          session_id: sessionId,
          utm_source: qParam('utm_source'),
          utm_medium: qParam('utm_medium'),
          utm_campaign: qParam('utm_campaign'),
          utm_content: qParam('utm_content'),
          utm_term: qParam('utm_term'),
          fbclid: qParam('fbclid'),
          gclid: qParam('gclid')
        };

        trackEvent('form_submit_attempt', `${cfg.type}|idk=${idempotencyKey.slice(0,8)}`);

        const sendResult = await sendLead(idempotencyKey, payload);

        if (sendResult.ok) {
          try {
            window.ApexSafeStorage.set('apex_lead_id', 'ch_' + sendResult.id);
            window.ApexSafeStorage.set('apex_lead_name', name);
            window.ApexSafeStorage.set('apex_lead_phone', phone);
            // Lưu thêm metadata cho chucmung.html
            window.ApexSafeStorage.set('apex_lead_idk', idempotencyKey);
            window.ApexSafeStorage.set('apex_lead_form_type', cfg.type);
            window.ApexSafeStorage.set('apex_lead_sent_at', Date.now());
          } catch(_){}
          trackEvent('form_submit_success', `${cfg.type}|id=${sendResult.id}|dur=${Date.now() - submitStartedAt}ms`);

          // Hiển thị success state
          showSuccess(cfg, form, successEl);

          // Tạm tắt redirect để user test trực tiếp trên landing
          // scheduleRedirectToCongrats_(idempotencyKey, name, phone, cfg.type);
        }

        form.dataset.submitting = '0';
      });
    }

    /* ===========================================================
       VALIDATION SĐT VIỆT NAM - UPDATED v20
       Hỗ trợ các đầu số di động VN: 03, 05, 07, 08, 09
       Và định dạng quốc tế: +84
       Quy tắc:
         - 0XXXXXXXXX  → 10 chữ số
         - +84XXXXXXXXX → bắt đầu +84, tiếp theo 9 chữ số (tổng digits = 11)
         - 84XXXXXXXXX  → 11 chữ số (không có +)
       =========================================================== */
    function validateVietnamPhone(phone){
      if (!phone || typeof phone !== 'string') return false;
      const trimmed = phone.trim();
      if (trimmed.length < 10) return false;

      // Chuẩn hóa về dạng 0XXXXXXXXX để check đầu số + đếm chữ số
      let normalized = trimmed;
      if (trimmed.startsWith('+84')) {
        normalized = '0' + trimmed.slice(3); // +84XXX... → 0XXX...
      } else if (trimmed.startsWith('+')) {
        // Bắt đầu bằng + mà không phải +84 → sai
        return false;
      } else if (trimmed.startsWith('84') && trimmed.length >= 11) {
        // 84XXXXXXXXX không có + → chuyển thành 0XXXXXXXXX
        normalized = '0' + trimmed.slice(2);
      }

      // Sau chuẩn hóa phải bắt đầu bằng 0 và là toàn chữ số
      if (!/^0\d{9,10}$/.test(normalized)) return false;

      // Tách phần đầu (3 ký tự đầu: 0 + 2 số đầu của nhà mạng)
      const prefix = normalized.slice(0, 3); // VD: 098, 090, 037, 086...

      // Đầu số di động Việt Nam hợp lệ
      const validPrefixes = [
        '032', '033', '034', '035', '036', '037', '038', '039', // Viettel
        '052', '056', '058',                                       // Vietnamobile
        '070', '076', '077', '078', '079',                          // MobiFone
        '081', '082', '083', '084', '085', '086', '087', '088', '089', // Vinaphone
        '090', '091', '092', '093', '094', '095', '096', '097', '098', '099' // MobiFone + Viettel
      ];

      if (!validPrefixes.includes(prefix)) return false;

      // Tổng chữ số trong chuỗi gốc:
      //   - 0XXXXXXXXX → 10 chữ số
      //   - 84XXXXXXXXX (không +) → 11 chữ số
      //   - +84XXXXXXXXX → 11 chữ số (bỏ dấu +)
      const digits = trimmed.replace(/\D/g, '');
      const isIntl = trimmed.startsWith('+84') || trimmed.startsWith('84');
      const expectedDigits = isIntl ? 11 : 10;
      if (digits.length !== expectedDigits) return false;

      return true;
    }

    /* ===========================================================
       CORE: GỬI LEAD → APEX BACKEND (POST /api/leads)
       ------------------------------------------------------------
       Bản gốc (2026-11-11) là "FRONTEND-ONLY TEST MODE": chỉ ghi log console
       và sessionStorage ⇒ lead KHÔNG vào CRM, KHÔNG có analytics, KHÔNG có
       chống bot PoW.

       Đã thay bằng lời gọi thật qua `window.ApexSheetsDirect.send()` — adapter
       `assets/js/apex-adapter.js` chuyển tiếp tới ApexTracker.sendLeadToBackend()
       ⇒ có PoW + honeypot + dedupe + geo + đồng bộ Google Sheets theo pipeline
       chuẩn của Apex.
       =========================================================== */
    function sendLead(idempotencyKey, payload) {
      if (DEBUG) console.log('[Apex Frontend Form Submit]:', payload);

      var enriched = Object.assign({}, payload, { idempotency_key: idempotencyKey });

      // Ưu tiên adapter của Apex (nếu đã nạp)
      if (window.ApexSheetsDirect && typeof window.ApexSheetsDirect.send === 'function') {
        return window.ApexSheetsDirect.send(enriched)
          .then(function (res) {
            if (res && res.ok) {
              return { ok: true, id: (res.id || 'lead_' + Date.now()) };
            }
            return { ok: false, error: (res && res.error) || 'send_failed' };
          })
          .catch(function (err) {
            if (DEBUG) console.error('[Apex] lỗi gửi lead:', err);
            return { ok: false, error: String(err) };
          });
      }

      // Fallback: chưa có adapter (preview offline) ⇒ báo lỗi rõ ràng,
      // KHÔNG giả vờ thành công để không mất lead âm thầm.
      return Promise.resolve({ ok: false, error: 'apex_adapter_not_loaded' });
    }

    /* ===========================================================
       REDIRECT → chucmung.html sau khi submit thành công.
       - Lưu data vào localStorage
       - Hiển thị loading overlay (1.2s) để user thấy success state
       - Chuyển trang với query params (idk + ts) để chucmung nhận diện
       - Có fallback: nếu user click nhiều lần, dùng sessionStorage flag
         để không redirect 2 lần
       =========================================================== */
    function scheduleRedirectToCongrats_(idempotencyKey, name, phone, formType) {
      // Tránh double-redirect
      try {
        if (sessionStorage.getItem('apex_redirecting') === '1') return;
        sessionStorage.setItem('apex_redirecting', '1');
      } catch (_) {}

      // Hiển thị fullscreen loading overlay
      showRedirectOverlay_(name);

      // Redirect sau 1.2s
      setTimeout(function () {
        var url = 'chucmung.html'
          + '?idk=' + encodeURIComponent(idempotencyKey)
          + '&form=' + encodeURIComponent(formType || 'hero')
          + '&ts=' + Date.now();
        if (DEBUG) _log('→ Redirecting to', url);
        window.location.href = url;
      }, 1200);
    }

    function showRedirectOverlay_(name) {
      // Nếu overlay đã tồn tại thì không tạo lại
      if (document.getElementById('apexRedirectOverlay')) return;

      var overlay = document.createElement('div');
      overlay.id = 'apexRedirectOverlay';
      overlay.setAttribute('role', 'status');
      overlay.setAttribute('aria-live', 'polite');
      overlay.style.cssText = [
        'position:fixed', 'inset:0', 'z-index:99999',
        'background:linear-gradient(135deg,#0A192F 0%,#1E293B 100%)',
        'display:flex', 'flex-direction:column',
        'align-items:center', 'justify-content:center',
        'color:#fff', 'padding:24px',
        'animation:apexOverlayIn 0.35s ease-out'
      ].join(';');

      overlay.innerHTML = [
        '<div style="text-align:center;max-width:420px;">',
        '  <div style="width:96px;height:96px;margin:0 auto 24px;',
        '    border-radius:50%;background:linear-gradient(135deg,#10b981,#059669);',
        '    display:flex;align-items:center;justify-content:center;',
        '    box-shadow:0 0 0 0 rgba(16,185,129,0.6);',
        '    animation:apexPulse 1.5s ease-out infinite;">',
        '    <svg width="56" height="56" viewBox="0 0 52 52" fill="none">',
        '      <path d="M14 27 L22 35 L38 17" stroke="#fff" stroke-width="5" stroke-linecap="round" stroke-linejoin="round" fill="none"/>',
        '    </svg>',
        '  </div>',
        '  <h2 style="font-size:28px;font-weight:900;margin:0 0 12px 0;line-height:1.2;">ĐĂNG KÝ THÀNH CÔNG!</h2>',
        '  <p style="color:#FFA94D;font-size:15px;margin:0 0 8px 0;font-weight:700;">Cảm ơn ' + escapeHtml_(name || 'bạn') + '</p>',
        '  <p style="color:rgba(255,255,255,0.85);font-size:14px;margin:0 0 24px 0;line-height:1.5;">Đang chuyển sang trang xác nhận...</p>',
        '  <div style="display:flex;gap:8px;justify-content:center;">',
        '    <div style="width:10px;height:10px;border-radius:50%;background:#FF6B00;animation:apexDot 1.2s ease-in-out infinite;"></div>',
        '    <div style="width:10px;height:10px;border-radius:50%;background:#FFA94D;animation:apexDot 1.2s ease-in-out 0.2s infinite;"></div>',
        '    <div style="width:10px;height:10px;border-radius:50%;background:#10b981;animation:apexDot 1.2s ease-in-out 0.4s infinite;"></div>',
        '  </div>',
        '</div>',
        '<style>',
        '@keyframes apexOverlayIn { from { opacity:0; } to { opacity:1; } }',
        '@keyframes apexPulse { 0% { box-shadow:0 0 0 0 rgba(16,185,129,0.6); } 70% { box-shadow:0 0 0 24px rgba(16,185,129,0); } 100% { box-shadow:0 0 0 0 rgba(16,185,129,0); } }',
        '@keyframes apexDot { 0%,80%,100% { transform:scale(0.6);opacity:0.5; } 40% { transform:scale(1);opacity:1; } }',
        '</style>'
      ].join('\n');

      document.body.appendChild(overlay);
    }

    function escapeHtml_(str) {
      if (str == null) return '';
      return String(str)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;');
    }

    /* ===========================================================
       4) MULTI-STEP FORM
       =========================================================== */
    function initMultiStepForm(){
      const form = document.getElementById('multiForm');
      if (!form) return;

      const channelInput = document.getElementById('multiChannel');
      const panes = form.querySelectorAll('[data-step-pane]');
      const indicators = document.querySelectorAll('[data-step-indicator]');
      const line = document.querySelector('[data-step-line]');

      const goToStep = (stepNum) => {
        panes.forEach(p => {
          if (parseInt(p.dataset.stepPane, 10) === stepNum) p.classList.add('is-active');
          else p.classList.remove('is-active');
        });
        indicators.forEach(ind => {
          const n = parseInt(ind.dataset.stepIndicator, 10);
          ind.classList.remove('is-active', 'is-done');
          if (n === stepNum) ind.classList.add('is-active');
          else if (n < stepNum) ind.classList.add('is-done');
        });
        if (line) {
          line.style.background = stepNum === 2
            ? 'linear-gradient(90deg, #FF6D00, #10B981)'
            : '#E5E7EB';
        }
        trackEvent('multistep_step_view', 'step_' + stepNum);
        setTimeout(() => {
          const firstInput = form.querySelector(`[data-step-pane="${stepNum}"] input:not([type=hidden])`);
          if (firstInput) firstInput.focus({ preventScroll: true });
        }, 200);
      };

      form.querySelectorAll('.channel-btn').forEach(btn => {
        btn.addEventListener('click', () => {
          const channel = btn.dataset.channel || '';
          const nextStep = parseInt(btn.dataset.nextStep, 10);
          if (channelInput) channelInput.value = channel;
          form.querySelectorAll('.channel-btn').forEach(b => b.classList.remove('is-selected'));
          btn.classList.add('is-selected');
          trackEvent('multistep_channel_selected', channel);
          setTimeout(() => goToStep(nextStep), 250);
        });
      });

      form.querySelectorAll('[data-prev-step]').forEach(btn => {
        btn.addEventListener('click', () => {
          const prev = parseInt(btn.dataset.prevStep, 10);
          goToStep(prev);
          trackEvent('multistep_back_clicked', 'from_step_2');
        });
      });

      const nameInput = document.getElementById('multiName');
      const phoneInput = document.getElementById('multiPhone');
      const phoneHint = document.getElementById('multiPhoneHint');
      const successEl = document.getElementById('multiSuccess');

      if (phoneInput) {
        phoneInput.addEventListener('input', () => {
          let v = phoneInput.value;
          v = v.replace(/[^\d+]/g, '');
          v = v.replace(/\+/g, (m, offset) => (offset === 0 ? '+' : ''));
          if (v.length > 13) v = v.slice(0, 13);
          phoneInput.value = v;
          if (v.length > 0) {
            phoneInput.classList.remove('error');
            if (phoneHint) phoneHint.classList.add('hidden');
          }
          trackEvent('phone_input', `multistep|len=${v.length}`);
        });
      }
      if (nameInput) {
        nameInput.addEventListener('input', () => {
          if (nameInput.value.length > 0) nameInput.classList.remove('error');
          trackEvent('name_input', `multistep|len=${nameInput.value.length}`);
        });
      }

form.addEventListener('submit', async e => {
        e.preventDefault();
        if (form.dataset.submitting === '1') return;
        const rawName = nameInput ? nameInput.value.trim() : '';
        const phone = phoneInput ? phoneInput.value.trim() : '';
        const channel = channelInput ? channelInput.value : '';

        let ok = true;
        let name = rawName; // mặc định
        // Name: bắt buộc, tối thiểu 2 ký tự (validation nhẹ — không ép đầy đủ Họ Tên)
        if (rawName.length < 2) {
          if (nameInput) {
            nameInput.classList.add('error');
            shake(nameInput);
            trackEvent('validation_error', `multistep|name|${rawName.length === 0 ? 'empty' : 'too_short'}`);
          }
          ok = false;
        } else if (rawName.length > 80) {
          if (nameInput) {
            nameInput.classList.add('error');
            shake(nameInput);
            trackEvent('validation_error', `multistep|name|too_long`);
          }
          ok = false;
        } else {
          name = rawName;
        }
        // Phone: dùng validateVietnamPhone (đầu số VN + +84)
        if (!validateVietnamPhone(phone)) {
          if (phoneInput) shake(phoneInput);
          if (phoneHint) phoneHint.classList.remove('hidden');
          trackEvent('validation_error', `multistep|phone|invalid_vn`);
          ok = false;
        }
        if (!ok) return;

        const honeypot = form.querySelector('input[name="website"]');
        if (honeypot && honeypot.value) return;

        form.dataset.submitting = '1';
        const submitBtn = form.querySelector('button[type="submit"]');
        if (submitBtn) {
          submitBtn.classList.add('btn-loading');
          submitBtn.innerHTML = '<span>ĐANG GỬI...</span>';
        }

        const idempotencyKey = generateUUID();
        const sessionId = getSessionId();
        const submitStartedAt = Date.now();

        const payload = {
          name, phone,
          form_type: 'multistep',
          channel,
          page_url: location.href,
          referrer: document.referrer || '',
          user_agent: navigator.userAgent,
          screen_res: `${screen.width}x${screen.height}`,
          viewport: `${window.innerWidth}x${window.innerHeight}`,
          device_type: /Mobi|Andr|iP(hone|ad|od)/i.test(navigator.userAgent) ? 'mobile' : 'desktop',
          session_id: sessionId,
          utm_source: qParam('utm_source'),
          utm_medium: qParam('utm_medium'),
          utm_campaign: qParam('utm_campaign'),
          utm_content: qParam('utm_content'),
          utm_term: qParam('utm_term'),
          fbclid: qParam('fbclid'),
          gclid: qParam('gclid')
        };

        trackEvent('multistep_submit_attempt', `${channel}|idk=${idempotencyKey.slice(0,8)}`);

        const sendResult = await sendLead(idempotencyKey, payload);

        if (sendResult.ok) {
          try {
            window.ApexSafeStorage.set('apex_lead_id', 'ch_' + sendResult.id);
            window.ApexSafeStorage.set('apex_lead_name', name);
            window.ApexSafeStorage.set('apex_lead_phone', phone);
            window.ApexSafeStorage.set('apex_lead_idk', idempotencyKey);
            window.ApexSafeStorage.set('apex_lead_form_type', 'multistep');
            window.ApexSafeStorage.set('apex_lead_channel', channel);
            window.ApexSafeStorage.set('apex_lead_sent_at', Date.now());
          } catch(_){}
          trackEvent('multistep_submit_success', `${channel}|id=${sendResult.id}|dur=${Date.now() - submitStartedAt}ms`);

          showMultiSuccess(form, successEl);

          // Tạm tắt redirect để user test trực tiếp trên landing
          // scheduleRedirectToCongrats_(idempotencyKey, name, phone, 'multistep');
        }

        form.dataset.submitting = '0';
      });

      function showMultiSuccess(form, successEl){
        form.style.display = 'none';
        if (successEl) successEl.classList.remove('hidden');
        const indicator = document.querySelector('.step-indicator');
        if (indicator) indicator.style.display = 'none';
      }

      trackEvent('multistep_view', 'step_1');
    }

    /* ===========================================================
       5) Smooth scroll
       =========================================================== */
    function initSmoothScroll(){
      document.querySelectorAll('a[href^="#"]').forEach(a => {
        a.addEventListener('click', e => {
          const href = a.getAttribute('href');
          if (!href || href === '#' || href.length < 2) return;
          const target = document.querySelector(href);
          if (target) {
            e.preventDefault();
            const headerH = 64;
            const top = target.getBoundingClientRect().top + window.pageYOffset - headerH;
            window.scrollTo({ top, behavior: 'smooth' });
            trackEvent('anchor_click', href);
          }
        });
      });
    }

    /* ===========================================================
       6) Sticky CTA visibility
       =========================================================== */
    function initStickyCtaVisibility(){
      const sticky = document.getElementById('stickyCta');
      if (!sticky) return;
      document.addEventListener('focusin', e => {
        const tag = (e.target.tagName || '').toLowerCase();
        if (tag === 'input' || tag === 'textarea' || tag === 'select') {
          document.body.classList.add('form-focused');
          sticky.classList.add('is-hidden');
        }
      });
      document.addEventListener('focusout', () => {
        setTimeout(() => {
          const ae = document.activeElement;
          const tag = (ae && ae.tagName || '').toLowerCase();
          if (tag !== 'input' && tag !== 'textarea' && tag !== 'select') {
            document.body.classList.remove('form-focused');
            sticky.classList.remove('is-hidden');
          }
        }, 150);
      });
      if (window.visualViewport) {
        const check = () => {
          const shrunk = window.innerHeight - window.visualViewport.height;
          if (shrunk > 120) {
            document.body.classList.add('form-focused');
            sticky.classList.add('is-hidden');
          }
        };
        window.visualViewport.addEventListener('resize', check);
        window.visualViewport.addEventListener('scroll', check);
      }
    }

    /* ===========================================================
       7) Real-time counters
       =========================================================== */
    function initSeatsCounter(){
      const els = [document.getElementById('seatsLeftTop'), document.getElementById('seatsLeftBottom')].filter(Boolean);
      if (!els.length) return;
      let current = parseInt(els[0].textContent, 10) || 8;
      const minSeats = 1;
      setInterval(() => {
        if (Math.random() < 0.35 && current > minSeats) {
          current--;
          els.forEach(el => el.textContent = current);
          trackEvent('seats_decreased', current);
        }
      }, 30000);
    }

    function initGiftsCounter(){
      const givenEl = document.getElementById('giftGiven');
      const totalEl = document.getElementById('giftTotal');
      const remEl = document.getElementById('giftRemaining');
      const bar = document.getElementById('giftBar');
      if (!givenEl || !bar) return;
      const total = parseInt(totalEl.textContent, 10) || 50;
      let given = parseInt(givenEl.textContent, 10) || 42;
      setInterval(() => {
        if (given < total && Math.random() < 0.3) {
          given++;
          givenEl.textContent = given;
          if (remEl) remEl.textContent = total - given;
          const pct = Math.min(100, Math.round((given / total) * 100));
          bar.style.width = pct + '%';
          trackEvent('gift_increased', given);
        }
      }, 45000);
    }

    /* ===========================================================
       7.5) LỊCH BUỔI CHIA SẺ TỰ ĐỘNG (theo tuần)
       - Mặc định: Thứ Ba hàng tuần lúc 20:00 (giờ địa phương)
       - Tự xác định buổi sắp tới, không cần sửa code mỗi tuần
       =========================================================== */
    var SESSION_DAY_DEFAULT = 2;   // 0=CN, 1=T2, 2=T3 (Thứ Ba), 3=T4, 4=T5, 5=T6, 6=T7
    var SESSION_HOUR_DEFAULT = 20; // 20:00
    var SESSION_MIN_DEFAULT = 0;
    var WEEKDAY_VN = ['Chủ Nhật','Thứ Hai','Thứ Ba','Thứ Tư','Thứ Năm','Thứ Sáu','Thứ Bảy'];

    // Cấu hình theo từng trang: <body data-session-day="2" data-session-hour="20">
    function sessionConfig(){
      const b = document.body;
      const d = parseInt(b?.dataset?.sessionDay, 10);
      const h = parseInt(b?.dataset?.sessionHour, 10);
      const m = parseInt(b?.dataset?.sessionMin, 10);
      return {
        day:  isFinite(d) && d >= 0 && d <= 6 ? d : SESSION_DAY_DEFAULT,
        hour: isFinite(h) && h >= 0 && h <= 23 ? h : SESSION_HOUR_DEFAULT,
        min:  isFinite(m) && m >= 0 && m <= 59 ? m : SESSION_MIN_DEFAULT
      };
    }

    // Buổi gần nhất (kể cả khi đang diễn ra) — luôn là Thứ Ba 20:00 tuần này
    function getNearestSession(from){
      const cfg = sessionConfig();
      const base = from ? new Date(from.getTime()) : new Date();
      const t = new Date(base.getFullYear(), base.getMonth(), base.getDate(), cfg.hour, cfg.min, 0, 0);
      let add = (cfg.day - t.getDay() + 7) % 7;
      t.setDate(t.getDate() + add);
      if (t.getTime() <= base.getTime()) {
        t.setDate(t.getDate() + 7);
      }
      return t;
    }

    // Buổi kế tiếp (luôn ở tương lai)
    function getNextSession(){
      const now = new Date();
      const t = getNearestSession(now);
      if (t.getTime() <= now.getTime()) t.setDate(t.getDate() + 7);
      return t;
    }

    // Buổi vừa diễn ra gần nhất (bắt đầu trong quá khứ, chưa kết thúc)
    function getCurrentSession(from){
      const cfg = sessionConfig();
      const base = from ? new Date(from.getTime()) : new Date();
      const t = new Date(base.getFullYear(), base.getMonth(), base.getDate(), cfg.hour, cfg.min, 0, 0);
      let back = (t.getDay() - cfg.day + 7) % 7;
      t.setDate(t.getDate() - back);
      // Nếu buổi đó đã kết thúc (> 2 tiếng) → buổi vừa diễn ra vẫn là buổi đó
      return t;
    }

    function formatVnDate(d){
      const p = n => String(n).padStart(2, '0');
      return p(d.getDate()) + '/' + p(d.getMonth() + 1) + '/' + d.getFullYear();
    }

    // Public API cho các trang khác dùng chung
    window.ApexSchedule = {
      next: getNextSession,
      nearest: getNearestSession,
      current: getCurrentSession,
      format: formatVnDate,
      weekday: function(d){ return WEEKDAY_VN[d.getDay()]; }
    };

    /* ===========================================================
       8) COUNTDOWN TIMER (tự động theo lịch tuần)
       - Target: buổi Thứ Ba 20:00 gần nhất / kế tiếp
       - Hỗ trợ nhiều bộ countdown trong 1 trang qua [data-cd="days|hours|mins|secs"]
       - Tự cập nhật label ngày giờ & tên thứ trong [data-session-date]
       =========================================================== */
    function initCountdownTimer(){
      // Hỗ trợ cả markup mới (data-cd) và markup cũ (id="cd-days"...)
      const daysEl = document.querySelector('[data-cd="days"]') || document.getElementById('cd-days');
      if (!daysEl) return;

      const pad = n => String(n).padStart(2, '0');
      const q  = sel => document.querySelectorAll(sel);
      const pick = (attr, legacyId) => {
        const list = q('[' + attr + ']');
        if (list.length) return list;
        const legacy = document.getElementById(legacyId);
        return legacy ? [legacy] : [];
      };
      const els = {
        days:  pick('data-cd="days"',  'cd-days'),
        hours: pick('data-cd="hours"', 'cd-hours'),
        mins:  pick('data-cd="mins"',  'cd-mins'),
        secs:  pick('data-cd="secs"',  'cd-secs')
      };
      const prev = { days: null, hours: null, mins: null, secs: null };

      // Đồng hồ góc phải (nếu có) — điền khi trang vừa mở
      const next = getNextSession();
      const cfg = sessionConfig();
      const hhmm = pad(cfg.hour) + ':' + pad(cfg.min);
      q('[data-session-date]').forEach(el => {
        el.textContent = formatVnDate(next);
      });
      q('[data-session-full]').forEach(el => {
        el.textContent = WEEKDAY_VN[next.getDay()] + ', ' + formatVnDate(next) + ' lúc ' + hhmm;
      });

      // Trạng thái buổi chia sẻ: sắp tới / đang diễn ra
      const updateStatus = () => {
        const now = new Date();
        const cur = getCurrentSession(now);
        const end = cur.getTime() + 2 * 3600000;
        const live = now.getTime() >= cur.getTime() && now.getTime() < end;
        q('[data-session-status]').forEach(el => {
          el.textContent = live ? 'ĐANG DIỄN RA' : 'SẮP DIỄN RA';
          el.classList.toggle('is-live', live);
        });
      };
      updateStatus();
      setInterval(updateStatus, 30000);

      const setText = (list, val) => {
        list.forEach(el => {
          if (el.textContent !== val) {
            el.textContent = val;
            // Hiệu ứng lật số
            el.classList.remove('cd-flip');
            void el.offsetWidth;
            el.classList.add('cd-flip');
          }
        });
      };

      const tick = () => {
        const now = Date.now();
        const cur = getCurrentSession(new Date(now));
        const end = cur.getTime() + 2 * 3600000;   // buổi dài 2 tiếng
        const live = now >= cur.getTime() && now < end;

        // Đang diễn ra → đếm về giờ kết thúc; chưa bắt đầu → đếm về giờ bắt đầu
        const target = live ? end : getNextSession().getTime();
        const diff = Math.max(0, target - now);

        const d = Math.floor(diff / 86400000);
        const h = Math.floor((diff % 86400000) / 3600000);
        const m = Math.floor((diff % 3600000) / 60000);
        const s = Math.floor((diff % 60000) / 1000);

        setText(els.days,  pad(d));
        setText(els.hours, pad(h));
        setText(els.mins,  pad(m));
        setText(els.secs,  pad(s));

        // Nhãn "Bắt đầu sau" / "Kết thúc sau"
        q('[data-cd-label]').forEach(el => {
          el.textContent = live ? 'Kết thúc sau:' : 'Còn:';
        });
        q('[data-cd-modal-label]').forEach(el => {
          el.textContent = live ? 'BUỔI CHIA SẺ ĐANG DIỄN RA' : 'BUỔI CHIA SẺ SẮP DIỄN RA';
        });

        // Khi buổi vừa kết thúc → làm mới ngày hiển thị
        if (diff <= 0) {
          const nn = getNextSession();
          q('[data-session-date]').forEach(el => { el.textContent = formatVnDate(nn); });
          q('[data-session-full]').forEach(el => {
            el.textContent = WEEKDAY_VN[nn.getDay()] + ', ' + formatVnDate(nn) + ' lúc ' + hhmm;
          });
        }
      };

      tick();
      setInterval(tick, 1000);
    }

    /* ===========================================================
       9) NEW v20 - VIDEO PLAYER (lazy-load)
       - Dùng <video poster=...> chuẩn của HTML5 (browser tự render poster)
       - Lazy-load <source>: chỉ khi user nhấn Play mới inject vào <video>
       - Custom controls: Play/Pause, Mute/Unmute, Progress, Fullscreen
       - Fallback: nếu source không load được hoặc file chưa tồn tại
                   → giữ poster + mở modal đăng ký
       =========================================================== */
    function initVideoPlayer(){
      const video = document.getElementById('apexVideo');
      const overlay = document.getElementById('videoOverlay');
      const playBtn = document.getElementById('videoPlayBtn');
      const overlaySub = document.getElementById('videoOverlaySub');
      const controls = document.getElementById('videoControls');
      const vcTogglePlay = document.getElementById('vcTogglePlay');
      const vcIconPause = document.getElementById('vcIconPause');
      const vcIconPlay = document.getElementById('vcIconPlay');
      const vcMute = document.getElementById('vcMute');
      const vcIconUnmute = document.getElementById('vcIconUnmute');
      const vcIconMute = document.getElementById('vcIconMute');
      const vcProgress = document.getElementById('vcProgress');
      const vcProgressFill = document.getElementById('vcProgressFill');
      const vcTime = document.getElementById('vcTime');
      const vcFullscreen = document.getElementById('vcFullscreen');

      if (!video || !playBtn) return;

      // Cấu hình: đường dẫn video khi deploy (bản local độc lập: ./assets/video/...)
      const VIDEO_SRC = video.getAttribute('data-src') || 'assets/video/video_gioi_thieu_apex.mp4';

      const fmtTime = s => {
        if (!isFinite(s) || s < 0) return '00:00';
        const m = Math.floor(s / 60);
        const sec = Math.floor(s % 60);
        return String(m).padStart(2, '0') + ':' + String(sec).padStart(2, '0');
      };

      const updatePlayIcon = () => {
        if (video.paused) {
          if (vcIconPause) vcIconPause.style.display = 'none';
          if (vcIconPlay)  vcIconPlay.style.display = '';
        } else {
          if (vcIconPause) vcIconPause.style.display = '';
          if (vcIconPlay)  vcIconPlay.style.display = 'none';
        }
      };

      const updateMuteIcon = () => {
        if (video.muted) {
          if (vcIconUnmute) vcIconUnmute.style.display = 'none';
          if (vcIconMute)   vcIconMute.style.display = '';
        } else {
          if (vcIconUnmute) vcIconUnmute.style.display = '';
          if (vcIconMute)   vcIconMute.style.display = 'none';
        }
      };

      const hideOverlay = () => {
        if (overlay) overlay.classList.add('hidden');
        if (controls) controls.style.display = 'flex';
      };

      const showRegistrationModal = () => {
        if (window.ApexModal) {
          window.ApexModal.open({ dataset: { ctaLabel: 'video_placeholder' } });
        }
      };

      // Kiểm tra xem có source video thật chưa bằng HEAD request
      let videoExists = false;
      const checkVideoExists = async () => {
        try {
          const head = await fetch(VIDEO_SRC, { method: 'HEAD' });
          videoExists = head.ok;
        } catch (e) {
          videoExists = false;
        }

        if (!videoExists) {
          // File chưa tồn tại → thông báo "Video đang cập nhật"
          if (overlaySub) {
            overlaySub.textContent = '🎬 Video giới thiệu đang được cập nhật – Đăng ký để nhận video ngay khi ra mắt';
            overlaySub.style.maxWidth = '600px';
            overlaySub.style.padding = '0 1rem';
          }
          // Vẫn giữ poster + nút Play, nhưng click → mở modal đăng ký
          playBtn.style.opacity = '0.6';
        }
      };
      checkVideoExists();

      const startPlay = () => {
        if (!videoExists) {
          // Mở modal đăng ký thay vì play video
          showRegistrationModal();
          return;
        }
        // Inject source nếu chưa có
        if (!video.querySelector('source')) {
          const sourceEl = document.createElement('source');
          sourceEl.src = VIDEO_SRC;
          sourceEl.type = 'video/mp4';
          video.appendChild(sourceEl);
          video.load();
        }
        hideOverlay();
        const tryPlay = () => {
          video.play().then(() => updatePlayIcon()).catch(err => {
            _warn('play error:', err);
            // Nếu play fail (autoplay blocked hoặc file lỗi) → hiện lại overlay
            if (overlay) overlay.classList.remove('hidden');
          });
        };
        if (video.readyState >= 2) tryPlay();
        else {
          video.addEventListener('loadeddata', tryPlay, { once: true });
          // Timeout fallback: nếu 5s không load được → fallback
          setTimeout(() => {
            if (video.readyState < 2) {
              _warn('load timeout, fallback');
              if (overlay) overlay.classList.remove('hidden');
            }
          }, 5000);
        }
      };

      playBtn.addEventListener('click', e => {
        e.stopPropagation();
        startPlay();
      });
      if (overlay) {
        overlay.addEventListener('click', e => {
          if (e.target === overlay) startPlay();
        });
      }

      // Custom controls
      if (vcTogglePlay) {
        vcTogglePlay.addEventListener('click', () => {
          if (video.paused) {
            video.play().catch(() => {});
          } else {
            video.pause();
          }
        });
      }
      video.addEventListener('play', updatePlayIcon);
      video.addEventListener('pause', updatePlayIcon);

      if (vcMute) {
        vcMute.addEventListener('click', () => {
          video.muted = !video.muted;
          updateMuteIcon();
        });
      }
      video.addEventListener('volumechange', updateMuteIcon);

      video.addEventListener('timeupdate', () => {
        if (vcProgressFill && isFinite(video.duration) && video.duration > 0) {
          const pct = (video.currentTime / video.duration) * 100;
          vcProgressFill.style.width = pct + '%';
        }
        if (vcTime) vcTime.textContent = fmtTime(video.currentTime);
      });

      video.addEventListener('loadedmetadata', () => {
        if (vcTime) vcTime.textContent = fmtTime(video.currentTime) + ' / ' + fmtTime(video.duration);
      });

      if (vcProgress) {
        vcProgress.addEventListener('click', e => {
          if (!isFinite(video.duration) || video.duration <= 0) return;
          const rect = vcProgress.getBoundingClientRect();
          const pct = (e.clientX - rect.left) / rect.width;
          video.currentTime = Math.max(0, Math.min(video.duration, pct * video.duration));
        });
      }

      if (vcFullscreen) {
        vcFullscreen.addEventListener('click', () => {
          const frame = video.closest('.video-frame');
          if (!frame) return;
          if (document.fullscreenElement) {
            document.exitFullscreen();
          } else if (frame.requestFullscreen) {
            frame.requestFullscreen();
          }
        });
      }

      const frameWrap = video.closest('.video-frame');
      if (frameWrap) {
        frameWrap.addEventListener('mouseenter', () => {
          if (controls && !video.paused) controls.classList.add('show');
        });
        frameWrap.addEventListener('mouseleave', () => {
          if (controls && !video.paused) controls.classList.remove('show');
        });
      }
    }

    /* ===========================================================
       10) NEW v20 - EVIDENCE CAROUSEL
       - Kéo ngang với nút prev/next + dots
       - Hỗ trợ cả touch swipe và mouse wheel
       =========================================================== */
    function initEvidenceCarousel(){
      const track = document.getElementById('evidenceTrack');
      const viewport = document.getElementById('evidenceViewport');
      const prevBtn = document.getElementById('evPrev');
      const nextBtn = document.getElementById('evNext');
      const dotsWrap = document.getElementById('evidenceDots');
      if (!track || !viewport) return;

      const cards = track.querySelectorAll('.evidence-card');
      if (!cards.length) return;

      let index = 0;
      const getVisibleCount = () => {
        if (window.innerWidth < 640) return 1;
        if (window.innerWidth < 1024) return 2;
        return 3;
      };

      const maxIndex = () => Math.max(0, cards.length - getVisibleCount());

      // Build dots
      const buildDots = () => {
        if (!dotsWrap) return;
        dotsWrap.innerHTML = '';
        const n = maxIndex() + 1;
        for (let i = 0; i < n; i++) {
          const d = document.createElement('button');
          d.type = 'button';
          d.className = 'evidence-dot' + (i === index ? ' is-active' : '');
          d.setAttribute('aria-label', 'Trang ' + (i + 1));
          d.addEventListener('click', () => goTo(i));
          dotsWrap.appendChild(d);
        }
      };

      // Tính cardWidth theo viewport (responsive)
      const getCardWidth = () => {
        if (cards.length === 0) return 0;
        // Lấy width thực tế của card đầu tiên sau khi CSS đã apply
        const rect = cards[0].getBoundingClientRect();
        return rect.width + 16; // +16 = gap 1rem
      };

      const update = () => {
        const cardWidth = getCardWidth();
        track.style.transform = `translateX(${-index * cardWidth}px)`;
        if (prevBtn) prevBtn.disabled = index <= 0;
        if (nextBtn) nextBtn.disabled = index >= maxIndex();
        if (dotsWrap) {
          dotsWrap.querySelectorAll('.evidence-dot').forEach((d, i) => {
            d.classList.toggle('is-active', i === index);
          });
        }
      };

      const goTo = i => {
        index = Math.max(0, Math.min(maxIndex(), i));
        update();
      };

      if (prevBtn) prevBtn.addEventListener('click', () => goTo(index - 1));
      if (nextBtn) nextBtn.addEventListener('click', () => goTo(index + 1));

      // Mouse wheel: chuyển slide
      viewport.addEventListener('wheel', e => {
        if (Math.abs(e.deltaY) > Math.abs(e.deltaX)) {
          e.preventDefault();
          if (e.deltaY > 0) goTo(index + 1);
          else goTo(index - 1);
        }
      }, { passive: false });

      // Touch swipe
      let touchStartX = 0;
      let touchDeltaX = 0;
      viewport.addEventListener('touchstart', e => {
        touchStartX = e.touches[0].clientX;
        touchDeltaX = 0;
      }, { passive: true });
      viewport.addEventListener('touchmove', e => {
        touchDeltaX = e.touches[0].clientX - touchStartX;
      }, { passive: true });
      viewport.addEventListener('touchend', () => {
        if (Math.abs(touchDeltaX) > 50) {
          if (touchDeltaX < 0) goTo(index + 1);
          else goTo(index - 1);
        }
      });

      // Resize
      let resizeT;
      window.addEventListener('resize', () => {
        clearTimeout(resizeT);
        resizeT = setTimeout(() => {
          if (index > maxIndex()) index = maxIndex();
          buildDots();
          update();
        }, 150);
      });

      buildDots();
      // Đợi 1 frame để CSS apply xong rồi mới tính width
      requestAnimationFrame(() => update());
      // Backup nếu ảnh load chậm
      window.addEventListener('load', () => update());
    }

    /* ===========================================================
       UI helpers
       =========================================================== */
    function showSuccess(cfg, form, successEl){
      if (!successEl) return;

      const isModal = cfg.type === 'modal';
      // Với modal, header nằm NGOÀI <form> → phải tìm trong .modal-glow-inner
      const scope = isModal ? (form.closest('.modal-glow-inner') || form) : form;

      // 1) Ẩn header (tiêu đề modal / header card)
      const header = scope.querySelector('.form-card-header, .modal-header-new');
      if (header) header.style.display = 'none';

      // 2) Ẩn phần nhập liệu
      if (form.contains(successEl)) {
        // Success nằm BÊN TRONG form (hero form) → ẩn các khối con khác, giữ form hiển thị
        Array.prototype.forEach.call(form.children, function (child) {
          if (child === successEl || child.contains(successEl)) return;
          child.style.display = 'none';
        });
        form.style.display = '';
      } else {
        // Success nằm NGOÀI form (modal) → ẩn hẳn form, modal chỉ còn màn thành công
        form.style.display = 'none';
      }

      // 3) Hiện màn thành công
      successEl.classList.remove('hidden');

      // 4) Ẩn phone hint nếu đang hiển thị
      const hint = document.getElementById(cfg.phoneHintId);
      if (hint) hint.classList.add('hidden');

      if (isModal && window.ApexModal) {
        setTimeout(() => window.ApexModal.close(), 2200);
      }
    }

    // Form Helpers (rung, validate, trackEvent, ids)
    function shake(el){
      el.classList.add('error');
      setTimeout(() => el.classList.remove('error'), 400);
    }

    /* ===========================================================
       Generic helpers
       =========================================================== */
    function qParam(name){
      const m = location.search.match(new RegExp('[?&]' + name + '=([^&#]*)'));
      return m ? decodeURIComponent(m[1]) : '';
    }

    function trackEvent(type, value){
      try {
        if (window.ApexTracker && typeof window.ApexTracker.send === 'function') {
          window.ApexTracker.send(type, value);
        }
      } catch(_){}
    }

    function generateUUID(){
      if (window.crypto && crypto.randomUUID) return crypto.randomUUID();
      return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, c => {
        const r = (Math.random() * 16) | 0;
        return (c === 'x' ? r : (r & 0x3) | 0x8).toString(16);
      });
    }

    function getSessionId(){
      try {
        const m = document.cookie.match(/(?:^|;\s*)apex_sid=([^;]+)/);
        if (m) return decodeURIComponent(m[1]);
      } catch(_){}
      return '';
    }

    // Kick off init AFTER all declarations
    ready(init);

  })();
