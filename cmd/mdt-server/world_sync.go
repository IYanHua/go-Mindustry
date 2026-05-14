package main

import (
	"math"
	"sort"
	"time"
	netserver "github.com/IYanHua/mdt-server/internal/net"
	"github.com/IYanHua/mdt-server/internal/config"
	"github.com/IYanHua/mdt-server/internal/protocol"
	"github.com/IYanHua/mdt-server/internal/world"
)

func syncCurrentWorldToConn(conn *netserver.Conn, wld *world.World) {
	if conn == nil || wld == nil {
		return
	}
	builds := wld.BuildSyncSnapshot()
	health := make([]int32, 0, 256)
	type buildConfigState struct {
		pos   int32
		value any
	}
	configs := make([]buildConfigState, 0, 64)
	for i := range builds {
		b := builds[i]
		if b.BlockID <= 0 {
			continue
		}
		_ = conn.SendAsync(&protocol.Remote_Tile_setTile_140{
			Tile:     protocol.TileBox{PosValue: b.Pos},
			Block:    protocol.BlockRef{BlkID: b.BlockID, BlkName: ""},
			Team:     protocol.Team{ID: byte(b.Team)},
			Rotation: int32(b.Rotation) & 0x3,
		})
		hp := b.Health
		if hp <= 0 {
			hp = 1000
		}
		health = append(health, b.Pos, int32(math.Float32bits(hp)))
		if len(health) >= 256 {
			_ = conn.SendAsync(&protocol.Remote_Tile_buildHealthUpdate_144{
				Buildings: protocol.IntSeq{Items: append([]int32(nil), health...)},
			})
			health = health[:0]
		}
		if cfgValue, ok := wld.BuildingConfigPacked(b.Pos); ok && shouldSendTileConfigForPacked(wld, b.Pos, cfgValue) {
			configs = append(configs, buildConfigState{
				pos:   b.Pos,
				value: cfgValue,
			})
		}
	}
	if len(health) > 0 {
		_ = conn.SendAsync(&protocol.Remote_Tile_buildHealthUpdate_144{
			Buildings: protocol.IntSeq{Items: health},
		})
	}
	sendBlockSnapshotsToConn(conn, wld)
	for _, cfg := range configs {
		if packet, ok := newTileConfigPacket(cfg.pos, cfg.value); ok {
			_ = conn.SendAsync(packet)
		}
	}
}

func syncLiveWorldRuntimeToConn(conn *netserver.Conn, wld *world.World) {
	if conn == nil || wld == nil {
		return
	}
	builds := wld.BuildSyncSnapshot()
	health := make([]int32, 0, 256)
	unsupportedPositions := make([]int32, 0, 64)
	configs := make(map[int32]any, 64)
	for i := range builds {
		b := builds[i]
		if b.BlockID <= 0 {
			continue
		}
		if wld.HasLiveMapStreamPayloadPacked(b.Pos) {
			hp := b.Health
			if hp <= 0 {
				hp = 1000
			}
			health = append(health, b.Pos, int32(math.Float32bits(hp)))
			if len(health) >= 256 {
				_ = conn.SendAsync(&protocol.Remote_Tile_buildHealthUpdate_144{
					Buildings: protocol.IntSeq{Items: append([]int32(nil), health...)},
				})
				health = health[:0]
			}
			if cfgValue, ok := wld.BuildingConfigPacked(b.Pos); ok && shouldSendTileConfigForPacked(wld, b.Pos, cfgValue) {
				configs[b.Pos] = cfgValue
			}
			continue
		}
		unsupportedPositions = append(unsupportedPositions, b.Pos)
	}
	if len(health) > 0 {
		_ = conn.SendAsync(&protocol.Remote_Tile_buildHealthUpdate_144{
			Buildings: protocol.IntSeq{Items: health},
		})
	}
	for _, pos := range unsupportedPositions {
		sendAuthoritativeTileStateToConn(conn, wld, pos)
	}
	sendBlockSnapshotsForPackedToConn(conn, wld, expandRelatedBlockSyncPackedPositions(wld, unsupportedPositions))
	sendTileConfigMapToConn(conn, configs)
}

func syncCoreItemsToConn(conn *netserver.Conn, wld *world.World) {
	// Mindustry-157 does not use InputHandler.setTileItems for authoritative
	// correction. Core inventory travels in stateSnapshot(coreData), and building
	// inventory/ammo travels in blockSnapshot(writeSync).
	_ = conn
	_ = wld
}

func syncCurrentRuntimeStateToConn(srv *netserver.Server, conn *netserver.Conn, wld *world.World, mapPath string) {
	if conn == nil || wld == nil {
		return
	}
	syncRulesToConn(conn, wld, mapPath)
	syncCurrentWorldToConn(conn, wld)
	if srv != nil {
		_ = srv.SyncEntitySnapshotsToConn(conn)
	}
}

