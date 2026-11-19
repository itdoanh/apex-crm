# Frontend — CRM Admin & Ladi Studio

**Không có build step.** Toàn bộ frontend là HTML/CSS/JS thuần, load Tailwind + Alpine + Chart.js từ CDN. Sửa file → refresh trình duyệt là thấy.

---

## 1. Cấu trúc

```
frontend/
├── admin/
│   ├── index.html          Master SPA — chứa TẤT CẢ panel
│   ├── admin.html          Alias của index.html
│   ├── studio.html         Ladi Studio — visual drag & drop builder
│   ├── reports.html        Trung Tâm Báo Cáo — 6 tab, 12 biểu đồ, 5 bảng
│   ├── config.js           API base URL, target server IP
│   ├── ladihtml.sdk.min.js SDK LadiPage (runtime cho Studio)
│   ├── css/
│   │   ├── admin.css       Dark glassmorphism tài chính
│   │   ├── base.css        Reset + biến CSS chung
│   │   ├── crm.css         Bảng, Kanban, drawer
│   │   ├── layout.css      Grid, sidebar, header
│   │   ├── reports.css     Trung Tâm Báo Cáo (thêm 2026-11-13)
│   │   └── studio.css      Canvas, gizmo, layers, inspector
│   └── js/
│       ├── app.js          Điều phối chính, tab switching
│       ├── auth.js         Login, session, đổi mật khẩu
│       ├── crm.js          Kanban 7 bước, Customer 360° drawer, Journey timeline
│       ├── analytics.js    Chart.js — heatmap, funnel, geo, ad placement
│       ├── reports.js      Trung Tâm Báo Cáo (thêm 2026-11-13)
│       ├── sheets.js       Google Sheets config + mapping matrix
│       ├── domains.js      DNS, domain, deploy route
│       ├── landings.js     CRUD landing, feature flags
│       ├── security.js     Spam quarantine, security stats
│       ├── tracker.js      Bản mirror (dùng cho preview landing)
│       └── studio/
│           ├── gizmo.js            Khung 8 chốt kéo giãn
│           ├── layers.js           Cây layer Z-Index
│           ├── history.js          Undo/Redo 50 bước
│           ├── context-menu.js     Menu chuột phải 20 hành động
│           ├── inspector.js        Panel thuộc tính 6 tab
│           ├── ast-mapper.js       Ánh xạ DOM ↔ AST
│           └── file-manager.js     Cây file landing
├── landing/
│   ├── index.html         Landing entry (alias của ladi.html)
│   ├── ladi.html          Landing chính (LadiPage format)
│   ├── chucmung.html      Trang cảm ơn
│   ├── tracker.js         Thu thập data + PoW solver
│   ├── config.js          Config client cho landing
│   ├── config.json        Config per-landing
│   ├── ladihtml.sdk.min.js
│   └── assets/            Ảnh (mỗi ảnh có .png + .webp)
```

> 🚨 **`backend/static/` là MIRROR của `frontend/admin/`.**
> Sửa UI phải sửa ở `frontend/admin/` rồi copy sang `backend/static/`.
> Không sửa trực tiếp trong `backend/static/` — sẽ bị mất khi đồng bộ.

---

## 2. `frontend/admin/index.html` — Master SPA

Một file HTML duy nhất chứa toàn bộ panel. Các panel được bật/tắt bằng Alpine.js `x-show`.

### Panel chính

| Panel | Nội dung |
|---|---|
| Quản Trị CRM | Bảng dữ liệu + Kanban 10 cột + Customer 360° drawer |
| Facebook Ads | Meta Conversions API: cấu hình, test event, Dataset Quality (EMQ), log gửi, breakdown chiến dịch, parse URL QC |
| Biểu Đồ & Giờ Vàng | Heatmap khung giờ, funnel, geo, ad placement, traffic source |
| Hiệu Quả Landing | So sánh CR% giữa các landing || Bảo Mật & Bot | Spam quarantine + security stats |
| Đồng Bộ Google Sheets | Config, mapping matrix, sync logs |

#### Panel "Đồng Bộ Google Sheets" — mapping matrix (2026-11-15)

> ⭐ **Danh mục trường được NẠP TỪ BACKEND**, không hardcode trong JS.
> `loadSheetFieldCatalog()` gọi `GET /api/integrations/sheets/defaults`
> ⇒ trả `available_fields` (~90 trường, có `group`) + `default_mappings`
> (bộ cột chuẩn). Fallback tối thiểu nếu API lỗi.

| Hàm | Vai trò |
|---|---|
| `loadSheetFieldCatalog()` | Nạp danh mục + bộ cột chuẩn (gọi khi mở trang & mở modal) |
| `buildFieldOptionsHtml(selectedKey)` | Dựng `<optgroup>` theo `group` — dễ tìm trong ~90 trường |
| `getDefaultStandardMappings()` | Bộ cột chuẩn (từ backend; fallback dựng từ danh mục) |
| `guessFieldFromHeader(header)` | Đoán field khi bấm "Đọc Tiêu Đề Từ Sheet" — đã nhận diện nhóm mới |
| `updateMappingField(index, key, value)` | Đổi field ⇒ tự cập nhật tiêu đề cột nếu tiêu đề còn là mặc định |
| `upgradeSheetMappings(id, name)` | Bấm "Bổ sung Cột" ⇒ `POST /api/integrations/sheets/configs/upgrade-mappings` |

> ⚠️ **Cột chuẩn mới không tự áp dụng cho config đã lưu** (tránh phá dữ liệu
> đang ghi). Admin bấm **"Bổ sung Cột"** trên từng config ⇒ cột mới thêm vào
> CUỐI, tiêu đề được ghi vào dòng 1 trên Sheet.

> Trong modal "Thêm kết nối Sheet" → phần **3. Kiểm Tra & Gửi Dòng Thử Nghiệm (Test Row)**:
> nút "Gửi Dòng Thử Nghiệm Ngay" gọi `POST /api/integrations/sheets/test`. Dòng ghi ra luôn có tên
> dạng `Test <3 số ngẫu nhiên>` (vd: `Test 734`) với dữ liệu mô phỏng người dùng thật (SĐT VN,
> nhà mạng, IP, tỉnh/thành, ISP, thiết bị, trình duyệt, UTM, URL landing...). UI hiển thị mã dòng
> để anh dễ tìm & xoá sau khi kiểm tra. Xem `docs/API.md` mục 9.1.
| Quản Lý Landing | CRUD + Code Studio + Live Preview |
| Quản Lý Tên Miền | DNS check, SSL check, deploy route |
| Nhật Ký & Phiên | Active sessions + audit logs |

