package flate

import (
	"bytes"
	"errors"
	"io"
	"sync"

	"github.com/FitrahHaque/Compression-Engine/compressor/huffman"
)

type DecompressionWriter struct {
	core *decompressionCore
}
type DecompressionReader struct {
	core *decompressionCore
}
type decompressionCore struct {
	isInputBufferClosed bool
	isEobReached        bool
	lock                sync.Mutex
	inputBuffer         io.ReadWriter
	outputBuffer        io.ReadWriter
	bitBuffer           *bitBuffer
	btype               uint32
	bfinal              uint32
	readChannel         chan byte
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
		dr.core.isInputBufferClosed = false
		return nil
	} else {
		return errors.New("underlying io.ReadWriter is not *bytes.Buffer. Type assertion failed")
	}
}

func (dw *DecompressionWriter) Write(data []byte) (int, error) {
	dw.core.lock.Lock()
	defer dw.core.lock.Unlock()
	if dw.core.isInputBufferClosed {
		return 0, errors.New("reading from the compression stream for the previous block has not completed yet!")
	}
	return dw.core.inputBuffer.Write(data)
}

func (dw *DecompressionWriter) Close() error {
	dw.core.lock.Lock()
	dw.core.isInputBufferClosed = true
	dw.core.lock.Unlock()
	return dw.decompress()
}

func NewDecompressionReaderAndWriter() (io.ReadCloser, io.WriteCloser) {
	newDecompressionCore := new(decompressionCore)
	newDecompressionCore.inputBuffer, newDecompressionCore.outputBuffer = new(bytes.Buffer), new(bytes.Buffer)
	newDecompressionCore.bitBuffer = new(bitBuffer)
	newDecompressionCore.isInputBufferClosed = false
	newDecompressionCore.readChannel = make(chan byte)
	newDecompressionReader, newDecompressionWriter := new(DecompressionReader), new(DecompressionWriter)
	newDecompressionReader.core, newDecompressionWriter.core = newDecompressionCore, newDecompressionCore
	return newDecompressionReader, newDecompressionWriter
}

func (dw *DecompressionWriter) decompress() error {
	dw.core.lock.Lock()
	defer dw.core.lock.Unlock()

	// bfinal
	if input, err := readCompressedContent(dw.core.bitBuffer, dw.core.inputBuffer, 1); err != nil {
		return err
	} else {
		dw.core.bfinal = input
	}

	// btype
	if input, err := readCompressedContent(dw.core.bitBuffer, dw.core.inputBuffer, 2); err != nil {
		return err
	} else {
		dw.core.btype = input
	}

	var HLIT, HDIST, HCLEN uint32

	// HLIT
	if input, err := readCompressedContent(dw.core.bitBuffer, dw.core.inputBuffer, 5); err != nil {
		return err
	} else {
		HLIT = input
	}
	// HDIST
	if input, err := readCompressedContent(dw.core.bitBuffer, dw.core.inputBuffer, 5); err != nil {
		return err
	} else {
		HDIST = input
	}

	// HCLEN
	if input, err := readCompressedContent(dw.core.bitBuffer, dw.core.inputBuffer, 4); err != nil {
		return err
	} else {
		HCLEN = input
	}

	HLIT += 257
	HDIST += 1
	HCLEN += 4

	// Code-Length Huffman Length
	var codeLengthHuffmanLengths []uint32
	for range HCLEN {
		if input, err := readCompressedContent(dw.core.bitBuffer, dw.core.inputBuffer, 3); err != nil {
			return err
		} else {
			codeLengthHuffmanLengths = append(codeLengthHuffmanLengths, input)
		}
	}
	newCodeLengthCode := new(CodeLengthCode)

	newCodeLengthCode.BuildHuffmanTree(codeLengthHuffmanLengths)
	dataReader := func(nbits uint) (uint32, error) {
		return readCompressedContent(dw.core.bitBuffer, dw.core.inputBuffer, nbits)
	}

	// Expanded Huffman Lengths
	litLenHuffmanLengths, distHuffmanLengths, err := newCodeLengthCode.ReadCondensedHuffman(dataReader, HLIT, HDIST)

}

