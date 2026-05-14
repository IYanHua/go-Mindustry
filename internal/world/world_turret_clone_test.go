package world

import (
	"math"
	"testing"
	"time"

	"github.com/IYanHua/mdt-server/internal/protocol"
)

func TestTriggerWaveSpawnsAtEdgeAndAdvances(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		339: "core-shard",
	}
	model.UnitNames = map[int16]string{
		0: "dagger",
		1: "dagger",
		2: "dagger",
	}
	w.SetModel(model)
	w.unitRuntimeProfilesByName["dagger"] = unitRuntimeProfile{Name: "dagger", Speed: 24}

	core := placeTestBuilding(t, w, 3, 12, 339, 1, 0)
	w.wavesMgr = NewWaveManager(&WaveConfig{
		InitialSpacingSec:  1,
		BaseSpacingSec:     1,
		EnemyBaseCount:     1,
		EnemyGrowthFactor:  0,
		MaxEnemiesPerGroup: 1,
		EnemyTypes:         []int16{1},
	})

	w.triggerWave(w.wavesMgr)
	if len(w.Model().Entities) != 1 {
		t.Fatalf("expected one wave unit to spawn, got %d", len(w.Model().Entities))
	}
	spawned := w.Model().Entities[0]
	if spawned.X < float32(w.Model().Width*8)*0.5 {
		t.Fatalf("expected wave spawn to use the far edge from the player core, got x=%f", spawned.X)
	}

	coreX := float32(core.X*8 + 4)
	coreY := float32(core.Y*8 + 4)
	before := float32(math.Hypot(float64(spawned.X-coreX), float64(spawned.Y-coreY)))
	stepForSeconds(w, 3)
	got := findTestEntity(t, w, spawned.ID)
	after := float32(math.Hypot(float64(got.X-coreX), float64(got.Y-coreY)))
	if after >= before-12 {
		t.Fatalf("expected wave unit to advance after spawning, before=%f after=%f", before, after)
	}
}

func TestRepairPointHealsDamagedFriendlyUnit(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(20, 20)
	model.BlockNames = map[int16]string{
		478: "power-source",
		900: "repair-point",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 8, 8, 900, 1, 0)
	placeTestBuilding(t, w, 8, 10, 478, 1, 0)
	linkPowerNode(t, w, 8, 10, protocol.Point2{X: 0, Y: -2})

	unit := w.Model().AddEntity(RawEntity{
		ID:        5001,
		TypeID:    35,
		Team:      1,
		X:         float32(10*8 + 4),
		Y:         float32(8*8 + 4),
		Health:    40,
		MaxHealth: 100,
	})

	stepForSeconds(w, 4)

	got := findTestEntity(t, w, unit.ID)
	if got.Health <= 40 {
		pos := int32(8*model.Width + 8)
		t.Fatalf("expected repair-point to heal damaged unit, got=%f state=%+v power=%f targets=%v", got.Health, w.repairTurretStates[pos], w.blockSyncPowerStatusLocked(pos, &w.Model().Tiles[pos], "repair-point"), w.repairTurretTilePositions)
	}
}

func TestRepairTurretBlockSyncIncludesHeadRotation(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
	model.BlockNames = map[int16]string{
		901: "repair-turret",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 5, 5, 901, 1, 0)
	pos := int32(5*model.Width + 5)
	w.repairTurretStates[pos] = repairTurretRuntimeState{Rotation: 37.5}

	snaps := w.BlockSyncSnapshotsForPacked([]int32{packTilePos(5, 5)})
	if len(snaps) != 1 {
		t.Fatalf("expected one repair-turret snapshot, got %d", len(snaps))
	}
	_, r := decodeBlockSyncBase(t, snaps[0].Data)
	rot, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read repair turret rotation failed: %v", err)
	}
	if math.Abs(float64(rot-37.5)) > 0.001 {
		t.Fatalf("expected repair turret sync rotation=37.5, got=%f", rot)
	}
}

func TestBlockSyncSnapshotsEncodeItemTurretRuntime(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
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

	tile := placeTestBuilding(t, w, 5, 5, 910, 2, 1)
	pos := int32(5*model.Width + 5)
	tile.Build.Items = []ItemStack{{Item: copperItemID, Amount: 12}}
	w.buildStates[pos] = buildCombatState{Cooldown: 0.25}
	w.rebuildActiveTilesLocked()

	snaps := w.ItemTurretBlockSyncSnapshotsForPacked([]int32{packTilePos(5, 5)})
	if len(snaps) != 1 {
		t.Fatalf("expected one turret block sync snapshot, got %d", len(snaps))
	}

	base, r := decodeBlockSyncBase(t, snaps[0].Data)
	if (base.ModuleBits & 1) == 0 {
		t.Fatalf("expected item turret to keep the vanilla base item module bit, bits=%08b", base.ModuleBits)
	}
	if len(base.Items) != 0 {
		t.Fatalf("expected item turret base item module to stay empty, got %v", base.Items)
	}
	if (base.ModuleBits & (1 << 2)) == 0 {
		t.Fatalf("expected item turret to keep the vanilla base liquid module bit, bits=%08b", base.ModuleBits)
	}
	if len(base.Liquids) != 0 {
		t.Fatalf("expected item turret base liquid module to stay empty, got %v", base.Liquids)
	}
	reloadCounter, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read turret reload counter failed: %v", err)
	}
	rot, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read turret rotation failed: %v", err)
	}
	ammoKinds, err := r.ReadByte()
	if err != nil {
		t.Fatalf("read turret ammo count failed: %v", err)
	}
	if math.Abs(float64(reloadCounter-0.35)) > 0.001 {
		t.Fatalf("expected turret reload counter 0.35, got %f", reloadCounter)
	}
	if math.Abs(float64(rot-90)) > 0.001 {
		t.Fatalf("expected turret rotation 90, got %f", rot)
	}
	if ammoKinds != 1 {
		t.Fatalf("expected one ammo entry, got %d", ammoKinds)
	}
	ammoItem, err := r.ReadInt16()
	if err != nil {
		t.Fatalf("read turret ammo item failed: %v", err)
	}
	ammoAmount, err := r.ReadInt16()
	if err != nil {
		t.Fatalf("read turret ammo amount failed: %v", err)
	}
	if ammoItem != int16(copperItemID) || ammoAmount != 12 {
		t.Fatalf("expected copper ammo x12, got item=%d amount=%d", ammoItem, ammoAmount)
	}
	if rem := r.Remaining(); rem != 0 {
		t.Fatalf("expected item turret sync payload to be fully consumed, remaining=%d", rem)
	}
}

func TestBlockSyncSnapshotsEncodePowerTurretRuntime(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.BlockNames = map[int16]string{
		478: "power-source",
		911: "arc",
	}
	w.SetModel(model)
	w.buildingProfilesByName["arc"] = buildingWeaponProfile{
		ClassName:     "PowerTurret",
		Range:         88,
		Damage:        24,
		Interval:      0.42,
		PowerCapacity: 140,
		PowerPerShot:  30,
		TargetAir:     true,
		TargetGround:  true,
		HitBuildings:  true,
		ChainCount:    2,
		ChainRange:    32,
	}

	placeTestBuilding(t, w, 8, 9, 478, 2, 0)
	placeTestBuilding(t, w, 8, 8, 911, 2, 0)
	linkPowerNode(t, w, 8, 9, protocol.Point2{X: 0, Y: -1})
	pos := int32(8*model.Width + 8)
	w.buildStates[pos] = buildCombatState{Cooldown: 0.12, Power: 75}
	w.rebuildActiveTilesLocked()

	snaps := w.BlockSyncSnapshotsForPacked([]int32{packTilePos(8, 8)})
	if len(snaps) != 1 {
		t.Fatalf("expected one power turret block sync snapshot, got %d", len(snaps))
	}

	base, r := decodeBlockSyncBase(t, snaps[0].Data)
	if (base.ModuleBits & (1 << 1)) == 0 {
		t.Fatalf("expected power turret to include power module, bits=%08b", base.ModuleBits)
	}
	reloadCounter, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read power turret reload counter failed: %v", err)
	}
	rot, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read power turret rotation failed: %v", err)
	}
	if math.Abs(float64(reloadCounter-0.30)) > 0.001 {
		t.Fatalf("expected power turret reload counter 0.30, got %f", reloadCounter)
	}
	if math.Abs(float64(rot-0)) > 0.001 {
		t.Fatalf("expected power turret rotation 0, got %f", rot)
	}
	if rem := r.Remaining(); rem != 0 {
		t.Fatalf("expected power turret sync payload to be fully consumed, remaining=%d", rem)
	}
}

