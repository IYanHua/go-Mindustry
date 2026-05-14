package world

import (
	"testing"
	"time"

	"github.com/IYanHua/mdt-server/internal/protocol"
)

func TestNuclearReactorOverheatsFromItemSource(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		315: "thorium-reactor",
		412: "item-source",
	}
	w.SetModel(model)

	reactor, _ := w.Model().TileAt(3, 3)
	reactor.Block = 315
	reactor.Team = 1
	reactor.Build = &Building{
		Block:     315,
		Team:      1,
		X:         3,
		Y:         3,
		Health:    1000,
		MaxHealth: 1000,
	}

	source, _ := w.Model().TileAt(2, 3)
	source.Block = 412
	source.Team = 1
	source.Build = &Building{
		Block:     412,
		Team:      1,
		X:         2,
		Y:         3,
		Health:    1000,
		MaxHealth: 1000,
	}
	w.rebuildBlockOccupancyLocked()
	w.ConfigureItemSource(int32(3*model.Width+2), 5)

	for i := 0; i < 300; i++ {
		w.Step(time.Second / 60)
	}

	reactor, _ = w.Model().TileAt(3, 3)
	if reactor.Block != 0 || reactor.Build != nil {
		t.Fatalf("expected thorium reactor to explode and be destroyed, got block=%d build=%v", reactor.Block, reactor.Build != nil)
	}
}

func TestPoweredTurretDoesNotRechargeWithoutTeamPower(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		410: "arc",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 8, 8, 410, 1, 0)
	arcPos := int32(8*model.Width + 8)
	w.buildStates[arcPos] = buildCombatState{Power: 0}
	model.Entities = append(model.Entities, RawEntity{
		ID:          1,
		TypeID:      35,
		Team:        2,
		X:           float32(8*8 + 4 + 32),
		Y:           float32(8*8 + 4),
		Health:      100,
		MaxHealth:   100,
		SlowMul:     1,
		RuntimeInit: true,
	})

	for i := 0; i < 240; i++ {
		w.Step(time.Second / 60)
	}

	if got := model.Entities[0].Health; got != 100 {
		t.Fatalf("expected unpowered arc to not fire, health=%f", got)
	}
	if st := w.buildStates[arcPos]; st.Power != 0 {
		t.Fatalf("expected unpowered arc to stay empty, power=%f", st.Power)
	}
}

func TestThoriumReactorPowersTurretRecharge(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		315: "thorium-reactor",
		316: "power-node",
		410: "arc",
	}
	w.SetModel(model)

	reactor := placeTestBuilding(t, w, 5, 8, 315, 1, 0)
	reactor.Build.AddItem(7, 30)
	reactor.Build.AddLiquid(3, 30)
	node := placeTestBuilding(t, w, 8, 8, 316, 1, 0)
	placeTestBuilding(t, w, 11, 8, 410, 1, 0)
	nodePos := int32(8*model.Width + 8)
	arcPos := int32(8*model.Width + 11)
	w.applyBuildingConfigLocked(nodePos, []protocol.Point2{{X: -3, Y: 0}, {X: 3, Y: 0}}, true)
	w.buildStates[arcPos] = buildCombatState{Power: 0}
	model.Entities = append(model.Entities, RawEntity{
		ID:          1,
		TypeID:      35,
		Team:        2,
		X:           float32(11*8 + 4 + 32),
		Y:           float32(8*8 + 4),
		Health:      100,
		MaxHealth:   100,
		SlowMul:     1,
		RuntimeInit: true,
	})

	for i := 0; i < 240; i++ {
		w.Step(time.Second / 60)
	}

	if got := model.Entities[0].Health; got >= 100 {
		t.Fatalf("expected powered arc to fire after reactor recharge, health=%f power=%f produced=%f consumed=%f", got, w.buildStates[arcPos].Power, w.teamPowerStates[1].Produced, w.teamPowerStates[1].Consumed)
	}
	if st := w.teamPowerStates[1]; st == nil || st.Produced <= 0 {
		t.Fatalf("expected team power production from thorium reactor")
	}
	_ = reactor
	_ = node
}

func TestThoriumReactorBuildsHeatProgress(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.BlockNames = map[int16]string{
		315: "thorium-reactor",
	}
	w.SetModel(model)

	reactor := placeTestBuilding(t, w, 8, 8, 315, 1, 0)
	reactor.Build.AddItem(thoriumItemID, 30)

	w.Step(time.Second / 60)

	reactorPos := int32(8*model.Width + 8)
	st, ok := w.reactorStates[reactorPos]
	if !ok {
		t.Fatal("expected thorium reactor runtime state to exist")
	}
	if st.Heat <= 0 {
		t.Fatalf("expected thorium reactor heat to rise, heat=%f", st.Heat)
	}
	if st.HeatProgress <= 0 {
		t.Fatalf("expected thorium reactor heat progress to rise like vanilla, heatProgress=%f", st.HeatProgress)
	}
	if got := w.heatStates[reactorPos]; got <= 0 {
		t.Fatalf("expected thorium reactor to publish heat state, heat=%f", got)
	}
}

func TestSolarPowerStoresIntoBattery(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.BlockNames = map[int16]string{
		420: "solar-panel-large",
		421: "battery",
		422: "power-node",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 4, 4, 420, 1, 0)
	placeTestBuilding(t, w, 8, 4, 421, 1, 0)
	placeTestBuilding(t, w, 6, 4, 422, 1, 0)
	nodePos := int32(4*model.Width + 6)
	w.applyBuildingConfigLocked(nodePos, []protocol.Point2{{X: -2, Y: 0}, {X: 2, Y: 0}}, true)

	for i := 0; i < 600; i++ {
		w.Step(time.Second / 60)
	}

	st := w.teamPowerStates[1]
	if st == nil {
		t.Fatalf("expected team power state to exist")
	}
	if st.Stored <= 0 {
		t.Fatalf("expected battery to store solar power, stored=%f", st.Stored)
	}
	if st.Stored > st.Capacity {
		t.Fatalf("expected stored power <= capacity, stored=%f capacity=%f", st.Stored, st.Capacity)
	}
}

func TestLaserDrillRequiresPowerToMine(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.BlockNames = map[int16]string{
		2:   "ore-copper",
		430: "laser-drill",
	}
	w.SetModel(model)
	paintAreaOverlay(t, w, 6, 6, 3, 2)
	drill := placeTestBuilding(t, w, 6, 6, 430, 1, 0)

	stepForSeconds(w, 20)

	if got := totalBuildingItems(drill.Build); got != 0 {
		t.Fatalf("expected unpowered laser drill to stay idle, items=%d", got)
	}
}

func TestLaserDrillMinesOreWhenPowered(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.BlockNames = map[int16]string{
		2:   "ore-copper",
		421: "battery",
		422: "power-node",
		430: "laser-drill",
	}
	w.SetModel(model)
	paintAreaOverlay(t, w, 6, 6, 3, 2)
	drill := placeTestBuilding(t, w, 6, 6, 430, 1, 0)
	placeTestBuilding(t, w, 6, 10, 421, 1, 0)
	placeTestBuilding(t, w, 6, 8, 422, 1, 0)
	w.powerStorageState[int32(10*model.Width+6)] = 4000
	linkPowerNode(t, w, 6, 8, protocol.Point2{X: 0, Y: -2}, protocol.Point2{X: 0, Y: 2})

	stepForSeconds(w, 12)

	if got := totalBuildingItems(drill.Build); got <= 0 {
		t.Fatalf("expected powered laser drill to mine ore, items=%d", got)
	}
}

func TestMechanicalDrillMinesWithoutPower(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.BlockNames = map[int16]string{
		2:   "ore-copper",
		429: "mechanical-drill",
	}
	w.SetModel(model)
	paintAreaOverlay(t, w, 5, 5, 2, 2)
	drill := placeTestBuilding(t, w, 5, 5, 429, 1, 0)

	stepForSeconds(w, 26)

	if got := totalBuildingItems(drill.Build); got <= 0 {
		t.Fatalf("expected mechanical drill to mine without power, items=%d", got)
	}
}

func TestImpactDrillOffloadsEntireBurstIntoAdjacentCore(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(20, 20)
	model.BlockNames = map[int16]string{
		339: "core-shard",
		16:  "ore-beryllium",
		478: "power-source",
		904: "impact-drill",
	}
	w.SetModel(model)
	paintAreaOverlay(t, w, 8, 8, 4, 16)
	core := placeTestBuilding(t, w, 5, 8, 339, 1, 0)
	drill := placeTestBuilding(t, w, 8, 8, 904, 1, 0)
	drill.Build.AddLiquid(waterLiquidID, 200)
	placeTestBuilding(t, w, 8, 12, 478, 1, 0)
	linkPowerNode(t, w, 8, 12, protocol.Point2{X: 0, Y: -4})

	for i := 0; i < 600; i++ {
		w.Step(time.Second / 60)
		if core.Build.ItemAmount(berylliumItemID) > 0 || drill.Build.ItemAmount(berylliumItemID) > 0 {
			break
		}
	}

	if got := core.Build.ItemAmount(berylliumItemID); got != 16 {
		t.Fatalf("expected adjacent core to receive full impact-drill burst 16, got %d", got)
	}
	if got := drill.Build.ItemAmount(berylliumItemID); got != 0 {
		t.Fatalf("expected impact-drill buffer to stay empty when offload path is open, got %d", got)
	}
}

