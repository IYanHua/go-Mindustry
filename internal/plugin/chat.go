package plugin

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// ChatCommand 描述一个聊天 /slash 命令。
type ChatCommand struct {
	Name        string
	Description string
	Permission  string // "op", "admin", "" 表示所有
	Handler     func(conn ConnInterface, args []string) bool
}

// ChatCommandRegistry 管理插件注册的聊天命令。
type ChatCommandRegistry struct {
	mu       sync.RWMutex
	commands map[string]ChatCommand
}

// NewChatCommandRegistry 创建一个新的聊天命令注册表。
func NewChatCommandRegistry() *ChatCommandRegistry {
	return &ChatCommandRegistry{
		commands: make(map[string]ChatCommand),
	}
}

// Register 注册一个聊天命令。如果名称冲突则返回错误。
func (r *ChatCommandRegistry) Register(cmd ChatCommand) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := strings.ToLower(strings.TrimPrefix(cmd.Name, "/"))
	if _, exists := r.commands[name]; exists {
		return fmt.Errorf("chat command /%s already registered", name)
	}
	r.commands[name] = cmd
	return nil
}

// Handle 查找并执行匹配的命令。返回 true 表示已处理。
func (r *ChatCommandRegistry) Handle(cmd string, conn ConnInterface, args []string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.commands[strings.ToLower(cmd)]
	if !ok {
		return false
	}
	return c.Handler(conn, args)
}

// Commands 返回所有已注册命令的列表（按名称排序）。
func (r *ChatCommandRegistry) Commands() []ChatCommand {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ChatCommand, 0, len(r.commands))
	for _, c := range r.commands {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
