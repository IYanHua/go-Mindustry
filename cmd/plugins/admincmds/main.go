package main

import (
	pkg "github.com/IYanHua/mdt-server/internal/plugin"
	"github.com/IYanHua/mdt-server/internal/plugin/builtins/admincmds"
)

// Plugin 是动态加载时的入口符号
var Plugin pkg.Plugin = &admincmds.Plugin{}

func main() {}
