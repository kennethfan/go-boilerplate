// Package model 定义 GORM 数据模型。
package model

import "time"

// User 用户表（对应 migrations/000001_init_users.up.sql）
type User struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Username  string    `gorm:"column:username;size:64;uniqueIndex;not null" json:"username"`
	Password  string    `gorm:"column:password;size:128;not null" json:"-"` // bcrypt 密文，不出参
	Nickname  string    `gorm:"column:nickname;size:64;not null;default:''" json:"nickname"`
	Role      string    `gorm:"column:role;size:32;not null;default:'user'" json:"role"` // admin / user
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

// TableName 自定义表名
func (User) TableName() string { return "users" }
