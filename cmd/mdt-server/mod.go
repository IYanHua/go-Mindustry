package main

import (
	"log"
	"plugin"
)

func ModLoader(path string) {
	log.Println("开始加载.", path)
	file, err := plugin.Open(path)
	if err == nil {
		log.Println("加载", path, "中...")
		function, err := file.Lookup("Init")
		if err == nil {
			i, ok := function.(func())
			if ok {
				i()
				log.Println("成功加载", path)
			} else {
				log.Println(path, "Init错误", function)
			}
		} else {
			log.Println(path, "没有找到Init")
		}
	}
}
