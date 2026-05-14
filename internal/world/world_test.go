package world

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/IYanHua/mdt-server/internal/protocol"
	"github.com/IYanHua/mdt-server/internal/vanilla"
)

type decodedBlockSyncBase struct {
	Health             float32
	Rotation           byte
	Team               byte
	Version            byte
	Enabled            byte
	ModuleBits         byte
	Items              map[ItemID]int32
	PowerLinks         []int32
	PowerStatus        float32
	Liquids            map[LiquidID]float32
	Efficiency         byte
	OptionalEfficiency byte
}

func decodeBlockSyncBase(t *testing.T, data []byte) (decodedBlockSyncBase, *protocol.Reader) {
	t.Helper()
	r := protocol.NewReader(data)
	var out decodedBlockSyncBase
	var err error
	if out.Health, err = r.ReadFloat32(); err != nil {
		t.Fatalf("read sync health failed: %v", err)
	}
	if out.Rotation, err = r.ReadByte(); err != nil {
		t.Fatalf("read sync rotation failed: %v", err)
	}
	if out.Team, err = r.ReadByte(); err != nil {
		t.Fatalf("read sync team failed: %v", err)
	}
	if out.Version, err = r.ReadByte(); err != nil {
		t.Fatalf("read sync version failed: %v", err)
	}
	if out.Enabled, err = r.ReadByte(); err != nil {
		t.Fatalf("read sync enabled failed: %v", err)
	}
	if out.ModuleBits, err = r.ReadByte(); err != nil {
		t.Fatalf("read sync module bits failed: %v", err)
	}
	if (out.ModuleBits & 1) != 0 {
		count, err := r.ReadInt16()
		if err != nil {
			t.Fatalf("read item module count failed: %v", err)
		}
		out.Items = make(map[ItemID]int32, count)
		for i := 0; i < int(count); i++ {
			id, err := r.ReadInt16()
			if err != nil {
				t.Fatalf("read item module id failed: %v", err)
			}
			amount, err := r.ReadInt32()
			if err != nil {
				t.Fatalf("read item module amount failed: %v", err)
			}
			out.Items[ItemID(id)] = amount
		}
	}
	if (out.ModuleBits & (1 << 1)) != 0 {
		count, err := r.ReadInt16()
		if err != nil {
			t.Fatalf("read power module link count failed: %v", err)
		}
		out.PowerLinks = make([]int32, 0, count)
		for i := 0; i < int(count); i++ {
			link, err := r.ReadInt32()
			if err != nil {
				t.Fatalf("read power module link failed: %v", err)
			}
			out.PowerLinks = append(out.PowerLinks, link)
		}
		if out.PowerStatus, err = r.ReadFloat32(); err != nil {
			t.Fatalf("read power module status failed: %v", err)
		}
	}
	if (out.ModuleBits & (1 << 2)) != 0 {
		count, err := r.ReadInt16()
		if err != nil {
			t.Fatalf("read liquid module count failed: %v", err)
		}
		out.Liquids = make(map[LiquidID]float32, count)
		for i := 0; i < int(count); i++ {
			id, err := r.ReadInt16()
			if err != nil {
				t.Fatalf("read liquid module id failed: %v", err)
			}
			amount, err := r.ReadFloat32()
			if err != nil {
				t.Fatalf("read liquid module amount failed: %v", err)
			}
			out.Liquids[LiquidID(id)] = amount
		}
	}
	if (out.ModuleBits & (1 << 4)) != 0 {
		if _, err := r.ReadFloat32(); err != nil {
			t.Fatalf("read timescale failed: %v", err)
		}
		if _, err := r.ReadFloat32(); err != nil {
			t.Fatalf("read timescale duration failed: %v", err)
		}
	}
	if (out.ModuleBits & (1 << 5)) != 0 {
		if _, err := r.ReadInt32(); err != nil {
			t.Fatalf("read last disabler failed: %v", err)
		}
	}
	if out.Version <= 2 {
		if _, err := r.ReadBool(); err != nil {
			t.Fatalf("read legacy consume flag failed: %v", err)
		}
	}
	if out.Efficiency, err = r.ReadByte(); err != nil {
		t.Fatalf("read efficiency failed: %v", err)
	}
	if out.OptionalEfficiency, err = r.ReadByte(); err != nil {
		t.Fatalf("read optional efficiency failed: %v", err)
	}
	return out, r
}

func placeTestBuilding(t *testing.T, w *World, x, y int, block int16, team TeamID, rotation int8) *Tile {
	t.Helper()
	tile, err := w.Model().TileAt(x, y)
	if err != nil || tile == nil {
		t.Fatalf("tile lookup failed at (%d,%d): %v", x, y, err)
	}
	tile.Block = BlockID(block)
	tile.Team = team
	tile.Rotation = rotation
	tile.Build = &Building{
		Block:     BlockID(block),
		Team:      team,
		Rotation:  rotation,
		X:         x,
		Y:         y,
		Health:    1000,
		MaxHealth: 1000,
	}
	w.rebuildBlockOccupancyLocked()
	return tile
}

func containsPos(slice []int32, pos int32) bool {
	for _, existing := range slice {
		if existing == pos {
			return true
		}
	}
	return false
}

func mustEncodeConfig(t *testing.T, value any) []byte {
	t.Helper()
	w := protocol.NewWriter()
	if err := protocol.WriteObject(w, value, nil); err != nil {
		t.Fatalf("encode config failed: %v", err)
	}
	return append([]byte(nil), w.Bytes()...)
}

func linkPowerNode(t *testing.T, w *World, x, y int, links ...protocol.Point2) {
	t.Helper()
	pos := int32(y*w.Model().Width + x)
	w.applyBuildingConfigLocked(pos, links, true)
}

func stepForSeconds(w *World, seconds float32) {
	frames := int(seconds*60 + 0.5)
	if frames < 1 {
		frames = 1
	}
	for i := 0; i < frames; i++ {
		w.Step(time.Second / 60)
	}
}

func stepForTicks(w *World, ticks int) {
	for i := 0; i < ticks; i++ {
		w.Step(time.Second / 60)
	}
}

func TestRebuildBlockOccupancyIndexesPhaseBuckets(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		1:  "item-source",
		2:  "liquid-source",
		3:  "conveyor",
		4:  "duct-router",
		5:  "bridge-conveyor",
		6:  "unloader",
		7:  "mass-driver",
		8:  "conduit",
		9:  "liquid-router",
		10: "bridge-conduit",
		11: "payload-conveyor",
		12: "ground-factory",
		13: "thorium-reactor",
		14: "additive-reconstructor",
	}
	w.SetModel(model)

	builds := []struct {
		x, y  int
		block int16
	}{
		{2, 2, 1},
		{4, 2, 2},
		{6, 2, 3},
		{8, 2, 4},
		{10, 2, 5},
		{12, 2, 6},
		{14, 2, 7},
		{2, 6, 8},
		{4, 6, 9},
		{6, 6, 10},
		{8, 6, 11},
		{10, 6, 12},
		{12, 6, 13},
		{14, 6, 14},
	}
	positions := make(map[int16]int32, len(builds))
	for _, build := range builds {
		tile := placeTestBuilding(t, w, build.x, build.y, build.block, 1, 0)
		positions[build.block] = int32(tile.Y*w.Model().Width + tile.X)
	}

	checks := []struct {
		name  string
		slice []int32
		pos   int32
	}{
		{"sandbox item-source", w.sandboxItemSourceTiles, positions[1]},
		{"sandbox liquid-source", w.sandboxLiquidSourceTiles, positions[2]},
		{"item conveyor", w.itemConveyorTilePositions, positions[3]},
		{"item duct/router", w.itemDuctTilePositions, positions[4]},
		{"item bridge", w.itemBridgeTilePositions, positions[5]},
		{"item unloader", w.itemUnloaderTilePositions, positions[6]},
		{"item mass-driver", w.itemMassDriverTilePositions, positions[7]},
		{"liquid conduit", w.liquidConduitTilePositions, positions[8]},
		{"liquid storage", w.liquidStorageTilePositions, positions[9]},
		{"liquid bridge", w.liquidBridgeTilePositions, positions[10]},
		{"payload transport", w.payloadTransportTiles, positions[11]},
		{"payload factory", w.payloadFactoryTilePositions, positions[12]},
		{"reactor", w.reactorTilePositions, positions[13]},
		{"payload reconstructor", w.payloadFactoryTilePositions, positions[14]},
	}
	for _, check := range checks {
		if !containsPos(check.slice, check.pos) {
			t.Fatalf("expected %s bucket to contain pos=%d", check.name, check.pos)
		}
	}

	if containsPos(w.payloadTransportTiles, positions[12]) {
		t.Fatalf("ground-factory should not be indexed as payload transport")
	}
	if containsPos(w.payloadFactoryTilePositions, positions[11]) {
		t.Fatalf("payload-conveyor should not be indexed as payload factory")
	}
}

func paintAreaOverlay(t *testing.T, w *World, cx, cy, size int, overlay int16) {
	t.Helper()
	low, high := blockFootprintRange(size)
	for dy := low; dy <= high; dy++ {
		for dx := low; dx <= high; dx++ {
			tile, err := w.Model().TileAt(cx+dx, cy+dy)
			if err != nil || tile == nil {
				t.Fatalf("overlay tile lookup failed at (%d,%d): %v", cx+dx, cy+dy, err)
			}
			tile.Overlay = OverlayID(overlay)
		}
	}
}

func paintAreaFloor(t *testing.T, w *World, cx, cy, size int, floor int16) {
	t.Helper()
	low, high := blockFootprintRange(size)
	for dy := low; dy <= high; dy++ {
		for dx := low; dx <= high; dx++ {
			tile, err := w.Model().TileAt(cx+dx, cy+dy)
			if err != nil || tile == nil {
				t.Fatalf("floor tile lookup failed at (%d,%d): %v", cx+dx, cy+dy, err)
			}
			tile.Floor = FloorID(floor)
		}
	}
}

func paintWallRect(t *testing.T, w *World, minX, minY, maxX, maxY int, block int16, skip map[int32]struct{}) {
	t.Helper()
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			if skip != nil {
				if _, ok := skip[packTilePos(x, y)]; ok {
					continue
				}
			}
			tile, err := w.Model().TileAt(x, y)
			if err != nil || tile == nil {
				t.Fatalf("wall tile lookup failed at (%d,%d): %v", x, y, err)
			}
			tile.Block = BlockID(block)
			tile.Build = nil
			tile.Team = 0
		}
	}
}

func pickCopperBasePartSchematicForTest(t *testing.T) vanilla.BasePartSchematic {
	t.Helper()
	parts, err := vanilla.LoadEmbeddedBasePartSchematics()
	if err != nil {
		t.Fatalf("load embedded baseparts: %v", err)
	}
	for _, part := range parts {
		hasCopper := false
		hasCore := false
		usableTiles := 0
		for _, tile := range part.Tiles {
			name := normalizeBlockLookupName(tile.Block)
			switch name {
			case "itemsource":
				if ref, ok := tile.Config.(vanilla.BasePartContentRef); ok && ref.ContentType == vanilla.BasePartContentItem && ItemID(ref.ID) == copperItemID {
					hasCopper = true
				}
				continue
			case "liquidsource", "powersource", "powervoid", "payloadsource", "payloadvoid", "heatsource":
				continue
			}
			if strings.HasPrefix(name, "core") {
				hasCore = true
			}
			usableTiles++
		}
		if hasCopper && !hasCore && usableTiles >= 2 {
			return part
		}
	}
	t.Fatal("expected an official copper basepart candidate for buildAi tests")
	return vanilla.BasePartSchematic{}
}

