# Live Service 安全文档

## 概述

本文档描述了 Live Service 的安全架构、安全策略、认证授权机制、数据保护和合规要求。包括网络安全、应用安全、数据安全和运维安全。

## 安全架构

### 安全层次

```
┌─────────────────────────────────────────────────────────────┐
│                    应用层安全                                │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐            │
│  │   认证授权   │ │   输入验证   │ │   审计日志   │            │
│  └─────────────┘ └─────────────┘ └─────────────┘            │
├─────────────────────────────────────────────────────────────┤
│                    网络层安全                                │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐            │
│  │    HTTPS    │ │    WAF      │ │   DDoS防护  │            │
│  └─────────────┘ └─────────────┘ └─────────────┘            │
├─────────────────────────────────────────────────────────────┤
│                    系统层安全                                │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐            │
│  │   容器安全   │ │   主机安全   │ │   网络安全   │            │
│  └─────────────┘ └─────────────┘ └─────────────┘            │
├─────────────────────────────────────────────────────────────┤
│                    数据层安全                                │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐            │
│  │   数据加密   │ │   访问控制   │ │   备份恢复   │            │
│  └─────────────┘ └─────────────┘ └─────────────┘            │
└─────────────────────────────────────────────────────────────┘
```

### 安全组件

- **身份认证**: JWT、OAuth2.0、API Key
- **访问控制**: RBAC、ABAC、权限管理
- **数据保护**: 加密、脱敏、访问审计
- **网络安全**: TLS、VPN、防火墙
- **应用安全**: 输入验证、SQL注入防护、XSS防护
- **监控审计**: 日志审计、行为分析、异常检测

## 认证与授权

### JWT 认证

```go
// internal/auth/jwt.go
package auth

import (
    "crypto/rsa"
    "errors"
    "fmt"
    "time"
    
    "github.com/golang-jwt/jwt/v4"
    "go.uber.org/zap"
)

type JWTManager struct {
    privateKey     *rsa.PrivateKey
    publicKey      *rsa.PublicKey
    accessTokenTTL time.Duration
    refreshTokenTTL time.Duration
    logger         *zap.Logger
}

type Claims struct {
    UserID   string `json:"user_id"`
    Username string `json:"username"`
    Role     string `json:"role"`
    jwt.RegisteredClaims
}

type TokenPair struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
    ExpiresAt    int64  `json:"expires_at"`
}

func NewJWTManager(privateKey *rsa.PrivateKey, publicKey *rsa.PublicKey, 
    accessTTL, refreshTTL time.Duration, logger *zap.Logger) *JWTManager {
    return &JWTManager{
        privateKey:      privateKey,
        publicKey:       publicKey,
        accessTokenTTL:  accessTTL,
        refreshTokenTTL: refreshTTL,
        logger:          logger,
    }
}

func (j *JWTManager) GenerateTokenPair(userID, username, role string) (*TokenPair, error) {
    // 生成访问令牌
    accessToken, err := j.generateAccessToken(userID, username, role)
    if err != nil {
        j.logger.Error("Failed to generate access token", zap.Error(err))
        return nil, fmt.Errorf("failed to generate access token: %w", err)
    }
    
    // 生成刷新令牌
    refreshToken, err := j.generateRefreshToken(userID)
    if err != nil {
        j.logger.Error("Failed to generate refresh token", zap.Error(err))
        return nil, fmt.Errorf("failed to generate refresh token: %w", err)
    }
    
    tokenPair := &TokenPair{
        AccessToken:  accessToken,
        RefreshToken: refreshToken,
        ExpiresAt:    time.Now().Add(j.accessTokenTTL).Unix(),
    }
    
    j.logger.Info("Token pair generated successfully",
        zap.String("user_id", userID),
        zap.String("username", username),
        zap.String("role", role))
    
    return tokenPair, nil
}

func (j *JWTManager) generateAccessToken(userID, username, role string) (string, error) {
    claims := Claims{
        UserID:   userID,
        Username: username,
        Role:     role,
        RegisteredClaims: jwt.RegisteredClaims{
            Subject:   userID,
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(j.accessTokenTTL)),
        },
    }
    
    token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
    return token.SignedString(j.privateKey)
}

func (j *JWTManager) generateRefreshToken(userID string) (string, error) {
    claims := jwt.RegisteredClaims{
        Subject:   userID,
        IssuedAt:  jwt.NewNumericDate(time.Now()),
        ExpiresAt: jwt.NewNumericDate(time.Now().Add(j.refreshTokenTTL)),
    }
    
    token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
    return token.SignedString(j.privateKey)
}

func (j *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return j.publicKey, nil
    })
    
    if err != nil {
        j.logger.Error("Failed to parse token", zap.Error(err))
        return nil, fmt.Errorf("invalid token: %w", err)
    }
    
    if !token.Valid {
        return nil, errors.New("invalid token")
    }
    
    claims, ok := token.Claims.(*Claims)
    if !ok {
        return nil, errors.New("invalid claims")
    }
    
    // 检查令牌是否过期
    if claims.ExpiresAt != nil && claims.ExpiresAt.Time.Before(time.Now()) {
        return nil, errors.New("token has expired")
    }
    
    return claims, nil
}

func (j *JWTManager) RefreshToken(refreshToken string) (*TokenPair, error) {
    // 验证刷新令牌
    claims, err := j.validateRefreshToken(refreshToken)
    if err != nil {
        j.logger.Error("Invalid refresh token", zap.Error(err))
        return nil, fmt.Errorf("invalid refresh token: %w", err)
    }
    
    // 生成新的令牌对
    // 这里需要从数据库获取用户信息
    userID := claims.Subject
    username := "" // 从数据库获取
    role := ""     // 从数据库获取
    
    return j.GenerateTokenPair(userID, username, role)
}

func (j *JWTManager) validateRefreshToken(tokenString string) (*jwt.RegisteredClaims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return j.publicKey, nil
    })
    
    if err != nil {
        return nil, fmt.Errorf("failed to parse refresh token: %w", err)
    }
    
    if !token.Valid {
        return nil, errors.New("invalid refresh token")
    }
    
    claims, ok := token.Claims.(*jwt.RegisteredClaims)
    if !ok {
        return nil, errors.New("invalid refresh token claims")
    }
    
    // 检查刷新令牌是否过期
    if claims.ExpiresAt != nil && claims.ExpiresAt.Time.Before(time.Now()) {
        return nil, errors.New("refresh token has expired")
    }
    
    return claims, nil
}
```

### API Key 认证

