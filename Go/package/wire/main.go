// @Title
// @Description
// @Author
// @Update
package main

import (
	"go-wire/model"

	"github.com/google/wire"
)

// Official: https://github.com/google/wire
// Blog: https://darjun.github.io/2020/03/02/godailylib/wire/
func main() {
	withOutInjection()
}

func withOutInjection() {
	monster := model.NewMonster()
	player := model.NewPlayer("Eminem")
	mission := model.NewMission(player, monster)

	mission.Start()
}

func initMission(name string) model.Mission {
	// command line execute wire to build wire_gen.go
	wire.Build(model.NewMonster, model.NewPlayer, model.NewMission)
	return model.Mission{}
}
