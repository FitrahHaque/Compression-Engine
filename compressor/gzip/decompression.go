package gzip

import (
	"encoding/binary"
	"errors"
	"hash"
	"hash/crc32"
	"io"
	"sync"
)

type DecompressionCore struct {
	lock           sync.Mutex
	Writer         *io.PipeWriter
	Reader         *io.PipeReader
	IsHeaderParsed bool
	Trailer        []byte
	CurrentCrc     hash.Hash32
	CurrentSize    uint32
	FlateWriter    io.WriteCloser
	FlateReader    io.ReadCloser
}

type DecompressionWriter struct {
	core *DecompressionCore
}

type DecompressionReader struct {
	core *DecompressionCore
}

func NewDecompressionReaderAndWriter(flateReader io.ReadCloser, flateWriter io.WriteCloser) (io.ReadCloser, io.WriteCloser) {
	newDecompressionCore := new(DecompressionCore)
	newDecompressionCore.Reader, newDecompressionCore.Writer = io.Pipe()
	newDecompressionCore.FlateReader, newDecompressionCore.FlateWriter = flateReader, flateWriter
	newDecompressionCore.CurrentCrc = crc32.NewIEEE()
	newDecompressionCore.Trailer = make([]byte, 8)
	newDecompressionReader, newDecompressionWriter := new(DecompressionReader), new(DecompressionWriter)
	newDecompressionReader.core, newDecompressionWriter.core = newDecompressionCore, newDecompressionCore
	return newDecompressionReader, newDecompressionWriter
}

func (dw *DecompressionWriter) Write(p []byte) (int, error) {
	dw.core.lock.Lock()
	// defer dw.core.lock.Unlock()
	if !dw.core.IsHeaderParsed {
		// header := p[:10]
		dw.core.IsHeaderParsed = true
		p = p[10:]
	}
	dw.core.lock.Unlock()
	// fmt.Printf("[ gzip.DecompressionWriter.Write ] 1\n")
	if len(dw.core.Trailer)+len(p) < 8 {
		// fmt.Printf("[ gzip.DecompressionWriter.Write ] 2\n")
		dw.core.Trailer = append(dw.core.Trailer, p...)
	} else if len(p) < 8 {
		// fmt.Printf("[ gzip.DecompressionWriter.Write ] 3\n")
		dw.core.Trailer = append(dw.core.Trailer[len(p):], p...)
	} else {
		copy(dw.core.Trailer, p[len(p)-8:])
		// fmt.Printf("[ gzip.DecompressionWriter.Write ] len(Trailer): %v\n", len(dw.core.Trailer))
	}
	return dw.core.FlateWriter.Write(p)
}

func (dw *DecompressionWriter) Close() error {
	dw.core.lock.Lock()
	defer dw.core.lock.Unlock()

	go func() {
		if err := dw.core.FlateWriter.Close(); err != nil {
			panic(err)
		}
	}()

	if _, err := io.Copy(dw.core.Writer, dw.core.FlateReader); err != nil {
		return err
	}
	if err := dw.core.FlateReader.Close(); err != nil {
		return err
	}
	return dw.core.Writer.Close()
}

func (dr *DecompressionReader) Read(p []byte) (int, error) {
	// dr.core.lock.Lock()
	// defer dr.core.lock.Unlock()

	if n, err := dr.core.Reader.Read(p); err != nil {
		return 0, err
	} else {
		dr.core.CurrentSize += uint32(n)
		// if f, err := os.OpenFile("decom.o", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err != nil {
		// 	panic(err)
		// } else {
		// 	f.Write(p)
		// }
		dr.core.CurrentCrc.Write(p[:n])
		return n, nil
	}
}

func (dr *DecompressionReader) Close() error {
	dr.core.lock.Lock()
	defer dr.core.lock.Unlock()

	if len(dr.core.Trailer) != 8 {
		return errors.New("trailer data is not sufficient")
	}
	givenCrc := binary.LittleEndian.Uint32(dr.core.Trailer[0:4])
	givenSize := binary.LittleEndian.Uint32(dr.core.Trailer[4:])
	// fmt.Printf("[ gzip.DecompressionReader.Close ] givenCrc: %v, given Size: %v\n", givenCrc, givenSize)
	// fmt.Printf("[ gzip.DecompressionReader.Close ] currentCrc: %v, currentSize: %v\n", dr.core.CurrentCrc.Sum32(), dr.core.CurrentSize)
	if givenSize != dr.core.CurrentSize {
		return errors.New("size did not match")
	}
	if givenCrc != dr.core.CurrentCrc.Sum32() {
		return errors.New("crc did not match")
	}
	return dr.core.Reader.Close()
}
