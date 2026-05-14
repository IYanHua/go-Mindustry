package world

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/IYanHua/mdt-server/internal/protocol"
	"github.com/IYanHua/mdt-server/internal/vanilla"
)

type Snapshot struct {
	WaveTime float32
	Wave     int32
	Enemies  int32
	Paused   bool
	GameOver bool
	TimeData int32
	Tps      int8
	Rand0    int64
	Rand1    int64
	Tick     uint64
}

func (s Snapshot) WaveTimeTicks() float32 {
	// Vanilla stores and transmits GameState.wavetime in 60Hz simulation ticks.
	// The stateSnapshot TPS byte is diagnostic/interpolation data, not the unit
	// scale for wave time.
	return s.WaveTime * 60
}

type TeamCoreItemSnapshot struct {
	Team  TeamID
	Items []ItemStack
}

type completedBuildingPlacement struct {
	Config            any
	SelfConfigTargets []int32
	ChangedConfigs    []powerAutoLinkChange
}

type Config struct {
	TPS                    int
	UseMapSyncDataFallback bool
	BlockSyncLogsEnabled   bool
}

type itemLogisticsPerf struct {
	Junctions time.Duration
	Conveyor  time.Duration
	Duct      time.Duration
	Router    time.Duration
	Bridge    time.Duration
	Unloader  time.Duration
	MassDrive time.Duration

	JunctionCount  int
	ConveyorCount  int
	DuctCount      int
	RouterCount    int
	BridgeCount    int
	UnloaderCount  int
	MassDriveCount int
}

type entitySpatialIndex struct {
	cellSize int
	cells    map[int64][]int
}

type buildingSpatialIndex struct {
	cellSize int
	cells    map[int64][]int32
}

type World struct {
	mu sync.RWMutex

	wave     int32
	waveTime float32
	tick     uint64
	timeSec  float32

	rand0 int64
	rand1 int64

	tps       int8
	actualTps int8

	tpsWindowStart time.Time
	tpsWindowTicks int32
	perfLogAt      time.Time

	start time.Time

	model *WorldModel

	paused   bool
	gameOver bool

	// 规则和波次管理器
	rulesMgr *RulesManager
	wavesMgr *WaveManager

	// 同步配置
	useMapSyncDataFallback bool
	blockSyncLogsEnabled   bool
	scheduler              worldDispatcher

	entityEvents      []EntityEvent
	bullets           []simBullet
	pendingMountShots []pendingMountShot
	bulletNextID      int32
	blockItemSyncTick map[int32]uint64

	blockNamesByID              map[int16]string
	blockNamesByIndex           []string
	powerStorageCapacityByBlock []float32
	unitNamesByID               map[int16]string
	unitTypeDefsByID            map[int16]vanilla.UnitTypeDef
	buildStates                 map[int32]buildCombatState
	controlledBuilds            map[int32]controlledBuildState
	controlledBuildByPlayer     map[int32]int32
	pendingBuilds               map[int32]pendingBuildState
	pendingBreaks               map[int32]pendingBreakState
	buildRejectLogTick          map[int32]uint64
	builderStates               map[int32]builderRuntimeState
	teamRebuildPlans            map[TeamID][]rebuildBlockPlan
	teamAIBuildPlans            map[TeamID][]teamBuildPlan
	teamBuildAIStates           map[TeamID]buildAIPlannerState
	buildAIParts                []buildAIBasePart
	buildAIPartsLoaded          bool
	factoryStates               map[int32]factoryState
	reconstructorStates         map[int32]reconstructorState
	drillStates                 map[int32]drillRuntimeState
	burstDrillStates            map[int32]burstDrillRuntimeState
	beamDrillStates             map[int32]beamDrillRuntimeState
	pumpStates                  map[int32]pumpRuntimeState
	crafterStates               map[int32]crafterRuntimeState
	heatStates                  map[int32]float32
	incineratorStates           map[int32]float32
	repairTurretStates          map[int32]repairTurretRuntimeState
	repairTowerStates           map[int32]repairTowerRuntimeState
	teamPowerStates             map[TeamID]*teamPowerState
	teamPowerBudget             map[TeamID]float32
	powerNetStates              map[int32]*powerNetState
	powerNetByPos               map[int32]int32
	powerNetIDs                 []int32
	powerNetTeams               map[int32]TeamID
	powerNetStorageRefs         map[int32][]powerStorageRef
	powerNetDirty               bool
	powerStorageState           map[int32]float32
	powerRequested              map[int32]float32
	powerSupplied               map[int32]float32
	powerGeneratorState         map[int32]*powerGeneratorState
	unitMountCDs                map[int32][]float32
	unitMountStates             map[int32][]unitMountState
	unitTargets                 map[int32]targetTrackState
	unitAIStates                map[int32]unitAIState
	unitMiningStates            map[int32]unitMiningState
	teamItems                   map[TeamID]map[ItemID]int32
	teamBuilderSpeed            map[TeamID]float32
	itemSourceCfg               map[int32]ItemID
	liquidSourceCfg             map[int32]LiquidID
	sorterCfg                   map[int32]ItemID
	unloaderCfg                 map[int32]ItemID
	payloadRouterCfg            map[int32]protocol.Content
	powerNodeLinks              map[int32][]int32
	bridgeLinks                 map[int32]int32
	massDriverLinks             map[int32]int32
	payloadDriverLinks          map[int32]int32
	bridgeBuffers               map[int32][]bufferedBridgeItem
	bridgeAcceptAcc             map[int32]float32
	conveyorStates              map[int32]*conveyorRuntimeState
	ductStates                  map[int32]*ductRuntimeState
	routerStates                map[int32]*routerRuntimeState
	stackStates                 map[int32]*stackRuntimeState
	massDriverStates            map[int32]*massDriverRuntimeState
	payloadStates               map[int32]*payloadRuntimeState
	payloadDeconstructorStates  map[int32]*payloadDeconstructorState
	payloadDriverStates         map[int32]*payloadDriverRuntimeState
	massDriverShots             []massDriverShot
	payloadDriverShots          []payloadDriverShot
	blockDumpIndex              map[int32]int
	dumpNeighborCache           map[int32][]int32
	unloaderLastUsed            map[int64]int
	itemSourceAccum             map[int32]float32
	routerInputPos              map[int32]int32
	routerRotation              map[int32]byte
	transportAccum              map[int32]float32
	junctionQueues              map[int32]junctionQueueState
	bridgeIncomingMask          map[int32]byte
	reactorStates               map[int32]nuclearReactorState
	storageLinkedCore           map[int32]int32
	teamPrimaryCore             map[TeamID]int32
	coreStorageCapacity         map[int32]int32
	blockOccupancy              map[int32]int32
	activeTilePositions         []int32
	itemLogisticsTilePositions  []int32
	itemConveyorTilePositions   []int32
	itemDuctTilePositions       []int32
	itemRouterTilePositions     []int32
	itemBridgeTilePositions     []int32
	itemUnloaderTilePositions   []int32
	itemMassDriverTilePositions []int32
	sandboxItemSourceTiles      []int32
	sandboxLiquidSourceTiles    []int32
	liquidConduitTilePositions  []int32
	liquidStorageTilePositions  []int32
	liquidBridgeTilePositions   []int32
	payloadFactoryTilePositions []int32
	payloadTransportTiles       []int32
	reactorTilePositions        []int32
	crafterTilePositions        []int32
	drillTilePositions          []int32
	burstDrillTilePositions     []int32
	beamDrillTilePositions      []int32
	pumpTilePositions           []int32
	incineratorTilePositions    []int32
	repairTurretTilePositions   []int32
	repairTowerTilePositions    []int32
	factoryTilePositions        []int32
	heatConductorTilePositions  []int32
	powerTilePositions          []int32
	powerDiodeTilePositions     []int32
	powerVoidTilePositions      []int32
	teamBuildingTiles           map[TeamID][]int32
	teamBuildingSpatial         map[TeamID]*buildingSpatialIndex
	teamCoreTiles               map[TeamID][]int32
	teamPowerTiles              map[TeamID][]int32
	teamPowerNodeTiles          map[TeamID][]int32
	turretTilePositions         []int32
	turretStates                map[int32]*turretRuntimeState
	mendProjectorPositions      []int32
	mendProjectorStates         map[int32]*mendProjectorState
	overdriveProjectorPositions []int32
	overdriveProjectorStates    map[int32]*overdriveProjectorState
	buildingBoostStates         map[int32]buildingBoostState
	forceProjectorPositions     []int32
	forceProjectorStates        map[int32]*forceProjectorState
	nextPlanOrder               uint64

	unitProfilesByType        map[int16]weaponProfile
	unitProfilesByName        map[string]weaponProfile
	unitRuntimeProfilesByName map[string]unitRuntimeProfile
	unitMountProfilesByName   map[string][]unitWeaponMountProfile
	buildingProfilesByName    map[string]buildingWeaponProfile
	blockCostsByName          map[string][]ItemStack
	blockBuildTimesByName     map[string]float32
	blockArmorByName          map[string]float32
	statusProfilesByID        map[int16]statusEffectProfile
	statusProfilesByName      map[string]statusEffectProfile

	groundPathPrev    []int32
	groundPathCost    []int32
	groundPathSeen    []uint32
	groundPathClosed  []uint32
	groundPathHeap    []groundPathNode
	groundPathVisitID uint32
}

const (
	// Vars.buildingRange in Mindustry 156.
	vanillaBuilderRange = 220.0
	// Builder state comes from clientSnapshot and should stop driving progress
	// quickly once snapshots stop arriving.
)

type BuildPlanOp struct {
	Breaking bool
	X        int32
	Y        int32
	Rotation int8
	BlockID  int16
	Config   any
}

type RotateBuildingResult struct {
	BlockID   int16
	Rotation  int8
	Team      TeamID
	EffectX   float32
	EffectY   float32
	EffectRot float32
}

type BuildingInfo struct {
	Pos      int32
	X        int32
	Y        int32
	BlockID  int16
	Name     string
	Team     TeamID
	Rotation int8
}

type BuildSyncState struct {
	Pos      int32
	X        int32
	Y        int32
	BlockID  int16
	Team     TeamID
	Rotation int8
	Health   float32
}

type BuildSyncSnapshotEntry struct {
	BuildSyncState
	Config []byte
}

type pendingBuildState struct {
	Owner            int32
	Team             TeamID
	BlockID          int16
	Rotation         int8
	Config           any
	QueueOrder       uint64
	Progress         float32
	VisualPlaced     bool
	LastHP           float32
	BuildCost        []ItemStack
	ItemsLeft        []int32
	Accumulator      []float32
	TotalAccumulator []float32
}

type builderRuntimeState struct {
	Owner      int32
	Team       TeamID
	UnitID     int32
	X          float32
	Y          float32
	Active     bool
	BuildRange float32
	UpdatedAt  time.Time
}

type rebuildBlockPlan struct {
	X        int32
	Y        int32
	Rotation int8
	BlockID  int16
	Config   any
}

type buildAIPlannerState struct {
	PlanScanCD     float32
	SpawnCD        float32
	RefreshPathCD  float32
	StartedPathing bool
	FoundPath      bool
	PathCells      map[int32]struct{}
}

type pendingBreakState struct {
	Owner       int32
	Team        TeamID
	BlockID     int16
	Rotation    int8
	QueueOrder  uint64
	VisualStart bool
	Progress    float32
	MaxHealth   float32
	LastHP      float32
	RefundTeam  TeamID
	RefundCost  []ItemStack
	RefundAccum map[ItemID]float32
	RefundTotal map[ItemID]float32
	Refunded    map[ItemID]int32
}

const constructBlockHealthMax = float32(10)

type factoryState struct {
	Progress    float32
	UnitType    int16
	CurrentPlan int16
	CommandPos  *protocol.Vec2
	Command     *protocol.UnitCommand
}

type drillRuntimeState struct {
	Progress float32
	Warmup   float32
}

type burstDrillRuntimeState struct {
	Progress float32
	Warmup   float32
}

type beamDrillRuntimeState struct {
	Time   float32
	Warmup float32
}

type repairTurretRuntimeState struct {
	Rotation       float32
	Strength       float32
	SearchProgress float32
	TargetID       int32
}

type repairTowerRuntimeState struct {
	Refresh       float32
	Warmup        float32
	TotalProgress float32
	Targets       []int32
}

type pumpRuntimeState struct {
	Warmup      float32
	Progress    float32
	Accumulator float32
}

type crafterRuntimeState struct {
	Progress      float32
	Warmup        float32
	TotalProgress float32
	Seed          uint32
}

type nuclearReactorState struct {
	Heat         float32
	HeatProgress float32
	FuelProgress float32
}

type bufferedBridgeItem struct {
	Item      ItemID
	AgeFrames float32
}

type conveyorRuntimeState struct {
	IDs          [3]ItemID
	XS           [3]float32
	YS           [3]float32
	Len          int
	LastInserted int
	Mid          int
	MinItem      float32
}

type routerRuntimeState struct {
	LastItem  ItemID
	HasItem   bool
	LastInput int32
	Time      float32
}

type ductRuntimeState struct {
	Progress float32
	Current  ItemID
	HasItem  bool
	RecDir   byte
}

type stackRuntimeState struct {
	Link      int32
	Cooldown  float32
	LastItem  ItemID
	HasItem   bool
	Unloading bool
}

type massDriverRuntimeState struct {
	ReloadCounter float32
}

type massDriverShot struct {
	FromPos      int32
	ToPos        int32
	TravelFrames float32
	AgeFrames    float32
	Transferred  []ItemStack
}

type payloadKind byte

const (
	payloadKindUnit payloadKind = iota
	payloadKindBlock
)

type payloadData struct {
	Kind       payloadKind
	BlockID    int16
	UnitTypeID int16
	Rotation   int8
	Serialized []byte
	Config     []byte
	Items      []ItemStack
	Liquids    []LiquidStack
	Power      float32
	Health     float32
	MaxHealth  float32
	UnitState  *RawEntity
}

type payloadRuntimeState struct {
	Payload   *payloadData
	Move      float32
	Work      float32
	RecDir    byte
	Exporting bool
}

type payloadDriverRuntimeState struct {
	ReloadCounter float32
	Charge        float32
}

type payloadDriverShot struct {
	FromPos      int32
	ToPos        int32
	TravelFrames float32
	AgeFrames    float32
	Payload      *payloadData
}

type junctionQueuedItem struct {
	Item    ItemID
	FromDir byte
	AgeSec  float32
}

type junctionQueueState [4][]junctionQueuedItem

type unloaderCandidateStat struct {
	pos        int32
	loadFactor float32
	canLoad    bool
	canUnload  bool
	notStorage bool
	lastUsed   int
}

type itemIDSeenSet struct {
	bits  uint64
	extra map[ItemID]struct{}
}

func (s *itemIDSeenSet) add(item ItemID) bool {
	if s == nil {
		return false
	}
	if item >= 0 && item < 64 {
		mask := uint64(1) << uint(item)
		if (s.bits & mask) != 0 {
			return false
		}
		s.bits |= mask
		return true
	}
	if s.extra == nil {
		s.extra = make(map[ItemID]struct{}, 4)
	}
	if _, exists := s.extra[item]; exists {
		return false
	}
	s.extra[item] = struct{}{}
	return true
}

type protocolContentLiquid LiquidID

func (l protocolContentLiquid) ContentType() protocol.ContentType { return protocol.ContentLiquid }
func (l protocolContentLiquid) ID() int16                         { return int16(l) }
func (l protocolContentLiquid) Name() string                      { return "" }

type EntityEventKind string

