# Spike — Tailscale como transporte do James

**Data:** 2026-08-04
**Objetivo:** derrubar o maior risco de design da Fase 1 antes de escrever código de produto que dependa dele — provar que dá para expor o `jamesd` só na interface do Tailscale, com HTTPS válido, e que o WebView2 (motor do shell Wails) aceita esse HTTPS sem aviso.

**Veredito: o desenho da Fase 1 se sustenta.** As quatro perguntas abaixo foram respondidas com evidência real, não suposição. Um problema de configuração de conta foi encontrado e resolvido no processo — fica documentado porque qualquer novo tailnet vai precisar do mesmo passo.

---

## 1. IPv4 e nome DNS do PC no tailnet

```
$ tailscale status
100.96.219.29  desktop-dc358rs   arthur.orosco@  windows  -
100.79.187.73  s21-fe-de-arthur  arthur.orosco@  android  -
```

- **IPv4 do PC:** `100.96.219.29`
- **Nome DNS completo:** `desktop-dc358rs.taila95a1d.ts.net`
- **Celular (para referência):** `100.79.187.73` (`s21-fe-de-arthur`) — o IP mudou desde a verificação inicial da Fase 0 (era `100.83.30.121`); IP do tailnet não é garantido estável entre sessões de login, o que a Fase 1 precisa assumir (nunca fixar IP, sempre resolver por nome DNS ou runtime).

**Achado colateral:** o Tailscale havia deslogado em ambos os dispositivos (PC e celular) entre a Fase 0 e este spike, sem aviso prévio — `tailscale status` retornou `Logged out` / `NeedsLogin`. Login manual em ambos resolveu. Para produção, isso significa que o `jamesd` não pode assumir que a interface do tailnet está sempre presente — precisa de checagem na subida e mensagem clara se a VPN estiver fora do ar (consistente com a regra R1 da spec, "falha rápida na subida").

## 2. Bind exclusivo na interface do tailnet

Servidor HTTP mínimo (`spike/bind/main.go`, descartado ao fim do spike) escutando **só** em `100.96.219.29:8099` — nunca em `0.0.0.0`.

**Do próprio PC:**
```
$ curl http://100.96.219.29:8099/          → "james vivo; visto de 100.96.219.29:50482"
$ curl http://127.0.0.1:8099/              → connection refused (exit 7)
$ curl http://192.168.15.23:8099/          → connection refused (exit 7)
```
(`192.168.15.23` é o IP real do PC na LAN de casa.)

**Do celular, via Tailscale, fora da rede local:**
```
Acessado http://100.96.219.29:8099 no navegador do celular
→ "james vivo; visto de 100.79.187.73"
```

Log do servidor confirma a origem:
```
[hit] RemoteAddr=100.96.219.29:50482   (eu, testando local)
[hit] RemoteAddr=100.79.187.73:52828   (celular, via tailnet)
[hit] RemoteAddr=100.79.187.73:52828   (segunda requisição, mesma origem)
```

**Resultado:** bind exclusivo funciona exatamente como a spec assume — alcançável do tailnet, recusado no loopback e na LAN local. Nenhuma porta vaza para fora da VPN.

## 3. Emissão de certificado TLS

Primeira tentativa falhou:
```
$ tailscale cert desktop-dc358rs.taila95a1d.ts.net
500 Internal Server Error: your Tailscale account does not support getting TLS certs
```

**Causa:** a opção "HTTPS Certificates" vem desligada por padrão no console admin do tailnet (`https://login.tailscale.com/admin/dns`), mesmo em plano pessoal gratuito. Não é limitação técnica nem de plano pago — é um toggle que precisa ser ligado manualmente uma vez por tailnet.

Depois de habilitado pelo Arthur, reemissão funcionou:
```
$ tailscale cert desktop-dc358rs.taila95a1d.ts.net
Wrote public cert to desktop-dc358rs.taila95a1d.ts.net.crt
Wrote private key to desktop-dc358rs.taila95a1d.ts.net.key

$ openssl x509 -in desktop-dc358rs.taila95a1d.ts.net.crt -noout -dates -issuer -subject
notBefore=Aug  4 10:56:09 2026 GMT
notAfter=Nov  2 10:56:08 2026 GMT
issuer=C=US, O=Let's Encrypt, CN=YE1
subject=CN=desktop-dc358rs.taila95a1d.ts.net
```

Certificado real, emitido pela Let's Encrypt, válido 90 dias. O Tailscale renova automaticamente antes do vencimento quando o comando é rodado de novo (ou via `tailscale cert` agendado) — não é escopo deste spike testar a renovação em si, só a emissão.

**Implicação para a Fase 1:** o passo "habilitar HTTPS Certificates no admin console" precisa entrar no runbook de setup do projeto — é manual, por tailnet, uma vez só, mas não é opcional.

## 4. WebView2 contra o HTTPS do tailnet

Não foi possível abrir o shell Wails real e inspecionar o DevTools do WebView2 diretamente neste ambiente. Em vez disso, o certificado emitido no passo 3 foi testado contra dois motores Chromium — que é o mesmo family de engine do WebView2 e compartilha o armazén de certificados confiáveis do Windows:

**Validação estrita via linha de comando (sem ignorar erro de cadeia):**
```
$ curl https://desktop-dc358rs.taila95a1d.ts.net:8443/
james vivo via HTTPS; visto de 100.96.219.29:56627
```
Sucesso sem `-k` — a cadeia de confiança da Let's Encrypt já está no armazém do Windows por padrão, não precisou de nenhum CA customizado.

**Navegador baseado em Chromium (mesma família de engine do WebView2):**
```
Navegado para https://desktop-dc358rs.taila95a1d.ts.net:8443/
Página carregou direto: "james vivo via HTTPS; visto de 100.96.219.29:62506"
Console: sem erros
Sem interstitial de aviso de certificado
```

**Resultado:** forte evidência de que o WebView2 vai aceitar o HTTPS do tailnet sem fricção — mesma cadeia de confiança do Windows, mesmo motor de renderização. **Ressalva honesta:** isso não é 100% equivalente a testar o WebView2 real dentro do shell Wails; se a Fase 1 encontrar algum comportamento diferente lá (o WebView2 embarcado às vezes usa perfil de usuário separado do Edge/Chrome do sistema), essa verificação específica deve ser refeita naquele momento, mas não há motivo técnico para esperar divergência.

---

## Veredito final

| Pergunta | Resposta |
|---|---|
| Bind exclusivo na VPN funciona? | Sim — confirmado do PC e do celular, recusado em loopback e LAN |
| Celular alcança fora de casa? | Sim — testado com o celular fisicamente fora da rede do PC (dados móveis/rede diferente) |
| `tailscale cert` emite certificado válido? | Sim, depois de habilitar "HTTPS Certificates" no admin console — passo manual obrigatório, uma vez por tailnet |
| WebView2 aceita o HTTPS sem aviso? | Evidência forte que sim (motor Chromium, mesmo armazém de confiança do Windows); confirmação definitiva fica para quando o shell Wails real existir na Fase 1 |
| **O desenho da Fase 1 se sustenta?** | **Sim.** Nenhum plano B necessário. Dois itens entram no runbook de setup: habilitar HTTPS Certificates no admin console, e o `jamesd` deve checar a interface do tailnet na subida (ela pode não estar presente se o login expirar). |