func syncDeferredLiveRuntimeStateToConn(srv *netserver.Server, conn *netserver.Conn, wld *world.World, mapPath string) {
	if conn == nil || wld == nil {
		return
	}
	syncRulesToConn(conn, wld, mapPath)
	syncLiveWorldRuntimeToConn(conn, wld)
	if srv != nil {
		_ = srv.SyncEntitySnapshotsToConn(conn)
	}
}

func liveRuntimeRepairDelay(srv *netserver.Server) time.Duration {
	_ = srv
	// Keep the second-pass repair behind the initial reload/respawn window so
	// streamed tiles have time to materialize their building entities client-side.
	return 550 * time.Millisecond
}

func scheduleCurrentRuntimeStateRepair(srv *netserver.Server, conn *netserver.Conn, wld *world.World, mapPath string, delay time.Duration) {
	if srv == nil || conn == nil || wld == nil || delay <= 0 || !conn.UsesLiveWorldStream() {
		return
	}
	go func(c *netserver.Conn) {
		time.Sleep(delay)
		if c == nil || !c.UsesLiveWorldStream() {
			return
		}
		// Some official clients still need a second-pass runtime repair after the
		// streamed map finishes applying. Keep this pass light: replay runtime
		// state/block snapshots without re-sending setTile for the whole map.
		syncDeferredLiveRuntimeStateToConn(srv, c, wld, mapPath)
	}(conn)
}

func syncPostConnectWorldStateToConn(srv *netserver.Server, conn *netserver.Conn, wld *world.World, baseModel *world.WorldModel, mapPath string, strategy config.AuthoritySyncStrategy) {
	if conn == nil || wld == nil {
		return
	}
	if conn.UsesLiveWorldStream() {
		// The live world-stream payload already carries the current structural map.
		// Delay the runtime repair pass so the client can finish constructing tile
		// entities before block snapshots/config packets arrive.
		scheduleCurrentRuntimeStateRepair(srv, conn, wld, mapPath, liveRuntimeRepairDelay(srv))
		return
	}
	syncRulesToConn(conn, wld, mapPath)
	syncAuthoritativeWorldToConn(srv, conn, wld, baseModel, strategy)
}

func syncAuthoritativeWorldToConn(srv *netserver.Server, conn *netserver.Conn, wld *world.World, baseModel *world.WorldModel, strategy config.AuthoritySyncStrategy) {
	if conn == nil || wld == nil {
		return
	}
	if conn.UsesLiveWorldStream() {
		syncDeferredLiveRuntimeStateToConn(srv, conn, wld, "")
		return
	}
	width, height, ok := wld.Bounds()
	if !ok || baseModel == nil || baseModel.Width != width || baseModel.Height != height {
		syncCurrentWorldToConn(conn, wld)
		if srv != nil {
			_ = srv.SyncEntitySnapshotsToConn(conn)
		}
		return
	}
	switch strategy {
	case config.AuthoritySyncOfficial:
		// Vanilla writeWorld already contains current live map state.
		// Our template stream does not, so the closest emulation is:
		// template world stream + diff correction + live block snapshots.
		syncWorldDiffToConn(conn, wld, baseModel)
	case config.AuthoritySyncStatic:
		// Static mode only reconciles differences against the template map stream.
		syncWorldDiffToConn(conn, wld, baseModel)
	default:
		// Dynamic mode previously replayed every live build through setTile/tileConfig,
		// which could reset factory/progress runtime on clients. Keep it on the
		// safer diff + blockSnapshot path until we have a byte-perfect live map writer.
		syncWorldDiffToConn(conn, wld, baseModel)
	}
	if srv != nil {
		_ = srv.SyncEntitySnapshotsToConn(conn)
	}
}

