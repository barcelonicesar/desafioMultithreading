# Desafio de Multithreading e APIs: consulta de CEP

Programa de terminal em Go que consulta BrasilAPI e ViaCEP simultaneamente, exibe o endereço da primeira resposta válida e identifica a API vencedora. O prazo total é de **1 segundo**, incluindo a leitura do JSON.

Usa apenas a biblioteca padrão e segue os conceitos dos exemplos deste repositório: goroutines, channels, select, net/http e context. Requer Go 1.24 ou superior e internet para as consultas reais.

## Executar

No PowerShell:

```powershell
cd C:\Users\za64542\Documents\GitHub\GoStudy\desafio-cep
go run . 01001000
```

O formato com hífen também é aceito:

```powershell
go run . 01001-000
```

Exemplo de saída quando a BrasilAPI vence (a vencedora pode mudar):

```text
API: BrasilAPI
{
  "cep": "01001000",
  "state": "SP",
  "city": "São Paulo",
  "neighborhood": "Sé",
  "street": "Praça da Sé",
  "service": "open-cep"
}
```

Todos os campos recebidos são preservados; os campos variam entre as APIs. Se o prazo vencer, a mensagem começa com:

```text
Erro: timeout: nenhuma API retornou um endereço em 1 segundo
```

CEP inválido, falhas HTTP, JSON inválido e CEP inexistente também são tratados. Se uma API falhar, a outra ainda pode vencer dentro do prazo. Se as duas falharem antes do prazo, são exibidos os erros das duas. Erros encerram o programa com código diferente de zero.

## Funcionamento

1. `validarCEP` verifica os oito dígitos e remove o hífen no formato `00000-000`.
2. `buscarEndereco` cria um contexto com prazo de um segundo e inicia uma goroutine por API.
3. Cada goroutine faz sua requisição com `net/http` e envia o resultado para um channel com buffer de duas posições.
4. O `select` aguarda um resultado ou o fim do prazo. A primeira resposta válida vence; falhas não vencem a corrida.
5. `defer cancel()` cancela a requisição mais lenta. O buffer permite que ela envie seu resultado sem bloquear, mesmo depois do retorno da busca.

O tempo de compilação de `go run` não entra no limite de um segundo. Caso as respostas cheguem praticamente juntas, não há uma ordem garantida.

Endpoints do enunciado:

- BrasilAPI: `https://brasilapi.com.br/api/cep/v1/{cep}`.
- ViaCEP: `http://viacep.com.br/ws/{cep}/json/`.

## Testar

Dentro da pasta do projeto:

```powershell
go test -v ./...
go vet ./...
go test -race ./...
```

Os testes usam servidores HTTP locais, sem depender da internet. Verificam as duas possíveis vencedoras, requisições simultâneas, cancelamento da perdedora, timeout durante a leitura do corpo, validação do CEP e respostas inválidas.

No Windows, `go test -race` exige CGO habilitado e um compilador C compatível. Os testes comuns podem ser executados com `go test -v ./...`.

## Estrutura

```text
desafio-cep/
├── .gitignore
├── go.mod
├── main.go
├── main_test.go
└── README.md
```

## Entrega no GitHub

O GoStudy reúne vários exercícios. Para cumprir a regra de um projeto por repositório, publique somente os cinco arquivos desta pasta na raiz de um repositório dedicado, com o código na branch `main`. Entregue o link desse repositório. Não publique a pasta `.gocache`, que contém apenas cache local de compilação.