func readCompressedContent(bb *bitBuffer, inputBuffer io.ReadWriter, nbits uint) (uint32, error) {
	if nbits > 32 {
		return 0, errors.New("cannot read more than 32 bits at once.")
	}
	for bb.bitsCount < nbits {
		newData := make([]byte, 1)
		if _, err := inputBuffer.Read(newData); err != nil {
			return 0, errors.New("not enough bits to read from the compressed data")
		}
		bb.bitsHolder |= uint32(newData[0]) << uint32(bb.bitsCount)
		bb.bitsCount += 8
	}
	output := bb.bitsHolder & ((1 << nbits) - 1)
	bb.bitsHolder >>= uint32(nbits)
	bb.bitsCount -= nbits
	return output, nil
}

func (clc *CodeLengthCode) BuildHuffmanTree(huffmanLengths []uint32) error {
	huffmanLengths = clc.reshuffle(huffmanLengths)
	if canonicalRoot, err := huffman.BuildCanonicalHuffmanDecoder(huffmanLengths); err != nil {
		return err
	} else {
		clc.CanonicalRoot = canonicalRoot
	}
	return nil
}

func (clc *CodeLengthCode) reshuffle(huffmanLengths []uint32) []uint32 {
	lengths := make([]uint32, 19)
	for i, length := range huffmanLengths {
		key := rleAlphabets.KeyOrder[i]
		lengths[key] = length
	}
	return lengths
}

func (clc *CodeLengthCode) ReadCondensedHuffman(dataReader func(uint) (uint32, error), HLIT, HDIST uint32) ([]uint32, []uint32, error) {
	remaining := HLIT + HDIST
	var concatenatedHuffmanLengths []uint32
	expandRule := func(rule int) ([]uint32, error) {
		extraBits := rleAlphabets.Alphabets[rule].ExtraBits
		var offset uint32
		if extraBits > 0 {
			if o, err := dataReader(uint(extraBits)); err != nil {
				return nil, err
			} else {
				offset = o
			}
		}
		var output []uint32
		if rule < 16 {
			return []uint32{uint32(rule)}, nil
		} else if rule == 16 {
			length := len(concatenatedHuffmanLengths)
			if length == 0 {
				return nil, errors.New("incorrectly condensed on empty slice")
			} else {
				n := rleAlphabets.Alphabets[rule].Base + int(offset)
				val := concatenatedHuffmanLengths[length-1]
				for range n {
					output = append(output, val)
				}
			}
		} else if rule < 19 {
			n := rleAlphabets.Alphabets[rule].Base + int(offset)
			for range n {
				output = append(output, 0)
			}
		} else {
			return nil, errors.New("no match found for the rule")
		}
		return output, nil
	}
	for remaining > 0 {
		if rule, err := TraverseHuffmanTree(dataReader, clc.CanonicalRoot); err != nil {
			return nil, nil, err
		} else if lengths, err := expandRule(int(rule)); err != nil {
			return nil, nil, err
		} else {
			concatenatedHuffmanLengths = append(concatenatedHuffmanLengths, lengths...)
		}
	}
	return concatenatedHuffmanLengths[:HLIT], concatenatedHuffmanLengths[HLIT : HLIT+HDIST], nil
}

func TraverseHuffmanTree(dataReader func(uint) (uint32, error), node *huffman.CanonicalHuffmanNode) (uint32, error) {
	if node.IsLeaf {
		return uint32(node.Item.GetValue()), nil
	}
	if input, err := dataReader(1); err != nil {
		return 0, err
	} else if input == 0 {
		if node.Left == nil {
			return 0, errors.New("tree traversal failed due to absence of appropriate subtree")
		} else {
			return TraverseHuffmanTree(dataReader, node.Left)
		}
	} else {
		if node.Right == nil {
			return 0, errors.New("tree traversal failed due to absence of appropriate subtree")
		} else {
			return TraverseHuffmanTree(dataReader, node.Right)
		}
	}
}
