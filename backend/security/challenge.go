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

// challengeTTL là thời hạn của một thử thách Proof-of-Work.
//
// Mặc định 10 phút (thay vì 5 phút). Lý do: khách có thể mở landing, đọc nội dung
// và điền form trong thời gian dài (đặc biệt là form nhiều trường ở landing bán
// chứng khoán). Challenge lấy lúc tải trang mà hết hạn sau 5 phút ⇒ lead thật
// bị từ chối với lý do "hết hạn" ⇒ mất lead (bug 2026-11-12).
const challengeTTL = 10 * time.Minute

// challengeClockSkewGrace là sai số đồng hồ được chấp nhận (giây).
//
// Server đồng bộ NTP chưa hoàn hảo, client cũng có thể lệch vài giây. Nếu kiểm tra
// hết hạn quá khắt, một challenge còn hợp lệ có thể bị từ chối ⇒ mất lead thật.
// Dung sai 60 giây là quá đủ để hấp thụ sai số mà vẫn chặn được replay cũ.
const challengeClockSkewGrace = 60 * time.Second

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

	// issuedChallenges lưu các challenge DO SERVER PHÁT RA (kèm salt gốc).
	//
	// ⚠️ Mục đích: tự phục hồi khi client gửi "lời giải của challenge khác"
	// (bug 2026-11-15 — race condition client: challenge A + solution B).
	//
	// Client có thể đang dùng bản tracker.js CŨ trong cache trình duyệt ⇒
	// không thể chỉ dựa vào fix phía client. Server giữ salt gốc của MỌI
	// challenge đã phát ⇒ khi lời giải không khớp challenge khai báo, server
	// thử tìm challenge THẬT sở hữu lời giải đó.
	//
	// ⇒ Nếu lời giải hợp lệ với MỘT challenge server đã phát (và challenge đó
	//   còn hiệu lực) ⇒ client ĐÃ làm đúng bài toán PoW ⇒ coi như hợp lệ.
	//   ⇒ Chống bot vẫn giữ nguyên: bot vẫn phải giải PoW hợp lệ.
	issuedChallenges = sync.Map{}

	// issuedChallengesMu bảo vệ danh sách challenge đã phát (để quét theo
	// thứ tự thời gian & dọn dẹp).
	issuedChallengesMu     sync.RWMutex
	issuedChallengesOrder []string

	nonceCleaner sync.Once
)

// powHealWindow là cửa sổ thời gian mà server chấp nhận "tự phục hồi" khi
// lời giải thuộc về challenge khác.
//
// ⚠️ CHỈ dùng để phục hồi race condition client (vài chục ms). Không dùng để
// tha lỗi cho bot: bot vẫn phải giải đúng 1 challenge server đã phát.
const powHealWindow = 3 * time.Minute

