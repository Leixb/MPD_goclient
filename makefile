.PHONY: install

MPD_goclient: bindata.go server.go
	go build

bindata.go:
	go-bindata static_files/...

install: bindata.go server.go
	go install
