package lzss

import (
	"bytes"
	"errors"
	"io"
	"slices"
	"strconv"
	"sync"
)

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
