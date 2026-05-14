package world

import (
	"math"
	"testing"
	"time"

	"github.com/IYanHua/mdt-server/internal/protocol"
)

func TestGroundAIAdvancesTowardEnemyCore(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		339: "core-shard",
	}
	model.UnitNames = map[int16]string{
		1: "dagger",
	}
	w.SetModel(model)
	w.unitRuntimeProfilesByName["dagger"] = unitRuntimeProfile{Name: "dagger", Speed: 24}

	core := placeTestBuilding(t, w, 3, 12, 339, 1, 0)
	ent := w.Model().AddEntity(RawEntity{
		TypeID:      1,
		X:           float32(20*8 + 4),
		Y:           float32(12*8 + 4),
		Team:        2,
		MineTilePos: invalidEntityTilePos,
	})

	coreX := float32(core.X*8 + 4)
	coreY := float32(core.Y*8 + 4)
	before := float32(math.Hypot(float64(ent.X-coreX), float64(ent.Y-coreY)))
	stepForSeconds(w, 3)
	got := findTestEntity(t, w, ent.ID)
	after := float32(math.Hypot(float64(got.X-coreX), float64(got.Y-coreY)))
	if after >= before-16 {
		t.Fatalf("expected ground AI to advance toward enemy core, before=%f after=%f", before, after)
	}
}

func TestGroundAIPathsAroundBlockingBuildings(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		339: "core-shard",
		600: "test-wall",
	}
	model.UnitNames = map[int16]string{
		1: "dagger",
	}
	w.SetModel(model)
	w.unitRuntimeProfilesByName["dagger"] = unitRuntimeProfile{Name: "dagger", Speed: 24}

	placeTestBuilding(t, w, 3, 12, 339, 1, 0)
	for y := 0; y < 24; y++ {
		if y == 4 {
			continue
		}
		placeTestBuilding(t, w, 10, y, 600, 1, 0)
	}

	ent := w.Model().AddEntity(RawEntity{
		TypeID:      1,
		X:           float32(20*8 + 4),
		Y:           float32(12*8 + 4),
		Team:        2,
		MineTilePos: invalidEntityTilePos,
	})

	stepForSeconds(w, 9)
	got := findTestEntity(t, w, ent.ID)
	if got.X >= float32(10*8+4) {
		t.Fatalf("expected ground AI to route around wall line, got x=%f y=%f", got.X, got.Y)
	}
}

func TestFlyingAIIgnoresGroundWallPathing(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		339: "core-shard",
		600: "test-wall",
	}
	model.UnitNames = map[int16]string{
		2: "flare",
	}
	w.SetModel(model)
	w.unitRuntimeProfilesByName["flare"] = unitRuntimeProfile{Name: "flare", Speed: 28, Flying: true}

	placeTestBuilding(t, w, 3, 12, 339, 1, 0)
	for y := 0; y < 24; y++ {
		placeTestBuilding(t, w, 10, y, 600, 1, 0)
	}

	startX := float32(20*8 + 4)
	startY := float32(12*8 + 4)
	ent := w.Model().AddEntity(RawEntity{
		TypeID:      2,
		X:           startX,
		Y:           startY,
		Team:        2,
		MineTilePos: invalidEntityTilePos,
	})

	stepForSeconds(w, 3)
	got := findTestEntity(t, w, ent.ID)
	if got.X >= startX-24 {
		t.Fatalf("expected flying AI to keep advancing through wall line, startX=%f gotX=%f", startX, got.X)
	}
	if math.Abs(float64(got.Y-startY)) > 12 {
		t.Fatalf("expected flying AI to keep a mostly direct line, startY=%f gotY=%f", startY, got.Y)
	}
}

func TestNavalAIIgnoresGroundWallPathing(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		339: "core-shard",
		600: "test-wall",
	}
	model.UnitNames = map[int16]string{
		25: "minke",
	}
	w.SetModel(model)
	w.unitRuntimeProfilesByName["minke"] = unitRuntimeProfile{Name: "minke", Speed: 24}

	placeTestBuilding(t, w, 3, 12, 339, 1, 0)
	for y := 0; y < 24; y++ {
		placeTestBuilding(t, w, 10, y, 600, 1, 0)
	}

	startX := float32(20*8 + 4)
	startY := float32(12*8 + 4)
	ent := w.Model().AddEntity(RawEntity{
		TypeID:      25,
		X:           startX,
		Y:           startY,
		Team:        2,
		MineTilePos: invalidEntityTilePos,
	})

	stepForSeconds(w, 3)
	got := findTestEntity(t, w, ent.ID)
	if got.X >= startX-24 {
		t.Fatalf("expected naval AI to keep advancing without ground A*, startX=%f gotX=%f", startX, got.X)
	}
	if math.Abs(float64(got.Y-startY)) > 12 {
		t.Fatalf("expected naval AI to keep a mostly direct line, startY=%f gotY=%f", startY, got.Y)
	}
}

func TestFlyingFollowAITracksNearbyAlly(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		339: "core-shard",
	}
	model.UnitNames = map[int16]string{
		10: "quell",
		11: "mace",
	}
	w.SetModel(model)
	w.unitRuntimeProfilesByName["quell"] = unitRuntimeProfile{Name: "quell", Speed: 28, Flying: true}
	w.unitRuntimeProfilesByName["mace"] = unitRuntimeProfile{Name: "mace", Speed: 6}

	placeTestBuilding(t, w, 3, 12, 339, 1, 0)
	ally := w.Model().AddEntity(RawEntity{
		TypeID:      11,
		X:           float32(20*8 + 4),
		Y:           float32(18*8 + 4),
		Team:        2,
		Health:      5000,
		MaxHealth:   5000,
		MineTilePos: invalidEntityTilePos,
	})
	startX := float32(20*8 + 4)
	startY := float32(8*8 + 4)
	follower := w.Model().AddEntity(RawEntity{
		TypeID:      10,
		X:           startX,
		Y:           startY,
		Team:        2,
		MineTilePos: invalidEntityTilePos,
	})

	before := float32(math.Hypot(float64(startX-ally.X), float64(startY-ally.Y)))
	stepForSeconds(w, 2)
	got := findTestEntity(t, w, follower.ID)
	after := float32(math.Hypot(float64(got.X-ally.X), float64(got.Y-ally.Y)))
	if after >= before-20 {
		t.Fatalf("expected FlyingFollowAI to close on ally before free-flying, before=%f after=%f", before, after)
	}
}

