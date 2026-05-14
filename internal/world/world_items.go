package world

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/IYanHua/mdt-server/internal/protocol"
)

const conveyorItemSpace = float32(0.4)

func isConveyorBlock(name string) bool {
	switch name {
	case "conveyor", "titanium-conveyor", "armored-conveyor":
		return true
	default:
		return false
	}
}

func isDuctBlock(name string) bool {
	switch name {
	case "duct", "armored-duct":
		return true
	default:
		return false
	}
}

func isStackConveyorBlock(name string) bool {
	switch name {
	case "plastanium-conveyor", "surge-conveyor":
		return true
	default:
		return false
	}
}

func isRotatingTransportBlock(name string) bool {
	switch name {
	case "conveyor", "titanium-conveyor", "armored-conveyor", "bridge-conveyor", "sorter", "inverted-sorter", "overflow-gate", "underflow-gate", "duct", "armored-duct", "duct-router", "overflow-duct", "underflow-duct", "duct-bridge", "duct-unloader", "plastanium-conveyor", "surge-conveyor", "surge-router":
		return true
	default:
		return false
	}
}

func isInstantTransferBlock(name string) bool {
	switch name {
	case "sorter", "inverted-sorter", "overflow-gate", "underflow-gate":
		return true
	default:
		return false
	}
}

func isRouterOrInstantTransferBlock(name string) bool {
	return isRouterBlock(name) || isInstantTransferBlock(name)
}

func isRouterBlock(name string) bool {
	return name == "router" || name == "distributor"
}

func isDuctRouterBlock(name string) bool {
	return name == "duct-router" || name == "surge-router"
}

func isItemBridgeBlock(name string) bool {
	return name == "bridge-conveyor" || name == "phase-conveyor" || name == "bridge-conduit" || name == "phase-conduit"
}

func (w *World) conveyorStateLocked(pos int32, tile *Tile) *conveyorRuntimeState {
	if st, ok := w.conveyorStates[pos]; ok && st != nil {
		return st
	}
	st := &conveyorRuntimeState{
		LastInserted: -1,
		MinItem:      1,
	}
	if tile != nil && tile.Build != nil {
		index := 0
		for _, stack := range tile.Build.Items {
			for amount := int32(0); amount < stack.Amount && index < len(st.IDs); amount++ {
				st.IDs[index] = stack.Item
				st.XS[index] = 0
				st.YS[index] = float32(index) * conveyorItemSpace
				index++
			}
			if index >= len(st.IDs) {
				break
			}
		}
		st.Len = index
		if st.Len > 0 {
			st.MinItem = st.YS[0]
		}
	}
	w.conveyorStates[pos] = st
	return st
}

func (w *World) syncConveyorInventoryLocked(pos int32, tile *Tile, st *conveyorRuntimeState) {
	if tile == nil || tile.Build == nil || st == nil {
		return
	}
	if st.Len <= 0 {
		if len(tile.Build.Items) != 0 {
			w.replaceBuildingItemsLocked(pos, tile, nil)
		}
		st.Len = 0
		st.MinItem = 1
		st.LastInserted = -1
		st.Mid = 0
		return
	}

	var (
		ids    [3]ItemID
		counts [3]int32
		used   int
	)
	for i := 0; i < st.Len; i++ {
		item := st.IDs[i]
		matched := false
		for j := 0; j < used; j++ {
			if ids[j] == item {
				counts[j]++
				matched = true
				break
			}
		}
		if !matched && used < len(ids) {
			ids[used] = item
			counts[used] = 1
			used++
		}
	}

	if len(tile.Build.Items) == used {
		matches := true
		for i := 0; i < used; i++ {
			found := false
			for _, stack := range tile.Build.Items {
				if stack.Item == ids[i] && stack.Amount == counts[i] {
					found = true
					break
				}
			}
			if !found {
				matches = false
				break
			}
		}
		if matches {
			return
		}
	}

	var items [3]ItemStack
	for i := 0; i < used; i++ {
		items[i] = ItemStack{Item: ids[i], Amount: counts[i]}
	}
	w.replaceBuildingItemsLocked(pos, tile, items[:used])
}

func (st *conveyorRuntimeState) add(index int) {
	if st == nil || st.Len >= len(st.IDs) {
		return
	}
	if index < 0 {
		index = 0
	}
	if index > st.Len {
		index = st.Len
	}
	for i := st.Len; i > index; i-- {
		st.IDs[i] = st.IDs[i-1]
		st.XS[i] = st.XS[i-1]
		st.YS[i] = st.YS[i-1]
	}
	st.Len++
}

func (st *conveyorRuntimeState) remove(index int) {
	if st == nil || index < 0 || index >= st.Len {
		return
	}
	for i := index; i < st.Len-1; i++ {
		st.IDs[i] = st.IDs[i+1]
		st.XS[i] = st.XS[i+1]
		st.YS[i] = st.YS[i+1]
	}
	st.Len--
	st.LastInserted = -1
	if st.Len >= 0 && st.Len < len(st.IDs) {
		st.IDs[st.Len] = 0
		st.XS[st.Len] = 0
		st.YS[st.Len] = 0
	}
}

func (w *World) conveyorAcceptsItemLocked(fromPos, toPos int32) bool {
	if w.model == nil || fromPos < 0 || toPos < 0 || int(fromPos) >= len(w.model.Tiles) || int(toPos) >= len(w.model.Tiles) {
		return false
	}
	toTile := &w.model.Tiles[toPos]
	st := w.conveyorStateLocked(toPos, toTile)
	if st.Len >= len(st.IDs) {
		return false
	}
	sourceSide, ok := w.flowDirBetweenLocked(fromPos, toPos)
	if !ok {
		return false
	}
	fromTile := &w.model.Tiles[fromPos]
	targetName := w.blockNameByID(int16(toTile.Block))
	if targetName == "armored-conveyor" {
		sourceName := w.blockNameByID(int16(fromTile.Block))
		if !isConveyorBlock(sourceName) && sourceSide != byte(((int(toTile.Rotation)%4)+4)%4) {
			return false
		}
	}
	direction := absInt(int(sourceSide) - int(toTile.Rotation))
	if direction == 0 {
		if nextPos, ok := w.forwardPosLocked(toPos, toTile.Rotation); ok && nextPos == fromPos && isRotatingTransportBlock(w.blockNameByID(int16(fromTile.Block))) {
			return false
		}
		return st.MinItem >= conveyorItemSpace
	}
	return direction%2 == 1 && st.MinItem > 0.7
}

func (w *World) conveyorHandleItemLocked(fromPos, toPos int32, item ItemID) bool {
	if !w.conveyorAcceptsItemLocked(fromPos, toPos) {
		return false
	}
	toTile := &w.model.Tiles[toPos]
	st := w.conveyorStateLocked(toPos, toTile)
	sourceSide, _ := w.flowDirBetweenLocked(fromPos, toPos)
	ang := int(sourceSide) - int(toTile.Rotation)
	x := float32(0)
	if ang == -1 || ang == 3 {
		x = 1
	} else if ang == 1 || ang == -3 {
		x = -1
	}
	if absInt(ang) == 0 {
		st.add(0)
		st.IDs[0] = item
		st.XS[0] = x
		st.YS[0] = 0
		st.LastInserted = 0
	} else {
		index := st.Mid
		if index < 0 {
			index = 0
		}
		if index > st.Len {
			index = st.Len
		}
		st.add(index)
		st.IDs[index] = item
		st.XS[index] = x
		st.YS[index] = 0.5
		st.LastInserted = index
	}
	st.MinItem = minf(st.MinItem, st.YS[st.LastInserted])
	w.syncConveyorInventoryLocked(toPos, toTile, st)
	return true
}

func (w *World) ductAcceptsItemLocked(fromPos, toPos int32, armored bool) bool {
	if w.model == nil || fromPos < 0 || toPos < 0 || int(fromPos) >= len(w.model.Tiles) || int(toPos) >= len(w.model.Tiles) {
		return false
	}
	toTile := &w.model.Tiles[toPos]
	if toTile.Build == nil || totalBuildingItems(toTile.Build) > 0 {
		return false
	}
	sourceDir, ok := w.flowDirBetweenLocked(fromPos, toPos)
	if !ok {
		return false
	}
	if armored {
		if sourceDir == byte(tileRotationNorm(toTile.Rotation)) {
			return true
		}
		fromTile := &w.model.Tiles[fromPos]
		name := w.blockNameByID(int16(fromTile.Block))
		if isDuctBlock(name) || isDuctRouterBlock(name) || name == "overflow-duct" || name == "underflow-duct" || name == "duct-bridge" || name == "duct-unloader" {
			if nextPos, ok := w.forwardPosLocked(fromPos, fromTile.Rotation); ok && nextPos == toPos {
				return true
			}
		}
		return false
	}
	return sourceDir != byte((tileRotationNorm(toTile.Rotation)+2)%4)
}

func (w *World) ductHandleItemLocked(fromPos, toPos int32, item ItemID, armored bool) bool {
	if !w.ductAcceptsItemLocked(fromPos, toPos, armored) {
		return false
	}
	toTile := &w.model.Tiles[toPos]
	st := w.ductStateLocked(toPos, toTile)
	sourceDir, _ := w.flowDirBetweenLocked(fromPos, toPos)
	if !w.addItemAtLocked(toPos, item, 1) {
		return false
	}
	st.Current = item
	st.HasItem = true
	st.Progress = -1
	st.RecDir = sourceDir
	return true
}

func (w *World) ductRouterAcceptsItemLocked(fromPos, toPos int32, item ItemID) bool {
	if w.model == nil || fromPos < 0 || toPos < 0 || int(fromPos) >= len(w.model.Tiles) || int(toPos) >= len(w.model.Tiles) {
		return false
	}
	toTile := &w.model.Tiles[toPos]
	if toTile.Build == nil {
		return false
	}
	if sourceDir, ok := w.flowDirBetweenLocked(fromPos, toPos); !ok || sourceDir != byte(tileRotationNorm(toTile.Rotation)) {
		return false
	}
	name := w.blockNameByID(int16(toTile.Block))
	cap := w.itemCapacityForBlockLocked(toTile)
	if totalBuildingItems(toTile.Build) >= cap {
		return false
	}
	if name == "surge-router" {
		st := w.stackStateLocked(toPos, toTile)
		return !st.Unloading && (!st.HasItem || st.LastItem == item)
	}
	st := w.ductStateLocked(toPos, toTile)
	return !st.HasItem && totalBuildingItems(toTile.Build) == 0
}

func (w *World) ductRouterHandleItemLocked(fromPos, toPos int32, item ItemID, stack bool) bool {
	if !w.ductRouterAcceptsItemLocked(fromPos, toPos, item) {
		return false
	}
	toTile := &w.model.Tiles[toPos]
	if !w.addItemAtLocked(toPos, item, 1) {
		return false
	}
	if stack {
		sst := w.stackStateLocked(toPos, toTile)
		sst.LastItem = item
		sst.HasItem = true
		sst.Link = toPos
		sst.Unloading = false
	}
	st := w.ductStateLocked(toPos, toTile)
	sourceDir, _ := w.flowDirBetweenLocked(fromPos, toPos)
	st.Current = item
	st.HasItem = true
	st.Progress = -1
	st.RecDir = sourceDir
	return true
}

func (w *World) ductBridgeAcceptsItemLocked(fromPos, toPos int32, item ItemID) bool {
	if w.model == nil || fromPos < 0 || toPos < 0 || int(fromPos) >= len(w.model.Tiles) || int(toPos) >= len(w.model.Tiles) {
		return false
	}
	toTile := &w.model.Tiles[toPos]
	if toTile.Build == nil || totalBuildingItems(toTile.Build) >= w.itemCapacityForBlockLocked(toTile) {
		return false
	}
	if _, ok := w.directionBridgeTargetLocked(toPos, toTile, "duct-bridge", 4); !ok {
		return false
	}
	rel, ok := w.relativeToEdgeLocked(fromPos, toPos)
	if !ok || rel == byte(tileRotationNorm(toTile.Rotation)) {
		return false
	}
	incomingDir, ok := w.flowDirBetweenLocked(fromPos, toPos)
	if !ok {
		return false
	}
	for _, otherPos := range w.itemDuctTilePositions {
		if otherPos == fromPos || otherPos == toPos || otherPos < 0 || int(otherPos) >= len(w.model.Tiles) {
			continue
		}
		other := &w.model.Tiles[otherPos]
		if other.Build == nil || other.Team != toTile.Team || w.blockNameByID(int16(other.Block)) != "duct-bridge" {
			continue
		}
		target, ok := w.directionBridgeTargetLocked(otherPos, other, "duct-bridge", 4)
		if !ok || target != toPos {
			continue
		}
		dir, ok := w.flowDirBetweenLocked(otherPos, toPos)
		if ok && dir == incomingDir {
			return false
		}
	}
	_ = item
	return true
}

func (w *World) stackConveyorAcceptsItemLocked(fromPos, toPos int32, item ItemID) bool {
	if w.model == nil || fromPos < 0 || toPos < 0 || int(fromPos) >= len(w.model.Tiles) || int(toPos) >= len(w.model.Tiles) {
		return false
	}
	toTile := &w.model.Tiles[toPos]
	if toTile.Build == nil {
		return false
	}
	st := w.stackStateLocked(toPos, toTile)
	if fromPos == toPos {
		if totalBuildingItems(toTile.Build) >= w.itemCapacityForBlockLocked(toTile) {
			return false
		}
		return !st.HasItem || st.LastItem == item
	}
	if st.Cooldown > 1 {
		return false
	}
	if !w.stackConveyorIsLoadingStateLocked(toPos, toTile) {
		return false
	}
	if totalBuildingItems(toTile.Build) >= w.itemCapacityForBlockLocked(toTile) {
		return false
	}
	if st.HasItem && st.LastItem != item {
		return false
	}
	if nextPos, ok := w.forwardPosLocked(toPos, toTile.Rotation); ok && nextPos == fromPos {
		return false
	}
	return true
}

func (w *World) stackConveyorHandleItemLocked(fromPos, toPos int32, item ItemID) bool {
	if !w.stackConveyorAcceptsItemLocked(fromPos, toPos, item) {
		return false
	}
	toTile := &w.model.Tiles[toPos]
	if !w.addItemAtLocked(toPos, item, 1) {
		return false
	}
	st := w.stackStateLocked(toPos, toTile)
	st.LastItem = item
	st.HasItem = true
	if st.Link < 0 {
		st.Link = toPos
	}
	return true
}

func (w *World) stackConveyorHasFrontStackLocked(pos int32, tile *Tile) (int32, *Tile, bool) {
	if w == nil || w.model == nil || tile == nil {
		return 0, nil, false
	}
	frontPos, ok := w.forwardPosLocked(pos, tile.Rotation)
	if !ok || frontPos < 0 || int(frontPos) >= len(w.model.Tiles) {
		return 0, nil, false
	}
	frontTile := &w.model.Tiles[frontPos]
	if frontTile.Build == nil || frontTile.Team != tile.Team || !isStackConveyorBlock(w.blockNameByID(int16(frontTile.Block))) {
		return 0, nil, false
	}
	return frontPos, frontTile, true
}

func (w *World) stackConveyorHasBackStackLocked(pos int32, tile *Tile) (int32, *Tile, bool) {
	if w == nil || w.model == nil || tile == nil {
		return 0, nil, false
	}
	backPos, ok := w.forwardPosLocked(pos, int8((int(tile.Rotation)+2)%4))
	if !ok || backPos < 0 || int(backPos) >= len(w.model.Tiles) {
		return 0, nil, false
	}
	backTile := &w.model.Tiles[backPos]
	if backTile.Build == nil || backTile.Team != tile.Team || !isStackConveyorBlock(w.blockNameByID(int16(backTile.Block))) {
		return 0, nil, false
	}
	return backPos, backTile, true
}

func (w *World) stackConveyorIsUnloadingStateLocked(pos int32, tile *Tile) bool {
	if w == nil || tile == nil || tile.Build == nil || tile.Block == 0 {
		return false
	}
	name := w.blockNameByID(int16(tile.Block))
	if !isStackConveyorBlock(name) {
		return false
	}
	_, _, hasFrontStack := w.stackConveyorHasFrontStackLocked(pos, tile)
	if name == "plastanium-conveyor" {
		_, _, hasBackStack := w.stackConveyorHasBackStackLocked(pos, tile)
		return !hasFrontStack && hasBackStack
	}
	return !hasFrontStack
}

func (w *World) stackConveyorIsLoadingStateLocked(pos int32, tile *Tile) bool {
	if w == nil || tile == nil || tile.Build == nil || tile.Block == 0 {
		return false
	}
	name := w.blockNameByID(int16(tile.Block))
	if !isStackConveyorBlock(name) {
		return false
	}
	_, _, hasFrontStack := w.stackConveyorHasFrontStackLocked(pos, tile)
	if !hasFrontStack {
		return false
	}
	backPos, backTile, hasBackStack := w.stackConveyorHasBackStackLocked(pos, tile)
	loading := !hasBackStack || w.stackConveyorIsUnloadingStateLocked(backPos, backTile)
	if !loading {
		return false
	}
	for _, otherPos := range w.dumpProximityLocked(pos) {
		if otherPos < 0 || int(otherPos) >= len(w.model.Tiles) || otherPos == pos {
			continue
		}
		other := &w.model.Tiles[otherPos]
		if other.Build == nil || other.Team != tile.Team || !isStackConveyorBlock(w.blockNameByID(int16(other.Block))) {
			continue
		}
		if nextPos, ok := w.forwardPosLocked(otherPos, other.Rotation); ok && nextPos == pos {
			return false
		}
	}
	return true
}

func (w *World) stackConveyorCanUnloadLocked(pos int32, tile *Tile) bool {
	return !w.stackConveyorIsLoadingStateLocked(pos, tile)
}

func tileRotationNorm(rotation int8) int {
	return ((int(rotation) % 4) + 4) % 4
}

