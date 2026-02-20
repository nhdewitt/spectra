-- name: CreateToken :exec
INSERT INTO registration_tokens (token, expires_at)
VALUES ($1, $2);

-- name: ValidateAndUseToken :one
UPDATE registration_tokens
SET used = TRUE, used_by = $2
WHERE token = $1 AND NOT used AND expires_at > NOW()
RETURNING token;

-- name: CleanExpiredTokens :exec
DELETE FROM registration_tokens WHERE expires_at < NOW() AND used;