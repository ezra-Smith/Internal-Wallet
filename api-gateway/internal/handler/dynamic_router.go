package handler

import (
	"net/http"
	"strings"

	"internalwallet/api-gateway/internal/config"
)

// DynamicRouter 用于支持带路径参数（{id}）的路由匹配。
//
// 注意：net/http ServeMux 不支持模板路由，因此我们将动态路由统一挂载到 "/"，
// 由该 Router 在运行时进行匹配。静态路由仍由 ServeMux 直接匹配，保持兼容。
type DynamicRouter struct {
	routes []dynamicRoute
}

type dynamicRoute struct {
	method     string
	pattern    string
	pathParams []string
	handler    http.Handler
}

func NewDynamicRouter() *DynamicRouter {
	return &DynamicRouter{
		routes: make([]dynamicRoute, 0),
	}
}

func (r *DynamicRouter) HasRoutes() bool {
	return len(r.routes) > 0
}

func (r *DynamicRouter) Register(route config.Route, handler http.Handler) {
	r.routes = append(r.routes, dynamicRoute{
		method:     strings.ToUpper(route.Method),
		pattern:    route.Path,
		pathParams: route.PathParams,
		handler:    handler,
	})
}

func (r *DynamicRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// ServeMux 已经进行了静态路由匹配，只有未命中的请求会走到这里。
	// 这里按注册顺序匹配；如后续动态路由数量变多，可优化为前缀索引。
	for _, rt := range r.routes {
		if req.Method != rt.method && req.Method != http.MethodOptions {
			continue
		}

		params := parsePathParams(req.URL.Path, rt.pattern)
		if len(rt.pathParams) > 0 {
			if len(params) != len(rt.pathParams) {
				continue
			}
			// 所有 path param 都必须存在且非空
			ok := true
			for _, key := range rt.pathParams {
				if params[key] == "" {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
		} else {
			// 无 path param 的动态路由（理论上不会走到这里），要求完全匹配
			if strings.TrimRight(req.URL.Path, "/") != strings.TrimRight(rt.pattern, "/") {
				continue
			}
		}

		rt.handler.ServeHTTP(w, req)
		return
	}

	// 根路径探针：返回 OK 作为健康检查响应
	if req.URL.Path == "/" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
		return
	}

	// 未匹配任何动态路由
	WriteError(w, "Not found", http.StatusNotFound)
}
