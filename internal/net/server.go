package net

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/IYanHua/mdt-server/internal/devlog"
	"github.com/IYanHua/mdt-server/internal/protocol"
	"github.com/IYanHua/mdt-server/internal/storage"
)

var globalVerboseNetLog atomic.Bool

const invalidTilePos int32 = -1
const maxMindustryPlayerNameBytes = 40

const (
	previewPlanCommitDelay = 100 * time.Millisecond
	maxPlayerPreviewPlans  = 1000
	previewPlanChunkSize   = 900 / 12
)

type NetEvent struct {
	Timestamp time.Time
	Kind      string
	Packet    string
	Detail    string
	ConnID    int32
	UUID      string
	IP        string
	Name      string
}

type BeginPlaceRequest struct {
	X        int32
	Y        int32
	BlockID  int16
	Rotation int8
	Config   any
}

type ConstructFinishRequest struct {
	Pos       int32
	BlockID   int16
	BuilderID int32
	Rotation  int8
	TeamID    byte
	Config    any
}

type DeconstructFinishRequest struct {
	Pos       int32
	BlockID   int16
	BuilderID int32
}

type previewPlanState struct {
	teamID        byte
	current       []*protocol.BuildPlan
	assembling    []*protocol.BuildPlan
	lastRecvGroup int32
	nextSendGroup int32
	lastRecvAt    time.Time
	receiving     bool
}

type entitySnapshotBaseEntry struct {
	entity  protocol.UnitSyncEntity
	encoded []byte
}

type entitySnapshotPlayerBase struct {
	unit                  protocol.UnitSyncEntity
	unitEncoded           []byte
	playerWithUnit        *protocol.PlayerEntity
	playerWithUnitData    []byte
	playerWithoutUnit     *protocol.PlayerEntity
	playerWithoutUnitData []byte
}

type entitySnapshotBase struct {
	builtAt       time.Time
	players       []entitySnapshotPlayerBase
	extraEntries  []entitySnapshotBaseEntry
	legacyPackets []*protocol.Remote_NetClient_entitySnapshot_32
}

type EntitySnapshotCacheStats struct {
	Hits               uint64
	Misses             uint64
	LastBuildDuration  time.Duration
	LastFilterDuration time.Duration
}

type AssemblerDroneSpawnedRequest struct {
	Pos    int32
	UnitID int32
}

type Server struct {
	Addr     string
	Registry *protocol.PacketRegistry
	Serial   *Serializer
	Content  *protocol.ContentRegistry
	TypeIO   *protocol.TypeIOContext

	mu              sync.Mutex
	conns           map[*Conn]struct{}
	pending         map[int32]*Conn
	byUDP           map[string]*Conn
	banUUID         map[string]string
	banIP           map[string]string
	admissionMu     sync.RWMutex
	admission       AdmissionPolicy
	recentKickUntil map[string]time.Time
	udpConn         *net.UDPConn
	tcpLn           *net.TCPListener
	shuttingDown    atomic.Bool
	opMu            sync.RWMutex
	ops             map[string]struct{}
	logMu           sync.Mutex
	logTimes        map[string]time.Time
	entityMu        sync.Mutex
	entities        map[int32]protocol.UnitSyncEntity
	unitNext        int32
	entitySnapMu    sync.RWMutex
	entitySnapCache entitySnapshotBase
	entitySnapHits  atomic.Uint64
	entitySnapMiss  atomic.Uint64
	entitySnapBuild atomic.Int64
	entitySnapView  atomic.Int64

	previewMu    sync.Mutex
	previewPlans map[int32]*previewPlanState

	// EventManager 事件管理器
	EventManager *storage.EventManager

	BuildVersion int
	WorldDataFn  func(*Conn, *protocol.ConnectPacket) ([]byte, error)
	OnEvent      func(NetEvent)
	playerIDNext int32

	Name                string
	Description         string
	VirtualPlayers      int32
	MapNameFn           func() string
	OnChat              func(*Conn, string) bool
	chatHandlers        []func(*Conn, string) bool
	SpawnTileFn         func() (protocol.Point2, bool)
	SpawnTileForConnFn  func(*Conn) (protocol.Point2, bool)
	AssignTeamForConnFn func(*Conn) byte
	// Optional provider for player-respawn unit type id (e.g. alpha).
	PlayerUnitTypeFn func() int16
	// Optional hook: apply client build plans from snapshot.
	OnBuildPlans func(*Conn, []*protocol.BuildPlan)
	// Optional hook: preview plans from clientSnapshot.
	OnBuildPlanPreview func(*Conn, []*protocol.BuildPlan)
	// Optional hooks: official building RPC chains.
	OnBeginBreak func(*Conn, int32, int32)
	OnBeginPlace func(*Conn, BeginPlaceRequest)
	// Optional hook: apply authoritative plans queue from client snapshots.
	OnBuildPlanSnapshot func(*Conn, []*protocol.BuildPlan)
	// Optional hooks: cancel queued build plans from client side.
	OnDeletePlans               func(*Conn, []int32)
	OnRemoveQueueBlock          func(*Conn, int32, int32, bool)
	OnRequestUnitPayload        func(*Conn, int32)
	OnRequestBuildPayload       func(*Conn, int32)
	OnRequestDropPayload        func(*Conn, float32, float32)
	OnUnitEnteredPayload        func(*Conn, int32, int32)
	OnCommandUnits              func(*Conn, []int32, any, any, any, bool, bool)
	OnSetUnitCommand            func(*Conn, []int32, *protocol.UnitCommand)
	OnSetUnitStance             func(*Conn, []int32, protocol.UnitStance, bool)
	OnCommandBuilding           func(*Conn, []int32, protocol.Vec2)
	OnRotateBlock               func(*Conn, int32, bool)
	OnBuildingControlSelect     func(*Conn, int32)
	OnUnitBuildingControlSelect func(*Conn, int32, int32)
	OnRequestItem               func(*Conn, int32, int16, int32)
	OnTransferInventory         func(*Conn, int32)
	OnRequestBlockSnapshot      func(*Conn, int32)
	OnDropItem                  func(*Conn, float32)
	OnTileConfig                func(*Conn, int32, any)
	OnConstructFinish           func(*Conn, ConstructFinishRequest)
	OnDeconstructFinish         func(*Conn, DeconstructFinishRequest)
	OnAssemblerUnitSpawned      func(*Conn, int32)
	OnAssemblerDroneSpawned     func(*Conn, AssemblerDroneSpawnedRequest)
	OnMenuChoose                func(*Conn, int32, int32)
	// Respawn delay in "frames" (Mindustry Time.delta units). Defaults to 60 if zero.
	RespawnDelayFrames float32
	// Optional hook: spawn player unit in world/state. Uses provided unitID, returns world coords.
	SpawnUnitFn func(c *Conn, unitID int32, tile protocol.Point2, unitType int16) (float32, float32, bool)
	// Optional hook: spawn/attach a player-controlled unit directly at a world position.
	// This mirrors vanilla InputHandler.unitClear() dock-style respawn for core builder units.
	SpawnUnitAtFn func(c *Conn, unitID int32, x, y, rotation float32, unitType int16, spawnedByCore bool) (float32, float32, bool)
	// Optional hook: remove a unit from world/state.
	DropUnitFn func(unitID int32)
	// Optional hook: query unit info from world/state.
	UnitInfoFn func(unitID int32) (UnitInfo, bool)
	// Optional hook: build an authoritative sync snapshot for one unit from world state.
	UnitSyncFn func(unitID int32, controller protocol.UnitController) (*protocol.UnitEntitySync, bool)
	// Optional hook: resolve the actual respawn unit type for a specific core/spawn tile.
	ResolveRespawnUnitTypeFn func(c *Conn, tile protocol.Point2, fallback int16) int16
	// Optional hook: reserve a world-authoritative entity ID for player units.
	ReserveUnitIDFn func() int32
	// Optional hooks: apply client snapshot motion/position into authoritative world state.
	// If unset, clientSnapshot will only update connection state but will not move entities in the world.
	SetUnitMotionFn           func(unitID int32, vx, vy, rotVel float32) bool
	SetUnitPositionFn         func(unitID int32, x, y, rotation float32) bool
	SetUnitRuntimeStateFn     func(unitID int32, state UnitRuntimeState) bool
	SetUnitStackFn            func(unitID int32, itemID int16, amount int32) bool
	SetUnitPlayerControllerFn func(unitID int32, playerID int32) bool
	ClaimControlledBuildFn    func(playerID int32, buildPos int32) (ControlledBuildInfo, bool)
	ControlledBuildInfoFn     func(playerID int32, buildPos int32) (ControlledBuildInfo, bool)
	ReleaseControlledBuildFn  func(playerID int32, buildPos int32) bool
	SetControlledBuildInputFn func(playerID int32, buildPos int32, aimX, aimY float32, shooting bool) bool
	// Optional hook: called once after initial connect/spawn sequence starts.
	OnPostConnect     func(*Conn)
	OnHotReloadConnFn func(*Conn)
	// Optional hook: called after connect packet validation/identity assignment
	// and before the first display-name refresh / connect_packet event emission.
	OnConnectAccepted func(*Conn, *protocol.ConnectPacket)
	// Optional network hooks for external core scheduling.
	OnConnOpen      func(*Conn)
	OnConnClose     func(*Conn)
	OnPacketDecoded func(*Conn, any, error) bool
	OnTracePacket   func(direction string, c *Conn, obj any, packetID int, frameworkID int, size int)

	StateSnapshotFn func() *protocol.Remote_NetClient_stateSnapshot_35
	// Appends additional entities into entity snapshot stream.
	// Return value is appended entity count.
	ExtraEntitySnapshotFn func(w *protocol.Writer) (int16, error)
	// Appends additional sync entities into entity snapshots using the same
	// per-entity packet splitting path as vanilla NetServer.writeEntitySnapshot().
	ExtraEntitySnapshotEntitiesFn func() ([]protocol.UnitSyncEntity, error)
	// Optional per-viewer hide hook for entity snapshots. Hidden IDs are sent
	// through hiddenSnapshot and omitted from the viewer's entitySnapshot stream.
	EntitySnapshotHiddenFn func(viewer *Conn, entity protocol.UnitSyncEntity) bool

	UdpRetryCount  int
	UdpRetryDelay  time.Duration
	UdpFallbackTCP bool

	entitySnapshotIntervalNs  atomic.Int64
	stateSnapshotIntervalNs   atomic.Int64
	clientSnapshotConfirm     atomic.Bool
	verifyClientPosition      atomic.Bool
	infoMu                    sync.RWMutex
	verboseNetLog             atomic.Bool
	packetRecvEventsEnabled   atomic.Bool
	packetSendEventsEnabled   atomic.Bool
	terminalPlayerLogsEnabled atomic.Bool
	terminalPlayerUUIDEnabled atomic.Bool
	respawnPacketLogsEnabled  atomic.Bool
	playerNameColorEnabled    atomic.Bool
	translatedConnLog         atomic.Bool
	joinLeaveChatEnabled      atomic.Bool
	publicConnIDFormatter     func(*Conn) string
	playerDisplayFormatter    func(*Conn) string

	// DevLogger 开发者日志（可选）
	DevLogger *devlog.DevLogger

	// CommandHandler 命令处理器
	CommandHandler *CommandHandler

	// VoteKickManager 投票踢人管理器
	VoteKickManager *VoteKickManager

	// AdminManager 管理员管理器
	AdminManager *AdminManager
}

