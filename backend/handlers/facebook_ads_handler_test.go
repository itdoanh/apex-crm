package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"apex-backend/db"
	"apex-backend/models"
)

// newTestStore mở store test trên DB local (docker apex-postgres).
func newTestStore() (*db.Store, error) {
	conn := os.Getenv("APEX_TEST_DATABASE_URL")
	if conn == "" {
		conn = "postgres://postgres:postgres@localhost:5432/apex_leads?sslmode=disable"
	}
	return db.NewStore(conn)
}

// TestFacebookAdsSaveConfigHandler kiểm tra handler POST /api/facebook-ads/config.
//
// 🐛 Regression 2026-11-15: câu INSERT thiếu 1 tham số ($24 = send_user_data)
// ⇒ Postgres trả "INSERT has more target columns than expressions" ⇒ HTTP 500.
func TestFacebookAdsSaveConfigHandler(t *testing.T) {
	store, err := newTestStore()
	if err != nil {
		t.Skipf("bỏ qua (không kết nối được DB): %v", err)
	}

	h := NewFacebookAdsHandler(nil, store)

	body, _ := json.Marshal(models.FacebookCAPIConfigInput{
		LandingID:      "__test_handler_capi__",
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
	})

	req := httptest.NewRequest(http.MethodPost, "/api/facebook-ads/config", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.HandleSaveConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("HTTP %d, body = %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Success bool `json:"success"`
		Config  struct {
			PixelID      string `json:"pixel_id"`
			SendUserData bool   `json:"send_user_data"`
		} `json:"config"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response lỗi: %v", err)
	}
	if !resp.Success {
		t.Fatalf("success = false: %s", rec.Body.String())
	}
	if resp.Config.PixelID != "1234567890" {
		t.Errorf("pixel_id sai: %q", resp.Config.PixelID)
	}
	if !resp.Config.SendUserData {
		t.Error("send_user_data phải TRUE")
	}

	// Dọn dẹp.
	_, _ = store.DeleteFacebookCAPIConfig("__test_handler_capi__")
}

// TestFacebookAdsToggleDeleteDefaultConfig kiểm tra Admin có thể BẬT/TẮT và
// XOÁ được DÒNG CẤU HÌNH MẶC ĐỊNH (landing_id = '').
//
// 🐛 Regression 2026-11-16: handler trả 400 "Thiếu landing_id" cho landing_id
// rỗng ⇒ Admin KHÔNG tắt/xoá được cấu hình mặc định.
func TestFacebookAdsToggleDeleteDefaultConfig(t *testing.T) {
	store, err := newTestStore()
	if err != nil {
		t.Skipf("bỏ qua (không kết nối được DB): %v", err)
	}

	h := NewFacebookAdsHandler(nil, store)

	// ⚠️ Backup dòng cấu hình mặc định hiện có (nếu có) ⇒ restore sau test,
	// không phá dữ liệu local.
	backup, _ := store.GetFacebookCAPIConfigExact("")
	restore := func() {
		if backup == nil {
			_, _ = store.DeleteFacebookCAPIConfig("")
			return
		}
		_, _ = store.UpsertFacebookCAPIConfig(models.FacebookCAPIConfigInput{
			LandingID:   "",
			IsActive:    &backup.IsActive,
			PixelID:     backup.PixelID,
			AccessToken: backup.AccessToken,
			APIVersion:  backup.APIVersion,
			SendLead:    backup.SendLead,
			CourseValue: backup.CourseValue,
			Currency:    backup.Currency,
		})
	}
	defer restore()

	// Dựng dòng cấu hình mặc định (landing_id = '') — dùng landing_id rỗng.
	isActive := true
	if _, err := store.UpsertFacebookCAPIConfig(models.FacebookCAPIConfigInput{
		LandingID:   "",
		IsActive:    &isActive,
		PixelID:     "__test_default_pixel__",
		AccessToken: "__test_default_token__",
		APIVersion:  "v27.0",
		SendLead:    true,
		CourseValue: 6000000,
		Currency:    "VND",
	}); err != nil {
		t.Skipf("bỏ qua (không ghi được dòng mặc định): %v", err)
	}

	// ─── 1) TOGGLE OFF dòng mặc định ───────────────────────────────
	toggleOff := `{"landing_id": "", "is_active": false}`
	req := httptest.NewRequest(http.MethodPost, "/api/facebook-ads/config/toggle", bytes.NewReader([]byte(toggleOff)))
	rec := httptest.NewRecorder()
	h.HandleToggleConfig(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("toggle OFF cấu hình mặc định: HTTP %d, body = %s", rec.Code, rec.Body.String())
	}

	cfg, err := store.GetFacebookCAPIConfigExact("")
	if err != nil {
		t.Fatalf("đọc cấu hình mặc định lỗi: %v", err)
	}
	if cfg == nil {
		t.Fatal("dòng cấu hình mặc định biến mất sau khi toggle")
	}
	if cfg.IsActive {
		t.Error("cấu hình mặc định phải TẮT sau khi toggle is_active=false")
	}

	// ─── 2) TOGGLE ON lại dòng mặc định ─────────────────────────────
	toggleOn := `{"landing_id": "", "is_active": true}`
	req = httptest.NewRequest(http.MethodPost, "/api/facebook-ads/config/toggle", bytes.NewReader([]byte(toggleOn)))
	rec = httptest.NewRecorder()
	h.HandleToggleConfig(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("toggle ON cấu hình mặc định: HTTP %d, body = %s", rec.Code, rec.Body.String())
	}

	cfg, _ = store.GetFacebookCAPIConfigExact("")
	if cfg == nil || !cfg.IsActive {
		t.Error("cấu hình mặc định phải BẬT sau khi toggle is_active=true")
	}

	// ─── 3) XOÁ dòng mặc định ───────────────────────────────────────
	req = httptest.NewRequest(http.MethodPost, "/api/facebook-ads/config/delete?landing_id=", nil)
	rec = httptest.NewRecorder()
	h.HandleDeleteConfig(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("xoá cấu hình mặc định: HTTP %d, body = %s", rec.Code, rec.Body.String())
	}

	cfg, _ = store.GetFacebookCAPIConfigExact("")
	if cfg != nil {
		t.Error("dòng cấu hình mặc định phải bị xoá")
	}
}
