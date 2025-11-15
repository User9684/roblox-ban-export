package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type GameJoinRestriction struct {
	Active          bool      `json:"active"`
	StartTime       time.Time `json:"startTime"`
	Duration        *string   `json:"duration,omitempty"`
	PrivateReason   string    `json:"privateReason"`
	DisplayedReason string    `json:"displayReason"`
	ExcludeAlts     bool      `json:"excludeAltAccounts"`
	Inherented      bool      `json:"inherented"`
}

type UserRestriction struct {
	Path        string              `json:"path"`
	User        string              `json:"user"`
	Restriction GameJoinRestriction `json:"gameJoinRestriction"`
}

type ApiResponse struct {
	UserRestrictions []UserRestriction `json:"userRestrictions"`
	NextPageToken    string            `json:"nextPageToken,omitempty"`
}

const UNIVERSE_ID = "6749060816"

var API_URI = "https://apis.roblox.com/cloud/v2/universes/%s/user-restrictions?maxPageSize=100&pageToken=%s"

func robloxRequest(method, uri string, body io.Reader) (*http.Response, error) {
	key := os.Getenv("API_KEY")

	request, err := http.NewRequest(method, uri, body)
	if err != nil {
		return nil, err
	}

	request.Header.Add("x-api-key", key)

	if body != nil {
		request.Header.Add("Content-Type", "application/json")
	}

	return http.DefaultClient.Do(request)
}

func generateCsv() {
	fmt.Println("Querying all bans...")

	file, err := os.OpenFile("bans.csv", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	if err != nil {
		panic(err)
	}

	var writer = csv.NewWriter(file)

	writer.Write([]string{
		"UserId", "Moderator", "Reason",
		"DisplayReason", "Creation", "Duration",
	})

	var nextPageToken = ""
	var count = 0
	for {
		res, err := robloxRequest(http.MethodGet, fmt.Sprintf(API_URI, UNIVERSE_ID, nextPageToken), nil)
		if err != nil {
			panic(err)
		}

		remaining, err := strconv.Atoi(res.Header.Get("x-ratelimit-remaining"))
		if err != nil {
			panic(err)
		}
		reset, err := strconv.Atoi(res.Header.Get("x-ratelimit-reset"))
		if err != nil {
			panic(err)
		}

		if remaining <= 1 {
			fmt.Printf("Ratelimited! Waiting %d seconds.\n", reset)

			time.Sleep(time.Duration(reset) * time.Second)
		}

		var data ApiResponse

		decoder := json.NewDecoder(res.Body)
		decoder.Decode(&data)

		for i, restriction := range data.UserRestrictions {
			count += 1

			userId := strings.Split(restriction.User, "/")[1]

			var gameRestriction = restriction.Restriction

			var moderator string
			var reason string
			var duration = "Permanent"

			_, moderator, found := strings.Cut(gameRestriction.PrivateReason, "Moderator - ")

			if found {
				moderatorName, internalReason, wasSplit := strings.Cut(moderator, "; ")

				moderator = moderatorName

				if wasSplit { // Private reason was provided by lotus
					reason = internalReason
				} else { // No private reason provided, default to DisplayedReason
					reason = gameRestriction.DisplayedReason
				}
			} else { // No moderator found in ban string, most likely manual ban
				moderator = "Manual Ban"
				reason = gameRestriction.PrivateReason
			}

			if gameRestriction.Duration != nil {
				duration = *gameRestriction.Duration
			}

			writer.Write([]string{
				userId, moderator, reason,
				gameRestriction.DisplayedReason, gameRestriction.StartTime.String(), duration,
			})

			if i%25 == 0 {
				writer.Flush()
			}
		}

		writer.Flush()

		nextPageToken = data.NextPageToken
		if len(nextPageToken) <= 0 {
			break
		}
	}

	writer.Flush()

	fmt.Printf("Finished querying bans! Total ban count: %d\n", count)
}

func main() {
	if err := godotenv.Load(); err != nil {
		panic(err)
	}

	generateCsv()
}
