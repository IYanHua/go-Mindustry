package main

import (
	"sync"
	"sync/atomic"

	netserver "github.com/IYanHua/mdt-server/internal/net"
	"github.com/IYanHua/mdt-server/internal/persist"
	"github.com/IYanHua/mdt-server/internal/world"
)

// App 聚合所有运行时状态，作为未来依赖注入重构的基础。
// 当前仍通过包级变量访问，逐步迁移到 App 方法。
type App struct {
	// 网络服务器
	Server *netserver.Server

	// 世界实例
	World *world.World

	// 个性化配置（原子操作，供 goroutine 读取）
	PlayerNameColorEnabled    atomic.Bool
	PublicConnUUIDEnabled     atomic.Bool
	JoinLeaveChatEnabled      atomic.Bool
	PlayerNamePrefix          atomic.Value
	PlayerNameSuffix          atomic.Value
	PlayerBindPrefixEnabled   atomic.Bool
	PlayerBoundPrefix         atomic.Value
	PlayerUnboundPrefix       atomic.Value
	PlayerTitleEnabled        atomic.Bool
	PlayerConnIDSuffixEnabled atomic.Bool
	PlayerConnIDSuffixFormat  atomic.Value

	// 持久化存储
	PublicConnUUIDStore *persist.PublicConnUUIDStore
	PlayerIdentityStore *persist.PlayerIdentityStore
	BindStatusResolver  *bindStatusResolver

	// 方块名翻译
	BlockNameTranslationMu sync.RWMutex
	BlockNameTranslations  map[string]string

	// 路径
	WorldRoots []string
	AssetsDir  string
	ConfigDir  string
	BaseDir    string
	WorldPath  atomic.Value
}
