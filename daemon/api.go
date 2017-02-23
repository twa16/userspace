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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/spf13/viper"
	auth "github.com/twa16/go-auth"
	"goji.io"
	"goji.io/pat"
	"net/http"
	"os"
	"os/signal"
	"time"
)

const (
	ADMIN_ADD_HOST    = "admin.host.add"
	ADMIN_READ_HOST   = "admin.host.read"
	ADMIN_UPDATE_HOST = "admin.host.update"
	ADMIN_DELETE_HOST = "admin.host.delete"
)

//getUserFromRequest Gets user from the X-Auth-Token that should be sent with all requests.
func getUserFromRequest(r *http.Request) (*auth.User, error) {
	r.ParseForm()
	if len(r.Header["X-Auth-Token"]) == 0 {
		return nil, errors.New("No Token Provided")
	}
	token := r.Header["X-Auth-Token"][0]
	sess, err := authProvider.CheckSessionKey(token)
	if err != nil {
		log.Critical("Error Checking Session: " + err.Error())
		return nil, errors.New("Internal Server Error")
	}
	if sess.AuthSession == nil {
		return nil, errors.New("Session does not exist")
	}
	if sess.IsExpired {
		return nil, errors.New("Session Expired")
	}
	user, err := authProvider.GetUserByID(sess.AuthSession.AuthUserID)
	if err != nil {
		log.Critical("Error Checking Session: " + err.Error())
		return nil, errors.New("Internal Server Error")
	}
	return &user, nil
}

//getOrchestratorInfoAPIHandler Returns OrchestratorInfo to clients
func getOrchestratorInfoAPIHandler(w http.ResponseWriter, r *http.Request) {
	//It is probably faster to do this just once. We will cross that bridge when we get there
	//I honestly forgot I could initialize structs like this.
	orcInfo := OrchestratorInfo{
		SupportsCAS:        viper.GetBool("SupportsCAS"),
		CASURL:             viper.GetString("CASURL"),
		AllowsRegistration: viper.GetBool("AllowRegistration"),
		AllowsLocalLogin:   viper.GetBool("AllowLocalLogin"),
	}
	jsonBytes, _ := json.Marshal(orcInfo)
	fmt.Fprint(w, string(jsonBytes))
}

//postSpaceAPIHandler Handles POST /api/v1/spaces - Called when a user wishes to create a space
func postSpaceAPIHandler(w http.ResponseWriter, r *http.Request) {
	//Get user and error out if it didn't work
	user, err := getUserFromRequest(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, err.Error())
		return
	}

	//Check permission
	hasPerm, err := authProvider.CheckPermission(user.ID, "user.space.create")
	if err != nil || !hasPerm {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, err.Error())
		return
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
	createdSpace.OwnerID = user.ID

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

//getSpacesAPIHandler Handles GET /api/v1/spaces -- Get lists of spaces
func getSpacesAPIHandler(w http.ResponseWriter, r *http.Request) {
	//Get user and error out if it didn't work
	user, err := getUserFromRequest(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, err.Error())
		return
	}
	//Get Spaces
	var spaces []Space
	err = database.Where("OwnerID", user.ID).Find(&spaces).Error
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err.Error())
		return
	}
	//Get Space Association
	spaces, err = GetSpaceArrayAssociation(database, spaces)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err.Error())
		return
	}
	//Marshal spaces to JSON
	jsonBytes, _ := json.Marshal(spaces)
	fmt.Fprint(w, string(jsonBytes))

}

//postDockerHostAPIHandler Handles the requests for adding a new docker host
func postDockerHostAPIHandler(w http.ResponseWriter, r *http.Request) {
	//Get user and error out if it didn't work
	user, err := getUserFromRequest(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, err.Error())
		return
	}

	//Check permission
	hasPerm, err := authProvider.CheckPermission(user.ID, ADMIN_ADD_HOST)
	if err != nil || !hasPerm {
		w.WriteHeader(http.StatusUnauthorized)
		if err != nil {
			fmt.Fprint(w, err.Error())
		} else {
			fmt.Fprint(w, "Unauthorized\n")
		}
		return
	}

	decoder := json.NewDecoder(r.Body)
	var dockerHost DockerInstance
	err = decoder.Decode(&dockerHost)
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

