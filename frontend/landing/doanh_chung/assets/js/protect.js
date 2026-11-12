/* ============================================
   APEX FINTECH - PROTECT.JS
   Bảo vệ frontend production:
     - Chặn chuột phải (context menu)
     - Chặn F12, Ctrl+Shift+I/J/C, Ctrl+U (view source)

   Lưu ý: Đây chỉ là lớp bảo vệ "thân thiện" — DevTools vẫn có thể bypass
   được. Mục tiêu chính: NGĂN người dùng phổ thông F12 xem code/logic.
   Bảo mật thật sự phải ở backend (Google Apps Script) — không lộ secret
   ở client. Xem code.gs để rotate secret nếu đã leak.

   ⚠️  ĐÃ TẮT TẠM THỜI: phần phát hiện DevTools đang mở (DevTools detection).
       Lý do: phương pháp so sánh outerWidth/innerWidth gây false-positive
       trên nhiều môi trường (mobile, ChromeOS tablet mode, ...), khiến
       landing bị khóa nhầm dù user không bật DevTools.
       Khi cần bật lại, dùng phương pháp detect an toàn hơn (vd: console.log
       override + debugger timing, hoặc dùng thư viện như devtools-detect).
   ============================================ */

(function(){
  'use strict';

  // Master switch — đặt false để tắt toàn bộ (ví dụ: khi cần debug)
  var PROTECT_ENABLED = true;

  if (!PROTECT_ENABLED) return;

  // ════════════════════════════════════════════════════════════════
  // 1. Chặn chuột phải
  // ════════════════════════════════════════════════════════════════
  document.addEventListener('contextmenu', function(e){
    e.preventDefault();
    return false;
  });

  // ════════════════════════════════════════════════════════════════
  // 2. Chặn phím tắt mở DevTools / View Source
  //    F12           → DevTools
  //    Ctrl+Shift+I  → DevTools (Inspect)
  //    Ctrl+Shift+J  → DevTools (Console)
  //    Ctrl+Shift+C  → DevTools (Element picker)
  //    Ctrl+U        → View Source
  //    Ctrl+S         → Save Page (chặn luôn)
  //    Cmd (Mac) variants cũng được cover
  // ════════════════════════════════════════════════════════════════
  document.addEventListener('keydown', function(e){
    var key = (e.key || '').toLowerCase();
    var blocked = false;

    // F12
    if (key === 'f12') blocked = true;

    // Ctrl + Shift + I/J/C
    if (e.ctrlKey && e.shiftKey && (key === 'i' || key === 'j' || key === 'c')) blocked = true;

    // Ctrl + U (view source)
    if (e.ctrlKey && key === 'u') blocked = true;

    // Ctrl + S (save page)
    if (e.ctrlKey && key === 's') blocked = true;

    // Cmd + Option + I (Mac DevTools)
    if (e.metaKey && e.altKey && key === 'i') blocked = true;

    if (blocked) {
      e.preventDefault();
      e.stopPropagation();
      return false;
    }
  }, true);

  // ════════════════════════════════════════════════════════════════
  // 3. Phát hiện DevTools đang mở — ĐÃ TẮT TẠM THỜI
  //    (Xem comment ở đầu file để biết lý do & cách bật lại)
  // ════════════════════════════════════════════════════════════════

})();
