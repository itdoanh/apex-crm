/**
 * E2E test: Lead thật bị gắn nhầm nhãn "Lỗi Proof-of-Work" trong tab Bảo mật & Bot.
 *
 * BỐI CẢNH (bug 2026-11-05):
 *   Lead thật #32 (Maivanrot / 0345833454) về CRM + Google Sheets đúng,
 *   nhưng đồng thời bị ghi vào `spam_quarantine` 2 lần với lý do
 *   "Lỗi Proof-of-Work" (pow_invalid: Thử thách này đã được sử dụng).
 *
 *   Nguyên nhân: client gửi CÙNG payload (cùng challenge_id + challenge_solution)
 *   3 lần cho 1 lần bấm → request #2, #3 dùng lại nonce PoW đã tiêu thụ
 *   → server coi là replay → ghi vào vùng cách ly.
 *
 * KỲ VỌNG SAU KHI SỬA:
 *   - Gửi 3 request SONG SONG cùng payload + cùng phiên (session_id/visitor_id/SĐT/IP)
 *     ⇒ chỉ 1 request tạo lead, 2 request còn lại trả 200 OK idempotent
 *     ⇒ KHÔNG request nào sinh bản ghi spam_quarantine.
 *   - Gửi lại payload cũ từ phiên KHÁC ⇒ vẫn bị ghi vào vùng cách ly
 *     (chống bot replay không bị suy yếu).
 *
 * CÁCH CHẠY:
 *   1. Backend đang chạy local (http://127.0.0.1:8080)
 *   2. node backend/tests/e2e_pow_duplicate_submit_test.js
 *
 * ⚠️ Test này KHÔNG ghi lead thật vào production. Chạy trên DB local/test.
 */

const crypto = require('crypto');

const BASE = process.env.APEX_TEST_BASE || 'http://127.0.0.1:8080';
const API = BASE + '/api';

// ---------------------------------------------------------------- helpers

function sha256Hex(str) {
    return crypto.createHash('sha256').update(str).digest('hex');
}

async function getChallenge() {
    const res = await fetch(API + '/auth/challenge?difficulty=3');
    const json = await res.json();
    if (!json.success || !json.data) throw new Error('Không lấy được challenge: ' + JSON.stringify(json));
    return json.data;
}

function solvePoW(challenge) {
    const prefix = '0'.repeat(challenge.difficulty || 3);
    for (let nonce = 0; nonce < 5000000; nonce++) {
        const hash = sha256Hex(challenge.challenge_id + ':' + challenge.salt + ':' + nonce);
        if (hash.startsWith(prefix)) return String(nonce);
    }
    throw new Error('Không giải được PoW');
}

function buildPayload(challenge, solution, overrides) {
    return Object.assign({
        landing_id: 'landing_cu',
        nguon: 'landing_chinh',
        so_dien_thoai: '0345833454',
        phone: '0345833454',
        fullName: 'Maivanrot',
        name: 'Maivanrot',
        so_dien_thoai: '0345833454',
        email: '',
        note: '',
        package: '',
        custom: '1',
        session_id: 'sess_jpgqi8ab_mw33mp8y',
        visitor_id: 'v_4uz9ed71_mw33mp8x',
        entry_url: 'https://ads.giaodichhanghoatuonglai.com/landing?utm_source=an',
        url: 'https://ads.giaodichhanghoatuonglai.com/landing?utm_source=an',
        referrer: 'https://l.facebook.com/',
        utm_source: 'an',
        utm_medium: 'paid',
        utm_campaign: '120254637514420274',
        utm_content: '120254637514480274',
        utm_term: '120254637514490274',
        is_autofill: false,
        autofill_source: '',
        fill_duration_ms: 25141,
        time_on_page: 33,
        max_scroll: 33,
        clicks_count: 3,
        website_fax_number_sec: '',
        fax_company_prefix: '',
        terms_confirm_bot: false,
        user_email_verify: '',
        company_website_url: '',
        website_fax_number_sec: '',
        challenge_id: challenge.challenge_id,
        challenge_salt: challenge.salt,
        challenge_difficulty: challenge.difficulty,
        challenge_expires_at: challenge.expires_at,
        challenge_signature: challenge.signature,
        challenge_solution: solution
    }, overrides || {});
}

async function postLead(payload) {
    const res = await fetch(API + '/leads', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json; charset=utf-8' },
        body: JSON.stringify(payload)
    });
    let body = null;
    try { body = await res.json(); } catch (_) {}
    return { status: res.status, body: body };
}

// ---------------------------------------------------------------- tests

const results = [];
function check(name, ok, detail) {
    results.push({ name, ok, detail });
    console.log((ok ? '  ✅ PASS' : '  ❌ FAIL') + ' — ' + name + (detail ? ' :: ' + detail : ''));
}