func TestMechanicalPumpPumpsFloorLiquid(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		1:   "water",
		440: "mechanical-pump",
	}
	w.SetModel(model)
	paintAreaFloor(t, w, 4, 4, 1, 1)
	pump := placeTestBuilding(t, w, 4, 4, 440, 1, 0)

	stepForSeconds(w, 2)

	if got := pump.Build.LiquidAmount(waterLiquidID); got <= 0 {
		t.Fatalf("expected mechanical pump to extract floor water, amount=%f", got)
	}
}

func TestRotaryPumpRequiresPowerToPump(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.BlockNames = map[int16]string{
		1:   "water",
		421: "battery",
		422: "power-node",
		441: "rotary-pump",
	}
	w.SetModel(model)
	paintAreaFloor(t, w, 6, 6, 2, 1)
	pump := placeTestBuilding(t, w, 6, 6, 441, 1, 0)

	stepForSeconds(w, 2)
	if got := pump.Build.LiquidAmount(waterLiquidID); got != 0 {
		t.Fatalf("expected unpowered rotary pump to stay idle, amount=%f", got)
	}

	placeTestBuilding(t, w, 6, 11, 421, 1, 0)
	placeTestBuilding(t, w, 6, 9, 422, 1, 0)
	w.powerStorageState[int32(11*model.Width+6)] = 4000
	linkPowerNode(t, w, 6, 9, protocol.Point2{X: 0, Y: -3}, protocol.Point2{X: 0, Y: 2})

	stepForSeconds(w, 2)

	if got := pump.Build.LiquidAmount(waterLiquidID); got <= 0 {
		t.Fatalf("expected powered rotary pump to extract floor water, amount=%f", got)
	}
}

func TestWaterExtractorProducesWhenPowered(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.BlockNames = map[int16]string{
		10:  "stone",
		421: "battery",
		422: "power-node",
		442: "water-extractor",
	}
	w.SetModel(model)
	paintAreaFloor(t, w, 6, 6, 2, 10)
	extractor := placeTestBuilding(t, w, 6, 6, 442, 1, 0)
	placeTestBuilding(t, w, 6, 11, 421, 1, 0)
	placeTestBuilding(t, w, 6, 9, 422, 1, 0)
	w.powerStorageState[int32(11*model.Width+6)] = 4000
	linkPowerNode(t, w, 6, 9, protocol.Point2{X: 0, Y: -3}, protocol.Point2{X: 0, Y: 2})

	stepForSeconds(w, 2)

	if got := extractor.Build.LiquidAmount(waterLiquidID); got <= 0 {
		t.Fatalf("expected powered water extractor to produce water, amount=%f", got)
	}
}

func TestOilExtractorConsumesResourcesWhenPowered(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(18, 18)
	model.BlockNames = map[int16]string{
		11:  "shale",
		421: "battery",
		422: "power-node",
		443: "oil-extractor",
	}
	w.SetModel(model)
	paintAreaFloor(t, w, 8, 8, 3, 11)
	extractor := placeTestBuilding(t, w, 8, 8, 443, 1, 0)
	extractor.Build.AddItem(sandItemID, 2)
	extractor.Build.AddLiquid(waterLiquidID, 40)
	placeTestBuilding(t, w, 8, 13, 421, 1, 0)
	placeTestBuilding(t, w, 8, 11, 422, 1, 0)
	w.powerStorageState[int32(13*model.Width+8)] = 4000
	linkPowerNode(t, w, 8, 11, protocol.Point2{X: 0, Y: -3}, protocol.Point2{X: 0, Y: 2})

	stepForSeconds(w, 5)

	if got := extractor.Build.LiquidAmount(oilLiquidID); got <= 0 {
		t.Fatalf("expected oil extractor to produce oil, amount=%f", got)
	}
	if got := extractor.Build.LiquidAmount(waterLiquidID); got >= 40 {
		t.Fatalf("expected oil extractor to consume water, remaining=%f", got)
	}
	if got := extractor.Build.ItemAmount(sandItemID); got >= 2 {
		t.Fatalf("expected oil extractor to consume sand over time, remaining=%d", got)
	}
}

func TestGraphitePressCraftsWithoutPower(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		450: "graphite-press",
	}
	w.SetModel(model)
	press := placeTestBuilding(t, w, 4, 4, 450, 1, 0)
	press.Build.AddItem(coalItemID, 2)

	stepForSeconds(w, 2)

	if got := press.Build.ItemAmount(graphiteItemID); got <= 0 {
		t.Fatalf("expected graphite press to craft graphite without power, amount=%d", got)
	}
}

func TestSiliconSmelterRequiresPower(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.BlockNames = map[int16]string{
		421: "battery",
		422: "power-node",
		451: "silicon-smelter",
	}
	w.SetModel(model)
	smelter := placeTestBuilding(t, w, 6, 6, 451, 1, 0)
	smelter.Build.AddItem(coalItemID, 2)
	smelter.Build.AddItem(sandItemID, 4)

	stepForSeconds(w, 2)
	if got := smelter.Build.ItemAmount(siliconItemID); got != 0 {
		t.Fatalf("expected unpowered silicon smelter to stay idle, amount=%d", got)
	}

	placeTestBuilding(t, w, 6, 11, 421, 1, 0)
	placeTestBuilding(t, w, 6, 9, 422, 1, 0)
	w.powerStorageState[int32(11*model.Width+6)] = 4000
	linkPowerNode(t, w, 6, 9, protocol.Point2{X: 0, Y: -3}, protocol.Point2{X: 0, Y: 2})

	stepForSeconds(w, 2)

	if got := smelter.Build.ItemAmount(siliconItemID); got <= 0 {
		t.Fatalf("expected powered silicon smelter to craft silicon, amount=%d", got)
	}
}

func TestSiliconArcFurnaceMatchesVanillaPowerCraft(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.BlockNames = map[int16]string{
		421: "battery",
		422: "power-node",
		454: "silicon-arc-furnace",
	}
	w.SetModel(model)
	furnace := placeTestBuilding(t, w, 6, 6, 454, 1, 0)
	furnace.Build.AddItem(graphiteItemID, 1)
	furnace.Build.AddItem(sandItemID, 4)

	stepForSeconds(w, 1)
	if got := furnace.Build.ItemAmount(siliconItemID); got != 0 {
		t.Fatalf("expected unpowered silicon arc furnace to stay idle, amount=%d", got)
	}

	placeTestBuilding(t, w, 6, 11, 421, 1, 0)
	placeTestBuilding(t, w, 6, 9, 422, 1, 0)
	w.powerStorageState[int32(11*model.Width+6)] = 4000
	linkPowerNode(t, w, 6, 9, protocol.Point2{X: 0, Y: -3}, protocol.Point2{X: 0, Y: 2})

	stepForSeconds(w, 1)

	if got := furnace.Build.ItemAmount(siliconItemID); got != 4 {
		t.Fatalf("expected powered silicon arc furnace to craft 4 silicon, amount=%d", got)
	}
	if got := furnace.Build.ItemAmount(graphiteItemID); got != 0 {
		t.Fatalf("expected powered silicon arc furnace to consume graphite, remaining=%d", got)
	}
	if got := furnace.Build.ItemAmount(sandItemID); got != 0 {
		t.Fatalf("expected powered silicon arc furnace to consume sand, remaining=%d", got)
	}
}

func TestCryofluidMixerProducesCryofluid(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.BlockNames = map[int16]string{
		421: "battery",
		422: "power-node",
		452: "cryofluid-mixer",
	}
	w.SetModel(model)
	mixer := placeTestBuilding(t, w, 6, 6, 452, 1, 0)
	mixer.Build.AddItem(titaniumItemID, 2)
	mixer.Build.AddLiquid(waterLiquidID, 36)
	placeTestBuilding(t, w, 6, 11, 421, 1, 0)
	placeTestBuilding(t, w, 6, 9, 422, 1, 0)
	w.powerStorageState[int32(11*model.Width+6)] = 4000
	linkPowerNode(t, w, 6, 9, protocol.Point2{X: 0, Y: -3}, protocol.Point2{X: 0, Y: 2})

	stepForSeconds(w, 3)

	if got := mixer.Build.LiquidAmount(cryofluidLiquidID); got <= 0 {
		t.Fatalf("expected cryofluid mixer to produce cryofluid, amount=%f", got)
	}
	if got := mixer.Build.LiquidAmount(waterLiquidID); got >= 36 {
		t.Fatalf("expected cryofluid mixer to consume water, remaining=%f", got)
	}
}

func TestSeparatorProducesItemsFromSlag(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.BlockNames = map[int16]string{
		421: "battery",
		422: "power-node",
		453: "separator",
	}
	w.SetModel(model)
	separator := placeTestBuilding(t, w, 6, 6, 453, 1, 0)
	separator.Build.AddLiquid(slagLiquidID, 20)
	placeTestBuilding(t, w, 6, 11, 421, 1, 0)
	placeTestBuilding(t, w, 6, 9, 422, 1, 0)
	w.powerStorageState[int32(11*model.Width+6)] = 4000
	linkPowerNode(t, w, 6, 9, protocol.Point2{X: 0, Y: -3}, protocol.Point2{X: 0, Y: 2})

	stepForSeconds(w, 3)

	total := totalBuildingItems(separator.Build)
	if total <= 0 {
		t.Fatalf("expected separator to produce at least one item, total=%d", total)
	}
	produced := separator.Build.ItemAmount(copperItemID) + separator.Build.ItemAmount(leadItemID) + separator.Build.ItemAmount(graphiteItemID) + separator.Build.ItemAmount(titaniumItemID)
	if produced != total {
		t.Fatalf("expected separator outputs to match vanilla result pool, total=%d produced=%d", total, produced)
	}
}

