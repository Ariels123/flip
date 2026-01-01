# FLIP2 Monitoring Dashboard Design

**Version:** 1.0
**Date:** 2025-12-31
**Status:** Design Complete

## Overview

Real-time operational dashboard for FLIP2 multi-agent system providing visibility into system health, agent activity, signal flow, and costs.

---

## Architecture

### Technology Stack

**Frontend:**
- **Alpine.js 3.x** - Lightweight reactive framework (15KB)
- **TailwindCSS 3.x (CDN)** - Utility-first CSS for rapid development
- **Chart.js 4.x** - Simple, beautiful charts
- **PocketBase JS SDK** - Real-time data sync

**Backend:**
- **PocketBase** - Embedded database with real-time subscriptions
- **Existing FLIP2 collections** - signals, agents, tasks, costs

**Deployment:**
- Served from `pb_public/` directory by PocketBase daemon
- No build step required (all CDN dependencies)
- Works on mobile, tablet, desktop

### Why This Stack?

| Choice | Reason |
|--------|--------|
| Alpine.js | No build step, reactive, tiny, similar to Vue |
| TailwindCSS CDN | Rapid prototyping, responsive by default, no compilation |
| Chart.js | Simple API, good defaults, widely used |
| PocketBase SDK | Built-in real-time, automatic reconnection, auth |

---

## Visual Design

### Layout Structure

```
┌─────────────────────────────────────────────────────────────┐
│  FLIP2 Dashboard              🟢 Live    Last: 2s ago     │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌───────────┐ │
│  │  Agents   │ │  Signals  │ │  Tasks    │ │  Costs    │ │
│  │    12     │ │   24/min  │ │    8      │ │  $2.43    │ │
│  │  online   │ │ avg       │ │ active    │ │  today    │ │
│  └───────────┘ └───────────┘ └───────────┘ └───────────┘ │
│                                                             │
│  ┌─────────────────────┐  ┌────────────────────────────┐  │
│  │  Agent Activity     │  │  Signal Throughput (1h)    │  │
│  │                     │  │  [Line Chart]              │  │
│  │  • claude-mac  ✓    │  │                            │  │
│  │  • claude-win  ✓    │  │  40 signals/min            │  │
│  │  • gemini      ✗    │  │                            │  │
│  │  • ag-win      ✓    │  │                            │  │
│  └─────────────────────┘  └────────────────────────────┘  │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Recent Signals (last 10)                            │  │
│  ├──────────────────────────────────────────────────────┤  │
│  │  claude-mac → claude-win  "Task complete"   2s ago   │  │
│  │  ag-win → coordinator     "Need help"       5s ago   │  │
│  │  claude-win → claude-mac  "Acknowledged"   12s ago   │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                             │
│  ┌─────────────────────┐  ┌────────────────────────────┐  │
│  │  Cost Breakdown     │  │  System Health             │  │
│  │  [Pie Chart]        │  │                            │  │
│  │                     │  │  DB Size: 24MB / 100MB     │  │
│  │  Claude  $1.80      │  │  Memory: 180MB / 512MB     │  │
│  │  Gemini  $0.63      │  │  Uptime: 3d 4h 12m         │  │
│  └─────────────────────┘  └────────────────────────────┘  │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Color Palette

**Background:**
- Primary: `#0f172a` (slate-900) - Dark, professional
- Secondary: `#1e293b` (slate-800) - Card backgrounds
- Tertiary: `#334155` (slate-700) - Borders

**Text:**
- Primary: `#f1f5f9` (slate-100) - Main text
- Secondary: `#94a3b8` (slate-400) - Secondary text
- Muted: `#64748b` (slate-500) - Labels

**Status:**
- Success: `#10b981` (emerald-500) - Online, success
- Warning: `#f59e0b` (amber-500) - Warnings
- Error: `#ef4444` (red-500) - Errors, offline
- Info: `#3b82f6` (blue-500) - Information

**Charts:**
- Primary: `#8b5cf6` (violet-500)
- Secondary: `#ec4899` (pink-500)
- Tertiary: `#06b6d4` (cyan-500)

