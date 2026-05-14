package net

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/IYanHua/mdt-server/internal/protocol"
	"github.com/IYanHua/mdt-server/internal/runtimeassets"
	"github.com/IYanHua/mdt-server/internal/worldstream"
)

func (s *Server) buildPlayerEntitySnapshot() (int16, []byte, error) {
	packets, _, err := s.buildEntitySnapshotPacketsForConn(nil)
	if err != nil {
		return 0, nil, err
	}
	if len(packets) == 0 || packets[0] == nil {
		return 0, nil, nil
	}
	return packets[0].Amount, packets[0].Data, nil
}

func (s *Server) buildEntitySnapshotPackets() ([]*protocol.Remote_NetClient_entitySnapshot_32, error) {
	packets, _, err := s.buildEntitySnapshotPacketsForConn(nil)
	return packets, err
}

func (s *Server) entitySnapshotHidden(viewer *Conn, entity protocol.UnitSyncEntity) bool {
	if s == nil || viewer == nil || entity == nil || s.EntitySnapshotHiddenFn == nil {
		return false
	}
	return s.EntitySnapshotHiddenFn(viewer, entity)
}

func appendHiddenEntityID(hiddenIDs *[]int32, hiddenSet map[int32]struct{}, entityID int32) {
	if entityID == 0 {
		return
	}
	if _, ok := hiddenSet[entityID]; ok {
		return
	}
	hiddenSet[entityID] = struct{}{}
	*hiddenIDs = append(*hiddenIDs, entityID)
}

const maxEntitySnapshotData = 32000

func (s *Server) buildEntitySnapshotPacketsForConn(viewer *Conn) ([]*protocol.Remote_NetClient_entitySnapshot_32, []int32, error) {
	base, err := s.buildEntitySnapshotBaseFresh()
	if err != nil {
		return nil, nil, err
	}
	return s.materializeEntitySnapshotPackets(viewer, base)
}

func (s *Server) buildEntitySnapshotPacketsForConnCached(viewer *Conn) ([]*protocol.Remote_NetClient_entitySnapshot_32, []int32, error) {
	base, err := s.getEntitySnapshotBaseCached()
	if err != nil {
		return nil, nil, err
	}
	start := time.Now()
	packets, hiddenIDs, err := s.materializeEntitySnapshotPackets(viewer, base)
	s.entitySnapView.Store(int64(time.Since(start)))
	return packets, hiddenIDs, err
}

func (s *Server) clearEntitySnapshotCache() {
	if s == nil {
		return
	}
	s.entitySnapMu.Lock()
	s.entitySnapCache = entitySnapshotBase{}
	s.entitySnapMu.Unlock()
}

func (s *Server) EntitySnapshotCacheStats() EntitySnapshotCacheStats {
	if s == nil {
		return EntitySnapshotCacheStats{}
	}
	return EntitySnapshotCacheStats{
		Hits:               s.entitySnapHits.Load(),
		Misses:             s.entitySnapMiss.Load(),
		LastBuildDuration:  time.Duration(s.entitySnapBuild.Load()),
		LastFilterDuration: time.Duration(s.entitySnapView.Load()),
	}
}

func (s *Server) getEntitySnapshotBaseCached() (entitySnapshotBase, error) {
	if s == nil {
		return entitySnapshotBase{}, nil
	}
	ttl := s.syncInterval()
	if ttl <= 0 {
		ttl = 200 * time.Millisecond
	}
	now := time.Now()

	s.entitySnapMu.RLock()
	cached := s.entitySnapCache
	if !cached.builtAt.IsZero() && now.Sub(cached.builtAt) < ttl {
		s.entitySnapMu.RUnlock()
		s.entitySnapHits.Add(1)
		return cached, nil
	}
	s.entitySnapMu.RUnlock()

	s.entitySnapMu.Lock()
	defer s.entitySnapMu.Unlock()
	cached = s.entitySnapCache
	if !cached.builtAt.IsZero() && now.Sub(cached.builtAt) < ttl {
		s.entitySnapHits.Add(1)
		return cached, nil
	}
	start := time.Now()
	built, err := s.buildEntitySnapshotBaseFresh()
	if err != nil {
		return entitySnapshotBase{}, err
	}
	built.builtAt = now
	s.entitySnapCache = built
	s.entitySnapMiss.Add(1)
	s.entitySnapBuild.Store(int64(time.Since(start)))
	return built, nil
}

