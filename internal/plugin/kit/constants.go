package kit

import "github.com/IYanHua/mdt-server/internal/world"

// UnitTypes 提供常用的单位类型名称。
var UnitTypes = struct {
	Dagger string // 步兵
	Crawler string // 爬行者
	Flare string // 小飞机
	Mono string // 矿机
	Poly string // 建造机
	Mega string // 重装机
	Quad string // 四翼机
	Oct string // 八翼机
	Risso string // 轻型坦克
	Minke string // 重型坦克
	Oxynoe string // 布莱亚
	Cyerce string // 水陆坦克
	Aegires string // 水陆重装
	Navanax string // 运输船
	Corvus string // 轰炸机
	Retusa string // 追踪者
	Elude string // 闪避者
	Avert string // 躲避者
	Anthicus string // 运输车
	Precept string // 重型运输车
	Disrupt string // 扰乱者
}{
	Dagger: "dagger",
	Crawler: "crawler",
	Flare: "flare",
	Mono: "mono",
	Poly: "poly",
	Mega: "mega",
	Quad: "quad",
	Oct: "oct",
	Risso: "risso",
	Minke: "minke",
	Oxynoe: "oxynoe",
	Cyerce: "cyerce",
	Aegires: "aegires",
	Navanax: "navanax",
	Corvus: "corvus",
	Retusa: "retusa",
	Elude: "elude",
	Avert: "avert",
	Anthicus: "anthicus",
	Precept: "precept",
	Disrupt: "disrupt",
}

// TeamID helpers
const (
	TeamDerelict = world.TeamID(0)
	TeamSharded  = world.TeamID(1)
	TeamCrux     = world.TeamID(2)
	TeamMalis    = world.TeamID(3)
	TeamGreen    = world.TeamID(4)
	TeamPurple   = world.TeamID(5)
	TeamBlue     = world.TeamID(6)   // 默认玩家队伍
)
