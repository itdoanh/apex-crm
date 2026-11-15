package db

import (
	"context"
	"os"
	"testing"
)

// testConnStringReport lấy chuỗi kết nối DB cho test.
func testConnStringReport() string {
	if s := os.Getenv("APEX_TEST_DATABASE_URL"); s != "" {
		return s
	}
	return "postgres://postgres:postgres@localhost:5432/apex_leads?sslmode=disable"
}

// TestGetReportAdHierarchyNoGroupByError là regression test cho bug 2026-11-15:
// tab "Quảng cáo" trả HTTP 500 với lỗi:
//
//	ERROR: column "analytics_sessions.utm_campaign" must appear in the GROUP BY
//	       clause or be used in an aggregate function (SQLSTATE 42803)
//
// NGUYÊN NHÂN: PostgreSQL resolve `GROUP BY campaign_name` theo TÊN CỘT THẬT
// của bảng (`analytics_sessions.campaign_name`), KHÔNG phải output alias. Trong
// khi đó SELECT dùng:
//
//	COALESCE(NULLIF(campaign_name,''), NULLIF(utm_campaign,''), 'none')
//
// ⇒ `utm_campaign` không nằm trong GROUP BY ⇒ 42803.
//
// CÁCH SỬA: dùng CTE tính sẵn giá trị hiển thị rồi GROUP BY trên cột của
// CTE (tên `grp_campaign`/`grp_adset`/`grp_ad` không trùng cột thật).
//
// KỲ VỌNG: hàm KHÔNG trả lỗi SQL (dù không có dữ liệu cũng phải OK).
func TestGetReportAdHierarchyNoGroupByError(t *testing.T) {
	store, err := NewStore(testConnStringReport())
	if err != nil {
		t.Skipf("không kết nối được DB test: %v", err)
	}
	ctx := context.Background()

	// Chèn 1 dòng mẫu để chắc chắn nhánh GROUP BY thực sự được thực thi.
	const landingID = "test_ad_hierarchy_groupby"
	_, _ = store.pool.Exec(ctx, `
		INSERT INTO analytics_sessions (
			session_id, visitor_id, landing_id, nguon,
			campaign_name, adset_name, ad_name,
			utm_campaign, utm_content, utm_term,
			converted_lead, is_bot
		) VALUES (
			'sess_test_ad_hierarchy_1', 'vis_test_ad_hierarchy_1', $1, 'test',
			'', '', '', '120254773249580274', '120254773249590274', '120254773249600274',
			TRUE, FALSE
		)
		ON CONFLICT (session_id) DO NOTHING
	`, landingID)
	defer func() {
		_, _ = store.pool.Exec(ctx, `DELETE FROM analytics_sessions WHERE session_id = $1`, "sess_test_ad_hierarchy_1")
	}()

	filter := ReportFilter{
		LandingIDs: []string{landingID},
		FromDate:   "2020-01-01",
		ToDate:     "2099-12-31",
		FromHour:   -1, // -1 = không lọc khung giờ
		ToHour:     -1,
		// TrafficScope rỗng ⇒ mặc định "human" (is_bot = FALSE) ⇒ dòng mẫu
		// phải có is_bot = FALSE (đã set ở trên).
	}

	items, err := store.GetReportAdHierarchy(filter, 300)
	if err != nil {
		t.Fatalf("GetReportAdHierarchy phải chạy được (không lỗi GROUP BY), nhận lỗi: %v", err)
	}

	// Dòng mẫu có campaign_name='' ⇒ phải fallback sang utm_campaign.
	found := false
	for _, it := range items {
		if it["campaign_name"] == "120254773249580274" {
			found = true
			if it["adset_name"] != "120254773249590274" {
				t.Fatalf("adset_name sai: %v", it["adset_name"])
			}
			if it["ad_name"] != "120254773249600274" {
				t.Fatalf("ad_name sai: %v", it["ad_name"])
			}
		}
	}
	if !found {
		t.Fatalf("không tìm thấy dòng fallback utm_campaign trong kết quả: %+v", items)
	}
}

