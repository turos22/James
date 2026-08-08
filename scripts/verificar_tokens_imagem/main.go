// Comando verificar_tokens_imagem confere a tabela de tiles de
// memoria.EstimarTokensImagem contra o que a API do Gemini realmente cobra.
// Existe porque a fórmula foi derivada da documentação, e documentação não é
// medição. Se divergir, a fórmula cede — não a API.
//
// CountTokens não serve pra isso: verificado ao vivo que devolve um valor
// fixo (~259) pra qualquer dimensão de imagem, de 300x200 a 8000x4000 — não
// aplica a tabela de tiles. O custo real só aparece no usageMetadata de uma
// chamada de geração de verdade (GenerateContent), em
// PromptTokensDetails, que isola o custo da modalidade "imagem" do resto
// do prompt.
package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"os"

	"github.com/joho/godotenv"
	"google.golang.org/genai"

	"james/internal/memoria"
)

func pngDe(largura, altura int) []byte {
	var buf bytes.Buffer
	_ = png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, largura, altura)))
	return buf.Bytes()
}

func main() {
	_ = godotenv.Load(".env")

	ctx := context.Background()
	cliente, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  os.Getenv("GEMINI_API_KEY"),
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		fmt.Println("erro ao criar cliente:", err)
		os.Exit(1)
	}

	casos := []struct{ largura, altura int }{{1920, 1080}, {3840, 2160}, {5120, 2880}}
	for _, c := range casos {
		imagem := pngDe(c.largura, c.altura)
		contents := []*genai.Content{{Role: "user", Parts: []*genai.Part{
			{InlineData: &genai.Blob{MIMEType: "image/png", Data: imagem}},
			{Text: "responda apenas OK"},
		}}}

		resp, err := cliente.Models.GenerateContent(ctx, "gemini-2.5-flash", contents, nil)
		if err != nil {
			fmt.Printf("%dx%d: erro %v\n", c.largura, c.altura, err)
			continue
		}
		nosso := memoria.EstimarTokensImagem(c.largura, c.altura)

		var imagemAPI int32 = -1
		for _, d := range resp.UsageMetadata.PromptTokensDetails {
			fmt.Printf("  modalidade=%s tokens=%d\n", d.Modality, d.TokenCount)
			if d.Modality == genai.MediaModalityImage {
				imagemAPI = d.TokenCount
			}
		}
		fmt.Printf("%dx%d: promptTotal=%d imagemAPI=%d nosso=%d diferenca=%d\n",
			c.largura, c.altura, resp.UsageMetadata.PromptTokenCount, imagemAPI, nosso, int(imagemAPI)-nosso)
	}
}
