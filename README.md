# CyberMedico

[![CI/CD](https://github.com/SimeonJovanovski32/rir/actions/workflows/ci-cd.yml/badge.svg?branch=main)](https://github.com/SimeonJovanovski32/rir/actions/workflows/ci-cd.yml)

**Nameščena aplikacija:** https://cybermedico-ryji.onrender.com

## CI/CD cevovod

Cevovod (`.github/workflows/ci-cd.yml`) se zažene ob vsakem pull requestu v `main` in ob vsakem pushu na `main`.

| Faza | Kaj naredi |
|---|---|
| `lint` | golangci-lint (konfiguracija v `.golangci.yml`) preveri Go kodo, `opa check` pa pravila Rego. Ob napaki job pade. |
| `build` | Zgradi statično Go binarno datoteko (z vgrajenimi statičnimi datotekami, migracijami in pravili) in jo skupaj z `Dockerfile` ter začetnimi podatki shrani kot artefakt `cybermedico-build`. Go build cache se predpomni prek `actions/setup-go`. |
| `test` | Zažene Go teste (`go test ./... -race`) in Rego teste (`opa test`). Poročilo o pokritosti (`coverage.html`, `coverage.out`, `opa-coverage.json`) shrani kot artefakt `coverage-report`. Če kateri koli test pade, pade cevovod. |
| `deploy` | Samo ob pushu na `main` in samo, če so `lint`, `build` in `test` uspešni. V okolju `production` prenese artefakt iz `build` (aplikacije ne gradi ponovno), ga zapakira v Docker sliko, jo objavi v GitHub Container Registry in prek deploy hooka namesti na Render. Na koncu počaka, da se aplikacija odzove na javnem URL-ju. |

Podatkovna baza je PostgreSQL na Neon, priloge in podpisi pa so v S3 združljivi shrambi Backblaze B2. Skrivnosti so shranjene kot GitHub Secrets okolja `production` oziroma kot spremenljivke okolja na Renderju, nikoli v kodi.

Platforma za spletne konzultacije s specialisti. Pacient, ki želi mnenje specialista, pogosto čaka več tednov ali mora potovati v drug kraj. CyberMedico mu omogoča, da od doma naroči konzultacijo pri izbranem zdravniku in mnenje prejme v nekaj dneh, brez obiska ambulante.

## Kako deluje

- **Pacient** (brez registracije) izbere specialista, izpolni obrazec s prilogami in naroči **pisno** ali **video** konzultacijo. Termin in povezavo za video klic prejme po e-pošti, klic poteka v brskalniku. Mnenje prejme po e-pošti, digitalno podpisano.
- **Zdravnik** se prijavi z geslom in TOTP kodo, odgovori na pisno konzultacijo v 72 urah ali določi termin video klica v 12 urah (nujno: 8 ur) ter po klicu odda mnenje v 24 urah. Pred vsakim rokom prejme opomnik.
- **Administrator** pregleduje vse konzultacije, tudi tiste, ki jih zdravnik ni opravil v roku.
- **Nadzorna plošča** zdravniku in administratorju prikaže statistiko z grafi: število konzultacij in gibanje v zadnjih 30 dneh, razmerje vrst, konzultacije po statusu oziroma po zdravnikih, prihodek v mesecu in odprte konzultacije.
- **Sistem** samodejno pošilja e-pošto in opomnike, podpiše mnenja in označi konzultacije, ki niso bile opravljene v roku.

## Uporabljene tehnologije

| Področje | Tehnologije |
|---|---|
| Frontend | templ, Datastar, Pico CSS, Quill, Gridstack + ECharts (nadzorna plošča), WebRTC |
| Backend | Go, `net/http`, ogen (OpenAPI), River (opravila v ozadju) |
| Podatki | PostgreSQL, pgx, golang-migrate, S3 (lokalno Silo, MinIO združljiv) |
| Varnost | bcrypt, TOTP, Open Policy Agent (Rego), HMAC žetoni, Ed25519 (zasebni ključi šifrirani z AES-256-GCM) |
| Razvoj | mise, Docker Compose, Caddy, Mailpit, golangci-lint, Hurl |
| Video | WebRTC, WebSocket signalizacija, coturn (TURN) |

## Lokalni zagon

### Predpogoji

- [mise](https://mise.jdx.dev/) - namesti vsa ostala orodja (Go, templ, ogen, Caddy, Hurl, OPA, golangci-lint) v različicah iz `mise.toml`.
- [Docker](https://www.docker.com/) z Docker Compose - PostgreSQL, shramba S3 (Silo), Mailpit in coturn.
- Za dostop prek `https://cybermedico.local` (povezave v e-pošti kažejo tja) vnos v datoteki hosts (`/etc/hosts`, na Windows `C:\Windows\System32\drivers\etc\hosts`):

  ```text
  127.0.0.1 cybermedico.local
  ```

### Zagon

```bash
mise trust
mise install
mise run develop
```

`develop` zažene vsebnike (`docker:up`), nato vzporedno templ (ponovno generiranje ob spremembi `.templ`), aplikacijo in Caddy. Ob zagonu aplikacija izvede migracije, naloži začetne podatke iz `resources/config/cybermedico/compose.sql` (vsakič na novo) in ustvari ključe za podpisovanje zdravnikom, ki jih še nimajo.

| Naslov | Kaj |
|---|---|
| `http://localhost:8080` | Aplikacija |
| `https://cybermedico.local` | Aplikacija prek Caddy (HTTPS; potrebno za video klic iz povezave v e-pošti) |
| `http://localhost:8025` | Mailpit: vsa poslana e-pošta |
| `http://localhost:9001` | Konzola shrambe S3 (`minioadmin` / `minioadmin`) |

Caddy uporablja lokalno certifikatno agencijo. Da ji brskalnik zaupa, jo enkrat namestite z `mise exec -- caddy trust`. Caddy posluša tudi na vratih 80; če so na Windows zasedena (IIS, HTTP.sys), v `resources/config/caddy/Caddyfile` nastavite drug `http_port`.

### Prijava za razvoj

Vsi računi v začetnih podatkih imajo geslo `cybermedico-dev` in skupno TOTP skrivnost, zapisano na vrhu `resources/config/cybermedico/compose.sql`; dodajte jo v aplikacijo za avtentikacijo (npr. Google Authenticator). Ti podatki so samo za lokalni razvoj.

| Račun | Vloga |
|---|---|
| `admin@cybermedico.com` | Administrator |
| `dr.carter@cybermedico.com` | Zdravnica (kardiologija) |
| `dr.wilson@cybermedico.com` | Zdravnik (nevrologija) |
| `dr.mitchell@cybermedico.com` | Zdravnica (endokrinologija) |
| `dr.bennett@cybermedico.com` | Zdravnik (ortopedija) |

### Opravila

| Ukaz | Kaj naredi |
|---|---|
| `mise run develop` | Zažene vse za razvoj |
| `mise run docker:up` / `docker:down` | Zažene oziroma ustavi vsebnike |
| `mise run api:run` | Zažene samo aplikacijo na `http://localhost:8080` |
| `mise run generate` | Ustvari strežniško kodo iz `cmd/cybermedico/openapi.yml` |
| `mise run lint` | golangci-lint s popravki |
| `mise run test:policies` | Testi pravil za avtorizacijo (Rego) |
| `mise run test:hurl` | HTTP testi proti zagnani aplikaciji |
| `go test ./...` | Go testi (podpisi, žetoni, video signalizacija) |

### Nastavitve

Vse nastavitve so spremenljivke okolja v `resources/config/cybermedico/compose.env`. Vrednosti v tej datoteki so samo za lokalni razvoj; v produkciji jih je treba zamenjati.

| Spremenljivka | Namen |
|---|---|
| `PORT` | Vrata aplikacije (privzeto 8080) |
| `CYBERMEDICO_SEED_FILE` | Začetni podatki, naloženi ob zagonu (v produkciji pustite prazno) |
| `POSTGRES_*` | Povezava na PostgreSQL |
| `S3_*` | Shramba za priloge (`S3_BUCKET`) in podpise (`S3_SIGNATURES_BUCKET`) |
| `SMTP_*`, `EMAIL_SENDER_*` | Pošiljanje e-pošte |
| `PUBLIC_BASE_URL` | Javni naslov aplikacije za povezave v e-pošti |
| `WEBRTC_JOIN_SECRET` | Skrivnost za povezave pacientov v video klic (HMAC) |
| `TURN_HOSTNAME`, `TURN_SHARED_SECRET`, `TURN_CREDENTIAL_TTL` | Strežnik TURN in časovno omejene poverilnice; brez njih se klic poveže le neposredno |
| `SIGNING_KEY` | 32 naključnih bajtov (base64), s katerimi so šifrirani zasebni ključi zdravnikov; nov ključ: `openssl rand -base64 32` |

## Dokumentacija

Vsa dokumentacija je v mapi [`docs/`](docs/):

- [Zasnova rešitve](docs/01-zasnova-resitve.md)
- [Funkcionalnosti in obseg](docs/02-funkcionalnosti-in-obseg.md)
- [Podatkovni model](docs/03-podatkovni-model.md)
- [Arhitektura in tehnološki sklad](docs/04-arhitektura-in-tehnoloski-sklad.md)
- [Uporabniške zgodbe](docs/05-uporabniske-zgodbe.md)
- [Wireframes](docs/06-wireframes.md)
- [Preverjanje MVP](docs/07-preverjanje-mvp.md)
- [Arhitekturne odločitve (ADR)](docs/adr/)

## Agentno programiranje

Projekt je pripravljen za delo s kodirnim agentom **[Claude Code](https://code.claude.com)** (uporabljen model: Claude Opus 5.5).

### Zagon

```bash
# enkratna namestitev
curl -fsSL https://claude.ai/install.sh | bash

# token za GitHub MCP (fine-grained, samo ta repozitorij) – nikoli v repozitorij
export GITHUB_PERSONAL_ACCESS_TOKEN=...

# zagon v korenu repozitorija
claude
```

Ob prvem zagonu potrdi zaupanje mapi in uporabo MCP strežnika iz `.mcp.json`. Z `/memory` preveriš, da je naložen `AGENTS.md`, z `/hooks` hook, z `/mcp` povezavo z GitHubom.

### Datoteke za agenta

| Datoteka / mapa | Namen |
|---|---|
| `AGENTS.md` | glavna navodila: ukazi za zagon, lint in teste, struktura projekta, kje je dokumentacija, konvencije, zaščitene datoteke, varnostne omejitve, MCP |
| `CLAUDE.md` | za Claude Code; uvozi `AGENTS.md` (`@AGENTS.md`) |
| `.claude/settings.json` | dovoljenja: samodejno dovoljeni lint, testi in generiranje; potrditev za `git push`, merge in spremembe migracij/workflowov; prepovedano branje `.env`/ključev ter `rm -rf`, `git push --force`, `git reset --hard`, `sudo` |
| `.claude/hooks/format.sh` | hook po vsakem urejanju: `gofmt` za `.go`, `templ fmt` za `.templ` |
| `.claude/skills/add-http-endpoint/` | skill za novo HTTP pot: `openapi.yml` → `mise run generate` → handler → templ → Rego → testi |
| `.claude/skills/write-tests/` | skill za Go, Rego in Hurl teste po konvencijah projekta |
| `.mcp.json` | GitHub MCP strežnik (PR, issues); token iz okoljske spremenljivke `GITHUB_PERSONAL_ACCESS_TOKEN` |
| `docs/specs/NNN-ime/` | specifikacijski tok za nove funkcionalnosti: `spec.md` → `plan.md` → `tasks.md` |

### Potek dela z agentom

1. Napiši `docs/specs/NNN-ime/spec.md` (zahteve in sprejemni kriteriji).
2. Agent pripravi `plan.md`, ki ga pregledaš in po potrebi popraviš.
3. Agent razdeli načrt na naloge v `tasks.md`.
4. Agent implementira naloge po skupinah (s testi), pred vsakim commitom sam požene lint in teste.
5. Odpreš PR v `main`; cevovod mora biti zelen, PR ročno pregledaš.

Primer: [`docs/specs/001-razpolozljivost-zdravnikov`](docs/specs/001-razpolozljivost-zdravnikov/).
