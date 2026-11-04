/**
 * Apex Multi-Landing Behavioral Analytics & Full Tracking SDK
 * - Phân tích chi tiết hành vi người dùng (Time on page, Scroll depth, Clicks, Form inputs, Timeline)
 * - Hệ thống phát hiện Bot / Crawler đa tầng (Client Heuristics & Server Verification)
 * - Tự động nạp cấu hình đa Landing Page qua config.json
 * - Quản lý Cookie 180 ngày cho khách đã đăng ký thành công (Deduping)
 * - Gửi dữ liệu về PostgreSQL chuyên dụng cho báo cáo thống kê
 */
(function (window, document) {
    'use strict';

    // ============================================================
    // 1. KHỞI TẠO ĐỊNH DANH (VISITOR ID & SESSION ID)
    // ============================================================
    const pageLoadTimestamp = Date.now();
    let maxScrollDepth = 0;
    let totalClicksCount = 0;
    let hasInteractedWithForm = false;
    let currentPhone = '';

    // Lấy hoặc tạo Visitor ID (lưu bền vững 365 ngày qua Cookie + LocalStorage)
    function getOrCreateVisitorId() {
        let vid = '';
        try {
            const match = document.cookie.match(/(?:^|;\s*)apex_vid=([^;]+)/);
            if (match && match[1]) {
                vid = decodeURIComponent(match[1]);
            }
            if (!vid && window.localStorage) {
                vid = localStorage.getItem('apex_vid') || '';
            }
        } catch (_) {}

        if (!vid) {
            vid = 'v_' + Math.random().toString(36).substring(2, 10) + '_' + Date.now().toString(36);
        }

        // Lưu lại cho 365 ngày
        try {
            const expires = new Date(Date.now() + 365 * 24 * 60 * 60 * 1000).toUTCString();
            document.cookie = 'apex_vid=' + encodeURIComponent(vid) + '; max-age=31536000; expires=' + expires + '; path=/; SameSite=Lax';
            if (window.localStorage) {
                localStorage.setItem('apex_vid', vid);
            }
        } catch (_) {}

        return vid;
    }

    // Lấy hoặc tạo Session ID (lưu trong sessionStorage cho tab hiện tại)
    function getOrCreateSessionId() {
        let sid = '';
        try {
            sid = sessionStorage.getItem('apex_session_id');
            if (!sid) {
                sid = 'sess_' + Math.random().toString(36).substring(2, 10) + '_' + Date.now().toString(36);
                sessionStorage.setItem('apex_session_id', sid);
            }
        } catch (_) {
            sid = 'sess_' + Math.random().toString(36).substring(2, 10) + '_' + Date.now().toString(36);
        }
        return sid;
    }

    const visitorId = getOrCreateVisitorId();
    const sessionId = getOrCreateSessionId();

    // ============================================================
    // 2. PHÁT HIỆN BOT / CRAWLER (DEEP CLIENT-SIDE BOT DETECTION)
    // ============================================================
    let mouseMoveCount = 0;
    let touchMoveCount = 0;
    let keyPressCount = 0;
    let scrollEventCount = 0;

    window.addEventListener('mousemove', function () { mouseMoveCount++; }, { passive: true, once: false });
    window.addEventListener('touchstart', function () { touchMoveCount++; }, { passive: true, once: false });
    window.addEventListener('keydown', function () { keyPressCount++; }, { passive: true, once: false });

    function evaluateBotSignals() {
        const reasons = [];
        let score = 0;

        // 1. Kiểm tra cờ webdriver tự động hóa (Selenium, Puppeteer, Playwright)
        if (navigator.webdriver === true) {
            score += 80;
            reasons.push('navigator.webdriver_detected');
        }

        // 2. Kiểm tra biến môi trường headless phổ biến
        if (window.callPhantom || window._phantom || window.__nightmare || window.Buffer) {
            score += 90;
            reasons.push('headless_runtime_detected');
        }

        // 3. User agent chứa từ khóa headless
        if (/HeadlessChrome|Lighthouse|Prerender|PhantomJS/i.test(navigator.userAgent)) {
            score += 95;
            reasons.push('headless_ua_detected');
        }

        // 4. Kích thước màn hình bất thường
        if (window.screen.width === 0 || window.screen.height === 0 || window.screen.availWidth === 0) {
            score += 70;
            reasons.push('zero_screen_dimensions');
        }

        // 5. Ngôn ngữ trình duyệt rỗng
        if (!navigator.languages || navigator.languages.length === 0) {
            score += 40;
            reasons.push('empty_languages');
        }

        // 6. Sau 8 giây nhưng hoàn toàn không có tương tác người dùng nào
        const timeElapsed = (Date.now() - pageLoadTimestamp) / 1000;
        if (timeElapsed >= 8 && (mouseMoveCount + touchMoveCount + keyPressCount + scrollEventCount === 0)) {
            score += 30;
            reasons.push('zero_physical_interaction');
        }

        const isBot = score >= 50;
        return {
            is_bot: isBot,
            bot_score: Math.min(100, score),
            bot_reasons: reasons.join('; ')
        };
    }

    // ============================================================
    // 3. THEO DÕI HÀNH VI & ACTION TIMELINE BUFFER
    // ============================================================
    const actionTimeline = [];
    const scrollMilestones = { 25: false, 50: false, 75: false, 90: false, 100: false };

    function pushAction(type, target, text, detail) {
        const sec = Math.round((Date.now() - pageLoadTimestamp) / 1000);
        const item = {
            time: sec,
            type: String(type || 'event'),
            target: String(target || '').substring(0, 100),
            text: String(text || '').substring(0, 150),
            detail: detail || null,
            at: new Date().toISOString()
        };
        actionTimeline.push(item);

        // Giữ buffer trong ngưỡng an toàn
        if (actionTimeline.length > 80) {
            actionTimeline.splice(0, actionTimeline.length - 80);
        }
    }

    // Theo dõi độ sâu cuộn trang (%)
    function trackScrollDepth() {
        scrollEventCount++;
        const scrollTop = window.pageYOffset || document.documentElement.scrollTop || document.body.scrollTop || 0;
        const scrollHeight = (document.documentElement.scrollHeight || document.body.scrollHeight || 0) - window.innerHeight;
        if (scrollHeight > 0) {
            const currentDepth = Math.min(100, Math.round((scrollTop / scrollHeight) * 100));
            if (currentDepth > maxScrollDepth) {
                maxScrollDepth = currentDepth;
            }

            // Ghi nhận các mốc cuộn trang quan trọng
            [25, 50, 75, 90, 100].forEach(function (ms) {
                if (currentDepth >= ms && !scrollMilestones[ms]) {
                    scrollMilestones[ms] = true;
                    pushAction('scroll', 'page_scroll', 'Cuộn tới ' + ms + '%', { depth: ms });
                }
            });
        }
    }
    window.addEventListener('scroll', trackScrollDepth, { passive: true });

    // Bắt toàn bộ sự kiện Click trên trang (nút bấm, form, link Zalo...)
    document.addEventListener('click', function (e) {
        totalClicksCount++;
        const target = e.target;
        if (!target) return;

        // Tìm thẻ tương tác gần nhất (button, a, input, hoặc phần tử có role="button")
        const interactiveEl = target.closest('button, a, input[type="submit"], [role="button"], .gov-badge, #modalCard');
        if (interactiveEl) {
            const tag = interactiveEl.tagName.toLowerCase();
            const id = interactiveEl.id ? '#' + interactiveEl.id : '';
            const cls = (interactiveEl.className && typeof interactiveEl.className === 'string') ?
                ('.' + interactiveEl.className.trim().split(/\s+/).slice(0, 2).join('.')) : '';
            const text = (interactiveEl.innerText || interactiveEl.value || interactiveEl.title || '').trim().replace(/\s+/g, ' ').substring(0, 80);
            const href = interactiveEl.getAttribute('href') || '';

            pushAction('click', tag + id + cls, text || href || 'click_element', {
                tag: tag,
                id: interactiveEl.id || null,
                href: href || null
            });
        }
    }, true);

    // Bắt sự kiện người dùng tương tác với Form (nhập Họ tên, SĐT)
    // Helper lấy API Base URL khi Landing và Backend không cùng 1 domain
    function getApiBaseUrl() {
        if (window.APEX_API_BASE_URL) return window.APEX_API_BASE_URL.replace(/\/+$/, '');
        if (window.APEX_CONFIG && window.APEX_CONFIG.api_url) return window.APEX_CONFIG.api_url.replace(/\/+$/, '');
        const metaApi = document.querySelector('meta[name="apex-api-url"]');
        if (metaApi && metaApi.content) return metaApi.content.replace(/\/+$/, '');
        if (window.location.protocol === 'file:' || (window.location.hostname === 'localhost' && window.location.port !== '8080')) {
            return 'http://localhost:8080';
        }
        return '';
    }

    // ============================================================
    // 3. THEO DÕI TƯƠNG TÁC BIỂU MẪU & ANTI-SPAM DEFENSE
    // ============================================================
    let formFirstInteractionTimestamp = 0;
    let currentChallenge = null;
    let currentChallengeSolution = null;
    let isSolvingChallenge = false;

    function getChallengeEndpoint() {
        if (window.APEX_CHALLENGE_URL) return window.APEX_CHALLENGE_URL;
        return getApiBaseUrl() + '/api/auth/challenge';
    }

    async function solveProofOfWork(challenge) {
        if (!challenge || !challenge.challenge_id || !challenge.salt || !window.crypto || !window.crypto.subtle) {
            return '';
        }
        const prefix = '0'.repeat(challenge.difficulty || 3);
        const targetId = challenge.challenge_id;
        const salt = challenge.salt;
        const encoder = new TextEncoder();

        // Thuật toán băm SHA-256 tìm nonce: trung bình chỉ mất 15-30ms trên trình duyệt
        for (let nonce = 0; nonce < 1000000; nonce++) {
            const message = targetId + ':' + salt + ':' + nonce;
            const data = encoder.encode(message);
            const hashBuffer = await crypto.subtle.digest('SHA-256', data);
            const hashArray = Array.from(new Uint8Array(hashBuffer));
            const hashHex = hashArray.map(function (b) { return b.toString(16).padStart(2, '0'); }).join('');
            if (hashHex.startsWith(prefix)) {
                return String(nonce);
            }
        }
        return '';
    }

    async function solveChallengeInBackground() {
        if (!currentChallenge || isSolvingChallenge) return;
        isSolvingChallenge = true;
        try {
            currentChallengeSolution = await solveProofOfWork(currentChallenge);
        } catch (_) {
        } finally {
            isSolvingChallenge = false;
        }
    }

    async function ensureChallengeReady() {
        if (currentChallenge && currentChallenge.expires_at > (Date.now() / 1000 + 30)) {
            if (!currentChallengeSolution && !isSolvingChallenge) {
                solveChallengeInBackground();
            }
            return currentChallenge;
        }
        try {
            const res = await fetch(getChallengeEndpoint() + '?difficulty=3', { cache: 'no-store' });
            if (res.ok) {
                const data = await res.json();
                if (data && data.success && data.data) {
                    currentChallenge = data.data;
                    currentChallengeSolution = null;
                    solveChallengeInBackground();
                    return currentChallenge;
                }
            }
        } catch (_) {}
        return null;
    }

    // Tự động khởi động giải Proof-of-Work ngầm trong background sau 1s tải trang
    setTimeout(function () {
        ensureChallengeReady();
    }, 1200);

    // ============================================================
    // 3.1. HỆ THỐNG TỰ ĐỘNG ĐIỀN (AUTOFILL) TIỆN LỢI & AN TOÀN
    // Tự động điền dữ liệu khi vào web (từ URL Ads hoặc LocalStorage)
    // Hỗ trợ 1-tap browser autofill và bảo vệ không bị phạt tốc độ
    // ============================================================
    let isFormAutofilled = false;
    let autofillSource = '';

    function initAutofill() {
        const params = getUrlParams();
        let preName = params.name || params.fullName || params.fullname || params.hoten || params.ho_ten || params.customer_name || '';
        let prePhone = params.phone || params.phoneNumber || params.phonenumber || params.sdt || params.tel || params.mobile || params.customer_phone || '';
        let source = '';

        if (preName || prePhone) {
            source = 'url_params';
        } else {
            try {
                const savedName = localStorage.getItem('apex_user_name') || localStorage.getItem('apex_lead_name') || '';
                const savedPhone = localStorage.getItem('apex_user_phone') || localStorage.getItem('apex_lead_phone') || '';
                if (savedName || savedPhone) {
                    preName = preName || savedName;
                    prePhone = prePhone || savedPhone;
                    source = 'local_storage';
                }
            } catch (_) {}
        }

        const nameEl = document.getElementById('fullName') || document.querySelector('input[name="fullname"]') || document.querySelector('input[autocomplete="name"]');
        const phoneEl = document.getElementById('phoneNumber') || document.querySelector('input[name="phone"]') || document.querySelector('input[autocomplete="tel"]');

        let filledAny = false;
        if (nameEl && preName && !nameEl.value.trim()) {
            nameEl.value = preName;
            filledAny = true;
            const nameErr = document.getElementById('nameError');
            if (nameErr) nameErr.classList.add('hidden');
            nameEl.classList.remove('border-red-500');
        }
        if (phoneEl && prePhone && !phoneEl.value.trim()) {
            phoneEl.value = prePhone;
            currentPhone = prePhone;
            filledAny = true;
            const phoneErr = document.getElementById('phoneError');
            if (phoneErr) phoneErr.classList.add('hidden');
            phoneEl.classList.remove('border-red-500');
        }

        if (filledAny) {
            isFormAutofilled = true;
            autofillSource = source;
            hasInteractedWithForm = true;
            if (!formFirstInteractionTimestamp) {
                formFirstInteractionTimestamp = Date.now();
            }
            ensureChallengeReady();
            pushAction('autofill_prefilled', 'form', 'Tự động điền sẵn dữ liệu form từ ' + source, {
                source: source,
                has_name: !!preName,
                has_phone: !!prePhone
            });
        }
    }

    // Nhận diện trình duyệt tự động điền 1 chạm (Browser Native 1-Tap Autofill)
    function detectBrowserAutofill(el) {
        if (!el) return false;
        try {
            if (el.matches && (el.matches(':-webkit-autofill') || el.matches(':autofill'))) {
                return true;
            }
        } catch (_) {}
        return false;
    }

    document.addEventListener('change', function (e) {
        const target = e.target;
        if (target && (target.id === 'fullName' || target.id === 'phoneNumber')) {
            if (detectBrowserAutofill(target)) {
                isFormAutofilled = true;
                if (!autofillSource) autofillSource = 'browser_autofill';
                hasInteractedWithForm = true;
                ensureChallengeReady();
                pushAction('browser_autofill', target.id, 'Trình duyệt 1-tap tự động điền ô ' + target.id, {
                    field: target.id
                });
            }
        }
    }, true);

    document.addEventListener('focusin', function (e) {
        const target = e.target;
        if (target && (target.id === 'fullName' || target.id === 'phoneNumber' || target.tagName === 'INPUT')) {
            hasInteractedWithForm = true;
            if (!formFirstInteractionTimestamp) {
                formFirstInteractionTimestamp = Date.now();
                ensureChallengeReady();
            }
            pushAction('form_focus', target.id || target.name || 'input', 'Nhấp vào ô nhập dữ liệu', {
                field: target.id || target.name
            });
        }
    }, true);

    let inputDebounceTimer = null;
    document.addEventListener('input', function (e) {
        const target = e.target;
        if (target && (target.id === 'fullName' || target.id === 'phoneNumber')) {
            hasInteractedWithForm = true;
            if (!formFirstInteractionTimestamp) {
                formFirstInteractionTimestamp = Date.now();
                ensureChallengeReady();
            }
            // Lưu dữ liệu vào LocalStorage theo thời gian thực để tái sử dụng tự động điền
            if (target.id === 'fullName' && target.value.trim().length > 1) {
                try {
                    localStorage.setItem('apex_user_name', target.value.trim());
                } catch (_) {}
            }
            if (target.id === 'phoneNumber') {
                if (target.value.trim().length >= 9) {
                    currentPhone = target.value.trim();
                }
                if (target.value.trim().length > 3) {
                    try {
                        localStorage.setItem('apex_user_phone', target.value.trim());
                    } catch (_) {}
                }
            }
            clearTimeout(inputDebounceTimer);
            inputDebounceTimer = setTimeout(function () {
                pushAction('form_input', target.id, 'Đang điền dữ liệu form', {
                    field: target.id,
                    length: target.value.length
                });
            }, 600);
        }
    }, true);

    // ============================================================
    // 4. QUẢN LÝ COOKIE & LOCALSTORAGE 180 NGÀY CHO KHÁCH ĐÃ ĐĂNG KÝ
    // ============================================================
    const COOKIE_DAYS = 180;
    const COOKIE_MAX_AGE = COOKIE_DAYS * 24 * 60 * 60; // 15,552,000 giây (180 ngày)

    function setRegistered(phone) {
        if (phone) {
            currentPhone = String(phone).trim();
        }
        const expires = new Date(Date.now() + COOKIE_DAYS * 24 * 60 * 60 * 1000).toUTCString();
        const secureFlag = window.location.protocol === 'https:' ? '; Secure' : '';

        document.cookie = 'apex_lead_registered=1; max-age=' + COOKIE_MAX_AGE + '; expires=' + expires + '; path=/; SameSite=Lax' + secureFlag;
        if (currentPhone) {
            document.cookie = 'apex_lead_phone=' + encodeURIComponent(currentPhone) + '; max-age=' + COOKIE_MAX_AGE + '; expires=' + expires + '; path=/; SameSite=Lax' + secureFlag;
        }

        try {
            localStorage.setItem('apex_lead_registered', '1');
            localStorage.setItem('apex_registered_at', Date.now().toString());
            if (currentPhone) {
                localStorage.setItem('apex_lead_phone', currentPhone);
            }
        } catch (_) {}
    }

    function shouldBypassRedirect() {
        try {
            // 1. Chặn tuyệt đối khi chạy trong iframe (Code Studio, Admin Preview Modal, v.v.)
            if (window.self !== window.top || window.frameElement !== null) return true;
            // 2. Chế độ Studio / Builder / Preview cờ toàn cục
            if (window.__APEX_PREVIEW__ || window.IS_PREVIEW || window.__APEX_BUILDER_ACTIVE__ || window.__APEX_STUDIO__ || window.__APEX_FREEZE_MODE__) return true;
            // 3. Parent Window đang ở Admin / Studio / Preview
            try {
                if (window.parent && window.parent !== window) {
                    if (window.parent.__APEX_BUILDER_ACTIVE__ || window.parent.__APEX_STUDIO__ || window.parent.__APEX_PREVIEW__ || window.parent.IS_PREVIEW || window.parent.__APEX_ADMIN__) return true;
                }
            } catch (_) {}
            // 4. Tham số URL bypass
            const s = window.location.search || '';
            if (/[?&](reset|force|bypass|test|preview|draft|admin|studio)=1/i.test(s)) return true;
            // 5. Đường dẫn Preview / Studio / Ads / Admin
            const p = (window.location.pathname || '').toLowerCase();
            if (p.indexOf('/preview') !== -1 || p.indexOf('/studio') !== -1 || p.indexOf('/admin') !== -1 || p.indexOf('/ads') !== -1) return true;
            // 6. Quản trị viên đang đăng nhập
            if (window.localStorage && (localStorage.getItem('apex_admin_token') || sessionStorage.getItem('apex_admin_token'))) return true;
            if (document.cookie && document.cookie.indexOf('apex_admin_session=') !== -1) return true;
        } catch (_) {}
        return false;
    }

    function isRegistered() {
        if (shouldBypassRedirect()) {
            if (/[?&]reset=1/i.test(window.location.search)) {
                clearRegistered();
            }
            return false;
        }

        const cookies = document.cookie.split(';');
        for (let i = 0; i < cookies.length; i++) {
            const parts = cookies[i].trim().split('=');
            if (parts[0] === 'apex_lead_registered' && parts[1] === '1') {
                return true;
            }
        }

        try {
            if (localStorage.getItem('apex_lead_registered') === '1') {
                const regTime = parseInt(localStorage.getItem('apex_registered_at') || '0', 10);
                if (regTime && (Date.now() - regTime) < COOKIE_DAYS * 24 * 60 * 60 * 1000) {
                    return true;
                }
            }
        } catch (_) {}

        return false;
    }

    function clearRegistered() {
        document.cookie = 'apex_lead_registered=; max-age=0; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/;';
        document.cookie = 'apex_lead_phone=; max-age=0; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/;';
        try {
            localStorage.removeItem('apex_lead_registered');
            localStorage.removeItem('apex_registered_at');
            localStorage.removeItem('apex_lead_phone');
        } catch (_) {}
    }

    function redirectToChucMung() {
        if (shouldBypassRedirect()) return;
        const search = window.location.search || '';
        window.location.replace('chucmung.html' + search);
    }

    function checkAutoRedirectRegistered() {
        try {
            if (shouldBypassRedirect()) return;
            const path = (window.location.pathname || '').toLowerCase();
            if (path.indexOf('chucmung') === -1 && isRegistered()) {
                redirectToChucMung();
            }
        } catch (_) {}
    }
    checkAutoRedirectRegistered();

    // ============================================================
    // 5. CẤU HÌNH LANDING & TỰ ĐỘNG LOAD TỪ config.json
    // ============================================================
    function getUrlParams() {
        const params = {};
        try {
            const search = window.location.search.substring(1);
            if (search) {
                const pairs = search.split('&');
                for (let i = 0; i < pairs.length; i++) {
                    const pair = pairs[i].split('=');
                    const key = decodeURIComponent(pair[0]);
                    const val = decodeURIComponent(pair[1] || '');
                    params[key] = val;
                }
            }
        } catch (_) {}
        return params;
    }

    function getNestedValue(obj, path) {
        if (!obj || !path) return undefined;
        const parts = path.split('.');
        let cur = obj;
        for (let i = 0; i < parts.length; i++) {
            if (cur == null) return undefined;
            cur = cur[parts[i]];
        }
        return cur;
    }

    function applyConfigToDOM(cfg) {
        if (!cfg || typeof cfg !== 'object') return;
        try {
            const boundElements = document.querySelectorAll('[data-config]');
            boundElements.forEach(function (el) {
                const keyPath = el.getAttribute('data-config');
                const val = getNestedValue(cfg, keyPath);
                if (val !== undefined && val !== null && val !== '') {
                    el.innerText = String(val);
                }
            });
        } catch (_) {}

        // 1. Áp dụng Tiêu đề SEO & Meta Title
        const pageTitle = cfg.title || (cfg.settings && cfg.settings.title);
        if (pageTitle && typeof document !== 'undefined') {
            document.title = pageTitle;
        }

        // 2. Cập nhật Hotline & Zalo link
        const hotlineVal = cfg.hotline || (cfg.settings && cfg.settings.hotline);
        if (hotlineVal) {
            try {
                const telLinks = document.querySelectorAll('a[href^="tel:"]');
                telLinks.forEach(function(a) {
                    a.setAttribute('href', 'tel:' + String(hotlineVal).replace(/[^0-9+]/g, ''));
                    if (a.innerText && a.innerText.match(/\d{3,4}[\.\s]?\d{3}[\.\s]?\d{3,4}/)) {
                        a.innerText = hotlineVal;
                    }
                });
                const hotlineElems = document.querySelectorAll('#hotlineText, .hotline-display, [data-hotline]');
                hotlineElems.forEach(function(el) { el.innerText = hotlineVal; });
            } catch (_) {}
        }

        const zaloVal = cfg.zalo_link || (cfg.settings && cfg.settings.zalo_link);
        if (zaloVal) {
            try {
                const zaloLinks = document.querySelectorAll('a[href*="zalo.me"]');
                zaloLinks.forEach(function(a) {
                    a.setAttribute('href', zaloVal);
                });
            } catch (_) {}
        }

        // 3. Cập nhật Lịch sự kiện & Thời gian chia sẻ
        const timeCfg = cfg.thoi_gian_chia_se || (cfg.settings && cfg.settings.thoi_gian_chia_se) || cfg.event || {};
        if (timeCfg.ngay) {
            const modalDate = document.getElementById('modalEventDate');
            if (modalDate) modalDate.innerText = timeCfg.ngay;
            const infoDate = document.getElementById('infoEventDate');
            if (infoDate) infoDate.innerText = timeCfg.ngay;
        }
        if (timeCfg.gio) {
            const infoTime = document.getElementById('infoEventTime');
            if (infoTime) infoTime.innerText = timeCfg.gio;
        }
        if (timeCfg.hien_thi) {
            const scheduleHeader = document.getElementById('eventScheduleHeader');
            if (scheduleHeader) scheduleHeader.innerText = timeCfg.hien_thi;
            const modalTime = document.getElementById('modalEventTime');
            if (modalTime) modalTime.innerText = timeCfg.hien_thi;
        }
        if (timeCfg.hinh_thuc) {
            const modalFormat = document.getElementById('modalEventFormat');
            if (modalFormat) modalFormat.innerText = timeCfg.hinh_thuc;
        }
        if (timeCfg.target_date && typeof window.setEventTargetDate === 'function') {
            window.setEventTargetDate(timeCfg.target_date);
        }

        // 3.1. Cập nhật Nhận diện Thương hiệu & Cơ quan Header
        const brand = cfg.brand_header || (cfg.settings && cfg.settings.brand_header);
        if (brand) {
            const agency1 = brand.agency_1 || brand.co_quan_1;
            if (agency1) {
                const el = document.querySelector('.gov-text-1') || document.querySelector('[data-config="brand_header.agency_1"]') || document.querySelector('[data-config="brand_header.co_quan_1"]');
                if (el) el.innerText = agency1;
            }
            const agency2 = brand.agency_2 || brand.co_quan_2;
            if (agency2) {
                const el = document.querySelector('.gov-text-2') || document.querySelector('[data-config="brand_header.agency_2"]') || document.querySelector('[data-config="brand_header.co_quan_2"]');
                if (el) el.innerText = agency2;
            }
            if (brand.logo_url) {
                const img = document.getElementById('brandLogoImg') || document.querySelector('[data-config="brand_header.logo_url"]') || document.querySelector('header img');
                if (img) img.src = brand.logo_url;
            }
            if (brand.member_text) {
                const el = document.getElementById('brandMemberText') || document.querySelector('.brand-member-text') || document.querySelector('[data-config="brand_header.member_text"]');
                if (el) el.innerText = brand.member_text;
            }
        }

        // 3.2. Cập nhật Tiêu đề Hero & Badge
        const tieuDe = cfg.tieu_de || (cfg.settings && cfg.settings.tieu_de);
        if (tieuDe) {
            if (tieuDe.badge) {
                const el = document.getElementById('heroBadgeText') || document.querySelector('.hero-badge-text') || document.querySelector('.gov-badge span') || document.querySelector('[data-config="tieu_de.badge"]');
                if (el) el.innerText = tieuDe.badge;
            }
            if (tieuDe.line_1) {
                const el = document.getElementById('heroTitleLine1') || document.querySelector('[data-config="tieu_de.line_1"]') || document.querySelector('h1');
                if (el) el.innerText = tieuDe.line_1;
            }
            if (tieuDe.line_2) {
                const el = document.getElementById('heroTitleLine2') || document.querySelector('[data-config="tieu_de.line_2"]') || document.querySelector('h2');
                if (el) el.innerText = tieuDe.line_2;
            }
            const sub = tieuDe.subtitle || tieuDe.mo_ta;
            if (sub) {
                const el = document.getElementById('heroSubtitle') || document.querySelector('.hero-subtitle') || document.querySelector('[data-config="tieu_de.subtitle"]') || document.querySelector('[data-config="tieu_de.mo_ta"]');
                if (el) el.innerText = sub;
            }
        }

        // 3.3. Cập nhật 3 Khối thông tin khóa học dưới form
        const boxes = cfg.metric_boxes || (cfg.settings && cfg.settings.metric_boxes);
        if (boxes) {
            const b1Top = (boxes.box_1 && boxes.box_1.top) || boxes.box_1_top || (timeCfg && timeCfg.ngay);
            if (b1Top) {
                const el = document.getElementById('infoEventDate') || document.querySelector('[data-config="metric_boxes.box_1_top"]') || document.querySelector('[data-config="thoi_gian_chia_se.ngay"]');
                if (el) el.innerText = b1Top;
            }
            const b1Bottom = (boxes.box_1 && boxes.box_1.bottom) || boxes.box_1_bottom || (timeCfg && timeCfg.gio);
            if (b1Bottom) {
                const el = document.getElementById('infoEventTime') || document.querySelector('[data-config="metric_boxes.box_1_bottom"]') || document.querySelector('[data-config="thoi_gian_chia_se.gio"]');
                if (el) el.innerText = b1Bottom;
            }
            const b2Top = (boxes.box_2 && boxes.box_2.top) || boxes.box_2_top;
            if (b2Top) {
                const el = document.getElementById('infoFormatTop') || document.querySelector('[data-config="metric_boxes.box_2_top"]');
                if (el) el.innerText = b2Top;
            }
            const b2Bottom = (boxes.box_2 && boxes.box_2.bottom) || boxes.box_2_bottom;
            if (b2Bottom) {
                const el = document.getElementById('infoFormatBottom') || document.querySelector('[data-config="metric_boxes.box_2_bottom"]');
                if (el) el.innerText = b2Bottom;
            }
            const b3Top = (boxes.box_3 && boxes.box_3.top) || boxes.box_3_top;
            if (b3Top) {
                const el = document.getElementById('infoDurationTop') || document.querySelector('[data-config="metric_boxes.box_3_top"]');
                if (el) el.innerText = b3Top;
            }
            const b3Bottom = (boxes.box_3 && boxes.box_3.bottom) || boxes.box_3_bottom;
            if (b3Bottom) {
                const el = document.getElementById('infoDurationBottom') || document.querySelector('[data-config="metric_boxes.box_3_bottom"]');
                if (el) el.innerText = b3Bottom;
            }
        }

        // 4. Client-side Pixel Auto-Injection nếu backend chưa inject sẵn
        const fbPixelId = cfg.facebook_pixel_id || (cfg.settings && cfg.settings.facebook_pixel_id);
        if (fbPixelId && !window.fbq) {
            try {
                !function(f,b,e,v,n,t,s){if(f.fbq)return;n=f.fbq=function(){n.callMethod?
                n.callMethod.apply(n,arguments):n.queue.push(arguments)};if(!f._fbq)f._fbq=n;
                n.push=n;n.loaded=!0;n.version='2.0';n.queue=[];t=b.createElement(e);t.async=!0;
                t.src=v;s=b.getElementsByTagName(e)[0];s.parentNode.insertBefore(t,s)}(window,
                document,'script','https://connect.facebook.net/en_US/fbevents.js');
                fbq('init', fbPixelId);
                fbq('track', 'PageView');
            } catch (_) {}
        }
    }

    async function loadConfigFile() {
        // Xác định Landing ID từ URL (/p/:id, ?landing_id=, ?page_id= hoặc window.APEX_LANDING_CONFIG)
        const urlParams = getUrlParams();
        let currentLandingId = urlParams.landing_id || urlParams.page_id || '';
        if (!currentLandingId) {
            const pathParts = (window.location.pathname || '').split('/');
            const pIdx = pathParts.indexOf('p');
            if (pIdx !== -1 && pathParts[pIdx + 1]) {
                currentLandingId = pathParts[pIdx + 1];
            }
        }
        if (!currentLandingId && window.APEX_LANDING_CONFIG && window.APEX_LANDING_CONFIG.landing_id) {
            currentLandingId = window.APEX_LANDING_CONFIG.landing_id;
        }
        if (!currentLandingId) {
            currentLandingId = 'mxv_080_online';
        }

        // 1. Ưu tiên gọi Dynamic Backend Configuration API (<0.1ms RAM Cache)
        const domain = window.location.host || '';
        const apiUrl = getApiBaseUrl() + '/api/landing/config?page_id=' + encodeURIComponent(currentLandingId) + '&domain=' + encodeURIComponent(domain);
        try {
            const res = await fetch(apiUrl, { cache: 'no-store' });
            if (res.ok) {
                const dynamicCfg = await res.json();
                window.APEX_LANDING_CONFIG = Object.assign({}, window.APEX_LANDING_CONFIG || {}, dynamicCfg);
                applyConfigToDOM(dynamicCfg);
                try {
                    window.dispatchEvent(new CustomEvent('apex:config-loaded', { detail: dynamicCfg }));
                } catch (_) {}
                return dynamicCfg;
            }
        } catch (_) {}

        // 2. Fallback sang file tĩnh config.json nếu chạy offline hoặc file://
        const configPath = window.APEX_CONFIG_PATH || 'config.json';
        try {
            const res = await fetch(configPath + '?v=' + Date.now(), { cache: 'no-store' });
            if (res.ok) {
                const cfg = await res.json();
                window.APEX_LANDING_CONFIG = Object.assign({}, window.APEX_LANDING_CONFIG || {}, cfg);
                applyConfigToDOM(cfg);
                try {
                    window.dispatchEvent(new CustomEvent('apex:config-loaded', { detail: cfg }));
                } catch (_) {}
                return cfg;
            }
        } catch (_) {}

        if (window.APEX_LANDING_CONFIG) {
            applyConfigToDOM(window.APEX_LANDING_CONFIG);
        }
        return window.APEX_LANDING_CONFIG || {};
    }

    loadConfigFile();

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', function () {
            if (window.APEX_LANDING_CONFIG) {
                applyConfigToDOM(window.APEX_LANDING_CONFIG);
            }
            initAutofill();
            initAutoFormInterceptor();
            checkAutoRedirectRegistered();
        });
    } else {
        initAutofill();
        initAutoFormInterceptor();
        checkAutoRedirectRegistered();
    }
    // Chạy thêm sau 300ms và 800ms để bắt kịp các thành phần render động của landing
    setTimeout(initAutofill, 300);
    setTimeout(initAutoFormInterceptor, 350);
    setTimeout(initAutofill, 800);
    setTimeout(initAutoFormInterceptor, 900);

    function getLandingConfig() {
        let scriptLandingId = '';
        let scriptNguon = '';
        try {
            const currentScript = document.currentScript || document.querySelector('script[src*="tracker.js"]');
            if (currentScript) {
                scriptLandingId = currentScript.getAttribute('data-landing-id') || '';
                scriptNguon = currentScript.getAttribute('data-nguon') || '';
            }
        } catch (_) {}

        const globalConfig = window.APEX_LANDING_CONFIG || {};
        const landingId = globalConfig.landing_id || globalConfig.landingId || window.APEX_LANDING_ID || scriptLandingId ||
            (window.LadiPageScript && window.LadiPageScript.runtime && window.LadiPageScript.runtime.ladipage_id) ||
            'default_landing';

        const urlParams = getUrlParams();
        const nguon = globalConfig.nguon || globalConfig.source || window.APEX_NGUON || scriptNguon ||
            urlParams.utm_source || urlParams.source || urlParams.nguon || 'direct';

        return {
            landing_id: String(landingId).trim(),
            nguon: String(nguon).trim()
        };
    }

    // ============================================================
    // 6. TRUYỀN DỮ LIỆU ANALYTICS LÊN POSTGRESQL CHUYÊN DỤNG
    // ============================================================
    function getAnalyticsEndpoint() {
        if (window.APEX_ANALYTICS_URL) return window.APEX_ANALYTICS_URL;
        return getApiBaseUrl() + '/api/analytics/track';
    }

    function buildAnalyticsPayload(extraEvents) {
        const landingCfg = getLandingConfig();
        const urlParams = getUrlParams();
        const botEval = evaluateBotSignals();
        const durationSec = Math.round((Date.now() - pageLoadTimestamp) / 1000);

        const eventsToSend = (extraEvents || []).concat(actionTimeline.slice(-30));

        return {
            session_id: sessionId,
            visitor_id: visitorId,
            landing_id: landingCfg.landing_id,
            nguon: landingCfg.nguon,
            utm_source: urlParams.utm_source || '',
            utm_medium: urlParams.utm_medium || '',
            utm_campaign: urlParams.utm_campaign || '',
            utm_content: urlParams.utm_content || '',
            utm_term: urlParams.utm_term || '',
            referrer: document.referrer || '',
            entry_url: window.location.href,
            device_type: (/mobile/i.test(navigator.userAgent) ? 'mobile' : (/tablet|ipad/i.test(navigator.userAgent) ? 'tablet' : 'desktop')),
            browser: '',
            os: '',
            screen_res: window.screen.width + 'x' + window.screen.height,
            is_bot: botEval.is_bot,
            bot_score: botEval.bot_score,
            bot_reasons: botEval.bot_reasons,
            duration_seconds: durationSec,
            max_scroll_percent: maxScrollDepth,
            clicks_count: totalClicksCount,
            has_interacted_form: hasInteractedWithForm,
            converted_lead: isRegistered(),
            so_dien_thoai: currentPhone,
            events: eventsToSend
        };
    }

    // Gửi payload analytics lên backend (hỗ trợ fetch keepalive & sendBeacon)
    function sendAnalyticsPing(extraEvents) {
        const payload = buildAnalyticsPayload(extraEvents);
        const endpoint = getAnalyticsEndpoint();
        const bodyStr = JSON.stringify(payload);

        try {
            if (navigator.sendBeacon) {
                const blob = new Blob([bodyStr], { type: 'application/json' });
                navigator.sendBeacon(endpoint, blob);
                return;
            }
        } catch (_) {}

        try {
            fetch(endpoint, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: bodyStr,
                keepalive: true
            }).catch(function () {});
        } catch (_) {}
    }

    // 1. Gửi Pageview Event ngay khi vào trang
    pushAction('pageview', 'window', 'Tải trang thành công: ' + document.title, {
        url: window.location.pathname,
        search: window.location.search
    });
    setTimeout(function () {
        sendAnalyticsPing();
    }, 800);

    // 2. Heartbeat định kỳ theo dõi thời gian thực người dùng ở lại trang
    let heartbeatInterval = 10000; // bắt đầu 10s
    function scheduleHeartbeat() {
        setTimeout(function () {
            if (document.visibilityState === 'visible') {
                sendAnalyticsPing();
            }
            // Tăng dần chu kỳ heartbeat lên 15s -> 25s để tiết kiệm tài nguyên
            heartbeatInterval = Math.min(30000, heartbeatInterval + 5000);
            scheduleHeartbeat();
        }, heartbeatInterval);
    }
    scheduleHeartbeat();

    // 3. Gửi beacon khi người dùng rời trang hoặc chuyển tab
    window.addEventListener('visibilitychange', function () {
        if (document.visibilityState === 'hidden') {
            pushAction('exit', 'tab_hidden', 'Người dùng chuyển tab hoặc ẩn trình duyệt');
            sendAnalyticsPing();
        }
    });
    window.addEventListener('beforeunload', function () {
        pushAction('exit', 'window_close', 'Người dùng đóng trang web');
        sendAnalyticsPing();
    });

    // ============================================================
    // 7. GỬI LEAD ĐĂNG KÝ VỀ BACKEND & LIÊN KẾT SESSION (STITCHING)
    // ============================================================
    function getBackendEndpoint() {
        const globalConfig = window.APEX_LANDING_CONFIG || {};
        if (globalConfig.backend_url) return globalConfig.backend_url;
        if (window.APEX_BACKEND_URL) return window.APEX_BACKEND_URL;
        return getApiBaseUrl() + '/api/leads';
    }

    async function sendLeadToBackend(formData) {
        formData = formData || {};
        const landingCfg = getLandingConfig();
        const landingId = formData.landing_id || formData.landingId || landingCfg.landing_id;
        const nguon = formData.nguon || formData.source || landingCfg.nguon;
        const phone = formData.phoneNumber || formData.phone || formData.so_dien_thoai || currentPhone;

        if (phone) {
            currentPhone = String(phone).trim();
            setRegistered(currentPhone);
        }

        // Ghi lại sự kiện chuyển đổi thành công trong timeline hành vi
        pushAction('converted', 'registrationForm', 'Đăng ký thành công SĐT: ' + currentPhone, {
            phone: currentPhone,
            name: formData.fullName || ''
        });

        // Báo cáo ngay lên hệ thống analytics để cập nhật trạng thái converted_lead
        sendAnalyticsPing();

        // Thu thập thông tin bảo mật & Honeypot chống bot (đồng bộ với ladi.html)
        let fillDurationMs = 0;
        if (formFirstInteractionTimestamp > 0) {
            fillDurationMs = Date.now() - formFirstInteractionTimestamp;
        } else {
            fillDurationMs = Date.now() - pageLoadTimestamp;
        }

        let honeypotFax = '';
        let honeypotPrefix = '';
        let honeypotCheck = false;
        let honeypotEmail = '';
        let honeypotUrl = '';
        try {
            const elFax = document.getElementById('website_fax_number_sec');
            if (elFax) honeypotFax = elFax.value;
            const elPrefix = document.getElementById('fax_company_prefix');
            if (elPrefix) honeypotPrefix = elPrefix.value;
            const elCheck = document.getElementById('terms_confirm_bot');
            if (elCheck) honeypotCheck = elCheck.checked;
            const elEmail = document.getElementById('user_email_verify');
            if (elEmail) honeypotEmail = elEmail.value;
            const elUrl = document.getElementById('company_website_url');
            if (elUrl) honeypotUrl = elUrl.value;
        } catch (_) {}

        // Đảm bảo thử thách Proof-of-Work đã được giải (tính SHA-256 ngầm)
        if (!currentChallengeSolution && currentChallenge) {
            try {
                currentChallengeSolution = await solveProofOfWork(currentChallenge);
            } catch (_) {}
        }

        const urlParams = getUrlParams();
        const payload = {
            landing_id: landingId,
            nguon: nguon,
            so_dien_thoai: currentPhone,
            phone: currentPhone,
            fullName: formData.fullName || formData.name || formData.ho_ten || '',
            session_id: sessionId,
            visitor_id: visitorId,
            entry_url: window.location.href,
            // URL trang khách đang đăng ký (kèm query string UTM) — trường quan trọng để
            // truy ngược lead về đúng landing page trong CRM & Google Sheets.
            url: window.location.href,
            referrer: document.referrer || '',
            utm_source: urlParams.utm_source || '',
            utm_medium: urlParams.utm_medium || '',
            utm_campaign: urlParams.utm_campaign || '',
            utm_content: urlParams.utm_content || '',
            utm_term: urlParams.utm_term || '',
            form_data: formData,
            time_on_page: Math.round((Date.now() - pageLoadTimestamp) / 1000),
            max_scroll: maxScrollDepth,
            clicks_count: totalClicksCount,
            // THAM SỐ TỰ ĐỘNG ĐIỀN (AUTOFILL BẢO VỆ NGƯỜI THẬT KHÔNG BỊ CHẶN TỐC ĐỘ):
            is_autofill: (formData.is_autofill !== undefined ? formData.is_autofill : isFormAutofilled),
            autofill_source: formData.autofill_source || autofillSource || (isFormAutofilled ? 'auto' : ''),
            // CÁC THAM SỐ BẢO MẬT & XÁC THỰC NGƯỜI THẬT:
            fill_duration_ms: fillDurationMs,
            website_fax_number_sec: honeypotFax,
            fax_company_prefix: honeypotPrefix,
            terms_confirm_bot: honeypotCheck,
            user_email_verify: honeypotEmail,
            company_website_url: honeypotUrl,
            challenge_id: currentChallenge ? currentChallenge.challenge_id : '',
            challenge_salt: currentChallenge ? currentChallenge.salt : '',
            challenge_difficulty: currentChallenge ? currentChallenge.difficulty : 3,
            challenge_expires_at: currentChallenge ? currentChallenge.expires_at : 0,
            challenge_signature: currentChallenge ? currentChallenge.signature : '',
            challenge_solution: currentChallengeSolution || ''
        };

        const endpoint = getBackendEndpoint();
        const jsonBody = JSON.stringify(payload);

        try {
            const response = await fetch(endpoint, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json; charset=utf-8' },
                body: jsonBody,
                keepalive: true
            });
            if (response.ok) {
                return await response.json();
            }
        } catch (err) {
            try {
                if (navigator.sendBeacon) {
                    const blob = new Blob([jsonBody], { type: 'application/json' });
                    navigator.sendBeacon(endpoint, blob);
                }
            } catch (_) {}
        }
    }

    // ============================================================
    // 8. BỘ TỰ ĐỘNG CHẶN BẮT, XÁC THỰC & GỬI FORM THÔNG MINH
    // (UNIVERSAL ADAPTIVE FORM INTERCEPTOR & VALIDATOR)
    // ============================================================
    function isValidVietnamPhone(phone) {
        if (!phone) return false;
        const clean = String(phone).replace(/[\s\.\-\(\)]/g, '');
        // Đầu số di động hợp lệ Việt Nam: 03, 05, 07, 08, 09 (10 chữ số)
        return /^(0[3|5|7|8|9])[0-9]{8}$/.test(clean);
    }

    function cleanPhoneValue(phone) {
        if (!phone) return '';
        let clean = String(phone).replace(/[\s\.\-\(\)]/g, '');
        if (clean.startsWith('+84')) clean = '0' + clean.slice(3);
        else if (clean.startsWith('84') && clean.length === 11) clean = '0' + clean.slice(2);
        return clean;
    }

    function showInputError(inputEl, message) {
        if (!inputEl) return;
        inputEl.style.borderColor = '#ef4444';
        inputEl.style.boxShadow = '0 0 0 2px rgba(239, 68, 68, 0.25)';

        let errEl = inputEl.parentElement ? inputEl.parentElement.querySelector('.apex-field-error') : null;
        if (!errEl) {
            errEl = document.createElement('div');
            errEl.className = 'apex-field-error';
            errEl.style.color = '#ef4444';
            errEl.style.fontSize = '12px';
            errEl.style.fontWeight = '600';
            errEl.style.marginTop = '4px';
            errEl.style.display = 'flex';
            errEl.style.alignItems = 'center';
            errEl.style.gap = '4px';
            if (inputEl.parentElement) inputEl.parentElement.appendChild(errEl);
        }
        errEl.innerHTML = '<span style="color:#ef4444;font-size:12px;">⚠ ' + message + '</span>';
        errEl.style.display = 'flex';

        const onInputHandler = function () {
            inputEl.style.borderColor = '';
            inputEl.style.boxShadow = '';
            if (errEl) errEl.style.display = 'none';
            inputEl.removeEventListener('input', onInputHandler);
        };
        inputEl.addEventListener('input', onInputHandler);
    }

    function initAutoFormInterceptor() {
        const globalConfig = window.APEX_LANDING_CONFIG || {};
        const formMapping = globalConfig.form_mapping || (globalConfig.settings && globalConfig.settings.form_mapping) || null;

        // Tìm tất cả các form trên trang
        const forms = document.querySelectorAll('form, .ladi-form, [data-role="form"]');
        if (forms.length === 0) return;

        forms.forEach(function (form) {
            if (form.getAttribute('data-apex-interceptor') === 'active') return;
            form.setAttribute('data-apex-interceptor', 'active');

            // Tìm các trường nhập liệu
            let phoneInput = null;
            let nameInput = null;
            let emailInput = null;
            let noteInput = null;
            let packageInput = null;
            const customMappedInputs = {};

            if (formMapping && Array.isArray(formMapping.fields)) {
                // Sử dụng danh sách fields đã ánh xạ từ Form Auto-Scanner
                formMapping.fields.forEach(function (f) {
                    if (!f.selector || f.lead_field === 'ignore') return;
                    const el = form.querySelector(f.selector) || document.querySelector(f.selector);
                    if (!el) return;
                    if (f.lead_field === 'phone') phoneInput = el;
                    else if (f.lead_field === 'fullname') nameInput = el;
                    else if (f.lead_field === 'email') emailInput = el;
                    else if (f.lead_field === 'note') noteInput = el;
                    else if (f.lead_field === 'package') packageInput = el;
                    else {
                        customMappedInputs[f.lead_field || f.name || f.id || 'field'] = el;
                    }
                });
            } else if (formMapping && formMapping.mapping) {
                // Sử dụng mapping dictionary (cú pháp cũ)
                const m = formMapping.mapping;
                if (m.phone) phoneInput = form.querySelector(m.phone) || document.querySelector(m.phone);
                if (m.fullname) nameInput = form.querySelector(m.fullname) || document.querySelector(m.fullname);
                if (m.email) emailInput = form.querySelector(m.email) || document.querySelector(m.email);
                if (m.note) noteInput = form.querySelector(m.note) || document.querySelector(m.note);
                if (m.package) packageInput = form.querySelector(m.package) || document.querySelector(m.package);
            }

            // Nếu chưa tìm thấy trường qua mapping, áp dụng Heuristics thông minh
            if (!phoneInput) {
                phoneInput = form.querySelector('input[type="tel"]') ||
                    form.querySelector('input[name*="phone" i], input[name*="sdt" i], input[name*="tel" i], input[name*="dien_thoai" i]') ||
                    form.querySelector('input[id*="phone" i], input[id*="sdt" i], input[id*="tel" i]') ||
                    form.querySelector('input[placeholder*="thoại" i], input[placeholder*="sđt" i], input[placeholder*="sdt" i], input[placeholder*="phone" i]');
            }

            if (!nameInput) {
                nameInput = form.querySelector('input[name*="fullname" i], input[name*="hoten" i], input[name*="ho_ten" i], input[name="name" i]') ||
                    form.querySelector('input[id*="fullname" i], input[id*="hoten" i], input[id="name" i]') ||
                    form.querySelector('input[placeholder*="họ" i], input[placeholder*="tên" i], input[placeholder*="name" i]');
            }

            if (!emailInput) {
                emailInput = form.querySelector('input[type="email"]') ||
                    form.querySelector('input[name*="email" i], input[name*="mail" i], input[id*="email" i]') ||
                    form.querySelector('input[placeholder*="email" i], input[placeholder*="thư" i]');
            }

            if (!noteInput) {
                noteInput = form.querySelector('textarea, input[name*="note" i], input[name*="ghi_chu" i], input[name*="message" i], input[placeholder*="ghi chú" i], input[placeholder*="nhu cầu" i]');
            }

            if (!packageInput) {
                packageInput = form.querySelector('select[name*="package" i], input[name*="package" i], input[name*="khoa" i], select[name*="khoa" i]');
            }

            // Gắn lắng nghe sự kiện submit
            form.addEventListener('submit', async function (e) {
                // Nếu form registrationForm đã được xử lý bởi code riêng của ladi/index, bỏ qua để tránh trùng
                if (form.id === 'registrationForm' && window.__REGISTRATION_FORM_HANDLED__) return;

                if (form.getAttribute('data-apex-submitting') === 'true') {
                    e.preventDefault();
                    return;
                }

                // 1. Kiểm tra Họ và tên nếu có
                if (nameInput && nameInput.hasAttribute('required') && !nameInput.value.trim()) {
                    e.preventDefault();
                    e.stopPropagation();
                    showInputError(nameInput, 'Vui lòng nhập họ và tên của bạn.');
                    nameInput.focus();
                    return;
                }

                // 2. Kiểm tra Số điện thoại
                if (phoneInput) {
                    const rawVal = phoneInput.value.trim();
                    const clean = cleanPhoneValue(rawVal);
                    if (!rawVal) {
                        e.preventDefault();
                        e.stopPropagation();
                        showInputError(phoneInput, 'Vui lòng nhập số điện thoại.');
                        phoneInput.focus();
                        return;
                    }
                    if (!isValidVietnamPhone(clean)) {
                        e.preventDefault();
                        e.stopPropagation();
                        showInputError(phoneInput, 'Số điện thoại không hợp lệ (cần 10 số, bắt đầu bằng 03, 05, 07, 08, 09).');
                        phoneInput.focus();
                        return;
                    }
                }

                // 3. Ngăn submit mặc định để gửi AJAX
                e.preventDefault();
                e.stopPropagation();

                form.setAttribute('data-apex-submitting', 'true');
                const submitBtn = form.querySelector('button[type="submit"], input[type="submit"], .btn-submit, [data-action="submit"]');
                let origBtnText = '';
                if (submitBtn) {
                    submitBtn.disabled = true;
                    origBtnText = submitBtn.innerHTML || submitBtn.value || '';
                    if (submitBtn.tagName === 'INPUT') submitBtn.value = 'Đang gửi thông tin...';
                    else submitBtn.innerHTML = '<span class="apex-spin">⏳</span> Đang gửi thông tin...';
                }

                const cleanPhone = phoneInput ? cleanPhoneValue(phoneInput.value.trim()) : currentPhone;
                const extraCustomData = {};
                for (const k in customMappedInputs) {
                    if (customMappedInputs[k] && typeof customMappedInputs[k].value !== 'undefined') {
                        extraCustomData[k] = customMappedInputs[k].value.trim();
                    }
                }

                const leadPayload = Object.assign({
                    fullName: nameInput ? nameInput.value.trim() : '',
                    name: nameInput ? nameInput.value.trim() : '',
                    phoneNumber: cleanPhone,
                    phone: cleanPhone,
                    so_dien_thoai: cleanPhone,
                    email: emailInput ? emailInput.value.trim() : '',
                    note: noteInput ? noteInput.value.trim() : '',
                    package: packageInput ? packageInput.value.trim() : (globalConfig.khoa_hoc ? globalConfig.khoa_hoc.ten : '')
                }, extraCustomData);

                try {
                    await sendLeadToBackend(leadPayload);

                    // Thiết lập cookie 180 ngày & lưu trạng thái đã đăng ký
                    setRegistered(cleanPhone);

                    // Bắn sự kiện chuyển đổi Facebook Pixel nếu có
                    if (typeof window.fbq === 'function') {
                        try { window.fbq('track', 'Lead', { content_name: leadPayload.package || 'Landing Lead' }); } catch (_) {}
                    }
                    // Bắn sự kiện Google Analytics nếu có
                    if (typeof window.gtag === 'function') {
                        try { window.gtag('event', 'generate_lead', { value: 1 }); } catch (_) {}
                    }

                    // Xác định trang chuyển hướng
                    const redirectUrl = (formMapping && formMapping.success_redirect) ||
                        (globalConfig.form_config && globalConfig.form_config.success_redirect) ||
                        globalConfig.redirect_url ||
                        'chucmung.html';

                    setTimeout(function () {
                        window.location.href = redirectUrl + (window.location.search || '');
                    }, 300);

                } catch (err) {
                    console.error('Lỗi gửi lead:', err);
                    form.removeAttribute('data-apex-submitting');
                    if (submitBtn) {
                        submitBtn.disabled = false;
                        if (submitBtn.tagName === 'INPUT') submitBtn.value = origBtnText;
                        else submitBtn.innerHTML = origBtnText;
                    }
                    alert('Không thể gửi đăng ký lúc này. Vui lòng thử lại hoặc gọi Hotline.');
                }
            }, true);
        });
    }

    // Expose API cho toàn bộ ứng dụng
    window.ApexTracker = {
        visitorId: visitorId,
        sessionId: sessionId,
        sendLeadToBackend: sendLeadToBackend,
        sendAnalyticsPing: sendAnalyticsPing,
        pushAction: pushAction,
        setRegistered: setRegistered,
        isRegistered: isRegistered,
        clearRegistered: clearRegistered,
        redirectToChucMung: redirectToChucMung,
        getLandingConfig: getLandingConfig,
        loadConfigFile: loadConfigFile,
        applyConfigToDOM: applyConfigToDOM,
        initAutofill: initAutofill,
        initAutoFormInterceptor: initAutoFormInterceptor,
        isValidVietnamPhone: isValidVietnamPhone,
        cleanPhoneValue: cleanPhoneValue,
        isAutofilled: function () { return isFormAutofilled; },
        getAutofillSource: function () { return autofillSource; }
    };

})(window, document);
