package errors

import "errors"

//go:generate stringer -type ErrCode -output error_code_string.go

// ErrCode 错误码类型
type ErrCode int32

const (
	// StoreSvcBaseErrorCode panic
	StoreSvcBaseErrorCode = 10000
	// GeneratorSvcBaseErrCode generator error code
	GeneratorSvcBaseErrCode = 20000
	// TenantSvcBaseErrCode tenant error code
	TenantSvcBaseErrCode = 30000
	// AdminSvcBaseErrCode admin error code
	AdminSvcBaseErrCode = 40000
)

const (
	// SUCCESS 约定成功为 0
	SUCCESS ErrCode = 0
)

const (
	// StoreSvcOperateAdapterFailed store适配器操作失败
	StoreSvcOperateAdapterFailed ErrCode = iota + StoreSvcBaseErrorCode
)

const (
	// GeneratorSvcParamsInvalid generator参数校验失败
	GeneratorSvcParamsInvalid ErrCode = iota + GeneratorSvcBaseErrCode
	// GeneratorSvcOperateDBFailed generator数据库操作失败
	GeneratorSvcOperateDBFailed
	// GeneratorSvcRecordIsExist generator记录已经存在
	GeneratorSvcRecordIsExist
	// GeneratorSvcRecordNotFound generator记录不存在
	GeneratorSvcRecordNotFound
	// GeneratorSvcObjectIDInvalid generator记录不存在
	GeneratorSvcObjectIDInvalid
)

const (
	// TenantSvcParamsInvalid tenant参数校验失败
	TenantSvcParamsInvalid ErrCode = iota + TenantSvcBaseErrCode
	// TenantSvcOperateDBFailed tenant数据库操作失败
	TenantSvcOperateDBFailed
	// TenantSvcRecordIsExist tenant记录已经存在
	TenantSvcRecordIsExist
	// TenantSvcRecordNotFound tenant记录不存在
	TenantSvcRecordNotFound
	// TenantSvcObjectIDInvalid  tenant记录不存在
	TenantSvcObjectIDInvalid
	// TenantSvcGetAuthTokenFailed tenant获取token失败
	TenantSvcGetAuthTokenFailed
	// TenantSvcAccessTokenParseFailed tenant解析token失败
	TenantSvcAccessTokenParseFailed
)

const (
	// AdminSvcParamsInvalid admin参数校验失败
	AdminSvcParamsInvalid ErrCode = iota + AdminSvcBaseErrCode
	// AdminSvcUnauthorized admin登录失败
	AdminSvcUnauthorized
	// AdminSvcOperateDBFailed admin数据库操作失败
	AdminSvcOperateDBFailed
	// AdminSvcForbidden admin鉴权失败
	AdminSvcForbidden
	// AdminSvcRecordIsExist admin记录已经存在
	AdminSvcRecordIsExist
	// AdminSvcDeleteRecordNotExist admin删除的记录不存在
	AdminSvcDeleteRecordNotExist
)

// Code 返回错误码
func (e ErrCode) Code() int32 {
	return int32(e)
}

// CheckErrorCode 判断错误码是否是成功错误吗
func CheckErrorCode(errCode int32) bool {
	return int32(SUCCESS) == errCode
}

// Is reports whether any error in err's chain matches target.
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// New returns an error that formats as the given text.
// Each call to New returns a distinct error value even if the text is identical.
func New(text string) error {
	return errors.New(text)
}

// Unwrap returns the result of calling the Unwrap method on err, if err's
// type contains an Unwrap method returning error.
// Otherwise, Unwrap returns nil.
func Unwrap(err error) error {
	return errors.Unwrap(err)
}

// As panics if target is not a non-nil pointer to either a type that implements
// error, or to any interface type.
func As(err error, target any) bool {
	return errors.As(err, target)
}
