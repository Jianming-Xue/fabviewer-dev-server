package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/fab-viewer/common-lib/api/reqctx"
)

// ApiResponse 是统一响应体，成功/失败同构（五字段恒序列化），HTTP 状态码
// 恒为 **200**（传输层只承载送达，业务结果由 code/success 表达）：
//
//	{"code":1,"success":true,"errMsg":"","data":{...},"txId":"..."}
//	{"code":0,"success":false,"errMsg":"...","data":null,"txId":"..."}
//
// code 是业务自己的错误码：OK 默认 **1**、Fail 默认 **0**，均经 WithCode
// 修改；txId 取自 RuntimeContext 记录的事务 ID（请求头 txId 透传优先，
// 未携带时由 runtimecontext 中间件生成兜底）；data 由业务 handler 指定：
// single 对象或 multi 对象集合（分页出参 pageable 整体作为 data）。
//
//nolint:revive // 命名沿用业务方拍板的 ApiResponse（非 Go 初始词规范 API）
type ApiResponse struct {
	Code    int    `json:"code"`
	Success bool   `json:"success"`
	ErrMsg  string `json:"errMsg"`
	Data    any    `json:"data"`
	TxID    string `json:"txId"`
}

// Option 调整 OK/Fail 的默认字段。
type Option func(*ApiResponse)

// WithCode 指定响应体的业务 code（OK 默认 1、Fail 默认 0；不影响 HTTP
// 状态码——统一响应的 HTTP 恒为 200）。
func WithCode(code int) Option {
	return func(r *ApiResponse) { r.Code = code }
}

// WithErrMsg 为成功响应补充提示消息（OK 默认为空串）。
func WithErrMsg(msg string) Option {
	return func(r *ApiResponse) { r.ErrMsg = msg }
}

// OK 写回成功响应：code 默认 1、success=true、errMsg 默认空串（可经
// WithErrMsg 设置），data 由 handler 给定（single 对象或集合均可）。
func OK(c *gin.Context, data any, opts ...Option) {
	r := newApiResponse(c, 1, true, "", data, opts)
	write(c, r)
}

// Fail 写回失败响应：code 默认 0、success 恒 false、data 恒 nil，errMsg
// 必填（空串时兜底 "internal error"）；code 可经 WithCode 换成业务自己的
// 错误码。常规业务错误走 c.Error(err) 交由 errorhandler 统一处理（按
// WithCodeMap 映射模块码）；本方法用于不经中间件的轻量场景。
func Fail(c *gin.Context, errMsg string, opts ...Option) {
	if errMsg == "" {
		errMsg = "internal error"
	}
	r := newApiResponse(c, 0, false, errMsg, nil, opts)
	write(c, r)
}

// newApiResponse 组装响应体：逐项应用 Option，txId 取 RuntimeContext
// 记录的事务 ID。
//
//nolint:revive // 同 ApiResponse：沿用业务方拍板命名
func newApiResponse(c *gin.Context, code int, success bool, errMsg string, data any, opts []Option) ApiResponse {
	r := ApiResponse{
		Code:    code,
		Success: success,
		ErrMsg:  errMsg,
		Data:    data,
		TxID:    reqctx.TxIDOf(c),
	}
	for _, opt := range opts {
		opt(&r)
	}
	return r
}

// write 将 ApiResponse 以 JSON 写回，HTTP 状态码恒 200；响应已写过时
// 跳过（防双写）。构建响应只有 OK/Fail 两个入口，本函数私有。
func write(c *gin.Context, resp ApiResponse) {
	if c.Writer.Written() {
		return
	}
	c.JSON(http.StatusOK, resp)
}