```go
// internal/auth/api_key.go
package auth

import (
    "context"
    "crypto/rand"
    "crypto/sha256"
    "encoding/hex"
    "errors"
    "fmt"
    "strings"
    "time"
    
    "go.uber.org/zap"
)

type APIKey struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    Key       string    `json:"key"`
    Hash      string    `json:"hash"`
    UserID    string    `json:"user_id"`
    Role      string    `json:"role"`
    Scopes    []string  `json:"scopes"`
    ExpiresAt time.Time `json:"expires_at"`
    CreatedAt time.Time `json:"created_at"`
    LastUsed  time.Time `json:"last_used"`
    IsActive  bool      `json:"is_active"`
}

type APIKeyManager struct {
    repository APIKeyRepository
    logger     *zap.Logger
}

type APIKeyRepository interface {
    CreateAPIKey(ctx context.Context, key *APIKey) error
    GetAPIKeyByHash(ctx context.Context, hash string) (*APIKey, error)
    UpdateAPIKeyLastUsed(ctx context.Context, keyID string) error
    RevokeAPIKey(ctx context.Context, keyID string) error
    ListAPIKeys(ctx context.Context, userID string) ([]*APIKey, error)
}

func NewAPIKeyManager(repo APIKeyRepository, logger *zap.Logger) *APIKeyManager {
    return &APIKeyManager{
        repository: repo,
        logger:     logger,
    }
}

func (m *APIKeyManager) GenerateAPIKey(ctx context.Context, name, userID, role string, scopes []string, expiresAt time.Time) (*APIKey, error) {
    // 生成API Key
    key, err := generateRandomKey(32)
    if err != nil {
        m.logger.Error("Failed to generate API key", zap.Error(err))
        return nil, fmt.Errorf("failed to generate API key: %w", err)
    }
    
    // 计算哈希值
    hash := hashAPIKey(key)
    
    apiKey := &APIKey{
        ID:        generateUUID(),
        Name:      name,
        Key:       key,
        Hash:      hash,
        UserID:    userID,
        Role:      role,
        Scopes:    scopes,
        ExpiresAt: expiresAt,
        CreatedAt: time.Now(),
        IsActive:  true,
    }
    
    // 保存到数据库
    if err := m.repository.CreateAPIKey(ctx, apiKey); err != nil {
        m.logger.Error("Failed to save API key", zap.Error(err))
        return nil, fmt.Errorf("failed to save API key: %w", err)
    }
    
    m.logger.Info("API key generated successfully",
        zap.String("key_id", apiKey.ID),
        zap.String("user_id", userID),
        zap.String("name", name))
    
    return apiKey, nil
}

func (m *APIKeyManager) ValidateAPIKey(ctx context.Context, apiKey string) (*APIKey, error) {
    if apiKey == "" {
        return nil, errors.New("API key is required")
    }
    
    // 计算哈希值
    hash := hashAPIKey(apiKey)
    
    // 从数据库获取API Key信息
    storedKey, err := m.repository.GetAPIKeyByHash(ctx, hash)
    if err != nil {
        m.logger.Error("Failed to get API key", zap.Error(err))
        return nil, fmt.Errorf("invalid API key: %w", err)
    }
    
    // 检查API Key是否有效
    if !storedKey.IsActive {
        m.logger.Warn("API key is not active", zap.String("key_id", storedKey.ID))
        return nil, errors.New("API key is not active")
    }
    
    // 检查是否过期
    if time.Now().After(storedKey.ExpiresAt) {
        m.logger.Warn("API key has expired", zap.String("key_id", storedKey.ID))
        return nil, errors.New("API key has expired")
    }
    
    // 更新最后使用时间
    if err := m.repository.UpdateAPIKeyLastUsed(ctx, storedKey.ID); err != nil {
        m.logger.Error("Failed to update API key last used time", zap.Error(err))
    }
    
    m.logger.Info("API key validated successfully",
        zap.String("key_id", storedKey.ID),
        zap.String("user_id", storedKey.UserID))
    
    return storedKey, nil
}

func (m *APIKeyManager) RevokeAPIKey(ctx context.Context, keyID string) error {
    if err := m.repository.RevokeAPIKey(ctx, keyID); err != nil {
        m.logger.Error("Failed to revoke API key", zap.Error(err))
        return fmt.Errorf("failed to revoke API key: %w", err)
    }
    
    m.logger.Info("API key revoked successfully", zap.String("key_id", keyID))
    return nil
}

func generateRandomKey(length int) (string, error) {
    bytes := make([]byte, length)
    if _, err := rand.Read(bytes); err != nil {
        return "", err
    }
    return hex.EncodeToString(bytes), nil
}

func hashAPIKey(key string) string {
    hash := sha256.Sum256([]byte(key))
    return hex.EncodeToString(hash[:])
}

func generateUUID() string {
    // 这里使用实际的UUID生成库
    return fmt.Sprintf("%d", time.Now().UnixNano())
}
```

### RBAC 权限管理

```go
// internal/auth/rbac.go
package auth

import (
    "context"
    "errors"
    "fmt"
    
    "go.uber.org/zap"
)

// 权限定义
type Permission string

const (
    // 直播流权限
    PermissionStreamCreate Permission = "stream:create"
    PermissionStreamRead   Permission = "stream:read"
    PermissionStreamUpdate Permission = "stream:update"
    PermissionStreamDelete Permission = "stream:delete"
    PermissionStreamStart  Permission = "stream:start"
    PermissionStreamStop   Permission = "stream:stop"
    
    // 直播间权限
    PermissionRoomCreate Permission = "room:create"
    PermissionRoomRead   Permission = "room:read"
    PermissionRoomUpdate Permission = "room:update"
    PermissionRoomDelete Permission = "room:delete"
    PermissionRoomJoin   Permission = "room:join"
    PermissionRoomLeave  Permission = "room:leave"
    
    // 聊天权限
    PermissionChatSend    Permission = "chat:send"
    PermissionChatRead    Permission = "chat:read"
    PermissionChatDelete  Permission = "chat:delete"
    PermissionChatModerate Permission = "chat:moderate"
    
    // 礼物权限
    PermissionGiftSend Permission = "gift:send"
    PermissionGiftRead Permission = "gift:read"
    
    // 用户权限
    PermissionUserRead   Permission = "user:read"
    PermissionUserUpdate Permission = "user:update"
    PermissionUserDelete Permission = "user:delete"
)

// 角色定义
type Role string

const (
    RoleAdmin     Role = "admin"
    RoleModerator Role = "moderator"
    RoleStreamer  Role = "streamer"
    RoleViewer    Role = "viewer"
    RoleGuest     Role = "guest"
)

// 角色权限映射
var rolePermissions = map[Role][]Permission{
    RoleAdmin: {
        PermissionStreamCreate, PermissionStreamRead, PermissionStreamUpdate, PermissionStreamDelete,
        PermissionStreamStart, PermissionStreamStop,
        PermissionRoomCreate, PermissionRoomRead, PermissionRoomUpdate, PermissionRoomDelete,
        PermissionRoomJoin, PermissionRoomLeave,
        PermissionChatSend, PermissionChatRead, PermissionChatDelete, PermissionChatModerate,
        PermissionGiftSend, PermissionGiftRead,
        PermissionUserRead, PermissionUserUpdate, PermissionUserDelete,
    },
    RoleModerator: {
        PermissionStreamRead,
        PermissionRoomRead, PermissionRoomJoin, PermissionRoomLeave,
        PermissionChatSend, PermissionChatRead, PermissionChatDelete, PermissionChatModerate,
        PermissionGiftSend, PermissionGiftRead,
        PermissionUserRead,
    },
    RoleStreamer: {
        PermissionStreamCreate, PermissionStreamRead, PermissionStreamUpdate, PermissionStreamDelete,
        PermissionStreamStart, PermissionStreamStop,
        PermissionRoomCreate, PermissionRoomRead, PermissionRoomUpdate, PermissionRoomDelete,
        PermissionRoomJoin, PermissionRoomLeave,
        PermissionChatSend, PermissionChatRead,
        PermissionGiftSend, PermissionGiftRead,
        PermissionUserRead,
    },
    RoleViewer: {
        PermissionStreamRead,
        PermissionRoomRead, PermissionRoomJoin, PermissionRoomLeave,
        PermissionChatSend, PermissionChatRead,
        PermissionGiftSend, PermissionGiftRead,
        PermissionUserRead,
    },
    RoleGuest: {
        PermissionStreamRead,
        PermissionRoomRead, PermissionRoomJoin,
        PermissionChatRead,
        PermissionGiftRead,
    },
}

type RBACManager struct {
    repository RBACRepository
    logger     *zap.Logger
}

type RBACRepository interface {
    GetUserRole(ctx context.Context, userID string) (Role, error)
    GetUserPermissions(ctx context.Context, userID string) ([]Permission, error)
    HasPermission(ctx context.Context, userID string, permission Permission) (bool, error)
    GrantPermission(ctx context.Context, userID string, permission Permission) error
    RevokePermission(ctx context.Context, userID string, permission Permission) error
}

func NewRBACManager(repo RBACRepository, logger *zap.Logger) *RBACManager {
    return &RBACManager{
        repository: repo,
        logger:     logger,
    }
}

func (r *RBACManager) CheckPermission(ctx context.Context, userID string, permission Permission) error {
    hasPermission, err := r.repository.HasPermission(ctx, userID, permission)
    if err != nil {
        r.logger.Error("Failed to check permission", 
            zap.String("user_id", userID),
            zap.String("permission", string(permission)),
            zap.Error(err))
        return fmt.Errorf("failed to check permission: %w", err)
    }
    
    if !hasPermission {
        r.logger.Warn("Permission denied",
            zap.String("user_id", userID),
            zap.String("permission", string(permission)))
        return fmt.Errorf("permission denied: %s", permission)
    }
    
    r.logger.Info("Permission granted",
        zap.String("user_id", userID),
        zap.String("permission", string(permission)))
    
    return nil
}

func (r *RBACManager) GetUserPermissions(ctx context.Context, userID string) ([]Permission, error) {
    permissions, err := r.repository.GetUserPermissions(ctx, userID)
    if err != nil {
        r.logger.Error("Failed to get user permissions",
            zap.String("user_id", userID),
            zap.Error(err))
        return nil, fmt.Errorf("failed to get user permissions: %w", err)
    }
    
    return permissions, nil
}

func (r *RBACManager) GrantPermission(ctx context.Context, userID string, permission Permission) error {
    if err := r.repository.GrantPermission(ctx, userID, permission); err != nil {
        r.logger.Error("Failed to grant permission",
            zap.String("user_id", userID),
            zap.String("permission", string(permission)),
            zap.Error(err))
        return fmt.Errorf("failed to grant permission: %w", err)
    }
    
    r.logger.Info("Permission granted",
        zap.String("user_id", userID),
        zap.String("permission", string(permission)))
    
    return nil
}

func (r *RBACManager) RevokePermission(ctx context.Context, userID string, permission Permission) error {
    if err := r.repository.RevokePermission(ctx, userID, permission); err != nil {
        r.logger.Error("Failed to revoke permission",
            zap.String("user_id", userID),
            zap.String("permission", string(permission)),
            zap.Error(err))
        return fmt.Errorf("failed to revoke permission: %w", err)
    }
    
    r.logger.Info("Permission revoked",
        zap.String("user_id", userID),
        zap.String("permission", string(permission)))
    
    return nil
}

func GetRolePermissions(role Role) ([]Permission, error) {
    permissions, exists := rolePermissions[role]
    if !exists {
        return nil, fmt.Errorf("unknown role: %s", role)
    }
    
    return permissions, nil
}

func IsValidRole(role Role) bool {
    _, exists := rolePermissions[role]
    return exists
}

func IsValidPermission(permission Permission) bool {
    for _, permissions := range rolePermissions {
        for _, p := range permissions {
            if p == permission {
                return true
            }
        }
    }
    return false
}
```

