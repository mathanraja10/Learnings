package http

import (
	"log"
	"net/http"
	"time"
)

func SlowHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	select {
	case <-time.After(10 * time.Second):
		log.Println("work completed")

	case <-ctx.Done():
		log.Println("request cancelled:", ctx.Err())
	}
}