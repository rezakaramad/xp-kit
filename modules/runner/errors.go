package runner

import (
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/response"

	"github.com/crossplane/function-sdk-go/errors"
)

// Fatal marks the function response as fatally failed and returns it.
func Fatal(rsp *fnv1.RunFunctionResponse, msg string, err error) (*fnv1.RunFunctionResponse, error) {
	wrapped := errors.Wrap(err, msg)
	if err == nil {
		wrapped = errors.New(msg)
	}

	response.ConditionFalse(rsp, "FunctionSuccess", "InternalError").
		WithMessage(wrapped.Error()).
		TargetCompositeAndClaim()
	response.Fatal(rsp, wrapped)

	return rsp, nil
}

// ConditionError is an error that carries a custom condition reason.
// When a Compose method returns a ConditionError, the runner uses its Reason
// as the condition reason instead of the default "CompositionError".
//
// Use NewConditionError to create one:
//
//	return nil, runner.NewConditionError("WaitingForPrincipalObjectID", "principal not yet available")
type ConditionError struct {
	reason  string
	message string
}

// NewConditionError creates a ConditionError with the given reason and message.
func NewConditionError(reason, message string) *ConditionError {
	return &ConditionError{reason: reason, message: message}
}

func (e *ConditionError) Error() string  { return e.message }
func (e *ConditionError) Reason() string { return e.reason }

// conditionReason extracts the condition reason from an error.
// Returns "CompositionError" for plain errors, or the custom reason for ConditionError.
func conditionReason(err error) string {
	var ce *ConditionError
	if errors.As(err, &ce) {
		return ce.Reason()
	}
	return "CompositionError"
}
