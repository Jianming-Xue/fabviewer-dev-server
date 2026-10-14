package api_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fab-viewer/common-lib/api"
	"github.com/fab-viewer/common-lib/errs"
	"github.com/fab-viewer/common-lib/paging"
)

// respBody 是响应体的测试解码目标，timestamp 用 time.Time 直接解码。
type respBody struct {
	Code      string          `json:"code"`
	Message   string          `json:"message"`
	Data      json.RawMessage `json:"data"`
	Details   map[string]any  `json:"details"`
	RequestID string          `json:"requestId"`
	Timestamp time.Time       `json:"timestamp"`
}

func newTestEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func doRequest(t *testing.T, r http.Handler, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), method, target, http.NoBody)
	r.ServeHTTP(w, req)
	return w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) respBody {
	t.Helper()
	var b respBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
	return b
}

func TestOK(t *testing.T) {
	r := newTestEngine()
	r.GET("/ok", func(c *gin.Context) {
		api.OK(c, map[string]any{"k": "v"})
	})

	w := doRequest(t, r, http.MethodGet, "/ok")
	require.Equal(t, http.StatusOK, w.Code)

	b := decode(t, w)
	assert.Equal(t, "OK", b.Code)
	assert.Equal(t, "success", b.Message)
	assert.JSONEq(t, `{"k":"v"}`, string(b.Data))
	assert.False(t, b.Timestamp.IsZero(), "timestamp 必填")
	assert.Empty(t, b.RequestID, "未启用 RequestID 中间件时为空且字段省略")
}

func TestOKCarriesRequestID(t *testing.T) {
	r := newTestEngine()
	r.GET("/ok", func(c *gin.Context) {
		c.Set(api.CtxRequestID, "rid-123")
		api.OK(c, nil)
	})

	b := decode(t, doRequest(t, r, http.MethodGet, "/ok"))
	assert.Equal(t, "rid-123", b.RequestID)
	assert.Empty(t, b.Data, "data 为 nil 时省略")
}

func TestCreated(t *testing.T) {
	r := newTestEngine()
	r.POST("/res", func(c *gin.Context) { api.Created(c, gin.H{"id": 1}) })

	w := doRequest(t, r, http.MethodPost, "/res")
	require.Equal(t, http.StatusCreated, w.Code)
	b := decode(t, w)
	assert.Equal(t, "created", b.Message)
	assert.JSONEq(t, `{"id":1}`, string(b.Data))
}

func TestNoContent(t *testing.T) {
	r := newTestEngine()
	r.DELETE("/res/1", func(c *gin.Context) { api.NoContent(c) })

	w := doRequest(t, r, http.MethodDelete, "/res/1")
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String(), "204 无响应体")
}

func TestOKPage(t *testing.T) {
	r := newTestEngine()
	r.GET("/list", func(c *gin.Context) {
		api.OKPage(c, paging.NewResult[string](
			paging.Query{Page: 2, Size: 2}, 5, []string{"a", "b"}))
	})

	b := decode(t, doRequest(t, r, http.MethodGet, "/list"))
	assert.Equal(t, "OK", b.Code)
	assert.JSONEq(t,
		`{"total":5,"page":2,"size":2,"items":["a","b"]}`, string(b.Data))
}

func TestOKPageEmptyItems(t *testing.T) {
	r := newTestEngine()
	r.GET("/empty", func(c *gin.Context) {
		api.OKPage(c, paging.NewResult[int](paging.Query{}, 0, nil))
	})

	b := decode(t, doRequest(t, r, http.MethodGet, "/empty"))
	assert.JSONEq(t, `{"total":0,"page":1,"size":20,"items":[]}`, string(b.Data),
		"空结果 items 必须是 [] 而非 null")
}

