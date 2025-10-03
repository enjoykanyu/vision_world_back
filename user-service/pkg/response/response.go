package response

import (
	"encoding/json"
	"net/http"

	"github.com/visionworld/user-service/pkg/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Code 响应代码
type Code int32

const (
	CodeSuccess           Code = 0    // 成功
	CodeInvalidParams     Code = 400  // 参数错误
	CodeUnauthorized      Code = 401  // 未授权
	CodeForbidden         Code = 403  // 无权限
	CodeNotFound          Code = 404  // 资源不存在
	CodeInternalError     Code = 500  // 内部错误
	CodeServiceError      Code = 503  // 服务错误
	CodeDatabaseError     Code = 600  // 数据库错误
	CodeCacheError        Code = 700  // 缓存错误
	CodeThirdPartyError   Code = 800  // 第三方服务错误
	CodeTooManyRequests   Code = 429  // 请求过多
	CodeUserLocked        Code = 1001 // 用户被锁定
	CodeInvalidToken      Code = 1002 // Token无效
	CodeTokenExpired      Code = 1003 // Token过期
	CodeUserNotFound      Code = 1004 // 用户不存在
	CodePasswordError     Code = 1005 // 密码错误
	CodeUserExists        Code = 1006 // 用户已存在
	CodeVerificationError Code = 1007 // 验证码错误
	CodeSMSLimit          Code = 1008 // 短信发送限制
)

// Message 响应消息映射
var Message = map[Code]string{
	CodeSuccess:           "success",
	CodeInvalidParams:     "参数错误",
	CodeUnauthorized:      "未授权",
	CodeForbidden:         "无权限",
	CodeNotFound:          "资源不存在",
	CodeInternalError:     "内部错误",
	CodeServiceError:      "服务错误",
	CodeDatabaseError:     "数据库错误",
	CodeCacheError:        "缓存错误",
	CodeThirdPartyError:   "第三方服务错误",
	CodeTooManyRequests:   "请求过多",
	CodeUserLocked:        "用户被锁定",
	CodeInvalidToken:      "Token无效",
	CodeTokenExpired:      "Token过期",
	CodeUserNotFound:      "用户不存在",
	CodePasswordError:     "密码错误",
	CodeUserExists:        "用户已存在",
	CodeVerificationError: "验证码错误",
	CodeSMSLimit:          "短信发送过于频繁",
}

// Response 通用响应
type Response struct {
	Code      int32                  `json:"code"`
	Message   string                 `json:"message"`
	Data      interface{}            `json:"data,omitempty"`
	Timestamp *timestamppb.Timestamp `json:"timestamp"`
}

// NewResponse 创建响应
func NewResponse(code Code, data interface{}) *Response {
	msg, ok := Message[code]
	if !ok {
		msg = "未知错误"
	}

	return &Response{
		Code:      int32(code),
		Message:   msg,
		Data:      data,
		Timestamp: timestamppb.Now(),
	}
}

// Success 成功响应
func Success(data interface{}) *Response {
	return NewResponse(CodeSuccess, data)
}

// Error 错误响应
func Error(code Code, message ...string) *Response {
	msg := Message[code]
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}

	resp := &Response{
		Code:      int32(code),
		Message:   msg,
		Timestamp: timestamppb.Now(),
	}

	// 记录错误日志
	logger.Errorw("错误响应", "code", code, "message", msg)

	return resp
}

// ErrorWithData 带数据的错误响应
func ErrorWithData(code Code, data interface{}, message ...string) *Response {
	resp := Error(code, message...)
	resp.Data = data
	return resp
}

// ToGRPCResponse 转换为gRPC响应
func ToGRPCResponse(resp *Response) (*anypb.Any, error) {
	data, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}

	return &anypb.Any{
		Value: data,
	}, nil
}

