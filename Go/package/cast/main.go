// @Title cast
// @Description go package for cast easily
// @Author Yuanhao
// @Update 2022-08-24
package main

import (
	"fmt"

	"github.com/spf13/cast"
)

// Official: https://github.com/spf13/cast
// Blog: https://darjun.github.io/2020/01/20/godailylib/cast/
func main() {
	ToType()
	ToTypeE()
	ToTypeSlice()
}

func ToType() {
	// ToString
	fmt.Println(cast.ToString("Yuanhao"))          // Yuanhao
	fmt.Println(cast.ToString(3))                  // 3
	fmt.Println(cast.ToString(3.14))               // 3.14
	fmt.Println(cast.ToString([]byte("Yuan Hao"))) // Yuan Hao
	fmt.Println(cast.ToString(nil))                // ""

	var foo interface{} = "Yuan Hao"
	fmt.Println(cast.ToString(foo)) // Yuan Hao

	// ToInt
	fmt.Println(cast.ToInt(3))     // 3
	fmt.Println(cast.ToInt(3.14))  // 3.14
	fmt.Println(cast.ToInt("3"))   // 3
	fmt.Println(cast.ToInt(true))  // 1
	fmt.Println(cast.ToInt(false)) // 0

	var eight interface{} = 8
	fmt.Println(cast.ToInt(eight)) // 8
	fmt.Println(cast.ToInt(nil))   // 0
}

func ToTypeE() {
	_, err := cast.ToIntE("Yuan")
	if err != nil {
		fmt.Printf("ToTypeE error: %#v\n", err)
	}
}

func ToTypeSlice() {
	sliceOfInt := []int{1, 3, 7}
	arrayOfInt := [3]int{8, 12}
	// ToIntSlice
	fmt.Println(cast.ToIntSlice(sliceOfInt)) // [1 3 7]
	fmt.Println(cast.ToIntSlice(arrayOfInt)) // [8 12 0]

	sliceOfInterface := []interface{}{1, 2.0, "darjun"}
	sliceOfString := []string{"abc", "dj", "pipi"}
	stringFields := " abc  def hij   "
	any := interface{}(37)
	// ToStringSliceE
	fmt.Println(cast.ToStringSlice(sliceOfInterface)) // [1 2 darjun]
	fmt.Println(cast.ToStringSlice(sliceOfString))    // [abc dj pipi]
	fmt.Println(cast.ToStringSlice(stringFields))     // [abc def hij]
	fmt.Println(cast.ToStringSlice(any))              // [37]
}
