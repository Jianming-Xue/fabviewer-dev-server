package clickhouse

import (
	"context"
	"testing"
)

// 基准说明：SDK 层的反射（AppendStruct/ScanStruct 的 ch tag 填充）在单测中
// 由 fake/mock 替代，以下基准度量的是本包装层的固有开销（循环、阈值判断、
// 初始容量分配），作为版本间的回归基线；真实端到端吞吐以集成测试为准。
// 运行：go test -bench=. -benchmem ./storage/clickhouse/

// BenchmarkQuerySlice 度量包装层逐行扫描开销（1 万行 fakeRows）。
func BenchmarkQuerySlice(b *testing.B) {
	const rowCount = 10_000
	rows := make([]chRow, rowCount)
	for i := range rows {
		rows[i] = chRow{ID: int64(i), Name: "n"}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		// fakeRows 游标在闭包内，逐轮新建（构造开销极小，不影响度量结论）
		c := NewFromConn(&fakeConn{queryRows: rowsFrom(rows...)})
		if _, err := QuerySlice[chRow](context.Background(), c, "SELECT id, name FROM t"); err != nil {
			b.Fatal(err)
		}
	}
}
