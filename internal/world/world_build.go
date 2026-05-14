package world

import (
	"reflect"
)

func isCenterBuildingTile(tile *Tile) bool {
	if tile == nil || tile.Build == nil || tile.Block == 0 {
		return false
	}
	return tile.Build.X == tile.X && tile.Build.Y == tile.Y
}

// ApplyBuildPlans applies incremental build/break operations from client packets.
func (w *World) ApplyBuildPlans(team TeamID, ops []BuildPlanOp) []int32 {
	return w.ApplyBuildPlansForOwner(0, team, ops)
}

func (w *World) ApplyBuildPlansForOwner(owner int32, team TeamID, ops []BuildPlanOp) []int32 {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.model == nil || len(ops) == 0 {
		return nil
	}
	changed := make([]int32, 0, len(ops))
	seen := make(map[int32]struct{}, len(ops))
	addChanged := func(pos int32) {
		if _, ok := seen[pos]; ok {
			return
		}
		seen[pos] = struct{}{}
		changed = append(changed, pos)
	}
	for _, op := range ops {
		if !w.model.InBounds(int(op.X), int(op.Y)) {
			continue
		}
		w.nextPlanOrder++
		w.applyBuildPlanOpLocked(owner, team, op, w.nextPlanOrder, addChanged)
	}
	return changed
}

// ApplyBuildPlanSnapshot reconciles one team's queue with authoritative snapshot plans.
// This matches vanilla queue semantics: absent plans are removed, present plans are ordered.
func (w *World) ApplyBuildPlanSnapshot(team TeamID, ops []BuildPlanOp) []int32 {
	return w.ApplyBuildPlanSnapshotForOwner(0, team, ops)
}

// ApplyPlacementPlanSnapshot reconciles only placement plans from client preview snapshots.
// Official client snapshots do not authoritatively carry break queues, so pending breaks
// must remain driven by beginBreak/removeQueue packets instead of being cleared here.
func (w *World) ApplyPlacementPlanSnapshot(team TeamID, ops []BuildPlanOp) []int32 {
	return w.ApplyPlacementPlanSnapshotForOwner(0, team, ops)
}

func (w *World) ApplyBuildPlanSnapshotForOwner(owner int32, team TeamID, ops []BuildPlanOp) []int32 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.applyBuildPlanSnapshotForOwnerLocked(owner, team, ops, true)
}

func (w *World) ApplyPlacementPlanSnapshotForOwner(owner int32, team TeamID, ops []BuildPlanOp) []int32 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.applyBuildPlanSnapshotForOwnerLocked(owner, team, ops, false)
}

func (w *World) applyBuildPlanSnapshotForOwnerLocked(owner int32, team TeamID, ops []BuildPlanOp, reconcileBreaks bool) []int32 {
	if w.model == nil {
		return nil
	}
	changed := make([]int32, 0, len(ops))
	seen := make(map[int32]struct{}, len(ops))
	addChanged := func(pos int32) {
		if _, ok := seen[pos]; ok {
			return
		}
		seen[pos] = struct{}{}
		changed = append(changed, pos)
	}

	ordered := make([]BuildPlanOp, 0, len(ops))
	wantBuild := make(map[int32]struct{}, len(ops))
	wantBreak := make(map[int32]struct{}, len(ops))
	for _, op := range ops {
		if !w.model.InBounds(int(op.X), int(op.Y)) {
			continue
		}
		pos := int32(int(op.Y)*w.model.Width + int(op.X))
		if op.Breaking {
			if _, ok := wantBreak[pos]; ok {
				continue
			}
			wantBreak[pos] = struct{}{}
			ordered = append(ordered, BuildPlanOp{
				Breaking: true,
				X:        op.X,
				Y:        op.Y,
			})
			continue
		}
		if op.BlockID <= 0 {
			continue
		}
		if _, ok := wantBuild[pos]; ok {
			continue
		}
		wantBuild[pos] = struct{}{}
		ordered = append(ordered, op)
	}

	_ = wantBuild
	_ = wantBreak

	// Reconcile removals: any queued plan for this team that is absent from
	// the latest authoritative snapshot must be dropped immediately.
	for pos, st := range w.pendingBuilds {
		if st.Team != team || st.Owner != owner {
			continue
		}
		if _, ok := wantBuild[pos]; ok {
			continue
		}
		w.cancelPendingBuildLocked(pos, st)
		addChanged(pos)
	}
	if reconcileBreaks {
		for pos, st := range w.pendingBreaks {
			if st.Team != team || st.Owner != owner {
				continue
			}
			if _, ok := wantBreak[pos]; ok {
				continue
			}
			delete(w.pendingBreaks, pos)
			addChanged(pos)
		}
	}

	for i, op := range ordered {
		w.applyBuildPlanOpLocked(owner, team, op, uint64(i+1), addChanged)
	}
	return changed
}

