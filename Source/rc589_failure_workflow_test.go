//go:build windows

package main

import "testing"

func TestRC589FailureBlankIsEmbedded(t *testing.T) {
	if len(failureBlankTemplateXLSX) < 4 || string(failureBlankTemplateXLSX[:2]) != "PK" {
		t.Fatal("approved failure workbook must be embedded into the standalone EXE")
	}
}

func TestRC589PreliminaryPageContainsAllPreliminaryFields(t *testing.T) {
	for _, index := range []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 16, 17, 18} {
		if section := wellCoreSection(index); section != 0 {
			t.Fatalf("core field %d moved outside preliminary page: section %d", index, section)
		}
	}
	for _, column := range []string{"H", "L", "Q", "R"} {
		found := false
		for _, spec := range wellExtraSpecs {
			if spec.column == column {
				found = spec.section == 0
				break
			}
		}
		if !found {
			t.Fatalf("preliminary workbook field %s is absent from first page", column)
		}
	}
	for _, spec := range wellExtraSpecs {
		if spec.column == "I" || spec.column == "J" {
			t.Fatalf("obsolete ORD field %s returned to the form", spec.column)
		}
	}
}

func TestRC589ExtendedWorkbookFieldsAreAvailable(t *testing.T) {
	wanted := []string{"U", "V", "W", "X", "Z", "AA", "AB", "AC", "AD", "AF", "AG", "AH", "AI", "AJ", "AK", "AL", "AM", "AN", "AO", "AP", "AQ", "AR", "AS", "AT", "AU", "AV", "AW", "AX", "AY", "AZ", "BA", "BB", "BC", "BD", "BE", "BF", "BG", "BH", "BI", "BJ", "BK", "BL", "BM", "BN"}
	have := map[string]bool{}
	for _, spec := range wellExtraSpecs {
		have[spec.column] = true
	}
	for _, column := range wanted {
		if !have[column] {
			t.Fatalf("manual extended investigation misses workbook column %s", column)
		}
	}
}
