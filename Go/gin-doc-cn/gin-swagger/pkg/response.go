package pkg

type SuccessResponse struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data"`
}

type SuccessResponses struct {
	Total  int64       `json:"total"`
	Status string      `json:"status"`
	Data   interface{} `json:"data"`
}

type SuccessWithNoDataResponse struct {
	Status string `json:"status"`
}

type ErrorResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