func syncWorldDiffToConn(conn *netserver.Conn, wld *world.World, baseModel *world.WorldModel) {
	if conn == nil || wld == nil {
		return
	}
	width, height, ok := wld.Bounds()
	if !ok || baseModel == nil || baseModel.Width != width || baseModel.Height != height {
		syncCurrentWorldToConn(conn, wld)
		return
	}

	baseStates := buildSyncSnapshotEntriesFromModel(baseModel)
	liveStates := wld.BuildSyncSnapshotWithConfig()
	baseByPos := make(map[int32]world.BuildSyncSnapshotEntry, len(baseStates))
	liveByPos := make(map[int32]world.BuildSyncSnapshotEntry, len(liveStates))
	posSet := make(map[int32]struct{}, len(baseStates)+len(liveStates))
	for _, state := range baseStates {
		baseByPos[state.Pos] = state
		posSet[state.Pos] = struct{}{}
	}
	for _, state := range liveStates {
		liveByPos[state.Pos] = state
		posSet[state.Pos] = struct{}{}
	}
	if len(posSet) == 0 {
		sendBlockSnapshotsToConn(conn, wld)
		syncCoreItemsToConn(conn, wld)
		return
	}
	positions := make([]int32, 0, len(posSet))
	for pos := range posSet {
		positions = append(positions, pos)
	}
	sort.Slice(positions, func(i, j int) bool { return positions[i] < positions[j] })

	health := make([]int32, 0, 256)
	changedPacked := make([]int32, 0, 256)
	configs := make(map[int32]any, 64)
	runtimeChangedPacked := wld.RuntimeChangedBlockSyncPackedPositions(baseModel)
	for _, pos := range positions {
		baseState, baseOK := baseByPos[pos]
		liveState, liveOK := liveByPos[pos]
		var cfgValue any
		cfgOK := false
		if liveOK && liveState.BlockID > 0 {
			if liveCfg, ok := wld.BuildingConfigPacked(pos); ok {
				cfgValue = liveCfg
				cfgOK = true
			} else if len(liveState.Config) > 0 {
				cfgValue, cfgOK = decodeTileConfigBytes(liveState.Config)
			}
			if cfgOK && shouldSendTileConfigForPacked(wld, pos, cfgValue) {
				configs[pos] = cfgValue
			}
		}
		structuralChange := !baseOK || !liveOK ||
			baseState.BlockID != liveState.BlockID ||
			baseState.Team != liveState.Team ||
			baseState.Rotation != liveState.Rotation
		healthChanged := !(baseOK && liveOK && sameBuildHealth(baseState.Health, liveState.Health))
		configChanged := !(baseOK && liveOK && sameConfigBytes(baseState.Config, liveState.Config))
		if !structuralChange && !healthChanged && !configChanged {
			continue
		}

		if !liveOK || liveState.BlockID <= 0 {
			if baseOK && baseState.BlockID > 0 {
				_ = conn.SendAsync(&protocol.Remote_Tile_buildDestroyed_143{
					Build: protocol.BuildingBox{PosValue: pos},
				})
			}
			_ = conn.SendAsync(&protocol.Remote_Tile_setTile_140{
				Tile:     protocol.TileBox{PosValue: pos},
				Block:    protocol.BlockRef{BlkID: 0, BlkName: ""},
				Team:     protocol.Team{ID: 0},
				Rotation: 0,
			})
			blockID := int16(0)
			if baseOK {
				blockID = baseState.BlockID
			}
			_ = conn.SendAsync(&protocol.Remote_ConstructBlock_deconstructFinish_145{
				Tile:    protocol.TileBox{PosValue: pos},
				Block:   protocol.BlockRef{BlkID: blockID, BlkName: ""},
				Builder: nil,
			})
			changedPacked = append(changedPacked, pos)
			continue
		}

		if !structuralChange {
			if healthChanged {
				hp := liveState.Health
				if hp <= 0 {
					hp = 1000
				}
				health = append(health, pos, int32(math.Float32bits(hp)))
				if len(health) >= 256 {
					_ = conn.SendAsync(&protocol.Remote_Tile_buildHealthUpdate_144{
						Buildings: protocol.IntSeq{Items: append([]int32(nil), health...)},
					})
					health = health[:0]
				}
			}
			changedPacked = append(changedPacked, pos)
			continue
		}

		if structuralChange && (modelTileIsConstructLike(baseModel, pos) || !baseOK || baseState.BlockID <= 0) {
			// Join-time correction can target tiles that were air in the template but
			// already exist live on the authoritative server. Finish any client-side
			// construct state before setTile so later blockSnapshot bytes apply to the
			// final building instead of a lingering build2 placeholder.
			_ = conn.SendAsync(&protocol.Remote_ConstructBlock_constructFinish_146{
				Tile:     protocol.TileBox{PosValue: pos},
				Block:    protocol.BlockRef{BlkID: liveState.BlockID, BlkName: ""},
				Builder:  nil,
				Rotation: liveState.Rotation & 0x3,
				Team:     protocol.Team{ID: byte(liveState.Team)},
				Config:   cfgValue,
			})
		}
		_ = conn.SendAsync(&protocol.Remote_Tile_setTile_140{
			Tile:     protocol.TileBox{PosValue: pos},
			Block:    protocol.BlockRef{BlkID: liveState.BlockID, BlkName: ""},
			Team:     protocol.Team{ID: byte(liveState.Team)},
			Rotation: int32(liveState.Rotation) & 0x3,
		})
		hp := liveState.Health
		if hp <= 0 {
			hp = 1000
		}
		health = append(health, pos, int32(math.Float32bits(hp)))
		changedPacked = append(changedPacked, pos)
		if len(health) >= 256 {
			_ = conn.SendAsync(&protocol.Remote_Tile_buildHealthUpdate_144{
				Buildings: protocol.IntSeq{Items: append([]int32(nil), health...)},
			})
			health = health[:0]
		}
	}
	if len(health) > 0 {
		_ = conn.SendAsync(&protocol.Remote_Tile_buildHealthUpdate_144{
			Buildings: protocol.IntSeq{Items: health},
		})
	}
	changedPacked = append(changedPacked, runtimeChangedPacked...)
	sendBlockSnapshotsForPackedToConn(conn, wld, expandRelatedBlockSyncPackedPositions(wld, changedPacked))
	syncCoreItemsToConn(conn, wld)
	sendTileConfigMapToConn(conn, configs)
}