func (w *World) applyBuildPlanOpLocked(owner int32, team TeamID, op BuildPlanOp, queueOrder uint64, addChanged func(int32)) {
	if w.model == nil || !w.model.InBounds(int(op.X), int(op.Y)) {
		return
	}
	tile, err := w.model.TileAt(int(op.X), int(op.Y))
	if err != nil || tile == nil {
		return
	}
	pos := int32(tile.Y*w.model.Width + tile.X)

	if op.Breaking {
		if st, ok := w.pendingBuilds[pos]; ok {
			if owner != 0 && st.Owner != 0 && st.Owner != owner {
				return
			}
			w.cancelPendingBuildLocked(pos, st)
			addChanged(pos)
		}
		delete(w.factoryStates, pos)
		if st, ok := w.pendingBreaks[pos]; ok {
			if st.BlockID == int16(tile.Block) && st.Team == team {
				if owner != 0 && st.Owner != 0 && st.Owner != owner {
					return
				}
				st.Owner = owner
				st.QueueOrder = queueOrder
				w.pendingBreaks[pos] = st
				return
			}
		}
		if tile.Build == nil && tile.Block == 0 {
			delete(w.pendingBreaks, pos)
			return
		}
		if rules := w.rulesMgr.Get(); rules != nil && (rules.InstantBuild || rules.Editor) {
			w.destroyTileLocked(tile, team, owner)
			delete(w.pendingBreaks, pos)
			addChanged(pos)
			return
		}
		maxHP := constructBreakStartHealth(tile)
		refundTeam, refundStacks := w.deconstructRefundStacks(tile, team)
		w.pendingBreaks[pos] = pendingBreakState{
			Owner:       owner,
			Team:        team,
			BlockID:     int16(tile.Block),
			Rotation:    tile.Rotation,
			QueueOrder:  queueOrder,
			VisualStart: false,
			Progress:    0,
			MaxHealth:   maxHP,
			LastHP:      maxHP,
			RefundTeam:  refundTeam,
			RefundCost:  append([]ItemStack(nil), refundStacks...),
		}
		addChanged(pos)
		return
	}

	if op.BlockID <= 0 {
		return
	}
	if pending, ok := w.pendingBuilds[pos]; ok {
		if owner != 0 && pending.Owner != 0 && pending.Owner != owner {
			return
		}
		if pending.BlockID == op.BlockID && pending.Team == team && pending.Rotation == op.Rotation && reflect.DeepEqual(pending.Config, op.Config) {
			pending.Owner = owner
			pending.QueueOrder = queueOrder
			w.pendingBuilds[pos] = pending
			return
		}
		w.cancelPendingBuildLocked(pos, pending)
	}
	if rules := w.rulesMgr.Get(); rules != nil &&
		rules.DerelictRepair &&
		team != 0 &&
		tile.Team == 0 &&
		tile.Block == BlockID(op.BlockID) {
		placed := w.placeCompletedBuildingLocked(pos, tile, team, op.BlockID, op.Rotation, op.Config)
		w.entityEvents = append(w.entityEvents,
			EntityEvent{
				Kind:     EntityEventBuildHealth,
				BuildPos: packTilePos(tile.X, tile.Y),
				BuildHP:  tile.Build.Health,
			},
			EntityEvent{
				Kind:        EntityEventBuildConstructed,
				BuildPos:    packTilePos(tile.X, tile.Y),
				BuildOwner:  owner,
				BuildTeam:   team,
				BuildBlock:  op.BlockID,
				BuildRot:    op.Rotation,
				BuildConfig: placed.Config,
			},
		)
		for _, target := range placed.SelfConfigTargets {
			if target < 0 || int(target) >= len(w.model.Tiles) {
				continue
			}
			targetTile := &w.model.Tiles[target]
			w.appendBuildConfigValueEventLocked(pos, packTilePos(targetTile.X, targetTile.Y))
		}
		for _, changed := range placed.ChangedConfigs {
			if changed.targetPos < 0 || int(changed.targetPos) >= len(w.model.Tiles) {
				continue
			}
			targetTile := &w.model.Tiles[changed.targetPos]
			w.appendBuildConfigValueEventLocked(changed.nodePos, packTilePos(targetTile.X, targetTile.Y))
		}
		w.clearOverlappingRebuildPlansLocked(team, tile.X, tile.Y, op.BlockID)
		w.clearOverlappingTeamBuildPlansLocked(team, tile.X, tile.Y, op.BlockID)
		delete(w.pendingBreaks, pos)
		delete(w.pendingBuilds, pos)
		addChanged(pos)
		return
	}
	if tile.Block == BlockID(op.BlockID) && tile.Team == team && tile.Rotation == op.Rotation && tile.Build != nil {
		w.clearOverlappingRebuildPlansLocked(team, tile.X, tile.Y, op.BlockID)
		w.clearOverlappingTeamBuildPlansLocked(team, tile.X, tile.Y, op.BlockID)
		w.applyBuildingConfigLocked(pos, op.Config, true)
		delete(w.pendingBreaks, pos)
		delete(w.pendingBuilds, pos)
		return
	}
	if rules := w.rulesMgr.Get(); rules != nil && (rules.InstantBuild || rules.Editor) {
		w.placeTileLocked(tile, team, op.BlockID, int8(op.Rotation), op.Config, owner)
		delete(w.pendingBuilds, pos)
		delete(w.pendingBreaks, pos)
		addChanged(pos)
		return
	}
	w.pendingBuilds[pos] = pendingBuildState{
		Owner:      owner,
		Team:       team,
		BlockID:    op.BlockID,
		Rotation:   op.Rotation,
		Config:     op.Config,
		QueueOrder: queueOrder,
		Progress:   0,
	}
	delete(w.pendingBreaks, pos)
	addChanged(pos)
}

