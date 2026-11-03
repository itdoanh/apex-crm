package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Cấu hình chuẩn khuyến nghị theo NIST / OWASP cho Argon2id
type Argon2Params struct {
	Memory      uint32 // 64 MB = 65536 KiB
	Iterations  uint32 // 3 vòng lặp
	Parallelism uint8  // 2 luồng
	SaltLength  uint32 // 16 bytes ngẫu nhiên
	KeyLength   uint32 // 32 bytes kết quả hash
}

var DefaultArgon2Params = &Argon2Params{
	Memory:      64 * 1024, // 64 MB
	Iterations:  3,
	Parallelism: 2,
	SaltLength:  16,
	KeyLength:   32,
}

// HashPassword băm mật khẩu bằng thuật toán Argon2id chuẩn OWASP
// Định dạng chuỗi kết quả: $argon2id$v=19$m=65536,t=3,p=2$<salt-base64>$<hash-base64>
func HashPassword(password string) (string, error) {
	if len(password) == 0 {
		return "", errors.New("mật khẩu không được để trống")
	}

	p := DefaultArgon2Params

	// Tạo muối ngẫu nhiên bằng CSPRNG
	salt := make([]byte, p.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("lỗi sinh salt ngẫu nhiên: %w", err)
	}

	// Băm mật khẩu bằng Argon2id
	hash := argon2.IDKey([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLength)

	// Mã hóa Base64 Raw (không padding)
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	// Format chuẩn PHC
	encoded := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.Memory, p.Iterations, p.Parallelism, b64Salt, b64Hash)

	return encoded, nil
}

// VerifyPassword kiểm tra mật khẩu người dùng nhập vào với chuỗi băm Argon2id đã lưu
func VerifyPassword(password, encodedHash string) (bool, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return false, errors.New("chuỗi hash không đúng định dạng chuẩn Argon2id")
	}

	if parts[1] != "argon2id" {
		return false, errors.New("thuật toán hash không phải là argon2id")
	}

	var version int
	_, err := fmt.Sscanf(parts[2], "v=%d", &version)
	if err != nil || version != argon2.Version {
		return false, errors.New("phiên bản Argon2 không tương thích")
	}

	p := &Argon2Params{}
	_, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Iterations, &p.Parallelism)
	if err != nil {
		return false, errors.New("không thể phân tích tham số bộ nhớ/vòng lặp của hash")
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, errors.New("lỗi giải mã salt")
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, errors.New("lỗi giải mã hash mong đợi")
	}
	p.KeyLength = uint32(len(expectedHash))

	// Tính toán lại hash với mật khẩu cần kiểm tra
	computedHash := argon2.IDKey([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLength)

	// So sánh thời gian hằng số (Constant Time Compare) chống tấn công Timing Attack
	if subtle.ConstantTimeCompare(expectedHash, computedHash) == 1 {
		return true, nil
	}

	return false, nil
}

// GenerateSessionToken sinh chuỗi token bảo mật 256-bit ngẫu nhiên và mã băm SHA-256
// Token thô (rawToken) được trả về cho client lưu cookie
// Token hash (SHA-256) được lưu trong CSDL để chống lộ dữ liệu nếu DB bị dump
func GenerateSessionToken() (rawToken string, tokenHash string, err error) {
	bytes := make([]byte, 32) // 256-bit entropy
	if _, err := rand.Read(bytes); err != nil {
		return "", "", fmt.Errorf("lỗi sinh session token: %w", err)
	}

	rawToken = "apex_sess_" + hex.EncodeToString(bytes)
	tokenHash = HashSessionToken(rawToken)
	return rawToken, tokenHash, nil
}

// HashSessionToken băm token bằng SHA-256
func HashSessionToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// ParseUserAgent phân tích thiết bị, trình duyệt và hệ điều hành từ User-Agent
func ParseUserAgent(ua string) (deviceType, browser, os string) {
	if ua == "" {
		return "desktop", "Chưa rõ", "Chưa rõ"
	}
	lower := strings.ToLower(ua)

	// Phân loại Thiết bị
	if strings.Contains(lower, "ipad") || strings.Contains(lower, "tablet") {
		deviceType = "tablet"
	} else if strings.Contains(lower, "mobile") || strings.Contains(lower, "iphone") || strings.Contains(lower, "android") {
		deviceType = "mobile"
	} else {
		deviceType = "desktop"
	}

	// Phân loại Hệ điều hành
	if strings.Contains(lower, "windows nt 10.0") || strings.Contains(lower, "windows 11") {
		os = "Windows 10/11"
	} else if strings.Contains(lower, "windows") {
		os = "Windows"
	} else if strings.Contains(lower, "iphone") || strings.Contains(lower, "ipad") {
		os = "iOS"
	} else if strings.Contains(lower, "android") {
		os = "Android"
	} else if strings.Contains(lower, "macintosh") || strings.Contains(lower, "mac os") {
		os = "macOS"
	} else if strings.Contains(lower, "linux") {
		os = "Linux"
	} else {
		os = "Khác"
	}

	// Phân loại Trình duyệt
	if strings.Contains(lower, "edg/") {
		browser = "Microsoft Edge"
	} else if strings.Contains(lower, "chrome/") && !strings.Contains(lower, "edg/") {
		browser = "Google Chrome"
	} else if strings.Contains(lower, "safari/") && !strings.Contains(lower, "chrome/") {
		browser = "Apple Safari"
	} else if strings.Contains(lower, "firefox/") {
		browser = "Mozilla Firefox"
	} else if strings.Contains(lower, "coccoc") {
		browser = "Cốc Cốc"
	} else if strings.Contains(lower, "opera") || strings.Contains(lower, "opr/") {
		browser = "Opera"
	} else {
		browser = "Trình duyệt khác"
	}

	return deviceType, browser, os
}
