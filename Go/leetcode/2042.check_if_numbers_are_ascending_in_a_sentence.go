import (
	"strconv"
	"strings"
)

func areNumbersAscending(s string) bool {
	strs := strings.Split(s, " ")
	var temp int
	for _, str := range strs {
		t, err := strconv.Atoi(str)
		if err == nil {
			if t > temp {
				temp = t
			} else {
				return false
			}
		}
	}
	return true
}
