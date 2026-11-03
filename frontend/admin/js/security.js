/* ==============================================================
   APEX ADMIN - SECURITY DASHBOARD, ANTI-BOT & QUARANTINE
   ============================================================== */

        // 14. Security Quarantine Dashboard Loader
        let curQuarantine = [];
        async function loadSecurityDashboard() {
            try {
                const resStats = await fetch('/api/security/stats');
                if (resStats.ok) {
                    const jsonStats = await resStats.json();
                    if (jsonStats && jsonStats.data) {
                        const s = jsonStats.data;
                        const total = s.total_spam_blocked || s.total_quarantine || 0;
                        document.getElementById('secTotalBlocked').innerText = total;
                        document.getElementById('badgeSpamCount').innerText = total;

                        let honeypotCount = 0;
                        let speedCount = 0;
                        let powCount = 0;
                        const rb = s.reasons_breakdown || {};
                        Object.keys(rb).forEach(k => {
                            const val = rb[k] || 0;
                            if (k.includes('honeypot')) honeypotCount += val;
                            else if (k.includes('rapid') || k.includes('speed')) speedCount += val;
                            else if (k.includes('pow')) powCount += val;
                        });
                        document.getElementById('secHoneypotCount').innerText = honeypotCount;
                        document.getElementById('secSpeedCount').innerText = speedCount;
                        document.getElementById('secPowCount').innerText = powCount;
                    }
                }

                const resLog = await fetch('/api/security/quarantine?limit=50');
                if (resLog.ok) {
                    const jsonLog = await resLog.json();
                    curQuarantine = jsonLog.data || [];
                    const tbody = document.getElementById('quarantineTableBody');
                    if (!tbody) return;

                    if (curQuarantine.length === 0) {
                        tbody.innerHTML = '<tr><td colspan="7" class="py-8 text-center text-slate-500">Vùng cách ly hiện tại không có request độc hại.</td></tr>';
                        return;
                    }

                    tbody.innerHTML = curQuarantine.map((item, idx) => {
                        let reasonBadge = `<span class="bg-slate-800 text-slate-300 text-xs px-2 py-0.5 rounded-full font-mono">${escapeHTML(item.reason)}</span>`;
                        if (item.reason.includes('honeypot')) {
                            reasonBadge = `<span class="bg-rose-950 text-rose-400 border border-rose-800 text-xs px-2 py-0.5 rounded-full font-semibold"><i class="fa-solid fa-spider mr-1"></i>Bẫy Honeypot</span>`;
                        } else if (item.reason.includes('rapid_submission')) {
                            reasonBadge = `<span class="bg-amber-950 text-amber-400 border border-amber-800 text-xs px-2 py-0.5 rounded-full font-semibold"><i class="fa-solid fa-bolt mr-1"></i>Bot Siêu Tốc (&lt;800ms)</span>`;
                        } else if (item.reason.includes('rate_limit')) {
                            reasonBadge = `<span class="bg-purple-950 text-purple-400 border border-purple-800 text-xs px-2 py-0.5 rounded-full font-semibold"><i class="fa-solid fa-gauge-high mr-1"></i>Vượt Rate-Limit</span>`;
                        } else if (item.reason.includes('pow')) {
                            reasonBadge = `<span class="bg-red-950 text-red-400 border border-red-800 text-xs px-2 py-0.5 rounded-full font-semibold"><i class="fa-solid fa-key mr-1"></i>Lỗi Proof-of-Work</span>`;
                        } else if (item.reason.includes('phone')) {
                            reasonBadge = `<span class="bg-orange-950 text-orange-400 border border-orange-800 text-xs px-2 py-0.5 rounded-full font-semibold"><i class="fa-solid fa-phone-slash mr-1"></i>SĐT Rác/Ảo</span>`;
                        }

                        return `
                            <tr class="hover:bg-slate-800/60 transition text-xs">
                                <td class="py-3 px-4 text-center font-mono text-slate-500">#${item.id}</td>
                                <td class="py-3 px-4 font-mono text-slate-300 whitespace-nowrap">${escapeHTML(item.created_at)}</td>
                                <td class="py-3 px-4 font-mono font-semibold text-rose-300">${escapeHTML(item.ip_address || '-')}</td>
                                <td class="py-3 px-4 font-mono font-bold text-slate-200">${escapeHTML(item.phone || '-')}</td>
                                <td class="py-3 px-4">${reasonBadge}</td>
                                <td class="py-3 px-4 text-slate-400 text-[11px] truncate max-w-xs" title="${escapeHTML(item.user_agent)}">${escapeHTML(item.user_agent || '-')}</td>
                                <td class="py-3 px-4 text-center">
                                    <button onclick="viewQuarantinePayload(${idx})" class="text-orange-400 hover:text-orange-300 bg-orange-950/40 border border-orange-800/60 px-2.5 py-1 rounded text-[11px] font-semibold transition">
                                        <i class="fa-solid fa-code mr-1"></i>Payload
                                    </button>
                                </td>
                            </tr>
                        `;
                    }).join('');
                }
            } catch (_) {}
        }

        function viewQuarantinePayload(idx) {
            const item = curQuarantine[idx];
            if (!item) return;
            document.getElementById('quarantineModalPayload').innerText = JSON.stringify(item.payload, null, 2);
            document.getElementById('quarantineModal').classList.remove('hidden');
        }

        function closeQuarantineModal() {
            document.getElementById('quarantineModal').classList.add('hidden');
        }

