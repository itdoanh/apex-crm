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

// Mã lý do xác thực Proof-of-Work.
// Dùng mã thay vì so khớp chuỗi để phân loại chính xác trong handler.
const (
	// PowCodeOK — PoW hợp lệ.
	PowCodeOK = "ok"
	// PowCodeMissingParams — thiếu tham số challenge.
	PowCodeMissingParams = "missing_params"
	// PowCodeExpired — challenge hết hạn.
	PowCodeExpired = "expired"
	// PowCodeBadSignature — chữ ký HMAC không hợp lệ.
	PowCodeBadSignature = "bad_signature"
	// PowCodeReplay — nonce đã dùng và KHÔNG khớp fingerprint phiên đăng ký
	// ⇒ nghi vấn bot chơi lại payload.
	PowCodeReplay = "replay"
	// PowCodeInvalidSolution — lời giải không thỏa điều kiện hash.
	PowCodeInvalidSolution = "invalid_solution"
	// PowCodeDuplicateSubmit — cùng phiên đăng ký gửi lại payload đã dùng
	// (nhiều handler submit chạy song song / retry mạng) ⇒ KHÔNG phải spam.
	PowCodeDuplicateSubmit = "duplicate_submit"
)

// duplicateSubmitWindow là cửa sổ (giây) mà trong đó cùng một fingerprint
// (session + visitor + SĐT + IP) gửi lại payload đã xác thực được coi là "gửi trùng do
// client" thay vì "replay attack".
//
// 60 giây: đủ để bắt hết các handler submit chạy song song (vài trăm ms) và các
// lần retry do mạng chập chờn, nhưng đủ ngắn để một bot replay payload cũ sau đó
// vẫn bị ghi vào vùng cách ly như bình thường.
const duplicateSubmitWindow = 60 * time.Second

var (
	// Server secret key dùng để ký HMAC cho các challenge token
	challengeSecretKey = resolveChallengeSecretKey()

	// usedNonces lưu các nonce đã qua sử dụng để chống Replay Attack
	usedNonces = sync.Map{}

	// consumedChallenges lưu fingerprint của các challenge đã được xác thực
	// HỢP LỆ, kèm thời điểm. Dùng để nhận diện chính xác request trùng lặp
	// do client gửi (cùng phiên đăng ký) so với bot replay payload.
	consumedChallenges = sync.Map{}

	// activeChallenges đánh dấu challenge đang được xử lý (request gốc).
	// Xử lý race khi 2 request CÙNG LÚC tới với cùng challenge.
	activeChallenges = sync.Map{}

	nonceCleaner sync.Once
)

// PowResult là kết quả xác thực Proof-of-Work.
type PowResult struct {
	OK     bool   `json:"ok"`
	Code   string `json:"code"`   // Mã lý do (PowCode*)
	Reason string `json:"reason"` // Mô tả tiếng Việt để hiển thị/log
}

// consumedChallenge lưu thông tin challenge đã xác thực hợp lệ.
type consumedChallenge struct {
	fingerprint string
	at          time.Time
}

// nonceRecord lưu thông tin nonce đã sử dụng kèm fingerprint của phiên đăng ký
// và thời điểm sử dụng. Dùng để phân biệt "gửi trùng do client" với "replay attack".
type nonceRecord struct {
	fingerprint string
	at          time.Time
}

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

