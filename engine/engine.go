package engine

import (
	"bytes"
	"fmt"
	"io"
	"os"
)

var Engines = [...]string{
	"huffman",
}

type compressor struct {
	compressionEngine     string
	compressedContent     []byte
	decomposed            []byte
	pos                   int
	maxSearchBufferLength int
}

var writers = map[string]interface{}{
	"huffman": huffman.NewWriter,
}

// var readers = map[string]interface{}{
// 	"huffman": huffman.NewReader,
// }

func (c *compressor) write(content []byte) error {
	newWriter := writers[c.compressionEngine]
	var b bytes.Buffer
	var w io.WriteCloser
	defer w.Close()
	w = newWriter.(func(io.Writer) io.WriteCloser)(&b)
	w.Write(content)
	c.compressedContent = b.Bytes()
	return nil
}

func CompressFiles(algorithms []string, files []string, fileExtension string) {
	for _, file := range files {
		compressFile(algorithms, file, file+fileExtension)
	}
}

func compressFile(algorithms []string, filePath string, outputFileName string) []byte {
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
	return compressed
}

func compress(content []byte, algorithms []string) []byte {
	for _, algorithm := range algorithms {
		file := compressor{
			maxSearchBufferLength: 4096,
			compressionEngine:     algorithm,
		}
		file.write(content)
		content = file.compressedContent
	}
	return content
}
