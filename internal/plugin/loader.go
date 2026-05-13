package plugin

import (
	"fmt"
	"log"
	"sync"

	"github.com/IYanHua/mdt-server/internal/config"
)

// PluginState 描述插件的生命周期状态。
type PluginState int

const (
	StateLoaded  PluginState = iota
	StateInit
	StateStarted
	StateStopped
)

func (s PluginState) String() string {
	switch s {
	case StateLoaded:
		return "loaded"
	case StateInit:
		return "init"
	case StateStarted:
		return "started"
	case StateStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

type pluginInstance struct {
	plugin Plugin
	state  PluginState
	builtin bool
}

// Manager 管理所有插件的生命周期。
type Manager struct {
	mu           sync.Mutex
	instances    map[string]*pluginInstance
	ctx          *Context
	consoleReg   *ConsoleCommandRegistry
	chatReg      *ChatCommandRegistry
	eventBus     *EventBus
	builtins     []Plugin
}

// NewManager 创建一个新的插件管理器。
func NewManager() *Manager {
	return &Manager{
		instances:  make(map[string]*pluginInstance),
		consoleReg: NewConsoleCommandRegistry(),
		chatReg:    NewChatCommandRegistry(),
		eventBus:   &EventBus{},
	}
}

// ConsoleCommands 返回控制台命令注册表。
func (m *Manager) ConsoleCommands() *ConsoleCommandRegistry {
	return m.consoleReg
}

// ChatCommands 返回聊天命令注册表。
func (m *Manager) ChatCommands() *ChatCommandRegistry {
	return m.chatReg
}

// EventBus 返回事件总线。
func (m *Manager) EventBus() *EventBus {
	return m.eventBus
}

// RegisterBuiltin 注册一个内建插件（编译进二进制）。
func (m *Manager) RegisterBuiltin(p Plugin) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := p.ID()
	if _, exists := m.instances[id]; exists {
		return fmt.Errorf("plugin %q already registered", id)
	}
	m.builtins = append(m.builtins, p)
	m.instances[id] = &pluginInstance{plugin: p, state: StateLoaded, builtin: true}
	return nil
}

// InitAll 使用给定的上下文初始化所有已注册的插件。
func (m *Manager) InitAll(ctx *Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ctx = ctx

	for _, inst := range m.instances {
		if inst.state != StateLoaded {
			continue
		}
		log.Printf("[plugin] init %s", inst.plugin.ID())
		if err := inst.plugin.Init(ctx); err != nil {
			return fmt.Errorf("plugin %s init: %w", inst.plugin.ID(), err)
		}
		inst.state = StateInit
	}
	return nil
}

// StartAll 启动所有已初始化的插件。
func (m *Manager) StartAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, inst := range m.instances {
		if inst.state != StateInit {
			continue
		}
		log.Printf("[plugin] start %s", inst.plugin.ID())
		if err := inst.plugin.Start(); err != nil {
			return fmt.Errorf("plugin %s start: %w", inst.plugin.ID(), err)
		}
		inst.state = StateStarted
	}
	return nil
}

// StopAll 停止所有已启动的插件。
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, inst := range m.instances {
		if inst.state != StateStarted {
			continue
		}
		log.Printf("[plugin] stop %s", inst.plugin.ID())
		_ = inst.plugin.Stop()
		inst.state = StateStopped
	}
}

// LoadedPlugins 返回所有已加载插件的 ID 和状态。
func (m *Manager) LoadedPlugins() map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]string, len(m.instances))
	for id, inst := range m.instances {
		out[id] = inst.state.String()
	}
	return out
}

// Config 返回插件配置部分。
func ConfigSection(cfg *config.Config) *config.Config {
	return cfg
}
