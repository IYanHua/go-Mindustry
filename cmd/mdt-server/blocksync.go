package main

import (
	stdlog "log"
	"math"
	"path/filepath"
	"sort"
	"strings"
	netserver "github.com/IYanHua/mdt-server/internal/net"
	"github.com/IYanHua/mdt-server/internal/protocol"
	"github.com/IYanHua/mdt-server/internal/world"
)

func encodeBlockSnapshotEntry(snap world.BlockSyncSnapshot) ([]byte, bool) {
	if snap.BlockID <= 0 || len(snap.Data) == 0 {
		return nil, false
	}
	writer := protocol.NewWriter()
	_ = writer.WriteInt32(snap.Pos)
	_ = writer.WriteInt16(snap.BlockID)
	_ = writer.WriteBytes(snap.Data)
	return append([]byte(nil), writer.Bytes()...), true
}

func buildIsolatedBlockSnapshotPackets(snaps []world.BlockSyncSnapshot) []*protocol.Remote_NetClient_blockSnapshot_34 {
	if len(snaps) == 0 {
		return nil
	}
	packets := make([]*protocol.Remote_NetClient_blockSnapshot_34, 0, len(snaps))
	for _, snap := range snaps {
		entry, ok := encodeBlockSnapshotEntry(snap)
		if !ok {
			continue
		}
		packets = append(packets, &protocol.Remote_NetClient_blockSnapshot_34{
			Amount: 1,
			Data:   entry,
		})
	}
	return packets
}

func buildBlockSnapshotPackets(snaps []world.BlockSyncSnapshot) []*protocol.Remote_NetClient_blockSnapshot_34 {
	if len(snaps) == 0 {
		return nil
	}
	packets := make([]*protocol.Remote_NetClient_blockSnapshot_34, 0, (len(snaps)/12)+1)
	writer := protocol.NewWriter()
	amount := int16(0)
	flush := func() {
		if amount <= 0 {
			return
		}
		packets = append(packets, &protocol.Remote_NetClient_blockSnapshot_34{
			Amount: amount,
			Data:   append([]byte(nil), writer.Bytes()...),
		})
		writer = protocol.NewWriter()
		amount = 0
	}
	for _, snap := range snaps {
		entry, ok := encodeBlockSnapshotEntry(snap)
		if !ok {
			continue
		}
		if len(entry) > maxBlockSnapshotPayloadBytes {
			flush()
			packets = append(packets, &protocol.Remote_NetClient_blockSnapshot_34{
				Amount: 1,
				Data:   entry,
			})
			continue
		}
		if amount > 0 && len(writer.Bytes())+len(entry) > maxBlockSnapshotPayloadBytes {
			flush()
		}
		_ = writer.WriteBytes(entry)
		amount++
	}
	flush()
	return packets
}

func currentWorldPathLooksHidden(path string) bool {
	path = strings.ToLower(strings.TrimSpace(filepath.ToSlash(path)))
	return strings.Contains(path, "/hidden/")
}

func currentRuntimeWorldPath() string {
	if v := runtimeWorldPath.Load(); v != nil {
		if path, ok := v.(string); ok {
			return path
		}
	}
	return ""
}

func filterBlockSnapshotsForViewerTeam(wld *world.World, snaps []world.BlockSyncSnapshot, viewerTeam byte) []world.BlockSyncSnapshot {
	if wld == nil || len(snaps) == 0 {
		return snaps
	}
	if viewerTeam == 0 {
		return nil
	}
	out := make([]world.BlockSyncSnapshot, 0, len(snaps))
	for _, snap := range snaps {
		info, ok := wld.BuildingInfoPacked(snap.Pos)
		if !ok {
			continue
		}
		if info.Team != 0 && byte(info.Team) != viewerTeam {
			continue
		}
		out = append(out, snap)
	}
	return out
}

func sendFilteredBlockSnapshotsToConn(conn *netserver.Conn, wld *world.World, snaps []world.BlockSyncSnapshot) {
	if conn == nil || wld == nil || len(snaps) == 0 {
		return
	}
	if currentWorldPathLooksHidden(currentRuntimeWorldPath()) {
		snaps = filterBlockSnapshotsForViewerTeam(wld, snaps, conn.TeamID())
	}
	for _, packet := range buildBlockSnapshotPackets(snaps) {
		_ = conn.SendAsync(packet)
	}
}

