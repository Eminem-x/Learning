package main

import (
    "du1/dir"
    "flag"
)

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

    // 输出结果
    var nfiles, nbytes int64
    for size := range fileSizes {
        nfiles++
        nbytes += size
    }
    dir.PrintDiskUsage(nfiles, nbytes)
}
