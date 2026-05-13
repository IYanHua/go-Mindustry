# Mindustry Java 服务端 vs Go (mdt-server) 对比诊断报告

> 日期: 2026-05-13
> Java 版本: Mindustry build 157 (官方)
> Go 版本: mdt-server 1.3.9-demo

---

## 一、架构对比总览

| 维度 | Java (官方) | Go (mdt-server) | 差异 |
|------|---|---|---|
| 核心架构 | 单进程，事件驱动 | 四核心多进程 (IPC) | Go 做了超集设计 |
| 网络层 | ArcNet (TCP+UDP, LZ4) | 自研 TCP+UDP, LZ4 | 兼容 |
| 协议层 | @Remote 注解生成 (~135 Call) | 手写 remote_packets.go (~120 stub) | **大量 stub 未实现** |
| 世界模拟 | World + EntityCollisions | World (13362 行) | Go 实现更细粒度 |
| TPS | 60 (固定) | 60-120 (可配置) | Go 扩展 |
| 插件系统 | Java Mod/Plugin + JS (Rhino) | Go native plugin (.so) | **不兼容** |

---

## 二、网络协议层详细对比

### 2.1 连接握手流程

**Java 流程:**
```
TCP连接 → Connect包 → 服务器检查: IP封禁/玩家限制 →
客户端发送 ConnectPacket {version, uuid, usid, mods[], name, locale, color, mobile} →
服务器验证: UUID/IP封禁, Kick冷却, 玩家限制, Mod兼容, 白名单, 版本 →
发送 WorldStream (StreamBegin + StreamChunks) →
客户端接收 → connectConfirm → 重生
```

**Go 流程 (当前):**
```
TCP连接 → ConnectPacket接收 →
准入策略检查: StrictIdentity, AllowCustomClients, PlayerLimit, Whitelist, BannedNames, BannedSubnets →
发送世界数据 →
postConnect → syncPostConnectWorldStateToConn
```

**🔴 缺陷:**
1. **缺少 USID (Session ID)**: Java 每个连接生成随机 USID 用于 IP 追踪。Go 未实现。
2. **缺少 Mod 兼容检查**: Java 对比客户端 Mod 列表。Go 中 `ExpectedMods` 字段存在但未实际执行检查。
3. **缺少 Connect 前序包**: Java 先发 `Connect` 包再发 `ConnectPacket`。Go 直接处理 ConnectPacket。
4. **缺少 CRC32 UUID 校验**: Java 在 UUID 后附加 CRC32。Go 未校验。

### 2.2 包优先级系统

**Java:**
- `priorityHigh (2)`: ConnectPacket, StreamBegin - 立即处理
- `priorityNormal (1)`: 大部分包 - 客户端未 loaded 时排队
- `priorityLow (0)`: 效果/音效 - 客户端未 loaded 时丢弃

**Go:**
- `Packet` 接口有 `Priority() int` 方法
- 但实际使用中未见优先级排队逻辑
- 所有包同等处理

**🟡 缺陷:**
- 没有实现按优先级分派包的逻辑。客户端在加载世界数据期间应排队普通优先级包。

### 2.3 可靠/不可靠通道

**Java:**
- TCP (可靠): ConnectPacket, WorldStream, Chat, Kick, 方块变更, 配置同步
- UDP (不可靠): EntitySnapshot, BlockSnapshot, StateSnapshot, Effects, Bullets

**Go:**
- 基本一致
- `server.go` 中有 `UdpRetryCount`, `UdpRetryDelay`, `UdpFallbackTCP` 配置

**🟢 状态:** 基本匹配，Go 甚至提供更多配置。

### 2.4 服务器发现

**Java:**
- UDP 组播 `227.2.7.7:20151`
- SRV 记录查询 `_mindustry._tcp.<address>`
- Ping 响应: 服务器名/地图/玩家数/波次/版本/模式/描述

**Go:**
- 未见组播发现实现
- 未见 SRV 记录查询
- 未见 ping 响应实现

**🔴 缺失:** 服务器发现协议完全未实现。客户端无法通过游戏内浏览器发现 Go 服务器。