func (s *Server) buildEntitySnapshotBaseFresh() (entitySnapshotBase, error) {
	players := s.connectedSnapshotPlayers()
	base := entitySnapshotBase{
		players:      make([]entitySnapshotPlayerBase, 0, len(players)),
		extraEntries: make([]entitySnapshotBaseEntry, 0, 8),
	}
	for _, p := range players {
		playerBase, ok, err := s.buildPlayerSnapshotBase(p)
		if err != nil {
			return entitySnapshotBase{}, err
		}
		if ok {
			base.players = append(base.players, playerBase)
		}
	}

	if s.ExtraEntitySnapshotEntitiesFn != nil {
		extraEntities, err := s.ExtraEntitySnapshotEntitiesFn()
		if err != nil {
			return entitySnapshotBase{}, err
		}
		filtered := make([]protocol.UnitSyncEntity, 0, len(extraEntities))
		for _, ent := range extraEntities {
			if ent != nil {
				filtered = append(filtered, ent)
			}
		}
		sort.Slice(filtered, func(i, j int) bool {
			if filtered[i].ID() != filtered[j].ID() {
				return filtered[i].ID() < filtered[j].ID()
			}
			return filtered[i].ClassID() < filtered[j].ClassID()
		})
		for _, ent := range filtered {
			snapshot := ent
			if typed, ok := ent.(*protocol.UnitEntitySync); ok {
				unit := cloneUnitEntitySync(typed)
				if unit == nil || !s.prepareUnitEntitySnapshot(unit) {
					continue
				}
				snapshot = unit
			}
			entry, err := s.encodeEntitySnapshotEntry(snapshot)
			if err != nil {
				return entitySnapshotBase{}, err
			}
			base.extraEntries = append(base.extraEntries, entitySnapshotBaseEntry{
				entity:  snapshot,
				encoded: entry,
			})
		}
	}

	if s.ExtraEntitySnapshotFn != nil {
		legacyWriter := protocol.NewWriterWithContext(s.TypeIO)
		n, err := s.ExtraEntitySnapshotFn(legacyWriter)
		if err != nil {
			return entitySnapshotBase{}, err
		}
		if n > 0 || len(legacyWriter.Bytes()) > 0 {
			if len(legacyWriter.Bytes()) > maxEntitySnapshotData {
				return entitySnapshotBase{}, fmt.Errorf("legacy entity snapshot payload too large: %d", len(legacyWriter.Bytes()))
			}
			base.legacyPackets = append(base.legacyPackets, &protocol.Remote_NetClient_entitySnapshot_32{
				Amount: n,
				Data:   append([]byte(nil), legacyWriter.Bytes()...),
			})
		}
	}
	return base, nil
}

func (s *Server) materializeEntitySnapshotPackets(viewer *Conn, base entitySnapshotBase) ([]*protocol.Remote_NetClient_entitySnapshot_32, []int32, error) {
	writer := protocol.NewWriterWithContext(s.TypeIO)
	packets := make([]*protocol.Remote_NetClient_entitySnapshot_32, 0, 4+len(base.legacyPackets))
	hiddenIDs := make([]int32, 0, 8)
	hiddenSet := map[int32]struct{}{}
	var sent int16
	flush := func() {
		if sent <= 0 {
			return
		}
		packets = append(packets, &protocol.Remote_NetClient_entitySnapshot_32{
			Amount: sent,
			Data:   append([]byte(nil), writer.Bytes()...),
		})
		writer = protocol.NewWriterWithContext(s.TypeIO)
		sent = 0
	}
	appendEntry := func(entry []byte) error {
		if len(entry) == 0 {
			return nil
		}
		if len(entry) > maxEntitySnapshotData {
			return fmt.Errorf("entity snapshot entry too large: %d", len(entry))
		}
		if sent > 0 && len(writer.Bytes())+len(entry) > maxEntitySnapshotData {
			flush()
		}
		if err := writer.WriteBytes(entry); err != nil {
			return err
		}
		sent++
		return nil
	}

	for _, player := range base.players {
		unitHidden := false
		if player.unit != nil {
			if s.entitySnapshotHidden(viewer, player.unit) {
				appendHiddenEntityID(&hiddenIDs, hiddenSet, player.unit.ID())
				unitHidden = true
			} else if err := appendEntry(player.unitEncoded); err != nil {
				return nil, nil, err
			}
		}

		playerEntity := player.playerWithoutUnit
		playerData := player.playerWithoutUnitData
		if !unitHidden && player.unit != nil {
			playerEntity = player.playerWithUnit
			playerData = player.playerWithUnitData
		}
		if playerEntity != nil && s.entitySnapshotHidden(viewer, playerEntity) {
			appendHiddenEntityID(&hiddenIDs, hiddenSet, playerEntity.ID())
			continue
		}
		if err := appendEntry(playerData); err != nil {
			return nil, nil, err
		}
	}

	for _, entry := range base.extraEntries {
		if s.entitySnapshotHidden(viewer, entry.entity) {
			appendHiddenEntityID(&hiddenIDs, hiddenSet, entry.entity.ID())
			continue
		}
		if err := appendEntry(entry.encoded); err != nil {
			return nil, nil, err
		}
	}

	flush()
	packets = append(packets, base.legacyPackets...)
	return packets, hiddenIDs, nil
}

func (s *Server) buildPlayerSnapshotBase(c *Conn) (entitySnapshotPlayerBase, bool, error) {
	if s == nil || c == nil || c.playerID == 0 {
		return entitySnapshotPlayerBase{}, false, nil
	}
	var unit protocol.UnitSyncEntity
	if snapshot := s.snapshotPlayerUnitEntity(c); snapshot != nil {
		if !s.prepareUnitEntitySnapshot(snapshot) {
			fmt.Printf("[net] skipped unit snapshot conn=%d player=%d unit=%d invalid_type=%d\n",
				c.id, c.playerID, snapshot.ID(), snapshot.TypeID)
		} else {
			unit = snapshot
		}
	}
	withUnit := &protocol.PlayerEntity{IDValue: c.playerID}
	s.updatePlayerEntity(withUnit, c)
	if unit != nil {
		withUnit.Unit = protocol.UnitBox{IDValue: unit.ID()}
	}
	withoutUnit := *withUnit
	withoutUnit.Unit = nil

	withUnitData, err := s.encodeEntitySnapshotEntry(withUnit)
	if err != nil {
		return entitySnapshotPlayerBase{}, false, err
	}
	withoutUnitData := withUnitData
	if unit != nil {
		withoutUnitData, err = s.encodeEntitySnapshotEntry(&withoutUnit)
		if err != nil {
			return entitySnapshotPlayerBase{}, false, err
		}
	}
	var unitData []byte
	if unit != nil {
		unitData, err = s.encodeEntitySnapshotEntry(unit)
		if err != nil {
			return entitySnapshotPlayerBase{}, false, err
		}
	}
	return entitySnapshotPlayerBase{
		unit:                  unit,
		unitEncoded:           unitData,
		playerWithUnit:        withUnit,
		playerWithUnitData:    withUnitData,
		playerWithoutUnit:     &withoutUnit,
		playerWithoutUnitData: withoutUnitData,
	}, true, nil
}

