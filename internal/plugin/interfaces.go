package plugin

import (
	"github.com/IYanHua/mdt-server/internal/world"
)

// ConnInterface 暴露插件所需的连接方法子集。
type ConnInterface interface {
	ConnID() int32
	UUID() string
	USID() string
	Name() string
	RemoteAddrString() string
	IsConnected() bool
	TeamID() world.TeamID
}

// ServerInterface 暴露插件所需的服务器方法子集。
type ServerInterface interface {
	BroadcastChat(message string)
	SendChatToConn(connID int32, message string)
	SendChat(c ConnInterface, message string)
	ListConnectedConns() []ConnInterface
	KickByID(id int32, reason string) bool
	ServerName() string
	PlayerDisplayName(c ConnInterface) string
	ReloadWorldLiveForAll() (int, int)
	OnChat(fn func(connID int32, message string) bool)
	SendInfoPopup(c ConnInterface, message string, duration float32, align, top, left, bottom, right int32)
	SendMenu(c ConnInterface, menuID int32, title, message string, options [][]string)
	SendInfoMessage(c ConnInterface, message string)
	SendOpenURI(c ConnInterface, uri string)
	MapName() string
	GameTimeSeconds() float64
	SessionCount() int
}

// WorldInterface 暴露插件所需的世界方法子集。
type WorldInterface interface {
	Snapshot() world.Snapshot
	SetPaused(paused bool)
	IsPaused() bool
	SetGameOver(v bool)
	IsGameOver() bool
	CurrentWave() int32
	TriggerWave()
	FillTeamCoreItems(team world.TeamID)
	GetWaveManager() *world.WaveManager
}
