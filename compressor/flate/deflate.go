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
	canonicalCode []huffman.CanonicalHuffmanCode
}

type LengthAlphabets struct {
	alphabets map[int]struct {
		extraBits  int
		baseLength int
	}
	canonicalCode []huffman.CanonicalHuffmanCode
}

type AlphabetCode interface {
	FindCode(value int) (int, int, error)
	Encode(items interface{}) (map[int]huffman.CanonicalHuffmanCode, error)
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
		257: {extraBits: 0, baseLength: 3}, 258: {extraBits: 0, baseLength: 4}, 259: {extraBits: 0, baseLength: 5}, 260: {extraBits: 0, baseLength: 6}, 261: {extraBits: 0, baseLength: 7}, 262: {extraBits: 0, baseLength: 8}, 263: {extraBits: 0, baseLength: 9}, 264: {extraBits: 0, baseLength: 10}, 265: {extraBits: 1, baseLength: 11}, 266: {extraBits: 1, baseLength: 13}, 267: {extraBits: 1, baseLength: 15}, 268: {extraBits: 1, baseLength: 17}, 269: {extraBits: 2, baseLength: 19}, 270: {extraBits: 2, baseLength: 23}, 271: {extraBits: 2, baseLength: 27}, 272: {extraBits: 3, baseLength: 31}, 273: {extraBits: 3, baseLength: 35}, 274: {extraBits: 3, baseLength: 43}, 275: {extraBits: 3, baseLength: 51}, 276: {extraBits: 3, baseLength: 59}, 277: {extraBits: 4}, 278: {extraBits: 4, baseLength: 83}, 279: {extraBits: 4, baseLength: 99}, 280: {extraBits: 4, baseLength: 115}, 281: {extraBits: 5, baseLength: 131}, 282: {extraBits: 5, baseLength: 163}, 283: {extraBits: 5, baseLength: 195}, 284: {extraBits: 5, baseLength: 227}, 285: {extraBits: 0, baseLength: 258}},
}

var distAlphabets = DistanceAlphabets{
	alphabets: []struct {
		extraBits    int
		baseDistance int
	}{
		{extraBits: 0, baseDistance: 1}, {extraBits: 0, baseDistance: 2}, {extraBits: 0, baseDistance: 3}, {extraBits: 0, baseDistance: 4}, {extraBits: 1, baseDistance: 5}, {extraBits: 1, baseDistance: 7}, {extraBits: 2, baseDistance: 9}, {extraBits: 2, baseDistance: 13}, {extraBits: 3, baseDistance: 17}, {extraBits: 3, baseDistance: 25}, {extraBits: 4, baseDistance: 33}, {extraBits: 4, baseDistance: 49}, {extraBits: 5, baseDistance: 65}, {extraBits: 5, baseDistance: 97}, {extraBits: 6, baseDistance: 129}, {extraBits: 6, baseDistance: 193}, {extraBits: 7, baseDistance: 257}, {extraBits: 7, baseDistance: 385}, {extraBits: 8, baseDistance: 513}, {extraBits: 8, baseDistance: 769}, {extraBits: 9, baseDistance: 1025}, {extraBits: 9, baseDistance: 1537}, {extraBits: 10, baseDistance: 2049}, {extraBits: 10, baseDistance: 3073}, {extraBits: 11, baseDistance: 4097}, {extraBits: 11, baseDistance: 6145}, {extraBits: 12, baseDistance: 8193}, {extraBits: 12, baseDistance: 12289}, {extraBits: 13, baseDistance: 16385}, {extraBits: 13, baseDistance: 24577}},
}

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
		return 0, errors.New("reading from the compression stream for the previous block has not completed yet!")
	}
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
	newCompressionCore.isInputBufferClosed = false
	newCompressionCore.btype = btype
	newCompressionCore.bfinal = bfinal
	newCompressionReader, newCompressionWriter := new(CompressionReader), new(CompressionWriter)
	newCompressionReader.core, newCompressionWriter.core = newCompressionCore, newCompressionCore
	return newCompressionReader, newCompressionWriter
}

func (da *DistanceAlphabets) FindCode(value int) (code int, offset int, err error) {
	if value < 1 || value > maxAllowedBackwardDistance {
		return 0, 0, errors.New("value is out of range to have a match with RFC distance code")
	}
	for i, info := range da.alphabets {
		if value >= info.baseDistance && (i+1 == len(da.alphabets) || value < da.alphabets[i+1].baseDistance) {
			offset := value - info.baseDistance
			return i, offset, nil
		}
	}
	return 0, 0, fmt.Errorf("no distance code found for the distance value %v\n", value)
}

