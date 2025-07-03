package gzip

import (
	"encoding/binary"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"sync"
)

type CompressionCore struct {
	lock        sync.Mutex
	Writer      *io.PipeWriter
	Reader      *io.PipeReader
	FlateWriter io.WriteCloser
	FlateReader io.ReadCloser
	Crc         hash.Hash32
	Size        uint32
}

type CompressionReader struct {
	core *CompressionCore
}

type CompressionWriter struct {
	core *CompressionCore
}

func NewCompressionReaderAndWriter(flateReader io.ReadCloser, flateWriter io.WriteCloser) (io.ReadCloser, io.WriteCloser) {
	newCompressionCore := new(CompressionCore)
	fmt.Printf("[ gzip.NewCompressionReaderAndWriter ] 1\n")
	newCompressionCore.Reader, newCompressionCore.Writer = io.Pipe()
	fmt.Printf("[ gzip.NewCompressionReaderAndWriter ] 2\n")
	newCompressionCore.FlateReader, newCompressionCore.FlateWriter = flateReader, flateWriter
	newCompressionCore.Crc = crc32.NewIEEE()
	newCompressionReader, newCompressionWriter := new(CompressionReader), new(CompressionWriter)
	newCompressionReader.core, newCompressionWriter.core = newCompressionCore, newCompressionCore
	header := [10]byte{
		0x1f, 0x8b, // ID1, ID2
		0x08,       // CM = deflate
		0x00,       // FLG
		0, 0, 0, 0, // MTIME
		0x00, // XFL
		0xff, // OS = unknown
	}
	fmt.Printf("[ gzip.NewCompressionReaderAndWriter ] 3\n")
	go newCompressionCore.Writer.Write(header[:])
	fmt.Printf("[ gzip.NewCompressionReaderAndWriter ] 4\n")
	return newCompressionReader, newCompressionWriter
}

func (cw *CompressionWriter) Write(p []byte) (int, error) {
	fmt.Printf("[ gzip.CompressionWriter.Write ] 1\n")
	cw.core.lock.Lock()
	defer cw.core.lock.Unlock()
	fmt.Printf("[ gzip.CompressionWriter.Write ] 2\n")
	cw.core.Crc.Write(p)
	cw.core.Size += uint32(len(p))
	return cw.core.FlateWriter.Write(p)
}

func (cw *CompressionWriter) Close() error {
	// cw.core.lock.Lock()
	// defer cw.core.lock.Unlock()
	fmt.Printf("[ gzip.CompressionWriter.Close ] 1\n")
	go func() {
		if err := cw.core.FlateWriter.Close(); err != nil {
			panic(err)
		}
		fmt.Printf("[ gzip.CompressionWriter.Close ] 2\n")
	}()
	fmt.Printf("[ gzip.CompressionWriter.Close ] 3\n")
	if _, err := io.Copy(cw.core.Writer, cw.core.FlateReader); err != nil {
		return err
	}
	fmt.Printf("[ gzip.CompressionWriter.Close ] 4\n")
	if err := cw.core.FlateReader.Close(); err != nil {
		return err
	}
	fmt.Printf("[ gzip.CompressionWriter.Close ] 5\n")
	var tail [8]byte
	binary.LittleEndian.PutUint32(tail[0:4], cw.core.Crc.Sum32())
	binary.LittleEndian.PutUint32(tail[4:8], cw.core.Size)
	cw.core.Writer.Write(tail[:])
	fmt.Printf("[ gzip.CompressionWriter.Close ] 6\n")
	return cw.core.Writer.Close()
}

func (cr *CompressionReader) Read(p []byte) (int, error) {
	fmt.Printf("[ gzip.CompressionReader.Read ] 1\n")
	// cr.core.lock.Lock()
	// defer cr.core.lock.Unlock()

	fmt.Printf("[ gzip.CompressionReader.Read ] 2\n")
	if n, err := cr.core.Reader.Read(p); err != nil {
		fmt.Printf("[ gzip.CompressionReader.Read ] error: %v\n", err)
		return 0, err
	} else {
		fmt.Printf("[ gzip.CompressionReader.Read ] bytes read: %v\n", n)
		return n, err
	}
}

func (cr *CompressionReader) Close() error {
	// cr.core.lock.Lock()
	// defer cr.core.lock.Unlock()
	fmt.Printf("[ gzip.CompressionReader.Close ] 1\n")
	return cr.core.Reader.Close()
}
