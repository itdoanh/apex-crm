/**
 * Cấu hình Landing Page & Kết nối Backend API
 * Dùng khi Landing Page được host trên domain riêng biệt với Backend.
 */
window.APEX_CONFIG = {
    landing_id: 'mxv_080_online',
    nguon: 'landing_chinh',
    // Backend API URL (bỏ trống nếu chạy cùng host qua reverse proxy)
    api_url: (window.location.protocol === 'file:' || (window.location.hostname === 'localhost' && window.location.port !== '8080'))
        ? 'http://localhost:8080'
        : ''
};
