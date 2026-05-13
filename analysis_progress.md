# go-Mindustry (mdt-server) 项目标准化重构 — 分析进度

> 最后更新: 2026-05-13 (第二轮)
> 当前状态: Phase 1 完成，Phase 2 基础完成，待继续深层重构

---

## 一、项目概览

| 项目 | 说明 |
|---|---|
| 名称 | mdt-server / go-Mindustry |
| 用途 | Mindustry 游戏服务器（Go 从零重写，兼容 build 157 协议） |
| 语言 | Go 1.22+ |
| 代码量 | 246 个 .go 文件，共 ~134,000 行 |
| 许可证 | GPL-3.0 |
| 作者 | 月月岛科技 (IYanHua) |
| 版本 | 1.3.9-demo |

### 目录结构

```
/root/go-Mindustry/
├── cmd/mdt-server/          # 主程序入口 (23 文件)
│   ├── main.go              # 7855 行 — 入口 + 世界同步 + 控制台 + 全部杂项
│   ├── mod.go               # Go plugin 加载器
│   ├── status_bar.go        # 状态栏
│   ├── join_popup.go        # 加入弹窗
│   ├── map_vote.go          # 投票换图
│   ├── unit_commands.go     # 单位命令 (active)
│   ├── unit_commands_fixed.go  # (build ignore — 废弃)
│   ├── unit_commands_v2.go     # (build ignore — 废弃)
│   ├── console_title*.go    # 跨平台控制台标题
│   ├── process_icon*.go     # 跨平台进程图标
│   ├── process_cpu*.go      # 跨平台 CPU 监控
│   └── *_test.go            # 测试文件 (6 个)
│
├── internal/
│   ├── api/                 # HTTP API (2 套实现，其中 1 套疑似废弃)
│   ├── bootstrap/           # 工作区初始化
│   ├── buildinfo/           # 构建版本信息
│   ├── buildsvc/            # 建筑服务
│   ├── config/              # TOML 配置（手写解析器 ~900 行）
│   ├── core/                # 四核心架构
│   ├── devlog/              # 开发者日志
│   ├── entity/              # 游戏实体
│   ├── events/              # 事件系统 (空壳，全 no-op)
│   ├── logging/             # 日志系统 (无接口，仅具体类型)
│   ├── logic/               # 逻辑系统 (lexer/parser/executor)
│   ├── net/                 # 网络核心 (6891 行 server.go)
│   ├── oracle/              # 预测/分析 (10 文件)
│   ├── persist/             # 数据持久化
│   ├── protocol/            # Mindustry 157 协议
│   ├── render/              # 体素渲染
│   ├── runtimeassets/       # 运行时资源
│   ├── sim/                 # 仿真引擎
│   ├── storage/             # 事件存储
│   ├── tracepoints/         # 追踪点
│   ├── vanilla/             # 原版数据配置
│   ├── video/               # 视频录制
│   ├── world/               # 游戏世界核心 (13362 行 world.go)
│   └── worldstream/         # 世界流编解码
│
├── configs/                 # TOML 配置文件
├── assets/worlds/           # 预置地图 (.msav)
├── custom_plugin/           # Go 插件子模块
├── mods/                    # 编译后的 .plugin 文件
├── data/                    # 运行数据
├── Center/                  # 版本/商务元信息
├── bundled.go              # //go:embed 嵌入式资源
├── go.mod                  # module mdt-server
├── go.work                 # Go 工作区
└── Makefile                # 构建系统
```

---

## 二、已发现的全部问题

### A. 单体巨型文件（需拆分）

| 文件 | 行数 | 问题 |
|---|---|---|
| `internal/world/world.go` | 13,362 | World struct ~90 字段，~891 方法。已按功能拆到多个文件，但 struct 本身仍然单体 |
| `cmd/mdt-server/main.go` | 7,855 | main() 函数 ~2,500 行，包含全部启动逻辑、回调注册、世界同步函数 |
| `internal/net/server.go` | 6,891 | Server struct ~200 字段，~219 方法。God object 模式 |
| `internal/protocol/remote_packets.go` | 6,327 | 自动生成代码，可移至独立目录 |

### B. 模块命名

