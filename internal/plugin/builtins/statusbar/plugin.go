package statusbar

import (
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/IYanHua/mdt-server/internal/config"
	"github.com/IYanHua/mdt-server/internal/plugin"
)

type runtimeConfig struct {
	Config         config.StatusBarConfig
	ServerName     string
	VirtualPlayers int
}

type StatusBarPlugin struct {
	srv       plugin.ServerInterface
	cfg       atomic.Value // *runtimeConfig
	stopCh    chan struct{}
	startTime time.Time
}

func (p *StatusBarPlugin) ID() string { return "statusbar" }

func (p *StatusBarPlugin) Init(ctx *plugin.Context) error {
	p.srv = ctx.Server
	p.startTime = time.Now()
	p.loadConfig(ctx.Config)

	ctx.Events.OnConfigReload = append(ctx.Events.OnConfigReload, func() {
		p.loadConfig(ctx.Config)
	})
	return nil
}

func (p *StatusBarPlugin) Start() error {
	p.stopCh = make(chan struct{})
	go p.loop()
	return nil
}

func (p *StatusBarPlugin) Stop() error {
	close(p.stopCh)
	return nil
}

func (p *StatusBarPlugin) loadConfig(cfg *config.Config) {
	p.cfg.Store(&runtimeConfig{
		Config:         cfg.StatusBar,
		ServerName:     cfg.Runtime.ServerName,
		VirtualPlayers: cfg.Runtime.VirtualPlayers,
	})
}

func (p *StatusBarPlugin) currentConfig() *runtimeConfig {
	if v := p.cfg.Load(); v != nil {
		return v.(*runtimeConfig)
	}
	return &runtimeConfig{}
}

func (p *StatusBarPlugin) loop() {
	cpuTracker := newProcessCPUTracker()
	for {
		select {
		case <-p.stopCh:
			return
		default:
		}
		cfg := p.currentConfig()
		interval := time.Duration(cfg.Config.RefreshIntervalSec) * time.Second
		if interval <= 0 {
			interval = 2 * time.Second
		}
		if cfg.Config.Enabled {
			cpuPercent := cpuTracker.Sample()
			memoryMB := currentProcessMemoryMB()
			for _, c := range p.srv.ListConnectedConns() {
				message := p.renderMessage(cfg, cpuPercent, memoryMB, c)
				if strings.TrimSpace(message) != "" {
					p.srv.SendInfoPopup(
						c, message,
						float32(cfg.Config.PopupDurationMs)/1000,
						statusBarAlignValue(cfg.Config.Align),
						int32(cfg.Config.Top),
						int32(cfg.Config.Left),
						int32(cfg.Config.Bottom),
						int32(cfg.Config.Right),
					)
				}
			}
		}
		time.Sleep(interval)
	}
}

func (p *StatusBarPlugin) renderMessage(cfg *runtimeConfig, cpuPercent, memoryMB float64, c plugin.ConnInterface) string {
	players := p.srv.SessionCount() + max(cfg.VirtualPlayers, 0)
	repl := strings.NewReplacer(
		"{server_name}", strings.TrimSpace(cfg.ServerName),
		"{cpu_percent}", formatFloat(cpuPercent),
		"{memory_mb}", formatFloat(memoryMB),
		"{players}", strconv.Itoa(players),
		"{current_map}", strings.TrimSpace(p.srv.MapName()),
		"{game_time}", p.gameTimeString(),
		"{player_name}", p.playerName(c),
		"{qq_group}", strings.TrimSpace(cfg.Config.QQGroupText),
		"{message}", strings.TrimSpace(cfg.Config.CustomMessageText),
		"{uptime}", time.Since(p.startTime).Truncate(time.Second).String(),
	)

	lines := make([]string, 0, 10)
	if cfg.Config.HeaderEnabled {
		lines = appendNonBlank(lines, repl.Replace(cfg.Config.HeaderText))
	}
	if cfg.Config.ServerNameEnabled {
		lines = appendNonBlank(lines, repl.Replace(cfg.Config.ServerNameFormat))
	}
	if cfg.Config.PerformanceEnabled {
		lines = appendNonBlank(lines, repl.Replace(cfg.Config.PerformanceFormat))
	}
	if cfg.Config.CurrentMapEnabled {
		lines = appendNonBlank(lines, repl.Replace(cfg.Config.CurrentMapFormat))
	}
	if cfg.Config.GameTimeEnabled {
		lines = appendNonBlank(lines, repl.Replace(cfg.Config.GameTimeFormat))
	}
	if cfg.Config.PlayerCountEnabled {
		lines = appendNonBlank(lines, repl.Replace(cfg.Config.PlayerCountFormat))
	}
	if cfg.Config.WelcomeEnabled {
		lines = appendNonBlank(lines, repl.Replace(cfg.Config.WelcomeFormat))
	}
	if cfg.Config.QQGroupEnabled {
		lines = appendNonBlank(lines, repl.Replace(cfg.Config.QQGroupFormat))
	}
	if cfg.Config.CustomMessageEnabled {
		lines = appendNonBlank(lines, repl.Replace(cfg.Config.CustomMessageFormat))
	}
	return strings.Join(lines, "\n")
}

func (p *StatusBarPlugin) gameTimeString() string {
	seconds := p.srv.GameTimeSeconds()
	if seconds > 0 {
		return formatDuration(time.Duration(seconds) * time.Second)
	}
	return formatDuration(time.Since(p.startTime))
}

func (p *StatusBarPlugin) playerName(c plugin.ConnInterface) string {
	if c == nil {
		return "Player"
	}
	name := strings.TrimSpace(p.srv.PlayerDisplayName(c))
	if name == "" {
		name = strings.TrimSpace(c.Name())
	}
	if name == "" {
		return "Player"
	}
	return name
}

func appendNonBlank(lines []string, line string) []string {
	if strings.TrimSpace(line) == "" {
		return lines
	}
	return append(lines, line)
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d.Seconds())
	hours := total / 3600
	minutes := (total % 3600) / 60
	seconds := total % 60
	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func formatFloat(v float64) string {
	if v < 0 {
		v = 0
	}
	return fmt.Sprintf("%.1f", v)
}

func statusBarAlignValue(raw string) int32 {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "top_left", "topleft":
		return 10
	case "top":
		return 3
	case "top_right", "topright":
		return 18
	case "left":
		return 9
	case "center":
		return 1
	case "right":
		return 17
	case "bottom_left", "bottomleft":
		return 12
	case "bottom":
		return 5
	case "bottom_right", "bottomright":
		return 20
	default:
		return 10
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