func TestBuilderAIExecutesEntityPlansWithoutExternalBuilderState(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		45:  "duo",
		339: "core-shard",
	}
	model.UnitNames = map[int16]string{
		35: "alpha",
	}
	w.SetModel(model)
	w.blockBuildTimesByName = map[string]float32{"duo": 1.5}

	core := placeTestBuilding(t, w, 0, 0, 339, 1, 0)
	core.Build.AddItem(0, 100)
	if _, err := w.AddEntityWithID(35, 9101, 20, 20, 1); err != nil {
		t.Fatalf("add alpha entity: %v", err)
	}
	for i := range w.Model().Entities {
		if w.Model().Entities[i].ID != 9101 {
			continue
		}
		w.Model().Entities[i].UpdateBuilding = true
		w.Model().Entities[i].Plans = []entityBuildPlan{{
			Pos:     packTilePos(2, 2),
			BlockID: 45,
		}}
		break
	}

	built := false
	for i := 0; i < 200; i++ {
		w.Step(time.Second / 60)
		tile, _ := w.Model().TileAt(2, 2)
		if tile.Block == 45 && tile.Build != nil {
			built = true
			break
		}
	}
	if !built {
		tile, _ := w.Model().TileAt(2, 2)
		t.Fatalf("expected builder AI plans to construct duo, got block=%d build=%v", tile.Block, tile.Build != nil)
	}
}

func TestBuilderAIStaysIdleWithoutThreat(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		339: "core-shard",
	}
	model.UnitNames = map[int16]string{
		35: "alpha",
	}
	w.SetModel(model)

	core := placeTestBuilding(t, w, 3, 3, 339, 1, 0)
	if _, err := w.AddEntityWithID(35, 9102, 20*8+4, 20*8+4, 1); err != nil {
		t.Fatalf("add alpha entity: %v", err)
	}
	start := findTestEntity(t, w, 9102)
	stepForSeconds(w, 3)
	got := findTestEntity(t, w, 9102)
	if moved := float32(math.Hypot(float64(got.X-start.X), float64(got.Y-start.Y))); moved > 0.001 {
		t.Fatalf("expected idle builder AI to stay put without threats, moved=%f start=(%f,%f) got=(%f,%f)", moved, start.X, start.Y, got.X, got.Y)
	}
	coreX := float32(core.X*8 + 4)
	coreY := float32(core.Y*8 + 4)
	before := float32(math.Hypot(float64(start.X-coreX), float64(start.Y-coreY)))
	after := float32(math.Hypot(float64(got.X-coreX), float64(got.Y-coreY)))
	if math.Abs(float64(after-before)) > 0.001 {
		t.Fatalf("expected idle builder AI to keep its core distance without threats, before=%f after=%f", before, after)
	}
}

func TestBuilderAIAlwaysFleeRetreatsToFriendlyCoreWhenThreatened(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		45:  "duo",
		339: "core-shard",
	}
	model.UnitNames = map[int16]string{
		1:  "dagger",
		35: "alpha",
	}
	w.SetModel(model)
	w.unitRuntimeProfilesByName["dagger"] = unitRuntimeProfile{Name: "dagger", Speed: 24}

	core := placeTestBuilding(t, w, 3, 3, 339, 1, 0)
	if _, err := w.AddEntityWithID(35, 9103, 20*8+4, 20*8+4, 1); err != nil {
		t.Fatalf("add alpha entity: %v", err)
	}
	for i := range w.Model().Entities {
		if w.Model().Entities[i].ID != 9103 {
			continue
		}
		w.Model().Entities[i].UpdateBuilding = true
		w.Model().Entities[i].Plans = []entityBuildPlan{{
			Pos:     packTilePos(20, 20),
			BlockID: 45,
		}}
		break
	}
	w.Model().AddEntity(RawEntity{
		TypeID:      1,
		X:           18*8 + 4,
		Y:           20*8 + 4,
		Team:        2,
		Health:      100,
		MaxHealth:   100,
		MoveSpeed:   24,
		MineTilePos: invalidEntityTilePos,
	})

	start := findTestEntity(t, w, 9103)
	coreX := float32(core.X*8 + 4)
	coreY := float32(core.Y*8 + 4)
	before := float32(math.Hypot(float64(start.X-coreX), float64(start.Y-coreY)))
	stepForSeconds(w, 1)
	got := findTestEntity(t, w, 9103)
	after := float32(math.Hypot(float64(got.X-coreX), float64(got.Y-coreY)))
	if after >= before {
		t.Fatalf("expected threatened builder AI to retreat toward friendly core, before=%f after=%f", before, after)
	}
	if len(got.Plans) != 0 {
		t.Fatalf("expected threatened builder AI to clear build plans while fleeing, got %v", got.Plans)
	}
}

func TestBuilderAIFollowsNearbyConstructBuilderBeforeRebuildQueue(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(64, 64)
	model.BlockNames = map[int16]string{
		45:  "duo",
		339: "core-shard",
	}
	model.UnitNames = map[int16]string{
		35: "alpha",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 0, 0, 339, 1, 0)
	leader := w.Model().AddEntity(RawEntity{
		ID:           9201,
		TypeID:       35,
		PlayerID:     1,
		X:            140,
		Y:            140,
		Team:         1,
		Health:       100,
		MaxHealth:    100,
		MoveSpeed:    24,
		BuildSpeed:   0.5,
		ItemCapacity: 30,
		Flying:       true,
	})
	op := BuildPlanOp{X: 18, Y: 18, Rotation: 0, BlockID: 45}
	primeAssistConstructBuilder(t, w, leader, op)
	w.teamRebuildPlans[1] = []rebuildBlockPlan{{
		X:       30,
		Y:       30,
		BlockID: 45,
	}}
	if _, err := w.AddEntityWithID(35, 9202, 18*8+4, 16*8+4, 1); err != nil {
		t.Fatalf("add follower alpha entity: %v", err)
	}

	w.Step(time.Second / 60)
	w.Step(time.Second / 60)

	got := findTestEntity(t, w, 9202)
	if len(got.Plans) == 0 {
		t.Fatal("expected nearby builder AI to copy active construct leader plan")
	}
	if got.Plans[0].Breaking {
		t.Fatalf("expected copied construct plan to remain a build plan, got %+v", got.Plans[0])
	}
	if got.Plans[0].Pos != packTilePos(18, 18) {
		t.Fatalf("expected nearby builder AI to follow construct leader plan at (18,18), got %+v", got.Plans[0])
	}
}

