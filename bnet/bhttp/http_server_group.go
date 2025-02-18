package bhttp

import (
	"github.com/gin-gonic/gin"
	"github.com/lamber92/go-brick/bcontext"
	"net/http"
	"reflect"
	"strings"
)

type RouterGroup struct {
	group      *gin.RouterGroup
	parent     *RouterGroup
	server     *Server
	prefix     string
	middleware []*gin.HandlerFunc
}

func (g *RouterGroup) Group(prefix string, groups ...func(group *RouterGroup)) *RouterGroup {
	if len(prefix) > 0 && prefix[0] != '/' {
		prefix = "/" + prefix
	}
	if prefix == "/" {
		prefix = ""
	}
	group := &RouterGroup{
		group:  g.group.Group(prefix),
		parent: g,
		server: g.server,
		prefix: prefix,
	}
	if len(g.middleware) > 0 {
		group.middleware = make([]*gin.HandlerFunc, len(g.middleware))
		copy(group.middleware, g.middleware)
	}
	if len(groups) > 0 {
		for _, v := range groups {
			v(group)
		}
	}
	return group
}

func (g *RouterGroup) Bind(objs ...interface{}) *RouterGroup {
	for _, v := range objs {
		g.register(v)
	}
	return g
}

// register 读取 `req struct` 的 Path/Method，自动注册路由
func (g *RouterGroup) register(obj interface{}) {
	objType := reflect.TypeOf(obj)
	objValue := reflect.ValueOf(obj)

	// 遍历所有方法
	for i := 0; i < objType.NumMethod(); i++ {
		method := objType.Method(i)

		// 解析 `req struct`
		// TODO: 需要校验参数个数和参数类型，给出可读性比较高的提示
		// 第一个参数是ctx，第二个参数是请求参数
		reqType := method.Type.In(2).Elem()
		metaField, exists := reqType.FieldByName("Meta")
		if !exists {
			continue
		}

		// 获取 Path 和 Method
		meta := metaField.Tag
		path := meta.Get("path")
		httpMethod := strings.ToUpper(meta.Get("method"))

		if path == "" || httpMethod == "" {
			continue
		}

		// 绑定 API
		handler := func(ctx *gin.Context) {
			// 解析 req struct
			reqInstance := reflect.New(reqType).Interface()
			var err error

			switch httpMethod {
			case http.MethodGet:
				err = ctx.ShouldBindQuery(reqInstance)
			case http.MethodPost, http.MethodPut:
				err = ctx.ShouldBindJSON(reqInstance)
			default:
				err = ctx.ShouldBind(reqInstance)
			}

			if err != nil {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			// 调用 API 方法
			bCtx := bcontext.NewWithCtx(ctx)
			response := method.Func.Call([]reflect.Value{objValue, reflect.ValueOf(reqInstance), reflect.ValueOf(bCtx)})
			if len(response) > 0 {
				ctx.JSON(http.StatusOK, response[0].Interface())
			}
		}

		// Gin 注册路由
		g.group.Handle(httpMethod, path, handler)
	}
}

func (g *RouterGroup) Middleware(handlers ...gin.HandlerFunc) *RouterGroup {
	for _, v := range handlers {
		g.group.Use(v)
	}
	return g
}
