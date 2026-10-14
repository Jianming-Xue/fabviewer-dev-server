package clickhouse

import (
	"context"
	"strings"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/fab-viewer/common-lib/errs"
)

// queryPreviewMax 是错误上下文中查询文本的最大长度（rune 数），超长截断，
// 避免大 SQL 撑爆错误消息与日志。
const queryPreviewMax = 64

// Client 是 ClickHouse 的轻量客户端：内部持有 clickhouse-go/v2 原生连接
// （SDK 自带连接池），本库不创建任何后台 goroutine。各方法均为薄转发，
// 失败时返回带操作上下文的 errs.CodeDB 错误；SDK 完整能力经 Conn() 逃生舱获取。
type Client struct {
	conn driver.Conn
}

// Open 按配置建立 ClickHouse 连接，并 Ping 验证连通性（fail-fast：配置或网络
// 问题在启动期即暴露，而非首次查询时）。配置非法返回 errs.CodeValidation，
// 连接失败返回 errs.CodeDB。
func Open(ctx context.Context, cfg Config) (*Client, error) { //nolint:gocritic //hugeParam：按值传配置是计划 API 的人体工学约定，建连路径一次性开销可忽略
	opts, err := cfg.toOptions()
	if err != nil {
		return nil, err
	}
	conn, err := ch.Open(&opts)
	if err != nil {
		return nil, errs.Wrapf(err, errs.CodeDB, "clickhouse.Open addrs=%v db=%s", cfg.Addrs, cfg.DB)
	}
	if err := conn.Ping(ctx); err != nil {
		_ = conn.Close()
		return nil, errs.Wrapf(err, errs.CodeDB, "clickhouse.Open ping addrs=%v db=%s", cfg.Addrs, cfg.DB)
	}
	return &Client{conn: conn}, nil
}

// Exec 执行无结果集语句（DDL / INSERT / SET 等），args 支持 clickhouse.Named
// 具名参数与位置参数。
func (c *Client) Exec(ctx context.Context, query string, args ...any) error {
	if err := c.conn.Exec(ctx, query, args...); err != nil {
		return errs.Wrapf(err, errs.CodeDB, "clickhouse.Exec query=%s", queryPreview(query))
	}
	return nil
}

// Query 执行查询并返回 SDK 原生结果集，调用方必须 defer rows.Close()。
// 逐行扫描可用 rows.Scan/ScanStruct（ch tag）；整片填充推荐直接使用包级
// QuerySlice[T]。
func (c *Client) Query(ctx context.Context, query string, args ...any) (driver.Rows, error) {
	rows, err := c.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, errs.Wrapf(err, errs.CodeDB, "clickhouse.Query query=%s", queryPreview(query))
	}
	return rows, nil
}

// Ping 验证当前连接可用性。
func (c *Client) Ping(ctx context.Context) error {
	return wrapDB(c.conn.Ping(ctx), "clickhouse.Ping")
}

// Close 关闭客户端并释放底层连接池，幂等。
func (c *Client) Close() error {
	return wrapDB(c.conn.Close(), "clickhouse.Close")
}

// Conn 返回底层 SDK 连接（逃生舱）：需要本包未覆盖的 SDK 能力
// （ServerVersion/AsyncInsert/QueryFormat 等）时使用。
func (c *Client) Conn() driver.Conn { return c.conn }

// queryPreview 返回错误上下文用的查询预览：压缩连续空白并按 rune 截断。
func queryPreview(q string) string {
	preview := strings.Join(strings.Fields(q), " ")
	if r := []rune(preview); len(r) > queryPreviewMax {
		return string(r[:queryPreviewMax]) + "…"
	}
	return preview
}

// wrapDB 在 err 非 nil 时包装为带操作上下文的 errs.CodeDB 错误，nil 原样
// 透传（返回真正的 nil error，避免直接透传 *Error 造成 typed-nil）。
func wrapDB(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	return errs.Wrapf(err, errs.CodeDB, format, args...)
}