func TestPeriodicBlockSyncSnapshotsSkipAllTurrets(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.BlockNames = map[int16]string{
		910: "duo",
		911: "arc",
		912: "container",
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
	w.buildingProfilesByName["arc"] = buildingWeaponProfile{
		ClassName:     "PowerTurret",
		Range:         88,
		Damage:        24,
		Interval:      0.42,
		PowerCapacity: 140,
		PowerPerShot:  30,
		TargetAir:     true,
		TargetGround:  true,
		HitBuildings:  true,
	}

	duo := placeTestBuilding(t, w, 5, 5, 910, 2, 1)
	duo.Build.Items = []ItemStack{{Item: copperItemID, Amount: 12}}
	arc := placeTestBuilding(t, w, 8, 8, 911, 2, 0)
	_ = arc
	container := placeTestBuilding(t, w, 10, 10, 912, 2, 0)
	container.Build.Items = []ItemStack{{Item: copperItemID, Amount: 5}}
	w.rebuildActiveTilesLocked()

	snaps := w.PeriodicBlockSyncSnapshotsLiveOnly()
	if len(snaps) != 1 {
		t.Fatalf("expected only the non-turret building in periodic snapshots, got %d", len(snaps))
	}
	if snaps[0].Pos != packTilePos(10, 10) {
		t.Fatalf("expected periodic snapshot to keep container only, got pos=%d", snaps[0].Pos)
	}

	targeted := w.BlockSyncSnapshotsForPackedLiveOnly([]int32{packTilePos(5, 5)})
	if len(targeted) != 0 {
		t.Fatalf("expected generic targeted snapshots to skip item turrets, got %d", len(targeted))
	}

	targeted = w.BlockSyncSnapshotsForPackedLiveOnly([]int32{packTilePos(8, 8)})
	if len(targeted) != 1 {
		t.Fatalf("expected generic targeted snapshots to keep power turrets, got %d", len(targeted))
	}
	if targeted[0].Pos != packTilePos(8, 8) {
		t.Fatalf("expected generic targeted snapshot to keep arc, got pos=%d", targeted[0].Pos)
	}

	targeted = w.ItemTurretBlockSyncSnapshotsForPackedLiveOnly([]int32{packTilePos(5, 5)})
	if len(targeted) != 1 {
		t.Fatalf("expected targeted duo snapshot to remain available, got %d", len(targeted))
	}
	if targeted[0].Pos != packTilePos(5, 5) {
		t.Fatalf("expected targeted snapshot to keep duo, got pos=%d", targeted[0].Pos)
	}
}

func TestPeriodicBlockSyncSnapshotsSkipConveyorsButTargetedKeepsThem(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.BlockNames = map[int16]string{
		257: "conveyor",
		912: "container",
	}
	w.SetModel(model)
	conveyor := placeTestBuilding(t, w, 5, 5, 257, 2, 1)
	conveyor.Build.Items = []ItemStack{{Item: copperItemID, Amount: 1}}
	conveyorPos := int32(conveyor.Y*w.Model().Width + conveyor.X)
	w.conveyorStates[conveyorPos] = &conveyorRuntimeState{
		IDs: [3]ItemID{copperItemID},
		XS:  [3]float32{1},
		YS:  [3]float32{0.25},
		Len: 1,
	}
	container := placeTestBuilding(t, w, 10, 10, 912, 2, 0)
	container.Build.Items = []ItemStack{{Item: copperItemID, Amount: 5}}
	w.rebuildActiveTilesLocked()

	snaps := w.PeriodicBlockSyncSnapshotsLiveOnly()
	if len(snaps) != 1 {
		t.Fatalf("expected periodic snapshots to skip conveyors and keep container only, got %d", len(snaps))
	}
	if snaps[0].Pos != packTilePos(10, 10) {
		t.Fatalf("expected periodic snapshot to keep container only, got pos=%d", snaps[0].Pos)
	}

	targeted := w.BlockSyncSnapshotsForPackedLiveOnly([]int32{protocol.PackPoint2(5, 5)})
	if len(targeted) != 1 {
		t.Fatalf("expected targeted live-only snapshots to keep conveyor runtime, got %d", len(targeted))
	}
	if targeted[0].Pos != protocol.PackPoint2(5, 5) {
		t.Fatalf("expected targeted live-only conveyor snapshot at %d, got %d", protocol.PackPoint2(5, 5), targeted[0].Pos)
	}
}

func TestBlockSyncSnapshotsEncodeTurretHeadRotation(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(12, 12)
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

	tile := placeTestBuilding(t, w, 5, 5, 910, 2, 1)
	pos := int32(5*model.Width + 5)
	tile.Build.Items = []ItemStack{{Item: copperItemID, Amount: 12}}
	w.buildStates[pos] = buildCombatState{
		Cooldown:       0.25,
		TurretRotation: 37.5,
		HasRotation:    true,
	}
	w.rebuildActiveTilesLocked()

	snaps := w.ItemTurretBlockSyncSnapshotsForPacked([]int32{packTilePos(5, 5)})
	if len(snaps) != 1 {
		t.Fatalf("expected one turret block sync snapshot, got %d", len(snaps))
	}

	_, r := decodeBlockSyncBase(t, snaps[0].Data)
	if _, err := r.ReadFloat32(); err != nil {
		t.Fatalf("read turret reload counter failed: %v", err)
	}
	rot, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read turret head rotation failed: %v", err)
	}
	if math.Abs(float64(rot-37.5)) > 0.001 {
		t.Fatalf("expected turret head rotation 37.5, got %f", rot)
	}
}

func TestMergeBuildingProfileIncludesTurretAimFields(t *testing.T) {
	p := buildingWeaponProfile{}
	mergeBuildingProfile(&p, vanillaTurretProfile{
		Rotate:               true,
		RotateSpeed:          7,
		BaseRotation:         15,
		PredictTarget:        true,
		TargetInterval:       0.4,
		TargetSwitchInterval: 0.7,
		ShootCone:            12,
		RotationLimit:        180,
	})
	if !p.Rotate || p.RotateSpeed != 7 || p.BaseRotation != 15 || !p.PredictTarget {
		t.Fatalf("expected turret aim fields to merge, got %+v", p)
	}
	if p.TargetInterval != 0.4 || p.TargetSwitchInterval != 0.7 || p.ShootCone != 12 || p.RotationLimit != 180 {
		t.Fatalf("expected turret targeting timings to merge, got %+v", p)
	}
}

