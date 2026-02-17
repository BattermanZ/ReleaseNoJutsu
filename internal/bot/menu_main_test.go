package bot

import (
	"fmt"
	"strings"
	"testing"

	"releasenojutsu/internal/appcopy"
)

func TestSendStatusMessage_NonAdmin_HidesGlobalAccountCount(t *testing.T) {
	b, database, api := setupBotForMessageTests(t)

	userID := int64(42)
	if err := database.EnsureUser(userID, false); err != nil {
		t.Fatalf("EnsureUser(user): %v", err)
	}
	if _, err := database.AddManga("40bc649f-7b49-4645-859e-6cd94136e722", "Dragon Ball", userID); err != nil {
		t.Fatalf("AddManga(): %v", err)
	}

	b.sendStatusMessage(userID, userID)
	got := api.lastMessageText(t)

	if strings.Contains(got, "Total authorized accounts:") {
		t.Fatalf("non-admin status should not show global account count, got: %q", got)
	}
}

func TestSendStatusMessage_AdminShowsGlobalAccountCount(t *testing.T) {
	b, database, api := setupBotForMessageTests(t)

	adminID := int64(1)
	otherUserID := int64(42)
	if err := database.EnsureUser(adminID, true); err != nil {
		t.Fatalf("EnsureUser(admin): %v", err)
	}
	if err := database.EnsureUser(otherUserID, false); err != nil {
		t.Fatalf("EnsureUser(other): %v", err)
	}

	b.sendStatusMessage(adminID, adminID)
	got := api.lastMessageText(t)

	wantLine := fmt.Sprintf(appcopy.Copy.Info.StatusRegisteredChats, 2)
	if !strings.Contains(got, wantLine) {
		t.Fatalf("admin status should include global account count line %q, got: %q", wantLine, got)
	}
}
