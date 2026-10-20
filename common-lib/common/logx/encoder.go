package logx

import (
	"time"

	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

// Rec 是单条日志的完整信息，自定义格式器的原料。
type Rec struct {
	Level string    // 级别（info/warn/error/debug）
	Time  time.Time // 时间
	Msg   string    // 消息
	KV    []any     // 交替键值（含 With 派生字段；map 展开无序）
}

// Formatter 是业务自定义的行格式器：返回完整一行内容（不含换行，logx
// 补）。既可产出字符串格式，也可自行 json.Marshal 产出自定义 JSON——
// 文件出口与远程出口可各配各的格式。
type Formatter func(Rec) string

// LineEncoder 把 Formatter 适配为出口编码器：Config 的 StdoutEncoder、
// File.Encoder、RemoteEncoder 均可注入，实现"每个出口自己的格式"。
func LineEncoder(f Formatter) zapcore.Encoder {
	return &lineEncoder{f: f, MapObjectEncoder: zapcore.NewMapObjectEncoder()}
}

// lineEncoder 实现 zapcore.Encoder：内嵌 MapObjectEncoder 承接 With 路径
// 的字段累积（ObjectEncoder 协议），EncodeEntry 时与本次字段合并交给
// 业务 Formatter。
type lineEncoder struct {
	f Formatter
	*zapcore.MapObjectEncoder
}

// EncodeEntry 合并累积字段与本次字段，交业务 Formatter 产出行。
func (e *lineEncoder) EncodeEntry(ent zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) { //nolint:gocritic //hugeParam：zapcore.Encoder 接口约定按值
	all := make(map[string]any, len(e.Fields)+len(fields))
	for k, v := range e.Fields {
		all[k] = v
	}
	m := zapcore.NewMapObjectEncoder()
	for _, fd := range fields {
		fd.AddTo(m)
	}
	for k, v := range m.Fields {
		all[k] = v
	}

	kv := make([]any, 0, 2*len(all))
	for k, v := range all {
		kv = append(kv, k, v)
	}

	line := e.f(Rec{Level: ent.Level.String(), Time: ent.Time, Msg: ent.Message, KV: kv})
	b := linePool.Get()
	b.AppendString(line)
	b.AppendByte('\n')
	return b, nil
}

// Clone 克隆编码器（携带累积字段），zap With 派生路径要求。
func (e *lineEncoder) Clone() zapcore.Encoder {
	c := &lineEncoder{f: e.f, MapObjectEncoder: zapcore.NewMapObjectEncoder()}
	for k, v := range e.Fields {
		c.Fields[k] = v
	}
	return c
}

var linePool = buffer.NewPool()
