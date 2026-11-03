/* ==============================================================
   APEX VISUAL WEB BUILDER - PHOTOSHOP HISTORY STACK & TIME TRAVEL
   50-Step Undo / Redo Snapshot Stack with Branch Pruning
   ============================================================== */

let MAX_HISTORY_STEPS = 50;
let studioHistory = [];
let studioHistoryIndex = -1;
let isApplyingHistory = false;

        // ============================================================
        function pushStudioHistory(actionTitle, code, nodeId = null) {
            if (isApplyingHistory || !code) return;
            const currentHead = studioHistory[studioHistoryIndex];
            if (currentHead && currentHead.code === code) return;

            // Cắt tỉa nhánh redo phía sau nếu đang đứng ở giữa lịch sử
            if (studioHistoryIndex < studioHistory.length - 1) {
                studioHistory = studioHistory.slice(0, studioHistoryIndex + 1);
            }

            const now = new Date();
            const timeStr = now.toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit', second: '2-digit' });

            studioHistory.push({
                name: actionTitle || 'Thao tác chỉnh sửa',
                code: code,
                nodeId: nodeId || selectedAstNodeId,
                time: timeStr
            });

            if (studioHistory.length > MAX_HISTORY_STEPS) {
                studioHistory.shift();
            }

            studioHistoryIndex = studioHistory.length - 1;
            updateStudioHistoryUI();
        }

        function updateStudioHistoryUI() {
            const btnUndo = document.getElementById('btnStudioUndo');
            const btnRedo = document.getElementById('btnStudioRedo');
            const badge = document.getElementById('csHistoryBadge');
            const totalBadge = document.getElementById('csHistoryTotalBadge');

            const canUndo = studioHistoryIndex > 0;
            const canRedo = studioHistoryIndex < studioHistory.length - 1;

            if (btnUndo) {
                btnUndo.disabled = !canUndo;
                if (canUndo) {
                    btnUndo.className = 'px-2 py-1 rounded-lg font-bold transition flex items-center gap-1 text-slate-200 hover:text-white hover:bg-slate-800 cursor-pointer';
                } else {
                    btnUndo.className = 'px-2 py-1 rounded-lg font-bold transition flex items-center gap-1 text-slate-600 cursor-not-allowed opacity-40';
                }
            }

            if (btnRedo) {
                btnRedo.disabled = !canRedo;
                if (canRedo) {
                    btnRedo.className = 'px-2 py-1 rounded-lg font-bold transition flex items-center gap-1 text-slate-200 hover:text-white hover:bg-slate-800 cursor-pointer';
                } else {
                    btnRedo.className = 'px-2 py-1 rounded-lg font-bold transition flex items-center gap-1 text-slate-600 cursor-not-allowed opacity-40';
                }
            }

            const currentStepNum = studioHistoryIndex >= 0 ? studioHistoryIndex + 1 : 0;
            const totalSteps = studioHistory.length;
            if (badge) badge.innerText = `${currentStepNum}/${totalSteps}`;
            if (totalBadge) totalBadge.innerText = `${totalSteps} / ${MAX_HISTORY_STEPS}`;

            const dropdown = document.getElementById('csHistoryDropdown');
            if (dropdown && !dropdown.classList.contains('hidden')) {
                renderStudioHistoryList();
            }
        }

        function studioUndo() {
            if (studioHistoryIndex <= 0) return;
            applyHistoryStep(studioHistoryIndex - 1);
        }

        function studioRedo() {
            if (studioHistoryIndex >= studioHistory.length - 1) return;
            applyHistoryStep(studioHistoryIndex + 1);
        }

        function applyHistoryStep(index) {
            if (index < 0 || index >= studioHistory.length) return;
            isApplyingHistory = true;
            studioHistoryIndex = index;
            const step = studioHistory[index];

            studioFileContents[activeStudioFile] = step.code;
            const editor = document.getElementById('csCodeEditor');
            if (editor) editor.value = step.code;
            updateEditorStats(false);

            astMapper.parse(step.code);
            refreshStudioPreview();

            if (step.nodeId && astMapper.astMap[step.nodeId]) {
                selectAstNode(step.nodeId);
            } else {
                selectedAstNodeId = null;
                const gizmo = document.getElementById('csGizmoBox');
                if (gizmo) gizmo.classList.add('hidden');
                const gizmoToolbar = document.getElementById('csGizmoToolbar');
                if (gizmoToolbar) gizmoToolbar.classList.add('hidden');
            }

            renderAstLayerTree();
            updateStudioHistoryUI();
            showToastNotification(`Khôi phục: ${step.name}`);
            isApplyingHistory = false;
        }

        function toggleStudioHistoryDropdown() {
            const dropdown = document.getElementById('csHistoryDropdown');
            if (!dropdown) return;
            dropdown.classList.toggle('hidden');
            if (!dropdown.classList.contains('hidden')) {
                renderStudioHistoryList();
            }
        }

        function renderStudioHistoryList() {
            const container = document.getElementById('csHistoryListContainer');
            if (!container) return;
            if (studioHistory.length === 0) {
                container.innerHTML = '<div class="text-slate-500 text-xs italic p-3 text-center">Chưa có lịch sử thao tác</div>';
                return;
            }

            let html = '';
            for (let i = studioHistory.length - 1; i >= 0; i--) {
                const step = studioHistory[i];
                const isActive = i === studioHistoryIndex;
                const isFuture = i > studioHistoryIndex;

                let icon = 'fa-solid fa-clock-rotate-left';
                if (i === 0) icon = 'fa-solid fa-file-code';
                else if (step.name.includes('Sửa chữ')) icon = 'fa-solid fa-pen-to-square';
                else if (step.name.includes('style') || step.name.includes('Style')) icon = 'fa-solid fa-brush';
                else if (step.name.includes('class') || step.name.includes('Class')) icon = 'fa-solid fa-tags';
                else if (step.name.includes('thẻ') || step.name.includes('tag')) icon = 'fa-solid fa-code';
                else if (step.name.includes('Nhân bản')) icon = 'fa-solid fa-clone';
                else if (step.name.includes('Xóa')) icon = 'fa-solid fa-trash-can';
                else if (step.name.includes('Dán')) icon = 'fa-solid fa-paste';
                else if (step.name.includes('chuyển') || step.name.includes('Kéo')) icon = 'fa-solid fa-up-down-left-right';

                html += `
                    <div onclick="applyHistoryStep(${i})"
                         class="px-2.5 py-1.5 rounded-xl cursor-pointer transition flex items-center justify-between text-xs select-none ${isActive ? 'bg-cyan-950/80 text-cyan-200 border border-cyan-700/80 font-bold shadow' : (isFuture ? 'text-slate-600 hover:bg-slate-800/50 hover:text-slate-400 opacity-60' : 'text-slate-300 hover:bg-slate-800 hover:text-white')}">
                        <div class="flex items-center gap-2 truncate">
                            <span class="text-[9px] font-mono ${isActive ? 'text-cyan-400 font-bold' : 'text-slate-500'}">#${i + 1}</span>
                            <i class="${icon} text-[10px] ${isActive ? 'text-cyan-400' : 'text-slate-500'}"></i>
                            <span class="truncate">${escapeHTML(step.name)}</span>
                        </div>
                        <div class="flex items-center gap-1.5 flex-shrink-0 text-[10px] text-slate-500 font-mono">
                            ${isActive ? '<span class="text-[8px] bg-cyan-500 text-slate-950 font-black px-1.5 py-0.2 rounded-full uppercase">Hiện tại</span>' : ''}
                            <span>${step.time}</span>
                        </div>
                    </div>
                `;
            }
            container.innerHTML = html;
        }

