package security

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"testing"
)

// solveForTest tìm nonce thỏa mãn điều kiện hash giống hàm solveProofOfWork phía client.
func solveForTest(t *testing.T, challengeID, salt string, difficulty int) string {
	t.Helper()
	prefix := ""
	for i := 0; i < difficulty; i++ {
		prefix += "0"
	}
	for nonce := 0; nonce < 1000000; nonce++ {
		dataToHash := fmt.Sprintf("%s:%s:%d", challengeID, salt, nonce)
		h := sha256.Sum256([]byte(dataToHash))
		hashHex := hex.EncodeToString(h[:])
		if len(hashHex) >= len(prefix) && hashHex[:len(prefix)] == prefix {
			return fmt.Sprintf("%d", nonce)
		}
	}
	t.Fatalf("không tìm được solution cho challenge %s", challengeID)
	return ""
}

// TestVerifyChallengeHappyPath: PoW đúng → OK.
func TestVerifyChallengeHappyPath(t *testing.T) {
	ch, err := GenerateChallenge(3)
	if err != nil {
		t.Fatalf("GenerateChallenge lỗi: %v", err)
	}
	sol := solveForTest(t, ch.ChallengeID, ch.Salt, 3)

	res := VerifyChallenge(ch.ChallengeID, ch.Salt, 3, ch.ExpiresAt, ch.Signature, sol, "sess_1|vis_1|0345833454|1.2.3.4")
	if !res.OK || res.Code != PowCodeOK {
		t.Fatalf("kỳ vọng OK, nhận được code=%s reason=%s", res.Code, res.Reason)
	}
}

// TestVerifyChallengeDuplicateSubmitSameSession:
// Request trùng lặp từ CÙNG phiên đăng ký ⇒ duplicate_submit (KHÔNG phải spam).
// Đây là trường hợp gây bug "lead thật bị gắn nhãn Lỗi Proof-of-Work".
func TestVerifyChallengeDuplicateSubmitSameSession(t *testing.T) {
	ch, err := GenerateChallenge(3)
	if err != nil {
		t.Fatalf("GenerateChallenge lỗi: %v", err)
	}
	sol := solveForTest(t, ch.ChallengeID, ch.Salt, 3)
	fp := "sess_abc|vis_xyz|0345833454|171.248.193.212"

	// Request gốc → OK
	res1 := VerifyChallenge(ch.ChallengeID, ch.Salt, 3, ch.ExpiresAt, ch.Signature, sol, fp)
	if !res1.OK {
		t.Fatalf("request gốc phải OK, nhận code=%s", res1.Code)
	}

	// Request thứ 2, CÙNG phiên → duplicate_submit
	res2 := VerifyChallenge(ch.ChallengeID, ch.Salt, 3, ch.ExpiresAt, ch.Signature, sol, fp)
	if res2.OK {
		t.Fatal("request trùng lặp không được OK")
	}
	if res2.Code != PowCodeDuplicateSubmit {
		t.Fatalf("kỳ vọng duplicate_submit, nhận code=%s reason=%s", res2.Code, res2.Reason)
	}

	// Request thứ 3, CÙNG phiên → vẫn duplicate_submit
	res3 := VerifyChallenge(ch.ChallengeID, ch.Salt, 3, ch.ExpiresAt, ch.Signature, sol, fp)
	if res3.Code != PowCodeDuplicateSubmit {
		t.Fatalf("kỳ vọng duplicate_submit, nhận code=%s", res3.Code)
	}
}

