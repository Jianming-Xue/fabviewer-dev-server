package commoncodes

import "github.com/fab-viewer/common-lib/common/bizerror"

// 库级 sentinel：供 errors.Is 探测（BizError.Is 按 Code 相等命中，任意
// 同码实例皆可命中）与根因链附加（AttachError 只进日志），Message 取
// 默认语言（英文）的字典文案。仅保留有真实调用点的；需要新 sentinel
// 时一行即可补回。
var (
	// ErrNotFound 资源未找到（OS 404 index_not_found 翻译、GORM 缺行根因等）。
	ErrNotFound = bizerror.NewError(NotFound, "not found")
	// ErrUnauthorized 未认证（OS 401 翻译）。
	ErrUnauthorized = bizerror.NewError(Unauthorized, "unauthorized")
	// ErrInvalidParameter 入参/配置非法（存储组件参数校验）。
	ErrInvalidParameter = bizerror.NewError(InvalidParameter, "invalid parameter")
	// ErrDBError 数据库/后端访问失败（建连、执行、反序列化）。
	ErrDBError = bizerror.NewError(DBError, "database error")
	// ErrRateLimited 请求过于频繁（OS 429 翻译）。
	ErrRateLimited = bizerror.NewError(RateLimited, "rate limited")
)
