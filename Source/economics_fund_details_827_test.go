//go:build windows

package main

import (
 "strings"
 "testing"
)

func TestEconomicsFundDetailPreservesEveryLongValue(t *testing.T) {
 long := strings.Repeat("История фактических отказов и технологическое обоснование 2024—2026. ", 12)
 rows := [][]string{
  {"Прогноз МРП, сут.", "280"},
  {"Основание выбора", long},
  {"2024.02.01 · TPN-01-F1", "МРП 82 → 117 сут.; эффект -681.1 тыс. руб. / 12 мес.; CAPEX 269225; OPEX 100250"},
  {"Источник", "Формула ECON_BASE_EXCEL_2026_09; синтетические значения"},
 }
 result := economicsFundDetailsText(rows)
 for _, row := range rows {
  if !strings.Contains(result, row[0]) || !strings.Contains(result, row[1]) {
   t.Fatalf("detail lost unwrapped text for %q", row[0])
  }
 }
 if !strings.Contains(result, "\r\n\r\n") {
  t.Fatal("long value should be a separate wrapped paragraph")
 }
 if got := economicsFundDetailsText(nil); got != "" {
  t.Fatalf("empty details should be empty, got %q", got)
 }
}
