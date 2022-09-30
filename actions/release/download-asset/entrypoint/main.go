package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	backoff "github.com/cenkalti/backoff/v4"
)

func main() {
	var config struct {
		Token          string
		Output         string
		URL            string
		RetryTimeLimit string
	}

	flag.StringVar(&config.Output, "output", "", "Filepath locatin of the downloaded asset")
	flag.StringVar(&config.URL, "url", "", "URL of the asset to download")
	flag.StringVar(&config.Token, "token", "", "Github Authorization Token")
	flag.StringVar(&config.RetryTimeLimit, "retry-time-limit", "1m", "How long to retry failures for")
	flag.Parse()

	if config.Output == "" {
		fail(errors.New(`missing required input "output"`))
	}

	if config.URL == "" {
		fail(errors.New(`missing required input "url"`))
	}

	if config.Token == "" {
		fail(errors.New(`missing required input "token"`))
	}

	retryTimeLimit, err := time.ParseDuration(config.RetryTimeLimit)
	if err != nil {
		fail(err)
	}

	req, err := http.NewRequest("GET", config.URL, nil)
	if err != nil {
		fail(fmt.Errorf("failed to create request: %w", err))
	}

	req.Header.Set("Authorization", fmt.Sprintf("token %s", config.Token))
	req.Header.Set("Accept", "application/octet-stream")

	exponentialBackoff := backoff.NewExponentialBackOff()
	exponentialBackoff.MaxElapsedTime = retryTimeLimit

	err = backoff.RetryNotify(func() error {
		fmt.Printf("Downloading asset: %s -> %s\n", config.URL, config.Output)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to complete request: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to download asset: unexpected status: %s", resp.Status)
		}

		file, err := os.Create(config.Output)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer file.Close()

		_, err = io.Copy(file, resp.Body)
		if err != nil {
			return fmt.Errorf("failed to write to output file: %w", err)
		}

		return nil
	},
		exponentialBackoff,
		func(err error, t time.Duration) {
			fmt.Println(err)
			fmt.Printf("Retrying in %s\n", t)
		},
	)

	if err != nil {
		fail(err)
	}

	fmt.Println("Download complete")
}

func fail(err error) {
	fmt.Printf("Error: %s", err)
	os.Exit(1)
}
