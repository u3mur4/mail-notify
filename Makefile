build:
	go run assets/assets_generate.go
	go build .
	# upx mail-notify