func TestDisassemblerConsumesScrapAndSlag(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(18, 18)
	model.BlockNames = map[int16]string{
		421: "battery",
		422: "power-node",
		458: "disassembler",
	}
	w.SetModel(model)
	disassembler := placeTestBuilding(t, w, 8, 8, 458, 1, 0)
	disassembler.Build.AddItem(scrapItemID, 2)
	disassembler.Build.AddLiquid(slagLiquidID, 20)
	placeTestBuilding(t, w, 8, 13, 421, 1, 0)
	placeTestBuilding(t, w, 8, 11, 422, 1, 0)
	w.powerStorageState[int32(13*model.Width+8)] = 4000
	linkPowerNode(t, w, 8, 11, protocol.Point2{X: 0, Y: -3}, protocol.Point2{X: 0, Y: 2})

	stepForSeconds(w, 1)

	total := totalBuildingItems(disassembler.Build)
	if total <= 0 {
		t.Fatalf("expected disassembler to produce at least one item, total=%d", total)
	}
	if got := disassembler.Build.ItemAmount(scrapItemID); got >= 2 {
		t.Fatalf("expected disassembler to consume scrap, remaining=%d", got)
	}
	if got := disassembler.Build.LiquidAmount(slagLiquidID); got >= 20 {
		t.Fatalf("expected disassembler to consume slag, remaining=%f", got)
	}
	produced := disassembler.Build.ItemAmount(sandItemID) + disassembler.Build.ItemAmount(graphiteItemID) + disassembler.Build.ItemAmount(titaniumItemID) + disassembler.Build.ItemAmount(thoriumItemID)
	if produced != total {
		t.Fatalf("expected disassembler outputs to match vanilla result pool, total=%d produced=%d", total, produced)
	}
}

func TestSlagCentrifugeConsumesSandAndSlag(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(18, 18)
	model.BlockNames = map[int16]string{
		421: "battery",
		422: "power-node",
		459: "slag-centrifuge",
	}
	w.SetModel(model)
	centrifuge := placeTestBuilding(t, w, 8, 8, 459, 1, 0)
	centrifuge.Build.AddItem(sandItemID, 1)
	centrifuge.Build.AddLiquid(slagLiquidID, 80)
	placeTestBuilding(t, w, 8, 13, 421, 1, 0)
	placeTestBuilding(t, w, 8, 11, 422, 1, 0)
	w.powerStorageState[int32(13*model.Width+8)] = 4000
	linkPowerNode(t, w, 8, 11, protocol.Point2{X: 0, Y: -3}, protocol.Point2{X: 0, Y: 2})

	stepForSeconds(w, 3)

	if got := centrifuge.Build.LiquidAmount(galliumLiquidID); got <= 0 {
		t.Fatalf("expected slag centrifuge to produce gallium, amount=%f", got)
	}
	if got := centrifuge.Build.LiquidAmount(slagLiquidID); got >= 80 {
		t.Fatalf("expected slag centrifuge to consume slag, remaining=%f", got)
	}
	if got := centrifuge.Build.ItemAmount(sandItemID); got != 0 {
		t.Fatalf("expected slag centrifuge to consume sand after one craft, remaining=%d", got)
	}
}

func TestPlastaniumCompressorConsumesOilAndTitanium(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.BlockNames = map[int16]string{
		421: "battery",
		422: "power-node",
		454: "plastanium-compressor",
	}
	w.SetModel(model)
	compressor := placeTestBuilding(t, w, 6, 6, 454, 1, 0)
	compressor.Build.AddItem(titaniumItemID, 4)
	compressor.Build.AddLiquid(oilLiquidID, 60)
	placeTestBuilding(t, w, 6, 11, 421, 1, 0)
	placeTestBuilding(t, w, 6, 9, 422, 1, 0)
	w.powerStorageState[int32(11*model.Width+6)] = 4000
	linkPowerNode(t, w, 6, 9, protocol.Point2{X: 0, Y: -3}, protocol.Point2{X: 0, Y: 2})

	stepForSeconds(w, 3)

	if got := compressor.Build.ItemAmount(plastaniumItemID); got <= 0 {
		t.Fatalf("expected plastanium compressor to craft plastanium, amount=%d", got)
	}
	if got := compressor.Build.LiquidAmount(oilLiquidID); got >= 60 {
		t.Fatalf("expected plastanium compressor to consume oil, remaining=%f", got)
	}
}

func TestSiliconCrucibleGetsHeatBoost(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(18, 18)
	model.BlockNames = map[int16]string{
		20:  "hotrock",
		421: "battery",
		422: "power-node",
		455: "silicon-crucible",
	}
	w.SetModel(model)
	paintAreaFloor(t, w, 8, 8, 3, 20)
	crucible := placeTestBuilding(t, w, 8, 8, 455, 1, 0)
	crucible.Build.AddItem(coalItemID, 4)
	crucible.Build.AddItem(sandItemID, 6)
	crucible.Build.AddItem(pyratiteItemID, 1)
	placeTestBuilding(t, w, 8, 13, 421, 1, 0)
	placeTestBuilding(t, w, 8, 11, 422, 1, 0)
	w.powerStorageState[int32(13*model.Width+8)] = 4000
	linkPowerNode(t, w, 8, 11, protocol.Point2{X: 0, Y: -3}, protocol.Point2{X: 0, Y: 2})

	stepForSeconds(w, 1)

	if got := crucible.Build.ItemAmount(siliconItemID); got < 8 {
		t.Fatalf("expected heated silicon crucible to finish one craft within 1s, amount=%d", got)
	}
}

func TestSporePressProducesOilWhenPowered(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.BlockNames = map[int16]string{
		421: "battery",
		422: "power-node",
		456: "spore-press",
	}
	w.SetModel(model)
	press := placeTestBuilding(t, w, 6, 6, 456, 1, 0)
	press.Build.AddItem(sporePodItemID, 2)
	placeTestBuilding(t, w, 6, 11, 421, 1, 0)
	placeTestBuilding(t, w, 6, 9, 422, 1, 0)
	w.powerStorageState[int32(11*model.Width+6)] = 4000
	linkPowerNode(t, w, 6, 9, protocol.Point2{X: 0, Y: -3}, protocol.Point2{X: 0, Y: 2})

	stepForSeconds(w, 1)

	if got := press.Build.LiquidAmount(oilLiquidID); got <= 0 {
		t.Fatalf("expected spore press to produce oil, amount=%f", got)
	}
}

func TestCultivatorGetsSporeBoost(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(20, 20)
	model.BlockNames = map[int16]string{
		21:  "spore-moss",
		22:  "stone",
		421: "battery",
		422: "power-node",
		457: "cultivator",
	}
	w.SetModel(model)
	paintAreaFloor(t, w, 6, 6, 2, 21)
	paintAreaFloor(t, w, 12, 6, 2, 22)
	boosted := placeTestBuilding(t, w, 6, 6, 457, 1, 0)
	plain := placeTestBuilding(t, w, 12, 6, 457, 1, 0)
	boosted.Build.AddLiquid(waterLiquidID, 80)
	plain.Build.AddLiquid(waterLiquidID, 80)
	placeTestBuilding(t, w, 9, 12, 421, 1, 0)
	placeTestBuilding(t, w, 9, 9, 422, 1, 0)
	w.powerStorageState[int32(12*model.Width+9)] = 4000
	linkPowerNode(t, w, 9, 9, protocol.Point2{X: -3, Y: -3}, protocol.Point2{X: 3, Y: -3}, protocol.Point2{X: 0, Y: 3})

	stepForSeconds(w, 1)

	if got := boosted.Build.ItemAmount(sporePodItemID); got <= 0 {
		t.Fatalf("expected spore-boosted cultivator to finish within 1s, amount=%d", got)
	}
	if got := plain.Build.ItemAmount(sporePodItemID); got != 0 {
		t.Fatalf("expected plain cultivator to still be in progress after 1s, amount=%d", got)
	}
}

func TestVentCondenserRequiresFullSteamFootprint(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(18, 18)
	model.BlockNames = map[int16]string{
		30:  "rhyolite",
		31:  "rhyolite-vent",
		421: "battery",
		422: "power-node",
		458: "vent-condenser",
	}
	w.SetModel(model)
	paintAreaFloor(t, w, 8, 8, 3, 31)
	tile, err := w.Model().TileAt(7, 7)
	if err != nil || tile == nil {
		t.Fatalf("floor tile lookup failed: %v", err)
	}
	tile.Floor = 30

	condenser := placeTestBuilding(t, w, 8, 8, 458, 1, 0)
	placeTestBuilding(t, w, 8, 13, 421, 1, 0)
	placeTestBuilding(t, w, 8, 11, 422, 1, 0)
	w.powerStorageState[int32(13*model.Width+8)] = 4000
	linkPowerNode(t, w, 8, 11, protocol.Point2{X: 0, Y: -3}, protocol.Point2{X: 0, Y: 2})

	stepForSeconds(w, 3)
	if got := condenser.Build.LiquidAmount(waterLiquidID); got != 0 {
		t.Fatalf("expected vent condenser to stay idle below vanilla min efficiency, amount=%f", got)
	}

	paintAreaFloor(t, w, 8, 8, 3, 31)
	stepForSeconds(w, 3)

	if got := condenser.Build.LiquidAmount(waterLiquidID); got <= 0 {
		t.Fatalf("expected vent condenser to produce water on full steam footprint, amount=%f", got)
	}
}

