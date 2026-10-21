package coreclient_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/coreclient"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/coretest"
)

var ana = coreclient.DirectoryUser{Token: "ana-token", Name: "Ana", Email: "ana@example.com"}

func TestFindUserReturnsDirectoryEntry(t *testing.T) {
	core := coretest.NewFakeCore(t)
	core.AddUser(ana)

	user, err := coreclient.NewUserDirectoryClient(core.URL(), coreclient.DefaultTimeout).FindUser(context.Background(), "caller-token", ana.Token)

	if err != nil || user != ana {
		t.Errorf("user = %+v, err = %v", user, err)
	}
}

func TestFindUserMapsErrors(t *testing.T) {
	core := coretest.NewFakeCore(t)
	client := coreclient.NewUserDirectoryClient(core.URL(), coreclient.DefaultTimeout)

	if _, err := client.FindUser(context.Background(), "caller-token", "nobody"); !errors.Is(err, coreclient.ErrUserNotFound) {
		t.Errorf("unknown user error = %v", err)
	}
	if _, err := client.FindUser(context.Background(), "", "nobody"); !errors.Is(err, coreclient.ErrDirectoryForbidden) {
		t.Errorf("missing token error = %v", err)
	}
	core.FailUserDirectory()
	if _, err := client.FindUser(context.Background(), "caller-token", "nobody"); !errors.Is(err, coreclient.ErrCoreUnavailable) {
		t.Errorf("failing core error = %v", err)
	}
}

func TestFindUserTimesOut(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(slow.Close)

	_, err := coreclient.NewUserDirectoryClient(slow.URL, 20*time.Millisecond).FindUser(context.Background(), "caller-token", "ana")

	if !errors.Is(err, coreclient.ErrCoreUnavailable) {
		t.Errorf("error = %v", err)
	}
}

func TestFindUserRejectsMalformedResponse(t *testing.T) {
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	t.Cleanup(broken.Close)

	_, err := coreclient.NewUserDirectoryClient(broken.URL, coreclient.DefaultTimeout).FindUser(context.Background(), "caller-token", "ana")

	if !errors.Is(err, coreclient.ErrCoreUnavailable) {
		t.Errorf("error = %v", err)
	}
}