func TestFail(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		wantStatus   int
		wantCode     string
		wantMessage  string
		wantDetails  map[string]any
		rootLeakData bool
	}{
		{
			name:        "已知错误码按码表映射",
			err:         errs.New(errs.CodeNotFound, "user 42 not found"),
			wantStatus:  http.StatusNotFound,
			wantCode:    "NOT_FOUND",
			wantMessage: "user 42 not found",
		},
		{
			name:        "带明细",
			err:         errs.New(errs.CodeValidation, "参数非法").WithDetail("field", "email"),
			wantStatus:  http.StatusBadRequest,
			wantCode:    "VALIDATION_ERROR",
			wantMessage: "参数非法",
			wantDetails: map[string]any{"field": "email"},
		},
		{
			name:        "包装错误沿用外层语义",
			err:         errs.Wrap(errors.New("dial timeout"), errs.CodeDB, "query orders"),
			wantStatus:  http.StatusInternalServerError,
			wantCode:    "DB_ERROR",
			wantMessage: "query orders",
		},
		{
			name:        "未映射自定义码按 500",
			err:         errs.New(errs.Code("USER_DISABLED"), "账号已禁用"),
			wantStatus:  http.StatusInternalServerError,
			wantCode:    "USER_DISABLED",
			wantMessage: "账号已禁用",
		},
		{
			name:        "sql.ErrNoRows 转 404",
			err:         sql.ErrNoRows,
			wantStatus:  http.StatusNotFound,
			wantCode:    "NOT_FOUND",
			wantMessage: "resource not found",
		},
		{
			name:        "context.Canceled 转 499",
			err:         context.Canceled,
			wantStatus:  api.StatusClientClosedRequest,
			wantCode:    "INTERNAL_ERROR",
			wantMessage: "client canceled request",
		},
		{
			name:        "未知错误根因不泄漏",
			err:         errors.New("secret token xyz"),
			wantStatus:  http.StatusInternalServerError,
			wantCode:    "INTERNAL_ERROR",
			wantMessage: "internal error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestEngine()
			r.GET("/f", func(c *gin.Context) { api.Fail(c, tt.err) })

			w := doRequest(t, r, http.MethodGet, "/f")
			require.Equal(t, tt.wantStatus, w.Code)

			b := decode(t, w)
			assert.Equal(t, tt.wantCode, b.Code)
			assert.Equal(t, tt.wantMessage, b.Message)
			if tt.wantDetails != nil {
				assert.Equal(t, tt.wantDetails, b.Details)
			} else {
				assert.Empty(t, b.Details)
			}
			if tt.err != nil {
				assert.NotContains(t, w.Body.String(), "secret token",
					"根因信息不得进入响应体")
			}
		})
	}
}

func TestWriteSkipsWhenWritten(t *testing.T) {
	r := newTestEngine()
	r.GET("/w", func(c *gin.Context) {
		c.String(http.StatusOK, "first")
		api.Write(c, http.StatusOK, &api.Body{Code: errs.CodeOK, Message: "second"})
	})

	w := doRequest(t, r, http.MethodGet, "/w")
	assert.Equal(t, "first", w.Body.String(), "已写过的响应不得被覆盖")
}

func TestNoContentSkipsWhenWritten(t *testing.T) {
	r := newTestEngine()
	r.GET("/nc", func(c *gin.Context) {
		c.String(http.StatusOK, "first")
		api.NoContent(c)
	})

	w := doRequest(t, r, http.MethodGet, "/nc")
	assert.Equal(t, "first", w.Body.String())
}

func TestFailSkipsWhenWritten(t *testing.T) {
	r := newTestEngine()
	r.GET("/f", func(c *gin.Context) {
		c.String(http.StatusOK, "first")
		api.Fail(c, errs.New(errs.CodeNotFound, "ignored"))
	})

	w := doRequest(t, r, http.MethodGet, "/f")
	assert.Equal(t, "first", w.Body.String())
}

func TestStatusFor(t *testing.T) {
	assert.Equal(t, http.StatusOK, api.StatusFor(errs.CodeOK))
	assert.Equal(t, http.StatusNotFound, api.StatusFor(errs.CodeNotFound))
	assert.Equal(t, http.StatusBadGateway, api.StatusFor(errs.CodeUpstream))
	assert.Equal(t, http.StatusInternalServerError, api.StatusFor(errs.Code("CUSTOM_X")),
		"未映射码按 500")
}
