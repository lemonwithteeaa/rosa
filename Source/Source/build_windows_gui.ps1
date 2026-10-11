$ErrorActionPreference = "Stop"
# Shared release contract: embed_icon_resource.py + verify_icon_resource.py,
# approved OF_Model_icon_round.ico. Version is read from main.go by Python.
python (Join-Path $PSScriptRoot "build_release.py")
if ($LASTEXITCODE -ne 0) { throw "Сборка или проверка РОСА не пройдена" }
