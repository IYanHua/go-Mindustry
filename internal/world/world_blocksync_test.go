package world

import (
	"math"
	"testing"
	"time"

	"github.com/IYanHua/mdt-server/internal/protocol"
)

func TestBlockSyncSnapshotsEncodeGenericCrafterRuntime(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		183: "silicon-smelter",
	}
	w.SetModel(model)
	tile := placeTestBuilding(t, w, 10, 10, 183, 1, 2)
	pos := int32(tile.Y*w.Model().Width + tile.X)
	tile.Build.Items = []ItemStack{
		{Item: coalItemID, Amount: 2},
		{Item: sandItemID, Amount: 4},
	}
	w.crafterStates[pos] = crafterRuntimeState{
		Progress: 0.625,
		Warmup:   0.5,
	}
	w.rebuildActiveTilesLocked()

	snaps := w.BlockSyncSnapshots()
	if len(snaps) != 1 {
		t.Fatalf("expected one block sync snapshot, got %d", len(snaps))
	}
	if snaps[0].Pos != protocol.PackPoint2(10, 10) {
		t.Fatalf("unexpected snapshot pos=%d", snaps[0].Pos)
	}
	base, r := decodeBlockSyncBase(t, snaps[0].Data)
	if base.Version != 3 {
		t.Fatalf("expected sync base version 3, got %d", base.Version)
	}
	if (base.ModuleBits & 1) == 0 {
		t.Fatalf("expected item module bit to be present, bits=%08b", base.ModuleBits)
	}
	if (base.ModuleBits & (1 << 1)) == 0 {
		t.Fatalf("expected power module bit to be present, bits=%08b", base.ModuleBits)
	}
	if got := base.Items[coalItemID]; got != 2 {
		t.Fatalf("expected coal amount 2, got %d", got)
	}
	if got := base.Items[sandItemID]; got != 4 {
		t.Fatalf("expected sand amount 4, got %d", got)
	}
	progress, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read crafter progress failed: %v", err)
	}
	warmup, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read crafter warmup failed: %v", err)
	}
	if math.Abs(float64(progress-0.625)) > 0.0001 {
		t.Fatalf("expected progress 0.625, got %f", progress)
	}
	if math.Abs(float64(warmup-0.5)) > 0.0001 {
		t.Fatalf("expected warmup 0.5, got %f", warmup)
	}
	if rem := r.Remaining(); rem != 0 {
		t.Fatalf("expected crafter sync payload to be fully consumed, remaining=%d", rem)
	}
}

func TestBlockSyncSnapshotsEncodePowerNodeRuntime(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		425: "power-node-large",
		430: "laser-drill",
	}
	w.SetModel(model)
	nodeTile := placeTestBuilding(t, w, 12, 10, 425, 1, 0)
	placeTestBuilding(t, w, 6, 10, 430, 1, 0)
	nodePos := int32(nodeTile.Y*w.Model().Width + nodeTile.X)
	w.applyBuildingConfigLocked(nodePos, []protocol.Point2{{X: -6, Y: 0}}, true)
	w.rebuildActiveTilesLocked()

	snaps := w.BlockSyncSnapshots()
	if len(snaps) == 0 {
		t.Fatal("expected at least one power-node snapshot")
	}
	var nodeSnap *BlockSyncSnapshot
	for i := range snaps {
		if snaps[i].Pos == protocol.PackPoint2(12, 10) {
			nodeSnap = &snaps[i]
			break
		}
	}
	if nodeSnap == nil {
		t.Fatalf("expected power-node snapshot at pos=%d, got %+v", protocol.PackPoint2(12, 10), snaps)
	}
	base, r := decodeBlockSyncBase(t, nodeSnap.Data)
	if (base.ModuleBits & (1 << 1)) == 0 {
		t.Fatalf("expected power module bit for power node, bits=%08b", base.ModuleBits)
	}
	targetPacked := protocol.PackPoint2(6, 10)
	if len(base.PowerLinks) != 1 || base.PowerLinks[0] != targetPacked {
		t.Fatalf("expected power-node runtime link to target packed=%d, got %v", targetPacked, base.PowerLinks)
	}
	if base.PowerStatus != 0 {
		t.Fatalf("expected power-node to sync vanilla non-consumer power status 0, got %f", base.PowerStatus)
	}
	if rem := r.Remaining(); rem != 0 {
		t.Fatalf("expected power-node sync payload to be fully consumed, remaining=%d", rem)
	}
}

func TestBlockSyncSnapshotsEncodeConductivePowerLinks(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		421: "battery",
		430: "laser-drill",
	}
	w.SetModel(model)
	placeTestBuilding(t, w, 6, 10, 421, 1, 0)
	placeTestBuilding(t, w, 7, 10, 430, 1, 0)
	w.rebuildActiveTilesLocked()

	snaps := w.BlockSyncSnapshots()
	if len(snaps) != 2 {
		t.Fatalf("expected two conductive block snapshots, got %d", len(snaps))
	}

	var batterySnap *BlockSyncSnapshot
	for i := range snaps {
		if snaps[i].Pos == protocol.PackPoint2(6, 10) {
			batterySnap = &snaps[i]
			break
		}
	}
	if batterySnap == nil {
		t.Fatalf("expected battery snapshot at pos=%d", protocol.PackPoint2(6, 10))
	}
	base, r := decodeBlockSyncBase(t, batterySnap.Data)
	if len(base.PowerLinks) != 0 {
		t.Fatalf("expected conductive adjacency to stay out of PowerModule.links, got %v", base.PowerLinks)
	}
	if rem := r.Remaining(); rem != 0 {
		t.Fatalf("expected conductive power snapshot payload to be fully consumed, remaining=%d", rem)
	}
}

func TestBlockSyncSnapshotsEncodeReversePowerNodeLinksOnConsumers(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		422: "power-node",
		421: "battery",
	}
	w.SetModel(model)
	node := placeTestBuilding(t, w, 8, 10, 422, 1, 0)
	battery := placeTestBuilding(t, w, 14, 10, 421, 1, 0)
	nodePacked := protocol.PackPoint2(int32(node.X), int32(node.Y))
	batteryPacked := protocol.PackPoint2(int32(battery.X), int32(battery.Y))
	nodePos := int32(node.Y*w.Model().Width + node.X)
	w.applyBuildingConfigLocked(nodePos, []protocol.Point2{{X: 6, Y: 0}}, true)
	w.rebuildActiveTilesLocked()

	snaps := w.BlockSyncSnapshots()
	if len(snaps) != 2 {
		t.Fatalf("expected node and drill snapshots, got %d", len(snaps))
	}

	var batterySnap *BlockSyncSnapshot
	for i := range snaps {
		if snaps[i].Pos == protocol.PackPoint2(14, 10) {
			batterySnap = &snaps[i]
			break
		}
	}
	if batterySnap == nil {
		t.Fatalf("expected battery snapshot at pos=%d", protocol.PackPoint2(14, 10))
	}
	base, r := decodeBlockSyncBase(t, batterySnap.Data)
	if len(base.PowerLinks) != 1 || base.PowerLinks[0] != nodePacked {
		t.Fatalf("expected battery power module to keep reverse node packed link %d, got %v", nodePacked, base.PowerLinks)
	}
	if base.PowerStatus != 0 {
		t.Fatalf("expected idle battery power status 0 without stored charge, got %f", base.PowerStatus)
	}
	if rem := r.Remaining(); rem != 0 {
		t.Fatalf("expected reverse node-link snapshot payload to be fully consumed, remaining=%d", rem)
	}

	var nodeSnap *BlockSyncSnapshot
	for i := range snaps {
		if snaps[i].Pos == protocol.PackPoint2(8, 10) {
			nodeSnap = &snaps[i]
			break
		}
	}
	if nodeSnap == nil {
		t.Fatalf("expected power-node snapshot at pos=%d", protocol.PackPoint2(8, 10))
	}
	nodeBase, rr := decodeBlockSyncBase(t, nodeSnap.Data)
	if len(nodeBase.PowerLinks) != 1 || nodeBase.PowerLinks[0] != batteryPacked {
		t.Fatalf("expected power-node to keep forward battery packed link %d, got %v", batteryPacked, nodeBase.PowerLinks)
	}
	if rem := rr.Remaining(); rem != 0 {
		t.Fatalf("expected node snapshot payload to be fully consumed, remaining=%d", rem)
	}
}