// FromGRPCResponse 从gRPC响应转换
func FromGRPCResponse(any *anypb.Any) (*Response, error) {
	var resp Response
	if err := json.Unmarshal(any.Value, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ToGRPCError 转换为gRPC错误
func ToGRPCError(code Code, message ...string) error {
	msg := Message[code]
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}

	// 映射到gRPC状态码
	var grpcCode codes.Code
	switch code {
	case CodeSuccess:
		grpcCode = codes.OK
	case CodeInvalidParams:
		grpcCode = codes.InvalidArgument
	case CodeUnauthorized:
		grpcCode = codes.Unauthenticated
	case CodeForbidden:
		grpcCode = codes.PermissionDenied
	case CodeNotFound:
		grpcCode = codes.NotFound
	case CodeTooManyRequests:
		grpcCode = codes.ResourceExhausted
	case CodeUserLocked, CodeInvalidToken, CodeTokenExpired:
		grpcCode = codes.Unauthenticated
	case CodeUserNotFound:
		grpcCode = codes.NotFound
	case CodePasswordError:
		grpcCode = codes.Unauthenticated
	case CodeUserExists:
		grpcCode = codes.AlreadyExists
	default:
		grpcCode = codes.Internal
	}

	return status.Error(grpcCode, msg)
}

// FromGRPCError 从gRPC错误转换
func FromGRPCError(err error) *Response {
	st, ok := status.FromError(err)
	if !ok {
		return Error(CodeInternalError, err.Error())
	}

	// 从gRPC状态码映射到自定义代码
	var code Code
	switch st.Code() {
	case codes.OK:
		code = CodeSuccess
	case codes.InvalidArgument:
		code = CodeInvalidParams
	case codes.Unauthenticated:
		code = CodeUnauthorized
	case codes.PermissionDenied:
		code = CodeForbidden
	case codes.NotFound:
		code = CodeNotFound
	case codes.ResourceExhausted:
		code = CodeTooManyRequests
	case codes.AlreadyExists:
		code = CodeUserExists
	default:
		code = CodeInternalError
	}

	return Error(code, st.Message())
}

// WriteJSON 写入JSON响应
func WriteJSON(w http.ResponseWriter, code int, resp *Response) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(resp)
}

// WriteSuccess 写入成功响应
func WriteSuccess(w http.ResponseWriter, data interface{}) error {
	return WriteJSON(w, http.StatusOK, Success(data))
}

// WriteError 写入错误响应
func WriteError(w http.ResponseWriter, code Code, message ...string) error {
	httpCode := getHTTPStatusCode(code)
	return WriteJSON(w, httpCode, Error(code, message...))
}

// getHTTPStatusCode 获取HTTP状态码
func getHTTPStatusCode(code Code) int {
	switch code {
	case CodeSuccess:
		return http.StatusOK
	case CodeInvalidParams:
		return http.StatusBadRequest
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeNotFound:
		return http.StatusNotFound
	case CodeTooManyRequests:
		return http.StatusTooManyRequests
	case CodeUserLocked, CodeInvalidToken, CodeTokenExpired:
		return http.StatusUnauthorized
	case CodeUserNotFound:
		return http.StatusNotFound
	case CodePasswordError:
		return http.StatusUnauthorized
	case CodeUserExists:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

// Pagination 分页信息
type Pagination struct {
	Page      int32 `json:"page"`
	PageSize  int32 `json:"page_size"`
	Total     int64 `json:"total"`
	TotalPage int32 `json:"total_page"`
}

// PaginatedResponse 分页响应
type PaginatedResponse struct {
	Code       int32                  `json:"code"`
	Message    string                 `json:"message"`
	Data       interface{}            `json:"data"`
	Pagination *Pagination            `json:"pagination"`
	Timestamp  *timestamppb.Timestamp `json:"timestamp"`
}

// NewPaginatedResponse 创建分页响应
func NewPaginatedResponse(code Code, data interface{}, pagination *Pagination) *PaginatedResponse {
	msg, ok := Message[code]
	if !ok {
		msg = "未知错误"
	}

	return &PaginatedResponse{
		Code:       int32(code),
		Message:    msg,
		Data:       data,
		Pagination: pagination,
		Timestamp:  timestamppb.Now(),
	}
}

// SuccessPaginated 成功分页响应
func SuccessPaginated(data interface{}, pagination *Pagination) *PaginatedResponse {
	return NewPaginatedResponse(CodeSuccess, data, pagination)
}
