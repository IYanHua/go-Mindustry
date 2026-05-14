package net

import (
	"fmt"
	"strings"
	"time"

	"github.com/IYanHua/mdt-server/internal/devlog"
	"github.com/IYanHua/mdt-server/internal/protocol"
)

func buildStateSnapshot(s *Server, c *Conn) *protocol.Remote_NetClient_stateSnapshot_35 {
	if s != nil && s.StateSnapshotFn != nil {
		if snap := s.StateSnapshotFn(); snap != nil {
			if len(snap.CoreData) == 0 {
				snap.CoreData = []byte{0}
			}
			return snap
		}
	}
	// Minimal legal state snapshot payload:
	// coreData starts with "teams" byte (0 teams).
	return &protocol.Remote_NetClient_stateSnapshot_35{
		WaveTime: 0,
		Wave:     1,
		Enemies:  0,
		Paused:   false,
		GameOver: false,
		TimeData: int32(time.Now().Unix()),
		Tps:      60,
		Rand0:    time.Now().UnixNano(),
		Rand1:    int64(c.id),
		CoreData: []byte{0},
	}
}

func (s *Server) sendUnreliable(c *Conn, obj any) error {
	if c == nil {
		return nil
	}
	if s.udpConn != nil {
		if addr := c.UDPAddr(); addr != nil {
			c.udpSendMu.Lock()
			defer c.udpSendMu.Unlock()
			payload, packetID, frameworkID, err := c.Encode(obj)
			if err != nil {
				c.udpErrors.Add(1)
				fmt.Printf("[net] sendUnreliable encode failed id=%d err=%v obj=%T\n", c.id, err, obj)
				return err
			}
			if packetID >= 0 && c.sendCount.Load() < 80 {
				s.verbosef("[net] tx-udp id=%d packet_id=%d type=%T len=%d\n", c.id, packetID, obj, len(payload))
			}
			retries := s.UdpRetryCount
			delay := s.UdpRetryDelay
			if retries < 0 {
				retries = 0
			}
			if delay <= 0 {
				delay = 5 * time.Millisecond
			}
			for attempt := 0; attempt <= retries; attempt++ {
				_, err = s.udpConn.WriteToUDP(payload, addr)
				if err == nil {
					if c.onSend != nil {
						c.onSend(obj, packetID, frameworkID, len(payload))
					}
					c.udpSent.Add(1)
					c.bytesSent.Add(int64(len(payload)))
					c.recordSend(obj, int64(len(payload)))
					return nil
				}
				c.udpErrors.Add(1)
				if attempt < retries {
					time.Sleep(delay)
				}
			}
		}
	}
	if s.UdpFallbackTCP {
		if s.udpConn != nil && c.UDPAddr() == nil {
			fmt.Printf("[net] sendUnreliable tcp fallback id=%d obj=%T reason=no_udp_addr\n", c.id, obj)
		}
		return c.Send(obj)
	}
	return nil
}

func (s *Server) sendPlayerSpawn(c *Conn) bool {
	if c == nil || c.playerID == 0 || (s.SpawnTileFn == nil && s.SpawnTileForConnFn == nil) {
		if s.DevLogger != nil {
			s.DevLogger.LogConnection("sendPlayerSpawn skipped", c.id, c.remoteIP(), c.name, c.uuid,
				devlog.BoolFld("has_player", c.playerID != 0),
				devlog.BoolFld("has_spawntilefn", s.SpawnTileFn != nil))
		}
		return false
	}
	pos, ok := s.spawnTileForConn(c)
	if !ok {
		if s.DevLogger != nil {
			s.DevLogger.LogConnection("sendPlayerSpawn no spawn tile", c.id, c.remoteIP(), c.name, c.uuid)
		}
		return false
	}
	return s.sendPlayerSpawnAt(c, pos)
}

func (s *Server) sendPlayerSpawnAt(c *Conn, pos protocol.Point2) bool {
	if c == nil || c.playerID == 0 {
		return false
	}
	tile := protocol.TileBox{PosValue: protocol.PackPoint2(pos.X, pos.Y)}
	player := &protocol.EntityBox{IDValue: c.playerID}
	if s.respawnPacketLogsEnabled.Load() {
		fmt.Printf("[重生] 正在发送玩家出生包: %s 出生点=(%d,%d) playerID=%d\n", s.publicConnField(c), pos.X, pos.Y, c.playerID)
	}
	if err := c.Send(&protocol.Remote_CoreBlock_playerSpawn_149{Tile: tile, Player: player}); err != nil {
		if s.respawnPacketLogsEnabled.Load() {
			fmt.Printf("[重生] 玩家出生包发送失败: %s error=%v\n", s.publicConnField(c), err)
		}
		if s.DevLogger != nil {
			s.DevLogger.LogConnection("sendPlayerSpawn send failed", c.id, c.remoteIP(), c.name, c.uuid,
				devlog.StringFld("error", err.Error()))
		}
		return false
	}
	if s.respawnPacketLogsEnabled.Load() {
		fmt.Printf("[重生] 玩家出生包发送完成: %s\n", s.publicConnField(c))
	}
	return true
}

