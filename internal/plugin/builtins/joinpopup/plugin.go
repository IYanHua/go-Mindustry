package joinpopup

import (
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/IYanHua/mdt-server/internal/config"
	"github.com/IYanHua/mdt-server/internal/plugin"
)

const (
	joinPopupMenuID    int32 = 910001
	helpPageBaseMenuID int32 = 910020
	helpPageSize             = 10
)

const (
	joinPopupOptionOpenURI int32 = 0
	joinPopupOptionHelp    int32 = 1
	joinPopupOptionVoteMap int32 = 2
	joinPopupOptionClose   int32 = 3
)

type runtimeConfig struct {
	Config         config.JoinPopupConfig
	ServerName     string
	VirtualPlayers int
}

type helpPage struct {
	Title   string
	Message string
	Buttons []helpCommandButton
}

type helpCommandButton struct {
	Label        string
	RunText      string
	UsageMessage string
}

// JoinPopupPlugin 在玩家加入时显示自定义弹窗菜单。
type JoinPopupPlugin struct {
	srv           plugin.ServerInterface
	chatCommands  *plugin.ChatCommandRegistry
	cfg           atomic.Value // *runtimeConfig
	showMapVote   func(connID int32, page int)
	mapVoteChoice func(connID int32, menuID, option int32) bool
}

func (p *JoinPopupPlugin) ID() string { return "joinpopup" }

func (p *JoinPopupPlugin) Init(ctx *plugin.Context) error {
	p.srv = ctx.Server
	p.chatCommands = ctx.ChatCommands
	p.loadConfig(ctx.Config)

	ctx.Events.OnConfigReload(func() {
		p.loadConfig(ctx.Config)
	})
	ctx.Events.OnPlayerJoin(func(c plugin.ConnInterface) {
		p.showForConn(c)
	})
	return nil
}

func (p *JoinPopupPlugin) Start() error { return nil }
func (p *JoinPopupPlugin) Stop() error  { return nil }

// SetMapVoteHandlers 设置地图投票相关回调。
func (p *JoinPopupPlugin) SetMapVoteHandlers(
	showVote func(connID int32, page int),
	voteChoice func(connID int32, menuID, option int32) bool,
) {
	p.showMapVote = showVote
	p.mapVoteChoice = voteChoice
}

func (p *JoinPopupPlugin) loadConfig(cfg *config.Config) {
	p.cfg.Store(&runtimeConfig{
		Config:         cfg.JoinPopup,
		ServerName:     cfg.Runtime.ServerName,
		VirtualPlayers: cfg.Runtime.VirtualPlayers,
	})
}

func (p *JoinPopupPlugin) currentConfig() *runtimeConfig {
	if v := p.cfg.Load(); v != nil {
		return v.(*runtimeConfig)
	}
	return &runtimeConfig{}
}

// HandleMenuChoice 处理加入弹窗及相关菜单的选择。返回 true 表示已处理。
func (p *JoinPopupPlugin) HandleMenuChoice(c plugin.ConnInterface, menuID, option int32) bool {
	if c == nil || option < 0 {
		return false
	}
	if p.mapVoteChoice != nil && p.mapVoteChoice(c.ConnID(), menuID, option) {
		return true
	}
	switch menuID {
	case joinPopupMenuID:
		switch option {
		case joinPopupOptionOpenURI:
			uri := p.render(c, p.currentConfig().Config.LinkURL)
			if strings.TrimSpace(uri) == "" {
				p.srv.SendInfoMessage(c, "[scarlet]当前未配置公告链接。[]")
				return true
			}
			p.srv.SendOpenURI(c, uri)
			return true
		case joinPopupOptionHelp:
			p.showHelpPage(c, 0)
			return true
		case joinPopupOptionVoteMap:
			if p.showMapVote != nil {
				p.showMapVote(c.ConnID(), 0)
			}
			return true
		case joinPopupOptionClose:
			return true
		}
	default:
		return p.handleHelpPageChoice(c, menuID, option)
	}
	return false
}

// ShowHelp 显示帮助首页。
func (p *JoinPopupPlugin) ShowHelp(c plugin.ConnInterface) {
	p.showHelpPage(c, 0)
}

