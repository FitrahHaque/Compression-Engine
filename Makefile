BINARY  := shrink
DESTDIR := $(HOME)/bin

.PHONY: all build install clean

all: build

build:
	go build -o $(BINARY).o .

install: build
	mkdir -p $(DESTDIR)
	install -m0755 $(BINARY).o $(DESTDIR)/$(BINARY)

clean:
	rm -f $(BINARY).o