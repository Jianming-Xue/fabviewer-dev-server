// Package logsink 提供 OpenSearch 日志出口：实现 logx.Sink，把批量日志行
// 经 Bulk API 直写按日索引（<prefix>-yyyy.MM.dd），供 logx.New 的 Remote
// 出口接入：
//
//	sink := logsink.New(cli, "logs")           // cli 为 opensearch.Client
//	log, closeLog, err := logx.New(logx.Config{
//	    Stdout: true,
//	    File:   &logx.FileConfig{Path: "app.log"},
//	    Remote: sink,
//	})
//
// 投递语义由 logx 承担（异步缓冲、凑批、失败丢弃计数不反压业务）；本包只
// 负责把一批 NDJSON 行安全投递到当日索引。按日分索引便于留存期管理
// （ILM/定期删除旧索引）；zap 的 JSON 行不含裸换行，可安全按行拼接。
package logsink

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"

	"github.com/fab-viewer/common-lib/storage/opensearch"
)

// LogSink 把批量日志行经 Bulk API 写入按日索引。
type LogSink struct {
	api    *opensearchapi.Client
	prefix string
	now    func() time.Time // 可注入时钟（测试）
}

// New 创建日志出口。cli 为已建立的 OpenSearch 客户端；prefix 为索引名
// 前缀（空则 "logs"，实际索引形如 logs-2026.10.20）。
func New(cli *opensearch.Client, prefix string) *LogSink {
	if prefix == "" {
		prefix = "logs"
	}
	s := &LogSink{prefix: prefix, now: time.Now}
	if cli != nil {
		s.api = cli.SDK()
	}
	return s
}

// Flush 将一批日志行以 NDJSON bulk 请求写入当日索引；部分条目失败即报错
// （调用方 logx 会丢弃本批并计数）。
func (s *LogSink) Flush(lines [][]byte) error {
	if s.api == nil {
		return fmt.Errorf("logsink: 客户端未初始化")
	}
	if len(lines) == 0 {
		return nil
	}
	index := fmt.Sprintf("%s-%s", s.prefix, s.now().Format("2006.01.02"))

	var body bytes.Buffer
	for _, line := range lines {
		fmt.Fprintf(&body, "{\"index\":{\"_index\":%q}}\n", index)
		body.Write(bytes.TrimRight(line, "\n")) // 单行化，防破坏 NDJSON
		body.WriteByte('\n')
	}

	resp, err := s.api.Bulk(context.Background(),
		opensearchapi.BulkReq{Body: strings.NewReader(body.String())})
	if err != nil {
		return fmt.Errorf("logsink: bulk 请求失败: %w", err)
	}
	if resp.Errors {
		return fmt.Errorf("logsink: bulk 部分条目失败（%d 条）", len(lines))
	}
	return nil
}
