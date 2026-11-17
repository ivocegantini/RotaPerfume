# Lote 43 de 2026-11-07: especificação (🟣 SecBrain)

> Gravada pelo 🤍 MegaBrain a partir do texto do 🟣 SecBrain. **R1 do SEC-73, conferido:** a cópia do keys.openpgp.org **não mostrava UID** (e-mail não verificado), então o segundo canal é fraco; o usuário aceitou o risco explicitamente em 2026-11-07.

Data: 2026-11-07. Caminhos: `SH` = `backend/shared`; `TD` = `SH/tests/deploy`. As linhas citadas são as da árvore em `4c32ab4`.

Decisões de segurança:
- **SEC-73:** aprovado, com uma ressalva: registrar o que foi visto no keys.openpgp.org (ver R1).
- **TST-28:** aprovado.
- **BACKUP-03:** aprovado.
- **BACKUP-04:** aprovado. É seguro: o parcial é o mesmo dump já validado.
- **BACKUP-05:** aprovado.
- **BACKUP-07:** aprovado, com a regra "troca de datadir recusada, salvo `--trocar-dados`, que passa pela guarda".

---

## SEC-73: fingerprint da chave apt da Cloudflare confirmado por um segundo canal

**Fato informado pelo MegaBrain como decisão do usuário (registrar no card):** keyserver.ubuntu.com e keys.openpgp.org servem a primária `CC94B39C77AE7342A68B89628A682D308D4E5E73` e a subchave `06C89DB3B80A8F4349697C76029E1444B7D9F50F`, iguais às de `deploy/cloudflare-apt.asc`.

**R1 (risco residual, registrar no card):**
- O keyserver.ubuntu.com aceita envio de qualquer pessoa, com qualquer UID. A igualdade lá prova que é a mesma chave, não que ela seja da Cloudflare.
- O canal forte é o keys.openpgp.org, porque lá a UID só aparece depois de verificar o e-mail. Anotar se a cópia de lá mostrava a UID verificada.
- Se não mostrava, o segundo canal é fraco, e o risco precisa ser aceito explicitamente.
- Opção para depois, mais independente: o repositório `cloudflare/cloudflared` no GitHub ou a documentação da Cloudflare.

**Onde o texto diz "falta confirmação" ou "só com --sem-tunel", e o que fazer em cada lugar:**

1. **`deploy/cloudflare-apt.fingerprint`, linhas 2 a 4.** Trocar por este texto (ASCII, como o resto do arquivo):
   ```
   # Origem: https://pkg.cloudflare.com/cloudflare-main.gpg (HTTPS da Cloudflare, baixada em 2026-11-06).
   # SEC-73: confirmado em 2026-11-07 por um segundo canal: keyserver.ubuntu.com e keys.openpgp.org
   # servem a mesma chave primaria e a subchave 06C89DB3B80A8F4349697C76029E1444B7D9F50F.
   ```
   - As linhas 5 e 6 ficam **intactas**: a linha útil continua sendo só `CC94B39C77AE7342A68B89628A682D308D4E5E73`.
   - O `conferir_chave_apt` (`deploy/restaurar-maquina.sh:674`) já ignora linhas `#`. Por isso a subchave no comentário não interfere.
2. **`docs/deploy-servidor.md:915`.** Trocar a frase em negrito por algo como:
   > "O fingerprint veio do HTTPS da Cloudflare e foi confirmado em 2026-11-07 por um segundo canal (SEC-73): keyserver.ubuntu.com e keys.openpgp.org servem a mesma chave e a subchave `06C8…F50F`. Ao trocar a chave, confirme o fingerprint novo por um segundo canal antes do commit e mude os dois arquivos juntos."
   - Manter o literal "segundo canal": o `TD/deploy09_estatico_linux_test.go:265` exige esse texto na §12.
   - A linha 923 ("Só em máquina real (ainda não feito)… o apt real, o pin e a assinatura") **não muda**: fala de validação em máquina real, não do fingerprint.
