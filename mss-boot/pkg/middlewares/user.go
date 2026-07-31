package middlewares

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2022/4/21 15:40
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2022/4/21 15:40
 */

/**
{
  "iss": "http://127.0.0.1:5556/dex",
  "sub": "CgcyMzQyNzQ5EgZnaXRodWI",
  "aud": "example-app",
  "exp": 1492882042,
  "iat": 1492795642,
  "at_hash": "bi96gOXZShvlWYtal9Eqiw",
  "email": "jane.doe@coreos.com",
  "email_verified": true,
  "groups": [
    "admins",
    "developers"
  ],
  "name": "Jane Doe"
}
*/

// User user
type User struct {
	Issuer        string   `json:"iss"`
	Subject       string   `json:"sub"`
	Audience      string   `json:"aud"`
	ExpiresAt     int64    `json:"exp"`
	IssuedAt      int64    `json:"iat"`
	AtHash        string   `json:"at_hash"`
	Email         string   `json:"email"`
	EmailVerified bool     `json:"email_verified"`
	Groups        []string `json:"groups"`
	Name          string   `json:"name"`
}