func NewServer(addr string, _ int) *Server {
	reg := protocol.NewRegistry()
	content := protocol.NewContentRegistry()
	ctx := content.Context()
	em := storage.NewEventManager()
	s := &Server{
		Addr:               addr,
		Registry:           reg,
		Serial:             &Serializer{Registry: reg, Ctx: ctx},
		Content:            content,
		TypeIO:             ctx,
		conns:              map[*Conn]struct{}{},
		pending:            map[int32]*Conn{},
		byUDP:              map[string]*Conn{},
		banUUID:            map[string]string{},
		banIP:              map[string]string{},
		admission:          DefaultAdmissionPolicy(),
		recentKickUntil:    map[string]time.Time{},
		EventManager:       em,
		BuildVersion:       157,
		WorldDataFn:        defaultWorldData,
		playerIDNext:       0,
		Name:               "mdt-server",
		Description:        "",
		VirtualPlayers:     0,
		entities:           map[int32]protocol.UnitSyncEntity{},
		previewPlans:       map[int32]*previewPlanState{},
		unitNext:           2000000000,
		ops:                map[string]struct{}{},
		logTimes:           map[string]time.Time{},
		UdpRetryCount:      2,
		UdpRetryDelay:      5 * time.Millisecond,
		UdpFallbackTCP:     true,
		RespawnDelayFrames: 60,
	}
	s.SetSnapshotIntervals(200, 200)
	s.verboseNetLog.Store(false)
	s.packetRecvEventsEnabled.Store(false)
	s.packetSendEventsEnabled.Store(false)
	s.terminalPlayerLogsEnabled.Store(true)
	s.terminalPlayerUUIDEnabled.Store(false)
	s.respawnPacketLogsEnabled.Store(true)
	s.playerNameColorEnabled.Store(true)
	s.translatedConnLog.Store(true)
	s.joinLeaveChatEnabled.Store(true)

	// 初始化命令处理器
	s.CommandHandler = NewCommandHandler()
	s.CommandHandler.RegisterDefaultCommands(s)

	// 初始化投票踢人管理器
	s.VoteKickManager = NewVoteKickManager(s)

	// 初始化管理员管理器
	s.AdminManager = NewAdminManager()
	globalServer = s

	return s
}

func (s *Server) SetVerboseNetLog(enabled bool) {
	s.verboseNetLog.Store(enabled)
	globalVerboseNetLog.Store(enabled)
}

func (s *Server) VerboseNetLogEnabled() bool {
	return s.verboseNetLog.Load()
}

func (s *Server) SetPacketRecvEventsEnabled(enabled bool) {
	s.packetRecvEventsEnabled.Store(enabled)
}

func (s *Server) PacketRecvEventsEnabled() bool {
	return s.packetRecvEventsEnabled.Load()
}

func (s *Server) SetPacketSendEventsEnabled(enabled bool) {
	s.packetSendEventsEnabled.Store(enabled)
}

func (s *Server) PacketSendEventsEnabled() bool {
	return s.packetSendEventsEnabled.Load()
}

func (s *Server) SetTerminalPlayerLogsEnabled(enabled bool) {
	s.terminalPlayerLogsEnabled.Store(enabled)
}

func (s *Server) TerminalPlayerLogsEnabled() bool {
	return s.terminalPlayerLogsEnabled.Load()
}

func (s *Server) SetTerminalPlayerUUIDEnabled(enabled bool) {
	s.terminalPlayerUUIDEnabled.Store(enabled)
}

func (s *Server) TerminalPlayerUUIDEnabled() bool {
	return s.terminalPlayerUUIDEnabled.Load()
}

func (s *Server) SetRespawnPacketLogsEnabled(enabled bool) {
	s.respawnPacketLogsEnabled.Store(enabled)
}

func (s *Server) RespawnPacketLogsEnabled() bool {
	return s.respawnPacketLogsEnabled.Load()
}

func (s *Server) SetPlayerNameColorEnabled(enabled bool) {
	s.playerNameColorEnabled.Store(enabled)
}

func (s *Server) PlayerNameColorEnabled() bool {
	return s.playerNameColorEnabled.Load()
}

func (s *Server) SetTranslatedConnLog(enabled bool) {
	s.translatedConnLog.Store(enabled)
}

func (s *Server) TranslatedConnLogEnabled() bool {
	return s.translatedConnLog.Load()
}

func (s *Server) SetPublicConnIDFormatter(fn func(*Conn) string) {
	s.publicConnIDFormatter = fn
}

func (s *Server) SetJoinLeaveChatEnabled(enabled bool) {
	s.joinLeaveChatEnabled.Store(enabled)
}

func (s *Server) SetPlayerDisplayFormatter(fn func(*Conn) string) {
	s.playerDisplayFormatter = fn
}

// AddChatHandler 注册一个额外的聊天处理器。多个处理器按注册顺序调用，
// 直到某个返回 true 表示已处理（此时不再调用后续处理器）。
func (s *Server) AddChatHandler(fn func(*Conn, string) bool) {
	s.chatHandlers = append(s.chatHandlers, fn)
}