func TestPowerNodeConfigCreatesSymmetricRuntimeLinks(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		421: "battery",
		422: "power-node",
	}
	w.SetModel(model)

	node := placeTestBuilding(t, w, 8, 10, 422, 1, 0)
	battery := placeTestBuilding(t, w, 14, 10, 421, 1, 0)
	nodePos := int32(node.Y*w.Model().Width + node.X)
	batteryPos := int32(battery.Y*w.Model().Width + battery.X)

	w.applyBuildingConfigLocked(nodePos, []protocol.Point2{{X: 6, Y: 0}}, true)

	if links := w.powerNodeLinks[nodePos]; len(links) != 1 || links[0] != batteryPos {
		t.Fatalf("expected node runtime links [%d], got %v", batteryPos, links)
	}
	if links := w.powerNodeLinks[batteryPos]; len(links) != 1 || links[0] != nodePos {
		t.Fatalf("expected battery reverse runtime links [%d], got %v", nodePos, links)
	}
	if node.Build == nil || len(node.Build.Config) == 0 {
		t.Fatal("expected power-node stored config to stay in sync with runtime links")
	}
	decoded, ok := decodeStoredBuildingConfig(node.Build.Config)
	if !ok {
		t.Fatal("expected power-node stored config to decode")
	}
	points, ok := decoded.([]protocol.Point2)
	if !ok || len(points) != 1 || points[0].X != 6 || points[0].Y != 0 {
		t.Fatalf("expected stored power-node config [{6 0}], got %T %#v", decoded, decoded)
	}
	if battery.Build != nil && len(battery.Build.Config) != 0 {
		t.Fatalf("expected battery to keep runtime power links only, got stored config bytes=%d", len(battery.Build.Config))
	}
}

func TestAutoLinkedPowerNodeCreatesSymmetricRuntimeLinks(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		425: "power-node-large",
		430: "laser-drill",
	}
	w.SetModel(model)

	nodeTile, err := model.TileAt(12, 10)
	if err != nil || nodeTile == nil {
		t.Fatalf("node tile lookup failed: %v", err)
	}
	w.placeTileLocked(nodeTile, 1, 425, 0, nil, 0)
	_ = w.DrainEntityEvents()

	consumerTile, err := model.TileAt(6, 10)
	if err != nil || consumerTile == nil {
		t.Fatalf("consumer tile lookup failed: %v", err)
	}
	w.placeTileLocked(consumerTile, 1, 430, 0, nil, 0)

	nodePos := int32(10*model.Width + 12)
	consumerPos := int32(10*model.Width + 6)
	if links := w.powerNodeLinks[nodePos]; len(links) != 1 || links[0] != consumerPos {
		t.Fatalf("expected autolinked node links [%d], got %v", consumerPos, links)
	}
	if links := w.powerNodeLinks[consumerPos]; len(links) != 1 || links[0] != nodePos {
		t.Fatalf("expected consumer reverse runtime links [%d], got %v", nodePos, links)
	}
	if nodeTile.Build == nil || len(nodeTile.Build.Config) == 0 {
		t.Fatal("expected autolinked power-node stored config to be written")
	}
}

func TestBuildingConfigPackedReturnsDetachedPointSlice(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		422: "power-node",
		421: "battery",
	}
	w.SetModel(model)
	node := placeTestBuilding(t, w, 8, 10, 422, 1, 0)
	placeTestBuilding(t, w, 14, 10, 421, 1, 0)
	nodePos := int32(node.Y*w.Model().Width + node.X)
	nodePacked := protocol.PackPoint2(8, 10)
	w.applyBuildingConfigLocked(nodePos, []protocol.Point2{{X: 6, Y: 0}}, true)

	value, ok := w.BuildingConfigPacked(nodePacked)
	if !ok {
		t.Fatal("expected detached config value")
	}
	points, ok := value.([]protocol.Point2)
	if !ok || len(points) != 1 {
		t.Fatalf("expected detached []Point2 config, got %T %#v", value, value)
	}
	points[0] = protocol.Point2{X: 99, Y: 99}

	value, ok = w.BuildingConfigPacked(nodePacked)
	if !ok {
		t.Fatal("expected detached config value on second read")
	}
	points, ok = value.([]protocol.Point2)
	if !ok || len(points) != 1 {
		t.Fatalf("expected detached []Point2 config on second read, got %T %#v", value, value)
	}
	if points[0].X != 6 || points[0].Y != 0 {
		t.Fatalf("expected world config to stay unchanged after caller mutation, got %+v", points[0])
	}
}

func TestRelatedBlockSyncPackedPositionsIncludeLinkedPowerBuildings(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		422: "power-node",
		101: "air-factory",
	}
	w.SetModel(model)
	node := placeTestBuilding(t, w, 8, 10, 422, 1, 0)
	placeTestBuilding(t, w, 14, 10, 101, 1, 1)
	nodePos := int32(node.Y*w.Model().Width + node.X)
	factoryPacked := protocol.PackPoint2(14, 10)
	nodePacked := protocol.PackPoint2(8, 10)
	w.applyBuildingConfigLocked(nodePos, []protocol.Point2{{X: 6, Y: 0}}, true)
	w.rebuildActiveTilesLocked()

	related := w.RelatedBlockSyncPackedPositions(nodePacked)
	if len(related) != 2 {
		t.Fatalf("expected node+factory related sync positions, got %v", related)
	}
	if related[0] != nodePacked || related[1] != factoryPacked {
		t.Fatalf("expected sorted related packed positions [%d %d], got %v", nodePacked, factoryPacked, related)
	}

	reverse := w.RelatedBlockSyncPackedPositions(factoryPacked)
	if len(reverse) != 2 {
		t.Fatalf("expected factory related sync positions to include node+factory, got %v", reverse)
	}
	if reverse[0] != nodePacked || reverse[1] != factoryPacked {
		t.Fatalf("expected sorted reverse related packed positions [%d %d], got %v", nodePacked, factoryPacked, reverse)
	}
}

func TestDestroyingLinkedPowerBuildingClearsBothSides(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		421: "battery",
		422: "power-node",
	}
	w.SetModel(model)

	node := placeTestBuilding(t, w, 8, 10, 422, 1, 0)
	battery := placeTestBuilding(t, w, 14, 10, 421, 1, 0)
	nodePos := int32(node.Y*w.Model().Width + node.X)
	batteryPos := int32(battery.Y*w.Model().Width + battery.X)
	w.applyBuildingConfigLocked(nodePos, []protocol.Point2{{X: 6, Y: 0}}, true)

	w.destroyTileLocked(battery, 1, 0)

	if links := w.powerNodeLinks[nodePos]; len(links) != 0 {
		t.Fatalf("expected destroying linked battery to clear node links, got %v", links)
	}
	if links := w.powerNodeLinks[batteryPos]; len(links) != 0 {
		t.Fatalf("expected destroying battery to clear reverse runtime links, got %v", links)
	}
	if _, ok := w.BuildingConfigPacked(protocol.PackPoint2(int32(node.X), int32(node.Y))); ok {
		t.Fatal("expected node config view to clear after linked battery is destroyed")
	}
	if node.Build != nil && len(node.Build.Config) != 0 {
		t.Fatalf("expected node stored config bytes to clear after destroy, got=%d", len(node.Build.Config))
	}
}

