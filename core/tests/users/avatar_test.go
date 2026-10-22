package users_test

import (
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/users"
)

func TestAvatarColorIsDrawnFromTheCreationInstant(t *testing.T) {
	createdAt := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	for offset, expected := range users.AvatarColors {
		user := users.User{CreatedAt: createdAt.Add(time.Duration(offset) * time.Microsecond)}
		if color := user.AvatarColor(); color != expected {
			t.Errorf("created %dµs later: AvatarColor() = %s, want %s", offset, color, expected)
		}
	}
}

func TestAvatarColorNeverChangesForTheSameUser(t *testing.T) {
	user := users.User{CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 678901000, time.UTC)}
	if first, second := user.AvatarColor(), user.AvatarColor(); first != second {
		t.Errorf("AvatarColor() = %s then %s", first, second)
	}
}
