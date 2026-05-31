// Command server levanta el backend del simulador: carga el dataset, arma el
// servidor WebSocket y escucha conexiones.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gperelstein/pokemon-battle-simulator-backend/internal/dex"
	"github.com/gperelstein/pokemon-battle-simulator-backend/internal/transport/ws"
)

func main() {
	dataPath := envOr("DATA_PATH", "data")
	addr := envOr("ADDR", ":8080")

	d, err := dex.Load(dataPath)
	if err != nil {
		log.Fatalf("cargando dataset desde %q: %v", dataPath, err)
	}

	srv := ws.New(d)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", srv.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	log.Printf("escuchando en %s (dataset: %s)", addr, dataPath)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("servidor: %v", err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