func (s *Server) BroadcastInfoPopup(message string, duration float32, align, top, left, bottom, right int32) {
	if s == nil {
		return
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	peers := make([]*Conn, 0, len(s.conns))
	s.mu.Lock()
	for c := range s.conns {
		if c != nil && c.hasConnected {
			peers = append(peers, c)
		}
	}
	s.mu.Unlock()
	msg := message
	packet := &protocol.Remote_Menus_infoPopup_118{
		Message:  msg,
		Duration: duration,
		Align:    align,
		Top:      top,
		Left:     left,
		Bottom:   bottom,
		Right:    right,
	}
	for _, peer := range peers {
		_ = peer.SendAsync(packet)
	}
}

func (s *Server) SendInfoPopup(c *Conn, message string, duration float32, align, top, left, bottom, right int32) {
	if s == nil || c == nil || !c.hasConnected {
		return
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	msg := message
	packet := &protocol.Remote_Menus_infoPopup_118{
		Message:  msg,
		Duration: duration,
		Align:    align,
		Top:      top,
		Left:     left,
		Bottom:   bottom,
		Right:    right,
	}
	_ = c.SendAsync(packet)
}

func (s *Server) SendInfoMessage(c *Conn, message string) {
	if s == nil || c == nil || !c.hasConnected {
		return
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	_ = c.SendAsync(&protocol.Remote_Menus_infoMessage_117{Message: message})
}

func (s *Server) SendMenu(c *Conn, menuID int32, title, message string, options [][]string) {
	if s == nil || c == nil || !c.hasConnected {
		return
	}
	cleaned := make([][]string, 0, len(options))
	for _, row := range options {
		rowOut := make([]string, 0, len(row))
		for _, option := range row {
			rowOut = append(rowOut, strings.TrimSpace(option))
		}
		cleaned = append(cleaned, rowOut)
	}
	_ = c.SendAsync(&protocol.Remote_Menus_menu_106{
		MenuId:  menuID,
		Title:   strings.TrimSpace(title),
		Message: strings.TrimSpace(message),
		Options: cleaned,
	})
}

func (s *Server) SendOpenURI(c *Conn, uri string) {
	if s == nil || c == nil || !c.hasConnected {
		return
	}
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return
	}
	_ = c.SendAsync(&protocol.Remote_Menus_openURI_128{Uri: uri})
}

func (s *Server) BroadcastSetHudTextReliable(message string) {
	if s == nil {
		return
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	peers := make([]*Conn, 0, len(s.conns))
	s.mu.Lock()
	for c := range s.conns {
		if c != nil && c.hasConnected {
			peers = append(peers, c)
		}
	}
	s.mu.Unlock()
	packet := &protocol.Remote_Menus_setHudTextReliable_115{Message: message}
	for _, peer := range peers {
		_ = peer.SendAsync(packet)
	}
}

func (s *Server) SendSetHudTextReliable(c *Conn, message string) {
	if s == nil || c == nil || !c.hasConnected {
		return
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	_ = c.SendAsync(&protocol.Remote_Menus_setHudTextReliable_115{Message: message})
}

func (s *Server) BroadcastHideHudText() {
	if s == nil {
		return
	}
	peers := make([]*Conn, 0, len(s.conns))
	s.mu.Lock()
	for c := range s.conns {
		if c != nil && c.hasConnected {
			peers = append(peers, c)
		}
	}
	s.mu.Unlock()
	packet := &protocol.Remote_Menus_hideHudText_114{}
	for _, peer := range peers {
		_ = peer.SendAsync(packet)
	}
}

func (s *Server) SendHideHudText(c *Conn) {
	if s == nil || c == nil || !c.hasConnected {
		return
	}
	_ = c.SendAsync(&protocol.Remote_Menus_hideHudText_114{})
}

func (s *Server) playerDisplayName(c *Conn) string {
	if c == nil {
		return "未知玩家"
	}
	if s != nil && s.playerDisplayFormatter != nil {
		if name := strings.TrimSpace(s.playerDisplayFormatter(c)); name != "" {
			return name
		}
	}
	name := strings.TrimSpace(c.rawName)
	if name == "" {
		name = strings.TrimSpace(c.name)
	}
	if name == "" {
		return "未知玩家"
	}
	return name
}

func (s *Server) PlayerDisplayName(c *Conn) string {
	return s.playerDisplayName(c)
}

func (s *Server) refreshPlayerDisplayName(c *Conn) {
	if s == nil || c == nil {
		return
	}
	c.name = s.playerDisplayName(c)
}

func (s *Server) RefreshPlayerDisplayNames() {
	if s == nil {
		return
	}
	s.mu.Lock()
	peers := make([]*Conn, 0, len(s.conns))
	for c := range s.conns {
		peers = append(peers, c)
	}
	s.mu.Unlock()
	for _, c := range peers {
		s.refreshPlayerDisplayName(c)
	}
}

func (s *Server) ListConnectedConns() []*Conn {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Conn, 0, len(s.conns))
	for c := range s.conns {
		if c != nil && c.hasConnected {
			out = append(out, c)
		}
	}
	return out
}

func cloneBuildPlans(plans []*protocol.BuildPlan) []*protocol.BuildPlan {
	if len(plans) == 0 {
		return nil
	}
	out := make([]*protocol.BuildPlan, 0, len(plans))
	for _, plan := range plans {
		if plan == nil {
			out = append(out, nil)
			continue
		}
		copyPlan := *plan
		if copyPlan.Block != nil {
			copyPlan.Block = protocol.BlockRef{BlkID: copyPlan.Block.ID(), BlkName: copyPlan.Block.Name()}
		}
		if clonedConfig, err := protocol.CloneObjectValue(copyPlan.Config); err == nil {
			copyPlan.Config = clonedConfig
		} else {
			copyPlan.Config = nil
		}
		out = append(out, &copyPlan)
	}
	return out
}

func (s *Server) recordClientPlanPreview(sender *Conn, groupID int32, plans []*protocol.BuildPlan) {
	if s == nil || sender == nil || sender.playerID == 0 {
		return
	}
	cloned := cloneBuildPlans(plans)

	s.previewMu.Lock()
	defer s.previewMu.Unlock()

	state := s.previewPlans[sender.playerID]
	if state == nil {
		state = &previewPlanState{}
		s.previewPlans[sender.playerID] = state
	}
	state.teamID = sender.TeamID()

	switch {
	case groupID > state.lastRecvGroup:
		state.assembling = state.assembling[:0]
		state.lastRecvGroup = groupID
		state.receiving = true
		state.lastRecvAt = time.Now()
	case groupID < state.lastRecvGroup:
		return
	case !state.receiving:
		return
	default:
		state.lastRecvAt = time.Now()
	}

	if len(cloned) == 0 {
		return
	}
	remaining := maxPlayerPreviewPlans - len(state.assembling)
	if remaining <= 0 {
		return
	}
	if len(cloned) > remaining {
		cloned = cloned[:remaining]
	}
	state.assembling = append(state.assembling, cloned...)
}

func (s *Server) commitClientPlanPreviewsLocked(now time.Time) {
	for _, state := range s.previewPlans {
		if state == nil || !state.receiving {
			continue
		}
		if now.Sub(state.lastRecvAt) < previewPlanCommitDelay {
			continue
		}
		state.receiving = false
		state.current = cloneBuildPlans(state.assembling)
		state.assembling = state.assembling[:0]
	}
}

func (s *Server) BroadcastStoredClientPlanPreviews() {
	s.BroadcastStoredClientPlanPreviewsAt(time.Now())
}

func (s *Server) BroadcastStoredClientPlanPreviewsAt(now time.Time) {
	if s == nil {
		return
	}

	type previewBatch struct {
		playerID int32
		teamID   byte
		groupID  int32
		plans    []*protocol.BuildPlan
	}

	s.previewMu.Lock()
	s.commitClientPlanPreviewsLocked(now)
	batches := make([]previewBatch, 0, len(s.previewPlans))
	for playerID, state := range s.previewPlans {
		if state == nil {
			continue
		}
		state.nextSendGroup++
		batches = append(batches, previewBatch{
			playerID: playerID,
			teamID:   state.teamID,
			groupID:  state.nextSendGroup,
			plans:    cloneBuildPlans(state.current),
		})
	}
	s.previewMu.Unlock()

	for _, batch := range batches {
		s.broadcastClientPlanPreviewPackets(batch.playerID, batch.teamID, batch.groupID, batch.plans)
	}
}

func (s *Server) broadcastClientPlanPreviewPackets(senderPlayerID int32, teamID byte, groupID int32, plans []*protocol.BuildPlan) {
	if s == nil || senderPlayerID == 0 {
		return
	}
	peers := s.ListConnectedConns()
	sendPacket := func(chunk []*protocol.BuildPlan) {
		for _, peer := range peers {
			if peer == nil || peer.playerID == 0 || peer.playerID == senderPlayerID {
				continue
			}
			if teamID != 0 && peer.TeamID() != teamID {
				continue
			}
			packet := &protocol.Remote_NetServer_clientPlanSnapshotReceived_47{
				Player:  &protocol.EntityBox{IDValue: senderPlayerID},
				GroupId: groupID,
				Plans:   cloneBuildPlans(chunk),
			}
			_ = peer.SendAsync(packet)
		}
	}

	if len(plans) == 0 {
		sendPacket(nil)
		return
	}
	for i := 0; i < len(plans); i += previewPlanChunkSize {
		end := i + previewPlanChunkSize
		if end > len(plans) {
			end = len(plans)
		}
		sendPacket(plans[i:end])
	}
}

func normalizePingLocationText(text any) any {
	switch v := text.(type) {
	case nil:
		return nil
	case string:
		return v
	default:
		return nil
	}
}

func (s *Server) broadcastPingLocation(sender *Conn, x, y float32, text any) {
	if s == nil || sender == nil || sender.playerID == 0 {
		return
	}
	peers := s.ListConnectedConns()
	for _, peer := range peers {
		if peer == nil || peer.playerID == 0 {
			continue
		}
		packet := &protocol.Remote_InputHandler_pingLocation_73{
			Player: &protocol.EntityBox{IDValue: sender.playerID},
			X:      x,
			Y:      y,
			Text:   normalizePingLocationText(text),
		}
		_ = peer.SendAsync(packet)
	}
}

func (s *Server) publicConnField(c *Conn) string {
	if s != nil && s.publicConnIDFormatter != nil {
		if id := strings.TrimSpace(s.publicConnIDFormatter(c)); id != "" {
			return fmt.Sprintf("conn_uuid=%s", id)
		}
	}
	if c == nil {
		return "connID=0"
	}
	return fmt.Sprintf("connID=%d", c.id)
}

func (s *Server) verbosef(format string, args ...any) {
	if s.verboseNetLog.Load() {
		fmt.Printf(format, args...)
	}
}

func (s *Server) shouldLogRepeatingNetEvent(key string, every time.Duration) bool {
	if s == nil || every <= 0 {
		return true
	}
	now := time.Now()
	s.logMu.Lock()
	defer s.logMu.Unlock()
	if last, ok := s.logTimes[key]; ok && now.Sub(last) < every {
		return false
	}
	s.logTimes[key] = now
	return true
}

func (s *Server) shouldLogClientDeadIgnored(c *Conn, now time.Time) bool {
	if s == nil || c == nil {
		return false
	}
	if !s.respawnPacketLogsEnabled.Load() {
		return false
	}
	if c.lastDeadIgnoreLogAt.IsZero() || now.Sub(c.lastDeadIgnoreLogAt) >= 2*time.Second {
		c.lastDeadIgnoreLogAt = now
		return true
	}
	return false
}

func normalizeSyncInterval(ms int, def int) time.Duration {
	if ms <= 0 {
		ms = def
	}
	if ms < 20 {
		ms = 20
	}
	if ms > 5000 {
		ms = 5000
	}
	return time.Duration(ms) * time.Millisecond
}

func (s *Server) SetSnapshotIntervals(entityMs, stateMs int) {
	entity := normalizeSyncInterval(entityMs, 100)
	state := normalizeSyncInterval(stateMs, 250)
	s.entitySnapshotIntervalNs.Store(int64(entity))
	s.stateSnapshotIntervalNs.Store(int64(state))
}

func (s *Server) SetClientSnapshotConnectFallbackEnabled(enabled bool) {
	s.clientSnapshotConfirm.Store(enabled)
}

func (s *Server) ClientSnapshotConnectFallbackEnabled() bool {
	if s == nil {
		return false
	}
	return s.clientSnapshotConfirm.Load()
}

func (s *Server) SetClientPositionVerificationEnabled(enabled bool) {
	if s == nil {
		return
	}
	s.verifyClientPosition.Store(enabled)
}

func (s *Server) ClientPositionVerificationEnabled() bool {
	if s == nil {
		return false
	}
	return s.verifyClientPosition.Load()
}

func (s *Server) SnapshotIntervalsMs() (entityMs int, stateMs int) {
	entity := time.Duration(s.entitySnapshotIntervalNs.Load())
	state := time.Duration(s.stateSnapshotIntervalNs.Load())
	if entity <= 0 {
		entity = 100 * time.Millisecond
	}
	if state <= 0 {
		state = 250 * time.Millisecond
	}
	return int(entity / time.Millisecond), int(state / time.Millisecond)
}

func (s *Server) syncInterval() time.Duration {
	if s == nil {
		return 200 * time.Millisecond
	}
	entity := time.Duration(s.entitySnapshotIntervalNs.Load())
	state := time.Duration(s.stateSnapshotIntervalNs.Load())
	if entity <= 0 {
		entity = 200 * time.Millisecond
	}
	if state <= 0 {
		state = 200 * time.Millisecond
	}
	if state < entity {
		return state
	}
	return entity
}

func snapshotPollInterval(syncInterval time.Duration) time.Duration {
	if syncInterval <= 20*time.Millisecond {
		return 20 * time.Millisecond
	}
	if syncInterval < 100*time.Millisecond {
		return syncInterval
	}
	return 20 * time.Millisecond
}

// KillSelfUnit clears current controlled unit for a player connection.
func (s *Server) KillSelfUnit(c *Conn) bool {
	if c == nil || c.playerID == 0 {
		return false
	}
	s.markDead(c, "kill-self")
	player := &protocol.EntityBox{IDValue: c.playerID}
	_ = c.SendAsync(&protocol.Remote_InputHandler_unitClear_95{Player: player})
	return true
}

func (s *Server) Serve() error {
	addr, err := net.ResolveTCPAddr("tcp", s.Addr)
	if err != nil {
		return err
	}
	ln, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return err
	}
	s.tcpLn = ln
	defer func() {
		_ = ln.Close()
		s.tcpLn = nil
	}()

	udpAddr := &net.UDPAddr{IP: addr.IP, Port: addr.Port}
	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}
	defer udpConn.Close()
	s.udpConn = udpConn
	defer func() {
		s.udpConn = nil
	}()
	go s.serveUDP(udpConn)

	for {
		c, err := ln.AcceptTCP()
		if err != nil {
			if s.shuttingDown.Load() && isListenerClosed(err) {
				return nil
			}
			return err
		}
		conn := NewConn(c, s.Serial)
		conn.id = s.nextID()
		conn.onSend = func(obj any, packetID int, frameworkID int, size int) {
			if s.OnTracePacket != nil {
				s.OnTracePacket("send", conn, obj, packetID, frameworkID, size)
			}
			if s.shouldSuppressHotNetEvent("packet_send") {
				return
			}
			s.emitEvent(conn, "packet_send", packetTypeName(obj), packetSendDetail(size, packetID, frameworkID))
		}
		s.addConn(conn)
		s.addPending(conn)
		if s.OnConnOpen != nil {
			s.OnConnOpen(conn)
		}

		// 记录详细连接日志
		if s.DevLogger != nil {
			s.DevLogger.LogConnection("tcp accepted", conn.id, c.RemoteAddr().String(), "unknown", "")
		} else {
			s.verbosef("[net] tcp accepted remote=%s id=%d\n", c.RemoteAddr().String(), conn.id)
		}

		_ = conn.Send(&protocol.RegisterTCP{ConnectionID: conn.id})
		s.emitEvent(conn, "tcp_accept", "", "")
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(c *Conn) {
	defer func() {
		if rec := recover(); rec != nil {
			errText := fmt.Sprintf("panic: %v", rec)
			fmt.Printf("[net] conn panic id=%d remote=%s err=%s\n", c.id, c.RemoteAddr().String(), errText)
			s.emitEvent(c, "conn_panic", "", errText)
		}
		s.clearConnControlledBuild(c)
		if c.hasBegunConnecting && !c.hasConnected {
			fmt.Printf("[net] connect aborted before confirm id=%d player=%d remote=%s name=%q uuid=%s live_worldstream=%v last_packet_id=%d last_framework_id=%d\n",
				c.id, c.playerID, c.RemoteAddr().String(), c.name, c.uuid, c.UsesLiveWorldStream(), c.lastRecvPacketID, c.lastRecvFrameworkID)
			s.emitEvent(c, "connect_aborted_pre_confirm", "", fmt.Sprintf("live_worldstream=%v last_packet_id=%d last_framework_id=%d", c.UsesLiveWorldStream(), c.lastRecvPacketID, c.lastRecvFrameworkID))
		}
		if c.hasConnected && c.playerID != 0 {
			s.broadcastPlayerDisconnect(c.playerID, c)
		}
		if c.hasConnected {
			s.broadcastJoinLeaveChat(c, false)
		}
		s.logPlayerLeaveCN(c)
		if s.OnConnClose != nil {
			s.OnConnClose(c)
		}
		c.Close()
		s.removeConn(c)
	}()

	for {
		obj, err := c.ReadObject()
		if s.OnPacketDecoded != nil && s.OnPacketDecoded(c, obj, err) {
			if err != nil {
				return
			}
			continue
		}
		if err != nil {
			if errors.Is(err, io.EOF) || isConnReadClosed(err) {
				s.verbosef("[net] tcp closed id=%d remote=%s\n", c.id, c.RemoteAddr().String())
				s.emitEvent(c, "tcp_closed", "", err.Error())
				return
			}
			// Skip malformed packet frame and keep the connection alive.
			fmt.Printf("[net] read object failed id=%d remote=%s packet_id=%d framework_id=%d err=%v\n", c.id, c.RemoteAddr().String(), c.lastRecvPacketID, c.lastRecvFrameworkID, err)
			s.emitEvent(c, "read_error", "", fmt.Sprintf("packet_id=%d framework_id=%d err=%v", c.lastRecvPacketID, c.lastRecvFrameworkID, err))
			continue
		}
		s.handlePacket(c, obj, true)
	}
}

func (s *Server) logPlayerJoinCN(c *Conn) {
	if c == nil || !s.translatedConnLog.Load() || !s.terminalPlayerLogsEnabled.Load() {
		return
	}
	ip, port := c.remoteEndpoint()
	name := s.playerDisplayName(c)
	if s.playerNameColorEnabled.Load() {
		name = RenderMindustryTextForTerminal(name)
	} else {
		name = StripMindustryColorTags(name)
	}
	if s.terminalPlayerUUIDEnabled.Load() {
		fmt.Printf("[终端] 玩家进入了游戏: 名称=%s UUID=%s %s 登录IP=%s 远程端口=%s\n",
			name, c.uuid, s.publicConnField(c), ip, port)
		return
	}
	fmt.Printf("[终端] 玩家进入了游戏: 名称=%s %s 登录IP=%s 远程端口=%s\n",
		name, s.publicConnField(c), ip, port)
}

func (s *Server) logPlayerLeaveCN(c *Conn) {
	if c == nil || !s.translatedConnLog.Load() || !s.terminalPlayerLogsEnabled.Load() || !c.hasBegunConnecting {
		return
	}
	ip, port := c.remoteEndpoint()
	name := s.playerDisplayName(c)
	if s.playerNameColorEnabled.Load() {
		name = RenderMindustryTextForTerminal(name)
	} else {
		name = StripMindustryColorTags(name)
	}
	if s.terminalPlayerUUIDEnabled.Load() {
		fmt.Printf("[终端] 玩家退出了游戏: 名称=%s UUID=%s %s 登录IP=%s 远程端口=%s\n",
			name, c.uuid, s.publicConnField(c), ip, port)
		return
	}
	fmt.Printf("[终端] 玩家退出了游戏: 名称=%s %s 登录IP=%s 远程端口=%s\n",
		name, s.publicConnField(c), ip, port)
}

var mindustryColorTagRE = regexp.MustCompile(`\[[^\]]*\]`)

func StripMindustryColorTags(name string) string {
	name = mindustryColorTagRE.ReplaceAllString(name, "")
	name = strings.TrimSpace(name)
	return name
}

func FixMindustryPlayerName(name string) string {
	name = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(name, "\n", ""), "\t", ""))
	if name == "[" || name == "]" {
		return ""
	}

	for i := 0; i < len(name); i++ {
		if name[i] != '[' {
			continue
		}
		if i == len(name)-1 || name[i+1] == '[' || (i > 0 && name[i-1] == '[') {
			continue
		}
		prev := name[:i]
		next := name[i:]
		name = prev + stripTransparentMindustryColor(next)
	}

	var b strings.Builder
	for _, r := range name {
		width := utf8.RuneLen(r)
		if width < 0 {
			width = len(string(r))
		}
		if b.Len()+width > maxMindustryPlayerNameBytes {
			break
		}
		b.WriteRune(r)
	}
	return b.String()
}

func stripTransparentMindustryColor(s string) string {
	if !strings.HasPrefix(s, "[") {
		return s
	}
	for i := 1; i < len(s); i++ {
		if s[i] != ']' {
			continue
		}
		tag := s[1:i]
		if isTransparentMindustryColor(tag) {
			return s[i+1:]
		}
		return s
	}
	return s
}

func isTransparentMindustryColor(tag string) bool {
	if strings.EqualFold(strings.TrimSpace(tag), "clear") {
		return true
	}
	tag = strings.TrimSpace(tag)
	if len(tag) == 0 {
		return false
	}
	if strings.HasPrefix(tag, "#") {
		tag = tag[1:]
	}
	if len(tag) != 8 {
		return false
	}
	raw, err := hex.DecodeString(tag)
	if err != nil || len(raw) != 4 {
		return false
	}
	return raw[3] < 0xFF
}

func RenderMindustryTextForTerminal(text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	const ansiReset = "\x1b[0m"
	var b strings.Builder
	colorOpen := false
	for i := 0; i < len(text); {
		if text[i] == '[' {
			if end := strings.IndexByte(text[i:], ']'); end >= 0 {
				tag := text[i+1 : i+end]
				switch {
				case tag == "":
					if colorOpen {
						b.WriteString(ansiReset)
						colorOpen = false
					}
				case strings.HasPrefix(tag, "#"):
					if r, g, bl, ok := parseMindustryHexColor(tag[1:]); ok {
						fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%dm", r, g, bl)
						colorOpen = true
					}
				}
				i += end + 1
				continue
			}
		}
		b.WriteByte(text[i])
		i++
	}
	if colorOpen {
		b.WriteString(ansiReset)
	}
	return b.String()
}

func parseMindustryHexColor(s string) (int, int, int, bool) {
	if len(s) != 6 && len(s) != 8 {
		return 0, 0, 0, false
	}
	if len(s) == 8 {
		s = s[:6]
	}
	raw, err := hex.DecodeString(s)
	if err != nil || len(raw) != 3 {
		return 0, 0, 0, false
	}
	return int(raw[0]), int(raw[1]), int(raw[2]), true
}

func displayPlainPlayerName(name string) string {
	return StripMindustryColorTags(name)
}

func isConnReadClosed(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "wsarecv") &&
		(strings.Contains(msg, "aborted") || strings.Contains(msg, "forcibly closed")) {
		return true
	}
	if strings.Contains(msg, "connection reset by peer") {
		return true
	}
	return false
}

