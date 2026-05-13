package oracle

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/IYanHua/mdt-server/internal/world"
	"github.com/IYanHua/mdt-server/internal/worldstream"
)

type GoTraceOptions struct {
	WorkspaceRoot string
}

func CollectGoTrace(scenario Scenario, opts GoTraceOptions) (Trace, error) {
	scenario = scenario.Normalized()
	mapPath, err := resolveScenarioFile(opts.WorkspaceRoot, scenario.MapPath)
	if err != nil {
		return Trace{}, fmt.Errorf("resolve map path: %w", err)
	}
	profilesPath := scenario.VanillaProfilesPath
	if profilesPath != "" {
		profilesPath, err = resolveScenarioFile(opts.WorkspaceRoot, profilesPath)
		if err != nil {
			return Trace{}, fmt.Errorf("resolve vanilla profiles path: %w", err)
		}
	}
	var hotReloadPath string
	if strings.TrimSpace(scenario.HotReloadMapPath) != "" {
		hotReloadPath, err = resolveScenarioFile(opts.WorkspaceRoot, scenario.HotReloadMapPath)
		if err != nil {
			return Trace{}, fmt.Errorf("resolve hot reload map path: %w", err)
		}
	}
	if scenario.Seed != 0 {
		rand.Seed(scenario.Seed)
	}

	w := world.New(world.Config{TPS: 60})
	trace := NewTrace("go-world", scenario)
	trace.Metadata["mapPathResolved"] = mapPath
	if profilesPath != "" {
		trace.Metadata["vanillaProfilesResolved"] = profilesPath
	}

	if err := loadScenarioWorld(w, mapPath, profilesPath); err != nil {
		return Trace{}, err
	}
	if model := w.CloneModel(); model != nil {
		trace.Metadata["mapWidth"] = strconv.Itoa(model.Width)
		trace.Metadata["mapHeight"] = strconv.Itoa(model.Height)
		trace.Metadata["msavVersion"] = strconv.Itoa(int(model.MSAVVersion))
	}
	if scenario.CaptureInitial {
		trace.Ticks = append(trace.Ticks, snapshotWorldTick(w, 0, nil))
	}

	for tick := 1; tick <= scenario.Ticks; tick++ {
		var events []TraceEvent
		if hotReloadPath != "" && scenario.HotReloadTick > 0 && tick == scenario.HotReloadTick {
			if err := loadScenarioWorld(w, hotReloadPath, profilesPath); err != nil {
				return Trace{}, fmt.Errorf("hot reload world at tick %d: %w", tick, err)
			}
			events = append(events, TraceEvent{
				Tick: tick,
				Kind: "map_hot_reload",
				Fields: map[string]string{
					"path": hotReloadPath,
				},
			})
		}
		w.Step(scenario.DeltaDuration())
		trace.Ticks = append(trace.Ticks, snapshotWorldTick(w, tick, events))
	}
	return trace, nil
}

func loadScenarioWorld(w *world.World, mapPath, profilesPath string) error {
	model, err := worldstream.LoadWorldModelFromMSAV(mapPath, nil)
	if err != nil {
		return fmt.Errorf("load msav %s: %w", mapPath, err)
	}
	w.SetModel(model)
	if profilesPath != "" {
		if err := w.LoadVanillaProfiles(profilesPath); err != nil {
			return fmt.Errorf("load vanilla profiles %s: %w", profilesPath, err)
		}
	}
	return nil
}

func snapshotWorldTick(w *world.World, index int, events []TraceEvent) TickTrace {
	model := w.CloneModel()
	tick := TickTrace{
		Index:  index,
		Tiles:  collectTileStates(model),
		Units:  collectUnitStates(model),
		Events: append([]TraceEvent(nil), events...),
	}
	return tick
}

