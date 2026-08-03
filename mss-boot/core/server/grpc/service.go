package grpc

/*
 * @Author: lwnmengjing
 * @Date: 2021/6/8 5:29 下午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/6/8 5:29 下午
 */

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Service service client
type Service struct {
	Connection  *grpc.ClientConn
	CallTimeout time.Duration
}

// Dial dial
func (e *Service) Dial(
	endpoint string,
	callTimeout time.Duration,
	unary ...grpc.UnaryClientInterceptor) (err error) {
	log.Printf("configure service with endpoint: %s", endpoint)

	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()

	if len(unary) == 0 {
		unary = defaultUnaryClientInterceptors()
	}
	e.Connection, err = grpc.DialContext(ctx,
		endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainStreamInterceptor(defaultStreamClientInterceptors()...),
		grpc.WithChainUnaryInterceptor(unary...),
		grpc.WithDefaultCallOptions(grpc.WaitForReady(true), grpc.MaxCallRecvMsgSize(defaultMaxMsgSize)),
	)

	if err != nil {
		msg := fmt.Sprintf("connect gRPC service %s failed", endpoint)
		log.Println(msg, err)
		return fmt.Errorf("%w, "+msg, err)
	}
	return nil
}

func defaultUnaryClientInterceptors() []grpc.UnaryClientInterceptor {
	return []grpc.UnaryClientInterceptor{
		//opentracing.UnaryClientInterceptor(),
		logging.UnaryClientInterceptor(InterceptorLogger(slog.Default())),
		//reqtags.UnaryClientInterceptor(),
	}
}

func defaultStreamClientInterceptors() []grpc.StreamClientInterceptor {
	return []grpc.StreamClientInterceptor{
		//opentracing.StreamClientInterceptor(),
		logging.StreamClientInterceptor(InterceptorLogger(slog.Default())),
		//reqtags.StreamClientInterceptor(),
	}
}
