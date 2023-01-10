package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

func main() {
	f, err := os.Open("/Users/bytedance/Downloads/jax.txt")
	if err != nil {
		fmt.Println("err:", err)
		return
	}
	defer f.Close()

	content, err := readTxt(f)
	if err != nil {
		fmt.Println("err:", err)
		return
	}
	fmt.Println("content:", len(content))

	process(content)
}

func readTxt(r io.Reader) ([]string, error) {
	reader := bufio.NewReader(r)

	l := make([]string, 0, 64)

	// 按行读取
	for {
		line, _, err := reader.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return nil, err
			}
		}

		l = append(l, strings.Trim(string(line), " "))
	}

	return l, nil
}

var strs []string

func writeTxt() {
	// create file
	f, err := os.Create("file.txt")
	if err != nil {
		log.Fatal(err)
	}
	// remember to close the file
	defer f.Close()

	for _, str := range strs {
		_, err := f.WriteString(str + "\n")
		if err != nil {
			log.Fatal(err)
		}
	}
}

func process(content []string) {
	var connectCnt, disconnectCnt int
	for _, v := range content {
		if strings.Contains(v, "client_connected") {
			splitContent(v)
			connectCnt++
		}
		if strings.Contains(v, "client_disconnected") {
			splitContent(v)
			disconnectCnt++
		}
	}
	writeTxt()
	fmt.Printf("connect: %d, disconnect: %d", connectCnt, disconnectCnt)
}

func splitContent(s string) {
	t := strings.Split(s, " ")
	str := t[1] + " " + t[2] + " " + t[15]
	strs = append(strs, str)
}