func TestBuilderAIFollowsNearbyDeconstructBuilder(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(64, 64)
	model.BlockNames = map[int16]string{
		45:  "duo",
		339: "core-shard",
	}
	model.UnitNames = map[int16]string{
		35: "alpha",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 0, 0, 339, 1, 0)
	placeTestBuilding(t, w, 18, 18, 45, 1, 0)
	leader := w.Model().AddEntity(RawEntity{
		ID:           9203,
		TypeID:       35,
		PlayerID:     1,
		X:            140,
		Y:            140,
		Team:         1,
		Health:       100,
		MaxHealth:    100,
		MoveSpeed:    24,
		BuildSpeed:   0.5,
		ItemCapacity: 30,
		Flying:       true,
	})
	op := BuildPlanOp{Breaking: true, X: 18, Y: 18}
	primeAssistConstructBuilder(t, w, leader, op)
	if _, err := w.AddEntityWithID(35, 9204, 18*8+4, 16*8+4, 1); err != nil {
		t.Fatalf("add follower alpha entity: %v", err)
	}

	w.Step(time.Second / 60)
	w.Step(time.Second / 60)

	got := findTestEntity(t, w, 9204)
	if len(got.Plans) == 0 {
		t.Fatal("expected nearby builder AI to copy active deconstruct leader plan")
	}
	if !got.Plans[0].Breaking || got.Plans[0].Pos != packTilePos(18, 18) {
		t.Fatalf("expected nearby builder AI to follow deconstruct leader plan at (18,18), got %+v", got.Plans[0])
	}
}

func TestBuilderAIRemovedQueuedRebuildPlanClearsEntityPlan(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		45:  "duo",
		339: "core-shard",
	}
	model.UnitNames = map[int16]string{
		35: "alpha",
	}
	w.SetModel(model)
	w.blockBuildTimesByName = map[string]float32{"duo": 1.5}
	rules := w.GetRulesManager().Get()
	rules.GhostBlocks = true

	core := placeTestBuilding(t, w, 0, 0, 339, 1, 0)
	core.Build.AddItem(0, 1000)
	placeTestBuilding(t, w, 10, 10, 45, 1, 0)
	if !w.DamageBuildingPacked(packTilePos(10, 10), 2000) {
		t.Fatal("expected destroyed building to enter broken-block rebuild queue")
	}
	if len(w.teamRebuildPlans[1]) != 1 {
		t.Fatalf("expected exactly one queued rebuild plan, got %d", len(w.teamRebuildPlans[1]))
	}
	if _, err := w.AddEntityWithID(35, 9205, 20, 20, 1); err != nil {
		t.Fatalf("add alpha entity: %v", err)
	}

	w.Step(time.Second / 60)
	got := findTestEntity(t, w, 9205)
	if len(got.Plans) == 0 || got.Plans[0].Pos != packTilePos(10, 10) {
		t.Fatalf("expected builder AI to pick queued rebuild plan at (10,10), got %+v", got.Plans)
	}

	delete(w.teamRebuildPlans, TeamID(1))
	stepForSeconds(w, 5)

	got = findTestEntity(t, w, 9205)
	if len(got.Plans) != 0 {
		t.Fatalf("expected builder AI to drop removed queued rebuild plan, got %+v", got.Plans)
	}
	tile, err := w.Model().TileAt(10, 10)
	if err != nil {
		t.Fatalf("tile lookup failed: %v", err)
	}
	if tile.Block != 0 || tile.Build != nil {
		t.Fatalf("expected removed queued rebuild plan to stop any hidden rebuild, got block=%d build=%v", tile.Block, tile.Build != nil)
	}
}

func TestBuilderAIQueuedRebuildPlanYieldsToPlayerBreakingSameTile(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		45:  "duo",
		339: "core-shard",
	}
	model.UnitNames = map[int16]string{
		35: "alpha",
	}
	w.SetModel(model)
	rules := w.GetRulesManager().Get()
	rules.GhostBlocks = true

	core := placeTestBuilding(t, w, 0, 0, 339, 1, 0)
	core.Build.AddItem(0, 1000)
	placeTestBuilding(t, w, 10, 10, 45, 1, 0)
	if !w.DamageBuildingPacked(packTilePos(10, 10), 2000) {
		t.Fatal("expected destroyed building to enter broken-block rebuild queue")
	}
	if _, err := w.AddEntityWithID(35, 9208, 20, 20, 1); err != nil {
		t.Fatalf("add autonomous alpha entity: %v", err)
	}

	w.Step(time.Second / 60)
	got := findTestEntity(t, w, 9208)
	if len(got.Plans) == 0 || got.Plans[0].Pos != packTilePos(10, 10) {
		t.Fatalf("expected autonomous builder to pick queued rebuild plan, got %+v", got.Plans)
	}

	playerBuilder := w.Model().AddEntity(RawEntity{
		ID:           9209,
		TypeID:       35,
		PlayerID:     7,
		X:            10*8 + 4,
		Y:            10*8 + 4,
		Team:         1,
		Health:       100,
		MaxHealth:    100,
		MoveSpeed:    24,
		BuildSpeed:   0.5,
		ItemCapacity: 30,
		Flying:       true,
	})
	w.UpdateBuilderState(playerBuilder.ID, playerBuilder.Team, playerBuilder.ID, playerBuilder.X, playerBuilder.Y, true, 220)
	if _, ok := w.SetEntityBuildState(playerBuilder.ID, true, []*protocol.BuildPlan{{
		Breaking: true,
		X:        10,
		Y:        10,
	}}); !ok {
		t.Fatal("expected player break plan to apply")
	}

	w.Step(time.Second / 60)

	got = findTestEntity(t, w, 9208)
	if len(got.Plans) != 0 {
		t.Fatalf("expected autonomous builder to yield rebuild plan to player breaking same tile, got %+v", got.Plans)
	}
	if _, ok := w.teamRebuildPlans[1]; ok {
		t.Fatalf("expected matching broken-block rebuild queue item to be cleared, got %+v", w.teamRebuildPlans[1])
	}
}

