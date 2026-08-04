# rodar_claude.ps1
param([string]$Instrucao)

# O Claude Code autentica pela assinatura Pro via OAuth. Variáveis de ambiente
# da Anthropic herdadas do shell sequestrariam a sessão para outro provider —
# era exatamente o que este script fazia antes, apontando para o OpenRouter.
Remove-Item Env:\ANTHROPIC_BASE_URL, Env:\ANTHROPIC_API_KEY, Env:\ANTHROPIC_AUTH_TOKEN -ErrorAction SilentlyContinue

claude $Instrucao

Read-Host "Tarefa finalizada. Pressione Enter para fechar"
