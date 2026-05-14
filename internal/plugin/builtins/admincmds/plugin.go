package admincmds

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/IYanHua/mdt-server/internal/plugin"
	"github.com/IYanHua/mdt-server/internal/world"
)

// Plugin provides administration and diagnostic console commands.
type Plugin struct {
	ctx *plugin.Context
}

func (p *Plugin) ID() string { return "builtins/admincmds" }

func (p *Plugin) Init(ctx *plugin.Context) error {
	p.ctx = ctx

	ctx.ConsoleCommands.Register(plugin.ConsoleCommand{
		Name: "players", Description: "列出当前在线玩家",
		Category: "admin", Handler: p.handlePlayers,
	})
	ctx.ConsoleCommands.Register(plugin.ConsoleCommand{
		Name: "uuid", Description: "查看玩家UUID: uuid [conn-id]",
		Category: "admin", Handler: p.handleUUID,
	})
	ctx.ConsoleCommands.Register(plugin.ConsoleCommand{
		Name: "info", Description: "显示服务器/世界基本信息",
		Category: "admin", Handler: p.handleInfo,
	})
	ctx.ConsoleCommands.Register(plugin.ConsoleCommand{
		Name: "pause", Description: "暂停/恢复游戏: pause [on|off]",
		Category: "admin", Handler: p.handlePause,
	})
	ctx.ConsoleCommands.Register(plugin.ConsoleCommand{
		Name: "gameover", Description: "设置游戏结束: gameover [on|off]",
		Category: "admin", Handler: p.handleGameOver,
	})
	ctx.ConsoleCommands.Register(plugin.ConsoleCommand{
		Name: "runwave", Description: "触发下一波: runwave [count]",
		Category: "admin", Handler: p.handleRunWave,
	})
	ctx.ConsoleCommands.Register(plugin.ConsoleCommand{
		Name: "fillitems", Description: "填满所有队伍核心物品: fillitems [team]",
		Category: "admin", Handler: p.handleFillItems,
	})
	ctx.ConsoleCommands.Register(plugin.ConsoleCommand{
		Name: "say", Description: "广播消息到服务器: say <message>",
		Category: "admin", Handler: p.handleSay,
	})
	ctx.ConsoleCommands.Register(plugin.ConsoleCommand{
		Name: "host", Description: "显示主机信息: host [name|desc|players]",
		Category: "admin", Handler: p.handleHost,
	})
	ctx.ConsoleCommands.Register(plugin.ConsoleCommand{
		Name: "ip", Description: "显示服务器IP和端口",
		Category: "admin", Handler: p.handleIP,
	})
	return nil
}

func (p *Plugin) Start() error { return nil }
func (p *Plugin) Stop() error  { return nil }

func (p *Plugin) handlePlayers(args []string) error {
	srv := p.ctx.Server
	conns := srv.ListConnectedConns()
	if len(conns) == 0 {
		fmt.Println("当前无在线连接")
		return nil
	}
	for _, c := range conns {
		fmt.Printf("id=%d connected=true ip=%s uuid=%s name=%q\n", c.ConnID(), c.RemoteAddrString(), c.UUID(), c.Name())
	}
	return nil
}

func (p *Plugin) handleUUID(args []string) error {
	srv := p.ctx.Server
	conns := srv.ListConnectedConns()
	if len(args) == 0 {
		if len(conns) == 0 {
			fmt.Println("当前无在线连接")
			return nil
		}
		for _, c := range conns {
			fmt.Printf("id=%d uuid=%s name=%q ip=%s\n", c.ConnID(), c.UUID(), c.Name(), c.RemoteAddrString())
		}
		return nil
	}
	id, err := strconv.ParseInt(args[0], 10, 32)
	if err != nil {
		fmt.Println("用法: uuid [conn-id]")
		return nil
	}
	for _, c := range conns {
		if c.ConnID() == int32(id) {
			fmt.Printf("id=%d uuid=%s name=%q ip=%s\n", c.ConnID(), c.UUID(), c.Name(), c.RemoteAddrString())
			return nil
		}
	}
	fmt.Println("未找到该连接ID")
	return nil
}

func (p *Plugin) handleInfo(args []string) error {
	srv := p.ctx.Server
	wld := p.ctx.World
	cfg := p.ctx.Config

	name := srv.ServerName()
	conns := srv.ListConnectedConns()
	fmt.Printf("Server: %s\n", name)
	fmt.Printf("Online: %d\n", len(conns))
	fmt.Printf("Map: %s\n", srv.MapName())
	if wld != nil {
		snap := wld.Snapshot()
		fmt.Printf("Wave: %d\n", wld.CurrentWave())
		fmt.Printf("Tick: %d\n", snap.Tick)
		fmt.Printf("Paused: %v GameOver: %v\n", wld.IsPaused(), wld.IsGameOver())
		fmt.Printf("TPS: %d\n", snap.Tps)
	}
	if cfg != nil {
		fmt.Printf("Config: %s\n", cfg.Source)
	}
	return nil
}