func (w *World) routerStateLocked(pos int32, tile *Tile) *routerRuntimeState {
	if st, ok := w.routerStates[pos]; ok && st != nil {
		if !st.HasItem && tile != nil && tile.Build != nil {
			if item, ok := firstBuildingItem(tile.Build); ok {
				st.LastItem = item
				st.HasItem = true
			}
		}
		return st
	}
	st := &routerRuntimeState{LastInput: -1}
	if tile != nil && tile.Build != nil {
		if item, ok := firstBuildingItem(tile.Build); ok {
			st.LastItem = item
			st.HasItem = true
		}
	}
	w.routerStates[pos] = st
	return st
}

func isItemLogisticsBlockName(name string) bool {
	switch name {
	case "conveyor", "titanium-conveyor", "armored-conveyor",
		"duct", "armored-duct", "duct-router", "overflow-duct", "underflow-duct", "duct-bridge", "duct-unloader",
		"router", "distributor",
		"bridge-conveyor", "phase-conveyor",
		"plastanium-conveyor", "surge-conveyor", "surge-router",
		"unloader", "mass-driver":
		return true
	default:
		return false
	}
}

func isItemConveyorBlockName(name string) bool {
	switch name {
	case "conveyor", "titanium-conveyor", "armored-conveyor", "plastanium-conveyor", "surge-conveyor":
		return true
	default:
		return false
	}
}

func isItemDuctBlockName(name string) bool {
	switch name {
	case "duct", "armored-duct", "duct-router", "overflow-duct", "underflow-duct", "duct-bridge", "duct-unloader":
		return true
	default:
		return false
	}
}

func isItemRouterBlockName(name string) bool {
	switch name {
	case "router", "distributor", "surge-router":
		return true
	default:
		return false
	}
}

func isItemBridgeBlockName(name string) bool {
	switch name {
	case "bridge-conveyor", "phase-conveyor":
		return true
	default:
		return false
	}
}

func isItemUnloaderBlockName(name string) bool {
	return name == "unloader"
}

func isLiquidConduitBlockName(name string) bool {
	switch name {
	case "conduit", "pulse-conduit", "plated-conduit", "reinforced-conduit":
		return true
	default:
		return false
	}
}

func isLiquidStorageBlockName(name string) bool {
	switch name {
	case "liquid-router", "liquid-container", "liquid-tank",
		"reinforced-liquid-router", "reinforced-liquid-container", "reinforced-liquid-tank":
		return true
	default:
		return false
	}
}

func isLiquidBridgeBlockName(name string) bool {
	switch name {
	case "bridge-conduit", "phase-conduit", "reinforced-bridge-conduit":
		return true
	default:
		return false
	}
}

func isPayloadFactoryBlockName(name string) bool {
	switch name {
	case "ground-factory", "air-factory", "naval-factory":
		return true
	default:
		return false
	}
}

func isPayloadTransportBlockName(name string) bool {
	switch name {
	case "payload-conveyor", "reinforced-payload-conveyor",
		"payload-router", "reinforced-payload-router",
		"payload-void",
		"small-deconstructor", "deconstructor", "payload-deconstructor",
		"payload-mass-driver", "large-payload-mass-driver",
		"payload-loader", "payload-unloader":
		return true
	default:
		return false
	}
}

func (w *World) stepItemLogistics(delta time.Duration, profileDetails bool) itemLogisticsPerf {
	var perf itemLogisticsPerf
	if w.model == nil {
		return perf
	}
	dt := float32(delta.Seconds())
	if dt <= 0 {
		return perf
	}
	var junctionStartedAt time.Time
	if profileDetails {
		junctionStartedAt = time.Now()
	}
	w.stepJunctions(dt)
	if profileDetails {
		perf.Junctions = time.Since(junctionStartedAt)
		perf.JunctionCount = len(w.junctionQueues)
	}
	var conveyorStartedAt time.Time
	if profileDetails {
		conveyorStartedAt = time.Now()
	}
	for _, pos := range w.itemConveyorTilePositions {
		if pos < 0 || int(pos) >= len(w.model.Tiles) {
			continue
		}
		tile := &w.model.Tiles[pos]
		if tile.Build == nil || tile.Block == 0 {
			continue
		}
		switch w.blockNameByID(int16(tile.Block)) {
		case "conveyor":
			w.stepConveyorLocked(pos, tile, 0.03, dt)
		case "titanium-conveyor", "armored-conveyor":
			w.stepConveyorLocked(pos, tile, 0.08, dt)
		case "plastanium-conveyor":
			w.stepStackConveyorLocked(pos, tile, 4.0/60.0, 2, true, dt)
		case "surge-conveyor":
			w.stepStackConveyorLocked(pos, tile, 5.0/60.0, 2, false, dt)
		}
	}
	if profileDetails {
		perf.Conveyor += time.Since(conveyorStartedAt)
		perf.ConveyorCount += len(w.itemConveyorTilePositions)
	}
	var ductStartedAt time.Time
	if profileDetails {
		ductStartedAt = time.Now()
	}
	for _, pos := range w.itemDuctTilePositions {
		if pos < 0 || int(pos) >= len(w.model.Tiles) {
			continue
		}
		tile := &w.model.Tiles[pos]
		if tile.Build == nil || tile.Block == 0 {
			continue
		}
		switch w.blockNameByID(int16(tile.Block)) {
		case "duct":
			var startedAt time.Time
			if profileDetails {
				startedAt = time.Now()
			}
			w.stepDuctLocked(pos, tile, 4, false, dt)
			if profileDetails {
				perf.Duct += time.Since(startedAt)
				perf.DuctCount++
			}
		case "armored-duct":
			var startedAt time.Time
			if profileDetails {
				startedAt = time.Now()
			}
			w.stepDuctLocked(pos, tile, 4, true, dt)
			if profileDetails {
				perf.Duct += time.Since(startedAt)
				perf.DuctCount++
			}
		case "duct-router":
			var startedAt time.Time
			if profileDetails {
				startedAt = time.Now()
			}
			w.stepDuctRouterLocked(pos, tile, 4, false, dt)
			if profileDetails {
				perf.Router += time.Since(startedAt)
				perf.RouterCount++
			}
		case "overflow-duct":
			var startedAt time.Time
			if profileDetails {
				startedAt = time.Now()
			}
			w.stepOverflowDuctLocked(pos, tile, 4, false, dt)
			if profileDetails {
				perf.Router += time.Since(startedAt)
				perf.RouterCount++
			}
		case "underflow-duct":
			var startedAt time.Time
			if profileDetails {
				startedAt = time.Now()
			}
			w.stepOverflowDuctLocked(pos, tile, 4, true, dt)
			if profileDetails {
				perf.Router += time.Since(startedAt)
				perf.RouterCount++
			}
		case "duct-bridge":
			var startedAt time.Time
			if profileDetails {
				startedAt = time.Now()
			}
			w.stepDuctBridgeLocked(pos, tile, 4, dt)
			if profileDetails {
				perf.Bridge += time.Since(startedAt)
				perf.BridgeCount++
			}
		case "duct-unloader":
			var startedAt time.Time
			if profileDetails {
				startedAt = time.Now()
			}
			w.stepDirectionalUnloaderLocked(pos, tile, 4, dt)
			if profileDetails {
				perf.Unloader += time.Since(startedAt)
				perf.UnloaderCount++
			}
		}
	}
	if profileDetails && perf.Duct == 0 {
		perf.Duct = time.Since(ductStartedAt)
	}
	var routerStartedAt time.Time
	if profileDetails {
		routerStartedAt = time.Now()
	}
	for _, pos := range w.itemRouterTilePositions {
		if pos < 0 || int(pos) >= len(w.model.Tiles) {
			continue
		}
		tile := &w.model.Tiles[pos]
		if tile.Build == nil || tile.Block == 0 {
			continue
		}
		switch w.blockNameByID(int16(tile.Block)) {
		case "router", "distributor":
			w.stepRouterLocked(pos, tile, 8, dt)
		case "surge-router":
			w.stepStackRouterLocked(pos, tile, 6, dt)
		}
	}
	if profileDetails {
		perf.Router += time.Since(routerStartedAt)
		perf.RouterCount += len(w.itemRouterTilePositions)
	}
	var bridgeStartedAt time.Time
	if profileDetails {
		bridgeStartedAt = time.Now()
	}
	for _, pos := range w.itemBridgeTilePositions {
		if pos < 0 || int(pos) >= len(w.model.Tiles) {
			continue
		}
		tile := &w.model.Tiles[pos]
		if tile.Build == nil || tile.Block == 0 {
			continue
		}
		switch w.blockNameByID(int16(tile.Block)) {
		case "bridge-conveyor":
			w.stepBridgeConveyorLocked(pos, tile, 11, dt)
		case "phase-conveyor":
			w.stepPhaseConveyorLocked(pos, tile, dt)
		}
	}
	if profileDetails {
		perf.Bridge += time.Since(bridgeStartedAt)
		perf.BridgeCount += len(w.itemBridgeTilePositions)
	}
	var unloaderStartedAt time.Time
	if profileDetails {
		unloaderStartedAt = time.Now()
	}
	for _, pos := range w.itemUnloaderTilePositions {
		if pos < 0 || int(pos) >= len(w.model.Tiles) {
			continue
		}
		tile := &w.model.Tiles[pos]
		if tile.Build == nil || tile.Block == 0 {
			continue
		}
		w.stepUnloaderLocked(pos, tile, dt)
	}
	if profileDetails {
		perf.Unloader += time.Since(unloaderStartedAt)
		perf.UnloaderCount += len(w.itemUnloaderTilePositions)
	}
	var massDriverStartedAt time.Time
	if profileDetails {
		massDriverStartedAt = time.Now()
	}
	for _, pos := range w.itemMassDriverTilePositions {
		if pos < 0 || int(pos) >= len(w.model.Tiles) {
			continue
		}
		tile := &w.model.Tiles[pos]
		if tile.Build == nil || tile.Block == 0 {
			continue
		}
		w.stepMassDriverLocked(pos, tile, dt)
	}
	if profileDetails {
		perf.MassDrive += time.Since(massDriverStartedAt)
		perf.MassDriveCount += len(w.itemMassDriverTilePositions)
	}
	var massShotStartedAt time.Time
	if profileDetails {
		massShotStartedAt = time.Now()
	}
	w.stepMassDriverShotsLocked(dt)
	if profileDetails {
		perf.MassDrive += time.Since(massShotStartedAt)
		perf.MassDriveCount += len(w.massDriverShots)
	}
	return perf
}

func (w *World) payloadStateLocked(pos int32) *payloadRuntimeState {
	if st, ok := w.payloadStates[pos]; ok && st != nil {
		return st
	}
	st := &payloadRuntimeState{}
	w.payloadStates[pos] = st
	return st
}

func (w *World) payloadDriverStateLocked(pos int32) *payloadDriverRuntimeState {
	if st, ok := w.payloadDriverStates[pos]; ok && st != nil {
		return st
	}
	st := &payloadDriverRuntimeState{}
	w.payloadDriverStates[pos] = st
	return st
}

func (w *World) syncPayloadTileLocked(tile *Tile, payload *payloadData) {
	if tile == nil || tile.Build == nil {
		return
	}
	if payload == nil {
		tile.Build.Payload = nil
		return
	}
	serialized, ok := w.serializePayloadDataLocked(payload)
	if !ok || len(serialized) == 0 {
		tile.Build.Payload = nil
		return
	}
	tile.Build.Payload = append(tile.Build.Payload[:0], serialized...)
}

func (w *World) clearPayloadLocked(pos int32, tile *Tile) {
	st := w.payloadStateLocked(pos)
	st.Payload = nil
	st.Move = 0
	st.Work = 0
	st.Exporting = false
	w.syncPayloadTileLocked(tile, nil)
}

func totalPayloadItems(payload *payloadData) int32 {
	if payload == nil {
		return 0
	}
	var total int32
	for _, stack := range payload.Items {
		total += stack.Amount
	}
	return total
}

func totalPayloadLiquids(payload *payloadData) float32 {
	if payload == nil {
		return 0
	}
	var total float32
	for _, stack := range payload.Liquids {
		total += stack.Amount
	}
	return total
}

func payloadAddItem(payload *payloadData, item ItemID, amount int32) {
	if payload == nil || amount <= 0 {
		return
	}
	payload.Serialized = nil
	for i := range payload.Items {
		if payload.Items[i].Item == item {
			payload.Items[i].Amount += amount
			return
		}
	}
	payload.Items = append(payload.Items, ItemStack{Item: item, Amount: amount})
}

func payloadRemoveItem(payload *payloadData, item ItemID, amount int32) bool {
	if payload == nil || amount <= 0 {
		return false
	}
	for i := range payload.Items {
		if payload.Items[i].Item != item {
			continue
		}
		if payload.Items[i].Amount < amount {
			return false
		}
		payload.Serialized = nil
		payload.Items[i].Amount -= amount
		if payload.Items[i].Amount <= 0 {
			payload.Items = append(payload.Items[:i], payload.Items[i+1:]...)
		}
		return true
	}
	return false
}

func payloadAddLiquid(payload *payloadData, liquid LiquidID, amount float32) {
	if payload == nil || amount <= 0 {
		return
	}
	payload.Serialized = nil
	for i := range payload.Liquids {
		if payload.Liquids[i].Liquid == liquid {
			payload.Liquids[i].Amount += amount
			return
		}
	}
	payload.Liquids = append(payload.Liquids, LiquidStack{Liquid: liquid, Amount: amount})
}

func payloadRemoveLiquid(payload *payloadData, liquid LiquidID, amount float32) bool {
	if payload == nil || amount <= 0 {
		return false
	}
	for i := range payload.Liquids {
		if payload.Liquids[i].Liquid != liquid {
			continue
		}
		if payload.Liquids[i].Amount+0.0001 < amount {
			return false
		}
		payload.Serialized = nil
		payload.Liquids[i].Amount -= amount
		if payload.Liquids[i].Amount <= 0.0001 {
			payload.Liquids = append(payload.Liquids[:i], payload.Liquids[i+1:]...)
		}
		return true
	}
	return false
}

func (w *World) payloadSizeBlocksLocked(payload *payloadData) int {
	if payload == nil {
		return 0
	}
	if payload.Kind == payloadKindBlock {
		name := w.blockNameByID(payload.BlockID)
		if name != "" {
			return blockSizeByName(name)
		}
	}
	if payload.Kind == payloadKindUnit {
		size := w.payloadWorldSizeLocked(payload) / 8
		if size > 0 {
			if blocks := int(math.Ceil(float64(size - 0.001))); blocks > 1 {
				return blocks
			}
			return 1
		}
	}
	return 1
}

func (w *World) payloadItemCapacityLocked(payload *payloadData) int32 {
	if payload == nil || payload.Kind != payloadKindBlock {
		return 0
	}
	return w.itemCapacityForBlockLocked(&Tile{Block: BlockID(payload.BlockID)})
}

func (w *World) payloadLiquidCapacityLocked(payload *payloadData) float32 {
	if payload == nil || payload.Kind != payloadKindBlock {
		return 0
	}
	return w.liquidCapacityForBlockLocked(&Tile{Block: BlockID(payload.BlockID)})
}

func (w *World) payloadFilterMatchesLocked(payload *payloadData, filter protocol.Content) bool {
	if payload == nil || filter == nil {
		return false
	}
	switch filter.ContentType() {
	case protocol.ContentBlock:
		return payload.Kind == payloadKindBlock && payload.BlockID == filter.ID()
	case protocol.ContentUnit:
		return payload.Kind == payloadKindUnit && payload.UnitTypeID == filter.ID()
	default:
		return false
	}
}

func isPayloadTransportBlock(name string) bool {
	switch name {
	case "payload-conveyor", "reinforced-payload-conveyor",
		"payload-router", "reinforced-payload-router",
		"payload-mass-driver", "large-payload-mass-driver",
		"payload-loader", "payload-unloader":
		return true
	default:
		return false
	}
}

func payloadMoveTimeByName(name string) float32 {
	switch name {
	case "reinforced-payload-conveyor", "reinforced-payload-router":
		return 35
	default:
		return 45
	}
}

func (w *World) payloadFrontTargetLocked(pos int32, tile *Tile, rotation int8) (int32, bool) {
	if w.model == nil || tile == nil {
		return 0, false
	}
	ntrns := w.blockSizeForTileLocked(tile)/2 + 1
	dx, dy := dirDelta(rotation)
	targetPos, ok := w.buildingOccupyingCellLocked(tile.X+dx*ntrns, tile.Y+dy*ntrns)
	if !ok || targetPos == pos || targetPos < 0 || int(targetPos) >= len(w.model.Tiles) {
		return 0, false
	}
	return targetPos, true
}

func (w *World) payloadAcceptsLocked(fromPos, toPos int32, payload *payloadData) bool {
	if w.model == nil || payload == nil || fromPos < 0 || toPos < 0 || int(fromPos) >= len(w.model.Tiles) || int(toPos) >= len(w.model.Tiles) {
		return false
	}
	fromTile := &w.model.Tiles[fromPos]
	toTile := &w.model.Tiles[toPos]
	if toTile.Build == nil || toTile.Team != fromTile.Team || toTile.Block == 0 {
		return false
	}
	if w.payloadStateLocked(toPos).Payload != nil {
		return false
	}
	size := w.payloadSizeBlocksLocked(payload)
	switch w.blockNameByID(int16(toTile.Block)) {
	case "payload-conveyor", "reinforced-payload-conveyor", "payload-router", "reinforced-payload-router":
		return size <= 3
	case "payload-loader", "payload-unloader":
		return payload.Kind == payloadKindBlock && size <= 3
	case "payload-mass-driver":
		return size <= 2
	case "large-payload-mass-driver":
		return size <= 4
	case "payload-void":
		return w.payloadVoidAcceptsPayloadLocked(toPos)
	case "small-deconstructor", "deconstructor", "payload-deconstructor":
		return w.payloadDeconstructorAcceptsPayloadLocked(toPos, toTile, payload)
	default:
		if isReconstructorBlockName(w.blockNameByID(int16(toTile.Block))) {
			return w.reconstructorAcceptsPayloadLocked(toPos, toTile, payload, fromPos, true)
		}
		return false
	}
}