func newPayloadBuildingWorld(t *testing.T, blockID int16, blockName string, rotation int8, unit RawEntity) (*World, int32, int32, int32) {
	t.Helper()
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.BlockNames = map[int16]string{
		blockID: blockName,
	}
	model.UnitNames = map[int16]string{
		unit.TypeID: "dagger",
	}
	w.SetModel(model)
	tile := placeTestBuilding(t, w, 6, 6, blockID, 1, rotation)
	buildPos := int32(tile.Y*w.Model().Width + tile.X)
	buildPacked := protocol.PackPoint2(int32(tile.X), int32(tile.Y))
	added := w.Model().AddEntity(unit)
	return w, buildPacked, buildPos, added.ID
}

func newPayloadControlSelectWorld(t *testing.T, unit RawEntity) (*World, int32, int32, int32) {
	t.Helper()
	return newPayloadBuildingWorld(t, 700, "payload-conveyor", 0, unit)
}

func TestControlSelectPayloadUnitPackedMovesStandingUnitIntoPayload(t *testing.T) {
	unit := RawEntity{
		ID:        11,
		TypeID:    7,
		Team:      1,
		X:         5*8 + 4,
		Y:         6*8 + 4,
		Health:    90,
		MaxHealth: 90,
	}
	w, buildPacked, buildPos, unitID := newPayloadControlSelectWorld(t, unit)

	if !w.ControlSelectPayloadUnitPacked(buildPacked, unitID) {
		t.Fatal("expected standing unit to enter payload")
	}

	payload := w.payloadStateLocked(buildPos).Payload
	if payload == nil || payload.Kind != payloadKindUnit || payload.UnitTypeID != 7 {
		t.Fatalf("expected dagger unit payload on building, got %+v", payload)
	}
	if payload.UnitState == nil || payload.UnitState.ID != 0 {
		t.Fatalf("expected payload unit state to be serialized detached copy, got %+v", payload.UnitState)
	}
	for _, ent := range w.Model().Entities {
		if ent.ID == unitID {
			t.Fatalf("expected world unit %d to be removed after entering payload", unitID)
		}
	}
}

func TestControlSelectPayloadUnitPackedRejectsSpawnedByCoreUnit(t *testing.T) {
	unit := RawEntity{
		ID:            12,
		TypeID:        7,
		Team:          1,
		X:             6*8 + 4,
		Y:             6*8 + 4,
		Health:        90,
		MaxHealth:     90,
		SpawnedByCore: true,
	}
	w, buildPacked, buildPos, unitID := newPayloadControlSelectWorld(t, unit)

	if w.ControlSelectPayloadUnitPacked(buildPacked, unitID) {
		t.Fatal("expected spawnedByCore unit to be rejected by payload control-select")
	}
	if payload := w.payloadStateLocked(buildPos).Payload; payload != nil {
		t.Fatalf("expected building payload to stay empty, got %+v", payload)
	}
	found := false
	for _, ent := range w.Model().Entities {
		if ent.ID == unitID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected spawnedByCore unit %d to remain in world", unitID)
	}
}