## 数据安全

### 数据加密

```go
// internal/crypto/encryption.go
package crypto

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "crypto/sha256"
    "encoding/base64"
    "errors"
    "fmt"
    "io"
    
    "golang.org/x/crypto/pbkdf2"
)

type EncryptionService struct {
    masterKey []byte
}

func NewEncryptionService(masterKey string) (*EncryptionService, error) {
    if masterKey == "" {
        return nil, errors.New("master key is required")
    }
    
    // 使用PBKDF2派生密钥
    key := pbkdf2.Key([]byte(masterKey), []byte("live-service-salt"), 100000, 32, sha256.New)
    
    return &EncryptionService{
        masterKey: key,
    }, nil
}

func (s *EncryptionService) Encrypt(plaintext []byte) (string, error) {
    // 创建AES密码块
    block, err := aes.NewCipher(s.masterKey)
    if err != nil {
        return "", fmt.Errorf("failed to create cipher: %w", err)
    }
    
    // 创建GCM模式
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", fmt.Errorf("failed to create GCM: %w", err)
    }
    
    // 生成随机nonce
    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return "", fmt.Errorf("failed to generate nonce: %w", err)
    }
    
    // 加密数据
    ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
    
    // 返回Base64编码的密文
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (s *EncryptionService) Decrypt(ciphertext string) ([]byte, error) {
    // 解码Base64
    data, err := base64.StdEncoding.DecodeString(ciphertext)
    if err != nil {
        return nil, fmt.Errorf("failed to decode base64: %w", err)
    }
    
    // 创建AES密码块
    block, err := aes.NewCipher(s.masterKey)
    if err != nil {
        return nil, fmt.Errorf("failed to create cipher: %w", err)
    }
    
    // 创建GCM模式
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, fmt.Errorf("failed to create GCM: %w", err)
    }
    
    // 检查数据长度
    if len(data) < gcm.NonceSize() {
        return nil, errors.New("ciphertext too short")
    }
    
    // 分离nonce和密文
    nonce, ciphertext := data[:gcm.NonceSize()], data[gcm.NonceSize():]
    
    // 解密数据
    plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to decrypt: %w", err)
    }
    
    return plaintext, nil
}

func (s *EncryptionService) EncryptString(plaintext string) (string, error) {
    return s.Encrypt([]byte(plaintext))
}

func (s *EncryptionService) DecryptString(ciphertext string) (string, error) {
    plaintext, err := s.Decrypt(ciphertext)
    if err != nil {
        return "", err
    }
    return string(plaintext), nil
}
```

### 数据脱敏

```go
// internal/crypto/sanitizer.go
package crypto

import (
    "regexp"
    "strings"
)

type DataSanitizer struct {
    patterns map[string]*regexp.Regexp
}

func NewDataSanitizer() *DataSanitizer {
    return &DataSanitizer{
        patterns: map[string]*regexp.Regexp{
            "email":       regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`),
            "phone":       regexp.MustCompile(`\b\d{3}[-.]?\d{3}[-.]?\d{4}\b`),
            "credit_card": regexp.MustCompile(`\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`),
            "ssn":         regexp.MustCompile(`\b\d{3}-?\d{2}-?\d{4}\b`),
            "ip":          regexp.MustCompile(`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`),
        },
    }
}

func (s *DataSanitizer) SanitizeEmail(email string) string {
    if email == "" {
        return ""
    }
    
    parts := strings.Split(email, "@")
    if len(parts) != 2 {
        return email
    }
    
    username := parts[0]
    domain := parts[1]
    
    // 隐藏用户名部分
    if len(username) <= 3 {
        return "***@" + domain
    }
    
    visible := username[:2]
    hidden := strings.Repeat("*", len(username)-2)
    return visible + hidden + "@" + domain
}

func (s *DataSanitizer) SanitizePhone(phone string) string {
    if phone == "" {
        return ""
    }
    
    // 移除非数字字符
    digits := regexp.MustCompile(`\D`).ReplaceAllString(phone, "")
    
    if len(digits) != 10 {
        return "***-***-****"
    }
    
    return fmt.Sprintf("***-***-%s", digits[6:])
}