---

## 三、世界流 (WorldStream) 对比

### 3.1 序列化内容

**Java `NetworkIO.writeWorld`:**
1. GameRules (JSON)
2. Map locale data (JSON)
3. Map tags
4. Wave number, waveTime, tick, random seeds
5. 连接玩家数据
6. Content header (已研究科技)
7. Content patches (方块修改)
8. 完整瓦片地图
9. Team blocks (所有建筑+库存+配置)
10. Logic markers
11. 自定义 Mod chunks

**Go `buildInitialWorldDataPayload` (main.go:7714):**
- 从 MSAV 构建世界流
- 支持 Core2 远程重写
- 支持世界缓存 (Core3)

**🟡 待验证:**
- Content header / patches 是否正确序列化
- Team blocks 完整度
- Logic markers 支持
- Mod custom chunks

### 3.2 流分块

**Java:** `StreamBegin` + 多个 `StreamChunk` (每块 maxTcpSize 字节)
**Go:** `stream.go` 中有 StreamBegin/StreamChunk 实现

**🟢 基本匹配。**

---

## 四、实体系统对比

### 4.1 实体快照

**Java:**
- 每 200ms 发送一次 (可配置)
- 批量: maxSnapshotSize = 800 字节
- 隐藏实体单独发送 (hiddenSnapshot)
- 每实体: ID + classID + writeSync 数据
- 客户端有 entitySnapshotTimeout = 20 秒

**Go:**
- `SyncEntityMs` 配置 (默认 60ms?)
- 批量: 类似逻辑
- `UnitSyncHiddenForViewer` 隐藏实体过滤
- 实体写同步在 `entitysync.go`

**🟡 差异:**
- Java 默认 200ms, Go 默认更短 (60ms?)
- 需要确认 Go 的 maxSnapshotSize 是否为 800

### 4.2 状态快照

**Java:**
- stateSnapshot 包含: waveTime, wave, enemies, paused, gameOver, timeData, TPS, rand0/rand1
- 每 team 的 core items

**Go:**
- `world.Snapshot()` 返回类似结构
- `persist.State` 用于持久化
- 需要验证 core items 同步完整性

### 4.3 实体类型系统

**Java `EntityMapping`:**
- 通过 `classId()` 映射实体类型
- 客户端创建实体时按 classId 查找工厂

**Go:**
- `UnitEntitySync` 实现 `UnitSyncEntity` 接口
- Content registry 用于 ID 解析
- 未见完整的 EntityMapping 机制

**🟡 待验证:** Content registry 中实体类型映射是否完整。

---

## 五、安全系统对比

### 5.1 封禁系统

| 封禁类型 | Java | Go |
|---------|------|-----|
| UUID 封禁 | ✅ 持久化 | ✅ `banUUID` |
| IP 封禁 | ✅ 持久化 | ✅ `banIP` |
| 子网封禁 | ✅ `subnet-ban` | ✅ `BannedSubnets` |
| 名称封禁 (regex) | ✅ `name-ban` | ✅ `BannedNames` |
| DOS 黑名单 | ✅ 内存 (packetSpamLimit) | ❌ **缺失** |
| Kick 冷却 | ✅ 持久化 | ✅ `RecentKickDuration` |

### 5.2 速率限制

**Java:**
- `interactRateWindow` / `interactRateLimit` / `interactRateKick`: 交互限速
- `messageRateLimit` / `messageSpamKick`: 聊天限速
- `packetSpamLimit`: 包速率限制 (300/3秒 → DOS)
- `chatSpamLimit`: 聊天包限制 (20/2秒 → 黑名单)

**Go:**
- Core4 (Policy Core) 设计用于速率限制
- `core4.AllowPacket()` 存在
- 但具体限制参数不足

**🔴 缺失:**
- 交互速率限制 (interactRateLimit)
- 消息速率限制 (messageRateLimit)
- DOS 黑名单自动封禁

### 5.3 反作弊