func TestControlSelectPayloadUnitPackedRejectsUnitOutsideBuildingFootprint(t *testing.T) {
	unit := RawEntity{
		ID:        13,
		TypeID:    7,
		Team:      1,
		X:         8*8 + 4,
		Y:         6*8 + 4,
		Health:    90,
		MaxHealth: 90,
	}
	w, buildPacked, buildPos, unitID := newPayloadControlSelectWorld(t, unit)

	if w.ControlSelectPayloadUnitPacked(buildPacked, unitID) {
		t.Fatal("expected unit outside payload building footprint to be rejected")
	}
	if payload := w.payloadStateLocked(buildPos).Payload; payload != nil {
		t.Fatalf("expected building payload to stay empty, got %+v", payload)
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

func TestControlSelectPayloadUnitPackedRejectsUnitDisallowedByRuntimeProfile(t *testing.T) {
	unit := RawEntity{
		ID:        15,
		TypeID:    8,
		Team:      1,
		X:         6*8 + 4,
		Y:         6*8 + 4,
		Health:    90,
		MaxHealth: 90,
	}
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.BlockNames = map[int16]string{
		700: "payload-conveyor",
	}
	model.UnitNames = map[int16]string{
		8: "custom-disallowed",
	}
	w.SetModel(model)
	tile := placeTestBuilding(t, w, 6, 6, 700, 1, 0)
	buildPacked := protocol.PackPoint2(int32(tile.X), int32(tile.Y))
	buildPos := int32(tile.Y*w.Model().Width + tile.X)
	w.unitRuntimeProfilesByName["customdisallowed"] = unitRuntimeProfile{
		Name:              "customdisallowed",
		HitSize:           8,
		AllowedInPayloads: false,
	}
	unitID := w.Model().AddEntity(unit).ID

	if w.ControlSelectPayloadUnitPacked(buildPacked, unitID) {
		t.Fatal("expected runtime profile allowedInPayloads=false to reject control-select payload")
	}
	if payload := w.payloadStateLocked(buildPos).Payload; payload != nil {
		t.Fatalf("expected building payload to stay empty, got %+v", payload)
	}
}

func TestControlSelectPayloadUnitPackedRejectsPayloadMassDriver(t *testing.T) {
	unit := RawEntity{
		ID:        16,
		TypeID:    7,
		Team:      1,
		X:         6*8 + 4,
		Y:         6*8 + 4,
		Health:    90,
		MaxHealth: 90,
	}
	w, buildPacked, buildPos, unitID := newPayloadBuildingWorld(t, 702, "payload-mass-driver", 0, unit)

	if w.ControlSelectPayloadUnitPacked(buildPacked, unitID) {
		t.Fatal("expected payload control-select to reject payload-mass-driver")
	}
	if payload := w.payloadStateLocked(buildPos).Payload; payload != nil {
		t.Fatalf("expected payload-mass-driver to stay empty, got %+v", payload)
	}
}

func TestControlSelectPayloadUnitPackedSetsPayloadRouterRecDir(t *testing.T) {
	unit := RawEntity{
		ID:        17,
		TypeID:    7,
		Team:      1,
		X:         6*8 + 4,
		Y:         6*8 + 4,
		Health:    90,
		MaxHealth: 90,
	}
	w, buildPacked, buildPos, unitID := newPayloadBuildingWorld(t, 701, "payload-router", 3, unit)

	if !w.ControlSelectPayloadUnitPacked(buildPacked, unitID) {
		t.Fatal("expected payload-router control-select to accept standing unit")
	}
	state := w.payloadStateLocked(buildPos)
	if state.Payload == nil || state.Payload.Kind != payloadKindUnit {
		t.Fatalf("expected payload-router payload state to receive unit payload, got %+v", state.Payload)
	}
	if want := byte(tileRotationNorm(3)); state.RecDir != want {
		t.Fatalf("expected payload-router recDir=%d after direct insert, got %d", want, state.RecDir)
	}
}

func TestEnterUnitPayloadPackedUsesPackedBuildingPos(t *testing.T) {
	unit := RawEntity{
		ID:        14,
		TypeID:    7,
		Team:      1,
		X:         6*8 + 4,
		Y:         6*8 + 4,
		Health:    90,
		MaxHealth: 90,
	}
	w, buildPacked, buildPos, unitID := newPayloadControlSelectWorld(t, unit)

	if !w.EnterUnitPayloadPacked(buildPacked, unitID) {
		t.Fatal("expected unitEnteredPayload path to accept packed building pos")
	}
	payload := w.payloadStateLocked(buildPos).Payload
	if payload == nil || payload.Kind != payloadKindUnit {
		t.Fatalf("expected payload state to receive unit payload, got %+v", payload)
	}
}

func TestEnterUnitPayloadPackedAcceptsSpawnedByCoreUnit(t *testing.T) {
	unit := RawEntity{
		ID:            18,
		TypeID:        7,
		Team:          1,
		X:             6*8 + 4,
		Y:             6*8 + 4,
		Health:        90,
		MaxHealth:     90,
		SpawnedByCore: true,
	}
	w, buildPacked, buildPos, unitID := newPayloadControlSelectWorld(t, unit)

	if !w.EnterUnitPayloadPacked(buildPacked, unitID) {
		t.Fatal("expected unitEnteredPayload path to allow spawnedByCore unit")
	}
	if payload := w.payloadStateLocked(buildPos).Payload; payload == nil || payload.Kind != payloadKindUnit {
		t.Fatalf("expected building payload to receive spawnedByCore unit, got %+v", payload)
	}
}

func TestEnterUnitPayloadPackedDoesNotRequireStandingOnBuilding(t *testing.T) {
	unit := RawEntity{
		ID:        19,
		TypeID:    7,
		Team:      1,
		X:         8*8 + 4,
		Y:         6*8 + 4,
		Health:    90,
		MaxHealth: 90,
	}
	w, buildPacked, buildPos, unitID := newPayloadControlSelectWorld(t, unit)

	if !w.EnterUnitPayloadPacked(buildPacked, unitID) {
		t.Fatal("expected unitEnteredPayload path to skip standing-on-building check")
	}
	if payload := w.payloadStateLocked(buildPos).Payload; payload == nil || payload.Kind != payloadKindUnit {
		t.Fatalf("expected payload state to receive off-footprint unit payload, got %+v", payload)
	}
}

func TestEnterUnitPayloadPackedAcceptsPayloadMassDriver(t *testing.T) {
	unit := RawEntity{
		ID:        20,
		TypeID:    7,
		Team:      1,
		X:         6*8 + 4,
		Y:         6*8 + 4,
		Health:    90,
		MaxHealth: 90,
	}
	w, buildPacked, buildPos, unitID := newPayloadBuildingWorld(t, 702, "payload-mass-driver", 0, unit)

	if !w.EnterUnitPayloadPacked(buildPacked, unitID) {
		t.Fatal("expected unitEnteredPayload path to accept payload-mass-driver")
	}
	if payload := w.payloadStateLocked(buildPos).Payload; payload == nil || payload.Kind != payloadKindUnit {
		t.Fatalf("expected payload-mass-driver to receive unit payload, got %+v", payload)
	}
}

func TestRequestUnitPayloadRejectsCarrierWithoutPickupUnits(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.UnitNames = map[int16]string{
		7:  "dagger",
		55: "incite",
	}
	w.SetModel(model)
	w.unitRuntimeProfilesByName["incite"] = unitRuntimeProfile{
		Name:              "incite",
		HitSize:           11,
		PickupUnits:       false,
		AllowedInPayloads: true,
	}
	w.unitRuntimeProfilesByName["dagger"] = unitRuntimeProfile{
		Name:              "dagger",
		HitSize:           8,
		PickupUnits:       true,
		AllowedInPayloads: true,
	}
	carrier := w.Model().AddEntity(RawEntity{
		ID:              21,
		TypeID:          55,
		Team:            1,
		X:               80,
		Y:               80,
		Health:          200,
		MaxHealth:       200,
		PayloadCapacity: 256,
	})
	target := w.Model().AddEntity(RawEntity{
		ID:        22,
		TypeID:    7,
		Team:      1,
		X:         84,
		Y:         80,
		Health:    90,
		MaxHealth: 90,
	})

	if _, ok := w.RequestUnitPayload(carrier.ID, target.ID); ok {
		t.Fatal("expected pickupUnits=false carrier to reject unit payload pickup")
	}
	if len(w.Model().Entities) != 2 {
		t.Fatalf("expected both entities to remain after rejected pickup, got %d", len(w.Model().Entities))
	}
}

func TestRequestUnitPayloadUsesDynamicHitSizeRange(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.UnitNames = map[int16]string{
		8:  "large-carrier",
		55: "large-target",
	}
	w.SetModel(model)
	w.unitRuntimeProfilesByName["largecarrier"] = unitRuntimeProfile{
		Name:              "largecarrier",
		HitSize:           24,
		PickupUnits:       true,
		AllowedInPayloads: true,
	}
	w.unitRuntimeProfilesByName["largetarget"] = unitRuntimeProfile{
		Name:              "largetarget",
		HitSize:           20,
		PickupUnits:       true,
		AllowedInPayloads: true,
	}
	carrier := w.Model().AddEntity(RawEntity{
		ID:              31,
		TypeID:          8,
		Team:            1,
		X:               80,
		Y:               80,
		Health:          300,
		MaxHealth:       300,
		PayloadCapacity: 1024,
	})
	target := w.Model().AddEntity(RawEntity{
		ID:        32,
		TypeID:    55,
		Team:      1,
		X:         140,
		Y:         80,
		Health:    160,
		MaxHealth: 160,
	})

	updated, ok := w.RequestUnitPayload(carrier.ID, target.ID)
	if !ok {
		t.Fatal("expected dynamic payload pickup range to accept larger unit farther than vanilla fixed 32")
	}
	if len(updated.Payloads) != 1 || updated.Payloads[0].Kind != payloadKindUnit || updated.Payloads[0].UnitTypeID != 55 {
		t.Fatalf("expected carrier to receive target as unit payload, got %+v", updated.Payloads)
	}
	if len(w.Model().Entities) != 1 {
		t.Fatalf("expected picked target to be removed from world, got %d entities", len(w.Model().Entities))
	}
}

func TestRequestUnitPayloadRejectsFlyingTarget(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.UnitNames = map[int16]string{
		8:  "large-carrier",
		55: "large-target",
	}
	w.SetModel(model)
	w.unitRuntimeProfilesByName["largecarrier"] = unitRuntimeProfile{
		Name:              "largecarrier",
		HitSize:           24,
		PickupUnits:       true,
		AllowedInPayloads: true,
	}
	w.unitRuntimeProfilesByName["largetarget"] = unitRuntimeProfile{
		Name:              "largetarget",
		HitSize:           20,
		PickupUnits:       true,
		AllowedInPayloads: true,
	}
	carrier := w.Model().AddEntity(RawEntity{
		ID:              33,
		TypeID:          8,
		Team:            1,
		X:               80,
		Y:               80,
		Health:          300,
		MaxHealth:       300,
		PayloadCapacity: 1024,
	})
	target := w.Model().AddEntity(RawEntity{
		ID:        34,
		TypeID:    55,
		Team:      1,
		X:         92,
		Y:         80,
		Health:    160,
		MaxHealth: 160,
		Flying:    true,
	})

	if _, ok := w.RequestUnitPayload(carrier.ID, target.ID); ok {
		t.Fatal("expected requestUnitPayload to reject flying target")
	}
	if len(w.Model().Entities) != 2 {
		t.Fatalf("expected flying target to remain in world, got %d entities", len(w.Model().Entities))
	}
}

func TestRequestDropPayloadWithoutUnitStateDoesNotInjectDefaultShieldOrArmor(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.UnitNames = map[int16]string{
		8:   "carrier",
		910: "plain-payload-unit",
	}
	w.SetModel(model)
	w.unitRuntimeProfilesByName["plainpayloadunit"] = unitRuntimeProfile{Name: "plain-payload-unit"}

	carrier := w.Model().AddEntity(RawEntity{
		ID:              41,
		TypeID:          8,
		Team:            1,
		X:               80,
		Y:               80,
		Health:          100,
		MaxHealth:       100,
		PayloadCapacity: 1024,
		Payloads: []payloadData{{
			Kind:       payloadKindUnit,
			UnitTypeID: 910,
			Health:     40,
			MaxHealth:  70,
		}},
		RuntimeInit: true,
		SlowMul:     1,
	})

	updated, ok := w.RequestDropPayload(carrier.ID, 96, 80)
	if !ok {
		t.Fatal("expected requestDropPayload to drop fallback unit payload")
	}
	if len(updated.Payloads) != 0 {
		t.Fatalf("expected carrier payloads to clear after drop, got %+v", updated.Payloads)
	}
	if len(w.Model().Entities) != 2 {
		t.Fatalf("expected carrier plus dropped unit, got %d entities", len(w.Model().Entities))
	}

	var dropped RawEntity
	found := false
	for _, ent := range w.Model().Entities {
		if ent.ID == carrier.ID {
			continue
		}
		dropped = ent
		found = true
		break
	}
	if !found {
		t.Fatal("expected dropped payload unit to be spawned")
	}
	if dropped.TypeID != 910 {
		t.Fatalf("expected dropped payload type 910, got %d", dropped.TypeID)
	}
	if dropped.Health != 40 || dropped.MaxHealth != 70 {
		t.Fatalf("expected dropped payload health 40/70, got %f/%f", dropped.Health, dropped.MaxHealth)
	}
	if dropped.Shield != 0 || dropped.ShieldMax != 0 || dropped.ShieldRegen != 0 {
		t.Fatalf("expected fallback dropped payload to avoid fake shield defaults, got shield=%f max=%f regen=%f", dropped.Shield, dropped.ShieldMax, dropped.ShieldRegen)
	}
	if dropped.Armor != 0 {
		t.Fatalf("expected fallback dropped payload to avoid fake armor default, got=%f", dropped.Armor)
	}
}

func TestRequestDropPayloadRestoresSerializedUnitPayloadState(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.UnitNames = map[int16]string{
		8:   "carrier",
		911: "serialized-payload-unit",
	}
	w.SetModel(model)
	w.unitRuntimeProfilesByName["serializedpayloadunit"] = unitRuntimeProfile{Name: "serialized-payload-unit"}

	source := RawEntity{
		TypeID:      911,
		Team:        1,
		Health:      35,
		MaxHealth:   80,
		Shield:      -2.5,
		Rotation:    45,
		RuntimeInit: true,
		SlowMul:     1,
		MineTilePos: invalidEntityTilePos,
	}
	payload := w.unitPayloadFromEntityLocked(source)
	if payload == nil {
		t.Fatal("expected serialized unit payload")
	}
	payload.UnitState = nil
	payload.UnitTypeID = -1

	carrier := w.Model().AddEntity(RawEntity{
		ID:              42,
		TypeID:          8,
		Team:            1,
		X:               80,
		Y:               80,
		Health:          100,
		MaxHealth:       100,
		PayloadCapacity: 1024,
		Payloads:        []payloadData{clonePayloadData(*payload)},
		RuntimeInit:     true,
		SlowMul:         1,
	})

	updated, ok := w.RequestDropPayload(carrier.ID, 96, 80)
	if !ok {
		t.Fatal("expected requestDropPayload to restore serialized unit payload state")
	}
	if len(updated.Payloads) != 0 {
		t.Fatalf("expected carrier payloads to clear after drop, got %+v", updated.Payloads)
	}

	var dropped RawEntity
	found := false
	for _, ent := range w.Model().Entities {
		if ent.ID == carrier.ID {
			continue
		}
		dropped = ent
		found = true
		break
	}
	if !found {
		t.Fatal("expected dropped serialized payload unit to be spawned")
	}
	if dropped.TypeID != 911 {
		t.Fatalf("expected dropped serialized payload type 911, got %d", dropped.TypeID)
	}
	if dropped.Health != 35 || dropped.MaxHealth != 80 {
		t.Fatalf("expected dropped serialized payload health 35/80, got %f/%f", dropped.Health, dropped.MaxHealth)
	}
	if dropped.Shield != -2.5 {
		t.Fatalf("expected dropped serialized payload shield -2.5, got %f", dropped.Shield)
	}
}

func TestRequestBuildPayloadPackedTakesCurrentPayloadBeforeBuilding(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.BlockNames = map[int16]string{
		700: "payload-conveyor",
	}
	model.UnitNames = map[int16]string{
		8:  "dagger",
		55: "large-carrier",
	}
	w.SetModel(model)
	w.unitRuntimeProfilesByName["dagger"] = unitRuntimeProfile{
		Name:              "dagger",
		HitSize:           8,
		AllowedInPayloads: true,
	}
	w.unitRuntimeProfilesByName["largecarrier"] = unitRuntimeProfile{
		Name:              "largecarrier",
		HitSize:           24,
		PickupUnits:       true,
		AllowedInPayloads: true,
	}
	tile := placeTestBuilding(t, w, 6, 6, 700, 1, 0)
	buildPos := int32(tile.Y*w.Model().Width + tile.X)
	buildPacked := protocol.PackPoint2(int32(tile.X), int32(tile.Y))
	setTestPayload(t, w, 6, 6, &payloadData{
		Kind:       payloadKindUnit,
		UnitTypeID: 8,
		Health:     90,
		MaxHealth:  90,
		UnitState: &RawEntity{
			TypeID:    8,
			Health:    90,
			MaxHealth: 90,
		},
	})
	carrier := w.Model().AddEntity(RawEntity{
		ID:              35,
		TypeID:          55,
		Team:            1,
		X:               6*8 + 4,
		Y:               6*8 + 4,
		Health:          300,
		MaxHealth:       300,
		PayloadCapacity: 1024,
	})

	updated, ok := w.RequestBuildPayloadPacked(carrier.ID, buildPacked)
	if !ok {
		t.Fatal("expected requestBuildPayload to take current building payload before detaching building")
	}
	if got := w.payloadStateLocked(buildPos).Payload; got != nil {
		t.Fatalf("expected building payload slot to be emptied after pickup, got %+v", got)
	}
	if tile.Block != 700 || tile.Build == nil {
		t.Fatalf("expected building to remain placed after taking internal payload, tile=%+v", tile)
	}
	if len(updated.Payloads) != 1 || updated.Payloads[0].Kind != payloadKindUnit || updated.Payloads[0].UnitTypeID != 8 {
		t.Fatalf("expected carrier to receive building's current payload, got %+v", updated.Payloads)
	}
}

func TestTryInsertItemLockedRejectsUnrelatedCrafterInputs(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		183: "silicon-smelter",
		418: "router",
	}
	w.SetModel(model)
	src := placeTestBuilding(t, w, 2, 2, 418, 1, 0)
	dst := placeTestBuilding(t, w, 3, 2, 183, 1, 0)
	srcPos := int32(src.Y*w.Model().Width + src.X)
	dstPos := int32(dst.Y*w.Model().Width + dst.X)

	if !w.canAcceptItemLocked(srcPos, dstPos, coalItemID, 0) {
		t.Fatalf("expected silicon smelter to accept required coal input")
	}
	if !w.canAcceptItemLocked(srcPos, dstPos, sandItemID, 0) {
		t.Fatalf("expected silicon smelter to accept required sand input")
	}
	if w.canAcceptItemLocked(srcPos, dstPos, leadItemID, 0) {
		t.Fatalf("expected silicon smelter to reject unrelated lead input")
	}
	if w.tryInsertItemLocked(srcPos, dstPos, leadItemID, 0) {
		t.Fatalf("expected tryInsertItemLocked to reject unrelated crafter input")
	}
	if got := dst.Build.ItemAmount(leadItemID); got != 0 {
		t.Fatalf("expected rejected lead input to leave crafter inventory unchanged, got %d", got)
	}
	if !w.tryInsertItemLocked(srcPos, dstPos, coalItemID, 0) {
		t.Fatalf("expected tryInsertItemLocked to insert required coal input")
	}
	if got := dst.Build.ItemAmount(coalItemID); got != 1 {
		t.Fatalf("expected inserted coal amount 1, got %d", got)
	}
}

func setTestPayload(t *testing.T, w *World, x, y int, payload *payloadData) int32 {
	t.Helper()
	pos := int32(y*w.Model().Width + x)
	tile, err := w.Model().TileAt(x, y)
	if err != nil || tile == nil || tile.Build == nil {
		t.Fatalf("payload tile lookup failed at (%d,%d): %v", x, y, err)
	}
	st := w.payloadStateLocked(pos)
	st.Payload = payload
	st.Move = 0
	st.Work = 0
	st.Exporting = false
	w.syncPayloadTileLocked(tile, payload)
	return pos
}

func TestWorldSnapshot(t *testing.T) {
	w := New(Config{TPS: 60})
	before := w.Snapshot()
	w.Step(500 * time.Millisecond)
	after := w.Snapshot()
	if after.WaveTime <= before.WaveTime {
		t.Fatalf("expected wavetime to increase, before=%v after=%v", before.WaveTime, after.WaveTime)
	}
	if after.Tps != 60 {
		t.Fatalf("expected tps=60, got=%d", after.Tps)
	}
}

func TestApplyBuildPlansIsAsync(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		45:  "duo",
		339: "core-shard",
	}
	w.SetModel(model)
	core := placeTestBuilding(t, w, 1, 1, 339, 1, 0)
	core.Build.AddItem(0, 100)

	ops := []BuildPlanOp{{
		Breaking: false,
		X:        2,
		Y:        3,
		Rotation: 1,
		BlockID:  45,
	}}
	w.ApplyBuildPlans(TeamID(1), ops)

	tile, err := w.Model().TileAt(2, 3)
	if err != nil || tile == nil {
		t.Fatalf("tile lookup failed: %v", err)
	}
	if tile.Block != 0 || tile.Build != nil {
		t.Fatalf("expected no immediate placement, got block=%d build=%v", tile.Block, tile.Build != nil)
	}

	w.Step(500 * time.Millisecond)
	tile, _ = w.Model().TileAt(2, 3)
	if tile.Block != 0 || tile.Build != nil {
		t.Fatalf("expected still pending build at 0.5s, got block=%d build=%v", tile.Block, tile.Build != nil)
	}

	placed := false
	for i := 0; i < 16; i++ { // up to 3.2s
		w.Step(200 * time.Millisecond)
		tile, _ = w.Model().TileAt(2, 3)
		if tile.Block == 45 && tile.Build != nil {
			placed = true
			break
		}
	}
	if !placed {
		tile, _ = w.Model().TileAt(2, 3)
		t.Fatalf("expected placed block after progress, got block=%d build=%v", tile.Block, tile.Build != nil)
	}
}

