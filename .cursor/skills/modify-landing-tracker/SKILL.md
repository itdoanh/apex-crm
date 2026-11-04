---
name: modify-landing-tracker
description: Sửa landing page HTML, tracker.js, config.json hoặc form đăng ký trong dự án Apex Fintech mà không làm hỏng tracking, anti-bot PoW hay autofill. Dùng khi người dùng yêu cầu thêm form, sửa nội dung landing, đổi màu/chữ, hoặc chỉnh tracker.
---

# Sửa Landing & Tracker — Apex Fintech

Landing page là phễu chuyển đổi → mọi thay đổi ảnh hưởng doanh thu. Sửa sai có thể **làm mất lead** mà không có lỗi hiển thị.

---

## File cần sửa

| File | Vai trò |
|---|---|
| `frontend/landing/ladi.html` | Landing chính (LadiPage format) |
| `frontend/landing/chucmung.html` | Trang cảm ơn sau đăng ký |
| `frontend/landing/tracker.js` | Thu thập data + PoW solver + analytics |
| `frontend/landing/config.json` | Config per-landing (ngày giờ, suất, nguồn) |

---

## 🚫 Tuyệt đối không được

| Sai lầm | Hậu quả |
|---|---|
| Xoá field honeypot khỏi DOM | Bot spam không bị chặn → đầy rác trong CRM |
| Bỏ `tracker.js` | Mất tracking + mất PoW → **mọi lead bị 403** |
| Gửi `POST /api/leads` trước khi giải PoW xong | Lead bị reject |
| Xoá `challenge_*` khỏi payload | Lead bị reject |
| Phá logic autofill-safe | CR giảm mạnh (autofill là nguồn lead lớn) |
| Xoá `data-config` binding | Nội dung động ngừng cập nhật |
| `autocomplete="off"` trên field chính | Chặn autofill |
| `<input type="number">` cho SĐT | Mất số 0 đầu → fail validator |
| Xoá `data-ve-id` khi clone element | Studio mất liên kết DOM↔code |

---

## ✅ Yêu cầu tối thiểu cho form

```html
<form id="leadForm">
  <input type="text" name="fullName" required placeholder="Họ và tên">
  <input type="tel"  name="phone"  required placeholder="Số điện thoại">

  <!-- HONEYPOT: ẩn bằng CSS thuần, phải CÒN trong DOM -->
  <input type="text" name="website" style="display:none" tabindex="-1" autocomplete="off">
  <input type="text" name="fax_company_prefix" style="visibility:hidden" tabindex="-1" autocomplete="off">

  <button type="submit">ĐĂNG KÝ</button>
</form>

<script src="tracker.js"></script>
```

### Tại sao `name="fullName"` / `name="phone"` quan trọng
- `phone` → validator SĐT VN 10 số.
- `fullName` → backend đọc `tracking->>'fullName'` để hiện tên trong CRM. Đổi tên field → CRM hiện tên trống.

---

## Sửa nội dung động — qua `config.json`

**Không** hardcode ngày/giờ/số suất vào HTML. Sửa `config.json`:

```json
{
  "landing_id": "mxv_080_online",
  "nguon": "landing_chinh",
  "thoi_gian_chia_se": {
    "target_date": "2026-11-03T20:00:00",
    "ngay": "03/11/2026",
    "gio": "20:00",
    "thoi_luong": "2 giờ",
    "hien_thi": "20:00 - Thứ Ba, ngày 03/11/2026",
    "hinh_thuc": "Học Online qua Zoom"
  },
  "khoa_hoc": {
    "ten": "Giải Mã Đầu Tư Hàng Hóa",
    "suat_hoc": "Chỉ dành cho 100 suất",
    "da_dang_ky": "Đã có 79 suất đăng ký"
  },
  "backend_url": "/api/leads"
}
```

Trong HTML:
```html
<span data-config="thoi_gian_chia_se.hien_thi">20:00 ngày 03/11/2026</span>
<span data-config="khoa_hoc.da_dang_ky">Đã có 79 suất đăng ký</span>
```

`tracker.js` tự điền mọi element có `data-config`.

### Đổi nguồn traffic
`nguon` trong `config.json` = giá trị ghi vào cột `nguon` của lead. Đổi nó → phân tích hiệu quả nguồn đổi theo. Phải khớp với giá trị filter trong CRM.

---

## Thêm pixel / script

Đặt trong `settings` của landing trong CRM (không hardcode vào HTML):
- `facebook_pixel_id`
- `google_analytics_id`
- `tiktok_pixel_id`
- `custom_head_scripts`
- `custom_body_scripts`

Backend tự inject khi render (`InjectDynamicScripts`).

---

## Sau khi sửa — test bắt buộc

```
□ Mở landing → F12 → Console không có error
□ Submit form → Network tab thấy POST /api/leads trả 200
□ Post-redirect → chucmung.html hiện đúng
□ Lead vào CRM (refresh danh sách) → đủ SĐT + tên
□ Customer 360° → tracking đầy đủ (scroll, clicks, time_on_page, UTM)
□ Honeypot vẫn trong DOM (inspect element)
□ Test Chrome autofill → vẫn gửi được (không bị chặn nhầm)
□ Mobile viewport (375px) → form hiển thị tốt
□ Content binding: đổi config.json → refresh → nội dung đổi theo
```

### Test nhanh bằng curl
```bash
curl -X POST http://localhost:8080/api/leads \
  -H "Content-Type: application/json" \
  -d '{"fullName":"Test","phone":"0984386538","landing_id":"mxv_080_online","nguon":"landing_chinh"}'
```
→ 403 vì thiếu PoW. Cần lấy challenge trước:
```bash
curl "http://localhost:8080/api/auth/challenge?difficulty=3"
```

---

## Deploy

Landing nằm trong git → deploy theo quy trình chuẩn:
```
sửa local → test → commit → push → git pull server → restart task → verify
```

Nếu landing được quản lý trong CRM (Studio / File Manager) → sửa trực tiếp trên UI, **không** sửa file trên server.

---

## Tài liệu liên quan

- `docs/LANDING.md` — hướng dẫn đầy đủ
- `docs/SECURITY.md` mục 2 (PoW), 3 (honeypot), 4 (autofill-safe)
- `frontend/landing/HUONG_DAN_LANDING_FORM_TRACKING.md`
