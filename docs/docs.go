// Package docs GENERATED BY SWAG; DO NOT EDIT
// This file was generated by swaggo/swag
package docs

import "github.com/swaggo/swag"

const docTemplate = `{
    "schemes": {{ marshal .Schemes }},
    "swagger": "2.0",
    "info": {
        "description": "{{escape .Description}}",
        "title": "{{.Title}}",
        "contact": {},
        "version": "{{.Version}}"
    },
    "host": "{{.Host}}",
    "basePath": "{{.BasePath}}",
    "paths": {
        "/admin/api/github/callback": {
            "get": {
                "description": "github回调",
                "consumes": [
                    "application/json"
                ],
                "tags": [
                    "generator"
                ],
                "summary": "github回调",
                "parameters": [
                    {
                        "type": "string",
                        "description": "code",
                        "name": "code",
                        "in": "query",
                        "required": true
                    },
                    {
                        "type": "string",
                        "description": "state",
                        "name": "state",
                        "in": "query",
                        "required": true
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/dto.GithubToken"
                        }
                    },
                    "422": {
                        "description": "Unprocessable Entity",
                        "schema": {
                            "$ref": "#/definitions/response.Response"
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "$ref": "#/definitions/response.Response"
                        }
                    }
                }
            }
        },
        "/admin/api/github/control": {
            "post": {
                "security": [
                    {
                        "Bearer": []
                    }
                ],
                "description": "创建或更新github配置",
                "consumes": [
                    "application/json"
                ],
                "tags": [
                    "generator"
                ],
                "summary": "创建或更新github配置",
                "parameters": [
                    {
                        "description": "data",
                        "name": "data",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/dto.GithubControlReq"
                        }
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK"
                    }
                }
            }
        },
        "/admin/api/github/get": {
            "get": {
                "security": [
                    {
                        "Bearer": []
                    }
                ],
                "description": "获取github配置",
                "consumes": [
                    "application/json"
                ],
                "tags": [
                    "generator"
                ],
                "summary": "获取github配置",
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/dto.GithubGetResp"
                        }
                    }
                }
            }
        },
        "/admin/api/github/get-login-url": {
            "get": {
                "description": "获取github登录地址",
                "consumes": [
                    "application/json"
                ],
                "tags": [
                    "generator"
                ],
                "summary": "获取github登录地址",
                "parameters": [
                    {
                        "type": "string",
                        "description": "state",
                        "name": "state",
                        "in": "query",
                        "required": true
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "type": "string"
                        }
                    }
                }
            }
        },
        "/admin/api/menu/authorize/{id}": {
            "get": {
                "security": [
                    {
                        "Bearer": []
                    }
                ],
                "description": "获取菜单权限",
                "consumes": [
                    "application/json"
                ],
                "tags": [
                    "menu"
                ],
                "summary": "获取菜单权限",
                "parameters": [
                    {
                        "type": "string",
                        "description": "id",
                        "name": "id",
                        "in": "path",
                        "required": true
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "type": "array",
                            "items": {
                                "type": "string"
                            }
                        }
                    }
                }
            },
            "put": {
                "security": [
                    {
                        "Bearer": []
                    }
                ],
                "description": "更新菜单权限",
                "consumes": [
                    "application/json"
                ],
                "tags": [
                    "menu"
                ],
                "summary": "更新菜单权限",
                "parameters": [
                    {
                        "type": "string",
                        "description": "id",
                        "name": "id",
                        "in": "path",
                        "required": true
                    },
                    {
                        "description": "data",
                        "name": "data",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/dto.UpdateAuthorizeRequest"
                        }
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK"
                    }
                }
            }
        },
        "/admin/api/menu/tree": {
            "get": {
                "security": [
                    {
                        "Bearer": []
                    }
                ],
                "description": "获取菜单树",
                "tags": [
                    "menu"
                ],
                "summary": "获取菜单树",
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "type": "array",
                            "items": {
                                "allOf": [
                                    {
                                        "$ref": "#/definitions/models.MenuSingle"
                                    },
                                    {
                                        "type": "object",
                                        "properties": {
                                            "children": {
                                                "type": "array",
                                                "items": {
                                                    "$ref": "#/definitions/models.MenuSingle"
                                                }
                                            }
                                        }
                                    }
                                ]
                            }
                        }
                    }
                }
            }
        },
        "/admin/api/model/migrate/{id}": {
            "get": {
                "security": [
                    {
                        "Bearer": []
                    }
                ],
                "description": "迁移虚拟模型",
                "tags": [
                    "model"
                ],
                "summary": "迁移虚拟模型",
                "responses": {
                    "200": {
                        "description": "OK"
                    }
                }
            }
        },
        "/admin/api/role/authorize": {
            "post": {
                "security": [
                    {
                        "Bearer": []
                    }
                ],
                "description": "给角色授权",
                "tags": [
                    "role"
                ],
                "summary": "角色授权",
                "parameters": [
                    {
                        "description": "data",
                        "name": "data",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/dto.AuthorizeRequest"
                        }
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK"
                    }
                }
            }
        },
        "/admin/api/roles": {
            "get": {
                "security": [
                    {
                        "Bearer": []
                    }
                ],
                "description": "角色列表",
                "consumes": [
                    "application/json"
                ],
                "tags": [
                    "role"
                ],
                "summary": "角色列表",
                "parameters": [
                    {
                        "type": "integer",
                        "description": "page",
                        "name": "page",
                        "in": "query"
                    },
                    {
                        "type": "integer",
                        "description": "pageSize",
                        "name": "page_size",
                        "in": "query"
                    },
                    {
                        "type": "string",
                        "description": "id",
                        "name": "id",
                        "in": "query"
                    },
                    {
                        "type": "string",
                        "description": "name",
                        "name": "name",
                        "in": "query"
                    },
                    {
                        "type": "integer",
                        "description": "status",
                        "name": "status",
                        "in": "query"
                    },
                    {
                        "type": "string",
                        "description": "remark",
                        "name": "remark",
                        "in": "query"
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "allOf": [
                                {
                                    "$ref": "#/definitions/response.Page"
                                },
                                {
                                    "type": "object",
                                    "properties": {
                                        "data": {
                                            "type": "array",
                                            "items": {
                                                "$ref": "#/definitions/models.Role"
                                            }
                                        }
                                    }
                                }
                            ]
                        }
                    }
                }
            },
            "post": {
                "security": [
                    {
                        "Bearer": []
                    }
                ],
                "description": "创建角色",
                "consumes": [
                    "application/json"
                ],
                "tags": [
                    "role"
                ],
                "summary": "创建角色",
                "parameters": [
                    {
                        "description": "data",
                        "name": "data",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/models.Role"
                        }
                    }
                ],
                "responses": {
                    "201": {
                        "description": "Created"
                    }
                }
            }
        },
        "/admin/api/roles/{id}": {
            "put": {
                "security": [
                    {
                        "Bearer": []
                    }
                ],
                "description": "更新角色",
                "consumes": [
                    "application/json"
                ],
                "tags": [
                    "role"
                ],
                "summary": "更新角色",
                "parameters": [
                    {
                        "type": "string",
                        "description": "id",
                        "name": "id",
                        "in": "path",
                        "required": true
                    },
                    {
                        "description": "data",
                        "name": "data",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/models.Role"
                        }
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK"
                    }
                }
            },
            "delete": {
                "security": [
                    {
                        "Bearer": []
                    }
                ],
                "description": "删除角色",
                "tags": [
                    "role"
                ],
                "summary": "删除角色",
                "parameters": [
                    {
                        "type": "string",
                        "description": "id",
                        "name": "id",
                        "in": "path",
                        "required": true
                    }
                ],
                "responses": {
                    "204": {
                        "description": "No Content"
                    }
                }
            }
        },
        "/admin/api/template/generate": {
            "post": {
                "security": [
                    {
                        "Bearer": []
                    }
                ],
                "description": "从模版生成代码",
                "consumes": [
                    "application/json"
                ],
                "tags": [
                    "generator"
                ],
                "summary": "从模版生成代码",
                "parameters": [
                    {
                        "description": "data",
                        "name": "data",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/dto.TemplateGenerateReq"
                        }
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/dto.TemplateGenerateResp"
                        }
                    }
                }
            }
        },
        "/admin/api/template/get-branches": {
            "get": {
                "security": [
                    {
                        "Bearer": []
                    }
                ],
                "description": "获取template分支",
                "consumes": [
                    "application/json"
                ],
                "tags": [
                    "generator"
                ],
                "summary": "获取template分支",
                "parameters": [
                    {
                        "type": "string",
                        "description": "template source",
                        "name": "source",
                        "in": "query",
                        "required": true
                    },
                    {
                        "type": "string",
                        "description": "access token",
                        "name": "accessToken",
                        "in": "query"
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/dto.TemplateGetBranchesResp"
                        }
                    }
                }
            }
        },
        "/admin/api/template/get-params": {
            "get": {
                "security": [
                    {
                        "Bearer": []
                    }
                ],
                "description": "获取template参数配置",
                "consumes": [
                    "application/json"
                ],
                "tags": [
                    "generator"
                ],
                "summary": "获取template参数配置",
                "parameters": [
                    {
                        "type": "string",
                        "description": "template source",
                        "name": "source",
                        "in": "query",
                        "required": true
                    },
                    {
                        "type": "string",
                        "description": "branch default:HEAD",
                        "name": "branch",
                        "in": "query"
                    },
                    {
                        "type": "string",
                        "description": "path default:.",
                        "name": "path",
                        "in": "query"
                    },
                    {
                        "type": "string",
                        "description": "access token",
                        "name": "accessToken",
                        "in": "query"
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/dto.TemplateGetParamsResp"
                        }
                    }
                }
            }
        },
        "/admin/api/template/get-path": {
            "get": {
                "security": [
                    {
                        "Bearer": []
                    }
                ],
                "description": "获取template文件路径list",
                "consumes": [
                    "application/json"
                ],
                "tags": [
                    "generator"
                ],
                "summary": "获取template文件路径list",
                "parameters": [
                    {
                        "type": "string",
                        "description": "template source",
                        "name": "source",
                        "in": "query",
                        "required": true
                    },
                    {
                        "type": "string",
                        "description": "branch default:HEAD",
                        "name": "branch",
                        "in": "query"
                    },
                    {
                        "type": "string",
                        "description": "access token",
                        "name": "accessToken",
                        "in": "query"
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/dto.TemplateGetPathResp"
                        }
                    }
                }
            }
        },
        "/admin/api/user/userInfo": {
            "get": {
                "security": [
                    {
                        "Bearer": []
                    }
                ],
                "description": "获取登录用户信息",
                "consumes": [
                    "application/json"
                ],
                "tags": [
                    "user"
                ],
                "summary": "获取登录用户信息",
                "parameters": [
                    {
                        "type": "string",
                        "description": "id",
                        "name": "id",
                        "in": "path",
                        "required": true
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/models.User"
                        }
                    }
                }
            }
        }
    },
    "definitions": {
        "dto.AuthorizeRequest": {
            "type": "object",
            "properties": {
                "apiIDS": {
                    "type": "array",
                    "items": {
                        "type": "string"
                    }
                },
                "menuIDS": {
                    "type": "array",
                    "items": {
                        "type": "string"
                    }
                },
                "roleID": {
                    "type": "string"
                }
            }
        },
        "dto.GenerateParams": {
            "type": "object",
            "required": [
                "repo"
            ],
            "properties": {
                "params": {
                    "type": "object",
                    "additionalProperties": {
                        "type": "string"
                    }
                },
                "repo": {
                    "type": "string"
                },
                "service": {
                    "type": "string"
                }
            }
        },
        "dto.GithubControlReq": {
            "type": "object",
            "required": [
                "password"
            ],
            "properties": {
                "password": {
                    "description": "github密码或者token",
                    "type": "string"
                }
            }
        },
        "dto.GithubGetResp": {
            "type": "object",
            "properties": {
                "configured": {
                    "description": "已配置",
                    "type": "boolean"
                },
                "createdAt": {
                    "description": "创建时间",
                    "type": "string"
                },
                "email": {
                    "description": "github邮箱",
                    "type": "string"
                },
                "updatedAt": {
                    "description": "更新时间",
                    "type": "string"
                }
            }
        },
        "dto.GithubToken": {
            "type": "object",
            "properties": {
                "accessToken": {
                    "description": "AccessToken is the token that authorizes and authenticates\nthe requests.",
                    "type": "string"
                },
                "expiry": {
                    "description": "Expiry is the optional expiration time of the access token.\n\nIf zero, TokenSource implementations will reuse the same\ntoken forever and RefreshToken or equivalent\nmechanisms for that TokenSource will not be used.",
                    "type": "string"
                },
                "refreshToken": {
                    "description": "RefreshToken is a token that's used by the application\n(as opposed to the user) to refresh the access token\nif it expires.",
                    "type": "string"
                },
                "tokenType": {
                    "description": "TokenType is the type of token.\nThe Type method returns either this or \"Bearer\", the default.",
                    "type": "string"
                }
            }
        },
        "dto.TemplateGenerateReq": {
            "type": "object",
            "properties": {
                "accessToken": {
                    "type": "string"
                },
                "email": {
                    "type": "string"
                },
                "generate": {
                    "$ref": "#/definitions/dto.GenerateParams"
                },
                "template": {
                    "$ref": "#/definitions/dto.TemplateParams"
                }
            }
        },
        "dto.TemplateGenerateResp": {
            "type": "object",
            "properties": {
                "branch": {
                    "type": "string"
                },
                "repo": {
                    "type": "string"
                }
            }
        },
        "dto.TemplateGetBranchesResp": {
            "type": "object",
            "properties": {
                "branches": {
                    "type": "array",
                    "items": {
                        "type": "string"
                    }
                }
            }
        },
        "dto.TemplateGetParamsResp": {
            "type": "object",
            "properties": {
                "params": {
                    "type": "array",
                    "items": {
                        "$ref": "#/definitions/dto.TemplateParam"
                    }
                }
            }
        },
        "dto.TemplateGetPathResp": {
            "type": "object",
            "properties": {
                "path": {
                    "type": "array",
                    "items": {
                        "type": "string"
                    }
                }
            }
        },
        "dto.TemplateParam": {
            "type": "object",
            "properties": {
                "name": {
                    "type": "string"
                },
                "tip": {
                    "type": "string"
                }
            }
        },
        "dto.TemplateParams": {
            "type": "object",
            "required": [
                "source"
            ],
            "properties": {
                "branch": {
                    "type": "string"
                },
                "path": {
                    "type": "string"
                },
                "source": {
                    "type": "string"
                }
            }
        },
        "dto.UpdateAuthorizeRequest": {
            "type": "object",
            "required": [
                "keys",
                "roleID"
            ],
            "properties": {
                "keys": {
                    "type": "array",
                    "items": {
                        "type": "string"
                    }
                },
                "roleID": {
                    "type": "string"
                }
            }
        },
        "models.MenuSingle": {
            "type": "object",
            "properties": {
                "breadcrumb": {
                    "type": "boolean"
                },
                "createdAt": {
                    "description": "CreatedAt create time",
                    "type": "string"
                },
                "id": {
                    "description": "ID primary key",
                    "type": "string"
                },
                "ignore": {
                    "type": "boolean"
                },
                "key": {
                    "type": "string"
                },
                "name": {
                    "type": "string"
                },
                "parentId": {
                    "type": "string"
                },
                "select": {
                    "type": "boolean"
                },
                "title": {
                    "type": "string"
                },
                "updatedAt": {
                    "description": "UpdatedAt update time",
                    "type": "string"
                }
            }
        },
        "models.Role": {
            "type": "object",
            "properties": {
                "createdAt": {
                    "description": "CreatedAt create time",
                    "type": "string"
                },
                "id": {
                    "description": "ID primary key",
                    "type": "string"
                },
                "name": {
                    "type": "string"
                },
                "remark": {
                    "type": "string"
                },
                "root": {
                    "type": "boolean"
                },
                "status": {
                    "type": "integer"
                },
                "updatedAt": {
                    "description": "UpdatedAt update time",
                    "type": "string"
                }
            }
        },
        "models.User": {
            "type": "object",
            "properties": {
                "accountId": {
                    "type": "string"
                },
                "avatar": {
                    "type": "string"
                },
                "createdAt": {
                    "description": "CreatedAt create time",
                    "type": "string"
                },
                "email": {
                    "type": "string"
                },
                "id": {
                    "description": "ID primary key",
                    "type": "string"
                },
                "introduction": {
                    "type": "string"
                },
                "job": {
                    "type": "string"
                },
                "jobName": {
                    "type": "string"
                },
                "location": {
                    "type": "string"
                },
                "locationName": {
                    "type": "string"
                },
                "name": {
                    "type": "string"
                },
                "organization": {
                    "type": "string"
                },
                "organizationName": {
                    "type": "string"
                },
                "password": {
                    "type": "string"
                },
                "permissions": {
                    "type": "object",
                    "additionalProperties": {
                        "type": "array",
                        "items": {
                            "type": "string"
                        }
                    }
                },
                "personalWebsite": {
                    "type": "string"
                },
                "phoneNumber": {
                    "type": "string"
                },
                "registrationTime": {
                    "type": "string"
                },
                "roleId": {
                    "type": "string"
                },
                "status": {
                    "type": "integer"
                },
                "updatedAt": {
                    "description": "UpdatedAt update time",
                    "type": "string"
                },
                "username": {
                    "type": "string"
                },
                "verified": {
                    "type": "boolean"
                }
            }
        },
        "response.Page": {
            "type": "object",
            "properties": {
                "current": {
                    "type": "integer"
                },
                "pageSize": {
                    "type": "integer"
                },
                "total": {
                    "type": "integer"
                }
            }
        },
        "response.Response": {
            "type": "object",
            "properties": {
                "code": {
                    "type": "integer"
                },
                "errorCode": {
                    "type": "string"
                },
                "errorMessage": {
                    "type": "string"
                },
                "host": {
                    "type": "string"
                },
                "showType": {
                    "type": "integer"
                },
                "status": {
                    "type": "string"
                },
                "success": {
                    "type": "boolean"
                },
                "traceId": {
                    "type": "string"
                }
            }
        }
    },
    "securityDefinitions": {
        "Bearer": {
            "type": "apiKey",
            "name": "Authorization",
            "in": "header"
        }
    }
}`

// SwaggerInfo holds exported Swagger Info so clients can modify it
var SwaggerInfo = &swag.Spec{
	Version:          "0.0.1",
	Host:             "",
	BasePath:         "",
	Schemes:          []string{},
	Title:            "admin API",
	Description:      "admin接口文档",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  docTemplate,
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}