func (w *World) payloadHandleLocked(fromPos, toPos int32, payload *payloadData) bool {
	if !w.payloadAcceptsLocked(fromPos, toPos, payload) {
		return false
	}
	targetTile := &w.model.Tiles[toPos]
	targetState := w.payloadStateLocked(toPos)
	targetState.Payload = payload
	targetState.Move = 0
	targetState.Work = 0
	targetState.Exporting = false
	if dir, ok := w.flowDirBetweenLocked(fromPos, toPos); ok {
		targetState.RecDir = dir
	} else {
		targetState.RecDir = byte(tileRotationNorm(targetTile.Rotation))
	}
	w.syncPayloadTileLocked(targetTile, payload)
	return true
}

func (w *World) payloadMoveOutLocked(pos int32, tile *Tile, targetPos int32) bool {
	st := w.payloadStateLocked(pos)
	if st.Payload == nil {
		return false
	}
	if !w.payloadHandleLocked(pos, targetPos, st.Payload) {
		return false
	}
	w.clearPayloadLocked(pos, tile)
	return true
}

func (w *World) stepPayloadLogistics(delta time.Duration) {
	if w.model == nil {
		return
	}
	frames := float32(delta.Seconds() * 60)
	if frames <= 0 {
		return
	}
	for _, pos := range w.payloadFactoryTilePositions {
		if pos < 0 || int(pos) >= len(w.model.Tiles) {
			continue
		}
		tile := &w.model.Tiles[pos]
		if tile.Build == nil || tile.Block == 0 {
			continue
		}
		switch w.blockNameByID(int16(tile.Block)) {
		case "ground-factory", "air-factory", "naval-factory":
			w.stepUnitFactoryPayloadLocked(pos, tile, frames)
		default:
			if isReconstructorBlockName(w.blockNameByID(int16(tile.Block))) {
				w.stepReconstructorLocked(pos, tile, frames)
			}
		}
	}
	for _, pos := range w.payloadTransportTiles {
		if pos < 0 || int(pos) >= len(w.model.Tiles) {
			continue
		}
		tile := &w.model.Tiles[pos]
		if tile.Build == nil || tile.Block == 0 {
			continue
		}
		switch w.blockNameByID(int16(tile.Block)) {
		case "payload-conveyor", "reinforced-payload-conveyor":
			w.stepPayloadConveyorLocked(pos, tile, payloadMoveTimeByName(w.blockNameByID(int16(tile.Block))), frames)
		case "payload-router", "reinforced-payload-router":
			w.stepPayloadRouterLocked(pos, tile, payloadMoveTimeByName(w.blockNameByID(int16(tile.Block))), frames)
		case "payload-void":
			w.stepPayloadVoidLocked(pos, tile, frames)
		case "small-deconstructor", "deconstructor", "payload-deconstructor":
			w.stepPayloadDeconstructorLocked(pos, tile, frames)
		case "payload-mass-driver":
			w.stepPayloadMassDriverLocked(pos, tile, 130, 90, frames)
		case "large-payload-mass-driver":
			w.stepPayloadMassDriverLocked(pos, tile, 130, 100, frames)
		case "payload-loader":
			w.stepPayloadLoaderLocked(pos, tile, frames)
		case "payload-unloader":
			w.stepPayloadUnloaderLocked(pos, tile, frames)
		}
	}
	w.stepPayloadDriverShotsLocked(frames)
}

func (w *World) stepUnitFactoryPayloadLocked(pos int32, tile *Tile, frames float32) {
	st := w.payloadStateLocked(pos)
	if st.Payload == nil {
		st.Move = 0
		w.syncPayloadTileLocked(tile, nil)
		return
	}
	moveTime := w.unitBlockPayloadMoveFramesLocked(tile)
	st.Move += frames
	if st.Move < moveTime {
		w.syncPayloadTileLocked(tile, st.Payload)
		return
	}
	if targetPos, ok := w.payloadFrontTargetLocked(pos, tile, tile.Rotation); ok && w.payloadMoveOutLocked(pos, tile, targetPos) {
		return
	}
	if w.dumpUnitPayloadFromTileLocked(pos, tile) {
		return
	}
	st.Move = moveTime
	w.syncPayloadTileLocked(tile, st.Payload)
}

func (w *World) unitBlockPayloadMoveFramesLocked(tile *Tile) float32 {
	size := float32(w.blockSizeForTileLocked(tile))
	// Match PayloadBlock payloadSpeed=0.7f closely enough for server-side timing.
	frames := (size * 8 / 2) / 0.7
	if frames < 1 {
		return 1
	}
	return frames
}

func (w *World) dumpUnitPayloadFromTileLocked(pos int32, tile *Tile) bool {
	if tile == nil || tile.Build == nil || w.model == nil {
		return false
	}
	state := w.payloadStateLocked(pos)
	payload := state.Payload
	if payload == nil || payload.Kind != payloadKindUnit {
		return false
	}
	typeID := payload.UnitTypeID
	if typeID <= 0 {
		if st, ok := w.factoryStates[pos]; ok && st.UnitType > 0 {
			typeID = st.UnitType
		}
	}
	if typeID <= 0 {
		return false
	}
	rotation := float32(tile.Rotation) * 90
	dist := float32(w.blockSizeForTileLocked(tile))*4 + 0.1
	rad := float32(rotation * math.Pi / 180)
	spawnX := float32(tile.X*8+4) + float32(math.Cos(float64(rad)))*dist
	spawnY := float32(tile.Y*8+4) + float32(math.Sin(float64(rad)))*dist
	if !w.canDumpProducedUnitLocked(tile.Build.Team, typeID, spawnX, spawnY, rotation) {
		return false
	}
	ent := w.newProducedUnitEntityLocked(typeID, tile.Build.Team, spawnX, spawnY, rotation)
	commandPos, command := w.unitCommandStateAtLocked(pos)
	if command != nil {
		ent.CommandID = command.ID
	}
	if commandPos != nil {
		ent.Behavior = "move"
		ent.TargetID = 0
		ent.PatrolAX = commandPos.X
		ent.PatrolAY = commandPos.Y
	}
	w.model.AddEntity(ent)
	w.clearPayloadLocked(pos, tile)
	return true
}

func (w *World) canDumpProducedUnitLocked(team TeamID, typeID int16, x, y, rotation float32) bool {
	if team == 0 || typeID <= 0 {
		return false
	}
	counts := map[TeamID]map[int16]int32{
		team: map[int16]int32{typeID: w.teamUnitCountByTypeLocked(team, typeID)},
	}
	if !w.canCreateUnitLocked(team, typeID, w.rulesMgr.Get(), nil, counts) {
		return false
	}
	dx := float32(math.Cos(float64(rotation * math.Pi / 180)))
	dy := float32(math.Sin(float64(rotation * math.Pi / 180)))
	tx := int((x + dx) / 8)
	ty := int((y + dy) / 8)
	if _, ok := w.buildingOccupyingCellLocked(tx, ty); ok {
		return false
	}
	tmp := w.newProducedUnitEntityLocked(typeID, team, x, y, rotation)
	if !isEntityFlying(tmp) {
		selfRadius := tmp.HitRadius
		if selfRadius <= 0 {
			selfRadius = entityHitRadiusForType(typeID)
		}
		maxDist := selfRadius * 1.05
		for _, other := range w.model.Entities {
			if other.Health <= 0 || isEntityFlying(other) {
				continue
			}
			otherRadius := other.HitRadius
			if otherRadius <= 0 {
				otherRadius = entityHitRadiusForType(other.TypeID)
			}
			limit := maxDist + otherRadius*0.5
			dx := other.X - x
			dy := other.Y - y
			if dx*dx+dy*dy <= limit*limit {
				return false
			}
		}
	}
	return true
}

func (w *World) stepPayloadConveyorLocked(pos int32, tile *Tile, moveTime, frames float32) {
	st := w.payloadStateLocked(pos)
	if st.Payload == nil {
		st.Move = 0
		w.syncPayloadTileLocked(tile, nil)
		return
	}
	st.Move += frames
	if st.Move < moveTime {
		w.syncPayloadTileLocked(tile, st.Payload)
		return
	}
	targetPos, ok := w.payloadFrontTargetLocked(pos, tile, tile.Rotation)
	if !ok || !w.payloadMoveOutLocked(pos, tile, targetPos) {
		st.Move = moveTime
		w.syncPayloadTileLocked(tile, st.Payload)
	}
}

func (w *World) stepPayloadRouterLocked(pos int32, tile *Tile, moveTime, frames float32) {
	st := w.payloadStateLocked(pos)
	if st.Payload == nil {
		st.Move = 0
		w.syncPayloadTileLocked(tile, nil)
		return
	}
	st.Move += frames
	if st.Move < moveTime {
		w.syncPayloadTileLocked(tile, st.Payload)
		return
	}
	filter := w.payloadRouterCfg[pos]
	matches := w.payloadFilterMatchesLocked(st.Payload, filter)
	forward := int8(st.RecDir % 4)
	candidates := make([]int8, 0, 4)
	if matches {
		candidates = append(candidates, forward)
	} else {
		start := int8((tileRotationNorm(tile.Rotation) + 1) % 4)
		for i := 0; i < 4; i++ {
			dir := int8((int(start) + i) % 4)
			if filter != nil && dir == forward {
				continue
			}
			candidates = append(candidates, dir)
		}
		if len(candidates) == 0 {
			candidates = append(candidates, forward)
		}
	}
	for _, dir := range candidates {
		targetPos, ok := w.payloadFrontTargetLocked(pos, tile, dir)
		if !ok {
			continue
		}
		if !w.payloadMoveOutLocked(pos, tile, targetPos) {
			continue
		}
		tile.Rotation = dir
		if tile.Build != nil {
			tile.Build.Rotation = dir
		}
		return
	}
	st.Move = moveTime
	w.syncPayloadTileLocked(tile, st.Payload)
}

func (w *World) payloadDriverTargetLocked(pos int32, tile *Tile) (int32, bool) {
	if w.model == nil || tile == nil {
		return 0, false
	}
	target, ok := w.payloadDriverLinks[pos]
	if !ok || target < 0 || int(target) >= len(w.model.Tiles) {
		return 0, false
	}
	targetTile := &w.model.Tiles[target]
	if targetTile.Build == nil || targetTile.Team != tile.Team || w.blockNameByID(int16(targetTile.Block)) != w.blockNameByID(int16(tile.Block)) {
		return 0, false
	}
	return target, true
}

func (w *World) payloadDriverIncomingShotsLocked(pos int32) int {
	count := 0
	for _, shot := range w.payloadDriverShots {
		if shot.ToPos == pos {
			count++
		}
	}
	return count
}

func (w *World) stepPayloadMassDriverLocked(pos int32, tile *Tile, reloadFrames, chargeFrames, frames float32) {
	st := w.payloadStateLocked(pos)
	driver := w.payloadDriverStateLocked(pos)
	if driver.ReloadCounter > 0 {
		driver.ReloadCounter = maxf(0, driver.ReloadCounter-frames)
	}
	if st.Payload == nil {
		driver.Charge = 0
		w.syncPayloadTileLocked(tile, nil)
		return
	}
	size := w.payloadSizeBlocksLocked(st.Payload)
	switch w.blockNameByID(int16(tile.Block)) {
	case "payload-mass-driver":
		if size > 2 {
			driver.Charge = 0
			w.syncPayloadTileLocked(tile, st.Payload)
			return
		}
	case "large-payload-mass-driver":
		if size > 4 {
			driver.Charge = 0
			w.syncPayloadTileLocked(tile, st.Payload)
			return
		}
	}
	target, ok := w.payloadDriverTargetLocked(pos, tile)
	if !ok || driver.ReloadCounter > 0 {
		driver.Charge = 0
		w.syncPayloadTileLocked(tile, st.Payload)
		return
	}
	targetState := w.payloadStateLocked(target)
	targetDriver := w.payloadDriverStateLocked(target)
	if targetState.Payload != nil || targetDriver.ReloadCounter > 0 || w.payloadDriverIncomingShotsLocked(target) > 0 {
		driver.Charge = 0
		w.syncPayloadTileLocked(tile, st.Payload)
		return
	}
	powerUse := float32(0.5)
	if w.blockNameByID(int16(tile.Block)) == "large-payload-mass-driver" {
		powerUse = 3
	}
	if !w.requirePowerAtLocked(pos, tile.Team, powerUse*(frames/60)) {
		w.syncPayloadTileLocked(tile, st.Payload)
		return
	}
	driver.Charge += frames
	if driver.Charge < chargeFrames {
		w.syncPayloadTileLocked(tile, st.Payload)
		return
	}
	targetTile := &w.model.Tiles[target]
	dx := float32(targetTile.X-tile.X) * 8
	dy := float32(targetTile.Y-tile.Y) * 8
	travelFrames := float32(math.Sqrt(float64(dx*dx+dy*dy)) / 5.5)
	if travelFrames < 1 {
		travelFrames = 1
	}
	w.payloadDriverShots = append(w.payloadDriverShots, payloadDriverShot{
		FromPos:      pos,
		ToPos:        target,
		TravelFrames: travelFrames,
		Payload:      st.Payload,
	})
	driver.Charge = 0
	driver.ReloadCounter = reloadFrames
	w.clearPayloadLocked(pos, tile)
}

func (w *World) stepPayloadDriverShotsLocked(frames float32) {
	if len(w.payloadDriverShots) == 0 {
		return
	}
	kept := w.payloadDriverShots[:0]
	for _, shot := range w.payloadDriverShots {
		shot.AgeFrames += frames
		if shot.AgeFrames < shot.TravelFrames {
			kept = append(kept, shot)
			continue
		}
		if w.model == nil || shot.ToPos < 0 || int(shot.ToPos) >= len(w.model.Tiles) {
			continue
		}
		targetTile := &w.model.Tiles[shot.ToPos]
		if targetTile.Build == nil {
			continue
		}
		if w.payloadStateLocked(shot.ToPos).Payload != nil {
			shot.AgeFrames = shot.TravelFrames
			kept = append(kept, shot)
			continue
		}
		state := w.payloadStateLocked(shot.ToPos)
		state.Payload = shot.Payload
		state.Move = 0
		state.Work = 0
		state.Exporting = false
		w.syncPayloadTileLocked(targetTile, shot.Payload)
	}
	w.payloadDriverShots = kept
}

func (w *World) stepPayloadLoaderLocked(pos int32, tile *Tile, frames float32) {
	st := w.payloadStateLocked(pos)
	if st.Payload == nil {
		st.Work = 0
		w.syncPayloadTileLocked(tile, nil)
		return
	}
	moved := false
	holding := false
	itemCap := w.payloadItemCapacityLocked(st.Payload)
	if itemCap > 0 && totalBuildingItems(tile.Build) > 0 && totalPayloadItems(st.Payload) < itemCap {
		holding = true
		st.Work += frames
		for st.Work >= 2 {
			transferred := false
			for j := 0; j < 8; j++ {
				item, ok := firstBuildingItem(tile.Build)
				if !ok || totalPayloadItems(st.Payload) >= itemCap {
					break
				}
				if !tile.Build.RemoveItem(item, 1) {
					break
				}
				payloadAddItem(st.Payload, item, 1)
				moved = true
				transferred = true
			}
			st.Work -= 2
			if !transferred {
				break
			}
		}
	}
	if liquid, _, ok := firstBuildingLiquid(tile.Build); ok {
		if liquidCap := w.payloadLiquidCapacityLocked(st.Payload); liquidCap > 0 {
			space := liquidCap - totalPayloadLiquids(st.Payload)
			available := tile.Build.LiquidAmount(liquid)
			if space > 0 && available > 0 {
				holding = true
				flow := minf(space, minf(available, 40*frames))
				if flow > 0 && tile.Build.RemoveLiquid(liquid, flow) {
					payloadAddLiquid(st.Payload, liquid, flow)
					moved = true
				}
			}
		}
	}
	if holding || moved {
		w.syncPayloadTileLocked(tile, st.Payload)
		return
	}
	if !moved {
		targetPos, ok := w.payloadFrontTargetLocked(pos, tile, tile.Rotation)
		if ok {
			_ = w.payloadMoveOutLocked(pos, tile, targetPos)
		}
		return
	}
}

func (w *World) stepPayloadUnloaderLocked(pos int32, tile *Tile, frames float32) {
	st := w.payloadStateLocked(pos)
	if st.Payload == nil {
		st.Work = 0
		w.syncPayloadTileLocked(tile, nil)
		return
	}
	moved := false
	holding := false
	itemCap := w.itemCapacityForBlockLocked(tile)
	if itemCap > 0 && totalPayloadItems(st.Payload) > 0 && totalBuildingItems(tile.Build) < itemCap {
		holding = true
		st.Work += frames
		for st.Work >= 2 {
			transferred := false
			for j := 0; j < 8; j++ {
				if totalBuildingItems(tile.Build) >= itemCap || len(st.Payload.Items) == 0 {
					break
				}
				item := st.Payload.Items[0].Item
				if !payloadRemoveItem(st.Payload, item, 1) {
					break
				}
				tile.Build.AddItem(item, 1)
				moved = true
				transferred = true
			}
			st.Work -= 2
			if !transferred {
				break
			}
		}
	}
	if liquidCap := w.liquidCapacityForBlockLocked(tile); liquidCap > 0 && len(st.Payload.Liquids) > 0 {
		liq := st.Payload.Liquids[0].Liquid
		space := liquidCap - totalBuildingLiquids(tile.Build)
		available := st.Payload.Liquids[0].Amount
		if space > 0 && available > 0 {
			holding = true
			flow := minf(space, minf(available, 40*frames))
			if flow > 0 && payloadRemoveLiquid(st.Payload, liq, flow) {
				tile.Build.AddLiquid(liq, flow)
				moved = true
			}
		}
	}
	if holding || moved {
		w.syncPayloadTileLocked(tile, st.Payload)
		return
	}
	if !moved && totalPayloadItems(st.Payload) == 0 && totalPayloadLiquids(st.Payload) == 0 {
		targetPos, ok := w.payloadFrontTargetLocked(pos, tile, tile.Rotation)
		if ok {
			_ = w.payloadMoveOutLocked(pos, tile, targetPos)
		}
		return
	}
}