- `go.mod`: `module mdt-server` — 不含域名前缀，不符合 Go 惯例
- 应为：`github.com/IYanHua/mdt-server` 或 `github.com/MonthZifang/mdt-server`
- `go.work` 同时引用 `.` 和 `./custom_plugin`，custom_plugin 的 `go.mod` 依赖主模块路径

### C. main 包中的类型（应移入 internal/）

**main.go 中 (~30 个类型)**:
- `worldState` (53行), `bindStatusCacheEntry` (58), `bindStatusResolver` (63) — 网络/绑定相关
- `detailedLogWriter` (252) — 日志相关
- `startupReport`, `startupItem`, `startupStatus` (725-753) — 启动报告
- `mapLoadStats` (823) — 地图加载
- `reactorExplosionGroup` (5158) — 世界事件
- `unitTypeRef`, `bulletTypeRef`, `itemRef`, `blockRef` (5325-5357) — 协议适配器
- `scriptPlugin` (4727), `scriptController` (6605) — 脚本管理
- `statusMonitor` (6879) — 资源监控
- `worldCache` (7568) — 世界缓存
- `buildConfigState` (6041), `snapshotLogKey` (2007) — 内部辅助

**其他 main 包文件中**:
- `mapVoteDecision`, `mapVoteResult`, `mapVoteSnapshot`, `mapVoteSession`, `mapVoteRuntime`, `mapVoteRuntimeConfig` (map_vote.go) — 应独立为 `internal/mapvote`
- `joinPopupRuntimeConfig`, `helpPage`, `helpCommandButton` (join_popup.go) — 应独立为 `internal/joinpopup`
- `statusBarRuntimeConfig` (status_bar.go) — 应独立为 `internal/statusbar`
- `unitCommandService`, `unitCommandTargetSpec`, `unitCommandRuntime` (unit_commands.go) — 应独立为 `internal/unitcmd`
- `processCPUTracker` 及 Windows 内存结构体 (process_cpu_windows.go) — 应独立为 `internal/platform`

### D. 全局可变状态

| 位置 | 变量 | 风险 |
|---|---|---|
| `internal/events/events.go` | `GlobalEventManager` | 高 — 所有方法为空操作（死代码），但 import 即可触发全局变量初始化 |
| `internal/net/commands.go` | `globalServer`, `globalCommandHandler` | 高 — 并行测试会竞态 |
| `internal/net/server.go` | `globalVerboseNetLog` | 中 — 原子操作，但仍为包级可变 |
| `internal/core/supervisor.go` | `spawnChildCoreProcessFn`, `closeChildCoreProcessFn` | 中 — 可替换函数变量（测试用） |
| `internal/config/config.go` | `tomlValueKinds` | 低 — 仅初始化一次 |
| `cmd/mdt-server/main.go` | 20+ `runtime*` 变量 (atomic.Value/Bool) | 高 — 大量包级可变状态 |

### E. 死代码 / 废弃代码

| 文件 | 说明 |
|---|---|
| `internal/api/endpoints.go` (277行) | 第二套 API 实现，与 server.go 功能重叠，中文注释，疑似未使用 |
| `internal/events/events.go` | EventManager 所有方法为空 (Dispatch, AddHook 等全部 no-op) |
| `cmd/mdt-server/unit_commands_fixed.go` (1047行) | `//go:build ignore` — 废弃变体 |
| `cmd/mdt-server/unit_commands_v2.go` (1016行) | `//go:build ignore` — 废弃变体 |
| `internal/api/notes.go`, `internal/net/notes.go`, `internal/storage/notes.go` | TODO 文件，无实际代码 |
| `internal/protocol/remote_packets.go` 内 120+ Remote 类型 | 标记为 TODO stubs (Read/Write 未实现) |

### F. 架构问题

