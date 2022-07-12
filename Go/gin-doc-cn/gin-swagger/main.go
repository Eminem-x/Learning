package main

import "gin-swagger/router"

func main() {
	router.InitRouter()

	// swag init -g router/router.go -o swagger/docs --parseDependency
	// rm -rf swagger && swag init -g router/router.go -o swagger/docs --parseDependency
}