func (w *World) stepConveyorLocked(pos int32, tile *Tile, speed float32, dt float32) {
	if tile == nil || tile.Build == nil || speed <= 0 {
		return
	}
	st := w.conveyorStateLocked(pos, tile)
	st.MinItem = 1
	st.Mid = 0
	if st.Len == 0 {
		w.syncConveyorInventoryLocked(pos, tile, st)
		return
	}

	var (
		nextState *conveyorRuntimeState
		nextPos   int32
		hasNext   bool
		aligned   bool
		nextMax   = float32(1)
	)
	if outPos, ok := w.forwardItemTargetPosLocked(pos, tile.Rotation); ok {
		nextPos = outPos
		hasNext = true
		nextTile := &w.model.Tiles[nextPos]
		if nextTile.Build != nil && nextTile.Team == tile.Team && isConveyorBlock(w.blockNameByID(int16(nextTile.Block))) {
			nextState = w.conveyorStateLocked(nextPos, nextTile)
			aligned = nextTile.Rotation == tile.Rotation
			if aligned {
				nextMax = 1 - maxf(conveyorItemSpace-nextState.MinItem, 0)
			}
		}
	}

	moved := speed * dt * 60
	for i := st.Len - 1; i >= 0; i-- {
		nextPosY := float32(100)
		if i < st.Len-1 {
			nextPosY = st.YS[i+1] - conveyorItemSpace
		}
		maxMove := clampf(nextPosY-st.YS[i], 0, moved)
		st.YS[i] += maxMove
		if st.YS[i] > nextMax {
			st.YS[i] = nextMax
		}
		if st.YS[i] > 0.5 && i > 0 {
			st.Mid = i - 1
		}
		st.XS[i] = approachf(st.XS[i], 0, moved*2)

		if st.YS[i] >= 1 {
			item := st.IDs[i]
			xcarry := st.XS[i]
			if hasNext && w.tryInsertItemLocked(pos, nextPos, item, 0) {
				if aligned && nextState != nil && nextState.LastInserted >= 0 && nextState.LastInserted < nextState.Len {
					nextState.XS[nextState.LastInserted] = xcarry
					w.syncConveyorInventoryLocked(nextPos, &w.model.Tiles[nextPos], nextState)
				}
				st.remove(i)
				continue
			}
		}
		if st.YS[i] < st.MinItem {
			st.MinItem = st.YS[i]
		}
	}
	if st.Len == 0 {
		st.MinItem = 1
	}
	w.syncConveyorInventoryLocked(pos, tile, st)
}

func (w *World) ductStateLocked(pos int32, tile *Tile) *ductRuntimeState {
	if st, ok := w.ductStates[pos]; ok && st != nil {
		if !st.HasItem && tile != nil && tile.Build != nil {
			if item, ok := firstBuildingItem(tile.Build); ok {
				st.Current = item
				st.HasItem = true
			}
		}
		return st
	}
	st := &ductRuntimeState{}
	if tile != nil && tile.Build != nil {
		if item, ok := firstBuildingItem(tile.Build); ok {
			st.Current = item
			st.HasItem = true
		}
	}
	w.ductStates[pos] = st
	return st
}

func (w *World) stackStateLocked(pos int32, tile *Tile) *stackRuntimeState {
	if st, ok := w.stackStates[pos]; ok && st != nil {
		if tile != nil && tile.Build != nil {
			if item, ok := firstBuildingItem(tile.Build); ok {
				st.LastItem = item
				st.HasItem = true
			} else {
				st.HasItem = false
			}
		}
		return st
	}
	st := &stackRuntimeState{Link: -1}
	if tile != nil && tile.Build != nil {
		if item, ok := firstBuildingItem(tile.Build); ok {
			st.LastItem = item
			st.HasItem = true
			st.Link = pos
		}
	}
	w.stackStates[pos] = st
	return st
}

func (w *World) stepDuctLocked(pos int32, tile *Tile, speed float32, armored bool, dt float32) {
	if tile == nil || tile.Build == nil || speed <= 0 {
		return
	}
	st := w.ductStateLocked(pos, tile)
	if !st.HasItem {
		st.Progress = 0
		return
	}
	nextPos, ok := w.forwardItemTargetPosLocked(pos, tile.Rotation)
	if !ok {
		st.Progress = 0
		return
	}
	st.Progress += dt * 60 / speed * 2
	threshold := float32(1 - 1/speed)
	if st.Progress < threshold {
		return
	}
	if !w.tryInsertItemLocked(pos, nextPos, st.Current, 0) {
		return
	}
	if !w.removeItemAtLocked(pos, st.Current, 1) {
		return
	}
	st.HasItem = false
	st.Progress = float32(math.Mod(float64(st.Progress), float64(threshold)))
	if item, ok := firstBuildingItem(tile.Build); ok {
		st.Current = item
		st.HasItem = true
	} else {
		st.Progress = 0
	}
	_ = armored
}

func (w *World) stepDuctRouterLocked(pos int32, tile *Tile, speed float32, stack bool, dt float32) {
	if tile == nil || tile.Build == nil || speed <= 0 {
		return
	}
	cap := w.itemCapacityForBlockLocked(tile)
	total := totalBuildingItems(tile.Build)
	st := w.ductStateLocked(pos, tile)
	if stack {
		sst := w.stackStateLocked(pos, tile)
		if sst.Unloading {
			for {
				target, ok := w.ductRouterTargetLocked(pos, tile, sst.LastItem)
				if !ok || tile.Build.ItemAmount(sst.LastItem) <= 0 {
					break
				}
				if !w.tryInsertItemLocked(pos, target, sst.LastItem, 0) {
					break
				}
				if !w.removeItemAtLocked(pos, sst.LastItem, 1) {
					break
				}
			}
			if item, ok := firstBuildingItem(tile.Build); ok {
				sst.LastItem = item
				sst.HasItem = true
			} else {
				sst.HasItem = false
				sst.Unloading = false
				st.HasItem = false
				st.Progress = 0
			}
			return
		}
		if sst.HasItem && total >= cap {
			st.Progress += dt * 60
			if st.Progress >= speed {
				st.Progress = float32(math.Mod(float64(st.Progress), float64(speed)))
				sst.Unloading = true
			}
		} else if !sst.HasItem {
			st.Progress = 0
		}
		return
	}
	if !st.HasItem {
		st.Progress = 0
		return
	}
	st.Progress += dt * 60 / speed * 2
	threshold := float32(1 - 1/speed)
	if st.Progress < threshold {
		return
	}
	target, ok := w.ductRouterTargetLocked(pos, tile, st.Current)
	if !ok {
		return
	}
	if !w.tryInsertItemLocked(pos, target, st.Current, 0) {
		return
	}
	if !w.removeItemAtLocked(pos, st.Current, 1) {
		return
	}
	st.HasItem = false
	st.Progress = float32(math.Mod(float64(st.Progress), float64(threshold)))
	if item, ok := firstBuildingItem(tile.Build); ok {
		st.Current = item
		st.HasItem = true
	} else {
		st.Progress = 0
	}
}

func (w *World) stepOverflowDuctLocked(pos int32, tile *Tile, speed float32, invert bool, dt float32) {
	if tile == nil || tile.Build == nil || speed <= 0 {
		return
	}
	st := w.ductStateLocked(pos, tile)
	if !st.HasItem {
		st.Progress = 0
		return
	}
	st.Progress += dt * 60 / speed * 2
	threshold := float32(1 - 1/speed)
	if st.Progress < threshold {
		return
	}
	target, ok := w.overflowDuctTargetLocked(pos, tile, st.Current, invert)
	if !ok {
		return
	}
	if !w.tryInsertItemLocked(pos, target, st.Current, 0) {
		return
	}
	if !w.removeItemAtLocked(pos, st.Current, 1) {
		return
	}
	if w.blockDumpIndex[pos] == 0 {
		w.blockDumpIndex[pos] = 2
	} else {
		w.blockDumpIndex[pos] = 0
	}
	st.HasItem = false
	st.Progress = float32(math.Mod(float64(st.Progress), float64(threshold)))
	if item, ok := firstBuildingItem(tile.Build); ok {
		st.Current = item
		st.HasItem = true
	} else {
		st.Progress = 0
	}
}

func (w *World) stepDuctBridgeLocked(pos int32, tile *Tile, speed float32, dt float32) {
	if tile == nil || tile.Build == nil || speed <= 0 {
		return
	}
	target, ok := w.directionBridgeTargetLocked(pos, tile, "duct-bridge", 4)
	if ok {
		w.transportAccum[pos] += dt * 60
		for w.transportAccum[pos] > speed {
			item, exists := firstBuildingItem(tile.Build)
			if !exists {
				break
			}
			targetTile := &w.model.Tiles[target]
			if totalBuildingItems(targetTile.Build) >= w.itemCapacityForBlockLocked(targetTile) {
				break
			}
			if !w.removeItemAtLocked(pos, item, 1) {
				break
			}
			if !w.addItemAtLocked(target, item, 1) {
				break
			}
			w.transportAccum[pos] -= speed
		}
		return
	}
	item, exists := firstBuildingItem(tile.Build)
	if !exists {
		return
	}
	nextPos, ok := w.forwardItemTargetPosLocked(pos, tile.Rotation)
	if !ok {
		return
	}
	if !w.tryInsertItemLocked(pos, nextPos, item, 0) {
		return
	}
	_ = w.removeItemAtLocked(pos, item, 1)
}

func (w *World) stepDirectionalUnloaderLocked(pos int32, tile *Tile, speed float32, dt float32) {
	if tile == nil || tile.Build == nil || speed <= 0 {
		return
	}
	w.transportAccum[pos] += dt * 60
	if w.transportAccum[pos] < speed {
		return
	}
	// Vanilla DirectionalUnloader consumes the cooldown window as soon as it
	// attempts an unload pass, even if no item actually moved.
	w.transportAccum[pos] = float32(math.Mod(float64(w.transportAccum[pos]), float64(speed)))
	frontPos, fok := w.forwardItemTargetPosLocked(pos, tile.Rotation)
	backPos, bok := w.forwardItemTargetPosLocked(pos, int8((int(tile.Rotation)+2)%4))
	if !fok || !bok {
		return
	}
	frontTile := &w.model.Tiles[frontPos]
	backTile := &w.model.Tiles[backPos]
	if frontTile.Build == nil || backTile.Build == nil || frontTile.Team != tile.Team || backTile.Team != tile.Team {
		return
	}
	if _, _, _, ok := w.sharedCoreInventoryLocked(backPos); ok {
		return
	}
	tryMove := func(item ItemID) bool {
		if !w.canUnloadItemFromBuildingLocked(backPos, backTile, item, false) {
			return false
		}
		if !w.tryInsertItemLocked(pos, frontPos, item, 0) {
			return false
		}
		if !w.removeItemAtLocked(backPos, item, 1) {
			_ = frontTile.Build.RemoveItem(item, 1)
			return false
		}
		return true
	}
	if item, ok := w.unloaderCfg[pos]; ok {
		_ = tryMove(item)
		return
	}
	items := w.rotatedInventoryItemIDsLocked(backPos, w.blockDumpIndex[pos])
	for _, item := range items {
		if tryMove(item) {
			w.blockDumpIndex[pos] = int(item) + 1
			return
		}
	}
}

func (w *World) stepStackConveyorLocked(pos int32, tile *Tile, speed float32, recharge float32, outputRouter bool, dt float32) {
	if tile == nil || tile.Build == nil || speed <= 0 {
		return
	}
	st := w.stackStateLocked(pos, tile)
	frames := dt * 60
	if st.Cooldown > 0 {
		st.Cooldown = maxf(0, st.Cooldown-speed*frames)
	}
	if item, ok := firstBuildingItem(tile.Build); ok {
		st.LastItem = item
		st.HasItem = true
		if st.Link < 0 {
			st.Link = pos
		}
	} else {
		st.HasItem = false
		st.Unloading = false
		st.Link = -1
		return
	}
	if w.blockNameByID(int16(tile.Block)) == "surge-conveyor" && !w.requirePowerAtLocked(pos, tile.Team, (1.0/60.0)*dt) {
		return
	}
	frontPos, hasFront := w.forwardItemTargetPosLocked(pos, tile.Rotation)
	if hasFront {
		frontTile := &w.model.Tiles[frontPos]
		if frontTile.Build != nil && frontTile.Team == tile.Team && isStackConveyorBlock(w.blockNameByID(int16(frontTile.Block))) && st.Cooldown <= 0 {
			frontState := w.stackStateLocked(frontPos, frontTile)
			if frontState.Link < 0 && (!outputRouter || totalBuildingItems(tile.Build) >= w.itemCapacityForBlockLocked(tile) || st.Link != pos) {
				w.replaceBuildingItemsLocked(frontPos, frontTile, tile.Build.Items)
				frontState.LastItem = st.LastItem
				frontState.HasItem = true
				frontState.Link = pos
				frontState.Cooldown = 1
				w.replaceBuildingItemsLocked(pos, tile, nil)
				st.HasItem = false
				st.Link = -1
				st.Cooldown = recharge
				return
			}
		}
	}
	if outputRouter {
		_ = w.dumpSingleItemLocked(pos, tile, &st.LastItem, nil)
	} else if hasFront {
		if w.tryInsertItemLocked(pos, frontPos, st.LastItem, 0) {
			_ = w.removeItemAtLocked(pos, st.LastItem, 1)
		}
	}
	if item, ok := firstBuildingItem(tile.Build); ok {
		st.LastItem = item
		st.HasItem = true
	} else {
		st.HasItem = false
		st.Link = -1
	}
}

func (w *World) stepStackRouterLocked(pos int32, tile *Tile, speed float32, dt float32) {
	if tile == nil || tile.Build == nil || speed <= 0 {
		return
	}
	dst := w.ductStateLocked(pos, tile)
	sst := w.stackStateLocked(pos, tile)
	if item, ok := firstBuildingItem(tile.Build); ok {
		sst.LastItem = item
		sst.HasItem = true
		dst.Current = item
		dst.HasItem = true
	} else {
		sst.HasItem = false
		sst.Unloading = false
		dst.HasItem = false
		dst.Progress = 0
		return
	}
	if w.blockNameByID(int16(tile.Block)) == "surge-router" && !w.requirePowerAtLocked(pos, tile.Team, (3.0/60.0)*dt) {
		return
	}
	if !sst.Unloading && totalBuildingItems(tile.Build) >= w.itemCapacityForBlockLocked(tile) {
		dst.Progress += dt * 60
		if dst.Progress >= speed {
			dst.Progress = float32(math.Mod(float64(dst.Progress), float64(speed)))
			sst.Unloading = true
		}
	}
	if !sst.Unloading {
		return
	}
	for {
		target, ok := w.ductRouterTargetLocked(pos, tile, sst.LastItem)
		if !ok || tile.Build.ItemAmount(sst.LastItem) <= 0 {
			break
		}
		if !w.tryInsertItemLocked(pos, target, sst.LastItem, 0) {
			break
		}
		if !w.removeItemAtLocked(pos, sst.LastItem, 1) {
			break
		}
	}
	if item, ok := firstBuildingItem(tile.Build); ok {
		sst.LastItem = item
		sst.HasItem = true
		dst.Current = item
		dst.HasItem = true
	} else {
		sst.HasItem = false
		sst.Unloading = false
		dst.HasItem = false
		dst.Progress = 0
	}
}

func (w *World) stepRouterLocked(pos int32, tile *Tile, rate float32, dt float32) {
	if tile == nil || tile.Build == nil || rate <= 0 {
		return
	}
	st := w.routerStateLocked(pos, tile)
	if !st.HasItem {
		if item, ok := firstBuildingItem(tile.Build); ok {
			st.LastItem = item
			st.HasItem = true
		} else {
			st.Time = 0
			w.transportAccum[pos] = 0
			return
		}
	}
	st.Time += rate * dt
	target, ok := w.routerTargetLocked(pos, tile, st.LastItem, false)
	if !ok {
		return
	}
	targetName := w.blockNameByID(int16(w.model.Tiles[target].Block))
	if st.Time < 1 && isRouterOrInstantTransferBlock(targetName) {
		return
	}
	target, ok = w.routerTargetLocked(pos, tile, st.LastItem, true)
	if !ok {
		return
	}
	if !w.tryInsertItemLocked(pos, target, st.LastItem, 0) {
		return
	}
	_ = w.removeItemAtLocked(pos, st.LastItem, 1)
	st.HasItem = false
	st.Time = 0
	w.transportAccum[pos] = 0
}

func (w *World) stepBridgeConveyorLocked(pos int32, tile *Tile, rate float32, dt float32) {
	if tile == nil || tile.Build == nil || rate <= 0 {
		return
	}
	target, linked := w.bridgeTargetLocked(pos, tile)
	if !linked {
		w.dumpSingleItemLocked(pos, tile, nil, func(targetPos int32, item ItemID) bool {
			side, ok := w.flowDirBetweenLocked(pos, targetPos)
			return ok && !w.bridgeHasIncomingFromSideLocked(pos, side)
		})
		return
	}

	const (
		bufferCapacity   = 14
		bufferDelayFrame = float32(74)
		acceptEveryFrame = float32(4)
	)

	buffer := w.bridgeBuffers[pos]
	for len(buffer) < bufferCapacity {
		item, ok := firstBuildingItem(tile.Build)
		if !ok {
			break
		}
		if !w.removeItemAtLocked(pos, item, 1) {
			break
		}
		buffer = append(buffer, bufferedBridgeItem{Item: item})
	}
	for i := range buffer {
		buffer[i].AgeFrames += dt * 60
	}
	w.bridgeAcceptAcc[pos] += dt * 60
	for len(buffer) > 0 && buffer[0].AgeFrames >= bufferDelayFrame && w.bridgeAcceptAcc[pos] >= acceptEveryFrame {
		if !w.tryInsertItemLocked(pos, target, buffer[0].Item, 0) {
			break
		}
		buffer = buffer[1:]
		w.bridgeAcceptAcc[pos] -= acceptEveryFrame
	}
	if len(buffer) == 0 {
		delete(w.bridgeBuffers, pos)
	} else {
		w.bridgeBuffers[pos] = buffer
	}
}

