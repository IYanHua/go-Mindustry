package main

import (
	"fmt"
	"github.com/IYanHua/mdt-server/internal/config"
	netserver "github.com/IYanHua/mdt-server/internal/net"
)

func printCompatStatus(cfg config.Config, srv *netserver.Server) {
	fmt.Println("原版一致性状态（基于当前 Go 多核服务端）:")
	unitSyncBase := srv != nil && (srv.ExtraEntitySnapshotEntitiesFn != nil || srv.ExtraEntitySnapshotFn != nil)
	items := []struct {
		Name   string
		Status string
	}{
		{"基础握手/连接流程", "已实现"},
		{"地图加载与流发送", "已实现"},
		{"聊天/踢封/OP/基础管理", "已实现"},
		{"API 管理与脚本自动化", "已实现"},
		{"原版参数加载管线(单位/炮塔 profiles.json)", "已实现"},
		{"原版源码提取器(vanilla gen 自动生成profiles)", "已实现"},
		{"单位同步(UnitEntity可见同步+销毁生命周期)", ternary(unitSyncBase, "已实现", "未完成")},
		{"单位战斗最小闭环(自动攻击/伤害/死亡)", "已实现"},
		{"单位多武器挂点(分挂点冷却并行开火)", "已实现"},
		{"目标过滤(空军/地面命中筛选)", "已实现"},
		{"目标优先级与命中体积(碰撞半径)", "已实现"},
		{"目标锁定与转火延迟(anti-jitter)", "已实现"},
		{"建筑最小闭环(受击/销毁广播)", "已实现"},
		{"关键战斗包对齐(buildHealthUpdate/销毁/子弹)", "已实现"},
		{"建筑炮塔攻击(按原版炮塔名匹配开火参数)", "已实现"},
		{"建筑炮塔资源约束(弹药/电力/连发)", "已实现"},
		{"单位同步(全部原版实体行为)", "未完成"},
		{"建筑/逻辑/战斗完整模拟(全机制)", "未完成"},
		{"原版全部网络包语义一致", "未完成"},
	}
	done := 0
	for _, it := range items {
		if it.Status == "已实现" {
			done++
		}
	}
	pct := int(float64(done) * 100 / float64(len(items)))
	fmt.Printf("一致性进度: %d%% (%d/%d)\n", pct, done, len(items))
	fmt.Printf("当前脚本配置文件: %s\n", cfg.Script.File)
	for _, it := range items {
		fmt.Printf("  - %s: %s\n", it.Name, it.Status)
	}
	fmt.Println("说明: 要做到“与原版一模一样”，核心缺口在完整单位/建筑/战斗逻辑模拟与全部包语义对齐。")
}

func ternary(ok bool, yes, no string) string {
	if ok {
		return yes
	}
	return no
}

