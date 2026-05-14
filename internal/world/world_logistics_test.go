package world

import (
	"math"
	"testing"
	"time"

	"github.com/IYanHua/mdt-server/internal/protocol"
)

func TestItemLogisticsMovesThroughConveyorChainToReactor(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		315: "thorium-reactor",
		412: "item-source",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 1, 3, 412, 1, 0)
	placeTestBuilding(t, w, 2, 3, 257, 1, 0)
	placeTestBuilding(t, w, 3, 3, 257, 1, 0)
	placeTestBuilding(t, w, 4, 3, 315, 1, 0)

	w.ConfigureItemSource(int32(3*model.Width+1), 5)

	for i := 0; i < 420; i++ {
		w.Step(time.Second / 60)
	}

	reactor, _ := w.Model().TileAt(4, 3)
	if reactor.Block != 0 || reactor.Build != nil {
		t.Fatalf("expected thorium reactor to explode after conveyor-fed thorium, got block=%d build=%v", reactor.Block, reactor.Build != nil)
	}
}

func TestSorterRoutesMatchingItemForward(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		412: "item-source",
		500: "sorter",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 1, 3, 412, 1, 0)
	placeTestBuilding(t, w, 2, 3, 500, 1, 0)
	east := placeTestBuilding(t, w, 3, 3, 257, 1, 0)
	north := placeTestBuilding(t, w, 2, 2, 257, 1, 3)

	w.ConfigureItemSource(int32(3*model.Width+1), 5)
	w.ConfigureSorter(int32(3*model.Width+2), 5)

	for i := 0; i < 120; i++ {
		w.Step(time.Second / 60)
	}

	if east.Build.ItemAmount(5) == 0 {
		t.Fatalf("expected matching item to go forward through sorter")
	}
	if north.Build.ItemAmount(5) != 0 {
		t.Fatalf("expected matching item not to route sideways, got north=%d", north.Build.ItemAmount(5))
	}
}

func TestJunctionPassesCrossedFlows(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		412: "item-source",
		501: "junction",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 1, 3, 412, 1, 0)
	placeTestBuilding(t, w, 2, 2, 412, 1, 0)
	placeTestBuilding(t, w, 2, 3, 501, 1, 0)
	east := placeTestBuilding(t, w, 3, 3, 257, 1, 0)
	south := placeTestBuilding(t, w, 2, 4, 257, 1, 1)

	w.ConfigureItemSource(int32(3*model.Width+1), 5)
	w.ConfigureItemSource(int32(2*model.Width+2), 0)

	for i := 0; i < 240; i++ {
		w.Step(time.Second / 60)
	}

	if east.Build.ItemAmount(5) == 0 {
		t.Fatalf("expected west->east flow to pass through junction")
	}
	if south.Build.ItemAmount(0) == 0 {
		t.Fatalf("expected north->south flow to pass through junction")
	}
}

func TestPendingBuildAppliesConfigOnCompletion(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		412: "item-source",
	}
	w.SetModel(model)

	pos := int32(3*model.Width + 2)
	w.UpdateBuilderState(1, 1, 9001, float32(2*8+4), float32(3*8+4), true, 220)
	w.ApplyBuildPlansForOwner(1, 1, []BuildPlanOp{{
		X:       2,
		Y:       3,
		BlockID: 412,
		Config:  protocol.ItemRef{ItmID: 5},
	}})

	for i := 0; i < 30; i++ {
		w.Step(200 * time.Millisecond)
		tile, _ := w.Model().TileAt(2, 3)
		if tile.Build != nil {
			break
		}
	}

	if got := w.itemSourceCfg[pos]; got != 5 {
		t.Fatalf("expected pending build config to apply item-source item=5, got=%d", got)
	}
}

func TestRestoreSavedBridgeAndItemSourceConfig(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		412: "item-source",
		420: "bridge-conveyor",
	}

	w.SetModel(model)
	source := placeTestBuilding(t, w, 1, 3, 412, 1, 0)
	bridgeA := placeTestBuilding(t, w, 2, 3, 420, 1, 0)
	placeTestBuilding(t, w, 4, 3, 420, 1, 0)
	out := placeTestBuilding(t, w, 5, 3, 257, 1, 0)

	source.Build.Config = mustEncodeConfig(t, protocol.ItemRef{ItmID: 5})
	bridgeA.Build.Config = mustEncodeConfig(t, protocol.Point2{X: 2, Y: 0})

	w.SetModel(model)

	for i := 0; i < 180; i++ {
		w.Step(time.Second / 60)
	}

	if out.Build.ItemAmount(5) == 0 {
		t.Fatalf("expected saved item-source + bridge config to restore and move items across bridge")
	}
}

func TestRestoreSavedSorterConfig(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		412: "item-source",
		500: "sorter",
	}

	w.SetModel(model)
	source := placeTestBuilding(t, w, 1, 3, 412, 1, 0)
	sorter := placeTestBuilding(t, w, 2, 3, 500, 1, 0)
	forward := placeTestBuilding(t, w, 3, 3, 257, 1, 0)
	side := placeTestBuilding(t, w, 2, 2, 257, 1, 3)

	source.Build.Config = mustEncodeConfig(t, protocol.ItemRef{ItmID: 5})
	sorter.Build.Config = mustEncodeConfig(t, protocol.ItemRef{ItmID: 5})

	w.SetModel(model)

	for i := 0; i < 120; i++ {
		w.Step(time.Second / 60)
	}

	if forward.Build.ItemAmount(5) == 0 {
		t.Fatalf("expected saved sorter config to restore matching forward route")
	}
	if side.Build.ItemAmount(5) != 0 {
		t.Fatalf("expected saved sorter config not to route matching item sideways, got=%d", side.Build.ItemAmount(5))
	}
}

func TestSorterIntConfigFallbackSyncPath(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		412: "item-source",
		500: "sorter",
	}

	w.SetModel(model)
	placeTestBuilding(t, w, 1, 3, 412, 1, 0)
	sorter := placeTestBuilding(t, w, 2, 3, 500, 1, 0)
	forward := placeTestBuilding(t, w, 3, 3, 257, 1, 0)
	side := placeTestBuilding(t, w, 2, 2, 257, 1, 3)

	w.ConfigureItemSource(int32(3*model.Width+1), 5)
	w.ConfigureBuilding(int32(3*model.Width+2), int32(5))

	for i := 0; i < 120; i++ {
		w.Step(time.Second / 60)
	}

	if got, ok := w.BuildingConfigPacked(protocol.PackPoint2(2, 3)); !ok {
		t.Fatalf("expected sorter config to persist")
	} else if item, ok := got.(protocol.ItemRef); !ok || item.ItmID != 5 {
		t.Fatalf("expected sorter normalized config item=5, got=%T %#v", got, got)
	}
	if forward.Build.ItemAmount(5) == 0 {
		t.Fatalf("expected int-based sorter config to route matching item forward")
	}
	if side.Build.ItemAmount(5) != 0 {
		t.Fatalf("expected int-based sorter config not to route matching item sideways, got=%d", side.Build.ItemAmount(5))
	}
	if sorter.Build == nil || len(sorter.Build.Config) == 0 {
		t.Fatalf("expected sorter config bytes to be stored")
	}
}

func TestUnlinkedBridgeDoesNotAcceptItems(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		412: "item-source",
		420: "bridge-conveyor",
	}
	w.SetModel(model)

	source := placeTestBuilding(t, w, 1, 3, 412, 1, 0)
	bridge := placeTestBuilding(t, w, 2, 3, 420, 1, 0)

	source.Build.Config = mustEncodeConfig(t, protocol.ItemRef{ItmID: 5})
	w.SetModel(model)

	for i := 0; i < 120; i++ {
		w.Step(time.Second / 60)
	}

	if bridge.Build.ItemAmount(5) != 0 {
		t.Fatalf("expected unlinked bridge not to accept items, got=%d", bridge.Build.ItemAmount(5))
	}
}

func TestLinkedBridgeRejectsInputFromLinkedSide(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		412: "item-source",
		420: "bridge-conveyor",
	}
	w.SetModel(model)

	source := placeTestBuilding(t, w, 3, 3, 412, 1, 0)
	bridge := placeTestBuilding(t, w, 2, 3, 420, 1, 0)
	placeTestBuilding(t, w, 5, 3, 420, 1, 0)

	source.Build.Config = mustEncodeConfig(t, protocol.ItemRef{ItmID: 5})
	bridge.Build.Config = mustEncodeConfig(t, protocol.Point2{X: 3, Y: 0})
	w.SetModel(model)

	for i := 0; i < 180; i++ {
		w.Step(time.Second / 60)
	}

	bridge, _ = w.Model().TileAt(2, 3)
	if totalBuildingItems(bridge.Build) != 0 {
		t.Fatalf("expected linked bridge to reject input from its link-facing side, got=%d", totalBuildingItems(bridge.Build))
	}
}

func TestUnlinkedBridgeDoesNotDumpBackTowardIncomingBridge(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(10, 8)
	model.BlockNames = map[int16]string{
		412: "item-source",
		418: "router",
		420: "bridge-conveyor",
	}
	w.SetModel(model)

	source := placeTestBuilding(t, w, 0, 3, 412, 1, 0)
	bridgeA := placeTestBuilding(t, w, 1, 3, 420, 1, 0)
	placeTestBuilding(t, w, 4, 3, 420, 1, 2)
	west := placeTestBuilding(t, w, 3, 3, 418, 1, 0)
	east := placeTestBuilding(t, w, 5, 3, 418, 1, 0)

	source.Build.Config = mustEncodeConfig(t, protocol.ItemRef{ItmID: 5})
	bridgeA.Build.Config = mustEncodeConfig(t, protocol.Point2{X: 3, Y: 0})
	w.SetModel(model)

	for i := 0; i < 420; i++ {
		w.Step(time.Second / 60)
	}

	if west.Build.ItemAmount(5) != 0 {
		t.Fatalf("expected unlinked bridge not to dump back toward incoming side, got west=%d", west.Build.ItemAmount(5))
	}
	if east.Build.ItemAmount(5) == 0 {
		t.Fatalf("expected unlinked bridge to dump to a non-incoming side")
	}
}

func TestConveyorRuntimeTracksPerItemPositions(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		418: "router",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 2, 2, 257, 1, 0)
	placeTestBuilding(t, w, 1, 2, 418, 1, 0)
	placeTestBuilding(t, w, 2, 1, 418, 1, 0)

	conveyorPos := int32(2*model.Width + 2)
	behindPos := int32(2*model.Width + 1)
	northPos := int32(1*model.Width + 2)

	if !w.tryInsertItemLocked(behindPos, conveyorPos, 5, 0) {
		t.Fatalf("expected rear insert into conveyor to succeed")
	}
	state := w.conveyorStates[conveyorPos]
	if state == nil || state.Len != 1 || state.YS[0] != 0 {
		t.Fatalf("expected first item at conveyor origin, got state=%+v", state)
	}

	w.Step(500 * time.Millisecond)
	state = w.conveyorStates[conveyorPos]
	if state == nil || state.MinItem <= 0.7 {
		t.Fatalf("expected conveyor item to advance enough for side insert, got minitem=%v", state.MinItem)
	}

	if !w.tryInsertItemLocked(northPos, conveyorPos, 6, 0) {
		t.Fatalf("expected side insert into conveyor to succeed after spacing opens")
	}
	state = w.conveyorStates[conveyorPos]
	if state.Len != 2 {
		t.Fatalf("expected 2 runtime items, got=%d", state.Len)
	}
	if state.YS[state.LastInserted] != 0.5 {
		t.Fatalf("expected side inserted item at y=0.5, got=%v", state.YS[state.LastInserted])
	}
	if totalBuildingItems(w.Model().Tiles[conveyorPos].Build) != 2 {
		t.Fatalf("expected mirrored building inventory total=2, got=%d", totalBuildingItems(w.Model().Tiles[conveyorPos].Build))
	}
}

