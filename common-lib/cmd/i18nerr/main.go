// Command i18nerr 从多语言消息 Go 文件生成模块码集合与翻译入口，使
// 「文件里的 key」与「代码里的码」一一对应、永不漂移：
//
//	cd usercodes && go run github.com/fab-viewer/common-lib/cmd/i18nerr \
//	    -dir . -pkg usercodes -default en
//
// 输入约定（目录下的 messages_<语言>.go，如 messages_en.go / messages_zh.go，
// 文件名即语言；包名约定 xxxcodes——包名即模块码命名空间，引用形如
// usercodes.UserNotFound）：
//
//	package usercodes
//
//	var MessagesEn = map[string]string{
//		"UserNotFound": "user {0} not found", // key 即码，value 为
//	}                                        // 文案模板，{0} {1} 为位置占位符
//
// 每文件恰好一个导出的 map[string]string 变量（复合字面量、键值均为字符串
// 字面量）。输出 messages_gen.go 四部分（key 要求形如导出标识符）：
//
//   - 码常量：名字与值均即 key 原文（fault 直接透传为响应体业务 code）
//   - 私有 dict 与 init：本模块多语言字典（语言包编译期注册）
//   - NewError(ctx, code, args...)：BizError 构造入口（getMessage 按请求
//     语言翻译 Message）
//   - NewMessage(ctx, code, args...)：纯文案 string 出口（不限于错误，
//     args 可选非强制；也可不带参取模板，经 i18n.InsertArgs 在调用点
//     自行替换）
//
// 各语言文件按同 key 对齐、占位符位置可不同（{n} 始终对应第 n 个参数）；
// Message 出口直出，出口不做翻译。
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/fab-viewer/common-lib/common/i18n"
)

// keyPattern 约束 key 形如导出标识符（UserNotFound），加 Err 前缀即合法变量名。
var keyPattern = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)

// filePattern 匹配 messages_<语言>.go（语言如 en、zh、zh-CN）。
var filePattern = regexp.MustCompile(`^messages_(.+)\.go$`)

// langPattern 约束文件名中的语言段（字母数字与连字符/下划线组，"_" 在
// 归一化中与 "-" 等价）。
var langPattern = regexp.MustCompile(`^[A-Za-z0-9]+([-_][A-Za-z0-9]+)*$`)

// msgFile 是一份 messages_<语言>.go 的解析结果。
type msgFile struct {
	lang    i18n.Language
	varName string // 消息 map 的变量名（如 MessagesEn）
	msgs    map[string]string
}

func main() {
	dir := flag.String("dir", ".", "消息文件所在目录（messages_<语言>.go）")
	pkg := flag.String("pkg", "", "生成文件所属包名（必填）")
	def := flag.String("default", "", "默认语言（必填，如 en；该语言文案成为错误变量的兜底消息）")
	out := flag.String("out", "messages_gen.go", "生成文件名（相对 -dir）")
	flag.Parse()
	if err := run(*dir, *pkg, *def, *out); err != nil {
		fmt.Fprintln(os.Stderr, "i18nerr:", err)
		os.Exit(1)
	}
}

// run 扫描目录、解析全部语言文件、生成并写回格式化后的变量集合。
func run(dir, pkg, def, out string) error {
	if pkg == "" || def == "" {
		return fmt.Errorf("-pkg/-default 均为必填")
	}
	files, err := loadMsgFiles(dir, filepath.Base(out))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("%s 下未找到 messages_<语言>.go", dir)
	}
	defLang := i18n.NormalizeLanguage(def)
	defFile, ok := files[defLang]
	if !ok {
		return fmt.Errorf("缺少默认语言文件 messages_%s.go", defLang)
	}
	if bad := invalidKeys(defFile.msgs); len(bad) > 0 {
		return fmt.Errorf("key 必须形如导出标识符（首字母大写、仅字母数字），非法 key: %s",
			strings.Join(bad, ", "))
	}
	src, err := format.Source([]byte(generate(pkg, defLang, files)))
	if err != nil {
		return fmt.Errorf("生成代码格式化失败: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, out), src, 0o644); err != nil { //nolint:gosec // 生成产物按常规 0644 供构建读取
		return fmt.Errorf("写入 %s 失败: %w", out, err)
	}
	return nil
}

