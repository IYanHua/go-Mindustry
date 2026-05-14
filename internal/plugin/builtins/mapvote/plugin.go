package mapvote

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IYanHua/mdt-server/internal/config"
	netpkg "github.com/IYanHua/mdt-server/internal/net"
	"github.com/IYanHua/mdt-server/internal/plugin"
	"github.com/IYanHua/mdt-server/internal/worldstream"
)

const (
	voteMapSelectMenuBaseID int32 = 910100
	voteMapPromptMenuID     int32 = 910200
	voteMapPageSize               = 4
)

const (
	mapVoteDecisionPending mapVoteDecision = iota
	mapVoteDecisionPassed
	mapVoteDecisionRejected
	mapVoteDecisionExpired
)

var errMapVoteActive = errors.New("已有换图投票进行中")

type mapVoteDecision int

type MapVoteResult struct {
	Decision  mapVoteDecision
	MapName   string
	WorldPath string
	StartedBy string
	Yes       int
	No        int
	Neutral   int
	Needed    int
}

type mapVoteSnapshot struct {
	MapName   string
	StartedBy string
	Yes       int
	No        int
	Neutral   int
	Needed    int
	ExpiresAt time.Time
}

type mapVoteSession struct {
	Token     uint64
	MapName   string
	WorldPath string
	StartedBy string
	Votes     map[string]int8
	ExpiresAt time.Time
	Timer     *time.Timer
}

type mapVoteRuntimeConfig struct {
	Duration      time.Duration
	StatusRefresh time.Duration
	PopupDuration float32
	HomeLinkURL   string
	Align         string
	Top           int
	Left          int
	Bottom        int
	Right         int
}

// ListMapsFunc is a callback to enumerate available maps.
type ListMapsFunc func() ([]string, error)

// ResolveWorldFunc resolves a map name to a world path.
type ResolveWorldFunc func(string) (string, error)

// ApplyWorldFunc applies a world change.
type ApplyWorldFunc func(string) error

// NotifyResultFunc is called with the final vote result.
type NotifyResultFunc func(MapVoteResult)

// MapVotePlugin implements plugin.Plugin for map voting.
type MapVotePlugin struct {
	mu          sync.Mutex
	cfg         *config.Config
	uiConfig    atomic.Value
	server      plugin.ServerInterface
	listMaps    ListMapsFunc
	resolveWorld ResolveWorldFunc
	applyWorld  ApplyWorldFunc
	notifyResult NotifyResultFunc
	duration    time.Duration
	nextToken   uint64
	active      *mapVoteSession
	statusToken uint64
	started     bool
}

func NewMapVotePlugin() *MapVotePlugin {
	return &MapVotePlugin{}
}

func (p *MapVotePlugin) ID() string { return "builtins/mapvote" }

func (p *MapVotePlugin) Init(ctx *plugin.Context) error {
	p.cfg = ctx.Config
	p.server = ctx.Server
	p.loadConfig(ctx.Config)

	ctx.ChatCommands.Register(plugin.ChatCommand{
		Name:        "votemap",
		Description: "打开换图投票菜单: /votemap [地图名]",
		Permission:  "",
		Handler:     p.handleChatVoteMap,
	})
	ctx.ChatCommands.Register(plugin.ChatCommand{
		Name:        "vote",
		Description: "参与换图投票: /vote yes|no|neutral",
		Permission:  "",
		Handler:     p.handleChatVote,
	})
	return nil
}

func (p *MapVotePlugin) Start() error {
	p.mu.Lock()
	p.started = true
	p.mu.Unlock()
	return nil
}

func (p *MapVotePlugin) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.active != nil && p.active.Timer != nil {
		p.active.Timer.Stop()
	}
	p.active = nil
	p.started = false
	return nil
}

// SetCallbacks wires the main.go world management functions into the plugin.
func (p *MapVotePlugin) SetCallbacks(listMaps ListMapsFunc, resolveWorld ResolveWorldFunc, applyWorld ApplyWorldFunc, notifyResult NotifyResultFunc) {
	p.listMaps = listMaps
	p.resolveWorld = resolveWorld
	p.applyWorld = applyWorld
	p.notifyResult = notifyResult
}

func (p *MapVotePlugin) loadConfig(cfg *config.Config) {
	p.uiConfig.Store(mapVoteConfigFrom(cfg.MapVote))
}

