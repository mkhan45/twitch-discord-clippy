package main

import (
	//"bufio"
	"fmt"
	"log"
	"os"

	"encoding/json"
	"github.com/bwmarrin/discordgo"
	"io/ioutil"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

var clientID string
var clientSecret string
var discordToken string
var twitchClient *TwitchClient
var discordChannels []string
var accountID = "206263706"
var sentClips = make(map[string]bool)

var redirectURI string

type clip struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	Title     string `json:"title"`
	CreatedAt string `json:"created_at"`
}

// OAuthInfo is info for OAuth
type OAuthInfo struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresIn    int      `json:"expires_in"`
	Scope        []string `json:"scope"`
	TokenType    string   `json:"token_type"`
}

// TwitchClient is a twitch client
type TwitchClient struct {
	httpClient   http.Client
	ClientID     string
	ClientSecret string
	AuthInfo     OAuthInfo
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
	if c.AuthInfo.AccessToken != "" {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.AuthInfo.AccessToken))
	}
	return req
}

func (c TwitchClient) getIDFromUsername(username string) string {
	url := "https://api.twitch.tv/helix/users?login=%s"
	req := c.newRequest(fmt.Sprintf(url, username), "GET")

	resp, err := c.httpClient.Do(req)
	handleErr(err)

	if resp.StatusCode == 401 {
		c.oAuthRefresh()
		return ""
	} else if resp.StatusCode != 200 {
		respBody, err := ioutil.ReadAll(resp.Body)
		handleErr(err)
		fmt.Printf("Error getting username: %s", string(respBody))
		return ""
	}

	type UserResponse struct {
		DisplayName string `json:"display_name"`
		ID          string `json:"id"`
	}

	decoder := json.NewDecoder(resp.Body)
	var parsedResp UserResponse
	decoder.Decode(&parsedResp)

	return parsedResp.ID
}

func (c TwitchClient) isUserStreaming(userID string) bool {
	reqStr := "https://api.twitch.tv/helix/channels?broadcaster_id=%s"
	req := c.newRequest(fmt.Sprintf(reqStr, userID), "GET")
	resp, err := c.httpClient.Do(req)
	handleErr(err)

	if resp.StatusCode == 401 {
		c.oAuthRefresh()
		return c.isUserStreaming(userID)
	} else if !(resp.StatusCode >= 200 && resp.StatusCode < 300) {
		body, err := ioutil.ReadAll(resp.Body)
		handleErr(err)
		fmt.Printf("Error getting broadcast: %s\n", string(body))
		return false
	}

	body, err := ioutil.ReadAll(resp.Body)
	handleErr(err)
	fmt.Printf("Error getting broadcast: %s\n", string(body))

	return false
}

func (c TwitchClient) getClips(userID string, count int, startTime time.Time, endTime time.Time) []string {
	reqStr := "https://api.twitch.tv/helix/clips?broadcaster_id=%s&first=%d&started_at=%s"
	startTimeStr := startTime.Format(time.RFC3339)
	req := c.newRequest(fmt.Sprintf(reqStr, userID, count, startTimeStr), "GET")
	resp, err := c.httpClient.Do(req)
	handleErr(err)

	if !(resp.StatusCode >= 200 && resp.StatusCode < 300) {
		body, err := ioutil.ReadAll(resp.Body)
		handleErr(err)
		fmt.Printf("Error getting clips: %s\n", string(body))
		return make([]string, 1)
	}

	type clipResponse struct {
		Data []clip `json:"data"`
	}

	decoder := json.NewDecoder(resp.Body)
	var parsedData clipResponse
	err = decoder.Decode(&parsedData)
	handleErr(err)

	retArr := make([]string, 5)
	for _, clip := range parsedData.Data {
		retArr = append(retArr, clip.URL)
	}
	return retArr
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

	return fmt.Sprintf(tmplStr, c.ClientID, redirectURI, "user:read:email")
}

func testHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "test")
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

	formattedURL := fmt.Sprintf(tokenReqURL, c.ClientID, c.ClientSecret, code, redirectURI)
	req := c.newRequest(formattedURL, "POST")
	resp, err := c.httpClient.Do(req)
	handleErr(err)

	if !(resp.StatusCode >= 200 && resp.StatusCode < 300) {
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Printf("Invalid Response on Gen Code: %+v\n", body)
		return
	}

	decoder := json.NewDecoder(resp.Body)

	var parsedResp OAuthInfo
	err = decoder.Decode(&parsedResp)
	handleErr(err)

	c.AuthInfo = parsedResp
	twitchClient = &c
	fmt.Printf("Auth Info: %+v\n", c.AuthInfo)
}

func (c TwitchClient) oAuthRefresh() {
	fmt.Printf("Refreshing OAuth Tokens\n")

	fmtURL := "https://id.twitch.tv/oauth2/token" +
		"?grant_type=refresh_token" +
		"&refresh_token=%s" +
		"&client_id=%s" +
		"&client_secret=%s"

	formattedURL := fmt.Sprintf(fmtURL, c.AuthInfo.RefreshToken, c.ClientID, c.ClientSecret)
	fmt.Printf("Auth Info: %+v\n", c.AuthInfo)
	fmt.Printf("Refresh URL: %s\n", formattedURL)
	req := c.newRequest(formattedURL, "POST")

	resp, err := c.httpClient.Do(req)
	handleErr(err)

	if resp.StatusCode != 200 {
		body, err := ioutil.ReadAll(resp.Body)
		handleErr(err)
		fmt.Printf("Error refreshing token: %s\n", string(body))
		return
	}

	type RefreshResponse struct {
		AccessToken  string   `json:"access_token"`
		RefreshToken string   `json:"refresh_token"`
		Scope        []string `json:"scope"`
	}

	decoder := json.NewDecoder(resp.Body)
	var parsedResp RefreshResponse
	err = decoder.Decode(&parsedResp)
	handleErr(err)

	c.AuthInfo.AccessToken = parsedResp.AccessToken
	c.AuthInfo.RefreshToken = parsedResp.RefreshToken
	c.AuthInfo.Scope = parsedResp.Scope
}

func init() {
	clientID = os.Getenv("TWITCH_CLIENT_ID")
	clientSecret = os.Getenv("TWITCH_CLIENT_SECRET")
	discordToken = os.Getenv("DISCORD_TOKEN")
	redirectURI = os.Getenv("TWITCH_REDIRECT_URI")
	fmt.Printf("Client ID: %s\n", clientID)
}

func main() {
	twitchClient = makeTwitchClient()
	fmt.Printf("OAuth URL: %s\n", twitchClient.oAuthGenURL())

	handler := http.NewServeMux()
	handler.HandleFunc("/twitch/oauthhandler", twitchClient.oAuthHandler)
	go http.ListenAndServeTLS(":8000", "keys/fullchain.pem", "keys/privkey.pem", handler)

	time.Sleep(12 * time.Second)
	//fmt.Println("waiting for enter")
	//bufio.NewReader(os.Stdin).ReadBytes('\n')
	//fmt.Println("continuing")

	discordClient, err := discordgo.New("Bot " + discordToken)
	handleErr(err)

	discordClient.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Content == "~~setchannel" {
			discordChannels = append(discordChannels, m.ChannelID)
			s.ChannelMessageSend(m.ChannelID, "Set channel to here")
			fmt.Printf("Set channel")
		}
	})
	err = discordClient.Open()
	handleErr(err)

	fmt.Println("Bot running")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)

	go func() {
		tick := 0
		for {
			tick++
			if tick%15 == 0 {
				twitchClient.oAuthRefresh()
			}

			clipURLs := twitchClient.getClips(accountID, 2, time.Now().Add(-time.Second*60), time.Now())
			for _, clipURL := range clipURLs {
				fmt.Printf("Clip: %s\n", clipURL)
				if clipURL != "" && sentClips[clipURL] != true {
					for _, channel := range discordChannels {
						discordClient.ChannelMessageSend(channel, clipURL)
						sentClips[clipURL] = true
					}
				}
			}
			time.Sleep(time.Second * 30)
		}
	}()

	<-sc
}
