package i18n_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/fab-viewer/common-lib/common/i18n"
	"github.com/fab-viewer/common-lib/common/rctx"
)

func TestNormalizeLanguage(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want i18n.Language
	}{
		{"原样保持", "zh-CN", i18n.ZhCN},
		{"地区小写转大写", "zh-cn", i18n.ZhCN},
		{"全大写", "ZH-CN", i18n.ZhCN},
		{"下划线写法", "zh_CN", i18n.ZhCN},
		{"带首尾空白", "  zh-CN\t", i18n.ZhCN},
		{"仅主子标签", "ZH", i18n.Language("zh")},
		{"空串", "", i18n.Language("")},
		{"三段式脚本标签", "zh-Hans-CN", i18n.Language("zh-hans-CN")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, i18n.NormalizeLanguage(tt.in))
		})
	}
}

func TestParseAcceptLanguage(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   []i18n.Language
	}{
		{"空串返回 nil", "", nil},
		{"单一语言", "zh-CN", []i18n.Language{i18n.ZhCN}},
		{"无 q 按声明顺序", "zh-CN,en", []i18n.Language{i18n.ZhCN, i18n.En}},
		{"按 q 值降序", "zh-CN,zh;q=0.9,en;q=0.8",
			[]i18n.Language{i18n.ZhCN, i18n.Language("zh"), i18n.En}},
		{"q 缺省为 1 优先", "en;q=0.5,zh-CN", []i18n.Language{i18n.ZhCN, i18n.En}},
		{"q 相同保持原始顺序", "en,fr", []i18n.Language{i18n.En, i18n.Language("fr")}},
		{"q=0 剔除", "fr;q=0,en", []i18n.Language{i18n.En}},
		{"通配符跳过", "*,en", []i18n.Language{i18n.En}},
		{"非法字符条目跳过", "zh-CN;v=1,zh$$!!,en", []i18n.Language{i18n.ZhCN, i18n.En}},
		{"空段子标签剔除", "zh-,en", []i18n.Language{i18n.En}},
		{"段超 8 字符剔除", "abcdefghi,en", []i18n.Language{i18n.En}},
		{"q 语法错误整条剔除", "fr;q=oops,en", []i18n.Language{i18n.En}},
		{"q 越界整条剔除", "fr;q=1.5,en", []i18n.Language{i18n.En}},
		{"下划线规范化", "zh_CN", []i18n.Language{i18n.ZhCN}},
		{"大小写规范化", "EN-us", []i18n.Language{i18n.Language("en-US")}},
		{"大写 q 属性", "en;Q=0.5,zh", []i18n.Language{i18n.Language("zh"), i18n.En}},
		{"重复语言保最高优先", "en,en;q=0.3", []i18n.Language{i18n.En}},
		{"全非法返回 nil", ",,;;q=", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, i18n.ParseAcceptLanguage(tt.header))
		})
	}
}

// newTestBundles 构造测试用消息集：默认 en（译文未注册，key 原文回退），
// 按推荐方式注册基础语言 zh（请求 zh-CN/zh-TW 收窄命中）。
func newTestBundles() *i18n.Bundles {
	b := i18n.NewBundles(i18n.En)
	b.Register(i18n.Language("zh"), map[string]string{
		"user not found":     "用户不存在",
		"user {0} not found": "用户 {0} 不存在",
		"resource not found": "资源不存在",
		"empty translation":  "",
	})
	return b
}

func TestBundlesRegister(t *testing.T) {
	t.Run("注册时规范化语言标签", func(t *testing.T) {
		b := i18n.NewBundles(i18n.En)
		b.Register("ZH_cn", map[string]string{"k": "值"})
		assert.Equal(t, "值", b.TOf(i18n.ZhCN, "k"), "ZH_cn 注册即可按 zh-CN 命中")
	})

	t.Run("注册后修改入参 map 不生效", func(t *testing.T) {
		b := i18n.NewBundles(i18n.En)
		msgs := map[string]string{"k": "值"}
		b.Register(i18n.ZhCN, msgs)
		msgs["k"] = "被篡改"
		assert.Equal(t, "值", b.TOf(i18n.ZhCN, "k"), "Register 复制入参")
	})

	t.Run("重复注册同 key 后者覆盖", func(t *testing.T) {
		b := i18n.NewBundles(i18n.En)
		b.Register(i18n.ZhCN, map[string]string{"k": "旧"})
		b.Register(i18n.ZhCN, map[string]string{"k": "新"})
		assert.Equal(t, "新", b.TOf(i18n.ZhCN, "k"))
	})

	t.Run("多次注册按 key 合并共存（多模块语言包）", func(t *testing.T) {
		b := i18n.NewBundles(i18n.En)
		b.Register(i18n.Language("zh"), map[string]string{"UserNotFound": "用户不存在"})
		b.Register(i18n.Language("zh"), map[string]string{"OrderNotFound": "订单不存在"})
		assert.Equal(t, "用户不存在", b.TOf("zh", "UserNotFound"), "后注册不清掉先注册")
		assert.Equal(t, "订单不存在", b.TOf("zh", "OrderNotFound"))
	})

	t.Run("默认语言规范化", func(t *testing.T) {
		assert.Equal(t, i18n.ZhCN, i18n.NewBundles("ZH_cn").DefaultLang())
	})
}

