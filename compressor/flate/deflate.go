package flate

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/FitrahHaque/Compression-Engine/compressor/huffman"
	"github.com/FitrahHaque/Compression-Engine/compressor/lzss"
)

type TokenKind int

const (
	LiteralToken TokenKind = iota
	MatchToken
)

type DistanceAlphabets struct {
	alphabets []struct {
		extraBits    int
		baseDistance int
	}
}

type LengthAlphabets struct {
	alphabets map[int]struct {
		extraBits  int
		baseLength int
	}
	keyOrder []int
}

type CodeLengthAlphabets struct {
	alphabets map[int]struct {
		extraBits int
	}
	keyOrder []int
}

type LitLengthCode struct {
	LitLengthHuffman []huffman.CanonicalHuffmanCode
}
type DistanceCode struct {
	DistanceHuffman []huffman.CanonicalHuffmanCode
}
type CodeLengthCode struct {
	HuffmanLengthCondensed []struct {
		RLECode int
		Offset  int
	}
	CondensedHuffman []huffman.CanonicalHuffmanCode
}
type AlphabetCode interface {
	FindCode(value int) (int, int, error)
	Encode(items any) ([]int, error)
}

type Token struct {
	Kind           TokenKind
	Value          byte
	Length         int
	Distance       int
	DistanceCode   int
	DistanceOffset int
	LengthCode     int
	LengthOffset   int
}

var maxAllowedBackwardDistance int = 32768
var maxAllowedMatchLength int = 258
var lenAlphabets = LengthAlphabets{
	alphabets: map[int]struct {
		extraBits  int
		baseLength int
	}{
		257: {extraBits: 0, baseLength: 3}, 258: {extraBits: 0, baseLength: 4}, 259: {extraBits: 0, baseLength: 5}, 260: {extraBits: 0, baseLength: 6}, 261: {extraBits: 0, baseLength: 7}, 262: {extraBits: 0, baseLength: 8}, 263: {extraBits: 0, baseLength: 9}, 264: {extraBits: 0, baseLength: 10}, 265: {extraBits: 1, baseLength: 11}, 266: {extraBits: 1, baseLength: 13}, 267: {extraBits: 1, baseLength: 15}, 268: {extraBits: 1, baseLength: 17}, 269: {extraBits: 2, baseLength: 19}, 270: {extraBits: 2, baseLength: 23}, 271: {extraBits: 2, baseLength: 27}, 272: {extraBits: 3, baseLength: 31}, 273: {extraBits: 3, baseLength: 35}, 274: {extraBits: 3, baseLength: 43}, 275: {extraBits: 3, baseLength: 51}, 276: {extraBits: 3, baseLength: 59}, 277: {extraBits: 4}, 278: {extraBits: 4, baseLength: 83}, 279: {extraBits: 4, baseLength: 99}, 280: {extraBits: 4, baseLength: 115}, 281: {extraBits: 5, baseLength: 131}, 282: {extraBits: 5, baseLength: 163}, 283: {extraBits: 5, baseLength: 195}, 284: {extraBits: 5, baseLength: 227}, 285: {extraBits: 0, baseLength: 258},
	},
	keyOrder: []int{
		257, 258, 259, 260, 261, 262, 263, 264, 265, 266, 267, 268, 269, 270, 271, 272, 273, 274, 275, 276, 277, 278, 279, 280, 281, 282, 283, 284, 285,
	},
}