func TestConveyorRuntimePassesItemsForward(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		418: "router",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 2, 2, 257, 1, 0)
	placeTestBuilding(t, w, 3, 2, 257, 1, 0)
	placeTestBuilding(t, w, 1, 2, 418, 1, 0)

	firstPos := int32(2*model.Width + 2)
	secondPos := int32(2*model.Width + 3)
	sourcePos := int32(2*model.Width + 1)

	if !w.tryInsertItemLocked(sourcePos, firstPos, 5, 0) {
		t.Fatalf("expected rear insert into first conveyor to succeed")
	}

	for i := 0; i < 120; i++ {
		w.Step(time.Second / 60)
	}

	first := w.conveyorStates[firstPos]
	second := w.conveyorStates[secondPos]
	if first != nil && first.Len != 0 {
		t.Fatalf("expected first conveyor to pass item onward, len=%d", first.Len)
	}
	if second == nil || second.Len == 0 {
		t.Fatalf("expected second conveyor runtime to receive item")
	}
	if totalBuildingItems(w.Model().Tiles[secondPos].Build) == 0 {
		t.Fatalf("expected mirrored inventory on second conveyor to contain item")
	}
}

func TestRouterImmediatelyPassesToConveyor(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		418: "router",
	}
	w.SetModel(model)

	router := placeTestBuilding(t, w, 2, 3, 418, 1, 0)
	placeTestBuilding(t, w, 3, 3, 257, 1, 0)
	router.Build.AddItem(5, 1)

	w.Step(time.Second / 60)

	conveyorPos := int32(3*model.Width + 3)
	state := w.conveyorStates[conveyorPos]
	if state == nil || state.Len == 0 {
		t.Fatalf("expected router to immediately pass item into conveyor on first frame")
	}
	if totalBuildingItems(router.Build) != 0 {
		t.Fatalf("expected router inventory to be empty after immediate pass, got=%d", totalBuildingItems(router.Build))
	}
}

func TestSorterRejectsInstantTransferThreeChain(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		500: "sorter",
		502: "overflow-gate",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 1, 3, 502, 1, 0)
	placeTestBuilding(t, w, 2, 3, 500, 1, 0)
	placeTestBuilding(t, w, 3, 3, 502, 1, 0)

	if w.tryInsertItemLocked(int32(3*model.Width+1), int32(3*model.Width+2), 5, 0) {
		t.Fatalf("expected sorter to reject instantTransfer three-chain forward path")
	}
}

func TestOverflowRejectsInstantTransferForward(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		500: "sorter",
		502: "overflow-gate",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 1, 3, 500, 1, 0)
	placeTestBuilding(t, w, 2, 3, 502, 1, 0)
	placeTestBuilding(t, w, 3, 3, 500, 1, 0)

	if w.tryInsertItemLocked(int32(3*model.Width+1), int32(3*model.Width+2), 5, 0) {
		t.Fatalf("expected overflow gate to reject instantTransfer forward chain")
	}
}

func TestSorterCanAcceptDoesNotFlipRotation(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		418: "router",
		500: "sorter",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 1, 3, 418, 1, 0)
	placeTestBuilding(t, w, 2, 3, 500, 1, 0)
	placeTestBuilding(t, w, 2, 2, 418, 1, 0)
	placeTestBuilding(t, w, 2, 4, 418, 1, 0)

	sorterPos := int32(3*model.Width + 2)
	sourcePos := int32(3*model.Width + 1)
	w.routerRotation[sorterPos] = 0

	if !w.canAcceptItemLocked(sourcePos, sorterPos, 5, 0) {
		t.Fatalf("expected sorter accept probe to succeed")
	}
	if w.routerRotation[sorterPos] != 0 {
		t.Fatalf("expected sorter accept probe not to flip rotation, got=%d", w.routerRotation[sorterPos])
	}
}

func TestOverflowCanAcceptDoesNotFlipRotation(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		418: "router",
		502: "overflow-gate",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 1, 3, 418, 1, 0)
	placeTestBuilding(t, w, 2, 3, 502, 1, 0)
	placeTestBuilding(t, w, 2, 2, 418, 1, 0)
	placeTestBuilding(t, w, 2, 4, 418, 1, 0)

	gatePos := int32(3*model.Width + 2)
	sourcePos := int32(3*model.Width + 1)
	w.routerRotation[gatePos] = 0

	if !w.canAcceptItemLocked(sourcePos, gatePos, 5, 0) {
		t.Fatalf("expected overflow accept probe to succeed")
	}
	if w.routerRotation[gatePos] != 0 {
		t.Fatalf("expected overflow accept probe not to flip rotation, got=%d", w.routerRotation[gatePos])
	}
}

func TestConsumeGeneratorItemCapacitiesMatchVanilla(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(4, 4)
	model.BlockNames = map[int16]string{
		308: "combustion-generator",
		309: "steam-generator",
		310: "differential-generator",
		311: "rtg-generator",
	}
	w.SetModel(model)

	tests := []struct {
		name  string
		block int16
		want  int32
	}{
		{name: "combustion-generator", block: 308, want: 10},
		{name: "steam-generator", block: 309, want: 10},
		{name: "differential-generator", block: 310, want: 10},
		{name: "rtg-generator", block: 311, want: 10},
	}

	for _, tc := range tests {
		if got := w.itemCapacityForBlockLocked(&Tile{Block: BlockID(tc.block)}); got != tc.want {
			t.Fatalf("expected %s capacity=%d, got=%d", tc.name, tc.want, got)
		}
	}
}

func TestUnderflowRoutesIntoConsumeGeneratorBeforeCore(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(10, 10)
	model.BlockNames = map[int16]string{
		308: "combustion-generator",
		339: "core-shard",
		412: "item-source",
		503: "underflow-gate",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 2, 5, 412, 1, 0)
	placeTestBuilding(t, w, 3, 5, 503, 1, 0)
	gen := placeTestBuilding(t, w, 3, 4, 308, 1, 0)
	core := placeTestBuilding(t, w, 5, 5, 339, 1, 0)

	sourcePos := int32(5*model.Width + 2)
	gatePos := int32(5*model.Width + 3)
	item := coalItemID

	if !w.tryInsertItemLocked(sourcePos, gatePos, item, 0) {
		t.Fatalf("expected underflow gate to route item into combustion generator")
	}
	if got := gen.Build.ItemAmount(item); got != 1 {
		t.Fatalf("expected combustion generator to receive item, got=%d", got)
	}
	if got := core.Build.ItemAmount(item); got != 0 {
		t.Fatalf("expected core inventory to remain untouched, got=%d", got)
	}
}

func TestArmoredConveyorRejectsSideInputFromNonConveyor(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		418: "router",
		259: "armored-conveyor",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 2, 2, 259, 1, 0)
	placeTestBuilding(t, w, 2, 1, 418, 1, 0)

	armoredPos := int32(2*model.Width + 2)
	sourcePos := int32(1*model.Width + 2)
	if w.tryInsertItemLocked(sourcePos, armoredPos, 5, 0) {
		t.Fatalf("expected armored conveyor to reject side input from non-conveyor")
	}
}

func TestArmoredConveyorAcceptsSideInputFromConveyor(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		259: "armored-conveyor",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 2, 2, 259, 1, 0)
	placeTestBuilding(t, w, 2, 1, 257, 1, 1)

	armoredPos := int32(2*model.Width + 2)
	sourcePos := int32(1*model.Width + 2)
	if !w.tryInsertItemLocked(sourcePos, armoredPos, 5, 0) {
		t.Fatalf("expected armored conveyor to accept side input from conveyor")
	}
}

func TestUnlinkedBridgeDumpRotatesTargets(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		418: "router",
		420: "bridge-conveyor",
	}
	w.SetModel(model)

	bridge := placeTestBuilding(t, w, 2, 2, 420, 1, 0)
	placeTestBuilding(t, w, 3, 2, 418, 1, 0)
	placeTestBuilding(t, w, 2, 3, 418, 1, 0)

	bridgePos := int32(2*model.Width + 2)
	first, ok := w.bridgeDumpTargetLocked(bridgePos, bridge, 5)
	if !ok {
		t.Fatalf("expected first dump target for unlinked bridge")
	}
	second, ok := w.bridgeDumpTargetLocked(bridgePos, bridge, 5)
	if !ok {
		t.Fatalf("expected second dump target for unlinked bridge")
	}
	if first == second {
		t.Fatalf("expected dump rotation to advance to a different target, got same=%d", first)
	}
}

func TestItemSourceDumpsOnePathPerUpdate(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		412: "item-source",
		418: "router",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 2, 2, 412, 1, 0)
	east := placeTestBuilding(t, w, 3, 2, 418, 1, 0)
	south := placeTestBuilding(t, w, 2, 3, 418, 1, 0)
	w.ConfigureItemSource(int32(2*model.Width+2), 5)

	w.Step(time.Second / 60)

	total := totalBuildingItems(east.Build) + totalBuildingItems(south.Build)
	if total != 1 {
		t.Fatalf("expected item source to dump through exactly one path on first update, got total=%d", total)
	}
}

func TestUnlinkedBridgeDumpsOnFirstFrame(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		418: "router",
		420: "bridge-conveyor",
	}
	w.SetModel(model)

	bridge := placeTestBuilding(t, w, 2, 2, 420, 1, 0)
	out := placeTestBuilding(t, w, 3, 2, 418, 1, 0)
	bridge.Build.AddItem(5, 1)

	w.Step(time.Second / 60)

	if totalBuildingItems(out.Build) != 1 {
		t.Fatalf("expected unlinked bridge to dump on first frame, got=%d", totalBuildingItems(out.Build))
	}
	if totalBuildingItems(bridge.Build) != 0 {
		t.Fatalf("expected bridge inventory empty after first-frame dump, got=%d", totalBuildingItems(bridge.Build))
	}
}

func TestDistributorUsesMultiBlockProximity(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(10, 10)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		600: "distributor",
	}
	w.SetModel(model)

	distributor := placeTestBuilding(t, w, 3, 3, 600, 1, 0)
	placeTestBuilding(t, w, 2, 3, 257, 1, 0)
	placeTestBuilding(t, w, 2, 4, 257, 1, 0)
	placeTestBuilding(t, w, 5, 3, 257, 1, 0)
	placeTestBuilding(t, w, 5, 4, 257, 1, 0)

	distributor.Build.AddItem(5, 1)

	w.Step(time.Second / 60)

	moved := 0
	for _, pos := range []int32{
		int32(3*model.Width + 2),
		int32(4*model.Width + 2),
		int32(3*model.Width + 5),
		int32(4*model.Width + 5),
	} {
		if st := w.conveyorStates[pos]; st != nil && st.Len > 0 {
			moved++
		}
	}
	if moved != 1 {
		t.Fatalf("expected distributor to route exactly one item into adjacent edge conveyor, moved=%d", moved)
	}
	if totalBuildingItems(distributor.Build) != 0 {
		t.Fatalf("expected distributor inventory empty after routing, got=%d", totalBuildingItems(distributor.Build))
	}
}

func TestItemSourceFeedsThoriumReactorAcrossMultiBlockEdge(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(10, 10)
	model.BlockNames = map[int16]string{
		315: "thorium-reactor",
		412: "item-source",
	}
	w.SetModel(model)

	reactor := placeTestBuilding(t, w, 4, 4, 315, 1, 0)
	placeTestBuilding(t, w, 6, 4, 412, 1, 0)
	w.ConfigureItemSource(int32(4*model.Width+6), 5)

	w.Step(time.Second / 60)

	if reactor.Build.ItemAmount(5) == 0 {
		t.Fatalf("expected item source to feed thorium reactor through multiblock edge")
	}
}