func (s *Server) connectedSnapshotPlayers() []*Conn {
	s.mu.Lock()
	players := make([]*Conn, 0, len(s.conns))
	for c := range s.conns {
		if c.hasConnected && c.playerID != 0 {
			players = append(players, c)
		}
	}
	s.mu.Unlock()
	sort.Slice(players, func(i, j int) bool {
		if players[i].playerID != players[j].playerID {
			return players[i].playerID < players[j].playerID
		}
		return players[i].id < players[j].id
	})
	return players
}

func (s *Server) encodeEntitySnapshotEntry(entity protocol.UnitSyncEntity) ([]byte, error) {
	if entity == nil {
		return nil, nil
	}
	ew := protocol.NewWriterWithContext(s.TypeIO)
	if err := ew.WriteInt32(entity.ID()); err != nil {
		return nil, err
	}
	if err := ew.WriteByte(entity.ClassID()); err != nil {
		return nil, err
	}
	entity.BeforeWrite()
	if err := entity.WriteSync(ew); err != nil {
		return nil, err
	}
	return ew.Bytes(), nil
}

func (s *Server) logInitialEntitySnapshotDebug(c *Conn, packets []*protocol.Remote_NetClient_entitySnapshot_32) {
	if s == nil || c == nil || len(packets) == 0 || !s.verboseNetLog.Load() {
		return
	}
	seq := int(c.entityDebugSent.Add(1))
	if seq > 4 {
		return
	}
	for i, packet := range packets {
		if packet == nil {
			continue
		}
		fmt.Printf("[net] entity snapshot debug conn=%d player=%d seq=%d packet=%d/%d amount=%d bytes=%d entries=%s\n",
			c.id, c.playerID, seq, i+1, len(packets), packet.Amount, len(packet.Data),
			strings.Join(s.describeEntitySnapshotPacket(packet), " | "))
	}
}

func (s *Server) describeEntitySnapshotPacket(packet *protocol.Remote_NetClient_entitySnapshot_32) []string {
	if s == nil || packet == nil {
		return nil
	}
	r := protocol.NewReaderWithContext(packet.Data, s.TypeIO)
	out := make([]string, 0, int(packet.Amount)+1)
	for i := 0; i < int(packet.Amount); i++ {
		entryStart := r.Remaining()
		id, err := r.ReadInt32()
		if err != nil {
			out = append(out, fmt.Sprintf("entry=%d read_id_err=%v", i, err))
			return out
		}
		classID, err := r.ReadByte()
		if err != nil {
			out = append(out, fmt.Sprintf("id=%d read_class_err=%v", id, err))
			return out
		}
		switch classID {
		case 12:
			player := &protocol.PlayerEntity{IDValue: id}
			if err := player.ReadSync(r); err != nil {
				out = append(out, fmt.Sprintf("id=%d class=%d player_err=%v", id, classID, err))
				return out
			}
			unitID := int32(0)
			if player.Unit != nil {
				unitID = player.Unit.ID()
			}
			out = append(out, fmt.Sprintf("id=%d class=%d player team=%d unit=%d name_bytes=%d len=%d",
				id, classID, player.TeamID, unitID, len([]byte(player.Name)), entryStart-r.Remaining()))
		default:
			unit := &protocol.UnitEntitySync{IDValue: id, ClassIDValue: classID, ClassIDSet: true}
			if err := unit.ReadSync(r); err != nil {
				out = append(out, fmt.Sprintf("id=%d class=%d unit_err=%v", id, classID, err))
				return out
			}
			out = append(out, fmt.Sprintf("id=%d class=%d unit type=%d team=%d len=%d",
				id, classID, unit.TypeID, unit.TeamID, entryStart-r.Remaining()))
		}
	}
	if rem := r.Remaining(); rem != 0 {
		out = append(out, fmt.Sprintf("remaining=%d", rem))
	}
	return out
}

func cloneControllerState(src any) any {
	state, ok := src.(*protocol.ControllerState)
	if !ok || state == nil {
		return src
	}
	copy := *state
	return &copy
}

func cloneUnitEntitySync(src *protocol.UnitEntitySync) *protocol.UnitEntitySync {
	if src == nil {
		return nil
	}
	copy := *src
	copy.Controller = cloneControllerState(src.Controller)
	copy.Abilities = append([]protocol.Ability(nil), src.Abilities...)
	copy.Mounts = append([]protocol.WeaponMount(nil), src.Mounts...)
	copy.Payloads = append([]protocol.Payload(nil), src.Payloads...)
	copy.Statuses = append([]protocol.StatusEntry(nil), src.Statuses...)
	if src.Plans != nil {
		copy.Plans = make([]*protocol.BuildPlan, len(src.Plans))
		for i, plan := range src.Plans {
			if plan == nil {
				continue
			}
			planCopy := *plan
			copy.Plans[i] = &planCopy
		}
	}
	return &copy
}

func controlledBlockUnitRef(pos int32) protocol.Unit {
	return protocol.BlockUnitRef{
		TileRef: protocol.BlockUnitTileRef{PosValue: pos},
	}
}

func extractControlledBuildPos(obj any) (int32, bool) {
	switch v := obj.(type) {
	case protocol.BlockUnit:
		if v.Tile() == nil {
			return 0, false
		}
		return v.Tile().Pos(), true
	default:
		return 0, false
	}
}

