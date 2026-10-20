// Package domain 定义业务实体与查询/存储接口（端口）：不含 HTTP、数据库
// 等技术细节，是全服务的核心词汇表。app 层依赖本层的接口编程，
// storage 层（oracle/memory）提供实现。
package domain

import "time"

// User 是用户实体：security-management 的核心聚合之一（角色、登录策略
// 见后续切片）。
type User struct {
	ID        string
	Username  string // 登录名，服务内唯一
	Name      string // 显示名（删除确认消息等面向用户展示用）
	Email     string
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy string
	UpdatedBy string
}
