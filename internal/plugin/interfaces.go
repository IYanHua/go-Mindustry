package plugin

import (
	"github.com/IYanHua/mdt-server/internal/world"
)

// ConnInterface 暴露插件所需的连接方法子集。
// 表示一个已连接的玩家。
type ConnInterface interface {
	// 标识
	ConnID() int32           // 连接唯一 ID
	UUID() string            // 玩家 UUID
	USID() string            // 玩家短 ID
	Name() string            // 玩家名称（含颜色标记）
	RemoteAddrString() string // 客户端网络地址

	// 状态
	IsConnected() bool       // 是否仍然连接
	TeamID() world.TeamID    // 所属队伍
	IsAdmin() bool           // 是否为管理员
}

// ServerInterface 暴露插件所需的服务器方法子集。
// 提供玩家管理、聊天、UI 弹窗等功能。
type ServerInterface interface {
	// 聊天与消息
	BroadcastChat(message string)
	SendChat(c ConnInterface, message string)
	SendChatToConn(connID int32, message string)
	OnChat(fn func(connID int32, message string) bool)

	// 连接管理
	ListConnectedConns() []ConnInterface
	ConnByID(id int32) ConnInterface
	KickByID(id int32, reason string) bool

	// 玩家信息
	ServerName() string
	PlayerDisplayName(c ConnInterface) string

	// 客户端 UI
	SendInfoPopup(c ConnInterface, message string, duration float32, align, top, left, bottom, right int32)
	SendMenu(c ConnInterface, menuID int32, title, message string, options [][]string)
	SendInfoMessage(c ConnInterface, message string)
	SendOpenURI(c ConnInterface, uri string)
	BroadcastSetHudTextReliable(message string)
	BroadcastHideHudText()

	// 世界
	MapName() string
	ReloadWorldLiveForAll() (int, int)

	// 统计
	GameTimeSeconds() float64
	SessionCount() int
}

// WorldInterface 暴露插件所需的世界模拟方法子集。
// 提供游戏状态、波次控制等功能。
type WorldInterface interface {
	// 快照
	Snapshot() world.Snapshot

	// 游戏状态
	IsPaused() bool
	SetPaused(paused bool)
	IsGameOver() bool
	SetGameOver(v bool)

	// 世界时间
	TickNumber() int64   // 当前 tick 编号
	WaveTime() float32   // 当前波次内时间（秒）

	// 波次控制
	CurrentWave() int32
	TriggerWave()
	GetWaveManager() *world.WaveManager

	// 队伍
	FillTeamCoreItems(team world.TeamID)
}