var distAlphabets = DistanceAlphabets{
	alphabets: []struct {
		extraBits    int
		baseDistance int
	}{
		{extraBits: 0, baseDistance: 1}, {extraBits: 0, baseDistance: 2}, {extraBits: 0, baseDistance: 3}, {extraBits: 0, baseDistance: 4}, {extraBits: 1, baseDistance: 5}, {extraBits: 1, baseDistance: 7}, {extraBits: 2, baseDistance: 9}, {extraBits: 2, baseDistance: 13}, {extraBits: 3, baseDistance: 17}, {extraBits: 3, baseDistance: 25}, {extraBits: 4, baseDistance: 33}, {extraBits: 4, baseDistance: 49}, {extraBits: 5, baseDistance: 65}, {extraBits: 5, baseDistance: 97}, {extraBits: 6, baseDistance: 129}, {extraBits: 6, baseDistance: 193}, {extraBits: 7, baseDistance: 257}, {extraBits: 7, baseDistance: 385}, {extraBits: 8, baseDistance: 513}, {extraBits: 8, baseDistance: 769}, {extraBits: 9, baseDistance: 1025}, {extraBits: 9, baseDistance: 1537}, {extraBits: 10, baseDistance: 2049}, {extraBits: 10, baseDistance: 3073}, {extraBits: 11, baseDistance: 4097}, {extraBits: 11, baseDistance: 6145}, {extraBits: 12, baseDistance: 8193}, {extraBits: 12, baseDistance: 12289}, {extraBits: 13, baseDistance: 16385}, {extraBits: 13, baseDistance: 24577},
	},
}

var rleAlphabets = CodeLengthAlphabets{
	alphabets: map[int]struct {
		extraBits int
	}{
		16: {extraBits: 2}, 17: {extraBits: 3}, 18: {extraBits: 7}, 0: {extraBits: 0}, 8: {extraBits: 0}, 7: {extraBits: 0}, 9: {extraBits: 0}, 6: {extraBits: 0}, 10: {extraBits: 0}, 5: {extraBits: 0}, 11: {extraBits: 0}, 4: {extraBits: 0}, 12: {extraBits: 0}, 3: {extraBits: 0}, 13: {extraBits: 0}, 2: {extraBits: 0}, 14: {extraBits: 0}, 1: {extraBits: 0}, 15: {extraBits: 0},
	},
	keyOrder: []int{
		16, 17, 18, 0, 8, 7, 9, 6, 10, 5, 11, 4, 12, 3, 13, 2, 14, 1, 15,
	},
}

type CompressionWriter struct {
	core *compressionCore
}
type CompressionReader struct {
	core *compressionCore
}
type bitBuffer struct {
	output     io.ReadWriter
	bitsHolder uint32
	bitsCount  uint
}

type compressionCore struct {
	isInputBufferClosed bool
	lock                sync.Mutex
	inputBuffer         io.ReadWriter
	outputBuffer        *bitBuffer
	btype               uint32
	bfinal              uint32
}

func (cr *CompressionReader) Read(data []byte) (int, error) {
	cr.core.lock.Lock()
	defer cr.core.lock.Unlock()
	if !cr.core.isInputBufferClosed {
		return 0, errors.New("input buffer not closed")
	}
	return cr.core.outputBuffer.output.Read(data)
}

func (cr *CompressionReader) Close() error {
	cr.core.lock.Lock()
	defer cr.core.lock.Unlock()
	if buf, ok := cr.core.inputBuffer.(*bytes.Buffer); ok {
		buf.Reset()
		cr.core.isInputBufferClosed = false
		return nil
	} else {
		return errors.New("underlying io.ReadWriter is not *bytes.Buffer. Type assertion failed")
	}
}

func (cw *CompressionWriter) Write(data []byte) (int, error) {
	cw.core.lock.Lock()
	defer cw.core.lock.Unlock()
	if cw.core.isInputBufferClosed {
		return 0, errors.New("reading from the compression stream for the previous block has not completed yet!")
	}
	fmt.Printf("[ flate.CompressionWriter.Write ] data written to inputBuffer\n")
	return cw.core.inputBuffer.Write(data)
}

func (cw *CompressionWriter) Close() error {
	cw.core.lock.Lock()
	cw.core.isInputBufferClosed = true
	originalData, err := io.ReadAll(cw.core.inputBuffer)
	cw.core.lock.Unlock()
	// fmt.Printf("[ DecompressionWriter.Close ] compressedData: %v\n", compressedData)
	if err != nil {
		return err
	}
	return cw.compress(originalData)
}

