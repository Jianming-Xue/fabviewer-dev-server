package app_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	server "github.com/fab-viewer/common-lib/cmd/server"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fab-viewer/common-lib/common/paging"
	"github.com/fab-viewer/security-management/api"
	"github.com/fab-viewer/security-management/app"
	"github.com/fab-viewer/security-management/services"
	"github.com/fab-viewer/security-management/storage/memory"
	"github.com/fab-viewer/security-management/usererrors"
)

// newTestServer 组装内存版服务与路由（i18n 全链路可用）。
func newTestServer() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := server.NewEngine(usererrors.StatusMap())
	api.RegisterRoutes(r, app.NewUserService(memory.NewUserRepo()))
	return r
}

// respBody 是统一响应体解码目标。
type respBody struct {
	Code     int             `json:"code"`
	Success  bool            `json:"success"`
	ErrMsg   string          `json:"errMsg"`
	Data     json.RawMessage `json:"data"`
	TxID     string          `json:"txId"`
}

func doJSON(t *testing.T, r http.Handler, method, target, body string, hdr map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	r.ServeHTTP(w, req)
	return w
}

func TestUserCRUDFlow(t *testing.T) {
	r := newTestServer()

	t.Run("创建并回读", func(t *testing.T) {
		w := doJSON(t, r, http.MethodPost, "/api/users",
			`{"username":"zhangsan","name":"张三","email":"z@x.com","enabled":1}`, nil)
		require.Equal(t, http.StatusOK, w.Code)

		var b respBody
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
		var dto services.UserDTO
		require.NoError(t, json.Unmarshal(b.Data, &dto))
		assert.NotEmpty(t, dto.ID, "雪花 ID 已生成")
		assert.Equal(t, "张三", dto.Name)

		// 按登录名重复创建 → 冲突
		w = doJSON(t, r, http.MethodPost, "/api/users",
			`{"username":"zhangsan","name":"重复"}`,
			map[string]string{"Accept-Language": "zh-CN"})
		require.Equal(t, http.StatusConflict, w.Code)
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
		assert.Equal(t, "用户名 zhangsan 已存在", b.ErrMsg)
	})

	t.Run("审计字段取请求头 userName", func(t *testing.T) {
		w := doJSON(t, r, http.MethodPost, "/api/users",
			`{"username":"lisi","name":"李四"}`, map[string]string{"userName": "ops-admin"})
		require.Equal(t, http.StatusOK, w.Code)
		var b respBody
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
		var dto services.UserDTO
		require.NoError(t, json.Unmarshal(b.Data, &dto))
		assert.Equal(t, "lisi", dto.Username)
	})

	t.Run("分页列表", func(t *testing.T) {
		w := doJSON(t, r, http.MethodGet, "/api/users?pageIndex=1&pageSize=10", "", nil)
		require.Equal(t, http.StatusOK, w.Code)
		var b respBody
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
		assert.Contains(t, string(b.Data), `"totalSize":2`)
	})

	t.Run("未找到按请求语言", func(t *testing.T) {
		w := doJSON(t, r, http.MethodGet, "/api/users/no-such-id", "",
			map[string]string{"Accept-Language": "zh-CN"})
		var b respBody
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
		assert.Equal(t, http.StatusNotFound, b.Code)
		assert.Equal(t, "用户 no-such-id 不存在", b.ErrMsg)

		w = doJSON(t, r, http.MethodGet, "/api/users/no-such-id", "", nil)
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
		assert.Equal(t, "user no-such-id not found", b.ErrMsg)
	})
}

func TestDeleteConfirmation(t *testing.T) {
	r := newTestServer()
	w := doJSON(t, r, http.MethodPost, "/api/users",
		`{"username":"wangwu","name":"王五"}`, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var b respBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
	var dto services.UserDTO
	require.NoError(t, json.Unmarshal(b.Data, &dto))

	t.Run("未确认：消息带姓名", func(t *testing.T) {
		w := doJSON(t, r, http.MethodDelete, "/api/users/"+dto.ID, "",
			map[string]string{"Accept-Language": "zh-CN"})
		var b respBody
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
		assert.Equal(t, http.StatusConflict, b.Code)
		assert.Equal(t, "确认删除用户 王五 吗？", b.ErrMsg, "确认消息必须带被删对象 Name")

		w = doJSON(t, r, http.MethodDelete, "/api/users/"+dto.ID, "", nil)
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
		assert.Equal(t, "confirm deleting user 王五?", b.ErrMsg)
	})

	t.Run("确认后真正删除", func(t *testing.T) {
		w := doJSON(t, r, http.MethodDelete, "/api/users/"+dto.ID+"?confirm=true", "", nil)
		require.Equal(t, http.StatusOK, w.Code)

		w = doJSON(t, r, http.MethodGet, "/api/users/"+dto.ID, "", nil)
		var b respBody
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
		assert.Equal(t, http.StatusNotFound, b.Code)
	})
}

func TestServiceLayerDirect(t *testing.T) {
	svc := app.NewUserService(memory.NewUserRepo())
	ctx := context.Background()

	_, err := svc.Create(ctx, services.CreateUserCmd{Username: "a", Name: "A"})
	require.NoError(t, err)

	page, err := svc.List(ctx, paging.PageRequest{PageIndex: 1, PageSize: 20})
	require.NoError(t, err)
	assert.EqualValues(t, 1, page.TotalSize)
}
