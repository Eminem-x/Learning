package main

import (
    "fmt"
    "log"
    "os"
)

var (
    errorLog = log.New(os.Stdout, "\033[31m[error]\033[0m ", log.LstdFlags|log.Lshortfile)
    infoLog  = log.New(os.Stdout, "\033[34m[info ]\033[0m ", log.LstdFlags|log.Lshortfile)
    // Literal copies a lock value from '*errorLog': type 'log.Logger' contains 'sync.Mutex' which is 'sync.Locker'
    loggers  = []log.Logger{*errorLog, *infoLog}
    logs     = []*log.Logger{errorLog, infoLog}
)

func main() {
    fmt.Println(&loggers[0])
    fmt.Println(loggers[0])

    // Call of 'fmt.Println' copies the lock value: type 'log.Logger' contains 'sync.Mutex' which is 'sync.Locker'
    fmt.Println(*logs[0])
    fmt.Println(logs[0])
}

