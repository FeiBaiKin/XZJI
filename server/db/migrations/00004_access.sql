-- +goose Up
ALTER TABLE posts
  ADD COLUMN price            BIGINT     NOT NULL DEFAULT 0 COMMENT '单独购买的价格（分），0 表示不能单独购买' AFTER pinned,
  ADD COLUMN member_free_rank INT        NOT NULL DEFAULT 0 COMMENT '会员等级达到该 rank 即免费，0 表示会员也不免费' AFTER price,
  ADD COLUMN member_discount  TINYINT(1) NOT NULL DEFAULT 1 COMMENT '等级不够免费的会员是否享受等级折扣' AFTER member_free_rank;

ALTER TABLE resources
  ADD COLUMN download_cond VARCHAR(32) NOT NULL DEFAULT 'public' COMMENT '下载区解锁条件：public / login / comment / paid / member:N' AFTER changelog;

CREATE TABLE post_hidden_blocks (
  id           BIGINT       NOT NULL AUTO_INCREMENT,
  post_id      BIGINT       NOT NULL,
  seq          INT          NOT NULL COMMENT '对应正文中占位符的序号，从 1 开始',
  cond         VARCHAR(32)  NOT NULL COMMENT '解锁条件，同 resources.download_cond',
  title        VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '未解锁时显示的标题',
  content_html MEDIUMTEXT   NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_hidden_post_seq (post_id, seq)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='隐藏内容块';

CREATE TABLE resource_links (
  id             BIGINT        NOT NULL AUTO_INCREMENT,
  post_id        BIGINT        NOT NULL,
  name           VARCHAR(64)   NOT NULL COMMENT '如：百度网盘、夸克网盘',
  url            VARCHAR(1024) NOT NULL,
  extract_code   VARCHAR(32)   NOT NULL DEFAULT '',
  unzip_password VARCHAR(64)   NOT NULL DEFAULT '',
  sort           INT           NOT NULL DEFAULT 0,
  status         VARCHAR(16)   NOT NULL DEFAULT 'ok' COMMENT 'ok / broken',
  created_at     DATETIME(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at     DATETIME(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_links_post (post_id, sort)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='下载地址';

CREATE TABLE link_reports (
  id         BIGINT       NOT NULL AUTO_INCREMENT,
  link_id    BIGINT       NOT NULL,
  post_id    BIGINT       NOT NULL,
  user_id    BIGINT       NOT NULL,
  reason     VARCHAR(255) NOT NULL DEFAULT '',
  status     VARCHAR(16)  NOT NULL DEFAULT 'pending' COMMENT 'pending / resolved / ignored',
  handled_at DATETIME(3)  NULL,
  created_at DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_reports_status (status, id),
  KEY idx_reports_link_user (link_id, user_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='下载地址失效反馈';

CREATE TABLE download_logs (
  id             BIGINT      NOT NULL AUTO_INCREMENT,
  user_id        BIGINT      NOT NULL DEFAULT 0 COMMENT '0 表示未登录',
  post_id        BIGINT      NOT NULL,
  link_id        BIGINT      NOT NULL,
  is_member_free TINYINT(1)  NOT NULL DEFAULT 0 COMMENT '靠会员身份免费下载，只有这类下载计入每日次数',
  ip             VARCHAR(45) NOT NULL DEFAULT '',
  created_at     DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_downloads_user (user_id, created_at),
  KEY idx_downloads_post (post_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='下载记录';

CREATE TABLE comments (
  id               BIGINT       NOT NULL AUTO_INCREMENT,
  post_id          BIGINT       NOT NULL,
  user_id          BIGINT       NOT NULL,
  parent_id        BIGINT       NULL COMMENT '所属的一级评论，NULL 表示本身是一级评论',
  reply_to_user_id BIGINT       NULL COMMENT '回复楼中楼时被回复的用户，用于显示 @昵称',
  content          TEXT         NOT NULL COMMENT '纯文本',
  status           VARCHAR(16)  NOT NULL COMMENT 'pending / approved / spam',
  ip               VARCHAR(45)  NOT NULL DEFAULT '',
  user_agent       VARCHAR(255) NOT NULL DEFAULT '',
  created_at       DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at       DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_comments_post (post_id, status, id),
  KEY idx_comments_user_post (user_id, post_id, status),
  KEY idx_comments_status (status, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='评论';

CREATE TABLE membership_levels (
  id                   BIGINT      NOT NULL AUTO_INCREMENT,
  name                 VARCHAR(32) NOT NULL,
  `rank`               INT         NOT NULL COMMENT '数字越大等级越高',
  discount_rate        INT         NOT NULL DEFAULT 100 COMMENT '80 表示 8 折，100 表示不打折',
  daily_download_limit INT         NOT NULL DEFAULT 0 COMMENT '每天靠会员身份免费下载的次数，0 表示不限',
  status               VARCHAR(16) NOT NULL DEFAULT 'active' COMMENT 'active / disabled',
  created_at           DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at           DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_levels_rank (`rank`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='会员等级';

CREATE TABLE user_memberships (
  user_id    BIGINT      NOT NULL,
  level_id   BIGINT      NOT NULL,
  expires_at DATETIME(3) NULL COMMENT 'NULL 表示永久',
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (user_id),
  KEY idx_memberships_level (level_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户当前的会员状态';

-- +goose Down
DROP TABLE user_memberships;
DROP TABLE membership_levels;
DROP TABLE comments;
DROP TABLE download_logs;
DROP TABLE link_reports;
DROP TABLE resource_links;
DROP TABLE post_hidden_blocks;
ALTER TABLE resources DROP COLUMN download_cond;
ALTER TABLE posts DROP COLUMN member_discount, DROP COLUMN member_free_rank, DROP COLUMN price;
