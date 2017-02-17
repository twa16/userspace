/*
 * Copyright 2017 Manuel Gauto (github.com/twa16)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
*/

package userspaced

import (
	"goji.io"
	"goji.io/pat"
	"net/http"
	"encoding/json"
	"fmt"
	auth "github.com/twa16/go-auth"
	"errors"
	"github.com/spf13/viper"
)

const (
	ADMIN_ADD_HOST = "admin.host.add"
	ADMIN_READ_HOST = "admin.host.read"
	ADMIN_UPDATE_HOST = "admin.host.update"
	ADMIN_DELETE_HOST = "admin.host.delete"
)


func getUserFromRequest(r *http.Request) (*auth.User, error){
	r.ParseForm()
	if len(r.Header["X-Auth-Token"]) == 0 {
		return nil, errors.New("No Token Provided")
	}
	token := r.Header["X-Auth-Token"][0]
	sess, err := authProvider.CheckSessionKey(token)
	if err != nil {
		log.Critical("Error Checking Session: "+err.Error())
		return nil, errors.New("Internal Server Error")
	}
	if sess.IsExpired {
		return nil, errors.New("Session Expired")
	}
	user, err := authProvider.GetUserByID(sess.AuthSession.AuthUserID)
	if err != nil {
		log.Critical("Error Checking Session: "+err.Error())
		return nil, errors.New("Internal Server Error")
	}
	return &user, nil
}

func getOrchestratorInfoAPIHandler(w http.ResponseWriter, r *http.Request) {
	//It is probably faster to do this just once. We will cross that bridge when we get there
	//I honestly forgot I could initialize structs like this.
	orcInfo := OrchestratorInfo{
		SupportsCAS: viper.GetBool("SupportCAS"),
		CASURL: viper.GetString("CASURL"),
		AllowsRegistration: viper.GetBool("AllowRegistration"),
		AllowsLocalLogin: viper.GetBool("AllowLocalLogin"),
	}
	jsonBytes, _ := json.Marshal(orcInfo)
	fmt.Fprint(w, string(jsonBytes))
}

func postSpaceAPIHandler(w http.ResponseWriter, r *http.Request) {
	user, err := getUserFromRequest(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, err.Error())
	}

	var spaceRequest Space
	jsonDecoder := json.NewDecoder(r.Body)
	err = jsonDecoder.Decode(&spaceRequest)
	//Ensure the request is valid JSON
	if err != nil {
		log.Debug(err)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Invalid Request: Error Decoding JSON")
		return
	}

	//Let's copy the data we want. Excludes anything that does not belong.
	var createdSpace Space
	createdSpace.ImageID = spaceRequest.ImageID
	createdSpace.SSHKeyID = spaceRequest.SSHKeyID
	createdSpace.FriendlyName = spaceRequest.FriendlyName
	createdSpace.OwnerID = user.Username

	log.Infof("Got Space Creation Request from %s\n", user.Username)
	//Check Quota
	isUnderQuota := checkQuotaRestrictions(user.Username)
	if !isUnderQuota {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, "Quota Exceeded")
		log.Warningf("Request from %s because of quota restrictions\n", user.Username)
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

	fmt.Fprint(w, "OK")
	defer r.Body.Close()
}

//pingAPIHandler Handles the ping test endpoint
func pingAPIHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "PONG")
}

//getImagesAPIHandler Get images
func getImagesAPIHandler(w http.ResponseWriter, r *http.Request) {
	images := []SpaceImage{}
	database.Find(&images)
	jsonBytes, _ := json.Marshal(images)
	fmt.Fprint(w, string(jsonBytes))
}

func getCASHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	fmt.Println(r.FormValue("ticket"))
}

var authProvider auth.AuthProvider
func startAPI() {
	authProvider.Database = database
	authProvider.SessionExpireTimeSeconds = 60 * 30
	authProvider.Startup()

	mux := goji.NewMux()
	mux.HandleFunc(pat.Post("/api/v1/spaces"), postSpaceAPIHandler)
	mux.HandleFunc(pat.Post("/api/v1/hosts"), postDockerHostAPIHandler)
	mux.HandleFunc(pat.Get("/api/v1/images"), getImagesAPIHandler)
	mux.HandleFunc(pat.Get("/api/v1/ping"), pingAPIHandler)
	mux.HandleFunc(pat.Get("/caslogin"), getCASHandler)
	mux.HandleFunc(pat.Get("/orchestratorinfo"), getOrchestratorInfoAPIHandler)
	log.Info("Starting API Mux...")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

//checkQuotaRestrictions Returns true if the user has not yet hit their quota on Spaces
func checkQuotaRestrictions(userID string) bool {
	//TODO: This should do the thing
	return true
}