package handlers

import (
	"github.com/gin-gonic/gin"

	"github.com/fab-viewer/common-lib/api/response"
)

// Readyz 是就绪探活 handler（GET /api/readyz，server.NewEngine 已内置
// 注册，访问日志默认跳过）：直接返回无数据的 OK，不带任何依赖就绪
// 判定——
//
//	{"code":"OK","success":true,"errMsg":"","data":null,"txId":"..."}
//
// 后续接入存储 Ping 等真实就绪检查时在本 handler 内扩展。
func Readyz(c *gin.Context) {
	response.OK(c, nil)
}
