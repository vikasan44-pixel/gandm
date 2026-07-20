package middleware

import (
	"mime"
	"net/http"
)

// LimitJSONBody bounds JSON requests before handlers decode them. Multipart
// uploads keep their larger, handler-specific limit.
func LimitJSONBody(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if mediaType == "application/json" && r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}