1. **无接口设计**: decoupling 依赖 struct 上的函数指针 (如 `Server.OnBuildPlans`, `Server.SpawnUnitFn`)，没有任何核心接口
2. **无 context.Context**: 整个代码库不使用 `context.Context`，无超时/取消/追踪
3. **logging.Logger 无接口**: 为具体 struct，无法 mock
4. **IPC 仅 Windows**: `listenIPC()`/`dialIPC()` 在非 Windows 返回错误 "named pipe ipc is only supported on windows"
5. **手写 TOML 解析器**: `config.go` 中 ~900 行自定义解析器，未使用标准库
6. **Core3 缓存单元素**: snapshot 缓存只有一条记录，无驱逐策略
7. **无优雅关闭**: StopAll() 不等待 in-flight 工作完成
8. **无速率限制**: child_core.go 的 send 方法每消息启动 goroutine，无背压

### G. 工程化缺失

| 项目 | 状态 |
|---|---|
| CI/CD | 无 (.github/ 不存在) |
| Linter | 无 golangci 配置 |
| 构建标准化 | Makefile 仅基础命令，二进制固定命名为 .exe |
| 错误处理 | 使用 fmt.Errorf / errors.New，无错误包装/类型 |
| 日志接口 | 无接口，无法在测试中替换 |
| 测试覆盖 | 67 个测试文件，但无覆盖率报告 |

### H. 代码风格

- 注释混用中英文（api/endpoints.go 纯中文，api/server.go 纯英文，core/ 中英混合）
- 大量注释描述 "WHAT" 而非 "WHY"（如 `// EventManager 事件管理器`）
- 中文本地化数据混在代码中（block name translations）
- 标识符无中文（符合 Go 规范）

---

## 三、当前架构示意

```
┌──────────────────────────────────────────────────────┐
│  main.go (7855 行) — 启动编排 + 世界同步 + 控制台      │
│  ├── 20+ runtime* 全局变量                            │
│  ├── 30+ 类型定义 (应移入 internal/)                    │
│  └── 100+ 函数 (main, runConsole, build*, sync*, ...) │
└────────┬─────────────────────────────────────────────┘
         │
    ┌────┴──────────────┬──────────────┬──────────────┐
    │                   │              │              │
    ▼                   ▼              ▼              ▼
┌──────────┐  ┌──────────────┐  ┌──────────┐  ┌──────────┐
│ Core1    │  │ Core2 (IO)   │  │ Core3    │  │ Core4    │
│ 游戏循环  │  │ 网络/持久化    │  │ 快照缓存  │  │ 策略/限速  │
│ 60-120   │  │ 事件/Mod     │  │          │  │ 分片      │
│ TPS      │  │              │  │          │  │          │
└──────────┘  └──────┬───────┘  └──────────┘  └──────────┘
         │            │ (IPC Windows Only)
         ▼            ▼
┌──────────────────────────────────────────────────────┐
│  internal/net    — 网络层 (6891 行 Server god object)  │
│  internal/world  — 世界层 (13362 行 World god object)  │
│  internal/protocol — 协议层 (6327 行 自动生成 stubs)     │
└──────────────────────────────────────────────────────┘
```

---

## 四、已提议的重构阶段（待确认）

### Phase 1: 工程基础标准化 (低风险，高收益)
1. 修正模块路径 `mdt-server` → `github.com/IYanHua/mdt-server`
2. 配置 golangci-lint
3. 添加 GitHub Actions CI (build + test + lint)
4. 标准化二进制命名 (去掉 .exe)
5. 清理死代码 (unit_commands_fixed.go, unit_commands_v2.go, events/, api/endpoints.go, notes.go)

### Phase 2: 包结构重组 (中风险)
1. 拆分 main.go → App struct + 按功能分离文件
2. 将 main 包中的类型移入对应的 internal/ 包
3. 提取独立子包: `internal/mapvote`, `internal/joinpopup`, `internal/statusbar`, `internal/unitcmd`, `internal/platform`
4. 清理全局变量，使用依赖注入

### Phase 3: 架构改进 (高风险)
1. 为核心组件引入接口 (Logger, World, Server, Storage)
2. 引入 context.Context 传播
3. 拆分 World struct (按子系统: PowerSystem, TurretSystem, UnitSystem...)
4. 手写 TOML 解析器替换为标准库
5. IPC 跨平台支持 (Unix domain socket)

### Phase 4: 代码质量提升
1. 统一注释语言 (建议英文)
2. 统一错误处理模式
3. 补充测试覆盖率
4. 添加 graceful shutdown

---

## 五、下一步需要确认的问题

