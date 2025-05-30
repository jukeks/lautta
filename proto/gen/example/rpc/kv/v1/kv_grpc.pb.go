// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             (unknown)
// source: example/rpc/kv/v1/kv.proto

package kvv1

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

const (
	KVService_Write_FullMethodName = "/example.rpc.kv.v1.KVService/Write"
)

// KVServiceClient is the client API for KVService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type KVServiceClient interface {
	Write(ctx context.Context, in *WriteRequest, opts ...grpc.CallOption) (*WriteResponse, error)
}

type kVServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewKVServiceClient(cc grpc.ClientConnInterface) KVServiceClient {
	return &kVServiceClient{cc}
}

func (c *kVServiceClient) Write(ctx context.Context, in *WriteRequest, opts ...grpc.CallOption) (*WriteResponse, error) {
	out := new(WriteResponse)
	err := c.cc.Invoke(ctx, KVService_Write_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// KVServiceServer is the server API for KVService service.
// All implementations must embed UnimplementedKVServiceServer
// for forward compatibility
type KVServiceServer interface {
	Write(context.Context, *WriteRequest) (*WriteResponse, error)
	mustEmbedUnimplementedKVServiceServer()
}

// UnimplementedKVServiceServer must be embedded to have forward compatible implementations.
type UnimplementedKVServiceServer struct {
}

func (UnimplementedKVServiceServer) Write(context.Context, *WriteRequest) (*WriteResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Write not implemented")
}
func (UnimplementedKVServiceServer) mustEmbedUnimplementedKVServiceServer() {}

// UnsafeKVServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to KVServiceServer will
// result in compilation errors.
type UnsafeKVServiceServer interface {
	mustEmbedUnimplementedKVServiceServer()
}

func RegisterKVServiceServer(s grpc.ServiceRegistrar, srv KVServiceServer) {
	s.RegisterService(&KVService_ServiceDesc, srv)
}

func _KVService_Write_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(WriteRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(KVServiceServer).Write(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: KVService_Write_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(KVServiceServer).Write(ctx, req.(*WriteRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// KVService_ServiceDesc is the grpc.ServiceDesc for KVService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var KVService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "example.rpc.kv.v1.KVService",
	HandlerType: (*KVServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Write",
			Handler:    _KVService_Write_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "example/rpc/kv/v1/kv.proto",
}
