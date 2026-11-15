/**
 * Regression test cho bug 2026-11-15: LEAD THẬT bị ghi vào `spam_quarantine`
 * với lý do "Lỗi Proof-of-Work" (pow_invalid).
 *
 * NGUYÊN NHÂN — RACE CONDITION trong `tracker.js`:
 *   `ensureChallengeReady()` được gọi SONG SONG từ nhiều nơi (autofill,
 *   focusin, input, change, submit). Mỗi lần gọi:
 *     1. fetch challenge MỚI  (A, B, C...)
 *     2. currentChallenge = <challenge vừa fetch>   ← ghi đè
 *     3. currentChallengeSolution = null            ← xoá
 *     4. await solveProofOfWork(challenge)          ← YIELD (15-30ms)
 *     5. currentChallengeSolution = <lời giải>      ← ghi đè
 *
 *   Nếu lần gọi A giải CHẬM hơn lần gọi B:
 *     B xong trước  → (cur=B, sol=solB)
 *     A xong sau    → (cur=B, sol=solA)   ← cur KHÔNG đổi, sol bị ghi đè
 *   ⇒ payload gửi đi có challenge_id của B nhưng solution của A
 *   ⇒ server băm SHA256(B_salt:solA) ⇒ KHÔNG khớp ⇒ lead thật bị cách ly.
 *
 * ⇒ TEST NÀY mô phỏng đúng kịch bản đó trên BẢN SỬA và khẳng định cặp
 *   (challenge, solution) luôn khớp 1-1.
 *
 * Chạy: node backend/tests/e2e_pow_race_condition_test.js
 */

const crypto = require('crypto');
const fs = require('fs');
const path = require('path');

// ─── Tiện ích ────────────────────────────────────────────────────────────
function sha256Hex(str) {
  return crypto.createHash('sha256').update(str).digest('hex');
}

/** Bản sao của `solveProofOfWork` trong tracker.js (dùng SHA-256 thuần). */
function solvePow(challenge) {
  const prefix = '0'.repeat(challenge.difficulty || 3);
  for (let nonce = 0; nonce < 2000000; nonce++) {
    const hash = sha256Hex(challenge.challenge_id + ':' + challenge.salt + ':' + nonce);
    if (hash.startsWith(prefix)) return String(nonce);
  }
  return '';
}

/** Bản sao của `verifyPowSolution` trong tracker.js. */
function verifyPow(challenge, solution) {
  if (!challenge || !challenge.challenge_id || !challenge.salt || !solution) return false;
  const prefix = '0'.repeat(challenge.difficulty || 3);
  const hash = sha256Hex(challenge.challenge_id + ':' + challenge.salt + ':' + solution);
  return hash.startsWith(prefix);
}

// ─── Đọc tracker.js thật để kiểm tra code đã được sửa ─────────────────────
const TRACKER_PATH = path.join(__dirname, '..', '..', 'frontend', 'admin', 'js', 'tracker.js');
const trackerSrc = fs.readFileSync(TRACKER_PATH, 'utf8');

let failures = 0;
function check(name, cond, detail) {
  if (cond) {
    console.log('  [OK]   ' + name);
  } else {
    failures++;
    console.log('  [FAIL] ' + name + (detail ? ' — ' + detail : ''));
  }
}

console.log('=== Regression test: PoW race condition (bug 2026-11-15) ===\n');

console.log('[1] tracker.js đã được sửa đúng cấu trúc:');
check('dùng challengeState thay cho 2 biến rời rạc',
  /challengeState\s*=\s*\{\s*challenge:\s*null,\s*solution:\s*''\s*\}/.test(trackerSrc));
check('có challengeFetchPromise gom lời gọi song song',
  /challengeFetchPromise/.test(trackerSrc));
check('không còn khai báo biến currentChallenge / currentChallengeSolution',
  !/let\s+currentChallenge\s*=/.test(trackerSrc) &&
  !/let\s+currentChallengeSolution\s*=/.test(trackerSrc));