// VerifyChallenge kiểm tra lời giải Proof-of-Work và chống Replay Attack.
//
// fingerprint là định danh phiên đăng ký của client (thường là
// session_id + số điện thoại). Nhờ đó server phân biệt được:
//   - PowCodeOK              : PoW hợp lệ.
//   - PowCodeDuplicateSubmit : CÙNG phiên đăng ký gửi lại payload đã dùng
//     (nhiều handler submit chạy song song, retry mạng, keepalive + sendBeacon)
//     ⇒ KHÔNG phải spam ⇒ handler sẽ bỏ qua, không ghi vào vùng cách ly.
//   - PowCodeReplay          : nonce đã dùng nhưng fingerprint KHÁC
//     ⇒ nghi vấn bot chơi lại payload ⇒ ghi vào vùng cách ly.
//
// BỐI CẢNH (sửa bug 2026-11-05): lead thật từ khách Facebook Audience Network
// bị hiển thị sai là "Lỗi Proof-of-Work" trong tab Bảo mật & Bot, trong khi
// lead vẫn về CRM và Google Sheets. Nguyên nhân: client gửi CÙNG payload 3 lần
// (capture listener + onsubmit + tracker interceptor), các request sau bị ghi
// vào spam_quarantine với lý do "Replay attack detected".
func VerifyChallenge(challengeID, salt string, difficulty int, expiresAt int64, signature, solution, fingerprint string) PowResult {
	if challengeID == "" || salt == "" || signature == "" || solution == "" {
		return PowResult{OK: false, Code: PowCodeMissingParams, Reason: "Thiếu tham số xác thực Proof-of-Work"}
	}

	// 1. Kiểm tra thời hạn hiệu lực
	if time.Now().Unix() > expiresAt {
		return PowResult{OK: false, Code: PowCodeExpired, Reason: "Thử thách xác thực đã hết hạn"}
	}

	// 2. Kiểm tra tính toàn vẹn chữ ký HMAC
	message := fmt.Sprintf("%s:%s:%d:%d", challengeID, salt, difficulty, expiresAt)
	h := hmac.New(sha256.New, challengeSecretKey)
	h.Write([]byte(message))
	expectedSig := hex.EncodeToString(h.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		return PowResult{OK: false, Code: PowCodeBadSignature, Reason: "Chữ ký thử thách không hợp lệ (Signature mismatch)"}
	}

	// 3. Kiểm tra bài toán hash SHA-256
	// Payload để băm: challengeID + ":" + salt + ":" + solution
	//
	// ⚠️ THỨ TỰ QUAN TRỌNG: kiểm tra hash TRƯỚC khi đánh dấu nonce đã dùng.
	// Như vậy lời giải sai không bao giờ ghi vào `usedNonces` ⇒ lần gửi hợp lệ
	// sau đó vẫn dùng được (không cần xóa thủ công, tránh race với request
	// song song đang xử lý cùng challenge).
	dataToHash := fmt.Sprintf("%s:%s:%s", challengeID, salt, solution)
	hashBytes := sha256.Sum256([]byte(dataToHash))
	hashHex := hex.EncodeToString(hashBytes[:])

	prefix := strings.Repeat("0", difficulty)
	if !strings.HasPrefix(hashHex, prefix) {
		return PowResult{OK: false, Code: PowCodeInvalidSolution, Reason: fmt.Sprintf("Lời giải không chính xác (Hash: %s, cần bắt đầu bằng %s)", hashHex[:8], prefix)}
	}

	// 4. Chống Replay Attack: đánh dấu nonce đã dùng (chỉ sau khi lời giải hợp lệ)
	//
	// ⚠️ `LoadOrStore` là NGUYÊN TỬ: fingerprint của request đầu tiên được ghi
	// kèm nonce ngay trong cùng thao tác. Nhờ vậy request trùng lặp đến SONG SONG
	// (vài micro giây) chắc chắn đọc được fingerprint ⇒ phân loại "gửi trùng do
	// client" chính xác, không phụ thuộc thứ tự ghi `consumedChallenges`.
	nonceKey := fmt.Sprintf("%s:%s", challengeID, solution)
	now := time.Now()
	if val, loaded := usedNonces.LoadOrStore(nonceKey, nonceRecord{
		fingerprint: fingerprint,
		at:          now,
	}); loaded {
		rec, ok := val.(nonceRecord)
		// Nonce đã dùng. Phân biệt gửi trùng của client với replay thật.
		//
		// Điều kiện "gửi trùng do client" (KHÔNG tính là spam):
		//   1) nonce được ghi bởi cùng fingerprint (cùng session + visitor + SĐT + IP)
		//      VÀ trong cửa sổ duplicateSubmitWindow, HOẶC
		//   2) challenge đang "active" (request gốc song song vẫn đang xử lý).
		if ok && rec.fingerprint == fingerprint && now.Sub(rec.at) <= duplicateSubmitWindow {
			return PowResult{
				OK:     false,
				Code:   PowCodeDuplicateSubmit,
				Reason: "Yêu cầu trùng lặp từ cùng phiên đăng ký (bỏ qua, không tính là spam)",
			}
		}
		if isDuplicateSubmit(challengeID, fingerprint) {
			return PowResult{
				OK:     false,
				Code:   PowCodeDuplicateSubmit,
				Reason: "Yêu cầu trùng lặp từ cùng phiên đăng ký (bỏ qua, không tính là spam)",
			}
		}
		return PowResult{OK: false, Code: PowCodeReplay, Reason: "Thử thách này đã được sử dụng (Replay attack detected)"}
	}

	// 5. Ghi nhận challenge đã xác thực hợp lệ kèm fingerprint của phiên đăng ký
	//
	// Ghi NGAY tại đây (không đợi tới cuối handler) để các request trùng lặp
	// đến sau — dù chỉ vài micro giây — đều nhận diện được là "gửi trùng do
	// client" thay vì "replay attack" ⇒ không ghi nhầm vào vùng cách ly.
	consumedChallenges.Store(challengeID, consumedChallenge{
		fingerprint: fingerprint,
		at:          time.Now(),
	})

	return PowResult{OK: true, Code: PowCodeOK, Reason: ""}
}

