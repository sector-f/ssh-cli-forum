-- +goose Up
CREATE TABLE invites (
    code TEXT PRIMARY KEY,
    created_at DATETIME NOT NULL,
    expires_at DATETIME NOT NULL,
    revoked_at TEXT
);

CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    pubkey BLOB NOT NULL,
    invite_code TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL DEFAULT '',
    role TEXT NOT NULL DEFAULT 'member',
    status TEXT NOT NULL DEFAULT 'active',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen_at TEXT,
    FOREIGN KEY (invite_code) REFERENCES invites(code)
);

CREATE TABLE boards (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    slug TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'active',
    created_by INTEGER,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (created_by) REFERENCES users(id)
);

CREATE TABLE threads (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    board_id INTEGER NOT NULL,
    author_id INTEGER NOT NULL,
    body TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_activity_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TEXT,
    FOREIGN KEY (board_id) REFERENCES boards(id),
    FOREIGN KEY (author_id) REFERENCES users(id)
);

CREATE TABLE comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    thread_id INTEGER NOT NULL,
    author_id INTEGER NOT NULL,
    body TEXT NOT NULL,
    posted_at DATETIME NOT NULL,
    deleted_at TEXT,
    FOREIGN KEY (thread_id) REFERENCES threads(id),
    FOREIGN KEY (author_id) REFERENCES users(id)
);

CREATE TABLE chat_channels (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    requires_membership BOOLEAN NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE chat_channel_memberships (
    channel_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    joined_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    left_at TEXT,
    PRIMARY KEY (channel_id, user_id),
    FOREIGN KEY (channel_id) REFERENCES chat_channels(id),
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE chat_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    channel_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    body TEXT NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY (channel_id) REFERENCES chat_channels(id),
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE UNIQUE INDEX idx_users_username_nocase ON users(lower(username));
CREATE UNIQUE INDEX idx_boards_slug ON boards(slug);
CREATE INDEX idx_boards_status_order ON boards(status, sort_order, name);
CREATE INDEX idx_threads_board_activity ON threads(board_id, last_activity_at DESC);
CREATE INDEX idx_comments_thread_posted ON comments(thread_id, posted_at);
CREATE INDEX idx_invites_expiry ON invites(expires_at);
CREATE INDEX idx_chat_messages_channel_id ON chat_messages(channel_id, id DESC);
CREATE INDEX idx_chat_memberships_user_channel ON chat_channel_memberships(user_id, channel_id, left_at);

INSERT INTO chat_channels(slug, name, requires_membership)
VALUES ('global', 'Global', 0);

-- +goose Down
DROP TABLE chat_messages;
DROP TABLE chat_channel_memberships;
DROP TABLE chat_channels;
DROP TABLE comments;
DROP TABLE threads;
DROP TABLE boards;
DROP TABLE users;
DROP TABLE invites;
