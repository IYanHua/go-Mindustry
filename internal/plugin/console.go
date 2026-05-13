package plugin

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// ConsoleCommand 描述一个控制台命令。
type ConsoleCommand struct {
	Name        string
	Description string
	Category    string
	Handler     func(args []string) error
}

// ConsoleCommandRegistry 管理插件注册的控制台命令。
type ConsoleCommandRegistry struct {
	mu       sync.RWMutex
	commands map[string]ConsoleCommand
}

// NewConsoleCommandRegistry 创建一个新的控制台命令注册表。
func NewConsoleCommandRegistry() *ConsoleCommandRegistry {
	return &ConsoleCommandRegistry{
		commands: make(map[string]ConsoleCommand),
	}
}

// Register 注册一个控制台命令。如果名称冲突则返回错误。
func (r *ConsoleCommandRegistry) Register(cmd ConsoleCommand) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := strings.ToLower(cmd.Name)
	if _, exists := r.commands[name]; exists {
		return fmt.Errorf("console command %q already registered", name)
	}
	r.commands[name] = cmd
	return nil
}

// Handle 查找并执行匹配的命令。返回 true 表示已处理。
func (r *ConsoleCommandRegistry) Handle(cmd string, args []string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.commands[strings.ToLower(cmd)]
	if !ok {
		return false
	}
	_ = c.Handler(args)
	return true
}

// Commands 返回所有已注册命令的列表（按名称排序）。
func (r *ConsoleCommandRegistry) Commands() []ConsoleCommand {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ConsoleCommand, 0, len(r.commands))
	for _, c := range r.commands {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// CommandsByCategory 按类别分组返回命令。
func (r *ConsoleCommandRegistry) CommandsByCategory() map[string][]ConsoleCommand {
	r.mu.RLock()
	defer r.mu.RUnlock()
	groups := make(map[string][]ConsoleCommand)
	for _, c := range r.commands {
		cat := c.Category
		if cat == "" {
			cat = "plugin"
		}
		groups[cat] = append(groups[cat], c)
	}
	return groups
}
