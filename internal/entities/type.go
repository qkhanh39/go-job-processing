package entities

type JobType string

const (
	JobTypeSendEmail JobType = "SEND_EMAIL"
)

func (s JobType) IsValid() bool {
	switch s {
	case JobTypeSendEmail:
		return true
	default:
		return false
	}
}