#### Badge "TEST" cho lead test (thêm 2026-11-16)

Lead mà **tên chứa chuỗi `test`** do admin tạo để kiểm tra hệ thống ⇒ backend tự
đánh dấu `is_test = TRUE`. Trong bảng CRM (desktop) và thẻ mobile, `crm.js` render badge
màu tím **"TEST"** + giảm opacity để admin không nhầm là lead thật.

> Lead test **VẪN hiện** trong CRM và **VẪN** đồng bộ Google Sheets, nhưng **KHÔNG**
> tính vào biểu đồ / thống kê / CR% và **KHÔNG** bắn Meta CAPI.
> Chi tiết: `docs/SECURITY.md` mục 6.2 · `docs/DATABASE.md` mục 2.

### Panel chuyển động
- Sidebar trái — icon + label, active state
- Header — tên user, nút đổi mật khẩu, đăng xuất
- Auto-refresh: dropdown `10s / 30s / 60s / Tắt` + nút refresh thủ công
- Filter bar: search, landing, nguồn, trạng thái, nhà mạng, ngày, khung giờ

### Cấu hình API
`frontend/admin/config.js`:
```javascript
window.APEX_ADMIN_CONFIG = {
    API_BASE_URL: '',        // rỗng = same-origin ( qua Caddy )
    BASE_PATH: '/ads',
    TARGET_SERVER_IP: '103.124.95.232'
};
```

### 📊 Báo cáo riêng từng Landing Page (thêm 2026-11-12)

Trong tab **Analytics** (`sectionTabAnalytics`) có panel **"BÁO CÁO RIÊNG TỪNG LANDING PAGE"**:

**Điều khiển:**
| Element | ID | Ghi chú |
|---|---|---|
| Dropdown chọn landing | `#reportLandingSelect` | nạp từ danh sách landing qua `populateReportLandingOptions()` |
| Nút granularity | `.report-granularity-btn` | `hour` / `day` / `week` → `data-granularity` |

**KPI (12 ô):**
`#lrTotalViews`, `#lrUniqueVisitors`, `#lrHumanVisitors`, `#lrBotVisitors`, `#lrTotalLeads`,
`#lrConversionRate`, `#lrAvgDuration`, `#lrAvgScroll`, `#lrVpnVisitors`, `#lrVpnRate`,
`#lrInAppVisitors`, `#lrMetaClicks`

**Biểu đồ (Chart.js):**
| ID | Nội dung |
|---|---|
| `#chartReportTimeseries` | views / human / bot / leads theo bucket |
| `#chartReportDevices` | top thiết bị (tên thật) |
| `#chartReportInApp` | in-app browser vs trình duyệt ngoài |
| `#chartReportBrowser` | trình duyệt + version |
| `#chartReportBots` | lý do bị flag bot |
| `#chartReportVPN` | nhà cung cấp VPN/proxy |

**Bảng:**
| ID | Nội dung |
|---|---|
| `#lrMacroTableBody` | Meta macros: campaign / adset / ad / placement + CR% |
| `#lrDeviceTableBody` | model thật (`Samsung Galaxy A52`), brand, code, views, CR%, duration, scroll |

**JS:** `frontend/admin/js/analytics.js` mục 14 — `onReportLandingChange`, `setReportGranularity`,
`buildReportParams`, `populateReportLandingOptions`, `loadLandingReport`.
`frontend/admin/js/app.js` gọi `loadLandingReport(true)` khi vào tab analytics;
`frontend/admin/js/crm.js` gọi `populateReportLandingOptions(landingIDs)` sau khi nạp danh sách.

> ⚠️ `renderFunnelChart()` nhận thêm tham số `funnel` — số liệu **thật** từ
> `analytics_sessions` (views / scrolled / interacted / converted), không còn là `totalLeads * 8`.

> ⚠️ Filter "Buổi Đêm" (22h→06h) dùng `from_hour=22&to_hour=6` — backend hỗ trợ
> **quay vòng qua nửa đêm** (`fromHour > toHour`).

---

## 2.1. Trung Tâm Báo Cáo — `frontend/admin/reports.html` (thiết kế lại 2026-11-13)

Trang báo cáo **độc lập**, phục vụ tại `/ads/reports.html`. Link vào từ header CRM
(`frontend/admin/index.html` → nút **"Trung Tâm Báo Cáo"**).

```
frontend/admin/
├── reports.html          Trang báo cáo 6 tab + sticky filter bar
├── css/reports.css       Style riêng: .rp-select, .rp-tab, .rp-btn, dropdown multi-landing
└── js/reports.js         Toàn bộ logic: filter state, 12 biểu đồ, 5 bảng, quarantine
```

> 🚨 Cần copy sang `backend/static/`:
> `reports.html` → `backend/static/reports.html`,
> `js/reports.js` → `backend/static/js/reports.js`,
> `css/reports.css` → `backend/static/css/reports.css`.

### 6 tab

| Tab | Nội dung | Biểu đồ | Bảng |
|---|---|---|---|
| 1. Tổng quan & Đa Landing | KPI + so sánh kỳ trước | 1 (combo bar+line) | 1 — leaderboard |
| 2. Meta Ads & Attribution | Phân bổ nền tảng/placement, CR in-app, heatmap 7×24 | 2, 3, 4 | 2 — campaign→adset→ad |
| 3. Hành Vi & Nội Dung | Phễu scroll depth, thời gian nhìn thấy, tương tác media | 5, 6, 7 | 3 — top điểm chạm |
| 4. Form & Phễu Chuyển Đổi | Phễu form 5 bước, KPI bỏ ngang, nguồn autofill | 8, 9 | — |
| 5. Thiết Bị & Chẩn Đoán | Phân bổ device/OS, chất lượng mạng | 10, 11 | 4 — model & viewport |
| 6. Bảo Mật & Chống Bot | KPI bot, lý do chặn, nhật ký cách ly | 12 | 5 — quarantine log |

### Sticky filter bar (dùng chung cho cả 6 tab)