func (da *DistanceAlphabets) Encode(items interface{}) ([]int, error) {
	tokens, ok := items.([]Token)
	if !ok {
		return nil, errors.New("distance huffman tree cannot be generated without the type of Token slice")
	}
	symbolFreq := make([]int, 30)
	for _, token := range tokens {
		if token.Kind == MatchToken {
			if code, offset, err := da.FindCode(token.Distance); err != nil {
				return nil, err
			} else {
				token.DistanceCode, token.DistanceOffset = code, offset
				symbolFreq[token.DistanceCode]++
			}
		}
	}
	if distHuffmanCode, err := huffman.BuildCanonicalHuffmanTree(symbolFreq, 15); err != nil {
		return nil, err
	} else {
		da.canonicalCode = distHuffmanCode
		return findLengthBoundary(distHuffmanCode, 0), nil
	}
}

func (la *LengthAlphabets) Encode(items interface{}) ([]int, error) {
	tokens, ok := items.([]Token)
	if !ok {
		return nil, errors.New("length huffman code cannot be generated without the type of Token slice")
	}
	symbolFreq := make([]int, 286)
	for _, token := range tokens {
		if token.Kind == LiteralToken {
			symbolFreq[token.Value]++
		} else {
			if code, offset, err := la.FindCode(token.Length); err != nil {
				return nil, err
			} else {
				token.LengthCode, token.LengthOffset = code, offset
				symbolFreq[token.LengthCode]++
			}
		}
	}
	symbolFreq[256]++
	if litLenHuffmanCode, err := huffman.BuildCanonicalHuffmanTree(symbolFreq, 15); err != nil {
		return nil, err
	} else {
		la.canonicalCode = litLenHuffmanCode
		return findLengthBoundary(litLenHuffmanCode, 256), nil
	}
}

func (la *LengthAlphabets) FindCode(value int) (code int, offset int, err error) {
	if value < 3 || value > maxAllowedMatchLength {
		return 0, 0, errors.New("value is out of range to have a match with RFC length code")
	}
	for key, info := range la.alphabets {
		if nextInfo, ok := la.alphabets[key+1]; value >= info.baseLength && (!ok || value < nextInfo.baseLength) {
			offset := value - info.baseLength
			return key, offset, nil
		}
	}
	return 0, 0, fmt.Errorf("no length code found for the length value %v\n", value)
}

func (cw *CompressionWriter) compress(content []byte) error {
	contentRune := []rune(string(content))
	refChannels := make([]chan lzss.Reference, len(contentRune))
	lzss.FindMatch(refChannels, contentRune, maxAllowedBackwardDistance, maxAllowedMatchLength)
	tokens, err := tokeniseLZSS(refChannels)
	if err != nil {
		return err
	}
	litLenHuffmanCodeLengths, err := lenAlphabets.Encode(tokens)
	if err != nil {
		return err
	}
	distHuffmanCodeLengths, err := distAlphabets.Encode(tokens)
	if err != nil {
		return err
	}
	cw.core.lock.Lock()
	defer cw.core.lock.Unlock()
	writeCompressedContent(cw.Write, cw.core.bfinal, 1)
	writeCompressedContent(cw.Write, cw.core.btype, 2)
	HLIT := len(litLenHuffmanCodeLengths) - 257
	HDIST := len(distHuffmanCodeLengths) - 1
	writeCompressedContent(cw.core.outputBuffer.Write, uint32(HLIT), 5)
	writeCompressedContent(cw.Write, uint32(HDIST), 5)
	concatenatedHuffmanCodeLengths := append(litLenHuffmanCodeLengths, distHuffmanCodeLengths...)

}

func tokeniseLZSS(refChannels []chan lzss.Reference) ([]Token, error) {
	var tokens []Token
	nextRunesToIgnore := 0
	for _, channel := range refChannels {
		ref := <-channel
		if nextRunesToIgnore > 0 {
			nextRunesToIgnore--
		} else if !ref.IsRef || ref.Size < 3 {
			literalBytes := []byte(string(ref.Value[0]))
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
				return nil, errors.New(fmt.Sprintf("token match cannot be longer than %v\n", maxAllowedMatchLength))
			}
			if ref.NegativeOffset > maxAllowedBackwardDistance {
				return nil, errors.New(fmt.Sprintf("token match cannot be farther backward than %v\n", maxAllowedBackwardDistance))
			}
			nextRunesToIgnore = ref.Size - 1
			token := Token{
				Kind:     MatchToken,
				Length:   ref.Size,
				Distance: ref.NegativeOffset,
			}
			tokens = append(tokens, token)
		}
	}
	return tokens, nil
}

func findLengthBoundary(items []huffman.CanonicalHuffmanCode, threshold int) []int {
	var length []int
	for i, info := range items {
		if i > threshold && info.Length == 0 {
			continue
		}
		length = append(length, info.Length)
	}
	return length
}

func writeCompressedContent(outputBufferWriter func([]byte) (int, error), value uint32, nbits uint) error {

}
