package main

import (
	"log"
	"net/http"
)

type statusRecorder struct {
	http.ResponseWriter
	status    int
	byteCount int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.status = code
	rec.ResponseWriter.WriteHeader(code)
}

func (rec *statusRecorder) Write(p []byte) (int, error) {
	bc, err := rec.ResponseWriter.Write(p)
	rec.byteCount += bc

	return bc, err
}

func loggingHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		rec := statusRecorder{w, 200, 0}
		next.ServeHTTP(&rec, req)
		remoteAddr := req.Header.Get("X-Forwarded-For")
		if remoteAddr == "" {
			remoteAddr = req.RemoteAddr
		}
		ua := req.Header.Get("User-Agent")

		log.Printf("%s - \"%s %s %s\" (%s) %d %d \"%s\"", remoteAddr, req.Method, req.URL.Path, req.Proto, req.Host, rec.status, rec.byteCount, ua)
	})
}
