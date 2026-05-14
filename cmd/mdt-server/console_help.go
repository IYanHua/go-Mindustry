package main

import (
	"fmt"
	"net"
	"strings"
	"github.com/IYanHua/mdt-server/internal/buildinfo"
	"github.com/IYanHua/mdt-server/internal/config"
)

func printConsoleIntro(serverName, worldPath, listenAddr, apiBind string, apiEnabled bool, personalization config.PersonalizationConfig) {
	if !personalization.ConsoleIntroEnabled {
		return
	}
	fmt.Println("========================================")
	if strings.TrimSpace(serverName) == "" {
		serverName = "mdt-server"
	}
	if personalization.ConsoleIntroServerNameEnabled {
		fmt.Printf("服务器名称: %s\n", serverName)
	}
	if personalization.ConsoleIntroCurrentMapEnabled {
		fmt.Printf("当前地图:   %s\n", worldPath)
	}
	if personalization.ConsoleIntroListenAddrEnabled {
		fmt.Printf("监听地址:   %s\n", listenAddr)
	}
	if personalization.ConsoleIntroLocalIPEnabled {
		if ip := firstLocalIPv4(); ip != "" {
			fmt.Printf("本机IP:     %s\n", ip)
		}
	}
	if personalization.ConsoleIntroAPIEnabled {
		if apiEnabled {
			fmt.Printf("API地址:    %s\n", apiBind)
		} else {
			fmt.Println("API地址:    已关闭")
		}
	}
	if personalization.ConsoleIntroHelpHintEnabled {
		fmt.Println("输入 `help all` 查看完整帮助")
	}
	fmt.Println("========================================")
}

func printHelp(cfg config.Config) {
	fmt.Println("控制台命令（输入 `help <分类>` 或 `help all`）:")
	fmt.Println("分类: basic vanilla runtime admin plugin script data scheduler persist net sync api chat close")
	fmt.Println("提示: 输入 `help basic` 查看常用命令")
}

func printBrandFooter() {
	displayName := strings.TrimSpace(buildinfo.DisplayName)
	gameVersion := strings.TrimSpace(buildinfo.GameVersion)
	externalVersion := strings.TrimSpace(buildinfo.Version)
	qqGroup := strings.TrimSpace(buildinfo.QQGroup)
	footer := strings.TrimSpace(buildinfo.FooterText)

	if displayName == "" && gameVersion == "" && externalVersion == "" && qqGroup == "" && footer == "" {
		return
	}

	const (
		deepBlue  = "\x1b[34m"
		lightBlue = "\x1b[94m"
		purple    = "\x1b[35m"
		pink      = "\x1b[95m"
		reset     = "\x1b[0m"
	)

	fmt.Printf("%s========================================%s\n", purple, reset)
	fmt.Println()
	if displayName != "" {
		fmt.Printf("%s名称：%s%s%s\n", deepBlue, lightBlue, displayName, reset)
	}
	if gameVersion != "" {
		fmt.Printf("%s游戏版本: %s%s%s\n", deepBlue, lightBlue, gameVersion, reset)
	}
	if externalVersion != "" {
		fmt.Printf("%s外部版本: %s%s%s\n", deepBlue, lightBlue, externalVersion, reset)
	}
	if qqGroup != "" {
		fmt.Printf("%s加入qq群：%s%s%s\n", deepBlue, lightBlue, qqGroup, reset)
	}
	if footer != "" {
		fmt.Printf("%s%s%s\n", pink, footer, reset)
	}
	fmt.Println()
	fmt.Printf("%s========================================%s\n", purple, reset)
}

