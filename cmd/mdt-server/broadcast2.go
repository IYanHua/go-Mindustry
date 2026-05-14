package main

import (
	netserver "github.com/IYanHua/mdt-server/internal/net"
	"github.com/IYanHua/mdt-server/internal/protocol"
	"github.com/IYanHua/mdt-server/internal/world"
)

func broadcastConstructFinish(srv *netserver.Server, buildPos int32, blockID int16, rot int8, team byte, builder protocol.Unit, config any) {
	if srv == nil || buildPos < 0 || blockID <= 0 {
		return
	}
	srv.Broadcast(&protocol.Remote_ConstructBlock_constructFinish_146{
		Tile:     protocol.TileBox{PosValue: buildPos},
		Block:    protocol.BlockRef{BlkID: blockID, BlkName: ""},
		Builder:  builder,
		Rotation: rot & 0x3,
		Team:     protocol.Team{ID: team},
		Config:   config,
	})
}

func broadcastBuildDestroyedState(srv *netserver.Server, ev world.EntityEvent) {
	if srv == nil || ev.BuildPos < 0 {
		return
	}
	broadcastTileBuildDestroyed(srv, ev.BuildPos)
	broadcastBuildDestroyed(srv, ev.BuildPos, ev.BuildBlock)
	broadcastSetTile(srv, ev.BuildPos, 0, 0, 0)
}

func bulletCreatePacketFromEvent(srv *netserver.Server, b world.BulletEvent) *protocol.Remote_BulletType_createBullet_58 {
	if srv == nil || b.BulletTyp < 0 {
		return nil
	}
	bulletType := srv.Content.BulletType(b.BulletTyp)
	if bulletType == nil {
		return nil
	}
	return &protocol.Remote_BulletType_createBullet_58{
		Type:        bulletType,
		Team:        protocol.Team{ID: byte(b.Team)},
		X:           b.X,
		Y:           b.Y,
		Angle:       b.Angle,
		Damage:      b.Damage,
		VelocityScl: 1,
		LifetimeScl: 1,
	}
}

func broadcastBulletCreate(srv *netserver.Server, b world.BulletEvent) {
	packet := bulletCreatePacketFromEvent(srv, b)
	if packet == nil {
		return
	}
	srv.BroadcastUnreliable(packet)
}

