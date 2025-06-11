package huffman

import (
	"bytes"
	"container/heap"
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
	w io.Writer
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
type huffmanTree interface {
	frequency() int
}
type huffmanLeaf struct {
	freq   int
	symbol rune
}
type huffmanNode struct {
	freq        int
	left, right huffmanTree
}

type huffmanHeap []huffmanTree

type bitString string

func (hub *huffmanHeap) Push(item interface{}) {
	*hub = append(*hub, item.(huffmanTree))
}

func (hub *huffmanHeap) Pop() interface{} {
	popped := (*hub)[len(*hub)-1]
	(*hub) = (*hub)[:len(*hub)-1]
	return popped
}

func (hub huffmanHeap) Len() int {
	return len(hub)
}

func (hub huffmanHeap) Less(i, j int) bool {
	return hub[i].frequency() < hub[j].frequency()
}

func (hub huffmanHeap) Swap(i, j int) {
	hub[i], hub[j] = hub[j], hub[i]
}

func (leaf huffmanLeaf) frequency() int {
	return leaf.freq
}

func (node huffmanNode) frequency() int {
	return node.freq
}

func (cw *CompressionWriter) Write(data []byte) (int, error) {
	compressed := compress(data)
	return cw.w.Write(compressed)
}

func (cw *CompressionWriter) Close() error {
	return nil
}

func NewCompressionWriter(writer io.Writer) io.WriteCloser {
	newCW := new(CompressionWriter)
	newCW.w = writer
	return newCW
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

func decompress(content []byte) []byte {
	contentString := string(content)
	compressionHeader := strings.SplitN(contentString, "\\\n", 2)[0]
	symbolFreq := make(map[rune]int)
	var freq int
	for i := 0; i < len(compressionHeader); i++ {
		if compressionHeader[i] == '|' && compressionHeader[i-1] != '|' {
			var err error
			if freq, err = strconv.Atoi(string(compressionHeader[i-1])); err != nil {
				panic(err)
			}
			if compressionHeader[i+1] != '\\' && (i+2 >= len(compressionHeader) || compressionHeader[i+2] != 'n') {
				symbolFreq[rune(compressionHeader[i+1])] = freq
			} else {
				symbolFreq[10] = freq
			}
		}
	}
	tree := buildTree(symbolFreq)
	decompressedData := decode(tree, contentString)
	return decompressedData
}

func buildTree(symbolFreq map[rune]int) huffmanTree {
	var treehub huffmanHeap
	for key, value := range symbolFreq {
		treehub = append(treehub, huffmanLeaf{
			freq:   value,
			symbol: key,
		})
	}
	heap.Init(&treehub)
	for treehub.Len() > 1 {
		x := heap.Pop(&treehub).(huffmanTree)
		y := heap.Pop(&treehub).(huffmanTree)
		heap.Push(&treehub, huffmanNode{
			freq:  x.frequency() + y.frequency(),
			left:  x,
			right: y,
		})
	}
	return heap.Pop(&treehub).(huffmanTree)
}

func getSymbolEncoding(tree huffmanTree, symbolEnc map[rune]string, currentPrefix []byte) map[rune]string {
	switch i := tree.(type) {
	case huffmanLeaf:
		symbolEnc[i.symbol] = string(currentPrefix)
		// b := bitString(string(currentPrefix))
		// fmt.Printf("symbol: %s, currentPrefix: %s, in bytes: %v\n", string(i.symbol), string(currentPrefix), b.asByteSlice())
		return symbolEnc
	case huffmanNode:
		symbolEnc = getSymbolEncoding(i.left, symbolEnc, append(currentPrefix, byte('0')))
		symbolEnc = getSymbolEncoding(i.right, symbolEnc, append(currentPrefix, byte('1')))
		return symbolEnc
	}
	return symbolEnc
}

func getSymbol(root huffmanTree, currentNode huffmanTree, huffmanCode string, index int, data *strings.Builder) {
	if index >= len(huffmanCode) {
		return
	}
	switch node := currentNode.(type) {
	case huffmanLeaf:
		fmt.Fprintf(data, "%s", string(node.symbol))
		getSymbol(root, root, huffmanCode, index+1, data)
		return
	case huffmanNode:
		if huffmanCode[index] == '0' {
			getSymbol(root, node.left, huffmanCode, index+1, data)
		} else {
			getSymbol(root, node.right, huffmanCode, index+1, data)
		}
		return
	}
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
	symbolEnc = getSymbolEncoding(tree, symbolEnc, []byte{})
	for _, symbol := range input {
		if _, ok := symbolEnc[symbol]; !ok {
			fmt.Println("Symbol does not exist in huffman tree.")
			os.Exit(1)
		}
	}
	paddingBits := bitString(strconv.FormatInt(int64((8-len(output.String())%8)%8), 2))
	paddingByte := paddingBits.asByteSlice()
	inputBitString := bitString(output.String())
	inputBytes := inputBitString.asByteSlice()
	// fmt.Printf("compressionHeader:\n%s\n\npaddingbyte:\n%v\n\ninputbytes:\n%v\n\n\n", compressionHeader.String(), paddingByte, inputBytes)
	return append([]byte(compressionHeader.String()), append([]byte("\\\n"), append(paddingByte, inputBytes...)...)...)
}

func decode(tree huffmanTree, input string) []byte {
	contentString := strings.SplitN(input, "\\\n", 2)[1]
	contentBytes := []byte(contentString)
	var huffmanCodeBuilder strings.Builder
	var offset int
	for i, bait := range contentBytes {
		if i > 0 {
			binary := fmt.Sprintf("%08b", bait)
			fmt.Fprintf(&huffmanCodeBuilder, "%s", binary)
		} else {
			offset = int(bait)
		}
	}
	huffmanCode := huffmanCodeBuilder.String()[offset:]
	var decompressedData strings.Builder
	getSymbol(tree, tree, huffmanCode, 0, &decompressedData)
	return []byte(decompressedData.String())
}
