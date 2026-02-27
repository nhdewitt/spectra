-- name: CreateUser :exec
INSERT INTO users (username, password, role)
VALUES (@username, @password, @role);

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