func NewCompressionReaderAndWriter(btype uint32, bfinal uint32) (io.ReadCloser, io.WriteCloser) {
	newCompressionCore := new(compressionCore)
	newBitBuffer := new(bitBuffer)
	newBitBuffer.output = new(bytes.Buffer)
	newCompressionCore.inputBuffer, newCompressionCore.outputBuffer = new(bytes.Buffer), newBitBuffer
	newCompressionCore.isInputBufferClosed = false
	newCompressionCore.btype = btype
	newCompressionCore.bfinal = bfinal
	newCompressionReader, newCompressionWriter := new(CompressionReader), new(CompressionWriter)
	newCompressionReader.core, newCompressionWriter.core = newCompressionCore, newCompressionCore
	fmt.Printf("[ flate.NewCompressionReaderAndWriter ] newCompressionCore: %v\n", newCompressionCore)
	return newCompressionReader, newCompressionWriter
}

func (dc *DistanceCode) FindCode(value int) (code int, offset int, err error) {
	if value < 1 || value > maxAllowedBackwardDistance {
		return 0, 0, errors.New("value is out of range to have a match with RFC distance code")
	}
	for i, info := range distAlphabets.alphabets {
		if value >= info.baseDistance && (i+1 == len(distAlphabets.alphabets) || value < distAlphabets.alphabets[i+1].baseDistance) {
			offset := value - info.baseDistance
			return i, offset, nil
		}
	}
	return 0, 0, fmt.Errorf("no distance code found for the distance value %v\n", value)
}

func (dc *DistanceCode) Encode(items any) ([]int, error) {
	tokens, ok := items.([]Token)
	if !ok {
		return nil, errors.New("distance huffman tree cannot be generated without the type of Token slice")
	}
	symbolFreq := make([]int, 30)
	for i := range tokens {
		token := &tokens[i]
		if token.Kind == MatchToken {
			if code, offset, err := dc.FindCode(token.Distance); err != nil {
				return nil, err
			} else {
				token.DistanceCode, token.DistanceOffset = code, offset
				symbolFreq[token.DistanceCode]++
				fmt.Printf("[ flate.DistanceCode.Encode ] Distance: %v --- DistanceCode: %v, DistanceOffset: %v\n", token.Distance, token.DistanceCode, token.DistanceOffset)
			}
		}
	}
	if distHuffmanCode, err := huffman.BuildCanonicalHuffmanTree(symbolFreq, 15); err != nil {
		return nil, err
	} else {
		for i, huffman := range distHuffmanCode {
			fmt.Printf("[ flate.DistanceCode.Encode ] DistanceCode: %v --- HuffmanCode: %v, HuffmanCodeLength: %v\n", i, huffman.Code, huffman.Length)
		}
		dc.DistanceHuffman = distHuffmanCode
		return findLengthBoundary(distHuffmanCode, 0, 15)
	}
}

func (llc *LitLengthCode) FindCode(value int) (code int, offset int, err error) {
	if value < 3 || value > maxAllowedMatchLength {
		return 0, 0, errors.New("value is out of range to have a match with RFC length code")
	}
	for _, key := range lenAlphabets.keyOrder {
		if nextInfo, ok := lenAlphabets.alphabets[key+1]; value >= lenAlphabets.alphabets[key].baseLength && (!ok || value < nextInfo.baseLength) {
			// fmt.Printf("[ flate.FindCode ] ")
			offset := value - lenAlphabets.alphabets[key].baseLength
			return key, offset, nil
		}
	}
	return 0, 0, fmt.Errorf("no length code found for the length value %v\n", value)
}

func (llc *LitLengthCode) Encode(items any) ([]int, error) {
	tokens, ok := items.([]Token)
	if !ok {
		return nil, errors.New("length huffman code cannot be generated without the type of Token slice")
	}
	symbolFreq := make([]int, 286)
	for i := range tokens {
		token := &tokens[i]
		if token.Kind == LiteralToken {
			symbolFreq[token.Value]++
			fmt.Printf("[ flate.LitLengthCode.Encode ] Literal: %v --- Code: %v\n", string(token.Value), token.Value)
		} else {
			if code, offset, err := llc.FindCode(token.Length); err != nil {
				return nil, err
			} else {
				token.LengthCode, token.LengthOffset = code, offset
				symbolFreq[token.LengthCode]++
				fmt.Printf("[ flate.LitLengthCode.Encode ] Length: %v --- LengthCode: %v, LengthOffset: %v\n", token.Length, token.LengthCode, token.LengthOffset)
			}
		}
	}
	symbolFreq[256]++
	if litLenHuffmanCode, err := huffman.BuildCanonicalHuffmanTree(symbolFreq, 15); err != nil {
		return nil, err
	} else {
		for i, huffman := range litLenHuffmanCode {
			fmt.Printf("[ flate.LitLengthCode.Encode ] LengthCode: %v --- HuffmanCode: %v, HuffmanCodeLength: %v\n", i, huffman.Code, huffman.Length)
		}
		llc.LitLengthHuffman = litLenHuffmanCode
		return findLengthBoundary(litLenHuffmanCode, 256, 15)
	}
}

