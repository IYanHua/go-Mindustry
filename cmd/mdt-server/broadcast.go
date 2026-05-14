package main

import (
	netserver "github.com/IYanHua/mdt-server/internal/net"
	"github.com/IYanHua/mdt-server/internal/protocol"
	"github.com/IYanHua/mdt-server/internal/world"
)

func broadcastSummonVisible(srv *netserver.Server, typeID int16, x, y float32, team byte) {
	if srv == nil {
		return
	}
	_ = team
	_ = typeID
	_ = x
	_ = y
	// Disabled for custom-client compatibility: this packet ID is not mapped yet
	// and can be misread as another call packet (causing client crash).
}

func broadcastUnitDestroy(srv *netserver.Server, entityID int32) {
	if srv == nil || entityID == 0 {
		return
	}
	srv.Broadcast(&protocol.Remote_Units_unitDestroy_55{Uid: entityID})
}

func unpackTilePos(pos int32) (int32, int32) {
	return int32(uint16((pos >> 16) & 0xFFFF)), int32(uint16(pos & 0xFFFF))
}

func broadcastSetTile(srv *netserver.Server, buildPos int32, blockID int16, rot int8, team byte) {
	if srv == nil || buildPos < 0 || blockID < 0 {
		return
	}
	srv.Broadcast(&protocol.Remote_Tile_setTile_140{
		Tile:     protocol.TileBox{PosValue: buildPos},
		Block:    protocol.BlockRef{BlkID: blockID, BlkName: ""},
		Team:     protocol.Team{ID: team},
		Rotation: int32(rot) & 0x3,
	})
}

func builderUnitForOwner(srv *netserver.Server, owner int32) protocol.Unit {
	if srv == nil || owner == 0 {
		return nil
	}
	for _, conn := range srv.ListConnectedConns() {
		if conn == nil || conn.PlayerID() != owner || conn.UnitID() == 0 {
			continue
		}
		return protocol.UnitBox{IDValue: conn.UnitID()}
	}
	return nil
}

func buildConstructConfigValue(wld *world.World, ev world.EntityEvent) any {
	if wld == nil {
		return ev.BuildConfig
	}
	if cfgValue, ok := wld.BuildingConfigPacked(ev.BuildPos); ok {
		return cfgValue
	}
	return ev.BuildConfig
}

func broadcastBuildConstructedState(srv *netserver.Server, wld *world.World, ev world.EntityEvent) {
	if srv == nil || ev.BuildPos < 0 || ev.BuildBlock <= 0 {
		return
	}
	cfgValue := buildConstructConfigValue(wld, ev)
	// Finish the client's ConstructBuild first. Sending setTile before this can
	// leave the client stuck on build2 and cause later blockSnapshot mismatches.
	broadcastConstructFinish(srv, ev.BuildPos, ev.BuildBlock, ev.BuildRot, byte(ev.BuildTeam), builderUnitForOwner(srv, ev.BuildOwner), cfgValue)
	broadcastSetTile(srv, ev.BuildPos, ev.BuildBlock, ev.BuildRot, byte(ev.BuildTeam))
	if cfgValue != nil {
		srv.BroadcastTileConfig(ev.BuildPos, cfgValue, nil)
	}
	broadcastRelatedBlockSnapshots(srv, wld, ev.BuildPos)
}

func broadcastBuildBeginPlace(srv *netserver.Server, buildPos int32, blockID int16, rot int8, team byte, config any) {
	if srv == nil || buildPos < 0 || blockID <= 0 {
		return
	}
	x, y := unpackTilePos(buildPos)
	srv.Broadcast(&protocol.Remote_Build_beginPlace_133{
		Unit:        nil,
		Result:      protocol.BlockRef{BlkID: blockID, BlkName: ""},
		Team:        protocol.Team{ID: team},
		X:           x,
		Y:           y,
		Rotation:    int32(rot) & 0x3,
		PlaceConfig: config,
	})
}