const (
	EntityEventRemoved             EntityEventKind = "removed"
	EntityEventBuildPlaced         EntityEventKind = "build_placed"
	EntityEventBuildConstructed    EntityEventKind = "build_constructed"
	EntityEventBuildConfig         EntityEventKind = "build_config"
	EntityEventBuildDeconstructing EntityEventKind = "build_deconstructing"
	EntityEventBuildCancelled      EntityEventKind = "build_cancelled"
	EntityEventBuildDestroyed      EntityEventKind = "build_destroyed"
	EntityEventBuildHealth         EntityEventKind = "build_health"
	EntityEventTeamItems           EntityEventKind = "team_items"
	EntityEventBlockItemSync       EntityEventKind = "block_item_sync"
	EntityEventItemTurretAmmoSync  EntityEventKind = "item_turret_ammo_sync"
	EntityEventTransferItemToUnit  EntityEventKind = "transfer_item_to_unit"
	EntityEventTransferItemToBuild EntityEventKind = "transfer_item_to_build"
	EntityEventBulletFired         EntityEventKind = "bullet_fired"
	EntityEventEffect              EntityEventKind = "effect"
)

type EntityEvent struct {
	Kind   EntityEventKind
	Entity RawEntity
	// BuildPos is packed tile position (Point2), not linear tile index.
	BuildPos    int32
	BuildOwner  int32
	BuildTeam   TeamID
	BuildBlock  int16
	BuildRot    int8
	BuildConfig any
	BuildHP     float32
	ItemID      ItemID
	ItemAmount  int32
	UnitID      int32
	TransferX   float32
	TransferY   float32
	Bullet      BulletEvent
	EffectName  string
	EffectX     float32
	EffectY     float32
	EffectRot   float32
}

func (w *World) appendBuildConfigEventLocked(pos int32) {
	if w == nil || w.model == nil || pos < 0 || int(pos) >= len(w.model.Tiles) {
		return
	}
	tile := &w.model.Tiles[pos]
	if tile.Build == nil || tile.Block == 0 {
		return
	}
	cfg, ok := w.normalizedBuildingConfigLocked(pos)
	if !ok {
		if tile.Build == nil || len(tile.Build.Config) == 0 {
			return
		}
		var decoded any
		decoded, ok = decodeStoredBuildingConfig(tile.Build.Config)
		if !ok {
			return
		}
		cfg = decoded
	}
	w.entityEvents = append(w.entityEvents, EntityEvent{
		Kind:        EntityEventBuildConfig,
		BuildPos:    packTilePos(tile.X, tile.Y),
		BuildTeam:   tile.Build.Team,
		BuildBlock:  int16(tile.Block),
		BuildRot:    tile.Rotation,
		BuildConfig: cfg,
	})
}

func (w *World) appendBuildConfigValueEventLocked(pos int32, value any) {
	if w == nil || w.model == nil || pos < 0 || int(pos) >= len(w.model.Tiles) {
		return
	}
	tile := &w.model.Tiles[pos]
	if tile.Build == nil || tile.Block == 0 {
		return
	}
	w.entityEvents = append(w.entityEvents, EntityEvent{
		Kind:        EntityEventBuildConfig,
		BuildPos:    packTilePos(tile.X, tile.Y),
		BuildTeam:   tile.Build.Team,
		BuildBlock:  int16(tile.Block),
		BuildRot:    tile.Rotation,
		BuildConfig: value,
	})
}

func (w *World) emitBlockItemSyncLocked(pos int32) {
	if w == nil || w.model == nil || pos < 0 || int(pos) >= len(w.model.Tiles) {
		return
	}
	if _, _, _, ok := w.sharedCoreInventoryLocked(pos); ok {
		return
	}
	tile := &w.model.Tiles[pos]
	if tile.Build == nil || tile.Block == 0 || tile.Build.Team == 0 {
		return
	}
	name := w.blockNameByID(int16(tile.Block))
	kind := w.classifyBlockSyncKindLocked(pos, tile, name)
	if kind == blockSyncNone {
		return
	}
	shouldSync := w.hasItemModuleForBlockSyncLocked(tile, name, kind)
	switch kind {
	case blockSyncUnitFactory, blockSyncReconstructor:
		shouldSync = true
	}
	// Mindustry-157 only syncs turrets through the shared blockSnapshot pass.
	// Pushing item-turret ammo changes through this extra event path creates a
	// second snapshot writer for the same build state and can rewind ammo views.
	if kind == blockSyncItemTurret {
		if lastTick, ok := w.blockItemSyncTick[pos]; ok && lastTick == w.tick {
			return
		}
		w.blockItemSyncTick[pos] = w.tick
		if w.blockSyncLogsEnabled {
			log.Printf("[turret-ammo] enqueue-sync pos=%d (%d,%d) block=%s tileTeam=%d buildTeam=%d tick=%d stacks=%s",
				pos, tile.X, tile.Y, name, tile.Team, tile.Build.Team, w.tick, w.debugItemStacksLocked(tile.Build.Items))
		}
		w.entityEvents = append(w.entityEvents, EntityEvent{
			Kind:       EntityEventItemTurretAmmoSync,
			BuildPos:   packTilePos(tile.X, tile.Y),
			BuildTeam:  tile.Build.Team,
			BuildBlock: int16(tile.Block),
		})
		return
	}
	if isPayloadProcessorBlockSyncKind(kind) {
		shouldSync = true
	}
	if !shouldSync {
		return
	}
	if lastTick, ok := w.blockItemSyncTick[pos]; ok && lastTick == w.tick {
		return
	}
	w.blockItemSyncTick[pos] = w.tick
	w.entityEvents = append(w.entityEvents, EntityEvent{
		Kind:       EntityEventBlockItemSync,
		BuildPos:   packTilePos(tile.X, tile.Y),
		BuildTeam:  tile.Build.Team,
		BuildBlock: int16(tile.Block),
	})
}

func itemStacksEqualByItem(a, b []ItemStack) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	counts := make(map[ItemID]int32, len(a)+len(b))
	nonZero := 0
	for _, stack := range a {
		if stack.Amount <= 0 {
			continue
		}
		counts[stack.Item] += stack.Amount
		nonZero++
	}
	for _, stack := range b {
		if stack.Amount <= 0 {
			continue
		}
		counts[stack.Item] -= stack.Amount
		nonZero++
	}
	if nonZero == 0 {
		return true
	}
	for _, amount := range counts {
		if amount != 0 {
			return false
		}
	}
	return true
}

func (w *World) replaceBuildingItemsLocked(pos int32, tile *Tile, items []ItemStack) {
	if tile == nil || tile.Build == nil {
		return
	}
	if itemStacksEqualByItem(tile.Build.Items, items) {
		return
	}
	if len(items) == 0 {
		tile.Build.Items = nil
		w.emitBlockItemSyncLocked(pos)
		return
	}
	if cap(tile.Build.Items) < len(items) {
		tile.Build.Items = make([]ItemStack, len(items))
	} else {
		tile.Build.Items = tile.Build.Items[:len(items)]
	}
	copy(tile.Build.Items, items)
	w.emitBlockItemSyncLocked(pos)
}

func (w *World) invalidateItemRoutingCachesLocked() {
	w.dumpNeighborCache = map[int32][]int32{}
	w.bridgeIncomingMask = map[int32]byte{}
}

func (w *World) debugItemStacksLocked(stacks []ItemStack) string {
	if len(stacks) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, stack := range stacks {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(strconv.Itoa(int(stack.Item)))
		b.WriteByte(':')
		b.WriteString(strconv.Itoa(int(stack.Amount)))
	}
	b.WriteByte(']')
	return b.String()
}

