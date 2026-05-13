package plugin

// Plugin 是所有插件必须实现的接口。
type Plugin interface {
	// ID 返回插件的唯一标识符。
	ID() string

	// Init 在插件加载后、Start 之前调用。用于注册命令和事件钩子。
	Init(ctx *Context) error

	// Start 在服务器就绪时调用。所有钩子此时已激活。
	Start() error

	// Stop 在服务器关闭时调用。插件应在此清理资源。
	Stop() error
}
