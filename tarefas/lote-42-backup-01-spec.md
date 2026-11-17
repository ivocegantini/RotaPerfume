# BACKUP-01: especificação de segurança

Nada foi executado nem editado, e nenhum segredo foi lido. Os caminhos são relativos a `/home/residentivo/.copilot/repos/RotaPerfume`.

## Resumo
- **Forma:** systemd timer, com script instalado fora do checkout.
- **Conta:** nova, `rotaperfumes_backup@172.30.42.21`, só com `SELECT`.
- **Restauração:** só em container com `--network none`.
- **`--recriar`:** exige backup de até 60 minutos, ou motivo registrado.
- **Maior risco que resta:** o backup fica no mesmo disco e ninguém é avisado de falha, até você decidir D1 e D2.

## Achados que mudam o card
1. **O backup manual documentado pode gerar arquivo ruim sem avisar.** `docs/deploy-servidor.md:430-437` não usa `pipefail` nem valida o resultado, então um dump que falha ainda gera um `.gz` válido. Antes de rodar o item 2 da limpeza (`tarefas/afazer.md:417`), use `set -o pipefail`, confira o código de saída e veja se `zcat ARQ | tail -n1` começa com `-- Dump completed`.
2. **Produção diverge da decisão "banco só local".** `tarefas/afazer.md:409` registra a 3306 publicada para `192.168.168.105`. Rodar `mariadb-docker.sh` sem `--dev-ip` nesse estado acusa divergência e exige `--recriar` (`deploy/mariadb-docker.sh:345-361`). Confirme o estado antes da instalação (seção 2).
3. **O dump sozinho não recupera os logins.** Os hashes dependem de `PASSWORD_PEPPER` (`deploy/api.env.example:51`), que fica fora do banco. Guarde o pepper em local separado: junto do dump, ele perde a função.
4. **O CasaOS não serve de cópia externa.** Ele está no mesmo IP da produção (`CLAUDE.md`: `192.168.168.106:3307`).
5. **`--dev-ip` altera o risco descrito no card.** A conta `casaos` só deixa de existir quando o item 7 da limpeza rodar.

## Requisitos

### 1. Forma
- **Agendamento: systemd timer, não cron.** O repo já instala unit assim (`deploy/mariadb-docker.sh:100-102,640-673`), e o timer oferece `Persistent=true`, journal e timeout.
- **Scripts:**
  - `deploy/backup-banco.sh`, com `--instalar`, `--agora` e `--status`.
  - `deploy/verificar-backup.sh`, separado e sem credencial nem rede.
- **O root nunca executa do checkout.** `--instalar` copia para `/usr/local/sbin/rotaperfumes-backup-banco` (700, root). Sem isso, um push na `main` poderia virar código rodando como root.
- **`--instalar`:** roda um primeiro backup e falha se ele falhar.
- **Timer:** `OnCalendar=*-*-* 03:30`, `Persistent=true`, `RandomizedDelaySec=15m`.
- **Service:**
  - `Type=oneshot`, `UMask=0077`, `NoNewPrivileges=yes`, `PrivateTmp=yes`, `ProtectSystem=strict`.
  - `ReadWritePaths` só com o diretório de backup, e `Requires/After=docker.service`.
  - `TimeoutStartSec=2h`.
  - `Restart=on-failure` com `RestartSec=15min`, limitado a 3 tentativas (cobre o banco ainda subindo após o boot).
- **Destino:** `/var/backups/rotaperfumes` (root, 700), validado com `conferir_dir_privado` (`deploy/mariadb-docker.sh:268-292`) e disjunto dos diretórios de dados, SSL, segredos e CA. O script recusa e não corrige.
- **Imagem:** a mesma string fixada. Inclua o novo script nos testes de paridade (`backend/shared/tests/deploy/deploy07_seed_producao_test.go:46-49`) e em `IMAGENS_FIXADAS_EM` (`Makefile:718`). Depois de trocar o digest, é preciso rodar `--instalar` de novo; isso entra nos passos de `deploy/mariadb-docker.sh:43-49`.

### 2. Conta de backup

| Critério | `rotaperfumes_admin` | `rotaperfumes_backup` (recomendada) |
|---|---|---|
| Privilégio | `ALL` em `rotaperfumes` (`:787`) | só `SELECT` |
| IP | `.20`, compartilhado | `.21`, exclusivo |
| Bug ou uso errado da credencial | pode destruir dados | não grava |
| Mudança em `mariadb-docker.sh` | nenhuma | pequena |

O conflito no `.20` é real: o estágio "CA do banco" usa esse IP em cada deploy (`Jenkinsfile:208`), e o Jenkins consulta a `main` a cada 2 minutos (`docs/deploy-servidor.md:445`). O `admin_sql`, o seed e o teste TLS do próprio `mariadb-docker.sh` (`:860-861`) usam o mesmo IP. Um dump longo faria o deploy falhar, ou o contrário.

**Limite honesto:** a conta não protege contra root comprometido, que já lê tudo. Ela limita erro e bug, e elimina o conflito de IP.

**Alterações em `mariadb-docker.sh`:**
- `IP_BACKUP=$REDE_PREFIXO.21` (`:295`). O `.21` fica fora da faixa dinâmica `.128/25` (`:584-585`).
- Senha gerada na lista de `:558`.
- Conta em `DESEJADAS` (`:766`) e em `sql_contas` (`:772`): `ed25519`, `REQUIRE SSL`, `GRANT SELECT ON rotaperfumes.*`.
- Conta nas listas de `:834`, `:845` e `:853`.
- Ajuda em `:128-129` e tabela em `docs/deploy-servidor.md:197`.
- Option file em `$SEGREDOS_DIR/backup-cliente/rotaperfumes_backup.cnf` (400, root). **Nunca em `container/`**, que é montado no container do banco (`:709`).

**Sem `TRIGGER` nem `EVENT`:** esses privilégios permitem criar objetos, então a conta deixaria de ser só leitura. Hoje `sql/` não tem trigger, view, rotina ou evento (busca sem resultados). Um teste de repositório deve falhar se algum deles surgir, forçando a revisão do `GRANT` e das flags.

**Instalação na produção já ativa:**
1. Atualize o checkout (`tarefas/afazer.md:411,421`: `/root/rotaperfumes` não é checkout git).
2. Rode `mariadb-docker.sh` **sem `--recriar`**, com o mesmo `--dev-ip` em uso, se a porta ainda estiver publicada. O container não reinicia (`:723`).
3. Rode em horário calmo: há um `REVOKE`+`GRANT` rápido nas contas (`:780-787`) e o firewall é recarregado (`:673`).
4. Rode `backup-banco.sh --instalar`.
5. Só depois rode o item 7 da limpeza (`--recriar`). Assim não há dependência circular com a guarda.