func TestBuilderAINearEnemyPlanUsesOfficialRectUnitCheck(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(96, 96)
	model.BlockNames = map[int16]string{
		45:  "duo",
		339: "core-shard",
		400: "lancer",
	}
	model.UnitNames = map[int16]string{
		1:  "dagger",
		35: "alpha",
	}
	w.SetModel(model)
	core := placeTestBuilding(t, w, 0, 0, 339, 1, 0)
	core.Build.AddItem(0, 1000)

	w.teamRebuildPlans[1] = []rebuildBlockPlan{{
		X:       10,
		Y:       10,
		BlockID: 45,
	}}
	if _, err := w.AddEntityWithID(35, 9210, 20, 20, 1); err != nil {
		t.Fatalf("add autonomous alpha entity: %v", err)
	}
	planX := float32(10*8 + 4)
	planY := float32(10*8 + 4)
	w.Model().AddEntity(RawEntity{
		ID:        9211,
		TypeID:    1,
		X:         planX + 300,
		Y:         planY,
		Team:      2,
		Health:    100,
		MaxHealth: 100,
	})

	w.Step(time.Second / 60)

	got := findTestEntity(t, w, 9210)
	if len(got.Plans) == 0 || got.Plans[0].Pos != packTilePos(10, 10) {
		t.Fatalf("expected builder AI to keep rebuild plan when enemy unit is outside official nearEnemy square, got %+v", got.Plans)
	}

	placeTestBuilding(t, w, 30, 10, 400, 2, 0)
	w.teamRebuildPlans[1] = []rebuildBlockPlan{{
		X:       10,
		Y:       10,
		BlockID: 45,
	}}
	for i := range w.Model().Entities {
		if w.Model().Entities[i].ID != 9210 {
			continue
		}
		w.Model().Entities[i].Plans = nil
		w.Model().Entities[i].UpdateBuilding = true
		break
	}

	w.Step(time.Second / 60)

	got = findTestEntity(t, w, 9210)
	if len(got.Plans) != 0 {
		t.Fatalf("expected enemy turret range to count as nearEnemy and block rebuild pickup, got %+v", got.Plans)
	}
}

func TestBuilderAIMultiBuilderUsesDistinctRebuildPlans(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(32, 32)
	model.BlockNames = map[int16]string{
		45:  "duo",
		339: "core-shard",
	}
	model.UnitNames = map[int16]string{
		35: "alpha",
	}
	w.SetModel(model)

	core := placeTestBuilding(t, w, 0, 0, 339, 1, 0)
	core.Build.AddItem(0, 1000)
	w.teamRebuildPlans[1] = []rebuildBlockPlan{
		{X: 5, Y: 5, BlockID: 45},
		{X: 9, Y: 9, BlockID: 45},
	}
	if _, err := w.AddEntityWithID(35, 9206, 20, 20, 1); err != nil {
		t.Fatalf("add first alpha entity: %v", err)
	}
	if _, err := w.AddEntityWithID(35, 9207, 24, 24, 1); err != nil {
		t.Fatalf("add second alpha entity: %v", err)
	}

	w.Step(time.Second / 60)

	first := findTestEntity(t, w, 9206)
	second := findTestEntity(t, w, 9207)
	if len(first.Plans) == 0 || len(second.Plans) == 0 {
		t.Fatalf("expected both builder AI units to acquire rebuild plans, first=%+v second=%+v", first.Plans, second.Plans)
	}
	if first.Plans[0].Pos == second.Plans[0].Pos {
		t.Fatalf("expected multi-builder rebuild queue to distribute distinct plans, both got %+v", first.Plans[0])
	}
}

func TestWaveTeamBuilderFallsBackToAdvanceLikeOfficial(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		339: "core-shard",
	}
	model.UnitNames = map[int16]string{
		35: "alpha",
	}
	w.SetModel(model)
	w.unitRuntimeProfilesByName["alpha"] = unitRuntimeProfile{Name: "alpha", Speed: 24, Flying: true}

	core := placeTestBuilding(t, w, 3, 12, 339, 1, 0)
	if _, err := w.AddEntityWithID(35, 9212, 20*8+4, 12*8+4, 2); err != nil {
		t.Fatalf("add wave-team alpha entity: %v", err)
	}

	start := findTestEntity(t, w, 9212)
	coreX := float32(core.X*8 + 4)
	coreY := float32(core.Y*8 + 4)
	before := float32(math.Hypot(float64(start.X-coreX), float64(start.Y-coreY)))
	stepForSeconds(w, 3)
	got := findTestEntity(t, w, 9212)
	after := float32(math.Hypot(float64(got.X-coreX), float64(got.Y-coreY)))
	if after >= before {
		t.Fatalf("expected wave-team builder to use fallback AI and advance toward enemy core, before=%f after=%f", before, after)
	}
}

func TestWaveTeamBuilderDoesNotFallbackWhenRtsAiEnabled(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		339: "core-shard",
	}
	model.UnitNames = map[int16]string{
		35: "alpha",
	}
	w.SetModel(model)
	w.unitRuntimeProfilesByName["alpha"] = unitRuntimeProfile{Name: "alpha", Speed: 24, Flying: true}

	rules := w.GetRulesManager().Get()
	rules.RtsAi = true

	placeTestBuilding(t, w, 3, 12, 339, 1, 0)
	if _, err := w.AddEntityWithID(35, 9213, 20*8+4, 12*8+4, 2); err != nil {
		t.Fatalf("add wave-team alpha entity: %v", err)
	}

	start := findTestEntity(t, w, 9213)
	stepForSeconds(w, 3)
	got := findTestEntity(t, w, 9213)
	if moved := float32(math.Hypot(float64(got.X-start.X), float64(got.Y-start.Y))); moved > 0.001 {
		t.Fatalf("expected wave-team builder to keep builder AI when rtsAi is enabled, moved=%f start=(%f,%f) got=(%f,%f)", moved, start.X, start.Y, got.X, got.Y)
	}
}