3. **`TD/deploy09_estatico_linux_test.go:33`.** Hoje exige `"confirmacao por um segundo canal"`, que vai sumir. Trocar por:
   - `NotContains(ToLower(fp), "ainda precisa")`;
   - `Contains` de `"sec-73"`, `"keyserver.ubuntu.com"`, `"keys.openpgp.org"` e `"06c89db3b80a8f4349697c76029e1444b7d9f50f"`.
   - Acrescentar no mesmo teste, com a saída do `gpg --show-keys` que ele já tem: a linha `fpr:` que segue a `sub:` tem o campo 10 igual a `06C89DB3B80A8F4349697C76029E1444B7D9F50F`. Assim o comentário não diverge do `.asc`.
   - O `require.Equal` da linha útil (linha 40) fica igual.
4. **`deploy/restaurar-maquina.sh`: nada muda.** Conferi com grep:
   - nenhuma mensagem, ajuda ou `--listar-manuais` fala em confirmação pendente;
   - o `--sem-tunel` continua como opção de ensaio (linhas 162, 237, 1162 e 1333).
5. **Kanban, a cargo do SubBrain:**
   - `tarefas/afazer.md:123` ("Até a conferência: … só com `--sem-tunel`") sai junto com o card;
   - `tarefas/feito.md:66` e `:141` são histórico: não editar, só citar o SEC-73 no fechamento;
   - `tarefas/lote-42-deploy-09-10-spec.md:207` também é histórico e fica como está.
   - Nada no `CLAUDE.md` nem na memória cita o tema.

**Testes:** só o item 3. Os testes de recusa de chave errada (`TD/deploy09_fluxo_linux_test.go:516-552`) continuam valendo sem mudança.

---

## TST-28: `-timeout=30m` no `go test`

**Ordem:** o `-timeout=30m` entra **antes** de `$(GO_TEST_FLAGS)`.
- O `cmd/go` e o binário de teste tratam o `-timeout` repetido como "o último vale". Assim, `GO_TEST_FLAGS=-timeout=45m` (ou `-timeout=0`) sobrescreve o valor fixo, e o usuário continua podendo mudar.
- O TestBrain deve confirmar isso empiricamente uma vez: um teste que dorme 2 s passa com `go test -timeout=1s -timeout=30m`.
- O `-timeout` vale **por pacote** (por binário de teste), não pelo `go test` inteiro.

**Makefile (deixar o `./...` logo depois de `go test`, porque testes de config procuram `"go test ./..."` e `"=== go test ./..."`):**

| Linha | Situação | Mudança |
|---|---|---|
| `Makefile:620` (`test-shared`) | obrigatória | `go test ./... -timeout=30m $(GO_TEST_FLAGS) -v -coverpkg=./... -coverprofile=coverage.txt` |
| `Makefile:623` (`cover-shared`) | obrigatória | `go test ./... -timeout=30m -coverpkg=./... -coverprofile=coverage.txt` |
| `Makefile:644` (`cover-combinada`, shared) | obrigatória | `-timeout=30m` logo depois do `./...` |
| `Makefile:628`, `:631`, `:645` (API) | recomendada | o mesmo `-timeout=30m`, por uniformidade |
| `Makefile:636-637` (`arch-check`) | não precisa | — |

- As linhas 623 e 644 também rodam o `tests/deploy` e estourariam os 10 min do mesmo jeito.
- O `test-integration` (`:661`) passa `GO_TEST_FLAGS="-p 1 -count=1"`, sem `-timeout`, então o 30m continua valendo. O `sec53_makefile_segredos_test.go:104` confere essa linha literal, que não muda.

**Jenkinsfile:**
- **Linhas 132-133:** `go vet ./... && go test ./... -timeout=30m` nas duas.
- **Orçamento:** o job tem 60 min (`:32`). No CI os testes com `unshare` pulam, então o `tests/deploy` é bem mais rápido. O 30m por pacote só garante que um teste travado gere pânico com o dump das goroutines antes de o job morrer.

