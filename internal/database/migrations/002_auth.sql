CREATE TABLE roles(name TEXT PRIMARY KEY);
INSERT INTO roles VALUES('student'),('cook'),('courier'),('admin');
ALTER TABLE users ADD COLUMN password_hash TEXT NOT NULL DEFAULT '!reset-required',ADD COLUMN role TEXT NOT NULL DEFAULT 'student' REFERENCES roles(name),ADD COLUMN session_version BIGINT NOT NULL DEFAULT 0;
UPDATE users SET is_blocked=true;
CREATE TABLE refresh_tokens(id BIGSERIAL PRIMARY KEY,user_id BIGINT NOT NULL REFERENCES users(id),token_hash TEXT UNIQUE NOT NULL,expires_at TIMESTAMPTZ NOT NULL,is_revoked BOOLEAN NOT NULL DEFAULT false);
