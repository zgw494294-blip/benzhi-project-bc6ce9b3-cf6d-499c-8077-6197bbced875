package httpapi

import (
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/domain"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
)

const maxBody = 1 << 20

type errorBody struct {
	Error struct {
		Code    domain.ErrorCode `json:"code"`
		Field   string           `json:"field,omitempty"`
		Message string           `json:"message"`
		Details any              `json:"details,omitempty"`
	} `json:"error"`
}

func decode(w http.ResponseWriter, r *http.Request, dst any) error {
	media, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || media != "application/json" {
		return domain.NewError(domain.CodeInvalid, "Content-Type", "请求必须使用 application/json")
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if e = dec.Decode(dst); e != nil {
		return domain.NewError(domain.CodeInvalid, "body", "JSON 请求体无效: "+e.Error())
	}
	if e = dec.Decode(&struct{}{}); e != io.EOF {
		return domain.NewError(domain.CodeInvalid, "body", "请求体只能包含一个 JSON 对象")
	}
	return nil
}
func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func fail(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := domain.ErrorCodeOf(err)
	field := ""
	message := "服务器内部错误"
	var b errorBody
	var de *domain.Error
	if errors.As(err, &de) {
		field = de.Field
		message = de.Message
		b.Error.Details = de.Details
		switch de.Code {
		case domain.CodeInvalid:
			status = 400
		case domain.CodeNotFound:
			status = 404
		case domain.CodeConflict, domain.CodeState:
			status = 409
		case domain.CodeForbidden:
			status = 403
		}
	}
	b.Error.Code = code
	if code == "" {
		b.Error.Code = "internal"
	}
	b.Error.Field = field
	b.Error.Message = message
	respond(w, status, b)
}
func principal(r *http.Request) domain.Principal {
	return domain.Principal{Name: strings.TrimSpace(r.Header.Get("X-Actor")), Role: domain.Role(strings.TrimSpace(r.Header.Get("X-Role")))}
}
func idem(r *http.Request) string { return strings.TrimSpace(r.Header.Get("Idempotency-Key")) }
