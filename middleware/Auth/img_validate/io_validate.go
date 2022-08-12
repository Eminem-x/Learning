package main

import (
	"bytes"
	"fmt"
	"image/png"
	"io/ioutil"
	"net/http"
	"time"
)

func main() {

	url := "www.baidu.com/"

	client := http.Client{
		Timeout: 5 * time.Second,
	}

	res, err := client.Get(url)

	if err != nil {
		fmt.Println(err)
		return
	}
	defer res.Body.Close()

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
	}

	// byte slice to bytes.Reader, which implements the io.Reader interface
	reader := bytes.NewReader(data)

	_, err = png.Decode(reader)
	if err != nil {
		fmt.Println(err)
	}
}
