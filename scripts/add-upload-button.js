/**
 * Chèn nút "Tải lên" (upload file từ máy) vào header của File Explorer panel
 * trong studio.html — thực hiện bằng Node để giữ nguyên encoding UTF-8.
 *
 * Usage: node scripts/add-upload-button.js frontend/admin/studio.html
 */
const fs = require('fs');

const file = process.argv[2];
if (!file) {
  console.error('Usage: node add-upload-button.js <studio.html>');
  process.exit(1);
}

let s = fs.readFileSync(file, 'utf8');

const ANCHOR = `                        <button onclick="openNewFolderModal()" class="text-amber-400 hover:text-amber-300 text-xs px-1.5 py-0.5 rounded hover:bg-slate-800 transition flex items-center gap-1" title="Tạo thư mục mới">`;

const UPLOAD_BTN = `                        <button onclick="openStudioUploadModal()" class="text-emerald-400 hover:text-emerald-300 text-xs px-1.5 py-0.5 rounded hover:bg-slate-800 transition flex items-center gap-1" title="Tải tệp từ máy lên server (ảnh, video, âm thanh, tài liệu...)">
                            <i class="fa-solid fa-cloud-arrow-up text-[10px]"></i>
                            <span class="text-[10px] font-semibold">Tải lên</span>
                        </button>
`;

if (s.includes('openStudioUploadModal')) {
  console.log(`${file}: nút upload đã tồn tại — bỏ qua`);
  process.exit(0);
}

if (!s.includes(ANCHOR)) {
  console.error(`${file}: không tìm thấy anchor`);
  process.exit(1);
}

s = s.replace(ANCHOR, UPLOAD_BTN + ANCHOR);
fs.writeFileSync(file, s, 'utf8');
console.log(`${file}: đã chèn nút upload`);
