package utils

import (
	"log"
	"net/http"
	"time"
)

type WrappedHandlerFunc func(http.ResponseWriter, *http.Request) int

func RequestLoggerWrapper(at string, handler WrappedHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestDurationTracker := time.Now()
		source := r.RemoteAddr
		var statusCode int
		statusCode = handler(w, r)

		log.Printf("Finished %s %s for %s with %d in %s\n",
			r.Method, at, source, statusCode, time.Since(requestDurationTracker))
	}
}
