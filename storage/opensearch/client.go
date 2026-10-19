package opensearch

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"

	"github.com/fab-viewer/common-lib/common/errs"
	"github.com/fab-viewer/common-lib/storage/opensearch/config"
)

// statusCodes 是 HTTP 状态码到 errs.Code 的精确映射；
// 未列出的状态码按区间归类（见 codeForStatus）。
var statusCodes = map[int]errs.Code{
	http.StatusUnauthorized:    errs.CodeUnauthorized,
	http.StatusForbidden:       errs.CodeForbidden,
	http.StatusNotFound:        errs.CodeNotFound,
	http.StatusConflict:        errs.CodeConflict,
	http.StatusTooManyRequests: errs.CodeTooMany,
}

// Client 是 OpenSearch 访问封装，内部持有 opensearch-go/v4 客户端，
// SDK 类型不越过本包边界（SDK() 逃生舱除外）。所有方法并发安全，
// 不创建任何后台 goroutine。
type Client struct {
	api *opensearchapi.Client
}

// Open 按配置创建 OpenSearch 客户端并完成连接装配。
// Addrs 为空（errs.CodeValidation）或地址非法（errs.CodeDB）时返回错误。
func Open(
	//nolint:gocritic // Config 为计划契约的配置值对象，按值传入防止调用方持有后修改
	cfg config.Config,
) (*Client, error) {
	sdkCfg, err := sdkConfig(cfg)
	if err != nil {
		return nil, err
	}
	api, err := opensearchapi.NewClient(opensearchapi.Config{Client: sdkCfg})
	if err != nil {
		return nil, errs.Wrapf(err, errs.CodeDB, "opensearch.Open addrs=%v", cfg.Addrs)
	}
	return &Client{api: api}, nil
}

// NewFromAPI 用既有 SDK 客户端构造 Client（高级用法/测试替身：SDK 客户端由
// 外部装配时复用；常规建连走 Open）。
func NewFromAPI(api *opensearchapi.Client) *Client { return &Client{api: api} }

// DoSearch 执行一条原生搜索请求并把传输/HTTP 层错误统一翻译为 errs 错误码
// （供 search 子包与业务直连 DSL 场景复用；响应解析由调用方完成）。
func (c *Client) DoSearch(ctx context.Context, req *opensearchapi.SearchReq) (*opensearchapi.SearchResp, error) {
	resp, err := c.api.Search(ctx, req)
	if err != nil {
		kv := "indices=" + strings.Join(req.Indices, ",")
		if len(req.Indices) == 1 {
			kv = "index=" + req.Indices[0]
		}
		return nil, translateError(err, "opensearch.Search", kv)
	}
	return resp, nil
}

// SDK 返回底层 opensearch-go/v4 客户端作为逃生舱：本包未覆盖的能力
// （聚合分析、Bulk、索引/快照管理等）经它直接使用 SDK。
// 注意：此后业务代码与 SDK 类型耦合，SDK 升级可能波及调用方。
func (c *Client) SDK() *opensearchapi.Client { return c.api }

// Index 将 doc 序列化为 JSON 写入 index 下指定 id 的文档：
// 不存在则创建、存在则整档覆盖（PUT /<index>/_doc/<id>）。id 为空时由
// 服务端自动生成。写失败按状态码翻译为统一错误码。
func (c *Client) Index(ctx context.Context, index, id string, doc any) error {
	if index == "" {
		return errs.Newf(errs.CodeValidation, "opensearch.Index index 不能为空")
	}
	if doc == nil {
		return errs.Newf(errs.CodeValidation,
			"opensearch.Index index=%s id=%s doc 不能为 nil", index, id)
	}
	body, err := json.Marshal(doc)
	if err != nil {
		return errs.Wrapf(err, errs.CodeValidation,
			"opensearch.Index index=%s id=%s 序列化文档", index, id)
	}
	_, err = c.api.Index(ctx, opensearchapi.IndexReq{
		Index:      index,
		DocumentID: id,
		Body:       bytes.NewReader(body),
	})
	if err != nil {
		return translateError(err, "opensearch.Index", "index="+index, "id="+id)
	}
	return nil
}

// Get 读取 index 下指定 id 的文档并反序列化到 dest。
// 文档或索引不存在时返回 (false, nil)；读取失败按状态码翻译错误码。
func Get[T any](ctx context.Context, c *Client, index, id string, dest *T) (bool, error) {
	if index == "" || id == "" {
		return false, errs.Newf(errs.CodeValidation,
			"opensearch.Get index 与 id 不能为空 index=%q id=%q", index, id)
	}
	if dest == nil {
		return false, errs.Newf(errs.CodeValidation,
			"opensearch.Get index=%s id=%s dest 不能为 nil", index, id)
	}
	resp, err := c.api.Document.Get(ctx, opensearchapi.DocumentGetReq{
		Index: index, DocumentID: id,
	})
	if err != nil {
		if errorStatus(err) == http.StatusNotFound {
			return false, nil
		}
		return false, translateError(err, "opensearch.Get", "index="+index, "id="+id)
	}
	if !resp.Found {
		return false, nil
	}
	if err := json.Unmarshal(resp.Source, dest); err != nil {
		return true, errs.Wrapf(err, errs.CodeDB,
			"opensearch.Get index=%s id=%s 反序列化 _source", index, id)
	}
	return true, nil
}