// loadMsgFiles 扫描目录下全部 messages_<语言>.go 并解析（out 为生成文件名，
// 与输入模式 messages_(.+).go 同形，显式跳过防止把产物当语言包解析）。
func loadMsgFiles(dir, out string) (map[i18n.Language]msgFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("读取目录 %s 失败: %w", dir, err)
	}
	files := make(map[i18n.Language]msgFile)
	for _, e := range entries {
		if e.IsDir() || e.Name() == out {
			continue
		}
		m := filePattern.FindStringSubmatch(e.Name())
		if m == nil {
			continue // doc.go、messages_gen.go 等非消息文件忽略
		}
		if !langPattern.MatchString(m[1]) {
			return nil, fmt.Errorf("%s: 文件名语言段 %q 非法", e.Name(), m[1])
		}
		f, err := parseMsgFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		f.lang = i18n.NormalizeLanguage(m[1])
		if prev, dup := files[f.lang]; dup {
			return nil, fmt.Errorf("语言 %s 重复定义（%s 与 %s）", f.lang, e.Name(), prev.varName)
		}
		files[f.lang] = f
	}
	return files, nil
}

// parseMsgFile 解析单个消息文件：取出其中唯一一个导出的 map[string]string
// 复合字面量（约定每文件一个），键值均须为字符串字面量。
func parseMsgFile(path string) (msgFile, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return msgFile{}, fmt.Errorf("解析 %s 失败: %w", path, err)
	}
	name, lit, err := findMsgMap(path, f)
	if err != nil {
		return msgFile{}, err
	}
	msgs, err := extractMsgs(path, fset, lit)
	if err != nil {
		return msgFile{}, err
	}
	return msgFile{varName: name, msgs: msgs}, nil
}

// findMsgMap 找到文件中唯一一个导出的 map[string]string 复合字面量。
func findMsgMap(path string, f *ast.File) (string, *ast.CompositeLit, error) {
	var name string
	var lit *ast.CompositeLit
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			continue
		}
		var err error
		if name, lit, err = scanVarSpecs(path, gd, name, lit); err != nil {
			return "", nil, err
		}
	}
	if name == "" {
		return "", nil, fmt.Errorf("%s: 未找到导出的 map[string]string 消息变量", path)
	}
	return name, lit, nil
}

// scanVarSpecs 扫描一组 var 声明，记录首个符合条件的消息 map（再遇即报错）。
func scanVarSpecs(path string, gd *ast.GenDecl, name string, lit *ast.CompositeLit,
) (string, *ast.CompositeLit, error) {
	for _, spec := range gd.Specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for i, n := range vs.Names {
			if !n.IsExported() || i >= len(vs.Values) {
				continue
			}
			l, ok := stringMapLit(vs.Values[i])
			if !ok {
				continue
			}
			if name != "" {
				return "", nil, fmt.Errorf("%s: 每文件只允许一个消息 map（%s 与 %s）",
					path, name, n.Name)
			}
			name, lit = n.Name, l
		}
	}
	return name, lit, nil
}

// extractMsgs 从复合字面量逐条提取字符串键值对。
func extractMsgs(path string, fset *token.FileSet, lit *ast.CompositeLit) (map[string]string, error) {
	msgs := make(map[string]string, len(lit.Elts))
	for _, elt := range lit.Elts {
		if err := addMsgEntry(path, fset, msgs, elt); err != nil {
			return nil, err
		}
	}
	return msgs, nil
}

// addMsgEntry 提取并写入单条 "key": "value"（key 重复即报错）。
func addMsgEntry(path string, fset *token.FileSet, msgs map[string]string, elt ast.Expr) error {
	kv, ok := elt.(*ast.KeyValueExpr)
	if !ok {
		return fmt.Errorf("%s: 消息条目必须是 key: value 形式（%s）",
			path, fset.Position(elt.Pos()))
	}
	key, err := stringLit(fset, kv.Key)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	value, err := stringLit(fset, kv.Value)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if _, dup := msgs[key]; dup {
		return fmt.Errorf("%s: key %q 重复", path, key)
	}
	msgs[key] = value
	return nil
}

// stringMapLit 判定表达式是否为 map[string]string 复合字面量。
func stringMapLit(expr ast.Expr) (*ast.CompositeLit, bool) {
	lit, ok := expr.(*ast.CompositeLit)
	if !ok {
		return nil, false
	}
	mt, ok := lit.Type.(*ast.MapType)
	if !ok {
		return nil, false
	}
	key, _ := mt.Key.(*ast.Ident)
	value, _ := mt.Value.(*ast.Ident)
	return lit, key != nil && value != nil && key.Name == "string" && value.Name == "string"
}

