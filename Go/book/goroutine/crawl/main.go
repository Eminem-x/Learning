package main

import (
	"crawl/links"
	"fmt"
	"log"
	"os"
)

func main() {
	workList := make(chan []string)

	// 从命令行参数开始
	go func() { workList <- os.Args[1:] }()

	// 并发爬去 web
	seen := make(map[string]bool)
	for list := range workList {
		for _, link := range list {
			if !seen[link] {
				seen[link] = true
				go func(link string) {
					workList <- crawl(link)
				}(link)
			}
		}
	}
}

func crawl(url string) []string {
	fmt.Println(url)
	list, err := links.Extract(url)
	if err != nil {
		log.Print(err)
	}
	return list
}
