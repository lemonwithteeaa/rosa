package main

import (
	"fmt"
	"strings"
)

func assessmentXMLText(value string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&apos;").Replace(value)
}

// assessmentApprovalDOCXTable renders the signed complication decisions in a
// compact seven-column grid. Existing approved snapshots are read-only.
func assessmentApprovalDOCXTable(rows []assessmentExportRow, sharedComment string) string {
	if len(rows) == 0 {
		return ""
	}
	headers := []string{"Осложнение", "Программа", "Технолог", "Статус", "Комментарий", "Утвердил", "Дата"}
	widths := []int{1380, 1100, 1100, 1450, 2510, 1190, 850}
	var b strings.Builder
	b.WriteString(`<w:tbl><w:tblPr><w:tblW w:w="9580" w:type="dxa"/><w:tblLayout w:type="fixed"/><w:tblBorders>`)
	b.WriteString(`<w:top w:val="single" w:sz="5" w:color="CAD4E2"/><w:left w:val="single" w:sz="5" w:color="CAD4E2"/><w:bottom w:val="single" w:sz="5" w:color="CAD4E2"/><w:right w:val="single" w:sz="5" w:color="CAD4E2"/><w:insideH w:val="single" w:sz="4" w:color="DCE4EE"/><w:insideV w:val="single" w:sz="4" w:color="DCE4EE"/></w:tblBorders>`)
	b.WriteString(`<w:tblCellMar><w:top w:w="75" w:type="dxa"/><w:left w:w="65" w:type="dxa"/><w:bottom w:w="75" w:type="dxa"/><w:right w:w="65" w:type="dxa"/></w:tblCellMar></w:tblPr><w:tblGrid>`)
	for _, width := range widths {
		fmt.Fprintf(&b, `<w:gridCol w:w="%d"/>`, width)
	}
	b.WriteString(`</w:tblGrid>`)
	cell := func(value string, width int, heading bool, alternate bool) string {
		fill := "FFFFFF"
		color := "1D3654"
		bold := ""
		if heading {
			fill = "173D6C"
			color = "FFFFFF"
			bold = `<w:b/>`
		} else if alternate {
			fill = "F1F5FA"
		}
		value = strings.TrimSpace(value)
		if value == "" {
			value = "—"
		}
		return fmt.Sprintf(`<w:tc><w:tcPr><w:tcW w:w="%d" w:type="dxa"/><w:shd w:val="clear" w:fill="%s"/><w:vAlign w:val="center"/></w:tcPr><w:p><w:pPr><w:spacing w:before="0" w:after="0" w:line="220" w:lineRule="auto"/></w:pPr><w:r><w:rPr>%s<w:color w:val="%s"/><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri"/><w:sz w:val="17"/></w:rPr><w:t xml:space="preserve">%s</w:t></w:r></w:p></w:tc>`, width, fill, bold, color, assessmentXMLText(value))
	}
	b.WriteString(`<w:tr><w:trPr><w:tblHeader w:val="true"/><w:cantSplit/></w:trPr>`)
	for i, h := range headers {
		b.WriteString(cell(h, widths[i], true, false))
	}
	b.WriteString(`</w:tr>`)
	for i, row := range rows {
		b.WriteString(`<w:tr><w:trPr><w:cantSplit/></w:trPr>`)
		values := []string{row.Complication, row.Program, row.Technologist, row.Status, row.Comment, row.ApprovedBy, row.ApprovedAt}
		for j, v := range values {
			b.WriteString(cell(v, widths[j], false, i%2 != 0))
		}
		b.WriteString(`</w:tr>`)
	}
	b.WriteString(`</w:tbl>`)
	if strings.TrimSpace(sharedComment) != "" {
		b.WriteString(`<w:p><w:pPr><w:spacing w:before="80" w:after="75"/></w:pPr><w:r><w:rPr><w:b/><w:sz w:val="19"/></w:rPr><w:t>Общий комментарий: </w:t></w:r><w:r><w:rPr><w:sz w:val="19"/></w:rPr><w:t xml:space="preserve">` + assessmentXMLText(sharedComment) + `</w:t></w:r></w:p>`)
	}
	return b.String()
}
