package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
)

func main() {
	fmt.Println("Dispatching")

	var config struct {
		Endpoint string
		Repos    string
		Token    string
		Event    string
		Payload  string
	}

	flag.StringVar(&config.Endpoint, "endpoint", "https://api.github.com", "Specifies endpoint for sending dispatch request")
	flag.StringVar(&config.Repos, "repos", "", "Specifies comma separated list of repos for sending dispatch request")
	flag.StringVar(&config.Token, "token", "", "Github Authorization Token")
	flag.StringVar(&config.Event, "event", "", "event type sent with the dispatch")
	flag.StringVar(&config.Payload, "payload", "", "payload sent with the dispatch")
	flag.Parse()

	if config.Event == "" {
		fail(errors.New("missing required input \"event\""))
	}

	if config.Payload == "" {
		fail(errors.New("missing required input \"payload\""))
	}

	if config.Repos == "" {
		fail(errors.New("missing required input \"repos\""))
	}

	if config.Token == "" {
		fail(errors.New("missing required input \"token\""))
	}

	var dispatch struct {
		EventType     string          `json:"event_type"`
		ClientPayload json.RawMessage `json:"client_payload"`
	}

	dispatch.EventType = config.Event
	dispatch.ClientPayload = json.RawMessage(config.Payload)

	payloadData, err := json.Marshal(&dispatch)
	if err != nil {
		fail(err)
	}

	repos := strings.Split(config.Repos, ",")

	for _, r := range repos {
		repo := strings.TrimSpace(r)
		fmt.Printf("  Repository: %s\n", repo)
		req, err := http.NewRequest("POST", fmt.Sprintf("%s/repos/%s/dispatches", config.Endpoint, repo), bytes.NewBuffer(payloadData))
		if err != nil {
			fail(fmt.Errorf("failed to create dispatch request: %w", err))
		}

		req.Header.Set("Authorization", fmt.Sprintf("token %s", config.Token))

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fail(fmt.Errorf("failed to complete dispatch request: %w", err))
		}

		if resp.StatusCode != http.StatusNoContent {
			dump, _ := httputil.DumpResponse(resp, true)
			fail(fmt.Errorf("unexpected response from dispatch request: %s", dump))
		}

		fmt.Println("Success!")
	}
}

func fail(err error) {
	fmt.Printf("Error: %s", err)
	os.Exit(1)
}