func (s *Server) writePlayerSync(w *protocol.Writer, c *Conn) error {
	if err := w.WriteBool(false); err != nil { // admin
		return err
	}
	if err := w.WriteBool(c.boosting); err != nil { // boosting
		return err
	}
	if err := protocol.WriteColor(w, protocol.Color{RGBA: c.color}); err != nil {
		return err
	}
	if err := w.WriteFloat32(c.pointerX); err != nil { // mouseX
		return err
	}
	if err := w.WriteFloat32(c.pointerY); err != nil { // mouseY
		return err
	}
	name := s.playerDisplayName(c)
	if err := protocol.WriteString(w, &name); err != nil {
		return err
	}
	if err := w.WriteInt16(-1); err != nil { // selectedBlock
		return err
	}
	if err := w.WriteInt32(0); err != nil { // selectedRotation
		return err
	}
	if err := w.WriteBool(c.shooting); err != nil { // shooting
		return err
	}
	teamID := c.TeamID()
	playerUnit := protocol.Unit(nil)
	if info, ok := s.currentControlledBuildInfo(c); ok {
		teamID = info.TeamID
		playerUnit = controlledBlockUnitRef(info.Pos)
	}
	if s.UnitInfoFn != nil && c.unitID != 0 {
		if info, ok := s.UnitInfoFn(c.unitID); ok && info.TeamID != 0 {
			teamID = info.TeamID
		}
	}
	if err := protocol.WriteTeam(w, &protocol.Team{ID: teamID}); err != nil {
		return err
	}
	if err := w.WriteBool(c.typing); err != nil { // typing
		return err
	}
	if err := protocol.WriteUnit(w, playerUnit); err != nil {
		return err
	}
	if err := w.WriteFloat32(c.snapX); err != nil { // x
		return err
	}
	if err := w.WriteFloat32(c.snapY); err != nil { // y
		return err
	}
	return nil
}

func (s *Server) ensurePlayerEntity(c *Conn) *protocol.PlayerEntity {
	if c == nil || c.playerID == 0 {
		return nil
	}
	s.entityMu.Lock()
	defer s.entityMu.Unlock()
	if ent, ok := s.entities[c.playerID]; ok {
		if p, ok2 := ent.(*protocol.PlayerEntity); ok2 {
			return p
		}
	}
	p := &protocol.PlayerEntity{IDValue: c.playerID}
	s.entities[c.playerID] = p
	return p
}

func (s *Server) updatePlayerEntity(p *protocol.PlayerEntity, c *Conn) {
	if p == nil || c == nil {
		return
	}
	p.Admin = false
	p.Boosting = c.boosting
	p.ColorRGBA = c.color
	p.MouseX = c.pointerX
	p.MouseY = c.pointerY
	p.Name = s.playerDisplayName(c)
	p.SelectedBlock = -1
	p.SelectedRotation = 0
	p.Shooting = c.shooting
	p.TeamID = c.TeamID()
	p.Unit = nil
	if info, ok := s.currentControlledBuildInfo(c); ok {
		p.TeamID = info.TeamID
		p.Unit = controlledBlockUnitRef(info.Pos)
		p.X = info.X
		p.Y = info.Y
	} else {
		p.X = c.snapX
		p.Y = c.snapY
	}
	if s.UnitInfoFn != nil && c.unitID != 0 {
		if info, ok := s.UnitInfoFn(c.unitID); ok && info.TeamID != 0 {
			p.TeamID = info.TeamID
		}
	}
	p.Typing = c.typing
	if c.unitID != 0 && !c.controlBuildActive && s.hasValidPlayerUnitEntity(c) {
		p.Unit = protocol.UnitBox{IDValue: c.unitID}
	}
}

func (s *Server) validUnitTypeID(typeID int16) bool {
	if s == nil || typeID <= 0 || s.Content == nil {
		return false
	}
	return s.Content.UnitType(typeID) != nil
}

func (s *Server) fallbackPlayerUnitTypeID() int16 {
	if s == nil || s.PlayerUnitTypeFn == nil {
		return 0
	}
	typeID := s.PlayerUnitTypeFn()
	if !s.validUnitTypeID(typeID) {
		return 0
	}
	return typeID
}

func (s *Server) unitTypeName(typeID int16) string {
	if s == nil || typeID <= 0 || s.Content == nil {
		return ""
	}
	unit := s.Content.UnitType(typeID)
	if unit == nil {
		return ""
	}
	return unit.Name()
}

func (s *Server) applyUnitEntityLayout(u *protocol.UnitEntitySync) {
	if u == nil {
		return
	}
	u.ApplyLayoutByName(s.unitTypeName(u.TypeID))
}

func (s *Server) authoritativeUnitSnapshot(unitID int32, controller protocol.UnitController) *protocol.UnitEntitySync {
	if s == nil || unitID == 0 || s.UnitSyncFn == nil {
		return nil
	}
	unit, ok := s.UnitSyncFn(unitID, controller)
	if !ok || unit == nil {
		return nil
	}
	return cloneUnitEntitySync(unit)
}

func (s *Server) normalizedUnitTypeID(typeID int16, controller protocol.UnitController) int16 {
	if s.validUnitTypeID(typeID) {
		return typeID
	}
	if state, ok := controller.(*protocol.ControllerState); ok && state != nil && state.Type == protocol.ControllerPlayer {
		return s.fallbackPlayerUnitTypeID()
	}
	return 0
}