func TestPrebuildAIFallbackRequiresOfficialAITeam(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(32, 32)
	model.BlockNames = map[int16]string{
		45:  "duo",
		339: "core-shard",
	}
	model.UnitNames = map[int16]string{
		35: "alpha",
	}
	w.SetModel(model)
	w.unitRuntimeProfilesByName["alpha"] = unitRuntimeProfile{
		Name:         "alpha",
		Speed:        24,
		Flying:       true,
		BuildSpeed:   0.5,
		MineSpeed:    6.5,
		MineTier:     1,
		ItemCapacity: 30,
		MineFloor:    true,
	}

	rules := w.GetRulesManager().Get()
	rules.AttackMode = true
	rules.PrebuildAi = true

	placeTestBuilding(t, w, 4, 4, 339, 1, 0)
	core := placeTestBuilding(t, w, 24, 16, 339, 2, 0)
	core.Build.AddItem(0, 100)
	w.queueTeamBuildPlanFrontLocked(2, BuildPlanOp{X: 20, Y: 16, BlockID: 45})

	if _, err := w.AddEntityWithID(35, 9301, 24*8+4, 16*8+4, 2); err != nil {
		t.Fatalf("add ai alpha entity: %v", err)
	}
	if _, ok := w.SetEntityFlag(9301, float64(packTilePos(core.X, core.Y))); !ok {
		t.Fatal("bind ai alpha to core")
	}
	if _, err := w.AddEntityWithID(35, 9302, 8*8+4, 8*8+4, 1); err != nil {
		t.Fatalf("add default-team alpha entity: %v", err)
	}
	if _, ok := w.SetEntityFlag(9302, float64(packTilePos(4, 4))); !ok {
		t.Fatal("bind default-team alpha to core")
	}

	w.Step(time.Second / 60)

	gotAI := findTestEntity(t, w, 9301)
	if len(gotAI.Plans) == 0 || gotAI.Plans[0].Pos != packTilePos(20, 16) {
		t.Fatalf("expected official AI team to use PrebuildAI queue plan, got %+v", gotAI.Plans)
	}
	gotDefault := findTestEntity(t, w, 9302)
	if len(gotDefault.Plans) != 0 {
		t.Fatalf("expected default team builder to keep BuilderAI instead of PrebuildAI, got %+v", gotDefault.Plans)
	}
}

func TestPrebuildAIMinesMissingItemsBeforeBuilding(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(32, 32)
	model.BlockNames = map[int16]string{
		0:   "air",
		1:   "stone",
		2:   "ore-copper",
		100: "router",
		339: "core-shard",
	}
	model.UnitNames = map[int16]string{
		35: "alpha",
	}
	model.Tiles[16*model.Width+10].Floor = 1
	model.Tiles[16*model.Width+10].Overlay = 2
	w.SetModel(model)
	w.unitRuntimeProfilesByName["alpha"] = unitRuntimeProfile{
		Name:         "alpha",
		Speed:        24,
		Flying:       true,
		BuildSpeed:   0.5,
		MineSpeed:    6.5,
		MineTier:     1,
		ItemCapacity: 30,
		MineFloor:    true,
	}
	w.blockBuildTimesByName["router"] = 0.35

	rules := w.GetRulesManager().Get()
	rules.AttackMode = true
	rules.PrebuildAi = true

	placeTestBuilding(t, w, 6, 16, 339, 2, 0)
	w.queueTeamBuildPlanFrontLocked(2, BuildPlanOp{X: 12, Y: 16, BlockID: 100})

	if _, err := w.AddEntityWithID(35, 9303, 8*8+4, 16*8+4, 2); err != nil {
		t.Fatalf("add ai alpha entity: %v", err)
	}
	if _, ok := w.SetEntityFlag(9303, float64(packTilePos(6, 16))); !ok {
		t.Fatal("bind ai alpha to core")
	}

	w.Step(time.Second / 60)
	got := findTestEntity(t, w, 9303)
	if got.UpdateBuilding || len(got.Plans) == 0 {
		t.Fatalf("expected PrebuildAI to queue the team plan immediately, updateBuilding=%v plans=%+v", got.UpdateBuilding, got.Plans)
	}

	stepForSeconds(w, 6)

	tile, err := w.Model().TileAt(12, 16)
	if err != nil || tile == nil {
		t.Fatalf("tile lookup failed: %v", err)
	}
	if tile.Block != 100 || tile.Build == nil || tile.Team != 2 {
		t.Fatalf("expected PrebuildAI to mine copper and construct router, block=%d build=%v team=%d", tile.Block, tile.Build != nil, tile.Team)
	}
	if len(w.teamAIBuildPlans[2]) != 0 {
		t.Fatalf("expected finished prebuild plan to be cleared from team queue, got %+v", w.teamAIBuildPlans[2])
	}
	got = findTestEntity(t, w, 9303)
	if got.Stack.Amount != 0 {
		t.Fatalf("expected builder stack to be deposited after prebuild, got %+v", got.Stack)
	}
}

func TestPrebuildAIRemovedTeamPlanClearsEntityPlan(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		45:  "duo",
		339: "core-shard",
	}
	model.UnitNames = map[int16]string{
		35: "alpha",
	}
	w.SetModel(model)
	w.unitRuntimeProfilesByName["alpha"] = unitRuntimeProfile{
		Name:         "alpha",
		Speed:        24,
		Flying:       true,
		BuildSpeed:   0.5,
		MineSpeed:    6.5,
		MineTier:     1,
		ItemCapacity: 30,
		MineFloor:    true,
	}

	rules := w.GetRulesManager().Get()
	rules.AttackMode = true
	rules.PrebuildAi = true

	core := placeTestBuilding(t, w, 3, 12, 339, 2, 0)
	core.Build.AddItem(0, 100)
	w.queueTeamBuildPlanFrontLocked(2, BuildPlanOp{X: 8, Y: 12, BlockID: 45})

	if _, err := w.AddEntityWithID(35, 9304, 6*8+4, 12*8+4, 2); err != nil {
		t.Fatalf("add ai alpha entity: %v", err)
	}
	if _, ok := w.SetEntityFlag(9304, float64(packTilePos(core.X, core.Y))); !ok {
		t.Fatal("bind ai alpha to core")
	}

	w.Step(time.Second / 60)
	got := findTestEntity(t, w, 9304)
	if len(got.Plans) == 0 || got.Plans[0].Pos != packTilePos(8, 12) {
		t.Fatalf("expected prebuild builder to pick queued team plan, got %+v", got.Plans)
	}

	delete(w.teamAIBuildPlans, TeamID(2))
	stepForSeconds(w, 1)

	got = findTestEntity(t, w, 9304)
	if len(got.Plans) != 0 {
		t.Fatalf("expected removed prebuild team plan to clear builder plan, got %+v", got.Plans)
	}
}

