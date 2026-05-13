package plugin

// EventBus 提供多处理器事件分发，允许多个插件订阅同一事件。
type EventBus struct {
	OnPlayerJoin  []PlayerJoinHandler
	OnPlayerLeave []PlayerLeaveHandler
	OnChat        []ChatHandler
	OnTick        []TickHandler
	OnConfigReload []ConfigReloadHandler
	OnShutdown    []ShutdownHandler
}

// PlayerJoinHandler 在玩家加入时调用。
type PlayerJoinHandler func(conn ConnInterface)

// PlayerLeaveHandler 在玩家离开时调用。
type PlayerLeaveHandler func(conn ConnInterface)

// ChatHandler 在收到聊天消息时调用。返回 true 表示消息已被处理（停止传播）。
type ChatHandler func(conn ConnInterface, message string) bool

// TickHandler 在每个游戏 tick 后调用。
type TickHandler func()

// ConfigReloadHandler 在配置热重载时调用。
type ConfigReloadHandler func()

// ShutdownHandler 在服务器关闭时调用。
type ShutdownHandler func()

// DispatchPlayerJoin 通知所有订阅者玩家加入。
func (eb *EventBus) DispatchPlayerJoin(conn ConnInterface) {
	for _, h := range eb.OnPlayerJoin {
		h(conn)
	}
}

// DispatchPlayerLeave 通知所有订阅者玩家离开。
func (eb *EventBus) DispatchPlayerLeave(conn ConnInterface) {
	for _, h := range eb.OnPlayerLeave {
		h(conn)
	}
}

// DispatchChat 通知所有聊天处理器。返回 true 表示消息已被某个处理器消费。
func (eb *EventBus) DispatchChat(conn ConnInterface, message string) bool {
	for _, h := range eb.OnChat {
		if h(conn, message) {
			return true
		}
	}
	return false
}

// DispatchTick 通知所有 tick 处理器。
func (eb *EventBus) DispatchTick() {
	for _, h := range eb.OnTick {
		h()
	}
}

// DispatchConfigReload 通知所有配置重载处理器。
func (eb *EventBus) DispatchConfigReload() {
	for _, h := range eb.OnConfigReload {
		h()
	}
}

// DispatchShutdown 通知所有关闭处理器。
func (eb *EventBus) DispatchShutdown() {
	for _, h := range eb.OnShutdown {
		h()
	}
}
