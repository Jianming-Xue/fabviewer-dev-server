package enrich

// 本文件只在测试编译期存在，向 enrich_test 包导出内部钩子，
// 不进入库的公共 API。

// SetRandRead 替换随机源（注入失败以测降级分支）。
var SetRandRead = func(f func([]byte) (int, error)) { randRead = f }

// RandReadForTest 返回当前随机源，用于测试后恢复。
var RandReadForTest = func() func([]byte) (int, error) { return randRead }

// RestoreRandRead 恢复随机源。
var RestoreRandRead = func(f func([]byte) (int, error)) { randRead = f }

// NewTxIDForTest 直接调用内部的 newTxID。
var NewTxIDForTest = newTxID