### 3. Integridade e falha

**Comando:**
- Em `docker run`: `--rm -i --name rotaperfumes-backup-cliente --network rotaperfumes-db --ip 172.30.42.21 --read-only --cap-drop ALL --security-opt no-new-privileges:true`, com CA e `.cnf` montados read-only.
- Em `mariadb-dump`: `--defaults-extra-file` como primeiro argumento, depois `--ssl-ca --ssl-verify-server-cert --single-transaction --skip-lock-tables --no-tablespaces --skip-add-drop-table --skip-triggers --hex-blob --default-character-set=utf8mb4 rotaperfumes`.

**Flags proibidas:**
- `-t`: o TTY corrompe o stream.
- `--databases`, `-B`, `--all-databases`: gravam `USE rotaperfumes`, e a restauração iria para a produção mesmo com outro banco na linha de comando.
- `--compact`, `--skip-comments`: removem o marcador final.
- `--skip-tz-utc`: altera `TIMESTAMP` restaurados.

`--skip-add-drop-table` é uma proteção deliberada: restaurar sobre tabelas existentes falha em vez de sobrescrever. Por isso, o `IMPORTAR_DUMP` com esse dump também falha em banco populado (`Jenkinsfile:234`).

**Ordem obrigatória (qualquer falha encerra sem tocar nos backups bons):**
1. `set -euo pipefail`, `umask 077`, sem `set -x`.
2. Trava com `flock`. Trava ocupada conta como falha, nunca como sucesso silencioso.
3. `docker rm -f` só no nome exato do cliente, para limpar sobra de execução anterior.
4. Conferir espaço: livre ≥ 2× o último backup + 1 GiB de reserva. O dump divide o disco com o datadir, então encher o disco derruba o banco.
5. Gravar em `.tmp-*` criado por `mktemp` no mesmo diretório. `trap` em `EXIT`, `TERM` e `INT` apaga o temporário e o cliente.
6. Conferir os dois códigos de saída (`PIPESTATUS`).
7. `gzip -t`.
8. **Última linha** começa com `-- Dump completed`. Procurar no arquivo inteiro não serve, porque esse texto pode estar em um dado de cliente.
9. Pelo menos 13 `CREATE TABLE` (`docs/deploy-servidor.md:682`), com `usuarios`, `clientes` e `vendedores` e dados em `usuarios`. Assim, um banco esvaziado não vira "backup bom".
10. Tamanho ≥ 4 KiB e ≥ 50% do último bom, salvo `--aceitar-reducao`, usada uma vez e registrada.
11. `chmod 600`, conferir dono e modo, `sync`, `mv` atômico recusando destino existente, `sync` do diretório.
12. Só então, retenção.

**Armadilha de implementação:** `zcat | grep -q` sob `pipefail` gera falso negativo por SIGPIPE. Use `grep -c` ou `awk`.

**Retenção:**
- Por contagem de backups bons, nunca por data: um servidor desligado por semanas não perde tudo.
- Mínimo aceito de 2. Valor zero ou não numérico é recusado.
- Apaga só arquivos regulares que casem com `^rotaperfumes-[0-9]{8}-[0-9]{6}Z(-manual)?\.sql\.gz$`, sem `rm -r`.
- `--agora` usa a série `-manual`, com contagem própria, para não expulsar as diárias.
- Disco cheio nunca dispara apagamento.

**Log e saída:**
- Journal só com caminho, tamanho, duração e etapa que falhou; sem linhas do dump.
- Código 0 em sucesso, 2 em uso incorreto e diferente de zero em qualquer outra falha.

**Como o operador descobre que parou:**
- `--status` sai com erro se o último backup bom tiver mais de 26 h, se o timer estiver inativo ou se a última execução falhou.
- Sozinho, isso depende de alguém consultar. **Sem D2 aprovado, uma falha continua invisível.**

### 4. Confidencialidade
- **Senha:** só no `.cnf` montado, como já é feito em `deploy/mariadb-docker.sh:572-573`. Isso a tira do `argv`, do `ps` e do `docker inspect`.
- **`-e MYSQL_PWD` não atende ao requisito:** o valor fica no `Config.Env` do container (`deploy/seed-producao.sh:467`, `Jenkinsfile:252`). O risco prático é moderado, pois o grupo `docker` já equivale a root, mas evita vazamento em diagnósticos. Vale abrir um card separado para os usos antigos.
- **Execução do cliente:** como root com `--cap-drop ALL`, que ainda lê o arquivo 400 do próprio root. Validar no descartável.
- **Remoção:** `rm` comum. `shred` não é confiável em SSD ou disco com journal; a proteção real é cifrar o disco.
- **Cópia e dados reais:** o dump de produção não vai para o CasaOS, para máquina de dev nem para agentes. Os testes usam dados sintéticos.

### 5. Guarda antes de `--recriar`
A troca de digest passa por `--recriar`: sem ele, o script só avisa (`:725-727`). O único `docker rm` está em `:688-693`.

- **Local:** depois de `:362` e antes de `:380`, respeitando "nada é mudado antes" (`:18-19`).
- **Condição:** `CONTAINER_ID` não vazio, `$RECRIAR` e `$DADOS_DIR/mysql` existente. É preciso calcular isso ali, pois `DADOS_INICIALIZADOS` só nasce em `:545`.
- **Exigência:** o backup mais recente do diretório tem de ser arquivo regular, sem symlink, root, 600, ter até 60 minutos sem data futura, passar em `gzip -t` e ter o marcador na última linha.
- **Mensagem de recusa:** indica `sudo rotaperfumes-backup-banco --agora`.
- **Forma de ignorar:** só `--sem-backup-motivo TEXTO`, seguindo `--casaos-native-motivo` (`:316-318,816-820`): 10 a 300 caracteres imprimíveis, gravado em arquivo 600 **antes** do `docker rm`, com aviso. Sem variável de ambiente nem detecção de teste. Sem `--recriar`, a opção é recusada.
- **Registro:** cada `--recriar` grava em arquivo 600 a data, o backup usado ou o motivo, e a imagem base.
- **Container parado:** como não dá para gerar backup novo, o caminho é o motivo.