func isConnWriteClosed(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "use of closed network connection") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset by peer") ||
		(strings.Contains(msg, "wsasend") && (strings.Contains(msg, "aborted") || strings.Contains(msg, "forcibly closed")))
}

func clientSnapshotIDAfter(current, last int32) bool {
	diff := uint32(current) - uint32(last)
	return diff != 0 && diff < 1<<31
}

func shouldAcceptClientSnapshotID(current, last int32) bool {
	if current == last {
		return true
	}
	return clientSnapshotIDAfter(current, last)
}

func isListenerClosed(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "use of closed network connection") ||
		strings.Contains(msg, "operation on non-socket")
}

func beginBreakPlan(x, y int32) *protocol.BuildPlan {
	return &protocol.BuildPlan{
		Breaking: true,
		X:        x,
		Y:        y,
	}
}

func beginPlacePlan(req BeginPlaceRequest) *protocol.BuildPlan {
	return &protocol.BuildPlan{
		Breaking: false,
		X:        req.X,
		Y:        req.Y,
		Rotation: byte(req.Rotation) & 0x03,
		Block:    protocol.BlockRef{BlkID: req.BlockID, BlkName: ""},
		Config:   req.Config,
	}
}

func beginPlaceRequestFromPlan(plan *protocol.BuildPlan) (BeginPlaceRequest, bool) {
	if plan == nil || plan.Breaking || plan.Block == nil {
		return BeginPlaceRequest{}, false
	}
	return BeginPlaceRequest{
		X:        plan.X,
		Y:        plan.Y,
		BlockID:  plan.Block.ID(),
		Rotation: int8(plan.Rotation & 0x03),
		Config:   plan.Config,
	}, true
}

func (s *Server) handleOfficialBeginBreak(c *Conn, x, y int32) {
	if s.OnBeginBreak != nil {
		s.OnBeginBreak(c, x, y)
		return
	}
	if s.OnBuildPlans != nil {
		s.OnBuildPlans(c, []*protocol.BuildPlan{beginBreakPlan(x, y)})
	}
}

func (s *Server) handleOfficialBeginPlace(c *Conn, req BeginPlaceRequest) {
	if req.BlockID <= 0 {
		return
	}
	if s.OnBeginPlace != nil {
		s.OnBeginPlace(c, req)
		return
	}
	if s.OnBuildPlans != nil {
		s.OnBuildPlans(c, []*protocol.BuildPlan{beginPlacePlan(req)})
	}
}

