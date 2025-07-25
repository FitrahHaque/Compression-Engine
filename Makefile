BINARY  := shrink
DESTDIR := $(HOME)/bin

.PHONY: all build install clean dev compress decompress deflate inflate gzip server

all: build

build:
	go build -o $(BINARY).o .

install: build
	mkdir -p $(DESTDIR)
	install -m0755 $(BINARY).o $(DESTDIR)/$(BINARY)

clean:
	rm -f $(BINARY).o

dev: install
	shrink --compress --algorithm=huffman a.txt
	shrink --decompress a.txt.shk

compress: install
	shrink --compress --algorithm=lzss b.txt

decompress: compress
	shrink --decompress --algorithm=lzss b.txt.shk

deflate: install
	shrink --compress --algorithm=flate f.txt

inflate: deflate
	shrink --decompress --algorithm=flate f.txt.shk

gzip: install
	shrink --compress --algorithm=gzip g.txt
	shrink --decompress --algorithm=gzip g.txt.shk

server: install
	shrink --server --serverPort=8080 --algorithm=gzip s.txt