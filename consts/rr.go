package consts

import "regexp"

type Command struct {
	MatchR *regexp.Regexp
	RetrieveR *regexp.Regexp
}

var Commands = []Command{
	{ regexp.MustCompile(`(?i)^кара`), nil },
}