// Delete 删除 index 下指定 id 的文档。文档或索引不存在视为删除成功
// （幂等语义），返回 nil；其余失败按状态码翻译错误码。
func (c *Client) Delete(ctx context.Context, index, id string) error {
	if index == "" || id == "" {
		return errs.Newf(errs.CodeValidation,
			"opensearch.Delete index 与 id 不能为空 index=%q id=%q", index, id)
	}
	_, err := c.api.Document.Delete(ctx, opensearchapi.DocumentDeleteReq{
		Index: index, DocumentID: id,
	})
	if err != nil {
		if errorStatus(err) == http.StatusNotFound {
			return nil
		}
		return translateError(err, "opensearch.Delete", "index="+index, "id="+id)
	}
	return nil
}

// translateError 将 SDK 错误包装为带操作上下文的统一错误：
// op 为 "opensearch.Index" 式操作名，kv 为 "index=x" "id=y" 式关键参数；
// 状态码按 statusCodes/区间映射错误码，无状态码（集群不可达等传输层错误）
// 归为 errs.CodeDB。ES 错误体的根因（type/reason）附加在 message 中。
func translateError(err error, op string, kv ...string) error {
	msg := op
	if len(kv) > 0 {
		msg += " " + strings.Join(kv, " ")
	}
	if detail := esErrorDetail(err); detail != "" {
		msg += ": " + detail
	}
	return errs.Wrapf(err, codeForStatus(errorStatus(err)), "%s", msg)
}

// codeForStatus 将 HTTP 状态码映射为统一错误码：
// 精确命中优先（401/403/404/409/429），其余 4xx（含 400 查询语法错误）
// 归 errs.CodeValidation，5xx 及无状态码归 errs.CodeDB。
func codeForStatus(status int) errs.Code {
	if code, ok := statusCodes[status]; ok {
		return code
	}
	switch {
	case status >= 500:
		return errs.CodeDB
	case status >= 400:
		return errs.CodeValidation
	default:
		return errs.CodeDB
	}
}

// errorStatus 从 SDK 错误链中提取 HTTP 状态码，取不到返回 0。
func errorStatus(err error) int {
	var structErr *opensearch.StructError
	if errors.As(err, &structErr) {
		return structErr.Status
	}
	var stringErr *opensearch.StringError
	if errors.As(err, &stringErr) {
		return stringErr.Status
	}
	return 0
}

// esErrorDetail 提取 ES 错误体中的根因描述（root_cause 优先，其次
// error.type: error.reason），非结构化错误体时退回原始字符串。
func esErrorDetail(err error) string {
	var structErr *opensearch.StructError
	if errors.As(err, &structErr) {
		if len(structErr.Err.RootCause) > 0 {
			cause := structErr.Err.RootCause[0]
			return cause.Type + ": " + cause.Reason
		}
		return structErr.Err.Type + ": " + structErr.Err.Reason
	}
	var stringErr *opensearch.StringError
	if errors.As(err, &stringErr) {
		return stringErr.Err
	}
	return ""
}

// sdkConfig 将 config.Config 映射为 opensearch-go/v4 的 SDK 配置：
// Pool.MaxIdleConns → MaxIdleConnsPerHost、Pool.ConnMaxIdleTime → IdleConnTimeout
// （均先经 WithDefaults 补默认）。Pool.MaxOpenConns/ConnMaxLifetime 是长连接
// 数据库的池语义，HTTP 连接池按需建连、由 Keep-Alive 复用，无精确对应，不映射。
func sdkConfig(c config.Config) (opensearch.Config, error) { //nolint:gocritic //hugeParam：值语义只读映射，与 Open 的按值 Config 一致
	if len(c.Addrs) == 0 {
		return opensearch.Config{}, errs.Newf(errs.CodeValidation,
			"opensearch.Config Addrs 不能为空")
	}
	pool := c.Pool.WithDefaults()
	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		MaxIdleConnsPerHost: pool.MaxIdleConns,
		IdleConnTimeout:     pool.ConnMaxIdleTime,
	}
	if c.Insecure {
		//nolint:gosec // Insecure 由调用方显式 opt-in，仅限内网自签证书集群跳过校验。
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return opensearch.Config{
		Addresses: c.Addrs,
		Username:  c.Username,
		Password:  c.Password,
		Transport: transport,
	}, nil
}
