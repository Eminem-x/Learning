package main

import (
    "du1/dir"
    "flag"
    "time"
)

var verbose = flag.Bool("v", false, "show verbose progress messages")

func main() {
    // 确定初始目录
    flag.Parse()
    roots := flag.Args()
    if len(roots) == 0 {
        roots = []string{"."}
    }

    // 遍历文件树
    fileSizes := make(chan int64)
    go func() {
        for _, root := range roots {
            dir.WalkDir(root, fileSizes)
        }
        close(fileSizes)
    }()

    var tick <-chan time.Time
    if *verbose {
        tick = time.Tick(500 * time.Millisecond)
    }
    var nfiles, nbytes int64
loop:
    for {
        select {
        case size, ok := <-fileSizes:
            if !ok {
                break loop
            }
            nfiles++
            nbytes += size
        case <-tick:
            dir.PrintDiskUsage(nfiles, nbytes)
        }
    }
    dir.PrintDiskUsage(nfiles, nbytes)
}
