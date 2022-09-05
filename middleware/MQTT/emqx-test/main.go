package main

import (
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

const (
	KeepAlive   = 60 * time.Second
	PingTimeout = 1 * time.Second
)

var (
	client mqtt.Client
)

func main() {
	r := gin.Default()
	r.GET("/emqx", func(c *gin.Context) {
		c.String(http.StatusOK, "Hello Emqx")
	})

	opts := mqtt.NewClientOptions().AddBroker("tcp://ip:port").SetClientID("ycx-yh")
	opts.SetKeepAlive(KeepAlive)
	opts.SetPingTimeout(PingTimeout)
	// 设置连接后需要做的动作 可以有效避免重连后 无法订阅 topic 的情况
	opts.SetOnConnectHandler(func(c mqtt.Client) {
		go Subscribe()
	})

	client = mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		fmt.Println("connect fail")
	}

	r.Run(":8080")
}

func Subscribe() {

	// sleep增加稳定性
	time.Sleep(5 * time.Second)

	if token := client.Subscribe("backend_topic", 2, nil); token.Wait() && token.Error() != nil {
		fmt.Println("subscribe fail")
	}

}
