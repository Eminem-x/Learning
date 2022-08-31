package model

import "fmt"

// Monster monster
type Monster struct {
	Name string
}

// NewMonster
func NewMonster() Monster {
	return Monster{Name: "Godzilla"}
}

// Player
type Player struct {
	Name string
}

// NewPlayer
func NewPlayer(name string) Player {
	return Player{Name: name}
}

// Mission
type Mission struct {
	Player  Player
	Monster Monster
}

//NewMission
func NewMission(p Player, m Monster) Mission {
	return Mission{p, m}
}

// Start
func (m Mission) Start() {
	fmt.Printf("%s defeats %s, world peace!\n", m.Player.Name, m.Monster.Name)
}
