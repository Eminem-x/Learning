func ValidateImage(url string) (isImage bool) {
	resp, err := http.Get(url) // ignore_security_alert
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.V1.Error("ValidateImage Body.Close err: %#v", err)
		}
	}(resp.Body)

	if err != nil {
		log.V1.Error("ValidateImage Get url err: %#v", err)
		return
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.V1.Error("ValidateImage ReadAll err: %#v", err)
		return
	}

	// 创建随机文件, 校验完毕后删除
	uuid, _ := uuid2.NewV4()
	path := uuid.String() + ".png"
	err = os.WriteFile(path, data, 0666)
	defer func(name string) {
		err := os.Remove(name) // ignore_security_alert
		if err != nil {
			log.V1.Error("os.Remove err: %#v", err)
		}
	}(path)

	if err != nil {
		log.V1.Error("ValidateImage WriteFile err: %#v", err)
		return
	}

	// 校验是否为合法图片
	image, err := os.Open(path)
	if err != nil {
		log.V1.Error("ValidateImage open err: %#v", err)
		return
	}

	_, err = png.Decode(image)
	if err != nil {
		log.V1.Error("ValidateImage png.Decode err: %#v", err)
		return
	}

	isImage = true
	return
}