func TestDestroyTileRemovesPowerIndexes(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		422: "power-node",
		480: "diode",
		481: "power-void",
	}
	w.SetModel(model)

	node := placeTestBuilding(t, w, 6, 10, 422, 1, 0)
	diode := placeTestBuilding(t, w, 10, 10, 480, 1, 0)
	voidTile := placeTestBuilding(t, w, 14, 10, 481, 1, 0)

	w.destroyTileLocked(node, 1, 0)
	if got := len(w.powerTilePositions); got != 2 {
		t.Fatalf("expected power tile count 2 after removing node, got %d", got)
	}
	if got := len(w.teamPowerTiles[1]); got != 2 {
		t.Fatalf("expected team power tile count 2 after removing node, got %d", got)
	}
	if got := len(w.teamPowerNodeTiles[1]); got != 0 {
		t.Fatalf("expected no team power nodes after removing node, got %d", got)
	}

	w.destroyTileLocked(diode, 1, 0)
	if got := len(w.powerTilePositions); got != 1 {
		t.Fatalf("expected power tile count 1 after removing diode, got %d", got)
	}
	if got := len(w.powerDiodeTilePositions); got != 0 {
		t.Fatalf("expected no power diodes after removal, got %d", got)
	}

	w.destroyTileLocked(voidTile, 1, 0)
	if got := len(w.powerTilePositions); got != 0 {
		t.Fatalf("expected all power tiles removed, got %d", got)
	}
	if got := len(w.powerVoidTilePositions); got != 0 {
		t.Fatalf("expected no power void tiles after removal, got %d", got)
	}
	if got := len(w.teamPowerTiles[1]); got != 0 {
		t.Fatalf("expected team power tile list cleared after removals, got %d", got)
	}
}

func TestPlaceCompletedBuildingReplacesOldOccupancyAndTeamIndexes(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(32, 32)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		341: "core-nucleus",
	}
	w.SetModel(model)

	tile := placeTestBuilding(t, w, 10, 10, 341, 1, 0)
	pos := int32(tile.Y*model.Width + tile.X)
	oldFootprint := packTilePos(8, 10)
	if got := w.blockOccupancy[oldFootprint]; got != pos {
		t.Fatalf("expected large-building occupancy at footprint cell to point to %d, got %d", pos, got)
	}

	w.placeCompletedBuildingLocked(pos, tile, 2, 257, 1, nil)

	if got := len(w.activeTilePositions); got != 1 {
		t.Fatalf("expected exactly one active tile after replacement, got %d", got)
	}
	if got := len(w.teamBuildingTiles[1]); got != 0 {
		t.Fatalf("expected old team building index cleared, got %d", got)
	}
	if got := len(w.teamCoreTiles[1]); got != 0 {
		t.Fatalf("expected old team core index cleared, got %d", got)
	}
	if got := len(w.teamBuildingTiles[2]); got != 1 {
		t.Fatalf("expected replacement to be indexed for new team, got %d", got)
	}
	if _, ok := w.blockOccupancy[oldFootprint]; ok {
		t.Fatalf("expected old large-building footprint occupancy cleared at %d", oldFootprint)
	}
	if got := w.blockOccupancy[packTilePos(10, 10)]; got != pos {
		t.Fatalf("expected new 1x1 occupancy to point to replacement center %d, got %d", pos, got)
	}
	if _, ok := w.buildingIndexFromPackedPosLocked(oldFootprint); ok {
		t.Fatalf("expected no building index at old footprint cell %d after shrinking replacement", oldFootprint)
	}
}

func TestPlaceCompletedBuildingReplacingPowerNodeClearsPowerLinksAndIndexes(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		421: "battery",
		422: "power-node",
	}
	w.SetModel(model)

	node := placeTestBuilding(t, w, 8, 10, 422, 1, 0)
	battery := placeTestBuilding(t, w, 14, 10, 421, 1, 0)
	nodePos := int32(node.Y*model.Width + node.X)
	batteryPos := int32(battery.Y*model.Width + battery.X)
	w.applyBuildingConfigLocked(nodePos, []protocol.Point2{{X: 6, Y: 0}}, true)

	w.placeCompletedBuildingLocked(nodePos, node, 1, 257, 0, nil)

	if links := w.powerNodeLinks[nodePos]; len(links) != 0 {
		t.Fatalf("expected replaced power node links cleared, got %v", links)
	}
	if links := w.powerNodeLinks[batteryPos]; len(links) != 0 {
		t.Fatalf("expected reverse power links cleared from battery, got %v", links)
	}
	if got := len(w.teamPowerNodeTiles[1]); got != 0 {
		t.Fatalf("expected no team power nodes after replacement, got %d", got)
	}
	if got := len(w.teamPowerTiles[1]); got != 1 {
		t.Fatalf("expected only the battery to stay in team power tiles, got %d", got)
	}
	for _, pos := range w.powerTilePositions {
		if pos == nodePos {
			t.Fatalf("expected replaced power node %d to be removed from power tile index", nodePos)
		}
	}
}

func TestBlockSyncSnapshotsEncodeBeamNodeBufferedPowerStatus(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		474: "beam-node",
	}
	w.SetModel(model)
	tile := placeTestBuilding(t, w, 9, 9, 474, 1, 0)
	pos := int32(tile.Y*w.Model().Width + tile.X)
	w.powerStorageState[pos] = 500
	w.rebuildActiveTilesLocked()

	snaps := w.BlockSyncSnapshots()
	if len(snaps) != 1 {
		t.Fatalf("expected one beam-node block sync snapshot, got %d", len(snaps))
	}
	base, r := decodeBlockSyncBase(t, snaps[0].Data)
	if math.Abs(float64(base.PowerStatus-0.5)) > 0.0001 {
		t.Fatalf("expected beam-node buffered power status 0.5, got %f", base.PowerStatus)
	}
	if rem := r.Remaining(); rem != 0 {
		t.Fatalf("expected beam-node sync payload to be fully consumed, remaining=%d", rem)
	}
}

func TestBlockSyncSnapshotsEncodeSolarPanelProductionEfficiency(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		404: "solar-panel",
	}
	w.SetModel(model)
	placeTestBuilding(t, w, 9, 9, 404, 1, 0)
	w.rebuildActiveTilesLocked()

	snaps := w.BlockSyncSnapshots()
	if len(snaps) != 1 {
		t.Fatalf("expected one solar-panel block sync snapshot, got %d", len(snaps))
	}
	base, r := decodeBlockSyncBase(t, snaps[0].Data)
	productionEfficiency, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read solar-panel production efficiency failed: %v", err)
	}
	generateTime, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read solar-panel generateTime failed: %v", err)
	}
	if base.PowerStatus != 0 {
		t.Fatalf("expected solar-panel power status 0 for non-consumer producer, got %f", base.PowerStatus)
	}
	if math.Abs(float64(productionEfficiency-1)) > 0.0001 {
		t.Fatalf("expected solar-panel production efficiency 1, got %f", productionEfficiency)
	}
	if math.Abs(float64(generateTime)) > 0.0001 {
		t.Fatalf("expected solar-panel generateTime 0, got %f", generateTime)
	}
	if rem := r.Remaining(); rem != 0 {
		t.Fatalf("expected solar-panel sync payload to be fully consumed, remaining=%d", rem)
	}
}

func TestBlockSyncSnapshotsEncodeUnloaderConfig(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		270: "unloader",
	}
	w.SetModel(model)
	tile := placeTestBuilding(t, w, 6, 7, 270, 1, 0)
	pos := int32(tile.Y*w.Model().Width + tile.X)
	w.unloaderCfg[pos] = siliconItemID
	w.rebuildActiveTilesLocked()

	snaps := w.BlockSyncSnapshots()
	if len(snaps) != 1 {
		t.Fatalf("expected one block sync snapshot, got %d", len(snaps))
	}
	base, r := decodeBlockSyncBase(t, snaps[0].Data)
	if (base.ModuleBits & 1) == 0 {
		t.Fatalf("expected unloader item module bit to be present, bits=%08b", base.ModuleBits)
	}
	itemID, err := r.ReadInt16()
	if err != nil {
		t.Fatalf("read unloader sort item failed: %v", err)
	}
	if itemID != int16(siliconItemID) {
		t.Fatalf("expected unloader sort item %d, got %d", siliconItemID, itemID)
	}
	if rem := r.Remaining(); rem != 0 {
		t.Fatalf("expected unloader sync payload to be fully consumed, remaining=%d", rem)
	}
}

