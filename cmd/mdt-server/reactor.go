package main

import (
	"fmt"
	"math"
	"strings"
	"github.com/IYanHua/mdt-server/internal/protocol"
	"github.com/IYanHua/mdt-server/internal/world"
)

type reactorExplosionGroup struct {
	effectName    string
	centerX       int
	centerY       int
	radiusTiles   int
	sourceIndex   int
	affectedIndex []int
}

func isReactorBlockName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "thorium-reactor", "impact-reactor", "flux-reactor", "neoplasia-reactor", "heat-reactor":
		return true
	default:
		return false
	}
}

func reactorExplosionRadiusByEffect(name string) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "reactorexplosion":
		return 19, true
	case "explosionreactorneoplasm":
		return 9, true
	default:
		return 0, false
	}
}

func effectWorldToTile(v float32) int {
	return int(math.Round(float64((v - 4) / 8)))
}

func destroyedBuildCoords(ev world.EntityEvent) (int, int) {
	pt := protocol.UnpackPoint2(ev.BuildPos)
	return int(pt.X), int(pt.Y)
}

func classifyReactorExplosionBuilds(wld *world.World, evs []world.EntityEvent) map[int]*reactorExplosionGroup {
	groups := make([]*reactorExplosionGroup, 0, 4)
	for i, ev := range evs {
		if ev.Kind != world.EntityEventEffect {
			continue
		}
		radius, ok := reactorExplosionRadiusByEffect(ev.EffectName)
		if !ok {
			continue
		}
		groups = append(groups, &reactorExplosionGroup{
			effectName:  ev.EffectName,
			centerX:     effectWorldToTile(ev.EffectX),
			centerY:     effectWorldToTile(ev.EffectY),
			radiusTiles: radius,
			sourceIndex: -1,
		})
		_ = i
	}
	if len(groups) == 0 {
		return nil
	}

	out := make(map[int]*reactorExplosionGroup)

	for _, group := range groups {
		for i, ev := range evs {
			if ev.Kind != world.EntityEventBuildDestroyed {
				continue
			}
			x, y := destroyedBuildCoords(ev)
			if x != group.centerX || y != group.centerY {
				continue
			}
			blockName := ""
			if wld != nil {
				blockName = wld.BlockNameByID(ev.BuildBlock)
			}
			if isReactorBlockName(blockName) {
				group.sourceIndex = i
				out[i] = group
				break
			}
			if group.sourceIndex < 0 {
				group.sourceIndex = i
				out[i] = group
			}
		}
	}

	for i, ev := range evs {
		if ev.Kind != world.EntityEventBuildDestroyed {
			continue
		}
		if _, ok := out[i]; ok {
			continue
		}
		x, y := destroyedBuildCoords(ev)
		best := (*reactorExplosionGroup)(nil)
		bestDist := math.MaxFloat64
		for _, group := range groups {
			dx := float64(x - group.centerX)
			dy := float64(y - group.centerY)
			dist2 := dx*dx + dy*dy
			if dist2 > float64(group.radiusTiles*group.radiusTiles) {
				continue
			}
			if best == nil || dist2 < bestDist {
				best = group
				bestDist = dist2
			}
		}
		if best != nil {
			best.affectedIndex = append(best.affectedIndex, i)
			out[i] = best
		}
	}

	return out
}

func logGroupedReactorExplosions(wld *world.World, evs []world.EntityEvent, grouped map[int]*reactorExplosionGroup) {
	if len(grouped) == 0 {
		return
	}
	seen := map[*reactorExplosionGroup]struct{}{}
	for _, group := range grouped {
		if group == nil {
			continue
		}
		if _, ok := seen[group]; ok {
			continue
		}
		seen[group] = struct{}{}

		sourceLabel := fmt.Sprintf("(x%d-y%d)", group.centerX, group.centerY)
		sourceTeam := world.TeamID(0)
		sourceBlockID := int16(0)
		sourceBlockName := "未知"
		if group.sourceIndex >= 0 && group.sourceIndex < len(evs) {
			ev := evs[group.sourceIndex]
			sourceTeam = ev.BuildTeam
			sourceBlockID = ev.BuildBlock
			sourceBlockName = blockDisplayName(wld, ev.BuildBlock)
			sx, sy := destroyedBuildCoords(ev)
			sourceLabel = fmt.Sprintf("(x%d-y%d)", sx, sy)
		}

		fmt.Printf("[反应堆爆炸] 源头=%s block=%d(%s) team=%d effect=%s 波及=%d\n",
			sourceLabel, sourceBlockID, sourceBlockName, sourceTeam, group.effectName, len(group.affectedIndex))

		if len(group.affectedIndex) == 0 {
			continue
		}
		parts := make([]string, 0, len(group.affectedIndex))
		for _, idx := range group.affectedIndex {
			if idx < 0 || idx >= len(evs) {
				continue
			}
			ev := evs[idx]
			x, y := destroyedBuildCoords(ev)
			parts = append(parts, fmt.Sprintf("(x%d-y%d)%s team=%d", x, y, blockDisplayName(wld, ev.BuildBlock), ev.BuildTeam))
		}
		if len(parts) > 0 {
			fmt.Printf("[爆炸波及] 源头=%s %s\n", sourceLabel, strings.Join(parts, ", "))
		}
	}
}