func cloneLiquidStacks(src []LiquidStack) []LiquidStack {
	if len(src) == 0 {
		return nil
	}
	out := make([]LiquidStack, len(src))
	copy(out, src)
	return out
}

func cloneBytes(src []byte) []byte {
	if len(src) == 0 {
		return nil
	}
	out := make([]byte, len(src))
	copy(out, src)
	return out
}

func (w *World) placeCompletedBuildingLocked(pos int32, tile *Tile, team TeamID, blockID int16, rotation int8, config any) completedBuildingPlacement {
	result := completedBuildingPlacement{Config: config}
	if w == nil || w.model == nil || tile == nil || pos < 0 || int(pos) >= len(w.model.Tiles) {
		return result
	}
	prevBlockName := w.blockNameByID(int16(tile.Block))
	prevPowerRelevant := w.isPowerRelevantBuildingLocked(tile)

	prevItems := []ItemStack(nil)
	prevLiquids := []LiquidStack(nil)
	prevPayload := []byte(nil)
	prevHealth := float32(1000)
	prevMaxHealth := float32(1000)
	if tile.Build != nil && tile.Block == BlockID(blockID) {
		prevItems = cloneItemStacks(tile.Build.Items)
		prevLiquids = cloneLiquidStacks(tile.Build.Liquids)
		prevPayload = cloneBytes(tile.Build.Payload)
		if normalized, ok := w.normalizedBuildingConfigLocked(pos); ok && result.Config == nil {
			result.Config = normalized
		}
		if tile.Build.MaxHealth > 0 {
			prevMaxHealth = tile.Build.MaxHealth
		}
		if tile.Build.Health > 0 {
			prevHealth = tile.Build.Health
		}
		if prevHealth > prevMaxHealth {
			prevHealth = prevMaxHealth
		}
	}

	if prevPowerRelevant {
		w.clearPowerLinksForBuildingLocked(pos)
	}
	w.removeActiveTileIndexLocked(pos, tile)
	w.setBuildingOccupancyLocked(pos, tile, false)
	w.clearBuildingRuntimeLocked(pos)

	tile.Block = BlockID(blockID)
	tile.Team = team
	tile.Rotation = rotation
	tile.Build = &Building{
		Block:     tile.Block,
		Team:      team,
		Rotation:  rotation,
		X:         tile.X,
		Y:         tile.Y,
		Items:     prevItems,
		Liquids:   prevLiquids,
		Payload:   prevPayload,
		Health:    prevHealth,
		MaxHealth: prevMaxHealth,
	}
	if tile.Build.Health <= 0 {
		tile.Build.Health = tile.Build.MaxHealth
	}
	if tile.Build.MaxHealth <= 0 {
		tile.Build.MaxHealth = 1000
		if tile.Build.Health <= 0 {
			tile.Build.Health = tile.Build.MaxHealth
		}
	}

	if prevPowerRelevant || w.isPowerRelevantBuildingLocked(tile) {
		w.invalidatePowerNetsLocked()
	}
	w.setBuildingOccupancyLocked(pos, tile, true)
	w.indexActiveTileLocked(pos, tile)
	w.applyBuildingConfigLocked(pos, result.Config, true)
	result.SelfConfigTargets = w.autoLinkPowerNodeLocked(pos)
	result.ChangedConfigs = w.autoLinkNearbyPowerNodesForBuildingLocked(pos)
	w.ensureTeamInventory(team)
	if affectsCoreStorageLinks(prevBlockName) || affectsCoreStorageLinks(w.blockNameByID(int16(tile.Block))) {
		w.refreshCoreStorageLinksLocked()
	}

	name := w.blockNameByID(int16(tile.Block))
	if _, ok := crafterProfilesByBlockName[name]; ok {
		w.crafterStates[pos] = crafterRuntimeState{}
	} else if _, ok := separatorProfilesByBlockName[name]; ok {
		w.crafterStates[pos] = crafterRuntimeState{}
	}
	if prof, ok := w.getBuildingWeaponProfile(int16(tile.Build.Block)); ok {
		w.buildStates[pos] = buildCombatState{Cooldown: prof.Interval, BeamLastLength: 0}
	}

	w.clearOverlappingRebuildPlansLocked(team, tile.X, tile.Y, blockID)
	w.clearOverlappingTeamBuildPlansLocked(team, tile.X, tile.Y, blockID)
	return result
}