func (clc *CodeLengthCode) FindCode(lengthHuffmanLengths []int) (err error) {
	countZero, countSame := 0, 0
	resolveCountZero := func() error {
		if countZero != 0 {
			if countZero < 3 {
				for range countZero {
					clc.HuffmanLengthCondensed = append(clc.HuffmanLengthCondensed, struct {
						RLECode int
						Offset  int
					}{
						RLECode: 0,
						Offset:  0,
					})
				}
			} else if countZero < 11 {
				clc.HuffmanLengthCondensed = append(clc.HuffmanLengthCondensed, struct {
					RLECode int
					Offset  int
				}{
					RLECode: 17,
					Offset:  countZero - 3,
				})
			} else if countZero < 138 {
				clc.HuffmanLengthCondensed = append(clc.HuffmanLengthCondensed, struct {
					RLECode int
					Offset  int
				}{
					RLECode: 18,
					Offset:  countZero - 11,
				})
			} else {
				return errors.New("such a long sequence of zeros cannot be encoded")
			}
		}
		return nil
	}
	resolveCountSame := func(prevIndex int) error {
		if countSame != 0 {
			if countSame < 3 {
				for range countSame {
					clc.HuffmanLengthCondensed = append(clc.HuffmanLengthCondensed, struct {
						RLECode int
						Offset  int
					}{
						RLECode: lengthHuffmanLengths[prevIndex],
						Offset:  0,
					})
				}
			} else if countSame < 6 {
				clc.HuffmanLengthCondensed = append(clc.HuffmanLengthCondensed, struct {
					RLECode int
					Offset  int
				}{
					RLECode: 16,
					Offset:  countSame - 3,
				})
			} else {
				return errors.New("such a long sequence of non-zeros cannot be encoded")
			}
			countSame = 0
		}
		return nil
	}
	for i, length := range lengthHuffmanLengths {
		if length == 0 {
			if err := resolveCountSame(i - 1); err != nil {
				return err
			}
			countZero++
			if countZero == 138 {
				clc.HuffmanLengthCondensed = append(clc.HuffmanLengthCondensed, struct {
					RLECode int
					Offset  int
				}{
					RLECode: 18,
					Offset:  countZero - 11,
				})
				countZero = 0
			}
		} else if i == 0 || length != lengthHuffmanLengths[i-1] {
			if err := resolveCountZero(); err != nil {
				return err
			}
			if err := resolveCountSame(i - 1); err != nil {
				return err
			}
			clc.HuffmanLengthCondensed = append(clc.HuffmanLengthCondensed, struct {
				RLECode int
				Offset  int
			}{
				RLECode: length,
				Offset:  0,
			})
		} else {
			countSame++
			if countSame == 6 {
				clc.HuffmanLengthCondensed = append(clc.HuffmanLengthCondensed, struct {
					RLECode int
					Offset  int
				}{
					RLECode: 16,
					Offset:  countSame - 3,
				})
				countSame = 0
			}
		}
	}
	if err := resolveCountZero(); err != nil {
		return err
	}
	if err := resolveCountSame(len(lengthHuffmanLengths) - 1); err != nil {
		return err
	}
	fmt.Printf("[ flate.CodeLengthCode.FindCode ] lengthHuffmanLength: %v\n", lengthHuffmanLengths)
	for i, condensed := range clc.HuffmanLengthCondensed {
		fmt.Printf("[ flate.CodeLengthCode.FindCode ] clc.HuffmanLengthCondensed[%v] --- RLECode: %v, Offset: %v\n", i, condensed.RLECode, condensed.Offset)
	}
	return nil
}