func (s *Server) hasValidPlayerUnitEntity(c *Conn) bool {
	if s == nil || c == nil || c.unitID == 0 || c.controlBuildActive {
		return false
	}
	if unit := s.authoritativeUnitSnapshot(c.unitID, &protocol.ControllerState{Type: protocol.ControllerPlayer, PlayerID: c.playerID}); unit != nil {
		return unit.Health > 0 && s.normalizedUnitTypeID(unit.TypeID, unit.Controller) > 0
	}
	if s.UnitInfoFn != nil {
		info, ok := s.UnitInfoFn(c.unitID)
		if !ok || info.Health <= 0 {
			return false
		}
		return s.normalizedUnitTypeID(info.TypeID, &protocol.ControllerState{Type: protocol.ControllerPlayer, PlayerID: c.playerID}) > 0
	}
	s.entityMu.Lock()
	defer s.entityMu.Unlock()
	ent, ok := s.entities[c.unitID]
	if !ok {
		return false
	}
	u, ok := ent.(*protocol.UnitEntitySync)
	if !ok || u == nil {
		return false
	}
	return s.validUnitTypeID(u.TypeID)
}

func (s *Server) prepareUnitEntitySnapshot(u *protocol.UnitEntitySync) bool {
	if s == nil || u == nil {
		return false
	}
	s.syncUnitFromWorld(u)
	if normalized := s.normalizedUnitTypeID(u.TypeID, u.Controller); normalized > 0 {
		u.TypeID = normalized
		s.applyUnitEntityLayout(u)
		return true
	}
	return false
}

func (s *Server) nextUnitID() int32 {
	for {
		var id int32
		if s.ReserveUnitIDFn != nil {
			id = s.ReserveUnitIDFn()
		}
		if id <= 0 {
			s.entityMu.Lock()
			id = s.unitNext
			s.unitNext++
			if s.unitNext <= 0 {
				s.unitNext = 2000000000
			}
			s.entityMu.Unlock()
		}
		if id <= 0 || s.entityIDConflicts(id) {
			continue
		}
		return id
	}
}

func (s *Server) entityIDConflicts(id int32) bool {
	if id <= 0 {
		return true
	}
	s.mu.Lock()
	for c := range s.conns {
		if c != nil && (c.playerID == id || c.unitID == id) {
			s.mu.Unlock()
			return true
		}
	}
	for _, c := range s.pending {
		if c != nil && (c.playerID == id || c.unitID == id) {
			s.mu.Unlock()
			return true
		}
	}
	s.mu.Unlock()

	s.entityMu.Lock()
	_, exists := s.entities[id]
	s.entityMu.Unlock()
	return exists
}

func (s *Server) ensurePlayerUnitEntity(c *Conn) *protocol.UnitEntitySync {
	if c == nil || c.playerID == 0 {
		return nil
	}
	if c.controlBuildActive {
		return nil
	}
	if c.unitID == 0 {
		c.unitID = s.nextUnitID()
	}
	playerTypeID := int16(1)
	if s.PlayerUnitTypeFn != nil {
		if t := s.PlayerUnitTypeFn(); t >= 0 {
			playerTypeID = t
		}
	}
	if fallback := s.fallbackPlayerUnitTypeID(); fallback > 0 {
		playerTypeID = fallback
	}
	s.entityMu.Lock()
	defer s.entityMu.Unlock()
	if ent, ok := s.entities[c.unitID]; ok {
		if u, ok2 := ent.(*protocol.UnitEntitySync); ok2 {
			// keep unit synced with latest client snapshot values
			u.Controller = &protocol.ControllerState{Type: protocol.ControllerPlayer, PlayerID: c.playerID}
			if u.Plans == nil {
				u.Plans = []*protocol.BuildPlan{}
			}
			if u.Mounts == nil {
				u.Mounts = []protocol.WeaponMount{}
			}
			if u.Abilities == nil {
				u.Abilities = []protocol.Ability{}
			}
			if u.Statuses == nil {
				u.Statuses = []protocol.StatusEntry{}
			}
			if u.Stack.Item == nil {
				u.Stack.Item = protocol.ItemRef{ItmID: 0, ItmName: ""}
				u.Stack.Amount = 0
			}
			s.syncUnitFromWorld(u)
			if normalized := s.normalizedUnitTypeID(u.TypeID, u.Controller); normalized > 0 {
				u.TypeID = normalized
			}
			if u.X == 0 && u.Y == 0 {
				u.X = c.snapX
				u.Y = c.snapY
			}
			if s.UnitInfoFn != nil {
				if info, ok := s.UnitInfoFn(c.unitID); ok && info.TeamID != 0 {
					u.TeamID = info.TeamID
				} else {
					u.TeamID = c.TeamID()
				}
			} else {
				u.TeamID = c.TeamID()
			}
			if s.UnitInfoFn == nil {
				u.TypeID = playerTypeID
			} else if info, ok := s.UnitInfoFn(c.unitID); !ok || !s.validUnitTypeID(info.TypeID) {
				u.TypeID = playerTypeID
			}
			s.applyUnitEntityLayout(u)
			u.Elevation = 1
			if u.Health <= 0 {
				u.Health = 100
			}
			return u
		}
	}
	if snapshot := s.authoritativeUnitSnapshot(c.unitID, &protocol.ControllerState{Type: protocol.ControllerPlayer, PlayerID: c.playerID}); snapshot != nil {
		if snapshot.Controller == nil {
			snapshot.Controller = &protocol.ControllerState{Type: protocol.ControllerPlayer, PlayerID: c.playerID}
		}
		if snapshot.X == 0 && snapshot.Y == 0 {
			snapshot.X = c.snapX
			snapshot.Y = c.snapY
		}
		s.entities[c.unitID] = snapshot
		return snapshot
	}
	u := &protocol.UnitEntitySync{
		IDValue:        c.unitID,
		Abilities:      []protocol.Ability{},
		Ammo:           0,
		Controller:     &protocol.ControllerState{Type: protocol.ControllerPlayer, PlayerID: c.playerID},
		Elevation:      1,
		Flag:           0,
		Health:         100,
		Shooting:       false,
		MineTile:       nil,
		Mounts:         []protocol.WeaponMount{},
		Plans:          []*protocol.BuildPlan{},
		Rotation:       90,
		Shield:         0,
		SpawnedByCore:  true,
		Stack:          protocol.ItemStack{Item: protocol.ItemRef{ItmID: 0, ItmName: ""}, Amount: 0},
		Statuses:       []protocol.StatusEntry{},
		TeamID:         c.TeamID(),
		TypeID:         playerTypeID,
		UpdateBuilding: false,
		Vel:            protocol.Vec2{X: 0, Y: 0},
		X:              c.snapX,
		Y:              c.snapY,
	}
	s.syncUnitFromWorld(u)
	s.applyUnitEntityLayout(u)
	s.entities[c.unitID] = u
	return u
}

