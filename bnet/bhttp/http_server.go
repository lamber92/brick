package bhttp

import (
	"github.com/gin-gonic/gin"
)

type Server struct {
	server *gin.Engine
}

func New() *Server {
	srv := gin.Default()
	return &Server{server: srv}
}

func (s *Server) Run(addr ...string) error {
	return s.server.Run(addr...)
}

func (s *Server) Middleware(handlers ...gin.HandlerFunc) *Server {
	for _, v := range handlers {
		s.server.Use(v)
	}
	return s
}

func (s *Server) Group(prefix string, groups ...func(group *RouterGroup)) *RouterGroup {
	if len(prefix) > 0 && prefix[0] != '/' {
		prefix = "/" + prefix
	}
	if prefix == "/" {
		prefix = ""
	}
	group := &RouterGroup{
		group:  s.server.Group(prefix),
		server: s,
		prefix: prefix,
	}
	if len(groups) > 0 {
		for _, v := range groups {
			v(group)
		}
	}
	return group
}
