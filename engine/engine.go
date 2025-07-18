package engine

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/FitrahHaque/Compression-Engine/compressor/flate"
	"github.com/FitrahHaque/Compression-Engine/compressor/gzip"
	"github.com/FitrahHaque/Compression-Engine/compressor/huffman"
	"github.com/FitrahHaque/Compression-Engine/compressor/lzss"
)

var Engines = [...]string{
	"huffman",
	"lzss",
	"flate",
	"gzip",
}

type FlateArgs struct {
	Bfinal uint32
	Btype  uint32
}

type GzipArgs struct {
	Bfinal uint32
	Btype  uint32
}

type compression struct {
	compressionEngine string
	writer            io.WriteCloser
	reader            io.ReadCloser
}

type decompression struct {
	decompressionEngine string
	writer              io.WriteCloser
	reader              io.ReadCloser
}

var compressionReaderAndWriters = map[string]any{
	"huffman": huffman.NewCompressionReaderAndWriter,
	"lzss":    lzss.NewCompressionReaderAndWriter,
	"flate":   flate.NewCompressionReaderAndWriter,
	"gzip":    gzip.NewCompressionReaderAndWriter,
}

var decompressionReaderAndWriters = map[string]any{
	"huffman": huffman.NewDecompressionReaderAndWriter,
	"lzss":    lzss.NewDecompressionReaderAndWriter,
	"flate":   flate.NewDecompressionReaderAndWriter,
	"gzip":    gzip.NewDecompressionReaderAndWriter,
}

func CompressFiles(algorithm string, files []string, fileExtension string, args any) {
	// fmt.Printf("[ engine.CompressFiles ] args: %v\n", args)
	for _, file := range files {
		compressFile(algorithm, file, file+fileExtension, args)
	}
}

func ClientCompress(algorithm string, filePath string, args any) io.ReadCloser {
	content := compressFile(algorithm, filePath, "", args)
	// fmt.Printf("algorithm -- %v, compressed content:\n%v\n", algorithm, content)
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		io.Copy(pw, bytes.NewReader(content))
	}()
	return pr
}

func compressFile(algorithm string, filePath string, outputFileName string, args any) []byte {
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		panic(err)
	}
	fmt.Println("Compressing...")
	data := compress(algorithm, fileContent, outputFileName, args)
	if len(outputFileName) > 0 {
		fmt.Printf("File `%s` has been compressed into the file `%s`\n", filePath, outputFileName)
	}
	return data
}

func compress(algorithm string, fileContent []byte, outputFileName string, args any) []byte {
	compressor := compression{
		compressionEngine: algorithm,
	}
	compressor.init(args)
	var content []byte
	var err error
	// fmt.Printf("[ engine.compress ] 1\n")
	go func() {
		if _, err := compressor.writer.Write(fileContent); err != nil {
			panic(err)
		}
		// fmt.Printf("[ engine.compress ] 2\n")
		if err = compressor.writer.Close(); err != nil {
			panic(err)
		}
		// fmt.Printf("[ engine.compress ] 3\n")
	}()

	// fmt.Printf("[ engine.compress ] 4\n")
	if content, err = io.ReadAll(compressor.reader); err != nil {
		panic(err)
	}
	// fmt.Printf("[ engine.compress ] 5\n")
	if err = compressor.reader.Close(); err != nil {
		panic(err)
	}
	// fmt.Printf("[ engine.compress ] 6\n")
	// fmt.Printf("[ engine.compress ] compressed content(in bytes):\n%v\n", content)
	if len(outputFileName) > 0 {
		if err = os.WriteFile(outputFileName, content, 0644); err != nil {
			panic(err)
		}
	}
	fmt.Printf("Original size (in bytes): %v\n", len(fileContent))
	fmt.Printf("Compressed size (in bytes): %v\n", len(content))
	fmt.Printf("Compression ratio: %.2f%%\n", float32(len(content))/float32(len(fileContent))*100)
	return content
}