func (s *Server) handlePacket(c *Conn, obj any, fromTCP bool) {
	if s != nil && s.OnTracePacket != nil {
		packetID := -1
		frameworkID := -1
		if c != nil {
			packetID = c.lastRecvPacketID
			frameworkID = c.lastRecvFrameworkID
		}
		s.OnTracePacket("recv", c, obj, packetID, frameworkID, 0)
	}
	detail := ""
	if fromTCP {
		if c.lastRecvPacketID >= 0 {
			detail = fmt.Sprintf("packet_id=%d", c.lastRecvPacketID)
		} else if c.lastRecvFrameworkID >= 0 {
			detail = fmt.Sprintf("framework_id=%d", c.lastRecvFrameworkID)
		}
	}
	s.emitEvent(c, "packet_recv", fmt.Sprintf("%T", obj), detail)

	switch v := obj.(type) {
	case *protocol.ConnectPacket:
		s.handleConnectPacket(c, v)
	case *protocol.Remote_NetServer_connectConfirm_50:
		s.handleOfficialConnectConfirm(c, v)
	case *protocol.Remote_NetServer_clientSnapshot_48:
		if s.ClientSnapshotConnectFallbackEnabled() {
			s.handleClientSnapshotConnectFallback(c, v)
		}
		if !c.hasConnected {
			s.emitEvent(c, "client_snapshot_ignored_pre_confirm", fmt.Sprintf("%T", v), "waiting_for_connect_confirm")
			return
		}
		// Drop out-of-order snapshots. Snapshot IDs are signed int32 values on
		// the wire and can wrap; compare in uint32 modulo sequence space.
		if last := c.lastClientSnapshot.Load(); c.lastClientSnapshotSet.Load() && !shouldAcceptClientSnapshotID(v.SnapshotID, last) {
			return
		}
		c.lastClientSnapshot.Store(v.SnapshotID)
		c.lastClientSnapshotSet.Store(true)

		nowMs := time.Now().UnixMilli()
		elapsed := int64(16)
		if prev := c.lastClientTimeMs.Load(); prev > 0 {
			elapsed = nowMs - prev
			if elapsed < 0 {
				elapsed = 0
			}
			if elapsed > 1500 {
				elapsed = 1500
			}
		}
		c.lastClientTimeMs.Store(nowMs)

		// Sanitize floats (avoid NaN/Inf poisoning).
		safeF := func(f float32) float32 {
			if math.IsNaN(float64(f)) || math.IsInf(float64(f), 0) {
				return 0
			}
			return f
		}
		v.X = safeF(v.X)
		v.Y = safeF(v.Y)
		v.PointerX = safeF(v.PointerX)
		v.PointerY = safeF(v.PointerY)
		v.Rotation = safeF(v.Rotation)
		v.BaseRotation = safeF(v.BaseRotation)
		v.XVelocity = safeF(v.XVelocity)
		v.YVelocity = safeF(v.YVelocity)
		v.ViewX = safeF(v.ViewX)
		v.ViewY = safeF(v.ViewY)
		v.ViewWidth = safeF(v.ViewWidth)
		v.ViewHeight = safeF(v.ViewHeight)

		c.viewX = v.ViewX
		c.viewY = v.ViewY
		c.viewWidth = v.ViewWidth
		c.viewHeight = v.ViewHeight

		c.pointerX = v.PointerX
		c.pointerY = v.PointerY
		c.shooting = v.Shooting
		c.boosting = v.Boosting
		c.typing = v.Chatting
		if !v.Dead {
			c.clientDeadIgnores = 0
			c.lastDeadIgnoreAt = time.Time{}
		}
		prevUnitID := c.unitID
		if v.Dead && !c.InWorldReloadGrace() {
			now := time.Now()
			freshSpawnGrace := s.connHasFreshSpawnBinding(c, now, 2*time.Second)
			recentRespawnGrace := s.connHasRecentRespawnWindow(c, now, 2*time.Second)
			liveWorldLoadAge, hasLiveWorldLoadAge := s.connLiveWorldLoadSettleAge(c, now)
			liveWorldLoadGrace := hasLiveWorldLoadAge && liveWorldLoadAge < 8*time.Second
			liveWorldLoadAge = liveWorldLoadAge.Round(10 * time.Millisecond)
			skipDeadRepair := false
			// Only honor client-dead when server agrees the unit is missing or dead.
			shouldDead := c.unitID == 0 && !c.controlBuildActive
			debugInfo := "unit=0"
			if c.controlBuildActive {
				if info, ok := s.currentControlledBuildInfo(c); ok {
					debugInfo = fmt.Sprintf("build=%d pos=(%.1f,%.1f) team=%d", info.Pos, info.X, info.Y, info.TeamID)
				} else {
					debugInfo = "build=missing"
					shouldDead = true
				}
			} else if shouldDead && recentRespawnGrace {
				debugInfo = fmt.Sprintf("unit=0 respawn_age=%s", now.Sub(recentNonZeroTime(c.lastSpawnAt, c.lastRespawnReq)).Round(10*time.Millisecond))
				shouldDead = false
				skipDeadRepair = true
			} else if shouldDead && liveWorldLoadGrace {
				debugInfo = fmt.Sprintf("unit=0 world=loading live_age=%s", liveWorldLoadAge)
				shouldDead = false
				skipDeadRepair = true
			} else if !shouldDead {
				if s.UnitInfoFn != nil {
					info, ok := s.UnitInfoFn(c.unitID)
					if !ok {
						if freshSpawnGrace || recentRespawnGrace {
							debugInfo = fmt.Sprintf("unit=%d world=missing respawn_age=%s", c.unitID, now.Sub(recentNonZeroTime(c.lastSpawnAt, c.lastRespawnReq)).Round(10*time.Millisecond))
							shouldDead = false
							skipDeadRepair = true
						} else if liveWorldLoadGrace {
							debugInfo = fmt.Sprintf("unit=%d world=missing live_age=%s", c.unitID, liveWorldLoadAge)
							shouldDead = false
							skipDeadRepair = true
						} else {
							debugInfo = fmt.Sprintf("unit=%d world=missing", c.unitID)
							shouldDead = true
						}
					} else {
						debugInfo = fmt.Sprintf("unit=%d world_hp=%.2f team=%d type=%d", c.unitID, info.Health, info.TeamID, info.TypeID)
						if info.Health <= 0 {
							shouldDead = true
						} else if recentRespawnGrace {
							debugInfo = fmt.Sprintf("unit=%d world_hp=%.2f team=%d type=%d respawn_age=%s",
								c.unitID, info.Health, info.TeamID, info.TypeID,
								now.Sub(recentNonZeroTime(c.lastSpawnAt, c.lastRespawnReq)).Round(10*time.Millisecond))
							shouldDead = false
							skipDeadRepair = true
						} else if liveWorldLoadGrace {
							debugInfo = fmt.Sprintf("unit=%d world_hp=%.2f team=%d type=%d live_age=%s",
								c.unitID, info.Health, info.TeamID, info.TypeID, liveWorldLoadAge)
							shouldDead = false
						}
					}
				} else {
					s.entityMu.Lock()
					_, ok := s.entities[c.unitID]
					s.entityMu.Unlock()
					debugInfo = fmt.Sprintf("unit=%d mirror_exists=%v", c.unitID, ok)
					if !ok {
						shouldDead = true
					}
				}
			}
			if shouldDead {
				if c.dead && c.unitID == 0 && !c.controlBuildActive {
					c.clientDeadIgnores = 0
					c.lastDeadIgnoreAt = time.Time{}
				} else {
					fmt.Printf("[net] client dead accepted conn=%d player=%d %s\n", c.id, c.playerID, debugInfo)
					c.clientDeadIgnores = 0
					c.lastDeadIgnoreAt = time.Time{}
					s.markDead(c, "client-dead")
				}
			} else {
				if s.shouldLogClientDeadIgnored(c, now) {
					fmt.Printf("[net] client dead ignored conn=%d player=%d %s\n", c.id, c.playerID, debugInfo)
				}
				if skipDeadRepair {
					c.clientDeadIgnores = 0
					c.lastDeadIgnoreAt = time.Time{}
				} else if s.shouldForceRespawnAfterDeadIgnored(c, now) {
					fmt.Printf("[net] client dead stuck conn=%d player=%d ignores=%d action=repair-alive-once\n",
						c.id, c.playerID, c.clientDeadIgnores)
					s.repairClientDeadStuckBinding(c)
				} else {
					s.repairClientDeadAliveBinding(c)
				}
			}
		}
		if c.controlBuildActive {
			if info, ok := s.currentControlledBuildInfo(c); ok {
				c.snapX = info.X
				c.snapY = info.Y
				if info.TeamID != 0 {
					c.teamID = info.TeamID
				}
			} else {
				s.markDead(c, "controlled-build-missing")
			}
		} else if v.UnitID != 0 {
			// Do not trust arbitrary client-reported unit IDs.
			// Only adopt when it is already current, or when server entity ownership matches this player.
			acceptUnitID := v.UnitID == c.unitID
			if !acceptUnitID {
				s.entityMu.Lock()
				ent, exists := s.entities[v.UnitID]
				s.entityMu.Unlock()
				if exists {
					if u, ok := ent.(*protocol.UnitEntitySync); ok {
						if ctrl, ok := u.Controller.(*protocol.ControllerState); ok && ctrl != nil &&
							ctrl.Type == protocol.ControllerPlayer && ctrl.PlayerID == c.playerID {
							acceptUnitID = true
						}
					}
				}
			}
			if acceptUnitID {
				c.unitID = v.UnitID
			}
		} else if c.dead {
			c.unitID = 0
		}
		if prevUnitID != 0 && prevUnitID != c.unitID {
			s.detachConnUnit(c, prevUnitID)
		}

		verifyPosition := s.ClientPositionVerificationEnabled()
		// Apply motion/position into authoritative world state when possible.
		ignorePosition := c.controlBuildActive || v.Dead || c.unitID == 0 || (v.UnitID != 0 && v.UnitID != c.unitID)
		if !ignorePosition {
			if !verifyPosition {
				// Official NetServer.clientSnapshot() only verifies position in
				// strict/headless mode. The normal dedicated-server path accepts
				// the client snapshot as authoritative movement input.
				c.snapX = v.X
				c.snapY = v.Y
				if s.SetUnitMotionFn != nil {
					_ = s.SetUnitMotionFn(c.unitID, v.XVelocity, v.YVelocity, 0)
				}
				if s.SetUnitPositionFn != nil {
					_ = s.SetUnitPositionFn(c.unitID, v.X, v.Y, v.Rotation)
				}
			} else {
				// If the client jumps too far away, correct them back to server position.
				// Vanilla uses tilesize*14. tilesize=8, so correctDist=112.
				const correctDist = 112.0
				var curX, curY float32
				var hasCur bool
				if s.UnitInfoFn != nil {
					if info, ok := s.UnitInfoFn(c.unitID); ok {
						curX, curY, hasCur = info.X, info.Y, true
					}
				}
				if !hasCur {
					// Fallback to last known snapshot position.
					curX, curY, hasCur = c.snapX, c.snapY, true
				}
				dx := float64(v.X - curX)
				dy := float64(v.Y - curY)
				dist := math.Hypot(dx, dy)
				if dist > correctDist {
					_ = c.SendAsync(&protocol.Remote_NetClient_setPosition_29{X: curX, Y: curY})
					// Keep server-side snap in sync with authoritative position.
					c.snapX, c.snapY = curX, curY
				} else {
					c.snapX = v.X
					c.snapY = v.Y
					if s.SetUnitMotionFn != nil {
						_ = s.SetUnitMotionFn(c.unitID, v.XVelocity, v.YVelocity, 0)
					}
					if s.SetUnitPositionFn != nil {
						_ = s.SetUnitPositionFn(c.unitID, v.X, v.Y, v.Rotation)
					}
				}
			}
		} else if !c.controlBuildActive {
			// When dead or mismatched, keep player coords updated but do not move unit.
			c.snapX = v.X
			c.snapY = v.Y
		}

		c.building = v.Building
		c.selectedRotation = v.SelectedRotation
		c.selectedBlockID = -1
		if v.SelectedBlock != nil {
			c.selectedBlockID = v.SelectedBlock.ID()
		}
		c.miningTilePos = invalidTilePos
		if v.Mining != nil {
			c.miningTilePos = v.Mining.Pos()
		}
		plans := extractBuildPlans(v.Plans)
		if c.controlBuildActive {
			if s.SetControlledBuildInputFn != nil {
				_ = s.SetControlledBuildInputFn(c.playerID, c.controlBuildPos, c.pointerX, c.pointerY, c.shooting)
			}
		} else if c.unitID != 0 && s.SetUnitRuntimeStateFn != nil {
			_ = s.SetUnitRuntimeStateFn(c.unitID, UnitRuntimeState{
				Shooting:       c.shooting,
				Boosting:       c.boosting,
				UpdateBuilding: c.building,
				MineTilePos:    c.miningTilePos,
				Plans:          plans,
			})
		}
		// Keep sync entities updated for snapshot stream.
		if p := s.ensurePlayerEntity(c); p != nil {
			s.updatePlayerEntity(p, c)
		}
		if c.unitID != 0 {
			if u := s.ensurePlayerUnitEntity(c); u != nil && s.UnitSyncFn == nil {
				u.Shooting = c.shooting
				u.MineTile = v.Mining
				u.UpdateBuilding = c.building
				u.Rotation = v.Rotation
				u.Vel = protocol.Vec2{X: v.XVelocity, Y: v.YVelocity}
				// X/Y will be refreshed from world by syncUnitFromWorld if UnitInfoFn is set.
				u.X = c.snapX
				u.Y = c.snapY
			}
		}
		if s.OnBuildPlanSnapshot != nil {
			s.OnBuildPlanSnapshot(c, plans)
		} else if s.OnBuildPlanPreview != nil {
			s.OnBuildPlanPreview(c, plans)
		} else if len(plans) > 0 && s.OnBuildPlans != nil {
			s.OnBuildPlans(c, plans)
		}
	case *protocol.Remote_NetServer_clientPlanSnapshot_46:
		plans := extractBuildPlans(v.Plans)
		s.recordClientPlanPreview(c, v.GroupId, plans)
		if s.OnBuildPlanPreview != nil {
			s.OnBuildPlanPreview(c, plans)
		} else if len(plans) > 0 && s.OnBuildPlanSnapshot != nil {
			s.OnBuildPlanSnapshot(c, plans)
		} else if len(plans) > 0 && s.OnBuildPlans != nil {
			s.OnBuildPlans(c, plans)
		}
	case *protocol.Remote_InputHandler_pingLocation_73:
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 73, "Remote_InputHandler_pingLocation_73", "ping_location")
		}
		s.broadcastPingLocation(c, v.X, v.Y, v.Text)
	case *protocol.Remote_NetClient_ping_18:
		// The framework keepalive loop is authoritative for connection liveness.
		_ = v
	case *protocol.Remote_NetClient_sendChatMessage_16:
		msg := sanitizeChatMessage(v.Message)
		if msg == "" {
			return
		}
		if s.OnChat != nil && s.OnChat(c, msg) {
			return
		}
		for _, h := range s.chatHandlers {
			if h(c, msg) {
				return
			}
		}
		s.broadcastPlayerChat(c, msg)
	case *protocol.Remote_InputHandler_buildingControlSelect_92:
		if s.DevLogger != nil {
			pos := int32(0)
			if v.Build != nil {
				pos = v.Build.Pos()
			}
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 10, "Remote_InputHandler_buildingControlSelect_92", fmt.Sprintf("pos=%d", pos))
		}
		if s.OnBuildingControlSelect != nil && v.Build != nil {
			s.OnBuildingControlSelect(c, v.Build.Pos())
		}
	case *protocol.Remote_Units_unitSpawn_51:
		// Handle unit spawn from server (unit from WorldStream)
		// This is called when a unit is spawned on the server and needs to be synced to clients
		if container := v.Container; container != nil {
			s.addEntity(container.Unit())
		}
		if s.DevLogger != nil {
			unit := v.Container.Unit()
			if unit != nil {
				s.DevLogger.LogUnit(unit.ID(), fmt.Sprintf("%T", unit), "unitSpawn",
					devlog.Int32Fld("unit_id", unit.ID()))
			}
		}
	case *protocol.Remote_Units_unitCapDeath_52:
		// Handle unit capacity death
		if unit := v.Unit; unit != nil {
			unitID := unit.ID()
			s.entityMu.Lock()
			if ent := s.entities[unitID]; ent != nil {
				delete(s.entities, unitID)
			}
			s.entityMu.Unlock()
			fmt.Printf("[net] unitCapDeath id=%d\n", unitID)
		}
	case *protocol.Remote_Units_unitEnvDeath_53:
		// Handle environmental unit death
		if unit := v.Unit; unit != nil {
			unitID := unit.ID()
			s.entityMu.Lock()
			if ent := s.entities[unitID]; ent != nil {
				delete(s.entities, unitID)
			}
			s.entityMu.Unlock()
			fmt.Printf("[net] unitEnvDeath id=%d\n", unitID)
		}
	case *protocol.Remote_Units_unitDeath_54:
		// Handle unit death (delayed)
		unitID := v.Uid
		s.entityMu.Lock()
		if ent := s.entities[unitID]; ent != nil {
			delete(s.entities, unitID)
		}
		s.entityMu.Unlock()
		fmt.Printf("[net] unitDeath id=%d\n", unitID)
	case *protocol.Remote_Units_unitDestroy_55:
		// Handle immediate unit destruction
		unitID := v.Uid
		s.entityMu.Lock()
		if ent := s.entities[unitID]; ent != nil {
			delete(s.entities, unitID)
		}
		s.entityMu.Unlock()
		fmt.Printf("[net] unitDestroy id=%d\n", unitID)
	case *protocol.Remote_Units_unitDespawn_56:
		// Handle unit despawn (removes unit but doesn't destroy)
		if unit := v.Unit; unit != nil {
			unitID := unit.ID()
			s.entityMu.Lock()
			if ent := s.entities[unitID]; ent != nil {
				delete(s.entities, unitID)
			}
			s.entityMu.Unlock()
			fmt.Printf("[net] unitDespawn id=%d\n", unitID)
		}
	case *protocol.Remote_Units_unitSafeDeath_57:
		// Handle unit safe death (non damaging death)
		if unit := v.Unit; unit != nil {
			unitID := unit.ID()
			s.entityMu.Lock()
			if ent := s.entities[unitID]; ent != nil {
				delete(s.entities, unitID)
			}
			s.entityMu.Unlock()
			fmt.Printf("[net] unitSafeDeath id=%d\n", unitID)
		}
	case *protocol.Remote_BulletType_createBullet_58:
		// Create bullet entity
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 55, "Remote_BulletType_createBullet_58", "create_bullet")
		}
	case *protocol.Remote_Teams_destroyPayload_59:
		// Destroy payload on team entities
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 56, "Remote_Teams_destroyPayload_59", "destroy_payload")
		}
	case *protocol.Remote_InputHandler_transferItemEffect_60:
		// 物品传输效果
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 57, "Remote_InputHandler_transferItemEffect_60", "transfer_item_effect")
		}
	case *protocol.Remote_InputHandler_takeItems_61:
		// 抽取物品
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 58, "Remote_InputHandler_takeItems_61", "take_items")
		}
	case *protocol.Remote_InputHandler_transferItemToUnit_62:
		// 传输物品到单位
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 59, "Remote_InputHandler_transferItemToUnit_62", "transfer_item_to_unit")
		}
	case *protocol.Remote_InputHandler_setItem_63:
		// 设置单个物品槽位
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 60, "Remote_InputHandler_setItem_63", "set_item")
		}
	case *protocol.Remote_InputHandler_setItems_64:
		// 批量设置物品
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 61, "Remote_InputHandler_setItems_64", "set_items")
		}
	case *protocol.Remote_InputHandler_setTileItems_65:
		// 设置地砖物品
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 62, "Remote_InputHandler_setTileItems_65", "set_tile_items")
		}
	case *protocol.Remote_InputHandler_clearItems_66:
		// 清空物品
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 63, "Remote_InputHandler_clearItems_66", "clear_items")
		}
	case *protocol.Remote_InputHandler_setLiquid_67:
		// 设置单个液体槽位
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 64, "Remote_InputHandler_setLiquid_67", "set_liquid")
		}
	case *protocol.Remote_InputHandler_setLiquids_68:
		// 批量设置液体
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 65, "Remote_InputHandler_setLiquids_68", "set_liquids")
		}
	case *protocol.Remote_InputHandler_setTileLiquids_69:
		// 设置地砖液体
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 66, "Remote_InputHandler_setTileLiquids_69", "set_tile_liquids")
		}
	case *protocol.Remote_InputHandler_clearLiquids_70:
		// 清空液体
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 67, "Remote_InputHandler_clearLiquids_70", "clear_liquids")
		}
	case *protocol.Remote_InputHandler_transferItemTo_71:
		// 通用物品传输到
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 68, "Remote_InputHandler_transferItemTo_71", "transfer_item_to")
		}
	case *protocol.Remote_InputHandler_deletePlans_72:
		// 删除建造计划
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 69, "Remote_InputHandler_deletePlans_72", "delete_plans")
		}
		if s.OnDeletePlans != nil && len(v.Positions) > 0 {
			s.OnDeletePlans(c, v.Positions)
		}
	case *protocol.Remote_InputHandler_commandUnits_74:
		// 指挥多个单位
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 70, "Remote_InputHandler_commandUnits_74", fmt.Sprintf("command_units(count=%d)", len(v.UnitIds)))
		}
		if s.OnCommandUnits != nil && len(v.UnitIds) > 0 {
			s.OnCommandUnits(c, v.UnitIds, v.BuildTarget, v.UnitTarget, v.PosTarget, v.QueueCommand, v.FinalBatch)
		}
	case *protocol.Remote_InputHandler_setUnitCommand_75:
		// 设置单位命令
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 71, "Remote_InputHandler_setUnitCommand_75", fmt.Sprintf("set_unit_command(count=%d)", len(v.UnitIds)))
		}
		if s.OnSetUnitCommand != nil && len(v.UnitIds) > 0 {
			s.OnSetUnitCommand(c, v.UnitIds, v.Command)
		}
	case *protocol.Remote_InputHandler_setUnitStance_76:
		// 设置单位姿态
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 72, "Remote_InputHandler_setUnitStance_76", fmt.Sprintf("set_unit_stance(count=%d)", len(v.UnitIds)))
		}
		if s.OnSetUnitStance != nil && len(v.UnitIds) > 0 {
			s.OnSetUnitStance(c, v.UnitIds, v.Stance, v.Enable)
		}
	case *protocol.Remote_InputHandler_commandBuilding_77:
		// 命令建筑
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 73, "Remote_InputHandler_commandBuilding_77", fmt.Sprintf("command_building(count=%d)", len(v.Buildings)))
		}
		if s.OnCommandBuilding != nil && len(v.Buildings) > 0 {
			s.OnCommandBuilding(c, v.Buildings, v.Target)
		}
	case *protocol.Remote_InputHandler_requestItem_78:
		// 请求物品
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 74, "Remote_InputHandler_requestItem_78", "request_item")
		}
		if s.OnRequestItem != nil && v.Build != nil && v.Item != nil {
			s.OnRequestItem(c, v.Build.Pos(), v.Item.ID(), v.Amount)
		}
	case *protocol.Remote_InputHandler_transferInventory_79:
		// 转移库存
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 75, "Remote_InputHandler_transferInventory_79", "transfer_inventory")
		}
		if s.OnTransferInventory != nil && v.Build != nil {
			s.OnTransferInventory(c, v.Build.Pos())
		}
	case *protocol.Remote_InputHandler_removeQueueBlock_80:
		// 从队列中移除方块
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 76, "Remote_InputHandler_removeQueueBlock_80", fmt.Sprintf("remove_queue(x=%d y=%d breaking=%v)", v.X, v.Y, v.Breaking))
		}
		if s.OnRemoveQueueBlock != nil {
			s.OnRemoveQueueBlock(c, v.X, v.Y, v.Breaking)
		}
	case *protocol.Remote_InputHandler_requestUnitPayload_81:
		// 请求单位载荷
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 77, "Remote_InputHandler_requestUnitPayload_81", "request_unit_payload")
		}
		if s.OnRequestUnitPayload != nil && v.Target != nil {
			s.OnRequestUnitPayload(c, v.Target.ID())
		}
	case *protocol.Remote_InputHandler_requestBuildPayload_82:
		// 请求建筑载荷
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 78, "Remote_InputHandler_requestBuildPayload_82", "request_build_payload")
		}
		if s.OnRequestBuildPayload != nil && v.Build != nil {
			s.OnRequestBuildPayload(c, v.Build.Pos())
		}
	case *protocol.Remote_InputHandler_pickedUnitPayload_83:
		// 单位运载选择
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 79, "Remote_InputHandler_pickedUnitPayload_83", "picked_unit_payload")
		}
		if s.OnRequestUnitPayload != nil && v.Target != nil {
			s.OnRequestUnitPayload(c, v.Target.ID())
		}
	case *protocol.Remote_InputHandler_pickedBuildPayload_84:
		// 建筑运载选择
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 80, "Remote_InputHandler_pickedBuildPayload_84", "picked_build_payload")
		}
		if s.OnRequestBuildPayload != nil && v.Build != nil {
			s.OnRequestBuildPayload(c, v.Build.Pos())
		}
	case *protocol.Remote_InputHandler_requestDropPayload_85:
		// 请求丢弃运载物
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 81, "Remote_InputHandler_requestDropPayload_85", fmt.Sprintf("request_drop_payload(x=%.1f y=%.1f)", v.X, v.Y))
		}
		if s.OnRequestDropPayload != nil {
			s.OnRequestDropPayload(c, v.X, v.Y)
		}
	case *protocol.Remote_InputHandler_payloadDropped_86:
		// 运载物已丢弃
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 82, "Remote_InputHandler_payloadDropped_86", fmt.Sprintf("payload_dropped(x=%.1f y=%.1f)", v.X, v.Y))
		}
		if s.OnRequestDropPayload != nil {
			s.OnRequestDropPayload(c, v.X, v.Y)
		}
	case *protocol.Remote_InputHandler_unitEnteredPayload_87:
		// 单位进入运载
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 83, "Remote_InputHandler_unitEnteredPayload_87", "unit_entered_payload")
		}
		if s.OnUnitEnteredPayload != nil && v.Unit != nil && v.Build != nil {
			s.OnUnitEnteredPayload(c, v.Unit.ID(), v.Build.Pos())
		}
	case *protocol.Remote_InputHandler_dropItem_88:
		// 投放物品
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 84, "Remote_InputHandler_dropItem_88", "drop_item")
		}
		if s.OnDropItem != nil {
			s.OnDropItem(c, v.Angle)
		}
	case *protocol.Remote_InputHandler_rotateBlock_89:
		if s.DevLogger != nil {
			pos := int32(0)
			if v.Build != nil {
				pos = v.Build.Pos()
			}
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 83, "Remote_InputHandler_rotateBlock_89", fmt.Sprintf("pos=%d dir=%v", pos, v.Direction))
		}
		if s.OnRotateBlock != nil && v.Build != nil {
			s.OnRotateBlock(c, v.Build.Pos(), v.Direction)
		}
	case *protocol.Remote_NetServer_requestDebugStatus_36:
		resp := &protocol.Remote_NetServer_debugStatusClient_37{
			Value:              0,
			LastClientSnapshot: c.lastClientSnapshot.Load(),
			SnapshotsSent:      c.snapshotsSent.Load(),
		}
		_ = c.Send(resp)
	case *protocol.Remote_NetServer_requestBlockSnapshot_45:
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 45, "Remote_NetServer_requestBlockSnapshot_45", fmt.Sprintf("pos=%d", v.Pos))
		}
		if s.OnRequestBlockSnapshot != nil {
			s.OnRequestBlockSnapshot(c, v.Pos)
		}
	case *protocol.Remote_Build_beginBreak_132:
		//客户端请求开始破坏建筑
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 123, "Remote_Build_beginBreak_132", fmt.Sprintf("x=%d y=%d", v.X, v.Y))
		}
		s.handleOfficialBeginBreak(c, v.X, v.Y)
	case *protocol.Remote_Build_beginPlace_133:
		// 客户端请求开始放置建筑
		blockID := int16(0)
		if v.Result != nil {
			blockID = v.Result.ID()
		}
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 124, "Remote_Build_beginPlace_133", fmt.Sprintf("x=%d y=%d block=%d", v.X, v.Y, blockID))
		}
		s.handleOfficialBeginPlace(c, BeginPlaceRequest{
			X:        v.X,
			Y:        v.Y,
			BlockID:  blockID,
			Rotation: int8(byte(v.Rotation) & 0x03),
			Config:   v.PlaceConfig,
		})
	case *protocol.Remote_InputHandler_tileConfig_90:
		if s.DevLogger != nil {
			pos := int32(0)
			if v.Build != nil {
				pos = v.Build.Pos()
			}
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 127, "Remote_InputHandler_tileConfig_90", fmt.Sprintf("pos=%d value=%T", pos, v.Value))
		}
		if s.OnTileConfig != nil && v.Build != nil {
			s.OnTileConfig(c, v.Build.Pos(), v.Value)
		}
	case *protocol.Remote_Tile_setFloor_137:
		// 设置地板
		if s.DevLogger != nil {
			floorID := int16(0)
			if v.Floor != nil {
				floorID = v.Floor.ID()
			}
			pos := v.Tile.Pos()
			x := int32(protocol.UnpackPoint2X(pos))
			y := int32(protocol.UnpackPoint2Y(pos))
			s.DevLogger.LogBuild(x, y, 0, "none", "setFloor",
				devlog.Int32Fld("tile_x", x),
				devlog.Int32Fld("tile_y", y),
				devlog.Int16Fld("floor_id", floorID))
		}
	case *protocol.Remote_Tile_setOverlay_138:
		// 设置覆盖层
		if s.DevLogger != nil {
			overlayID := int16(0)
			if v.Overlay != nil {
				overlayID = v.Overlay.ID()
			}
			pos := v.Tile.Pos()
			x := int32(protocol.UnpackPoint2X(pos))
			y := int32(protocol.UnpackPoint2Y(pos))
			s.DevLogger.LogBuild(x, y, 0, "none", "setOverlay",
				devlog.Int32Fld("tile_x", x),
				devlog.Int32Fld("tile_y", y),
				devlog.Int16Fld("overlay_id", overlayID))
		}
	case *protocol.Remote_Tile_removeTile_139:
		// 移除 Tile
		if s.DevLogger != nil {
			pos := v.Tile.Pos()
			x := int32(protocol.UnpackPoint2X(pos))
			y := int32(protocol.UnpackPoint2Y(pos))
			s.DevLogger.LogBuild(x, y, 0, "none", "removeTile",
				devlog.Int32Fld("tile_x", x),
				devlog.Int32Fld("tile_y", y))
		}
	case *protocol.Remote_Tile_setTile_140:
		// 设置 Tile（_block）
		if s.DevLogger != nil {
			blockID := int32(0)
			if v.Block != nil {
				blockID = int32(v.Block.ID())
			}
			team := "none"
			if v.Team.ID >= 0 && v.Team.ID < 16 {
				teamNames := []string{"none", "cyan", "purple", "pink", "sharded", "blue", "green", "crux", "moray", "brown", "olive", "teal", "cosmic", "yellow", "black", "white"}
				if int(v.Team.ID) < len(teamNames) {
					team = teamNames[v.Team.ID]
				}
			}
			pos := v.Tile.Pos()
			x := int32(protocol.UnpackPoint2X(pos))
			y := int32(protocol.UnpackPoint2Y(pos))
			s.DevLogger.LogBuild(x, y, int16(blockID), team, "setTile",
				devlog.Int32Fld("tile_pos", pos),
				devlog.Int32Fld("block_id", blockID),
				devlog.Int32Fld("rotation", v.Rotation))
		}
	case *protocol.Remote_Tile_buildDestroyed_143:
		// 建筑销毁
		if s.DevLogger != nil {
			pos := int32(0)
			if v.Build != nil {
				pos = v.Build.Pos()
			}
			x := int32(0)
			y := int32(0)
			if pos != 0 {
				pt := protocol.UnpackPoint2(pos)
				x = pt.X
				y = pt.Y
			}
			s.DevLogger.LogBuild(x, y, 0, "none", "buildDestroyed",
				devlog.Int32Fld("tile_pos", pos))
		}
	case *protocol.Remote_Tile_buildHealthUpdate_144:
		// 建筑生命值更新 - 简化处理（数据包结构需要进一步分析）
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 135, "Remote_Tile_buildHealthUpdate_144", "building_health_update")
		}
	case *protocol.Remote_InputHandler_unitControl_94:
		// 单位控制（需要进一步分析数据包结构）
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 90, "Remote_InputHandler_unitControl_94", "unit_control")
		}
		if c != nil && c.playerID != 0 {
			if buildPos, ok := extractControlledBuildPos(v.Unit); ok {
				s.controlBlockUnit(c, buildPos)
			} else if unitID := extractUnitID(v.Unit); unitID != 0 {
				s.unitControl(c, unitID)
			}
		}
	case *protocol.Remote_InputHandler_unitClear_95:
		// Vanilla InputHandler.unitClear() clears the current unit and immediately
		// requests a new core spawn; it is not a delayed death-timer request.
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 91, "Remote_InputHandler_unitClear_95", "unit_clear")
		}
		s.handleOfficialUnitClear(c)
	case protocol.FrameworkMessage:
		switch m := v.(type) {
		case *protocol.Ping:
			if !m.IsReply {
				m.IsReply = true
				_ = c.Send(m)
			}
		default:
			// ignore keepalive
		}
	case *protocol.Remote_Logic_sectorCapture_1:
		// 逻辑 - 领域捕获事件
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 1, "Remote_Logic_sectorCapture_1", "sector_capture")
		}
	case *protocol.Remote_Logic_updateGameOver_2:
		// 逻辑 - 游戏结束状态更新
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 2, "Remote_Logic_updateGameOver_2", "update_gameover")
		}
	case *protocol.Remote_Logic_gameOver_3:
		// 逻辑 - 游戏结束事件
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 3, "Remote_Logic_gameOver_3", "game_over")
		}
	case *protocol.Remote_Logic_researched_4:
		// 逻辑 - 科研解锁事件
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 4, "Remote_Logic_researched_4", "researched")
		}
	case *protocol.Remote_LExecutor_setMapArea_96:
		// 逻辑执行器 - 设置地图区域
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 92, "Remote_LExecutor_setMapArea_96", "set_map_area")
		}
	case *protocol.Remote_LExecutor_logicExplosion_97:
		// 逻辑执行器 - 逻辑爆炸效果
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 93, "Remote_LExecutor_logicExplosion_97", "logic_explosion")
		}
	case *protocol.Remote_LExecutor_syncVariable_98:
		// 逻辑执行器 - 同步变量值
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 94, "Remote_LExecutor_syncVariable_98", "sync_variable")
		}
	case *protocol.Remote_LExecutor_setFlag_99:
		// 逻辑执行器 - 设置逻辑标志
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 95, "Remote_LExecutor_setFlag_99", "set_flag")
		}
	case *protocol.Remote_LExecutor_createMarker_100:
		// 逻辑执行器 - 创建标记
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 96, "Remote_LExecutor_createMarker_100", "create_marker")
		}
	case *protocol.Remote_LExecutor_removeMarker_101:
		// 逻辑执行器 - 删除标记
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 97, "Remote_LExecutor_removeMarker_101", "remove_marker")
		}
	case *protocol.Remote_LExecutor_updateMarker_102:
		// 逻辑执行器 - 更新标记 (忽略，服务器不处理逻辑处理器)
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 98, "Remote_LExecutor_updateMarker_102", "update_marker")
		}
	case *protocol.Remote_LExecutor_updateMarkerText_103:
		// 逻辑执行器 - 更新标记文本 (忽略，服务器不处理逻辑处理器)
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 99, "Remote_LExecutor_updateMarkerText_103", "update_marker_text")
		}
	case *protocol.Remote_LExecutor_updateMarkerTexture_104:
		// 逻辑执行器 - 更新标记纹理 (忽略，服务器不处理逻辑处理器)
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 100, "Remote_LExecutor_updateMarkerTexture_104", "update_marker_texture")
		}
	case *protocol.Remote_Weather_createWeather_105:
		// 天气 - 创建天气效果
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 101, "Remote_Weather_createWeather_105", "create_weather")
		}
	case *protocol.Remote_Menus_menu_106:
		// 菜单 - 显示菜单
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 102, "Remote_Menus_menu_106", "menu")
		}
	case *protocol.Remote_Menus_followUpMenu_107:
		// 菜单 - 跟随菜单
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 103, "Remote_Menus_followUpMenu_107", "follow_up_menu")
		}
	case *protocol.Remote_Menus_hideFollowUpMenu_108:
		// 菜单 - 隐藏跟随菜单
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 104, "Remote_Menus_hideFollowUpMenu_108", "hide_follow_up_menu")
		}
	case *protocol.Remote_Menus_menuChoose_109:
		// 菜单 - 菜单选择
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 105, "Remote_Menus_menuChoose_109", "menu_choose")
		}
		if s.OnMenuChoose != nil {
			s.OnMenuChoose(c, v.MenuId, v.Option)
		}
	case *protocol.Remote_Menus_textInput_110:
		// 菜单 - 文本输入 (merged 106/107 -> 110)
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 110, "Remote_Menus_textInput_110", "text_input")
		}
	case *protocol.Remote_Menus_textInputResult_112:
		// 菜单 - 文本输入结果
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 108, "Remote_Menus_textInputResult_112", "text_input_result")
		}
	case *protocol.Remote_Menus_setHudText_113:
		// 菜单 - 设置HUD文本
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 109, "Remote_Menus_setHudText_113", "set_hud_text")
		}
	case *protocol.Remote_Menus_hideHudText_114:
		// 菜单 - 隐藏HUD文本
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 110, "Remote_Menus_hideHudText_114", "hide_hud_text")
		}
	case *protocol.Remote_Menus_setHudTextReliable_115:
		// 菜单 - 设置可靠HUD文本
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 111, "Remote_Menus_setHudTextReliable_115", "set_hud_text_reliable")
		}
	case *protocol.Remote_Menus_announce_116:
		// 菜单 - 公告
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 112, "Remote_Menus_announce_116", "announce")
		}
	case *protocol.Remote_Menus_infoMessage_117:
		// 菜单 - 信息消息
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 113, "Remote_Menus_infoMessage_117", "info_message")
		}
	case *protocol.Remote_Menus_infoPopup_118:
		// 菜单 - 信息弹窗
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 114, "Remote_Menus_infoPopup_118", "info_popup")
		}
	case *protocol.Remote_Menus_label_122:
		// 菜单 - 标签
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 115, "Remote_Menus_label_122", "label")
		}
	case *protocol.Remote_Menus_infoPopupReliable_119:
		// 菜单 - 可靠性信息弹窗
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 116, "Remote_Menus_infoPopupReliable_119", "info_popup_reliable")
		}
	case *protocol.Remote_Menus_labelReliable_123:
		// 菜单 - 可靠性标签
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 117, "Remote_Menus_labelReliable_123", "label_reliable")
		}
	case *protocol.Remote_Menus_infoToast_126:
		// 菜单 - 信息Toast
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 118, "Remote_Menus_infoToast_126", "info_toast")
		}
	case *protocol.Remote_Menus_warningToast_127:
		// 菜单 - 警告Toast
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 119, "Remote_Menus_warningToast_127", "warning_toast")
		}
	case *protocol.Remote_Menus_openURI_128:
		// 菜单 - 打开URI
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 120, "Remote_Menus_openURI_128", "open_uri")
		}
	case *protocol.Remote_Menus_removeWorldLabel_130:
		// 菜单 - 移除世界标签
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 121, "Remote_Menus_removeWorldLabel_130", "remove_world_label")
		}
	case *protocol.Remote_HudFragment_setPlayerTeamEditor_131:
		// HUD - 设置玩家队伍编辑器
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 122, "Remote_HudFragment_setPlayerTeamEditor_131", "set_player_team_editor")
		}
	case *protocol.Remote_Tile_setTileBlocks_134:
		// Tile - 设置 Tile 的 Blocks
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 125, "Remote_Tile_setTileBlocks_134", "set_tile_blocks")
		}
	case *protocol.Remote_Tile_setTileFloors_135:
		// Tile - 设置 Tile 的 Floors
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 126, "Remote_Tile_setTileFloors_135", "set_tile_floors")
		}
	case *protocol.Remote_Tile_setTileOverlays_136:
		// Tile - 设置 Tile 的 Overlays
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 127, "Remote_Tile_setTileOverlays_136", "set_tile_overlays")
		}
	case *protocol.Remote_Tile_setTeam_141:
		// Tile - 设置单个建筑的队伍
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 132, "Remote_Tile_setTeam_141", "set_team")
		}
	case *protocol.Remote_Tile_setTeams_142:
		// Tile - 批量设置建筑队伍
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 133, "Remote_Tile_setTeams_142", "set_teams")
		}
	case *protocol.Remote_InputHandler_unitBuildingControlSelect_93:
		if s.DevLogger != nil {
			pos := int32(0)
			unitID := int32(0)
			if v.Build != nil {
				pos = v.Build.Pos()
			}
			if v.Unit != nil {
				unitID = v.Unit.ID()
			}
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 135, "Remote_InputHandler_unitBuildingControlSelect_93", fmt.Sprintf("unit=%d pos=%d", unitID, pos))
		}
		if s.OnUnitBuildingControlSelect != nil && v.Unit != nil && v.Build != nil {
			s.OnUnitBuildingControlSelect(c, v.Unit.ID(), v.Build.Pos())
		}
	case *protocol.Remote_ConstructBlock_deconstructFinish_145:
		// 构造方块 - 解构完成
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 136, "Remote_ConstructBlock_deconstructFinish_145", "deconstruct_finish")
		}
		if s.OnDeconstructFinish != nil && v.Tile != nil && v.Block != nil {
			req := DeconstructFinishRequest{
				Pos:     v.Tile.Pos(),
				BlockID: v.Block.ID(),
			}
			if v.Builder != nil {
				req.BuilderID = v.Builder.ID()
			}
			s.OnDeconstructFinish(c, req)
		}
	case *protocol.Remote_ConstructBlock_constructFinish_146:
		// 构造方块 - 构造完成
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 137, "Remote_ConstructBlock_constructFinish_146", "construct_finish")
		}
		if s.OnConstructFinish != nil && v.Tile != nil && v.Block != nil {
			req := ConstructFinishRequest{
				Pos:      v.Tile.Pos(),
				BlockID:  v.Block.ID(),
				Rotation: v.Rotation,
				TeamID:   v.Team.ID,
				Config:   v.Config,
			}
			if v.Builder != nil {
				// Builder is any type - try to extract ID if it has one
				switch b := v.Builder.(type) {
				case interface{ ID() int32 }:
					req.BuilderID = b.ID()
				}
			}
			s.OnConstructFinish(c, req)
		}
	case *protocol.Remote_LandingPad_landingPadLanded_147:
		// 着陆平台 - 着陆
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 138, "Remote_LandingPad_landingPadLanded_147", "landing_pad_landed")
		}
	case *protocol.Remote_AutoDoor_autoDoorToggle_148:
		// 自动门 - 切换
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 139, "Remote_AutoDoor_autoDoorToggle_148", "auto_door_toggle")
		}
	case *protocol.Remote_CoreBlock_playerSpawn_149:
		// 核心区块 - 玩家生成
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 140, "Remote_CoreBlock_playerSpawn_149", "player_spawn")
		}
	case *protocol.Remote_UnitAssembler_assemblerUnitSpawned_150:
		// 单位组装器 - 单位生成
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 141, "Remote_UnitAssembler_assemblerUnitSpawned_150", "assembler_unit_spawned")
		}
		if s.OnAssemblerUnitSpawned != nil && v.Tile != nil {
			s.OnAssemblerUnitSpawned(c, v.Tile.Pos())
		}
	case *protocol.Remote_UnitAssembler_assemblerDroneSpawned_151:
		// 单位组装器 - 无人机生成
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 142, "Remote_UnitAssembler_assemblerDroneSpawned_151", "assembler_drone_spawned")
		}
		if s.OnAssemblerDroneSpawned != nil && v.Tile != nil {
			s.OnAssemblerDroneSpawned(c, AssemblerDroneSpawnedRequest{
				Pos:    v.Tile.Pos(),
				UnitID: v.Id,
			})
		}
	case *protocol.Remote_UnitBlock_unitBlockSpawn_152:
		// 单位方块 - 单位方块生成
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 143, "Remote_UnitBlock_unitBlockSpawn_152", "unit_block_spawn")
		}
	case *protocol.Remote_UnitCargoLoader_unitTetherBlockSpawned_153:
		// 单位货物加载器 - 无人机生成
		if s.DevLogger != nil {
			s.DevLogger.LogPacketReceived(c.id, c.playerID, 144, "Remote_UnitCargoLoader_unitTetherBlockSpawned_153", "unit_tether_block_spawned")
		}
	default:
		// ignore
	}
}

