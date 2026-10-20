package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/fab-viewer/common-lib/api"
	"github.com/fab-viewer/common-lib/common/actortx"
	"github.com/fab-viewer/common-lib/common/i18n"
	"github.com/fab-viewer/common-lib/common/rctx"
)

// FillRuntimeContext 构建请求运行时上下文的中间件工厂：解析请求头
// txId/txTime/txName/userName 与标准头 Accept-Language（取首选语言，
// i18n.ParseAcceptLanguage 规范化）组装为 rctx.RuntimeContext，三路注入——
//
//   - 请求 context（rctx.SetRuntimeContext）：service/storage 层经 rctx.GetRuntimeContext
//     全链路可读
//   - 操作者上下文（actortx.SetActorContext，取 userName）：oracle 审计钩子
//     自动填
//     CreatedBy/UpdatedBy（userName 为空不注入）
//   - gin.Context（key 为 api.CtxRuntimeContext）：响应体 txId 字段读取
//
// 兜底规则：txId 缺失用 crypto/rand 生成 16 字节 hex（缺失兜底行为待定，
// 如需调整只改本函数）；txTime 缺失或非 yyyyMMDDHHmmss 用服务器时间；
// Accept-Language 缺失/非法时 Lang 为空串（i18n 翻译回退默认语言）。
// 统一回写响应头 txId。
func FillRuntimeContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		rc := rctx.RuntimeContext{
			ID:   c.GetHeader(api.HeaderTxID),
			Time: parseTxTime(c.GetHeader(api.HeaderTxTime)),
			Name: c.GetHeader(api.HeaderTxName),
			User: c.GetHeader(api.HeaderUserName),
		}
		if rc.ID == "" {
			rc.ID = newTxID()
		}
		if langs := i18n.ParseAcceptLanguage(c.GetHeader(api.HeaderAcceptLanguage)); len(langs) > 0 {
			rc.Lang = string(langs[0])
		}
		c.Set(api.CtxRuntimeContext, rc)

		ctx := rctx.SetRuntimeContext(c.Request.Context(), rc)
		ctx = actortx.SetActorContext(ctx, rc.User)
		c.Request = c.Request.WithContext(ctx)

		c.Header(api.HeaderTxID, rc.ID)
		c.Next()
	}
}

// randRead 可在测试中替换以模拟 crypto/rand 失败。
var randRead = rand.Read

// newTxID 生成 32 字符 hex 事务 ID；crypto/rand 失败（极罕见）时降级为
// 纳秒时间戳保证唯一性兜底。
func newTxID() string {
	b := make([]byte, 16)
	if _, err := randRead(b); err != nil {
		return fmt.Sprintf("t%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// txTimeLayout 是 txTime 请求头的约定格式（yyyyMMDDHHmmss）。
const txTimeLayout = "20060102150405"

// parseTxTime 解析 txTime：空串或格式非法时兜底为服务器当前时间。
func parseTxTime(v string) time.Time {
	if v == "" {
		return time.Now()
	}
	t, err := time.Parse(txTimeLayout, v)
	if err != nil {
		return time.Now()
	}
	return t
}
