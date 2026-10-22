package runtimecontext

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/fab-viewer/common-lib/common/i18n"
	"github.com/fab-viewer/common-lib/common/rctx"
	"github.com/fab-viewer/common-lib/common/units"
)

// wire 契约：请求运行时上下文的五个请求头字段名（本中间件是唯一组装方，
// 常量随生产方走；响应头 txId 回写也用 HeaderTxID）。
const (
	// HeaderTxID 是事务 ID 的请求/响应头字段名（请求携带则透传）。
	HeaderTxID = "txId"
	// HeaderTxTime 是事务时间的请求头字段名，格式 yyyyMMddHHmmss
	// （缺失/非法由中间件兜底服务器时间）。
	HeaderTxTime = "txTime"
	// HeaderTxName 是事务名称的请求头字段名。
	HeaderTxName = "txName"
	// HeaderUserName 是操作用户名的请求头字段名（即审计操作者）。
	HeaderUserName = "userName"
	// HeaderAcceptLanguage 是标准请求语言头（值如 "zh-CN,zh;q=0.9,en;q=0.8"，
	// 取首选语言规范化后注入 RuntimeContext 的请求语言）。
	HeaderAcceptLanguage = "Accept-Language"
)

// FillRuntimeContext 构建请求运行时上下文的中间件工厂：解析请求头
// txId/txTime/txName/userName 与标准头 Accept-Language（取首选语言，
// i18n.ParseAcceptLanguage 规范化）组装为 rctx.RuntimeContext，注入
// 请求 context（c.Request 替换）——handler 侧 rctx.GetRuntimeContext
// 直取，响应体 txId / 访问日志 / 审计钩子同一份数据；认证中间件更新
// 操作者同样经 rctx.SetRuntimeContext 替换 c.Request，全链路即见新值。
//
// 兜底规则：txId 缺失用 crypto/rand 生成 16 字节 hex（缺失兜底行为待定，
// 如需调整只改本函数）；txTime 缺失或非法用服务器时间（经
// units.ParseDateTime 归一）；Accept-Language 缺失/非法时 Lang 为空串
// （i18n 翻译回退默认语言）。统一回写响应头 txId。
func FillRuntimeContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(HeaderTxID)
		if id == "" {
			id = newTxID()
		}
		lang := ""
		if langs := i18n.ParseAcceptLanguage(c.GetHeader(HeaderAcceptLanguage)); len(langs) > 0 {
			lang = string(langs[0])
		}
		rc := rctx.RuntimeContext{}.
			WithID(id).
			WithTime(parseTxTime(c.GetHeader(HeaderTxTime))).
			WithName(c.GetHeader(HeaderTxName)).
			WithUser(c.GetHeader(HeaderUserName)).
			WithLang(lang)

		c.Request = c.Request.WithContext(rctx.SetRuntimeContext(c.Request.Context(), rc))
		c.Header(HeaderTxID, rc.FromID())
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

// parseTxTime 解析 txTime（yyyyMMddHHmmss，经 units.ParseDateTime 归一）：
// 空串或格式非法时兜底为服务器当前时间。
func parseTxTime(v string) time.Time {
	if v == "" {
		return time.Now()
	}
	t, err := units.ParseDateTime(v)
	if err != nil {
		return time.Now()
	}
	return t
}
