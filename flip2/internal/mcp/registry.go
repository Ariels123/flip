package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	_ "modernc.org/sqlite"
)

// registryImpl implements the Registry interface with thread-safe server management.
//
// The registry maintains a map of server name to server instance, along with
// cached capability and tool mappings for efficient lookups. All operations
// are protected by a read-write mutex to support concurrent access.
//
// Design decisions:
// - Uses map[string]Server for O(1) server lookup by name
// - Caches tool-to-server mappings to avoid repeated ListTools calls
// - Uses RWMutex to allow concurrent reads while protecting writes
// - Rebuilds caches on registration/deregistration for consistency
// - Persistence: Save() and Load() methods enable SQLite-based durability
type registryImpl struct {
	// mu protects all fields below
	mu sync.RWMutex

	// servers maps server name to server instance
	servers map[string]Server

	// serverInfos maps server name to ServerInfo metadata (for metadata-only entries)
	// This is used for AddServerInfo when no active Server instance exists
	serverInfos map[string]*ServerInfo

	// toolProviders maps tool name to the server that provides it
	// This cache is rebuilt when servers are registered/deregistered
	toolProviders map[string]Server

	// serverHealth tracks the last known health status of each server
	// true = server responded to last Ping, false = server is down
	serverHealth map[string]bool

	// dbPath is the path to the SQLite database for persistence (optional)
	// If empty, persistence is disabled
	dbPath string
}

// Compile-time check that registryImpl implements Registry interface
var _ Registry = (*registryImpl)(nil)

// NewRegistry creates a new MCP server registry without persistence.
func NewRegistry() Registry {
	return &registryImpl{
		servers:       make(map[string]Server),
		serverInfos:   make(map[string]*ServerInfo),
		toolProviders: make(map[string]Server),
		serverHealth:  make(map[string]bool),
		dbPath:        "",
	}
}

// NewRegistryWithDB creates a new MCP server registry with SQLite persistence.
// The database path will be used by Save() and Load() methods.
// If the database file doesn't exist, it will be created with the proper schema.
func NewRegistryWithDB(dbPath string) (Registry, error) {
	reg := &registryImpl{
		servers:       make(map[string]Server),
		serverInfos:   make(map[string]*ServerInfo),
		toolProviders: make(map[string]Server),
		serverHealth:  make(map[string]bool),
		dbPath:        dbPath,
	}

	// Initialize the database schema
	if err := reg.initializeDB(); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return reg, nil
}

// Register adds a server to the registry.
// The server should already be initialized.
//
// Returns an error if:
// - The server's ServerInfo is nil (not initialized)
// - A server with the same name is already registered
// Auto-saves to database if persistence is enabled.
func (r *registryImpl) Register(ctx context.Context, server Server) error {
	info := server.ServerInfo()
	if info == nil {
		return fmt.Errorf("server not initialized: ServerInfo is nil")
	}

	serverName := info.Name

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.servers[serverName]; exists {
		return fmt.Errorf("server %q is already registered", serverName)
	}

	// Add server to registry
	r.servers[serverName] = server

	// Set health status: if already loaded from database, preserve it; otherwise default to healthy
	if _, exists := r.serverHealth[serverName]; !exists {
		r.serverHealth[serverName] = true // Assume healthy on first registration
	}

	// Rebuild tool provider cache to include new server's tools
	r.rebuildToolProviderCache(ctx)

	// Auto-save to database if persistence is enabled
	r.mu.Unlock()
	err := r.saveServerOnRegister(serverName)
	r.mu.Lock()

	if err != nil {
		// Log error but don't fail registration
		// In a production system, this should use proper logging
		_ = err
	}

	return nil
}

