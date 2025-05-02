package rest

import (
	"encoding/json"
	"fmt"

	"github.com/go-resty/resty/v2"
)

type ErrorResponse struct {
	Error  string              `json:"error"`
	Errors map[string][]string `json:"errors"`
}

func (e *ErrorResponse) IsFrameworkError() bool {
	return e.Error != ""
}
func (e *ErrorResponse) IsValidationError() bool {
	return len(e.Errors) > 0
}

func ParseJSONErrorResponse(resp *resty.Response) (ErrorResponse, error) {
	var errResp ErrorResponse
	if err := json.Unmarshal(resp.Body(), &errResp); err != nil {
		return errResp, fmt.Errorf("failed to parse error response: %w", err)
	}
	return errResp, nil
}
