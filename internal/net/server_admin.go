package net

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/IYanHua/mdt-server/internal/protocol"
)

type SessionInfo struct {
	ID        int32
	Name      string
	UUID      string
	IP        string
	Connected bool
	Stats     ConnStats
}

type PlayerSnapshot struct {
	ID        int32
	Name      string
	UUID      string
	IP        string
	Connected bool
	UnitID    int32
	TeamID    byte
	X         float32
	Y         float32
	Building  bool
	Dead      bool
}

type ConnStats struct {
	Sent        int64
	SendErrors  int64
	Queued      int64
	QueueFull   int64
	BytesSent   int64
	UdpSent     int64
	UdpErrors   int64
	ByTypeSent  map[string]int64
	ByTypeBytes map[string]int64
}

func (c *Conn) Stats() ConnStats {
	byTypeSent := map[string]int64{}
	byTypeBytes := map[string]int64{}
	c.statsMu.Lock()
	for k, v := range c.byTypeSent {
		byTypeSent[k] = v
	}
	for k, v := range c.byTypeBytes {
		byTypeBytes[k] = v
	}
	c.statsMu.Unlock()
	return ConnStats{
		Sent:        c.sendCount.Load(),
		SendErrors:  c.sendErrors.Load(),
		Queued:      c.sendQueued.Load(),
		QueueFull:   c.sendQueueFull.Load(),
		BytesSent:   c.bytesSent.Load(),
		UdpSent:     c.udpSent.Load(),
		UdpErrors:   c.udpErrors.Load(),
		ByTypeSent:  byTypeSent,
		ByTypeBytes: byTypeBytes,
	}
}

func (c *Conn) recordSend(obj any, size int64) {
	name := packetTypeName(obj)
	c.statsMu.Lock()
	c.byTypeSent[name]++
	c.byTypeBytes[name] += size
	c.statsMu.Unlock()
}

func packetTypeName(obj any) string {
	if obj == nil {
		return "<nil>"
	}
	return reflect.TypeOf(obj).String()
}

func packetSendDetail(size, packetID, frameworkID int) string {
	var b strings.Builder
	b.Grow(40)
	b.WriteString("size=")
	b.WriteString(strconv.Itoa(size))
	if packetID >= 0 {
		b.WriteString(" packet_id=")
		b.WriteString(strconv.Itoa(packetID))
	}
	if frameworkID >= 0 {
		b.WriteString(" framework_id=")
		b.WriteString(strconv.Itoa(frameworkID))
	}
	return b.String()
}

func (s *Server) ListSessions() []SessionInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]SessionInfo, 0, len(s.conns))
	for c := range s.conns {
		out = append(out, SessionInfo{
			ID:        c.id,
			Name:      c.name,
			UUID:      c.uuid,
			IP:        c.remoteIP(),
			Connected: c.hasConnected,
			Stats:     c.Stats(),
		})
	}
	return out
}

func (s *Server) ListPlayerSnapshots() []PlayerSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]PlayerSnapshot, 0, len(s.conns))
	for c := range s.conns {
		x, y := c.SnapshotPos()
		out = append(out, PlayerSnapshot{
			ID:        c.id,
			Name:      c.Name(),
			UUID:      c.UUID(),
			IP:        c.remoteIP(),
			Connected: c.hasConnected,
			UnitID:    c.UnitID(),
			TeamID:    c.TeamID(),
			X:         x,
			Y:         y,
			Building:  c.IsBuilding(),
			Dead:      c.IsDead(),
		})
	}
	return out
}

func (s *Server) KickByID(id int32, reason string) bool {
	var target *Conn
	s.mu.Lock()
	for c := range s.conns {
		if c.id == id {
			target = c
			break
		}
	}
	s.mu.Unlock()
	if target == nil {
		return false
	}
	if reason == "" {
		reason = "kicked by admin"
	}
	s.noteRecentKick(target.uuid, target.remoteIP())
	_ = target.SendAsync(&protocol.Remote_NetClient_kick_22{Reason: reason})
	s.closeConnLater(target, 250*time.Millisecond)
	return true
}

