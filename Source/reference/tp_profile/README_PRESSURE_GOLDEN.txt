РОСА — ПОДГОТОВКА ВНЕШНЕГО PHREEQC-ЭТАЛОНА ДЛЯ ДАВЛЕНИЯ
Контрольная точка 7.3.11.140-RC

ЗАЧЕМ
В РОСА уже есть три точки T/P-профиля, но давление намеренно хранится только как исходное условие и НЕ меняет SI. Следующий разрешённый шаг по плану — получить отдельный воспроизводимый численный эталон PHREEQC для влияния pressure и только после этого решать, как включать pressure-поправку.

ЧТО ПОДГОТОВЛЕНО
1. pressure_reference.pqi — отдельная задача PHREEQC с одной фиксированной водой при 25 °C и четырьмя давлениями: 1, 100, 500 и 1000 atm.
2. В selected output должны попасть PRESSURE, температура и SI Calcite/Gypsum/Barite/Celestite.
3. verify_pressure_reference.py — проверяет структуру фактического selected output; он НЕ создаёт и НЕ подменяет результаты PHREEQC.
4. pressure_reference_manifest.txt — фиксирует версию, базу, ожидаемые давления и источники синтаксиса.

ВАЖНО
- В 7.3.11.140-RC НИКАКАЯ pressure-поправка в формулы РОСА не включена.
- Файл golden.tsv намеренно не создаётся до фактического прогона официального PHREEQC.
- Запрещено вручную заполнять golden или рассчитывать его формулами РОСА.
- После внешнего прогона сохранить: точную версию PHREEQC, хеш phreeqc.dat, input, selected output, полный output/stderr и команду запуска.

ОФИЦИАЛЬНАЯ ОСНОВА
USGS PHREEQC 3.8.6-17100.
REACTION_PRESSURE задаёт давления batch-reaction в atm.
SOLUTION поддерживает pressure.
В USER_PUNCH функция PRESSURE возвращает текущее давление раствора в atm.

Документация:
https://water.usgs.gov/water-resources/software/PHREEQC/documentation/phreeqc3-html/phreeqc3-41.htm
https://water.usgs.gov/water-resources/software/PHREEQC/documentation/phreeqc3-html/phreeqc3-48.htm
https://water.usgs.gov/water-resources/software/PHREEQC/documentation/phreeqc3-html/phreeqc3-61.htm

КРИТЕРИЙ ЗАВЕРШЕНИЯ СЛЕДУЮЩЕГО ШАГА
Фактически запущенный официальный PHREEQC должен сформировать pressure_selected.tsv; verify_pressure_reference.py должен пройти; результаты и хеши должны быть сохранены. Только затем можно сравнивать pressure-зависимость с ядром РОСА и обсуждать включение давления в SI.