### Typography

- **Headings:** Inter font (Google Fonts)
- **Body:** System UI stack for performance
- **Monospace:** JetBrains Mono for code/IDs

---

## Data Architecture

### Real-Time Subscriptions

```javascript
// Subscribe to signals collection (last 50)
pb.collection('signals').subscribe('*', (e) => {
  handleSignalUpdate(e.action, e.record)
})

// Subscribe to agents collection
pb.collection('agents').subscribe('*', (e) => {
  handleAgentUpdate(e.action, e.record)
})

// Subscribe to costs collection
pb.collection('costs').subscribe('*', (e) => {
  handleCostUpdate(e.action, e.record)
})
```

### Data Refresh Strategy

| Component | Strategy | Interval |
|-----------|----------|----------|
| Stats Cards | Real-time subscription | Immediate |
| Signal List | Real-time subscription | Immediate |
| Charts | Polling (aggregated data) | 5 seconds |
| System Health | Polling (daemon metrics) | 10 seconds |

### API Endpoints Needed

```
GET /api/stats/summary          - Overall system stats
GET /api/stats/throughput?hours=1  - Signal throughput timeseries
GET /api/stats/costs?days=1     - Cost breakdown by agent/model
GET /api/health                 - System health metrics
```

---

## Component Breakdown

### 1. Stats Cards (Top Row)

**Purpose:** At-a-glance system overview

**Data Sources:**
- Agents: `COUNT(agents WHERE status='online')`
- Signals: `COUNT(signals WHERE created > NOW() - 1min)`
- Tasks: `COUNT(tasks WHERE status='in_progress')`
- Costs: `SUM(costs WHERE timestamp > TODAY)`

**Update:** Real-time via subscriptions

### 2. Agent Activity Panel

**Purpose:** Monitor which agents are active/idle

**Display:**
- Agent name
- Status indicator (green=online, red=offline)
- Last seen timestamp
- Backend type badge

**Interaction:** Click to see agent details

### 3. Signal Throughput Chart

**Purpose:** Visualize signal flow over time

**Type:** Line chart (Chart.js)
**X-axis:** Time (5-minute buckets)
**Y-axis:** Signals per minute
**Data:** Aggregated from signals collection

### 4. Recent Signals Stream

**Purpose:** Live signal activity feed

**Display:**
- From → To agents
- Content preview (truncated to 50 chars)
- Time ago
- Priority badge

**Interaction:**
- Click to see full signal
- Auto-scroll on new signals
- Pause/resume stream

### 5. Cost Breakdown Chart

**Purpose:** Show cost distribution

**Type:** Doughnut chart (Chart.js)
**Segments:** By agent or by model (toggle)
**Data:** From costs collection

### 6. System Health Panel

**Purpose:** Monitor daemon and database health

**Metrics:**
- Database size / limit
- Memory usage / limit
- Uptime
- Error rate (last hour)

---

## Implementation Plan

### Phase 1: Foundation (1 hour)

**Files to create:**
```
pb_public/
├── index.html           # Main dashboard HTML
├── css/
│   └── dashboard.css    # Custom CSS (minimal)
└── js/
    └── dashboard.js     # Alpine.js components
```

**Tasks:**
1. Create HTML structure with TailwindCSS
2. Initialize PocketBase SDK connection
3. Implement authentication (use PocketBase admin auth)
4. Create Alpine.js store for shared state

### Phase 2: Core Components (2 hours)

**Tasks:**
1. Stats cards with real-time updates
2. Agent activity panel
3. Recent signals stream
4. Basic styling and layout

### Phase 3: Charts & Analytics (2 hours)

**Tasks:**
1. Signal throughput line chart
2. Cost breakdown pie chart
3. API endpoints for aggregated data
4. Polish and responsive design

### Phase 4: System Health (1 hour)

**Tasks:**
1. System health panel
2. Error tracking
3. Connection status indicator
4. Final testing on mobile

**Total:** ~6 hours

---

## Sample Code

### index.html Structure

