package main

import (
	"strconv"
	"strings"
	netserver "github.com/IYanHua/mdt-server/internal/net"
	"github.com/IYanHua/mdt-server/internal/protocol"
	"github.com/IYanHua/mdt-server/internal/world"
)

func resolveTeamCoreTile(wld *world.World, team world.TeamID, ref protocol.Point2) (protocol.Point2, bool) {
	pos, _, ok := resolveTeamCoreTileWithName(wld, team, ref)
	return pos, ok
}

func resolveTeamCoreTileWithName(wld *world.World, team world.TeamID, ref protocol.Point2) (protocol.Point2, string, bool) {
	if wld == nil || team == 0 {
		return protocol.Point2{}, "", false
	}
	model := wld.CloneModel()
	if model == nil || model.Width <= 0 || model.Height <= 0 || len(model.Tiles) == 0 {
		return protocol.Point2{}, "", false
	}
	refX := int(ref.X)
	refY := int(ref.Y)
	if !model.InBounds(refX, refY) {
		refX = model.Width / 2
		refY = model.Height / 2
	}
	bestRank := -1
	bestDist2 := int(^uint(0) >> 1)
	bestPos := protocol.Point2{}
	bestName := ""
	consider := func(t *world.Tile) {
		if t == nil || t.Block <= 0 || t.Build == nil {
			return
		}
		blockName := model.BlockNames[int16(t.Block)]
		_, rank, ok := coreUnitNameAndRankByBlockName(blockName)
		if !ok {
			return
		}
		dx := t.X - refX
		dy := t.Y - refY
		dist2 := dx*dx + dy*dy
		if rank > bestRank || (rank == bestRank && dist2 < bestDist2) {
			bestRank = rank
			bestDist2 = dist2
			bestPos = protocol.Point2{X: int32(t.X), Y: int32(t.Y)}
			bestName = blockName
		}
	}
	for _, packed := range wld.TeamCorePositions(team) {
		x := int(protocol.UnpackPoint2X(packed))
		y := int(protocol.UnpackPoint2Y(packed))
		t, err := model.TileAt(x, y)
		if err != nil {
			continue
		}
		consider(t)
	}
	if bestRank < 0 {
		for i := range model.Tiles {
			t := &model.Tiles[i]
			if t == nil || t.Build == nil || t.Block <= 0 {
				continue
			}
			if t.Build.X != t.X || t.Build.Y != t.Y {
				continue
			}
			owner := t.Team
			if t.Build.Team != 0 {
				owner = t.Build.Team
			}
			if owner != team {
				continue
			}
			consider(t)
		}
	}
	if bestRank < 0 {
		return protocol.Point2{}, "", false
	}
	return bestPos, bestName, true
}

func resolveConnTeam(c *netserver.Conn, wld *world.World) world.TeamID {
	defaultTeam := resolveDefaultPlayerTeam(wld)
	if c == nil {
		return defaultTeam
	}
	if teamID := c.TeamID(); teamID != 0 {
		return world.TeamID(teamID)
	}
	if wld == nil {
		return defaultTeam
	}
	if unitID := c.UnitID(); unitID != 0 {
		if ent, ok := wld.GetEntity(unitID); ok && ent.Team != 0 {
			return ent.Team
		}
	}
	if playerID := c.PlayerID(); playerID != 0 {
		if ent, ok := wld.GetEntity(playerID); ok && ent.Team != 0 {
			return ent.Team
		}
	}
	return defaultTeam
}

func assignConnTeamVanilla(srv *netserver.Server, wld *world.World, c *netserver.Conn) world.TeamID {
	defaultTeam := resolveDefaultPlayerTeam(wld)
	if wld == nil {
		return defaultTeam
	}
	rulesMgr := wld.GetRulesManager()
	if rulesMgr == nil {
		return defaultTeam
	}
	rules := rulesMgr.Get()
	if rules == nil || !rules.Pvp {
		return defaultTeam
	}
	waveTeam := resolveConfiguredTeamID(rules.WaveTeam, world.TeamID(2))
	counts := map[byte]int{}
	if srv != nil {
		counts = srv.ConnectedTeamCounts()
	}
	if c != nil {
		if current := c.TeamID(); counts[current] > 0 {
			counts[current]--
		}
	}
	bestTeam := world.TeamID(0)
	bestCount := int(^uint(0) >> 1)
	for rawTeam := 1; rawTeam <= 255; rawTeam++ {
		team := world.TeamID(rawTeam)
		if team == waveTeam && rules.Waves {
			continue
		}
		if _, ok := resolveTeamCoreTile(wld, team, protocol.Point2{}); !ok {
			continue
		}
		count := counts[byte(team)]
		if count < bestCount || (count == bestCount && (bestTeam == 0 || team < bestTeam)) {
			bestTeam = team
			bestCount = count
		}
	}
	if bestTeam != 0 {
		return bestTeam
	}
	return defaultTeam
}

func resolveDefaultPlayerTeam(wld *world.World) world.TeamID {
	const fallback = world.TeamID(1)
	if wld == nil {
		return fallback
	}
	rulesMgr := wld.GetRulesManager()
	if rulesMgr == nil {
		return fallback
	}
	rules := rulesMgr.Get()
	if rules == nil {
		return fallback
	}
	return resolveConfiguredTeamID(rules.DefaultTeam, fallback)
}

func resolveConfiguredTeamID(value string, fallback world.TeamID) world.TeamID {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "derelict":
		return world.TeamID(0)
	case "sharded":
		return world.TeamID(1)
	case "crux":
		return world.TeamID(2)
	case "malis":
		return world.TeamID(3)
	case "green":
		return world.TeamID(4)
	case "blue":
		return world.TeamID(5)
	default:
		if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && n >= 0 && n <= 255 {
			return world.TeamID(n)
		}
		return fallback
	}
}

