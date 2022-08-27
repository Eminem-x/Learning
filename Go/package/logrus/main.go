// @Title logrus
// @Description go package enhanced the log
// @Author Yuanhao
// @Update 2022-08-27
package main

import (
	"bytes"
	"io"
	"log"
	"os"

	"github.com/sirupsen/logrus"
)

// Official: https://github.com/sirupsen/logrus
// Blog: https://darjun.github.io/2020/02/07/godailylib/logrus/
func main() {
	basicLog()
	ioLog()
	jsonLog()
	hookLog()
}

func basicLog() {
	logrus.SetLevel(logrus.TraceLevel)
	// Add filename and method information to output log
	logrus.SetReportCaller(true)

	// Add fields
	logrus.WithFields(logrus.Fields{
		"name": "ycx",
		"age":  21,
	}).Info("info msg")
	// If want all logs add some fields, use the return value of WithFileds
	// requestLogger := logrus.WithFields()

	// Log level increase from top to bottom
	logrus.Trace("trace msg")
	logrus.Debug("debug msg")
	logrus.Info("info msg")
	logrus.Warn("warn msg")
	logrus.Error("error msg")
	// logrus.Fatal("fatal msg")
	// logrus.Panic("panic msg")
}

func ioLog() {
	writer1 := &bytes.Buffer{}
	writer2 := os.Stdout
	writer3, err := os.OpenFile("log.txt", os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		log.Fatal("create file log.txt failed: %#v", err)
	}

	logrus.SetOutput(io.MultiWriter(writer1, writer2, writer3))
	logrus.Info("info msg")
}

func jsonLog() {
	logrus.SetLevel(logrus.TraceLevel)
	logrus.SetFormatter(&logrus.JSONFormatter{})

	logrus.Trace("trace msg")
	logrus.Debug("debug msg")
	logrus.Info("info msg")
	logrus.Warn("warn msg")
	logrus.Error("error msg")
	// logrus.Fatal("fatal msg")
	// logrus.Panic("panic msg")
}

type AppHook struct {
	AppName string
}

func (h *AppHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *AppHook) Fire(entry *logrus.Entry) error {
	entry.Data["app"] = h.AppName
	return nil
}

func hookLog() {
	// hook can make output fields added to output logs to diffrent dest accrording to level
	h := &AppHook{AppName: "awesome-web"}
	logrus.AddHook(h)

	// hook can send logs to redis/mongodb/ActiveMQ
	logrus.Info("info msg")
}
