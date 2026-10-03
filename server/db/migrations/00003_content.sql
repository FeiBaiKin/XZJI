-- +goose Up
CREATE TABLE media (
  id         BIGINT       NOT NULL AUTO_INCREMENT,
  user_id    BIGINT       NOT NULL,
  path       VARCHAR(255) NOT NULL COMMENT '相对上传目录的路径，对外地址 = media.base_url + path',
  mime       VARCHAR(32)  NOT NULL,
  size       BIGINT       NOT NULL,
  width      INT          NOT NULL,
  height     INT          NOT NULL,
  color      CHAR(7)      NOT NULL DEFAULT '' COMMENT '主色，如 #2F6BFF，用于给软件图标配底色',
  variants   JSON         NULL COMMENT '各尺寸缩略图路径',
  created_at DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_media_path (path),
  KEY idx_media_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='上传文件';

CREATE TABLE categories (
  id              BIGINT       NOT NULL AUTO_INCREMENT,
  parent_id       BIGINT       NULL,
  name            VARCHAR(64)  NOT NULL,
  slug            VARCHAR(64)  NOT NULL,
  icon            VARCHAR(32)  NOT NULL DEFAULT '' COMMENT '图标名，对应前台 icons.svg',
  color           VARCHAR(16)  NOT NULL DEFAULT '' COMMENT '渐变色：blue / red / orange / green / purple / gold / vip',
  description     VARCHAR(255) NOT NULL DEFAULT '',
  sort            INT          NOT NULL DEFAULT 0 COMMENT '越小越靠前',
  seo_title       VARCHAR(255) NOT NULL DEFAULT '',
  seo_description VARCHAR(500) NOT NULL DEFAULT '',
  created_at      DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at      DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_categories_slug (slug),
  KEY idx_categories_parent (parent_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='分类（文章和资源共用）';

CREATE TABLE tags (
  id         BIGINT      NOT NULL AUTO_INCREMENT,
  name       VARCHAR(64) NOT NULL,
  slug       VARCHAR(64) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_tags_name (name),
  UNIQUE KEY uk_tags_slug (slug)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='标签';

CREATE TABLE posts (
  id              BIGINT       NOT NULL AUTO_INCREMENT,
  type            VARCHAR(16)  NOT NULL COMMENT 'article / resource',
  title           VARCHAR(255) NOT NULL,
  summary         VARCHAR(500) NOT NULL DEFAULT '',
  summary_auto    TINYINT(1)   NOT NULL DEFAULT 0 COMMENT '摘要由正文自动截取，修改正文时重新生成',
  content_html    MEDIUMTEXT   NOT NULL COMMENT '过滤后的正文，图片地址以 @media/ 开头，输出时替换为 media.base_url',
  toc             JSON         NULL COMMENT '正文目录，保存时从标题提取',
  cover_media_id  BIGINT       NULL,
  category_id     BIGINT       NOT NULL,
  author_id       BIGINT       NOT NULL,
  status          VARCHAR(16)  NOT NULL COMMENT 'draft / published / trash',
  pinned          TINYINT(1)   NOT NULL DEFAULT 0 COMMENT '置顶',
  view_count      INT          NOT NULL DEFAULT 0,
  comment_count   INT          NOT NULL DEFAULT 0,
  seo_title       VARCHAR(255) NOT NULL DEFAULT '',
  seo_keywords    VARCHAR(255) NOT NULL DEFAULT '',
  seo_description VARCHAR(500) NOT NULL DEFAULT '',
  published_at    DATETIME(3)  NULL COMMENT '发布时间，晚于当前时间表示定时发布',
  created_at      DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at      DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_posts_status_published (status, published_at),
  KEY idx_posts_category (category_id, status, published_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='内容（文章和资源）';

CREATE TABLE post_tags (
  post_id BIGINT NOT NULL,
  tag_id  BIGINT NOT NULL,
  sort    INT    NOT NULL DEFAULT 0 COMMENT '标签在这篇内容里的顺序',
  PRIMARY KEY (post_id, tag_id),
  KEY idx_post_tags_tag (tag_id, post_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='内容与标签';

CREATE TABLE resources (
  post_id       BIGINT      NOT NULL,
  platform      VARCHAR(32) NOT NULL DEFAULT '' COMMENT 'Android / Windows / macOS / 源码 等',
  version       VARCHAR(64) NOT NULL DEFAULT '',
  file_size     VARCHAR(32) NOT NULL DEFAULT '',
  icon_media_id BIGINT      NULL COMMENT '方形软件图标，没有封面图时显示',
  attrs         JSON        NULL COMMENT '自定义属性 [{name, value}]',
  changelog     JSON        NULL COMMENT '更新日志 [{version, date, note}]，新的在前',
  PRIMARY KEY (post_id),
  KEY idx_resources_platform (platform)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='资源扩展信息';

-- +goose Down
DROP TABLE resources;
DROP TABLE post_tags;
DROP TABLE posts;
DROP TABLE tags;
DROP TABLE categories;
DROP TABLE media;
