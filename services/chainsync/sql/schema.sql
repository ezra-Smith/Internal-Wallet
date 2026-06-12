-- ChainSync Service Database Schema

-- 创建数据库
CREATE DATABASE IF NOT EXISTS chainsync CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE chainsync;

-- 区块表
CREATE TABLE IF NOT EXISTS `blocks` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `chain` varchar(32) NOT NULL COMMENT '区块链类型：ethereum, bsc, tron',
  `block_number` bigint(20) unsigned NOT NULL COMMENT '区块号',
  `block_hash` varchar(128) NOT NULL COMMENT '区块哈希',
  `parent_hash` varchar(128) DEFAULT NULL COMMENT '父区块哈希',
  `timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '区块时间戳',
  `transaction_count` int(11) NOT NULL DEFAULT 0 COMMENT '交易数量',
  `miner` varchar(64) DEFAULT NULL COMMENT '矿工地址',
  `difficulty` varchar(64) DEFAULT NULL COMMENT '难度值',
  `gas_limit` bigint(20) unsigned DEFAULT 0 COMMENT 'Gas限制',
  `gas_used` bigint(20) unsigned DEFAULT 0 COMMENT 'Gas使用量',
  `size` bigint(20) unsigned DEFAULT 0 COMMENT '区块大小（字节）',
  `sync_status` varchar(32) NOT NULL DEFAULT 'synced' COMMENT '同步状态：pending, synced, failed',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_chain_block` (`chain`, `block_number`),
  KEY `idx_block_hash` (`block_hash`),
  KEY `idx_timestamp` (`timestamp`),
  KEY `idx_sync_status` (`sync_status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='区块信息表';

-- 交易表
CREATE TABLE IF NOT EXISTS `transactions` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `chain` varchar(32) NOT NULL COMMENT '区块链类型',
  `tx_hash` varchar(128) NOT NULL COMMENT '交易哈希',
  `block_number` bigint(20) unsigned NOT NULL COMMENT '区块号',
  `block_hash` varchar(128) NOT NULL COMMENT '区块哈希',
  `transaction_index` int(11) NOT NULL DEFAULT 0 COMMENT '交易在区块中的索引',
  `from_address` varchar(128) NOT NULL COMMENT '发送方地址',
  `to_address` varchar(128) DEFAULT NULL COMMENT '接收方地址',
  `value` varchar(64) NOT NULL DEFAULT '0' COMMENT '交易金额（最小单位）',
  `gas_price` varchar(64) DEFAULT NULL COMMENT 'Gas价格',
  `gas_used` varchar(64) DEFAULT 0 COMMENT 'Gas使用量',
  `gas_fee` varchar(64) DEFAULT NULL COMMENT 'Gas费用',
  `nonce` varchar(64) DEFAULT NULL COMMENT 'Nonce',
  `input_data` longtext COMMENT '输入数据',
  `confirmations` bigint(20) unsigned DEFAULT 0 COMMENT '确认数',
  `status` varchar(32) NOT NULL DEFAULT 'pending' COMMENT '交易状态：pending, confirmed, failed, reverted',
  `contract_address` varchar(128) DEFAULT NULL COMMENT '合约地址（合约创建时）',
  `error_message` text COMMENT '错误信息（失败时）',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_chain_tx` (`chain`, `tx_hash`),
  KEY `idx_from_address` (`from_address`),
  KEY `idx_to_address` (`to_address`),
  KEY `idx_block_number` (`block_number`),
  KEY `idx_status` (`status`),
  KEY `idx_confirmations` (`confirmations`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='交易信息表';

-- 交易日志表
CREATE TABLE IF NOT EXISTS `transaction_logs` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `chain` varchar(32) NOT NULL COMMENT '区块链类型',
  `transaction_hash` varchar(128) NOT NULL COMMENT '交易哈希',
  `log_index` int(11) NOT NULL COMMENT '日志索引',
  `address` varchar(128) NOT NULL COMMENT '合约地址',
  `topics` json DEFAULT NULL COMMENT '事件主题',
  `data` longtext COMMENT '事件数据',
  `block_number` bigint(20) unsigned NOT NULL COMMENT '区块号',
  `block_hash` varchar(128) NOT NULL COMMENT '区块哈希',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_chain_tx_log` (`chain`, `transaction_hash`, `log_index`),
  KEY `idx_address` (`address`),
  KEY `idx_block_number` (`block_number`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='交易日志表';

-- 地址监控表
CREATE TABLE IF NOT EXISTS `address_monitors` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `monitor_id` varchar(128) NOT NULL COMMENT '监控ID',
  `chain` varchar(32) NOT NULL COMMENT '区块链类型',
  `address` varchar(128) NOT NULL COMMENT '监控地址',
  `monitor_type` varchar(32) NOT NULL COMMENT '监控类型：balance, transaction, contract, all',
  `priority` varchar(16) NOT NULL DEFAULT 'normal' COMMENT '优先级：low, normal, high, critical',
  `tag` varchar(64) DEFAULT NULL COMMENT '标签',
  `webhook_url` varchar(512) DEFAULT NULL COMMENT '回调URL',
  `active` tinyint(1) NOT NULL DEFAULT 1 COMMENT '是否激活：1-激活，0-停用',
  `metadata` json DEFAULT NULL COMMENT '额外元数据',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `last_activity` timestamp NULL DEFAULT NULL COMMENT '最后活动时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_monitor_id` (`monitor_id`),
  KEY `idx_chain_address` (`chain`, `address`),
  KEY `idx_active` (`active`),
  KEY `idx_monitor_type` (`monitor_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='地址监控表';

-- 余额记录表
CREATE TABLE IF NOT EXISTS `balance_records` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `chain` varchar(32) NOT NULL COMMENT '区块链类型',
  `address` varchar(128) NOT NULL COMMENT '地址',
  `token_address` varchar(128) DEFAULT NULL COMMENT '代币合约地址，NULL表示原生代币',
  `token_symbol` varchar(16) DEFAULT NULL COMMENT '代币符号',
  `token_decimals` int(11) DEFAULT NULL COMMENT '代币精度',
  `balance` varchar(64) NOT NULL DEFAULT '0' COMMENT '余额（最小单位）',
  `formatted_balance` decimal(36,18) DEFAULT 0 COMMENT '格式化余额',
  `usd_value` decimal(20,8) DEFAULT 0 COMMENT 'USD价值',
  `block_number` bigint(20) unsigned DEFAULT NULL COMMENT '区块号',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `idx_address_token` (`address`, `token_address`),
  KEY `idx_chain` (`chain`),
  KEY `idx_block_number` (`block_number`),
  KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='余额记录表';

-- 同步任务表
CREATE TABLE IF NOT EXISTS `sync_tasks` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `sync_id` varchar(128) NOT NULL COMMENT '同步任务ID',
  `chain` varchar(32) NOT NULL COMMENT '区块链类型',
  `status` varchar(32) NOT NULL DEFAULT 'stopped' COMMENT '同步状态：stopped, syncing, caught_up, error, paused',
  `start_block` bigint(20) unsigned NOT NULL DEFAULT 0 COMMENT '起始区块号',
  `end_block` bigint(20) unsigned NOT NULL DEFAULT 0 COMMENT '结束区块号，0表示持续同步',
  `current_block` bigint(20) unsigned NOT NULL DEFAULT 0 COMMENT '当前区块号',
  `latest_block` bigint(20) unsigned NOT NULL DEFAULT 0 COMMENT '最新区块号',
  `blocks_per_second` decimal(10,2) DEFAULT 0 COMMENT '同步速度（块/秒）',
  `estimated_remaining_seconds` bigint(20) DEFAULT 0 COMMENT '预计剩余时间（秒）',
  `last_error` text COMMENT '最后错误信息',
  `start_time` timestamp NULL DEFAULT NULL COMMENT '开始时间',
  `end_time` timestamp NULL DEFAULT NULL COMMENT '结束时间',
  `last_update_time` timestamp NULL DEFAULT NULL COMMENT '最后更新时间',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_sync_id` (`sync_id`),
  KEY `idx_chain_status` (`chain`, `status`),
  KEY `idx_last_update` (`last_update_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='同步任务表';

-- 服务商状态表
CREATE TABLE IF NOT EXISTS `provider_status` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `provider_id` varchar(128) NOT NULL COMMENT '服务商ID',
  `provider_type` varchar(32) NOT NULL COMMENT '服务商类型',
  `chain` varchar(32) NOT NULL COMMENT '区块链类型',
  `name` varchar(64) NOT NULL COMMENT '服务商名称',
  `endpoint` varchar(512) NOT NULL COMMENT '端点URL',
  `healthy` tinyint(1) NOT NULL DEFAULT 1 COMMENT '是否健康：1-健康，0-不健康',
  `response_time_ms` bigint(20) DEFAULT 0 COMMENT '响应时间（毫秒）',
  `success_rate` int(11) DEFAULT 100 COMMENT '成功率（百分比）',
  `total_requests` bigint(20) DEFAULT 0 COMMENT '总请求数',
  `failed_requests` bigint(20) DEFAULT 0 COMMENT '失败请求数',
  `last_success_time` timestamp NULL DEFAULT NULL COMMENT '最后成功时间',
  `last_error_time` timestamp NULL DEFAULT NULL COMMENT '最后错误时间',
  `last_error` text COMMENT '最后错误信息',
  `is_primary` tinyint(1) NOT NULL DEFAULT 0 COMMENT '是否为主要服务商',
  `weight` decimal(5,2) DEFAULT 1.00 COMMENT '权重',
  `config` json DEFAULT NULL COMMENT '配置信息',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_provider_chain` (`provider_id`, `chain`),
  KEY `idx_chain_healthy` (`chain`, `healthy`),
  KEY `idx_provider_type` (`provider_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='服务商状态表';

-- 通知记录表
CREATE TABLE IF NOT EXISTS `notifications` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `monitor_id` varchar(128) DEFAULT NULL COMMENT '监控ID',
  `chain` varchar(32) NOT NULL COMMENT '区块链类型',
  `address` varchar(128) DEFAULT NULL COMMENT '地址',
  `notification_type` varchar(32) NOT NULL COMMENT '通知类型：balance_change, transaction, contract_event',
  `webhook_url` varchar(512) DEFAULT NULL COMMENT '回调URL',
  `payload` json NOT NULL COMMENT '通知载荷',
  `status` varchar(32) NOT NULL DEFAULT 'pending' COMMENT '状态：pending, sent, failed',
  `http_status` int(11) DEFAULT NULL COMMENT 'HTTP状态码',
  `response` text COMMENT '响应内容',
  `error_message` text COMMENT '错误信息',
  `retry_count` int(11) NOT NULL DEFAULT 0 COMMENT '重试次数',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `sent_at` timestamp NULL DEFAULT NULL COMMENT '发送时间',
  PRIMARY KEY (`id`),
  KEY `idx_monitor_id` (`monitor_id`),
  KEY `idx_status` (`status`),
  KEY `idx_notification_type` (`notification_type`),
  KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='通知记录表';

-- 添加索引优化查询性能
ALTER TABLE `transactions` ADD INDEX `idx_created_at` (`created_at`);
ALTER TABLE `balance_records` ADD INDEX `idx_address_chain` (`address`, `chain`);
ALTER TABLE `sync_tasks` ADD INDEX `idx_status` (`status`);
ALTER TABLE `provider_status` ADD INDEX `idx_updated_at` (`updated_at`);

-- 插入一些初始数据
INSERT INTO `sync_tasks` (`sync_id`, `chain`, `status`, `start_block`, `current_block`) VALUES
('ethereum_init', 'ethereum', 'stopped', 18800000, 18800000),
('bsc_init', 'bsc', 'stopped', 35000000, 35000000),
('tron_init', 'tron', 'stopped', 50000000, 50000000)
ON DUPLICATE KEY UPDATE `sync_id` = `sync_id`;