// TestVerifyChallengeReplayDifferentSession:
// Bot chơi lại payload từ phiên KHÁC ⇒ phải bị coi là replay (ghi vào cách ly).
func TestVerifyChallengeReplayDifferentSession(t *testing.T) {
	ch, err := GenerateChallenge(3)
	if err != nil {
		t.Fatalf("GenerateChallenge lỗi: %v", err)
	}
	sol := solveForTest(t, ch.ChallengeID, ch.Salt, 3)

	res1 := VerifyChallenge(ch.ChallengeID, ch.Salt, 3, ch.ExpiresAt, ch.Signature, sol, "sess_A|vis_A|0345833454|1.1.1.1")
	if !res1.OK {
		t.Fatalf("request gốc phải OK, nhận code=%s", res1.Code)
	}

	res2 := VerifyChallenge(ch.ChallengeID, ch.Salt, 3, ch.ExpiresAt, ch.Signature, sol, "sess_B|vis_B|0999888777|9.9.9.9")
	if res2.OK || res2.Code != PowCodeReplay {
		t.Fatalf("kỳ vọng replay, nhận code=%s reason=%s", res2.Code, res2.Reason)
	}
}

// TestVerifyChallengeConcurrentDuplicate:
// 2 request CÙNG LÚC tới server (request gốc đang xử lý) ⇒ request sau là duplicate.
func TestVerifyChallengeConcurrentDuplicate(t *testing.T) {
	ch, err := GenerateChallenge(3)
	if err != nil {
		t.Fatalf("GenerateChallenge lỗi: %v", err)
	}
	sol := solveForTest(t, ch.ChallengeID, ch.Salt, 3)
	fp := "sess_C|vis_C|0345833454|2.2.2.2"

	res1 := VerifyChallenge(ch.ChallengeID, ch.Salt, 3, ch.ExpiresAt, ch.Signature, sol, fp)
	if !res1.OK {
		t.Fatalf("request gốc phải OK, nhận code=%s", res1.Code)
	}

	// Mô phỏng request gốc đang được xử lý (MarkChallengeActive)
	MarkChallengeActive(ch.ChallengeID)

	res2 := VerifyChallenge(ch.ChallengeID, ch.Salt, 3, ch.ExpiresAt, ch.Signature, sol, fp)
	if res2.Code != PowCodeDuplicateSubmit {
		t.Fatalf("kỳ vọng duplicate_submit khi request gốc đang active, nhận code=%s", res2.Code)
	}
	ReleaseChallengeActive(ch.ChallengeID)
}

// TestVerifyChallengeInvalidSolution: lời giải sai → invalid_solution,
// và nonce phải được trả lại để lời giải đúng sau đó vẫn dùng được.
func TestVerifyChallengeInvalidSolution(t *testing.T) {
	ch, err := GenerateChallenge(3)
	if err != nil {
		t.Fatalf("GenerateChallenge lỗi: %v", err)
	}

	res := VerifyChallenge(ch.ChallengeID, ch.Salt, 3, ch.ExpiresAt, ch.Signature, "999999999", "sess_D|vis_D|0345833454|3.3.3.3")
	if res.OK || res.Code != PowCodeInvalidSolution {
		t.Fatalf("kỳ vọng invalid_solution, nhận code=%s reason=%s", res.Code, res.Reason)
	}

	// Lời giải đúng sau đó vẫn phải dùng được (nonce đã được trả lại)
	sol := solveForTest(t, ch.ChallengeID, ch.Salt, 3)
	res2 := VerifyChallenge(ch.ChallengeID, ch.Salt, 3, ch.ExpiresAt, ch.Signature, sol, "sess_D|vis_D|0345833454|3.3.3.3")
	if !res2.OK {
		t.Fatalf("lời giải đúng phải OK sau khi lời giải sai bị từ chối, nhận code=%s", res2.Code)
	}
}

// TestVerifyChallengeBadSignature: chữ ký sai → bad_signature (vẫn ghi cách ly).
func TestVerifyChallengeBadSignature(t *testing.T) {
	ch, err := GenerateChallenge(3)
	if err != nil {
		t.Fatalf("GenerateChallenge lỗi: %v", err)
	}
	sol := solveForTest(t, ch.ChallengeID, ch.Salt, 3)

	res := VerifyChallenge(ch.ChallengeID, ch.Salt, 3, ch.ExpiresAt, "deadbeef", sol, "sess_E|vis_E|0345833454|4.4.4.4")
	if res.OK || res.Code != PowCodeBadSignature {
		t.Fatalf("kỳ vọng bad_signature, nhận code=%s", res.Code)
	}
}

