package interceptors

import (
	grpclogging "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
	"github.com/hasanhakkaev/yqapp-demo/internal/telemetry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"time"
)

func NewClientUnaryInterceptors(telemeter telemetry.Telemetry) grpc.DialOption {
	return grpc.WithChainUnaryInterceptor(
		grpclogging.UnaryClientInterceptor(interceptorLogger(telemeter.Logger)),
		retry.UnaryClientInterceptor(
			retry.WithCodes(codes.ResourceExhausted, codes.Unavailable),
			retry.WithMax(10),
			retry.WithBackoff(retry.BackoffExponential(50*time.Millisecond)),
		),
	)
}

func NewClientStreamInterceptors(telemeter telemetry.Telemetry) grpc.DialOption {
	return grpc.WithChainStreamInterceptor(
		grpclogging.StreamClientInterceptor(interceptorLogger(telemeter.Logger)),
		retry.StreamClientInterceptor(
			retry.WithCodes(codes.ResourceExhausted, codes.Unavailable),
			retry.WithMax(10),
			retry.WithBackoff(retry.BackoffExponential(50*time.Millisecond)),
		),
	)
}
