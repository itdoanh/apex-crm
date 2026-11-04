/**
 * Xoá Tab "Form" (cfgTabSecForm) và Tab "Sự Kiện" (cfgTabSecEvent) khỏi
 * Landing Settings Modal — HTML Code Studio là nguồn sự thật duy nhất.
 *
 * Chạy trên file gốc UTF-8 (đọc/ghi bằng Buffer, KHÔNG dùng Get-Content của
 * PowerShell 5.1 vì nó decode sai encoding → sinh mojibake).
 */
const fs = require('fs');

const files = process.argv.slice(2);
if (files.length === 0) {
  console.error('Usage: node strip-settings-tabs.js <file.html> [...]');
  process.exit(1);
}

/** Xoá 1 khối HTML cân bằng thẻ <div> theo id (đếm open/close). */
function removeDivById(html, id) {
  const marker = `id="${id}"`;
  const at = html.indexOf(marker);
  if (at === -1) return html;

  // Lùi về đầu thẻ mở <div ...>
  const openStart = html.lastIndexOf('<div', at);
  if (openStart === -1) return html;

  // Tìm thẻ mở tương ứng bằng cách quét token <div ...> và </div>
  const tagRe = /<div\b[^>]*>|<\/div>/g;
  tagRe.lastIndex = openStart;

  let depth = 0;
  let m;
  while ((m = tagRe.exec(html)) !== null) {
    if (m[0] === '</div>') {
      depth--;
      if (depth === 0) {
        // Cắt tới hết thẻ đóng + newline kế tiếp
        let end = tagRe.lastIndex;
        if (html[end] === '\r') end++;
        if (html[end] === '\n') end++;
        return html.slice(0, openStart) + html.slice(end);
      }
    } else {
      depth++;
    }
  }
  return html;
}

/** Xoá 1 nút <button ... id="...">...</button> kèm newline. */
function removeButtonById(html, id) {
  const re = new RegExp(`[ \\t]*<button[^>]*id="${id}"[\\s\\S]*?<\\/button>\\r?\\n`, 'g');
  return html.replace(re, '');
}

for (const file of files) {
  let s = fs.readFileSync(file, 'utf8');
  const orig = s;

  // 1. Xoá 2 section tab (kèm nội dung bên trong)
  s = removeDivById(s, 'cfgTabSecForm');
  s = removeDivById(s, 'cfgTabSecEvent');

  // 2. Xoá nút chuyển tab tương ứng
  s = removeButtonById(s, 'cfgTabBtnForm');
  s = removeButtonById(s, 'cfgTabBtnEvent');

  if (s !== orig) {
    fs.writeFileSync(file, s, 'utf8');
    console.log(`${file}: updated (${orig.length} -> ${s.length})`);
  } else {
    console.log(`${file}: no change`);
  }
}