// TestVerifyChallengeExpired: challenge hết hạn → expired.
func TestVerifyChallengeExpired(t *testing.T) {
	ch, err := GenerateChallenge(3)
	if err != nil {
		t.Fatalf("GenerateChallenge lỗi: %v", err)
	}
	sol := solveForTest(t, ch.ChallengeID, ch.Salt, 3)

	res := VerifyChallenge(ch.ChallengeID, ch.Salt, 3, ch.ExpiresAt-3600, ch.Signature, sol, "sess_F|vis_F|0345833454|5.5.5.5")
	if res.OK || res.Code != PowCodeExpired {
		t.Fatalf("kỳ vọng expired, nhận code=%s", res.Code)
	}
}

// TestVerifyChallengeMissingParams: thiếu tham số → missing_params.
func TestVerifyChallengeMissingParams(t *testing.T) {
	res := VerifyChallenge("", "", 3, 0, "", "", "fp")
	if res.OK || res.Code != PowCodeMissingParams {
		t.Fatalf("kỳ vọng missing_params, nhận code=%s", res.Code)
	}
}

// TestVerifyChallengeHealMismatchedSolution — regression test cho bug 2026-11-15:
// lead thật #65 (0368942836 / "Luc van pao") bị ghi vào `spam_quarantine` với
// lý do "Lỗi Proof-of-Work" (`pow_invalid`).
//
// BẰNG CHỨNG: HMAC của challenge khớp (challenge do server thật phát), nhưng
// SHA256(challenge_id:salt:solution) KHÔNG bắt đầu bằng "000" ⇒ lời giải thuộc
// về challenge KHÁC ⇒ race condition client (challenge A + solution B).
//
// ⇒ SERVER phải tự phục hồi: quét các challenge nó đã phát, tìm challenge thật
// sở hữu lời giải ⇒ coi như hợp lệ (client ĐÃ giải PoW đúng).
//
// KỲ VỌNG: PoW hợp lệ (OK) thay vì invalid_solution.
func TestVerifyChallengeHealMismatchedSolution(t *testing.T) {
	// Challenge A — client giải được (lời giải thật).
	chA, err := GenerateChallenge(3)
	if err != nil {
		t.Fatalf("GenerateChallenge lỗi: %v", err)
	}
	solA := solveForTest(t, chA.ChallengeID, chA.Salt, 3)

	// Challenge B — client KHAI BÁO nhưng lời giải gửi lên là của challenge A.
	chB, err := GenerateChallenge(3)
	if err != nil {
		t.Fatalf("GenerateChallenge lỗi: %v", err)
	}

	res := VerifyChallenge(chB.ChallengeID, chB.Salt, 3, chB.ExpiresAt, chB.Signature, solA,
		"sess_heal|vis_heal|0368942836|104.28.199.125")
	if !res.OK {
		t.Fatalf("server phải tự phục hồi PoW (race condition client), nhận code=%s reason=%s", res.Code, res.Reason)
	}
	if res.EffectiveChallengeID != chA.ChallengeID {
		t.Fatalf("EffectiveChallengeID phải là challenge thật (%s), nhận %s", chA.ChallengeID, res.EffectiveChallengeID)
	}
}

// TestVerifyChallengeHealDoesNotAcceptGarbage — lớp phục hồi KHÔNG được nới lỏng
// chống bot: lời giải bất kỳ (không thuộc challenge nào server đã phát) vẫn
// phải bị từ chối.
func TestVerifyChallengeHealDoesNotAcceptGarbage(t *testing.T) {
	ch, err := GenerateChallenge(3)
	if err != nil {
		t.Fatalf("GenerateChallenge lỗi: %v", err)
	}

	// Lời giải ngẫu nhiên — không thuộc challenge nào server đã phát.
	res := VerifyChallenge(ch.ChallengeID, ch.Salt, 3, ch.ExpiresAt, ch.Signature, "42424242",
		"sess_garbage|vis_garbage|0368942836|104.28.199.125")
	if res.OK || res.Code != PowCodeInvalidSolution {
		t.Fatalf("lời giải rác phải bị từ chối (invalid_solution), nhận code=%s reason=%s", res.Code, res.Reason)
	}
}

