# 用户服务 Model 设计

## 概述

用户服务的 model 层采用 GORM + Redis 的架构，支持用户基本信息、关注关系、统计信息等核心功能。

## 数据模型

### 1. User (用户表)
- **表名**: `vw_users`
- **功能**: 存储用户基本信息
- **字段**:
  - `id`: 用户ID (主键)
  - `username`: 用户名 (唯一索引)
  - `phone`: 手机号
  - `email`: 邮箱
  - `password`: 密码哈希
  - `avatar`: 头像URL
  - `signature`: 个人签名
  - `follow_count`: 关注数量
  - `follower_count`: 粉丝数量
  - `total_favorited`: 获赞总数
  - `work_count`: 作品数量
  - `favorite_count`: 喜欢作品数量
  - `status`: 用户状态 (active/inactive/banned)
  - `created_at`: 创建时间
  - `updated_at`: 更新时间

### 2. UserFollow (关注关系表)
- **表名**: `vw_user_follows`
- **功能**: 存储用户之间的关注关系
- **字段**:
  - `id`: 关系ID (主键)
  - `user_id`: 关注者ID
  - `follow_user_id`: 被关注者ID
  - `status`: 关注状态 (following/unfollowed)
  - `created_at`: 创建时间
  - `updated_at`: 更新时间
- **索引**: 
  - 联合唯一索引: (user_id, follow_user_id)
  - 普通索引: user_id, follow_user_id, status

### 3. UserStats (用户统计表)
- **表名**: `vw_user_stats`
- **功能**: 存储用户的详细统计数据
- **字段**:
  - `id`: 统计ID (主键)
  - `user_id`: 用户ID (唯一)
  - `follow_count`: 关注数
  - `follower_count`: 粉丝数
  - `total_favorited`: 获赞数
  - `work_count`: 作品数
  - `favorite_count`: 喜欢数
  - `created_at`: 创建时间
  - `updated_at`: 更新时间

### 4. UserStatsDaily (用户每日统计表)
- **表名**: `vw_user_stats_daily`
- **功能**: 存储用户的每日统计数据，用于趋势分析
- **字段**:
  - `id`: 统计ID (主键)
  - `user_id`: 用户ID
  - `date`: 统计日期
  - `follow_count`: 当日关注数
  - `follower_count`: 当日粉丝数
  - `total_favorited`: 当日获赞数
  - `work_count`: 当日作品数
  - `favorite_count`: 当日喜欢数
  - `created_at`: 创建时间
  - `updated_at`: 更新时间
- **索引**: 联合索引 (user_id, date)

## 缓存设计

### Redis 键前缀
- `user:`: 用户基本信息缓存
- `user:stats:`: 用户统计信息缓存
- `user:follow:`: 用户关注关系缓存

### 缓存策略
- **用户信息缓存**: 15分钟过期
- **用户统计缓存**: 5分钟过期
- **关注关系缓存**: 10分钟过期

## 使用示例

### 1. 创建用户
```go
user := &model.User{
    Username: "testuser",
    Phone:    "13800138000",
    Password: "hashed_password",
    Avatar:   "https://example.com/avatar.jpg",
    Status:   model.UserStatusActive,
}
err := db.Create(user).Error
```

### 2. 获取用户信息（带缓存）
```go
// 从缓存获取
cacheKey := model.GetUserCacheKey(userID)
if cachedData, err := redis.Get(ctx, cacheKey).Result(); err == nil {
    var userCache model.UserCache
    if err := userCache.FromJSON([]byte(cachedData)); err == nil {
        return userCache.ToProto()
    }
}

// 缓存未命中，从数据库获取
var user model.User
if err := db.Where("id = ? AND status = ?", userID, model.UserStatusActive).First(&user).Error; err != nil {
    return nil, err
}

// 缓存用户信息
userCache := user.ToCache()
if cacheData, err := userCache.ToJSON(); err == nil {
    redis.Set(ctx, cacheKey, cacheData, 15*time.Minute)
}
```

### 3. 关注用户
```go
follow := &model.UserFollow{
    UserID:       userID,
    FollowUserID: followUserID,
    Status:       model.FollowStatusFollowing,
}
err := db.Create(follow).Error
```

### 4. 更新用户统计
```go
// 使用事务更新用户统计信息
err := db.Transaction(func(tx *gorm.DB) error {
    // 更新用户表中的统计字段
    if err := tx.Model(&model.User{}).
        Where("id = ?", userID).
        Update("follower_count", gorm.Expr("follower_count + ?", 1)).Error; err != nil {
        return err
    }
    
    // 更新统计表
    if err := tx.Model(&model.UserStats{}).
        Where("user_id = ?", userID).
        Update("follower_count", gorm.Expr("follower_count + ?", 1)).Error; err != nil {
        return err
    }
    
    return nil
})
```

## 数据库初始化

运行数据库初始化命令：
```bash
cd /Users/kanyu/Desktop/project/kanyu_server/new_project/project/vision_world_back/service/user_service
go run cmd/init_db/main.go
```

## 设计特点

1. **冗余存储**: 用户表中冗余存储关注/粉丝数量，提高查询性能
2. **缓存优化**: 使用 Redis 缓存热点数据，减少数据库压力
3. **异步更新**: 统计信息更新可以异步处理，提高响应速度
4. **分表设计**: 每日统计数据单独成表，便于数据分析和清理
5. **索引优化**: 关键字段都建立了合适的索引
6. **软删除**: 使用状态字段实现软删除，保留数据完整性