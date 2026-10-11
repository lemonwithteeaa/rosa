//go:build windows

package main

import _ "embed"

//go:embed РОСА_БЛАНК_ЛАБОРАТОРНОГО_АНАЛИЗА.xlsx
var laboratoryBlankTemplateXLSX []byte
