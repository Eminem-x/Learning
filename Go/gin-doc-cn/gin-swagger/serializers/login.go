package serializers

type Login struct {
	Username string
	Password string
}

type LoginResponse struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func SerializeLogin() Login {
	return Login{
		Username: "ycx",
		Password: "yh",
	}
}
