package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type API struct {
	Nome string
	URL  string
}

type Resultado struct {
	API   string
	Dados json.RawMessage
	Erro  error
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "Uso: go run . <cep> (exemplo: go run . 01001000)")
		os.Exit(1)
	}
	cep, err := validarCEP(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "Erro:", err)
		os.Exit(1)
	}
	apis := []API{
		{Nome: "BrasilAPI", URL: "https://brasilapi.com.br/api/cep/v1/" + cep},
		{Nome: "ViaCEP", URL: "http://viacep.com.br/ws/" + cep + "/json/"},
	}
	resultado, err := buscarEndereco(http.DefaultClient, apis)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Erro:", err)
		os.Exit(1)
	}
	fmt.Println("API:", resultado.API)
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(resultado.Dados); err != nil {
		fmt.Fprintln(os.Stderr, "Erro ao exibir endereço:", err)
		os.Exit(1)
	}
}

func validarCEP(entrada string) (string, error) {
	cep := entrada
	if len(cep) == 9 && cep[5] == '-' {
		cep = cep[:5] + cep[6:]
	}
	if len(cep) != 8 {
		return "", errors.New("o CEP deve ter 8 dígitos (exemplo: 01001000 ou 01001-000)")
	}
	for _, digito := range cep {
		if digito < '0' || digito > '9' {
			return "", errors.New("o CEP deve conter apenas números")
		}
	}
	return cep, nil
}

func buscarEndereco(client *http.Client, apis []API) (Resultado, error) {
	// O prazo é compartilhado pelas duas requisições e inclui a leitura do JSON.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel() // Cancela a requisição perdedora quando a busca termina.

	// O buffer evita bloquear um envio após o retorno desta função.
	resultados := make(chan Resultado, len(apis))
	for _, api := range apis {
		go consultarAPI(ctx, client, api, resultados)
	}
	var falhas []string
	for range apis {
		select {
		case <-ctx.Done():
			return Resultado{}, fmt.Errorf("timeout: nenhuma API retornou um endereço em 1 segundo: %w", ctx.Err())
		case resultado := <-resultados:
			if ctx.Err() != nil {
				return Resultado{}, fmt.Errorf("timeout: nenhuma API retornou um endereço em 1 segundo: %w", ctx.Err())
			}
			if resultado.Erro == nil {
				return resultado, nil
			}
			falhas = append(falhas, resultado.API+": "+resultado.Erro.Error())
		}
	}
	return Resultado{}, fmt.Errorf("não foi possível consultar o CEP: %s", strings.Join(falhas, "; "))
}

func consultarAPI(ctx context.Context, client *http.Client, api API, resultados chan<- Resultado) {
	dados, err := fazerRequisicao(ctx, client, api.URL)
	resultados <- Resultado{API: api.Nome, Dados: dados, Erro: err}
}

func fazerRequisicao(ctx context.Context, client *http.Client, url string) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("resposta HTTP %d", resp.StatusCode)
	}
	var dados json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&dados); err != nil {
		return nil, fmt.Errorf("JSON inválido: %w", err)
	}
	var endereco struct {
		CEP  string `json:"cep"`
		Erro bool   `json:"erro"`
	}
	if err := json.Unmarshal(dados, &endereco); err != nil {
		return nil, fmt.Errorf("endereço inválido: %w", err)
	}
	if endereco.Erro || endereco.CEP == "" {
		return nil, errors.New("CEP não encontrado ou resposta sem CEP")
	}
	return dados, nil
}
