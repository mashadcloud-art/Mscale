package db

import "database/sql"

func Migrate(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			display_name TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS devices (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			device_name TEXT NOT NULL,
			platform TEXT NOT NULL,
			device_type TEXT NOT NULL,
			public_key TEXT NOT NULL,
			overlay_ip TEXT,
			app_version TEXT,
			os_version TEXT,
			enrolled_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_seen_at DATETIME,
			status TEXT NOT NULL DEFAULT 'offline'
		);`,

		`CREATE TABLE IF NOT EXISTS exit_nodes (
			id TEXT PRIMARY KEY,
			device_id TEXT NOT NULL UNIQUE,
			owner_user_id TEXT NOT NULL,
			label TEXT NOT NULL,
			country_code TEXT,
			city TEXT,
			is_private INTEGER NOT NULL DEFAULT 1,
			is_enabled INTEGER NOT NULL DEFAULT 1,
			health_status TEXT NOT NULL DEFAULT 'unknown',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS invites (
			id TEXT PRIMARY KEY,
			code TEXT UNIQUE NOT NULL,
			inviter_user_id TEXT NOT NULL,
			invitee_email TEXT,
			target_type TEXT NOT NULL,
			target_id TEXT NOT NULL,
			role_name TEXT DEFAULT 'member',
			max_uses INTEGER NOT NULL DEFAULT 1,
			used_count INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'active',
			expires_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS exit_node_access (
			id TEXT PRIMARY KEY,
			exit_node_id TEXT NOT NULL,
			grantee_user_id TEXT NOT NULL,
			granted_by_user_id TEXT NOT NULL,
			access_mode TEXT NOT NULL DEFAULT 'use',
			status TEXT NOT NULL DEFAULT 'active',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME,
			UNIQUE(exit_node_id, grantee_user_id)
		);`,

		`CREATE TABLE IF NOT EXISTS device_sessions (
			id TEXT PRIMARY KEY,
			device_id TEXT NOT NULL,
			started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_heartbeat_at DATETIME,
			ended_at DATETIME,
			status TEXT NOT NULL DEFAULT 'active'
		);`,

		`CREATE TABLE IF NOT EXISTS audit_logs (
			id TEXT PRIMARY KEY,
			actor_user_id TEXT,
			action TEXT NOT NULL,
			target_type TEXT NOT NULL,
			target_id TEXT NOT NULL,
			metadata_json TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS user_sessions (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			expires_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS auth_bridge_tokens (
			token TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			expires_at DATETIME NOT NULL
		);`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}

	return nil
}
