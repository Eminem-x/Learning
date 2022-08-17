// @Title
// @Description
// @Author
// @Update
package tool

import (
	"fmt"
	"time"

	"github.com/jinzhu/now"
)

func TestNow() {
	// calculatig time based on current time
	fmt.Println(time.Now()) // 2022-08-17 15:20:00 Wes
	fmt.Println(now.BeginningOfMinute())

	// parse string to time
	t, err := now.Parse("1999-12-12 12:20:21")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(t)
}
