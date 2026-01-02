-- Sessions Table
-- Stores metadata about each session execution
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'created',
    coordinator_id TEXT NOT NULL,
    parent_session_id TEXT,
    description TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at DATETIME,
    completed_at DATETIME,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_heartbeat_at DATETIME,
    message_count INTEGER NOT NULL DEFAULT 0,
    agent_count INTEGER NOT NULL DEFAULT 0,
    task_count INTEGER NOT NULL DEFAULT 0,
    error_count INTEGER NOT NULL DEFAULT 0,
    metadata TEXT
);

CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);
CREATE INDEX IF NOT EXISTS idx_sessions_coordinator_id ON sessions(coordinator_id);
CREATE INDEX IF NOT EXISTS idx_sessions_parent_session_id ON sessions(parent_session_id);
CREATE INDEX IF NOT EXISTS idx_sessions_created_at ON sessions(created_at DESC);

-- Session Messages Table
-- Logs all messages exchanged within a session
CREATE TABLE IF NOT EXISTS session_messages (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,
    sender_id TEXT NOT NULL,
    recipient_id TEXT,
    content TEXT NOT NULL,
    content_type TEXT NOT NULL DEFAULT 'text',
    message_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    tokens_used TEXT,
    metadata TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    processed_at DATETIME,
    error TEXT,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_session_messages_session_id ON session_messages(session_id);
CREATE INDEX IF NOT EXISTS idx_session_messages_sender_id ON session_messages(sender_id);
CREATE INDEX IF NOT EXISTS idx_session_messages_recipient_id ON session_messages(recipient_id);
CREATE INDEX IF NOT EXISTS idx_session_messages_message_type ON session_messages(message_type);
CREATE INDEX IF NOT EXISTS idx_session_messages_status ON session_messages(status);
CREATE INDEX IF NOT EXISTS idx_session_messages_created_at ON session_messages(created_at DESC);

-- Session Agents Table
-- Tracks agents participating in a session
CREATE TABLE IF NOT EXISTS session_agents (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    name TEXT NOT NULL,
    model TEXT NOT NULL,
    role TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'joining',
    joined_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_activity_at DATETIME,
    left_at DATETIME,
    message_count INTEGER NOT NULL DEFAULT 0,
    task_count INTEGER NOT NULL DEFAULT 0,
    properties TEXT,
    metadata TEXT,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
    UNIQUE (session_id, agent_id)
);

CREATE INDEX IF NOT EXISTS idx_session_agents_session_id ON session_agents(session_id);
CREATE INDEX IF NOT EXISTS idx_session_agents_agent_id ON session_agents(agent_id);
CREATE INDEX IF NOT EXISTS idx_session_agents_status ON session_agents(status);
CREATE INDEX IF NOT EXISTS idx_session_agents_role ON session_agents(role);

-- Session Tasks Table
-- Tracks tasks spawned within a session
CREATE TABLE IF NOT EXISTS session_tasks (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    assigned_agent_id TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'created',
    input TEXT,
    result TEXT,
    error TEXT,
    priority INTEGER NOT NULL DEFAULT 0,
    retry_count INTEGER NOT NULL DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 3,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at DATETIME,
    completed_at DATETIME,
    due_at DATETIME,
    metrics TEXT,
    dependencies TEXT,
    tags TEXT,
    metadata TEXT,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
    FOREIGN KEY (assigned_agent_id) REFERENCES session_agents(agent_id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_session_tasks_session_id ON session_tasks(session_id);
CREATE INDEX IF NOT EXISTS idx_session_tasks_assigned_agent_id ON session_tasks(assigned_agent_id);
CREATE INDEX IF NOT EXISTS idx_session_tasks_status ON session_tasks(status);
CREATE INDEX IF NOT EXISTS idx_session_tasks_priority ON session_tasks(priority DESC);
CREATE INDEX IF NOT EXISTS idx_session_tasks_created_at ON session_tasks(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_session_tasks_due_at ON session_tasks(due_at);

-- Session Variables Table
-- Stores session-scoped variables and configuration
CREATE TABLE IF NOT EXISTS session_variables (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    key TEXT NOT NULL,
    value TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
    UNIQUE (session_id, key)
);

CREATE INDEX IF NOT EXISTS idx_session_variables_session_id ON session_variables(session_id);
