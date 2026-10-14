package api_test

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/fab-viewer/common-lib/api"
	"github.com/fab-viewer/common-lib/paging"
)

func TestPageQuery(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  paging.Query
	}{
		{"无参数取默认", "", paging.Query{Page: 1, Size: 20}},
		{"合法参数原样", "?page=2&size=30", paging.Query{Page: 2, Size: 30}},
		{"非数字回退默认", "?page=abc&size=xyz", paging.Query{Page: 1, Size: 20}},
		{"负数修正", "?page=-1&size=-5", paging.Query{Page: 1, Size: 20}},
		{"超上限截断", "?page=3&size=99999", paging.Query{Page: 3, Size: paging.MaxSize}},
		{"部分参数", "?size=50", paging.Query{Page: 1, Size: 50}},
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
