-- MySQL：初始版本用户表
-- 注意：casbin_rule 表由 gorm-adapter 首次启动时自动创建，无需在此迁移

CREATE TABLE IF NOT EXISTS `users` (
    `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `username`   VARCHAR(64)     NOT NULL,
    `password`   VARCHAR(128)    NOT NULL COMMENT 'bcrypt 密文',
    `nickname`   VARCHAR(64)     NOT NULL DEFAULT '',
    `role`       VARCHAR(32)     NOT NULL DEFAULT 'user' COMMENT 'admin / user',
    `created_at` TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_username` (`username`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT '用户表';
