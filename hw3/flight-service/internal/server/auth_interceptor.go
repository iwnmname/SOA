package server

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func APIKeyAuthUnaryInterceptor(expectedKey string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing credentials")
		}

		keys := md.Get("x-api-key")
		if len(keys) == 0 || keys[0] == "" {
			return nil, status.Error(codes.Unauthenticated, "missing credentials")
		}

		if keys[0] != expectedKey {
			return nil, status.Error(codes.Unauthenticated, "invalid credentials")
		}

		return handler(ctx, req)
	}
}

