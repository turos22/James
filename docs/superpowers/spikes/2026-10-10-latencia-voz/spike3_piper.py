"""Spike 3: TTS local (Piper, pt_BR-faber-medium) no lugar do edge-tts.

1. Piper isolado: carga, tempo até o primeiro áudio, fator de tempo real (RTF).
2. Mesmas frases em Piper e em edge-tts (Andrew), salvas para comparar de ouvido.
3. Pipeline encadeado real com Piper: STT (whisper-server -ac 512, porta 8179)
   -> LLM até a 1ª frase (Gemini 2.5 Flash, substituto do Haiku) -> Piper 1º áudio.
"""

import asyncio
import re
import statistics
import time
import wave
from pathlib import Path

import edge_tts
import requests
from google import genai
from google.genai import types
from piper import PiperVoice

AQUI = Path(__file__).parent
ENV = Path("/home/arthurorosco/SI/Projetos/Athena(Go)/meu-app-ts/.env")
VOZ_PIPER = AQUI / "piper-voz" / "pt_BR-faber-medium.onnx"
VOZ_EDGE = "en-US-AndrewMultilingualNeural"
W = 0.7
RODADAS = 10
SISTEMA = "Responda sempre em português do Brasil, em uma ou duas frases curtas, sem Markdown."
FIM = re.compile(r"[.!?](\s|$)")
FRASES = [
    "A capital da Austrália é Canberra.",
    "Claro, Arthur, já abri o Firefox com a previsão do tempo de Florianópolis.",
    "O container do Postgres está de pé e respondendo na porta 5434.",
    "Pronto, anotei que você precisa comprar leite e pão.",
    "Agora são três e quinze da tarde.",
]


def chave():
    for linha in ENV.read_text().splitlines():
        if linha.startswith("GEMINI_API_KEY="):
            return linha.split("=", 1)[1].strip().strip('"').strip("'")


def resumo(nome, v):
    print(f"  {nome}: mediana={statistics.median(v):.3f}s min={min(v):.3f}s max={max(v):.3f}s (n={len(v)})")


def piper_primeiro_audio(voz, frase):
    inicio = time.monotonic()
    primeiro = None
    pcm = bytearray()
    taxa = 22050
    for pedaco in voz.synthesize(frase):
        if primeiro is None:
            primeiro = time.monotonic() - inicio
        taxa = pedaco.sample_rate
        pcm.extend(pedaco.audio_int16_bytes)
    total = time.monotonic() - inicio
    return primeiro, total, bytes(pcm), taxa


def salvar_wav(caminho, pcm, taxa):
    with wave.open(str(caminho), "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(taxa)
        w.writeframes(pcm)


async def edge_mp3(frase, destino):
    await edge_tts.Communicate(frase, VOZ_EDGE).save(str(destino))


def stt():
    inicio = time.monotonic()
    with open(AQUI / "pergunta.wav", "rb") as f:
        r = requests.post("http://127.0.0.1:8179/inference",
                          files={"file": ("pergunta.wav", f, "audio/wav")},
                          data={"response_format": "json", "temperature": "0.0"}, timeout=60)
    return time.monotonic() - inicio, r.json().get("text", "").strip()


def llm_primeira_frase(cliente, texto):
    inicio = time.monotonic()
    acumulado = ""
    config = types.GenerateContentConfig(system_instruction=SISTEMA,
                                         thinking_config=types.ThinkingConfig(thinking_budget=0))
    for pedaco in cliente.models.generate_content_stream(model="gemini-2.5-flash", contents=texto, config=config):
        acumulado += pedaco.text or ""
        m = FIM.search(acumulado)
        if m:
            return time.monotonic() - inicio, acumulado[: m.end()].strip()
    return time.monotonic() - inicio, acumulado.strip()


def main():
    inicio = time.monotonic()
    voz = PiperVoice.load(VOZ_PIPER)
    print(f"== Piper: carga da voz {time.monotonic() - inicio:.2f}s (uma vez, no boot) ==")
    piper_primeiro_audio(voz, "Aquecendo.")

    primeiros, rtfs = [], []
    for i, frase in enumerate(FRASES):
        for _ in range(2):
            p, total, pcm, taxa = piper_primeiro_audio(voz, frase)
            primeiros.append(p)
            rtfs.append(total / (len(pcm) / 2 / taxa))
        salvar_wav(AQUI / f"comparar_{i + 1}_piper.wav", pcm, taxa)
        asyncio.run(edge_mp3(frase, AQUI / f"comparar_{i + 1}_edge.mp3"))
    resumo("primeiro áudio (frase inteira, 5 frases x 2)", primeiros)
    print(f"  RTF mediano: {statistics.median(rtfs):.3f} (tempo de síntese / duração do áudio)")

    print("\n== Encadeado real: STT -> LLM 1ª frase -> Piper 1º áudio ==")
    cliente = genai.Client(api_key=chave())
    stt()
    llm_primeira_frase(cliente, "oi")
    totais, sobrepostos = [], []
    for n in range(RODADAS):
        t_stt, texto = stt()
        t_llm, frase = llm_primeira_frase(cliente, texto)
        t_tts, _, _, _ = piper_primeiro_audio(voz, frase)
        totais.append(W + t_stt + t_llm + t_tts)
        sobrepostos.append(max(W, t_stt) + t_llm + t_tts)
        print(f"  {n + 1}: stt={t_stt:.2f} llm={t_llm:.2f} piper={t_tts:.2f} | STT durante W={sobrepostos[-1]:.2f}s | frase={frase!r}")
    resumo("fim da fala -> 1º som, STT só depois de W", totais)
    resumo("fim da fala -> 1º som, STT começando no 1º silêncio", sobrepostos)


main()
