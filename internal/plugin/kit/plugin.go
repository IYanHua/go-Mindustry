// Package kit 为插件开发者提供辅助工具，简化插件开发。
package kit

import (
	"github.com/IYanHua/mdt-server/internal/config"
	"github.com/IYanHua/mdt-server/internal/plugin"
)

// BasePlugin 提供 Plugin 接口的基础实现。
// 插件嵌入此结构体即可减少样板代码。
type BasePlugin struct {
	IDStr   string
	Version string
}

// ID 返回插件 ID。
func (p *BasePlugin) ID() string { return p.IDStr }

// Init 默认空实现。
func (p *BasePlugin) Init(ctx *plugin.Context) error { return nil }

// Start 默认空实现。
func (p *BasePlugin) Start() error { return nil }

// Stop 默认空实现。
func (p *BasePlugin) Stop() error { return nil }

// Version 返回插件版本（可选）。
func (p *BasePlugin) VersionString() string { return p.Version }

// ---- 可选接口 ----

// ReloadablePlugin 表示插件支持配置热重载。
type ReloadablePlugin interface {
	plugin.Plugin
	ReloadConfig(cfg *config.Config) error
}