func (s *DataSanitizer) SanitizeCreditCard(card string) string {
    if card == "" {
        return ""
    }
    
    // 移除非数字字符
    digits := regexp.MustCompile(`\D`).ReplaceAllString(card, "")
    
    if len(digits) != 16 {
        return "****-****-****-****"
    }
    
    return fmt.Sprintf("****-****-****-%s", digits[12:])
}

func (s *DataSanitizer) SanitizeSSN(ssn string) string {
    if ssn == "" {
        return ""
    }
    
    // 移除非数字字符
    digits := regexp.MustCompile(`\D`).ReplaceAllString(ssn, "")
    
    if len(digits) != 9 {
        return "***-**-****"
    }
    
    return "***-**-****"
}

func (s *DataSanitizer) SanitizeIP(ip string) string {
    if ip == "" {
        return ""
    }
    
    parts := strings.Split(ip, ".")
    if len(parts) != 4 {
        return "***.***.***.***"
    }
    
    return fmt.Sprintf("%s.***.***.%s", parts[0], parts[3])
}

func (s *DataSanitizer) SanitizeText(text string) string {
    if text == "" {
        return ""
    }
    
    result := text
    
    // 脱敏邮箱
    result = s.patterns["email"].ReplaceAllStringFunc(result, s.SanitizeEmail)
    
    // 脱敏手机号
    result = s.patterns["phone"].ReplaceAllStringFunc(result, s.SanitizePhone)
    
    // 脱敏信用卡
    result = s.patterns["credit_card"].ReplaceAllStringFunc(result, s.SanitizeCreditCard)
    
    // 脱敏SSN
    result = s.patterns["ssn"].ReplaceAllStringFunc(result, s.SanitizeSSN)
    
    // 脱敏IP地址
    result = s.patterns["ip"].ReplaceAllStringFunc(result, s.SanitizeIP)
    
    return result
}
```

## 网络安全

### TLS 配置

```go
// internal/security/tls.go
package security

import (
    "crypto/tls"
    "crypto/x509"
    "errors"
    "fmt"
    "io/ioutil"
    "time"
    
    "go.uber.org/zap"
)

type TLSConfig struct {
    CertFile         string
    KeyFile          string
    CAFile           string
    MinVersion       uint16
    MaxVersion       uint16
    CipherSuites     []uint16
    CurvePreferences []tls.CurveID
}

func NewTLSConfig(config TLSConfig, logger *zap.Logger) (*tls.Config, error) {
    // 加载证书
    cert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
    if err != nil {
        logger.Error("Failed to load certificate", zap.Error(err))
        return nil, fmt.Errorf("failed to load certificate: %w", err)
    }
    
    tlsConfig := &tls.Config{
        Certificates: []tls.Certificate{cert},
        MinVersion:   config.MinVersion,
        MaxVersion:   config.MaxVersion,
    }
    
    // 设置TLS版本
    if config.MinVersion == 0 {
        tlsConfig.MinVersion = tls.VersionTLS12
    }
    
    if config.MaxVersion == 0 {
        tlsConfig.MaxVersion = tls.VersionTLS13
    }
    
    // 设置加密套件
    if len(config.CipherSuites) > 0 {
        tlsConfig.CipherSuites = config.CipherSuites
    } else {
        // 默认使用安全的加密套件
        tlsConfig.CipherSuites = []uint16{
            tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
            tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
            tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
            tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
            tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
        }
    }
    
    // 设置椭圆曲线
    if len(config.CurvePreferences) > 0 {
        tlsConfig.CurvePreferences = config.CurvePreferences
    } else {
        tlsConfig.CurvePreferences = []tls.CurveID{
            tls.CurveP256,
            tls.X25519,
        }
    }
    
    // 加载CA证书（如果提供）
    if config.CAFile != "" {
        caCert, err := ioutil.ReadFile(config.CAFile)
        if err != nil {
            logger.Error("Failed to read CA file", zap.Error(err))
            return nil, fmt.Errorf("failed to read CA file: %w", err)
        }
        
        caCertPool := x509.NewCertPool()
        if !caCertPool.AppendCertsFromPEM(caCert) {
            logger.Error("Failed to parse CA certificate")
            return nil, errors.New("failed to parse CA certificate")
        }
        
        tlsConfig.RootCAs = caCertPool
        tlsConfig.ClientCAs = caCertPool
        tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
    }
    
    // 启用安全特性
    tlsConfig.PreferServerCipherSuites = true
    tlsConfig.SessionTicketsDisabled = true
    
    logger.Info("TLS configuration created successfully",
        zap.Uint16("min_version", tlsConfig.MinVersion),
        zap.Uint16("max_version", tlsConfig.MaxVersion),
        zap.Int("cipher_suites", len(tlsConfig.CipherSuites)))
    
    return tlsConfig, nil
}

// GetSecureTLSConfig 获取安全的TLS配置
func GetSecureTLSConfig() *tls.Config {
    return &tls.Config{
        MinVersion: tls.VersionTLS12,
        MaxVersion: tls.VersionTLS13,
        CipherSuites: []uint16{
            tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
            tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
            tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
            tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
            tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
        },
        CurvePreferences: []tls.CurveID{
            tls.CurveP256,
            tls.X25519,
        },
        PreferServerCipherSuites: true,
        SessionTicketsDisabled:   true,
    }
}
```

### 防火墙规则

```yaml
# 防火墙规则配置
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: live-service-network-policy
  namespace: live-service
spec:
  podSelector:
    matchLabels:
      app: live-service
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: ingress-nginx
    - podSelector:
        matchLabels:
          app: live-service
    ports:
    - protocol: TCP
      port: 8080
    - protocol: TCP
      port: 9090
  - from:
    - namespaceSelector:
        matchLabels:
          name: monitoring
    ports:
    - protocol: TCP
      port: 9090
  egress:
  - to:
    - namespaceSelector:
        matchLabels:
          name: mysql
    ports:
    - protocol: TCP
      port: 3306
  - to:
    - namespaceSelector:
        matchLabels:
          name: redis
    ports:
    - protocol: TCP
      port: 6379
  - to:
    - namespaceSelector:
        matchLabels:
          name: kube-system
    ports:
    - protocol: TCP
      port: 53
    - protocol: UDP
      port: 53
```

## 应用安全

### 输入验证

```go
// internal/security/validation.go
package security

import (
    "encoding/json"
    "fmt"
    "html"
    "regexp"
    "strings"
    
    "github.com/go-playground/validator/v10"
)

type InputValidator struct {
    validator *validator.Validate
}

func NewInputValidator() *InputValidator {
    v := validator.New()
    
    // 注册自定义验证器
    v.RegisterValidation("username", validateUsername)
    v.RegisterValidation("password", validatePassword)
    v.RegisterValidation("safe_text", validateSafeText)
    v.RegisterValidation("stream_title", validateStreamTitle)
    
    return &InputValidator{validator: v}
}

func (v *InputValidator) ValidateStruct(data interface{}) error {
    return v.validator.Struct(data)
}

func (v *InputValidator) ValidateUsername(username string) error {
    if username == "" {
        return fmt.Errorf("username is required")
    }
    
    if len(username) < 3 || len(username) > 30 {
        return fmt.Errorf("username must be between 3 and 30 characters")
    }
    
    // 只允许字母、数字、下划线
    if !regexp.MustCompile(`^[a-zA-Z0-9_]+$`).MatchString(username) {
        return fmt.Errorf("username can only contain letters, numbers, and underscores")
    }
    
    // 检查保留用户名
    reservedUsernames := []string{
        "admin", "root", "system", "api", "test", "guest",
    }
    
    for _, reserved := range reservedUsernames {
        if strings.EqualFold(username, reserved) {
            return fmt.Errorf("username is reserved")
        }
    }
    
    return nil
}