func TestBuildingTurretRotationTurnsGraduallyBeforeFiring(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		910: "duo",
	}
	model.UnitNames = map[int16]string{
		35: "alpha",
	}
	w.SetModel(model)
	w.buildingProfilesByName["duo"] = buildingWeaponProfile{
		ClassName:      "ItemTurret",
		FireMode:       "projectile",
		Range:          136,
		Damage:         9,
		Interval:       0.1,
		BulletType:     94,
		BulletSpeed:    60,
		HitBuildings:   true,
		TargetAir:      true,
		TargetGround:   true,
		Rotate:         true,
		RotateSpeed:    5,
		PredictTarget:  false,
		ShootCone:      5,
		AmmoCapacity:   80,
		AmmoPerShot:    1,
		TargetInterval: 0.2,
	}

	tile := placeTestBuilding(t, w, 10, 10, 910, 1, 0)
	tile.Build.Items = []ItemStack{{Item: copperItemID, Amount: 12}}
	w.rebuildActiveTilesLocked()
	pos := int32(10*model.Width + 10)

	w.Model().AddEntity(RawEntity{
		ID:                  1,
		TypeID:              35,
		Team:                2,
		X:                   float32(10*8 + 4),
		Y:                   float32(15*8 + 4),
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

	stepWorldFrames(w, 1)

	state := w.buildStates[pos]
	if math.Abs(float64(state.TurretRotation-5)) > 0.001 {
		t.Fatalf("expected turret to rotate by 5 degrees on first frame, got %f", state.TurretRotation)
	}
	if got := len(w.bullets); got != 0 {
		t.Fatalf("expected turret not to fire before lining up with target, bullets=%d", got)
	}

	stepWorldFrames(w, 17)

	state = w.buildStates[pos]
	if state.TurretRotation < 89.9 {
		t.Fatalf("expected turret to finish turning toward 90 degrees, got %f", state.TurretRotation)
	}
	if got := len(w.bullets); got == 0 {
		t.Fatalf("expected turret to fire after finishing rotation")
	}
}

func TestCloneModelForWorldStreamBuildsConveyorPayload(t *testing.T) {
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

	clone := w.CloneModelForWorldStream()
	if clone == nil {
		t.Fatal("expected world stream model clone")
	}
	tileClone, err := clone.TileAt(6, 6)
	if err != nil || tileClone == nil || tileClone.Build == nil {
		t.Fatalf("clone tile lookup failed: %v", err)
	}
	if tileClone.Build.MapSyncRevision != 1 {
		t.Fatalf("expected conveyor world-stream revision 1, got %d", tileClone.Build.MapSyncRevision)
	}
	if len(tileClone.Build.MapSyncData) == 0 {
		t.Fatal("expected conveyor world-stream payload bytes")
	}
}

func TestCloneModelForWorldStreamBuildsItemTurretPayload(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
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
	tile := placeTestBuilding(t, w, 5, 5, 910, 2, 1)
	pos := int32(5*model.Width + 5)
	tile.Build.Items = []ItemStack{{Item: copperItemID, Amount: 12}}
	w.buildStates[pos] = buildCombatState{
		Cooldown:       0.25,
		TurretRotation: 37.5,
		HasRotation:    true,
	}
	w.rebuildActiveTilesLocked()

	clone := w.CloneModelForWorldStream()
	if clone == nil {
		t.Fatal("expected world stream model clone")
	}
	tileClone, err := clone.TileAt(5, 5)
	if err != nil || tileClone == nil || tileClone.Build == nil {
		t.Fatalf("clone tile lookup failed: %v", err)
	}
	if tileClone.Build.MapSyncRevision != 2 {
		t.Fatalf("expected item turret world-stream revision 2, got %d", tileClone.Build.MapSyncRevision)
	}
	if len(tileClone.Build.MapSyncData) == 0 {
		t.Fatal("expected item turret world-stream payload bytes")
	}
}

func TestCloneModelForWorldStreamBuildsPayloadRouterPayload(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		701: "payload-router",
		257: "conveyor",
	}
	w.SetModel(model)
	tile := placeTestBuilding(t, w, 8, 8, 701, 1, 0)
	pos := int32(8*model.Width + 8)
	payload := &payloadData{Kind: payloadKindBlock, BlockID: 257}
	w.payloadStateLocked(pos).Payload = payload
	w.payloadStateLocked(pos).RecDir = 2
	w.syncPayloadTileLocked(tile, payload)
	w.ConfigureBuildingPacked(protocol.PackPoint2(8, 8), protocol.BlockRef{BlkID: 257})
	w.rebuildActiveTilesLocked()

	clone := w.CloneModelForWorldStream()
	if clone == nil {
		t.Fatal("expected world stream model clone")
	}
	tileClone, err := clone.TileAt(8, 8)
	if err != nil || tileClone == nil || tileClone.Build == nil {
		t.Fatalf("clone tile lookup failed: %v", err)
	}
	if tileClone.Build.MapSyncRevision != 1 {
		t.Fatalf("expected payload router world-stream revision 1, got %d", tileClone.Build.MapSyncRevision)
	}
	if len(tileClone.Build.MapSyncData) == 0 {
		t.Fatal("expected payload router world-stream payload bytes")
	}
}

func TestCloneModelForWorldStreamBuildsPayloadLoaderPayload(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		703: "payload-loader",
		257: "conveyor",
	}
	w.SetModel(model)
	tile := placeTestBuilding(t, w, 8, 8, 703, 1, 0)
	pos := int32(8*model.Width + 8)
	payload := &payloadData{Kind: payloadKindBlock, BlockID: 257}
	w.payloadStateLocked(pos).Payload = payload
	w.payloadStateLocked(pos).Exporting = true
	w.syncPayloadTileLocked(tile, payload)
	w.rebuildActiveTilesLocked()

	clone := w.CloneModelForWorldStream()
	if clone == nil {
		t.Fatal("expected world stream model clone")
	}
	tileClone, err := clone.TileAt(8, 8)
	if err != nil || tileClone == nil || tileClone.Build == nil {
		t.Fatalf("clone tile lookup failed: %v", err)
	}
	if tileClone.Build.MapSyncRevision != 1 {
		t.Fatalf("expected payload loader world-stream revision 1, got %d", tileClone.Build.MapSyncRevision)
	}
	if len(tileClone.Build.MapSyncData) == 0 {
		t.Fatal("expected payload loader world-stream payload bytes")
	}
}

func TestCloneModelForWorldStreamBuildsPowerNodePayload(t *testing.T) {
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
	w.applyBuildingConfigLocked(nodePos, []protocol.Point2{{X: 6, Y: 0}}, true)
	w.rebuildActiveTilesLocked()

	clone := w.CloneModelForWorldStream()
	if clone == nil {
		t.Fatal("expected world stream model clone")
	}
	tileClone, err := clone.TileAt(8, 10)
	if err != nil || tileClone == nil || tileClone.Build == nil {
		t.Fatalf("clone tile lookup failed: %v", err)
	}
	if tileClone.Build.MapSyncRevision != 0 {
		t.Fatalf("expected power node world-stream revision 0, got %d", tileClone.Build.MapSyncRevision)
	}
	if len(tileClone.Build.MapSyncData) == 0 {
		t.Fatal("expected power node world-stream payload bytes")
	}
}

func TestCloneModelForWorldStreamBuildsDuctPayload(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		440: "duct",
	}
	w.SetModel(model)
	tile := placeTestBuilding(t, w, 6, 6, 440, 1, 0)
	pos := int32(6*model.Width + 6)
	tile.Build.Items = []ItemStack{{Item: copperItemID, Amount: 1}}
	st := w.ductStateLocked(pos, tile)
	st.Current = copperItemID
	st.HasItem = true
	st.RecDir = 2
	w.rebuildActiveTilesLocked()

	clone := w.CloneModelForWorldStream()
	if clone == nil {
		t.Fatal("expected world stream model clone")
	}
	tileClone, err := clone.TileAt(6, 6)
	if err != nil || tileClone == nil || tileClone.Build == nil {
		t.Fatalf("clone tile lookup failed: %v", err)
	}
	if tileClone.Build.MapSyncRevision != 1 {
		t.Fatalf("expected duct world-stream revision 1, got %d", tileClone.Build.MapSyncRevision)
	}
	if len(tileClone.Build.MapSyncData) == 0 {
		t.Fatal("expected duct world-stream payload bytes")
	}
}

