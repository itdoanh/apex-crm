package googlesheets

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"apex-backend/models"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

const DefaultBotEmail = "sheet-bot@rinva-drive-tool.iam.gserviceaccount.com"

var (
	// Regex nhận diện Spreadsheet ID từ bất kỳ URL Google Sheet nào user dán vào
	spreadsheetIDRegex = regexp.MustCompile(`/spreadsheets/(?:u/\d+/)?d/([a-zA-Z0-9-_]+)`)
)

// ResolveCredentials trả về chuỗi JSON Service Account: nếu người dùng không truyền thì tự động lấy từ file máy chủ
func ResolveCredentials(credentialsJSON string) (string, error) {
	trimmed := strings.TrimSpace(credentialsJSON)
	if trimmed != "" {
		return trimmed, nil
	}

	// Tìm file service-account.json trên máy chủ
	candidates := []string{
		os.Getenv("GOOGLE_SERVICE_ACCOUNT_FILE"),
		"service-account.json",
		filepath.Join("backend", "service-account.json"),
		filepath.Join("..", "service-account.json"),
		filepath.Join("config", "service-account.json"),
		filepath.Join("backend", "config", "service-account.json"),
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "service-account.json"))
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "..", "service-account.json"))
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "backend", "service-account.json"))
	}

	for _, path := range candidates {
		if path == "" {
			continue
		}
		if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
			return string(data), nil
		}
	}

	// Kiểm tra biến môi trường GOOGLE_SERVICE_ACCOUNT_JSON
	if envJSON := os.Getenv("GOOGLE_SERVICE_ACCOUNT_JSON"); strings.TrimSpace(envJSON) != "" {
		return envJSON, nil
	}

	return "", fmt.Errorf("hệ thống chưa cấu hình Service Account Key. Vui lòng đặt file service-account.json vào thư mục dự án hoặc dán key")
}

// ExtractSpreadsheetID bóc tách ID của Google Sheet từ URL hoặc chuỗi ID
func ExtractSpreadsheetID(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}
	matches := spreadsheetIDRegex.FindStringSubmatch(input)
	if len(matches) > 1 {
		return matches[1]
	}
	// Nếu người dùng nhập trực tiếp ID
	clean := strings.Trim(input, "/")
	if len(clean) >= 20 && !strings.Contains(clean, " ") && !strings.Contains(clean, "http") {
		return clean
	}
	return input
}

// GetServiceAccountEmail trích xuất email của Service Account từ chuỗi JSON key
func GetServiceAccountEmail(credentialsJSON string) string {
	var parsed struct {
		ClientEmail string `json:"client_email"`
		Type        string `json:"type"`
	}
	if err := json.Unmarshal([]byte(credentialsJSON), &parsed); err == nil && parsed.ClientEmail != "" {
		return parsed.ClientEmail
	}
	return DefaultBotEmail
}

