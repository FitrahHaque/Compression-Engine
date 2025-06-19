package flate

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/FitrahHaque/Compression-Engine/compressor/lzss"
)

type TokenKind int

const (
	LiteralToken TokenKind = iota
	MatchToken
)

type distanceAlphabet struct {
	extraBits    int
	baseDistance int
}

type litLenAlphabet struct {
	extraBits  int
	baseLength int
}

type Token struct {
	Kind     TokenKind
	Value    byte
	Length   int
	Distance int
}

var maxAllowedBackwardDistance int = 32768
var maxAllowedMatchLength int = 258
var litLenAlphabets = map[int]litLenAlphabet{
	257: {extraBits: 0, baseLength: 3}, 258: {extraBits: 0, baseLength: 4}, 259: {extraBits: 0, baseLength: 5}, 260: {extraBits: 0, baseLength: 6}, 261: {extraBits: 0, baseLength: 7}, 262: {extraBits: 0, baseLength: 8}, 263: {extraBits: 0, baseLength: 9}, 264: {extraBits: 0, baseLength: 10}, 265: {extraBits: 1, baseLength: 11}, 266: {extraBits: 1, baseLength: 13}, 267: {extraBits: 1, baseLength: 15}, 268: {extraBits: 1, baseLength: 17}, 269: {extraBits: 2, baseLength: 19}, 270: {extraBits: 2, baseLength: 23}, 271: {extraBits: 2, baseLength: 27}, 272: {extraBits: 3, baseLength: 31}, 273: {extraBits: 3, baseLength: 35}, 274: {extraBits: 3, baseLength: 43}, 275: {extraBits: 3, baseLength: 51}, 276: {extraBits: 3, baseLength: 59}, 277: {extraBits: 4}, 278: {extraBits: 4, baseLength: 83}, 279: {extraBits: 4, baseLength: 99}, 280: {extraBits: 4, baseLength: 115}, 281: {extraBits: 5, baseLength: 131}, 282: {extraBits: 5, baseLength: 163}, 283: {extraBits: 5, baseLength: 195}, 284: {extraBits: 5, baseLength: 227}, 285: {extraBits: 0, baseLength: 258},
}

var distAlphabets = []distanceAlphabet{
	{extraBits: 0, baseDistance: 1}, {extraBits: 0, baseDistance: 2}, {extraBits: 0, baseDistance: 3}, {extraBits: 0, baseDistance: 4}, {extraBits: 1, baseDistance: 5}, {extraBits: 1, baseDistance: 7}, {extraBits: 2, baseDistance: 9}, {extraBits: 2, baseDistance: 13}, {extraBits: 3, baseDistance: 17}, {extraBits: 3, baseDistance: 25}, {extraBits: 4, baseDistance: 33}, {extraBits: 4, baseDistance: 49}, {extraBits: 5, baseDistance: 65}, {extraBits: 5, baseDistance: 97}, {extraBits: 6, baseDistance: 129}, {extraBits: 6, baseDistance: 193}, {extraBits: 7, baseDistance: 257}, {extraBits: 7, baseDistance: 385}, {extraBits: 8, baseDistance: 513}, {extraBits: 8, baseDistance: 769}, {extraBits: 9, baseDistance: 1025}, {extraBits: 9, baseDistance: 1537}, {extraBits: 10, baseDistance: 2049}, {extraBits: 10, baseDistance: 3073}, {extraBits: 11, baseDistance: 4097}, {extraBits: 11, baseDistance: 6145}, {extraBits: 12, baseDistance: 8193}, {extraBits: 12, baseDistance: 12289}, {extraBits: 13, baseDistance: 16385}, {extraBits: 13, baseDistance: 24577},
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
	btype               int
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
	compressedData, err := compress(originalData)
	if err != nil {
		return err
	}
	if _, err = cw.core.outputBuffer.Write(compressedData); err != nil {
		return err
	}
	return nil
}

func NewCompressionReaderAndWriter(btype int) (io.ReadCloser, io.WriteCloser) {
	newCompressionCore := new(compressionCore)
	newCompressionCore.inputBuffer, newCompressionCore.outputBuffer = new(bytes.Buffer), new(bytes.Buffer)
	newCompressionCore.isInputBufferClosed = false
	newCompressionCore.btype = btype
	newCompressionReader, newCompressionWriter := new(CompressionReader), new(CompressionWriter)
	newCompressionReader.core, newCompressionWriter.core = newCompressionCore, newCompressionCore
	return newCompressionReader, newCompressionWriter
}

func compress(content []byte) ([]byte, error) {
	contentRune := []rune(string(content))
	refChannels := make([]chan lzss.Reference, len(contentRune))
	lzss.FindMatch(refChannels, contentRune, maxAllowedBackwardDistance, maxAllowedMatchLength)
	tokens, err := tokeniseLZSS(refChannels)
	if err != nil {
		return nil, err
	}
	return nil, nil
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
