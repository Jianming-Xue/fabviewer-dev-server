package logx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNumberedWriterRotation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	w, err := newNumberedWriter(path, 25, 2) // 25B 上限，留 2 份
	require.NoError(t, err)

	line := strings.Repeat("x", 10) + "\n" // 11B/行
	for range 6 {
		n, err := w.Write([]byte(line))
		require.NoError(t, err)
		require.Equal(t, len(line), n)
	}
	require.NoError(t, w.Close())

	names := lsNames(t, dir)
	assert.True(t, names["app.log"], "当前文件存在")
	assert.True(t, names["app.log.1"], "第一次滚动产生 .1")
	assert.True(t, names["app.log.2"], "第二次滚动产生 .2")
	assert.False(t, names["app.log.3"], "留档上限 2：最老被删")
}

func TestNumberedWriterAppendOnReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")

	w, err := newNumberedWriter(path, 25, 2)
	require.NoError(t, err)
	_, err = w.Write([]byte(strings.Repeat("a", 20) + "\n")) // 21B
	require.NoError(t, err)
	require.NoError(t, w.Close())

	w2, err := newNumberedWriter(path, 25, 2)
	require.NoError(t, err)
	_, err = w2.Write([]byte(strings.Repeat("b", 10) + "\n")) // 21+11>25 → 滚动
	require.NoError(t, err)
	require.NoError(t, w2.Close())

	assert.True(t, lsNames(t, dir)["app.log.1"], "重启续写按既有大小判断滚动")
}

func TestNumberedWriterAutoMkdir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deep", "nested", "app.log")
	w, err := newNumberedWriter(path, 100, 2)
	require.NoError(t, err)
	_, err = w.Write([]byte("hi\n"))
	require.NoError(t, err)
	require.NoError(t, w.Sync())
	require.NoError(t, w.Close())

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "hi\n", string(data))
}

func TestNumberedWriterOpenError(t *testing.T) {
	// 路径是目录 → 打开失败
	dir := t.TempDir()
	_, err := newNumberedWriter(dir, 10, 2)
	require.Error(t, err)
}

func lsNames(t *testing.T, dir string) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name()] = true
	}
	return names
}

func TestNumberedWriterMkdirDenied(t *testing.T) {
	parent := t.TempDir()
	require.NoError(t, os.Chmod(parent, 0o500))
	t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })

	_, err := newNumberedWriter(filepath.Join(parent, "sub", "app.log"), 10, 2)
	require.Error(t, err)
}

func TestNumberedWriterRotateRenameBlockedButReopenOK(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	// 预置同名目录占用 app.log.1：改名失败，但重开当前文件成功、日志不断
	require.NoError(t, os.Mkdir(path+".1", 0o755))

	w, err := newNumberedWriter(path, 10, 2)
	require.NoError(t, err)
	_, err = w.Write([]byte(strings.Repeat("x", 20) + "\n"))
	require.NoError(t, err, "改名失败时兜底重开继续写")
	require.NoError(t, w.Close())
}

func TestNumberedWriterRotateError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	w, err := newNumberedWriter(path, 10, 2)
	require.NoError(t, err)
	_, err = w.Write([]byte("seed\n")) // 先有内容，后续写才触发滚动判断
	require.NoError(t, err)
	// 目录只读（改名失败）+ 文件不可写（重开失败）→ Write 报错
	require.NoError(t, os.Chmod(dir, 0o500))
	require.NoError(t, w.f.Chmod(0o000))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	_, err = w.Write([]byte(strings.Repeat("x", 20) + "\n"))
	require.Error(t, err)
}

func TestNumberedWriterUnavailableSyncClose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	w, err := newNumberedWriter(path, 10, 2)
	require.NoError(t, err)
	_, err = w.Write([]byte("seed\n"))
	require.NoError(t, err)
	// 滚动失败后 fd 置空：Sync/Close 均报"不可用"而非 panic
	require.NoError(t, os.Chmod(dir, 0o500))
	require.NoError(t, w.f.Chmod(0o000))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	_, err = w.Write([]byte(strings.Repeat("x", 20) + "\n"))
	require.Error(t, err)
	assert.Error(t, w.Sync())
	assert.Error(t, w.Close())
}
