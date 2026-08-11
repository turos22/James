@echo off
setlocal
rem ============================================================
rem  James - subida completa a partir do zero.
rem  Pre-requisito unico: Docker Desktop ja rodando.
rem
rem  Sobe, nesta ordem (a ordem importa, ver comentarios):
rem    1. Postgres (docker compose) e espera ficar saudavel
rem    2. build da SPA  -> precisa vir ANTES do jamesd
rem    3. jamesd        -> janela propria
rem    4. james-device  -> janela propria (voz e tela)
rem    5. shell Wails   -> nesta janela; fechar encerra o script
rem ============================================================

rem Roda sempre a partir da raiz do repo, nao de onde foi chamado:
rem o jamesd le config.toml, prompts/sistema.md e grava em audios/
rem por caminho relativo.
cd /d "%~dp0"

echo.
echo [1/5] Conferindo pre-requisitos...

if not exist ".env" (
    echo   ERRO: .env nao encontrado na raiz.
    echo   Copie .env.example para .env e preencha GEMINI_API_KEY,
    echo   TAVILY_API_KEY, POSTGRES_PASSWORD e DATABASE_URL.
    goto :falhou
)

docker info >nul 2>&1
if errorlevel 1 (
    echo   ERRO: o Docker nao respondeu.
    echo   Abra o Docker Desktop e espere ficar verde antes de rodar isto.
    goto :falhou
)
echo   ok

echo.
echo [2/5] Subindo o Postgres...

rem up -d e idempotente: container ja saudavel vira no-op. Sem "down"
rem de proposito - recriar o container a cada execucao custa ~10s e nao
rem limpa nada (o volume james_pgdata sobrevive de qualquer jeito).
docker compose up -d
if errorlevel 1 (
    echo   ERRO: docker compose falhou.
    goto :falhou
)

rem Esperar de verdade, e nao um "timeout /t 3" no escuro: num boot frio
rem o Postgres leva alguns segundos para aceitar conexao, e o jamesd
rem morre no arranque se subir antes disso.
set TENTATIVAS=0
:aguardar_postgres
docker exec james-postgres pg_isready -U james -d james >nul 2>&1
if not errorlevel 1 goto :postgres_pronto
set /a TENTATIVAS+=1
if %TENTATIVAS% geq 60 (
    echo   ERRO: o Postgres nao ficou saudavel em 60s.
    echo   Veja: docker logs james-postgres
    goto :falhou
)
timeout /t 1 /nobreak >nul
goto :aguardar_postgres

:postgres_pronto
echo   ok - james-postgres saudavel

echo.
echo [3/5] Compilando a SPA e o backend...

if not exist "web\node_modules" (
    echo   node_modules ausente, instalando dependencias do front...
    call npm install --prefix web
    if errorlevel 1 (
        echo   ERRO: npm install falhou.
        goto :falhou
    )
)

rem ARMADILHA: web/embed.go usa //go:embed all:dist, entao a SPA e
rem compilada DENTRO do binario do jamesd. Pular este build (ou rodar o
rem jamesd antes dele) serve a interface antiga sem nenhum aviso.
call npm run build --prefix web
if errorlevel 1 (
    echo   ERRO: o build da SPA falhou. O jamesd serviria a interface velha.
    goto :falhou
)

rem Compila aqui, na janela que voce esta olhando: erro de compilacao
rem numa janela filha some junto com ela. Nao gera artefato - go build
rem sem -o descarta os binarios e so aquece o cache.
go build ./...
if errorlevel 1 (
    echo   ERRO: o codigo Go nao compila.
    goto :falhou
)
echo   ok

echo.
echo [4/5] Subindo jamesd e james-device...

start "jamesd" cmd /k go run .\cmd\jamesd

rem O device conecta em ws://127.0.0.1:8080/ws/dispositivo. Ele reconecta
rem sozinho a cada 2s (conexao.py, laco de executar), entao subir antes do
rem jamesd nao o deixa fora pra sempre - so atrasa. Ainda assim esperamos o
rem /api/saude responder: sem isso a janela do device abre cuspindo erro de
rem conexao por varios segundos, e isso mascara falha de verdade.
set TENTATIVAS=0
:aguardar_jamesd
curl -s -m 2 http://127.0.0.1:8080/api/saude >nul 2>&1
if not errorlevel 1 goto :jamesd_pronto
set /a TENTATIVAS+=1
if %TENTATIVAS% geq 60 (
    echo   ERRO: o jamesd nao respondeu em 60s. Veja a janela "jamesd".
    goto :falhou
)
timeout /t 1 /nobreak >nul
goto :aguardar_jamesd

:jamesd_pronto
echo   ok - jamesd de pe em http://127.0.0.1:8080

rem cwd na raiz de proposito: o device resolve o pacote james_device
rem pelo diretorio dispositivo/ e le assets/ por caminho relativo.
rem Depende de microfone real - sem microfone esta janela reclama e o
rem resto do sistema segue funcionando normalmente pelo teclado.
start "james-device" cmd /k python -m dispositivo.james_device.main

echo.
echo [5/5] Abrindo o shell Wails (fechar esta janela encerra tudo)...
echo.

pushd shell
wails dev
popd

echo.
echo Shell encerrado. As janelas "jamesd" e "james-device" continuam
echo abertas - feche na mao, ou rode: docker compose stop
goto :fim

:falhou
echo.
echo Subida abortada.
exit /b 1

:fim
endlocal