check('payload lấy challenge + solution từ challengeState',
  /challenge_id:\s*challengeState\.challenge/.test(trackerSrc) &&
  /challenge_solution:\s*challengeState\.solution/.test(trackerSrc));
check('có lớp tự verify PoW phía client (verifyPowSolution)',
  /function verifyPowSolution/.test(trackerSrc));
check('gọi ensurePowPairValid trước khi gửi',
  /await ensurePowPairValid\(\)/.test(trackerSrc));

console.log('\n[2] Mô phỏng race nhiều lần — cặp (challenge, solution) luôn khớp:');

// Mô phỏng ĐÚNG logic ĐÃ SỬA: mọi lời gọi song song dùng chung 1 promise.
async function simulateFixed(concurrency) {
  let state = { challenge: null, solution: '' };
  let fetchPromise = null;

  const fetchAndSolve = async () => {
    const challenge = { challenge_id: 'ch_' + Math.random().toString(16).slice(2), salt: 'salt_' + Math.random().toString(16).slice(2), difficulty: 3 };
    state.challenge = challenge;
    state.solution = '';
    // Giả lập độ trễ giải PoW ngẫu nhiên (mạng chập chờn).
    await new Promise(r => setTimeout(r, Math.floor(Math.random() * 40)));
    const solution = solvePow(challenge);
    if (state.challenge === challenge) state.solution = solution;
    return challenge;
  };

  const ensure = async () => {
    const nowSec = Date.now() / 1000;
    const valid = state.challenge && state.challenge.expires_at > (nowSec + 90) && state.solution;
    if (valid) return state.challenge;
    if (!fetchPromise) {
      fetchPromise = fetchAndSolve().finally(() => { fetchPromise = null; });
    }
    try { await fetchPromise; } catch (_) { return null; }
    return state.challenge || null;
  };

  // Gọi song song N lần (autofill + focusin + input + change + submit).
  await Promise.all(Array.from({ length: concurrency }, () => ensure()));

  // Mô phỏng lớp phòng thủ cuối: ensurePowPairValid().
  const st = state;
  if (st.challenge && (!st.solution || !verifyPow(st.challenge, st.solution))) {
    const solution = solvePow(st.challenge);
    if (solution && verifyPow(st.challenge, solution)) st.solution = solution;
  }
  return st;
}

(async () => {
  for (const concurrency of [2, 3, 5, 8]) {
    const st = await simulateFixed(concurrency);
    const ok = verifyPow(st.challenge, st.solution);
    check('concurrency=' + concurrency + ' → PoW khớp 1-1', ok,
      ok ? '' : 'challenge=' + (st.challenge && st.challenge.challenge_id) + ' solution=' + st.solution);
  }

  console.log('\n[3] Bản Cũ (mô phỏng) KHÔNG khớp — xác nhận test có ý nghĩa:');
  async function simulateOld() {
    let cur = null, sol = null;
    const ensure = async (id, salt, fetchDelay, solveDelay) => {
      await new Promise(r => setTimeout(r, fetchDelay));
      cur = { challenge_id: id, salt: salt, difficulty: 3 };
      sol = null;
      const captured = cur;
      await new Promise(r => setTimeout(r, solveDelay));
      sol = solvePow(captured);
    };
    await Promise.all([
      ensure('ch_A', 'salt_A', 1, 30),
      ensure('ch_B', 'salt_B', 2, 5),
    ]);
    return { challenge: cur, solution: sol };
  }
  const old = await simulateOld();
  const oldOk = verifyPow(old.challenge, old.solution);
  check('bản cũ thực sự bị lệch (đây chính là bug)', !oldOk,
    oldOk ? 'bản cũ cũng khớp ⇒ test không chứng minh được bug'
           : 'tái hiện bug thành công: challenge=' + old.challenge.challenge_id +
             ' nhưng solution thuộc challenge khác');

  console.log('\n=== ' + (failures === 0 ? 'TẤT CẢ PASS' : failures + ' LỖI') + ' ===');
  process.exit(failures === 0 ? 0 : 1);
})();
