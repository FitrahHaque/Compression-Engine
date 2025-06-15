BINARY  := shrink
DESTDIR := $(HOME)/bin

.PHONY: all build install clean dev compress decompress

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