func (c *compression) init(params any) {
	if !slices.Contains(Engines[:], c.compressionEngine) {
		fmt.Println("compression engine does not exist")
		os.Exit(1)
	}
	newReaderAndWriterFunc := compressionReaderAndWriters[c.compressionEngine]
	switch c.compressionEngine {
	case "huffman":
		c.reader, c.writer = newReaderAndWriterFunc.(func() (io.ReadCloser, io.WriteCloser))()
	case "lzss":
		c.reader, c.writer = newReaderAndWriterFunc.(func(int, int) (io.ReadCloser, io.WriteCloser))(4096, 4096)
	case "flate":
		if args, ok := params.(FlateArgs); !ok {
			panic("arguments missing for flate")
		} else {
			// fmt.Printf("[ engine.compression.init ] case flate selected with args: %v\n", args)
			c.reader, c.writer = newReaderAndWriterFunc.(func(uint32, uint32) (io.ReadCloser, io.WriteCloser))(args.Btype, args.Bfinal)
		}
	case "gzip":
		if args, ok := params.(GzipArgs); !ok {
			panic("arguments missing for gzip")
		} else {
			r, w := compressionReaderAndWriters["flate"].(func(uint32, uint32) (io.ReadCloser, io.WriteCloser))(args.Btype, args.Bfinal)
			c.reader, c.writer = newReaderAndWriterFunc.(func(io.ReadCloser, io.WriteCloser) (io.ReadCloser, io.WriteCloser))(r, w)
		}
	}
}

func DecompressFiles(algorithm string, files []string) {
	// fmt.Printf("DecompresFiles function params: (algorithms, files): (%v, %v)\n", algorithms, files)
	for _, file := range files {
		decompressFile(algorithm, file)
	}
}

func decompressFile(algorithm string, compressedFilePath string) {
	// outputFileName := strings.TrimSuffix(compressedFilePath, filepath.Ext(compressedFilePath))
	outputFileName := strings.SplitN(compressedFilePath, ".", 2)[0]
	outputFileName = outputFileName + "-decompressed" + ".txt"
	fileContent, err := os.ReadFile(compressedFilePath)
	// fmt.Printf("decompressFile function: compressFilePath: %v, outputFileName: %v, fileContent: %v\n", compressedFilePath, outputFileName, fileContent)
	if err != nil {
		panic(err)
	}
	fmt.Println("Decompressing...")
	decompress(algorithm, fileContent, outputFileName)
	fmt.Printf("File `%s` has been decompressed into File `%s` into the current directory\n", compressedFilePath, outputFileName)
}

func ServerDecompress(algorithm string, reader io.Reader) io.ReadCloser {
	if fileContent, err := io.ReadAll(reader); err != nil {
		panic(err)
	} else {
		content := decompress(algorithm, fileContent, "")
		pr, pw := io.Pipe()
		go func() {
			defer pw.Close()
			io.Copy(pw, bytes.NewReader(content))
		}()
		return pr
	}
}

func decompress(algorithm string, fileContent []byte, outputFileName string) []byte {
	decompressor := decompression{
		decompressionEngine: algorithm,
	}
	decompressor.init()
	var content []byte
	var err error
	go func() {
		if _, err = decompressor.writer.Write(fileContent); err != nil {
			panic(err)
		}
		if err = decompressor.writer.Close(); err != nil {
			panic(err)
		}
	}()
	if content, err = io.ReadAll(decompressor.reader); err != nil {
		panic(err)
	}
	if err = decompressor.reader.Close(); err != nil {
		panic(err)
	}
	if len(outputFileName) > 0 {
		if err = os.WriteFile(outputFileName, content, 0666); err != nil {
			panic(err)
		}
	}
	return content
}

func (d *decompression) init() {
	if !slices.Contains(Engines[:], d.decompressionEngine) {
		fmt.Println("decompression engine does not exist")
		os.Exit(1)
	}
	newReaderAndWriterFunc := decompressionReaderAndWriters[d.decompressionEngine]
	switch d.decompressionEngine {
	case "huffman":
		d.reader, d.writer = newReaderAndWriterFunc.(func() (io.ReadCloser, io.WriteCloser))()
	case "lzss":
		d.reader, d.writer = newReaderAndWriterFunc.(func() (io.ReadCloser, io.WriteCloser))()
	case "flate":
		d.reader, d.writer = newReaderAndWriterFunc.(func() (io.ReadCloser, io.WriteCloser))()
	case "gzip":
		r, w := decompressionReaderAndWriters["flate"].(func() (io.ReadCloser, io.WriteCloser))()
		d.reader, d.writer = newReaderAndWriterFunc.(func(io.ReadCloser, io.WriteCloser) (io.ReadCloser, io.WriteCloser))(r, w)
	}
}
