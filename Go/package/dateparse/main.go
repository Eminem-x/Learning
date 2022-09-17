package main

import (
	"fmt"
	"log"
	"time"

	"github.com/araddon/dateparse"
)

var Date = "2006-01-02 15:04:05"

// Official: https://github.com/araddon/dateparse
// Blog: https://darjun.github.io/2021/06/24/godailylib/dateparse/
func main() {
	basic()
	timezone()
}

func basic() {
	t1, err := dateparse.ParseAny("3/1/2014")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(t1.Format(Date))

	t2, err := dateparse.ParseAny("mm/dd/yyyy")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(t2.Format("2006-01-02 15:04:05"))
}

func timezone() {
	tz1, _ := time.LoadLocation("America/Chicago")
	t1, _ := dateparse.ParseIn("2021-06-24 15:50:30", tz1)
	fmt.Println(t1.Local().Format("2006-01-02 15:04:05"))

	t2, _ := dateparse.ParseIn("2021-06-24 15:50:30", time.Local)
	fmt.Println(t2.Local().Format("2006-01-02 15:04:05"))
}