func broadcastBuildDeconstructBegin(srv *netserver.Server, buildPos int32, team byte) {
	if srv == nil || buildPos < 0 {
		return
	}
	x, y := unpackTilePos(buildPos)
	srv.Broadcast(&protocol.Remote_Build_beginBreak_132{
		Unit: nil,
		Team: protocol.Team{ID: team},
		X:    x,
		Y:    y,
	})
}

func broadcastBuildDestroyed(srv *netserver.Server, buildPos int32, blockID int16) {
	if srv == nil || buildPos < 0 {
		return
	}
	if blockID < 0 {
		blockID = 0
	}
	srv.Broadcast(&protocol.Remote_ConstructBlock_deconstructFinish_145{
		Tile:    protocol.TileBox{PosValue: buildPos},
		Block:   protocol.BlockRef{BlkID: blockID, BlkName: ""},
		Builder: nil,
	})
}

func broadcastTileBuildDestroyed(srv *netserver.Server, buildPos int32) {
	if srv == nil || buildPos < 0 {
		return
	}
	srv.Broadcast(&protocol.Remote_Tile_buildDestroyed_143{
		Build: protocol.BuildingBox{PosValue: buildPos},
	})
}

func broadcastBuildHealthUpdate(srv *netserver.Server, items []int32) {
	if srv == nil || len(items) == 0 {
		return
	}
	srv.Broadcast(&protocol.Remote_Tile_buildHealthUpdate_144{
		Buildings: protocol.IntSeq{Items: items},
	})
}

func broadcastEffectReliable(srv *netserver.Server, effectID int16, x, y, rotation float32) {
	if srv == nil || effectID < 0 {
		return
	}
	srv.Broadcast(&protocol.Remote_NetClient_effectReliable_13{
		Effect:   protocol.Effect{ID: effectID},
		X:        x,
		Y:        y,
		Rotation: rotation,
		Color:    protocol.Color{RGBA: -1},
	})
}

func broadcastTakeItems(srv *netserver.Server, buildPos int32, itemID int16, amount int32, unitID int32) {
	if srv == nil || buildPos < 0 || itemID < 0 || amount <= 0 || unitID == 0 {
		return
	}
	srv.BroadcastUnreliable(&protocol.Remote_InputHandler_takeItems_61{
		Build:  protocol.BuildingBox{PosValue: buildPos},
		Item:   protocol.ItemRef{ItmID: itemID},
		Amount: amount,
		To:     protocol.UnitBox{IDValue: unitID},
	})
}

func broadcastTransferItemToUnit(srv *netserver.Server, itemID int16, x, y float32, unitID int32) {
	if srv == nil || itemID < 0 || unitID == 0 {
		return
	}
	srv.BroadcastUnreliable(&protocol.Remote_InputHandler_transferItemToUnit_62{
		Item: protocol.ItemRef{ItmID: itemID},
		X:    x,
		Y:    y,
		To:   &protocol.EntityBox{IDValue: unitID},
	})
}

func broadcastTransferItemTo(srv *netserver.Server, unitID int32, itemID int16, amount int32, x, y float32, buildPos int32) {
	if srv == nil || unitID == 0 || buildPos < 0 || itemID < 0 || amount <= 0 {
		return
	}
	srv.BroadcastUnreliable(&protocol.Remote_InputHandler_transferItemTo_71{
		Unit:   protocol.UnitBox{IDValue: unitID},
		Item:   protocol.ItemRef{ItmID: itemID},
		Amount: amount,
		X:      x,
		Y:      y,
		Build:  protocol.BuildingBox{PosValue: buildPos},
	})
}

func broadcastDropItem(srv *netserver.Server, playerID int32, angle float32) {
	if srv == nil || playerID == 0 {
		return
	}
	srv.BroadcastUnreliable(&protocol.Remote_InputHandler_dropItem_88{
		Player: &protocol.EntityBox{IDValue: playerID},
		Angle:  angle,
	})
}

const maxBlockSnapshotPayloadBytes = 800