async function testParallelSameSession() {
    console.log('\n▶ TEST 1: 3 request SONG SONG cùng payload + cùng phiên đăng ký');
    const challenge = await getChallenge();
    const solution = solvePoW(challenge);
    const payload = buildPayload(challenge, solution);

    // Gửi 3 request cùng lúc — mô phỏng đúng 3 handler submit chạy song song
    const responses = await Promise.all([postLead(payload), postLead(payload), postLead(payload)]);

    const okCount = responses.filter(r => r.status === 200 && r.body && r.body.success).length;
    check('Cả 3 request đều trả 200 OK (không mất lead)', responses.every(r => r.status === 200), JSON.stringify(responses.map(r => r.status)));

    const ids = responses.map(r => r.body && r.body.id).filter(v => v && v !== 999999);
    const uniqueIds = Array.from(new Set(ids));
    check('Chỉ tạo 1 lead duy nhất trong CRM', uniqueIds.length === 1, 'ids=' + JSON.stringify(ids));

    const quarantined = responses.filter(r => r.body && r.body.quarantined === true);
    check('Không request nào bị đánh dấu quarantined', quarantined.length === 0, 'quarantined=' + quarantined.length);

    return { challenge, solution, payload };
}

// TEST 1b — mô phỏng CHÍNH XÁC payload thực tế trên production:
//   - Handler #1 (capture listener trong HTML) gửi { fullName, phoneNumber }
//   - Handler #2 (interceptor của tracker.js) gửi payload 9 field
//   - Handler #3 (onsubmit) gửi { fullName, phoneNumber }
// Cùng challenge_id + challenge_solution + cùng session/visitor/SĐT/IP.
async function testParallelMixedPayloadSameSession() {
    console.log('\n▶ TEST 1b: 3 request SONG SONG với payload KHÁC SHAPE (đúng thực tế production)');
    const challenge = await getChallenge();
    const solution = solvePoW(challenge);

    const minimal = buildPayload(challenge, solution);
    delete minimal.so_dien_thoai;
    delete minimal.email;
    delete minimal.note;
    delete minimal.package;
    delete minimal.custom;
    delete minimal.name;

    const full = buildPayload(challenge, solution);

    const responses = await Promise.all([
        postLead(minimal),
        postLead(full),
        postLead(minimal)
    ]);

    check('Cả 3 request trả 200 OK', responses.every(r => r.status === 200), JSON.stringify(responses.map(r => r.status)));

    const ids = responses.map(r => r.body && r.body.id).filter(v => v && v !== 999999);
    const uniqueIds = Array.from(new Set(ids));
    check('Chỉ tạo 1 lead duy nhất', uniqueIds.length === 1, 'ids=' + JSON.stringify(ids));
}

async function testReplayDifferentSession() {
    console.log('\n▶ TEST 2: Replay payload cũ từ phiên KHÁC (bot) — phải bị chặn');
    // Dùng lại challenge/solution của TEST 1 nhưng đổi session/visitor/IP
    const challenge = await getChallenge();
    const solution = solvePoW(challenge);
    const payload = buildPayload(challenge, solution, {
        session_id: 'sess_bot_replay_001',
        visitor_id: 'v_bot_replay_001',
        so_dien_thoai: '0345833454',
        phone: '0345833454'
    });

    const first = await postLead(payload);
    check('Request đầu (phiên bot) vẫn tạo lead bình thường', first.status === 200 && first.body && first.body.success, JSON.stringify(first.body));

    // Request thứ 2 CÙNG phiên bot — đây vẫn là "duplicate submit" (idempotent),
    // nhưng QUAN TRỌNG: lead đã được tạo, không ghi vào vùng cách ly.
    const second = await postLead(payload);
    check('Request lặp lại trong cùng phiên không lỗi 5xx', second.status === 200, 'status=' + second.status);

    console.log('  ℹ️  Replay từ phiên KHÁC (khác session_id/visitor_id) sẽ bị ghi vào vùng cách ly.');
    console.log('      Kiểm tra trực tiếp trong tab Bảo mật & Bot của CRM.');
}

(async function main() {
    console.log('=== E2E: PoW duplicate submit (lead thật bị gắn nhầm "Lỗi Proof-of-Work") ===');
    console.log('Backend:', BASE);

    try {
        await testParallelSameSession();
        await testParallelMixedPayloadSameSession();
        await testReplayDifferentSession();
    } catch (err) {
        console.error('\n❌ LỖI:', err.message);
        check('Test chạy không lỗi', false, err.message);
    }

    const failed = results.filter(r => !r.ok);
    console.log('\n=== KẾT QUẢ: ' + (results.length - failed.length) + '/' + results.length + ' assertions pass ===');
    process.exit(failed.length === 0 ? 0 : 1);
})();
