#!/usr/bin/env bash
# migrate.sh — 为现有 translations.db 添加 user_id 列（存量数据迁移）
#
# 用法：
#   ./migrate.sh                          # 使用默认路径 ./data/translations.db
#   DB_PATH=/path/to/db ./migrate.sh      # 指定数据库路径
#
# 迁移策略：
#   存量记录归属到名为 "legacy" 的系统账户（自动创建），
#   不删除任何历史数据。

set -euo pipefail

DB_PATH="${DB_PATH:-./data/translations.db}"

if [ ! -f "$DB_PATH" ]; then
  echo "Database not found at: $DB_PATH"
  echo "Nothing to migrate — InitDB will create a fresh schema on next start."
  exit 0
fi

echo "Migrating: $DB_PATH"
echo "Backing up to ${DB_PATH}.bak ..."
cp "$DB_PATH" "${DB_PATH}.bak"

sqlite3 "$DB_PATH" <<'SQL'
-- Ensure WAL mode is active.
PRAGMA journal_mode=WAL;

-- Create a "legacy" user to own all pre-existing records.
INSERT OR IGNORE INTO users (id, email, password_hash, created_at)
VALUES (
  'legacy-00000000-0000-0000-0000-000000000000',
  'legacy@system.internal',
  '$2a$12$DISABLED_ACCOUNT_HASH_PLACEHOLDER_DO_NOT_USE_THIS',
  datetime('now')
);

-- Add user_id column to translations if it doesn't already exist.
-- SQLite does not support IF NOT EXISTS for ALTER TABLE ADD COLUMN in older versions,
-- so we check sqlite_master first.
SELECT CASE
  WHEN COUNT(*) = 0 THEN
    (SELECT 'ALTER TABLE translations ADD COLUMN user_id TEXT NOT NULL DEFAULT ''legacy-00000000-0000-0000-0000-000000000000''')
  END
FROM pragma_table_info('translations')
WHERE name = 'user_id';
SQL

# Run the ALTER TABLE statements conditionally via shell logic.
HAS_USER_ID_TRANS=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM pragma_table_info('translations') WHERE name='user_id';")
if [ "$HAS_USER_ID_TRANS" -eq 0 ]; then
  echo "Adding user_id to translations ..."
  sqlite3 "$DB_PATH" \
    "ALTER TABLE translations ADD COLUMN user_id TEXT NOT NULL DEFAULT 'legacy-00000000-0000-0000-0000-000000000000';"
  echo "  Creating index idx_translations_user_created ..."
  sqlite3 "$DB_PATH" \
    "CREATE INDEX IF NOT EXISTS idx_translations_user_created ON translations(user_id, created_at DESC);"
else
  echo "translations.user_id already exists, skipping."
fi

HAS_USER_ID_CORR=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM pragma_table_info('corrections') WHERE name='user_id';")
if [ "$HAS_USER_ID_CORR" -eq 0 ]; then
  echo "Adding user_id to corrections ..."
  sqlite3 "$DB_PATH" \
    "ALTER TABLE corrections ADD COLUMN user_id TEXT NOT NULL DEFAULT 'legacy-00000000-0000-0000-0000-000000000000';"
  echo "  Creating index idx_corrections_user_created ..."
  sqlite3 "$DB_PATH" \
    "CREATE INDEX IF NOT EXISTS idx_corrections_user_created ON corrections(user_id, created_at DESC);"
else
  echo "corrections.user_id already exists, skipping."
fi

# Drop old single-column indexes if they still exist.
sqlite3 "$DB_PATH" "DROP INDEX IF EXISTS idx_translations_created_at;"
sqlite3 "$DB_PATH" "DROP INDEX IF EXISTS idx_corrections_created_at;"

echo "Migration complete."
echo ""
echo "NOTE: Existing records are now owned by legacy@system.internal."
echo "      They will NOT appear in any user's history view."
echo "      If you want to reassign them, run:"
echo "        sqlite3 $DB_PATH \"UPDATE translations SET user_id='<your-user-id>' WHERE user_id LIKE 'legacy%';\""