// GetSpreadsheetMetadata kết nối tới Google Sheets API lấy thông tin tên bảng tính và danh sách các Tab
func GetSpreadsheetMetadata(ctx context.Context, credentialsJSON string, rawSpreadsheetID string) (*models.SheetMetadataResponse, error) {
	resolvedCreds, err := ResolveCredentials(credentialsJSON)
	if err != nil {
		return nil, err
	}
	credentialsJSON = resolvedCreds

	sheetID := ExtractSpreadsheetID(rawSpreadsheetID)
	if sheetID == "" {
		return nil, fmt.Errorf("Spreadsheet ID hoặc URL không hợp lệ")
	}

	srv, err := sheets.NewService(ctx, option.WithCredentialsJSON([]byte(credentialsJSON)))
	if err != nil {
		return nil, fmt.Errorf("không thể khởi tạo kết nối Google Service: %w", err)
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	resp, err := srv.Spreadsheets.Get(sheetID).Context(ctxTimeout).Do()
	if err != nil {
		email := GetServiceAccountEmail(credentialsJSON)
		if strings.Contains(err.Error(), "404") {
			return nil, fmt.Errorf("không tìm thấy Google Sheet (404). Hãy kiểm tra lại link Google Sheet hoặc chắc chắn bạn đã bấm nút 'Chia sẻ (Share)' và cấp quyền 'Người chỉnh sửa (Editor)' cho email bot: %s", email)
		}
		if strings.Contains(err.Error(), "403") {
			return nil, fmt.Errorf("quyền truy cập bị từ chối (403 Permission Denied). Vui lòng mở Google Sheet -> Bấm nút 'Chia sẻ (Share)' ở góc phải trên -> Dán email bot và chọn quyền 'Người chỉnh sửa (Editor)': %s", email)
		}
		return nil, fmt.Errorf("lỗi kết nối Google Sheets API: %w", err)
	}

	tabs := make([]models.SheetTabInfo, 0, len(resp.Sheets))
	for _, s := range resp.Sheets {
		if s.Properties == nil {
			continue
		}
		var rowCount, colCount int64
		if s.Properties.GridProperties != nil {
			rowCount = s.Properties.GridProperties.RowCount
			colCount = s.Properties.GridProperties.ColumnCount
		}
		tabs = append(tabs, models.SheetTabInfo{
			ID:          s.Properties.SheetId,
			Title:       s.Properties.Title,
			RowCount:    rowCount,
			ColumnCount: colCount,
		})
	}

	return &models.SheetMetadataResponse{
		SpreadsheetID:  sheetID,
		Title:          resp.Properties.Title,
		SpreadsheetURL: fmt.Sprintf("https://docs.google.com/spreadsheets/d/%s/edit", sheetID),
		Tabs:           tabs,
		ServiceAccount: GetServiceAccountEmail(credentialsJSON),
	}, nil
}

// GetSheetHeaders đọc dòng đầu tiên (Row 1) của Tab để lấy danh sách tên các cột tiêu đề
func GetSheetHeaders(ctx context.Context, credentialsJSON string, rawSpreadsheetID string, tabName string) ([]string, error) {
	resolvedCreds, err := ResolveCredentials(credentialsJSON)
	if err != nil {
		return nil, err
	}
	credentialsJSON = resolvedCreds

	sheetID := ExtractSpreadsheetID(rawSpreadsheetID)
	srv, err := sheets.NewService(ctx, option.WithCredentialsJSON([]byte(credentialsJSON)))
	if err != nil {
		return nil, fmt.Errorf("không thể khởi tạo Google Sheets Client: %w", err)
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	readRange := fmt.Sprintf("'%s'!1:1", tabName)
	resp, err := srv.Spreadsheets.Values.Get(sheetID, readRange).Context(ctxTimeout).Do()
	if err != nil {
		return nil, fmt.Errorf("lỗi đọc dòng tiêu đề: %w", err)
	}

	headers := make([]string, 0)
	if len(resp.Values) > 0 {
		for _, cell := range resp.Values[0] {
			headers = append(headers, fmt.Sprintf("%v", cell))
		}
	}
	return headers, nil
}

// WriteHeaders ghi dòng tiêu đề chuẩn vào dòng 1 của trang tính nếu trang tính còn trống
func WriteHeaders(ctx context.Context, credentialsJSON string, rawSpreadsheetID string, tabName string, headers []string) error {
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

	ctxTimeout, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	rowValues := make([]interface{}, len(headers))
	for i, h := range headers {
		rowValues[i] = h
	}

	vr := &sheets.ValueRange{
		Values: [][]interface{}{rowValues},
	}

	writeRange := fmt.Sprintf("'%s'!A1", tabName)
	_, err = srv.Spreadsheets.Values.Update(sheetID, writeRange, vr).
		ValueInputOption("USER_ENTERED").
		Context(ctxTimeout).
		Do()
	return err
}

// AppendRow thêm một dòng dữ liệu mới vào cuối bảng tính của Google Sheet
func AppendRow(ctx context.Context, credentialsJSON string, rawSpreadsheetID string, tabName string, rowValues []interface{}) (string, error) {
	resolvedCreds, err := ResolveCredentials(credentialsJSON)
	if err != nil {
		return "", err
	}
	credentialsJSON = resolvedCreds

	sheetID := ExtractSpreadsheetID(rawSpreadsheetID)
	srv, err := sheets.NewService(ctx, option.WithCredentialsJSON([]byte(credentialsJSON)))
	if err != nil {
		return "", fmt.Errorf("không thể khởi tạo Google Sheets Client: %w", err)
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	vr := &sheets.ValueRange{
		Values: [][]interface{}{rowValues},
	}

	appendRange := fmt.Sprintf("'%s'!A1", tabName)
	resp, err := srv.Spreadsheets.Values.Append(sheetID, appendRange, vr).
		ValueInputOption("USER_ENTERED").
		InsertDataOption("INSERT_ROWS").
		Context(ctxTimeout).
		Do()
	if err != nil {
		return "", fmt.Errorf("lỗi ghi dữ liệu vào Google Sheet: %w", err)
	}

	updatedRange := ""
	if resp.Updates != nil {
		updatedRange = resp.Updates.UpdatedRange
	}
	return updatedRange, nil
}

// ListDriveSpreadsheets duyệt toàn bộ các Google Sheet mà tài khoản Google Project có quyền truy cập
func ListDriveSpreadsheets(ctx context.Context, credentialsJSON string) ([]models.DriveFileInfo, error) {
	resolvedCreds, err := ResolveCredentials(credentialsJSON)
	if err != nil {
		return nil, err
	}
	credentialsJSON = resolvedCreds

	driveSrv, err := drive.NewService(ctx, option.WithCredentialsJSON([]byte(credentialsJSON)))
	if err != nil {
		return nil, fmt.Errorf("không thể khởi tạo Google Drive Client: %w", err)
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	// Tìm các file là Google Spreadsheet
	query := "mimeType='application/vnd.google-apps.spreadsheet' and trashed=false"
	r, err := driveSrv.Files.List().
		Q(query).
		Fields("files(id, name, webViewLink)").
		PageSize(50).
		Context(ctxTimeout).
		Do()
	if err != nil {
		return nil, fmt.Errorf("lỗi duyệt danh sách Google Drive: %w", err)
	}

	files := make([]models.DriveFileInfo, 0, len(r.Files))
	for _, f := range r.Files {
		files = append(files, models.DriveFileInfo{
			ID:          f.Id,
			Name:        f.Name,
			WebViewLink: f.WebViewLink,
		})
	}
	return files, nil
}
