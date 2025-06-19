package lzss

import (
	"bytes"
	"errors"
	"io"
	"slices"
	"strconv"
	"sync"

	pb "github.com/cheggaaa/pb/v3"
)

type compressionCore struct {
	isInputBufferClosed bool
	lock                sync.Mutex
	inputBuffer         io.ReadWriter
	outputBuffer        io.ReadWriter
	maxMatchDistance    int
	maxMatchLength      int
}

type CompressionWriter struct {
	core *compressionCore
}

type CompressionReader struct {
	core *compressionCore
}

func (cw *CompressionWriter) Write(data []byte) (int, error) {
	cw.core.lock.Lock()
	defer cw.core.lock.Unlock()
	return cw.core.inputBuffer.Write(data)
}

func (cw *CompressionWriter) Close() error {
	cw.core.lock.Lock()
	defer cw.core.lock.Unlock()
	cw.core.isInputBufferClosed = true
	originalData, err := io.ReadAll(cw.core.inputBuffer)
	if err != nil {
		return err
	}
	compressedData := compress(originalData, cw.core.maxMatchDistance, cw.core.maxMatchLength)
	if _, err = cw.core.outputBuffer.Write(compressedData); err != nil {
		return err
	}
	return nil
}

func (cr *CompressionReader) Read(data []byte) (int, error) {
	cr.core.lock.Lock()
	defer cr.core.lock.Unlock()
	if !cr.core.isInputBufferClosed {
		return 0, errors.New("compression failed because compression content upload has not been signaled as complete!")
	}
	return cr.core.outputBuffer.Read(data)
}

func (cr *CompressionReader) Close() error {
	cr.core.lock.Lock()
	defer cr.core.lock.Unlock()
	if buf, ok := cr.core.inputBuffer.(*bytes.Buffer); ok {
		buf.Reset()
		return nil
	} else {
		return errors.New("Original content buffer closing failure. Type assertion failed because underlying io.ReadWriter is not *bytes.Buffer.")
	}
}

func NewCompressionReaderAndWriter(matchDistance, matchLength int) (io.ReadCloser, io.WriteCloser) {
	newCompressionCore := new(compressionCore)
	newCompressionCore.inputBuffer, newCompressionCore.outputBuffer = new(bytes.Buffer), new(bytes.Buffer)
	newCompressionCore.isInputBufferClosed = false
	newCompressionCore.maxMatchDistance = matchDistance
	newCompressionCore.maxMatchLength = min(matchLength, matchDistance)
	newCompressionReader, newCompressionWriter := new(CompressionReader), new(CompressionWriter)
	newCompressionReader.core, newCompressionWriter.core = newCompressionCore, newCompressionCore
	return newCompressionReader, newCompressionWriter
}

func FindMatch(refChannels []chan Reference, content []rune, matchDistance, matchLength int) {
	for i := range len(content) {
		refChannels[i] = make(chan Reference, 1)
		searchStartIdx := max(0, i-matchDistance)
		nextEndIdx := min(len(content), i+matchLength)
		// fmt.Printf("[ lzss - compress ] index %v\tsearchBuffer\n%v\n", i, string(content[searchStartIdx:i]))
		// fmt.Printf("[ lzss - compress ] index %v\tpattern\n%v\n", i, string(content[i:nextEndIdx]))
		go matchSearchBuffer(refChannels[i], content[searchStartIdx:i], []rune{content[i]}, content[i+1:nextEndIdx])
	}
}

func compress(content []byte, matchDistance, matchLength int) []byte {
	contentString := string(content)
	// fmt.Printf("[ lzss - compress ] contentString:%v\n", contentString)
	contentRune := []rune(contentString)
	contentRune = escapeConflictingSymbols(contentRune)

	bar := pb.New(len(contentRune))
	bar.Set(pb.Bytes, true)
	bar.Start()

	refChannels := make([]chan Reference, len(contentRune))
	FindMatch(refChannels, contentRune, matchDistance, matchLength)
	var compressedContentRune []rune
	nextRunesToIgnore := 0
	for _, channel := range refChannels {
		ref := <-channel
		if nextRunesToIgnore > 0 {
			nextRunesToIgnore--
		} else if ref.IsRef {
			// fmt.Printf("[ lzss - compress ] isRef at index %v for content: %v\n", i, string(ref.value))
			encoding := getSymbolEncoded(ref.NegativeOffset, ref.Size)
			if len(encoding) < ref.Size {
				compressedContentRune = append(compressedContentRune, encoding...)
				nextRunesToIgnore = ref.Size - 1
			} else {
				// fmt.Printf("[ lzss - compress ] ref not used at index: %v, content at loc: %v\n", i, string(ref.value[0]))
				compressedContentRune = append(compressedContentRune, ref.Value[0])
			}
		} else {
			compressedContentRune = append(compressedContentRune, ref.Value...)
		}
		bar.Increment()
	}
	// fmt.Printf("[ lzss - compress ] compressContent\n%v\n", string(compressedContentRune))
	compressedContent := []byte(string(compressedContentRune))
	return compressedContent
}

func findPrefix(pattern []rune) []int {
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

func kmp(searchBuffer []rune, pattern []rune) (int, int) {
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
			if k == len(pattern) {
				break
			}
		}

	}
	return best, bestIndex
}

func matchSearchBuffer(refChannel chan<- Reference, searchBuffer []rune, scanRunes []rune, nextRunes []rune) {
	pattern := append(scanRunes, nextRunes...)
	// fmt.Printf("[ lzss - matchSearchBuffer ] searchBuffer\n%v\n", string(searchBuffer))
	// fmt.Printf("[ lzss - matchSearchBuffer ] pattern\n%v\n", string(pattern))
	matchedLength, matchedAt := kmp(searchBuffer, pattern)
	var ref Reference
	if matchedLength > 1 {
		ref.IsRef = true
		ref.Value = pattern[:matchedLength]
		ref.Size = matchedLength
		ref.NegativeOffset = len(searchBuffer) - matchedAt
	} else {
		ref.IsRef = false
		ref.Value = scanRunes
		ref.Size = len(scanRunes)
	}
	refChannel <- ref
}

func escapeConflictingSymbols(content []rune) []rune {
	filteredContent := make([]rune, 0)
	for _, symbol := range content {
		if slices.Contains(conflictingLiterals, symbol) {
			filteredContent = append(filteredContent, []rune{Escape, symbol}...)
		} else {
			filteredContent = append(filteredContent, symbol)
		}
	}
	return filteredContent
}

func getSymbolEncoded(negOffset int, length int) []rune {
	var output []rune
	output = append(output, Opening)
	output = append(output, []rune(strconv.Itoa(negOffset))...)
	output = append(output, Separator)
	output = append(output, []rune(strconv.Itoa(length))...)
	output = append(output, Closing)
	return output
}
