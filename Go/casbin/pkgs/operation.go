package pkgs

import "casbin/model"

var (
	Authorized model.Users
)

func init() {
	Authorized = append(Authorized, model.User{
		ID:   1,
		Name: "Admin",
		Role: "admin",
	})
	Authorized = append(Authorized, model.User{
		ID:   2,
		Name: "Test",
		Role: "member",
	})
	Authorized = append(Authorized, model.User{
		ID:   3,
		Name: "Ycx",
		Role: "member",
	})
}
