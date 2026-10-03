-- 生产环境建库建账号。执行前把密码换成强密码，并写入 server/config.yaml 的 db.dsn
CREATE DATABASE IF NOT EXISTS xzji CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS 'xzji'@'localhost' IDENTIFIED BY '请替换为强密码';
GRANT ALL PRIVILEGES ON xzji.* TO 'xzji'@'localhost';
FLUSH PRIVILEGES;
