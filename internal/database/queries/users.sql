-- name: GetUserConfig :many
SELECT config_key, config_value
FROM user_config
WHERE user_id = @user_id;

-- name: GetUserConfigKey :one
SELECT config_value
FROM user_config
WHERE user_id = @user_id AND config_key = @config_key;

-- name: SetUserConfig :exec
INSERT INTO user_config (user_id, config_key, config_value, updated_at)
VALUES (@user_id, @config_key, @config_value, NOW())
ON CONFLICT (user_id, config_key)
DO UPDATE SET config_value = EXCLUDED.config_value, updated_at = NOW();

-- name: DeleteUserConfig :exec
DELETE FROM user_config
WHERE user_id = @user_id AND config_key = @config_key;