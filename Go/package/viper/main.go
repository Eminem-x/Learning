// @Title viper
// @Description package for go configuration
// @Author Yunahao
// @Update 2022-08-23
package main

import (
	"fmt"
	"log"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func init() {
	pflag.Int("redis.port", 8381, "Redis port to connect")
	// bind with command
	viper.BindPFlags(pflag.CommandLine)
	// bind with env
	viper.AutomaticEnv()
}

// official: https://github.com/spf13/viper
// blog: https://darjun.github.io/2020/01/18/godailylib/viper/
func main() {
	// get configuration
	viper.SetConfigName("config")
	viper.SetConfigType("toml")
	viper.AddConfigPath(".")
	viper.SetDefault("redis.port", 6381)
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatal("read config failed: %v", err)
	}

	// output k-v
	fmt.Println(viper.Get("app_name"))
	fmt.Println(viper.Get("log_level"))

	fmt.Println("mysql: ", viper.Get("mysql"))
	fmt.Println("mysql ip: ", viper.Get("mysql.ip"))
	fmt.Println("mysql port: ", viper.Get("mysql.port"))
	fmt.Println("mysql user: ", viper.Get("mysql.user"))
	fmt.Println("mysql password: ", viper.Get("mysql.password"))
	fmt.Println("mysql database: ", viper.Get("mysql.database"))

	fmt.Println("redis ip: ", viper.Get("redis.ip"))
	viper.Set("redis.port", 8000) // set the highest priority key
	fmt.Println("redis port: ", viper.Get("redis.port"))

	// check is the key value exists
	if viper.IsSet("redis.port") {
		fmt.Println("redis.port is set")
	} else {
		fmt.Println("redis.port is not set")
	}

	// output AutomaticEnv k-v
	viper.BindEnv("go.os", "GOOS")
	fmt.Println("GOPATH: ", viper.Get("go.os"))

	// viper.Unmarshal()

	viper.WatchConfig() // watch the change of config

	// save configuration
	err := viper.SafeWriteConfig()
	if err != nil {
		log.Fatal("write config failed: ", err)
	}
}