func (p *JoinPopupPlugin) showForConn(c plugin.ConnInterface) {
	if c == nil {
		return
	}
	cfg := p.currentConfig()
	if !cfg.Config.Enabled {
		return
	}
	if delay := time.Duration(cfg.Config.DelayMs) * time.Millisecond; delay > 0 {
		time.Sleep(delay)
	}
	p.showJoinMenu(c)
}

func (p *JoinPopupPlugin) showJoinMenu(c plugin.ConnInterface) {
	if c == nil {
		return
	}
	cfg := p.currentConfig()
	title := p.render(c, cfg.Config.Title)
	message := p.buildMessage(c)
	if strings.TrimSpace(title) == "" && strings.TrimSpace(message) == "" {
		return
	}
	p.srv.SendMenu(c, joinPopupMenuID, title, message, [][]string{
		{"打开链接", "帮助"},
		{"投票换图", "关闭"},
	})
}

func (p *JoinPopupPlugin) buildMessage(c plugin.ConnInterface) string {
	cfg := p.currentConfig()
	intro := p.render(c, cfg.Config.Message)
	announcement := p.render(c, cfg.Config.AnnouncementText)
	switch {
	case strings.TrimSpace(intro) == "":
		return announcement
	case strings.TrimSpace(announcement) == "":
		return intro
	default:
		return intro + "\n\n" + announcement
	}
}

func (p *JoinPopupPlugin) showHelpPage(c plugin.ConnInterface, page int) {
	if c == nil {
		return
	}
	pages := p.helpPages(c)
	if len(pages) == 0 {
		return
	}
	if page < 0 {
		page = 0
	}
	if page >= len(pages) {
		page = len(pages) - 1
	}
	pg := pages[page]
	p.srv.SendMenu(c, helpPageBaseMenuID+int32(page), pg.Title, pg.Message, p.helpPageOptions(pg, page, len(pages)))
}

func (p *JoinPopupPlugin) helpPages(c plugin.ConnInterface) []helpPage {
	cfg := p.currentConfig()
	intro := p.render(c, cfg.Config.HelpText)
	buttons := p.helpCommandButtons()
	if len(buttons) == 0 {
		return nil
	}
	totalPages := (len(buttons) + helpPageSize - 1) / helpPageSize
	pages := make([]helpPage, 0, totalPages)
	for pageIdx := 0; pageIdx < totalPages; pageIdx++ {
		start := pageIdx * helpPageSize
		end := min(start+helpPageSize, len(buttons))
		msgLines := make([]string, 0, 3)
		if pageIdx == 0 && strings.TrimSpace(intro) != "" {
			msgLines = append(msgLines, intro)
			msgLines = append(msgLines, "")
		}
		msgLines = append(msgLines, "[accent]点击下方命令按钮会直接执行对应命令。[]")
		msgLines = append(msgLines, "[gray]需要参数的命令会在聊天框提示用法。[]")
		pages = append(pages, helpPage{
			Title:   "[accent]帮助 " + strconv.Itoa(pageIdx+1) + "/" + strconv.Itoa(totalPages) + "[]",
			Message: strings.Join(compactHelpLines(msgLines), "\n"),
			Buttons: append([]helpCommandButton(nil), buttons[start:end]...),
		})
	}
	return pages
}