func (clc *CodeLengthCode) Encode(items any) ([]int, error) {
	lengthHuffmanLengths, ok := items.([]int)
	if !ok {
		return nil, errors.New("code-length huffman code cannot be generated without the type of int slice")
	}
	if err := clc.FindCode(lengthHuffmanLengths); err != nil {
		return nil, err
	}
	symbolFreq := make([]int, 19)
	for _, info := range clc.HuffmanLengthCondensed {
		symbolFreq[info.RLECode]++
	}
	if codeLengthHuffmanCode, err := huffman.BuildCanonicalHuffmanTree(symbolFreq, 3); err != nil {
		return nil, err
	} else {
		clc.CondensedHuffman = make([]huffman.CanonicalHuffmanCode, len(codeLengthHuffmanCode))
		copy(clc.CondensedHuffman, codeLengthHuffmanCode)
		for i, huffman := range clc.CondensedHuffman {
			fmt.Printf("[ flate.CodeLengthCode.Encode ] RLECode: %v --- HuffmanCode: %v, HuffmanCodeLength: %v\n", i, huffman.Code, huffman.Length)
		}
		clc.reorder(codeLengthHuffmanCode)
		for i, huffman := range codeLengthHuffmanCode {
			key := rleAlphabets.keyOrder[i]
			fmt.Printf("[ flate.CodeLengthCode.Encode ] RLECode: %v --- HuffmanCode: %v, HuffmanCodeLength: %v\n", key, huffman.Code, huffman.Length)
		}
		return findLengthBoundary(codeLengthHuffmanCode, 3, 3)
	}
}

func (clc *CodeLengthCode) reorder(code []huffman.CanonicalHuffmanCode) {
	var huffmanLengths []huffman.CanonicalHuffmanCode
	for _, key := range rleAlphabets.keyOrder {
		huffmanLengths = append(huffmanLengths, code[key])
	}
	code = huffmanLengths
}