func TestBuildPlanWaitsForMaterialsInsteadOfRejecting(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		45:  "duo",
		339: "core-shard",
	}
	w.SetModel(model)
	core := placeTestBuilding(t, w, 1, 1, 339, 1, 0)
	pos := int32(2 + 2*model.Width)

	w.ApplyBuildPlans(TeamID(1), []BuildPlanOp{{
		X: 2, Y: 2, BlockID: 45,
	}})
	if _, ok := w.pendingBuilds[pos]; !ok {
		t.Fatalf("expected duo plan to stay queued without materials")
	}

	stepForSeconds(w, 1)
	st, ok := w.pendingBuilds[pos]
	if !ok {
		t.Fatalf("expected queued plan to remain pending while missing all copper")
	}
	if st.VisualPlaced {
		t.Fatalf("expected build to wait for first material before beginPlace semantics")
	}

	core.Build.AddItem(0, 1)
	stepForSeconds(w, 0.2)
	st, ok = w.pendingBuilds[pos]
	if !ok {
		t.Fatalf("expected pending build after only 1 copper")
	}
	if !st.VisualPlaced {
		t.Fatalf("expected build to begin once one copper became available")
	}

	core.Build.AddItem(0, 34)
	built := false
	for i := 0; i < 40; i++ {
		w.Step(200 * time.Millisecond)
		tile, _ := w.Model().TileAt(2, 2)
		if tile.Block == 45 && tile.Build != nil {
			built = true
			break
		}
	}
	if !built {
		tile, _ := w.Model().TileAt(2, 2)
		t.Fatalf("expected duo to finish after remaining copper arrived, got block=%d build=%v", tile.Block, tile.Build != nil)
	}
}

func TestCancelPendingBuildRefundsOnlyConsumedItems(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		45:  "duo",
		339: "core-shard",
	}
	w.SetModel(model)
	core := placeTestBuilding(t, w, 1, 1, 339, 1, 0)
	core.Build.AddItem(0, 10)

	w.ApplyBuildPlans(TeamID(1), []BuildPlanOp{{
		X: 2, Y: 2, BlockID: 45,
	}})

	spent := false
	for i := 0; i < 120; i++ {
		w.Step(time.Second / 60)
		if core.Build.ItemAmount(0) < 10 {
			spent = true
			break
		}
	}
	if !spent {
		t.Fatalf("expected partial duo build to consume some copper before cancellation")
	}
	beforeCancel := core.Build.ItemAmount(0)
	if beforeCancel <= 0 || beforeCancel >= 10 {
		t.Fatalf("expected only a partial spend before cancel, got %d", beforeCancel)
	}

	w.CancelBuildAt(2, 2, false)

	if _, ok := w.pendingBuilds[int32(2+2*model.Width)]; ok {
		t.Fatalf("expected pending build removed after cancel")
	}
	if got := core.Build.ItemAmount(0); got != 10 {
		t.Fatalf("expected cancel to refund only consumed copper back to original 10, got %d", got)
	}
}

func TestLaterBuildPlansStayQueuedWhenItemsRunOut(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		45:  "duo",
		339: "core-shard",
	}
	w.SetModel(model)
	core := placeTestBuilding(t, w, 1, 1, 339, 1, 0)
	core.Build.AddItem(0, 35)

	posA := int32(2 + 2*model.Width)
	posB := int32(3 + 2*model.Width)
	w.ApplyBuildPlans(TeamID(1), []BuildPlanOp{
		{X: 2, Y: 2, BlockID: 45},
		{X: 3, Y: 2, BlockID: 45},
	})

	if len(w.pendingBuilds) != 2 {
		t.Fatalf("expected both build plans queued instead of rejecting the later one, pending=%d", len(w.pendingBuilds))
	}
	if _, ok := w.pendingBuilds[posA]; !ok {
		t.Fatalf("expected first pending build to exist")
	}
	if _, ok := w.pendingBuilds[posB]; !ok {
		t.Fatalf("expected second pending build to remain queued")
	}

	firstBuilt := false
	for i := 0; i < 40; i++ {
		w.Step(200 * time.Millisecond)
		tile, _ := w.Model().TileAt(2, 2)
		if tile.Block == 45 && tile.Build != nil {
			firstBuilt = true
			break
		}
	}
	if !firstBuilt {
		tile, _ := w.Model().TileAt(2, 2)
		t.Fatalf("expected first duo to finish, got block=%d build=%v", tile.Block, tile.Build != nil)
	}
	if _, ok := w.pendingBuilds[posB]; !ok {
		t.Fatalf("expected second plan to still be queued after first consumed all copper")
	}

	core.Build.AddItem(0, 35)
	secondBuilt := false
	for i := 0; i < 40; i++ {
		w.Step(200 * time.Millisecond)
		tile, _ := w.Model().TileAt(3, 2)
		if tile.Block == 45 && tile.Build != nil {
			secondBuilt = true
			break
		}
	}
	if !secondBuilt {
		tile, _ := w.Model().TileAt(3, 2)
		t.Fatalf("expected second duo to finish after copper refill, got block=%d build=%v", tile.Block, tile.Build != nil)
	}
}

func TestSetModelResetsRulesBetweenMaps(t *testing.T) {
	w := New(Config{TPS: 60})

	first := NewWorldModel(8, 8)
	first.Tags = map[string]string{
		"rules": `{"attackMode":true,"enemyCoreBuildRadius":123,"defaultTeam":"blue"}`,
	}
	w.SetModel(first)

	rules := w.GetRulesManager().Get()
	if !rules.AttackMode || rules.EnemyCoreBuildRadius != 123 || rules.DefaultTeam != "blue" {
		t.Fatalf("expected first map rules to apply, got attack=%v radius=%v defaultTeam=%q", rules.AttackMode, rules.EnemyCoreBuildRadius, rules.DefaultTeam)
	}

	second := NewWorldModel(8, 8)
	w.SetModel(second)

	rules = w.GetRulesManager().Get()
	if rules.AttackMode {
		t.Fatalf("expected attack mode reset to default false")
	}
	if rules.EnemyCoreBuildRadius != 400 {
		t.Fatalf("expected enemy core build radius reset to default 400, got %v", rules.EnemyCoreBuildRadius)
	}
	if rules.DefaultTeam != "sharded" {
		t.Fatalf("expected default team reset to sharded, got %q", rules.DefaultTeam)
	}
}

func TestSetModelInfersAttackGamemodeFromMultipleCoreTeams(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		339: "core-shard",
	}
	tileA, _ := model.TileAt(1, 1)
	tileA.Block = 339
	tileA.Team = 1
	tileB, _ := model.TileAt(6, 6)
	tileB.Block = 339
	tileB.Team = 2

	w.SetModel(model)

	rules := w.GetRulesManager().Get()
	if !rules.AttackMode {
		t.Fatalf("expected attack mode inferred from multi-team cores")
	}
	if rules.WaveSpacing != 120.0 {
		t.Fatalf("expected attack mode wave spacing=120, got %v", rules.WaveSpacing)
	}
	if rules.InfiniteResources {
		t.Fatalf("expected attack mode not to enable global infinite resources")
	}
	if !rules.teamInfiniteResources(2) {
		t.Fatalf("expected attack mode to grant infinite resources to wave team")
	}
}