func TestPrebuildAICoreSpawnCreatesDedicatedBuilderPerCore(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(32, 32)
	model.BlockNames = map[int16]string{
		339: "core-shard",
		340: "core-foundation",
	}
	model.UnitNames = map[int16]string{
		35: "alpha",
		36: "beta",
	}
	w.SetModel(model)

	rules := w.GetRulesManager().Get()
	rules.PrebuildAi = true

	placeTestBuilding(t, w, 5, 5, 339, 2, 0)
	placeTestBuilding(t, w, 10, 10, 339, 2, 0)
	placeTestBuilding(t, w, 16, 16, 340, 2, 0)

	w.Step(time.Second / 60)

	model = w.Model()
	if got := len(model.Entities); got != 3 {
		t.Fatalf("expected one dedicated core builder per core, got %d entities", got)
	}

	want := map[int16]map[float64]struct{}{
		35: {
			float64(packTilePos(5, 5)):   {},
			float64(packTilePos(10, 10)): {},
		},
		36: {
			float64(packTilePos(16, 16)): {},
		},
	}
	for _, ent := range model.Entities {
		flags, ok := want[ent.TypeID]
		if !ok {
			t.Fatalf("unexpected spawned core builder type=%d flag=%f", ent.TypeID, ent.Flag)
		}
		if !ent.SpawnedByCore {
			t.Fatalf("expected spawned core builder to be marked spawnedByCore, entity=%+v", ent)
		}
		if _, ok := flags[ent.Flag]; !ok {
			t.Fatalf("unexpected core binding for type=%d flag=%f", ent.TypeID, ent.Flag)
		}
		delete(flags, ent.Flag)
	}
	for typeID, flags := range want {
		if len(flags) != 0 {
			t.Fatalf("missing core builders for type=%d flags=%v", typeID, flags)
		}
	}
}

func TestPrebuildAICoreSpawnDoesNotShareBoundBuilderAcrossCores(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		339: "core-shard",
	}
	model.UnitNames = map[int16]string{
		35: "alpha",
	}
	w.SetModel(model)

	rules := w.GetRulesManager().Get()
	rules.PrebuildAi = true

	placeTestBuilding(t, w, 4, 4, 339, 2, 0)
	placeTestBuilding(t, w, 12, 12, 339, 2, 0)

	if _, err := w.AddEntityWithID(35, 9401, 4*8+4, 4*8+4, 2); err != nil {
		t.Fatalf("add prebound alpha entity: %v", err)
	}
	if _, ok := w.SetEntityFlag(9401, float64(packTilePos(4, 4))); !ok {
		t.Fatal("bind alpha to first core")
	}

	w.Step(time.Second / 60)

	model = w.Model()
	if got := len(model.Entities); got != 2 {
		t.Fatalf("expected existing bound builder plus one missing builder, got %d entities", got)
	}

	seen := map[float64]struct{}{}
	for _, ent := range model.Entities {
		if ent.TypeID != 35 {
			t.Fatalf("expected only alpha builders, got type=%d", ent.TypeID)
		}
		seen[ent.Flag] = struct{}{}
	}
	if _, ok := seen[float64(packTilePos(4, 4))]; !ok {
		t.Fatal("expected first core builder to remain bound")
	}
	if _, ok := seen[float64(packTilePos(12, 12))]; !ok {
		t.Fatal("expected second core to receive its own bound builder")
	}
}

func TestBuildAIQueuesMechanicalDrillPlanNearOre(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(32, 32)
	model.BlockNames = map[int16]string{
		2:   "ore-copper",
		339: "core-shard",
		429: "mechanical-drill",
	}
	w.SetModel(model)
	paintAreaOverlay(t, w, 10, 8, 2, 2)
	placeTestBuilding(t, w, 8, 8, 339, 2, 0)

	rules := w.GetRulesManager().Get()
	rules.AttackMode = true
	rules.BuildAi = true
	rules.BuildAiTier = 1

	w.Step(time.Second / 60)

	plans := w.teamAIBuildPlans[2]
	if len(plans) == 0 {
		t.Fatal("expected buildAi to queue at least one team build plan")
	}
	if plans[0].BlockID != 429 {
		t.Fatalf("expected buildAi to queue mechanical drill first, got %+v", plans[0])
	}
	if x, y := int(plans[0].X), int(plans[0].Y); abs(x-8) > buildAISeedRangeTiles || abs(y-8) > buildAISeedRangeTiles {
		t.Fatalf("expected queued drill to stay near core seed, plan=%+v", plans[0])
	}
}

func TestBuildAIFillCoresRefillsPrimaryCoreWithoutFillItemsRule(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.BlockNames = map[int16]string{
		343: "core-citadel",
	}
	w.SetModel(model)
	core := placeTestBuilding(t, w, 6, 6, 343, 2, 0)

	rules := w.GetRulesManager().Get()
	rules.BuildAi = true
	if rules.teamFillItems(2) {
		t.Fatal("expected fillItems rule to stay disabled for this buildAi fill-cores test")
	}

	pos := int32(core.Y*model.Width + core.X)
	capacity := w.itemCapacityAtLocked(pos)
	if capacity <= 0 {
		t.Fatalf("expected positive core capacity, got %d", capacity)
	}
	if got := core.Build.ItemAmount(copperItemID); got != 0 {
		t.Fatalf("expected empty core before buildAi fill, got copper=%d", got)
	}

	w.Step(time.Second / 60)

	if got := core.Build.ItemAmount(copperItemID); got != capacity {
		t.Fatalf("expected buildAi fill-cores to refill copper to %d, got %d", capacity, got)
	}
	if got := core.Build.ItemAmount(titaniumItemID); got != capacity {
		t.Fatalf("expected buildAi fill-cores to refill titanium to %d, got %d", capacity, got)
	}
}

func TestBuildAICoreSpawnSpawnsCoreUnitAfterOfficialInterval(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		339: "core-shard",
	}
	model.UnitNames = map[int16]string{
		35: "alpha",
	}
	w.SetModel(model)
	placeTestBuilding(t, w, 8, 8, 339, 2, 0)

	rules := w.GetRulesManager().Get()
	rules.BuildAi = true
	rules.AiCoreSpawn = true

	stepForSeconds(w, 6.2)

	if len(w.Model().Entities) != 1 {
		t.Fatalf("expected one core-spawned alpha after aiCoreSpawn interval, got %d", len(w.Model().Entities))
	}
	got := w.Model().Entities[0]
	if got.TypeID != 35 || got.Team != 2 {
		t.Fatalf("expected spawned alpha for team 2, got %+v", got)
	}
	if !got.SpawnedByCore {
		t.Fatalf("expected aiCoreSpawn unit to be marked spawnedByCore, got %+v", got)
	}
}

