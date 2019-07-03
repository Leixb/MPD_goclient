MPD_goclient: bindata.go server.go
	go build

bindata.go: static_files/assets/bundle.js
	go-bindata static_files/...

static_files/assets/bundle.js:
	 $(MAKE) -C webpack
	 cp webpack/dist/bundle.js static_files/assets/