func (s *Server) playerUnitEntity(c *Conn) *protocol.UnitEntitySync {
	if c == nil || c.playerID == 0 || c.unitID == 0 || c.controlBuildActive {
		return nil
	}
	s.entityMu.Lock()
	defer s.entityMu.Unlock()
	if ent, ok := s.entities[c.unitID]; ok {
		if u, ok2 := ent.(*protocol.UnitEntitySync); ok2 {
			u.Controller = &protocol.ControllerState{Type: protocol.ControllerPlayer, PlayerID: c.playerID}
			s.syncUnitFromWorld(u)
			if normalized := s.normalizedUnitTypeID(u.TypeID, u.Controller); normalized > 0 {
				u.TypeID = normalized
			}
			s.applyUnitEntityLayout(u)
			if u.X == 0 && u.Y == 0 {
				u.X = c.snapX
				u.Y = c.snapY
			}
			return u
		}
	}
	if snapshot := s.authoritativeUnitSnapshot(c.unitID, &protocol.ControllerState{Type: protocol.ControllerPlayer, PlayerID: c.playerID}); snapshot != nil {
		s.entities[c.unitID] = snapshot
		return snapshot
	}
	return nil
}

func (s *Server) snapshotPlayerUnitEntity(c *Conn) *protocol.UnitEntitySync {
	if s == nil || c == nil || c.playerID == 0 || c.unitID == 0 || c.dead || c.controlBuildActive {
		return nil
	}
	var worldInfo UnitInfo
	var hasWorldInfo bool
	if s.UnitInfoFn != nil {
		info, ok := s.UnitInfoFn(c.unitID)
		if !ok || info.Health <= 0 {
			return nil
		}
		if s.normalizedUnitTypeID(info.TypeID, &protocol.ControllerState{Type: protocol.ControllerPlayer, PlayerID: c.playerID}) <= 0 {
			return nil
		}
		worldInfo = info
		hasWorldInfo = true
	}
	s.entityMu.Lock()
	defer s.entityMu.Unlock()
	ent, ok := s.entities[c.unitID]
	var u *protocol.UnitEntitySync
	if ok {
		u, _ = ent.(*protocol.UnitEntitySync)
	}
	if u == nil {
		if snapshot := s.authoritativeUnitSnapshot(c.unitID, &protocol.ControllerState{Type: protocol.ControllerPlayer, PlayerID: c.playerID}); snapshot != nil {
			if snapshot.Controller == nil {
				snapshot.Controller = &protocol.ControllerState{Type: protocol.ControllerPlayer, PlayerID: c.playerID}
			}
			if snapshot.X == 0 && snapshot.Y == 0 {
				snapshot.X = c.snapX
				snapshot.Y = c.snapY
			}
			s.entities[c.unitID] = snapshot
			return snapshot
		}
		if !hasWorldInfo {
			return nil
		}
		typeID := s.normalizedUnitTypeID(worldInfo.TypeID, &protocol.ControllerState{Type: protocol.ControllerPlayer, PlayerID: c.playerID})
		if typeID <= 0 {
			return nil
		}
		u = &protocol.UnitEntitySync{
			IDValue:        c.unitID,
			Abilities:      []protocol.Ability{},
			Ammo:           0,
			Controller:     &protocol.ControllerState{Type: protocol.ControllerPlayer, PlayerID: c.playerID},
			Elevation:      1,
			Flag:           0,
			Health:         worldInfo.Health,
			Shooting:       false,
			MineTile:       nil,
			Mounts:         []protocol.WeaponMount{},
			Plans:          []*protocol.BuildPlan{},
			Rotation:       90,
			Shield:         0,
			SpawnedByCore:  true,
			Stack:          protocol.ItemStack{Item: protocol.ItemRef{ItmID: 0, ItmName: ""}, Amount: 0},
			Statuses:       []protocol.StatusEntry{},
			TeamID:         worldInfo.TeamID,
			TypeID:         typeID,
			UpdateBuilding: false,
			Vel:            protocol.Vec2{X: 0, Y: 0},
			X:              worldInfo.X,
			Y:              worldInfo.Y,
		}
		s.entities[c.unitID] = u
	}
	if u.Controller == nil {
		u.Controller = &protocol.ControllerState{Type: protocol.ControllerPlayer, PlayerID: c.playerID}
	}
	s.syncUnitFromWorld(u)
	if normalized := s.normalizedUnitTypeID(u.TypeID, u.Controller); normalized > 0 {
		u.TypeID = normalized
	} else {
		return nil
	}
	s.applyUnitEntityLayout(u)
	if u.X == 0 && u.Y == 0 {
		u.X = c.snapX
		u.Y = c.snapY
	}
	return cloneUnitEntitySync(u)
}