func tileCenterWorld(pos protocol.Point2) (float32, float32) {
	return float32(pos.X*8 + 4), float32(pos.Y*8 + 4)
}

func (s *Server) prepareInitialConnectState(c *Conn) {
	if s == nil || c == nil || c.playerID == 0 {
		return
	}
	s.assignConnTeam(c, false)
	s.clearConnControlledBuild(c)
	if c.unitID != 0 {
		s.dropPlayerUnitEntity(c, c.unitID)
		c.unitID = 0
	}
	if pos, ok := s.spawnTileForConn(c); ok {
		c.snapX, c.snapY = tileCenterWorld(pos)
	}
	c.dead = true
	c.deathTimer = 0
	c.syncTime.Store(0)
	c.lastRespawnCheck = time.Now()
	c.lastRespawnReq = time.Time{}
	c.lastSpawnAt = time.Time{}
	c.lastSpawnRepairAt = time.Time{}
	c.lastDeadIgnoreAt = time.Time{}
	c.clientDeadIgnores = 0
	c.endRespawnChain("")
	c.miningTilePos = invalidTilePos
	if p := s.ensurePlayerEntity(c); p != nil {
		s.updatePlayerEntity(p, c)
	}
}

func (s *Server) sendInitialPlayerSnapshot(c *Conn) {
	if s == nil || c == nil || c.playerID == 0 || c.serial == nil {
		return
	}
	if p := s.ensurePlayerEntity(c); p != nil {
		s.updatePlayerEntity(p, c)
		_ = s.sendEntitySnapshotToConn(c, p)
	}
}

func (s *Server) initialConnectRespawnDelay() time.Duration {
	if s == nil {
		return 350 * time.Millisecond
	}
	delay := time.Duration(s.entitySnapshotIntervalNs.Load()) * 2
	if delay < 350*time.Millisecond {
		delay = 350 * time.Millisecond
	}
	if delay > time.Second {
		delay = time.Second
	}
	return delay
}

func (s *Server) scheduleInitialConnectRespawn(c *Conn) {
	if s == nil || c == nil || c.playerID == 0 {
		return
	}
	time.AfterFunc(s.initialConnectRespawnDelay(), func() {
		if c == nil {
			return
		}
		select {
		case <-c.closed:
			return
		default:
		}
		if !c.beginRespawnChain("initial-connect") {
			return
		}
		defer c.endRespawnChain("initial-connect")
		now := time.Now()
		if !c.hasConnected || c.playerID == 0 || !c.dead || c.unitID != 0 || c.controlBuildActive {
			return
		}
		if s.connUnitAlive(c) {
			return
		}
		if !c.lastSpawnAt.IsZero() && now.Sub(c.lastSpawnAt) >= 0 && now.Sub(c.lastSpawnAt) < 2*time.Second {
			return
		}
		if s.spawnRespawnUnit(c) {
			s.finishRespawn(c)
			s.sendImmediateAliveSync(c, "initial-connect")
			fmt.Printf("[net] respawn sent conn=%d chain=initial-connect\n", c.id)
		} else {
			fmt.Printf("[net] respawn skipped conn=%d chain=initial-connect reason=no-spawn-tile\n", c.id)
		}
	})
}

func (s *Server) playerRespawnUnitType(c *Conn, pos protocol.Point2) int16 {
	playerTypeID := int16(1)
	if fallback := s.fallbackPlayerUnitTypeID(); fallback > 0 {
		playerTypeID = fallback
	} else if s.PlayerUnitTypeFn != nil {
		if t := s.PlayerUnitTypeFn(); t > 0 {
			playerTypeID = t
		}
	}
	if s.ResolveRespawnUnitTypeFn != nil {
		if resolved := s.ResolveRespawnUnitTypeFn(c, pos, playerTypeID); s.validUnitTypeID(resolved) {
			return resolved
		}
	}
	return playerTypeID
}

func (s *Server) isCoreDockRespawnUnitType(typeID int16) bool {
	if !s.validUnitTypeID(typeID) || s.Content == nil {
		return false
	}
	unit := s.Content.UnitType(typeID)
	if unit == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(unit.Name())) {
	case "evoke", "incite", "emanate":
		return true
	default:
		return false
	}
}

func (s *Server) currentControlledBuildInfo(c *Conn) (ControlledBuildInfo, bool) {
	if s == nil || c == nil || c.playerID == 0 || !c.controlBuildActive || s.ControlledBuildInfoFn == nil {
		return ControlledBuildInfo{}, false
	}
	info, ok := s.ControlledBuildInfoFn(c.playerID, c.controlBuildPos)
	if !ok {
		return ControlledBuildInfo{}, false
	}
	return info, true
}

