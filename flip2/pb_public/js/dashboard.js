// FLIP2 Dashboard - Alpine.js Component
// Phase 1: Foundation - Basic PocketBase connection and state management

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
    lastUpdate: 'Never',
    pb: null,

    // Initialize dashboard
    init() {
      console.log('[FLIP2 Dashboard] Initializing...');
      this.connectPocketBase();
      this.updateLastUpdateTime();

      // Update "last update" time every second
      setInterval(() => {
        this.updateLastUpdateTime();
      }, 1000);
    },

    // Connect to PocketBase
    async connectPocketBase() {
      try {
        console.log('[PocketBase] Connecting to http://localhost:8090...');
        this.pb = new PocketBase('http://localhost:8090');

        // Check if admin is authenticated
        if (!this.pb.authStore.isValid) {
          console.warn('[PocketBase] Not authenticated. Admin login required.');
          console.log('[PocketBase] Please log in at: http://localhost:8090/_/');
          // For Phase 1, we won't redirect - just log the warning
          // In later phases, we can add proper authentication flow
        } else {
          console.log('[PocketBase] Already authenticated as:', this.pb.authStore.model?.email);
        }

        this.connected = true;
        console.log('[PocketBase] Connected successfully');

        // Load initial data (Phase 2)
        // this.loadInitialData();

        // Subscribe to updates (Phase 2)
        // this.subscribeToUpdates();

      } catch (error) {
        console.error('[PocketBase] Connection failed:', error);
        this.connected = false;

        // Retry connection after 5 seconds
        setTimeout(() => {
          console.log('[PocketBase] Retrying connection...');
          this.connectPocketBase();
        }, 5000);
      }
    },

    // Load initial data from PocketBase (Placeholder for Phase 2)
    async loadInitialData() {
      console.log('[Dashboard] Loading initial data...');
      // TODO: Phase 2 - Implement data loading
      // await Promise.all([
      //   this.loadAgents(),
      //   this.loadRecentSignals(),
      //   this.loadCosts(),
      //   this.updateStats()
      // ]);
    },

    // Subscribe to real-time updates (Placeholder for Phase 2)
    subscribeToUpdates() {
      console.log('[Dashboard] Setting up real-time subscriptions...');
      // TODO: Phase 2 - Implement real-time subscriptions

      // Example for signals:
      // this.pb.collection('signals').subscribe('*', (e) => {
      //   if (e.action === 'create') {
      //     this.signals.unshift(e.record);
      //     if (this.signals.length > 50) this.signals.pop();
      //     this.updateStats();
      //   }
      // });
    },

    // Update last update timestamp
    updateLastUpdateTime() {
      const now = new Date();
      const seconds = Math.floor((now - this.lastUpdateTimestamp) / 1000);

      if (!this.lastUpdateTimestamp) {
        this.lastUpdate = 'Just now';
        this.lastUpdateTimestamp = now;
        return;
      }

      if (seconds < 60) {
        this.lastUpdate = `${seconds}s ago`;
      } else if (seconds < 3600) {
        this.lastUpdate = `${Math.floor(seconds / 60)}m ago`;
      } else {
        this.lastUpdate = `${Math.floor(seconds / 3600)}h ago`;
      }
    },

    // Utility: Truncate long content
    truncateContent(content, maxLength = 50) {
      if (!content) return '';
      if (content.length <= maxLength) return content;
      return content.substring(0, maxLength) + '...';
    },

    // Utility: Format time ago
    timeAgo(timestamp) {
      if (!timestamp) return '';

      const now = new Date();
      const then = new Date(timestamp);
      const seconds = Math.floor((now - then) / 1000);

      if (seconds < 60) return `${seconds}s ago`;
      if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
      if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
      return `${Math.floor(seconds / 86400)}d ago`;
    },

    // Placeholder methods for later phases
    async loadAgents() {
      console.log('[Dashboard] Loading agents...');
      // TODO: Phase 2
    },

    async loadRecentSignals() {
      console.log('[Dashboard] Loading recent signals...');
      // TODO: Phase 2
    },

    async loadCosts() {
      console.log('[Dashboard] Loading costs...');
      // TODO: Phase 2
    },

    async updateStats() {
      console.log('[Dashboard] Updating stats...');
      // TODO: Phase 2
    },

    async updateCharts() {
      console.log('[Dashboard] Updating charts...');
      // TODO: Phase 3
    }
  }));
});

// Log when Alpine.js is ready
document.addEventListener('alpine:initialized', () => {
  console.log('[Alpine.js] Initialized successfully');
});
