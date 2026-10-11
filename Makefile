.PHONY: build

build:
	go build -tags sqlite_fts5 -o pkb_bin ./cmd/pkb
