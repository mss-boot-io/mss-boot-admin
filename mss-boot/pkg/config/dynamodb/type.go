package dynamodb

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2022/10/6 21:48:26
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2022/10/6 21:48:26
 */

import "github.com/aws/aws-sdk-go-v2/service/dynamodb"

// Tabler ddb model interface
type Tabler interface {
	TableName() string
	Make()
	C() *dynamodb.Client
}
