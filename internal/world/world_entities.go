package world

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/IYanHua/mdt-server/internal/vanilla"
)

func (w *World) UpdateBuilderState(owner int32, team TeamID, unitID int32, x, y float32, active bool, buildRange float32) {
	if w == nil || owner == 0 {
		return
	}
	if buildRange <= 0 {
		buildRange = vanillaBuilderRange
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.builderStates == nil {
		w.builderStates = map[int32]builderRuntimeState{}
	}
	w.builderStates[owner] = builderRuntimeState{
		Owner:      owner,
		Team:       team,
		UnitID:     unitID,
		X:          x,
		Y:          y,
		Active:     active,
		BuildRange: buildRange,
		UpdatedAt:  time.Now(),
	}
}

func (w *World) ClearBuilderState(owner int32) {
	if w == nil || owner == 0 {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.builderStates, owner)
}

func (w *World) HasPendingPlansForOwner(owner int32) bool {
	if w == nil || owner == 0 {
		return false
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, st := range w.pendingBuilds {
		if st.Owner == owner {
			return true
		}
	}
	for _, st := range w.pendingBreaks {
		if st.Owner == owner {
			return true
		}
	}
	return false
}

func (w *World) builderCanActLocked(owner int32, team TeamID, tile *Tile) bool {
	if owner == 0 || tile == nil {
		return true
	}
	state, ok := w.builderStates[owner]
	if !ok {
		return false
	}
	if !state.Active {
		return false
	}
	if state.Team != 0 && team != 0 && state.Team != team {
		return false
	}
	rules := w.rulesMgr.Get()
	if rules != nil && (rules.Editor || rules.InfiniteResources || rules.teamInfiniteResources(team)) {
		return true
	}
	rangeLimit := state.BuildRange
	if rangeLimit <= 0 {
		rangeLimit = vanillaBuilderRange
	}
	tx := float32(tile.X*8 + 4)
	ty := float32(tile.Y*8 + 4)
	dx := tx - state.X
	dy := ty - state.Y
	return dx*dx+dy*dy <= rangeLimit*rangeLimit
}

func (w *World) pendingConstructContributorSpeedLocked(pos int32, tile *Tile, owner int32, team TeamID, breaking bool, blockID int16, rotation int8) float32 {
	if w == nil || w.model == nil || tile == nil || team == 0 {
		return 0
	}
	total := float32(0)
	if owner == 0 || w.builderCanActLocked(owner, team, tile) {
		total += w.builderSpeedForOwnerLocked(owner, team)
	}
	for candidateOwner, state := range w.builderStates {
		if candidateOwner == owner || state.Team != 0 && state.Team != team {
			continue
		}
		if !w.builderCanActLocked(candidateOwner, team, tile) {
			continue
		}
		entity, ok := w.entityByIDLocked(state.UnitID)
		if !ok || entity.Team != team || entity.Health <= 0 || entity.BuildSpeed <= 0 || !entity.UpdateBuilding || len(entity.Plans) == 0 {
			continue
		}
		plan, ok := primaryAssistBuildPlan(entity)
		if !ok || plan.Breaking != breaking || int32(tile.X) != plan.X || int32(tile.Y) != plan.Y {
			continue
		}
		if !breaking {
			if plan.BlockID != blockID || int8(plan.Rotation) != rotation {
				continue
			}
		}
		total += w.builderSpeedForOwnerLocked(candidateOwner, team)
	}
	return total
}

func (w *World) entityByIDLocked(id int32) (RawEntity, bool) {
	if w == nil || w.model == nil || id == 0 {
		return RawEntity{}, false
	}
	for _, entity := range w.model.Entities {
		if entity.ID != id {
			continue
		}
		return entity, true
	}
	return RawEntity{}, false
}

func (w *World) appendBuildCancelledLocked(pos int32, st pendingBuildState) {
	if !st.VisualPlaced {
		return
	}
	x := int(pos % int32(w.model.Width))
	y := int(pos / int32(w.model.Width))
	w.entityEvents = append(w.entityEvents, EntityEvent{
		Kind:       EntityEventBuildCancelled,
		BuildPos:   packTilePos(x, y),
		BuildOwner: st.Owner,
		BuildTeam:  st.Team,
		BuildBlock: st.BlockID,
	})
}

func (w *World) cancelPendingBuildLocked(pos int32, st pendingBuildState) {
	delete(w.pendingBuilds, pos)
	w.refundPendingBuildConsumedLocked(st)
	w.appendBuildCancelledLocked(pos, st)
}

func (w *World) stepPendingBuilds(delta time.Duration) {
	if w.model == nil || len(w.pendingBuilds) == 0 {
		return
	}
	dt := float32(delta.Seconds())
	if dt <= 0 {
		return
	}
	activePosByOwner := make(map[int32]int32, len(w.pendingBuilds))
	activeOrderByOwner := make(map[int32]uint64, len(w.pendingBuilds))
	for pos, st := range w.pendingBuilds {
		if st.Team == 0 {
			continue
		}
		ownerKey := st.Owner
		if ownerKey == 0 {
			ownerKey = -1 - int32(st.Team)
		}
		if st.QueueOrder == 0 {
			w.nextPlanOrder++
			st.QueueOrder = w.nextPlanOrder
			w.pendingBuilds[pos] = st
		}
		if curOrder, ok := activeOrderByOwner[ownerKey]; !ok || st.QueueOrder < curOrder {
			activeOrderByOwner[ownerKey] = st.QueueOrder
			activePosByOwner[ownerKey] = pos
		}
	}
	earliestBreakByOwner := make(map[int32]uint64, len(w.pendingBreaks))
	for _, st := range w.pendingBreaks {
		if st.Team == 0 {
			continue
		}
		ownerKey := st.Owner
		if ownerKey == 0 {
			ownerKey = -1 - int32(st.Team)
		}
		if cur, ok := earliestBreakByOwner[ownerKey]; !ok || st.QueueOrder < cur {
			earliestBreakByOwner[ownerKey] = st.QueueOrder
		}
	}
	rules := w.rulesMgr.Get()
	for owner, pos := range activePosByOwner {
		st, ok := w.pendingBuilds[pos]
		if !ok {
			continue
		}
		if breakOrder, ok := earliestBreakByOwner[owner]; ok && breakOrder < st.QueueOrder {
			continue
		}
		x := int(pos % int32(w.model.Width))
		y := int(pos / int32(w.model.Width))
		if !w.model.InBounds(x, y) {
			delete(w.pendingBuilds, pos)
			continue
		}
		tile, err := w.model.TileAt(x, y)
		if err != nil || tile == nil {
			delete(w.pendingBuilds, pos)
			continue
		}
		builderSpeed := w.pendingConstructContributorSpeedLocked(pos, tile, st.Owner, st.Team, false, st.BlockID, st.Rotation)
		if builderSpeed <= 0 {
			continue
		}
		w.ensurePendingBuildCostStateLocked(&st)
		buildDuration := w.buildDurationSecondsForBuilderSpeedLocked(st.BlockID, st.Team, rules, builderSpeed)
		progressBefore := clampf(st.Progress, 0, 1)
		progressStep := dt / buildDuration
		if progressStep > 0 {
			progressStep = w.applyVanillaBuildCostStepLocked(st.Team, &st, progressStep)
		}
		if !st.VisualPlaced {
			shouldVisualPlace := progressStep > 0
			if !shouldVisualPlace && st.Owner != 0 && w.builderCanActLocked(st.Owner, st.Team, tile) {
				shouldVisualPlace = true
			}
			if !shouldVisualPlace {
				w.pendingBuilds[pos] = st
				continue
			}
			w.entityEvents = append(w.entityEvents, EntityEvent{
				Kind:        EntityEventBuildPlaced,
				BuildPos:    packTilePos(tile.X, tile.Y),
				BuildOwner:  st.Owner,
				BuildTeam:   st.Team,
				BuildBlock:  st.BlockID,
				BuildRot:    st.Rotation,
				BuildConfig: st.Config,
			})
			st.VisualPlaced = true
			st.LastHP = 1
			w.entityEvents = append(w.entityEvents, EntityEvent{
				Kind:     EntityEventBuildHealth,
				BuildPos: packTilePos(tile.X, tile.Y),
				BuildHP:  st.LastHP,
			})
		}
		st.Progress = clampf(st.Progress+progressStep, 0, 1)
		hpNow := constructBlockHealthMax * clampf(st.Progress, 0, 1)
		if hpNow < 1 {
			hpNow = 1
		}
		if hpNow-st.LastHP >= 1 || st.Progress >= 1 {
			st.LastHP = hpNow
			w.entityEvents = append(w.entityEvents, EntityEvent{
				Kind:     EntityEventBuildHealth,
				BuildPos: packTilePos(tile.X, tile.Y),
				BuildHP:  hpNow,
			})
		}
		if st.Progress < 1 {
			w.pendingBuilds[pos] = st
			continue
		}
		if !w.finishPendingBuildCostLocked(st.Team, &st) {
			if progressBefore != st.Progress {
				w.pendingBuilds[pos] = st
				continue
			}
			w.pendingBuilds[pos] = st
			continue
		}
		placed := w.placeCompletedBuildingLocked(pos, tile, st.Team, st.BlockID, st.Rotation, st.Config)
		w.entityEvents = append(w.entityEvents, EntityEvent{
			Kind:     EntityEventBuildHealth,
			BuildPos: packTilePos(tile.X, tile.Y),
			BuildHP:  tile.Build.Health,
		}, EntityEvent{
			Kind:        EntityEventBuildConstructed,
			BuildPos:    packTilePos(tile.X, tile.Y),
			BuildOwner:  st.Owner,
			BuildTeam:   st.Team,
			BuildBlock:  st.BlockID,
			BuildRot:    st.Rotation,
			BuildConfig: placed.Config,
		})
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
		delete(w.pendingBuilds, pos)
	}
	// REMOVED: rebuildActiveTilesLocked() - use incremental indexing instead
}

func (w *World) stepPendingBreaks(delta time.Duration) {
	if w.model == nil || len(w.pendingBreaks) == 0 {
		return
	}
	dt := float32(delta.Seconds())
	if dt <= 0 {
		return
	}
	activePosByOwner := make(map[int32]int32, len(w.pendingBreaks))
	activeOrderByOwner := make(map[int32]uint64, len(w.pendingBreaks))
	for pos, st := range w.pendingBreaks {
		if st.Team == 0 {
			continue
		}
		ownerKey := st.Owner
		if ownerKey == 0 {
			ownerKey = -1 - int32(st.Team)
		}
		if st.QueueOrder == 0 {
			w.nextPlanOrder++
			st.QueueOrder = w.nextPlanOrder
			w.pendingBreaks[pos] = st
		}
		if curOrder, ok := activeOrderByOwner[ownerKey]; !ok || st.QueueOrder < curOrder {
			activeOrderByOwner[ownerKey] = st.QueueOrder
			activePosByOwner[ownerKey] = pos
		}
	}
	earliestBuildByOwner := make(map[int32]uint64, len(w.pendingBuilds))
	for _, st := range w.pendingBuilds {
		if st.Team == 0 {
			continue
		}
		ownerKey := st.Owner
		if ownerKey == 0 {
			ownerKey = -1 - int32(st.Team)
		}
		if cur, ok := earliestBuildByOwner[ownerKey]; !ok || st.QueueOrder < cur {
			earliestBuildByOwner[ownerKey] = st.QueueOrder
		}
	}
	rules := w.rulesMgr.Get()
	for owner, pos := range activePosByOwner {
		st, ok := w.pendingBreaks[pos]
		if !ok {
			continue
		}
		if buildOrder, ok := earliestBuildByOwner[owner]; ok && buildOrder < st.QueueOrder {
			continue
		}
		x := int(pos % int32(w.model.Width))
		y := int(pos / int32(w.model.Width))
		if !w.model.InBounds(x, y) {
			delete(w.pendingBreaks, pos)
			continue
		}
		tile, err := w.model.TileAt(x, y)
		if err != nil || tile == nil || tile.Block == 0 {
			delete(w.pendingBreaks, pos)
			continue
		}
		builderSpeed := w.pendingConstructContributorSpeedLocked(pos, tile, st.Owner, st.Team, true, st.BlockID, st.Rotation)
		if builderSpeed <= 0 {
			continue
		}
		breakDuration := w.buildDurationSecondsForBuilderSpeedLocked(st.BlockID, st.Team, rules, builderSpeed)
		if breakDuration < float32(1.0/60.0) {
			breakDuration = float32(1.0 / 60.0)
		}
		if !st.VisualStart {
			w.entityEvents = append(w.entityEvents, EntityEvent{
				Kind:       EntityEventBuildDeconstructing,
				BuildPos:   packTilePos(tile.X, tile.Y),
				BuildOwner: st.Owner,
				BuildTeam:  st.Team,
				BuildBlock: st.BlockID,
				BuildRot:   st.Rotation,
			})
			st.VisualStart = true
		}
		amount := dt / breakDuration
		progressBefore := clampf(st.Progress, 0, 1)
		clampedAmount := amount
		if remaining := 1 - progressBefore; clampedAmount > remaining {
			clampedAmount = remaining
		}
		if clampedAmount > 0 {
			st.RefundAccum, st.RefundTotal, st.Refunded = w.applyVanillaDeconstructRefundStepLocked(
				st.RefundTeam, st.RefundCost, clampedAmount, st.RefundAccum, st.RefundTotal, st.Refunded,
			)
		}
		st.Progress += amount
		progress := clampf(st.Progress, 0, 1)
		hpNow := st.MaxHealth * (1 - progress)
		if hpNow < 0 {
			hpNow = 0
		}
		if tile.Build != nil {
			tile.Build.Health = hpNow
		}
		if st.LastHP-hpNow >= 1 || hpNow <= 0 || st.Progress >= 1 {
			st.LastHP = hpNow
			w.entityEvents = append(w.entityEvents, EntityEvent{
				Kind:     EntityEventBuildHealth,
				BuildPos: packTilePos(tile.X, tile.Y),
				BuildHP:  hpNow,
			})
		}
		if st.Progress < 1 {
			w.pendingBreaks[pos] = st
			continue
		}
		st.Refunded = w.finishVanillaDeconstructRefundLocked(st.RefundTeam, st.RefundCost, st.Refunded)
		teamOld := tile.Team
		if tile.Build != nil && tile.Build.Team != 0 {
			teamOld = tile.Build.Team
		}
		if teamOld == 0 {
			teamOld = st.Team
		}
		// CRITICAL: Remove from indices BEFORE clearing tile data
		// Otherwise removeActiveTileIndexLocked cannot identify the building type
		w.removeActiveTileIndexLocked(pos, tile)
		w.setBuildingOccupancyLocked(pos, tile, false)
		tile.Build = nil
		tile.Block = 0
		tile.Team = 0
		tile.Rotation = 0
		delete(w.buildStates, pos)
		w.entityEvents = append(w.entityEvents, EntityEvent{
			Kind:       EntityEventBuildDestroyed,
			BuildPos:   packTilePos(tile.X, tile.Y),
			BuildOwner: st.Owner,
			BuildTeam:  teamOld,
			BuildBlock: st.BlockID,
		})
		delete(w.pendingBreaks, pos)
	}
	// REMOVED: rebuildActiveTilesLocked() - use incremental indexing instead
}

func normalizeUnitName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "_", "")
	name = strings.ReplaceAll(name, "-", "")
	name = strings.ReplaceAll(name, " ", "")
	return name
}