// TestVerifyChallengeHealStillBlocksReplay — sau khi phục hồi, cơ chế chống
// replay vẫn phải hoạt động: gửi lại cùng cặp (challenge B + solution A) từ
// phiên KHÁC ⇒ replay ⇒ bị ghi vào vùng cách ly.
func TestVerifyChallengeHealStillBlocksReplay(t *testing.T) {
	chA, err := GenerateChallenge(3)
	if err != nil {
		t.Fatalf("GenerateChallenge lỗi: %v", err)
	}
	solA := solveForTest(t, chA.ChallengeID, chA.Salt, 3)

	chB, err := GenerateChallenge(3)
	if err != nil {
		t.Fatalf("GenerateChallenge lỗi: %v", err)
	}

	// Phiên 1 ⇒ OK (được phục hồi).
	res1 := VerifyChallenge(chB.ChallengeID, chB.Salt, 3, chB.ExpiresAt, chB.Signature, solA, "sess_R1|v1|0368942836|1.1.1.1")
	if !res1.OK {
		t.Fatalf("lần 1 phải OK, nhận code=%s reason=%s", res1.Code, res1.Reason)
	}

	// Phiên 2 (khác) ⇒ replay ⇒ bị chặn.
	res2 := VerifyChallenge(chB.ChallengeID, chB.Salt, 3, chB.ExpiresAt, chB.Signature, solA, "sess_R2|v2|0999888777|2.2.2.2")
	if res2.OK || res2.Code != PowCodeReplay {
		t.Fatalf("lần 2 từ phiên khác phải là replay, nhận code=%s reason=%s", res2.Code, res2.Reason)
	}
}

// TestVerifyChallengeDuplicateSubmitAfterHeal — sau khi phục hồi, request trùng
// lặp từ CÙNG phiên đăng ký vẫn phải là duplicate_submit (KHÔNG phải spam) ⇒
// lead thật không bị ghi vào vùng cách ly.
func TestVerifyChallengeDuplicateSubmitAfterHeal(t *testing.T) {
	chA, err := GenerateChallenge(3)
	if err != nil {
		t.Fatalf("GenerateChallenge lỗi: %v", err)
	}
	solA := solveForTest(t, chA.ChallengeID, chA.Salt, 3)

	chB, err := GenerateChallenge(3)
	if err != nil {
		t.Fatalf("GenerateChallenge lỗi: %v", err)
	}
	fp := "sess_DUP|v_DUP|0368942836|104.28.199.125"

	res1 := VerifyChallenge(chB.ChallengeID, chB.Salt, 3, chB.ExpiresAt, chB.Signature, solA, fp)
	if !res1.OK {
		t.Fatalf("lần 1 phải OK, nhận code=%s reason=%s", res1.Code, res1.Reason)
	}

	res2 := VerifyChallenge(chB.ChallengeID, chB.Salt, 3, chB.ExpiresAt, chB.Signature, solA, fp)
	if res2.OK || res2.Code != PowCodeDuplicateSubmit {
		t.Fatalf("lần 2 cùng phiên phải là duplicate_submit, nhận code=%s reason=%s", res2.Code, res2.Reason)
	}
}