func (p *JoinPopupPlugin) helpCommandButtons() []helpCommandButton {
	return []helpCommandButton{
		{Label: "/help\n打开帮助", RunText: "/help"},
		{Label: "/status\n查看状态", RunText: "/status"},
		{Label: "/sync\n重新同步", RunText: "/sync"},
		{Label: "/votemap\n投票换图", RunText: "/votemap"},
		{Label: "/vote\n投票页面", RunText: "/vote"},
		{Label: "/kill\n清除单位", RunText: "/kill"},
		{Label: "/stop\nOP停服", RunText: "/stop"},
		{Label: "/summon\nOP召唤", UsageMessage: "[scarlet]用法: /summon <typeId|unitName> [x y] [count] [team][]"},
		{Label: "/despawn\nOP移除", UsageMessage: "[scarlet]用法: /despawn <entityId>[]"},
		{Label: "/umove\n单位速度", UsageMessage: "[scarlet]用法: /umove <entityId> <vx> <vy> [rotVel][]"},
		{Label: "/uteleport\n单位传送", UsageMessage: "[scarlet]用法: /uteleport <entityId> <x> <y> [rotation][]"},
		{Label: "/ulife\n单位寿命", UsageMessage: "[scarlet]用法: /ulife <entityId> <seconds>[]"},
		{Label: "/ufollow\n单位跟随", UsageMessage: "[scarlet]用法: /ufollow <entityId> <targetId> [speed][]"},
		{Label: "/upatrol\n单位巡逻", UsageMessage: "[scarlet]用法: /upatrol <entityId> <x1> <y1> <x2> <y2> [speed][]"},
		{Label: "/ubehavior\n清除行为", UsageMessage: "[scarlet]用法: /ubehavior clear <entityId>[]"},
	}
}

func (p *JoinPopupPlugin) helpPageOptions(pg helpPage, page, total int) [][]string {
	options := make([][]string, 0, len(pg.Buttons)+1)
	for _, button := range pg.Buttons {
		options = append(options, []string{button.Label})
	}
	prevLabel := "[gray]上一页[]"
	nextLabel := "[gray]下一页[]"
	if page > 0 {
		prevLabel = "上一页"
	}
	if page+1 < total {
		nextLabel = "下一页"
	}
	options = append(options, []string{prevLabel, "关闭", nextLabel})
	return options
}

func (p *JoinPopupPlugin) handleHelpPageChoice(c plugin.ConnInterface, menuID, option int32) bool {
	pages := p.helpPages(c)
	if len(pages) == 0 {
		return false
	}
	page := int(menuID - helpPageBaseMenuID)
	if page < 0 || page >= len(pages) {
		return false
	}
	buttons := pages[page].Buttons
	if option >= 0 && option < int32(len(buttons)) {
		p.runHelpButton(c, buttons[option])
		return true
	}
	option -= int32(len(buttons))
	if option < 0 || option > 2 {
		return false
	}
	switch option {
	case 0:
		if page > 0 {
			p.showHelpPage(c, page-1)
		}
		return true
	case 1:
		return true
	case 2:
		if page+1 < len(pages) {
			p.showHelpPage(c, page+1)
		}
		return true
	}
	return false
}

func (p *JoinPopupPlugin) runHelpButton(c plugin.ConnInterface, button helpCommandButton) {
	if c == nil {
		return
	}
	runText := strings.TrimSpace(button.RunText)
	if runText != "" && strings.HasPrefix(runText, "/") {
		parts := strings.Fields(runText)
		cmdName := strings.TrimPrefix(parts[0], "/")
		args := parts[1:]
		if p.chatCommands != nil && p.chatCommands.Handle(cmdName, c, args) {
			return
		}
	}
	if msg := strings.TrimSpace(button.UsageMessage); msg != "" {
		p.srv.SendChat(c, msg)
	}
}

func (p *JoinPopupPlugin) render(c plugin.ConnInterface, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	cfg := p.currentConfig()
	serverName := strings.TrimSpace(cfg.ServerName)
	if serverName == "" {
		serverName = "mdt-server"
	}
	playerName := strings.TrimSpace(p.srv.PlayerDisplayName(c))
	if playerName == "" {
		playerName = strings.TrimSpace(c.Name())
	}
	if playerName == "" {
		playerName = "玩家"
	}
	currentMap := strings.TrimSpace(p.srv.MapName())
	if currentMap == "" {
		currentMap = "unknown"
	}
	players := p.srv.SessionCount() + max(cfg.VirtualPlayers, 0)
	return strings.NewReplacer(
		"{server_name}", serverName,
		"{player_name}", playerName,
		"{current_map}", currentMap,
		"{players}", strconv.Itoa(players),
		"{link_url}", strings.TrimSpace(cfg.Config.LinkURL),
	).Replace(raw)
}

func compactHelpLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	prevBlank := true
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if !prevBlank {
				out = append(out, "")
			}
			prevBlank = true
			continue
		}
		out = append(out, line)
		prevBlank = false
	}
	if len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	return out
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
