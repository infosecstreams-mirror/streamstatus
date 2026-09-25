package main

import (
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
	// "strings"
	"sync"

	"github.com/nicklaw5/helix/v2"
	// "github.com/nikoksr/notify"
	// "github.com/nikoksr/notify/service/pushbullet"
	log "github.com/sirupsen/logrus"
)

var version = "unknown"

type App struct {
	store    Store
	client   *helix.Client
	// notifier *notify.Notify
	mutex    *sync.Mutex
}

func main() {
	buildInfo, _ := debug.ReadBuildInfo()
	log.Printf("started streamstatus %s", version)
	fmt.Printf("%s\n", buildInfo.String())

	// Setup Database
	if len(os.Getenv("DATABASE_URL")) == 0 {
		log.Fatalln("error: no DATABASE_URL specified in environment!")
	}
	store, err := NewPostgresStore(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("failed to init database: %v", err)
	}

	// Setup Twitch Client
	if len(os.Getenv("TW_CLIENT_ID")) == 0 || len(os.Getenv("TW_CLIENT_SECRET")) == 0 {
		log.Fatalln("error: no TW_CLIENT_ID and/or TW_CLIENT_SECRET specified in environment! https://dev.twitch.tv/console/app")
	}

	client, err := helix.NewClient(&helix.Options{
		ClientID:     os.Getenv("TW_CLIENT_ID"),
		ClientSecret: os.Getenv("TW_CLIENT_SECRET"),
	})
	if err != nil {
		log.Fatalln(err)
	}

	access_token, err := client.RequestAppAccessToken([]string{})
	if err != nil {
		log.Fatalln(err)
	}
	client.SetAppAccessToken(access_token.Data.AccessToken)

	// Setup notifications
	/*
	if len(os.Getenv("SS_PUSHBULLET_APIKEY")) == 0 || len(os.Getenv("SS_PUSHBULLET_DEVICES")) == 0 {
		log.Fatalln("error: no SS_PUSHBULLET_APIKEY and/or SS_PUSHBULLET_DEVICES specified in environment! https://www.pushbullet.com/#settings/account")
	}
	notifier := notify.New()
	pushbulletService := pushbullet.New(os.Getenv("SS_PUSHBULLET_APIKEY"))
	for _, device := range strings.Split(os.Getenv("SS_PUSHBULLET_DEVICES"), ",") {
		pushbulletService.AddReceivers(device)
	}
	notifier.UseServices(pushbulletService)
	*/

	app := &App{
		store:    store,
		client:   client,
		// notifier: notifier,
		mutex:    &sync.Mutex{},
	}

	port := ":8080"
	if os.Getenv("PORT") != "" {
		port = ":" + os.Getenv("PORT")
	}

	// Create rate limiter (5 requests per second, burst of 10)
	limiter := newRateLimiter(5, 10)

	// Register handlers
	http.HandleFunc("/api/status", limiter.limitMiddleware(app.handleGetStatus))
	http.HandleFunc("/api/streamers", limiter.limitMiddleware(app.handleAddStreamer))
	http.HandleFunc("/webhook/callbacks", app.handleWebhook)

	log.Printf("server starting on %s", port)
	log.Fatal(http.ListenAndServe(port, nil))
}