func (w *World) BlockSyncLogsEnabled() bool {
	if w == nil {
		return false
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.blockSyncLogsEnabled
}

func (w *World) DebugItemTurretAmmoPacked(packedPos int32) string {
	if w == nil {
		return "world=nil"
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.model == nil {
		return "model=nil"
	}
	pos, ok := w.tileIndexFromPackedPosLocked(packedPos)
	if !ok || pos < 0 || int(pos) >= len(w.model.Tiles) {
		return fmt.Sprintf("packed=%d missing", packedPos)
	}
	tile := &w.model.Tiles[pos]
	if tile == nil {
		return fmt.Sprintf("packed=%d tile=nil", packedPos)
	}
	if tile.Build == nil || tile.Block == 0 {
		return fmt.Sprintf("packed=%d (%d,%d) block=%d build=nil tileTeam=%d", packedPos, tile.X, tile.Y, tile.Block, tile.Team)
	}
	name := w.blockNameByID(int16(tile.Block))
	totalAmmo := int32(0)
	if prof, ok := w.getBuildingWeaponProfile(int16(tile.Build.Block)); ok && w.buildingUsesItemAmmoLocked(tile, prof) {
		totalAmmo = w.totalBuildingAmmoLocked(tile, prof)
	}
	return fmt.Sprintf("packed=%d (%d,%d) block=%s tileTeam=%d buildTeam=%d totalAmmo=%d stacks=%s",
		packedPos, tile.X, tile.Y, name, tile.Team, tile.Build.Team, totalAmmo, w.debugItemStacksLocked(tile.Build.Items))
}

func packTilePos(x, y int) int32 {
	return (int32(x)&0xFFFF)<<16 | (int32(y) & 0xFFFF)
}

func unpackTilePos(pos int32) (int, int) {
	return int(uint16((pos >> 16) & 0xFFFF)), int(uint16(pos & 0xFFFF))
}

func New(cfg Config) *World {
	tps := cfg.TPS
	if tps <= 0 {
		tps = 60
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	return &World{
		wave:                        1,
		waveTime:                    0,
		tick:                        0,
		rand0:                       rng.Int63(),
		rand1:                       rng.Int63(),
		tps:                         int8(tps),
		actualTps:                   int8(tps),
		tpsWindowStart:              time.Now(),
		start:                       time.Now(),
		useMapSyncDataFallback:      cfg.UseMapSyncDataFallback,
		blockSyncLogsEnabled:        cfg.BlockSyncLogsEnabled,
		pendingMountShots:           []pendingMountShot{},
		bulletNextID:                1,
		blockItemSyncTick:           map[int32]uint64{},
		buildStates:                 map[int32]buildCombatState{},
		controlledBuilds:            map[int32]controlledBuildState{},
		controlledBuildByPlayer:     map[int32]int32{},
		pendingBuilds:               map[int32]pendingBuildState{},
		pendingBreaks:               map[int32]pendingBreakState{},
		buildRejectLogTick:          map[int32]uint64{},
		builderStates:               map[int32]builderRuntimeState{},
		teamRebuildPlans:            map[TeamID][]rebuildBlockPlan{},
		teamAIBuildPlans:            map[TeamID][]teamBuildPlan{},
		teamBuildAIStates:           map[TeamID]buildAIPlannerState{},
		factoryStates:               map[int32]factoryState{},
		reconstructorStates:         map[int32]reconstructorState{},
		drillStates:                 map[int32]drillRuntimeState{},
		burstDrillStates:            map[int32]burstDrillRuntimeState{},
		beamDrillStates:             map[int32]beamDrillRuntimeState{},
		pumpStates:                  map[int32]pumpRuntimeState{},
		crafterStates:               map[int32]crafterRuntimeState{},
		heatStates:                  map[int32]float32{},
		incineratorStates:           map[int32]float32{},
		repairTurretStates:          map[int32]repairTurretRuntimeState{},
		repairTowerStates:           map[int32]repairTowerRuntimeState{},
		teamPowerStates:             map[TeamID]*teamPowerState{},
		teamPowerBudget:             map[TeamID]float32{},
		powerNetStates:              map[int32]*powerNetState{},
		powerNetByPos:               map[int32]int32{},
		powerNetTeams:               map[int32]TeamID{},
		powerNetStorageRefs:         map[int32][]powerStorageRef{},
		powerNetDirty:               true,
		powerStorageState:           map[int32]float32{},
		powerRequested:              map[int32]float32{},
		powerSupplied:               map[int32]float32{},
		powerGeneratorState:         map[int32]*powerGeneratorState{},
		unitMountCDs:                map[int32][]float32{},
		unitMountStates:             map[int32][]unitMountState{},
		unitTargets:                 map[int32]targetTrackState{},
		unitAIStates:                map[int32]unitAIState{},
		unitMiningStates:            map[int32]unitMiningState{},
		teamItems:                   map[TeamID]map[ItemID]int32{},
		teamBuilderSpeed:            map[TeamID]float32{1: 0.5},
		itemSourceCfg:               map[int32]ItemID{},
		liquidSourceCfg:             map[int32]LiquidID{},
		sorterCfg:                   map[int32]ItemID{},
		unloaderCfg:                 map[int32]ItemID{},
		payloadRouterCfg:            map[int32]protocol.Content{},
		powerNodeLinks:              map[int32][]int32{},
		bridgeLinks:                 map[int32]int32{},
		massDriverLinks:             map[int32]int32{},
		payloadDriverLinks:          map[int32]int32{},
		bridgeBuffers:               map[int32][]bufferedBridgeItem{},
		bridgeAcceptAcc:             map[int32]float32{},
		conveyorStates:              map[int32]*conveyorRuntimeState{},
		ductStates:                  map[int32]*ductRuntimeState{},
		routerStates:                map[int32]*routerRuntimeState{},
		stackStates:                 map[int32]*stackRuntimeState{},
		massDriverStates:            map[int32]*massDriverRuntimeState{},
		payloadStates:               map[int32]*payloadRuntimeState{},
		payloadDeconstructorStates:  map[int32]*payloadDeconstructorState{},
		payloadDriverStates:         map[int32]*payloadDriverRuntimeState{},
		massDriverShots:             []massDriverShot{},
		payloadDriverShots:          []payloadDriverShot{},
		blockDumpIndex:              map[int32]int{},
		dumpNeighborCache:           map[int32][]int32{},
		unloaderLastUsed:            map[int64]int{},
		itemSourceAccum:             map[int32]float32{},
		routerInputPos:              map[int32]int32{},
		routerRotation:              map[int32]byte{},
		transportAccum:              map[int32]float32{},
		junctionQueues:              map[int32]junctionQueueState{},
		bridgeIncomingMask:          map[int32]byte{},
		reactorStates:               map[int32]nuclearReactorState{},
		storageLinkedCore:           map[int32]int32{},
		teamPrimaryCore:             map[TeamID]int32{},
		coreStorageCapacity:         map[int32]int32{},
		blockOccupancy:              map[int32]int32{},
		itemLogisticsTilePositions:  []int32{},
		itemConveyorTilePositions:   []int32{},
		itemDuctTilePositions:       []int32{},
		itemRouterTilePositions:     []int32{},
		itemBridgeTilePositions:     []int32{},
		itemUnloaderTilePositions:   []int32{},
		itemMassDriverTilePositions: []int32{},
		sandboxItemSourceTiles:      []int32{},
		sandboxLiquidSourceTiles:    []int32{},
		liquidConduitTilePositions:  []int32{},
		liquidStorageTilePositions:  []int32{},
		liquidBridgeTilePositions:   []int32{},
		payloadFactoryTilePositions: []int32{},
		payloadTransportTiles:       []int32{},
		reactorTilePositions:        []int32{},
		crafterTilePositions:        []int32{},
		drillTilePositions:          []int32{},
		burstDrillTilePositions:     []int32{},
		beamDrillTilePositions:      []int32{},
		pumpTilePositions:           []int32{},
		incineratorTilePositions:    []int32{},
		repairTurretTilePositions:   []int32{},
		repairTowerTilePositions:    []int32{},
		factoryTilePositions:        []int32{},
		heatConductorTilePositions:  []int32{},
		powerTilePositions:          []int32{},
		powerDiodeTilePositions:     []int32{},
		powerVoidTilePositions:      []int32{},
		teamBuildingTiles:           map[TeamID][]int32{},
		teamBuildingSpatial:         map[TeamID]*buildingSpatialIndex{},
		teamCoreTiles:               map[TeamID][]int32{},
		teamPowerTiles:              map[TeamID][]int32{},
		teamPowerNodeTiles:          map[TeamID][]int32{},
		turretTilePositions:         []int32{},
		unitProfilesByType:          cloneUnitWeaponProfiles(weaponProfilesByType),
		unitProfilesByName:          map[string]weaponProfile{},
		unitRuntimeProfilesByName:   map[string]unitRuntimeProfile{},
		unitMountProfilesByName:     map[string][]unitWeaponMountProfile{},
		buildingProfilesByName:      cloneBuildingWeaponProfiles(buildingWeaponProfilesByName),
		blockCostsByName:            map[string][]ItemStack{},
		blockBuildTimesByName:       map[string]float32{},
		blockArmorByName:            map[string]float32{},
		statusProfilesByID:          map[int16]statusEffectProfile{},
		statusProfilesByName:        map[string]statusEffectProfile{},
		groundPathVisitID:           1,
		rulesMgr:                    NewRulesManager(nil),
		wavesMgr:                    NewWaveManager(nil),
	}
}

func (w *World) Step(delta time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()
	stepStartedAt := time.Now()
	w.tick++
	now := time.Now()
	if w.tpsWindowStart.IsZero() {
		w.tpsWindowStart = now
	}
	w.tpsWindowTicks++
	if elapsed := now.Sub(w.tpsWindowStart); elapsed >= time.Second {
		measured := int(math.Round(float64(w.tpsWindowTicks) / elapsed.Seconds()))
		if measured <= 0 {
			measured = 1
		}
		if measured > int(w.tps) {
			measured = int(w.tps)
		}
		w.actualTps = int8(measured)
		w.tpsWindowStart = now
		w.tpsWindowTicks = 0
	}
	dt := float32(delta.Seconds())
	if dt > 0 {
		w.timeSec += dt
		rules := w.rulesMgr.Get()
		wavesEnabled := rules == nil || rules.Waves
		waveTimer := rules == nil || rules.WaveTimer
		if wavesEnabled && waveTimer {
			// Countdown-only model: initialize when empty, then decrement.
			if w.waveTime <= 0 {
				w.waveTime = w.nextWaveSpacingSec()
			}
			w.waveTime -= dt
			if w.waveTime <= 0 {
				w.triggerWave(w.wavesMgr)
				w.waveTime = w.nextWaveSpacingSec()
			}
		}
	}

	w.stepFillItemsLocked()

	pendingBuildStartedAt := time.Now()
	w.stepPendingBuilds(delta)
	pendingBuildDur := time.Since(pendingBuildStartedAt)

	pendingBreakStartedAt := time.Now()
	w.stepPendingBreaks(delta)
	pendingBreakDur := time.Since(pendingBreakStartedAt)

	sandboxStartedAt := time.Now()
	w.stepSandboxSources(delta)
	sandboxDur := time.Since(sandboxStartedAt)

	liquidStartedAt := time.Now()
	w.stepLiquidLogistics(delta)
	liquidDur := time.Since(liquidStartedAt)

	reactorStartedAt := time.Now()
	w.stepNuclearReactors(delta)
	reactorDur := time.Since(reactorStartedAt)

	w.stepOverdriveProjectorsLocked(delta)

	w.beginTeamPowerStep(delta)
	factoryStartedAt := time.Now()
	w.stepFactoryProduction(delta)
	drillParallelDur := w.stepDrillProduction(delta)
	burstParallelDur := w.stepBurstDrillProduction(delta)
	beamParallelDur := w.stepBeamDrillProduction(delta)
	pumpParallelDur := w.stepPumpProduction(delta)
	w.stepCrafterProduction(delta)
	w.stepHeatConductorsLocked()
	w.stepIncinerators(delta)
	w.stepRepairBlocks(delta)
	w.stepSupportBuildingsLocked(delta)
	factoryDur := time.Since(factoryStartedAt)

	itemStartedAt := time.Now()
	itemPerf := w.stepItemLogistics(delta, w.shouldProfileItemPerfLocked(now))
	itemDur := time.Since(itemStartedAt)

	payloadStartedAt := time.Now()
	w.stepPayloadLogistics(delta)
	payloadDur := time.Since(payloadStartedAt)

	w.stepBuildAIFillCoresLocked()
	w.stepBuildAICoreSpawnLocked(dt)
	w.stepBuildAIRefreshPathsLocked(dt)
	w.stepBuildAIPlansLocked(dt)
	w.stepPrebuildAICoreBuildersLocked()

	entitiesStartedAt := time.Now()
	entityMovementDur, entityCombatDur, buildingCombatDur, bulletDur := w.stepEntities(delta)
	entitiesDur := time.Since(entitiesStartedAt)
	w.stepBuildingBoosts(delta)
	w.endTeamPowerStep()

	totalDur := time.Since(stepStartedAt)
	if totalDur > 0 {
		estimated := int(math.Round(float64(time.Second) / float64(totalDur)))
		if estimated <= 0 {
			estimated = 1
		}
		if estimated > int(w.tps) {
			estimated = int(w.tps)
		}
		if w.actualTps <= 0 || now.Sub(w.tpsWindowStart) < time.Second {
			w.actualTps = int8(estimated)
		}
	}
	if w.shouldLogPerfLocked(totalDur) {
		entityCount := 0
		if w.model != nil {
			entityCount = len(w.model.Entities)
		}
		fmt.Printf("[perf] step=%s tps=%d/%d active=%d entities=%d bullets=%d pendingBuilds=%d pendingBreaks=%d phases{build=%s break=%s factory=%s sandbox=%s liquid=%s item=%s payload=%s reactor=%s entities=%s} itemPhases{junction=%s/%d conveyor=%s/%d duct=%s/%d router=%s/%d bridge=%s/%d unloader=%s/%d mass=%s/%d} entityPhases{move=%s combat=%s building=%s bullets=%s} parallel{drillScan=%s burstScan=%s beamScan=%s pumpScan=%s}\n",
			totalDur.Round(time.Millisecond),
			w.actualTps,
			w.tps,
			len(w.activeTilePositions),
			entityCount,
			len(w.bullets),
			len(w.pendingBuilds),
			len(w.pendingBreaks),
			pendingBuildDur.Round(time.Millisecond),
			pendingBreakDur.Round(time.Millisecond),
			factoryDur.Round(time.Millisecond),
			sandboxDur.Round(time.Millisecond),
			liquidDur.Round(time.Millisecond),
			itemDur.Round(time.Millisecond),
			payloadDur.Round(time.Millisecond),
			reactorDur.Round(time.Millisecond),
			entitiesDur.Round(time.Millisecond),
			itemPerf.Junctions.Round(time.Millisecond),
			itemPerf.JunctionCount,
			itemPerf.Conveyor.Round(time.Millisecond),
			itemPerf.ConveyorCount,
			itemPerf.Duct.Round(time.Millisecond),
			itemPerf.DuctCount,
			itemPerf.Router.Round(time.Millisecond),
			itemPerf.RouterCount,
			itemPerf.Bridge.Round(time.Millisecond),
			itemPerf.BridgeCount,
			itemPerf.Unloader.Round(time.Millisecond),
			itemPerf.UnloaderCount,
			itemPerf.MassDrive.Round(time.Millisecond),
			itemPerf.MassDriveCount,
			entityMovementDur.Round(time.Millisecond),
			entityCombatDur.Round(time.Millisecond),
			buildingCombatDur.Round(time.Millisecond),
			bulletDur.Round(time.Millisecond),
			drillParallelDur.Round(time.Millisecond),
			burstParallelDur.Round(time.Millisecond),
			beamParallelDur.Round(time.Millisecond),
			pumpParallelDur.Round(time.Millisecond),
		)
		w.perfLogAt = time.Now()
	}
}

func (w *World) shouldLogPerfLocked(totalDur time.Duration) bool {
	now := time.Now()
	if !w.perfLogAt.IsZero() && now.Sub(w.perfLogAt) < 2*time.Second {
		return false
	}
	if totalDur >= 50*time.Millisecond {
		return true
	}
	return w.actualTps > 0 && w.actualTps <= int8(max(20, int(w.tps)/2))
}

func (w *World) shouldProfileItemPerfLocked(now time.Time) bool {
	return w.perfLogAt.IsZero() || now.Sub(w.perfLogAt) >= 2*time.Second
}

func (w *World) ConfigureItemSource(pos int32, item ItemID) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.model == nil || pos < 0 || int(pos) >= len(w.model.Tiles) {
		return
	}
	w.applyBuildingConfigLocked(pos, protocol.ItemRef{ItmID: int16(item)}, true)
}

func (w *World) ConfigureLiquidSource(pos int32, liquid LiquidID) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.model == nil || pos < 0 || int(pos) >= len(w.model.Tiles) {
		return
	}
	w.applyBuildingConfigLocked(pos, protocolContentLiquid(liquid), true)
}

func (w *World) ConfigureSorter(pos int32, item ItemID) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.model == nil || pos < 0 || int(pos) >= len(w.model.Tiles) {
		return
	}
	w.applyBuildingConfigLocked(pos, protocol.ItemRef{ItmID: int16(item)}, true)
}

func (w *World) ConfigureUnloader(pos int32, item ItemID) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.model == nil || pos < 0 || int(pos) >= len(w.model.Tiles) {
		return
	}
	w.applyBuildingConfigLocked(pos, protocol.ItemRef{ItmID: int16(item)}, true)
}

func (w *World) ConfigureBuilding(pos int32, value any) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.model == nil || pos < 0 || int(pos) >= len(w.model.Tiles) {
		return
	}
	w.applyBuildingConfigLocked(pos, value, true)
}

func (w *World) ConfigureBuildingPacked(pos int32, value any) {
	w.mu.Lock()
	defer w.mu.Unlock()
	index, ok := w.tileIndexFromPackedPosLocked(pos)
	if !ok {
		return
	}
	w.applyBuildingConfigLocked(index, value, true)
}

func (w *World) BuildingInfoPacked(pos int32) (BuildingInfo, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	index, ok := w.buildingIndexFromPackedPosLocked(pos)
	if !ok || w.model == nil || index < 0 || int(index) >= len(w.model.Tiles) {
		return BuildingInfo{}, false
	}
	tile := &w.model.Tiles[index]
	if tile.Block == 0 || tile.Build == nil {
		return BuildingInfo{}, false
	}

	team := tile.Team
	if tile.Build != nil {
		team = tile.Build.Team
	}
	return BuildingInfo{
		Pos:      packTilePos(tile.X, tile.Y),
		X:        int32(tile.X),
		Y:        int32(tile.Y),
		BlockID:  int16(tile.Block),
		Name:     w.blockNameByID(int16(tile.Block)),
		Team:     team,
		Rotation: tile.Rotation,
	}, true
}

func (w *World) BlockSyncSuppressedPacked(packedPos int32) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	index, ok := w.buildingIndexFromPackedPosLocked(packedPos)
	if !ok {
		return false
	}
	return w.blockSyncSuppressedLocked(index)
}

func (w *World) RotateBuildingPacked(pos int32, direction bool) (RotateBuildingResult, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()

	index, ok := w.buildingIndexFromPackedPosLocked(pos)
	if !ok || w.model == nil || index < 0 || int(index) >= len(w.model.Tiles) {
		return RotateBuildingResult{}, false
	}
	tile := &w.model.Tiles[index]
	if tile.Block == 0 || tile.Build == nil {
		return RotateBuildingResult{}, false
	}

	step := -1
	if direction {
		step = 1
	}
	nextRotation := int8((tileRotationNorm(tile.Rotation) + step + 4) % 4)
	tile.Rotation = nextRotation
	tile.Build.Rotation = nextRotation
	if tile.Build.Team != 0 {
		tile.Team = tile.Build.Team
	}
	if w.isPowerRelevantBuildingLocked(tile) {
		w.invalidatePowerNetsLocked()
	}

	return RotateBuildingResult{
		BlockID:   int16(tile.Block),
		Rotation:  nextRotation,
		Team:      tile.Team,
		EffectX:   float32(tile.X*8 + 4),
		EffectY:   float32(tile.Y*8 + 4),
		EffectRot: float32(w.blockSizeForTileLocked(tile)),
	}, true
}

func (w *World) CommandBuildingsPacked(positions []int32, target protocol.Vec2) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.model == nil || len(positions) == 0 {
		return
	}
	for _, packed := range positions {
		index, ok := w.tileIndexFromPackedPosLocked(packed)
		if !ok {
			continue
		}
		if w.unitFactoryConfigBlockAtLocked(index) {
			w.configureUnitFactoryCommandPosLocked(index, target)
			continue
		}
		if isReconstructorBlockName(w.blockNameByID(int16(w.model.Tiles[index].Block))) {
			w.configureReconstructorCommandPosLocked(index, target)
		}
	}
}

func (w *World) configureItemContentLocked(pos int32, item ItemID) {
	if w.model == nil || pos < 0 || int(pos) >= len(w.model.Tiles) {
		return
	}
	switch w.blockNameByID(int16(w.model.Tiles[pos].Block)) {
	case "item-source":
		w.itemSourceCfg[pos] = item
		delete(w.liquidSourceCfg, pos)
	case "sorter", "inverted-sorter", "duct-router", "surge-router":
		w.sorterCfg[pos] = item
	case "unloader", "duct-unloader":
		w.unloaderCfg[pos] = item
	}
}

func (w *World) configurePayloadContentLocked(pos int32, content protocol.Content) bool {
	if w.model == nil || pos < 0 || int(pos) >= len(w.model.Tiles) || content == nil {
		return false
	}
	switch w.blockNameByID(int16(w.model.Tiles[pos].Block)) {
	case "payload-router", "reinforced-payload-router":
		switch content.ContentType() {
		case protocol.ContentBlock, protocol.ContentUnit:
			w.payloadRouterCfg[pos] = content
			return true
		}
	}
	return false
}

func (w *World) configureLiquidContentLocked(pos int32, liquid LiquidID) {
	if w.model == nil || pos < 0 || int(pos) >= len(w.model.Tiles) {
		return
	}
	switch w.blockNameByID(int16(w.model.Tiles[pos].Block)) {
	case "liquid-source":
		w.liquidSourceCfg[pos] = liquid
		delete(w.itemSourceCfg, pos)
	}
}

func (w *World) itemConfigBlockAtLocked(pos int32) bool {
	if w.model == nil || pos < 0 || int(pos) >= len(w.model.Tiles) {
		return false
	}
	switch w.blockNameByID(int16(w.model.Tiles[pos].Block)) {
	case "item-source", "sorter", "inverted-sorter", "duct-router", "surge-router", "unloader", "duct-unloader":
		return true
	default:
		return false
	}
}

func (w *World) unitFactoryConfigBlockAtLocked(pos int32) bool {
	if w.model == nil || pos < 0 || int(pos) >= len(w.model.Tiles) {
		return false
	}
	switch w.blockNameByID(int16(w.model.Tiles[pos].Block)) {
	case "ground-factory", "air-factory", "naval-factory":
		return true
	default:
		return false
	}
}

func (w *World) ClearBuildingConfig(pos int32) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.unitFactoryConfigBlockAtLocked(pos) {
		_ = w.clearUnitFactoryCommandLocked(pos)
		return
	}
	if w.model != nil && pos >= 0 && int(pos) < len(w.model.Tiles) && isReconstructorBlockName(w.blockNameByID(int16(w.model.Tiles[pos].Block))) {
		_ = w.clearReconstructorCommandLocked(pos)
		return
	}
	w.clearConfiguredStateLocked(pos)
}

func (w *World) BuildingConfigPacked(pos int32) (any, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	index, ok := w.tileIndexFromPackedPosLocked(pos)
	if !ok {
		return nil, false
	}
	if value, ok := w.normalizedBuildingConfigLocked(index); ok {
		return cloneStoredBuildingConfigValue(value)
	}
	if w.model == nil || index < 0 || int(index) >= len(w.model.Tiles) {
		return nil, false
	}
	tile := &w.model.Tiles[index]
	if tile.Build == nil || len(tile.Build.Config) == 0 {
		return nil, false
	}
	return decodeStoredBuildingConfig(tile.Build.Config)
}

