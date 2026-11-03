/* ==============================================================
   APEX VISUAL WEB BUILDER - SELECTION GIZMO & OVERLAY RESIZE/DRAG
   ============================================================== */

        // ============================================================
        // VISUAL GIZMO DRAG & RESIZE CONTROLLER (POINTER CAPTURE & SHIELD)
        // ============================================================
        let resizeCurrentW = 0, resizeCurrentH = 0;
        let moveCurrentDx = 0, moveCurrentDy = 0;

        // Kéo biến đổi kích thước (Resize Handles 60 FPS + PointerCapture)
        function startGizmoResize(e, dir) {
            e.preventDefault();
            e.stopPropagation();

            if (!selectedAstNodeId) return;
            if (studioLockedNodes.has(selectedAstNodeId)) {
                showToastNotification('Layer này đang bị khóa! Mở khóa trong menu hoặc Cây Layer để co dãn.');
                return;
            }
            const iframe = document.getElementById('csPreviewIframe');
            const gizmoBox = document.getElementById('csGizmoBox');
            if (!iframe || !gizmoBox) return;

            isDraggingGizmo = true;
            isMovingGizmo = false;
            currentResizeDir = dir;
            resizeStartX = e.clientX;
            resizeStartY = e.clientY;

            resizeStartRect = {
                left: parseFloat(gizmoBox.style.left) || 0,
                top: parseFloat(gizmoBox.style.top) || 0,
                width: parseFloat(gizmoBox.style.width) || 0,
                height: parseFloat(gizmoBox.style.height) || 0
            };
            resizeCurrentW = resizeStartRect.width;
            resizeCurrentH = resizeStartRect.height;

            // 1. Kích hoạt Pointer Capture trên handle để giữ chặt con trỏ bất kể di chuyển đi đâu
            activeCapturedElement = e.currentTarget || e.target;
            activePointerId = e.pointerId;
            if (activeCapturedElement && typeof activeCapturedElement.setPointerCapture === 'function' && e.pointerId !== undefined) {
                try { activeCapturedElement.setPointerCapture(e.pointerId); } catch(err) {}
            }

            // 2. Phủ tấm chắn trong suốt ngăn chặn iframe nuốt sự kiện chuột
            const shield = document.getElementById('csDragShield');
            if (shield) shield.classList.remove('hidden');
            iframe.style.pointerEvents = 'none';

            // 3. Đổi con trỏ trên toàn trang phù hợp với hướng kéo
            document.body.style.cursor = (dir.includes('nw') || dir.includes('se')) ? 'nwse-resize' :
                                         (dir.includes('ne') || dir.includes('sw')) ? 'nesw-resize' :
                                         (dir.includes('e')  || dir.includes('w'))  ? 'ew-resize' : 'ns-resize';
        }

        // Kéo di chuyển phần tử (Drag to Move Element Live 60 FPS)
        function startGizmoMove(e) {
            e.preventDefault();
            e.stopPropagation();

            if (!selectedAstNodeId) return;
            if (studioLockedNodes.has(selectedAstNodeId)) {
                showToastNotification('Layer này đang bị khóa! Mở khóa trong menu hoặc Cây Layer để kéo.');
                return;
            }
            const iframe = document.getElementById('csPreviewIframe');
            const gizmoBox = document.getElementById('csGizmoBox');
            if (!iframe || !gizmoBox) return;

            isMovingGizmo = true;
            isDraggingGizmo = false;
            moveStartX = e.clientX;
            moveStartY = e.clientY;
            moveCurrentDx = 0;
            moveCurrentDy = 0;

            moveStartLeft = parseFloat(gizmoBox.style.left) || 0;
            moveStartTop = parseFloat(gizmoBox.style.top) || 0;

            // Đọc vị trí top/left ban đầu của node từ AST mapper
            const node = astMapper.astMap[selectedAstNodeId];
            let initL = 0, initT = 0;
            if (node && node.attrs && node.attrs['style']) {
                const sVal = node.attrs['style'].value || '';
                const matchL = sVal.match(/left:\s*(-?[\d.]+)px/i);
                const matchT = sVal.match(/top:\s*(-?[\d.]+)px/i);
                if (matchL) initL = parseFloat(matchL[1]) || 0;
                if (matchT) initT = parseFloat(matchT[1]) || 0;
            }
            moveInitialStyleLeft = initL;
            moveInitialStyleTop = initT;

            // 1. Kích hoạt Pointer Capture
            activeCapturedElement = e.currentTarget || e.target;
            activePointerId = e.pointerId;
            if (activeCapturedElement && typeof activeCapturedElement.setPointerCapture === 'function' && e.pointerId !== undefined) {
                try { activeCapturedElement.setPointerCapture(e.pointerId); } catch(err) {}
            }

            // 2. Bật tấm chắn ngăn iframe chiếm sự kiện
            const shield = document.getElementById('csDragShield');
            if (shield) shield.classList.remove('hidden');
            iframe.style.pointerEvents = 'none';

            // 3. Đổi con trỏ sang grabbing
            document.body.style.cursor = 'grabbing';
        }

        function handleGizmoPointerMove(ev) {
            if (!isDraggingGizmo && !isMovingGizmo) return;
            const gizmoBox = document.getElementById('csGizmoBox');
            const iframe = document.getElementById('csPreviewIframe');
            if (!gizmoBox || !iframe || !iframe.contentWindow) return;

            // TRƯỜNG HỢP 1: KÉO CO GIÃN KÍCH THƯỚC (RESIZE)
            if (isDraggingGizmo && resizeStartRect) {
                const dx = ev.clientX - resizeStartX;
                const dy = ev.clientY - resizeStartY;
                const dir = currentResizeDir;

                let newW = resizeStartRect.width;
                let newH = resizeStartRect.height;
                let newL = resizeStartRect.left;
                let newT = resizeStartRect.top;

                if (dir.includes('e')) newW = Math.max(20, resizeStartRect.width + dx);
                if (dir.includes('s')) newH = Math.max(10, resizeStartRect.height + dy);
                if (dir.includes('w')) {
                    const possibleW = resizeStartRect.width - dx;
                    if (possibleW >= 20) {
                        newW = possibleW;
                        newL = resizeStartRect.left + dx;
                    }
                }
                if (dir.includes('n')) {
                    const possibleH = resizeStartRect.height - dy;
                    if (possibleH >= 10) {
                        newH = possibleH;
                        newT = resizeStartRect.top + dy;
                    }
                }

                // Giữ tỷ lệ khung hình (Aspect Ratio) khi nhấn Shift
                if (ev.shiftKey && resizeStartRect.height > 0) {
                    const ratio = resizeStartRect.width / resizeStartRect.height;
                    if (dir === 'e' || dir === 'w') {
                        newH = Math.max(10, newW / ratio);
                    } else {
                        newW = Math.max(20, newH * ratio);
                    }
                }

                resizeCurrentW = Math.round(newW);
                resizeCurrentH = Math.round(newH);

                gizmoBox.style.width = newW + 'px';
                gizmoBox.style.height = newH + 'px';
                gizmoBox.style.left = newL + 'px';
                gizmoBox.style.top = newT + 'px';

                const sizeTag = document.getElementById('csGizmoSize');
                if (sizeTag) sizeTag.innerText = `${resizeCurrentW} × ${resizeCurrentH}px`;

                // Cập nhật giá trị vào Inspector inputs
                const wInput = document.getElementById('inspStyleWidth');
                const hInput = document.getElementById('inspStyleHeight');
                if (wInput) wInput.value = resizeCurrentW + 'px';
                if (hInput) hInput.value = resizeCurrentH + 'px';

                // Bắn cập nhật 60 FPS sang iframe preview
                iframe.contentWindow.postMessage({
                    type: 'APEX_CMD_UPDATE_STYLE_TEMP',
                    nodeId: selectedAstNodeId,
                    styles: {
                        width: resizeCurrentW + 'px',
                        height: resizeCurrentH + 'px'
                    }
                }, '*');
            }

            // TRƯỜNG HỢP 2: KÉO DI CHUYỂN VỊ TRÍ (DRAG TO MOVE)
            if (isMovingGizmo) {
                const dx = ev.clientX - moveStartX;
                const dy = ev.clientY - moveStartY;
                moveCurrentDx = dx;
                moveCurrentDy = dy;

                gizmoBox.style.left = (moveStartLeft + dx) + 'px';
                gizmoBox.style.top = (moveStartTop + dy) + 'px';

                const targetLeft = Math.round(moveInitialStyleLeft + dx);
                const targetTop = Math.round(moveInitialStyleTop + dy);

                // Cập nhật trực tiếp lên Inspector
                const posInput = document.getElementById('inspStylePosition');
                const lInput = document.getElementById('inspStyleLeft');
                const tInput = document.getElementById('inspStyleTop');
                if (posInput && !posInput.value) posInput.value = 'relative';
                if (lInput) lInput.value = targetLeft + 'px';
                if (tInput) tInput.value = targetTop + 'px';

                // Bắn cập nhật 60 FPS sang iframe preview
                iframe.contentWindow.postMessage({
                    type: 'APEX_CMD_UPDATE_STYLE_TEMP',
                    nodeId: selectedAstNodeId,
                    styles: {
                        position: 'relative',
                        left: targetLeft + 'px',
                        top: targetTop + 'px'
                    }
                }, '*');
            }
        }

        // Dập tắt và giải phóng toàn bộ trạng thái kéo (End Drag & Resize Safely)
        function endGizmoDragOrResize() {
            if (!isDraggingGizmo && !isMovingGizmo) return;

            const wasResizing = isDraggingGizmo;
            const wasMoving = isMovingGizmo;

            isDraggingGizmo = false;
            isMovingGizmo = false;
            currentResizeDir = '';

            // 1. Giải phóng Pointer Capture an toàn
            if (activeCapturedElement && activePointerId !== null && typeof activeCapturedElement.releasePointerCapture === 'function') {
                try { activeCapturedElement.releasePointerCapture(activePointerId); } catch(err) {}
            }
            activeCapturedElement = null;
            activePointerId = null;

            // 2. Tắt tấm chắn chuột và trả lại tương tác cho iframe
            const shield = document.getElementById('csDragShield');
            if (shield) shield.classList.add('hidden');

            const iframe = document.getElementById('csPreviewIframe');
            if (iframe) iframe.style.pointerEvents = 'auto';

            // 3. Khôi phục con trỏ chuột
            document.body.style.cursor = '';

            // 4. Lưu thay đổi chính thức vào AST Source Mapper
            if (selectedAstNodeId) {
                if (wasResizing) {
                    const finalW = resizeCurrentW || Math.round(parseFloat(document.getElementById('csGizmoBox').style.width));
                    const finalH = resizeCurrentH || Math.round(parseFloat(document.getElementById('csGizmoBox').style.height));
                    applyAstStyleChange(selectedAstNodeId, {
                        width: finalW + 'px',
                        height: finalH + 'px'
                    }, 'Co dãn kích thước');
                } else if (wasMoving) {
                    const finalLeft = Math.round(moveInitialStyleLeft + moveCurrentDx);
                    const finalTop = Math.round(moveInitialStyleTop + moveCurrentDy);
                    applyAstStyleChange(selectedAstNodeId, {
                        position: 'relative',
                        left: finalLeft + 'px',
                        top: finalTop + 'px'
                    }, 'Kéo di chuyển');
                }

                // Cập nhật lại khung gizmo chuẩn xác theo vị trí mới
                setTimeout(() => {
                    if (selectedAstNodeId) requestNodeRect(selectedAstNodeId);
                }, 150);
            }
        }

        // Đăng ký các lá chắn sự kiện cấp Window toàn diện chống dính chuột 100%
        window.addEventListener('pointermove', handleGizmoPointerMove);
        window.addEventListener('pointerup', endGizmoDragOrResize);
        window.addEventListener('pointercancel', endGizmoDragOrResize);
        window.addEventListener('mouseup', endGizmoDragOrResize);
        window.addEventListener('blur', endGizmoDragOrResize);

        // Sửa chữ trực tiếp tại chỗ (Inline Text Editing)
        function startInlineTextEdit() {
            if (!selectedAstNodeId) return;
            const iframe = document.getElementById('csPreviewIframe');
            if (iframe && iframe.contentWindow) {
                iframe.contentWindow.postMessage({ type: 'APEX_CMD_START_INLINE_EDIT', nodeId: selectedAstNodeId }, '*');
            }
        }

        // Toast Notification thông báo thao tác trong Visual Web Builder
        function showToastNotification(message, type = 'info') {
            let toast = document.getElementById('csStudioToast');
            if (!toast) {
                toast = document.createElement('div');
                toast.id = 'csStudioToast';
                toast.className = 'fixed bottom-8 left-1/2 -translate-x-1/2 z-[120] bg-slate-900/95 text-slate-100 px-4 py-2 rounded-xl shadow-2xl border border-slate-700/80 backdrop-blur-md flex items-center gap-2 text-xs font-medium transition-all duration-300 pointer-events-none opacity-0 translate-y-3';
                document.body.appendChild(toast);
            }
            let icon = '<i class="fa-solid fa-circle-check text-cyan-400"></i>';
            if (type === 'warn') icon = '<i class="fa-solid fa-triangle-exclamation text-amber-400"></i>';
            else if (type === 'error') icon = '<i class="fa-solid fa-circle-xmark text-rose-400"></i>';

            toast.innerHTML = `${icon}<span>${escapeHTML(message)}</span>`;
            toast.classList.remove('opacity-0', 'translate-y-3');
            toast.classList.add('opacity-100', 'translate-y-0');

            clearTimeout(window.__csToastTimeout);
            window.__csToastTimeout = setTimeout(() => {
                toast.classList.remove('opacity-100', 'translate-y-0');
                toast.classList.add('opacity-0', 'translate-y-3');
            }, 2400);
        }