func TestItemSourceFeedsCoreAcrossMultiBlockEdge(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		343: "core-citadel",
		412: "item-source",
	}
	w.SetModel(model)

	core := placeTestBuilding(t, w, 5, 5, 343, 1, 0)
	placeTestBuilding(t, w, 8, 5, 412, 1, 0)
	w.ConfigureItemSource(int32(5*model.Width+8), 1)

	w.Step(time.Second / 60)

	if core.Build.ItemAmount(1) == 0 {
		t.Fatalf("expected item source to feed core through multiblock edge")
	}
}

func TestConveyorFeedsCoreAcrossMultiBlockEdge(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(14, 14)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		343: "core-citadel",
	}
	w.SetModel(model)

	core := placeTestBuilding(t, w, 5, 5, 343, 1, 0)
	conveyor := placeTestBuilding(t, w, 8, 5, 257, 1, 2)
	conveyor.Build.AddItem(0, 1)

	for i := 0; i < 120; i++ {
		w.Step(time.Second / 60)
		if core.Build.ItemAmount(0) > 0 {
			break
		}
	}

	if core.Build.ItemAmount(0) == 0 {
		t.Fatalf("expected conveyor to feed core through multiblock edge")
	}
	if conveyor.Build.ItemAmount(0) != 0 {
		t.Fatalf("expected conveyor inventory drained into core, got=%d", conveyor.Build.ItemAmount(0))
	}
}

func TestTeamCoreItemSnapshotsUseRealCoreInventory(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		343: "core-citadel",
	}
	w.SetModel(model)

	core := placeTestBuilding(t, w, 5, 5, 343, 1, 0)
	core.Build.AddItem(0, 7)
	core.Build.AddItem(5, 3)
	w.teamItems[1] = map[ItemID]int32{
		0: 2900,
		1: 1900,
	}

	snapshots := w.TeamCoreItemSnapshots()
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 core snapshot, got %d", len(snapshots))
	}
	if snapshots[0].Team != 1 {
		t.Fatalf("expected team 1 snapshot, got team %d", snapshots[0].Team)
	}
	if len(snapshots[0].Items) != 2 {
		t.Fatalf("expected 2 real core items, got %d", len(snapshots[0].Items))
	}
	if snapshots[0].Items[0].Item != 0 || snapshots[0].Items[0].Amount != 7 {
		t.Fatalf("expected copper amount 7, got item=%d amount=%d", snapshots[0].Items[0].Item, snapshots[0].Items[0].Amount)
	}
	if snapshots[0].Items[1].Item != 5 || snapshots[0].Items[1].Amount != 3 {
		t.Fatalf("expected sand amount 3, got item=%d amount=%d", snapshots[0].Items[1].Item, snapshots[0].Items[1].Amount)
	}
}

func TestTeamCoreItemSnapshotsDoNotFallbackToTeamInventoryWhenCoreEmpty(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		343: "core-citadel",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 5, 5, 343, 1, 0)
	w.teamItems[1] = map[ItemID]int32{
		0: 120,
		4: 45,
	}

	snapshots := w.TeamCoreItemSnapshots()
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 core snapshot, got %d", len(snapshots))
	}
	if snapshots[0].Team != 1 {
		t.Fatalf("expected team 1 snapshot, got team %d", snapshots[0].Team)
	}
	if len(snapshots[0].Items) != 0 {
		t.Fatalf("expected empty real core inventory, got %d items", len(snapshots[0].Items))
	}
}

func TestFillItemsTeamFillsRealCoreInventoryOnStep(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		343: "core-citadel",
	}
	w.SetModel(model)

	core := placeTestBuilding(t, w, 5, 5, 343, 1, 0)
	rules := w.GetRulesManager().Get()
	rules.setTeamRule(1, TeamRule{FillItems: true})

	pos := int32(core.Y*model.Width + core.X)
	capacity := w.itemCapacityAtLocked(pos)
	if capacity <= 0 {
		t.Fatalf("expected positive shared core capacity, got %d", capacity)
	}
	if got := core.Build.ItemAmount(copperItemID); got != 0 {
		t.Fatalf("expected empty core before fill step, got copper=%d", got)
	}

	w.Step(time.Second / 60)

	snapshots := w.TeamCoreItemSnapshots()
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 core snapshot, got %d", len(snapshots))
	}
	if snapshots[0].Team != 1 {
		t.Fatalf("expected team 1 snapshot, got team %d", snapshots[0].Team)
	}
	if got := core.Build.ItemAmount(copperItemID); got != capacity {
		t.Fatalf("expected fillItems step to fill copper to %d, got %d", capacity, got)
	}
	if got := core.Build.ItemAmount(titaniumItemID); got != capacity {
		t.Fatalf("expected fillItems step to fill titanium to %d, got %d", capacity, got)
	}
	itemAmounts := make(map[ItemID]int32, len(snapshots[0].Items))
	for _, stack := range snapshots[0].Items {
		itemAmounts[stack.Item] = stack.Amount
	}
	if got := itemAmounts[copperItemID]; got != capacity {
		t.Fatalf("expected core snapshot copper=%d after fill step, got %d", capacity, got)
	}
	if got := itemAmounts[titaniumItemID]; got != capacity {
		t.Fatalf("expected core snapshot titanium=%d after fill step, got %d", capacity, got)
	}
}

func TestTeamItemsReflectRealCoreInput(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		343: "core-citadel",
		412: "item-source",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 5, 5, 343, 1, 0)
	placeTestBuilding(t, w, 8, 5, 412, 1, 0)
	w.ConfigureItemSource(int32(5*model.Width+8), 0)

	w.Step(time.Second / 60)

	items := w.TeamItems(1)
	if items[0] == 0 {
		t.Fatalf("expected team item view to reflect real core input, got copper=%d", items[0])
	}
}

func TestCoreFeedEmitsTeamItemEvent(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		343: "core-citadel",
		412: "item-source",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 5, 5, 343, 1, 0)
	placeTestBuilding(t, w, 8, 5, 412, 1, 0)
	w.ConfigureItemSource(int32(5*model.Width+8), 0)

	w.Step(time.Second / 60)

	evs := w.DrainEntityEvents()
	for _, ev := range evs {
		if ev.Kind == EntityEventTeamItems && ev.BuildTeam == 1 && ev.ItemID == 0 && ev.ItemAmount > 0 {
			return
		}
	}
	t.Fatalf("expected feeding a core to emit a team item sync event")
}

func TestSiliconSmelterDumpsOutputIntoCoreAndEmitsTeamItemEvent(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.BlockNames = map[int16]string{
		343: "core-citadel",
		421: "battery",
		422: "power-node",
		451: "silicon-smelter",
	}
	w.SetModel(model)

	core := placeTestBuilding(t, w, 10, 6, 343, 1, 0)
	smelter := placeTestBuilding(t, w, 6, 6, 451, 1, 0)
	placeTestBuilding(t, w, 6, 11, 421, 1, 0)
	placeTestBuilding(t, w, 6, 9, 422, 1, 0)
	w.powerStorageState[int32(11*model.Width+6)] = 4000
	linkPowerNode(t, w, 6, 9, protocol.Point2{X: 0, Y: -3}, protocol.Point2{X: 0, Y: 2})

	smelter.Build.AddItem(coalItemID, 2)
	smelter.Build.AddItem(sandItemID, 4)

	foundTeamItemEvent := false
	foundBlockItemSync := false
	for i := 0; i < 240; i++ {
		w.Step(time.Second / 60)
		for _, ev := range w.DrainEntityEvents() {
			if ev.Kind == EntityEventTeamItems && ev.BuildTeam == 1 && ev.ItemID == siliconItemID && ev.ItemAmount > 0 {
				foundTeamItemEvent = true
			}
			if ev.Kind == EntityEventBlockItemSync && ev.BuildPos == protocol.PackPoint2(6, 6) {
				foundBlockItemSync = true
			}
		}
		if core.Build.ItemAmount(siliconItemID) > 0 {
			break
		}
	}

	if got := core.Build.ItemAmount(siliconItemID); got <= 0 {
		t.Fatalf("expected silicon smelter output to reach adjacent core, got=%d", got)
	}
	if got := smelter.Build.ItemAmount(siliconItemID); got != 0 {
		t.Fatalf("expected smelter output buffer to dump into the core, got=%d", got)
	}
	if got := w.TeamItems(1)[siliconItemID]; got <= 0 {
		t.Fatalf("expected team item view to reflect dumped silicon, got=%d", got)
	}
	if !foundTeamItemEvent {
		t.Fatalf("expected crafter dumping into core to emit a team item sync event")
	}
	if !foundBlockItemSync {
		t.Fatalf("expected crafter inventory changes to emit a block item sync event")
	}
}

func TestFindNearestFriendlyCoreLockedChoosesNearestCore(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		339: "core-shard",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 4, 4, 339, 1, 0)
	placeTestBuilding(t, w, 18, 4, 339, 1, 0)

	src := RawEntity{Team: 1, X: float32(18*8 + 4), Y: float32(6*8 + 4)}
	target, ok := w.findNearestFriendlyCoreLocked(src)
	if !ok {
		t.Fatalf("expected nearest friendly core")
	}
	if target.BuildPos != int32(4*model.Width+18) {
		t.Fatalf("expected right core to be chosen, got pos=%d", target.BuildPos)
	}
	if !w.entityNearCoreLocked(RawEntity{Team: 1, X: float32(18*8 + 4), Y: float32(4*8 + 4)}, 80) {
		t.Fatalf("expected entityNearCoreLocked to detect nearby core via cached core positions")
	}
}

func TestFindNearestEnemyCoreLockedChoosesNearestEnemyCore(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		339: "core-shard",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 4, 4, 339, 2, 0)
	placeTestBuilding(t, w, 18, 4, 339, 3, 0)

	src := RawEntity{Team: 1, X: float32(17*8 + 4), Y: float32(5*8 + 4)}
	target, ok := w.findNearestEnemyCoreLocked(src)
	if !ok {
		t.Fatalf("expected nearest enemy core")
	}
	if target.BuildPos != int32(4*model.Width+18) {
		t.Fatalf("expected nearest enemy core at right side, got pos=%d", target.BuildPos)
	}
}

func TestActiveTileIndexesTrackPowerAndLogisticsCategories(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		339: "core-shard",
		422: "power-node",
		480: "diode",
		481: "power-void",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 2, 2, 257, 1, 0)
	placeTestBuilding(t, w, 4, 4, 422, 1, 0)
	placeTestBuilding(t, w, 6, 4, 480, 1, 0)
	placeTestBuilding(t, w, 8, 4, 481, 1, 0)
	placeTestBuilding(t, w, 10, 10, 339, 1, 0)

	if got := len(w.itemLogisticsTilePositions); got != 1 {
		t.Fatalf("expected 1 item logistics tile, got %d", got)
	}
	if got := len(w.powerTilePositions); got != 3 {
		t.Fatalf("expected 3 power tiles, got %d", got)
	}
	if got := len(w.powerDiodeTilePositions); got != 1 {
		t.Fatalf("expected 1 power diode tile, got %d", got)
	}
	if got := len(w.powerVoidTilePositions); got != 1 {
		t.Fatalf("expected 1 power void tile, got %d", got)
	}
	if got := len(w.teamPowerTiles[1]); got != 3 {
		t.Fatalf("expected team power tile list size 3, got %d", got)
	}
	if got := len(w.teamPowerNodeTiles[1]); got != 1 {
		t.Fatalf("expected team power node tile list size 1, got %d", got)
	}
	if got := len(w.teamCoreTiles[1]); got != 1 {
		t.Fatalf("expected team core tile list size 1, got %d", got)
	}
}

