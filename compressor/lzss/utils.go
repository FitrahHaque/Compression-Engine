package lzss

const (
	Opening   = '<'
	Closing   = '>'
	Separator = ','
	Escape    = '\\'
)

type Reference struct {
	value          []rune
	isRef          bool
	negativeOffset int
	size           int
}

var conflictingLiterals = []rune{'<', '>', ',', '\\'}