func (w *World) placeTileLocked(tile *Tile, team TeamID, blockID int16, rotation int8, config any, owner int32) {
	if tile == nil {
		return
	}
	pos := packTilePos(tile.X, tile.Y)
	w.entityEvents = append(w.entityEvents,
		EntityEvent{
			Kind:        EntityEventBuildPlaced,
			BuildPos:    pos,
			BuildOwner:  owner,
			BuildTeam:   team,
			BuildBlock:  blockID,
			BuildRot:    rotation,
			BuildConfig: config,
		},
		EntityEvent{
			Kind:     EntityEventBuildHealth,
			BuildPos: pos,
			BuildHP:  1000,
		},
	)
	posIndex := int32(tile.Y*w.model.Width + tile.X)
	placed := w.placeCompletedBuildingLocked(posIndex, tile, team, blockID, rotation, config)
	w.entityEvents = append(w.entityEvents, EntityEvent{
		Kind:        EntityEventBuildConstructed,
		BuildPos:    pos,
		BuildOwner:  owner,
		BuildTeam:   team,
		BuildBlock:  blockID,
		BuildRot:    rotation,
		BuildConfig: placed.Config,
	})
	for _, target := range placed.SelfConfigTargets {
		if target < 0 || int(target) >= len(w.model.Tiles) {
			continue
		}
		targetTile := &w.model.Tiles[target]
		w.appendBuildConfigValueEventLocked(posIndex, packTilePos(targetTile.X, targetTile.Y))
	}
	for _, changed := range placed.ChangedConfigs {
		if changed.targetPos < 0 || int(changed.targetPos) >= len(w.model.Tiles) {
			continue
		}
		targetTile := &w.model.Tiles[changed.targetPos]
		w.appendBuildConfigValueEventLocked(changed.nodePos, packTilePos(targetTile.X, targetTile.Y))
	}
}

func (w *World) destroyTileLocked(tile *Tile, fallbackTeam TeamID, owner int32) {
	if tile == nil || (tile.Block == 0 && tile.Build == nil) {
		return
	}
	pos := int32(tile.Y*w.model.Width + tile.X)
	blockID := int16(tile.Block)
	oldBlockName := w.blockNameByID(blockID)
	teamOld := tile.Team
	if tile.Build != nil && tile.Build.Team != 0 {
		teamOld = tile.Build.Team
	}
	if teamOld == 0 {
		teamOld = fallbackTeam
	}
	powerRelevant := w.isPowerRelevantBuildingLocked(tile)
	if powerRelevant {
		w.clearPowerLinksForBuildingLocked(pos)
	}
	w.refundDeconstructCost(tile, fallbackTeam)
	// CRITICAL: Remove from indices BEFORE clearing tile data
	w.removeActiveTileIndexLocked(pos, tile)
	w.setBuildingOccupancyLocked(pos, tile, false)
	tile.Block = 0
	tile.Rotation = 0
	tile.Team = 0
	tile.Build = nil
	w.clearBuildingRuntimeLocked(pos)
	if powerRelevant {
		w.invalidatePowerNetsLocked()
	}
	if affectsCoreStorageLinks(oldBlockName) {
		w.refreshCoreStorageLinksLocked()
	}
	w.entityEvents = append(w.entityEvents, EntityEvent{
		Kind:       EntityEventBuildDestroyed,
		BuildPos:   packTilePos(tile.X, tile.Y),
		BuildOwner: owner,
		BuildTeam:  teamOld,
		BuildBlock: blockID,
	})
}

