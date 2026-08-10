package trace

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/grpc/metadata"
)

// InjectHTTPHeaders 把当前 ctx 的传播上下文（traceparent/tracestate）注入 HTTP
// 请求头：调用方在发出下游 HTTP 请求前调用，使下游服务经 Extract 接续本链路。
func InjectHTTPHeaders(ctx context.Context, header http.Header) {
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(header))
}

// InjectGRPCMetadata 把当前 ctx 的传播上下文注入 gRPC 出站 metadata 并返回新 ctx：
// 调用方在发出下游 gRPC 调用时用返回的 ctx，使下游服务经 Extract 接续本链路。
func InjectGRPCMetadata(ctx context.Context) context.Context {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.MD{}
	} else {
		md = md.Copy()
	}
	otel.GetTextMapPropagator().Inject(ctx, metadataCarrier(md))
	return metadata.NewOutgoingContext(ctx, md)
}

// metadataCarrier 把 gRPC metadata 适配为 propagation.TextMapCarrier（键小写）。
type metadataCarrier metadata.MD

func (c metadataCarrier) Get(key string) string {
	values := metadata.MD(c).Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (c metadataCarrier) Set(key, value string) {
	metadata.MD(c).Set(key, value)
}

func (c metadataCarrier) Keys() []string {
	md := metadata.MD(c)
	keys := make([]string, 0, len(md))
	for k := range md {
		keys = append(keys, k)
	}
	return keys
}
