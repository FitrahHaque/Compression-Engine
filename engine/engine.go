package engine

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/FitrahHaque/Compression-Engine/compressor/huffman"
)

var Engines = [...]string{
	"huffman",
}

type compression struct {
	compressionEngine string
	compressedContent []byte
}

type decompression struct {
	decompressionEngine string
	writer              io.WriteCloser
	reader              io.ReadCloser
	decompressedContent []byte
}

var compressionWriters = map[string]interface{}{
	"huffman": huffman.NewCompressionWriter,
}

var decompressionReaderAndWriters = map[string]interface{}{
	"huffman": huffman.NewDecompressionReaderAndWriter,
}

func (c *compression) write(content []byte) (int, error) {
	newCm := compressionWriters[c.compressionEngine]
	var b bytes.Buffer
	var w io.WriteCloser
	w = newCm.(func(io.Writer) io.WriteCloser)(&b)
	defer w.Close()
	w.Write(content)
	c.compressedContent = b.Bytes()
	return len(c.compressedContent), nil
}

func CompressFiles(algorithms []string, files []string, fileExtension string) {
	for _, file := range files {
		compressFile(algorithms, file, file+fileExtension)
	}
}

func compressFile(algorithms []string, filePath string, outputFileName string) {
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		panic(err)
	}
	fmt.Println("Compressing...")
	compressed := compress(fileContent, algorithms)
	if err = os.WriteFile(outputFileName, compressed, 0644); err != nil {
		panic(err)
	}
	fmt.Printf("Original size (in bytes): %v\n", len(fileContent))
	fmt.Printf("Compressed size (in bytes): %v\n", len(compressed))
	fmt.Printf("Compression ratio: %.2f%%\n", float32(len(compressed))/float32(len(fileContent))*100)
}

func compress(content []byte, algorithms []string) []byte {
	for _, algorithm := range algorithms {
		compressor := compression{
			compressionEngine: algorithm,
		}
		if _, err := compressor.write(content); err != nil {
			fmt.Println("error compressing the document")
			os.Exit(1)
		}
		content = compressor.compressedContent
	}
	return content
}

func DecompressFiles(algorithms []string, files []string) {
	for _, file := range files {
		decompressFile(algorithms, file)
	}
}

func decompressFile(algorithms []string, compressedFilePath string) {
	outputFileName := strings.TrimSuffix(compressedFilePath, filepath.Ext(compressedFilePath))
	fileContent, err := os.ReadFile(compressedFilePath)
	if err != nil {
		panic(err)
	}
	fmt.Println("Decompressing...")
	slices.Reverse(algorithms)
	var decompressors []decompression
	for _, algorithm := range algorithms {
		decompressor := decompression{
			decompressionEngine: algorithm,
		}
		decompressor.init()
		// defer decompressor.compressedReader.Close()
		decompressors = append(decompressors, decompressor)
	}
	content := fileContent
	for _, d := range decompressors {
		d.writer.Write(content)
		d.writer.Close()
		if content, err = io.ReadAll(d.reader); err != nil {
			panic(err)
		}
	}
	if err = os.WriteFile(outputFileName, content, 0666); err != nil {
		panic(err)
	}
}

func Exists(strList []string, t string) bool {
	for _, s := range strList {
		if s == t {
			return true
		}
	}
	return false
}

func (d *decompression) init() {
	if !Exists(Engines[:], d.decompressionEngine) {
		fmt.Println("decompression engine does not exist")
		os.Exit(1)
	}
	newReaderAndWriterFunc := decompressionReaderAndWriters[d.decompressionEngine]
	switch d.decompressionEngine {
	case "huffman":
		d.reader, d.writer = newReaderAndWriterFunc.(func() (io.ReadCloser, io.WriteCloser))()
		return
	default:
		return
	}
}
