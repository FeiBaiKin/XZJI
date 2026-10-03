-- +goose Up
CREATE TABLE settings (
  `key`      VARCHAR(64) NOT NULL,
  `value`    JSON        NOT NULL,
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='站点设置';

INSERT INTO settings (`key`, `value`) VALUES ('site.name', '"XZJI"');

-- +goose Down
DROP TABLE settings;