func TestSetModelAppliesSandboxDefaultsBeforeRuleOverlay(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.Tags = map[string]string{
		"rules": `{"infiniteResources":true}`,
	}

	w.SetModel(model)

	rules := w.GetRulesManager().Get()
	if !rules.InfiniteResources {
		t.Fatalf("expected sandbox infinite resources")
	}
	if !rules.AllowEditRules {
		t.Fatalf("expected sandbox allowEditRules default to be applied")
	}
	if rules.WaveTimer {
		t.Fatalf("expected sandbox waveTimer=false")
	}
}

func TestSetModelAppliesEditorDefaultsBeforeRuleOverlay(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.Tags = map[string]string{
		"rules": `{"editor":true}`,
	}

	w.SetModel(model)

	rules := w.GetRulesManager().Get()
	if !rules.Editor || !rules.InstantBuild || !rules.InfiniteResources {
		t.Fatalf("expected editor defaults, got editor=%v instant=%v infinite=%v", rules.Editor, rules.InstantBuild, rules.InfiniteResources)
	}
	if rules.Waves || rules.WaveTimer {
		t.Fatalf("expected editor to disable waves and timer, got waves=%v timer=%v", rules.Waves, rules.WaveTimer)
	}
}

func TestSetModelPrefersExplicitMapModeBeforeOverlayHeuristics(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.Tags = map[string]string{
		"mode":  "survival",
		"rules": `{"infiniteResources":true}`,
	}

	w.SetModel(model)

	rules := w.GetRulesManager().Get()
	if rules.ModeName != "survival" {
		t.Fatalf("expected explicit map mode survival, got %q", rules.ModeName)
	}
	if !rules.InfiniteResources {
		t.Fatalf("expected overlay infiniteResources to remain enabled")
	}
	if !rules.Waves || !rules.WaveTimer {
		t.Fatalf("expected survival defaults to remain active, got waves=%v timer=%v", rules.Waves, rules.WaveTimer)
	}
}

func TestDeconstructRefund(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		45:  "duo",
		339: "core-shard",
	}
	w.SetModel(model)
	core := placeTestBuilding(t, w, 1, 1, 339, 1, 0)
	core.Build.AddItem(0, 100)

	w.ApplyBuildPlans(TeamID(1), []BuildPlanOp{{
		X: 2, Y: 2, BlockID: 45,
	}})
	w.Step(3 * time.Second)
	mid := w.TeamItems(TeamID(1))[0]
	if mid >= 3000 {
		t.Fatalf("expected build to consume copper from starter inventory, mid=%d", mid)
	}

	w.ApplyBuildPlans(TeamID(1), []BuildPlanOp{{
		Breaking: true, X: 2, Y: 2,
	}})
	tile, _ := w.Model().TileAt(2, 2)
	_, refundStacks := w.deconstructRefundStacks(tile, TeamID(1))
	expectedRefund := int32(0)
	for _, stack := range refundStacks {
		if stack.Item == 0 {
			expectedRefund = stack.Amount
			break
		}
	}
	if expectedRefund <= 0 {
		t.Fatalf("expected duo deconstruct to refund copper, refund=%v", refundStacks)
	}
	breakDuration := w.buildDurationSecondsForTeam(45, TeamID(1), w.GetRulesManager().Get())
	stepForSeconds(w, breakDuration*0.6)
	during := w.TeamItems(TeamID(1))[0]
	if during <= mid {
		t.Fatalf("expected deconstruct progress to refund during dismantle, mid=%d during=%d", mid, during)
	}
	if during >= mid+expectedRefund {
		t.Fatalf("expected partial refund before dismantle completion, mid=%d during=%d expected_final=%d", mid, during, mid+expectedRefund)
	}

	stepForSeconds(w, breakDuration)
	after := w.TeamItems(TeamID(1))[0]
	if after != mid+expectedRefund {
		t.Fatalf("expected exact final refund after deconstruct, mid=%d during=%d after=%d expected=%d", mid, during, after, mid+expectedRefund)
	}
}

func TestBuilderDurationUsesOwnerUnitSpeedAndIgnoresUnitBuildSpeedRule(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		45: "duo",
	}
	model.UnitNames = map[int16]string{
		35: "alpha",
		37: "gamma",
	}
	w.SetModel(model)
	w.blockBuildTimesByName = map[string]float32{"duo": 2}

	rules := DefaultRules()
	rules.UnitBuildSpeedMultiplier = 4
	rules.BuildSpeedMultiplier = 1
	w.GetRulesManager().Set(rules)
	w.SetTeamBuilderSpeed(1, 0.5)

	if _, err := w.AddEntityWithID(37, 9001, 20, 20, 1); err != nil {
		t.Fatalf("add gamma entity: %v", err)
	}
	w.UpdateBuilderState(101, 1, 9001, 20, 20, true, 220)

	got := w.buildDurationSecondsForOwnerLocked(45, 101, 1, w.GetRulesManager().Get())
	want := w.blockBuildTimesByName["duo"]
	if want <= 0 {
		t.Fatalf("expected duo build time metadata")
	}
	want /= 1.0 // gamma buildSpeed
	if math.Abs(float64(got-want)) > 0.0001 {
		t.Fatalf("expected owner gamma build speed to decide duration and ignore unitBuildSpeedMultiplier, want=%f got=%f", want, got)
	}
}

func TestBuildAndDeconstructProgressHealthUseConstructBlockScale(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		45:  "duo",
		339: "core-shard",
	}
	model.UnitNames = map[int16]string{
		35: "alpha",
	}
	w.SetModel(model)
	w.blockBuildTimesByName = map[string]float32{"duo": 1.5}

	core := placeTestBuilding(t, w, 4, 4, 339, 1, 0)
	core.Build.AddItem(0, 100)
	owner := int32(101)
	team := TeamID(1)
	if _, err := w.AddEntityWithID(35, 9001, 20, 20, 1); err != nil {
		t.Fatalf("add alpha entity: %v", err)
	}
	w.UpdateBuilderState(owner, team, 9001, float32(2*8+4), float32(2*8+4), true, 220)

	w.ApplyBuildPlanSnapshotForOwner(owner, team, []BuildPlanOp{{
		X: 2, Y: 2, BlockID: 45,
	}})

	for i := 0; i < 200; i++ {
		w.Step(time.Second / 60)
		tile, _ := w.Model().TileAt(2, 2)
		if tile.Block != 0 || tile.Build != nil {
			break
		}
		for _, ev := range w.DrainEntityEvents() {
			if ev.Kind == EntityEventBuildHealth && ev.BuildHP > constructBlockHealthMax {
				t.Fatalf("expected in-progress build health to stay on construct scale <= %.0f, got=%f", constructBlockHealthMax, ev.BuildHP)
			}
		}
	}
	_ = w.DrainEntityEvents()

	w.ApplyBuildPlanSnapshotForOwner(owner, team, []BuildPlanOp{{
		Breaking: true, X: 2, Y: 2,
	}})

	for i := 0; i < 200; i++ {
		w.Step(time.Second / 60)
		tile, _ := w.Model().TileAt(2, 2)
		if tile.Block == 0 && tile.Build == nil {
			break
		}
		for _, ev := range w.DrainEntityEvents() {
			if ev.Kind == EntityEventBuildHealth && ev.BuildHP > constructBlockHealthMax {
				t.Fatalf("expected in-progress deconstruct health to stay on construct scale <= %.0f, got=%f", constructBlockHealthMax, ev.BuildHP)
			}
		}
	}
}

func TestFactoryProductionSpawnsUnit(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		100: "ground-factory",
		339: "core-shard",
		421: "battery",
		422: "power-node",
	}
	model.UnitNames = map[int16]string{7: "dagger"}
	w.SetModel(model)
	core := placeTestBuilding(t, w, 1, 10, 339, 1, 0)
	core.Build.AddItem(copperItemID, 200)
	core.Build.AddItem(leadItemID, 200)
	core.Build.AddItem(siliconItemID, 200)
	placeTestBuilding(t, w, 3, 8, 421, 1, 0)
	placeTestBuilding(t, w, 3, 6, 422, 1, 0)
	w.powerStorageState[int32(8*model.Width+3)] = 4000

	factory := placeTestBuilding(t, w, 3, 3, 100, 1, 0)
	factory.Build.AddItem(siliconItemID, 10)
	factory.Build.AddItem(leadItemID, 10)
	linkPowerNode(t, w, 3, 6, protocol.Point2{X: 0, Y: -3}, protocol.Point2{X: 0, Y: 2})
	nodePos := int32(6*model.Width + 3)
	if got := len(w.powerNodeLinks[nodePos]); got != 2 {
		t.Fatalf("expected power node to keep 2 links, got=%d links=%v", got, w.powerNodeLinks[nodePos])
	}
	w.Step(time.Second / 60)
	if st := w.teamPowerStates[1]; st == nil || st.Capacity <= 0 || st.Stored <= 0 {
		t.Fatalf("expected linked battery power for factory, state=%+v", st)
	}
	stepForSeconds(w, 9)
	if len(w.Model().Entities) != 0 {
		t.Fatalf("expected no unit before factory cycle, got=%d", len(w.Model().Entities))
	}
	stepForSeconds(w, 7)
	if len(w.Model().Entities) == 0 {
		t.Fatalf("expected produced unit, got=%d", len(w.Model().Entities))
	}
	factoryPos := int32(3 + 3*model.Width)
	if got := w.payloadStateLocked(factoryPos).Payload; got != nil {
		t.Fatalf("expected factory payload to dump after spawning, got=%+v", got)
	}
	if got := core.Build.ItemAmount(siliconItemID); got != 200 {
		t.Fatalf("expected factory production to keep core silicon unchanged, got=%d", got)
	}
	if got := core.Build.ItemAmount(leadItemID); got != 200 {
		t.Fatalf("expected factory production to keep core lead unchanged, got=%d", got)
	}
}

func TestFactoryProductionStallsWithoutPower(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		100: "ground-factory",
		339: "core-shard",
	}
	model.UnitNames = map[int16]string{7: "dagger"}
	w.SetModel(model)
	core := placeTestBuilding(t, w, 1, 10, 339, 1, 0)
	core.Build.AddItem(0, 200)
	core.Build.AddItem(1, 200)
	core.Build.AddItem(2, 200)

	factory := placeTestBuilding(t, w, 3, 3, 100, 1, 0)
	factory.Build.AddItem(siliconItemID, 10)
	factory.Build.AddItem(leadItemID, 10)
	stepForSeconds(w, 20)

	if got := len(w.Model().Entities); got != 0 {
		t.Fatalf("expected unpowered factory to stay idle, entities=%d", got)
	}
}

