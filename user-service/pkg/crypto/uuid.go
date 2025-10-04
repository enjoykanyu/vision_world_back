package crypto

import (
	"crypto/rand"
	"fmt"
	"time"
)

// GenerateUUID 生成UUID
func GenerateUUID() string {
	uuid := make([]byte, 16)
	n, err := rand.Read(uuid)
	if n != len(uuid) || err != nil {
		return fmt.Sprintf("error-%d", time.Now().Unix())
	}
	// 设置版本号 (4) 和变体号 (RFC 4122)
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // Version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // Variant RFC 4122
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:])
}
