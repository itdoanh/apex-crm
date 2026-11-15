package models

import (
	"sort"
	"strings"
)

// ─── Pipeline trạng thái khách hàng (CRM) ───────────────────────────────
//
// Ngành hàng "Giải Mã Đầu Tư Hàng Hóa" chỉ cần khách mua 1 lần, không cần
// khách quay lại đăng ký. Vì vậy pipeline được thiết kế theo 3 mức:
//
//	1. Miễn phí   : moi → da_goi → ket_ban_zalo → da_hoc_buoi_chia_se (đã học buổi chia sẻ)
//	2. Có phí     : da_hoc_chuyen_sau (đã học chuyên sâu, ≈ 6.000.000đ)
//	3. Kết       : da_mua_hang (khách đã mua hàng — sự kiện chuyển đổi cuối cùng)
//
// Ngoài ra có các trạng thái kết thúc không mua:
//   - spam_khong_lien_lac : spam / không liên lạc được
//   - khong_quan_tam      : khách không quan tâm
//   - da_hoc_truoc_do     : khách đã học khoá trước đó rồi (không cần mua lại)
//   - huy                 : huỷ / sai số điện thoại
//
// ⚠️ Tương thích ngược: 3 giá trị cũ (`tiem_nang`, `chot`, `huy`) vẫn còn
// trong DB của các lead cũ. `Chot` được hợp nhất về `da_mua_hang`,
// `TiemNang` được hợp nhất về `da_hoc_buoi_chia_se` khi đọc/nhập.

const (
	// Trạng thái đầu vào
	StatusMoi = "moi" // Mới tiếp nhận (lead mới từ landing)

	// Giai đoạn sales chăm sóc
	StatusDaGoi      = "da_goi"       // Đã gọi điện
	StatusKetBanZalo = "ket_ban_zalo" // Đã kết bạn Zalo

	// Giai đoạn miễn phí
	StatusDaHocBuoiChiaSe = "da_hoc_buoi_chia_se" // Đã học buổi chia sẻ

	// Giai đoạn có phí
	StatusDaHocChuyenSau = "da_hoc_chuyen_sau" // Đã học chuyên sâu (≈6.000.000đ)

	// Giai đoạn kết — chuyển đổi cuối cùng
	StatusDaMuaHang = "da_mua_hang" // Khách hàng đã mua hàng

	// Kết thúc không mua
	StatusSpamKhongLienLac = "spam_khong_lien_lac" // Spam hoặc không liên lạc được
	StatusKhongQuanTam     = "khong_quan_tam"      // Khách không quan tâm
	StatusDaHocTruocDo     = "da_hoc_truoc_do"     // Đã học trước đó rồi
	StatusHuy              = "huy"                 // Huỷ / Sai số

	// ─── Giá trị cũ (tương thích ngược) ────────────────────────────
	StatusTiemNang  = "tiem_nang"   // Cũ → hợp nhất vào da_hoc_buoi_chia_se
	StatusChot      = "chot"        // Cũ → hợp nhất vào da_mua_hang
	StatusDaVaoZoom = "da_vao_zoom" // Cũ → hợp nhất vào da_hoc_buoi_chia_se
)

// LeadStatusDef mô tả 1 trạng thái trong pipeline.
type LeadStatusDef struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Short string `json:"short"`
	Icon  string `json:"icon"`
	// Stage phục vụ báo cáo tổng hợp:
	//   "new"       – mới, chưa chăm sóc
	//   "contacted" – sales đã tiếp xúc
	//   "free"      – đã học buổi chia sẻ (miễn phí)
	//   "paid"      – đã học chuyên sâu (đã chi tiền)
	//   "won"       – đã mua hàng (chuyển đổi cuối)
	//   "lost"      – không mua
	Stage string `json:"stage"`
	// Màu hiển thị dùng chung cho CRM UI (Tailwind).
	Color string `json:"color"`
	Badge string `json:"badge"`
	// HasValue = trạng thái gắn với giá trị tiền thật (dùng cho CAPI Purchase).
	HasValue bool `json:"has_value"`
	// CAPIEvent là tên sự kiện Meta nếu trạng thái này sinh ra chuyển đổi.
	//
	// ⚠️ Rỗng ⇒ trạng thái này KHÔNG sinh sự kiện Meta (ví dụ `ket_ban_zalo`:
	// đã kết bạn Zalo nhưng chưa phải chuyển đổi có giá trị).
	CAPIEvent string `json:"capi_event,omitempty"`
	// CAPIHasValue = sự kiện này cần gửi kèm `custom_data.value`
	// (Subscribe / Purchase có giá trị tiền thật; Lead / Contact / StartTrial
	// thì không).
	CAPIHasValue bool `json:"capi_has_value,omitempty"`
}

