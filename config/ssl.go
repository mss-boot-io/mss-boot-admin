package config

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/3/1 10:20:14
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/3/1 10:20:14
 */

type Ssl struct {
	KeyStr string
	Pem    string
	Enable bool
	Domain string
}

var SslConfig = new(Ssl)
