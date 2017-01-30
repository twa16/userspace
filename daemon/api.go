package main

import (
	"goji.io"
	"goji.io/pat"
	"net/http"
	"encoding/json"
	"fmt"
)

func postDockerHostAPIHandler(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var dockerHost DockerInstance
	err := decoder.Decode(&dockerHost)
	if err != nil {
		panic(err)
	}

	fmt.Fprintf(w, "OK")
	defer r.Body.Close()
}

func startAPI() {
	mux := goji.NewMux()
	mux.HandleFunc(pat.Post("/api/v1/hosts"), postDockerHostAPIHandler)
	log.Info("Starting API Mux...")
	log.Fatal(http.ListenAndServe(":8080", mux))
}