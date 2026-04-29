package gascity

import "fmt"

// APIError preserves GasCity problem details and request correlation metadata.
type APIError struct {
	Method     string
	Path       string
	StatusCode int
	RequestID  string
	Problem    *ProblemDetails
}

func (e *APIError) Error() string {
	statusCode := e.StatusCode
	if statusCode == 0 && e.Problem != nil {
		statusCode = e.Problem.Status
	}

	message := fmt.Sprintf("gascity %s %s: unexpected status %d", e.Method, e.Path, statusCode)
	if e.Problem != nil {
		switch {
		case e.Problem.Detail != "":
			message = fmt.Sprintf("%s: %s", message, e.Problem.Detail)
		case e.Problem.Title != "":
			message = fmt.Sprintf("%s: %s", message, e.Problem.Title)
		}
	}
	if e.RequestID != "" {
		message = fmt.Sprintf("%s (request_id=%s)", message, e.RequestID)
	}
	return message
}
