package auth

import (
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/db"
)

// Keeps an OAuth and Hytale game session up-to-date by scheduling refreshes shortly before expiry.
type AuthStore struct {
	config                  *config.Config
	db                      *db.DB
	httpClient              *http.Client
	oauthToken              db.OAuthToken
	profileUUID             string
	gameSession             db.GameSessionToken
	tokenMutex              *sync.Mutex
	oauthRefreshTimer       *time.Timer
	gameSessionRefreshTimer *time.Timer
}

var expiredGameSessionError = errors.New("Expired game session")

// Authenticates to Hytale OAuth and creates a game session.
// Will resume from tokens stored in database if available.
// Otherwise, asks the user to complete the OAuth flow.
func NewAuthStore(config *config.Config, db *db.DB, httpClient *http.Client) (*AuthStore, error) {
	store := &AuthStore{
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

func (a *AuthStore) initialize() error {
	oauthToken, err := a.initializeOAuth()
	if err != nil {
		return err
	}
	a.oauthToken = oauthToken

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

	a.scheduleOAuthRefresh()
	a.scheduleGameSessionRefresh()
	return nil
}

// Attempts to initialize the OAuth token directly from storage
// Refreshes if near expiry
func (a *AuthStore) initializeOAuth() (db.OAuthToken, error) {
	storedOAuthToken, err := a.db.GetOAuthToken()
	if err != nil || storedOAuthToken == nil {
		if err != nil {
			log.Printf("Error getting stored OAuth token: %v", err)
		}

		newToken, err := a.performOAuthAndStore()
		if err != nil {
			return db.OAuthToken{}, err
		}
		return newToken, nil
	}

	log.Println("Found stored OAuth token")
	updatedToken, err := a.ensureOAuthRefreshed(*storedOAuthToken)
	if err != nil {
		return db.OAuthToken{}, err
	}
	return updatedToken, nil
}

// Attempts to initialize the game session token directly from storage
// Refreshes if near expiry
func (a *AuthStore) initializeFreshGameSession() (db.GameSessionToken, string, error) {
	storedGameSession, err := a.db.GetGameSession()
	if err != nil || storedGameSession == nil {
		if err != nil {
			log.Printf("Error getting stored game session token: %v", err)
		}
		return db.GameSessionToken{}, "", errors.New("No stored game session token found")
	}

	log.Println("Found stored game session token")
	storedProfileUUID, err := a.db.GetProfileUUID()
	if err != nil || storedProfileUUID == "" {
		if err != nil {
			log.Printf("Error getting stored profile UUID: %v", err)
		}
		return db.GameSessionToken{}, "", errors.New("No stored profile UUID found")
	}

	// Unlike OAuth, trying to refresh an expired game session will fail
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
func (a *AuthStore) initializeProfile(oAuthToken db.OAuthToken) (string, error) {
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

func (a AuthStore) performOAuthAndStore() (db.OAuthToken, error) {
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

func (a AuthStore) fetchProfileUUIDAndStore(oAuthToken db.OAuthToken) (string, error) {
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

func (a AuthStore) createGameSessionAndStore(oAuthToken db.OAuthToken, profileUUID string) (db.GameSessionToken, error) {
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

// Refreshes the token if expired, otherwise returns the original token
func (a AuthStore) ensureOAuthRefreshed(oAuthToken db.OAuthToken) (db.OAuthToken, error) {
	if time.Now().Add(time.Duration(a.config.Auth.OAuthRefreshBuffer) * time.Second).After(oAuthToken.ExpiresAt) {
		return a.refreshOAuthAndStore(oAuthToken)
	}
	return oAuthToken, nil
}

// Refreshes the token if expired, otherwise returns the original token
func (a AuthStore) refreshOAuthAndStore(oAuthToken db.OAuthToken) (db.OAuthToken, error) {
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
func (a *AuthStore) ensureGameSessionRefreshed(gameSession db.GameSessionToken, profileUUID string) (db.GameSessionToken, error) {
	if time.Now().Add(time.Duration(a.config.Auth.GameSessionRefreshBuffer) * time.Second).After(gameSession.ExpiresAt) {
		return a.refreshGameSessionAndStore(gameSession, profileUUID)
	}
	return gameSession, nil
}

// Refreshes the token if expired, otherwise returns the original token
func (a AuthStore) refreshGameSessionAndStore(gameSession db.GameSessionToken, profileUUID string) (db.GameSessionToken, error) {
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

func (a *AuthStore) scheduleOAuthRefresh() {
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
			log.Printf("Error refreshing OAuth token: %v", err)
			return
		}

		a.tokenMutex.Lock()
		a.oauthToken = refreshedOAuthToken
		a.tokenMutex.Unlock()

		a.scheduleOAuthRefresh()
	})
}

func (a *AuthStore) scheduleGameSessionRefresh() {
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
				log.Printf("Error refreshing OAuth token: %v", err)
				return
			}

			a.tokenMutex.Lock()
			a.oauthToken = refreshedOAuthToken
			a.tokenMutex.Unlock()

			refreshedSessionToken, err = a.createGameSessionAndStore(refreshedOAuthToken, a.profileUUID)
			if err != nil {
				log.Printf("error creating game session: %v", err)
				return
			}
		}

		a.tokenMutex.Lock()
		a.gameSession = refreshedSessionToken
		a.tokenMutex.Unlock()

		a.scheduleGameSessionRefresh()
	})
}

// Retrieves the current OAuth access token.
// Returns an error if the token is expired, as the background task should keep it up-to-date.
func (a *AuthStore) GetOAuthToken() (string, error) {
	a.tokenMutex.Lock()
	defer a.tokenMutex.Unlock()

	if time.Now().After(a.oauthToken.ExpiresAt) {
		return "", errors.New("OAuth token is expired")
	}

	return a.oauthToken.AccessToken, nil
}

// Retrieves the current game session token.
// Returns an error if the token is expired, as the background task should keep it up-to-date.
func (a *AuthStore) GetGameSessionToken() (string, error) {
	a.tokenMutex.Lock()
	defer a.tokenMutex.Unlock()

	if time.Now().After(a.gameSession.ExpiresAt) {
		return "", errors.New("Game session token is expired")
	}

	return a.gameSession.SessionToken, nil
}