func (s *Server) clearConnControlledBuild(c *Conn) bool {
	if s == nil || c == nil || c.playerID == 0 || !c.controlBuildActive {
		return false
	}
	if s.ReleaseControlledBuildFn != nil {
		_ = s.ReleaseControlledBuildFn(c.playerID, c.controlBuildPos)
	}
	c.controlBuildPos = 0
	c.controlBuildActive = false
	return true
}

func (s *Server) controlBlockUnit(c *Conn, buildPos int32) bool {
	if s == nil || c == nil || c.playerID == 0 || c.dead || buildPos == 0 || s.ClaimControlledBuildFn == nil {
		return false
	}
	info, ok := s.ClaimControlledBuildFn(c.playerID, buildPos)
	if !ok {
		return false
	}
	if c.unitID != 0 {
		s.detachConnUnit(c, c.unitID)
		c.unitID = 0
	}
	if c.controlBuildActive && c.controlBuildPos != buildPos {
		s.clearConnControlledBuild(c)
	}
	c.controlBuildPos = info.Pos
	c.controlBuildActive = true
	c.snapX = info.X
	c.snapY = info.Y
	if info.TeamID != 0 {
		c.teamID = info.TeamID
	}
	c.dead = false
	c.deathTimer = 0
	c.lastRespawnCheck = time.Time{}
	c.miningTilePos = invalidTilePos
	if s.SetControlledBuildInputFn != nil {
		_ = s.SetControlledBuildInputFn(c.playerID, c.controlBuildPos, c.pointerX, c.pointerY, c.shooting)
	}
	if p := s.ensurePlayerEntity(c); p != nil {
		s.updatePlayerEntity(p, c)
		if c.serial != nil {
			s.sendEntitySnapshotToConn(c, p)
		}
	}
	return true
}

func (s *Server) currentPlayerUnitState(c *Conn) (*protocol.UnitEntitySync, bool) {
	if s == nil || c == nil || c.playerID == 0 || c.unitID == 0 || c.controlBuildActive {
		return nil, false
	}
	return s.playerControlledUnitState(c, c.unitID)
}

func (s *Server) playerControlledUnitState(c *Conn, unitID int32) (*protocol.UnitEntitySync, bool) {
	if s == nil || c == nil || c.playerID == 0 || unitID == 0 {
		return nil, false
	}
	s.entityMu.Lock()
	defer s.entityMu.Unlock()
	ent, ok := s.entities[unitID]
	if !ok {
		return nil, false
	}
	unit, ok := ent.(*protocol.UnitEntitySync)
	if !ok || unit == nil {
		return nil, false
	}
	state, ok := unit.Controller.(*protocol.ControllerState)
	if !ok || state == nil || state.Type != protocol.ControllerPlayer || state.PlayerID != c.playerID {
		return nil, false
	}
	copy := *unit
	return &copy, true
}

func (s *Server) detachConnUnit(c *Conn, unitID int32) bool {
	if s == nil || c == nil || c.playerID == 0 || unitID == 0 {
		return false
	}
	if unit, ok := s.playerControlledUnitState(c, unitID); ok && unit != nil && unit.SpawnedByCore {
		s.dropPlayerUnitEntity(c, unitID)
		if c.unitID == unitID {
			c.unitID = 0
		}
		return true
	}
	return s.releaseConnUnitControl(c, unitID)
}

func (s *Server) releaseConnUnitControl(c *Conn, unitID int32) bool {
	if s == nil || c == nil || c.playerID == 0 || unitID == 0 {
		return false
	}
	if s.SetUnitPlayerControllerFn != nil {
		_ = s.SetUnitPlayerControllerFn(unitID, 0)
	}
	s.entityMu.Lock()
	if ent, ok := s.entities[unitID]; ok {
		if unit, ok := ent.(*protocol.UnitEntitySync); ok && unit != nil {
			if state, ok := unit.Controller.(*protocol.ControllerState); ok && state != nil &&
				state.Type == protocol.ControllerPlayer && state.PlayerID == c.playerID {
				unit.Controller = &protocol.ControllerState{Type: protocol.ControllerGenericAI}
				unit.UpdateBuilding = false
			}
		}
	}
	s.entityMu.Unlock()
	if c.unitID == unitID {
		c.unitID = 0
	}
	return true
}

func (s *Server) consumeConnUnit(c *Conn, unitID int32) bool {
	if s == nil || c == nil || c.playerID == 0 || unitID == 0 {
		return false
	}
	s.entityMu.Lock()
	if ent, ok := s.entities[unitID]; ok {
		if unit, ok := ent.(*protocol.UnitEntitySync); ok && unit != nil {
			if state, ok := unit.Controller.(*protocol.ControllerState); ok && state != nil &&
				state.Type == protocol.ControllerPlayer && state.PlayerID == c.playerID {
				delete(s.entities, unitID)
			}
		}
	}
	s.entityMu.Unlock()
	if c.unitID == unitID {
		c.unitID = 0
	}
	return true
}