**Por que não quebra o existente:**
- Primeira instalação não tem container, então a guarda não dispara.
- Os ensaios atuais continuam verdes: nenhum teste cria `dados/mysql` (busca sem resultados) e o stub também não (`mariadb_docker_stubs_test.go:55-70`).
- **Detalhe obrigatório:** rode `conferir_dir_privado` no `--backup-dir` só quando a guarda disparar. No user namespace, ancestrais reais aparecem com overflow uid, então a validação sempre ativa quebraria os ensaios.
- Novos ensaios passam `--backup-dir` dentro de `$FAKE`, com fixture de `dados/mysql`.

**Risco que fica (card à parte, baixa):** `--recriar --dados /vazio` não dispara a guarda e sobe um banco vazio, embora os dados antigos continuem no caminho original.

### 6. Restauração

**`deploy/verificar-backup.sh ARQ`:**
- Sobe um container descartável da imagem fixada, com `--network none`, `--cap-drop ALL`, `--user mysql`, `--read-only` e tmpfs limitado para datadir, `/run/mysqld` e `/tmp`.
- Não tem opção de host, porta, usuário, senha, rede ou container, nem monta `SEGREDOS_DIR` ou o datadir. Assim, não existe caminho até a produção.
- Recusa arquivo que não seja regular, passe em `gzip -t` ou tenha `USE`/`CREATE DATABASE` no início de linha.
- Imprime só contagens, nunca linhas de dados.
- O `trap` remove apenas o ID do container que o próprio script criou.
- **Critério de aprovação:** importação com código 0, pelo menos 13 tabelas, um admin em `usuarios` e `clientes`/`vendedores` com linhas.
- **Frequência:** roda no servidor depois de `--instalar`, mensalmente e antes de trocar digest. `--status` avisa se a última verificação tiver mais de 35 dias.

**Restauração real na produção:**
- Só runbook, sem script novo e sem alvo no Makefile (SEC-46 continua valendo, `Makefile:373`).
- **Sequência do runbook:**
  1. Parar a API (`Jenkinsfile:235`).
  2. Fazer backup do estado atual, mesmo danificado.
  3. Verificar o arquivo com `verificar-backup.sh`.
  4. Esvaziar o banco conscientemente; o dump não apaga tabelas.
  5. Importar pelo `admin_sql` existente (`docs/deploy-servidor.md:666-673`).
  6. Rodar `TRUNCATE refresh_tokens` (`Jenkinsfile:263`).
  7. Aplicar migrações mais novas que o dump.
  8. Conferir o Passo 7.
- **Pré-requisito documentado:** o mesmo `PASSWORD_PEPPER`.
- **Perda do servidor:** documentar também a nova instalação (`mariadb-docker.sh`) antes da importação, pois as contas não estão no dump.

### 7. Testes obrigatórios (TestBrain)

**Ensaio com stubs** (padrão de `mariadb_docker_stubs_test.go`), todos sem alterar o último backup bom e sem rodar a retenção:
- `mariadb-dump` sai com erro depois de escrever dados.
- `gzip` falha, incluindo disco cheio.
- Sem marcador, marcador só no meio do arquivo, dump vazio, tabelas faltando, `usuarios` sem dados.
- Queda acima de 50%, com e sem `--aceitar-reducao`.
- Espaço insuficiente antes de iniciar.
- Trava ocupada.
- SIGTERM: não sobra `.tmp-*` nem cliente.
- Cliente antigo preso de execução anterior.
- Regressão do SIGPIPE com arquivo grande.

**Retenção:**
- Retenção de 0, negativa e não numérica recusada.
- Nomes parecidos, symlink e subdiretório nunca apagados.
- Datas antigas não apagam os últimos N.
- Backups manuais não expulsam os diários.

**Permissões:**
- Diretório 755, dono errado, symlink e ancestral gravável recusados.
- Backup final sempre 600.

**Segredos:**
- Sentinela da senha ausente em `FAKE/log`, na saída e no `argv` do stub.
- Ausência de `-e MYSQL_PWD`, `-t` e flags proibidas.
- Sem `set -x`.

**Guarda:**
- Sem backup, antigo, com data futura, modo 644, symlink, corrompido ou sem marcador: recusa **sem** `docker rm` e sem nenhuma mudança anterior.
- Motivo curto, com quebra de linha, ou sem `--recriar`: recusa.
- Motivo válido: registrado antes do `docker rm`.
- Variável de ambiente não ignora a guarda.
- Primeira instalação e reexecução sem `--recriar`: seguem normais.
- Ensaios atuais continuam verdes (`mariadb_docker_test.go:990,1009,1024,1038`).

**Conta:**
- Convergência cria a conta só com `SELECT` e TLS.
- `.cnf` 400 fora de `container/`.
- Conta com outro host é removida.
- Reexecução não troca a senha.

**Verificação:**
- Teste estático: `verificar-backup.sh` não contém `rotaperfumes-db`, `SEGREDOS` nem `--ip`.
- Opções como `--host`, `--container` e `--rede-nome` são recusadas.
- `docker run` sempre com `--network none`.
- Dump com `USE` é recusado.
- Saída sem nenhum dado sentinela.

**Repositório:**
- Paridade da imagem.
- Unit executa de `/usr/local/sbin`, nunca do checkout.
- `sql/` sem trigger, view, rotina ou evento.
- `shellcheck` (`Makefile:671`).

**Real, no descartável local com dados sintéticos:**
- Com a conta só `SELECT` na 11.4.13, o dump completo funciona com as flags exatas.
- `INSERT`, `UPDATE`, `DELETE`, `DROP`, `CREATE` e `TRIGGER` são negados.
- Sem TLS é recusado.
- Ida e volta preserva contagens, acentos e valores de `TIMESTAMP` e `DATETIME` (DB-08, `sql/09_ddl_clientes.sql`).
- Restaurar sobre tabela existente falha.
- `--read-only` funciona e o root sem capabilities lê o `.cnf`.
- Testes que exigem root ficam com o usuário, como `TestTST15_MariadbDocker_Real`.

Estes pontos são hipóteses até esse teste rodar, pois não executei nada.

### 8. Divisão

| Responsável | Entrega |
|---|---|
| **DataBrain** (primeiro) | Conta, `GRANT` e `.cnf` no `mariadb-docker.sh`; flags do dump; critérios de conteúdo; `verificar-backup.sh`; runbook de restauração; `docs/manual-base-de-dados.md` |
| **BackBrain** (depois) | `backup-banco.sh`: trava, temporário, retenção, `--status`, `--instalar`, units; guarda em `mariadb-docker.sh`; `docs/deploy-servidor.md`, substituindo o backup manual (`:422-440`); `Makefile:718`; mensagens que citam `--recriar` |
| **TestBrain** | Seção 7 |
| **Usuário** | Executar tudo no servidor e decidir o que está abaixo |
| **SecBrain** | Revisar antes do push, pois um push na `main` é deploy em produção |

