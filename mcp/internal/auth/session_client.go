package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/platform"
)

// NativeClientType is the session kind core gives to clients that keep the refresh token themselves.
const NativeClientType = "native"

type LoginRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	Client     string `json:"client"`
	DeviceName string `json:"deviceName"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type SignInResponse struct {
	AccessToken          string    `json:"accessToken"`
	AccessTokenExpiresAt time.Time `json:"accessTokenExpiresAt"`
	RefreshToken         string    `json:"refreshToken"`
	Session              string    `json:"session"`
}

// SessionClient calls core's session endpoints.
type SessionClient struct {
	api        *platform.APIClient
	deviceName string
}

func NewSessionClient(api *platform.APIClient, deviceName string) *SessionClient {
	return &SessionClient{api: api, deviceName: deviceName}
}

func (client *SessionClient) Login(ctx context.Context, email, password string) (SignInResponse, error) {
	return client.signIn(ctx, "/api/core/v1/auth/login", LoginRequest{
		Email: email, Password: password, Client: NativeClientType, DeviceName: client.deviceName,
	})
}

func (client *SessionClient) Refresh(ctx context.Context, refreshToken string) (SignInResponse, error) {
	return client.signIn(ctx, "/api/core/v1/auth/refresh", RefreshRequest{RefreshToken: refreshToken})
}

func (client *SessionClient) Logout(ctx context.Context, accessToken string) error {
	_, err := client.api.SendWithToken(ctx, platform.APIRequest{Method: http.MethodPost, Path: "/api/core/v1/auth/logout"}, accessToken)
	return err
}

func (client *SessionClient) signIn(ctx context.Context, path string, body any) (SignInResponse, error) {
	raw, err := client.api.SendWithToken(ctx, platform.APIRequest{Method: http.MethodPost, Path: path, Body: body}, "")
	if err != nil {
		return SignInResponse{}, err
	}
	var response SignInResponse
	if err := json.Unmarshal(raw, &response); err != nil || response.AccessToken == "" || response.RefreshToken == "" {
		return SignInResponse{}, fmt.Errorf("%w: unexpected sign-in response", platform.ErrInstanceUnavailable)
	}
	return response, nil
}
