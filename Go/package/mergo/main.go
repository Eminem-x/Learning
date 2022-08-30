// @Title mergo
// @Description package for merge same struct and map
// @Author Yuanhao
// @Update 2022-08-30
package main

import (
	"fmt"
	"log"

	"github.com/imdario/mergo"
)

type redisConfig struct {
	Address string
	Port    int
	DB      int
}

var defaultConfig = redisConfig{
	Address: "127.0.0.1",
	Port:    6381,
	DB:      1,
}

// Official: https://github.com/imdario/mergo
// Blog: https://darjun.github.io/2020/03/11/godailylib/mergo/
func main() {
	basic()
	slice()
	checkType()
}

func basic() {
	var config redisConfig

	// override the default value
	// mergo.Merge(&config, defaultConfig, mergo.WithOverride)
	if err := mergo.Merge(&config, defaultConfig); err != nil {
		log.Fatal(err)
	}

	fmt.Println("redis address: ", config.Address)
	fmt.Println("redis port: ", config.Port)
	fmt.Println("redis db: ", config.DB)

	var m = make(map[string]interface{})
	if err := mergo.Map(&m, defaultConfig); err != nil {
		log.Fatal(err)
	}

	fmt.Println(m)
}

type mysqlConfig struct {
	Address string
	Port    int
	DBs     []int
}

var defaultMysqlConfig = mysqlConfig{
	Address: "127.0.0.1",
	Port:    6381,
	DBs:     []int{1},
}

func slice() {
	var config mysqlConfig
	config.DBs = []int{2, 3}

	// whether overried or not retain, it will append value
	if err := mergo.Merge(&config, defaultMysqlConfig, mergo.WithAppendSlice); err != nil {
		log.Fatal(err)
	}

	// WithOverrideEmptySlice WithOverwriteWithEmptyValue
	// These two func will not override nil
	fmt.Println("redis address: ", config.Address)
	fmt.Println("redis port: ", config.Port)
	fmt.Println("redis dbs: ", config.DBs)
}

func checkType() {
	m1 := make(map[string]interface{})
	m1["dbs"] = []uint32{2, 3}

	m2 := make(map[string]interface{})
	m2["dbs"] = []int{1}

	// map[string]interface{}, so interface not must be same
	if err := mergo.Map(&m1, &m2, mergo.WithOverride); err != nil {
		log.Fatal(err)
	}

	m3 := make(map[string]interface{})
	m3["dbs"] = []uint32{2, 3}

	// you can use mergo.WithTypeCheck to check type
	if err := mergo.Map(&m3, &m2, mergo.WithOverride, mergo.WithTypeCheck); err != nil {
		fmt.Println(err)
	}

	fmt.Println(m1)
}
