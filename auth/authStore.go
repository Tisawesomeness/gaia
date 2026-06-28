package auth

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/db"
	"github.com/Tisawesomeness/gaia/util"
)

type Token struct {
	Token    string
	AuthType AuthType
}

// Keeps an OAuth and Hytale game session up-to-date by scheduling refreshes shortly before expiry.
type AuthStore interface {
	// Retrieves the current OAuth access token.
	// Returns an error if the token is expired, as the background task should keep it up-to-date.
	GetOAuthToken() (Token, error)
	// Retrieves the current game session token.
	// Returns an error if the token is expired, as the background task should keep it up-to-date.
	GetGameSessionToken() (Token, error)
}

type authStore struct {
	config     *config.Config
	db         *db.DB
	httpClient *http.Client

	authType                AuthType
	oauthToken              db.OAuthToken
	profileUUID             string
	gameSession             db.GameSessionToken
	tokenMutex              *sync.Mutex
	oauthRefreshTimer       *time.Timer
	gameSessionRefreshTimer *time.Timer
}

var (
	expiredOAuthError       = errors.New("Expired OAuth")
	expiredGameSessionError = errors.New("Expired game session")
)

// Authenticates to Hytale OAuth and creates a server game session.
// Will resume from tokens stored in database if available.
// Otherwise, asks the bot admin to complete the OAuth flow.
func NewAuthStore(config *config.Config, db *db.DB, httpClient *http.Client) (AuthStore, error) {
	store := &authStore{
		config:     config,
		db:         db,
		httpClient: httpClient,
		authType:   getAuthType(config),
		tokenMutex: &sync.Mutex{},
	}

	err := store.initialize()
	if err != nil {
		return nil, err
	}

	return store, nil
}

func getAuthType(config *config.Config) AuthType {
	switch config.Credentials.AuthMethod {
	case "launcher":
		return Launcher
	case "server":
		return Server
	default:
		panic("unknown auth type")
	}
}

func (a *authStore) initialize() error {
	err := a.initSessions()
	if err != nil {
		return err
	}
	a.scheduleOAuthRefresh()
	a.scheduleGameSessionRefresh()
	return nil
}

func (a *authStore) initSessions() error {
	oauthToken, err := a.initializeFreshOAuth()
	if err != nil {
		if errors.Is(err, expiredOAuthError) {
			log.Println("OAuth token expired, starting new flow...")
		} else {
			log.Printf("Failed to initialize OAuth: %v", err)
			log.Println("Starting new OAuth flow...")
		}

		newToken, err := a.performOAuthAndStore()
		if err != nil {
			return err
		}
		a.oauthToken = newToken
	} else {
		a.oauthToken = oauthToken
	}

	gameSession, profileUUID, err := a.initializeFreshGameSession()
	if err != nil {
		if errors.Is(err, expiredGameSessionError) {
			log.Println("Game session expired, falling back to OAuth")
		} else {
			log.Printf("Failed to initialize game session: %v", err)
			log.Println("Falling back to OAuth")
		}

		profileUUID, err = a.initializeProfile(a.oauthToken)
		if err != nil {
			return err
		}
		a.profileUUID = profileUUID

		gameSession, err = a.createGameSessionAndStore(a.oauthToken, profileUUID)
		if err != nil {
			return err
		}
		a.gameSession = gameSession
	} else {
		a.gameSession = gameSession
		a.profileUUID = profileUUID
	}
	return nil
}

// Attempts to initialize the OAuth token directly from storage
// Refreshes if near expiry
func (a *authStore) initializeFreshOAuth() (db.OAuthToken, error) {
	storedOAuthToken, err := a.db.GetOAuthToken()
	if err != nil || storedOAuthToken == nil {
		return db.OAuthToken{}, errors.New("No stored OAuth token")
	}
	if storedOAuthToken.AuthType != a.authType.String() {
		return db.OAuthToken{}, errors.New("Stored OAuth token was authenticated with different method, re-auth needed")
	}
	log.Println("Found stored OAuth token")

	if time.Now().After((*storedOAuthToken).ExpiresAt) {
		return db.OAuthToken{}, expiredOAuthError
	}
	refreshedToken, err := a.ensureOAuthRefreshed(*storedOAuthToken)
	if err != nil {
		return db.OAuthToken{}, err
	}

	return refreshedToken, nil
}

