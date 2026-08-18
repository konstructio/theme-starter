// theme-starter — the smallest possible theme theme.
// Serves the static frontend; all theme logic lives in the browser.
//
// Assets are embedded in the binary (go:embed) because cloud-native
// buildpacks strip source files from the final image — a bare
// http.Dir("static") would 404 in production.
package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"
)

//go:embed static
var assets embed.FS

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	static, err := fs.Sub(assets, "static")
	if err != nil {
		log.Fatal(err)
	}
	http.Handle("/", requestLog(http.FileServer(http.FS(static))))
	log.Printf("theme-starter serving on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// requestLog gives the platform's runtime-logs stream something true to
// show: one line per request served. Without it the pod is silent and the
// logs panel in every theme viewer reads as broken.
func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Millisecond))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}
