package auth

import (
	"errors"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sync"
	"time"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/db"
	"github.com/Tisawesomeness/gaia/util"
	tld "github.com/jpillora/go-tld"
)

// Keeps an OAuth and Hytale game session up-to-date by scheduling refreshes shortly before expiry.
type AuthStore interface {
	// Retrieves the current OAuth access token.
	// Returns an error if the token is expired, as the background task should keep it up-to-date.
	GetOAuthToken() (string, error)
	// Retrieves the current game session token.
	// Returns an error if the token is expired, as the background task should keep it up-to-date.
	GetGameSessionToken() (string, error)
	// Returns the HTTP client with Kratos login cookies stored, and whether it exists.
	// Must ONLY be used for the hytale.com domain, since this HTTP client does not separate cookies by domain.
	GetKratosClient() (*http.Client, bool)
	// Saves the Kratos client cookies to restore later.
	// Does nothing if Kratos is not enabled.
	SaveKratosClient() error
}

type authStore struct {
	config     *config.Config
	db         *db.DB
	httpClient *http.Client

	oauthToken              db.OAuthToken
	profileUUID             string
	gameSession             db.GameSessionToken
	tokenMutex              *sync.Mutex
	oauthRefreshTimer       *time.Timer
	gameSessionRefreshTimer *time.Timer

	// May be nil
	kratosClient       *http.Client
	kratosRefreshTimer *time.Timer
}

var (
	expiredOAuthError       = errors.New("Expired OAuth")
	expiredGameSessionError = errors.New("Expired game session")
	expiredKratosError      = errors.New("Expired Kratos")
)

// Authenticates to Hytale OAuth, creates a game session, and authenticates to Kratos auth (if configured).
// Will resume from tokens stored in database if available.
// Otherwise, asks the bot admin to complete the OAuth flow.
func NewAuthStore(config *config.Config, db *db.DB, httpClient *http.Client) (AuthStore, error) {
	store := &authStore{
		config:     config,
		db:         db,
		httpClient: httpClient,
		tokenMutex: &sync.Mutex{},
	}

	err := store.initialize()
	if err != nil {
		return nil, err
	}

	return store, nil
}

func (a *authStore) initialize() error {
	err := a.initSessions()
	if err != nil {
		return err
	}

	if a.config.Credentials.HytaleEmail != "" && a.config.Credentials.HytalePassword != "" {
		err = a.initKratos()
		if err != nil {
			log.Printf("Could not init Kratos: %v", err)
		}
	}

	a.scheduleOAuthRefresh()
	a.scheduleGameSessionRefresh()
	if a.kratosClient != nil {
		a.scheduleKratosRefresh()
	}
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

func (a *authStore) initKratos() error {
	kratosClient, err := a.initializeFreshKratos()
	if err != nil {
		if errors.Is(err, expiredKratosError) {
			log.Println("Kratos session expired, starting new flow...")
		} else {
			log.Printf("Failed to initialize Kratos: %v", err)
			log.Println("Starting new Kratos flow...")
		}

		newKratosClient, err := a.kratosLoginAndStore()
		if err != nil {
			return err
		}
		a.kratosClient = newKratosClient
	} else {
		a.kratosClient = kratosClient
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
	log.Println("Found stored game session token")

	storedProfileUUID, err := a.db.GetProfileUUID()
	if err != nil || storedProfileUUID == "" {
		return db.GameSessionToken{}, "", errors.New("No stored profile UUID found")
	}

	if time.Now().After((*storedGameSession).ExpiresAt) {
		return db.GameSessionToken{}, "", expiredGameSessionError
	}
	refreshedSession, err := a.ensureGameSessionRefreshed(*storedGameSession, storedProfileUUID)
	if err != nil {
		return db.GameSessionToken{}, "", err
	}

	return refreshedSession, storedProfileUUID, nil
}

// Tries to initialize the profile UUID from database first,
// then tries requesting it with the OAuth token
func (a *authStore) initializeProfile(oAuthToken db.OAuthToken) (string, error) {
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

func (a *authStore) initializeFreshKratos() (*http.Client, error) {
	kratosClient, err := initHTTPWithCookies(a.config)
	if err != nil {
		return nil, err
	}
	baseUrl, err := a.baseHytaleUrl()
	if err != nil {
		return nil, err
	}

	storedKratosCookies, err := a.db.GetKratosCookies()
	if err != nil || storedKratosCookies == "" {
		return nil, errors.New("No stored Kratos cookies found")
	}
	log.Println("Found stored Kratos cookies")

	cookies, err := http.ParseCookie(storedKratosCookies)
	if err != nil {
		return nil, err
	}
	kratosClient.Jar.SetCookies(baseUrl, cookies)

	expiry := a.kratosExpiry(kratosClient)
	if expiry == nil || time.Now().After(*expiry) {
		return nil, expiredKratosError
	}
	err = a.ensureKratosRefreshed(kratosClient)
	if err != nil {
		return nil, err
	}

	return kratosClient, nil
}

func (a authStore) performOAuthAndStore() (db.OAuthToken, error) {
	util.DiscordLog(a.config, a.httpClient, "Authentication required!")
	tokenResponse, err := OAuthDeviceFlow(a.config, a.httpClient)
	if err != nil {
		return db.OAuthToken{}, err
	}

	token := db.OAuthToken{
		AccessToken:  tokenResponse.AccessToken,
		RefreshToken: tokenResponse.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResponse.ExpiresIn) * time.Second),
	}

	err = a.db.SetOAuthToken(token)
	if err != nil {
		return db.OAuthToken{}, err
	}

	return token, nil
}

func (a authStore) fetchProfileUUIDAndStore(oAuthToken db.OAuthToken) (string, error) {
	log.Println("Fetching account profiles...")
	profiles, err := GetAccountProfiles(oAuthToken.AccessToken, a.config, a.httpClient)
	if err != nil {
		return "", err
	}

	if len(profiles.Profiles) <= 0 {
		return "", errors.New("No profiles found!")
	}

	// Arbitrary choice of first UUID available
	profileUUID := profiles.Profiles[0].UUID
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
	}
	err = a.db.SetGameSession(session)
	if err != nil {
		return db.GameSessionToken{}, err
	}

	return session, nil
}

