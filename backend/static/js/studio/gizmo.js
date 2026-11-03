/* ==============================================================
   APEX VISUAL WEB BUILDER - SELECTION GIZMO & OVERLAY RESIZE/DRAG
   ============================================================== */

        // ============================================================
        // VISUAL GIZMO DRAG & RESIZE CONTROLLER (POINTER CAPTURE & SHIELD)
        // ============================================================
        var isDraggingGizmo = false;
        var isMovingGizmo = false;
        var currentResizeDir = '';
        var resizeStartRect = null;
        var resizeStartX = 0, resizeStartY = 0;
        var moveStartX = 0, moveStartY = 0;
        var moveStartLeft = 0, moveStartTop = 0;
        var moveInitialStyleLeft = 0, moveInitialStyleTop = 0;
        var activeCapturedElement = null, activePointerId = null;
        var resizeCurrentW = 0, resizeCurrentH = 0;
        var moveCurrentDx = 0, moveCurrentDy = 0;

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

            // Đọc vị trí top/left ban đầu của node từ AST mapper (Phân biệt độc lập PC vs Mobile)
            const node = astMapper.astMap[selectedAstNodeId];
            let initL = 0, initT = 0;
            if (studioDeviceMode === 'mobile') {
                const mob = astMapper.getMobileStyles(selectedAstNodeId);
                if (mob.left !== undefined) initL = parseFloat(mob.left) || 0;
                else if (node && node.attrs && node.attrs['style']) {
                    const matchL = (node.attrs['style'].value || '').match(/left:\s*(-?[\d.]+)px/i);
                    if (matchL) initL = parseFloat(matchL[1]) || 0;
                }
                if (mob.top !== undefined) initT = parseFloat(mob.top) || 0;
                else if (node && node.attrs && node.attrs['style']) {
                    const matchT = (node.attrs['style'].value || '').match(/top:\s*(-?[\d.]+)px/i);
                    if (matchT) initT = parseFloat(matchT[1]) || 0;
                }
            } else {
                const pc = astMapper.getPCStyles(selectedAstNodeId);
                if (pc.left !== undefined) initL = parseFloat(pc.left) || 0;
                else if (node && node.attrs && node.attrs['style']) {
                    const sVal = node.attrs['style'].value || '';
                    const matchL = sVal.match(/left:\s*(-?[\d.]+)px/i);
                    if (matchL) initL = parseFloat(matchL[1]) || 0;
                }
                if (pc.top !== undefined) initT = parseFloat(pc.top) || 0;
                else if (node && node.attrs && node.attrs['style']) {
                    const sVal = node.attrs['style'].value || '';
                    const matchT = sVal.match(/top:\s*(-?[\d.]+)px/i);
                    if (matchT) initT = parseFloat(matchT[1]) || 0;
                }
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

                // Xóa sạch inline styles tạm thời khỏi DOM iframe trên cả PC và Mobile
                // để phần tử hoàn toàn tuân theo Media Query tương ứng (@media min-width 769px / max-width 768px),
                // triệt tiêu 100% hiện tượng inline styles rò rỉ làm ảnh hưởng thiết bị còn lại.
                const iframe = document.getElementById('csPreviewIframe');
                if (iframe && iframe.contentWindow) {
                    iframe.contentWindow.postMessage({
                        type: 'APEX_CMD_CLEAR_TEMP_STYLES',
                        nodeId: selectedAstNodeId,
                        props: ['left', 'top', 'position', 'width', 'height']
                    }, '*');
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

        /* ==============================================================
           CANVAS INTERACTION CONTROLLER: 2-SIDE RESIZING, ZOOM & PAN
           Boundary Clamped Panning, Space Grab/Grabbing Hand, 60fps Transforms
           ============================================================== */

        let canvasZoom = 1.0;
        let canvasPanX = 0;
        let canvasPanY = 0;
        let isCanvasPanning = false;
        let isSpaceKeyPressed = false;
        let isCtrlKeyPressed = false;
        let panStartX = 0, panStartY = 0;
        let panStartPanX = 0, panStartPanY = 0;

        let isResizingCanvasWidth = false;
        let canvasResizeHandleDir = ''; // 'left' | 'right'
        let canvasResizeStartX = 0;
        let canvasResizeStartWidth = 0;

        // Tính toán giới hạn kéo (Boundary Clamping)
        function getCanvasPanLimits() {
            const viewport = document.getElementById('csCanvasViewport');
            const wrapper = document.getElementById('csIframeWrapper');
            if (!viewport || !wrapper) return { maxPanX: 0, maxPanY: 0, canPanX: false, canPanY: false };

            const vw = viewport.clientWidth;
            const vh = viewport.clientHeight;
            const ww = wrapper.offsetWidth;
            const wh = wrapper.offsetHeight;

            // Chiều ngang: Chỉ cho phép kéo khi khung hiển thị vượt ra ngoài khung nhìn
            let maxPanX = 0;
            let canPanX = false;
            if (ww > vw) {
                maxPanX = (ww - vw) / 2 + 100;
                canPanX = true;
            }

            // Chiều dọc: Chỉ cho phép kéo khi chiều cao vượt quá khung nhìn
            let maxPanY = 0;
            let canPanY = false;
            if (wh > vh) {
                maxPanY = (wh - vh) / 2 + 100;
                canPanY = true;
            }

            return { maxPanX, maxPanY, canPanX, canPanY };
        }

        function clampCanvasPan(panX, panY, zoom = canvasZoom) {
            const limits = getCanvasPanLimits();
            let clampedX = 0;
            let clampedY = 0;

            if (limits.canPanX) {
                clampedX = Math.max(-limits.maxPanX, Math.min(limits.maxPanX, panX));
            }
            if (limits.canPanY) {
                clampedY = Math.max(-limits.maxPanY, Math.min(limits.maxPanY, panY));
            }
            return { x: clampedX, y: clampedY };
        }

        function applyCanvasTransform() {
            const wrapper = document.getElementById('csIframeWrapper');
            const lbl = document.getElementById('lblZoomLevel');
            const iframe = document.getElementById('csPreviewIframe');
            if (!wrapper) return;

            // Khung ngoài luôn giữ nguyên vị trí cố định, tuyệt đối không di chuyển hay làm méo khung
            wrapper.style.transform = 'none';

            // Phóng to / nhỏ nội dung TRONG khung preview (Inner Content Zoom inside Iframe)
            if (iframe && iframe.contentWindow) {
                iframe.contentWindow.postMessage({ type: 'APEX_CMD_SET_CONTENT_ZOOM', zoom: canvasZoom }, '*');
            }

            if (lbl) {
                lbl.innerText = `${Math.round(canvasZoom * 100)}%`;
            }

            // Cập nhật lại vị trí gizmo nếu đang có node được chọn
            if (selectedAstNodeId) {
                requestNodeRect(selectedAstNodeId);
            }
        }

        function setCanvasZoom(targetZoom) {
            const newZoom = Math.max(0.2, Math.min(3.0, Math.round(targetZoom * 100) / 100));
            canvasZoom = newZoom;
            applyCanvasTransform();
        }

        function stepCanvasZoom(delta) {
            setCanvasZoom(canvasZoom + delta);
        }

        function resetCanvasZoom() {
            canvasZoom = 1.0;
            canvasPanX = 0;
            canvasPanY = 0;
            applyCanvasTransform();
            showToastNotification('Đã đặt lại kích thước xem 100%');
        }

        // Điều khiển trạng thái phím Space (Phím cách) - Chuyển con trỏ sang bàn tay grab & bật lá chắn Pan Shield
        function setSpacePanState(active) {
            isSpaceKeyPressed = active;
            const viewport = document.getElementById('csCanvasViewport');
            const shield = document.getElementById('csCanvasPanShield');

            if (active) {
                if (viewport) viewport.classList.add('cs-pan-grab');
                if (shield) {
                    shield.classList.remove('hidden');
                    shield.classList.add('cs-pan-grab');
                }
            } else {
                if (!isCanvasPanning) {
                    if (viewport) viewport.classList.remove('cs-pan-grab', 'cs-pan-grabbing');
                    if (shield) {
                        shield.classList.remove('cs-pan-grab', 'cs-pan-grabbing');
                        shield.classList.add('hidden');
                    }
                }
            }
        }

        function setCtrlPanState(active) {
            isCtrlKeyPressed = active;
            setSpacePanState(active || isSpaceKeyPressed);
        }

        window.setSpacePanState = setSpacePanState;
        window.setCtrlPanState = setCtrlPanState;

        // Bắt đầu kéo cuộn nội dung trong iframe bằng phím Space + kéo chuột (Inner Content Pan)
        function startCanvasPan(e) {
            if (isResizingCanvasWidth || isDraggingGizmo || isMovingGizmo) return;
            const isPanTrigger = isSpaceKeyPressed || e.code === 'Space' || e.button === 1 || isCtrlKeyPressed;
            if (!isPanTrigger) return;

            const iframe = document.getElementById('csPreviewIframe');
            let canScrollX = false;
            let canScrollY = false;
            if (iframe && iframe.contentWindow) {
                try {
                    const doc = iframe.contentDocument || iframe.contentWindow.document;
                    if (doc) {
                        const docEl = doc.documentElement;
                        const body = doc.body;
                        const scrollW = Math.max(docEl.scrollWidth || 0, body ? body.scrollWidth : 0);
                        const scrollH = Math.max(docEl.scrollHeight || 0, body ? body.scrollHeight : 0);
                        const clientW = docEl.clientWidth || iframe.clientWidth;
                        const clientH = docEl.clientHeight || iframe.clientHeight;
                        canScrollX = scrollW > clientW + 2;
                        canScrollY = scrollH > clientH + 2;
                    }
                } catch(err) {
                    canScrollX = true;
                    canScrollY = true;
                }
            }

            if (!canScrollX && !canScrollY) {
                showToastNotification('Nội dung đang hiển thị trọn vẹn, không có phần khuất để cuộn', 'info');
                return;
            }

            e.preventDefault();
            e.stopPropagation();

            isCanvasPanning = true;
            panStartX = e.clientX;
            panStartY = e.clientY;

            const viewport = document.getElementById('csCanvasViewport');
            const shield = document.getElementById('csCanvasPanShield');

            if (viewport) {
                viewport.classList.remove('cs-pan-grab');
                viewport.classList.add('cs-pan-grabbing');
            }
            if (shield) {
                shield.classList.remove('hidden', 'cs-pan-grab');
                shield.classList.add('cs-pan-grabbing');
                if (typeof shield.setPointerCapture === 'function' && e.pointerId !== undefined) {
                    try { shield.setPointerCapture(e.pointerId); } catch(err) {}
                }
            }
            document.body.style.cursor = 'grabbing';
        }

        function handleCanvasPanMove(e) {
            if (!isCanvasPanning) return;
            e.preventDefault();

            const dx = e.clientX - panStartX;
            const dy = e.clientY - panStartY;
            panStartX = e.clientX;
            panStartY = e.clientY;

            const iframe = document.getElementById('csPreviewIframe');
            if (iframe && iframe.contentWindow) {
                try {
                    iframe.contentWindow.scrollBy({ left: -dx, top: -dy, behavior: 'instant' });
                } catch(err) {
                    iframe.contentWindow.postMessage({ type: 'APEX_CMD_SCROLL_PAN', dx: -dx, dy: -dy }, '*');
                }
            }

            if (selectedAstNodeId) {
                requestNodeRect(selectedAstNodeId);
            }
        }

        function endCanvasPan() {
            if (!isCanvasPanning) return;
            isCanvasPanning = false;

            const viewport = document.getElementById('csCanvasViewport');
            const shield = document.getElementById('csCanvasPanShield');

            if (viewport) viewport.classList.remove('cs-pan-grabbing');
            if (shield) shield.classList.remove('cs-pan-grabbing');
            document.body.style.cursor = '';

            if (isSpaceKeyPressed || isCtrlKeyPressed) {
                if (viewport) viewport.classList.add('cs-pan-grab');
                if (shield) shield.classList.add('cs-pan-grab');
            } else {
                if (shield) shield.classList.add('hidden');
            }

            if (selectedAstNodeId) {
                requestNodeRect(selectedAstNodeId);
            }
        }

        // Điều chỉnh kích thước khung Iframe bằng 2 thanh kéo 2 bên (Side Handles)
        function startCanvasWidthResize(e, dir) {
            e.preventDefault();
            e.stopPropagation();

            const wrapper = document.getElementById('csIframeWrapper');
            if (!wrapper) return;

            isResizingCanvasWidth = true;
            canvasResizeHandleDir = dir;
            canvasResizeStartX = e.clientX;
            canvasResizeStartWidth = wrapper.offsetWidth;

            const badge = document.getElementById('csCanvasWidthBadge');
            if (badge) {
                badge.classList.remove('hidden');
                badge.innerText = `${Math.round(canvasResizeStartWidth)}px`;
            }

            const shield = document.getElementById('csDragShield');
            if (shield) shield.classList.remove('hidden');
            document.body.style.cursor = 'ew-resize';

            if (typeof e.target.setPointerCapture === 'function' && e.pointerId !== undefined) {
                try { e.target.setPointerCapture(e.pointerId); } catch(err) {}
            }
        }

        function handleCanvasWidthResize(e) {
            if (!isResizingCanvasWidth) return;
            const wrapper = document.getElementById('csIframeWrapper');
            const badge = document.getElementById('csCanvasWidthBadge');
            if (!wrapper) return;

            // Delta kéo đối xứng 2 bên không phụ thuộc flexbox hay vị trí tâm viewport
            const dx = e.clientX - canvasResizeStartX;
            let newWidth = canvasResizeStartWidth;
            if (canvasResizeHandleDir === 'right') {
                newWidth = canvasResizeStartWidth + dx * 2;
            } else if (canvasResizeHandleDir === 'left') {
                newWidth = canvasResizeStartWidth - dx * 2;
            }

            // Giới hạn trong khoảng 320px đến 3840px (Màn hình 4K siêu rộng, tự do co dãn)
            newWidth = Math.max(320, Math.min(3840, Math.round(newWidth)));
            wrapper.style.width = newWidth + 'px';
            wrapper.style.maxWidth = 'none';
            wrapper.style.flexShrink = '0';

            if (badge) {
                badge.innerText = `${newWidth}px`;
                badge.classList.remove('hidden');
            }

            applyCanvasTransform();
        }

        function endCanvasWidthResize() {
            if (!isResizingCanvasWidth) return;
            isResizingCanvasWidth = false;

            const shield = document.getElementById('csDragShield');
            if (shield) shield.classList.add('hidden');
            document.body.style.cursor = '';

            const badge = document.getElementById('csCanvasWidthBadge');
            if (badge) {
                setTimeout(() => {
                    if (!isResizingCanvasWidth) badge.classList.add('hidden');
                }, 1200);
            }

            if (selectedAstNodeId) {
                requestNodeRect(selectedAstNodeId);
            }
        }

        // Khởi tạo các sự kiện Canvas Interaction (Zoom, Pan, Resize Handles, Space Grab)
        function initCanvasInteractions() {
            const viewport = document.getElementById('csCanvasViewport');
            const shield = document.getElementById('csCanvasPanShield');
            const handleL = document.getElementById('csCanvasResizeHandleLeft');
            const handleR = document.getElementById('csCanvasResizeHandleRight');

            // 1. Gắn sự kiện cho 2 chốt kéo 2 bên mép
            if (handleL) {
                handleL.addEventListener('pointerdown', (e) => startCanvasWidthResize(e, 'left'));
            }
            if (handleR) {
                handleR.addEventListener('pointerdown', (e) => startCanvasWidthResize(e, 'right'));
            }

            // 2. Gắn sự kiện Pan trên lá chắn và viewport
            if (shield) {
                shield.addEventListener('pointerdown', startCanvasPan);
            }
            if (viewport) {
                viewport.addEventListener('pointerdown', startCanvasPan);
                // Ctrl + Wheel Zoom trên Canvas Viewport (Inner Content Zoom)
                viewport.addEventListener('wheel', (e) => {
                    if (e.ctrlKey || e.metaKey) {
                        e.preventDefault();
                        e.stopPropagation();
                        const delta = e.deltaY < 0 ? 0.08 : -0.08;
                        stepCanvasZoom(delta);
                    }
                }, { passive: false });
            }

            // 3. Lắng nghe nhấn giữ / thả phím Space (Phím cách) & Ctrl trên toàn cửa sổ Studio
            window.addEventListener('keydown', (e) => {
                if (e.code === 'Space' || e.key === ' ') {
                    const activeTag = document.activeElement ? document.activeElement.tagName.toLowerCase() : '';
                    const isEd = document.activeElement && document.activeElement.isContentEditable;
                    if (activeTag !== 'input' && activeTag !== 'textarea' && !isEd) {
                        e.preventDefault();
                        setSpacePanState(true);
                    }
                }
                if (e.key === 'Control' || e.key === 'Meta') {
                    const activeTag = document.activeElement ? document.activeElement.tagName.toLowerCase() : '';
                    if (activeTag !== 'input' && activeTag !== 'textarea') {
                        setCtrlPanState(true);
                    }
                }
            });

            window.addEventListener('keyup', (e) => {
                if (e.code === 'Space' || e.key === ' ') {
                    setSpacePanState(false);
                }
                if (e.key === 'Control' || e.key === 'Meta') {
                    setCtrlPanState(false);
                }
            });

            window.addEventListener('blur', () => {
                setSpacePanState(false);
                setCtrlPanState(false);
                endCanvasPan();
                endCanvasWidthResize();
            });

            // 4. Lắng nghe di chuột toàn trang cho Pan & Resize Handles
            window.addEventListener('pointermove', (e) => {
                handleCanvasWidthResize(e);
                handleCanvasPanMove(e);
            });

            window.addEventListener('pointerup', (e) => {
                endCanvasWidthResize();
                endCanvasPan();
            });

            window.addEventListener('pointercancel', (e) => {
                endCanvasWidthResize();
                endCanvasPan();
            });
        }

        // Khởi chạy khi DOM đã sẵn sàng
        if (document.readyState === 'loading') {
            document.addEventListener('DOMContentLoaded', initCanvasInteractions);
        } else {
            initCanvasInteractions();
        }

