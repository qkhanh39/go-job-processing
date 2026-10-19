package handler

import (
	"errors"
	"net/http"

	"github.com/qkhanh39/go-common/api"
	"github.com/qkhanh39/go-common/apperrors"
	"github.com/qkhanh39/go-common/validation"
	"github.com/qkhanh39/go-job-processing/internal/adapter/http/dto"
	"github.com/qkhanh39/go-job-processing/internal/entities"
	"github.com/qkhanh39/go-job-processing/internal/service/job"
)

type JobHandler struct {
	jobService job.JobService
	validator  *validation.Validator
}

func NewJobHandler(
	jobService job.JobService,
	validator *validation.Validator,
) *JobHandler {
	return &JobHandler{
		jobService: jobService,
		validator:  validator,
	}
}

func (h *JobHandler) Create(w http.ResponseWriter, r *http.Request) {
	var jobReq dto.CreateJobRequest
	if err := api.NewDecoder(r).Body(&jobReq).Err(); err != nil {
		api.Response(w, r).WithError(apperrors.BadRequestError{Issue: entities.ErrInvalidJson.Error()}).Send()
		return
	}

	if err := h.validator.Validate(jobReq); err != nil {
		api.Response(w, r).WithError(err).Send()
		return
	}

	job, err := h.jobService.Create(r.Context(), jobReq.ToInput())
	if err != nil {
		switch {
		case errors.Is(err, entities.ErrInvalidPayload):
			api.Response(w, r).WithError(apperrors.BadRequestError{Issue: err.Error()}).Send()
		case errors.Is(err, entities.ErrUnsupportedJobType):
			api.Response(w, r).WithError(apperrors.BadRequestError{Issue: err.Error()}).Send()
		default:
			api.Response(w, r).WithError(apperrors.InternalError{Issue: "failed to create job"}).Send()
		}
		return
	}

	response := dto.ToCreateJobResponse(job)
	api.Response(w, r).WithData(response).Status(http.StatusCreated).Send()
}
