package conn

import (
	"context"

	"github.com/fab-viewer/common-lib/common/bizerror"
	"github.com/fab-viewer/common-lib/common/commoncodes"
	commonconfig "github.com/fab-viewer/common-lib/common/config"
	"github.com/fab-viewer/common-lib/storage/oracle/config"
)

// SectionName 是本组件在配置 map 中的段名（application.yml 顶层 oracle 段）。
const SectionName = "oracle"

// Init 从配置 map 初始化 Oracle 连接（组件自治形态，与 logx.Init 同款
// 约定，由组装根经 wire 统一调用）：读 "oracle" 段绑定 Config（字段按
// 完整名忽略大小写对齐，camelCase 键直命中；sessionParams/pool 嵌套段
// 同原理）→ Open（内部 WithDefaults/Validate/建连探活）。
//
// 是否启用由段的存在性决定：无 oracle 段即未启用，返回 (nil, 空 cleanup,
// nil)，回退方案（如内存实现）由业务自决——环境变量覆盖只能改写文件中
// 已有的叶子，不能凭空建段，启用与否完全由配置文件表达。段存在但缺必填
// 字段在建连前即报错（VALIDATION_ERROR），建连失败报 DB_ERROR。敏感值
// （password）建议保持文件占位、经叶子环境变量覆盖注入（如
// security.oracle.password=...）。
//
// cleanup 承载 Close，接入 wire cleanup 链逆序释放（HTTP 停机 → 关存储
// → 关日志）。WithLogger 等定制经 opts 透传给 Open。
func Init(m map[string]any, opts ...Option) (*DB, func(), error) {
	section := commonconfig.Get(m, SectionName)
	if section == nil {
		return nil, func() {}, nil
	}
	var cfg config.Config
	if err := commonconfig.Bind(section, &cfg); err != nil {
		return nil, nil, bizerror.NewError(commoncodes.InvalidParameter,
			"oracle.Init 配置段 {0} 绑定失败", SectionName).AttachError(err, nil)
	}
	db, err := Open(context.Background(), cfg, opts...)
	if err != nil {
		return nil, nil, err
	}
	return db, func() { _ = db.Close() }, nil
}
