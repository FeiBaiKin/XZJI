-- +goose Up
CREATE TABLE users (
  id                BIGINT       NOT NULL AUTO_INCREMENT,
  email             VARCHAR(191) NOT NULL,
  password_hash     VARCHAR(255) NOT NULL,
  nickname          VARCHAR(64)  NOT NULL,
  avatar_media_id   BIGINT       NULL,
  role              VARCHAR(16)  NOT NULL DEFAULT 'user' COMMENT 'user / admin',
  status            VARCHAR(16)  NOT NULL DEFAULT 'active' COMMENT 'active / banned',
  inviter_id        BIGINT       NULL COMMENT '邀请人，用于推广分佣',
  email_verified_at DATETIME(3)  NULL,
  last_login_at     DATETIME(3)  NULL,
  created_at        DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at        DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_users_email (email),
  KEY idx_users_inviter (inviter_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户';

CREATE TABLE sessions (
  id          CHAR(64)     NOT NULL COMMENT '令牌的 SHA-256，客户端只持有原始令牌',
  user_id     BIGINT       NOT NULL,
  client      VARCHAR(16)  NOT NULL COMMENT 'web / admin / app',
  ttl_seconds INT          NOT NULL COMMENT '有效期时长，续期时沿用',
  ip          VARCHAR(45)  NOT NULL DEFAULT '',
  user_agent  VARCHAR(255) NOT NULL DEFAULT '',
  expires_at  DATETIME(3)  NOT NULL,
  created_at  DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_sessions_user (user_id),
  KEY idx_sessions_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='登录会话';

CREATE TABLE email_codes (
  id         BIGINT       NOT NULL AUTO_INCREMENT,
  email      VARCHAR(191) NOT NULL,
  purpose    VARCHAR(16)  NOT NULL COMMENT 'register / reset_password',
  code_hash  CHAR(64)     NOT NULL,
  attempts   INT          NOT NULL DEFAULT 0 COMMENT '校验失败次数，达到上限即作废',
  ip         VARCHAR(45)  NOT NULL DEFAULT '',
  expires_at DATETIME(3)  NOT NULL,
  used_at    DATETIME(3)  NULL COMMENT '已使用或已作废的时间',
  created_at DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_email_codes_lookup (email, purpose, id),
  KEY idx_email_codes_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='邮箱验证码';

CREATE TABLE admin_logs (
  id          BIGINT      NOT NULL AUTO_INCREMENT,
  admin_id    BIGINT      NOT NULL,
  action      VARCHAR(64) NOT NULL,
  target_type VARCHAR(32) NOT NULL DEFAULT '',
  target_id   BIGINT      NOT NULL DEFAULT 0,
  detail      JSON        NULL,
  ip          VARCHAR(45) NOT NULL DEFAULT '',
  created_at  DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_admin_logs_admin (admin_id, id),
  KEY idx_admin_logs_target (target_type, target_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='后台操作日志';

-- +goose Down
DROP TABLE admin_logs;
DROP TABLE email_codes;
DROP TABLE sessions;
DROP TABLE users;
