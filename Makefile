build:
	@if [ ! -f pkg/mailwatch/key ]; then \
		echo "Generating new encryption key..."; \
		dd if=/dev/urandom bs=32 count=1 2>/dev/null of=pkg/mailwatch/key; \
	else \
		echo "Key already exists, skipping key generation."; \
	fi
	go build -o mail-notify cmd/mailwatcher/*.go
	upx mail-notify