func TestActiveTileIndexesTrackProductionCategories(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(20, 20)
	model.BlockNames = map[int16]string{
		100: "ground-factory",
		429: "mechanical-drill",
		440: "mechanical-pump",
		442: "water-extractor",
		451: "silicon-smelter",
		453: "separator",
		459: "incinerator",
		465: "heat-redirector",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 2, 2, 451, 1, 0)
	placeTestBuilding(t, w, 4, 2, 453, 1, 0)
	placeTestBuilding(t, w, 6, 2, 429, 1, 0)
	placeTestBuilding(t, w, 8, 2, 440, 1, 0)
	placeTestBuilding(t, w, 10, 2, 442, 1, 0)
	placeTestBuilding(t, w, 12, 2, 459, 1, 0)
	placeTestBuilding(t, w, 14, 2, 100, 1, 0)
	placeTestBuilding(t, w, 16, 2, 465, 1, 0)

	if got := len(w.crafterTilePositions); got != 2 {
		t.Fatalf("expected 2 crafter tiles, got %d", got)
	}
	if got := len(w.drillTilePositions); got != 1 {
		t.Fatalf("expected 1 drill tile, got %d", got)
	}
	if got := len(w.pumpTilePositions); got != 2 {
		t.Fatalf("expected 2 pump tiles, got %d", got)
	}
	if got := len(w.incineratorTilePositions); got != 1 {
		t.Fatalf("expected 1 incinerator tile, got %d", got)
	}
	if got := len(w.factoryTilePositions); got != 1 {
		t.Fatalf("expected 1 factory tile, got %d", got)
	}
	if got := len(w.heatConductorTilePositions); got != 1 {
		t.Fatalf("expected 1 heat conductor tile, got %d", got)
	}
}

func TestLinkedStorageMergesIntoCoreInventory(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 12)
	model.BlockNames = map[int16]string{
		339: "core-shard",
		431: "vault",
	}
	w.SetModel(model)

	core := placeTestBuilding(t, w, 5, 5, 339, 1, 0)
	store := placeTestBuilding(t, w, 8, 5, 431, 1, 0)
	store.Build.AddItem(0, 4)
	w.rebuildBlockOccupancyLocked()

	if got := w.TeamItems(1)[0]; got != 4 {
		t.Fatalf("expected linked vault items to merge into core inventory view, got %d", got)
	}
	if core.Build.ItemAmount(0) != 4 {
		t.Fatalf("expected primary core to hold merged linked inventory, got %d", core.Build.ItemAmount(0))
	}
	if store.Build.ItemAmount(0) != 0 {
		t.Fatalf("expected linked vault local inventory cleared after merge, got %d", store.Build.ItemAmount(0))
	}
	positions := w.TeamItemSyncPositions(1)
	if len(positions) != 2 {
		t.Fatalf("expected core and linked vault sync positions, got %d", len(positions))
	}
}

func TestBuildCostConsumesCoreInventoryWhenCorePresent(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		45:  "duo",
		343: "core-citadel",
	}
	w.SetModel(model)

	core := placeTestBuilding(t, w, 5, 5, 343, 1, 0)
	core.Build.AddItem(0, 40)

	w.ApplyBuildPlans(TeamID(1), []BuildPlanOp{{
		X: 2, Y: 2, BlockID: 45,
	}})
	w.Step(3 * time.Second)

	if core.Build.ItemAmount(0) != 5 {
		t.Fatalf("expected duo build to consume real core copper, got %d", core.Build.ItemAmount(0))
	}
}

func TestFillItemsTeamBuildUsesRealCoreInventory(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		45:  "duo",
		343: "core-citadel",
	}
	w.SetModel(model)

	core := placeTestBuilding(t, w, 5, 5, 343, 1, 0)
	rules := w.GetRulesManager().Get()
	rules.setTeamRule(1, TeamRule{FillItems: true})
	pos := int32(core.Y*model.Width + core.X)
	capacity := w.itemCapacityAtLocked(pos)
	if capacity <= 0 {
		t.Fatalf("expected positive shared core capacity, got %d", capacity)
	}

	w.ApplyBuildPlans(TeamID(1), []BuildPlanOp{{
		X: 2, Y: 2, BlockID: 45,
	}})
	stepForSeconds(w, 3)

	tile, err := w.Model().TileAt(2, 2)
	if err != nil || tile == nil {
		t.Fatalf("built tile lookup failed: %v", err)
	}
	if tile.Block != 45 || tile.Build == nil {
		t.Fatalf("expected duo to be fully constructed from fillItems core, got block=%d build=%v", tile.Block, tile.Build != nil)
	}
	if got := core.Build.ItemAmount(copperItemID); got != capacity {
		t.Fatalf("expected fillItems core copper to refill back to %d after build, got %d", capacity, got)
	}
}

func TestSandboxModeBuildIgnoresResourceCost(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		45:  "duo",
		339: "core-shard",
	}
	model.Tags = map[string]string{
		"mode": "sandbox",
	}
	w.SetModel(model)

	core := placeTestBuilding(t, w, 5, 5, 339, 1, 0)
	w.ApplyBuildPlans(TeamID(1), []BuildPlanOp{{
		X: 2, Y: 2, BlockID: 45,
	}})
	stepForSeconds(w, 3)

	tile, err := w.Model().TileAt(2, 2)
	if err != nil || tile == nil {
		t.Fatalf("built tile lookup failed: %v", err)
	}
	if tile.Block != 45 || tile.Build == nil {
		t.Fatalf("expected sandbox build to finish without materials, got block=%d build=%v", tile.Block, tile.Build != nil)
	}
	if got := core.Build.ItemAmount(copperItemID); got != 0 {
		t.Fatalf("expected sandbox build to consume no core copper, got %d", got)
	}
}

func TestAttackModeWaveTeamBuildIgnoresResourceCost(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		45:  "duo",
		339: "core-shard",
	}
	model.Tags = map[string]string{
		"mode": "attack",
	}
	w.SetModel(model)

	core := placeTestBuilding(t, w, 5, 5, 339, 2, 0)
	w.ApplyBuildPlans(TeamID(2), []BuildPlanOp{{
		X: 2, Y: 2, BlockID: 45,
	}})
	stepForSeconds(w, 3)

	tile, err := w.Model().TileAt(2, 2)
	if err != nil || tile == nil {
		t.Fatalf("built tile lookup failed: %v", err)
	}
	if tile.Block != 45 || tile.Build == nil {
		t.Fatalf("expected attack wave team build to finish without materials, got block=%d build=%v", tile.Block, tile.Build != nil)
	}
	if got := core.Build.ItemAmount(copperItemID); got != 0 {
		t.Fatalf("expected attack wave team build to consume no core copper, got %d", got)
	}
}

func TestSurvivalModeBuildStillWaitsForResources(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		45:  "duo",
		339: "core-shard",
	}
	model.Tags = map[string]string{
		"mode": "survival",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 5, 5, 339, 1, 0)
	pos := int32(2 + 2*model.Width)
	w.ApplyBuildPlans(TeamID(1), []BuildPlanOp{{
		X: 2, Y: 2, BlockID: 45,
	}})
	stepForSeconds(w, 3)

	tile, err := w.Model().TileAt(2, 2)
	if err != nil || tile == nil {
		t.Fatalf("built tile lookup failed: %v", err)
	}
	if tile.Block != 0 || tile.Build != nil {
		t.Fatalf("expected survival build without copper to stay pending, got block=%d build=%v", tile.Block, tile.Build != nil)
	}
	if _, ok := w.pendingBuilds[pos]; !ok {
		t.Fatalf("expected survival build to remain queued while missing materials")
	}
}

func TestPvpModeBuildStillWaitsForResources(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		45:  "duo",
		339: "core-shard",
	}
	model.Tags = map[string]string{
		"mode": "pvp",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 5, 5, 339, 1, 0)
	pos := int32(2 + 2*model.Width)
	w.ApplyBuildPlans(TeamID(1), []BuildPlanOp{{
		X: 2, Y: 2, BlockID: 45,
	}})
	stepForSeconds(w, 3)

	tile, err := w.Model().TileAt(2, 2)
	if err != nil || tile == nil {
		t.Fatalf("built tile lookup failed: %v", err)
	}
	if tile.Block != 0 || tile.Build != nil {
		t.Fatalf("expected pvp build without copper to stay pending, got block=%d build=%v", tile.Block, tile.Build != nil)
	}
	if _, ok := w.pendingBuilds[pos]; !ok {
		t.Fatalf("expected pvp build to remain queued while missing materials")
	}
}

func TestPhaseConveyorTransfersLinkedItems(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 8)
	model.BlockNames = map[int16]string{
		418: "router",
		421: "phase-conveyor",
	}
	w.SetModel(model)

	src := placeTestBuilding(t, w, 2, 3, 421, 1, 0)
	dst := placeTestBuilding(t, w, 6, 3, 421, 1, 0)
	out := placeTestBuilding(t, w, 7, 3, 418, 1, 0)
	w.ConfigureBuilding(int32(3*model.Width+2), protocol.Point2{X: 4, Y: 0})

	src.Build.AddItem(5, 1)

	for i := 0; i < 6; i++ {
		w.Step(time.Second / 60)
	}

	if totalBuildingItems(out.Build) != 1 {
		t.Fatalf("expected linked phase conveyor to transfer item, got=%d", totalBuildingItems(out.Build))
	}
	if totalBuildingItems(dst.Build) != 0 {
		t.Fatalf("expected target phase conveyor inventory drained after transfer, got=%d", totalBuildingItems(dst.Build))
	}
}

func TestConfigureBuildingPackedBridgeUsesOfficialAbsolutePos(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 8)
	model.BlockNames = map[int16]string{
		420: "bridge-conveyor",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 2, 3, 420, 1, 0)
	placeTestBuilding(t, w, 6, 3, 420, 1, 0)

	srcPacked := protocol.PackPoint2(2, 3)
	dstPacked := protocol.PackPoint2(6, 3)

	w.ConfigureBuildingPacked(srcPacked, dstPacked)

	cfg, ok := w.BuildingConfigPacked(srcPacked)
	if !ok {
		t.Fatalf("expected packed building config to be readable")
	}
	point, ok := cfg.(protocol.Point2)
	if !ok {
		t.Fatalf("expected normalized bridge config to be Point2, got %T", cfg)
	}
	if point.X != 4 || point.Y != 0 {
		t.Fatalf("expected relative bridge config (4,0), got (%d,%d)", point.X, point.Y)
	}
}

func TestRotateBuildingPackedUpdatesTileRotation(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		425: "power-node-large",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 3, 4, 425, 1, 0)
	pos := protocol.PackPoint2(3, 4)

	res, ok := w.RotateBuildingPacked(pos, true)
	if !ok {
		t.Fatal("expected rotate call to succeed")
	}
	if res.BlockID != 425 || res.Rotation != 1 || res.Team != 1 {
		t.Fatalf("unexpected rotate result: %+v", res)
	}
	if res.EffectX != 28 || res.EffectY != 36 {
		t.Fatalf("expected effect position (28,36), got (%f,%f)", res.EffectX, res.EffectY)
	}
	if res.EffectRot != 2 {
		t.Fatalf("expected rotate effect payload size 2, got %f", res.EffectRot)
	}

	tile, err := model.TileAt(3, 4)
	if err != nil || tile == nil || tile.Build == nil {
		t.Fatalf("tile lookup failed after rotate: %v", err)
	}
	if tile.Rotation != 1 || tile.Build.Rotation != 1 {
		t.Fatalf("expected tile/build rotation=1, got tile=%d build=%d", tile.Rotation, tile.Build.Rotation)
	}
}