| Nhóm | Element | ID |
|---|---|---|
| Thời gian | 7 preset: Hôm nay · Hôm qua · 7 ngày · 30 ngày · Tháng này · 1 năm · Tuỳ chọn | `.rp-btn[data-preset]` |
| Thời gian | Từ / Đến (date input) | `#rpFromDate` `#rpToDate` |
| Thời gian | Từ giờ / Đến giờ (0–23) | `#rpFromHour` `#rpToHour` |
| So sánh | Checkbox "So với kỳ trước" | `#rpCompare` |
| Landing | Multi-select (checkbox list) | `#rpLandingMulti` |
| Landing | Nguồn / Landing path | `#rpNguon` `#rpLandingPath` |
| Meta Ads | Account / Campaign / Adset / Ad / Platform / Placement | `#rpAdAccount` `#rpCampaign` `#rpAdset` `#rpAdName` `#rpSiteSource` `#rpPlacement` |
| Khách | Loại khách: Mới / Quay lại / Tất cả | `#rpVisitorType` |
| Thiết bị | Loại thiết bị / OS / In-app | `#rpDeviceType` `#rpOS` `#rpInApp` `#rpInAppType` |
| Chất lượng | human / all / bot | `#rpTrafficScope` |
| Live | **Tự động tải lại**: Tắt / 10s / 30s / 1' / 5' / 10' | `#rpAutoRefresh` `#rpAutoRefreshStatus` |

### Tự động tải lại dữ liệu mới nhất (Live) — thêm 2026-11-13

Dropdown `#rpAutoRefresh` cho phép admin chọn chu kỳ tự động nạp lại dữ liệu mới
nhất trong Trung Tâm Báo Cáo.

| Giá trị | Chu kỳ |
|---|---|
| `0` | Tắt (mặc định) |
| `10000` | 10 giây |
| `30000` | 30 giây |
| `60000` | 1 phút |
| `300000` | 5 phút |
| `600000` | 10 phút |

| Hàm | Việc |
|---|---|
| `setAutoRefresh(value)` | Đọc lựa chọn, lưu `localStorage` (`apex_rp_auto_refresh`), khởi động timer, nạp lại ngay |
| `rpAutoRefreshStart()` | Tạo `setInterval` theo chu kỳ (KHÔNG dùng `setTimeout` lặp để tránh tràn timer) |
| `rpAutoRefreshPause(paused)` | Tạm dừng / tiếp tục timer |
| `initAutoRefresh()` | Khôi phục lựa chọn đã lưu, gắn sự kiện tạm dừng |
| `refreshActiveTab(silent)` | Nạp lại tab đang active; `silent=true` ⇒ không nháy toast lỗi, không cuộn trang |

**Hành vi thông minh (tránh làm phiền admin):**

- Tab trình duyệt bị ẩn ⇒ timer bị **dừng hẳn** (`visibilitychange`) ⇒ không hót
  băng server khi admin rời bàn phím.
- Admin đang chọn bộ lọc ngày/giờ ⇒ timer tạm dừng, `blur`/`change` thì chạy lại.
- Lần gọi tự động dùng `silent=true` ⇒ không nháy toast lỗi đỏ, không reset scroll.
- Lựa chọn được lưu `localStorage` ⇒ giữ nguyên sau F5.
- Giá trị lưu được kiểm tra lại (`allowed = [0,10000,30000,60000,300000,600000]`)
  ⇒ `localStorage` bị sửa tay không gây lỗi.


### Hành trình khách hàng (Customer Journey Timeline)

Trong drawer **Customer 360°** của CRM (`frontend/admin/js/crm.js`):

| Hàm | Việc |
|---|---|
| `loadCustomerJourney(phone)` | Gọi `GET /api/analytics/report/journey?phone=…` |
| `renderJourneyTimeline(data)` | Vẽ hồ sơ + timeline từng giây |

> ⭐ **Doanh thu + điểm lead nhập tay** (thêm 2026-11-19 — fix Events Manager
> "thiếu thông số giá trị và đơn vị tiền tệ" 100% sự kiện Lead):
> Drawer Customer 360° có 2 ô: `Doanh thu (VND)` (`#drawerConversionValue`) +
> `Điểm Lead (thang Meta 0–10)` (`#drawerLeadScore`). `openLeadDrawer()` nạp giá trị
> đã lưu; `saveLeadFromDrawer()` gửi `value` + `lead_score` trong `POST /api/leads/update`
> ⇒ CAPI kèm `currency: "VND"` + `value` + `lead_score`, Sheets + CSV cùng cập nhật.

Container trong `frontend/admin/index.html`: `#customerJourneyPanel`.

### Bắt buộc — an toàn render

- Mọi giá trị từ server phải qua `escapeHTML()` trước khi chèn vào HTML.
- **Không** dùng template literal động cho class Tailwind (vd `text-${tone}-400`) vì
  Tailwind Play CDN không JIT được class sinh động → dùng bảng tra static
  (`RP_ICON_TONE` trong `reports.js`).
- `toggleAdsetRows()` dùng **key sinh từ index** (`c0`, `c1`…), không dùng tên
  campaign ⇒ tránh lỗi escape trong `querySelector`.

---

## 3. Ladi Studio (`frontend/admin/studio.html`)

Visual drag & drop builder cho landing page.

### Kiến trúc

```
studio.html
  ├── iframe preview  ← render landing HTML thật
  ├── Canvas controls (zoom, pan, hand mode)
  ├── Layers panel   ← cây layer, Z-Index, ẩn/khoá
  ├── Inspector      ← 6 tab thuộc tính
  ├── Content tab    ← sửa nhanh text/ảnh/CTA
  ├── CSS tab        ← CSS 2 chiều + effect library
  └── Code tab       ← editor HTML/JS/CSS + search-replace
```

### Tính năng chính

| Tính năng | Chi tiết |
|---|---|
| Gizmo 8 chốt | Kéo giãn chốt từ 4 góc + 4 cạnh, khóa tỷ lệ khung hình |
| History 50 bước | Undo `Ctrl+Z` / Redo `Ctrl+Y`, dropdown lịch sử |
| Layers | 7 nhóm: Text, Image, Button, Form, Section, Box, Icon |
| Context menu | 20 hành động, xuyên thấu iframe, chống tràn mép màn hình |
| Hand mode | Nhấn giữ `Space` để pan canvas |
| AST mapper | Đồng bộ DOM ↔ code, giữ `data-ve-id` xuyên suốt |
| File Manager | Cây thư mục + tạo/đổi tên/xoá file & thư mục |
| Upload từ máy | Nút "Tải lên" trong File Explorer → `POST /api/landings/files/upload` |

### 🖥️ Viewport PC — FLUID (sửa 2026-11-05)

