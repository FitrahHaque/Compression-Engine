package flate

import (
	"bytes"
	"errors"
	"fmt"
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
	// fmt.Printf("[ flate.DecompressionWriter.Write ] data written to the inputBuffer\n")
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
	// fmt.Printf("[ flate.NewDecompressionReaderAndWriter ] newDecompressionCore: %v\n", newDecompressionCore)
	return newDecompressionReader, newDecompressionWriter
}

func (dw *DecompressionWriter) decompress() error {
	dw.core.lock.Lock()
	defer dw.core.lock.Unlock()

	dataReader := func(nbits uint) (uint32, error) {
		return readCompressedContent(dw.core.bitBuffer, dw.core.inputBuffer, nbits)
	}
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

	// fmt.Printf("[ flate.DecompressionWriter.decompress ] HLIT: %v, HDIST: %v, HCLEN: %v\n", HLIT, HDIST, HCLEN)

	// Code-Length Huffman Length
	var codeLengthHuffmanLengths []uint32
	for range HCLEN {
		if input, err := readCompressedContent(dw.core.bitBuffer, dw.core.inputBuffer, 3); err != nil {
			return err
		} else {
			codeLengthHuffmanLengths = append(codeLengthHuffmanLengths, input)
		}
	}
	// fmt.Printf("[ flate.DecompressionWriter.decompress ] codeLengthHuffmanLengths: %v\n", codeLengthHuffmanLengths)
	newCodeLengthCode := new(CodeLengthCode)
	newCodeLengthCode.BuildHuffmanTree(codeLengthHuffmanLengths)

	// Expanded Huffman Lengths
	newLitLengthCode := new(LitLengthCode)
	newDistanceCode := new(DistanceCode)
	if litLenHuffmanLengths, distHuffmanLengths, err := newCodeLengthCode.ReadCondensedHuffman(dataReader, HLIT, HDIST); err != nil {
		return err
	} else {
		// fmt.printf("[ flate.DecompressionWriter.decompress ] len(litLenHuffmanLengths): %v, len(distHuffmanLengths): %v\n", len(litLenHuffmanLengths), len(distHuffmanLengths))
		// fmt.printf("[ flate.DecompressionWriter.decompress ] litLenHuffmanLengths: %v, distHuffmanLengths: %v\n", litLenHuffmanLengths, distHuffmanLengths)
		if err := newLitLengthCode.BuildHuffmanTree(litLenHuffmanLengths); err != nil {
			return err
		}
		if err := newDistanceCode.BuildHuffmanTree(distHuffmanLengths); err != nil {
			return err
		}
	}
	// Now I have built all the huffman tree
	// Read Token, the huffman code is decoded.
	if tokens, err := ReadTokens(dataReader, newLitLengthCode, newDistanceCode); err != nil {
		return err
	} else {
		// tokens should be converted into text as the decompressed data
		data := DecodeTokens(tokens)
		// fmt.printf("[ flate.DecompressionWriter.decompress ] decompressed data: %v\n", string(data))
		if _, err := dw.core.outputBuffer.Write(data); err != nil {
			return err
		}
	}
	return nil
}

func DecodeTokens(tokens []Token) []byte {
	var output []byte
	findMatch := func(length, negOffset int) {
		outputSoFarRune := []rune(string(output))
		currentIdx := len(outputSoFarRune)
		startIdx := currentIdx - negOffset
		endIdx := startIdx + length
		match := []byte(string(outputSoFarRune[startIdx:endIdx]))
		// fmt.printf("[ flate.DecodeTokens.findMatch ] outputSoFar: %v\nmatch: %v\n", string(output), string(match))
		output = append(output, match...)
	}
	for _, token := range tokens {
		switch token.Kind {
		case LiteralToken:
			output = append(output, token.Value)
		case MatchToken:
			findMatch(token.Length, token.Distance)
		}
	}
	return output
}

