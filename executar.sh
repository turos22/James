#!/usr/bin/env bash
# ============================================================
#  James - subida completa a partir do zero, no Linux.
#  Espelho do executar.bat; os comentarios de armadilha de la valem aqui.
#  Pre-requisito: daemon do Docker rodando e usuario no grupo docker.
#
#    1. pre-requisitos
#    2. Postgres (docker compose) e espera ficar saudavel
#    3. build da SPA -> precisa vir ANTES do jamesd (go:embed)
#    4. jamesd e james-device em segundo plano, logs em .executar/
#    5. shell Wails se der; senao o navegador. Ctrl+C encerra tudo.
# ============================================================
set -euo pipefail

# Sempre a partir da raiz do repo: o jamesd le config.toml, prompts/ e
# grava em audios/ por caminho relativo.
cd "$(dirname "$(readlink -f "$0")")"

LOGS=.executar
mkdir -p "$LOGS"
PIDS=()

encerrar() {
    for pid in "${PIDS[@]}"; do
        kill "$pid" 2>/dev/null || true
    done
}
trap encerrar EXIT
trap 'exit 130' INT TERM

falhar() {
    echo "  ERRO: $*" >&2
    echo
    echo "Subida abortada."
    exit 1
}

echo
echo "[1/5] Conferindo pre-requisitos..."
[ -f .env ] || falhar ".env nao encontrado na raiz. Copie .env.example para .env e preencha GEMINI_API_KEY, TAVILY_API_KEY, POSTGRES_PASSWORD e DATABASE_URL."
docker info >/dev/null 2>&1 || falhar "o Docker nao respondeu. Rode: sudo systemctl start docker"
echo "  ok"

echo
echo "[2/5] Subindo o Postgres..."
docker compose up -d || falhar "docker compose falhou."

# Esperar de verdade: num boot frio o Postgres leva segundos para aceitar
# conexao, e o jamesd morre no arranque se subir antes.
for tentativa in $(seq 1 60); do
    docker exec james-postgres pg_isready -U james -d james >/dev/null 2>&1 && break
    [ "$tentativa" -eq 60 ] && falhar "o Postgres nao ficou saudavel em 60s. Veja: docker logs james-postgres"
    sleep 1
done

# pg_isready roda DENTRO do container e nao prova que a 5434 chega nele.
# Ver o comentario longo no executar.bat: outro Postgres com a porta roubada
# vira "password authentication failed", que aponta para senha sem ser.
if ! docker ps --filter name=james-postgres --format '{{.Ports}}' | grep -q 5434; then
    echo "  Quem esta com a 5434 no momento:"
    docker ps --format '     {{.Names}}  ->  {{.Ports}}' | grep 5434 || true
    falhar "o james-postgres esta de pe, mas NAO publica a porta 5434. Libere a porta (docker stop <container>) ou troque em docker-compose.yml E no DATABASE_URL do .env."
fi
echo "  ok - james-postgres saudavel e publicando a 5434"

echo
echo "[3/5] Compilando a SPA e o backend..."
if [ ! -d web/node_modules ]; then
    echo "  node_modules ausente, instalando dependencias do front..."
    npm install --prefix web || falhar "npm install falhou."
fi
# ARMADILHA: web/embed.go usa //go:embed all:dist. Pular este build serve a
# interface antiga sem nenhum aviso.
npm run build --prefix web || falhar "o build da SPA falhou. O jamesd serviria a interface velha."
# Binario, e nao go run: go run deixa o processo real como neto, e o kill
# do trap so pegaria o go.
go build -o "$LOGS/jamesd" ./cmd/jamesd || falhar "o codigo Go nao compila."
echo "  ok"

echo
echo "[4/5] Subindo jamesd e james-device..."
"$LOGS/jamesd" >"$LOGS/jamesd.log" 2>&1 &
PIDS+=($!)
PID_JAMESD=$!

