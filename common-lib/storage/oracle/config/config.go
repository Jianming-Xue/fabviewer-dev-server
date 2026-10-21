package config

import (
	"sort"
	"strconv"
	"strings"

	"github.com/fab-viewer/common-lib/common/bizerror"
	"github.com/fab-viewer/common-lib/storage"
)

// DefaultPort 是 Oracle 监听端口的默认值。
const DefaultPort = 1521

// DefaultSlowSQLMs 是慢 SQL 日志阈值的默认值（毫秒）。
const DefaultSlowSQLMs = 200

// LogLevel 是 GORM SQL 日志级别，字符串取值映射 gorm/logger 的四级别。
type LogLevel string

// 日志级别取值；零值 "" 按 LogLevelWarn 处理。
const (
	// LogLevelSilent 不输出任何 SQL 日志。
	LogLevelSilent LogLevel = "silent"
	// LogLevelError 仅输出错误 SQL。
	LogLevelError LogLevel = "error"
	// LogLevelWarn 输出错误与慢 SQL（默认）。
	LogLevelWarn LogLevel = "warn"
	// LogLevelInfo 输出全部 SQL。
	LogLevelInfo LogLevel = "info"
)

// Config 是 Oracle 连接配置。除连接信息（User/Password/Host/ServiceName）外，
// 其余字段零值均按默认处理：Port=1521、SlowSQLMs=200、LogLevel=warn、
// PrepareStmt=启用、Pool=storage.PoolConfig 默认值。
type Config struct {
	// User 是数据库用户名。
	User string
	// Password 是数据库密码。
	Password string
	// Host 是数据库主机名或地址。
	Host string
	// Port 是监听端口，<=0 时按 1521 处理。
	Port int
	// ServiceName 是服务名（SID 场景请改用完整连接串接入方式）。
	ServiceName string
	// SessionParams 是会话初始化参数，逐条生成 ALTER SESSION SET k=v。
	SessionParams map[string]string
	// Pool 是连接池配置，经 Pool.WithDefaults 补齐默认值。
	Pool storage.PoolConfig
	// SlowSQLMs 是慢 SQL 阈值（毫秒），<=0 时按 200 处理。
	SlowSQLMs int
	// LogLevel 是 SQL 日志级别，"" 按 warn 处理，其余非法值在 Open 时报错。
	LogLevel LogLevel
	// PrepareStmt 控制是否启用 GORM 预编译语句缓存；nil（零值）视为启用（默认），
	// 指向 false 时关闭。GORM 侧由 *bool 表达"默认开"的三态语义。
	PrepareStmt *bool
}

// WithDefaults 返回补齐默认值后的 Config 副本：Port=1521、SlowSQLMs=200、
// LogLevel=warn、Pool 四参数默认化；PrepareStmt 保持原样（nil 即默认启用）。
func (c Config) WithDefaults() Config { //nolint:gocritic // 配置值对象按值传递保证不可变语义
	if c.Port <= 0 {
		c.Port = DefaultPort
	}
	if c.SlowSQLMs <= 0 {
		c.SlowSQLMs = DefaultSlowSQLMs
	}
	if c.LogLevel == "" {
		c.LogLevel = LogLevelWarn
	}
	c.Pool = c.Pool.WithDefaults()
	return c
}

// Validate 检查连接必需字段与枚举取值，返回带上下文的业务错误。
func (c Config) Validate() error { //nolint:gocritic // 配置值对象按值传递保证不可变语义
	if c.User == "" {
		return bizerror.NewError(bizerror.CodeValidation, "oracle.Config.User 不能为空")
	}
	if c.Host == "" {
		return bizerror.NewError(bizerror.CodeValidation, "oracle.Config.Host 不能为空")
	}
	if c.ServiceName == "" {
		return bizerror.NewError(bizerror.CodeValidation, "oracle.Config.ServiceName 不能为空")
	}
	switch c.LogLevel {
	case LogLevelSilent, LogLevelError, LogLevelWarn, LogLevelInfo:
	default:
		return bizerror.NewError(bizerror.CodeValidation, "oracle.Config.LogLevel 非法取值 {0}", c.LogLevel)
	}
	return nil
}

// BuildDSN 生成 godror 风格的连接串：
//
//	user="u" password="p" connectString="host:port/service" alterSession="K=V"
//
// SessionParams 按键排序保证确定性；全部值经 strconv.Quote 转义，
// 可被 godror.ParseDSN 原样还原。应在 WithDefaults 之后调用（Port 补默认）。
func (c Config) BuildDSN() string { //nolint:gocritic // 配置值对象按值传递保证不可变语义
	var b strings.Builder
	b.WriteString("user=")
	b.WriteString(strconv.Quote(c.User))
	b.WriteString(" password=")
	b.WriteString(strconv.Quote(c.Password))
	b.WriteString(" connectString=")
	b.WriteString(strconv.Quote(c.ConnectString()))
	for _, k := range sortedKeys(c.SessionParams) {
		b.WriteString(" alterSession=")
		// 值会被 godror 二次 logfmt 解码（dsn.go: 对 alterSession 逐条再 parse）：
		// 含空格的值（如 NLS 日期格式）必须在第二层再引一层，否则被拆成两条残缺的
		// ALTER SESSION（ORA-02248，真实库集成测试抓出）。
		b.WriteString(strconv.Quote(k + "=" + strconv.Quote(c.SessionParams[k])))
	}
	return b.String()
}

// ConnectString 拼接 host:port/service 形式的连接描述（不含凭证，可入日志）。
func (c Config) ConnectString() string { //nolint:gocritic // 配置值对象按值传递保证不可变语义
	return c.Host + ":" + strconv.Itoa(c.Port) + "/" + c.ServiceName
}

// sortedKeys 返回映射按键升序排序后的键列表，nil/空映射返回 nil。
func sortedKeys(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
