package restmodel

type AuthSuccess struct {
	/* variables */
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	ExpiresIn   int64  `json:"expires_in"`
}

type UserInfo struct {
	ID string
	Name string
}