#!/usr/bin/env bash
# ============================================================
#  James - subida completa a partir do zero, no Linux.
#  Espelho do executar.bat; os comentarios de armadilha de la valem aqui.
#  Pre-requisito: daemon do Docker rodando e usuario no grupo docker.
#
#    1. pre-requisitos
#    2. Postgres (docker compose) e espera ficar saudavel
#    3. build da SPA -> precisa vir ANTES do jamesd (go:embed)
#    4. STT (whisper-server) e TTS (Piper) locais, depois jamesd e
#       james-device em segundo plano, logs em .executar/
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
echo "[4/5] Subindo o STT e o TTS locais, o jamesd e o james-device..."

PYTHON_DEVICE=dispositivo/.venv/bin/python
[ -x "$PYTHON_DEVICE" ] || PYTHON_DEVICE=python3

# Espera um endpoint HTTP responder: esperar_http <url> <segundos>
esperar_http() {
    local url=$1 segundos=$2
    for _ in $(seq 1 "$segundos"); do
        curl -s -m 2 -o /dev/null "$url" && return 0
        sleep 1
    done
    return 1
}

# --- STT: whisper-server (whisper.cpp, Vulkan, modelo sempre carregado) ---
# O -ac vem de JAMES_WHISPER_AC e o device le a MESMA variavel: ele recusa um
# teto de comando maior que o que o -ac cobre (10 s com 512). Mudar so de um
# lado cortaria o fim dos comandos em silencio.
export JAMES_WHISPER_AC="${JAMES_WHISPER_AC:-512}"
WHISPER_URL="${JAMES_WHISPER_URL:-http://127.0.0.1:8179}"
WHISPER_SERVIDOR="${JAMES_WHISPER_SERVIDOR:-$HOME/.local/share/james/whisper.cpp/bin/whisper-server}"
WHISPER_MODELO="${JAMES_WHISPER_MODELO:-$HOME/.local/share/james/modelos/ggml-large-v3-turbo-q8_0.bin}"

if [ "${JAMES_STT:-whisper}" = "whisper" ]; then
    if esperar_http "$WHISPER_URL/" 1; then
        # Reaproveitar so vale se ele subiu com o MESMO -ac: o device valida o
        # teto de comando contra JAMES_WHISPER_AC, e um servidor velho com -ac
        # menor cortaria o fim dos comandos longos sem nenhum aviso.
        PID_VELHO=$(ss -ltnpH "sport = :${WHISPER_URL##*:}" 2>/dev/null | sed -n 's/.*pid=\([0-9]*\).*/\1/p' | head -1)
        if [ -z "$PID_VELHO" ] || [ ! -r "/proc/$PID_VELHO/cmdline" ]; then
            echo "  AVISO: ja ha um whisper-server em $WHISPER_URL e nao consegui conferir o -ac dele; reaproveitando."
        elif tr '\0' ' ' < "/proc/$PID_VELHO/cmdline" | grep -q -- " -ac $JAMES_WHISPER_AC "; then
            echo "  ok - whisper-server ja estava de pe em $WHISPER_URL com -ac $JAMES_WHISPER_AC (reaproveitando)"
        else
            falhar "ja ha um whisper-server em $WHISPER_URL (pid $PID_VELHO) com outro -ac. Pare-o (kill $PID_VELHO) e rode de novo, ou ajuste JAMES_WHISPER_AC."
        fi
    elif [ -x "$WHISPER_SERVIDOR" ] && [ -f "$WHISPER_MODELO" ]; then
        # As .so do whisper.cpp ficam ao lado do binario.
        LD_LIBRARY_PATH="$(dirname "$WHISPER_SERVIDOR")${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}" \
            "$WHISPER_SERVIDOR" -m "$WHISPER_MODELO" -l pt -t "${JAMES_WHISPER_THREADS:-6}" \
            -ac "$JAMES_WHISPER_AC" --port "${WHISPER_URL##*:}" >"$LOGS/whisper.log" 2>&1 &
        PIDS+=($!)
        PID_WHISPER=$!
        if esperar_http "$WHISPER_URL/" 60 && kill -0 "$PID_WHISPER" 2>/dev/null; then
            echo "  ok - whisper-server de pe em $WHISPER_URL, -ac $JAMES_WHISPER_AC (log: $LOGS/whisper.log)"
        else
            echo "  AVISO: o whisper-server nao subiu (veja $LOGS/whisper.log); o device usa o STT do Google."
            export JAMES_STT=google
        fi
    else
        echo "  AVISO: whisper-server ou modelo ausente ($WHISPER_SERVIDOR / $WHISPER_MODELO); o device usa o STT do Google."
        export JAMES_STT=google
    fi
else
    echo "  JAMES_STT=${JAMES_STT}: whisper-server nao sobe"
fi

# --- TTS: voz do Piper, para as respostas do turno de TEXTO ---
# O jamesd le a porta de [voz] tts_url no config.toml (8180 por padrao).
TTS_PORTA="${JAMES_TTS_PORTA:-8180}"
if esperar_http "http://127.0.0.1:$TTS_PORTA/saude" 1; then
    echo "  ok - servidor de TTS ja estava de pe na porta $TTS_PORTA (reaproveitando)"
else
    "$PYTHON_DEVICE" -m dispositivo.james_device.servidor_tts >"$LOGS/tts.log" 2>&1 &
    PIDS+=($!)
    PID_TTS=$!
    if esperar_http "http://127.0.0.1:$TTS_PORTA/saude" 30 && kill -0 "$PID_TTS" 2>/dev/null; then
        echo "  ok - servidor de TTS de pe na porta $TTS_PORTA (log: $LOGS/tts.log)"
    else
        echo "  AVISO: o servidor de TTS nao subiu (veja $LOGS/tts.log); as respostas digitadas ficam sem voz. A voz por microfone nao depende dele."
    fi
fi

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

# Qual janela abrir. JAMES_SHELL=chrome|wails|navegador|nenhum força uma; sem ele,
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
    nenhum) echo "  sem janela (JAMES_SHELL=nenhum); Ctrl+C encerra tudo."; wait "$PID_JAMESD" ;;
    *)      abrir_no_navegador ;;
esac
