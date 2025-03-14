build:
	go generate ./assets/assets_generate.go
	go build .
	# upx mail-notify