func (s *Server) ReleaseConnUnitControl(c *Conn) bool {
	if s == nil || c == nil {
		return false
	}
	if c.controlBuildActive {
		return s.clearConnControlledBuild(c)
	}
	return s.releaseConnUnitControl(c, c.unitID)
}

func (s *Server) ConsumeConnUnit(c *Conn, unitID int32) bool {
	return s.consumeConnUnit(c, unitID)
}

func (s *Server) HandleCoreBuildingControlSelect(c *Conn, pos protocol.Point2) bool {
	if s == nil || c == nil || c.playerID == 0 || c.dead {
		return false
	}

	s.clearConnControlledBuild(c)
	oldUnitID := c.unitID
	if oldUnitID != 0 {
		s.detachConnUnit(c, oldUnitID)
	}

	playerTypeID := s.playerRespawnUnitType(c, pos)
	if s.SpawnUnitFn != nil {
		c.lastRespawnReq = time.Now()
		c.unitID = s.nextUnitID()
		if x, y, ok := s.SpawnUnitFn(c, c.unitID, pos, playerTypeID); ok {
			c.snapX = x
			c.snapY = y
			c.lastSpawnAt = time.Now()
			c.lastSpawnRepairAt = time.Time{}
			c.lastDeadIgnoreAt = time.Time{}
			c.clientDeadIgnores = 0
			c.miningTilePos = invalidTilePos
		} else {
			c.unitID = 0
			return false
		}
	}
	s.ensurePlayerUnitEntity(c)
	if !s.sendPlayerSpawnAt(c, pos) {
		return false
	}
	c.dead = false
	c.deathTimer = 0
	c.lastRespawnCheck = time.Time{}
	c.lastRespawnReq = time.Now()
	return true
}

func (s *Server) tryDockedUnitClearRespawn(c *Conn, source string) bool {
	if s == nil || c == nil || c.playerID == 0 || c.dead || c.unitID == 0 || s.SpawnUnitAtFn == nil {
		return false
	}
	unit, ok := s.currentPlayerUnitState(c)
	if !ok || unit == nil || unit.SpawnedByCore {
		return false
	}
	pos, ok := s.spawnTileForConn(c)
	if !ok {
		return false
	}
	respawnType := s.playerRespawnUnitType(c, pos)
	if !s.isCoreDockRespawnUnitType(respawnType) {
		return false
	}

	x, y := c.snapX, c.snapY
	if unit.X != 0 || unit.Y != 0 {
		x, y = unit.X, unit.Y
	}
	rotation := unit.Rotation

	oldUnitID := c.unitID
	s.dropPlayerUnitEntity(c, oldUnitID)

	c.lastRespawnReq = time.Now()
	c.unitID = s.nextUnitID()
	sx, sy, spawned := s.SpawnUnitAtFn(c, c.unitID, x, y, rotation, respawnType, true)
	if !spawned {
		c.unitID = 0
		return false
	}

	c.snapX = sx
	c.snapY = sy
	c.dead = false
	c.deathTimer = 0
	c.lastSpawnAt = time.Now()
	c.lastSpawnRepairAt = time.Time{}
	c.lastDeadIgnoreAt = time.Time{}
	c.clientDeadIgnores = 0
	c.lastRespawnCheck = time.Time{}
	c.lastRespawnReq = time.Now()
	c.miningTilePos = invalidTilePos

	if u := s.ensurePlayerUnitEntity(c); u != nil {
		u.SpawnedByCore = true
		u.Rotation = rotation
	}
	if p := s.ensurePlayerEntity(c); p != nil {
		s.updatePlayerEntity(p, c)
	}
	_ = c.SendAsync(&protocol.Remote_NetClient_setPosition_29{X: c.snapX, Y: c.snapY})
	fmt.Printf("[net] dock respawn conn=%d player=%d old_unit=%d new_unit=%d type=%d source=%s pos=(%.1f,%.1f)\n",
		c.id, c.playerID, oldUnitID, c.unitID, respawnType, source, c.snapX, c.snapY)
	return true
}

func (s *Server) spawnPlayerInitial(c *Conn) {
	if c == nil || c.playerID == 0 || (s.SpawnTileFn == nil && s.SpawnTileForConnFn == nil) {
		return
	}
	s.assignConnTeam(c, false)
	pos, ok := s.spawnTileForConn(c)
	if !ok {
		return
	}
	playerTypeID := s.playerRespawnUnitType(c, pos)
	if s.SpawnUnitFn != nil {
		if c.unitID != 0 {
			s.dropPlayerUnitEntity(c, c.unitID)
			c.unitID = 0
		}
		c.lastRespawnReq = time.Now()
		c.unitID = s.nextUnitID()
		if x, y, ok := s.SpawnUnitFn(c, c.unitID, pos, playerTypeID); ok {
			if s.UnitInfoFn != nil {
				if _, exists := s.UnitInfoFn(c.unitID); !exists {
					if s.DropUnitFn != nil {
						s.DropUnitFn(c.unitID)
					}
					c.unitID = 0
					return
				}
			}
			c.snapX = x
			c.snapY = y
			c.lastSpawnAt = time.Now()
			c.lastSpawnRepairAt = time.Time{}
			c.lastDeadIgnoreAt = time.Time{}
			c.clientDeadIgnores = 0
			c.miningTilePos = invalidTilePos
		} else {
			c.unitID = 0
			return
		}
	}
	if c.unitID == 0 {
		return
	}
	s.ensurePlayerUnitEntity(c)
	if !s.sendPlayerSpawnAt(c, pos) {
		s.dropPlayerUnitEntity(c, c.unitID)
		c.unitID = 0
	}
}

