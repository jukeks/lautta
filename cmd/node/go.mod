module github.com/jukeks/lautta/cmd/node

go 1.24.3

replace github.com/jukeks/lautta => ../../

require (
	github.com/jukeks/lautta v0.0.0-00010101000000-000000000000
	github.com/jukeks/tukki v0.0.0-20250524143744-5165f7f8a720
	google.golang.org/grpc v1.72.1
)

require ( // indirect
	github.com/bits-and-blooms/bitset v1.10.0 // indirect
	github.com/bits-and-blooms/bloom/v3 v3.7.0 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	golang.org/x/net v0.35.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250218202821-56aae31c358a // indirect
	google.golang.org/protobuf v1.36.6 // indirect
)