func TestTOf(t *testing.T) {
	b := newTestBundles()
	tests := []struct {
		name string
		lang i18n.Language
		key  string
		args []any
		want string
	}{
		{"基础语言精确命中", "zh", "user not found", nil, "用户不存在"},
		{"请求带地区收窄命中基础语言", i18n.ZhCN, "user not found", nil, "用户不存在"},
		{"其他地区变体同样收窄命中", "zh-TW", "user not found", nil, "用户不存在"},
		{"默认语言回退：未注册语言", "fr-FR", "user not found", nil, "user not found"},
		{"四处未命中返回 key 原文", "fr-FR", "nope", nil, "nope"},
		{"命中且带参数按序替换", i18n.ZhCN, "user {0} not found", []any{42}, "用户 42 不存在"},
		{"未命中返回 key 原文不插参", "fr-FR", "user {0} not found", []any{42}, "user {0} not found"},
		{"空译文视同未注册回退默认", i18n.ZhCN, "empty translation", nil, "empty translation"},
		{"默认语言未注册命中 key 原文", i18n.En, "user not found", nil, "user not found"},
		{"空语言走默认回退", "", "user not found", nil, "user not found"},
		{"语言参数规范化", "zh-cn", "user not found", nil, "用户不存在"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := b.TOf(tt.lang, tt.key, tt.args...)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTOfNoWidening(t *testing.T) {
	// 只收窄不放宽：注册在 zh-CN 的译文不会被请求 zh 命中，回退默认语言。
	b := i18n.NewBundles(i18n.En)
	b.Register(i18n.ZhCN, map[string]string{"user not found": "用户不存在"})

	assert.Equal(t, "用户不存在", b.TOf(i18n.ZhCN, "user not found"), "精确命中")
	assert.Equal(t, "user not found", b.TOf("zh", "user not found"),
		"请求 zh 不放宽命中 zh-CN，回退默认语言的 key 原文")
}

func TestT(t *testing.T) {
	b := newTestBundles()

	t.Run("从请求上下文取语言", func(t *testing.T) {
		ctx := rctx.SetRuntimeContext(context.Background(), rctx.RuntimeContext{Lang: "zh-CN"})
		assert.Equal(t, "用户不存在", b.T(ctx, "user not found"))
	})

	t.Run("带参数格式化", func(t *testing.T) {
		ctx := rctx.SetRuntimeContext(context.Background(), rctx.RuntimeContext{Lang: "zh-CN"})
		assert.Equal(t, "用户 7 不存在", b.T(ctx, "user {0} not found", 7))
	})

	t.Run("未注入语言回退默认", func(t *testing.T) {
		assert.Equal(t, "user not found", b.T(context.Background(), "user not found"))
	})

	t.Run("nil context 回退默认", func(t *testing.T) {
		var nilCtx context.Context
		assert.Equal(t, "user not found", b.T(nilCtx, "user not found"))
	})
}

func TestBundlesConcurrentReadWrite(t *testing.T) {
	b := i18n.NewBundles(i18n.En)
	b.Register(i18n.ZhCN, map[string]string{"k": "值"})

	var wg sync.WaitGroup
	for i := range 4 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			b.Register(i18n.En, map[string]string{"k": fmt.Sprintf("v%d", i)})
		}(i)
	}
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NotEmpty(t, b.TOf(i18n.ZhCN, "k"), "读侧不受并发注册影响")
		}()
	}
	wg.Wait()
}