func (s *Server) spawnRespawnUnit(c *Conn) bool {
	if c == nil || c.playerID == 0 {
		return false
	}
	if s.SpawnTileFn == nil && s.SpawnTileForConnFn == nil {
		return false
	}
	s.assignConnTeam(c, false)
	pos, ok := s.spawnTileForConn(c)
	if !ok {
		return false
	}
	playerTypeID := s.playerRespawnUnitType(c, pos)
	if s.SpawnUnitFn != nil {
		if c.unitID != 0 {
			s.dropPlayerUnitEntity(c, c.unitID)
			c.unitID = 0
		}
		c.lastRespawnReq = time.Now()
		c.unitID = s.nextUnitID()
		if x, y, ok := s.SpawnUnitFn(c, c.unitID, pos, playerTypeID); ok {
			if s.UnitInfoFn != nil {
				if _, exists := s.UnitInfoFn(c.unitID); !exists {
					if s.DropUnitFn != nil {
						s.DropUnitFn(c.unitID)
					}
					c.unitID = 0
					return false
				}
			}
			c.snapX = x
			c.snapY = y
			c.lastSpawnAt = time.Now()
			c.lastSpawnRepairAt = time.Time{}
			c.lastDeadIgnoreAt = time.Time{}
			c.clientDeadIgnores = 0
			c.miningTilePos = invalidTilePos
		} else {
			c.unitID = 0
			return false
		}
	}
	if c.unitID == 0 {
		return false
	}
	s.ensurePlayerUnitEntity(c)
	if !s.sendPlayerSpawnAt(c, pos) {
		s.dropPlayerUnitEntity(c, c.unitID)
		c.unitID = 0
		return false
	}
	return true
}

func (s *Server) clearConnUnitForRespawn(c *Conn, source string) {
	if s == nil || c == nil || c.playerID == 0 {
		return
	}
	s.clearConnControlledBuild(c)
	oldUnitID := c.unitID
	if oldUnitID == 0 {
		return
	}
	if unit, ok := s.playerControlledUnitState(c, oldUnitID); ok && unit != nil && !unit.SpawnedByCore {
		s.releaseConnUnitControl(c, oldUnitID)
		fmt.Printf("[net] unitClear released controlled unit conn=%d player=%d unit=%d source=%s\n",
			c.id, c.playerID, oldUnitID, source)
		return
	}
	if !s.dropPlayerUnitEntity(c, oldUnitID) && s.DropUnitFn != nil {
		s.DropUnitFn(oldUnitID)
	}
	if c.unitID == oldUnitID {
		c.unitID = 0
	}
}

func (s *Server) markDeadAfterUnitClear(c *Conn, source string) {
	if c == nil || c.playerID == 0 {
		return
	}
	c.dead = true
	c.deathTimer = 0
	c.lastRespawnCheck = time.Now()
	c.lastSpawnAt = time.Time{}
	c.lastSpawnRepairAt = time.Time{}
	c.lastDeadIgnoreAt = time.Time{}
	c.clientDeadIgnores = 0
	c.miningTilePos = invalidTilePos
	fmt.Printf("[net] player unit cleared conn=%d player=%d team=%d source=%s snap=(%.1f,%.1f)\n",
		c.id, c.playerID, c.TeamID(), source, c.snapX, c.snapY)
}

func (s *Server) finishRespawn(c *Conn) {
	if c == nil {
		return
	}
	c.dead = false
	c.deathTimer = 0
	c.lastRespawnCheck = time.Time{}
	c.lastRespawnReq = time.Now()
}

