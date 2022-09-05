package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

// mqtt 重新拉起订阅脚本
func main() {

	cookie := ""

	client := &http.Client{}

	req, _ := http.NewRequest("GET", "", nil)
	req.Header.Add("Authorization", "Basic YWRtaW46cHVibGlj")
	req.Header.Add("Cookie", cookie)
	resp, _ := client.Do(req)
	body, _ := ioutil.ReadAll(resp.Body)

	var t AutoGenerated
	err := json.Unmarshal(body, &t)
	if err != nil {
		fmt.Printf("json.Unmarshal: %#v\n", err)
	}

	var cnt int
	for i := 0; i < len(t.Data); i++ {
		data := t.Data[i]
		if strings.Contains(data.Clientid, "vm_") && data.SubscriptionsCnt == 0 {

			payload := make(map[string]interface{})
			payload["clientid"] = data.Clientid
			payload["qos"] = 2
			payload["topic"] = "backend_topic"
			bytesData, _ := json.Marshal(payload)

			req, _ := http.NewRequest("POST", "", bytes.NewReader(bytesData))
			req.Header.Add("Authorization", "Basic YWRtaW46cHVibGlj")
			req.Header.Add("Cookie", cookie)
			resp, _ := client.Do(req)
			_, err := ioutil.ReadAll(resp.Body)

			if err == nil {
				cnt++
			}
		}
	}

	fmt.Printf("subsribe count: %#v\n", cnt)
}

type AutoGenerated struct {
	Meta struct {
		Page    int  `json:"page"`
		Limit   int  `json:"limit"`
		Hasnext bool `json:"hasnext"`
		Count   int  `json:"count"`
	} `json:"meta"`
	Data []struct {
		MqueueDropped    int    `json:"mqueue_dropped"`
		CleanStart       bool   `json:"clean_start"`
		Zone             string `json:"zone"`
		SendCnt          int    `json:"send_cnt"`
		Keepalive        int    `json:"keepalive"`
		IsBridge         bool   `json:"is_bridge"`
		SendOct          int    `json:"send_oct"`
		ProtoName        string `json:"proto_name"`
		RecvCnt          int    `json:"recv_cnt"`
		HeapSize         int    `json:"heap_size"`
		MqueueLen        int    `json:"mqueue_len"`
		Connected        bool   `json:"connected"`
		ConnectedAt      string `json:"connected_at"`
		Clientid         string `json:"clientid"`
		RecvOct          int    `json:"recv_oct"`
		SendPkt          int    `json:"send_pkt"`
		RecvPkt          int    `json:"recv_pkt"`
		Username         string `json:"username"`
		MaxSubscriptions int    `json:"max_subscriptions"`
		CreatedAt        string `json:"created_at"`
		ProtoVer         int    `json:"proto_ver"`
		ExpiryInterval   int    `json:"expiry_interval"`
		MaxAwaitingRel   int    `json:"max_awaiting_rel"`
		RecvMsg          int    `json:"recv_msg"`
		Inflight         int    `json:"inflight"`
		Port             int    `json:"port"`
		MailboxLen       int    `json:"mailbox_len"`
		Node             string `json:"node"`
		MaxInflight      int    `json:"max_inflight"`
		SendMsg          int    `json:"send_msg"`
		Reductions       int    `json:"reductions"`
		Mountpoint       string `json:"mountpoint"`
		AwaitingRel      int    `json:"awaiting_rel"`
		MaxMqueue        int    `json:"max_mqueue"`
		SubscriptionsCnt int    `json:"subscriptions_cnt"`
		IPAddress        string `json:"ip_address"`
	} `json:"data"`
	Code int `json:"code"`
}