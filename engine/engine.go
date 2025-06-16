package engine

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/FitrahHaque/Compression-Engine/compressor/huffman"
	"github.com/FitrahHaque/Compression-Engine/compressor/lzss"
)

var Engines = [...]string{
	"huffman",
	"lzss",
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
}

var decompressionReaderAndWriters = map[string]any{
	"huffman": huffman.NewDecompressionReaderAndWriter,
	"lzss":    lzss.NewDecompressionReaderAndWriter,
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
	var compressors []compression
	for _, algorithm := range algorithms {
		compressor := compression{
			compressionEngine: algorithm,
		}
		compressor.init()
		compressors = append(compressors, compressor)
	}
	content := fileContent
	for _, c := range compressors {
		if _, err := c.writer.Write(content); err != nil {
			panic(err)
		}
		if err = c.writer.Close(); err != nil {
			panic(err)
		}
		if content, err = io.ReadAll(c.reader); err != nil {
			panic(err)
		}
		if err = c.reader.Close(); err != nil {
			panic(err)
		}
	}
	if err = os.WriteFile(outputFileName, content, 0644); err != nil {
		panic(err)
	}
	fmt.Printf("File `%s` has been compressed into the file `%s`\n", filePath, outputFileName)
	fmt.Printf("Original size (in bytes): %v\n", len(fileContent))
	fmt.Printf("Compressed size (in bytes): %v\n", len(content))
	fmt.Printf("Compression ratio: %.2f%%\n", float32(len(content))/float32(len(fileContent))*100)
}

func DecompressFiles(algorithms []string, files []string) {
	// fmt.Printf("DecompresFiles function params: (algorithms, files): (%v, %v)\n", algorithms, files)
	for _, file := range files {
		decompressFile(algorithms, file)
	}
}

func decompressFile(algorithms []string, compressedFilePath string) {
	// outputFileName := strings.TrimSuffix(compressedFilePath, filepath.Ext(compressedFilePath))
	outputFileName := strings.SplitN(compressedFilePath, ".", 2)[0]
	outputFileName = outputFileName + "-decompressed" + ".txt"
	fileContent, err := os.ReadFile(compressedFilePath)
	// fmt.Printf("decompressFile function: compressFilePath: %v, outputFileName: %v, fileContent: %v\n", compressedFilePath, outputFileName, fileContent)
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
		decompressors = append(decompressors, decompressor)
	}
	content := fileContent
	for _, d := range decompressors {
		if _, err = d.writer.Write(content); err != nil {
			panic(err)
		}
		if err = d.writer.Close(); err != nil {
			panic(err)
		}
		if content, err = io.ReadAll(d.reader); err != nil {
			panic(err)
		}
		if err = d.reader.Close(); err != nil {
			panic(err)
		}
	}
	if err = os.WriteFile(outputFileName, content, 0666); err != nil {
		panic(err)
	}
	fmt.Printf("File `%s` has been decompressed into File `%s` into the current directory\n", compressedFilePath, outputFileName)
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
		return
	case "lzss":
		d.reader, d.writer = newReaderAndWriterFunc.(func() (io.ReadCloser, io.WriteCloser))()
	default:
		return
	}
}

func (c *compression) init() {
	if !slices.Contains(Engines[:], c.compressionEngine) {
		fmt.Println("compression engine does not exist")
		os.Exit(1)
	}
	newReaderAndWriterFunc := compressionReaderAndWriters[c.compressionEngine]
	switch c.compressionEngine {
	case "huffman":
		c.reader, c.writer = newReaderAndWriterFunc.(func() (io.ReadCloser, io.WriteCloser))()
		return
	case "lzss":
		c.reader, c.writer = newReaderAndWriterFunc.(func() (io.ReadCloser, io.WriteCloser))()
	default:
		return
	}
}