// Official InputHandler.unitClear() chain:
// explicit player respawn request from the 157 client.
func (s *Server) handleOfficialUnitClear(c *Conn) {
	if c == nil || c.playerID == 0 {
		return
	}
	if c.respawnChainInProgress() {
		return
	}
	if c.InWorldReloadGrace() && c.lastSpawnAt.IsZero() && c.unitID == 0 && !c.controlBuildActive {
		return
	}
	now := time.Now()
	if !c.lastRespawnReq.IsZero() && now.Sub(c.lastRespawnReq) < 250*time.Millisecond {
		return
	}
	if !c.lastSpawnAt.IsZero() && now.Sub(c.lastSpawnAt) < 250*time.Millisecond {
		return
	}
	if !c.beginRespawnChain("official-unitClear") {
		return
	}
	defer c.endRespawnChain("official-unitClear")
	if s.connUnitAlive(c) {
		if s.tryDockedUnitClearRespawn(c, "unitClear-91") {
			return
		}
		c.lastRespawnReq = now
		s.clearConnUnitForRespawn(c, "unitClear-91")
		s.markDeadAfterUnitClear(c, "unitClear-91")
		if s.spawnRespawnUnit(c) {
			s.finishRespawn(c)
			s.sendImmediateAliveSync(c, "official-unitClear")
			fmt.Printf("[net] respawn sent conn=%d chain=official-unitClear\n", c.id)
		} else {
			fmt.Printf("[net] respawn skipped conn=%d chain=official-unitClear reason=no-spawn-tile\n", c.id)
		}
		return
	}
	if s.connFreshSpawnWorldBindingMissing(c, now, 2*time.Second) {
		fmt.Printf("[net] stale unitClear ignored conn=%d player=%d unit=%d age=%s\n",
			c.id, c.playerID, c.unitID, now.Sub(c.lastSpawnAt).Round(10*time.Millisecond))
		return
	}
	c.lastRespawnReq = now
	s.markDead(c, "unitClear-91")
	if s.spawnRespawnUnit(c) {
		s.finishRespawn(c)
		s.sendImmediateAliveSync(c, "official-unitClear")
		fmt.Printf("[net] respawn sent conn=%d chain=official-unitClear\n", c.id)
	} else {
		fmt.Printf("[net] respawn skipped conn=%d chain=official-unitClear reason=no-spawn-tile\n", c.id)
	}
}

func (s *Server) spawnTileForConn(c *Conn) (protocol.Point2, bool) {
	if s == nil {
		return protocol.Point2{}, false
	}
	if s.SpawnTileForConnFn != nil {
		if pos, ok := s.SpawnTileForConnFn(c); ok {
			return pos, true
		}
	}
	if s.SpawnTileFn != nil {
		return s.SpawnTileFn()
	}
	return protocol.Point2{}, false
}

func (s *Server) shouldForceRespawnAfterDeadIgnored(c *Conn, now time.Time) bool {
	if s == nil || c == nil || c.playerID == 0 {
		return false
	}
	if !c.lastSpawnRepairAt.IsZero() && (c.lastSpawnAt.IsZero() || !c.lastSpawnAt.After(c.lastSpawnRepairAt)) {
		return false
	}
	if c.lastDeadIgnoreAt.IsZero() || now.Sub(c.lastDeadIgnoreAt) > 1500*time.Millisecond {
		c.clientDeadIgnores = 0
	}
	c.lastDeadIgnoreAt = now
	c.clientDeadIgnores++
	if c.lastSpawnAt.IsZero() {
		return false
	}
	if now.Sub(c.lastSpawnAt) < 500*time.Millisecond {
		return false
	}
	return c.clientDeadIgnores >= 3
}

func recentNonZeroTime(a, b time.Time) time.Time {
	switch {
	case a.IsZero():
		return b
	case b.IsZero():
		return a
	case a.After(b):
		return a
	default:
		return b
	}
}

func (s *Server) connHasRecentRespawnWindow(c *Conn, now time.Time, window time.Duration) bool {
	if c == nil || c.playerID == 0 || window <= 0 {
		return false
	}
	last := recentNonZeroTime(c.lastSpawnAt, c.lastRespawnReq)
	if last.IsZero() {
		return false
	}
	age := now.Sub(last)
	return age >= 0 && age < window
}

func (s *Server) connLiveWorldLoadSettleAge(c *Conn, now time.Time) (time.Duration, bool) {
	if c == nil || c.playerID == 0 || !c.UsesLiveWorldStream() {
		return 0, false
	}
	start := recentNonZeroTime(c.connectTime, recentNonZeroTime(c.lastSpawnAt, c.lastRespawnReq))
	if start.IsZero() {
		return 0, false
	}
	age := now.Sub(start)
	if age < 0 {
		return 0, false
	}
	return age, true
}

func (s *Server) connHasFreshSpawnBinding(c *Conn, now time.Time, window time.Duration) bool {
	if s == nil || c == nil || c.playerID == 0 {
		return false
	}
	if c.controlBuildActive || c.dead || c.unitID == 0 || c.lastSpawnAt.IsZero() {
		return false
	}
	age := now.Sub(c.lastSpawnAt)
	return age >= 0 && age < window
}

func (s *Server) connFreshSpawnWorldBindingMissing(c *Conn, now time.Time, window time.Duration) bool {
	if !s.connHasFreshSpawnBinding(c, now, window) {
		return false
	}
	if c.lastRespawnReq.IsZero() || now.Sub(c.lastRespawnReq) >= window {
		return false
	}
	if s.UnitInfoFn != nil {
		_, ok := s.UnitInfoFn(c.unitID)
		return !ok
	}
	s.entityMu.Lock()
	defer s.entityMu.Unlock()
	_, ok := s.entities[c.unitID]
	return !ok
}

