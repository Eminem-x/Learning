// @Title
// @Description
// @Author
// @Update
package common

import (
	"fmt"
	"time"
)

func GetFormatDate(timestamp time.Time) string {
	return timestamp.Format("2006-01-02")
}

func TestFormat() {
	timestamp := time.Now()
	fmt.Println(GetFormatDate(timestamp))
}
