module metarang/websocket-gateway/tests

go 1.25.13

require (
	github.com/alicebob/miniredis/v2 v2.34.0
	github.com/gorilla/websocket v1.5.3
	metarang/websocket-gateway v0.0.0
)

require (
	github.com/alicebob/gopher-json v0.0.0-20230218143504-906a9b012302 // indirect
	github.com/andybalholm/brotli v1.2.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/gookit/color v1.5.4 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/quic-go/qpack v0.6.0 // indirect
	github.com/quic-go/quic-go v0.60.0 // indirect
	github.com/redis/go-redis/v9 v9.17.2 // indirect
	github.com/vmihailenco/msgpack/v5 v5.4.1 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	github.com/xo/terminfo v0.0.0-20210125001918-ca9a967f8778 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	github.com/zishang520/engine.io-go-parser v1.3.2 // indirect
	github.com/zishang520/engine.io/v2 v2.5.0 // indirect
	github.com/zishang520/socket.io-go-parser/v2 v2.5.0 // indirect
	github.com/zishang520/socket.io/v2 v2.5.0 // indirect
	github.com/zishang520/webtransport-go v0.9.1 // indirect
	golang.org/x/crypto v0.55.0 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
)

replace metarang/websocket-gateway => ../../services/websocket-gateway

replace metarang/shared => ../../shared

// Keep in sync with services/websocket-gateway: patched quic-go via local shim.
replace github.com/zishang520/webtransport-go => ../../services/websocket-gateway/third_party/webtransport-go
