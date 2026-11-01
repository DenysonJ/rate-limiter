# SPEC-limiter-001: Pacote de rate-limit importável

- **Status:** implemented
- **Tipo:** feature
- **Módulo:** limiter
- **Arquitetura:** clean
- **Stack:** go
- **Atualizado em:** 2026-10-30

## 1. Objetivo de Negócio

Transformar este projeto em uma biblioteca Go importável por qualquer outro projeto, 
de modo que adicionar rate-limiting (por IP ou por token) seja apenas importar um
pacote e plugar um middleware — sem reimplementar a lógica a cada projeto. Hoje o código
reutilizável vive em `internal/` (inacessível a módulos externos) e está acoplado a Gin, a
um logger concreto e à leitura de variáveis de ambiente, o que impede o reúso.

## 2. Contexto e Escopo

O consumidor é um desenvolvedor Go que importa o pacote em sua aplicação e o conecta ao seu
servidor HTTP. O ponto de entrada em runtime é o **middleware HTTP**: a cada requisição ele
deriva a chave (token ou IP), consulta o core de rate-limit e deixa passar ou responde
`429`. O core expõe `Allow(ctx, key) (bool, error)` e uma **porta de estratégia** de
armazenamento (contagem + bloqueio), com implementações in-memory (1ª classe) e Redis
(subpacote isolado). A configuração é programática; o carregamento via `.env` vira helper
opcional.

### Fora de escopo

- Novos algoritmos de limitação (sliding window log, token bucket, leaky bucket): mantém-se
  a contagem por janela fixa com bloqueio, já existente.
- Estratégias de armazenamento além de in-memory e Redis (Memcached, Postgres, etc.).
- Limitação distribuída com sincronização entre nós além do que o backend compartilhado
  (Redis) já oferece.
- Métricas/observabilidade (Prometheus, tracing) embutidas no core.
- Versionamento semântico, publicação de tags e pipeline de release do módulo.
- Adapters para frameworks além de `net/http` e Gin (echo, fiber, chi têm compatibilidade
  via o middleware `net/http`, mas sem adapter dedicado nesta spec).

## 3. Regras de Negócio (EARS)

- **RN-01:** O pacote DEVE ser importável por qualquer módulo Go externo — todo o código
  reutilizável (core, porta de estratégia, estratégias, middlewares e helper de config) DEVE
  residir fora de `internal/`, sob o caminho público do módulo.
- **RN-02:** O core DEVE ser agnóstico de framework e de infraestrutura — não DEVE importar
  nenhum framework HTTP (Gin etc.), cliente de banco/cache concreto nem logger concreto.
- **RN-03:** WHERE uma função de extração de chave for fornecida pelo consumidor, o sistema
  DEVE usá-la para derivar a chave; caso contrário o sistema DEVE ler o header configurado
  (padrão `API_KEY`) e, SE esse header estiver ausente ou vazio, ENTÃO DEVE usar o IP do
  cliente como chave.
- **RN-04:** SE existir configuração específica para a chave (token) consultada, ENTÃO o
  sistema DEVE aplicar os limites dessa configuração; caso contrário DEVE aplicar a
  configuração padrão (IP).
- **RN-05:** WHEN `Allow` é chamado para uma chave não bloqueada, o sistema DEVE incrementar
  o contador da chave dentro da janela configurada e DEVE fazer o contador expirar ao fim da
  janela.
- **RN-06:** IF o contador da chave exceder o limite dentro da janela, THEN o sistema DEVE
  bloquear a chave pela duração de bloqueio configurada e DEVE negar a requisição.
- **RN-07:** WHILE uma chave estiver bloqueada, o sistema DEVE negar toda requisição dessa
  chave sem incrementar o contador.
- **RN-08:** WHEN a estratégia de armazenamento retornar erro, o sistema DEVE aplicar a
  política de falha configurada — por padrão **fail-open** (permitir a requisição) — e DEVE
  propagar o erro ao chamador.
- **RN-09:** O método `Allow` do core DEVE retornar `(permitido bool, erro error)` e o core
  NÃO DEVE logar nem escrever em stdout/stderr.