// isDuplicateSubmit kiểm tra request hiện tại có phải là bản gửi lại của
// CÙNG phiên đăng ký đã xác thực trước đó hay không.
//
// Trả về true khi:
//   - Challenge đang "active" (request gốc song song vẫn đang xử lý) VÀ cùng
//     fingerprint phiên đăng ký, HOẶC
//   - Challenge đã xác thực hợp lệ trước đó với CÙNG fingerprint, trong
//     cửa sổ duplicateSubmitWindow.
func isDuplicateSubmit(challengeID, fingerprint string) bool {
	val, ok := consumedChallenges.Load(challengeID)
	if !ok {
		return false
	}
	cc, ok := val.(consumedChallenge)
	if !ok {
		return false
	}
	if cc.fingerprint != fingerprint {
		return false
	}
	if time.Since(cc.at) <= duplicateSubmitWindow {
		return true
	}
	// Ngoài cửa sổ duplicate submit nhưng request gốc vẫn đang chạy ⇒ vẫn coi
	// là gửi trùng do client (không phải replay attack).
	if _, active := activeChallenges.Load(challengeID); active {
		return true
	}
	return false
}

// MarkChallengeActive đánh dấu challenge đang được xử lý.
// Được gọi NGAY SAU khi PoW hợp lệ, và giải phóng (ReleaseChallengeActive)
// sau khi handler đã lưu lead xong.
func MarkChallengeActive(challengeID string) {
	if challengeID == "" {
		return
	}
	activeChallenges.Store(challengeID, time.Now().Add(2*time.Minute))
}

// ReleaseChallengeActive giải phóng đánh dấu active sau khi xử lý xong.
func ReleaseChallengeActive(challengeID string) {
	if challengeID == "" {
		return
	}
	activeChallenges.Delete(challengeID)
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
					if rec, ok := val.(nonceRecord); ok && now.Sub(rec.at) > 10*time.Minute {
						usedNonces.Delete(key)
					}
					return true
				})
				// Dọn fingerprint đã cũ (ngoài cửa sổ duplicate submit).
				consumedChallenges.Range(func(key, val interface{}) bool {
					if cc, ok := val.(consumedChallenge); ok && now.Sub(cc.at) > duplicateSubmitWindow {
						consumedChallenges.Delete(key)
					}
					return true
				})
				// Dọn các challenge "active" quá hạn (phòng trường hợp handler bị kill
				// giữa chừng, không gọi ReleaseChallengeActive).
				activeChallenges.Range(func(key, val interface{}) bool {
					if expireTime, ok := val.(time.Time); ok && now.After(expireTime) {
						activeChallenges.Delete(key)
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