func printHelpCategory(cfg config.Config, category string) {
	if category == "" {
		category = "basic"
	}
	if category == "all" {
		for _, c := range []string{"basic", "vanilla", "runtime", "admin", "plugin", "script", "data", "scheduler", "persist", "net", "sync", "api", "chat", "close"} {
			printHelpCategory(cfg, c)
		}
		return
	}
	fmt.Printf("【%s】\n", category)
	switch category {
	case "basic":
		printHelpCmd("help [category|all]", "显示帮助")
		printHelpCmd("maps", "列出可用地图")
		printHelpCmd("world", "查看当前地图文件")
		printHelpCmd("server status", "查看服务器名称/简介/虚拟人数")
		printHelpCmd("server name <名称>", "设置服务器名称（写入配置）")
		printHelpCmd("server desc <简介>", "设置服务器简介（写入配置）")
		printHelpCmd("server players <虚拟人数>", "设置大厅虚拟人数（写入配置）")
		printHelpCmd("host random", "原版式重载到随机地图（不踢人）")
		printHelpCmd("host <map-name>", "原版式重载到 core/assets/maps/default/<map-name>.msav")
		printHelpCmd("host <file-path>", "原版式重载到指定 .msav")
		printHelpCmd("hotload random", "在线热加载到随机地图（不踢人）")
		printHelpCmd("hotload <map-name|file-path>", "在线热加载到指定地图（不踢人）")
		printHelpCmd("ip", "显示本机 IP 和监听地址")
		printHelpCmd("selfcheck", "基本自检（地址/端口/地图/配置）")
	case "vanilla":
		printHelpCmd("vanilla status", "查看原版参数文件路径")
		printHelpCmd("vanilla reload [path]", "重载原版参数文件（可选修改路径并写入配置）")
		printHelpCmd("vanilla gen [repoRoot] [outPath]", "从原版源码自动生成并加载 profiles.json（可选输出路径）")
		printHelpCmd("vanilla ids gen [repoRoot] [outPath]", "从原版源码/logicids.dat 生成并加载 content IDs")
		printHelpCmd("vanilla ids reload [path]", "重载 content IDs 到协议内容注册表")
		fmt.Printf("  当前文件: %s\n", canonicalRuntimePath(cfg.Runtime.VanillaProfiles))
	case "runtime":
		printHelpCmd("status", "输出服务器资源状态")
		printHelpCmd("status watch on|off", "周期输出服务器资源状态")
		printHelpCmd("players", "列出当前连接")
		printHelpCmd("uuid [conn-id]", "列出在线连接uuid或查询指定连接uuid")
	case "admin":
		printHelpCmd("#<msg>", "向所有玩家发送聊天")
		printHelpCmd("kick <conn-id> [reason]", "踢出指定连接")
		printHelpCmd("ban uuid|ip|id ...", "封禁")
		printHelpCmd("unban uuid|ip ...", "解封")
		printHelpCmd("bans", "查看封禁列表")
		printHelpCmd("op <uuid>", "设置OP")
		printHelpCmd("opid <conn-id>", "按连接ID设置OP（自动取uuid）")
		printHelpCmd("deop <uuid>", "移除OP")
		printHelpCmd("ops", "列出OP")
		printHelpCmd("despawn <entity-id>", "移除单位实体并广播销毁")
		printHelpCmd("umove <entity-id> <vx> <vy> [rot-vel]", "设置单位速度/角速度")
		printHelpCmd("uteleport <entity-id> <x> <y> [rotation]", "传送单位并设置朝向")
		printHelpCmd("ulife <entity-id> <seconds>", "设置单位寿命(<=0为无限)")
		printHelpCmd("ufollow <entity-id> <target-id> [speed]", "设置单位跟随目标")
		printHelpCmd("upatrol <entity-id> <x1> <y1> <x2> <y2> [speed]", "设置单位巡逻")
		printHelpCmd("ubehavior clear <entity-id>", "清除单位行为并停止")
	case "plugin":
		printHelpCmd("mods", "列出 Java 模组（jar）")
		printHelpCmd("mod", "列出脚本插件（js/go/node）")
		printHelpCmd("js <script.js> [args]", fmt.Sprintf("在 %s 目录运行 Node.js 脚本", cfg.Mods.JSDir))
		printHelpCmd("node <script.js> [args]", fmt.Sprintf("在 %s 目录运行 Node.js 脚本", cfg.Mods.NodeDir))
		printHelpCmd("go <target|.> [args]", fmt.Sprintf("在 %s 目录运行 go run", cfg.Mods.GoDir))
	case "script":
		printHelpCmd("script file", "显示脚本配置文件路径（JSON）")
		printHelpCmd("script gc now", "立即执行 GC+释放内存")
		printHelpCmd("script gc daily <HH:MM|off>", "设置每日定时 GC（写入配置）")
		printHelpCmd("script startup list", "查看开机任务（来自配置文件）")
		fmt.Println("  建议: 通过 JSON 配置脚本任务，开机自动读取执行")
	case "data":
		printHelpCmd("data status", "查看事件存储状态")
		printHelpCmd("data db on|off", "切换 database_enabled")
		printHelpCmd("data mode <mode>", "设置 file|postgres|mysql|redis")
		printHelpCmd("data dir <path>", "设置文件存储目录")
	case "scheduler":
		printHelpCmd("scheduler status|on|off", "调度器配置")
		fmt.Printf("  scheduler 状态:           %s\n", colorState(cfg.Runtime.SchedulerEnabled))
	case "persist":
		fmt.Printf("  snapshot: enabled=%v cold_dir=%s file=%s cold_interval=%ds hot_interval=%ds retention=%dd\n",
			cfg.Persist.Enabled, canonicalRuntimePath(cfg.Persist.Directory), cfg.Persist.File, cfg.Persist.IntervalSec, cfg.Persist.HotIntervalSec, cfg.Persist.RetentionDays)
		fmt.Printf("  script file: %s\n", canonicalRuntimePath(cfg.Script.File))
		fmt.Printf("  ops file: %s\n", canonicalRuntimePath(cfg.Admin.OpsFile))
	case "net":
		fmt.Printf("  UDP 重试次数: %d\n", cfg.Net.UdpRetryCount)
		fmt.Printf("  UDP 重试间隔: %dms\n", cfg.Net.UdpRetryDelayMs)
		fmt.Printf("  UDP 失败回退 TCP: %v\n", cfg.Net.UdpFallbackTCP)
	case "sync":
		printHelpCmd("sync status", "查看同步频率")
		printHelpCmd("sync entity <ms>", "设置实体快照频率（毫秒）")
		printHelpCmd("sync state <ms>", "设置状态快照频率（毫秒）")
		printHelpCmd("sync set <entityMs> <stateMs>", "一次性设置实体/状态频率（毫秒）")
		printHelpCmd("sync default", "恢复默认：entity=100ms state=250ms")
		fmt.Printf("  当前配置: entity=%dms state=%dms\n", cfg.Net.SyncEntityMs, cfg.Net.SyncStateMs)
	case "api":
		printHelpCmd("api status", "查看 API 状态")
		printHelpCmd("api keys", "查看已有 APIKEY")
		printHelpCmd("api keygen", "生成并保存 APIKEY")
		printHelpCmd("api keydel <key>", "删除 APIKEY")
		printHelpCmd("apikey", "兼容旧命令，显示状态")
	case "chat":
		printHelpCmd("/help", "查看玩家命令帮助")
		printHelpCmd("/status", "查看服务器状态")
		printHelpCmd("/summon <typeId|unitName> [x y] [count] [team]", "OP召唤单位（支持 alpha/mono/nova；省略坐标=玩家脚底）")
		printHelpCmd("/despawn <entityId>", "OP移除指定单位")
		printHelpCmd("/kill", "杀死自己当前单位（附身/未附身均可）")
		printHelpCmd("/stop", "OP保存并关闭服务器")
	case "close":
		printHelpCmd("stop", "保存并关闭服务器")
		printHelpCmd("exit", "立即退出服务器（不保存）")
	default:
		fmt.Printf("未知分类: %s\n", category)
	}
}

func printHelpCmd(cmd, desc string) {
	fmt.Printf("  \x1b[34m%-28s\x1b[0m %s\n", cmd, desc)
}

func firstLocalIPv4() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet == nil || ipNet.IP == nil {
				continue
			}
			ip := ipNet.IP.To4()
			if ip == nil || ip.IsLoopback() {
				continue
			}
			return ip.String()
		}
	}
	return ""
}

