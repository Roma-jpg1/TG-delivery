package middleware

import "net/http"

const adminTokenHeader = "X-Admin-Token"

func RequireAdminToken(expectedToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if expectedToken == "" {
				next.ServeHTTP(w, r)
				return
			}
			if r.Header.Get(adminTokenHeader) != expectedToken {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte("{\"error\":\"unauthorized\"}"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