Trabalhe em sequência em `mariadb-docker.sh`, para evitar conflito no mesmo arquivo.

## Decisões do usuário

| # | Decisão | Recomendação |
|---|---|---|
| D1 | **Cópia fora do servidor** (o backup local não protege contra perda da máquina ou do disco) | **Agora:** semanal, em disco USB cifrado guardado fora do local. **Se houver outra máquina:** ela busca os arquivos (pull) com chave SSH restrita, para o servidor invadido não conseguir apagar a cópia. |
| D2 | **Aviso de falha** | Dead-man's switch externo (ex.: healthchecks.io). Só o sucesso envia um ping, então detecta falha, timer parado e servidor desligado. Exige conta e saída HTTPS; a URL fica em arquivo de root, via `curl -K`, e falha no ping não derruba o backup. **Alternativa:** consultar `--status` à mão, sem aviso real. |
| D3 | **Cifrar o backup** | **Local:** não, pois root e o datadir continuam acessíveis e isso só adiciona gestão de chave. **Cópia externa:** sim, com `age` usando chave pública no servidor e chave privada offline, com teste de decifragem. **Pergunta em aberto:** o disco do servidor é cifrado? Em um notebook (`Makefile:299` sugere isso pelo hostname), um roubo expõe datadir e backups. |
| D4 | **Retenção** (dados pessoais apagados continuam nos backups; vale confirmar a política de LGPD) | 14 diários e 3 manuais. Na cópia externa, 4 semanais. |
| D5 | **Janela da guarda** | 60 minutos. Com 24 h, o backup das 03:30 sempre passaria, mesmo com um dia de vendas não coberto. |
| D6 | **Outros caminhos que gravam em produção** | Incluir `seed-producao.sh --forcar` neste card (`:489-494`), com a mesma exigência. Abrir card próprio para `IMPORTAR_DUMP` (`Jenkinsfile:219-235`): o Jenkins não lê um diretório 700 e pode rodar em container, então a checagem é outro desenho. |
| D7 | **Script de restauração em produção** | Não agora; só runbook. Se quiser depois, em card próprio, com confirmação digitada, backup prévio e revisão do SecBrain. |

## Riscos que permanecem
- **Até D1:** perder o disco significa perder tudo.
- **Até D2:** a rotina pode parar sem ninguém perceber.
- **Perda de até 24 h:** é o limite de um dump diário. Reduzir isso exigiria binlog, o que não recomendo agora, salvo se esse volume de perda for inaceitável.
- **Root comprometido:** lê os backups e pode apagá-los. Só uma cópia em pull ou offline resiste.
- **Pepper e senhas:** `/root/pepper-producao.txt` ainda está no servidor (`tarefas/afazer.md:411,418`). Não inclua nada de `/root` na cópia externa do dump.

## Arquivos de referência
- `/home/residentivo/.copilot/repos/RotaPerfume/deploy/mariadb-docker.sh`
- `/home/residentivo/.copilot/repos/RotaPerfume/deploy/seed-producao.sh`
- `/home/residentivo/.copilot/repos/RotaPerfume/Jenkinsfile`
- `/home/residentivo/.copilot/repos/RotaPerfume/Makefile`
- `/home/residentivo/.copilot/repos/RotaPerfume/docs/deploy-servidor.md`
- `/home/residentivo/.copilot/repos/RotaPerfume/tarefas/afazer.md`
- `/home/residentivo/.copilot/repos/RotaPerfume/backend/shared/tests/deploy/mariadb_docker_stubs_test.go`
- `/home/residentivo/.copilot/repos/RotaPerfume/backend/shared/tests/deploy/mariadb_docker_test.go`
- `/home/residentivo/.copilot/repos/RotaPerfume/backend/shared/tests/deploy/deploy07_seed_producao_test.go`

---

# BACKUP-01: entrega do DataBrain concluída

Conta, `verificar-backup.sh` e runbook estão prontos, e todas as hipóteses da especificação se confirmaram no descartável local. Nada rodou como root nem no servidor. Sem commit nem push.

## Resultados
| Verificação | Resultado |
|---|---|
| `make shellcheck` | código 0 |
| `make lint` | código 0 |
| `go clean -testcache && env -u INTEGRATION make -k test` | código 0; 44 pacotes `ok`, 0 `FAIL` |
| Validação real, sobre os scripts finais | 107 ok, 0 falhas |
| `TestBACKUP01_VerificarBackup_Real` (`BACKUP01_DOCKER=1`, sem root) | passou; é opt-in, rodei fora do `make test` |

## Arquivos
- **Alterados:**
  - `/home/residentivo/.copilot/repos/RotaPerfume/deploy/mariadb-docker.sh`
  - `/home/residentivo/.copilot/repos/RotaPerfume/docs/manual-base-de-dados.md` (nova seção 24)
  - `/home/residentivo/.copilot/repos/RotaPerfume/docs/deploy-servidor.md` (uma linha na tabela 3.4)
  - `/home/residentivo/.copilot/repos/RotaPerfume/backend/shared/tests/deploy/mariadb_docker_test.go` (`TestTST15_MariadbDocker_Real` agora espera a nova conta)
- **Novos:**
  - `/home/residentivo/.copilot/repos/RotaPerfume/deploy/verificar-backup.sh`
  - `/home/residentivo/.copilot/repos/RotaPerfume/backend/shared/tests/deploy/backup01_databrain_test.go`
- **Evidência, fora do repositório:**
  - `/tmp/claude-1000/-home-residentivo--copilot-repos-RotaPerfume/99eb3c74-bed8-4106-a3d7-f0dcd62bf83b/scratchpad/l42-validar.sh`
  - `/tmp/claude-1000/-home-residentivo--copilot-repos-RotaPerfume/99eb3c74-bed8-4106-a3d7-f0dcd62bf83b/scratchpad/l42-saida.txt`

## O que foi validado de verdade
O ambiente usou a imagem 11.4.13 fixada, o mesmo cnf de TLS/ed25519/`sql_mode`, o schema real de `sql/ordem-ddl.txt` e dados sintéticos. O SQL das contas é a função `sql_contas` extraída do próprio script.

