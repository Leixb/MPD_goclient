.PHONY: install

MPD_goclient: bindata.go server.go
	go build

bindata.go:
	go-bindata static_files/...

install: bindata.go server.go
	go install

deps:
	go get -u github.com/go-bindata/go-bindata/...
	go get -u github.com/Leixb/mpdconn
	go get -u github.com/gin-gonic/gin
	go get -u github.com/akamensky/argparse
