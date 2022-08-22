// @Title go-ini
// @Description go package used to process .iml
// @Author Yuanhao
// @Update 2022-08-22
package main

import (
	"fmt"
	"log"
	"os"

	"gopkg.in/ini.v1"
)

type Config struct {
	AppName  string `ini:"app_name"`
	LogLevel string `ini:"log_level"`

	MySQL MySQLConfig `ini:"mysql"`
	Redis RedisConfig `ini:"redis"`
}

type MySQLConfig struct {
	IP       string `ini:"ip"`
	Port     int    `ini:"port"`
	User     string `ini:"user"`
	Password string `ini:"password"`
	Database string `ini:"database"`
}

type RedisConfig struct {
	IP   string `ini:"ip"`
	Port int    `ini:"port"`
}

// 官方文档 https://ini.unknwon.io/
// 官方仓库 https://github.com/go-ini/ini
// 博客 https://darjun.github.io/2020/01/15/godailylib/go-ini/
func main() {
	readIni()
	writeIni()
	mapToStruct()
}

func readIni() {
	cfg, err := ini.Load("my.ini")
	if err != nil {
		log.Fatal("Fail to read file: ", err)
	}

	// output sections' name
	fmt.Println("sections: ", cfg.Sections())
	fmt.Println("names: ", cfg.SectionStrings())

	// output specific section's content
	fmt.Println("App Name:", cfg.Section("").Key("app_name").String())
	fmt.Println("Log Level:", cfg.Section("").Key("log_level").String())

	fmt.Println("MySQL IP:", cfg.Section("mysql").Key("ip").String())
	mysqlPort, err := cfg.Section("mysql").Key("port").Int()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("MySQL Port:", mysqlPort)
	fmt.Println("MySQL User:", cfg.Section("mysql").Key("user").String())
	fmt.Println("MySQL Password:", cfg.Section("mysql").Key("password").String())
	fmt.Println("MySQL Database:", cfg.Section("mysql").Key("database").String())

	fmt.Println("Redis IP:", cfg.Section("redis").Key("ip").String())
	redisPort, err := cfg.Section("redis").Key("port").Int()                   // careful Int(), if x6381 will error
	fmt.Println("Redis Port:", cfg.Section("redis").Key("port").MustInt(6381)) // Must* will avoid cast error
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Redis Port:", redisPort)
}

func writeIni() {
	cfg := ini.Empty()

	defaultSection := cfg.Section("")
	defaultSection.NewKey("app_name", "awesome web")
	defaultSection.NewKey("log_level", "DEBUG")

	mysqlSection, err := cfg.NewSection("mysql")
	if err != nil {
		fmt.Println("new mysql section failed:", err)
		return
	}
	mysqlSection.NewKey("ip", "127.0.0.1")
	mysqlSection.NewKey("port", "3306")
	mysqlSection.NewKey("user", "root")
	mysqlSection.NewKey("password", "123456")
	mysqlSection.NewKey("database", "awesome")

	redisSection, err := cfg.NewSection("redis")
	if err != nil {
		fmt.Println("new redis section failed:", err)
		return
	}
	redisSection.NewKey("ip", "127.0.0.1")
	redisSection.NewKey("port", "6381")

	err = cfg.SaveTo("my-test.ini")
	if err != nil {
		fmt.Println("SaveTo failed: ", err)
	}

	err = cfg.SaveToIndent("my-pretty.ini", "\t")
	if err != nil {
		fmt.Println("SaveToIndent failed: ", err)
	}

	cfg.WriteTo(os.Stdout)
	fmt.Println()
	cfg.WriteToIndent(os.Stdout, "\t")
}

func mapToStruct() {
	cfg, err := ini.Load("my.ini")
	if err != nil {
		fmt.Println("load my.ini failed: ", err)
	}

	c := Config{}
	cfg.MapTo(&c)

	fmt.Println(c)
}
