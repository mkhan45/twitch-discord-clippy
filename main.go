package main

import (
	"fmt"
	"log"
	"os"

	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"
)

var clientID string
var clientSecret string
var accountID = "206263706"
var twitchClient *TwitchClient

type OAuthInfo struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresIn    int      `json:"expires_in"`
	Scope        []string `json:"scope"`
	TokenType    string   `json:"token_type"`
}

type TwitchClient struct {
	httpClient   http.Client
	ClientID     string
	ClientSecret string
        AuthInfo OAuthInfo
}

func handleErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func (c TwitchClient) newRequest(url string, ty string) *http.Request {
	req, err := http.NewRequest(ty, url, nil)
	if err != nil {
		log.Fatalln(err)
	}
	req.Header.Add("Client-ID", c.ClientID)
	return req
}

func (c TwitchClient) getClips(user_id string, count int, startTime time.Time, endTime time.Time) {
	req := c.newRequest(fmt.Sprintf("https://api.twitch.tv/helix/clips?broadcaster_id=%s", user_id), "GET")
	resp, err := c.httpClient.Do(req)
	handleErr(err)

	body, err := ioutil.ReadAll(resp.Body)
	handleErr(err)
	fmt.Printf("Response test: %s\n", string(body))
}

func makeTwitchClient() *TwitchClient {
	return &TwitchClient{
		httpClient:   http.Client{},
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}
}

func (c TwitchClient) oAuthGenURL() string {
	tmplStr := "https://id.twitch.tv/oauth2/authorize" +
		"?client_id=%s" +
		"&redirect_uri=%s" +
		"&response_type=code" +
		"&scope=%s"

	return fmt.Sprintf(tmplStr, c.ClientID, "http://localhost/twitch/oauthhandler", "user:read:email")
}

func (c TwitchClient) oAuthHandler(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	fmt.Fprintf(w, "trying to validate")

	tokenReqURL :=
		"https://id.twitch.tv/oauth2/token" +
			"?client_id=%s" +
			"&client_secret=%s" +
			"&code=%s" +
			"&grant_type=authorization_code" +
			"&redirect_uri=%s"

	formattedURL := fmt.Sprintf(tokenReqURL, c.ClientID, c.ClientSecret, code, "http://localhost/twitch/oauthhandler")
	req := c.newRequest(formattedURL, "POST")
	resp, err := c.httpClient.Do(req)
	handleErr(err)

	if resp.StatusCode == 400 {
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Printf("Invalid Response on Gen Code: %+v\n", body)
		return
	}

	decoder := json.NewDecoder(resp.Body)

	var parsedResp OAuthInfo
	err = decoder.Decode(&parsedResp)
	handleErr(err)

	fmt.Printf("%+v\n", parsedResp)
        c.AuthInfo = parsedResp
}

func (c TwitchClient) oAuthRefresh() {
   fmtURL := "https://id.twitch.tv/oauth2/token" +
   "--data-urlencode" +
   "?grant_type=refresh_token" +
   "&refresh_token=%s" +
   "&client_id=%s" +
   "&client_secret=%s"

   formattedURL := fmt.Sprintf(fmtURL, c.AuthInfo.RefreshToken, c.ClientID, c.ClientSecret)
   req := c.newRequest(formattedURL, "POST")

   resp, err := c.httpClient.Do(req)
   handleErr(err)

   if resp.StatusCode == 400 {
      body, err := ioutil.ReadAll(resp.Body)
      handleErr(err)
      fmt.Printf("Error refreshing token: %s\n", string(body))
      return
   }

   type RefreshResponse struct {
      AccessToken string `json:"access_token"`
      RefreshToken string `json:"refresh_token"`
      Scope string `json:"scope"`
   }

   decoder := json.NewDecoder(resp.Body)
   var parsedResp RefreshResponse
   err = decoder.Decode(&parsedResp)
   handleErr(err)

   c.AuthInfo.AccessToken = parsedResp.AccessToken
   c.AuthInfo.RefreshToken = parsedResp.RefreshToken
}

func init() {
   clientID = os.Getenv("TWITCH_CLIENT_ID")
   clientSecret = os.Getenv("TWITCH_CLIENT_SECRET")
   fmt.Printf("Client ID: %s\n", clientID)
}

func main() {
   twitchClient = makeTwitchClient()
   fmt.Printf("OAuth URL: %s\n", twitchClient.oAuthGenURL())

   handler := http.NewServeMux()
   handler.HandleFunc("/twitch/oauthhandler", twitchClient.oAuthHandler)
   server := http.Server{Addr: ":8080", Handler: handler}
   go server.ListenAndServe()
   time.Sleep(15 * time.Second)
   server.Shutdown(context.TODO())

   // twitchClient.getClips(accountID, 5, time.Now(), time.Now())
}
