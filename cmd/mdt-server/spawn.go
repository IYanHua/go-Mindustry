package main

import (
	"strconv"
	"strings"
	"github.com/IYanHua/mdt-server/internal/protocol"
	"github.com/IYanHua/mdt-server/internal/world"
)

func resolveUnitTypeArg(arg string, wld *world.World) (int16, string, bool) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return 0, "", false
	}
	if v, err := strconv.ParseInt(arg, 10, 16); err == nil {
		typeID := int16(v)
		name := ""
		if wld != nil {
			name = wld.UnitNameByTypeID(typeID)
		}
		if name == "" {
			name = "unknown"
		}
		return typeID, name, true
	}
	if wld == nil {
		return 0, "", false
	}
	if typeID, ok := wld.ResolveUnitTypeID(arg); ok {
		name := wld.UnitNameByTypeID(typeID)
		if name == "" {
			name = strings.ToLower(strings.TrimSpace(arg))
		}
		return typeID, name, true
	}
	return 0, "", false
}

func fallbackSpawnPosFromModel(model *world.WorldModel) (protocol.Point2, bool) {
	if model == nil || model.Width <= 0 || model.Height <= 0 || len(model.Tiles) == 0 {
		return protocol.Point2{}, false
	}
	var firstOwnedBuild protocol.Point2
	firstOwnedBuildOK := false
	for i := range model.Tiles {
		tile := &model.Tiles[i]
		if tile == nil || tile.Build == nil || tile.Block == 0 || tile.Build.Health <= 0 {
			continue
		}
		if tile.Build.X != tile.X || tile.Build.Y != tile.Y {
			continue
		}
		owner := tile.Team
		if tile.Build.Team != 0 {
			owner = tile.Build.Team
		}
		if owner == 0 {
			continue
		}
		if !firstOwnedBuildOK {
			firstOwnedBuild = protocol.Point2{X: int32(tile.X), Y: int32(tile.Y)}
			firstOwnedBuildOK = true
		}
		name := strings.ToLower(strings.TrimSpace(model.BlockNames[int16(tile.Build.Block)]))
		if strings.Contains(name, "core") || strings.Contains(name, "foundation") || strings.Contains(name, "nucleus") {
			return protocol.Point2{X: int32(tile.X), Y: int32(tile.Y)}, true
		}
	}
	if firstOwnedBuildOK {
		return firstOwnedBuild, true
	}
	return protocol.Point2{X: int32(model.Width / 2), Y: int32(model.Height / 2)}, true
}

func worldModelBlockNameAt(model *world.WorldModel, x, y int) string {
	if model == nil || !model.InBounds(x, y) {
		return ""
	}
	tile, err := model.TileAt(x, y)
	if err != nil || tile == nil || tile.Block <= 0 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(model.BlockNames[int16(tile.Block)]))
}

func resolveRespawnUnitTypeByCoreTile(wld *world.World, tile protocol.Point2, team world.TeamID, fallback int16) int16 {
	if wld == nil {
		return fallback
	}
	if _, coreName, ok := resolveTeamCoreTileWithName(wld, team, tile); ok {
		if unitName, _, ok := coreUnitNameAndRankByBlockName(coreName); ok {
			if unitTypeID, ok := wld.ResolveUnitTypeID(unitName); ok {
				return unitTypeID
			}
		}
	}
	model := wld.CloneModel()
	if model == nil {
		return fallback
	}
	if unitName, _, ok := coreUnitNameAndRankByBlockName(worldModelBlockNameAt(model, int(tile.X), int(tile.Y))); ok {
		if unitTypeID, ok := wld.ResolveUnitTypeID(unitName); ok {
			return unitTypeID
		}
	}
	return fallback
}

func coreUnitNameAndRankByBlockName(blockName string) (string, int, bool) {
	name := strings.ToLower(strings.TrimSpace(blockName))
	switch {
	case strings.Contains(name, "core-shard"):
		return "alpha", 1, true
	case strings.Contains(name, "core-foundation"):
		return "beta", 2, true
	case strings.Contains(name, "core-nucleus"):
		return "gamma", 3, true
	case strings.Contains(name, "core-bastion"):
		return "evoke", 1, true
	case strings.Contains(name, "core-citadel"):
		return "incite", 2, true
	case strings.Contains(name, "core-acropolis"):
		return "emanate", 3, true
	default:
		return "", 0, false
	}
}

