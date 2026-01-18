package auth

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/db"
)

type AuthStore struct {
	config      config.Config
	db          db.DB
	httpClient  *http.Client
	oauthToken  db.OAuthToken
	profileUUID string
	gameSession db.GameSessionToken
}

// Authenticates to Hytale OAuth and creates a game session.
// Will resume from tokens stored in database if available.
// Otherwise, asks the user to complete the OAuth flow.
func NewAuthStore(config config.Config, db db.DB, httpClient *http.Client) (*AuthStore, error) {
	store := &AuthStore{
		config:     config,
		db:         db,
		httpClient: httpClient,
	}

	err := store.initialize()
	if err != nil {
		return nil, err
	}

	return store, nil
}

func (a *AuthStore) initialize() error {
	oAuthToken, err := a.initializeOAuth()
	if err != nil {
		return err
	}
	a.oauthToken = oAuthToken

	profileUUID, err := a.initializeProfile(oAuthToken)
	if err != nil {
		return err
	}
	a.profileUUID = profileUUID

	gameSession, err := a.initializeGameSession(oAuthToken, profileUUID)
	if err != nil {
		return err
	}
	a.gameSession = gameSession
	return nil
}

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
	} else {
		log.Println("Found stored OAuth token")
		updatedToken, err := a.ensureOAuthRefreshed(*storedOAuthToken)
		if err != nil {
			return db.OAuthToken{}, err
		}
		return updatedToken, nil
	}
}

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

func (a *AuthStore) initializeGameSession(oAuthToken db.OAuthToken, profileUUID string) (db.GameSessionToken, error) {
	storedGameSession, err := a.db.GetGameSession()
	if err != nil || storedGameSession == nil {
		if err != nil {
			log.Printf("Error getting stored game session token: %v", err)
		}
		newGameSession, err := a.createGameSessionAndStore(oAuthToken, profileUUID)
		if err != nil {
			return db.GameSessionToken{}, err
		}
		return newGameSession, nil
	} else {
		log.Println("Found stored game session token")
		newGameSession, err := a.ensureGameSessionRefreshed(oAuthToken, profileUUID, *storedGameSession)
		if err != nil {
			return db.GameSessionToken{}, err
		}
		return newGameSession, nil
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
func (a *AuthStore) ensureOAuthRefreshed(oAuthToken db.OAuthToken) (db.OAuthToken, error) {
	if time.Now().After(oAuthToken.ExpiresAt) {
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

		a.oauthToken = token
		return token, nil
	}

	return oAuthToken, nil
}

// Refreshes the token if expired, otherwise returns the original token
func (a *AuthStore) ensureGameSessionRefreshed(oAuthToken db.OAuthToken, profileUUID string, gameSession db.GameSessionToken) (db.GameSessionToken, error) {
	if time.Now().After(gameSession.ExpiresAt) {
		log.Println("Refreshing game session token...")
		tokenResponse, err := a.ensureOAuthRefreshed(oAuthToken)
		if err != nil {
			return db.GameSessionToken{}, err
		}

		newSession, err := a.createGameSessionAndStore(tokenResponse, profileUUID)
		if err != nil {
			return db.GameSessionToken{}, err
		}

		a.gameSession = newSession
		return newSession, nil
	}

	return gameSession, nil
}

// Retrieves the current game session token.
// If expired, will block and request a new one.
func (a *AuthStore) GetGameSessionToken() (string, error) {
	session, err := a.ensureGameSessionRefreshed(a.oauthToken, a.profileUUID, a.gameSession)
	if err != nil {
		return "", err
	}
	return session.SessionToken, nil
}
