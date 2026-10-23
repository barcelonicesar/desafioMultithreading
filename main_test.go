package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestValidarCEP(t *testing.T) {
	for _, entrada := range []string{"01001000", "01001-000"} {
		cep, err := validarCEP(entrada)
		if err != nil || cep != "01001000" {
			t.Fatalf("validarCEP(%q) = %q, %v", entrada, cep, err)
		}
	}
	for _, entrada := range []string{"", "123", "010010000", "abcdefgh", "0100-1000", "01001 000"} {
		if _, err := validarCEP(entrada); err == nil {
			t.Errorf("validarCEP(%q) deveria retornar erro", entrada)
		}
	}
}

func TestPrimeiraRespostaECancelamento(t *testing.T) {
	for _, vencedora := range []string{"BrasilAPI", "ViaCEP"} {
		t.Run(vencedora, func(t *testing.T) {
			iniciouLenta := make(chan struct{})
			cancelouLenta := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/"+vencedora {
					// A rápida só responde quando a outra também estiver em andamento.
					select {
					case <-iniciouLenta:
						fmt.Fprint(w, `{"cep":"01001000","logradouro":"Praça da Sé"}`)
					case <-r.Context().Done():
					}
					return
				}
				close(iniciouLenta)
				<-r.Context().Done()
				close(cancelouLenta)
			}))
			defer server.Close()
			resultado, err := buscarEndereco(server.Client(), []API{
				{Nome: "BrasilAPI", URL: server.URL + "/BrasilAPI"},
				{Nome: "ViaCEP", URL: server.URL + "/ViaCEP"},
			})
			if err != nil || resultado.API != vencedora {
				t.Fatalf("resultado = %+v, erro = %v", resultado, err)
			}
			if !strings.Contains(string(resultado.Dados), "Praça da Sé") {
				t.Fatalf("endereço não foi preservado: %s", resultado.Dados)
			}
			select {
			case <-cancelouLenta:
			case <-time.After(time.Second):
				t.Fatal("a requisição mais lenta não foi cancelada")
			}
		})
	}
}

func TestTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	inicio := time.Now()
	_, err := buscarEndereco(server.Client(), []API{
		{Nome: "BrasilAPI", URL: server.URL},
		{Nome: "ViaCEP", URL: server.URL},
	})
	duracao := time.Since(inicio)
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("esperava timeout, recebeu %v", err)
	}
	if duracao < time.Second || duracao > 2*time.Second {
		t.Fatalf("timeout fora do prazo esperado: %v", duracao)
	}
}

func TestFalhaNaoImpedeOutraAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/falha" {
			http.Error(w, "indisponível", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprint(w, `{"cep":"01001-000","logradouro":"Praça da Sé"}`)
	}))
	defer server.Close()
	resultado, err := buscarEndereco(server.Client(), []API{
		{Nome: "BrasilAPI", URL: server.URL + "/falha"},
		{Nome: "ViaCEP", URL: server.URL + "/sucesso"},
	})
	if err != nil || resultado.API != "ViaCEP" {
		t.Fatalf("resultado = %+v, erro = %v", resultado, err)
	}
}

func TestRespostasInvalidas(t *testing.T) {
	casos := []struct {
		nome   string
		status int
		corpo  string
	}{
		{"HTTP 404", http.StatusNotFound, `{}`},
		{"CEP inexistente", http.StatusOK, `{"erro":true}`},
		{"JSON inválido", http.StatusOK, `isto não é JSON`},
		{"sem CEP", http.StatusOK, `{}`},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(caso.status)
				fmt.Fprint(w, caso.corpo)
			}))
			defer server.Close()
			_, err := buscarEndereco(server.Client(), []API{
				{Nome: "BrasilAPI", URL: server.URL},
				{Nome: "ViaCEP", URL: server.URL},
			})
			if err == nil || !strings.Contains(err.Error(), "BrasilAPI:") || !strings.Contains(err.Error(), "ViaCEP:") {
				t.Fatalf("esperava os erros das duas APIs, recebeu %v", err)
			}
		})
	}
}