**Teste de paridade** (novo `TD/tst28_timeout_go_test.go`, estático, sem build tag):
- (a) Na receita de `test-shared`, a linha `go test` contém `go test ./... -timeout=30m $(GO_TEST_FLAGS)`, nessa ordem.
- (b) Também `cover-shared` e `cover-combinada` (e as da API, se o BackBrain as mudar) têm o mesmo valor.
- (c) No `estagioJenkins(t, lerRaiz(t,"Jenkinsfile"), "Testes Go")`, existem exatamente 2 ocorrências de `go test ./...` e as duas têm `-timeout=30m`.
- (d) Um valor único: extrair com `-timeout=(\S+)` do Makefile e do Jenkinsfile; todos iguais e `time.ParseDuration < 60m`, lido do `timeout(time: 60`.
- (e) Opcional, dinâmico, com o stub de `go` dos testes de config: `make test-shared GO_TEST_FLAGS=-timeout=45m` registra `-timeout=30m -timeout=45m`, nessa ordem.

**Aceite do card:** `make -k test` verde **sem** `GO_TEST_FLAGS`.

---

## BACKUP-03: tolerância de relógio de 300 s nas guardas

**Regra:** a mesma do `backup-banco.sh:439/792`.
- É "data futura" quando `carimbo > agora + TOLERANCIA_RELOGIO_S` **ou** `mtime > agora + TOLERANCIA_RELOGIO_S`.
- A idade continua `agora - f <= janela*60`. Com `f` até 300 s no futuro, a idade fica negativa e passa.
- Efeito máximo: a janela cresce 5 min. É aceitável.

**Mudanças:**
- **Constante:** `readonly TOLERANCIA_RELOGIO_S=300` global, com o comentário igual ao de `backup-banco.sh:106`:
  - em `deploy/mariadb-docker.sh`, junto de `BACKUP_JANELA_LIVRE_MAX` (`:96`);
  - em `deploy/seed-producao.sh`, junto de `:75`.
  - Fica **fora** da função, para não quebrar a paridade byte a byte de `conferir_backup_recente` (`TD/backup01_backend_linux_test.go:311`).
- **Na função** (`mariadb-docker.sh:477` e `seed-producao.sh:356`, texto idêntico): trocar `(( f <= agora )) || die "… tem data futura; $dica"` por:
  `(( f <= agora + TOLERANCIA_RELOGIO_S )) || die "o backup mais recente ($nome) tem data futura (mais de ${TOLERANCIA_RELOGIO_S} s a frente do relogio); $dica"`
- **Fora do escopo:** `restaurar-maquina.sh:1156`, que só gera aviso.

**Paridade (teste novo ou acréscimo no `:311`):**
- `(?m)^readonly TOLERANCIA_RELOGIO_S=(\d+)$` aparece exatamente uma vez em cada um dos três scripts, com valor igual a `300`.
- As duas funções contêm `agora + TOLERANCIA_RELOGIO_S` e não contêm `f <= agora ))`.

**Bordas, nas duas guardas (`--recriar` e `--forcar`)** (margem de 60 s para não ficar instável):

| Caso | Resultado |
|---|---|
| carimbo +240 s | aceito |
| carimbo +360 s | recusa "data futura", sem `docker rm` nem registro |
| mtime +240 s (`os.Chtimes`, carimbo agora) | aceito |
| mtime +360 s | recusa |
| +2 h (já existe) | continua com "data futura" |

---

## BACKUP-04: aceitar `parcial-*` na guarda

**O que um parcial contém** (`backup-banco.sh:725-738` e `676-690`):
- É o **mesmo dump** de um par completo, publicado depois de passar por `conferir_dump` (gzip -t, marcador na última linha, ≥13 tabelas, `usuarios`/`clientes`/`vendedores` com dados em `usuarios`, piso de 4096 bytes) e por `conferir_queda_dump` (50%, ou `--aceitar-reducao` registrado).
- Fica com root e modo 600 (`fixar_arquivo`).
- **Falta só o tar de imagens.** A retenção mantém os 3 parciais mais novos.

**Avaliação: seguro.**
- O `--recriar` e o `seed --forcar` não tocam no volume de imagens.
- A guarda já não exige o tar: o teste "backup recente sem exigir o arquivo de imagens" (`TD/backup01_backend_linux_test.go:1122`) aceita um `rotaperfumes-*.sql.gz` órfão. Recusar o parcial é inconsistente com isso.
- Hoje, com as imagens falhando de forma repetida, a única saída é o `--sem-backup-motivo`, que **não confere nada**. Aceitar o parcial é estritamente melhor.
- **Restrições que ficam:**
  - `pre-restauracao-*` continua recusado;
  - o `restaurar-maquina.sh` continua recusando o parcial (precisa das imagens): `TD/deploy09_recusas_linux_test.go:183` fica como está;
  - o `--status` e a retenção continuam sem contar o parcial.