func TestCloneModelForWorldStreamBuildsDuctRouterPayload(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		446: "duct-router",
	}
	w.SetModel(model)
	tile := placeTestBuilding(t, w, 6, 6, 446, 1, 0)
	pos := int32(6*model.Width + 6)
	tile.Build.Items = []ItemStack{{Item: copperItemID, Amount: 1}}
	w.sorterCfg[pos] = copperItemID
	w.rebuildActiveTilesLocked()

	clone := w.CloneModelForWorldStream()
	if clone == nil {
		t.Fatal("expected world stream model clone")
	}
	tileClone, err := clone.TileAt(6, 6)
	if err != nil || tileClone == nil || tileClone.Build == nil {
		t.Fatalf("clone tile lookup failed: %v", err)
	}
	if tileClone.Build.MapSyncRevision != 1 {
		t.Fatalf("expected duct-router world-stream revision 1, got %d", tileClone.Build.MapSyncRevision)
	}
	if len(tileClone.Build.MapSyncData) == 0 {
		t.Fatal("expected duct-router world-stream payload bytes")
	}
}

func TestCloneModelForWorldStreamBuildsSorterPayload(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		262: "sorter",
	}
	w.SetModel(model)
	placeTestBuilding(t, w, 6, 6, 262, 1, 0)
	pos := int32(6*model.Width + 6)
	w.sorterCfg[pos] = copperItemID
	w.rebuildActiveTilesLocked()

	clone := w.CloneModelForWorldStream()
	if clone == nil {
		t.Fatal("expected world stream model clone")
	}
	tileClone, err := clone.TileAt(6, 6)
	if err != nil || tileClone == nil || tileClone.Build == nil {
		t.Fatalf("clone tile lookup failed: %v", err)
	}
	if tileClone.Build.MapSyncRevision != 2 {
		t.Fatalf("expected sorter world-stream revision 2, got %d", tileClone.Build.MapSyncRevision)
	}
	if len(tileClone.Build.MapSyncData) == 0 {
		t.Fatal("expected sorter world-stream payload bytes")
	}
}

func TestCloneModelForWorldStreamBuildsOverflowGatePayload(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		265: "overflow-gate",
	}
	w.SetModel(model)
	placeTestBuilding(t, w, 6, 6, 265, 1, 0)
	w.rebuildActiveTilesLocked()

	clone := w.CloneModelForWorldStream()
	if clone == nil {
		t.Fatal("expected world stream model clone")
	}
	tileClone, err := clone.TileAt(6, 6)
	if err != nil || tileClone == nil || tileClone.Build == nil {
		t.Fatalf("clone tile lookup failed: %v", err)
	}
	if tileClone.Build.MapSyncRevision != 4 {
		t.Fatalf("expected overflow-gate world-stream revision 4, got %d", tileClone.Build.MapSyncRevision)
	}
	if len(tileClone.Build.MapSyncData) == 0 {
		t.Fatal("expected overflow-gate world-stream payload bytes")
	}
}

func TestCloneModelForWorldStreamBuildsPhaseConveyorPayload(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		262: "phase-conveyor",
	}
	w.SetModel(model)
	placeTestBuilding(t, w, 6, 6, 262, 1, 0)
	placeTestBuilding(t, w, 10, 6, 262, 1, 0)
	pos := int32(6*model.Width + 6)
	target := int32(6*model.Width + 10)
	w.bridgeLinks[pos] = target
	w.rebuildActiveTilesLocked()

	clone := w.CloneModelForWorldStream()
	if clone == nil {
		t.Fatal("expected world stream model clone")
	}
	tileClone, err := clone.TileAt(6, 6)
	if err != nil || tileClone == nil || tileClone.Build == nil {
		t.Fatalf("clone tile lookup failed: %v", err)
	}
	if tileClone.Build.MapSyncRevision != 1 {
		t.Fatalf("expected phase conveyor world-stream revision 1, got %d", tileClone.Build.MapSyncRevision)
	}
	if len(tileClone.Build.MapSyncData) == 0 {
		t.Fatal("expected phase conveyor world-stream payload bytes")
	}
	_, r := decodeBlockSyncBase(t, tileClone.Build.MapSyncData)
	link, err := r.ReadInt32()
	if err != nil {
		t.Fatalf("read phase conveyor link failed: %v", err)
	}
	if want := packTilePos(10, 6); link != want {
		t.Fatalf("expected phase conveyor link %d, got %d", want, link)
	}
	warmup, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read phase conveyor warmup failed: %v", err)
	}
	if math.Abs(float64(warmup-1)) > 0.0001 {
		t.Fatalf("expected linked phase conveyor warmup 1, got %f", warmup)
	}
	incoming, err := r.ReadByte()
	if err != nil {
		t.Fatalf("read phase conveyor incoming count failed: %v", err)
	}
	if incoming != 0 {
		t.Fatalf("expected no incoming phase conveyors, got %d", incoming)
	}
	moved, err := r.ReadBool()
	if err != nil {
		t.Fatalf("read phase conveyor moved flag failed: %v", err)
	}
	if moved {
		t.Fatal("expected idle phase conveyor payload to report unmoved state")
	}
	if remaining := r.Remaining(); remaining != 0 {
		t.Fatalf("expected phase conveyor payload to be fully consumed, got %d trailing bytes", remaining)
	}
}

func TestCloneModelForWorldStreamBuildsItemSourcePayload(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.BlockNames = map[int16]string{
		412: "item-source",
	}
	w.SetModel(model)
	placeTestBuilding(t, w, 4, 4, 412, 1, 0)
	pos := int32(4*model.Width + 4)
	w.ConfigureItemSource(pos, thoriumItemID)

	clone := w.CloneModelForWorldStream()
	if clone == nil {
		t.Fatal("expected world stream model clone")
	}
	tileClone, err := clone.TileAt(4, 4)
	if err != nil || tileClone == nil || tileClone.Build == nil {
		t.Fatalf("clone tile lookup failed: %v", err)
	}
	if tileClone.Build.MapSyncRevision != 0 {
		t.Fatalf("expected item source world-stream revision 0, got %d", tileClone.Build.MapSyncRevision)
	}
	_, r := decodeBlockSyncBase(t, tileClone.Build.MapSyncData)
	itemID, err := r.ReadInt16()
	if err != nil {
		t.Fatalf("read item source item id failed: %v", err)
	}
	if itemID != int16(thoriumItemID) {
		t.Fatalf("expected item source item %d, got %d", thoriumItemID, itemID)
	}
}

func TestCloneModelForWorldStreamBuildsDrillPayload(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.BlockNames = map[int16]string{
		429: "mechanical-drill",
	}
	w.SetModel(model)
	placeTestBuilding(t, w, 5, 5, 429, 1, 0)
	pos := int32(5*model.Width + 5)
	w.drillStates[pos] = drillRuntimeState{
		Progress: 33.5,
		Warmup:   0.75,
	}

	clone := w.CloneModelForWorldStream()
	if clone == nil {
		t.Fatal("expected world stream model clone")
	}
	tileClone, err := clone.TileAt(5, 5)
	if err != nil || tileClone == nil || tileClone.Build == nil {
		t.Fatalf("clone tile lookup failed: %v", err)
	}
	if tileClone.Build.MapSyncRevision != 1 {
		t.Fatalf("expected drill world-stream revision 1, got %d", tileClone.Build.MapSyncRevision)
	}
	_, r := decodeBlockSyncBase(t, tileClone.Build.MapSyncData)
	progress, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read drill progress failed: %v", err)
	}
	warmup, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read drill warmup failed: %v", err)
	}
	if math.Abs(float64(progress-33.5)) > 0.0001 {
		t.Fatalf("expected drill progress 33.5, got %f", progress)
	}
	if math.Abs(float64(warmup-0.75)) > 0.0001 {
		t.Fatalf("expected drill warmup 0.75, got %f", warmup)
	}
}