func TestBlockSyncSnapshotsEncodeConveyorRuntime(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		257: "conveyor",
	}
	w.SetModel(model)
	tile := placeTestBuilding(t, w, 6, 6, 257, 1, 0)
	pos := int32(tile.Y*w.Model().Width + tile.X)
	tile.Build.Items = []ItemStack{
		{Item: copperItemID, Amount: 1},
		{Item: siliconItemID, Amount: 1},
	}
	w.conveyorStates[pos] = &conveyorRuntimeState{
		IDs:          [3]ItemID{copperItemID, siliconItemID},
		XS:           [3]float32{1, -1},
		YS:           [3]float32{0.25, 0.75},
		Len:          2,
		LastInserted: 1,
		Mid:          1,
		MinItem:      0.25,
	}
	w.rebuildActiveTilesLocked()

	snaps := w.BlockSyncSnapshots()
	if len(snaps) != 1 {
		t.Fatalf("expected one conveyor block sync snapshot, got %d", len(snaps))
	}
	base, r := decodeBlockSyncBase(t, snaps[0].Data)
	if got := base.Items[copperItemID]; got != 1 {
		t.Fatalf("expected conveyor copper amount 1, got %d", got)
	}
	if got := base.Items[siliconItemID]; got != 1 {
		t.Fatalf("expected conveyor silicon amount 1, got %d", got)
	}
	amount, err := r.ReadInt32()
	if err != nil {
		t.Fatalf("read conveyor item count failed: %v", err)
	}
	if amount != 2 {
		t.Fatalf("expected 2 conveyor runtime items, got %d", amount)
	}
	firstItem, err := r.ReadInt16()
	if err != nil {
		t.Fatalf("read first conveyor item failed: %v", err)
	}
	firstX, err := r.ReadByte()
	if err != nil {
		t.Fatalf("read first conveyor x failed: %v", err)
	}
	firstY, err := r.ReadByte()
	if err != nil {
		t.Fatalf("read first conveyor y failed: %v", err)
	}
	if firstItem != int16(copperItemID) || firstX != signedByteFromInt(127) || firstY != signedByteFromInt(-64) {
		t.Fatalf("unexpected first conveyor runtime entry item=%d x=%d y=%d", firstItem, firstX, firstY)
	}
	secondItem, err := r.ReadInt16()
	if err != nil {
		t.Fatalf("read second conveyor item failed: %v", err)
	}
	secondX, err := r.ReadByte()
	if err != nil {
		t.Fatalf("read second conveyor x failed: %v", err)
	}
	secondY, err := r.ReadByte()
	if err != nil {
		t.Fatalf("read second conveyor y failed: %v", err)
	}
	if secondItem != int16(siliconItemID) || secondX != signedByteFromInt(-127) || secondY != signedByteFromInt(63) {
		t.Fatalf("unexpected second conveyor runtime entry item=%d x=%d y=%d", secondItem, secondX, secondY)
	}
	if rem := r.Remaining(); rem != 0 {
		t.Fatalf("expected conveyor sync payload to be fully consumed, remaining=%d", rem)
	}
}

func TestBlockSyncSnapshotsLiveOnlyIncludeConveyorRuntime(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		257: "conveyor",
	}
	w.SetModel(model)
	tile := placeTestBuilding(t, w, 6, 6, 257, 1, 0)
	pos := int32(tile.Y*w.Model().Width + tile.X)
	tile.Build.Items = []ItemStack{{Item: copperItemID, Amount: 1}}
	w.conveyorStates[pos] = &conveyorRuntimeState{
		IDs: [3]ItemID{copperItemID},
		XS:  [3]float32{1},
		YS:  [3]float32{0.5},
		Len: 1,
	}
	w.rebuildActiveTilesLocked()

	snaps := w.BlockSyncSnapshotsLiveOnly()
	if len(snaps) != 1 {
		t.Fatalf("expected one live-only conveyor block sync snapshot, got %d", len(snaps))
	}
	if snaps[0].Pos != protocol.PackPoint2(6, 6) {
		t.Fatalf("expected live-only conveyor snapshot at %d, got %d", protocol.PackPoint2(6, 6), snaps[0].Pos)
	}
	base, r := decodeBlockSyncBase(t, snaps[0].Data)
	if got := base.Items[copperItemID]; got != 1 {
		t.Fatalf("expected live-only conveyor copper amount 1, got %d", got)
	}
	amount, err := r.ReadInt32()
	if err != nil {
		t.Fatalf("read live-only conveyor item count failed: %v", err)
	}
	if amount != 1 {
		t.Fatalf("expected 1 live-only conveyor runtime item, got %d", amount)
	}
}

func TestBlockSyncSnapshotsEncodeRouterRuntimeViaBaseOnly(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		418: "router",
	}
	w.SetModel(model)
	tile := placeTestBuilding(t, w, 9, 6, 418, 1, 0)
	tile.Build.Items = []ItemStack{{Item: copperItemID, Amount: 1}}
	w.rebuildActiveTilesLocked()

	snaps := w.BlockSyncSnapshots()
	if len(snaps) != 1 {
		t.Fatalf("expected one router block sync snapshot, got %d", len(snaps))
	}
	base, r := decodeBlockSyncBase(t, snaps[0].Data)
	if got := base.Items[copperItemID]; got != 1 {
		t.Fatalf("expected router copper amount 1, got %d", got)
	}
	if rem := r.Remaining(); rem != 0 {
		t.Fatalf("expected router base-only sync payload to be fully consumed, remaining=%d", rem)
	}
}

func TestBlockSyncSnapshotsEncodeStackConveyorRuntime(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		447: "plastanium-conveyor",
	}
	w.SetModel(model)
	tile := placeTestBuilding(t, w, 8, 8, 447, 1, 1)
	target := placeTestBuilding(t, w, 9, 8, 447, 1, 1)
	pos := int32(tile.Y*w.Model().Width + tile.X)
	targetPos := int32(target.Y*w.Model().Width + target.X)
	tile.Build.Items = []ItemStack{{Item: copperItemID, Amount: 3}}
	w.stackStates[pos] = &stackRuntimeState{
		Link:      targetPos,
		Cooldown:  0.35,
		LastItem:  copperItemID,
		HasItem:   true,
		Unloading: false,
	}
	w.rebuildActiveTilesLocked()

	snaps := w.BlockSyncSnapshots()
	var snap *BlockSyncSnapshot
	for i := range snaps {
		if snaps[i].Pos == protocol.PackPoint2(8, 8) {
			snap = &snaps[i]
			break
		}
	}
	if snap == nil {
		t.Fatal("expected plastanium conveyor snapshot")
	}
	base, r := decodeBlockSyncBase(t, snap.Data)
	if got := base.Items[copperItemID]; got != 3 {
		t.Fatalf("expected plastanium conveyor copper amount 3, got %d", got)
	}
	link, err := r.ReadInt32()
	if err != nil {
		t.Fatalf("read plastanium conveyor link failed: %v", err)
	}
	cooldown, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read plastanium conveyor cooldown failed: %v", err)
	}
	if link != protocol.PackPoint2(9, 8) {
		t.Fatalf("expected plastanium conveyor link %d, got %d", protocol.PackPoint2(9, 8), link)
	}
	if math.Abs(float64(cooldown-0.35)) > 0.0001 {
		t.Fatalf("expected plastanium conveyor cooldown 0.35, got %f", cooldown)
	}
	if rem := r.Remaining(); rem != 0 {
		t.Fatalf("expected plastanium conveyor sync payload to be fully consumed, remaining=%d", rem)
	}
}

