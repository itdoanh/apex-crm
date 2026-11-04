/* ==============================================================
   APEX VISUAL WEB BUILDER - FORM AUTO-SCANNER & DYNAMIC MAPPING
   ==============================================================

   Bộ quét form & UI ánh xạ trường đã được gỡ bỏ (2026-11-04).
   Lý do: các UI này ép ghi đè nội dung landing (hotline, zalo, hero title,
   metric box, dynamic content) và gây rối khi sửa tay trong Code Studio.
   HTML viết tay trong Code Studio là nguồn sự thật duy nhất.

   Ánh xạ trường lead (form_mapping trong CSDL) vẫn hoạt động bình thường:
   tracker.js đọc map sẵn có trong CSDL và tự nhận diện input theo heuristic
   (input[type=tel], name*fullname, name*email, textarea...).
   ============================================================== */