func TestElectricHeaterBuildsHeatWhenPowered(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.BlockNames = map[int16]string{
		421: "battery",
		422: "power-node",
		461: "electric-heater",
	}
	w.SetModel(model)
	placeTestBuilding(t, w, 6, 6, 461, 1, 0)
	placeTestBuilding(t, w, 6, 11, 421, 1, 0)
	placeTestBuilding(t, w, 6, 9, 422, 1, 0)
	w.powerStorageState[int32(11*model.Width+6)] = 4000
	linkPowerNode(t, w, 6, 9, protocol.Point2{X: 0, Y: -3}, protocol.Point2{X: 0, Y: 2})

	stepForSeconds(w, 1)

	if heat := w.heatStates[int32(6*model.Width+6)]; heat <= 0 {
		t.Fatalf("expected powered electric heater to build heat, heat=%f", heat)
	}
}

func TestHeatSourceProducesMaxHeatImmediately(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		490: "heat-source",
	}
	w.SetModel(model)
	placeTestBuilding(t, w, 4, 4, 490, 1, 0)

	w.Step(time.Second / 60)

	if heat := w.heatStates[int32(4*model.Width+4)]; heat < 999 {
		t.Fatalf("expected heat-source to publish vanilla max heat immediately, heat=%f", heat)
	}
}

func TestItemVoidAcceptsAndDeletesItems(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		430: "router",
		491: "item-void",
	}
	w.SetModel(model)

	src := placeTestBuilding(t, w, 3, 3, 430, 1, 0)
	placeTestBuilding(t, w, 4, 3, 491, 1, 0)
	src.Build.AddItem(coalItemID, 1)
	item := coalItemID

	moved := w.dumpSingleItemLocked(int32(3*model.Width+3), src, &item, nil)
	if !moved {
		t.Fatal("expected item-void to accept dumped item")
	}
	if got := src.Build.ItemAmount(coalItemID); got != 0 {
		t.Fatalf("expected item to be deleted by item-void, remaining=%d", got)
	}
}

func TestLiquidVoidAcceptsAndDeletesLiquids(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		431: "liquid-router",
		492: "liquid-void",
	}
	w.SetModel(model)

	src := placeTestBuilding(t, w, 3, 3, 431, 1, 0)
	placeTestBuilding(t, w, 4, 3, 492, 1, 0)
	src.Build.AddLiquid(waterLiquidID, 10)

	moved := w.dumpLiquidLocked(int32(3*model.Width+3), src, waterLiquidID, 10)
	if !moved {
		t.Fatal("expected liquid-void to accept dumped liquid")
	}
	if got := src.Build.LiquidAmount(waterLiquidID); got != 0 {
		t.Fatalf("expected liquid to be deleted by liquid-void, remaining=%f", got)
	}
}

func TestAtmosphericConcentratorRequiresHeat(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(20, 20)
	model.BlockNames = map[int16]string{
		421: "battery",
		422: "power-node",
		462: "slag-heater",
		463: "atmospheric-concentrator",
	}
	w.SetModel(model)
	concentrator := placeTestBuilding(t, w, 8, 8, 463, 1, 0)
	placeTestBuilding(t, w, 8, 13, 421, 1, 0)
	placeTestBuilding(t, w, 8, 11, 422, 1, 0)
	w.powerStorageState[int32(13*model.Width+8)] = 4000
	linkPowerNode(t, w, 8, 11, protocol.Point2{X: 0, Y: -3}, protocol.Point2{X: 0, Y: 2})

	stepForSeconds(w, 2)
	if got := concentrator.Build.LiquidAmount(nitrogenLiquidID); got != 0 {
		t.Fatalf("expected unheated atmospheric concentrator to stay idle, amount=%f", got)
	}

	west := placeTestBuilding(t, w, 5, 8, 462, 1, 0)
	east := placeTestBuilding(t, w, 11, 8, 462, 1, 2)
	north := placeTestBuilding(t, w, 8, 5, 462, 1, 1)
	west.Build.AddLiquid(slagLiquidID, 120)
	east.Build.AddLiquid(slagLiquidID, 120)
	north.Build.AddLiquid(slagLiquidID, 120)

	stepForSeconds(w, 3)

	if got := concentrator.Build.LiquidAmount(nitrogenLiquidID); got <= 0 {
		t.Fatalf("expected heated atmospheric concentrator to produce nitrogen, amount=%f", got)
	}
	if heat := w.crafterReceivedHeatLocked(int32(8*model.Width+8), concentrator); heat < 24 {
		t.Fatalf("expected atmospheric concentrator to receive vanilla heat requirement, heat=%f", heat)
	}
}

func TestOxidationChamberProducesOxideAndHeat(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(18, 18)
	model.BlockNames = map[int16]string{
		421: "battery",
		422: "power-node",
		464: "oxidation-chamber",
	}
	w.SetModel(model)
	chamber := placeTestBuilding(t, w, 8, 8, 464, 1, 0)
	chamber.Build.AddItem(berylliumItemID, 2)
	chamber.Build.AddLiquid(ozoneLiquidID, 10)
	placeTestBuilding(t, w, 8, 13, 421, 1, 0)
	placeTestBuilding(t, w, 8, 11, 422, 1, 0)
	w.powerStorageState[int32(13*model.Width+8)] = 4000
	linkPowerNode(t, w, 8, 11, protocol.Point2{X: 0, Y: -3}, protocol.Point2{X: 0, Y: 2})

	stepForSeconds(w, 3)

	if got := chamber.Build.ItemAmount(oxideItemID); got <= 0 {
		t.Fatalf("expected oxidation chamber to craft oxide, amount=%d", got)
	}
	if heat := w.heatStates[int32(8*model.Width+8)]; heat <= 0 {
		t.Fatalf("expected oxidation chamber to output heat while active, heat=%f", heat)
	}
}

func TestHeatRedirectorRelaysHeatToCrafter(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		421: "battery",
		422: "power-node",
		462: "slag-heater",
		463: "atmospheric-concentrator",
		465: "heat-redirector",
	}
	w.SetModel(model)
	concentrator := placeTestBuilding(t, w, 12, 12, 463, 1, 0)
	placeTestBuilding(t, w, 12, 17, 421, 1, 0)
	placeTestBuilding(t, w, 12, 15, 422, 1, 0)
	w.powerStorageState[int32(17*model.Width+12)] = 4000
	linkPowerNode(t, w, 12, 15, protocol.Point2{X: 0, Y: -3}, protocol.Point2{X: 0, Y: 2})

	placeTestBuilding(t, w, 9, 12, 465, 1, 0)
	placeTestBuilding(t, w, 15, 12, 465, 1, 2)
	placeTestBuilding(t, w, 12, 9, 465, 1, 1)
	westHeater := placeTestBuilding(t, w, 6, 12, 462, 1, 0)
	eastHeater := placeTestBuilding(t, w, 18, 12, 462, 1, 2)
	northHeater := placeTestBuilding(t, w, 12, 6, 462, 1, 1)
	westHeater.Build.AddLiquid(slagLiquidID, 240)
	eastHeater.Build.AddLiquid(slagLiquidID, 240)
	northHeater.Build.AddLiquid(slagLiquidID, 240)

	stepForSeconds(w, 4)

	if got := concentrator.Build.LiquidAmount(nitrogenLiquidID); got <= 0 {
		t.Fatalf("expected redirected heat to drive atmospheric concentrator, amount=%f", got)
	}
	if heat := w.crafterReceivedHeatLocked(int32(12*model.Width+12), concentrator); heat < 24 {
		t.Fatalf("expected redirected heat to satisfy vanilla requirement, heat=%f", heat)
	}
}

func TestHeatRouterDoesNotOutputToFront(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(20, 20)
	model.BlockNames = map[int16]string{
		463: "atmospheric-concentrator",
		466: "heat-router",
	}
	w.SetModel(model)
	router := placeTestBuilding(t, w, 10, 10, 466, 1, 0)
	w.heatStates[int32(10*model.Width+10)] = 24
	blocked := placeTestBuilding(t, w, 13, 10, 463, 1, 0)
	allowed := placeTestBuilding(t, w, 10, 7, 463, 1, 0)

	if heat := w.crafterReceivedHeatLocked(int32(10*model.Width+13), blocked); heat != 0 {
		t.Fatalf("expected heat router front side to block heat, heat=%f", heat)
	}
	if heat := w.crafterReceivedHeatLocked(int32(7*model.Width+10), allowed); heat <= 0 {
		t.Fatalf("expected heat router side to output split heat, heat=%f", heat)
	}
	if heat := w.crafterReceivedHeatLocked(int32(7*model.Width+10), allowed); heat >= 24 {
		t.Fatalf("expected heat router side to split heat across surfaces, heat=%f", heat)
	}
	if router == nil {
		t.Fatalf("expected heat router placement to succeed")
	}
}