func previewHex(data []byte, max int) string {
	if len(data) == 0 || max <= 0 {
		return ""
	}
	if len(data) > max {
		data = data[:max]
	}
	return hex.EncodeToString(data)
}

func extractString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case *string:
		if t == nil {
			return ""
		}
		return *t
	default:
		return ""
	}
}

func extractBuildPlans(v any) []*protocol.BuildPlan {
	switch t := v.(type) {
	case []*protocol.BuildPlan:
		return t
	case []any:
		out := make([]*protocol.BuildPlan, 0, len(t))
		for _, it := range t {
			if p, ok := it.(*protocol.BuildPlan); ok && p != nil {
				out = append(out, p)
			}
		}
		return out
	default:
		return nil
	}
}

func sanitizeChatMessage(s string) string {
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Keep parity with Mindustry text length constraint.
	runes := []rune(s)
	if len(runes) > 150 {
		s = string(runes[:150])
	}
	return s
}

func formatChatName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "[lightgray][server][white]: "
	}
	return name + "[white]: "
}

func makeSendMessagePacket(message string, unformatted *string) protocol.Packet {
	if unformatted == nil {
		return &protocol.Remote_NetClient_sendMessage_14{Message: message}
	}
	return &protocol.Remote_NetClient_sendMessage_15{
		Message:      message,
		Unformatted:  *unformatted,
		Playersender: nil,
	}
}

