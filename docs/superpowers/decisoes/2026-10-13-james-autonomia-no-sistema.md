# Autonomia do James no sistema (anotado em 2026-10-13, para depois)

Origem: aceite ao vivo da voz em tempo real. O Arthur gostou da latência e da
continuação de 6 s, e disse: "a única coisa que me incomoda é que ele ainda não
tem tanta autonomia para executar comandos no sistema".

## 1. Pacote que o James sugeriu: `wmctrl`

- Serve para trazer a janela do Firefox para frente pelo terminal.
- Está nos repositórios oficiais do Fedora: `sudo dnf install wmctrl`.
- **Não foi instalado.** O Arthur recusou na hora; só anotou o nome.
- Ressalva técnica: `wmctrl` é X11. No KDE Wayland ele só enxerga janelas que
  rodam via XWayland. O Firefox nativo em Wayland não aparece nele. Por isso a
  escolha do pacote entra na spec B2 (tela e janelas no Wayland), onde as
  alternativas (`kdotool`, `ydotool`, API do KWin) serão comparadas. Não instalar
  nada antes disso.

## 2. Melhoria pedida: navegar pelos arquivos com mais autonomia

Regra do Arthur:

| Ação | Aprovação |
|---|---|
| Navegar, listar, ler, buscar arquivos | **não precisa** |
| Criar | **precisa** |
| Editar | **precisa** |
| Apagar | **precisa** |

O que já existe: a denylist do shell no Linux (`internal/ferramentas`, 73 casos)
confirma o que apaga, abre a máquina ou executa código baixado. O resto roda
livre.

A verificar na spec, não assumido:
- A denylist hoje cobre **criar e editar** (`touch`, `tee`, `>`, `>>`, `sed -i`,
  `cp`, `mv`, `mkdir`, editores)? Se não cobre, a regra do Arthur exige ampliar.
- Leitura de arquivos sensíveis (`~/.ssh`, `.env`, chaves) sem aprovação é
  aceitável, ou ler esses também pede confirmação?
- Talvez valha uma ferramenta própria de arquivos (listar, ler, buscar) em vez de
  depender do shell, para a leitura ser rápida e a escrita ficar sempre atrás de
  aprovação, sem depender de reconhecer comando por texto.

Depende de B2? Não. É independente: pode virar spec própria.