func (w *World) clearBuildingRuntimeLocked(pos int32) {
	w.invalidateItemRoutingCachesLocked()
	w.clearControlledBuildingLocked(pos)
	w.clearConfiguredStateLocked(pos)
	delete(w.bridgeBuffers, pos)
	delete(w.bridgeAcceptAcc, pos)
	delete(w.conveyorStates, pos)
	delete(w.ductStates, pos)
	delete(w.routerStates, pos)
	delete(w.stackStates, pos)
	delete(w.massDriverStates, pos)
	delete(w.payloadStates, pos)
	delete(w.payloadDeconstructorStates, pos)
	delete(w.payloadDriverStates, pos)
	delete(w.blockDumpIndex, pos)
	delete(w.itemSourceAccum, pos)
	delete(w.routerInputPos, pos)
	delete(w.routerRotation, pos)
	delete(w.transportAccum, pos)
	delete(w.junctionQueues, pos)
	delete(w.reactorStates, pos)
	delete(w.drillStates, pos)
	delete(w.burstDrillStates, pos)
	delete(w.beamDrillStates, pos)
	delete(w.pumpStates, pos)
	delete(w.crafterStates, pos)
	delete(w.reconstructorStates, pos)
	delete(w.heatStates, pos)
	delete(w.incineratorStates, pos)
	delete(w.repairTurretStates, pos)
	delete(w.repairTowerStates, pos)
	delete(w.turretStates, pos)
	delete(w.mendProjectorStates, pos)
	delete(w.overdriveProjectorStates, pos)
	delete(w.buildingBoostStates, pos)
	delete(w.forceProjectorStates, pos)
	delete(w.powerStorageState, pos)
	delete(w.powerGeneratorState, pos)
}

func (w *World) clearConfiguredStateLocked(pos int32) {
	w.invalidateItemRoutingCachesLocked()
	if w.model != nil && pos >= 0 && int(pos) < len(w.model.Tiles) {
		switch w.blockNameByID(int16(w.model.Tiles[pos].Block)) {
		case "power-node", "power-node-large", "surge-tower", "beam-link", "power-source":
			w.clearPowerLinksForBuildingLocked(pos)
		}
	}
	delete(w.itemSourceCfg, pos)
	delete(w.liquidSourceCfg, pos)
	delete(w.sorterCfg, pos)
	delete(w.unloaderCfg, pos)
	delete(w.payloadRouterCfg, pos)
	delete(w.bridgeLinks, pos)
	delete(w.massDriverLinks, pos)
	delete(w.payloadDriverLinks, pos)
	if w.model != nil && pos >= 0 && int(pos) < len(w.model.Tiles) {
		if tile := &w.model.Tiles[pos]; tile.Build != nil {
			tile.Build.Config = nil
		}
	}
}

func (w *World) tileIndexFromPackedPosLocked(pos int32) (int32, bool) {
	if w.model == nil {
		return 0, false
	}
	x := int(protocol.UnpackPoint2X(pos))
	y := int(protocol.UnpackPoint2Y(pos))
	if !w.model.InBounds(x, y) {
		return 0, false
	}
	return int32(y*w.model.Width + x), true
}

func (w *World) centerBuildingIndexLocked(pos int32) (int32, bool) {
	if w.model == nil || pos < 0 || int(pos) >= len(w.model.Tiles) {
		return 0, false
	}
	tile := &w.model.Tiles[pos]
	if isCenterBuildingTile(tile) {
		return pos, true
	}
	if tile.Build == nil {
		return 0, false
	}
	cx := tile.Build.X
	cy := tile.Build.Y
	if !w.model.InBounds(cx, cy) {
		return 0, false
	}
	centerPos := int32(cy*w.model.Width + cx)
	if centerPos < 0 || int(centerPos) >= len(w.model.Tiles) {
		return 0, false
	}
	if !isCenterBuildingTile(&w.model.Tiles[centerPos]) {
		return 0, false
	}
	return centerPos, true
}

func (w *World) buildingIndexFromPackedPosLocked(pos int32) (int32, bool) {
	if w.model == nil {
		return 0, false
	}
	x := int(protocol.UnpackPoint2X(pos))
	y := int(protocol.UnpackPoint2Y(pos))
	if !w.model.InBounds(x, y) {
		return 0, false
	}
	if centerPos, ok := w.blockOccupancy[packTilePos(x, y)]; ok {
		return w.centerBuildingIndexLocked(centerPos)
	}
	return w.centerBuildingIndexLocked(int32(y*w.model.Width + x))
}

func (w *World) applyBuildingConfigLocked(pos int32, value any, persist bool) {
	if w.model == nil || pos < 0 || int(pos) >= len(w.model.Tiles) {
		return
	}
	tile := &w.model.Tiles[pos]
	if tile.Build == nil || tile.Block == 0 {
		return
	}

	if value == nil {
		if w.unitFactoryConfigBlockAtLocked(pos) {
			w.clearUnitFactoryCommandLocked(pos)
			if persist {
				if normalized, ok := w.normalizedBuildingConfigLocked(pos); ok {
					w.storeBuildingConfigLocked(tile, normalized)
				} else {
					tile.Build.Config = nil
				}
			}
			return
		}
		if isReconstructorBlockName(w.blockNameByID(int16(tile.Block))) {
			w.clearReconstructorCommandLocked(pos)
			if persist {
				if normalized, ok := w.normalizedBuildingConfigLocked(pos); ok {
					w.storeBuildingConfigLocked(tile, normalized)
				} else {
					tile.Build.Config = nil
				}
			}
			return
		}
		w.clearConfiguredStateLocked(pos)
		if persist {
			tile.Build.Config = nil
		}
		return
	}

	applied := false
	switch v := value.(type) {
	case protocol.Content:
		switch v.ContentType() {
		case protocol.ContentItem:
			w.configureItemContentLocked(pos, ItemID(v.ID()))
			applied = true
		case protocol.ContentLiquid:
			w.configureLiquidContentLocked(pos, LiquidID(v.ID()))
			applied = true
		case protocol.ContentBlock:
			applied = w.configurePayloadContentLocked(pos, v)
		case protocol.ContentUnit:
			if w.unitFactoryConfigBlockAtLocked(pos) {
				applied = w.configureUnitFactoryUnitLocked(pos, v.ID())
			} else {
				applied = w.configurePayloadContentLocked(pos, v)
			}
		}
	case protocol.Point2:
		applied = w.configurePointConfigLocked(pos, v)
	case []protocol.Point2:
		applied = w.configurePointSeqConfigLocked(pos, v)
	case protocol.UnitCommand:
		if w.unitFactoryConfigBlockAtLocked(pos) {
			applied = w.configureUnitFactoryCommandLocked(pos, &v)
		} else if isReconstructorBlockName(w.blockNameByID(int16(tile.Block))) {
			applied = w.configureReconstructorCommandLocked(pos, &v)
		}
	case int32:
		if w.itemConfigBlockAtLocked(pos) {
			w.configureItemContentLocked(pos, ItemID(v))
			applied = true
		} else if w.unitFactoryConfigBlockAtLocked(pos) {
			applied = w.configureUnitFactoryPlanLocked(pos, int16(v))
		} else {
			applied = w.configureAbsoluteLinkLocked(pos, v)
		}
	case int:
		if w.itemConfigBlockAtLocked(pos) {
			w.configureItemContentLocked(pos, ItemID(v))
			applied = true
		} else if w.unitFactoryConfigBlockAtLocked(pos) {
			applied = w.configureUnitFactoryPlanLocked(pos, int16(v))
		} else {
			applied = w.configureAbsoluteLinkLocked(pos, int32(v))
		}
	case int16:
		if w.itemConfigBlockAtLocked(pos) {
			w.configureItemContentLocked(pos, ItemID(v))
			applied = true
		} else if w.unitFactoryConfigBlockAtLocked(pos) {
			applied = w.configureUnitFactoryPlanLocked(pos, v)
		} else {
			applied = w.configureAbsoluteLinkLocked(pos, int32(v))
		}
	}

	if persist && applied {
		if normalized, ok := w.normalizedBuildingConfigLocked(pos); ok {
			w.storeBuildingConfigLocked(tile, normalized)
		} else {
			w.storeBuildingConfigLocked(tile, value)
		}
	}
	if persist && applied && tile.Build != nil {
		tile.Build.MapSyncData = nil
		tile.Build.MapSyncTail = nil
		tile.Build.MapPowerLinks = nil
		tile.Build.MapPowerStatus = 0
		tile.Build.MapPowerStatusSet = false
	}
}

func (w *World) configurePointConfigLocked(pos int32, p protocol.Point2) bool {
	if w.model == nil || pos < 0 || int(pos) >= len(w.model.Tiles) {
		return false
	}
	tile := &w.model.Tiles[pos]
	switch w.blockNameByID(int16(tile.Block)) {
	case "power-node", "power-node-large", "surge-tower", "beam-link", "power-source":
		targetX := tile.X + int(p.X)
		targetY := tile.Y + int(p.Y)
		if !w.model.InBounds(targetX, targetY) {
			return false
		}
		return w.configureAbsoluteLinkLocked(pos, int32(targetY*w.model.Width+targetX))
	case "bridge-conveyor", "phase-conveyor", "bridge-conduit", "phase-conduit", "mass-driver", "payload-mass-driver", "large-payload-mass-driver":
		targetX := tile.X + int(p.X)
		targetY := tile.Y + int(p.Y)
		if !w.model.InBounds(targetX, targetY) {
			return false
		}
		return w.configureAbsoluteLinkLocked(pos, int32(targetY*w.model.Width+targetX))
	default:
		return false
	}
}

func (w *World) configurePointSeqConfigLocked(pos int32, points []protocol.Point2) bool {
	if w.model == nil || pos < 0 || int(pos) >= len(w.model.Tiles) {
		return false
	}
	tile := &w.model.Tiles[pos]
	switch w.blockNameByID(int16(tile.Block)) {
	case "power-node", "power-node-large", "surge-tower", "beam-link", "power-source":
		targets := make([]int32, 0, len(points))
		for _, p := range points {
			targetX := tile.X + int(p.X)
			targetY := tile.Y + int(p.Y)
			if !w.model.InBounds(targetX, targetY) {
				continue
			}
			targets = append(targets, int32(targetY*w.model.Width+targetX))
		}
		return w.configurePowerNodeLinksLocked(pos, targets)
	default:
		return false
	}
}

func (w *World) configureAbsoluteLinkLocked(pos, target int32) bool {
	if w.model == nil || pos < 0 || int(pos) >= len(w.model.Tiles) {
		return false
	}
	if target < 0 {
		if w.model != nil && pos >= 0 && int(pos) < len(w.model.Tiles) {
			switch w.blockNameByID(int16(w.model.Tiles[pos].Block)) {
			case "power-node", "power-node-large", "surge-tower", "beam-link", "power-source":
				w.clearPowerLinksForBuildingLocked(pos)
			case "bridge-conveyor", "phase-conveyor":
				delete(w.bridgeLinks, pos)
				w.invalidateItemRoutingCachesLocked()
			case "mass-driver":
				delete(w.massDriverLinks, pos)
			case "payload-mass-driver", "large-payload-mass-driver":
				delete(w.payloadDriverLinks, pos)
			}
		}
		return true
	}
	tile := &w.model.Tiles[pos]
	name := w.blockNameByID(int16(tile.Block))
	if name == "power-node" || name == "power-node-large" || name == "surge-tower" || name == "beam-link" || name == "power-source" {
		return w.togglePowerNodeLinkLocked(pos, target)
	}
	if name != "bridge-conveyor" && name != "phase-conveyor" && name != "bridge-conduit" && name != "phase-conduit" && name != "mass-driver" && name != "payload-mass-driver" && name != "large-payload-mass-driver" {
		return false
	}
	targetPos, ok := w.resolveAbsoluteLinkTargetLocked(target)
	if !ok {
		return false
	}
	targetTile := &w.model.Tiles[targetPos]
	dx := targetTile.X - tile.X
	dy := targetTile.Y - tile.Y
	if dx != 0 && dy != 0 {
		return false
	}
	if dx == 0 && dy == 0 {
		return false
	}
	rangeLimit := 4
	switch name {
	case "phase-conveyor":
		rangeLimit = 12
	case "mass-driver":
		rangeLimit = 55
	case "payload-mass-driver":
		rangeLimit = 87
	case "large-payload-mass-driver":
		rangeLimit = 262
	}
	if absInt(dx) > rangeLimit || absInt(dy) > rangeLimit {
		return false
	}
	switch name {
	case "bridge-conveyor", "phase-conveyor", "bridge-conduit", "phase-conduit":
		targetName := w.blockNameByID(int16(targetTile.Block))
		if targetName != name {
			return false
		}
		w.bridgeLinks[pos] = targetPos
		w.invalidateItemRoutingCachesLocked()
	case "mass-driver":
		if w.blockNameByID(int16(targetTile.Block)) != "mass-driver" {
			return false
		}
		w.massDriverLinks[pos] = targetPos
	case "payload-mass-driver", "large-payload-mass-driver":
		if w.blockNameByID(int16(targetTile.Block)) != name {
			return false
		}
		w.payloadDriverLinks[pos] = targetPos
	}
	return true
}

func (w *World) resolveAbsoluteLinkTargetLocked(target int32) (int32, bool) {
	if w.model == nil {
		return 0, false
	}
	tx := protocol.UnpackPoint2X(target)
	ty := protocol.UnpackPoint2Y(target)
	if w.model.InBounds(int(tx), int(ty)) {
		return int32(int(ty)*w.model.Width + int(tx)), true
	}
	if target >= 0 && int(target) < len(w.model.Tiles) {
		return target, true
	}
	return 0, false
}

func (w *World) normalizedBuildingConfigLocked(pos int32) (any, bool) {
	if w.model == nil || pos < 0 || int(pos) >= len(w.model.Tiles) {
		return nil, false
	}
	tile := &w.model.Tiles[pos]
	if tile.Build == nil || tile.Block == 0 {
		return nil, false
	}
	switch w.blockNameByID(int16(tile.Block)) {
	case "item-source":
		item, ok := w.itemSourceCfg[pos]
		if !ok {
			return nil, false
		}
		return protocol.ItemRef{ItmID: int16(item)}, true
	case "liquid-source":
		liquid, ok := w.liquidSourceCfg[pos]
		if !ok {
			return nil, false
		}
		return protocolContentLiquid(liquid), true
	case "sorter", "inverted-sorter", "duct-router", "surge-router", "unloader", "duct-unloader":
		item, ok := w.sorterCfg[pos]
		switch w.blockNameByID(int16(tile.Block)) {
		case "unloader", "duct-unloader":
			item, ok = w.unloaderCfg[pos]
		}
		if !ok {
			return nil, false
		}
		return protocol.ItemRef{ItmID: int16(item)}, true
	case "payload-router", "reinforced-payload-router":
		filter, ok := w.payloadRouterCfg[pos]
		if !ok || filter == nil {
			return nil, false
		}
		return filter, true
	case "ground-factory", "air-factory", "naval-factory":
		value, ok := w.unitFactoryConfigValueLocked(pos, tile)
		if !ok {
			return nil, false
		}
		return value, true
	case "additive-reconstructor", "multiplicative-reconstructor", "exponential-reconstructor", "tetrative-reconstructor",
		"tank-refabricator", "ship-refabricator", "mech-refabricator", "prime-refabricator":
		_, command := w.reconstructorCommandStateLocked(pos)
		if command == nil {
			return nil, false
		}
		return *command, true
	case "power-node", "power-node-large", "surge-tower", "beam-link", "power-source":
		links := w.powerNodeLinks[pos]
		if len(links) == 0 {
			return nil, false
		}
		out := make([]protocol.Point2, 0, len(links))
		for _, target := range links {
			if target < 0 || int(target) >= len(w.model.Tiles) {
				continue
			}
			targetTile := &w.model.Tiles[target]
			out = append(out, protocol.Point2{X: int32(targetTile.X - tile.X), Y: int32(targetTile.Y - tile.Y)})
		}
		if len(out) == 0 {
			return nil, false
		}
		sort.Slice(out, func(i, j int) bool {
			if out[i].X == out[j].X {
				return out[i].Y < out[j].Y
			}
			return out[i].X < out[j].X
		})
		return out, true
	case "bridge-conveyor", "phase-conveyor", "bridge-conduit", "phase-conduit":
		target, ok := w.bridgeLinks[pos]
		if !ok || target < 0 || int(target) >= len(w.model.Tiles) {
			return nil, false
		}
		targetTile := &w.model.Tiles[target]
		return protocol.Point2{X: int32(targetTile.X - tile.X), Y: int32(targetTile.Y - tile.Y)}, true
	case "mass-driver":
		target, ok := w.massDriverLinks[pos]
		if !ok || target < 0 || int(target) >= len(w.model.Tiles) {
			return nil, false
		}
		targetTile := &w.model.Tiles[target]
		return protocol.Point2{X: int32(targetTile.X - tile.X), Y: int32(targetTile.Y - tile.Y)}, true
	case "payload-mass-driver", "large-payload-mass-driver":
		target, ok := w.payloadDriverLinks[pos]
		if !ok || target < 0 || int(target) >= len(w.model.Tiles) {
			return nil, false
		}
		targetTile := &w.model.Tiles[target]
		return protocol.Point2{X: int32(targetTile.X - tile.X), Y: int32(targetTile.Y - tile.Y)}, true
	default:
		return nil, false
	}
}