func (s *Server) SetConnUnitStack(c *Conn, itemID int16, amount int32) {
	if s == nil || c == nil || c.playerID == 0 {
		return
	}
	if c.unitID != 0 && s.SetUnitStackFn != nil {
		_ = s.SetUnitStackFn(c.unitID, itemID, amount)
	}
	u := s.playerUnitEntity(c)
	if u == nil && c.unitID != 0 && s.connUnitAlive(c) {
		u = s.ensurePlayerUnitEntity(c)
	}
	if u == nil {
		return
	}
	if amount <= 0 {
		u.Stack.Item = protocol.ItemRef{ItmID: 0, ItmName: ""}
		u.Stack.Amount = 0
		return
	}
	if s.TypeIO != nil && s.TypeIO.ItemLookup != nil {
		if item := s.TypeIO.ItemLookup(itemID); item != nil {
			u.Stack.Item = item
			u.Stack.Amount = amount
			return
		}
	}
	u.Stack.Item = protocol.ItemRef{ItmID: itemID, ItmName: ""}
	u.Stack.Amount = amount
}

func (s *Server) syncUnitFromWorld(u *protocol.UnitEntitySync) {
	if s == nil || u == nil {
		return
	}
	if snapshot := s.authoritativeUnitSnapshot(u.ID(), u.Controller); snapshot != nil {
		*u = *snapshot
		return
	}
	if s.UnitInfoFn == nil {
		return
	}
	info, ok := s.UnitInfoFn(u.ID())
	if !ok {
		return
	}
	u.X = info.X
	u.Y = info.Y
	if info.Health >= 0 {
		u.Health = info.Health
	}
	if normalized := s.normalizedUnitTypeID(info.TypeID, u.Controller); normalized > 0 {
		u.TypeID = normalized
	}
	if info.TeamID != 0 {
		u.TeamID = info.TeamID
	}
}

func (s *Server) dropPlayerUnitEntity(c *Conn, unitID int32) bool {
	if c == nil || c.playerID == 0 || unitID == 0 {
		return false
	}
	s.entityMu.Lock()
	defer s.entityMu.Unlock()
	ent, ok := s.entities[unitID]
	if !ok {
		return false
	}
	u, ok := ent.(*protocol.UnitEntitySync)
	if !ok {
		return false
	}
	state, ok := u.Controller.(*protocol.ControllerState)
	if !ok || state == nil || state.Type != protocol.ControllerPlayer || state.PlayerID != c.playerID {
		return false
	}
	delete(s.entities, unitID)
	if s.DropUnitFn != nil {
		s.DropUnitFn(unitID)
	}
	return true
}

func (s *Server) broadcastPlayerDisconnect(playerID int32, except *Conn) {
	s.mu.Lock()
	peers := make([]*Conn, 0, len(s.conns))
	for c := range s.conns {
		if c == nil || c == except {
			continue
		}
		peers = append(peers, c)
	}
	s.mu.Unlock()
	for _, peer := range peers {
		_ = peer.SendAsync(&protocol.Remote_NetClient_playerDisconnect_31{Playerid: playerID})
	}
}

func (s *Server) BroadcastTileConfig(pos int32, value any, except *Conn) {
	if pos < 0 {
		return
	}
	s.mu.Lock()
	peers := make([]*Conn, 0, len(s.conns))
	for c := range s.conns {
		if c == nil || c == except || !c.hasConnected {
			continue
		}
		peers = append(peers, c)
	}
	s.mu.Unlock()
	for _, peer := range peers {
		clonedValue, err := protocol.CloneObjectValue(value)
		if err != nil {
			fmt.Printf("[net] skip tileConfig pos=%d err=%v type=%T\n", pos, err, value)
			continue
		}
		packet := &protocol.Remote_InputHandler_tileConfig_90{
			Build: protocol.BuildingBox{PosValue: pos},
			Value: clonedValue,
		}
		_ = peer.SendAsync(packet)
	}
}

func (s *Server) addEntity(u protocol.Unit) {
	if u == nil {
		return
	}
	// Convert Unit to UnitSyncEntity if needed
	var syncEnt protocol.UnitSyncEntity
	if ent, ok := u.(protocol.UnitSyncEntity); ok {
		syncEnt = ent
	} else {
		// If it's just a Unit with no sync support, create a sync entity from it
		// This is a simplified approach - in practice, units should already be UnitSyncEntity
		return
	}

	s.entityMu.Lock()
	defer s.entityMu.Unlock()
	id := syncEnt.ID()
	if id == 0 {
		// Assign a new ID if not set
		s.unitNext++
		id = s.unitNext
		syncEnt.SetID(id)
	}
	s.entities[id] = syncEnt
}

// removeEntity 从实体列表中移除指定ID的实体
func (s *Server) removeEntity(id int32) {
	s.entityMu.Lock()
	defer s.entityMu.Unlock()
	delete(s.entities, id)
}

// getEntity 获取指定ID的实体
func (s *Server) getEntity(id int32) (protocol.UnitSyncEntity, bool) {
	s.entityMu.Lock()
	defer s.entityMu.Unlock()
	ent, ok := s.entities[id]
	if syncEnt, ok2 := ent.(protocol.UnitSyncEntity); ok2 && ok {
		return syncEnt, true
	}
	return nil, false
}

// broadcastEntitySync 广播实体同步数据给所有连接
func (s *Server) broadcastEntitySync(entity protocol.UnitSyncEntity) {
	s.mu.Lock()
	peers := make([]*Conn, 0, len(s.conns))
	for c := range s.conns {
		if c != nil && c.hasConnected {
			peers = append(peers, c)
		}
	}
	s.mu.Unlock()

	for _, c := range peers {
		if err := c.Send(&protocol.Remote_NetClient_entitySnapshot_32{
			Amount: 1,
			Data:   s.serializeEntity(entity),
		}); err != nil {
			fmt.Printf("[net] failed to send entity sync: %v\n", err)
		}
	}
}