func TestCloneModelForWorldStreamBuildsGeneratorPayload(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.BlockNames = map[int16]string{
		308: "combustion-generator",
	}
	w.SetModel(model)
	placeTestBuilding(t, w, 6, 6, 308, 1, 0)
	pos := int32(6*model.Width + 6)
	w.powerGeneratorState[pos] = &powerGeneratorState{
		FuelFrames: 90,
	}

	clone := w.CloneModelForWorldStream()
	if clone == nil {
		t.Fatal("expected world stream model clone")
	}
	tileClone, err := clone.TileAt(6, 6)
	if err != nil || tileClone == nil || tileClone.Build == nil {
		t.Fatalf("clone tile lookup failed: %v", err)
	}
	if tileClone.Build.MapSyncRevision != 1 {
		t.Fatalf("expected generator world-stream revision 1, got %d", tileClone.Build.MapSyncRevision)
	}
	_, r := decodeBlockSyncBase(t, tileClone.Build.MapSyncData)
	productionEfficiency, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read generator productionEfficiency failed: %v", err)
	}
	generateTime, err := r.ReadFloat32()
	if err != nil {
		t.Fatalf("read generator generateTime failed: %v", err)
	}
	if math.Abs(float64(productionEfficiency-1)) > 0.0001 {
		t.Fatalf("expected generator productionEfficiency 1, got %f", productionEfficiency)
	}
	if math.Abs(float64(generateTime-90)) > 0.0001 {
		t.Fatalf("expected generator generateTime 90, got %f", generateTime)
	}
}

func TestCloneModelForWorldStreamMarksMultiblockEdges(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		703: "payload-loader",
	}
	w.SetModel(model)
	placeTestBuilding(t, w, 8, 8, 703, 1, 0)
	w.rebuildActiveTilesLocked()

	clone := w.CloneModelForWorldStream()
	if clone == nil {
		t.Fatal("expected world stream model clone")
	}
	centerTile, err := clone.TileAt(8, 8)
	if err != nil || centerTile == nil || centerTile.Build == nil {
		t.Fatalf("center tile lookup failed: %v", err)
	}
	edgeTile, err := clone.TileAt(7, 8)
	if err != nil || edgeTile == nil {
		t.Fatalf("edge tile lookup failed: %v", err)
	}
	if edgeTile.Block != 703 {
		t.Fatalf("expected multiblock edge to mirror center block 703, got %d", edgeTile.Block)
	}
	if edgeTile.Build == nil {
		t.Fatal("expected multiblock edge to share center build for world stream encoding")
	}
	if edgeTile.Build != centerTile.Build {
		t.Fatal("expected multiblock edge to reference center build")
	}
}

func TestCloneModelForWorldStreamBuildsPayloadMassDriverPayload(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		702: "payload-mass-driver",
		257: "conveyor",
	}
	w.SetModel(model)
	tile := placeTestBuilding(t, w, 8, 8, 702, 1, 0)
	pos := int32(8*model.Width + 8)
	target := placeTestBuilding(t, w, 14, 8, 702, 1, 0)
	targetPos := int32(target.Y*model.Width + target.X)
	payload := &payloadData{Kind: payloadKindBlock, BlockID: 257}
	w.payloadStateLocked(pos).Payload = payload
	w.payloadDriverLinks[pos] = targetPos
	w.payloadDriverStateLocked(pos).Charge = 12
	w.syncPayloadTileLocked(tile, payload)
	w.rebuildActiveTilesLocked()

	clone := w.CloneModelForWorldStream()
	if clone == nil {
		t.Fatal("expected world stream model clone")
	}
	tileClone, err := clone.TileAt(8, 8)
	if err != nil || tileClone == nil || tileClone.Build == nil {
		t.Fatalf("clone tile lookup failed: %v", err)
	}
	if tileClone.Build.MapSyncRevision != 1 {
		t.Fatalf("expected payload mass driver world-stream revision 1, got %d", tileClone.Build.MapSyncRevision)
	}
	if len(tileClone.Build.MapSyncData) == 0 {
		t.Fatal("expected payload mass driver world-stream payload bytes")
	}
}

func TestCloneModelForWorldStreamBuildsPayloadDeconstructorPayload(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		705: "payload-deconstructor",
		257: "conveyor",
	}
	w.SetModel(model)
	placeTestBuilding(t, w, 8, 8, 705, 1, 0)
	pos := int32(8*model.Width + 8)
	state := w.payloadDeconstructorStateLocked(pos)
	state.Progress = 0.5
	state.Accum = []float32{1.25, 0.5}
	state.Deconstructing = &payloadData{Kind: payloadKindBlock, BlockID: 257}
	w.rebuildActiveTilesLocked()

	clone := w.CloneModelForWorldStream()
	if clone == nil {
		t.Fatal("expected world stream model clone")
	}
	tileClone, err := clone.TileAt(8, 8)
	if err != nil || tileClone == nil || tileClone.Build == nil {
		t.Fatalf("clone tile lookup failed: %v", err)
	}
	if tileClone.Build.MapSyncRevision != 0 {
		t.Fatalf("expected payload deconstructor world-stream revision 0, got %d", tileClone.Build.MapSyncRevision)
	}
	if len(tileClone.Build.MapSyncData) == 0 {
		t.Fatal("expected payload deconstructor world-stream payload bytes")
	}
}

func TestSetModelSkipsMalformedBuildingPayload(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.BlockNames = map[int16]string{
		701: "payload-router",
	}
	tile, err := model.TileAt(8, 8)
	if err != nil || tile == nil {
		t.Fatalf("tile lookup failed: %v", err)
	}
	tile.Block = 701
	tile.Team = 1
	tile.Rotation = 1
	tile.Build = &Building{
		Block:    701,
		Team:     1,
		Rotation: 1,
		X:        8,
		Y:        8,
		Payload:  []byte{1},
	}

	w.SetModel(model)

	pos := int32(tile.Y*model.Width + tile.X)
	if st := w.payloadStates[pos]; st != nil && st.Payload != nil {
		t.Fatalf("expected malformed payload to be skipped, got %+v", st.Payload)
	}
}

