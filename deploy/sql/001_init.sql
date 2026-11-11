-- 001_init.sql —— 基础表结构
--
-- 执行方式（库和账号已单独创建过，这里只建表）：
--   kubectl exec -i -n infra deploy/mysql -- \
--     mysql -uyulin -p"$MYSQL_PASSWORD" yulin < deploy/sql/001_init.sql
--
-- 设计说明：
--   * 引擎 InnoDB、字符集 utf8mb4：校友姓名与备注可能出现生僻字和 emoji，
--     utf8（3 字节）存不下，必须 utf8mb4。
--   * 索引 (graduation_year, class_name) 对应"按届 + 按班"的列表检索，
--     这也是校友录最高频的查询路径。
--   * 软删除用 deleted_at 而非物理删除，历史档案不该真的消失。

CREATE TABLE IF NOT EXISTS `alumni` (
  `id`              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
  `name`            VARCHAR(64)     NOT NULL                COMMENT '姓名',
  `gender`          TINYINT         NOT NULL DEFAULT 0      COMMENT '0未知 1男 2女',
  `graduation_year` INT             NOT NULL                COMMENT '毕业年份，如 2015',
  `class_name`      VARCHAR(32)     NOT NULL DEFAULT ''     COMMENT '班级，如 高三(2)班',
  `city`            VARCHAR(64)     NOT NULL DEFAULT ''     COMMENT '当前所在城市',
  `industry`        VARCHAR(64)     NOT NULL DEFAULT ''     COMMENT '所属行业',
  `company`         VARCHAR(128)    NOT NULL DEFAULT ''     COMMENT '公司/单位',
  `position`        VARCHAR(64)     NOT NULL DEFAULT ''     COMMENT '职位',
  `phone`           VARCHAR(32)     NOT NULL DEFAULT ''     COMMENT '手机号（敏感，需按可见性输出）',
  `email`           VARCHAR(128)    NOT NULL DEFAULT ''     COMMENT '邮箱（敏感）',
  `remark`          VARCHAR(255)    NOT NULL DEFAULT ''     COMMENT '备注',
  `created_at`      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at`      DATETIME(3)     NULL     DEFAULT NULL   COMMENT '软删除时间',
  PRIMARY KEY (`id`),
  KEY `idx_name` (`name`),
  KEY `idx_year_class` (`graduation_year`, `class_name`),
  KEY `idx_city` (`city`),
  KEY `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='校友档案';
