package googlesheets

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"apex-backend/models"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// ─── Cập nhật dòng đã có trong Sheet khi CRM đổi trạng thái ─────────────
//
// 📌 BỐI CẢNH (2026-11-15):
//
// Trước đây Google Sheets chỉ nhận 1 DÒNG khi lead MỚI tạo. Sau đó sales
// đổi trạng thái trong CRM (`moi` → `da_goi` → `da_hoc_buoi_chia_se` → …)
// thì Sheet KHÔNG cập nhật ⇒ cột "Trạng Thái" kẹt vĩnh viễn ở `moi`.
//
// ⇒ Bổ sung cơ chế: khi đổi trạng thái, backend tìm dòng đã ghi (qua
// `google_sheet_sync_logs.row_range`) và cập nhật ĐÚNG các ô tương ứng
// (trạng thái, nhãn, giai đoạn, sự kiện Meta, ghi chú, người phụ trách).
//
// ⚠️ KHÔNG ghi lại dòng MỚI (append) — nếu append sẽ sinh dòng trùng.

// rowRangeRegex bắt "Tab!A123:BS123" hoặc "A123:BS123" ⇒ lấy số dòng 123.
var rowRangeRegex = regexp.MustCompile(`(?i)(?:^|!)([A-Z]{1,3})(\d+)(?::([A-Z]{1,3})(\d+))?$`)

// ParseRowNumber trích số dòng (row) từ row_range của Sheets API.
//
// Trả về (0, false) nếu không parse được.
func ParseRowNumber(rowRange string) (int, bool) {
	rowRange = strings.TrimSpace(rowRange)
	if rowRange == "" {
		return 0, false
	}
	m := rowRangeRegex.FindStringSubmatch(rowRange)
	if len(m) == 0 {
		return 0, false
	}
	n, err := strconv.Atoi(m[2])
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// sheetStatusFields là các trường trong dòng Sheet được cập nhật khi CRM
// đổi trạng thái / ghi chú.
//
// ⚠️ Đây là danh sách CỐ ĐỊNH — thêm trường mới vào đây thì phải cập nhật
// `buildStatusRowValues` cho khớp.
var sheetStatusFields = []string{
	"trang_thai",
	"trang_thai_label",
	"trang_thai_stage",
	"capi_event",
	"capi_has_value",
	"conversion_value",
	"lead_score",
	"ghi_chu",
	"nguoi_phu_trach",
	"cap_nhat_luc",
}

// StatusColumnIndexes trả về map {field: chỉ số cột (0-based)} cho các trường
// trạng thái trong 1 cấu hình ánh xạ.
//
// Trả về nil nếu cấu hình KHÔNG map trường trạng thái nào ⇒ không cần cập nhật.
func StatusColumnIndexes(mappings []models.FieldMapping) map[string]int {
	out := make(map[string]int)
	for i, m := range mappings {
		for _, f := range sheetStatusFields {
			if m.Field == f {
				out[f] = i
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// BuildStatusRowValues dựng giá trị cho các ô trạng thái theo chỉ số cột.
//
// Trả về map {chỉ số cột: giá trị}.
//
// ⚠️ Ô nào có giá trị RỖNG (VD: ghi chú bị xoá) sẽ BỎ QUA — không ghi đè
// bằng chuỗi rỗng (đồng nghĩa xoá sạch dữ liệu người dùng nhập tay).
func BuildStatusRowValues(mappings []models.FieldMapping, lead *models.Lead, trackingData map[string]interface{}) map[int]interface{} {
	idx := StatusColumnIndexes(mappings)
	if len(idx) == 0 {
		return nil
	}
	out := make(map[int]interface{}, len(idx))
	for field, col := range idx {
		v := ExtractLeadValue(field, "", lead, trackingData)
		if s, ok := v.(string); ok && s == "" {
			// Không ghi đè giá trị cũ bằng chuỗi rỗng (VD: ghi chú bị xoá).
			continue
		}
		out[col] = v
	}
	return out
}

// UpdateRowCells cập nhật các ô cụ thể của 1 dòng trong Sheet.
//
// `cells` là map {chỉ số cột (0-based): giá trị}.
//
// ⚠️ Dùng `Values.BatchUpdate` (nhiều range) để chỉ ghi đúng các ô cần đổi,
// không ghi lại cả dòng ⇒ tránh mất dữ liệu do người dùng sửa tay.
func UpdateRowCells(ctx context.Context, credentialsJSON string, rawSpreadsheetID string, tabName string, rowNumber int, cells map[int]interface{}) error {
	if len(cells) == 0 || rowNumber <= 0 {
		return nil
	}

	resolvedCreds, err := ResolveCredentials(credentialsJSON)
	if err != nil {
		return err
	}
	credentialsJSON = resolvedCreds

	sheetID := ExtractSpreadsheetID(rawSpreadsheetID)
	srv, err := sheets.NewService(ctx, option.WithCredentialsJSON([]byte(credentialsJSON)))
	if err != nil {
		return fmt.Errorf("không thể khởi tạo Google Sheets Client: %w", err)
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	valueRanges := make([]*sheets.ValueRange, 0, len(cells))
	for colIdx, val := range cells {
		col := columnLetter(colIdx)
		valueRanges = append(valueRanges, &sheets.ValueRange{
			Range: fmt.Sprintf("'%s'!%s%d:%s%d", tabName, col, rowNumber, col, rowNumber),
			Values: [][]interface{}{{val}},
		})
	}

	// ⚠️ KHÔNG dùng `ValueInputOption("USER_ENTERED")` — giữ `RAW` (mặc định).
	//
	// Lý do: `USER_ENTERED` khiến Sheets DIỄN GIẢI nội dung ô như công thức
	// (=...) ⇒ rủi ro formula injection nếu ghi chú sales chứa ký tự "=".
	// Các ô trạng thái đều là TEXT ⇒ `RAW` là lựa chọn đúng và an toàn.
	_, err = srv.Spreadsheets.Values.BatchUpdate(sheetID, &sheets.BatchUpdateValuesRequest{
		Data: valueRanges,
	}).Context(ctxTimeout).Do()
	if err != nil {
		return fmt.Errorf("lỗi cập nhật dòng %d trong Google Sheet: %w", rowNumber, err)
	}
	return nil
}