func TestUnloaderMovesConfiguredItemBetweenAdjacentBlocks(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		430: "unloader",
		431: "vault",
	}
	w.SetModel(model)

	store := placeTestBuilding(t, w, 2, 3, 431, 1, 0)
	placeTestBuilding(t, w, 3, 3, 430, 1, 0)
	placeTestBuilding(t, w, 4, 3, 257, 1, 0)
	store.Build.AddItem(5, 3)
	w.ConfigureUnloader(int32(3*model.Width+3), 5)

	for i := 0; i < 6; i++ {
		w.Step(time.Second / 60)
	}

	if st := w.conveyorStates[int32(3*model.Width+4)]; st == nil || st.Len == 0 {
		t.Fatalf("expected unloader to move configured item into conveyor")
	}
	if store.Build.ItemAmount(5) >= 3 {
		t.Fatalf("expected unloader to remove item from source storage")
	}
}

func TestUnloaderPrefersOnlyGiveSourceOverFactoryInputBuffer(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		419: "item-void",
		430: "unloader",
		451: "silicon-smelter",
		100: "ground-factory",
	}
	model.UnitNames = map[int16]string{
		7: "dagger",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 3, 4, 419, 1, 0)
	placeTestBuilding(t, w, 3, 3, 430, 1, 0)
	smelter := placeTestBuilding(t, w, 4, 3, 451, 1, 0)
	factory := placeTestBuilding(t, w, 3, 1, 100, 1, 0)

	smelter.Build.AddItem(siliconItemID, 1)
	factory.Build.AddItem(siliconItemID, 5)
	unloaderPos := int32(3*model.Width + 3)
	w.ConfigureUnloader(unloaderPos, siliconItemID)

	neighbors := w.dumpProximityLocked(unloaderPos)
	if len(neighbors) != 3 {
		t.Fatalf("expected unloader to see 3 neighbors, got %d", len(neighbors))
	}
	item, ok := w.unloaderTargetItemLocked(unloaderPos, neighbors)
	if !ok || item != siliconItemID {
		t.Fatalf("expected unloader target item %d, got item=%d ok=%v", siliconItemID, item, ok)
	}
	fromPos, toPos, ok := w.unloaderTransferPairPreviewLocked(unloaderPos, neighbors, siliconItemID)
	if !ok {
		t.Fatalf("expected unloader transfer pair for silicon, neighbors=%v", neighbors)
	}
	if fromPos != int32(3*model.Width+4) {
		t.Fatalf("expected silicon-smelter to be chosen as source, got fromPos=%d", fromPos)
	}
	if toPos != int32(4*model.Width+3) {
		t.Fatalf("expected north item-void to be chosen as target, got toPos=%d", toPos)
	}

	w.transportAccum[unloaderPos] = float32(60.0 / 11.0)
	unloaderTile, err := w.Model().TileAt(3, 3)
	if err != nil || unloaderTile == nil {
		t.Fatalf("unloader tile lookup failed: %v", err)
	}
	w.stepUnloaderLocked(unloaderPos, unloaderTile, 0)
	foundBlockItemSync := false
	for _, ev := range w.DrainEntityEvents() {
		if ev.Kind == EntityEventBlockItemSync && ev.BuildPos == protocol.PackPoint2(4, 3) {
			foundBlockItemSync = true
		}
	}

	if got := smelter.Build.ItemAmount(siliconItemID); got != 0 {
		t.Fatalf("expected unloader to prefer the only-give silicon-smelter output first, smelter silicon=%d", got)
	}
	if got := factory.Build.ItemAmount(siliconItemID); got != 5 {
		t.Fatalf("expected unit factory silicon buffer to stay intact, got=%d", got)
	}
	if !foundBlockItemSync {
		t.Fatalf("expected unloader source inventory change to emit a block item sync event")
	}
}

func TestDumpProximityCacheRebuildsAfterNeighborRemoval(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(10, 10)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		500: "container",
	}
	w.SetModel(model)

	src := placeTestBuilding(t, w, 4, 4, 257, 1, 0)
	dst := placeTestBuilding(t, w, 5, 4, 500, 1, 0)
	srcPos := int32(src.Y*model.Width + src.X)

	neighbors := w.dumpProximityLocked(srcPos)
	if len(neighbors) != 1 || neighbors[0] != int32(dst.Y*model.Width+dst.X) {
		t.Fatalf("expected initial dump proximity to contain destination, got %v", neighbors)
	}

	dst.Block = 0
	dst.Team = 0
	dst.Build = nil
	w.rebuildBlockOccupancyLocked()

	neighbors = w.dumpProximityLocked(srcPos)
	if len(neighbors) != 0 {
		t.Fatalf("expected dump proximity cache to rebuild after neighbor removal, got %v", neighbors)
	}
}

func TestUnloaderRespectsAllowCoreUnloadersRuleForLinkedCoreStorage(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		430: "unloader",
		431: "container",
		432: "core-shard",
	}
	w.SetModel(model)

	core := placeTestBuilding(t, w, 3, 4, 432, 1, 0)
	placeTestBuilding(t, w, 5, 4, 431, 1, 0)
	out := placeTestBuilding(t, w, 8, 4, 257, 1, 0)
	placeTestBuilding(t, w, 7, 4, 430, 1, 0)

	storePos := int32(4*model.Width + 5)
	if linked, ok := w.storageLinkedCore[storePos]; !ok || linked != int32(4*model.Width+3) {
		t.Fatalf("expected container to be linked to the nearby core, linked=%d ok=%v", linked, ok)
	}

	core.Build.AddItem(copperItemID, 1)
	rules := w.GetRulesManager().Get()
	rules.AllowCoreUnloaders = false

	unloaderPos := int32(4*model.Width + 7)
	w.ConfigureUnloader(unloaderPos, copperItemID)
	w.transportAccum[unloaderPos] = float32(60.0 / 11.0)

	unloaderTile, err := w.Model().TileAt(7, 4)
	if err != nil || unloaderTile == nil {
		t.Fatalf("unloader tile lookup failed: %v", err)
	}

	w.stepUnloaderLocked(unloaderPos, unloaderTile, 0)
	if got := totalBuildingItems(out.Build); got != 0 {
		t.Fatalf("expected unloader to respect allowCoreUnloaders=false, moved=%d", got)
	}
	if got := core.Build.ItemAmount(copperItemID); got != 1 {
		t.Fatalf("expected core inventory to remain intact when core unloaders are disabled, got=%d", got)
	}

	rules.AllowCoreUnloaders = true
	w.stepUnloaderLocked(unloaderPos, unloaderTile, 0)
	if got := totalBuildingItems(out.Build); got == 0 {
		t.Fatalf("expected unloader to resume unloading once allowCoreUnloaders is re-enabled")
	}
	if got := core.Build.ItemAmount(copperItemID); got != 0 {
		t.Fatalf("expected core inventory to decrease after re-enabling core unloaders, got=%d", got)
	}
}

func TestUnloaderDoesNotPullFromConveyor(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		430: "unloader",
	}
	w.SetModel(model)

	src := placeTestBuilding(t, w, 2, 3, 257, 1, 0)
	placeTestBuilding(t, w, 3, 3, 430, 1, 0)
	out := placeTestBuilding(t, w, 4, 3, 257, 1, 0)

	src.Build.AddItem(copperItemID, 1)
	unloaderPos := int32(3*model.Width + 3)
	w.ConfigureUnloader(unloaderPos, copperItemID)
	w.transportAccum[unloaderPos] = float32(60.0 / 11.0)

	unloaderTile, err := w.Model().TileAt(3, 3)
	if err != nil || unloaderTile == nil {
		t.Fatalf("unloader tile lookup failed: %v", err)
	}

	w.stepUnloaderLocked(unloaderPos, unloaderTile, 0)
	if got := totalBuildingItems(out.Build); got != 0 {
		t.Fatalf("expected unloader not to pull from conveyor, moved=%d", got)
	}
	if got := src.Build.ItemAmount(copperItemID); got != 1 {
		t.Fatalf("expected conveyor source inventory to remain untouched, got=%d", got)
	}
}

func TestUnloaderDoesNotPullFromPlastaniumLoadingDock(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(10, 8)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		430: "unloader",
		447: "plastanium-conveyor",
	}
	w.SetModel(model)

	src := placeTestBuilding(t, w, 3, 3, 447, 1, 0)
	placeTestBuilding(t, w, 4, 3, 447, 1, 0)
	placeTestBuilding(t, w, 3, 2, 430, 1, 0)
	out := placeTestBuilding(t, w, 4, 2, 257, 1, 0)
	src.Build.AddItem(copperItemID, 1)

	unloaderPos := int32(2*model.Width + 3)
	w.ConfigureUnloader(unloaderPos, copperItemID)
	w.transportAccum[unloaderPos] = float32(60.0 / 11.0)

	unloaderTile, err := w.Model().TileAt(3, 2)
	if err != nil || unloaderTile == nil {
		t.Fatalf("unloader tile lookup failed: %v", err)
	}

	w.stepUnloaderLocked(unloaderPos, unloaderTile, 0)
	if got := totalBuildingItems(out.Build); got != 0 {
		t.Fatalf("expected unloader not to pull from plastanium loading dock, moved=%d", got)
	}
	if got := src.Build.ItemAmount(copperItemID); got != 1 {
		t.Fatalf("expected plastanium loading dock inventory to remain untouched, got=%d", got)
	}
}

func TestUnloaderCanPullFromSurgeConveyorUnloadState(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(10, 8)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		430: "unloader",
		448: "surge-conveyor",
	}
	w.SetModel(model)

	src := placeTestBuilding(t, w, 3, 3, 448, 1, 0)
	placeTestBuilding(t, w, 3, 2, 430, 1, 0)
	out := placeTestBuilding(t, w, 4, 2, 257, 1, 0)
	src.Build.AddItem(copperItemID, 1)

	unloaderPos := int32(2*model.Width + 3)
	w.ConfigureUnloader(unloaderPos, copperItemID)
	w.transportAccum[unloaderPos] = float32(60.0 / 11.0)

	unloaderTile, err := w.Model().TileAt(3, 2)
	if err != nil || unloaderTile == nil {
		t.Fatalf("unloader tile lookup failed: %v", err)
	}

	w.stepUnloaderLocked(unloaderPos, unloaderTile, 0)
	if got := totalBuildingItems(out.Build); got == 0 {
		t.Fatalf("expected unloader to pull from surge conveyor when it is not a loading dock")
	}
	if got := src.Build.ItemAmount(copperItemID); got != 0 {
		t.Fatalf("expected surge conveyor source inventory to decrease, got=%d", got)
	}
}

func TestPlastaniumLoadingDockAcceptsSideInput(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(10, 8)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		447: "plastanium-conveyor",
	}
	w.SetModel(model)

	source := placeTestBuilding(t, w, 3, 2, 257, 1, 0)
	placeTestBuilding(t, w, 3, 3, 447, 1, 0)
	placeTestBuilding(t, w, 4, 3, 447, 1, 0)

	fromPos := int32(2*model.Width + 3)
	toPos := int32(3*model.Width + 3)
	if !w.stackConveyorAcceptsItemLocked(fromPos, toPos, copperItemID) {
		t.Fatalf("expected plastanium loading dock to accept side input from conveyor at %v", source)
	}
}

func TestSurgeConveyorRejectsSideInputWhenNotLoadingDock(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(10, 8)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		448: "surge-conveyor",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 3, 2, 257, 1, 0)
	placeTestBuilding(t, w, 3, 3, 448, 1, 0)

	fromPos := int32(2*model.Width + 3)
	toPos := int32(3*model.Width + 3)
	if w.stackConveyorAcceptsItemLocked(fromPos, toPos, copperItemID) {
		t.Fatal("expected surge conveyor to reject side input when it is not in loading-dock state")
	}
}

