package main

import (
	"fmt"
	"sync"
)

type Callback struct {
	param string
}

var (
	CallbackChannelPoolMap = make(map[string]chan Callback)
	CallbackChannelPool    = sync.Pool{
		New: func() interface{} {
			return make(chan Callback)
		},
	}
)

// 多实例广播机制下,具体实例处理 topic 的方式
func main() {
	t := Callback{param: "uuid + code"}
	CallbackChannelPoolMap["uuid + code"] = CallbackChannelPool.Get().(chan Callback)
	// TODO wait callback
	callback := <-CallbackChannelPoolMap["uuid + code"]
	fmt.Println(callback)
}

func CallbackEvent(callback Callback) {
	c, ok := CallbackChannelPoolMap[callback.param]
	if !ok {
		return
	}
	c <- callback
}