// Attempts to initialize the game session token directly from storage
// Refreshes if near expiry
func (a *authStore) initializeFreshGameSession() (db.GameSessionToken, string, error) {
	storedGameSession, err := a.db.GetGameSession()
	if err != nil || storedGameSession == nil {
		return db.GameSessionToken{}, "", errors.New("No stored game session token found")
	}
	if storedGameSession.AuthType != a.authType.String() {
		return db.GameSessionToken{}, "", errors.New("Stored game session was authenticated with different method,, re-auth needed")
	}
	log.Println("Found stored game session token")

	var profileUUID string
	if a.config.Credentials.ProfileUUID != "" {
		profileUUID = a.config.Credentials.ProfileUUID
	} else {
		storedProfileUUID, err := a.db.GetProfileUUID()
		if err != nil || storedProfileUUID == "" {
			return db.GameSessionToken{}, "", errors.New("No stored profile UUID found")
		}
		profileUUID = storedProfileUUID
	}

	if time.Now().After((*storedGameSession).ExpiresAt) {
		return db.GameSessionToken{}, "", expiredGameSessionError
	}
	refreshedSession, err := a.ensureGameSessionRefreshed(*storedGameSession, profileUUID)
	if err != nil {
		return db.GameSessionToken{}, "", err
	}

	return refreshedSession, profileUUID, nil
}

// Tries to initialize the profile UUID from database first,
// then tries requesting it with the OAuth token
// If config.Credentials.ProfileUUID is set, use that directly
func (a *authStore) initializeProfile(oAuthToken db.OAuthToken) (string, error) {
	if a.config.Credentials.ProfileUUID != "" {
		log.Printf("Using config profile: %s", a.config.Credentials.ProfileUUID)
		return a.config.Credentials.ProfileUUID, nil
	}

	storedUUID, err := a.db.GetProfileUUID()
	if err != nil || storedUUID == "" {
		if err != nil {
			log.Printf("Error getting stored profile: %v", err)
		}
		newUUID, err := a.fetchProfileUUIDAndStore(oAuthToken)
		if err != nil {
			return "", err
		}
		return newUUID, nil
	} else {
		log.Printf("Found stored profile: %s", storedUUID)
		return storedUUID, nil
	}
}

func (a authStore) onAuthRequired(deviceAuthResponse DeviceAuthResponse) {
	util.DiscordLog(a.config, a.httpClient, "Authentication required!")
	log.Println("===================================")
	log.Println("===== Authentication Required =====")
	log.Printf("Visit: %s\n", deviceAuthResponse.VerificationURI)
	log.Printf("Enter code: %s\n", deviceAuthResponse.UserCode)
	log.Println("===================================")
}

func (a authStore) performOAuthAndStore() (db.OAuthToken, error) {
	var tokenResponse TokenResponse
	var err error
	if a.authType == Server {
		tokenResponse, err = OAuthDeviceFlow(context.Background(), a.config, a.httpClient, a.onAuthRequired)
	} else {
		tokenResponse, err = a.performBrowserOauth()
	}
	if err != nil {
		return db.OAuthToken{}, err
	}

	token := db.OAuthToken{
		AccessToken:  tokenResponse.AccessToken,
		RefreshToken: tokenResponse.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResponse.ExpiresIn) * time.Second),
		AuthType:     a.authType.String(),
	}

	err = a.db.SetOAuthToken(token)
	if err != nil {
		return db.OAuthToken{}, err
	}

	return token, nil
}

type paramsAndErr struct {
	Params RedirectParams
	Err    error
}

