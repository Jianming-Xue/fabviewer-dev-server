package batch

import (
	"testing"

	"go.uber.org/mock/gomock"
)

// 基准说明：SDK 层的反射（AppendStruct/ScanStruct 的 ch tag 填充）在单测中
// 由 fake/mock 替代，以下基准度量的是本包装层的固有开销（循环、阈值判断、
// 初始容量分配），作为版本间的回归基线；真实端到端吞吐以集成测试为准。
// 运行：go test -bench=. -benchmem ./storage/clickhouse/...

// BenchmarkBatchAppend 度量单行追加的阈值判断与计数开销（阈值设为大值避免触发发送）。
func BenchmarkBatchAppend(b *testing.B) {
	ctrl := gomock.NewController(b)
	defer ctrl.Finish()

	sender := NewMockBatchSender(ctrl)
	sender.EXPECT().AppendStruct(gomock.Any()).Return(nil).AnyTimes()
	batch := &Batch[chRow]{
		table:  "bench",
		size:   1 << 30, // 不触发自动发送，纯测单行开销
		sender: sender,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := batch.Append(chRow{ID: 1, Name: "n"}); err != nil {
			b.Fatal(err)
		}
	}
}
