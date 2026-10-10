"""Spike 2: pipeline otimizado (STT na GPU + LLM com streaming + TTS com streaming).

Mede cada etapa isolada e o encadeamento real. Não há chave da Anthropic nesta
máquina, então o LLM rápido aqui é o Gemini 2.5 Flash com thinking desligado,
como substituto declarado do Claude Haiku 5.5.

W = espera de silêncio que fecha a frase (parâmetro de desenho, não medido).
"""

import asyncio
import json
import re
import statistics
import sys
import time
from pathlib import Path

import edge_tts
import requests
from google import genai
from google.genai import types

AQUI = Path(__file__).parent
ENV = Path("/home/arthurorosco/SI/Projetos/Athena(Go)/meu-app-ts/.env")
RODADAS = 5
W = 0.7
VOZ = "en-US-AndrewMultilingualNeural"
SISTEMA = "Responda sempre em português do Brasil, em uma ou duas frases curtas."
FIM_DE_FRASE = re.compile(r"[.!?](\s|$)")


def chave() -> str:
    for linha in ENV.read_text().splitlines():
        if linha.startswith("GEMINI_API_KEY="):
            return linha.split("=", 1)[1].strip().strip('"').strip("'")
    raise SystemExit("GEMINI_API_KEY ausente do .env")


def resumo(nome, valores):
    print(f"  {nome}: mediana={statistics.median(valores):.3f}s min={min(valores):.3f}s max={max(valores):.3f}s (n={len(valores)})")


def stt(porta: int) -> tuple[float, str]:
    inicio = time.monotonic()
    with open(AQUI / "pergunta.wav", "rb") as f:
        r = requests.post(f"http://127.0.0.1:{porta}/inference",
                          files={"file": ("pergunta.wav", f, "audio/wav")},
                          data={"response_format": "json", "temperature": "0.0"}, timeout=60)
    return time.monotonic() - inicio, r.json().get("text", "").strip()


def llm_primeira_frase(cliente, texto: str, modelo: str) -> tuple[float, float, str]:
    inicio = time.monotonic()
    primeiro_pedaco = None
    acumulado = ""
    config = types.GenerateContentConfig(system_instruction=SISTEMA,
                                         thinking_config=types.ThinkingConfig(thinking_budget=0))
    for pedaco in cliente.models.generate_content_stream(model=modelo, contents=texto, config=config):
        if primeiro_pedaco is None:
            primeiro_pedaco = time.monotonic() - inicio
        acumulado += pedaco.text or ""
        m = FIM_DE_FRASE.search(acumulado)
        if m:
            return primeiro_pedaco, time.monotonic() - inicio, acumulado[: m.end()].strip()
    return primeiro_pedaco or 0.0, time.monotonic() - inicio, acumulado.strip()


async def tts_primeiro_audio(frase: str) -> float:
    inicio = time.monotonic()
    async for parte in edge_tts.Communicate(frase, VOZ).stream():
        if parte["type"] == "audio":
            return time.monotonic() - inicio
    return float("nan")


def main():
    cliente = genai.Client(api_key=chave())
    modelo = sys.argv[1] if len(sys.argv) > 1 else "gemini-2.5-flash"

    print("== STT (whisper.cpp turbo q8, Vulkan RX 6650 XT, modelo já carregado) ==")
    melhor = None
    for porta, nome in ((8178, "contexto completo"), (8179, "-ac 512")):
        stt(porta)  # aquecimento
        tempos, textos = [], set()
        for _ in range(RODADAS):
            t, txt = stt(porta)
            tempos.append(t)
            textos.add(txt)
        resumo(f"{nome} (porta {porta})", tempos)
        print(f"    texto(s): {json.dumps(sorted(textos), ensure_ascii=False)}")
        if melhor is None or statistics.median(tempos) < melhor[1]:
            melhor = (porta, statistics.median(tempos))

    print(f"\n== LLM ({modelo}, thinking desligado, streaming) ==")
    llm_primeira_frase(cliente, "oi", modelo)  # aquecimento de conexão
    p1, f1, frases = [], [], []
    for _ in range(RODADAS):
        a, b, frase = llm_primeira_frase(cliente, "Qual é a capital da Austrália e quantas horas de diferença ela tem para Brasília?", modelo)
        p1.append(a)
        f1.append(b)
        frases.append(frase)
    resumo("primeiro pedaço", p1)
    resumo("primeira frase completa", f1)
    print(f"    exemplo: {frases[0]!r}")

    print(f"\n== TTS (edge-tts {VOZ}, streaming) ==")
    asyncio.run(tts_primeiro_audio("Oi."))  # aquecimento
    tt = [asyncio.run(tts_primeiro_audio(frases[i % len(frases)])) for i in range(RODADAS)]
    resumo("primeiro pedaço de áudio", tt)

    print(f"\n== Encadeado real: STT (porta {melhor[0]}) -> LLM 1ª frase -> TTS 1º áudio ==")
    totais, sobrepostos = [], []
    for n in range(RODADAS):
        t_stt, texto = stt(melhor[0])
        _, t_llm, frase = llm_primeira_frase(cliente, texto, modelo)
        t_tts = asyncio.run(tts_primeiro_audio(frase))
        sem = W + t_stt + t_llm + t_tts
        com = max(W, t_stt) + t_llm + t_tts
        totais.append(sem)
        sobrepostos.append(com)
        print(f"  {n + 1}: stt={t_stt:.2f} llm={t_llm:.2f} tts={t_tts:.2f} | W+tudo={sem:.2f}s | STT durante W={com:.2f}s")
    resumo("fim da fala -> 1º som, STT só depois de W", totais)
    resumo("fim da fala -> 1º som, STT começando no 1º silêncio", sobrepostos)


main()
