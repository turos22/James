package ferramentas

import (
	"path/filepath"
	"strings"
)

// Arquivo sensível (spec 2026-10-14 arquivos, D8): o que costuma guardar
// segredo. Ler é livre, mas o valor chega ao modelo mascarado (mascara.go).

// nomesSensiveis casam com o nome do arquivo, sem diferenciar maiúscula.
var nomesSensiveis = map[string]bool{
	".netrc": true, ".git-credentials": true, ".npmrc": true, ".pypirc": true,
	"shadow": true, "gshadow": true, "sudoers": true,
}

// pastasSensiveis: qualquer arquivo dentro delas é sensível.
var pastasSensiveis = []string{"/.ssh/", "/.gnupg/", "/etc/sudoers.d/"}

// sufixosDeCaminhoSensivel: o fim do caminho completo (ex.: .kube/config).
var sufixosDeCaminhoSensivel = []string{
	"/.aws/credentials", "/.kube/config", "/.docker/config.json",
}

var extensoesSensiveis = map[string]bool{".pem": true, ".key": true, ".p12": true, ".pfx": true}

// prefixosDeChavePrivada: id_rsa, id_ed25519, id_ecdsa e a .pub de cada uma.
var prefixosDeChavePrivada = []string{"id_rsa", "id_ed25519", "id_ecdsa"}

// modelosDeEnv são arquivos .env que existem para ser lidos e copiados.
var modelosDeEnv = map[string]bool{".env.example": true, ".env.sample": true, ".env.template": true}

// nomesDeSegredo casam com o nome SEM a extensão (secrets.yaml, credentials).
var nomesDeSegredo = map[string]bool{
	"secrets": true, "secret": true, "credentials": true, "credential": true, "senhas": true,
}

// extensoesDeConfiguracao: a regra por nome só vale para arquivo de configuração
// ou sem extensão. credentials.go e secrets.py são código, e mascarar cada
// "x = y" deles o deixaria ilegível sem proteger nada.
var extensoesDeConfiguracao = map[string]bool{
	"": true, ".json": true, ".yaml": true, ".yml": true, ".toml": true, ".ini": true,
	".cfg": true, ".conf": true, ".properties": true, ".txt": true,
}

// prefixosDeJSONDeSegredo: arquivos de credencial que a nuvem entrega com nome
// gerado (client_secret_123.json, service-account-prod.json).
var prefixosDeJSONDeSegredo = []string{"client_secret", "service-account", "serviceaccount"}

func nomeParecidoComSegredo(nome string) bool {
	for _, modelo := range []string{".example", ".sample", ".template"} {
		if strings.Contains(nome, modelo) {
			return false
		}
	}
	extensao := filepath.Ext(nome)
	if extensao == ".env" { // producao.env, app.env
		return true
	}
	base := strings.TrimSuffix(nome, extensao)
	if nomesDeSegredo[base] && extensoesDeConfiguracao[extensao] {
		return true
	}
	if extensao == ".json" {
		for _, prefixo := range prefixosDeJSONDeSegredo {
			if strings.HasPrefix(base, prefixo) {
				return true
			}
		}
	}
	return false
}

// CaminhoSensivel diz se o arquivo pode guardar segredo. Olha o caminho
// pedido E o destino real, se for link simbólico: um atalho para
// ~/.ssh/id_rsa não pode escapar da máscara.
func CaminhoSensivel(caminho string) bool {
	if caminho == "" {
		return false
	}
	if nomeOuPastaSensivel(caminho) {
		return true
	}
	if real, err := filepath.EvalSymlinks(caminho); err == nil && real != caminho {
		return nomeOuPastaSensivel(real)
	}
	return false
}

func nomeOuPastaSensivel(caminho string) bool {
	limpo := strings.ToLower(filepath.ToSlash(filepath.Clean(caminho)))
	nome := limpo[strings.LastIndex(limpo, "/")+1:]

	if modelosDeEnv[nome] {
		return false
	}
	if nome == ".env" || strings.HasPrefix(nome, ".env.") {
		return true
	}
	if nomesSensiveis[nome] || extensoesSensiveis[filepath.Ext(nome)] || nomeParecidoComSegredo(nome) {
		return true
	}
	for _, prefixo := range prefixosDeChavePrivada {
		if strings.HasPrefix(nome, prefixo) {
			return true
		}
	}
	for _, pasta := range pastasSensiveis {
		if strings.Contains(limpo, pasta) {
			return true
		}
	}
	for _, sufixo := range sufixosDeCaminhoSensivel {
		if strings.HasSuffix(limpo, sufixo) {
			return true
		}
	}
	return false
}
