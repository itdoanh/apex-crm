/* ==============================================================
   APEX VISUAL WEB BUILDER - AST SOURCE MAPPER & 2-WAY SYNC
   Non-Destructive Tokenizer, DOM Tree Mutator & String Splicing
   ============================================================== */

        // ============================================================
        class ApexASTSourceMapper {
            constructor() {
                this.rawHTML = '';
                this.astMap = {};
                this.tree = [];
                this.nodeCounter = 0;
                this.mobileStylesMap = {};
                this.voidElements = new Set([
                    'area', 'base', 'br', 'col', 'embed', 'hr', 'img', 'input',
                    'link', 'meta', 'param', 'source', 'track', 'wbr'
                ]);
            }

            parse(htmlString) {
                this.rawHTML = htmlString || '';
                this.astMap = {};
                this.tree = [];
                this.nodeCounter = 0;

                const stack = [];
                let i = 0;
                const len = this.rawHTML.length;

                while (i < len) {
                    const nextLt = this.rawHTML.indexOf('<', i);
                    if (nextLt === -1) break;

                    // 1. Comment <!-- ... -->
                    if (this.rawHTML.startsWith('<!--', nextLt)) {
                        const commentEnd = this.rawHTML.indexOf('-->', nextLt + 4);
                        i = commentEnd === -1 ? len : commentEnd + 3;
                        continue;
                    }

                    // 2. DOCTYPE <!DOCTYPE ...>
                    if (this.rawHTML.substring(nextLt, nextLt + 9).toUpperCase() === '<!DOCTYPE') {
                        const docEnd = this.rawHTML.indexOf('>', nextLt + 9);
                        i = docEnd === -1 ? len : docEnd + 1;
                        continue;
                    }

                    // 3. Closing tag </tag>
                    if (this.rawHTML.startsWith('</', nextLt)) {
                        const closeTagEnd = this.rawHTML.indexOf('>', nextLt + 2);
                        if (closeTagEnd === -1) {
                            i = len;
                            break;
                        }
                        const closeTagName = this.rawHTML.substring(nextLt + 2, closeTagEnd).trim().split(/\s+/)[0].toLowerCase();

                        let matchIdx = -1;
                        for (let s = stack.length - 1; s >= 0; s--) {
                            if (stack[s].tag === closeTagName) {
                                matchIdx = s;
                                break;
                            }
                        }
                        if (matchIdx !== -1) {
                            const node = stack[matchIdx];
                            node.closeTagStart = nextLt;
                            node.closeTagEnd = closeTagEnd + 1;
                            node.endOffset = closeTagEnd + 1;
                            node.innerStart = node.openTagEnd;
                            node.innerEnd = node.closeTagStart;
                            node.innerHTML = this.rawHTML.substring(node.innerStart, node.innerEnd);
                            node.textContent = node.innerHTML.replace(/<[^>]*>/g, '').trim();
                            stack.splice(matchIdx);
                        }
                        i = closeTagEnd + 1;
                        continue;
                    }

                    // 4. Opening tag <tag ...>
                    const tagChar = this.rawHTML[nextLt + 1];
                    if (!tagChar || !/[a-zA-Z]/.test(tagChar)) {
                        i = nextLt + 1;
                        continue;
                    }

                    let inSingleQuote = false;
                    let inDoubleQuote = false;
                    let openTagEnd = -1;
                    for (let j = nextLt + 1; j < len; j++) {
                        const ch = this.rawHTML[j];
                        if (ch === '"' && !inSingleQuote) inDoubleQuote = !inDoubleQuote;
                        else if (ch === "'" && !inDoubleQuote) inSingleQuote = !inSingleQuote;
                        else if (ch === '>' && !inSingleQuote && !inDoubleQuote) {
                            openTagEnd = j + 1;
                            break;
                        }
                    }

                    if (openTagEnd === -1) {
                        i = len;
                        break;
                    }

                    const openTagRaw = this.rawHTML.substring(nextLt, openTagEnd);
                    const isSelfClosing = openTagRaw.endsWith('/>');

                    const tagMatch = openTagRaw.match(/^<([a-zA-Z0-9\-]+)/);
                    if (!tagMatch) {
                        i = openTagEnd;
                        continue;
                    }
                    const tagName = tagMatch[1].toLowerCase();

                    // Bỏ qua thẻ style responsive đặc biệt của Studio để không làm lệch Node ID của các phần tử DOM
                    if (tagName === 'style' && openTagRaw.includes('__apex_responsive_styles__')) {
                        const closeStyleIdx = this.rawHTML.toLowerCase().indexOf('</style>', openTagEnd);
                        i = (closeStyleIdx !== -1) ? closeStyleIdx + 8 : openTagEnd;
                        continue;
                    }

                    const attrs = {};
                    const attrList = [];
                    const attrRegex = /([a-zA-Z0-9_:\-]+)(?:\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+)))?/g;
                    attrRegex.lastIndex = tagMatch[0].length;
                    let attrM;
                    while ((attrM = attrRegex.exec(openTagRaw)) !== null) {
                        const attrName = attrM[1].toLowerCase();
                        if (attrName === '/' || attrName === '') continue;
                        const attrVal = attrM[2] !== undefined ? attrM[2] : (attrM[3] !== undefined ? attrM[3] : (attrM[4] !== undefined ? attrM[4] : ''));
                        const rawAttrStr = attrM[0];
                        const attrStartOffset = nextLt + attrM.index;
                        const attrEndOffset = attrStartOffset + rawAttrStr.length;

                        const attrObj = {
                            name: attrName,
                            value: attrVal,
                            raw: rawAttrStr,
                            startOffset: attrStartOffset,
                            endOffset: attrEndOffset
                        };
                        attrs[attrName] = attrObj;
                        attrList.push(attrObj);
                    }

                    // Tái sử dụng data-ve-id sẵn có trong mã HTML để duy trì đồng bộ 100% giữa Code, Preview và Deploy
                    let nodeId = (attrs['data-ve-id'] && attrs['data-ve-id'].value) ? attrs['data-ve-id'].value : null;
                    if (!nodeId) {
                        this.nodeCounter++;
                        nodeId = 'node-' + this.nodeCounter;
                    } else {
                        const m = nodeId.match(/^node-(\d+)$/);
                        if (m) {
                            const num = parseInt(m[1], 10);
                            if (num > this.nodeCounter) this.nodeCounter = num;
                        }
                    }

                    const node = {
                        id: nodeId,
                        tag: tagName,
                        startOffset: nextLt,
                        endOffset: openTagEnd,
                        openTagStart: nextLt,
                        openTagEnd: openTagEnd,
                        openTagRaw: openTagRaw,
                        closeTagStart: -1,
                        closeTagEnd: -1,
                        innerStart: openTagEnd,
                        innerEnd: openTagEnd,
                        innerHTML: '',
                        textContent: '',
                        attrs: attrs,
                        attrList: attrList,
                        classes: attrs['class'] ? attrs['class'].value.split(/\s+/).filter(Boolean) : [],
                        parentId: stack.length > 0 ? stack[stack.length - 1].id : null,
                        children: []
                    };

                    this.astMap[nodeId] = node;

                    if (node.parentId && this.astMap[node.parentId]) {
                        this.astMap[node.parentId].children.push(nodeId);
                    } else {
                        this.tree.push(nodeId);
                    }

                    // Special tags: <script> and <style>
                    if (tagName === 'script' || tagName === 'style') {
                        const endTagStr = '</' + tagName + '>';
                        const endTagIdx = this.rawHTML.toLowerCase().indexOf(endTagStr, openTagEnd);
                        if (endTagIdx !== -1) {
                            node.closeTagStart = endTagIdx;
                            node.closeTagEnd = endTagIdx + endTagStr.length;
                            node.endOffset = node.closeTagEnd;
                            node.innerStart = openTagEnd;
                            node.innerEnd = endTagIdx;
                            node.innerHTML = this.rawHTML.substring(node.innerStart, node.innerEnd);
                            node.textContent = node.innerHTML;
                            i = node.closeTagEnd;
                        } else {
                            i = openTagEnd;
                        }
                        continue;
                    }

                    if (this.voidElements.has(tagName) || isSelfClosing) {
                        node.endOffset = openTagEnd;
                        i = openTagEnd;
                        continue;
                    }

                    stack.push(node);
                    i = openTagEnd;
                }

                this.parseResponsiveStyles();
                return this;
            }

            // Tiêm data-ve-id vào bản Preview Iframe (Không ghi vào file đĩa hay CSDL!)
            generateInstrumentedHTML() {
                if (!this.rawHTML) return '';
                let result = '';
                let lastPos = 0;

                const sortedNodes = Object.values(this.astMap).sort((a, b) => a.openTagStart - b.openTagStart);

                for (const node of sortedNodes) {
                    result += this.rawHTML.substring(lastPos, node.openTagStart);
                    let openTag = node.openTagRaw;
                    if (!node.attrs['data-ve-id']) {
                        const insertIdStr = ` data-ve-id="${node.id}"`;
                        if (openTag.endsWith('/>')) {
                            openTag = openTag.substring(0, openTag.length - 2) + insertIdStr + '/>';
                        } else if (openTag.endsWith('>')) {
                            openTag = openTag.substring(0, openTag.length - 1) + insertIdStr + '>';
                        }
                    }
                    result += openTag;
                    lastPos = node.openTagEnd;
                }

                result += this.rawHTML.substring(lastPos);

                // Triệt để vô hiệu hóa mọi lệnh chuyển hướng sang chucmung.html trong Code Studio Live Preview (không để sót ngoặc thừa)
                result = result.replace(/(?:window\.|origLocation\.)?location\.replace\s*\(\s*['"]chucmung\.html[^;\r\n]*;?/gi, "/* [STUDIO_NO_REDIRECT] */ console.log('[Studio] Blocked location.replace to chucmung.html');");
                result = result.replace(/(?:window\.|origLocation\.)?location\.href\s*=\s*['"]chucmung\.html[^;\r\n]*;?/gi, "/* [STUDIO_NO_REDIRECT] */ console.log('[Studio] Blocked location.href to chucmung.html');");
                result = result.replace(/origReplace\s*\(\s*['"]chucmung\.html[^;\r\n]*;?/gi, "/* [STUDIO_NO_REDIRECT] */ console.log('[Studio] Blocked origReplace to chucmung.html');");
                result = result.replace(/window\.redirectToChucMung\s*\([^;\r\n]*;?/gi, "/* [STUDIO_NO_REDIRECT] */ console.log('[Studio] Blocked redirectToChucMung call');");

                const helperScript = this.getHelperScript();
                const headLower = result.toLowerCase().indexOf('<head>');
                if (headLower !== -1) {
                    result = result.substring(0, headLower + 6) + helperScript + result.substring(headLower + 6);
                } else {
                    result = helperScript + result;
                }

                return result;
            }

            getHelperScript() {
                return `\n<base href="/">\n<script id="__APEX_BUILDER_HELPER_SCRIPT__">\n` +
                    `(function() {\n` +
                    `    window.__APEX_BUILDER_ACTIVE__ = true;\n` +
                    `    window.__APEX_STUDIO__ = true;\n` +
                    `    window.__APEX_PREVIEW__ = true;\n` +
                    `    window.IS_PREVIEW = true;\n` +
                    `    window.__APEX_FREEZE_MODE__ = true;\n` +
                    `    window.redirectToChucMung = function() { console.log('[Studio] Blocked redirectToChucMung call'); };\n` +
                    `    document.addEventListener('DOMContentLoaded', function() {\n` +
                    `        var observer = new MutationObserver(function(mutations) {\n` +
                    `            mutations.forEach(function(m) {\n` +
                    `                m.addedNodes.forEach(function(n) {\n` +
                    `                    if (n.nodeType === 1 && !n.getAttribute('data-ve-id')) {\n` +
                    `                        n.setAttribute('data-ve-ephemeral', 'true');\n` +
                    `                        var desc = n.querySelectorAll('*');\n` +
                    `                        for (var i = 0; i < desc.length; i++) {\n` +
                    `                            if (!desc[i].getAttribute('data-ve-id')) desc[i].setAttribute('data-ve-ephemeral', 'true');\n` +
                    `                        }\n` +
                    `                    }\n` +
                    `                });\n` +
                    `            });\n` +
                    `        });\n` +
                    `        if (document.body) observer.observe(document.body, { childList: true, subtree: true });\n` +
                    `    });\n` +
                    `    function getValidTarget(target) {\n` +
                    `        var curr = target;\n` +
                    `        while (curr && curr !== document.body && curr !== document.documentElement) {\n` +
                    `            if (curr.hasAttribute('data-ve-id') && !curr.hasAttribute('data-ve-ephemeral')) return curr;\n` +
                    `            curr = curr.parentElement;\n` +
                    `        }\n` +
                    `        return (curr && curr.hasAttribute('data-ve-id')) ? curr : null;\n` +
                    `    }\n` +
                    `    function getNodeData(el) {\n` +
                    `        if (!el) return null;\n` +
                    `        var rect = el.getBoundingClientRect();\n` +
                    `        var style = window.getComputedStyle(el);\n` +
                    `        var attrs = {};\n` +
                    `        for (var i = 0; i < el.attributes.length; i++) {\n` +
                    `            attrs[el.attributes[i].name] = el.attributes[i].value;\n` +
                    `        }\n` +
                    `        return {\n` +
                    `            nodeId: el.getAttribute('data-ve-id'),\n` +
                    `            tagName: el.tagName.toLowerCase(),\n` +
                    `            className: el.className || '',\n` +
                    `            text: el.innerText || el.textContent || '',\n` +
                    `            rect: { top: rect.top, left: rect.left, width: rect.width, height: rect.height },\n` +
                    `            style: {\n` +
                    `                display: style.display,\n` +
                    `                flexDirection: style.flexDirection,\n` +
                    `                justifyContent: style.justifyContent,\n` +
                    `                alignItems: style.alignItems,\n` +
                    `                gap: style.gap,\n` +
                    `                position: style.position,\n` +
                    `                top: style.top,\n` +
                    `                right: style.right,\n` +
                    `                bottom: style.bottom,\n` +
                    `                left: style.left,\n` +
                    `                zIndex: style.zIndex,\n` +
                    `                fontSize: style.fontSize,\n` +
                    `                fontWeight: style.fontWeight,\n` +
                    `                color: style.color,\n` +
                    `                textAlign: style.textAlign,\n` +
                    `                textTransform: style.textTransform,\n` +
                    `                lineHeight: style.lineHeight,\n` +
                    `                backgroundColor: style.backgroundColor,\n` +
                    `                backgroundImage: style.backgroundImage,\n` +
                    `                opacity: style.opacity,\n` +
                    `                boxShadow: style.boxShadow,\n` +
                    `                width: style.width,\n` +
                    `                height: style.height,\n` +
                    `                maxWidth: style.maxWidth,\n` +
                    `                minWidth: style.minWidth,\n` +
                    `                marginTop: style.marginTop,\n` +
                    `                marginRight: style.marginRight,\n` +
                    `                marginBottom: style.marginBottom,\n` +
                    `                marginLeft: style.marginLeft,\n` +
                    `                paddingTop: style.paddingTop,\n` +
                    `                paddingRight: style.paddingRight,\n` +
                    `                paddingBottom: style.paddingBottom,\n` +
                    `                paddingLeft: style.paddingLeft,\n` +
                    `                padding: style.padding,\n` +
                    `                margin: style.margin,\n` +
                    `                borderRadius: style.borderRadius,\n` +
                    `                borderWidth: style.borderWidth,\n` +
                    `                borderStyle: style.borderStyle,\n` +
                    `                borderColor: style.borderColor\n` +
                    `            },\n` +
                    `            attributes: attrs\n` +
                    `        };\n` +
                    `    }\n` +
                    `    function setIframeSpaceCursor(state) {\n` +
                    `        var st = document.getElementById('__apex_pan_cursor_style__');\n` +
                    `        if (state === 'grab') {\n` +
                    `            if (!st) {\n` +
                    `                st = document.createElement('style');\n` +
                    `                st.id = '__apex_pan_cursor_style__';\n` +
                    `                document.head.appendChild(st);\n` +
                    `            }\n` +
                    `            st.textContent = 'html, body, *, a, button, input { cursor: grab !important; user-select: none !important; -webkit-user-select: none !important; }';\n` +
                    `        } else if (state === 'grabbing') {\n` +
                    `            if (!st) {\n` +
                    `                st = document.createElement('style');\n` +
                    `                st.id = '__apex_pan_cursor_style__';\n` +
                    `                document.head.appendChild(st);\n` +
                    `            }\n` +
                    `            st.textContent = 'html, body, *, a, button, input { cursor: grabbing !important; user-select: none !important; -webkit-user-select: none !important; }';\n` +
                    `        } else {\n` +
                    `            if (st) st.remove();\n` +
                    `        }\n` +
                    `    }\n` +
                    `    document.addEventListener('click', function(e) {\n` +
                    `        if (window.__APEX_FREEZE_MODE__) { e.preventDefault(); e.stopPropagation(); }\n` +
                    `        if (__apex_space_pressed || __apex_is_panning) { e.preventDefault(); e.stopPropagation(); return; }\n` +
                    `        var t = getValidTarget(e.target);\n` +
                    `        if (t) window.parent.postMessage({ type: 'APEX_BUILDER_NODE_CLICKED', data: getNodeData(t) }, '*');\n` +
                    `    }, true);\n` +
                    `    document.addEventListener('dblclick', function(e) {\n` +
                    `        if (window.__APEX_FREEZE_MODE__) { e.preventDefault(); e.stopPropagation(); }\n` +
                    `        if (__apex_space_pressed || __apex_is_panning) { e.preventDefault(); e.stopPropagation(); return; }\n` +
                    `        var t = getValidTarget(e.target);\n` +
                    `        if (t) window.parent.postMessage({ type: 'APEX_BUILDER_NODE_DBLCLICKED', data: getNodeData(t) }, '*');\n` +
                    `    }, true);\n` +
                    `    document.addEventListener('mouseover', function(e) {\n` +
                    `        if (!window.__APEX_BUILDER_ACTIVE__ || !window.__APEX_FREEZE_MODE__) return;\n` +
                    `        if (__apex_space_pressed || __apex_is_panning) return;\n` +
                    `        var t = getValidTarget(e.target);\n` +
                    `        if (t) {\n` +
                    `            var r = t.getBoundingClientRect();\n` +
                    `            window.parent.postMessage({ type: 'APEX_BUILDER_NODE_HOVERED', nodeId: t.getAttribute('data-ve-id'), tagName: t.tagName.toLowerCase(), className: t.className || '', rect: { top: r.top, left: r.left, width: r.width, height: r.height } }, '*');\n` +
                    `        }\n` +
                    `    }, true);\n` +
                    `    document.addEventListener('mouseleave', function() {\n` +
                    `        window.parent.postMessage({ type: 'APEX_BUILDER_HOVER_LEAVE' }, '*');\n` +
                    `    });\n` +
                    `    function notifyVp() {\n` +
                    `        window.parent.postMessage({ type: 'APEX_BUILDER_VIEWPORT_CHANGED' }, '*');\n` +
                    `    }\n` +
                    `    window.addEventListener('scroll', notifyVp, { passive: true });\n` +
                    `    window.addEventListener('resize', notifyVp, { passive: true });\n` +
                    `    var __apex_space_pressed = false;\n` +
                    `    var __apex_is_panning = false;\n` +
                    `    var __apex_pan_last_x = 0;\n` +
                    `    var __apex_pan_last_y = 0;\n` +
                    `    var __apex_pan_raf = null;\n` +
                    `    var __apex_pan_accum_dx = 0;\n` +
                    `    var __apex_pan_accum_dy = 0;\n` +
                    `    function applyIframePan() {\n` +
                    `        __apex_pan_raf = null;\n` +
                    `        if (__apex_pan_accum_dx === 0 && __apex_pan_accum_dy === 0) return;\n` +
                    `        var dx = __apex_pan_accum_dx;\n` +
                    `        var dy = __apex_pan_accum_dy;\n` +
                    `        __apex_pan_accum_dx = 0;\n` +
                    `        __apex_pan_accum_dy = 0;\n` +
                    `        window.scrollBy({ left: -dx, top: -dy, behavior: 'instant' });\n` +
                    `        window.parent.postMessage({ type: 'APEX_BUILDER_PAN_MOVE', dx: dx, dy: dy }, '*');\n` +
                    `    }\n` +
                    `    window.addEventListener('mouseup', function() {\n` +
                    `        if (__apex_is_panning) {\n` +
                    `            __apex_is_panning = false;\n` +
                    `            if (__apex_pan_raf) { cancelAnimationFrame(__apex_pan_raf); applyIframePan(); }\n` +
                    `            setIframeSpaceCursor(__apex_space_pressed ? 'grab' : null);\n` +
                    `            window.parent.postMessage({ type: 'APEX_BUILDER_PAN_END' }, '*');\n` +
                    `        }\n` +
                    `        window.parent.postMessage({ type: 'APEX_BUILDER_IFRAME_MOUSEUP' }, '*');\n` +
                    `    });\n` +
                    `    window.addEventListener('pointerup', function() {\n` +
                    `        if (__apex_is_panning) {\n` +
                    `            __apex_is_panning = false;\n` +
                    `            if (__apex_pan_raf) { cancelAnimationFrame(__apex_pan_raf); applyIframePan(); }\n` +
                    `            setIframeSpaceCursor(__apex_space_pressed ? 'grab' : null);\n` +
                    `            window.parent.postMessage({ type: 'APEX_BUILDER_PAN_END' }, '*');\n` +
                    `        }\n` +
                    `        window.parent.postMessage({ type: 'APEX_BUILDER_IFRAME_MOUSEUP' }, '*');\n` +
                    `    });\n` +
                    `    document.addEventListener('pointerdown', function(e) {\n` +
                    `        if (__apex_space_pressed || e.button === 1) {\n` +
                    `            e.preventDefault();\n` +
                    `            e.stopPropagation();\n` +
                    `            __apex_is_panning = true;\n` +
                    `            __apex_pan_last_x = e.clientX;\n` +
                    `            __apex_pan_last_y = e.clientY;\n` +
                    `            setIframeSpaceCursor('grabbing');\n` +
                    `            window.parent.postMessage({\n` +
                    `                type: 'APEX_BUILDER_PAN_START',\n` +
                    `                clientX: e.clientX,\n` +
                    `                clientY: e.clientY\n` +
                    `            }, '*');\n` +
                    `        }\n` +
                    `    }, true);\n` +
                    `    document.addEventListener('pointermove', function(e) {\n` +
                    `        if (__apex_is_panning) {\n` +
                    `            e.preventDefault();\n` +
                    `            e.stopPropagation();\n` +
                    `            var dx = e.clientX - __apex_pan_last_x;\n` +
                    `            var dy = e.clientY - __apex_pan_last_y;\n` +
                    `            __apex_pan_last_x = e.clientX;\n` +
                    `            __apex_pan_last_y = e.clientY;\n` +
                    `            __apex_pan_accum_dx += dx;\n` +
                    `            __apex_pan_accum_dy += dy;\n` +
                    `            if (!__apex_pan_raf) {\n` +
                    `                __apex_pan_raf = requestAnimationFrame(applyIframePan);\n` +
                    `            }\n` +
                    `        }\n` +
                    `    }, true);\n` +
                    `    window.addEventListener('keydown', function(e) {\n` +
                    `        if (e.key === 'Control' || e.key === 'Meta') {\n` +
                    `            window.parent.postMessage({ type: 'APEX_BUILDER_KEY_CTRL', pressed: true }, '*');\n` +
                    `        }\n` +
                    `        if (e.code === 'Space' || e.key === ' ') {\n` +
                    `            var t = (document.activeElement ? document.activeElement.tagName : '').toLowerCase();\n` +
                    `            var isEd = document.activeElement && document.activeElement.isContentEditable;\n` +
                    `            if (t !== 'input' && t !== 'textarea' && !isEd) {\n` +
                    `                e.preventDefault();\n` +
                    `                __apex_space_pressed = true;\n` +
                    `                setIframeSpaceCursor('grab');\n` +
                    `                window.parent.postMessage({ type: 'APEX_BUILDER_KEY_SPACE', pressed: true }, '*');\n` +
                    `            }\n` +
                    `        }\n` +
                    `    }, true);\n` +
                    `    window.addEventListener('keyup', function(e) {\n` +
                    `        if (e.key === 'Control' || e.key === 'Meta') {\n` +
                    `            window.parent.postMessage({ type: 'APEX_BUILDER_KEY_CTRL', pressed: false }, '*');\n` +
                    `        }\n` +
                    `        if (e.code === 'Space' || e.key === ' ') {\n` +
                    `            __apex_space_pressed = false;\n` +
                    `            __apex_is_panning = false;\n` +
                    `            setIframeSpaceCursor(null);\n` +
                    `            window.parent.postMessage({ type: 'APEX_BUILDER_KEY_SPACE', pressed: false }, '*');\n` +
                    `        }\n` +
                    `    }, true);\n` +
                    `    window.addEventListener('blur', function() {\n` +
                    `        if (__apex_space_pressed) {\n` +
                    `            __apex_space_pressed = false;\n` +
                    `            __apex_is_panning = false;\n` +
                    `            setIframeSpaceCursor(null);\n` +
                    `            window.parent.postMessage({ type: 'APEX_BUILDER_KEY_SPACE', pressed: false }, '*');\n` +
                    `        }\n` +
                    `    });\n` +
                    `    window.addEventListener('wheel', function(e) {\n` +
                    `        if (e.ctrlKey || e.metaKey) {\n` +
                    `            e.preventDefault();\n` +
                    `            window.parent.postMessage({ type: 'APEX_BUILDER_WHEEL_ZOOM', deltaY: e.deltaY }, '*');\n` +
                    `        }\n` +
                    `    }, { passive: false });\n` +
                    `    document.addEventListener('contextmenu', function(e) {\n` +
                    `        if (window.__APEX_FREEZE_MODE__ || window.__APEX_BUILDER_ACTIVE__) {\n` +
                    `            e.preventDefault();\n` +
                    `            e.stopPropagation();\n` +
                    `            var t = getValidTarget(e.target);\n` +
                    `            window.parent.postMessage({\n` +
                    `                type: 'APEX_BUILDER_CONTEXT_MENU',\n` +
                    `                nodeId: t ? t.getAttribute('data-ve-id') : null,\n` +
                    `                clientX: e.clientX,\n` +
                    `                clientY: e.clientY,\n` +
                    `                nodeData: t ? getNodeData(t) : null\n` +
                    `            }, '*');\n` +
                    `        }\n` +
                    `    }, true);\n` +
                    `    window.addEventListener('message', function(ev) {\n` +
                    `        if (!ev.data || typeof ev.data !== 'object') return;\n` +
                    `        var m = ev.data;\n` +
                    `        if (m.type === 'APEX_CMD_SET_FREEZE_MODE') {\n` +
                    `            window.__APEX_FREEZE_MODE__ = !!m.enabled;\n` +
                    `            var st = document.getElementById('__apex_freeze_css__');\n` +
                    `            if (m.enabled) {\n` +
                    `                if (!st) {\n` +
                    `                    st = document.createElement('style');\n` +
                    `                    st.id = '__apex_freeze_css__';\n` +
                    `                    st.innerHTML = '[data-aos] { opacity: 1 !important; transform: none !important; transition: none !important; animation: none !important; } a { cursor: pointer; }';\n` +
                    `                    document.head.appendChild(st);\n` +
                    `                }\n` +
                    `            } else if (st) st.remove();\n` +
                    `        }\n` +
                    `        if (m.type === 'APEX_CMD_GET_NODE_RECT') {\n` +
                    `            var el = document.querySelector('[data-ve-id=\"' + m.nodeId + '\"]');\n` +
                    `            if (el) {\n` +
                    `                if (m.scrollIntoView) {\n` +
                    `                    try { el.scrollIntoView({ behavior: 'smooth', block: 'center', inline: 'nearest' }); } catch(e) {}\n` +
                    `                }\n` +
                    `                window.parent.postMessage({ type: 'APEX_BUILDER_RECT_RESPONSE', data: getNodeData(el) }, '*');\n` +
                    `            }\n` +
                    `        }\n` +
                    `        if (m.type === 'APEX_CMD_SPACE_STATE') {\n` +
                    `            __apex_space_pressed = !!m.pressed;\n` +
                    `            setIframeSpaceCursor(__apex_space_pressed ? 'grab' : null);\n` +
                    `        }\n` +
                    `        if (m.type === 'APEX_CMD_PAN_ACTIVE') {\n` +
                    `            __apex_is_panning = !!m.active;\n` +
                    `            setIframeSpaceCursor(__apex_is_panning ? 'grabbing' : (__apex_space_pressed ? 'grab' : null));\n` +
                    `        }\n` +
                    `        if (m.type === 'APEX_CMD_SET_CONTENT_ZOOM') {\n` +
                    `            var z = parseFloat(m.zoom) || 1.0;\n` +
                    `            document.body.style.zoom = z;\n` +
                    `            window.__APEX_CONTENT_ZOOM__ = z;\n` +
                    `        }\n` +
                    `        if (m.type === 'APEX_CMD_SCROLL_PAN') {\n` +
                    `            try { window.scrollBy({ left: m.dx, top: m.dy, behavior: 'instant' }); } catch(e) {}\n` +
                    `        }\n` +
                    `        if (m.type === 'APEX_CMD_CLEAR_TEMP_STYLES') {\n` +
                    `            var el = document.querySelector('[data-ve-id=\"' + m.nodeId + '\"]');\n` +
                    `            if (el) {\n` +
                    `                var props = m.props || ['left', 'top', 'position', 'width', 'height', 'max-width', 'min-width'];\n` +
                    `                for (var p = 0; p < props.length; p++) {\n` +
                    `                    var pr = props[p].indexOf('-') !== -1 ? props[p] : props[p].replace(/[A-Z]/g, function(l) { return '-' + l.toLowerCase(); });\n` +
                    `                    el.style.removeProperty(pr);\n` +
                    `                }\n` +
                    `            }\n` +
                    `        }\n` +
                    `        if (m.type === 'APEX_CMD_UPDATE_RESPONSIVE_CSS') {\n` +
                    `            var rst = document.getElementById('__apex_responsive_styles__');\n` +
                    `            if (!rst) {\n` +
                    `                rst = document.createElement('style');\n` +
                    `                rst.id = '__apex_responsive_styles__';\n` +
                    `                document.head.appendChild(rst);\n` +
                    `            }\n` +
                    `            rst.textContent = m.cssText || '';\n` +
                    `        }\n` +
                    `        if (m.type === 'APEX_CMD_UPDATE_STYLE_TEMP') {\n` +
                    `            var el = document.querySelector('[data-ve-id=\"' + m.nodeId + '\"]');\n` +
                    `            if (el && m.styles) {\n` +
                    `                for (var k in m.styles) {\n` +
                    `                    var pr = k.indexOf('-') !== -1 ? k : k.replace(/[A-Z]/g, function(l) { return '-' + l.toLowerCase(); });\n` +
                    `                    var val = m.styles[k];\n` +
                    `                    if (val === null || val === '') {\n` +
                    `                        el.style.removeProperty(pr);\n` +
                    `                    } else {\n` +
                    `                        el.style.setProperty(pr, String(val), 'important');\n` +
                    `                    }\n` +
                    `                }\n` +
                    `            }\n` +
                    `        }\n` +
                    `        if (m.type === 'APEX_CMD_UPDATE_CLASS_TEMP') {\n` +
                    `            var el = document.querySelector('[data-ve-id=\"' + m.nodeId + '\"]');\n` +
                    `            if (el) el.className = m.className;\n` +
                    `        }\n` +
                    `        if (m.type === 'APEX_CMD_UPDATE_ATTR_TEMP') {\n` +
                    `            var el = document.querySelector('[data-ve-id=\"' + m.nodeId + '\"]');\n` +
                    `            if (el) {\n` +
                    `                if (m.val === null || m.val === undefined || m.val === '') {\n` +
                    `                    el.removeAttribute(m.attr);\n` +
                    `                } else {\n` +
                    `                    el.setAttribute(m.attr, m.val);\n` +
                    `                    if (m.attr === 'src' && el.tagName === 'IMG') el.src = m.val;\n` +
                    `                    if (m.attr === 'href' && el.tagName === 'A') el.href = m.val;\n` +
                    `                }\n` +
                    `            }\n` +
                    `        }\n` +
                    `        if (m.type === 'APEX_CMD_UPDATE_TEXT_TEMP') {\n` +
                    `            var el = document.querySelector('[data-ve-id=\"' + m.nodeId + '\"]');\n` +
                    `            if (el) el.innerText = m.text;\n` +
                    `        }\n` +
                    `        if (m.type === 'APEX_CMD_START_INLINE_EDIT') {\n` +
                    `            var el = document.querySelector('[data-ve-id=\"' + m.nodeId + '\"]');\n` +
                    `            if (el) {\n` +
                    `                el.setAttribute('contenteditable', 'true');\n` +
                    `                el.style.outline = '2px dashed #06b6d4';\n` +
                    `                el.style.outlineOffset = '2px';\n` +
                    `                el.focus();\n` +
                    `                function onBlur() {\n` +
                    `                    el.removeAttribute('contenteditable');\n` +
                    `                    el.style.outline = '';\n` +
                    `                    el.style.outlineOffset = '';\n` +
                    `                    el.removeEventListener('blur', onBlur);\n` +
                    `                    window.parent.postMessage({ type: 'APEX_BUILDER_INLINE_EDIT_FINISHED', nodeId: m.nodeId, newText: el.innerText }, '*');\n` +
                    `                }\n` +
                    `                el.addEventListener('blur', onBlur);\n` +
                    `            }\n` +
                    `        }\n` +
                    `    });\n` +
                    `    window.parent.postMessage({ type: 'APEX_BUILDER_READY' }, '*');\n` +
                    `})();\n` +
                    '<' + '/script>\n';
            }

            // --- THUẬT TOÁN NON-DESTRUCTIVE STRING SPLICING ---
            spliceText(nodeId, newText) {
                const node = this.astMap[nodeId];
                if (!node) return this.rawHTML;

                const before = this.rawHTML.substring(0, node.innerStart);
                const after = this.rawHTML.substring(node.innerEnd);
                this.rawHTML = before + newText + after;
                this.parse(this.rawHTML);
                return this.rawHTML;
            }

            spliceAttribute(nodeId, attrName, attrValue) {
                const node = this.astMap[nodeId];
                if (!node) return this.rawHTML;

                attrName = attrName.toLowerCase();
                const existingAttr = node.attrs[attrName];

                if (existingAttr) {
                    const before = this.rawHTML.substring(0, existingAttr.startOffset);
                    const after = this.rawHTML.substring(existingAttr.endOffset);
                    let replacement = '';
                    if (attrValue !== null && attrValue !== undefined && attrValue !== false && attrValue !== '') {
                        replacement = `${attrName}="${String(attrValue).replace(/"/g, '&quot;')}"`;
                    }
                    this.rawHTML = before + replacement + after;
                } else {
                    if (attrValue !== null && attrValue !== undefined && attrValue !== false && attrValue !== '') {
                        const openRaw = node.openTagRaw;
                        let insertRelPos = openRaw.length - 1;
                        if (openRaw.endsWith('/>')) insertRelPos = openRaw.length - 2;
                        const insertAbsPos = node.openTagStart + insertRelPos;

                        const before = this.rawHTML.substring(0, insertAbsPos);
                        const after = this.rawHTML.substring(insertAbsPos);
                        const insertion = ` ${attrName}="${String(attrValue).replace(/"/g, '&quot;')}"`;
                        this.rawHTML = before + insertion + after;
                    }
                }
                this.parse(this.rawHTML);
                return this.rawHTML;
            }

            spliceClasses(nodeId, classListOrString) {
                const classStr = Array.isArray(classListOrString) ? classListOrString.join(' ') : String(classListOrString);
                return this.spliceAttribute(nodeId, 'class', classStr.trim());
            }

            spliceTagName(nodeId, newTagName) {
                const node = this.astMap[nodeId];
                if (!node || !newTagName) return this.rawHTML;
                newTagName = newTagName.toLowerCase().trim();
                const oldTag = (node.tag || '').toLowerCase();
                if (oldTag === newTagName) return this.rawHTML;

                let html = this.rawHTML;
                // Thay thế thẻ đóng trước (từ sau ra trước để không lệch offset thẻ mở)
                if (node.closeTagStart !== undefined && node.closeTagEnd !== undefined) {
                    const closeBefore = html.substring(0, node.closeTagStart);
                    const closeAfter = html.substring(node.closeTagEnd);
                    html = closeBefore + `</${newTagName}>` + closeAfter;
                }

                // Thay thế thẻ mở: <oldTag -> <newTag
                const openTagSlice = html.substring(node.openTagStart, node.openTagEnd);
                const updatedOpenTag = openTagSlice.replace(new RegExp(`^<${oldTag}(\\s|>|/)`, 'i'), `<${newTagName}$1`);
                html = html.substring(0, node.openTagStart) + updatedOpenTag + html.substring(node.openTagEnd);

                this.rawHTML = html;
                this.parse(this.rawHTML);
                return this.rawHTML;
            }

            spliceStyle(nodeId, styleObj) {
                const node = this.astMap[nodeId];
                if (!node) return this.rawHTML;

                const existingStyleStr = node.attrs['style'] ? node.attrs['style'].value : '';
                const styles = {};
                existingStyleStr.split(';').forEach(s => {
                    const parts = s.split(':');
                    if (parts.length >= 2) {
                        styles[parts[0].trim()] = parts.slice(1).join(':').trim();
                    }
                });

                for (const k in styleObj) {
                    if (styleObj[k] === null || styleObj[k] === '') {
                        delete styles[k];
                    } else {
                        styles[k] = styleObj[k];
                    }
                }

                const newStyleStr = Object.keys(styles).map(k => `${k}: ${styles[k]}`).join('; ');
                return this.spliceAttribute(nodeId, 'style', newStyleStr || null);
            }

            // --- PHÂN HỆ QUẢN LÝ RESPONSIVE STYLES ĐỘC LẬP (PC VS MOBILE CHUẨN THIẾT KẾ) ---
            parseResponsiveStyles() {
                this.pcStylesMap = {};
                this.mobileStylesMap = {};
                if (!this.rawHTML) return;
                const styleMatch = this.rawHTML.match(/<style\s+id=["']__apex_responsive_styles__["'][^>]*>([\s\S]*?)<\/style>/i);
                if (!styleMatch) return;
                const content = styleMatch[1];

                // 1. Phân tích quy tắc PC Desktop: @media (min-width: 769px) { ... }
                const pcMatch = content.match(/@media[^{]*min-width[^{]*\{([\s\S]*?)\}\s*(?=@media|$)/i);
                if (pcMatch) {
                    this.parseCssRulesIntoMap(pcMatch[1], this.pcStylesMap);
                }

                // 2. Phân tích quy tắc Mobile: @media (max-width: 768px) { ... }
                const mobMatch = content.match(/@media[^{]*max-width[^{]*\{([\s\S]*?)\}\s*(?=@media|$)/i);
                if (mobMatch) {
                    this.parseCssRulesIntoMap(mobMatch[1], this.mobileStylesMap);
                }
            }

            parseCssRulesIntoMap(cssBody, targetMap) {
                const ruleRegex = /\[data-ve-id=["']?([a-zA-Z0-9_-]+)["']?\]\s*\{([^}]+)\}/gi;
                let rMatch;
                while ((rMatch = ruleRegex.exec(cssBody)) !== null) {
                    const nodeId = rMatch[1];
                    const declarations = rMatch[2];
                    const styles = {};
                    declarations.split(';').forEach(decl => {
                        const parts = decl.split(':');
                        if (parts.length >= 2) {
                            const prop = parts[0].trim().toLowerCase();
                            const val = parts.slice(1).join(':').replace(/\s*!important\s*$/i, '').trim();
                            if (prop && val) styles[prop] = val;
                        }
                    });
                    if (Object.keys(styles).length > 0) {
                        targetMap[nodeId] = styles;
                    }
                }
            }

            getPCStyles(nodeId) {
                return (this.pcStylesMap && this.pcStylesMap[nodeId]) ? { ...this.pcStylesMap[nodeId] } : {};
            }

            setPCStyles(nodeId, styleObj) {
                if (!this.pcStylesMap) this.pcStylesMap = {};
                if (!this.pcStylesMap[nodeId]) this.pcStylesMap[nodeId] = {};

                for (const prop in styleObj) {
                    const val = styleObj[prop];
                    if (val === null || val === '') {
                        delete this.pcStylesMap[nodeId][prop];
                    } else {
                        this.pcStylesMap[nodeId][prop] = String(val).trim();
                    }
                }

                if (Object.keys(this.pcStylesMap[nodeId]).length === 0) {
                    delete this.pcStylesMap[nodeId];
                } else {
                    // Đảm bảo node sở hữu data-ve-id cố định trong mã HTML gốc để selector [data-ve-id="..."] khớp 100% trên trang live deploy
                    const node = this.astMap[nodeId];
                    if (node && !node.attrs['data-ve-id']) {
                        this.spliceAttribute(nodeId, 'data-ve-id', nodeId);
                    }
                }

                return this.generateAndInjectResponsiveStyles();
            }

            getMobileStyles(nodeId) {
                return (this.mobileStylesMap && this.mobileStylesMap[nodeId]) ? { ...this.mobileStylesMap[nodeId] } : {};
            }

            setMobileStyles(nodeId, styleObj) {
                if (!this.mobileStylesMap) this.mobileStylesMap = {};
                if (!this.mobileStylesMap[nodeId]) this.mobileStylesMap[nodeId] = {};

                for (const prop in styleObj) {
                    const val = styleObj[prop];
                    if (val === null || val === '') {
                        delete this.mobileStylesMap[nodeId][prop];
                    } else {
                        this.mobileStylesMap[nodeId][prop] = String(val).trim();
                    }
                }

                if (Object.keys(this.mobileStylesMap[nodeId]).length === 0) {
                    delete this.mobileStylesMap[nodeId];
                } else {
                    // Đảm bảo node sở hữu data-ve-id cố định trong mã HTML gốc để selector [data-ve-id="..."] khớp 100% trên trang live deploy
                    const node = this.astMap[nodeId];
                    if (node && !node.attrs['data-ve-id']) {
                        this.spliceAttribute(nodeId, 'data-ve-id', nodeId);
                    }
                }

                return this.generateAndInjectResponsiveStyles();
            }

            getDeviceStyles(nodeId, mode = 'pc') {
                return mode === 'mobile' ? this.getMobileStyles(nodeId) : this.getPCStyles(nodeId);
            }

            setDeviceStyles(nodeId, styleObj, mode = 'pc') {
                return mode === 'mobile' ? this.setMobileStyles(nodeId, styleObj) : this.setPCStyles(nodeId, styleObj);
            }

            generateAndInjectResponsiveStyles() {
                // 1. Tạo quy tắc cho PC Desktop (> 768px)
                const pcRules = [];
                for (const nId in this.pcStylesMap) {
                    const s = this.pcStylesMap[nId];
                    const decls = Object.keys(s).map(p => `        ${p}: ${s[p]} !important;`).join('\n');
                    if (decls) {
                        pcRules.push(`    [data-ve-id="${nId}"] {\n${decls}\n    }`);
                    }
                }

                // 2. Tạo quy tắc cho Mobile (<= 768px)
                const mobRules = [];
                for (const nId in this.mobileStylesMap) {
                    const s = this.mobileStylesMap[nId];
                    const decls = Object.keys(s).map(p => `        ${p}: ${s[p]} !important;`).join('\n');
                    if (decls) {
                        mobRules.push(`    [data-ve-id="${nId}"] {\n${decls}\n    }`);
                    }
                }

                const cssBlocks = [];
                if (pcRules.length > 0) {
                    cssBlocks.push(`@media (min-width: 769px) {\n    /* Apex Studio PC Desktop Styles (Chỉ áp dụng PC > 768px) */\n${pcRules.join('\n')}\n}`);
                }
                if (mobRules.length > 0) {
                    cssBlocks.push(`@media (max-width: 768px) {\n    /* Apex Studio Mobile Responsive Styles (Chỉ áp dụng Mobile <= 768px) */\n${mobRules.join('\n')}\n}`);
                }

                const cssBody = cssBlocks.join('\n\n');
                const tagStart = '<style id="__apex_responsive_styles__">';
                const tagEnd = '</style>';
                const fullBlock = cssBody ? `${tagStart}\n${cssBody}\n${tagEnd}` : '';

                const existingMatch = this.rawHTML.match(/<style\s+id=["']__apex_responsive_styles__["'][^>]*>[\s\S]*?<\/style>/i);
                if (existingMatch) {
                    if (fullBlock) {
                        this.rawHTML = this.rawHTML.replace(existingMatch[0], fullBlock);
                    } else {
                        this.rawHTML = this.rawHTML.replace(existingMatch[0], '');
                    }
                } else if (fullBlock) {
                    const headCloseIdx = this.rawHTML.toLowerCase().indexOf('</head>');
                    if (headCloseIdx !== -1) {
                        this.rawHTML = this.rawHTML.substring(0, headCloseIdx) + '    ' + fullBlock + '\n' + this.rawHTML.substring(headCloseIdx);
                    } else {
                        const bodyCloseIdx = this.rawHTML.toLowerCase().indexOf('</body>');
                        if (bodyCloseIdx !== -1) {
                            this.rawHTML = this.rawHTML.substring(0, bodyCloseIdx) + '    ' + fullBlock + '\n' + this.rawHTML.substring(bodyCloseIdx);
                        } else {
                            this.rawHTML += '\n' + fullBlock;
                        }
                    }
                }

                this.parse(this.rawHTML);
                return { html: this.rawHTML, css: cssBody };
            }

            deleteNode(nodeId) {
                const node = this.astMap[nodeId];
                if (!node) return this.rawHTML;

                this.rawHTML = this.rawHTML.substring(0, node.startOffset) + this.rawHTML.substring(node.endOffset);
                this.parse(this.rawHTML);
                return this.rawHTML;
            }

            duplicateNode(nodeId) {
                const node = this.astMap[nodeId];
                if (!node) return this.rawHTML;

                const nodeStr = this.rawHTML.substring(node.startOffset, node.endOffset);
                this.rawHTML = this.rawHTML.substring(0, node.endOffset) + '\n' + nodeStr + this.rawHTML.substring(node.endOffset);
                this.parse(this.rawHTML);
                return this.rawHTML;
            }

            moveNodeUp(nodeId) {
                const node = this.astMap[nodeId];
                if (!node) return { html: this.rawHTML, newNodeId: nodeId, toString() { return this.html; } };

                const parentId = node.parentId;
                const siblings = (parentId && this.astMap[parentId])
                    ? this.astMap[parentId].children
                    : this.tree;
                const idx = siblings.indexOf(nodeId);
                if (idx <= 0) return { html: this.rawHTML, newNodeId: nodeId, toString() { return this.html; } };

                const prevNode = this.astMap[siblings[idx - 1]];
                if (!prevNode) return { html: this.rawHTML, newNodeId: nodeId, toString() { return this.html; } };

                const pStart = prevNode.startOffset;
                const pEnd = prevNode.endOffset;
                const cStart = node.startOffset;
                const cEnd = node.endOffset;

                const prevText = this.rawHTML.substring(pStart, pEnd);
                const inBetween = this.rawHTML.substring(pEnd, cStart);
                const currText = this.rawHTML.substring(cStart, cEnd);

                this.rawHTML = this.rawHTML.substring(0, pStart) + currText + inBetween + prevText + this.rawHTML.substring(cEnd);
                this.parse(this.rawHTML);

                const newSiblings = (parentId && this.astMap[parentId])
                    ? this.astMap[parentId].children
                    : this.tree;
                const newNodeId = (newSiblings && newSiblings[idx - 1]) ? newSiblings[idx - 1] : nodeId;
                this.lastMovedNodeId = newNodeId;

                return { html: this.rawHTML, newNodeId: newNodeId, toString() { return this.html; } };
            }

            moveNodeDown(nodeId) {
                const node = this.astMap[nodeId];
                if (!node) return { html: this.rawHTML, newNodeId: nodeId, toString() { return this.html; } };

                const parentId = node.parentId;
                const siblings = (parentId && this.astMap[parentId])
                    ? this.astMap[parentId].children
                    : this.tree;
                const idx = siblings.indexOf(nodeId);
                if (idx === -1 || idx >= siblings.length - 1) return { html: this.rawHTML, newNodeId: nodeId, toString() { return this.html; } };

                const nextNode = this.astMap[siblings[idx + 1]];
                if (!nextNode) return { html: this.rawHTML, newNodeId: nodeId, toString() { return this.html; } };

                const cStart = node.startOffset;
                const cEnd = node.endOffset;
                const nStart = nextNode.startOffset;
                const nEnd = nextNode.endOffset;

                const currText = this.rawHTML.substring(cStart, cEnd);
                const inBetween = this.rawHTML.substring(cEnd, nStart);
                const nextText = this.rawHTML.substring(nStart, nEnd);

                this.rawHTML = this.rawHTML.substring(0, cStart) + nextText + inBetween + currText + this.rawHTML.substring(nEnd);
                this.parse(this.rawHTML);

                const newSiblings = (parentId && this.astMap[parentId])
                    ? this.astMap[parentId].children
                    : this.tree;
                const newNodeId = (newSiblings && newSiblings[idx + 1]) ? newSiblings[idx + 1] : nodeId;
                this.lastMovedNodeId = newNodeId;

                return { html: this.rawHTML, newNodeId: newNodeId, toString() { return this.html; } };
            }

            moveNodeToTop(nodeId) {
                const node = this.astMap[nodeId];
                if (!node) return { html: this.rawHTML, newNodeId: nodeId, toString() { return this.html; } };

                const parentId = node.parentId;
                const siblings = (parentId && this.astMap[parentId])
                    ? this.astMap[parentId].children
                    : this.tree;
                const idx = siblings.indexOf(nodeId);
                if (idx <= 0) return { html: this.rawHTML, newNodeId: nodeId, toString() { return this.html; } };

                const firstNode = this.astMap[siblings[0]];
                if (!firstNode) return { html: this.rawHTML, newNodeId: nodeId, toString() { return this.html; } };

                const cStart = node.startOffset;
                const cEnd = node.endOffset;
                const fStart = firstNode.startOffset;

                const currText = this.rawHTML.substring(cStart, cEnd);
                const beforeFirst = this.rawHTML.substring(0, fStart);
                const between = this.rawHTML.substring(fStart, cStart);
                const afterCurr = this.rawHTML.substring(cEnd);

                this.rawHTML = beforeFirst + currText + '\n' + between + afterCurr;
                this.parse(this.rawHTML);

                const newSiblings = (parentId && this.astMap[parentId])
                    ? this.astMap[parentId].children
                    : this.tree;
                const newNodeId = (newSiblings && newSiblings[0]) ? newSiblings[0] : nodeId;
                this.lastMovedNodeId = newNodeId;

                return { html: this.rawHTML, newNodeId: newNodeId, toString() { return this.html; } };
            }

            moveNodeToBottom(nodeId) {
                const node = this.astMap[nodeId];
                if (!node) return { html: this.rawHTML, newNodeId: nodeId, toString() { return this.html; } };

                const parentId = node.parentId;
                const siblings = (parentId && this.astMap[parentId])
                    ? this.astMap[parentId].children
                    : this.tree;
                const idx = siblings.indexOf(nodeId);
                if (idx === -1 || idx >= siblings.length - 1) return { html: this.rawHTML, newNodeId: nodeId, toString() { return this.html; } };

                const lastNode = this.astMap[siblings[siblings.length - 1]];
                if (!lastNode) return { html: this.rawHTML, newNodeId: nodeId, toString() { return this.html; } };

                const cStart = node.startOffset;
                const cEnd = node.endOffset;
                const lEnd = lastNode.endOffset;

                const currText = this.rawHTML.substring(cStart, cEnd);
                const beforeCurr = this.rawHTML.substring(0, cStart);
                const between = this.rawHTML.substring(cEnd, lEnd);
                const afterLast = this.rawHTML.substring(lEnd);

                this.rawHTML = beforeCurr + between + '\n' + currText + afterLast;
                this.parse(this.rawHTML);

                const newSiblings = (parentId && this.astMap[parentId])
                    ? this.astMap[parentId].children
                    : this.tree;
                const newNodeId = (newSiblings && newSiblings[newSiblings.length - 1]) ? newSiblings[newSiblings.length - 1] : nodeId;
                this.lastMovedNodeId = newNodeId;

                return { html: this.rawHTML, newNodeId: newNodeId, toString() { return this.html; } };
            }

            changeNodeTag(nodeId, newTagName) {
                const node = this.astMap[nodeId];
                if (!node || !newTagName) return this.rawHTML;
                newTagName = newTagName.toLowerCase().trim();
                const oldTag = node.tag.toLowerCase();
                if (oldTag === newTagName) return this.rawHTML;

                let html = this.rawHTML;

                // 1. Đổi thẻ đóng trước (từ phải sang trái để không lệch offset phía trước)
                if (node.closeTagStart !== undefined && node.closeTagEnd !== undefined) {
                    const closeBefore = html.substring(0, node.closeTagStart);
                    const closeAfter = html.substring(node.closeTagEnd);
                    html = closeBefore + '</' + newTagName + '>' + closeAfter;
                }

                // 2. Đổi tên thẻ mở
                const tagStart = node.openTagStart + 1; // sau dấu '<'
                const tagEnd = tagStart + oldTag.length;
                const openBefore = html.substring(0, tagStart);
                const openAfter = html.substring(tagEnd);
                html = openBefore + newTagName + openAfter;

                this.rawHTML = html;
                this.parse(this.rawHTML);
                return this.rawHTML;
            }

            insertNodeAfter(targetNodeId, htmlSnippet) {
                const node = this.astMap[targetNodeId];
                if (!node || !htmlSnippet) return this.rawHTML;

                const before = this.rawHTML.substring(0, node.endOffset);
                const after = this.rawHTML.substring(node.endOffset);
                this.rawHTML = before + '\n' + htmlSnippet + after;
                this.parse(this.rawHTML);
                return this.rawHTML;
            }

            getNodePath(nodeId) {
                const path = [];
                let curr = this.astMap[nodeId];
                while (curr) {
                    path.unshift(curr);
                    curr = curr.parentId ? this.astMap[curr.parentId] : null;
                }
                return path;
            }
        }