//getCASHandler Handles CAS tickets sent from clients
func getCASHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	ticket := r.FormValue("ticket")
	valResp, err := casServer.ValidateTicket(ticket)
	if err != nil {
		log.Warning("Error handling CAS login: " + err.Error())
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, "Error")
		return
	}
	if valResp.IsValid {
		log.Infof("Authenticated %s using CAS\n", valResp.Username)
		user, err := authProvider.GetUser(valResp.Username)
		if err != nil {
			//If the internal user does not exist, make it
			if err.Error() == "record not found" {
				var user auth.User
				user.Username = valResp.Username
				user.Permissions = []auth.Permission{
					{Permission: "user.*"},
				}
				authProvider.CreateUser(user)

			} else {
				log.Warning("Error getting user for CAS login: " + err.Error())
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprint(w, "Error")
				return
			}
		}
		//Generate the Session
		session, err := authProvider.GenerateSessionKey(user.ID, false)
		if err != nil {
			log.Critical("Error Generating Session: " + err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "Internal Server Error")
			return
		}
		//JSONify and send our response
		jsonBytes, _ := json.Marshal(session)
		fmt.Fprint(w, string(jsonBytes))
		return
	} else {
		fmt.Println("Invalid Ticket")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, "Error")
	}
}

var authProvider auth.AuthProvider

//startAPI Starts the API server and required services such as the AuthProvider
func startAPI() {
	//Start auth provider
	authProvider.Database = database
	authProvider.SessionExpireTimeSeconds = 60 * 30
	authProvider.Startup()

	//Ensure admin user exists
	ensureAdminUser()

	log.Info("Ensuring HTTPS Certificates Exist")
	apiKeyFile := viper.GetString("ApiHttpsKey")
	apiCertFile := viper.GetString("ApiHttpsCertificate")
	log.Debugf("Using Key File: %s\n", apiKeyFile)
	log.Debugf("Using Certificate File: %s\n", apiCertFile)
	//Check to see if cert exists
	_, errKey := os.Stat(apiKeyFile)
	_, errCert := os.Stat(apiCertFile)
	if os.IsNotExist(errKey) || os.IsNotExist(errCert) {
		log.Warning("Generating HTTPS Certificate for API")
		os.Remove(apiKeyFile)
		os.Remove(apiCertFile)
		privateKey, certificate, err := CreateSelfSignedCertificate(viper.GetString("ApiHost"))
		if err != nil {
			log.Fatalf("Error generating API Certificates: %s\n", err.Error())
			panic(err)
		}
		err = WriteCertificateToFile(certificate, apiCertFile)
		if err != nil {
			log.Fatalf("Error saving certificate: %s\n", err.Error())
			panic(err)
		}
		err = WritePrivateKeyToFile(privateKey, apiKeyFile)
		if err != nil {
			log.Fatalf("Error saving private key: %s\n", err.Error())
			panic(err)
		}
		log.Info("API Certificate Generation Complete.")
	}

	mux := goji.NewMux()
	mux.HandleFunc(pat.Post("/api/v1/spaces"), postSpaceAPIHandler)
	mux.HandleFunc(pat.Get("/api/v1/spaces"), getSpacesAPIHandler)
	mux.HandleFunc(pat.Post("/api/v1/hosts"), postDockerHostAPIHandler)
	mux.HandleFunc(pat.Get("/api/v1/images"), getImagesAPIHandler)
	mux.HandleFunc(pat.Get("/api/v1/ping"), pingAPIHandler)
	mux.HandleFunc(pat.Get("/caslogin"), getCASHandler)
	mux.HandleFunc(pat.Get("/orchestratorinfo"), getOrchestratorInfoAPIHandler)
	log.Info("Starting API Mux...")
	srv := &http.Server{Addr: ":8080", Handler: mux}
	srv.ListenAndServeTLS(apiCertFile, apiKeyFile)

	// subscribe to SIGINT signals
	stopChan := make(chan os.Signal)
	signal.Notify(stopChan, os.Interrupt)
	<-stopChan // wait for SIGINT
	log.Info("Shutting down server...")

	// shut down gracefully, but wait no longer than 5 seconds before halting
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	srv.Shutdown(ctx)

	log.Info("Server gracefully stopped")
}

func ensureAdminUser() {
	/*adminUser := auth.User{
		FirstName: "Default",
		LastName: "Administrator",
		Username: "admin",
		Permissions: []auth.Permission{
			{Permission: "*.*"},
		},
	}*/

}

//checkQuotaRestrictions Returns true if the user has not yet hit their quota on Spaces
func checkQuotaRestrictions(userID string) bool {
	//TODO: This should do the thing
	return true
}