// Deregister removes a server from the registry and closes its connection.
// Auto-deletes from database if persistence is enabled.
func (r *registryImpl) Deregister(ctx context.Context, serverName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	server, exists := r.servers[serverName]
	if !exists {
		return fmt.Errorf("server %q is not registered", serverName)
	}

	// Close the server connection
	if err := server.Close(); err != nil {
		return fmt.Errorf("failed to close server %q: %w", serverName, err)
	}

	// Remove from registry
	delete(r.servers, serverName)
	delete(r.serverInfos, serverName)
	delete(r.serverHealth, serverName)

	// Rebuild tool provider cache to remove this server's tools
	r.rebuildToolProviderCache(ctx)

	// Auto-delete from database if persistence is enabled
	r.mu.Unlock()
	err := r.deleteServerOnDeregister(serverName)
	r.mu.Lock()

	if err != nil {
		// Log error but don't fail deregistration
		// In a production system, this should use proper logging
		_ = err
	}

	return nil
}

// Get retrieves a server by name.
func (r *registryImpl) Get(serverName string) (Server, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	server, exists := r.servers[serverName]
	return server, exists
}

// List returns all registered server names.
func (r *registryImpl) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.servers))
	for name := range r.servers {
		names = append(names, name)
	}
	return names
}

// ListByCapability returns servers that have a specific capability.
// Capability names: "tools", "resources", "prompts", "sampling", "logging"
func (r *registryImpl) ListByCapability(capability string) []Server {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matching []Server

	for _, server := range r.servers {
		caps := server.Capabilities()
		if caps == nil {
			continue
		}

		hasCapability := false
		switch capability {
		case "tools":
			hasCapability = caps.Tools != nil
		case "resources":
			hasCapability = caps.Resources != nil
		case "prompts":
			hasCapability = caps.Prompts != nil
		case "logging":
			hasCapability = caps.Logging != nil
		case "completions":
			hasCapability = caps.Completions != nil
		}

		if hasCapability {
			matching = append(matching, server)
		}
	}

	return matching
}

// FindToolProvider finds a server that provides a specific tool.
func (r *registryImpl) FindToolProvider(toolName string) (Server, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	server, exists := r.toolProviders[toolName]
	return server, exists
}

// FindResourceProvider finds a server that can provide a resource.
//
// This implementation checks each server's resources to find a match.
// For better performance with many resources, consider caching resource
// URIs similar to how tools are cached.
func (r *registryImpl) FindResourceProvider(uri string) (Server, bool) {
	r.mu.RLock()
	servers := make([]Server, 0, len(r.servers))
	for _, server := range r.servers {
		servers = append(servers, server)
	}
	r.mu.RUnlock()

	// Check each server for the resource
	// We need to release the lock before making network calls
	ctx := context.Background()
	for _, server := range servers {
		caps := server.Capabilities()
		if caps == nil || caps.Resources == nil {
			continue
		}

		// Try to read the resource - if it succeeds, this server provides it
		// This is more reliable than listing resources, which may be paginated
		_, err := server.ReadResource(ctx, uri)
		if err == nil {
			return server, true
		}
	}

	return nil, false
}

// AllTools returns all tools from all registered servers.
// Returns a map of tool name to the server that provides it.
func (r *registryImpl) AllTools(ctx context.Context) (map[string]Server, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Return a copy of the cached tool providers
	result := make(map[string]Server, len(r.toolProviders))
	for toolName, server := range r.toolProviders {
		result[toolName] = server
	}
	return result, nil
}

// AllResources returns all resources from all registered servers.
func (r *registryImpl) AllResources(ctx context.Context) ([]Resource, error) {
	r.mu.RLock()
	servers := make([]Server, 0, len(r.servers))
	for _, server := range r.servers {
		servers = append(servers, server)
	}
	r.mu.RUnlock()

	var allResources []Resource

	// Collect resources from each server
	for _, server := range servers {
		caps := server.Capabilities()
		if caps == nil || caps.Resources == nil {
			continue
		}

		// List all resources (handle pagination)
		var cursor *string
		for {
			result, err := server.ListResources(ctx, cursor)
			if err != nil {
				return nil, fmt.Errorf("failed to list resources from server %q: %w",
					server.ServerInfo().Name, err)
			}

			allResources = append(allResources, result.Resources...)

			if result.NextCursor == nil {
				break
			}
			cursor = result.NextCursor
		}
	}

	return allResources, nil
}

