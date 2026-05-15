package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type Meta struct {
	Page  int `json:"page,omitempty"`
	Limit int `json:"limit,omitempty"`
	Total int `json:"total,omitempty"`
}

type Envelope struct {
	Success bool  `json:"success"`
	Data    any   `json:"data,omitempty"`
	Error   any   `json:"error,omitempty"`
	Meta    *Meta `json:"meta,omitempty"`
}

type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  any    `json:"detail,omitempty"`
}

func WriteJSON(w http.ResponseWriter, status int, payload Envelope) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func Success(w http.ResponseWriter, status int, data any, meta *Meta) {
	WriteJSON(w, status, Envelope{Success: true, Data: data, Meta: meta})
}

func Failure(w http.ResponseWriter, status int, code, message string, detail any) {
	WriteJSON(w, status, Envelope{Success: false, Error: ErrorPayload{Code: code, Message: message, Detail: detail}})
}

func DecodeJSON(r *http.Request, dst any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	return nil
}

func QueryInt(values map[string][]string, key string, fallback int) int {
	if raw := first(values[key]); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			return parsed
		}
	}
	return fallback
}

func first(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func BearerToken(r *http.Request) (string, error) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return "", errors.New("missing authorization header")
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", errors.New("invalid authorization header")
	}
	return strings.TrimSpace(parts[1]), nil
}