func (p *Plugin) handlePause(args []string) error {
	wld := p.ctx.World
	if wld == nil {
		fmt.Println("世界未初始化")
		return nil
	}
	if len(args) == 0 {
		if wld.IsPaused() {
			fmt.Println("当前: 已暂停")
		} else {
			fmt.Println("当前: 运行中")
		}
		return nil
	}
	switch strings.ToLower(args[0]) {
	case "on", "1", "true":
		wld.SetPaused(true)
		fmt.Println("已暂停")
	case "off", "0", "false":
		wld.SetPaused(false)
		fmt.Println("已恢复")
	default:
		fmt.Println("用法: pause [on|off]")
	}
	return nil
}

func (p *Plugin) handleGameOver(args []string) error {
	wld := p.ctx.World
	if wld == nil {
		fmt.Println("世界未初始化")
		return nil
	}
	if len(args) == 0 {
		if wld.IsGameOver() {
			fmt.Println("当前: 已结束")
		} else {
			fmt.Println("当前: 游戏中")
		}
		return nil
	}
	switch strings.ToLower(args[0]) {
	case "on", "1", "true":
		wld.SetGameOver(true)
		fmt.Println("已设置游戏结束")
	case "off", "0", "false":
		wld.SetGameOver(false)
		fmt.Println("已取消游戏结束")
	default:
		fmt.Println("用法: gameover [on|off]")
	}
	return nil
}

func (p *Plugin) handleRunWave(args []string) error {
	wld := p.ctx.World
	if wld == nil {
		fmt.Println("世界未初始化")
		return nil
	}
	count := 1
	if len(args) > 0 {
		if n, err := strconv.Atoi(args[0]); err == nil && n > 0 {
			count = n
		}
	}
	for i := 0; i < count; i++ {
		wld.TriggerWave()
	}
	wave := wld.CurrentWave()
	fmt.Printf("已触发%d波，当前: %d\n", count, wave)
	return nil
}

func (p *Plugin) handleFillItems(args []string) error {
	wld := p.ctx.World
	if wld == nil {
		fmt.Println("世界未初始化")
		return nil
	}
	if len(args) == 0 {
		// Fill all teams by iterating common team IDs
		for _, team := range []world.TeamID{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15} {
			wld.FillTeamCoreItems(team)
		}
		fmt.Println("已填满所有队伍核心物品")
		return nil
	}
	team, err := strconv.ParseInt(args[0], 10, 32)
	if err != nil {
		fmt.Println("用法: fillitems [team-id]")
		return nil
	}
	wld.FillTeamCoreItems(world.TeamID(int32(team)))
	fmt.Printf("已填满队伍 %d 核心物品\n", team)
	return nil
}

func (p *Plugin) handleSay(args []string) error {
	if len(args) == 0 {
		fmt.Println("用法: say <message>")
		return nil
	}
	msg := strings.Join(args, " ")
	p.ctx.Server.BroadcastChat(fmt.Sprintf("[orange][SERVER] %s[]", msg))
	fmt.Printf("已广播: %s\n", msg)
	return nil
}

func (p *Plugin) handleHost(args []string) error {
	srv := p.ctx.Server
	cfg := p.ctx.Config

	if len(args) == 0 {
		name := ""
		if cfg != nil {
			name = cfg.Runtime.ServerName
		}
		fmt.Printf("Host: %s\n", name)
		fmt.Printf("Online: %d\n", len(srv.ListConnectedConns()))
		return nil
	}
	switch strings.ToLower(args[0]) {
	case "name":
		if cfg != nil {
			fmt.Println(cfg.Runtime.ServerName)
		}
	case "desc":
		if cfg != nil {
			fmt.Println(cfg.Runtime.ServerDesc)
		}
	case "players":
		conns := srv.ListConnectedConns()
		fmt.Printf("Online: %d\n", len(conns))
		for _, c := range conns {
			fmt.Printf("  %s (id=%d)\n", c.Name(), c.ConnID())
		}
	default:
		fmt.Println("用法: host [name|desc|players]")
	}
	return nil
}

func (p *Plugin) handleIP(args []string) error {
	cfg := p.ctx.Config
	if cfg == nil {
		fmt.Println("配置未加载")
		return nil
	}
	fmt.Printf("Bind: %s\n", cfg.API.Bind)
	return nil
}
