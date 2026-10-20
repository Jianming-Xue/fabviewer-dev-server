package logx

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// numberedWriter 按编号滚动写文件：base 写满 MaxBytes 即滚动——最老的
// base.N 删除，base.N-1 → base.N 依此类推，base → base.1，随后开新 base
// 继续写。命名即 log、log.1、log.2 ...（log.N），保留最近 Keep 份。
// 目录不存在时自动创建；rotate 中个别改名失败（文件被外部占用等）忽略，
// 保证日志链路不中断。
type numberedWriter struct {
	mu       sync.Mutex
	path     string
	maxBytes int64
	keep     int
	f        *os.File
	size     int64
}

// newNumberedWriter 打开（或创建）base 文件，maxBytes<=0 或 keep<=0 由
// 调用方兜底默认值。
func newNumberedWriter(path string, maxBytes int64, keep int) (*numberedWriter, error) {
	w := &numberedWriter{path: path, maxBytes: maxBytes, keep: keep}
	if err := w.open(); err != nil {
		return nil, err
	}
	return w, nil
}

// open 以追加模式打开当前文件并记录既有大小（重启续写不重复滚动）。
func (w *numberedWriter) open() error {
	if dir := filepath.Dir(w.path); dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("logx: 创建日志目录 %s 失败: %w", dir, err)
		}
	}
	f, err := os.OpenFile(w.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640) //nolint:gosec // 日志文件供采集器（filebeat 等）同组读取，0640 最小开放
	if err != nil {
		w.f = nil
		return fmt.Errorf("logx: 打开日志文件 %s 失败: %w", w.path, err)
	}
	// 追加模式下末位偏移即既有大小；Seek 失败按 0 处理（最坏延迟一次滚动）
	size, _ := f.Seek(0, io.SeekEnd)
	w.f, w.size = f, size
	return nil
}

// Write 写入并在超限时滚动后落盘。滚动为尽力而为：改名/重开失败时
// 置空 fd 由本次 Write 报错，绝不写已关闭的旧 fd。
func (w *numberedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.size > 0 && w.size+int64(len(p)) > w.maxBytes {
		w.rotate()
		if w.f == nil {
			return 0, fmt.Errorf("logx: 滚动后重开 %s 失败", w.path)
		}
	}
	n, err := w.f.Write(p)
	w.size += int64(n)
	return n, err
}

// Sync 落盘当前文件。
func (w *numberedWriter) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return fmt.Errorf("logx: 日志文件 %s 不可用", w.path)
	}
	return w.f.Sync()
}

// Close 关闭当前文件。
func (w *numberedWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return fmt.Errorf("logx: 日志文件 %s 不可用", w.path)
	}
	return w.f.Close()
}

// rotate 执行一次编号滚动：删 base.Keep，base.Keep-1..base.1 逐个 +1，
// base 改名 base.1，重开新 base；改名失败（如被占用）也要尝试重开原文件
// 继续写，保日志不断。失败时 f 置空、由调用方报错。
func (w *numberedWriter) rotate() {
	_ = w.f.Close()
	_ = os.Remove(w.backup(w.keep))
	for i := w.keep - 1; i >= 1; i-- {
		_ = os.Rename(w.backup(i), w.backup(i+1))
	}
	if err := os.Rename(w.path, w.backup(1)); err != nil {
		// 改名失败：重开原文件继续写（打开已存在文件不要求目录写权限）
		_ = w.open()
		return
	}
	_ = w.open()
}

// backup 返回第 i 份留档路径（base.i）。
func (w *numberedWriter) backup(i int) string {
	return fmt.Sprintf("%s.%d", w.path, i)
}