// stringLit 提取字符串字面量的值（不支持拼接等复杂表达式，报错带位置）。
func stringLit(fset *token.FileSet, expr ast.Expr) (string, error) {
	basic, ok := expr.(*ast.BasicLit)
	if !ok || basic.Kind != token.STRING {
		return "", fmt.Errorf("仅支持字符串字面量（%s）", fset.Position(expr.Pos()))
	}
	s, err := strconv.Unquote(basic.Value)
	if err != nil {
		return "", fmt.Errorf("字符串字面量非法（%s）: %w", fset.Position(expr.Pos()), err)
	}
	return s, nil
}

// invalidKeys 返回不符合 keyPattern 的 key（排序稳定便于报错定位）。
func invalidKeys(msgs map[string]string) []string {
	var bad []string
	for k := range msgs {
		if !keyPattern.MatchString(k) {
			bad = append(bad, k)
		}
	}
	sort.Strings(bad)
	return bad
}

// generate 拼装 messages_gen.go 源码（key 与语言均升序，输出确定）。
func generate(pkg string, defLang i18n.Language, files map[i18n.Language]msgFile) string {
	def := files[defLang]
	keys := make([]string, 0, len(def.msgs))
	for k := range def.msgs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	langs := make([]i18n.Language, 0, len(files))
	for l := range files {
		langs = append(langs, l)
	}
	sort.Slice(langs, func(a, b int) bool { return langs[a] < langs[b] })

	var sb strings.Builder
	sb.WriteString("// Code generated by i18nerr from messages_*.go. DO NOT EDIT.\n\n")
	fmt.Fprintf(&sb, "package %s\n\n", pkg)
	sb.WriteString("import (\n")
	sb.WriteString("\t\"context\"\n\n")
	sb.WriteString("\t\"github.com/fab-viewer/common-lib/common/bizerror\"\n")
	sb.WriteString("\t\"github.com/fab-viewer/common-lib/common/i18n\"\n")
	sb.WriteString(")\n\n")
	sb.WriteString("// 码常量：名字与 messages_<语言>.go 中的 key 完全一致，值即 key\n")
	sb.WriteString("// 原文（fault 直接透传为响应体业务 code）。本包即模块码命名空间，\n")
	fmt.Fprintf(&sb, "// 引用形如 %s.UserNotFound。\n", pkg)
	sb.WriteString("const (\n")
	for _, k := range keys {
		fmt.Fprintf(&sb, "\t%s bizerror.Code = %q\n", k, k)
	}
	sb.WriteString(")\n\n")
	sb.WriteString("// dict 本模块多语言字典（私有，语言包编译期注册）。\n")
	fmt.Fprintf(&sb, "var dict = i18n.NewBundles(i18n.Language(%q))\n\n", defLang)
	sb.WriteString("func init() {\n")
	for _, l := range langs {
		fmt.Fprintf(&sb, "\tdict.Register(i18n.Language(%q), %s)\n", l, files[l].varName)
	}
	sb.WriteString("}\n\n")
	sb.WriteString("// getMessage 按请求语言查出翻译后的 message：回退链为请求语言 →\n")
	sb.WriteString("// 基础语言 → 模块默认语言 → key 原文。\n")
	sb.WriteString("func getMessage(ctx context.Context, code bizerror.Code) string {\n")
	sb.WriteString("\treturn dict.T(ctx, string(code))\n")
	sb.WriteString("}\n\n")
	sb.WriteString("// NewError 产出 BizError：Message 经 getMessage 按请求语言翻译，\n")
	sb.WriteString("// args 可选（按序替换模板中的 {0} {1} ...，与链式 InsertArgs 等价）。\n")
	sb.WriteString("func NewError(ctx context.Context, code bizerror.Code, args ...any) *bizerror.BizError {\n")
	sb.WriteString("\treturn bizerror.NewError(code, getMessage(ctx, code), args...)\n")
	sb.WriteString("}\n\n")
	sb.WriteString("// NewMessage 产出纯文案 string（不限于错误）：同样按请求语言翻译，\n")
	sb.WriteString("// args 可选非强制；也可不带参取模板，经 i18n.InsertArgs 在调用点\n")
	sb.WriteString("// 自行替换。\n")
	sb.WriteString("func NewMessage(ctx context.Context, code bizerror.Code, args ...any) string {\n")
	sb.WriteString("\treturn i18n.InsertArgs(getMessage(ctx, code), args...)\n")
	sb.WriteString("}\n")
	return sb.String()
}
