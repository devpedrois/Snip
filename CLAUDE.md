# CLAUDE.md — Contexto do Projeto

> Este arquivo é a **fonte da verdade** para o Claude Code ao trabalhar neste repositório. Antes de qualquer alteração, releia este documento e respeite rigorosamente as convenções, a stack e o plano de PRs descritos aqui.

---

## 1. Visão Geral do Projeto

**Nome:** `snip`
**Resumo:** API RESTful em Go para encurtamento de URLs com cache em Redis, persistência em MySQL e analytics assíncrono via goroutines/channels. Projetado para alta concorrência, simulando um microsserviço pronto para produção.

**Objetivos técnicos do repositório:**
- Demonstrar uso idiomático de Go (goroutines, channels, context, interfaces).
- Padrão Cache-Aside com Redis na frente do MySQL.
- Logging assíncrono e analytics não-bloqueantes.
- Infraestrutura totalmente containerizada (Docker + Docker Compose).
- Cobertura de testes unitários relevante e código documentado.

---

## 2. Stack Tecnológica (fixa — não trocar sem aprovação)

| Camada | Tecnologia |
|---|---|
| Linguagem | **Go 1.22+** |
| Roteamento HTTP | `chi` (`github.com/go-chi/chi/v5`) |
| Banco de dados | **MySQL 8** |
| Driver MySQL | `github.com/go-sql-driver/mysql` |
| Migrations | `github.com/golang-migrate/migrate/v4` |
| Cache | **Redis 7** |
| Cliente Redis | `github.com/redis/go-redis/v9` |
| Logging | `log/slog` (stdlib) |
| Configuração | `github.com/joho/godotenv` + variáveis de ambiente |
| Testes | `testing` + `github.com/stretchr/testify` |
| Containerização | Docker + Docker Compose |

**Não introduzir:** ORMs (GORM, ent), frameworks pesados (Gin/Echo são aceitáveis somente se justificado), libs redundantes com a stdlib.

---

## 3. Estrutura de Diretórios

Siga o layout padrão da comunidade Go (`golang-standards/project-layout`):

```
snip/
├── cmd/
│   └── api/
│       └── main.go                 # Entrypoint da aplicação
├── internal/                       # Código privado da app
│   ├── config/                     # Carregamento de env vars
│   ├── domain/                     # Entidades (URL, Click) e erros de domínio
│   ├── handler/                    # Handlers HTTP (chi)
│   ├── service/                    # Regras de negócio
│   ├── repository/
│   │   ├── mysql/                  # Implementação MySQL
│   │   └── redis/                  # Implementação Redis (cache)
│   ├── hash/                       # Algoritmo Base62
│   ├── analytics/                  # Worker assíncrono de clicks
│   └── middleware/                 # Logging, recovery, request-id
├── migrations/                     # Arquivos .sql do golang-migrate
│   ├── 000001_create_urls_table.up.sql
│   ├── 000001_create_urls_table.down.sql
│   ├── 000002_create_clicks_table.up.sql
│   └── 000002_create_clicks_table.down.sql
├── tests/                          # Testes de integração e e2e
│   ├── integration/
│   └── e2e/
├── docker/
│   ├── Dockerfile
│   └── mysql/init.sql              # (opcional) seed inicial
├── docker-compose.yml
├── Makefile
├── .env.example
├── .gitignore
├── .dockerignore
├── go.mod
├── go.sum
├── CLAUDE.md                       # Este arquivo
└── README.md
```

**Regras de organização:**
- Nada em `internal/` é importado fora do projeto — use isso para forçar encapsulamento.
- Testes unitários ficam **junto ao código** (`*_test.go` no mesmo pacote).
- Testes que sobem MySQL/Redis reais ficam em `tests/integration/`.
- `pkg/` **não** é necessário — não exponha código publicamente sem motivo.

---

## 4. Regras de Negócio (autoritativas)

1. **Encurtamento (anônimo):** `POST /api/shorten` recebe `{"url": "https://..."}` e retorna `{"hash": "abc1234", "short_url": "http://host/abc1234"}`.
2. **Hash:** sempre **7 caracteres em Base62** (`[0-9a-zA-Z]`), gerado a partir do `id` auto-incremental da tabela `urls`.
3. **Validação da URL longa:**
   - Schema obrigatório (`http` ou `https`).
   - Host válido (parsing com `net/url`).
   - **Não** fazer HEAD request bloqueante na request de shorten — se a URL estiver fora do ar isso é problema do cliente, não da nossa API.
