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

    // ============================================================
    // 1.0 KHÓA NGĂN CÁCH THEO LANDING (TÁCH BIỆT HOÀN TOÀN)
    // ------------------------------------------------------------
    // Mỗi landing page dùng một bộ khóa lưu trữ RIÊNG:
    //   apex_vid_<landingKey>, apex_lead_registered_<landingKey>, ...
    //
    // LÝ DO (yêu cầu 2026-11-12): trước đây localStorage/cookie dùng chung
    // cho MỌI landing trên cùng domain ⇒ khách đã đăng ký landing A bị coi
    // như đã đăng ký landing B ⇒ thống kê & dedupe bị lẫn giữa các landing.
    //
    // ⇒ Mỗi landing có visitor_id, trạng thái đăng ký và dữ liệu autofill
    //   hoàn toàn độc lập. Định danh toàn cục vẫn được giữ qua cookie Meta
    //   `_fbp` / `_fbc` (đã gửi lên server ở trường `fbp`).
    function resolveLandingKey() {
        try {
            // 1. data-landing-id trên thẻ <script src="tracker.js"> — backend luôn gắn
            const s = document.currentScript || document.querySelector('script[src*="tracker.js"]');
            if (s) {
                const attr = (s.getAttribute('data-landing-id') || '').trim();
                if (attr) return attr;
            }
        } catch (_) {}
        try {
            const cfg = window.APEX_LANDING_CONFIG || {};
            if (cfg.landing_id) return String(cfg.landing_id);
        } catch (_) {}
        try {
            const params = getUrlParams();
            if (params.landing_id) return String(params.landing_id);
            if (params.page_id) return String(params.page_id);
        } catch (_) {}
        try {
            const pathParts = (window.location.pathname || '').split('/');
            const pIdx = pathParts.indexOf('p');
            if (pIdx !== -1 && pathParts[pIdx + 1]) return pathParts[pIdx + 1];
        } catch (_) {}
        return 'default_landing';
    }

    // Chuẩn hoá landing key để dùng làm tên khóa lưu trữ (chỉ ký tự an toàn).
    const LANDING_KEY = (function () {
        var raw = resolveLandingKey() || 'default_landing';
        return String(raw).replace(/[^a-zA-Z0-9_\-]/g, '_').substring(0, 60) || 'default_landing';
    })();

    // Hàm bọc storage key theo landing hiện tại.
    function scopedKey(base) {
        return base + '_' + LANDING_KEY;
    }

    // Lấy hoặc tạo Visitor ID (lưu bền vững 365 ngày qua Cookie + LocalStorage)
    // ⚠️ KHÓA ĐÃ TÁCH THEO LANDING (apex_vid_<landingKey>).
    function getOrCreateVisitorId() {
        const storageKey = scopedKey('apex_vid');
        let vid = '';
        try {
            const match = document.cookie.match(new RegExp('(?:^|;\\s*)' + storageKey + '=([^;]+)'));
            if (match && match[1]) {
                vid = decodeURIComponent(match[1]);
            }
            if (!vid && window.localStorage) {
                vid = localStorage.getItem(storageKey) || '';
            }
        } catch (_) {}

        if (!vid) {
            vid = 'v_' + Math.random().toString(36).substring(2, 10) + '_' + Date.now().toString(36);
        }

        // Lưu lại cho 365 ngày
        try {
            const expires = new Date(Date.now() + 365 * 24 * 60 * 60 * 1000).toUTCString();
            document.cookie = storageKey + '=' + encodeURIComponent(vid) + '; max-age=31536000; expires=' + expires + '; path=/; SameSite=Lax';
            if (window.localStorage) {
                localStorage.setItem(storageKey, vid);
            }
        } catch (_) {}

        return vid;
    }

    // Lấy hoặc tạo Session ID (lưu trong sessionStorage cho tab hiện tại)
    function getOrCreateSessionId() {
        const storageKey = scopedKey('apex_session_id');
        let sid = '';
        try {
            sid = sessionStorage.getItem(storageKey);
            if (!sid) {
                sid = 'sess_' + Math.random().toString(36).substring(2, 10) + '_' + Date.now().toString(36);
                sessionStorage.setItem(storageKey, sid);
            }
        } catch (_) {
            sid = 'sess_' + Math.random().toString(36).substring(2, 10) + '_' + Date.now().toString(36);
        }
        return sid;
    }

    const visitorId = getOrCreateVisitorId();
    const sessionId = getOrCreateSessionId();

    // Đếm số lần phiên của visitor trên landing này (new vs returning).
    let sessionCount = 1;
    try {
        const scKey = scopedKey('apex_session_count');
        const prev = parseInt(localStorage.getItem(scKey) || '0', 10) || 0;
        sessionCount = prev + 1;
        localStorage.setItem(scKey, String(sessionCount));
    } catch (_) {}

    // ============================================================
    // 1.1 THU THẬP MACRO META ADS + MÃ ĐỊNH DANH LƯỢT NHẤP (CAPI)
    // ------------------------------------------------------------
    // Meta thay thế các macro sau trong URL lúc hiển thị quảng cáo:
    //   {{site_source_name}} {{placement}} {{campaign.id}} {{adset.id}}
    //   {{ad.id}} {{campaign.name}} {{adset.name}} {{ad.name}}
    // Chuỗi UTM khuyến nghị:
    //   utm_medium=paid&utm_source={{site_source_name}}
    //   &utm_placement={{placement}}&utm_campaign={{campaign.id}}
    //   &utm_content={{adset.id}}&utm_term={{ad.id}}
    //   &campaign_name={{campaign.name}}&adset_name={{adset.name}}&ad_name={{ad.name}}
    //
    // Ngoài ra Meta tự gắn `fbclid` vào URL mỗi lượt click, và Meta Pixel
    // ghi cookie `_fbp` (định danh trình duyệt) + `_fbc` (định danh lượt click).
    // ⇒ 3 mã này giúp gửi Conversions API khớp chính xác 100% lượt nhấp.
    function readCookie(name) {
        try {
            const parts = (document.cookie || '').split(';');
            for (let i = 0; i < parts.length; i++) {
                const kv = parts[i].trim().split('=');
                if (kv[0] === name) return decodeURIComponent(kv.slice(1).join('=') || '');
            }
        } catch (_) {}
        return '';
    }

    function collectMetaTracking(urlParams) {
        const p = urlParams || getUrlParams();
        const fbclid = (p.fbclid || '').trim();
        const igshid = (p.igshid || '').trim();

        // Cookie Meta Pixel
        let fbc = readCookie('_fbc');
        let fbp = readCookie('_fbp');

        // Dựng lại `_fbc` từ `fbclid` nếu thiếu (định dạng Meta: fb.1.<ts>.<fbclid>)
        if (!fbc && fbclid) {
            fbc = (fbclid.indexOf('fb.') === 0 && (fbclid.match(/\./g) || []).length >= 3)
                ? fbclid
                : 'fb.1.0.' + fbclid;
        }

        // `site_source_name` chỉ có 4 giá trị: fb / ig / msg / an
        let siteSourceName = (p.site_source_name || '').trim();
        if (!siteSourceName) {
            const src = (p.utm_source || '').trim().toLowerCase();
            if (src === 'facebook' || src === 'fb') siteSourceName = 'fb';
            else if (src === 'instagram' || src === 'ig') siteSourceName = 'ig';
            else if (src === 'messenger' || src === 'msg') siteSourceName = 'msg';
            else if (src === 'audience network' || src === 'an') siteSourceName = 'an';
        }

        return {
            utm_source: (p.utm_source || '').trim(),
            utm_medium: (p.utm_medium || '').trim(),
            utm_campaign: (p.utm_campaign || '').trim(),
            utm_content: (p.utm_content || '').trim(),
            utm_term: (p.utm_term || '').trim(),
            utm_placement: (p.utm_placement || p.placement || '').trim(),
            campaign_name: (p.campaign_name || '').trim(),
            adset_name: (p.adset_name || '').trim(),
            ad_name: (p.ad_name || '').trim(),
            ad_account_id: (p.ad_account_id || p.acc_id || p.ad_account || '').trim(),
            site_source_name: siteSourceName,
            fbclid: fbclid,
            fbc: fbc,
            fbp: fbp,
            igshid: igshid,
            meta_click_id: fbclid || igshid
        };
    }

    // ============================================================
    // 1.2 NHẬN DIỆN THIẾT BỊ & TRÌNH DUYỆT CHI TIẾT (PHÍA CLIENT)
    // ------------------------------------------------------------
    // Bổ sung cho phân tích User-Agent phía server:
    //   - tên & phiên bản trình duyệt (kể cả trình duyệt trong app FB/IG/Zalo)
    //   - mã model thiết bị (vd: "SM-A525F") để server map sang tên thương mại
    //   - trạng thái phần cứng: RAM, số lõi, cảm ứng, viewport, múi giờ, mạng
    function detectInAppBrowser() {
        const ua = (navigator.userAgent || '').toLowerCase();
        // Trình duyệt trong app Facebook: "FBAN" (iOS) / "FBAV" (Android)
        if (/fbav|fb_iab|fb_iab\/|fbav\/|fb_\/|fb\//.test(ua) || /\bfban\b/.test(ua)) return 'facebook';
        if (/instagram|ig_|\biga\b/.test(ua)) return 'instagram';
        if (/messenger|\bmsgr\b/.test(ua)) return 'messenger';
        if (/zalo|zalosdk/.test(ua)) return 'zalo';
        if (/tiktok|musical_ly|bytedance/.test(ua)) return 'tiktok';
        if (/line\//.test(ua)) return 'line';
        if (/coccoc|coc_coc/.test(ua)) return 'coccoc';
        return '';
    }

    function detectBrowserInfo() {
        const ua = navigator.userAgent || '';
        const u = ua.toLowerCase();
        let name = 'Other';
        let version = '';
        let engine = 'Other';

        // Ưu tiên trình duyệt trong app
        const inApp = detectInAppBrowser();
        if (inApp) {
            name = inApp === 'zalo' ? 'Zalo In-App'
                : inApp === 'coccoc' ? 'Cốc Cốc'
                : inApp.charAt(0).toUpperCase() + inApp.slice(1) + ' In-App';
            const vm = ua.match(/(?:Version|CriOS|FxiOS|EdgiOS)\/([0-9.]+)/);
            version = vm ? vm[1] : '';
            engine = 'Blink';
            return { name: name, version: version, engine: engine, inApp: inApp };
        }

        if (/edg\//.test(u)) { name = 'Edge'; const m = ua.match(/Edg(?:e|A|iOS)?\/([0-9.]+)/); version = m ? m[1] : ''; engine = 'Blink'; }
        else if (/opr\/|opera/.test(u)) { name = 'Opera'; const m = ua.match(/OPR\/([0-9.]+)/); version = m ? m[1] : ''; engine = 'Blink'; }
        else if (/samsungbrowser/.test(u)) { name = 'Samsung Internet'; const m = ua.match(/SamsungBrowser\/([0-9.]+)/); version = m ? m[1] : ''; engine = 'Blink'; }
        else if (/ucbrowser/.test(u)) { name = 'UC Browser'; const m = ua.match(/UCBrowser\/([0-9.]+)/); version = m ? m[1] : ''; engine = 'Blink'; }
        else if (/miuibrowser/.test(u)) { name = 'Mi Browser'; const m = ua.match(/MiuiBrowser\/([0-9.]+)/); version = m ? m[1] : ''; engine = 'Blink'; }
        else if (/coc_coc_browser/.test(u)) { name = 'Cốc Cốc'; const m = ua.match(/coc_coc_browser\/([0-9.]+)/); version = m ? m[1] : ''; engine = 'Blink'; }
        else if (/firefox\/|fxios/.test(u)) { name = 'Firefox'; const m = ua.match(/(?:Firefox|FxiOS)\/([0-9.]+)/); version = m ? m[1] : ''; engine = 'Gecko'; }
        else if (/chrome\/|crios/.test(u)) { name = 'Chrome'; const m = ua.match(/(?:Chrome|CriOS)\/([0-9.]+)/); version = m ? m[1] : ''; engine = 'Blink'; }
        else if (/safari\//.test(u) && !/chrome/.test(u)) { name = 'Safari'; const m = ua.match(/Version\/([0-9.]+)/); version = m ? m[1] : ''; engine = 'WebKit'; }
        else if (/msie |trident\//.test(u)) { name = 'IE'; engine = 'Trident'; }

        return { name: name, version: version, engine: engine, inApp: '' };
    }

    function detectDeviceModelCode() {
        const ua = navigator.userAgent || '';
        // Android: "Android 13; SM-A525F Build/..."
        let m = ua.match(/Android[^;]*;\s*([^;)]+?)(?:\s+Build\/|[;)])/);
        if (m && m[1]) {
            const code = m[1].trim();
            // Bỏ chuỗi build ngẫu nhiên (vd: "SM-A525F" giữ nguyên, "OPPO CPH2207" giữ nguyên)
            if (/^[A-Za-z0-9\-_]{3,32}$/.test(code)) return code;
        }
        // iOS: "iPhone14,3" → mã model
        m = ua.match(/\((iPhone|iPad|iPod)[0-9]+,[0-9]+\)/);
        if (m && m[0]) return m[0].replace(/[()]/g, '');
        return '';
    }

    function detectOSInfo() {
        const ua = navigator.userAgent || '';
        let os = 'Other';
        let version = '';
        let m;
        if (/windows/i.test(ua)) { os = 'Windows'; m = ua.match(/Windows NT ([0-9.]+)/); version = m ? m[1] : ''; }
        else if (/android/i.test(ua)) { os = 'Android'; m = ua.match(/Android[\s/]+([0-9.]+)/); version = m ? m[1] : ''; }
        else if (/iphone|ipad|ipod/i.test(ua)) { os = 'iOS'; m = ua.match(/OS ([0-9_]+)/); version = m ? m[1].replace(/_/g, '.') : ''; }
        else if (/mac os x/i.test(ua)) { os = 'macOS'; m = ua.match(/Mac OS X ([0-9_.]+)/); version = m ? m[1].replace(/_/g, '.') : ''; }
        else if (/linux/i.test(ua)) { os = 'Linux'; }
        else if (/cros/i.test(ua)) { os = 'ChromeOS'; }
        return { os: os, version: version };
    }

    function collectDeviceInfo() {
        const nav = navigator || {};
        const scr = window.screen || {};
        const browser = detectBrowserInfo();
        const osInfo = detectOSInfo();
        const conn = nav.connection || nav.webkitConnection || null;

        // Trạng thái thiết bị (mô tả ngắn, dùng để phân tích)
        const deviceStateParts = [];
        try {
            deviceStateParts.push('ram=' + (nav.deviceMemory || 'n/a'));
            deviceStateParts.push('cores=' + (nav.hardwareConcurrency || 'n/a'));
            deviceStateParts.push('touch=' + (nav.maxTouchPoints || 0));
            deviceStateParts.push('dpr=' + (window.devicePixelRatio || 1));
            deviceStateParts.push('orient=' + (screen.orientation && screen.orientation.type ? screen.orientation.type : 'n/a'));
            deviceStateParts.push('battery=' + (nav.getBattery ? 'api' : 'no'));
        } catch (_) {}

        return {
            device_type: (/tablet|ipad/i.test(nav.userAgent) ? 'tablet' : (/mobi|iphone|android/i.test(nav.userAgent) ? 'mobile' : 'desktop')),
            browser: browser.name,
            browser_version: browser.version,
            browser_engine: browser.engine,
            is_in_app_browser: !!browser.inApp,
            in_app_type: browser.inApp,
            os: osInfo.os,
            os_version: osInfo.version,
            device_model_code: detectDeviceModelCode(),
            device_memory: parseInt(nav.deviceMemory || '0', 10) || 0,
            device_cores: parseInt(nav.hardwareConcurrency || '0', 10) || 0,
            device_touchpoints: parseInt(nav.maxTouchPoints || '0', 10) || 0,
            viewport_res: (window.innerWidth || 0) + 'x' + (window.innerHeight || 0),
            screen_res: (scr.width || 0) + 'x' + (scr.height || 0),
            screen_color_depth: parseInt(scr.colorDepth || '0', 10) || 0,
            pixel_ratio: String(window.devicePixelRatio || 1),
            timezone: (function () {
                try {
                    return Intl.DateTimeFormat().resolvedOptions().timeZone || '';
                } catch (_) { return ''; }
            })(),
            language: (nav.language || (nav.languages && nav.languages[0]) || ''),
            platform_info: (nav.platform || '') + ' | ' + (nav.vendor || ''),
            connection_type: conn ? (conn.effectiveType || conn.type || '') : '',
            device_state: deviceStateParts.join('; ')
        };
    }

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

    // ─── Bộ đếm hành vi chi tiết (2026-11-12) ─────────────────────
    // visibleSeconds / hiddenSeconds: thời gian tab thật sự hiển thị (chống
    // "time on page" bị thổi phồng khi khách để tab ở chạy nền).
    let visibleSeconds = 0;
    let hiddenSeconds = 0;
    let lastVisibilityChange = Date.now();
    let hasVideoPlayed = false;
    let hasZaloClicked = false;
    let hasTelClicked = false;
    let formSubmitAttempts = 0;
    let formValidationErrors = 0;

    // Cập nhật thời gian hiển thị/ẩn mỗi khi trạng thái tab đổi.
    (function trackVisibility() {
        document.addEventListener('visibilitychange', function () {
            var now = Date.now();
            var delta = Math.round((now - lastVisibilityChange) / 1000);
            if (delta > 0) {
                if (document.visibilityState === 'hidden') hiddenSeconds += delta;
                else visibleSeconds += delta;
            }
            lastVisibilityChange = now;
        });
    })();

    // Phát hiện hành động cụ thể: video, Zalo, gọi điện.
    (function trackSpecificActions() {
        try {
            document.addEventListener('play', function (e) {
                var t = e.target;
                if (t && (t.tagName === 'VIDEO' || t.tagName === 'AUDIO')) {
                    if (!hasVideoPlayed) {
                        hasVideoPlayed = true;
                        pushAction('video_play', 'media', 'Phát video: ' + (t.currentSrc || t.src || '').substring(0, 120));
                    }
                }
            }, true);

            document.addEventListener('click', function (e) {
                var el = e.target && e.target.closest ? e.target.closest('a[href]') : null;
                if (!el) return;
                var href = el.getAttribute('href') || '';
                if (/^tel:/i.test(href) && !hasTelClicked) {
                    hasTelClicked = true;
                    pushAction('tel_click', 'a[href^=tel:]', 'Bấm gọi điện: ' + href);
                }
                if (/zalo\.me|chat\.zalo\.ua|zalo\d/i.test(href) && !hasZaloClicked) {
                    hasZaloClicked = true;
                    pushAction('zalo_click', 'a[href*=zalo]', 'Bấm mở Zalo: ' + href.substring(0, 120));
                }
            }, true);
        } catch (_) {}
    })();

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

    // ensureChallengeReady đảm bảo có 1 challenge CÒN HIỆU LỰC & ĐÃ GIẢI xong.
    //
    // ⚠️ BUG ĐÃ SỬA (2026-11-12): trước đây challenge được lấy lúc tải trang
    // và TTL chỉ 5 phút. Khách đọc nội dung + điền form > 5 phút (landing bán
    // chứng khoán có form nhiều trường) ⇒ challenge hết hạn ⇒ server từ chối với
    // lý do "hết hạn" ⇒ MẤT LEAD THẬT.
    //
    // ⇒ Luôn kiểm tra thời hạn thực tế TRƯỚC khi gửi. Nếu sắp hết hạn (< 90s)
    //   hoặc đã hết hạn ⇒ lấy challenge MỚI và giải PoW ngay tại thời điểm submit.
    const CHALLENGE_SAFETY_MARGIN = 90; // giây

    async function ensureChallengeReady(forceRefresh) {
        var nowSec = Date.now() / 1000;
        var valid = currentChallenge &&
            currentChallenge.expires_at > (nowSec + CHALLENGE_SAFETY_MARGIN) &&
            currentChallengeSolution;

        if (valid && !forceRefresh) {
            return currentChallenge;
        }

        // Challenge cũ đã hết hạn hoặc chưa có lời giải ⇒ lấy mới.
        try {
            const res = await fetch(getChallengeEndpoint() + '?difficulty=3', { cache: 'no-store' });
            if (res.ok) {
                const data = await res.json();
                if (data && data.success && data.data) {
                    currentChallenge = data.data;
                    currentChallengeSolution = null;
                    // Giải PoW ngay (bất đồng bộ) — thường chỉ 15-30ms.
                    currentChallengeSolution = await solveProofOfWork(currentChallenge);
                    return currentChallenge;
                }
            }
        } catch (_) {}

        // Không lấy được challenge mới nhưng challenge cũ vẫn còn hiệu lực
        // ⇒ dùng lại (server có dung sai clock skew 60s).
        if (currentChallenge && currentChallenge.expires_at > nowSec) {
            if (!currentChallengeSolution) {
                currentChallengeSolution = await solveProofOfWork(currentChallenge);
            }
            return currentChallenge;
        }
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
                // ⚠️ KHÓA ĐÃ TÁCH THEO LANDING (2026-11-12): dữ liệu autofill
                // của landing A không được tự điền vào form của landing B.
                const savedName = localStorage.getItem(scopedKey('apex_user_name')) || localStorage.getItem(scopedKey('apex_lead_name')) || '';
                const savedPhone = localStorage.getItem(scopedKey('apex_user_phone')) || localStorage.getItem(scopedKey('apex_lead_phone')) || '';
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
                    localStorage.setItem(scopedKey('apex_user_name'), target.value.trim());
                } catch (_) {}
            }
            if (target.id === 'phoneNumber') {
                if (target.value.trim().length >= 9) {
                    currentPhone = target.value.trim();
                }
                if (target.value.trim().length > 3) {
                    try {
                        localStorage.setItem(scopedKey('apex_user_phone'), target.value.trim());
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

        // ⚠️ KHÓA ĐÃ TÁCH THEO LANDING (2026-11-12): trước đây dùng key chung
        // `apex_lead_registered` ⇒ khách đã đăng ký landing A bị coi như đã
        // đăng ký landing B ⇒ thống kê & dedupe bị lẫn giữa các landing.
        const regKey = scopedKey('apex_lead_registered');
        const phoneKey = scopedKey('apex_lead_phone');
        const regAtKey = scopedKey('apex_registered_at');

        document.cookie = regKey + '=1; max-age=' + COOKIE_MAX_AGE + '; expires=' + expires + '; path=/; SameSite=Lax' + secureFlag;
        if (currentPhone) {
            document.cookie = phoneKey + '=' + encodeURIComponent(currentPhone) + '; max-age=' + COOKIE_MAX_AGE + '; expires=' + expires + '; path=/; SameSite=Lax' + secureFlag;
        }

        try {
            localStorage.setItem(regKey, '1');
            localStorage.setItem(regAtKey, Date.now().toString());
            if (currentPhone) {
                localStorage.setItem(phoneKey, currentPhone);
            }
        } catch (_) {}
    }

    // Chế độ Preview / Code Studio: KHÔNG được tải config từ mạng và KHÔNG được
    // ghi đè DOM — để nội dung HTML viết tay trong editor là nguồn sự thật duy nhất.
    function isStudioBuilder() {
        try {
            return !!(window.__APEX_STUDIO__ || window.__APEX_BUILDER_ACTIVE__ || window.__APEX_FREEZE_MODE__);
        } catch (_) {
            return false;
        }
    }

    function isAnyPreview() {
        try {
            if (isStudioBuilder()) return true;
            if (window.__APEX_PREVIEW__ || window.IS_PREVIEW) return true;
            if (window.self !== window.top || window.frameElement !== null) return true;
        } catch (_) {}
        return false;
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
        const regKey = scopedKey('apex_lead_registered');
        for (let i = 0; i < cookies.length; i++) {
            const parts = cookies[i].trim().split('=');
            if (parts[0] === regKey && parts[1] === '1') {
                return true;
            }
        }

        try {
            if (localStorage.getItem(regKey) === '1') {
                const regTime = parseInt(localStorage.getItem(scopedKey('apex_registered_at')) || '0', 10);
                if (regTime && (Date.now() - regTime) < COOKIE_DAYS * 24 * 60 * 60 * 1000) {
                    return true;
                }
            }
        } catch (_) {}

        return false;
    }

    function clearRegistered() {
        const regKey = scopedKey('apex_lead_registered');
        const phoneKey = scopedKey('apex_lead_phone');
        document.cookie = regKey + '=; max-age=0; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/;';
        document.cookie = phoneKey + '=; max-age=0; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/;';
        try {
            localStorage.removeItem(regKey);
            localStorage.removeItem(scopedKey('apex_registered_at'));
            localStorage.removeItem(phoneKey);
        } catch (_) {}
    }

    // ============================================================
    // 4.1 CHUẨN HOÁ URL CHUYỂN HƯỚNG SAU KHI ĐĂNG KÝ
    // ============================================================
    //
    // BỐI CẢNH (bug 2026-11-06 — landing_cu sinh lead TRÙNG SĐT):
    //   `form_config.success_redirect` trong CSDL được cấu hình là
    //   "ads.giaodichhanghoatuonglai.com/chucmung" (có domain nhúng).
    //   Khi gán vào `window.location.href`, trình duyệt phân giải tương đối
    //   theo `<base href="/landing/">` ⇒ URL thành
    //   `/landing/ads.giaodichhanghoatuonglai.com/chucmung` — VẪN nằm trong
    //   prefix của landing ⇒ edge router phục vụ LẠI trang CÓ FORM ĐĂNG KÝ.
    //   Khách bị quay lại form, bấm "Đăng ký" lần nữa ⇒ sinh thêm lead trùng
    //   SĐT (cùng thời gian) — hiện tượng "bấm đăng ký bị lặp mấy lead giống nhau".
    //
    // CHUẨN HOÁ:
    //   - URL tuyệt đối (http/https) → giữ nguyên (chủ ý chuyển sang domain khác).
    //   - "domain.com/path"           → bỏ phần domain, giữ "path" (+ ".html" nếu thiếu).
    //   - Đường dẫn tương đối / trang nội bộ khác → giữ nguyên.
    //   - Nếu kết quả trỏ về CHÍNH trang đang xem (có form đăng ký) → dùng
    //     'chucmung.html' để chắc chắn rời khỏi form.
    function normalizeSuccessRedirect(raw) {
        let v = String(raw || '').trim();
        if (!v) return 'chucmung.html';

        // URL tuyệt đối → giữ nguyên.
        if (/^https?:\/\//i.test(v)) return v;

        // Bỏ query/hash.
        v = v.split('#')[0].split('?')[0];

        // "domain.com/path" (domain nhúng, KHÔNG có scheme) → chỉ giữ phần path.
        let strippedDomain = false;
        const m = v.match(/^[^/]*\.[^/]*\/(.*)$/);
        if (m && m[1]) {
            v = m[1].replace(/^\/+/, '');
            strippedDomain = true;
        }

        v = v.replace(/^\/+/, '');
        if (!v) return 'chucmung.html';

        // Trang nội bộ không có đuôi file → thêm ".html" để KHÔNG bị rơi về index
        // của landing (index chứa form đăng ký ⇒ khách bấm lại sinh lead trùng).
        if (v.indexOf('.') === -1 && (strippedDomain || v.toLowerCase() === 'chucmung')) {
            v += '.html';
        }

        // Chuyển hướng về chính trang đang xem (có form) → chuyển sang trang cảm ơn.
        const curFile = (window.location.pathname || '').split('/').pop().toLowerCase();
        if (curFile && v.toLowerCase() === curFile) {
            v = 'chucmung.html';
        }

        return v;
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

    // ============================================================
    // 3.9 KIỂM TRA HTML ĐÃ CÓ META PIXEL HARDCODE
    // ============================================================
    //
    // Mục đích: landing export từ LadiPage đã có sẵn block Meta
    // Pixel trong HTML (fbq('init') + fbq('track','PageView')). Nếu JS công
    // inject thêm → PageView nhân đôi → Meta Ads Manager đếm sai.
    //
    // Để phát hiện chỉ quan sát DOM + nội dung script đã nạp.
    function htmlHasMetaPixel() {
        try {
            // 1. Thư viện fbq đã tồn tại → đã init rồi.
            if (typeof window.fbq === 'function') return true;

            // 2. Quét text trong các thẻ script đã nạp.
            var scripts = document.querySelectorAll('script');
            for (var i = 0; i < scripts.length; i++) {
                var txt = scripts[i].textContent || '';
                if (!txt) continue;
                if (txt.indexOf('fbevents.js') !== -1) return true;
                if (txt.indexOf("fbq('init'") !== -1 || txt.indexOf('fbq("init"') !== -1) return true;
            }
        } catch (_) {}
        return false;
    }

    // ============================================================
    // APPLY CONFIG TO DOM
    // ------------------------------------------------------------
    // NGUYÊN TẮC: HTML viết tay trong Code Studio là NGUỒN SỰ THẬT DUY NHẤT.
    // Hệ thống KHÔNG tự động ghi đè nội dung, link tel:/zalo.me, tiêu đề hero,
    // ô thông tin hay thời gian sự kiện trên DOM. Muốn đổi gì thì sửa trực tiếp
    // trong mã HTML. Cấu hình động chỉ dùng cho: Meta Title + Pixel Tracking.
    // ============================================================
    function applyConfigToDOM(cfg) {
        if (!cfg || typeof cfg !== 'object') return;

        // 0. Code Studio / Preview: không áp dụng bất kỳ config nào lên DOM.
        //    HTML viết tay trong editor là nguồn sự thật duy nhất.
        if (isAnyPreview()) return;

        // 1. Meta Title (SEO) — an toàn, không ảnh hưởng nội dung giao diện
        const pageTitle = cfg.title || (cfg.settings && cfg.settings.title);
        if (pageTitle && typeof document !== 'undefined') {
            document.title = pageTitle;
        }

        // 4. Client-side Pixel Auto-Injection nếu backend chưa inject sẵn
        //
        // ⚠️ CHỐNG CHÈN TRÜNG PIXEL (fix 2026-11-06):
        // HTML export từ LadiPage đã có sẵn block Meta Pixel hardcode
        // (`fbq('init', ...)` + `fbq('track', 'PageView')`). Nếu tracker.js vẫn
        // inject thêm 1 block nữa → PageView nhân đôi → Meta Ads
        // Manager đếm sai lượt hiển thị, quy ngữ quy đổi.
        //
        // Điều kiện không inject:
        //   - `window.fbq` đã tồn tại (backend đã inject rồi).
        //   - HTML đã có `fbq('init', ...)` hoặc nạp `fbevents.js`.
        const fbPixelId = cfg.facebook_pixel_id || (cfg.settings && cfg.settings.facebook_pixel_id);
        if (fbPixelId && !window.fbq && !htmlHasMetaPixel()) {
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
        // Code Studio / Preview: KHÔNG tải config từ mạng.
        // Lý do: HTML trong editor có thể chứa sẵn `window.APEX_LANDING_CONFIG` của
        // template gốc (vd: mxv_080_online) → nếu tải/ghi đè, preview sẽ hiển thị
        // sai landing. HTML viết tay trong Code Studio là nguồn sự thật duy nhất.
        if (isAnyPreview()) {
            return window.APEX_LANDING_CONFIG || {};
        }

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
        // Ưu tiên `data-landing-id` trên thẻ <script src="tracker.js"> — backend luôn
        // gắn đúng page_id của landing đang phục vụ (bug chồng chéo landing 2026-11-05).
        if (!currentLandingId) {
            try {
                const s = document.currentScript || document.querySelector('script[src*="tracker.js"]');
                if (s) {
                    const attr = s.getAttribute('data-landing-id') || '';
                    if (attr) currentLandingId = attr;
                }
            } catch (_) {}
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

        // Thứ tự ưu tiên landing_id (quan trọng — bug chồng chéo landing 2026-11-05):
        // 1. `data-landing-id` trên thẻ <script src="tracker.js"> — backend luôn gắn
        //    đúng page_id của landing đang phục vụ ⇒ CHẮC CHẮN ĐÚNG.
        // 2. `window.APEX_LANDING_CONFIG.landing_id` — backend inject trước tracker.
        // 3. `window.APEX_LANDING_ID` — biến JS do landing tự khai báo.
        // 4. URL `/p/<page_id>` — chỉ dùng cho landing phục vụ qua đường dẫn /p/.
        //
       // BUG CŨ: đưa `globalConfig.landing_id` lên đầu làm landing dùng chung template
        // (vd: `mxv_080_online`) ghi đè ⇒ analytics/lead về nhầm landing.
        const urlParams = getUrlParams();
        let urlLandingId = urlParams.landing_id || urlParams.page_id || '';
        if (!urlLandingId) {
            const pathParts = (window.location.pathname || '').split('/');
            const pIdx = pathParts.indexOf('p');
            if (pIdx !== -1 && pathParts[pIdx + 1]) {
                urlLandingId = pathParts[pIdx + 1];
            }
        }

        const landingId = scriptLandingId ||
            globalConfig.landing_id || globalConfig.landingId || window.APEX_LANDING_ID ||
            urlLandingId ||
            (window.LadiPageScript && window.LadiPageScript.runtime && window.LadiPageScript.runtime.ladipage_id) ||
            'default_landing';

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
        const meta = collectMetaTracking(urlParams);
        const dev = collectDeviceInfo();

        const eventsToSend = (extraEvents || []).concat(actionTimeline.slice(-30));

        return {
            session_id: sessionId,
            visitor_id: visitorId,
            landing_id: landingCfg.landing_id,
            nguon: landingCfg.nguon,
            // ─── Macro Meta Ads đầy đủ ───
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
            // ─── Định danh lượt nhấp Meta (CAPI) ───
            fbclid: meta.fbclid,
            fbc: meta.fbc,
            fbp: meta.fbp,
            igshid: meta.igshid,
            meta_click_id: meta.meta_click_id,
            referrer: document.referrer || '',
            entry_url: window.location.href,
            device_type: dev.device_type,
            browser: dev.browser,
            os: dev.os,
            screen_res: dev.screen_res,
            // ─── Thiết bị & trình duyệt chi tiết ───
            browser_version: dev.browser_version,
            browser_engine: dev.browser_engine,
            is_in_app_browser: dev.is_in_app_browser,
            in_app_type: dev.in_app_type,
            os_version: dev.os_version,
            device_model_code: dev.device_model_code,
            device_memory: dev.device_memory,
            device_cores: dev.device_cores,
            device_touchpoints: dev.device_touchpoints,
            viewport_res: dev.viewport_res,
            screen_color_depth: dev.screen_color_depth,
            pixel_ratio: dev.pixel_ratio,
            timezone: dev.timezone,
            language: dev.language,
            platform_info: dev.platform_info,
            connection_type: dev.connection_type,
            device_state: dev.device_state,
            // ─── Hành vi chi tiết ───
            visible_seconds: visibleSeconds,
            hidden_seconds: hiddenSeconds,
            session_count: sessionCount,
            landing_path: window.location.pathname || '',
            referrer_host: (function () {
                try { return document.referrer ? new URL(document.referrer).host : ''; } catch (_) { return ''; }
            })(),
            is_new_visitor: sessionCount <= 1,
            has_video_play: hasVideoPlayed,
            has_zalo_click: hasZaloClicked,
            has_tel_click: hasTelClicked,
            form_submit_attempts: formSubmitAttempts,
            form_validation_errors: formValidationErrors,
            scroll_depth_25: !!scrollMilestones[25],
            scroll_depth_50: !!scrollMilestones[50],
            scroll_depth_75: !!scrollMilestones[75],
            scroll_depth_90: !!scrollMilestones[90],
            scroll_depth_100: !!scrollMilestones[100],
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
    //
    // ⚠️ THROTTLE (fix 2026-11-13):
    // `ApexTracker.send()` (do landing gọi khi có tương tác: click, input, modal_open…)
    // đẩy 1 action + gọi `sendAnalyticsPing()` ngay. Landing `doanh_chung` gọi
    // `trackEvent()` ở MỖI keystroke trong ô tên/SĐT ⇒ mỗi phím gõ = 1 POST
    // `/api/analytics/track`. Trang dài + người dùng nhập nhanh ⇒ hàng chục
    // request/giây ⇒ server bị nghiêng, log đầy lỗi 500, dữ liệu analytics bị
    // ghi đè liên tục (mất event thật do `ON CONFLICT` chỉ giữ giá trị mới nhất).
    //
    // CÁCH SỬA: gom (coalesce) các ping gần nhau thành 1 request.
    // - `sendAnalyticsPing()` từ code hệ thống (pageview, heartbeat, exit, converted)
    //   vẫn gửi NGAY ⇒ không mất dữ liệu quan trọng.
    // - `ApexTracker.send()` (event tương tác) dùng `sendAnalyticsPingThrottled()`
    //   ⇒ gộp trong cửa sổ 3 giây, nhưng luôn flush khi page unload/visibility hidden.
    const ANALYTICS_THROTTLE_MS = 3000;
    let analyticsThrottleTimer = null;
    let analyticsPendingEvents = [];

    // Gửi ngay, không throttle (dùng cho pageview / heartbeat / exit / converted).
    function sendAnalyticsPing(extraEvents) {
        // Code Studio / Preview: KHÔNG gửi analytics để tránh làm nhiễu dữ liệu thật.
        if (isAnyPreview()) return;

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

    // Gửi analytics có throttle — dùng cho event tương tác liên tục (keystroke...).
    //
    // Gom nhiều lời gọi trong `ANALYTICS_THROTTLE_MS` thành 1 request duy nhất.
    // Events đang chờ được giữ lại (`analyticsPendingEvents`) và gửi kèm lần flush
    // ⇒ không mất hành vi người dùng, chỉ giảm số lần gọi HTTP.
    function sendAnalyticsPingThrottled(extraEvents) {
        if (isAnyPreview()) return;

        if (extraEvents && extraEvents.length) {
            analyticsPendingEvents = analyticsPendingEvents.concat(extraEvents);
        }

        if (analyticsThrottleTimer) return; // đang chờ flush

        analyticsThrottleTimer = setTimeout(function () {
            analyticsThrottleTimer = null;
            const pending = analyticsPendingEvents;
            analyticsPendingEvents = [];
            sendAnalyticsPing(pending);
        }, ANALYTICS_THROTTLE_MS);
    }

    // Flush ngay các event đang chờ (dùng khi rời trang) — không để mất dữ liệu.
    function flushAnalyticsPending() {
        if (analyticsThrottleTimer) {
            clearTimeout(analyticsThrottleTimer);
            analyticsThrottleTimer = null;
        }
        if (!analyticsPendingEvents.length) return;
        const pending = analyticsPendingEvents;
        analyticsPendingEvents = [];
        sendAnalyticsPing(pending);
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
            // Flush các event tương tác đang chờ TRƯỚC khi gửi ping exit,
            // để không mất dữ liệu keystroke/click đang chờ throttle.
            flushAnalyticsPending();
            sendAnalyticsPing();
        }
    });
    window.addEventListener('beforeunload', function () {
        pushAction('exit', 'window_close', 'Người dùng đóng trang web');
        flushAnalyticsPending();
        sendAnalyticsPing();
    });

    // ============================================================
    // 6. GỬI LEAD ĐĂNG KÝ VỀ BACKEND & LIÊN KẾT SESSION (STITCHING)
    // ============================================================

    // disableSubmitButtons khoá tất cả nút submit của form trong lúc đang gửi.
    //
    // Bảo vệ chống double-click / Enter nhiều lần ⇒ tránh gửi trùng payload
    // (server coi request thứ 2 là replay attack → lead thật bị gắn nhãn sai).
    function disableSubmitButtons(disabled) {
        try {
            const btns = document.querySelectorAll('button[type="submit"], input[type="submit"], button[data-action="submit"], .btn-submit, #submitBtn, #btnSubmit');
            for (let i = 0; i < btns.length; i++) {
                const b = btns[i];
                if (disabled) {
                    if (!b.getAttribute('data-apex-orig-html')) {
                        b.setAttribute('data-apex-orig-html', b.innerHTML || b.value || '');
                    }
                    b.disabled = true;
                    if (b.tagName === 'INPUT') b.value = 'Đang gửi thông tin...';
                    else b.innerHTML = '<span class="apex-spin">⏳</span> Đang gửi thông tin...';
                } else {
                    const orig = b.getAttribute('data-apex-orig-html');
                    b.disabled = false;
                    if (orig) {
                        if (b.tagName === 'INPUT') b.value = orig;
                        else b.innerHTML = orig;
                    }
                }
            }
        } catch (_) {}
    }

    // CHỐNG GỬI TRÙNG PAYLOAD (sửa bug 2026-11-05)
    //
    // BỐI CẢNH: landing có NHIỀU handler submit cùng chạy cho 1 lần bấm:
    //   1) capture listener trong index.html (`document.addEventListener('submit')`)
    //   2) inline `onsubmit="handleFormSubmit(event)"` trong index.html
    //   3) interceptor của tracker.js (`initAutoFormInterceptor`)
    // Cả 3 đều gọi `sendLeadToBackend` ⇒ cùng 1 payload (kể cả cùng
    // challenge_id + challenge_solution) được POST lên /api/leads 3 lần.
    //
    // HẬU QUẢ: request #1 tạo lead (về CRM + Google Sheets — đúng), nhưng
    // request #2, #3 dùng lại nonce PoW đã tiêu thụ ⇒ server ghi vào
    // spam_quarantine với lý do "Replay attack detected" ⇒ LEAD THẬT bị
    // hiển thị sai là "Lỗi Proof-of-Work" trong tab Bảo mật & Bot.
    //
    // CÁCH SỬA: khóa gửi theo SĐT trong cửa sổ ngắn (30s). Chỉ request đầu
    // tiên được gửi; các lần sau bị bỏ qua (idempotent).
    //
    // Lưu ý: KHÔNG dùng cờ `data-apex-submitting` vì cờ đó chỉ được tracker's
    // interceptor set — 2 handler còn lại không hề biết ⇒ vẫn gửi trùng.
    const LEAD_DEDUP_WINDOW_MS = 30000;
    let leadDedupLock = { phone: '', at: 0 };

    function isDuplicateLeadSubmit(phone) {
        const key = String(phone || '').replace(/[^\d+]/g, '');
        if (!key) return false;
        const now = Date.now();
        return leadDedupLock.phone === key && (now - leadDedupLock.at) < LEAD_DEDUP_WINDOW_MS;
    }

    function lockLeadSubmit(phone) {
        leadDedupLock = {
            phone: String(phone || '').replace(/[^\d+]/g, ''),
            at: Date.now()
        };
    }

    // Mở lại khoá khi request thất bại (mạng lỗi / server lỗi) để khách có thể
    // bấm lại nút đăng ký. Chỉ mở khoá khi chắc chắn server CHƯA nhận lead.
    function unlockLeadSubmit() {
        leadDedupLock = { phone: '', at: 0 };
    }

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

        // ⚠️ CHỐNG GỬI TRÙNG: nhiều handler submit có thể gọi hàm này cho cùng
        // 1 lần bấm của khách (xem giải thích ở LEAD_DEDUP_WINDOW_MS).
        // Gửi trùng làm request sau bị server ghi vào spam_quarantine ⇒ lead thật
        // bị gắn nhần nhãn "Lỗi Proof-of-Work".
        if (isDuplicateLeadSubmit(phone)) {
            return { success: true, message: 'Dữ liệu lead đã được ghi nhận thành công', duplicate: true };
        }
        lockLeadSubmit(phone);

        // ⚠️ KHOÁ NÚT GỬI NGAY LẬP TỨC (2026-11-12)
        //
        // Lý do: khách double-click (hoặc dùng Enter 2 lần) ⇒ 2 request gửi
        // song song với CÙNG payload. Request #1 tạo lead thật, request #2 bị
        // server ghi vào vùng cách ly với lý do "Replay attack" ⇒ lead thật bị
        // hiển thị sai là "Lỗi Proof-of-Work".
        //
        // ⇒ Disable tất cả nút submit ngay khi bắt đầu gửi (trước cả khi await).
        disableSubmitButtons(true);

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
        //
        // ⚠️ SỬA BUG MẤT LEAD (2026-11-12): trước đây chỉ giải PoW khi
        // `currentChallengeSolution` rỗng. Nếu challenge đã hết hạn (khách ở
        // trang > TTL) ⇒ server trả lỗi "hết hạn" ⇒ lead thật bị mất.
        // ⇒ LUÔN gọi ensureChallengeReady() trước khi gửi để lấy challenge
        //   còn hiệu lực & giải lại ngay tại thời điểm bấm nút.
        formSubmitAttempts++;
        try {
            await ensureChallengeReady(false);
        } catch (_) {}

        const urlParams = getUrlParams();
        const meta = collectMetaTracking(urlParams);
        const dev = collectDeviceInfo();
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
            // ─── Macro Meta Ads đầy đủ ───
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
            // ─── Định danh lượt nhấp Meta (CAPI) ───
            fbclid: meta.fbclid,
            fbc: meta.fbc,
            fbp: meta.fbp,
            igshid: meta.igshid,
            meta_click_id: meta.meta_click_id,
            // ─── Thiết bị & trình duyệt chi tiết ───
            device_type: dev.device_type,
            browser: dev.browser,
            browser_version: dev.browser_version,
            browser_engine: dev.browser_engine,
            is_in_app_browser: dev.is_in_app_browser,
            in_app_type: dev.in_app_type,
            os: dev.os,
            os_version: dev.os_version,
            device_model_code: dev.device_model_code,
            device_memory: dev.device_memory,
            device_cores: dev.device_cores,
            device_touchpoints: dev.device_touchpoints,
            viewport_res: dev.viewport_res,
            screen_res: dev.screen_res,
            screen_color_depth: dev.screen_color_depth,
            pixel_ratio: dev.pixel_ratio,
            timezone: dev.timezone,
            language: dev.language,
            platform_info: dev.platform_info,
            connection_type: dev.connection_type,
            device_state: dev.device_state,
            // ─── Hành vi ───
            visible_seconds: visibleSeconds,
            hidden_seconds: hiddenSeconds,
            session_count: sessionCount,
            is_new_visitor: sessionCount <= 1,
            has_video_play: hasVideoPlayed,
            has_zalo_click: hasZaloClicked,
            has_tel_click: hasTelClicked,
            scroll_depth_25: !!scrollMilestones[25],
            scroll_depth_50: !!scrollMilestones[50],
            scroll_depth_75: !!scrollMilestones[75],
            scroll_depth_90: !!scrollMilestones[90],
            scroll_depth_100: !!scrollMilestones[100],
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
            // Server trả lỗi (4xx/5xx) ⇒ lead chưa chắc đã được tạo ⇒ mở khoá
            // để khách bấm lại được.
            unlockLeadSubmit();
            disableSubmitButtons(false);
        } catch (err) {
            // Lỗi mạng ⇒ có thể request chưa tới server ⇒ mở khoá cho lần thử lại.
            unlockLeadSubmit();
            disableSubmitButtons(false);
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
        formValidationErrors++;
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
        // Code Studio / Preview: KHÔNG chặn form, KHÔNG gửi lead thật.
        // HTML viết tay trong editor là nguồn sự thật duy nhất.
        if (isAnyPreview()) return;

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

                    // Xác định trang chuyển hướng sau khi gửi lead thành công.
                    // Ưu tiên biến khai báo trong chính mã HTML (nguồn sự thật duy nhất),
                    // sau đó mới fallback về cấu hình CSDL cũ (tương thích ngược).
                    const redirectUrl = normalizeSuccessRedirect(
                        window.APEX_SUCCESS_REDIRECT ||
                        (formMapping && formMapping.success_redirect) ||
                        (globalConfig.form_config && globalConfig.form_config.success_redirect) ||
                        globalConfig.redirect_url ||
                        'chucmung.html'
                    );

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
        landingKey: LANDING_KEY,
        sendLeadToBackend: sendLeadToBackend,
        sendAnalyticsPing: sendAnalyticsPing,
        pushAction: pushAction,
        // Alias tương thích ngược với tracker.js của landing cũ (doanh_chung):
        // landing gọi `window.ApexTracker.send(type, value)` ⇒ đẩy vào timeline.
        send: function (type, value) {
            pushAction(String(type || 'event'), 'custom', String(value || ''));
            // ⚠️ Dùng bản THROTTLED — landing gọi `trackEvent()` ở mỗi keystroke
            // ⇒ gửi ngay sẽ tạo hàng chục POST /api/analytics/track mỗi giây.
            sendAnalyticsPingThrottled();
        },
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
        // API mới (2026-11-12) — dùng cho landing tích hợp riêng (vd: doanh_chung)
        collectMetaTracking: collectMetaTracking,
        collectDeviceInfo: collectDeviceInfo,
        detectInAppBrowser: detectInAppBrowser,
        ensureChallengeReady: ensureChallengeReady,
        getTrackingState: function () {
            return {
                visible_seconds: visibleSeconds,
                hidden_seconds: hiddenSeconds,
                session_count: sessionCount,
                has_video_play: hasVideoPlayed,
                has_zalo_click: hasZaloClicked,
                has_tel_click: hasTelClicked,
                form_submit_attempts: formSubmitAttempts,
                form_validation_errors: formValidationErrors,
                scroll_milestones: scrollMilestones,
                max_scroll: maxScrollDepth,
                clicks_count: totalClicksCount
            };
        },
        isAutofilled: function () { return isFormAutofilled; },
        getAutofillSource: function () { return autofillSource; }
    };

})(window, document);
