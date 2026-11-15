package googlesheets

import (
	"testing"

	"apex-backend/models"
)

func TestParseRowNumber(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"Data!A123:BS123", 123, true},
		{"'Sheet Data'!A5:C5", 5, true},
		{"A42", 42, true},
		{"A1", 1, true},
		{"", 0, false},
		{"abc", 0, false},
		{"A0", 0, false},
	}
	for _, c := range cases {
		got, ok := ParseRowNumber(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("ParseRowNumber(%q) = (%d, %v), want (%d, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestStatusColumnIndexes(t *testing.T) {
	mappings := []models.FieldMapping{
		{Field: "thoi_gian"},
		{Field: "fullName"},
		{Field: "trang_thai"},
		{Field: "trang_thai_label"},
		{Field: "ghi_chu"},
		{Field: "utm_campaign"},
	}
	idx := StatusColumnIndexes(mappings)
	if idx == nil {
		t.Fatal("expected non-nil index map")
	}
	if idx["trang_thai"] != 2 || idx["trang_thai_label"] != 3 || idx["ghi_chu"] != 4 {
		t.Fatalf("unexpected indexes: %#v", idx)
	}
	if _, ok := idx["utm_campaign"]; ok {
		t.Fatal("utm_campaign must not be a status field")
	}
}

func TestStatusColumnIndexes_KhongCoTruongTrangThai(t *testing.T) {
	mappings := []models.FieldMapping{
		{Field: "thoi_gian"},
		{Field: "fullName"},
	}
	if idx := StatusColumnIndexes(mappings); idx != nil {
		t.Fatalf("expected nil, got %#v", idx)
	}
}

func TestBuildStatusRowValues(t *testing.T) {
	mappings := []models.FieldMapping{
		{Field: "thoi_gian"},
		{Field: "trang_thai"},
		{Field: "trang_thai_label"},
		{Field: "ghi_chu"},
	}
	lead := &models.Lead{
		TrangThai: models.StatusDaMuaHang,
		GhiChu:    "Đã chốt",
	}

	cells := BuildStatusRowValues(mappings, lead, map[string]interface{}{})
	if cells == nil {
		t.Fatal("expected non-nil cells")
	}
	if cells[1] != models.StatusDaMuaHang {
		t.Errorf("trang_thai: got %v", cells[1])
	}
	if cells[2] != "Khách hàng đã mua hàng" {
		t.Errorf("trang_thai_label: got %v", cells[2])
	}
	if cells[3] != "Đã chốt" {
		t.Errorf("ghi_chu: got %v", cells[3])
	}
	// Cột không phải trạng thái (thoi_gian) không được ghi.
	if _, ok := cells[0]; ok {
		t.Error("thoi_gian must not be updated on status change")
	}
}

// Ghi chú rỗng ⇒ BỎ QUA (không ghi đè dữ liệu người dùng nhập tay).
func TestBuildStatusRowValues_GhiChuRong_ThenBoQua(t *testing.T) {
	mappings := []models.FieldMapping{
		{Field: "trang_thai"},
		{Field: "ghi_chu"},
	}
	lead := &models.Lead{TrangThai: models.StatusDaGoi}

	cells := BuildStatusRowValues(mappings, lead, map[string]interface{}{})
	if _, ok := cells[1]; ok {
		t.Fatalf("ghi_chu rỗng phải bị bỏ qua, got %#v", cells)
	}
	if cells[0] != models.StatusDaGoi {
		t.Fatalf("trang_thai: got %v", cells[0])
	}
}
