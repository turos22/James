"""Spike 1b: tokens cobrados pelo Gemini Live por turno, com o prompt real do James.

Uma sessão, dois turnos (pergunta + seguimento). Manda o áudio em tempo real e
silêncio só até a resposta começar (depois para de enviar, como faria o device).
Imprime o usage_metadata que o servidor devolve.
"""

import asyncio
import json
import sys
import time
from pathlib import Path

from google import genai
from google.genai import types

AQUI = Path(__file__).parent
RAIZ = Path("/home/arthurorosco/SI/Projetos/Athena(Go)/meu-app-ts")
MODELO = sys.argv[1] if len(sys.argv) > 1 else "gemini-3.8-live"
BLOCO = 640
MIME = "audio/pcm;rate=16000"


def chave():
    for linha in (RAIZ / ".env").read_text().splitlines():
        if linha.startswith("GEMINI_API_KEY="):
            return linha.split("=", 1)[1].strip().strip('"').strip("'")


async def turno(sessao, pcm: bytes, rotulo: str):
    estado = {"primeiro": False, "fim": False}
    usos = []
    duracao_resposta = 0

    async def enviar():
        inicio = time.monotonic()
        for i in range(0, len(pcm), BLOCO):
            await sessao.send_realtime_input(audio=types.Blob(data=pcm[i:i + BLOCO], mime_type=MIME))
            await asyncio.sleep(max(0.0, inicio + (i + BLOCO) / 32000 - time.monotonic()))
        while not estado["primeiro"] and not estado["fim"]:
            await sessao.send_realtime_input(audio=types.Blob(data=b"\0" * BLOCO, mime_type=MIME))
            await asyncio.sleep(0.02)

    async def receber():
        nonlocal duracao_resposta
        async for msg in sessao.receive():
            if msg.usage_metadata:
                usos.append(msg.usage_metadata.model_dump(exclude_none=True))
            sc = msg.server_content
            if sc and sc.model_turn:
                for p in sc.model_turn.parts:
                    if p.inline_data and p.inline_data.data:
                        estado["primeiro"] = True
                        duracao_resposta += len(p.inline_data.data)
            if sc and sc.turn_complete:
                estado["fim"] = True
                return

    envio = asyncio.create_task(enviar())
    await asyncio.wait_for(receber(), timeout=40)
    estado["fim"] = True
    await envio
    print(f"== {rotulo}: fala {len(pcm) / 32000:.2f}s, resposta {duracao_resposta / 48000:.1f}s ==")
    for u in usos:
        print(json.dumps(u, ensure_ascii=False))


async def main():
    sistema = (RAIZ / "prompts/sistema.md").read_text()
    config = {"response_modalities": ["AUDIO"], "system_instruction": sistema}
    cliente = genai.Client(api_key=chave())
    async with cliente.aio.live.connect(model=MODELO, config=config) as sessao:
        await turno(sessao, (AQUI / "pergunta.raw").read_bytes(), "turno 1")
        await turno(sessao, (AQUI / "seguimento.raw").read_bytes(), "turno 2")


asyncio.run(main())
