-- name: CreateUser :exec
INSERT INTO users (username, password, role)
VALUES (@username, @password, @role);

-- name: UpsertSuperadmin :exec
INSERT INTO users (username, password, role)
VALUES (@username, @password, 'superadmin')
ON CONFLICT (username) DO UPDATE
SET password = @password, role = 'superadmin', updated_at = NOW();

-- name: GetUserByUsername :one
SELECT id, username, password, role, created_at
FROM users
WHERE username = @username;

-- name: UserCount :one
SELECT COUNT(*) FROM users;

-- name: CreateSession :exec
INSERT INTO sessions (token, user_id, ip_address, expires_at)
VALUES (@token, @user_id, @ip_address, @expires_at);

-- name: GetSession :one
SELECT s.token, s.user_id, s.ip_address, s.expires_at, u.username, u.role
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token = @token AND s.expires_at > NOW();

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token = @token;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at <= NOW();

-- name: DeleteUserSessions :exec
DELETE FROM sessions WHERE user_id = @user_id;

-- name: ListUsers :many
SELECT id, username, role, created_at, updated_at
FROM users
ORDER BY created_at ASC;

-- name: GetUserByID :one
SELECT id, username, role, created_at, updated_at
FROM users
WHERE id = @id;

-- name: DeleteUser :execrows
-- Refuses to delete the last superadmin. Zero rows means refused or not found.
-- Locking every superadmin row first serializes concurrent deletes and demotions.
-- A separate count would let two requests each see two superadmins and remove
-- one apiece.
WITH superadmins AS (
	SELECT id FROM users WHERE role = 'superadmin' ORDER BY id FOR UPDATE
)
DELETE FROM users
WHERE users.id = @id
	AND (users.role <> 'superadmin' OR (SELECT count(*) FROM superadmins) > 1);

-- name: UpdateUserRole :execrows
-- Refuses to demote the last superadmin, the the same locking as DeleteUser.
WITH superadmins AS (
	SELECT id FROM users WHERE role = 'superadmin' ORDER BY id FOR UPDATE
)
UPDATE users SET role = @role, updated_at = NOW()
WHERE users.id = @id
	AND (users.role <> 'superadmin'
		OR @role::text = 'superadmin'
		OR (SELECT count(*) FROM superadmins) > 1);

-- name: UpdateUserPassword :exec
UPDATE users SET password = @password, updated_at = NOW()
WHERE id = @id;

-- name: ListUsersWithLastLogin :many
SELECT u.id, u.username, u.role, u.created_at, u.updated_at,
	MAX(s.created_at)::TIMESTAMPTZ AS last_login
FROM users u
LEFT JOIN sessions s ON s.user_id = u.id
GROUP BY u.id, u.username, u.role, u.created_at, u.updated_at
ORDER BY u.created_at ASC;