**Comportamento** (função idêntica nos dois scripts):
- **Candidatos:** `^rotaperfumes-(C)(-manual)?\.sql\.gz$` e `^parcial-rotaperfumes-(C)(-manual)?\.sql\.gz$`, com `C=[0-9]{8}-[0-9]{6}Z`. Ler os dois globs.
- **Escolha:** o mais novo **pelo carimbo** (não pela ordem do glob), com a chave `C` + rank (completo = 2, parcial = 1); no mesmo carimbo vence o completo. Comparar com `[[ $chave > $melhor ]]` (o `LC_ALL=C` já é local).
- **Sem recuo:** o escolhido passa pelas **mesmas** conferências de hoje (regular, sem link, `0 600`, carimbo, futuro com a folga do BACKUP-03, janela, `gzip -t`, marcador). Se falhar, recusa, sem recuar para um mais antigo (mesma regra de "o mais novo corrompido não recua").
- **Carimbo:** a regex da linha 471/350 passa a `^(parcial-)?rotaperfumes-(…)`. Ajustar os índices do `BASH_REMATCH`.
- **Aviso ao aceitar um parcial:**
  `aviso "o backup usado e parcial ($nome): o dump vale para o banco, mas o tar de imagens desse backup falhou; confira com journalctl -u rotaperfumes-backup-banco.service"`
- **Registro:** formato igual. O `backup=<caminho>` já mostra o `parcial-`.
- **Tamanho:** o card cita "tamanho", mas a guarda **não** confere tamanho hoje, nem para o par completo. Não ampliar o escopo; registrar como desvio.

**Textos a atualizar:**
- `mariadb-docker.sh:453-457`, `seed-producao.sh:334-336`, `backup-banco.sh:34-37`;
- `docs/deploy-servidor.md:489` ("não satisfaz … a guarda do `--recriar`") e `:491` ("Nome `pre-restauracao-*` ou `parcial-*` não vale" vira "só `pre-restauracao-*` não vale; `parcial-*` vale, com aviso");
- a tabela da `:571` pode citar que a guarda aceita o parcial.

**Testes existentes que mudam de recusa para aceite:**
- `TD/backup01_backend_linux_test.go:1092` ("parcial nao vale");
- `TD/backup01_testbrain_linux_test.go:1391` ("parcial novo com bom antigo": passa a aceitar e registrar o parcial);
- `:1465` ("parcial mais novo": `backup=` passa a ser o parcial);
- `:1643` (seed "parcial").

**Testes novos, nas duas guardas:**

| Caso | Resultado |
|---|---|
| parcial recente sozinho | aceito, com aviso e registro do parcial |
| parcial `-manual` | aceito |
| parcial mais novo corrompido + completo bom dentro da janela | recusa "corrompido", sem recuar |
| parcial com symlink, modo 644, sem marcador, futuro +360 s, antigo | recusa |
| completo e parcial no mesmo carimbo | o completo é escolhido |
| `pre-restauracao-*` mais novo | continua ignorado |

---

## BACKUP-05: a retenção relê todos os pares mantidos da série

**Comportamento** (`backup-banco.sh:458-482`, `aplicar_retencao`):
- **Quando:** só se `n > 0` (há o que apagar). Mantém o "sem nada a apagar não relê", o teste da `:1391`, e não falha dentro dos primeiros N dias.
- **Por que só aí:** o risco do card só existe quando algo vai ser apagado.
- **Quem é relido** (sem repetição): o par novo, mais **todos os pares que ficam na série** (`pares[n..fim]`), mais os dois mais novos de `todos` (regra atual).
- **Quem não é relido:** os pares que vão ser apagados. Apagar um par corrompido não perde nada.
- **Reprovação:** `conferir_par_publicado`, como hoje (gzip -t no dump e no tar, marcador). Qualquer reprovação dá `die`, **sem apagar nada**, com o par novo intacto.
- **Mensagem:**
  `"o par X foi reprovado (ERRO): nada foi apagado e o par novo ficou intacto. Confira esse arquivo; se estiver corrompido, mova o par (dump e imagens) para fora de $BACKUP_DIR"`
  O prefixo "foi reprovado" é o que os testes atuais procuram.
