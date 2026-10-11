package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC322BuildMarker(t *testing.T) {
	b, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `const appBuildVersion = "7.3.11.`) || !strings.Contains(s, `-RC"`) {
		t.Fatal("current RC build marker missing")
	}
}

func TestRC322ArchiveIsInAdaptiveNavigationOrder(t *testing.T) {
	b, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	want := `navOrder := []int{ID_DASH, ID_OBJECTS, ID_LAB, ID_ASSESS, ID_ACTIVITIES, ID_MRP, ID_ECON, ID_RESULT, ID_ARCHIVE}`
	if !strings.Contains(string(b), want) {
		t.Fatal("8. Утверждённый реестр отсутствует в адаптивной раскладке бокового меню")
	}
}

func TestRC322ORPUsesFieldSpecificCompactFont(t *testing.T) {
	b, err := os.ReadFile("laboratory.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `if longCaption && hFontHeaderSmall != 0`) {
		t.Fatal("для длинных лабораторных заголовков не закреплён компактный шрифт")
	}
	if !strings.Contains(s, `else if hFontBold != 0`) {
		t.Fatal("после ОВП не восстанавливается стандартный шрифт остальных полей")
	}
}

func TestRC322ArchiveTempGuard(t *testing.T) {
	b, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	s := strings.ToLower(string(b))
	for _, token := range []string{`runningfromarchivetemp`, `rar$exa`, `распакуйте архив`} {
		if !strings.Contains(s, token) {
			t.Fatalf("archive temp guard missing %q", token)
		}
	}
}
