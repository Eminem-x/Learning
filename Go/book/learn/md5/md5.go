package main

import (
	"crypto/md5"
	"fmt"
	"os"
	"strconv"
	"time"
)

func main() {
	// stdin args
	var s string
	for i := 1; i < 2; i++ {
		s = os.Args[i]
	}
	// paginator
	businessData := `{"pageNum":`
	businessData += s + `,"pageSize":100}`
	fmt.Println(businessData)

	// md5 encrpt
	timestamp := time.Now().Unix() * 1e3
	secret := ""
	sign := fmt.Sprintf("%x", md5.Sum([]byte(secret+strconv.FormatInt(timestamp, 10)+businessData)))

	fmt.Println(timestamp)
	fmt.Println(sign)
}
