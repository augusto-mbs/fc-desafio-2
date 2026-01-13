package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// Estrutura para parse de respostas da API - Brasil API
type BrasilAPIResponse struct {
	CEP          string `json:"cep"`
	State        string `json:"state"`
	City         string `json:"city"`
	Neighborhood string `json:"neighborhood"`
	Street       string `json:"street"`
	Service      string `json:"service"`
}

// Estrutura para parse de respostas da API - Via CEP
type ViaCEPResponse struct {
	CEP         string `json:"cep"`
	Logradouro  string `json:"logradouro"`
	Complemento string `json:"complemento"`
	Bairro      string `json:"bairro"`
	Localidade  string `json:"localidade"`
	UF          string `json:"uf"`
	IBGE        string `json:"ibge"`
	GIA         string `json:"gia"`
	DDD         string `json:"ddd"`
	Siafi       string `json:"siafi"`
}

// Estrutura para unificada para apresentar a API mais rápida
type CEPResult struct {
	API        string
	CEP        string
	Logradouro string
	Bairro     string
	Cidade     string
	Estado     string
	Origem     string // "brasilapi" ou "viacep"
}

func main() {
	cep := "01001000" // CEP da Praça da Sé, São Paulo
	// Cep que utilizei onde retornou APIs diferentes.
	//cep := "13335320" // ViaCEP 13333-140 | Brasil API 13335-320

	fmt.Printf("Buscando CEP: %s\n\n", cep)

	// Contexto com timeout de 1 segundo
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Canais de comunição entre as goroutines
	chResultCEP := make(chan *CEPResult, 2)
	chError := make(chan error, 2)

	// Concorrência entre as goroutines
	go fetchBrasilAPI(ctx, cep, chResultCEP, chError)
	go fetchViaCEP(ctx, cep, chResultCEP, chError)

	// Aguarda o primeiro resultado ou timeout
	select {
	case result := <-chResultCEP:
		// Primeira API que responda com sucesso
		displayResult(result)

	case <-chError:
		// Se houver falha de uma API, aguarda receber o resultado da outra
		select {
		case result := <-chResultCEP:
			displayResult(result)
		case <-ctx.Done():
			log.Fatal("Timeout: Nenhuma API respondeu a tempo")
		}
	case <-ctx.Done():
		// Timeout de 1 segundo atingido
		log.Fatal("Timeout: Nenhuma API respondeu a tempo")
	}
}

// Função para busca do cep utilizando a API Brasil API
func fetchBrasilAPI(ctx context.Context, cep string, chResultCEP chan<- *CEPResult, chError chan<- error) {
	// URL
	url := fmt.Sprintf("https://brasilapi.com.br/api/cep/v1/%s", cep)

	// Chamada com contexto
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		chError <- fmt.Errorf("Brasil API: erro na requisição: %v", err)
		return
	}

	// Executa a requisição
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		chError <- fmt.Errorf("Brasil API: erro HTTP: %v", err)
		return
	}
	defer resp.Body.Close()

	// Checa o status code da requisição
	if resp.StatusCode != http.StatusOK {
		chError <- fmt.Errorf("Brasil API: status %d", resp.StatusCode)
		return
	}

	// Realiza leitura e parse das respostas
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		chError <- fmt.Errorf("Brasil API: erro na leitura: %v", err)
		return
	}

	var apiResponse BrasilAPIResponse
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		chError <- fmt.Errorf("Brasil API: erro no parse: %v", err)
		return
	}

	// Resultado unificado
	result := &CEPResult{
		API:        "Brasil API",
		CEP:        apiResponse.CEP,
		Logradouro: apiResponse.Street,
		Bairro:     apiResponse.Neighborhood,
		Cidade:     apiResponse.City,
		Estado:     apiResponse.State,
		Origem:     "brasilapi",
	}

	// Envia o resultado através do canal
	select {
	case chResultCEP <- result:
		// Resultado enviado com sucesso
	case <-ctx.Done():
		// Contexto cancelado
		return
	}
}

// Função para busca do cep utilizando a API ViaCEP
func fetchViaCEP(ctx context.Context, cep string, chResultCEP chan<- *CEPResult, chError chan<- error) {
	// URL
	url := fmt.Sprintf("http://viacep.com.br/ws/%s/json/", cep)

	// Chamada com contexto
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		chError <- fmt.Errorf("ViaCEP: erro na requisição: %v", err)
		return
	}

	// Executa a requisição
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		chError <- fmt.Errorf("ViaCEP: erro HTTP: %v", err)
		return
	}
	defer resp.Body.Close()

	// Checa o status code da requisição
	if resp.StatusCode != http.StatusOK {
		chError <- fmt.Errorf("ViaCEP: status %d", resp.StatusCode)
		return
	}

	// Realiza leitura e parse das respostas
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		chError <- fmt.Errorf("ViaCEP: erro na leitura: %v", err)
		return
	}

	var apiResponse ViaCEPResponse
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		chError <- fmt.Errorf("ViaCEP: erro no parse: %v", err)
		return
	}

	// Verifica se o CEP foi localizado
	if apiResponse.CEP == "" {
		chError <- fmt.Errorf("ViaCEP: CEP não encontrado")
		return
	}

	// Resultado unificado
	result := &CEPResult{
		API:        "ViaCEP",
		CEP:        apiResponse.CEP,
		Logradouro: apiResponse.Logradouro,
		Bairro:     apiResponse.Bairro,
		Cidade:     apiResponse.Localidade,
		Estado:     apiResponse.UF,
		Origem:     "viacep",
	}

	// Envia o resultado através do canal
	select {
	case chResultCEP <- result:
		// Resultado enviado com sucesso
	case <-ctx.Done():
		// Contexto cancelado
		return
	}
}

// Exibe a saída do CEP encontrado da API que forneceu o resultado mais rápido
func displayResult(result *CEPResult) {
	fmt.Println("Dados do CEP localizado")
	fmt.Println("=============================")
	fmt.Printf("API vencedora: %s\n", result.API)
	fmt.Printf("CEP: %s\n", result.CEP)
	fmt.Printf("Logradoruo: %s\n", result.Logradouro)
	fmt.Printf("Bairro: %s\n", result.Bairro)
	fmt.Printf("Cidade: %s\n", result.Cidade)
	fmt.Printf("Estado: %s\n", result.Estado)
	fmt.Printf("Origem: %s\n", result.Origem)
	fmt.Println("=============================")
	fmt.Println("Utilização da API mais rápida com sucesso!")
}