func TestDerelictTurretDoesNotAttack(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		910: "duo",
	}
	model.UnitNames = map[int16]string{
		35: "alpha",
	}
	w.SetModel(model)
	w.buildingProfilesByName["duo"] = buildingWeaponProfile{
		ClassName:    "ItemTurret",
		Range:        136,
		Damage:       12,
		Interval:     0.1,
		BulletSpeed:  60,
		TargetAir:    true,
		TargetGround: true,
		HitBuildings: true,
	}

	placeTestBuilding(t, w, 10, 10, 910, 0, 0)
	w.rebuildActiveTilesLocked()
	w.Model().AddEntity(RawEntity{
		ID:                  1,
		TypeID:              35,
		Team:                1,
		X:                   float32(13*8 + 4),
		Y:                   float32(10*8 + 4),
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

	stepForSeconds(w, 1)

	if got := findTestEntity(t, w, 1).Health; math.Abs(float64(got-100)) > 0.001 {
		t.Fatalf("expected derelict turret to stay idle, target health=%f", got)
	}
}

func TestTargetBuildsTurretPrioritizesBuilding(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(32, 32)
	model.BlockNames = map[int16]string{
		950: "scathe",
		600: "copper-wall",
	}
	model.UnitNames = map[int16]string{
		35: "alpha",
	}
	w.SetModel(model)
	w.buildingProfilesByName["scathe"] = buildingWeaponProfile{
		ClassName:    "PowerTurret",
		Range:        240,
		Damage:       25,
		Interval:     1.0,
		BulletSpeed:  80,
		TargetAir:    true,
		TargetGround: true,
		HitBuildings: true,
		TargetBuilds: true,
	}

	placeTestBuilding(t, w, 10, 10, 950, 2, 0)
	target := placeTestBuilding(t, w, 14, 10, 600, 1, 0)
	target.Build.Health = 100
	target.Build.MaxHealth = 100
	w.rebuildActiveTilesLocked()
	w.Model().AddEntity(RawEntity{
		ID:                  1,
		TypeID:              35,
		Team:                1,
		X:                   float32(10*8 + 4),
		Y:                   float32(14*8 + 4),
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

	stepForSeconds(w, 0.5)

	if target.Build != nil && target.Build.Health >= 100 {
		t.Fatalf("expected target-builds turret to damage building first, health=%f", target.Build.Health)
	}
	if got := findTestEntity(t, w, 1).Health; math.Abs(float64(got-100)) > 0.001 {
		t.Fatalf("expected off-axis unit to be ignored while turret targets building, health=%f", got)
	}
}

func TestOverdriveProjectorBoostsGraphitePress(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(20, 20)
	model.BlockNames = map[int16]string{
		900: "overdrive-projector",
		901: "graphite-press",
	}
	w.SetModel(model)

	projector := placeTestBuilding(t, w, 8, 8, 900, 1, 0)
	press := placeTestBuilding(t, w, 10, 8, 901, 1, 0)
	press.Build.AddItem(coalItemID, 2)
	w.rebuildActiveTilesLocked()

	projectorPos := int32(projector.Y*w.Model().Width + projector.X)
	pressPos := int32(press.Y*w.Model().Width + press.X)
	w.overdriveProjectorStateLocked(projectorPos).Charge = overdriveProjectorProfiles["overdrive-projector"].Reload

	w.Step(time.Second / 60)

	if got := w.buildingTimeScaleLocked(pressPos); got <= 1 {
		t.Fatalf("expected overdrive projector to apply a boost, timeScale=%f", got)
	}

	stepForTicks(w, 59)

	if got := press.Build.ItemAmount(graphiteItemID); got != 1 {
		t.Fatalf("expected boosted graphite press to finish one craft in 60 ticks, amount=%d", got)
	}
}

func TestSingleTickBuildingBoostLastsThroughCurrentStep(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(20, 20)
	model.BlockNames = map[int16]string{
		901: "graphite-press",
	}
	w.SetModel(model)

	press := placeTestBuilding(t, w, 10, 8, 901, 1, 0)
	press.Build.AddItem(coalItemID, 2)
	w.rebuildActiveTilesLocked()

	pressPos := int32(press.Y*w.Model().Width + press.X)
	w.applyBuildingBoostLocked(pressPos, 1.5, 0.5)

	w.Step(time.Second / 60)

	state, ok := w.crafterStates[pressPos]
	if !ok {
		t.Fatal("expected graphite press runtime state to exist after stepping")
	}
	expected := float32(1.5 / 90.0)
	if math.Abs(float64(state.Progress-expected)) > 0.0001 {
		t.Fatalf("expected one boosted tick of crafter progress, progress=%f want=%f", state.Progress, expected)
	}
	if got := w.buildingTimeScaleLocked(pressPos); got != 1 {
		t.Fatalf("expected one-tick boost to expire after work completes, timeScale=%f", got)
	}
}

func TestOverdriveDomeMatchesVanillaBuild157Profile(t *testing.T) {
	prof, ok := overdriveProjectorProfiles["overdrive-dome"]
	if !ok {
		t.Fatal("expected overdrive-dome profile to exist")
	}
	if prof.Range != 200 || prof.Reload != 60 {
		t.Fatalf("unexpected overdrive-dome range/reload: %+v", prof)
	}
	if prof.SpeedBoost != 2.5 {
		t.Fatalf("expected overdrive-dome speed boost 2.5, got %f", prof.SpeedBoost)
	}
	if prof.UseTime != 300 {
		t.Fatalf("expected overdrive-dome use time 300, got %f", prof.UseTime)
	}
	if prof.HasBoost {
		t.Fatal("expected overdrive-dome to disable phase item boosting in 157.4")
	}
}

func TestOverdriveProjectorSpeedsArcCooldown(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		900: "overdrive-projector",
		911: "arc",
	}
	model.UnitNames = map[int16]string{
		35: "alpha",
	}
	w.SetModel(model)
	w.buildingProfilesByName["arc"] = buildingWeaponProfile{
		ClassName:     "PowerTurret",
		Range:         88,
		Damage:        24,
		Interval:      0.42,
		PowerCapacity: 140,
		PowerPerShot:  30,
		TargetAir:     true,
		TargetGround:  true,
		HitBuildings:  true,
	}

	projector := placeTestBuilding(t, w, 8, 8, 900, 1, 0)
	arc := placeTestBuilding(t, w, 10, 8, 911, 1, 0)
	arcPos := int32(arc.Y*w.Model().Width + arc.X)
	w.buildStates[arcPos] = buildCombatState{
		Power:          60,
		TurretRotation: 0,
		HasRotation:    true,
	}
	w.Model().AddEntity(RawEntity{
		ID:                  1,
		TypeID:              35,
		Team:                2,
		X:                   float32(11*8 + 4),
		Y:                   float32(8*8 + 4),
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
	w.rebuildActiveTilesLocked()

	projectorPos := int32(projector.Y*w.Model().Width + projector.X)
	w.overdriveProjectorStateLocked(projectorPos).Charge = overdriveProjectorProfiles["overdrive-projector"].Reload

	w.Step(time.Second / 60)

	if got := w.buildingTimeScaleLocked(arcPos); got <= 1 {
		t.Fatalf("expected arc turret to receive overdrive boost, timeScale=%f", got)
	}

	stepForSeconds(w, 0.35)

	if got := findTestEntity(t, w, 1).Health; got > 52.001 {
		t.Fatalf("expected boosted arc turret to fire twice within 0.35s, target health=%f", got)
	}
}

func TestOverdriveProjectorSkipsVanillaNonBoostableBlocks(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		900: "overdrive-projector",
		901: "graphite-press",
		902: "battery",
		903: "core-shard",
		904: "landing-pad",
		905: "tile-logic-display",
	}
	w.SetModel(model)

	projector := placeTestBuilding(t, w, 8, 8, 900, 1, 0)
	press := placeTestBuilding(t, w, 10, 8, 901, 1, 0)
	battery := placeTestBuilding(t, w, 8, 10, 902, 1, 0)
	core := placeTestBuilding(t, w, 10, 10, 903, 1, 0)
	landingPad := placeTestBuilding(t, w, 12, 8, 904, 1, 0)
	display := placeTestBuilding(t, w, 12, 10, 905, 1, 0)
	press.Build.AddItem(coalItemID, 2)
	w.rebuildActiveTilesLocked()

	projectorPos := int32(projector.Y*w.Model().Width + projector.X)
	pressPos := int32(press.Y*w.Model().Width + press.X)
	batteryPos := int32(battery.Y*w.Model().Width + battery.X)
	corePos := int32(core.Y*w.Model().Width + core.X)
	landingPadPos := int32(landingPad.Y*w.Model().Width + landingPad.X)
	displayPos := int32(display.Y*w.Model().Width + display.X)
	w.overdriveProjectorStateLocked(projectorPos).Charge = overdriveProjectorProfiles["overdrive-projector"].Reload

	w.Step(time.Second / 60)

	if got := w.buildingTimeScaleLocked(projectorPos); got != 1 {
		t.Fatalf("expected overdrive projector to ignore itself, timeScale=%f", got)
	}
	if got := w.buildingTimeScaleLocked(pressPos); got <= 1 {
		t.Fatalf("expected graphite press to receive overdrive boost, timeScale=%f", got)
	}
	if got := w.buildingTimeScaleLocked(batteryPos); got != 1 {
		t.Fatalf("expected battery to stay unboosted, timeScale=%f", got)
	}
	if got := w.buildingTimeScaleLocked(corePos); got != 1 {
		t.Fatalf("expected core to stay unboosted, timeScale=%f", got)
	}
	if got := w.buildingTimeScaleLocked(landingPadPos); got != 1 {
		t.Fatalf("expected landing pad to stay unboosted, timeScale=%f", got)
	}
	if got := w.buildingTimeScaleLocked(displayPos); got != 1 {
		t.Fatalf("expected tile logic display to stay unboosted, timeScale=%f", got)
	}
}

func TestDestroyingBoostedBuildingClearsBoostState(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(20, 20)
	model.BlockNames = map[int16]string{
		900: "overdrive-projector",
		901: "graphite-press",
	}
	w.SetModel(model)

	projector := placeTestBuilding(t, w, 8, 8, 900, 1, 0)
	press := placeTestBuilding(t, w, 10, 8, 901, 1, 0)
	press.Build.AddItem(coalItemID, 2)
	w.rebuildActiveTilesLocked()

	projectorPos := int32(projector.Y*w.Model().Width + projector.X)
	pressPos := int32(press.Y*w.Model().Width + press.X)
	w.overdriveProjectorStateLocked(projectorPos).Charge = overdriveProjectorProfiles["overdrive-projector"].Reload

	w.Step(time.Second / 60)

	if got := w.buildingTimeScaleLocked(pressPos); got <= 1 {
		t.Fatalf("expected graphite press to receive overdrive boost, timeScale=%f", got)
	}

	if !w.applyDamageToBuildingRaw(pressPos, 5000) {
		t.Fatal("expected graphite press destruction to succeed")
	}
	if _, ok := w.buildingBoostStates[pressPos]; ok {
		t.Fatal("expected boosted building state to clear on destruction")
	}
	if got := w.buildingTimeScaleLocked(pressPos); got != 1 {
		t.Fatalf("expected destroyed building timeScale to reset, got %f", got)
	}
}

func TestDestroyingOverdriveProjectorClearsRuntimeState(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(20, 20)
	model.BlockNames = map[int16]string{
		900: "overdrive-projector",
	}
	w.SetModel(model)

	projector := placeTestBuilding(t, w, 8, 8, 900, 1, 0)
	projectorPos := int32(projector.Y*w.Model().Width + projector.X)
	w.overdriveProjectorStateLocked(projectorPos).Charge = 10

	if !w.applyDamageToBuildingRaw(projectorPos, 5000) {
		t.Fatal("expected overdrive projector destruction to succeed")
	}
	if _, ok := w.overdriveProjectorStates[projectorPos]; ok {
		t.Fatal("expected overdrive projector runtime state to clear on destruction")
	}
}

func TestUnitRepairTowerHealsMultipleUnits(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		478: "power-source",
		902: "unit-repair-tower",
	}
	w.SetModel(model)

	tower := placeTestBuilding(t, w, 12, 12, 902, 1, 0)
	placeTestBuilding(t, w, 12, 15, 478, 1, 0)
	tower.Build.AddLiquid(ozoneLiquidID, 30)
	linkPowerNode(t, w, 12, 15, protocol.Point2{X: 0, Y: -3})

	first := w.Model().AddEntity(RawEntity{ID: 5002, TypeID: 35, Team: 1, X: float32(10*8 + 4), Y: float32(12*8 + 4), Health: 30, MaxHealth: 100})
	second := w.Model().AddEntity(RawEntity{ID: 5003, TypeID: 35, Team: 1, X: float32(14*8 + 4), Y: float32(12*8 + 4), Health: 45, MaxHealth: 100})

	stepForSeconds(w, 3)

	gotFirst := findTestEntity(t, w, first.ID)
	gotSecond := findTestEntity(t, w, second.ID)
	if gotFirst.Health <= 30 || gotSecond.Health <= 45 {
		t.Fatalf("expected unit-repair-tower to heal both units, got=(%f,%f)", gotFirst.Health, gotSecond.Health)
	}
}

func TestSuppressionFieldAbilitySuppresssEnemyMendProjectorAndExpires(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		910: "mend-projector",
		911: "container",
	}
	model.UnitNames = map[int16]string{
		950: "suppressor",
	}
	w.SetModel(model)
	w.unitRuntimeProfilesByName["suppressor"] = unitRuntimeProfile{
		Name: "suppressor",
		Abilities: []unitAbilityProfile{{
			Kind:     unitAbilitySuppressionField,
			Active:   true,
			Reload:   1.5,
			Cooldown: 1.5,
			Range:    200,
		}},
	}

	projector := placeTestBuilding(t, w, 12, 12, 910, 2, 0)
	target := placeTestBuilding(t, w, 14, 12, 911, 2, 0)
	target.Build.Health = 400
	target.Build.MaxHealth = 1000

	w.Model().AddEntity(RawEntity{
		ID:          7001,
		TypeID:      950,
		Team:        1,
		X:           float32(12*8 + 4),
		Y:           float32(8*8 + 4),
		Health:      100,
		MaxHealth:   100,
		RuntimeInit: true,
		SlowMul:     1,
	})

	stepForSeconds(w, 1.6)

	if !w.isBuildingHealSuppressedLocked(projector.Build) {
		t.Fatal("expected suppression ability to suppress enemy mend-projector")
	}

	projectorPos := int32(projector.Y*model.Width + projector.X)
	w.mendProjectorStateLocked(projectorPos).Charge = mendProjectorProfiles["mend-projector"].Reload
	beforeSuppressed := target.Build.Health
	w.stepSupportBuildingsLocked(time.Second / 60)
	if got := target.Build.Health; got != beforeSuppressed {
		t.Fatalf("expected suppressed mend-projector to stop healing, before=%f after=%f", beforeSuppressed, got)
	}

	w.Model().Entities = nil
	w.mendProjectorStateLocked(projectorPos).Charge = 0
	stepForSeconds(w, 2)

	if w.isBuildingHealSuppressedLocked(projector.Build) {
		t.Fatal("expected mend-projector suppression to expire after suppressor is gone")
	}

	beforeExpired := target.Build.Health
	w.mendProjectorStateLocked(projectorPos).Charge = mendProjectorProfiles["mend-projector"].Reload
	w.stepSupportBuildingsLocked(time.Second / 60)
	if got := target.Build.Health; got <= beforeExpired {
		t.Fatalf("expected mend-projector healing to resume after suppression expires, before=%f after=%f", beforeExpired, got)
	}
}

func TestSuppressionFieldAbilitySuppressesEnemyUnitRepairTower(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		478: "power-source",
		902: "unit-repair-tower",
	}
	model.UnitNames = map[int16]string{
		950: "suppressor",
	}
	w.SetModel(model)
	w.unitRuntimeProfilesByName["suppressor"] = unitRuntimeProfile{
		Name: "suppressor",
		Abilities: []unitAbilityProfile{{
			Kind:     unitAbilitySuppressionField,
			Active:   true,
			Reload:   1.5,
			Cooldown: 1.5,
			Range:    200,
		}},
	}

	tower := placeTestBuilding(t, w, 12, 12, 902, 2, 0)
	placeTestBuilding(t, w, 12, 15, 478, 2, 0)
	tower.Build.AddLiquid(ozoneLiquidID, 30)
	linkPowerNode(t, w, 12, 15, protocol.Point2{X: 0, Y: -3})

	w.Model().AddEntity(RawEntity{
		ID:          7002,
		TypeID:      950,
		Team:        1,
		X:           float32(12*8 + 4),
		Y:           float32(8*8 + 4),
		Health:      100,
		MaxHealth:   100,
		RuntimeInit: true,
		SlowMul:     1,
	})

	stepForSeconds(w, 1.6)

	if !w.isBuildingHealSuppressedLocked(tower.Build) {
		t.Fatal("expected suppression ability to suppress enemy unit-repair-tower")
	}

	unit := w.Model().AddEntity(RawEntity{
		ID:        7003,
		TypeID:    35,
		Team:      2,
		X:         float32(10*8 + 4),
		Y:         float32(12*8 + 4),
		Health:    20,
		MaxHealth: 100,
	})

	towerPos := int32(tower.Y*model.Width + tower.X)
	w.repairTowerStates[towerPos] = repairTowerRuntimeState{Targets: []int32{unit.ID}}
	beforeSuppressed := findTestEntity(t, w, unit.ID).Health
	w.stepRepairBlocks(time.Second / 60)
	if got := findTestEntity(t, w, unit.ID).Health; got != beforeSuppressed {
		t.Fatalf("expected suppressed unit-repair-tower to stop healing, before=%f after=%f", beforeSuppressed, got)
	}

	w.Model().Entities = w.Model().Entities[1:]
	stepForSeconds(w, 2)

	if w.isBuildingHealSuppressedLocked(tower.Build) {
		t.Fatal("expected unit-repair-tower suppression to expire after suppressor is gone")
	}

	w.repairTowerStates[towerPos] = repairTowerRuntimeState{Targets: []int32{unit.ID}}
	beforeExpired := findTestEntity(t, w, unit.ID).Health
	w.Step(time.Second / 60)
	if got := findTestEntity(t, w, unit.ID).Health; got <= beforeExpired {
		t.Fatalf("expected unit-repair-tower healing to resume after suppression expires, before=%f after=%f", beforeExpired, got)
	}
}