func readCompressedContent(bb *bitBuffer, inputBuffer io.ReadWriter, nbits uint) (uint32, error) {
	if nbits > 32 {
		return 0, errors.New("cannot read more than 32 bits at once.")
	}
	for bb.bitsCount < nbits {
		newData := make([]byte, 1)
		if _, err := inputBuffer.Read(newData); err != nil {
			return 0, fmt.Errorf("not enough bits to read from the compressed data: %v\n", err)
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
		// traceTree(clc.CanonicalRoot, 0)
	}
	return nil
}

func (llc *LitLengthCode) BuildHuffmanTree(huffmanLengths []uint32) error {
	if canonicalRoot, err := huffman.BuildCanonicalHuffmanDecoder(huffmanLengths); err != nil {
		return err
	} else {
		llc.CanonicalRoot = canonicalRoot
		// traceTree(llc.CanonicalRoot, 0)
	}
	return nil
}

func (dc *DistanceCode) BuildHuffmanTree(huffmanLengths []uint32) error {
	if canonicalRoot, err := huffman.BuildCanonicalHuffmanDecoder(huffmanLengths); err != nil {
		return err
	} else {
		dc.CanonicalRoot = canonicalRoot
		// traceTree(dc.CanonicalRoot, 0)
	}
	return nil
}

func (clc *CodeLengthCode) reshuffle(huffmanLengths []uint32) []uint32 {
	lengths := make([]uint32, 19)
	for i, length := range huffmanLengths {
		key := rleAlphabets.KeyOrder[i]
		lengths[key] = length
	}
	// for i, length := range lengths {
	// 	if length != 0 {
	// 		fmt.printf("[ flate.CodeLengthCode.reshuffle ] RLECode: %v --- HuffmanCodeLength: %v\n", i, length)
	// 	}
	// }
	return lengths
}

func (clc *CodeLengthCode) ReadCondensedHuffman(dataReader func(uint) (uint32, error), HLIT, HDIST uint32) ([]uint32, []uint32, error) {
	total := HLIT + HDIST
	var concatenatedHuffmanLengths []uint32
	// fmt.printf("[ flate.CodeLengthCode.ReadCondensedHuffman.expandRule ] rule:\n")
	expandRule := func(rule int) ([]uint32, error) {
		extraBits := rleAlphabets.Alphabets[rule].ExtraBits
		var offset int
		if extraBits > 0 {
			if o, err := dataReader(uint(extraBits)); err != nil {
				return nil, err
			} else {
				offset = int(o)
			}
		}
		// fmt.Printf("code: %v, offset: %v\n", rule, offset)
		var output []uint32
		if rule < 16 {
			return []uint32{uint32(rule)}, nil
		} else if rule == 16 {
			length := len(concatenatedHuffmanLengths)
			if length == 0 {
				return nil, errors.New("incorrectly condensed on empty slice")
			} else {
				n := rleAlphabets.Alphabets[rule].Base + offset
				val := concatenatedHuffmanLengths[length-1]
				for range n {
					output = append(output, val)
				}
			}
		} else if rule < 19 {
			n := rleAlphabets.Alphabets[rule].Base + offset
			for range n {
				output = append(output, 0)
			}
		} else {
			return nil, errors.New("no match found for the rule")
		}
		return output, nil
	}
	cnt := 0
	for len(concatenatedHuffmanLengths) < int(total) {
		if rule, err := TraverseHuffmanTree(dataReader, clc.CanonicalRoot); err != nil {
			return nil, nil, err
		} else if lengths, err := expandRule(int(rule)); err != nil {
			return nil, nil, err
		} else {
			cnt++
			concatenatedHuffmanLengths = append(concatenatedHuffmanLengths, lengths...)
		}
	}
	// fmt.printf("[ flate.CodeLengthCode.ReadCondensedHuffman ] len(Rules): %v\n", cnt)
	// fmt.printf("[ flate.CodeLengthCode.ReadCondensedHuffman ] len(concatenatedHUffmanLengths): %v\n", len(concatenatedHuffmanLengths))
	// fmt.printf("[ flate.CodeLengthCode.ReadCondensedHuffman ] concatenatedHUffmanLengths: %v\n", concatenatedHuffmanLengths)
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

func ReadTokens(dataReader func(uint) (uint32, error), newlitLenthCode *LitLengthCode, newDistanceCode *DistanceCode) ([]Token, error) {
	var tokens []Token
	decodeLitLenRule := func(rule int) (TokenKind, int, int, error) {
		extraBits := lenAlphabets.Alphabets[rule].ExtraBits
		var offset int
		if extraBits > 0 {
			if o, err := dataReader(uint(extraBits)); err != nil {
				return 0, 0, 0, err
			} else {
				offset = int(o)
			}
		}
		if rule < 256 {
			return LiteralToken, rule, 0, nil
		} else if rule == 256 {
			return EndOfBlockToken, rule, 0, nil
		} else if rule < 286 {
			length := lenAlphabets.Alphabets[rule].Base + offset
			return MatchToken, length, offset, nil
		} else {
			return 0, 0, 0, errors.New("no match found for the rule")
		}
	}
	decodeDistRule := func(rule int) (int, int, error) {
		extraBits := distAlphabets.Alphabets[rule].ExtraBits
		var offset int
		if extraBits > 0 {
			if o, err := dataReader(uint(extraBits)); err != nil {
				return 0, 0, err
			} else {
				offset = int(o)
			}
		}
		distance := distAlphabets.Alphabets[rule].Base + offset
		return distance, offset, nil
	}
	for true {
		if rule, err := TraverseHuffmanTree(dataReader, newlitLenthCode.CanonicalRoot); err != nil {
			return nil, err
		} else {
			var token Token
			if tokenKind, value, lengthOffset, err := decodeLitLenRule(int(rule)); err != nil {
				return nil, err
			} else if tokenKind == MatchToken {
				token = Token{
					Kind:         tokenKind,
					Length:       value,
					LengthCode:   int(rule),
					LengthOffset: lengthOffset,
				}
				if rule, err := TraverseHuffmanTree(dataReader, newDistanceCode.CanonicalRoot); err != nil {
					return nil, err
				} else {
					if distance, distanceOffset, err := decodeDistRule(int(rule)); err != nil {
						return nil, err
					} else {
						token.Distance = distance
						token.DistanceCode = int(rule)
						token.DistanceOffset = distanceOffset
					}
				}
			} else if tokenKind == LiteralToken {
				token = Token{
					Kind:  tokenKind,
					Value: byte(value),
				}
			} else if tokenKind == EndOfBlockToken {
				return tokens, nil
			}
			tokens = append(tokens, token)
		}
	}
	return nil, errors.New("this line should never be reached")
}