func (a authStore) kratosLoginAndStore() (*http.Client, error) {
	kratosClient, err := initHTTPWithCookies(a.config)
	if err != nil {
		return nil, err
	}
	baseUrl, err := a.baseHytaleUrl()
	if err != nil {
		return nil, err
	}

	log.Println("Starting Kratos login...")
	err = KratosLogin(a.config, kratosClient)
	if err != nil {
		return nil, err
	}
	log.Println("Kratos login successful")

	cookies := kratosClient.Jar.Cookies(baseUrl)
	err = a.db.SetKratosCookies(util.AsCookieHeader(cookies))
	if err != nil {
		return nil, err
	}

	return kratosClient, nil
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
	tokenResponse, err := OAuthRefresh(oAuthToken.RefreshToken, a.config, a.httpClient)
	if err != nil {
		return db.OAuthToken{}, err
	}

	token := db.OAuthToken{
		AccessToken:  tokenResponse.AccessToken,
		RefreshToken: tokenResponse.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResponse.ExpiresIn) * time.Second),
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
	}

	err = a.db.SetGameSession(session)
	if err != nil {
		return db.GameSessionToken{}, err
	}

	return session, nil
}

func (a authStore) ensureKratosRefreshed(kratosClient *http.Client) error {
	var expiresAt time.Time
	expiry := a.kratosExpiry(a.kratosClient)
	if expiry == nil {
		expiresAt = time.Now()
	} else {
		expiresAt = *expiry
	}

	if time.Now().Add(time.Duration(a.config.Kratos.RenewalBufferSeconds) * time.Second).After(expiresAt) {
		return a.refreshKratosAndStore(kratosClient)
	}
	return nil
}

func (a authStore) refreshKratosAndStore(kratosClient *http.Client) error {
	baseUrl, err := a.baseHytaleUrl()
	if err != nil {
		return err
	}

	log.Println("Refreshing Kratos...")
	err = KratosLogin(a.config, a.kratosClient)
	if err != nil {
		return err
	}

	cookies := kratosClient.Jar.Cookies(baseUrl)
	err = a.db.SetKratosCookies(util.AsCookieHeader(cookies))

	return nil
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

func (a *authStore) scheduleKratosRefresh() {
	var expiresAt time.Time
	expiry := a.kratosExpiry(a.kratosClient)
	if expiry == nil {
		expiresAt = time.Now()
	} else {
		expiresAt = *expiry
	}

	timeUntilRefresh := max(time.Until(expiresAt)-time.Duration(a.config.Kratos.RenewalBufferSeconds)*time.Second, 0)

	a.kratosRefreshTimer = time.AfterFunc(timeUntilRefresh, func() {
		err := a.ensureKratosRefreshed(a.kratosClient)
		if err != nil {
			util.DiscordLogf(a.config, a.httpClient, "Error refreshing Kratos: %v", err)
			return
		}

		a.scheduleKratosRefresh()
	})
}

func initHTTPWithCookies(config *config.Config) (*http.Client, error) {
	// No PublicSuffixList is OK as long as client obeys instructions
	// to only use for hytale.com domain
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	tr := &http.Transport{
		MaxIdleConns: config.HTTP.MaxIdleConns,
	}
	return &http.Client{
		Transport: tr,
		Timeout:   time.Duration(config.HTTP.Timeout) * time.Second,
		Jar:       jar,
	}, nil
}

func (a *authStore) baseHytaleUrl() (*url.URL, error) {
	parsed, err := tld.Parse(a.config.Kratos.AccountsBackend)
	if err != nil {
		return nil, err
	}
	return &url.URL{
		Scheme: "https",
		Host:   parsed.Domain + "." + parsed.TLD,
	}, nil
}

func (a *authStore) kratosExpiry(kratosClient *http.Client) *time.Time {
	baseUrl, err := a.baseHytaleUrl()
	if err != nil {
		return nil
	}
	cookies := kratosClient.Jar.Cookies(baseUrl)
	for _, cookie := range cookies {
		if cookie.Name == KratosSessionCookie {
			// While server can send either Max-Age or Expires,
			// Go's default HTTP client always sets the Expires field
			return &cookie.Expires
		}
	}
	return nil
}

func (a *authStore) GetOAuthToken() (string, error) {
	a.tokenMutex.Lock()
	defer a.tokenMutex.Unlock()

	if time.Now().After(a.oauthToken.ExpiresAt) {
		return "", errors.New("OAuth token is expired")
	}

	return a.oauthToken.AccessToken, nil
}

func (a *authStore) GetGameSessionToken() (string, error) {
	a.tokenMutex.Lock()
	defer a.tokenMutex.Unlock()

	if time.Now().After(a.gameSession.ExpiresAt) {
		return "", errors.New("Game session token is expired")
	}

	return a.gameSession.SessionToken, nil
}

func (a *authStore) GetKratosClient() (*http.Client, bool) {
	return a.kratosClient, a.kratosClient != nil
}

func (a *authStore) SaveKratosClient() error {
	baseUrl, err := a.baseHytaleUrl()
	if err != nil {
		return nil
	}
	if a.kratosClient != nil {
		cookies := a.kratosClient.Jar.Cookies(baseUrl)
		return a.db.SetKratosCookies(util.AsCookieHeader(cookies))
	}
	return nil
}