func TestElectrolyzerSplitsLiquidOutputsByDirection(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(20, 20)
	model.BlockNames = map[int16]string{
		421: "battery",
		422: "power-node",
		423: "liquid-router",
		460: "electrolyzer",
	}
	w.SetModel(model)
	electrolyzer := placeTestBuilding(t, w, 8, 8, 460, 1, 0)
	electrolyzer.Build.AddLiquid(waterLiquidID, 20)
	north := placeTestBuilding(t, w, 8, 6, 423, 1, 0)
	south := placeTestBuilding(t, w, 8, 10, 423, 1, 0)
	placeTestBuilding(t, w, 8, 13, 421, 1, 0)
	placeTestBuilding(t, w, 8, 11, 422, 1, 0)
	w.powerStorageState[int32(13*model.Width+8)] = 4000
	linkPowerNode(t, w, 8, 11, protocol.Point2{X: 0, Y: -3}, protocol.Point2{X: 0, Y: 2})

	stepForSeconds(w, 2)

	if got := south.Build.LiquidAmount(ozoneLiquidID); got <= 0 {
		t.Fatalf("expected south side to receive ozone, amount=%f", got)
	}
	if got := south.Build.LiquidAmount(hydrogenLiquidID); got != 0 {
		t.Fatalf("expected south side to reject hydrogen, amount=%f", got)
	}
	if got := north.Build.LiquidAmount(hydrogenLiquidID); got <= 0 {
		t.Fatalf("expected north side to receive hydrogen, amount=%f", got)
	}
	if got := north.Build.LiquidAmount(ozoneLiquidID); got != 0 {
		t.Fatalf("expected north side to reject ozone, amount=%f", got)
	}
}

func TestCarbideCrucibleRequiresHeatAndCraftsCarbide(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		421: "battery",
		422: "power-node",
		462: "slag-heater",
		465: "heat-redirector",
		467: "carbide-crucible",
	}
	w.SetModel(model)
	crucible := placeTestBuilding(t, w, 10, 10, 467, 1, 0)
	crucible.Build.AddItem(tungstenItemID, 6)
	crucible.Build.AddItem(graphiteItemID, 9)
	placeTestBuilding(t, w, 10, 18, 421, 1, 0)
	placeTestBuilding(t, w, 10, 16, 422, 1, 0)
	w.powerStorageState[int32(18*model.Width+10)] = 4000
	linkPowerNode(t, w, 10, 16, protocol.Point2{X: 0, Y: -6}, protocol.Point2{X: 0, Y: 2})

	stepForSeconds(w, 2)
	if got := crucible.Build.ItemAmount(carbideItemID); got != 0 {
		t.Fatalf("expected unheated carbide crucible to stay idle, amount=%d", got)
	}

	placeTestBuilding(t, w, 7, 10, 465, 1, 0)
	west := placeTestBuilding(t, w, 4, 10, 462, 1, 0)
	redirectorNorth := placeTestBuilding(t, w, 7, 7, 462, 1, 1)
	east := placeTestBuilding(t, w, 13, 10, 462, 1, 2)
	north := placeTestBuilding(t, w, 10, 7, 462, 1, 1)
	south := placeTestBuilding(t, w, 10, 13, 462, 1, 3)
	west.Build.AddLiquid(slagLiquidID, 120)
	redirectorNorth.Build.AddLiquid(slagLiquidID, 120)
	east.Build.AddLiquid(slagLiquidID, 120)
	north.Build.AddLiquid(slagLiquidID, 120)
	south.Build.AddLiquid(slagLiquidID, 120)

	stepForSeconds(w, 3)

	if got := crucible.Build.ItemAmount(carbideItemID); got <= 0 {
		t.Fatalf("expected heated carbide crucible to craft carbide, amount=%d heat=%f", got, w.crafterReceivedHeatLocked(int32(10*model.Width+10), crucible))
	}
	if heat := w.crafterReceivedHeatLocked(int32(10*model.Width+10), crucible); heat < 40 {
		t.Fatalf("expected carbide crucible to receive vanilla heat requirement, heat=%f", heat)
	}
}

func TestSurgeCrucibleRequiresHeatAndCraftsSurgeAlloy(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		421: "battery",
		422: "power-node",
		462: "slag-heater",
		465: "heat-redirector",
		467: "surge-crucible",
	}
	w.SetModel(model)
	crucible := placeTestBuilding(t, w, 10, 10, 467, 1, 0)
	crucible.Build.AddItem(siliconItemID, 9)
	crucible.Build.AddLiquid(slagLiquidID, 200)
	placeTestBuilding(t, w, 10, 18, 421, 1, 0)
	placeTestBuilding(t, w, 10, 16, 422, 1, 0)
	w.powerStorageState[int32(18*model.Width+10)] = 4000
	linkPowerNode(t, w, 10, 16, protocol.Point2{X: 0, Y: -6}, protocol.Point2{X: 0, Y: 2})

	stepForSeconds(w, 2)
	if got := crucible.Build.ItemAmount(surgeAlloyItemID); got != 0 {
		t.Fatalf("expected unheated surge crucible to stay idle, amount=%d", got)
	}

	placeTestBuilding(t, w, 7, 10, 465, 1, 0)
	west := placeTestBuilding(t, w, 4, 10, 462, 1, 0)
	redirectorNorth := placeTestBuilding(t, w, 7, 7, 462, 1, 1)
	east := placeTestBuilding(t, w, 13, 10, 462, 1, 2)
	north := placeTestBuilding(t, w, 10, 7, 462, 1, 1)
	south := placeTestBuilding(t, w, 10, 13, 462, 1, 3)
	west.Build.AddLiquid(slagLiquidID, 120)
	redirectorNorth.Build.AddLiquid(slagLiquidID, 120)
	east.Build.AddLiquid(slagLiquidID, 120)
	north.Build.AddLiquid(slagLiquidID, 120)
	south.Build.AddLiquid(slagLiquidID, 120)

	stepForSeconds(w, 3)

	if got := crucible.Build.ItemAmount(surgeAlloyItemID); got <= 0 {
		t.Fatalf("expected heated surge crucible to craft surge alloy, amount=%d heat=%f", got, w.crafterReceivedHeatLocked(int32(10*model.Width+10), crucible))
	}
	if heat := w.crafterReceivedHeatLocked(int32(10*model.Width+10), crucible); heat < 40 {
		t.Fatalf("expected surge crucible to receive vanilla heat requirement, heat=%f", heat)
	}
}

func TestCyanogenSynthesizerRequiresHeatAndCraftsCyanogen(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		421: "battery",
		422: "power-node",
		462: "slag-heater",
		467: "cyanogen-synthesizer",
	}
	w.SetModel(model)
	synth := placeTestBuilding(t, w, 10, 10, 467, 1, 0)
	synth.Build.AddItem(graphiteItemID, 6)
	synth.Build.AddLiquid(arkyciteLiquidID, 80)
	placeTestBuilding(t, w, 10, 18, 421, 1, 0)
	placeTestBuilding(t, w, 10, 16, 422, 1, 0)
	w.powerStorageState[int32(18*model.Width+10)] = 4000
	linkPowerNode(t, w, 10, 16, protocol.Point2{X: 0, Y: -6}, protocol.Point2{X: 0, Y: 2})

	stepForSeconds(w, 2)
	if got := synth.Build.LiquidAmount(cyanogenLiquidID); got != 0 {
		t.Fatalf("expected unheated cyanogen synthesizer to stay idle, amount=%f", got)
	}

	west := placeTestBuilding(t, w, 7, 10, 462, 1, 0)
	east := placeTestBuilding(t, w, 13, 10, 462, 1, 2)
	north := placeTestBuilding(t, w, 10, 7, 462, 1, 1)
	west.Build.AddLiquid(slagLiquidID, 120)
	east.Build.AddLiquid(slagLiquidID, 120)
	north.Build.AddLiquid(slagLiquidID, 120)

	stepForSeconds(w, 3)

	if got := synth.Build.LiquidAmount(cyanogenLiquidID); got <= 0 {
		t.Fatalf("expected heated cyanogen synthesizer to craft cyanogen, amount=%f heat=%f", got, w.crafterReceivedHeatLocked(int32(10*model.Width+10), synth))
	}
	if heat := w.crafterReceivedHeatLocked(int32(10*model.Width+10), synth); heat < 20 {
		t.Fatalf("expected cyanogen synthesizer to receive vanilla heat requirement, heat=%f", heat)
	}
}

func TestPhaseSynthesizerRequiresHeatAndCraftsPhaseFabric(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		421: "battery",
		422: "power-node",
		462: "slag-heater",
		467: "phase-synthesizer",
	}
	w.SetModel(model)
	synth := placeTestBuilding(t, w, 10, 10, 467, 1, 0)
	synth.Build.AddItem(thoriumItemID, 6)
	synth.Build.AddItem(sandItemID, 18)
	synth.Build.AddLiquid(ozoneLiquidID, 20)
	placeTestBuilding(t, w, 10, 18, 421, 1, 0)
	placeTestBuilding(t, w, 10, 16, 422, 1, 0)
	w.powerStorageState[int32(18*model.Width+10)] = 4000
	linkPowerNode(t, w, 10, 16, protocol.Point2{X: 0, Y: -6}, protocol.Point2{X: 0, Y: 2})

	stepForSeconds(w, 2)
	if got := synth.Build.ItemAmount(phaseFabricItemID); got != 0 {
		t.Fatalf("expected unheated phase synthesizer to stay idle, amount=%d", got)
	}

	west := placeTestBuilding(t, w, 7, 10, 462, 1, 0)
	east := placeTestBuilding(t, w, 13, 10, 462, 1, 2)
	north := placeTestBuilding(t, w, 10, 7, 462, 1, 1)
	south := placeTestBuilding(t, w, 10, 13, 462, 1, 3)
	west.Build.AddLiquid(slagLiquidID, 120)
	east.Build.AddLiquid(slagLiquidID, 120)
	north.Build.AddLiquid(slagLiquidID, 120)
	south.Build.AddLiquid(slagLiquidID, 120)

	stepForSeconds(w, 3)

	if got := synth.Build.ItemAmount(phaseFabricItemID); got <= 0 {
		t.Fatalf("expected heated phase synthesizer to craft phase fabric, amount=%d heat=%f", got, w.crafterReceivedHeatLocked(int32(10*model.Width+10), synth))
	}
	if heat := w.crafterReceivedHeatLocked(int32(10*model.Width+10), synth); heat < 32 {
		t.Fatalf("expected phase synthesizer to receive vanilla heat requirement, heat=%f", heat)
	}
}

