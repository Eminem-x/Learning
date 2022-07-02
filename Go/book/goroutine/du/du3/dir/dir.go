package dir

import (
    "fmt"
    "io/ioutil"
    "os"
    "path/filepath"
    "sync"
)

// WalkDir 递归地遍历以 dir 为根目录的整个文件树
// 并在 fileSizes 上发送每个已找到的文件的大小
func WalkDir (dir string, n *sync.WaitGroup ,fileSizes chan<- int64) {
    defer n.Done()
    for _, entry := range dirents(dir) {
        if entry.IsDir() {
            n.Add(1)
            subdir := filepath.Join(dir, entry.Name())
            go WalkDir(subdir, n, fileSizes)
        } else {
            fileSizes <- entry.Size()
        }
    }
}

var sema = make(chan struct{}, 20)

// dirents 返回 dir 目录中的数目
func dirents(dir string) []os.FileInfo {
    sema <- struct{}{}
    defer func() { <-sema }()

    entries, err := ioutil.ReadDir(dir)
    if err != nil {
        fmt.Fprintf(os.Stderr, "du1: %v\n", err)
        return nil
    }
    return entries
}

func PrintDiskUsage(nfiles, nbytes int64) {
    fmt.Printf("%d files    %1.f GB\n", nfiles, float64(nbytes) / 1e9)
}
