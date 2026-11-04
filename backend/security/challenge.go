package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// defaultChallengeSecretKey là giá trị dự phòng khi CHALLENGE_SECRET chưa được cấu hình.
// Nên đặt CHALLENGE_SECRET trong .env để có thể rotate secret mà không cần sửa code.
const defaultChallengeSecretKey = "apex_fintech_security_secret_2026_salt_xyz99"

var (
	// Server secret key dùng để ký HMAC cho các challenge token
	challengeSecretKey = resolveChallengeSecretKey()

	// Bộ nhớ cache lưu các nonce đã qua sử dụng để chống Replay Attack
	usedNonces   = sync.Map{}
	nonceCleaner sync.Once
)

// resolveChallengeSecretKey lấy secret từ biến môi trường CHALLENGE_SECRET,
// fallback về giá trị mặc định nếu không được cấu hình.
func resolveChallengeSecretKey() []byte {
	if s := strings.TrimSpace(os.Getenv("CHALLENGE_SECRET")); s != "" {
		return []byte(s)
	}
	return []byte(defaultChallengeSecretKey)
}

// ChallengeResponse đại diện cho thử thách mật mã gửi cho client
type ChallengeResponse struct {
	ChallengeID string `json:"challenge_id"`
	Salt        string `json:"salt"`
	Difficulty  int    `json:"difficulty"`
	ExpiresAt   int64  `json:"expires_at"`
	Signature   string `json:"signature"`
}

// GenerateChallenge tạo một thử thách Proof-of-Work ngầm cho Front-End
func GenerateChallenge(difficulty int) (*ChallengeResponse, error) {
	if difficulty <= 0 {
		difficulty = 3 // Độ khó mặc định: SHA-256 bắt đầu bằng "000" (client mất ~15-30ms)
	}

	rawID := make([]byte, 16)
	if _, err := rand.Read(rawID); err != nil {
		return nil, err
	}
	challengeID := "ch_" + hex.EncodeToString(rawID)

	rawSalt := make([]byte, 16)
	if _, err := rand.Read(rawSalt); err != nil {
		return nil, err
	}
	salt := hex.EncodeToString(rawSalt)

	expiresAt := time.Now().Add(5 * time.Minute).Unix()

	// Ký HMAC để đảm bảo challenge không bị client sửa đổi thông số
	message := fmt.Sprintf("%s:%s:%d:%d", challengeID, salt, difficulty, expiresAt)
	h := hmac.New(sha256.New, challengeSecretKey)
	h.Write([]byte(message))
	signature := hex.EncodeToString(h.Sum(nil))

	startNonceCleaner()

	return &ChallengeResponse{
		ChallengeID: challengeID,
		Salt:        salt,
		Difficulty:  difficulty,
		ExpiresAt:   expiresAt,
		Signature:   signature,
	}, nil
}

// VerifyChallenge kiểm tra lời giải Proof-of-Work và chống Replay Attack
func VerifyChallenge(challengeID, salt string, difficulty int, expiresAt int64, signature, solution string) (bool, string) {
	if challengeID == "" || salt == "" || signature == "" || solution == "" {
		return false, "Thiếu tham số xác thực Proof-of-Work"
	}

	// 1. Kiểm tra thời hạn hiệu lực
	if time.Now().Unix() > expiresAt {
		return false, "Thử thách xác thực đã hết hạn"
	}

	// 2. Kiểm tra tính toàn vẹn chữ ký HMAC
	message := fmt.Sprintf("%s:%s:%d:%d", challengeID, salt, difficulty, expiresAt)
	h := hmac.New(sha256.New, challengeSecretKey)
	h.Write([]byte(message))
	expectedSig := hex.EncodeToString(h.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		return false, "Chữ ký thử thách không hợp lệ (Signature mismatch)"
	}

	// 3. Chống Replay Attack: kiểm tra xem solution/nonce này đã từng sử dụng chưa
	nonceKey := fmt.Sprintf("%s:%s", challengeID, solution)
	if _, loaded := usedNonces.LoadOrStore(nonceKey, time.Now().Add(10*time.Minute)); loaded {
		return false, "Thử thách này đã được sử dụng (Replay attack detected)"
	}

	// 4. Kiểm tra bài toán hash SHA-256
	// Payload để băm: challengeID + ":" + salt + ":" + solution
	dataToHash := fmt.Sprintf("%s:%s:%s", challengeID, salt, solution)
	hashBytes := sha256.Sum256([]byte(dataToHash))
	hashHex := hex.EncodeToString(hashBytes[:])

	prefix := strings.Repeat("0", difficulty)
	if !strings.HasPrefix(hashHex, prefix) {
		return false, fmt.Sprintf("Lời giải không chính xác (Hash: %s, cần bắt đầu bằng %s)", hashHex[:8], prefix)
	}

	return true, ""
}

// startNonceCleaner dọn dẹp các nonce cũ trong bộ nhớ
func startNonceCleaner() {
	nonceCleaner.Do(func() {
		go func() {
			ticker := time.NewTicker(5 * time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				now := time.Now()
				usedNonces.Range(func(key, val interface{}) bool {
					if expireTime, ok := val.(time.Time); ok && now.After(expireTime) {
						usedNonces.Delete(key)
					}
					return true
				})
			}
		}()
	})
}

// ParseDifficulty chuyển đổi chuỗi sang int
func ParseDifficulty(str string) int {
	d, err := strconv.Atoi(str)
	if err != nil || d <= 0 || d > 6 {
		return 3
	}
	return d
}
