package main

import (
	"fmt"

	"github.com/thedevsaddam/gojsonq"
)

// Official: https://github.com/thedevsaddam/gojsonq
// Blog: https://darjun.github.io/2020/02/24/godailylib/gojsonq/
func main() {
	basicUsage()
	ioUsage()
}

func basicUsage() {
	content := `{
  "user": {
    "name": "dj",
    "age": 18,
    "address": {
      "provice": "shanghai",
      "district": "xuhui"
    },
    "hobbies":["chess", "programming", "game"]
  }
}`

	gq := gojsonq.New().FromString(content)
	district := gq.Find("user.address.district")
	fmt.Println(district)

	// 在查询之后，我们手动调用了一次Reset()方法，
	// 因为JSONQ对象在调用Find方法时，内部会记录当前的节点，下一个查询会从上次查找的节点开始。
	gq.Reset()

	// 或者可以 copy 当前节点状态，以便后续使用
	// gpCopy := gq.Copy()

	hobby := gq.Find("user.hobbies.[0]")
	fmt.Println(hobby)
}

func ioUsage() {
	gq := gojsonq.New().File("./data.json")
	fmt.Println(gq.Find("items.[1].price"))
}