**Java:**
- 位置验证: `correctDist = tilesize * 14f`
- 客户端快照检查: `rejectedRequests` (被拒绝的建造计划)
- Strict 模式: 强制唯一 UUID, 正位置检查

**Go:**
- `StrictIdentity` 配置
- 建造计划处理 (buildService)
- 未见位置验证逻辑

**🔴 缺失:**
- 客户端位置矫正验证 (`correctDist`)
- 被拒绝建造计划的追踪

---

## 六、控制台命令对比

### Java 支持但 Go 缺失的命令

| Java 命令 | 状态 | 说明 |
|----------|----|------|
| `pause on/off` | ❌ | 暂停游戏 |
| `rules add/remove [name] [value]` | ❌ | 运行时修改规则 |
| `fillitems [team]` | ❌ | 填充核心物品 |
| `shuffle [mode]` | ❌ | 随机地图模式 |
| `nextmap <name>` | ❌ | 指定下一张地图 |
| `loadautosave` | ❌ | 加载自动存档 |
| `load <slot>` | ❌ | 加载存档槽 |
| `save <slot>` | ❌ | 保存到存档槽 |
| `saves` | ❌ | 列出存档 |
| `gameover` | ❌ | 强制结束 |
| `info <IP/UUID/name>` | ❌ | 查询玩家信息 |
| `search <name>` | ❌ | 搜索玩家名 |
| `runwave` | ❌ | 触发下一波 |
| `js <script>` | 部分 | Go 有 `js` 命令但实现不同 |
| `mod <name>` | 部分 | Go 有 `mod` 但列出插件 |
| `say <message>` | 变体 | Go 用 `#` 前缀发送聊天 |
| `yes` | ❌ | 执行上次建议的命令 |
| `dos-ban` | ❌ | DOS 封禁管理 |

### Go 独有命令

| Go 命令 | 说明 |
|--------|------|
| `hotload` | 在线热加载地图 (Java 无) |
| `selfcheck` | 自检诊断 |
| `apikey` | API 密钥管理 |
| `progress` | 服务器进度概览 |
| `compat` | 兼容性状态 |
| `api` | HTTP API 配置 |
| `vanilla` | Vanilla profiles 管理 (生成/重载) |
| `ubehavior/umove/uteleport/ulife/ufollow/upatrol` | 单位控制命令 |
| `despawn` | 移除单位命令 |

---

## 七、地图和游戏模式对比

### 7.1 地图加载

| 功能 | Java | Go |
|------|------|-----|
| 内置地图 | 19 个 (msav) | assets/worlds/*.msav |
| 自定义地图 | config/maps/*.msav | 可配置目录 |
| Workshop 地图 | Steam Workshop | ❌ **不适用** |
| Mod 地图 | mods/*/maps/ | ❌ **缺失** |
| 随机地图生成 | ❌ | ✅ **Go 独有** |
| Map tags 解析 | ✅ | ✅ |
| Map shuffle | ✅ (4 种模式) | ❌ **缺失** |

### 7.2 游戏模式

**Java:**
- Survival (生存)
- Sandbox (沙盒)
- Attack (攻击)
- PvP
- Editor (编辑器)

**Go:** 实现了 Rules 系统可配置这些模式，但未见编辑器模式。

### 7.3 地图投票

**Java:** 无地图投票系统 (由管理员控制)
**Go:** 有完整的 `map_vote.go` 实现

**🟢 Go 独有特性。**

---

## 八、插件/Mod 系统对比

| 维度 | Java | Go |
|------|------|-----|
| 插件类型 | Java JAR, JS (Rhino), JSON content | Go native plugin (.so/.dll) |
| Mod 元数据 | mod.json/hjson/plugin.json | 无标准元数据 |
| 依赖管理 | 声明式依赖 + 软依赖 | 无 |
| 服务器命令注册 | `registerServerCommands()` | 无标准接口 |
| 客户端命令注册 | `registerClientCommands()` | 无 |
| 聊天过滤器 | `addChatFilter()` | 无 |
| 操作过滤器 | `addActionFilter()` | 无 |
| 包处理器 | `addPacketHandler()` | 无 |
| 配置 API | `getConfigFolder()` | 无 |
| Content 补丁 | `config/patches/*.json` | ❌ **缺失** |

