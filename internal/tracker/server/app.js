'use strict';

// First ensure plugin is registered
const annotationPlugin = window['chartjs-plugin-annotation'];
if (!annotationPlugin) {
    console.error('Annotation plugin not loaded!');
}
Chart.register(annotationPlugin);

// Test that annotation works
console.log('Annotation plugin registered:', Chart.registry.plugins.get('annotation'));

// Global state
let currentTimeRange = {
    start: new Date(Date.now() - 24 * 60 * 60 * 1000),
    end: new Date()
};
let currentPreset = "Last 24h";
let levels = [];
let timeEdited = false;

// Preset definitions
const presets = {
    "Last 24h": () => ({
        start: new Date(Date.now() - 24 * 60 * 60 * 1000),
        end: new Date()
    }),
    "Last 7d": () => ({
        start: new Date(Date.now() - 7 * 24 * 60 * 60 * 1000),
        end: new Date()
    }),
    "Last 30d": () => ({
        start: new Date(Date.now() - 30 * 24 * 60 * 60 * 1000),
        end: new Date()
    })
};

// Helper Functions
function toRFC3339(date) {
    return date.toISOString();
}

function formatTimestamp(timestamp, rangeStart, rangeEnd, isEvent = false) {
    const date = isEvent ? new Date(timestamp) : new Date(timestamp * 1000);
    const rangeDays = (rangeEnd - rangeStart) / (1000 * 60 * 60 * 24);

    if (rangeDays <= 1) {
        return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    } else if (rangeDays <= 7) {
        return date.toLocaleString([], {
            weekday: 'short',
            hour: '2-digit',
            minute: '2-digit'
        });
    } else {
        return date.toLocaleString([], {
            month: 'short',
            day: 'numeric',
            hour: '2-digit',
            minute: '2-digit'
        });
    }
}