func TestBlockSyncSnapshotsEncodeMassDriverRuntime(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		432: "mass-driver",
	}
	w.SetModel(model)
	src := placeTestBuilding(t, w, 6, 10, 432, 1, 0)
	dst := placeTestBuilding(t, w, 12, 10, 432, 1, 0)
	pos := int32(src.Y*w.Model().Width + src.X)
	targetPos := int32(dst.Y*w.Model().Width + dst.X)
	src.Build.Items = []ItemStack{{Item: copperItemID, Amount: 20}}
	w.massDriverLinks[pos] = targetPos
	w.rebuildActiveTilesLocked()

	snaps := w.BlockSyncSnapshots()
	var snap *BlockSyncSnapshot
	for i := range snaps {
		if snaps[i].Pos == protocol.PackPoint2(6, 10) {
			snap = &snaps[i]
			break
		}
	}
	if snap == nil {
		t.Fatal("expected mass-driver snapshot")
	}
	base, r := decodeBlockSyncBase(t, snap.Data)
	if got := base.Items[copperItemID]; got != 20 {
		t.Fatalf("expected mass-driver copper amount 20, got %d", got)
	}
	link, err := r.ReadInt32()
	if err != nil {
		t.Fatalf("read mass-driver link failed: %v", err)
	}
	rotation, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read mass-driver rotation failed: %v", err)
	}
	state, err := r.ReadByte()
	if err != nil {
		t.Fatalf("read mass-driver state failed: %v", err)
	}
	if link != protocol.PackPoint2(12, 10) {
		t.Fatalf("expected mass-driver link %d, got %d", protocol.PackPoint2(12, 10), link)
	}
	wantRotation := lookAt(float32(src.X*8+4), float32(src.Y*8+4), float32(dst.X*8+4), float32(dst.Y*8+4))
	if math.Abs(float64(rotation-wantRotation)) > 0.0001 {
		t.Fatalf("expected mass-driver rotation %f, got %f", wantRotation, rotation)
	}
	if state != 2 {
		t.Fatalf("expected linked mass-driver to sync shooting state=2, got %d", state)
	}
	if rem := r.Remaining(); rem != 0 {
		t.Fatalf("expected mass-driver sync payload to be fully consumed, remaining=%d", rem)
	}
}

func TestBlockSyncSnapshotsEncodeLinkedStorageSharedInventory(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		339: "core-shard",
		431: "vault",
	}
	w.SetModel(model)
	placeTestBuilding(t, w, 5, 5, 339, 1, 0)
	store := placeTestBuilding(t, w, 8, 5, 431, 1, 0)
	store.Build.AddItem(copperItemID, 4)
	w.rebuildBlockOccupancyLocked()

	snaps := w.BlockSyncSnapshots()
	if len(snaps) != 1 {
		t.Fatalf("expected one storage block sync snapshot, got %d", len(snaps))
	}
	if snaps[0].Pos != protocol.PackPoint2(8, 5) {
		t.Fatalf("unexpected linked storage snapshot pos=%d", snaps[0].Pos)
	}
	base, r := decodeBlockSyncBase(t, snaps[0].Data)
	if got := base.Items[copperItemID]; got != 4 {
		t.Fatalf("expected linked storage sync to expose shared core copper=4, got %d", got)
	}
	if rem := r.Remaining(); rem != 0 {
		t.Fatalf("expected linked storage sync payload to be fully consumed, remaining=%d", rem)
	}
}

func TestBlockSyncSnapshotsEncodeStandaloneStorageMultipleItems(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		500: "container",
	}
	w.SetModel(model)
	tile := placeTestBuilding(t, w, 7, 6, 500, 1, 0)
	tile.Build.Items = []ItemStack{
		{Item: copperItemID, Amount: 3},
		{Item: leadItemID, Amount: 5},
		{Item: coalItemID, Amount: 2},
		{Item: sandItemID, Amount: 4},
	}

	snaps := w.BlockSyncSnapshots()
	if len(snaps) != 1 {
		t.Fatalf("expected one storage snapshot, got %d", len(snaps))
	}
	base, r := decodeBlockSyncBase(t, snaps[0].Data)
	if got := base.Items[copperItemID]; got != 3 {
		t.Fatalf("expected copper=3, got %d", got)
	}
	if got := base.Items[leadItemID]; got != 5 {
		t.Fatalf("expected lead=5, got %d", got)
	}
	if got := base.Items[coalItemID]; got != 2 {
		t.Fatalf("expected coal=2, got %d", got)
	}
	if got := base.Items[sandItemID]; got != 4 {
		t.Fatalf("expected sand=4, got %d", got)
	}
	if rem := r.Remaining(); rem != 0 {
		t.Fatalf("expected standalone storage sync payload to be fully consumed, remaining=%d", rem)
	}
}

func TestBlockSyncSnapshotsSkipPendingBreakTurret(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.BlockNames = map[int16]string{
		910: "duo",
	}
	w.SetModel(model)
	w.buildingProfilesByName["duo"] = buildingWeaponProfile{
		ClassName:    "ItemTurret",
		Range:        136,
		Damage:       9,
		Interval:     0.6,
		AmmoCapacity: 80,
		AmmoPerShot:  1,
		TargetAir:    true,
		TargetGround: true,
		HitBuildings: true,
	}
	tile := placeTestBuilding(t, w, 5, 6, 910, 1, 0)
	pos := int32(tile.Y*w.Model().Width + tile.X)
	w.pendingBreaks[pos] = pendingBreakState{
		Team:    1,
		BlockID: 910,
	}

	if snaps := w.ItemTurretBlockSyncSnapshotsForPackedLiveOnly([]int32{packTilePos(tile.X, tile.Y)}); len(snaps) != 0 {
		t.Fatalf("expected pending-break item-turret snapshots to be suppressed, got %d", len(snaps))
	}
	if snaps := w.TurretBlockSyncSnapshotsLiveOnly(); len(snaps) != 0 {
		t.Fatalf("expected pending-break turret periodic snapshots to be suppressed, got %d", len(snaps))
	}
	if builds := w.BuildSyncSnapshot(); len(builds) != 0 {
		t.Fatalf("expected pending-break turret to be absent from build sync snapshot, got %d", len(builds))
	}
}

func TestBlockSyncSnapshotsEncodeUnitFactoryRuntime(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		100: "ground-factory",
	}
	model.UnitNames = map[int16]string{
		7: "dagger",
		8: "crawler",
	}
	w.SetModel(model)
	tile := placeTestBuilding(t, w, 10, 10, 100, 1, 1)
	pos := int32(tile.Y*w.Model().Width + tile.X)
	tile.Build.Items = []ItemStack{
		{Item: siliconItemID, Amount: 8},
		{Item: coalItemID, Amount: 10},
	}
	w.factoryStates[pos] = factoryState{
		Progress:    123.5,
		UnitType:    8,
		CurrentPlan: 1,
	}
	w.rebuildActiveTilesLocked()

	snaps := w.BlockSyncSnapshots()
	if len(snaps) != 1 {
		t.Fatalf("expected one unit factory block sync snapshot, got %d", len(snaps))
	}
	base, r := decodeBlockSyncBase(t, snaps[0].Data)
	if got := base.Items[siliconItemID]; got != 8 {
		t.Fatalf("expected silicon amount 8, got %d", got)
	}
	if got := base.Items[coalItemID]; got != 10 {
		t.Fatalf("expected coal amount 10, got %d", got)
	}
	payX, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read unit factory payVector.x failed: %v", err)
	}
	payY, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read unit factory payVector.y failed: %v", err)
	}
	payRotation, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read unit factory payRotation failed: %v", err)
	}
	payloadExists, err := r.ReadBool()
	if err != nil {
		t.Fatalf("read unit factory payload exists flag failed: %v", err)
	}
	progress, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read unit factory progress failed: %v", err)
	}
	currentPlan, err := r.ReadInt16()
	if err != nil {
		t.Fatalf("read unit factory current plan failed: %v", err)
	}
	commandPos, err := protocol.ReadVecNullable(r)
	if err != nil {
		t.Fatalf("read unit factory commandPos failed: %v", err)
	}
	command, err := protocol.ReadCommand(r, nil)
	if err != nil {
		t.Fatalf("read unit factory command failed: %v", err)
	}
	if math.Abs(float64(payX)) > 0.0001 || math.Abs(float64(payY)) > 0.0001 {
		t.Fatalf("expected idle unit factory payload offset to stay at origin, got x=%f y=%f", payX, payY)
	}
	if math.Abs(float64(payRotation-90)) > 0.0001 {
		t.Fatalf("expected unit factory payload rotation 90, got %f", payRotation)
	}
	if payloadExists {
		t.Fatalf("expected empty unit factory payload")
	}
	if math.Abs(float64(progress-123.5)) > 0.0001 {
		t.Fatalf("expected progress 123.5, got %f", progress)
	}
	if currentPlan != 1 {
		t.Fatalf("expected current plan 1, got %d", currentPlan)
	}
	if commandPos != nil {
		t.Fatalf("expected nil commandPos, got %+v", commandPos)
	}
	if command != nil {
		t.Fatalf("expected nil command, got %+v", command)
	}
	if rem := r.Remaining(); rem != 0 {
		t.Fatalf("expected unit factory sync payload to be fully consumed, remaining=%d", rem)
	}
}