func TestBuildAIRefreshPathBuildsCorridorTowardEnemyCore(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 14)
	model.BlockNames = map[int16]string{
		1:   "spawn",
		339: "core-shard",
	}
	w.SetModel(model)
	model.Tiles[0].Overlay = 1
	model.Tiles[7*model.Width+23].Overlay = 1
	placeTestBuilding(t, w, 3, 7, 339, 1, 0)
	placeTestBuilding(t, w, 20, 7, 339, 2, 0)

	rules := w.GetRulesManager().Get()
	rules.BuildAi = true

	w.Step(time.Second / 60)

	state := w.teamBuildAIStates[2]
	if len(state.PathCells) == 0 {
		t.Fatal("expected buildAi refresh path to generate a non-empty corridor")
	}
	_, roots, ok := w.buildAIEnemyCoreCellsLocked(2)
	if !ok || len(roots) == 0 {
		t.Fatal("expected enemy core roots for buildAi path test")
	}
	spawnX, spawnY, ok := w.buildAIPathSpawnCellLocked(roots)
	if !ok {
		t.Fatal("expected buildAi path spawn cell")
	}
	if _, ok := state.PathCells[packTilePos(spawnX, spawnY)]; !ok {
		t.Fatalf("expected buildAi corridor to include chosen spawn cell (%d,%d), path=%v", spawnX, spawnY, state.PathCells)
	}
	if _, ok := state.PathCells[packTilePos(3, 7)]; !ok {
		t.Fatalf("expected buildAi corridor to reach enemy core footprint, path=%v", state.PathCells)
	}
}

func TestBuildAITryPlaceAvoidsRefreshedPathCorridor(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 14)
	model.BlockNames = map[int16]string{
		1:   "spawn",
		45:  "duo",
		339: "core-shard",
	}
	w.SetModel(model)
	model.Tiles[7*model.Width+23].Overlay = 1
	placeTestBuilding(t, w, 3, 7, 339, 1, 0)
	placeTestBuilding(t, w, 20, 7, 339, 2, 0)

	rules := w.GetRulesManager().Get()
	rules.BuildAi = true

	w.Step(time.Second / 60)
	_, roots, ok := w.buildAIEnemyCoreCellsLocked(2)
	if !ok || len(roots) == 0 {
		t.Fatal("expected enemy core roots for path rejection test")
	}
	spawnX, spawnY, ok := w.buildAIPathSpawnCellLocked(roots)
	if !ok {
		t.Fatal("expected buildAi path spawn cell for path rejection test")
	}

	part := buildAIBasePart{
		Name:   "manual-solid-path-test",
		Width:  1,
		Height: 1,
		Tiles: []buildAIBasePartTile{{
			BlockName: "duo",
			BlockID:   45,
		}},
	}
	if w.queueBuildAIPartAtSeedLocked(2, part, spawnX, spawnY, 0) {
		t.Fatal("expected buildAi tryPlace to reject solid block placement intersecting refreshed path corridor")
	}
	if len(w.teamAIBuildPlans[2]) != 0 {
		t.Fatalf("expected path-rejected tryPlace to avoid queueing plans, got %+v", w.teamAIBuildPlans[2])
	}
}

func TestBuildAIPathSpawnUsesLastOfficialSpawnOverlay(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 14)
	model.BlockNames = map[int16]string{
		1: "spawn",
	}
	w.SetModel(model)
	model.Tiles[1*model.Width+1].Overlay = 1
	model.Tiles[7*model.Width+23].Overlay = 1

	spawnX, spawnY, ok := w.buildAIPathSpawnCellLocked(nil)
	if !ok {
		t.Fatal("expected buildAi path spawn cell from official spawn overlay")
	}
	if spawnX != 23 || spawnY != 7 {
		t.Fatalf("expected buildAi path to use the last official spawn overlay, got (%d,%d)", spawnX, spawnY)
	}
}

func TestBuildAIPathSpawnAppendsAttackWaveCoreSpawnsAfterOverlaySpawns(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(32, 16)
	model.BlockNames = map[int16]string{
		1:   "spawn",
		339: "core-shard",
	}
	w.SetModel(model)
	model.Tiles[7*model.Width+31].Overlay = 1
	placeTestBuilding(t, w, 4, 7, 339, 1, 0)
	placeTestBuilding(t, w, 20, 7, 339, 2, 0)

	rules := w.GetRulesManager().Get()
	rules.AttackMode = true
	rules.WavesSpawnAtCores = true

	spawnX, spawnY, ok := w.buildAIPathSpawnCellLocked(nil)
	if !ok {
		t.Fatal("expected buildAi path spawn cell to include official attack-mode wave core spawns")
	}
	if spawnX != 15 || spawnY != 7 {
		t.Fatalf("expected buildAi path to append wave-core ground spawn after overlays and choose (15,7), got (%d,%d)", spawnX, spawnY)
	}
}

func TestBuildAIOverlaySpawnRadiusCheckDoesNotUseWaveCoreSpawns(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(32, 16)
	model.BlockNames = map[int16]string{
		339: "core-shard",
	}
	w.SetModel(model)
	placeTestBuilding(t, w, 4, 7, 339, 1, 0)
	placeTestBuilding(t, w, 20, 7, 339, 2, 0)

	rules := w.GetRulesManager().Get()
	rules.AttackMode = true
	rules.WavesSpawnAtCores = true

	if w.buildAITileNearGroundSpawnLocked(15, 7, buildAISpawnProtectRadiusTiles) {
		t.Fatal("expected buildAi 40-tile spawn protection to stay tied to official overlay spawns, not attack-mode wave-core spawn points")
	}
}

func TestBuildAIPathBlockingUsesOfficialSolidSemantics(t *testing.T) {
	w := New(Config{TPS: 60})
	w.SetModel(NewWorldModel(16, 16))
	w.teamBuildAIStates = map[TeamID]buildAIPlannerState{
		2: {
			PathCells: map[int32]struct{}{
				packTilePos(5, 5): {},
			},
		},
	}

	if !w.buildAIPlanIntersectsPathLocked(2, 5, 5, "bridge-conveyor") {
		t.Fatal("expected official solid bridge-conveyor to block buildAi path corridor")
	}
	if w.buildAIPlanIntersectsPathLocked(2, 5, 5, "router") {
		t.Fatal("expected official non-solid router to be ignored by buildAi path corridor checks")
	}
	if w.buildAIPlanIntersectsPathLocked(2, 5, 5, "payload-conveyor") {
		t.Fatal("expected official non-solid payload-conveyor to be ignored by buildAi path corridor checks")
	}
}