function formatCurrency(cents) {
    return `$${(cents / 100).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
}

function findMatchingPreset(start, end) {
    return Object.entries(presets).find(([label, getRange]) => {
        const preset = getRange();
        return Math.abs(preset.start - start) < 1000 && Math.abs(preset.end - end) < 1000;
    })?.[0];
}

function updateDatesAndFetch(start, end) {
    currentTimeRange = { start, end };
    timeEdited = true;
    updateDashboard();
}

function getChartTimestamp(event, chart) {
    const rect = chart.canvas.getBoundingClientRect();
    const x = event.clientX - rect.left;
    const xScale = chart.scales.x;
    const index = Math.round((x / xScale.width) * (levels.length - 1));

    if (index >= 0 && index < levels.length) {
        return levels[index].timestamp;
    }
    return null;
}

// Initialize Chart
const levelChart = new Chart(document.getElementById('levelChart'), {
    type: 'line',
    data: {
        labels: [],
        datasets: [{
            label: 'Caffeine Level',
            data: [],
            borderColor: '#4ade80',
            tension: 0.4,
            pointRadius: 0
        }]
    },
    options: {
        responsive: true,
        maintainAspectRatio: false,  // This is crucial for fixed height
        height: 300, // Set a fixed height
        scales: {
            y: {
                beginAtZero: true,
                max: 2000,
                grid: { color: '#333' },
                ticks: { color: '#fff' }
            },
            x: {
                grid: { color: '#333' },
                ticks: {
                    color: '#fff',
                    maxRotation: 45,
                    minRotation: 45
                }
            }
        },
        plugins: {
            legend: { display: false },
            tooltip: {
                enabled: true,
                mode: 'index',
                intersect: false
            },
            annotation: {
                common: {
                    drawTime: 'afterDraw'
                },
                annotations: {
                    crosshair: {
                        type: 'line',
                        xMin: 0,
                        xMax: 0,
                        borderColor: 'rgba(255, 255, 255, 0.5)',
                        borderWidth: 1,
                        borderDash: [5, 5],
                        display: false
                    },
                    redBox: {
                        type: 'box',
                        yMin: 1800,
                        yMax: 2000,
                        backgroundColor: 'rgba(255, 0, 0, 0.1)',
                        borderWidth: 0
                    },
                    greenBox: {
                        type: 'box',
                        yMin: 350,
                        yMax: 450,
                        backgroundColor: 'rgba(0, 255, 0, 0.1)',
                        borderWidth: 0
                    }
                },
            }
        },
    }
});

// Initialize date picker
const dateRangePicker = flatpickr("#dateRange", {
    mode: "range",
    enableTime: true,
    dateFormat: "M j, Y h:i K",
    defaultHour: 0,
    theme: "dark",
    defaultDate: [currentTimeRange.start, currentTimeRange.end],
    onChange: function(selectedDates) {
        if (selectedDates.length === 2) {
            updateDatesAndFetch(selectedDates[0], selectedDates[1]);
            const preset = findMatchingPreset(selectedDates[0], selectedDates[1]);
            if (preset) {
                setTimeout(() => {
                    this.input.value = preset;
                }, 0);
            }
        }
    },
    onOpen: function() {
        this._currentValue = this.input.value;
    },
    onClose: function(selectedDates) {
        if (selectedDates.length === 2) {
            const preset = findMatchingPreset(selectedDates[0], selectedDates[1]);
            if (preset) {
                this.input.value = preset;
            } else if (this._currentValue && this._currentValue.includes('Last')) {
                this.input.value = this._currentValue;
            }
        }
    },
    onReady: function(selectedDates, dateStr, instance) {
        const presetContainer = document.createElement('div');
        presetContainer.className = 'flatpickr-presets flex flex-wrap p-2 border-b border-zinc-700';

        Object.entries(presets).forEach(([label, getRange]) => {
            const button = document.createElement('button');
            button.className = 'bg-zinc-800 hover:bg-zinc-700 text-white px-3 py-1 rounded m-1 text-sm';
            button.textContent = label;
            button.addEventListener('click', () => {
                const range = getRange();
                instance.setDate([range.start, range.end]);
                currentPreset = label;
                instance.input.value = label;
                updateDatesAndFetch(range.start, range.end);
            });
            presetContainer.appendChild(button);
        });

        instance.calendarContainer.insertBefore(
            presetContainer,
            instance.calendarContainer.firstChild
        );

        if (selectedDates.length === 2) {
            const preset = findMatchingPreset(selectedDates[0], selectedDates[1]);
            if (preset) {
                instance.input.value = preset;
            }
        }
    }
});

// Update dashboard data
function updateDashboard() {
    const { start, end } = currentTimeRange;
    const queryParams = `?start=${toRFC3339(start)}&end=${toRFC3339(end)}`;
    const summaryQueryParams = timeEdited ? queryParams : '';

    Promise.all([
        fetch('/api/levels' + queryParams),
        fetch('/api/events' + queryParams),
        fetch('/api/events/summary' + summaryQueryParams)
    ])
        .then(responses => Promise.all(responses.map(r => r.json())))
        .then(([newLevels, events, summary]) => {
            // Update global levels data
            levels = newLevels.filter(l => {
                const timestamp = l.timestamp * 1000; // Convert to milliseconds
                return timestamp >= start.getTime() && timestamp <= end.getTime();
            });

            // Update chart
            levelChart.data.labels = levels.map(l => formatTimestamp(l.timestamp, start, end));
            levelChart.data.datasets[0].data = levels.map(l => Math.round(l.level));
            levelChart.update('none'); // Use 'none' for smoother updates

            // Update current level
            const currentLevel = levels.length > 0 ? levels[levels.length - 1].level : 0;
            document.getElementById('currentLevel').textContent = `${Math.round(currentLevel)} mg`;

            // Update lifetime stats
            document.getElementById('lifetimeIntake').textContent = `${summary.intake.toLocaleString()} mg`;
            document.getElementById('lifetimeCost').textContent = formatCurrency(summary.cost);

            // Update events list
            document.getElementById('eventsList').innerHTML = events.map(event => `
            <div>
                $${(event.cost / 100).toFixed(2)} on ${event.description} at 
                ${formatTimestamp(event.timestamp, start, end, true)} for ${event.amount}mg
            </div>
        `).join('');

            // Ensure preset display persists
            if (currentPreset && dateRangePicker && dateRangePicker.input) {
                dateRangePicker.input.value = currentPreset;
            }
        })
        .catch(error => console.error('Error fetching data:', error));
}

// Initial update
updateDashboard();