func (v *InputValidator) ValidatePassword(password string) error {
    if password == "" {
        return fmt.Errorf("password is required")
    }
    
    if len(password) < 8 {
        return fmt.Errorf("password must be at least 8 characters long")
    }
    
    if len(password) > 128 {
        return fmt.Errorf("password must not exceed 128 characters")
    }
    
    // 检查复杂度
    hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
    hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
    hasDigit := regexp.MustCompile(`[0-9]`).MatchString(password)
    hasSpecial := regexp.MustCompile(`[!@#$%^&*(),.?":{}|<>]`).MatchString(password)
    
    if !hasUpper || !hasLower || !hasDigit || !hasSpecial {
        return fmt.Errorf("password must contain uppercase, lowercase, digit, and special character")
    }
    
    // 检查常见弱密码
    weakPasswords := []string{
        "password", "123456", "12345678", "qwerty", "abc123",
        "password123", "admin123", "letmein", "welcome",
    }
    
    for _, weak := range weakPasswords {
        if strings.Contains(strings.ToLower(password), weak) {
            return fmt.Errorf("password is too common")
        }
    }
    
    return nil
}

func (v *InputValidator) SanitizeInput(input string) string {
    if input == "" {
        return ""
    }
    
    // HTML转义
    input = html.EscapeString(input)
    
    // 移除控制字符
    input = regexp.MustCompile(`[\x00-\x1F\x7F]`).ReplaceAllString(input, "")
    
    // 移除多个空格
    input = regexp.MustCompile(`\s+`).ReplaceAllString(input, " ")
    
    // 修剪前后空格
    input = strings.TrimSpace(input)
    
    return input
}

func (v *InputValidator) ValidateJSON(data []byte) error {
    var result interface{}
    if err := json.Unmarshal(data, &result); err != nil {
        return fmt.Errorf("invalid JSON: %w", err)
    }
    
    // 递归验证JSON数据
    if err := v.validateJSONValue(result); err != nil {
        return fmt.Errorf("invalid JSON content: %w", err)
    }
    
    return nil
}

func (v *InputValidator) validateJSONValue(value interface{}) error {
    switch val := value.(type) {
    case string:
        // 验证字符串长度
        if len(val) > 10000 {
            return fmt.Errorf("string too long")
        }
        
        // 检查是否包含恶意内容
        if err := v.validateSafeText(val); err != nil {
            return err
        }
        
    case map[string]interface{}:
        // 验证对象深度
        if err := v.validateJSONObjectDepth(val, 0); err != nil {
            return err
        }
        
        // 验证每个值
        for _, v := range val {
            if err := v.validateJSONValue(v); err != nil {
                return err
            }
        }
        
    case []interface{}:
        // 验证数组长度
        if len(val) > 1000 {
            return fmt.Errorf("array too long")
        }
        
        // 验证每个元素
        for _, v := range val {
            if err := v.validateJSONValue(v); err != nil {
                return err
            }
        }
    }
    
    return nil
}

func (v *InputValidator) validateJSONObjectDepth(obj map[string]interface{}, depth int) error {
    if depth > 10 {
        return fmt.Errorf("object depth too deep")
    }
    
    for _, value := range obj {
        if nestedObj, ok := value.(map[string]interface{}); ok {
            if err := v.validateJSONObjectDepth(nestedObj, depth+1); err != nil {
                return err
            }
        }
    }
    
    return nil
}

// 自定义验证器函数
func validateUsername(fl validator.FieldLevel) bool {
    username := fl.Field().String()
    validator := NewInputValidator()
    return validator.ValidateUsername(username) == nil
}

func validatePassword(fl validator.FieldLevel) bool {
    password := fl.Field().String()
    validator := NewInputValidator()
    return validator.ValidatePassword(password) == nil
}

