# Roteiro de teste: produtos e imagem do produto

**Escopo:** `/admin/produtos` (CRUD, inativar) e a imagem do produto (envio, troca, remoção, leitura pública e geração em lote).
**Pré-requisitos:** `make dev-api`, `make dev-frontend` (abra em `http://localhost:3000`, mesmo host da API por causa do `Cross-Origin-Resource-Policy: same-site`), login como admin. Produtos sem imagem: `make db-gerar-imagens-produtos`.
**Tempo estimado:** 20 a 30 minutos.

Arquivos de teste: JPEG < 1 MB, PNG renomeado para `foto.jpg`, arquivo > 1 MB, SVG renomeado para `logo.png` e `.txt` renomeado para `teste.png`.

## 1. Produtos

| Passo | Ação | Esperado |
| --- | --- | --- |
| 1.1 | Abrir `/admin/produtos` | 1ª coluna "Imagem" com miniatura; sem imagem, quadro cinza "Sem imagem". |
| 1.2 | Criar produto | "Produto "<nome>" criado com sucesso."; 201 com `created_at`/`updated_at` de agora. O modal "Novo" não tem a seção Imagem. |
| 1.3 | Editar e salvar | "Produto "<nome>" atualizado com sucesso.", some em ~4 s; sair da tela antes disso não gera erro no Console. |
| 1.4 | Inativar e reativar pelo status | Alterna o `ativo`; produto inativo some da vitrine da loja. |
| 1.5 | Usuário normal | Sem "Produtos" no menu; a seção Imagem só mostra a imagem, sem seletor nem botões. API de envio/remoção: 403. |

## 2. Imagem pela tela

| Passo | Ação | Esperado |
| --- | --- | --- |
| 2.1 | Editar → seção Imagem → "Arquivo (JPEG, PNG ou WebP, ate 1 MB)" com o JPEG | Prévia no lugar da atual; "Descartar selecao" desiste. |
| 2.2 | "Enviar imagem" | "Imagem enviada com sucesso."; miniatura atualizada sem recarregar. O envio independe de "Salvar alterações". |
| 2.3 | Network do envio | `POST /api/produtos/{id}/imagem` com `X-Requested-With: XMLHttpRequest` e `multipart/form-data`; `imagem_url` = `/api/public/produtos/imagens/<uuid>.jpg`. |
| 2.4 | PNG renomeado para `.jpg` | Aceito; a extensão final vem do conteúdo (`.png`). WebP é gravado como PNG. |
| 2.5 | "Trocar imagem" com outro arquivo | Novo UUID; a URL antiga dá 404 `"imagem não encontrada"` e o arquivo antigo some de `backend/rotaperfumes-api/data/produtos-imagens/`. |
| 2.6 | Arquivo > 1 MB | "imagem excede 1 MB", sem chamar a API. |
| 2.7 | SVG ou `.txt` renomeados | "arquivo de imagem inválido (use JPEG, PNG ou WebP)" (se sair a requisição, 400 com a mesma mensagem); a imagem anterior continua. |
| 2.8 | "Remover imagem" | Pergunta "Remover a imagem do produto "<nome>"?"; cancelar não remove; confirmar: "Imagem removida com sucesso.", "Sem imagem" e URL antiga 404. |

## 3. API (pasta Produtos do Postman ou curl)

| Passo | Chamada | Esperado |
| --- | --- | --- |
| 3.1 | Upload sem `X-Requested-With` | 403 `"requisição de upload recusada"`. |
| 3.2 | Upload sem token / com vendedor | 401 / 403. |
| 3.3 | `/api/produtos/0/imagem` / `/api/produtos/999999/imagem` | 400 `"id inválido"` / 404 `"produto não encontrado"`, sem arquivo novo. |
| 3.4 | `GET` público sem token | 200, `Content-Type` de imagem, `Cache-Control: public, max-age=31536000, immutable`, `X-Content-Type-Options: nosniff`, `Cross-Origin-Resource-Policy: same-site`, sem `Set-Cookie`. |
| 3.5 | `curl -I` / `curl -H "Range: bytes=0-99"` | 200 sem corpo / 206. |
| 3.6 | Nome em maiúsculas, `.jpeg`, `.svg` ou `.tmp-<uuid>` | 404. |
| 3.7 | `DELETE` duas vezes | 200 nas duas, `imagem_url: null`. |

## 4. Geração em lote

| Passo | Comando | Esperado |
| --- | --- | --- |
| 4.1 | `cd backend/rotaperfumes-api && go run ./cmd/gerarimagens` (sem `-confirmar`) | Não grava nada. |
| 4.2 | `make db-gerar-imagens-produtos GERAR_IMAGENS_FLAGS=-dry-run` | Lista os produtos sem gravar. |
| 4.3 | `make db-gerar-imagens-produtos` | PNG 600x600 com o nome na diagonal só para quem não tem imagem; loga os totais. `-sobrescrever` troca todas. |

## 5. Limpeza

- Apague os produtos de teste (ou inative) e remova as imagens enviadas pela tela.