**🔴 重大差异:** Go 的插件系统仅支持 Go native plugin（仅限 Linux/macOS），且缺乏 Java 版的标准插件 API（命令注册、过滤器、包处理器等）。

---

## 九、持久化系统对比

### 9.1 存档格式

**Java:**
- SaveIO: 完整游戏状态序列化
- 自动存档 (autosave): 槽位轮换
- 存档槽: save <slot>, load <slot>
- JSON 设置存储
- 玩家数据: JSON 持久化

**Go:**
- MSAV 地图保存 (msav.go)
- 热快照 (hot snapshot): 内存中，周期性刷新
- 冷快照 (cold snapshot): JSON 文件，7 天保留
- 状态持久化 (persist/state.go)
- 事件录制 (storage/)

**🟡 差异:**
- Go 的快照比 Java 的 SaveIO 粒度更粗 (只保存顶层状态，不保存完整世界)
- Go 缺少多存档槽管理
- Go 缺少自动存档槽位轮换

### 9.2 玩家数据

**Java `PlayerInfo`:**
- UUID, names[], ips[], lastName, connectTime
- admin 状态, timesJoined, timesKicked
- 持久化到 JSON

**Go:**
- `persist.PlayerIdentityStore` - 身份映射
- `persist.PublicConnUUIDStore` - 连接 UUID
- ops.json - OP 列表
- 缺少完整的 PlayerInfo 追踪

**🟡 缺失:** 无 `timesJoined`, `timesKicked` 统计。无全玩家档案查询 (`/info`)。

---

## 十、协议兼容性关键缺陷汇总

### 🔴 严重 (会导致客户端无法连接或崩溃)

1. **服务器发现未实现**: 无 UDP 组播响应，客户端浏览器找不到 Go 服务器
2. **remote_packets.go 120+ stub 未实现**: 标注为 "Read/Write are TODO stubs"，客户端可能收不到关键数据
3. **缺少 USID 生成**: 连接标识不完整
4. **缺少 Connect 前序包处理**: 可能导致客户端连接流程异常

### 🟡 中等 (功能缺失或行为差异)

5. **缺少 CRC32 UUID 校验**: UUID 完整性保护缺失
6. **缺少 Mod 兼容检查**: 不同 Mod 的客户端可能连接失败
7. **优先级排队未实现**: 加载期间包处理顺序可能异常
8. **entitySnapshotTimeout 未实现**: 客户端断线检测依赖于此
9. **位置验证缺失**: 反作弊能力不足
10. **速率限制不完整**: 无 DOS 防护
11. **存档槽管理缺失**: 无 save/load 槽位
12. **编辑器模式缺失**: 无法用于地图编辑
13. **Content patches 不支持**: 无法运行时修改方块属性

### 🟢 Go 独有优势

- 120 TPS (Java 固定 60)
- 四核心并行架构
- HTTP API 管理接口
- 地图投票系统
- 热加载地图 (hotload)
- 在线视频录制
- 随机地图生成
- 更多单位控制命令

---

## 十一、建议修复优先级

### P0 - 阻塞性问题
1. 实现服务器发现 (UDP 组播 + ping 响应)
2. 完成 remote_packets.go 中关键包的 Read/Write (至少 connectConfirm, entitySnapshot, blockSnapshot, stateSnapshot)
3. 实现 USID 生成和 Connect 前序包流程

### P1 - 功能完整
4. 实现完整连接握手 (版本检查, Mod 兼容, CRC32 校验)
5. 实现优先级包排队系统
6. 实现 DOS/速率限制
7. 实现位置验证反作弊

### P2 - 体验提升
8. 存档槽管理 (save/load 多槽位)
9. 实现 Java 缺失的命令 (pause, runwave, gameover, fillitems 等)
10. 完善玩家信息追踪
11. Content patches 支持

### P3 - 扩展
12. 完整插件 API (命令注册、过滤器、包处理器)
13. 编辑器模式
14. 地图 shuffle 模式
