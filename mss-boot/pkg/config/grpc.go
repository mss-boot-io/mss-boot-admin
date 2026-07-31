package config

import (
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/timeout"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/mss-boot-io/mss-boot/core/server"
	serverGRPC "github.com/mss-boot-io/mss-boot/core/server/grpc"
)

// GRPC grpc服务公共配置(选用)
type GRPC struct {
	ServerParams `yaml:",inline" json:",inline"`
	Clients      Clients `yaml:"client" json:"client"`
}

type ServerParams struct {
	Addr     string        `yaml:"addr" json:"addr"` // default:  :9090
	CertFile string        `yaml:"certFile" json:"certFile"`
	KeyFile  string        `yaml:"keyFile" json:"keyFile"`
	Timeout  time.Duration `yaml:"timeout" json:"timeout"` // default: 10
}

// Init grpc server
func (e *GRPC) Init(
	register func(srv *serverGRPC.Server),
	opts ...serverGRPC.Option) server.Runnable {
	if opts == nil {
		opts = make([]serverGRPC.Option, 0)
	}
	opts = append(opts,
		serverGRPC.WithAddr(e.Addr),
		serverGRPC.WithKey(e.KeyFile),
		serverGRPC.WithCert(e.CertFile),
		serverGRPC.WithTimeout(time.Duration(e.Timeout)*time.Second),
	)
	s := serverGRPC.New("grpc", opts...)
	s.Register(register)
	return s
}

type Clients map[string]ServerParams

func (cs Clients) makeClient(key string, opts ...grpc.DialOption) *grpc.ClientConn {
	params, ok := cs[key]
	if !ok {
		return nil
	}
	if opts == nil {
		opts = make([]grpc.DialOption, 0)
	}
	if params.CertFile != "" && params.KeyFile != "" {
		creds, err := credentials.NewClientTLSFromFile(params.CertFile, params.KeyFile)
		if err != nil {
			return nil
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	if params.Timeout > 0 {
		opts = append(opts,
			grpc.WithChainUnaryInterceptor(
				timeout.UnaryClientInterceptor(params.Timeout),
			))
	}
	conn, err := grpc.NewClient(params.Addr, opts...)
	if err != nil {
		return nil
	}
	return conn
}

func (e *GRPC) GetGRPCClient(key string, opts ...grpc.DialOption) *grpc.ClientConn {
	return e.Clients.makeClient(key, opts...)
}
