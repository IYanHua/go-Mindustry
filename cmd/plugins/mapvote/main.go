package main

import (
	pkg "github.com/IYanHua/mdt-server/internal/plugin"
	"github.com/IYanHua/mdt-server/internal/plugin/builtins/mapvote"
)

// Plugin 是动态加载时的入口符号
var Plugin pkg.Plugin = mapvote.NewMapVotePlugin()

func main() {}