**Vấn đề đã gặp**: chế độ PC của Code Studio hardcode `width: 1440px` cho khung preview
⇒ co cửa sổ trình duyệt thì preview **vẫn cứng ở 1440px**, phải cuộn ngang, và
**không khớp với web PC thật** (web thật co giãn theo viewport).

**Cách sửa** — PC mode dùng chiều rộng **fluid** (`width: 100%`):

| Thành phần | Hành vi mới |
|---|---|
| `csIframeWrapper` (PC) | `width: 100%` + `min-width: 0px` ⇒ co/nở theo khung nhìn |
| `min-width: 0px` | Không ép khung rộng tối thiểu ⇒ **không tràn khung** khi panel split hẹp (fix 2026-11-15) |
| Mobile mode | Giữ nguyên khung cố định `390 × 844` (iPhone 14/15/16 Pro) |
| Readout toolbar | `#csCanvasWidthLive` hiển thị chiều rộng thực tế của khung preview |
| Nút "Tự động" | `#btnCanvasAutoWidth` — chỉ hiện khi đã ghim width thủ công; bấm để trở về fluid |
| Kéo 2 chốt mép | Vẫn dùng được → **ghim** width cố định (`studioCanvasManualWidth`), PC mode ngừng fluid |

> ⚠️ **Đã đổi (fix 2026-11-15)**: trước đây PC dùng `min-width: 780px` để giữ
> media query `@media (min-width: 769px)` luôn bật. Hậu quả: khung preview **tràn**
> khi panel split hẹp (~566px) ⇒ "khung hình không chuẩn". Nay bỏ clamp, PC styles
> được giữ bật bằng cơ chế **ép PC trong preview** (xem mục zoom bên dưới).

Hàm quản lý (trong `js/studio/gizmo.js`):

| Hàm | Vai trò |
|---|---|
| `applyStudioCanvasWidth()` | Áp dụng width theo device mode (PC fluid / Mobile cố định) |
| `computeCanvasViewportWidth(boxW, zoom, isMobile)` | Tính viewport **nội bộ** của iframe (PC = `boxW / zoom`, Mobile = 390px) |
| `updateStudioCanvasWidthReadout()` | Cập nhật readout + hiện/ẩn nút "Tự động" |
| `resetStudioCanvasAutoWidth()` | Bỏ ghim width → PC mode trở lại fluid |

`ResizeObserver` trên `#csCanvasViewport` + `#csIframeWrapper` cập nhật readout + gizmo khi
người dùng co / giãn cửa sổ hoặc kéo thanh split.

> ⚠️ **Lưu ý khi sửa `studio.html`**: nút PC hiện ghi "PC (Tự động)" (không còn "PC (1440px)").

### 🔍 Zoom preview — MÔ PHỎNG ZOOM THẬT CỦA TRÌNH DUYỆT (sửa 2026-11-05)

**Vấn đề đã gặp**: zoom của Studio dùng `document.body.style.zoom` **bên trong** iframe.
Hệ quả:

- Viewport **nội bộ** của iframe **không đổi** ⇒ **media query không đổi** ⇒ layout PC
  vẫn cứng ở bề rộng cũ.
- Chỉ chữ / toán hình bị thu nhỏ ⇒ *"zoom nhỏ ra nhưng khung hình web không rộng ra"*,
  khác hẳn web PC thật (web thật zoom nhỏ ⇒ bề rộng viewport tăng ⇒ layout co giãn).

**Cách sửa** — mô phỏng đúng cơ chế zoom của trình duyệt:

```
iframe.style.width  = viewportW   (viewport NỘI BỘ, nơi media query áp dụng)
iframe.style.height = boxH / scale
iframe.style.transform = scale(scale)   (transform-origin: top left)
```

| Device mode | `viewportW` (nội bộ) | `scale` | Ý nghĩa |
|---|---|---|---|
| PC | `clamp(boxW / zoom, 320, 3840)` | `boxW / viewportW` | Zoom nhỏ ⇒ viewport nội bộ **RỘNG RA** ⇒ layout co giãn thật |
| Mobile | `390` (cố định) | `zoom` | Giữ đúng viewport thiết bị; zoom chỉ thu phóng hình ảnh |

- Lệnh mới `APEX_CMD_RESET_CONTENT_ZOOM` (trong `js/studio/ast-mapper.js`) xoá
  `body.zoom` cũ để không nhân đôi tỉ lệ.

> ⚠️ **Đã đổi (fix 2026-11-15)**: trước đây PC dùng
> `viewportW = Math.max(boxW / zoom, 780)`. Hậu quả:
> - Panel split hẹp (~566px) ⇒ wrapper ép `min-width: 780px` ⇒ **khung tràn**, bị cắt cụt.
> - `Math.max(…, 780)` **clamp** viewport ⇒ đổi zoom **không** làm viewport đổi ⇒
>   media query không đổi ⇒ **bố cục web không co giãn** ⇒ zoom "vô hiệu".
>
> Nay: `computeCanvasViewportWidth()` trong `gizmo.js` — PC = `boxW / zoom`
> (clamp 320–3840), Mobile = 390px cố định.

#### Ép PC styles trong preview (fix 2026-11-15)

Vì viewport nội bộ có thể **< 769px** khi zoom vào, media query
`@media (min-width: 769px)` (PC styles của Studio) sẽ tắt ⇒ các style user đã chỉnh
trong Inspector (PC scope) biến mất.

| Thành phần | Vai trò |
|---|---|
| `ast-mapper.buildPreviewPCForceCSS()` | Sinh CSS chứa PC rules **không bọc** media query, selector `html[data-apex-preview-device="pc"] [data-ve-id="…"]` |
| `ast-mapper.getHelperScript()` | Inject style block `#__apex_preview_pc_force_styles__` vào preview |
| `syncPreviewDeviceToIframe()` (`file-manager.js`) | Gửi `APEX_CMD_SET_PREVIEW_DEVICE` (device + pcForceCss) xuống iframe |
| Handler `APEX_CMD_SET_PREVIEW_DEVICE` (`ast-mapper.js`) | Set `data-apex-preview-device` trên `<html>` + cập nhật style block |