// AllPrompts returns all prompts from all registered servers.
func (r *registryImpl) AllPrompts(ctx context.Context) ([]Prompt, error) {
	r.mu.RLock()
	servers := make([]Server, 0, len(r.servers))
	for _, server := range r.servers {
		servers = append(servers, server)
	}
	r.mu.RUnlock()

	var allPrompts []Prompt

	// Collect prompts from each server
	for _, server := range servers {
		caps := server.Capabilities()
		if caps == nil || caps.Prompts == nil {
			continue
		}

		// List all prompts (handle pagination)
		var cursor *string
		for {
			result, err := server.ListPrompts(ctx, cursor)
			if err != nil {
				return nil, fmt.Errorf("failed to list prompts from server %q: %w",
					server.ServerInfo().Name, err)
			}

			allPrompts = append(allPrompts, result.Prompts...)

			if result.NextCursor == nil {
				break
			}
			cursor = result.NextCursor
		}
	}

	return allPrompts, nil
}

// Close closes all server connections.
func (r *registryImpl) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []error

	for name, server := range r.servers {
		if err := server.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close server %q: %w", name, err))
		}
	}

	// Clear all maps
	r.servers = make(map[string]Server)
	r.toolProviders = make(map[string]Server)
	r.serverHealth = make(map[string]bool)

	if len(errs) > 0 {
		return fmt.Errorf("errors closing servers: %v", errs)
	}

	return nil
}

// Update applies a custom update function to a server's metadata.
// Thread-safe via write lock.
func (r *registryImpl) Update(serverName string, updateFn func(*ServerInfo) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	server, exists := r.servers[serverName]
	if !exists {
		return fmt.Errorf("server %q is not registered", serverName)
	}

	// Get the current ServerInfo
	info := server.ServerInfo()
	if info == nil {
		return fmt.Errorf("server %q has no ServerInfo", serverName)
	}

	// Apply the update function
	if err := updateFn(info); err != nil {
		return fmt.Errorf("failed to update server %q: %w", serverName, err)
	}

	return nil
}

// ListAll returns detailed information about all registered servers.
// Thread-safe via read lock.
func (r *registryImpl) ListAll() []ServerInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	infos := make([]ServerInfo, 0, len(r.servers))
	for _, server := range r.servers {
		info := server.ServerInfo()
		if info != nil {
			// Create a copy with health status included
			serverInfo := *info
			infos = append(infos, serverInfo)
		}
	}

	return infos
}

// GetHealth checks the health status of a server.
// Thread-safe via read lock.
func (r *registryImpl) GetHealth(serverName string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, exists := r.servers[serverName]; !exists {
		return false, fmt.Errorf("server %q is not registered", serverName)
	}

	health := r.serverHealth[serverName]
	return health, nil
}

// SetHealth updates the health status of a server.
// Thread-safe via write lock.
// Auto-saves to database if persistence is enabled.
func (r *registryImpl) SetHealth(serverName string, healthy bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.servers[serverName]; !exists {
		return fmt.Errorf("server %q is not registered", serverName)
	}

	r.serverHealth[serverName] = healthy

	// Auto-save health status to database if persistence is enabled
	if r.dbPath != "" {
		r.mu.Unlock()
		server := r.servers[serverName]
		r.mu.Lock()

		serverInfo := server.ServerInfo()
		if serverInfo == nil {
			return nil
		}

		db, err := sql.Open("sqlite", r.dbPath)
		if err != nil {
			return nil // Don't fail on database error
		}
		defer db.Close()

		updateSQL := `
		UPDATE mcp_servers SET health = ?, updated_at = CURRENT_TIMESTAMP
		WHERE name = ?
		`
		_, _ = db.Exec(updateSQL, healthy, serverName)
	}

	return nil
}