| Hipótese | Resultado |
|---|---|
| Dump completo com a conta só `SELECT` | códigos 0 e 0, stderr vazio, 13 `CREATE TABLE`, marcador na última linha |
| Escrita e criação negadas | `INSERT`, `UPDATE`, `DELETE`, `DROP`, `CREATE` e `TRIGGER` negados, e também `TRUNCATE`, `ALTER`, `CREATE DATABASE`, view, procedure, event, `GRANT` e `SELECT` em `mysql.*` |
| Sem TLS | erro 3159 pelo `require_secure_transport`; com ele desligado, erro 1045 pelo `REQUIRE SSL` da conta |
| Origem errada | IP `.22` recusado |
| Ida e volta, em servidor com fuso +05:00 | valores idênticos lidos em -03:00: acentos, emoji, `TIMESTAMP`, `DATETIME`, `DATE`, `DECIMAL`; contagem e `CHECKSUM TABLE` iguais nas 13 tabelas; collation e constraints iguais |
| Restaurar sobre tabela existente | erro 1050, dados intactos |
| `--read-only` e `.cnf` 400 com `--cap-drop ALL` | funciona com dono root, `CapEff` zerado e TLS 1.3 verificado; um 400 de outro dono **não** é lido |
| Senha exposta | fora do `docker inspect`, e sem `MYSQL_PWD` no Env |
| `verificar-backup.sh` | aprova o dump bom em 4 s; saída sem dados sentinela; reprova truncado, `USE`, gzip inválido, symlink, dump sem tabelas e duplicata (só `ERROR 1062 ... at line N`); 10 opções recusadas com código 2 |
| Arquivo grande (400 mil linhas) | aprovado, sem falso negativo por SIGPIPE; com `--tmpfs-mb 256`, reprova com erro 1114 |
| Runbook, passos 2, 4, 5 e 6 | banco danificado restaurado pelo comando do `admin_sql`; no passo 8, só a contagem de 13 tabelas; os `GRANT` sobrevivem ao `DROP DATABASE` |

**Hipótese da especificação confirmada na prática:** `zcat | grep -q` sob `pipefail` deu falso negativo no meu validador, mesmo num dump de 5,5 KB.

## O que não foi validado
- **`mariadb-docker.sh` de ponta a ponta:** exige root. Só o ensaio com stubs cobre o script; os ajustes em `TestTST15_MariadbDocker_Real` não foram executados e ficam para o usuário, com `sudo`.
- **Permissões do `.cnf`:** o modo 400 do root veio de um `chown` em container auxiliar. O arquivo gerado pelo script real só foi conferido no ensaio com stubs.
- **Runbook:** não testei `docker compose stop/start`, o timer, o passo 7 (migrações) nem a perda do servidor de ponta a ponta. A seção 24.7 do manual registra isso.
- **Produção:** nada foi executado lá.

## Desvios da especificação
1. **`-h rotaperfumes-mariadb -P 3306` adicionados:** a lista da seção 3 não traz o host, e sem ele o dump não conecta. O `.cnf` só tem `user` e `password`.
2. **`clientes` vazia gera só `AVISO`:** a especificação pede `clientes`/`vendedores` com linhas, mas a produção registra 13 tabelas, 42 vendedores e admin, sem menção a clientes, e o seed nasce sem eles. A reprovação estrita recusaria o backup de hoje. `vendedores` vazia reprova. O 🟣 SecBrain deve confirmar, ou endurecer quando houver clientes.
3. **Reforços além do pedido em `verificar-backup.sh`:**
   - exige o marcador na última linha;
   - recusa também `DROP DATABASE`, `CREATE SCHEMA` e `\u`;
   - reprova banco extra criado pela importação;
   - usa `--sandbox` na importação e `--skip-networking` no servidor.
4. **Sem o entrypoint da imagem:** uso `mariadb-install-db` e entro pelo socket com o uid `mysql`. Assim não existe senha nenhuma e não há servidor temporário atrapalhando a espera.
5. **`--tmpfs-mb N` adicionado** (256 a 262144, padrão 2048), porque um dump grande pode não caber.
6. **`.ultima-verificacao` gravado ao lado do arquivo:** a especificação pede o aviso de 35 dias no `--status`, mas não definia quem registra.
7. **Topologia do teste:** rede Docker própria, sem porta publicada, em vez de `127.0.0.1`. A conta é por IP e precisava do `.21` real.
8. **`docs/deploy-servidor.md`:** só a linha da conta (seção 2). O resto do arquivo segue com o 🟡 BackBrain.

**Limitação conhecida:** um `SIGTERM` durante a importação só é tratado quando ela termina. O container é removido (código 143), mas não na hora.

## Para o 🟡 BackBrain
**Comando final, com `$IMG` igual ao `IMAGEM_BASE`:**
```bash
docker run --rm -i --name rotaperfumes-backup-cliente --network rotaperfumes-db --ip 172.30.42.21 \
  --read-only --cap-drop ALL --security-opt no-new-privileges:true \
  --mount type=bind,source=/etc/mysql/ssl/ca.pem,target=/run/backup/ca.pem,readonly \
  --mount type=bind,source=/root/rotaperfumes-mariadb/backup-cliente/rotaperfumes_backup.cnf,target=/run/backup/cliente.cnf,readonly \
  --entrypoint mariadb-dump "$IMG" --defaults-extra-file=/run/backup/cliente.cnf \
  -h rotaperfumes-mariadb -P 3306 --ssl-ca=/run/backup/ca.pem --ssl-verify-server-cert \
  --single-transaction --skip-lock-tables --no-tablespaces --skip-add-drop-table --skip-triggers \
  --hex-blob --default-character-set=utf8mb4 rotaperfumes
```
Validei tudo isso, menos o `--name`, que vem da especificação e o validador não usou.

