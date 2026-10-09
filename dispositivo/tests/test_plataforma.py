import subprocess
import sys
from pathlib import Path

RAIZ_DISPOSITIVO = Path(__file__).resolve().parents[1]


def _importa_limpo(modulo: str) -> subprocess.CompletedProcess:
    """Importa num processo novo: sys.modules deste processo já pode ter o
    módulo (ou o Win32) carregado por outro teste."""
    return subprocess.run(
        [sys.executable, "-c", f"import {modulo}"],
        cwd=RAIZ_DISPOSITIVO, capture_output=True, text=True,
    )


def test_superficies_importa_em_qualquer_sistema():
    r = _importa_limpo("james_device.superficies")
    assert r.returncode == 0, r.stderr