func validateSafeText(fl validator.FieldLevel) bool {
    text := fl.Field().String()
    
    // 检查是否包含恶意脚本
    if regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`).MatchString(text) {
        return false
    }
    
    // 检查是否包含事件处理器
    if regexp.MustCompile(`(?i)on\w+\s*=`).MatchString(text) {
        return false
    }
    
    // 检查是否包含JavaScript协议
    if regexp.MustCompile(`(?i)javascript:`).MatchString(text) {
        return false
    }
    
    // 检查是否包含数据URI
    if regexp.MustCompile(`(?i)data:[^;,]*;base64,`).MatchString(text) {
        return false
    }
    
    return true
}

func validateStreamTitle(fl validator.FieldLevel) bool {
    title := fl.Field().String()
    
    if len(title) > 100 {
        return false
    }
    
    // 检查是否包含敏感词
    sensitiveWords := []string{
        "spam", "scam", "fake", "hack", "crack", "porn", "adult",
    }
    
    for _, word := range sensitiveWords {
        if strings.Contains(strings.ToLower(title), word) {
            return false
        }
    }
    
    return true
}
```

### SQL 注入防护

```go
// internal/security/sql.go
package security

import (
    "database/sql"
    "fmt"
    "regexp"
    "strings"
)

type SQLValidator struct {
    dangerousPatterns []*regexp.Regexp
}

func NewSQLValidator() *SQLValidator {
    return &SQLValidator{
        dangerousPatterns: []*regexp.Regexp{
            regexp.MustCompile(`(?i)(union\s+select)`),
            regexp.MustCompile(`(?i)(insert\s+into)`),
            regexp.MustCompile(`(?i)(delete\s+from)`),
            regexp.MustCompile(`(?i)(drop\s+table)`),
            regexp.MustCompile(`(?i)(alter\s+table)`),
            regexp.MustCompile(`(?i)(exec\s*\()`),
            regexp.MustCompile(`(?i)(execute\s*\()`),
            regexp.MustCompile(`(?i)(script\s*\()`),
            regexp.MustCompile(`(?i)(<script)`),
            regexp.MustCompile(`(?i)(--)`),
            regexp.MustCompile(`(?i)(/\*)`),
            regexp.MustCompile(`(?i)(\*/)`),
            regexp.MustCompile(`(?i)(;)`),
        },
    }
}

func (v *SQLValidator) ValidateInput(input string) error {
    if input == "" {
        return nil
    }
    
    // 检查危险模式
    for _, pattern := range v.dangerousPatterns {
        if pattern.MatchString(input) {
            return fmt.Errorf("potentially dangerous SQL pattern detected")
        }
    }
    
    // 检查特殊字符
    if strings.Contains(input, "'") || strings.Contains(input, "\"") {
        return fmt.Errorf("SQL injection attempt detected: quotes not allowed")
    }
    
    return nil
}

func (v *SQLValidator) SanitizeInput(input string) string {
    if input == "" {
        return ""
    }
    
    // 移除危险字符
    sanitized := input
    
    // 移除SQL注释
    sanitized = regexp.MustCompile(`--.*$`).ReplaceAllString(sanitized, "")
    sanitized = regexp.MustCompile(`/\*.*?\*/`).ReplaceAllString(sanitized, "")
    
    // 移除特殊字符
    sanitized = strings.ReplaceAll(sanitized, ";", "")
    sanitized = strings.ReplaceAll(sanitized, "'", "")
    sanitized = strings.ReplaceAll(sanitized, "\"", "")
    
    // 修剪空白
    sanitized = strings.TrimSpace(sanitized)
    
    return sanitized
}

// SQLQueryBuilder 安全的SQL查询构建器
type SQLQueryBuilder struct {
    table  string
    fields []string
    where  []Condition
    order  []Order
    limit  int
    offset int
}

type Condition struct {
    Field string
    Op    string
    Value interface{}
}

type Order struct {
    Field string
    Dir   string
}

func NewSQLQueryBuilder(table string) *SQLQueryBuilder {
    return &SQLQueryBuilder{
        table:  table,
        fields: []string{},
        where:  []Condition{},
        order:  []Order{},
    }
}

func (b *SQLQueryBuilder) Select(fields ...string) *SQLQueryBuilder {
    b.fields = fields
    return b
}

func (b *SQLQueryBuilder) Where(field, op string, value interface{}) *SQLQueryBuilder {
    b.where = append(b.where, Condition{
        Field: field,
        Op:    op,
        Value: value,
    })
    return b
}

func (b *SQLQueryBuilder) OrderBy(field, dir string) *SQLQueryBuilder {
    b.order = append(b.order, Order{
        Field: field,
        Dir:   dir,
    })
    return b
}

func (b *SQLQueryBuilder) Limit(limit int) *SQLQueryBuilder {
    b.limit = limit
    return b
}

func (b *SQLQueryBuilder) Offset(offset int) *SQLQueryBuilder {
    b.offset = offset
    return b
}

func (b *SQLQueryBuilder) Build() (string, []interface{}, error) {
    if b.table == "" {
        return "", nil, fmt.Errorf("table name is required")
    }
    
    var query strings.Builder
    var args []interface{}
    
    // SELECT 子句
    if len(b.fields) == 0 {
        query.WriteString("SELECT *")
    } else {
        query.WriteString("SELECT ")
        for i, field := range b.fields {
            if i > 0 {
                query.WriteString(", ")
            }
            query.WriteString(field)
        }
    }
    
    // FROM 子句
    query.WriteString(" FROM ")
    query.WriteString(b.table)
    
    // WHERE 子句
    if len(b.where) > 0 {
        query.WriteString(" WHERE ")
        for i, condition := range b.where {
            if i > 0 {
                query.WriteString(" AND ")
            }
            query.WriteString(fmt.Sprintf("%s %s ?", condition.Field, condition.Op))
            args = append(args, condition.Value)
        }
    }
    
    // ORDER BY 子句
    if len(b.order) > 0 {
        query.WriteString(" ORDER BY ")
        for i, order := range b.order {
            if i > 0 {
                query.WriteString(", ")
            }
            query.WriteString(fmt.Sprintf("%s %s", order.Field, order.Dir))
        }
    }
    
    // LIMIT 子句
    if b.limit > 0 {
        query.WriteString(fmt.Sprintf(" LIMIT %d", b.limit))
    }
    
    // OFFSET 子句
    if b.offset > 0 {
        query.WriteString(fmt.Sprintf(" OFFSET %d", b.offset))
    }
    
    return query.String(), args, nil
}
```

## 安全监控

### 安全事件监控

```go
// internal/security/monitor.go
package security

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
    
    "go.uber.org/zap"
)

type SecurityEvent struct {
    ID          string                 `json:"id"`
    Type        string                 `json:"type"`
    Severity    string                 `json:"severity"`
    Source      string                 `json:"source"`
    UserID      string                 `json:"user_id"`
    IPAddress   string                 `json:"ip_address"`
    UserAgent   string                 `json:"user_agent"`
    Description string                 `json:"description"`
    Metadata    map[string]interface{} `json:"metadata"`
    Timestamp   time.Time              `json:"timestamp"`
}

type SecurityMonitor struct {
    logger     *zap.Logger
    repository SecurityEventRepository
    notifier   SecurityNotifier
}

type SecurityEventRepository interface {
    CreateSecurityEvent(ctx context.Context, event *SecurityEvent) error
    GetSecurityEvents(ctx context.Context, filters map[string]interface{}) ([]*SecurityEvent, error)
    GetSecurityEventStats(ctx context.Context, startTime, endTime time.Time) (map[string]interface{}, error)
}

type SecurityNotifier interface {
    NotifySecurityEvent(event *SecurityEvent) error
}

func NewSecurityMonitor(logger *zap.Logger, repo SecurityEventRepository, notifier SecurityNotifier) *SecurityMonitor {
    return &SecurityMonitor{
        logger:     logger,
        repository: repo,
        notifier:   notifier,
    }
}

func (m *SecurityMonitor) LogSecurityEvent(ctx context.Context, eventType, severity, source, userID string, metadata map[string]interface{}) error {
    event := &SecurityEvent{
        ID:        generateEventID(),
        Type:      eventType,
        Severity:  severity,
        Source:    source,
        UserID:    userID,
        Metadata:  metadata,
        Timestamp: time.Now(),
    }
    
    // 记录安全事件
    if err := m.repository.CreateSecurityEvent(ctx, event); err != nil {
        m.logger.Error("Failed to create security event", zap.Error(err))
        return fmt.Errorf("failed to create security event: %w", err)
    }
    
    // 发送通知（如果是高严重性事件）
    if severity == "high" || severity == "critical" {
        if err := m.notifier.NotifySecurityEvent(event); err != nil {
            m.logger.Error("Failed to notify security event", zap.Error(err))
        }
    }
    
    // 记录日志
    m.logger.Warn("Security event detected",
        zap.String("event_id", event.ID),
        zap.String("type", eventType),
        zap.String("severity", severity),
        zap.String("source", source),
        zap.String("user_id", userID),
        zap.Any("metadata", metadata))
    
    return nil
}

func (m *SecurityMonitor) LogFailedLogin(ctx context.Context, username, ipAddress, userAgent, reason string) error {
    metadata := map[string]interface{}{
        "username":   username,
        "reason":     reason,
        "user_agent": userAgent,
    }
    
    return m.LogSecurityEvent(ctx, "failed_login", "medium", "auth", "", metadata)
}

func (m *SecurityMonitor) LogSuspiciousActivity(ctx context.Context, userID, activity, details string) error {
    metadata := map[string]interface{}{
        "activity": activity,
        "details":  details,
    }
    
    return m.LogSecurityEvent(ctx, "suspicious_activity", "high", "user", userID, metadata)
}

func (m *SecurityMonitor) LogPermissionDenied(ctx context.Context, userID, permission, resource string) error {
    metadata := map[string]interface{}{
        "permission": permission,
        "resource": resource,
    }
    
    return m.LogSecurityEvent(ctx, "permission_denied", "medium", "auth", userID, metadata)
}

func (m *SecurityMonitor) LogRateLimitExceeded(ctx context.Context, userID, endpoint, limit string) error {
    metadata := map[string]interface{}{
        "endpoint": endpoint,
        "limit":    limit,
    }
    
    return m.LogSecurityEvent(ctx, "rate_limit_exceeded", "low", "api", userID, metadata)
}

func (m *SecurityMonitor) LogDataAccess(ctx context.Context, userID, dataType, action string, metadata map[string]interface{}) error {
    eventMetadata := map[string]interface{}{
        "data_type": dataType,
        "action":    action,
    }
    
    for k, v := range metadata {
        eventMetadata[k] = v
    }
    
    return m.LogSecurityEvent(ctx, "data_access", "info", "data", userID, eventMetadata)
}

func (m *SecurityMonitor) GetSecurityStats(ctx context.Context, startTime, endTime time.Time) (map[string]interface{}, error) {
    stats, err := m.repository.GetSecurityEventStats(ctx, startTime, endTime)
    if err != nil {
        m.logger.Error("Failed to get security stats", zap.Error(err))
        return nil, fmt.Errorf("failed to get security stats: %w", err)
    }
    
    return stats, nil
}

func generateEventID() string {
    return fmt.Sprintf("evt_%d", time.Now().UnixNano())
}
```

## 合规要求

### GDPR 合规

```go
// internal/compliance/gdpr.go
package compliance

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
    
    "go.uber.org/zap"
)

// GDPRDataSubject GDPR数据主体
type GDPRDataSubject struct {
    UserID    string                 `json:"user_id"`
    DataTypes []string               `json:"data_types"`
    Data      map[string]interface{} `json:"data"`
    CreatedAt time.Time              `json:"created_at"`
    UpdatedAt time.Time              `json:"updated_at"`
}

// GDPRRequest GDPR请求
type GDPRRequest struct {
    ID          string    `json:"id"`
    UserID      string    `json:"user_id"`
    Type        string    `json:"type"` // access, rectification, erasure, portability
    Status      string    `json:"status"`
    RequestedAt time.Time `json:"requested_at"`
    CompletedAt time.Time `json:"completed_at"`
    Result      string    `json:"result"`
}

type GDPRManager struct {
    repository GDPRRepository
    logger     *zap.Logger
}

type GDPRRepository interface {
    GetUserData(ctx context.Context, userID string) (*GDPRDataSubject, error)
    ExportUserData(ctx context.Context, userID string) ([]byte, error)
    DeleteUserData(ctx context.Context, userID string) error
    CreateGDPRRequest(ctx context.Context, request *GDPRRequest) error
    UpdateGDPRRequest(ctx context.Context, request *GDPRRequest) error
    GetGDPRRequest(ctx context.Context, requestID string) (*GDPRRequest, error)
}

func NewGDPRManager(repo GDPRRepository, logger *zap.Logger) *GDPRManager {
    return &GDPRManager{
        repository: repo,
        logger:     logger,
    }
}

// HandleDataAccessRequest 处理数据访问请求
func (m *GDPRManager) HandleDataAccessRequest(ctx context.Context, userID string) ([]byte, error) {
    m.logger.Info("Handling data access request", zap.String("user_id", userID))
    
    // 获取用户数据
    data, err := m.repository.ExportUserData(ctx, userID)
    if err != nil {
        m.logger.Error("Failed to export user data", zap.Error(err))
        return nil, fmt.Errorf("failed to export user data: %w", err)
    }
    
    // 创建GDPR请求记录
    request := &GDPRRequest{
        ID:          generateRequestID(),
        UserID:      userID,
        Type:        "access",
        Status:      "completed",
        RequestedAt: time.Now(),
        CompletedAt: time.Now(),
        Result:      "opt_out_status_updated",
    }
    
    if err := m.repository.CreateCCPARequest(ctx, request); err != nil {
        m.logger.Error("Failed to create CCPA request", zap.Error(err))
        return fmt.Errorf("failed to create CCPA request: %w", err)
    }
    
    m.logger.Info("CCPA opt-out request completed", zap.String("user_id", userID))
    
    return nil
}

// GetUserOptOutStatus 获取用户选择退出状态
func (m *CCPAManager) GetUserOptOutStatus(ctx context.Context, userID string) (bool, error) {
    status, err := m.repository.GetUserOptOutStatus(ctx, userID)
    if err != nil {
        m.logger.Error("Failed to get user opt-out status", zap.Error(err))
        return false, fmt.Errorf("failed to get user opt-out status: %w", err)
    }
    
    return status, nil
}

func generateCCPARequestID() string {
    return fmt.Sprintf("ccpa_%d", time.Now().UnixNano())
}
```

## 安全最佳实践

### 1. 认证安全

- 使用强密码策略
- 实施多因素认证 (MFA)
- 定期轮换API密钥
- 使用安全的会话管理
- 实施账户锁定机制

### 2. 数据安全

- 加密敏感数据（静态和传输中）
- 实施数据脱敏
- 定期备份和测试恢复
- 实施数据保留策略
- 遵守数据保护法规

### 3. 网络安全

- 使用HTTPS/TLS
- 实施网络分段
- 配置防火墙规则
- 使用VPN进行远程访问
- 定期安全扫描

### 4. 应用安全

- 输入验证和清理
- SQL注入防护
- XSS防护
- CSRF防护
- 安全的错误处理

### 5. 监控和审计

- 实施安全事件监控
- 记录所有安全相关事件
- 设置安全告警
- 定期安全审计
- 事件响应计划

### 6. 合规要求

- 遵守GDPR、CCPA等法规
- 实施数据主体权利
- 数据保护影响评估
- 隐私政策
- 数据处理协议

## 安全工具推荐

### 1. 静态代码分析

- **gosec**: Go语言安全分析工具
- **golangci-lint**: 多功能的Go语言lint工具
- **sonarqube**: 代码质量和安全分析平台

### 2. 依赖扫描

- **nancy**: Go模块漏洞扫描
- **snyk**: 开源依赖安全扫描
- **whitesource**: 开源组件安全管理

### 3. 容器安全

- **trivy**: 容器镜像漏洞扫描
- **clair**: 容器镜像安全分析
- **docker-bench-security**: Docker安全基准测试

### 4. 网络安全

- **OWASP ZAP**: Web应用安全扫描
- **nmap**: 网络发现和安全审计
- **wireshark**: 网络协议分析

### 5. 密钥管理

- **HashiCorp Vault**: 密钥和机密管理
- **AWS KMS**: AWS密钥管理服务
- **Azure Key Vault**: Azure密钥管理服务

## 安全事件响应

### 1. 事件分类

| 级别 | 描述 | 响应时间 | 示例 |
|------|------|----------|------|
| P1 - 严重 | 系统完全不可用或数据泄露 | 15分钟 | 数据库泄露、服务瘫痪 |
| P2 - 高 | 重要功能受影响 | 1小时 | API滥用、性能严重下降 |
| P3 - 中 | 一般功能受影响 | 4小时 | 个别功能异常 |
| P4 - 低 | 轻微影响或建议 | 24小时 | 配置优化、安全建议 |

### 2. 响应流程

```
检测 → 评估 → 遏制 → 根除 → 恢复 → 总结
```

### 3. 应急联系

- **安全团队**: security@company.com
- **开发团队**: dev@company.com
- **运维团队**: ops@company.com
- **法务团队**: legal@company.com

### 4. 文档和报告

- 安全事件报告模板
- 事后分析报告
- 改进措施跟踪
- 合规报告

## 安全培训和意识

### 1. 开发人员培训

- 安全编码实践
- 常见漏洞识别
- 安全测试方法
- 代码审查技巧

### 2. 运维人员培训

- 安全配置管理
- 监控和告警
- 事件响应流程
- 备份和恢复

### 3. 定期演练

- 渗透测试
- 红蓝对抗
- 应急响应演练
- 灾难恢复测试

## 相关资源

- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [NIST Cybersecurity Framework](https://www.nist.gov/cyberframework)
- [ISO 27001](https://www.iso.org/isoiec-27001-information-security.html)
- [GDPR](https://gdpr.eu/)
- [CCPA](https://oag.ca.gov/privacy/ccpa)
- [Go Security Guide](https://golang.org/doc/security)
- [Kubernetes Security](https://kubernetes.io/docs/concepts/security/)

## 更新日志

| 日期 | 版本 | 更新内容 |
|------|------|----------|
| 2024-01-01 | 1.0.0 | 初始版本 |
| 2024-01-15 | 1.1.0 | 添加GDPR和CCPA合规 |
| 2024-02-01 | 1.2.0 | 添加安全监控和事件响应 |
| 2024-02-15 | 1.3.0 | 添加安全工具和最佳实践 |.Now(),
        CompletedAt: time.Now(),
        Result:      "data_exported",
    }
    
    if err := m.repository.CreateGDPRRequest(ctx, request); err != nil {
        m.logger.Error("Failed to create GDPR request", zap.Error(err))
    }
    
    m.logger.Info("Data access request completed", zap.String("user_id", userID))
    
    return data, nil
}

// HandleDataErasureRequest 处理数据删除请求
func (m *GDPRManager) HandleDataErasureRequest(ctx context.Context, userID string) error {
    m.logger.Info("Handling data erasure request", zap.String("user_id", userID))
    
    // 检查是否可以删除数据（例如是否有未完成的订单）
    if err := m.canDeleteUserData(ctx, userID); err != nil {
        m.logger.Warn("Cannot delete user data", zap.Error(err))
        return fmt.Errorf("cannot delete user data: %w", err)
    }
    
    // 删除用户数据
    if err := m.repository.DeleteUserData(ctx, userID); err != nil {
        m.logger.Error("Failed to delete user data", zap.Error(err))
        return fmt.Errorf("failed to delete user data: %w", err)
    }
    
    // 创建GDPR请求记录
    request := &GDPRRequest{
        ID:          generateRequestID(),
        UserID:      userID,
        Type:        "erasure",
        Status:      "completed",
        RequestedAt: time.Now(),
        CompletedAt: time.Now(),
        Result:      "data_deleted",
    }
    
    if err := m.repository.CreateGDPRRequest(ctx, request); err != nil {
        m.logger.Error("Failed to create GDPR request", zap.Error(err))
    }
    
    m.logger.Info("Data erasure request completed", zap.String("user_id", userID))
    
    return nil
}

// HandleDataPortabilityRequest 处理数据可携带性请求
func (m *GDPRManager) HandleDataPortabilityRequest(ctx context.Context, userID string) ([]byte, error) {
    m.logger.Info("Handling data portability request", zap.String("user_id", userID))
    
    // 获取用户数据（结构化格式）
    dataSubject, err := m.repository.GetUserData(ctx, userID)
    if err != nil {
        m.logger.Error("Failed to get user data", zap.Error(err))
        return nil, fmt.Errorf("failed to get user data: %w", err)
    }
    
    // 转换为标准格式
    portableData := map[string]interface{}{
        "user_id":     dataSubject.UserID,
        "data_types":  dataSubject.DataTypes,
        "data":        dataSubject.Data,
        "export_date": time.Now().Format(time.RFC3339),
        "format":      "json",
        "version":     "1.0",
    }
    
    // JSON编码
    data, err := json.MarshalIndent(portableData, "", "  ")
    if err != nil {
        m.logger.Error("Failed to marshal portable data", zap.Error(err))
        return nil, fmt.Errorf("failed to marshal portable data: %w", err)
    }
    
    // 创建GDPR请求记录
    request := &GDPRRequest{
        ID:          generateRequestID(),
        UserID:      userID,
        Type:        "portability",
        Status:      "completed",
        RequestedAt: time.Now(),
        CompletedAt: time.Now(),
        Result:      "data_ported",
    }
    
    if err := m.repository.CreateGDPRRequest(ctx, request); err != nil {
        m.logger.Error("Failed to create GDPR request", zap.Error(err))
    }
    
    m.logger.Info("Data portability request completed", zap.String("user_id", userID))
    
    return data, nil
}

func (m *GDPRManager) canDeleteUserData(ctx context.Context, userID string) error {
    // 这里应该实现具体的业务逻辑检查
    // 例如：
    // - 检查是否有未完成的订单
    // - 检查是否有未结算的财务记录
    // - 检查是否有法律要求保留的数据
    
    return nil
}

func generateRequestID() string {
    return fmt.Sprintf("gdpr_%d", time.Now().UnixNano())
}
```

### CCPA 合规

```go
// internal/compliance/ccpa.go
package compliance

import (
    "context"
    "fmt"
    "time"
    
    "go.uber.org/zap"
)

// CCPARequest CCPA请求
type CCPARequest struct {
    ID          string    `json:"id"`
    UserID      string    `json:"user_id"`
    Type        string    `json:"type"` // know, delete, opt-out
    Status      string    `json:"status"`
    RequestedAt time.Time `json:"requested_at"`
    CompletedAt time.Time `json:"completed_at"`
    Result      string    `json:"result"`
}

type CCPAManager struct {
    repository CCPARepository
    logger     *zap.Logger
}

type CCPARepository interface {
    CreateCCPARequest(ctx context.Context, request *CCPARequest) error
    UpdateCCPARequest(ctx context.Context, request *CCPARequest) error
    GetCCPARequest(ctx context.Context, requestID string) (*CCPARequest, error)
    GetUserOptOutStatus(ctx context.Context, userID string) (bool, error)
    SetUserOptOutStatus(ctx context.Context, userID string, optOut bool) error
}

func NewCCPAManager(repo CCPARepository, logger *zap.Logger) *CCPAManager {
    return &CCPAManager{
        repository: repo,
        logger:     logger,
    }
}

// HandleKnowRequest 处理知情请求
func (m *CCPAManager) HandleKnowRequest(ctx context.Context, userID string) error {
    m.logger.Info("Handling CCPA know request", zap.String("user_id", userID))
    
    // 这里实现具体的知情请求处理逻辑
    // 返回用户数据的收集、使用和共享信息
    
    request := &CCPARequest{
        ID:          generateCCPARequestID(),
        UserID:      userID,
        Type:        "know",
        Status:      "completed",
        RequestedAt: time.Now(),
        CompletedAt: time.Now(),
        Result:      "information_provided",
    }
    
    if err := m.repository.CreateCCPARequest(ctx, request); err != nil {
        m.logger.Error("Failed to create CCPA request", zap.Error(err))
        return fmt.Errorf("failed to create CCPA request: %w", err)
    }
    
    m.logger.Info("CCPA know request completed", zap.String("user_id", userID))
    
    return nil
}

// HandleDeleteRequest 处理删除请求
func (m *CCPAManager) HandleDeleteRequest(ctx context.Context, userID string) error {
    m.logger.Info("Handling CCPA delete request", zap.String("user_id", userID))
    
    // 这里实现具体的删除请求处理逻辑
    // 删除用户的个人信息
    
    request := &CCPARequest{
        ID:          generateCCPARequestID(),
        UserID:      userID,
        Type:        "delete",
        Status:      "completed",
        RequestedAt: time.Now(),
        CompletedAt: time.Now(),
        Result:      "data_deleted",
    }
    
    if err := m.repository.CreateCCPARequest(ctx, request); err != nil {
        m.logger.Error("Failed to create CCPA request", zap.Error(err))
        return fmt.Errorf("failed to create CCPA request: %w", err)
    }
    
    m.logger.Info("CCPA delete request completed", zap.String("user_id", userID))
    
    return nil
}

// HandleOptOutRequest 处理选择退出请求
func (m *CCPAManager) HandleOptOutRequest(ctx context.Context, userID string, optOut bool) error {
    m.logger.Info("Handling CCPA opt-out request", 
        zap.String("user_id", userID),
        zap.Bool("opt_out", optOut))
    
    // 设置用户的选择退出状态
    if err := m.repository.SetUserOptOutStatus(ctx, userID, optOut); err != nil {
        m.logger.Error("Failed to set opt-out status", zap.Error(err))
        return fmt.Errorf("failed to set opt-out status: %w", err)
    }
    
    request := &CCPARequest{
        ID:          generateCCPARequestID(),
        UserID:      userID,
        Type:        "opt-out",
        Status:      "completed",
        RequestedAt: time