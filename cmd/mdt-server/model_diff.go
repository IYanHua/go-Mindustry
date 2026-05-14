package main

import (
	"bytes"
	"math"
	"strings"
	"github.com/IYanHua/mdt-server/internal/protocol"
	"github.com/IYanHua/mdt-server/internal/world"
)

func buildSyncSnapshotEntriesFromModel(model *world.WorldModel) []world.BuildSyncSnapshotEntry {
	if model == nil || len(model.Tiles) == 0 {
		return nil
	}
	out := make([]world.BuildSyncSnapshotEntry, 0, len(model.Tiles)/4)
	for i := range model.Tiles {
		tile := model.Tiles[i]
		if tile.Block <= 0 || tile.Build == nil {
			continue
		}
		if tile.Build.X != tile.X || tile.Build.Y != tile.Y {
			continue
		}
		hp := float32(1000)
		if tile.Build != nil && tile.Build.Health > 0 {
			hp = tile.Build.Health
		}
		team := tile.Team
		if tile.Build != nil {
			team = tile.Build.Team
		}
		entry := world.BuildSyncSnapshotEntry{
			BuildSyncState: world.BuildSyncState{
				Pos:      protocol.PackPoint2(int32(tile.X), int32(tile.Y)),
				X:        int32(tile.X),
				Y:        int32(tile.Y),
				BlockID:  int16(tile.Block),
				Team:     team,
				Rotation: tile.Rotation,
				Health:   hp,
			},
		}
		if len(tile.Build.Config) > 0 {
			entry.Config = append([]byte(nil), tile.Build.Config...)
		}
		out = append(out, entry)
	}
	return out
}

func tileAtPacked(model *world.WorldModel, pos int32) (*world.Tile, bool) {
	if model == nil {
		return nil, false
	}
	pt := protocol.UnpackPoint2(pos)
	if !model.InBounds(int(pt.X), int(pt.Y)) {
		return nil, false
	}
	tile, err := model.TileAt(int(pt.X), int(pt.Y))
	if err != nil || tile == nil {
		return nil, false
	}
	return tile, true
}

func decodeTileConfigValue(tile *world.Tile) (any, bool) {
	if tile == nil || tile.Build == nil || len(tile.Build.Config) == 0 {
		return nil, false
	}
	return decodeTileConfigBytes(tile.Build.Config)
}

func decodeTileConfigBytes(raw []byte) (any, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	value, err := protocol.ReadObject(protocol.NewReader(raw), false, nil)
	if err != nil {
		return nil, false
	}
	return value, true
}

func isConstructLikeBlockName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if !strings.HasPrefix(name, "build") || len(name) <= len("build") {
		return false
	}
	for _, r := range name[len("build"):] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func modelTileIsConstructLike(model *world.WorldModel, pos int32) bool {
	tile, ok := tileAtPacked(model, pos)
	if !ok || tile == nil || tile.Block <= 0 || model == nil {
		return false
	}
	return isConstructLikeBlockName(model.BlockNames[int16(tile.Block)])
}

func sameConfigBytes(baseCfg, liveCfg []byte) bool {
	return bytes.Equal(baseCfg, liveCfg)
}

func sameBuildHealth(a, b float32) bool {
	return math.Abs(float64(a-b)) < 0.01
}