- **RN-10:** WHEN o core nega a requisição, o middleware DEVE responder
  `429 Too Many Requests` com o corpo configurável (padrão: mensagem JSON de limite
  excedido) e DEVE interromper a cadeia, não invocando o próximo handler.
- **RN-11:** O core DEVE definir a porta `Strategy`; o pacote DEVE fornecer uma estratégia
  in-memory de primeira classe e a estratégia Redis DEVE residir em subpacote isolado, de
  forma que importar apenas o core ou o in-memory NÃO force a dependência `go-redis`.
- **RN-12:** O sistema DEVE permitir configuração programática (construtor + opções) sem
  depender de variáveis de ambiente; o carregamento via `.env`/env DEVE ser um helper
  opcional, separado do core.
- **RN-13:** O sistema DEVE propagar o `context.Context` da requisição por todas as
  operações de I/O da estratégia (incrementar, bloquear, consultar bloqueio), respeitando
  cancelamento e deadlines.

### Tabela de decisão — `Allow(ctx, key)`

| Estado da chave | Resultado do backend          | Decisão                                                  | Regra        |
|-----------------|-------------------------------|----------------------------------------------------------|--------------|
| Bloqueada       | —                             | Nega; não incrementa                                     | RN-07        |
| Não bloqueada   | erro em qualquer operação     | Aplica política de falha (padrão permite) + propaga erro | RN-08        |
| Não bloqueada   | contador ≤ limite             | Permite                                                  | RN-05, RN-04 |
| Não bloqueada   | contador > limite             | Bloqueia a chave e nega                                  | RN-06        |

## 4. Casos de Uso

### UC-01: Primeira requisição de uma chave nova — exercita RN-03, RN-04, RN-05

- **Dado** uma chave sem contador nem bloqueio e configuração padrão (IP) `Limit=10`
- **Quando** o consumidor chama `Allow(ctx, key)` (ou chega a 1ª requisição no middleware)
- **Então** o contador vira 1, a requisição é permitida e `Allow` retorna `(true, nil)`

### UC-02: Token tem prioridade sobre o limite de IP — exercita RN-03, RN-04

- **Dado** um token `abc123` com `Limit=100` registrado e limite de IP `Limit=10`
- **Quando** chega uma requisição com header `API_KEY: abc123`
- **Então** o sistema aplica o limite do token (100), não o do IP

### UC-03: Chave excede o limite e é bloqueada — exercita RN-05, RN-06, RN-10

- **Dado** uma chave com `Limit=2`, `Window=5s`, `BlockDuration=5s` e 2 acessos na janela
- **Quando** chega a 3ª requisição dentro da janela
- **Então** o sistema bloqueia a chave por 5s, nega a requisição, `Allow` retorna
  `(false, nil)` e o middleware responde `429` com o corpo configurado

### UC-04: Requisição de chave já bloqueada — exercita RN-07

- **Dado** uma chave atualmente bloqueada
- **Quando** chega uma nova requisição dessa chave
- **Então** o sistema nega sem incrementar o contador e o middleware responde `429`

### UC-05: Backend indisponível com política padrão (fail-open) — exercita RN-08, RN-09

- **Dado** a estratégia Redis retornando erro (ex.: Redis fora do ar) e política padrão
- **Quando** o consumidor chama `Allow(ctx, key)`
- **Então** `Allow` retorna `(true, erroDoBackend)` — permite, mas devolve o erro para o
  chamador decidir logar/tratar; o core não loga

### UC-06: Backend indisponível com política fail-closed — exercita RN-08

- **Dado** a estratégia retornando erro e política configurada como fail-closed
- **Quando** o consumidor chama `Allow(ctx, key)`
- **Então** `Allow` retorna `(false, erroDoBackend)` e o middleware responde `429`

### UC-07: Importar só o in-memory sem puxar go-redis — exercita RN-01, RN-02, RN-11

- **Dado** um projeto externo que importa apenas o core + estratégia in-memory
- **Quando** ele compila
- **Então** a dependência `go-redis` não entra no grafo de compilação daquele binário

### UC-08: Configuração 100% programática — exercita RN-12

- **Dado** um projeto que configura limites via construtor/opções, sem `.env`
- **Quando** inicializa o rate-limiter sem definir variáveis de ambiente
- **Então** o rate-limiter funciona normalmente, sem erro por env ausente

