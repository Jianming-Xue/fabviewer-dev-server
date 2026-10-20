package snowflake

import (
	"errors"
	"strconv"
	"sync"
	"time"
)

// epochMs 是自定义纪元（2026-01-01T00:00:00Z 的毫秒值）：经典雪花布局的
// 41 位相对时间从此起算，可用到约 2096 年。ID 字符串中的时间戳段为绝对
// Unix 毫秒（13 位，可读且全局可排序）。
const epochMs int64 = 1767225600000

// 位布局：41 位时间戳 + 10 位节点 + 12 位序列。
const (
	seqBits  = 12
	nodeBits = 10
	seqMask  = int64(-1) ^ (int64(-1) << seqBits)
	maxNode  = int64(-1) ^ (int64(-1) << nodeBits) // 0..1023
	maxSeq   = seqMask
)

// ErrInvalidNode 表示节点号超出 0..1023。
var ErrInvalidNode = errors.New("snowflake: node must be in [0, 1023]")

// Generator 是雪花 ID 生成器，并发安全。
type Generator struct {
	mu     sync.Mutex
	node   int64
	lastMs int64
	seq    int64
}

// New 创建节点号为 node（0..1023）的生成器。
func New(node uint16) (*Generator, error) {
	if int64(node) > maxNode {
		return nil, ErrInvalidNode
	}
	return &Generator{node: int64(node)}, nil
}

// Next 生成一个点分隔雪花 ID："<毫秒时间戳>.<节点>.<序列>"。
// 时钟回拨时阻塞等待时间追平（保证单调与不重号）。
func (g *Generator) Next() string {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := nowMs()
	for now < g.lastMs { // 时钟回拨：等待追平
		now = nowMs()
	}
	switch {
	case now == g.lastMs:
		g.seq++
		if g.seq > maxSeq { // 当前毫秒序列耗尽，自旋到下一毫秒
			for now <= g.lastMs {
				now = nowMs()
			}
			g.seq = 0
		}
	default:
		g.seq = 0
	}
	g.lastMs = now

	return strconv.FormatInt(now, 10) + "." +
		strconv.FormatInt(g.node, 10) + "." +
		strconv.FormatInt(g.seq, 10)
}

var (
	defaultMu sync.RWMutex
	defaultG  *Generator
)

// SetNode 设置全局默认生成器的节点号（进程内调用一次，多实例节点必须不同）。
func SetNode(node uint16) error {
	g, err := New(node)
	if err != nil {
		return err
	}
	defaultMu.Lock()
	defaultG = g
	defaultMu.Unlock()
	return nil
}

// Next 用全局默认生成器产出一个 ID；未 SetNode 时节点为 0
// （单实例可用，多实例务必先 SetNode）。
func Next() string {
	defaultMu.RLock()
	g := defaultG
	defaultMu.RUnlock()
	if g == nil {
		g = &Generator{}
	}
	return g.Next()
}

// nowFn 可在测试中替换以模拟时钟回拨。
var nowFn = func() int64 { return time.Now().UnixMilli() }

func nowMs() int64 { return nowFn() }
