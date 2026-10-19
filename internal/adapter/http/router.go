package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/qkhanh39/go-common/validation"
	"github.com/qkhanh39/go-job-processing/internal/adapter/http/handler"
	"github.com/qkhanh39/go-job-processing/internal/service/job"
)

type Router struct {
	jobHandler *handler.JobHandler
}

func New(
	jobService job.JobService,
) *Router {
	validator := validation.New()

	return &Router{
		jobHandler: handler.NewJobHandler(jobService, validator),
	}
}

func (h *Router) SetupRoutes() http.Handler {
	r := chi.NewRouter()

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/jobs", h.jobHandler.Create)
	})

	return r
}
