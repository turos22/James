@echo off
docker compose down
docker compose up -d
npm run build --prefix web
start "jamesd" cmd /k go run .\cmd\jamesd
timeout /t 3 /nobreak >nul
cd shell
wails dev