func (w *World) CancelBuildPlansPacked(positions []int32) {
	w.CancelBuildPlansPackedForOwner(0, positions)
}

func (w *World) CancelBuildPlansPackedForOwner(owner int32, positions []int32) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.model == nil || len(positions) == 0 {
		return
	}
	for _, packed := range positions {
		x, y := unpackTilePos(packed)
		if !w.model.InBounds(x, y) {
			continue
		}
		pos := int32(y*w.model.Width + x)
		if st, ok := w.pendingBuilds[pos]; ok {
			if owner != 0 && st.Owner != 0 && st.Owner != owner {
				continue
			}
			w.cancelPendingBuildLocked(pos, st)
		}
		if st, ok := w.pendingBreaks[pos]; ok {
			if owner != 0 && st.Owner != 0 && st.Owner != owner {
				continue
			}
			delete(w.pendingBreaks, pos)
		}
	}
}

func (w *World) CancelBuildAt(x, y int32, breaking bool) {
	w.CancelBuildAtForOwner(0, x, y, breaking)
}

func (w *World) CancelBuildAtForOwner(owner int32, x, y int32, breaking bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.model == nil || !w.model.InBounds(int(x), int(y)) {
		return
	}
	pos := int32(int(y)*w.model.Width + int(x))
	if breaking {
		if st, ok := w.pendingBreaks[pos]; ok {
			if owner != 0 && st.Owner != 0 && st.Owner != owner {
				return
			}
			delete(w.pendingBreaks, pos)
		}
		return
	}
	if st, ok := w.pendingBuilds[pos]; ok {
		if owner != 0 && st.Owner != 0 && st.Owner != owner {
			return
		}
		w.cancelPendingBuildLocked(pos, st)
	}
}

