# snip

API RESTful em Go para encurtamento de URLs com cache em Redis, persistência em MySQL e analytics assíncrono. Microsserviço pronto para produção demonstrando uso idiomático de Go com goroutines, channels e padrão Cache-Aside.

## Pré-requisitos

- [Docker](https://docs.docker.com/get-docker/) e [Docker Compose](https://docs.docker.com/compose/)
- [Go 1.22+](https://golang.org/dl/) (apenas para desenvolvimento local)

## Como rodar

```bash
cp .env.example .env
make up
curl localhost:8080/health
```