func (s *Server) sendEntitySnapshotToConn(c *Conn, entities ...protocol.UnitSyncEntity) bool {
	if s == nil || c == nil {
		return false
	}

	var data []byte
	var amount int16
	appendEntity := func(entity protocol.UnitSyncEntity) {
		if entity == nil {
			return
		}
		entry := s.serializeEntity(entity)
		if len(entry) == 0 {
			return
		}
		data = append(data, entry...)
		amount++
	}
	for _, entity := range entities {
		if entity == nil || entity.ClassID() == 12 {
			continue
		}
		appendEntity(entity)
	}
	for _, entity := range entities {
		if entity == nil || entity.ClassID() != 12 {
			continue
		}
		appendEntity(entity)
	}
	if amount == 0 {
		return false
	}

	if err := s.sendUnreliable(c, &protocol.Remote_NetClient_entitySnapshot_32{
		Amount: amount,
		Data:   data,
	}); err != nil {
		fmt.Printf("[net] alive entity snapshot send failed conn=%d player=%d err=%v\n", c.id, c.playerID, err)
		return false
	}
	return true
}

func (s *Server) repairAliveSpawnBinding(c *Conn, reason string, allowRepeat bool) {
	if s == nil || c == nil || c.playerID == 0 || c.unitID == 0 {
		return
	}
	if c.InWorldReloadGrace() || !s.connUnitAlive(c) {
		return
	}
	now := time.Now()
	if !c.lastSpawnRepairAt.IsZero() {
		if !allowRepeat {
			if c.lastSpawnAt.IsZero() || !c.lastSpawnAt.After(c.lastSpawnRepairAt) {
				return
			}
		} else if now.Sub(c.lastSpawnRepairAt) < 250*time.Millisecond {
			return
		}
	}
	var unit protocol.UnitSyncEntity
	if u := s.ensurePlayerUnitEntity(c); u != nil {
		s.syncUnitFromWorld(u)
		c.snapX = u.X
		c.snapY = u.Y
		if s.prepareUnitEntitySnapshot(u) {
			unit = u
		}
	}
	var player protocol.UnitSyncEntity
	if p := s.ensurePlayerEntity(c); p != nil {
		s.updatePlayerEntity(p, c)
		player = p
	}
	c.dead = false
	c.deathTimer = 0
	c.lastRespawnCheck = now
	c.lastSpawnRepairAt = now
	c.clientDeadIgnores = 0
	c.lastDeadIgnoreAt = time.Time{}
	snapshotSent := false
	if c.serial != nil {
		snapshotSent = s.sendEntitySnapshotToConn(c, player, unit)
		_ = c.SendAsync(&protocol.Remote_NetClient_setPosition_29{X: c.snapX, Y: c.snapY})
	}
	fmt.Printf("[net] alive binding synced conn=%d player=%d unit=%d source=%s snapshot=%v pos=(%.1f,%.1f)\n",
		c.id, c.playerID, c.unitID, reason, snapshotSent, c.snapX, c.snapY)
}

func (s *Server) sendImmediateAliveSync(c *Conn, source string) {
	if s == nil || c == nil || c.playerID == 0 {
		return
	}
	var unit protocol.UnitSyncEntity
	if u := s.snapshotPlayerUnitEntity(c); u != nil && s.prepareUnitEntitySnapshot(u) {
		unit = u
		c.snapX = u.X
		c.snapY = u.Y
	}
	var player protocol.UnitSyncEntity
	if p := s.ensurePlayerEntity(c); p != nil {
		s.updatePlayerEntity(p, c)
		player = p
	}
	snapshotSent := false
	if c.serial != nil {
		snapshotSent = s.sendEntitySnapshotToConn(c, unit, player)
		_ = c.SendAsync(&protocol.Remote_NetClient_setPosition_29{X: c.snapX, Y: c.snapY})
	}
	fmt.Printf("[net] alive binding synced conn=%d player=%d unit=%d source=%s snapshot=%v pos=(%.1f,%.1f)\n",
		c.id, c.playerID, c.unitID, source, snapshotSent, c.snapX, c.snapY)
}

func (s *Server) repairClientDeadAliveBinding(c *Conn) {
	s.repairAliveSpawnBinding(c, "client-dead-ignored", false)
}

func (s *Server) repairClientDeadStuckBinding(c *Conn) {
	s.repairAliveSpawnBinding(c, "client-dead-stuck", true)
}

func (s *Server) repairOfficialUnitClearAliveBinding(c *Conn) {
	s.repairAliveSpawnBinding(c, "official-unitClear-alive", true)
}

func (s *Server) handleDeathTimerRespawn(c *Conn) {
	if c == nil || c.playerID == 0 {
		return
	}
	if !c.beginRespawnChain("death-timer") {
		return
	}
	defer c.endRespawnChain("death-timer")
	if s.spawnRespawnUnit(c) {
		s.finishRespawn(c)
		s.sendImmediateAliveSync(c, "death-timer")
		fmt.Printf("[net] respawn sent conn=%d chain=death-timer\n", c.id)
	} else {
		fmt.Printf("[net] respawn skipped conn=%d chain=death-timer reason=no-spawn-tile\n", c.id)
	}
}

