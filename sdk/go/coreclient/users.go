// Package coreclient calls core's public API on behalf of the user who made the module request.
package coreclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const DefaultTimeout = 3 * time.Second

var (
	ErrUserNotFound       = errors.New("user not found in core")
	ErrDirectoryForbidden = errors.New("caller may not read core's user directory")
	ErrCoreUnavailable    = errors.New("core is unavailable")
)

type DirectoryUser struct {
	Token       string `json:"token"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	AvatarColor string `json:"avatarColor"`
}

type UserDirectoryClient struct {
	coreURL    string
	httpClient *http.Client
}

func NewUserDirectoryClient(coreURL string, timeout time.Duration) *UserDirectoryClient {
	return &UserDirectoryClient{coreURL: strings.TrimRight(coreURL, "/"), httpClient: &http.Client{Timeout: timeout}}
}

// FindUser forwards the caller's access token, so core applies the caller's own permissions.
func (client *UserDirectoryClient) FindUser(ctx context.Context, accessToken, userToken string) (DirectoryUser, error) {
	endpoint := client.coreURL + "/api/core/v1/users/" + url.PathEscape(userToken)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return DirectoryUser{}, fmt.Errorf("build user request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	response, err := client.httpClient.Do(request)
	if err != nil {
		return DirectoryUser{}, fmt.Errorf("%w: %v", ErrCoreUnavailable, err)
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusOK:
		var user DirectoryUser
		if err := json.NewDecoder(response.Body).Decode(&user); err != nil {
			return DirectoryUser{}, fmt.Errorf("%w: decode user: %v", ErrCoreUnavailable, err)
		}
		return user, nil
	case http.StatusNotFound:
		return DirectoryUser{}, ErrUserNotFound
	case http.StatusUnauthorized, http.StatusForbidden:
		return DirectoryUser{}, ErrDirectoryForbidden
	default:
		return DirectoryUser{}, fmt.Errorf("%w: status %d", ErrCoreUnavailable, response.StatusCode)
	}
}