## 5. API / Contrato (superfície pública do pacote)

**Core (`ratelimiter`):**

```
type Config struct {
    Limit         int
    Window        time.Duration
    BlockDuration time.Duration
}

type Strategy interface {                 // porta definida no core (RN-11)
    Increment(ctx context.Context, key string, window time.Duration) (int, error)
    Block(ctx context.Context, key string, ttl time.Duration) error
    IsBlocked(ctx context.Context, key string) (bool, error)
}

func New(strategy Strategy, defaultConfig Config, opts ...Option) *RateLimiter
func (rl *RateLimiter) RegisterTokenConfig(key string, cfg Config)
func (rl *RateLimiter) Allow(ctx context.Context, key string) (bool, error)   // RN-09

// Opções (funcionais): política de falha (fail-open padrão), configs de token, etc.
func WithFailClosed() Option
func WithTokenConfigs(map[string]Config) Option
```

**Estratégias:**
- `ratelimiter/store/memory` — estratégia in-memory de 1ª classe (só stdlib).
- `ratelimiter/store/redisstore` — estratégia Redis (subpacote; única que importa `go-redis`). Nomeado `redisstore` para não colidir com o pacote `redis` do `go-redis` no importador.

**Middlewares:**
- `ratelimiter/middleware/nethttp` — `func Handler(rl *ratelimiter.RateLimiter, opts ...MWOption) func(http.Handler) http.Handler`
- `ratelimiter/middleware/ginmw` — adapter equivalente para `gin.HandlerFunc`. Nomeado `ginmw` para não colidir com o pacote `gin` do `gin-gonic` no importador.
- Opções do middleware: header da chave (padrão `API_KEY`), função de extração de chave,
  corpo/status da resposta de bloqueio.

**Helper opcional de config:** `ratelimiter/config` (ou similar) — carrega `Config` padrão e
`map[string]Config` a partir de env/`.env`. Não é importado pelo core.

**Contrato HTTP do middleware (resposta de bloqueio — padrão):**

| Item   | Valor                                                                                                           |
|--------|-----------------------------------------------------------------------------------------------------------------|
| Status | `429 Too Many Requests`                                                                                         |
| Body   | `{"message": "you have reached the maximum number of requests or actions allowed within a certain time frame"}` |

## 6. Condições de Erro

| Código                  | HTTP        | Quando ocorre                                                                                                             | Regra               |
|-------------------------|-------------|---------------------------------------------------------------------------------------------------------------------------|---------------------|
| ERR_RATE_LIMITED        | 429         | Chave excede o limite na janela ou está bloqueada                                                                         | RN-06, RN-07, RN-10 |
| ERR_BACKEND_UNAVAILABLE | 429 / 200\* | Estratégia retorna erro; `Allow` propaga o erro ao chamador. \* `200` (passa) sob fail-open padrão; `429` sob fail-closed | RN-08, RN-09        |

> Os erros do core são valores Go (erros sentinela exportados, envolvidos com `%w`). A
> tradução para status HTTP ocorre no middleware (adapter), nunca no core.

## 7. Notas de Arquitetura

Derivadas de `clean-architecture.md` e `golang-best-practices.md`. Orientação estrutural, não código.

**Camadas e fronteiras**
- **Core (entidade + caso de uso):** pacote `ratelimiter` com `RateLimiter`, `Config`,
  a porta `Strategy`, a política de falha e a lógica de `Allow`. É a camada mais interna —
  **zero** dependências de framework, cache ou logger (RN-02). A regra "token tem prioridade
  sobre IP" e a decisão de bloqueio são regras de aplicação e vivem aqui.
- **Adapters (interface adapters):** os middlewares `nethttp` e `gin` traduzem a requisição
  externa em uma chamada `Allow` e traduzem a negação em resposta `429`. São finos: extraem
  a chave, chamam o core, formatam a saída. Dependem do core (dependência aponta para
  dentro), nunca o contrário.
- **Infra (frameworks & drivers):** a estratégia Redis em `store/redis` implementa a porta
  `Strategy` e é o único ponto que conhece `go-redis`. O in-memory em `store/memory` é um
  adapter sem I/O externo. Trocar Redis por outro backend não toca o core (teste de fumaça
  da arquitetura).

