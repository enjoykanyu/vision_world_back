-- 直播服务数据库初始化脚本

-- 创建数据库
CREATE DATABASE IF NOT EXISTS live_service DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

USE live_service;

-- 直播流表
CREATE TABLE IF NOT EXISTS live_streams (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
    title VARCHAR(255) NOT NULL COMMENT '直播标题',
    description TEXT COMMENT '直播描述',
    category_id INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '分类ID',
    status ENUM('preparing', 'streaming', 'paused', 'ended', 'banned') NOT NULL DEFAULT 'preparing' COMMENT '直播状态',
    stream_url VARCHAR(512) COMMENT '推流地址',
    playback_url VARCHAR(512) COMMENT '回放地址',
    cover_image VARCHAR(512) COMMENT '封面图片',
    viewer_count INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '观看人数',
    like_count INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '点赞数',
    gift_count INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '礼物数',
    gift_value BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '礼物价值',
    duration INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '直播时长(秒)',
    start_time BIGINT NOT NULL DEFAULT 0 COMMENT '开始时间',
    end_time BIGINT NOT NULL DEFAULT 0 COMMENT '结束时间',
    created_at BIGINT NOT NULL DEFAULT 0 COMMENT '创建时间',
    updated_at BIGINT NOT NULL DEFAULT 0 COMMENT '更新时间',
    INDEX idx_user_id (user_id),
    INDEX idx_status (status),
    INDEX idx_category_id (category_id),
    INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='直播流表';

-- 直播间表
CREATE TABLE IF NOT EXISTS live_rooms (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    stream_id BIGINT UNSIGNED NOT NULL COMMENT '直播流ID',
    title VARCHAR(255) NOT NULL COMMENT '直播间标题',
    description TEXT COMMENT '直播间描述',
    category_id INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '分类ID',
    status ENUM('active', 'inactive', 'banned') NOT NULL DEFAULT 'active' COMMENT '直播间状态',
    viewer_count INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '当前观看人数',
    max_viewer_count INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '最大观看人数',
    like_count INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '点赞数',
    gift_count INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '礼物数',
    gift_value BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '礼物价值',
    chat_count INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '聊天消息数',
    created_at BIGINT NOT NULL DEFAULT 0 COMMENT '创建时间',
    updated_at BIGINT NOT NULL DEFAULT 0 COMMENT '更新时间',
    INDEX idx_stream_id (stream_id),
    INDEX idx_status (status),
    INDEX idx_category_id (category_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='直播间表';

-- 直播观看者表
CREATE TABLE IF NOT EXISTS live_viewers (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    stream_id BIGINT UNSIGNED NOT NULL COMMENT '直播流ID',
    user_id BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
    user_name VARCHAR(100) NOT NULL COMMENT '用户名',
    user_avatar VARCHAR(512) COMMENT '用户头像',
    join_time BIGINT NOT NULL DEFAULT 0 COMMENT '加入时间',
    leave_time BIGINT NOT NULL DEFAULT 0 COMMENT '离开时间',
    duration INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '观看时长(秒)',
    is_muted BOOLEAN NOT NULL DEFAULT FALSE COMMENT '是否被禁言',
    created_at BIGINT NOT NULL DEFAULT 0 COMMENT '创建时间',
    updated_at BIGINT NOT NULL DEFAULT 0 COMMENT '更新时间',
    UNIQUE KEY uk_stream_user (stream_id, user_id),
    INDEX idx_stream_id (stream_id),
    INDEX idx_user_id (user_id),
    INDEX idx_join_time (join_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='直播观看者表';

-- 直播聊天消息表
CREATE TABLE IF NOT EXISTS live_chats (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    stream_id BIGINT UNSIGNED NOT NULL COMMENT '直播流ID',
    user_id BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
    user_name VARCHAR(100) NOT NULL COMMENT '用户名',
    user_avatar VARCHAR(512) COMMENT '用户头像',
    content TEXT NOT NULL COMMENT '消息内容',
    content_type VARCHAR(50) NOT NULL DEFAULT 'text' COMMENT '内容类型(text, image, emoji)',
    is_system BOOLEAN NOT NULL DEFAULT FALSE COMMENT '是否系统消息',
    is_deleted BOOLEAN NOT NULL DEFAULT FALSE COMMENT '是否已删除',
    created_at BIGINT NOT NULL DEFAULT 0 COMMENT '创建时间',
    updated_at BIGINT NOT NULL DEFAULT 0 COMMENT '更新时间',
    INDEX idx_stream_id (stream_id),
    INDEX idx_user_id (user_id),
    INDEX idx_created_at (created_at),
    INDEX idx_is_deleted (is_deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='直播聊天消息表';

-- 直播礼物表
CREATE TABLE IF NOT EXISTS live_gifts (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    stream_id BIGINT UNSIGNED NOT NULL COMMENT '直播流ID',
    user_id BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
    user_name VARCHAR(100) NOT NULL COMMENT '用户名',
    user_avatar VARCHAR(512) COMMENT '用户头像',
    gift_id INT UNSIGNED NOT NULL COMMENT '礼物ID',
    gift_name VARCHAR(100) NOT NULL COMMENT '礼物名称',
    gift_icon VARCHAR(512) COMMENT '礼物图标',
    gift_price BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '礼物价格(金币)',
    gift_count INT UNSIGNED NOT NULL DEFAULT 1 COMMENT '礼物数量',
    total_value BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '总价值',
    message TEXT COMMENT '留言消息',
    effect_type VARCHAR(50) COMMENT '特效类型',
    created_at BIGINT NOT NULL DEFAULT 0 COMMENT '创建时间',
    INDEX idx_stream_id (stream_id),
    INDEX idx_user_id (user_id),
    INDEX idx_gift_id (gift_id),
    INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='直播礼物表';

-- 礼物配置表
CREATE TABLE IF NOT EXISTS gift_configs (
    id INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL COMMENT '礼物名称',
    icon VARCHAR(512) COMMENT '礼物图标',
    price BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '价格(金币)',
    coin_price BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '金币价格',
    category VARCHAR(50) NOT NULL DEFAULT 'normal' COMMENT '分类',
    level INT UNSIGNED NOT NULL DEFAULT 1 COMMENT '等级',
    effect_type VARCHAR(50) COMMENT '特效类型',
    effect_value VARCHAR(512) COMMENT '特效值',
    description TEXT COMMENT '描述',
    is_active BOOLEAN NOT NULL DEFAULT TRUE COMMENT '是否启用',
    sort_order INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '排序',
    created_at BIGINT NOT NULL DEFAULT 0 COMMENT '创建时间',
    updated_at BIGINT NOT NULL DEFAULT 0 COMMENT '更新时间',
    INDEX idx_category (category),
    INDEX idx_level (level),
    INDEX idx_is_active (is_active),
    INDEX idx_sort_order (sort_order)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='礼物配置表';

-- 直播分类表
CREATE TABLE IF NOT EXISTS live_categories (
    id INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL COMMENT '分类名称',
    icon VARCHAR(512) COMMENT '分类图标',
    sort_order INT NOT NULL DEFAULT 0 COMMENT '排序',
    is_active BOOLEAN NOT NULL DEFAULT TRUE COMMENT '是否启用',
    created_at BIGINT NOT NULL DEFAULT 0 COMMENT '创建时间',
    updated_at BIGINT NOT NULL DEFAULT 0 COMMENT '更新时间',
    INDEX idx_sort_order (sort_order),
    INDEX idx_is_active (is_active)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='直播分类表';

-- 用户直播统计表
CREATE TABLE IF NOT EXISTS user_live_stats (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
    total_streams INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '总直播数',
    total_duration INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '总直播时长(秒)',
    total_viewers BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '总观看人数',
    max_viewers INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '最大观看人数',
    total_gifts INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '总礼物数',
    total_gift_value BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '总礼物价值',
    total_likes INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '总点赞数',
    follower_count INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '粉丝数',
    level INT UNSIGNED NOT NULL DEFAULT 1 COMMENT '等级',
    experience BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '经验值',
    created_at BIGINT NOT NULL DEFAULT 0 COMMENT '创建时间',
    updated_at BIGINT NOT NULL DEFAULT 0 COMMENT '更新时间',
    UNIQUE KEY uk_user_id (user_id),
    INDEX idx_level (level),
    INDEX idx_experience (experience)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户直播统计表';

-- 直播统计表
CREATE TABLE IF NOT EXISTS live_stats (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    stream_id BIGINT UNSIGNED NOT NULL COMMENT '直播流ID',
    total_viewers BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '总观看人数',
    current_viewers INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '当前观看人数',
    max_viewers INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '最大观看人数',
    like_count INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '点赞数',
    gift_count INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '礼物数',
    comment_count INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '评论数',
    share_count INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '分享数',
    duration INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '直播时长(秒)',
    gift_value BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '礼物价值',
    created_at BIGINT NOT NULL DEFAULT 0 COMMENT '创建时间',
    updated_at BIGINT NOT NULL DEFAULT 0 COMMENT '更新时间',
    UNIQUE KEY uk_stream_id (stream_id),
    INDEX idx_current_viewers (current_viewers),
    INDEX idx_gift_value (gift_value)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='直播统计表';

-- 直播回放表
CREATE TABLE IF NOT EXISTS live_playbacks (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    stream_id BIGINT UNSIGNED NOT NULL COMMENT '直播流ID',
    playback_url VARCHAR(512) NOT NULL COMMENT '回放地址',
    duration INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '时长(秒)',
    file_size BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '文件大小(字节)',
    format VARCHAR(50) NOT NULL DEFAULT 'mp4' COMMENT '格式',
    quality VARCHAR(50) NOT NULL DEFAULT '1080p' COMMENT '清晰度',
    status ENUM('processing', 'completed', 'failed') NOT NULL DEFAULT 'processing' COMMENT '状态',
    created_at BIGINT NOT NULL DEFAULT 0 COMMENT '创建时间',
    updated_at BIGINT NOT NULL DEFAULT 0 COMMENT '更新时间',
    UNIQUE KEY uk_stream_id (stream_id),
    INDEX idx_status (status),
    INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='直播回放表';

-- 用户禁言表
CREATE TABLE IF NOT EXISTS user_mutes (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    stream_id BIGINT UNSIGNED NOT NULL COMMENT '直播流ID',
    user_id BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
    start_time BIGINT NOT NULL DEFAULT 0 COMMENT '开始时间',
    duration INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '禁言时长(秒)',
    end_time BIGINT NOT NULL DEFAULT 0 COMMENT '结束时间',
    reason TEXT COMMENT '禁言原因',
    muted_by BIGINT UNSIGNED NOT NULL COMMENT '操作者ID',
    is_active BOOLEAN NOT NULL DEFAULT TRUE COMMENT '是否有效',
    created_at BIGINT NOT NULL DEFAULT 0 COMMENT '创建时间',
    updated_at BIGINT NOT NULL DEFAULT 0 COMMENT '更新时间',
    INDEX idx_stream_user (stream_id, user_id),
    INDEX idx_end_time (end_time),
    INDEX idx_is_active (is_active)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户禁言表';

-- 敏感词表
CREATE TABLE IF NOT EXISTS banned_words (
    id INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    word VARCHAR(100) NOT NULL COMMENT '敏感词',
    category VARCHAR(50) NOT NULL DEFAULT 'general' COMMENT '分类',
    level INT UNSIGNED NOT NULL DEFAULT 1 COMMENT '等级',
    is_active BOOLEAN NOT NULL DEFAULT TRUE COMMENT '是否启用',
    created_at BIGINT NOT NULL DEFAULT 0 COMMENT '创建时间',
    updated_at BIGINT NOT NULL DEFAULT 0 COMMENT '更新时间',
    UNIQUE KEY uk_word (word),
    INDEX idx_category (category),
    INDEX idx_level (level),
    INDEX idx_is_active (is_active)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='敏感词表';

-- 插入默认数据
INSERT INTO live_categories (name, icon, sort_order, is_active, created_at, updated_at) VALUES
('游戏', '🎮', 1, true, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('娱乐', '🎭', 2, true, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('音乐', '🎵', 3, true, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('舞蹈', '💃', 4, true, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('美食', '🍔', 5, true, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('旅行', '✈️', 6, true, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('学习', '📚', 7, true, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('运动', '⚽', 8, true, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('聊天', '💬', 9, true, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('其他', '🌟', 10, true, UNIX_TIMESTAMP(), UNIX_TIMESTAMP());

INSERT INTO gift_configs (name, icon, price, coin_price, category, level, effect_type, effect_value, description, is_active, sort_order, created_at, updated_at) VALUES
('玫瑰', '🌹', 10, 10, 'normal', 1, 'emoji', '🌹', '浪漫的玫瑰', true, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('爱心', '❤️', 20, 20, 'normal', 1, 'emoji', '❤️', '真挚的爱心', true, 2, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('蛋糕', '🎂', 50, 50, 'normal', 2, 'emoji', '🎂', '甜美的蛋糕', true, 3, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('钻石', '💎', 100, 100, 'rare', 3, 'sparkle', '💎✨', '闪耀的钻石', true, 4, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('火箭', '🚀', 500, 500, 'legendary', 4, 'animation', 'rocket', '冲天的火箭', true, 5, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('皇冠', '👑', 1000, 1000, 'legendary', 5, 'crown', '👑', '尊贵的皇冠', true, 6, UNIX_TIMESTAMP(), UNIX_TIMESTAMP());

INSERT INTO banned_words (word, category, level, is_active, created_at, updated_at) VALUES
('广告', 'spam', 1, true, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('微信', 'contact', 2, true, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('QQ', 'contact', 2, true, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('电话', 'contact', 2, true, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('诈骗', 'fraud', 3, true, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('赌博', 'illegal', 3, true, UNIX_TIMESTAMP(), UNIX_TIMESTAMP());

-- 创建事件调度器，定期清理过期数据
CREATE EVENT IF NOT EXISTS clean_expired_mutes
ON SCHEDULE EVERY 1 HOUR
DO
  DELETE FROM user_mutes WHERE end_time < UNIX_TIMESTAMP() AND is_active = true;

CREATE EVENT IF NOT EXISTS clean_old_chats
ON SCHEDULE EVERY 1 DAY
DO
  DELETE FROM live_chats WHERE created_at < UNIX_TIMESTAMP() - 7*24*60*60 AND is_deleted = true;