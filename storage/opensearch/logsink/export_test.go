package logsink

import "time"

// WithClockForTest 注入时钟（仅测试编译期存在），用于断言按日索引名。
func (s *LogSink) WithClockForTest(now func() time.Time) *LogSink {
	c := *s
	c.now = now
	return &c
}