func TestHeatReactorCraftsFissileMatterAndHeat(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(18, 18)
	model.BlockNames = map[int16]string{
		469: "heat-reactor",
	}
	w.SetModel(model)

	reactor := placeTestBuilding(t, w, 8, 8, 469, 1, 0)
	reactor.Build.AddItem(thoriumItemID, 6)
	reactor.Build.AddLiquid(nitrogenLiquidID, 10)

	stepForSeconds(w, 10.2)

	if got := reactor.Build.ItemAmount(fissileMatterItemID); got <= 0 {
		t.Fatalf("expected heat reactor to craft fissile matter, amount=%d", got)
	}
	if heat := w.heatStates[int32(8*model.Width+8)]; heat <= 0 {
		t.Fatalf("expected heat reactor to output heat while active, heat=%f", heat)
	}
}

func TestTurbineCondenserProducesPowerAndWaterOnSteam(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(20, 20)
	model.BlockNames = map[int16]string{
		31:  "rhyolite-vent",
		421: "battery",
		422: "power-node",
		470: "turbine-condenser",
	}
	w.SetModel(model)
	paintAreaFloor(t, w, 8, 8, 3, 31)

	condenser := placeTestBuilding(t, w, 8, 8, 470, 1, 0)
	placeTestBuilding(t, w, 8, 13, 421, 1, 0)
	placeTestBuilding(t, w, 8, 11, 422, 1, 0)
	w.powerStorageState[int32(13*model.Width+8)] = 0
	linkPowerNode(t, w, 8, 11, protocol.Point2{X: 0, Y: -3}, protocol.Point2{X: 0, Y: 2})

	stepForSeconds(w, 3)

	if got := condenser.Build.LiquidAmount(waterLiquidID); got <= 0 {
		t.Fatalf("expected turbine condenser to output water on steam, amount=%f", got)
	}
	if st := w.teamPowerStates[1]; st == nil || st.Stored <= 0 || st.Produced <= 0 {
		t.Fatalf("expected turbine condenser to produce and store power, state=%+v", st)
	}
}

func TestChemicalCombustionChamberConsumesLiquidsForPower(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(18, 18)
	model.BlockNames = map[int16]string{
		421: "battery",
		422: "power-node",
		471: "chemical-combustion-chamber",
	}
	w.SetModel(model)

	chamber := placeTestBuilding(t, w, 8, 8, 471, 1, 0)
	chamber.Build.AddLiquid(ozoneLiquidID, 20)
	chamber.Build.AddLiquid(arkyciteLiquidID, 80)
	placeTestBuilding(t, w, 8, 13, 421, 1, 0)
	placeTestBuilding(t, w, 8, 11, 422, 1, 0)
	w.powerStorageState[int32(13*model.Width+8)] = 0
	linkPowerNode(t, w, 8, 11, protocol.Point2{X: 0, Y: -3}, protocol.Point2{X: 0, Y: 2})

	stepForSeconds(w, 2)

	if st := w.teamPowerStates[1]; st == nil || st.Stored <= 0 || st.Produced <= 0 {
		t.Fatalf("expected chemical combustion chamber to produce power, state=%+v", st)
	}
	if got := chamber.Build.LiquidAmount(ozoneLiquidID); got >= 20 {
		t.Fatalf("expected chemical combustion chamber to consume ozone, amount=%f", got)
	}
	if got := chamber.Build.LiquidAmount(arkyciteLiquidID); got >= 80 {
		t.Fatalf("expected chemical combustion chamber to consume arkycite, amount=%f", got)
	}
}

func TestPyrolysisGeneratorProducesPowerAndWater(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(18, 18)
	model.BlockNames = map[int16]string{
		421: "battery",
		422: "power-node",
		472: "pyrolysis-generator",
	}
	w.SetModel(model)

	gen := placeTestBuilding(t, w, 8, 8, 472, 1, 0)
	gen.Build.AddLiquid(slagLiquidID, 60)
	gen.Build.AddLiquid(arkyciteLiquidID, 100)
	placeTestBuilding(t, w, 8, 13, 421, 1, 0)
	placeTestBuilding(t, w, 8, 11, 422, 1, 0)
	w.powerStorageState[int32(13*model.Width+8)] = 0
	linkPowerNode(t, w, 8, 11, protocol.Point2{X: 0, Y: -3}, protocol.Point2{X: 0, Y: 2})

	stepForSeconds(w, 2)

	if st := w.teamPowerStates[1]; st == nil || st.Stored <= 0 || st.Produced <= 0 {
		t.Fatalf("expected pyrolysis generator to produce power, state=%+v", st)
	}
	if got := gen.Build.LiquidAmount(waterLiquidID); got <= 0 {
		t.Fatalf("expected pyrolysis generator to output water, amount=%f", got)
	}
}

func TestFluxReactorRequiresHeatToProducePower(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		421: "battery",
		422: "power-node",
		473: "flux-reactor",
	}
	w.SetModel(model)

	reactor := placeTestBuilding(t, w, 10, 10, 473, 1, 0)
	reactor.Build.AddLiquid(cyanogenLiquidID, 30)
	placeTestBuilding(t, w, 10, 18, 421, 1, 0)
	placeTestBuilding(t, w, 10, 15, 422, 1, 0)
	linkPowerNode(t, w, 10, 15, protocol.Point2{X: 0, Y: -5}, protocol.Point2{X: 0, Y: 3})

	w.Step(time.Second / 60)

	if st := w.teamPowerStates[1]; st != nil && st.Stored > 0 {
		t.Fatalf("expected flux reactor without heat to stay idle, state=%+v", st)
	}
	if got := reactor.Build.LiquidAmount(cyanogenLiquidID); got != 30 {
		t.Fatalf("expected flux reactor without heat to preserve coolant, amount=%f", got)
	}
}

func TestFluxReactorConsumesCyanogenAndProducesPowerWithHeat(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		421: "battery",
		422: "power-node",
		465: "heat-redirector",
		473: "flux-reactor",
	}
	w.SetModel(model)

	reactor := placeTestBuilding(t, w, 10, 10, 473, 1, 0)
	reactor.Build.AddLiquid(cyanogenLiquidID, 30)
	placeTestBuilding(t, w, 6, 10, 465, 1, 0)
	redirectorPos := int32(10*model.Width + 6)
	w.heatStates[redirectorPos] = 150

	placeTestBuilding(t, w, 10, 18, 421, 1, 0)
	placeTestBuilding(t, w, 10, 15, 422, 1, 0)
	linkPowerNode(t, w, 10, 15, protocol.Point2{X: 0, Y: -5}, protocol.Point2{X: 0, Y: 3})

	w.Step(time.Second / 60)

	if st := w.teamPowerStates[1]; st == nil || st.Stored <= 0 || st.Produced <= 0 {
		t.Fatalf("expected heated flux reactor to produce power, state=%+v", st)
	}
	if got := reactor.Build.LiquidAmount(cyanogenLiquidID); got >= 30 {
		t.Fatalf("expected heated flux reactor to consume cyanogen, amount=%f", got)
	}
}

func TestImpactReactorWarmupDoesNotJumpToFullImmediately(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		421: "battery",
		422: "power-node",
		481: "impact-reactor",
	}
	w.SetModel(model)

	reactor := placeTestBuilding(t, w, 10, 10, 481, 1, 0)
	reactor.Build.AddItem(blastCompoundItemID, 2)
	reactor.Build.AddLiquid(cryofluidLiquidID, 80)

	placeTestBuilding(t, w, 10, 18, 421, 1, 0)
	placeTestBuilding(t, w, 10, 15, 422, 1, 0)
	linkPowerNode(t, w, 10, 15, protocol.Point2{X: 0, Y: -5}, protocol.Point2{X: 0, Y: 3})

	batteryPos := int32(18*model.Width + 10)
	w.powerStorageState[batteryPos] = 4000

	w.Step(time.Second / 60)

	reactorPos := int32(10*model.Width + 10)
	st := w.powerGeneratorState[reactorPos]
	if st == nil {
		t.Fatal("expected impact reactor runtime state to exist")
	}
	if st.Warmup <= 0 || st.Warmup >= 0.01 {
		t.Fatalf("expected impact reactor warmup to start near zero like vanilla, warmup=%f", st.Warmup)
	}
	if st.FuelFrames <= 0 || st.FuelFrames >= 140 {
		t.Fatalf("expected impact reactor fuel timer to tick down after startup, fuel=%f", st.FuelFrames)
	}
}

func TestImpactReactorWarmupDecaysWithoutStartupPower(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		481: "impact-reactor",
	}
	w.SetModel(model)

	reactor := placeTestBuilding(t, w, 10, 10, 481, 1, 0)
	reactor.Build.AddItem(blastCompoundItemID, 1)
	reactor.Build.AddLiquid(cryofluidLiquidID, 80)

	reactorPos := int32(10*model.Width + 10)
	w.powerGeneratorState[reactorPos] = &powerGeneratorState{
		FuelFrames: 60,
		Warmup:     1,
	}

	w.Step(time.Second / 60)

	st := w.powerGeneratorState[reactorPos]
	if st == nil {
		t.Fatal("expected impact reactor runtime state to persist")
	}
	if st.Warmup >= 1 {
		t.Fatalf("expected impact reactor warmup to decay without startup power, warmup=%f", st.Warmup)
	}
}

