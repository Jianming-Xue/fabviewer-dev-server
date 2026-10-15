package oracle

import (
	"strings"

	"gorm.io/gorm/schema"
)

// upperNamer 把内层 Namer 的全部产出转为大写：GORM 默认 snake_case 小写
// 加引号后生成 "id"/"name"，而 Oracle 无引号标识符一律按大写存储，两者不
// 匹配会报 ORA-00904（真实库集成测试抓出）。整体大写后与标准 Oracle DDL
// （CREATE TABLE T (ID NUMBER, NAME VARCHAR2)）对齐，业务模型无需逐字段
// 写 column tag。
type upperNamer struct{ inner schema.Namer }

var _ schema.Namer = upperNamer{}

// TableName 实现 schema.Namer。
func (n upperNamer) TableName(table string) string {
	return strings.ToUpper(n.inner.TableName(table))
}

// SchemaName 实现 schema.Namer。
func (n upperNamer) SchemaName(table string) string {
	return strings.ToUpper(n.inner.SchemaName(table))
}

// ColumnName 实现 schema.Namer。
func (n upperNamer) ColumnName(table, column string) string {
	return strings.ToUpper(n.inner.ColumnName(table, column))
}

// JoinTableName 实现 schema.Namer。
func (n upperNamer) JoinTableName(joinTable string) string {
	return strings.ToUpper(n.inner.JoinTableName(joinTable))
}

// RelationshipFKName 实现 schema.Namer。
//
//nolint:gocritic // 值参数为 gorm Namer 接口固定签名，无法改指针
func (n upperNamer) RelationshipFKName(rel schema.Relationship) string {
	return strings.ToUpper(n.inner.RelationshipFKName(rel))
}

// CheckerName 实现 schema.Namer。
func (n upperNamer) CheckerName(table, column string) string {
	return strings.ToUpper(n.inner.CheckerName(table, column))
}

// IndexName 实现 schema.Namer。
func (n upperNamer) IndexName(table, column string) string {
	return strings.ToUpper(n.inner.IndexName(table, column))
}

// UniqueName 实现 schema.Namer。
func (n upperNamer) UniqueName(table, column string) string {
	return strings.ToUpper(n.inner.UniqueName(table, column))
}