// TestVerifyChallengeHealBadSignatureStillRejected — lớp phục hồi KHÔNG được
// bỏ qua bước kiểm tra HMAC: client sửa tham số challenge ⇒ vẫn bị từ chối.
func TestVerifyChallengeHealBadSignatureStillRejected(t *testing.T) {
	chA, err := GenerateChallenge(3)
	if err != nil {
		t.Fatalf("GenerateChallenge lỗi: %v", err)
	}
	solA := solveForTest(t, chA.ChallengeID, chA.Salt, 3)

	chB, err := GenerateChallenge(3)
	if err != nil {
		t.Fatalf("GenerateChallenge lỗi: %v", err)
	}

	// Chữ ký sai ⇒ bad_signature (KHÔNG được phục hồi).
	res := VerifyChallenge(chB.ChallengeID, chB.Salt, 3, chB.ExpiresAt, "deadbeef", solA,
		"sess_badsig|v|0368942836|104.28.199.125")
	if res.OK || res.Code != PowCodeBadSignature {
		t.Fatalf("chữ ký sai phải bị từ chối (bad_signature), nhận code=%s reason=%s", res.Code, res.Reason)
	}
}
// 3 request CÙNG LÚC tới (capture listener + onsubmit + tracker interceptor)
// với CÙNG payload (cùng challenge_id + challenge_solution + cùng phiên).
//
// KỲ VỌNG: đúng 1 request OK (tạo lead), 2 request còn lại là duplicate_submit
// ⇒ KHÔNG request nào bị ghi vào vùng cách ly ⇒ lead thật không bị gắn nhãn
// "Lỗi Proof-of-Work" trong tab Bảo mật & Bot.
func TestVerifyChallengeParallelSameSession(t *testing.T) {
	ch, err := GenerateChallenge(3)
	if err != nil {
		t.Fatalf("GenerateChallenge lỗi: %v", err)
	}
	sol := solveForTest(t, ch.ChallengeID, ch.Salt, 3)
	fp := "sess_jpgqi8ab_mw33mp8y|v_4uz9ed71_mw33mp8x|0345833454|171.248.193.212"

	const n = 3
	results := make([]PowResult, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			results[idx] = VerifyChallenge(ch.ChallengeID, ch.Salt, 3, ch.ExpiresAt, ch.Signature, sol, fp)
		}(i)
	}
	close(start)
	wg.Wait()

	okCount := 0
	dupCount := 0
	for _, res := range results {
		switch res.Code {
		case PowCodeOK:
			okCount++
		case PowCodeDuplicateSubmit:
			dupCount++
		default:
			t.Fatalf("gặp mã lý do không mong đợi: %s (%s)", res.Code, res.Reason)
		}
	}
	if okCount != 1 {
		t.Fatalf("kỳ vọng đúng 1 request OK, nhận %d", okCount)
	}
	if dupCount != n-1 {
		t.Fatalf("kỳ vọng %d request duplicate_submit, nhận %d", n-1, dupCount)
	}
}

// TestVerifyChallengeParallelDifferentSession — 3 request song song nhưng khác
// fingerprint (3 phiên/3 thiết bị khác nhau) ⇒ chỉ 1 request được hợp lệ,
// 2 request còn lại là replay (bị ghi vào vùng cách ly) ⇒ chống bot vẫn hoạt động.
func TestVerifyChallengeParallelDifferentSession(t *testing.T) {
	ch, err := GenerateChallenge(3)
	if err != nil {
		t.Fatalf("GenerateChallenge lỗi: %v", err)
	}
	sol := solveForTest(t, ch.ChallengeID, ch.Salt, 3)

	fps := []string{
		"sess_A|v_A|0345833454|1.1.1.1",
		"sess_B|v_B|0345833454|2.2.2.2",
		"sess_C|v_C|0345833454|3.3.3.3",
	}
	const n = 3
	results := make([]PowResult, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			results[idx] = VerifyChallenge(ch.ChallengeID, ch.Salt, 3, ch.ExpiresAt, ch.Signature, sol, fps[idx])
		}(i)
	}
	close(start)
	wg.Wait()

	okCount := 0
	replayCount := 0
	for _, res := range results {
		switch res.Code {
		case PowCodeOK:
			okCount++
		case PowCodeReplay:
			replayCount++
		default:
			t.Fatalf("gặp mã lý do không mong đợi: %s (%s)", res.Code, res.Reason)
		}
	}
	if okCount != 1 {
		t.Fatalf("kỳ vọng đúng 1 request OK, nhận %d", okCount)
	}
	if replayCount != n-1 {
		t.Fatalf("kỳ vọng %d request replay, nhận %d", n-1, replayCount)
	}
}