func sendIsolatedFilteredBlockSnapshotsToConn(conn *netserver.Conn, wld *world.World, snaps []world.BlockSyncSnapshot) {
	if conn == nil || wld == nil || len(snaps) == 0 {
		return
	}
	if currentWorldPathLooksHidden(currentRuntimeWorldPath()) {
		snaps = filterBlockSnapshotsForViewerTeam(wld, snaps, conn.TeamID())
	}
	for _, packet := range buildIsolatedBlockSnapshotPackets(snaps) {
		_ = conn.SendAsync(packet)
	}
}

func broadcastFilteredBlockSnapshots(srv *netserver.Server, wld *world.World, snaps []world.BlockSyncSnapshot, unreliable bool) {
	if srv == nil || wld == nil || len(snaps) == 0 {
		return
	}
	if currentWorldPathLooksHidden(currentRuntimeWorldPath()) {
		for _, conn := range srv.ListConnectedConns() {
			if conn == nil || conn.InWorldReloadGrace() {
				continue
			}
			filtered := filterBlockSnapshotsForViewerTeam(wld, snaps, conn.TeamID())
			for _, packet := range buildBlockSnapshotPackets(filtered) {
				_ = conn.SendAsync(packet)
			}
		}
		return
	}
	for _, packet := range buildBlockSnapshotPackets(snaps) {
		if unreliable {
			srv.BroadcastUnreliable(packet)
		} else {
			srv.Broadcast(packet)
		}
	}
}

func sendBlockSnapshotsToConn(conn *netserver.Conn, wld *world.World) {
	if conn == nil || wld == nil {
		return
	}
	sendFilteredBlockSnapshotsToConn(conn, wld, wld.BlockSyncSnapshotsLiveOnly())
	sendFilteredBlockSnapshotsToConn(conn, wld, wld.ItemTurretBlockSyncSnapshotsLiveOnly())
}

func newTileConfigPacket(pos int32, value any) (*protocol.Remote_InputHandler_tileConfig_90, bool) {
	clonedValue, err := protocol.CloneObjectValue(value)
	if err != nil {
		stdlog.Printf("[tileconfig] skip pos=%d err=%v type=%T", pos, err, value)
		return nil, false
	}
	return &protocol.Remote_InputHandler_tileConfig_90{
		Build: protocol.BuildingBox{PosValue: pos},
		Value: clonedValue,
	}, true
}

func sendTileConfigMapToConn(conn *netserver.Conn, configs map[int32]any) {
	if conn == nil || len(configs) == 0 {
		return
	}
	positions := make([]int32, 0, len(configs))
	for pos := range configs {
		positions = append(positions, pos)
	}
	sort.Slice(positions, func(i, j int) bool { return positions[i] < positions[j] })
	for _, pos := range positions {
		if packet, ok := newTileConfigPacket(pos, configs[pos]); ok {
			_ = conn.SendAsync(packet)
		}
	}
}

func shouldSendTileConfigForPacked(wld *world.World, pos int32, value any) bool {
	if wld == nil || value == nil {
		return false
	}
	if !isSupportedTileConfigValue(value) {
		return false
	}
	info, ok := wld.BuildingInfoPacked(pos)
	if !ok {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(info.Name)) {
	case "item-source",
		"liquid-source",
		"sorter",
		"inverted-sorter",
		"duct-router",
		"surge-router",
		"unloader",
		"duct-unloader",
		"payload-router",
		"reinforced-payload-router",
		"power-node",
		"power-node-large",
		"surge-tower",
		"beam-link",
		"power-source",
		"bridge-conveyor",
		"phase-conveyor",
		"bridge-conduit",
		"phase-conduit",
		"mass-driver",
		"payload-mass-driver",
		"large-payload-mass-driver":
		return true
	default:
		return false
	}
}

func isSupportedTileConfigValue(value any) bool {
	switch value.(type) {
	case protocol.ItemRef,
		protocol.Point2,
		[]protocol.Point2,
		protocol.Content:
		return true
	default:
		return false
	}
}

func sendBlockSnapshotsForPackedToConn(conn *netserver.Conn, wld *world.World, packedPositions []int32) {
	if conn == nil || wld == nil || len(packedPositions) == 0 {
		return
	}
	sendFilteredBlockSnapshotsToConn(conn, wld, wld.BlockSyncSnapshotsForPackedLiveOnly(packedPositions))
	sendFilteredBlockSnapshotsToConn(conn, wld, wld.ItemTurretBlockSyncSnapshotsForPackedLiveOnly(packedPositions))
}

