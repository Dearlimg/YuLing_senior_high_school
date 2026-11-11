// Package resp 定义 HTTP 层统一的响应结构与错误码。
// 目的只有一个：让所有接口的返回形状一致，前端不必为每个接口写一套解析逻辑。
package resp

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// 业务错误码。与 HTTP 状态码解耦：
// HTTP 状态码表达"这次请求在协议层发生了什么事"，
// code 表达"业务上具体是什么原因"，两者不能互相替代。
const (
	CodeOK          = 0
	CodeInvalidArgs = 40001
	CodeNotFound    = 40401
	CodeInternal    = 50001
)

// Body 统一响应体。
type Body struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}

// OK 返回成功响应。
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{Code: CodeOK, Msg: "ok", Data: data})
}

// Fail 返回业务失败响应，httpStatus 由调用方按语义决定。
func Fail(c *gin.Context, httpStatus, code int, msg string) {
	c.JSON(httpStatus, Body{Code: code, Msg: msg})
}

// BadRequest 参数错误。
func BadRequest(c *gin.Context, msg string) {
	Fail(c, http.StatusBadRequest, CodeInvalidArgs, msg)
}

// NotFound 资源不存在。
func NotFound(c *gin.Context, msg string) {
	Fail(c, http.StatusNotFound, CodeNotFound, msg)
}

// Internal 服务端错误。对外不暴露内部细节，细节只走日志。
func Internal(c *gin.Context, msg string) {
	Fail(c, http.StatusInternalServerError, CodeInternal, msg)
}