> 🚨 **Scope GLOBAL bắt buộc (fix 2026-11-15)**: `syncPreviewDeviceToIframe()` phải
> được khai báo ở **scope global** (cùng cấp với `setStudioDevice`), **KHÔNG** khai
> báo bên trong `setStudioDevice()`. Nếu khai báo bên trong ⇒ hàm không có scope global
> ⇒ `inspector.js` (`applyAstStyleChange`) và `refreshStudioPreview()` đều gọi
> `typeof syncPreviewDeviceToIframe === 'function'` ⇒ luôn `false` ⇒
> `data-apex-preview-device` **không bao giờ được set** ⇒ CSS ép PC không áp dụng.
>
> Bổ sung: style block `#__apex_preview_pc_force_styles__` **luôn được tạo** (kể cả
> rỗng) và handler `APEX_CMD_SET_PREVIEW_DEVICE` **tự tạo** element nếu thiếu.

> 🚨 **Ràng buộc quan trọng**: style block ép PC này **CHỈ tồn tại trong preview
> iframe**, **không bao giờ** được ghi vào `code_html` / `landing_files` ⇒ production
> vẫn dùng media query thật ⇒ không ảnh hưởng trang live.

**Quy đổi tọa độ** (vì iframe nay bị `transform: scale()` ⇒ tọa độ trong iframe là CSS px
**nội bộ**, chưa scale):

| Nơi | Cách quy đổi |
|---|---|
| `repositionGizmoBox()` (`inspector.js`) | `rect.* × window.canvasRenderScale` |
| Hover box (`inspector.js`) | `msg.rect.* × window.canvasRenderScale` |
| Context menu (`inspector.js`) | `msg.clientX/Y × window.canvasRenderScale` |
| Pan từ trong iframe (`gizmo.js`) | `data.clientX/Y × window.canvasRenderScale` |
| Gizmo drag / resize (`gizmo.js`) | `screenDx / scale` ⇒ CSS px nội bộ |

Biến `window.canvasRenderScale` được export từ `gizmo.js` (tỉ lệ scale **thực tế**,
khác `canvasZoom` khi bị clamp bởi giới hạn 320–3840px).

Readout `#csCanvasWidthLive` hiển thị **viewport nội bộ** (nơi media query áp dụng),
nên khi zoom 50% sẽ hiện `1556px` — khớp chính xác bề rộng mà landing thật sự nhận.

> 📌 **Cache tĩnh**: `main.go` đã set `Cache-Control` cho file tĩnh (`setStaticCacheHeaders`).
> Nếu thấy UI cũ sau khi deploy → hard reload (`Ctrl+Shift+R`) hoặc kiểm tra lại header.

### 📁 File Manager — lưu & deploy TOÀN BỘ file (từ 2026-11-04)

**Vấn đề đã gặp**: trước đây `saveStudioCode()` chỉ POST **1 file đang mở** và
`deployCodeLive()` chỉ đồng bộ `index.html` ⇒ file ảnh/video upload trong thư mục
bị "mất" khi deploy lên domain.

**Cách sửa**:

| Hành động | Endpoint | Gửi gì |
|---|---|---|
| **Lưu Code** (`Ctrl+S`) | `POST /api/landings/files/save` | **Toàn bộ** file văn bản trong cây |
| **Deploy** (`Ctrl+Enter`) | `POST /api/landings/files/save` + `POST /api/landings/deploy` | **Toàn bộ** file văn bản (payload `files`) |

Hàm helper quan trọng trong `js/studio/file-manager.js`:

| Hàm | Vai trò |
|---|---|
| `collectStudioFilesPayload()` | Gom `studioFileContents` thành mảng `{file_path, file_type, content}` — **chỉ file văn bản** |
| `isBinaryStudioPath(path)` | Loại file nhị phân (ảnh/video/audio/font/pdf) khỏi payload |
| `reloadStudioFilesAfterMutation()` | Refresh cây file từ server sau upload/xoá (giữ nội dung đang sửa dở) |
| `openStudioUploadModal(folder)` | Mở modal upload, chọn thư mục đích |
| `submitStudioUpload()` | `POST /api/landings/files/upload` (multipart, nhiều file) |

> 💡 **Vì sao không gửi file nhị phân trong JSON?** File upload lên tới 100MB và được lưu
> dạng base64 trong DB. Gửi lại qua JSON mỗi lần lưu sẽ làm request nặng và chậm.
> Backend tự đồng bộ file nhị phân sang domain đích qua `SyncLandingFilesToDomain()`.

> ⚠️ **Thứ tự deploy trong backend** (`HandleDeployLanding`):
> 1. `SyncLandingFilesToDomain("", targetDomain, page_id)` — copy file có sẵn trong DB.
> 2. Ghi đè bằng `payload.Files` — nội dung mới nhất.
>
> Đảo thứ tự sẽ khiến file upload bị bản gốc cũ đè mất.

### Keyboard shortcuts

| Phím | Hành động |
|---|---|
| `Ctrl+Z` | Undo |
| `Ctrl+Y` | Redo |
| `Ctrl+S` | Lưu code + toàn bộ file (`POST /api/landings/files/save`) |
| `Space` (giữ) | Hand mode / pan |
| `Delete` | Xoá element đang chọn |
| `Esc` | Bỏ chọn |

### Bảo toàn `data-ve-id`

`data-ve-id` là khoá định danh element giữa DOM và AST. **Phải duy trì xuyên suốt** — mất nó thì inspector/layers/code sync mất liên kết.

---

## 4. Landing page (`frontend/landing/`)

### `config.json` — config per-landing

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

### Binding trong HTML
```html
<span data-config="thoi_gian_chia_se.hien_thi">20:00 ngày 03/11/2026</span>
<span data-config="thoi_gian_chia_se.ngay">03/11/2026</span>
<span data-config="khoa_hoc.da_dang_ky">Đã có 79 suất đăng ký</span>
```
`tracker.js` đọc `config.json` → tự động điền vào mọi element có `data-config`.

### Countdown
Đếm ngược tới `target_date` (ISO 8601).

---

## 5. `tracker.js` — thu thập dữ liệu

### Trách nhiệm
1. Đọc `config.json` → gắn `landing_id` + `nguon`.
2. Gửi `POST /api/analytics/track` (heartbeat) — thu thập scroll, click, mouse.
3. `GET /api/auth/challenge` → giải PoW.
4. Submit form → `POST /api/leads` kèm tracking + PoW solution.
5. Set cookie 180 ngày → redirect `chucmung.html` nếu đã đăng ký.
6. **Chặn SĐT rác TRƯỚC khi gửi** (`blockJunkPhone()`) — lưu cookie
   `apex_junk_blocked_<landing>` (30 ngày) + chuyển `chucmung.html` ⇒ KHÔNG
   POST `/api/leads`, KHÔNG bắn `fbq('track','Lead')` ⇒ **không tốn tiền QC**.

