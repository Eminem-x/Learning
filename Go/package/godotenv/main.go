// @Title godotenv
// @Description go package for process .env file
// @Author Yuanhao
// @Update 2022-08-28
package main

import (
	"bytes"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Official: https://github.com/joho/godotenv
// Blog: https://darjun.github.io/2020/02/12/godailylib/godotenv/
func main() {
	loadManually()
	loadAutomatically()
	loadSpecially()
	writeEnv()
}

func loadManually() {
	// make a .env file in the current dir
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("name: ", os.Getenv("name"))
	fmt.Println("age: ", os.Getenv("age"))
}

func loadAutomatically() {
	// _ "github.com/joho/godotenv/autoload"
	fmt.Println("name: ", os.Getenv("name"))
	fmt.Println("age: ", os.Getenv("age"))
}

func loadSpecially() {
	// Load a file with any name which not have to be suffixed with .env
	err := godotenv.Load("common", "dev.env")
	// Load接收多个文件名作为参数，如果不传入文件名，默认读取.env文件的内容
	// 如果多个文件中存在同一个键，那么先出现的优先，后出现的不生效
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("name: ", os.Getenv("name"))
	fmt.Println("version: ", os.Getenv("version"))
	fmt.Println("database: ", os.Getenv("database"))
}

func writeEnv() {
	buf := &bytes.Buffer{}
	buf.WriteString("name = awesome web")
	buf.WriteByte('\n')
	buf.WriteString("version = 0.0.1")

	env, err := godotenv.Parse(buf)
	if err != nil {
		log.Fatal(err)
	}

	err = godotenv.Write(env, "./save.env")
	if err != nil {
		log.Fatal(err)
	}
}