func (w *World) stepPhaseConveyorLocked(pos int32, tile *Tile, dt float32) {
	if tile == nil || tile.Build == nil {
		return
	}
	target, linked := w.bridgeTargetLocked(pos, tile)
	if !linked {
		w.dumpSingleItemLocked(pos, tile, nil, func(targetPos int32, item ItemID) bool {
			side, ok := w.flowDirBetweenLocked(pos, targetPos)
			return ok && !w.bridgeHasIncomingFromSideLocked(pos, side)
		})
		return
	}
	w.transportAccum[pos] += dt * 60
	for w.transportAccum[pos] >= 2 {
		item, ok := firstBuildingItem(tile.Build)
		if !ok {
			break
		}
		if !w.tryInsertItemLocked(pos, target, item, 0) {
			break
		}
		if !w.removeItemAtLocked(pos, item, 1) {
			break
		}
		w.transportAccum[pos] -= 2
	}
}

func (w *World) stepUnloaderLocked(pos int32, tile *Tile, dt float32) {
	if tile == nil || tile.Build == nil {
		return
	}
	const unloadSpeedFrames = float32(60.0 / 11.0)
	w.transportAccum[pos] += dt * 60
	if w.transportAccum[pos] < unloadSpeedFrames {
		return
	}
	neighbors := w.dumpProximityLocked(pos)
	if len(neighbors) < 2 {
		return
	}
	item, ok := w.unloaderTargetItemLocked(pos, neighbors)
	if !ok {
		w.transportAccum[pos] = minf(w.transportAccum[pos], unloadSpeedFrames)
		return
	}
	fromPos, toPos, ok := w.unloaderTransferPairLocked(pos, neighbors, item)
	if !ok {
		w.transportAccum[pos] = minf(w.transportAccum[pos], unloadSpeedFrames)
		return
	}
	fromTile := &w.model.Tiles[fromPos]
	toTile := &w.model.Tiles[toPos]
	if fromTile.Build == nil || toTile.Build == nil {
		w.transportAccum[pos] = minf(w.transportAccum[pos], unloadSpeedFrames)
		return
	}
	if !w.tryInsertItemLocked(pos, toPos, item, 0) {
		w.transportAccum[pos] = minf(w.transportAccum[pos], unloadSpeedFrames)
		return
	}
	if !w.removeItemAtLocked(fromPos, item, 1) {
		_ = w.removeItemAtLocked(toPos, item, 1)
		w.transportAccum[pos] = minf(w.transportAccum[pos], unloadSpeedFrames)
		return
	}
	w.transportAccum[pos] = float32(math.Mod(float64(w.transportAccum[pos]), float64(unloadSpeedFrames)))
}

func (w *World) stepMassDriverLocked(pos int32, tile *Tile, dt float32) {
	if tile == nil || tile.Build == nil {
		return
	}
	st := w.massDriverStateLocked(pos)
	if st.ReloadCounter > 0 {
		st.ReloadCounter = maxf(0, st.ReloadCounter-dt*60/200)
	}
	target, ok := w.massDriverTargetLocked(pos, tile)
	if !ok || st.ReloadCounter > 0 {
		return
	}
	if totalBuildingItems(tile.Build) < 10 {
		return
	}
	targetTile := &w.model.Tiles[target]
	if targetTile.Build == nil || totalBuildingItems(targetTile.Build) > 230 {
		return
	}
	if w.massDriverIncomingShotsLocked(target) > 0 {
		return
	}
	if !w.requirePowerAtLocked(pos, tile.Team, 1.75*dt) {
		return
	}
	items := w.massDriverTakePayloadLocked(pos, tile, 120)
	if len(items) == 0 {
		return
	}
	st.ReloadCounter = 1
	dx := float32(targetTile.X-tile.X) * 8
	dy := float32(targetTile.Y-tile.Y) * 8
	travelFrames := float32(math.Sqrt(float64(dx*dx+dy*dy)) / 5.5)
	if travelFrames < 1 {
		travelFrames = 1
	}
	w.massDriverShots = append(w.massDriverShots, massDriverShot{
		FromPos:      pos,
		ToPos:        target,
		TravelFrames: travelFrames,
		Transferred:  items,
	})
	srcX := float32(tile.X*8 + 4)
	srcY := float32(tile.Y*8 + 4)
	dstX := float32(targetTile.X*8 + 4)
	dstY := float32(targetTile.Y*8 + 4)
	angle := lookAt(srcX, srcY, dstX, dstY)
	rad := float32(angle * math.Pi / 180)
	const massDriverEffectOffset = float32(7)
	w.emitEffectLocked("shootbig2", srcX+float32(math.Cos(float64(rad)))*massDriverEffectOffset, srcY+float32(math.Sin(float64(rad)))*massDriverEffectOffset, angle)
	w.emitEffectLocked("shootbigsmoke2", srcX+float32(math.Cos(float64(rad)))*massDriverEffectOffset, srcY+float32(math.Sin(float64(rad)))*massDriverEffectOffset, angle)
}

func (w *World) stepMassDriverShotsLocked(dt float32) {
	if len(w.massDriverShots) == 0 {
		return
	}
	kept := w.massDriverShots[:0]
	for _, shot := range w.massDriverShots {
		shot.AgeFrames += dt * 60
		if shot.AgeFrames < shot.TravelFrames {
			kept = append(kept, shot)
			continue
		}
		targetTile := (*Tile)(nil)
		if w.model != nil && shot.ToPos >= 0 && int(shot.ToPos) < len(w.model.Tiles) {
			targetTile = &w.model.Tiles[shot.ToPos]
		}
		if targetTile == nil || targetTile.Build == nil || w.blockNameByID(int16(targetTile.Block)) != "mass-driver" {
			continue
		}
		total := totalBuildingItems(targetTile.Build)
		for _, stack := range shot.Transferred {
			if stack.Amount <= 0 {
				continue
			}
			space := int32(240 - total)
			if space <= 0 {
				break
			}
			amount := stack.Amount
			if amount > space {
				amount = space
			}
			if !w.addItemAtLocked(shot.ToPos, stack.Item, amount) {
				continue
			}
			total += amount
		}
		w.massDriverStateLocked(shot.ToPos).ReloadCounter = 1
		w.emitEffectLocked("minebig", float32(targetTile.X*8+4), float32(targetTile.Y*8+4), 0)
	}
	w.massDriverShots = kept
}

func (w *World) stepJunctions(dt float32) {
	const travelSec = float32(26.0 / 60.0)
	for pos, state := range w.junctionQueues {
		empty := true
		for dir := 0; dir < len(state); dir++ {
			queue := state[dir]
			if len(queue) == 0 {
				continue
			}
			empty = false
			for i := range queue {
				queue[i].AgeSec += dt
			}
			head := queue[0]
			if head.AgeSec >= travelSec {
				outPos, ok := w.forwardItemTargetPosLocked(pos, int8(head.FromDir))
				if ok && w.tryInsertItemLocked(pos, outPos, head.Item, 0) {
					queue = queue[1:]
				}
			}
			state[dir] = queue
			if len(queue) > 0 {
				empty = false
			}
		}
		if empty {
			delete(w.junctionQueues, pos)
			continue
		}
		w.junctionQueues[pos] = state
	}
}

func (w *World) tryInsertItemLocked(fromPos, toPos int32, item ItemID, depth int) bool {
	if depth > 8 || w.model == nil || toPos < 0 || int(toPos) >= len(w.model.Tiles) {
		return false
	}
	fromTile := &w.model.Tiles[fromPos]
	toTile := &w.model.Tiles[toPos]
	if toTile.Build == nil || toTile.Block == 0 || toTile.Team != fromTile.Team {
		return false
	}
	switch w.blockNameByID(int16(toTile.Block)) {
	case "conveyor", "titanium-conveyor", "armored-conveyor":
		return w.conveyorHandleItemLocked(fromPos, toPos, item)
	case "duct":
		return w.ductHandleItemLocked(fromPos, toPos, item, false)
	case "armored-duct":
		return w.ductHandleItemLocked(fromPos, toPos, item, true)
	case "duct-router":
		return w.ductRouterHandleItemLocked(fromPos, toPos, item, false)
	case "overflow-duct":
		return w.ductHandleItemLocked(fromPos, toPos, item, false)
	case "underflow-duct":
		return w.ductHandleItemLocked(fromPos, toPos, item, false)
	case "duct-bridge":
		if !w.ductBridgeAcceptsItemLocked(fromPos, toPos, item) {
			return false
		}
		return w.addItemAtLocked(toPos, item, 1)
	case "duct-unloader":
		return false
	case "bridge-conveyor", "phase-conveyor":
		if !w.bridgeAllowsInputLocked(fromPos, toPos) {
			return false
		}
		cap := w.itemCapacityAtLocked(toPos)
		if w.totalItemsAtLocked(toPos) >= cap {
			return false
		}
		return w.addItemAtLocked(toPos, item, 1)
	case "mass-driver":
		cap := w.itemCapacityAtLocked(toPos)
		if cap <= 0 || w.totalItemsAtLocked(toPos) >= cap {
			return false
		}
		if _, ok := w.massDriverTargetLocked(toPos, toTile); !ok {
			return false
		}
		return w.addItemAtLocked(toPos, item, 1)
	case "router", "distributor":
		st := w.routerStateLocked(toPos, toTile)
		if st.HasItem || totalBuildingItems(toTile.Build) >= 1 {
			return false
		}
		if !w.addItemAtLocked(toPos, item, 1) {
			return false
		}
		st.LastItem = item
		st.HasItem = true
		st.Time = 0
		st.LastInput = fromPos
		w.routerInputPos[toPos] = fromPos
		return true
	case "plastanium-conveyor", "surge-conveyor":
		return w.stackConveyorHandleItemLocked(fromPos, toPos, item)
	case "surge-router":
		return w.ductRouterHandleItemLocked(fromPos, toPos, item, true)
	case "junction":
		dir, ok := flowDir(fromTile.X, fromTile.Y, toTile.X, toTile.Y)
		if !ok {
			return false
		}
		outPos, ok := w.forwardItemTargetPosLocked(toPos, int8(dir))
		if !ok {
			return false
		}
		outTile := &w.model.Tiles[outPos]
		if outTile.Build == nil || outTile.Block == 0 || outTile.Team != toTile.Team {
			return false
		}
		state := w.junctionQueues[toPos]
		if len(state[dir]) >= 6 {
			return false
		}
		state[dir] = append(state[dir], junctionQueuedItem{Item: item, FromDir: dir})
		w.junctionQueues[toPos] = state
		return true
	case "sorter", "inverted-sorter":
		target, ok := w.sorterTargetLocked(fromPos, toPos, item, w.blockNameByID(int16(toTile.Block)) == "inverted-sorter", true)
		if !ok {
			return false
		}
		return w.tryInsertItemLocked(toPos, target, item, depth+1)
	case "overflow-gate":
		target, ok := w.overflowTargetLocked(fromPos, toPos, item, false, true)
		if !ok {
			return false
		}
		return w.tryInsertItemLocked(toPos, target, item, depth+1)
	case "underflow-gate":
		target, ok := w.overflowTargetLocked(fromPos, toPos, item, true, true)
		if !ok {
			return false
		}
		return w.tryInsertItemLocked(toPos, target, item, depth+1)
	case "thorium-reactor":
		return w.storeAcceptedBuildingItemLocked(toPos, toTile, item, 1)
	case "item-void":
		return true
	case "incinerator", "slag-incinerator":
		if !w.incineratorAcceptsItemLocked(toPos) {
			return false
		}
		w.incineratorBurnItemLocked(toPos)
		return true
	default:
		return w.storeAcceptedBuildingItemLocked(toPos, toTile, item, 1)
	}
}

func (w *World) bridgeTargetLocked(pos int32, tile *Tile) (int32, bool) {
	target, ok := w.bridgeLinks[pos]
	if !ok || target < 0 || int(target) >= len(w.model.Tiles) {
		return 0, false
	}
	targetTile := &w.model.Tiles[target]
	name := w.blockNameByID(int16(tile.Block))
	if targetTile.Build == nil || targetTile.Team != tile.Team || w.blockNameByID(int16(targetTile.Block)) != name {
		return 0, false
	}
	return target, true
}

func firstBuildingLiquid(build *Building) (LiquidID, float32, bool) {
	if build == nil {
		return 0, 0, false
	}
	var (
		bestLiquid LiquidID
		bestAmount float32
		found      bool
	)
	for _, stack := range build.Liquids {
		if stack.Amount <= 0 {
			continue
		}
		if !found || stack.Amount > bestAmount {
			bestLiquid = stack.Liquid
			bestAmount = stack.Amount
			found = true
		}
	}
	return bestLiquid, bestAmount, found
}

func totalBuildingLiquids(build *Building) float32 {
	if build == nil {
		return 0
	}
	total := float32(0)
	for _, stack := range build.Liquids {
		if stack.Amount > 0 {
			total += stack.Amount
		}
	}
	return total
}

func (w *World) liquidCanStoreLocked(tile *Tile, liquid LiquidID) bool {
	if tile == nil || tile.Build == nil {
		return false
	}
	total := totalBuildingLiquids(tile.Build)
	if total >= w.liquidCapacityForBlockLocked(tile) {
		return false
	}
	current, amount, ok := firstBuildingLiquid(tile.Build)
	return !ok || current == liquid || amount < 0.2
}

func (w *World) conduitAcceptsLiquidLocked(fromPos, toPos int32, liquid LiquidID, armored bool) bool {
	if w.model == nil || fromPos < 0 || toPos < 0 || int(fromPos) >= len(w.model.Tiles) || int(toPos) >= len(w.model.Tiles) {
		return false
	}
	toTile := &w.model.Tiles[toPos]
	if toTile.Build == nil || !w.liquidCanStoreLocked(toTile, liquid) {
		return false
	}
	sourceSide, ok := w.flowDirBetweenLocked(fromPos, toPos)
	if !ok {
		return false
	}
	if sourceSide == byte((tileRotationNorm(toTile.Rotation)+2)%4) {
		return false
	}
	if !armored {
		return true
	}
	fromTile := &w.model.Tiles[fromPos]
	fromName := w.blockNameByID(int16(fromTile.Block))
	if fromName == "conduit" || fromName == "pulse-conduit" || fromName == "plated-conduit" || fromName == "reinforced-conduit" ||
		fromName == "reinforced-bridge-conduit" || fromName == "liquid-junction" || fromName == "reinforced-liquid-junction" {
		return true
	}
	return sourceSide == byte(tileRotationNorm(toTile.Rotation))
}

func (w *World) canAcceptLiquidLocked(fromPos, toPos int32, liquid LiquidID, depth int) bool {
	if depth > 8 || w.model == nil || fromPos < 0 || toPos < 0 || int(fromPos) >= len(w.model.Tiles) || int(toPos) >= len(w.model.Tiles) {
		return false
	}
	fromTile := &w.model.Tiles[fromPos]
	toTile := &w.model.Tiles[toPos]
	if toTile.Build == nil || toTile.Block == 0 || toTile.Team != fromTile.Team {
		return false
	}
	name := w.blockNameByID(int16(toTile.Block))
	switch name {
	case "conduit", "pulse-conduit":
		return w.conduitAcceptsLiquidLocked(fromPos, toPos, liquid, false)
	case "plated-conduit":
		return w.conduitAcceptsLiquidLocked(fromPos, toPos, liquid, true)
	case "reinforced-conduit":
		return w.conduitAcceptsLiquidLocked(fromPos, toPos, liquid, false)
	case "liquid-void":
		return true
	case "liquid-router", "liquid-container", "liquid-tank", "reinforced-liquid-router", "reinforced-liquid-container", "reinforced-liquid-tank", "thorium-reactor":
		return w.liquidCanStoreLocked(toTile, liquid)
	case "bridge-conduit", "phase-conduit":
		return w.bridgeAllowsInputLocked(fromPos, toPos) && w.liquidCanStoreLocked(toTile, liquid)
	case "reinforced-bridge-conduit":
		if _, ok := w.directionBridgeTargetLocked(toPos, toTile, "reinforced-bridge-conduit", 4); !ok {
			return false
		}
		rel, ok := w.relativeToEdgeLocked(fromPos, toPos)
		return ok && rel != byte(tileRotationNorm(toTile.Rotation)) && w.liquidCanStoreLocked(toTile, liquid)
	case "liquid-junction", "reinforced-liquid-junction":
		_, ok := w.liquidJunctionDestinationLocked(fromPos, toPos, liquid, depth+1)
		return ok
	case "incinerator":
		return w.incineratorAcceptsLiquidLocked(toPos, liquid)
	case "slag-incinerator":
		return liquid == slagLiquidID && w.incineratorAcceptsLiquidLocked(toPos, liquid)
	case "repair-turret":
		return repairTurretAcceptsLiquid(liquid) && w.liquidCanStoreLocked(toTile, liquid)
	case "unit-repair-tower":
		return liquid == ozoneLiquidID && w.liquidCanStoreLocked(toTile, liquid)
	case "plasma-bore":
		return liquid == hydrogenLiquidID && w.liquidCanStoreLocked(toTile, liquid)
	case "large-plasma-bore":
		return (liquid == hydrogenLiquidID || liquid == nitrogenLiquidID) && w.liquidCanStoreLocked(toTile, liquid)
	case "impact-drill":
		return (liquid == waterLiquidID || liquid == ozoneLiquidID) && w.liquidCanStoreLocked(toTile, liquid)
	case "eruption-drill":
		return (liquid == hydrogenLiquidID || liquid == cyanogenLiquidID) && w.liquidCanStoreLocked(toTile, liquid)
	default:
		if isReconstructorBlockName(name) {
			return w.reconstructorAcceptsLiquidLocked(toTile, liquid)
		}
		cap := w.liquidCapacityForBlockLocked(toTile)
		return cap > 0 && w.liquidCanStoreLocked(toTile, liquid)
	}
}

