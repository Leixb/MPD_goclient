MPD_goclient: assets/bundle.js
	go build
assets/bundle.js:
	 $(MAKE) -C webpack
	 cp webpack/dist/bundle.js assets/
