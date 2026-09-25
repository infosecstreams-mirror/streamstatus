package main

import (
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/nicklaw5/helix/v2"
	"github.com/nikoksr/notify"
	"github.com/nikoksr/notify/service/pushbullet"
	log "github.com/sirupsen/logrus"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "unknown"

// main do the work.
func main() {
	buildInfo, _ := debug.ReadBuildInfo()
	log.Printf("started streamstatus %s", version)
	fmt.Printf("%s\n", buildInfo.String())

	// Setup file and repo paths.
	var repoUrl string
	if len(os.Getenv("SS_GH_REPO")) == 0 {
		log.Info("no SS_GH_REPO specified in environment, defaulting to: https://github.com/infosecstreams-mirror/infosecstreams-mirror.github.io")
		repoUrl = "https://github.com/infosecstreams-mirror/infosecstreams-mirror.github.io"
	} else {
		repoUrl = os.Getenv("SS_GH_REPO")
	}
	parts := strings.Split(strings.TrimSuffix(repoUrl, "/"), "/")
	owner := parts[len(parts)-2]
	repoPath := parts[len(parts)-1]

	// Setup auth.
	if len(os.Getenv("SS_TOKEN")) == 0 || len(os.Getenv("SS_SECRETKEY")) == 0 {
		log.Fatalln("error: no SS_TOKEN and/or SS_SECRETKEY specified in environment!")
	}
	ghToken := os.Getenv("SS_TOKEN")

	if len(os.Getenv("TW_CLIENT_ID")) == 0 || len(os.Getenv("TW_CLIENT_SECRET")) == 0 {
		log.Fatalln("error: no TW_CLIENT_ID and/or TW_CLIENT_SECRET specified in environment! https://dev.twitch.tv/console/app")
	}

	client, err := helix.NewClient(&helix.Options{
		ClientID:     os.Getenv("TW_CLIENT_ID"),
		ClientSecret: os.Getenv("TW_CLIENT_SECRET"),
	})
	if err != nil {
		log.Fatalln(err)
		return
	}

	access_token, err := client.RequestAppAccessToken([]string{})
	if err != nil {
		log.Fatalln(err)
		return
	}
	client.SetAppAccessToken(access_token.Data.AccessToken)

	// Setup notifications
	if len(os.Getenv("SS_PUSHBULLET_APIKEY")) == 0 || len(os.Getenv("SS_PUSHBULLET_DEVICES")) == 0 {
		log.Fatalln("error: no SS_PUSHBULLET_APIKEY and/or SS_PUSHBULLET_DEVICES specified in environment! https://www.pushbullet.com/#settings/account")
	}
	notifier := notify.New()
	pushbullet := pushbullet.New(os.Getenv("SS_PUSHBULLET_APIKEY"))
	for _, device := range strings.Split(os.Getenv("SS_PUSHBULLET_DEVICES"), ",") {
		pushbullet.AddReceivers(device)
	}
	notifier.UseServices(pushbullet)

	// Create StreamersRepo object
	var repo = StreamersRepo{
		GitHubToken:        ghToken,
		Owner:              owner,
		Repo:               repoPath,
		StatusMap:          make(map[string]StreamerState),
		client:             client,
		notificationClient: notifier,
		mutex:              &sync.Mutex{},
	}
	
	err = repo.fetchRemoteStatus()
	if err != nil {
		log.Warnf("failed to fetch initial remote status.json (might not exist yet): %v", err)
	} else {
		log.Infof("Successfully fetched initial status.json with %d streamers.", len(repo.StatusMap))
	}

	port := ":8080"
	// Google Cloud Run defaults to 8080. Their platform
	// sets the $PORT ENV var if you override it with, e.g.:
	// `gcloud run services update <service-name> --port <port>`.
	if os.Getenv("PORT") != "" {
		port = ":" + os.Getenv("PORT")
	} else if os.Getenv("SS_PORT") != "" {
		port = ":" + os.Getenv("SS_PORT")
	}

	// Listen and serve.
	log.Printf("server starting on %s", port)
	http.HandleFunc("/webhook/callbacks", repo.eventsubStatus)
	log.Fatal(http.ListenAndServe(port, nil))
}