func (w *World) CancelBuildPlansByTeam(team TeamID) {
	if team == 0 {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.model == nil {
		return
	}
	for pos, st := range w.pendingBuilds {
		if st.Team != team {
			continue
		}
		w.cancelPendingBuildLocked(pos, st)
	}
	for pos, st := range w.pendingBreaks {
		if st.Team == team {
			delete(w.pendingBreaks, pos)
		}
	}
}

func (w *World) CancelBuildPlansByOwner(owner int32) {
	if owner == 0 {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.cancelBuildPlansByOwnerLocked(owner)
}

func (w *World) cancelBuildPlansByOwnerLocked(owner int32) {
	if owner == 0 || w.model == nil {
		return
	}
	delete(w.builderStates, owner)
	if w.model == nil {
		return
	}
	for pos, st := range w.pendingBuilds {
		if st.Owner != owner {
			continue
		}
		w.cancelPendingBuildLocked(pos, st)
	}
	for pos, st := range w.pendingBreaks {
		if st.Owner == owner {
			delete(w.pendingBreaks, pos)
		}
	}
}

func (w *World) SetEntityMotion(id int32, vx, vy, rotVel float32) (RawEntity, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.model == nil {
		return RawEntity{}, false
	}
	for i := range w.model.Entities {
		if w.model.Entities[i].ID != id {
			continue
		}
		e := &w.model.Entities[i]
		e.VelX = vx
		e.VelY = vy
		e.RotVel = rotVel
		w.model.EntitiesRev++
		return *e, true
	}
	return RawEntity{}, false
}

func (w *World) SetEntityPosition(id int32, x, y, rotation float32) (RawEntity, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.model == nil {
		return RawEntity{}, false
	}
	for i := range w.model.Entities {
		if w.model.Entities[i].ID != id {
			continue
		}
		e := &w.model.Entities[i]
		e.X = x
		e.Y = y
		e.Rotation = rotation
		w.model.EntitiesRev++
		return *e, true
	}
	return RawEntity{}, false
}

func (w *World) SetEntityPlayerController(id, playerID int32) (RawEntity, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.model == nil {
		return RawEntity{}, false
	}
	for i := range w.model.Entities {
		if w.model.Entities[i].ID != id {
			continue
		}
		e := &w.model.Entities[i]
		e.PlayerID = playerID
		w.model.EntitiesRev++
		return *e, true
	}
	return RawEntity{}, false
}

func (w *World) SetEntityLife(id int32, lifeSec float32) (RawEntity, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.model == nil {
		return RawEntity{}, false
	}
	for i := range w.model.Entities {
		if w.model.Entities[i].ID != id {
			continue
		}
		e := &w.model.Entities[i]
		if lifeSec <= 0 {
			e.LifeSec = 0
			e.AgeSec = 0
		} else {
			e.LifeSec = lifeSec
			if e.AgeSec > e.LifeSec {
				e.AgeSec = e.LifeSec
			}
		}
		w.model.EntitiesRev++
		return *e, true
	}
	return RawEntity{}, false
}

func (w *World) SetEntityFollow(id int32, targetID int32, speed float32) (RawEntity, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.model == nil {
		return RawEntity{}, false
	}
	for i := range w.model.Entities {
		if w.model.Entities[i].ID != id {
			continue
		}
		e := &w.model.Entities[i]
		e.Behavior = "follow"
		e.TargetID = targetID
		e.PatrolToB = false
		e.MoveSpeed = speed
		w.model.EntitiesRev++
		return *e, true
	}
	return RawEntity{}, false
}

func (w *World) SetEntityMoveTo(id int32, x, y, speed float32) (RawEntity, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.model == nil {
		return RawEntity{}, false
	}
	for i := range w.model.Entities {
		if w.model.Entities[i].ID != id {
			continue
		}
		e := &w.model.Entities[i]
		e.Behavior = "move"
		e.TargetID = 0
		e.PatrolAX = x
		e.PatrolAY = y
		e.PatrolBX = 0
		e.PatrolBY = 0
		e.PatrolToB = false
		e.MoveSpeed = speed
		w.model.EntitiesRev++
		return *e, true
	}
	return RawEntity{}, false
}

func (w *World) SetEntityPatrol(id int32, ax, ay, bx, by, speed float32) (RawEntity, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.model == nil {
		return RawEntity{}, false
	}
	for i := range w.model.Entities {
		if w.model.Entities[i].ID != id {
			continue
		}
		e := &w.model.Entities[i]
		e.Behavior = "patrol"
		e.TargetID = 0
		e.PatrolAX = ax
		e.PatrolAY = ay
		e.PatrolBX = bx
		e.PatrolBY = by
		e.PatrolToB = true
		e.MoveSpeed = speed
		w.model.EntitiesRev++
		return *e, true
	}
	return RawEntity{}, false
}

func (w *World) ClearEntityBehavior(id int32) (RawEntity, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.model == nil {
		return RawEntity{}, false
	}
	for i := range w.model.Entities {
		if w.model.Entities[i].ID != id {
			continue
		}
		e := &w.model.Entities[i]
		e.Behavior = ""
		e.TargetID = 0
		e.VelX = 0
		e.VelY = 0
		e.RotVel = 0
		w.model.EntitiesRev++
		return *e, true
	}
	return RawEntity{}, false
}

func (w *World) SetEntityCommandIdle(id int32) (RawEntity, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.model == nil {
		return RawEntity{}, false
	}
	for i := range w.model.Entities {
		if w.model.Entities[i].ID != id {
			continue
		}
		e := &w.model.Entities[i]
		e.Behavior = "command"
		e.TargetID = 0
		e.PatrolAX = 0
		e.PatrolAY = 0
		e.PatrolBX = 0
		e.PatrolBY = 0
		e.PatrolToB = false
		e.VelX = 0
		e.VelY = 0
		e.RotVel = 0
		e.MoveSpeed = 0
		w.model.EntitiesRev++
		return *e, true
	}
	return RawEntity{}, false
}

func (w *World) DrainEntityEvents() []EntityEvent {
	return w.DrainEntityEventsInto(nil)
}

func (w *World) DrainEntityEventsInto(dst []EntityEvent) []EntityEvent {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.entityEvents) == 0 {
		return dst[:0]
	}
	dst = append(dst[:0], w.entityEvents...)
	w.entityEvents = w.entityEvents[:0]
	return dst
}

