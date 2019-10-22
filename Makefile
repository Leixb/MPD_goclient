.PHONY: install deps

MPD_goclient: bindata.go server.go
	go build

bindata.go: static_files/*
	go-bindata static_files/...

install: bindata.go server.go
	go install

deps:
	go get -u github.com/go-bindata/go-bindata/...