1. **模块路径**: 使用 `github.com/IYanHua/mdt-server` 还是 `github.com/MonthZifang/mdt-server`？
2. **重构范围**: 建议优先执行 Phase 1+2，Phase 3 需要逐项确认。是否同意这个优先级？
3. **IPC 跨平台**: 是否需要 Linux/macOS 的多核心进程支持？（目前仅 Windows）
4. **TOML 解析器**: 是否接受引入第三方库 (BurntSushi/toml)，还是保留手写解析器？
5. **注释语言**: 统一为英文还是保留中文？
6. **远程包 (remote_packets.go)**: 120+ 个 stub 类型是否需要实际实现还是维持现状？

---

## 六、关键文件路径速查

| 文件 | 用途 |
|---|---|
| `cmd/mdt-server/main.go` | 程序入口，7855 行 |
| `cmd/mdt-server/mod.go` | Plugin 加载器 |
| `cmd/mdt-server/unit_commands.go` | 单位 AI 命令 |
| `internal/api/server.go` | 主 HTTP API (822 行) |
| `internal/api/endpoints.go` | 疑似废弃的 API 实现 |
| `internal/config/config.go` | 配置 + 手写 TOML 解析器 (2393 行) |
| `internal/core/core.go` | Core1+Core2+Core3+Core4 定义 (964 行) |
| `internal/core/server_core.go` | ServerCore 编排器 |
| `internal/net/server.go` | 网络 Server god object (6891 行) |
| `internal/protocol/packet.go` | Packet 接口定义 (69 行) |
| `internal/protocol/remote_packets.go` | 自动生成的远程包 (6327 行) |
| `internal/world/world.go` | World god object (13362 行) |
| `internal/events/events.go` | 死代码 EventManager |
| `bundled.go` | //go:embed 嵌入式资源 |
| `go.mod` | 模块定义 |
| `go.work` | Go 工作区 |
| `Makefile` | 构建脚本 |

---

## 七、已完成的修改（2026-05-13 第二轮会话）

### ✅ Phase 1 全部完成

| 任务 | 详情 |
|---|---|
| 模块路径修正 | `mdt-server` → `github.com/MonthZifang/mdt-server`（go.mod, go.work, custom_plugin/go.mod, 所有 ~90+ 个 .go 文件 import 路径） |
| 死代码清理 | 删除: `unit_commands_fixed.go`, `unit_commands_v2.go`, `internal/events/` (整个包), `internal/api/endpoints.go`, `internal/api/notes.go`, `internal/net/notes.go`, `internal/storage/notes.go` |
| 构建标准化 | Makefile 重写: 平台自适应二进制命名、build-plugin 目标、test-cover、lint、fmt、tidy |
| CI/CD | 创建 `.github/workflows/ci.yml` (build + test + lint) 和 `.golangci.yml` |
| writerJSON 修复 | 删除 endpoints.go 后，在 api/server.go 末尾补充 writeJSON 辅助函数 |

### ✅ Phase 2 基础完成

| 任务 | 详情 |
|---|---|
| App struct | 创建 `cmd/mdt-server/app.go`，聚合所有运行时状态（原子变量、持久化存储、路径等），为后续依赖注入重构奠定基础 |

### ⏳ Phase 2 遗留（需要后续会话）

| 任务 | 原因 |
|---|---|
| main.go 拆分 (console.go / sync.go) | 7855 行文件需仔细按行范围提取，手动操作高风险。建议使用 Go AST 工具辅助拆分 |
| 类型搬迁 (mapvote/joinpopup/statusbar/unitcmd 等) | 这些类型深度耦合 main 包全局变量，需要先完成 App struct 迁移才能搬迁 |
| 全局变量 → App 注入 | 需要穿透 ~200 个函数引用，逐层重构 |
| 平台代码搬迁 (process_cpu, process_icon, console_title) | 构建标签 + 紧密耦合，需要小心处理 |

### 测试状态

- `go build ./...` ✅ 通过
- `go test ./...` — 4 个预先存在的失败（路径分隔符 Linux/Windows 差异 + 缺失测试数据文件），非本次引入
  - `internal/oracle` — TestJavaLauncherSourceMatchesToolMirror (missing Java file)
  - `internal/runtimeassets` — TestBootstrapWorldCandidates* (Windows backslash vs forward slash)
  - `internal/worldstream` — TestBuildWorldStreamFromWeatheredChannels* (missing .msav file)

