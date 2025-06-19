package lzss

const (
	Opening   = '<'
	Closing   = '>'
	Separator = ','
	Escape    = '\\'
)

type Reference struct {
	Value          []rune
	IsRef          bool
	NegativeOffset int
	Size           int
}

var conflictingLiterals = []rune{'<', '>', ',', '\\'}
