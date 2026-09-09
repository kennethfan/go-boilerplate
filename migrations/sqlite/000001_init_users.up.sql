-- SQLite：初始版本用户表
-- 注意：casbin_rule 表由 gorm-adapter 首次启动时自动创建，无需在此迁移

CREATE TABLE IF NOT EXISTS users (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    username   VARCHAR(64)  NOT NULL UNIQUE,
    password   VARCHAR(128) NOT NULL,
    nickname   VARCHAR(64)  NOT NULL DEFAULT '',
    role       VARCHAR(32)  NOT NULL DEFAULT 'user',
    created_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP
);
