package dto

import "time"

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/7/30 14:10:02
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/7/30 14:10:02
 */

type ResponseNonce struct {
	Nonce string `json:"nonce"`
}

type UserAuthTokenGenerateRequest struct {
	ValidityPeriod time.Duration `form:"validityPeriod" query:"validityPeriod"`
}

// UserAuthTokenSummary is the recoverable metadata returned by list and secret
// responses. It deliberately contains no raw token or digest.
type UserAuthTokenSummary struct {
	ID          string    `json:"id"`
	UserID      string    `json:"userID"`
	Fingerprint string    `json:"fingerprint"`
	ExpiredAt   time.Time `json:"expiredAt"`
	Revoked     bool      `json:"revoked"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// UserAuthTokenSecretResponse is returned only after a successful create or
// rotation. Token must never be used by list or persistent model responses.
type UserAuthTokenSecretResponse struct {
	UserAuthTokenSummary
	Token string `json:"token"`
}