func TestBlockSyncSnapshotsEncodeUnitFactoryCommandState(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		100: "ground-factory",
	}
	model.UnitNames = map[int16]string{
		7: "dagger",
	}
	w.SetModel(model)
	tile := placeTestBuilding(t, w, 5, 5, 100, 1, 0)
	pos := int32(tile.Y*w.Model().Width + tile.X)
	wantPos := &protocol.Vec2{X: 96, Y: 112}
	wantCommand := &protocol.UnitCommand{ID: 2, Name: "repair"}
	w.factoryStates[pos] = factoryState{
		Progress:    60,
		UnitType:    7,
		CurrentPlan: 0,
		CommandPos:  wantPos,
		Command:     wantCommand,
	}
	w.rebuildActiveTilesLocked()

	snaps := w.BlockSyncSnapshots()
	if len(snaps) != 1 {
		t.Fatalf("expected one unit factory block sync snapshot, got %d", len(snaps))
	}
	_, r := decodeBlockSyncBase(t, snaps[0].Data)
	if _, err := r.ReadFloat32(); err != nil {
		t.Fatalf("read unit factory payVector.x failed: %v", err)
	}
	if _, err := r.ReadFloat32(); err != nil {
		t.Fatalf("read unit factory payVector.y failed: %v", err)
	}
	if _, err := r.ReadFloat32(); err != nil {
		t.Fatalf("read unit factory payRotation failed: %v", err)
	}
	if _, err := r.ReadBool(); err != nil {
		t.Fatalf("read unit factory payload exists failed: %v", err)
	}
	if _, err := r.ReadFloat32(); err != nil {
		t.Fatalf("read unit factory progress failed: %v", err)
	}
	if _, err := r.ReadInt16(); err != nil {
		t.Fatalf("read unit factory current plan failed: %v", err)
	}
	commandPos, err := protocol.ReadVecNullable(r)
	if err != nil {
		t.Fatalf("read unit factory commandPos failed: %v", err)
	}
	command, err := protocol.ReadCommand(r, nil)
	if err != nil {
		t.Fatalf("read unit factory command failed: %v", err)
	}
	if commandPos == nil || math.Abs(float64(commandPos.X-wantPos.X)) > 0.0001 || math.Abs(float64(commandPos.Y-wantPos.Y)) > 0.0001 {
		t.Fatalf("expected unit factory commandPos %+v, got %+v", wantPos, commandPos)
	}
	if command == nil || command.ID != wantCommand.ID {
		t.Fatalf("expected unit factory command id %d, got %+v", wantCommand.ID, command)
	}
}

func TestBlockSyncSnapshotsEncodeDrillRuntime(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		430: "laser-drill",
	}
	w.SetModel(model)
	tile := placeTestBuilding(t, w, 9, 9, 430, 1, 0)
	pos := int32(tile.Y*w.Model().Width + tile.X)
	w.drillStates[pos] = drillRuntimeState{
		Progress: 91.5,
		Warmup:   0.75,
	}
	w.rebuildActiveTilesLocked()

	snaps := w.BlockSyncSnapshots()
	if len(snaps) != 1 {
		t.Fatalf("expected one drill block sync snapshot, got %d", len(snaps))
	}
	base, r := decodeBlockSyncBase(t, snaps[0].Data)
	if (base.ModuleBits & (1 << 1)) == 0 {
		t.Fatalf("expected drill power module bit to be present, bits=%08b", base.ModuleBits)
	}
	progress, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read drill progress failed: %v", err)
	}
	warmup, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read drill warmup failed: %v", err)
	}
	if math.Abs(float64(progress-91.5)) > 0.0001 {
		t.Fatalf("expected drill progress 91.5, got %f", progress)
	}
	if math.Abs(float64(warmup-0.75)) > 0.0001 {
		t.Fatalf("expected drill warmup 0.75, got %f", warmup)
	}
	if rem := r.Remaining(); rem != 0 {
		t.Fatalf("expected drill sync payload to be fully consumed, remaining=%d", rem)
	}
}

func TestBlockSyncSnapshotsEncodePumpRuntime(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		441: "rotary-pump",
	}
	w.SetModel(model)
	tile := placeTestBuilding(t, w, 11, 7, 441, 1, 0)
	pos := int32(tile.Y*w.Model().Width + tile.X)
	tile.Build.AddLiquid(waterLiquidID, 12.5)
	w.pumpStates[pos] = pumpRuntimeState{
		Warmup:   0.65,
		Progress: 17,
	}
	w.rebuildActiveTilesLocked()

	snaps := w.BlockSyncSnapshots()
	if len(snaps) != 1 {
		t.Fatalf("expected one pump block sync snapshot, got %d", len(snaps))
	}
	base, r := decodeBlockSyncBase(t, snaps[0].Data)
	if (base.ModuleBits & (1 << 1)) == 0 {
		t.Fatalf("expected pump power module bit to be present, bits=%08b", base.ModuleBits)
	}
	if (base.ModuleBits & (1 << 2)) == 0 {
		t.Fatalf("expected pump liquid module bit to be present, bits=%08b", base.ModuleBits)
	}
	if got := base.Liquids[waterLiquidID]; math.Abs(float64(got-12.5)) > 0.0001 {
		t.Fatalf("expected pump liquid amount 12.5, got %f", got)
	}
	if base.Efficiency == 0 {
		t.Fatalf("expected pump efficiency byte to reflect warmup, got 0")
	}
	if rem := r.Remaining(); rem != 0 {
		t.Fatalf("expected pump sync payload to be fully consumed, remaining=%d", rem)
	}
}

func TestBlockSyncSnapshotsEncodeHeatProducerRuntime(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		461: "electric-heater",
	}
	w.SetModel(model)
	tile := placeTestBuilding(t, w, 8, 8, 461, 1, 0)
	pos := int32(tile.Y*w.Model().Width + tile.X)
	w.crafterStates[pos] = crafterRuntimeState{
		Progress: 33.25,
		Warmup:   0.5,
	}
	w.heatStates[pos] = 2.25
	w.rebuildActiveTilesLocked()

	snaps := w.BlockSyncSnapshots()
	if len(snaps) != 1 {
		t.Fatalf("expected one heat producer block sync snapshot, got %d", len(snaps))
	}
	base, r := decodeBlockSyncBase(t, snaps[0].Data)
	if (base.ModuleBits & (1 << 1)) == 0 {
		t.Fatalf("expected heat producer power module bit to be present, bits=%08b", base.ModuleBits)
	}
	progress, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read heat producer progress failed: %v", err)
	}
	warmup, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read heat producer warmup failed: %v", err)
	}
	heat, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read heat producer heat failed: %v", err)
	}
	if math.Abs(float64(progress-33.25)) > 0.0001 {
		t.Fatalf("expected heat producer progress 33.25, got %f", progress)
	}
	if math.Abs(float64(warmup-0.5)) > 0.0001 {
		t.Fatalf("expected heat producer warmup 0.5, got %f", warmup)
	}
	if math.Abs(float64(heat-2.25)) > 0.0001 {
		t.Fatalf("expected heat producer heat 2.25, got %f", heat)
	}
	if rem := r.Remaining(); rem != 0 {
		t.Fatalf("expected heat producer sync payload to be fully consumed, remaining=%d", rem)
	}
}

