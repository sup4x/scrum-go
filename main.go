package main

import (
	"fmt"
	"scrum/consts"
	"scrum/director"
)

func main() {
	director := director.Director{}
	director.ProcessCommand()
	var rr = consts.Commands[0].MatchR
	fmt.Println(rr.MatchString("кара 233"))
	fmt.Println(rr.MatchString(" 22"))
}