package search

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/stretchr/testify/require"

	osclient "github.com/fab-viewer/common-lib/storage/opensearch"
)

// capturedRequest 记录一次 RoundTrip 收到的请求要素。
type capturedRequest struct {
	Method string
	Path   string
	Body   string
}

// countingBody 是带关闭计数的响应体：SDK 传输层读取完响应后会关闭原始
// Body（再以 NopCloser 替换），计数用于断言全链路每个 Body 都被关闭。
type countingBody struct {
	reader *strings.Reader
	closed int
}

func (b *countingBody) Read(p []byte) (int, error) { return b.reader.Read(p) }

func (b *countingBody) Close() error {
	b.closed++
	return nil
}

// fakeTransport 注入 SDK 的 Config.Transport，固定返回配置的状态码与
// 响体（或网络错误），记录收到的请求及每个响应体的关闭次数，
// 使单测走完 opensearch-go 完整调用链路。
type fakeTransport struct {
	status  int
	body    string
	failErr error // 非 nil 时 RoundTrip 返回网络错误

	mu       sync.Mutex
	requests []capturedRequest
	bodies   []*countingBody
}

func (f *fakeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body := ""
	if req.Body != nil {
		data, _ := io.ReadAll(req.Body)
		_ = req.Body.Close()
		body = string(data)
	}
	f.mu.Lock()
	f.requests = append(f.requests, capturedRequest{Method: req.Method, Path: req.URL.Path, Body: body})
	f.mu.Unlock()
	if f.failErr != nil {
		return nil, f.failErr
	}
	respBody := &countingBody{reader: strings.NewReader(f.body)}
	f.mu.Lock()
	f.bodies = append(f.bodies, respBody)
	f.mu.Unlock()
	return &http.Response{
		StatusCode: f.status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       respBody,
		Request:    req,
	}, nil
}

// requestCount 返回已捕获的请求数。
func (f *fakeTransport) requestCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}

// request 返回第 i 个捕获请求。
func (f *fakeTransport) request(t *testing.T, i int) capturedRequest {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	require.Greater(t, len(f.requests), i, "请求数不足")
	return f.requests[i]
}

// assertAllClosed 断言每个响应体都被关闭且仅关闭一次。
func (f *fakeTransport) assertAllClosed(t *testing.T) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	require.NotEmpty(t, f.bodies, "没有任何响应体被创建")
	for i, b := range f.bodies {
		require.Equal(t, 1, b.closed, "第 %d 个响应体应被关闭且仅关闭一次", i)
	}
}

// newFakeClient 用 fakeTransport 构造走完整 SDK 链路的 Client。
func newFakeClient(t *testing.T, rt http.RoundTripper) *osclient.Client {
	t.Helper()
	api, err := opensearchapi.NewClient(opensearchapi.Config{Client: opensearch.Config{
		Addresses: []string{"http://127.0.0.1:19200"},
		Transport: rt,
	}})
	require.NoError(t, err)
	return osclient.NewFromAPI(api)
}
