package api_test

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/fab-viewer/common-lib/api"
	"github.com/fab-viewer/common-lib/common/paging"
)

func TestPageQuery(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  paging.Query
	}{
		{"无参数取默认", "", paging.Query{PageIndex: 1, PageSize: 20}},
		{"合法参数原样", "?pageIndex=2&pageSize=30", paging.Query{PageIndex: 2, PageSize: 30}},
		{"非数字回退默认", "?pageIndex=abc&pageSize=xyz", paging.Query{PageIndex: 1, PageSize: 20}},
		{"负数修正", "?pageIndex=-1&pageSize=-5", paging.Query{PageIndex: 1, PageSize: 20}},
		{"超上限截断", "?pageIndex=3&pageSize=99999", paging.Query{PageIndex: 3, PageSize: paging.MaxSize}},
		{"部分参数", "?pageSize=50", paging.Query{PageIndex: 1, PageSize: 50}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got paging.Query
			r := newTestEngine()
			r.GET("/q", func(c *gin.Context) { got = api.PageQuery(c) })

			doRequest(t, r, http.MethodGet, "/q"+tt.query)

			assert.Equal(t, tt.want, got)
		})
	}
}