func (w *World) storeBuildingConfigLocked(tile *Tile, value any) {
	if tile == nil || tile.Build == nil {
		return
	}
	writer := protocol.NewWriter()
	normalized := value
	switch v := value.(type) {
	case int:
		normalized = int32(v)
	}
	if err := protocol.WriteObject(writer, normalized, nil); err != nil {
		return
	}
	tile.Build.Config = append(tile.Build.Config[:0], writer.Bytes()...)
}

func (w *World) restoreTileConfigsLocked() {
	if w.model == nil {
		return
	}
	for i := range w.model.Tiles {
		tile := &w.model.Tiles[i]
		if tile.Build == nil || len(tile.Build.Config) == 0 {
			continue
		}
		value, ok := decodeStoredBuildingConfig(tile.Build.Config)
		if !ok {
			continue
		}
		w.applyBuildingConfigLocked(int32(i), value, false)
	}
}

func decodeStoredBuildingConfig(data []byte) (any, bool) {
	if len(data) == 0 {
		return nil, false
	}
	value, err := protocol.ReadObject(protocol.NewReader(data), false, nil)
	if err != nil {
		return nil, false
	}
	return value, true
}

func cloneStoredBuildingConfigValue(value any) (any, bool) {
	switch v := value.(type) {
	case protocol.ItemRef:
		return v, true
	case protocol.BlockRef:
		return v, true
	case protocol.Point2:
		return v, true
	case []protocol.Point2:
		out := append([]protocol.Point2(nil), v...)
		return out, true
	case protocol.UnitCommand:
		return v, true
	case int32:
		return v, true
	case int16:
		return v, true
	case int:
		return v, true
	case bool:
		return v, true
	case float64:
		return v, true
	case string:
		return v, true
	case []byte:
		out := append([]byte(nil), v...)
		return out, true
	}
	writer := protocol.NewWriter()
	if err := protocol.WriteObject(writer, value, nil); err != nil {
		return nil, false
	}
	return decodeStoredBuildingConfig(writer.Bytes())
}

func (w *World) restorePayloadStatesLocked() {
	if w.model == nil {
		return
	}
	for i := range w.model.Tiles {
		tile := &w.model.Tiles[i]
		if tile.Build == nil || len(tile.Build.Payload) == 0 {
			continue
		}
		payload, ok := decodePayloadData(tile.Build.Payload)
		if !ok {
			continue
		}
		w.payloadStates[int32(i)] = &payloadRuntimeState{Payload: payload}
	}
}

func decodePayloadData(data []byte) (out *payloadData, ok bool) {
	if len(data) == 0 {
		return nil, false
	}
	defer func() {
		if recover() != nil {
			out = nil
			ok = false
		}
	}()
	decoded, err := protocol.ReadPayload(protocol.NewReader(data), nil)
	if err != nil {
		return nil, false
	}
	out = &payloadData{
		UnitTypeID: -1,
		Serialized: append([]byte(nil), data...),
	}
	switch v := decoded.(type) {
	case protocol.BuildPayload:
		out.Kind = payloadKindBlock
		out.BlockID = v.BlockID
		return out, true
	case protocol.UnitPayload:
		out.Kind = payloadKindUnit
		if entity, ok := decodeRawUnitPayloadEntity(v.Raw, v.ClassID); ok && entity != nil {
			out.UnitTypeID = entity.TypeID
			out.Rotation = buildRotationFromDegrees(entity.Rotation)
			out.Health = entity.Health
			out.MaxHealth = entity.MaxHealth
			clone := cloneRawEntity(*entity)
			out.UnitState = &clone
		}
		return out, true
	case protocol.PayloadBox:
		if len(v.Raw) == 0 {
			return nil, false
		}
		switch v.Raw[0] {
		case protocol.PayloadBlock:
			out.Kind = payloadKindBlock
			if len(v.Raw) >= 4 {
				out.BlockID = int16(uint16(v.Raw[1])<<8 | uint16(v.Raw[2]))
			}
			return out, true
		case protocol.PayloadUnit:
			out.Kind = payloadKindUnit
			if len(v.Raw) >= 3 {
				if entity, ok := decodeRawUnitPayloadEntity(v.Raw[2:], v.Raw[1]); ok && entity != nil {
					out.UnitTypeID = entity.TypeID
					out.Rotation = buildRotationFromDegrees(entity.Rotation)
					out.Health = entity.Health
					out.MaxHealth = entity.MaxHealth
					clone := cloneRawEntity(*entity)
					out.UnitState = &clone
				}
			}
			return out, true
		}
	}
	return nil, false
}

func (w *World) BuildingConfig(pos int32) (any, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.model == nil || pos < 0 || int(pos) >= len(w.model.Tiles) {
		return nil, false
	}
	if value, ok := w.normalizedBuildingConfigLocked(pos); ok {
		return value, true
	}
	tile := &w.model.Tiles[pos]
	if tile.Build == nil || len(tile.Build.Config) == 0 {
		return nil, false
	}
	return decodeStoredBuildingConfig(tile.Build.Config)
}

func (w *World) stepSandboxSources(delta time.Duration) {
	if w.model == nil {
		return
	}
	frameDelta := float32(delta.Seconds() * 60)
	liquidRate := frameDelta
	if liquidRate < 1 {
		liquidRate = 1
	}
	for _, pos := range w.sandboxItemSourceTiles {
		if pos < 0 || int(pos) >= len(w.model.Tiles) {
			continue
		}
		tile := &w.model.Tiles[pos]
		if tile.Build == nil || tile.Block == 0 {
			continue
		}
		if item, ok := w.itemSourceCfg[pos]; ok {
			w.pushItemSourceLocked(pos, tile, item, frameDelta)
		}
	}
	for _, pos := range w.sandboxLiquidSourceTiles {
		if pos < 0 || int(pos) >= len(w.model.Tiles) {
			continue
		}
		tile := &w.model.Tiles[pos]
		if tile.Build == nil || tile.Block == 0 {
			continue
		}
		if liquid, ok := w.liquidSourceCfg[pos]; ok {
			w.pushLiquidSourceLocked(pos, tile, liquid, liquidRate)
		}
	}
}

func (w *World) pushItemSourceLocked(pos int32, tile *Tile, item ItemID, frameDelta float32) {
	if tile == nil || tile.Build == nil || w.model == nil || frameDelta <= 0 {
		return
	}
	const itemsPerSecond = float32(100)
	limit := float32(60) / itemsPerSecond
	w.itemSourceAccum[pos] += frameDelta
	for w.itemSourceAccum[pos] >= limit {
		w.dumpGeneratedItemLocked(pos, tile, item)
		w.itemSourceAccum[pos] -= limit
	}
}

func (w *World) dumpGeneratedItemLocked(pos int32, tile *Tile, item ItemID) bool {
	if tile == nil || tile.Build == nil || w.model == nil {
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
		if w.tryInsertItemLocked(pos, neighbors[index], item, 0) {
			w.advanceDumpIndexLocked(pos, index+1, len(neighbors))
			return true
		}
		w.advanceDumpIndexLocked(pos, index+1, len(neighbors))
	}
	return false
}

func (w *World) pushLiquidSourceLocked(pos int32, tile *Tile, liquid LiquidID, amount float32) {
	if tile == nil || tile.Build == nil || w.model == nil || amount <= 0 {
		return
	}
	for _, off := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		nx, ny := tile.X+off[0], tile.Y+off[1]
		if !w.model.InBounds(nx, ny) {
			continue
		}
		other := &w.model.Tiles[ny*w.model.Width+nx]
		if other.Build == nil || other.Team != tile.Team || other.Block == 0 {
			continue
		}
		if moved := w.tryMoveLiquidLocked(pos, int32(ny*w.model.Width+nx), liquid, amount, 0); moved > 0 {
			break
		}
	}
}

func (w *World) itemCapacityForBlockLocked(tile *Tile) int32 {
	if tile == nil || tile.Block == 0 {
		return 0
	}
	name := w.blockNameByID(int16(tile.Block))
	switch name {
	case "conveyor", "titanium-conveyor", "armored-conveyor":
		return 3
	case "duct", "armored-duct", "duct-router", "overflow-duct", "underflow-duct":
		return 1
	case "combustion-generator", "steam-generator", "differential-generator", "rtg-generator":
		return 10
	case "graphite-press", "silicon-smelter", "kiln", "plastanium-compressor", "cryofluid-mixer", "pyratite-mixer", "blast-mixer", "separator", "pulverizer", "coal-centrifuge", "spore-press", "cultivator", "oxidation-chamber", "phase-heater":
		return 10
	case "neoplasia-reactor":
		return 10
	case "mechanical-drill", "pneumatic-drill", "laser-drill":
		return 10
	case "blast-drill":
		return 20
	case "plasma-bore":
		return 10
	case "large-plasma-bore":
		return 20
	case "impact-drill":
		return 40
	case "eruption-drill":
		return 60
	case "oil-extractor", "melter", "disassembler", "slag-centrifuge":
		return 10
	case "multi-press", "surge-smelter", "carbide-crucible", "surge-crucible", "heat-reactor":
		return 20
	case "phase-weaver", "silicon-crucible", "silicon-arc-furnace":
		return 30
	case "phase-synthesizer":
		return 40
	case "ground-factory", "air-factory", "naval-factory":
		return unitFactoryScaledAmount(unitFactoryTotalItemCapacity(name), w.unitCostMultiplierLocked(tile.Team))
	case "small-deconstructor", "deconstructor", "payload-deconstructor":
		if prof, ok := payloadDeconstructorProfileByName(name); ok {
			return prof.ItemCapacity
		}
		return 0
	case "core-shard":
		return 4000
	case "core-foundation":
		return 9000
	case "core-nucleus":
		return 13000
	case "core-bastion":
		return 2000
	case "core-citadel":
		return 3000
	case "core-acropolis":
		return 4000
	case "container":
		return 300
	case "vault":
		return 1000
	case "reinforced-container":
		return 160
	case "reinforced-vault":
		return 900
	case "duct-bridge":
		return 4
	case "plastanium-conveyor", "surge-conveyor", "surge-router":
		return 10
	case "router", "distributor":
		return 1
	case "bridge-conveyor", "phase-conveyor":
		return 10
	case "mass-driver":
		return 120
	case "payload-loader", "payload-unloader":
		return 100
	case "thorium-reactor":
		return 30
	default:
		if isReconstructorBlockName(name) {
			return w.reconstructorItemCapacityLocked(tile)
		}
		return 0
	}
}

func (w *World) liquidCapacityForBlockLocked(tile *Tile) float32 {
	if tile == nil || tile.Block == 0 {
		return 0
	}
	name := w.blockNameByID(int16(tile.Block))
	switch name {
	case "conduit":
		return 20
	case "pulse-conduit":
		return 40
	case "plated-conduit", "reinforced-conduit":
		return 50
	case "multi-press", "plastanium-compressor", "coal-centrifuge", "spore-press":
		return 60
	case "thorium-reactor":
		return 30
	case "mechanical-pump":
		return 20
	case "rotary-pump":
		return 80
	case "impulse-pump":
		return 200
	case "water-extractor", "oil-extractor":
		return 40
	case "cryofluid-mixer":
		return 36
	case "electrolyzer":
		return 50
	case "melter":
		return 10
	case "slag-incinerator":
		return 10
	case "separator":
		return 40
	case "slag-centrifuge":
		return 80
	case "repair-turret":
		return 96
	case "unit-repair-tower":
		return 30
	case "plasma-bore":
		return 10
	case "large-plasma-bore":
		return 30
	case "impact-drill":
		return 100
	case "eruption-drill":
		return 40
	case "heat-reactor":
		return 10
	case "surge-crucible":
		return 80 * 5
	case "slag-heater":
		return 120
	case "neoplasia-reactor":
		return 80
	case "vent-condenser":
		return 60
	case "atmospheric-concentrator":
		return 60
	case "oxidation-chamber":
		return 30
	case "turbine-condenser":
		return 20
	case "chemical-combustion-chamber":
		return 20 * 5
	case "pyrolysis-generator":
		return 30 * 5
	case "flux-reactor":
		return 30
	case "cyanogen-synthesizer":
		return 80
	case "phase-synthesizer":
		return 10 * 4
	case "cultivator":
		return 80
	case "disassembler":
		return 12
	case "liquid-router":
		return 120
	case "liquid-container":
		return 700
	case "liquid-tank":
		return 1800
	case "bridge-conduit", "phase-conduit":
		return 100
	case "payload-loader", "payload-unloader":
		return 100
	case "reinforced-liquid-router":
		return 150
	case "reinforced-liquid-container":
		return 1000
	case "reinforced-liquid-tank":
		return 2700
	case "reinforced-bridge-conduit":
		return 120
	default:
		if prof, ok := reconstructorProfileByName(name); ok {
			return reconstructorLiquidCapacity(prof)
		}
		return 0
	}
}

