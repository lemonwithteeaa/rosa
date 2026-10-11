//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC580MethodsReferenceUXContract(t *testing.T) {
	b, err := os.ReadFile("methods_reference_dialog.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	wants := []string{
		"ROSAMethodsReferenceDialogV5",
		"methodsReferenceSourceListHandle",
		"openMethodsReferenceSourceLinkPopup",
		"methodsSourcePopupCopy",
		"WM_COPY",
		"notify == 2",
		"populateAllMethodsReferenceSources",
	}
	for _, want := range wants {
		if !strings.Contains(s, want) {
			t.Errorf("RC580 missing %q", want)
		}
	}
	for _, forbidden := range []string{"methodsReferenceCloseButton", "methodsReferenceMinimizeButton", "methodsReferenceMaxRestoreButton"} {
		if strings.Contains(s, forbidden) {
			t.Errorf("RC580 legacy bottom window control returned: %s", forbidden)
		}
	}
	if strings.Contains(s, `strings.Index(t, ":")`) {
		t.Error("RC580 must not bold arbitrary inline labels before colon")
	}
}