for tentativa in $(seq 1 60); do
    curl -s -m 2 http://127.0.0.1:8080/api/saude >/dev/null 2>&1 && break
    if ! kill -0 "$PID_JAMESD" 2>/dev/null; then
        tail -n 20 "$LOGS/jamesd.log" >&2
        falhar "o jamesd morreu no arranque. Log completo: $LOGS/jamesd.log"
    fi
    [ "$tentativa" -eq 60 ] && falhar "o jamesd nao respondeu em 60s. Veja $LOGS/jamesd.log"
    sleep 1
done
echo "  ok - jamesd de pe em http://127.0.0.1:8080 (log: $LOGS/jamesd.log)"

# cwd na raiz de proposito: o device le dispositivo/assets/ por caminho
# relativo. Sem microfone ele reclama no log e o resto segue pelo teclado.
PYTHON_DEVICE=dispositivo/.venv/bin/python
[ -x "$PYTHON_DEVICE" ] || PYTHON_DEVICE=python3
"$PYTHON_DEVICE" -m dispositivo.james_device.main >"$LOGS/device.log" 2>&1 &
PIDS+=($!)
echo "  ok - james-device em segundo plano (log: $LOGS/device.log)"

echo
echo "[5/5] Abrindo a interface (Ctrl+C encerra tudo)..."

abrir_no_navegador() {
    echo "  abrindo no navegador: http://127.0.0.1:8080"
    xdg-open http://127.0.0.1:8080 >/dev/null 2>&1 || true
    wait "$PID_JAMESD"
}

# Qual janela abrir. JAMES_SHELL=chrome|wails|navegador força uma; sem ele,
# a ordem é chrome, wails, navegador.
#
# Por que o Chrome vem primeiro: o WebKitGTK do shell Wails bloqueia o
# autoplay sem gesto do usuário (medido: play() rejeita com NotAllowedError e
# o AudioContext fica suspenso), e o Wails 2.11 não expõe essa configuração.
# Num turno por voz ninguém clica na janela, então o James responderia mudo.
# O Chrome com --autoplay-policy=no-user-gesture-required toca (medido). Perfil
# próprio em $LOGS, pra flag não afetar o seu Chrome de uso diário. O stderr
# vai pra $LOGS/chrome.log: o Chrome despeja ruído (registro GCM, TensorFlow
# Lite, libwayland no desligamento) que não é do James e afoga o terminal.
abrir_chrome() {
    google-chrome --user-data-dir="$LOGS/chrome-perfil" --no-first-run \
        --no-default-browser-check --autoplay-policy=no-user-gesture-required \
        --app=http://127.0.0.1:8080/ 2>"$LOGS/chrome.log"
}

# Wails: compila com go build e não com o CLI do wails (o CLI v2.11.0 quebra
# com Go 1.27, "package context without types"; o go build com as tags abaixo
# funciona). O shell é só uma janela que abre http://127.0.0.1:8080 (ver
# shell/frontend/index.html).
abrir_wails() {
    pkg-config --exists webkit2gtk-4.1 2>/dev/null || return 1
    go build -tags desktop,production,webkit2_41 -o "$LOGS/shell" ./shell || return 1
    # cwd em shell/ de proposito, como no .bat (pushd shell).
    (cd shell && "../$LOGS/shell")
}

escolha="${JAMES_SHELL:-}"
if [ -z "$escolha" ]; then
    if command -v google-chrome >/dev/null 2>&1; then escolha=chrome
    elif pkg-config --exists webkit2gtk-4.1 2>/dev/null; then escolha=wails
    else escolha=navegador; fi
fi
echo "  janela: $escolha"

# Janela fechada = saida limpa = o script termina e o trap derruba o resto.
# Se a janela caiu com erro, o navegador e o plano B (e o backend segue).
case "$escolha" in
    chrome) abrir_chrome || { echo "  o Chrome caiu; seguindo no navegador."; abrir_no_navegador; } ;;
    wails)  abrir_wails  || { echo "  o shell nativo caiu; seguindo no navegador."; abrir_no_navegador; } ;;
    *)      abrir_no_navegador ;;
esac