// rebuildToolProviderCache rebuilds the tool name -> server mapping.
// Must be called with r.mu held (write lock).
func (r *registryImpl) rebuildToolProviderCache(ctx context.Context) {
	// Clear existing cache
	r.toolProviders = make(map[string]Server)

	// Get server names in sorted order for consistent iteration
	// This ensures that when multiple servers provide the same tool,
	// the one registered first (alphabetically) is preferred
	serverNames := make([]string, 0, len(r.servers))
	for name := range r.servers {
		serverNames = append(serverNames, name)
	}
	sort.Strings(serverNames)

	// Build new cache from all servers in sorted order
	for _, serverName := range serverNames {
		server := r.servers[serverName]
		caps := server.Capabilities()
		if caps == nil || caps.Tools == nil {
			continue
		}

		// List all tools from this server (handle pagination)
		var cursor *string
		for {
			result, err := server.ListTools(ctx, cursor)
			if err != nil {
				// Log error but continue with other servers
				// In a production system, this should use proper logging
				_ = serverName // Suppress unused variable warning
				break
			}

			// Add each tool to the cache
			for _, tool := range result.Tools {
				// First server to provide a tool wins
				// This handles tool name conflicts across servers
				if _, exists := r.toolProviders[tool.Name]; !exists {
					r.toolProviders[tool.Name] = server
				}
			}

			if result.NextCursor == nil {
				break
			}
			cursor = result.NextCursor
		}
	}
}

