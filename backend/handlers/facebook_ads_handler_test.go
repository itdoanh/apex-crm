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
	_ = store.DeleteFacebookCAPIConfig("__test_handler_capi__")
}
