package batch

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// fakeConn 是 driver.Conn 的测试替身（与根包 client_test 同构，分包后各自持有）：
// 嵌入接口满足全部方法签名，仅覆盖 Batch 路径实际触达的方法。
type fakeConn struct {
	driver.Conn
	prepareErr  error
	prepareCall []string
}

func (f *fakeConn) PrepareBatch(_ context.Context, query string, _ ...driver.PrepareBatchOption) (driver.Batch, error) {
	f.prepareCall = append(f.prepareCall, query)
	if f.prepareErr != nil {
		return nil, f.prepareErr
	}
	return &fakeBatch{}, nil
}

// fakeBatch 是 driver.Batch 的空替身：嵌入接口满足签名，方法不应被调用。
type fakeBatch struct {
	driver.Batch
}
