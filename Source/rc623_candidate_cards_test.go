package main

import "testing"

func TestRC623LongMethodTitleKeepsMeasuredHeight(t *testing.T) {
	title, detail := methodsCandidateCardHeights(44, 28, 108)
	if title < 54 {
		t.Fatalf("three-line method title was cropped: title height %d", title)
	}
	if detail < 36 {
		t.Fatalf("status plaque was cropped: detail height %d", detail)
	}
	if title+detail > 108 {
		t.Fatalf("card allocation overflowed: %d + %d", title, detail)
	}
}

func TestRC623CandidateCardHeightDegradesWithoutOverflow(t *testing.T) {
	title, detail := methodsCandidateCardHeights(60, 34, 76)
	if title < 34 || detail < 1 || title+detail > 76 {
		t.Fatalf("invalid compact allocation: title=%d detail=%d", title, detail)
	}
}