func mapVoteConfigFrom(cfg config.MapVoteConfig) mapVoteRuntimeConfig {
	duration := time.Duration(cfg.DurationSec) * time.Second
	if duration <= 0 {
		duration = 15 * time.Second
	}
	refresh := time.Duration(cfg.StatusRefreshMs) * time.Millisecond
	if refresh <= 0 {
		refresh = 1500 * time.Millisecond
	}
	popupDuration := float32(cfg.PopupDurationMs) / 1000
	if popupDuration <= 0 {
		popupDuration = 1.8
	}
	return mapVoteRuntimeConfig{
		Duration:      duration,
		StatusRefresh: refresh,
		PopupDuration: popupDuration,
		HomeLinkURL:   strings.TrimSpace(cfg.HomeLinkURL),
		Align:         strings.TrimSpace(cfg.Align),
		Top:           cfg.Top,
		Left:          cfg.Left,
		Bottom:        cfg.Bottom,
		Right:         cfg.Right,
	}
}

func (p *MapVotePlugin) currentConfig() mapVoteRuntimeConfig {
	if v := p.uiConfig.Load(); v != nil {
		if cfg, ok := v.(mapVoteRuntimeConfig); ok {
			return cfg
		}
	}
	return mapVoteConfigFrom(config.Default().MapVote)
}

func (p *MapVotePlugin) voteDuration() time.Duration {
	if p.duration > 0 {
		return p.duration
	}
	return p.currentConfig().Duration
}

func (p *MapVotePlugin) activeToken() uint64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.active == nil {
		return 0
	}
	return p.active.Token
}