```html
<!DOCTYPE html>
<html lang="en" class="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>FLIP2 Dashboard</title>

  <!-- TailwindCSS -->
  <script src="https://cdn.tailwindcss.com"></script>

  <!-- Alpine.js -->
  <script defer src="https://cdn.jsdelivr.net/npm/alpinejs@3.x.x/dist/cdn.min.js"></script>

  <!-- Chart.js -->
  <script src="https://cdn.jsdelivr.net/npm/chart.js@4.x.x"></script>

  <!-- PocketBase SDK -->
  <script src="https://cdn.jsdelivr.net/npm/pocketbase@0.21.x/dist/pocketbase.umd.js"></script>

  <!-- Google Fonts -->
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap" rel="stylesheet">

  <style>
    body { font-family: 'Inter', system-ui, sans-serif; }
  </style>
</head>
<body class="bg-slate-900 text-slate-100" x-data="dashboard">
  <!-- Dashboard content -->
</body>
</html>
```

### Alpine.js Dashboard Component

```javascript
document.addEventListener('alpine:init', () => {
  Alpine.data('dashboard', () => ({
    // State
    connected: false,
    agents: [],
    signals: [],
    costs: [],
    stats: {
      agentsOnline: 0,
      signalsPerMin: 0,
      activeTasks: 0,
      costToday: 0
    },

    // Initialize
    init() {
      this.connectPocketBase()
      this.subscribeToUpdates()
      this.loadInitialData()
      this.startPolling()
    },

    // PocketBase connection
    async connectPocketBase() {
      this.pb = new PocketBase('http://localhost:8090')

      // Check if admin is authenticated
      if (!this.pb.authStore.isValid) {
        // Redirect to PocketBase admin login
        window.location.href = '/_/'
      }

      this.connected = true
    },

    // Subscribe to real-time updates
    subscribeToUpdates() {
      // Signals
      this.pb.collection('signals').subscribe('*', (e) => {
        if (e.action === 'create') {
          this.signals.unshift(e.record)
          if (this.signals.length > 50) this.signals.pop()
        }
      })

      // Agents
      this.pb.collection('agents').subscribe('*', (e) => {
        this.loadAgents()
      })

      // Costs
      this.pb.collection('costs').subscribe('*', (e) => {
        this.loadCosts()
      })
    },

    // Load initial data
    async loadInitialData() {
      await Promise.all([
        this.loadAgents(),
        this.loadRecentSignals(),
        this.loadCosts(),
        this.updateStats()
      ])
    },

    // Polling for aggregated data
    startPolling() {
      setInterval(() => {
        this.updateStats()
        this.updateCharts()
      }, 5000)
    }
  }))
})
```

---

## Security Considerations

1. **Authentication:** Dashboard requires PocketBase admin authentication
2. **API Keys:** Never expose in frontend code
3. **CORS:** PocketBase handles automatically
4. **Rate Limiting:** Use PocketBase built-in limits

---

## Mobile Optimization

- **Responsive grid:** 1 column on mobile, 2 on tablet, 3 on desktop
- **Touch-friendly:** Large tap targets (min 44px)
- **Performance:** Lazy load charts, virtual scrolling for signals
- **Offline:** Show "Disconnected" banner, buffer updates

---

## Future Enhancements

- [ ] Custom date range picker for cost analysis
- [ ] Agent-specific drill-down pages
- [ ] Export cost reports to CSV
- [ ] Dark/light mode toggle
- [ ] Alert configuration UI
- [ ] Task Gantt chart
- [ ] Signal search and filtering
- [ ] Performance metrics (P50, P95, P99 latency)

---

## Success Metrics

**Performance:**
- Dashboard loads in <2 seconds
- Real-time updates appear within 500ms
- Works smoothly with 1000+ signals

**Usability:**
- Mobile score >90 (Lighthouse)
- Accessible (WCAG 2.1 AA)
- No JavaScript errors in console

**Value:**
- Reduces debugging time by 50%
- Provides visibility into all agents
- Enables proactive cost monitoring