func (a authStore) performBrowserOauth() (TokenResponse, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var once sync.Once

	resultCh := make(chan paramsAndErr, 1)
	var notifyCh chan string
	port := uint16(9999) // Arbitrary default port

	// Local web server (if enabled) captures redirect after authorization
	// Fully automatic like Hytale Launcher, but requires running bot and browser
	// on the same machine
	if a.config.Credentials.StartRedirectListener {
		listener, err := net.Listen("tcp", "")
		if err != nil {
			return TokenResponse{}, fmt.Errorf("failed to start listener: %v", err)
		}
		defer listener.Close()

		notifyCh = make(chan string, 1)
		port = uint16(listener.Addr().(*net.TCPAddr).Port)

		server := &http.Server{
			Handler: a.capturedRedirectHandler(&once, resultCh, notifyCh),
		}
		go func() {
			defer server.Close()
			if err := server.Serve(listener); err != nil && err != http.ErrServerClosed && !errors.Is(err, net.ErrClosed) {
				send(&once, resultCh, RedirectParams{}, fmt.Errorf("error starting server: %v\n", err))
			}
		}()
	}

	// Buidls the auth URL, then passes to getRedirectFromUser() to show to user and extract the authorization code
	// Once getRedirectFromUser() returns, OAuthBrowserFlow() exchanges the authorization code for a token
	token, err := OAuthBrowserFlow(ctx, a.config, a.httpClient, port, a.getRedirectFromUserFunc(&once, resultCh))

	// Update web page with auth status, if enabled
	if notifyCh != nil {
		select {
		case notifyCh <- func() string {
			if err == nil {
				return "Authentication successful. You can close this window."
			} else {
				return err.Error()
			}
		}():
		default: // No receiver, do nothing
		}
	}
	return token, err
}

func (a authStore) capturedRedirectHandler(once *sync.Once, resultCh chan<- paramsAndErr, notifyCh <-chan string) http.HandlerFunc {
	// Listens for any HTTP request, should be the user getting redirected after authorizing
	// code/state or error is sent to resultCh so getRedirectFromUser() can proceed
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		code := query.Get("code")
		state := query.Get("state")
		if code == "" || state == "" {
			send(once, resultCh, RedirectParams{}, errors.New("missing code or state parameters"))
			http.Error(w, "Missing code or state parameters", http.StatusBadRequest)
		} else {
			send(once, resultCh, RedirectParams{Code: code, State: state}, nil)
			fmt.Fprintln(w, "Exchanging authorization code for token...")
		}
		// When authentication completes/fails, notify user in their browser
		select {
		case notification := <-notifyCh:
			fmt.Fprintln(w, notification)
		case <-r.Context().Done():
			// exit
		}
	}
}

func (a authStore) getRedirectFromUserFunc(once *sync.Once, resultCh chan paramsAndErr) RedirectCaptureFunc {
	return func(ctx context.Context, authUrl string) (RedirectParams, error) {
		log.Println("===================================")
		log.Println("===== Authentication Required =====")
		log.Printf("Visit: %s\n", authUrl)
		log.Println("After logging in, you will be redirected automatically.")
		log.Println("If you are redirected to a non-existent page, paste the URL below.")
		log.Println("===================================")

		// In case user can't reach local web server (or not enabled), also accept URL pasted in stdin
		fmt.Print("> ")
		stdinReader := bufio.NewReader(os.Stdin)
		go func() {
			// Known issue: ReadString() blocks forever if there is no input
			// Not a big deal (causes a single goroutine leak), but needs to be
			// fixed if future code also reads from stdin
			input, err := stdinReader.ReadString('\n')
			if err != nil {
				send(once, resultCh, RedirectParams{}, fmt.Errorf("failed to read input: %v\n", err))
				return
			}
			input = strings.TrimSpace(input)

			u, err := url.Parse(input)
			if err != nil {
				send(once, resultCh, RedirectParams{}, fmt.Errorf("not a URL: %v\n", err))
				return
			}

			query := u.Query()
			params := RedirectParams{
				Code:  query.Get("code"),
				State: query.Get("state"),
			}
			send(once, resultCh, params, nil)
		}()

		// Wait for either web server or stdin
		select {
		case params := <-resultCh:
			return params.Params, params.Err
		case <-ctx.Done():
			return RedirectParams{}, ctx.Err()
		}
	}
}