func TestNeoplasiaReactorProducesPowerHeatAndNeoplasm(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		421: "battery",
		422: "power-node",
		480: "neoplasia-reactor",
	}
	w.SetModel(model)

	reactor := placeTestBuilding(t, w, 10, 10, 480, 1, 0)
	reactor.Build.AddItem(phaseFabricItemID, 1)
	reactor.Build.AddLiquid(arkyciteLiquidID, 80)
	reactor.Build.AddLiquid(waterLiquidID, 10)

	placeTestBuilding(t, w, 10, 18, 421, 1, 0)
	placeTestBuilding(t, w, 10, 15, 422, 1, 0)
	linkPowerNode(t, w, 10, 15, protocol.Point2{X: 0, Y: -5}, protocol.Point2{X: 0, Y: 3})

	w.Step(time.Second / 60)

	if st := w.teamPowerStates[1]; st == nil || st.Produced <= 0 || st.Stored <= 0 {
		t.Fatalf("expected neoplasia reactor to produce power, state=%+v", st)
	}
	if heat := w.heatStates[int32(10*model.Width+10)]; heat <= 0 {
		t.Fatalf("expected neoplasia reactor to produce heat, heat=%f", heat)
	}
	if got := reactor.Build.LiquidAmount(neoplasmLiquidID); got <= 0 {
		t.Fatalf("expected neoplasia reactor to output neoplasm, amount=%f", got)
	}
}

func TestNeoplasiaReactorExplodesWhenNeoplasmFills(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		480: "neoplasia-reactor",
	}
	w.SetModel(model)

	reactor := placeTestBuilding(t, w, 10, 10, 480, 1, 0)
	reactor.Build.AddItem(phaseFabricItemID, 1)
	reactor.Build.AddLiquid(arkyciteLiquidID, 80)
	reactor.Build.AddLiquid(waterLiquidID, 10)
	reactor.Build.AddLiquid(neoplasmLiquidID, 79.9)

	w.Step(time.Second / 60)

	if reactor.Block != 0 || reactor.Build != nil {
		t.Fatalf("expected neoplasia reactor to be destroyed on full neoplasm, block=%d build=%v", reactor.Block, reactor.Build)
	}
}

func TestIncineratorBurnsItemsWhenPowered(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		421: "battery",
		422: "power-node",
		459: "incinerator",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 4, 6, 257, 1, 0)
	placeTestBuilding(t, w, 6, 10, 421, 1, 0)
	placeTestBuilding(t, w, 6, 8, 422, 1, 0)
	placeTestBuilding(t, w, 6, 6, 459, 1, 0)
	w.powerStorageState[int32(10*model.Width+6)] = 4000
	linkPowerNode(t, w, 6, 8, protocol.Point2{X: 0, Y: -2}, protocol.Point2{X: 0, Y: 2})

	stepForSeconds(w, 1)

	incPos := int32(6*model.Width + 6)
	srcPos := int32(6*model.Width + 4)
	if heat := w.incineratorStates[incPos]; heat <= 0.5 {
		t.Fatalf("expected powered incinerator to heat up, heat=%f", heat)
	}
	if !w.tryInsertItemLocked(srcPos, incPos, copperItemID, 0) {
		t.Fatalf("expected hot incinerator to accept and burn item")
	}
	if got := totalBuildingItems(w.Model().Tiles[incPos].Build); got != 0 {
		t.Fatalf("expected incinerator to keep no item inventory, total=%d", got)
	}
}

func TestIncineratorBurnsLiquidsWhenPowered(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.BlockNames = map[int16]string{
		500: "conduit",
		421: "battery",
		422: "power-node",
		459: "incinerator",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 4, 6, 500, 1, 0)
	placeTestBuilding(t, w, 6, 10, 421, 1, 0)
	placeTestBuilding(t, w, 6, 8, 422, 1, 0)
	placeTestBuilding(t, w, 6, 6, 459, 1, 0)
	w.powerStorageState[int32(10*model.Width+6)] = 4000
	linkPowerNode(t, w, 6, 8, protocol.Point2{X: 0, Y: -2}, protocol.Point2{X: 0, Y: 2})

	stepForSeconds(w, 1)

	incPos := int32(6*model.Width + 6)
	srcPos := int32(6*model.Width + 4)
	moved := w.tryMoveLiquidLocked(srcPos, incPos, waterLiquidID, 5, 0)
	if moved <= 0 {
		t.Fatalf("expected hot incinerator to accept and burn liquid, moved=%f", moved)
	}
	if got := totalBuildingLiquids(w.Model().Tiles[incPos].Build); got != 0 {
		t.Fatalf("expected incinerator to keep no liquid inventory, total=%f", got)
	}
}

func TestPowerDiodeTransfersBatteryChargeOneWay(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.BlockNames = map[int16]string{
		421: "battery",
		422: "diode",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 4, 6, 421, 1, 0)
	placeTestBuilding(t, w, 5, 6, 422, 1, 0)
	placeTestBuilding(t, w, 6, 6, 421, 1, 0)

	backPos := int32(6*model.Width + 4)
	frontPos := int32(6*model.Width + 6)
	w.powerStorageState[backPos] = 3000
	w.powerStorageState[frontPos] = 0

	w.Step(time.Second / 60)

	if got := w.powerStorageState[frontPos]; got <= 0 {
		t.Fatalf("expected diode to move power into front graph, stored=%f", got)
	}
	if got := w.powerStorageState[backPos]; got >= 3000 {
		t.Fatalf("expected diode to drain some back-graph power, stored=%f", got)
	}
}

func TestBeamNodeConnectsBatteryToPoweredConsumer(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(20, 20)
	model.BlockNames = map[int16]string{
		421: "battery",
		430: "laser-drill",
		474: "beam-node",
		2:   "ore-copper",
	}
	w.SetModel(model)
	paintAreaOverlay(t, w, 14, 10, 3, 2)

	placeTestBuilding(t, w, 4, 10, 421, 1, 0)
	drill := placeTestBuilding(t, w, 14, 10, 430, 1, 0)
	placeTestBuilding(t, w, 10, 10, 474, 1, 0)
	w.powerStorageState[int32(10*model.Width+4)] = 3000

	stepForSeconds(w, 3)

	if got := totalBuildingItems(drill.Build); got <= 0 {
		t.Fatalf("expected beam node to power laser drill from battery, items=%d", got)
	}
}

func TestBeamNodeBlockedByPlastaniumWall(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(20, 20)
	model.BlockNames = map[int16]string{
		421: "battery",
		430: "laser-drill",
		474: "beam-node",
		475: "plastanium-wall",
		2:   "ore-copper",
	}
	w.SetModel(model)
	paintAreaOverlay(t, w, 14, 10, 3, 2)

	placeTestBuilding(t, w, 4, 10, 421, 1, 0)
	drill := placeTestBuilding(t, w, 14, 10, 430, 1, 0)
	placeTestBuilding(t, w, 10, 10, 474, 1, 0)
	placeTestBuilding(t, w, 12, 10, 475, 1, 0)
	w.powerStorageState[int32(10*model.Width+4)] = 3000

	stepForSeconds(w, 3)

	if got := totalBuildingItems(drill.Build); got != 0 {
		t.Fatalf("expected plastanium wall to block beam node power transfer, items=%d", got)
	}
}

func TestBeamTowerProvidesLargeBufferedStorage(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		420: "solar-panel-large",
		476: "beam-tower",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 6, 10, 420, 1, 0)
	placeTestBuilding(t, w, 12, 10, 476, 1, 0)

	stepForSeconds(w, 10)

	if st := w.teamPowerStates[1]; st == nil || st.Capacity < 40000 || st.Stored <= 0 {
		t.Fatalf("expected beam tower to contribute large power storage, state=%+v", st)
	}
}

func TestPowerNodeLargeAutoLinksOnPlacement(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		421: "battery",
		425: "power-node-large",
	}
	w.SetModel(model)

	batteryTile, err := model.TileAt(5, 10)
	if err != nil || batteryTile == nil {
		t.Fatalf("battery tile lookup failed: %v", err)
	}
	w.placeTileLocked(batteryTile, 1, 421, 0, nil, 0)

	nodeTile, err := model.TileAt(12, 10)
	if err != nil || nodeTile == nil {
		t.Fatalf("node tile lookup failed: %v", err)
	}
	w.placeTileLocked(nodeTile, 1, 425, 0, nil, 0)

	nodePos := int32(10*model.Width + 12)
	batteryPos := int32(10*model.Width + 5)
	links := w.powerNodeLinks[nodePos]
	if len(links) == 0 {
		t.Fatal("expected power-node-large to autolink on placement")
	}
	found := false
	for _, link := range links {
		if link == batteryPos {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected power-node-large to autolink to nearby battery, links=%v", links)
	}
}