### Data thu thập

| Nhóm | Fields |
|---|---|
| Form | `fullName`, `phone`, `email`, `note`, `package` |
| Hành vi | `time_on_page`, `max_scroll`, `clicks_count`, `fill_duration_ms` |
| Autofill | `is_autofill`, `autofill_source` |
| UTM | `utm_source`, `utm_medium`, `utm_campaign`, `utm_content`, `utm_term` |
| URL | `url` (URL đầy đủ trang đăng ký kèm query string), `entry_url`, `referrer` |
| Fingerprint | `visitor_id`, `session_id`, `device_type`, `browser`, `os`, `screen_res` |
| PoW | `challenge_id`, `challenge_salt`, `challenge_difficulty`, `challenge_signature`, `challenge_solution` |

> ⭐ **`url` là trường quan trọng** — server lưu vào cột `leads.url` và đồng bộ Google Sheets
> bằng field mapping `url`. Xem `docs/API.md` mục 3 & `docs/DATABASE.md` mục 2.

### Bất biến
- **Không** được gỡ field honeypot (`website`, `fax_company_prefix`, `user_email_verify`, `company_website_url`) khỏi DOM.
- **Không** được phá autofill-safe — xem `docs/SECURITY.md` mục 4.
- **Không** được thêm logic sửa DOM từ cấu hình CSDL (hotline, zalo, tiêu đề hero, metric box...).
  HTML viết tay trong Code Studio là **nguồn sự thật duy nhất**. Xem `docs/LANDING.md` mục 5.
- **Mọi** module mới có side-effect lên DOM phải guard bằng `if (isAnyPreview()) return;`
  để Code Studio preview không bị nhiễu.
- **MỘT lần bấm = MỘT `POST /api/leads`.** `sendLeadToBackend()` có khoá chống gửi trùng
  theo SĐT (`LEAD_DEDUP_WINDOW_MS = 30000`). Không được gọi `sendLeadToBackend()` từ
  nhiều handler submit cùng lúc — nếu không, request thứ 2+ dùng lại nonce PoW đã tiêu thụ
  ⇒ server ghi vào `spam_quarantine` ⇒ **lead thật bị gắn nhầm nhãn "Lỗi Proof-of-Work"**.
  Chi tiết: `docs/LANDING.md` mục 3 & `docs/SECURITY.md` mục 2.
- **Chặn SĐT rác phải nằm TRƯỚC chỗ disable nút / gọi `fetch`** trong
  `sendLeadToBackend()`. Nếu đặt sau ⇒ request đã gửi ⇒ đã tốn tiền QC.
  Server có lớp phòng thủ thứ 2 (`security.IsJunkPhone`) nhưng lúc đó đã tốn 1
  request. Chi tiết: `docs/SECURITY.md` mục 6.1.

### Code Studio preview — cách ly

`tracker.js` có 3 helper để nhận biết đang chạy trong preview/Studio:

| Helper | Ý nghĩa |
|---|---|
| `isStudioBuilder()` | `window.__APEX_STUDIO__` / `__APEX_BUILDER_ACTIVE__` / `__APEX_FREEZE_MODE__` |
| `isAnyPreview()` | Studio builder **hoặc** `__APEX_PREVIEW__` / `IS_PREVIEW` **hoặc** đang nằm trong iframe |

Khi preview:
- Không nạp `config.json` từ network.
- Không chạy `applyConfigToDOM()` (chỉ set `document.title` + Pixel ngoài preview).
- Không chặn form (`initAutoFormInterceptor` return sớm).
- Không gửi analytics (`sendAnalyticsPing` return sớm).

Thêm vào `ast-mapper.getHelperScript()` (inject vào preview iframe):
`__APEX_STUDIO_LANDING_ID__`, `__APEX_STUDIO_DOMAIN__`, `__APEX_STUDIO_LANDING_NAME__` —
dùng để ghi đè `window.APEX_LANDING_CONFIG` của template gốc.

> ⚠️ **Bẫy ghi đè config (đã fix 2026-11-05, commit `897f57a`):**
> Helper script của Studio được inject ở **đầu** `<head>`, nhưng script trong HTML
> template thường gán `window.APEX_LANDING_CONFIG = { landing_id: 'mxv_080_online', ... }`
> ở **phía sau** → ghi đè mất giá trị đã ép ⇒ preview hiển thị sai landing.
>
> Cách sửa: dùng `Object.defineProperty(window, 'APEX_LANDING_CONFIG', { get, set })`
> với setter mỗi lần HTML gán lại config thì tự ép lại `landing_id` / `domain` / `name`
> của landing đang mở trong Studio. **Không được** quay lại cách gán thẳng
> `window.APEX_LANDING_CONFIG = Object.assign(...)` vì sẽ tái diễn lỗi trên.

### 🔎 `<base href>` của preview iframe (fix 2026-11-15)

Helper script của Studio chứa thẻ `<base href="…">` quyết định cách trình duyệt
phân giải **URL tương đối** trong HTML landing (`assets/css/style.css`, `tracker.js`,
`assets/img/…`).

| Giá trị | Kết quả |
|---|---|
| `/` | `assets/css/style.css` ⇒ `https://crm.hanghoaphaisinh.net/assets/...` ⇒ **404** (assets nằm trong `landing_files` theo `page_id`) |
| `/p/<page_id>/` | ⇒ route `ServePublicLanding` (`backend/main.go`) ⇒ đọc `landing_files` theo `page_id` ⇒ **200 OK** ✅ |

| Hàm | Vai trò |
|---|---|
| `ast-mapper.getPreviewBaseHref()` | Trả `/p/<page_id>/` (encodeURIComponent), fallback `/` nếu chưa có page_id |

> 🚨 **BẮT BUỘC**: khi chèn `<base>` + helper script vào `<head>`, **phải** tìm thẻ
> `<head>` thật (bỏ qua HTML comment) — dùng `findRealTagInsertIndex(html, 'head')`.
> Tìm bằng `indexOf('<head>')` sẽ khớp nhầm text `<head>` trong comment ⇒ `<base>`
> bị chèn trong comment ⇒ **vô hiệu** ⇒ mất giao diện. Chi tiết: `docs/LANDING.md`.

### Upload file trong File Manager

