/**
 * E2E test: replay payload PoW từ phiên KHÁC phải bị ghi vào vùng cách ly.
 * Đây là test AN TOÀN chống bot — bảo đảm fix "duplicate submit" KHÔNG làm
 * suy yếu lớp phòng thủ Proof-of-Work.
 *
 * Usage: node backend/tests/e2e_pow_replay_blocked_test.js
 */
const crypto = require('crypto');

const BASE = process.env.APEX_TEST_BASE || 'http://127.0.0.1:8080';
const API = BASE + '/api';

function sha256Hex(str) {
    return crypto.createHash('sha256').update(str).digest('hex');
}

async function getChallenge() {
    const res = await fetch(API + '/auth/challenge?difficulty=3');
    const json = await res.json();
    if (!json.success || !json.data) throw new Error('Không lấy được challenge');
    return json.data;
}

function solvePoW(challenge) {
    const prefix = '0'.repeat(challenge.difficulty || 3);
    for (let nonce = 0; nonce < 5000000; nonce++) {
        if (sha256Hex(challenge.challenge_id + ':' + challenge.salt + ':' + nonce).startsWith(prefix)) {
            return String(nonce);
        }
    }
    throw new Error('Không giải được PoW');
}

function payload(challenge, solution, sessionId, visitorId) {
    return {
        landing_id: 'landing_cu',
        nguon: 'landing_chinh',
        so_dien_thoai: '0358123456',
        phone: '0358123456',
        fullName: 'Test Bot',
        session_id: sessionId,
        visitor_id: visitorId,
        entry_url: 'https://ads.giaodichhanghoatuonglai.com/landing?utm_source=an',
        url: 'https://ads.giaodichhanghoatuonglai.com/landing?utm_source=an',
        referrer: 'https://l.facebook.com/',
        utm_source: 'an',
        is_autofill: false,
        fill_duration_ms: 30000,
        time_on_page: 40,
        max_scroll: 40,
        clicks_count: 3,
        website_fax_number_sec: '',
        fax_company_prefix: '',
        terms_confirm_bot: false,
        user_email_verify: '',
        company_website_url: '',
        challenge_id: challenge.challenge_id,
        challenge_salt: challenge.salt,
        challenge_difficulty: challenge.difficulty,
        challenge_expires_at: challenge.expires_at,
        challenge_signature: challenge.signature,
        challenge_solution: solution
    };
}

async function post(body) {
    const res = await fetch(API + '/leads', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json; charset=utf-8' },
        body: JSON.stringify(body)
    });
    let json = null;
    try { json = await res.json(); } catch (_) {}
    return { status: res.status, json };
}

(async function main() {
    console.log('=== E2E: replay PoW từ phiên khác phải bị CÁCH LY (chống bot) ===');

    const ch = await getChallenge();
    const sol = solvePoW(ch);

    console.log('\n▶ Bước 1: request gốc (phiên A) — phải tạo lead');
    const first = await post(payload(ch, sol, 'sess_A', 'v_A'));
    console.log('  status=' + first.status + ' id=' + (first.json && first.json.id));
    if (!first.json || !first.json.success || !first.json.id || first.json.id === 999999) {
        console.error('  ❌ Request gốc phải tạo lead');
        process.exit(1);
    }
    console.log('  ✅ Request gốc tạo lead id=' + first.json.id);

    console.log('\n▶ Bước 2: BOT chơi lại CÙNG payload nhưng phiên khác (session B) — phải bị cách ly');
    const replay = await post(payload(ch, sol, 'sess_B_BOT', 'v_B_BOT'));
    console.log('  status=' + replay.status + ' body=' + JSON.stringify(replay.json));
    // Shadow ban: server trả 200 OK giả để bot không nhận biết
    if (replay.status === 200 && replay.json && replay.json.id === 999999) {
        console.log('  ✅ Bot nhận 200 OK giả (shadow ban) — KHÔNG tạo lead');
    } else {
        console.error('  ❌ Bot replay phải bị shadow ban');
        process.exit(1);
    }

    console.log('\n▶ Bước 3: kiểm tra bảng spam_quarantine có bản ghi "pow_invalid"');
    console.log('  (kiểm tra thủ công: SELECT id, phone, reason FROM spam_quarantine WHERE reason LIKE \'pow_invalid%\')');
    console.log('\n=== KẾT QUẢ: replay từ phiên khác ĐÃ bị chặn ===');
})().catch(err => {
    console.error('❌ LỖI:', err.message);
    process.exit(1);
});
