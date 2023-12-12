package director

import "fmt"

type Director struct {}

func(director * Director) ProcessCommand() {
	fmt.Println(`Director is created`)
}