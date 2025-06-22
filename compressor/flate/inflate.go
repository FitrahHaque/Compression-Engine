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
	if input, err := dw.readCompressedContent(1); err != nil {
		return err
	} else {
		dw.core.bfinal = input
	}

	// btype
	if input, err := dw.readCompressedContent(2); err != nil {
		return err
	} else {
		dw.core.btype = input
	}

	var HLIT, HDIST, HCLEN uint32

	// HLIT
	if input, err := dw.readCompressedContent(5); err != nil {
		return err
	} else {
		HLIT = input
	}
	// HDIST
	if input, err := dw.readCompressedContent(5); err != nil {
		return err
	} else {
		HDIST = input
	}

	// HCLEN
	if input, err := dw.readCompressedContent(4); err != nil {
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
		if input, err := dw.readCompressedContent(3); err != nil {
			return err
		} else {
			codeLengthHuffmanLengths = append(codeLengthHuffmanLengths, input)
		}
	}
	newCodeLengthCode := new(CodeLengthCode)
	newCodeLengthCode.Decode(codeLengthHuffmanLengths)

}

// func (dw *DecompressionWriter) readBits(nbits int) (uint32, error) {
// 	if nbits > 32 {
// 		return 0, errors.New("cannot read more than 32 bits at once.")
// 	}
// 	return dw.core.outputBuffer.readCompressedContent(nbits)

// }

func (dw *DecompressionWriter) readCompressedContent(nbits uint) (uint32, error) {
	bb := dw.core.bitBuffer
	if nbits > 32 {
		return 0, errors.New("cannot read more than 32 bits at once.")
	}
	for bb.bitsCount < nbits {
		newData := make([]byte, 1)
		if _, err := dw.core.inputBuffer.Read(newData); err != nil {
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

func (clc *CodeLengthCode) Decode(huffmanLengths []uint32) error {
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
		key := rleAlphabets.keyOrder[i]
		lengths[key] = length
	}
	return lengths
}