**Critérios de conteúdo, com o formato real da 11.4.13:**
- **Marcador:** a última linha começa com `-- Dump completed`.
- **Tabelas:** `^CREATE TABLE ` aparece 13 vezes ou mais, e `` ^CREATE TABLE `usuarios` \( `` existe; o mesmo vale para `clientes` e `vendedores`.
- **Dados em `usuarios`:** `` ^INSERT INTO `usuarios` VALUES$ ``. **O `VALUES` termina a linha**: cada registro vem na linha seguinte, começando com `(`. Um padrão com `VALUES ` e espaço falha.
- **Armadilhas nos dados:** quebras saem escapadas como `\n`, então `USE` em dado de cliente não começa linha. Um `-- Dump completed` em dado também fica no meio da linha, o que confirma a regra da última linha.
- **Piso de 4 KiB:** o dump com 13 tabelas e 7 linhas tem 5.511 bytes. O piso funciona, com pouca folga.
- **Contagens:** use `grep -c` ou `awk`, nunca `grep -q`.

**Nomes no `mariadb-docker.sh`:**
- `DB_BACKUP`, `IP_BACKUP` (`$REDE_PREFIXO.21`) e `SENHA_BACKUP`, esta com `unset` após o uso;
- `BACKUP_CNF_DIR` e `BACKUP_CNF`, que são variáveis locais do ponto de gravação;
- o arquivo é criado depois do `sql_contas | root_sql` e trocado com `mv`.

**O que você precisa fazer:**
- **Guarda do `--recriar`:** as linhas da especificação andaram cerca de 6. Encaixe entre o fim do bloco `Container ... difere do desejado` (`die "rode de novo com --recriar..."`) e `etapa "Imagem $IMAGEM"`. Não mexi nesse trecho.
- **`Makefile`, `IMAGENS_FIXADAS_EM`:** inclua também `deploy/verificar-backup.sh`, que tem `IMAGEM_BASE` próprio. O teste `TestBACKUP01_VerificarBackup_Estatico` já cobre a paridade com o `mariadb-docker.sh`. Atualize o texto que fala em "mariadb nos três".
- **Nome do timer:** o manual cita `rotaperfumes-backup-banco.timer`, `/usr/local/sbin/rotaperfumes-backup-banco` e `/var/backups/rotaperfumes`. Se mudar o nome, ajuste a seção 24.
- **`--status`:** leia `<dir>/.ultima-verificacao` (600), com as linhas `epoch=`, `arquivo=` e `sha256=`. Esse nome não casa com o regex da retenção.
- **`pre-restauracao-*.sql.gz`:** o runbook grava esse nome em `/var/backups/rotaperfumes`. Ele fica fora da retenção, e a guarda não deve aceitá-lo como backup válido.
- **Backup manual em `docs/deploy-servidor.md`:** ele usa `--routines --triggers`, que a conta só `SELECT` não deve usar. Substitua, como previsto.

## Para o 🟣 SecBrain e o usuário
- **Lacuna fora da especificação:** as imagens de produto ficam no volume Docker `produto_imagens`, fora do dump. Numa perda do servidor, `produtos.imagem` apontaria para arquivos inexistentes. Já está no runbook, mas merece entrar na decisão D1 ou em card próprio.
- **Runbook:** segue a seção 6 da especificação; a seção 24.5 também manda parar o timer antes de restaurar, para um dump agendado não disputar o IP `.21`. Antes de confiar nele num incidente, vale um ensaio completo fora da produção.

## Limpeza do ambiente local
- **Removidos:** container, rede e imagem do Lote 42, além dos arquivos temporários do teste.
- **Volumes:** meu validador deixou 4 volumes anônimos, um por rodada. Conferi que estavam vazios e sem uso e removi só esses, pelo nome. O `verificar-backup.sh` não deixou nenhum.
- **Mantidos:** `rp-descartavel-l36` segue no ar, com o volume dele. O volume `0f8e73e1`, de 2026-11-05, não é meu e ficou.

---

# BACKUP-01: entrega do BackBrain

🟡 BackBrain: **BACKUP-01 entregue na minha parte, incluindo as imagens de produto e o destino em SSD.** Falta testar no servidor: nada rodou com o systemd real nem em produção. Sem commit, sem push, sem `sudo`, sem acesso ao servidor.

## Resultados
| Verificação | Resultado |
|---|---|
| `make shellcheck` | código 0 |
| `make lint` | código 0 |
| `go clean -testcache && env -u INTEGRATION make -k test` | código 0; 44 pacotes `ok`, 0 `FAIL` |
| Testes `BACKUP01_(Backup\|Guarda)` | 109 `PASS`, somando subtestes; 0 falhas e 0 skips |
| Validação real em Docker, com o script final | 26 ok, 0 falhas |
| `systemd-analyze verify` nas units geradas | sem apontamentos (só sintaxe) |

- **Skips:** a saída do `make test` tem 62 linhas com `SKIP`. Não as classifiquei; nenhuma é dos testes acima.
- **Ordem das rodadas:** `make shellcheck` e `make lint` rodaram antes do último ajuste, que só mudou o manual. O `make test` rodou depois dele.

## Arquivos
**Novos:**
- `/home/residentivo/.copilot/repos/RotaPerfume/deploy/backup-banco.sh`
- `/home/residentivo/.copilot/repos/RotaPerfume/backend/shared/tests/deploy/backup01_backend_linux_test.go`

**Alterados:**
- `/home/residentivo/.copilot/repos/RotaPerfume/deploy/mariadb-docker.sh`
- `/home/residentivo/.copilot/repos/RotaPerfume/deploy/seed-producao.sh`
- `/home/residentivo/.copilot/repos/RotaPerfume/Makefile`
- `/home/residentivo/.copilot/repos/RotaPerfume/Jenkinsfile` (**só comentário**, nos passos de troca de digest)
- `/home/residentivo/.copilot/repos/RotaPerfume/docs/deploy-servidor.md`
- `/home/residentivo/.copilot/repos/RotaPerfume/docs/manual-base-de-dados.md` (seção 24)
- `/home/residentivo/.copilot/repos/RotaPerfume/docs/Instal_SSL.md`
- `/home/residentivo/.copilot/repos/RotaPerfume/backend/shared/tests/deploy/tst21_seed_terminal_real_linux_test.go` (o `--forcar` agora passa `--sem-backup-motivo`)
- `/home/residentivo/.copilot/repos/RotaPerfume/backend/shared/tests/deploy/tst17_seed_producao_real_test.go` (mesmo ajuste; **não executado**, exige root)

**Evidência, fora do repositório:**
- `/tmp/claude-1000/-home-residentivo--copilot-repos-RotaPerfume/99eb3c74-bed8-4106-a3d7-f0dcd62bf83b/scratchpad/l42b-real.sh`
- `/tmp/claude-1000/-home-residentivo--copilot-repos-RotaPerfume/99eb3c74-bed8-4106-a3d7-f0dcd62bf83b/scratchpad/l42b-saida.txt`

Não toquei em `CLAUDE.md` nem em `tarefas/`, e não reverti a entrega do DataBrain. Na minha conferência, antes da suíte final, restava `rp-descartavel-l36`, que não mexi. O container `rp-descartavel-l42b`, os volumes, a rede e a imagem de teste foram removidos.

## Decisões sobre imagens e SSD
**Origem das imagens**
- O nome do volume é constante, `rotaperfumes_produto_imagens`, e o script confere as labels do compose (`rotaperfumes/produto_imagens`). Um teste confere `name:` e o volume no `docker-compose.yml`.
- A leitura usa a mesma imagem fixada, com `--network none --read-only --cap-drop ALL`, volume `readonly` e `--user 65532:65532`.
- O `--user` é necessário: o catálogo grava 750/640 e o root sem capabilities não leria esses arquivos.

**Volume ausente**
- É sempre falha, sem exceção. Toda instalação que serve o site tem o volume, criado no 1º deploy.
- Antes do 1º deploy, `--instalar` recusa e manda rodar o job.

**Dump quando as imagens falham**
- O dump já validado vira `parcial-rotaperfumes-*.sql.gz` e a execução sai com erro.
- Ele não conta como bom: não dispara a retenção e não satisfaz o `--status` nem a guarda.
- Ficam só os 3 parciais mais novos, para falhas repetidas não encherem o disco.

**Guarda do `--recriar` e do `seed --forcar`**
- Exigem só o dump, porque nenhum dos dois toca no volume. Se exigissem o par, um problema no volume bloquearia a manutenção do banco.
- Com falha nas imagens, a saída é `--sem-backup-motivo`, que fica registrado.

**Espaço livre exigido**
- A fórmula é: 2× dump anterior + o maior entre 2× imagens anteriores e o tamanho atual do volume (`du`) + 1 GiB.
- O `du` cobre a 1ª execução, que não tem par anterior.

**Verificação do tar**
- Fica no `--status` e em `--conferir-imagens`, com só contagens na saída.
- Recusa entrada absoluta, `..`, link, dispositivo e tar sem entradas.
- `verificar-backup.sh` não foi alterado.

**SSD**
- Comando: `--instalar --backup-dir DIR --exigir-montagem PONTO`.
- A unit ganha `RequiresMountsFor=`. O script confere com `mountpoint -q` no início e antes de publicar.
- `--instalar` recusa sem criar o diretório se o disco não estiver montado.
- O valor fica em `/etc/rotaperfumes/backup-banco.conf`, e `--sem-exigir-montagem` o remove.
- `ReadWritePaths` e a conferência de espaço seguem o destino configurado.
- Cifragem e abertura do disco ficam fora do script.

**Valores configuráveis**
- Retenção: `--retencao-diarios` e `--retencao-manuais` (14 e 3).
- Janela da guarda: `--backup-janela-min` (60).
- O ponto para o ping (D2) é só um comentário no fim de `executar_backup`.

## O que foi validado de verdade
Foi usado o `--agora` real do script, com a imagem 11.4.13, TLS e a conta só `SELECT` criada pelo `sql_contas` real, com o schema real e dados sintéticos. O volume sintético tinha dono 65532 e modos 750/640. Conferi antes que não havia volume `rotaperfumes_*` nem rede `rotaperfumes-db` nesta máquina.

- **Par:** mesmo carimbo nos dois arquivos, modo 600; a saída não traz dados, nomes do tar nem senha.
- **Conferências:** `--conferir-imagens`, `--status` e `verificar-backup.sh` aprovam o dump do script.
- **Ida e volta do volume:** com os dois comandos do passo 6a do manual, sobre um volume com lixo. Nomes, dono, modos e sha256 ficaram idênticos.
- **Casos de borda:**
  - volume vazio passa com `AVISO`;
  - volume ausente gera parcial, mantém os pares e não cria o volume;
  - SIGTERM durante o tar sai com código 143 na hora, sem `.tmp-*` nem cliente;
  - a retenção remove por par.

**Limite dessa validação:** rodei como não root, em user namespace. Um wrapper de `docker` trocou só o nome da rede, o nome do volume e os caminhos que o daemon resolve. O `.cnf` montado era uma cópia 400 de dono root real.

**Só em ensaio (stubs ou namespace):**
- as guardas do `--recriar` e do `seed --forcar`;
- as falhas de conteúdo do dump;
- a queda de 50%;
- a trava;
- o `--instalar`, com `systemctl` falso;
- o `--exigir-montagem`, com `mountpoint` real sobre tmpfs, incluindo o caso desmontado sem gravar nada.

## O que não foi validado
- **Timer, sandbox da unit e `RequiresMountsFor=` no systemd real.** Duas hipóteses merecem atenção na primeira execução:
  - o cliente Docker funcionar sob `ProtectSystem=strict`;
  - o estado da unit durante as novas tentativas, que o `--instalar` acompanha por `systemctl show`.
- **Disco físico** e `mariadb-docker.sh` de ponta a ponta.
- **`TestTST15_MariadbDocker_Real` e o teste real do seed**, que exigem root.
- **`docker compose stop/start` da API** e a seção 24.6 de ponta a ponta.

## Desvios da especificação
1. **`ln -T` + `rm` em vez de `mv`:** `ln` recusa destino existente de forma atômica, e `mv -n` não garante isso em todas as versões.
2. **Piso de 4 KiB sobre o dump descomprimido:** a especificação não dizia qual tamanho, e o dump real comprimido pode ficar abaixo disso. A regra dos 50% compara os `.gz`. `--aceitar-reducao` não dispensa o piso.
3. **`--agora` e `--status` recusam rodar do checkout**, em vez de redirecionar para a cópia. `--conferir-imagens` pode rodar do checkout, porque só lê o arquivo e é necessário na perda do servidor.
4. **1º backup com `systemctl start --no-block`, acompanhando o estado:** com `Restart=on-failure`, um `start` bloqueante de oneshot pode ficar preso durante as novas tentativas. Em caso de falha, o script para a unit e não liga o timer.
5. **`--sem-backup-motivo` e `--backup-janela-min` também no seed**, com registro em `seed-forcar.registro`. A guarda vem antes da confirmação digitada.
6. **Idade do dump conferida pelo carimbo do nome e pelo mtime:** um `touch` ou `cp` de arquivo antigo não passa.
7. **Reforço:** `.tmp-*` antigos são apagados com a trava na mão (sobra de SIGKILL).
8. **`--dados`/`--recriar` e imagens:** `--recriar --dados /vazio` continua sem disparar a guarda, risco já registrado para card próprio.

## Limites conhecidos
- O dump e o tar não são do mesmo instante: um upload entre os dois fica sem par exato.
- O GNU tar sai com erro se um arquivo mudar durante a leitura. Isso gera parcial e nova tentativa pela unit.
- Um SIGTERM apaga também o dump temporário.
- Dump sem imagens não conta como bom e nunca é apagado pela retenção. O `--status` avisa.
- Com `--backup-dir` fora do padrão, é preciso passá-lo também às guardas, que não leem o conf.
- Trocar o destino não move os backups antigos, e a retenção do diretório anterior para de rodar.

## Inventário: o que o site precisa e não está no backup
Não li nenhum valor; a tabela vem de nomes em `deploy/api.env.example`, do compose, do `Jenkinsfile` e da documentação.

| Item | Onde vive | Dá para regenerar? |
|---|---|---|
| `PASSWORD_PEPPER` | Secret file `rotaperfumes-api-env` no Jenkins; `/root/pepper-producao.txt` ainda no servidor | **Não.** Sem ele, nenhum login funciona; todas as senhas precisam ser redefinidas |
| `JWT_SECRET` | mesmo Secret file | Sim; só as sessões caem |
| `TURNSTILE_SECRET_KEY` | mesmo Secret file; origem no painel da Cloudflare | Sim, pelo painel |
| `TURNSTILE_SITE_KEY` (pública) | parâmetro do job no `Jenkinsfile` | Sim, já está no repositório e no painel |
| `SMTP_PASSWORD`, `SMTP_*`, `SECURITY_ALERT_EMAILS` | mesmo Secret file; origem no provedor de e-mail | Sim, pelo provedor; os demais valores precisam ser anotados |
| Demais variáveis (`CORS_ALLOWED_ORIGINS`, `TURNSTILE_HOSTNAMES` etc.) | mesmo Secret file; modelo em `deploy/api.env.example` | Sim, mas à mão |
| `DB_SENHA` da API e credencial `rotaperfumes-db-admin` | Secret file e Jenkins; origem em `/root/rotaperfumes-mariadb/*.senha` | Sim; o `mariadb-docker.sh` cria senhas novas e as credenciais precisam ser atualizadas |
| `root.senha`, `.cnf` do backup e demais senhas | `/root/rotaperfumes-mariadb` | Sim, numa instalação nova. Com o datadir antigo, sem `root.senha`, o script recusa |
| CA do banco e certificados | `/etc/mysql/ssl` | Sim, com `--nova-ca`; exige trocar `deploy/db-ca.fingerprint` |
| `ca-key.pem` | `/root/rotaperfumes-ca` (ainda no servidor; deveria ir para mídia offline) | Sim, gerando outra CA; sem ela, a CA atual não reemite certificados |
| `caddy_data` | volume Docker | Sim. A CA interna muda, e a raiz nova precisa ser reinstalada nos clientes; o certificado de `mail.rotaperfumes.com.br` é reemitido |
| `caddy_config` | volume Docker | Sim, é gerado pelo Caddy |
| Configuração do Jenkins (job e credenciais) | no próprio Jenkins, no servidor | Sim, à mão: `docs/deploy-servidor.md`, seções 5 e 6 |
| Cloudflare Tunnel (`cloudflared`) | host do servidor; painel da Cloudflare | Sim, à mão: seção 1 do mesmo documento. Confira se o token vive só lá |
| Firewall e units | gerados pelo `mariadb-docker.sh` e pelo `--instalar` | Sim, rodando os scripts |
| Código, `deploy/`, `sql/` | repositório GitHub | Sim, desde que haja push: a `main` local está à frente da origin |

**Maior risco a decidir:** o pepper é o único item sem como regenerar. Ele precisa de uma cópia em cofre, separada do dump, e essa cópia não pode ser `/root`. Os secrets do Secret file também só existem no Jenkins e, em parte, nos painéis. Uma cópia do conteúdo em cofre evita refazer tudo à mão.

**Em aberto para o usuário:**
- D1 e D2 seguem sem decisão: sem cópia externa, perder o disco ainda significa perder tudo; sem aviso, uma falha só aparece no `--status`.
- O SSD no mesmo servidor protege contra falha do disco do sistema, mas não contra perda da máquina nem contra root comprometido.

## Comandos do usuário no servidor, em ordem
Depois da revisão do 🟣 SecBrain e do push na `main`, que é deploy em produção:

1. **Atualize o checkout:** item 6 da limpeza, seção 11, passo 1. Depois: `cd /root/rotaperfumes`
2. **Crie as contas novas, em horário calmo e sem `--recriar`** (atualizado no fechamento do Lote 42, 2026-11-07, com o DEPLOY-10; item 6a do DEPLOY-08 e `docs/deploy-servidor.md` §11, passo 11). Uma única execução, com o `--dev-ip` em uso enquanto a porta estiver publicada (só transição, sem conta): cria `rotaperfumes_backup` e `rotaperfumes-admin`, remove `casaos@*` com qualquer host e instala o `/usr/local/sbin/rotaperfumes-dba`, sem reiniciar o container:
   `sudo bash deploy/mariadb-docker.sh --dev-ip 192.168.168.105`
   Confira: `echo 'SELECT CURRENT_USER()' | sudo rotaperfumes-dba` deve trazer `rotaperfumes-admin@172.30.42.22`. Saída se algo falhar: `root@localhost`.
3. **Instale a rotina:** `sudo bash deploy/backup-banco.sh --instalar`
   - Com SSD já cifrado e montado, acrescente `--backup-dir /mnt/ssd/rotaperfumes --exigir-montagem /mnt/ssd`.
4. **Confira:** `sudo rotaperfumes-backup-banco --status`
5. **Verifique o dump:** `sudo bash deploy/verificar-backup.sh /var/backups/rotaperfumes/rotaperfumes-AAAAMMDD-HHMMSSZ.sql.gz`
6. **Confira o timer e o log:** `systemctl list-timers rotaperfumes-backup-banco.timer` e `journalctl -u rotaperfumes-backup-banco.service -n 20`
7. **Antes do item 7 da limpeza:** `sudo rotaperfumes-backup-banco --agora`, e até 60 minutos depois: `sudo bash deploy/mariadb-docker.sh --recriar`
8. **No dia seguinte:** repita o `--status` para confirmar que o timer rodou sozinho.

Com outro destino, passe `--backup-dir` no passo 7 e use o caminho certo no passo 5.

## Pendências para outros agentes
- **🔴 TestBrain:** completar a seção 7. Falta a regressão do SIGPIPE com arquivo grande no `backup-banco.sh` e o dono errado do destino, entre outros; o ensaio cobre só parte da lista. Rodar com `sudo` o `TestTST15_MariadbDocker_Real` e o teste real do seed.
- **🔵 SubBrain:**
  - atualizar o `CLAUDE.md`, que ainda cita o digest só no `Jenkinsfile` e no `mariadb-docker.sh`;
  - ajustar o item 2 da limpeza em `tarefas/afazer.md`, porque a seção agora se chama "Backup do banco e das imagens de produto" e o backup manual saiu;
  - abrir os cards de `IMPORTAR_DUMP` e de `-e MYSQL_PWD`.
- **🟣 SecBrain, revisar antes do push:**
  - `--sem-backup-motivo` no seed;
  - parciais fora da guarda;
  - exceção do `--conferir-imagens` no checkout;
  - `clientes` vazia gerar só `AVISO`, vindo do DataBrain.
