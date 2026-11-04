---
name: add-api-endpoint
description: Thêm endpoint REST mới vào backend Go của dự án Apex Fintech — model, store method, handler, đăng ký route, cập nhật docs. Dùng khi người dùng yêu cầu thêm API, lấy dữ liệu mới, hoặc tích hợp tính năng mới cần gọi từ frontend.
---

# Thêm API Endpoint — Apex Fintech

Thực hiện đầy đủ 5 bước. Bỏ bước nào → frontend sẽ lỗi lúc runtime.

---

## Bước 1 — Model (`backend/models/`)

```go
// backend/models/thong_ke_abc.go
package models

type ThongKeABC struct {
    ID        int64  `json:"id"`
    Ten       string `json:"ten"`
    SoLuong   int    `json:"so_luong"`
    CreatedAt string `json:"created_at"`
}
```

**Quy ước JSON:** `snake_case` (`created_at`, `so_dien_thoai`). Frontend đọc trực tiếp từ JSON → phải khớp.

---

## Bước 2 — Store method (`backend/db/`)

```go
// backend/db/thong_ke.go
func (s *Store) GetThongKeABC(ctx context.Context, landingID string, fromDate, toDate string) ([]models.ThongKeABC, error) {
    ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
    defer cancel()

    args := []interface{}{}
    conditions := []string{}

    if strings.TrimSpace(landingID) != "" {
        args = append(args, strings.TrimSpace(landingID))
        conditions = append(conditions, fmt.Sprintf("landing_id = $%d", len(args)))
    }

    where := ""
    if len(conditions) > 0 {
        where = "WHERE " + strings.Join(conditions, " AND ")
    }

    query := fmt.Sprintf(`
        SELECT id, ten, so_luong,
               TO_CHAR(thoi_gian AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS')
        FROM leads
        %s
        ORDER BY id DESC
        LIMIT 500;
    `, where)

    rows, err := s.pool.Query(ctx, query, args...)
    if err != nil {
        return nil, fmt.Errorf("lỗi truy vấn thống kê ABC: %w", err)
    }
    defer rows.Close()

    items := make([]models.ThongKeABC, 0)
    for rows.Next() {
        var it models.ThongKeABC
        if err := rows.Scan(&it.ID, &it.Ten, &it.SoLuong, &it.CreatedAt); err != nil {
            return nil, fmt.Errorf("lỗi đọc dòng thống kê: %w", err)
        }
        items = append(items, it)
    }
    return items, rows.Err()
}
```

### Bắt buộc
- Parameterized query — **không** nối chuỗi user input vào SQL.
- `AT TIME ZONE 'Asia/Ho_Chi_Minh'` cho mọi so sánh thời gian.
- `context.WithTimeout`.
- `COALESCE` cho cột có thể NULL.
- Wrap lỗi có context.

---

## Bước 3 — Handler (`backend/handlers/`)

```go
// backend/handlers/thong_ke_handler.go
package handlers

type ThongKeHandler struct {
    store *db.Store
}

func NewThongKeHandler(store *db.Store) *ThongKeHandler {
    return &ThongKeHandler{store: store}
}

func (h *ThongKeHandler) HandleGetThongKe(w http.ResponseWriter, r *http.Request) {
    landingID := r.URL.Query().Get("landing_id")
    fromDate := r.URL.Query().Get("from_date")
    toDate := r.URL.Query().Get("to_date")

    items, err := h.store.GetThongKeABC(r.Context(), landingID, fromDate, toDate)
    if err != nil {
        log.Printf("[ERROR] lấy thống kê ABC: %v", err)
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusInternalServerError)
        _, _ = w.Write([]byte(`{"success":false,"error":"Không thể tải dữ liệu"}`))
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    _ = json.NewEncoder(w).Encode(map[string]interface{}{
        "success": true,
        "data":    items,
        "total":   len(items),
    })
}
```

### Bắt buộc
- Set `Content-Type` **trước** `WriteHeader`.
- `WriteHeader` **trước** `Write`.
- **Không** leak chi tiết lỗi nội bộ ra response → trả thông báo chung, chi tiết vào log.
- Luôn `log.Printf("[ERROR] ...")` khi lỗi.

---

## Bước 4 — Đăng ký route (`backend/main.go`)

### Endpoint ADMIN (phần lớn trường hợp)
```go
thongKeHandler := handlers.NewThongKeHandler(store)

mux.HandleFunc("/api/thong-ke/abc",
    leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(thongKeHandler.HandleGetThongKe)))
```

### Endpoint PUBLIC (rất hiếp — chỉ khi thực sự cần)
```go
mux.HandleFunc("/api/landing/config",
    leadHandler.CORSMiddleware(landingHandler.HandleGetConfig))
```

### ⚠️ Thứ tự middleware BẮT BUỘC
```
CORSMiddleware( RequireAdminAuth( handler ) )
```
Đảo ngược → response 401 không có header CORS → browser báo lỗi CORS thay vì 401 rõ ràng.

### Route theo method
```go
mux.HandleFunc("/api/thong-ke/abc", func(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        thongKeHandler.HandleGetThongKe(w, r)
    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
})
```

---

## Bước 5 — Cập nhật docs

```markdown
<!-- docs/API.md -->
### `GET /api/thong-ke/abc` — Admin
Query: `?landing_id=&from_date=&to_date=`

Response:
```json
{ "success": true, "data": [ ... ], "total": 2 }
```
```

Cũng cập nhật `docs/DATABASE.md` nếu có bảng/cột mới.

---

## Bước 6 (nếu có) — Frontend gọi endpoint

```javascript
const res = await fetch(`${API_BASE}/api/thong-ke/abc?landing_id=${encodeURIComponent(id)}`, {
    headers: { 'Authorization': `Bearer ${getToken()}` }
});
if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || `HTTP ${res.status}`);
}
const data = await res.json();
```

**Luôn** `encodeURIComponent` cho query param, kiểm tra `res.ok`, escape output HTML.

---

## Bước 7 — Test

```bash
# Local
cd backend && go run main.go

# Token
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"ApexAdmin@2026!"}'

# Gọi endpoint
curl "http://localhost:8080/api/thong-ke/abc?landing_id=mxv_080_online" \
  -H "Authorization: Bearer <token>"

# Không token → phải 401
curl -i "http://localhost:8080/api/thong-ke/abc"
```

---

## Checklist

```
□ Model có tag JSON snake_case
□ Query parameterized, không nối chuỗi input
□ Có AT TIME ZONE 'Asia/Ho_Chi_Minh' nếu query thời gian
□ Có context.WithTimeout
□ Handler set Content-Type + WriteHeader trước Write
□ Handler log lỗi, không leak chi tiết ra response
□ Route bọc CORSMiddleware(RequireAdminAuth(handler)) — đúng thứ tự
□ Đã cập nhật docs/API.md
□ go build thành công
□ Test trả đúng, test không token trả 401
```

---

## Endpoint PUBLIC — thận trọng

Chỉ làm PUBLIC khi:
- Không chứa dữ liệu nhạy cảm
- Không tạo tải lớn lên DB
- Không bị lạm dụng (spam, scrape)

Nếu public → phải có rate limit + PoW nếu là endpoint ghi dữ liệu.
Xem `docs/SECURITY.md`.