func (w *World) liquidJunctionDestinationLocked(fromPos, junctionPos int32, liquid LiquidID, depth int) (int32, bool) {
	if depth > 8 || w.model == nil || junctionPos < 0 || int(junctionPos) >= len(w.model.Tiles) {
		return 0, false
	}
	sourceDir, ok := w.relativeToEdgeLocked(fromPos, junctionPos)
	if !ok {
		return 0, false
	}
	outDir := byte((int(sourceDir) + 2) % 4)
	nextPos, ok := w.forwardPosLocked(junctionPos, int8(outDir))
	if !ok {
		return 0, false
	}
	nextTile := &w.model.Tiles[nextPos]
	if nextTile.Build == nil || nextTile.Block == 0 {
		return 0, false
	}
	name := w.blockNameByID(int16(nextTile.Block))
	if name == "liquid-junction" || name == "reinforced-liquid-junction" {
		return w.liquidJunctionDestinationLocked(junctionPos, nextPos, liquid, depth+1)
	}
	if !w.canAcceptLiquidLocked(junctionPos, nextPos, liquid, depth+1) {
		return 0, false
	}
	return nextPos, true
}

func (w *World) tryMoveLiquidLocked(fromPos, toPos int32, liquid LiquidID, amount float32, depth int) float32 {
	if amount <= 0 || !w.canAcceptLiquidLocked(fromPos, toPos, liquid, depth) || w.model == nil {
		return 0
	}
	toTile := &w.model.Tiles[toPos]
	name := w.blockNameByID(int16(toTile.Block))
	if name == "liquid-junction" || name == "reinforced-liquid-junction" {
		target, ok := w.liquidJunctionDestinationLocked(fromPos, toPos, liquid, depth+1)
		if !ok {
			return 0
		}
		return w.tryMoveLiquidLocked(toPos, target, liquid, amount, depth+1)
	}
	if name == "incinerator" {
		w.incineratorBurnLiquidLocked(toPos)
		return amount
	}
	if name == "liquid-void" {
		return amount
	}
	cap := w.liquidCapacityForBlockLocked(toTile)
	if cap <= 0 {
		return 0
	}
	current := totalBuildingLiquids(toTile.Build)
	space := cap - current
	if space <= 0 {
		return 0
	}
	if amount > space {
		amount = space
	}
	if amount <= 0 {
		return 0
	}
	toTile.Build.AddLiquid(liquid, amount)
	return amount
}

func (w *World) dumpLiquidLocked(pos int32, tile *Tile, liquid LiquidID, amount float32) bool {
	if tile == nil || tile.Build == nil || amount <= 0 || w.model == nil {
		return false
	}
	neighbors := w.dumpProximityLocked(pos)
	if len(neighbors) == 0 {
		return false
	}
	start := 0
	if idx, ok := w.blockDumpIndex[pos]; ok && len(neighbors) > 0 {
		start = ((idx % len(neighbors)) + len(neighbors)) % len(neighbors)
	}
	for i := 0; i < len(neighbors); i++ {
		index := (start + i) % len(neighbors)
		target := neighbors[index]
		moved := w.tryMoveLiquidLocked(pos, target, liquid, amount, 0)
		w.advanceDumpIndexLocked(pos, index+1, len(neighbors))
		if moved > 0 {
			_ = tile.Build.RemoveLiquid(liquid, moved)
			return true
		}
	}
	return false
}

func (w *World) bridgeAllowsInputLocked(fromPos, bridgePos int32) bool {
	if w.model == nil || fromPos < 0 || bridgePos < 0 || int(fromPos) >= len(w.model.Tiles) || int(bridgePos) >= len(w.model.Tiles) {
		return false
	}
	bridgeTile := &w.model.Tiles[bridgePos]
	bridgeName := w.blockNameByID(int16(bridgeTile.Block))
	if !isItemBridgeBlock(bridgeName) {
		return false
	}
	if w.bridgeLinks[fromPos] == bridgePos && w.blockNameByID(int16(w.model.Tiles[fromPos].Block)) == bridgeName {
		return true
	}
	target, ok := w.bridgeLinks[bridgePos]
	if !ok || target < 0 || int(target) >= len(w.model.Tiles) {
		return false
	}
	targetTile := &w.model.Tiles[target]
	if targetTile.Build == nil || targetTile.Team != bridgeTile.Team || w.blockNameByID(int16(targetTile.Block)) != bridgeName {
		return false
	}
	linkSide, ok := axisDir(targetTile.X, targetTile.Y, bridgeTile.X, bridgeTile.Y)
	if !ok {
		return false
	}
	fromTile := &w.model.Tiles[fromPos]
	sourceSide, ok := relativeDir(fromTile.X, fromTile.Y, bridgeTile.X, bridgeTile.Y)
	if !ok {
		return false
	}
	return sourceSide != linkSide
}

func (w *World) bridgeHasIncomingFromSideLocked(bridgePos int32, side byte) bool {
	if w.model == nil || bridgePos < 0 || int(bridgePos) >= len(w.model.Tiles) {
		return false
	}
	if len(w.bridgeIncomingMask) == 0 && len(w.bridgeLinks) > 0 {
		mask := make(map[int32]byte, len(w.bridgeLinks))
		for otherPos, target := range w.bridgeLinks {
			if target < 0 || otherPos < 0 || int(target) >= len(w.model.Tiles) || int(otherPos) >= len(w.model.Tiles) {
				continue
			}
			bridgeTile := &w.model.Tiles[target]
			otherTile := &w.model.Tiles[otherPos]
			if bridgeTile.Build == nil || otherTile.Build == nil || otherTile.Team != bridgeTile.Team {
				continue
			}
			bridgeName := w.blockNameByID(int16(bridgeTile.Block))
			if w.blockNameByID(int16(otherTile.Block)) != bridgeName {
				continue
			}
			incomingSide, ok := axisDir(otherTile.X, otherTile.Y, bridgeTile.X, bridgeTile.Y)
			if !ok || incomingSide >= 8 {
				continue
			}
			mask[target] |= 1 << incomingSide
		}
		w.bridgeIncomingMask = mask
	}
	return (w.bridgeIncomingMask[bridgePos] & (1 << side)) != 0
}

func (w *World) massDriverStateLocked(pos int32) *massDriverRuntimeState {
	if st, ok := w.massDriverStates[pos]; ok && st != nil {
		return st
	}
	st := &massDriverRuntimeState{}
	w.massDriverStates[pos] = st
	return st
}

func (w *World) massDriverTargetLocked(pos int32, tile *Tile) (int32, bool) {
	target, ok := w.massDriverLinks[pos]
	if !ok || target < 0 || int(target) >= len(w.model.Tiles) {
		return 0, false
	}
	targetTile := &w.model.Tiles[target]
	if tile == nil || targetTile.Build == nil || targetTile.Team != tile.Team || w.blockNameByID(int16(targetTile.Block)) != "mass-driver" {
		return 0, false
	}
	dx := float32(targetTile.X - tile.X)
	dy := float32(targetTile.Y - tile.Y)
	if dx*dx+dy*dy > 55*55 {
		return 0, false
	}
	return target, true
}

func (w *World) massDriverIncomingShotsLocked(targetPos int32) int {
	count := 0
	for _, shot := range w.massDriverShots {
		if shot.ToPos == targetPos {
			count++
		}
	}
	return count
}

func (w *World) massDriverTakePayloadLocked(pos int32, tile *Tile, limit int32) []ItemStack {
	if tile == nil || tile.Build == nil || limit <= 0 {
		return nil
	}
	total := int32(0)
	out := make([]ItemStack, 0, len(tile.Build.Items))
	for _, stack := range append([]ItemStack(nil), tile.Build.Items...) {
		if stack.Amount <= 0 || total >= limit {
			continue
		}
		amount := stack.Amount
		if amount > limit-total {
			amount = limit - total
		}
		if amount <= 0 {
			continue
		}
		if w.removeItemAtLocked(pos, stack.Item, amount) {
			out = append(out, ItemStack{Item: stack.Item, Amount: amount})
			total += amount
		}
	}
	return out
}

func isStorageLikeBlock(name string) bool {
	switch name {
	case "core-shard", "core-foundation", "core-nucleus", "core-bastion", "core-citadel", "core-acropolis", "container", "vault", "reinforced-container", "reinforced-vault":
		return true
	default:
		return false
	}
}

func isCoreBlockName(name string) bool {
	return strings.HasPrefix(name, "core-")
}

func isCoreMergeStorageBlock(name string) bool {
	switch name {
	case "container", "vault", "reinforced-container", "reinforced-vault":
		return true
	default:
		return false
	}
}

func affectsCoreStorageLinks(name string) bool {
	return isCoreBlockName(name) || isCoreMergeStorageBlock(name)
}

func (w *World) allowCoreUnloadersLocked() bool {
	if w == nil || w.rulesMgr == nil {
		return true
	}
	rules := w.rulesMgr.Get()
	return rules == nil || rules.AllowCoreUnloaders
}

func normalizeItemStackMap(items map[ItemID]int32, maxPerItem int32) []ItemStack {
	if len(items) == 0 {
		return nil
	}
	ids := make([]int, 0, len(items))
	for item, amount := range items {
		if amount <= 0 {
			continue
		}
		ids = append(ids, int(item))
	}
	sort.Ints(ids)
	out := make([]ItemStack, 0, len(ids))
	for _, rawID := range ids {
		item := ItemID(rawID)
		amount := items[item]
		if amount <= 0 {
			continue
		}
		if maxPerItem > 0 && amount > maxPerItem {
			amount = maxPerItem
		}
		out = append(out, ItemStack{Item: item, Amount: amount})
	}
	return out
}

func (w *World) refreshCoreStorageLinksLocked() {
	w.storageLinkedCore = map[int32]int32{}
	w.teamPrimaryCore = map[TeamID]int32{}
	w.coreStorageCapacity = map[int32]int32{}
	if w.model == nil {
		return
	}

	for team, cores := range w.teamCoreTiles {
		if len(cores) == 0 {
			continue
		}
		cores = append([]int32(nil), cores...)
		sort.Slice(cores, func(i, j int) bool { return cores[i] < cores[j] })
		primary := cores[0]
		w.teamPrimaryCore[team] = primary

		totalCapacity := int32(0)
		mergedItems := make(map[ItemID]int32)
		ownedStorages := make(map[int32]struct{})

		for _, corePos := range cores {
			coreTile := &w.model.Tiles[corePos]
			totalCapacity += w.itemCapacityForBlockLocked(coreTile)
			for _, stack := range coreTile.Build.Items {
				if stack.Amount > 0 {
					mergedItems[stack.Item] += stack.Amount
				}
			}
		}

		queue := append([]int32(nil), cores...)
		for len(queue) > 0 {
			anchorPos := queue[0]
			queue = queue[1:]
			if anchorPos < 0 || int(anchorPos) >= len(w.model.Tiles) {
				continue
			}
			anchor := &w.model.Tiles[anchorPos]
			w.forEachTouchingCoreStorageLocked(anchor, team, func(otherPos int32, other *Tile) {
				if otherPos < 0 || int(otherPos) >= len(w.model.Tiles) || otherPos == anchorPos {
					return
				}
				if _, exists := ownedStorages[otherPos]; exists {
					return
				}
				ownedStorages[otherPos] = struct{}{}
				w.storageLinkedCore[otherPos] = primary
				totalCapacity += w.itemCapacityForBlockLocked(other)
				for _, stack := range other.Build.Items {
					if stack.Amount > 0 {
						mergedItems[stack.Item] += stack.Amount
					}
				}
				queue = append(queue, otherPos)
			})
		}

		normalized := normalizeItemStackMap(mergedItems, totalCapacity)
		if primary >= 0 && int(primary) < len(w.model.Tiles) {
			if build := w.model.Tiles[primary].Build; build != nil {
				build.Items = normalized
			}
		}
		for _, corePos := range cores {
			w.coreStorageCapacity[corePos] = totalCapacity
			if corePos == primary {
				continue
			}
			if build := w.model.Tiles[corePos].Build; build != nil {
				build.Items = nil
			}
		}
		for storagePos := range ownedStorages {
			if build := w.model.Tiles[storagePos].Build; build != nil {
				build.Items = nil
			}
		}
	}
}

func (w *World) forEachTouchingCoreStorageLocked(anchor *Tile, team TeamID, visit func(pos int32, tile *Tile)) {
	if w == nil || w.model == nil || anchor == nil || visit == nil {
		return
	}
	if len(w.blockOccupancy) == 0 {
		for _, otherPos := range w.teamBuildingTiles[team] {
			if otherPos < 0 || int(otherPos) >= len(w.model.Tiles) {
				continue
			}
			other := &w.model.Tiles[otherPos]
			if other.Build == nil || other.Block == 0 || other.Team != team {
				continue
			}
			if !isCoreMergeStorageBlock(w.blockNameByID(int16(other.Block))) || !w.storageFootprintsTouchLocked(anchor, other) {
				continue
			}
			visit(otherPos, other)
		}
		return
	}
	low, high := blockFootprintRange(w.blockSizeForTileLocked(anchor))
	minX := anchor.X + low - 1
	maxX := anchor.X + high + 1
	minY := anchor.Y + low - 1
	maxY := anchor.Y + high + 1
	seen := map[int32]struct{}{}
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			if !w.model.InBounds(x, y) {
				continue
			}
			otherPos, ok := w.buildingOccupyingCellLocked(x, y)
			if !ok || otherPos < 0 || int(otherPos) >= len(w.model.Tiles) {
				continue
			}
			if _, exists := seen[otherPos]; exists {
				continue
			}
			seen[otherPos] = struct{}{}
			other := &w.model.Tiles[otherPos]
			if other.Build == nil || other.Block == 0 || other.Team != team {
				continue
			}
			if !isCoreMergeStorageBlock(w.blockNameByID(int16(other.Block))) || !w.storageFootprintsTouchLocked(anchor, other) {
				continue
			}
			visit(otherPos, other)
		}
	}
}

func (w *World) storageFootprintsTouchLocked(a, b *Tile) bool {
	if w == nil || a == nil || b == nil {
		return false
	}
	lowA, highA := blockFootprintRange(w.blockSizeForTileLocked(a))
	lowB, highB := blockFootprintRange(w.blockSizeForTileLocked(b))
	ax1, ax2 := a.X+lowA, a.X+highA
	ay1, ay2 := a.Y+lowA, a.Y+highA
	bx1, bx2 := b.X+lowB, b.X+highB
	by1, by2 := b.Y+lowB, b.Y+highB
	return ax1 <= bx2+1 && bx1 <= ax2+1 && ay1 <= by2+1 && by1 <= ay2+1
}

func (w *World) itemInventoryPosLocked(pos int32) (int32, bool) {
	if w.model == nil || pos < 0 || int(pos) >= len(w.model.Tiles) {
		return 0, false
	}
	cur := pos
	for i := 0; i < 3; i++ {
		tile := &w.model.Tiles[cur]
		name := w.blockNameByID(int16(tile.Block))
		if linked, ok := w.storageLinkedCore[cur]; ok && linked != cur {
			cur = linked
			continue
		}
		if isCoreBlockName(name) {
			if primary, ok := w.teamPrimaryCore[tile.Team]; ok && primary != cur {
				cur = primary
				continue
			}
		}
		return cur, true
	}
	return cur, true
}

func (w *World) sharedCoreInventoryLocked(pos int32) (TeamID, int32, *Building, bool) {
	if w.model == nil || pos < 0 || int(pos) >= len(w.model.Tiles) {
		return 0, 0, nil, false
	}
	tile := &w.model.Tiles[pos]
	name := w.blockNameByID(int16(tile.Block))
	if !isCoreBlockName(name) {
		if _, ok := w.storageLinkedCore[pos]; !ok {
			return 0, 0, nil, false
		}
	}
	proxyPos, ok := w.itemInventoryPosLocked(pos)
	if !ok || proxyPos < 0 || int(proxyPos) >= len(w.model.Tiles) {
		return 0, 0, nil, false
	}
	proxyTile := &w.model.Tiles[proxyPos]
	if proxyTile.Build == nil || proxyTile.Block == 0 {
		return 0, 0, nil, false
	}
	capacity := w.coreStorageCapacity[proxyPos]
	if capacity <= 0 {
		capacity = w.itemCapacityForBlockLocked(proxyTile)
	}
	return proxyTile.Team, proxyPos, proxyTile.Build, true
}

func (w *World) canUnloadItemFromBuildingLocked(pos int32, tile *Tile, item ItemID, allowCoreUnload bool) bool {
	if w == nil || tile == nil || tile.Build == nil || tile.Block == 0 {
		return false
	}
	name := w.blockNameByID(int16(tile.Block))
	if isStackConveyorBlock(name) {
		if !w.stackConveyorCanUnloadLocked(pos, tile) {
			return false
		}
		return w.itemAmountAtLocked(pos, item) > 0
	}
	switch name {
	case "conveyor", "titanium-conveyor", "armored-conveyor",
		"duct", "armored-duct", "duct-junction", "duct-router",
		"junction", "bridge-conveyor", "phase-conveyor",
		"overflow-duct", "underflow-duct", "overflow-gate", "underflow-gate",
		"router", "distributor", "sorter", "inverted-sorter",
		"unloader", "duct-unloader":
		return false
	}
	if _, _, _, ok := w.sharedCoreInventoryLocked(pos); ok {
		if !allowCoreUnload || !w.allowCoreUnloadersLocked() {
			return false
		}
	}
	return w.itemAmountAtLocked(pos, item) > 0
}

