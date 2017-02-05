package main

import (
	"goji.io"
	"goji.io/pat"
	"net/http"
	"encoding/json"
	"fmt"
)

func postSpaceAPIHandler(w http.ResponseWriter, r *http.Request) {
	var requestingUser User

	var spaceRequest Space
	jsonDecoder := json.NewDecoder(r.Body)
	err := jsonDecoder.Decode(spaceRequest)
	//Ensure the request is valid JSON
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Invalid Request: Error Decoding JSON")
		return
	}

	//Let's copy the data we want. Excludes anything that does not belong.
	var createdSpace Space
	createdSpace.ImageID = spaceRequest.ImageID
	createdSpace.SSHKeyID = spaceRequest.SSHKeyID
	createdSpace.FriendlyName = spaceRequest.FriendlyName
	createdSpace.OwnerID = requestingUser.UserID

	log.Infof("Got Space Creation Request from %s\n", requestingUser.UserID)
	//Check Quota
	isUnderQuota := checkQuotaRestrictions(requestingUser.UserID)
	if !isUnderQuota {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, "Quota Exceeded")
		log.Warningf("Request from %s because of quota restrictions\n", requestingUser.UserID)
		return
	}

	go startSpace(database, createdSpace)

	fmt.Fprint(w, "Space creation started")
}

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
	mux.HandleFunc(pat.Post("/api/v1/hosts"), postSpaceAPIHandler)
	mux.HandleFunc(pat.Post("/api/v1/hosts"), postDockerHostAPIHandler)
	mux.HandleFunc(pat.Get("/api/v1/ping"), pingAPIHandler)
	log.Info("Starting API Mux...")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

//checkQuotaRestrictions Returns true if the user has not yet hit their quota on Spaces
func checkQuotaRestrictions(userID string) bool {
	//TODO: This should do the thing
	return true
}