func (cw *CompressionWriter) compress(content []byte) error {
	contentRune := []rune(string(content))
	fmt.Printf("[ flate.CompressionWriter.compress ] contentString %v\n", string(content))
	refChannels := make([]chan lzss.Reference, len(contentRune))
	lzss.FindMatch(refChannels, contentRune, maxAllowedBackwardDistance, maxAllowedMatchLength)
	tokens, err := tokeniseLZSS(refChannels)
	if err != nil {
		return err
	}
	newLitLengthCode := new(LitLengthCode)
	litLenHuffmanLengths, err := newLitLengthCode.Encode(tokens)
	if err != nil {
		return err
	}
	newDistanceCode := new(DistanceCode)
	distHuffmanLengths, err := newDistanceCode.Encode(tokens)
	if err != nil {
		return err
	}
	concatenatedHuffmanLengths := append(litLenHuffmanLengths, distHuffmanLengths...)
	fmt.Printf("[ flate.CompressinWriter.compress ] len(concatenatedHuffmanLengths): %v\n", len(concatenatedHuffmanLengths))
	newCodeLengthCode := new(CodeLengthCode)
	codeLengthHuffmanLengths, err := newCodeLengthCode.Encode(concatenatedHuffmanLengths)
	if err != nil {
		return err
	}
	HLIT := len(litLenHuffmanLengths) - 257
	HDIST := len(distHuffmanLengths) - 1
	HCLEN := len(codeLengthHuffmanLengths) - 4
	cw.core.lock.Lock()
	defer cw.core.lock.Unlock()
	fmt.Printf("[ flate.CompressionWriter.compress ] bfinal: %v\n", cw.core.bfinal)
	cw.core.outputBuffer.writeCompressedContent(cw.core.bfinal, 1)
	fmt.Printf("[ flate.CompressionWriter.compress ] btype: %v\n", cw.core.btype)
	cw.core.outputBuffer.writeCompressedContent(cw.core.btype, 2)
	fmt.Printf("[ flate.CompressionWriter.compress ] HLIT: %v\n", HLIT)
	cw.core.outputBuffer.writeCompressedContent(uint32(HLIT), 5)
	fmt.Printf("[ flate.CompressionWriter.compress ] HDIST: %v\n", HDIST)
	cw.core.outputBuffer.writeCompressedContent(uint32(HDIST), 5)
	fmt.Printf("[ flate.CompressionWriter.compress ] HCLEN: %v\n", HCLEN)
	cw.core.outputBuffer.writeCompressedContent(uint32(HCLEN), 4)
	for _, codeLen := range codeLengthHuffmanLengths {
		// fmt.Printf("[ flate.CompressionWriter.compress ] code")
		cw.core.outputBuffer.writeCompressedContent(uint32(codeLen), 3)
	}
	for _, code := range newCodeLengthCode.HuffmanLengthCondensed {
		condensedHuff := newCodeLengthCode.CondensedHuffman[code.RLECode]
		fmt.Printf("[ flate.CompressionWriter.compress ] Condensed -- RLECode: %v --- HuffmanCode: %v, HuffmanCodeLength: %v\n", code.RLECode, condensedHuff.Code, condensedHuff.Length)
		cw.core.outputBuffer.writeCompressedContent(uint32(condensedHuff.Code), uint(condensedHuff.Length))
		if rleAlphabets.alphabets[code.RLECode].extraBits > 0 {
			fmt.Printf("[ flate.CompressionWriter.compress ] Condensed -- RLECode: %v, Offset: %v --- bitlength: %v\n", code.RLECode, code.Offset, rleAlphabets.alphabets[code.RLECode].extraBits)
			cw.core.outputBuffer.writeCompressedContent(uint32(code.Offset), uint(rleAlphabets.alphabets[code.RLECode].extraBits))
		}
	}
	for _, token := range tokens {
		if token.Kind == LiteralToken {
			litLenHuff := newLitLengthCode.LitLengthHuffman[token.Value]
			fmt.Printf("[ flate.CompressionWriter.compress ] Literal: %v --- HuffmanCode: %v, HuffmanCodeLength: %v\n", string(token.Value), litLenHuff.Code, litLenHuff.Length)
			cw.core.outputBuffer.writeCompressedContent(uint32(litLenHuff.Code), uint(litLenHuff.Length))
		} else {
			litLenHuff := newLitLengthCode.LitLengthHuffman[token.LengthCode]
			fmt.Printf("[ flate.CompressionWriter.compress ] Length: %v, LengthCode: %v --- HuffmanCode: %v, HuffmanCodeLength: %v\n", token.Length, token.LengthCode, litLenHuff.Code, litLenHuff.Length)
			cw.core.outputBuffer.writeCompressedContent(uint32(litLenHuff.Code), uint(litLenHuff.Length))
			if lenAlphabets.alphabets[token.LengthCode].extraBits > 0 {
				fmt.Printf("[ flate.CompressionWriter.compress ] Length: %v, LengthCode: %v, Offset: %v --- bitLength: %v\n", token.Length, litLenHuff.Code, token.LengthOffset, lenAlphabets.alphabets[token.LengthCode].extraBits)
				cw.core.outputBuffer.writeCompressedContent(uint32(token.LengthOffset), uint(lenAlphabets.alphabets[token.LengthCode].extraBits))
			}
			distHuff := newDistanceCode.DistanceHuffman[token.DistanceCode]
			fmt.Printf("[ flate.CompressionWriter.compress ] Distance: %v, DistanceCode: %v --- HuffmanCode: %v, HuffmanCodeLength: %v\n", token.Distance, token.DistanceCode, distHuff.Code, distHuff.Length)
			cw.core.outputBuffer.writeCompressedContent(uint32(distHuff.Code), uint(distHuff.Length))
			if distAlphabets.alphabets[token.DistanceCode].extraBits > 0 {
				fmt.Printf("[ flate.CompressionWriter.compress ] Distance: %v, DistanceCode: %v, Offset: %v --- bitLength: %v\n", token.Distance, distHuff.Code, token.DistanceOffset, distAlphabets.alphabets[token.DistanceCode].extraBits)
				cw.core.outputBuffer.writeCompressedContent(uint32(token.DistanceOffset), uint(distAlphabets.alphabets[token.DistanceCode].extraBits))
			}
		}
	}
	eobHuff := newLitLengthCode.LitLengthHuffman[256]
	fmt.Printf("[ flate.CompressionWriter.compress ] EOB: %v --- HuffmanCode: %v, HuffmanCodeLength: %v\n", 256, eobHuff.Code, eobHuff.Length)
	cw.core.outputBuffer.writeCompressedContent(uint32(eobHuff.Code), uint(eobHuff.Length))
	return cw.core.outputBuffer.flushAlign()
}

