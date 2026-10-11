package main

// All primary sections share the same upper notice strip. Reserve the right
// edge for the training-data note and keep it clear of the caution text.
func sectionNoticeLayout(clientWidth int32) (textWidth, labelX int32) {
	labelX = clientWidth - 300
	textWidth = labelX - 58 - 14
	if textWidth < 0 {
		textWidth = 0
	}
	return
}
