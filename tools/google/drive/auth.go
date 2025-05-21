package drive

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/pkg/browser"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

var (
	credsFilePath string
	tokenPath     string
)

type tokenSavingSource struct {
	src       oauth2.TokenSource
	tokenPath string
}

func (s *tokenSavingSource) Token() (*oauth2.Token, error) {
	tok, err := s.src.Token()
	if err != nil {
		return nil, err
	}
	saveToken(s.tokenPath, tok)
	return tok, nil
}

func init() {
	flag.StringVar(&credsFilePath, "creds-file-path", "", "Path to OAuth2 credentials file")
	flag.StringVar(&tokenPath, "token-path", "", "Path to store token file")
}

// // Authorize handles browser-based login and returns an authenticated Drive client.
// func Authorize() (*drive.Service, error) {
// 	//ln("AUTHORIZE")
// 	flag.Parse()

// 	b, err := os.ReadFile(credsFilePath)
// 	if err != nil {
// 		//fmt.Println("unable to read client secret file: %w", err)
// 		return nil, fmt.Errorf("unable to read client secret file: %w", err)
// 	}

// 	config, err := google.ConfigFromJSON(b, drive.DriveMetadataReadonlyScope)
// 	if err != nil {
// 		//fmt.Println("unable to parse client secret file to config: %w", err)
// 		return nil, fmt.Errorf("unable to parse client secret file to config: %w", err)
// 	}

// 	token, err := tokenFromFile(tokenPath)
// 	if err != nil || !token.Valid() {
// 		token, err = getTokenFromWeb(config)
// 		if err != nil {
// 			//fmt.Println("unable to get token from web: %w", err)
// 			return nil, fmt.Errorf("unable to get token from web: %w", err)
// 		}
// 		saveToken(tokenPath, token)
// 	}

// 	client := config.Client(context.Background(), token)
// 	srv, err := drive.NewService(context.Background(), option.WithHTTPClient(client))
// 	if err != nil {
// 		//fmt.Println("unable to retrieve Drive client: %w", err)
// 		return nil, fmt.Errorf("unable to retrieve Drive client: %w", err)
// 	}

// 	return srv, nil
// }

func Authorize() (*drive.Service, error) {
	flag.Parse()

	b, err := os.ReadFile(credsFilePath)
	if err != nil {
		return nil, fmt.Errorf("unable to read client secret file: %w", err)
	}

	config, err := google.ConfigFromJSON(b, drive.DriveReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret file to config: %w", err)
	}

	tok, err := tokenFromFile(tokenPath)
	if err != nil {
		tok, err = getTokenFromWeb(config)
		if err != nil {
			return nil, fmt.Errorf("unable to get token from web: %w", err)
		}
		saveToken(tokenPath, tok)
	}

	// Wrap the token in a token source that auto-refreshes
	tokenSource := config.TokenSource(context.Background(), tok)

	// Wrap token source to save updated token automatically after refresh
	autoRefreshTokenSource := &tokenSavingSource{
		src:       tokenSource,
		tokenPath: tokenPath,
	}

	client := oauth2.NewClient(context.Background(), autoRefreshTokenSource)
	srv, err := drive.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Drive client: %w", err)
	}

	return srv, nil
}

func getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	// Use a local server to receive the code
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	redirectURL := fmt.Sprintf("http://localhost:%d", port)
	config.RedirectURL = redirectURL

	codeCh := make(chan string)
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Failed to parse response", http.StatusBadRequest)
			return
		}
		code := r.FormValue("code")
		fmt.Fprint(w, "Authorization successful. You may close this window.")
		codeCh <- code
	})}

	go func() {
		_ = srv.Serve(listener)
	}()

	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	//fmt.Printf("Opening browser for auth: %s\n", authURL)
	_ = browser.OpenURL(authURL)

	code := <-codeCh
	_ = srv.Shutdown(context.Background())

	return config.Exchange(context.Background(), code)
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	token := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(token)
	return token, err
}

func saveToken(path string, token *oauth2.Token) {
	//fmt.Printf("Saving token to %s\n", path)
	f, err := os.Create(path)
	if err != nil {
		log.Fatalf("Unable to save oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}
