/**
 * APEX FINTECH - CRM & ADMIN CONFIGURATION
 * Hỗ trợ chạy Admin độc lập trên domain riêng (ví dụ: crm.hanghoaphaisinh.net/ads)
 * hoặc bất kỳ môi trường web hosting / CDN / Cloudflare Pages / Vercel nào.
 */
window.APEX_ADMIN_CONFIG = (function() {
    // 1. Kiểm tra xem người dùng có lưu cấu hình tùy chỉnh trong localStorage không
    var savedEndpoint = null;
    try {
        savedEndpoint = localStorage.getItem('apex_api_endpoint');
    } catch (_) {}

    // 2. Xác định API Base URL mặc định
    var defaultApiUrl = '';
    var hostname = window.location.hostname;

    if (savedEndpoint && savedEndpoint.trim() !== '') {
        defaultApiUrl = savedEndpoint.trim().replace(/\/+$/, '');
    } else if (hostname === 'localhost' || hostname === '127.0.0.1') {
        // Môi trường Dev Local: nếu truy cập trực tiếp port 8080 thì dùng relative, port khác (LiveServer) trỏ :8080
        defaultApiUrl = window.location.port === '8080' ? '' : 'http://localhost:8080';
    } else if (hostname === 'crm.hanghoaphaisinh.net' || hostname.indexOf('hanghoaphaisinh.net') !== -1) {
        // Môi trường Production: mặc định gọi relative cùng origin (https://crm.hanghoaphaisinh.net/api/...)
        // Tránh lỗi khi chưa cấu hình riêng subdomain api.hanghoaphaisinh.net
        defaultApiUrl = '';
    } else {
        // Domain khác hoặc hosting bất kỳ: dùng relative URL
        defaultApiUrl = '';
    }

    return {
        // Endpoint Backend API
        API_BASE_URL: defaultApiUrl,

        // Đường dẫn gốc của Admin UI (ví dụ: '/ads' hoặc '/admin')
        BASE_PATH: '/ads',

        // Cấu hình NS chuẩn của hệ thống
        SYSTEM_NAMESERVERS: [
            'ns1.hanghoaphaisinh.net',
            'ns2.hanghoaphaisinh.net'
        ],

        // Server IP mục tiêu
        TARGET_SERVER_IP: '103.124.95.232',

        // Hàm cập nhật Endpoint tức thì
        setApiEndpoint: function(url) {
            var cleanUrl = (url || '').trim().replace(/\/+$/, '');
            try {
                if (cleanUrl) {
                    localStorage.setItem('apex_api_endpoint', cleanUrl);
                } else {
                    localStorage.removeItem('apex_api_endpoint');
                }
            } catch (_) {}
            this.API_BASE_URL = cleanUrl;
            return cleanUrl;
        }
    };
})();