func TestMassDriverTransfersBatchToLinkedTarget(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 12)
	model.BlockNames = map[int16]string{
		421: "battery",
		422: "power-node",
		432: "mass-driver",
	}
	w.SetModel(model)

	src := placeTestBuilding(t, w, 4, 4, 432, 1, 0)
	dst := placeTestBuilding(t, w, 14, 4, 432, 1, 0)
	placeTestBuilding(t, w, 4, 8, 421, 1, 0)
	placeTestBuilding(t, w, 14, 8, 421, 1, 0)
	placeTestBuilding(t, w, 4, 6, 422, 1, 0)
	placeTestBuilding(t, w, 14, 6, 422, 1, 0)
	w.powerStorageState[int32(8*model.Width+4)] = 4000
	w.powerStorageState[int32(8*model.Width+14)] = 4000
	linkPowerNode(t, w, 4, 6, protocol.Point2{X: 0, Y: -2}, protocol.Point2{X: 0, Y: 2})
	linkPowerNode(t, w, 14, 6, protocol.Point2{X: 0, Y: -2}, protocol.Point2{X: 0, Y: 2})
	w.ConfigureBuilding(int32(4*model.Width+4), protocol.Point2{X: 10, Y: 0})
	src.Build.AddItem(5, 20)

	for i := 0; i < 260; i++ {
		w.Step(time.Second / 60)
	}

	if totalBuildingItems(dst.Build) == 0 {
		t.Fatalf("expected mass driver to transfer batch to linked target")
	}
	if totalBuildingItems(src.Build) >= 20 {
		t.Fatalf("expected mass driver source inventory to decrease")
	}
}

func TestDuctMovesItemForward(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		412: "item-source",
		440: "duct",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 1, 3, 412, 1, 0)
	placeTestBuilding(t, w, 2, 3, 440, 1, 0)
	placeTestBuilding(t, w, 3, 3, 440, 1, 0)
	out := placeTestBuilding(t, w, 4, 3, 257, 1, 0)
	w.ConfigureItemSource(int32(3*model.Width+1), 5)

	for i := 0; i < 40; i++ {
		w.Step(time.Second / 60)
	}

	if totalBuildingItems(out.Build) == 0 {
		t.Fatalf("expected duct chain to move item into output")
	}
}

func TestDuctBridgeTransfersBetweenLinks(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(10, 8)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		440: "duct",
		445: "duct-bridge",
	}
	w.SetModel(model)

	in := placeTestBuilding(t, w, 1, 3, 440, 1, 0)
	placeTestBuilding(t, w, 2, 3, 445, 1, 0)
	placeTestBuilding(t, w, 5, 3, 445, 1, 0)
	out := placeTestBuilding(t, w, 6, 3, 257, 1, 0)
	in.Build.AddItem(5, 1)

	for i := 0; i < 30; i++ {
		w.Step(time.Second / 60)
	}

	if totalBuildingItems(out.Build) == 0 {
		t.Fatalf("expected duct bridge pair to deliver item forward")
	}
}

func TestOverflowDuctFlipsSideFallbackAfterFrontTransfer(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(10, 8)
	model.BlockNames = map[int16]string{
		431: "container",
		445: "overflow-duct",
	}
	w.SetModel(model)

	duct := placeTestBuilding(t, w, 3, 3, 445, 1, 0)
	front := placeTestBuilding(t, w, 4, 3, 431, 1, 0)
	left := placeTestBuilding(t, w, 3, 2, 431, 1, 0)
	right := placeTestBuilding(t, w, 3, 4, 431, 1, 0)
	duct.Build.AddItem(copperItemID, 2)

	for i := 0; i < 2; i++ {
		w.Step(time.Second / 60)
	}
	if got := front.Build.ItemAmount(copperItemID); got != 1 {
		t.Fatalf("expected first overflow-duct transfer to go forward, got=%d", got)
	}

	front.Team = 2
	front.Build.Team = 2

	for i := 0; i < 2; i++ {
		w.Step(time.Second / 60)
	}
	if got := right.Build.ItemAmount(copperItemID); got != 1 {
		t.Fatalf("expected fallback side order to flip after forward transfer, right=%d", got)
	}
	if got := left.Build.ItemAmount(copperItemID); got != 0 {
		t.Fatalf("expected flipped overflow-duct fallback not to send second item left, left=%d", got)
	}
}

func TestUnderflowDuctFlipsSideFallbackAfterSingleSideTransfer(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(10, 8)
	model.BlockNames = map[int16]string{
		431: "container",
		446: "underflow-duct",
	}
	w.SetModel(model)

	duct := placeTestBuilding(t, w, 3, 3, 446, 1, 0)
	left := placeTestBuilding(t, w, 3, 2, 431, 1, 0)
	right := placeTestBuilding(t, w, 3, 4, 431, 2, 0)
	duct.Build.AddItem(copperItemID, 2)

	for i := 0; i < 2; i++ {
		w.Step(time.Second / 60)
	}
	if got := left.Build.ItemAmount(copperItemID); got != 1 {
		t.Fatalf("expected first underflow-duct transfer to use the only open side, left=%d", got)
	}

	right.Team = 1
	right.Build.Team = 1

	for i := 0; i < 2; i++ {
		w.Step(time.Second / 60)
	}
	if got := right.Build.ItemAmount(copperItemID); got != 1 {
		t.Fatalf("expected side fallback order to flip after single-side transfer, right=%d", got)
	}
	if got := left.Build.ItemAmount(copperItemID); got != 1 {
		t.Fatalf("expected second underflow-duct transfer not to reuse left side, left=%d", got)
	}
}

func TestDirectionalDuctUnloaderMovesConfiguredItem(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		431: "vault",
		446: "duct-unloader",
	}
	w.SetModel(model)

	store := placeTestBuilding(t, w, 1, 3, 431, 1, 0)
	placeTestBuilding(t, w, 2, 3, 446, 1, 0)
	out := placeTestBuilding(t, w, 3, 3, 257, 1, 0)
	store.Build.AddItem(5, 3)
	w.ConfigureUnloader(int32(3*model.Width+2), 5)

	for i := 0; i < 20; i++ {
		w.Step(time.Second / 60)
	}

	if totalBuildingItems(out.Build) == 0 {
		t.Fatalf("expected duct-unloader to push configured item forward")
	}
	if store.Build.ItemAmount(5) >= 3 {
		t.Fatalf("expected duct-unloader to remove item from rear storage")
	}
}

func TestDirectionalDuctUnloaderConsumesCooldownAfterBlockedAttempt(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		431: "vault",
		446: "duct-unloader",
	}
	w.SetModel(model)

	store := placeTestBuilding(t, w, 1, 3, 431, 1, 0)
	placeTestBuilding(t, w, 2, 3, 446, 1, 0)
	out := placeTestBuilding(t, w, 3, 3, 257, 2, 0)
	store.Build.AddItem(5, 3)

	unloaderPos := int32(3*model.Width + 2)
	w.ConfigureUnloader(unloaderPos, 5)
	w.transportAccum[unloaderPos] = 4

	unloaderTile, err := w.Model().TileAt(2, 3)
	if err != nil || unloaderTile == nil {
		t.Fatalf("duct-unloader tile lookup failed: %v", err)
	}

	// First attempt is blocked by the front block belonging to another team.
	w.stepDirectionalUnloaderLocked(unloaderPos, unloaderTile, 4, 0)
	if got := totalBuildingItems(out.Build); got != 0 {
		t.Fatalf("expected blocked duct-unloader attempt to move no items, got=%d", got)
	}

	// Open the front side. Vanilla still waits for the next full cooldown window
	// because the blocked attempt consumed its timer.
	out.Team = 1
	out.Build.Team = 1
	w.stepDirectionalUnloaderLocked(unloaderPos, unloaderTile, 4, float32(1.0/60.0))
	if got := totalBuildingItems(out.Build); got != 0 {
		t.Fatalf("expected duct-unloader to wait for a fresh cooldown after a blocked attempt, got=%d", got)
	}

	for i := 0; i < 3; i++ {
		w.stepDirectionalUnloaderLocked(unloaderPos, unloaderTile, 4, float32(1.0/60.0))
	}
	if got := totalBuildingItems(out.Build); got == 0 {
		t.Fatalf("expected duct-unloader to move item after the next full cooldown")
	}
}

func TestDirectionalDuctUnloaderBlocksCoreLinkedStorage(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		431: "container",
		432: "core-shard",
		446: "duct-unloader",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 3, 4, 432, 1, 0)
	store := placeTestBuilding(t, w, 5, 4, 431, 1, 0)
	out := placeTestBuilding(t, w, 8, 4, 257, 1, 0)
	placeTestBuilding(t, w, 7, 4, 446, 1, 0)

	storePos := int32(4*model.Width + 5)
	if _, ok := w.storageLinkedCore[storePos]; !ok {
		t.Fatal("expected container to be linked to the nearby core")
	}

	// Mirror the official linkedCore guard explicitly: even if stale map/runtime
	// data leaves an item stack on the linked container, the duct-unloader must
	// not extract from it.
	store.Build.Items = []ItemStack{{Item: copperItemID, Amount: 1}}

	unloaderPos := int32(4*model.Width + 7)
	w.ConfigureUnloader(unloaderPos, copperItemID)
	w.transportAccum[unloaderPos] = 4

	unloaderTile, err := w.Model().TileAt(7, 4)
	if err != nil || unloaderTile == nil {
		t.Fatalf("duct-unloader tile lookup failed: %v", err)
	}

	w.stepDirectionalUnloaderLocked(unloaderPos, unloaderTile, 4, 0)
	if got := totalBuildingItems(out.Build); got != 0 {
		t.Fatalf("expected duct-unloader to reject linked-core storage, moved=%d", got)
	}
	if got := store.Build.ItemAmount(copperItemID); got != 1 {
		t.Fatalf("expected linked storage inventory to remain untouched, got=%d", got)
	}
}

func TestPlastaniumConveyorTransfersWholeStack(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(10, 8)
	model.BlockNames = map[int16]string{
		447: "plastanium-conveyor",
	}
	w.SetModel(model)

	src := placeTestBuilding(t, w, 2, 3, 447, 1, 0)
	dst := placeTestBuilding(t, w, 3, 3, 447, 1, 0)
	src.Build.AddItem(5, 10)

	for i := 0; i < 80; i++ {
		w.Step(time.Second / 60)
	}

	if totalBuildingItems(dst.Build) == 0 {
		t.Fatalf("expected plastanium conveyor to transfer stacked items")
	}
	if totalBuildingItems(src.Build) != 0 {
		t.Fatalf("expected source plastanium conveyor to hand off full stack")
	}
}

func TestSurgeRouterUnloadsBatchToForwardSide(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		421: "battery",
		422: "power-node",
		448: "surge-router",
	}
	w.SetModel(model)

	router := placeTestBuilding(t, w, 2, 3, 448, 1, 0)
	out := placeTestBuilding(t, w, 3, 3, 257, 1, 0)
	placeTestBuilding(t, w, 2, 6, 421, 1, 0)
	placeTestBuilding(t, w, 2, 5, 422, 1, 0)
	w.powerStorageState[int32(6*model.Width+2)] = 4000
	linkPowerNode(t, w, 2, 5, protocol.Point2{X: 0, Y: -2}, protocol.Point2{X: 0, Y: 1})
	router.Build.AddItem(5, 10)
	w.ConfigureSorter(int32(3*model.Width+2), 5)

	for i := 0; i < 80; i++ {
		w.Step(time.Second / 60)
	}

	if totalBuildingItems(out.Build) == 0 {
		t.Fatalf("expected surge router to unload batch to forward output")
	}
}

func TestLiquidRouterMovesLiquidForward(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		450: "liquid-router",
		315: "thorium-reactor",
	}
	w.SetModel(model)

	router := placeTestBuilding(t, w, 2, 3, 450, 1, 0)
	store := placeTestBuilding(t, w, 3, 3, 315, 1, 0)
	router.Build.AddLiquid(1, 10)

	for i := 0; i < 30; i++ {
		w.Step(time.Second / 60)
	}

	if totalBuildingLiquids(store.Build) <= 0 {
		t.Fatalf("expected liquid router to move liquid into container")
	}
}

