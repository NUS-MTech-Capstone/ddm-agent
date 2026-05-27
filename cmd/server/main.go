package main

import (
	"log"
	"net/http"

	"github.com/NUS-MTech-Capstone/ddm-agent/internal/config"
	"github.com/NUS-MTech-Capstone/ddm-agent/internal/httpapi"
)

func main() {
	cfg := config.Load()
	addr := ":" + cfg.Port
	log.Printf("ddm-agent listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, httpapi.New()))
}
