SOURCE := go.mod go.sum $(wildcard *.go)
TEMPLATES := $(wildcard templates/*.html)
STATIC := $(wildcard static/*)
BINARY_NAME := todo

.PHONY: run build clean

$(BINARY_NAME): $(SOURCE) $(TEMPLATES) $(STATIC) 
	go build -o $@ .

build: $(BINARY_NAME)

run: 
	go run .

clean:
	rm $(BINARY_NAME) || true