func tokeniseLZSS(refChannels []chan lzss.Reference) ([]Token, error) {
	var tokens []Token
	nextRunesToIgnore := 0
	for i, channel := range refChannels {
		ref := <-channel
		if nextRunesToIgnore > 0 {
			nextRunesToIgnore--
		} else if !ref.IsRef || ref.Size < 3 {
			literalBytes := []byte(string(ref.Value[0]))
			fmt.Printf("[ flate.tokeniseLZSS ] no match on index %v -- literal: %v\n", i, string(ref.Value[0]))
			for _, literalByte := range literalBytes {
				token := Token{
					Kind:  LiteralToken,
					Value: literalByte,
				}
				tokens = append(tokens, token)
			}
		} else {
			if ref.Size > ref.NegativeOffset {
				return nil, errors.New("token match overlapping with the reference")
			}
			if ref.Size > maxAllowedMatchLength {
				return nil, fmt.Errorf("token match cannot be longer than %v\n", maxAllowedMatchLength)
			}
			if ref.NegativeOffset > maxAllowedBackwardDistance {
				return nil, fmt.Errorf("token match cannot be farther backward than %v\n", maxAllowedBackwardDistance)
			}
			nextRunesToIgnore = ref.Size - 1
			token := Token{
				Kind:     MatchToken,
				Length:   ref.Size,
				Distance: ref.NegativeOffset,
			}
			fmt.Printf("[ flate.tokeniseLZSS ] match on index %v -- Length: %v, Distance: %v\n", i, ref.Size, ref.NegativeOffset)
			tokens = append(tokens, token)
		}
	}
	return tokens, nil
}

func findLengthBoundary(items []huffman.CanonicalHuffmanCode, threshold, limit int) ([]int, error) {
	var length []int
	for i, info := range items {
		if info.Length > limit {
			return nil, errors.New("length is too long for the huffman code")
		}
		if i > threshold && info.Length == 0 {
			continue
		}
		length = append(length, info.Length)
	}
	fmt.Printf("[ flate.findLengthBoundary ] len(items): %v, len(length): %v\n", len(items), len(length))
	return length, nil
}

func (bb *bitBuffer) writeCompressedContent(value uint32, nbits uint) error {
	if nbits == 0 {
		return nil
	}
	trimbits := min(nbits, 32-bb.bitsCount)
	bb.bitsHolder |= (value & ((1 << trimbits) - 1)) << uint32(bb.bitsCount)
	bb.bitsCount += trimbits
	for bb.bitsCount >= 8 {
		lowestByte := byte(bb.bitsHolder & 0xFF)
		if _, err := bb.output.Write([]byte{lowestByte}); err != nil {
			return err
		}
		fmt.Printf("[ flate.writeCompressedContent ] Emitted lowestByte: %v\n", lowestByte)
		bb.bitsHolder >>= 8
		bb.bitsCount -= 8
	}
	value >>= uint32(trimbits)
	return bb.writeCompressedContent(value, nbits-trimbits)
}

func (bb *bitBuffer) flushAlign() error {
	if bb.bitsCount > 8 {
		return errors.New("bits not written to the output buffer yet")
	}
	if bb.bitsCount > 0 {
		fmt.Printf("[ flate.bitBuffer.flushAlign ] pad with %v bits\n", 8-bb.bitsCount)
		return bb.writeCompressedContent(0, 8-bb.bitsCount)
	}
	fmt.Printf("[ flate.bitBuffer.flushAlign ] no padding needed\n")
	return nil
}
