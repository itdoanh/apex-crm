package db

import (
	"context"
	"os"
	"testing"

	"apex-backend/models"
)

// TestUpsertFacebookCAPIConfigRoundTrip kiểm tra lệnh INSERT/ON CONFLICT của
// cấu hình CAPI chạy đúng trên DB thật (cần DB local).
//
// 🐛 Regression 2026-11-15: câu VALUES chỉ liệt kê $1..$23 trong khi bảng có
// 24 cột tham số (thiếu `send_user_data`) ⇒ Postgres trả
// "INSERT has more target columns than expressions" ⇒ HTTP 500 khi lưu cấu hình.
func TestUpsertFacebookCAPIConfigRoundTrip(t *testing.T) {
	store, err := NewStore(testConnString())
	if err != nil {
		t.Skipf("bỏ qua (không kết nối được DB): %v", err)
	}

	const landingID = "__test_upsert_capi__"
	defer func() {
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM facebook_capi_configs WHERE landing_id = $1`, landingID)
	}()

	isActive := true
	in := models.FacebookCAPIConfigInput{
		LandingID:      landingID,
		IsActive:       &isActive,
		PixelID:        "1234567890",
		AccessToken:    "test-token-abcdefgh",
		APIVersion:     "v27.0",
		SendPageView:   true,
		SendLead:       true,
		SendPurchase:   true,
		SendCRMSync:    true,
		CourseValue:    6000000,
		Currency:       "VND",
		HashEmail:      true,
		HashPhone:      true,
		HashName:       true,
		HashAddress:    true,
		HashExternalID: true,
		HashCity:       true,
		HashState:      true,
		HashZip:        true,
		HashCountry:    true,
		SendUserData:   true,
	}

	cfg, err := store.UpsertFacebookCAPIConfig(in)
	if err != nil {
		t.Fatalf("Upsert lần 1 lỗi: %v", err)
	}
	if cfg == nil {
		t.Fatal("Upsert lần 1 phải trả về cấu hình")
	}
	if !cfg.SendUserData {
		t.Error("send_user_data phải được lưu là TRUE")
	}
	if cfg.PixelID != "1234567890" {
		t.Errorf("pixel_id sai: %q", cfg.PixelID)
	}
	if cfg.CourseValue != 6000000 {
		t.Errorf("course_value sai: %v", cfg.CourseValue)
	}

	// Lần 2: đổi giá trị + token rỗng ⇒ phải GIỮ token cũ.
	in.PixelID = "999"
	in.AccessToken = ""
	cfg2, err := store.UpsertFacebookCAPIConfig(in)
	if err != nil {
		t.Fatalf("Upsert lần 2 lỗi: %v", err)
	}
	if cfg2.AccessToken != "test-token-abcdefgh" {
		t.Errorf("token cũ phải được giữ, có: %q", cfg2.AccessToken)
	}
	if cfg2.PixelID != "999" {
		t.Errorf("pixel_id phải được cập nhật, có: %q", cfg2.PixelID)
	}

	// Xoá dòng test.
	if err := store.DeleteFacebookCAPIConfig(landingID); err != nil {
		t.Fatalf("xoá cấu hình test lỗi: %v", err)
	}
}

// TestUpsertFacebookCAPIConfigEmptyLandingID đảm bảo cấu hình mặc định
// (landing_id = ”) cũng lưu được — đây là dòng mà Admin UI dùng khi
// chưa chọn landing cụ thể.
func TestUpsertFacebookCAPIConfigEmptyLandingID(t *testing.T) {
	if os.Getenv("APEX_TEST_CAPI_DEFAULT") == "" {
		t.Skip("bỏ qua (đặt APEX_TEST_CAPI_DEFAULT=1 để chạy, sẽ ghi đè cấu hình mặc định)")
	}
	store, err := NewStore(testConnString())
	if err != nil {
		t.Skipf("bỏ qua (không kết nối được DB): %v", err)
	}
	isActive := false
	cfg, err := store.UpsertFacebookCAPIConfig(models.FacebookCAPIConfigInput{
		LandingID: "",
		IsActive:  &isActive,
		PixelID:   "0000000000",
	})
	if err != nil {
		t.Fatalf("Upsert cấu hình mặc định lỗi: %v", err)
	}
	if cfg == nil {
		t.Fatal("phải trả về cấu hình mặc định")
	}
}