- **Sinalização:** a unit termina com falha, e `Result=failed` aparece no `--status`. O "último bom", o mais antigo da lista de remoção, continua no disco.

**Textos a atualizar:**
- `backup-banco.sh:30-32` e `:458-460` ("Os demais nao sao relidos" vira "relê todos os pares que ficam na série, mais os dois mais novos de todas");
- `docs/deploy-servidor.md:485` ("Os pares mais antigos não são relidos" vira "Antes de apagar, relê todos os pares que ficam na série…").

**Testes** (novo, ou ampliar o `TestBACKUP01_Retencao_ParCorrompido`, `TD/backup01_backend_linux_test.go:1342`):
- **Cenário:** `RETENCAO_DIARIOS=3`, pares `d1 < d2 < d3 < d4` e o novo. Fica `n=2`: apagaria `d1` e `d2` e manteria `d3`, `d4` e o novo.
- **Corromper `d3`** (o "meio mantido", que hoje não é relido): rc=1, a saída cita "o par rotaperfumes-d3… foi reprovado", não há "par removido", `d1` e `d2` continuam existindo, e o par novo também. Fazer as três variantes: gzip ruim no dump, dump sem marcador e tar com gzip ruim.
- **Corromper `d1`** (vai ser apagado): rc=0, e `d1` e `d2` são removidos.
- **Todos bons:** rc=0, e `d1` e `d2` são removidos.
- **Existente** "pares antigos nao sao relidos" (`:1382`): continua passando, porque o `meio` está na lista de remoção. Renomear para "pares que saem nao sao relidos".

**Risco de custo:** com 14 pares, cada execução diária faz `gzip -t` em todos os tars de imagens e lê os dumps de ponta a ponta. Registrar como aceito, ou medir no servidor. No teste, usar poucos pares para não somar tempo ao pacote (hoje ~580 s).

---

## BACKUP-07: troca de `--dados` num container existente

**Detecção** (`mariadb-docker.sh`, logo depois das linhas 396-399, com `CONTAINER_ID` não vazio, antes da 434):
```
docker inspect -f '{{range .Mounts}}{{if eq .Destination "/var/lib/mysql"}}{{.Type}}|{{.Source}}{{"\n"}}{{end}}{{end}}' "$CONTAINER"
```
- A saída precisa ser **exatamente uma** linha `bind|<caminho absoluto>`, com o caminho no charset da linha 289.
- Se não for (inspect falhou, vazio, duas linhas, `volume`): `die "nao consegui ler o datadir atual do container $CONTAINER (montagem de /var/lib/mysql: '<saida>'): confira com docker inspect; nada foi mudado"`.
- `DADOS_ATUAL` = essa origem. A comparação é por `realpath -m` dos dois lados, como na linha 302.

**Regra:**
1. **Sem `--recriar`, com o datadir diferente:** entra no `DIVERGE` (`:435-449`) como `"datadir $DADOS_DIR (atual: $DADOS_ATUAL)"`.
2. **Com `--recriar`, diferente e sem `--trocar-dados`:** recusa **sempre**, mesmo com backup recente ou `--sem-backup-motivo`, antes de qualquer mudança:
   `die "--dados $DADOS_DIR difere do datadir atual do container ($DADOS_ATUAL): o --recriar subiria outro banco (vazio, se o diretorio estiver vazio) e os dados atuais ficariam fora do site. Para mover o datadir: backup, copie os dados para o novo caminho e use --recriar --trocar-dados"`
3. **Com `--recriar --trocar-dados`:**
   - se `$DADOS_DIR/mysql` não existe: `die "--trocar-dados: $DADOS_DIR nao tem dados (mysql/ ausente): trocar para um datadir vazio nunca e aceito"`, sem válvula;
   - se existe: a guarda **sempre** dispara (backup recente, ou `--sem-backup-motivo`). O registro (`:929`) ganha **no fim** ` dados_antes=$DADOS_ATUAL dados=$DADOS_DIR`, só nesse caso.