func send(once *sync.Once, c chan<- paramsAndErr, params RedirectParams, err error) {
	once.Do(func() {
		c <- paramsAndErr{Params: params, Err: err}
	})
}

func (a authStore) fetchProfileUUIDAndStore(oAuthToken db.OAuthToken) (string, error) {
	log.Println("Fetching account profiles...")
	profiles, err := GetAccountProfiles(oAuthToken.AccessToken, a.authType, a.config, a.httpClient)
	if err != nil {
		return "", err
	}

	if len(profiles) <= 0 {
		return "", errors.New("No profiles found!")
	}

	// Arbitrary choice of first UUID available
	profileUUID := profiles[0].UUID
	log.Printf("Using first profile: %s", profileUUID)
	err = a.db.SetProfileUUID(profileUUID)
	if err != nil {
		return "", err
	}

	return profileUUID, nil
}

func (a authStore) createGameSessionAndStore(oAuthToken db.OAuthToken, profileUUID string) (db.GameSessionToken, error) {
	if time.Now().After(oAuthToken.ExpiresAt) {
		return db.GameSessionToken{}, errors.New("Tried to create game session from expired OAuth token")
	}

	log.Println("Creating game session...")
	sessionResponse, err := CreateGameSession(oAuthToken.AccessToken, profileUUID, a.config, a.httpClient)
	if err != nil {
		return db.GameSessionToken{}, err
	}

	expiresAt, err := time.Parse(time.RFC3339Nano, sessionResponse.ExpiresAt)
	if err != nil {
		return db.GameSessionToken{}, err
	}
	session := db.GameSessionToken{
		SessionToken: sessionResponse.SessionToken,
		ExpiresAt:    expiresAt,
		AuthType:     oAuthToken.AuthType,
	}
	err = a.db.SetGameSession(session)
	if err != nil {
		return db.GameSessionToken{}, err
	}

	return session, nil
}

// Refreshes the token if expired, otherwise returns the original token
func (a authStore) ensureOAuthRefreshed(oAuthToken db.OAuthToken) (db.OAuthToken, error) {
	if time.Now().Add(time.Duration(a.config.Auth.OAuthRefreshBuffer) * time.Second).After(oAuthToken.ExpiresAt) {
		return a.refreshOAuthAndStore(oAuthToken)
	}
	return oAuthToken, nil
}

// Refreshes the token if expired, otherwise returns the original token
func (a authStore) refreshOAuthAndStore(oAuthToken db.OAuthToken) (db.OAuthToken, error) {
	log.Println("Refreshing OAuth token...")
	tokenResponse, err := OAuthRefresh(a.config, a.httpClient, oAuthToken.RefreshToken, a.authType)
	if err != nil {
		return db.OAuthToken{}, err
	}

	token := db.OAuthToken{
		AccessToken:  tokenResponse.AccessToken,
		RefreshToken: tokenResponse.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResponse.ExpiresIn) * time.Second),
		AuthType:     oAuthToken.AuthType,
	}

	err = a.db.SetOAuthToken(token)
	if err != nil {
		return db.OAuthToken{}, err
	}

	return token, nil
}

// Refreshes the token if expired, otherwise returns the original token
func (a authStore) ensureGameSessionRefreshed(gameSession db.GameSessionToken, profileUUID string) (db.GameSessionToken, error) {
	if time.Now().Add(time.Duration(a.config.Auth.GameSessionRefreshBuffer) * time.Second).After(gameSession.ExpiresAt) {
		return a.refreshGameSessionAndStore(gameSession, profileUUID)
	}
	return gameSession, nil
}

