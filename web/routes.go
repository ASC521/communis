package web

import (
	"log/slog"
	"net/http"
)

func addRoutes(mux *http.ServeMux, logger *slog.Logger) {

	baseChain := chain{RequestLogger([]string{}, logger)}

	mux.Handle("/", baseChain.thenFunc(func(w http.ResponseWriter, r *http.Request) {
		html := "<h1>Hello World!</h1>"
		w.Write([]byte(html))
	}))
}