func (w *World) stepNuclearReactors(delta time.Duration) {
	if w.model == nil {
		return
	}
	deltaFrames := float32(delta.Seconds() * 60)
	if deltaFrames <= 0 {
		return
	}
	const (
		reactorItemCapacity = float32(30)
		// Mindustry 156 Blocks.thoriumReactor:
		// itemDuration = 360f, heating = 0.02f, heatOutput = 15f, heatWarmupRate = 1f
		reactorHeatingPerFrame    = float32(0.02)
		reactorItemDurationFrames = float32(360)
		reactorHeatOutput         = float32(15)
		reactorHeatWarmupRate     = float32(1)
		reactorAmbientCooldown    = float32(60 * 20)
		reactorCoolantPower       = float32(0.5)
		reactorExplosionRadius    = 19
		reactorExplosionDamage    = float32(1250 * 4)
		reactorSmokeThreshold     = float32(0.3)
		reactorSmokeRadius        = float32(12)
	)
	for _, pos := range w.reactorTilePositions {
		if pos < 0 || int(pos) >= len(w.model.Tiles) {
			continue
		}
		tile := &w.model.Tiles[pos]
		if tile.Build == nil || tile.Block == 0 {
			continue
		}
		state := w.reactorStates[pos]
		fuel := itemAmountOneOf(tile.Build, thoriumItemID, legacyThoriumItemID)
		fullness := clampf(float32(fuel)/reactorItemCapacity, 0, 1)

		if fuel > 0 {
			state.Heat += fullness * reactorHeatingPerFrame * minf(deltaFrames, 4)
			state.FuelProgress += deltaFrames
			for state.FuelProgress >= reactorItemDurationFrames {
				if !removeOneItemOfLocked(tile.Build, thoriumItemID, legacyThoriumItemID) {
					state.FuelProgress = 0
					break
				}
				state.FuelProgress -= reactorItemDurationFrames
			}
		} else {
			state.FuelProgress = 0
			state.Heat = maxf(0, state.Heat-deltaFrames/reactorAmbientCooldown)
		}

		if state.Heat > 0 && len(tile.Build.Liquids) > 0 {
			cur := tile.Build.Liquids[0]
			maxUsed := minf(cur.Amount, state.Heat/reactorCoolantPower)
			if maxUsed > 0 && tile.Build.RemoveLiquid(cur.Liquid, maxUsed) {
				state.Heat -= maxUsed * reactorCoolantPower
			}
		}

		if state.Heat > reactorSmokeThreshold {
			smoke := 1 + (state.Heat-reactorSmokeThreshold)/(1-reactorSmokeThreshold)
			chance := clampf((smoke/20)*deltaFrames, 0, 1)
			if rand.Float32() < chance {
				cx := float32(tile.X*8 + 4)
				cy := float32(tile.Y*8 + 4)
				w.emitEffectLocked(
					"reactorsmoke",
					cx+(rand.Float32()*2-1)*reactorSmokeRadius,
					cy+(rand.Float32()*2-1)*reactorSmokeRadius,
					0,
				)
			}
		}

		state.Heat = clampf(state.Heat, 0, 1)
		state.HeatProgress = approachf(state.HeatProgress, state.Heat*reactorHeatOutput, reactorHeatWarmupRate*deltaFrames)
		if state.HeatProgress <= 0.0001 {
			delete(w.heatStates, pos)
		} else {
			w.heatStates[pos] = state.HeatProgress
		}
		w.reactorStates[pos] = state

		if state.Heat >= 0.999 {
			w.explodeNuclearReactorLocked(tile.X, tile.Y, pos, tile.Team, reactorExplosionRadius, reactorExplosionDamage)
		}
	}
}

func (w *World) stepLiquidLogistics(delta time.Duration) {
	if w.model == nil {
		return
	}
	dt := float32(delta.Seconds())
	if dt <= 0 {
		return
	}
	for _, pos := range w.liquidConduitTilePositions {
		if pos < 0 || int(pos) >= len(w.model.Tiles) {
			continue
		}
		tile := &w.model.Tiles[pos]
		if tile.Build == nil || tile.Block == 0 {
			continue
		}
		switch w.blockNameByID(int16(tile.Block)) {
		case "conduit":
			w.stepConduitLocked(pos, tile, 1.0, true, dt)
		case "pulse-conduit":
			w.stepConduitLocked(pos, tile, 1.025, true, dt)
		case "plated-conduit":
			w.stepConduitLocked(pos, tile, 1.025, false, dt)
		case "reinforced-conduit":
			w.stepConduitLocked(pos, tile, 1.03, true, dt)
		}
	}
	for _, pos := range w.liquidStorageTilePositions {
		if pos < 0 || int(pos) >= len(w.model.Tiles) {
			continue
		}
		tile := &w.model.Tiles[pos]
		if tile.Build == nil || tile.Block == 0 {
			continue
		}
		if liquid, _, ok := firstBuildingLiquid(tile.Build); ok {
			w.dumpLiquidLocked(pos, tile, liquid, dt*60)
		}
	}
	for _, pos := range w.liquidBridgeTilePositions {
		if pos < 0 || int(pos) >= len(w.model.Tiles) {
			continue
		}
		tile := &w.model.Tiles[pos]
		if tile.Build == nil || tile.Block == 0 {
			continue
		}
		switch w.blockNameByID(int16(tile.Block)) {
		case "bridge-conduit", "phase-conduit":
			w.stepLiquidBridgeLocked(pos, tile, dt)
		case "reinforced-bridge-conduit":
			w.stepDirectionalLiquidBridgeLocked(pos, tile, dt)
		}
	}
}

func (w *World) stepLiquidBridgeLocked(pos int32, tile *Tile, dt float32) {
	if tile == nil || tile.Build == nil {
		return
	}
	liquid, _, ok := firstBuildingLiquid(tile.Build)
	if !ok {
		return
	}
	target, linked := w.bridgeTargetLocked(pos, tile)
	if !linked {
		_ = w.dumpLiquidLocked(pos, tile, liquid, dt*60)
		return
	}
	moved := w.tryMoveLiquidLocked(pos, target, liquid, dt*60, 0)
	if moved > 0 {
		_ = tile.Build.RemoveLiquid(liquid, moved)
	}
}

func (w *World) stepDirectionalLiquidBridgeLocked(pos int32, tile *Tile, dt float32) {
	if tile == nil || tile.Build == nil {
		return
	}
	liquid, _, ok := firstBuildingLiquid(tile.Build)
	if !ok {
		return
	}
	target, linked := w.directionBridgeTargetLocked(pos, tile, "reinforced-bridge-conduit", 4)
	if linked {
		moved := w.tryMoveLiquidLocked(pos, target, liquid, dt*60, 0)
		if moved > 0 {
			_ = tile.Build.RemoveLiquid(liquid, moved)
		}
		return
	}
	if nextPos, ok := w.forwardPosLocked(pos, tile.Rotation); ok {
		if moved := w.tryMoveLiquidLocked(pos, nextPos, liquid, dt*60, 0); moved > 0 {
			_ = tile.Build.RemoveLiquid(liquid, moved)
		}
	}
}

func (w *World) stepConduitLocked(pos int32, tile *Tile, pressure float32, leaks bool, dt float32) {
	if tile == nil || tile.Build == nil || pressure <= 0 {
		return
	}
	liquid, amount, ok := firstBuildingLiquid(tile.Build)
	if !ok || amount <= 0.0001 {
		return
	}
	move := amount
	maxMove := dt * 60 * pressure
	if move > maxMove {
		move = maxMove
	}
	if move <= 0 {
		return
	}
	if nextPos, ok := w.forwardPosLocked(pos, tile.Rotation); ok {
		if moved := w.tryMoveLiquidLocked(pos, nextPos, liquid, move, 0); moved > 0 {
			_ = tile.Build.RemoveLiquid(liquid, moved)
			return
		}
	}
	if leaks {
		_ = w.dumpLiquidLocked(pos, tile, liquid, move)
	}
}

func (w *World) explodeNuclearReactorLocked(x, y int, pos int32, team TeamID, radius int, damage float32) {
	rules := w.rulesMgr.Get()
	if rules != nil && !rules.ReactorExplosions {
		_ = w.applyDamageToBuilding(pos, damage)
		w.clearBuildingRuntimeLocked(pos)
		return
	}
	w.emitEffectLocked("reactorexplosion", float32(x*8+4), float32(y*8+4), 0)
	for ty := y - radius; ty <= y+radius; ty++ {
		for tx := x - radius; tx <= x+radius; tx++ {
			if !w.model.InBounds(tx, ty) {
				continue
			}
			dx := tx - x
			dy := ty - y
			if dx*dx+dy*dy > radius*radius {
				continue
			}
			tpos := int32(ty*w.model.Width + tx)
			if w.applyDamageToBuilding(tpos, damage) {
				w.clearBuildingRuntimeLocked(tpos)
			}
		}
	}
	delete(w.reactorStates, pos)
}

func blockSizeByName(name string) int {
	switch name {
	case "distributor":
		return 2
	case "mechanical-drill", "pneumatic-drill", "rotary-pump", "water-extractor", "graphite-press", "silicon-smelter", "kiln", "plastanium-compressor", "phase-weaver", "cryofluid-mixer", "pyratite-mixer", "blast-mixer", "separator", "coal-centrifuge", "spore-press", "cultivator", "electric-heater", "phase-heater":
		return 2
	case "repair-turret", "unit-repair-tower", "plasma-bore":
		return 2
	case "power-node-large", "surge-tower", "thermal-generator", "steam-generator", "rtg-generator", "multi-press", "surge-smelter", "small-heat-redirector":
		return 2
	case "container", "reinforced-container":
		return 2
	case "laser-drill", "impulse-pump", "oil-extractor", "melter", "silicon-crucible", "disassembler", "silicon-arc-furnace", "electrolyzer", "atmospheric-concentrator", "oxidation-chamber", "slag-heater", "vent-condenser", "slag-centrifuge", "heat-reactor", "turbine-condenser", "chemical-combustion-chamber", "pyrolysis-generator", "carbide-crucible", "surge-crucible", "cyanogen-synthesizer", "phase-synthesizer", "heat-redirector", "heat-router", "large-plasma-bore":
		return 3
	case "battery-large", "solar-panel-large", "differential-generator", "beam-tower", "beam-link":
		return 3
	case "core-shard", "vault", "reinforced-vault", "thorium-reactor", "mass-driver", "payload-conveyor", "reinforced-payload-conveyor", "payload-router", "reinforced-payload-router", "payload-mass-driver", "payload-loader", "payload-unloader", "additive-reconstructor", "tank-refabricator", "ship-refabricator", "mech-refabricator", "small-deconstructor":
		return 3
	case "ground-factory", "air-factory", "naval-factory":
		return 3
	case "blast-drill", "impact-drill":
		return 4
	case "core-foundation", "core-bastion", "impact-reactor":
		return 4
	case "core-nucleus", "core-citadel", "large-payload-mass-driver", "flux-reactor", "neoplasia-reactor", "multiplicative-reconstructor", "prime-refabricator", "deconstructor", "payload-deconstructor", "payload-void", "eruption-drill":
		return 5
	case "exponential-reconstructor":
		return 7
	case "tetrative-reconstructor":
		return 9
	case "core-acropolis":
		return 6
	default:
		return 1
	}
}

func (w *World) blockSizeForTileLocked(tile *Tile) int {
	if tile == nil || tile.Block == 0 {
		return 1
	}
	return blockSizeByName(w.blockNameByID(int16(tile.Block)))
}

func blockFootprintRange(size int) (int, int) {
	if size <= 1 {
		return 0, 0
	}
	return -((size - 1) / 2), size / 2
}

var blockEdgeOffsetCache = newBlockEdgeOffsetCache()

func newBlockEdgeOffsetCache() map[int][][2]int {
	cache := make(map[int][][2]int, 9)
	for size := 1; size <= 9; size++ {
		cache[size] = computeBlockEdgeOffsets(size)
	}
	return cache
}

func blockEdgeOffsets(size int) [][2]int {
	if cached, ok := blockEdgeOffsetCache[size]; ok {
		return cached
	}
	return computeBlockEdgeOffsets(size)
}

func computeBlockEdgeOffsets(size int) [][2]int {
	low, high := blockFootprintRange(size)
	out := make([][2]int, 0, size*4)
	for x := low; x <= high; x++ {
		out = append(out, [2]int{x, low - 1})
		out = append(out, [2]int{x, high + 1})
	}
	for y := low; y <= high; y++ {
		out = append(out, [2]int{low - 1, y})
		out = append(out, [2]int{high + 1, y})
	}
	sort.Slice(out, func(i, j int) bool {
		ai := math.Atan2(float64(out[i][1]), float64(out[i][0]))
		aj := math.Atan2(float64(out[j][1]), float64(out[j][0]))
		if ai < 0 {
			ai += 2 * math.Pi
		}
		if aj < 0 {
			aj += 2 * math.Pi
		}
		if ai == aj {
			if out[i][0] == out[j][0] {
				return out[i][1] < out[j][1]
			}
			return out[i][0] < out[j][0]
		}
		return ai < aj
	})
	return out
}

func (w *World) rebuildBlockOccupancyLocked() {
	w.blockOccupancy = map[int32]int32{}
	w.dumpNeighborCache = map[int32][]int32{}
	w.unloaderLastUsed = map[int64]int{}
	w.activeTilePositions = w.activeTilePositions[:0]
	w.itemLogisticsTilePositions = w.itemLogisticsTilePositions[:0]
	w.itemConveyorTilePositions = w.itemConveyorTilePositions[:0]
	w.itemDuctTilePositions = w.itemDuctTilePositions[:0]
	w.itemRouterTilePositions = w.itemRouterTilePositions[:0]
	w.itemBridgeTilePositions = w.itemBridgeTilePositions[:0]
	w.itemUnloaderTilePositions = w.itemUnloaderTilePositions[:0]
	w.itemMassDriverTilePositions = w.itemMassDriverTilePositions[:0]
	w.sandboxItemSourceTiles = w.sandboxItemSourceTiles[:0]
	w.sandboxLiquidSourceTiles = w.sandboxLiquidSourceTiles[:0]
	w.liquidConduitTilePositions = w.liquidConduitTilePositions[:0]
	w.liquidStorageTilePositions = w.liquidStorageTilePositions[:0]
	w.liquidBridgeTilePositions = w.liquidBridgeTilePositions[:0]
	w.payloadFactoryTilePositions = w.payloadFactoryTilePositions[:0]
	w.payloadTransportTiles = w.payloadTransportTiles[:0]
	w.reactorTilePositions = w.reactorTilePositions[:0]
	w.crafterTilePositions = w.crafterTilePositions[:0]
	w.drillTilePositions = w.drillTilePositions[:0]
	w.burstDrillTilePositions = w.burstDrillTilePositions[:0]
	w.beamDrillTilePositions = w.beamDrillTilePositions[:0]
	w.pumpTilePositions = w.pumpTilePositions[:0]
	w.incineratorTilePositions = w.incineratorTilePositions[:0]
	w.repairTurretTilePositions = w.repairTurretTilePositions[:0]
	w.repairTowerTilePositions = w.repairTowerTilePositions[:0]
	w.factoryTilePositions = w.factoryTilePositions[:0]
	w.heatConductorTilePositions = w.heatConductorTilePositions[:0]
	w.powerTilePositions = w.powerTilePositions[:0]
	w.powerDiodeTilePositions = w.powerDiodeTilePositions[:0]
	w.powerVoidTilePositions = w.powerVoidTilePositions[:0]
	w.turretTilePositions = w.turretTilePositions[:0]
	w.mendProjectorPositions = w.mendProjectorPositions[:0]
	w.overdriveProjectorPositions = w.overdriveProjectorPositions[:0]
	w.forceProjectorPositions = w.forceProjectorPositions[:0]
	w.teamBuildingTiles = map[TeamID][]int32{}
	w.teamBuildingSpatial = map[TeamID]*buildingSpatialIndex{}
	w.teamCoreTiles = map[TeamID][]int32{}
	w.teamPowerTiles = map[TeamID][]int32{}
	w.teamPowerNodeTiles = map[TeamID][]int32{}
	if w.model == nil {
		return
	}
	for i := range w.model.Tiles {
		tile := &w.model.Tiles[i]
		if !isCenterBuildingTile(tile) {
			continue
		}
		w.indexActiveTileLocked(int32(i), tile)
		w.setBuildingOccupancyLocked(int32(i), tile, true)
	}
	w.refreshCoreStorageLinksLocked()
}