func TestSlagIncineratorAcceptsSlagAndBurnsItems(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(16, 16)
	model.BlockNames = map[int16]string{
		500: "conduit",
		903: "slag-incinerator",
	}
	w.SetModel(model)

	placeTestBuilding(t, w, 5, 6, 500, 1, 0)
	inc := placeTestBuilding(t, w, 6, 6, 903, 1, 0)

	srcPos := int32(6*model.Width + 5)
	incPos := int32(6*model.Width + 6)
	if moved := w.tryMoveLiquidLocked(srcPos, incPos, slagLiquidID, 5, 0); moved <= 0 {
		t.Fatalf("expected slag-incinerator to accept slag input, moved=%f", moved)
	}

	stepForSeconds(w, 1)

	if !w.tryInsertItemLocked(srcPos, incPos, copperItemID, 0) {
		t.Fatal("expected slag-incinerator to burn items after slag gate is present")
	}
	if got := totalBuildingItems(inc.Build); got != 0 {
		t.Fatalf("expected slag-incinerator to keep no item inventory, got=%d", got)
	}
	if got := inc.Build.LiquidAmount(slagLiquidID); got <= 0 {
		t.Fatalf("expected slag gate liquid to stay stored, got=%f", got)
	}
}

func TestImpactDrillRequiresWater(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		2:   "ore-beryllium",
		478: "power-source",
		904: "impact-drill",
	}
	w.SetModel(model)

	paintAreaOverlay(t, w, 8, 8, 4, 2)
	drill := placeTestBuilding(t, w, 8, 8, 904, 1, 0)
	placeTestBuilding(t, w, 8, 12, 478, 1, 0)
	linkPowerNode(t, w, 8, 12, protocol.Point2{X: 0, Y: -4})

	stepForSeconds(w, 7)

	if got := totalBuildingItems(drill.Build); got != 0 {
		t.Fatalf("expected impact-drill without water to stay idle, items=%d", got)
	}
}