func collectTileStates(model *world.WorldModel) []TileState {
	if model == nil || len(model.Tiles) == 0 {
		return nil
	}
	out := make([]TileState, 0, len(model.Tiles)/8)
	for i := range model.Tiles {
		tile := model.Tiles[i]
		if !includeTileInTrace(tile) {
			continue
		}
		state := TileState{
			X:         tile.X,
			Y:         tile.Y,
			FloorID:   int(tile.Floor),
			OverlayID: int(tile.Overlay),
			BlockID:   int(tile.Block),
			TeamID:    int(tile.Team),
			Rotation:  int(tile.Rotation),
			Logic: LogicState{
				Enabled: false,
			},
		}
		if tile.Build != nil {
			state.BuildHealth = float64(tile.Build.Health)
			state.Items = collectItemSlots(tile.Build.Items)
			state.Liquids = collectLiquidSlots(tile.Build.Liquids)
			if len(tile.Build.Payload) > 0 {
				state.PayloadUnitTypeIDs = []int{len(tile.Build.Payload)}
			}
		}
		out = append(out, state)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Y != out[j].Y {
			return out[i].Y < out[j].Y
		}
		return out[i].X < out[j].X
	})
	return out
}

func includeTileInTrace(tile world.Tile) bool {
	if tile.Build != nil {
		return true
	}
	return tile.Block > 0
}

func collectItemSlots(stacks []world.ItemStack) []SlotState {
	if len(stacks) == 0 {
		return nil
	}
	out := make([]SlotState, 0, len(stacks))
	for _, stack := range stacks {
		if stack.Amount == 0 {
			continue
		}
		out = append(out, SlotState{ID: int(stack.Item), Amount: float64(stack.Amount)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func collectLiquidSlots(stacks []world.LiquidStack) []SlotState {
	if len(stacks) == 0 {
		return nil
	}
	out := make([]SlotState, 0, len(stacks))
	for _, stack := range stacks {
		if stack.Amount == 0 {
			continue
		}
		out = append(out, SlotState{ID: int(stack.Liquid), Amount: float64(stack.Amount)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func collectUnitStates(model *world.WorldModel) []UnitState {
	if model == nil || len(model.Entities) == 0 {
		return nil
	}
	out := make([]UnitState, 0, len(model.Entities))
	for _, ent := range model.Entities {
		if ent.ID == 0 || ent.TypeID <= 0 {
			continue
		}
		state := UnitState{
			ID:             int(ent.ID),
			TypeID:         int(ent.TypeID),
			TeamID:         int(ent.Team),
			X:              float64(ent.X),
			Y:              float64(ent.Y),
			Rotation:       float64(ent.Rotation),
			VelocityX:      float64(ent.VelX),
			VelocityY:      float64(ent.VelY),
			Health:         float64(ent.Health),
			Shield:         float64(ent.Shield),
			ControllerType: controllerType(ent),
			AIState: map[string]string{
				"behavior": strings.TrimSpace(ent.Behavior),
			},
		}
		if ent.CommandID != 0 {
			state.AIState["commandId"] = strconv.Itoa(int(ent.CommandID))
		}
		if ent.TargetID != 0 {
			state.AIState["targetId"] = strconv.Itoa(int(ent.TargetID))
		}
		if ent.PlayerID != 0 {
			state.AIState["playerId"] = strconv.Itoa(int(ent.PlayerID))
		}
		out = append(out, state)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func controllerType(ent world.RawEntity) string {
	if ent.PlayerID != 0 {
		return "player"
	}
	if strings.TrimSpace(ent.Behavior) != "" {
		return strings.TrimSpace(ent.Behavior)
	}
	if ent.CommandID != 0 {
		return "command"
	}
	return "none"
}

func resolveScenarioFile(root, path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path is empty")
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	base := strings.TrimSpace(root)
	if base == "" {
		var err error
		base, err = filepath.Abs(".")
		if err != nil {
			return "", err
		}
	}
	return filepath.Clean(filepath.Join(base, filepath.FromSlash(path))), nil
}
