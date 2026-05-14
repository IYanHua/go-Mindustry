package plugin

import "sync"

// EventBus 提供线程安全的多处理器事件分发，允许多个插件订阅同一事件。
// 通过 Subscribe 方法注册处理器并返回取消函数。
type EventBus struct {
	mu sync.RWMutex

	onPlayerJoin    []PlayerJoinHandler
	onPlayerLeave   []PlayerLeaveHandler
	onChat          []ChatHandler
	onTick          []TickHandler
	onConfigReload  []ConfigReloadHandler
	onShutdown      []ShutdownHandler
	onWorldLoad     []WorldLoadHandler
	onWaveStart     []WaveStartHandler
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

// WorldLoadHandler 在世界加载完成后调用。
type WorldLoadHandler func(mapName string)

// WaveStartHandler 在新一波开始时调用。
type WaveStartHandler func(wave int32)

// ----- Subscribe methods (线程安全，返回取消函数) -----

// OnPlayerJoin 订阅玩家加入事件。返回取消订阅的函数。
func (eb *EventBus) OnPlayerJoin(h PlayerJoinHandler) (unsubscribe func()) {
	if eb == nil {
		return func() {}
	}
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.onPlayerJoin = append(eb.onPlayerJoin, h)
	idx := len(eb.onPlayerJoin) - 1
	return func() { eb.removePlayerJoin(idx) }
}

// OnPlayerLeave 订阅玩家离开事件。
func (eb *EventBus) OnPlayerLeave(h PlayerLeaveHandler) (unsubscribe func()) {
	if eb == nil {
		return func() {}
	}
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.onPlayerLeave = append(eb.onPlayerLeave, h)
	idx := len(eb.onPlayerLeave) - 1
	return func() { eb.removePlayerLeave(idx) }
}

// OnChat 订阅聊天事件。
func (eb *EventBus) OnChat(h ChatHandler) (unsubscribe func()) {
	if eb == nil {
		return func() {}
	}
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.onChat = append(eb.onChat, h)
	idx := len(eb.onChat) - 1
	return func() { eb.removeChat(idx) }
}

// OnTick 订阅游戏 tick 事件。
func (eb *EventBus) OnTick(h TickHandler) (unsubscribe func()) {
	if eb == nil {
		return func() {}
	}
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.onTick = append(eb.onTick, h)
	idx := len(eb.onTick) - 1
	return func() { eb.removeTick(idx) }
}

// OnConfigReload 订阅配置重载事件。
func (eb *EventBus) OnConfigReload(h ConfigReloadHandler) (unsubscribe func()) {
	if eb == nil {
		return func() {}
	}
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.onConfigReload = append(eb.onConfigReload, h)
	idx := len(eb.onConfigReload) - 1
	return func() { eb.removeConfigReload(idx) }
}

// OnShutdown 订阅服务器关闭事件。
func (eb *EventBus) OnShutdown(h ShutdownHandler) (unsubscribe func()) {
	if eb == nil {
		return func() {}
	}
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.onShutdown = append(eb.onShutdown, h)
	idx := len(eb.onShutdown) - 1
	return func() { eb.removeShutdown(idx) }
}

// OnWorldLoad 订阅世界加载事件。
func (eb *EventBus) OnWorldLoad(h WorldLoadHandler) (unsubscribe func()) {
	if eb == nil {
		return func() {}
	}
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.onWorldLoad = append(eb.onWorldLoad, h)
	idx := len(eb.onWorldLoad) - 1
	return func() { eb.removeWorldLoad(idx) }
}

// OnWaveStart 订阅新波事件。
func (eb *EventBus) OnWaveStart(h WaveStartHandler) (unsubscribe func()) {
	if eb == nil {
		return func() {}
	}
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.onWaveStart = append(eb.onWaveStart, h)
	idx := len(eb.onWaveStart) - 1
	return func() { eb.removeWaveStart(idx) }
}

// ----- Remove methods (内部使用) -----

func (eb *EventBus) removePlayerJoin(idx int) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.onPlayerJoin[idx] = nil
}

func (eb *EventBus) removePlayerLeave(idx int) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.onPlayerLeave[idx] = nil
}

func (eb *EventBus) removeChat(idx int) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.onChat[idx] = nil
}

func (eb *EventBus) removeTick(idx int) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.onTick[idx] = nil
}

func (eb *EventBus) removeConfigReload(idx int) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.onConfigReload[idx] = nil
}

func (eb *EventBus) removeShutdown(idx int) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.onShutdown[idx] = nil
}

func (eb *EventBus) removeWorldLoad(idx int) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.onWorldLoad[idx] = nil
}

func (eb *EventBus) removeWaveStart(idx int) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.onWaveStart[idx] = nil
}

// ----- Dispatch methods (线程安全) -----

// DispatchPlayerJoin 通知所有订阅者玩家加入。
func (eb *EventBus) DispatchPlayerJoin(conn ConnInterface) {
	if eb == nil {
		return
	}
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	for _, h := range eb.onPlayerJoin {
		if h != nil {
			h(conn)
		}
	}
}

// DispatchPlayerLeave 通知所有订阅者玩家离开。
func (eb *EventBus) DispatchPlayerLeave(conn ConnInterface) {
	if eb == nil {
		return
	}
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	for _, h := range eb.onPlayerLeave {
		if h != nil {
			h(conn)
		}
	}
}

// DispatchChat 通知所有聊天处理器。返回 true 表示消息已被某个处理器消费。
func (eb *EventBus) DispatchChat(conn ConnInterface, message string) bool {
	if eb == nil {
		return false
	}
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	for _, h := range eb.onChat {
		if h != nil && h(conn, message) {
			return true
		}
	}
	return false
}

// DispatchTick 通知所有 tick 处理器。
func (eb *EventBus) DispatchTick() {
	if eb == nil {
		return
	}
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	for _, h := range eb.onTick {
		if h != nil {
			h()
		}
	}
}

// DispatchConfigReload 通知所有配置重载处理器。
func (eb *EventBus) DispatchConfigReload() {
	if eb == nil {
		return
	}
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	for _, h := range eb.onConfigReload {
		if h != nil {
			h()
		}
	}
}

// DispatchShutdown 通知所有关闭处理器。
func (eb *EventBus) DispatchShutdown() {
	if eb == nil {
		return
	}
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	for _, h := range eb.onShutdown {
		if h != nil {
			h()
		}
	}
}

// DispatchWorldLoad 通知世界加载完成。
func (eb *EventBus) DispatchWorldLoad(mapName string) {
	if eb == nil {
		return
	}
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	for _, h := range eb.onWorldLoad {
		if h != nil {
			h(mapName)
		}
	}
}

// DispatchWaveStart 通知新波开始。
func (eb *EventBus) DispatchWaveStart(wave int32) {
	if eb == nil {
		return
	}
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	for _, h := range eb.onWaveStart {
		if h != nil {
			h(wave)
		}
	}
}