func TestBlockSyncSnapshotsEncodeVariableReactorRuntime(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		473: "flux-reactor",
	}
	w.SetModel(model)
	tile := placeTestBuilding(t, w, 9, 9, 473, 1, 0)
	pos := int32(tile.Y*w.Model().Width + tile.X)
	tile.Build.AddLiquid(cyanogenLiquidID, 30)
	w.heatStates[pos] = 75
	w.powerGeneratorState[pos] = &powerGeneratorState{
		Warmup:      0.6,
		Instability: 0.2,
	}
	w.rebuildActiveTilesLocked()

	snaps := w.BlockSyncSnapshots()
	if len(snaps) != 1 {
		t.Fatalf("expected one variable reactor block sync snapshot, got %d", len(snaps))
	}
	base, r := decodeBlockSyncBase(t, snaps[0].Data)
	if (base.ModuleBits & (1 << 1)) == 0 {
		t.Fatalf("expected variable reactor power module bit to be present, bits=%08b", base.ModuleBits)
	}
	if (base.ModuleBits & (1 << 2)) == 0 {
		t.Fatalf("expected variable reactor liquid module bit to be present, bits=%08b", base.ModuleBits)
	}
	productionEfficiency, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read variable reactor production efficiency failed: %v", err)
	}
	generateTime, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read variable reactor generateTime failed: %v", err)
	}
	heat, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read variable reactor heat failed: %v", err)
	}
	instability, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read variable reactor instability failed: %v", err)
	}
	warmup, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read variable reactor warmup failed: %v", err)
	}
	if math.Abs(float64(productionEfficiency-0.5)) > 0.0001 {
		t.Fatalf("expected variable reactor production efficiency 0.5, got %f", productionEfficiency)
	}
	if math.Abs(float64(generateTime)) > 0.0001 {
		t.Fatalf("expected variable reactor generateTime 0, got %f", generateTime)
	}
	if math.Abs(float64(heat-75)) > 0.0001 {
		t.Fatalf("expected variable reactor heat 75, got %f", heat)
	}
	if math.Abs(float64(instability-0.2)) > 0.0001 {
		t.Fatalf("expected variable reactor instability 0.2, got %f", instability)
	}
	if math.Abs(float64(warmup-0.6)) > 0.0001 {
		t.Fatalf("expected variable reactor warmup 0.6, got %f", warmup)
	}
	if rem := r.Remaining(); rem != 0 {
		t.Fatalf("expected variable reactor sync payload to be fully consumed, remaining=%d", rem)
	}
}

func TestBlockSyncPowerStatusTracksActualSupplyRatioPerBuilding(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		404: "solar-panel",
		422: "power-node",
		430: "laser-drill",
	}
	w.SetModel(model)
	placeTestBuilding(t, w, 4, 8, 404, 1, 0)
	placeTestBuilding(t, w, 8, 8, 430, 1, 0)
	placeTestBuilding(t, w, 6, 8, 422, 1, 0)
	w.Step(time.Second)

	snaps := w.BlockSyncSnapshots()
	var drillSnap *BlockSyncSnapshot
	for i := range snaps {
		if snaps[i].Pos == protocol.PackPoint2(8, 8) {
			drillSnap = &snaps[i]
			break
		}
	}
	if drillSnap == nil {
		t.Fatal("expected laser drill block sync snapshot")
	}
	base, _ := decodeBlockSyncBase(t, drillSnap.Data)
	if base.PowerStatus != 0 {
		t.Fatalf("expected underpowered laser drill to sync power status 0, got %f", base.PowerStatus)
	}
}

func TestBlockSyncUnitFactoryEfficiencyTracksSyncedPowerCoverage(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		100: "ground-factory",
	}
	model.UnitNames = map[int16]string{
		7: "dagger",
	}
	w.SetModel(model)
	tile := placeTestBuilding(t, w, 5, 5, 100, 1, 0)
	pos := int32(tile.Y*w.Model().Width + tile.X)
	tile.Build.AddItem(siliconItemID, 10)
	tile.Build.AddItem(leadItemID, 10)
	w.factoryStates[pos] = factoryState{
		Progress:    60,
		CurrentPlan: 0,
		UnitType:    7,
	}
	w.powerRequested[pos] = 12
	w.powerSupplied[pos] = 6
	w.rebuildActiveTilesLocked()

	snaps := w.BlockSyncSnapshots()
	if len(snaps) != 1 {
		t.Fatalf("expected one unit factory block sync snapshot, got %d", len(snaps))
	}
	base, r := decodeBlockSyncBase(t, snaps[0].Data)
	if math.Abs(float64(base.PowerStatus-0.5)) > 0.0001 {
		t.Fatalf("expected unit factory power status 0.5, got %f", base.PowerStatus)
	}
	if math.Abs(float64(float32(base.Efficiency)/255-0.5)) > 0.02 {
		t.Fatalf("expected unit factory efficiency byte to track power coverage ~=0.5, got raw=%d", base.Efficiency)
	}
	if math.Abs(float64(float32(base.OptionalEfficiency)/255-0.5)) > 0.02 {
		t.Fatalf("expected unit factory optional efficiency byte to track power coverage ~=0.5, got raw=%d", base.OptionalEfficiency)
	}
	_ = r
}

func TestBlockSyncAirFactoryKeepsReversePowerNodeLink(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		101: "air-factory",
		422: "power-node",
	}
	w.SetModel(model)
	factory := placeTestBuilding(t, w, 12, 10, 101, 1, 1)
	node := placeTestBuilding(t, w, 6, 10, 422, 1, 0)
	factoryPos := int32(factory.Y*w.Model().Width + factory.X)
	nodePos := int32(node.Y*w.Model().Width + node.X)
	factory.Build.AddItem(siliconItemID, 15)
	w.factoryStates[factoryPos] = factoryState{
		Progress:    180,
		CurrentPlan: 0,
	}
	w.applyBuildingConfigLocked(nodePos, []protocol.Point2{{X: 6, Y: 0}}, true)
	w.rebuildActiveTilesLocked()

	snaps := w.BlockSyncSnapshots()
	var factorySnap *BlockSyncSnapshot
	for i := range snaps {
		if snaps[i].Pos == protocol.PackPoint2(12, 10) {
			factorySnap = &snaps[i]
			break
		}
	}
	if factorySnap == nil {
		t.Fatal("expected air-factory block sync snapshot")
	}
	base, r := decodeBlockSyncBase(t, factorySnap.Data)
	nodePacked := protocol.PackPoint2(6, 10)
	if len(base.PowerLinks) != 1 || base.PowerLinks[0] != nodePacked {
		t.Fatalf("expected air-factory power module to keep reverse node packed link %d, got %v", nodePacked, base.PowerLinks)
	}
	if _, err := r.ReadFloat32(); err != nil {
		t.Fatalf("read air-factory payVector.x failed: %v", err)
	}
	if _, err := r.ReadFloat32(); err != nil {
		t.Fatalf("read air-factory payVector.y failed: %v", err)
	}
	if _, err := r.ReadFloat32(); err != nil {
		t.Fatalf("read air-factory payRotation failed: %v", err)
	}
	payloadExists, err := r.ReadBool()
	if err != nil {
		t.Fatalf("read air-factory payload flag failed: %v", err)
	}
	if payloadExists {
		t.Fatal("expected air-factory payload to stay empty in this snapshot")
	}
	progress, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read air-factory progress failed: %v", err)
	}
	if math.Abs(float64(progress-180)) > 0.0001 {
		t.Fatalf("expected air-factory progress 180, got %f", progress)
	}
	currentPlan, err := r.ReadInt16()
	if err != nil {
		t.Fatalf("read air-factory current plan failed: %v", err)
	}
	if currentPlan != 0 {
		t.Fatalf("expected air-factory current plan 0, got %d", currentPlan)
	}
	commandPos, err := protocol.ReadVecNullable(r)
	if err != nil {
		t.Fatalf("read air-factory commandPos failed: %v", err)
	}
	if commandPos != nil {
		t.Fatalf("expected nil air-factory commandPos, got %+v", commandPos)
	}
	command, err := protocol.ReadCommand(r, nil)
	if err != nil {
		t.Fatalf("read air-factory command failed: %v", err)
	}
	if command != nil {
		t.Fatalf("expected nil air-factory command, got %+v", command)
	}
	if rem := r.Remaining(); rem != 0 {
		t.Fatalf("expected air-factory sync payload to be fully consumed, remaining=%d", rem)
	}
}

