// Package base 提供 ORM base 对象 BasePO：ID + 创建人/创建时间/修改人/修改时间
// 五个审计字段，业务 PO 嵌入即获得审计能力（钩子经 actor 上下文自动填充，
// 列名全大写对齐 Oracle DDL）。
package base