// serializeEntity 序列化实体为字节数组
func (s *Server) serializeEntity(entity protocol.UnitSyncEntity) []byte {
	if entity == nil {
		return nil
	}
	w := protocol.NewWriterWithContext(s.TypeIO)
	if err := w.WriteInt32(entity.ID()); err != nil {
		return nil
	}
	if err := w.WriteByte(entity.ClassID()); err != nil {
		return nil
	}
	entity.BeforeWrite()
	if err := entity.WriteSync(w); err != nil {
		return nil
	}
	return append([]byte(nil), w.Bytes()...)
}

// sendWorldHandshake pushes world stream data to the connected client.
func (s *Server) sendWorldHandshake(c *Conn, pkt *protocol.ConnectPacket) error {
	worldData, err := s.WorldDataFn(c, pkt)
	if err != nil {
		return err
	}
	if inspection, ierr := worldstream.InspectWorldStreamPayload(worldData); ierr == nil {
		fmt.Printf("[net] world handshake payload conn=%d player=%d live=%v bytes=%d raw=%d player=%d..%d content=%d..%d patchesEnd=%d mapEnd=%d teamBlocks=%d tail=%d tailPrefix=%s\n",
			c.id, c.playerID, c.UsesLiveWorldStream(), inspection.CompressedLen, inspection.RawLen,
			inspection.PlayerStart, inspection.PlayerEnd, inspection.ContentStart, inspection.ContentEnd,
			inspection.PatchesEnd, inspection.MapEnd, inspection.TeamBlocksLen, inspection.TailLen, inspection.TailPrefixHex)
	} else {
		fmt.Printf("[net] world handshake inspect failed conn=%d player=%d live=%v bytes=%d err=%v\n",
			c.id, c.playerID, c.UsesLiveWorldStream(), len(worldData), ierr)
	}
	worldID, ok := s.Registry.PacketID(&protocol.WorldStream{})
	if !ok {
		return fmt.Errorf("world stream packet id not found")
	}
	return c.SendStream(worldID, worldData)
}

func (s *Server) sendWorldDataBegin(c *Conn) error {
	if c == nil {
		return fmt.Errorf("invalid connection")
	}
	return c.Send(&protocol.Remote_NetClient_worldDataBegin_28{})
}

// SyncWorldToConn re-synchronizes a single connected client with the current world.
// Match vanilla /sync semantics: send worldDataBegin(), then stream the world again.
// Do not clear server-side unit ownership or mark the player dead here.
func (s *Server) SyncWorldToConn(c *Conn) error {
	if s == nil || c == nil || c.playerID == 0 {
		return fmt.Errorf("invalid connection")
	}
	c.syncTime.Store(0)
	c.SetWorldReloadGrace(4 * time.Second)
	c.queueReloadConfirm(false)
	if err := s.sendWorldDataBegin(c); err != nil {
		return err
	}
	if err := s.sendWorldHandshake(c, nil); err != nil {
		return err
	}
	return nil
}

// SyncEntitySnapshotsToConn pushes the current entity snapshot set to one client
// immediately, instead of waiting for the periodic entity snapshot ticker.
func (s *Server) SyncEntitySnapshotsToConn(c *Conn) error {
	if s == nil || c == nil || c.playerID == 0 {
		return fmt.Errorf("invalid connection")
	}
	packets, hiddenIDs, err := s.buildEntitySnapshotPacketsForConn(c)
	if err != nil {
		return err
	}
	for _, packet := range packets {
		if packet == nil {
			continue
		}
		if err := s.sendUnreliable(c, packet); err != nil {
			return err
		}
	}
	return s.sendHiddenSnapshotToConn(c, hiddenIDs)
}

// ReloadWorldLiveForAll pushes a fresh world stream to all online players without kicking.
// After stream sync, it forces a respawn so client camera/unit rebinds to the new world.
func (s *Server) ReloadWorldLiveForAll() (reloaded int, failed int) {
	s.mu.Lock()
	peers := make([]*Conn, 0, len(s.conns))
	for c := range s.conns {
		if c != nil && c.hasConnected && c.playerID != 0 {
			peers = append(peers, c)
		}
	}
	s.mu.Unlock()

	ready := make([]*Conn, 0, len(peers))
	for _, c := range peers {
		s.assignConnTeam(c, true)
		s.beginWorldHotReload(c)
		if err := s.sendWorldDataBegin(c); err != nil {
			failed++
			s.emitEvent(c, "world_hot_reload_failed", "", err.Error())
			continue
		}
		ready = append(ready, c)
	}

	for _, c := range ready {
		if err := s.sendWorldHandshake(c, nil); err != nil {
			failed++
			s.emitEvent(c, "world_hot_reload_failed", "", err.Error())
			continue
		}
		reloaded++
	}
	return reloaded, failed
}

// ReloadWorldLiveForAllLegacy is a deprecated alias for ReloadWorldLiveForAll.
func (s *Server) ReloadWorldLiveForAllLegacy() (reloaded int, failed int) {
	return s.ReloadWorldLiveForAll()
}

func defaultWorldData(_ *Conn, _ *protocol.ConnectPacket) ([]byte, error) {
	if data, _, err := runtimeassets.LoadBootstrapWorld(""); err == nil {
		return data, nil
	}

	var out bytes.Buffer
	zw := zlib.NewWriter(&out)
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

