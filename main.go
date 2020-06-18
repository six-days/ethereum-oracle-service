package main

import (
	"flag"
	"fmt"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
	"os"
	"six-days/oracle-service/conf"
	"six-days/oracle-service/core"
)

const VERSION = "1.0.0"

var (
	h bool
	v bool

	configPath string
	logPath    string
)

func init() {
	flag.BoolVar(&h, "h", false, "this help")
	flag.BoolVar(&v, "v", false, "show version")
	flag.StringVar(&configPath, "c", "./conf/app.conf", "config path")
	flag.StringVar(&logPath, "l", "./logs", "log path")

	flag.Usage = usage
}

func usage() {
	fmt.Fprintf(os.Stdout, `oracle_service version: %s
Usage: oracle_service [-h help] [-v version] [-c config path] [-l log path]

Options:
`, VERSION)
	flag.PrintDefaults()
}

func setLog() {
	logs.NewLogger(10000)
	if err := logs.SetLogger(logs.AdapterFile, `{"filename":"`+logPath+`/project.log","level":7,"daily":true,"maxdays":30}`); err != nil {
		panic("Failed to set log")
	}
	logs.SetLevel(7)
	logs.EnableFuncCallDepth(true)
	logs.Async()
}

func setConfig() {
	if err := beego.LoadAppConfig("ini", configPath); err != nil {
		panic("Failed to load config file, " + err.Error())
	}
	if err := conf.SetAppConfig(); err != nil {
		panic("Config error, " + err.Error())
	}
}

func main() {
	flag.Parse()
	if h {
		flag.Usage()
		return
	}
	if v {
		fmt.Println("oracle-service version:", VERSION)
		return
	}
	// init config
	setConfig()

	// init log
	setLog()

	watch, err := core.NewEventWatch(conf.GetAppConfig())
	if err != nil {
		logs.Error("[NewEventWatch] failed: ", err.Error())
		panic("NewEventWatch error, " + err.Error())
	}
	logs.Info("start watch...")
	watch.Start()
}
