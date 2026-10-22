package response_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fab-viewer/common-lib/api/internal/testutil"
	"github.com/fab-viewer/common-lib/api/response"
	"github.com/fab-viewer/common-lib/common/paging"
	"github.com/fab-viewer/common-lib/common/rctx"
)

func TestOK(t *testing.T) {
	r := testutil.NewTestEngine()
	r.GET("/ok", func(c *gin.Context) {
		response.OK(c, map[string]any{"k": "v"})
	})

	w := testutil.DoGet(r, "/ok")
	require.Equal(t, http.StatusOK, w.Code)

	b := testutil.Decode(t, w)
	assert.Equal(t, "OK", b.Code, "业务 code 默认 OK")
	assert.True(t, b.Success)
	assert.Empty(t, b.ErrMsg, "成功时 errMsg 默认空串（字段保留，同构）")
	assert.JSONEq(t, `{"k":"v"}`, string(b.Data), "data 由 handler 给定")
	assert.Empty(t, b.TxID, "未启用 runtimecontext 中间件时为空")
}

func TestOKCollection(t *testing.T) {
	r := testutil.NewTestEngine()
	r.GET("/list", func(c *gin.Context) {
		response.OK(c, []string{"a", "b"})
	})

	b := testutil.Decode(t, testutil.DoGet(r, "/list"))
	assert.JSONEq(t, `["a","b"]`, string(b.Data), "multi 对象集合直接作为 data")
}

func TestOKPageableAsData(t *testing.T) {
	r := testutil.NewTestEngine()
	r.GET("/page", func(c *gin.Context) {
		response.OK(c, paging.NewPageable[string](
			paging.PageRequest{PageIndex: 2, PageSize: 2}, 5, []string{"a", "b"}))
	})

	b := testutil.Decode(t, testutil.DoGet(r, "/page"))
	assert.JSONEq(t,
		`{"pageIndex":2,"pageSize":2,"totalSize":5,"totalPage":3,"data":["a","b"]}`, string(b.Data),
		"分页 pageable 整体作为 data")
}

func TestOKWithErrMsg(t *testing.T) {
	r := testutil.NewTestEngine()
	r.GET("/ok", func(c *gin.Context) {
		response.OK(c, nil, response.WithErrMsg("partial success"))
	})

	b := testutil.Decode(t, testutil.DoGet(r, "/ok"))
	assert.True(t, b.Success)
	assert.Equal(t, "partial success", b.ErrMsg, "成功响应可经 WithErrMsg 补充提示")
	assert.Equal(t, "null", string(b.Data), "contents 为 nil 时序列化为 null")
}

func TestOKCarriesTxID(t *testing.T) {
	r := testutil.NewTestEngine()
	r.GET("/ok", func(c *gin.Context) {
		c.Request = c.Request.WithContext(
			rctx.SetRuntimeContext(c.Request.Context(), rctx.RuntimeContext{}.WithID("tx-123")))
		response.OK(c, nil)
	})

	b := testutil.Decode(t, testutil.DoGet(r, "/ok"))
	assert.Equal(t, "tx-123", b.TxID, "txId 取 RuntimeContext 记录的事务 ID")
}

func TestFail(t *testing.T) {
	tests := []struct {
		name       string
		errMsg     string
		opts       []response.Option
		wantCode   string
		wantErrMsg string
	}{
		{
			name:       "默认 code 0",
			errMsg:     "boom",
			wantCode:   "FAIL",
			wantErrMsg: "boom",
		},
		{
			name:       "WithCode 换业务码",
			errMsg:     "user not found",
			opts:       []response.Option{response.WithCode("USER_NOT_FOUND")},
			wantCode:   "USER_NOT_FOUND",
			wantErrMsg: "user not found",
		},
		{
			name:       "空 errMsg 兜底公共错误 InternalError",
			errMsg:     "",
			wantCode:   "InternalError",
			wantErrMsg: "internal error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testutil.NewTestEngine()
			r.GET("/f", func(c *gin.Context) { response.Fail(c, tt.errMsg, tt.opts...) })

			w := testutil.DoGet(r, "/f")
			require.Equal(t, http.StatusOK, w.Code, "HTTP 状态码恒 200，与业务 code 解耦")

			b := testutil.Decode(t, w)
			assert.Equal(t, tt.wantCode, b.Code)
			assert.False(t, b.Success, "Fail 的 success 恒 false")
			assert.Equal(t, tt.wantErrMsg, b.ErrMsg, "errMsg 必填非空")
			assert.Equal(t, "null", string(b.Data), "Fail 的 data 恒 null")
		})
	}
}

func TestFailSkipsWhenWritten(t *testing.T) {
	r := testutil.NewTestEngine()
	r.GET("/f", func(c *gin.Context) {
		c.String(http.StatusOK, "first")
		response.Fail(c, "ignored")
	})

	w := testutil.DoGet(r, "/f")
	assert.Equal(t, "first", w.Body.String(), "已写过的响应不得被覆盖")
}

func TestOKSkipsWhenWritten(t *testing.T) {
	r := testutil.NewTestEngine()
	r.GET("/o", func(c *gin.Context) {
		c.String(http.StatusOK, "first")
		response.OK(c, "ignored")
	})

	w := testutil.DoGet(r, "/o")
	assert.Equal(t, "first", w.Body.String(), "已写过的响应不得被覆盖")
}

