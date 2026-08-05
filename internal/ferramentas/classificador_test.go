package ferramentas

import "testing"

func TestComandoEhLeitura(t *testing.T) {
	casos := []struct {
		nome    string
		comando string
		querido bool
	}{
		// Aceitos: verbo de leitura, sem token proibido.
		{"get simples", "Get-Process", true},
		{"get com parâmetro", "Get-Content C:\\temp\\log.txt", true},
		{"caixa alta", "GET-PROCESS", true},
		{"caixa baixa", "get-process", true},
		{"espaço na frente", "   Get-Date  ", true},
		{"alias dir", "dir", true},
		{"alias ls com caminho", "ls C:\\Users", true},
		{"whoami", "whoami", true},
		{"test-path", "Test-Path C:\\temp", true},
		{"pipeline todo leitura", "Get-Process | Select-Object Name | Sort-Object Name", true},
		{"where-object sem chaves", "Get-Process | Where-Object CPU -gt 10", true},

		// Rejeitados por verbo fora da allowlist.
		{"remove-item", "Remove-Item C:\\temp\\x.txt", false},
		{"stop-process", "Stop-Process -Name notepad", false},
		{"set-content", "Set-Content a.txt 'oi'", false},
		{"new-item", "New-Item -ItemType Directory teste", false},
		{"executável direto", "C:\\Windows\\System32\\calc.exe", false},
		{"cmdlet desconhecido", "Foo-Bar", false},
		{"prefixo get sem sufixo", "get-", false},

		// Rejeitados por token proibido, mesmo com verbo de leitura.
		{"ponto e vírgula", "Get-Process; Remove-Item x", false},
		{"e comercial", "Get-Process & calc.exe", false},
		{"crase", "Get-Process `n", false},
		{"subexpressão", "Get-Content $(whoami)", false},
		{"array subexpressão", "Get-Content @(1,2)", false},
		{"chaves scriptblock", "Get-Process | Where-Object { $_.CPU -gt 10 }", false},
		{"redirecionamento saída", "Get-Process > saida.txt", false},
		{"append", "Get-Process >> saida.txt", false},
		{"redirecionamento entrada", "Get-Content < entrada.txt", false},
		{"stop parsing", "Get-Process --% algo", false},
		{"acesso estático", "Get-Date; [System.IO.File]::Delete('x')", false},
		{"invoke-expression", "Invoke-Expression 'Remove-Item x'", false},

		// Rejeitados por estágio de escrita no pipeline.
		{"pipeline com escrita no fim", "Get-Process | Stop-Process", false},
		{"pipeline com escrita no meio", "Get-Content a | Set-Content b | Get-Item c", false},

		// Degenerados.
		{"vazio", "", false},
		{"só espaço", "   ", false},
		{"só pipe", "|", false},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := ComandoEhLeitura(c.comando); got != c.querido {
				t.Errorf("ComandoEhLeitura(%q) = %v, queria %v", c.comando, got, c.querido)
			}
		})
	}
}
