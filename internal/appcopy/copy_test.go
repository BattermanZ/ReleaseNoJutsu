package appcopy

import (
	"strings"
	"testing"
)

func TestCopy_CouldNotAddManga_DoesNotReferenceMissingListCommand(t *testing.T) {
	if strings.Contains(Copy.Errors.CouldNotAddManga, "/list") {
		t.Fatalf("CouldNotAddManga should not reference /list: %q", Copy.Errors.CouldNotAddManga)
	}
}

func TestCopy_NewChapterAlertFooter_ExplainsStartOpensMenu(t *testing.T) {
	if !strings.Contains(Copy.Info.NewChapterAlertFooter, "open the menu") {
		t.Fatalf("NewChapterAlertFooter should explain /start opens menu: %q", Copy.Info.NewChapterAlertFooter)
	}
	if !strings.Contains(Copy.Info.NewChapterAlertFooterPlain, "open the menu") {
		t.Fatalf("NewChapterAlertFooterPlain should explain /start opens menu: %q", Copy.Info.NewChapterAlertFooterPlain)
	}
}

func TestCopy_HelpText_IncludesGenPairCommand(t *testing.T) {
	if !strings.Contains(Copy.Info.HelpText, "/genpair") {
		t.Fatalf("HelpText should include /genpair guidance")
	}
}
