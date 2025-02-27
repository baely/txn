package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/hostrouter"
)

type Server struct {
	*http.Server

	hostRouter hostrouter.Routes
}

func New() *Server {
	hr := hostrouter.New()

	s := &Server{
		Server: &http.Server{
			Addr: ":8080",
		},
		hostRouter: hr,
	}

	r := chi.NewRouter()
	r.Mount("/", hr)
	s.Server.Handler = r

	return s
}

func (s *Server) RegisterDomain(domain string, router chi.Router) {
	s.hostRouter.Map(domain, router)
}