func TestImpactDrillProducesBurstWithWaterAndPower(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		2:   "ore-beryllium",
		478: "power-source",
		904: "impact-drill",
	}
	w.SetModel(model)

	paintAreaOverlay(t, w, 8, 8, 4, 2)
	drill := placeTestBuilding(t, w, 8, 8, 904, 1, 0)
	drill.Build.AddLiquid(waterLiquidID, 100)
	placeTestBuilding(t, w, 8, 12, 478, 1, 0)
	linkPowerNode(t, w, 8, 12, protocol.Point2{X: 0, Y: -4})

	stepForSeconds(w, 7)

	if got := totalBuildingItems(drill.Build); got <= 0 {
		pos := int32(8*model.Width + 8)
		t.Fatalf("expected impact-drill burst to produce items, got=%d state=%+v power=%f liquids=%v", got, w.burstDrillStates[pos], w.blockSyncPowerStatusLocked(pos, &w.Model().Tiles[pos], "impact-drill"), drill.Build.Liquids)
	}
}

func TestEruptionDrillProducesWithHydrogen(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(28, 28)
	model.BlockNames = map[int16]string{
		2:   "ore-tungsten",
		478: "power-source",
		905: "eruption-drill",
	}
	w.SetModel(model)

	paintAreaOverlay(t, w, 10, 10, 5, 2)
	drill := placeTestBuilding(t, w, 10, 10, 905, 1, 0)
	drill.Build.AddLiquid(hydrogenLiquidID, 40)
	placeTestBuilding(t, w, 10, 15, 478, 1, 0)
	linkPowerNode(t, w, 10, 15, protocol.Point2{X: 0, Y: -5})

	stepForSeconds(w, 6)

	if got := totalBuildingItems(drill.Build); got <= 0 {
		t.Fatalf("expected eruption-drill to produce items with hydrogen, got=%d", got)
	}
}

func TestPlasmaBoreMinesWallOreWhenPowered(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(24, 24)
	model.BlockNames = map[int16]string{
		2:   "ore-beryllium",
		478: "power-source",
		906: "plasma-bore",
	}
	w.SetModel(model)

	drill := placeTestBuilding(t, w, 8, 8, 906, 1, 0)
	placeTestBuilding(t, w, 8, 12, 478, 1, 0)
	linkPowerNode(t, w, 8, 12, protocol.Point2{X: 0, Y: -4})

	skip := map[int32]struct{}{}
	low, high := blockFootprintRange(blockSizeByName("plasma-bore"))
	for dy := low; dy <= high; dy++ {
		for dx := low; dx <= high; dx++ {
			skip[packTilePos(8+dx, 8+dy)] = struct{}{}
		}
	}
	skip[packTilePos(8, 12)] = struct{}{}
	paintWallRect(t, w, 4, 4, 16, 16, 2, skip)

	stepForSeconds(w, 4)

	if got := totalBuildingItems(drill.Build); got <= 0 {
		pos := int32(8*model.Width + 8)
		t.Fatalf("expected plasma-bore to mine surrounding wall ore, got=%d state=%+v power=%f", got, w.beamDrillStates[pos], w.blockSyncPowerStatusLocked(pos, &w.Model().Tiles[pos], "plasma-bore"))
	}
}

func TestLargePlasmaBoreRequiresHydrogen(t *testing.T) {
	w := New(Config{TPS: 60})
	model := NewWorldModel(28, 28)
	model.BlockNames = map[int16]string{
		2:   "ore-tungsten",
		478: "power-source",
		907: "large-plasma-bore",
	}
	w.SetModel(model)

	drill := placeTestBuilding(t, w, 10, 10, 907, 1, 0)
	placeTestBuilding(t, w, 10, 15, 478, 1, 0)
	linkPowerNode(t, w, 10, 15, protocol.Point2{X: 0, Y: -5})

	skip := map[int32]struct{}{}
	low, high := blockFootprintRange(blockSizeByName("large-plasma-bore"))
	for dy := low; dy <= high; dy++ {
		for dx := low; dx <= high; dx++ {
			skip[packTilePos(10+dx, 10+dy)] = struct{}{}
		}
	}
	skip[packTilePos(10, 15)] = struct{}{}
	paintWallRect(t, w, 4, 4, 18, 18, 2, skip)

	stepForSeconds(w, 4)
	if got := totalBuildingItems(drill.Build); got != 0 {
		t.Fatalf("expected large-plasma-bore without hydrogen to stay idle, got=%d", got)
	}

	drill.Build.AddLiquid(hydrogenLiquidID, 30)
	stepForSeconds(w, 3)
	if got := totalBuildingItems(drill.Build); got <= 0 {
		pos := int32(10*model.Width + 10)
		t.Fatalf("expected large-plasma-bore with hydrogen to mine, got=%d state=%+v power=%f liquids=%v", got, w.beamDrillStates[pos], w.blockSyncPowerStatusLocked(pos, &w.Model().Tiles[pos], "large-plasma-bore"), drill.Build.Liquids)
	}
}
