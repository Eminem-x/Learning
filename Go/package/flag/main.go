// @Title flag
// @Description study go package: flag
// @Author Yuanhao
// @Update 2022-08-19
package main

import (
	"flag"
	"fmt"
	"time"
)

var (
	intflag    int
	boolflag   bool
	stringflag string
	period     time.Duration
)

func init() {
	flag.IntVar(&intflag, "intflag", 0, "int flag value")
	flag.BoolVar(&boolflag, "boolflag", false, "bool flag value")
	flag.StringVar(&stringflag, "stringflag", "default", "string flag value")
	flag.StringVar(&stringflag, "sf", "default", "string flag value(shorthand)")
	flag.DurationVar(&period, "period", 1*time.Second, "sleep period")
}

func main() {
	flag.Parse()
	parseDuration()
	printFlag()
	printArgs()
}

func printFlag() {
	fmt.Println("int flag:", intflag)
	fmt.Println("bool flag:", boolflag)
	fmt.Println("string flag:", stringflag)
}

func printArgs() {
	fmt.Println(flag.Args())
	fmt.Println("Non-Flag Argument Count:", flag.NArg())
	for i := 0; i < flag.NArg(); i++ {
		fmt.Printf("Argument %d: %s\n", i, flag.Arg(i))
	}

	fmt.Println("Flag Count:", flag.NFlag())
}

func parseDuration() {
	fmt.Printf("Sleeping for %v...\n", period)
	time.Sleep(period)
}