func TestWrap(t *testing.T) {
	// userVO 模拟 service 层 VO；userDTO 模拟 api 层出参对象。
	type userVO struct {
		Name      string
		Enabled   bool
		CreatedAt time.Time
		Extra     string // DTO 没有的字段，应被忽略
	}
	type userDTO struct {
		Name      string
		Enabled   int    // bool → int（1/0）
		CreatedAt string // time.Time → "yyyy-MM-dd HH:mm:ss"
	}

	t.Run("时间转字符串、布尔转数字", func(t *testing.T) {
		dto, err := response.Wrap[userDTO](userVO{
			Name:      "zhangsan",
			Enabled:   true,
			CreatedAt: time.Date(2026, 10, 21, 10, 0, 0, 0, time.Local),
		})
		require.NoError(t, err)
		assert.Equal(t, "zhangsan", dto.Name)
		assert.Equal(t, 1, dto.Enabled, "bool true → int 1")
		assert.Equal(t, "20261021100000", dto.CreatedAt, "time.Time → yyyyMMddHHmmss（与入参同格式）")
	})

	t.Run("false 转 0", func(t *testing.T) {
		dto, err := response.Wrap[userDTO](userVO{})
		require.NoError(t, err)
		assert.Equal(t, 0, dto.Enabled, "bool false → int 0")
		assert.Empty(t, dto.CreatedAt, "零值 time → 空串")
	})

	t.Run("大小写不一致按忽略大小写匹配", func(t *testing.T) {
		type srcVO struct {
			UserName string
		}
		type dstDTO struct {
			Username string
		}
		dto, err := response.Wrap[dstDTO](srcVO{UserName: "lisi"})
		require.NoError(t, err)
		assert.Equal(t, "lisi", dto.Username)
	})

	t.Run("无法转换时报错", func(t *testing.T) {
		type badVO struct {
			Level string
		}
		type badDTO struct {
			Level int
		}
		_, err := response.Wrap[badDTO](badVO{Level: "abc"})
		assert.Error(t, err, "非数字字符串转 int 应报错")
	})

	t.Run("嵌套 struct 与 slice 递归展开", func(t *testing.T) {
		type itemVO struct {
			Title string
			Done  bool
		}
		type orderVO struct {
			OrderNo int64 `json:"orderNo"`
			Items   []itemVO
			Buyer   *itemVO // 指针字段
		}
		type itemDTO struct {
			Title string
			Done  int
		}
		type orderDTO struct {
			OrderNo string `mapstructure:"orderNo"` // 整数 → 字符串
			Items   []itemDTO
			Buyer   *itemDTO
		}
		dto, err := response.Wrap[orderDTO](orderVO{
			OrderNo: 1792575466174,
			Items:   []itemVO{{Title: "a", Done: true}, {Title: "b"}},
			Buyer:   &itemVO{Title: "buyer"},
		})
		require.NoError(t, err)
		assert.Equal(t, "1792575466174", dto.OrderNo, "整数 → 字符串（json tag 命名）")
		require.Len(t, dto.Items, 2)
		assert.Equal(t, "a", dto.Items[0].Title)
		assert.Equal(t, 1, dto.Items[0].Done, "slice 内的 bool → int")
		assert.Equal(t, 0, dto.Items[1].Done)
		require.NotNil(t, dto.Buyer)
		assert.Equal(t, "buyer", dto.Buyer.Title, "指针字段解引用展开")
	})

	t.Run("map 字段与数组递归展开", func(t *testing.T) {
		type attrVO struct {
			Key   string
			Valid bool
		}
		type holderVO struct {
			Meta map[string]attrVO
			Tags [2]string
		}
		type holderDTO struct {
			Meta map[string]attrVO
			Tags []string
		}
		dto, err := response.Wrap[holderDTO](holderVO{
			Meta: map[string]attrVO{"a": {Key: "k1", Valid: true}},
			Tags: [2]string{"x", "y"},
		})
		require.NoError(t, err)
		require.Len(t, dto.Meta, 1)
		assert.Equal(t, "k1", dto.Meta["a"].Key, "map 值为 struct 时递归展开后仍可按 struct 接")
		assert.Equal(t, []string{"x", "y"}, dto.Tags, "数组转 slice")
	})

	t.Run("未导出字段跳过与 nil 指针", func(t *testing.T) {
		type inner struct{ Name string }
		type outerVO struct {
			Name  string
			inner string
			Sub   *inner // nil 指针保持 nil
		}
		type outerDTO struct {
			Name string
			Sub  *inner
		}
		dto, err := response.Wrap[outerDTO](outerVO{Name: "a", inner: "x"})
		require.NoError(t, err)
		assert.Equal(t, "a", dto.Name)
		assert.Nil(t, dto.Sub, "nil 指针不产生字段")
	})

	t.Run("零值 time 转空串", func(t *testing.T) {
		type tsVO struct {
			At time.Time
		}
		type tsDTO struct {
			At string
		}
		dto, err := response.Wrap[tsDTO](tsVO{})
		require.NoError(t, err)
		assert.Empty(t, dto.At, "零值时间没有业务含义，出参为空串")
	})

	t.Run("目标同为 time.Time 时原样保留", func(t *testing.T) {
		type bothVO struct {
			At time.Time
		}
		type bothDTO struct {
			At time.Time
		}
		now := time.Date(2026, 10, 21, 10, 0, 0, 0, time.Local)
		dto, err := response.Wrap[bothDTO](bothVO{At: now})
		require.NoError(t, err)
		assert.Equal(t, now, dto.At)
	})
}
