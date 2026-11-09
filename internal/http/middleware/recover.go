package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"

	api "github.com/verdex4/go-course-avito/internal/generated"
)

// Recover перехватывает паники, логирует их и возвращает application/problem+json.
func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					stack := debug.Stack()
					logger.Error("panic recovered",
						"error", err,
						"stack", string(stack),
						"method", r.Method,
						"path", r.URL.Path,
					)

					defer func() {
						_ = recover()
					}()

					w.Header().Set("Content-Type", "application/problem+json")
					w.WriteHeader(http.StatusInternalServerError)

					problem := api.Problem{
						Type:   "https://tripgo.example/problems/internal-error",
						Title:  "Internal Server Error",
						Status: http.StatusInternalServerError,
						Detail: new("Internal server error"),
						Code:   "internal_error",
					}

					_ = json.NewEncoder(w).Encode(problem)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
