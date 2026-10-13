"""Spike 4: Claude Haiku 5.5 no pipeline (o número que faltava).

1. Haiku isolado, com streaming: primeiro pedaço e primeira frase completa,
   com prompt curto e com o prompts/sistema.md real do James.
2. Pipeline encadeado real: STT (whisper-server -ac 512, porta 8179)
   -> Haiku até a 1ª frase -> Piper 1º áudio.
Lê CLAUDE_API_KEY do .env sem imprimir.
"""

import re
import statistics
import time
from pathlib import Path

import anthropic
import requests
from piper import PiperVoice

AQUI = Path(__file__).parent
RAIZ = Path("/home/arthurorosco/SI/Projetos/Athena(Go)/meu-app-ts")
MODELO = "claude-haiku-5-5"
W = 0.7
RODADAS = 10
CURTO = "Responda sempre em português do Brasil, em uma ou duas frases curtas, sem Markdown."
FIM = re.compile(r"[.!?](\s|$)")
PERGUNTA = "Qual é a capital da Austrália e quantas horas de diferença ela tem para Brasília?"


def chave():
    for linha in (RAIZ / ".env").read_text().splitlines():
        if linha.startswith("CLAUDE_API_KEY="):
            return linha.split("=", 1)[1].strip().strip('"').strip("'")


def resumo(nome, v):
    print(f"  {nome}: mediana={statistics.median(v):.3f}s min={min(v):.3f}s max={max(v):.3f}s (n={len(v)})")


def haiku(cliente, sistema, texto, extra=None):
    inicio = time.monotonic()
    primeiro = None
    acumulado = ""
    frase = None
    t_frase = None
    with cliente.messages.stream(model=MODELO, max_tokens=1024, system=sistema,
                                 messages=[{"role": "user", "content": texto}], **(extra or {})) as fluxo:
        for pedaco in fluxo.text_stream:
            if primeiro is None:
                primeiro = time.monotonic() - inicio
            acumulado += pedaco
            m = FIM.search(acumulado)
            if m and frase is None:
                frase, t_frase = acumulado[: m.end()].strip(), time.monotonic() - inicio
        final = fluxo.get_final_message()
    return primeiro, t_frase or (time.monotonic() - inicio), frase or acumulado.strip(), acumulado.strip(), final.usage


def stt():
    inicio = time.monotonic()
    with open(AQUI / "pergunta.wav", "rb") as f:
        r = requests.post("http://127.0.0.1:8179/inference",
                          files={"file": ("pergunta.wav", f, "audio/wav")},
                          data={"response_format": "json", "temperature": "0.0"}, timeout=60)
    return time.monotonic() - inicio, r.json().get("text", "").strip()


def piper_primeiro(voz, frase):
    inicio = time.monotonic()
    for _ in voz.synthesize(frase):
        return time.monotonic() - inicio
    return float("nan")


VARIANTES = {
    "padrão (adaptativo, effort medium)": {},
    "adaptativo, effort low": {"output_config": {"effort": "low"}},
    "pensamento desligado": {"thinking": {"type": "disabled"}},
}


def main():
    cliente = anthropic.Anthropic(api_key=chave())
    sistema_real = (RAIZ / "prompts/sistema.md").read_text()
    haiku(cliente, CURTO, "oi")
    melhor = None
    for nome_v, extra in VARIANTES.items():
        for rotulo, sistema in (("prompt curto", CURTO), ("sistema.md real", sistema_real)):
            p, f, respostas, usos = [], [], [], []
            for _ in range(RODADAS):
                a, b, _, completa, uso = haiku(cliente, sistema, PERGUNTA, extra)
                p.append(a if a is not None else float("nan"))
                f.append(b)
                respostas.append(completa)
                usos.append(uso)
            print(f"== {nome_v} | {rotulo} ==")
            resumo("primeiro pedaço", [x for x in p if x == x])
            resumo("primeira frase", f)
            pens = [getattr(getattr(u, "output_tokens_details", None), "thinking_tokens", 0) or 0 for u in usos]
            acertos = sum(1 for r in respostas if "13" in r)
            print(f"  pensamento mediano={statistics.median(pens)} tok | respostas citando 13 h: {acertos}/{len(respostas)}")
            print(f"  exemplo: {respostas[0][:170]!r}")
            if rotulo == "prompt curto" and (melhor is None or statistics.median(f) < melhor[1]):
                melhor = (nome_v, statistics.median(f))

    nome_v = melhor[0]
    print(f"\n== Encadeado real com '{nome_v}': STT -> Haiku 1ª frase -> Piper 1º áudio ==")
    voz = PiperVoice.load(AQUI / "piper-voz" / "pt_BR-faber-medium.onnx")
    piper_primeiro(voz, "Aquecendo.")
    stt()
    totais = []
    for n in range(RODADAS):
        t_stt, texto = stt()
        _, t_llm, frase, _, _ = haiku(cliente, CURTO, texto, VARIANTES[nome_v])
        t_tts = piper_primeiro(voz, frase)
        totais.append(max(W, t_stt) + t_llm + t_tts)
        print(f"  {n + 1}: stt={t_stt:.2f} haiku={t_llm:.2f} piper={t_tts:.2f} | total={totais[-1]:.2f}s")
    resumo("fim da fala -> 1º som (STT durante W)", totais)


main()