func TestUnitFactoryBlockSyncSnapshotsIncludeOnlyFactories(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		101: "air-factory",
		500: "container",
	}
	w.SetModel(model)
	placeTestBuilding(t, w, 12, 10, 101, 1, 1)
	placeTestBuilding(t, w, 4, 4, 500, 1, 0)
	w.rebuildActiveTilesLocked()

	snaps := w.UnitFactoryBlockSyncSnapshots()
	if len(snaps) != 1 {
		t.Fatalf("expected one unit factory block sync snapshot, got %d", len(snaps))
	}
	if snaps[0].Pos != protocol.PackPoint2(12, 10) {
		t.Fatalf("expected air-factory snapshot at pos=%d, got %d", protocol.PackPoint2(12, 10), snaps[0].Pos)
	}
}

func TestUnitFactoryConfigAndAcceptedInputsMatchCurrentPlan(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		100: "ground-factory",
		418: "router",
	}
	model.UnitNames = map[int16]string{
		7: "dagger",
		8: "crawler",
	}
	w.SetModel(model)
	src := placeTestBuilding(t, w, 2, 2, 418, 1, 0)
	factory := placeTestBuilding(t, w, 3, 2, 100, 1, 0)
	srcPos := int32(src.Y*w.Model().Width + src.X)
	factoryPos := int32(factory.Y*w.Model().Width + factory.X)

	if got, ok := w.BuildingConfigPacked(protocol.PackPoint2(3, 2)); !ok {
		t.Fatalf("expected default unit factory config to be present")
	} else if got != int32(0) {
		t.Fatalf("expected default unit factory plan 0, got %#v", got)
	}
	if !w.canAcceptItemLocked(srcPos, factoryPos, siliconItemID, 0) {
		t.Fatalf("expected default dagger plan to accept silicon")
	}
	if !w.canAcceptItemLocked(srcPos, factoryPos, leadItemID, 0) {
		t.Fatalf("expected default dagger plan to accept lead")
	}
	if w.canAcceptItemLocked(srcPos, factoryPos, coalItemID, 0) {
		t.Fatalf("expected default dagger plan to reject coal")
	}

	w.applyBuildingConfigLocked(factoryPos, int32(1), true)

	if got, ok := w.BuildingConfigPacked(protocol.PackPoint2(3, 2)); !ok {
		t.Fatalf("expected configured unit factory config to be present")
	} else if got != int32(1) {
		t.Fatalf("expected configured unit factory plan 1, got %#v", got)
	}
	if !w.canAcceptItemLocked(srcPos, factoryPos, coalItemID, 0) {
		t.Fatalf("expected crawler plan to accept coal")
	}
	if !w.canAcceptItemLocked(srcPos, factoryPos, siliconItemID, 0) {
		t.Fatalf("expected crawler plan to accept silicon")
	}
	if w.canAcceptItemLocked(srcPos, factoryPos, leadItemID, 0) {
		t.Fatalf("expected crawler plan to reject lead")
	}
}

func TestUnitFactoryCommandConfigPreservesPlanSelection(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		100: "ground-factory",
	}
	model.UnitNames = map[int16]string{
		7: "dagger",
		8: "crawler",
	}
	w.SetModel(model)
	factory := placeTestBuilding(t, w, 3, 2, 100, 1, 0)
	factoryPacked := protocol.PackPoint2(3, 2)
	factoryPos := int32(factory.Y*w.Model().Width + factory.X)

	w.applyBuildingConfigLocked(factoryPos, int32(1), true)
	w.applyBuildingConfigLocked(factoryPos, protocol.UnitCommand{ID: 2, Name: "repair"}, true)
	w.CommandBuildingsPacked([]int32{factoryPacked}, protocol.Vec2{X: 88, Y: 104})

	if got, ok := w.BuildingConfigPacked(factoryPacked); !ok {
		t.Fatalf("expected configured unit factory config to be present")
	} else if got != int32(1) {
		t.Fatalf("expected configured unit factory plan 1 to remain selected, got %#v", got)
	}
	if st := w.factoryStates[factoryPos]; st.Command == nil || st.Command.ID != 2 {
		t.Fatalf("expected unit factory command id 2, got %+v", st.Command)
	} else if st.CommandPos == nil || math.Abs(float64(st.CommandPos.X-88)) > 0.0001 || math.Abs(float64(st.CommandPos.Y-104)) > 0.0001 {
		t.Fatalf("expected unit factory commandPos (88,104), got %+v", st.CommandPos)
	}

	w.applyBuildingConfigLocked(factoryPos, nil, true)

	if got, ok := w.BuildingConfigPacked(factoryPacked); !ok {
		t.Fatalf("expected unit factory plan to stay configured after clearing command")
	} else if got != int32(1) {
		t.Fatalf("expected unit factory plan 1 after clearing command, got %#v", got)
	}
	if st := w.factoryStates[factoryPos]; st.Command != nil {
		t.Fatalf("expected unit factory command to clear, got %+v", st.Command)
	} else if st.CommandPos == nil || math.Abs(float64(st.CommandPos.X-88)) > 0.0001 || math.Abs(float64(st.CommandPos.Y-104)) > 0.0001 {
		t.Fatalf("expected unit factory commandPos to stay set, got %+v", st.CommandPos)
	}
}

func TestFactoryDumpedUnitCarriesCommandState(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(20, 20)
	model.BlockNames = map[int16]string{
		100: "ground-factory",
		339: "core-shard",
	}
	model.UnitNames = map[int16]string{
		7: "dagger",
	}
	w.SetModel(model)
	placeTestBuilding(t, w, 2, 2, 339, 1, 0)
	tile := placeTestBuilding(t, w, 5, 5, 100, 1, 0)
	pos := int32(tile.Y*w.Model().Width + tile.X)
	state := factoryState{
		UnitType:    7,
		CurrentPlan: 0,
		CommandPos:  &protocol.Vec2{X: 120, Y: 96},
		Command:     &protocol.UnitCommand{ID: 2, Name: "repair"},
	}
	w.factoryStates[pos] = state
	w.payloadStates[pos] = &payloadRuntimeState{Payload: w.newFactoryUnitPayloadLocked(tile, state)}
	if w.payloadStates[pos].Payload == nil {
		t.Fatal("expected factory payload to be created")
	}

	if !w.dumpUnitPayloadFromTileLocked(pos, tile) {
		t.Fatal("expected factory payload to dump into a world unit")
	}

	found := false
	for _, ent := range w.model.Entities {
		if ent.TypeID != 7 || ent.Team != 1 {
			continue
		}
		found = true
		if ent.CommandID != 2 {
			t.Fatalf("expected dumped unit command id 2, got %d", ent.CommandID)
		}
		if ent.Behavior != "move" {
			t.Fatalf("expected dumped unit behavior move, got %q", ent.Behavior)
		}
		if math.Abs(float64(ent.PatrolAX-120)) > 0.0001 || math.Abs(float64(ent.PatrolAY-96)) > 0.0001 {
			t.Fatalf("expected dumped unit target (120,96), got (%f,%f)", ent.PatrolAX, ent.PatrolAY)
		}
	}
	if !found {
		t.Fatal("expected dumped factory unit entity to exist")
	}
}