func (w *World) rebuildActiveTilesLocked() {
	// CRITICAL: This function should ONLY be called on map load or major world changes
	// For individual building changes, use indexActiveTileLocked/removeActiveTileIndexLocked instead
	w.activeTilePositions = w.activeTilePositions[:0]
	w.itemLogisticsTilePositions = w.itemLogisticsTilePositions[:0]
	w.itemConveyorTilePositions = w.itemConveyorTilePositions[:0]
	w.itemDuctTilePositions = w.itemDuctTilePositions[:0]
	w.itemRouterTilePositions = w.itemRouterTilePositions[:0]
	w.itemBridgeTilePositions = w.itemBridgeTilePositions[:0]
	w.itemUnloaderTilePositions = w.itemUnloaderTilePositions[:0]
	w.itemMassDriverTilePositions = w.itemMassDriverTilePositions[:0]
	w.sandboxItemSourceTiles = w.sandboxItemSourceTiles[:0]
	w.sandboxLiquidSourceTiles = w.sandboxLiquidSourceTiles[:0]
	w.liquidConduitTilePositions = w.liquidConduitTilePositions[:0]
	w.liquidStorageTilePositions = w.liquidStorageTilePositions[:0]
	w.liquidBridgeTilePositions = w.liquidBridgeTilePositions[:0]
	w.payloadFactoryTilePositions = w.payloadFactoryTilePositions[:0]
	w.payloadTransportTiles = w.payloadTransportTiles[:0]
	w.reactorTilePositions = w.reactorTilePositions[:0]
	w.crafterTilePositions = w.crafterTilePositions[:0]
	w.drillTilePositions = w.drillTilePositions[:0]
	w.burstDrillTilePositions = w.burstDrillTilePositions[:0]
	w.beamDrillTilePositions = w.beamDrillTilePositions[:0]
	w.pumpTilePositions = w.pumpTilePositions[:0]
	w.incineratorTilePositions = w.incineratorTilePositions[:0]
	w.repairTurretTilePositions = w.repairTurretTilePositions[:0]
	w.repairTowerTilePositions = w.repairTowerTilePositions[:0]
	w.factoryTilePositions = w.factoryTilePositions[:0]
	w.heatConductorTilePositions = w.heatConductorTilePositions[:0]
	w.powerTilePositions = w.powerTilePositions[:0]
	w.powerDiodeTilePositions = w.powerDiodeTilePositions[:0]
	w.powerVoidTilePositions = w.powerVoidTilePositions[:0]
	w.turretTilePositions = w.turretTilePositions[:0]
	w.mendProjectorPositions = w.mendProjectorPositions[:0]
	w.overdriveProjectorPositions = w.overdriveProjectorPositions[:0]
	w.forceProjectorPositions = w.forceProjectorPositions[:0]
	w.teamBuildingTiles = map[TeamID][]int32{}
	w.teamBuildingSpatial = map[TeamID]*buildingSpatialIndex{}
	w.teamCoreTiles = map[TeamID][]int32{}
	w.teamPowerTiles = map[TeamID][]int32{}
	w.teamPowerNodeTiles = map[TeamID][]int32{}
	if w.model == nil {
		return
	}
	for i := range w.model.Tiles {
		tile := &w.model.Tiles[i]
		if !isCenterBuildingTile(tile) {
			continue
		}
		w.indexActiveTileLocked(int32(i), tile)
	}
}

func removeIndexedPosAll(slice []int32, pos int32) []int32 {
	if len(slice) == 0 {
		return slice
	}
	out := slice[:0]
	for _, existing := range slice {
		if existing == pos {
			continue
		}
		out = append(out, existing)
	}
	return out
}

// removeActiveTileIndexLocked removes a single tile from all active tile indices
// This is the incremental version that should be used instead of rebuildActiveTilesLocked
func (w *World) removeActiveTileIndexLocked(pos int32, tile *Tile) {
	if tile == nil {
		return
	}

	// Remove from activeTilePositions
	w.activeTilePositions = removeIndexedPosAll(w.activeTilePositions, pos)

	name := w.blockNameByID(int16(tile.Block))

	// Remove from itemLogisticsTilePositions
	if isItemLogisticsBlockName(name) {
		w.itemLogisticsTilePositions = removeIndexedPosAll(w.itemLogisticsTilePositions, pos)
	}
	if isItemConveyorBlockName(name) {
		w.itemConveyorTilePositions = removeIndexedPosAll(w.itemConveyorTilePositions, pos)
	}
	if isItemDuctBlockName(name) {
		w.itemDuctTilePositions = removeIndexedPosAll(w.itemDuctTilePositions, pos)
	}
	if isItemRouterBlockName(name) {
		w.itemRouterTilePositions = removeIndexedPosAll(w.itemRouterTilePositions, pos)
	}
	if isItemBridgeBlockName(name) {
		w.itemBridgeTilePositions = removeIndexedPosAll(w.itemBridgeTilePositions, pos)
	}
	if isItemUnloaderBlockName(name) {
		w.itemUnloaderTilePositions = removeIndexedPosAll(w.itemUnloaderTilePositions, pos)
	}
	if name == "mass-driver" {
		w.itemMassDriverTilePositions = removeIndexedPosAll(w.itemMassDriverTilePositions, pos)
	}
	if name == "item-source" {
		w.sandboxItemSourceTiles = removeIndexedPosAll(w.sandboxItemSourceTiles, pos)
	}
	if name == "liquid-source" {
		w.sandboxLiquidSourceTiles = removeIndexedPosAll(w.sandboxLiquidSourceTiles, pos)
	}
	if isLiquidConduitBlockName(name) {
		w.liquidConduitTilePositions = removeIndexedPosAll(w.liquidConduitTilePositions, pos)
	}
	if isLiquidStorageBlockName(name) {
		w.liquidStorageTilePositions = removeIndexedPosAll(w.liquidStorageTilePositions, pos)
	}
	if isLiquidBridgeBlockName(name) {
		w.liquidBridgeTilePositions = removeIndexedPosAll(w.liquidBridgeTilePositions, pos)
	}
	if isPayloadFactoryBlockName(name) || isReconstructorBlockName(name) {
		w.payloadFactoryTilePositions = removeIndexedPosAll(w.payloadFactoryTilePositions, pos)
	}
	if isPayloadTransportBlockName(name) {
		w.payloadTransportTiles = removeIndexedPosAll(w.payloadTransportTiles, pos)
	}
	if name == "thorium-reactor" {
		w.reactorTilePositions = removeIndexedPosAll(w.reactorTilePositions, pos)
	}

	// Remove from crafterTilePositions
	if _, ok := crafterProfilesByBlockName[name]; ok {
		w.crafterTilePositions = removeIndexedPosAll(w.crafterTilePositions, pos)
	} else if _, ok := separatorProfilesByBlockName[name]; ok {
		w.crafterTilePositions = removeIndexedPosAll(w.crafterTilePositions, pos)
	}

	// Remove from other tile position lists
	if _, ok := drillProfilesByBlockName[name]; ok {
		w.drillTilePositions = removeIndexedPosAll(w.drillTilePositions, pos)
	}

	if _, ok := burstDrillProfilesByBlockName[name]; ok {
		w.burstDrillTilePositions = removeIndexedPosAll(w.burstDrillTilePositions, pos)
	}

	if _, ok := beamDrillProfilesByBlockName[name]; ok {
		w.beamDrillTilePositions = removeIndexedPosAll(w.beamDrillTilePositions, pos)
	}

	if _, ok := floorPumpProfilesByBlockName[name]; ok {
		w.pumpTilePositions = removeIndexedPosAll(w.pumpTilePositions, pos)
	} else if _, ok := solidPumpProfilesByBlockName[name]; ok {
		w.pumpTilePositions = removeIndexedPosAll(w.pumpTilePositions, pos)
	}

	if name == "incinerator" || name == "slag-incinerator" {
		w.incineratorTilePositions = removeIndexedPosAll(w.incineratorTilePositions, pos)
	}

	if _, ok := repairTurretProfilesByBlockName[name]; ok {
		w.repairTurretTilePositions = removeIndexedPosAll(w.repairTurretTilePositions, pos)
	}

	if _, ok := repairTowerProfilesByBlockName[name]; ok {
		w.repairTowerTilePositions = removeIndexedPosAll(w.repairTowerTilePositions, pos)
	}

	if _, ok := unitFactoryPlansByBlockName[name]; ok {
		w.factoryTilePositions = removeIndexedPosAll(w.factoryTilePositions, pos)
	}

	if isHeatConductorBlockName(name) {
		w.heatConductorTilePositions = removeIndexedPosAll(w.heatConductorTilePositions, pos)
	}

	// Remove from turret positions
	if prof, ok := w.getBuildingWeaponProfile(int16(tile.Block)); ok && prof.Damage > 0 && prof.Interval > 0 && prof.Range > 0 {
		w.turretTilePositions = removeIndexedPosAll(w.turretTilePositions, pos)
	}

	// Remove from support building positions
	if _, ok := mendProjectorProfiles[name]; ok {
		w.mendProjectorPositions = removeIndexedPosAll(w.mendProjectorPositions, pos)
	}

	if _, ok := overdriveProjectorProfiles[name]; ok {
		w.overdriveProjectorPositions = removeIndexedPosAll(w.overdriveProjectorPositions, pos)
	}

	if _, ok := forceProjectorProfiles[name]; ok {
		w.forceProjectorPositions = removeIndexedPosAll(w.forceProjectorPositions, pos)
	}

	if w.isPowerRelevantBuildingLocked(tile) {
		w.powerTilePositions = removeIndexedPosAll(w.powerTilePositions, pos)
		if isPowerNodeBlockName(name) {
			team := tile.Team
			if tile.Build != nil && tile.Build.Team != 0 {
				team = tile.Build.Team
			}
			if team != 0 {
				w.teamPowerNodeTiles[team] = removeIndexedPosAll(w.teamPowerNodeTiles[team], pos)
			}
		}
		if name == "diode" {
			w.powerDiodeTilePositions = removeIndexedPosAll(w.powerDiodeTilePositions, pos)
		}
		if name == "power-void" {
			w.powerVoidTilePositions = removeIndexedPosAll(w.powerVoidTilePositions, pos)
		}
	}

	// Remove from team building tiles
	team := tile.Team
	if tile.Build != nil && tile.Build.Team != 0 {
		team = tile.Build.Team
	}
	if team != 0 {
		w.teamBuildingTiles[team] = removeIndexedPosAll(w.teamBuildingTiles[team], pos)
		if w.isPowerRelevantBuildingLocked(tile) {
			w.teamPowerTiles[team] = removeIndexedPosAll(w.teamPowerTiles[team], pos)
			if isPowerNodeBlockName(name) {
				w.teamPowerNodeTiles[team] = removeIndexedPosAll(w.teamPowerNodeTiles[team], pos)
			}
		}

		// Remove from spatial index
		if idx, ok := w.teamBuildingSpatial[team]; ok {
			idx.remove(tile.X, tile.Y, pos)
		}

		// Remove from core tiles
		if isCoreBlockName(name) {
			w.teamCoreTiles[team] = removeIndexedPosAll(w.teamCoreTiles[team], pos)
		}
	}
}

func (w *World) indexActiveTileLocked(pos int32, tile *Tile) {
	if !isCenterBuildingTile(tile) {
		return
	}
	w.activeTilePositions = append(w.activeTilePositions, pos)
	name := w.blockNameByID(int16(tile.Block))
	if isItemLogisticsBlockName(name) {
		w.itemLogisticsTilePositions = append(w.itemLogisticsTilePositions, pos)
	}
	if isItemConveyorBlockName(name) {
		w.itemConveyorTilePositions = append(w.itemConveyorTilePositions, pos)
	}
	if isItemDuctBlockName(name) {
		w.itemDuctTilePositions = append(w.itemDuctTilePositions, pos)
	}
	if isItemRouterBlockName(name) {
		w.itemRouterTilePositions = append(w.itemRouterTilePositions, pos)
	}
	if isItemBridgeBlockName(name) {
		w.itemBridgeTilePositions = append(w.itemBridgeTilePositions, pos)
	}
	if isItemUnloaderBlockName(name) {
		w.itemUnloaderTilePositions = append(w.itemUnloaderTilePositions, pos)
	}
	if name == "mass-driver" {
		w.itemMassDriverTilePositions = append(w.itemMassDriverTilePositions, pos)
	}
	if name == "item-source" {
		w.sandboxItemSourceTiles = append(w.sandboxItemSourceTiles, pos)
	}
	if name == "liquid-source" {
		w.sandboxLiquidSourceTiles = append(w.sandboxLiquidSourceTiles, pos)
	}
	if isLiquidConduitBlockName(name) {
		w.liquidConduitTilePositions = append(w.liquidConduitTilePositions, pos)
	}
	if isLiquidStorageBlockName(name) {
		w.liquidStorageTilePositions = append(w.liquidStorageTilePositions, pos)
	}
	if isLiquidBridgeBlockName(name) {
		w.liquidBridgeTilePositions = append(w.liquidBridgeTilePositions, pos)
	}
	if isPayloadFactoryBlockName(name) || isReconstructorBlockName(name) {
		w.payloadFactoryTilePositions = append(w.payloadFactoryTilePositions, pos)
	}
	if isPayloadTransportBlockName(name) {
		w.payloadTransportTiles = append(w.payloadTransportTiles, pos)
	}
	if name == "thorium-reactor" {
		w.reactorTilePositions = append(w.reactorTilePositions, pos)
	}
	if _, ok := crafterProfilesByBlockName[name]; ok {
		w.crafterTilePositions = append(w.crafterTilePositions, pos)
	} else if _, ok := separatorProfilesByBlockName[name]; ok {
		w.crafterTilePositions = append(w.crafterTilePositions, pos)
	}
	if _, ok := drillProfilesByBlockName[name]; ok {
		w.drillTilePositions = append(w.drillTilePositions, pos)
	}
	if _, ok := burstDrillProfilesByBlockName[name]; ok {
		w.burstDrillTilePositions = append(w.burstDrillTilePositions, pos)
	}
	if _, ok := beamDrillProfilesByBlockName[name]; ok {
		w.beamDrillTilePositions = append(w.beamDrillTilePositions, pos)
	}
	if _, ok := floorPumpProfilesByBlockName[name]; ok {
		w.pumpTilePositions = append(w.pumpTilePositions, pos)
	} else if _, ok := solidPumpProfilesByBlockName[name]; ok {
		w.pumpTilePositions = append(w.pumpTilePositions, pos)
	}
	if name == "incinerator" || name == "slag-incinerator" {
		w.incineratorTilePositions = append(w.incineratorTilePositions, pos)
	}
	if _, ok := repairTurretProfilesByBlockName[name]; ok {
		w.repairTurretTilePositions = append(w.repairTurretTilePositions, pos)
	}
	if _, ok := repairTowerProfilesByBlockName[name]; ok {
		w.repairTowerTilePositions = append(w.repairTowerTilePositions, pos)
	}
	if _, ok := unitFactoryPlansByBlockName[name]; ok {
		w.factoryTilePositions = append(w.factoryTilePositions, pos)
	}
	if isHeatConductorBlockName(name) {
		w.heatConductorTilePositions = append(w.heatConductorTilePositions, pos)
	}
	team := tile.Build.Team
	if team == 0 {
		team = tile.Team
	}
	if team != 0 && w.isPowerRelevantBuildingLocked(tile) {
		w.powerTilePositions = append(w.powerTilePositions, pos)
		w.teamPowerTiles[team] = append(w.teamPowerTiles[team], pos)
		if isPowerNodeBlockName(name) {
			w.teamPowerNodeTiles[team] = append(w.teamPowerNodeTiles[team], pos)
		}
		if name == "diode" {
			w.powerDiodeTilePositions = append(w.powerDiodeTilePositions, pos)
		}
		if name == "power-void" {
			w.powerVoidTilePositions = append(w.powerVoidTilePositions, pos)
		}
	}
	if team != 0 {
		w.teamBuildingTiles[team] = append(w.teamBuildingTiles[team], pos)
		idx := w.teamBuildingSpatial[team]
		if idx == nil {
			idx = &buildingSpatialIndex{cellSize: 64, cells: map[int64][]int32{}}
			w.teamBuildingSpatial[team] = idx
		}
		idx.insert(tile.X, tile.Y, pos)
		if isCoreBlockName(name) {
			w.teamCoreTiles[team] = append(w.teamCoreTiles[team], pos)
		}
	}
	if prof, ok := w.getBuildingWeaponProfile(int16(tile.Build.Block)); ok && prof.Damage > 0 && prof.Interval > 0 && prof.Range > 0 {
		w.turretTilePositions = append(w.turretTilePositions, pos)
	}
	// Index support buildings
	if _, ok := mendProjectorProfiles[name]; ok {
		w.mendProjectorPositions = append(w.mendProjectorPositions, pos)
	}
	if _, ok := overdriveProjectorProfiles[name]; ok {
		w.overdriveProjectorPositions = append(w.overdriveProjectorPositions, pos)
	}
	if _, ok := forceProjectorProfiles[name]; ok {
		w.forceProjectorPositions = append(w.forceProjectorPositions, pos)
	}
}

