package api

import (
	"encoding/json"
	"fmt"
	"strings"
)

type APIError struct {
	StatusCode int
	Code       int
	Subcode    int
	Message    string
	Type       string
	Details    string
	TraceID    string
	Body       string
}

func (err *APIError) Error() string {
	message := err.Message
	if message == "" {
		message = strings.TrimSpace(err.Body)
	}
	if message == "" {
		message = "request failed"
	}
	trace := ""
	if err.TraceID != "" {
		trace = "; fbtrace_id: " + err.TraceID
	}
	return fmt.Sprintf("Graph API error (%d/%d): %s%s; hint: %s", err.StatusCode, err.Code, message, trace, err.Hint())
}

func (err *APIError) Hint() string {
	switch {
	case err.Code == 190 || err.StatusCode == 401:
		return "run meta auth login and verify the token has not expired"
	case err.Code == 10 || err.Code == 200 || err.StatusCode == 403:
		return "check the app review status and permissions for this account"
	case err.StatusCode == 404:
		return "verify the object id with the corresponding list command"
	case isThrottleCode(err.Code) || err.StatusCode == 429:
		return "rate limited; reduce request volume and retry after the indicated delay"
	case err.StatusCode >= 500:
		return "server error; the failure is usually transient, so retry later"
	default:
		return "rerun with --verbose and inspect permissions and request fields"
	}
}

type graphErrorEnvelope struct {
	Error struct {
		Message     string `json:"message"`
		Type        string `json:"type"`
		Code        int    `json:"code"`
		Subcode     int    `json:"error_subcode"`
		UserTitle   string `json:"error_user_title"`
		UserMessage string `json:"error_user_msg"`
		TraceID     string `json:"fbtrace_id"`
	} `json:"error"`
}

func decodeAPIError(status int, body []byte) error {
	var envelope graphErrorEnvelope
	_ = json.Unmarshal(body, &envelope)
	details := strings.TrimSpace(strings.Join([]string{envelope.Error.UserTitle, envelope.Error.UserMessage}, ": "))
	details = strings.Trim(details, ": ")
	return &APIError{
		StatusCode: status,
		Code:       envelope.Error.Code,
		Subcode:    envelope.Error.Subcode,
		Message:    envelope.Error.Message,
		Type:       envelope.Error.Type,
		Details:    details,
		TraceID:    envelope.Error.TraceID,
		Body:       truncate(string(body), 4096),
	}
}

func isThrottleCode(code int) bool {
	return code == 4 || code == 17 || code == 32 || code == 613 ||
		(code >= 80000 && code <= 80009) || code == 80014
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max] + "…"
}
