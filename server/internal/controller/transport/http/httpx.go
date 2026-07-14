package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"example.com/project-template/internal/controller/application/apperror"
)

type problem struct {
	Type      string         `json:"type"`
	Title     string         `json:"title"`
	Status    int            `json:"status"`
	Code      string         `json:"code"`
	Detail    string         `json:"detail,omitempty"`
	RequestID string         `json:"requestId,omitempty"`
	Errors    []problemField `json:"errors,omitempty"`
}

type problemField struct {
	Location string `json:"location"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return apperror.Malformed("request body is invalid", apperror.Field{Name: "body", Code: "INVALID_JSON", Message: jsonErrorDetail(err)})
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return apperror.Malformed("request body must contain one JSON object", apperror.Field{Name: "body", Code: "INVALID_JSON", Message: "must contain one JSON object"})
	}
	return nil
}

func jsonErrorDetail(err error) string {
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		return "contains malformed JSON"
	}
	if errors.Is(err, io.EOF) {
		return "must not be empty"
	}
	return "does not match the expected shape"
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	code := "INTERNAL_ERROR"
	title := http.StatusText(status)
	detail := "an unexpected error occurred"
	var fields []problemField

	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		code = appErr.Code
		detail = appErr.Message
		switch appErr.Kind {
		case apperror.KindMalformed:
			status = http.StatusBadRequest
		case apperror.KindInvalid:
			status = http.StatusUnprocessableEntity
		case apperror.KindUnauthorized:
			status = http.StatusUnauthorized
		case apperror.KindForbidden:
			status = http.StatusForbidden
		case apperror.KindNotFound:
			status = http.StatusNotFound
		case apperror.KindConflict:
			status = http.StatusConflict
		case apperror.KindUnavailable:
			status = http.StatusServiceUnavailable
		case apperror.KindMethod:
			status = http.StatusMethodNotAllowed
		}
		title = http.StatusText(status)
		fields = make([]problemField, 0, len(appErr.Fields))
		for _, field := range appErr.Fields {
			location := field.Name
			if location != "body" && location != "query" && location != "path" && !strings.Contains(location, ".") {
				location = "body." + location
			}
			fields = append(fields, problemField{Location: location, Code: field.Code, Message: field.Message})
		}
	}

	requestID := chimiddleware.GetReqID(r.Context())
	if requestID != "" {
		w.Header().Set("X-Request-ID", requestID)
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem{
		Type: "https://example.com/problems/" + code, Title: title, Status: status,
		Code: code, Detail: detail, RequestID: requestID, Errors: fields,
	})
}

func methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, apperror.MethodNotAllowed(fmt.Sprintf("method %s is not allowed", r.Method)))
}
