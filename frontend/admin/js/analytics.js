/* ==============================================================
   APEX ADMIN - ANALYTICS, CHARTS, GEO HEATMAP & SESSIONS REPLAY
   ============================================================== */

        // Chart instances
        let chartHourly = null;
        let chartDaily = null;
        let chartStatus = null;
        let chartSource = null;
        let chartCarrier = null;
        let chartAutofill = null;
        let chartCities = null;
        let chartRegions = null;
        let chartISPs = null;
        let chartAdPlacements = null;
        let chartPlacementCR = null;



        // 12. CHARTS RENDERING (Chart.js)
        function renderHourlyChart(hourlyData, peakHourStr) {
            const ctx = document.getElementById('chartHourlyCanvas');
            if (!ctx) return;

            const labels = hourlyData.map(d => d.label);
            const data = hourlyData.map(d => d.count);
            const maxVal = Math.max(...data, 1);

            // Highlight peak hour
            const bgColors = data.map(v => (v > 0 && v === maxVal) ? '#ea580c' : 'rgba(234, 88, 12, 0.45)');
            const borderColors = data.map(v => (v > 0 && v === maxVal) ? '#f97316' : '#c2410c');

            if (chartHourly) chartHourly.destroy();

            chartHourly = new Chart(ctx, {
                type: 'bar',
                data: {
                    labels: labels,
                    datasets: [{
                        label: 'Số lượng leads',
                        data: data,
                        backgroundColor: bgColors,
                        borderColor: borderColors,
                        borderWidth: 1.5,
                        borderRadius: 6
                    }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: {
                        legend: { display: false },
                        tooltip: {
                            callbacks: {
                                title: items => `Khung giờ: ${items[0].label} - ${String((parseInt(items[0].label, 10) + 1) % 24).padStart(2, '0')}:00`,
                                label: item => `Leads ghi nhận: ${item.formattedValue}`
                            }
                        }
                    },
                    scales: {
                        x: {
                            grid: { color: 'rgba(51, 65, 85, 0.3)' },
                            ticks: { color: '#94a3b8', font: { size: 10 } }
                        },
                        y: {
                            beginAtZero: true,
                            grid: { color: 'rgba(51, 65, 85, 0.3)' },
                            ticks: { color: '#94a3b8', precision: 0, font: { size: 10 } }
                        }
                    }
                }
            });
        }

        function renderDailyTrendChart(dailyData) {
            const ctx = document.getElementById('chartDailyCanvas');
            if (!ctx) return;

            const labels = dailyData.map(d => d.date.slice(5));
            const data = dailyData.map(d => d.count);

            if (chartDaily) chartDaily.destroy();

            chartDaily = new Chart(ctx, {
                type: 'line',
                data: {
                    labels: labels,
                    datasets: [{
                        label: 'Leads mới',
                        data: data,
                        borderColor: '#10b981',
                        backgroundColor: 'rgba(16, 185, 129, 0.15)',
                        fill: true,
                        tension: 0.35,
                        pointBackgroundColor: '#10b981',
                        pointBorderColor: '#ffffff',
                        pointRadius: 4,
                        borderWidth: 2.5
                    }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: {
                        legend: { display: false }
                    },
                    scales: {
                        x: {
                            grid: { color: 'rgba(51, 65, 85, 0.3)' },
                            ticks: { color: '#94a3b8', font: { size: 10 } }
                        },
                        y: {
                            beginAtZero: true,
                            grid: { color: 'rgba(51, 65, 85, 0.3)' },
                            ticks: { color: '#94a3b8', precision: 0, font: { size: 10 } }
                        }
                    }
                }
            });
        }

        function renderStatusChart(statusCounts) {
            const ctx = document.getElementById('chartStatusCanvas');
            if (!ctx) return;

            const labels = ['Mới', 'Đã gọi', 'Zalo', 'Vào Zoom', 'Tiềm năng', 'Chốt HĐ', 'Hủy'];
            const keys = ['moi', 'da_goi', 'ket_ban_zalo', 'da_vao_zoom', 'tiem_nang', 'chot', 'huy'];
            const data = keys.map(k => statusCounts[k] || 0);
            const colors = ['#38bdf8', '#fbbf24', '#60a5fa', '#c084fc', '#fb923c', '#34d399', '#f87171'];

            if (chartStatus) chartStatus.destroy();

            chartStatus = new Chart(ctx, {
                type: 'doughnut',
                data: {
                    labels: labels,
                    datasets: [{
                        data: data,
                        backgroundColor: colors,
                        borderWidth: 2,
                        borderColor: '#0f172a'
                    }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: {
                        legend: {
                            position: 'bottom',
                            labels: { color: '#cbd5e1', font: { size: 10 }, boxWidth: 10 }
                        }
                    },
                    cutout: '62%'
                }
            });
        }

        function renderSourceChart(sourceCounts) {
            const ctx = document.getElementById('chartSourceCanvas');
            if (!ctx) return;

            const labels = (sourceCounts || []).map(s => s.source);
            const data = (sourceCounts || []).map(s => s.count);

            if (chartSource) chartSource.destroy();

            chartSource = new Chart(ctx, {
                type: 'bar',
                data: {
                    labels: labels,
                    datasets: [{
                        label: 'Leads',
                        data: data,
                        backgroundColor: '#f59e0b',
                        borderRadius: 5
                    }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    indexAxis: 'y',
                    plugins: { legend: { display: false } },
                    scales: {
                        x: { beginAtZero: true, grid: { color: 'rgba(51, 65, 85, 0.3)' }, ticks: { color: '#94a3b8', font: { size: 9 }, precision: 0 } },
                        y: { grid: { display: false }, ticks: { color: '#cbd5e1', font: { size: 10 } } }
                    }
                }
            });
        }

        function renderCarrierChart(carrierCounts) {
            const ctx = document.getElementById('chartCarrierCanvas');
            if (!ctx) return;

            const labels = Object.keys(carrierCounts || {});
            const data = Object.values(carrierCounts || {});
            const colors = ['#ef4444', '#3b82f6', '#f59e0b', '#f97316', '#64748b'];

            if (chartCarrier) chartCarrier.destroy();

            chartCarrier = new Chart(ctx, {
                type: 'pie',
                data: {
                    labels: labels,
                    datasets: [{
                        data: data,
                        backgroundColor: colors,
                        borderWidth: 2,
                        borderColor: '#0f172a'
                    }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: {
                        legend: { position: 'bottom', labels: { color: '#cbd5e1', font: { size: 10 }, boxWidth: 10 } }
                    }
                }
            });
        }

        function renderAutofillChart(af) {
            const ctx = document.getElementById('chartAutofillCanvas');
            if (!ctx) return;

            const data = [af.autofill || 0, af.manual || 0];

            if (chartAutofill) chartAutofill.destroy();

            chartAutofill = new Chart(ctx, {
                type: 'doughnut',
                data: {
                    labels: ['Tự động điền 1-Tap', 'Gõ tay thông thường'],
                    datasets: [{
                        data: data,
                        backgroundColor: ['#3b82f6', '#475569'],
                        borderWidth: 2,
                        borderColor: '#0f172a'
                    }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: {
                        legend: { position: 'bottom', labels: { color: '#cbd5e1', font: { size: 10 }, boxWidth: 10 } }
                    },
                    cutout: '60%'
                }
            });
        }

        // renderFunnelChart vẽ phễu chuyển đổi.
        //
        // ⚠️ 2026-11-12: TRƯỚC ĐÂY dùng số ước lượng `totalLeads * 8` ⇒ phễu
        // hiển thị sai hoàn toàn so với dữ liệu thật. Nay nhận `funnel` từ
        // API `/api/crm/stats` (đếm trực tiếp trên `analytics_sessions`).
        // Nếu API không trả funnel ⇒ fallback về 0 chứ không bịa số.
        function renderFunnelChart(totalLeads, highIntent, closed, funnel) {
            const container = document.getElementById('funnelContainer');
            if (!container) return;

            const f = funnel || {};
            const views = Number(f.views || 0);
            const scrolled = Number(f.scrolled || 0);
            const interacted = Number(f.interacted || 0);
            const converted = Number(f.converted || totalLeads || 0);
            const closedCount = Number(f.closed || closed || 0);

            const pct = (n, base) => base > 0 ? Math.round((n / base) * 100) : 0;

            const steps = [
                { name: '1. Khách Xem Trang (Lượt Xem Thật)', count: views, percent: 100, color: 'bg-blue-600' },
                { name: '2. Cuộn Trang Xem Nội Dung (>50%)', count: scrolled, percent: pct(scrolled, views), color: 'bg-indigo-600' },
                { name: '3. Tương Tác Ô Nhập Form', count: interacted, percent: pct(interacted, views), color: 'bg-purple-600' },
                { name: '4. Đăng Ký Lead Thành Công', count: converted, percent: pct(converted, views), color: 'bg-orange-500' },
                { name: '5. Khách Hàng Tiềm Năng & Chốt HĐ', count: closedCount, percent: pct(closedCount, converted), color: 'bg-emerald-500' }
            ];

            container.innerHTML = steps.map(s => `
                <div class="space-y-1">
                    <div class="flex justify-between text-xs">
                        <span class="font-semibold text-slate-300">${s.name}</span>
                        <span class="font-mono font-bold text-white">${s.count.toLocaleString()} (${s.percent}%)</span>
                    </div>
                    <div class="w-full bg-slate-950 rounded-full h-3 overflow-hidden border border-slate-800">
                        <div class="${s.color} h-3 rounded-full transition-all duration-500" style="width: ${Math.max(s.percent, 3)}%"></div>
                    </div>
                </div>
            `).join('');
        }

        // 12b. GEO & AD PLACEMENT ANALYTICS LOADERS & CHARTS
        async function loadGeoAnalyticsCharts() {
            try {
                const p = buildFilterParams();
                const res = await fetch('/api/analytics/geo?' + p.toString());
                if (!res.ok) return;
                const json = await res.json();
                const d = json.data || {};
                renderCitiesChart(d.cities || []);
                renderRegionsChart(d.regions || []);
                renderGeoPerformanceTable(d.cities || []);
            } catch (err) {
                console.error("Lỗi nạp Geo Analytics:", err);
            }
        }

        function renderCitiesChart(cities) {
            const ctx = document.getElementById('chartCitiesCanvas');
            if (!ctx) return;

            const topCities = (cities || []).slice(0, 10);
            if (topCities.length === 0) {
                if (chartCities) chartCities.destroy();
                return;
            }

            const labels = topCities.map(c => c.name);
            const views = topCities.map(c => c.total_views);
            const humans = topCities.map(c => c.human_visitors);
            const leads = topCities.map(c => c.leads);

            if (chartCities) chartCities.destroy();

            chartCities = new Chart(ctx, {
                type: 'bar',
                data: {
                    labels: labels,
                    datasets: [
                        {
                            label: 'Lượt Xem (Views)',
                            data: views,
                            backgroundColor: 'rgba(59, 130, 246, 0.75)',
                            borderColor: '#3b82f6',
                            borderWidth: 1.5,
                            borderRadius: 4
                        },
                        {
                            label: 'Khách Thật (Human)',
                            data: humans,
                            backgroundColor: 'rgba(16, 185, 129, 0.75)',
                            borderColor: '#10b981',
                            borderWidth: 1.5,
                            borderRadius: 4
                        },
                        {
                            label: 'Leads Chuyển Đổi',
                            data: leads,
                            backgroundColor: 'rgba(249, 115, 22, 0.9)',
                            borderColor: '#f97316',
                            borderWidth: 1.5,
                            borderRadius: 4
                        }
                    ]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: {
                        legend: {
                            position: 'top',
                            labels: { color: '#cbd5e1', font: { size: 11 }, boxWidth: 12 }
                        },
                        tooltip: {
                            callbacks: {
                                afterBody: items => {
                                    const idx = items[0].dataIndex;
                                    const c = topCities[idx];
                                    return `Vùng: ${c.region || ''} | CR%: ${c.conversion_rate || 0}%`;
                                }
                            }
                        }
                    },
                    scales: {
                        x: {
                            grid: { color: 'rgba(51, 65, 85, 0.25)' },
                            ticks: { color: '#94a3b8', font: { size: 10 } }
                        },
                        y: {
                            beginAtZero: true,
                            grid: { color: 'rgba(51, 65, 85, 0.25)' },
                            ticks: { color: '#94a3b8', precision: 0, font: { size: 10 } }
                        }
                    }
                }
            });
        }

        function renderRegionsChart(regions) {
            const ctx = document.getElementById('chartRegionsCanvas');
            if (!ctx) return;

            const rData = regions || [];
            if (rData.length === 0) {
                if (chartRegions) chartRegions.destroy();
                return;
            }

            const labels = rData.map(r => r.name);
            const data = rData.map(r => r.total_views);

            const colorMap = {
                'Miền Bắc': '#3b82f6',
                'Miền Trung': '#f59e0b',
                'Miền Nam': '#10b981',
                'Quốc Tế': '#8b5cf6'
            };
            const bgColors = labels.map(l => colorMap[l] || '#64748b');

            if (chartRegions) chartRegions.destroy();

            chartRegions = new Chart(ctx, {
                type: 'doughnut',
                data: {
                    labels: labels,
                    datasets: [{
                        data: data,
                        backgroundColor: bgColors,
                        borderWidth: 2,
                        borderColor: '#0f172a'
                    }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: {
                        legend: { position: 'bottom', labels: { color: '#cbd5e1', font: { size: 10 }, boxWidth: 10 } },
                        tooltip: {
                            callbacks: {
                                label: item => {
                                    const r = rData[item.dataIndex];
                                    return ` ${r.name}: ${r.total_views} views (${r.leads} leads - CR: ${r.conversion_rate}%)`;
                                }
                            }
                        }
                    },
                    cutout: '60%'
                }
            });
        }

        async function loadAdPlacementAnalyticsCharts() {
            try {
                const p = buildFilterParams();
                const res = await fetch('/api/analytics/ad-placements?' + p.toString());
                if (!res.ok) return;
                const json = await res.json();
                const d = json.data || {};
                renderAdPlacementsChart(d.placements || []);
                renderPlacementCRChart(d.placements || []);
            } catch (err) {
                console.error("Lỗi nạp Ad Placement Analytics:", err);
            }
        }

        function renderAdPlacementsChart(placements) {
            const ctx = document.getElementById('chartAdPlacementsCanvas');
            if (!ctx) return;

            const topP = (placements || []).slice(0, 10);
            if (topP.length === 0) {
                if (chartAdPlacements) chartAdPlacements.destroy();
                return;
            }

            const labels = topP.map(p => `${p.placement}`);
            const views = topP.map(p => p.total_views);
            const leads = topP.map(p => p.leads);

            if (chartAdPlacements) chartAdPlacements.destroy();

            chartAdPlacements = new Chart(ctx, {
                type: 'bar',
                data: {
                    labels: labels,
                    datasets: [
                        {
                            label: 'Lượt Xem (Views)',
                            data: views,
                            backgroundColor: 'rgba(56, 189, 248, 0.75)',
                            borderColor: '#38bdf8',
                            borderWidth: 1.5,
                            borderRadius: 4
                        },
                        {
                            label: 'Leads Thu Được',
                            data: leads,
                            backgroundColor: 'rgba(244, 63, 94, 0.85)',
                            borderColor: '#f43f5e',
                            borderWidth: 1.5,
                            borderRadius: 4
                        }
                    ]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: {
                        legend: { position: 'top', labels: { color: '#cbd5e1', font: { size: 11 }, boxWidth: 12 } },
                        tooltip: {
                            callbacks: {
                                afterBody: items => {
                                    const p = topP[items[0].dataIndex];
                                    return `Nền tảng: ${p.platform} | CR: ${p.conversion_rate}%`;
                                }
                            }
                        }
                    },
                    scales: {
                        x: {
                            grid: { color: 'rgba(51, 65, 85, 0.25)' },
                            ticks: { color: '#94a3b8', font: { size: 9 }, maxRotation: 25 }
                        },
                        y: {
                            beginAtZero: true,
                            grid: { color: 'rgba(51, 65, 85, 0.25)' },
                            ticks: { color: '#94a3b8', precision: 0, font: { size: 10 } }
                        }
                    }
                }
            });
        }

        function renderPlacementCRChart(placements) {
            const ctx = document.getElementById('chartPlacementCRCanvas');
            if (!ctx) return;

            const valid = (placements || []).filter(p => p.total_views > 0).sort((a, b) => b.conversion_rate - a.conversion_rate).slice(0, 8);
            if (valid.length === 0) {
                if (chartPlacementCR) chartPlacementCR.destroy();
                return;
            }

            const labels = valid.map(p => p.placement.length > 20 ? p.placement.slice(0, 18) + '...' : p.placement);
            const crData = valid.map(p => p.conversion_rate);
            const bgColors = valid.map(p => p.conversion_rate >= 15 ? 'rgba(16, 185, 129, 0.85)' : (p.conversion_rate >= 8 ? 'rgba(59, 130, 246, 0.85)' : 'rgba(245, 158, 11, 0.85)'));

            if (chartPlacementCR) chartPlacementCR.destroy();

            chartPlacementCR = new Chart(ctx, {
                type: 'bar',
                data: {
                    labels: labels,
                    datasets: [{
                        label: 'CR %',
                        data: crData,
                        backgroundColor: bgColors,
                        borderRadius: 4
                    }]
                },
                options: {
                    indexAxis: 'y',
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: {
                        legend: { display: false },
                        tooltip: {
                            callbacks: {
                                label: item => ` Tỷ lệ chuyển đổi: ${item.formattedValue}%`
                            }
                        }
                    },
                    scales: {
                        x: {
                            beginAtZero: true,
                            grid: { color: 'rgba(51, 65, 85, 0.25)' },
                            ticks: {
                                color: '#94a3b8',
                                callback: v => v + '%',
                                font: { size: 9 }
                            }
                        },
                        y: {
                            grid: { display: false },
                            ticks: { color: '#cbd5e1', font: { size: 9 } }
                        }
                    }
                }
            });
        }

        function renderGeoPerformanceTable(cities) {
            const tbody = document.getElementById('geoPerfBody');
            if (!tbody) return;

            if (!cities || cities.length === 0) {
                tbody.innerHTML = '<tr><td colspan="7" class="py-6 text-center text-slate-500">Chưa có dữ liệu địa lý IP</td></tr>';
                return;
            }

            tbody.innerHTML = cities.map(c => {
                let crBadge = 'bg-slate-800 text-slate-400 border-slate-700';
                if (c.conversion_rate >= 15) crBadge = 'bg-emerald-950 text-emerald-400 border-emerald-800';
                else if (c.conversion_rate >= 8) crBadge = 'bg-blue-950 text-blue-400 border-blue-800';
                else if (c.conversion_rate > 0) crBadge = 'bg-amber-950 text-amber-400 border-amber-800';

                let evalBadge = '<span class="text-slate-500 text-xs">Cần thêm data</span>';
                if (c.leads >= 5 || c.conversion_rate >= 15) {
                    evalBadge = '<span class="bg-emerald-950/70 text-emerald-300 border border-emerald-800 text-[11px] px-2 py-0.5 rounded-full font-semibold"><i class="fa-solid fa-fire text-amber-400 mr-1"></i>Trọng Điểm & Tiềm Năng</span>';
                } else if (c.leads >= 1 || c.conversion_rate >= 5) {
                    evalBadge = '<span class="bg-blue-950/70 text-blue-300 border border-blue-800 text-[11px] px-2 py-0.5 rounded-full font-semibold"><i class="fa-solid fa-chart-line text-blue-400 mr-1"></i>Ổn Định</span>';
                } else if (c.total_views >= 10 && c.leads === 0) {
                    evalBadge = '<span class="bg-rose-950/70 text-rose-300 border border-rose-800 text-[11px] px-2 py-0.5 rounded-full font-semibold"><i class="fa-solid fa-triangle-exclamation text-rose-400 mr-1"></i>Cần Tối Ưu QC</span>';
                }

                return `
                    <tr class="hover:bg-slate-800/60 transition text-xs">
                        <td class="py-3 px-4 font-bold text-slate-200">
                            <i class="fa-solid fa-location-dot text-rose-400 mr-2"></i>${escapeHTML(c.name)}
                        </td>
                        <td class="py-3 px-4">
                            <span class="text-xs px-2 py-0.5 rounded font-bold ${getRegionBadgeClass(c.region)}">${escapeHTML(c.region || 'Miền Bắc')}</span>
                        </td>
                        <td class="py-3 px-4 text-center font-mono text-slate-300 font-semibold">${c.total_views.toLocaleString()}</td>
                        <td class="py-3 px-4 text-center font-mono text-emerald-400 font-semibold">${c.human_visitors.toLocaleString()}</td>
                        <td class="py-3 px-4 text-center font-mono text-orange-400 font-bold">${c.leads.toLocaleString()}</td>
                        <td class="py-3 px-4 text-center">
                            <span class="px-2 py-0.5 rounded-full text-xs font-mono font-bold border ${crBadge}">${c.conversion_rate}%</span>
                        </td>
                        <td class="py-3 px-4 text-center">${evalBadge}</td>
                    </tr>
                `;
            }).join('');
        }

        // 13. Landing Matrix & Sessions Loaders
        async function loadLandingPerformance() {
            try {
                const p = buildFilterParams();
                const res = await fetch('/api/analytics/landings?' + p.toString());
                if (!res.ok) return;
                const json = await res.json();
                const items = json.data || [];
                const tbody = document.getElementById('landingPerfBody');
                if (!tbody) return;

                if (items.length === 0) {
                    tbody.innerHTML = '<tr><td colspan="9" class="py-6 text-center text-slate-500">Chưa có dữ liệu</td></tr>';
                    return;
                }

                tbody.innerHTML = items.map(item => `
                    <tr class="hover:bg-slate-800/60 transition">
                        <td class="py-3 px-4 font-mono font-bold text-orange-400">${escapeHTML(item.landing_id)}</td>
                        <td class="py-3 px-4 text-center font-mono font-bold text-white">${item.total_views || 0}</td>
                        <td class="py-3 px-4 text-center font-mono text-emerald-400 font-semibold">${item.human_visitors || 0}</td>
                        <td class="py-3 px-4 text-center font-mono text-rose-400">${item.bot_visitors || 0}</td>
                        <td class="py-3 px-4 text-center font-mono text-purple-300">${item.form_interactions || 0}</td>
                        <td class="py-3 px-4 text-center font-mono font-bold text-orange-400 text-sm">${item.leads ?? item.converted_leads ?? 0}</td>
                        <td class="py-3 px-4 text-center font-bold text-emerald-400">${Number(item.conversion_rate || 0).toFixed(1)}%</td>
                        <td class="py-3 px-4 text-center font-mono text-slate-300">${Math.round(item.avg_duration_seconds || item.avg_duration_sec || 0)}s</td>
                        <td class="py-3 px-4 text-center font-mono text-slate-300">${Math.round(item.avg_scroll_percent || 0)}%</td>
                    </tr>
                `).join('');
            } catch (_) {}
        }

        async function loadSourcePerformance() {
            try {
                const p = buildFilterParams();
                const res = await fetch('/api/analytics/sources?' + p.toString());
                if (!res.ok) return;
                const json = await res.json();
                const items = json.data || [];
                const tbody = document.getElementById('sourcePerfBody');
                if (!tbody) return;

                if (items.length === 0) {
                    tbody.innerHTML = '<tr><td colspan="8" class="py-6 text-center text-slate-500">Chưa có dữ liệu</td></tr>';
                    return;
                }

                tbody.innerHTML = items.map(item => `
                    <tr class="hover:bg-slate-800/60 transition">
                        <td class="py-3 px-4 font-bold text-white">${escapeHTML(item.nguon || 'direct')}</td>
                        <td class="py-3 px-4 text-slate-300 font-mono">${escapeHTML(item.utm_campaign || '-')}</td>
                        <td class="py-3 px-4 text-slate-400 font-mono">${escapeHTML(item.utm_content || '-')}</td>
                        <td class="py-3 px-4 text-center font-mono text-white font-bold">${item.total_views || 0}</td>
                        <td class="py-3 px-4 text-center font-mono text-emerald-400 font-semibold">${item.human_visitors || 0}</td>
                        <td class="py-3 px-4 text-center font-mono text-rose-400">${item.bot_visitors ?? ((item.total_views || 0) - (item.human_visitors || 0))}</td>
                        <td class="py-3 px-4 text-center font-mono font-bold text-orange-400">${item.leads ?? item.converted_leads ?? 0}</td>
                        <td class="py-3 px-4 text-center font-bold text-emerald-400">${Number(item.conversion_rate || 0).toFixed(1)}%</td>
                    </tr>
                `).join('');
            } catch (_) {}
        }

        async function loadSessions() {
            try {
                const p = buildFilterParams();
                p.set('page', '1');
                p.set('limit', '25');

                const res = await fetch('/api/analytics/sessions?' + p.toString());
                if (!res.ok) return;
                const json = await res.json();
                const sessions = json.data || [];
                const tbody = document.getElementById('sessionsTableBody');
                if (!tbody) return;

                if (sessions.length === 0) {
                    tbody.innerHTML = '<tr><td colspan="11" class="py-6 text-center text-slate-500">Chưa có phiên</td></tr>';
                    return;
                }

                tbody.innerHTML = sessions.map(s => {
                    const isBot = s.is_bot;
                    const botBadge = isBot
                        ? `<span class="bg-rose-950 text-rose-400 border border-rose-800 text-[10px] px-2 py-0.5 rounded-full font-semibold"><i class="fa-solid fa-robot mr-1"></i>Bot (${s.bot_score}%)</span>`
                        : `<span class="bg-emerald-950 text-emerald-400 border border-emerald-800 text-[10px] px-2 py-0.5 rounded-full font-semibold"><i class="fa-solid fa-user-check mr-1"></i>Người Thật</span>`;

                    const leadBadge = s.converted_lead
                        ? `<span class="bg-orange-950 text-orange-400 border border-orange-800 text-[10px] px-2 py-0.5 rounded font-bold font-mono">${escapeHTML(s.so_dien_thoai || 'Đã đăng ký')}</span>`
                        : `<span class="text-slate-600 text-[10px] font-mono">Chưa gửi form</span>`;

                    const cityText = escapeHTML(s.city || 'Chưa rõ');
                    const regionText = escapeHTML(s.region || 'Miền Bắc');
                    const ispText = escapeHTML(s.isp || 'Khác');
                    const platformText = escapeHTML(s.ad_platform || 'Trực Tiếp');
                    const placementText = escapeHTML(s.ad_placement || 'Direct');

                    return `
                        <tr class="hover:bg-slate-800/60 transition text-xs">
                            <td class="py-3 px-4 font-mono text-slate-300 text-[11px] whitespace-nowrap">${escapeHTML(s.first_seen.slice(5, 19))}</td>
                            <td class="py-3 px-4 font-mono text-slate-300">
                                <div>${escapeHTML(s.session_id.slice(0, 16))}...</div>
                                <div class="text-[10px] text-slate-500">${escapeHTML(s.ip_address)}</div>
                            </td>
                            <td class="py-3 px-4">
                                <div class="flex items-center gap-1.5 flex-wrap">
                                    <span class="font-bold text-slate-200">${cityText}</span>
                                    <span class="text-[9px] px-1.5 py-0.5 rounded font-bold ${getRegionBadgeClass(s.region)}">${regionText}</span>
                                </div>
                                <div class="text-[10px] text-slate-500 font-mono mt-0.5"><i class="fa-solid fa-tower-cell mr-1"></i>${ispText}</div>
                            </td>
                            <td class="py-3 px-4">
                                <div class="flex items-center gap-1">
                                    <span class="text-[10px] px-1.5 py-0.5 rounded font-bold ${getAdPlatformBadgeClass(s.ad_platform)}">${platformText}</span>
                                </div>
                                <div class="mt-0.5">
                                    <span class="text-[10px] px-1.5 py-0.5 rounded font-semibold ${getAdPlacementBadgeClass(s.ad_platform, s.ad_placement)}">${placementText}</span>
                                </div>
                            </td>
                            <td class="py-3 px-4">${botBadge}</td>
                            <td class="py-3 px-4 text-slate-300">${escapeHTML(s.device_type)} • ${escapeHTML(s.browser)}</td>
                            <td class="py-3 px-4 font-mono text-orange-400">${escapeHTML(s.landing_id)} <span class="text-slate-500 text-[10px]">(${escapeHTML(s.nguon)})</span></td>
                            <td class="py-3 px-4 text-center font-mono text-slate-300 font-semibold">${s.duration_seconds}s</td>
                            <td class="py-3 px-4 text-center font-mono text-slate-300">${s.max_scroll_percent}%</td>
                            <td class="py-3 px-4 text-center">${leadBadge}</td>
                            <td class="py-3 px-4 text-center">
                                <button onclick="viewSessionTimeline('${s.session_id}')" class="text-emerald-400 hover:text-emerald-300 bg-emerald-950/40 border border-emerald-800/60 px-2 py-1 rounded text-[11px] font-semibold transition">
                                    <i class="fa-solid fa-shoe-prints mr-1"></i>Timeline
                                </button>
                            </td>
                        </tr>
                    `;
                }).join('');
            } catch (_) {}
        }

        async function viewSessionTimeline(sessionId) {
            try {
                const res = await fetch('/api/analytics/session/' + encodeURIComponent(sessionId));
                if (!res.ok) return;
                const json = await res.json();
                const sess = json.data;
                if (!sess) return;

                document.getElementById('modalSessionId').innerText = sess.session_id;
                document.getElementById('modalSessionSub').innerText = `${sess.ip_address} • ${sess.city || 'Chưa rõ'} (${sess.region || 'Miền Bắc'}) • ${sess.isp || 'Mạng khác'} • ${sess.ad_placement || sess.ad_platform || 'Trực tiếp'} • ${sess.device_type} • ${sess.browser} (${sess.os}) • ${sess.duration_seconds}s trên web`;

                let actions = [];
                try {
                    actions = typeof sess.actions === 'string' ? JSON.parse(sess.actions) : (sess.actions || []);
                } catch (_) {}

                const container = document.getElementById('sessionTimelineContainer');
                if (actions.length === 0) {
                    container.innerHTML = '<div class="text-slate-500 text-center py-6">Không có hành động nào được ghi lại trong session này.</div>';
                } else {
                    container.innerHTML = actions.map((act, idx) => `
                        <div class="flex items-start space-x-3 p-2.5 rounded-xl bg-slate-950 border border-slate-800">
                            <span class="w-6 h-6 rounded-full bg-slate-800 text-slate-400 flex items-center justify-center text-[10px] font-bold flex-shrink-0">${idx + 1}</span>
                            <div class="flex-1">
                                <div class="flex items-center justify-between text-slate-200">
                                    <span class="font-bold text-orange-400">${escapeHTML(act.event || act.action || 'event')}</span>
                                    <span class="text-[10px] text-slate-500 font-mono">${escapeHTML(act.timestamp ? new Date(act.timestamp).toLocaleTimeString() : '')}</span>
                                </div>
                                <div class="text-slate-300 text-[11px] mt-0.5">${escapeHTML(act.details || act.label || '')}</div>
                            </div>
                        </div>
                    `).join('');
                }

                document.getElementById('sessionModal').classList.remove('hidden');
            } catch (err) {
                alert('Lỗi tải timeline: ' + err.message);
            }
        }

        function closeSessionModal() {
            document.getElementById('sessionModal').classList.add('hidden');
        }

// ==============================================================
// 14. BÁO CÁO RIÊNG TỪNG LANDING PAGE (2026-11-12)
//     Mỗi landing có bộ KPI + biểu đồ + báo cáo riêng biệt.
// ==============================================================

        let chartReportTimeseries = null;
        let chartReportDevices = null;
        let chartReportInApp = null;
        let chartReportBrowser = null;
        let chartReportBots = null;
        let chartReportVPN = null;

        // Điểm dữ liệu cache của báo cáo hiện tại (dùng để đổi granularity
        // mà không phải gọi API lại).
        let landingReportCache = null;
        let reportGranularity = 'day';

        // reportGranularityLabel đổi nhãn tiếng Việt cho đúng ngữ cảnh.
        const reportGranularityLabel = {
            hour: 'Theo Giờ',
            day: 'Theo Ngày',
            week: 'Theo Tuần'
        };

        // onReportLandingChange được gọi khi người dùng đổi landing trong
        // báo cáo riêng. Đồng thời đồng bộ bộ lọc toàn cục để 2 phần khớp nhau.
        function onReportLandingChange() {
            const sel = document.getElementById('reportLandingSelect');
            const globalSel = document.getElementById('filterLanding');
            if (sel && globalSel) {
                globalSel.value = sel.value;
            }
            loadLandingReport(true);
        }

        function setReportGranularity(g) {
            reportGranularity = g;
            document.querySelectorAll('.lr-granularity-btn').forEach(btn => {
                const active = btn.dataset.lrGranularity === g;
                btn.classList.toggle('bg-orange-600', active);
                btn.classList.toggle('text-white', active);
                btn.classList.toggle('text-slate-400', !active);
            });
            loadLandingReport(true);
        }

        // buildReportParams dựng query string cho báo cáo landing riêng.
        // Dùng bộ lọc ngày + khung giờ toàn cục để khớp với phần CRM/KPI.
        function buildReportParams() {
            const p = new URLSearchParams();
            const sel = document.getElementById('reportLandingSelect');
            const landing = sel ? sel.value.trim() : '';
            const nguonEl = document.getElementById('filterNguon');
            const nguon = nguonEl ? nguonEl.value.trim() : '';
            const fromDate = document.getElementById('filterFromDate').value.trim();
            const toDate = document.getElementById('filterToDate').value.trim();
            const hourRange = document.getElementById('filterHourRange').value;

            if (landing) p.set('landing_id', landing);
            if (nguon) p.set('nguon', nguon);
            if (fromDate) p.set('from_date', fromDate);
            if (toDate) p.set('to_date', toDate);

            if (hourRange === 'golden') {
                p.set('from_hour', '19');
                p.set('to_hour', '22');
            } else if (hourRange === 'morning') {
                p.set('from_hour', '8');
                p.set('to_hour', '12');
            } else if (hourRange === 'afternoon') {
                p.set('from_hour', '13');
                p.set('to_hour', '17');
            } else if (hourRange === 'night') {
                // Khoảng vòng qua nửa đêm (22h → 06h). Backend đã hỗ trợ
                // from_hour > to_hour ⇒ dùng đúng khoảng này.
                p.set('from_hour', '22');
                p.set('to_hour', '6');
            }

            p.set('days', '30');
            return p;
        }

        // populateReportLandingOptions đổ danh sách landing có trong DB vào
        // dropdown báo cáo riêng (không ghi đè lựa chọn hiện tại).
        function populateReportLandingOptions(landingIDs) {
            const sel = document.getElementById('reportLandingSelect');
            if (!sel) return;
            const current = sel.value;
            if (sel.options.length <= 1 && Array.isArray(landingIDs)) {
                landingIDs.forEach(lid => {
                    const opt = document.createElement('option');
                    opt.value = lid;
                    opt.innerText = lid;
                    sel.appendChild(opt);
                });
                sel.value = current;
            }
        }

        async function loadLandingReport(force) {
            const sel = document.getElementById('reportLandingSelect');
            const landingID = sel ? sel.value.trim() : '';

            try {
                const p = buildReportParams();
                const res = await fetch('/api/analytics/landing-report?' + p.toString());
                if (!res.ok) return;
                const json = await res.json();
                if (!json.success) return;
                const d = json.data || {};
                landingReportCache = d;

                renderReportKPI(d.overview || {}, d.vpn || {}, d.bots || {});
                renderReportTimeseries(d.timeseries || {});
                renderReportDevices(d.devices || []);
                renderReportInApp(d.in_app || []);
                renderReportBrowser(d.browsers || []);
                renderReportBots(d.bots || {});
                renderReportVPN(d.vpn || {});
                renderReportMacroTable(d.meta_macros || []);
                renderReportDeviceTable(d.devices || []);
            } catch (err) {
                console.error('Lỗi tải báo cáo landing:', err);
            }
        }

        function renderReportKPI(ov, vpn, bots) {
            const set = (id, val) => {
                const el = document.getElementById(id);
                if (el) el.innerText = val;
            };
            set('lrTotalViews', Number(ov.total_views || 0).toLocaleString('vi-VN'));
            set('lrUniqueVisitors', `${Number(ov.total_visitors || 0).toLocaleString('vi-VN')} khách unique`);
            set('lrHumanVisitors', Number(ov.human_visitors || 0).toLocaleString('vi-VN'));
            set('lrBotVisitors', `${Number(ov.bot_visitors || 0).toLocaleString('vi-VN')} bot (${Number(ov.bot_rate_percent || 0).toFixed(1)}%)`);
            set('lrTotalLeads', Number(ov.total_leads || 0).toLocaleString('vi-VN'));
            set('lrConversionRate', `CR ${Number(ov.conversion_rate || 0).toFixed(2)}%`);
            set('lrAvgDuration', `${Math.round(Number(ov.avg_duration_seconds || 0))}s`);
            set('lrAvgScroll', `scroll ${Math.round(Number(ov.avg_max_scroll_percent || 0))}%`);
            set('lrVpnVisitors', Number(ov.vpn_visitors || 0).toLocaleString('vi-VN'));
            set('lrVpnRate', `${Number(ov.vpn_rate_percent || 0).toFixed(1)}% lượt · ${Number(vpn.proxy_sessions || 0)} proxy · ${Number(vpn.tor_sessions || 0)} tor`);
            set('lrInAppVisitors', Number(ov.in_app_visitors || 0).toLocaleString('vi-VN'));
            set('lrMetaClicks', `${Number(ov.meta_click_count || 0).toLocaleString('vi-VN')} click Meta`);
        }

        function renderReportTimeseries(ts) {
            const ctx = document.getElementById('chartReportTimeseries');
            if (!ctx) return;

            const rows = ts[reportGranularity] || [];
            const labels = rows.map(r => r.label);
            const titleEl = document.getElementById('lrTimeseriesTitle');
            const subEl = document.getElementById('lrTimeseriesSub');
            if (titleEl) titleEl.innerText = `Xu Hướng Lượt Xem / Người Thật / Bot / Lead ${reportGranularityLabel[reportGranularity] || ''}`;
            if (subEl) subEl.innerText = rows.length ? `${rows.length} mốc thời gian · tổng ${rows.reduce((a, r) => a + (r.views || 0), 0)} lượt xem` : 'Không có dữ liệu trong khoảng đã chọn';

            if (chartReportTimeseries) chartReportTimeseries.destroy();

            chartReportTimeseries = new Chart(ctx, {
                type: 'line',
                data: {
                    labels: labels,
                    datasets: [
                        {
                            label: 'Lượt xem',
                            data: rows.map(r => r.views || 0),
                            borderColor: '#f97316',
                            backgroundColor: 'rgba(249, 115, 22, 0.12)',
                            fill: true,
                            tension: 0.35,
                            borderWidth: 2
                        },
                        {
                            label: 'Người thật',
                            data: rows.map(r => r.human_visitors || 0),
                            borderColor: '#34d399',
                            backgroundColor: 'rgba(52, 211, 153, 0.08)',
                            fill: false,
                            tension: 0.35,
                            borderWidth: 2
                        },
                        {
                            label: 'Bot',
                            data: rows.map(r => r.bot_visitors || 0),
                            borderColor: '#f43f5e',
                            backgroundColor: 'rgba(244, 63, 94, 0.08)',
                            fill: false,
                            tension: 0.35,
                            borderWidth: 2,
                            borderDash: [4, 3]
                        },
                        {
                            label: 'Lead',
                            data: rows.map(r => r.leads || 0),
                            borderColor: '#38bdf8',
                            backgroundColor: 'rgba(56, 189, 248, 0.08)',
                            fill: false,
                            tension: 0.35,
                            borderWidth: 2,
                            pointRadius: 3
                        }
                    ]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    interaction: { mode: 'index', intersect: false },
                    plugins: {
                        legend: { labels: { color: '#cbd5e1', boxWidth: 12, font: { size: 10 } } },
                        tooltip: {
                            callbacks: {
                                afterBody: (items) => {
                                    const r = rows[items[0].dataIndex];
                                    return `CR: ${Number(r?.conversion_rate || 0).toFixed(2)}% · Thời gian TB: ${Math.round(r?.avg_duration_seconds || 0)}s`;
                                }
                            }
                        }
                    },
                    scales: {
                        x: { grid: { color: 'rgba(51, 65, 85, 0.25)' }, ticks: { color: '#94a3b8', font: { size: 9 }, maxRotation: 0, autoSkipPadding: 12 } },
                        y: { beginAtZero: true, grid: { color: 'rgba(51, 65, 85, 0.25)' }, ticks: { color: '#94a3b8', precision: 0, font: { size: 9 } } }
                    }
                }
            });
        }

        function renderReportDevices(devices) {
            const ctx = document.getElementById('chartReportDevices');
            if (!ctx) return;

            const top = (devices || []).slice(0, 10);
            const labels = top.map(d => (d.name || 'Khác').length > 22 ? (d.name || '').slice(0, 20) + '…' : (d.name || 'Khác'));
            const views = top.map(d => d.total_views || 0);
            const leads = top.map(d => d.leads || 0);

            if (chartReportDevices) chartReportDevices.destroy();

            chartReportDevices = new Chart(ctx, {
                type: 'bar',
                data: {
                    labels: labels,
                    datasets: [
                        { label: 'Lượt xem', data: views, backgroundColor: 'rgba(34, 211, 238, 0.65)', borderColor: '#22d3ee', borderWidth: 1, borderRadius: 4 },
                        { label: 'Lead', data: leads, backgroundColor: 'rgba(249, 115, 22, 0.8)', borderColor: '#fb923c', borderWidth: 1, borderRadius: 4 }
                    ]
                },
                options: {
                    indexAxis: 'y',
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: {
                        legend: { labels: { color: '#cbd5e1', boxWidth: 12, font: { size: 10 } } },
                        tooltip: {
                            callbacks: {
                                afterBody: (items) => {
                                    const d = top[items[0].dataIndex];
                                    return `Người thật: ${d.human_visitors || 0} · CR: ${Number(d.conversion_rate || 0).toFixed(2)}%`;
                                }
                            }
                        }
                    },
                    scales: {
                        x: { beginAtZero: true, grid: { color: 'rgba(51, 65, 85, 0.25)' }, ticks: { color: '#94a3b8', precision: 0, font: { size: 9 } } },
                        y: { grid: { display: false }, ticks: { color: '#cbd5e1', font: { size: 9 } } }
                    }
                }
            });
        }

        function renderReportInApp(items) {
            const ctx = document.getElementById('chartReportInApp');
            if (!ctx) return;

            const labels = (items || []).map(i => i.name);
            const views = (items || []).map(i => i.total_views || 0);
            const leads = (items || []).map(i => i.leads || 0);

            if (chartReportInApp) chartReportInApp.destroy();

            chartReportInApp = new Chart(ctx, {
                type: 'doughnut',
                data: {
                    labels: labels,
                    datasets: [
                        { label: 'Lượt xem', data: views, backgroundColor: ['#a78bfa', '#38bdf8', '#f472b6', '#fbbf24', '#34d399'], borderColor: '#0f172a', borderWidth: 2 },
                        { label: 'Lead', data: leads, backgroundColor: ['rgba(167, 139, 250, 0.45)', 'rgba(56, 189, 248, 0.45)', 'rgba(244, 114, 182, 0.45)', 'rgba(251, 191, 36, 0.45)', 'rgba(52, 211, 153, 0.45)'], borderColor: '#0f172a', borderWidth: 2 }
                    ]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: {
                        legend: { labels: { color: '#cbd5e1', boxWidth: 12, font: { size: 10 } } },
                        tooltip: { callbacks: { label: (i) => `${i.dataset.label}: ${i.formattedValue}` } }
                    }
                }
            });
        }

        function renderReportBrowser(items) {
            const ctx = document.getElementById('chartReportBrowser');
            if (!ctx) return;

            const top = (items || []).slice(0, 8);
            const labels = top.map(i => i.name);
            const views = top.map(i => i.total_views || 0);

            if (chartReportBrowser) chartReportBrowser.destroy();

            chartReportBrowser = new Chart(ctx, {
                type: 'bar',
                data: {
                    labels: labels,
                    datasets: [{ label: 'Lượt xem', data: views, backgroundColor: 'rgba(251, 146, 60, 0.7)', borderColor: '#fb923c', borderWidth: 1, borderRadius: 4 }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: { legend: { display: false } },
                    scales: {
                        x: { grid: { display: false }, ticks: { color: '#cbd5e1', font: { size: 9 }, maxRotation: 45 } },
                        y: { beginAtZero: true, grid: { color: 'rgba(51, 65, 85, 0.25)' }, ticks: { color: '#94a3b8', precision: 0, font: { size: 9 } } }
                    }
                }
            });
        }

        function renderReportBots(botStats) {
            const ctx = document.getElementById('chartReportBots');
            if (!ctx) return;

            const reasons = (botStats && botStats.reasons) || [];
            const labels = reasons.map(r => r.name);
            const data = reasons.map(r => r.count);

            if (chartReportBots) chartReportBots.destroy();

            chartReportBots = new Chart(ctx, {
                type: 'bar',
                data: {
                    labels: labels,
                    datasets: [{ label: 'Số phiên bot', data: data, backgroundColor: 'rgba(244, 63, 94, 0.7)', borderColor: '#f43f5e', borderWidth: 1, borderRadius: 4 }]
                },
                options: {
                    indexAxis: 'y',
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: { legend: { display: false } },
                    scales: {
                        x: { beginAtZero: true, grid: { color: 'rgba(51, 65, 85, 0.25)' }, ticks: { color: '#94a3b8', precision: 0, font: { size: 9 } } },
                        y: { grid: { display: false }, ticks: { color: '#cbd5e1', font: { size: 9 } } }
                    }
                }
            });
        }

        function renderReportVPN(vpnStats) {
            const ctx = document.getElementById('chartReportVPN');
            if (!ctx) return;

            const providers = (vpnStats && vpnStats.providers) || [];
            const labels = providers.map(p => p.name);
            const data = providers.map(p => p.count);

            if (chartReportVPN) chartReportVPN.destroy();

            chartReportVPN = new Chart(ctx, {
                type: 'bar',
                data: {
                    labels: labels,
                    datasets: [{ label: 'Số phiên', data: data, backgroundColor: 'rgba(56, 189, 248, 0.7)', borderColor: '#38bdf8', borderWidth: 1, borderRadius: 4 }]
                },
                options: {
                    indexAxis: 'y',
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: { legend: { display: false } },
                    scales: {
                        x: { beginAtZero: true, grid: { color: 'rgba(51, 65, 85, 0.25)' }, ticks: { color: '#94a3b8', precision: 0, font: { size: 9 } } },
                        y: { grid: { display: false }, ticks: { color: '#cbd5e1', font: { size: 9 } } }
                    }
                }
            });
        }

        function renderReportMacroTable(macros) {
            const tbody = document.getElementById('lrMacroTableBody');
            if (!tbody) return;

            if (!macros || macros.length === 0) {
                tbody.innerHTML = '<tr><td colspan="9" class="py-6 text-center text-slate-500">Chưa có dữ liệu Meta Ads cho landing này</td></tr>';
                return;
            }

            tbody.innerHTML = macros.map(m => {
                const cr = Number(m.conversion_rate || 0);
                const crBadge = cr >= 10 ? 'bg-emerald-950 text-emerald-400 border-emerald-800'
                    : cr >= 3 ? 'bg-amber-950 text-amber-400 border-amber-800'
                        : 'bg-slate-800 text-slate-400 border-slate-700';
                return `
                    <tr class="hover:bg-slate-800/60 transition text-xs">
                        <td class="py-3 px-4">
                            <div class="text-slate-200 font-semibold">${escapeHTML(m.campaign_name || '(không rõ)')}</div>
                            <div class="text-[10px] text-slate-500 font-mono">ID: ${escapeHTML(m.campaign_id || '-')}</div>
                        </td>
                        <td class="py-3 px-4">
                            <div class="text-slate-300">${escapeHTML(m.adset_name || '(không rõ)')}</div>
                            <div class="text-[10px] text-slate-500 font-mono">ID: ${escapeHTML(m.adset_id || '-')}</div>
                        </td>
                        <td class="py-3 px-4">
                            <div class="text-slate-300">${escapeHTML(m.ad_name || '(không rõ)')}</div>
                            <div class="text-[10px] text-slate-500 font-mono">ID: ${escapeHTML(m.ad_id || '-')}</div>
                        </td>
                        <td class="py-3 px-4 text-[11px] text-slate-300 font-mono">${escapeHTML(m.placement || '-')}</td>
                        <td class="py-3 px-4 text-center font-mono text-slate-300">${m.total_views || 0}</td>
                        <td class="py-3 px-4 text-center font-mono text-emerald-400">${m.human_visitors || 0}</td>
                        <td class="py-3 px-4 text-center font-mono font-bold text-orange-400">${m.leads || 0}</td>
                        <td class="py-3 px-4 text-center"><span class="px-2 py-0.5 rounded-full text-[10px] font-mono font-bold border ${crBadge}">${cr.toFixed(2)}%</span></td>
                        <td class="py-3 px-4 text-center font-mono text-blue-300">${m.meta_clicks || 0}</td>
                    </tr>
                `;
            }).join('');
        }

        function renderReportDeviceTable(devices) {
            const tbody = document.getElementById('lrDeviceTableBody');
            if (!tbody) return;

            if (!devices || devices.length === 0) {
                tbody.innerHTML = '<tr><td colspan="9" class="py-6 text-center text-slate-500">Chưa có dữ liệu thiết bị</td></tr>';
                return;
            }

            tbody.innerHTML = devices.map(d => {
                const cr = Number(d.conversion_rate || 0);
                const crBadge = cr >= 10 ? 'bg-emerald-950 text-emerald-400 border-emerald-800'
                    : cr >= 3 ? 'bg-amber-950 text-amber-400 border-amber-800'
                        : 'bg-slate-800 text-slate-400 border-slate-700';
                return `
                    <tr class="hover:bg-slate-800/60 transition text-xs">
                        <td class="py-3 px-4 text-slate-200 font-semibold">${escapeHTML(d.name || 'Khác')}</td>
                        <td class="py-3 px-4 text-slate-400">${escapeHTML(d.brand || '-')}</td>
                        <td class="py-3 px-4 font-mono text-slate-500 text-[11px]">${escapeHTML(d.model || '-')}</td>
                        <td class="py-3 px-4 text-center font-mono text-slate-300">${d.total_views || 0}</td>
                        <td class="py-3 px-4 text-center font-mono text-emerald-400">${d.human_visitors || 0}</td>
                        <td class="py-3 px-4 text-center font-mono font-bold text-orange-400">${d.leads || 0}</td>
                        <td class="py-3 px-4 text-center"><span class="px-2 py-0.5 rounded-full text-[10px] font-mono font-bold border ${crBadge}">${cr.toFixed(2)}%</span></td>
                        <td class="py-3 px-4 text-center font-mono text-slate-300">${Math.round(d.avg_duration_seconds || 0)}s</td>
                        <td class="py-3 px-4 text-center font-mono text-slate-300">${Math.round(d.avg_scroll_percent || 0)}%</td>
                    </tr>
                `;
            }).join('');
        }