func (p *MapVotePlugin) currentTotalPlayers() int {
	if p.server != nil {
		return len(p.server.ListConnectedConns())
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.active != nil {
		return max(len(p.active.Votes), 1)
	}
	return 1
}

func (p *MapVotePlugin) startStatusLoop(token uint64) {
	if p.server == nil || token == 0 {
		return
	}
	p.mu.Lock()
	if p.statusToken == token {
		p.mu.Unlock()
		return
	}
	p.statusToken = token
	p.mu.Unlock()
	go p.runStatusLoop(token)
}

func (p *MapVotePlugin) runStatusLoop(token uint64) {
	for {
		if !p.broadcastStatus(token) {
			return
		}
		time.Sleep(p.currentConfig().StatusRefresh)
	}
}

func (p *MapVotePlugin) BroadcastActiveStatus() {
	if token := p.activeToken(); token != 0 {
		_ = p.broadcastStatus(token)
	}
}

func (p *MapVotePlugin) broadcastStatus(token uint64) bool {
	if p.server == nil || token == 0 {
		return false
	}
	totalPlayers := len(p.server.ListConnectedConns())
	p.mu.Lock()
	if p.active == nil || p.active.Token != token {
		if p.statusToken == token {
			p.statusToken = 0
		}
		p.mu.Unlock()
		return false
	}
	if totalPlayers < 1 {
		totalPlayers = max(len(p.active.Votes), 1)
	}
	snapshot := buildMapVoteSnapshot(p.active, totalPlayers)
	p.mu.Unlock()
	p.broadcastStatusToServer(snapshot)
	return true
}

func (p *MapVotePlugin) broadcastStatusToServer(snapshot mapVoteSnapshot) {
	if p.server == nil {
		return
	}
	remaining := time.Until(snapshot.ExpiresAt)
	if remaining < 0 {
		remaining = 0
	}
	lines := []string{
		"[accent]换图投票[]",
		fmt.Sprintf("地图: [white]%s[]", snapshot.MapName),
		fmt.Sprintf("发起: %s", snapshot.StartedBy),
		fmt.Sprintf("同意: [green]%d[]/[white]%d[]  反对: [scarlet]%d[]  中立: [lightgray]%d[]", snapshot.Yes, snapshot.Needed, snapshot.No, snapshot.Neutral),
		fmt.Sprintf("剩余: [white]%.1fs[]", remaining.Seconds()),
	}
	p.server.BroadcastSetHudTextReliable(strings.Join(lines, "\n"))
}

func voteParticipantKey(c plugin.ConnInterface) string {
	if c == nil {
		return ""
	}
	if uuid := strings.ToLower(strings.TrimSpace(c.UUID())); uuid != "" {
		return uuid
	}
	return fmt.Sprintf("conn:%d", c.ConnID())
}

func (p *MapVotePlugin) voteParticipantName(c plugin.ConnInterface) string {
	if p.server != nil && c != nil {
		if name := strings.TrimSpace(p.server.PlayerDisplayName(c)); name != "" {
			return name
		}
	}
	if c != nil {
		if name := strings.TrimSpace(c.Name()); name != "" {
			return name
		}
	}
	return "玩家"
}

func neededVotes(totalPlayers int) int {
	if totalPlayers < 1 {
		totalPlayers = 1
	}
	return totalPlayers/2 + 1
}

func countMapVotesDetailed(votes map[string]int8) (yes, no, neutral int) {
	for _, vote := range votes {
		switch vote {
		case 1:
			yes++
		case -1:
			no++
		case 0:
			neutral++
		}
	}
	return yes, no, neutral
}

func evaluateMapVote(yes, no, totalPlayers, voted int) mapVoteDecision {
	need := neededVotes(totalPlayers)
	if yes >= need {
		return mapVoteDecisionPassed
	}
	if no >= need {
		return mapVoteDecisionRejected
	}
	if voted >= max(totalPlayers, 1) {
		return mapVoteDecisionRejected
	}
	return mapVoteDecisionPending
}

func (p *MapVotePlugin) snapshot(totalPlayers int) (mapVoteSnapshot, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.active == nil {
		return mapVoteSnapshot{}, false
	}
	return buildMapVoteSnapshot(p.active, totalPlayers), true
}

func buildMapVoteSnapshot(active *mapVoteSession, totalPlayers int) mapVoteSnapshot {
	if active == nil {
		return mapVoteSnapshot{}
	}
	yes, no, neutral := countMapVotesDetailed(active.Votes)
	return mapVoteSnapshot{
		MapName:   active.MapName,
		StartedBy: active.StartedBy,
		Yes:       yes,
		No:        no,
		Neutral:   neutral,
		Needed:    neededVotes(totalPlayers),
		ExpiresAt: active.ExpiresAt,
	}
}

func finalizeMapVote(active *mapVoteSession, decision mapVoteDecision, totalPlayers int) MapVoteResult {
	yes, no, neutral := countMapVotesDetailed(active.Votes)
	return MapVoteResult{
		Decision:  decision,
		MapName:   active.MapName,
		WorldPath: active.WorldPath,
		StartedBy: active.StartedBy,
		Yes:       yes,
		No:        no,
		Neutral:   neutral,
		Needed:    neededVotes(totalPlayers),
	}
}

func (p *MapVotePlugin) beginVote(worldPath, mapName, starterKey, starterName string, totalPlayers int) (mapVoteSnapshot, MapVoteResult, error) {
	now := time.Now()
	duration := p.voteDuration()
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.active != nil {
		return buildMapVoteSnapshot(p.active, totalPlayers), MapVoteResult{}, errMapVoteActive
	}
	p.nextToken++
	session := &mapVoteSession{
		Token:     p.nextToken,
		MapName:   mapName,
		WorldPath: worldPath,
		StartedBy: starterName,
		Votes: map[string]int8{
			starterKey: 1,
		},
		ExpiresAt: now.Add(duration),
	}
	yes, no, _ := countMapVotesDetailed(session.Votes)
	decision := evaluateMapVote(yes, no, totalPlayers, yes+no)
	if decision != mapVoteDecisionPending {
		return mapVoteSnapshot{}, finalizeMapVote(session, decision, totalPlayers), nil
	}
	session.Timer = time.AfterFunc(duration, func() {
		p.expireVote(session.Token)
	})
	p.active = session
	return buildMapVoteSnapshot(session, totalPlayers), MapVoteResult{Decision: mapVoteDecisionPending}, nil
}

func (p *MapVotePlugin) castVote(voterKey string, value int8, totalPlayers int) (mapVoteSnapshot, MapVoteResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.active == nil {
		return mapVoteSnapshot{}, MapVoteResult{}, errors.New("当前没有进行中的换图投票")
	}
	if value != 1 && value != -1 && value != 0 {
		return buildMapVoteSnapshot(p.active, totalPlayers), MapVoteResult{}, errors.New("无效投票")
	}
	p.active.Votes[voterKey] = value
	yes, no, _ := countMapVotesDetailed(p.active.Votes)
	decision := evaluateMapVote(yes, no, totalPlayers, yes+no)
	if decision == mapVoteDecisionPending {
		return buildMapVoteSnapshot(p.active, totalPlayers), MapVoteResult{Decision: mapVoteDecisionPending}, nil
	}
	finished := p.active
	if finished.Timer != nil {
		finished.Timer.Stop()
	}
	p.active = nil
	return mapVoteSnapshot{}, finalizeMapVote(finished, decision, totalPlayers), nil
}

func (p *MapVotePlugin) expireVote(token uint64) {
	p.mu.Lock()
	if p.active == nil || p.active.Token != token {
		p.mu.Unlock()
		return
	}
	finished := p.active
	p.active = nil
	p.mu.Unlock()
	result := finalizeMapVote(finished, mapVoteDecisionExpired, max(len(finished.Votes), 1))
	if p.notifyResult != nil {
		p.notifyResult(result)
		return
	}
	p.handleResult(result)
}

func (p *MapVotePlugin) listPage(page int) ([]string, int, int, error) {
	if p.listMaps == nil {
		return nil, 0, 0, errors.New("vote map list is unavailable")
	}
	maps, err := p.listMaps()
	if err != nil {
		return nil, 0, 0, err
	}
	totalPages := 1
	if len(maps) > 0 {
		totalPages = (len(maps) + voteMapPageSize - 1) / voteMapPageSize
	}
	if page < 0 {
		page = 0
	}
	if page >= totalPages {
		page = totalPages - 1
	}
	start := page * voteMapPageSize
	end := min(start+voteMapPageSize, len(maps))
	if start >= len(maps) {
		return nil, page, totalPages, nil
	}
	return maps[start:end], page, totalPages, nil
}

// ShowVoteMenu opens the map vote menu for a connection (used by joinpopup wiring).
func (p *MapVotePlugin) ShowVoteMenu(c plugin.ConnInterface, page int) {
	if p.server == nil || c == nil {
		return
	}
	if _, ok := p.snapshot(len(p.server.ListConnectedConns())); ok {
		p.ShowActiveVoteMenu(c)
		return
	}
	pageMaps, currentPage, totalPages, err := p.listPage(page)
	if err != nil {
		p.server.SendInfoMessage(c, fmt.Sprintf("[scarlet]读取地图列表失败: %s[]", err.Error()))
		return
	}
	if len(pageMaps) == 0 {
		p.server.SendInfoMessage(c, "[scarlet]当前没有可投票的地图。[]")
		return
	}
	lines := []string{
		"[accent]投票换图[]",
		fmt.Sprintf("当前地图: [white]%s[]", p.server.MapName()),
		fmt.Sprintf("第 [white]%d/%d[] 页，选择一张地图发起投票。", currentPage+1, totalPages),
	}
	p.server.SendMenu(c, voteMapSelectMenuBaseID+int32(currentPage), "[accent]投票换图[]", strings.Join(lines, "\n"), mapVoteSelectionOptions(pageMaps, currentPage, totalPages, p.currentConfig().HomeLinkURL != ""))
}

func mapVoteSelectionOptions(pageMaps []string, page, totalPages int, hasLink bool) [][]string {
	options := make([][]string, 0, 4)
	for i := 0; i < len(pageMaps); i += 2 {
		end := min(i+2, len(pageMaps))
		row := make([]string, 0, end-i)
		for _, name := range pageMaps[i:end] {
			row = append(row, name)
		}
		options = append(options, row)
	}
	nav := make([]string, 0, 2)
	if page > 0 {
		nav = append(nav, "上一页")
	}
	if page+1 < totalPages {
		nav = append(nav, "下一页")
	}
	if len(nav) > 0 {
		options = append(options, nav)
	}
	if hasLink {
		options = append(options, []string{"打开链接", "关闭"})
	} else {
		options = append(options, []string{"关闭"})
	}
	return options
}

// ShowActiveVoteMenu opens the active vote menu for a connection.
func (p *MapVotePlugin) ShowActiveVoteMenu(c plugin.ConnInterface) {
	if p.server == nil || c == nil {
		return
	}
	snapshot, ok := p.snapshot(len(p.server.ListConnectedConns()))
	if !ok {
		p.server.SendInfoMessage(c, "[scarlet]当前没有进行中的换图投票。[]")
		return
	}
	remaining := time.Until(snapshot.ExpiresAt)
	if remaining < 0 {
		remaining = 0
	}
	lines := []string{
		"[accent]换图投票[]",
		fmt.Sprintf("目标地图: [white]%s[]", snapshot.MapName),
		fmt.Sprintf("发起玩家: %s", snapshot.StartedBy),
		fmt.Sprintf("同意: [green]%d[]  反对: [scarlet]%d[]  中立: [lightgray]%d[]", snapshot.Yes, snapshot.No, snapshot.Neutral),
		fmt.Sprintf("通过需要: [white]%d[]  剩余: [white]%ds[]", snapshot.Needed, int(remaining/time.Second)),
	}
	p.server.SendMenu(c, voteMapPromptMenuID, "[accent]换图投票[]", strings.Join(lines, "\n"), [][]string{
		{"同意", "反对", "中立", "关闭"},
	})
}

func (p *MapVotePlugin) showActiveVoteMenuToAll() {
	if p.server == nil {
		return
	}
	for _, conn := range p.server.ListConnectedConns() {
		p.ShowActiveVoteMenu(conn)
	}
}

// HandleVoteMenuChoice handles menu selections for vote menus. Returns true if handled.
func (p *MapVotePlugin) HandleVoteMenuChoice(c plugin.ConnInterface, menuID, option int32) bool {
	if p.server == nil || c == nil {
		return false
	}
	switch {
	case menuID == voteMapPromptMenuID:
		switch option {
		case 0:
			p.castVoteAndNotify(c, 1)
		case 1:
			p.castVoteAndNotify(c, -1)
		case 2:
			p.castVoteAndNotify(c, 0)
		case 3:
			p.server.BroadcastChat(fmt.Sprintf("[accent]%s[] 关闭了投票窗口。", p.voteParticipantName(c)))
			return true
		}
		return true
	case menuID >= voteMapSelectMenuBaseID && menuID < voteMapSelectMenuBaseID+100:
		linkURL := p.currentConfig().HomeLinkURL
		hasLink := strings.TrimSpace(linkURL) != ""
		pageMaps, page, totalPages, err := p.listPage(int(menuID - voteMapSelectMenuBaseID))
		if err != nil {
			p.server.SendInfoMessage(c, fmt.Sprintf("[scarlet]读取地图列表失败: %s[]", err.Error()))
			return true
		}
		if option < int32(len(pageMaps)) {
			p.startVoteForConn(c, pageMaps[option])
			return true
		}
		option -= int32(len(pageMaps))
		if page > 0 {
			if option == 0 {
				p.ShowVoteMenu(c, page-1)
				return true
			}
			option--
		}
		if page+1 < totalPages {
			if option == 0 {
				p.ShowVoteMenu(c, page+1)
				return true
			}
			option--
		}
		if hasLink {
			if option == 0 {
				p.server.SendOpenURI(c, linkURL)
				return true
			}
			option--
		}
		return true
	default:
		return false
	}
}

func (p *MapVotePlugin) startVoteForConn(c plugin.ConnInterface, target string) {
	if p.server == nil || c == nil {
		return
	}
	if _, ok := p.snapshot(len(p.server.ListConnectedConns())); ok {
		p.ShowActiveVoteMenu(c)
		return
	}
	if p.resolveWorld == nil {
		p.server.SendInfoMessage(c, "[scarlet]当前无法解析目标地图。[]")
		return
	}
	worldPath, err := p.resolveWorld(strings.TrimSpace(target))
	if err != nil {
		p.server.SendChat(c, fmt.Sprintf("[scarlet]地图无效: %s[]", err.Error()))
		return
	}
	mapName := worldstream.TrimMapName(filepath.Base(worldPath))
	snapshot, result, err := p.beginVote(worldPath, mapName, voteParticipantKey(c), p.voteParticipantName(c), len(p.server.ListConnectedConns()))
	if err != nil {
		if errors.Is(err, errMapVoteActive) {
			p.server.SendChat(c, "[scarlet]已有换图投票进行中。[]")
			p.ShowActiveVoteMenu(c)
			return
		}
		p.server.SendChat(c, fmt.Sprintf("[scarlet]发起投票失败: %s[]", err.Error()))
		return
	}
	if result.Decision == mapVoteDecisionPending {
		durationSec := int(p.currentConfig().Duration / time.Second)
		p.server.BroadcastChat(fmt.Sprintf("[accent]%s[] 发起了换图投票: [white]%s[] ([green]%d/%d[]，限时 [white]%ds[]，输入 [white]/vote[] 可投票)", p.voteParticipantName(c), snapshot.MapName, snapshot.Yes, snapshot.Needed, durationSec))
		if token := p.activeToken(); token != 0 {
			p.startStatusLoop(token)
		}
		p.BroadcastActiveStatus()
		p.showActiveVoteMenuToAll()
		return
	}
	p.handleResult(result)
}

func (p *MapVotePlugin) castVoteAndNotify(c plugin.ConnInterface, vote int8) {
	if p.server == nil || c == nil {
		return
	}
	_, result, err := p.castVote(voteParticipantKey(c), vote, len(p.server.ListConnectedConns()))
	if err != nil {
		p.server.SendChat(c, fmt.Sprintf("[scarlet]%s[]", err.Error()))
		return
	}
	choiceLabel := mapVoteChoiceLabel(vote)
	p.server.SendChat(c, fmt.Sprintf("[accent]你选择了%s。[]", choiceLabel))
	p.server.BroadcastChat(fmt.Sprintf("[accent]%s[] 选择了%s。", p.voteParticipantName(c), choiceLabel))
	if result.Decision == mapVoteDecisionPending {
		p.BroadcastActiveStatus()
		return
	}
	p.handleResult(result)
}

func mapVoteChoiceLabel(vote int8) string {
	switch vote {
	case 1:
		return "[green]同意[]"
	case -1:
		return "[scarlet]反对[]"
	case 0:
		return "[lightgray]中立[]"
	default:
		return "[lightgray]未知[]"
	}
}

func (p *MapVotePlugin) handleResult(result MapVoteResult) {
	if result.Decision == mapVoteDecisionPending {
		return
	}
	if p.server != nil {
		p.server.BroadcastHideHudText()
	}
	switch result.Decision {
	case mapVoteDecisionPassed:
		if p.server != nil {
			p.server.BroadcastChat(fmt.Sprintf("[accent]换图投票通过[]: [white]%s[] ([green]%d/%d[] 反对 [scarlet]%d[] 中立 [lightgray]%d[])", result.MapName, result.Yes, result.Needed, result.No, result.Neutral))
		}
		if p.applyWorld == nil {
			if p.server != nil {
				p.server.BroadcastChat("[scarlet]换图失败：切图回调未初始化。[]")
			}
			return
		}
		if err := p.applyWorld(result.WorldPath); err != nil && p.server != nil {
			p.server.BroadcastChat(fmt.Sprintf("[scarlet]换图失败: %s[]", err.Error()))
		}
	case mapVoteDecisionRejected:
		if p.server != nil {
			p.server.BroadcastChat(fmt.Sprintf("[scarlet]换图投票未通过[]: [white]%s[] (同意 [green]%d[] / 反对 [scarlet]%d[] / 中立 [lightgray]%d[] / 需要 %d)", result.MapName, result.Yes, result.No, result.Neutral, result.Needed))
		}
	case mapVoteDecisionExpired:
		if p.server != nil {
			p.server.BroadcastChat(fmt.Sprintf("[scarlet]换图投票超时[]: [white]%s[] (同意 [green]%d[] / 反对 [scarlet]%d[] / 中立 [lightgray]%d[])", result.MapName, result.Yes, result.No, result.Neutral))
		}
	}
}

// ReloadConfig updates the plugin configuration after a hot reload.
func (p *MapVotePlugin) ReloadConfig(cfg *config.Config) {
	p.loadConfig(cfg)
}

// --- Chat command handlers ---

func (p *MapVotePlugin) handleChatVoteMap(c plugin.ConnInterface, args []string) bool {
	if c == nil {
		return true
	}
	if len(args) > 0 {
		p.startVoteForConn(c, strings.Join(args, " "))
	} else {
		p.ShowVoteMenu(c, 0)
	}
	return true
}

func (p *MapVotePlugin) handleChatVote(c plugin.ConnInterface, args []string) bool {
	if c == nil {
		return true
	}
	if len(args) == 0 {
		p.ShowActiveVoteMenu(c)
		return true
	}
	switch strings.ToLower(args[0]) {
	case "yes", "y", "1", "同意":
		p.castVoteAndNotify(c, 1)
	case "no", "n", "0", "反对":
		p.castVoteAndNotify(c, -1)
	case "neutral", "mid", "中立", "abstain":
		p.castVoteAndNotify(c, 0)
	default:
		p.server.SendChat(c, "[scarlet]用法: /vote yes|no|neutral[]")
	}
	return true
}

// --- Backward-compat wrappers for net.Conn ---

// ShowVoteMenuForConn opens the vote menu for a raw *net.Conn.
func (p *MapVotePlugin) ShowVoteMenuForConn(c *netpkg.Conn, page int) {
	p.ShowVoteMenu(plugin.WrapConn(c), page)
}

// HandleVoteMenuChoiceForConn handles a menu choice from a raw *net.Conn.
func (p *MapVotePlugin) HandleVoteMenuChoiceForConn(c *netpkg.Conn, menuID, option int32) bool {
	return p.HandleVoteMenuChoice(plugin.WrapConn(c), menuID, option)
}