func expandRelatedBlockSyncPackedPositions(wld *world.World, packedPositions []int32) []int32 {
	if wld == nil || len(packedPositions) == 0 {
		return nil
	}
	seen := make(map[int32]struct{}, len(packedPositions)*2)
	out := make([]int32, 0, len(packedPositions)*2)
	add := func(pos int32) {
		if pos < 0 {
			return
		}
		if _, ok := seen[pos]; ok {
			return
		}
		seen[pos] = struct{}{}
		out = append(out, pos)
	}
	for _, packed := range packedPositions {
		add(packed)
		for _, related := range wld.RelatedBlockSyncPackedPositions(packed) {
			add(related)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func sendRequestedBlockSnapshotToConn(conn *netserver.Conn, wld *world.World, pos int32) {
	if conn == nil || wld == nil || pos < 0 {
		return
	}
	// Match vanilla NetServer.requestBlockSnapshot(): only send writeSync bytes for
	// the requested building. Re-sending construct/setTile here races blockSnapshot
	// against fresh build creation and can wipe client-side inventory/ammo views.
	sendIsolatedFilteredBlockSnapshotsToConn(conn, wld, wld.BlockSyncSnapshotsForPackedLiveOnly([]int32{pos}))
	sendIsolatedFilteredBlockSnapshotsToConn(conn, wld, wld.ItemTurretBlockSyncSnapshotsForPackedLiveOnly([]int32{pos}))
}

func authoritativeTileStatePacketsForPacked(wld *world.World, pos int32) []any {
	if wld == nil || pos < 0 {
		return nil
	}
	if wld.BlockSyncSuppressedPacked(pos) {
		return []any{
			&protocol.Remote_Tile_buildDestroyed_143{
				Build: protocol.BuildingBox{PosValue: pos},
			},
			&protocol.Remote_Tile_setTile_140{
				Tile:     protocol.TileBox{PosValue: pos},
				Block:    protocol.BlockRef{BlkID: 0, BlkName: ""},
				Team:     protocol.Team{ID: 0},
				Rotation: 0,
			},
			&protocol.Remote_ConstructBlock_deconstructFinish_145{
				Tile:    protocol.TileBox{PosValue: pos},
				Block:   protocol.BlockRef{BlkID: 0, BlkName: ""},
				Builder: nil,
			},
		}
	}
	liveModel := wld.CloneModel()
	if liveModel == nil {
		return nil
	}
	tile, ok := tileAtPacked(liveModel, pos)
	if !ok {
		return nil
	}
	if tile == nil || tile.Block <= 0 || tile.Build == nil {
		return []any{
			&protocol.Remote_Tile_buildDestroyed_143{
				Build: protocol.BuildingBox{PosValue: pos},
			},
			&protocol.Remote_Tile_setTile_140{
				Tile:     protocol.TileBox{PosValue: pos},
				Block:    protocol.BlockRef{BlkID: 0, BlkName: ""},
				Team:     protocol.Team{ID: 0},
				Rotation: 0,
			},
			&protocol.Remote_ConstructBlock_deconstructFinish_145{
				Tile:    protocol.TileBox{PosValue: pos},
				Block:   protocol.BlockRef{BlkID: 0, BlkName: ""},
				Builder: nil,
			},
		}
	}

	team := tile.Team
	if tile.Build != nil {
		team = tile.Build.Team
	}
	hp := tile.Build.Health
	if hp <= 0 {
		hp = tile.Build.MaxHealth
	}
	if hp <= 0 {
		hp = 1000
	}

	cfgValue, cfgOK := wld.BuildingConfigPacked(pos)
	packets := []any{
		&protocol.Remote_ConstructBlock_constructFinish_146{
			Tile:     protocol.TileBox{PosValue: pos},
			Block:    protocol.BlockRef{BlkID: int16(tile.Block), BlkName: ""},
			Builder:  nil,
			Rotation: tile.Rotation & 0x3,
			Team:     protocol.Team{ID: byte(team)},
			Config:   cfgValue,
		},
		&protocol.Remote_Tile_setTile_140{
			Tile:     protocol.TileBox{PosValue: pos},
			Block:    protocol.BlockRef{BlkID: int16(tile.Block), BlkName: ""},
			Team:     protocol.Team{ID: byte(team)},
			Rotation: int32(tile.Rotation) & 0x3,
		},
		&protocol.Remote_Tile_buildHealthUpdate_144{
			Buildings: protocol.IntSeq{
				Items: []int32{pos, int32(math.Float32bits(hp))},
			},
		},
	}
	if cfgOK {
		if packet, ok := newTileConfigPacket(pos, cfgValue); ok {
			packets = append(packets, packet)
		}
	}
	for _, packet := range buildBlockSnapshotPackets(wld.BlockSyncSnapshotsForPackedLiveOnly([]int32{pos})) {
		packets = append(packets, packet)
	}
	for _, packet := range buildBlockSnapshotPackets(wld.ItemTurretBlockSyncSnapshotsForPackedLiveOnly([]int32{pos})) {
		packets = append(packets, packet)
	}
	return packets
}

func sendAuthoritativeTileStateToConn(conn *netserver.Conn, wld *world.World, pos int32) {
	if conn == nil || wld == nil {
		return
	}
	for _, packet := range authoritativeTileStatePacketsForPacked(wld, pos) {
		_ = conn.SendAsync(packet)
	}
	sendSharedTeamItemStateForPackedToConn(conn, wld, pos)
}

func sharedTeamItemStatePacketsForPacked(wld *world.World, pos int32) []any {
	// Official NetServer does not emit InputHandler.setTileItems packets for
	// corrective sync. Keeping this path disabled avoids overwriting client item
	// modules with stale team totals.
	_ = wld
	_ = pos
	return nil
}

func sendSharedTeamItemStateForPackedToConn(conn *netserver.Conn, wld *world.World, pos int32) {
	if conn == nil || wld == nil {
		return
	}
	for _, packet := range sharedTeamItemStatePacketsForPacked(wld, pos) {
		_ = conn.SendAsync(packet)
	}
}

func broadcastBlockSnapshots(srv *netserver.Server, wld *world.World) {
	if srv == nil || wld == nil {
		return
	}
	// The world stream already delivered msav inline sync bytes on load/connect.
	// Periodic overlays must be generated from current runtime state only, or they
	// can replay stale map bytes back onto active clients.
	broadcastFilteredBlockSnapshots(srv, wld, wld.BlockSyncSnapshotsLiveOnly(), true)
	broadcastFilteredBlockSnapshots(srv, wld, wld.ItemTurretBlockSyncSnapshotsLiveOnly(), true)
}

func broadcastBlockSnapshotsForPacked(srv *netserver.Server, wld *world.World, packedPositions []int32) {
	if srv == nil || wld == nil || len(packedPositions) == 0 {
		return
	}
	broadcastFilteredBlockSnapshots(srv, wld, wld.BlockSyncSnapshotsForPackedLiveOnly(packedPositions), false)
}

func broadcastItemBlockSnapshotsForPacked(srv *netserver.Server, wld *world.World, packedPositions []int32) {
	if srv == nil || wld == nil || len(packedPositions) == 0 {
		return
	}
	broadcastFilteredBlockSnapshots(srv, wld, wld.BlockSyncSnapshotsForPackedLiveOnly(packedPositions), false)
}

func broadcastItemTurretAmmoSnapshotsForPacked(srv *netserver.Server, wld *world.World, packedPositions []int32) {
	if srv == nil || wld == nil || len(packedPositions) == 0 {
		return
	}
	if wld.BlockSyncLogsEnabled() {
		for _, packed := range packedPositions {
			stdlog.Printf("[turret-ammo] broadcast %s", wld.DebugItemTurretAmmoPacked(packed))
		}
	}
	broadcastFilteredBlockSnapshots(srv, wld, wld.ItemTurretBlockSyncSnapshotsForPackedLiveOnly(packedPositions), false)
}

func broadcastRelatedBlockSnapshots(srv *netserver.Server, wld *world.World, packedPos int32) {
	if srv == nil || wld == nil || packedPos < 0 {
		return
	}
	positions := wld.RelatedBlockSyncPackedPositions(packedPos)
	broadcastBlockSnapshotsForPacked(srv, wld, positions)
	broadcastItemTurretAmmoSnapshotsForPacked(srv, wld, positions)
}


