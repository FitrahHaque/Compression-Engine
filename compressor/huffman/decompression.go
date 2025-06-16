package huffman

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

type DecompressionWriter struct {
	core *decompressionCore
}
type DecompressionReader struct {
	core *decompressionCore
}

type decompressionCore struct {
	isInputBufferClosed bool
	lock                sync.Mutex
	inputBuffer         io.ReadWriter
	outputBuffer        io.ReadWriter
}

func (dr *DecompressionReader) Read(data []byte) (int, error) {
	dr.core.lock.Lock()
	defer dr.core.lock.Unlock()
	if !dr.core.isInputBufferClosed {
		return 0, errors.New("input buffer not closed")
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
		return errors.New("underlying io.ReadWriter is not *bytes.Buffer. Type assertion failed")
	}
}

func (dw *DecompressionWriter) Write(data []byte) (int, error) {
	dw.core.lock.Lock()
	defer dw.core.lock.Unlock()
	// fmt.Printf("[ DecompressionWriter.Write ] data: %v\n", data)
	return dw.core.inputBuffer.Write(data)
}

func (dw *DecompressionWriter) Close() error {
	dw.core.lock.Lock()
	defer dw.core.lock.Unlock()
	dw.core.isInputBufferClosed = true
	compressedData, err := io.ReadAll(dw.core.inputBuffer)
	// fmt.Printf("[ DecompressionWriter.Close ] compressedData: %v\n", compressedData)
	if err != nil {
		return err
	}
	decompressedData := decompress(compressedData)
	if _, err = dw.core.outputBuffer.Write(decompressedData); err != nil {
		return err
	}
	return nil
}

func NewDecompressionReaderAndWriter() (io.ReadCloser, io.WriteCloser) {
	newDecompressionCore := new(decompressionCore)
	newDecompressionCore.inputBuffer, newDecompressionCore.outputBuffer = new(bytes.Buffer), new(bytes.Buffer)
	newDecompressionCore.isInputBufferClosed = false
	newDecompressionReader, newDecompressionWriter := new(DecompressionReader), new(DecompressionWriter)
	newDecompressionReader.core, newDecompressionWriter.core = newDecompressionCore, newDecompressionCore
	return newDecompressionReader, newDecompressionWriter
}

func decompress(content []byte) []byte {
	contentString := string(content)
	compressionHeader := strings.SplitN(contentString, "\\\n", 2)[0]
	// fmt.Printf("[ decompress ] compressionHeader: %v\n", compressionHeader)
	headerRunes := []rune(compressionHeader)
	symbolFreq := make(map[rune]int)
	for i := range len(headerRunes) {
		if headerRunes[i] == '|' && headerRunes[i-1] != '|' {
			endFreq := i
			startFreq := endFreq - 1
			for startFreq > 0 && unicode.IsDigit(headerRunes[startFreq-1]) && (startFreq == 1 || headerRunes[startFreq-2] != rune('|')) {
				startFreq--
			}
			freq, err := strconv.Atoi(string(headerRunes[startFreq:endFreq]))
			if err != nil {
				panic(err)
			}
			if headerRunes[i+1] != rune('\\') || i+2 >= len(headerRunes) || headerRunes[i+2] != 'n' {
				symbolFreq[headerRunes[i+1]] = freq
			} else {
				symbolFreq[10] = freq
			}
		}
	}
	tree := buildTree(symbolFreq)
	decompressedData := decode(tree, contentString)
	return decompressedData
}

func getSymbolDecoded(root huffmanTree, huffmanCode string) *strings.Builder {
	var data strings.Builder
	switch node := root.(type) {
	case huffmanLeaf:
		fmt.Fprintf(&data, "%s", string(node.symbol))
		return &data
	case huffmanNode:
		for index := 0; index < len(huffmanCode); index++ {
			if huffmanCode[index] == '0' {
				var err error
				if index, err = getSymbol(node.left, huffmanCode, index, &data); err != nil {
					panic(err)
				}
			} else {
				var err error
				if index, err = getSymbol(node.right, huffmanCode, index, &data); err != nil {
					panic(err)
				}
			}
		}
	}
	return &data
}

func getSymbol(currentNode huffmanTree, huffmanCode string, index int, data *strings.Builder) (int, error) {
	switch node := currentNode.(type) {
	case huffmanLeaf:
		// fmt.Printf("[ getSymbol ] node.symbol %v\n", string(node.symbol))
		fmt.Fprintf(data, "%s", string(node.symbol))
		return index, nil
	case huffmanNode:
		index++
		if index >= len(huffmanCode) {
			return index, errors.New("[ getSymbol ] out of index error")
		}
		if huffmanCode[index] == '0' {
			return getSymbol(node.left, huffmanCode, index, data)
		} else {
			return getSymbol(node.right, huffmanCode, index, data)
		}
	default:
		return -1, errors.New("[ getSymbol ] type unknown")
	}
}

func decode(tree huffmanTree, input string) []byte {
	contentString := strings.SplitN(input, "\\\n", 2)[1]
	contentBytes := []byte(contentString)
	// fmt.Printf("[ decode ] contentString: %v\n", contentBytes)
	var huffmanCodeBuilder strings.Builder
	var offset int
	for i, bait := range contentBytes {
		if i > 0 {
			binary := fmt.Sprintf("%08b", bait)
			// fmt.Printf("[ decode ] bait: %v --- binary: %v\n", bait, binary)
			fmt.Fprintf(&huffmanCodeBuilder, "%s", binary)
		} else {
			offset = int(bait)
		}
	}
	// fmt.Printf("[ decode ] offset: %v\n", offset)
	huffmanCode := huffmanCodeBuilder.String()[offset:]
	// fmt.Printf("[ decode ] huffmanCode: %v\n", huffmanCode)
	var decompressedData *strings.Builder = getSymbolDecoded(tree, huffmanCode)
	// fmt.Printf("[ decode ] decompressedData: %v\n", decompressedData.String())
	return []byte(decompressedData.String())
}