// Refreshes the token if expired, otherwise returns the original token
func (a authStore) refreshGameSessionAndStore(gameSession db.GameSessionToken, profileUUID string) (db.GameSessionToken, error) {
	log.Println("Refreshing game session token...")
	sessionResponse, err := RefreshGameSession(gameSession.SessionToken, profileUUID, a.config, a.httpClient)
	if err != nil {
		return db.GameSessionToken{}, err
	}

	expiresAt, err := time.Parse(time.RFC3339Nano, sessionResponse.ExpiresAt)
	if err != nil {
		return db.GameSessionToken{}, err
	}
	session := db.GameSessionToken{
		SessionToken: sessionResponse.SessionToken,
		ExpiresAt:    expiresAt,
		AuthType:     gameSession.AuthType,
	}

	err = a.db.SetGameSession(session)
	if err != nil {
		return db.GameSessionToken{}, err
	}

	return session, nil
}

func (a *authStore) scheduleOAuthRefresh() {
	a.tokenMutex.Lock()
	expiresAt := a.oauthToken.ExpiresAt
	a.tokenMutex.Unlock()

	timeUntilRefresh := max(time.Until(expiresAt)-time.Duration(a.config.Auth.OAuthRefreshBuffer)*time.Second, 0)

	a.oauthRefreshTimer = time.AfterFunc(timeUntilRefresh, func() {
		a.tokenMutex.Lock()
		oauthToken := a.oauthToken
		a.tokenMutex.Unlock()

		refreshedOAuthToken, err := a.ensureOAuthRefreshed(oauthToken)
		if err != nil {
			util.DiscordLogf(a.config, a.httpClient, "Error refreshing OAuth token: %v", err)
			return
		}

		a.tokenMutex.Lock()
		a.oauthToken = refreshedOAuthToken
		a.tokenMutex.Unlock()

		a.scheduleOAuthRefresh()
	})
}

func (a *authStore) scheduleGameSessionRefresh() {
	a.tokenMutex.Lock()
	expiresAt := a.gameSession.ExpiresAt
	a.tokenMutex.Unlock()

	timeUntilRefresh := max(time.Until(expiresAt)-time.Duration(a.config.Auth.GameSessionRefreshBuffer)*time.Second, 0)

	a.gameSessionRefreshTimer = time.AfterFunc(timeUntilRefresh, func() {
		a.tokenMutex.Lock()
		gameSession := a.gameSession
		a.tokenMutex.Unlock()

		refreshedSessionToken, err := a.refreshGameSessionAndStore(gameSession, a.profileUUID)
		if err != nil {
			log.Printf("Error refreshing game session token: %v", err)
			log.Println("Falling back to OAuth")

			a.tokenMutex.Lock()
			oauthToken := a.oauthToken
			a.tokenMutex.Unlock()

			refreshedOAuthToken, err := a.ensureOAuthRefreshed(oauthToken)
			if err != nil {
				util.DiscordLogf(a.config, a.httpClient, "Error refreshing OAuth token: %v", err)
				return
			}

			a.tokenMutex.Lock()
			a.oauthToken = refreshedOAuthToken
			a.tokenMutex.Unlock()

			refreshedSessionToken, err = a.createGameSessionAndStore(refreshedOAuthToken, a.profileUUID)
			if err != nil {
				util.DiscordLogf(a.config, a.httpClient, "Error creating game session after refresh: %v", err)
				return
			}
		}

		a.tokenMutex.Lock()
		a.gameSession = refreshedSessionToken
		a.tokenMutex.Unlock()

		a.scheduleGameSessionRefresh()
	})
}

func (a *authStore) GetOAuthToken() (Token, error) {
	a.tokenMutex.Lock()
	defer a.tokenMutex.Unlock()

	if time.Now().After(a.oauthToken.ExpiresAt) {
		return Token{}, errors.New("OAuth token is expired")
	}

	return Token{
		Token:    a.oauthToken.AccessToken,
		AuthType: a.authType,
	}, nil
}

func (a *authStore) GetGameSessionToken() (Token, error) {
	a.tokenMutex.Lock()
	defer a.tokenMutex.Unlock()

	if time.Now().After(a.gameSession.ExpiresAt) {
		return Token{}, errors.New("Game session token is expired")
	}

	return Token{
		Token:    a.gameSession.SessionToken,
		AuthType: a.authType,
	}, nil
}