// KickForMapChange sends the map-change kick string and then closes shortly after
// so clients can auto-reconnect cleanly.
func (s *Server) KickForMapChange(id int32) bool {
	var target *Conn
	s.mu.Lock()
	for c := range s.conns {
		if c.id == id {
			target = c
			break
		}
	}
	s.mu.Unlock()
	if target == nil {
		return false
	}
	// Use blocking send for map-change path to avoid drop under send-queue pressure.
	_ = target.Send(&protocol.Remote_NetClient_kick_22{Reason: "map changed, please reconnect"})
	s.closeConnLater(target, 400*time.Millisecond)
	return true
}

// NotifyShutdown sends a disconnect reason to all live connections and closes them shortly after.
// Use blocking sends here so clients have the best chance to display the message before exit.
func (s *Server) NotifyShutdown(reason string, delay time.Duration) int {
	if s == nil {
		return 0
	}
	if reason == "" {
		reason = "server shutting down"
	}
	if delay < 0 {
		delay = 0
	}
	s.mu.Lock()
	targets := make([]*Conn, 0, len(s.conns))
	for c := range s.conns {
		targets = append(targets, c)
	}
	s.mu.Unlock()
	for _, c := range targets {
		if c == nil {
			continue
		}
		_ = c.Send(&protocol.Remote_NetClient_kick_22{Reason: reason})
		s.closeConnLater(c, delay)
	}
	return len(targets)
}

func (s *Server) closeConnLater(c *Conn, delay time.Duration) {
	if c == nil {
		return
	}
	if delay < 0 {
		delay = 0
	}
	time.AfterFunc(delay, func() {
		_ = c.Close()
	})
}

func (s *Server) Shutdown() {
	if s == nil {
		return
	}
	s.shuttingDown.Store(true)

	if s.tcpLn != nil {
		_ = s.tcpLn.Close()
	}
	if s.udpConn != nil {
		_ = s.udpConn.Close()
	}

	s.mu.Lock()
	targets := make([]*Conn, 0, len(s.conns)+len(s.pending))
	for c := range s.conns {
		targets = append(targets, c)
	}
	for _, c := range s.pending {
		targets = append(targets, c)
	}
	s.mu.Unlock()

	for _, c := range targets {
		if c != nil {
			_ = c.Close()
		}
	}
}