// TestAllReportQueriesRunWithoutSQLError là regression test bao trùm cho
// bug 2026-11-15 (lỗi 42803 khiến tab Quảng cáo / Báo cáo trả HTTP 500).
//
// ⚠️ PostgreSQL resolve `GROUP BY <tên>` theo TÊN CỘT THẬT của bảng trước,
// rồi mới tới output alias. Nếu SELECT có biểu thức COALESCE fallback sang
// cột KHÁC (vd `COALESCE(NULLIF(campaign_name,''), NULLIF(utm_campaign,''))`)
// mà GROUP BY chỉ nhắm tên cột thật ⇒ cột fallback không nằm trong
// GROUP BY ⇒ lỗi 42803.
//
// ⇒ Test này gọi TẤT CẢ hàm báo cáo với bộ lọc rộng nhất để bắt lỗi SQL
// còn sót trong bất kỳ query nào.
func TestAllReportQueriesRunWithoutSQLError(t *testing.T) {
	store, err := NewStore(testConnStringReport())
	if err != nil {
		t.Skipf("không kết nối được DB test: %v", err)
	}

	// Bộ lọc rộng nhất: mọi landing, mọi khoảng thời gian, mọi traffic scope.
	f := ReportFilter{
		FromDate:     "2020-01-01",
		ToDate:       "2099-12-31",
		FromHour:     -1,
		ToHour:       -1,
		VisitorType:  "all",
		TrafficScope: "all",
	}

	type check struct {
		name string
		run  func() error
	}
	checks := []check{
		{"GetReportSummary", func() error {
			_, err := store.GetReportSummary(f, false)
			return err
		}},
		{"GetReportTimeSeries", func() error {
			_, err := store.GetReportTimeSeries(f, "day")
			return err
		}},
		{"GetReportLandingLeaderboard", func() error {
			_, err := store.GetReportLandingLeaderboard(f)
			return err
		}},
		{"GetReportMetaPlatform", func() error {
			_, err := store.GetReportMetaPlatform(f)
			return err
		}},
		{"GetReportMetaPlacement", func() error {
			_, err := store.GetReportMetaPlacement(f)
			return err
		}},
		{"GetReportInAppBrowser", func() error {
			_, err := store.GetReportInAppBrowser(f)
			return err
		}},
		{"GetReportAdHierarchy", func() error {
			_, err := store.GetReportAdHierarchy(f, 300)
			return err
		}},
		{"GetReportHourHeatmap", func() error {
			_, _, err := store.GetReportHourHeatmap(f, "leads")
			return err
		}},
		{"GetReportScrollFunnel", func() error {
			_, err := store.GetReportScrollFunnel(f)
			return err
		}},
		{"GetReportVisibleTimeDistribution", func() error {
			_, err := store.GetReportVisibleTimeDistribution(f)
			return err
		}},
		{"GetReportEngagement", func() error {
			_, err := store.GetReportEngagement(f)
			return err
		}},
		{"GetReportTopClickedElements", func() error {
			_, err := store.GetReportTopClickedElements(f, 30)
			return err
		}},
		{"GetReportFormFunnel", func() error {
			_, err := store.GetReportFormFunnel(f)
			return err
		}},
		{"GetReportAutofillSource", func() error {
			_, err := store.GetReportAutofillSource(f)
			return err
		}},
		{"GetReportDeviceBreakdown", func() error {
			_, err := store.GetReportDeviceBreakdown(f)
			return err
		}},
		{"GetReportDeviceModels", func() error {
			_, err := store.GetReportDeviceModels(f, 30)
			return err
		}},
		{"GetReportConnectionHardware", func() error {
			_, err := store.GetReportConnectionHardware(f)
			return err
		}},
		{"GetReportSecurity", func() error {
			_, err := store.GetReportSecurity(f)
			return err
		}},
		{"GetReportFilterOptions", func() error {
			_, err := store.GetReportFilterOptions()
			return err
		}},
		{"GetReportQuarantine", func() error {
			_, err := store.GetReportQuarantine("", 30, 0)
			return err
		}},
		{"GetAnalyticsTimeSeries(day)", func() error {
			_, err := store.GetAnalyticsTimeSeries(NewAnalyticsFilter("", "", "", "", 365), "day")
			return err
		}},
		{"GetAnalyticsTimeSeries(hour)", func() error {
			_, err := store.GetAnalyticsTimeSeries(NewAnalyticsFilter("", "", "", "", 365), "hour")
			return err
		}},
		{"GetAnalyticsTimeSeries(week)", func() error {
			_, err := store.GetAnalyticsTimeSeries(NewAnalyticsFilter("", "", "", "", 365), "week")
			return err
		}},
		{"GetDeviceAnalytics(model)", func() error {
			_, err := store.GetDeviceAnalytics(NewAnalyticsFilter("", "", "", "", 365), "model", 20)
			return err
		}},
		{"GetDeviceAnalytics(brand)", func() error {
			_, err := store.GetDeviceAnalytics(NewAnalyticsFilter("", "", "", "", 365), "brand", 20)
			return err
		}},
		{"GetDeviceAnalytics(browser)", func() error {
			_, err := store.GetDeviceAnalytics(NewAnalyticsFilter("", "", "", "", 365), "browser", 20)
			return err
		}},
		{"GetDeviceAnalytics(os)", func() error {
			_, err := store.GetDeviceAnalytics(NewAnalyticsFilter("", "", "", "", 365), "os", 20)
			return err
		}},
		{"GetDeviceAnalytics(inapp)", func() error {
			_, err := store.GetDeviceAnalytics(NewAnalyticsFilter("", "", "", "", 365), "inapp", 20)
			return err
		}},
		{"GetDeviceAnalytics(viewport)", func() error {
			_, err := store.GetDeviceAnalytics(NewAnalyticsFilter("", "", "", "", 365), "viewport", 20)
			return err
		}},
		{"GetDeviceAnalytics(resolution)", func() error {
			_, err := store.GetDeviceAnalytics(NewAnalyticsFilter("", "", "", "", 365), "resolution", 20)
			return err
		}},
		{"GetMetaMacroAnalytics", func() error {
			_, err := store.GetMetaMacroAnalytics(NewAnalyticsFilter("", "", "", "", 365), 50)
			return err
		}},
		{"GetBotAnalytics", func() error {
			_, err := store.GetBotAnalytics(NewAnalyticsFilter("", "", "", "", 365), 20)
			return err
		}},
		{"GetVPNAnalytics", func() error {
			_, err := store.GetVPNAnalytics(NewAnalyticsFilter("", "", "", "", 365))
			return err
		}},
		{"GetLandingReportRange", func() error {
			_, err := store.GetLandingReportRange("", "", "", "", 30, -1, -1)
			return err
		}},
		{"GetOverviewExtended", func() error {
			_, err := store.GetOverviewExtended("", "", "", "", -1, -1)
			return err
		}},
		{"GetAnalyticsOverview", func() error {
			_, err := store.GetAnalyticsOverview("", "")
			return err
		}},
		{"GetLandingPerformance", func() error {
			_, err := store.GetLandingPerformance("", "", "", "", -1, -1)
			return err
		}},
		{"GetSourcePerformance", func() error {
			_, err := store.GetSourcePerformance("", "", "", "", -1, -1)
			return err
		}},
		{"GetGeoAnalytics", func() error {
			_, err := store.GetGeoAnalytics("", "", "", "", -1, -1)
			return err
		}},
		{"GetAdPlacementAnalytics", func() error {
			_, err := store.GetAdPlacementAnalytics("", "", "", "", -1, -1)
			return err
		}},
		{"GetSecurityStats", func() error {
			_, err := store.GetSecurityStats()
			return err
		}},
	}

	for _, c := range checks {
		if err := c.run(); err != nil {
			t.Errorf("%s lỗi SQL: %v", c.name, err)
		}
	}
}