// LeadStatusOrder là thứ tự hiển thị trên Kanban (trái → phải).
var LeadStatusOrder = []string{
	StatusMoi,
	StatusDaGoi,
	StatusKetBanZalo,
	StatusDaHocBuoiChiaSe,
	StatusDaHocChuyenSau,
	StatusDaMuaHang,
	StatusDaHocTruocDo,
	StatusKhongQuanTam,
	StatusSpamKhongLienLac,
	StatusHuy,
}

// LeadStatusDefs là danh sách đầy đủ các trạng thái hợp lệ.
//
// 📌 ÁNH XẠ CRM → META CAPI (yêu cầu anh itdoanh 2026-11-15):
//
//	Sự kiện Meta được bắn tự động mỗi khi sales đổi trạng thái lead:
//
//	  moi                 → Lead          (form đăng ký — cũng bắn lúc submit)
//	  da_goi              → Contact       (đã gọi điện)
//	  ket_ban_zalo        → (không bắn)  — kết bạn Zalo chưa phải chuyển đổi
//	  da_hoc_buoi_chia_se → StartTrial    (đã học buổi chia sẻ ⇒ bắt đầu dùng thử)
//	  da_hoc_chuyen_sau   → Subscribe     (đã học chuyên sâu, ≈6.000.000đ)
//	  da_mua_hang         → Purchase      (khách đã mua hàng — chuyển đổi cuối)
//	  (các trạng thái kết thúc không mua → không bắn)
//
// ⚠️ `SendCRMSync` trong cấu hình CAPI điều khiển TOÀN BỘ nhóm này.
var LeadStatusDefs = []LeadStatusDef{
	{
		Key: StatusMoi, Label: "Mới tiếp nhận", Short: "Mới", Icon: "📥",
		Stage: "new", Color: "sky", Badge: "bg-sky-950 text-sky-400 border-sky-800",
		CAPIEvent: "Lead",
	},
	{
		Key: StatusDaGoi, Label: "Đã gọi điện", Short: "Đã gọi", Icon: "📞",
		Stage: "contacted", Color: "amber", Badge: "bg-amber-950 text-amber-400 border-amber-800",
		CAPIEvent: "Contact",
	},
	{
		Key: StatusKetBanZalo, Label: "Đã kết bạn Zalo", Short: "Zalo", Icon: "💬",
		Stage: "contacted", Color: "blue", Badge: "bg-blue-950 text-blue-400 border-blue-800",
	},
	{
		Key: StatusDaHocBuoiChiaSe, Label: "Đã học buổi chia sẻ", Short: "Chia sẻ", Icon: "💻",
		Stage: "free", Color: "purple", Badge: "bg-purple-950 text-purple-400 border-purple-800",
		CAPIEvent: "StartTrial",
	},
	{
		Key: StatusDaHocChuyenSau, Label: "Đã học chuyên sâu", Short: "Chuyên sâu", Icon: "📘",
		Stage: "paid", Color: "indigo", Badge: "bg-indigo-950 text-indigo-400 border-indigo-800",
		HasValue: true, CAPIEvent: "Subscribe", CAPIHasValue: true,
	},
	{
		Key: StatusDaMuaHang, Label: "Khách hàng đã mua hàng", Short: "Đã mua", Icon: "🏆",
		Stage: "won", Color: "emerald", Badge: "bg-emerald-950 text-emerald-400 border-emerald-800",
		HasValue: true, CAPIEvent: "Purchase", CAPIHasValue: true,
	},
	{
		Key: StatusDaHocTruocDo, Label: "Đã học trước đó rồi", Short: "Học trước", Icon: "🔁",
		Stage: "lost", Color: "slate", Badge: "bg-slate-950 text-slate-400 border-slate-800",
	},
	{
		Key: StatusKhongQuanTam, Label: "Không quan tâm", Short: "Không QT", Icon: "🙅",
		Stage: "lost", Color: "orange", Badge: "bg-orange-950 text-orange-400 border-orange-800",
	},
	{
		Key: StatusSpamKhongLienLac, Label: "Spam / Không liên lạc được", Short: "Spam", Icon: "🚫",
		Stage: "lost", Color: "rose", Badge: "bg-rose-950 text-rose-400 border-rose-800",
	},
	{
		Key: StatusHuy, Label: "Hủy / Sai số", Short: "Hủy", Icon: "❌",
		Stage: "lost", Color: "zinc", Badge: "bg-zinc-950 text-zinc-400 border-zinc-800",
	},
}