func TestPowerNodeLargeAutoLinksNearbyConsumerOnPlacement(t *testing.T) {
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
	links := w.powerNodeLinks[nodePos]
	if len(links) == 0 {
		t.Fatal("expected power-node-large to autolink nearby consumer on placement")
	}
	found := false
	for _, link := range links {
		if link == consumerPos {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected power-node-large to link nearby consumer, links=%v", links)
	}

	cfg, ok := w.BuildingConfigPacked(protocol.PackPoint2(12, 10))
	if !ok {
		t.Fatal("expected autolinked power node config to be readable")
	}
	points, ok := cfg.([]protocol.Point2)
	if !ok {
		t.Fatalf("expected power node config as []Point2, got %T", cfg)
	}
	found = false
	for _, point := range points {
		if point.X == -6 && point.Y == 0 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected power node config to include relative consumer link (-6,0), got %#v", points)
	}
}

func TestPowerNodeLargeAutoLinkAvoidsDuplicateConductingGraph(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		421: "battery",
		425: "power-node-large",
		430: "laser-drill",
	}
	w.SetModel(model)

	batteryTile, err := model.TileAt(5, 10)
	if err != nil || batteryTile == nil {
		t.Fatalf("battery tile lookup failed: %v", err)
	}
	w.placeTileLocked(batteryTile, 1, 421, 0, nil, 0)

	drillTile, err := model.TileAt(7, 10)
	if err != nil || drillTile == nil {
		t.Fatalf("drill tile lookup failed: %v", err)
	}
	w.placeTileLocked(drillTile, 1, 430, 0, nil, 0)

	nodeTile, err := model.TileAt(12, 10)
	if err != nil || nodeTile == nil {
		t.Fatalf("node tile lookup failed: %v", err)
	}
	w.placeTileLocked(nodeTile, 1, 425, 0, nil, 0)

	nodePos := int32(10*model.Width + 12)
	drillPos := int32(10*model.Width + 7)
	links := w.powerNodeLinks[nodePos]
	if len(links) != 1 {
		t.Fatalf("expected one autolink target for the shared conducting graph, got %v", links)
	}
	if links[0] != drillPos {
		t.Fatalf("expected nearer drill %d to represent the shared conducting graph, got %v", drillPos, links)
	}
}

func TestPowerNodeLargeAutoLinkBlockedByPlastaniumWall(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		425: "power-node-large",
		430: "laser-drill",
		475: "plastanium-wall",
	}
	w.SetModel(model)

	consumerTile, err := model.TileAt(6, 10)
	if err != nil || consumerTile == nil {
		t.Fatalf("consumer tile lookup failed: %v", err)
	}
	w.placeTileLocked(consumerTile, 1, 430, 0, nil, 0)

	wallTile, err := model.TileAt(9, 10)
	if err != nil || wallTile == nil {
		t.Fatalf("wall tile lookup failed: %v", err)
	}
	w.placeTileLocked(wallTile, 1, 475, 0, nil, 0)

	nodeTile, err := model.TileAt(12, 10)
	if err != nil || nodeTile == nil {
		t.Fatalf("node tile lookup failed: %v", err)
	}
	w.placeTileLocked(nodeTile, 1, 425, 0, nil, 0)

	nodePos := int32(10*model.Width + 12)
	if links := w.powerNodeLinks[nodePos]; len(links) != 0 {
		t.Fatalf("expected plastanium wall to block power-node autolink, links=%v", links)
	}
}

func TestAutoLinkedPowerNodeEmitsConfigEvent(t *testing.T) {
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

	nodePacked := protocol.PackPoint2(12, 10)
	targetPacked := protocol.PackPoint2(6, 10)
	for _, ev := range w.DrainEntityEvents() {
		if ev.Kind != EntityEventBuildConfig || ev.BuildPos != nodePacked {
			continue
		}
		target, ok := ev.BuildConfig.(int32)
		if !ok {
			t.Fatalf("expected power node config event payload as packed int32, got %T", ev.BuildConfig)
		}
		if target != targetPacked {
			t.Fatalf("expected packed target=%d, got %d", targetPacked, target)
		}
		return
	}
	t.Fatal("expected autolinked power node to emit build_config event")
}

func TestAutoLinkedPowerNodeConfigEventComesAfterConstructed(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		425: "power-node-large",
		430: "laser-drill",
	}
	w.SetModel(model)

	consumerTile, err := model.TileAt(6, 10)
	if err != nil || consumerTile == nil {
		t.Fatalf("consumer tile lookup failed: %v", err)
	}
	w.placeTileLocked(consumerTile, 1, 430, 0, nil, 0)
	_ = w.DrainEntityEvents()

	nodeTile, err := model.TileAt(12, 10)
	if err != nil || nodeTile == nil {
		t.Fatalf("node tile lookup failed: %v", err)
	}
	w.placeTileLocked(nodeTile, 1, 425, 0, nil, 0)

	nodePacked := protocol.PackPoint2(12, 10)
	constructIndex := -1
	configIndex := -1
	for i, ev := range w.DrainEntityEvents() {
		if ev.BuildPos != nodePacked {
			continue
		}
		if ev.Kind == EntityEventBuildConstructed && constructIndex < 0 {
			constructIndex = i
		}
		if ev.Kind == EntityEventBuildConfig && configIndex < 0 {
			configIndex = i
		}
	}
	if constructIndex < 0 {
		t.Fatal("expected power node constructed event")
	}
	if configIndex < 0 {
		t.Fatal("expected power node build_config event")
	}
	if configIndex <= constructIndex {
		t.Fatalf("expected build_config after build_constructed, got constructed=%d config=%d", constructIndex, configIndex)
	}
}

func TestBuildStepDoesNotDeadlockWhenPowerNodeAutoLinks(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		339: "core-shard",
		421: "battery",
		425: "power-node-large",
	}
	w.SetModel(model)

	core := placeTestBuilding(t, w, 0, 0, 339, 1, 0)
	core.Build.AddItem(0, 100)
	placeTestBuilding(t, w, 6, 10, 421, 1, 0)

	owner := int32(101)
	team := TeamID(1)
	w.UpdateBuilderState(owner, team, 9001, float32(12*8+4), float32(10*8+4), true, 220)
	w.ApplyBuildPlanSnapshotForOwner(owner, team, []BuildPlanOp{{
		X: 12, Y: 10, BlockID: 425,
	}})

	done := make(chan struct{})
	go func() {
		w.Step(200 * time.Millisecond)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("step deadlocked while constructing autolinked power node")
	}
}

func TestBeamLinkTransfersPowerAcrossLongDistance(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(80, 20)
	model.BlockNames = map[int16]string{
		421: "battery",
		422: "power-node",
		430: "laser-drill",
		477: "beam-link",
		2:   "ore-copper",
	}
	w.SetModel(model)
	paintAreaOverlay(t, w, 70, 10, 3, 2)

	placeTestBuilding(t, w, 6, 10, 421, 1, 0)
	placeTestBuilding(t, w, 8, 10, 422, 1, 0)
	placeTestBuilding(t, w, 10, 10, 477, 1, 0)
	placeTestBuilding(t, w, 60, 10, 477, 1, 0)
	placeTestBuilding(t, w, 66, 10, 422, 1, 0)
	drill := placeTestBuilding(t, w, 70, 10, 430, 1, 0)
	w.powerStorageState[int32(10*model.Width+6)] = 3000
	w.applyBuildingConfigLocked(int32(10*model.Width+10), []protocol.Point2{{X: 50, Y: 0}}, true)
	linkPowerNode(t, w, 8, 10, protocol.Point2{X: -2, Y: 0}, protocol.Point2{X: 2, Y: 0})
	linkPowerNode(t, w, 66, 10, protocol.Point2{X: -6, Y: 0}, protocol.Point2{X: 4, Y: 0})

	stepForSeconds(w, 3)

	if got := totalBuildingItems(drill.Build); got <= 0 {
		t.Fatalf("expected beam link to transfer power across long range, items=%d", got)
	}
}

func TestPowerSourcePowersLaserDrill(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(20, 20)
	model.BlockNames = map[int16]string{
		2:   "ore-copper",
		430: "laser-drill",
		478: "power-source",
	}
	w.SetModel(model)
	paintAreaOverlay(t, w, 10, 8, 3, 2)

	drill := placeTestBuilding(t, w, 10, 8, 430, 1, 0)
	placeTestBuilding(t, w, 10, 12, 478, 1, 0)
	linkPowerNode(t, w, 10, 12, protocol.Point2{X: 0, Y: -4})

	stepForSeconds(w, 3)

	if got := totalBuildingItems(drill.Build); got <= 0 {
		t.Fatalf("expected power source to power laser drill, items=%d", got)
	}
}

func TestPowerVoidDrainsNetworkStorage(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(20, 20)
	model.BlockNames = map[int16]string{
		420: "solar-panel-large",
		421: "battery",
		422: "power-node",
		479: "power-void",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 4, 8, 420, 1, 0)
	placeTestBuilding(t, w, 8, 8, 421, 1, 0)
	placeTestBuilding(t, w, 6, 8, 422, 1, 0)
	placeTestBuilding(t, w, 6, 11, 479, 1, 0)
	linkPowerNode(t, w, 6, 8, protocol.Point2{X: -2, Y: 0}, protocol.Point2{X: 2, Y: 0}, protocol.Point2{X: 0, Y: 3})

	stepForSeconds(w, 10)

	if st := w.teamPowerStates[1]; st == nil || st.Produced <= 0 {
		t.Fatalf("expected power void network to still produce power, state=%+v", st)
	} else {
		if st.Stored != 0 {
			t.Fatalf("expected power void to drain all stored power, state=%+v", st)
		}
		if st.Consumed <= 0 {
			t.Fatalf("expected power void to consume network power, state=%+v", st)
		}
	}
}

