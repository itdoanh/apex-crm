/**
 * Sửa mojibake (double-encoding UTF-8 → CP1252 → UTF-8) trong file HTML.
 * Chạy cho file đã bị PowerShell 5.1 decode sai.
 *
 * Usage: node scripts/fix-mojibake.js <file> [...]
 */
const fs = require('fs');

function fixDoubleEncoding(s) {
  // Latin-1 (CP1252) → UTF-8: mỗi byte >0x7F cần 2 byte UTF-8
  const bytes = [];
  for (let i = 0; i < s.length; i++) {
    const code = s.charCodeAt(i);
    if (code <= 0xff) bytes.push(code);
    else return s; // đã là UTF-8 sạch, không sửa
  }
  const buf = Buffer.from(bytes);
  const decoded = buf.toString('utf8');
  // Chỉ chấp nhận nếu kết quả không còn chứa byte thay thế (U+FFFD)
  return decoded.includes('\uFFFD') ? s : decoded;
}

for (const f of process.argv.slice(2)) {
  const s = fs.readFileSync(f, 'utf8');
  const fixed = fixDoubleEncoding(s);
  if (fixed !== s) {
    fs.writeFileSync(f, fixed, 'utf8');
    console.log(`${f}: fixed (${s.length} -> ${fixed.length})`);
  } else {
    console.log(`${f}: clean`);
  }
}
