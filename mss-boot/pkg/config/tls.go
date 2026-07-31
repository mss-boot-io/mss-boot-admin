package config

/*
 * @Author: lwnmengjing
 * @Date: 2022/3/21 19:05
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2022/3/21 19:05
 */

import (
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"os"
)

// TLS config
type TLS struct {
	// Cert cert file path
	Cert string `yaml:"cert" json:"cert"`
	// Key file path
	Key string `yaml:"key" json:"key"`
	// Ca file path
	Ca string `yaml:"ca" json:"ca"`
}

// GetTLS get tls config
func (c *TLS) GetTLS() (*tls.Config, error) {
	if c != nil && c.Cert != "" {
		// 从证书相关文件中读取和解析信息，得到证书公钥、密钥对
		cert, err := tls.LoadX509KeyPair(c.Cert, c.Key)
		if err != nil {
			slog.Error("tls.LoadX509KeyPair error", "err", err)
			return nil, err
		}
		// 创建一个新的、空的 CertPool，并尝试解析 PEM 编码的证书，解析成功会将其加到 CertPool 中
		certPool := x509.NewCertPool()
		ca, err := os.ReadFile(c.Ca)
		if err != nil {
			slog.Error("ioutil.ReadFile error", "err", err)
			return nil, err
		}

		if ok := certPool.AppendCertsFromPEM(ca); !ok {
			slog.Error("certPool.AppendCertsFromPEM error", "err", err)
			return nil, err
		}
		return &tls.Config{
			// 设置证书链，允许包含一个或多个
			Certificates: []tls.Certificate{cert},
			// 要求必须校验客户端的证书
			ClientAuth: tls.RequireAndVerifyClientCert,
			// 设置根证书的集合，校验方式使用 ClientAuth 中设定的模式
			ClientCAs: certPool,
		}, nil
	}
	return nil, nil
}
