package plugin

import (
	"github.com/IYanHua/mdt-server/internal/config"
)

// Context 持有插件所需的所有服务器上下文。
type Context struct {
	// 核心依赖
	Server ServerInterface
	World  WorldInterface
	Config *config.Config

	// 命令注册表
	ConsoleCommands *ConsoleCommandRegistry
	ChatCommands    *ChatCommandRegistry

	// 事件总线
	Events *EventBus

	// 日志
	Logger *Logger
}

// NewContext 创建一个新的插件上下文。
func NewContext(
	srv ServerInterface,
	wld WorldInterface,
	cfg *config.Config,
	console *ConsoleCommandRegistry,
	chat *ChatCommandRegistry,
	events *EventBus,
	logger *Logger,
) *Context {
	return &Context{
		Server:          srv,
		World:           wld,
		Config:          cfg,
		ConsoleCommands: console,
		ChatCommands:    chat,
		Events:          events,
		Logger:          logger,
	}
}
