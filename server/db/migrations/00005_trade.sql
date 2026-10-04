-- +goose Up
ALTER TABLE membership_levels
  ADD COLUMN description VARCHAR(64) NOT NULL DEFAULT '' COMMENT '一句话介绍，如：适合偶尔下载' AFTER name,
  ADD COLUMN perks       JSON        NULL COMMENT '自定义权益说明 ["失效链接优先补", ...]，前台显示在自动生成的权益之后' AFTER daily_download_limit;

CREATE TABLE membership_plans (
  id             BIGINT      NOT NULL AUTO_INCREMENT,
  level_id       BIGINT      NOT NULL,
  name           VARCHAR(32) NOT NULL COMMENT '月卡 / 年卡 / 永久',
  duration_days  INT         NOT NULL COMMENT '0 表示永久',
  price          BIGINT      NOT NULL COMMENT '售价（分）',
  original_price BIGINT      NOT NULL DEFAULT 0 COMMENT '划线价（分），0 表示不显示',
  badge          VARCHAR(16) NOT NULL DEFAULT '' COMMENT '角标，如：最多人选',
  status         VARCHAR(16) NOT NULL DEFAULT 'active' COMMENT 'active / disabled',
  sort           INT         NOT NULL DEFAULT 0,
  created_at     DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at     DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_plans_level (level_id, sort)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='会员套餐';

CREATE TABLE orders (
  id              BIGINT       NOT NULL AUTO_INCREMENT,
  order_no        VARCHAR(32)  NOT NULL COMMENT '对外展示的订单号：站点时区的时间戳 + 随机数',
  user_id         BIGINT       NOT NULL,
  type            VARCHAR(16)  NOT NULL COMMENT 'post / membership / recharge',
  target_id       BIGINT       NOT NULL DEFAULT 0 COMMENT '内容 ID 或套餐 ID，充值为 0',
  title           VARCHAR(255) NOT NULL,
  snapshot        JSON         NOT NULL COMMENT '下单时的商品快照与支付后的发放结果',
  amount          BIGINT       NOT NULL COMMENT '原价（分）',
  discount_amount BIGINT       NOT NULL DEFAULT 0 COMMENT '会员折扣或升级抵扣（分）',
  pay_amount      BIGINT       NOT NULL COMMENT '应付金额（分）',
  pay_method      VARCHAR(16)  NOT NULL DEFAULT '' COMMENT 'wechat / alipay / balance，空表示还没发起支付',
  status          VARCHAR(16)  NOT NULL DEFAULT 'pending' COMMENT 'pending / paid / closed / refunded',
  expires_at      DATETIME(3)  NOT NULL,
  paid_at         DATETIME(3)  NULL,
  refund_method   VARCHAR(16)  NOT NULL DEFAULT '' COMMENT 'balance 退回余额 / original 原路退回（只记录）',
  refund_remark   VARCHAR(255) NOT NULL DEFAULT '',
  refunded_at     DATETIME(3)  NULL,
  created_at      DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at      DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_orders_no (order_no),
  KEY idx_orders_user (user_id, status, id),
  KEY idx_orders_status (status, expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='订单';

CREATE TABLE payments (
  id         BIGINT      NOT NULL AUTO_INCREMENT,
  order_id   BIGINT      NOT NULL,
  channel    VARCHAR(16) NOT NULL COMMENT 'wechat / alipay',
  trade_no   VARCHAR(64) NOT NULL COMMENT '支付渠道的交易号',
  amount     BIGINT      NOT NULL,
  status     VARCHAR(16) NOT NULL DEFAULT 'success' COMMENT 'success 已入账 / duplicate 订单已由其他交易支付，需人工退款',
  raw_notify TEXT        NOT NULL COMMENT '回调或查询的原文，排查问题用',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_payments_trade (channel, trade_no),
  KEY idx_payments_order (order_id),
  KEY idx_payments_status (status, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='第三方支付记录';

CREATE TABLE user_purchases (
  user_id    BIGINT      NOT NULL,
  post_id    BIGINT      NOT NULL,
  order_id   BIGINT      NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (user_id, post_id),
  KEY idx_purchases_user (user_id, created_at),
  KEY idx_purchases_order (order_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='已购内容';

CREATE TABLE wallets (
  user_id    BIGINT      NOT NULL,
  balance    BIGINT      NOT NULL DEFAULT 0 COMMENT '余额（分），只能通过写流水变动',
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='钱包';

CREATE TABLE wallet_ledger (
  id            BIGINT       NOT NULL AUTO_INCREMENT,
  user_id       BIGINT       NOT NULL,
  account       VARCHAR(16)  NOT NULL DEFAULT 'balance' COMMENT 'balance；第二阶段增加 commission',
  amount        BIGINT       NOT NULL COMMENT '正数入账，负数出账',
  balance_after BIGINT       NOT NULL,
  biz_type      VARCHAR(32)  NOT NULL COMMENT 'recharge / purchase / refund / admin_adjust',
  biz_id        BIGINT       NOT NULL DEFAULT 0 COMMENT '订单 ID；后台调整为管理员 ID',
  remark        VARCHAR(255) NOT NULL DEFAULT '',
  created_at    DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_ledger_user (user_id, account, id),
  KEY idx_ledger_biz (biz_type, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='余额流水，只增不改';

-- +goose Down
DROP TABLE wallet_ledger;
DROP TABLE wallets;
DROP TABLE user_purchases;
DROP TABLE payments;
DROP TABLE orders;
DROP TABLE membership_plans;
ALTER TABLE membership_levels DROP COLUMN perks, DROP COLUMN description;