func TestLiquidBridgeTransfersLinkedLiquid(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(10, 8)
	model.BlockNames = map[int16]string{
		452: "bridge-conduit",
		315: "thorium-reactor",
	}
	w.SetModel(model)

	bridge := placeTestBuilding(t, w, 2, 3, 452, 1, 0)
	placeTestBuilding(t, w, 5, 3, 452, 1, 0)
	tank := placeTestBuilding(t, w, 6, 3, 315, 1, 0)
	bridge.Build.AddLiquid(1, 20)
	w.ConfigureBuilding(int32(3*model.Width+2), protocol.Point2{X: 3, Y: 0})

	for i := 0; i < 40; i++ {
		w.Step(time.Second / 60)
	}

	if totalBuildingLiquids(tank.Build) <= 0 {
		t.Fatalf("expected linked liquid bridge to move liquid into tank")
	}
}

func TestConduitMovesLiquidForward(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		315: "thorium-reactor",
		454: "conduit",
	}
	w.SetModel(model)

	src := placeTestBuilding(t, w, 1, 3, 454, 1, 0)
	placeTestBuilding(t, w, 2, 3, 454, 1, 0)
	dst := placeTestBuilding(t, w, 3, 3, 315, 1, 0)
	src.Build.AddLiquid(1, 20)

	for i := 0; i < 30; i++ {
		w.Step(time.Second / 60)
	}

	if totalBuildingLiquids(dst.Build) <= 0 {
		t.Fatalf("expected conduit chain to move liquid into destination")
	}
}

func TestPlatedConduitAcceptsRearLiquid(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		315: "thorium-reactor",
		455: "plated-conduit",
	}
	w.SetModel(model)

	src := placeTestBuilding(t, w, 1, 3, 455, 1, 0)
	dst := placeTestBuilding(t, w, 2, 3, 315, 1, 0)
	src.Build.AddLiquid(1, 30)

	for i := 0; i < 30; i++ {
		w.Step(time.Second / 60)
	}

	if totalBuildingLiquids(dst.Build) <= 0 {
		t.Fatalf("expected plated conduit to move liquid forward")
	}
}

func TestPayloadConveyorTransfersBlockPayloadForward(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(20, 12)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		700: "payload-conveyor",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 4, 4, 700, 1, 0)
	placeTestBuilding(t, w, 7, 4, 700, 1, 0)

	sourcePos := setTestPayload(t, w, 4, 4, &payloadData{Kind: payloadKindBlock, BlockID: 257})
	targetPos := int32(4*model.Width + 7)

	for i := 0; i < 60; i++ {
		w.Step(time.Second / 60)
	}

	if w.payloadStateLocked(sourcePos).Payload != nil {
		t.Fatalf("expected source payload conveyor to hand off payload")
	}
	if got := w.payloadStateLocked(targetPos).Payload; got == nil || got.BlockID != 257 {
		t.Fatalf("expected target payload conveyor to receive block payload, got=%+v", got)
	}
}

func TestPayloadRouterRoutesMatchingBlockForward(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 16)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		700: "payload-conveyor",
		701: "payload-router",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 8, 8, 701, 1, 1)
	placeTestBuilding(t, w, 11, 8, 700, 1, 0)
	placeTestBuilding(t, w, 8, 11, 700, 1, 1)

	routerPos := int32(8*model.Width + 8)
	w.ConfigureBuildingPacked(protocol.PackPoint2(8, 8), protocol.BlockRef{BlkID: 257})
	cfg, ok := w.BuildingConfigPacked(protocol.PackPoint2(8, 8))
	if !ok {
		t.Fatalf("expected payload router config to persist")
	}
	filter, ok := cfg.(protocol.Content)
	if !ok || filter.ContentType() != protocol.ContentBlock || filter.ID() != 257 {
		t.Fatalf("expected payload router block filter config, got=%T %+v", cfg, cfg)
	}

	st := w.payloadStateLocked(routerPos)
	st.Payload = &payloadData{Kind: payloadKindBlock, BlockID: 257}
	st.RecDir = 0

	for i := 0; i < 60; i++ {
		w.Step(time.Second / 60)
	}

	forwardPos := int32(8*model.Width + 11)
	sidePos := int32(11*model.Width + 8)
	if got := w.payloadStateLocked(forwardPos).Payload; got == nil || got.BlockID != 257 {
		t.Fatalf("expected matching payload to route forward, got=%+v", got)
	}
	if got := w.payloadStateLocked(sidePos).Payload; got != nil {
		t.Fatalf("expected side target to stay empty, got=%+v", got)
	}
}

func TestPayloadMassDriverTransfersPayload(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(32, 16)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		421: "battery",
		422: "power-node",
		702: "payload-mass-driver",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 6, 8, 702, 1, 0)
	placeTestBuilding(t, w, 16, 8, 702, 1, 0)
	placeTestBuilding(t, w, 6, 14, 421, 1, 0)
	placeTestBuilding(t, w, 16, 14, 421, 1, 0)
	placeTestBuilding(t, w, 6, 11, 422, 1, 0)
	placeTestBuilding(t, w, 16, 11, 422, 1, 0)
	w.powerStorageState[int32(14*model.Width+6)] = 4000
	w.powerStorageState[int32(14*model.Width+16)] = 4000
	linkPowerNode(t, w, 6, 11, protocol.Point2{X: 0, Y: -3}, protocol.Point2{X: 0, Y: 3})
	linkPowerNode(t, w, 16, 11, protocol.Point2{X: 0, Y: -3}, protocol.Point2{X: 0, Y: 3})

	sourcePos := int32(8*model.Width + 6)
	targetPos := int32(8*model.Width + 16)
	w.ConfigureBuilding(sourcePos, protocol.Point2{X: 10, Y: 0})
	setTestPayload(t, w, 6, 8, &payloadData{Kind: payloadKindBlock, BlockID: 257})

	for i := 0; i < 260; i++ {
		w.Step(time.Second / 60)
	}

	if w.payloadStateLocked(sourcePos).Payload != nil {
		t.Fatalf("expected source payload mass driver to launch payload")
	}
	if got := w.payloadStateLocked(targetPos).Payload; got == nil || got.BlockID != 257 {
		t.Fatalf("expected target payload mass driver to receive payload, got=%+v", got)
	}
}

func TestPayloadLoaderAndUnloaderTransferItems(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 16)
	model.BlockNames = map[int16]string{
		315: "thorium-reactor",
		703: "payload-loader",
		704: "payload-unloader",
	}
	w.SetModel(model)

	loader := placeTestBuilding(t, w, 8, 8, 703, 1, 0)
	unloader := placeTestBuilding(t, w, 11, 8, 704, 1, 0)
	loader.Build.AddItem(5, 6)
	setTestPayload(t, w, 8, 8, &payloadData{Kind: payloadKindBlock, BlockID: 315})

	for i := 0; i < 180; i++ {
		w.Step(time.Second / 60)
	}

	if unloader.Build.ItemAmount(5) == 0 {
		t.Fatalf("expected payload loader/unloader pair to transfer items into unloader inventory")
	}
}

func TestControlSelectPayloadUnitPackedAcceptsReconstructor(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.BlockNames = map[int16]string{
		706: "additive-reconstructor",
	}
	model.UnitNames = map[int16]string{
		7: "dagger",
		8: "mace",
	}
	w.SetModel(model)
	tile := placeTestBuilding(t, w, 6, 6, 706, 1, 0)
	buildPacked := protocol.PackPoint2(int32(tile.X), int32(tile.Y))
	buildPos := int32(tile.Y*w.Model().Width + tile.X)
	unitID := w.Model().AddEntity(RawEntity{
		ID:        81,
		TypeID:    7,
		Team:      1,
		X:         6*8 + 4,
		Y:         6*8 + 4,
		Health:    90,
		MaxHealth: 90,
	}).ID

	if !w.ControlSelectPayloadUnitPacked(buildPacked, unitID) {
		t.Fatal("expected reconstructor control-select to accept upgradeable unit payload")
	}

	payload := w.payloadStateLocked(buildPos).Payload
	if payload == nil || payload.Kind != payloadKindUnit || payload.UnitTypeID != 7 {
		t.Fatalf("expected reconstructor to receive dagger payload, got %+v", payload)
	}
}

func TestEnterUnitPayloadPackedRejectsSpawnedByCoreOnPayloadDeconstructor(t *testing.T) {
	unit := RawEntity{
		ID:            82,
		TypeID:        7,
		Team:          1,
		X:             8*8 + 4,
		Y:             8*8 + 4,
		Health:        90,
		MaxHealth:     90,
		SpawnedByCore: true,
	}
	w, buildPacked, buildPos, unitID := newPayloadBuildingWorld(t, 705, "small-deconstructor", 0, unit)

	if w.EnterUnitPayloadPacked(buildPacked, unitID) {
		t.Fatal("expected unitEnteredPayload path to reject spawnedByCore units on payload deconstructor")
	}
	if payload := w.payloadStateLocked(buildPos).Payload; payload != nil {
		t.Fatalf("expected small deconstructor payload to stay empty, got %+v", payload)
	}
	found := false
	for _, ent := range w.Model().Entities {
		if ent.ID == unitID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected rejected unit %d to remain in world", unitID)
	}
}

func TestPayloadVoidConsumesPayload(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(20, 20)
	model.BlockNames = map[int16]string{
		705: "payload-void",
	}
	model.UnitNames = map[int16]string{
		7: "dagger",
	}
	w.SetModel(model)
	tile := placeTestBuilding(t, w, 8, 8, 705, 1, 0)
	pos := int32(tile.Y*w.Model().Width + tile.X)
	payload := w.unitPayloadFromEntityLocked(w.newProducedUnitEntityLocked(7, 1, 0, 0, 0))
	if payload == nil {
		t.Fatal("expected dagger payload for payload-void test")
	}
	setTestPayload(t, w, 8, 8, payload)

	stepForSeconds(w, 1)

	if got := w.payloadStateLocked(pos).Payload; got != nil {
		t.Fatalf("expected payload-void to incinerate payload, got %+v", got)
	}
	if len(tile.Build.Payload) != 0 {
		t.Fatalf("expected payload-void build payload bytes to clear, got len=%d", len(tile.Build.Payload))
	}
}

func TestPayloadDeconstructorProcessesUnitPayload(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(20, 20)
	model.BlockNames = map[int16]string{
		421: "battery",
		422: "power-node",
		705: "small-deconstructor",
	}
	model.UnitNames = map[int16]string{
		7: "dagger",
	}
	w.SetModel(model)
	tile := placeTestBuilding(t, w, 8, 8, 705, 1, 0)
	pos := int32(tile.Y*w.Model().Width + tile.X)
	placeTestBuilding(t, w, 5, 8, 421, 1, 0)
	placeTestBuilding(t, w, 6, 8, 422, 1, 0)
	w.powerStorageState[int32(8*model.Width+5)] = 4000
	linkPowerNode(t, w, 6, 8, protocol.Point2{X: -1, Y: 0}, protocol.Point2{X: 2, Y: 0})

	payload := w.unitPayloadFromEntityLocked(w.newProducedUnitEntityLocked(7, 1, 0, 0, 0))
	if payload == nil {
		t.Fatal("expected dagger payload for deconstructor test")
	}
	setTestPayload(t, w, 8, 8, payload)

	stepForSeconds(w, 1)

	if got := tile.Build.ItemAmount(siliconItemID); got != 10 {
		t.Fatalf("expected deconstructor to recover 10 silicon, got %d", got)
	}
	if got := tile.Build.ItemAmount(leadItemID); got != 10 {
		t.Fatalf("expected deconstructor to recover 10 lead, got %d", got)
	}
	if got := w.payloadStateLocked(pos).Payload; got != nil {
		t.Fatalf("expected deconstructor input payload to clear, got %+v", got)
	}
	if st, ok := w.payloadDeconstructorStates[pos]; ok && st != nil && st.Deconstructing != nil {
		t.Fatalf("expected deconstructor runtime payload to finish, got %+v", st.Deconstructing)
	}
}

