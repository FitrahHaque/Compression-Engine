package lz

import (
	"bytes"
	"errors"
	"io"
	"slices"
	"strconv"
	"sync"

	pb "github.com/cheggaaa/pb/v3"
)

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

type compressionCore struct {
	isInputBufferClosed bool
	lock                sync.Mutex
	inputBuffer         io.ReadWriter
	outputBuffer        io.ReadWriter
}

type CompressionWriter struct {
	core *compressionCore
}

type CompressionReader struct {
	core *compressionCore
}

type decompressionCore struct {
	isInputBufferClosed bool
	lock                sync.Mutex
	inputBuffer         io.ReadWriter
	outputBuffer        io.ReadWriter
}

type DecompressionWriter struct {
	core *decompressionCore
}

type DecompressionReader struct {
	core *decompressionCore
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
	compressedData := compress(originalData, 4096)
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

func NewCompressionReaderAndWriter() (io.ReadCloser, io.WriteCloser) {
	newCompressionCore := new(compressionCore)
	newCompressionCore.inputBuffer, newCompressionCore.outputBuffer = new(bytes.Buffer), new(bytes.Buffer)
	newCompressionCore.isInputBufferClosed = false
	newCompressionReader, newCompressionWriter := new(CompressionReader), new(CompressionWriter)
	newCompressionReader.core, newCompressionWriter.core = newCompressionCore, newCompressionCore
	return newCompressionReader, newCompressionWriter
}

func compress(content []byte, maxSearchBufferLength int) []byte {
	contentString := string(content)
	// fmt.Printf("[ lzss - compress ] contentString:%v\n", contentString)
	contentRune := []rune(contentString)
	contentRune = escapeConflictingSymbols(contentRune)

	bar := pb.New(len(contentRune))
	bar.Set(pb.Bytes, true)
	bar.Start()

	refChannels := make([]chan Reference, len(contentRune))
	for i := range len(contentRune) {
		refChannels[i] = make(chan Reference, 1)
		searchStartIdx := max(0, i-maxSearchBufferLength)
		nextEndIdx := min(len(contentRune), i+maxSearchBufferLength-1)
		// fmt.Printf("[ lzss - compress ] index %v\tsearchBuffer\n%v\n", i, string(content[searchStartIdx:i]))
		// fmt.Printf("[ lzss - compress ] index %v\tpattern\n%v\n", i, string(content[i:nextEndIdx]))
		go matchSearchBuffer(refChannels[i], contentRune[searchStartIdx:i], []rune{contentRune[i]}, contentRune[i+1:nextEndIdx])
	}

	var compressedContentRune []rune
	nextBytesToIgnore := 0
	for _, channel := range refChannels {
		ref := <-channel
		if nextBytesToIgnore > 0 {
			nextBytesToIgnore--
		} else if ref.isRef {
			// fmt.Printf("[ lzss - compress ] isRef at index %v for content: %v\n", i, string(ref.value))
			encoding := getSymbolEncoded(ref.negativeOffset, ref.size)
			if len(encoding) < ref.size {
				compressedContentRune = append(compressedContentRune, encoding...)
				nextBytesToIgnore = ref.size - 1
			} else {
				// fmt.Printf("[ lzss - compress ] ref not used at index: %v, content at loc: %v\n", i, string(ref.value[0]))
				compressedContentRune = append(compressedContentRune, ref.value[0])
			}
		} else {
			compressedContentRune = append(compressedContentRune, ref.value...)
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

func matchSearchBuffer(refChannel chan<- Reference, searchBuffer []rune, scanBytes []rune, nextBytes []rune) {
	pattern := append(scanBytes, nextBytes...)
	// fmt.Printf("[ lzss - matchSearchBuffer ] searchBuffer\n%v\n", string(searchBuffer))
	// fmt.Printf("[ lzss - matchSearchBuffer ] pattern\n%v\n", string(pattern))
	matchedLength, matchedAt := kmp(searchBuffer, pattern)
	var ref Reference
	if matchedLength > 1 {
		ref.isRef = true
		ref.value = pattern[:matchedLength]
		ref.size = matchedLength
		ref.negativeOffset = len(searchBuffer) - matchedAt
	} else {
		ref.isRef = false
		ref.value = scanBytes
		ref.size = len(scanBytes)
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

func (dw *DecompressionWriter) Write(data []byte) (int, error) {
	dw.core.lock.Lock()
	defer dw.core.lock.Unlock()
	return dw.core.inputBuffer.Write(data)
}

func (dw *DecompressionWriter) Close() error {
	dw.core.lock.Lock()
	defer dw.core.lock.Unlock()
	dw.core.isInputBufferClosed = true
	compressedData, err := io.ReadAll(dw.core.inputBuffer)
	if err != nil {
		return err
	}
	decompressedData, err := decompress(compressedData)
	if err != nil {
		return err
	}
	if _, err = dw.core.outputBuffer.Write(decompressedData); err != nil {
		return err
	}
	return nil
}

func (dr *DecompressionReader) Read(data []byte) (int, error) {
	dr.core.lock.Lock()
	defer dr.core.lock.Unlock()
	if !dr.core.isInputBufferClosed {
		return 0, errors.New("decompression failed because compression content upload has not been signaled as complete!")
	}
	return dr.core.outputBuffer.Read(data)
}

func (dr *DecompressionReader) Close() error {
	dr.core.lock.Lock()
	defer dr.core.lock.Unlock()
	if buf, ok := dr.core.inputBuffer.(*bytes.Buffer); ok {
		buf.Reset()
		return nil
	} else {
		return errors.New("Compression content buffer closing failure. Type assertion failed because underlying io.ReadWriter is not *bytes.Buffer.")
	}
}

func NewDecompressionReaderAndWriter() (io.ReadCloser, io.WriteCloser) {
	newDecompressionCore := new(decompressionCore)
	newDecompressionCore.inputBuffer, newDecompressionCore.outputBuffer = new(bytes.Buffer), new(bytes.Buffer)
	newDecompressionCore.isInputBufferClosed = false
	newDecompressionReader, newDecompressionWriter := new(DecompressionReader), new(DecompressionWriter)
	newDecompressionReader.core, newDecompressionWriter.core = newDecompressionCore, newDecompressionCore
	return newDecompressionReader, newDecompressionWriter
}

func decompress(content []byte) ([]byte, error) {
	contentString := string(content)
	contentRune := []rune(contentString)
	var err error
	if contentRune, err = decodeBackRefs(contentRune); err != nil {
		return nil, err
	}
	if contentRune, err = removeEscapes(contentRune); err != nil {
		return nil, err
	}
	decompressedContent := []byte(string(contentRune))
	return decompressedContent, nil
}

func decodeBackRefs(refedContent []rune) ([]rune, error) {
	refOn := false
	var currentNegOffset, currentLength, currentRefStart int
	var refValue []rune
	var derefedContent []rune
	for i := 0; i < len(refedContent); i++ {
		if refOn == false && refedContent[i] == Opening && countEscapesInReverse(refedContent, i-1)%2 == 0 {
			refValue = []rune{}
			currentRefStart = len(derefedContent)
			refOn = true
		} else if refOn == true {
			if refedContent[i] == Separator {
				var err error
				if currentNegOffset, err = strconv.Atoi(string(refValue)); err != nil {
					return nil, err
				}
				refValue = []rune{}
			} else if refedContent[i] == Closing {
				var err error
				if currentLength, err = strconv.Atoi(string(refValue)); err != nil {
					return nil, err
				}
				refOn = false
				derefedContent = append(derefedContent, replaceRef(derefedContent, currentRefStart, currentNegOffset, currentLength)...)
			} else {
				refValue = append(refValue, refedContent[i])
			}
		} else {
			derefedContent = append(derefedContent, refedContent[i])
		}
	}
	return derefedContent, nil
}

func countEscapesInReverse(content []rune, endIdx int) int {
	if endIdx < 0 {
		return 0
	}
	count := 0
	for i := endIdx; i >= 0; i-- {
		if content[i] != Escape {
			return count
		}
		count++
	}
	return count
}

func replaceRef(content []rune, refIdx, negOffset, length int) []rune {
	startIdx := refIdx - negOffset
	endIdx := startIdx + length
	return content[startIdx:endIdx]
}

func removeEscapes(content []rune) ([]rune, error) {
	var cleanedContent []rune
	for i := len(content) - 1; i >= 0; i-- {
		if slices.Contains(conflictingLiterals, content[i]) {
			if i == 0 || content[i-1] != Escape {
				return nil, errors.New("decompression failed due to conflicting literal not escaped in the compressed input")
			}
			cleanedContent = append(cleanedContent, content[i])
			i--
		} else {
			cleanedContent = append(cleanedContent, content[i])
		}
	}
	slices.Reverse(cleanedContent)
	return cleanedContent, nil
}
