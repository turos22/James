"""Spike 1: latência do Gemini Live, do fim da fala ao primeiro som.

Transmite pergunta.raw (PCM 16 kHz mono) em tempo real, como um microfone, e
continua mandando silêncio depois (a detecção de fim de fala é do servidor).
T0 = instante em que a última amostra com voz "foi falada".
T1 = chegada do primeiro pedaço de áudio da resposta.
"""

import array
import asyncio
import json
import statistics
import sys
import time
import wave
from pathlib import Path

from google import genai
from google.genai import types

AQUI = Path(__file__).parent
ENV = Path("/home/arthurorosco/SI/Projetos/Athena(Go)/meu-app-ts/.env")
MODELO = sys.argv[1] if len(sys.argv) > 1 else "gemini-3.8-live"
RODADAS = int(sys.argv[2]) if len(sys.argv) > 2 else 5
BLOCO = 320 * 2  # 20 ms de PCM 16 kHz 16 bits
MIME = "audio/pcm;rate=16000"


def chave() -> str:
    for linha in ENV.read_text().splitlines():
        if linha.startswith("GEMINI_API_KEY="):
            return linha.split("=", 1)[1].strip().strip('"').strip("'")
    raise SystemExit("GEMINI_API_KEY ausente do .env")


def fim_da_fala_s(pcm: bytes, limiar: int = 600) -> float:
    a = array.array("h")
    a.frombytes(pcm)
    janela = 320
    ultimo = 0
    for i in range(0, len(a), janela):
        if max(abs(x) for x in a[i:i + janela]) > limiar:
            ultimo = i + janela
    return ultimo / 16000


async def rodada(cliente, pcm: bytes, fim_s: float, n: int) -> dict:
    config = {
        "response_modalities": ["AUDIO"],
        "system_instruction": "Responda sempre em português do Brasil, em uma ou duas frases curtas.",
        "input_audio_transcription": {},
        "output_audio_transcription": {},
    }
    async with cliente.aio.live.connect(model=MODELO, config=config) as sessao:
        estado = {"t0": None, "t1": None, "fim": False}
        audio_resposta = bytearray()
        ouviu, disse = [], []

        async def enviar():
            inicio = time.monotonic()
            estado["t0"] = inicio + fim_s
            for i in range(0, len(pcm), BLOCO):
                await sessao.send_realtime_input(audio=types.Blob(data=pcm[i:i + BLOCO], mime_type=MIME))
                alvo = inicio + (i + BLOCO) / 32000
                await asyncio.sleep(max(0.0, alvo - time.monotonic()))
            silencio = b"\0" * BLOCO
            while not estado["fim"]:
                await sessao.send_realtime_input(audio=types.Blob(data=silencio, mime_type=MIME))
                await asyncio.sleep(0.02)

        async def receber():
            async for msg in sessao.receive():
                sc = msg.server_content
                if not sc:
                    continue
                if sc.input_transcription and sc.input_transcription.text:
                    ouviu.append(sc.input_transcription.text)
                if sc.output_transcription and sc.output_transcription.text:
                    disse.append(sc.output_transcription.text)
                if sc.model_turn:
                    for parte in sc.model_turn.parts:
                        if parte.inline_data and parte.inline_data.data:
                            if estado["t1"] is None:
                                estado["t1"] = time.monotonic()
                            audio_resposta.extend(parte.inline_data.data)
                if sc.turn_complete:
                    estado["fim"] = True
                    return

        envio = asyncio.create_task(enviar())
        try:
            await asyncio.wait_for(receber(), timeout=40)
        finally:
            estado["fim"] = True
            await envio

    with wave.open(str(AQUI / f"spike1_resposta_{n}.wav"), "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(24000)
        w.writeframes(bytes(audio_resposta))
    return {
        "latencia_s": round(estado["t1"] - estado["t0"], 3) if estado["t1"] else None,
        "ouviu": "".join(ouviu).strip(),
        "disse": "".join(disse).strip(),
        "resposta_s": round(len(audio_resposta) / 48000, 1),
    }


async def main():
    pcm = (AQUI / "pergunta.raw").read_bytes()
    fim_s = fim_da_fala_s(pcm)
    print(f"modelo={MODELO} fala={len(pcm) / 32000:.2f}s fim_da_fala={fim_s:.2f}s rodadas={RODADAS}")
    cliente = genai.Client(api_key=chave())
    resultados = []
    for n in range(1, RODADAS + 1):
        try:
            r = await rodada(cliente, pcm, fim_s, n)
        except Exception as erro:  # registra e segue: spike, não produção
            r = {"erro": f"{type(erro).__name__}: {erro}"[:300]}
        resultados.append(r)
        print(n, json.dumps(r, ensure_ascii=False))
    lat = [r["latencia_s"] for r in resultados if r.get("latencia_s") is not None]
    if lat:
        print(f"LATENCIA fim-da-fala -> primeiro som: mediana={statistics.median(lat):.2f}s "
              f"min={min(lat):.2f}s max={max(lat):.2f}s (n={len(lat)})")


asyncio.run(main())
