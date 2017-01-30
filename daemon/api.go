package main

import (
	"goji.io"
	"goji.io/pat"
	"net/http"
	"encoding/json"
	"fmt"
)

//postDockerHostAPIHandler Handles the requests for adding a new docker host
func postDockerHostAPIHandler(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var dockerHost DockerInstance
	err := decoder.Decode(&dockerHost)
	if err != nil {
		panic(err)
	}

	//Call the connection methods
	addAndConnectToDockerInstance(database, &dockerHost)

	fmt.Fprintf(w, "OK")
	defer r.Body.Close()
}

//pingAPIHandler Handles the ping test endpoint
func pingAPIHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "PONG")
}

func startAPI() {
	mux := goji.NewMux()
	mux.HandleFunc(pat.Post("/api/v1/hosts"), postDockerHostAPIHandler)
	mux.HandleFunc(pat.Get("/api/v1/ping"), pingAPIHandler)
	log.Info("Starting API Mux...")
	log.Fatal(http.ListenAndServe(":8080", mux))
}