// initializeDB creates the SQLite database schema if it doesn't exist.
// Called during NewRegistryWithDB initialization.
func (r *registryImpl) initializeDB() error {
	if r.dbPath == "" {
		return nil // Persistence disabled
	}

	db, err := sql.Open("sqlite", r.dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Create the mcp_servers table if it doesn't exist
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS mcp_servers (
		name TEXT PRIMARY KEY,
		capabilities TEXT NOT NULL,
		health BOOLEAN NOT NULL,
		metadata TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)
	`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	return nil
}

// SaveRegistry persists the current registry state to SQLite.
// Only saves server metadata and health status, not the active Server instances.
// Returns an error if dbPath is empty (persistence not enabled).
func (r *registryImpl) SaveRegistry() error {
	if r.dbPath == "" {
		return fmt.Errorf("persistence not enabled: dbPath is empty")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	db, err := sql.Open("sqlite", r.dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Clear existing records
	if _, err := tx.Exec("DELETE FROM mcp_servers"); err != nil {
		return fmt.Errorf("failed to clear table: %w", err)
	}

	// Insert all current servers
	insertSQL := `
	INSERT INTO mcp_servers (name, capabilities, health, metadata, updated_at)
	VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
	`

	stmt, err := tx.Prepare(insertSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for serverName, server := range r.servers {
		serverInfo := server.ServerInfo()
		if serverInfo == nil {
			continue
		}

		// Serialize capabilities to JSON
		caps := server.Capabilities()
		capsJSON, err := json.Marshal(caps)
		if err != nil {
			return fmt.Errorf("failed to marshal capabilities for server %q: %w", serverName, err)
		}

		// Serialize server info as metadata
		metadataJSON, err := json.Marshal(serverInfo)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata for server %q: %w", serverName, err)
		}

		health := r.serverHealth[serverName]

		_, err = stmt.Exec(serverName, capsJSON, health, metadataJSON)
		if err != nil {
			return fmt.Errorf("failed to insert server %q: %w", serverName, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// LoadRegistry loads server metadata and health status from SQLite.
// Does not restore Server instances - those must be reconstructed externally.
// Populates serverHealth map and returns a map of server names to their metadata.
// Returns an error if dbPath is empty (persistence not enabled).
func (r *registryImpl) LoadRegistry() (map[string]ServerInfo, error) {
	if r.dbPath == "" {
		return nil, fmt.Errorf("persistence not enabled: dbPath is empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	db, err := sql.Open("sqlite", r.dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Query all servers
	querySQL := `SELECT name, capabilities, health, metadata FROM mcp_servers`
	rows, err := db.Query(querySQL)
	if err != nil {
		return nil, fmt.Errorf("failed to query servers: %w", err)
	}
	defer rows.Close()

	serverMetadata := make(map[string]ServerInfo)

	for rows.Next() {
		var name string
		var capsJSON, metadataJSON string
		var health bool

		err := rows.Scan(&name, &capsJSON, &health, &metadataJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Restore server info from metadata
		var serverInfo ServerInfo
		err = json.Unmarshal([]byte(metadataJSON), &serverInfo)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata for server %q: %w", name, err)
		}

		// Restore health status
		r.serverHealth[name] = health

		serverMetadata[name] = serverInfo
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return serverMetadata, nil
}

// SaveServerOnRegister is called automatically when a server is registered.
// Auto-saves if persistence is enabled.
func (r *registryImpl) saveServerOnRegister(serverName string) error {
	if r.dbPath == "" {
		return nil // Persistence disabled
	}

	r.mu.RLock()
	server, exists := r.servers[serverName]
	health := r.serverHealth[serverName]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("server %q not found", serverName)
	}

	serverInfo := server.ServerInfo()
	if serverInfo == nil {
		return fmt.Errorf("server %q has no ServerInfo", serverName)
	}

	db, err := sql.Open("sqlite", r.dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	caps := server.Capabilities()
	capsJSON, _ := json.Marshal(caps)
	metadataJSON, _ := json.Marshal(serverInfo)

	insertSQL := `
	INSERT OR REPLACE INTO mcp_servers (name, capabilities, health, metadata, updated_at)
	VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
	`

	_, err = db.Exec(insertSQL, serverName, capsJSON, health, metadataJSON)
	if err != nil {
		return fmt.Errorf("failed to save server %q: %w", serverName, err)
	}

	return nil
}

// DeleteServerOnDeregister is called automatically when a server is deregistered.
// Auto-deletes from persistence if enabled.
func (r *registryImpl) deleteServerOnDeregister(serverName string) error {
	if r.dbPath == "" {
		return nil // Persistence disabled
	}

	db, err := sql.Open("sqlite", r.dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	deleteSQL := `DELETE FROM mcp_servers WHERE name = ?`
	_, err = db.Exec(deleteSQL, serverName)
	if err != nil {
		return fmt.Errorf("failed to delete server %q: %w", serverName, err)
	}

	return nil
}

// =============================================================================
// CRUD Operations for ServerInfo (MCP-003)
// =============================================================================

// AddServerInfo adds a new ServerInfo to the registry.
// Returns an error if a server with the same ID (Name) already exists.
// Thread-safe via write lock.
// Auto-saves to database if persistence is enabled.
func (r *registryImpl) AddServerInfo(serverInfo *ServerInfo) error {
	if serverInfo == nil {
		return fmt.Errorf("serverInfo cannot be nil")
	}

	if serverInfo.Name == "" {
		return fmt.Errorf("serverInfo.Name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if server already exists (either as active server or metadata entry)
	if _, exists := r.servers[serverInfo.Name]; exists {
		return fmt.Errorf("server %q already exists", serverInfo.Name)
	}

	if _, exists := r.serverInfos[serverInfo.Name]; exists {
		return fmt.Errorf("server %q already exists", serverInfo.Name)
	}

	// Store the ServerInfo metadata
	copy := *serverInfo
	r.serverInfos[serverInfo.Name] = &copy
	r.serverHealth[serverInfo.Name] = true

	// Auto-save to database if persistence is enabled
	r.mu.Unlock()
	err := r.saveServerInfoToDB(serverInfo)
	r.mu.Lock()

	if err != nil {
		// Clean up on database failure
		delete(r.serverInfos, serverInfo.Name)
		delete(r.serverHealth, serverInfo.Name)
		return fmt.Errorf("failed to save server info to database: %w", err)
	}

	return nil
}

// RemoveServerInfo removes a ServerInfo from the registry by ID.
// Returns an error if the server doesn't exist.
// Thread-safe via write lock.
// Auto-deletes from database if persistence is enabled.
func (r *registryImpl) RemoveServerInfo(id string) error {
	if id == "" {
		return fmt.Errorf("id cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if server exists (either as active server or metadata entry)
	serverExists := false
	if _, exists := r.servers[id]; exists {
		serverExists = true
	}
	if _, exists := r.serverInfos[id]; exists {
		serverExists = true
	}

	if !serverExists {
		return fmt.Errorf("server %q not found", id)
	}

	// Remove from registry
	delete(r.servers, id)
	delete(r.serverInfos, id)
	delete(r.serverHealth, id)

	// Auto-delete from database if persistence is enabled
	r.mu.Unlock()
	err := r.deleteServerInfoFromDB(id)
	r.mu.Lock()

	if err != nil {
		return fmt.Errorf("failed to delete server info from database: %w", err)
	}

	return nil
}

// UpdateServerInfo updates an existing ServerInfo by ID.
// Returns an error if the server doesn't exist.
// Thread-safe via write lock.
// Auto-saves to database if persistence is enabled.
func (r *registryImpl) UpdateServerInfo(id string, serverInfo *ServerInfo) error {
	if id == "" {
		return fmt.Errorf("id cannot be empty")
	}

	if serverInfo == nil {
		return fmt.Errorf("serverInfo cannot be nil")
	}

	// ID and Name must match
	if serverInfo.Name != id {
		return fmt.Errorf("serverInfo.Name %q does not match id %q", serverInfo.Name, id)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if server exists (either as active server or metadata entry)
	serverExists := false
	if _, exists := r.servers[id]; exists {
		serverExists = true
	}
	if _, exists := r.serverInfos[id]; exists {
		serverExists = true
	}

	if !serverExists {
		return fmt.Errorf("server %q not found", id)
	}

	// Update the metadata entry
	copy := *serverInfo
	r.serverInfos[id] = &copy

	// Auto-save to database if persistence is enabled
	r.mu.Unlock()
	err := r.saveServerInfoToDB(serverInfo)
	r.mu.Lock()

	if err != nil {
		return fmt.Errorf("failed to save updated server info to database: %w", err)
	}

	return nil
}

// GetServerInfo retrieves a ServerInfo by ID.
// Returns nil if the server doesn't exist.
// Thread-safe via read lock.
func (r *registryImpl) GetServerInfo(id string) *ServerInfo {
	if id == "" {
		return nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check if we have metadata-only entry
	if info, exists := r.serverInfos[id]; exists {
		// Return a copy to prevent external modification
		copy := *info
		return &copy
	}

	// Check if we have a registered server
	if server, exists := r.servers[id]; exists {
		info := server.ServerInfo()
		if info != nil {
			// Return a copy to prevent external modification
			copy := *info
			return &copy
		}
	}

	// If no registered server, try to load from database
	if r.dbPath != "" {
		info, err := r.loadServerInfoFromDB(id)
		if err == nil && info != nil {
			// Return a copy to prevent external modification
			copy := *info
			return &copy
		}
	}

	return nil
}

// ListServerInfos returns all ServerInfo entries in the registry.
// Thread-safe via read lock.
func (r *registryImpl) ListServerInfos() []*ServerInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Collect ServerInfo from metadata-only entries first
	infos := make([]*ServerInfo, 0, len(r.serverInfos)+len(r.servers))
	seen := make(map[string]bool)

	// Add metadata-only entries
	for _, info := range r.serverInfos {
		if info != nil {
			// Create a copy to prevent external modification
			copy := *info
			infos = append(infos, &copy)
			seen[info.Name] = true
		}
	}

	// Collect ServerInfo from registered servers
	for _, server := range r.servers {
		info := server.ServerInfo()
		if info != nil && !seen[info.Name] {
			// Create a copy to prevent external modification
			copy := *info
			infos = append(infos, &copy)
			seen[info.Name] = true
		}
	}

	// If persistence is enabled, also load metadata-only entries from database
	if r.dbPath != "" {
		// Load all entries from database
		allInfos, err := r.loadAllServerInfosFromDB()
		if err == nil {
			for _, info := range allInfos {
				if !seen[info.Name] {
					// Create a copy to prevent external modification
					copy := *info
					infos = append(infos, &copy)
					seen[info.Name] = true
				}
			}
		}
	}

	return infos
}

// =============================================================================
// Helper methods for database persistence (private)
// =============================================================================

// saveServerInfoToDB saves a ServerInfo to the database.
// Must be called with r.mu unlocked to avoid deadlock.
func (r *registryImpl) saveServerInfoToDB(serverInfo *ServerInfo) error {
	if r.dbPath == "" {
		return nil // Persistence disabled
	}

	if serverInfo == nil {
		return fmt.Errorf("serverInfo is nil")
	}

	db, err := sql.Open("sqlite", r.dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Serialize ServerInfo as metadata
	metadataJSON, err := json.Marshal(serverInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal server info: %w", err)
	}

	// Use empty JSON for capabilities and default health status
	capsJSON := "{}"
	health := true

	// Get health status if available
	r.mu.RLock()
	if h, exists := r.serverHealth[serverInfo.Name]; exists {
		health = h
	}
	r.mu.RUnlock()

	insertSQL := `
	INSERT OR REPLACE INTO mcp_servers (name, capabilities, health, metadata, updated_at)
	VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
	`

	_, err = db.Exec(insertSQL, serverInfo.Name, capsJSON, health, metadataJSON)
	if err != nil {
		return fmt.Errorf("failed to insert server info: %w", err)
	}

	return nil
}

// deleteServerInfoFromDB deletes a ServerInfo from the database.
// Must be called with r.mu unlocked to avoid deadlock.
func (r *registryImpl) deleteServerInfoFromDB(id string) error {
	if r.dbPath == "" {
		return nil // Persistence disabled
	}

	if id == "" {
		return fmt.Errorf("id is empty")
	}

	db, err := sql.Open("sqlite", r.dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	deleteSQL := `DELETE FROM mcp_servers WHERE name = ?`
	_, err = db.Exec(deleteSQL, id)
	if err != nil {
		return fmt.Errorf("failed to delete server info: %w", err)
	}

	return nil
}

// loadServerInfoFromDB loads a single ServerInfo from the database.
// Must be called with r.mu unlocked to avoid deadlock.
func (r *registryImpl) loadServerInfoFromDB(id string) (*ServerInfo, error) {
	if r.dbPath == "" {
		return nil, fmt.Errorf("persistence not enabled")
	}

	if id == "" {
		return nil, fmt.Errorf("id is empty")
	}

	db, err := sql.Open("sqlite", r.dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	querySQL := `SELECT metadata FROM mcp_servers WHERE name = ?`
	var metadataJSON string
	err = db.QueryRow(querySQL, id).Scan(&metadataJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("server %q not found", id)
		}
		return nil, fmt.Errorf("failed to query server info: %w", err)
	}

	var serverInfo ServerInfo
	err = json.Unmarshal([]byte(metadataJSON), &serverInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal server info: %w", err)
	}

	return &serverInfo, nil
}

// loadAllServerInfosFromDB loads all ServerInfo entries from the database.
// Must be called with r.mu unlocked to avoid deadlock.
func (r *registryImpl) loadAllServerInfosFromDB() ([]*ServerInfo, error) {
	if r.dbPath == "" {
		return nil, fmt.Errorf("persistence not enabled")
	}

	db, err := sql.Open("sqlite", r.dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	querySQL := `SELECT metadata FROM mcp_servers`
	rows, err := db.Query(querySQL)
	if err != nil {
		return nil, fmt.Errorf("failed to query servers: %w", err)
	}
	defer rows.Close()

	var infos []*ServerInfo

	for rows.Next() {
		var metadataJSON string
		err := rows.Scan(&metadataJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		var serverInfo ServerInfo
		err = json.Unmarshal([]byte(metadataJSON), &serverInfo)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal server info: %w", err)
		}

		infos = append(infos, &serverInfo)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return infos, nil
}
