package snowflake

import (
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var idPattern = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

func TestFormat(t *testing.T) {
	g, err := New(7)
	require.NoError(t, err)

	id := g.Next()
	assert.Regexp(t, idPattern, id, "三段点分隔")
	parts := strings.Split(id, ".")
	require.Len(t, parts, 3)
	assert.Len(t, parts[0], 13, "时间戳段为 13 位 Unix 毫秒")
	assert.Equal(t, "7", parts[1], "节点段为构造时节点号")
	n, err := strconv.ParseInt(parts[2], 10, 64)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, int64(0))
	assert.LessOrEqual(t, n, maxSeq)
}

func TestNewInvalidNode(t *testing.T) {
	_, err := New(1024)
	assert.ErrorIs(t, err, ErrInvalidNode)
	_, err = New(0)
	assert.NoError(t, err)
}

func TestSetNode(t *testing.T) {
	t.Cleanup(func() { _ = SetNode(0) })

	require.NoError(t, SetNode(23))
	assert.Equal(t, "23", strings.Split(Next(), ".")[1])
}

func TestDefaultNextWithoutSetNode(t *testing.T) {
	defaultMu.Lock()
	saved := defaultG
	defaultG = nil
	defaultMu.Unlock()
	t.Cleanup(func() {
		defaultMu.Lock()
		defaultG = saved
		defaultMu.Unlock()
	})

	assert.Regexp(t, idPattern, Next(), "未 SetNode 时节点为 0 也可用")
	assert.Equal(t, "0", strings.Split(Next(), ".")[1])
}

func TestUniquenessConcurrent(t *testing.T) {
	g, err := New(42)
	require.NoError(t, err)

	seen := make(map[string]struct{}, 8192)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 1024 {
				id := g.Next()
				mu.Lock()
				seen[id] = struct{}{}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	assert.Len(t, seen, 8*1024, "并发 8192 个 ID 不得重复")
}

func TestSameMillisecondSequence(t *testing.T) {
	fake := int64(1760000000000)
	var calls atomic.Int64
	// 计数式假时钟：前 4097 次读数落在同一毫秒，之后前进 1ms——
	// 第 4097 个 ID 触发序列耗尽自旋分支
	nowFn = func() int64 {
		if calls.Add(1) <= maxSeq+1 {
			return fake
		}
		return fake + 1
	}
	t.Cleanup(func() { nowFn = func() int64 { return time.Now().UnixMilli() } })

	g, err := New(1)
	require.NoError(t, err)

	const total = maxSeq + 2 // 4096 个同毫秒 + 1 个溢出自旋后
	seen := make(map[string]struct{}, total)
	atFake, afterFake := 0, 0
	for range total {
		id := g.Next()
		seen[id] = struct{}{}
		switch mustParseTS(t, id) {
		case fake:
			atFake++
		case fake + 1:
			afterFake++
		}
	}
	assert.Equal(t, int(maxSeq)+1, atFake, "冻结毫秒内发出完整序列段（seq 0..4095 共 4096 个）")
	assert.Equal(t, 1, afterFake, "序列耗尽后自旋进入下一毫秒")
	assert.Len(t, seen, int(total), "全部 ID 唯一")
}

func TestClockRollbackWaits(t *testing.T) {
	real := int64(1760000000000)
	current := real
	nowFn = func() int64 { return current }
	t.Cleanup(func() { nowFn = func() int64 { return time.Now().UnixMilli() } })

	g, err := New(2)
	require.NoError(t, err)
	first := g.Next()
	require.Equal(t, real, mustParseTS(t, first))

	// 时钟回拨 50ms，Next 应阻塞等待时间追平后返回（不重号、不回退）
	go func() {
		for range 5 {
			current += 10 // 逐步追平
			time.Sleep(1 * time.Millisecond)
		}
	}()
	second := g.Next()
	assert.GreaterOrEqual(t, mustParseTS(t, second), real,
		"回拨期间生成的 ID 时间戳不得小于已发时间戳")
}

func mustParseTS(t *testing.T, id string) int64 {
	t.Helper()
	ts, err := strconv.ParseInt(strings.Split(id, ".")[0], 10, 64)
	require.NoError(t, err)
	return ts
}