func TestReconstructorUpgradesUnitPayload(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		418: "router",
		421: "battery",
		422: "power-node",
		706: "additive-reconstructor",
	}
	model.UnitNames = map[int16]string{
		7: "dagger",
		8: "mace",
	}
	w.SetModel(model)
	tile := placeTestBuilding(t, w, 10, 10, 706, 1, 0)
	pos := int32(tile.Y*w.Model().Width + tile.X)
	placeTestBuilding(t, w, 12, 10, 418, 1, 0)
	placeTestBuilding(t, w, 7, 10, 421, 1, 0)
	placeTestBuilding(t, w, 8, 10, 422, 1, 0)
	w.powerStorageState[int32(10*model.Width+7)] = 4000
	linkPowerNode(t, w, 8, 10, protocol.Point2{X: -1, Y: 0}, protocol.Point2{X: 2, Y: 0})
	tile.Build.AddItem(siliconItemID, 40)
	tile.Build.AddItem(graphiteItemID, 40)

	payload := w.unitPayloadFromEntityLocked(w.newProducedUnitEntityLocked(7, 1, 0, 0, 0))
	if payload == nil {
		t.Fatal("expected dagger payload for reconstructor test")
	}
	setTestPayload(t, w, 10, 10, payload)

	stepForSeconds(w, 11)

	current := w.payloadStateLocked(pos).Payload
	if current == nil || current.Kind != payloadKindUnit || current.UnitTypeID != 8 {
		t.Fatalf("expected additive reconstructor to upgrade dagger into mace payload, got %+v", current)
	}
	if got := tile.Build.ItemAmount(siliconItemID); got != 0 {
		t.Fatalf("expected reconstructor to consume silicon on completion, got %d", got)
	}
	if got := tile.Build.ItemAmount(graphiteItemID); got != 0 {
		t.Fatalf("expected reconstructor to consume graphite on completion, got %d", got)
	}
}

func TestReconstructorDumpedUnitCarriesCommandState(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(20, 20)
	model.BlockNames = map[int16]string{
		339: "core-shard",
		706: "additive-reconstructor",
	}
	model.UnitNames = map[int16]string{
		8: "mace",
	}
	w.SetModel(model)
	placeTestBuilding(t, w, 2, 2, 339, 1, 0)
	tile := placeTestBuilding(t, w, 8, 8, 706, 1, 0)
	pos := int32(tile.Y*w.Model().Width + tile.X)
	w.reconstructorStates[pos] = reconstructorState{
		CommandPos: &protocol.Vec2{X: 144, Y: 80},
		Command:    &protocol.UnitCommand{ID: 2, Name: "repair"},
	}
	payload := w.unitPayloadFromEntityLocked(w.newProducedUnitEntityLocked(8, 1, 0, 0, 0))
	if payload == nil {
		t.Fatal("expected upgraded unit payload for reconstructor dump test")
	}
	w.payloadStates[pos] = &payloadRuntimeState{Payload: payload}
	w.syncPayloadTileLocked(tile, payload)

	if !w.dumpUnitPayloadFromTileLocked(pos, tile) {
		t.Fatal("expected reconstructor payload to dump into a world unit")
	}

	found := false
	for _, ent := range w.model.Entities {
		if ent.TypeID != 8 || ent.Team != 1 {
			continue
		}
		found = true
		if ent.CommandID != 2 {
			t.Fatalf("expected dumped reconstructor unit command id 2, got %d", ent.CommandID)
		}
		if ent.Behavior != "move" {
			t.Fatalf("expected dumped reconstructor unit behavior move, got %q", ent.Behavior)
		}
		if math.Abs(float64(ent.PatrolAX-144)) > 0.0001 || math.Abs(float64(ent.PatrolAY-80)) > 0.0001 {
			t.Fatalf("expected dumped reconstructor unit target (144,80), got (%f,%f)", ent.PatrolAX, ent.PatrolAY)
		}
	}
	if !found {
		t.Fatal("expected dumped reconstructor unit entity to exist")
	}
}

func TestBlockSyncSnapshotsEncodePayloadProcessorsRuntime(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(28, 20)
	model.BlockNames = map[int16]string{
		705: "payload-void",
		706: "small-deconstructor",
		707: "additive-reconstructor",
	}
	model.UnitNames = map[int16]string{
		7: "dagger",
	}
	w.SetModel(model)
	voidTile := placeTestBuilding(t, w, 6, 8, 705, 1, 0)
	deconTile := placeTestBuilding(t, w, 14, 8, 706, 1, 0)
	reconTile := placeTestBuilding(t, w, 22, 8, 707, 1, 0)
	voidPos := int32(voidTile.Y*w.Model().Width + voidTile.X)
	deconPos := int32(deconTile.Y*w.Model().Width + deconTile.X)
	reconPos := int32(reconTile.Y*w.Model().Width + reconTile.X)

	voidPayload := w.unitPayloadFromEntityLocked(w.newProducedUnitEntityLocked(7, 1, 0, 0, 0))
	deconstructingPayload := w.unitPayloadFromEntityLocked(w.newProducedUnitEntityLocked(7, 1, 0, 0, 0))
	if voidPayload == nil || deconstructingPayload == nil {
		t.Fatal("expected payload processor test payloads")
	}
	setTestPayload(t, w, 6, 8, voidPayload)
	w.payloadDeconstructorStates[deconPos] = &payloadDeconstructorState{
		Deconstructing: deconstructingPayload,
		Accum:          []float32{1.25, 0.5},
		Progress:       0.75,
		PayRotation:    45,
	}
	w.reconstructorStates[reconPos] = reconstructorState{
		Progress:   321,
		CommandPos: &protocol.Vec2{X: 160, Y: 96},
		Command:    &protocol.UnitCommand{ID: 2, Name: "repair"},
	}
	w.rebuildActiveTilesLocked()

	fastSnaps := w.PayloadProcessorBlockSyncSnapshots()
	if len(fastSnaps) != 3 {
		t.Fatalf("expected three fast payload processor snapshots, got %d", len(fastSnaps))
	}

	snaps := w.BlockSyncSnapshots()
	if len(snaps) != 3 {
		t.Fatalf("expected three payload processor snapshots, got %d", len(snaps))
	}
	byPos := make(map[int32]BlockSyncSnapshot, len(snaps))
	for _, snap := range snaps {
		byPos[snap.Pos] = snap
	}

	voidSnap, ok := byPos[protocol.PackPoint2(int32(voidTile.X), int32(voidTile.Y))]
	if !ok {
		t.Fatalf("missing payload-void snapshot at %d", protocol.PackPoint2(int32(voidTile.X), int32(voidTile.Y)))
	}
	_, r := decodeBlockSyncBase(t, voidSnap.Data)
	if _, err := r.ReadFloat32(); err != nil {
		t.Fatalf("read payload-void payVector.x failed: %v", err)
	}
	if _, err := r.ReadFloat32(); err != nil {
		t.Fatalf("read payload-void payVector.y failed: %v", err)
	}
	if _, err := r.ReadFloat32(); err != nil {
		t.Fatalf("read payload-void payRotation failed: %v", err)
	}
	payloadExists, err := r.ReadBool()
	if err != nil {
		t.Fatalf("read payload-void payload exists failed: %v", err)
	}
	if !payloadExists {
		t.Fatal("expected payload-void snapshot to include payload bytes")
	}

	deconSnap, ok := byPos[protocol.PackPoint2(int32(deconTile.X), int32(deconTile.Y))]
	if !ok {
		t.Fatalf("missing payload-deconstructor snapshot at %d", protocol.PackPoint2(int32(deconTile.X), int32(deconTile.Y)))
	}
	_, r = decodeBlockSyncBase(t, deconSnap.Data)
	if _, err := r.ReadFloat32(); err != nil {
		t.Fatalf("read deconstructor payVector.x failed: %v", err)
	}
	if _, err := r.ReadFloat32(); err != nil {
		t.Fatalf("read deconstructor payVector.y failed: %v", err)
	}
	payRotation, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read deconstructor payRotation failed: %v", err)
	}
	if math.Abs(float64(payRotation-45)) > 0.0001 {
		t.Fatalf("expected deconstructor payRotation 45, got %f", payRotation)
	}
	currentExists, err := r.ReadBool()
	if err != nil {
		t.Fatalf("read deconstructor current payload exists failed: %v", err)
	}
	if currentExists {
		t.Fatal("expected deconstructor current payload slot to be empty during deconstruction")
	}
	progress, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read deconstructor progress failed: %v", err)
	}
	if math.Abs(float64(progress-0.75)) > 0.0001 {
		t.Fatalf("expected deconstructor progress 0.75, got %f", progress)
	}
	accumLen, err := r.ReadInt16()
	if err != nil {
		t.Fatalf("read deconstructor accum length failed: %v", err)
	}
	if accumLen != 2 {
		t.Fatalf("expected deconstructor accum length 2, got %d", accumLen)
	}
	acc0, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read deconstructor accum[0] failed: %v", err)
	}
	acc1, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read deconstructor accum[1] failed: %v", err)
	}
	if math.Abs(float64(acc0-1.25)) > 0.0001 || math.Abs(float64(acc1-0.5)) > 0.0001 {
		t.Fatalf("expected deconstructor accum [1.25 0.5], got [%f %f]", acc0, acc1)
	}
	deconstructingExists, err := r.ReadBool()
	if err != nil {
		t.Fatalf("read deconstructor deconstructing payload exists failed: %v", err)
	}
	if !deconstructingExists {
		t.Fatal("expected deconstructor snapshot to include deconstructing payload bytes")
	}

	reconSnap, ok := byPos[protocol.PackPoint2(int32(reconTile.X), int32(reconTile.Y))]
	if !ok {
		t.Fatalf("missing reconstructor snapshot at %d", protocol.PackPoint2(int32(reconTile.X), int32(reconTile.Y)))
	}
	_, r = decodeBlockSyncBase(t, reconSnap.Data)
	if _, err := r.ReadFloat32(); err != nil {
		t.Fatalf("read reconstructor payVector.x failed: %v", err)
	}
	if _, err := r.ReadFloat32(); err != nil {
		t.Fatalf("read reconstructor payVector.y failed: %v", err)
	}
	if _, err := r.ReadFloat32(); err != nil {
		t.Fatalf("read reconstructor payRotation failed: %v", err)
	}
	payloadExists, err = r.ReadBool()
	if err != nil {
		t.Fatalf("read reconstructor payload exists failed: %v", err)
	}
	if payloadExists {
		t.Fatal("expected reconstructor payload slot to be empty in snapshot test")
	}
	progress, err = r.ReadFloat32()
	if err != nil {
		t.Fatalf("read reconstructor progress failed: %v", err)
	}
	if math.Abs(float64(progress-321)) > 0.0001 {
		t.Fatalf("expected reconstructor progress 321, got %f", progress)
	}
	commandPos, err := protocol.ReadVecNullable(r)
	if err != nil {
		t.Fatalf("read reconstructor commandPos failed: %v", err)
	}
	command, err := protocol.ReadCommand(r, nil)
	if err != nil {
		t.Fatalf("read reconstructor command failed: %v", err)
	}
	if commandPos == nil || math.Abs(float64(commandPos.X-160)) > 0.0001 || math.Abs(float64(commandPos.Y-96)) > 0.0001 {
		t.Fatalf("expected reconstructor commandPos (160,96), got %+v", commandPos)
	}
	if command == nil || command.ID != 2 {
		t.Fatalf("expected reconstructor command id 2, got %+v", command)
	}
	if rem := r.Remaining(); rem != 0 {
		t.Fatalf("expected reconstructor sync payload to be fully consumed, remaining=%d", rem)
	}

	_ = voidPos
}