// ResolveUnitTypeID accepts either a numeric type id string or a unit name
// like "alpha", "mono", "nova" and resolves it to type id.
func (w *World) ResolveUnitTypeID(arg string) (int16, bool) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return 0, false
	}
	if v, err := strconv.ParseInt(arg, 10, 16); err == nil {
		return int16(v), true
	}
	want := normalizeUnitName(arg)
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.resolveUnitTypeIDLocked(want)
}

func (w *World) resolveUnitTypeIDLocked(want string) (int16, bool) {
	for id, name := range w.unitNamesByID {
		if normalizeUnitName(name) == want {
			return id, true
		}
	}
	switch want {
	case "alpha":
		return 35, true
	case "beta":
		return 36, true
	case "gamma":
		return 37, true
	case "evoke":
		return 53, true
	case "incite":
		return 54, true
	case "emanate":
		return 55, true
	}
	return 0, false
}

func (w *World) UnitNameByTypeID(typeID int16) string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.unitNamesByID == nil {
		return ""
	}
	return w.unitNamesByID[typeID]
}

func (w *World) Model() *WorldModel {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.model
}

func (w *World) Bounds() (int, int, bool) {
	if w == nil {
		return 0, 0, false
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.model == nil || w.model.Width <= 0 || w.model.Height <= 0 {
		return 0, 0, false
	}
	return w.model.Width, w.model.Height, true
}

func (w *World) RulesTagRaw() string {
	if w == nil {
		return ""
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.model == nil || w.model.Tags == nil {
		return ""
	}
	return strings.TrimSpace(w.model.Tags["rules"])
}

func (w *World) CloneModel() *WorldModel {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.model == nil {
		return nil
	}
	return w.model.Clone()
}

func (w *World) BlockNameByID(blockID int16) string {
	if w == nil {
		return ""
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.blockNameByID(blockID)
}

func (w *World) AddEntity(typeID int16, x, y float32, team TeamID) (RawEntity, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.model == nil {
		return RawEntity{}, ErrOutOfBounds
	}
	ent := RawEntity{
		TypeID:              typeID,
		X:                   x,
		Y:                   y,
		Team:                team,
		Health:              100,
		MaxHealth:           100,
		Shield:              0,
		ShieldMax:           0,
		ShieldRegen:         0,
		Armor:               0,
		SlowMul:             1,
		StatusDamageMul:     1,
		StatusHealthMul:     1,
		StatusSpeedMul:      1,
		StatusReloadMul:     1,
		StatusBuildSpeedMul: 1,
		StatusDragMul:       1,
		StatusArmorOverride: -1,
		RuntimeInit:         true,
		MineTilePos:         invalidEntityTilePos,
	}
	w.applyUnitTypeDef(&ent)
	w.applyWeaponProfile(&ent)
	if isEntityFlying(ent) {
		ent.Elevation = 1
	}
	return w.model.AddEntity(ent), nil
}

func (w *World) ReserveEntityID() int32 {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.model == nil {
		return 0
	}
	if w.model.NextEntityID <= 0 {
		w.model.NextEntityID = 1
	}
	id := w.model.NextEntityID
	w.model.NextEntityID++
	return id
}

func (w *World) AddEntityWithID(typeID int16, id int32, x, y float32, team TeamID) (RawEntity, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.model == nil {
		return RawEntity{}, ErrOutOfBounds
	}
	for i := range w.model.Entities {
		if w.model.Entities[i].ID == id {
			return RawEntity{}, ErrEntityExists
		}
	}
	ent := RawEntity{
		TypeID:              typeID,
		ID:                  id,
		X:                   x,
		Y:                   y,
		Health:              100,
		MaxHealth:           100,
		Shield:              0,
		ShieldMax:           0,
		ShieldRegen:         0,
		Armor:               0,
		SlowMul:             1,
		StatusDamageMul:     1,
		StatusHealthMul:     1,
		StatusSpeedMul:      1,
		StatusReloadMul:     1,
		StatusBuildSpeedMul: 1,
		StatusDragMul:       1,
		StatusArmorOverride: -1,
		RuntimeInit:         true,
		MineTilePos:         invalidEntityTilePos,
		Team:                team,
	}
	w.applyUnitTypeDef(&ent)
	w.applyWeaponProfile(&ent)
	if isEntityFlying(ent) {
		ent.Elevation = 1
	}
	return w.model.AddEntity(ent), nil
}

func (w *World) RemoveEntity(id int32) (RawEntity, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.removeEntityLocked(id)
}

func (w *World) GetEntity(id int32) (RawEntity, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.model == nil {
		return RawEntity{}, false
	}
	for i := range w.model.Entities {
		if w.model.Entities[i].ID == id {
			return w.model.Entities[i], true
		}
	}
	return RawEntity{}, false
}

func (w *World) TeamItems(team TeamID) map[ItemID]int32 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if items := w.teamCoreItemsLocked(team); items != nil {
		return items
	}
	src := w.teamItems[team]
	if len(src) == 0 {
		return map[ItemID]int32{}
	}
	out := make(map[ItemID]int32, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func (w *World) TeamCoreItemSnapshots() []TeamCoreItemSnapshot {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.model == nil {
		return nil
	}
	teams := make(map[TeamID]map[ItemID]int32, len(w.teamCoreTiles))
	for team, positions := range w.teamCoreTiles {
		if team == 0 || len(positions) == 0 {
			continue
		}
		items := make(map[ItemID]int32)
		for _, pos := range positions {
			if pos < 0 || int(pos) >= len(w.model.Tiles) {
				continue
			}
			tile := &w.model.Tiles[pos]
			if tile.Build == nil || tile.Block <= 0 {
				continue
			}
			for _, stack := range tile.Build.Items {
				if stack.Amount <= 0 {
					continue
				}
				items[stack.Item] += stack.Amount
			}
		}
		teams[team] = items
	}
	if len(teams) == 0 {
		return nil
	}
	order := make([]int, 0, len(teams))
	for team := range teams {
		order = append(order, int(team))
	}
	sort.Ints(order)
	out := make([]TeamCoreItemSnapshot, 0, len(order))
	for _, rawTeam := range order {
		team := TeamID(rawTeam)
		itemMap := teams[team]
		itemIDs := make([]int, 0, len(itemMap))
		for item, amount := range itemMap {
			if amount > 0 {
				itemIDs = append(itemIDs, int(item))
			}
		}
		sort.Ints(itemIDs)
		items := make([]ItemStack, 0, len(itemIDs))
		for _, rawItem := range itemIDs {
			item := ItemID(rawItem)
			items = append(items, ItemStack{Item: item, Amount: itemMap[item]})
		}
		out = append(out, TeamCoreItemSnapshot{Team: team, Items: items})
	}
	return out
}

func (w *World) TeamItemSyncPositions(team TeamID) []int32 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.model == nil || team == 0 {
		return nil
	}
	positions := make([]int32, 0, 8)
	seen := make(map[int32]struct{})
	for _, pos := range w.teamCoreTiles[team] {
		if pos < 0 || int(pos) >= len(w.model.Tiles) {
			continue
		}
		if w.blockSyncSuppressedLocked(pos) {
			continue
		}
		tile := &w.model.Tiles[pos]
		if tile.Team != team || tile.Build == nil || tile.Block == 0 {
			continue
		}
		packed := packTilePos(tile.X, tile.Y)
		if _, ok := seen[packed]; !ok {
			seen[packed] = struct{}{}
			positions = append(positions, packed)
		}
	}
	for pos, corePos := range w.storageLinkedCore {
		if corePos < 0 || int(corePos) >= len(w.model.Tiles) || pos < 0 || int(pos) >= len(w.model.Tiles) {
			continue
		}
		if w.blockSyncSuppressedLocked(pos) || w.blockSyncSuppressedLocked(corePos) {
			continue
		}
		coreTile := &w.model.Tiles[corePos]
		storageTile := &w.model.Tiles[pos]
		if coreTile.Team != team || storageTile.Team != team || storageTile.Build == nil || storageTile.Block == 0 {
			continue
		}
		packed := packTilePos(storageTile.X, storageTile.Y)
		if _, ok := seen[packed]; !ok {
			seen[packed] = struct{}{}
			positions = append(positions, packed)
		}
	}
	sort.Slice(positions, func(i, j int) bool { return positions[i] < positions[j] })
	return positions
}

func unitBuildSpeedByName(name string) float32 {
	switch normalizeUnitName(name) {
	case "alpha":
		return 0.5
	case "beta":
		return 0.75
	case "gamma":
		return 1.0
	case "evoke":
		return 1.2
	case "incite":
		return 1.4
	case "emanate":
		return 1.5
	default:
		return 0.5
	}
}

func constructBreakStartHealth(tile *Tile) float32 {
	if tile == nil || tile.Build == nil {
		return constructBlockHealthMax
	}
	maxHealth := tile.Build.MaxHealth
	if maxHealth <= 0 {
		maxHealth = tile.Build.Health
	}
	if maxHealth <= 0 {
		return constructBlockHealthMax
	}
	start := constructBlockHealthMax * clampf(tile.Build.Health/maxHealth, 0, 1)
	if start < 0 {
		return 0
	}
	if start > constructBlockHealthMax {
		return constructBlockHealthMax
	}
	return start
}

func (w *World) builderSpeedForUnitTypeLocked(typeID int16) float32 {
	if prof, ok := w.unitRuntimeProfileForTypeLocked(typeID); ok && prof.BuildSpeed > 0 {
		return prof.BuildSpeed
	}
	name := ""
	if w.unitNamesByID != nil {
		name = w.unitNamesByID[typeID]
	}
	if strings.TrimSpace(name) == "" {
		name = fallbackUnitNameByTypeID(typeID)
	}
	return unitBuildSpeedByName(name)
}

func fallbackUnitNameByTypeID(typeID int16) string {
	switch typeID {
	case 35:
		return "alpha"
	case 36:
		return "beta"
	case 37:
		return "gamma"
	case 53:
		return "evoke"
	case 54:
		return "incite"
	case 55:
		return "emanate"
	default:
		return ""
	}
}

func fallbackCoreUnitTypeDef(name string) (vanilla.UnitTypeDef, bool) {
	switch normalizeUnitName(name) {
	case "alpha":
		return vanilla.UnitTypeDef{Name: "alpha", Health: 150, Speed: 3, HitSize: 8, RotateSpeed: 15}, true
	case "beta":
		return vanilla.UnitTypeDef{Name: "beta", Health: 170, Speed: 3.3, HitSize: 9, RotateSpeed: 17}, true
	case "gamma":
		return vanilla.UnitTypeDef{Name: "gamma", Health: 220, Speed: 3.55, HitSize: 11, RotateSpeed: 19}, true
	case "evoke":
		return vanilla.UnitTypeDef{Name: "evoke", Health: 300, Armor: 1, Speed: 5.6, HitSize: 9, RotateSpeed: 15}, true
	case "incite":
		return vanilla.UnitTypeDef{Name: "incite", Health: 500, Armor: 2, Speed: 7, HitSize: 11, RotateSpeed: 17}, true
	case "emanate":
		return vanilla.UnitTypeDef{Name: "emanate", Health: 700, Armor: 3, Speed: 7.5, HitSize: 12, RotateSpeed: 19}, true
	default:
		return vanilla.UnitTypeDef{}, false
	}
}

func (w *World) BuilderSpeedForUnitType(typeID int16) float32 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	name := ""
	if w.unitNamesByID != nil {
		name = w.unitNamesByID[typeID]
	}
	if strings.TrimSpace(name) == "" {
		name = fallbackUnitNameByTypeID(typeID)
	}
	return unitBuildSpeedByName(name)
}

func (w *World) SetTeamBuilderSpeed(team TeamID, speed float32) {
	if team == 0 || speed <= 0 {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.teamBuilderSpeed == nil {
		w.teamBuilderSpeed = make(map[TeamID]float32)
	}
	w.teamBuilderSpeed[team] = speed
}

func (w *World) TeamCorePositions(team TeamID) []int32 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.model == nil || team == 0 {
		return nil
	}
	positions := w.teamCoreTiles[team]
	out := make([]int32, 0, len(positions))
	for _, pos := range positions {
		if pos < 0 || int(pos) >= len(w.model.Tiles) {
			continue
		}
		t := &w.model.Tiles[pos]
		if t.Block > 0 {
			out = append(out, packTilePos(t.X, t.Y))
		}
	}
	return out
}

func (w *World) BuildSyncSnapshot() []BuildSyncState {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.model == nil || len(w.model.Tiles) == 0 {
		return nil
	}
	out := make([]BuildSyncState, 0, len(w.activeTilePositions))
	for _, pos := range w.activeTilePositions {
		if pos < 0 || int(pos) >= len(w.model.Tiles) {
			continue
		}
		if w.blockSyncSuppressedLocked(pos) {
			continue
		}
		t := w.model.Tiles[pos]
		if !isCenterBuildingTile(&t) {
			continue
		}
		hp := float32(1000)
		if t.Build != nil && t.Build.Health > 0 {
			hp = t.Build.Health
		}
		team := t.Team
		if t.Build != nil {
			team = t.Build.Team
		}
		out = append(out, BuildSyncState{
			Pos:      packTilePos(t.X, t.Y),
			X:        int32(t.X),
			Y:        int32(t.Y),
			BlockID:  int16(t.Block),
			Team:     team,
			Rotation: t.Rotation,
			Health:   hp,
		})
	}
	return out
}

func (w *World) BuildSyncSnapshotWithConfig() []BuildSyncSnapshotEntry {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.model == nil || len(w.model.Tiles) == 0 {
		return nil
	}
	out := make([]BuildSyncSnapshotEntry, 0, len(w.activeTilePositions))
	for _, pos := range w.activeTilePositions {
		if pos < 0 || int(pos) >= len(w.model.Tiles) {
			continue
		}
		if w.blockSyncSuppressedLocked(pos) {
			continue
		}
		t := w.model.Tiles[pos]
		if !isCenterBuildingTile(&t) {
			continue
		}
		hp := float32(1000)
		if t.Build != nil && t.Build.Health > 0 {
			hp = t.Build.Health
		}
		team := t.Team
		if t.Build != nil {
			team = t.Build.Team
		}
		entry := BuildSyncSnapshotEntry{
			BuildSyncState: BuildSyncState{
				Pos:      packTilePos(t.X, t.Y),
				X:        int32(t.X),
				Y:        int32(t.Y),
				BlockID:  int16(t.Block),
				Team:     team,
				Rotation: t.Rotation,
				Health:   hp,
			},
		}
		if t.Build != nil && len(t.Build.Config) > 0 {
			entry.Config = append([]byte(nil), t.Build.Config...)
		}
		out = append(out, entry)
	}
	return out
}

