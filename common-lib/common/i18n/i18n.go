package i18n

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/fab-viewer/common-lib/common/rctx"
)

// Language 是 BCP 47 风格的语言标签（主子标签-地区子标签，如 "zh-CN"、"en"）。
// 标签经规范化（NormalizeLanguage）后比较与存储，大小写与 "_" 写法不敏感。
type Language string

// 预置常用语言常量，其他语言（如 Language("ja-JP")）可自行定义。
const (
	ZhCN Language = "zh-CN"
	En   Language = "en"
)

// NormalizeLanguage 规范化语言标签：去首尾空白、"_" 转 "-"、主子标签小写、
// 2 字母子标签视为地区大写。"ZH_cn" / "zh_CN" / " zh-CN " 均得 "zh-CN"，
// "ZH" 得 "zh"，空串得空串。
func NormalizeLanguage(tag string) Language {
	parts := strings.Split(
		strings.ToLower(strings.TrimSpace(strings.ReplaceAll(tag, "_", "-"))), "-")
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) == 2 {
			parts[i] = strings.ToUpper(parts[i])
		}
	}
	return Language(strings.Join(parts, "-"))
}

// ParseAcceptLanguage 解析标准 Accept-Language 头（RFC 9110），返回按 q 值
// 降序排列的候选语言。q 缺省按 1、q=0 表示不可接受（剔除）、"*" 与非法条目
// 跳过，q 相同保持原始顺序，重复语言只保留优先级最高的一次；空头返回 nil。
func ParseAcceptLanguage(header string) []Language {
	type item struct {
		lang Language
		q    float64
	}
	entries := strings.Split(header, ",")
	items := make([]item, 0, len(entries))
	for _, entry := range entries {
		lang, q, ok := parseEntry(entry)
		if !ok {
			continue
		}
		items = append(items, item{lang: lang, q: q})
	}
	if len(items) == 0 {
		return nil
	}
	sort.SliceStable(items, func(a, b int) bool { return items[a].q > items[b].q })

	seen := make(map[Language]struct{}, len(items))
	langs := make([]Language, 0, len(items))
	for _, it := range items {
		if _, dup := seen[it.lang]; dup {
			continue
		}
		seen[it.lang] = struct{}{}
		langs = append(langs, it.lang)
	}
	return langs
}

