package api

import (
	"github.com/gin-gonic/gin"

	"github.com/fab-viewer/common-lib/common/paging"
)

// PageQuery 从请求查询参数解析分页对象（pageIndex/pageSize）。
// 参数缺失、非数字等非法输入一律回退默认值（第 1 页、每页 20 条），
// 超上限的 pageSize 由 paging.PageRequest.Normalize 截断。
func PageQuery(c *gin.Context) paging.PageRequest {
	var q paging.PageRequest
	// 绑定失败时 q 保持零值，Normalize 后即为默认分页，无需感知错误
	_ = c.ShouldBindQuery(&q)
	return q.Normalize()
}
