package main

import (
	"path"
	"encoding/json"
	"html/template"
	"github.com/tbruyelle/hipchat-go/hipchat"
	"github.com/jasonlvhit/gocron"
	"github.com/gorilla/mux"
	"net/http"
	"log"
	"strconv"
	"./util"
)

func (c *Context) Routes() *mux.Router {
	r := mux.NewRouter()

	gocron.Every(1).Second().Do(func () {
		log.Printf("Hello")
	})

	gocron.Start()

	// healthcheck required by Micros
	r.Path("/healthcheck").Methods("GET").HandlerFunc(c.healthcheck)

	// Descriptor for atlassian connect
	r.Path("/").Methods("GET").HandlerFunc(c.atlassianConnect)
	r.Path("/atlassian-connect.json").Methods("GET").HandlerFunc(c.atlassianConnect)

	// HipChat specific API routes
	r.Path("/installable").Methods("POST").HandlerFunc(c.installable)
	r.Path("/config").Methods("GET").HandlerFunc(c.config)
	r.Path("/hook").Methods("POST").HandlerFunc(c.hook)

	r.PathPrefix("/").Handler(http.FileServer(http.Dir(c.static)))

	return r
}

func (c *Context) healthcheck(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode([]string{"OK"})
}

func (c *Context) atlassianConnect(w http.ResponseWriter, r *http.Request) {
	lp := path.Join("./static", "atlassian-connect.json")
	vals := map[string]string{
		"LocalBaseUrl": c.baseURL,
	}
	tmpl, err := template.ParseFiles(lp)
	if err != nil {
		log.Fatalf("%v", err)
	}
	tmpl.ExecuteTemplate(w, "config", vals)
}

func (c *Context) installable(w http.ResponseWriter, r *http.Request) {
	authPayload, err := util.DecodePostJSON(r, true)
	if err != nil {
		log.Fatalf("Parsed auth data failed:%v")
	}

	credentials := hipchat.ClientCredentials{
		ClientID: authPayload["oauthId"].(string),
		ClientSecret: authPayload["oauthSecret"].(string),
	}
	roomName := strconv.Itoa(int(authPayload["roomId"].(float64)))
	newClient := hipchat.NewClient("")
	tok, _, err := newClient.GenerateToken(credentials, []string{hipchat.ScopeSendNotification})
	if err != nil {
		log.Fatalf("Client.GetAccessToken returns an error %v", err)
	}
	rc := &RoomConfig{
		name: roomName,
		hc: tok.CreateClient(),
	}
	c.rooms[roomName] = rc
	util.PrintDump(w, r, false)
	json.NewEncoder(w).Encode([]string{"OK"})
}

func (c *Context) config(w http.ResponseWriter, r *http.Request) {
	signedRequest := r.URL.Query().Get("signed_request")
	lp := path.Join("./static", "layout.hbs")
	fp := path.Join("./static", "config.hbs")
	vals := map[string]string{
		"LocalBaseUrl": c.baseURL,
		"SignedRequest": signedRequest,
		"HostScriptUrl": c.baseURL,
	}
	tmpl, err := template.ParseFiles(lp, fp)
	if err != nil {
		log.Fatalf("%v", err)
	}
	tmpl.ExecuteTemplate(w, "layout", vals)
}

func (c *Context) hook(w http.ResponseWriter, r *http.Request) {
	payLoad, err := util.DecodePostJSON(r, true)
	if err != nil {
		log.Fatalf("Parsed auth data failed:%v\n", err)
	}

	roomIDNumber := int((payLoad["item"].(util.PayloadItem))["room"].(util.PayloadItem)["id"].(float64))
	roomID := strconv.Itoa(roomIDNumber)

	util.PrintDump(w, r, true)

	log.Printf("Sending notification to %s\n", roomID)
	notifRq := &hipchat.NotificationRequest{
		Message: "Let's join <strong>video call</strong>: <a href=\"https://hipchat.me/testorumo\">Testorumo</a>!",
		MessageFormat: "html",
		Color: "red",
	}

	if _, ok := c.rooms[roomID]; ok {
		_, err = c.rooms[roomID].hc.Room.Notification(roomID, notifRq)
		if err != nil {
			log.Printf("Failed to notify HipChat channel:%v\n", err)
		}
	} else {
		log.Printf("Room is not registered correctly:%v\n", c.rooms)
	}
}