func TestBuildAIDrillFallbackAvoidsOfficialSpawnRadius(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(80, 20)
	model.BlockNames = map[int16]string{
		1:   "spawn",
		2:   "ore-copper",
		339: "core-shard",
		429: "mechanical-drill",
	}
	w.SetModel(model)
	model.Tiles[10*model.Width+5].Overlay = 1
	paintAreaOverlay(t, w, 35, 10, 2, 2)
	paintAreaOverlay(t, w, 55, 10, 2, 2)
	placeTestBuilding(t, w, 45, 10, 339, 2, 0)

	op, ok := w.findBuildAIDrillPlanLocked(2, 45, 10)
	if !ok {
		t.Fatal("expected buildAi drill fallback to find a valid ore tile outside the official spawn radius")
	}
	if op.X != 54 || op.Y != 10 {
		t.Fatalf("expected buildAi drill fallback to skip spawn-adjacent ore and choose (54,10), got (%d,%d)", op.X, op.Y)
	}
}

func TestBuildAIQueuesOfficialBasePartIntoIndependentTeamPlans(t *testing.T) {
	raw := pickCopperBasePartSchematicForTest(t)

	w := New(Config{TPS: 60})
	model := NewWorldModel(96, 96)
	model.BlockNames = map[int16]string{
		2:   "ore-copper",
		339: "core-shard",
	}
	nextID := int16(1000)
	for _, tile := range raw.Tiles {
		name := normalizeBlockLookupName(tile.Block)
		switch name {
		case "itemsource", "liquidsource", "powersource", "powervoid", "payloadsource", "payloadvoid", "heatsource":
			continue
		}
		known := false
		for _, existing := range model.BlockNames {
			if existing == tile.Block {
				known = true
				break
			}
		}
		if known {
			continue
		}
		model.BlockNames[nextID] = tile.Block
		nextID++
	}
	w.SetModel(model)
	placeTestBuilding(t, w, 12, 12, 339, 2, 0)

	part, ok := w.convertBuildAIBasePartLocked(raw)
	if !ok {
		t.Fatalf("expected official copper basepart %q to convert into buildAi part", raw.Name)
	}
	if len(part.Tiles) < 2 {
		t.Fatalf("expected converted official basepart to stay multi-tile, got %d", len(part.Tiles))
	}

	seedX, seedY := 48, 48
	cx := seedX - part.CenterX
	cy := seedY - part.CenterY
	for _, tile := range part.Tiles {
		if !buildAIBasePartCountsForCenter(tile.BlockName) {
			continue
		}
		paintAreaOverlay(t, w, tile.X+cx, tile.Y+cy, blockSizeByName(tile.BlockName), 2)
	}

	if !w.queueBuildAIPartAtSeedLocked(2, part, seedX, seedY, 0) {
		t.Fatalf("expected official basepart %q to queue successfully", raw.Name)
	}

	plans := w.teamAIBuildPlans[2]
	if len(plans) != len(part.Tiles) {
		t.Fatalf("expected one independent team plan per basepart tile, want=%d got=%d", len(part.Tiles), len(plans))
	}
	seen := map[int32]struct{}{}
	for _, plan := range plans {
		pos := packTilePos(int(plan.X), int(plan.Y))
		if _, ok := seen[pos]; ok {
			t.Fatalf("expected basepart queue to keep distinct tile plans, got duplicate pos=%d plans=%+v", pos, plans)
		}
		seen[pos] = struct{}{}
	}
}

func TestBuilderAIConsumesTeamBuildPlansWhenBuildAIEnabled(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(32, 32)
	model.BlockNames = map[int16]string{
		45:  "duo",
		339: "core-shard",
	}
	model.UnitNames = map[int16]string{
		35: "alpha",
	}
	w.SetModel(model)
	placeTestBuilding(t, w, 5, 5, 339, 1, 0).Build.AddItem(0, 100)
	w.queueTeamBuildPlanBackLocked(1, BuildPlanOp{X: 9, Y: 5, BlockID: 45})

	rules := w.GetRulesManager().Get()
	rules.BuildAi = true
	rules.BuildAiTier = 1

	if _, err := w.AddEntityWithID(35, 9501, 5*8+4, 5*8+4, 1); err != nil {
		t.Fatalf("add alpha entity: %v", err)
	}

	w.Step(time.Second / 60)

	got := findTestEntity(t, w, 9501)
	if len(got.Plans) == 0 || got.Plans[0].Pos != packTilePos(9, 5) {
		t.Fatalf("expected buildAi-enabled builder to consume team build plan, got %+v", got.Plans)
	}
}

func TestBuilderAIBuildAiModeUsesTeamPlanQueueInsteadOfRebuildQueue(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		45:  "duo",
		339: "core-shard",
	}
	model.UnitNames = map[int16]string{
		35: "alpha",
	}
	w.SetModel(model)
	core := placeTestBuilding(t, w, 4, 4, 339, 1, 0)
	core.Build.AddItem(0, 1000)
	placeTestBuilding(t, w, 10, 10, 45, 1, 0)
	if !w.DamageBuildingPacked(packTilePos(10, 10), 2000) {
		t.Fatal("expected destroyed building to enter rebuild queues")
	}

	rules := w.GetRulesManager().Get()
	rules.BuildAi = true
	rules.BuildAiTier = 1

	if _, err := w.AddEntityWithID(35, 9502, 6*8+4, 4*8+4, 1); err != nil {
		t.Fatalf("add alpha entity: %v", err)
	}

	w.Step(time.Second / 60)
	got := findTestEntity(t, w, 9502)
	if len(got.Plans) == 0 || got.Plans[0].Pos != packTilePos(10, 10) {
		t.Fatalf("expected buildAi-mode builder to pick mirrored team plan, got %+v", got.Plans)
	}

	delete(w.teamAIBuildPlans, TeamID(1))
	stepForSeconds(w, 1)

	got = findTestEntity(t, w, 9502)
	if len(got.Plans) != 0 {
		t.Fatalf("expected removing team build queue to clear buildAi-mode builder plan even if rebuild queue remains, got %+v", got.Plans)
	}
	if len(w.teamRebuildPlans[1]) == 0 {
		t.Fatal("expected dedicated rebuild queue to remain untouched by buildAi-mode validation test")
	}
}