func (s *Server) broadcastSimpleMessage(message string) {
	s.mu.Lock()
	peers := make([]*Conn, 0, len(s.conns))
	for c := range s.conns {
		peers = append(peers, c)
	}
	s.mu.Unlock()

	for _, peer := range peers {
		_ = peer.SendAsync(makeSendMessagePacket(message, nil))
	}
}

func (s *Server) broadcastPlayerChat(sender *Conn, message string) {
	if sender == nil {
		return
	}
	message = sanitizeChatMessage(message)
	if message == "" {
		return
	}
	senderName := s.playerDisplayName(sender)
	if sender.playerID == 0 {
		// Fallback for unexpected state.
		s.broadcastSimpleMessage(formatChatName(senderName) + message)
		return
	}
	s.mu.Lock()
	peers := make([]*Conn, 0, len(s.conns))
	for c := range s.conns {
		peers = append(peers, c)
	}
	s.mu.Unlock()
	formatted := formatChatName(senderName) + message
	for _, peer := range peers {
		raw := message
		_ = peer.SendAsync(makeSendMessagePacket(formatted, &raw))
	}
}

func (s *Server) sendSystemMessage(c *Conn, message string) {
	if c == nil {
		return
	}
	_ = c.SendAsync(makeSendMessagePacket(message, nil))
}