| Bước | API | Ghi chú |
|---|---|---|
| Chọn thư mục đích | — | Dropdown `studioUploadFolder` được populate từ `populateFolderDropdowns()` |
| Upload | `POST /api/landings/files/upload` | FormData: `page_id`, `domain`, `folder`, `files[]` (multi-file) |
| Lưu / Deploy | `POST /api/landings/files/save` | Batch toàn bộ cây file |
| Deploy lên domain | `POST /api/landings/deploy` | Kèm `files[]` → đồng bộ asset nhị phân lên domain đích |

> ⚠️ `populateFolderDropdowns()` phải populate **cả** `studioUploadFolder`.
> Bỏ sót → dropdown thư mục đích chỉ có "(Thư mục gốc /)" ⇒ không upload được
> vào thư mục con. Xem `docs/API.md` mục upload + `docs/LANDING.md` mục 6.

---

## 6. Quy trình sửa UI (quan trọng)

```
1. Sửa file trong frontend/admin/  (nguồn chuẩn)
2. Copy sang backend/static/        (mirror)
   Copy-Item frontend\admin\js\crm.js backend\static\js\crm.js
3. Nếu đang chạy local: refresh trình duyệt (không cần build)
4. Nếu production: commit + push + git pull + restart task
```

### Kiểm tra mirror có đồng bộ
```powershell
git diff --stat frontend/admin backend/static
```

> ⚠️ `copy` bằng PowerShell `Copy-Item` giữ nguyên byte → OK. Nhưng nếu dùng `Set-Content`
> / redirect (`>`), file sẽ bị đổi line-ending CRLF → LF ⇒ **nội dung khác nhau** dù logic
> giống. Luôn dùng `cmd /c copy /Y` cho file mirror.

---

## 6.5. Tab "Facebook Ads" — Meta Conversions API (thêm 2026-11-15)

| File | Vai trò |
|---|---|
| `frontend/admin/js/facebook-ads.js` | Controller của tab Facebook Ads (nguồn chuẩn) |
| `frontend/admin/js/lead-status.js` | Loader danh mục 10 trạng thái CRM (dùng chung CRM + Analytics) |
| `backend/integrations/facebook/*` | Backend: hashing, payload, Graph API client, Gateway, Dataset Quality, dispatcher |

### Danh mục trạng thái động (không hardcode)

`lead-status.js` gọi `GET /api/facebook-ads/lead-statuses` → `window.LeadStatusCatalog`:

```javascript
{ key, label, short, icon, stage, color, badge, has_value, capi_event }
```

- `stage`: `new` · `contacted` · `free` · `paid` · `won` · `lost` — dùng để gom báo cáo.
- `has_value` + `capi_event`: trạng thái sinh sự kiện Meta (kèm giá trị nếu `has_value`)
  (`da_goi` → `Contact`, `da_hoc_buoi_chia_se` → `StartTrial`,
  `da_hoc_chuyen_sau` → `Subscribe`, `da_mua_hang` → `Purchase`).
  `capi_has_value` = sự kiện có cần gửi `custom_data.value`.
- `renderStatusOptions()` → đổ option của mọi `<select>` trạng thái trong bảng & drawer.
- `applyStatusCatalogToUI()` → dựng Kanban **10 cột** động từ catalog (thêm/bớt cột tự động).
- Fallback: nếu API lỗi, dùng catalog mặc định hardcode trong `lead-status.js` ⇒ UI không trắng.

### Thẻ trạng thái trong Analytics

`analytics.js` dựng biểu đồ tròn (donut) 10 lát theo `STATUS_INFO` động — màu lấy từ
trường `color` của catalog ⇒ không phải sửa lại biểu đồ khi thêm trạng thái.

### Deduplication browser ↔ server

`frontend/landing/tracker.js`:
1. `ensureLeadEventId()` sinh `event_id` (UUID v4) và lưu vào `sessionStorage` theo landing.
2. Gửi `event_id` trong payload `POST /api/leads`.
3. `fbq('track','Lead',{ content_name: '…' }, { eventID: <cùng id> })`.

Backend dùng lại đúng `event_id` đó ⇒ Meta gộp thành 1 sự kiện (không đếm đôi).