func TestFactoryProductionProgressStallsAndResumesWithPowerRestore(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		100: "ground-factory",
		339: "core-shard",
		421: "battery",
		422: "power-node",
	}
	model.UnitNames = map[int16]string{7: "dagger"}
	w.SetModel(model)
	core := placeTestBuilding(t, w, 1, 10, 339, 1, 0)
	core.Build.AddItem(copperItemID, 200)
	core.Build.AddItem(leadItemID, 200)
	core.Build.AddItem(siliconItemID, 200)
	placeTestBuilding(t, w, 3, 8, 421, 1, 0)
	placeTestBuilding(t, w, 3, 6, 422, 1, 0)
	batteryPos := int32(8*model.Width + 3)
	w.powerStorageState[batteryPos] = 4000

	factory := placeTestBuilding(t, w, 3, 3, 100, 1, 0)
	factory.Build.AddItem(siliconItemID, 10)
	factory.Build.AddItem(leadItemID, 10)
	linkPowerNode(t, w, 3, 6, protocol.Point2{X: 0, Y: -3}, protocol.Point2{X: 0, Y: 2})

	factoryPos := int32(3 + 3*model.Width)
	nodePos := int32(6*model.Width + 3)
	stepForSeconds(w, 5)

	progressBeforeLoss := w.factoryStates[factoryPos].Progress
	if progressBeforeLoss <= 0 || progressBeforeLoss >= unitFactoryPlansByBlockName["ground-factory"][0].TimeFrames {
		t.Fatalf("expected in-flight factory progress before power loss, got %f", progressBeforeLoss)
	}

	w.applyBuildingConfigLocked(nodePos, nil, true)
	w.Step(time.Second / 60)
	progressAtLoss := w.factoryStates[factoryPos].Progress
	stepForSeconds(w, 3)
	progressDuringLoss := w.factoryStates[factoryPos].Progress
	if math.Abs(float64(progressDuringLoss-progressAtLoss)) > 0.0001 {
		t.Fatalf("expected power loss to stall progress instead of resetting, before=%f after=%f", progressAtLoss, progressDuringLoss)
	}

	w.powerStorageState[batteryPos] = 4000
	linkPowerNode(t, w, 3, 6, protocol.Point2{X: 0, Y: -3}, protocol.Point2{X: 0, Y: 2})
	stepForSeconds(w, 2)
	progressAfterRestore := w.factoryStates[factoryPos].Progress
	if progressAfterRestore <= progressDuringLoss {
		t.Fatalf("expected restored power to resume progress, stalled=%f restored=%f", progressDuringLoss, progressAfterRestore)
	}

	stepForSeconds(w, 10)
	if len(w.Model().Entities) == 0 {
		t.Fatalf("expected restored factory to finish production, entities=%d progress=%f", len(w.Model().Entities), w.factoryStates[factoryPos].Progress)
	}
}

func TestFactoryProductionOutputsUnitPayloadToPayloadConveyor(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		100: "ground-factory",
		339: "core-shard",
		421: "battery",
		422: "power-node",
		700: "payload-conveyor",
	}
	model.UnitNames = map[int16]string{7: "dagger"}
	w.SetModel(model)
	rules := DefaultRules()
	rules.Waves = false
	w.GetRulesManager().Set(rules)
	core := placeTestBuilding(t, w, 1, 10, 339, 1, 0)
	core.Build.AddItem(0, 2000)
	core.Build.AddItem(1, 2000)
	core.Build.AddItem(2, 2000)
	placeTestBuilding(t, w, 3, 8, 421, 1, 0)
	placeTestBuilding(t, w, 3, 6, 422, 1, 0)
	w.powerStorageState[int32(8*model.Width+3)] = 4000
	placeTestBuilding(t, w, 5, 3, 700, 1, 0)

	factory := placeTestBuilding(t, w, 3, 3, 100, 1, 0)
	factory.Build.AddItem(siliconItemID, 10)
	factory.Build.AddItem(leadItemID, 10)
	linkPowerNode(t, w, 3, 6, protocol.Point2{X: 0, Y: -3}, protocol.Point2{X: 0, Y: 2})
	stepForSeconds(w, 17)

	if got := len(w.Model().Entities); got != 0 {
		t.Fatalf("expected unit to stay as payload when conveyor is in front, got entities=%d", got)
	}
	conveyorPos := int32(5 + 3*model.Width)
	payload := w.payloadStateLocked(conveyorPos).Payload
	if payload == nil || payload.Kind != payloadKindUnit || payload.UnitTypeID != 7 {
		t.Fatalf("expected conveyor to receive dagger unit payload, got=%+v", payload)
	}
}

func TestFactoryUnitPayloadUsesOfficialEntityWriteHeader(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		100: "ground-factory",
	}
	model.UnitNames = map[int16]string{
		7: "dagger",
	}
	w.SetModel(model)
	factory := placeTestBuilding(t, w, 3, 3, 100, 1, 0)

	payload := w.newFactoryUnitPayloadLocked(factory, factoryState{UnitType: 7})
	if payload == nil {
		t.Fatal("expected factory unit payload")
	}
	r := protocol.NewReader(payload.Serialized)
	exists, err := r.ReadBool()
	if err != nil {
		t.Fatalf("read payload exists failed: %v", err)
	}
	if !exists {
		t.Fatal("expected payload exists flag to be true")
	}
	kind, err := r.ReadByte()
	if err != nil {
		t.Fatalf("read payload kind failed: %v", err)
	}
	if kind != protocol.PayloadUnit {
		t.Fatalf("expected payload kind unit, got %d", kind)
	}
	classID, err := r.ReadByte()
	if err != nil {
		t.Fatalf("read unit payload class id failed: %v", err)
	}
	if classID != 4 {
		t.Fatalf("expected dagger payload class id 4, got %d", classID)
	}
	revision, err := r.ReadInt16()
	if err != nil {
		t.Fatalf("read unit payload revision failed: %v", err)
	}
	if revision != 7 {
		t.Fatalf("expected dagger payload revision 7, got %d", revision)
	}
}

func TestFactoryProductionHonorsCoreUnitCap(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		100: "ground-factory",
		339: "core-shard",
		421: "battery",
		422: "power-node",
	}
	model.UnitNames = map[int16]string{7: "dagger"}
	w.SetModel(model)
	core := placeTestBuilding(t, w, 1, 1, 339, 1, 0)
	core.Build.AddItem(0, 2000)
	core.Build.AddItem(1, 2000)
	core.Build.AddItem(2, 2000)
	placeTestBuilding(t, w, 3, 8, 421, 1, 0)
	placeTestBuilding(t, w, 3, 6, 422, 1, 0)
	w.powerStorageState[int32(8*model.Width+3)] = 4000

	placeTestBuilding(t, w, 3, 3, 100, 1, 0)
	factoryPos := int32(3 + 3*model.Width)
	linkPowerNode(t, w, 3, 6, protocol.Point2{X: 0, Y: -3}, protocol.Point2{X: 0, Y: 2})

	countUnits := func() int {
		total := 0
		for _, e := range w.Model().Entities {
			if e.Team == 1 && e.TypeID == 7 {
				total++
			}
		}
		return total
	}

	for i := 0; i < 8; i++ {
		w.Model().AddEntity(w.newProducedUnitEntityLocked(7, 1, 60+float32(i*4), 80, 0))
	}
	if got := countUnits(); got != 8 {
		t.Fatalf("expected preseeded shard cap count 8, got=%d", got)
	}

	factoryTile := &w.Model().Tiles[factoryPos]
	if factoryTile.Build == nil || factoryTile.Block == 0 {
		t.Fatalf("expected factory to remain placed before capped cycle, block=%d build=%v entities=%d", factoryTile.Block, factoryTile.Build != nil, len(w.Model().Entities))
	}
	factoryTile.Build.AddItem(siliconItemID, 10)
	factoryTile.Build.AddItem(leadItemID, 10)
	stepForSeconds(w, 17)
	if got := countUnits(); got != 8 {
		t.Fatalf("expected 9th unit to remain blocked by cap, got=%d", got)
	}
	if payload := w.payloadStateLocked(factoryPos).Payload; payload == nil || payload.Kind != payloadKindUnit {
		t.Fatalf("expected capped factory to hold a unit payload, got=%+v", payload)
	}

	removedID := w.Model().Entities[0].ID
	if _, ok := w.model.RemoveEntity(removedID); !ok {
		t.Fatalf("expected to remove one capped unit id=%d", removedID)
	}
	w.Step(time.Second / 60)
	if got := countUnits(); got != 8 {
		t.Fatalf("expected held payload to dump after cap freed, got=%d", got)
	}
	if payload := w.payloadStateLocked(factoryPos).Payload; payload != nil {
		t.Fatalf("expected held payload to clear after dump, got=%+v", payload)
	}
}

func TestBuildPlanSnapshotClearsOnlyCurrentOwner(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		45:  "duo",
		46:  "duo",
		339: "core-shard",
	}
	w.SetModel(model)
	core := placeTestBuilding(t, w, 0, 0, 339, 1, 0)
	core.Build.AddItem(0, 100)

	ownerA := int32(101)
	ownerB := int32(202)
	team := TeamID(1)
	w.UpdateBuilderState(ownerA, team, 9001, float32(1*8+4), float32(1*8+4), true, 220)
	w.UpdateBuilderState(ownerB, team, 9002, float32(2*8+4), float32(2*8+4), true, 220)

	w.ApplyBuildPlanSnapshotForOwner(ownerA, team, []BuildPlanOp{{
		X: 1, Y: 1, BlockID: 45,
	}})
	w.ApplyBuildPlanSnapshotForOwner(ownerB, team, []BuildPlanOp{{
		X: 2, Y: 2, BlockID: 46,
	}})

	// Owner A sends an empty authoritative snapshot, equivalent to Q-clearing plans.
	w.ApplyBuildPlanSnapshotForOwner(ownerA, team, nil)

	// Owner B's plan must remain and continue progressing.
	placed := false
	for i := 0; i < 20; i++ {
		w.Step(200 * time.Millisecond)
		tileA, _ := w.Model().TileAt(1, 1)
		if tileA.Block != 0 || tileA.Build != nil {
			t.Fatalf("owner A plan should have been cleared, got block=%d build=%v", tileA.Block, tileA.Build != nil)
		}
		tileB, _ := w.Model().TileAt(2, 2)
		if tileB.Block == 46 && tileB.Build != nil {
			placed = true
			break
		}
	}
	if !placed {
		tileB, _ := w.Model().TileAt(2, 2)
		t.Fatalf("owner B plan should remain active, got block=%d build=%v", tileB.Block, tileB.Build != nil)
	}
}