### 后续建议

1. 使用 `gorename` 或 Go AST 工具自动化 main.go 拆分（减少手动错误）
2. 优先将全局变量迁移到 App struct 字段（渐进式，每次 3-5 个变量）
3. 完成后才能搬迁 mapvote/joinpopup/statusbar 到 internal/ 子包
4. 修复预先存在的 Windows/Linux 路径测试失败

---

## 八、P0/P1 修复 — 2026-05-13 第三轮会话

### 🔍 现场勘查结果

对 java_go_comparison.md 诊断报告进行逐项验证后发现，Go 实现远比初始评估完整：

| 诊断项目 | 初始判断 | 实际状态 |
|---------|---------|---------|
| 服务器发现 (UDP 组播) | ❌ 缺失 | ✅ `server.go:3413` `serveUDP()` 完整实现 |
| remote_packets.go stub | ❌ 120+ stub | ✅ 154 Read/Write 全部实现，仅 connectConfirm 无操作（正确） |
| 连接握手 USID | ❌ 缺失 | ✅ `server.go:3800-3864` 完整提取+存储 |
| CRC32 UUID 校验 | ❌ 缺失 | ✅ `packets.go:165-174` 写端完整，读端正确消费 |
| Mod 兼容检查 | ❌ 缺失 | ✅ `admission.go:343` `incompatibleModsMessage` + `server.go:3843` |
| 包优先级排队 | ❌ 缺失 | ✅ `server.go:3277-3332` `SendAsyncPriority` + 双通道 sendLoop |
| 速率限制 | ❌ 缺失 | ✅ `policy_core.go:187` `AllowPacket` 滑动窗口 (4096/5s) |
| 位置验证反作弊 | ❌ 缺失 | ✅ `server.go:1601-1623` `correctDist=112.0` |
| stateSnapshot | ❌ 缺失 | ✅ `remote_packets.go:1242-1330` 完整读/写实现 |

### ✅ P0 修复（全部已存在，无需修改）

| 任务 | 发现 |
|------|------|
| 服务器发现 | `serveUDP()` + `handleUDPDatagram()` + `buildServerData()` 完整实现 |
| remote_packets.go | 所有 154 种包 Read/Write 完整实现 |
| 连接握手 | USID/CRC32/Mod兼容/封禁/白名单/版本检查全部完整 |

### ✅ P1 修复

| 任务 | 状态 |
|------|------|
| 速率限制 | Core4 `AllowPacket()` 已存在。Java 式的 per-interaction 细分限速为 P2 优化 |
| 反作弊 | `correctDist=112.0` 位置矫正 + `hasDuplicateIdentity` 严格身份检查均已实现 |
| 包优先级 | `SendAsyncPriority` + `sendLoop` 双通道 (outHigh/outNorm) 已实现 |

### ✅ P1 — 控制台命令补全（新增）

在 `runConsole()` 函数中新增以下命令：

| 命令 | 实现 |
|------|------|
| `say <message>` | `srv.BroadcastChat(msg)` — 广播聊天 |
| `info <UUID\|IP\|name>` | 搜索 `srv.ListConnectedConns()` 打印匹配玩家详情 |
| `search <name>` | 按名称子串搜索在线玩家 |
| `pause on/off` | `wld.SetPaused(bool)` — 暂停/恢复游戏 |
| `gameover` | `wld.SetGameOver(true)` + 广播通知 |
| `runwave` | `wld.TriggerWave()` — 触发下一波次 |
| `fillitems <team-id>` | `wld.FillTeamCoreItems(TeamID)` — 填充队伍核心物品 |
| `yes` | 存根（提示无上次命令记忆） |

文件变更：
- `cmd/mdt-server/main.go`: `runConsole` 新增 `wld *world.World` 参数 + 8 个命令 case
- `internal/world/world.go`: 新增 `paused`/`gameOver` 字段, 7 个公共方法, Snapshot/ApplySnapshot 反映真实状态
