package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

func main() {
	jsonStr := []byte(`{ "dql": "SELECT * from users WHERE id = 1" }`) // ignore_security_alert
	url := ""
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-jwt-token", "")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("%v", err)
	}
	// 需要关闭 Body: https://zhuaxia.xyz/detail/21670
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	t := AutoGenerated{}
	err = json.Unmarshal(body, &t)
	fmt.Println(t.Data.Data[0].Email)
}

type AutoGenerated struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Amount int `json:"amount"`
		Data   []struct {
			ID               int         `json:"id"`
			CreatedAt        string      `json:"created_at"`
			UpdatedAt        string      `json:"updated_at"`
			DeletedAt        interface{} `json:"deleted_at"`
			NickName         string      `json:"nick_name"`
			Email            string      `json:"email"`
			EmailPrefix      string      `json:"email_prefix"`
			AvatarURL        string      `json:"avatar_url"`
			ChineseNickName  string      `json:"chinese_nick_name"`
			EnglishNickName  string      `json:"english_nick_name"`
			JapaneseNickName string      `json:"japanese_nick_name"`
			Country          string      `json:"country"`
			Province         string      `json:"province"`
			City             string      `json:"city"`
			Language         string      `json:"language"`
			Gender           string      `json:"gender"`
			TenantKey        string      `json:"tenant_key"`
			TannaID          interface{} `json:"tanna_id"`
			OpenHashID       string      `json:"open_hash_id"`
			UnionHashID      string      `json:"union_hash_id"`
			UserHashID       string      `json:"user_hash_id"`
			AccessToken      string      `json:"access_token"`
			RefreshToken     string      `json:"refresh_token"`
			ExpiresAt        string      `json:"expires_at"`
			IsLimited        interface{} `json:"is_limited"`
			DepartmentHashID string      `json:"department_hash_id"`
			LeaderUserHashID string      `json:"leader_user_hash_id"`
			MetaInfo         string      `json:"meta_info"`
		} `json:"data"`
		CostTimeMs int `json:"cost_time_ms"`
	} `json:"data"`
}