func (w *World) setBuildingOccupancyLocked(pos int32, tile *Tile, occupy bool) {
	if tile == nil || !isCenterBuildingTile(tile) {
		return
	}
	low, high := blockFootprintRange(w.blockSizeForTileLocked(tile))
	for y := tile.Y + low; y <= tile.Y+high; y++ {
		for x := tile.X + low; x <= tile.X+high; x++ {
			if w.model != nil && !w.model.InBounds(x, y) {
				continue
			}
			key := packTilePos(x, y)
			if occupy {
				w.blockOccupancy[key] = pos
				continue
			}
			if cur, ok := w.blockOccupancy[key]; ok && cur == pos {
				delete(w.blockOccupancy, key)
			}
		}
	}
}

func (w *World) buildingOccupyingCellLocked(x, y int) (int32, bool) {
	if w.model == nil || !w.model.InBounds(x, y) {
		return 0, false
	}
	if pos, ok := w.blockOccupancy[packTilePos(x, y)]; ok && pos >= 0 && int(pos) < len(w.model.Tiles) {
		return w.centerBuildingIndexLocked(pos)
	}
	pos := int32(y*w.model.Width + x)
	return w.centerBuildingIndexLocked(pos)
}

func (w *World) facingEdgeLocked(fromPos, toPos int32) (int, int, bool) {
	if w.model == nil || fromPos < 0 || toPos < 0 || int(fromPos) >= len(w.model.Tiles) || int(toPos) >= len(w.model.Tiles) {
		return 0, 0, false
	}
	fromTile := &w.model.Tiles[fromPos]
	toTile := &w.model.Tiles[toPos]
	low, high := blockFootprintRange(w.blockSizeForTileLocked(fromTile))
	dx := toTile.X - fromTile.X
	dy := toTile.Y - fromTile.Y
	if dx < low {
		dx = low
	}
	if dx > high {
		dx = high
	}
	if dy < low {
		dy = low
	}
	if dy > high {
		dy = high
	}
	return fromTile.X + dx, fromTile.Y + dy, true
}

func (w *World) relativeToEdgeLocked(fromPos, toPos int32) (byte, bool) {
	fx, fy, ok := w.facingEdgeLocked(fromPos, toPos)
	if !ok {
		return 0, false
	}
	toTile := &w.model.Tiles[toPos]
	return relativeDir(fx, fy, toTile.X, toTile.Y)
}

func (w *World) flowDirBetweenLocked(fromPos, toPos int32) (byte, bool) {
	side, ok := w.relativeToEdgeLocked(fromPos, toPos)
	if !ok {
		return 0, false
	}
	return oppositeDir(side), true
}

func dirDelta(rotation int8) (int, int) {
	switch ((int(rotation) % 4) + 4) % 4 {
	case 0:
		return 1, 0
	case 1:
		return 0, 1
	case 2:
		return -1, 0
	default:
		return 0, -1
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func relativeDir(fromX, fromY, toX, toY int) (byte, bool) {
	switch {
	case fromX == toX+1 && fromY == toY:
		return 0, true
	case fromX == toX && fromY == toY+1:
		return 1, true
	case fromX == toX-1 && fromY == toY:
		return 2, true
	case fromX == toX && fromY == toY-1:
		return 3, true
	default:
		return 0, false
	}
}

func axisDir(fromX, fromY, toX, toY int) (byte, bool) {
	dx := fromX - toX
	dy := fromY - toY
	if dx == 0 && dy == 0 {
		return 0, false
	}
	if absInt(dx) >= absInt(dy) {
		if dx > 0 {
			return 0, true
		}
		return 2, true
	}
	if dy > 0 {
		return 1, true
	}
	return 3, true
}

func firstBuildingItem(b *Building) (ItemID, bool) {
	if b == nil {
		return 0, false
	}
	for _, stack := range b.Items {
		if stack.Amount > 0 {
			return stack.Item, true
		}
	}
	return 0, false
}

func totalBuildingItems(b *Building) int32 {
	if b == nil {
		return 0
	}
	total := int32(0)
	for _, stack := range b.Items {
		total += stack.Amount
	}
	return total
}

func (w *World) nextWaveSpacingSec() float32 {
	rules := w.rulesMgr.Get()
	if rules == nil {
		return 120
	}
	// Vanilla Logic.play gives the first wave a 2x grace period unless an
	// explicit initialWaveSpacing is configured.
	if w.wave <= 1 {
		if rules.InitialWaveSpacing > 0 {
			return rules.InitialWaveSpacing
		}
		if rules.WaveSpacing > 0 {
			return rules.WaveSpacing * 2
		}
	}
	if rules.WaveSpacing > 0 {
		return rules.WaveSpacing
	}
	return 120
}

func (w *World) Snapshot() Snapshot {
	w.mu.RLock()
	defer w.mu.RUnlock()
	tps := w.actualTps
	if tps <= 0 {
		tps = w.tps
	}
	return Snapshot{
		WaveTime: w.waveTime,
		Wave:     w.wave,
		Enemies:  0,
		Paused:   w.paused,
		GameOver: w.gameOver,
		TimeData: int32(time.Since(w.start).Seconds()),
		Tps:      tps,
		Rand0:    w.rand0,
		Rand1:    w.rand1,
		Tick:     w.tick,
	}
}

func (w *World) ApplySnapshot(s Snapshot) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.waveTime = s.WaveTime
	if w.waveTime < 0 {
		w.waveTime = 0
	}
	if s.Wave > 0 {
		w.wave = s.Wave
	}
	w.tick = s.Tick
	w.rand0 = s.Rand0
	w.rand1 = s.Rand1
	if s.Tps > 0 {
		w.tps = s.Tps
		w.actualTps = s.Tps
	}
	if s.TimeData > 0 {
		w.start = time.Now().Add(-time.Duration(s.TimeData) * time.Second)
	} else {
		w.start = time.Now()
	}
	w.paused = s.Paused
	w.gameOver = s.GameOver
}

func (w *World) SetPaused(paused bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.paused = paused
}

func (w *World) IsPaused() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.paused
}

func (w *World) SetGameOver(v bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.gameOver = v
}

func (w *World) IsGameOver() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.gameOver
}

func (w *World) CurrentWave() int32 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.wave
}

func (w *World) TriggerWave() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.wavesMgr != nil {
		w.triggerWave(w.wavesMgr)
	}
}

func (w *World) FillTeamCoreItems(team TeamID) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.model == nil || team == 0 {
		return
	}
	rules := w.rulesMgr.Get()
	if rules == nil {
		return
	}
	rules.setTeamRule(team, TeamRule{FillItems: true})
	w.stepFillItemsLocked()
}

func (w *World) SetModel(m *WorldModel) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.model = m
	w.rulesMgr.Set(DefaultRules())
	w.buildStates = map[int32]buildCombatState{}
	w.pendingBuilds = map[int32]pendingBuildState{}
	w.pendingBreaks = map[int32]pendingBreakState{}
	w.buildRejectLogTick = map[int32]uint64{}
	w.builderStates = map[int32]builderRuntimeState{}
	w.teamRebuildPlans = map[TeamID][]rebuildBlockPlan{}
	w.teamAIBuildPlans = map[TeamID][]teamBuildPlan{}
	w.teamBuildAIStates = map[TeamID]buildAIPlannerState{}
	w.buildAIParts = nil
	w.buildAIPartsLoaded = false
	w.factoryStates = map[int32]factoryState{}
	w.reconstructorStates = map[int32]reconstructorState{}
	w.drillStates = map[int32]drillRuntimeState{}
	w.burstDrillStates = map[int32]burstDrillRuntimeState{}
	w.beamDrillStates = map[int32]beamDrillRuntimeState{}
	w.pumpStates = map[int32]pumpRuntimeState{}
	w.crafterStates = map[int32]crafterRuntimeState{}
	w.heatStates = map[int32]float32{}
	w.incineratorStates = map[int32]float32{}
	w.repairTurretStates = map[int32]repairTurretRuntimeState{}
	w.repairTowerStates = map[int32]repairTowerRuntimeState{}
	w.buildingBoostStates = map[int32]buildingBoostState{}
	w.teamPowerStates = map[TeamID]*teamPowerState{}
	w.teamPowerBudget = map[TeamID]float32{}
	w.powerNetStates = map[int32]*powerNetState{}
	w.powerNetByPos = map[int32]int32{}
	w.powerNetIDs = nil
	w.powerNetTeams = map[int32]TeamID{}
	w.powerNetStorageRefs = map[int32][]powerStorageRef{}
	w.powerNetDirty = true
	w.powerStorageState = map[int32]float32{}
	w.powerGeneratorState = map[int32]*powerGeneratorState{}
	w.unitMountCDs = map[int32][]float32{}
	w.unitMountStates = map[int32][]unitMountState{}
	w.pendingMountShots = []pendingMountShot{}
	w.unitTargets = map[int32]targetTrackState{}
	w.unitAIStates = map[int32]unitAIState{}
	w.unitMiningStates = map[int32]unitMiningState{}
	w.teamItems = map[TeamID]map[ItemID]int32{}
	w.itemSourceCfg = map[int32]ItemID{}
	w.liquidSourceCfg = map[int32]LiquidID{}
	w.sorterCfg = map[int32]ItemID{}
	w.unloaderCfg = map[int32]ItemID{}
	w.payloadRouterCfg = map[int32]protocol.Content{}
	w.powerNodeLinks = map[int32][]int32{}
	w.bridgeLinks = map[int32]int32{}
	w.massDriverLinks = map[int32]int32{}
	w.payloadDriverLinks = map[int32]int32{}
	w.bridgeBuffers = map[int32][]bufferedBridgeItem{}
	w.bridgeAcceptAcc = map[int32]float32{}
	w.conveyorStates = map[int32]*conveyorRuntimeState{}
	w.ductStates = map[int32]*ductRuntimeState{}
	w.routerStates = map[int32]*routerRuntimeState{}
	w.stackStates = map[int32]*stackRuntimeState{}
	w.massDriverStates = map[int32]*massDriverRuntimeState{}
	w.payloadStates = map[int32]*payloadRuntimeState{}
	w.payloadDeconstructorStates = map[int32]*payloadDeconstructorState{}
	w.payloadDriverStates = map[int32]*payloadDriverRuntimeState{}
	w.massDriverShots = []massDriverShot{}
	w.payloadDriverShots = []payloadDriverShot{}
	w.blockDumpIndex = map[int32]int{}
	w.dumpNeighborCache = map[int32][]int32{}
	w.itemSourceAccum = map[int32]float32{}
	w.routerInputPos = map[int32]int32{}
	w.routerRotation = map[int32]byte{}
	w.transportAccum = map[int32]float32{}
	w.junctionQueues = map[int32]junctionQueueState{}
	w.bridgeIncomingMask = map[int32]byte{}
	w.reactorStates = map[int32]nuclearReactorState{}
	w.storageLinkedCore = map[int32]int32{}
	w.teamPrimaryCore = map[TeamID]int32{}
	w.coreStorageCapacity = map[int32]int32{}
	w.blockOccupancy = map[int32]int32{}
	w.activeTilePositions = nil
	w.itemLogisticsTilePositions = nil
	w.itemConveyorTilePositions = nil
	w.itemDuctTilePositions = nil
	w.itemRouterTilePositions = nil
	w.itemBridgeTilePositions = nil
	w.itemUnloaderTilePositions = nil
	w.itemMassDriverTilePositions = nil
	w.sandboxItemSourceTiles = nil
	w.sandboxLiquidSourceTiles = nil
	w.liquidConduitTilePositions = nil
	w.liquidStorageTilePositions = nil
	w.liquidBridgeTilePositions = nil
	w.payloadFactoryTilePositions = nil
	w.payloadTransportTiles = nil
	w.reactorTilePositions = nil
	w.crafterTilePositions = nil
	w.drillTilePositions = nil
	w.burstDrillTilePositions = nil
	w.beamDrillTilePositions = nil
	w.pumpTilePositions = nil
	w.incineratorTilePositions = nil
	w.repairTurretTilePositions = nil
	w.repairTowerTilePositions = nil
	w.factoryTilePositions = nil
	w.heatConductorTilePositions = nil
	w.powerTilePositions = nil
	w.powerDiodeTilePositions = nil
	w.powerVoidTilePositions = nil
	w.teamBuildingTiles = map[TeamID][]int32{}
	w.teamBuildingSpatial = map[TeamID]*buildingSpatialIndex{}
	w.teamCoreTiles = map[TeamID][]int32{}
	w.teamPowerTiles = map[TeamID][]int32{}
	w.teamPowerNodeTiles = map[TeamID][]int32{}
	w.turretTilePositions = nil
	w.nextPlanOrder = 0
	w.blockNamesByID = nil
	w.blockNamesByIndex = nil
	w.powerStorageCapacityByBlock = nil
	w.unitNamesByID = nil
	w.unitTypeDefsByID = nil
	w.statusProfilesByID = map[int16]statusEffectProfile{}
	w.statusProfilesByName = map[string]statusEffectProfile{}

	// 每次切图都从默认规则重新解析，再按原版 Gamemode 预设与地图 rules 叠加，
	// 避免旧地图规则残留，也避免漏掉 attack/sandbox/editor/pvp 的模式默认值。
	if m != nil {
		raw := strings.TrimSpace(tagValue(m.Tags, "rules"))
		if base, err := decodeRulesWithGamemodeDefaults([]byte(raw), m.Tags, m); err == nil && base != nil {
			w.rulesMgr.Set(base)
		} else {
			w.rulesMgr.Set(DefaultRules())
		}
		// 应用倍率到现有单位和建筑
		w.applyRulesToEntities()
	}

	if m != nil && len(m.BlockNames) > 0 {
		w.blockNamesByID = make(map[int16]string, len(m.BlockNames))
		maxBlockID := int16(0)
		for k := range m.BlockNames {
			if k > maxBlockID {
				maxBlockID = k
			}
		}
		w.blockNamesByIndex = make([]string, int(maxBlockID)+1)
		w.powerStorageCapacityByBlock = make([]float32, int(maxBlockID)+1)
		for k, v := range m.BlockNames {
			name := strings.ToLower(strings.TrimSpace(v))
			w.blockNamesByID[k] = name
			if k >= 0 {
				w.blockNamesByIndex[int(k)] = name
				w.powerStorageCapacityByBlock[int(k)] = powerStorageCapacityByBlockName(name)
			}
		}
	}
	if m != nil && len(m.UnitNames) > 0 {
		w.unitNamesByID = make(map[int16]string, len(m.UnitNames))
		for k, v := range m.UnitNames {
			w.unitNamesByID[k] = strings.ToLower(strings.TrimSpace(v))
		}
		w.unitTypeDefsByID = make(map[int16]vanilla.UnitTypeDef, len(m.UnitNames))
		for id, name := range w.unitNamesByID {
			if def, ok := vanilla.UnitTypesByName[name]; ok {
				w.unitTypeDefsByID[id] = def
			}
		}
	}
	w.rebuildBlockOccupancyLocked()
	w.restoreTileConfigsLocked()
	w.restorePayloadStatesLocked()
}
