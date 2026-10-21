package clickhouse

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fab-viewer/common-lib/common/bizerror"
	"github.com/fab-viewer/common-lib/storage/clickhouse/config"
)

// fakeConn 是 driver.Conn 的测试替身：嵌入接口满足全部方法签名，仅覆盖
// Client 实际转发的五个方法；未覆盖方法一旦被调用会因内嵌 nil 而 panic，
// 由测试失败暴露。
type fakeConn struct {
	driver.Conn
	execErr     error
	queryErr    error
	queryRows   driver.Rows
	pingErr     error
	closeErr    error
	prepareErr  error
	execCalls   []string
	queryCalls  []string
	prepareCall []string
	closed      bool
}

func (f *fakeConn) Exec(_ context.Context, query string, _ ...any) error {
	f.execCalls = append(f.execCalls, query)
	return f.execErr
}

func (f *fakeConn) Query(_ context.Context, query string, _ ...any) (driver.Rows, error) {
	f.queryCalls = append(f.queryCalls, query)
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	return f.queryRows, nil
}

func (f *fakeConn) Ping(context.Context) error { return f.pingErr }

func (f *fakeConn) Close() error {
	f.closed = true
	return f.closeErr
}

func (f *fakeConn) PrepareBatch(_ context.Context, query string, _ ...driver.PrepareBatchOption) (driver.Batch, error) {
	f.prepareCall = append(f.prepareCall, query)
	if f.prepareErr != nil {
		return nil, f.prepareErr
	}
	return &fakeBatch{}, nil
}

// fakeBatch 是 driver.Batch 的空替身：嵌入接口满足签名，方法不应被调用。
type fakeBatch struct{ driver.Batch }

func TestOpen(t *testing.T) {
	t.Run("配置非法返回 CodeValidation 且不拨号", func(t *testing.T) {
		_, err := Open(context.Background(), config.Config{Compression: "bogus"})
		var e *bizerror.BizError
		require.ErrorAs(t, err, &e)
		assert.Equal(t, bizerror.CodeValidation, e.Code)
	})

	t.Run("连接失败返回 CodeDB 带地址上下文", func(t *testing.T) {
		cfg := config.Config{Addrs: []string{"127.0.0.1:1"}, DialTimeout: 200 * time.Millisecond}
		_, err := Open(context.Background(), cfg)
		var e *bizerror.BizError
		require.ErrorAs(t, err, &e)
		assert.Equal(t, bizerror.CodeDB, e.Code)
		assert.Contains(t, err.Error(), "clickhouse.Open ping addrs=[127.0.0.1:1]")
	})
}

func TestClientExec(t *testing.T) {
	t.Run("成功并透传语句", func(t *testing.T) {
		conn := &fakeConn{}
		c := &Client{conn: conn}
		assert.NoError(t, c.Exec(context.Background(), "INSERT INTO t VALUES (1)"))
		assert.Equal(t, []string{"INSERT INTO t VALUES (1)"}, conn.execCalls)
	})

	t.Run("失败带查询预览上下文", func(t *testing.T) {
		c := &Client{conn: &fakeConn{execErr: errors.New("exec boom")}}
		err := c.Exec(context.Background(), "SELECT\n  id,\n  name\nFROM t")
		var e *bizerror.BizError
		require.ErrorAs(t, err, &e)
		assert.Equal(t, bizerror.CodeDB, e.Code)
		assert.Contains(t, err.Error(), "clickhouse.Exec query=SELECT id, name FROM t: exec boom")
	})
}

func TestClientQuery(t *testing.T) {
	t.Run("成功返回结果集", func(t *testing.T) {
		rows := &fakeRows{}
		c := &Client{conn: &fakeConn{queryRows: rows}}
		got, err := c.Query(context.Background(), "SELECT 1")
		require.NoError(t, err)
		assert.Same(t, rows, got)
	})

	t.Run("失败带查询上下文", func(t *testing.T) {
		c := &Client{conn: &fakeConn{queryErr: errors.New("query boom")}}
		rows, err := c.Query(context.Background(), "SELECT boom")
		var e *bizerror.BizError
		require.ErrorAs(t, err, &e)
		assert.Equal(t, bizerror.CodeDB, e.Code)
		assert.Nil(t, rows) // 失败时不产生结果集，无需关闭
		assert.Contains(t, err.Error(), "clickhouse.Query query=SELECT boom")
	})
}

func TestClientPingAndClose(t *testing.T) {
	t.Run("Ping 成功", func(t *testing.T) {
		assert.NoError(t, (&Client{conn: &fakeConn{}}).Ping(context.Background()))
	})
	t.Run("Ping 失败", func(t *testing.T) {
		err := (&Client{conn: &fakeConn{pingErr: errors.New("ping boom")}}).Ping(context.Background())
		assert.Contains(t, err.Error(), "clickhouse.Ping")
	})
	t.Run("Close 成功", func(t *testing.T) {
		conn := &fakeConn{}
		assert.NoError(t, (&Client{conn: conn}).Close())
		assert.True(t, conn.closed)
	})
	t.Run("Close 失败", func(t *testing.T) {
		err := (&Client{conn: &fakeConn{closeErr: errors.New("close boom")}}).Close()
		assert.Contains(t, err.Error(), "clickhouse.Close")
	})
	t.Run("Conn 逃生舱返回底层连接", func(t *testing.T) {
		conn := &fakeConn{}
		c := &Client{conn: conn}
		driverConn, ok := c.Conn().(*fakeConn)
		require.True(t, ok)
		assert.Same(t, conn, driverConn)
	})
}

func TestQueryPreview(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"原样保留", "SELECT 1", "SELECT 1"},
		{"压缩空白", "SELECT\n  id,\n  name\nFROM t", "SELECT id, name FROM t"},
		{"超长按 rune 截断", strings.Repeat("a", 100), strings.Repeat("a", queryPreviewMax) + "…"},
		{"空串", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, queryPreview(tt.in))
		})
	}
}