func (w *World) itemCapacityAtLocked(pos int32) int32 {
	if team, proxyPos, _, ok := w.sharedCoreInventoryLocked(pos); ok && team != 0 {
		if cap := w.coreStorageCapacity[proxyPos]; cap > 0 {
			return cap
		}
	}
	if w.model == nil || pos < 0 || int(pos) >= len(w.model.Tiles) {
		return 0
	}
	return w.itemCapacityForBlockLocked(&w.model.Tiles[pos])
}

func (w *World) itemAmountAtLocked(pos int32, item ItemID) int32 {
	if _, _, build, ok := w.sharedCoreInventoryLocked(pos); ok {
		return build.ItemAmount(item)
	}
	if w.model == nil || pos < 0 || int(pos) >= len(w.model.Tiles) {
		return 0
	}
	tile := &w.model.Tiles[pos]
	if tile.Build == nil {
		return 0
	}
	if w.buildingHidesInventoryItemsLocked(pos, tile) {
		return 0
	}
	return tile.Build.ItemAmount(item)
}

func (w *World) totalItemsAtLocked(pos int32) int32 {
	if _, _, build, ok := w.sharedCoreInventoryLocked(pos); ok {
		return totalBuildingItems(build)
	}
	if w.model == nil || pos < 0 || int(pos) >= len(w.model.Tiles) {
		return 0
	}
	tile := &w.model.Tiles[pos]
	if tile.Build == nil {
		return 0
	}
	if w.buildingHidesInventoryItemsLocked(pos, tile) {
		return 0
	}
	return totalBuildingItems(tile.Build)
}

func (w *World) addItemAtLocked(pos int32, item ItemID, amount int32) bool {
	if amount <= 0 {
		return false
	}
	if team, _, build, ok := w.sharedCoreInventoryLocked(pos); ok {
		build.AddItem(item, amount)
		w.emitTeamCoreItemsLocked(team, []ItemID{item})
		return true
	}
	if w.model == nil || pos < 0 || int(pos) >= len(w.model.Tiles) {
		return false
	}
	tile := &w.model.Tiles[pos]
	if tile.Build == nil {
		return false
	}
	tile.Build.AddItem(item, amount)
	w.emitBlockItemSyncLocked(pos)
	return true
}

func (w *World) removeItemAtLocked(pos int32, item ItemID, amount int32) bool {
	if amount <= 0 {
		return false
	}
	if team, _, build, ok := w.sharedCoreInventoryLocked(pos); ok {
		if !build.RemoveItem(item, amount) {
			return false
		}
		w.emitTeamCoreItemsLocked(team, []ItemID{item})
		return true
	}
	if w.model == nil || pos < 0 || int(pos) >= len(w.model.Tiles) {
		return false
	}
	tile := &w.model.Tiles[pos]
	if tile.Build == nil {
		return false
	}
	if w.buildingHidesInventoryItemsLocked(pos, tile) {
		return false
	}
	if !tile.Build.RemoveItem(item, amount) {
		return false
	}
	w.emitBlockItemSyncLocked(pos)
	return true
}

func (w *World) unloaderTargetItemLocked(pos int32, neighbors []int32) (ItemID, bool) {
	if item, ok := w.unloaderCfg[pos]; ok {
		if _, _, found := w.unloaderTransferPairPreviewLocked(pos, neighbors, item); found {
			return item, true
		}
		return 0, false
	}
	var itemsBuf [32]ItemID
	var seen itemIDSeenSet
	items := w.collectUnloaderCandidateItemIDsLocked(pos, neighbors, itemsBuf[:0], &seen)
	items = rotateItemIDsByStart(items, w.blockDumpIndex[pos])
	for _, item := range items {
		if _, _, found := w.unloaderTransferPairPreviewLocked(pos, neighbors, item); found {
			w.blockDumpIndex[pos] = int(item)
			return item, true
		}
	}
	return 0, false
}

func (w *World) collectUnloaderCandidateItemIDsLocked(pos int32, neighbors []int32, dst []ItemID, seen *itemIDSeenSet) []ItemID {
	for _, otherPos := range neighbors {
		if otherPos < 0 || int(otherPos) >= len(w.model.Tiles) || otherPos == pos {
			continue
		}
		other := &w.model.Tiles[otherPos]
		if other.Build == nil || other.Team != w.model.Tiles[pos].Team {
			continue
		}
		dst = w.appendInventoryItemIDsLocked(otherPos, dst, seen)
	}
	return dst
}

func (w *World) rotatedInventoryItemIDsLocked(pos int32, start int) []ItemID {
	items := make([]ItemID, 0, 8)
	var seen itemIDSeenSet
	items = w.appendInventoryItemIDsLocked(pos, items, &seen)
	return rotateItemIDsByStart(items, start)
}

func (w *World) appendInventoryItemIDsLocked(pos int32, dst []ItemID, seen *itemIDSeenSet) []ItemID {
	if seen == nil || w.model == nil || pos < 0 || int(pos) >= len(w.model.Tiles) {
		return dst
	}
	if _, _, build, ok := w.sharedCoreInventoryLocked(pos); ok {
		for _, stack := range build.Items {
			if stack.Amount <= 0 {
				continue
			}
			if seen.add(stack.Item) {
				dst = append(dst, stack.Item)
			}
		}
		return dst
	}
	tile := &w.model.Tiles[pos]
	if tile.Build == nil {
		return dst
	}
	if w.buildingHidesInventoryItemsLocked(pos, tile) {
		return dst
	}
	for _, stack := range tile.Build.Items {
		if stack.Amount <= 0 {
			continue
		}
		if seen.add(stack.Item) {
			dst = append(dst, stack.Item)
		}
	}
	return dst
}

func rotateItemIDsByStart(items []ItemID, start int) []ItemID {
	if len(items) == 0 {
		return nil
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i] < items[j]
	})
	if start <= 0 {
		return items
	}
	split := 0
	for split < len(items) && int(items[split]) < start {
		split++
	}
	if split == 0 || split >= len(items) {
		return items
	}
	reverseItemIDs(items[:split])
	reverseItemIDs(items[split:])
	reverseItemIDs(items)
	return items
}

func reverseItemIDs(items []ItemID) {
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
}

func boolCompare(a, b bool) int {
	switch {
	case a == b:
		return 0
	case !a && b:
		return -1
	default:
		return 1
	}
}

func unloaderLastUsedKey(unloaderPos, otherPos int32) int64 {
	return int64(uint32(unloaderPos))<<32 | int64(uint32(otherPos))
}

func (w *World) unloaderTransferPairPreviewLocked(pos int32, neighbors []int32, item ItemID) (int32, int32, bool) {
	return w.unloaderTransferPairInternalLocked(pos, neighbors, item, false)
}

func (w *World) unloaderTransferPairLocked(pos int32, neighbors []int32, item ItemID) (int32, int32, bool) {
	return w.unloaderTransferPairInternalLocked(pos, neighbors, item, true)
}

func (w *World) unloaderTransferPairInternalLocked(pos int32, neighbors []int32, item ItemID, updateLastUsed bool) (int32, int32, bool) {
	if w.model == nil || pos < 0 || int(pos) >= len(w.model.Tiles) {
		return 0, 0, false
	}

	var statsBuf [16]unloaderCandidateStat
	stats := statsBuf[:0]
	if len(neighbors) > len(statsBuf) {
		stats = make([]unloaderCandidateStat, 0, len(neighbors))
	}
	hasProvider := false
	hasReceiver := false
	isDistinct := false

	for _, otherPos := range neighbors {
		if otherPos < 0 || int(otherPos) >= len(w.model.Tiles) || otherPos == pos {
			continue
		}
		other := &w.model.Tiles[otherPos]
		if other.Build == nil || other.Block == 0 || other.Team != w.model.Tiles[pos].Team {
			continue
		}

		notStorage := !isStorageLikeBlock(w.blockNameByID(int16(other.Block)))
		canLoad := notStorage && w.canAcceptItemLocked(pos, otherPos, item, 0)
		canUnload := w.canUnloadItemFromBuildingLocked(otherPos, other, item, true)
		if !canLoad && !canUnload {
			continue
		}

		isDistinct = isDistinct || (hasProvider && canLoad) || (hasReceiver && canUnload)
		hasProvider = hasProvider || canUnload
		hasReceiver = hasReceiver || canLoad

		cap := w.unloaderMaximumAcceptedItemLocked(otherPos, other, item)
		loadFactor := float32(0)
		if cap > 0 {
			loadFactor = float32(w.itemAmountAtLocked(otherPos, item)) / float32(cap)
		}

		lastUsed := w.unloaderLastUsed[unloaderLastUsedKey(pos, otherPos)] + 1
		if updateLastUsed {
			w.unloaderLastUsed[unloaderLastUsedKey(pos, otherPos)] = lastUsed
		}

		stats = append(stats, unloaderCandidateStat{
			pos:        otherPos,
			loadFactor: loadFactor,
			canLoad:    canLoad,
			canUnload:  canUnload,
			notStorage: notStorage,
			lastUsed:   lastUsed,
		})
	}

	if !isDistinct || len(stats) < 2 {
		return 0, 0, false
	}

	sort.SliceStable(stats, func(i, j int) bool {
		x, y := stats[i], stats[j]
		if cmp := boolCompare(!x.notStorage, !y.notStorage); cmp != 0 {
			return cmp < 0
		}
		if cmp := boolCompare(x.canUnload && !x.canLoad, y.canUnload && !y.canLoad); cmp != 0 {
			return cmp < 0
		}
		if cmp := boolCompare(x.canUnload || !x.canLoad, y.canUnload || !y.canLoad); cmp != 0 {
			return cmp < 0
		}
		if x.loadFactor != y.loadFactor {
			return x.loadFactor < y.loadFactor
		}
		return x.lastUsed > y.lastUsed
	})

	var dumpingTo *unloaderCandidateStat
	var dumpingFrom *unloaderCandidateStat
	for i := range stats {
		if stats[i].canLoad {
			dumpingTo = &stats[i]
			break
		}
	}
	for i := len(stats) - 1; i >= 0; i-- {
		if stats[i].canUnload {
			dumpingFrom = &stats[i]
			break
		}
	}

	if dumpingFrom == nil || dumpingTo == nil || dumpingFrom.pos == dumpingTo.pos {
		return 0, 0, false
	}
	if dumpingFrom.loadFactor == dumpingTo.loadFactor && dumpingFrom.canLoad {
		return 0, 0, false
	}
	if updateLastUsed {
		w.unloaderLastUsed[unloaderLastUsedKey(pos, dumpingTo.pos)] = 0
		w.unloaderLastUsed[unloaderLastUsedKey(pos, dumpingFrom.pos)] = 0
	}
	return dumpingFrom.pos, dumpingTo.pos, true
}

func (w *World) unloaderMaximumAcceptedItemLocked(pos int32, tile *Tile, item ItemID) int32 {
	if w == nil || tile == nil || tile.Build == nil || tile.Block == 0 {
		return 0
	}
	switch w.blockNameByID(int16(tile.Block)) {
	case "ground-factory", "air-factory", "naval-factory",
		"additive-reconstructor", "multiplicative-reconstructor", "exponential-reconstructor", "tetrative-reconstructor",
		"tank-refabricator", "ship-refabricator", "mech-refabricator", "prime-refabricator":
		return w.maximumAcceptedItemForBlockLocked(pos, tile, item)
	default:
		return w.itemCapacityAtLocked(pos)
	}
}

func (w *World) dumpProximityLocked(pos int32) []int32 {
	if w.model == nil || pos < 0 || int(pos) >= len(w.model.Tiles) {
		return nil
	}
	tile := &w.model.Tiles[pos]
	neighbors, ok := w.dumpNeighborCache[pos]
	if !ok {
		offsets := blockEdgeOffsets(w.blockSizeForTileLocked(tile))
		neighbors = make([]int32, 0, len(offsets))
		for _, off := range offsets {
			otherPos, ok := w.buildingOccupyingCellLocked(tile.X+off[0], tile.Y+off[1])
			if !ok || otherPos == pos {
				continue
			}
			other := &w.model.Tiles[otherPos]
			if other.Build == nil || other.Block == 0 || other.Team != tile.Team {
				continue
			}
			duplicate := false
			for _, existing := range neighbors {
				if existing == otherPos {
					duplicate = true
					break
				}
			}
			if duplicate {
				continue
			}
			neighbors = append(neighbors, otherPos)
		}
		w.dumpNeighborCache[pos] = neighbors
	}
	return neighbors
}

func (w *World) advanceDumpIndexLocked(pos int32, next int, count int) {
	if count <= 0 {
		delete(w.blockDumpIndex, pos)
		return
	}
	w.blockDumpIndex[pos] = ((next % count) + count) % count
}

func (w *World) dumpSingleItemLocked(pos int32, tile *Tile, specific *ItemID, canDump func(int32, ItemID) bool) bool {
	if tile == nil || tile.Build == nil || w.model == nil || len(tile.Build.Items) == 0 {
		return false
	}
	neighbors := w.dumpProximityLocked(pos)
	if len(neighbors) == 0 {
		return false
	}
	start := 0
	if idx, ok := w.blockDumpIndex[pos]; ok {
		start = ((idx % len(neighbors)) + len(neighbors)) % len(neighbors)
	}
	for i := 0; i < len(neighbors); i++ {
		index := (start + i) % len(neighbors)
		target := neighbors[index]
		tryDump := func(item ItemID) bool {
			if canDump != nil && !canDump(target, item) {
				return false
			}
			if !w.tryInsertItemLocked(pos, target, item, 0) {
				return false
			}
			if tile.Build.RemoveItem(item, 1) {
				w.emitBlockItemSyncLocked(pos)
				w.advanceDumpIndexLocked(pos, index+1, len(neighbors))
				return true
			}
			return false
		}
		if specific != nil {
			if tile.Build.ItemAmount(*specific) > 0 && tryDump(*specific) {
				return true
			}
		} else {
			for _, stack := range tile.Build.Items {
				if stack.Amount <= 0 {
					continue
				}
				if tryDump(stack.Item) {
					return true
				}
			}
		}
		w.advanceDumpIndexLocked(pos, index+1, len(neighbors))
	}
	return false
}

func (w *World) offloadProducedItemLocked(pos int32, tile *Tile, item ItemID) bool {
	if tile == nil || tile.Build == nil || w.model == nil {
		return false
	}
	if target, ok := w.dumpTargetLocked(pos, tile, item); ok {
		if w.tryInsertItemLocked(pos, target, item, 0) {
			return true
		}
	}
	if totalBuildingItems(tile.Build) >= w.itemCapacityForBlockLocked(tile) {
		return false
	}
	tile.Build.AddItem(item, 1)
	w.emitBlockItemSyncLocked(pos)
	return true
}

func (w *World) bridgeDumpTargetLocked(pos int32, tile *Tile, item ItemID) (int32, bool) {
	neighbors := w.dumpProximityLocked(pos)
	if len(neighbors) == 0 {
		return 0, false
	}
	start := 0
	if idx, ok := w.blockDumpIndex[pos]; ok && len(neighbors) > 0 {
		start = ((idx % len(neighbors)) + len(neighbors)) % len(neighbors)
	}
	for i := 0; i < len(neighbors); i++ {
		index := (start + i) % len(neighbors)
		target := neighbors[index]
		other := &w.model.Tiles[target]
		side, ok := flowDir(tile.X, tile.Y, other.X, other.Y)
		if !ok {
			w.blockDumpIndex[pos] = (index + 1) % len(neighbors)
			continue
		}
		if !w.bridgeHasIncomingFromSideLocked(pos, side) && w.canAcceptItemLocked(pos, target, item, 0) {
			w.blockDumpIndex[pos] = (index + 1) % len(neighbors)
			return target, true
		}
		w.blockDumpIndex[pos] = (index + 1) % len(neighbors)
	}
	return 0, false
}

func (w *World) directionBridgeTargetLocked(pos int32, tile *Tile, want string, maxRange int) (int32, bool) {
	if w.model == nil || tile == nil {
		return 0, false
	}
	dx, dy := dirDelta(tile.Rotation)
	for i := 1; i <= maxRange; i++ {
		nx, ny := tile.X+dx*i, tile.Y+dy*i
		if !w.model.InBounds(nx, ny) {
			break
		}
		target := int32(ny*w.model.Width + nx)
		other := &w.model.Tiles[target]
		if other.Build == nil || other.Team != tile.Team {
			continue
		}
		if w.blockNameByID(int16(other.Block)) == want {
			return target, true
		}
	}
	return 0, false
}

func (w *World) dumpTargetLocked(pos int32, tile *Tile, item ItemID) (int32, bool) {
	neighbors := w.dumpProximityLocked(pos)
	if len(neighbors) == 0 {
		return 0, false
	}
	start := 0
	if idx, ok := w.blockDumpIndex[pos]; ok && len(neighbors) > 0 {
		start = ((idx % len(neighbors)) + len(neighbors)) % len(neighbors)
	}
	for i := 0; i < len(neighbors); i++ {
		index := (start + i) % len(neighbors)
		target := neighbors[index]
		if w.canAcceptItemLocked(pos, target, item, 0) {
			w.blockDumpIndex[pos] = (index + 1) % len(neighbors)
			return target, true
		}
		w.blockDumpIndex[pos] = (index + 1) % len(neighbors)
	}
	return 0, false
}

