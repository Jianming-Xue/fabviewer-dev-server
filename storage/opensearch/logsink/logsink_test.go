package logsink_test

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	sdk "github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fab-viewer/common-lib/storage/opensearch"
	"github.com/fab-viewer/common-lib/storage/opensearch/logsink"
)

// capturingRT 捕获 bulk 请求并返回成功响应。
type capturingRT struct {
	mu   sync.Mutex
	reqs []captured
}

type captured struct{ Method, Path, Body string }

func (rt *capturingRT) RoundTrip(r *http.Request) (*http.Response, error) {
	body := ""
	if r.Body != nil {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		_ = r.Body.Close()
	}
	rt.mu.Lock()
	rt.reqs = append(rt.reqs, captured{r.Method, r.URL.Path, body})
	rt.mu.Unlock()
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"took":1,"errors":false,"items":[{"index":{"status":201}}]}`)),
	}, nil
}

// newSink 构建带捕获传输的 LogSink 与请求记录。
func newSink(t *testing.T, prefix string, now time.Time) (*logsink.LogSink, *capturingRT) {
	t.Helper()
	rt := &capturingRT{}
	api, err := opensearchapi.NewClient(opensearchapi.Config{
		Client: sdk.Config{Transport: rt},
	})
	require.NoError(t, err)
	s := logsink.New(opensearch.NewFromAPI(api), prefix)
	return s.WithClockForTest(func() time.Time { return now }), rt
}

func TestFlushBulkRequest(t *testing.T) {
	now := time.Date(2026, 10, 20, 15, 0, 0, 0, time.UTC)
	s, rt := newSink(t, "logs", now)

	lines := [][]byte{
		[]byte(`{"level":"info","msg":"access","tx_id":"t1"}` + "\n"),
		[]byte(`{"level":"error","msg":"boom"}` + "\n"),
	}
	require.NoError(t, s.Flush(lines))

	require.Len(t, rt.reqs, 1)
	got := rt.reqs[0]
	assert.Equal(t, http.MethodPost, got.Method)
	assert.Equal(t, "/_bulk", got.Path)

	want := `{"index":{"_index":"logs-2026.10.20"}}` + "\n" +
		`{"level":"info","msg":"access","tx_id":"t1"}` + "\n" +
		`{"index":{"_index":"logs-2026.10.20"}}` + "\n" +
		`{"level":"error","msg":"boom"}` + "\n"
	assert.Equal(t, want, got.Body, "NDJSON：每行日志前一条 index 指令，指向当日索引")
}

func TestFlushEmptyAndNil(t *testing.T) {
	now := time.Date(2026, 10, 20, 15, 0, 0, 0, time.UTC)
	s, rt := newSink(t, "", now)

	require.NoError(t, s.Flush(nil), "空批直接跳过")
	require.NoError(t, s.Flush([][]byte{}))
	assert.Empty(t, rt.reqs)
}

func TestFlushPrefixDefault(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	s, rt := newSink(t, "", now) // 前缀空 → logs

	require.NoError(t, s.Flush([][]byte{[]byte("{}")}))
	assert.Contains(t, rt.reqs[0].Body, `"_index":"logs-2026.01.02"`)
}

func TestFlushNilClient(t *testing.T) {
	s := logsink.New(nil, "")
	err := s.Flush([][]byte{[]byte("{}")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "客户端未初始化")
}

func TestFlushBulkFailure(t *testing.T) {
	rt := &failingRT{}
	api, err := opensearchapi.NewClient(opensearchapi.Config{
		Client: sdk.Config{Transport: rt},
	})
	require.NoError(t, err)
	s := logsink.New(opensearch.NewFromAPI(api), "logs")

	err = s.Flush([][]byte{[]byte("{}")})
	require.Error(t, err, "errors=true 时整体报错")
	assert.Contains(t, err.Error(), "部分条目失败")
}

// errorRT 网络层直接失败。
type errorRT struct{}

func (errorRT) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("connection refused")
}

func TestFlushTransportError(t *testing.T) {
	api, err := opensearchapi.NewClient(opensearchapi.Config{
		Client: sdk.Config{Transport: errorRT{}},
	})
	require.NoError(t, err)
	s := logsink.New(opensearch.NewFromAPI(api), "logs")

	err = s.Flush([][]byte{[]byte("{}")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bulk 请求失败")
}

// failingRT 返回 errors=true 的 bulk 响应。
type failingRT struct{}

func (failingRT) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"took":1,"errors":true,"items":[{"index":{"status":429}}]}`)),
	}, nil
}
