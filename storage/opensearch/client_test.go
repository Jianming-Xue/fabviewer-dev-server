package opensearch

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fab-viewer/common-lib/common/errs"
	"github.com/fab-viewer/common-lib/storage/opensearch/config"
)

// TestOpen 验证 Open 的装配与失败路径。
func TestOpen(t *testing.T) {
	t.Run("成功", func(t *testing.T) {
		cli, err := Open(config.Config{Addrs: []string{"http://127.0.0.1:19200"}})
		require.NoError(t, err)
		require.NotNil(t, cli)
		assert.NotNil(t, cli.SDK(), "SDK 逃生舱应返回底层客户端")
	})

	t.Run("空 Addrs 拒绝", func(t *testing.T) {
		_, err := Open(config.Config{})
		require.Error(t, err)
		require.ErrorIs(t, err, errs.ErrValidation)
	})

	t.Run("非法地址", func(t *testing.T) {
		_, err := Open(config.Config{Addrs: []string{"://bad-address"}})
		require.Error(t, err)
		e, ok := errs.From(err)
		require.True(t, ok)
		assert.Equal(t, errs.CodeDB, e.Code)
	})
}

// orderDoc 是 Index/Get/Delete 共用的业务文档替身。
type orderDoc struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	N     int    `json:"n"`
}

// TestIndex 走完整 SDK 链路验证文档写入：请求形态、成功、
// 参数校验、序列化失败与状态码翻译，并断言响应体必被关闭。
func TestIndex(t *testing.T) {
	t.Run("成功 PUT", func(t *testing.T) {
		ft := &fakeTransport{status: http.StatusOK, body: `{"_index":"orders","_id":"o-1","result":"created"}`}
		cli := newFakeClient(t, ft)
		require.NoError(t, cli.Index(context.Background(), "orders", "o-1", orderDoc{ID: "o-1", Title: "t"}))

		req := ft.firstRequest(t)
		assert.Equal(t, http.MethodPut, req.Method)
		assert.Equal(t, "/orders/_doc/o-1", req.Path)
		assert.JSONEq(t, `{"id":"o-1","title":"t","n":0}`, req.Body)
		assert.Equal(t, 1, ft.requestCount())
		ft.assertAllClosed(t)
	})

	t.Run("空 id 走 POST 自动生成", func(t *testing.T) {
		ft := &fakeTransport{status: http.StatusCreated, body: `{"result":"created"}`}
		cli := newFakeClient(t, ft)
		require.NoError(t, cli.Index(context.Background(), "orders", "", orderDoc{ID: "x"}))

		req := ft.firstRequest(t)
		assert.Equal(t, http.MethodPost, req.Method)
		assert.Equal(t, "/orders/_doc", req.Path)
		ft.assertAllClosed(t)
	})

	t.Run("空 index 拒绝", func(t *testing.T) {
		ft := &fakeTransport{}
		cli := newFakeClient(t, ft)
		err := cli.Index(context.Background(), "", "o-1", orderDoc{})
		require.ErrorIs(t, err, errs.ErrValidation)
		assert.Equal(t, 0, ft.requestCount(), "参数校验失败不应发起请求")
	})

	t.Run("nil doc 拒绝", func(t *testing.T) {
		ft := &fakeTransport{}
		cli := newFakeClient(t, ft)
		err := cli.Index(context.Background(), "orders", "o-1", nil)
		require.ErrorIs(t, err, errs.ErrValidation)
		assert.Equal(t, 0, ft.requestCount())
	})

	t.Run("doc 序列化失败", func(t *testing.T) {
		ft := &fakeTransport{}
		cli := newFakeClient(t, ft)
		err := cli.Index(context.Background(), "orders", "o-1", map[string]any{"fn": func() {}})
		require.ErrorIs(t, err, errs.ErrValidation)
		assert.Equal(t, 0, ft.requestCount())
	})

	t.Run("400 带 ES 错误体翻译为 VALIDATION", func(t *testing.T) {
		ft := &fakeTransport{status: http.StatusBadRequest, body: `{
			"error": {"root_cause": [{"type": "mapper_parsing_exception", "reason": "failed to parse"}],
			          "type": "mapper_parsing_exception", "reason": "failed to parse"},
			"status": 400}`}
		cli := newFakeClient(t, ft)
		err := cli.Index(context.Background(), "orders", "o-1", orderDoc{})
		require.Error(t, err)
		e, ok := errs.From(err)
		require.True(t, ok)
		assert.Equal(t, errs.CodeValidation, e.Code)
		assert.Contains(t, err.Error(), "mapper_parsing_exception: failed to parse")
		assert.Contains(t, err.Error(), "opensearch.Index index=orders id=o-1")
		ft.assertAllClosed(t)
	})

	t.Run("500 翻译为 DB", func(t *testing.T) {
		ft := &fakeTransport{status: http.StatusInternalServerError, body: `{"error":"internal","status":500}`}
		cli := newFakeClient(t, ft)
		err := cli.Index(context.Background(), "orders", "o-1", orderDoc{})
		require.ErrorIs(t, err, errs.ErrDB)
		ft.assertAllClosed(t)
	})
}