// parseEntry 解析单个条目（如 "zh-CN;q=0.9"）为规范化语言与质量值。
// 无效条目（空标签、"*"、非法字符、q 语法错误、q 越界或为 0）返回 false；
// q 以外的参数（如 "v=1"）按 RFC 允许存在，忽略不参与解析。
func parseEntry(entry string) (Language, float64, bool) {
	tag, rest, _ := strings.Cut(entry, ";")
	lang := NormalizeLanguage(tag)
	if !validLang(lang) {
		return "", 0, false
	}
	q := 1.0
	for _, param := range strings.Split(rest, ";") {
		name, value, found := strings.Cut(strings.TrimSpace(param), "=")
		if !found || !strings.EqualFold(name, "q") {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || v < 0 || v > 1 {
			return "", 0, false
		}
		q = v
	}
	if q == 0 {
		return "", 0, false
	}
	return lang, q, true
}

// validLang 校验规范化后的标签：非空、非 "*"、每段 1-8 个字母数字。
func validLang(lang Language) bool {
	if lang == "" || lang == "*" {
		return false
	}
	for _, part := range strings.Split(string(lang), "-") {
		if len(part) < 1 || len(part) > 8 {
			return false
		}
		for _, r := range part {
			if !isLangRune(r) {
				return false
			}
		}
	}
	return true
}

// isLangRune 判定标签中允许的字符（字母与数字）。
func isLangRune(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
}

// baseOf 取语言的基础子标签："zh-CN" → "zh"；无子标签时第二返回值为 false。
func baseOf(lang Language) (Language, bool) {
	if i := strings.IndexByte(string(lang), '-'); i > 0 {
		return lang[:i], true
	}
	return "", false
}

// Bundles 是多语言消息集：语言 → key → 模板。只承载机制，译文内容完全由
// 业务服务自行注册。并发安全（RWMutex）：启动期一次注册、服务期并发查询
// 是常规用法，运行期增补也安全。
type Bundles struct {
	mu          sync.RWMutex
	defaultLang Language
	messages    map[Language]map[string]string
}

// NewBundles 创建消息集并指定默认语言。默认语言是回退链的终点，通常取
// 业务服务的开发语言（未注册译文的 key 原样输出，开发语言即兜底文案）。
func NewBundles(defaultLang Language) *Bundles {
	return &Bundles{
		defaultLang: NormalizeLanguage(string(defaultLang)),
		messages:    make(map[Language]map[string]string),
	}
}

// Register 按语言合并注册消息集：同 key 后注册覆盖先注册，不同 key 共存
// ——模块语言包（messages_<语言>.go + i18nerr 生成的 init 注册）编译期
// 注册进本模块字典。入参被复制（注册后修改原 map 不生效），lang 自动
// 规范化。key 即模块码（见包文档与 cmd/i18nerr），模板用 {0} {1} 位置
// 占位符承接参数；字典随模块包走（每模块一个），同 key 跨模块互不冲突。
func (b *Bundles) Register(lang Language, msgs map[string]string) {
	cp := make(map[string]string, len(msgs))
	for k, v := range msgs {
		cp[k] = v
	}
	b.mu.Lock()
	l := NormalizeLanguage(string(lang))
	if b.messages[l] == nil {
		b.messages[l] = make(map[string]string, len(cp))
	}
	for k, v := range cp {
		b.messages[l][k] = v
	}
	b.mu.Unlock()
}

// DefaultLang 返回默认语言（规范化后）。
func (b *Bundles) DefaultLang() Language {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.defaultLang
}

// T 按请求上下文语言翻译：语言取 rctx.GetRuntimeContext(ctx).FromLang()（由
// middleware.FillRuntimeContext 解析 Accept-Language 注入；未注入时
// 为空串，走默认语言回退），查找链与 TOf 相同。
func (b *Bundles) T(ctx context.Context, key string, args ...any) string {
	return b.TOf(Language(rctx.GetRuntimeContext(ctx).FromLang()), key, args...)
}

// TOf 按指定语言翻译 key。查找链：该语言精确命中 → 基础语言（zh-CN → zh；
// 只收窄不放宽：请求 zh 不会命中注册在 zh-CN 的译文）→ 默认语言 → key
// 原文。模板用位置占位符 {0} {1} ...（各语言占位符在句中位置可不同），
// args 按调用序替换（同 bizerror.InsertArgs 语义）。
func (b *Bundles) TOf(lang Language, key string, args ...any) string {
	lang = NormalizeLanguage(string(lang))
	msg := b.lookup(lang, key)
	if msg == "" {
		if base, ok := baseOf(lang); ok {
			msg = b.lookup(base, key)
		}
	}
	if msg == "" {
		msg = b.lookup(b.DefaultLang(), key)
	}
	if msg == "" {
		return key
	}
	return InsertArgs(msg, args...)
}

// InsertArgs 将模板中的 {0} {1} ... 按序替换为参数值，返回替换后的文案
// （无占位符或参数多余时忽略）。与 bizerror.BizError.InsertArgs 同语义
// （两包互不依赖，各自实现）：拿到模板 string 后可在调用点自行替换，
// 供 NewMessage 不带参取模板后的补充插参等场景使用。
func InsertArgs(template string, args ...any) string {
	for i, a := range args {
		template = strings.ReplaceAll(template,
			"{"+strconv.Itoa(i)+"}", fmt.Sprint(a))
	}
	return template
}

// lookup 读单条译文，未命中返回空串（空译文视同未注册，走回退链）。
func (b *Bundles) lookup(lang Language, key string) string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.messages[lang][key]
}
