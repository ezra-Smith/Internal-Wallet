package utils

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// TracingInterceptor 链路追踪拦截器
// 自动捕���gRPC调用的路径、参数、结果等信息
func TracingInterceptor(serviceName string) grpc.UnaryServerInterceptor {
	tracer := otel.Tracer(serviceName)

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// 提取方法名（去掉服务前缀，只保留方法名）
		methodName := extractMethodName(info.FullMethod)

		// 开始链路追踪
		spanCtx, span := tracer.Start(ctx, methodName)
		defer span.End()

		// 设置基本属性
		span.SetAttributes(
			attribute.String("grpc.service", serviceName),
			attribute.String("grpc.method", methodName),
			attribute.String("grpc.full_method", info.FullMethod),
			attribute.String("service.type", "grpc"),
		)

		// 从上下文中提取客户端信息
		if clientInfo := extractClientInfo(ctx); clientInfo != nil {
			span.SetAttributes(clientInfo...)
		}

		// 提取请求参数信息
		if reqAttributes := extractRequestInfo(req); reqAttributes != nil {
			span.SetAttributes(reqAttributes...)
		}

		// 记录请求开始事件
		span.AddEvent("grpc.request_started", trace.WithTimestamp(time.Now()))

		// 调用实际的处理函数
		resp, err := handler(spanCtx, req)

		// 记录处理完成事件
		span.AddEvent("grpc.request_completed", trace.WithTimestamp(time.Now()))

		if err != nil {
			// 记录错误信息
			st, ok := status.FromError(err)
			if ok {
				span.SetAttributes(
					attribute.String("grpc.error_code", st.Code().String()),
					attribute.String("grpc.error_message", st.Message()),
				)
				span.SetStatus(codes.Error, st.Message())
			} else {
				span.SetAttributes(
					attribute.String("error.type", "unknown"),
					attribute.String("error.message", err.Error()),
				)
				span.SetStatus(codes.Error, err.Error())
			}
			span.RecordError(err)
		} else {
			// 记录成功响应信息
			if respAttributes := extractResponseInfo(resp); respAttributes != nil {
				span.SetAttributes(respAttributes...)
			}
			span.SetStatus(codes.Ok, "success")
			span.SetAttributes(attribute.Bool("success", true))
		}

		return resp, err
	}
}

// ClientTracingInterceptor 客户端链路追踪拦截器
func ClientTracingInterceptor(serviceName string) grpc.UnaryClientInterceptor {
	tracer := otel.Tracer(serviceName + "-client")

	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		methodName := extractMethodName(method)

		spanCtx, span := tracer.Start(ctx, methodName+"-client")
		defer span.End()

		// 设置客户端属性
		span.SetAttributes(
			attribute.String("grpc.client.service", serviceName),
			attribute.String("grpc.client.method", methodName),
			attribute.String("grpc.client.full_method", method),
		)

		// 提取请求信息
		if reqAttributes := extractRequestInfo(req); reqAttributes != nil {
			span.SetAttributes(reqAttributes...)
		}

		span.AddEvent("grpc.client.request_started")

		// 注入链路追踪信息到metadata
		if injectableCtx, injectErr := injectSpanContext(spanCtx); injectErr == nil {
			ctx = injectableCtx
		}

		err := invoker(spanCtx, method, req, reply, cc, opts...)

		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.RecordError(err)
		} else {
			span.SetStatus(codes.Ok, "success")
			if respAttributes := extractResponseInfo(reply); respAttributes != nil {
				span.SetAttributes(respAttributes...)
			}
		}

		span.AddEvent("grpc.client.request_completed")

		return err
	}
}

// extractMethodName 从gRPC完整方法名中提取方法名
// 例如: /pb.Account/CreateAccount -> CreateAccount
func extractMethodName(fullMethod string) string {
	parts := strings.Split(fullMethod, "/")
	if len(parts) >= 3 {
		return parts[2]
	}
	return fullMethod
}

// extractClientInfo 从上下文中提取客户端信息
func extractClientInfo(ctx context.Context) []attribute.KeyValue {
	var attrs []attribute.KeyValue

	// 提取对端信息
	if p, ok := peer.FromContext(ctx); ok {
		attrs = append(attrs,
			attribute.String("client.addr", p.Addr.String()),
		)

		// 安全地处理AuthInfo（可能为nil）
		if p.AuthInfo != nil {
			attrs = append(attrs,
				attribute.String("client.auth_type", p.AuthInfo.AuthType()),
			)
		}
	}

	// 提取metadata信息
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		// 提取有用的metadata
		if userAgent := md["user-agent"]; len(userAgent) > 0 {
			attrs = append(attrs, attribute.String("client.user_agent", userAgent[0]))
		}

		if traceID := md["x-trace-id"]; len(traceID) > 0 {
			attrs = append(attrs, attribute.String("trace.id", traceID[0]))
		}

		if userID := md["x-user-id"]; len(userID) > 0 {
			attrs = append(attrs, attribute.String("user.id", userID[0]))
		}

		if requestID := md["x-request-id"]; len(requestID) > 0 {
			attrs = append(attrs, attribute.String("request.id", requestID[0]))
		}
	}

	return attrs
}

// extractRequestInfo 从请求对象中提取有用的信息
func extractRequestInfo(req interface{}) []attribute.KeyValue {
	if req == nil {
		return nil
	}

	// 记录请求类型
	reqType := fmt.Sprintf("%T", req)
	attrs := []attribute.KeyValue{
		attribute.String("request.type", reqType),
	}

	// 安全地提取特定类型的请求信息
	if r, ok := req.(interface{ GetUserId() int64 }); ok {
		attrs = append(attrs, attribute.Int64("request.user_id", r.GetUserId()))
	} else if r, ok := req.(interface{ GetUserId() uint64 }); ok {
		attrs = append(attrs, attribute.Int64("request.user_id", int64(r.GetUserId())))
	}

	return attrs
}

// extractResponseInfo 从响应对象中提取有用的信息
func extractResponseInfo(resp interface{}) []attribute.KeyValue {
	if resp == nil {
		return nil
	}

	// 记录响应类型
	respType := fmt.Sprintf("%T", resp)
	attrs := []attribute.KeyValue{
		attribute.String("response.type", respType),
	}

	// 安全地提取特定类型的响应信息
	if r, ok := resp.(interface{ GetCode() int32 }); ok {
		attrs = append(attrs, attribute.Int64("response.code", int64(r.GetCode())))
	} else if r, ok := resp.(interface{ GetCode() int64 }); ok {
		attrs = append(attrs, attribute.Int64("response.code", r.GetCode()))
	} else if r, ok := resp.(interface{ GetMessage() string }); ok {
		attrs = append(attrs, attribute.String("response.message", r.GetMessage()))
	}

	return attrs
}

// injectSpanContext 将链路追踪信息注入到context中
func injectSpanContext(ctx context.Context) (context.Context, error) {
	// 这里可以使用OpenTelemetry的propagation功能
	// 简化实现，实际项目中应该使用标准的propagation
	return ctx, nil
}

// TracingServerOption 链路追踪的服务器选项
func TracingServerOption(serviceName string) grpc.ServerOption {
	return grpc.UnaryInterceptor(TracingInterceptor(serviceName))
}

// TracingClientOption 链路追踪的客户端选项
func TracingClientOption(serviceName string) grpc.DialOption {
	return grpc.WithUnaryInterceptor(ClientTracingInterceptor(serviceName))
}