> 📌 **`eventID` là tham số THỨ 4** của `fbq`, KHÔNG phải thuộc tính trong object
> tham số thứ 3 (theo tài liệu Meta "Xử lý sự kiện Pixel và sự kiện API Chuyển đổi
> trùng lặp"):
>
> ```js
> ✅ fbq('track', 'Lead', { content_name: '…' }, { eventID: 'EVENT_ID' })
> ❌ fbq('track', 'Lead', { content_name: '…', eventID: 'EVENT_ID' })
> ```

> ⚠️ `tracker.js` có **7 bản mirror** — sửa 1 chỗ phải copy đủ:
> `tracker.js` (root), `frontend/landing/tracker.js`, `frontend/admin/tracker.js`,
> `frontend/admin/js/tracker.js`, `backend/tracker.js`, `backend/static/tracker.js`,
> `backend/static/js/tracker.js`.
> Test `backend/handlers/landing_tracker_pixel_test.go::TestTrackerMirrorsAreIdentical`
> sẽ FAIL nếu lệch.

### Deduplication PageView browser ↔ server (thêm 2026-11-16)

Ngoài `Lead`, sự kiện **`PageView`** cũng cần dedup: nếu Pixel web và CAPI server
cùng bắn mà **không đồng bộ `event_id`**, Meta tính **1 lượt vào trang thành 2 lượt
PageView** ⇒ sai lệch toàn bộ chỉ số Landing Page View + phân phối quảng cáo.

| Tầng | Thành phần | Nội dung |
|---|---|---|
| Server | `injectPageViewEventIDSnippet()` | Chèn script ở **đầu `<head>`** sinh MỘT `window.APEX_PV_EVENT_ID` (prefix `pv-`). Idempotent. |
| Server | `rewritePageViewWithEventID()` | Ghi đè mọi `fbq('track','PageView')` hardcode trong HTML landing (export LadiPage) để kèm `{ eventID: window.APEX_PV_EVENT_ID }`. Lệnh đã có `eventID` ⇒ giữ nguyên. |
| Client | `ensurePageViewEventId()` | Đọc `window.APEX_PV_EVENT_ID` (do server inject) hoặc tự sinh ⇒ 1 giá trị / lần tải trang. |
| Client | `buildAnalyticsPayload()` | Gửi `event_id` lên server trong payload `/api/analytics/track`. |
| Server | `dispatchCAPIPageView()` | Landing CÓ Pixel ⇒ chỉ bắn CAPI PageView khi có `event_id` chung. Thiếu ⇒ bỏ qua. |

**Hiển thị trong CRM:** bảng *Lịch sử gửi sự kiện* (tab Facebook Ads) hiển thị
`event_id` **đầy đủ** + badge **"✓ đã gộp với Pixel"** cho sự kiện `PageView` có
prefix `pv-` (`frontend/admin/js/facebook-ads.js` → `renderFacebookAdsLogs()`).

Checkbox *"Gửi PageView từ server"* có tooltip giải thích cơ chế dedup
(`frontend/admin/index.html`).

### Quản lý CAPI theo từng landing (bật/tắt nhanh)

| Thành phần | Mô tả |
|---|---|
| `loadFacebookAdsLandings()` | Nạp `GET /api/landings` → `fbAdsLandings` (danh sách landing trong hệ thống) |
| `renderFacebookAdsLandingOptions()` | Dựng option cho 3 dropdown: `fbAdsFormLanding`, `fbAdsLandingSelect`, `fbAdsLandingFilter` |
| `fbAdsToggleConfig(landingId, wantActive)` | Gọi `POST /api/facebook-ads/config/toggle` → bật/tắt CAPI cho 1 landing |
| `fbAdsDeleteConfig(landingId)` | Gọi `POST /api/facebook-ads/config/delete?landing_id=…` → xoá cấu hình |

- Dropdown luôn có lựa chọn đầu tiên là **Mặc định** (`landing_id = ""`) — cấu hình
  fallback áp dụng cho mọi landing chưa có cấu hình riêng.
- Option có badge `●` (đang bật) / `○` (đang tắt) để Admin thấy trạng thái ngay.
- ⚠️ **Dòng "Mặc định" CÓ nút Bật/Tắt và Xoá** (fix 2026-11-16) — trước đây
  backend trả `400 "Thiếu landing_id"` ⇒ Admin không tắt/xoá được cấu hình mặc định.
  - Tắt dòng mặc định ⇒ mọi landing **chưa** có cấu hình riêng ngừng gửi CAPI.
  - Xoá dòng mặc định ⇒ các landing đó không còn cấu hình nào ⇒ ngừng gửi CAPI.
- `fbAdsToggleConfig` hỏi xác nhận (`confirm`) trước khi **tắt** dòng mặc định
  (cảnh báo ảnh hưởng mọi landing chưa có cấu hình riêng).
- `fbAdsDeleteConfig` hiển thị cảnh báo tương tự khi xoá dòng mặc định.
- Khi render lại dropdown, lựa chọn đang dùng của Admin được giữ nguyên.

### Checkbox "Chỉ gửi khi lead đến từ Facebook / Meta"

| Thành phần | Mô tả |
|---|---|
| `fbAdsFormOnlyFB` (index.html) | Checkbox trong form cấu hình CAPI, mặc định **checked** |
| `fbAdsPopulateConfig` | Nạp `cfg.only_facebook_traffic` (thiếu ⇒ coi như `true`) |
| `fbAdsResetForm` | Reset về `true` |
| `fbAdsSaveConfig` | Gửi `only_facebook_traffic` trong payload |

- `true` ⇒ backend chỉ bắn CAPI cho lead có bằng chứng đến từ Meta
  (`fbc` / `fbclid` / `site_source_name` / `utm_source` Meta / referrer Meta / `fbp`).
  Lead organic, Google Ads, TikTok, truy cập thẳng ⇒ bị skip.
- `false` ⇒ gửi cho mọi lead (dùng khi muốn đo đạc rộng, không quan tâm so khớp).
- Backend giữ **giá trị cũ trong DB** nếu admin không gửi field này.

### Dataset Quality — render theo format Meta v27.0

`renderFacebookAdsQuality(quality)` nhận payload đã chuẩn hoá từ backend:

```javascript
{ pixel_id, fetched_at, events: [...], aggregate: {...} }
```

> 📌 Meta v27.0 trả `web` là **MẢNG** (mỗi phần tử = 1 `event_name` kèm toàn bộ
> chỉ số riêng). Bản cũ parse `web` như 1 object ⇒ mọi thẻ đều hiển thị "—".

- 4 thẻ tổng quan: EMQ TB · Coverage TB · Chi tiêu sự kiện · Số sự kiện + số vấn đề.
- Mỗi event là 1 `<details>` mở rộng được, bên trong 6 khối:
  1. **Khoá so khớp** (match_key_feedback: identifier · coverage · ACR potential)
  2. **Bỏ trùng lặp** (dedupe_key_feedback: browser · server · overall coverage)
  3. **Chất lượng & độ trễ** (coverage · goal · upload_frequency · delay 2h/24h · spend)
  4. **Chẩn đoán EMQ** (6 nhóm lỗi từ 09/2026: IP trống, IP chuyển tiếp, IPv4, FBC, email placeholder, phone placeholder)
  5. **Chẩn đoán giá trị** (value_diagnostics: issue_type · count · rate)
  6. **Nhóm quảng cáo** (event_ad_sets: tên · chi tiêu · event_use)

### Test Event Code

Ô nhập `test_event_code` trong form "Gửi Test Event" — lấy mã ở
**Events Manager → Data Sources → Pixel → Test Events**.

> ⚠️ Meta **CHỈ chấp nhận mã do Events Manager sinh ra**. Mã tự bịa (vd `TEST_123`)
> ⇒ Meta trả lỗi 100 "Invalid parameter" ⇒ Test Events không hoạt động.
> Bỏ trống ⇒ sự kiện được gửi vào **dữ liệu thật** (dùng để đối soát),
> không hiện trong Test Events.

---

## 7. Test E2E

```bash
# Node (Playwright)
node backend/tests/e2e_domain_management_test.js
node backend/tests/e2e_ad_geo_test.js

# Python
python backend/tests/test_full_suite.py
python backend/tests/test_visual_web_builder.py
```

Test cần server đang chạy + admin credentials:
```json
{ "username": "admin", "password": "ApexAdmin@2026!" }
```

---

## 8. Lưu ý về encoding

Một số file cũ trong repo lưu UTF-8 nhưng hiển thị mojibake khi đọc bằng `cmd /c type` (console codepage). Đây là **vấn đề hiển thị console**, không phải lỗi file. Dùng `Get-Content -Encoding UTF8` trong PowerShell hoặc đọc bằng editor.

File server copy từ git giữ đúng encoding UTF-8 — kiểm tra bằng:
```powershell
[System.IO.File]::ReadAllText("C:\apex\backend\README.md", [System.Text.Encoding]::UTF8).Substring(0,100)
```
