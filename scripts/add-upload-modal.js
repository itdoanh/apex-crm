/**
 * Chèn modal "Tải tệp từ máy lên server" vào studio.html.
 * Thực hiện bằng Node (giữ nguyên encoding UTF-8).
 *
 * Usage: node scripts/add-upload-modal.js frontend/admin/studio.html
 */
const fs = require('fs');

const file = process.argv[2];
if (!file) {
  console.error('Usage: node add-upload-modal.js <studio.html>');
  process.exit(1);
}

let s = fs.readFileSync(file, 'utf8');

if (s.includes('studioUploadModal')) {
  console.log(`${file}: modal upload đã tồn tại — bỏ qua`);
  process.exit(0);
}

const ANCHOR = `    <!-- Toast Notification Container -->`;

const MODAL = `    <!-- ============================================================== -->
    <!-- MODAL TẢI TỆP TỪ MÁY LÊN SERVER (File Manager)                -->
    <!-- ============================================================== -->
    <div id="studioUploadModal" class="fixed inset-0 z-[60] bg-black/80 backdrop-blur-sm hidden flex items-center justify-center p-4">
        <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-md w-full p-5 space-y-4 shadow-2xl">
            <div class="flex items-center justify-between border-b border-slate-800 pb-3">
                <h4 class="font-bold text-sm text-white flex items-center gap-2">
                    <i class="fa-solid fa-cloud-arrow-up text-emerald-400"></i>
                    <span>Tải Tệp Từ Máy Lên Server</span>
                </h4>
                <button onclick="closeStudioUploadModal()" class="text-slate-400 hover:text-white"><i class="fa-solid fa-xmark"></i></button>
            </div>
            <div>
                <label class="block text-xs font-bold text-slate-300 mb-1">Thư Mục Đích Trong Landing (Folder)</label>
                <select id="studioUploadFolder" class="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-xs text-white font-mono focus:outline-none focus:border-cyan-500">
                    <option value="">(Thư mục gốc /)</option>
                </select>
                <span class="text-[10px] text-slate-500 mt-1 block">File sẽ được lưu vào bảng <code>landing_files</code> và tự động deploy khi bấm "Deploy".</span>
            </div>
            <div>
                <label class="block text-xs font-bold text-slate-300 mb-1">Chọn Tệp Từ Máy</label>
                <input type="file" id="studioUploadInput" multiple
                    class="w-full text-xs text-slate-400 file:mr-3 file:rounded-lg file:border-0 file:bg-emerald-600 file:px-3 file:py-1.5 file:text-xs file:font-bold file:text-white hover:file:bg-emerald-500 cursor-pointer">
                <span class="text-[10px] text-slate-500 mt-1 block">Hỗ trợ: ảnh (jpg, png, webp, svg, gif, ico), video (mp4, webm), âm thanh (mp3, wav, ogg, m4a), tài liệu (pdf), font (woff2, ttf). Tối đa 100MB/tệp.</span>
            </div>
            <div id="studioUploadProgress" class="hidden space-y-1.5">
                <div class="h-1.5 bg-slate-800 rounded-full overflow-hidden">
                    <div id="studioUploadBar" class="h-full bg-emerald-500 transition-all duration-200" style="width:0%"></div>
                </div>
                <div id="studioUploadStatus" class="text-[10px] text-slate-400 font-mono">Đang tải lên...</div>
            </div>
            <div class="flex items-center justify-end space-x-2 pt-2">
                <button onclick="closeStudioUploadModal()" class="bg-slate-800 text-slate-300 text-xs px-3 py-1.5 rounded-lg hover:bg-slate-700">Hủy</button>
                <button onclick="submitStudioUpload()" id="btnStudioUploadConfirm" class="bg-emerald-600 hover:bg-emerald-500 text-white font-bold text-xs px-4 py-1.5 rounded-lg transition">Tải Lên</button>
            </div>
        </div>
    </div>

`;

if (!s.includes(ANCHOR)) {
  console.error(`${file}: không tìm thấy anchor`);
  process.exit(1);
}

s = s.replace(ANCHOR, MODAL + ANCHOR);
fs.writeFileSync(file, s, 'utf8');
console.log(`${file}: đã chèn modal upload`);