// leadStatusMap tra cứu nhanh theo key.
var leadStatusMap = func() map[string]LeadStatusDef {
	m := make(map[string]LeadStatusDef, len(LeadStatusDefs))
	for _, d := range LeadStatusDefs {
		m[d.Key] = d
	}
	return m
}()

// NormalizeLeadStatus chuẩn hoá trạng thái, hợp nhất giá trị cũ về giá trị mới.
// Trả về key hợp lệ; nếu rỗng thì mặc định `moi`.
func NormalizeLeadStatus(raw string) string {
	switch raw {
	case StatusTiemNang:
		// "Khách tiềm năng" của pipeline cũ ≈ đã tiếp nhận & quan tâm
		// → hợp nhất vào bước đã học buổi chia sẻ (giai đoạn miễn phí).
		return StatusDaHocBuoiChiaSe
	case StatusChot:
		// "Đã chốt HĐ" ≈ khách đã mua hàng.
		return StatusDaMuaHang
	case StatusDaVaoZoom:
		// "Đã vào Zoom" của pipeline cũ ≈ đã học buổi chia sẻ.
		return StatusDaHocBuoiChiaSe
	case "":
		return StatusMoi
	}
	if _, ok := leadStatusMap[raw]; ok {
		return raw
	}
	return StatusMoi
}

// IsValidLeadStatus kiểm tra trạng thái có hợp lệ không.
//
// Khác với NormalizeLeadStatus (luôn trả về 1 trạng thái hợp lệ), hàm này
// trả FALSE nếu giá trị gốc không nằm trong danh mục — dùng để validate input
// từ API/UI mà KHÔNG tự động gán "moi" (tránh nuốt nhầm trạng thái).
// Giá trị cũ (`tiem_nang`, `chot`) vẫn được coi là hợp lệ vì có thể còn trong DB.
func IsValidLeadStatus(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	if _, ok := leadStatusMap[raw]; ok {
		return true
	}
	switch raw {
	case StatusTiemNang, StatusChot, StatusDaVaoZoom:
		return true
	}
	return false
}

// GetLeadStatus trả về định nghĩa trạng thái.
func GetLeadStatus(key string) (LeadStatusDef, bool) {
	d, ok := leadStatusMap[NormalizeLeadStatus(key)]
	return d, ok
}

// LeadStatusValue là trạng thái gắn giá trị tiền thật (dùng cho CAPI Purchase).
//
// ⚠️ HÀM NÀY GIỮ NGUYÊN HÀNH VI CŨ (chỉ trả về true cho trạng thái có
// giá trị tiền) để không phá code đang gọi. Dùng `LeadStatusCAPIEvent`
// cho logic đồng bộ CRM ⇄ CAPI mới.
func LeadStatusValue(key string) (bool, string) {
	d, ok := leadStatusMap[NormalizeLeadStatus(key)]
	if !ok || !d.HasValue {
		return false, ""
	}
	return true, d.CAPIEvent
}

// LeadStatusCAPIEvent trả về thông tin sự kiện Meta tương ứng với 1 trạng
// thái CRM.
//
// Trả về ("", false) nếu trạng thái đó KHÔNG sinh sự kiện Meta (ví dụ
// `ket_ban_zalo`, các trạng thái kết thúc không mua).
//
// hasValue = true ⇒ sự kiện cần gửi kèm `custom_data.value`
// (Subscribe / Purchase).
func LeadStatusCAPIEvent(key string) (eventName string, hasValue bool, ok bool) {
	d, found := leadStatusMap[NormalizeLeadStatus(key)]
	if !found || strings.TrimSpace(d.CAPIEvent) == "" {
		return "", false, false
	}
	return d.CAPIEvent, d.CAPIHasValue, true
}

// ListLeadStatusDefs trả về toàn bộ định nghĩa trạng thái (cho API admin).
func ListLeadStatusDefs() []LeadStatusDef {
	out := make([]LeadStatusDef, len(LeadStatusDefs))
	copy(out, LeadStatusDefs)
	sort.SliceStable(out, func(i, j int) bool {
		pi, pj := indexOfStatus(out[i].Key), indexOfStatus(out[j].Key)
		return pi < pj
	})
	return out
}

func indexOfStatus(key string) int {
	for i, k := range LeadStatusOrder {
		if k == key {
			return i
		}
	}
	return len(LeadStatusOrder)
}