func (w *World) ductRouterTargetLocked(pos int32, tile *Tile, item ItemID) (int32, bool) {
	neighbors := w.dumpProximityLocked(pos)
	if len(neighbors) == 0 || tile == nil {
		return 0, false
	}
	filter, hasFilter := w.sorterCfg[pos]
	start := 0
	if idx, ok := w.blockDumpIndex[pos]; ok && len(neighbors) > 0 {
		start = ((idx % len(neighbors)) + len(neighbors)) % len(neighbors)
	}
	for i := 0; i < len(neighbors); i++ {
		index := (start + i) % len(neighbors)
		target := neighbors[index]
		other := &w.model.Tiles[target]
		rel, ok := relativeDir(other.X, other.Y, tile.X, tile.Y)
		if !ok || rel == byte((int(tile.Rotation)+2)%4) {
			continue
		}
		if hasFilter && ((item == filter) != (rel == byte(tile.Rotation))) {
			continue
		}
		if w.canAcceptItemLocked(pos, target, item, 0) {
			w.advanceDumpIndexLocked(pos, index+1, len(neighbors))
			return target, true
		}
	}
	return 0, false
}

func (w *World) routerTargetLocked(pos int32, tile *Tile, item ItemID, set bool) (int32, bool) {
	neighbors := w.dumpProximityLocked(pos)
	if len(neighbors) == 0 {
		return 0, false
	}
	inPos := w.routerInputPos[pos]
	if st, ok := w.routerStates[pos]; ok && st != nil && st.LastInput >= 0 {
		inPos = st.LastInput
	}
	start := int(w.routerRotation[pos] % byte(len(neighbors)))
	skipInput := false
	if inPos >= 0 && int(inPos) < len(w.model.Tiles) {
		skipInput = w.blockNameByID(int16(w.model.Tiles[inPos].Block)) == "overflow-gate"
	}
	for i := 0; i < len(neighbors); i++ {
		outPos := neighbors[(start+i)%len(neighbors)]
		if set {
			w.routerRotation[pos] = byte((int(w.routerRotation[pos]) + 1) % len(neighbors))
		}
		if skipInput && outPos == inPos {
			continue
		}
		if w.canAcceptItemLocked(pos, outPos, item, 0) {
			return outPos, true
		}
	}
	return 0, false
}

func (w *World) overflowDuctTargetLocked(pos int32, tile *Tile, item ItemID, invert bool) (int32, bool) {
	if w.model == nil || tile == nil {
		return 0, false
	}
	useLeft := w.blockDumpIndex[pos] == 0
	tryDir := func(dir byte) (int32, bool) {
		target, ok := w.forwardItemTargetPosLocked(pos, int8(dir))
		if !ok {
			return 0, false
		}
		if !w.canAcceptItemLocked(pos, target, item, 0) {
			return 0, false
		}
		return target, true
	}
	leftDir := byte((int(tile.Rotation) + 3) % 4)
	rightDir := byte((int(tile.Rotation) + 1) % 4)
	if invert {
		left, lok := tryDir(leftDir)
		right, rok := tryDir(rightDir)
		if lok && !rok {
			return left, true
		}
		if rok && !lok {
			return right, true
		}
		if lok && rok {
			if useLeft {
				return left, true
			}
			return right, true
		}
		return 0, false
	}
	if front, ok := tryDir(byte(tile.Rotation)); ok {
		return front, true
	}
	left, lok := tryDir(leftDir)
	right, rok := tryDir(rightDir)
	if lok && !rok {
		return left, true
	}
	if rok && !lok {
		return right, true
	}
	if lok && rok {
		if useLeft {
			return left, true
		}
		return right, true
	}
	return 0, false
}

func (w *World) sorterTargetLocked(fromPos, sorterPos int32, item ItemID, invert bool, flip bool) (int32, bool) {
	fromTile := &w.model.Tiles[fromPos]
	sorterTile := &w.model.Tiles[sorterPos]
	sourceSide, ok := relativeDir(fromTile.X, fromTile.Y, sorterTile.X, sorterTile.Y)
	if !ok {
		return 0, false
	}
	dir := oppositeDir(sourceSide)
	filter, hasFilter := w.sorterCfg[sorterPos]
	match := hasFilter && filter == item
	fromInst := isInstantTransferBlock(w.blockNameByID(int16(fromTile.Block)))
	if match != invert {
		out, ok := w.forwardItemTargetPosLocked(sorterPos, int8(dir))
		if ok && (!fromInst || !isInstantTransferBlock(w.blockNameByID(int16(w.model.Tiles[out].Block)))) && w.canAcceptItemLocked(sorterPos, out, item, 1) {
			return out, true
		}
		return 0, false
	}
	leftDir := byte((int(dir) + 3) % 4)
	rightDir := byte((int(dir) + 1) % 4)
	left, lok := w.forwardItemTargetPosLocked(sorterPos, int8(leftDir))
	right, rok := w.forwardItemTargetPosLocked(sorterPos, int8(rightDir))
	canLeft := lok && (!fromInst || !isInstantTransferBlock(w.blockNameByID(int16(w.model.Tiles[left].Block)))) && w.canAcceptItemLocked(sorterPos, left, item, 1)
	canRight := rok && (!fromInst || !isInstantTransferBlock(w.blockNameByID(int16(w.model.Tiles[right].Block)))) && w.canAcceptItemLocked(sorterPos, right, item, 1)
	if canLeft && !canRight {
		return left, true
	}
	if canRight && !canLeft {
		return right, true
	}
	if canLeft && canRight {
		bit := byte(1 << dir)
		useLeft := (w.routerRotation[sorterPos] & bit) == 0
		if flip {
			w.routerRotation[sorterPos] ^= bit
		}
		if useLeft {
			return left, true
		}
		return right, true
	}
	return 0, false
}

func flowDir(fromX, fromY, toX, toY int) (byte, bool) {
	side, ok := relativeDir(fromX, fromY, toX, toY)
	if !ok {
		return 0, false
	}
	return oppositeDir(side), true
}

func oppositeDir(dir byte) byte {
	return byte((int(dir) + 2) % 4)
}

func (w *World) overflowTargetLocked(fromPos, gatePos int32, item ItemID, invert bool, flip bool) (int32, bool) {
	fromTile := &w.model.Tiles[fromPos]
	fromDir, ok := w.relativeToEdgeLocked(fromPos, gatePos)
	if !ok {
		return 0, false
	}
	fromInst := isInstantTransferBlock(w.blockNameByID(int16(fromTile.Block)))
	forwardDir := byte((int(fromDir) + 2) % 4)
	forward, fok := w.forwardItemTargetPosLocked(gatePos, int8(forwardDir))
	canForward := fok && (!fromInst || !isInstantTransferBlock(w.blockNameByID(int16(w.model.Tiles[forward].Block)))) && w.canAcceptItemLocked(gatePos, forward, item, 1)
	if !canForward || invert {
		leftDir := byte((int(fromDir) + 3) % 4)
		rightDir := byte((int(fromDir) + 1) % 4)
		left, lok := w.forwardItemTargetPosLocked(gatePos, int8(leftDir))
		right, rok := w.forwardItemTargetPosLocked(gatePos, int8(rightDir))
		canLeft := lok && (!fromInst || !isInstantTransferBlock(w.blockNameByID(int16(w.model.Tiles[left].Block)))) && w.canAcceptItemLocked(gatePos, left, item, 1)
		canRight := rok && (!fromInst || !isInstantTransferBlock(w.blockNameByID(int16(w.model.Tiles[right].Block)))) && w.canAcceptItemLocked(gatePos, right, item, 1)
		if !canLeft && !canRight {
			if invert && canForward {
				return forward, true
			}
			return 0, false
		}
		if canLeft && !canRight {
			return left, true
		}
		if canRight && !canLeft {
			return right, true
		}
		bit := byte(1 << fromDir)
		useLeft := (w.routerRotation[gatePos] & bit) == 0
		if flip {
			w.routerRotation[gatePos] ^= bit
		}
		if useLeft {
			return left, true
		}
		return right, true
	}
	return forward, true
}

func containsItemInStacks(stacks []ItemStack, item ItemID) bool {
	for _, stack := range stacks {
		if stack.Item == item && stack.Amount > 0 {
			return true
		}
	}
	return false
}

func (w *World) maximumAcceptedItemForBlockLocked(pos int32, tile *Tile, item ItemID) int32 {
	if w == nil || tile == nil || tile.Build == nil || tile.Block == 0 {
		return 0
	}
	return w.maximumAcceptedItemForBlockNameLocked(pos, tile, w.blockNameByID(int16(tile.Block)), item)
}

func (w *World) maximumAcceptedItemForBlockNameLocked(pos int32, tile *Tile, name string, item ItemID) int32 {
	if w == nil || tile == nil || tile.Build == nil || tile.Block == 0 || name == "" {
		return 0
	}
	switch name {
	case "core-shard", "core-foundation", "core-nucleus", "core-bastion", "core-citadel", "core-acropolis",
		"container", "vault", "reinforced-container", "reinforced-vault":
		return w.itemCapacityAtLocked(pos)
	case "payload-loader", "payload-unloader":
		return w.itemCapacityForBlockLocked(tile)
	case "ground-factory", "air-factory", "naval-factory":
		plan, ok := w.unitFactorySelectedPlanLocked(pos, tile)
		if !ok || !containsItemInStacks(plan.Cost, item) {
			return 0
		}
		return unitFactoryScaledAmount(unitFactoryItemCapacity(name, item), w.unitCostMultiplierLocked(tile.Build.Team))
	case "additive-reconstructor", "multiplicative-reconstructor", "exponential-reconstructor", "tetrative-reconstructor",
		"tank-refabricator", "ship-refabricator", "mech-refabricator", "prime-refabricator":
		return w.reconstructorMaximumAcceptedItemLocked(tile, item)
	case "combustion-generator", "steam-generator":
		if item == coalItemID || item == pyratiteItemID || item == sporePodItemID {
			return w.itemCapacityForBlockLocked(tile)
		}
		return 0
	case "differential-generator":
		if item == pyratiteItemID {
			return w.itemCapacityForBlockLocked(tile)
		}
		return 0
	case "rtg-generator":
		if item == phaseFabricItemID || item == thoriumItemID || item == legacyThoriumItemID {
			return w.itemCapacityForBlockLocked(tile)
		}
		return 0
	case "thorium-reactor":
		if item == thoriumItemID || item == legacyThoriumItemID {
			return w.itemCapacityForBlockLocked(tile)
		}
		return 0
	case "impact-reactor":
		if item == blastCompoundItemID {
			return w.itemCapacityForBlockLocked(tile)
		}
		return 0
	case "neoplasia-reactor":
		if item == phaseFabricItemID {
			return w.itemCapacityForBlockLocked(tile)
		}
		return 0
	}
	if prof, ok := crafterProfilesByBlockName[name]; ok {
		if containsItemInStacks(prof.InputItems, item) {
			return w.itemCapacityForBlockLocked(tile)
		}
		return 0
	}
	if prof, ok := separatorProfilesByBlockName[name]; ok {
		if containsItemInStacks(prof.InputItems, item) {
			return w.itemCapacityForBlockLocked(tile)
		}
		return 0
	}
	if prof, ok := solidPumpProfilesByBlockName[name]; ok {
		if prof.ItemUseTimeFrames > 0 && prof.ItemConsume == item {
			return w.itemCapacityForBlockLocked(tile)
		}
		return 0
	}
	return 0
}

func (w *World) buildingUsesTotalItemCapacityLocked(pos int32, tile *Tile) bool {
	if w == nil || tile == nil || tile.Build == nil || tile.Block == 0 {
		return false
	}
	if _, _, _, shared := w.sharedCoreInventoryLocked(pos); shared {
		return false
	}
	return buildingUsesTotalItemCapacityName(w.blockNameByID(int16(tile.Block)))
}

func buildingUsesTotalItemCapacityName(name string) bool {
	return name == "container" || name == "vault" || name == "reinforced-container" || name == "reinforced-vault"
}

func (w *World) acceptsBuildingItemLocked(pos int32, tile *Tile, item ItemID) bool {
	if tile != nil && tile.Build != nil {
		if prof, ok := w.getBuildingWeaponProfile(int16(tile.Build.Block)); ok && w.buildingUsesItemAmmoLocked(tile, prof) {
			return w.turretAcceptItemLocked(pos, tile, item)
		}
	}
	cap := w.maximumAcceptedItemForBlockLocked(pos, tile, item)
	if cap <= 0 {
		return false
	}
	if w.buildingUsesTotalItemCapacityLocked(pos, tile) {
		return w.totalItemsAtLocked(pos) < cap
	}
	return w.itemAmountAtLocked(pos, item) < cap
}

func (w *World) storeAcceptedBuildingItemLocked(pos int32, tile *Tile, item ItemID, amount int32) bool {
	if amount <= 0 {
		return false
	}
	name := ""
	if tile != nil && tile.Build != nil {
		name = w.blockNameByID(int16(tile.Block))
		if name == "" {
			name = w.blockNameByID(int16(tile.Build.Block))
		}
		if prof, ok := w.buildingWeaponProfileByNameLocked(name); ok && classifyTurretBlockSyncKind(name, prof) == blockSyncItemTurret {
			return w.turretHandleItemLocked(pos, tile, item, amount)
		}
	}
	cap := w.maximumAcceptedItemForBlockNameLocked(pos, tile, name, item)
	if cap <= 0 {
		return false
	}
	usesTotal := buildingUsesTotalItemCapacityName(name)
	if usesTotal {
		if _, _, _, shared := w.sharedCoreInventoryLocked(pos); shared {
			usesTotal = false
		}
	}
	if usesTotal {
		if w.totalItemsAtLocked(pos)+amount > cap {
			return false
		}
	} else if w.itemAmountAtLocked(pos, item)+amount > cap {
		return false
	}
	return w.addItemAtLocked(pos, item, amount)
}

func (w *World) canAcceptItemLocked(fromPos, toPos int32, item ItemID, depth int) bool {
	if depth > 8 || w.model == nil || toPos < 0 || int(toPos) >= len(w.model.Tiles) {
		return false
	}
	fromTile := &w.model.Tiles[fromPos]
	toTile := &w.model.Tiles[toPos]
	if toTile.Build == nil || toTile.Block == 0 || toTile.Team != fromTile.Team {
		return false
	}
	switch w.blockNameByID(int16(toTile.Block)) {
	case "conveyor", "titanium-conveyor", "armored-conveyor":
		return w.conveyorAcceptsItemLocked(fromPos, toPos)
	case "duct":
		return w.ductAcceptsItemLocked(fromPos, toPos, false)
	case "armored-duct":
		return w.ductAcceptsItemLocked(fromPos, toPos, true)
	case "duct-router", "surge-router":
		return w.ductRouterAcceptsItemLocked(fromPos, toPos, item)
	case "overflow-duct", "underflow-duct":
		return w.ductAcceptsItemLocked(fromPos, toPos, false)
	case "duct-bridge":
		return w.ductBridgeAcceptsItemLocked(fromPos, toPos, item)
	case "duct-unloader":
		return false
	case "bridge-conveyor", "phase-conveyor":
		return w.bridgeAllowsInputLocked(fromPos, toPos) && w.totalItemsAtLocked(toPos) < w.itemCapacityAtLocked(toPos)
	case "mass-driver":
		_, ok := w.massDriverTargetLocked(toPos, toTile)
		return ok && w.totalItemsAtLocked(toPos) < w.itemCapacityAtLocked(toPos)
	case "router", "distributor":
		st := w.routerStateLocked(toPos, toTile)
		return !st.HasItem && totalBuildingItems(toTile.Build) < 1
	case "plastanium-conveyor", "surge-conveyor":
		return w.stackConveyorAcceptsItemLocked(fromPos, toPos, item)
	case "junction":
		dir, ok := flowDir(fromTile.X, fromTile.Y, toTile.X, toTile.Y)
		if !ok {
			return false
		}
		outPos, ok := w.forwardItemTargetPosLocked(toPos, int8(dir))
		if !ok {
			return false
		}
		outTile := &w.model.Tiles[outPos]
		if outTile.Build == nil || outTile.Block == 0 || outTile.Team != toTile.Team {
			return false
		}
		state := w.junctionQueues[toPos]
		return len(state[dir]) < 6
	case "sorter", "inverted-sorter":
		target, ok := w.sorterTargetLocked(fromPos, toPos, item, w.blockNameByID(int16(toTile.Block)) == "inverted-sorter", false)
		return ok && w.canAcceptItemLocked(toPos, target, item, depth+1)
	case "overflow-gate":
		target, ok := w.overflowTargetLocked(fromPos, toPos, item, false, false)
		return ok && w.canAcceptItemLocked(toPos, target, item, depth+1)
	case "underflow-gate":
		target, ok := w.overflowTargetLocked(fromPos, toPos, item, true, false)
		return ok && w.canAcceptItemLocked(toPos, target, item, depth+1)
	case "thorium-reactor":
		return w.acceptsBuildingItemLocked(toPos, toTile, item)
	case "item-void":
		return true
	case "incinerator", "slag-incinerator":
		return w.incineratorAcceptsItemLocked(toPos)
	default:
		return w.acceptsBuildingItemLocked(toPos, toTile, item)
	}
}

func (w *World) forwardPosLocked(pos int32, rotation int8) (int32, bool) {
	if w.model == nil || pos < 0 || int(pos) >= len(w.model.Tiles) {
		return 0, false
	}
	tile := &w.model.Tiles[pos]
	dx, dy := dirDelta(rotation)
	nx, ny := tile.X+dx, tile.Y+dy
	if !w.model.InBounds(nx, ny) {
		return 0, false
	}
	return int32(ny*w.model.Width + nx), true
}

func (w *World) forwardItemTargetPosLocked(pos int32, rotation int8) (int32, bool) {
	targetPos, ok := w.forwardPosLocked(pos, rotation)
	if !ok || w.model == nil || targetPos < 0 || int(targetPos) >= len(w.model.Tiles) {
		return 0, false
	}
	tile := &w.model.Tiles[targetPos]
	if occ, ok := w.buildingOccupyingCellLocked(tile.X, tile.Y); ok {
		return occ, true
	}
	return targetPos, true
}

