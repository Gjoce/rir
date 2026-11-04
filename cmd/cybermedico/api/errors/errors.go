package errors

import (
	"bytes"
	"context"
	"cybermedico/cmd/cybermedico/api/log"
	"cybermedico/layouts"
	"errors"
	"log/slog"
	"net/http"
)

var ErrNoop = errors.New("noop-error")

type HtmlError struct {
	Code       int
	Msg        string
	Err        error
	ButtonText string
	ButtonLink string
}

func (e HtmlError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}

	return e.Msg
}

func ErrorHandler(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, ErrNoop) {
		return
	}

	title := "Something went wrong"
	body := "An unexpected error occurred. Please try again later."
	code := http.StatusInternalServerError
	buttonText := "Go back"
	buttonLink := "/"

	var htmlErr HtmlError
	if errors.As(err, &htmlErr) {
		if htmlErr.Code != 0 {
			code = htmlErr.Code
			title = http.StatusText(htmlErr.Code)
		}

		if htmlErr.Msg != "" {
			body = htmlErr.Msg
		}

		if htmlErr.ButtonText != "" {
			buttonText = htmlErr.ButtonText
		}

		if htmlErr.ButtonLink != "" {
			buttonLink = htmlErr.ButtonLink
		}
	}

	level := slog.LevelError
	if code < http.StatusInternalServerError {
		level = slog.LevelWarn
	}

	log.Logger.Log(
		ctx,
		level,
		body,
		slog.Group("request",
			slog.String("path", r.URL.Path),
			slog.String("method", r.Method),
		),
		slog.String("error", err.Error()),
		slog.Int("status_code", code),
	)

	var buff bytes.Buffer
	if err = layouts.Error(title, body, buttonText, buttonLink).Render(ctx, &buff); err != nil {
		log.Logger.Error("failed to render error page", "error", err)
		http.Error(w, body, code)

		return
	}

	w.WriteHeader(code)
	if _, err = w.Write(buff.Bytes()); err != nil {
		log.Logger.Error("failed to write error response", "error", err)
	}
}

func NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	var buff bytes.Buffer
	if err := layouts.Error("Page Not Found", "The page you are looking for does not exist.", "Go back", "/").Render(r.Context(), &buff); err != nil {
		log.Logger.Error("failed to render not found page", "error", err)
		http.Error(w, "Not found", http.StatusNotFound)

		return
	}

	w.WriteHeader(http.StatusNotFound)
	if _, err := w.Write(buff.Bytes()); err != nil {
		log.Logger.Error("failed to write not found response", "error", err)
	}
}
