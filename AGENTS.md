# AGENTS.md

Navodila za kodirne agente (Claude Code ipd.) na projektu **CyberMedico**: platforma za spletne konzultacije s specialisti (pacient naroči pisno ali video konzultacijo, zdravnik odgovori, mnenje je digitalno podpisano).

## Tehnologije

- **Go 1.27** monolit (`net/http`, ogen iz OpenAPI), strežniško izrisovanje s **templ** + **Datastar**
- **PostgreSQL** (pgx, golang-migrate), opravila v ozadju z **River**
- S3 združljiva shramba (lokalno Silo/MinIO), e-pošta (lokalno Mailpit)
- Avtorizacija z **Open Policy Agent** (Rego), TOTP, podpisi Ed25519
- Orodja v točnih različicah prek **mise** (`mise.toml`)

Ni ločenega frontenda: strani izriše isti Go strežnik (ADR 0001, 0002).

## Ukazi

| Namen | Ukaz |
|---|---|
| Namestitev orodij | `mise trust && mise install` |
| Zagon vsega (Docker + templ + app + Caddy) | `mise run develop` |
| Samo aplikacija (`http://localhost:8080`) | `mise run api:run` |
| Vsebniki gor / dol | `mise run docker:up` / `mise run docker:down` |
| Lint (Go) | `mise exec -- golangci-lint run ./...` |
| Lint (Rego) | `mise exec -- opa check authorization/policies authorization/policies_test` |
| Testi (Go) | `mise exec -- go test ./...` |
| Testi (Rego) | `mise run test:policies` |
| HTTP testi (potrebujejo zagnano app) | `mise run test:hurl` |
| Generiranje templ kode | `mise exec -- templ generate` |
| Generiranje strežnika iz OpenAPI | `mise run generate` |

Lokalni računi: `admin@cybermedico.com`, `dr.carter@cybermedico.com` (geslo `cybermedico-dev`, TOTP skrivnost na vrhu `resources/config/cybermedico/compose.sql`). Samo za razvoj.

## Struktura

```
cmd/cybermedico/
  main.go            zagon: migracije, seed, River, HTTP strežnik
  openapi.yml        opis HTTP vmesnika (vir resnice za poti)
  ogen/              GENERIRANO iz openapi.yml – ne urejaj ročno
  api/html/          HTTP handlerji (*.go) in strani (*.templ)
  api/util/          pomožne funkcije (kontekst, čas, teme)
  migrations/        SQL migracije (NNNNN_ime.up.sql)
  signing/ storage/ email/   podpisi, S3, pošiljanje e-pošte
  static/            CSS, JS, slike (vgrajeno z go:embed)
authorization/policies/       Rego pravila, testi v policies_test/
components/ layouts/ widgets/ email_templates/   templ komponente
video/               WebRTC signalizacija (WebSocket)
workers/             River opravila (e-pošta, opomniki, podpisi, roki)
tests/hurl/          HTTP testi
resources/config/    lokalne nastavitve (compose.env, compose.sql, Caddyfile)
docs/                dokumentacija, ADR, specifikacije
```

## Dokumentacija – kdaj jo prebrati

- `docs/02-funkcionalnosti-in-obseg.md` – vloge in funkcionalnosti F1–F11; **preden** dodaš novo funkcionalnost.
- `docs/05-uporabniske-zgodbe.md` – uporabniške zgodbe s sprejemnimi kriteriji; **preden** implementiraš ali spreminjaš obnašanje.
- `docs/03-podatkovni-model.md` – tabele in relacije; **preden** pišeš SQL ali migracijo.
- `docs/04-arhitektura-in-tehnoloski-sklad.md` in `docs/adr/` – arhitekturne odločitve; **preden** uvedeš novo knjižnico ali vzorec.
- `docs/06-wireframes.md`, `docs/wireframes/` – izgled strani; pri delu na UI.
- `docs/specs/NNN-ime/` – specifikacije novih funkcionalnosti (spec → plan → tasks).

## Konvencije

- **Go**: `gofmt`, imena po Go konvencijah (CamelCase za izvoz, kratka imena v ozkem obsegu), napake vedno preveri in ovij z `fmt.Errorf("...: %w", err)`.
- **Testi**: standardni paket `testing`, tabelarični testi s `t.Run`, datoteka `*_test.go` ob kodi; brez zunanjih test knjižnic.
- **HTTP**: nova pot se najprej doda v `openapi.yml`, nato `mise run generate`, nato handler v `api/html/`.
- **UI**: strani v `*.templ`, po spremembi `templ generate`; generirane `*_templ.go` datoteke se commitajo.
- **SQL**: parametrizirane poizvedbe (`@ime` / `$1`), nikoli lepljenje nizov.
- **Avtorizacija**: pravila v Rego, vsako novo pravilo ima test v `policies_test/`.
- **Commiti**: Conventional Commits (`feat:`, `fix:`, `test:`, `docs:`, `ci:`, `chore:`), ena logična sprememba na commit.
- **Veje in PR**: delo v ločeni veji, združitev v `main` samo prek PR z zelenim cevovodom.
- Besedila v UI in e-pošti so v angleščini, dokumentacija v slovenščini.

## Pravilo pred oddajo

Pred vsakim commitom oziroma predlogom spremembe agent **sam** požene:

```
mise exec -- golangci-lint run ./...
mise exec -- go test ./...
mise run test:policies
```

Sprememba je pripravljena šele, ko so vsi trije ukazi uspešni. Če se spremeni `openapi.yml` ali `*.templ`, prej požene še generiranje.

## Brez izrecne potrditve uporabnika NE spreminjaj

- `cmd/cybermedico/migrations/` – obstoječih migracij nikoli ne spreminjaj; novo migracijo predlagaj in počakaj na potrditev.
- `.github/workflows/` – CI/CD cevovod.
- `resources/config/` in vse `.env` datoteke – konfiguracija okolij in skrivnosti.
- `mise.toml`, `mise.lock`, `go.mod`, `go.sum`, `vendor/` – orodja in odvisnosti.
- `cmd/cybermedico/ogen/` in `*_templ.go` – samo prek generatorja, nikoli ročno.
- `.claude/settings.json`, `.mcp.json`, `AGENTS.md` – nastavitve agenta.

## Varnostne omejitve

Nastavljene v `.claude/settings.json`:

- prepovedano branje `.env*`, `*.pem`, `*.key` in `resources/config/**/compose.env`;
- prepovedani ukazi `rm -rf`, `git push --force` / `-f`, `git reset --hard`, `sudo`;
- `git push` in `gh pr merge` zahtevata potrditev uporabnika;
- hook po vsakem urejanju `.go` datoteke zažene `gofmt` na spremenjeni datoteki.

Tokeni za MCP strežnike so samo v okoljskih spremenljivkah, nikoli v repozitoriju.
