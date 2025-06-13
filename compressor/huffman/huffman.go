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
	getFrequency() int
	getId() int
}
type huffmanLeaf struct {
	freq, id int
	symbol   rune
}
type huffmanNode struct {
	freq, id    int
	left, right huffmanTree
}

type huffmanHeap []huffmanTree

type bitString string

func (hub *huffmanHeap) Push(item any) {
	*hub = append(*hub, item.(huffmanTree))
}

func (hub *huffmanHeap) Pop() any {
	popped := (*hub)[len(*hub)-1]
	(*hub) = (*hub)[:len(*hub)-1]
	return popped
}

func (hub huffmanHeap) Len() int {
	return len(hub)
}

func (hub huffmanHeap) Less(i, j int) bool {
	if hub[i].getFrequency() != hub[j].getFrequency() {
		return hub[i].getFrequency() < hub[j].getFrequency()
	}
	return hub[i].getId() < hub[j].getId()
}

func (hub huffmanHeap) Swap(i, j int) {
	hub[i], hub[j] = hub[j], hub[i]
}

func (leaf huffmanLeaf) getId() int {
	return leaf.id
}

func (leaf huffmanLeaf) getFrequency() int {
	return leaf.freq
}

func (node huffmanNode) getFrequency() int {
	return node.freq
}

func (node huffmanNode) getId() int {
	return node.id
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
	// fmt.Printf("[ decompress ] compressionHeader: %v\n", compressionHeader)
	symbolFreq := make(map[rune]int)
	var freq int
	for i := range len(compressionHeader) {
		if compressionHeader[i] == '|' && compressionHeader[i-1] != '|' {
			var err error
			if freq, err = strconv.Atoi(string(compressionHeader[i-1])); err != nil {
				panic(err)
			}
			if compressionHeader[i+1] != '\\' || i+2 >= len(compressionHeader) || compressionHeader[i+2] != 'n' {
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
	var keys []rune
	for r := range symbolFreq {
		keys = append(keys, r)
	}
	slices.Sort(keys)
	var treehub huffmanHeap
	monoId := 0
	for _, key := range keys {
		treehub = append(treehub, huffmanLeaf{
			freq:   symbolFreq[key],
			symbol: key,
			id:     monoId,
		})
		monoId++
	}
	heap.Init(&treehub)
	for treehub.Len() > 1 {
		x := heap.Pop(&treehub).(huffmanTree)
		y := heap.Pop(&treehub).(huffmanTree)
		heap.Push(&treehub, huffmanNode{
			freq:  x.getFrequency() + y.getFrequency(),
			left:  x,
			right: y,
			id:    monoId,
		})
		monoId++
	}
	return heap.Pop(&treehub).(huffmanTree)
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
			return -1, errors.New("[ getSymbol ] out of index error")
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
	// fmt.Printf("[ encode ] compressionHeader:\n%s\n\nlen(output.String()):%v\n\npaddingBits:%v\n\npaddingbyte:\n%v\n\ninputbytes:\n%v\n\n\n", compressionHeader.String(), len(output.String()), paddingBits, paddingByte, inputBytes)
	out := append([]byte(compressionHeader.String()), append([]byte("\\\n"), append(paddingByte, inputBytes...)...)...)
	// fmt.Printf("[ encode ] final out: %v\n", out)
	return out
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
