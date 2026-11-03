/* ==============================================================
   APEX VISUAL WEB BUILDER - DEEP DOM LAYER TREE & STACKING CONTEXT
   Recursive AST Traversal, 7 Visual Categories, ABS/Z-Index Badge
   ============================================================== */

        // ============================================================
        // 3. NÂNG CẤP CHI TIẾT PHÂN HỆ LAYER (DOM TREE & STACKING CONTEXT)
        // ============================================================
        function getNodeCategory(node) {
            if (!node || !node.tag) return { category: 'container', icon: 'fa-solid fa-cube text-slate-400', typeLabel: 'Thẻ', preview: '', badgeColor: '' };
            const tag = node.tag.toLowerCase();

            // 1. Text
            if (['h1','h2','h3','h4','h5','h6','p','span','strong','em','b','i','small','label','blockquote'].includes(tag)) {
                const snippet = (node.textContent || '').substring(0, 24);
                return {
                    category: 'text',
                    icon: 'fa-solid fa-font text-cyan-400',
                    typeLabel: 'Text',
                    preview: snippet ? `"${snippet}..."` : '',
                    badgeColor: 'text-cyan-400 border-cyan-800/60 bg-cyan-950/50'
                };
            }

            // 2. Ảnh & Đồ họa
            if (['img','svg','picture','video','canvas','figure'].includes(tag)) {
                let preview = '';
                if (node.attrs['alt']) preview = node.attrs['alt'].value;
                else if (node.attrs['src']) preview = node.attrs['src'].value.split('/').pop();
                return {
                    category: 'image',
                    icon: 'fa-solid fa-image text-purple-400',
                    typeLabel: 'Ảnh',
                    preview: preview ? preview.substring(0, 20) : '',
                    badgeColor: 'text-purple-400 border-purple-800/60 bg-purple-950/50'
                };
            }

            // 3. Nút bấm & Liên kết
            if (['button','a'].includes(tag) || (node.attrs['role'] && node.attrs['role'].value === 'button')) {
                const text = node.textContent || (node.attrs['href'] ? node.attrs['href'].value : 'Nút');
                return {
                    category: 'button',
                    icon: 'fa-solid fa-arrow-pointer text-sky-400',
                    typeLabel: 'Nút/Link',
                    preview: text ? text.substring(0, 20) : '',
                    badgeColor: 'text-sky-400 border-sky-800/60 bg-sky-950/50'
                };
            }

            // 4. Form & Input
            if (['form','input','textarea','select','option'].includes(tag)) {
                const preview = node.attrs['type'] ? `type="${node.attrs['type'].value}"` : (node.attrs['name'] ? `name="${node.attrs['name'].value}"` : '');
                return {
                    category: 'form',
                    icon: 'fa-solid fa-keyboard text-emerald-400',
                    typeLabel: 'Form',
                    preview: preview,
                    badgeColor: 'text-emerald-400 border-emerald-800/60 bg-emerald-950/50'
                };
            }

            // 5. Danh sách & Bảng
            if (['ul','ol','li','table','thead','tbody','tr','td','th'].includes(tag)) {
                return {
                    category: 'list',
                    icon: 'fa-solid fa-list text-rose-400',
                    typeLabel: 'Danh sách',
                    preview: node.children && node.children.length ? `${node.children.length} mục` : '',
                    badgeColor: 'text-rose-400 border-rose-800/60 bg-rose-950/50'
                };
            }

            // 6. Embed & Khác
            if (['iframe','audio','embed','object'].includes(tag)) {
                return {
                    category: 'embed',
                    icon: 'fa-solid fa-cube text-slate-400',
                    typeLabel: 'Embed',
                    preview: node.attrs['src'] ? node.attrs['src'].value.split('/').pop() : '',
                    badgeColor: 'text-slate-400 border-slate-700/60 bg-slate-900/50'
                };
            }

            // 7. Khung bố cục (Layout Containers)
            let preview = '';
            if (node.attrs['id']) preview = '#' + node.attrs['id'].value;
            else if (node.attrs['class']) preview = '.' + node.attrs['class'].value.trim().split(/\s+/)[0];
            else if (node.children && node.children.length) preview = `${node.children.length} con`;

            return {
                category: 'container',
                icon: 'fa-solid fa-layer-group text-amber-400',
                typeLabel: 'Khung',
                preview: preview,
                badgeColor: 'text-amber-400 border-amber-800/60 bg-amber-950/50'
            };
        }

        function checkNodeHidden(node) {
            if (!node) return false;
            const sVal = node.attrs['style'] ? node.attrs['style'].value.toLowerCase() : '';
            const cVal = node.attrs['class'] ? node.attrs['class'].value : '';
            return sVal.includes('display: none') || sVal.includes('display:none') ||
                   sVal.includes('opacity: 0') || sVal.includes('opacity:0') ||
                   node.attrs['hidden'] !== undefined || /\bhidden\b/.test(cVal);
        }

        function checkNodeStacking(node) {
            if (!node) return [];
            const badges = [];
            const sVal = node.attrs['style'] ? node.attrs['style'].value.toLowerCase() : '';
            const cVal = node.attrs['class'] ? node.attrs['class'].value : '';

            if (sVal.includes('position: absolute') || /\babsolute\b/.test(cVal)) badges.push({ text: 'ABS', cls: 'bg-rose-950 text-rose-300 border-rose-800/60' });
            else if (sVal.includes('position: fixed') || /\bfixed\b/.test(cVal)) badges.push({ text: 'FIX', cls: 'bg-purple-950 text-purple-300 border-purple-800/60' });
            else if (sVal.includes('position: sticky') || /\bsticky\b/.test(cVal)) badges.push({ text: 'STK', cls: 'bg-sky-950 text-sky-300 border-sky-800/60' });

            const zMatch = sVal.match(/z-index:\s*(-?\d+)/) || cVal.match(/\bz-(\d+)\b/);
            if (zMatch) {
                badges.push({ text: `Z:${zMatch[1]}`, cls: 'bg-indigo-950 text-indigo-300 border-indigo-800/60' });
            }
            return badges;
        }

        function setLayerFilter(cat) {
            layerFilterCategory = cat;
            const filterBtns = ['all', 'text', 'image', 'button', 'form', 'container'];
            filterBtns.forEach(b => {
                const el = document.getElementById(`btnLayerFilter_${b}`);
                if (!el) return;
                if (b === cat) {
                    el.className = 'px-2 py-0.5 rounded-full bg-cyan-600 text-white font-bold transition flex items-center gap-1';
                } else {
                    el.className = 'px-2 py-0.5 rounded-full bg-slate-800 text-slate-400 hover:text-cyan-300 transition flex items-center gap-1';
                }
            });
            renderAstLayerTree();
        }

        function onLayerSearchInput() {
            const input = document.getElementById('layerSearchInput');
            layerFilterKeyword = input ? input.value.trim().toLowerCase() : '';
            const clearBtn = document.getElementById('btnClearLayerSearch');
            if (clearBtn) {
                if (layerFilterKeyword) clearBtn.classList.remove('hidden');
                else clearBtn.classList.add('hidden');
            }
            renderAstLayerTree();
        }

        function clearLayerSearch() {
            const input = document.getElementById('layerSearchInput');
            if (input) input.value = '';
            layerFilterKeyword = '';
            const clearBtn = document.getElementById('btnClearLayerSearch');
            if (clearBtn) clearBtn.classList.add('hidden');
            renderAstLayerTree();
        }

        function toggleAllLayers(expanded) {
            for (let id in astMapper.astMap) {
                layerExpandedState[id] = !!expanded;
            }
            renderAstLayerTree();
        }

        function toggleNodeExpand(nodeId, e) {
            if (e) { e.preventDefault(); e.stopPropagation(); }
            layerExpandedState[nodeId] = !isNodeExpanded(nodeId);
            renderAstLayerTree();
        }

        function isNodeExpanded(nodeId, depth = 0) {
            if (layerFilterKeyword) return true; // Bung hết khi search
            if (layerExpandedState[nodeId] !== undefined) return layerExpandedState[nodeId];
            return depth <= 3; // Mặc định mở 3 cấp đầu tiên
        }

        function doesNodeMatchFilter(node) {
            if (!node) return false;
            // 1. Lọc theo Danh mục
            if (layerFilterCategory !== 'all') {
                const info = getNodeCategory(node);
                if (info.category !== layerFilterCategory) {
                    let childMatches = false;
                    if (node.children) {
                        for (let cid of node.children) {
                            if (doesNodeMatchFilter(astMapper.astMap[cid])) {
                                childMatches = true;
                                break;
                            }
                        }
                    }
                    if (!childMatches) return false;
                }
            }

            // 2. Lọc theo Từ khóa Search
            if (layerFilterKeyword) {
                const tag = node.tag.toLowerCase();
                const idVal = node.attrs['id'] ? node.attrs['id'].value.toLowerCase() : '';
                const clsVal = node.attrs['class'] ? node.attrs['class'].value.toLowerCase() : '';
                const textVal = (node.textContent || '').toLowerCase();
                const matchSelf = tag.includes(layerFilterKeyword) ||
                                  idVal.includes(layerFilterKeyword) ||
                                  clsVal.includes(layerFilterKeyword) ||
                                  textVal.includes(layerFilterKeyword) ||
                                  node.id.toLowerCase().includes(layerFilterKeyword);
                if (matchSelf) return true;

                if (node.children) {
                    for (let cid of node.children) {
                        if (doesNodeMatchFilter(astMapper.astMap[cid])) return true;
                    }
                }
                return false;
            }

            return true;
        }

        // Vẽ Cây Phân Cấp DOM (Layer Tree Đa Tầng Chuyên Sâu)
        function renderAstLayerTree() {
            const container = document.getElementById('inspLayerTreeContainer');
            if (!container) return;

            let totalNodes = Object.keys(astMapper.astMap).length;
            let hiddenCount = 0;

            for (let id in astMapper.astMap) {
                if (checkNodeHidden(astMapper.astMap[id])) hiddenCount++;
            }

            const statsEl = document.getElementById('layerTreeStats');
            const hiddenStatsEl = document.getElementById('layerTreeHiddenStats');
            if (statsEl) statsEl.innerText = `${totalNodes} layers`;
            if (hiddenStatsEl) hiddenStatsEl.innerText = hiddenCount > 0 ? `${hiddenCount} ẩn` : '';

            function renderTreeNode(nodeId, depth = 0) {
                const node = astMapper.astMap[nodeId];
                if (!node) return '';

                if (!doesNodeMatchFilter(node)) return '';

                const isSelected = node.id === selectedAstNodeId;
                const isHidden = checkNodeHidden(node);
                const isLocked = studioLockedNodes.has(node.id);
                const info = getNodeCategory(node);
                const stackingBadges = checkNodeStacking(node);
                const hasChildren = node.children && node.children.length > 0;
                const expanded = isNodeExpanded(node.id, depth);

                const indentPx = depth * 14 + 4;

                let rowHtml = `
                    <div id="layer-row-${node.id}"
                         onclick="selectAstNode('${node.id}')"
                         oncontextmenu="event.preventDefault(); event.stopPropagation(); selectAstNode('${node.id}'); openContextMenu(event.clientX, event.clientY, '${node.id}');"
                         style="padding-left: ${indentPx}px;"
                         class="group py-1 px-1.5 rounded-lg cursor-pointer transition select-none flex items-center justify-between gap-1 text-[11px] ${isSelected ? 'bg-cyan-950/90 text-cyan-200 border border-cyan-700/80 shadow-sm' : 'hover:bg-slate-850/80 text-slate-300'} ${isHidden ? 'opacity-50' : ''}">

                        <!-- Cột Trái: Toggle Chevron, Icon Loại, Tên Thẻ, Preview, Badges -->
                        <div class="flex items-center gap-1.5 truncate flex-1 min-w-0">
                            <!-- Nút bung / thu gọn con -->
                            ${hasChildren ? `
                                <button type="button" onclick="toggleNodeExpand('${node.id}', event)" class="w-3.5 h-3.5 flex items-center justify-center text-slate-500 hover:text-white flex-shrink-0">
                                    <i class="fa-solid fa-chevron-right text-[8px] transition-transform duration-150 ${expanded ? 'rotate-90' : ''}"></i>
                                </button>
                            ` : '<span class="w-3.5 flex-shrink-0"></span>'}

                            <!-- Icon phân loại -->
                            <i class="${info.icon} text-[10px] flex-shrink-0"></i>

                            <!-- Tên thẻ Tag Name -->
                            <span class="font-bold text-[11px] ${isSelected ? 'text-white' : 'text-slate-200'} ${isHidden ? 'line-through' : ''}">${node.tag}</span>

                            <!-- Preview trích đoạn text hoặc class/id -->
                            ${info.preview ? `<span class="text-[10px] text-slate-400 truncate flex-shrink min-w-0">${escapeHTML(info.preview)}</span>` : ''}

                            <!-- Badges Stacking Context: ABS, FIX, STK, Z-Index -->
                            ${stackingBadges.map(b => `<span class="text-[8px] font-mono px-1 py-0.2 rounded border font-bold ${b.cls} flex-shrink-0">${b.text}</span>`).join('')}
                        </div>

                        <!-- Cột Phải: Phím tắt hành động nhanh (Lock, Ẩn/Hiện) -->
                        <div class="flex items-center gap-1 flex-shrink-0 opacity-80 group-hover:opacity-100">
                            <!-- Nút Khóa / Mở Khóa -->
                            <button type="button" onclick="event.stopPropagation(); toggleCurrentNodeLock('${node.id}');"
                                    class="w-5 h-5 rounded flex items-center justify-center transition ${isLocked ? 'text-yellow-400 bg-yellow-950/60 border border-yellow-800/60' : 'text-slate-500 hover:text-slate-200 hover:bg-slate-800'}"
                                    title="${isLocked ? 'Đang khóa - Bấm để mở' : 'Bấm để khóa layer này'}">
                                <i class="fa-solid ${isLocked ? 'fa-lock' : 'fa-lock-open'} text-[9px]"></i>
                            </button>

                            <!-- Nút Ẩn / Hiện -->
                            <button type="button" onclick="event.stopPropagation(); toggleCurrentNodeVisibility('${node.id}');"
                                    class="w-5 h-5 rounded flex items-center justify-center transition ${isHidden ? 'text-amber-400 bg-amber-950/60 border border-amber-800/60' : 'text-slate-500 hover:text-slate-200 hover:bg-slate-800'}"
                                    title="${isHidden ? 'Đang ẩn - Bấm để hiện' : 'Bấm để ẩn layer này'}">
                                <i class="fa-solid ${isHidden ? 'fa-eye-slash' : 'fa-eye'} text-[9px]"></i>
                            </button>
                        </div>
                    </div>
                `;

                if (hasChildren && expanded) {
                    node.children.forEach(cid => {
                        rowHtml += renderTreeNode(cid, depth + 1);
                    });
                }
                return rowHtml;
            }

            let fullTreeHtml = '';
            astMapper.tree.forEach(rootId => {
                fullTreeHtml += renderTreeNode(rootId, 0);
            });
            container.innerHTML = fullTreeHtml || '<div class="text-slate-500 text-xs italic p-3 text-center">Không tìm thấy layer phù hợp</div>';
        }