4. **`--trocar-dados` sem `--recriar`, ou com o mesmo caminho:** `die "--trocar-dados so com --recriar e um --dados diferente do atual"`.
5. **Condição de disparo da guarda** (`:487`): passa a `[[ -d "$DADOS_ATUAL/mysql" || -d "$DADOS_DIR/mysql" ]]`, com `DADOS_ATUAL` vazio quando não há container.

**Stub** (`TD/mariadb_docker_stubs_test.go`; não mexer na linha `rm)`, que os testes do BACKUP-01 trocam por texto):
- No `run -d` (linhas 65-73), acrescentar:
  `-v) [[ "$2" == *:/var/lib/mysql ]] && echo "bind|${2%:/var/lib/mysql}" >"$S/dados_montado"; shift 2 ;;`
- No `inspect` (linhas 45-53), acrescentar `*.Mounts*) cat "$S/dados_montado" 2>/dev/null ;;`.
- Os ensaios sempre criam o container pelo `run -d` (o `novaGuarda` roda `--nova-ca` antes), então os testes atuais passam com o mesmo `--dados`. Para simular as falhas, o teste reescreve `state/dados_montado`.

**Testes** (todos os de recusa: sem `docker rm`, sem `docker build`, sem `recriar.registro`):

| Caso | Resultado |
|---|---|
| `--recriar --dados <dir vazio 700>` com backup recente | recusa "difere do datadir atual" |
| o mesmo com `--sem-backup-motivo` | recusa |
| `--recriar` com o caminho original | aceito (regressão) |
| `--trocar-dados` com novo vazio | recusa "nunca e aceito" |
| `--trocar-dados` com novo populado, sem backup | "nenhum backup" |
| `--trocar-dados` com novo populado e backup recente | aceito: `docker run -v <novo>:/var/lib/mysql`, registro com `dados_antes=` e `dados=` |
| `--trocar-dados` sem `--recriar`, ou com o mesmo caminho | recusa |
| sem `--recriar`, `--dados` diferente | DIVERGE e "rode de novo com --recriar" |
| `dados_montado` vazio, com duas linhas ou `volume\|x` | recusa "nao consegui ler o datadir atual" |

**Riscos que restam (registrar):**
- Quem roda `docker rm` à mão e depois o script com outro `--dados` sobe um banco vazio sem guarda. A guarda protege contra engano, não contra o root.
- Datadir perdido em disco morto: o caminho é o `restaurar-maquina.sh`.

---

## Notas gerais

- **Ordem sugerida:** BACKUP-03 e BACKUP-04 juntos (a mesma função nos dois scripts, a mesma paridade), depois BACKUP-07, BACKUP-05, TST-28 e SEC-73.
- **Custo dos testes:** cada ensaio com stub custa alguns segundos. Agrupar casos em tabelas e reaproveitar o `novaGuarda`/`novoSeedPreenchido` para não aproximar o pacote dos 30 min.

**Arquivos:**
- `/home/residentivo/.copilot/repos/RotaPerfume/deploy/cloudflare-apt.fingerprint`
- `/home/residentivo/.copilot/repos/RotaPerfume/docs/deploy-servidor.md`
- `/home/residentivo/.copilot/repos/RotaPerfume/backend/shared/tests/deploy/deploy09_estatico_linux_test.go`
- `/home/residentivo/.copilot/repos/RotaPerfume/Makefile`
- `/home/residentivo/.copilot/repos/RotaPerfume/Jenkinsfile`
- `/home/residentivo/.copilot/repos/RotaPerfume/deploy/backup-banco.sh`
- `/home/residentivo/.copilot/repos/RotaPerfume/deploy/mariadb-docker.sh`
- `/home/residentivo/.copilot/repos/RotaPerfume/deploy/seed-producao.sh`
- `/home/residentivo/.copilot/repos/RotaPerfume/backend/shared/tests/deploy/mariadb_docker_stubs_test.go`
- `/home/residentivo/.copilot/repos/RotaPerfume/backend/shared/tests/deploy/backup01_backend_linux_test.go`
- `/home/residentivo/.copilot/repos/RotaPerfume/backend/shared/tests/deploy/backup01_testbrain_linux_test.go`