4. **Redirecionamento:** `GET /{hash}` retorna **HTTP 301** para a URL original. Se não existir, **HTTP 404**. Se expirada, **HTTP 410**.
5. **Cache (Cache-Aside):**
   - Em `GET /{hash}`: lê do Redis primeiro. **Miss** → consulta MySQL → popula Redis com TTL → responde.
   - Em `POST /api/shorten`: grava no MySQL → popula Redis.
   - TTL no Redis: **30 dias** (configurável via env).
6. **Analytics:**
   - Cada redirecionamento dispara um evento de click contendo `url_id`, `accessed_at`, `user_agent`, `ip`.
   - O evento é **enviado a um channel** e gravado por um pool de workers em goroutines.
   - **A resposta HTTP nunca espera o INSERT do click** (latência do redirect ≈ tempo do GET no cache).
7. **Expiração:** Links expiram após **30 dias de inatividade** (campo `last_accessed_at` é atualizado a cada acesso, em background — não bloqueia o redirect).
8. **Endpoint de Analytics:** `GET /api/analytics/{hash}` retorna contagem total, clicks por dia (últimos 30) e top User-Agents.

---

## 5. Modelo de Dados

### Tabela `urls`
```sql
CREATE TABLE urls (
  id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  hash            VARCHAR(10) NOT NULL UNIQUE,
  original_url    TEXT NOT NULL,
  created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_accessed_at TIMESTAMP NULL,
  expires_at      TIMESTAMP NULL,
  INDEX idx_hash (hash),
  INDEX idx_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### Tabela `clicks`
```sql
CREATE TABLE clicks (
  id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  url_id      BIGINT UNSIGNED NOT NULL,
  accessed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  user_agent  VARCHAR(512) NULL,
  ip          VARCHAR(45) NULL,
  FOREIGN KEY (url_id) REFERENCES urls(id) ON DELETE CASCADE,
  INDEX idx_url_id (url_id),
  INDEX idx_accessed_at (accessed_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### Chaves no Redis
- `snip:url:{hash}` → `original_url` (TTL configurável)
- `snip:meta:{hash}` → JSON com `{id, expires_at}` (opcional, evita 2 queries)

---

## 6. Convenções de Código

### Geral
- `gofmt` + `goimports` obrigatórios. Sem exceções.
- `go vet` deve passar limpo. `golangci-lint run` deve passar limpo.
- Nomes em **inglês** para código (variáveis, funções, pacotes, comentários de doc).
- Comentários explicativos no código podem ser em português, mas **GoDoc nas funções públicas em inglês**.

### Errors
- Erros de domínio em `internal/domain/errors.go` como sentinelas: `var ErrURLNotFound = errors.New("url not found")`.
- Use `errors.Is`/`errors.As` para verificação. Nunca compare strings de erro.
- Wrap com `fmt.Errorf("contexto: %w", err)`.

### Context
- **Toda** função que faz I/O recebe `ctx context.Context` como primeiro parâmetro.
- Handlers usam `r.Context()` e propagam até o repository.

### HTTP / JSON
- Requests e responses sempre com structs definidas (não usar `map[string]interface{}`).
- Erros retornam `{"error": "mensagem", "code": "ERR_CODE"}` com status HTTP coerente.

### Logging
- Use `slog` com handler JSON em produção, texto em dev.
- Sempre incluir `request_id` (middleware) e campos relevantes (`hash`, `url_id`).
- **Nunca** logar URLs originais com dados sensíveis em clear text além do necessário.

### Testes
- Tabela de testes (`tests := []struct{ name string; ... }{}`) é o padrão.
- Use `testify/assert` e `testify/require`.
- Mocks com interfaces — sem libs de mock automáticas exceto se realmente necessário.
- Cada PR deve incluir testes para o que adicionou.

### Conventional Commits (obrigatório)
Formato: `<tipo>(<escopo opcional>): <descrição em minúsculas>`

Tipos aceitos: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`.

Exemplos:
- `feat(shorten): add POST /api/shorten endpoint`
- `feat(cache): integrate redis with cache-aside pattern`
- `fix(hash): handle base62 collision edge case`
- `test(analytics): add worker pool unit tests`
- `docs(readme): add docker-compose run instructions`
- `refactor(repository): extract url repository interface`

**Branches:** `feat/<slug>`, `fix/<slug>`, `chore/<slug>`, `docs/<slug>`. Cada PR vive na sua branch.

---

## 7. Plano de Pull Requests (10 PRs)

> O Claude Code deve trabalhar **uma PR por vez**, sem antecipar trabalho de PRs futuras. Cada PR deve ser **mergeável de forma independente** (a aplicação compila e passa nos testes ao final de cada uma).

### PR #1 — `chore: project bootstrap and docker infrastructure`
**Objetivo:** Subir o esqueleto do projeto e a infraestrutura local.
**Entregáveis:**
- Estrutura de diretórios completa (vazia onde aplicável, com `.gitkeep`).
- `go.mod` com módulo `github.com/<usuario>/snip`.
- `cmd/api/main.go` com servidor HTTP mínimo respondendo `GET /health` → `{"status":"ok"}`.
- `docker/Dockerfile` (multi-stage, imagem final `scratch` ou `alpine`).
- `docker-compose.yml` com 3 serviços: `api`, `mysql`, `redis`. Healthchecks configurados.
- `.env.example` com todas as variáveis necessárias.
- `.gitignore`, `.dockerignore`.
- `Makefile` com targets: `up`, `down`, `build`, `run`, `test`, `lint`, `fmt`.
- `README.md` inicial (será expandido na PR #10).
- `CLAUDE.md` (este arquivo).

**Critério de aceitação:** `make up` sobe os 3 containers, `curl localhost:8080/health` retorna 200.

---

### PR #2 — `feat(db): config, domain entities and database migrations`
**Objetivo:** Fundação do domínio e infraestrutura de banco sem lógica de query.
**Entregáveis:**
- `internal/config/config.go`: struct `Config` + `LoadConfig()` lendo env vars com `joho/godotenv`. Sem panics — retorne erro.
- `internal/domain/url.go`: struct `URL`.
- `internal/domain/click.go`: struct `Click`.
- `internal/domain/errors.go`: sentinelas `ErrURLNotFound`, `ErrURLExpired`, `ErrHashConflict`.
- Migrations em `migrations/` (urls e clicks, up/down com DDL da seção 5).
- `internal/repository/mysql/connection.go`: `NewMySQLDB(cfg)` com pool configurado.
- `cmd/api/main.go`: executa migrations no startup e mantém `/health`.

**Critério de aceitação:** `make up` sobe tudo, migrations aplicadas, tabelas existem no MySQL, `/health` 200.

---

### PR #3 — `feat(repository): url and click repository layer`
**Objetivo:** Camada de repositório MySQL isolada com interfaces e testes.
**Entregáveis:**
- `internal/repository/mysql/url_repository.go`: interface `URLRepository` + `MySQLURLRepository`.
- `internal/repository/mysql/click_repository.go`: interface `ClickRepository` + `MySQLClickRepository`.
- Testes unitários com `go-sqlmock` para ambos os repositórios.

**Critério de aceitação:** `make test` passa, interfaces definidas e implementadas.

---

### PR #4 — `feat(hash): base62 encoder and url validator`
**Objetivo:** Algoritmo de hash e validação de URLs — código puro, sem I/O.
**Entregáveis:**
- `internal/hash/base62.go`: `Encode(id uint64) string` e `Decode(s string) (uint64, error)` com offset para 7 chars.
- `internal/hash/validator.go`: `ValidateURL(raw string) error`.
- Testes com cobertura ≥ 90% no pacote `internal/hash`.

**Critério de aceitação:** `go test ./internal/hash/... -cover` ≥ 90%.

---

### PR #5 — `feat(shorten): POST /api/shorten endpoint`
**Objetivo:** Endpoint de criação de URLs encurtadas, sem cache.
**Entregáveis:**
- `internal/handler/dto.go`: DTOs de request/response.
- `internal/service/shortener.go`: `ShortenerService` com fluxo valida → cria → hash → retorna.
- `internal/handler/shorten.go`: handler HTTP (201/400/500).
- `internal/middleware/`: RequestID, Logger, Recoverer.
- Testes unitários do service e do handler.

**Critério de aceitação:** `POST /api/shorten` funciona end-to-end via docker compose.

---

### PR #6 — `feat(redirect): GET /{hash} with 301 redirect`
**Objetivo:** Endpoint de redirecionamento, ainda sem cache.
**Entregáveis:**
- `internal/service/redirector.go`: `RedirectorService` com lookup + verificação de expiração.
- `internal/handler/redirect.go`: 301/404/410.
- Fire-and-forget provisório para `UpdateLastAccessed` (com `// TODO(PR#8): move to analytics dispatcher`).
- Testes do service e handler.

**Critério de aceitação:** `curl -I localhost:8080/<hash>` retorna 301 com Location correto.

---

### PR #7 — `feat(cache): redis cache-aside integration`
**Objetivo:** Integrar Redis como cache de leitura.
**Entregáveis:**
- `internal/repository/redis/connection.go`: `NewRedisClient(cfg)`.
- `internal/repository/redis/cache.go`: interface `URLCache` + `RedisURLCache`.
- Refatorar redirector com Cache-Aside. Falha de cache nunca derruba o redirect.
- Refatorar shortener para popular cache após criar.
- Testes com `miniredis`.

**Critério de aceitação:** Apenas 1 query SQL em N requests. Redirect funciona com Redis fora.

---

### PR #8 — `feat(analytics): async click dispatcher with goroutines`
**Objetivo:** Pool de workers assíncrono para gravação de clicks.
**Entregáveis:**
- `internal/analytics/event.go`: struct `ClickEvent`.
- `internal/analytics/dispatcher.go`: `Dispatcher` com channel, workers, graceful shutdown, métricas no log.
- Refatorar redirect: remover fire-and-forget, usar `dispatcher.Submit`.
- Graceful shutdown em `main.go` (SIGINT/SIGTERM → drain).
- Testes do dispatcher.

**Critério de aceitação:** Carga com `wrk` mantém p99 baixo. Clicks persistidos. Shutdown gracioso.

---

### PR #9 — `feat(analytics): GET /api/analytics/{hash} endpoint`
**Objetivo:** Endpoint de consulta de analytics.
**Entregáveis:**
- `internal/service/analytics.go`: `AnalyticsService` com `GetStats(ctx, hash)` → total, clicks/dia, top user-agents.
- `internal/handler/analytics.go`: handler HTTP respondendo JSON.
- Testes do service e handler.

**Critério de aceitação:** `curl localhost:8080/api/analytics/<hash>` retorna JSON estruturado.

---

### PR #10 — `docs: production-ready readme, integration tests and polish`
**Objetivo:** Documentação, testes de integração e polish final.
**Entregáveis:**
- `README.md` profissional completo (arquitetura Mermaid, setup, API docs, variáveis, decisões).
- `tests/integration/` com `testcontainers-go`.
- Health check enriquecido (MySQL + Redis ping → 503 se falhar).
- Logging revisado (request_id, hash, latency_ms).
- Cobertura ≥ 70%.
- `make coverage`, `make test-integration`.
- (Opcional) GitHub Actions CI.

**Critério de aceitação:** Dev novo clona, `make up`, sistema 100% funcional seguindo apenas o README.

---

## 8. Variáveis de Ambiente

```env
# App
APP_PORT=8080
APP_ENV=development              # development | production
BASE_URL=http://localhost:8080

# MySQL
MYSQL_HOST=mysql
MYSQL_PORT=3306
MYSQL_USER=shortener
MYSQL_PASSWORD=shortener_pass
MYSQL_DATABASE=snip
MYSQL_MAX_OPEN_CONNS=25
MYSQL_MAX_IDLE_CONNS=10

# Redis
REDIS_HOST=redis
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0
REDIS_TTL_DAYS=30

# Analytics
ANALYTICS_WORKERS=4
ANALYTICS_BUFFER=1000

# URL
URL_EXPIRATION_DAYS=30
```

---

## 9. Diretrizes para o Claude Code

Ao trabalhar neste repositório, o Claude Code **deve**:

1. **Sempre relê este `CLAUDE.md`** antes de iniciar uma PR.
2. **Não pular etapas:** se está na PR #5, não implementar Redis (isso é PR #7).
3. **Não inventar dependências:** usar apenas as listadas na seção 2.
4. **Escrever testes** para todo código novo na PR atual.
5. **Rodar `gofmt`, `go vet`, `go test ./...`** antes de declarar uma PR pronta.
6. **Commits pequenos e atômicos** dentro da PR, seguindo Conventional Commits.
7. **Se algo na descrição da PR for ambíguo, perguntar antes de assumir.**
8. **Não modificar este arquivo** sem instrução explícita.
9. **Não criar arquivos fora da estrutura definida** na seção 3.
10. **Justificar com comentário** qualquer escolha que se afaste das convenções (raro, mas permitido).

### Anti-padrões proibidos
- `panic()` em código de produção (apenas em `init()` quando faz sentido).
- `interface{}` / `any` desnecessários.
- Goroutines sem mecanismo de shutdown.
- SQL inline em handlers (sempre via repository).
- Hardcoded de strings de configuração (sempre env).
- Commits do tipo `update`, `wip`, `fix stuff`.

---

## 10. Comandos Rápidos

```bash
# Subir ambiente
make up

# Derrubar ambiente
make down

# Rodar testes
make test

# Testes de integração
make test-integration

# Ver cobertura
make coverage

# Lint
make lint

# Formatar
make fmt

# Aplicar migrations
make migrate-up

# Reverter última migration
make migrate-down

# Logs da API
docker compose logs -f api
```

---

**Última atualização deste documento:** início do projeto.
**Mantenedor:** time de desenvolvimento.
