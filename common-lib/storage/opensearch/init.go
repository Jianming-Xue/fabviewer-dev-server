package opensearch

import (
	"github.com/fab-viewer/common-lib/common/bizerror"
	"github.com/fab-viewer/common-lib/common/commoncodes"
	commonconfig "github.com/fab-viewer/common-lib/common/config"
	"github.com/fab-viewer/common-lib/storage/opensearch/config"
)

// SectionName 是本组件在配置 map 中的段名（application.yml 顶层
// opensearch 段）。
const SectionName = "opensearch"

// Init 从配置 map 初始化 OpenSearch 客户端（组件自治形态，与 logx.Init
// 同款约定，由组装根经 wire 统一调用）：读 "opensearch" 段绑定 Config
// （字段按完整名忽略大小写对齐，addrs/username/password/insecure/pool）→
// 校验 addrs 非空 → Open 装配客户端（不拨号，首个请求才真正建连）。
//
// 是否启用由段的存在性决定：无 opensearch 段即未启用，返回 (nil, 空
// cleanup, nil)，是否回退由业务自决——环境变量覆盖只能改写文件中已有的
// 叶子，启用与否完全由配置文件表达。段存在但缺 addrs 报 VALIDATION_ERROR。
// 敏感值（password）建议保持文件占位、经叶子环境变量覆盖注入。
//
// cleanup 为空操作：opensearch-go 是纯 HTTP 客户端、无后台资源，无需
// 显式关闭（空闲连接由 transport 自行回收），保留返回值以统一 wire
// cleanup 链形态。
func Init(m map[string]any) (*Client, func(), error) {
	section := commonconfig.Get(m, SectionName)
	if section == nil {
		return nil, func() {}, nil
	}
	var cfg config.Config
	if err := commonconfig.Bind(section, &cfg); err != nil {
		return nil, nil, bizerror.NewError(commoncodes.InvalidParameter,
			"opensearch.Init 配置段 {0} 绑定失败", SectionName).AttachError(err, nil)
	}
	if len(cfg.Addrs) == 0 {
		return nil, nil, bizerror.NewError(commoncodes.InvalidParameter,
			"opensearch.Init 配置段 {0} 缺 addrs", SectionName)
	}
	cli, err := Open(cfg)
	if err != nil {
		return nil, nil, err
	}
	return cli, func() {}, nil
}
