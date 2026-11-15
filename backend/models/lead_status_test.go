package models

import "testing"

// ─── Pipeline trạng thái CRM 10 cột ────────────────────────────────────

func TestLeadStatusDefsCount(t *testing.T) {
	if len(LeadStatusDefs) != 10 {
		t.Fatalf("phải có đúng 10 trạng thái, thực tế %d", len(LeadStatusDefs))
	}
}

func TestLeadStatusOrderMatchesDefs(t *testing.T) {
	if len(LeadStatusOrder) != len(LeadStatusDefs) {
		t.Fatalf("LeadStatusOrder (%d) lệch số trạng thái (%d)", len(LeadStatusOrder), len(LeadStatusDefs))
	}
	inOrder := map[string]bool{}
	for _, k := range LeadStatusOrder {
		if inOrder[k] {
			t.Fatalf("trạng thái %q bị lặp trong LeadStatusOrder", k)
		}
		inOrder[k] = true
	}
	for _, d := range LeadStatusDefs {
		if !inOrder[d.Key] {
			t.Fatalf("trạng thái %q có trong LeadStatusDefs nhưng thiếu trong LeadStatusOrder", d.Key)
		}
	}
}

func TestNewStatusesExist(t *testing.T) {
	// 4 trạng thái bổ sung theo yêu cầu nghiệp vụ.
	for _, k := range []string{
		StatusSpamKhongLienLac,
		StatusKhongQuanTam,
		StatusDaMuaHang,
		StatusDaHocTruocDo,
	} {
		if _, ok := GetLeadStatus(k); !ok {
			t.Fatalf("thiếu trạng thái %q", k)
		}
	}
}

func TestNormalizeLeadStatusLegacy(t *testing.T) {
	cases := map[string]string{
		StatusTiemNang:  StatusDaHocBuoiChiaSe,
		StatusChot:      StatusDaMuaHang,
		StatusDaVaoZoom: StatusDaHocBuoiChiaSe,
		"":              StatusMoi,
		"khong_ton_tai": StatusMoi,
	}
	for in, want := range cases {
		if got := NormalizeLeadStatus(in); got != want {
			t.Fatalf("NormalizeLeadStatus(%q) = %q, muốn %q", in, got, want)
		}
	}
}

func TestIsValidLeadStatus(t *testing.T) {
	if !IsValidLeadStatus(StatusDaMuaHang) {
		t.Fatal("da_mua_hang phải hợp lệ")
	}
	if !IsValidLeadStatus(StatusChot) {
		t.Fatal("giá trị cũ 'chot' sau khi chuẩn hoá phải hợp lệ")
	}
	if IsValidLeadStatus("xyz") {
		t.Fatal("trạng thái không tồn tại phải không hợp lệ")
	}
}

func TestLeadStatusValueOnlyPaidStatuses(t *testing.T) {
	// Chỉ 2 trạng thái có giá trị tiền thật mới sinh sự kiện Meta.
	for _, k := range []string{StatusDaHocChuyenSau, StatusDaMuaHang} {
		hasValue, ev := LeadStatusValue(k)
		if !hasValue || ev == "" {
			t.Fatalf("trạng thái %q phải có giá trị + tên sự kiện CAPI", k)
		}
	}
	if hasValue, _ := LeadStatusValue(StatusDaHocBuoiChiaSe); hasValue {
		t.Fatal("da_hoc_buoi_chia_se là bước miễn phí → không sinh sự kiện có giá trị")
	}
	for _, k := range []string{StatusMoi, StatusDaGoi, StatusKetBanZalo, StatusKhongQuanTam,
		StatusSpamKhongLienLac, StatusDaHocTruocDo, StatusHuy} {
		if hasValue, _ := LeadStatusValue(k); hasValue {
			t.Fatalf("trạng thái %q không được gắn giá trị", k)
		}
	}
}

func TestLeadStatusValueEventNames(t *testing.T) {
	if _, ev := LeadStatusValue(StatusDaHocChuyenSau); ev != "Subscribe" {
		t.Fatalf("da_hoc_chuyen_sau phải map sang Subscribe, thực tế %q", ev)
	}
	if _, ev := LeadStatusValue(StatusDaMuaHang); ev != "Purchase" {
		t.Fatalf("da_mua_hang phải map sang Purchase, thực tế %q", ev)
	}
}

func TestLeadStatusStages(t *testing.T) {
	want := map[string]string{
		StatusMoi:              "new",
		StatusDaGoi:            "contacted",
		StatusKetBanZalo:       "contacted",
		StatusDaHocBuoiChiaSe:  "free",
		StatusDaHocChuyenSau:   "paid",
		StatusDaMuaHang:        "won",
		StatusDaHocTruocDo:     "lost",
		StatusKhongQuanTam:     "lost",
		StatusSpamKhongLienLac: "lost",
		StatusHuy:              "lost",
	}
	for k, stage := range want {
		d, ok := GetLeadStatus(k)
		if !ok {
			t.Fatalf("thiếu trạng thái %q", k)
		}
		if d.Stage != stage {
			t.Fatalf("%q: stage = %q, muốn %q", k, d.Stage, stage)
		}
	}
}

func TestListLeadStatusDefsSortedByOrder(t *testing.T) {
	list := ListLeadStatusDefs()
	if len(list) != len(LeadStatusOrder) {
		t.Fatalf("ListLeadStatusDefs trả %d mục, muốn %d", len(list), len(LeadStatusOrder))
	}
	for i, k := range LeadStatusOrder {
		if list[i].Key != k {
			t.Fatalf("vị trí %d = %q, muốn %q", i, list[i].Key, k)
		}
	}
}
