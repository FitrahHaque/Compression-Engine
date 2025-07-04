# Compression-Engine

A concept project to learn how the Gzip algorithm works from the ground up. Supports multiple lossless compression engines, CLI tools for local file compression/decompression, and an HTTP server that accepts compressed payloads and decompresses it.

## 🔧 Supported Algorithms

- **Huffman**
- **LZSS** (sliding-window, parallel block processing)
- **Deflate** (LZSS + dynamic/fixed Huffman coding)
- **Gzip** (DEFLATE + Gzip header & trailer)

## 🚀 Installation

```sh
git clone https://github.com/FitrahHaque/Compression-Engine.git
cd Compression-Engine
go mod tidy
go build -o shrink .
```

(Optional) Install into your ~/bin for easy CLI access:
```sh
make install
```

Make sure `~/bin` is on your PATH:
```sh
echo 'export PATH="$HOME/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

## 🖥️ Local CLI Usage

**Compress a file:**
```sh
shrink --compress --algorithm=huffman   --outfileext=.huf example.txt
shrink --compress --algorithm=lzss      --outfileext=.lzs example.txt
shrink --compress --algorithm=deflate   --outfileext=.dfl example.txt
shrink --compress --algorithm=gzip      --outfileext=.gz  example.txt
```

**Decompress a file:**
```sh
shrink --decompress --algorithm=huffman example.txt.huf
shrink --decompress --algorithm=lzss    example.txt.lzs
shrink --decompress --algorithm=deflate example.txt.dfl
shrink --decompress --algorithm=gzip    example.txt.gz
```

<!-- **Benchmark compression:**
```sh
shrink --benchmark \
  --algorithm="huffman,lzss,deflate,gzip" \
  example.txt,large.log
``` -->

## 🌐 HTTP Server Mode

Spin up a one-request server that accepts a compressed POST body, writes out the decompressed payload, and then shuts down:
```sh
shrink --server \
  --serverPort=8081 \
  --algorithm=gzip \
  somefile.txt
```

Client will POST `somefile.txt` (compressed with your chosen algorithm).  
Server auto-decompresses and writes it to `server-decompressed.txt`.  
Server shuts down after handling the single request.
