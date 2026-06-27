// Unit test utilities. Must not import anything other than auth to prevent import loops.
package atestutil

import "github.com/Tisawesomeness/gaia/auth"

type TestAuthStore struct {
	AuthType         auth.AuthType
	OAuthToken       string
	GameSessionToken string
}

func (a TestAuthStore) GetOAuthToken() (auth.Token, error) {
	return auth.Token{
		Token:    a.GameSessionToken,
		AuthType: a.AuthType,
	}, nil
}

func (a TestAuthStore) GetGameSessionToken() (auth.Token, error) {
	return auth.Token{
		Token:    a.OAuthToken,
		AuthType: a.AuthType,
	}, nil
}

func NewTestAuthStore(authType auth.AuthType) TestAuthStore {
	return TestAuthStore{
		AuthType:         authType,
		OAuthToken:       "sample",
		GameSessionToken: "sample",
	}
}
