package lz

import (
	"io"
	"slices"
	"strconv"

	pb "github.com/cheggaaa/pb/v3"
)

const DefaultWindowSize = 4096
const (
	Opening   = '<'
	Closing   = '>'
	Separator = ','
)

type Reference struct {
	value          []byte
	isRef          bool
	negativeOffset int
	size           int
}

var conflictingLiterals = []rune{'<', '>', ',', '\\'}

type CompressionWriter struct {
	windowSize int
	writer     io.Writer
}

func (cw *CompressionWriter) Write(data []byte) (int, error) {
	compressed := compress(data, cw.windowSize)
	return cw.writer.Write(compressed)
}

func (cw *CompressionWriter) Close() error {
	return nil //will do the handoff here like huffman
}

func NewCompressionWriter(w io.Writer) io.WriteCloser {
	newCW := new(CompressionWriter)
	newCW.windowSize = DefaultWindowSize
	newCW.writer = w
	return newCW
}

func compress(content []byte, maxSearchBufferLength int) []byte {
	contentString := string(content)
	contentRune := []rune(contentString)
	contentRune = escapeConflictingSymbols(contentRune)
	contentString = string(contentRune)
	content = []byte(contentString)

	bar := pb.New(len(content))
	bar.Set(pb.Bytes, true)
	bar.Start()

	refChannels := make([]chan Reference, len(content))
	for i := range len(content) {
		refChannels[i] = make(chan Reference, 1)
		searchStartIdx := max(0, i-maxSearchBufferLength)
		nextEndIdx := min(len(content), i+maxSearchBufferLength)
		go matchSearchBuffer(refChannels[i], content[searchStartIdx:i], []byte{content[i]}, content[i:nextEndIdx])
	}

	var compressedContent []byte
	nextBytesToIgnore := 0
	for _, channel := range refChannels {
		ref := <-channel
		if nextBytesToIgnore > 0 {
			nextBytesToIgnore--
		} else if ref.isRef {
			encoding := getSymbolEncoded(ref.negativeOffset, ref.size)
			if len(encoding) < ref.size {
				compressedContent = append(compressedContent, encoding...)
				nextBytesToIgnore = ref.size - 1
			} else {
				compressedContent = append(compressedContent, ref.value[0])
			}
		} else {
			compressedContent = append(compressedContent, ref.value...)
		}
		bar.Increment()
	}
	return compressedContent
}

func findPrefix(pattern []byte) []int {
	pi := make([]int, len(pattern))
	for i := 1; i < len(pattern); i++ {
		j := pi[i-1]
		for j > 0 && pattern[i] != pattern[j] {
			j = pi[j-1]
		}
		if pattern[i] == pattern[j] {
			j++
		}
		pi[i] = j
	}
	return pi
}

func kmp(searchBuffer []byte, pattern []byte) (int, int) {
	pi := findPrefix(pattern)
	best, k, bestIndex := 0, 0, 0
	for i, b := range searchBuffer {
		for k > 0 && b != pattern[k] {
			k = pi[k-1]
		}
		if b == pattern[k] {
			k++
		}
		if best < k {
			best = k
			bestIndex = i - k + 1
		}
	}
	return best, bestIndex
}

func matchSearchBuffer(refChannel chan<- Reference, searchBuffer []byte, scanBytes []byte, nextBytes []byte) {
	pattern := append(scanBytes, nextBytes...)
	matchedLength, matchedAt := kmp(searchBuffer, pattern)
	var ref Reference
	if matchedLength > 1 {
		ref.isRef = true
		ref.value = pattern[:matchedLength]
		ref.size = matchedLength
		ref.negativeOffset = len(pattern) - matchedAt
	} else {
		ref.isRef = false
		ref.value = scanBytes
		ref.size = len(scanBytes)
	}
	refChannel <- ref
}

func escapeConflictingSymbols(content []rune) []rune {
	escapeRune := '\\'
	filteredContent := make([]rune, 0)
	for _, symbol := range content {
		if slices.Contains(conflictingLiterals, symbol) {
			filteredContent = append(filteredContent, []rune{escapeRune, symbol}...)
		} else {
			filteredContent = append(filteredContent, symbol)
		}
	}
	return filteredContent
}

func getSymbolEncoded(negOffset int, length int) []byte {
	return []byte(string(Opening) + strconv.Itoa(negOffset) + string(Separator) + strconv.Itoa(length) + string(Closing))
}