func (s *Server) SendChat(c *Conn, message string) {
	s.sendSystemMessage(c, message)
}

func (s *Server) broadcastJoinLeaveChat(c *Conn, joined bool) {
	if s == nil || c == nil || !s.joinLeaveChatEnabled.Load() || !c.hasBegunConnecting {
		return
	}
	name := strings.TrimSpace(s.playerDisplayName(c))
	if name == "" {
		name = "未知玩家"
	}
	action := "加入了游戏"
	if !joined {
		action = "退出了游戏"
	}
	message := sanitizeChatMessage(fmt.Sprintf("%s %s", name, action))
	if message == "" {
		return
	}
	s.broadcastSimpleMessage(message)
}

func (s *Server) BroadcastChat(message string) {
	message = sanitizeChatMessage(message)
	if message == "" {
		return
	}
	formatted := formatChatName("") + message
	s.broadcastSimpleMessage(formatted)
}

func (s *Server) Broadcast(obj any) {
	if obj == nil {
		return
	}
	s.mu.Lock()
	peers := make([]*Conn, 0, len(s.conns))
	for c := range s.conns {
		peers = append(peers, c)
	}
	s.mu.Unlock()
	for _, peer := range peers {
		if peer == nil || !peer.hasConnected || peer.InWorldReloadGrace() {
			continue
		}
		_ = peer.SendAsync(obj)
	}
}

func (s *Server) BroadcastUnreliable(obj any) {
	if obj == nil {
		return
	}
	s.mu.Lock()
	peers := make([]*Conn, 0, len(s.conns))
	for c := range s.conns {
		peers = append(peers, c)
	}
	s.mu.Unlock()
	for _, peer := range peers {
		if peer == nil || !peer.hasConnected || peer.InWorldReloadGrace() {
			continue
		}
		if s.udpConn != nil && peer.UDPAddr() == nil {
			if _, isBlockSnapshot := obj.(*protocol.Remote_NetClient_blockSnapshot_34); isBlockSnapshot {
				continue
			}
		}
		_ = s.sendUnreliable(peer, obj)
	}
}

func (s *Server) SendStatusTo(c *Conn) {
	if c == nil {
		return
	}
	msg := fmt.Sprintf("[accent]status[]: sessions=%d", len(s.ListSessions()))
	s.sendSystemMessage(c, msg)
}

func (s *Server) emitEvent(c *Conn, kind, packet, detail string) {
	if s.OnEvent == nil {
		if s.EventManager == nil || s.shouldSuppressHotNetEvent(kind) {
			return
		}
	} else if s.shouldSuppressHotNetEvent(kind) {
		return
	}
	ev := NetEvent{
		Timestamp: time.Now().UTC(),
		Kind:      kind,
		Packet:    packet,
		Detail:    detail,
	}
	if c != nil {
		ev.ConnID = c.id
		ev.UUID = c.uuid
		ev.IP = c.remoteIP()
		ev.Name = c.name
	}
	if s.OnEvent != nil {
		s.OnEvent(ev)
	}

	// 也分发到事件管理器
	if s.EventManager != nil {
		// 将 NetEvent 转换 storage.Event
		stgEv := storage.Event{
			Timestamp: ev.Timestamp,
			Kind:      ev.Kind,
			Packet:    ev.Packet,
			Detail:    ev.Detail,
			ConnID:    ev.ConnID,
			UUID:      ev.UUID,
			IP:        ev.IP,
			Name:      ev.Name,
		}
		// todo: 映射 kind 到 Trigger
		s.EventManager.Dispatch(stgEv)
	}
}

func (s *Server) shouldSuppressHotNetEvent(kind string) bool {
	if s == nil || s.verboseNetLog.Load() {
		return false
	}
	switch kind {
	case "packet_recv":
		return !s.packetRecvEventsEnabled.Load()
	case "packet_send":
		return !s.packetSendEventsEnabled.Load()
	default:
		return false
	}
}

func (s *Server) addConn(c *Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conns[c] = struct{}{}
	s.clearEntitySnapshotCache()
}

func (s *Server) removeConn(c *Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.conns, c)
	delete(s.pending, c.id)
	if c.udpAddr != nil {
		delete(s.byUDP, c.udpAddr.String())
	}
	if c.playerID != 0 {
		s.previewMu.Lock()
		delete(s.previewPlans, c.playerID)
		s.previewMu.Unlock()
		s.entityMu.Lock()
		delete(s.entities, c.playerID)
		if c.unitID != 0 {
			delete(s.entities, c.unitID)
		}
		s.entityMu.Unlock()
	}
	s.clearEntitySnapshotCache()
}

