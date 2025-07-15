BINARY = go-watch-something
SRC = main.go
PREFIX = /usr/local
BINDIR = $(PREFIX)/bin

.PHONY: build install clean

build:
	go build -o $(BINARY) $(SRC)

install: build
	install -Dm755 $(BINARY) $(BINDIR)/$(BINARY)

clean:
	rm -f $(BINARY)

