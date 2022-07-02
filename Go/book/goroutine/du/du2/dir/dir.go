package dir

import (
    "fmt"
    "io/ioutil"
    "os"
    "path/filepath"
)

// WalkDir 递归地遍历以 dir 为根目录的整个文件树
// 并在 fileSizes 上发送每个已找到的文件的大小
func WalkDir (dir string, fileSizes chan<- int64) {
    for _, entry := range dirents(dir) {
        if entry.IsDir() {
            subdir := filepath.Join(dir, entry.Name())
            WalkDir(subdir, fileSizes)
        } else {
            fileSizes <- entry.Size()
        }
    }
}


// dirents 返回 dir 目录中的数目
func dirents(dir string) []os.FileInfo {
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