func TestCancelBuildAtForOwnerDoesNotTouchOtherOwner(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		45:  "duo",
		46:  "duo",
		339: "core-shard",
	}
	w.SetModel(model)
	core := placeTestBuilding(t, w, 0, 0, 339, 1, 0)
	core.Build.AddItem(0, 100)

	ownerA := int32(101)
	ownerB := int32(202)
	team := TeamID(1)
	w.UpdateBuilderState(ownerA, team, 9001, float32(1*8+4), float32(1*8+4), true, 220)
	w.UpdateBuilderState(ownerB, team, 9002, float32(2*8+4), float32(2*8+4), true, 220)

	w.ApplyBuildPlansForOwner(ownerA, team, []BuildPlanOp{{
		X: 1, Y: 1, BlockID: 45,
	}})
	w.ApplyBuildPlansForOwner(ownerB, team, []BuildPlanOp{{
		X: 2, Y: 2, BlockID: 46,
	}})

	w.CancelBuildAtForOwner(ownerA, 1, 1, false)

	for i := 0; i < 20; i++ {
		w.Step(200 * time.Millisecond)
	}

	tileA, _ := w.Model().TileAt(1, 1)
	if tileA.Block != 0 || tileA.Build != nil {
		t.Fatalf("owner A tile should remain empty after cancel, got block=%d build=%v", tileA.Block, tileA.Build != nil)
	}
	tileB, _ := w.Model().TileAt(2, 2)
	if tileB.Block != 46 || tileB.Build == nil {
		t.Fatalf("owner B tile should still build successfully, got block=%d build=%v", tileB.Block, tileB.Build != nil)
	}
}

func TestBuildSnapshotWaitsForActiveBuilderInRange(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		45:  "duo",
		339: "core-shard",
	}
	w.SetModel(model)
	core := placeTestBuilding(t, w, 0, 0, 339, 1, 0)
	core.Build.AddItem(0, 100)

	owner := int32(101)
	team := TeamID(1)
	w.ApplyBuildPlanSnapshotForOwner(owner, team, []BuildPlanOp{{
		X: 4, Y: 4, BlockID: 45,
	}})

	// Queue alone must not start build visuals or progress until the builder is
	// both active and inside Vars.buildingRange, mirroring BuilderComp.
	w.UpdateBuilderState(owner, team, 9001, 0, 0, false, 220)
	for i := 0; i < 10; i++ {
		w.Step(200 * time.Millisecond)
	}
	for _, ev := range w.DrainEntityEvents() {
		if ev.Kind == EntityEventBuildPlaced || ev.Kind == EntityEventBuildConstructed {
			t.Fatalf("unexpected build progress while builder inactive: %+v", ev)
		}
	}
	tile, _ := w.Model().TileAt(4, 4)
	if tile.Block != 0 || tile.Build != nil {
		t.Fatalf("expected queued plan to stay unbuilt while inactive, got block=%d build=%v", tile.Block, tile.Build != nil)
	}

	w.UpdateBuilderState(owner, team, 9001, float32(4*8+4), float32(4*8+4), true, 220)
	built := false
	for i := 0; i < 20; i++ {
		w.Step(200 * time.Millisecond)
		tile, _ = w.Model().TileAt(4, 4)
		if tile.Block == 45 && tile.Build != nil {
			built = true
			break
		}
	}
	if !built {
		tile, _ = w.Model().TileAt(4, 4)
		t.Fatalf("expected build to finish once builder became active and in range, got block=%d build=%v", tile.Block, tile.Build != nil)
	}
}

func TestBuilderVisualPlaceDoesNotRequireStartItems(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		45:  "duo",
		339: "core-shard",
	}
	w.SetModel(model)
	_ = placeTestBuilding(t, w, 0, 0, 339, 1, 0)

	owner := int32(101)
	team := TeamID(1)
	w.ApplyBuildPlanSnapshotForOwner(owner, team, []BuildPlanOp{{
		X: 4, Y: 4, BlockID: 45,
	}})
	w.UpdateBuilderState(owner, team, 9001, float32(4*8+4), float32(4*8+4), true, 220)

	w.Step(200 * time.Millisecond)

	evs := w.DrainEntityEvents()
	placed := false
	constructed := false
	for _, ev := range evs {
		if ev.Kind == EntityEventBuildPlaced {
			placed = true
		}
		if ev.Kind == EntityEventBuildConstructed {
			constructed = true
		}
	}
	if !placed {
		t.Fatal("expected active builder to emit build_placed even without starting items")
	}
	if constructed {
		t.Fatal("expected missing items to prevent immediate construction completion")
	}
}

func TestStaleBuilderStateStillProgressesBuild(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		45:  "duo",
		339: "core-shard",
	}
	w.SetModel(model)
	core := placeTestBuilding(t, w, 0, 0, 339, 1, 0)
	core.Build.AddItem(0, 100)

	owner := int32(101)
	team := TeamID(1)
	w.ApplyBuildPlanSnapshotForOwner(owner, team, []BuildPlanOp{{
		X: 4, Y: 4, BlockID: 45,
	}})
	w.UpdateBuilderState(owner, team, 9001, float32(4*8+4), float32(4*8+4), true, 220)
	state := w.builderStates[owner]
	state.UpdatedAt = time.Now().Add(-5 * time.Second)
	w.builderStates[owner] = state

	for i := 0; i < 20; i++ {
		w.Step(200 * time.Millisecond)
		tile, _ := w.Model().TileAt(4, 4)
		if tile.Block == 45 && tile.Build != nil {
			return
		}
	}
	tile, _ := w.Model().TileAt(4, 4)
	t.Fatalf("expected stale builder state to still allow progress like Java, got block=%d build=%v", tile.Block, tile.Build != nil)
}

func TestPlaceTileLockedResetsStaleCrafterRuntimeState(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		188: "melter",
		189: "cryofluid-mixer",
	}
	w.SetModel(model)

	tile, err := model.TileAt(4, 4)
	if err != nil || tile == nil {
		t.Fatalf("tile lookup failed: %v", err)
	}
	pos := int32(4*model.Width + 4)
	tile.Block = 188
	tile.Team = 1
	tile.Rotation = 0
	tile.Build = &Building{
		Block:     188,
		Team:      1,
		Rotation:  0,
		X:         4,
		Y:         4,
		Health:    1000,
		MaxHealth: 1000,
	}
	w.crafterStates[pos] = crafterRuntimeState{Progress: 0.75, Warmup: 0.5, Seed: 7}
	w.rebuildBlockOccupancyLocked()

	w.placeTileLocked(tile, 1, 189, 0, nil, 0)

	state, ok := w.crafterStates[pos]
	if !ok {
		t.Fatal("expected new crafter runtime state to exist after placement")
	}
	if state.Progress != 0 || state.Warmup != 0 || state.Seed != 0 {
		t.Fatalf("expected stale crafter runtime state to reset, got %+v", state)
	}
}

func TestSnapshotCancelEmitsBuildCancelledNotDestroyed(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		45:  "duo",
		339: "core-shard",
	}
	w.SetModel(model)
	core := placeTestBuilding(t, w, 0, 0, 339, 1, 0)
	core.Build.AddItem(0, 100)

	owner := int32(101)
	team := TeamID(1)
	w.UpdateBuilderState(owner, team, 9001, float32(1*8+4), float32(1*8+4), true, 220)
	w.ApplyBuildPlanSnapshotForOwner(owner, team, []BuildPlanOp{{
		X: 1, Y: 1, BlockID: 45,
	}})
	w.Step(200 * time.Millisecond)
	_ = w.DrainEntityEvents()

	w.ApplyBuildPlanSnapshotForOwner(owner, team, nil)
	evs := w.DrainEntityEvents()
	cancelled := false
	for _, ev := range evs {
		if ev.Kind == EntityEventBuildDestroyed {
			t.Fatalf("expected queue cancel to avoid build_destroyed, got %+v", ev)
		}
		if ev.Kind == EntityEventBuildCancelled {
			cancelled = true
		}
	}
	if !cancelled {
		t.Fatalf("expected build_cancelled event after authoritative queue clear")
	}
	tile, _ := w.Model().TileAt(1, 1)
	if tile.Block != 0 || tile.Build != nil {
		t.Fatalf("expected cancelled tile to remain empty, got block=%d build=%v", tile.Block, tile.Build != nil)
	}
}

func TestPlacementSnapshotPreservesPendingBreaks(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.BlockNames = map[int16]string{
		45:  "duo",
		339: "core-shard",
	}
	w.SetModel(model)
	core := placeTestBuilding(t, w, 0, 0, 339, 1, 0)
	core.Build.AddItem(0, 100)
	placeTestBuilding(t, w, 1, 1, 45, 1, 0)

	owner := int32(101)
	team := TeamID(1)
	w.UpdateBuilderState(owner, team, 9001, float32(1*8+4), float32(1*8+4), true, 220)
	w.ApplyBuildPlansForOwner(owner, team, []BuildPlanOp{{
		Breaking: true,
		X:        1,
		Y:        1,
	}})

	pos := int32(1 + 1*model.Width)
	if _, ok := w.pendingBreaks[pos]; !ok {
		t.Fatalf("expected pending break at pos=%d before placement snapshot reconcile", pos)
	}

	w.ApplyPlacementPlanSnapshotForOwner(owner, team, nil)

	if _, ok := w.pendingBreaks[pos]; !ok {
		t.Fatalf("expected placement snapshot reconcile to preserve pending break at pos=%d", pos)
	}
}

func TestPlayerEntityOutOfBoundsIsClampedNotRemoved(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	w.SetModel(model)

	if _, err := w.AddEntityWithID(35, 1234, -16, 12, 1); err != nil {
		t.Fatalf("add entity: %v", err)
	}
	if _, ok := w.SetEntityPlayerController(1234, 77); !ok {
		t.Fatalf("expected player controller to be set")
	}

	w.Step(time.Second / 60)

	ent, ok := w.GetEntity(1234)
	if !ok {
		t.Fatalf("expected player-controlled entity to survive out-of-bounds correction")
	}
	if ent.X < 0 || ent.Y < 0 {
		t.Fatalf("expected clamped position, got (%f,%f)", ent.X, ent.Y)
	}
}

func TestReserveEntityIDPreventsWorldAllocationCollision(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	w.SetModel(model)

	reserved := w.ReserveEntityID()
	if reserved == 0 {
		t.Fatalf("expected reserved entity id")
	}
	if _, err := w.AddEntityWithID(35, reserved, 8, 8, 1); err != nil {
		t.Fatalf("add reserved entity: %v", err)
	}
	ent, err := w.AddEntity(35, 16, 16, 2)
	if err != nil {
		t.Fatalf("add next entity: %v", err)
	}
	if ent.ID == reserved {
		t.Fatalf("expected next entity id to differ from reserved id %d", reserved)
	}
}

func TestAddEntityWithIDRejectsDuplicateID(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	w.SetModel(model)

	if _, err := w.AddEntityWithID(35, 4321, 8, 8, 1); err != nil {
		t.Fatalf("add entity: %v", err)
	}
	if _, err := w.AddEntityWithID(35, 4321, 16, 16, 1); !errors.Is(err, ErrEntityExists) {
		t.Fatalf("expected ErrEntityExists, got %v", err)
	}
}

