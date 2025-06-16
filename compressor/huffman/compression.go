package huffman

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
)

type CompressionWriter struct {
	core *compressionCore
}
type CompressionReader struct {
	core *compressionCore
}

type compressionCore struct {
	isInputBufferClosed bool
	lock                sync.Mutex
	inputBuffer         io.ReadWriter
	outputBuffer        io.ReadWriter
}

func (cr *CompressionReader) Read(data []byte) (int, error) {
	cr.core.lock.Lock()
	defer cr.core.lock.Unlock()
	if !cr.core.isInputBufferClosed {
		return 0, errors.New("input buffer not closed")
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
		return errors.New("underlying io.ReadWriter is not *bytes.Buffer. Type assertion failed")
	}
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
	// fmt.Printf("[ DecompressionWriter.Close ] compressedData: %v\n", compressedData)
	if err != nil {
		return err
	}
	compressedData := compress(originalData)
	if _, err = cw.core.outputBuffer.Write(compressedData); err != nil {
		return err
	}
	return nil
}

func NewCompressionReaderAndWriter() (io.ReadCloser, io.WriteCloser) {
	newCompressionCore := new(compressionCore)
	newCompressionCore.inputBuffer, newCompressionCore.outputBuffer = new(bytes.Buffer), new(bytes.Buffer)
	newCompressionCore.isInputBufferClosed = false
	newCompressionReader, newCompressionWriter := new(CompressionReader), new(CompressionWriter)
	newCompressionReader.core, newCompressionWriter.core = newCompressionCore, newCompressionCore
	return newCompressionReader, newCompressionWriter
}

func compress(content []byte) []byte {
	contentString := string(content)
	symbolFreq := make(map[rune]int)
	for _, c := range contentString {
		symbolFreq[c]++
	}
	var compressionHeader strings.Builder
	for key, val := range symbolFreq {
		if key == 10 {
			fmt.Fprintf(&compressionHeader, "%s|\\n", strconv.Itoa(val))
		} else {
			fmt.Fprintf(&compressionHeader, "%s|%s", strconv.Itoa(val), string(key))
		}
	}
	tree := buildTree(symbolFreq)
	compressed := encode(tree, contentString, compressionHeader)
	return compressed
}

func getSymbolEncoding(tree huffmanTree, symbolEnc map[rune]string, currentPrefix []byte) {
	switch node := tree.(type) {
	case huffmanLeaf:
		symbolEnc[node.symbol] = string(currentPrefix)
		// b := bitString(string(currentPrefix))
		// fmt.Printf("[ getSymbolEncoding ] symbol: %s, currentPrefix: %s, in bytes: %v\n", string(node.symbol), string(currentPrefix), b.asByteSlice())
		return
	case huffmanNode:
		getSymbolEncoding(node.left, symbolEnc, append(currentPrefix, byte('0')))
		getSymbolEncoding(node.right, symbolEnc, append(currentPrefix, byte('1')))
		return
	}
	return
}

func (b bitString) asByteSlice() []byte {
	var output []byte
	for i := len(b); i > 0; i -= 8 {
		var chunk string
		if i < 8 {
			chunk = string(b[:i])
		} else {
			chunk = string(b[i-8 : i])
		}
		chunkInt, err := strconv.ParseUint(chunk, 2, 8)
		if err != nil {
			fmt.Println("Error converting string to byte for compression")
			os.Exit(1)
		}
		output = append(output, byte(chunkInt))
	}
	slices.Reverse(output)
	return output
}

func encode(tree huffmanTree, input string, compressionHeader strings.Builder) []byte {
	var output strings.Builder
	symbolEnc := make(map[rune]string)
	getSymbolEncoding(tree, symbolEnc, []byte{})
	for _, symbol := range input {
		encoding, ok := symbolEnc[symbol]
		if !ok {
			fmt.Println("Symbol does not exist in huffman tree.")
			os.Exit(1)
		}
		fmt.Fprintf(&output, "%s", encoding)
	}
	paddingBits := bitString(strconv.FormatInt(int64((8-len(output.String())%8)%8), 2))
	paddingByte := paddingBits.asByteSlice()
	// fmt.Printf("[ encode ] output: %v\n", output.String())
	inputBitString := bitString(output.String())
	inputBytes := inputBitString.asByteSlice()
	// fmt.Printf("[ encode ] compressionHeader:%s\n\nlen(output.String()):%v\n\npaddingBits:%v\n\npaddingbyte:\n%v\n\ninputbytes:\n%v\n\n\n", compressionHeader.String(), len(output.String()), paddingBits, paddingByte, inputBytes)
	out := append([]byte(compressionHeader.String()), append([]byte("\\\n"), append(paddingByte, inputBytes...)...)...)
	// fmt.Printf("[ encode ] final out: %v\n", out)
	return out
}