func (s *Server) handleMapHotReloadRespawn(c *Conn) {
	if c == nil || c.playerID == 0 {
		return
	}
	if !c.beginRespawnChain("map-hot-reload") {
		return
	}
	defer c.endRespawnChain("map-hot-reload")
	if s.spawnRespawnUnit(c) {
		s.finishRespawn(c)
		s.sendImmediateAliveSync(c, "map-hot-reload")
		fmt.Printf("[net] respawn sent conn=%d chain=map-hot-reload\n", c.id)
	} else {
		fmt.Printf("[net] respawn skipped conn=%d chain=map-hot-reload reason=no-spawn-tile\n", c.id)
	}
}

func (s *Server) respawnDelayFrames() float32 {
	if s == nil || s.RespawnDelayFrames <= 0 {
		return 60
	}
	return s.RespawnDelayFrames
}

func (s *Server) markDead(c *Conn, source string) {
	if c == nil || c.playerID == 0 {
		return
	}
	s.clearConnControlledBuild(c)
	if c.unitID != 0 {
		s.dropPlayerUnitEntity(c, c.unitID)
		c.unitID = 0
	}
	if !c.dead {
		c.dead = true
		c.deathTimer = 0
		c.lastRespawnCheck = time.Now()
		c.lastSpawnAt = time.Time{}
		c.lastSpawnRepairAt = time.Time{}
		c.lastDeadIgnoreAt = time.Time{}
		c.clientDeadIgnores = 0
		c.miningTilePos = invalidTilePos
	}
	fmt.Printf("[net] player dead conn=%d player=%d team=%d source=%s snap=(%.1f,%.1f)\n",
		c.id, c.playerID, c.TeamID(), source, c.snapX, c.snapY)
	if s.DevLogger != nil {
		s.DevLogger.LogConnection("player_dead", c.id, c.remoteIP(), c.name, c.uuid,
			devlog.StringFld("source", source))
	}
}

func (s *Server) maybeRespawn(c *Conn) {
	if c == nil || c.playerID == 0 {
		return
	}
	if c.respawnChainInProgress() {
		return
	}
	if c.InWorldReloadGrace() {
		return
	}
	if c.dead && s.connUnitAlive(c) {
		c.dead = false
		c.deathTimer = 0
		c.lastRespawnCheck = time.Now()
		return
	}
	if !c.dead {
		// Do not self-mark living players as dead from polling. Actual deaths are
		// already signalled by explicit world/client events; mirror gaps here would
		// otherwise create false death-timer respawn loops.
		if c.unitID != 0 {
			if u := s.playerUnitEntity(c); u != nil {
				s.syncUnitFromWorld(u)
			} else if s.connUnitAlive(c) {
				if u := s.ensurePlayerUnitEntity(c); u != nil {
					s.syncUnitFromWorld(u)
				}
			}
		}
		c.deathTimer = 0
		c.lastRespawnCheck = time.Now()
		return
	}
	if _, ok := s.spawnTileForConn(c); !ok {
		c.lastRespawnCheck = time.Now()
		return
	}
	now := time.Now()
	if c.lastRespawnCheck.IsZero() {
		c.lastRespawnCheck = now
		return
	}
	dt := now.Sub(c.lastRespawnCheck).Seconds()
	if dt <= 0 {
		c.lastRespawnCheck = now
		return
	}
	c.lastRespawnCheck = now
	// Mindustry Time.delta is scaled to 60 FPS; deathDelay=60f == ~1s.
	c.deathTimer += float32(dt) * 60
	if c.deathTimer >= s.respawnDelayFrames() {
		c.deathTimer = 0
		s.handleDeathTimerRespawn(c)
	}
}

func (s *Server) connUnitAlive(c *Conn) bool {
	if s == nil || c == nil || c.playerID == 0 {
		return false
	}
	if c.controlBuildActive {
		_, ok := s.currentControlledBuildInfo(c)
		return ok
	}
	if c.unitID == 0 {
		return false
	}
	if s.UnitInfoFn != nil {
		if info, ok := s.UnitInfoFn(c.unitID); ok {
			return info.Health > 0
		}
		return false
	}
	s.entityMu.Lock()
	defer s.entityMu.Unlock()
	ent, ok := s.entities[c.unitID]
	if !ok {
		return false
	}
	if u, ok := ent.(*protocol.UnitEntitySync); ok {
		return u.Health > 0
	}
	return true
}

func (s *Server) MarkUnitDead(unitID int32, source string) {
	if s == nil || unitID == 0 {
		return
	}
	s.mu.Lock()
	conns := make([]*Conn, 0, len(s.conns))
	for c := range s.conns {
		conns = append(conns, c)
	}
	s.mu.Unlock()
	for _, c := range conns {
		if c.unitID == unitID {
			s.markDead(c, source)
			return
		}
	}
}

