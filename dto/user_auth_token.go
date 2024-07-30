package dto

import "time"

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/7/30 14:10:02
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/7/30 14:10:02
 */

type UserAuthTokenGenerateRequest struct {
	ValidityPeriod time.Duration `form:"validityPeriod" query:"validityPeriod"`
}
