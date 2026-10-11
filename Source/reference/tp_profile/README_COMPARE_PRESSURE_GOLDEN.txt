СРАВНЕНИЕ PRESSURE-GOLDEN С ТЕКУЩЕЙ РОСА

1. Сначала выполните RUN_PRESSURE_REFERENCE.cmd официальным USGS PHREEQC.
2. Затем ACCEPT_PRESSURE_GOLDEN.cmd. Только после успешного gate появятся pressure_golden.tsv и pressure_golden_manifest.txt.
3. После этого запустите COMPARE_PRESSURE_GOLDEN.cmd.
4. Скрипт НЕ придумывает pressure-поправку и НЕ изменяет код расчёта РОСА.
5. Он сравнивает внешнее изменение SI относительно 1 atm с текущим pressure-response РОСА, который пока равен нулю по проектному ограничению.
6. Результат записывается в pressure_comparison_report.txt.
7. Если внешний PHREEQC показывает заметное изменение SI, отчёт ставит decision=PRESSURE_EFFECT_DETECTED_EXTERNALLY__ROSA_IMPLEMENTATION_REQUIRED_BEFORE_ENABLE.
8. Это gate перед будущей реализацией: наличие golden не разрешает автоматически добавлять эмпирическую формулу.