func TestAddEntityWithIDDoesNotInjectDefaultShieldOrArmor(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.UnitNames = map[int16]string{
		910: "plain-unit",
	}
	w.SetModel(model)
	w.unitRuntimeProfilesByName["plain-unit"] = unitRuntimeProfile{Name: "plain-unit"}

	ent, err := w.AddEntityWithID(910, 5001, 8, 8, 1)
	if err != nil {
		t.Fatalf("add entity: %v", err)
	}
	if ent.Shield != 0 || ent.ShieldMax != 0 || ent.ShieldRegen != 0 {
		t.Fatalf("expected plain unit to start without default shield, got shield=%f max=%f regen=%f", ent.Shield, ent.ShieldMax, ent.ShieldRegen)
	}
	if ent.Armor != 0 {
		t.Fatalf("expected plain unit to start without default armor, got=%f", ent.Armor)
	}
}

func TestNewProducedUnitEntityDoesNotInjectDefaultShieldOrArmor(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	model.UnitNames = map[int16]string{
		911: "produced-plain-unit",
	}
	w.SetModel(model)
	w.unitRuntimeProfilesByName["produced-plain-unit"] = unitRuntimeProfile{Name: "produced-plain-unit"}

	ent := w.newProducedUnitEntityLocked(911, 1, 16, 16, 0)
	if ent.Shield != 0 || ent.ShieldMax != 0 || ent.ShieldRegen != 0 {
		t.Fatalf("expected produced plain unit to start without default shield, got shield=%f max=%f regen=%f", ent.Shield, ent.ShieldMax, ent.ShieldRegen)
	}
	if ent.Armor != 0 {
		t.Fatalf("expected produced plain unit to start without default armor, got=%f", ent.Armor)
	}
}

func TestCustomCombatDamagesPlayerControlledUnit(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(32, 32)
	w.SetModel(model)

	enemy := RawEntity{
		ID:                 1,
		TypeID:             56,
		Team:               2,
		X:                  100,
		Y:                  100,
		Health:             100,
		MaxHealth:          100,
		AttackDamage:       40,
		AttackInterval:     0.05,
		AttackRange:        160,
		AttackFireMode:     "beam",
		AttackTargetAir:    true,
		AttackTargetGround: true,
		SlowMul:            1,
		RuntimeInit:        true,
	}
	player := RawEntity{
		ID:          2,
		PlayerID:    77,
		TypeID:      37,
		Team:        1,
		X:           120,
		Y:           100,
		Health:      220,
		MaxHealth:   220,
		Shield:      0,
		ShieldMax:   0,
		SlowMul:     1,
		RuntimeInit: true,
	}
	model.Entities = append(model.Entities, enemy, player)

	for i := 0; i < 20; i++ {
		w.Step(time.Second / 60)
	}

	ent, ok := w.GetEntity(2)
	if !ok {
		t.Fatalf("expected player-controlled entity to remain present")
	}
	if ent.Health >= 220 {
		t.Fatalf("expected custom combat to damage player-controlled unit health, got=%f", ent.Health)
	}
}

func TestUnitBeamUsesBuildingDamageMultiplier(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		600: "test-wall",
	}
	w.SetModel(model)

	tile := placeTestBuilding(t, w, 12, 12, 600, 2, 0)
	tile.Build.Health = 100
	tile.Build.MaxHealth = 100

	model.Entities = append(model.Entities, RawEntity{
		ID:                   1,
		TypeID:               35,
		Team:                 1,
		X:                    float32(12*8 - 32),
		Y:                    float32(12*8 + 4),
		Health:               100,
		MaxHealth:            100,
		AttackDamage:         20,
		AttackBuildingDamage: 0.25,
		AttackInterval:       0.05,
		AttackRange:          120,
		AttackFireMode:       "beam",
		AttackBuildings:      true,
		AttackTargetGround:   true,
		SlowMul:              1,
		StatusDamageMul:      1,
		StatusHealthMul:      1,
		StatusSpeedMul:       1,
		StatusReloadMul:      1,
		StatusBuildSpeedMul:  1,
		StatusDragMul:        1,
		StatusArmorOverride:  -1,
		RuntimeInit:          true,
	})

	w.Step(time.Second / 60)

	if got := tile.Build.Health; math.Abs(float64(got-95)) > 0.01 {
		t.Fatalf("expected beam to deal 5 building damage, got health=%f", got)
	}
}

func TestProjectileUsesBuildingDamageMultiplierBelowOne(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		600: "test-wall",
	}
	w.SetModel(model)

	tile := placeTestBuilding(t, w, 12, 12, 600, 2, 0)
	tile.Build.Health = 100
	tile.Build.MaxHealth = 100

	w.bullets = append(w.bullets, simBullet{
		ID:             1,
		Team:           1,
		X:              float32(12*8 + 4),
		Y:              float32(12*8 + 4),
		Damage:         20,
		Radius:         8,
		HitBuilds:      true,
		TargetGround:   true,
		BuildingDamage: 0.25,
	})
	w.stepBullets(0, map[int32]int{}, nil, nil)

	if got := tile.Build.Health; math.Abs(float64(got-95)) > 0.01 {
		t.Fatalf("expected projectile to deal 5 building damage, got health=%f", got)
	}
}

func TestProjectileCanDealZeroBuildingDamage(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		600: "test-wall",
	}
	w.SetModel(model)

	tile := placeTestBuilding(t, w, 12, 12, 600, 2, 0)
	tile.Build.Health = 100
	tile.Build.MaxHealth = 100

	w.bullets = append(w.bullets, simBullet{
		ID:             1,
		Team:           1,
		X:              float32(12*8 + 4),
		Y:              float32(12*8 + 4),
		Damage:         20,
		Radius:         8,
		HitBuilds:      true,
		TargetGround:   true,
		BuildingDamage: 0,
	})
	w.stepBullets(0, map[int32]int{}, nil, nil)

	if got := tile.Build.Health; math.Abs(float64(got-100)) > 0.01 {
		t.Fatalf("expected zero building multiplier to deal no building damage, got health=%f", got)
	}
}

func TestBurningStatusDamagesAndExpires(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	w.SetModel(model)
	w.statusProfilesByID[1] = statusEffectProfile{
		ID:                   1,
		Name:                 "burning",
		DamageMultiplier:     1,
		HealthMultiplier:     1,
		SpeedMultiplier:      1,
		ReloadMultiplier:     1,
		BuildSpeedMultiplier: 1,
		DragMultiplier:       1,
		Damage:               0.167 * 60,
	}
	w.statusProfilesByName["burning"] = w.statusProfilesByID[1]

	model.Entities = append(model.Entities, RawEntity{
		ID:                  1,
		TypeID:              35,
		Team:                1,
		X:                   16,
		Y:                   16,
		Health:              100,
		MaxHealth:           100,
		SlowMul:             1,
		StatusDamageMul:     1,
		StatusHealthMul:     1,
		StatusSpeedMul:      1,
		StatusReloadMul:     1,
		StatusBuildSpeedMul: 1,
		StatusDragMul:       1,
		StatusArmorOverride: -1,
		RuntimeInit:         true,
	})
	w.applyStatusToEntity(&model.Entities[0], 1, "burning", 1)

	stepForSeconds(w, 1.2)

	if got := model.Entities[0].Health; got >= 95 {
		t.Fatalf("expected burning DOT to reduce health, got=%f", got)
	}
	if len(model.Entities[0].Statuses) != 0 {
		t.Fatalf("expected burning to expire, statuses=%v", model.Entities[0].Statuses)
	}
}

func TestWetAndShockedUseOfficialReactiveDamage(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(8, 8)
	w.SetModel(model)
	w.statusProfilesByID[2] = statusEffectProfile{
		ID:                   2,
		Name:                 "wet",
		DamageMultiplier:     1,
		HealthMultiplier:     1,
		SpeedMultiplier:      0.94,
		ReloadMultiplier:     1,
		BuildSpeedMultiplier: 1,
		DragMultiplier:       1,
		TransitionDamage:     14,
	}
	w.statusProfilesByName["wet"] = w.statusProfilesByID[2]
	w.statusProfilesByID[3] = statusEffectProfile{
		ID:                   3,
		Name:                 "shocked",
		DamageMultiplier:     1,
		HealthMultiplier:     1,
		SpeedMultiplier:      1,
		ReloadMultiplier:     1,
		BuildSpeedMultiplier: 1,
		DragMultiplier:       1,
		Reactive:             true,
	}
	w.statusProfilesByName["shocked"] = w.statusProfilesByID[3]

	entity := RawEntity{
		ID:                  1,
		TypeID:              35,
		Team:                1,
		Health:              100,
		MaxHealth:           100,
		Shield:              0,
		ShieldMax:           0,
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
	}
	w.applyStatusToEntity(&entity, 2, "wet", 2)
	w.applyStatusToEntity(&entity, 3, "shocked", 1)

	if math.Abs(float64(entity.Health-86)) > 0.01 {
		t.Fatalf("expected wet+shocked transition damage=14, got health=%f", entity.Health)
	}
	if len(entity.Statuses) != 1 || entity.Statuses[0].Name != "wet" {
		t.Fatalf("expected reactive shocked to not persist, statuses=%v", entity.Statuses)
	}
}

func TestDisarmedStatusPreventsAttacking(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	w.SetModel(model)
	w.statusProfilesByID[4] = statusEffectProfile{
		ID:                   4,
		Name:                 "disarmed",
		DamageMultiplier:     1,
		HealthMultiplier:     1,
		SpeedMultiplier:      1,
		ReloadMultiplier:     1,
		BuildSpeedMultiplier: 1,
		DragMultiplier:       1,
		Disarm:               true,
	}
	w.statusProfilesByName["disarmed"] = w.statusProfilesByID[4]

	model.Entities = append(model.Entities,
		RawEntity{
			ID:                  1,
			TypeID:              35,
			Team:                1,
			X:                   40,
			Y:                   40,
			Health:              100,
			MaxHealth:           100,
			AttackDamage:        20,
			AttackInterval:      0.05,
			AttackRange:         80,
			AttackFireMode:      "beam",
			AttackTargetAir:     true,
			AttackTargetGround:  true,
			SlowMul:             1,
			StatusDamageMul:     1,
			StatusHealthMul:     1,
			StatusSpeedMul:      1,
			StatusReloadMul:     1,
			StatusBuildSpeedMul: 1,
			StatusDragMul:       1,
			StatusArmorOverride: -1,
			RuntimeInit:         true,
		},
		RawEntity{
			ID:                  2,
			TypeID:              35,
			Team:                2,
			X:                   72,
			Y:                   40,
			Health:              100,
			MaxHealth:           100,
			SlowMul:             1,
			StatusDamageMul:     1,
			StatusHealthMul:     1,
			StatusSpeedMul:      1,
			StatusReloadMul:     1,
			StatusBuildSpeedMul: 1,
			StatusDragMul:       1,
			StatusArmorOverride: -1,
			RuntimeInit:         true,
		},
	)
	w.applyStatusToEntity(&model.Entities[0], 4, "disarmed", 2)

	stepForSeconds(w, 0.5)

	if got := model.Entities[1].Health; got != 100 {
		t.Fatalf("expected disarmed attacker to deal no damage, target health=%f", got)
	}
}

func findTestEntity(t *testing.T, w *World, id int32) RawEntity {
	t.Helper()
	for _, ent := range w.Model().Entities {
		if ent.ID == id {
			return ent
		}
	}
	t.Fatalf("entity %d not found", id)
	return RawEntity{}
}

