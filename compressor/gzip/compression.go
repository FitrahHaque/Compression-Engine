package gzip

import (
	"encoding/binary"
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
	newCompressionCore.Reader, newCompressionCore.Writer = io.Pipe()
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
	newCompressionCore.Writer.Write(header[:])
	return newCompressionReader.core.Reader, newCompressionWriter.core.Writer
}

func (cw *CompressionWriter) Write(p []byte) (int, error) {
	cw.core.lock.Lock()
	defer cw.core.lock.Unlock()
	cw.core.Crc.Write(p)
	cw.core.Size += uint32(len(p))
	return cw.core.FlateWriter.Write(p)
}

func (cw *CompressionWriter) Close() error {
	cw.core.lock.Lock()
	defer cw.core.lock.Unlock()
	go func() {
		if err := cw.core.FlateWriter.Close(); err != nil {
			panic(err)
		}
	}()
	if _, err := io.Copy(cw.core.Writer, cw.core.FlateReader); err != nil {
		return err
	}
	var tail [8]byte
	binary.LittleEndian.PutUint32(tail[0:4], cw.core.Crc.Sum32())
	binary.LittleEndian.PutUint32(tail[4:8], cw.core.Size)
	cw.core.Writer.Write(tail[:])
	cw.core.Writer.Close()
	return nil
}

func (cr *CompressionReader) Read(p []byte) (int, error) {
	cr.core.lock.Lock()
	defer cr.core.lock.Unlock()

	return cr.core.Reader.Read(p)
}

func (cr *CompressionReader) Close() error {
	cr.core.lock.Lock()
	defer cr.core.lock.Unlock()
	return cr.core.Reader.Close()
}