func (s *Server) unitControl(c *Conn, unitID int32) {
	if c == nil || c.playerID == 0 || unitID == 0 {
		return
	}
	s.clearConnControlledBuild(c)
	oldUnitID := c.unitID
	// Look up unit info from world when available.
	var info UnitInfo
	if s.UnitInfoFn != nil {
		if v, ok := s.UnitInfoFn(unitID); ok {
			info = v
		} else {
			return
		}
	}

	// Load or create entity sync entry for this unit.
	var u *protocol.UnitEntitySync
	s.entityMu.Lock()
	if ent, ok := s.entities[unitID]; ok {
		if uu, ok2 := ent.(*protocol.UnitEntitySync); ok2 {
			u = uu
		}
	}
	if u == nil {
		if snapshot := s.authoritativeUnitSnapshot(unitID, nil); snapshot != nil {
			u = snapshot
			s.entities[unitID] = u
		} else {
			typeID := info.TypeID
			if !s.validUnitTypeID(typeID) {
				typeID = s.fallbackPlayerUnitTypeID()
			}
			u = &protocol.UnitEntitySync{
				IDValue:        unitID,
				Abilities:      []protocol.Ability{},
				Controller:     nil,
				Elevation:      0,
				Flag:           0,
				Health:         info.Health,
				Shooting:       false,
				Mounts:         []protocol.WeaponMount{},
				Plans:          []*protocol.BuildPlan{},
				Rotation:       0,
				Shield:         0,
				SpawnedByCore:  false,
				Stack:          protocol.ItemStack{Item: protocol.ItemRef{ItmID: 0, ItmName: ""}, Amount: 0},
				Statuses:       []protocol.StatusEntry{},
				TeamID:         info.TeamID,
				TypeID:         typeID,
				UpdateBuilding: false,
				Vel:            protocol.Vec2{X: 0, Y: 0},
				X:              info.X,
				Y:              info.Y,
			}
			s.entities[unitID] = u
		}
	}
	// Basic validation: same team and reasonably close.
	if info.TeamID != 0 && info.TeamID != c.TeamID() {
		s.entityMu.Unlock()
		return
	}
	if state, ok := u.Controller.(*protocol.ControllerState); ok && state != nil {
		if state.Type == protocol.ControllerPlayer && state.PlayerID != c.playerID {
			s.entityMu.Unlock()
			return
		}
	}
	u.Controller = &protocol.ControllerState{Type: protocol.ControllerPlayer, PlayerID: c.playerID}
	if normalized := s.normalizedUnitTypeID(u.TypeID, u.Controller); normalized > 0 {
		u.TypeID = normalized
	} else {
		delete(s.entities, unitID)
		s.entityMu.Unlock()
		return
	}
	s.entityMu.Unlock()

	if s.SetUnitPlayerControllerFn != nil && !s.SetUnitPlayerControllerFn(unitID, c.playerID) {
		return
	}
	if oldUnitID != 0 && oldUnitID != unitID {
		s.detachConnUnit(c, oldUnitID)
	}

	c.unitID = unitID
	c.snapX = info.X
	c.snapY = info.Y
	if info.TeamID != 0 {
		c.teamID = info.TeamID
	}
	c.dead = false
	c.deathTimer = 0
	c.lastRespawnCheck = time.Time{}
	c.lastDeadIgnoreAt = time.Time{}
	c.clientDeadIgnores = 0
	c.miningTilePos = invalidTilePos

	var unit protocol.UnitSyncEntity
	if snapshot := s.playerUnitEntity(c); snapshot != nil && s.prepareUnitEntitySnapshot(snapshot) {
		unit = snapshot
	}
	var player protocol.UnitSyncEntity
	if p := s.ensurePlayerEntity(c); p != nil {
		s.updatePlayerEntity(p, c)
		player = p
	}
	_ = s.sendEntitySnapshotToConn(c, player, unit)
	_ = c.SendAsync(&protocol.Remote_NetClient_setPosition_29{X: c.snapX, Y: c.snapY})
}

func extractUnitID(obj any) int32 {
	switch v := obj.(type) {
	case nil:
		return 0
	case protocol.UnitBox:
		return v.ID()
	case *protocol.EntityBox:
		if v == nil {
			return 0
		}
		return v.ID()
	case protocol.UnitSyncEntity:
		if v == nil {
			return 0
		}
		return v.ID()
	case *protocol.UnitEntitySync:
		if v == nil {
			return 0
		}
		return v.ID()
	default:
		return 0
	}
}

// IsAdmin 检查连接是否是管理员。
func (c *Conn) IsAdmin() bool {
	return c.isAdmin()
}

// isAdmin 检查连接是否是管理员
func (c *Conn) isAdmin() bool {
	if c == nil {
		return false
	}
	// 检查UUID是否在操作员列表中
	server := globalServer
	if server != nil {
		return server.AdminManager != nil && server.AdminManager.IsOp(c.uuid)
	}
	return false
}

// SendChat 发送聊天消息
func (c *Conn) SendChat(message string) error {
	if c == nil {
		return fmt.Errorf("connection is nil")
	}
	return c.SendAsync(makeSendMessagePacket(message, nil))
}

// playerName 返回玩家名称
func (c *Conn) playerName() string {
	if c == nil {
		return ""
	}
	return c.Name()
}
