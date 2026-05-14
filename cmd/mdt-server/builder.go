package main

import (
	netserver "github.com/IYanHua/mdt-server/internal/net"
	"github.com/IYanHua/mdt-server/internal/protocol"
	"github.com/IYanHua/mdt-server/internal/world"
)

func resolveBuildOwner(c *netserver.Conn) int32 {
	if c == nil {
		return 0
	}
	if id := c.PlayerID(); id != 0 {
		return id
	}
	return c.ConnID()
}

func hasQueuedBuildPlans(plans []*protocol.BuildPlan) bool {
	for _, plan := range plans {
		if plan != nil {
			return true
		}
	}
	return false
}

func builderSnapshotActive(dead bool, unitID int32, building bool, plans []*protocol.BuildPlan, forceActive bool) bool {
	if dead || unitID == 0 {
		return false
	}
	if forceActive {
		return true
	}
	return building || hasQueuedBuildPlans(plans)
}

func syncBuilderStateFromConnSnapshot(wld *world.World, c *netserver.Conn, owner int32, team world.TeamID, plans []*protocol.BuildPlan, forceActive bool) {
	if wld == nil || c == nil || owner == 0 {
		return
	}
	snapX, snapY := c.SnapshotPos()
	active := builderSnapshotActive(c.IsDead(), c.UnitID(), c.IsBuilding(), plans, forceActive)
	if !active && wld.HasPendingPlansForOwner(owner) {
		active = true
	}
	// UnitType.buildRange defaults to Vars.buildingRange in 157.
	wld.UpdateBuilderState(owner, team, c.UnitID(), snapX, snapY, active, 220)
}

// getSpawnPos 获取重生点位置（支持多核心轮转）
