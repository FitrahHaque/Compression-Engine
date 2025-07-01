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
	EndOfBlockToken
)

type Rulebook struct {
	Alphabets map[int]struct {
		ExtraBits int
		Base      int
	}
	KeyOrder []int
}

type LitLengthCode struct {
	LitLengthHuffman []huffman.CanonicalHuffman
	CanonicalRoot    *huffman.CanonicalHuffmanNode
}
type DistanceCode struct {
	DistanceHuffman []huffman.CanonicalHuffman
	CanonicalRoot   *huffman.CanonicalHuffmanNode
}
type CodeLengthCode struct {
	HuffmanLengthCondensed []struct {
		RLECode int
		Offset  int
	}
	CondensedHuffman []huffman.CanonicalHuffman
	CanonicalRoot    *huffman.CanonicalHuffmanNode
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
var lenAlphabets = Rulebook{
	Alphabets: map[int]struct {
		ExtraBits int
		Base      int
	}{
		257: {ExtraBits: 0, Base: 3}, 258: {ExtraBits: 0, Base: 4}, 259: {ExtraBits: 0, Base: 5}, 260: {ExtraBits: 0, Base: 6}, 261: {ExtraBits: 0, Base: 7}, 262: {ExtraBits: 0, Base: 8}, 263: {ExtraBits: 0, Base: 9}, 264: {ExtraBits: 0, Base: 10}, 265: {ExtraBits: 1, Base: 11}, 266: {ExtraBits: 1, Base: 13}, 267: {ExtraBits: 1, Base: 15}, 268: {ExtraBits: 1, Base: 17}, 269: {ExtraBits: 2, Base: 19}, 270: {ExtraBits: 2, Base: 23}, 271: {ExtraBits: 2, Base: 27}, 272: {ExtraBits: 3, Base: 31}, 273: {ExtraBits: 3, Base: 35}, 274: {ExtraBits: 3, Base: 43}, 275: {ExtraBits: 3, Base: 51}, 276: {ExtraBits: 3, Base: 59}, 277: {ExtraBits: 4, Base: 67}, 278: {ExtraBits: 4, Base: 83}, 279: {ExtraBits: 4, Base: 99}, 280: {ExtraBits: 4, Base: 115}, 281: {ExtraBits: 5, Base: 131}, 282: {ExtraBits: 5, Base: 163}, 283: {ExtraBits: 5, Base: 195}, 284: {ExtraBits: 5, Base: 227}, 285: {ExtraBits: 0, Base: 258},
	},
	KeyOrder: []int{
		257, 258, 259, 260, 261, 262, 263, 264, 265, 266, 267, 268, 269, 270, 271, 272, 273, 274, 275, 276, 277, 278, 279, 280, 281, 282, 283, 284, 285,
	},
}

var distAlphabets = Rulebook{
	Alphabets: map[int]struct {
		ExtraBits int
		Base      int
	}{
		0: {ExtraBits: 0, Base: 1}, 1: {ExtraBits: 0, Base: 2}, 2: {ExtraBits: 0, Base: 3}, 3: {ExtraBits: 0, Base: 4}, 4: {ExtraBits: 1, Base: 5}, 5: {ExtraBits: 1, Base: 7}, 6: {ExtraBits: 2, Base: 9}, 7: {ExtraBits: 2, Base: 13}, 8: {ExtraBits: 3, Base: 17}, 9: {ExtraBits: 3, Base: 25}, 10: {ExtraBits: 4, Base: 33}, 11: {ExtraBits: 4, Base: 49}, 12: {ExtraBits: 5, Base: 65}, 13: {ExtraBits: 5, Base: 97}, 14: {ExtraBits: 6, Base: 129}, 15: {ExtraBits: 6, Base: 193}, 16: {ExtraBits: 7, Base: 257}, 17: {ExtraBits: 7, Base: 385}, 18: {ExtraBits: 8, Base: 513}, 19: {ExtraBits: 8, Base: 769}, 20: {ExtraBits: 9, Base: 1025}, 21: {ExtraBits: 9, Base: 1537}, 22: {ExtraBits: 10, Base: 2049}, 23: {ExtraBits: 10, Base: 3073}, 24: {ExtraBits: 11, Base: 4097}, 25: {ExtraBits: 11, Base: 6145}, 26: {ExtraBits: 12, Base: 8193}, 27: {ExtraBits: 12, Base: 12289}, 28: {ExtraBits: 13, Base: 16385}, 29: {ExtraBits: 13, Base: 24577},
	},
	KeyOrder: []int{
		0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29,
	},
}

var rleAlphabets = Rulebook{
	Alphabets: map[int]struct {
		ExtraBits int
		Base      int
	}{
		16: {ExtraBits: 2, Base: 3}, 17: {ExtraBits: 3, Base: 3}, 18: {ExtraBits: 7, Base: 11}, 0: {ExtraBits: 0, Base: 0}, 8: {ExtraBits: 0, Base: 8}, 7: {ExtraBits: 0, Base: 7}, 9: {ExtraBits: 0, Base: 9}, 6: {ExtraBits: 0, Base: 6}, 10: {ExtraBits: 0, Base: 10}, 5: {ExtraBits: 0, Base: 5}, 11: {ExtraBits: 0, Base: 11}, 4: {ExtraBits: 0, Base: 4}, 12: {ExtraBits: 0, Base: 12}, 3: {ExtraBits: 0, Base: 3}, 13: {ExtraBits: 0, Base: 13}, 2: {ExtraBits: 0, Base: 2}, 14: {ExtraBits: 0, Base: 14}, 1: {ExtraBits: 0, Base: 1}, 15: {ExtraBits: 0, Base: 15},
	},
	KeyOrder: []int{
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
	bitsHolder uint32
	bitsCount  uint
}

type compressionCore struct {
	isInputBufferClosed bool
	lock                sync.Mutex
	inputBuffer         io.ReadWriter
	outputBuffer        io.ReadWriter
	bitBuffer           *bitBuffer
	btype               uint32
	bfinal              uint32
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
		return 0, errors.New("reading from the original stream for the previous block has not completed yet!")
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
	newCompressionCore.inputBuffer, newCompressionCore.outputBuffer = new(bytes.Buffer), new(bytes.Buffer)
	newCompressionCore.bitBuffer = new(bitBuffer)
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
	for i, info := range distAlphabets.Alphabets {
		if value >= info.Base && (i+1 == len(distAlphabets.Alphabets) || value < distAlphabets.Alphabets[i+1].Base) {
			offset := value - info.Base
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
	if distHuffmanCode, err := huffman.BuildCanonicalHuffmanEncoder(symbolFreq, 15); err != nil {
		return nil, err
	} else {
		for i, huffman := range distHuffmanCode {
			if huffman != nil {
				fmt.Printf("[ flate.DistanceCode.Encode ] DistanceCode: %v --- HuffmanCode: %v, HuffmanCodeLength: %v\n", i, huffman.GetValue(), huffman.GetLength())
			}
		}
		dc.DistanceHuffman = distHuffmanCode
		return findLengthBoundary(distHuffmanCode, 0, 15)
	}
}

func (llc *LitLengthCode) FindCode(value int) (code int, offset int, err error) {
	if value < 3 || value > maxAllowedMatchLength {
		return 0, 0, errors.New("value is out of range to have a match with RFC length code")
	}
	for _, key := range lenAlphabets.KeyOrder {
		if nextInfo, ok := lenAlphabets.Alphabets[key+1]; value >= lenAlphabets.Alphabets[key].Base && (!ok || value < nextInfo.Base) {
			// fmt.Printf("[ flate.FindCode ] ")
			offset := value - lenAlphabets.Alphabets[key].Base
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
	if litLenHuffmanCode, err := huffman.BuildCanonicalHuffmanEncoder(symbolFreq, 15); err != nil {
		return nil, err
	} else {
		fmt.Printf("[ flate.LitLengthCode.Encode ] len(litLenHuffmanCode): %v\n", len(litLenHuffmanCode))
		for i, huff := range litLenHuffmanCode {
			if huff != nil {
				fmt.Printf("[ flate.LitLengthCode.Encode ] LengthCode: %v --- HuffmanCode: %v, HuffmanCodeLength: %v\n", i, huff.GetValue(), huff.GetLength())
			}
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
	if codeLengthHuffmanCode, err := huffman.BuildCanonicalHuffmanEncoder(symbolFreq, 7); err != nil {
		return nil, err
	} else {
		clc.CondensedHuffman = make([]huffman.CanonicalHuffman, len(codeLengthHuffmanCode))
		copy(clc.CondensedHuffman, codeLengthHuffmanCode)
		for i, huffman := range clc.CondensedHuffman {
			if huffman != nil {
				fmt.Printf("[ flate.CodeLengthCode.Encode ] RLECode: %v --- HuffmanCode: %v, HuffmanCodeLength: %v\n", i, huffman.GetValue(), huffman.GetLength())
			}
		}
		codeLengthHuffmanCode = clc.shuffle(codeLengthHuffmanCode)
		for i, huffman := range codeLengthHuffmanCode {
			if huffman != nil {
				key := rleAlphabets.KeyOrder[i]
				fmt.Printf("[ flate.CodeLengthCode.Encode ] RLECode: %v --- HuffmanCode: %v, HuffmanCodeLength: %v\n", key, huffman.GetValue(), huffman.GetLength())
			}
		}
		return findLengthBoundary(codeLengthHuffmanCode, 3, 7)
	}
}

func (clc *CodeLengthCode) shuffle(code []huffman.CanonicalHuffman) []huffman.CanonicalHuffman {
	var huffmanLengths []huffman.CanonicalHuffman
	for _, key := range rleAlphabets.KeyOrder {
		huffmanLengths = append(huffmanLengths, code[key])
	}
	return huffmanLengths
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
	fmt.Printf("[ flate.CompressionWriter.compress ] len(litLenHuffmanLengths): %v\n", len(litLenHuffmanLengths))
	if err != nil {
		return err
	}
	newDistanceCode := new(DistanceCode)
	distHuffmanLengths, err := newDistanceCode.Encode(tokens)
	fmt.Printf("[ flate.CompressionWriter.compress ] len(distHuffmanLengths): %v\n", len(distHuffmanLengths))
	if err != nil {
		return err
	}
	concatenatedHuffmanLengths := append(litLenHuffmanLengths, distHuffmanLengths...)
	fmt.Printf("[ flate.CompressionWriter.compress ] len(concatenatedHuffmanLengths): %v\n", len(concatenatedHuffmanLengths))
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
	fmt.Printf("[ flate.CompressionWriter.compress ] bfinal: %v, bits: %v\n", cw.core.bfinal, 1)
	cw.writeCompressedContent(cw.core.bfinal, 1)
	fmt.Printf("[ flate.CompressionWriter.compress ] btype: %v, bits: %v\n", cw.core.btype, 2)
	cw.writeCompressedContent(cw.core.btype, 2)
	fmt.Printf("[ flate.CompressionWriter.compress ] HLIT: %v, bits: %v\n", uint32(HLIT), 5)
	cw.writeCompressedContent(uint32(HLIT), 5)
	fmt.Printf("[ flate.CompressionWriter.compress ] HDIST: %v, bits: %v\n", uint32(HDIST), 5)
	cw.writeCompressedContent(uint32(HDIST), 5)
	fmt.Printf("[ flate.CompressionWriter.compress ] HCLEN: %v, bits: %v\n", uint32(HCLEN), 4)
	cw.writeCompressedContent(uint32(HCLEN), 4)
	for _, codeLen := range codeLengthHuffmanLengths {
		fmt.Printf("[ flate.CompressionWriter.compress ] RLEHuffmanLength: %v, bits: 3\n", codeLen)
		cw.writeCompressedContent(uint32(codeLen), 3)
	}
	for _, code := range newCodeLengthCode.HuffmanLengthCondensed {
		condensedHuff := newCodeLengthCode.CondensedHuffman[code.RLECode]
		fmt.Printf("[ flate.CompressionWriter.compress ] Condensed -- RLECode: %v --- HuffmanCode: %v, HuffmanCodeLength: %v\n", code.RLECode, condensedHuff.GetValue(), condensedHuff.GetLength())
		cw.writeCompressedContent(huffman.Reverse(uint32(condensedHuff.GetValue()), uint32(condensedHuff.GetLength())), uint(condensedHuff.GetLength()))
		if rleAlphabets.Alphabets[code.RLECode].ExtraBits > 0 {
			fmt.Printf("[ flate.CompressionWriter.compress ] Condensed -- RLECode: %v, Offset: %v --- bitlength: %v\n", code.RLECode, code.Offset, rleAlphabets.Alphabets[code.RLECode].ExtraBits)
			cw.writeCompressedContent(uint32(code.Offset), uint(rleAlphabets.Alphabets[code.RLECode].ExtraBits))
		}
	}
	for _, token := range tokens {
		if token.Kind == LiteralToken {
			litLenHuff := newLitLengthCode.LitLengthHuffman[token.Value]
			fmt.Printf("[ flate.CompressionWriter.compress ] Literal: %v --- HuffmanCode: %v, HuffmanCodeLength: %v\n", string(token.Value), litLenHuff.GetValue(), litLenHuff.GetLength())
			cw.writeCompressedContent(huffman.Reverse(uint32(litLenHuff.GetValue()), uint32(litLenHuff.GetLength())), uint(litLenHuff.GetLength()))
		} else {
			litLenHuff := newLitLengthCode.LitLengthHuffman[token.LengthCode]
			fmt.Printf("[ flate.CompressionWriter.compress ] Length: %v, LengthCode: %v --- HuffmanCode: %v, HuffmanCodeLength: %v\n", token.Length, token.LengthCode, litLenHuff.GetValue(), litLenHuff.GetLength())
			cw.writeCompressedContent(huffman.Reverse(uint32(litLenHuff.GetValue()), uint32(litLenHuff.GetLength())), uint(litLenHuff.GetLength()))
			if lenAlphabets.Alphabets[token.LengthCode].ExtraBits > 0 {
				fmt.Printf("[ flate.CompressionWriter.compress ] Length: %v, LengthCode: %v, Offset: %v --- bitLength: %v\n", token.Length, litLenHuff.GetValue(), token.LengthOffset, lenAlphabets.Alphabets[token.LengthCode].ExtraBits)
				cw.writeCompressedContent(uint32(token.LengthOffset), uint(lenAlphabets.Alphabets[token.LengthCode].ExtraBits))
			}
			distHuff := newDistanceCode.DistanceHuffman[token.DistanceCode]
			fmt.Printf("[ flate.CompressionWriter.compress ] Distance: %v, DistanceCode: %v --- HuffmanCode: %v, HuffmanCodeLength: %v\n", token.Distance, token.DistanceCode, distHuff.GetValue(), distHuff.GetLength())
			cw.writeCompressedContent(huffman.Reverse(uint32(distHuff.GetValue()), uint32(distHuff.GetLength())), uint(distHuff.GetLength()))
			if distAlphabets.Alphabets[token.DistanceCode].ExtraBits > 0 {
				fmt.Printf("[ flate.CompressionWriter.compress ] Distance: %v, DistanceCode: %v, Offset: %v --- bitLength: %v\n", token.Distance, token.DistanceCode, token.DistanceOffset, distAlphabets.Alphabets[token.DistanceCode].ExtraBits)
				cw.writeCompressedContent(uint32(token.DistanceOffset), uint(distAlphabets.Alphabets[token.DistanceCode].ExtraBits))
			}
		}
	}
	eobHuff := newLitLengthCode.LitLengthHuffman[256]
	fmt.Printf("[ flate.CompressionWriter.compress ] EOB: %v --- HuffmanCode: %v, HuffmanCodeLength: %v\n", 256, eobHuff.GetValue(), eobHuff.GetLength())
	cw.writeCompressedContent(uint32(eobHuff.GetValue()), uint(eobHuff.GetLength()))
	return cw.flushAlign()
}

func (cw *CompressionWriter) writeCompressedContent(value uint32, nbits uint) error {
	bb := cw.core.bitBuffer
	if nbits == 0 {
		return nil
	}
	fmt.Printf("[ flate.CompressionWriter.writeCompressedContent ] value: %0*b, nbits: %v\n", nbits, value, nbits)
	trimbits := min(nbits, 32-bb.bitsCount)
	bb.bitsHolder |= (value & ((1 << trimbits) - 1)) << uint32(bb.bitsCount)
	bb.bitsCount += trimbits
	for bb.bitsCount >= 8 {
		lowestByte := byte(bb.bitsHolder & 0xFF)
		if _, err := cw.core.outputBuffer.Write([]byte{lowestByte}); err != nil {
			return err
		}
		fmt.Printf("[ flate.writeCompressedContent ] Emitted lowestByte: %08b\n", lowestByte)
		bb.bitsHolder >>= 8
		bb.bitsCount -= 8
	}
	value >>= uint32(trimbits)
	return cw.writeCompressedContent(value, nbits-trimbits)
}

func (cw *CompressionWriter) flushAlign() error {
	bb := cw.core.bitBuffer
	if bb.bitsCount > 8 {
		return errors.New("bits not written to the output buffer yet")
	}
	if bb.bitsCount > 0 {
		fmt.Printf("[ flate.bitBuffer.flushAlign ] pad with %v bits\n", 8-bb.bitsCount)
		return cw.writeCompressedContent(0, 8-bb.bitsCount)
	}
	fmt.Printf("[ flate.bitBuffer.flushAlign ] no padding needed\n")
	return nil
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

func findLengthBoundary(items []huffman.CanonicalHuffman, threshold, limit int) ([]int, error) {
	var length []int
	var zeros []int
	for i, info := range items {
		if i <= threshold {
			if info == nil {
				length = append(length, 0)
			} else {
				length = append(length, info.GetLength())
			}
		} else if info == nil || info.GetLength() == 0 {
			zeros = append(zeros, 0)
		} else if info.GetLength() > limit {
			return nil, errors.New("length is too long for the huffman code")
		} else {
			length = append(length, zeros...)
			zeros = []int{}
			length = append(length, info.GetLength())
		}
	}
	fmt.Printf("[ flate.findLengthBoundary ] len(items): %v, len(length): %v\n", len(items), len(length))
	return length, nil
}
