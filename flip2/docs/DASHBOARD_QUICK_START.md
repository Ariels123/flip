# FLIP2 Dashboard - Quick Start Guide

**Phase 1 Implementation Complete**
**Date:** 2025-12-31

---

## Overview

The FLIP2 Monitoring Dashboard provides real-time visibility into your multi-agent system. Phase 1 establishes the foundation with a no-build-step architecture using CDN-based dependencies.

---

## Starting the Dashboard

### 1. Start the FLIP2 Daemon

```bash
cd /Users/arielspivakovsky/src/flip/flip2
./flip2d --config config/config.yaml --foreground
```

The daemon will:
- Start PocketBase on port 8090
- Enable TLS (HTTPS)
- Serve the dashboard from `pb_public/`

### 2. Access the Dashboard

Open your browser to:
```
http://localhost:8090
```

Or with HTTPS (if TLS is configured):
```
https://localhost:8090
```

---

## What You'll See (Phase 1)

### Header
- **FLIP2 Dashboard** title
- **Connection Status**: Green dot = Connected, Red dot = Disconnected
- **Last Update**: Timestamp showing when data was last refreshed

### Stats Cards (Top Row)
Four cards showing:
- **Agents**: Number of online agents (currently shows 0)
- **Signals**: Signals per minute average (currently shows 0)
- **Tasks**: Active tasks count (currently shows 0)
- **Costs**: Total cost today (currently shows $0.00)

### Agent Activity Panel
- Lists all registered agents
- Shows online/offline status (green/red dot)
- Displays backend type (claude, gemini, antigravity)
- Currently shows "No agents registered"

### Recent Signals Stream
- Shows last 10 signals
- Displays: From Agent → To Agent
- Shows content preview (truncated to 50 chars)
- Relative timestamp (e.g., "5s ago")
- Currently shows "No signals yet"

### Placeholders
- **Signal Throughput Chart**: Coming in Phase 3
- **Cost Breakdown Chart**: Coming in Phase 3
- **System Health Panel**: Coming in Phase 4

---

## Browser Console

The dashboard includes comprehensive logging. Open browser DevTools (F12) to see:

### On Load
```
[FLIP2 Dashboard] Initializing...
[PocketBase] Connecting to http://localhost:8090...
[PocketBase] Connected successfully
[Alpine.js] Initialized successfully
```

### If Not Authenticated
```
[PocketBase] Not authenticated. Admin login required.
[PocketBase] Please log in at: http://localhost:8090/_/
```

### On Connection Failure
```
[PocketBase] Connection failed: [error details]
[PocketBase] Retrying connection...
```

---

## File Locations

All dashboard files are in the `pb_public/` directory:

```
/Users/arielspivakovsky/src/flip/flip2/pb_public/
├── index.html           # Main dashboard HTML
├── css/
│   └── dashboard.css    # Custom styles
└── js/
    └── dashboard.js     # Alpine.js components
```

---

## Configuration

The dashboard connects to PocketBase using the configuration in:
```
/Users/arielspivakovsky/src/flip/flip2/config/config.yaml
```

Key settings:
```yaml
pocketbase:
  host: 0.0.0.0
  port: 8090
  data_dir: ./pb_data
  tls:
    enabled: true
    cert_file: ./certs/flip2.crt
    key_file: ./certs/flip2.key
```

---

## Troubleshooting

### Dashboard doesn't load
1. **Check daemon is running:**
   ```bash
   ps aux | grep flip2d
   ```

2. **Check port 8090 is listening:**
   ```bash
   lsof -i :8090
   ```

3. **Check daemon logs:**
   ```bash
   tail -f /tmp/flip2d.log
   ```

### Connection status shows "Disconnected"
1. Open browser DevTools (F12) and check Console tab
2. Look for PocketBase connection errors
3. Verify daemon is running on port 8090
4. Check for CORS or network errors

### Stats show zeros
This is expected in Phase 1. The dashboard displays:
- Default values (0, 0, 0, $0.00)
- "No agents registered"
- "No signals yet"

Phase 2 will implement:
- Data loading from PocketBase collections
- Real-time subscriptions for live updates
- Actual stats calculations

### Browser console errors
Common issues:
- **CORS errors**: Daemon may not be running
- **Failed to load resource**: Check CDN connectivity
- **Alpine is not defined**: Alpine.js CDN failed to load
- **PocketBase is not defined**: PocketBase SDK CDN failed to load

---

## Architecture

### No Build Step
The dashboard uses CDN-only dependencies:
- **TailwindCSS** (CSS framework)
- **Alpine.js** (Reactive JavaScript)
- **Chart.js** (Charts - Phase 3)
- **PocketBase SDK** (Real-time data)
- **Google Fonts** (Inter font family)

### Why This Matters
- No npm install required
- No build process
- No bundler needed
- Direct edit → refresh workflow
- Easy to modify and extend

### Technology Stack
```
Frontend: Alpine.js 3.14.1 + TailwindCSS 3.x
Backend: PocketBase (embedded in flip2d)
Real-time: PocketBase subscriptions (WebSocket)
Charts: Chart.js 4.4.1 (Phase 3)
```

---

## Phase Roadmap

### Phase 1: Foundation ✅ COMPLETE
- Basic dashboard structure
- PocketBase connection
- Dark theme UI
- Alpine.js setup

### Phase 2: Core Components (Next)
- Stats cards with real data
- Agent activity with live updates
- Recent signals stream
- Real-time subscriptions

### Phase 3: Charts & Analytics
- Signal throughput line chart
- Cost breakdown pie chart
- API endpoints for aggregated data
- Polish and responsive design

### Phase 4: System Health
- System health metrics
- Error tracking
- Connection resilience
- Mobile optimization

---

## Development

### Making Changes
1. Edit files in `pb_public/`:
   - `index.html` - Structure and layout
   - `css/dashboard.css` - Custom styles
   - `js/dashboard.js` - Logic and state

2. Refresh browser (no build step needed)

3. Check browser console for errors

### Adding Features
- Follow the design spec in `docs/MONITORING_DASHBOARD_DESIGN.md`
- Use Alpine.js for reactivity
- Use TailwindCSS for styling
- Keep console logging for debugging

### Testing
1. Start daemon in foreground mode:
   ```bash
   ./flip2d --config config/config.yaml --foreground
   ```

2. Open http://localhost:8090

3. Open browser DevTools (F12)

4. Check Console tab for logs

5. Test connection indicator:
   - Green = Connected
   - Red = Disconnected

---

## Next Steps

To continue development (Phase 2):
1. Implement `loadInitialData()` in `dashboard.js`
2. Implement `subscribeToUpdates()` for real-time data
3. Connect stats cards to actual data
4. Populate agent activity panel
5. Populate recent signals stream

See `docs/MONITORING_DASHBOARD_DESIGN.md` for full specifications.

---

## Support

- **Design Spec**: `/docs/MONITORING_DASHBOARD_DESIGN.md`
- **Phase 1 Summary**: `/docs/PHASE1_COMPLETE.md`
- **Config File**: `/config/config.yaml`
- **Daemon Logs**: `/tmp/flip2d.log`

---

## Summary

Phase 1 provides:
- Professional dark-themed UI
- PocketBase connection with auto-retry
- Responsive layout (mobile-friendly)
- Foundation for real-time updates
- Comprehensive console logging
- No build step complexity

The dashboard is ready for Phase 2 implementation: data integration and real-time subscriptions.
