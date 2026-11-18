# Cache Redis do e-commerce (CACHE-01)

Cache na API das leituras **públicas** da loja (o navegador nunca fala com o Redis). Fail-open: Redis fora = API segue sem cache (pausa de 30 s após erro).

## Configuração
- `REDIS_URL=redis://192.168.168.106:48110/0` (RESP/TCP, não `http://`; vazio desliga). Senha só em `REDIS_PASSWORD`.
- `CACHE_APPNAME`: produção `rotaperfumes`, dev `rotaperfumes-dev` (o mesmo Redis serve os dois; o appname separa).

## Chave
- Dado: `<appname>:<area>:v<versao>[:<hash dos parâmetros>]`. Versão: contador `<appname>:<area>:versao`.
- Invalidar = `INCR` da versão (as entradas velhas expiram pelo TTL). A subida da API invalida todas as áreas.

## Áreas das telas da loja (`/` e `/loja`)
| Área | Rota | TTL | Invalida em |
| --- | --- | --- | --- |
| `vitrine` | `GET /api/loja/produtos` (sem `?q`) e `GET /api/loja/produtos/{id}` (ECOM-11) | 5 min | criar/editar/ativar produto, enviar/remover imagem |
| `filtros` | `GET /api/loja/filtros` | 15 min | criar/editar/ativar produto |
| `imagem_produto` | `GET /api/public/produtos/imagens/{arquivo}` | 1 h | enviar/remover imagem |

- Fora do cache: login, cadastro, Google, confirmação de e-mail e `/me` (POST ou sessão), busca livre `?q`, HEAD e respostas ≠ 200.
- Termos e política são HTML estático do Next (`frontend/public`), fora da API.
- Importação CSV de produtos (CLI) não invalida: o TTL cobre, ou reinicie a API.
- Diagnóstico: cabeçalho `X-Cache: HIT|MISS|BYPASS`. Teste real: `REDIS_TESTE_URL=redis://… go test ./tests/adapters/rediscache/`.
