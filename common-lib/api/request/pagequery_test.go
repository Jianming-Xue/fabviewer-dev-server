package request_test

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/fab-viewer/common-lib/api/internal/testutil"
	"github.com/fab-viewer/common-lib/api/request"
	"github.com/fab-viewer/common-lib/common/paging"
)

func TestPageQuery(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  paging.PageRequest
	}{
		{"无参数取默认", "", paging.PageRequest{PageIndex: 1, PageSize: paging.DefaultSize}},
		{"合法参数原样", "?pageIndex=2&pageSize=30", paging.PageRequest{PageIndex: 2, PageSize: 30}},
		{"非数字回退默认", "?pageIndex=abc&pageSize=xyz", paging.PageRequest{PageIndex: 1, PageSize: paging.DefaultSize}},
		{"负数修正", "?pageIndex=-1&pageSize=-5", paging.PageRequest{PageIndex: 1, PageSize: paging.DefaultSize}},
		{"超上限截断", "?pageIndex=3&pageSize=99999", paging.PageRequest{PageIndex: 3, PageSize: paging.MaxPageSize}},
		{"部分参数", "?pageSize=50", paging.PageRequest{PageIndex: 1, PageSize: 50}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got paging.PageRequest
			r := testutil.NewTestEngine()
			r.GET("/q", func(c *gin.Context) { got = request.PageQuery(c) })

			testutil.DoGet(r, "/q"+tt.query)

			assert.Equal(t, tt.want, got)
		})
	}
}