// TestGet 走完整 SDK 链路验证文档读取：命中、未命中、状态码翻译
// 与 _source 反序列化失败路径，并断言响应体必被关闭。
func TestGet(t *testing.T) {
	ctx := context.Background()

	t.Run("命中", func(t *testing.T) {
		ft := &fakeTransport{status: http.StatusOK, body: `{
			"_index": "orders", "_id": "o-1", "found": true,
			"_source": {"id": "o-1", "title": "t", "n": 7}}`}
		cli := newFakeClient(t, ft)
		var dest orderDoc
		found, err := Get[orderDoc](ctx, cli, "orders", "o-1", &dest)
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, orderDoc{ID: "o-1", Title: "t", N: 7}, dest)

		req := ft.firstRequest(t)
		assert.Equal(t, http.MethodGet, req.Method)
		assert.Equal(t, "/orders/_doc/o-1", req.Path)
		ft.assertAllClosed(t)
	})

	t.Run("404 未命中返回 false nil", func(t *testing.T) {
		ft := &fakeTransport{status: http.StatusNotFound, body: `{"_index":"orders","_id":"o-2","found":false}`}
		cli := newFakeClient(t, ft)
		var dest orderDoc
		found, err := Get[orderDoc](ctx, cli, "orders", "o-2", &dest)
		require.NoError(t, err)
		assert.False(t, found)
		ft.assertAllClosed(t)
	})

	t.Run("200 但 found=false", func(t *testing.T) {
		ft := &fakeTransport{status: http.StatusOK, body: `{"found":false}`}
		cli := newFakeClient(t, ft)
		found, err := Get[orderDoc](ctx, cli, "orders", "o-3", new(orderDoc))
		require.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("401 翻译为 UNAUTHORIZED", func(t *testing.T) {
		ft := &fakeTransport{status: http.StatusUnauthorized, body: `{"error":"unauthorized","status":401}`}
		cli := newFakeClient(t, ft)
		_, err := Get[orderDoc](ctx, cli, "orders", "o-1", new(orderDoc))
		require.ErrorIs(t, err, errs.ErrUnauthorized)
		ft.assertAllClosed(t)
	})

	t.Run("500 翻译为 DB", func(t *testing.T) {
		ft := &fakeTransport{status: http.StatusInternalServerError, body: `{"error":"boom","status":500}`}
		cli := newFakeClient(t, ft)
		_, err := Get[orderDoc](ctx, cli, "orders", "o-1", new(orderDoc))
		require.ErrorIs(t, err, errs.ErrDB)
		assert.Contains(t, err.Error(), "opensearch.Get index=orders id=o-1")
		ft.assertAllClosed(t)
	})

	t.Run("网络错误翻译为 DB", func(t *testing.T) {
		ft := &fakeTransport{failErr: errors.New("connection refused")}
		cli := newFakeClient(t, ft)
		_, err := Get[orderDoc](ctx, cli, "orders", "o-1", new(orderDoc))
		require.ErrorIs(t, err, errs.ErrDB)
		assert.Contains(t, err.Error(), "connection refused")
	})

	t.Run("_source 反序列化失败翻译为 DB", func(t *testing.T) {
		ft := &fakeTransport{status: http.StatusOK, body: `{
			"found": true, "_source": {"id": 123, "title": "t"}}`}
		cli := newFakeClient(t, ft)
		found, err := Get[orderDoc](ctx, cli, "orders", "o-1", new(orderDoc))
		require.ErrorIs(t, err, errs.ErrDB)
		assert.True(t, found, "文档本身已命中，仅类型不匹配")
		ft.assertAllClosed(t)
	})

	t.Run("参数校验", func(t *testing.T) {
		ft := &fakeTransport{}
		cli := newFakeClient(t, ft)
		_, err := Get[orderDoc](ctx, cli, "", "o-1", new(orderDoc))
		require.ErrorIs(t, err, errs.ErrValidation)
		_, err = Get[orderDoc](ctx, cli, "orders", "", new(orderDoc))
		require.ErrorIs(t, err, errs.ErrValidation)
		_, err = Get[orderDoc](ctx, cli, "orders", "o-1", nil)
		require.ErrorIs(t, err, errs.ErrValidation)
		assert.Equal(t, 0, ft.requestCount())
	})
}

// TestDelete 走完整 SDK 链路验证文档删除：成功、404 幂等、
// 状态码翻译与参数校验，并断言响应体必被关闭。
func TestDelete(t *testing.T) {
	ctx := context.Background()

	t.Run("成功", func(t *testing.T) {
		ft := &fakeTransport{status: http.StatusOK, body: `{"_index":"orders","_id":"o-1","result":"deleted"}`}
		cli := newFakeClient(t, ft)
		require.NoError(t, cli.Delete(ctx, "orders", "o-1"))

		req := ft.firstRequest(t)
		assert.Equal(t, http.MethodDelete, req.Method)
		assert.Equal(t, "/orders/_doc/o-1", req.Path)
		ft.assertAllClosed(t)
	})

	t.Run("404 幂等返回 nil", func(t *testing.T) {
		ft := &fakeTransport{status: http.StatusNotFound, body: `{"result":"not_found"}`}
		cli := newFakeClient(t, ft)
		require.NoError(t, cli.Delete(ctx, "orders", "missing"))
		ft.assertAllClosed(t)
	})

	t.Run("400 翻译为 VALIDATION", func(t *testing.T) {
		ft := &fakeTransport{status: http.StatusBadRequest, body: `{"error":"bad","status":400}`}
		cli := newFakeClient(t, ft)
		err := cli.Delete(ctx, "orders", "o-1")
		require.ErrorIs(t, err, errs.ErrValidation)
		ft.assertAllClosed(t)
	})

	t.Run("500 翻译为 DB", func(t *testing.T) {
		ft := &fakeTransport{status: http.StatusInternalServerError, body: `{"error":"boom","status":500}`}
		cli := newFakeClient(t, ft)
		err := cli.Delete(ctx, "orders", "o-1")
		require.ErrorIs(t, err, errs.ErrDB)
		assert.Contains(t, err.Error(), "opensearch.Delete index=orders id=o-1")
		ft.assertAllClosed(t)
	})

	t.Run("网络错误翻译为 DB", func(t *testing.T) {
		ft := &fakeTransport{failErr: errors.New("dial tcp: refused")}
		cli := newFakeClient(t, ft)
		err := cli.Delete(ctx, "orders", "o-1")
		require.ErrorIs(t, err, errs.ErrDB)
	})

	t.Run("参数校验", func(t *testing.T) {
		ft := &fakeTransport{}
		cli := newFakeClient(t, ft)
		require.ErrorIs(t, cli.Delete(ctx, "", "o-1"), errs.ErrValidation)
		require.ErrorIs(t, cli.Delete(ctx, "orders", ""), errs.ErrValidation)
		assert.Equal(t, 0, ft.requestCount())
	})
}