**Regra da dependência & ports/adapters**
- `Strategy` é a **porta** definida no core (aceita interface, retorna struct; interface
  pequena de 3 métodos, no pacote que a consome — idioma Go). As implementações a satisfazem
  de fora. Isso inverte a dependência: o core não conhece Redis.
- **Ponto de composição:** cada aplicação importadora é quem instancia a estratégia
  concreta, monta o `Config` e injeta tudo no core e no middleware — exatamente o papel de um
  `cmd/.../main.go`. O `cmd/` atual deste repositório passa a ser um **exemplo executável**
  de composição, não parte da biblioteca.

**Isolamento de dependências (RN-01, RN-11)**
- Mover o código de `internal/` para o caminho público do módulo é condição necessária para
  importabilidade (`internal/` é restrito ao próprio módulo em Go).
- O Redis em subpacote garante que importar só o core/in-memory não arrasta `go-redis` para o
  binário do consumidor. Se for desejável que `go-redis` nem apareça no `go.mod` de quem não
  usa Redis, o subpacote Redis pode virar um **módulo Go separado** (decisão de empacotamento
  a validar; fora do escopo funcional desta spec).

**Convenções Go aplicáveis**
- `Allow` retorna `(bool, error)`; erros envolvidos com `%w` e nomes de variável de erro
  únicos (sem shadowing). Erros sentinela exportados para o consumidor distinguir via
  `errors.Is`.
- `context.Context` propagado em toda I/O da estratégia (RN-13) — o middleware passa o
  `ctx` da requisição, não `context.Background()`.
- Sem logger global no core; sem variáveis globais mutáveis; injeção explícita de
  dependências. Opções opcionais via **functional options**.
- Testabilidade: core testável com a estratégia in-memory (sem Redis, sem HTTP), em
  table-driven tests com testify; estratégia Redis testada à parte na borda.

## 8. Glossário (termos novos)

- **Strategy (estratégia de armazenamento):** porta do core que abstrai a contagem por
  janela e o bloqueio de chaves; implementada por in-memory e Redis.
- **Chave (key):** identificador sobre o qual o limite é aplicado — o token (header) ou, na
  ausência dele, o IP do cliente.
- **Janela (window):** intervalo de tempo dentro do qual os acessos de uma chave são
  contados; ao expirar, o contador reinicia.
- **Fail-open / fail-closed:** política de comportamento quando o backend de armazenamento
  falha — permitir (fail-open, padrão) ou negar (fail-closed) a requisição.
- **Ponto de composição (composition root):** único lugar (o `main` da aplicação
  importadora) onde estratégia concreta, configuração e middleware são instanciados e
  conectados.

## 9. Rastreabilidade (preenchido por /implement-spec)

- **Implementação:** `ratelimiter/{strategy.go,limiter.go}` (core),
  `ratelimiter/store/{memory,redisstore}/`, `ratelimiter/middleware/{nethttp,ginmw}/`,
  `ratelimiter/config/config.go` (helper opcional); `cmd/main.go` (exemplo de composição)
- **Testes:** `ratelimiter/limiter_test.go`, `ratelimiter/store/memory/memory_test.go`,
  `ratelimiter/middleware/nethttp/nethttp_test.go`,
  `ratelimiter/middleware/ginmw/ginmw_test.go`, `ratelimiter/config/config_test.go`
- **Doc:** `README.md` (importação, composição, opções, política de falha e tabela de erros)
- **Cobertura de regras:** RN-01 ✓ · RN-02 ✓ · RN-03 ✓ · RN-04 ✓ · RN-05 ✓ · RN-06 ✓ ·
  RN-07 ✓ · RN-08 ✓ · RN-09 ✓ · RN-10 ✓ · RN-11 ✓ · RN-12 ✓ · RN-13 ✓

> **Notas:** RN-01/RN-02 verificadas estruturalmente (código fora de `internal/`, core sem
> imports de framework/infra). `store/redis` é testado na borda (requer Redis real); o
> caminho de erro do backend que ele alimenta já é coberto no core via stub
> (fail-open/fail-closed).
