РОСА — внешний pressure benchmark PHREEQC

1. Нужен официальный USGS PHREEQC 3.8.6-17100.
2. Запустите RUN_PRESSURE_REFERENCE.cmd. Скрипт ищет phreeqc.exe в таком порядке: явный параметр -PhreeqcExe; переменная PHREEQC_EXE; рядом со скриптом; папки engine\ и portable\; PATH; стандартные папки USGS.
3. Если PHREEQC установлен нестандартно:
   powershell -ExecutionPolicy Bypass -File .\run_pressure_reference.ps1 -PhreeqcExe "C:\...\phreeqc.exe"
4. Используется локальная копия phreeqc.dat. Её SHA-256 фиксируется в pressure_execution_manifest.txt.
5. Успешный прогон создаёт:
   - pressure_full_output.txt
   - pressure_selected.tsv
   - pressure_execution_manifest.txt
6. verify_pressure_reference.py проверяет наличие четырёх давлений 1/100/500/1000 atm и конечные SI Calcite/Gypsum/Barite/Celestite.
7. Даже после успешного прогона pressure-поправка в РОСА автоматически НЕ включается. Сначала результат должен быть принят как внешний golden и сопоставлен с ядром РОСА.

Официальная страница загрузки:
https://water.usgs.gov/water-resources/software/PHREEQC/

8. После успешного внешнего прогона запустите ACCEPT_PRESSURE_GOLDEN.cmd. Он НЕ создаёт значения сам: повторно запускает verifier, сверяет SHA-256 input/database и версию engine, после чего копирует подтверждённый pressure_selected.tsv в pressure_golden.tsv и создаёт pressure_golden_manifest.txt.
9. Даже принятый golden ещё не включает давление в расчёты РОСА автоматически: следующим отдельным шагом выполняется численное сравнение с ядром и только затем явная реализация pressure-зависимости.
