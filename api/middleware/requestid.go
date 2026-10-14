package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/fab-viewer/common-lib/api"
)

// RequestID 解析或生成请求 ID：请求头已携带 X-Request-Id 则透传，
// 否则用 crypto/rand 生成 16 字节 hex；统一写入 gin.Context（key 为
// api.CtxRequestID，供响应体 requestId 字段读取）并回写响应头。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader(api.HeaderRequestID)
		if rid == "" {
			rid = newRequestID()
		}
		c.Set(api.CtxRequestID, rid)
		c.Header(api.HeaderRequestID, rid)
		c.Next()
	}
}

// randRead 可在测试中替换以模拟 crypto/rand 失败。
var randRead = rand.Read

// newRequestID 生成 32 字符 hex 请求 ID；crypto/rand 失败（极罕见）时
// 降级为纳秒时间戳保证唯一性兜底。
func newRequestID() string {
	b := make([]byte, 16)
	if _, err := randRead(b); err != nil {
		return fmt.Sprintf("t%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
