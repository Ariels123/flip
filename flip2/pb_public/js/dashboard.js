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
        await this.loadInitialData();

        // Subscribe to updates (Phase 2)
        this.subscribeToUpdates();

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

    // Load initial data from PocketBase (Phase 2)
    async loadInitialData() {
      console.log('[Dashboard] Loading initial data...');
      try {
        await Promise.all([
          this.loadAgents(),
          this.loadRecentSignals(),
          this.loadCosts()
        ]);

        // Update stats after all data is loaded
        await this.updateStats();

        // Mark the timestamp for "last update"
        this.lastUpdateTimestamp = new Date();
        console.log('[Dashboard] Initial data loaded successfully');
      } catch (error) {
        console.error('[Dashboard] Error loading initial data:', error);
      }
    },

    // Subscribe to real-time updates (Phase 2)
    subscribeToUpdates() {
      console.log('[Dashboard] Setting up real-time subscriptions...');

      // Subscribe to signals collection
      this.pb.collection('signals').subscribe('*', (e) => {
        console.log('[Signals] Real-time event:', e.action, e.record);

        if (e.action === 'create') {
          // Add new signal to the beginning of the array
          this.signals.unshift(e.record);
          // Keep only the last 50 signals (FIFO)
          if (this.signals.length > 50) {
            this.signals.pop();
          }
        } else if (e.action === 'update') {
          // Update existing signal
          const index = this.signals.findIndex(s => s.id === e.record.id);
          if (index !== -1) {
            this.signals[index] = e.record;
          }
        } else if (e.action === 'delete') {
          // Remove deleted signal
          this.signals = this.signals.filter(s => s.id !== e.record.id);
        }

        // Update stats after signal change
        this.updateStats();
        this.lastUpdateTimestamp = new Date();
      }, (err) => {
        console.error('[Signals] Subscription error:', err);
        // Auto-retry subscription after 5 seconds
        setTimeout(() => {
          console.log('[Signals] Retrying subscription...');
          this.subscribeToUpdates();
        }, 5000);
      });

      // Subscribe to agents collection
      this.pb.collection('agents').subscribe('*', (e) => {
        console.log('[Agents] Real-time event:', e.action, e.record);

        if (e.action === 'create') {
          this.agents.push(e.record);
        } else if (e.action === 'update') {
          const index = this.agents.findIndex(a => a.id === e.record.id);
          if (index !== -1) {
            this.agents[index] = e.record;
          }
        } else if (e.action === 'delete') {
          this.agents = this.agents.filter(a => a.id !== e.record.id);
        }

        // Update stats after agent change
        this.updateStats();
        this.lastUpdateTimestamp = new Date();
      }, (err) => {
        console.error('[Agents] Subscription error:', err);
      });

      // Subscribe to costs collection
      this.pb.collection('costs').subscribe('*', (e) => {
        console.log('[Costs] Real-time event:', e.action, e.record);

        if (e.action === 'create') {
          this.costs.push(e.record);
        } else if (e.action === 'update') {
          const index = this.costs.findIndex(c => c.id === e.record.id);
          if (index !== -1) {
            this.costs[index] = e.record;
          }
        } else if (e.action === 'delete') {
          this.costs = this.costs.filter(c => c.id !== e.record.id);
        }

        // Update stats after cost change
        this.updateStats();
        this.lastUpdateTimestamp = new Date();
      }, (err) => {
        console.error('[Costs] Subscription error:', err);
      });

      console.log('[Dashboard] Real-time subscriptions active');
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

    // Load agents from PocketBase (Phase 2)
    async loadAgents() {
      console.log('[Dashboard] Loading agents...');
      try {
        const records = await this.pb.collection('agents').getFullList({
          sort: '-created'
        });
        this.agents = records;
        console.log(`[Agents] Loaded ${records.length} agents`);
      } catch (error) {
        console.error('[Agents] Error loading:', error);
        // Set empty array if collection doesn't exist
        this.agents = [];
      }
    },

    // Load recent signals from PocketBase (Phase 2)
    async loadRecentSignals() {
      console.log('[Dashboard] Loading recent signals...');
      try {
        const records = await this.pb.collection('signals').getFullList({
          sort: '-created',
          limit: 50
        });
        this.signals = records;
        console.log(`[Signals] Loaded ${records.length} signals`);
      } catch (error) {
        console.error('[Signals] Error loading:', error);
        // Set empty array if collection doesn't exist
        this.signals = [];
      }
    },

    // Load today's costs from PocketBase (Phase 2)
    async loadCosts() {
      console.log('[Dashboard] Loading costs...');
      try {
        // Get costs from today (midnight to now)
        const today = new Date();
        today.setHours(0, 0, 0, 0);
        const todayStr = today.toISOString();

        const records = await this.pb.collection('costs').getFullList({
          filter: `created >= "${todayStr}"`,
          sort: '-created'
        });
        this.costs = records;
        console.log(`[Costs] Loaded ${records.length} cost records for today`);
      } catch (error) {
        console.error('[Costs] Error loading:', error);
        // Set empty array if collection doesn't exist
        this.costs = [];
      }
    },

    // Calculate and update dashboard stats (Phase 2)
    async updateStats() {
      console.log('[Dashboard] Updating stats...');

      // 1. Agents Online: count agents with recent heartbeat (last 5 minutes)
      const fiveMinutesAgo = new Date(Date.now() - 5 * 60 * 1000);
      this.stats.agentsOnline = this.agents.filter(agent => {
        if (!agent.last_heartbeat) return false;
        const heartbeat = new Date(agent.last_heartbeat);
        return heartbeat > fiveMinutesAgo;
      }).length;

      // 2. Signals Per Minute: count signals in the last minute
      const oneMinuteAgo = new Date(Date.now() - 60 * 1000);
      this.stats.signalsPerMin = this.signals.filter(signal => {
        const created = new Date(signal.created);
        return created > oneMinuteAgo;
      }).length;

      // 3. Active Tasks: try to count from tasks collection if it exists
      try {
        const activeTasks = await this.pb.collection('tasks').getList(1, 1, {
          filter: 'status = "in_progress"'
        });
        this.stats.activeTasks = activeTasks.totalItems || 0;
      } catch (error) {
        // Tasks collection might not exist or be accessible
        console.warn('[Stats] Could not load active tasks:', error.message);
        this.stats.activeTasks = 0;
      }

      // 4. Cost Today: sum up all costs from today
      this.stats.costToday = this.costs.reduce((sum, cost) => {
        // Assume costs have an 'amount' or 'cost' field
        const amount = cost.amount || cost.cost || 0;
        return sum + parseFloat(amount);
      }, 0);

      console.log('[Stats] Updated:', this.stats);
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