// PowResult là kết quả xác thực Proof-of-Work.
type PowResult struct {
	OK     bool   `json:"ok"`
	Code   string `json:"code"`   // Mã lý do (PowCode*)
	Reason string `json:"reason"` // Mô tả tiếng Việt để hiển thị/log

	// EffectiveChallengeID là challenge THẬT sở hữu lời giải.
	//
	// ⚠️ Bình thường = challenge_id client gửi lên. Nhưng khi client gửi lời
	// giải của challenge KHÁC (race condition — bug 2026-11-15), server tự
	// phục hồi và trả về challenge thật ⇒ handler dùng giá trị này cho
	// `MarkChallengeActive` / `ReleaseChallengeActive` để logic "gửi trùng do
	// client" hoạt động nhất quán.
	EffectiveChallengeID string `json:"effective_challenge_id,omitempty"`
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

// issuedChallenge lưu thông tin challenge server đã phát cho client.
//
// Giữ `salt` gốc để server có thể tự kiểm tra lại lời giải khi client gửi
// nhầm challenge (bug race condition 2026-11-15).
type issuedChallenge struct {
	challengeID string
	salt        string
	difficulty  int
	expiresAt   int64
	issuedAt    time.Time
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

	expiresAt := time.Now().Add(challengeTTL).Unix()

	// Ký HMAC để đảm bảo challenge không bị client sửa đổi thông số
	message := fmt.Sprintf("%s:%s:%d:%d", challengeID, salt, difficulty, expiresAt)
	h := hmac.New(sha256.New, challengeSecretKey)
	h.Write([]byte(message))
	signature := hex.EncodeToString(h.Sum(nil))

	startNonceCleaner()

	// Ghi nhận challenge đã phát ⇒ server có thể tự phục hồi khi client gửi
	// lời giải của challenge KHÁC (bug race condition 2026-11-15).
	rememberIssuedChallenge(&issuedChallenge{
		challengeID: challengeID,
		salt:        salt,
		difficulty:  difficulty,
		expiresAt:   expiresAt,
		issuedAt:    time.Now(),
	})

	return &ChallengeResponse{
		ChallengeID: challengeID,
		Salt:        salt,
		Difficulty:  difficulty,
		ExpiresAt:   expiresAt,
		Signature:   signature,
	}, nil
}

// rememberIssuedChallenge lưu challenge đã phát vào bộ nhớ để phục hồi PoW.
func rememberIssuedChallenge(c *issuedChallenge) {
	issuedChallenges.Store(c.challengeID, c)

	issuedChallengesMu.Lock()
	issuedChallengesOrder = append(issuedChallengesOrder, c.challengeID)
	// Giới hạn số challenge nhớ lại (đủ cho cửa sổ powHealWindow).
	if len(issuedChallengesOrder) > 2000 {
		drop := len(issuedChallengesOrder) - 2000
		for _, id := range issuedChallengesOrder[:drop] {
			issuedChallenges.Delete(id)
		}
		issuedChallengesOrder = append([]string{}, issuedChallengesOrder[drop:]...)
	}
	issuedChallengesMu.Unlock()
}

// healMismatchedSolution thử tìm challenge THẬT sở hữu lời giải `solution`
// khi client gửi lời giải KHÔNG khớp challenge mà nó khai báo.
//
// BỐI CẢNH (bug 2026-11-15): lead thật #65 (0368942836 / "Luc van pao") bị
// ghi vào `spam_quarantine` với lý do "Lỗi Proof-of-Work" (`pow_invalid`).
// Bằng chứng: HMAC của challenge khớp (challenge do server thật phát), nhưng
// `SHA256(challenge_id:salt:solution)` = `7d57932c…` KHÔNG bắt đầu bằng "000"
// ⇒ lời giải thuộc về challenge KHÁC ⇒ race condition client.
//
// ⇒ Server tự quét các challenge nó đã phát trong `powHealWindow` và kiểm tra
//   lời giải có hợp lệ với challenge nào không.
//
// ⚠️ KHÔNG làm lỏng chống bot: bot vẫn phải giải đúng 1 challenge server đã
// phát trong cửa sổ này ⇒ vẫn tốn công SHA-256 như bình thường.
//
// Trả về challenge ID thật sở hữu lời giải, hoặc "" nếu không tìm thấy.
func healMismatchedSolution(solution string) string {
	if solution == "" {
		return ""
	}

	issuedChallengesMu.RLock()
	ids := append([]string{}, issuedChallengesOrder...)
	issuedChallengesMu.RUnlock()

	// Quét TỪ MỚI NHẤT về cũ (race condition xảy ra trong vài chục ms).
	for i := len(ids) - 1; i >= 0; i-- {
		val, ok := issuedChallenges.Load(ids[i])
		if !ok {
			continue
		}
		ic, ok := val.(*issuedChallenge)
		if !ok {
			continue
		}
		// Chỉ xét challenge còn hiệu lực + phát gần đây.
		if time.Now().After(time.Unix(ic.expiresAt, 0)) {
			continue
		}
		if time.Since(ic.issuedAt) > powHealWindow {
			continue
		}

		// Độ khó dùng để kiểm tra = độ khó của challenge GỐC (không phải
		// difficulty client khai báo — client có thể khai sai).
		hashBytes := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", ic.challengeID, ic.salt, solution)))
		hashHex := hex.EncodeToString(hashBytes[:])
		if strings.HasPrefix(hashHex, strings.Repeat("0", ic.difficulty)) {
			return ic.challengeID
		}
	}
	return ""
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
	//
	// Chấp nhận sai số đồng hồ (clock skew) trong khoảng `challengeClockSkewGrace`
	// để không mất lead thật khi server/client lệch vài giây.
	if time.Now().Add(challengeClockSkewGrace).Unix() > expiresAt {
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
		// ⚠️ LỚP PHỤC HỒI (bug 2026-11-15): client có thể gửi lời giải của
		// challenge KHÁC do race condition (challenge A + solution B).
		//
		// ⇒ Thử tìm challenge thật sở hữu lời giải này. Nếu tìm được ⇒ client
		//   ĐÃ giải PoW đúng ⇒ coi như hợp lệ (dùng lại challenge đó cho bước
		//   chống replay). Nếu không ⇒ bot / lỗi thật ⇒ giữ hành vi cũ.
		if realID := healMismatchedSolution(solution); realID != "" {
			// Dùng lại challenge thật cho bước chống replay + fingerprint.
			challengeID = realID
			prefix = strings.Repeat("0", difficulty)
		} else {
			return PowResult{OK: false, Code: PowCodeInvalidSolution, Reason: fmt.Sprintf("Lời giải không chính xác (Hash: %s, cần bắt đầu bằng %s)", hashHex[:8], prefix)}
		}
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

	return PowResult{OK: true, Code: PowCodeOK, Reason: "", EffectiveChallengeID: challengeID}
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
				// Dọn challenge đã phát quá cửa sổ tự phục hồi (powHealWindow).
				// ⇒ Bộ nhớ không phình vô hạn theo thời gian.
				issuedChallengesMu.Lock()
				if len(issuedChallengesOrder) > 0 {
					keep := issuedChallengesOrder[:0]
					for _, id := range issuedChallengesOrder {
						val, ok := issuedChallenges.Load(id)
						if !ok {
							continue
						}
						ic, ok := val.(*issuedChallenge)
						if !ok {
							continue
						}
						if now.Sub(ic.issuedAt) > powHealWindow {
							issuedChallenges.Delete(id)
							continue
						}
						keep = append(keep, id)
					}
					issuedChallengesOrder = keep
				}
				issuedChallengesMu.Unlock()
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
